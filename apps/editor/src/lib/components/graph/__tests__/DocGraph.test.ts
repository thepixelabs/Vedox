/**
 * DocGraph.test.ts
 *
 * Component tests for DocGraph.svelte — the Cytoscape.js doc reference graph.
 *
 * Responsibilities tested:
 *   1. Shows loading state, then error when api.getGraph throws.
 *   2. Renders node/edge counts in the toolbar when data is provided.
 *   3. Filter chip toggles change aria-checked state.
 *   4. Tapping a node navigates to the correct doc URL.
 *   5. Passing `data` prop skips the fetch entirely.
 *
 * Cytoscape and cytoscape-cose-bilkent are mocked at module level.
 * ResizeObserver is stubbed (not available in jsdom).
 * The api client (`$lib/api/client`) is mocked so no network calls fire.
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import type { GraphData } from '$lib/api/client';

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

interface CytoscapeMockHandle {
  listeners: Record<string, Array<(evt: unknown) => void>>;
  cy: {
    on: ReturnType<typeof vi.fn>;
    resize: ReturnType<typeof vi.fn>;
    destroy: ReturnType<typeof vi.fn>;
    zoom: ReturnType<typeof vi.fn>;
    fit: ReturnType<typeof vi.fn>;
    layout: ReturnType<typeof vi.fn>;
    elements: ReturnType<typeof vi.fn>;
    add: ReturnType<typeof vi.fn>;
    width: ReturnType<typeof vi.fn>;
    height: ReturnType<typeof vi.fn>;
    $: ReturnType<typeof vi.fn>;
    _fireTap(nodeData: Record<string, string>): void;
  };
  cytoscapeFactory: ReturnType<typeof vi.fn>;
  use: ReturnType<typeof vi.fn>;
}

const cytoscapeMock = vi.hoisted<CytoscapeMockHandle>(() => {
  const listeners: Record<string, Array<(evt: unknown) => void>> = {};
  const elements = {
    addClass: vi.fn().mockReturnThis(),
    removeClass: vi.fn().mockReturnThis(),
    remove: vi.fn(),
  };
  const cy: CytoscapeMockHandle['cy'] = {
    on: vi.fn((event: string, _sel: string, handler: (evt: unknown) => void) => {
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
      const node = {
        data: (k: string) => nodeData[k],
        id: () => `${nodeData.project}/${nodeData.slug}`,
      };
      for (const h of listeners['tap'] ?? []) h({ target: node });
    },
  };
  return { cy, listeners, cytoscapeFactory: vi.fn(() => cy), use: vi.fn() };
});

vi.mock('cytoscape', () => ({
  default: Object.assign(cytoscapeMock.cytoscapeFactory, { use: cytoscapeMock.use }),
}));

vi.mock('cytoscape-cose-bilkent', () => ({ default: vi.fn() }));

// Mock the api client so DocGraph's self-fetch path is fully controllable.
const getGraphMock = vi.hoisted(() => vi.fn());
vi.mock('$lib/api/client', async () => {
  const actual = await vi.importActual<typeof import('$lib/api/client')>('$lib/api/client');
  return {
    ...actual,
    api: { ...actual.api, getGraph: getGraphMock },
  };
});

import DocGraph from '../DocGraph.svelte';
import { goto } from '$app/navigation';
import { ApiError } from '$lib/api/client';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeGraphData(): GraphData {
  return {
    nodes: [
      { id: 'p/adr.md', project: 'p', slug: 'adr', title: 'ADR 001', type: 'adr', status: 'published', degree_in: 1, degree_out: 0, modified: '2026-01-01T00:00:00Z' },
      { id: 'p/howto.md', project: 'p', slug: 'howto', title: 'Install guide', type: 'how-to', status: 'published', degree_in: 0, degree_out: 1, modified: '2026-01-02T00:00:00Z' },
    ],
    edges: [{ id: 'e1', source: 'p/howto.md', target: 'p/adr.md', kind: 'mdlink', broken: false }],
    truncated: false,
    total_nodes: 2,
    total_edges: 1,
  };
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
    getGraphMock.mockReset();
  });

  afterEach(() => {
    vi.stubGlobal('ResizeObserver', resizeObserverStub);
  });

  it('shows loading overlay then error state when api.getGraph throws', async () => {
    getGraphMock.mockRejectedValueOnce(new ApiError('VDX-500', 'boom', 500));
    render(DocGraph, { props: { project: 'p' } });

    expect(screen.getByRole('status')).toBeInTheDocument();

    const alert = await screen.findByRole('alert');
    expect(alert).toBeInTheDocument();
    expect(alert).toHaveTextContent(/VDX-500/);
  });

  it('renders node and edge count when self-fetched via api.getGraph', async () => {
    getGraphMock.mockResolvedValueOnce(makeGraphData());
    render(DocGraph, { props: { project: 'p' } });

    await waitFor(
      () => expect(screen.queryByRole('status')).not.toBeInTheDocument(),
      { timeout: 5000 },
    );

    const stats = screen.getByLabelText(/graph statistics/i);
    await waitFor(() => {
      expect(stats).toHaveTextContent('2 nodes');
      expect(stats).toHaveTextContent('1 edges');
    });
    expect(getGraphMock).toHaveBeenCalledWith('p');
  });

  it('skips the fetch when pre-loaded data is supplied via the `data` prop', async () => {
    render(DocGraph, { props: { data: makeGraphData() } });

    await waitFor(
      () => expect(screen.queryByRole('status')).not.toBeInTheDocument(),
      { timeout: 5000 },
    );
    expect(getGraphMock).not.toHaveBeenCalled();
  });

  it('filter chip toggles aria-checked state when clicked', async () => {
    getGraphMock.mockResolvedValueOnce(makeGraphData());
    render(DocGraph, { props: { project: 'p' } });

    await waitFor(
      () => expect(screen.queryByRole('status')).not.toBeInTheDocument(),
      { timeout: 5000 },
    );

    const adrChip = await screen.findByRole('switch', { name: /show adr docs/i });
    expect(adrChip).toHaveAttribute('aria-checked', 'false');

    await fireEvent.click(adrChip);
    await waitFor(() => {
      expect(screen.getByRole('switch', { name: /hide adr docs/i })).toHaveAttribute(
        'aria-checked',
        'true',
      );
    });
  });

  it('tapping a graph node navigates to the correct doc URL', async () => {
    getGraphMock.mockResolvedValueOnce(makeGraphData());
    render(DocGraph, { props: { project: 'p' } });

    await waitFor(
      () => expect(screen.queryByRole('status')).not.toBeInTheDocument(),
      { timeout: 5000 },
    );
    await waitFor(() => expect(cytoscapeMock.cytoscapeFactory).toHaveBeenCalled());

    cytoscapeMock.cy._fireTap({ project: 'p', slug: 'adr' });

    await waitFor(() => expect(goto).toHaveBeenCalledWith('/projects/p/docs/adr'));
  });
});
