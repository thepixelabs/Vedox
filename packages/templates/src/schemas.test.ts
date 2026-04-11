/**
 * Unit tests for validateFrontmatter.
 *
 * Mirrors the acceptance criteria in VDX-P1-007:
 * - Valid ADR frontmatter → valid: true, no errors
 * - Missing `title` → valid: false, errors contains message about title
 * - Runbook missing on_call_severity → valid: false, errors contain message
 * - project: "" → valid: true (or false from other fields), warnings include project warning
 * - Unknown type → valid: false
 */

import { describe, expect, it } from "vitest";
import { validateFrontmatter } from "./schemas.js";

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const validAdr = {
  title: "ADR-001: Use Go for the CLI backend",
  type: "adr",
  status: "proposed",
  date: "2026-04-07",
  project: "vedox",
  tags: ["backend", "go"],
  author: "alice",
};

const validRunbook = {
  title: "Vedox not loading workspace",
  type: "runbook",
  status: "published",
  date: "2026-04-07",
  project: "vedox",
  on_call_severity: "P1",
  last_tested: "2026-04-07",
  tags: [],
  author: "bob",
};

const validApiRef = {
  title: "Documents API",
  type: "api-reference",
  status: "published",
  date: "2026-04-07",
  project: "vedox",
  version: "v1",
  tags: [],
  author: "carol",
};

const validReadme = {
  title: "Vedox",
  type: "readme",
  status: "published",
  date: "2026-04-07",
  project: "vedox",
  tags: [],
  author: "dave",
};

const validHowTo = {
  title: "How to add a project",
  type: "how-to",
  status: "published",
  date: "2026-04-07",
  project: "vedox",
  tags: [],
  author: "eve",
};

// ---------------------------------------------------------------------------
// ADR
// ---------------------------------------------------------------------------

