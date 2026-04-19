/**
 * DocGraph.test.ts
 *
 * Component tests for DocGraph.svelte — the Cytoscape.js doc reference graph.
 *
 * Responsibilities tested:
 *   1. Shows loading state, then error on /api/graph 404.
 *   2. Fetches graph data and renders node/edge counts in the toolbar.
 *   3. Filter chip toggles change aria-checked state.
 *   4. Tapping a node navigates to the doc URL.
 *
 * Cytoscape and cytoscape-cose-bilkent are mocked at module level.
 * ResizeObserver is stubbed (not available in jsdom).
 * Graph data is delivered via mocked fetch to avoid Svelte 5 effect loops.
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));

// ResizeObserver is not available in jsdom. Stub it at module level so the
// stub outlives Svelte 5's async effect teardown (which fires after cleanup).
const resizeObserverStub = vi.fn().mockImplementation(() => ({
  observe: vi.fn(),
  disconnect: vi.fn(),
  unobserve: vi.fn(),
}));
vi.stubGlobal('ResizeObserver', resizeObserverStub);

const cytoscapeMock = vi.hoisted(() => {
  const listeners = {};
  const elements = {
    addClass: vi.fn().mockReturnThis(),
    removeClass: vi.fn().mockReturnThis(),
    remove: vi.fn(),
  };
  const cy = {
    on: vi.fn((event, _sel, handler) => {
      if (!listeners[event]) listeners[event] = [];
      listeners[event].push(handler);
    }),
    resize: vi.fn(),
    destroy: vi.fn(),
    zoom: vi.fn(),
    fit: vi.fn(),
    layout: vi.fn(() => ({ run: vi.fn() })),
    elements: vi.fn(() => elements),
    add: vi.fn(),
    width: vi.fn(() => 800),
    height: vi.fn(() => 600),
    $: vi.fn(() => ({ length: 0, first: vi.fn() })),
    _fireTap(nodeData) {
      const node = { data: (k) => nodeData[k], id: () => `${nodeData.project}/${nodeData.slug}` };
      for (const h of listeners['tap'] ?? []) h({ target: node });
    },
  };
  return { cy, listeners, cytoscapeFactory: vi.fn(() => cy), use: vi.fn() };
});

vi.mock('cytoscape', () => ({
  default: Object.assign(cytoscapeMock.cytoscapeFactory, { use: cytoscapeMock.use }),
}));

vi.mock('cytoscape-cose-bilkent', () => ({ default: vi.fn() }));

import DocGraph from '../DocGraph.svelte';
import { goto } from '$app/navigation';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeGraphData() {
  return {
    nodes: [
      { id: 'p/adr.md', project: 'p', slug: 'adr', title: 'ADR 001', type: 'adr', status: 'published', degree_in: 1, degree_out: 0, modified: '2026-01-01T00:00:00Z' },
      { id: 'p/howto.md', project: 'p', slug: 'howto', title: 'Install guide', type: 'how-to', status: 'published', degree_in: 0, degree_out: 1, modified: '2026-01-02T00:00:00Z' },
    ],
    edges: [{ source: 'p/howto.md', target: 'p/adr.md', kind: 'mdlink', broken: false }],
    truncated: false,
    total_nodes: 2,
    total_edges: 1,
  };
}

function mockFetch404() {
  globalThis.fetch = vi.fn(async () => new Response('Not Found', { status: 404, statusText: 'Not Found' }));
}

function mockFetchGraph(data) {
  globalThis.fetch = vi.fn(async () =>
    new Response(JSON.stringify(data), { status: 200, headers: { 'Content-Type': 'application/json' } })
  );
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('DocGraph', () => {
  beforeEach(() => {
    cytoscapeMock.cytoscapeFactory.mockClear();
    cytoscapeMock.cy.on.mockClear();
    cytoscapeMock.cy.destroy.mockClear();
    for (const key of Object.keys(cytoscapeMock.listeners)) {
      delete cytoscapeMock.listeners[key];
    }
    vi.mocked(goto).mockClear();
  });

  afterEach(() => {
    // Re-stub ResizeObserver in case any other afterEach removed it.
    vi.stubGlobal('ResizeObserver', resizeObserverStub);
  });

  it('shows loading overlay then error state when /api/graph returns 404', async () => {
    mockFetch404();
    render(DocGraph);

    expect(screen.getByRole('status')).toBeInTheDocument();

    const alert = await screen.findByRole('alert');
    expect(alert).toBeInTheDocument();
    expect(alert).toHaveTextContent(/404/i);
  });

  it('renders node and edge count in the toolbar when graph data is fetched', async () => {
    mockFetchGraph(makeGraphData());
    render(DocGraph);

    // Wait for loading to clear before checking stats.
    await waitFor(
      () => expect(screen.queryByRole('status')).not.toBeInTheDocument(),
      { timeout: 5000 },
    );

    const stats = screen.getByLabelText(/graph statistics/i);
    await waitFor(() => {
      expect(stats).toHaveTextContent('2 nodes');
      expect(stats).toHaveTextContent('1 edges');
    });
  });

  it('filter chip toggles aria-checked state when clicked', async () => {
    mockFetchGraph(makeGraphData());
    render(DocGraph);

    await waitFor(
      () => expect(screen.queryByRole('status')).not.toBeInTheDocument(),
      { timeout: 5000 },
    );

    const adrChip = await screen.findByRole('switch', { name: /show adr docs/i });
    expect(adrChip).toHaveAttribute('aria-checked', 'false');

    await fireEvent.click(adrChip);
    await waitFor(() => {
      expect(screen.getByRole('switch', { name: /hide adr docs/i })).toHaveAttribute('aria-checked', 'true');
    });
  });

  it('tapping a graph node navigates to the correct doc URL', async () => {
    mockFetchGraph(makeGraphData());
    render(DocGraph);

    await waitFor(
      () => expect(screen.queryByRole('status')).not.toBeInTheDocument(),
      { timeout: 5000 },
    );
    await waitFor(() => expect(cytoscapeMock.cytoscapeFactory).toHaveBeenCalled());

    cytoscapeMock.cy._fireTap({ project: 'p', slug: 'adr' });

    await waitFor(() => expect(goto).toHaveBeenCalledWith('/projects/p/docs/adr'));
  });
});
