/**
 * NotificationSettings.test.ts
 *
 * Tests for NotificationSettings.svelte — toast duration segmented control and
 * badge visibility toggle.
 *
 * Design contract:
 *   - The "Toast duration" segmented control has six options; clicking one calls
 *     updatePrefs('notifications', { toastDuration }).
 *   - The "Count badge" toggle switch reflects `prefs.badgeVisible` via
 *     aria-checked and calls updatePrefs with the flipped value on click.
 */

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';

// ---------------------------------------------------------------------------
// Mocks — vi.hoisted runs before module imports; no svelte/store available here.
// We hand-roll the minimal Svelte store contract instead.
// ---------------------------------------------------------------------------

const prefsMock = vi.hoisted(() => {
  type State = { notifications: { toastDuration: number; soundEnabled: boolean; badgeVisible: boolean } };

  const _subscribers = new Set<(v: State) => void>();
  let _current: State = { notifications: { toastDuration: 4000, soundEnabled: false, badgeVisible: true } };

  function subscribe(fn: (v: State) => void): () => void {
    fn(_current);
    _subscribers.add(fn);
    return () => { _subscribers.delete(fn); };
  }

  function _set(next: State): void {
    _current = next;
    _subscribers.forEach((fn) => fn(_current));
  }

  return { subscribe, _set, updatePrefs: vi.fn() };
});

vi.mock('$lib/stores/preferences', () => ({
  userPrefs: { subscribe: prefsMock.subscribe },
  updatePrefs: prefsMock.updatePrefs,
}));

import NotificationSettings from '$lib/components/settings/NotificationSettings.svelte';

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('NotificationSettings', () => {
  beforeEach(() => {
    prefsMock.updatePrefs.mockReset();
    prefsMock._set({
      notifications: { toastDuration: 4000, soundEnabled: false, badgeVisible: true },
    });
  });

  it('should call updatePrefs with the selected toast duration when an option is clicked', async () => {
    render(NotificationSettings);

    const durationGroup = screen.getByRole('group', { name: /toast duration/i });
    const options = durationGroup.querySelectorAll('button[type="button"]');
    // 1.5s, 2.5s, 4s, 6s, 10s, Persistent
    expect(options.length).toBe(6);

    // "4 s" is the active default.
    const fourSBtn = screen.getByRole('button', { name: /^4 s$/i });
    expect(fourSBtn).toHaveAttribute('aria-pressed', 'true');

    // Click "Persistent" (value 0).
    const persistentBtn = screen.getByRole('button', { name: /^persistent$/i });
    await fireEvent.click(persistentBtn);

    expect(prefsMock.updatePrefs).toHaveBeenCalledWith('notifications', { toastDuration: 0 });
  });

  it('should toggle badge visibility and call updatePrefs with the flipped value', async () => {
    render(NotificationSettings);

    // badgeVisible defaults to true — the switch reports aria-checked="true".
    const badgeSwitch = screen.getByRole('switch', { name: /toggle count badge visibility/i });
    expect(badgeSwitch).toHaveAttribute('aria-checked', 'true');

    // Click to turn off the badge.
    await fireEvent.click(badgeSwitch);

    expect(prefsMock.updatePrefs).toHaveBeenCalledWith('notifications', { badgeVisible: false });
  });
});
