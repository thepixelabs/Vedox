# @vedox/markdown-core

**Status: Phase 2 placeholder — not yet implemented.**

This package is reserved for the shared Vedox Markdown parser, to be implemented as a Go WASM module in Phase 2.

## Phase 1 interim rule (CTO ruling, 2026-04-07)

The Go backend parser (`apps/cli`) is authoritative in Phase 1. The SvelteKit frontend round-trips Markdown through its own prosemirror-markdown serializer, but on any save the backend re-parses and the backend's AST is what hits disk and SQLite. Any divergence surfaces as a golden-file test failure.

## Phase 2 plan

Evaluate TinyGo WASM build of the Go Markdown parser. Ship as `@vedox/markdown-core` if the gzipped bundle is under 500KB. Otherwise, keep the Phase 1 interim rule permanently.

## Why this directory exists now

The PNPM workspace and Turborepo pipeline reference `packages/*`. Having this directory as a valid (if empty) workspace package prevents CI failures when the pipeline resolves the workspace graph.
