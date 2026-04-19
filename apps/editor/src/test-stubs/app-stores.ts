/**
 * app-stores.ts
 *
 * Vitest stub for SvelteKit's `$app/stores` virtual module.
 *
 * Provides a minimal `page` store that returns static params/url values
 * suitable for unit tests. Components that derive active state from the
 * URL (e.g. DocTree's currentPath) will see an empty params object, which
 * is the correct baseline for tests that don't exercise URL-dependent logic.
 */

import { readable } from 'svelte/store';

export const page = readable({
  url: new URL('http://localhost/'),
  params: {} as Record<string, string>,
  route: { id: null },
  status: 200,
  error: null,
  data: {},
  state: {},
  form: undefined,
});

export const navigating = readable(null);
export const updated = readable(false);
