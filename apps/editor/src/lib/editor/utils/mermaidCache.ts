/**
 * mermaidCache.ts
 *
 * Hash-keyed SVG cache stored in localStorage for Mermaid diagrams.
 * Prevents re-rendering identical diagrams on every keystroke and
 * survives page reload without a network round-trip.
 *
 * Cache key format: `vedox-mermaid-${djb2Hash}`
 * Cache entries are plain strings (sanitized SVG markup).
 *
 * Security: SVG strings retrieved from this cache are passed through
 * DOMPurify before insertion into the DOM. This module does not touch
 * the DOM directly.
 */

import mermaid from 'mermaid';

// ---------------------------------------------------------------------------
// Mermaid initialisation (idempotent — safe to call multiple times)
// ---------------------------------------------------------------------------

let mermaidInitialised = false;
let mermaidCurrentDark: boolean | null = null;

export function initialiseMermaid(darkMode: boolean): void {
  // Re-initialise when the dark/light mode flips so diagram colours
  // follow the active Vedox theme.
  if (mermaidInitialised && mermaidCurrentDark === darkMode) return;
  mermaid.initialize({
    startOnLoad: false,
    theme: darkMode ? 'dark' : 'default',
    securityLevel: 'strict', // SVG only, no click handlers
    fontFamily: 'inherit'
  });
  mermaidInitialised = true;
  mermaidCurrentDark = darkMode;
}

// ---------------------------------------------------------------------------
// Hash (djb2 — fast, good distribution for short strings)
// ---------------------------------------------------------------------------

function djb2(str: string): number {
  let hash = 5381;
  for (let i = 0; i < str.length; i++) {
    hash = ((hash << 5) + hash) ^ str.charCodeAt(i);
    hash = hash >>> 0; // keep 32-bit unsigned
  }
  return hash;
}

function cacheKey(source: string, darkMode = false): string {
  const mode = darkMode ? 'd' : 'l';
  return `vedox-mermaid-${mode}-${djb2(source)}`;
}

// ---------------------------------------------------------------------------
// Cache read / write
// ---------------------------------------------------------------------------

const CACHE_PREFIX = 'vedox-mermaid-';
const MAX_CACHE_ENTRIES = 50;

function pruneCache(): void {
  try {
    const keys: string[] = [];
    for (let i = 0; i < localStorage.length; i++) {
      const k = localStorage.key(i);
      if (k?.startsWith(CACHE_PREFIX)) keys.push(k);
    }
    // Evict oldest half when over limit (FIFO approximation)
    if (keys.length > MAX_CACHE_ENTRIES) {
      const toDelete = keys.slice(0, keys.length - MAX_CACHE_ENTRIES);
      for (const k of toDelete) localStorage.removeItem(k);
    }
  } catch {
    // localStorage unavailable (SSR, private mode) — ignore silently.
  }
}

function getCached(source: string, darkMode: boolean): string | null {
  try {
    return localStorage.getItem(cacheKey(source, darkMode));
  } catch {
    return null;
  }
}

function setCached(source: string, darkMode: boolean, svg: string): void {
  try {
    pruneCache();
    localStorage.setItem(cacheKey(source, darkMode), svg);
  } catch {
    // Quota exceeded or unavailable — best effort.
  }
}

// ---------------------------------------------------------------------------
// Render
// ---------------------------------------------------------------------------

let renderCounter = 0;

/**
 * Render a Mermaid diagram source string to an SVG string.
 * Returns the cached SVG immediately if available; otherwise calls
 * mermaid.render() and stores the result.
 *
 * @param source  Raw Mermaid diagram source (without the ``` fences).
 * @param darkMode  Whether to use the dark Mermaid theme.
 * @returns  SVG string (not yet sanitized — caller must run DOMPurify).
 */
export async function renderMermaid(
  source: string,
  darkMode = false
): Promise<string> {
  const cached = getCached(source, darkMode);
  if (cached) return cached;

  initialiseMermaid(darkMode);

  const id = `vedox-mermaid-${++renderCounter}`;

  try {
    const { svg } = await mermaid.render(id, source.trim());
    setCached(source, darkMode, svg);
    return svg;
  } catch (err) {
    const message =
      err instanceof Error ? err.message : 'Mermaid render failed';
    // Return a minimal error SVG so the node island shows feedback.
    return `<svg xmlns="http://www.w3.org/2000/svg" width="300" height="60">
      <rect width="300" height="60" fill="#fee" rx="4"/>
      <text x="12" y="24" font-family="monospace" font-size="12" fill="#c00">Mermaid error</text>
      <text x="12" y="44" font-family="monospace" font-size="10" fill="#c00">${escapeXml(message.slice(0, 60))}</text>
    </svg>`;
  }
}

function escapeXml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&apos;');
}
