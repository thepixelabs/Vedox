<script lang="ts">
  import { fade } from 'svelte/transition';
  import { editor } from '$lib/content';
  import { reveal } from '$lib/actions/reveal';

  type TabId = (typeof editor.tabs)[number]['id'];
  let activeTab = $state<TabId>(editor.tabs[0].id);
  let frameEl: HTMLElement;
  let flashing = $state(false);

  function onTabKeydown(e: KeyboardEvent) {
    const tabs = editor.tabs;
    const idx = tabs.findIndex((t) => t.id === activeTab);
    let next = idx;
    if (e.key === 'ArrowRight') next = (idx + 1) % tabs.length;
    else if (e.key === 'ArrowLeft') next = (idx - 1 + tabs.length) % tabs.length;
    else return;
    e.preventDefault();
    activeTab = tabs[next].id;
    const btn = (e.currentTarget as HTMLElement)?.querySelector<HTMLButtonElement>(
      `[data-tab="${tabs[next].id}"]`,
    );
    btn?.focus();
    btn?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
  }

  function switchTab(id: TabId) {
    if (id === activeTab) return;
    activeTab = id;
    flashing = true;
    // 320ms > transition duration (300ms) to avoid snap at transition end
    setTimeout(() => (flashing = false), 320);
  }
</script>