describe("validateFrontmatter — adr", () => {
  it("accepts a fully valid ADR", () => {
    const result = validateFrontmatter(validAdr, "adr");
    expect(result.valid).toBe(true);
    expect(result.errors).toHaveLength(0);
  });

  it("errors when title is missing", () => {
    const { title: _title, ...rest } = validAdr;
    const result = validateFrontmatter(rest, "adr");
    expect(result.valid).toBe(false);
    expect(result.errors.some((e) => e.includes("title"))).toBe(true);
  });

  it("errors when title is empty string", () => {
    const result = validateFrontmatter({ ...validAdr, title: "" }, "adr");
    expect(result.valid).toBe(false);
    expect(result.errors.some((e) => e.includes("title"))).toBe(true);
  });

  it("errors on invalid status value", () => {
    const result = validateFrontmatter(
      { ...validAdr, status: "published" },
      "adr"
    );
    expect(result.valid).toBe(false);
    expect(result.errors.some((e) => e.includes("status"))).toBe(true);
  });

  it("accepts all valid adr status values", () => {
    for (const status of ["proposed", "accepted", "deprecated", "superseded"]) {
      const result = validateFrontmatter(
        { ...validAdr, status, superseded_by: status === "superseded" ? "ADR-002" : "" },
        "adr"
      );
      expect(result.valid).toBe(true);
    }
  });

  it("errors on malformed date", () => {
    const result = validateFrontmatter(
      { ...validAdr, date: "07-04-2026" },
      "adr"
    );
    expect(result.valid).toBe(false);
    expect(result.errors.some((e) => e.includes("YYYY-MM-DD"))).toBe(true);
  });

  it("warns when project is empty string", () => {
    const result = validateFrontmatter(
      { ...validAdr, project: "" },
      "adr"
    );
    // Empty project → still valid, but a warning is present
    expect(result.valid).toBe(true);
    expect(result.warnings.some((w) => w.includes("project"))).toBe(true);
  });

  it("warns when author is absent", () => {
    const { author: _a, ...rest } = validAdr;
    const result = validateFrontmatter(rest, "adr");
    expect(result.valid).toBe(true);
    expect(result.warnings.some((w) => w.includes("author"))).toBe(true);
  });

  it("warns when status is superseded but superseded_by is absent", () => {
    const result = validateFrontmatter(
      { ...validAdr, status: "superseded", superseded_by: "" },
      "adr"
    );
    expect(result.valid).toBe(true);
    expect(
      result.warnings.some((w) => w.includes("superseded_by"))
    ).toBe(true);
  });

  it("does not warn about superseded_by when status is not superseded", () => {
    const result = validateFrontmatter(
      { ...validAdr, status: "accepted", superseded_by: "" },
      "adr"
    );
    expect(result.valid).toBe(true);
    expect(
      result.warnings.some((w) => w.includes("superseded_by"))
    ).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// Runbook
// ---------------------------------------------------------------------------

describe("validateFrontmatter — runbook", () => {
  it("accepts a fully valid runbook", () => {
    const result = validateFrontmatter(validRunbook, "runbook");
    expect(result.valid).toBe(true);
    expect(result.errors).toHaveLength(0);
  });

  it("errors when on_call_severity is missing", () => {
    const { on_call_severity: _s, ...rest } = validRunbook;
    const result = validateFrontmatter(rest, "runbook");
    expect(result.valid).toBe(false);
    expect(result.errors.some((e) => e.includes("on_call_severity"))).toBe(
      true
    );
  });

  it("errors when on_call_severity is an invalid value", () => {
    const result = validateFrontmatter(
      { ...validRunbook, on_call_severity: "P4" },
      "runbook"
    );
    expect(result.valid).toBe(false);
    expect(result.errors.some((e) => e.includes("on_call_severity"))).toBe(
      true
    );
  });

  it("errors when last_tested is missing", () => {
    const { last_tested: _lt, ...rest } = validRunbook;
    const result = validateFrontmatter(rest, "runbook");
    expect(result.valid).toBe(false);
    expect(result.errors.some((e) => e.includes("last_tested"))).toBe(true);
  });

  it("accepts all valid on_call_severity values", () => {
    for (const sev of ["P1", "P2", "P3"]) {
      const result = validateFrontmatter(
        { ...validRunbook, on_call_severity: sev },
        "runbook"
      );
      expect(result.valid).toBe(true);
    }
  });

  it("warns when last_tested is still the placeholder", () => {
    const result = validateFrontmatter(
      { ...validRunbook, last_tested: "YYYY-MM-DD" },
      "runbook"
    );
    // The placeholder is not a valid ISO date, so this should be an error, not a warning.
    // Verify the error surface rather than the warning.
    expect(result.valid).toBe(false);
    expect(result.errors.some((e) => e.includes("last_tested") || e.includes("YYYY-MM-DD"))).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// API Reference
// ---------------------------------------------------------------------------

describe("validateFrontmatter — api-reference", () => {
  it("accepts a fully valid api-reference", () => {
    const result = validateFrontmatter(validApiRef, "api-reference");
    expect(result.valid).toBe(true);
    expect(result.errors).toHaveLength(0);
  });

  it("errors when version is missing", () => {
    const { version: _v, ...rest } = validApiRef;
    const result = validateFrontmatter(rest, "api-reference");
    expect(result.valid).toBe(false);
    expect(result.errors.some((e) => e.includes("version"))).toBe(true);
  });

  it("errors when version is empty string", () => {
    const result = validateFrontmatter(
      { ...validApiRef, version: "" },
      "api-reference"
    );
    expect(result.valid).toBe(false);
    expect(result.errors.some((e) => e.includes("version"))).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// README
// ---------------------------------------------------------------------------

describe("validateFrontmatter — readme", () => {
  it("accepts a fully valid readme", () => {
    const result = validateFrontmatter(validReadme, "readme");
    expect(result.valid).toBe(true);
    expect(result.errors).toHaveLength(0);
  });
});

// ---------------------------------------------------------------------------
// How-To
// ---------------------------------------------------------------------------

describe("validateFrontmatter — how-to", () => {
  it("accepts a fully valid how-to", () => {
    const result = validateFrontmatter(validHowTo, "how-to");
    expect(result.valid).toBe(true);
    expect(result.errors).toHaveLength(0);
  });
});

// ---------------------------------------------------------------------------
// Unknown type
// ---------------------------------------------------------------------------

describe("validateFrontmatter — unknown type", () => {
  it("errors on an unrecognised type", () => {
    const result = validateFrontmatter(validAdr, "kanban");
    expect(result.valid).toBe(false);
    expect(result.errors.some((e) => e.includes("kanban"))).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// Common warning: future date
// ---------------------------------------------------------------------------

describe("validateFrontmatter — future date warning", () => {
  it("warns when date is in the future", () => {
    const result = validateFrontmatter(
      { ...validAdr, date: "2099-12-31" },
      "adr"
    );
    expect(result.valid).toBe(true);
    expect(result.warnings.some((w) => w.includes("future"))).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// Non-blocking guarantee
// ---------------------------------------------------------------------------

describe("non-blocking guarantee", () => {
  it("never throws — returns ValidationResult for arbitrary input", () => {
    expect(() => validateFrontmatter(null, "adr")).not.toThrow();
    expect(() => validateFrontmatter(undefined, "adr")).not.toThrow();
    expect(() => validateFrontmatter(42, "adr")).not.toThrow();
    expect(() => validateFrontmatter({}, "adr")).not.toThrow();
    expect(() => validateFrontmatter({}, "not-a-type")).not.toThrow();
  });

  it("returns valid: false (not an exception) for null input", () => {
    const result = validateFrontmatter(null, "adr");
    expect(result.valid).toBe(false);
    expect(Array.isArray(result.errors)).toBe(true);
  });
});
