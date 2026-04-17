<!--
  TabStub.svelte — generic stand-in for the four ProviderDrawer child tabs
  (Memory / Permissions / MCP / Agents) when testing the drawer shell.

  Renders a single deterministic node with `data-testid="tab-stub"` and the
  tab name passed via the `label` prop, so assertions can confirm which tab
  was mounted without pulling in the real tab's API calls.
-->
<script lang="ts">
  interface Props {
    label?: string;
    project?: string;
    providerId?: string;
    providerName?: string;
  }
  const { label = 'unnamed' }: Props = $props();
</script>

<div data-testid="tab-stub" data-tab={label}>tab:{label}</div>
