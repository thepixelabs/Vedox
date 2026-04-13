<!--
  McpTab.svelte — Claude .mcp.json server registry editor.

  Renders a read-only list of detected servers above the JSON editor so users
  can see at a glance what they have without scanning JSON. The textarea is
  the source of truth — Add/Remove buttons mutate the parsed object and
  re-serialise back into the textarea, which triggers re-validation.

  Gemini and Codex MCP editing is not yet wired up — those providers render
  the shared "pending" notice.
-->

<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError, type DetectedProviderId } from '$lib/api/client';
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
  let loading = $state(false);

  let parsedServers = $derived.by(() => {
    try {
      const v = JSON.parse(raw) as unknown;
      if (typeof v === 'object' && v !== null && !Array.isArray(v)) {
        return v as Record<string, unknown>;
      }
      return null;
    } catch {
      return null;
    }
  });

  let serverNames = $derived(parsedServers ? Object.keys(parsedServers) : []);

  function tryParse(): Record<string, unknown> | null {
    try {
      const v = JSON.parse(raw) as unknown;
      if (typeof v !== 'object' || v === null || Array.isArray(v)) {
        parseError = 'MCP servers must be a JSON object keyed by server name.';
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
    loading = true;
    try {
      const res = await api.getClaudeMCP(project);
      raw = JSON.stringify(res.servers ?? {}, null, 2);
      etag = res.etag;
      dirty = false;
      conflict = false;
      loaded = true;
      tryParse();
    } catch (err) {
      saveError = err instanceof Error ? err.message : 'Load failed';
    } finally {
      loading = false;
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

  function summarise(value: unknown): string {
    if (typeof value !== 'object' || value === null) return '—';
    const obj = value as Record<string, unknown>;
    if (typeof obj['url'] === 'string') return obj['url'];
    if (typeof obj['command'] === 'string') {
      const args = Array.isArray(obj['args']) ? ` ${(obj['args'] as unknown[]).join(' ')}` : '';
      return `${obj['command'] as string}${args}`;
    }
    return 'custom server';
  }

  function addServer() {
    const parsed = tryParse() ?? {};
    let i = 1;
    while (`new-server-${i}` in parsed) i++;
    parsed[`new-server-${i}`] = { command: 'npx', args: ['-y', '@example/mcp-server'] };
    raw = JSON.stringify(parsed, null, 2);
    dirty = true;
    tryParse();
  }

  function removeServer(name: string) {
    const parsed = tryParse();
    if (!parsed) return;
    delete parsed[name];
    raw = JSON.stringify(parsed, null, 2);
    dirty = true;
  }

  async function save() {
    const parsed = tryParse();
    if (!parsed) return;
    saving = true;
    saveError = null;
    try {
      const res = await api.putClaudeMCP(project, parsed, etag);
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
    type="pending"
    {providerName}
    capabilityName="MCP servers"
  />
{:else}
  <div class="mcp-tab">
    <header class="mcp-tab__header">
      <div>
        <h3 class="mcp-tab__title">.mcp.json servers</h3>
        <p class="mcp-tab__subtitle">Model Context Protocol servers Claude can call.</p>
      </div>
      <button type="button" class="btn" onclick={addServer} disabled={!loaded}>+ Add server</button>
    </header>

    {#if conflict}
      <div class="conflict-banner" role="alert">
        <strong>File changed externally.</strong>
        <span>.mcp.json was rewritten by another tool.</span>
        <div class="conflict-banner__actions">
          <button type="button" class="btn btn--primary" onclick={reloadFromServer}>Reload</button>
          <button type="button" class="btn" onclick={() => (conflict = false)}>Keep my edits</button>
        </div>
      </div>
    {/if}

    {#if loading && !loaded}
      <p class="status">Loading MCP servers…</p>
    {:else}
      {#if serverNames.length > 0 && parsedServers}
        <ul class="mcp-tab__list" role="list">
          {#each serverNames as name (name)}
            <li class="mcp-row">
              <div class="mcp-row__main">
                <span class="mcp-row__name">{name}</span>
                <span class="mcp-row__detail">{summarise(parsedServers[name])}</span>
              </div>
              <button
                type="button"
                class="mcp-row__remove"
                aria-label="Remove {name}"
                onclick={() => removeServer(name)}
              >×</button>
            </li>
          {/each}
        </ul>
      {:else if !parseError}
        <p class="empty">No MCP servers configured.</p>
      {/if}

      <details class="mcp-tab__json">
        <summary>Edit raw JSON</summary>
        <textarea
          class="mcp-tab__textarea"
          class:invalid={!!parseError}
          value={raw}
          oninput={onInput}
          spellcheck="false"
          aria-label=".mcp.json content"
          aria-invalid={!!parseError}
        ></textarea>
        {#if parseError}
          <p class="error" role="alert">{parseError}</p>
        {/if}
      </details>

      <footer class="mcp-tab__footer">
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
  .mcp-tab { display: flex; flex-direction: column; gap: var(--space-3); height: 100%; }
  .mcp-tab__header { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--space-3); }
  .mcp-tab__title { margin: 0; font-size: var(--text-base); font-weight: 600; color: var(--text-1); }
  .mcp-tab__subtitle { margin: var(--space-1) 0 0 0; font-size: var(--text-xs); color: var(--text-3); }

  .mcp-tab__list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .mcp-row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-3);
    background: var(--surface-2);
    border: 1px solid var(--border-hairline);
    border-radius: var(--radius-md);
  }
  .mcp-row__main { display: flex; flex-direction: column; gap: 2px; min-width: 0; flex: 1; }
  .mcp-row__name {
    font-size: var(--text-sm);
    font-weight: 600;
    color: var(--text-1);
    font-family: var(--font-mono);
  }
  .mcp-row__detail {
    font-size: var(--text-xs);
    color: var(--text-3);
    font-family: var(--font-mono);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .mcp-row__remove {
    background: none;
    border: 1px solid var(--border-default);
    color: var(--text-3);
    width: 24px; height: 24px;
    border-radius: var(--radius-sm);
    cursor: pointer;
    font-size: 16px;
    line-height: 1;
  }
  .mcp-row__remove:hover { color: var(--error); border-color: var(--error); }
  .mcp-row__remove:focus-visible { outline: 2px solid var(--accent-solid); outline-offset: 2px; }

  .empty { font-size: var(--text-xs); color: var(--text-4); margin: 0; padding: var(--space-3) 0; }

  .mcp-tab__json summary {
    cursor: pointer;
    font-size: var(--text-xs);
    color: var(--text-3);
    padding: var(--space-1) 0;
  }
  .mcp-tab__json summary:focus-visible { outline: 2px solid var(--accent-solid); outline-offset: 2px; }
  .mcp-tab__textarea {
    width: 100%;
    min-height: 200px;
    margin-top: var(--space-2);
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
  .mcp-tab__textarea.invalid { border-color: var(--error); }
  .mcp-tab__textarea:focus-visible { outline: 2px solid var(--accent-solid); outline-offset: 2px; }

  .mcp-tab__footer {
    display: flex; align-items: center; justify-content: space-between; gap: var(--space-3);
    margin-top: auto;
  }
  .muted { font-size: var(--text-xs); color: var(--text-4); }
  .error { font-size: var(--text-xs); color: var(--error); margin: var(--space-1) 0 0 0; }
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
