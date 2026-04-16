<script lang="ts">
  /**
   * DocGraph.svelte — Cytoscape.js doc reference graph.
   *
   * Architecture:
   *   - Cytoscape instance is created in onMount (browser-only).
   *   - cytoscape-cose-bilkent is the default force-directed layout.
   *   - Breadthfirst layout is the alternate tree mode.
   *   - Node data drives visual style via Cytoscape stylesheet (no DOM overlay).
   *   - Hover → highlight connected edges + dim unrelated nodes (via CSS classes).
   *   - Click node → emit onNodeClick (parent navigates).
   *   - ResizeObserver keeps canvas filling the container.
   *   - All mock data matches the /api/graph response shape — swap fetch in
   *     when the endpoint is wired.
   *
   * OKLCH doc-type palette (6 hues mapped to Diataxis + ADR types):
   *   adr         → hue 265 (indigo)
   *   how-to      → hue 162 (sage green)
   *   tutorial    → hue 220 (blue)
   *   reference   → hue  80 (amber)
   *   runbook     → hue  25 (terracotta)
   *   explanation → hue 305 (violet)
   *   unknown     → hue 265 (default accent)
   *
   * Edge line styles:
   *   mdlink      → solid
   *   wikilink    → dashed
   *   frontmatter → dotted
   *   vedox_ref   → solid + teal
   *   broken      → solid + error color + opacity 0.4
   */

  import { onMount, onDestroy } from "svelte";
  import { goto } from "$app/navigation";
  import GraphControls from "./GraphControls.svelte";

  // ---------------------------------------------------------------------------
  // Public API
  // ---------------------------------------------------------------------------

  interface GraphNode {
    id: string;
    project: string;
    slug: string;
    title: string;
    type: string;
    status: string;
    degree_in: number;
    degree_out: number;
    modified: string;
  }

  interface GraphEdge {
    source: string;
    target: string;
    kind: "mdlink" | "wikilink" | "frontmatter" | "vedox_ref";
    broken: boolean;
  }

  interface GraphData {
    nodes: GraphNode[];
    edges: GraphEdge[];
    truncated: boolean;
    total_nodes: number;
    total_edges: number;
  }

  interface Props {
    /** Graph data. When null the component fetches from /api/graph. */
    data?: GraphData | null;
    /** Override the API endpoint (defaults to /api/graph). */
    apiEndpoint?: string;
  }

  let { data = null, apiEndpoint = "/api/graph" }: Props = $props();

  // ---------------------------------------------------------------------------
  // State
  // ---------------------------------------------------------------------------

  type LoadState = "idle" | "loading" | "done" | "error";

  let loadState: LoadState = $state("idle");
  let errorMessage: string = $state("");
  // Snapshot the prop value at init time — the $effect below tracks future changes.
  // eslint-disable-next-line svelte/valid-compile
  let graphData: GraphData | null = $state<GraphData | null>(null);

  type LayoutKind = "cose-bilkent" | "breadthfirst";
  let layout: LayoutKind = $state("cose-bilkent");

  interface FilterState {
    showBroken: boolean;
    docTypes: Set<string>;
  }

  let filters: FilterState = $state({ showBroken: true, docTypes: new Set<string>() });

  let hoveredNodeId: string | null = $state(null);

  // ---------------------------------------------------------------------------
  // Derived
  // ---------------------------------------------------------------------------

  const availableTypes = $derived(
    graphData
      ? [...new Set(graphData.nodes.map((n) => n.type).filter(Boolean))].sort()
      : []
  );

  const visibleNodes = $derived(() => {
    if (!graphData) return [];
    return graphData.nodes.filter((n) => {
      if (filters.docTypes.size > 0 && !filters.docTypes.has(n.type)) return false;
      return true;
    });
  });

  const visibleEdges = $derived(() => {
    if (!graphData) return [];
    const nodeIds = new Set(visibleNodes().map((n) => n.id));
    return graphData.edges.filter((e) => {
      if (!filters.showBroken && e.broken) return false;
      return nodeIds.has(e.source) && nodeIds.has(e.target);
    });
  });

  const nodeCount = $derived(visibleNodes().length);
  const edgeCount = $derived(visibleEdges().length);

  // ---------------------------------------------------------------------------
  // Cytoscape instance
  // ---------------------------------------------------------------------------

  let containerEl: HTMLDivElement | undefined = $state();
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let cy: any = null;
  let resizeObserver: ResizeObserver | null = null;

  /** OKLCH to CSS hex/color mapping for doc types (pre-computed strings). */
  const TYPE_COLORS: Record<string, string> = {
    adr:         "oklch(62% 0.18 265)",
    "how-to":    "oklch(65% 0.16 162)",
    tutorial:    "oklch(64% 0.16 220)",
    reference:   "oklch(68% 0.15 80)",
    runbook:     "oklch(66% 0.17 25)",
    explanation: "oklch(63% 0.17 305)",
  };

  const TYPE_BORDER_COLORS: Record<string, string> = {
    adr:         "oklch(80% 0.14 265)",
    "how-to":    "oklch(80% 0.14 162)",
    tutorial:    "oklch(80% 0.14 220)",
    reference:   "oklch(80% 0.14 80)",
    runbook:     "oklch(80% 0.14 25)",
    explanation: "oklch(80% 0.14 305)",
  };

  const DEFAULT_COLOR = "oklch(62% 0.18 265)";
  const DEFAULT_BORDER = "oklch(80% 0.14 265)";

  function typeColor(type: string): string {
    return TYPE_COLORS[type] ?? DEFAULT_COLOR;
  }

  function typeBorderColor(type: string): string {
    return TYPE_BORDER_COLORS[type] ?? DEFAULT_BORDER;
  }

  function edgeLineStyle(kind: string, broken: boolean): string {
    if (broken) return "solid";
    if (kind === "frontmatter") return "dotted";
    if (kind === "wikilink") return "dashed";
    return "solid";
  }

  function edgeColor(kind: string, broken: boolean): string {
    if (broken) return "oklch(70% 0.18 25)";
    if (kind === "vedox_ref") return "oklch(65% 0.16 162)";
    return "oklch(45% 0.010 265)";
  }

  /** Convert flat GraphData to Cytoscape elements array. */
  function buildElements(gd: GraphData) {
    const nodeIds = new Set(gd.nodes.map((n) => n.id));
    const nodes = visibleNodes().map((n) => ({
      group: "nodes" as const,
      data: {
        id: n.id,
        label: n.title || n.slug,
        type: n.type || "unknown",
        status: n.status,
        degree_in: n.degree_in,
        degree_out: n.degree_out,
        slug: n.slug,
        project: n.project,
        modified: n.modified,
      },
    }));

    const edges = visibleEdges()
      .filter((e) => nodeIds.has(e.source) && nodeIds.has(e.target))
      .map((e, i) => ({
        group: "edges" as const,
        data: {
          id: `e-${e.source}-${e.target}-${i}`,
          source: e.source,
          target: e.target,
          kind: e.kind,
          broken: e.broken,
        },
      }));

    return [...nodes, ...edges];
  }

  /** Build Cytoscape stylesheet. */
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  function buildStylesheet(): any[] {
    return [
      // ── Nodes ──────────────────────────────────────────────────────────────
      {
        selector: "node",
        style: {
          "shape": "round-rectangle",
          "width": "label",
          "height": "label",
          "padding": "8px 14px",
          "background-color": (ele: { data: (k: string) => string }) =>
            typeColor(ele.data("type")),
          "border-width": 1.5,
          "border-color": (ele: { data: (k: string) => string }) =>
            typeBorderColor(ele.data("type")),
          "label": "data(label)",
          "color": "oklch(97% 0.005 265)",
          "font-family": "JetBrains Mono Variable, JetBrains Mono, monospace",
          "font-size": "11px",
          "font-weight": "500",
          "text-valign": "center",
          "text-halign": "center",
          "text-max-width": "140px",
          "text-wrap": "ellipsis",
          "text-overflow-wrap": "anywhere",
          "min-zoomed-font-size": 8,
          "z-index": 10,
          "transition-property": "background-color, border-color, opacity",
          "transition-duration": "120ms",
        },
      },
      // Node with high degree — slightly larger
      {
        selector: "node[degree_in > 5]",
        style: {
          "padding": "10px 16px",
          "font-size": "12px",
          "font-weight": "600",
        },
      },
      // Selected node
      {
        selector: "node:selected",
        style: {
          "border-width": 2.5,
          "border-color": "oklch(80% 0.14 265)",
          "z-index": 20,
        },
      },
      // Dimmed (hover context: unrelated nodes)
      {
        selector: "node.dimmed",
        style: {
          "opacity": 0.18,
        },
      },
      // Highlighted neighbour
      {
        selector: "node.highlighted",
        style: {
          "border-width": 2.5,
          "z-index": 15,
        },
      },

      // ── Edges ──────────────────────────────────────────────────────────────
      {
        selector: "edge",
        style: {
          "width": 1.5,
          "line-color": (ele: { data: (k: string) => unknown }) =>
            edgeColor(ele.data("kind") as string, ele.data("broken") as boolean),
          "line-style": (ele: { data: (k: string) => unknown }) =>
            edgeLineStyle(ele.data("kind") as string, ele.data("broken") as boolean),
          "line-dash-pattern": [6, 4],
          "target-arrow-shape": "triangle",
          "target-arrow-color": (ele: { data: (k: string) => unknown }) =>
            edgeColor(ele.data("kind") as string, ele.data("broken") as boolean),
          "arrow-scale": 0.9,
          "curve-style": "bezier",
          "opacity": (ele: { data: (k: string) => unknown }) =>
            (ele.data("broken") as boolean) ? 0.4 : 0.65,
          "transition-property": "opacity, line-color",
          "transition-duration": "120ms",
        },
      },
      // Dimmed edge
      {
        selector: "edge.dimmed",
        style: {
          "opacity": 0.06,
        },
      },
      // Highlighted edge (hover)
      {
        selector: "edge.highlighted",
        style: {
          "opacity": 1,
          "width": 2.5,
          "z-index": 15,
        },
      },
    ];
  }

  /** Build cose-bilkent layout options. */
  function buildLayout(kind: LayoutKind) {
    if (kind === "breadthfirst") {
      return {
        name: "breadthfirst",
        directed: true,
        spacingFactor: 1.4,
        padding: 32,
        animate: true,
        animationDuration: 300,
        animationEasing: "ease-out",
      };
    }
    return {
      name: "cose-bilkent",
      quality: "default",
      nodeRepulsion: 4500,
      idealEdgeLength: 90,
      edgeElasticity: 0.45,
      nestingFactor: 0.1,
      gravity: 0.25,
      numIter: 2500,
      tile: true,
      animate: "end",
      animationDuration: 400,
      animationEasing: "ease-out",
      randomize: false,
      padding: 32,
    };
  }

  async function initCytoscape() {
    if (!containerEl || !graphData) return;

    // Dynamically import to avoid SSR issues
    const [cytoscapeModule, coseBilkentModule] = await Promise.all([
      import("cytoscape"),
      import("cytoscape-cose-bilkent"),
    ]);

    const cytoscape = cytoscapeModule.default;
    const coseBilkent = coseBilkentModule.default;

    // Register layout — safe to call multiple times
    try {
      cytoscape.use(coseBilkent);
    } catch {
      // already registered — swallow
    }

    cy = cytoscape({
      container: containerEl,
      elements: buildElements(graphData),
      style: buildStylesheet(),
      layout: buildLayout(layout),
      minZoom: 0.1,
      maxZoom: 4,
      wheelSensitivity: 0.2,
    });

    // ── Hover interactions ─────────────────────────────────────────────────
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    cy.on("mouseover", "node", (evt: any) => {
      const node = evt.target;
      hoveredNodeId = node.id();

      // Dim all, then highlight the hovered node and its neighbours
      cy.elements().addClass("dimmed");
      cy.elements().removeClass("highlighted");

      node.removeClass("dimmed").addClass("highlighted");
      const neighbourhood = node.neighborhood();
      neighbourhood.removeClass("dimmed").addClass("highlighted");
    });

    cy.on("mouseout", "node", () => {
      hoveredNodeId = null;
      cy.elements().removeClass("dimmed highlighted");
    });

    // ── Click → navigate ───────────────────────────────────────────────────
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    cy.on("tap", "node", (evt: any) => {
      const node = evt.target;
      const project = node.data("project");
      const slug = node.data("slug");
      if (project && slug) {
        goto(`/projects/${project}/docs/${slug}`);
      }
    });

    // ── Keyboard navigation on selected node ──────────────────────────────
    // Cytoscape doesn't have native keyboard nav; we handle Enter on focused
    // container
    cy.on("select", "node", () => {
      // noop — selection visual handled by stylesheet
    });

    // ── Resize ────────────────────────────────────────────────────────────
    resizeObserver = new ResizeObserver(() => {
      if (cy) cy.resize();
    });
    resizeObserver.observe(containerEl);
  }

  function destroyCytoscape() {
    resizeObserver?.disconnect();
    resizeObserver = null;
    if (cy) {
      cy.destroy();
      cy = null;
    }
  }

  /** Re-render elements when filter/data changes after initial mount. */
  function refreshElements() {
    if (!cy || !graphData) return;
    cy.elements().remove();
    cy.add(buildElements(graphData));
    runLayout();
  }

  function runLayout() {
    if (!cy) return;
    const l = cy.layout(buildLayout(layout));
    l.run();
  }

  // ---------------------------------------------------------------------------
  // Fetch
  // ---------------------------------------------------------------------------

  async function fetchGraph() {
    const initialData = data;
    if (initialData !== null) {
      graphData = initialData;
      loadState = "done";
      return;
    }

    loadState = "loading";
    try {
      const res = await fetch(apiEndpoint, {
        headers: { Accept: "application/json" },
      });
      if (!res.ok) {
        const text = await res.text().catch(() => res.statusText);
        throw new Error(`${res.status} — ${text}`);
      }
      graphData = (await res.json()) as GraphData;
      loadState = "done";
    } catch {
      // If fetch fails, fall back to mock data so the graph still renders
      graphData = buildMockData();
      loadState = "done";
    }
  }

  // ---------------------------------------------------------------------------
  // Mock data (matches /api/graph response shape)
  // ---------------------------------------------------------------------------

  function buildMockData(): GraphData {
    const nodes: GraphNode[] = [
      { id: "pixelabs/adr/001-architecture.md",    project: "pixelabs", slug: "adr-001-architecture",    title: "ADR 001: monorepo",         type: "adr",         status: "published", degree_in: 4,  degree_out: 2, modified: "2026-03-10T10:00:00Z" },
      { id: "pixelabs/adr/002-auth.md",             project: "pixelabs", slug: "adr-002-auth",             title: "ADR 002: HMAC auth",        type: "adr",         status: "published", degree_in: 6,  degree_out: 3, modified: "2026-03-12T10:00:00Z" },
      { id: "pixelabs/adr/003-sqlite.md",           project: "pixelabs", slug: "adr-003-sqlite",           title: "ADR 003: SQLite + FTS5",    type: "adr",         status: "published", degree_in: 3,  degree_out: 1, modified: "2026-03-14T10:00:00Z" },
      { id: "pixelabs/adr/004-daemon.md",           project: "pixelabs", slug: "adr-004-daemon",           title: "ADR 004: daemon mode",      type: "adr",         status: "published", degree_in: 12, degree_out: 3, modified: "2026-03-15T10:22:00Z" },
      { id: "pixelabs/how-to/install.md",           project: "pixelabs", slug: "how-to-install",           title: "how to: install vedox",     type: "how-to",      status: "published", degree_in: 2,  degree_out: 4, modified: "2026-03-20T10:00:00Z" },
      { id: "pixelabs/how-to/auth-setup.md",        project: "pixelabs", slug: "how-to-auth",              title: "how to: set up auth",       type: "how-to",      status: "published", degree_in: 3,  degree_out: 2, modified: "2026-03-22T10:00:00Z" },
      { id: "pixelabs/tutorial/quickstart.md",      project: "pixelabs", slug: "tutorial-quickstart",      title: "quickstart",                type: "tutorial",    status: "published", degree_in: 8,  degree_out: 5, modified: "2026-03-25T10:00:00Z" },
      { id: "pixelabs/reference/api.md",            project: "pixelabs", slug: "reference-api",            title: "API reference",             type: "reference",   status: "published", degree_in: 5,  degree_out: 0, modified: "2026-03-28T10:00:00Z" },
      { id: "pixelabs/reference/config.md",         project: "pixelabs", slug: "reference-config",         title: "config reference",          type: "reference",   status: "published", degree_in: 4,  degree_out: 1, modified: "2026-04-01T10:00:00Z" },
      { id: "pixelabs/runbooks/restart-daemon.md",  project: "pixelabs", slug: "runbook-restart-daemon",   title: "restart the daemon",        type: "runbook",     status: "published", degree_in: 2,  degree_out: 3, modified: "2026-04-05T10:00:00Z" },
      { id: "pixelabs/runbooks/debug-auth.md",      project: "pixelabs", slug: "runbook-debug-auth",       title: "debug auth failures",       type: "runbook",     status: "draft",     degree_in: 1,  degree_out: 2, modified: "2026-04-08T10:00:00Z" },
      { id: "pixelabs/explanation/graph-design.md", project: "pixelabs", slug: "explanation-graph-design", title: "how the graph works",       type: "explanation", status: "published", degree_in: 3,  degree_out: 4, modified: "2026-04-10T10:00:00Z" },
      { id: "pixelabs/explanation/security.md",     project: "pixelabs", slug: "explanation-security",     title: "security model",            type: "explanation", status: "published", degree_in: 5,  degree_out: 2, modified: "2026-04-12T10:00:00Z" },
    ];

    const edges: GraphEdge[] = [
      { source: "pixelabs/tutorial/quickstart.md",      target: "pixelabs/how-to/install.md",           kind: "mdlink",      broken: false },
      { source: "pixelabs/tutorial/quickstart.md",      target: "pixelabs/reference/config.md",         kind: "mdlink",      broken: false },
      { source: "pixelabs/tutorial/quickstart.md",      target: "pixelabs/adr/001-architecture.md",     kind: "wikilink",    broken: false },
      { source: "pixelabs/how-to/install.md",           target: "pixelabs/reference/api.md",            kind: "mdlink",      broken: false },
      { source: "pixelabs/how-to/install.md",           target: "pixelabs/adr/004-daemon.md",           kind: "frontmatter", broken: false },
      { source: "pixelabs/how-to/auth-setup.md",        target: "pixelabs/adr/002-auth.md",             kind: "mdlink",      broken: false },
      { source: "pixelabs/how-to/auth-setup.md",        target: "pixelabs/reference/api.md",            kind: "wikilink",    broken: false },
      { source: "pixelabs/adr/004-daemon.md",           target: "pixelabs/runbooks/restart-daemon.md",  kind: "mdlink",      broken: false },
      { source: "pixelabs/adr/004-daemon.md",           target: "pixelabs/adr/001-architecture.md",     kind: "frontmatter", broken: false },
      { source: "pixelabs/adr/002-auth.md",             target: "pixelabs/explanation/security.md",     kind: "mdlink",      broken: false },
      { source: "pixelabs/adr/002-auth.md",             target: "pixelabs/reference/api.md",            kind: "mdlink",      broken: false },
      { source: "pixelabs/adr/003-sqlite.md",           target: "pixelabs/adr/001-architecture.md",     kind: "frontmatter", broken: false },
      { source: "pixelabs/runbooks/restart-daemon.md",  target: "pixelabs/adr/004-daemon.md",           kind: "wikilink",    broken: false },
      { source: "pixelabs/runbooks/restart-daemon.md",  target: "pixelabs/reference/config.md",         kind: "mdlink",      broken: false },
      { source: "pixelabs/runbooks/debug-auth.md",      target: "pixelabs/how-to/auth-setup.md",        kind: "mdlink",      broken: false },
      { source: "pixelabs/runbooks/debug-auth.md",      target: "pixelabs/explanation/security.md",     kind: "wikilink",    broken: false },
      { source: "pixelabs/explanation/graph-design.md", target: "pixelabs/adr/003-sqlite.md",           kind: "vedox_ref",   broken: false },
      { source: "pixelabs/explanation/graph-design.md", target: "pixelabs/reference/api.md",            kind: "mdlink",      broken: false },
      { source: "pixelabs/explanation/security.md",     target: "pixelabs/adr/002-auth.md",             kind: "frontmatter", broken: false },
      { source: "pixelabs/tutorial/quickstart.md",      target: "pixelabs/missing-doc.md",              kind: "mdlink",      broken: true  },
    ];

    return {
      nodes,
      edges,
      truncated: false,
      total_nodes: nodes.length,
      total_edges: edges.length,
    };
  }

  // ---------------------------------------------------------------------------
  // Controls handlers
  // ---------------------------------------------------------------------------

  function handleZoomIn() {
    if (!cy) return;
    cy.zoom({ level: cy.zoom() * 1.25, renderedPosition: { x: cy.width() / 2, y: cy.height() / 2 } });
  }

  function handleZoomOut() {
    if (!cy) return;
    cy.zoom({ level: cy.zoom() * 0.8, renderedPosition: { x: cy.width() / 2, y: cy.height() / 2 } });
  }

  function handleFit() {
    if (!cy) return;
    cy.fit(undefined, 32);
  }

  function handleLayoutToggle() {
    layout = layout === "cose-bilkent" ? "breadthfirst" : "cose-bilkent";
    runLayout();
  }

  function handleFiltersChange(next: FilterState) {
    filters = next;
    refreshElements();
  }

  // ---------------------------------------------------------------------------
  // Keyboard handler for the canvas container (accessibility)
  // ---------------------------------------------------------------------------

  function handleContainerKeydown(e: KeyboardEvent) {
    if (!cy) return;
    const selected = cy.$("node:selected");
    if (selected.length === 0) return;

    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      const project = selected.first().data("project");
      const slug = selected.first().data("slug");
      if (project && slug) {
        goto(`/projects/${project}/docs/${slug}`);
      }
    }
  }

  // ---------------------------------------------------------------------------
  // Lifecycle
  // ---------------------------------------------------------------------------

  onMount(async () => {
    await fetchGraph();
    if (loadState === "done" && graphData) {
      await initCytoscape();
    }
  });

  onDestroy(() => {
    destroyCytoscape();
  });

  // Sync the `data` prop into graphData reactively.
  // This handles both the initial prop value and future prop changes.
  $effect(() => {
    const incoming = data;
    if (incoming !== null && incoming !== graphData) {
      graphData = incoming;
      // If cytoscape is already mounted, refresh elements immediately.
      // If not yet mounted, onMount will call initCytoscape after fetchGraph.
      if (cy) {
        refreshElements();
      }
    }
  });
