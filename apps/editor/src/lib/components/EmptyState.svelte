<script lang="ts">
  /**
   * EmptyState.svelte — reusable empty / zero-data state component.
   *
   * Renders a centered card with an icon, heading, body copy, and up to two
   * action buttons (CTA + optional secondary ghost button).
   *
   * Design spec:
   *   - Centered vertically and horizontally in its container
   *   - Icon: 48×48, --color-text-muted
   *   - Heading: --font-size-xl, --color-text-primary
   *   - Body: --font-size-base, --color-text-secondary, max-width 380px, centered
   *   - CTA: accent-filled, --color-accent bg, --color-text-inverse text, --radius-md
   *   - Secondary: ghost style, --color-text-secondary
   *   - Fade-in: opacity 0 → 1 over 200ms, translateY(8px) → 0
   */

  interface ActionDef {
    label: string;
    onClick?: () => void;
    href?: string;
  }

  interface Props {
    /**
     * Icon content. Pass an inline SVG string for vector icons, or a plain
     * emoji/character for text icons. Rendered at 48×48 in muted color.
     */
    icon: string;
    heading: string;
    body: string;
    cta?: ActionDef;
    secondary?: ActionDef;
    /** When true the icon slot is replaced with a 40px spinning ring. */
    spinning?: boolean;
  }

  let {
    icon,
    heading,
    body,
    cta,
    secondary,
    spinning = false,
  }: Props = $props();

  /** Returns true when the string looks like an inline SVG element. */
  function isSvg(s: string): boolean {
    return s.trimStart().startsWith('<svg');
  }
</script>

<div class="empty-state" role="region" aria-label={heading}>
  <div class="empty-state__card">
    <!-- Icon / spinner -->
    <div class="empty-state__icon" aria-hidden="true">
      {#if spinning}
        <span class="empty-state__spinner"></span>
      {:else if isSvg(icon)}
        <!-- eslint-disable-next-line svelte/no-at-html-tags -->
        {@html icon}
      {:else}
        <span class="empty-state__emoji">{icon}</span>
      {/if}
    </div>

    <!-- Text -->
    <h2 class="empty-state__heading">{heading}</h2>
    <p class="empty-state__body">{body}</p>

    <!-- Actions -->
    {#if cta || secondary}
      <div class="empty-state__actions">
        {#if cta}
          {#if cta.href}
            <a
              class="empty-state__cta"
              href={cta.href}
              role="button"
              onclick={cta.onClick}
            >{cta.label}</a>
          {:else}
            <button
              class="empty-state__cta"
              type="button"
              onclick={cta.onClick}
            >{cta.label}</button>
          {/if}
        {/if}

        {#if secondary}
          {#if secondary.href}
            <a
              class="empty-state__secondary"
              href={secondary.href}
              role="button"
              onclick={secondary.onClick}
            >{secondary.label}</a>
          {:else}
            <button
              class="empty-state__secondary"
              type="button"
              onclick={secondary.onClick}
            >{secondary.label}</button>
          {/if}
        {/if}
      </div>
    {/if}
  </div>
</div>

<style>
  /* ── Container ────────────────────────────────────────────────────────────── */

  .empty-state {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100%;
    padding: var(--space-8);
  }

  /* ── Card ─────────────────────────────────────────────────────────────────── */

  .empty-state__card {
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    gap: var(--space-4);
    /* Subtle fade-up entrance */
    animation: empty-state-in 200ms ease both;
  }

  @keyframes empty-state-in {
    from {
      opacity: 0;
      transform: translateY(8px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  /* ── Icon area ────────────────────────────────────────────────────────────── */

  .empty-state__icon {
    color: var(--color-text-muted);
    width: 48px;
    height: 48px;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  /* SVG children inherit the muted color via currentColor */
  .empty-state__icon :global(svg) {
    width: 48px;
    height: 48px;
    color: var(--color-text-muted);
  }

  .empty-state__emoji {
    font-size: 40px;
    line-height: 1;
    /* Emoji don't respond to currentColor so just size them. */
  }

  /* ── Spinner ──────────────────────────────────────────────────────────────── */

  .empty-state__spinner {
    display: block;
    width: 40px;
    height: 40px;
    border: 3px solid var(--color-border);
    border-top-color: var(--color-accent);
    border-radius: 50%;
    animation: empty-state-spin 700ms linear infinite;
  }

  @keyframes empty-state-spin {
    to {
      transform: rotate(360deg);
    }
  }

  /* ── Typography ───────────────────────────────────────────────────────────── */

  .empty-state__heading {
    font-size: var(--font-size-xl);
    font-weight: 600;
    color: var(--color-text-primary);
    letter-spacing: -0.02em;
    /* Reset h2 margin — gap on the parent card controls spacing */
    margin: 0;
  }

  .empty-state__body {
    font-size: var(--font-size-base);
    color: var(--color-text-secondary);
    line-height: 1.6;
    max-width: 380px;
    margin: 0;
  }

  /* ── Actions ──────────────────────────────────────────────────────────────── */

  .empty-state__actions {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-3);
    justify-content: center;
    margin-top: var(--space-2);
  }

  /* Shared base for both button and anchor CTA */
  .empty-state__cta,
  .empty-state__secondary {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: var(--space-2) var(--space-5);
    font-size: var(--font-size-base);
    font-weight: 500;
    font-family: var(--font-sans);
    border-radius: var(--radius-md);
    text-decoration: none;
    cursor: pointer;
    transition:
      background-color var(--duration-fast) var(--ease-out),
      color var(--duration-fast) var(--ease-out),
      border-color var(--duration-fast) var(--ease-out),
      transform 80ms var(--ease-out);
    /* Reset button defaults */
    border: none;
    outline: none;
  }

  /* Filled / accent CTA */
  .empty-state__cta {
    background-color: var(--color-accent);
    color: var(--color-text-inverse);
    border: 1px solid transparent;
  }

  .empty-state__cta:hover {
    background-color: var(--color-accent-hover);
    color: var(--color-text-inverse);
    transform: translateY(-1px);
    text-decoration: none;
  }

  .empty-state__cta:active {
    transform: translateY(0);
  }

  .empty-state__cta:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 3px;
  }

  /* Ghost secondary */
  .empty-state__secondary {
    background-color: transparent;
    color: var(--color-text-secondary);
    border: 1px solid var(--color-border);
  }

  .empty-state__secondary:hover {
    border-color: var(--color-border-strong);
    color: var(--color-text-primary);
    text-decoration: none;
  }

  .empty-state__secondary:active {
    transform: translateY(0);
  }

  .empty-state__secondary:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 3px;
  }

  /* ── Reduced motion ──────────────────────────────────────────────────────── */
  @media (prefers-reduced-motion: reduce) {
    .empty-state__card {
      animation: none;
    }
    .empty-state__spinner {
      animation: none;
      border-top-color: var(--color-accent);
    }
    .empty-state__cta,
    .empty-state__secondary {
      transition: none;
    }
    .empty-state__cta:hover,
    .empty-state__cta:active,
    .empty-state__secondary:active {
      transform: none;
    }
  }
</style>
