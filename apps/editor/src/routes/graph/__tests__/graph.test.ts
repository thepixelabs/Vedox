/**
 * graph.test.ts
 *
 * Route-level tests for /graph (+page.svelte).
 *
 * The graph page is a thin shell that:
 *   1. Renders a page header with the heading "reference graph"
 *   2. Renders the subtitle hint text
 *   3. Mounts the DocGraph component inside the canvas area
 *
 * DocGraph itself owns fetch / Cytoscape — we replace it with the shared
 * TabStub so the route tests focus on the page-shell contract.
 */

import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/svelte';

// ---------------------------------------------------------------------------
// Mocks — must be declared before the component import.
// ---------------------------------------------------------------------------

vi.mock('$lib/components/graph/DocGraph.svelte', () =>
  import('../../../test-stubs/TabStub.svelte'),
);

// DocGraph imports goto from $app/navigation. Stub it to avoid module errors.
vi.mock('$app/navigation', () => ({ goto: vi.fn() }));

import GraphPage from '../+page.svelte';

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('/graph page shell', () => {
  it('should render the page title heading', () => {
    render(GraphPage);

    const heading = screen.getByRole('heading', { level: 1 });
    expect(heading).toHaveTextContent('reference graph');
  });

  it('should render the user instruction subtitle', () => {
    render(GraphPage);

    expect(screen.getByText(/click a node to open the doc/i)).toBeInTheDocument();
  });
});
