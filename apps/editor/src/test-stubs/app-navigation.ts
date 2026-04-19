/**
 * app-navigation.ts
 *
 * Vitest stub for SvelteKit's `$app/navigation` virtual module.
 *
 * All navigation functions are replaced with vi.fn() stubs so tests can
 * assert that navigation was triggered without actually changing the URL.
 * Tests that need to inspect calls must import `goto` from this module
 * (or use vi.mocked(goto) after vi.mock('$app/navigation')).
 */

import { vi } from 'vitest';

export const goto = vi.fn();
export const invalidate = vi.fn();
export const invalidateAll = vi.fn();
export const preloadCode = vi.fn();
export const preloadData = vi.fn();
export const pushState = vi.fn();
export const replaceState = vi.fn();
export const beforeNavigate = vi.fn();
export const afterNavigate = vi.fn();
export const onNavigate = vi.fn();
