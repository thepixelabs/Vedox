/**
 * KeyboardSettings.test.ts
 *
 * Tests for KeyboardSettings.svelte — shortcut list rendering, click-to-remap
 * key capture mode, conflict detection, and reset-to-default.
 *
 * Design contract:
 *   - Every shortcut in shortcuts-data renders as a dt (description) + key badge.
 *   - Clicking a key badge enters capture mode: the input appears, the badge
 *     disappears, and pressing a key combo updates `pendingKey`.
 *   - If two shortcuts share the same effective key (after overrides), both rows
 *     get the `.shortcut-row--conflict` class AND each shows a conflict indicator.
 *   - Clicking "Reset all shortcuts to defaults" calls
 *     updatePrefs('keyboard', { overrides: {} }).
 */

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';

// ---------------------------------------------------------------------------
// Mocks — vi.hoisted runs before module imports; hand-roll minimal store contract.
// ---------------------------------------------------------------------------

// Controllable keyboard overrides — start with no overrides so defaults show.
const prefsMock = vi.hoisted(() => {
  type State = { keyboard: { overrides: Record<string, string> } };

  const _subscribers = new Set<(v: State) => void>();
  let _current: State = { keyboard: { overrides: {} } };

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

import KeyboardSettings from '$lib/components/settings/KeyboardSettings.svelte';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function renderComponent(props: Record<string, unknown> = {}) {
  return render(KeyboardSettings, { props });
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('KeyboardSettings', () => {
  beforeEach(() => {
    prefsMock.updatePrefs.mockReset();
    prefsMock._set({ keyboard: { overrides: {} } });
  });

  it('should render a shortcut row for every entry in shortcuts-data', () => {
    renderComponent();

    // shortcuts-data exports 12 shortcuts. Each dt has the description text.
    // We verify at least the Navigation category headings and a sample action.
    expect(screen.getByText('Open command palette')).toBeInTheDocument();
    expect(screen.getByText('Bold')).toBeInTheDocument();
    expect(screen.getByText('Split pane')).toBeInTheDocument();

    // Each shortcut has a clickable key badge (button wrapping a kbd).
    const keyBadges = document.querySelectorAll('.key-badge');
    expect(keyBadges.length).toBe(12);
  });

  it('should enter key-capture mode when a key badge is clicked, then confirm on Enter key', async () => {
    renderComponent();

    // Click the badge for "Bold" (⌘B).
    const boldBadge = screen.getByRole('button', {
      name: /change shortcut for bold/i,
    });
    await fireEvent.click(boldBadge);

    // Capture input appears; badge disappears.
    const captureInput = screen.getByRole('textbox', { name: /press key combination for bold/i });
    expect(captureInput).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /change shortcut for bold/i })).toBeNull();

    // Simulate pressing ⌘G.
    await fireEvent.keyDown(captureInput, { key: 'G', metaKey: true });

    // Confirm by pressing Enter.
    await fireEvent.keyDown(captureInput, { key: 'Enter' });

    // updatePrefs should have been called with the new binding.
    expect(prefsMock.updatePrefs).toHaveBeenCalledWith('keyboard', {
      overrides: expect.objectContaining({ '⌘B': '⌘+G' }),
    });
  });

  it('should highlight both conflicting rows when two shortcuts share the same key', () => {
    // Override ⌘B to ⌘K — now both "Open command palette" (⌘K) and "Bold" (⌘B
    // overridden to ⌘K) share the same effective key.
    prefsMock._set({ keyboard: { overrides: { '⌘B': '⌘K' } } });

    renderComponent();

    // Both rows should have the conflict class.
    const conflictRows = document.querySelectorAll('.shortcut-row--conflict');
    expect(conflictRows.length).toBe(2);

    // Both should show the conflict indicator (the amber "!" badge).
    const conflictIndicators = document.querySelectorAll('.conflict-label');
    expect(conflictIndicators.length).toBe(2);
  });

  it('should call updatePrefs with empty overrides when the reset-all button is clicked', async () => {
    // Start with an existing override so the button is enabled.
    prefsMock._set({ keyboard: { overrides: { '⌘B': '⌘G' } } });

    renderComponent();

    const resetBtn = screen.getByRole('button', { name: /reset all shortcuts to defaults/i });
    expect(resetBtn).not.toBeDisabled();

    await fireEvent.click(resetBtn);

    expect(prefsMock.updatePrefs).toHaveBeenCalledWith('keyboard', { overrides: {} });
  });
});
