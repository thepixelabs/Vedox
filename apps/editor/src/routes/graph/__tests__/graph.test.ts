/**
 * graph.test.ts
 *
 * Route-level tests for /graph (+page.svelte).
 *
 * /graph is a project-picker landing page — the real graph canvas lives at
 * /projects/[project]/graph. This page reads from the projects store and
 * renders either a list of project links or a zero-state CTA.
 */

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import { projectsStore } from '$lib/stores/projects';

// $app/navigation is imported transitively; stub to avoid module errors.
vi.mock('$app/navigation', () => ({ goto: vi.fn() }));

import GraphPage from '../+page.svelte';

describe('/graph picker', () => {
  beforeEach(() => {
    projectsStore.setProjects([]);
  });

  it('renders the page title heading', () => {
    render(GraphPage);

    const heading = screen.getByRole('heading', { level: 1 });
    expect(heading).toHaveTextContent('reference graph');
  });

  it('renders an empty-state CTA when no projects are registered', () => {
    render(GraphPage);

    expect(screen.getByText(/no projects registered yet/i)).toBeInTheDocument();
    const cta = screen.getByRole('link', { name: /register a project/i });
    expect(cta).toHaveAttribute('href', '/onboarding');
  });

  it('lists every registered project and links each to its per-project graph', () => {
    projectsStore.setProjects([
      { id: 'alpha', name: 'alpha', docs: [], docCount: 12 },
      { id: 'beta', name: 'beta', docs: [], docCount: 3 },
    ]);
    render(GraphPage);

    const alphaLink = screen.getByRole('link', { name: /alpha/i });
    expect(alphaLink).toHaveAttribute('href', '/projects/alpha/graph');

    const betaLink = screen.getByRole('link', { name: /beta/i });
    expect(betaLink).toHaveAttribute('href', '/projects/beta/graph');
  });
});