<section id={editor.id} class="showcase">
  <div class="container">
    <p class="kicker" use:reveal>{editor.kicker}</p>
    <h2 use:reveal={{ delay: 60 }}>{editor.title}</h2>
    <p class="lede" use:reveal={{ delay: 120 }}>{editor.body}</p>

    <p class="eng-label" aria-hidden="true">v1.0 · editor.core</p>

    <div class="frame" class:flash={flashing} bind:this={frameEl}>
      <div class="chrome">
        <span class="dot r"></span>
        <span class="dot y"></span>
        <span class="dot g"></span>
        <div
          class="tabs"
          role="tablist"
          aria-label="Editor mode"
          tabindex="-1"
          onkeydown={onTabKeydown}
        >
          {#each editor.tabs as tab (tab.id)}
            <button
              type="button"
              role="tab"
              class="tab"
              class:active={activeTab === tab.id}
              aria-selected={activeTab === tab.id}
              aria-controls="tabpanel-{tab.id}"
              data-tab={tab.id}
              tabindex={activeTab === tab.id ? 0 : -1}
              onclick={() => switchTab(tab.id)}
            >
              {tab.label}
            </button>
          {/each}
        </div>
      </div>
      {#key activeTab}
        <div
          class="pane"
          id="tabpanel-{activeTab}"
          role="tabpanel"
          aria-label={editor.tabs.find((t) => t.id === activeTab)?.label}
          in:fade={{ duration: 150 }}
        >
          {#if activeTab === 'wysiwyg'}
            <div class="wys">
              <h3>Architecture decisions</h3>
              <p>
                Vedox indexes every Markdown file under a project root and
                serves a <em>local</em> editor. There is no database; files on
                disk <strong>are</strong> the database.
              </p>
              <ul>
                <li>Read-through cache keyed on mtime</li>
                <li>Full-text search via SQLite FTS5</li>
                <li>Zero background network calls</li>
              </ul>
              <pre class="code"><span class="mono">vedox dev --root ./docs</span></pre>
            </div>
          {:else if activeTab === 'source'}
            <div class="src-pane">
              <div class="src">
                <div class="ln">1</div>
                <span class="md"># Architecture decisions</span>
                <div class="ln">2</div>
                <span></span>
                <div class="ln">3</div>
                <span>Vedox indexes every Markdown file under a project root and serves a *local* editor. There is no database; files on disk **are** the database.</span>
                <div class="ln">4</div>
                <span></span>
                <div class="ln">5</div>
                <span>- Read-through cache keyed on mtime</span>
                <div class="ln">6</div>
                <span>- Full-text search via SQLite FTS5</span>
                <div class="ln">7</div>
                <span>- Zero background network calls</span>
                <div class="ln">8</div>
                <span></span>
                <div class="ln">9</div>
                <span class="md">```sh</span>
                <div class="ln">10</div>
                <span>vedox dev --root ./docs</span>
                <div class="ln">11</div>
                <span class="md">```</span>
              </div>
            </div>
          {:else if activeTab === 'mermaid'}
            <div class="mock-pane">
              <div class="mermaid-mock" aria-hidden="true">
                <div class="mermaid-box start">vedox dev</div>
                <div class="mermaid-arrow">&darr;</div>
                <div class="mermaid-box">scan workspace</div>
                <div class="mermaid-arrow">&darr;</div>
                <div class="mermaid-box">index markdown files</div>
                <div class="mermaid-arrow">&darr;</div>
                <div class="mermaid-box">open editor on localhost</div>
                <div class="mermaid-arrow">&darr;</div>
                <div class="mermaid-box end">git commit</div>
              </div>
            </div>
          {:else if activeTab === 'code'}
            <div class="src-pane">
              <pre class="code-block"><code><span class="ln"> 1</span>  <span class="kw">func</span> <span class="fn">NewLocalAdapter</span>(root <span class="tp">string</span>) *<span class="tp">LocalAdapter</span> {'{'}{'\n'}<span class="ln"> 2</span>  	<span class="kw">return</span> &amp;<span class="tp">LocalAdapter</span>{'{'}{'\n'}<span class="ln"> 3</span>  		root: <span class="fn">filepath.Clean</span>(root),{'\n'}<span class="ln"> 4</span>  	{'}'}{'\n'}<span class="ln"> 5</span>  {'}'}{'\n'}<span class="ln"> 6</span>{'\n'}<span class="ln"> 7</span>  <span class="cm">// safePath resolves and validates the path</span>{'\n'}<span class="ln"> 8</span>  <span class="kw">func</span> (a *<span class="tp">LocalAdapter</span>) <span class="fn">safePath</span>(rel <span class="tp">string</span>) (<span class="tp">string</span>, <span class="tp">error</span>) {'{'}{'\n'}<span class="ln"> 9</span>  	abs := <span class="fn">filepath.Join</span>(a.root, rel){'\n'}<span class="ln">10</span>  	<span class="kw">if</span> !<span class="fn">strings.HasPrefix</span>(abs, a.root) {'{'}{'\n'}<span class="ln">11</span>  		<span class="kw">return</span> <span class="str">""</span>, <span class="fn">ErrPathTraversal</span>{'\n'}<span class="ln">12</span>  	{'}'}</code></pre>
            </div>
          {:else if activeTab === 'frontmatter'}
            <div class="mock-pane">
              <div class="fm-panel">
                <div class="fm-field">
                  <span class="fm-label">title</span>
                  <span class="fm-value">Architecture decisions</span>
                </div>
                <div class="fm-field">
                  <span class="fm-label">type</span>
                  <span class="fm-value chip">adr</span>
                </div>
                <div class="fm-field">
                  <span class="fm-label">status</span>
                  <span class="fm-value chip accepted">accepted</span>
                </div>
                <div class="fm-field">
                  <span class="fm-label">date</span>
                  <span class="fm-value">2026-04-09</span>
                </div>
                <div class="fm-field">
                  <span class="fm-label">tags</span>
                  <span class="fm-tags">
                    <span class="tag">architecture</span>
                    <span class="tag">sqlite</span>
                    <span class="tag">design</span>
                  </span>
                </div>
                <div class="fm-lint">
                  <svg viewBox="0 0 24 24" width="14" height="14" aria-hidden="true">
                    <path d="M5 13l4 4L19 7" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" />
                  </svg>
                  16 / 16 lint rules pass
                </div>
              </div>
            </div>
          {/if}
        </div>
      {/key}

      <!-- Tab description below the mock -->
      {#each editor.tabs as tab (tab.id)}
        {#if tab.id === activeTab}
          <p class="tab-desc">{tab.description}</p>
        {/if}
      {/each}
    </div>

    <ul class="callouts">
      {#each editor.callouts as c (c.label)}
        <li>
          <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
            <path
              d="M5 13l4 4l10-10"
              fill="none"
              stroke="currentColor"
              stroke-width="2.2"
              stroke-linecap="round"
              stroke-linejoin="round"
            />
          </svg>
          <span>{c.label}</span>
        </li>
      {/each}
    </ul>
  </div>
</section>

<style>
  .kicker {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    text-transform: uppercase;
    letter-spacing: 0.14em;
    color: var(--color-accent);
    margin-bottom: var(--space-4);
  }
  h2 {
    font-size: clamp(28px, 4vw, 44px);
    margin-bottom: var(--space-5);
    max-width: 22ch;
  }
  .lede {
    font-size: var(--font-size-lg);
    max-width: 60ch;
    margin-bottom: var(--space-6);
  }
  .eng-label {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-text-muted);
    margin-bottom: var(--space-3);
    letter-spacing: 0.06em;
  }
  .frame {
    border: 1px solid var(--color-border);
    border-radius: var(--radius-xl);
    overflow: hidden;
    background: var(--color-surface-elevated);
    box-shadow:
      0 12px 40px rgba(0, 0, 0, 0.08),
      0 0 0 1px color-mix(in srgb, var(--color-accent) 8%, transparent);
    transition: border-color var(--duration-base, 300ms) var(--ease-standard, ease);
  }
  .frame.flash {
    border-color: color-mix(in srgb, var(--color-accent) 60%, transparent);
  }
  .chrome {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: 10px var(--space-4);
    background: var(--color-surface-overlay);
    border-bottom: 1px solid var(--color-border);
  }
  .dot {
    width: 10px;
    height: 10px;
    border-radius: 999px;
  }
  .dot.r { background: #ff5f57; }
  .dot.y { background: #febc2e; }
  .dot.g { background: #28c840; }
  .tabs {
    display: flex;
    gap: 2px;
    margin-left: var(--space-4);
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
  }
  .tab {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-text-muted);
    padding: 4px 10px;
    border-radius: var(--radius-sm);
    background: transparent;
    border: none;
    cursor: pointer;
    transition: color 120ms ease, background 120ms ease;
  }
  .tab:hover {
    color: var(--color-text-secondary);
  }
  .tab.active {
    color: var(--color-accent);
    background: var(--color-accent-subtle);
    box-shadow: inset 0 -2px 0 var(--color-accent);
  }
  .pane {
    min-height: 320px;
  }
  .tab-desc {
    padding: var(--space-4) var(--space-6);
    font-size: var(--font-size-sm);
    color: var(--color-text-muted);
    border-top: 1px solid var(--color-border);
    background: var(--color-surface-overlay);
  }

  /* ---- WYSIWYG pane ---- */
  .wys {
    padding: var(--space-8);
  }
  .wys h3 {
    font-size: var(--font-size-2xl);
    margin-bottom: var(--space-3);
  }
  .wys p { margin-bottom: var(--space-3); }
  .wys ul {
    padding-left: var(--space-5);
    color: var(--color-text-secondary);
    margin-bottom: var(--space-4);
  }
  .wys li { margin-bottom: 4px; }
  .code {
    background: var(--color-surface-base);
    border: 1px solid var(--color-border);
    padding: var(--space-3) var(--space-4);
    border-radius: var(--radius-md);
    font-size: var(--font-size-sm);
  }

  /* ---- Source pane ---- */
  .src-pane {
    padding: 0;
    overflow-x: auto;
  }
  .src {
    display: grid;
    grid-template-columns: 36px 1fr;
    padding: var(--space-4) 0;
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    line-height: 1.8;
    background: var(--color-surface-base);
  }
  .src .ln {
    color: var(--color-text-muted);
    text-align: right;
    padding-right: var(--space-3);
    user-select: none;
  }
  .src span {
    color: var(--color-text-secondary);
    padding-right: var(--space-4);
  }
  .src .md { color: var(--color-accent); }

  /* ---- Code block pre ---- */
  .code-block {
    margin: 0;
    padding: var(--space-6);
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    line-height: 1.8;
    background: var(--color-surface-base);
    color: var(--color-text-secondary);
    overflow-x: auto;
    min-height: 320px;
  }
  .code-block code {
    font-family: inherit;
  }
  .code-block .ln {
    display: inline-block;
    width: 2ch;
    color: var(--color-text-muted);
    text-align: right;
    user-select: none;
    margin-right: var(--space-3);
  }
  .code-block .kw { color: var(--color-accent); }
  .code-block .fn { color: var(--color-info); }
  .code-block .tp { color: var(--color-warning); }
  .code-block .cm { color: var(--color-text-muted); font-style: italic; }
  .code-block .str { color: var(--color-success); }

  /* ---- Mock panes (mermaid, frontmatter) ---- */
  .mock-pane {
    padding: var(--space-8);
    display: flex;
    justify-content: center;
    align-items: center;
    min-height: 320px;
  }

  /* Mermaid mock */
  .mermaid-mock {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-2);
  }
  .mermaid-box {
    padding: var(--space-3) var(--space-6);
    border: 1px solid var(--color-border-strong);
    border-radius: var(--radius-md);
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    color: var(--color-text-primary);
    background: var(--color-surface-base);
    min-width: 200px;
    text-align: center;
  }
  .mermaid-box.start {
    background: var(--color-accent-subtle);
    border-color: var(--color-accent);
    color: var(--color-accent);
    border-radius: 999px;
  }
  .mermaid-box.end {
    background: color-mix(in srgb, var(--color-success) 12%, transparent);
    border-color: var(--color-success);
    color: var(--color-success);
    border-radius: 999px;
  }
  .mermaid-arrow {
    color: var(--color-text-muted);
    font-size: var(--font-size-lg);
  }

  /* Frontmatter mock */
  .fm-panel {
    width: 100%;
    max-width: 400px;
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
  .fm-field {
    display: flex;
    align-items: center;
    gap: var(--space-4);
  }
  .fm-label {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-text-muted);
    min-width: 60px;
    text-align: right;
  }
  .fm-value {
    font-size: var(--font-size-sm);
    color: var(--color-text-primary);
  }
  .fm-value.chip {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    padding: 2px 8px;
    border-radius: var(--radius-sm);
    background: var(--color-surface-overlay);
    color: var(--color-text-secondary);
  }
  .fm-value.accepted {
    background: color-mix(in srgb, var(--color-success) 15%, transparent);
    color: var(--color-success);
  }
  .fm-tags {
    display: flex;
    gap: var(--space-2);
    flex-wrap: wrap;
  }
  .tag {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    padding: 2px 8px;
    border-radius: var(--radius-sm);
    background: var(--color-accent-subtle);
    color: var(--color-accent);
  }
  .fm-lint {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding-top: var(--space-4);
    border-top: 1px solid var(--color-border);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-success);
  }

  /* ---- Callouts ---- */
  .callouts {
    list-style: none;
    margin-top: var(--space-8);
    display: grid;
    gap: var(--space-3);
    grid-template-columns: 1fr;
  }
  @media (min-width: 760px) {
    .callouts {
      grid-template-columns: repeat(2, 1fr);
    }
  }
  .callouts li {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    color: var(--color-text-secondary);
  }
  .callouts svg {
    color: var(--color-accent);
    flex-shrink: 0;
  }
</style>
