/**
 * ScanProjects.test.ts
 *
 * Tests for ScanProjects.svelte — onboarding step 1 (detect projects).
 *
 * Design contract:
 *   - On mount the component fires GET /api/scan automatically. While in
 *     flight, the loading spinner is visible.
 *   - If fetch times out or the daemon is unreachable, the component shows an
 *     offline state (not a hard error) with a retry button.
 *   - When scan succeeds, detected repos are rendered as checked checkboxes.
 *     Unchecking one removes it from the selection; re-checking it adds it back.
 *
 * We mock globalThis.fetch so no real network calls are made. The onboarding
 * store is mocked so tests do not bleed into localStorage.
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

const storeMock = vi.hoisted(() => ({
  detectedProjects: [] as Array<{ path: string; name: string; hasGit: boolean; docCount: number }>,
  setDetectedProjects: vi.fn(),
}));

vi.mock('$lib/stores/onboarding.svelte', () => ({
  onboardingStore: {
    get detectedProjects() {
      return storeMock.detectedProjects;
    },
    setDetectedProjects: storeMock.setDetectedProjects,
  },
}));

import ScanProjects from '$lib/components/onboarding/ScanProjects.svelte';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function renderComponent() {
  return render(ScanProjects, {
    props: {
      onNext: vi.fn(),
      onSkip: vi.fn(),
    },
  });
}

const SAMPLE_PROJECTS = [
  { path: '/home/dev/alpha', name: 'alpha', hasGit: true, docCount: 5 },
  { path: '/home/dev/beta', name: 'beta', hasGit: true, docCount: 2 },
];

function mockFetchOk(projects = SAMPLE_PROJECTS) {
  globalThis.fetch = vi.fn(async () =>
    new Response(JSON.stringify({ projects }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }),
  ) as unknown as typeof fetch;
}

/** Simulates a daemon timeout / offline condition. */
function mockFetchOffline() {
  globalThis.fetch = vi.fn(async () => {
    throw new TypeError('Failed to fetch');
  }) as unknown as typeof fetch;
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('ScanProjects (onboarding step 1)', () => {
  beforeEach(() => {
    storeMock.detectedProjects = [];
    storeMock.setDetectedProjects.mockReset();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('should trigger an auto-scan on mount and show detected repos as checked checkboxes', async () => {
    mockFetchOk();
    renderComponent();

    // The component hits fetch once on mount.
    await waitFor(() => expect(globalThis.fetch).toHaveBeenCalledTimes(1));
    const [url] = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0]!;
    expect(url).toBe('/api/scan');

    // After the fetch resolves, both repos appear as checked checkboxes.
    const alphaCheck = await screen.findByRole('checkbox', { name: /include alpha/i });
    const betaCheck = await screen.findByRole('checkbox', { name: /include beta/i });

    expect(alphaCheck).toBeChecked();
    expect(betaCheck).toBeChecked();
  });

  it('should show the offline state when the daemon fetch fails with a network error', async () => {
    mockFetchOffline();
    renderComponent();

    // Wait for the component to transition out of the loading state.
    await waitFor(() =>
      expect(screen.queryByText(/scanning filesystem/i)).not.toBeInTheDocument(),
    );

    // The offline panel should be visible.
    expect(screen.getByText(/the vedox daemon is not running/i)).toBeInTheDocument();

    // A retry button is present so the user can try again.
    expect(screen.getByRole('button', { name: /retry scan/i })).toBeInTheDocument();
  });

  it('should uncheck a pre-selected repo when its checkbox is clicked', async () => {
    mockFetchOk();
    renderComponent();

    const alphaCheck = await screen.findByRole('checkbox', { name: /include alpha/i });
    expect(alphaCheck).toBeChecked();

    // Uncheck alpha.
    await fireEvent.click(alphaCheck);

    expect(alphaCheck).not.toBeChecked();

    // Beta remains checked.
    const betaCheck = screen.getByRole('checkbox', { name: /include beta/i });
    expect(betaCheck).toBeChecked();
  });
});
