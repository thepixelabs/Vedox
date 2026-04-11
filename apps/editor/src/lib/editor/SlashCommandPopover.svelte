<!--
  SlashCommandPopover.svelte

  Global popover that listens for window events dispatched by the
  SlashCommand Tiptap extension. Renders a grouped, filtered list of
  content-insertion commands. Keyboard navigation is forwarded from
  ProseMirror via `vedox-slash-nav` events.

  Events consumed:
    - vedox-slash-open  → show popover at coords with items
    - vedox-slash-update → refresh items for new query
    - vedox-slash-close → hide
    - vedox-slash-nav   → handle ArrowDown/Up/Enter/Escape
-->

<script lang="ts">
  import { onMount } from 'svelte';
  import type { SlashCommand } from './slash-commands/registry';
  import type { SlashCommandEventDetail } from './extensions/SlashCommand';

  // ---------------------------------------------------------------------------
  // State
  // ---------------------------------------------------------------------------

  let visible = $state(false);
  let items = $state<SlashCommand[]>([]);
  let selectedIndex = $state(0);
  let coords = $state<{ top: number; left: number }>({ top: 0, left: 0 });
  let onSelectFn = $state<((cmd: SlashCommand) => void) | null>(null);
  let onCloseFn = $state<(() => void) | null>(null);

  // Group items by `group` for section headers
  const grouped = $derived(() => {
    const groups: Array<{ name: string; items: SlashCommand[] }> = [];
    let current: { name: string; items: SlashCommand[] } | null = null;
    for (const item of items) {
      if (!current || current.name !== item.group) {
        current = { name: item.group, items: [] };
        groups.push(current);
      }
      current.items.push(item);
    }
    return groups;
  });

  // Flat index of items (for arrow navigation)
  const flatItems = $derived(items);

  // ---------------------------------------------------------------------------
  // Event handlers
  // ---------------------------------------------------------------------------

  function handleOpen(e: Event): void {
    const detail = (e as CustomEvent<SlashCommandEventDetail>).detail;
    items = detail.items;
    coords = detail.coords;
    onSelectFn = detail.onSelect;
    onCloseFn = detail.onClose;
    selectedIndex = 0;
    visible = true;
  }

  function handleUpdate(e: Event): void {
    const detail = (e as CustomEvent<{ query: string; items: SlashCommand[] }>).detail;
    items = detail.items;
    selectedIndex = Math.min(selectedIndex, Math.max(0, items.length - 1));
  }

  function handleClose(): void {
    visible = false;
    items = [];
    onSelectFn = null;
    onCloseFn = null;
  }

  function handleNav(e: Event): void {
    if (!visible) return;
    const detail = (e as CustomEvent<{ key: string; accept: () => void }>).detail;
    const key = detail.key;

    if (key === 'ArrowDown') {
      selectedIndex = (selectedIndex + 1) % Math.max(1, flatItems.length);
      detail.accept();
    } else if (key === 'ArrowUp') {
      selectedIndex =
        (selectedIndex - 1 + Math.max(1, flatItems.length)) %
        Math.max(1, flatItems.length);
      detail.accept();
    } else if (key === 'Enter') {
      const selected = flatItems[selectedIndex];
      if (selected && onSelectFn) {
        onSelectFn(selected);
      }
      detail.accept();
    } else if (key === 'Escape') {
      if (onCloseFn) onCloseFn();
      detail.accept();
    }
  }

  // ---------------------------------------------------------------------------
  // Lifecycle
  // ---------------------------------------------------------------------------

  onMount(() => {
    window.addEventListener('vedox-slash-open', handleOpen);
    window.addEventListener('vedox-slash-update', handleUpdate);
    window.addEventListener('vedox-slash-close', handleClose);
    window.addEventListener('vedox-slash-nav', handleNav);

    return () => {
      window.removeEventListener('vedox-slash-open', handleOpen);
      window.removeEventListener('vedox-slash-update', handleUpdate);
      window.removeEventListener('vedox-slash-close', handleClose);
      window.removeEventListener('vedox-slash-nav', handleNav);
    };
  });

  function handleClick(cmd: SlashCommand): void {
    if (onSelectFn) onSelectFn(cmd);
  }

  function flatIndexOf(cmd: SlashCommand): number {
    return flatItems.indexOf(cmd);
  }
