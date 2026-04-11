import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	preprocess: vitePreprocess(),

	kit: {
		// Static adapter — the entire site prerenders to flat HTML + assets.
		// No Node runtime, no SSR server, no edge functions.
		adapter: adapter({
			pages: 'build',
			assets: 'build',
			fallback: undefined,
			precompress: false,
			strict: true
		}),
		prerender: {
			handleHttpError: 'fail',
			handleUnseenRoutes: 'warn',
			entries: ['*', '/robots.txt', '/sitemap.xml']
		},
		alias: {
			$content: 'src/lib/content.ts'
		}
	}
};

export default config;
