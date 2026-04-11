<!--
  CodeBlockView.svelte — Svelte renderer for a Tiptap code block node.

  This component is instantiated by CodeBlockShiki.ts (the Tiptap extension)
  when a code_block node needs to render in the editor. It receives the raw
  code text and the language tag, asks highlight.ts to produce Shiki HTML,
  and draws the chrome frame — a subtle border, a top header with the
  language tag (left) and a copy button (right), and optional line numbers
  when the block has more than three lines.

  Colours are never hardcoded here. Everything flows through CSS variables
  from tokens.css (--code-bg, --code-border, --code-header-bg, --code-text,
  --code-muted, --text-1, --surface-1, --accent, etc.). The creative agent
  is wiring those up in parallel.

  Props
  -----
  code      : string — raw code text (always the source of truth)
  language  : string — language tag from the fence, e.g. "ts", "go"
  onCopy    : (code: string) => void (optional)
      Fires after the user clicks copy and the clipboard write succeeded.
      Used by tests; production uses the built-in toast.

  Note on reactivity
  ------------------
  The `code` and `language` props can be rebound by Tiptap when the user
  edits the block (live re-highlight is a nice-to-have; Tiptap's baseline
  CodeBlockLowlight re-renders on every transaction). We debounce the
  re-highlight so typing feels snappy.
-->
<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { highlight, normalizeLang, type CodeTheme } from './highlight';

  // ---------------------------------------------------------------------------
  // Props
  // ---------------------------------------------------------------------------
  interface Props {
    code: string;
    language?: string;
    onCopy?: (code: string) => void;
  }

  let { code, language = 'text', onCopy }: Props = $props();

  // ---------------------------------------------------------------------------
  // State
  // ---------------------------------------------------------------------------

  /** Rendered Shiki HTML (a <pre> element with inlined token styles). */
  let highlightedHtml = $state<string>('');

  /** Whether a highlight pass is currently in flight. */
  let isRendering = $state<boolean>(false);

  /** Whether the last copy succeeded — drives the "Copied" toast. */
  let showCopied = $state<boolean>(false);

  /** Number of lines in the source — used to decide whether to show gutters. */
  const lineCount = $derived(code.split('\n').length);

  /** Whether to render a gutter with line numbers. */
  const showLineNumbers = $derived(lineCount > 3);

  /** Normalized language id for the badge. */
  const badgeLang = $derived(normalizeLang(language));

  // ---------------------------------------------------------------------------
  // Theme resolution
  // ---------------------------------------------------------------------------
  // Read `data-theme` off <html>. The creative agent's theme store manages
  // that attribute, so we just read whatever's there.
  function resolveTheme(): CodeTheme {
    if (typeof document === 'undefined') return 'github-dark';
    return document.documentElement.getAttribute('data-theme') === 'light'
      ? 'github-light'
      : 'github-dark';
  }

  let theme = $state<CodeTheme>('github-dark');

  // ---------------------------------------------------------------------------
  // Highlight pipeline
  // ---------------------------------------------------------------------------
  let renderToken = 0; // monotonic counter to drop stale results

  async function render(nextCode: string, nextLang: string, nextTheme: CodeTheme): Promise<void> {
    const token = ++renderToken;
    isRendering = true;
    try {
      const html = await highlight(nextCode, nextLang, { theme: nextTheme });
      // Only commit if we're still the newest render; otherwise a faster
      // follow-up call already updated the DOM.
      if (token === renderToken) {
        highlightedHtml = html;
      }
    } catch {
      // highlight() already degrades to plaintext on failure; if even that
      // threw, fall back to an escaped raw render.
      if (token === renderToken) {
        highlightedHtml = fallbackPre(nextCode);
      }
    } finally {
      if (token === renderToken) {
        isRendering = false;
      }
    }
  }

  function fallbackPre(src: string): string {
    const escaped = src
      .replaceAll('&', '&amp;')
      .replaceAll('<', '&lt;')
      .replaceAll('>', '&gt;');
    return `<pre class="shiki-fallback"><code>${escaped}</code></pre>`;
  }

  // Reactive re-highlight when code, language, or theme changes.
  $effect(() => {
    // Touch the reactive reads so Svelte knows to re-run us.
    const nextCode = code;
    const nextLang = language;
    const nextTheme = theme;
    // untrack() keeps our internal state updates from forming a feedback loop.
    untrack(() => {
      void render(nextCode, nextLang, nextTheme);
    });
  });

  // Watch for theme changes at the <html> level. The design system toggles
  // `data-theme` on the root; we just mirror that and re-render.
  onMount(() => {
    theme = resolveTheme();
    const obs = new MutationObserver(() => {
      const next = resolveTheme();
      if (next !== theme) theme = next;
    });
    obs.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['data-theme'],
    });
    return () => obs.disconnect();
  });

  // ---------------------------------------------------------------------------
  // Copy to clipboard
  // ---------------------------------------------------------------------------
  let copiedTimer: ReturnType<typeof setTimeout> | null = null;

  async function handleCopy(): Promise<void> {
    try {
      await navigator.clipboard.writeText(code);
      onCopy?.(code);
      showCopied = true;
      if (copiedTimer) clearTimeout(copiedTimer);
      copiedTimer = setTimeout(() => {
        showCopied = false;
      }, 1500);
    } catch (err) {
      // Clipboard write can fail in insecure contexts or when the user
      // denies permission. Log and keep the UI honest — no false-positive
      // "Copied" toast.
      // eslint-disable-next-line no-console
      console.warn('[vedox] Clipboard write failed:', err);
    }
  }
</script>