</script>

{#if visible && items.length > 0}
  <div
    class="slash-popover"
    style:left="{coords.left}px"
    style:top="{coords.top}px"
    role="listbox"
    aria-label="Slash command menu"
  >
    {#each grouped() as group (group.name)}
      <div class="slash-popover__group">
        <div class="slash-popover__group-header">{group.name}</div>
        {#each group.items as cmd (cmd.id)}
          {@const idx = flatIndexOf(cmd)}
          <button
            type="button"
            class="slash-popover__item"
            class:slash-popover__item--selected={idx === selectedIndex}
            role="option"
            aria-selected={idx === selectedIndex}
            onmousedown={(e) => {
              e.preventDefault();
              handleClick(cmd);
            }}
            onmouseenter={() => (selectedIndex = idx)}
          >
            <span class="slash-popover__icon">{@html cmd.icon}</span>
            <span class="slash-popover__text">
              <span class="slash-popover__label">{cmd.label}</span>
              <span class="slash-popover__desc">{cmd.description}</span>
            </span>
          </button>
        {/each}
      </div>
    {/each}
  </div>
{:else if visible && items.length === 0}
  <div
    class="slash-popover slash-popover--empty"
    style:left="{coords.left}px"
    style:top="{coords.top}px"
    role="status"
  >
    <div class="slash-popover__empty">No matching commands</div>
  </div>
{/if}

<style>
  .slash-popover {
    position: fixed;
    z-index: var(--z-popover, 60);
    min-width: 280px;
    max-width: 340px;
    max-height: 360px;
    overflow-y: auto;
    background: var(--surface-4, #1e1e1e);
    border: 1px solid var(--border-default, rgba(255, 255, 255, 0.1));
    border-radius: var(--radius-md, 8px);
    box-shadow: var(--shadow-overlay, 0 8px 24px rgba(0, 0, 0, 0.4));
    padding: 4px 0;
    font-family: var(--font-body, system-ui, sans-serif);
    font-size: 13px;
  }

  .slash-popover--empty {
    min-width: 180px;
  }

  .slash-popover__empty {
    padding: 10px 14px;
    color: var(--text-3, rgba(255, 255, 255, 0.5));
    font-size: 12px;
    text-align: center;
  }

  .slash-popover__group {
    padding: 4px 0;
  }

  .slash-popover__group + .slash-popover__group {
    border-top: 1px solid var(--border-hairline, rgba(255, 255, 255, 0.06));
    margin-top: 2px;
    padding-top: 6px;
  }

  .slash-popover__group-header {
    padding: 4px 14px 2px;
    font-size: 10px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--text-3, rgba(255, 255, 255, 0.4));
  }

  .slash-popover__item {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 6px 14px;
    background: transparent;
    border: none;
    cursor: pointer;
    text-align: left;
    color: var(--text-1, rgba(255, 255, 255, 0.9));
    font: inherit;
    transition: background-color 80ms ease;
  }

  .slash-popover__item:hover,
  .slash-popover__item--selected {
    background: var(--accent-subtle, rgba(59, 130, 246, 0.15));
  }

  .slash-popover__item--selected {
    color: var(--accent-text, var(--text-1, #fff));
  }

  .slash-popover__icon {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    flex-shrink: 0;
    color: var(--text-2, rgba(255, 255, 255, 0.7));
  }

  .slash-popover__item--selected .slash-popover__icon {
    color: var(--accent-solid, #3b82f6);
  }

  .slash-popover__text {
    display: flex;
    flex-direction: column;
    gap: 1px;
    min-width: 0;
  }

  .slash-popover__label {
    font-size: 13px;
    font-weight: 500;
    color: inherit;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .slash-popover__desc {
    font-size: 11px;
    color: var(--text-3, rgba(255, 255, 255, 0.5));
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .slash-popover:focus-visible {
    outline: 2px solid var(--accent-solid, #3b82f6);
    outline-offset: 2px;
  }
</style>
