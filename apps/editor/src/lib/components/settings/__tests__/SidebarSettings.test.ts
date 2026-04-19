/**
 * SidebarSettings.test.ts
 *
 * Tests for SidebarSettings.svelte — the default panel selector and the doc
 * tree grouping toggle.
 *
 * Design contract:
 *   - "Default panel" segmented control has three options; clicking one calls
 *     updatePrefs('sidebar', { defaultPanel }).
 *   - "Doc tree grouping" segmented control has three options; clicking one
 *     calls updatePrefs('sidebar', { docTreeGrouping }).
 *   - sidebarStore.setPosition() is called when a position button is clicked.
 *   - aria-pressed reflects active state from the store.
 */

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';

// ---------------------------------------------------------------------------
// Mocks — vi.hoisted runs before module imports; hand-roll minimal store contract.
// ---------------------------------------------------------------------------

const prefsMock = vi.hoisted(() => {
  type State = { sidebar: { defaultPanel: string; collapseOnOpen: boolean; docTreeGrouping: string } };

  const _subscribers = new Set<(v: State) => void>();
  let _current: State = { sidebar: { defaultPanel: 'tree', collapseOnOpen: false, docTreeGrouping: 'type-first' } };

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

const sidebarMock = vi.hoisted(() => {
  type SidebarState = { collapsed: boolean; width: number; position: string; overview: boolean };

  const _subscribers = new Set<(v: SidebarState) => void>();
  const _current: SidebarState = { collapsed: false, width: 240, position: 'left', overview: false };

  function subscribe(fn: (v: SidebarState) => void): () => void {
    fn(_current);
    _subscribers.add(fn);
    return () => { _subscribers.delete(fn); };
  }

  return { subscribe, setPosition: vi.fn() };
});

vi.mock('$lib/stores/preferences', () => ({
  userPrefs: { subscribe: prefsMock.subscribe },
  updatePrefs: prefsMock.updatePrefs,
}));

vi.mock('$lib/stores/sidebar', () => ({
  sidebarStore: sidebarMock,
}));

import SidebarSettings from '$lib/components/settings/SidebarSettings.svelte';

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('SidebarSettings', () => {
  beforeEach(() => {
    prefsMock.updatePrefs.mockReset();
    sidebarMock.setPosition.mockReset();
    prefsMock._set({
      sidebar: { defaultPanel: 'tree', collapseOnOpen: false, docTreeGrouping: 'type-first' },
    });
  });

  it('should call updatePrefs when a non-active default panel is selected', async () => {
    render(SidebarSettings);

    const panelGroup = screen.getByRole('group', { name: /default panel/i });
    const buttons = panelGroup.querySelectorAll('button[type="button"]');
    expect(buttons.length).toBe(3); // Doc tree, Filter, Overview

    // "Doc tree" (tree) is active by default.
    const treeBtn = screen.getByRole('button', { name: /^doc tree$/i });
    expect(treeBtn).toHaveAttribute('aria-pressed', 'true');

    // Click Filter.
    const filterBtn = screen.getByRole('button', { name: /^filter$/i });
    await fireEvent.click(filterBtn);

    expect(prefsMock.updatePrefs).toHaveBeenCalledWith('sidebar', { defaultPanel: 'filter' });
  });

  it('should call updatePrefs when a non-active doc tree grouping is selected', async () => {
    render(SidebarSettings);

    const groupingGroup = screen.getByRole('group', { name: /doc tree grouping/i });
    const buttons = groupingGroup.querySelectorAll('button[type="button"]');
    expect(buttons.length).toBe(3); // Type-first, Folder-first, Flat

    // "Type-first" is active by default.
    const typefirstBtn = screen.getByRole('button', { name: /^type-first$/i });
    expect(typefirstBtn).toHaveAttribute('aria-pressed', 'true');

    // Click Flat.
    const flatBtn = screen.getByRole('button', { name: /^flat$/i });
    await fireEvent.click(flatBtn);

    expect(prefsMock.updatePrefs).toHaveBeenCalledWith('sidebar', { docTreeGrouping: 'flat' });
  });
});
