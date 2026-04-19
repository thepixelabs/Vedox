/**
 * EditorSettings.test.ts
 *
 * Tests for EditorSettings.svelte — default view selector and autosave
 * interval input.
 *
 * Design contract:
 *   - The "Default view" segmented control shows three options (Split, Preview,
 *     Source); clicking one calls updatePrefs('editor', { defaultView }).
 *   - The "Auto-save interval" segmented control shows six options; clicking one
 *     calls updatePrefs('editor', { autoSaveInterval }).
 *   - aria-pressed reflects which option is currently active in the store.
 */

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';

// ---------------------------------------------------------------------------
// Mocks — declared before the component import.
// vi.hoisted runs before any module-level imports, so we cannot use `writable`
// from 'svelte/store' here. We hand-roll the minimal Svelte store contract
// (subscribe / notify subscribers on set) instead.
// ---------------------------------------------------------------------------

const prefsMock = vi.hoisted(() => {
  type State = { editor: { defaultView: string; autoSaveInterval: number; spellCheck: boolean } };

  const _subscribers = new Set<(v: State) => void>();
  let _current: State = { editor: { defaultView: 'split', autoSaveInterval: 3000, spellCheck: false } };

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

import EditorSettings from '$lib/components/settings/EditorSettings.svelte';

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('EditorSettings', () => {
  beforeEach(() => {
    prefsMock.updatePrefs.mockReset();
    prefsMock._set({ editor: { defaultView: 'split', autoSaveInterval: 3000, spellCheck: false } });
  });

  it('should show three view options and call updatePrefs when a non-active view is selected', async () => {
    render(EditorSettings);

    const viewGroup = screen.getByRole('group', { name: /default view/i });
    const buttons = viewGroup.querySelectorAll('button[type="button"]');
    expect(buttons.length).toBe(3);

    // Split is the active default — its aria-pressed should be true.
    const splitBtn = screen.getByRole('button', { name: /^split$/i });
    expect(splitBtn).toHaveAttribute('aria-pressed', 'true');

    // Click Preview.
    const previewBtn = screen.getByRole('button', { name: /^preview$/i });
    await fireEvent.click(previewBtn);

    expect(prefsMock.updatePrefs).toHaveBeenCalledWith('editor', { defaultView: 'preview' });
  });

  it('should call updatePrefs with the correct interval when an autosave option is clicked', async () => {
    render(EditorSettings);

    const autoSaveGroup = screen.getByRole('group', { name: /auto-save interval/i });
    const buttons = autoSaveGroup.querySelectorAll('button[type="button"]');
    // Options: Off, 1 s, 3 s, 5 s, 10 s, 30 s
    expect(buttons.length).toBe(6);

    // "3 s" is the active default.
    const threeSBtn = screen.getByRole('button', { name: /^3 s$/i });
    expect(threeSBtn).toHaveAttribute('aria-pressed', 'true');

    // Click "Off" (value 0).
    const offBtn = screen.getByRole('button', { name: /^off$/i });
    await fireEvent.click(offBtn);

    expect(prefsMock.updatePrefs).toHaveBeenCalledWith('editor', { autoSaveInterval: 0 });
  });
});
