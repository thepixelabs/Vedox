<script lang="ts">
  /**
   * ProjectSwitcher — search-first project selector at the top of the sidebar.
   *
   * Design decisions:
   *   - Not a dropdown. A search input that filters inline — search-first UX.
   *   - Keyboard navigable: arrow keys move through results, Enter selects,
   *     Escape clears focus/closes.
   *   - ARIA combobox pattern: role="combobox" on input, role="listbox" on list.
   *   - Shows all projects when input is empty (acts as a quick switcher).
   */

  import { goto } from "$app/navigation";
  import { page } from "$app/stores";
  import type { Project } from "$lib/stores/projects";

  interface Props {
    projects: Project[];
  }

  let { projects }: Props = $props();

  let query = $state("");
  let activeIndex = $state(-1);
  let isOpen = $state(false);
  let inputEl: HTMLInputElement | undefined = $state();
  let listEl: HTMLUListElement | undefined = $state();

  const currentProjectId = $derived(
    ($page.params as Record<string, string>)["project"] ?? null
  );

  const filtered = $derived(
    query.trim() === ""
      ? projects
      : projects.filter(
          (p) =>
            p.name.toLowerCase().includes(query.toLowerCase()) ||
            p.id.toLowerCase().includes(query.toLowerCase())
        )
  );

  const currentProject = $derived(
    projects.find((p) => p.id === currentProjectId) ?? null
  );

  function open() {
    isOpen = true;
    activeIndex = -1;
  }

  function close() {
    isOpen = false;
    query = "";
    activeIndex = -1;
  }

  function select(project: Project) {
    close();
    goto(`/projects/${project.id}`);
  }

  function handleKeydown(event: KeyboardEvent) {
    if (!isOpen) {
      if (event.key === "ArrowDown" || event.key === "Enter") {
        open();
        event.preventDefault();
      }
      return;
    }

    switch (event.key) {
      case "ArrowDown":
        event.preventDefault();
        activeIndex = Math.min(activeIndex + 1, filtered.length - 1);
        scrollActiveIntoView();
        break;

      case "ArrowUp":
        event.preventDefault();
        activeIndex = Math.max(activeIndex - 1, 0);
        scrollActiveIntoView();
        break;

      case "Enter":
        event.preventDefault();
        if (activeIndex >= 0 && filtered[activeIndex]) {
          select(filtered[activeIndex]);
        } else if (filtered.length === 1) {
          select(filtered[0]);
        }
        break;

      case "Escape":
        event.preventDefault();
        close();
        inputEl?.blur();
        break;

      case "Tab":
        close();
        break;
    }
  }

  function scrollActiveIntoView() {
    if (!listEl || activeIndex < 0) return;
    const items = listEl.querySelectorAll('[role="option"]');
    const active = items[activeIndex] as HTMLElement | undefined;
    active?.scrollIntoView({ block: "nearest" });
  }

  function handleBlur(event: FocusEvent) {
    // Close only if focus left the entire switcher (not moved to list)
    const related = event.relatedTarget as Node | null;
    if (listEl && related && listEl.contains(related)) return;
    close();
  }

  const listboxId = "project-switcher-listbox";
  const inputId = "project-switcher-input";
</script>

<div class="switcher">
  <div class="switcher__input-wrap">
    <!-- Search icon -->
    <span class="switcher__search-icon" aria-hidden="true">
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="11" cy="11" r="8"/>
        <path d="m21 21-4.35-4.35"/>
      </svg>
    </span>

    <input
      bind:this={inputEl}
      id={inputId}
      type="text"
      class="switcher__input"
      placeholder={currentProject?.name ?? "Switch project…"}
      autocomplete="off"
      spellcheck="false"
      role="combobox"
      aria-expanded={isOpen}
      aria-controls={listboxId}
      aria-autocomplete="list"
      aria-activedescendant={activeIndex >= 0 ? `switcher-option-${activeIndex}` : undefined}
      bind:value={query}
      onfocus={open}
      onblur={handleBlur}
      onkeydown={handleKeydown}
    />
  </div>

  {#if isOpen && filtered.length > 0}
    <ul
      bind:this={listEl}
      id={listboxId}
      class="switcher__list"
      role="listbox"
      aria-label="Projects"
    >
      {#each filtered as project, i (project.id)}
        <li
          id="switcher-option-{i}"
          class="switcher__option"
          class:switcher__option--active={i === activeIndex}
          class:switcher__option--current={project.id === currentProjectId}
          role="option"
          aria-selected={project.id === currentProjectId}
          onmousedown={(e) => {
            e.preventDefault(); // prevent blur before click registers
            select(project);
          }}
        >
          <span class="switcher__option-name">{project.name}</span>
          {#if project.id === currentProjectId}
            <span class="switcher__option-check" aria-hidden="true">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                <path d="M20 6 9 17l-5-5"/>
              </svg>
            </span>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}

  {#if isOpen && filtered.length === 0}
    <div class="switcher__empty" role="status" aria-live="polite">
      No projects match "{query}"
    </div>
  {/if}
</div>

<style>
  .switcher {
    position: relative;
  }

  .switcher__input-wrap {
    position: relative;
    display: flex;
    align-items: center;
  }

  .switcher__search-icon {
    position: absolute;
    left: var(--space-2);
    display: flex;
    align-items: center;
    color: var(--color-text-muted);
    pointer-events: none;
  }

  .switcher__input {
    width: 100%;
    padding: var(--space-2) var(--space-2) var(--space-2) 28px;
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-text-primary);
    font-family: var(--font-sans);
    font-size: var(--font-size-sm);
    line-height: 1.4;
    transition: border-color var(--duration-fast) var(--ease-out), background-color var(--duration-fast) var(--ease-out);
  }

  .switcher__input::placeholder {
    color: var(--color-text-muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .switcher__input:focus {
    outline: none;
    border-color: var(--color-accent);
    background-color: var(--color-surface-base);
  }

  .switcher__list {
    position: absolute;
    top: calc(100% + 4px);
    left: 0;
    right: 0;
    z-index: 100;
    list-style: none;
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    box-shadow: var(--shadow-md);
    overflow-y: auto;
    max-height: 220px;
    padding: var(--space-1) 0;
  }

  .switcher__option {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-2) var(--space-3);
    cursor: pointer;
    color: var(--color-text-secondary);
    font-size: var(--font-size-sm);
    transition: background-color 80ms var(--ease-out), color 80ms var(--ease-out);
  }

  .switcher__option:hover,
  .switcher__option--active {
    background-color: var(--color-surface-overlay);
    color: var(--color-text-primary);
  }

  .switcher__option--current {
    color: var(--color-text-primary);
  }

  .switcher__option-name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .switcher__option-check {
    flex-shrink: 0;
    color: var(--color-accent);
    margin-left: var(--space-2);
  }

  .switcher__empty {
    position: absolute;
    top: calc(100% + 4px);
    left: 0;
    right: 0;
    padding: var(--space-3);
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-text-muted);
    font-size: var(--font-size-sm);
    text-align: center;
  }
</style>
