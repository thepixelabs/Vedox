/**
 * analytics.test.ts
 *
 * Route-level tests for /analytics (+page.svelte).
 *
 * The analytics page:
 *   1. Renders the page heading "analytics"
 *   2. Fires GET /api/analytics/summary on mount via onMount
 *   3. Shows an error banner with role="alert" on a non-2xx response
 *   4. Shows a refresh button once data loads and re-fetches on click
 *
 * The four child components (StatCard, DocsPerProject, VelocityChart,
 * PipelineStatus) are replaced with the project-standard TabStub so these
 * tests stay focused on the page shell without pulling in D3, canvas, or
 * chart dependencies.
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

// ---------------------------------------------------------------------------
// Mocks — must be declared before the component import.
// ---------------------------------------------------------------------------

vi.mock('$lib/components/analytics/StatCard.svelte', () =>
  import('../../../test-stubs/TabStub.svelte'),
);
vi.mock('$lib/components/analytics/DocsPerProject.svelte', () =>
  import('../../../test-stubs/TabStub.svelte'),
);
vi.mock('$lib/components/analytics/VelocityChart.svelte', () =>
  import('../../../test-stubs/TabStub.svelte'),
);
vi.mock('$lib/components/analytics/PipelineStatus.svelte', () =>
  import('../../../test-stubs/TabStub.svelte'),
);

import AnalyticsPage from '../+page.svelte';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const SAMPLE_SUMMARY = {
  pipeline_ready: true,
  total_docs: 42,
  docs_per_project: { pixelabs: 30, vedox: 12 },
  change_velocity_7d: 5,
  change_velocity_30d: 18,
};

function mockFetchOk(body = SAMPLE_SUMMARY) {
  globalThis.fetch = vi.fn(async () =>
    new Response(JSON.stringify(body), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }),
  ) as unknown as typeof fetch;
}

function mockFetchError(status = 503, text = 'Service Unavailable') {
  globalThis.fetch = vi.fn(async () =>
    new Response(text, { status }),
  ) as unknown as typeof fetch;
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('/analytics page shell', () => {
  beforeEach(() => {
    mockFetchOk();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('should render the analytics heading', async () => {
    render(AnalyticsPage);

    const heading = screen.getByRole('heading', { level: 1 });
    expect(heading).toHaveTextContent('analytics');
  });

  it('should fire GET /api/analytics/summary on mount', async () => {
    render(AnalyticsPage);

    await waitFor(() => expect(globalThis.fetch).toHaveBeenCalledTimes(1));
    const [url] = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0]!;
    expect(url).toBe('/api/analytics/summary');
  });

  it('should show an error alert when the API returns a non-2xx status', async () => {
    mockFetchError(503, 'Service Unavailable');
    render(AnalyticsPage);

    const alert = await screen.findByRole('alert');
    expect(alert).toBeInTheDocument();
    expect(alert).toHaveTextContent(/failed to load analytics/i);
  });

  it('should show a refresh button after successful load and re-fetch on click', async () => {
    render(AnalyticsPage);

    const refreshBtn = await screen.findByRole('button', { name: /refresh analytics/i });
    expect(refreshBtn).toBeInTheDocument();

    await fireEvent.click(refreshBtn);

    await waitFor(() =>
      expect(globalThis.fetch).toHaveBeenCalledTimes(2),
    );
  });
});
