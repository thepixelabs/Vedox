/**
 * ProviderDrawer.test.ts
 *
 * Component tests for the multi-provider config drawer (VDX-PD3-FE).
 *
 * The drawer's responsibility is the shell: opening, closing, switching
 * between concern tabs (Memory / Permissions / MCP / Agents), and surfacing
 * loading / error state from the underlying provider store. The four child
 * tabs (which each own their own loads + saves) are stubbed so these tests
 * stay focused on observable shell behaviour and don't break when, say,
 * MemoryTab adds a new field.
 *
 * Mocks live behind `vi.mock` calls below — the store mock exposes a tiny
 * controllable surface so each test can drive `detectedProviders`,
 * `drawerLoading`, and `drawerError` independently.
 */

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';

// ---------------------------------------------------------------------------
// Mocks — must be declared before importing the component under test.
// ---------------------------------------------------------------------------

// Tab children are stubbed: the real tabs do their own fetches and would pull
// network noise into shell tests. We swap them all for the same labelled stub.
vi.mock('$lib/components/MemoryTab.svelte', () => import('../../../../test-stubs/TabStub.svelte'));
vi.mock('$lib/components/PermissionsTab.svelte', () => import('../../../../test-stubs/TabStub.svelte'));
vi.mock('$lib/components/McpTab.svelte', () => import('../../../../test-stubs/TabStub.svelte'));
vi.mock('$lib/components/AgentsTab.svelte', () => import('../../../../test-stubs/TabStub.svelte'));

// Same trick for the relative imports inside ProviderDrawer.svelte (it imports
// from `./MemoryTab.svelte` etc., not via `$lib`).
vi.mock('../../MemoryTab.svelte', () => import('../../../../test-stubs/TabStub.svelte'));
vi.mock('../../PermissionsTab.svelte', () => import('../../../../test-stubs/TabStub.svelte'));
vi.mock('../../McpTab.svelte', () => import('../../../../test-stubs/TabStub.svelte'));
vi.mock('../../AgentsTab.svelte', () => import('../../../../test-stubs/TabStub.svelte'));

// Controllable provider store. Declared inside `vi.hoisted` so the bindings are
// initialised before `vi.mock` factories run (vi.mock calls are hoisted to the
// very top of the file by Vitest).
const storeMock = vi.hoisted(() => {
  return {
    state: {
      drawerOpen: false,
      activeProviderId: null as 'claude' | 'codex' | 'gemini' | null,
      detectedProviders: [] as Array<{
        id: 'claude' | 'codex' | 'gemini';
        name: string;
        available: boolean;
        scope: 'project' | 'global';
      }>,
      drawerLoading: false,
      drawerError: null as string | null,
    },
    openDrawer: vi.fn(),
    closeDrawer: vi.fn(),
    setActiveProvider: vi.fn(),
  };
});

const { state, openDrawer, closeDrawer, setActiveProvider } = storeMock;

vi.mock('$lib/stores/providerConfig.svelte', () => ({
  providerDrawer: {
    get drawerOpen() { return storeMock.state.drawerOpen; },
    get activeProviderId() { return storeMock.state.activeProviderId; },
    get detectedProviders() { return storeMock.state.detectedProviders; },
    get drawerLoading() { return storeMock.state.drawerLoading; },
    get drawerError() { return storeMock.state.drawerError; },
    openDrawer: storeMock.openDrawer,
    closeDrawer: storeMock.closeDrawer,
    setActiveProvider: storeMock.setActiveProvider,
  },
}));

