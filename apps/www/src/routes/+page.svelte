<script lang="ts">
	import Hero from '$lib/components/Hero.svelte';
	import Pitch from '$lib/components/Pitch.svelte';
	import Pillars from '$lib/components/Pillars.svelte';
	import EditorShowcase from '$lib/components/EditorShowcase.svelte';
	import FeatureGrid from '$lib/components/FeatureGrid.svelte';
	import SocialProof from '$lib/components/SocialProof.svelte';
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
<Workflow />

<div class="skeptics-roadmap">
	<img
		class="brass-bleed"
		src="/vedox-logo-07-brass-registry-rembg.png"
		alt=""
		aria-hidden="true"
		loading="lazy"
		decoding="async"
		width="480"
		height="480"
	/>
	<div class="sections-layer">
		<FAQ />
		<Roadmap />
	</div>
</div>

<Waitlist />

<style>
	.skeptics-roadmap {
		position: relative;
		overflow: hidden;
	}
	.brass-bleed {
		position: absolute;
		right: -5%;
		top: 42%;
		transform: translateY(-50%);
		width: min(38%, 480px);
		height: auto;
		opacity: 0.085;
		mix-blend-mode: luminosity;
		filter: grayscale(30%);
		pointer-events: none;
		user-select: none;
		z-index: 0;
	}
	.sections-layer {
		position: relative;
		z-index: 1;
	}
	@media (max-width: 1023px) {
		.brass-bleed {
			width: min(45%, 360px);
			opacity: 0.06;
			right: -8%;
		}
	}
	@media (max-width: 767px) {
		.brass-bleed {
			display: none;
		}
	}
</style>

