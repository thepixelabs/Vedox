<script lang="ts">
  /**
   * VelocityChart.svelte — 7-day and 30-day change velocity display.
   *
   * v2.0: number-first display with directional trend indicator.
   * v2.1: will add a grouped bar chart using canvas or an SVG sparkline.
   *
   * Props:
   *   velocity7d  — doc changes in the last 7 days
   *   velocity30d — doc changes in the last 30 days
   *   loading     — skeleton state
   */

  interface Props {
    velocity7d: number;
    velocity30d: number;
    loading?: boolean;
  }

  let { velocity7d, velocity30d, loading = false }: Props = $props();

  /** Trend within the 30d window: week is faster, slower, or similar. */
  const weeklyAvgOf30 = $derived(Math.round(velocity30d / 4));

  const sevenDayTrend = $derived((): "up" | "down" | "flat" => {
    if (velocity7d > weeklyAvgOf30 * 1.1) return "up";
    if (velocity7d < weeklyAvgOf30 * 0.9) return "down";
    return "flat";
  });

  const trendLabel: Record<"up" | "down" | "flat", string> = {
    up:   "above 30-day weekly average",
    down: "below 30-day weekly average",
    flat: "on pace with 30-day weekly average",
  };
</script>

<section class="velocity" aria-labelledby="velocity-heading">
  <header class="velocity__header">
    <h2 class="velocity__heading" id="velocity-heading">change velocity</h2>
    <span class="velocity__note">v2.1 will add a 12-week bar chart</span>
  </header>

  <div class="velocity__panels">
    <!-- 7-day panel -->
    <div class="velocity__panel" aria-label="7-day velocity">
      <span class="velocity__window">7d</span>
      {#if loading}
        <span class="velocity__skeleton velocity__skeleton--val" aria-hidden="true"></span>
        <span class="velocity__skeleton velocity__skeleton--sub" aria-hidden="true"></span>
      {:else}
        <div class="velocity__val-row">
          <span class="velocity__val">{velocity7d}</span>
          <span
            class="velocity__trend velocity__trend--{sevenDayTrend()}"
            aria-label={trendLabel[sevenDayTrend()]}
          >
            {#if sevenDayTrend() === "up"}
              <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                <polyline points="18 15 12 9 6 15"/>
              </svg>
            {:else if sevenDayTrend() === "down"}
              <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                <polyline points="6 9 12 15 18 9"/>
              </svg>
            {:else}
              <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" aria-hidden="true">
                <line x1="5" y1="12" x2="19" y2="12"/>
              </svg>
            {/if}
          </span>
        </div>
        <span class="velocity__sub">docs changed</span>
      {/if}
    </div>

    <div class="velocity__divider" aria-hidden="true"></div>

    <!-- 30-day panel -->
    <div class="velocity__panel" aria-label="30-day velocity">
      <span class="velocity__window">30d</span>
      {#if loading}
        <span class="velocity__skeleton velocity__skeleton--val" aria-hidden="true"></span>
        <span class="velocity__skeleton velocity__skeleton--sub" aria-hidden="true"></span>
      {:else}
        <div class="velocity__val-row">
          <span class="velocity__val">{velocity30d}</span>
        </div>
        <span class="velocity__sub">
          ~{weeklyAvgOf30}/wk avg
        </span>
      {/if}
    </div>
  </div>
</section>

<style>
  /* ── Section ───────────────────────────────────────────────────────────── */

  .velocity {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .velocity__header {
    display: flex;
    align-items: baseline;
    gap: var(--space-3);
  }

  .velocity__heading {
    font-size: var(--text-2xs, 11px);
    font-weight: 600;
    letter-spacing: var(--tracking-wider, 0.06em);
    text-transform: uppercase;
    color: var(--color-text-muted);
    font-family: var(--font-mono);
    margin: 0;
  }

  .velocity__note {
    font-size: var(--font-size-sm, 12px);
    color: var(--color-text-muted);
    font-family: var(--font-mono);
    opacity: 0.6;
  }

  /* ── Panels row ────────────────────────────────────────────────────────── */

  .velocity__panels {
    display: flex;
    align-items: stretch;
    gap: 0;
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    overflow: hidden;
  }

  .velocity__panel {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    padding: var(--space-4) var(--space-5);
  }

  .velocity__divider {
    width: 1px;
    background-color: var(--color-border);
    flex-shrink: 0;
  }

  /* ── Window label ──────────────────────────────────────────────────────── */

  .velocity__window {
    font-size: var(--text-2xs, 11px);
    font-weight: 600;
    letter-spacing: var(--tracking-wider, 0.06em);
    text-transform: uppercase;
    color: var(--color-text-muted);
    font-family: var(--font-mono);
  }

  /* ── Value row ─────────────────────────────────────────────────────────── */

  .velocity__val-row {
    display: flex;
    align-items: flex-end;
    gap: var(--space-2);
    flex: 1;
  }

  .velocity__val {
    font-size: var(--text-2xl, 28px);
    font-weight: 600;
    color: var(--color-text-primary);
    font-variant-numeric: tabular-nums;
    letter-spacing: var(--tracking-tighter, -0.025em);
    line-height: 1;
  }

  /* ── Trend badge ───────────────────────────────────────────────────────── */

  .velocity__trend {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 20px;
    height: 20px;
    border-radius: var(--radius-full);
    flex-shrink: 0;
    margin-bottom: 2px;
  }

  .velocity__trend--up {
    background-color: color-mix(in srgb, var(--color-success, oklch(74% 0.16 162)) 15%, transparent);
    color: var(--color-success, oklch(74% 0.16 162));
  }

  .velocity__trend--down {
    background-color: color-mix(in srgb, var(--color-error, oklch(70% 0.18 25)) 15%, transparent);
    color: var(--color-error, oklch(70% 0.18 25));
  }

  .velocity__trend--flat {
    background-color: var(--color-surface-overlay);
    color: var(--color-text-muted);
  }

  /* ── Subtitle ──────────────────────────────────────────────────────────── */

  .velocity__sub {
    font-size: var(--font-size-sm, 12px);
    color: var(--color-text-muted);
    font-family: var(--font-mono);
  }

  /* ── Skeleton ──────────────────────────────────────────────────────────── */

  .velocity__skeleton {
    display: block;
    background-color: var(--color-surface-overlay);
    border-radius: var(--radius-sm);
    animation: velocity-shimmer 1.4s ease-in-out infinite;
  }

  .velocity__skeleton--val {
    width: 60px;
    height: 28px;
    margin-top: auto;
  }

  .velocity__skeleton--sub {
    width: 80px;
    height: 12px;
  }

  @keyframes velocity-shimmer {
    0%   { opacity: 0.5; }
    50%  { opacity: 0.9; }
    100% { opacity: 0.5; }
  }

  /* ── Responsive ────────────────────────────────────────────────────────── */

  @media (max-width: 400px) {
    .velocity__panels {
      flex-direction: column;
    }

    .velocity__divider {
      width: 100%;
      height: 1px;
    }
  }

  /* ── Reduced motion ────────────────────────────────────────────────────── */

  @media (prefers-reduced-motion: reduce) {
    .velocity__skeleton {
      animation: none;
      opacity: 0.6;
    }
  }
</style>
