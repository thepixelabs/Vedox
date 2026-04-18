<script lang="ts">
  /**
   * /analytics — Vedox analytics dashboard.
   *
   * Fetches GET /api/analytics/summary on mount.
   * Layout: page header → pipeline status → 4-card stat strip → 2-column
   *   content area (DocsPerProject + VelocityChart) → expansion slots for v2.1.
   *
   * Designed for expansion: every section is a labelled <section> with an
   * aria-labelledby ID so future sections slot in without layout rewrites.
   *
   * Backend response shape (v1):
   *   {
   *     pipeline_ready:      boolean,
   *     total_docs:          number,
   *     docs_per_project:    Record<string, number>,
   *     change_velocity_7d:  number,
   *     change_velocity_30d: number,
   *   }
   */

  import { onMount } from "svelte";
  import StatCard from "$lib/components/analytics/StatCard.svelte";
  import DocsPerProject from "$lib/components/analytics/DocsPerProject.svelte";
  import VelocityChart from "$lib/components/analytics/VelocityChart.svelte";
  import PipelineStatus from "$lib/components/analytics/PipelineStatus.svelte";

  // ---------------------------------------------------------------------------
  // API response shape
  // ---------------------------------------------------------------------------

  interface AnalyticsSummary {
    pipeline_ready: boolean;
    total_docs: number;
    docs_per_project: Record<string, number>;
    change_velocity_7d: number;
    change_velocity_30d: number;
  }

  // ---------------------------------------------------------------------------
  // State
  // ---------------------------------------------------------------------------

  type LoadState = "idle" | "loading" | "done" | "error";

  let loadState: LoadState = $state("idle");
  let summary = $state<AnalyticsSummary | null>(null);
  let errorMessage: string = $state("");

  // ---------------------------------------------------------------------------
  // Derived display values
  // ---------------------------------------------------------------------------

  const pipelineReady = $derived(summary?.pipeline_ready ?? true);
  const totalDocs = $derived(summary ? String(summary.total_docs) : "—");
  const projectCount = $derived(
    summary ? String(Object.keys(summary.docs_per_project).length) : "—"
  );
  const velocity7d = $derived(summary?.change_velocity_7d ?? 0);
  const velocity30d = $derived(summary?.change_velocity_30d ?? 0);
  const docsPerProject = $derived(summary?.docs_per_project ?? {});

  /** Simple velocity trend for the StatCard. */
  const velocityTrend = $derived((): "up" | "down" | "flat" => {
    if (!summary) return "flat";
    const avg30weekly = Math.round(summary.change_velocity_30d / 4);
    if (summary.change_velocity_7d > avg30weekly * 1.1) return "up";
    if (summary.change_velocity_7d < avg30weekly * 0.9) return "down";
    return "flat";
  });

  const isLoading = $derived(loadState === "idle" || loadState === "loading");

  // ---------------------------------------------------------------------------
  // Fetch
  // ---------------------------------------------------------------------------

  async function fetchSummary() {
    loadState = "loading";
    try {
      const res = await fetch("/api/analytics/summary", {
        headers: { Accept: "application/json" },
      });
      if (!res.ok) {
        const text = await res.text().catch(() => res.statusText);
        throw new Error(`${res.status} — ${text}`);
      }
      summary = (await res.json()) as AnalyticsSummary;
      loadState = "done";
    } catch (err) {
      errorMessage = err instanceof Error ? err.message : "unknown error";
      loadState = "error";
    }
  }

  onMount(() => {
    fetchSummary();
  });
</script>

<svelte:head>
  <title>Analytics — Vedox</title>
</svelte:head>

