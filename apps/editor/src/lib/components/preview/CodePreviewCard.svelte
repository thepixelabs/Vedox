<!--
  CodePreviewCard.svelte — floating hover card for vedox:// code cross-links.

  Shows a syntax-highlighted excerpt from a file resolved via
  GET /api/preview?url=vedox://file/path#L10-L25.

  Layout:
    ┌────────────────────────────────────────┐
    │ path/to/file.go           go  [L10-25] │  header
    ├────────────────────────────────────────┤
    │ <shimmer skeleton | highlighted code>  │  body (max 300px, scrolls)
    ├────────────────────────────────────────┤
    │ ⚠ truncated — file > 500 KB            │  (only when truncated)
    └────────────────────────────────────────┘

  Positioning:
    - Anchored below the link element by default.
    - Flips above when there is not enough space below (viewport-edge aware).
    - Max width: 480px. Min width: 260px.
    - Card is 400px default width, shrinks at narrow viewports.

  Dismiss:
    - Click outside the card.
    - Escape key.
    - Mouse leaves both the trigger link AND the card, after a 300ms delay.

  Cache:
    - LRU in-memory cache keyed on the full vedox:// URL (max 64 entries).
    - Successful responses and "not found" errors are both cached to stop
      repeated round-trips on re-hover.

  Accessibility:
    - role="tooltip" on the card (connected to the trigger via aria-describedby).
    - Escape dismisses and returns focus to the trigger.
    - Card itself is focusable (tabindex="0") and contains scrollable code.
    - prefers-reduced-motion: animations are suppressed.
-->

