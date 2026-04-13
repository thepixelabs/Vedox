<!--
  AgentsTab.svelte — Claude .claude/agents/*.md subagent definitions.

  Master/detail layout: list of agents on the left, inline edit form on the
  right when one is selected. "+ New" creates a draft agent that posts on
  first save.
-->

<script lang="ts">
  import { onMount } from 'svelte';
  import {
    api,
    ApiError,
    type AgentDetail,
    type AgentSummary,
    type DetectedProviderId,
  } from '$lib/api/client';
  import ProviderCapabilityNotice from './ProviderCapabilityNotice.svelte';

  interface Props {
    project: string;
    providerId: DetectedProviderId;
    providerName: string;
  }

  const { project, providerId, providerName }: Props = $props();

  let agents = $state<AgentSummary[]>([]);
  let loading = $state(false);
  let listError = $state<string | null>(null);

  // Editor state — null means no agent selected.
  let editing = $state<(AgentDetail & { isNew: boolean }) | null>(null);
  let saving = $state(false);
  let saveError = $state<string | null>(null);
  let conflict = $state(false);

  async function reloadList() {
    if (providerId !== 'claude') return;
    loading = true;
    listError = null;
    try {
      const res = await api.listAgents(project);
      agents = res.agents;
    } catch (err) {
      listError = err instanceof Error ? err.message : 'Failed to list agents';
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    void reloadList();
  });

  async function selectAgent(filename: string) {
    saveError = null;
    conflict = false;
    try {
      const detail = await api.getAgent(project, filename);
      editing = { ...detail, isNew: false };
    } catch (err) {
      saveError = err instanceof Error ? err.message : 'Failed to load agent';
    }
  }

  function newAgent() {
    saveError = null;
    conflict = false;
    editing = {
      filename: '',
      name: '',
      description: '',
      version: '1.0.0',
      body: '# Agent prompt\n\n',
      etag: '',
      isNew: true,
    };
  }

  function cancel() {
    editing = null;
    saveError = null;
    conflict = false;
  }

  async function save() {
    if (!editing) return;
    saving = true;
    saveError = null;
    try {
      if (editing.isNew) {
        if (!editing.filename.trim()) {
          saveError = 'Filename is required.';
          saving = false;
          return;
        }
        const res = await api.createAgent(project, {
          filename: editing.filename,
          name: editing.name,
          description: editing.description,
          version: editing.version,
          body: editing.body,
        });
        editing = { ...editing, etag: res.etag, isNew: false };
      } else {
        const res = await api.putAgent(project, editing.filename, {
          name: editing.name,
          description: editing.description,
          version: editing.version,
          body: editing.body,
          etag: editing.etag,
        });
        editing = { ...editing, etag: res.etag };
      }
      await reloadList();
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

  async function reloadEditing() {
    if (!editing || editing.isNew) return;
    conflict = false;
    await selectAgent(editing.filename);
  }

  async function deleteAgent() {
    if (!editing || editing.isNew) {
      editing = null;
      return;
    }
    if (!confirm(`Delete agent ${editing.filename}? This cannot be undone.`)) return;
    try {
      await api.deleteAgent(project, editing.filename);
      editing = null;
      await reloadList();
    } catch (err) {
      saveError = err instanceof Error ? err.message : 'Delete failed';
    }
  }
</script>

{#if providerId !== 'claude'}
  <ProviderCapabilityNotice
    type="not-supported"
    {providerName}
    capabilityName="Agents"
  />
{:else}
  <div class="agents-tab">
    <header class="agents-tab__header">
      <div>
        <h3 class="agents-tab__title">Subagents</h3>
        <p class="agents-tab__subtitle">Markdown-defined personas Claude can spawn.</p>
      </div>
      <button type="button" class="btn" onclick={newAgent}>+ New agent</button>
    </header>

    {#if loading && agents.length === 0}
      <p class="status">Loading agents…</p>
    {:else if listError}
      <p class="error" role="alert">{listError}</p>
    {/if}

    <div class="agents-tab__layout">
      <ul class="agents-tab__list" role="list">
        {#each agents as a (a.filename)}
          <li>
            <button
              type="button"
              class="agent-row"
              class:agent-row--active={editing && !editing.isNew && editing.filename === a.filename}
              onclick={() => selectAgent(a.filename)}
            >
              <span class="agent-row__name">{a.name || a.filename}</span>
              <span class="agent-row__desc">{a.description || '—'}</span>
              <span class="agent-row__version">v{a.version}</span>
            </button>
          </li>
        {/each}
        {#if agents.length === 0 && !loading && !listError}
          <li class="empty">No agents defined yet.</li>
        {/if}
      </ul>

      {#if editing}
        <section class="agent-editor" aria-label="Agent editor">
          {#if conflict}
            <div class="conflict-banner" role="alert">
              <strong>File changed externally.</strong>
              <span>This agent file was rewritten by another tool.</span>
              <div class="conflict-banner__actions">
                <button type="button" class="btn btn--primary" onclick={reloadEditing}>Reload</button>
                <button type="button" class="btn" onclick={() => (conflict = false)}>Keep my edits</button>
              </div>
            </div>
          {/if}

          <label class="field">
            <span>Filename</span>
            <input
              type="text"
              bind:value={editing.filename}
              disabled={!editing.isNew}
              placeholder="my-agent.md"
            />
          </label>
          <label class="field">
            <span>Name</span>
            <input type="text" bind:value={editing.name} placeholder="Code Reviewer" />
          </label>
          <label class="field">
            <span>Description</span>
            <input
              type="text"
              bind:value={editing.description}
              placeholder="Reviews code for bugs and style issues"
            />
          </label>
          <label class="field">
            <span>Version</span>
            <input type="text" bind:value={editing.version} placeholder="1.0.0" />
          </label>
          <label class="field field--body">
            <span>Body (markdown)</span>
            <textarea
              bind:value={editing.body}
              spellcheck="false"
              rows="12"
            ></textarea>
          </label>

          <footer class="agent-editor__footer">
            {#if saveError}
              <span class="error">{saveError}</span>
            {/if}
            <div class="agent-editor__actions">
              {#if !editing.isNew}
                <button type="button" class="btn btn--danger" onclick={deleteAgent}>Delete</button>
              {/if}
              <button type="button" class="btn" onclick={cancel}>Cancel</button>
              <button type="button" class="btn btn--primary" disabled={saving} onclick={save}>
                {saving ? 'Saving…' : 'Save'}
              </button>
            </div>
          </footer>
        </section>
      {/if}
    </div>
  </div>
{/if}

<style>
  .agents-tab { display: flex; flex-direction: column; gap: var(--space-3); height: 100%; }
  .agents-tab__header { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--space-3); }
  .agents-tab__title { margin: 0; font-size: var(--text-base); font-weight: 600; color: var(--text-1); }
  .agents-tab__subtitle { margin: var(--space-1) 0 0 0; font-size: var(--text-xs); color: var(--text-3); }

  .agents-tab__layout { display: flex; flex-direction: column; gap: var(--space-3); flex: 1; min-height: 0; }

  .agents-tab__list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    max-height: 200px;
    overflow-y: auto;
  }

  .agent-row {
    display: grid;
    grid-template-columns: 1fr auto;
    grid-template-areas: "name version" "desc desc";
    gap: 2px var(--space-2);
    width: 100%;
    text-align: left;
    padding: var(--space-2) var(--space-3);
    background: var(--surface-2);
    border: 1px solid var(--border-hairline);
    border-radius: var(--radius-sm);
    cursor: pointer;
    color: inherit;
    font-family: inherit;
  }
  .agent-row:hover { background: var(--surface-3); }
  .agent-row:focus-visible { outline: 2px solid var(--accent-solid); outline-offset: 2px; }
  .agent-row--active {
    border-color: var(--accent-solid);
    background: var(--accent-subtle);
  }
  .agent-row__name {
    grid-area: name;
    font-size: var(--text-sm);
    font-weight: 600;
    color: var(--text-1);
  }
  .agent-row__version {
    grid-area: version;
    font-size: var(--text-2xs);
    color: var(--text-4);
    font-family: var(--font-mono);
  }
  .agent-row__desc {
    grid-area: desc;
    font-size: var(--text-xs);
    color: var(--text-3);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .empty { font-size: var(--text-xs); color: var(--text-4); padding: var(--space-3); list-style: none; }

  .agent-editor {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-3);
    background: var(--surface-1);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-md);
    flex: 1;
    overflow-y: auto;
  }
  .field { display: flex; flex-direction: column; gap: var(--space-1); }
  .field span {
    font-size: var(--text-2xs);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text-4);
    font-weight: 600;
  }
  .field input, .field textarea {
    padding: var(--space-2);
    background: var(--surface-2);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-sm);
    color: var(--text-1);
    font-family: var(--font-body);
    font-size: var(--text-sm);
  }
  .field--body textarea {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    line-height: var(--leading-relaxed);
    resize: vertical;
  }
  .field input:focus-visible, .field textarea:focus-visible {
    outline: 2px solid var(--accent-solid);
    outline-offset: 2px;
    border-color: var(--accent-solid);
  }
  .field input:disabled { opacity: 0.6; cursor: not-allowed; }

  .agent-editor__footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    margin-top: var(--space-2);
  }
  .agent-editor__actions { display: flex; gap: var(--space-2); margin-left: auto; }

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

  .error { font-size: var(--text-xs); color: var(--error); margin: 0; }
  .status { font-size: var(--text-sm); color: var(--text-3); padding: var(--space-4); }

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
  .btn--danger {
    color: var(--error);
    border-color: oklch(from var(--error) l c h / 0.4);
  }
  .btn--danger:hover:not(:disabled) {
    background: oklch(from var(--error) l c h / 0.12);
  }
</style>
