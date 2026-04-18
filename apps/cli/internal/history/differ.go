// Package history provides prose-level diff and git-backed history for Vedox
// documentation files.
//
// The package is intentionally pure heuristics — no LLM, no cloud calls. The
// diff operates at the Markdown structural unit (heading, paragraph, code fence,
// list item, frontmatter block) rather than at the line level. This matches
// how authors think about their changes ("I added a section on Setup") and
// avoids the cosmetic noise of line-rewrap diffs.
//
// CTO ruling (v2): pure heuristics only. Local-LLM opt-in is v2.1.
package history

import (
	"fmt"
	"strings"
)

// ChangeType classifies what happened to a block between two document versions.
type ChangeType string

const (
	// ChangeAdded means the block is present in newContent but not in oldContent.
	ChangeAdded ChangeType = "added"
	// ChangeRemoved means the block was in oldContent but is absent in newContent.
	ChangeRemoved ChangeType = "removed"
	// ChangeModified means the block exists in both versions but its content changed.
	ChangeModified ChangeType = "modified"
	// ChangeMoved means the block content is identical in both versions but appears
	// at a different structural position (different parent heading or rank order).
	ChangeMoved ChangeType = "moved"
)

// BlockKind classifies the structural role of a Markdown block.
type BlockKind string

const (
	BlockFrontmatter BlockKind = "frontmatter"
	BlockHeading     BlockKind = "heading"
	BlockParagraph   BlockKind = "paragraph"
	BlockCodeFence   BlockKind = "code_fence"
	BlockListItem    BlockKind = "list_item"
	BlockUnknown     BlockKind = "unknown"
)

// Block is a single structural unit extracted from a Markdown document.
type Block struct {
	// Kind classifies the block type.
	Kind BlockKind
	// Level is the heading level (1–6) for BlockHeading, 0 for all other kinds.
	Level int
	// Raw is the exact source text of the block (trimmed, no surrounding blank lines).
	Raw string
	// Section is the nearest enclosing heading text (empty at document start).
	Section string
	// Lang is the language hint for BlockCodeFence (e.g. "go", "hcl"), empty otherwise.
	Lang string
}

// Change describes a single structural difference between two document versions.
type Change struct {
	// Type classifies the nature of the change.
	Type ChangeType
	// Kind is the block type that changed.
	Kind BlockKind
	// Section is the nearest enclosing heading at the point of change.
	Section string
	// OldContent is the original block text (empty for ChangeAdded).
	OldContent string
	// NewContent is the new block text (empty for ChangeRemoved).
	NewContent string
	// Summary is a short auto-generated human-readable description of this change,
	// e.g. "Added paragraph under '## Setup'".
	Summary string
}

// DiffDocs computes a structural diff between oldContent and newContent.
// It splits each document into blocks (headings, paragraphs, code fences, list
// items, frontmatter), then applies a Myers sequence diff at the block level.
// The returned slice is ordered by position in the new document (adds/modifies)
// or position in the old document (removes).
func DiffDocs(oldContent, newContent string) []Change {
	oldBlocks := splitBlocks(oldContent)
	newBlocks := splitBlocks(newContent)
	return myersDiff(oldBlocks, newBlocks)
}

