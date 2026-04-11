<script lang="ts">
	import Hero from '$lib/components/Hero.svelte';
	import Pitch from '$lib/components/Pitch.svelte';
	import Pillars from '$lib/components/Pillars.svelte';
	import EditorShowcase from '$lib/components/EditorShowcase.svelte';
	import FeatureGrid from '$lib/components/FeatureGrid.svelte';
	import Comparison from '$lib/components/Comparison.svelte';
	import InstallCommand from '$lib/components/InstallCommand.svelte';
	import Workflow from '$lib/components/Workflow.svelte';
	import FAQ from '$lib/components/FAQ.svelte';
	import Roadmap from '$lib/components/Roadmap.svelte';
	import Waitlist from '$lib/components/Waitlist.svelte';
	import { seo, site, jsonLd } from '$lib/content';
</script>

<svelte:head>
	<title>{seo.title}</title>
	<meta name="description" content={seo.description} />
	<meta name="keywords" content={seo.keywords.join(', ')} />
	<link rel="canonical" href={site.url} />

	<meta property="og:type" content="website" />
	<meta property="og:url" content={site.url} />
	<meta property="og:title" content={seo.title} />
	<meta property="og:description" content={seo.description} />
	<meta property="og:image" content={`${site.url}${seo.ogImage}`} />

	<meta name="twitter:card" content="summary_large_image" />
	<meta name="twitter:site" content={seo.twitterHandle} />
	<meta name="twitter:title" content={seo.title} />
	<meta name="twitter:description" content={seo.description} />
	<meta name="twitter:image" content={`${site.url}${seo.ogImage}`} />

	<!-- eslint-disable-next-line svelte/no-at-html-tags -->
	{@html '<script type="application/ld+json">' +
		JSON.stringify(jsonLd).replace(/</g, '\\u003c') +
		'<' +
		'/script>'}
</svelte:head>

<Hero />
<Pitch />
<Pillars />
<EditorShowcase />
<FeatureGrid />
<Comparison />
<div class="mid-cta"><InstallCommand location="compare" /></div>
<Workflow />
<FAQ />
<Roadmap />
<Waitlist />

<style>
	.mid-cta {
		display: flex;
		justify-content: center;
		padding: var(--space-10) 0;
	}
</style>
