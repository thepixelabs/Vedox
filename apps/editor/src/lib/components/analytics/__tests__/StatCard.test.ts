/**
 * StatCard.test.ts
 *
 * Component tests for StatCard.svelte — the compact metric card shown in the
 * analytics overview strip.
 *
 * Observable behaviour:
 *   1. Renders the metric value and label.
 *   2. Renders trend indicator with the correct accessible label
 *      for "up", "down", and "flat" directions.
 *   3. Renders an optional subtitle when the prop is supplied.
 */

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import StatCard from '../StatCard.svelte';

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('StatCard', () => {
  it('renders the metric value and label', () => {
    render(StatCard, { props: { label: 'Total Docs', value: '47' } });

    // The card uses aria-label="{label}: {value}" on the root element.
    const card = screen.getByLabelText(/total docs: 47/i);
    expect(card).toBeInTheDocument();

    // Both label and value text appear in the DOM.
    expect(screen.getByText('Total Docs')).toBeInTheDocument();
    expect(screen.getByText('47')).toBeInTheDocument();
  });

  it('renders correct accessible label for each trend direction', () => {
    const { rerender } = render(StatCard, {
      props: { label: 'Coverage', value: '80%', trend: 'up' },
    });

    expect(screen.getByLabelText(/trending up/i)).toBeInTheDocument();

    rerender({ label: 'Coverage', value: '80%', trend: 'down' });
    expect(screen.getByLabelText(/trending down/i)).toBeInTheDocument();

    rerender({ label: 'Coverage', value: '80%', trend: 'flat' });
    expect(screen.getByLabelText(/flat/i)).toBeInTheDocument();
  });

  it('renders the subtitle when the prop is supplied and no subtitle when omitted', () => {
    const { rerender } = render(StatCard, {
      props: { label: 'Velocity', value: '12', subtitle: '+3 this week' },
    });

    expect(screen.getByText('+3 this week')).toBeInTheDocument();

    // When subtitle is null (default), no subtitle element is rendered.
    rerender({ label: 'Velocity', value: '12', subtitle: null });
    expect(screen.queryByText(/this week/i)).not.toBeInTheDocument();
  });
});