// splitBlocks parses a Markdown document into its structural blocks.
// The algorithm is a single linear pass — no full AST. This keeps the
// dependency surface to zero and is sufficient for structural-level diffing.
//
// Frontmatter (--- delimited YAML at document start) is emitted as a single
// BlockFrontmatter. ATX headings (# … ######) become BlockHeading. Fenced code
// blocks (``` or ~~~) become BlockCodeFence. Lines starting with a list marker
// (-, *, +, or N.) become BlockListItem. Everything else is grouped into
// BlockParagraph runs separated by blank lines.
func splitBlocks(content string) []Block {
	lines := strings.Split(content, "\n")
	var blocks []Block
	currentSection := ""

	i := 0
	n := len(lines)

	// --- Frontmatter ---
	if i < n && strings.TrimSpace(lines[i]) == "---" {
		j := i + 1
		for j < n && strings.TrimSpace(lines[j]) != "---" {
			j++
		}
		if j < n {
			raw := strings.Join(lines[i:j+1], "\n")
			blocks = append(blocks, Block{
				Kind:    BlockFrontmatter,
				Raw:     raw,
				Section: "",
			})
			i = j + 1
		}
	}

	// --- Body ---
	for i < n {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Skip blank lines between blocks.
		if trimmed == "" {
			i++
			continue
		}

		// ATX heading: one or more # chars followed by a space.
		if level, rest, ok := parseHeading(trimmed); ok {
			currentSection = strings.TrimSpace(rest)
			blocks = append(blocks, Block{
				Kind:    BlockHeading,
				Level:   level,
				Raw:     trimmed,
				Section: currentSection,
			})
			i++
			continue
		}

		// Fenced code block: ``` or ~~~
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fence := trimmed[:3]
			lang := strings.TrimSpace(trimmed[3:])
			j := i + 1
			for j < n && !strings.HasPrefix(strings.TrimSpace(lines[j]), fence) {
				j++
			}
			// Include the closing fence line.
			if j < n {
				j++
			}
			raw := strings.Join(lines[i:j], "\n")
			blocks = append(blocks, Block{
				Kind:    BlockCodeFence,
				Raw:     raw,
				Section: currentSection,
				Lang:    lang,
			})
			i = j
			continue
		}

		// List item: leading -, *, +, or N. (ordered list).
		if isListMarker(trimmed) {
			// Collect continuation lines (indented or same-level list items).
			j := i + 1
			for j < n {
				t := strings.TrimSpace(lines[j])
				if t == "" {
					break
				}
				if isListMarker(t) || strings.HasPrefix(lines[j], " ") || strings.HasPrefix(lines[j], "\t") {
					j++
					continue
				}
				break
			}
			raw := strings.Join(lines[i:j], "\n")
			blocks = append(blocks, Block{
				Kind:    BlockListItem,
				Raw:     strings.TrimSpace(raw),
				Section: currentSection,
			})
			i = j
			continue
		}

		// Paragraph: run of non-blank, non-heading, non-fence, non-list lines.
		j := i + 1
		for j < n {
			t := strings.TrimSpace(lines[j])
			if t == "" {
				break
			}
			if _, _, ok := parseHeading(t); ok {
				break
			}
			if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
				break
			}
			if isListMarker(t) {
				break
			}
			j++
		}
		raw := strings.TrimSpace(strings.Join(lines[i:j], "\n"))
		blocks = append(blocks, Block{
			Kind:    BlockParagraph,
			Raw:     raw,
			Section: currentSection,
		})
		i = j
	}

	return blocks
}

// parseHeading returns (level, restOfLine, true) for an ATX heading, or
// (0, "", false) for non-headings.
func parseHeading(s string) (int, string, bool) {
	if !strings.HasPrefix(s, "#") {
		return 0, "", false
	}
	level := 0
	for level < len(s) && s[level] == '#' {
		level++
	}
	if level > 6 {
		return 0, "", false
	}
	// Must be followed by a space (ATX heading rule) or end of line.
	if level < len(s) && s[level] != ' ' {
		return 0, "", false
	}
	rest := ""
	if level < len(s) {
		rest = s[level+1:]
	}
	return level, rest, true
}

// isListMarker reports whether s begins with a Markdown list marker.
func isListMarker(s string) bool {
	if len(s) == 0 {
		return false
	}
	// Unordered: -, *, + followed by space.
	if (s[0] == '-' || s[0] == '*' || s[0] == '+') && len(s) > 1 && s[1] == ' ' {
		return true
	}
	// Ordered: one or more digits followed by . and space.
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i > 0 && i < len(s) && s[i] == '.' && i+1 < len(s) && s[i+1] == ' '
}

// ── Myers diff at the block level ────────────────────────────────────────────

// editOp is a single edit operation from the Myers diff algorithm.
type editOp struct {
	isDelete bool // true = delete from old; false = insert from new
	oldI     int  // index into oldBlocks (valid when isDelete=true)
	newI     int  // index into newBlocks (valid when isDelete=false)
}