// Import after mocks are registered.
import ProviderDrawer from '../../ProviderDrawer.svelte';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function resetStore() {
  state.drawerOpen = false;
  state.activeProviderId = 'claude';
  state.detectedProviders = [
    { id: 'claude', name: 'Claude Code', available: true,  scope: 'project' },
    { id: 'codex',  name: 'Codex',       available: true,  scope: 'global'  },
    { id: 'gemini', name: 'Gemini',      available: false, scope: 'project' },
  ];
  state.drawerLoading = false;
  state.drawerError = null;
  openDrawer.mockReset();
  closeDrawer.mockReset();
  setActiveProvider.mockReset();
  // Re-attach implementations after reset.
  openDrawer.mockImplementation(async (_project: string) => {
    state.drawerOpen = true;
    if (state.detectedProviders.length > 0 && !state.activeProviderId) {
      const firstAvailable = state.detectedProviders.find((p) => p.available);
      state.activeProviderId = firstAvailable?.id ?? null;
    }
  });
  closeDrawer.mockImplementation(() => {
    state.drawerOpen = false;
  });
  setActiveProvider.mockImplementation((id: 'claude' | 'codex' | 'gemini') => {
    state.activeProviderId = id;
  });
}

function renderOpen() {
  return render(ProviderDrawer, {
    props: { project: 'demo', open: true, onclose: vi.fn() },
  });
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('ProviderDrawer', () => {
  beforeEach(() => {
    resetStore();
  });

  it('renders the drawer shell, provider pills and Memory tab when open', () => {
    renderOpen();

    // Dialog shell + accessible name.
    const dialog = screen.getByRole('dialog', { name: /provider config/i });
    expect(dialog).toBeInTheDocument();

    // Provider pills mirror the detectedProviders list.
    expect(screen.getByRole('tab', { name: /claude code/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /codex/i })).toBeInTheDocument();
    // Gemini is unavailable — pill renders but is disabled.
    const gemini = screen.getByRole('tab', { name: /gemini/i });
    expect(gemini).toBeDisabled();

    // Default concern is Memory, so the (stubbed) Memory tab is mounted.
    const tab = screen.getByTestId('tab-stub');
    expect(tab).toHaveAttribute('data-tab', 'unnamed');
    // Concern rail shows all four labels.
    expect(screen.getByRole('button', { name: /^Memory$/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Permissions$/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^MCP$/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^Agents$/i })).toBeInTheDocument();
  });

  it('switches the active concern when a rail button is clicked', async () => {
    renderOpen();

    // Sanity: Memory rail button starts with aria-current="page".
    const memoryBtn = screen.getByRole('button', { name: /^Memory$/i });
    expect(memoryBtn).toHaveAttribute('aria-current', 'page');

    // Cycle through Permissions → MCP → Agents and back to Memory.
    for (const label of ['Permissions', 'MCP', 'Agents', 'Memory']) {
      const btn = screen.getByRole('button', { name: new RegExp(`^${label}$`, 'i') });
      await fireEvent.click(btn);
      expect(btn).toHaveAttribute('aria-current', 'page');
    }
  });

  it('calls store.openDrawer on the project when `open` flips true', async () => {
    // Render closed first…
    const { rerender } = render(ProviderDrawer, {
      props: { project: 'demo', open: false, onclose: vi.fn() },
    });
    expect(openDrawer).not.toHaveBeenCalled();

    // …then open it. The reactive $effect should fire openDrawer('demo').
    await rerender({ project: 'demo', open: true, onclose: vi.fn() });
    expect(openDrawer).toHaveBeenCalledTimes(1);
    expect(openDrawer).toHaveBeenCalledWith('demo');
  });

  it('shows the error banner when the store has a drawerError', () => {
    state.drawerError = '[VDX-003] provider not detected';
    renderOpen();
    const banner = screen.getByRole('alert');
    expect(banner).toHaveTextContent('[VDX-003] provider not detected');
  });

  it('renders a loading hint while detecting providers', () => {
    state.detectedProviders = [];
    state.drawerLoading = true;
    state.activeProviderId = null;
    renderOpen();
    expect(screen.getByText(/detecting providers/i)).toBeInTheDocument();
    // No active provider → empty-state in the main pane.
    expect(screen.getByText(/no provider selected/i)).toBeInTheDocument();
  });

  it('invokes onclose when the backdrop button is clicked', async () => {
    const onclose = vi.fn();
    render(ProviderDrawer, { props: { project: 'demo', open: true, onclose } });

    const backdrop = screen.getByRole('button', { name: /close provider config/i });
    await fireEvent.click(backdrop);
    expect(onclose).toHaveBeenCalledTimes(1);
  });
});