<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { highlight, normalizeLang } from '$lib/editor/codeblock/highlight';

  // ---------------------------------------------------------------------------
  // Public API
  // ---------------------------------------------------------------------------

  interface Props {
    /** The full vedox:// URL this card is previewing. */
    vedoxUrl: string;
    /** The DOM element of the trigger link — used to position the card. */
    triggerEl: HTMLElement;
    /**
     * Unique id used for aria-describedby linkage between trigger and tooltip.
     * Must match what VedoxLinkHandler sets on the trigger's aria-describedby.
     */
    cardId: string;
    /** Called when the card should be dismissed. */
    onDismiss: () => void;
  }

  const { vedoxUrl, triggerEl, cardId, onDismiss }: Props = $props();

  // ---------------------------------------------------------------------------
  // Preview API response shape
  // ---------------------------------------------------------------------------

  interface PreviewResponse {
    filePath: string;
    language: string;
    content: string;
    startLine: number;
    endLine: number;
    totalLines: number;
    truncated: boolean;
  }

  // ---------------------------------------------------------------------------
  // LRU cache (module-scoped, shared across all card instances)
  // ---------------------------------------------------------------------------

  const LRU_MAX = 64;

  type CacheEntry =
    | { ok: true; data: PreviewResponse }
    | { ok: false; message: string };

  // We use a Map — insertion order is iteration order, so we can evict the
  // first (oldest) key when the cache is full.
  const previewCache = new Map<string, CacheEntry>();

  function cacheSet(key: string, entry: CacheEntry): void {
    if (previewCache.size >= LRU_MAX) {
      // Evict the oldest entry (first in insertion order).
      const firstKey = previewCache.keys().next().value;
      if (firstKey !== undefined) previewCache.delete(firstKey);
    }
    previewCache.set(key, entry);
  }

  // ---------------------------------------------------------------------------
  // Component state
  // ---------------------------------------------------------------------------

  type LoadState = 'loading' | 'loaded' | 'error';

  let loadState: LoadState = $state('loading');
  let preview: PreviewResponse | null = $state(null);
  let errorMsg: string = $state('');
  let highlightedHtml: string = $state('');

  // Card DOM reference (needed for outside-click detection).
  let cardEl: HTMLDivElement | null = $state(null);

  // Positioning state.
  let cardTop: number = $state(0);
  let cardLeft: number = $state(0);
  let flipAbove: boolean = $state(false);

  // Mouse-leave dismiss timer.
  let dismissTimer: ReturnType<typeof setTimeout> | null = null;

  // ---------------------------------------------------------------------------
  // Position calculation
  // ---------------------------------------------------------------------------

  /**
   * Place the card below the trigger, flipping above when the card would
   * overflow the bottom edge of the viewport.
   */
  function recalcPosition(): void {
    if (!triggerEl) return;

    const CARD_HEIGHT_EST = 340; // max-height + header; px
    const CARD_WIDTH = 480;
    const GAP = 6; // px gap between trigger and card

    const rect = triggerEl.getBoundingClientRect();
    const vw = window.innerWidth;
    const vh = window.innerHeight;

    // Horizontal: align with trigger, clamp so card doesn't fall off screen.
    const rawLeft = rect.left + window.scrollX;
    const maxLeft = vw - CARD_WIDTH - 8;
    cardLeft = Math.max(8, Math.min(rawLeft, maxLeft));

    // Vertical: prefer below; flip above if not enough room.
    const spaceBelow = vh - rect.bottom;
    flipAbove = spaceBelow < CARD_HEIGHT_EST && rect.top > CARD_HEIGHT_EST;

    if (flipAbove) {
      cardTop = rect.top + window.scrollY - CARD_HEIGHT_EST - GAP;
    } else {
      cardTop = rect.bottom + window.scrollY + GAP;
    }
  }

  // ---------------------------------------------------------------------------
  // Data fetching
  // ---------------------------------------------------------------------------

  /**
   * Detect the theme from the document element so Shiki matches the editor.
   */
  function resolveTheme(): 'github-dark' | 'github-light' {
    if (typeof document === 'undefined') return 'github-dark';
    const t = document.documentElement.getAttribute('data-theme') ?? '';
    return t === 'eclipse' || t === 'paper' ? 'github-light' : 'github-dark';
  }

  async function fetchPreview(): Promise<void> {
    // Cache hit — instant.
    const cached = previewCache.get(vedoxUrl);
    if (cached) {
      if (cached.ok) {
        preview = cached.data;
        await renderHighlight(cached.data);
        loadState = 'loaded';
      } else {
        errorMsg = cached.message;
        loadState = 'error';
      }
      return;
    }

    loadState = 'loading';

    try {
      const params = new URLSearchParams({ url: vedoxUrl });
      const res = await fetch(`/api/preview?${params.toString()}`, {
        headers: { Accept: 'application/json' },
      });

      if (!res.ok) {
        let msg = `HTTP ${res.status}`;
        try {
          const body = (await res.json()) as { message?: string };
          if (body.message) msg = body.message;
        } catch {
          /* ignore */
        }
        const entry: CacheEntry = { ok: false, message: msg };
        cacheSet(vedoxUrl, entry);
        errorMsg = msg;
        loadState = 'error';
        return;
      }

      const data = (await res.json()) as PreviewResponse;
      cacheSet(vedoxUrl, { ok: true, data });
      preview = data;
      await renderHighlight(data);
      loadState = 'loaded';
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Network error';
      // Network failures are NOT cached — they may succeed on retry.
      errorMsg = msg;
      loadState = 'error';
    }
  }

  async function renderHighlight(data: PreviewResponse): Promise<void> {
    const lang = normalizeLang(data.language);
    const theme = resolveTheme();
    highlightedHtml = await highlight(data.content, lang, { theme });
  }

  // ---------------------------------------------------------------------------
  // Mouse-leave dismiss (300ms delay so user can move cursor into card)
  // ---------------------------------------------------------------------------

  export function scheduleHide(): void {
    if (dismissTimer !== null) return;
    dismissTimer = setTimeout(() => {
      onDismiss();
    }, 300);
  }

  export function cancelHide(): void {
    if (dismissTimer !== null) {
      clearTimeout(dismissTimer);
      dismissTimer = null;
    }
  }

  // ---------------------------------------------------------------------------
  // Keyboard: Escape
  // ---------------------------------------------------------------------------

  function onKeydown(e: KeyboardEvent): void {
    if (e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      onDismiss();
      triggerEl?.focus();
    }
  }

  // ---------------------------------------------------------------------------
  // Click-outside dismiss
  // ---------------------------------------------------------------------------

  function onDocumentClick(e: MouseEvent): void {
    if (!cardEl) return;
    const target = e.target as Node | null;
    if (target && !cardEl.contains(target) && !triggerEl.contains(target)) {
      onDismiss();
    }
  }

  // ---------------------------------------------------------------------------
  // Lifecycle
  // ---------------------------------------------------------------------------

  onMount(() => {
    recalcPosition();

    // Reposition on scroll or resize.
    window.addEventListener('scroll', recalcPosition, { passive: true });
    window.addEventListener('resize', recalcPosition, { passive: true });

    document.addEventListener('keydown', onKeydown, true);
    // Defer outside-click listener by one frame so the triggering click
    // does not immediately dismiss the card.
    requestAnimationFrame(() => {
      document.addEventListener('click', onDocumentClick, true);
    });

    void fetchPreview();
  });

  onDestroy(() => {
    window.removeEventListener('scroll', recalcPosition);
    window.removeEventListener('resize', recalcPosition);
    document.removeEventListener('keydown', onKeydown, true);
    document.removeEventListener('click', onDocumentClick, true);
    if (dismissTimer !== null) clearTimeout(dismissTimer);
  });

  // ---------------------------------------------------------------------------
  // Derived display values
  // ---------------------------------------------------------------------------

  const displayPath: string = $derived.by(() => {
    const p = preview as PreviewResponse | null;
    return p != null ? p.filePath : '';
  });
  const displayLang: string = $derived.by(() => {
    const p = preview as PreviewResponse | null;
    return p != null ? p.language : '';
  });
  const lineRange: string = $derived.by(() => {
    const p = preview as PreviewResponse | null;
    return p != null ? `L${p.startLine}–${p.endLine}` : '';
  });
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
  bind:this={cardEl}
  id={cardId}
  class="cpc"
  class:cpc--above={flipAbove}
  role="tooltip"
  aria-label="Code preview"
  tabindex="-1"
  style:top="{cardTop}px"
  style:left="{cardLeft}px"
  onmouseenter={cancelHide}
  onmouseleave={scheduleHide}
