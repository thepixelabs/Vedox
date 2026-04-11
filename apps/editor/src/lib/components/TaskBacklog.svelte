<script lang="ts">
  /**
   * TaskBacklog — per-project flat task list (VDX-P2-H).
   *
   * Features:
   *   - Inline task creation (Enter or "+" button)
   *   - Status cycling by clicking the status chip: todo → in-progress → done → todo
   *   - Drag-to-reorder via fractional indexing; renumber envelope handled
   *   - Single-click delete (no modal — it's just a task)
   *   - Inline API error messages; no toast library
   *   - Empty state: plain text, no heavy EmptyState component
   *
   * Uses StatusList for rendering; maps Task → StatusListItem.
   */

  import { onMount } from 'svelte'
  import { StatusList } from '$lib/components/StatusList'
  import type { StatusListItem } from '$lib/components/StatusList'
  import { api, ApiError, type Task } from '$lib/api/client'

  // ── Props ─────────────────────────────────────────────────────────────────────

  interface Props {
    project: string
    /** Bindable: receives the current task count so the parent can show a badge. */
    taskCount?: number
  }

  let { project, taskCount = $bindable(0) }: Props = $props()

  // ── State ─────────────────────────────────────────────────────────────────────

  type LoadState = 'idle' | 'loading' | 'done' | 'error'

  let loadState: LoadState = $state('idle')
  let tasks: Task[] = $state([])
  let loadError = $state('')

  // Keep the bindable count in sync with the authoritative task list.
  $effect(() => { taskCount = tasks.length })

  // Add-task form
  let newTitle = $state('')
  let addError = $state('')
  let isAdding = $state(false)

  // Per-task operation errors: { [taskId]: errorMessage }
  let taskErrors: Record<string, string> = $state({})

  // ── Lifecycle ──────────────────────────────────────────────────────────────────

  onMount(async () => {
    await loadTasks()
  })

  async function loadTasks() {
    loadState = 'loading'
    loadError = ''
    try {
      tasks = await api.getTasks(project)
      loadState = 'done'
    } catch (err) {
      loadState = 'error'
      loadError = formatError(err)
    }
  }

  // ── Add task ──────────────────────────────────────────────────────────────────

  async function handleAdd() {
    const title = newTitle.trim()
    if (!title) return
    isAdding = true
    addError = ''
    try {
      const task = await api.createTask(project, title)
      tasks = [...tasks, task]
      newTitle = ''
    } catch (err) {
      addError = formatError(err)
    } finally {
      isAdding = false
    }
  }

  function handleAddKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault()
      handleAdd()
    }
  }

  // ── Status cycling ────────────────────────────────────────────────────────────

  const STATUS_CYCLE: Record<Task['status'], Task['status']> = {
    'todo': 'in-progress',
    'in-progress': 'done',
    'done': 'todo',
  }

  async function cycleStatus(task: Task) {
    const nextStatus = STATUS_CYCLE[task.status]
    // Optimistic update
    tasks = tasks.map(t => t.id === task.id ? { ...t, status: nextStatus } : t)
    clearTaskError(task.id)

    try {
      const result = await api.updateTask(project, task.id, { status: nextStatus })
      if ('renumbered' in result) {
        tasks = result.tasks
      } else {
        tasks = tasks.map(t => t.id === result.id ? result : t)
      }
    } catch (err) {
      // Revert optimistic update on failure
      tasks = tasks.map(t => t.id === task.id ? { ...t, status: task.status } : t)
      setTaskError(task.id, formatError(err))
    }
  }

  // ── Delete ────────────────────────────────────────────────────────────────────

  async function deleteTask(task: Task) {
    // Optimistic removal
    const snapshot = tasks
    tasks = tasks.filter(t => t.id !== task.id)
    clearTaskError(task.id)

    try {
      await api.deleteTask(project, task.id)
    } catch (err) {
      // Restore on failure
      tasks = snapshot
      setTaskError(task.id, formatError(err))
    }
  }

  // ── Drag-to-reorder ────────────────────────────────────────────────────────────

  async function handleReorder(newOrder: string[]) {
    // Compute new fractional position for each moved item.
    // The StatusList gives us the full new order as an ID array.
    // We find which item changed position and compute its midpoint.
    const oldPositions = new Map(tasks.map(t => [t.id, t.position]))

    // Rebuild the ordered task list from the new ID sequence.
    const idToTask = new Map(tasks.map(t => [t.id, t]))
    const reordered = newOrder.map(id => idToTask.get(id)!).filter(Boolean)

    // Find the item whose index changed most (the dragged item).
    // Compare the new index order to the old.
    const oldOrder = tasks.map(t => t.id)
    let movedId: string | null = null
    for (let i = 0; i < newOrder.length; i++) {
      if (newOrder[i] !== oldOrder[i]) {
        movedId = newOrder[i]
        break
      }
    }
    if (!movedId) return

    const movedIdx = reordered.findIndex(t => t.id === movedId)
    const prev = movedIdx > 0 ? reordered[movedIdx - 1] : null
    const next = movedIdx < reordered.length - 1 ? reordered[movedIdx + 1] : null

    const prevPos = prev ? oldPositions.get(prev.id) ?? prev.position : 0
    const nextPos = next ? oldPositions.get(next.id) ?? next.position : (prevPos + 2)

    const newPosition = (prevPos + nextPos) / 2

    // Optimistic update
    tasks = reordered.map((t, i) =>
      t.id === movedId ? { ...t, position: newPosition } : { ...t, position: t.position }
    )

    try {
      const result = await api.updateTask(project, movedId, { position: newPosition })
      if ('renumbered' in result) {
        // Server renumbered everything — use the authoritative list
        tasks = result.tasks
      } else {
        tasks = tasks.map(t => t.id === result.id ? result : t)
      }
    } catch (err) {
      // Restore original order on failure
      tasks = tasks.map(t => ({ ...t, position: oldPositions.get(t.id) ?? t.position }))
        .sort((a, b) => a.position - b.position)
      // Show error on the moved item
      setTaskError(movedId, formatError(err))
    }
  }

  // ── StatusList mapping ────────────────────────────────────────────────────────

  const STATUS_LABELS: Record<Task['status'], string> = {
    'todo': 'Todo',
    'in-progress': 'In Progress',
    'done': 'Done',
  }

  let listItems = $derived(tasks.map((task): StatusListItem => ({
    id: task.id,
    title: task.title,
    status: task.status,
    actions: [
      {
        // Label shows next status so the action is self-documenting.
        // e.g. task is "todo" → button reads "In Progress" (what it becomes)
        label: STATUS_LABELS[STATUS_CYCLE[task.status]],
        onClick: () => cycleStatus(task),
      },
      {
        label: '×',
        variant: 'danger' as const,
        onClick: () => deleteTask(task),
      },
    ],
    // Surface per-task error as description so it appears inline under the title
    ...(taskErrors[task.id] ? { description: taskErrors[task.id] } : {}),
  })))

  // ── Error helpers ─────────────────────────────────────────────────────────────

  function setTaskError(id: string, msg: string) {
    taskErrors = { ...taskErrors, [id]: msg }
  }

  function clearTaskError(id: string) {
    const next = { ...taskErrors }
    delete next[id]
    taskErrors = next
  }

  function formatError(err: unknown): string {
    if (err instanceof ApiError) return `[${err.code}] ${err.message}`
    if (err instanceof Error) return err.message
    return 'Unknown error'
  }
