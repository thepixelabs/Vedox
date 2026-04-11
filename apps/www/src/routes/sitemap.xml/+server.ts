import { site } from '$lib/content';

export const prerender = true;

export function GET() {
	const urls = [site.url + '/'];
	const xml = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${urls
	.map(
		(u) =>
			`  <url><loc>${u}</loc><changefreq>weekly</changefreq><priority>1.0</priority></url>`
	)
	.join('\n')}
</urlset>
`;
	return new Response(xml, {
		headers: { 'content-type': 'application/xml; charset=utf-8' },
	});
}
