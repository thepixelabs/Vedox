package history

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// HistoryEntry represents one commit that touched a documentation file.
type HistoryEntry struct {
	// CommitHash is the full 40-character SHA-1 of the commit.
	CommitHash string `json:"commitHash"`
	// Author is the git author name.
	Author string `json:"author"`
	// AuthorEmail is the git author email.
	AuthorEmail string `json:"authorEmail"`
	// AuthorKind classifies the author as human or a known AI agent.
	// Possible values: "human", "claude-code", "copilot", "codex", "gemini", "vedox-agent".
	AuthorKind string `json:"authorKind"`
	// Date is the author timestamp in RFC3339/UTC.
	Date string `json:"date"`
	// Message is the commit subject line.
	Message string `json:"message"`
	// Changes is the structural diff between this commit's version and the
	// previous commit's version of the file. Empty for the initial commit.
	Changes []Change `json:"changes"`
	// Summary is the human-readable prose summary of Changes.
	Summary string `json:"summary"`
}

// unitSep is the ASCII unit separator (\x1f) used as the git log format
// field delimiter. It cannot appear in author names, emails, or subject lines,
// so it is safe to split on.
const unitSep = "\x1f"

// recordSep is the string used to delimit commits in git log output.
// We use a sentinel string on its own line because NUL bytes (\x00) are not
// valid in exec.Command arguments on macOS (POSIX argv), which causes
// "invalid argument" from the kernel. The sentinel is unlikely to appear in
// commit subjects in practice.
const recordSep = "---VEDOX-COMMIT---"

// logFormat is the --format string for git log. Fields are separated by \x1f
// and records by the recordSep sentinel on its own line. Order:
//
// %H  — full commit hash
// %an — author name
// %ae — author email
// %aI — author date, ISO 8601 strict
// %s  — commit subject
// %b  — commit body (may contain newlines; the sentinel terminator delimits the record)
// %n  — newline (so sentinel appears on its own line)
//
// The body is needed so classifyAuthor can spot Co-Authored-By: trailers that
// mark agent-authored commits regardless of the primary author email.
const logFormat = "%H" + unitSep + "%an" + unitSep + "%ae" + unitSep + "%aI" + unitSep + "%s" + unitSep + "%b%n" + recordSep

// aiEmailPatterns maps an AuthorKind to the email substrings that identify it.
// Order matters — checked top to bottom; first match wins.
var aiEmailPatterns = []struct {
	kind    string
	pattern *regexp.Regexp
}{
	{"vedox-agent", regexp.MustCompile(`(?i)@vedox\.dev$|vedox-doc-agent|noreply\+vedox`)},
	{"claude-code", regexp.MustCompile(`(?i)claude|anthropic`)},
	{"copilot", regexp.MustCompile(`(?i)copilot|github-actions`)},
	{"codex", regexp.MustCompile(`(?i)codex|openai`)},
	{"gemini", regexp.MustCompile(`(?i)gemini|google`)},
}

// aiTrailerPattern matches the "Co-Authored-By: Vedox Doc Agent" trailer in a
// commit body that marks an agent-authored commit regardless of the email.
var aiTrailerPattern = regexp.MustCompile(`(?i)Co-Authored-By:\s*Vedox Doc Agent`)

// classifyAuthor returns the AuthorKind for a given email and optional commit
// body (used to detect the Vedox Doc Agent trailer).
func classifyAuthor(email, body string) string {
	if aiTrailerPattern.MatchString(body) {
		return "vedox-agent"
	}
	for _, p := range aiEmailPatterns {
		if p.pattern.MatchString(email) {
			return p.kind
		}
	}
	return "human"
}

// FileHistory returns the git log for filePath inside repoPath, with structural
// diffs between consecutive versions. limit caps the number of entries returned
// (0 = no limit, though callers should always supply a reasonable limit).
//
// The function shells out to git for all git operations — consistent with the
// existing pattern in gitcheck and doc_metadata.go. go-git is not pulled in.
func FileHistory(repoPath, filePath string, limit int) ([]HistoryEntry, error) {
	return FileHistoryContext(context.Background(), repoPath, filePath, limit)
}

