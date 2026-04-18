<script lang="ts">
  /**
   * NotificationSettings — Category 7
   *
   * Toast duration, sound toggle, badge visibility.
   */

  import { userPrefs, updatePrefs } from '$lib/stores/preferences';

  interface Props {
    searchQuery?: string;
  }

  let { searchQuery = '' }: Props = $props();

  const toastDurations = [
    { value: 1500, label: '1.5 s' },
    { value: 2500, label: '2.5 s' },
    { value: 4000, label: '4 s' },
    { value: 6000, label: '6 s' },
    { value: 10000, label: '10 s' },
    { value: 0, label: 'Persistent' },
  ];

  function matches(text: string): boolean {
    if (!searchQuery) return true;
    return text.toLowerCase().includes(searchQuery.toLowerCase());
  }

  const prefs = $derived($userPrefs.notifications);

  // Format duration label for the current value if it's a custom value
  function durationLabel(ms: number): string {
    const match = toastDurations.find((d) => d.value === ms);
    return match ? match.label : `${ms / 1000} s`;
  }
</script>

<div class="settings-category">

  <!-- Toast duration -->
  {#if matches('toast') || matches('notification') || matches('duration') || matches('dismiss')}
    <div class="setting-row">
      <div class="setting-row__label">
        <span class="setting-row__name">Toast duration</span>
        <span class="setting-row__desc">
          How long toast notifications stay visible before auto-dismissing. "Persistent" keeps them until manually dismissed.
        </span>
      </div>
      <div class="setting-row__control">
        <div class="seg-buttons" role="group" aria-label="Toast duration">
          {#each toastDurations as opt (opt.value)}
            <button
              type="button"
              class="seg-btn"
              class:seg-btn--active={prefs.toastDuration === opt.value}
              aria-pressed={prefs.toastDuration === opt.value}
              onclick={() => updatePrefs('notifications', { toastDuration: opt.value })}
            >{opt.label}</button>
          {/each}
        </div>
      </div>
    </div>
  {/if}

  <!-- Sound -->
  {#if matches('sound') || matches('audio') || matches('notification') || matches('chime')}
    <div class="setting-row">
      <div class="setting-row__label">
        <span class="setting-row__name">Notification sounds</span>
        <span class="setting-row__desc">Play a subtle chime when the agent completes a task or an error occurs.</span>
      </div>
      <div class="setting-row__control">
        <button
          type="button"
          role="switch"
          class="toggle-switch"
          class:toggle-switch--on={prefs.soundEnabled}
          aria-checked={prefs.soundEnabled}
          onclick={() => updatePrefs('notifications', { soundEnabled: !prefs.soundEnabled })}
          aria-label="Toggle notification sounds"
        >
          <span class="toggle-switch__thumb" aria-hidden="true"></span>
        </button>
      </div>
    </div>
  {/if}

  <!-- Badge visibility -->
  {#if matches('badge') || matches('count') || matches('indicator') || matches('notification')}
    <div class="setting-row">
      <div class="setting-row__label">
        <span class="setting-row__name">Count badge</span>
        <span class="setting-row__desc">Show a numeric badge on the Review Queue link in the sidebar when there are pending items.</span>
      </div>
      <div class="setting-row__control">
        <button
          type="button"
          role="switch"
          class="toggle-switch"
          class:toggle-switch--on={prefs.badgeVisible}
          aria-checked={prefs.badgeVisible}
          onclick={() => updatePrefs('notifications', { badgeVisible: !prefs.badgeVisible })}
          aria-label="Toggle count badge visibility"
        >
          <span class="toggle-switch__thumb" aria-hidden="true"></span>
        </button>
      </div>
    </div>
  {/if}

</div>

<style>
  .settings-category {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .setting-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-6);
    padding: var(--space-3) 0;
    border-bottom: 1px solid var(--color-border);
  }

  .setting-row:last-child {
    border-bottom: none;
  }

  .setting-row__label {
    display: flex;
    flex-direction: column;
    gap: 2px;
    flex: 1;
    min-width: 0;
  }

  .setting-row__name {
    font-size: var(--font-size-sm);
    font-weight: 500;
    color: var(--color-text-primary);
  }

  .setting-row__desc {
    font-size: var(--font-size-sm);
    color: var(--color-text-muted);
    line-height: 1.4;
  }

  .setting-row__control {
    flex-shrink: 0;
  }

  .seg-buttons {
    display: flex;
    gap: 2px;
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 2px;
  }

  .seg-btn {
    padding: var(--space-1) var(--space-3);
    background: none;
    border: none;
    border-radius: calc(var(--radius-md) - 2px);
    color: var(--color-text-muted);
    font-size: var(--font-size-sm);
    font-family: var(--font-sans);
    cursor: pointer;
    transition: background-color 100ms ease, color 100ms ease;
    white-space: nowrap;
    line-height: 1.4;
  }

  .seg-btn:hover {
    color: var(--color-text-primary);
  }

  .seg-btn--active {
    background-color: var(--color-surface-base);
    color: var(--color-text-primary);
    font-weight: 500;
    box-shadow: var(--shadow-sm);
  }

  .seg-btn:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  .toggle-switch {
    position: relative;
    display: inline-flex;
    align-items: center;
    width: 40px;
    height: 22px;
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: 11px;
    cursor: pointer;
    padding: 0;
    transition: background-color 150ms ease, border-color 150ms ease;
  }

  .toggle-switch--on {
    background-color: var(--color-accent);
    border-color: var(--color-accent);
  }

  .toggle-switch:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  .toggle-switch__thumb {
    position: absolute;
    left: 2px;
    width: 16px;
    height: 16px;
    background-color: var(--color-text-inverse);
    border-radius: 50%;
    transition: transform 150ms ease;
    box-shadow: var(--shadow-sm);
  }

  .toggle-switch--on .toggle-switch__thumb {
    transform: translateX(18px);
  }
</style>
