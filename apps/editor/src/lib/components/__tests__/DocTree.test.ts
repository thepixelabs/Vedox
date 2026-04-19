/**
 * DocTree.test.ts
 *
 * Component tests for DocTree.svelte — the hierarchical document navigator.
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

vi.mock('$lib/stores/projects', async (importOriginal) => {
  const real = await importOriginal();
  return {
    ...real,
    projectsStore: {
      subscribe: vi.fn(),
      setProjectDocs: vi.fn(),
    },
  };
});

vi.mock('$lib/api/client', () => ({
  api: { getProjectDocs: vi.fn().mockResolvedValue([]) },
}));

vi.mock('$lib/stores/panes', () => ({
  panesStore: { split: vi.fn(), open: vi.fn() },
}));

import DocTree from '../DocTree.svelte';

// ---------------------------------------------------------------------------
// localStorage stub
// ---------------------------------------------------------------------------

function makeLocalStorageStub() {
  const store = new Map();
  return {
    getItem: (key) => store.get(key) ?? null,
    setItem: (key, value) => { store.set(key, value); },
    removeItem: (key) => { store.delete(key); },
    clear: () => { store.clear(); },
    get length() { return store.size; },
    key: (i) => [...store.keys()][i] ?? null,
    _store: store,
  };
}

// ---------------------------------------------------------------------------
// Factories
// ---------------------------------------------------------------------------

function makeDoc(overrides) {
  return { type: 'how-to', folder: '', ...overrides };
}

function makeProject(docs = []) {
  return { id: 'proj-1', name: 'Project One', docs };
}

function renderTree(project) {
  return render(DocTree, { props: { project } });
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('DocTree', () => {
  let localStorageStub;

  beforeEach(() => {
    localStorageStub = makeLocalStorageStub();
    vi.stubGlobal('localStorage', localStorageStub);
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('renders the empty-folder heading when the project has no docs', async () => {
    renderTree(makeProject([]));
    const heading = await screen.findByRole('heading', { name: /empty folder/i });
    expect(heading).toBeInTheDocument();
  });

  it('renders type-group headers (role=treeitem) for each distinct doc type', async () => {
    const docs = [
      makeDoc({ path: 'install.md', title: 'Install guide', type: 'how-to' }),
      makeDoc({ path: 'adr-001.md', title: 'ADR 001', type: 'adr' }),
      makeDoc({ path: 'quickstart.md', title: 'Quickstart', type: 'tutorial' }),
    ];
    renderTree(makeProject(docs));

    // Group buttons have aria-label="{type} (N docs)" — match exactly enough
    // to avoid collision with doc link treeitems.
    expect(await screen.findByRole('treeitem', { name: /how-to \(\d+ docs?\)/i })).toBeInTheDocument();
    expect(screen.getByRole('treeitem', { name: /^adr \(\d+ docs?\)/i })).toBeInTheDocument();
    expect(screen.getByRole('treeitem', { name: /tutorial \(\d+ docs?\)/i })).toBeInTheDocument();

    // Doc titles appear as treeitem links.
    expect(screen.getByRole('treeitem', { name: /install guide/i })).toBeInTheDocument();
    expect(screen.getByRole('treeitem', { name: /adr 001/i })).toBeInTheDocument();
    expect(screen.getByRole('treeitem', { name: /quickstart/i })).toBeInTheDocument();
  });

  it('should narrow visible items when the filter input is typed into', async () => {
    const docs = [
      makeDoc({ path: 'install.md', title: 'Install guide', type: 'how-to' }),
      makeDoc({ path: 'auth.md', title: 'Auth setup', type: 'how-to' }),
      makeDoc({ path: 'adr-001.md', title: 'ADR 001 monorepo', type: 'adr' }),
    ];
    renderTree(makeProject(docs));

    await screen.findByRole('treeitem', { name: /install guide/i });

    const filterInput = screen.getByRole('searchbox', { name: /filter documents/i });
    await fireEvent.input(filterInput, { target: { value: 'auth' } });

    expect(screen.getByRole('treeitem', { name: /auth setup/i })).toBeInTheDocument();
    expect(screen.queryByRole('treeitem', { name: /install guide/i })).not.toBeInTheDocument();
    expect(screen.queryByRole('treeitem', { name: /adr 001/i })).not.toBeInTheDocument();
    expect(screen.queryByText(/no match for/i)).not.toBeInTheDocument();
  });

  it('persists expand/collapse state to localStorage and restores on re-mount', async () => {
    const project = makeProject([
      makeDoc({ path: 'install.md', title: 'Install guide', type: 'how-to' }),
    ]);

    const { unmount } = renderTree(project);
    const groupBtn = await screen.findByRole('treeitem', { name: /how-to \(\d+ docs?\)/i });

    // Default: expanded. Collapse the group.
    await fireEvent.click(groupBtn);
    expect(screen.queryByRole('treeitem', { name: /install guide/i })).not.toBeInTheDocument();

    // Collapsed state is written to localStorage.
    const storageKey = `vedox:tree:${project.id}:expanded`;
    const stored = JSON.parse(localStorageStub.getItem(storageKey) ?? '[]');
    expect(stored).not.toContain('how-to');

    unmount();

    // Re-mount restores collapsed state.
    renderTree(project);
    await screen.findByRole('treeitem', { name: /how-to \(\d+ docs?\)/i });
    expect(screen.queryByRole('treeitem', { name: /install guide/i })).not.toBeInTheDocument();
  });

  it('should have an href pointing to the doc URL for each treeitem link', async () => {
    renderTree(makeProject([
      makeDoc({ path: 'how-to/deploy.md', title: 'Deploy guide', type: 'how-to' }),
    ]));

    const link = await screen.findByRole('treeitem', { name: /deploy guide/i });
    expect(link).toHaveAttribute('href', '/projects/proj-1/docs/how-to/deploy.md');
  });

  it('exposes role="tree" on the document list for ARIA conformance', async () => {
    renderTree(makeProject([
      makeDoc({ path: 'readme.md', title: 'Readme', type: 'readme' }),
    ]));

    // Wait for the tree to render — confirm role="tree" is present.
    // Use findByRole on 'tree' directly (not 'treeitem') to avoid ambiguity
    // between the group button and the doc link that both match /readme/.
    const tree = await screen.findByRole('tree', { name: /documents/i });
    expect(tree).toBeInTheDocument();
  });
});