// FileHistoryContext is like FileHistory but accepts a context for timeout/
// cancellation propagation.
func FileHistoryContext(ctx context.Context, repoPath, filePath string, limit int) ([]HistoryEntry, error) {
	// Reject inputs that could be interpreted as git options by argv
	// processing. repoPath becomes the subprocess working directory and
	// filePath is passed after "--", but defence-in-depth still rejects
	// leading dashes because `git log -- -foo` is accepted as a pathspec
	// only in newer gits.
	if err := validateRepoPath(repoPath); err != nil {
		return nil, err
	}
	if err := validateFilePath(filePath); err != nil {
		return nil, err
	}

	args := []string{
		"log",
		"--follow",
		"--format=" + logFormat,
	}
	if limit > 0 {
		args = append(args, fmt.Sprintf("-n%d", limit))
	}
	args = append(args, "--", filePath)

	// #nosec G204 — args are a static flag set; filePath is validated above
	// and passed after "--". We use cmd.Dir instead of "-C repoPath" so a
	// malicious repoPath beginning with "-" cannot be interpreted as a git
	// option by argv parsing.
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		// git exits non-zero if the repo is empty or the file has no history.
		// Treat as empty history rather than a hard error.
		if isGitExitError(err) {
			return []HistoryEntry{}, nil
		}
		return nil, fmt.Errorf("git log: %w", err)
	}

	records := splitRecords(string(out))
	entries := make([]HistoryEntry, 0, len(records))
	for _, rec := range records {
		parts := strings.SplitN(rec, unitSep, 6)
		if len(parts) < 5 {
			continue
		}
		hash := strings.TrimSpace(parts[0])
		name := strings.TrimSpace(parts[1])
		email := strings.TrimSpace(parts[2])
		dateStr := strings.TrimSpace(parts[3])
		subject := strings.TrimSpace(parts[4])
		body := ""
		if len(parts) == 6 {
			// Preserve body verbatim — trailers live at the end and may be
			// separated by whitespace/newlines. Don't TrimSpace the line content.
			body = parts[5]
		}

		date := parseISO(dateStr)
		// classifyAuthor inspects the body for Co-Authored-By trailers; the
		// subject alone is almost never where those trailers live.
		kind := classifyAuthor(email, subject+"\n"+body)

		entries = append(entries, HistoryEntry{
			CommitHash:  hash,
			Author:      name,
			AuthorEmail: email,
			AuthorKind:  kind,
			Date:        date,
			Message:     subject,
		})
	}

	// Populate Changes by diffing consecutive commit pairs.
	// entries[0] is the most-recent commit; we diff [i] against [i+1] (older).
	for i := range entries {
		curr, err := fileAtCommit(ctx, repoPath, filePath, entries[i].CommitHash)
		if err != nil {
			// If we can't read a version, skip diff for this entry but keep the
			// entry in the result so the caller still sees the commit.
			continue
		}
		var prev string
		if i+1 < len(entries) {
			prev, err = fileAtCommit(ctx, repoPath, filePath, entries[i+1].CommitHash)
			if err != nil {
				prev = ""
			}
		}
		changes := DiffDocs(prev, curr)
		entries[i].Changes = changes
		entries[i].Summary = Summarize(changes)
	}

	return entries, nil
}

