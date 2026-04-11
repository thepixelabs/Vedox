/**
 * highlight.ts — Shiki singleton for syntax-highlighted code blocks.
 *
 * This module owns the lifetime of a single Shiki HighlighterCore instance so
 * that:
 *   - Repeated highlight() calls share loaded themes and grammars.
 *   - Initial load happens exactly once per page — language/theme bundles are
 *     loaded lazily on the first call, never on module import.
 *
 * We use @shikijs/engine-javascript (no WASM) to keep the editor bundle
 * snappy and to avoid async WASM instantiation in the live preview path.
 *
 * Default themes:
 *   - github-dark   (used when document element carries data-theme="dark")
 *   - github-light  (everything else)
 *
 * These are referenced by name — CSS variables like --code-bg / --code-border
 * control the frame; Shiki paints the tokens with inline colors from the
 * theme JSON.
 *
 * Preloaded languages (the fifteen most common in a dev docs CMS):
 *   typescript, javascript, go, rust, python, html, css, json, yaml, toml,
 *   bash, sql, markdown, svelte, mermaid
 *
 * Languages outside this set trigger a lazy dynamic import the first time
 * they appear — Shiki handles the grammar fetch; callers just await.
 */

import {
  createHighlighterCore,
  type HighlighterCore,
} from 'shiki/core';
import { createJavaScriptRegexEngine } from 'shiki/engine-javascript.mjs';

// ---------------------------------------------------------------------------
// Theme + language preload sets
// ---------------------------------------------------------------------------

/** Theme ids we keep loaded by default. */
export const DEFAULT_THEMES = ['github-dark', 'github-light'] as const;
export type CodeTheme = (typeof DEFAULT_THEMES)[number];

/**
 * Pre-loaded languages. This list is the minimum set the editor promises to
 * highlight synchronously. Anything else will be loaded on demand the first
 * time highlight() sees it.
 *
 * Order is intentional — the first entries are the ones most doc blocks are
 * written in, so the highlighter finishes warming them first.
 */
export const DEFAULT_LANGUAGES = [
  'typescript',
  'javascript',
  'go',
  'rust',
  'python',
  'html',
  'css',
  'json',
  'yaml',
  'toml',
  'bash',
  'sql',
  'markdown',
  'svelte',
  'mermaid',
] as const;
export type CodeLang = (typeof DEFAULT_LANGUAGES)[number] | string;

// ---------------------------------------------------------------------------
// Singleton bootstrapping
// ---------------------------------------------------------------------------

/**
 * Promise for the in-flight highlighter init, shared across all callers.
 * Kept module-scoped so every concurrent await getHighlighter() resolves to
 * the same instance — no race, no double init.
 */
let highlighterPromise: Promise<HighlighterCore> | null = null;

/** Set of languages we have successfully loaded at runtime (incl. lazy). */
const loadedLanguages = new Set<string>();

/**
 * Create — or return the cached promise for — the shared highlighter.
 *
 * Why a promise and not an instance:
 *   The first call is async (loads theme + language JSON). We want concurrent
 *   callers to queue on the same promise instead of each kicking off their
 *   own init.
 */
export function getHighlighter(): Promise<HighlighterCore> {
  if (highlighterPromise) return highlighterPromise;

  highlighterPromise = (async (): Promise<HighlighterCore> => {
    // Load theme and language JSONs in parallel. Shiki ships these as
    // individual ESM modules so tree-shaking only pulls what we import.
    const [themes, langs] = await Promise.all([
      Promise.all([
        import('shiki/themes/github-dark.mjs').then((m) => m.default),
        import('shiki/themes/github-light.mjs').then((m) => m.default),
      ]),
      Promise.all([
        import('shiki/langs/typescript.mjs').then((m) => m.default),
        import('shiki/langs/javascript.mjs').then((m) => m.default),
        import('shiki/langs/go.mjs').then((m) => m.default),
        import('shiki/langs/rust.mjs').then((m) => m.default),
        import('shiki/langs/python.mjs').then((m) => m.default),
        import('shiki/langs/html.mjs').then((m) => m.default),
        import('shiki/langs/css.mjs').then((m) => m.default),
        import('shiki/langs/json.mjs').then((m) => m.default),
        import('shiki/langs/yaml.mjs').then((m) => m.default),
        import('shiki/langs/toml.mjs').then((m) => m.default),
        import('shiki/langs/bash.mjs').then((m) => m.default),
        import('shiki/langs/sql.mjs').then((m) => m.default),
        import('shiki/langs/markdown.mjs').then((m) => m.default),
        import('shiki/langs/svelte.mjs').then((m) => m.default),
        import('shiki/langs/mermaid.mjs').then((m) => m.default),
      ]),
    ]);

    const h = await createHighlighterCore({
      themes,
      langs,
      // The JS regex engine sidesteps WASM instantiation. Slightly slower than
      // oniguruma on huge files but indistinguishable for typical doc blocks
      // and avoids a ~600KB WASM payload.
      engine: createJavaScriptRegexEngine(),
    });

    // Record what we preloaded so ensureLanguage() can skip them.
    for (const lang of DEFAULT_LANGUAGES) loadedLanguages.add(lang);

    return h;
  })();

  return highlighterPromise;
}