</script>

<div class="task-backlog">
  <!-- Add task input -->
  <div class="task-backlog__add">
    <input
      class="task-backlog__add-input"
      class:task-backlog__add-input--error={!!addError}
      type="text"
      placeholder="New task…"
      bind:value={newTitle}
      onkeydown={handleAddKeydown}
      disabled={isAdding}
      aria-label="New task title"
      aria-invalid={!!addError}
      aria-describedby={addError ? 'task-backlog-add-error' : undefined}
    />
    <button
      class="task-backlog__add-btn"
      type="button"
      onclick={handleAdd}
      disabled={isAdding || !newTitle.trim()}
      aria-label="Add task"
    >+</button>
  </div>

  {#if addError}
    <p class="task-backlog__add-error" id="task-backlog-add-error" role="alert">{addError}</p>
  {/if}

  <!-- Task list -->
  {#if loadState === 'loading' || loadState === 'idle'}
    <div class="task-backlog__loading" aria-live="polite" aria-busy="true">
      <span class="task-backlog__spinner" aria-hidden="true"></span>
      Loading tasks…
    </div>
  {:else if loadState === 'error'}
    <div class="task-backlog__error" role="alert">
      <span>{loadError}</span>
      <button class="task-backlog__retry" type="button" onclick={loadTasks}>Retry</button>
    </div>
  {:else if tasks.length === 0}
    <p class="task-backlog__empty">No tasks yet. Add one above.</p>
  {:else}
    <div class="task-backlog__list">
      <StatusList
        items={listItems}
        draggable={true}
        onReorder={handleReorder}
        emptyMessage="No tasks yet. Add one above."
      />
    </div>
  {/if}
</div>

<style>
  /* ── Container ────────────────────────────────────────────────────────────── */
  .task-backlog {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  /* ── Add row ──────────────────────────────────────────────────────────────── */
  .task-backlog__add {
    display: flex;
    gap: var(--space-2);
    align-items: center;
  }

  .task-backlog__add-input {
    flex: 1;
    min-width: 0;
    padding: var(--space-2) var(--space-3);
    font-family: var(--font-sans);
    font-size: var(--font-size-sm);
    color: var(--color-text-primary);
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    outline: none;
    transition: border-color 80ms var(--ease-out), background-color 80ms var(--ease-out);
  }

  .task-backlog__add-input::placeholder {
    color: var(--color-text-muted);
  }

  .task-backlog__add-input:focus {
    border-color: var(--color-accent);
    background-color: var(--color-surface-base);
  }

  .task-backlog__add-input--error {
    border-color: var(--color-error);
  }

  .task-backlog__add-input:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .task-backlog__add-btn {
    flex-shrink: 0;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 30px;
    height: 30px;
    padding: 0;
    font-family: var(--font-sans);
    font-size: var(--font-size-base);
    font-weight: 500;
    color: var(--color-text-inverse);
    background-color: var(--color-accent);
    border: none;
    border-radius: var(--radius-md);
    cursor: pointer;
    transition: background-color 80ms var(--ease-out), opacity 80ms var(--ease-out);
  }

  .task-backlog__add-btn:hover:not(:disabled) {
    background-color: var(--color-accent-hover);
  }

  .task-backlog__add-btn:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  .task-backlog__add-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  /* ── Add error ────────────────────────────────────────────────────────────── */
  .task-backlog__add-error {
    margin: 0;
    font-size: var(--font-size-sm);
    color: var(--color-error);
    font-family: var(--font-mono);
  }

  /* ── Loading ──────────────────────────────────────────────────────────────── */
  .task-backlog__loading {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    color: var(--color-text-muted);
    font-size: var(--font-size-sm);
    padding: var(--space-4) 0;
  }

  .task-backlog__spinner {
    display: inline-block;
    width: 12px;
    height: 12px;
    border: 2px solid var(--color-border);
    border-top-color: var(--color-accent);
    border-radius: 50%;
    animation: tb-spin 600ms linear infinite;
    flex-shrink: 0;
  }

  @keyframes tb-spin {
    to { transform: rotate(360deg); }
  }

  /* ── Error state ──────────────────────────────────────────────────────────── */
  .task-backlog__error {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-3) var(--space-4);
    background-color: color-mix(in srgb, var(--color-error) 8%, transparent);
    border: 1px solid color-mix(in srgb, var(--color-error) 25%, transparent);
    border-radius: var(--radius-md);
    font-size: var(--font-size-sm);
    color: var(--color-error);
  }

  .task-backlog__retry {
    margin-left: auto;
    flex-shrink: 0;
    padding: 2px var(--space-2);
    background: none;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    font-family: var(--font-sans);
    font-size: var(--font-size-sm);
    color: var(--color-text-secondary);
    cursor: pointer;
    transition: border-color 80ms var(--ease-out), color 80ms var(--ease-out);
  }

  .task-backlog__retry:hover {
    border-color: var(--color-border-strong);
    color: var(--color-text-primary);
  }

  .task-backlog__retry:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  /* ── Empty state ──────────────────────────────────────────────────────────── */
  .task-backlog__empty {
    margin: 0;
    padding: var(--space-4) 0;
    font-size: var(--font-size-sm);
    color: var(--color-text-muted);
  }

  /* ── List wrapper ─────────────────────────────────────────────────────────── */
  .task-backlog__list {
    /* No extra wrapper styles needed — StatusList is full-width */
  }

  /* ── Reduced motion ───────────────────────────────────────────────────────── */
  @media (prefers-reduced-motion: reduce) {
    .task-backlog__spinner {
      animation: none;
      border-top-color: var(--color-accent);
    }

    .task-backlog__add-input,
    .task-backlog__add-btn,
    .task-backlog__retry {
      transition: none;
    }
  }
</style>
