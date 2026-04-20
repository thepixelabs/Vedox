<script lang="ts">
  /**
   * FolderPicker.svelte — inline directory browser for selecting a path.
   *
   * Features:
   *   - Lists subdirectories for the current path.
   *   - "Up" button to navigate to the parent directory.
   *   - Click a folder to enter it.
   *   - "Select" button to choose the current path.
   */

  import { onMount, untrack } from 'svelte';
  import { api, type BrowseResponse } from '$lib/api/client';

  interface Props {
    /** Initial path to start browsing from. Defaults to user's home. */
    initialPath?: string;
    /** Called when the user confirms their selection. */
    onSelect: (path: string) => void;
    /** Called when the user cancels the picker. */
    onCancel: () => void;
  }

  let { initialPath = '', onSelect, onCancel }: Props = $props();

  // untrack: initialPath is a one-time seed. The component navigates away from
  // it immediately in onMount; prop changes after mount are not meaningful here.
  let currentPath: string = $state(untrack(() => initialPath));
  let parentPath: string = $state('');
  let directories: Array<{ name: string; path: string }> = $state([]);
  let isLoading: boolean = $state(true);
  let error: string | null = $state(null);

  async function loadPath(path?: string) {
    isLoading = true;
    error = null;
    try {
      const res = await api.browse(path);
      currentPath = res.path;
      parentPath = res.parent;
      directories = res.directories;
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to load directory';
    } finally {
      isLoading = false;
    }
  }

  onMount(() => {
    loadPath(currentPath || undefined);
  });

  function handleSelect() {
    onSelect(currentPath);
  }
</script>

<div class="folder-picker" role="dialog" aria-label="Select a folder">
  <div class="folder-picker__header">
    <div class="folder-picker__current-path" title={currentPath}>
      {currentPath}
    </div>
  </div>

  <div class="folder-picker__list-container">
    {#if isLoading}
      <div class="folder-picker__status">
        <span class="folder-picker__spinner"></span>
        Loading...
      </div>
    {:else if error}
      <div class="folder-picker__status folder-picker__status--error">
        {error}
        <button class="folder-picker__retry" type="button" onclick={() => loadPath(currentPath)}>
          Retry
        </button>
      </div>
    {:else}
      <ul class="folder-picker__list">
        {#if parentPath}
          <li class="folder-picker__item">
            <button
              class="folder-picker__dir-btn folder-picker__dir-btn--parent"
              type="button"
              onclick={() => loadPath(parentPath)}
            >
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <polyline points="15 18 9 12 15 6"/>
              </svg>
              .. (Parent Directory)
            </button>
          </li>
        {/if}

        {#each directories as dir (dir.path)}
          <li class="folder-picker__item">
            <button
              class="folder-picker__dir-btn"
              type="button"
              onclick={() => loadPath(dir.path)}
            >
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
              </svg>
              {dir.name}
            </button>
          </li>
        {/each}

        {#if directories.length === 0 && !parentPath}
          <li class="folder-picker__status">No directories found.</li>
        {/if}
      </ul>
    {/if}
  </div>

  <div class="folder-picker__footer">
    <button class="folder-picker__btn folder-picker__btn--secondary" type="button" onclick={onCancel}>
      Cancel
    </button>
    <button
      class="folder-picker__btn folder-picker__btn--primary"
      type="button"
      onclick={handleSelect}
      disabled={isLoading}
    >
      Select Folder
    </button>
  </div>
</div>

<style>
  .folder-picker {
    display: flex;
    flex-direction: column;
    background-color: var(--color-surface);
    height: 500px; /* Taller shape for "hamburgery" look */
    width: 100%;
    overflow: hidden;
    /* Removed border and radius as they are now provided by the wrapper card */
  }

  .folder-picker__header {
    padding: var(--space-3) var(--space-4);
    background-color: var(--color-surface-elevated);
    border-bottom: 1px solid var(--color-border);
  }

  .folder-picker__current-path {
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    color: var(--color-text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    direction: rtl; /* show the end of the path if it overflows */
    text-align: left;
  }

  .folder-picker__list-container {
    flex: 1;
    overflow-y: auto;
    padding: var(--space-1);
  }

  .folder-picker__list {
    list-style: none;
    padding: 0;
    margin: 0;
  }

  .folder-picker__item {
    margin: 1px 0;
  }

  .folder-picker__dir-btn {
    width: 100%;
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-2) var(--space-3);
    background: none;
    border: none;
    border-radius: var(--radius-sm);
    color: var(--color-text-primary);
    font-size: var(--font-size-sm);
    text-align: left;
    cursor: pointer;
    transition: background-color 80ms var(--ease-out);
  }

  .folder-picker__dir-btn:hover {
    background-color: var(--color-surface-overlay);
  }

  .folder-picker__dir-btn--parent {
    color: var(--color-text-secondary);
    font-style: italic;
  }

  .folder-picker__dir-btn svg {
    color: var(--color-text-muted);
    flex-shrink: 0;
  }

  .folder-picker__status {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    gap: var(--space-3);
    color: var(--color-text-muted);
    font-size: var(--font-size-sm);
    padding: var(--space-8);
  }

  .folder-picker__status--error {
    color: var(--color-error);
    text-align: center;
  }

  .folder-picker__spinner {
    width: 20px;
    height: 20px;
    border: 2px solid var(--color-border);
    border-top-color: var(--color-accent);
    border-radius: 50%;
    animation: spin 800ms linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .folder-picker__retry {
    background: none;
    border: 1px solid var(--color-error);
    color: var(--color-error);
    padding: var(--space-1) var(--space-3);
    border-radius: var(--radius-sm);
    cursor: pointer;
    font-size: var(--font-size-xs);
  }

  .folder-picker__footer {
    display: flex;
    justify-content: flex-end;
    gap: var(--space-3);
    padding: var(--space-3) var(--space-4);
    background-color: var(--color-surface-elevated);
    border-top: 1px solid var(--color-border);
  }

  .folder-picker__btn {
    padding: var(--space-2) var(--space-4);
    font-size: var(--font-size-sm);
    font-weight: 500;
    border-radius: var(--radius-sm);
    cursor: pointer;
    transition: background-color var(--duration-fast) var(--ease-out), color var(--duration-fast) var(--ease-out), border-color var(--duration-fast) var(--ease-out);
  }

  .folder-picker__btn--primary {
    background-color: var(--color-accent);
    color: var(--color-text-inverse);
    border: none;
  }

  .folder-picker__btn--primary:hover:not(:disabled) {
    background-color: var(--color-accent-hover);
  }

  .folder-picker__btn--secondary {
    background: none;
    border: 1px solid var(--color-border);
    color: var(--color-text-secondary);
  }

  .folder-picker__btn--secondary:hover {
    border-color: var(--color-border-strong);
    color: var(--color-text-primary);
  }

  .folder-picker__btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
</style>