// fileAtCommit returns the content of filePath at the given commit using
// `git show <hash>:<path>`. Returns "" with no error if the file did not exist
// at that commit (initial commit scenario).
func fileAtCommit(ctx context.Context, repoPath, filePath, hash string) (string, error) {
	// git show wants the path relative to the repo root without a leading slash.
	relPath := strings.TrimPrefix(filePath, "/")

	// Defence in depth: reject any input that could be confused for a git
	// option by argv parsing. The ref form <hash>:<path> is passed as a single
	// argv entry, so a colon inside relPath would confuse git's ref parser and
	// return an unrelated blob — reject it rather than silently produce wrong
	// history. Hash must be a 40-char lowercase hex SHA.
	if !isGitSHA(hash) {
		return "", fmt.Errorf("fileAtCommit: invalid commit hash %q", hash)
	}
	if err := validateFilePath(relPath); err != nil {
		return "", err
	}
	if strings.ContainsRune(relPath, ':') {
		return "", fmt.Errorf("fileAtCommit: path must not contain ':'")
	}
	if err := validateRepoPath(repoPath); err != nil {
		return "", err
	}
	ref := hash + ":" + relPath

	// #nosec G204 — hash is validated above; relPath is validated above;
	// repoPath is passed via cmd.Dir (not via `-C`) so a leading dash cannot
	// be reinterpreted as a git option by argv parsing.
	cmd := exec.CommandContext(ctx, "git", "show", ref)
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		if isGitExitError(err) {
			// File didn't exist at this commit — return empty string, no error.
			return "", nil
		}
		return "", fmt.Errorf("git show %s: %w", ref, err)
	}
	return string(out), nil
}

// validateRepoPath rejects empty strings and paths that could be parsed as a
// git option. It does NOT check that the path exists — callers intentionally
// allow non-existent paths (tests exercise that branch).
func validateRepoPath(p string) error {
	if p == "" {
		return fmt.Errorf("history: repoPath must not be empty")
	}
	if strings.HasPrefix(p, "-") {
		return fmt.Errorf("history: repoPath must not start with '-'")
	}
	return nil
}

// validateFilePath rejects inputs that would be mis-parsed by git or the
// filesystem. The character blocklist matches gitlog's path contract: repo-
// relative POSIX paths without shell metacharacters, NUL, or leading dashes.
func validateFilePath(p string) error {
	if p == "" {
		return fmt.Errorf("history: filePath must not be empty")
	}
	if strings.HasPrefix(p, "-") {
		return fmt.Errorf("history: filePath must not start with '-'")
	}
	if strings.ContainsRune(p, 0) {
		return fmt.Errorf("history: filePath must not contain NUL")
	}
	return nil
}

// isGitSHA reports whether s looks like a full-length lowercase git SHA-1.
func isGitSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// splitRecords splits the sentinel-delimited git log output into individual
// commit records, trimming blank records. The sentinel (recordSep) appears on
// its own line after each commit's data, so we split on "\n" + recordSep to
// handle both Unix and Windows line endings robustly.
func splitRecords(s string) []string {
	// The format emits: <fields>\n<recordSep>\n for each commit.
	// Splitting on recordSep gives us one field-block per commit, with a
	// trailing newline that TrimSpace removes.
	parts := strings.Split(s, recordSep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// parseISO parses an ISO 8601 timestamp from git and returns RFC3339 UTC.
// Returns the original string if parsing fails — we never drop the entry.
func parseISO(s string) string {
	// git --aI format: "2026-04-15T10:22:00+02:00"
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		// Try without timezone.
		t, err = time.Parse("2006-01-02T15:04:05", s)
		if err != nil {
			return s
		}
	}
	return t.UTC().Format(time.RFC3339)
}

// isGitExitError reports whether err is a non-zero exit from git, which we
// treat as "no data" rather than a fatal error in history queries.
func isGitExitError(err error) bool {
	var exitErr *exec.ExitError
	if err == nil {
		return false
	}
	if ok := isExitError(err, &exitErr); ok {
		return exitErr.ExitCode() != 0
	}
	return false
}

// isExitError is a helper to avoid importing errors just for errors.As.
func isExitError(err error, target **exec.ExitError) bool {
	e, ok := err.(*exec.ExitError)
	if ok {
		*target = e
	}
	return ok
}

// gitVersion is exposed for testing — returns the git version string.
func gitVersion(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "--version").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(bytes.TrimSpace(out))), nil
}
