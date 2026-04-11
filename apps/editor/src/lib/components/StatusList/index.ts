/**
 * StatusList barrel export
 *
 * Public surface for Phase 2 task backlog and Phase 3 agent review queue.
 *
 * Usage — full list:
 *   import { StatusList } from '$lib/components/StatusList'
 *   import type { StatusListItem } from '$lib/components/StatusList'
 *
 * Usage — chip only (Phase 3 header badges, etc.):
 *   import { StatusChip } from '$lib/components/StatusList'
 */

export { default as StatusList } from './StatusList.svelte'
export { default as StatusChip } from './StatusChip.svelte'

/**
 * Canonical item shape consumed by StatusList and StatusChip.
 * Extend this type (not replace it) in downstream consumers.
 */
export interface StatusListItem {
  /** Unique identifier — used as Svelte keyed-each key and reorder payload. */
  id: string
  /** Display title. Rendered as a link when `href` is provided. */
  title: string
  /** Drives chip color and ARIA label. */
  status: 'todo' | 'in-progress' | 'done' | 'review' | 'rejected'
  /** Optional secondary text below the title. Clamped to 3 lines. */
  description?: string
  /** Tertiary metadata line — agent name, date, file path, etc. Monospace. */
  meta?: string
  /** When set, the item title renders as an anchor tag pointing here. */
  href?: string
  /** Inline action buttons rendered below title/description. */
  actions?: Array<{
    label: string
    onClick: () => void
    /** Omit for neutral/secondary style. */
    variant?: 'primary' | 'danger'
  }>
}