<div class="analytics">
  <!-- ── Page header ────────────────────────────────────────────────────── -->
  <header class="analytics__header">
    <div class="analytics__title-row">
      <h1 class="analytics__title">analytics</h1>
      {#if loadState === "done"}
        <button
          class="analytics__refresh"
          type="button"
          aria-label="Refresh analytics"
          onclick={fetchSummary}
        >
          <svg
            width="13"
            height="13"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
            aria-hidden="true"
          >
            <polyline points="23 4 23 10 17 10"/>
            <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/>
          </svg>
          refresh
        </button>
      {/if}
    </div>
    <p class="analytics__subtitle">
      metrics based on your own usage — all data stays on device.
    </p>
  </header>

  <!-- ── Error state ────────────────────────────────────────────────────── -->
  {#if loadState === "error"}
    <div class="analytics__error" role="alert" aria-live="assertive">
      <svg
        width="14"
        height="14"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <circle cx="12" cy="12" r="10"/>
        <line x1="12" y1="8" x2="12" y2="12"/>
        <line x1="12" y1="16" x2="12.01" y2="16"/>
      </svg>
      <div class="analytics__error-body">
        <span class="analytics__error-title">failed to load analytics</span>
        <span class="analytics__error-msg">{errorMessage}</span>
      </div>
      <button
        class="analytics__error-retry"
        type="button"
        onclick={fetchSummary}
      >retry</button>
    </div>
  {/if}

  <!-- ── Pipeline warming banner ────────────────────────────────────────── -->
  {#if loadState === "done" && !pipelineReady}
    <PipelineStatus ready={pipelineReady} />
  {/if}

  <!-- ── Overview strip: 4 StatCards ───────────────────────────────────── -->
  <section class="analytics__section" aria-labelledby="overview-heading">
    <h2 class="sr-only" id="overview-heading">Overview</h2>
    <div class="analytics__stat-grid">
      <StatCard
        label="total docs"
        value={isLoading ? "" : totalDocs}
        loading={isLoading}
        subtitle={isLoading ? null : "across all projects"}
        href="#docs-per-project"
      />
      <StatCard
        label="projects"
        value={isLoading ? "" : projectCount}
        loading={isLoading}
        subtitle={isLoading ? null : "indexed"}
        href="#docs-per-project"
      />
      <StatCard
        label="7d velocity"
        value={isLoading ? "" : String(velocity7d)}
        loading={isLoading}
        trend={isLoading ? null : velocityTrend()}
        subtitle={isLoading ? null : "doc changes this week"}
        href="#velocity"
      />
      <StatCard
        label="30d velocity"
        value={isLoading ? "" : String(velocity30d)}
        loading={isLoading}
        subtitle={isLoading ? null : "doc changes this month"}
        href="#velocity"
      />
    </div>
  </section>

  <!-- ── Primary charts ─────────────────────────────────────────────────── -->
  <div class="analytics__charts">
    <!-- Docs per project bar chart -->
    <section
      class="analytics__chart-card"
      id="docs-per-project"
      aria-labelledby="dpp-section-heading"
    >
      <DocsPerProject data={docsPerProject} loading={isLoading} />
    </section>

    <!-- Velocity numbers -->
    <section
      class="analytics__chart-card"
      id="velocity"
      aria-labelledby="velocity-section-heading"
    >
      <VelocityChart
        velocity7d={velocity7d}
        velocity30d={velocity30d}
        loading={isLoading}
      />
    </section>
  </div>

  <!-- ── Future expansion zone ──────────────────────────────────────────── -->
  <!--
    v2.1 slots: docs by type (ring chart), most-referenced docs, staleness
    report, word-count trend, activity heatmap. Each will insert a new
    <section class="analytics__chart-card"> here without touching the
    layout above.
  -->
  <section class="analytics__coming-soon" aria-label="Upcoming metrics">
    <span class="analytics__coming-soon__label">more metrics coming in v2.1</span>
    <ul class="analytics__coming-soon__list" aria-label="Planned metrics">
      <li>docs by type</li>
      <li>most-referenced docs</li>
      <li>staleness report</li>
      <li>word count trend</li>
      <li>activity heatmap</li>
    </ul>
  </section>
</div>

<style>
  /* ── Page shell ────────────────────────────────────────────────────────── */

  .analytics {
    display: flex;
    flex-direction: column;
    gap: var(--space-8);
    padding: var(--space-8);
    max-width: 960px;
    /* Allow full-width on wide viewports without collapsing below min */
    width: 100%;
    /* If the layout shell uses overflow-y: auto on the main area, this
       ensures the page can scroll past the viewport height. */
  }

  /* ── Page header ───────────────────────────────────────────────────────── */

  .analytics__header {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .analytics__title-row {
    display: flex;
    align-items: center;
    gap: var(--space-4);
  }

  .analytics__title {
    font-size: var(--text-xl, 22px);
    font-weight: 600;
    color: var(--color-text-primary);
    letter-spacing: var(--tracking-tight, -0.015em);
    margin: 0;
    font-family: var(--font-mono);
  }

  .analytics__subtitle {
    font-size: var(--font-size-sm, 12px);
    color: var(--color-text-muted);
    margin: 0;
    font-family: var(--font-mono);
  }

  /* ── Refresh button ────────────────────────────────────────────────────── */

  .analytics__refresh {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    padding: 4px var(--space-3);
    background: none;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-text-muted);
    font-size: var(--font-size-sm, 12px);
    font-family: var(--font-mono);
    cursor: pointer;
    transition:
      color 80ms var(--ease-out),
      border-color 80ms var(--ease-out);
  }

  .analytics__refresh:hover {
    color: var(--color-text-primary);
    border-color: var(--color-text-muted);
  }

  .analytics__refresh:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  /* ── Error banner ──────────────────────────────────────────────────────── */

  .analytics__error {
    display: flex;
    align-items: flex-start;
    gap: var(--space-3);
    padding: var(--space-4) var(--space-5);
    background-color: color-mix(in srgb, var(--color-error, oklch(70% 0.18 25)) 10%, transparent);
    border: 1px solid color-mix(in srgb, var(--color-error, oklch(70% 0.18 25)) 30%, transparent);
    border-radius: var(--radius-lg);
    color: var(--color-error, oklch(70% 0.18 25));
  }

  .analytics__error-body {
    display: flex;
    flex-direction: column;
    gap: 2px;
    flex: 1;
  }

  .analytics__error-title {
    font-size: var(--font-size-sm, 12px);
    font-weight: 600;
    font-family: var(--font-mono);
  }

  .analytics__error-msg {
    font-size: var(--font-size-sm, 12px);
    color: var(--color-text-muted);
    font-family: var(--font-mono);
  }

  .analytics__error-retry {
    flex-shrink: 0;
    padding: 4px var(--space-3);
    background: none;
    border: 1px solid color-mix(in srgb, var(--color-error, oklch(70% 0.18 25)) 40%, transparent);
    border-radius: var(--radius-md);
    color: var(--color-error, oklch(70% 0.18 25));
    font-size: var(--font-size-sm, 12px);
    font-family: var(--font-mono);
    cursor: pointer;
    align-self: center;
    transition: border-color 80ms var(--ease-out);
  }

  .analytics__error-retry:hover {
    border-color: var(--color-error, oklch(70% 0.18 25));
  }

  .analytics__error-retry:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  /* ── Sections ──────────────────────────────────────────────────────────── */

  .analytics__section {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  /* ── Stat grid: 4 cards, responsive ───────────────────────────────────── */

  .analytics__stat-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: var(--space-4);
  }

  @media (max-width: 720px) {
    .analytics__stat-grid {
      grid-template-columns: repeat(2, 1fr);
    }
  }

  @media (max-width: 400px) {
    .analytics__stat-grid {
      grid-template-columns: 1fr;
    }
  }

  /* ── Charts area: 2 columns on wide, stacked on narrow ───────────────── */

  .analytics__charts {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--space-6);
    align-items: start;
  }

  @media (max-width: 640px) {
    .analytics__charts {
      grid-template-columns: 1fr;
    }
  }

  /* ── Individual chart card ─────────────────────────────────────────────── */

  .analytics__chart-card {
    padding: var(--space-6);
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-xl);
  }

  /* ── Expansion / coming soon zone ─────────────────────────────────────── */

  .analytics__coming-soon {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    padding: var(--space-5) var(--space-6);
    border: 1px dashed var(--color-border);
    border-radius: var(--radius-xl);
  }

  .analytics__coming-soon__label {
    font-size: var(--text-2xs, 11px);
    font-weight: 600;
    letter-spacing: var(--tracking-wider, 0.06em);
    text-transform: uppercase;
    color: var(--color-text-muted);
    font-family: var(--font-mono);
  }

  .analytics__coming-soon__list {
    list-style: none;
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    padding: 0;
    margin: 0;
  }

  .analytics__coming-soon__list li {
    display: inline-flex;
    align-items: center;
    padding: 3px var(--space-3);
    font-size: var(--font-size-sm, 12px);
    font-family: var(--font-mono);
    color: var(--color-text-muted);
    background-color: var(--color-surface-overlay);
    border-radius: var(--radius-full);
    border: 1px solid var(--color-border);
  }

  /* ── Accessible utility ────────────────────────────────────────────────── */

  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border-width: 0;
  }

  /* ── Responsive padding ────────────────────────────────────────────────── */

  @media (max-width: 640px) {
    .analytics {
      padding: var(--space-4);
      gap: var(--space-6);
    }
  }

  /* ── Reduced motion ────────────────────────────────────────────────────── */

  @media (prefers-reduced-motion: reduce) {
    .analytics__refresh {
      transition: none;
    }

    .analytics__error-retry {
      transition: none;
    }
  }
</style>
