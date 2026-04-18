/**
 * test-setup.ts
 *
 * Global Vitest setup. Loaded before every test file via vitest.config.ts
 * `setupFiles`.
 *
 *   - Registers @testing-library/jest-dom custom matchers (toBeInTheDocument,
 *     toHaveTextContent, …) so component assertions read like specs.
 *   - Runs @testing-library/svelte's `cleanup()` after each test to unmount
 *     anything still attached to document.body and avoid cross-test leakage.
 */

import '@testing-library/jest-dom/vitest';
import { afterEach } from 'vitest';
import { cleanup } from '@testing-library/svelte';

afterEach(() => {
  cleanup();
});
