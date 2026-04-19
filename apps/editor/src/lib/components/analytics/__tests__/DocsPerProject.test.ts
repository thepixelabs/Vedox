/**
 * DocsPerProject.test.ts
 *
 * Component tests for DocsPerProject.svelte — the CSS-only horizontal bar
 * chart showing doc count per project.
 *
 * Observable behaviour:
 *   1. Renders one bar row per project, sorted by count descending.
 *   2. Shows the "no projects indexed yet" empty state when data is empty.
 */

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import DocsPerProject from '../DocsPerProject.svelte';

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('DocsPerProject', () => {
  it('renders a labelled row for each project with its doc count', () => {
    render(DocsPerProject, {
      props: { data: { alpha: 30, beta: 12, gamma: 5 } },
    });

    // Heading is present.
    expect(screen.getByRole('heading', { name: /docs per project/i })).toBeInTheDocument();

    // Each project name appears as a label.
    expect(screen.getByText('alpha')).toBeInTheDocument();
    expect(screen.getByText('beta')).toBeInTheDocument();
    expect(screen.getByText('gamma')).toBeInTheDocument();

    // Each count value is accessible via aria-label.
    expect(screen.getByLabelText('30 docs')).toBeInTheDocument();
    expect(screen.getByLabelText('12 docs')).toBeInTheDocument();
    expect(screen.getByLabelText('5 docs')).toBeInTheDocument();

    // Project count badge shows total project count.
    expect(screen.getByText(/3 projects/i)).toBeInTheDocument();
  });

  it('renders the empty state when the data record is empty', () => {
    render(DocsPerProject, { props: { data: {} } });

    expect(screen.getByText(/no projects indexed yet/i)).toBeInTheDocument();

    // No list items present.
    expect(screen.queryByRole('listitem')).not.toBeInTheDocument();
  });
});
