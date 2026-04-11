/**
 * Unit tests for the template registry.
 *
 * Mirrors acceptance criteria in VDX-P1-007:
 * - getTemplate("adr") returns a string containing "## Context"
 * - listTemplates() returns exactly 5 entries
 * - getTemplate with unknown type throws (not silently returns undefined)
 */

import { describe, expect, it } from "vitest";
import { getTemplate, listTemplates } from "./registry.js";

describe("getTemplate", () => {
  it('returns a non-empty string for "adr"', () => {
    const t = getTemplate("adr");
    expect(typeof t).toBe("string");
    expect(t.length).toBeGreaterThan(0);
  });

  it('adr template contains "## Context"', () => {
    expect(getTemplate("adr")).toContain("## Context");
  });

  it('adr template contains "## Decision"', () => {
    expect(getTemplate("adr")).toContain("## Decision");
  });

  it('adr template contains "## Consequences"', () => {
    expect(getTemplate("adr")).toContain("## Consequences");
  });

  it('adr template contains "## Alternatives Considered"', () => {
    expect(getTemplate("adr")).toContain("## Alternatives Considered");
  });

  it('api-reference template contains "## Overview"', () => {
    expect(getTemplate("api-reference")).toContain("## Overview");
  });

  it('api-reference template contains "## Endpoints"', () => {
    expect(getTemplate("api-reference")).toContain("## Endpoints");
  });

  it('api-reference template contains "## Error Codes"', () => {
    expect(getTemplate("api-reference")).toContain("## Error Codes");
  });

  it('runbook template contains "## Symptoms"', () => {
    expect(getTemplate("runbook")).toContain("## Symptoms");
  });

  it('runbook template contains "## Immediate Actions"', () => {
    expect(getTemplate("runbook")).toContain("## Immediate Actions");
  });

  it('runbook template contains "## Root Cause Investigation"', () => {
    expect(getTemplate("runbook")).toContain("## Root Cause Investigation");
  });

  it('runbook template contains "## Resolution Steps"', () => {
    expect(getTemplate("runbook")).toContain("## Resolution Steps");
  });

  it('runbook template contains "## Prevention"', () => {
    expect(getTemplate("runbook")).toContain("## Prevention");
  });

  it('readme template contains "## Installation"', () => {
    expect(getTemplate("readme")).toContain("## Installation");
  });

  it('readme template contains "## Contributing"', () => {
    expect(getTemplate("readme")).toContain("## Contributing");
  });

  it('how-to template contains "## Prerequisites"', () => {
    expect(getTemplate("how-to")).toContain("## Prerequisites");
  });

  it('how-to template contains "## Steps"', () => {
    expect(getTemplate("how-to")).toContain("## Steps");
  });

  it('how-to template contains "## Verification"', () => {
    expect(getTemplate("how-to")).toContain("## Verification");
  });

  it('how-to template contains "## Troubleshooting"', () => {
    expect(getTemplate("how-to")).toContain("## Troubleshooting");
  });

  it("throws for an unknown template type", () => {
    expect(() => getTemplate("kanban")).toThrow(/kanban/);
  });

  it("all templates contain valid YAML frontmatter delimiter", () => {
    const types = ["adr", "api-reference", "runbook", "readme", "how-to"];
    for (const type of types) {
      const content = getTemplate(type);
      expect(content.startsWith("---\n"), `${type} template must start with ---`).toBe(true);
    }
  });
});

describe("listTemplates", () => {
  it("returns exactly 5 templates", () => {
    expect(listTemplates()).toHaveLength(5);
  });

  it("includes all expected types", () => {
    const types = listTemplates().map((t) => t.type);
    expect(types).toContain("adr");
    expect(types).toContain("api-reference");
    expect(types).toContain("runbook");
    expect(types).toContain("readme");
    expect(types).toContain("how-to");
  });

  it("each entry has a non-empty title and description", () => {
    for (const t of listTemplates()) {
      expect(t.title.length, `${t.type} title must not be empty`).toBeGreaterThan(0);
      expect(t.description.length, `${t.type} description must not be empty`).toBeGreaterThan(0);
    }
  });

  it("returns a copy — mutations do not affect the registry", () => {
    const list1 = listTemplates();
    list1[0]!.title = "MUTATED";
    const list2 = listTemplates();
    expect(list2[0]!.title).not.toBe("MUTATED");
  });
});
