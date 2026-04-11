/**
 * reading.ts — reading measure store
 * Controls the --reading-measure CSS variable on :root.
 * Three widths: narrow (64ch), default (68ch), wide (80ch).
 * Persists to localStorage at vedox:reading-measure.
 */
import { writable } from 'svelte/store';
import { browser } from '$app/environment';

export type ReadingMeasure = 'narrow' | 'default' | 'wide';

const KEY = 'vedox:reading-measure';
const DEFAULT: ReadingMeasure = 'default';

const MEASURE_MAP: Record<ReadingMeasure, string> = {
  narrow: 'var(--measure-narrow, 64ch)',
  default: 'var(--measure-default, 68ch)',
  wide: 'var(--measure-wide, 80ch)',
};

function getStored(): ReadingMeasure {
  if (!browser) return DEFAULT;
  const v = localStorage.getItem(KEY);
  if (v === 'narrow' || v === 'default' || v === 'wide') return v;
  return DEFAULT;
}

function syncCss(m: ReadingMeasure): void {
  if (!browser) return;
  document.documentElement.style.setProperty('--reading-measure', MEASURE_MAP[m]);
}

function createReadingStore() {
  const { subscribe, set, update } = writable<ReadingMeasure>(getStored());

  // Sync on init
  syncCss(getStored());

  function setMeasure(m: ReadingMeasure): void {
    syncCss(m);
    if (browser) {
      try { localStorage.setItem(KEY, m); } catch { /* quota / private-mode */ }
    }
    set(m);
  }

  function cycle(): void {
    update(current => {
      const order: ReadingMeasure[] = ['narrow', 'default', 'wide'];
      const next = order[(order.indexOf(current) + 1) % order.length];
      setMeasure(next);
      return next;
    });
  }

  return { subscribe, setMeasure, cycle };
}

export const readingStore = createReadingStore();