// myersDiff applies the Myers diff algorithm to two slices of Block, comparing
// blocks by their Raw text. It returns a []Change describing the structural
// differences between the two versions.
//
// Implementation:
//   - O(ND) forward pass: for each edit distance d, compute the furthest-reaching
//     endpoint on each diagonal k = x - y. A "snake" extends the endpoint along
//     equal blocks. We snapshot v after every step d so the back-trace can
//     reconstruct the shortest edit script.
//   - Back-trace: working from (n,m) backward to (0,0), at each step d we look
//     at the snapshot from step d-1 to determine whether the last non-diagonal
//     move was a delete (right) or insert (down), record that op, then skip the
//     snake to find the prior position.
//
// Equal (matching) blocks produce no Change. Adjacent delete+insert pairs on
// the same kind+section are folded into ChangeModified. Blocks whose Raw text
// appears on both sides but at different positions are flagged as ChangeMoved.
func myersDiff(oldBlocks, newBlocks []Block) []Change {
	n := len(oldBlocks)
	m := len(newBlocks)

	// Fast paths.
	if n == 0 && m == 0 {
		return nil
	}
	if n == 0 {
		return insertsOnly(newBlocks, oldBlocks)
	}
	if m == 0 {
		return deletesOnly(oldBlocks, newBlocks)
	}

	max := n + m
	// v[k + max] = furthest-reaching x (old index) on diagonal k at the end of
	// the current step. Diagonals range from -max to +max; the +max offset keeps
	// all indices non-negative.
	v := make([]int, 2*max+2)

	// trace[d] is a copy of v taken AFTER step d completes.
	// The back-trace reads trace[d-1] to find the v state before step d.
	trace := make([][]int, max+1)

	var finalD int
	found := false

	for d := 0; d <= max; d++ {
		for k := -d; k <= d; k += 2 {
			kOff := k + max
			var x int
			if k == -d || (k != d && v[kOff-1] < v[kOff+1]) {
				x = v[kOff+1] // move down (insert from new)
			} else {
				x = v[kOff-1] + 1 // move right (delete from old)
			}
			y := x - k
			// Extend the snake: skip equal blocks.
			for x < n && y < m && oldBlocks[x].Raw == newBlocks[y].Raw {
				x++
				y++
			}
			v[kOff] = x
			if x >= n && y >= m {
				found = true
				finalD = d
				break
			}
		}
		// Snapshot AFTER this step — back-trace needs the post-step state.
		snap := make([]int, len(v))
		copy(snap, v)
		trace[d] = snap
		if found {
			break
		}
	}

	// Back-trace: reconstruct the edit ops (deletes and inserts only; equal
	// blocks are implicit and not recorded).
	ops := make([]editOp, 0, finalD)
	x, y := n, m
	for d := finalD; d > 0; d-- {
		// trace[d-1] is the v state after step d-1, i.e. the state the algorithm
		// was in at the start of step d.
		prev := trace[d-1]
		k := x - y
		kOff := k + max

		// Determine which diagonal we arrived from at step d.
		// This mirrors the forward-pass move selection logic exactly.
		var prevK int
		if k == -d || (k != d && prev[kOff-1] < prev[kOff+1]) {
			prevK = k + 1 // arrived via a down move (insert)
		} else {
			prevK = k - 1 // arrived via a right move (delete)
		}

		// prevX, prevY: position on diagonal prevK after step d-1's snake.
		prevX := prev[prevK+max]
		prevY := prevX - prevK

		// Record the single non-diagonal move that followed (prevX, prevY).
		if prevK == k+1 {
			// Down move: insert new[prevY] (then snake ran to x,y).
			ops = append(ops, editOp{isDelete: false, newI: prevY})
		} else {
			// Right move: delete old[prevX] (then snake ran to x,y).
			ops = append(ops, editOp{isDelete: true, oldI: prevX})
		}

		// Step back through the snake: new position is (prevX, prevY).
		x, y = prevX, prevY
	}

	// ops was built in reverse order (most recent first). Reverse to get
	// chronological order aligned with the forward document structure.
	for i, j := 0, len(ops)-1; i < j; i, j = i+1, j-1 {
		ops[i], ops[j] = ops[j], ops[i]
	}

	return opsToChanges(ops, oldBlocks, newBlocks)
}

// insertsOnly builds a Change list when oldBlocks is empty — everything is an add.
func insertsOnly(newBlocks, oldBlocks []Block) []Change {
	changes := make([]Change, 0, len(newBlocks))
	for _, nb := range newBlocks {
		changes = append(changes, Change{
			Type:       ChangeAdded,
			Kind:       nb.Kind,
			Section:    nb.Section,
			OldContent: "",
			NewContent: nb.Raw,
			Summary:    summariseChange(ChangeAdded, nb.Kind, nb.Section, nb.Lang),
		})
	}
	return changes
}

