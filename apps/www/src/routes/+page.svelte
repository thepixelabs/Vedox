<script lang="ts">
	import Hero from '$lib/components/Hero.svelte';
	import Pitch from '$lib/components/Pitch.svelte';
	import Pillars from '$lib/components/Pillars.svelte';
	import EditorShowcase from '$lib/components/EditorShowcase.svelte';
	import FeatureGrid from '$lib/components/FeatureGrid.svelte';
	import SocialProof from '$lib/components/SocialProof.svelte';
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
<SocialProof />
<Comparison />
<section id="install" class="interstitial">
	<div class="container interstitial-inner">
		<p class="comment">// if the table convinced you, you already know what to do.</p>
		<InstallCommand location="compare" />
	</div>
</section>
<Workflow />
<FAQ />
<Roadmap />
<Waitlist />

<style>
	.interstitial {
		border-top: 1px solid var(--color-border);
		border-bottom: 1px solid var(--color-border);
		padding: var(--space-12) 0 !important;
		margin-top: var(--space-8);
	}
	.interstitial-inner {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
		align-items: flex-start;
	}
	.comment {
		font-family: var(--font-mono);
		font-size: var(--font-size-sm);
		color: var(--color-text-muted);
	}
</style>