>
  {#if loadState === 'loading'}
    <!-- Shimmer skeleton matching code block shape -->
    <div class="cpc__header cpc__header--skeleton" aria-hidden="true">
      <span class="cpc__skeleton cpc__skeleton--path"></span>
      <span class="cpc__skeleton cpc__skeleton--badge"></span>
    </div>
    <div class="cpc__body" aria-busy="true" aria-label="Loading code preview">
      <div class="cpc__shimmer-lines" aria-hidden="true">
        {#each Array(8) as _, i (i)}
          <span
            class="cpc__shimmer-line"
            style:width="{40 + Math.abs(Math.sin(i * 2.1)) * 55}%"
          ></span>
        {/each}
      </div>
    </div>

  {:else if loadState === 'error'}
    <div class="cpc__header">
      <span class="cpc__path" title={vedoxUrl}>{vedoxUrl}</span>
    </div>
    <div class="cpc__body cpc__body--error" role="alert">
      <svg
        class="cpc__error-icon"
        width="14"
        height="14"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.8"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <circle cx="12" cy="12" r="10"/>
        <line x1="12" y1="8" x2="12" y2="12"/>
        <line x1="12" y1="16" x2="12.01" y2="16"/>
      </svg>
      <span>{errorMsg}</span>
    </div>

  {:else if loadState === 'loaded' && preview}
    <!-- Header: path + lang badge + line range -->
    <header class="cpc__header">
      <span class="cpc__path" title={displayPath}>{displayPath}</span>
      <div class="cpc__meta">
        {#if displayLang && displayLang !== 'text'}
          <span class="cpc__lang-badge">{displayLang}</span>
        {/if}
        {#if lineRange}
          <span class="cpc__line-range">{lineRange}</span>
        {/if}
      </div>
    </header>

    <!-- Code body -->
    <div class="cpc__body cpc__body--code">
      {@html highlightedHtml}
    </div>

    <!-- Truncated indicator -->
    {#if preview.truncated}
      <footer class="cpc__footer">
        <svg
          width="12"
          height="12"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          <path d="M10.3 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
          <line x1="12" y1="9" x2="12" y2="13"/>
          <line x1="12" y1="17" x2="12.01" y2="17"/>
        </svg>
        <span>Showing excerpt — file exceeds 500 KB</span>
      </footer>
    {/if}
  {/if}
</div>

<style>
  /* ── Card shell ──────────────────────────────────────────────────────────── */

  .cpc {
    position: absolute;
    width: 480px;
    max-width: min(480px, calc(100vw - 16px));
    max-height: 340px;
    background: var(--surface-4);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-lg);
    box-shadow:
      0 0 0 1px oklch(from var(--border-default) l c h / 0.5),
      0 4px 16px oklch(0% 0 0 / 0.36),
      0 24px 64px oklch(0% 0 0 / 0.28);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    z-index: var(--z-tooltip);
    /* Entry animation */
    animation: cpc-in var(--duration-default) var(--ease-out) both;
  }

  .cpc--above {
    animation-name: cpc-in-above;
  }

  @keyframes cpc-in {
    from { opacity: 0; transform: translateY(-6px) scale(0.97); }
    to   { opacity: 1; transform: translateY(0)    scale(1); }
  }

  @keyframes cpc-in-above {
    from { opacity: 0; transform: translateY(6px) scale(0.97); }
    to   { opacity: 1; transform: translateY(0)   scale(1); }
  }

  @media (prefers-reduced-motion: reduce) {
    .cpc { animation: none; }
  }

  /* ── Header ──────────────────────────────────────────────────────────────── */

  .cpc__header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    background: var(--surface-3);
    border-bottom: 1px solid var(--border-hairline);
    flex-shrink: 0;
    min-height: 32px;
  }

  .cpc__header--skeleton {
    gap: var(--space-3);
  }

  .cpc__path {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--text-2);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 1;
    min-width: 0;
  }

  .cpc__meta {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    flex-shrink: 0;
  }

  .cpc__lang-badge {
    font-family: var(--font-mono);
    font-size: var(--text-2xs);
    font-weight: 600;
    text-transform: lowercase;
    letter-spacing: var(--tracking-wide);
    padding: 2px var(--space-2);
    border-radius: var(--radius-sm);
    background: var(--accent-subtle);
    color: var(--accent-text);
    border: 1px solid var(--accent-border);
    white-space: nowrap;
  }

  .cpc__line-range {
    font-family: var(--font-mono);
    font-size: var(--text-2xs);
    color: var(--text-4);
    white-space: nowrap;
  }

  /* ── Body ────────────────────────────────────────────────────────────────── */

  .cpc__body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    overflow-x: auto;
  }

  /* Shiki emits <pre class="shiki …"><code>…</code></pre>. */
  .cpc__body--code :global(pre.shiki) {
    margin: 0;
    padding: var(--space-3);
    background: var(--surface-code) !important;
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    line-height: 1.6;
    tab-size: 2;
    overflow: visible; /* body handles scroll */
  }

  .cpc__body--code :global(pre.shiki code) {
    background: transparent;
    font-family: inherit;
    font-size: inherit;
    line-height: inherit;
  }

  /* Error body */
  .cpc__body--error {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
    color: var(--error);
    font-size: var(--text-sm);
    font-family: var(--font-body);
  }

  .cpc__error-icon {
    flex-shrink: 0;
    opacity: 0.8;
  }

  /* ── Truncation footer ───────────────────────────────────────────────────── */

  .cpc__footer {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    padding: var(--space-1) var(--space-3);
    background: oklch(from var(--warning) l c h / 0.10);
    border-top: 1px solid oklch(from var(--warning) l c h / 0.30);
    color: var(--warning);
    font-size: var(--text-2xs);
    font-family: var(--font-body);
    flex-shrink: 0;
  }

  /* ── Shimmer skeleton ────────────────────────────────────────────────────── */

  .cpc__skeleton {
    display: block;
    border-radius: var(--radius-sm);
    background: linear-gradient(
      90deg,
      var(--border-hairline) 0%,
      var(--border-default) 50%,
      var(--border-hairline) 100%
    );
    background-size: 200% 100%;
    animation: cpc-shimmer 1.4s var(--ease-in-out) infinite;
    height: 12px;
  }

  .cpc__skeleton--path {
    flex: 1;
    max-width: 220px;
  }

  .cpc__skeleton--badge {
    width: 36px;
    flex-shrink: 0;
  }

  .cpc__shimmer-lines {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
  }

  .cpc__shimmer-line {
    display: block;
    height: 11px;
    border-radius: var(--radius-sm);
    background: linear-gradient(
      90deg,
      var(--border-hairline) 0%,
      var(--border-default) 50%,
      var(--border-hairline) 100%
    );
    background-size: 200% 100%;
    animation: cpc-shimmer 1.4s var(--ease-in-out) infinite;
  }

  /* Stagger each line so they shimmer in sequence, not all at once. */
  .cpc__shimmer-line:nth-child(2) { animation-delay: 0.07s; }
  .cpc__shimmer-line:nth-child(3) { animation-delay: 0.14s; }
  .cpc__shimmer-line:nth-child(4) { animation-delay: 0.21s; }
  .cpc__shimmer-line:nth-child(5) { animation-delay: 0.28s; }
  .cpc__shimmer-line:nth-child(6) { animation-delay: 0.35s; }
  .cpc__shimmer-line:nth-child(7) { animation-delay: 0.42s; }
  .cpc__shimmer-line:nth-child(8) { animation-delay: 0.49s; }

  @keyframes cpc-shimmer {
    0%   { background-position: 200% 0; }
    100% { background-position: -200% 0; }
  }

  @media (prefers-reduced-motion: reduce) {
    .cpc__skeleton,
    .cpc__shimmer-line {
      animation: none;
      background: var(--border-default);
    }
  }
</style>
