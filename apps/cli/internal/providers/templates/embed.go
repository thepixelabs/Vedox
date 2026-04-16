// Package templates embeds the per-provider agent instruction prose packs into
// the Go binary at compile time. Each .md file is the instruction body for one
// provider adapter. Template variables {{DAEMON_PORT}}, {{HMAC_KEY_ID}}, and
// {{DAEMON_URL}} are substituted at install time by the adapter's Plan/Install
// methods — they are not processed here.
package templates

import _ "embed"

// Claude is the instruction body for the Claude Code MCP subagent.
//
//go:embed claude.md
var Claude string

// Codex is the instruction body for the OpenAI Codex CLI AGENTS.md block.
//
//go:embed codex.md
var Codex string

// Copilot is the instruction body for the GitHub Copilot instructions section.
//
//go:embed copilot.md
var Copilot string

// Gemini is the instruction body for the Google Gemini CLI extension manifest.
//
//go:embed gemini.md
var Gemini string
