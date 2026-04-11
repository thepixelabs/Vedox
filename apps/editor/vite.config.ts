import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [sveltekit()],
	server: {
		// Bind to loopback only — matches Vedox network policy.
		host: '127.0.0.1',
		port: 5151,
		strictPort: true,
		proxy: {
			// Proxy /api/* to the Go backend on 127.0.0.1:5150.
			// Both ports are chosen to be memorable (Van Halen "5150" + adjacent)
			// and uncommon in the wild to avoid collisions with other dev tools.
			// changeOrigin is false: we are on the same host, no Origin spoofing needed.
			'/api': {
				target: 'http://127.0.0.1:5150',
				changeOrigin: false,
			},
		},
	},
});
