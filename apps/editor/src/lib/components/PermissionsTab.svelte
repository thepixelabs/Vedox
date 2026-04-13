<!--
  PermissionsTab.svelte — Claude .claude/settings.json permissions editor.

  We treat the permissions block as opaque JSON for v1 — the schema is still
  shifting upstream, so a structured editor would lock us into a frozen
  shape. The textarea validates JSON on every keystroke and highlights the
  parse error inline; Save is disabled until the JSON parses cleanly.
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

  let raw = $state('{}');
  let etag = $state('');
  let dirty = $state(false);
  let saving = $state(false);
  let parseError = $state<string | null>(null);
  let saveError = $state<string | null>(null);
  let conflict = $state(false);
  let loaded = $state(false);

  function tryParse(): Record<string, unknown> | null {
    try {
      const v = JSON.parse(raw) as unknown;
      if (typeof v !== 'object' || v === null || Array.isArray(v)) {
        parseError = 'Permissions must be a JSON object.';
        return null;
      }
      parseError = null;
      return v as Record<string, unknown>;
    } catch (e) {
      parseError = e instanceof Error ? e.message : 'Invalid JSON';
      return null;
    }
  }

  async function load() {
    if (providerId !== 'claude') return;
    await providerDrawer.loadClaudeConfig(project);
    const cfg = providerDrawer.claudeConfig;
    if (cfg) {
      raw = JSON.stringify(cfg.permissions.raw, null, 2);
      etag = cfg.permissions.etag;
      dirty = false;
      conflict = false;
      loaded = true;
      tryParse();
    }
  }

  onMount(() => {
    void load();
  });

  function onInput(e: Event) {
    raw = (e.target as HTMLTextAreaElement).value;
    dirty = true;
    tryParse();
  }

  async function save() {
    const parsed = tryParse();
    if (!parsed) return;
    saving = true;
    saveError = null;
    try {
      const res = await api.putClaudePermissions(project, parsed, etag);
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
</script>

{#if providerId !== 'claude'}
  <ProviderCapabilityNotice
    type="not-supported"
    {providerName}
    capabilityName="Permissions"
  />
{:else}
  <div class="perms-tab">
    <header class="perms-tab__header">
      <h3 class="perms-tab__title">settings.json permissions</h3>
      <p class="perms-tab__subtitle">
        Tool allow/deny rules for Claude Code in this project.
      </p>
    </header>

    {#if conflict}
      <div class="conflict-banner" role="alert">
        <strong>File changed externally.</strong>
        <span>settings.json was rewritten by another tool.</span>
        <div class="conflict-banner__actions">
          <button type="button" class="btn btn--primary" onclick={reloadFromServer}>Reload</button>
          <button type="button" class="btn" onclick={() => (conflict = false)}>Keep my edits</button>
        </div>
      </div>
    {/if}

    {#if !loaded && providerDrawer.drawerLoading}
      <p class="status">Loading permissions…</p>
    {:else}
      <textarea
        class="perms-tab__textarea"
        class:invalid={!!parseError}
        value={raw}
        oninput={onInput}
        spellcheck="false"
        aria-label="settings.json content"
        aria-invalid={!!parseError}
      ></textarea>

      {#if parseError}
        <p class="error" role="alert">{parseError}</p>
      {/if}

      <footer class="perms-tab__footer">
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
          disabled={!dirty || saving || !!parseError}
          onclick={save}
        >
          {saving ? 'Saving…' : 'Save'}
        </button>
      </footer>
    {/if}
  </div>
{/if}

<style>
  .perms-tab { display: flex; flex-direction: column; gap: var(--space-3); height: 100%; }
  .perms-tab__title { margin: 0; font-size: var(--text-base); font-weight: 600; color: var(--text-1); }
  .perms-tab__subtitle { margin: var(--space-1) 0 0 0; font-size: var(--text-xs); color: var(--text-3); }
  .perms-tab__textarea {
    flex: 1;
    min-height: 280px;
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
  .perms-tab__textarea:focus-visible {
    outline: 2px solid var(--accent-solid);
    outline-offset: 2px;
  }
  .perms-tab__textarea.invalid {
    border-color: var(--error);
  }
  .perms-tab__footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
  }
  .muted { font-size: var(--text-xs); color: var(--text-4); }
  .error { font-size: var(--text-xs); color: var(--error); margin: 0; }
  .status { font-size: var(--text-sm); color: var(--text-3); padding: var(--space-4); }

  .conflict-banner {
    display: flex; flex-wrap: wrap; align-items: center;
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
  }
  .btn:hover:not(:disabled) { background: var(--surface-4); color: var(--text-1); }
  .btn:focus-visible { outline: 2px solid var(--accent-solid); outline-offset: 2px; }
  .btn:disabled { opacity: 0.5; cursor: not-allowed; }
  .btn--primary {
    background: var(--accent-solid);
    border-color: var(--accent-solid);
    color: var(--accent-contrast);
  }
</style>
