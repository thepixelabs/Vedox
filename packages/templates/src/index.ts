/**
 * @vedox/templates — public API
 *
 * Exports:
 *   validateFrontmatter  — validate raw frontmatter against the schema for a
 *                          given document type; returns ValidationResult (never throws)
 *   ValidationResult     — { valid: boolean; errors: string[]; warnings: string[] }
 *   getTemplate          — return the raw Markdown string for a document type
 *   listTemplates        — return metadata for all registered templates
 *   TemplateInfo         — { type: string; title: string; description: string }
 */

export { validateFrontmatter } from "./schemas.js";
export type { ValidationResult } from "./schemas.js";

export { getTemplate, listTemplates } from "./registry.js";
export type { TemplateInfo } from "./registry.js";
