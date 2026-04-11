<!--
  SidebarOverview.svelte

  Project overview panel shown at ultra-wide viewports (>= 2560px) when
  overview mode is enabled in the sidebar store.

  v1 stubs — all sections show placeholder content. Future phases will wire
  these to reactive stores backed by the project API.
-->

<script lang="ts">
  // Stub overview panel — real data wired in a future phase.
</script>

<div class="sidebar-overview">
  <section class="overview-section">
    <h3 class="overview-heading">Pinned</h3>
    <p class="overview-empty">No pinned docs yet</p>
  </section>
  <section class="overview-section">
    <h3 class="overview-heading">Recent</h3>
    <p class="overview-empty">Open a document to see history</p>
  </section>
  <section class="overview-section">
    <h3 class="overview-heading">Tags</h3>
    <p class="overview-empty">No tags yet</p>
  </section>
  <section class="overview-section">
    <h3 class="overview-heading">Git</h3>
    <p class="overview-status">Up to date</p>
  </section>
</div>

<style>
  .sidebar-overview {
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .overview-heading {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-text-muted);
    margin: 0 0 8px 0;
    font-weight: 500;
  }

  .overview-empty,
  .overview-status {
    font-size: 12px;
    color: var(--color-text-muted);
    margin: 0;
  }

  .overview-status {
    color: var(--color-success, #38a169);
  }
</style>
