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
 */

import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte({ hot: false })],
  test: {
    // jsdom is required for Tiptap / ProseMirror DOM operations.
    environment: 'jsdom',

    // Include all test files in the lib directory.
    include: ['src/**/*.test.ts', 'src/**/*.spec.ts'],

    globals: true,

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
    // Required so Vitest can resolve .svelte files from test files.
    conditions: ['browser', 'import', 'module', 'default']
  }
});