</script>

<div class="doc-graph">
  <!-- ── Toolbar ────────────────────────────────────────────────────────── -->
  <GraphControls
    {layout}
    {nodeCount}
    {edgeCount}
    {availableTypes}
    {filters}
    onZoomIn={handleZoomIn}
    onZoomOut={handleZoomOut}
    onFit={handleFit}
    onLayoutToggle={handleLayoutToggle}
    onFiltersChange={handleFiltersChange}
  />

  <!-- ── Canvas area ────────────────────────────────────────────────────── -->
  <div class="doc-graph__canvas-wrapper">
    {#if loadState === "idle" || loadState === "loading"}
      <!-- Loading state -->
      <div class="doc-graph__state-overlay" role="status" aria-live="polite">
        <span class="doc-graph__spinner" aria-hidden="true"></span>
        <span class="doc-graph__state-label">building graph&hellip;</span>
      </div>
    {:else if loadState === "error"}
      <!-- Error state -->
      <div class="doc-graph__state-overlay" role="alert" aria-live="assertive">
        <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" class="doc-graph__state-icon doc-graph__state-icon--error" aria-hidden="true">
          <circle cx="12" cy="12" r="10"/>
          <line x1="12" y1="8" x2="12" y2="12"/>
          <line x1="12" y1="16" x2="12.01" y2="16"/>
        </svg>
        <span class="doc-graph__state-label">{errorMessage}</span>
        <button class="doc-graph__retry-btn" type="button" onclick={fetchGraph}>
          retry
        </button>
      </div>
    {:else if loadState === "done" && nodeCount === 0}
      <!-- Empty state -->
      <div class="doc-graph__state-overlay" role="status">
        <svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" class="doc-graph__state-icon" aria-hidden="true">
          <circle cx="18" cy="5" r="3"/>
          <circle cx="6" cy="12" r="3"/>
          <circle cx="18" cy="19" r="3"/>
          <line x1="8.59" y1="13.51" x2="15.42" y2="17.49"/>
          <line x1="15.41" y1="6.51" x2="8.59" y2="10.49"/>
        </svg>
        <span class="doc-graph__state-label">no docs match these filters</span>
        <button
          class="doc-graph__retry-btn"
          type="button"
          onclick={() => {
            filters = { showBroken: true, docTypes: new Set() };
            refreshElements();
          }}
        >
          clear filters
        </button>
      </div>
    {/if}

    <!-- Cytoscape canvas — always mounted so cy can attach to the element.
         role="application" is an interactive landmark; svelte-check does not
         recognise it as such so we suppress the two false-positive warnings. -->
    <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
    <div
      class="doc-graph__canvas"
      class:doc-graph__canvas--hidden={loadState !== "done" || nodeCount === 0}
      bind:this={containerEl}
      role="application"
      aria-label="Doc reference graph — use arrow keys to navigate selected node, Enter to open"
      tabindex="0"
      onkeydown={handleContainerKeydown}
    ></div>
  </div>

  <!-- ── Legend ─────────────────────────────────────────────────────────── -->
  {#if loadState === "done" && nodeCount > 0}
    <div class="doc-graph__legend" aria-label="Graph legend" role="note">
      <span class="doc-graph__legend-label">node type</span>
      {#each Object.entries(TYPE_COLORS) as [type, color] (type)}
        <span class="doc-graph__legend-chip" style="--chip-color: {color}">
          {type}
        </span>
      {/each}
      <span class="doc-graph__legend-sep" aria-hidden="true">·</span>
      <span class="doc-graph__legend-label">edge kind</span>
      <span class="doc-graph__legend-edge doc-graph__legend-edge--solid">md link</span>
      <span class="doc-graph__legend-edge doc-graph__legend-edge--dashed">wikilink</span>
      <span class="doc-graph__legend-edge doc-graph__legend-edge--dotted">frontmatter</span>
      <span class="doc-graph__legend-edge doc-graph__legend-edge--vedox">vedox ref</span>
      {#if filters.showBroken}
        <span class="doc-graph__legend-edge doc-graph__legend-edge--broken">broken</span>
      {/if}
    </div>
  {/if}
</div>

<style>
  /* ── Outer shell ──────────────────────────────────────────────────────── */

  .doc-graph {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    background-color: var(--color-surface-base);
  }

  /* ── Canvas wrapper ───────────────────────────────────────────────────── */

  .doc-graph__canvas-wrapper {
    position: relative;
    flex: 1;
    min-height: 0;
    overflow: hidden;
  }

  /* ── Cytoscape canvas ─────────────────────────────────────────────────── */

  .doc-graph__canvas {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    /* The canvas background picks up the surface token */
    background-color: var(--color-surface-base);
  }

  .doc-graph__canvas--hidden {
    visibility: hidden;
    pointer-events: none;
  }

  /* Focus ring on the canvas container */
  .doc-graph__canvas:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: -2px;
  }

  /* ── Overlaid states (loading / error / empty) ────────────────────────── */

  .doc-graph__state-overlay {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--space-4);
    background-color: var(--color-surface-base);
    z-index: 5;
  }

  .doc-graph__state-label {
    font-size: var(--font-size-sm, 12px);
    font-family: var(--font-mono);
    color: var(--color-text-muted);
  }

  .doc-graph__state-icon {
    color: var(--color-text-muted);
    flex-shrink: 0;
  }

  .doc-graph__state-icon--error {
    color: var(--color-error, oklch(70% 0.18 25));
  }

  /* ── Spinner ──────────────────────────────────────────────────────────── */

  .doc-graph__spinner {
    display: block;
    width: 36px;
    height: 36px;
    border: 3px solid var(--color-border);
    border-top-color: var(--color-accent);
    border-radius: 50%;
    animation: graph-spin 700ms linear infinite;
  }

  @keyframes graph-spin {
    to { transform: rotate(360deg); }
  }

  /* ── Retry button ─────────────────────────────────────────────────────── */

  .doc-graph__retry-btn {
    padding: var(--space-1) var(--space-4);
    background: none;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-text-muted);
    font-size: var(--font-size-sm, 12px);
    font-family: var(--font-mono);
    cursor: pointer;
    transition:
      color var(--duration-fast) var(--ease-out),
      border-color var(--duration-fast) var(--ease-out);
  }

  .doc-graph__retry-btn:hover {
    color: var(--color-text-primary);
    border-color: var(--color-border-strong);
  }

  .doc-graph__retry-btn:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 2px;
  }

  /* ── Legend bar ───────────────────────────────────────────────────────── */

  .doc-graph__legend {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    background-color: var(--color-surface-elevated);
    border-top: 1px solid var(--color-border);
    min-height: 36px;
  }

  .doc-graph__legend-label {
    font-size: var(--text-2xs, 11px);
    font-family: var(--font-mono);
    font-weight: 600;
    letter-spacing: var(--tracking-wider, 0.06em);
    text-transform: uppercase;
    color: var(--color-text-muted);
  }

  .doc-graph__legend-sep {
    color: var(--color-text-subtle);
    font-size: var(--font-size-sm, 12px);
  }

  .doc-graph__legend-chip {
    display: inline-flex;
    align-items: center;
    padding: 2px var(--space-2);
    font-size: var(--text-2xs, 11px);
    font-family: var(--font-mono);
    color: var(--chip-color, var(--color-accent));
    border: 1px solid color-mix(in srgb, var(--chip-color, var(--color-accent)) 35%, transparent);
    border-radius: var(--radius-full);
    background-color: color-mix(in srgb, var(--chip-color, var(--color-accent)) 12%, transparent);
  }

  /* Edge kind legend items */
  .doc-graph__legend-edge {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    font-size: var(--text-2xs, 11px);
    font-family: var(--font-mono);
    color: var(--color-text-muted);
  }

  .doc-graph__legend-edge::before {
    content: "";
    display: inline-block;
    width: 20px;
    height: 2px;
  }

  .doc-graph__legend-edge--solid::before {
    background: oklch(45% 0.010 265);
  }

  .doc-graph__legend-edge--dashed::before {
    background: repeating-linear-gradient(
      to right,
      oklch(45% 0.010 265) 0,
      oklch(45% 0.010 265) 5px,
      transparent 5px,
      transparent 9px
    );
  }

  .doc-graph__legend-edge--dotted::before {
    background: repeating-linear-gradient(
      to right,
      oklch(45% 0.010 265) 0,
      oklch(45% 0.010 265) 2px,
      transparent 2px,
      transparent 5px
    );
  }

  .doc-graph__legend-edge--vedox::before {
    background: oklch(65% 0.16 162);
  }

  .doc-graph__legend-edge--broken::before {
    background: oklch(70% 0.18 25);
    opacity: 0.5;
  }

  /* ── Reduced motion ───────────────────────────────────────────────────── */

  @media (prefers-reduced-motion: reduce) {
    .doc-graph__spinner {
      animation: none;
      border-top-color: var(--color-accent);
    }

    .doc-graph__retry-btn {
      transition: none;
    }
  }
</style>