// deletesOnly builds a Change list when newBlocks is empty — everything is a remove.
func deletesOnly(oldBlocks, newBlocks []Block) []Change {
	changes := make([]Change, 0, len(oldBlocks))
	for _, ob := range oldBlocks {
		changes = append(changes, Change{
			Type:       ChangeRemoved,
			Kind:       ob.Kind,
			Section:    ob.Section,
			OldContent: ob.Raw,
			NewContent: "",
			Summary:    summariseChange(ChangeRemoved, ob.Kind, ob.Section, ob.Lang),
		})
	}
	return changes
}

// opsToChanges converts a flat list of delete/insert ops into semantic Changes.
// Adjacent delete+insert pairs on the same kind+section are folded into
// ChangeModified. Blocks whose raw text exists on both sides are ChangeMoved.
func opsToChanges(ops []editOp, oldBlocks, newBlocks []Block) []Change {
	var changes []Change
	for i := 0; i < len(ops); {
		op := ops[i]
		if op.isDelete {
			old := oldBlocks[op.oldI]
			// Look ahead for an immediately following insert of the same kind+section.
			if i+1 < len(ops) && !ops[i+1].isDelete {
				nb := newBlocks[ops[i+1].newI]
				if old.Kind == nb.Kind && old.Section == nb.Section && old.Kind != BlockHeading {
					changes = append(changes, Change{
						Type:       ChangeModified,
						Kind:       old.Kind,
						Section:    old.Section,
						OldContent: old.Raw,
						NewContent: nb.Raw,
						Summary:    summariseChange(ChangeModified, old.Kind, old.Section, nb.Lang),
					})
					i += 2
					continue
				}
			}
			// Check if the block moved (its raw text appears in newBlocks).
			if blockExistsIn(old.Raw, newBlocks) {
				changes = append(changes, Change{
					Type:       ChangeMoved,
					Kind:       old.Kind,
					Section:    old.Section,
					OldContent: old.Raw,
					NewContent: old.Raw,
					Summary:    summariseChange(ChangeMoved, old.Kind, old.Section, old.Lang),
				})
			} else {
				changes = append(changes, Change{
					Type:       ChangeRemoved,
					Kind:       old.Kind,
					Section:    old.Section,
					OldContent: old.Raw,
					NewContent: "",
					Summary:    summariseChange(ChangeRemoved, old.Kind, old.Section, old.Lang),
				})
			}
			i++
		} else {
			nb := newBlocks[op.newI]
			// Skip inserts for blocks that already existed (move handled on delete side).
			if blockExistsIn(nb.Raw, oldBlocks) {
				i++
				continue
			}
			changes = append(changes, Change{
				Type:       ChangeAdded,
				Kind:       nb.Kind,
				Section:    nb.Section,
				OldContent: "",
				NewContent: nb.Raw,
				Summary:    summariseChange(ChangeAdded, nb.Kind, nb.Section, nb.Lang),
			})
			i++
		}
	}
	return changes
}

// blockExistsIn reports whether a block with the given raw text exists in bs.
func blockExistsIn(raw string, bs []Block) bool {
	for _, b := range bs {
		if b.Raw == raw {
			return true
		}
	}
	return false
}

// summariseChange returns a short human-readable description of a single change.
func summariseChange(ct ChangeType, kind BlockKind, section, lang string) string {
	kindLabel := blockKindLabel(kind, lang)
	sectionSuffix := ""
	if section != "" {
		sectionSuffix = fmt.Sprintf(" under '%s'", section)
	}
	switch ct {
	case ChangeAdded:
		return fmt.Sprintf("Added %s%s", kindLabel, sectionSuffix)
	case ChangeRemoved:
		return fmt.Sprintf("Removed %s%s", kindLabel, sectionSuffix)
	case ChangeModified:
		return fmt.Sprintf("Modified %s%s", kindLabel, sectionSuffix)
	case ChangeMoved:
		return fmt.Sprintf("Moved %s%s", kindLabel, sectionSuffix)
	default:
		return fmt.Sprintf("Changed %s%s", kindLabel, sectionSuffix)
	}
}

// blockKindLabel returns a human-readable label for a block kind.
func blockKindLabel(kind BlockKind, lang string) string {
	switch kind {
	case BlockFrontmatter:
		return "frontmatter"
	case BlockHeading:
		return "heading"
	case BlockParagraph:
		return "paragraph"
	case BlockCodeFence:
		if lang != "" {
			return fmt.Sprintf("%s code block", lang)
		}
		return "code block"
	case BlockListItem:
		return "list"
	default:
		return "block"
	}
}
