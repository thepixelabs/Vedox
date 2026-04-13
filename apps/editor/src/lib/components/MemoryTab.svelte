<!--
  MemoryTab.svelte — Claude .claude/CLAUDE.md memory editor.

  Loads the memory body via providerDrawer.loadClaudeConfig() and lets the
  user edit it with simple textarea + Save. ETag conflict handling: on 409
  we surface a yellow banner offering "Reload" (drop local edits) or "Keep"
  (keep local edits but stop saving until the user re-saves with the new
  etag).
-->

<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError, type DetectedProviderId } from '$lib/api/client';
  import { providerDrawer } from '$lib/stores/providerConfig.svelte';
  import ProviderCapabilityNotice from './ProviderCapabilityNotice.svelte';

  interface Props {
    project: string;
    providerId: DetectedProviderId;
    providerName: string;
  }

  const { project, providerId, providerName }: Props = $props();

  let content = $state('');
  let etag = $state('');
  let dirty = $state(false);
  let saving = $state(false);
  let saveError = $state<string | null>(null);
  let conflict = $state(false);
  let loaded = $state(false);

  async function load() {
    if (providerId !== 'claude') return;
    await providerDrawer.loadClaudeConfig(project);
    const cfg = providerDrawer.claudeConfig;
    if (cfg) {
      content = cfg.memory.content;
      etag = cfg.memory.etag;
      dirty = false;
      conflict = false;
      loaded = true;
    }
  }

  onMount(() => {
    void load();
  });

  function onInput(e: Event) {
    content = (e.target as HTMLTextAreaElement).value;
    dirty = true;
  }

  async function save() {
    saving = true;
    saveError = null;
    try {
      const res = await api.putClaudeMemory(project, content, etag);
      etag = res.etag;
      dirty = false;
      conflict = false;
    } catch (err) {
      if (err instanceof ApiError && (err.status === 409 || err.code === 'conflict')) {
        conflict = true;
      } else {
        saveError = err instanceof Error ? err.message : 'Save failed';
      }
    } finally {
      saving = false;
    }
  }

  async function reloadFromServer() {
    conflict = false;
    await load();
  }

  function dismissConflict() {
    // Keep local edits — user must Save again, which will still 409 unless
    // they reload first. We just hide the banner so they can review.
    conflict = false;
  }
</script>

{#if providerId !== 'claude'}
  <ProviderCapabilityNotice
    type="not-supported"
    {providerName}
    capabilityName="Memory"
  />
{:else}
  <div class="memory-tab">
    <header class="memory-tab__header">
      <div>
        <h3 class="memory-tab__title">CLAUDE.md memory</h3>
        <p class="memory-tab__subtitle">
          Persistent context loaded into every Claude Code session for this project.
        </p>
      </div>
    </header>

    {#if conflict}
      <div class="conflict-banner" role="alert">
        <strong>File changed externally.</strong>
        <span>Another tool wrote to CLAUDE.md while you were editing.</span>
        <div class="conflict-banner__actions">
          <button type="button" class="btn btn--primary" onclick={reloadFromServer}>Reload</button>
          <button type="button" class="btn" onclick={dismissConflict}>Keep my edits</button>
        </div>
      </div>
    {/if}

    {#if !loaded && providerDrawer.drawerLoading}
      <p class="status">Loading memory…</p>
    {:else}
      <textarea
        class="memory-tab__textarea"
        value={content}
        oninput={onInput}
        spellcheck="false"
        placeholder="# Project memory&#10;&#10;Persistent notes Claude should remember…"
        aria-label="CLAUDE.md content"
      ></textarea>

      <footer class="memory-tab__footer">
        {#if saveError}
          <span class="error">{saveError}</span>
        {:else if dirty}
          <span class="muted">Unsaved changes</span>
        {:else}
          <span class="muted">Saved</span>
        {/if}
        <button
          type="button"
          class="btn btn--primary"
          disabled={!dirty || saving}
          onclick={save}
        >
          {saving ? 'Saving…' : 'Save'}
        </button>
      </footer>
    {/if}
  </div>
{/if}

<style>
  .memory-tab {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    height: 100%;
  }
  .memory-tab__header { display: flex; justify-content: space-between; align-items: flex-start; }
  .memory-tab__title {
    margin: 0;
    font-size: var(--text-base);
    font-weight: 600;
    color: var(--text-1);
  }
  .memory-tab__subtitle {
    margin: var(--space-1) 0 0 0;
    font-size: var(--text-xs);
    color: var(--text-3);
  }
  .memory-tab__textarea {
    flex: 1;
    min-height: 280px;
    width: 100%;
    padding: var(--space-3);
    background: var(--surface-1);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-md);
    color: var(--text-1);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    line-height: var(--leading-relaxed);
    resize: vertical;
  }
  .memory-tab__textarea:focus-visible {
    outline: 2px solid var(--accent-solid);
    outline-offset: 2px;
    border-color: var(--accent-solid);
  }
  .memory-tab__footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
  }
  .muted { font-size: var(--text-xs); color: var(--text-4); }
  .error { font-size: var(--text-xs); color: var(--error); }
  .status { font-size: var(--text-sm); color: var(--text-3); padding: var(--space-4); }

  .conflict-banner {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-2) var(--space-3);
    padding: var(--space-3) var(--space-4);
    border: 1px solid oklch(from var(--warning) l c h / 0.5);
    background: oklch(from var(--warning) l c h / 0.12);
    border-radius: var(--radius-md);
    color: var(--text-1);
    font-size: var(--text-xs);
  }
  .conflict-banner strong { color: var(--warning); }
  .conflict-banner__actions { display: flex; gap: var(--space-2); margin-left: auto; }

  .btn {
    padding: var(--space-1) var(--space-3);
    border-radius: var(--radius-sm);
    border: 1px solid var(--border-default);
    background: var(--surface-3);
    color: var(--text-2);
    font-family: var(--font-body);
    font-size: var(--text-xs);
    font-weight: 500;
    cursor: pointer;
    transition: background-color var(--duration-fast, 120ms) ease;
  }
  .btn:hover:not(:disabled) { background: var(--surface-4); color: var(--text-1); }
  .btn:focus-visible { outline: 2px solid var(--accent-solid); outline-offset: 2px; }
  .btn:disabled { opacity: 0.5; cursor: not-allowed; }
  .btn--primary {
    background: var(--accent-solid);
    border-color: var(--accent-solid);
    color: var(--accent-contrast);
  }
  .btn--primary:hover:not(:disabled) {
    background: var(--accent-solid-hover, var(--accent-solid));
    color: var(--accent-contrast);
  }
</style>
