/**
 * app-environment.ts
 *
 * Vitest stub for SvelteKit's `$app/environment` virtual module. SvelteKit
 * supplies these flags during `vite dev`/`build`; under Vitest we are running
 * in jsdom so we pin everything to the values that make the most sense for
 * unit tests:
 *
 *   browser  = true   — jsdom provides document/window/localStorage
 *   dev      = true   — surfaces dev-only branches in the code under test
 *   building = false  — never rendering for the prerender pass
 *   version  = ''     — no kit version to surface
 */

export const browser = true;
export const dev = true;
export const building = false;
export const version = '';
