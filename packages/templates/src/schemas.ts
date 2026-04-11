/**
 * Frontmatter schema validation for Vedox document templates.
 *
 * Design rules:
 * - Never throws. Always returns a ValidationResult.
 * - Validation failure populates `errors` or `warnings` but never prevents saving.
 *   The caller (UI) decides how to present the result.
 * - `errors` = field missing or type-invalid (meaningful save is impossible without it)
 * - `warnings` = field present but suspicious (empty project, future date, missing author, etc.)
 */

import { z, ZodError } from "zod";

// ---------------------------------------------------------------------------
// Public types
// ---------------------------------------------------------------------------

export interface ValidationResult {
  valid: boolean;
  errors: string[];
  warnings: string[];
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** ISO 8601 date: YYYY-MM-DD */
const isoDate = z
  .string()
  .regex(/^\d{4}-\d{2}-\d{2}$/, "must be a date in YYYY-MM-DD format");

const commonStatus = z.enum(["draft", "published", "deprecated"]);

// ---------------------------------------------------------------------------
// Base schema — fields shared by all document types
// ---------------------------------------------------------------------------

const baseSchema = z.object({
  title: z.string().min(1, "title must not be empty"),
  type: z.enum(["adr", "api-reference", "runbook", "readme", "how-to"], {
    errorMap: () => ({
      message:
        'type must be one of: adr, api-reference, runbook, readme, how-to',
    }),
  }),
  date: isoDate,
  // project is required but may be an empty string — validated with a
  // warning rather than an error (a new doc may not yet be assigned)
  project: z.string(),
  tags: z.array(z.string()).optional(),
  author: z.string().optional(),
});

// ---------------------------------------------------------------------------
// Per-type schemas (discriminated on `type`)
// ---------------------------------------------------------------------------

const adrSchema = baseSchema.extend({
  type: z.literal("adr"),
  status: z.enum(["proposed", "accepted", "deprecated", "superseded"], {
    errorMap: () => ({
      message:
        "status for adr must be one of: proposed, accepted, deprecated, superseded",
    }),
  }),
  superseded_by: z.string().optional(),
});

const apiReferenceSchema = baseSchema.extend({
  type: z.literal("api-reference"),
  status: commonStatus,
  version: z.string().min(1, "version must not be empty"),
});

const runbookSchema = baseSchema.extend({
  type: z.literal("runbook"),
  status: commonStatus,
  on_call_severity: z.enum(["P1", "P2", "P3"], {
    errorMap: () => ({
      message: "on_call_severity must be one of: P1, P2, P3",
    }),
  }),
  last_tested: isoDate.describe("last_tested"),
});

const readmeSchema = baseSchema.extend({
  type: z.literal("readme"),
  status: commonStatus,
});

const howToSchema = baseSchema.extend({
  type: z.literal("how-to"),
  status: commonStatus,
});

// ---------------------------------------------------------------------------
// Schema map — keyed by document type string
// ---------------------------------------------------------------------------

const schemaByType: Record<string, z.ZodTypeAny> = {
  adr: adrSchema,
  "api-reference": apiReferenceSchema,
  runbook: runbookSchema,
  readme: readmeSchema,
  "how-to": howToSchema,
};

// ---------------------------------------------------------------------------
// Warning checks (run after successful schema parse)
// ---------------------------------------------------------------------------

function collectWarnings(data: Record<string, unknown>, type: string): string[] {
  const warnings: string[] = [];

  // Empty project is allowed but suspicious
  if (data["project"] === "") {
    warnings.push("project is empty — consider assigning this document to a project");
  }

  // Missing author is not an error but worth flagging
  if (!data["author"] || (data["author"] as string).trim() === "") {
    warnings.push("author is not set");
  }

  // Future date
  if (typeof data["date"] === "string") {
    const docDate = new Date(data["date"] as string);
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    if (docDate > today) {
      warnings.push(`date ${data["date"] as string} is in the future`);
    }
  }

  // ADR: status === "superseded" but superseded_by is absent
  if (
    type === "adr" &&
    data["status"] === "superseded" &&
    (!data["superseded_by"] || (data["superseded_by"] as string).trim() === "")
  ) {
    warnings.push(
      'status is "superseded" but superseded_by is not set — add the ADR number that supersedes this one'
    );
  }

  // Runbook: last_tested placeholder still present
  if (type === "runbook" && data["last_tested"] === "YYYY-MM-DD") {
    warnings.push(
      "last_tested is still the placeholder value — update it after drilling this runbook"
    );
  }

  return warnings;
}

// ---------------------------------------------------------------------------
// Format Zod errors into readable strings
// ---------------------------------------------------------------------------

function formatZodErrors(err: ZodError): string[] {
  return err.issues.map((issue) => {
    const path = issue.path.length > 0 ? issue.path.join(".") + ": " : "";
    return `${path}${issue.message}`;
  });
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

/**
 * Validate raw frontmatter against the schema for the given document type.
 *
 * @param raw   The raw frontmatter object (parsed from YAML, type unknown).
 * @param type  The document type string (e.g. "adr", "runbook"). If the type
 *              is unrecognised, a base-schema validation is performed and an
 *              error is included in the result.
 * @returns     A ValidationResult — never throws.
 */
export function validateFrontmatter(
  raw: unknown,
  type: string
): ValidationResult {
  const schema = schemaByType[type];

  if (!schema) {
    return {
      valid: false,
      errors: [
        `unknown document type "${type}" — must be one of: ${Object.keys(schemaByType).join(", ")}`,
      ],
      warnings: [],
    };
  }

  const result = schema.safeParse(raw);

  if (!result.success) {
    return {
      valid: false,
      errors: formatZodErrors(result.error),
      warnings: [],
    };
  }

  // Schema passed — run warning checks on the validated data
  const warnings = collectWarnings(
    result.data as Record<string, unknown>,
    type
  );

  return {
    valid: true,
    errors: [],
    warnings,
  };
}
