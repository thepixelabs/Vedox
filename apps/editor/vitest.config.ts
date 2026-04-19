/**
 * vitest.config.ts
 *
 * Vitest configuration for @vedox/editor.
 *
 * The round-trip golden-file tests require a DOM environment (jsdom) because
 * Tiptap / ProseMirror create and manipulate DOM nodes during parsing and
 * serialization. The jsdom environment provides the necessary globals
 * (document, window, localStorage) without a real browser.
 *
 * These tests BLOCK merge if they fail — they are the enforcement mechanism
 * for the round-trip fidelity invariant (Phase 1 CTO ruling).
 *
 * Component tests (ProviderDrawer, InstallAgent) mount real Svelte components
 * with @testing-library/svelte. They depend on the aliases + setup file below
 * to stub SvelteKit's virtual `$app/*` modules (which only exist when running
 * under `vite dev`/`build`) and to register jest-dom matchers.
 */

import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { svelteTesting } from '@testing-library/svelte/vite';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const here = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  // svelteTesting() adds the `browser` resolve condition, registers an auto
  // cleanup hook, and most importantly marks @testing-library/svelte and
  // @testing-library/svelte-core as `ssr.noExternal` so the Svelte plugin
  // compiles their `.svelte.js` files (which use runes) instead of letting
  // Vite ship them raw — without this we get `rune_outside_svelte` at render.
  plugins: [svelte({ hot: false }), svelteTesting()],
  test: {
    // jsdom is required for Tiptap / ProseMirror DOM operations and for any
    // component test that touches the document tree.
    environment: 'jsdom',

    // Include all test files under src/.
    include: ['src/**/*.test.ts', 'src/**/*.spec.ts'],

    globals: true,

    // Setup file registers jest-dom matchers + cleans up between tests.
    setupFiles: ['./src/test-setup.ts'],

    // Longer timeout for Mermaid render calls in integration tests.
    testTimeout: 15000,

    // Coverage configuration (optional, but wired for CI).
    coverage: {
      provider: 'v8',
      include: ['src/lib/editor/**/*.ts', 'src/lib/editor/**/*.svelte'],
      exclude: ['src/lib/editor/__tests__/**']
    },

    // Round-trip tests are CI blockers — treat all failures as hard failures.
    // No retry on failure; flaky tests must be fixed, not hidden.
    retry: 0
  },
  resolve: {
    // SvelteKit auto-resolves `$lib`, `$styles`, and `$app/*` in
    // `vite dev`/`build`; vitest does not load the kit plugin, so we wire up
    // these aliases manually. Use the array form so prefix matches like
    // `$lib/components/Foo.svelte` rewrite to `<root>/src/lib/components/Foo.svelte`.
    alias: [
      { find: /^\$lib\/(.*)$/, replacement: path.resolve(here, 'src/lib') + '/$1' },
      { find: /^\$lib$/,        replacement: path.resolve(here, 'src/lib') },
      { find: /^\$styles\/(.*)$/, replacement: path.resolve(here, 'src/styles') + '/$1' },
      { find: '$app/environment', replacement: path.resolve(here, 'src/test-stubs/app-environment.ts') },
      { find: '$app/stores',     replacement: path.resolve(here, 'src/test-stubs/app-stores.ts') },
      { find: '$app/navigation', replacement: path.resolve(here, 'src/test-stubs/app-navigation.ts') },
    ],
    // Required so Vitest can resolve .svelte files from test files.
    conditions: ['browser', 'import', 'module', 'default']
  }
});
