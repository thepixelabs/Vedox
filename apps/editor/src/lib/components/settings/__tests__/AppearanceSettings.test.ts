/**
 * AppearanceSettings.test.ts
 *
 * Tests for AppearanceSettings.svelte — the theme, font-size, and accent color
 * settings panel.
 *
 * Design contract:
 *   - Clicking a theme card calls themeStore.setTheme() and updatePrefs().
 *   - Clicking a font-size segment button calls updatePrefs() with the correct
 *     size and reflects the new active state in aria-pressed.
 *   - Selecting a non-default font size persists the preference in the store
 *     (observable via aria-pressed on re-render).
 */

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';

// ---------------------------------------------------------------------------
// Mocks — declared before importing the component under test.
// vi.hoisted runs before ANY module import, so we hand-roll the minimal Svelte
// store contract (subscribe / notify) rather than importing `writable`.
//
// AppearanceSettings.svelte calls localStorage.getItem() synchronously inside
// $state() initialisers (runs at module-import time). We must stub localStorage
// before the component module is resolved. vi.hoisted guarantees this order.
// ---------------------------------------------------------------------------

// Patch localStorage before any module loads.
vi.hoisted(() => {
  const _store: Record<string, string> = {};
  const localStorageStub = {
    getItem: (key: string) => _store[key] ?? null,
    setItem: (key: string, value: string) => { _store[key] = value; },
    removeItem: (key: string) => { delete _store[key]; },
    clear: () => { for (const k in _store) delete _store[k]; },
    get length() { return Object.keys(_store).length; },
    key: (i: number) => Object.keys(_store)[i] ?? null,
  };
  // Overwrite localStorage on both window and global so jsdom sees it.
  Object.defineProperty(globalThis, 'localStorage', {
    value: localStorageStub,
    writable: true,
    configurable: true,
  });
});

type AppearanceState = {
  appearance: {
    theme: string;
    accentColor: string;
    fontSize: string;
    lineHeight: string;
    measure: string;
    density: string;
    treeGrouping: string;
  };
};

// Minimal hand-rolled store for userPrefs.
const prefsMock = vi.hoisted(() => {
  const _subscribers = new Set<(v: AppearanceState) => void>();
  let _current: AppearanceState = {
    appearance: {
      theme: 'graphite', accentColor: '', fontSize: '16px',
      lineHeight: 'normal', measure: 'default', density: 'comfortable',
      treeGrouping: 'type-first',
    },
  };

  function subscribe(fn: (v: AppearanceState) => void): () => void {
    fn(_current);
    _subscribers.add(fn);
    return () => { _subscribers.delete(fn); };
  }

  function _set(next: AppearanceState): void {
    _current = next;
    _subscribers.forEach((fn) => fn(_current));
  }

  return { subscribe, _set, updatePrefs: vi.fn() };
});

// Minimal hand-rolled store for themeStore.
const themeStoreMock = vi.hoisted(() => {
  const _subscribers = new Set<(v: string) => void>();
  let _current = 'graphite';

  function subscribe(fn: (v: string) => void): () => void {
    fn(_current);
    _subscribers.add(fn);
    return () => { _subscribers.delete(fn); };
  }

  function setTheme(t: string): void {
    _current = t;
    _subscribers.forEach((fn) => fn(_current));
  }

  return { subscribe, setTheme: vi.fn(setTheme) };
});

// Minimal hand-rolled stores for densityStore and readingStore.
const densityStoreMock = vi.hoisted(() => {
  const _subscribers = new Set<(v: string) => void>();
  const _current = 'comfortable';
  function subscribe(fn: (v: string) => void): () => void {
    fn(_current);
    _subscribers.add(fn);
    return () => { _subscribers.delete(fn); };
  }
  return { subscribe, setDensity: vi.fn() };
});

const readingStoreMock = vi.hoisted(() => {
  const _subscribers = new Set<(v: string) => void>();
  const _current = 'default';
  function subscribe(fn: (v: string) => void): () => void {
    fn(_current);
    _subscribers.add(fn);
    return () => { _subscribers.delete(fn); };
  }
  return { subscribe, setMeasure: vi.fn() };
});

vi.mock('$lib/theme/store', () => ({
  themeStore: themeStoreMock,
  densityStore: densityStoreMock,
}));

vi.mock('$lib/stores/reading', () => ({
  readingStore: readingStoreMock,
}));

vi.mock('$lib/stores/preferences', () => ({
  userPrefs: { subscribe: prefsMock.subscribe },
  updatePrefs: prefsMock.updatePrefs,
}));

// ThemePreviewCard and FontPicker are child components with their own
// dependencies. Replace with the project-standard stub.
vi.mock('$lib/components/ThemePreviewCard.svelte', () =>
  import('../../../../test-stubs/TabStub.svelte'),
);
vi.mock('$lib/components/FontPicker.svelte', () =>
  import('../../../../test-stubs/TabStub.svelte'),
);

import AppearanceSettings from '$lib/components/settings/AppearanceSettings.svelte';

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('AppearanceSettings', () => {
  beforeEach(() => {
    prefsMock.updatePrefs.mockReset();
    themeStoreMock.setTheme.mockClear();
    prefsMock._set({
      appearance: {
        theme: 'graphite', accentColor: '', fontSize: '16px',
        lineHeight: 'normal', measure: 'default', density: 'comfortable',
        treeGrouping: 'type-first',
      },
    });
  });

  it('should call themeStore.setTheme and updatePrefs when a theme card is clicked', async () => {
    render(AppearanceSettings);

    const themeGrid = document.querySelector('[role="radiogroup"]');
    expect(themeGrid).toBeTruthy();
    const themeCardButtons = themeGrid!.querySelectorAll('button[type="button"]');
    // 5 themes: graphite, eclipse, ember, paper, solar.
    expect(themeCardButtons.length).toBe(5);

    // Click the second button (eclipse, index 1).
    await fireEvent.click(themeCardButtons[1]!);

    expect(themeStoreMock.setTheme).toHaveBeenCalledWith('eclipse');
    expect(prefsMock.updatePrefs).toHaveBeenCalledWith('appearance', { theme: 'eclipse' });
  });

  it('should call updatePrefs with the selected font size when a size segment button is clicked', async () => {
    render(AppearanceSettings);

    const fontSizeGroup = screen.getByRole('group', { name: /font size/i });
    const sizeButtons = fontSizeGroup.querySelectorAll('button[type="button"]');
    expect(sizeButtons.length).toBe(3);

    // Click "Large" (value '18px').
    const largeButton = screen.getByRole('button', { name: /^large$/i });
    await fireEvent.click(largeButton);

    expect(prefsMock.updatePrefs).toHaveBeenCalledWith('appearance', { fontSize: '18px' });
  });

  it('should reflect active font size from the store via aria-pressed', () => {
    prefsMock._set({
      appearance: {
        theme: 'graphite', accentColor: '', fontSize: '13px',
        lineHeight: 'normal', measure: 'default', density: 'comfortable',
        treeGrouping: 'type-first',
      },
    });

    render(AppearanceSettings);

    // Scope queries to the font-size group to disambiguate from the "Default"
    // button in the Reading width group (which also renders a "Default" option).
    const fontSizeGroup = screen.getByRole('group', { name: /font size/i });
    const smallBtn = fontSizeGroup.querySelector('button[aria-pressed="true"]');
    const defaultBtn = fontSizeGroup.querySelector('button[aria-pressed="false"]');

    expect(smallBtn).toHaveTextContent('Small');
    expect(defaultBtn).toHaveTextContent('Default');
    expect(smallBtn).toHaveAttribute('aria-pressed', 'true');
  });
});
