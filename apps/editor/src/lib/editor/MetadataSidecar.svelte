<!--
  MetadataSidecar.svelte

  Right-rail metadata panel for the document editor. Shows git metadata
  (last modified, contributors) and parsed frontmatter fields.

  Visibility: hidden below 1920px viewport width via CSS media query in the
  parent Editor.svelte wrapper. This component renders unconditionally and
  lets CSS handle the show/hide so there is no JS overhead.
-->

<script lang="ts">
  import { frontmatterStore } from '$lib/stores/frontmatter-schema';

  interface Props {
    projectId?: string;
    docPath?: string;
  }

  let { projectId = '', docPath = '' }: Props = $props();

  const metadata = frontmatterStore;

  function formatDate(iso: string | null): string {
    if (!iso) return '\u2014';
    const d = new Date(iso);
    if (isNaN(d.getTime())) return '\u2014';
    return d.toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  }

  function relativize(iso: string | null): string {
    if (!iso) return '';
    const diff = Date.now() - new Date(iso).getTime();
    const days = Math.floor(diff / 86400000);
    if (days === 0) return 'today';
    if (days === 1) return 'yesterday';
    if (days < 30) return `${days}d ago`;
    if (days < 365) return `${Math.floor(days / 30)}mo ago`;
    return `${Math.floor(days / 365)}y ago`;
  }
</script>

<aside class="metadata-sidecar" aria-label="Document metadata">
  <!-- Last modified -->
  <section class="sidecar-section">
    <h3 class="sidecar-heading">Last modified</h3>
    <p class="sidecar-value tabular">
      {#if $metadata.lastModifiedRaw}
        <span title={$metadata.lastModifiedRaw}>{formatDate($metadata.lastModifiedRaw)}</span>
        <span class="sidecar-secondary"> &middot; {relativize($metadata.lastModifiedRaw)}</span>
      {:else}
        <span class="sidecar-empty">&mdash;</span>
      {/if}
    </p>
  </section>

  <!-- Contributors -->
  {#if $metadata.contributors.length > 0}
  <section class="sidecar-section">
    <h3 class="sidecar-heading">Contributors</h3>
    <ul class="sidecar-contributors">
      {#each $metadata.contributors.slice(0, 5) as name}
        <li class="sidecar-value">{name}</li>
      {/each}
    </ul>
  </section>
  {/if}

  <!-- Frontmatter fields -->
  {#if $metadata.fields.length > 0}
  <section class="sidecar-section">
    <h3 class="sidecar-heading">Properties</h3>
    <dl class="sidecar-fields">
      {#each $metadata.fields as field}
        <div class="sidecar-field-row">
          <dt class="sidecar-field-key">{field.key}</dt>
          <dd class="sidecar-field-value">
            {#if field.type === 'array' && Array.isArray(field.value)}
              <span class="sidecar-tags">
                {#each field.value as tag}
                  <span class="sidecar-tag">{tag}</span>
                {/each}
              </span>
            {:else}
              {String(field.value ?? '\u2014')}
            {/if}
          </dd>
        </div>
      {/each}
    </dl>
  </section>
  {/if}

  <!-- Edit on GitHub (stub) -->
  {#if docPath}
  <section class="sidecar-section">
    <h3 class="sidecar-heading">Source</h3>
    <p class="sidecar-value sidecar-secondary">Edit on GitHub &#8599;</p>
  </section>
  {/if}
</aside>

<style>
  .metadata-sidecar {
    width: var(--sidecar-width, 280px);
    height: 100%;
    overflow-y: auto;
    padding: 20px 16px;
    border-left: 1px solid var(--color-border);
    background: var(--color-surface-elevated);
    display: flex;
    flex-direction: column;
    gap: 20px;
    font-size: 12px;
    color: var(--color-text-secondary);
    flex-shrink: 0;
  }

  .sidecar-section {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .sidecar-heading {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-text-muted);
    margin: 0;
    font-weight: 500;
  }

  .sidecar-value {
    margin: 0;
    color: var(--color-text-secondary);
  }

  .sidecar-secondary {
    color: var(--color-text-muted);
  }

  .sidecar-empty {
    color: var(--color-text-muted);
  }

  .tabular {
    font-feature-settings: "tnum" 1, "zero" 1;
  }

  .sidecar-contributors {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .sidecar-fields {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .sidecar-field-row {
    display: grid;
    grid-template-columns: 80px 1fr;
    gap: 8px;
    align-items: baseline;
  }

  .sidecar-field-key {
    font-size: 10px;
    color: var(--color-text-muted);
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }

  .sidecar-field-value {
    margin: 0;
    color: var(--color-text-secondary);
    word-break: break-word;
  }

  .sidecar-tags {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
  }

  .sidecar-tag {
    padding: 1px 6px;
    border-radius: 9999px;
    background: color-mix(in srgb, var(--color-accent) 12%, transparent);
    color: var(--color-accent);
    font-size: 10px;
  }
</style>
