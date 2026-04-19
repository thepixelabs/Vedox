/**
 * AllDone.test.ts
 *
 * Tests for AllDone.svelte — onboarding step 5 (summary + first doc CTA).
 *
 * Design contract:
 *   - The summary list reflects what was actually configured during onboarding:
 *     number of detected projects, registered repos, and selected providers.
 *   - The "start here" suggestion block contains a Cmd+N CTA.
 *   - Clicking "./open vedox" calls onboardingStore.complete() then onFinish().
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

const storeMock = vi.hoisted(() => ({
  registeredRepos: [] as string[],
  selectedProviders: [] as string[],
  detectedProjects: [] as Array<{ path: string; name: string; hasGit: boolean; docCount: number }>,
  skippedSteps: [] as number[],
  complete: vi.fn(),
  reset: vi.fn(),
}));

vi.mock('$lib/stores/onboarding.svelte', () => ({
  onboardingStore: {
    get registeredRepos() { return storeMock.registeredRepos; },
    get selectedProviders() { return storeMock.selectedProviders; },
    get detectedProjects() { return storeMock.detectedProjects; },
    get skippedSteps() { return storeMock.skippedSteps; },
    complete: storeMock.complete,
    reset: storeMock.reset,
  },
}));

import AllDone from '$lib/components/onboarding/AllDone.svelte';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function renderComponent(overrides: Partial<{ onBack: () => void; onFinish: () => void }> = {}) {
  const onBack = vi.fn();
  const onFinish = vi.fn();
  return {
    ...render(AllDone, { props: { onBack, onFinish, ...overrides } }),
    onBack,
    onFinish,
  };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('AllDone (onboarding step 5)', () => {
  beforeEach(() => {
    storeMock.registeredRepos = [];
    storeMock.selectedProviders = [];
    storeMock.detectedProjects = [];
    storeMock.skippedSteps = [];
    storeMock.complete.mockReset();
  });

  it('should show the count of detected projects and registered repos in the summary', () => {
    storeMock.detectedProjects = [
      { path: '/home/dev/alpha', name: 'alpha', hasGit: true, docCount: 3 },
      { path: '/home/dev/beta', name: 'beta', hasGit: true, docCount: 1 },
    ];
    storeMock.registeredRepos = ['/home/dev/alpha'];
    storeMock.selectedProviders = ['claude-code'];

    renderComponent();

    // Detected project count.
    expect(screen.getByText(/detected 2 projects/i)).toBeInTheDocument();

    // Registered repo path.
    expect(screen.getByText('/home/dev/alpha')).toBeInTheDocument();

    // Provider label (mapped from id 'claude-code').
    expect(screen.getByText(/claude code/i)).toBeInTheDocument();
  });

  it('should include a Cmd+N CTA in the start-here suggestion and call onFinish when ./open vedox is clicked', async () => {
    const { onFinish } = renderComponent();

    // The suggestion block mentions Cmd+N.
    const cmdN = screen.getByText('Cmd+N');
    expect(cmdN.tagName.toLowerCase()).toBe('kbd');

    // Clicking the primary CTA calls complete() then onFinish().
    const finishBtn = screen.getByRole('button', { name: /open vedox/i });
    await fireEvent.click(finishBtn);

    expect(storeMock.complete).toHaveBeenCalledTimes(1);
    expect(onFinish).toHaveBeenCalledTimes(1);
  });
});
