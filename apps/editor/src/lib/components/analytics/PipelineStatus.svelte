<script lang="ts">
  /**
   * PipelineStatus.svelte — shown when `pipeline_ready: false`.
   *
   * Renders a warm status banner with a spinner and copy explaining
   * that analytics are computing. Hides itself when pipeline is ready.
   *
   * Props:
   *   ready — pass-through of `pipeline_ready` from the API response
   */

  interface Props {
    ready: boolean;
  }

  let { ready }: Props = $props();
</script>

{#if !ready}
  <div class="pipeline-status" role="status" aria-live="polite" aria-busy="true">
    <span class="pipeline-status__spinner" aria-hidden="true"></span>
    <div class="pipeline-status__body">
      <span class="pipeline-status__title">pipeline warming up</span>
      <span class="pipeline-status__desc">
        analytics are computing in the background — check back in a moment.
      </span>
    </div>
  </div>
{/if}

<style>
  .pipeline-status {
    display: flex;
    align-items: flex-start;
    gap: var(--space-3);
    padding: var(--space-4) var(--space-5);
    background-color: color-mix(in srgb, var(--color-info, oklch(72% 0.14 230)) 10%, transparent);
    border: 1px solid color-mix(in srgb, var(--color-info, oklch(72% 0.14 230)) 30%, transparent);
    border-radius: var(--radius-lg);
    color: var(--color-info, oklch(72% 0.14 230));
  }

  /* ── Spinner ───────────────────────────────────────────────────────────── */

  .pipeline-status__spinner {
    flex-shrink: 0;
    display: inline-block;
    width: 16px;
    height: 16px;
    margin-top: 2px;
    border: 2px solid color-mix(in srgb, var(--color-info, oklch(72% 0.14 230)) 30%, transparent);
    border-top-color: var(--color-info, oklch(72% 0.14 230));
    border-radius: 50%;
    animation: pipeline-spin 700ms linear infinite;
  }

  @keyframes pipeline-spin {
    to { transform: rotate(360deg); }
  }

  /* ── Body ──────────────────────────────────────────────────────────────── */

  .pipeline-status__body {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .pipeline-status__title {
    font-size: var(--font-size-sm, 12px);
    font-weight: 600;
    font-family: var(--font-mono);
    color: var(--color-info, oklch(72% 0.14 230));
  }

  .pipeline-status__desc {
    font-size: var(--font-size-sm, 12px);
    color: var(--color-text-muted);
    line-height: 1.4;
  }

  /* ── Reduced motion ────────────────────────────────────────────────────── */

  @media (prefers-reduced-motion: reduce) {
    .pipeline-status__spinner {
      animation: none;
      opacity: 0.5;
    }
  }
</style>