<div class="code-block" data-language={badgeLang} data-theme={theme}>
  <header class="code-block__header">
    <span class="code-block__lang" aria-label={`Language: ${badgeLang}`}>
      {badgeLang}
    </span>
    <button
      type="button"
      class="code-block__copy"
      onclick={handleCopy}
      aria-label={showCopied ? 'Copied to clipboard' : 'Copy code to clipboard'}
      aria-live="polite"
    >
      {#if showCopied}
        <svg
          width="13"
          height="13"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2.25"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <polyline points="20 6 9 17 4 12" />
        </svg>
        <span>Copied</span>
      {:else}
        <svg
          width="13"
          height="13"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.75"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
          <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
        </svg>
        <span>Copy</span>
      {/if}
    </button>
  </header>

  <div class="code-block__body" class:code-block__body--numbered={showLineNumbers}>
    {#if showLineNumbers}
      <div class="code-block__gutter" aria-hidden="true">
        {#each Array.from({ length: lineCount }, (_, i) => i + 1) as n (n)}
          <span class="code-block__lineno">{n}</span>
        {/each}
      </div>
    {/if}

    <div class="code-block__shiki">
      {#if isRendering && !highlightedHtml}
        <!-- First paint: show a lightweight plaintext placeholder so the
             editor isn't empty while Shiki warms up. -->
        <pre class="code-block__placeholder"><code>{code}</code></pre>
      {:else}
        <!-- Shiki output is authored by us, not user-provided HTML. Safe
             to render verbatim. -->
        {@html highlightedHtml}
      {/if}
    </div>
  </div>
</div>

<style>
  /* ── Frame ─────────────────────────────────────────────────────────────── */
  .code-block {
    position: relative;
    margin: 1em 0;
    border: 1px solid var(--code-border, var(--border-hairline, rgba(255, 255, 255, 0.08)));
    border-radius: var(--radius-md, 10px);
    background-color: var(--code-bg, var(--surface-code, #0d1117));
    overflow: hidden;
    font-family: var(--font-mono, ui-monospace, 'SF Mono', Menlo, monospace);
  }

  .code-block:focus-within {
    border-color: var(--accent, var(--color-accent));
    box-shadow: 0 0 0 1px
      color-mix(in srgb, var(--accent, var(--color-accent)) 35%, transparent);
  }

  /* ── Header strip ───────────────────────────────────────────────────────── */
  .code-block__header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 6px 12px;
    background-color: var(--code-header-bg, var(--color-surface-overlay, rgba(255, 255, 255, 0.03)));
    border-bottom: 1px solid var(--code-border, var(--border-hairline, rgba(255, 255, 255, 0.06)));
    font-size: 11px;
    line-height: 1;
  }

  .code-block__lang {
    text-transform: lowercase;
    letter-spacing: 0.04em;
    color: var(--code-muted, var(--color-text-muted));
    font-weight: 500;
  }

  .code-block__copy {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 4px 8px;
    font: inherit;
    font-size: 11px;
    color: var(--code-muted, var(--color-text-muted));
    background: transparent;
    border: 1px solid transparent;
    border-radius: var(--radius-sm, 6px);
    cursor: pointer;
    transition:
      background-color 80ms ease,
      color 80ms ease,
      border-color 80ms ease;
  }

  .code-block__copy:hover,
  .code-block__copy:focus-visible {
    color: var(--text-1, var(--color-text-primary));
    background-color: var(--code-header-hover, var(--color-surface-overlay, rgba(255, 255, 255, 0.06)));
    border-color: var(--code-border, var(--border-hairline));
    outline: none;
  }

  .code-block__copy:focus-visible {
    outline: 2px solid var(--accent, var(--color-accent));
    outline-offset: 2px;
  }

  /* ── Body ───────────────────────────────────────────────────────────────── */
  .code-block__body {
    display: grid;
    grid-template-columns: 1fr;
    overflow-x: auto;
  }

  .code-block__body--numbered {
    grid-template-columns: auto 1fr;
  }

  .code-block__gutter {
    display: flex;
    flex-direction: column;
    padding: 16px 10px 16px 14px;
    text-align: right;
    user-select: none;
    color: var(--code-muted, var(--color-text-subtle, #6e7681));
    font-size: 12px;
    line-height: 1.55;
    border-right: 1px solid var(--code-border, var(--border-hairline, rgba(255, 255, 255, 0.06)));
    background-color: color-mix(
      in srgb,
      var(--code-bg, var(--color-surface-elevated, #0d1117)) 95%,
      transparent
    );
  }

  .code-block__lineno {
    display: block;
    font-variant-numeric: tabular-nums;
  }

  .code-block__shiki {
    /* Shiki outputs its own <pre> — we just style its container. */
    min-width: 0;
    overflow-x: auto;
    color: var(--code-text, var(--color-text-primary));
  }

  /* Normalise the Shiki <pre> so our frame owns the padding and background. */
  .code-block__shiki :global(pre) {
    margin: 0;
    padding: 16px 18px;
    background-color: transparent !important;
    font-family: inherit;
    font-size: 13px;
    line-height: 1.55;
    overflow-x: auto;
  }

  .code-block__shiki :global(pre code) {
    font-family: inherit;
    color: inherit;
    background: transparent;
    border: none;
    padding: 0;
  }

  /* Plaintext fallback used before first Shiki paint and on unknown grammars. */
  .code-block__placeholder {
    margin: 0;
    padding: 16px 18px;
    background: transparent;
    color: var(--code-text, var(--color-text-primary));
    font-family: inherit;
    font-size: 13px;
    line-height: 1.55;
    white-space: pre;
  }

  .code-block__shiki :global(.shiki-fallback) {
    color: var(--code-text, var(--color-text-primary));
  }

  /* ── Reduced motion ─────────────────────────────────────────────────────── */
  @media (prefers-reduced-motion: reduce) {
    .code-block__copy {
      transition: none;
    }
  }
</style>
