/**
 * SetupRepos.test.ts
 *
 * Tests for SetupRepos.svelte — onboarding step 2 (create or register a doc
 * repo).
 *
 * Design contract:
 *   - Clicking "./create new repo" transitions from the chooser to create mode
 *     and the form lets the user name a repo + parent folder. Submitting fires
 *     POST /api/repos/create and calls onNext after a short delay on success.
 *   - Clicking "./register existing folder" transitions to register mode and
 *     submitting fires POST /api/repos/register.
 *   - Clicking "./skip — use inbox" calls the onSkip prop and the hint copy
 *     mentions `~/.vedox/inbox/`.
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

const storeMock = vi.hoisted(() => ({
  addRegisteredRepo: vi.fn(),
}));

vi.mock('$lib/stores/onboarding.svelte', () => ({
  onboardingStore: {
    addRegisteredRepo: storeMock.addRegisteredRepo,
  },
}));

import SetupRepos from '$lib/components/onboarding/SetupRepos.svelte';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function renderComponent(overrides: Partial<{ onBack: () => void; onNext: () => void; onSkip: () => void }> = {}) {
  const onBack = vi.fn();
  const onNext = vi.fn();
  const onSkip = vi.fn();
  const result = render(SetupRepos, {
    props: { onBack, onNext, onSkip, ...overrides },
  });
  return { ...result, onBack, onNext, onSkip };
}

function mockCreateOk(repoPath = '/home/dev/my-docs') {
  globalThis.fetch = vi.fn(async () =>
    new Response(JSON.stringify({ path: repoPath }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }),
  ) as unknown as typeof fetch;
}

function mockRegisterOk(repoPath = '/existing/folder') {
  globalThis.fetch = vi.fn(async () =>
    new Response(JSON.stringify({ path: repoPath }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }),
  ) as unknown as typeof fetch;
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('SetupRepos (onboarding step 2)', () => {
  beforeEach(() => {
    storeMock.addRegisteredRepo.mockReset();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it('should show create-mode form on choosing ./create new repo and submit to /api/repos/create', async () => {
    vi.useFakeTimers();
    mockCreateOk('/home/dev/my-docs');
    const { onNext } = renderComponent();

    // Chooser is visible initially.
    await fireEvent.click(screen.getByText('./create new repo'));

    // Create-mode form appears.
    const nameInput = screen.getByRole('textbox', { name: /new repo name/i });
    expect(nameInput).toBeInTheDocument();

    // Fill in the name.
    await fireEvent.input(nameInput, { target: { value: 'my-docs' } });

    // Submit.
    const submitBtn = screen.getByRole('button', { name: /create repo/i });
    await fireEvent.click(submitBtn);

    await waitFor(() => expect(globalThis.fetch).toHaveBeenCalledTimes(1));
    const [url, init] = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0]!;
    expect(url).toBe('/api/repos/create');
    expect(init).toMatchObject({ method: 'POST' });
    const body = JSON.parse((init as RequestInit).body as string);
    expect(body).toMatchObject({ name: 'my-docs', type: 'bare-local' });

    // Store is notified.
    await waitFor(() =>
      expect(storeMock.addRegisteredRepo).toHaveBeenCalledWith('/home/dev/my-docs'),
    );

    // onNext is called after the success delay.
    await vi.advanceTimersByTimeAsync(600);
    expect(onNext).toHaveBeenCalledTimes(1);
  });

  it('should show register-mode form on choosing ./register existing folder and submit to /api/repos/register', async () => {
    vi.useFakeTimers();
    mockRegisterOk('/existing/folder');
    const { onNext } = renderComponent();

    await fireEvent.click(screen.getByText('./register existing folder'));

    const pathInput = screen.getByRole('textbox', { name: /existing folder path/i });
    await fireEvent.input(pathInput, { target: { value: '/existing/folder' } });

    const submitBtn = screen.getByRole('button', { name: /register folder/i });
    await fireEvent.click(submitBtn);

    await waitFor(() => expect(globalThis.fetch).toHaveBeenCalledTimes(1));
    const [url, init] = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0]!;
    expect(url).toBe('/api/repos/register');
    const body = JSON.parse((init as RequestInit).body as string);
    expect(body).toMatchObject({ path: '/existing/folder', type: 'bare-local' });

    await vi.advanceTimersByTimeAsync(600);
    expect(onNext).toHaveBeenCalledTimes(1);
  });

  it('should call onSkip and show inbox fallback hint when ./skip is clicked from the chooser', async () => {
    const { onSkip } = renderComponent();

    // The skip-hint copy is always visible in the header.
    expect(screen.getByText(/~\/.vedox\/inbox\//i)).toBeInTheDocument();

    await fireEvent.click(screen.getByRole('button', { name: /skip.*inbox/i }));

    expect(onSkip).toHaveBeenCalledTimes(1);
  });
});
