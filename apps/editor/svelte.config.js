import adapter from '@sveltejs/adapter-auto';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	preprocess: vitePreprocess(),

	kit: {
		adapter: adapter(),

		// CSP is enforced in two places:
		//   1. hooks.server.ts — HTTP header on every dev server response (primary)
		//   2. app.html <meta http-equiv> — defence-in-depth for static vedox build output
		//
		// We do NOT use SvelteKit's built-in csp config because it requires nonce
		// or hash generation which conflicts with our SPA (ssr=false) setup.
		// The manual approach in hooks.server.ts is explicit and auditable.

		alias: {
			$styles: 'src/styles',
		}
	}
};

export default config;