/**
 * Ensure a grammar is loaded before we try to highlight with it.
 *
 * Returns the canonical language id that Shiki recognises (e.g. "ts" -> "ts",
 * unknown/unsupported -> "text"). Callers must pass the returned id to
 * highlight().
 */
async function ensureLanguage(h: HighlighterCore, lang: string): Promise<string> {
  const normalized = normalizeLang(lang);
  if (normalized === 'text') return 'text';

  // Fast path: already loaded.
  if (h.getLoadedLanguages().includes(normalized)) {
    loadedLanguages.add(normalized);
    return normalized;
  }

  // Slow path: attempt a lazy dynamic import. Shiki's /langs/<id>.mjs files
  // mirror the VSCode tmLanguage catalog. If the file doesn't exist we fall
  // back to plaintext rather than throwing — a missing grammar should degrade
  // gracefully, not crash the editor.
  try {
    const mod = (await import(
      /* @vite-ignore */ `shiki/langs/${normalized}.mjs`
    )) as { default: Parameters<HighlighterCore['loadLanguage']>[0] };
    await h.loadLanguage(mod.default);
    loadedLanguages.add(normalized);
    return normalized;
  } catch {
    return 'text';
  }
}

/**
 * Normalize common language aliases to Shiki's canonical ids. This is the
 * single place where we map TS shorthand ("ts", "tsx", "typescriptreact")
 * to the actual grammar name Shiki ships.
 *
 * Anything we don't recognise is passed through as-is; ensureLanguage() will
 * try to lazy-load it and fall back to "text" on failure.
 */
export function normalizeLang(lang: string | null | undefined): string {
  if (!lang) return 'text';
  const l = lang.trim().toLowerCase();
  switch (l) {
    case '':
    case 'plaintext':
    case 'plain':
    case 'text':
    case 'txt':
      return 'text';
    case 'ts':
    case 'typescript':
      return 'typescript';
    case 'tsx':
    case 'typescriptreact':
      return 'tsx';
    case 'js':
    case 'javascript':
      return 'javascript';
    case 'jsx':
    case 'javascriptreact':
      return 'jsx';
    case 'golang':
      return 'go';
    case 'py':
    case 'python':
      return 'python';
    case 'rs':
    case 'rust':
      return 'rust';
    case 'sh':
    case 'shell':
    case 'zsh':
      return 'bash';
    case 'yml':
      return 'yaml';
    case 'md':
      return 'markdown';
    default:
      return l;
  }
}

// ---------------------------------------------------------------------------
// Public highlight() entry point
// ---------------------------------------------------------------------------

export interface HighlightOptions {
  /** Either 'github-dark' or 'github-light' (default: 'github-dark'). */
  theme?: CodeTheme;
}

/**
 * Highlight a snippet of code and return the Shiki-generated HTML.
 *
 * The returned string is a `<pre class="shiki …"><code>…</code></pre>`
 * structure with inline token styles. Callers drop it into a container via
 * `@html` (Svelte) — Shiki's output is authored by us, not user input, so
 * it's safe to render.
 *
 * If the requested language isn't available and can't be lazy-loaded, the
 * block is rendered as plaintext with the default theme's foreground colour
 * — never an error.
 */
export async function highlight(
  code: string,
  lang: string,
  options: HighlightOptions = {},
): Promise<string> {
  const theme = options.theme ?? 'github-dark';
  const h = await getHighlighter();
  const resolvedLang = await ensureLanguage(h, lang);

  // codeToHtml is synchronous once the grammar is loaded.
  return h.codeToHtml(code, {
    lang: resolvedLang,
    theme,
  });
}

/**
 * Eagerly warm the highlighter on idle. Call this from the root layout so the
 * first code block a user scrolls to is already painted — otherwise the first
 * block briefly shows as plaintext while Shiki initialises.
 */
export function warmHighlighter(): void {
  // Fire and forget. Errors are captured by the promise cache.
  getHighlighter().catch((err) => {
    // Log once, don't spam. The UI will just render plaintext if init fails.
    // eslint-disable-next-line no-console
    console.warn('[vedox] Shiki init failed, falling back to plaintext:', err);
  });
}
