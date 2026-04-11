/**
 * vedox.config.ts — Workspace configuration for Vedox.
 *
 * Place this file at the root of your documentation workspace.
 * It is loaded at runtime by `vedox dev` and `vedox build`.
 *
 * All fields are optional and fall back to documented defaults.
 *
 * This file is pure TypeScript with no side-effects at import time.
 * The Vedox CLI imports it via ts-node / esbuild in-process; do not
 * place startup code, network calls, or async work at the top level.
 */

// ---------------------------------------------------------------------------
// Types — exported so user configs can reference them with import type.
// ---------------------------------------------------------------------------

/** Port binding configuration. */
export interface PortConfig {
  /**
   * Port for the development server.
   * @default 3001
   */
  dev?: number;

  /**
   * Port for the production server (used by `vedox serve` if ever added).
   * @default 3000
   */
  prod?: number;
}

/** Logging configuration. */
export interface LogConfig {
  /**
   * Directory where structured JSON logs are written.
   * Supports ~ expansion.
   * @default "~/.vedox/logs"
   */
  dir?: string;

  /**
   * Log level. One of: "debug" | "info" | "warn" | "error"
   * @default "info"
   */
  level?: "debug" | "info" | "warn" | "error";

  /**
   * Retention period in days. Logs older than this are deleted on startup.
   * @default 7
   */
  retentionDays?: number;
}

/** Network policy. Vedox binds to loopback by default; zero outbound calls. */
export interface NetworkConfig {
  /**
   * Host to bind the dev server to.
   * Set to "0.0.0.0" only for Docker / team deployments and only with
   * explicit acknowledgement — Vedox will print a startup warning.
   * @default "127.0.0.1"
   */
  host?: string;

  /**
   * Allow outbound network calls (e.g. future opt-in version check).
   * Vedox makes zero outbound calls by default. This must be explicitly
   * set to true to enable any future network feature.
   * @default false
   */
  allowOutbound?: boolean;
}

/** Git integration settings. */
export interface GitConfig {
  /**
   * Default commit message template for "Publish" actions.
   * The token {title} is replaced with the document title.
   * @default "docs: update {title}"
   */
  commitMessageTemplate?: string;

  /**
   * Whether to GPG-sign commits. Requires a configured signing key.
   * Not supported in Phase 1 — will be silently ignored until implemented.
   * @default false
   */
  signCommits?: boolean;
}

/** Top-level Vedox workspace configuration schema. */
export interface VedoxConfig {
  /**
   * Human-readable name for this workspace, used in the editor UI header.
   * @default The basename of the workspace directory.
   */
  name?: string;

  /**
   * Glob patterns (relative to workspace root) for Markdown files to include.
   * @default ["**\/*.md", "**\/*.mdx"]
   */
  include?: string[];

  /**
   * Glob patterns to exclude from indexing and the editor.
   * @default ["node_modules/**", ".git/**", ".vedox/**", "dist/**"]
   */
  exclude?: string[];

  /** Port binding settings. */
  ports?: PortConfig;

  /** Logging settings. */
  log?: LogConfig;

  /** Network policy. Vedox is loopback-only by default. */
  network?: NetworkConfig;

  /** Git integration settings. */
  git?: GitConfig;

  /**
   * Canonical project allowlist. Any document whose frontmatter `project:`
   * field is not in this list is rejected by the linter. This is the single
   * source of truth — do not duplicate it in WRITING_FRAMEWORK.md or in the
   * linter config.
   */
  projects?: string[];
}

// ---------------------------------------------------------------------------
// Example configuration — edit and uncomment as needed.
// ---------------------------------------------------------------------------

const config: VedoxConfig = {
  name: "Vedox Docs",

  include: ["**/*.md"],

  exclude: [
    "node_modules/**",
    ".git/**",
    ".vedox/**",
    "dist/**",
    "build/**",
  ],

  ports: {
    dev: 3001,
    // prod: 3000,
  },

  log: {
    // dir: "~/.vedox/logs",
    level: "info",
    retentionDays: 7,
  },

  network: {
    // host: "127.0.0.1",       // Change to "0.0.0.0" for Docker deployments
    //                           // only — Vedox will display a startup warning.
    allowOutbound: false,       // Zero outbound calls by default.
  },

  git: {
    commitMessageTemplate: "docs: update {title}",
    // signCommits: false,
  },

  // Canonical project allowlist. See WRITING_FRAMEWORK.md §5.
  // The linter rejects any document with `project:` not in this list.
  projects: ["vedox"],
};

export default config;
