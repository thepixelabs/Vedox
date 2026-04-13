<script lang="ts">
	import { track } from '$lib/analytics';
	import { site } from '$lib/content';

	interface Props {
		location?: string;
	}
	let { location = 'hero' }: Props = $props();

	let copied = $state(false);
	let timer: ReturnType<typeof setTimeout> | null = null;

	async function copy() {
		try {
			await navigator.clipboard.writeText(site.installCommand);
			copied = true;
			track('Install Copy', { location });
			if (timer) clearTimeout(timer);
			timer = setTimeout(() => (copied = false), 1800);
		} catch {
			copied = false;
		}
	}
</script>

<div class="cmd-wrapper">
	<button
		type="button"
		class="cmd"
		onclick={copy}
		aria-label="Copy install command to clipboard"
	>
		<span class="prompt" aria-hidden="true">$</span>
		<code>{site.installCommand}</code>
		<span class="badge" aria-live="polite">
			{#if copied}
				Copied
			{:else}
				Copy
			{/if}
		</span>
	</button>
</div>

<style>
	.cmd-wrapper {
		display: inline-flex;
		align-items: center;
	}
	.cmd {
		display: inline-flex;
		align-items: center;
		gap: var(--space-3);
		padding: 14px 18px;
		font-family: var(--font-mono);
		font-size: var(--font-size-base);
		color: var(--color-text-primary);
		background: var(--color-surface-elevated);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-lg);
		cursor: pointer;
		box-shadow: var(--shadow-md);
		transition:
			border-color 120ms ease,
			background 120ms ease,
			transform 120ms ease;
	}
	.cmd:hover {
		border-color: var(--color-accent);
		background: var(--color-surface-overlay);
	}
	.cmd:active {
		transform: translateY(1px);
	}
	.prompt {
		color: var(--color-accent);
		user-select: none;
	}
	code {
		white-space: nowrap;
	}
	.badge {
		font-family: var(--font-sans);
		font-size: var(--font-size-xs);
		color: var(--color-text-inverse);
		background: var(--color-accent);
		padding: 3px 8px;
		border-radius: var(--radius-sm);
		min-width: 52px;
		text-align: center;
	}
</style>
