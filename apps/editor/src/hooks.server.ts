/**
 * hooks.server.ts — SvelteKit server hooks for Vedox editor.
 *
 * Responsibilities:
 *   1. Content-Security-Policy header on every response (primary enforcement)
 *   2. Strict transport and framing headers
 *
 * The CSP in app.html is a defence-in-depth fallback for the static build
 * output. This hook is the authoritative CSP for the dev server.
 */

import type { Handle } from "@sveltejs/kit";

const CSP =
  "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; object-src 'none'; base-uri 'self';";

export const handle: Handle = async ({ event, resolve }) => {
  const response = await resolve(event);

  // Content-Security-Policy — primary enforcement for the dev server
  response.headers.set("Content-Security-Policy", CSP);

  // Additional security headers
  response.headers.set("X-Frame-Options", "DENY");
  response.headers.set("X-Content-Type-Options", "nosniff");
  response.headers.set("Referrer-Policy", "strict-origin-when-cross-origin");

  return response;
};
