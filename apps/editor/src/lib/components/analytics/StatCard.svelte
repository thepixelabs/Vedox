<script lang="ts">
  /**
   * StatCard.svelte — compact metric card for the analytics overview strip.
   *
   * Props:
   *   label     — metric name (e.g. "Total Docs")
   *   value     — formatted display value (e.g. "47" or "—")
   *   trend     — optional direction: "up" | "down" | "flat"
   *   subtitle  — optional subtext below the value (e.g. "+3 this week")
   *   href      — optional anchor; clicking the card scrolls to a section
   *   loading   — shows skeleton shimmer when true
   */

  interface Props {
    label: string;
    value: string | number;
    trend?: "up" | "down" | "flat" | null;
    subtitle?: string | null;
    href?: string | null;
    loading?: boolean;
  }

  let {
    label,
    value,
    trend = null,
    subtitle = null,
    href = null,
    loading = false,
  }: Props = $props();

  const displayValue = $derived(
    loading ? "" : String(value)
  );
</script>

{#if href}
  <a class="stat-card" class:stat-card--loading={loading} {href} aria-label="{label}: {displayValue}">
    {@render cardBody()}
  </a>
{:else}
  <div class="stat-card" class:stat-card--loading={loading} aria-label="{label}: {displayValue}">
    {@render cardBody()}
  </div>
{/if}

{#snippet cardBody()}
  <span class="stat-card__label">{label}</span>

  <div class="stat-card__value-row">
    {#if loading}
      <span class="stat-card__skeleton stat-card__skeleton--value" aria-hidden="true"></span>
    {:else}
      <span class="stat-card__value">{displayValue}</span>
      {#if trend}
        <span
          class="stat-card__trend stat-card__trend--{trend}"
          aria-label={trend === "up" ? "trending up" : trend === "down" ? "trending down" : "flat"}
        >
          {#if trend === "up"}
            <!-- Up arrow -->
            <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <polyline points="18 15 12 9 6 15"/>
            </svg>
          {:else if trend === "down"}
            <!-- Down arrow -->
            <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <polyline points="6 9 12 15 18 9"/>
            </svg>
          {:else}
            <!-- Flat: minus -->
            <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" aria-hidden="true">
              <line x1="5" y1="12" x2="19" y2="12"/>
            </svg>
          {/if}
        </span>
      {/if}
    {/if}
  </div>

  {#if loading}
    <span class="stat-card__skeleton stat-card__skeleton--sub" aria-hidden="true"></span>
  {:else if subtitle}
    <span class="stat-card__subtitle">{subtitle}</span>
  {/if}
{/snippet}

<style>
  /* ── Base card ─────────────────────────────────────────────────────────── */

  .stat-card {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    padding: var(--space-4) var(--space-5);
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    text-decoration: none;
    color: inherit;
    min-height: 96px;
    transition:
      border-color 100ms var(--ease-out),
      background-color 100ms var(--ease-out);
  }

  a.stat-card:hover {
    border-color: var(--color-accent);
    background-color: var(--color-surface-overlay);
    cursor: pointer;
  }

  a.stat-card:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  /* ── Label ─────────────────────────────────────────────────────────────── */

  .stat-card__label {
    font-size: var(--text-2xs, 11px);
    font-weight: 600;
    letter-spacing: var(--tracking-wider, 0.06em);
    text-transform: uppercase;
    color: var(--color-text-muted);
    font-family: var(--font-mono);
    line-height: 1;
  }

  /* ── Value row ─────────────────────────────────────────────────────────── */

  .stat-card__value-row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex: 1;
    align-items: flex-end;
  }

  .stat-card__value {
    font-size: var(--text-2xl, 28px);
    font-weight: 600;
    color: var(--color-text-primary);
    font-variant-numeric: tabular-nums;
    letter-spacing: var(--tracking-tighter, -0.025em);
    line-height: 1;
  }

  /* ── Trend indicator ───────────────────────────────────────────────────── */

  .stat-card__trend {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 20px;
    height: 20px;
    border-radius: var(--radius-full);
    flex-shrink: 0;
    margin-bottom: 2px;
  }

  .stat-card__trend--up {
    background-color: color-mix(in srgb, var(--color-success, oklch(74% 0.16 162)) 15%, transparent);
    color: var(--color-success, oklch(74% 0.16 162));
  }

  .stat-card__trend--down {
    background-color: color-mix(in srgb, var(--color-error, oklch(70% 0.18 25)) 15%, transparent);
    color: var(--color-error, oklch(70% 0.18 25));
  }

  .stat-card__trend--flat {
    background-color: var(--color-surface-overlay);
    color: var(--color-text-muted);
  }

  /* ── Subtitle ──────────────────────────────────────────────────────────── */

  .stat-card__subtitle {
    font-size: var(--font-size-sm, 12px);
    color: var(--color-text-muted);
    font-family: var(--font-mono);
    line-height: 1;
  }

  /* ── Loading skeleton ──────────────────────────────────────────────────── */

  .stat-card--loading {
    pointer-events: none;
  }

  .stat-card__skeleton {
    display: block;
    background-color: var(--color-surface-overlay);
    border-radius: var(--radius-sm);
    animation: stat-shimmer 1.4s ease-in-out infinite;
  }

  .stat-card__skeleton--value {
    width: 60%;
    height: 28px;
    margin-top: auto;
  }

  .stat-card__skeleton--sub {
    width: 40%;
    height: 12px;
  }

  @keyframes stat-shimmer {
    0%   { opacity: 0.5; }
    50%  { opacity: 0.9; }
    100% { opacity: 0.5; }
  }

  /* ── Reduced motion ────────────────────────────────────────────────────── */

  @media (prefers-reduced-motion: reduce) {
    .stat-card {
      transition: none;
    }

    .stat-card__skeleton {
      animation: none;
      opacity: 0.6;
    }
  }
</style>
