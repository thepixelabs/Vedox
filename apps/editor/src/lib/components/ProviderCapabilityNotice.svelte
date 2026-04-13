<!--
  ProviderCapabilityNotice.svelte — shared empty-state for unsupported or
  pending capabilities inside the Provider Drawer.

  Two flavours:
    type="not-supported"  → this provider does not expose this concern at all
                            (e.g. Codex has no .CLAUDE.md memory file).
    type="pending"        → the adapter is on the roadmap but not implemented
                            in this build (e.g. Gemini MCP).
-->

<script lang="ts">
  interface Props {
    type: 'not-supported' | 'pending';
    providerName: string;
    capabilityName: string;
  }

  const { type, providerName, capabilityName }: Props = $props();

  const heading = $derived(
    type === 'not-supported'
      ? `${capabilityName} is not supported by ${providerName}`
      : `${capabilityName} adapter is on the way`,
  );

  const body = $derived(
    type === 'not-supported'
      ? `${providerName} does not expose a ${capabilityName.toLowerCase()} concern. Switch to a provider that does, or pick a different tab.`
      : `Editing ${capabilityName.toLowerCase()} for ${providerName} is not yet wired up in Vedox. We are tracking it on the provider-drawer roadmap.`,
  );
</script>

<div class="notice notice--{type}" role="status">
  <div class="notice__icon" aria-hidden="true">
    {#if type === 'not-supported'}
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="12" cy="12" r="9" />
        <line x1="5.6" y1="5.6" x2="18.4" y2="18.4" />
      </svg>
    {:else}
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="12" cy="12" r="9" />
        <polyline points="12 7 12 12 15 14" />
      </svg>
    {/if}
  </div>
  <div class="notice__body">
    <p class="notice__heading">{heading}</p>
    <p class="notice__detail">{body}</p>
  </div>
</div>

<style>
  .notice {
    display: flex;
    align-items: flex-start;
    gap: var(--space-3);
    padding: var(--space-5);
    border: 1px dashed var(--border-default);
    border-radius: var(--radius-md);
    background: var(--surface-2);
    color: var(--text-3);
    margin: var(--space-4) 0;
  }

  .notice--pending {
    border-color: oklch(from var(--warning) l c h / 0.5);
    background: oklch(from var(--warning) l c h / 0.06);
  }

  .notice__icon {
    flex-shrink: 0;
    color: var(--text-4);
  }

  .notice--pending .notice__icon {
    color: var(--warning);
  }

  .notice__body {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .notice__heading {
    margin: 0;
    font-size: var(--text-sm);
    font-weight: 600;
    color: var(--text-1);
    line-height: var(--leading-snug);
  }

  .notice__detail {
    margin: 0;
    font-size: var(--text-xs);
    color: var(--text-3);
    line-height: var(--leading-relaxed);
  }
</style>
