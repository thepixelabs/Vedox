---
title: "Vedox Documentation"
type: explanation
status: published
date: 2026-04-07
project: "vedox"
tags: ["docs", "index", "onboarding", "framework", "governance"]
author: "Vedox CEO"
audience: "all-writers-human-and-agent"
summary: "Entry point for Vedox documentation. Required reading for any human or AI agent that intends to write content into this tree."
---

# Vedox Documentation

This directory holds all of Vedox's own documentation. Vedox is dogfooded — every page in `docs/` is authored, edited, and served by Vedox itself.

## Required reading (in this order)

Anyone — human or AI — who intends to write content into this tree MUST read the following two documents end to end before writing a single line. These are not suggestions. A submission from a writer who has not read them will be rejected in review.

1. [WRITING_FRAMEWORK.md](./WRITING_FRAMEWORK.md) — the content / editorial / schema / governance contract. Defines content types, frontmatter schemas, naming, lifecycle, the agent contract, and the linter rule inventory.
2. [DESIGN_FRAMEWORK.md](./DESIGN_FRAMEWORK.md) — the visual / IA / component / accessibility / editor-UX contract. Maintained in parallel by the creative-technologist role.

Reading order matters: WRITING first, DESIGN second. WRITING decides what a document IS; DESIGN decides what it LOOKS LIKE.

## Directory layout

| Directory | Purpose | Type(s) |
|---|---|---|
| `adr/` | Architecture decision records | `adr` |
| `how-to/` | Task-oriented procedural guides | `how-to` |
| `runbooks/` | Incident response procedures | `runbook` |
| `api-reference/` | HTTP, CLI, MCP, SDK references | `api-reference` |
| `explanation/` | Conceptual background documents | `explanation` |
| `issues/` | Bug reports, feature requests, postmortems | `issue` |
| `platform/` | Product / feature documentation | `platform` |
| `infrastructure/` | Deployment, environments, IaC | `infrastructure` |
| `network/` | Network, ports, protocols, security boundaries | `network` |
| `logging/` | Log format and observability docs | `logging` |

The ten content types are defined exhaustively in [WRITING_FRAMEWORK.md Section 4](./WRITING_FRAMEWORK.md#4-content-types). There are no others.

## Starting points

- New to Vedox? Read [ADR-001: Markdown as Source of Truth](./adr/001-markdown-as-source-of-truth.md) first.
- Writing a how-to? Start at [How to Write a How-To](./how-to/use-how-to-template.md) after the framework.
- Writing an ADR? Start at [How to Write an ADR](./how-to/use-adr-template.md) after the framework.
- Writing a runbook? Start at [How to Write a Runbook](./how-to/use-runbook-template.md) after the framework.
- Writing an API reference? Start at [How to Write an API Reference](./how-to/use-api-reference-template.md) after the framework.
- Writing a README? Start at [How to Write a README](./how-to/use-readme-template.md) after the framework.

## Governance

Amendments to either framework go through an ADR (`type: adr`). Silent amendments are forbidden. The frameworks are the constitution of Vedox documentation; changing them is an explicit, recorded act.
