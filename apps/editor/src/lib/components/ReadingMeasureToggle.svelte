<script lang="ts">
  import { readingStore } from '$lib/stores/reading';
  import type { ReadingMeasure } from '$lib/stores/reading';

  const measures: { value: ReadingMeasure; label: string; title: string }[] = [
    { value: 'narrow', label: '\u2194', title: 'Narrow (64ch)' },
    { value: 'default', label: '\u27F7', title: 'Default (68ch)' },
    { value: 'wide', label: '\u27FA', title: 'Wide (80ch)' },
  ];
</script>

<div class="measure-toggle" role="group" aria-label="Reading width">
  {#each measures as m}
    <button
      class="measure-btn"
      class:measure-btn--active={$readingStore === m.value}
      type="button"
      title={m.title}
      aria-label={m.title}
      aria-pressed={$readingStore === m.value}
      onclick={() => readingStore.setMeasure(m.value)}
    >
      {m.label}
    </button>
  {/each}
</div>

<style>
  .measure-toggle {
    display: flex;
    align-items: center;
    border: 1px solid var(--border-hairline);
    border-radius: var(--radius-sm);
    overflow: hidden;
  }

  .measure-btn {
    width: 28px;
    height: 24px;
    border: none;
    background: transparent;
    color: var(--text-3);
    cursor: pointer;
    font-size: var(--text-sm);
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background-color var(--duration-fast) var(--ease-out), color var(--duration-fast) var(--ease-out);
  }

  .measure-btn + .measure-btn {
    border-left: 1px solid var(--border-hairline);
  }

  .measure-btn:hover {
    background: var(--surface-4);
    color: var(--text-1);
  }

  .measure-btn--active {
    background: var(--accent-subtle);
    color: var(--accent-text);
  }
</style>
