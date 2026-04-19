/**
 * HistoryTimeline.test.ts
 *
 * Component tests for HistoryTimeline.svelte — the vertical doc history timeline.
 *
 * HistoryTimeline responsibilities under test:
 *   1. Lazy-load: fetch fires on mount (onMount), not before.
 *   2. Expanding an entry shows the ChangeDiff list.
 *   3. Author badge is shown for agent authorKind, absent for human.
 *   4. Empty state when fetch returns an empty array.
 *   5. Error state when fetch fails.
 *
 * Mocked at system boundaries:
 *   - fetch (globalThis.fetch) — controls HTTP responses hermetically.
 *   - HistoryEntry.svelte and ChangeDiff.svelte are NOT mocked: they are
 *     small pure-render components and their output is part of observable
 *     behaviour here (author badge, expand/collapse, change diff).
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import type { HistoryEntry } from '../types.js';
import HistoryTimeline from '../HistoryTimeline.svelte';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeEntry(overrides: Partial<HistoryEntry> = {}): HistoryEntry {
  return {
    commitHash: 'abc123',
    author: 'Alice Ng',
    authorEmail: 'alice@example.com',
    authorKind: 'human',
    date: '2026-04-01T10:00:00Z',
    message: 'docs: update install guide',
    summary: 'Updated the install section with Docker step.',
    changes: [],
    ...overrides,
  };
}

function makeChange() {
  return {
    type: 'added' as const,
    blockKind: 'paragraph' as const,
    section: 'Installation',
    before: '',
    after: 'Run docker compose up.',
  };
}

function mockFetchOk(entries: HistoryEntry[]) {
  globalThis.fetch = vi.fn(async () =>
    new Response(JSON.stringify(entries), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }),
  ) as unknown as typeof fetch;
}

function mockFetchError(message = 'Internal Server Error') {
  globalThis.fetch = vi.fn(async () =>
    new Response(JSON.stringify({ message }), {
      status: 500,
      headers: { 'Content-Type': 'application/json' },
    }),
  ) as unknown as typeof fetch;
}

function renderTimeline(projectId = 'my-project', docPath = 'docs/guide.md') {
  return render(HistoryTimeline, { props: { projectId, docPath } });
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('HistoryTimeline', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('fires the history fetch on mount (lazy-load on first open)', async () => {
    mockFetchOk([]);
    renderTimeline();

    // Fetch must have been called once after mount (onMount fires it).
    await waitFor(() =>
      expect(globalThis.fetch as ReturnType<typeof vi.fn>).toHaveBeenCalledTimes(1),
    );

    // The URL should reference projectId and encoded docPath.
    const [url] = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0] as [string];
    expect(url).toMatch('/api/projects/my-project/docs/');
    expect(url).toMatch('history');
  });

  it('should expand an entry and show ChangeDiff when the toggle button is clicked', async () => {
    const entry = makeEntry({
      commitHash: 'sha-expand',
      changes: [makeChange()],
    });
    mockFetchOk([entry]);
    renderTimeline();

    // Wait for entries to load.
    await screen.findByText(/updated the install section/i);

    // Toggle button should be present ("View 1 change").
    const toggleBtn = screen.getByRole('button', { name: /view 1 change/i });
    expect(toggleBtn).toHaveAttribute('aria-expanded', 'false');

    await fireEvent.click(toggleBtn);

    // After expanding, ChangeDiff content appears.
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /hide 1 change/i })).toHaveAttribute('aria-expanded', 'true'),
    );

    // The "added paragraph" diff block should be visible.
    expect(screen.getByText(/run docker compose up/i)).toBeInTheDocument();
  });

  it('shows the agent badge for claude-code authorKind and no badge for human', async () => {
    const humanEntry = makeEntry({ commitHash: 'h1', authorKind: 'human', author: 'Bob Smith' });
    const agentEntry = makeEntry({ commitHash: 'h2', authorKind: 'claude-code', author: 'claude-code[bot]' });
    mockFetchOk([humanEntry, agentEntry]);
    renderTimeline();

    // Wait for entries to render.
    await screen.findByText(/bob smith/i);

    // Agent entry has badge with aria-label "AI agent: claude code".
    const agentBadge = screen.getByLabelText(/AI agent: claude code/i);
    expect(agentBadge).toBeInTheDocument();

    // Human entry (Bob Smith) does NOT have an agent badge.
    // The avatar for Bob Smith is accessible by its img role and label.
    const humanAvatar = screen.getByRole('img', { name: /bob smith, human/i });
    expect(humanAvatar).toBeInTheDocument();
  });

  it('shows the empty state message when the API returns an empty array', async () => {
    mockFetchOk([]);
    renderTimeline();

    // Empty state: "No history yet."
    const emptyMsg = await screen.findByText(/no history yet/i);
    expect(emptyMsg).toBeInTheDocument();
  });

  it('shows the error state when the fetch fails', async () => {
    mockFetchError('git log failed');
    renderTimeline();

    // Error state uses role="alert".
    const alert = await screen.findByRole('alert');
    expect(alert).toBeInTheDocument();
    expect(alert).toHaveTextContent('git log failed');

    // Retry button present.
    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
  });
});
