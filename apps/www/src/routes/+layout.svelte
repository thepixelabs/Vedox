<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import Header from '$lib/components/Header.svelte';
	import Footer from '$lib/components/Footer.svelte';
	import { initAnalytics, initScrollDepth, initSectionVisibility } from '$lib/analytics';

	let { children } = $props();

	onMount(() => {
		initAnalytics();
		const cleanupScroll = initScrollDepth();
		const cleanupSections = initSectionVisibility();
		return () => {
			cleanupScroll();
			cleanupSections();
		};
	});
</script>

<Header />
<main>
	{@render children?.()}
</main>
<Footer />
