/**
 * InstallAgent.test.ts
 *
 * Component tests for onboarding step 3 (InstallAgent.svelte).
 *
 * The step exposes four AI providers (Claude Code, Codex, Gemini, Copilot),
 * lets the user multi-select, and POSTs to `/api/agent/install` for each
 * selection in parallel. We mock global fetch so tests run hermetically and
 * we mock the onboarding store so test state never bleeds into localStorage
 * (or other tests via the singleton).
 *
 * The reactive flow we care about:
 *   1. initial render → 4 checkboxes, none selected, install button disabled
 *   2. checkbox click → provider becomes selected, install button enables
 *   3. install button click → fetch fires, status flips to "done" on 2xx
 *   4. fetch failure → status flips to "error" and the install button stays
 *      enabled so the user can re-trigger (the de-facto retry path)
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

// ---------------------------------------------------------------------------
// Mocks — must be declared before importing the component under test.
// ---------------------------------------------------------------------------

// Onboarding store is a singleton that touches localStorage. We swap it for
// a controllable in-memory stub so tests can assert on `setSelectedProviders`
// without persistence side effects. `vi.hoisted` runs before `vi.mock`
// factories so the bindings exist when the factory closes over them.
const storeMock = vi.hoisted(() => ({
  state: { selectedProviders: [] as string[] },
  setSelectedProviders: vi.fn(),
}));

const { state: onboardingState, setSelectedProviders } = storeMock;

vi.mock('$lib/stores/onboarding.svelte', () => ({
  onboardingStore: {
    get selectedProviders() { return storeMock.state.selectedProviders; },
    setSelectedProviders: storeMock.setSelectedProviders,
  },
}));

import InstallAgent from '$lib/components/onboarding/InstallAgent.svelte';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const PROVIDER_LABELS = ['Claude Code', 'OpenAI Codex', 'Google Gemini', 'GitHub Copilot'];

function mockFetchOk() {
  const fetchMock = vi.fn(async () =>
    new Response(JSON.stringify({ ok: true }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }),
  );
  globalThis.fetch = fetchMock as unknown as typeof fetch;
  return fetchMock;
}

function mockFetchError(message = 'daemon offline') {
  const fetchMock = vi.fn(async () =>
    new Response(JSON.stringify({ message }), {
      status: 503,
      headers: { 'Content-Type': 'application/json' },
    }),
  );
  globalThis.fetch = fetchMock as unknown as typeof fetch;
  return fetchMock;
}

function renderInstallAgent() {
  return render(InstallAgent, {
    props: {
      onBack: vi.fn(),
      onNext: vi.fn(),
      onSkip: vi.fn(),
    },
  });
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('InstallAgent (onboarding step 3)', () => {
  beforeEach(() => {
    onboardingState.selectedProviders = [];
    setSelectedProviders.mockReset();
    setSelectedProviders.mockImplementation((providers: string[]) => {
      onboardingState.selectedProviders = providers;
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('renders all four providers with no selection and a disabled install button', () => {
    renderInstallAgent();

    for (const label of PROVIDER_LABELS) {
      const cb = screen.getByRole('checkbox', { name: new RegExp(`select ${label}`, 'i') });
      expect(cb).toBeInTheDocument();
      expect(cb).not.toBeChecked();
    }

    const installBtn = screen.getByRole('button', { name: /install selected/i });
    expect(installBtn).toBeDisabled();
  });

  it('selecting a provider and clicking install fires POST /api/agent/install with that provider id', async () => {
    const fetchMock = mockFetchOk();
    renderInstallAgent();

    const claudeBox = screen.getByRole('checkbox', { name: /select claude code/i });
    await fireEvent.click(claudeBox);
    expect(claudeBox).toBeChecked();

    const installBtn = screen.getByRole('button', { name: /install selected/i });
    expect(installBtn).toBeEnabled();
    await fireEvent.click(installBtn);

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe('/api/agent/install');
    expect(init).toMatchObject({ method: 'POST' });
    expect(JSON.parse((init as RequestInit).body as string)).toEqual({ provider: 'claude-code' });
  });

  it('shows the success checkmark for an installed provider on a 2xx response', async () => {
    mockFetchOk();
    renderInstallAgent();

    await fireEvent.click(screen.getByRole('checkbox', { name: /select openai codex/i }));
    await fireEvent.click(screen.getByRole('button', { name: /install selected/i }));

    // The "Installed" affordance is the SVG checkmark wrapped in a span with
    // aria-label="Installed". Wait for it to appear once the fetch resolves.
    const done = await screen.findByLabelText('Installed');
    expect(done).toBeInTheDocument();

    // Footer flips to ./continue once every selected provider is done.
    expect(screen.getByRole('button', { name: /continue/i })).toBeInTheDocument();
  });

  it('shows the error indicator and leaves the install button enabled for retry on a non-2xx response', async () => {
    const fetchMock = mockFetchError('daemon offline');
    renderInstallAgent();

    await fireEvent.click(screen.getByRole('checkbox', { name: /select google gemini/i }));
    await fireEvent.click(screen.getByRole('button', { name: /install selected/i }));

    // Error mark + role=alert error message both render once the fetch rejects.
    expect(await screen.findByLabelText(/install failed/i)).toBeInTheDocument();
    expect(await screen.findByRole('alert')).toHaveTextContent('daemon offline');

    // The button remains the install button (label resets from "installing...")
    // and is still enabled so the user can re-trigger — this is the component's
    // de-facto retry path. NOTE: there is no dedicated "Retry" button, which is
    // a UX gap worth surfacing.
    const installBtn = await screen.findByRole('button', { name: /install selected/i });
    expect(installBtn).toBeEnabled();

    // Re-clicking should fire fetch again.
    await fireEvent.click(installBtn);
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
  });
});
