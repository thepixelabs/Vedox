/**
 * frontmatter.ts
 *
 * Parse and serialize YAML frontmatter from/to raw Markdown strings.
 * Uses gray-matter for robust parsing that matches the Go backend's
 * goldmark-frontmatter behavior.
 *
 * Security: field values are treated as plain strings; no HTML is injected.
 * The Go backend is authoritative — this utility must produce output the
 * backend parser accepts without modification (Phase 1 interim rule).
 */

import matter from 'gray-matter';
import { z } from 'zod';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface FrontmatterFields {
  title: string;
  slug?: string;
  type: string;
  status: string;
  date: string;
  tags: string[];
  /** Any additional keys present in the source frontmatter are preserved as-is. */
  [key: string]: unknown;
}

export interface ParsedDocument {
  frontmatter: FrontmatterFields;
  body: string;
  /** True if the source had a frontmatter block (even if empty). */
  hasFrontmatter: boolean;
}

// ---------------------------------------------------------------------------
// Zod validation schema
// ---------------------------------------------------------------------------

/**
 * Warn-only schema: all fields are optional so validation never hard-blocks.
 * Used by FrontmatterPanel.svelte to surface field-level warnings.
 */
export const FrontmatterSchema = z.object({
  title: z.string().min(1, 'Title is required').optional(),
  slug: z.string().optional(),
  type: z
    .enum([
      'adr',
      'how-to',
      'runbook',
      'readme',
      'api-reference',
      'explanation',
      'issue',
      'platform',
      'infrastructure',
      'network',
      'logging',
      ''
    ])
    .optional(),
  status: z
    .enum(['draft', 'review', 'published', 'deprecated', 'superseded', ''])
    .optional(),
  date: z
    .string()
    .regex(/^\d{4}-\d{2}-\d{2}$/, 'Date must be YYYY-MM-DD')
    .optional()
    .or(z.literal('')),
  tags: z.array(z.string()).optional()
});

export type FrontmatterValidationResult = {
  success: boolean;
  errors: Partial<Record<keyof FrontmatterFields, string>>;
};

// ---------------------------------------------------------------------------
// Default values
// ---------------------------------------------------------------------------

function defaultFrontmatter(): FrontmatterFields {
  return {
    title: '',
    slug: '',
    type: '',
    status: 'draft',
    date: new Date().toISOString().slice(0, 10),
    tags: []
  };
}

/**
 * Normalize deprecated status aliases in-place (CTO-approved shim).
 * Runs before Zod validation so legacy documents parse successfully.
 *
 *   approved  → published
 *   archived  → deprecated
 *   accepted  → published (except on ADRs, where it remains valid)
 */
function normalizeDeprecatedAliases(data: Record<string, unknown>): void {
  const status = data.status;
  const type = data.type;
  if (status === 'approved') {
    data.status = 'published';
    console.warn('LINT-W-001: status "approved" is deprecated, normalized to "published"');
  } else if (status === 'archived') {
    data.status = 'deprecated';
    console.warn('LINT-W-001: status "archived" is deprecated, normalized to "deprecated"');
  } else if (status === 'accepted' && type !== 'adr') {
    data.status = 'published';
    console.warn('LINT-W-001: status "accepted" is only valid on ADRs, normalized to "published"');
  }
}

// ---------------------------------------------------------------------------
// Parse
// ---------------------------------------------------------------------------

/**
 * Parse a raw Markdown string (which may or may not have YAML frontmatter)
 * into structured fields and the body text.
 *
 * Preserves unknown frontmatter keys so round-trip never drops data.
 */
export function parseDocument(raw: string): ParsedDocument {
  let parsed: matter.GrayMatterFile<string>;
  let hasFrontmatter = false;

  try {
    // gray-matter only sets data if the block exists and is valid YAML.
    const trimmed = raw.trimStart();
    hasFrontmatter = trimmed.startsWith('---');
    parsed = matter(raw);
  } catch {
    // Malformed frontmatter — treat entire content as body.
    return {
      frontmatter: defaultFrontmatter(),
      body: raw,
      hasFrontmatter: false
    };
  }

  // Normalize deprecated status aliases before merging/validating.
  normalizeDeprecatedAliases(parsed.data as Record<string, unknown>);

  const fm: FrontmatterFields = {
    ...defaultFrontmatter(),
    ...parsed.data
  };

  // Normalise tags to always be an array of strings.
  if (!Array.isArray(fm.tags)) {
    fm.tags = fm.tags ? [String(fm.tags)] : [];
  }

  return {
    frontmatter: fm,
    body: parsed.content,
    hasFrontmatter
  };
}

// ---------------------------------------------------------------------------
// Serialize
// ---------------------------------------------------------------------------

/**
 * Serialize structured frontmatter fields and a body string back into a
 * complete Markdown document. Keys are ordered canonically so the output
 * is deterministic — the Go backend produces the same key order.
 *
 * Unknown/extra keys are preserved after the canonical five.
 */
export function serializeDocument(
  frontmatter: FrontmatterFields,
  body: string
): string {
  const { title, slug, type, status, date, tags, ...rest } = frontmatter;

  // Build an ordered object. gray-matter preserves insertion order.
  const data: Record<string, unknown> = {};
  if (title !== undefined) data.title = title;
  if (typeof slug === 'string' && slug !== '') data.slug = slug;
  if (type !== undefined && type !== '') data.type = type;
  if (status !== undefined && status !== '') data.status = status;
  if (date !== undefined && date !== '') data.date = date;
  if (tags !== undefined && tags.length > 0) data.tags = tags;

  // Append extra keys
  for (const [k, v] of Object.entries(rest)) {
    data[k] = v;
  }

  const hasData = Object.keys(data).length > 0;
  if (!hasData) {
    // No frontmatter — return body only, preserving exact whitespace.
    return body;
  }

  // gray-matter stringify: content must start with \n to get the blank line
  // between the closing --- and the body. This matches goldmark behavior.
  const bodyWithLeadingNewline = body.startsWith('\n') ? body : '\n' + body;
  return matter.stringify(bodyWithLeadingNewline, data);
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

export function validateFrontmatter(
  fm: Partial<FrontmatterFields>
): FrontmatterValidationResult {
  const result = FrontmatterSchema.safeParse(fm);
  if (result.success) {
    return { success: true, errors: {} };
  }

  const errors: Partial<Record<keyof FrontmatterFields, string>> = {};
  for (const issue of result.error.issues) {
    const key = issue.path[0] as keyof FrontmatterFields;
    if (key) {
      errors[key] = issue.message;
    }
  }

  return { success: false, errors };
}
