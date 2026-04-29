<script lang="ts">
  import { onMount } from 'svelte';
  import { hero, site } from '$lib/content';
  import InstallCommand from './InstallCommand.svelte';
  import { track } from '$lib/analytics';

  let heroEl: HTMLElement;
  let h1El: HTMLElement;
  let typingTarget: HTMLElement;
  let mx = $state(0.5);
  let my = $state(0.5);
  let rafPending = false;
  const TYPING_TEXT = 'documentation that lives in your repo.';

  onMount(() => {
    // Word-stagger headline
    const prefersReduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (!prefersReduced && h1El) {
      const words = h1El.textContent?.split(' ') ?? [];
      h1El.innerHTML = words
        .map((w, i) => `<span class="hw" style="--i:${i}">${w}</span>`)
        .join(' ');
      // Double-rAF: first frame paints opacity:0, second triggers the transition
      requestAnimationFrame(() => {
        requestAnimationFrame(() => {
          h1El.querySelectorAll('.hw').forEach((el) => el.classList.add('visible'));
        });
      });
    }

    // Typing animation for mock editor subtitle
    if (!prefersReduced && typingTarget) {
      typingTarget.textContent = '';
      typingTarget.style.visibility = 'visible';
      let i = 0;
      const cursor = document.createElement('span');
      cursor.className = 'type-cursor';
      cursor.textContent = '|';
      typingTarget.appendChild(cursor);
      setTimeout(() => {
        const interval = setInterval(() => {
          if (i < TYPING_TEXT.length) {
            typingTarget.insertBefore(document.createTextNode(TYPING_TEXT[i]), cursor);
            i++;
          } else {
            clearInterval(interval);
            setTimeout(() => cursor.remove(), 1200);
          }
        }, 28);
      }, 900);
    }
  });

  function handleMouseMove(e: MouseEvent) {
    if (rafPending) return;
    rafPending = true;
    requestAnimationFrame(() => {
      rafPending = false;
      const rect = heroEl?.getBoundingClientRect();
      if (!rect) return;
      mx = (e.clientX - rect.left) / rect.width;
      my = (e.clientY - rect.top) / rect.height;
    });
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<section id="top" class="hero" bind:this={heroEl} onmousemove={handleMouseMove}>
  <!-- Aurora SVG layer -->
  <svg class="aurora" aria-hidden="true" viewBox="0 0 1200 600" preserveAspectRatio="xMidYMid slice">
    <defs>
      <filter id="aurora-blur">
        <feGaussianBlur stdDeviation="60" />
      </filter>
    </defs>
    <ellipse class="aurora-1" cx={`${30 + mx * 10}%`} cy={`${20 + my * 8}%`} rx="35%" ry="25%" fill="#818cf8" opacity="0.22" filter="url(#aurora-blur)" />
    <ellipse class="aurora-2" cx="72%" cy="40%" rx="28%" ry="20%" fill="#a78bfa" opacity="0.18" filter="url(#aurora-blur)" />
    <ellipse class="aurora-3" cx="50%" cy="-5%" rx="40%" ry="20%" fill="#34d399" opacity="0.10" filter="url(#aurora-blur)" />
  </svg>

  <div class="hero-noise" aria-hidden="true"></div>

  <!-- Split-folio — prominent atmospheric book behind the hero content -->
  <div class="folio-bg" aria-hidden="true">
    <img
      src="/vedox-logo-09-split-folio-rembg.png"
      alt=""
      loading="eager"
      decoding="async"
    />
    <!-- Soft gradient fade at the bottom edge so the book blends into the section -->
    <div class="folio-fade"></div>
  </div>

  <div class="container hero-inner">
    <p class="eyebrow"><span class="eyebrow-path">{hero.eyebrow}</span></p>
    <h1 bind:this={h1El}>{hero.headline}</h1>
    <p class="sub">{hero.sub}</p>
    <div class="ctas">
      <InstallCommand location="hero" />
      <a
        class="secondary"
        href={hero.secondaryCta.href}
        rel="noopener"
        onclick={() => track('GitHub Click', { location: 'hero' })}
      >
        {hero.secondaryCta.label}
        <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true">
          <path
            d="M7 17 L17 7 M9 7 H17 V15"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
          />
        </svg>
      </a>
    </div>
    <p class="trust">{hero.trustLine}</p>

    <div class="screenshot-wrap" aria-hidden="true">
      <div class="mock-frame">

        <!-- Mac window chrome -->
        <div class="mock-titlebar">
          <div class="mock-dots">
            <span class="mock-dot" style="background:#ff5f57"></span>
            <span class="mock-dot" style="background:#febc2e"></span>
            <span class="mock-dot" style="background:#28c840"></span>
          </div>
          <span class="mock-title">Vedox — docs/README.md</span>
          <div class="mock-titlebar-right"></div>
        </div>

        <!-- App body: activity bar + sidebar + editor -->
        <div class="mock-body">

          <!-- Activity bar -->
          <div class="mock-activity">
            <!-- Files icon (active) -->
            <button class="mock-act-btn active" title="Files">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                <path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"/>
                <polyline points="13 2 13 9 20 9"/>
              </svg>
            </button>
            <!-- Search icon -->
            <button class="mock-act-btn" title="Search">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="11" cy="11" r="8"/>
                <line x1="21" y1="21" x2="16.65" y2="16.65"/>
              </svg>
            </button>
            <!-- Git icon -->
            <button class="mock-act-btn" title="Git">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="18" cy="18" r="3"/>
                <circle cx="6" cy="6" r="3"/>
                <path d="M13 6h3a2 2 0 0 1 2 2v7"/>
                <line x1="6" y1="9" x2="6" y2="21"/>
              </svg>
            </button>
            <div class="mock-act-spacer"></div>
            <!-- Settings icon -->
            <button class="mock-act-btn" title="Settings">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="12" cy="12" r="3"/>
                <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>
              </svg>
            </button>
          </div>

          <!-- Sidebar -->
          <div class="mock-sidebar">
            <div class="mock-sidebar-header">docs</div>
            <ul class="mock-tree">
              <li class="mock-tree-folder">
                <svg width="10" height="10" viewBox="0 0 10 10" fill="currentColor"><path d="M2 3l4 4 4-4" stroke="currentColor" stroke-width="1.2" fill="none" stroke-linecap="round"/></svg>
                docs/
              </li>
              <li class="mock-tree-file active">
                <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>
                README.md
              </li>
              <li class="mock-tree-file">
                <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>
                getting-started.md
              </li>
              <li class="mock-tree-file">
                <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>
                architecture.md
              </li>
              <li class="mock-tree-file">
                <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>
                contributing.md
              </li>
              <li class="mock-tree-folder">
                <svg width="10" height="10" viewBox="0 0 10 10" fill="currentColor"><path d="M2 3l4 4 4-4" stroke="currentColor" stroke-width="1.2" fill="none" stroke-linecap="round"/></svg>
                adr/
              </li>
              <li class="mock-tree-file indent">
                <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>
                001-local-first.md
              </li>
              <li class="mock-tree-file indent">
                <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>
                002-editor-choice.md
              </li>
              <li class="mock-tree-folder">
                <svg width="10" height="10" viewBox="0 0 10 10" fill="currentColor"><path d="M2 3l4 4 4-4" stroke="currentColor" stroke-width="1.2" fill="none" stroke-linecap="round"/></svg>
                api/
              </li>
              <li class="mock-tree-file indent">
                <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>
                endpoints.md
              </li>
            </ul>
            <div class="mock-search-stub">
              <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
              <span>Search docs...</span>
              <kbd>⌘K</kbd>
            </div>
          </div>

          <!-- Editor area -->
          <div class="mock-editor">

            <!-- Tab bar -->
            <div class="mock-tabbar">
              <div class="mock-tabs">
                <div class="mock-tab active">
                  <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>
                  README.md
                  <span class="mock-tab-close">×</span>
                </div>
                <div class="mock-tab">
                  <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>
                  architecture.md
                  <span class="mock-tab-close">×</span>
                </div>
              </div>
              <div class="mock-mode-toggle">
                <span class="mock-mode active">WYSIWYG</span>
                <span class="mock-mode">Source</span>
              </div>
            </div>

            <!-- Document content (WYSIWYG rendered) -->
            <div class="mock-doc-area">

              <!-- Floating bubble toolbar (selection simulation) -->
              <div class="mock-bubble-toolbar">
                <button class="mock-bubble-btn" aria-label="Bold"><strong>B</strong></button>
                <button class="mock-bubble-btn" aria-label="Italic"><em>I</em></button>
                <button class="mock-bubble-btn" aria-label="Link">
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/></svg>
                </button>
                <button class="mock-bubble-btn" aria-label="Code">
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2"><polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/></svg>
                </button>
              </div>

              <div class="mock-doc-content">
                <h1 class="mock-h1">Vedox</h1>
                <p class="mock-subtitle" bind:this={typingTarget} style="visibility:hidden">{TYPING_TEXT}</p>

                <h2 class="mock-h2">Getting Started</h2>
                <p class="mock-p">Run <code class="mock-inline-code">vedox dev</code> in any folder with Markdown files. Vedox scans, indexes, and opens a local editor at <code class="mock-inline-code">http://127.0.0.1:4123</code>. No config required.</p>

                <h2 class="mock-h2">Installation</h2>
                <div class="mock-code-block">
                  <div class="mock-code-header">
                    <span class="mock-code-lang">shell</span>
                    <button class="mock-code-copy">copy</button>
                  </div>
                  <pre class="mock-pre"><span class="mock-kw">$</span> brew install thepixelabs/tap/vedox
<span class="mock-kw">$</span> vedox dev</pre>
                </div>

                <h2 class="mock-h2">What it does</h2>
                <p class="mock-p">Vedox reads and writes the same Markdown files that Git already tracks. Close the tab, run <code class="mock-inline-code">git diff</code>, commit. That's the loop.</p>
              </div>
            </div>

            <!-- Status bar -->
            <div class="mock-statusbar">
              <div class="mock-status-left">
                <span class="mock-status-item">README.md</span>
                <span class="mock-status-sep">·</span>
                <span class="mock-status-item">markdown</span>
                <span class="mock-status-sep">·</span>
                <span class="mock-status-item mock-status-saved">
                  <svg width="9" height="9" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"/></svg>
                  autosaved
                </span>
              </div>
              <div class="mock-status-right">
                <span class="mock-status-item">312 words</span>
                <span class="mock-status-sep">·</span>
                <span class="mock-status-item mock-status-lint">lint 0/16</span>
                <span class="mock-status-sep">·</span>
                <span class="mock-status-item mock-status-mode">WYSIWYG</span>
              </div>
            </div>

          </div><!-- /mock-editor -->
        </div><!-- /mock-body -->
      </div><!-- /mock-frame -->
    </div><!-- /screenshot-wrap -->
  </div>
</section>

<style>
  .hero {
    position: relative;
    padding-top: clamp(56px, 8vw, 100px);
    padding-bottom: clamp(48px, 7vw, 80px);
    overflow: hidden;
  }
  .aurora {
    position: absolute;
    inset: -20% -10%;
    width: 120%;
    height: 120%;
    pointer-events: none;
    z-index: 0;
  }
  .aurora-1 {
    animation: aurora-drift-1 18s ease-in-out infinite alternate;
  }
  .aurora-2 {
    animation: aurora-drift-2 22s ease-in-out infinite alternate;
  }
  .aurora-3 {
    animation: aurora-drift-3 26s ease-in-out infinite alternate;
  }
  @keyframes aurora-drift-1 {
    from { transform: translate(0, 0) scale(1); }
    to { transform: translate(4%, 3%) scale(1.08); }
  }
  @keyframes aurora-drift-2 {
    from { transform: translate(0, 0) scale(1); }
    to { transform: translate(-5%, 4%) scale(0.95); }
  }
  @keyframes aurora-drift-3 {
    from { transform: translate(0, 0) scale(1); }
    to { transform: translate(3%, -4%) scale(1.05); }
  }
  @media (prefers-reduced-motion: reduce) {
    .aurora-1,
    .aurora-2,
    .aurora-3 {
      animation: none;
      /* Freeze at mid-cycle transform — don't strip to 0 */
      transform: translate(2%, 1.5%) scale(1.04);
    }
  }
  .hero-noise {
    position: absolute;
    inset: 0;
    background-image: var(--noise-bg);
    background-repeat: repeat;
    background-size: 200px 200px;
    opacity: 0.04;
    pointer-events: none;
    z-index: 0;
    mix-blend-mode: overlay;
  }
  :global([data-theme='dark']) .hero-noise {
    opacity: 0.07;
  }
  /* ---- Split-folio background ---- */
  .folio-bg {
    position: absolute;
    /* Anchor right side; wide enough to be prominent without blocking the headline */
    right: -4%;
    top: 0;
    bottom: 0;
    width: 58%;
    pointer-events: none;
    z-index: 0;
    display: flex;
    align-items: center;
    justify-content: flex-end;
    opacity: 0.20;
  }
  .folio-bg img {
    width: 100%;
    height: 85%;
    object-fit: contain;
    object-position: center right;
    max-width: none;
    mix-blend-mode: normal;
  }
  /* Bottom fade — blends the book into the section boundary */
  .folio-fade {
    position: absolute;
    bottom: 0;
    left: 0;
    right: 0;
    height: 30%;
    background: linear-gradient(to top, var(--color-surface-base, #0a0a0a), transparent);
    pointer-events: none;
  }
  /* On mobile the book is too wide — hide it so text stays readable */
  @media (max-width: 768px) {
    .folio-bg {
      display: none;
    }
  }
  /* On tablet, pull opacity down slightly so it doesn't crowd the headline */
  @media (min-width: 769px) and (max-width: 1024px) {
    .folio-bg {
      opacity: 0.13;
    }
  }

  .hero-inner {
    position: relative;
    z-index: 1;
    text-align: center;
  }
  .eyebrow {
    display: inline-block;
    margin-bottom: var(--space-6);
  }
  .eyebrow-path {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    letter-spacing: 0.08em;
    color: var(--color-accent);
    background: var(--color-accent-subtle);
    padding: 6px 14px;
    border-radius: 999px;
    border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
  }
  h1 {
    font-size: var(--mkt-display);
    line-height: var(--mkt-display-line);
    max-width: 18ch;
    margin: 0 auto var(--space-6);
    overflow: hidden;
  }
  :global(.hw) {
    display: inline-block;
    opacity: 0;
    transform: translateY(16px) skewY(2deg);
    transition:
      opacity var(--duration-entrance) var(--ease-expo-out) calc(var(--i) * 80ms),
      transform var(--duration-entrance) var(--ease-expo-out) calc(var(--i) * 80ms);
  }
  :global(.hw.visible) {
    opacity: 1;
    transform: translateY(0) skewY(0);
  }
  .sub {
    font-size: clamp(16px, 2vw, 20px);
    max-width: 60ch;
    margin: 0 auto var(--space-8);
    color: var(--color-text-secondary);
  }
  .ctas {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-4);
    justify-content: center;
    align-items: center;
    margin-bottom: var(--space-6);
  }
  .secondary {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    padding: 14px 18px;
    border-radius: var(--radius-lg);
    border: 1px solid var(--color-border-strong);
    color: var(--color-text-primary);
    font-weight: 500;
  }
  .secondary:hover {
    border-color: var(--color-accent);
    text-decoration: none;
  }
  .trust {
    font-size: var(--font-size-sm);
    color: var(--color-text-muted);
    margin-bottom: var(--space-12);
  }

  /* ── Screenshot wrapper ─────────────────────────────────────── */
  .screenshot-wrap {
    position: relative;
    margin: 0 auto;
    max-width: 1040px;
  }
  .screenshot-wrap::before {
    content: '';
    position: absolute;
    inset: -40px -60px;
    background: radial-gradient(
      ellipse at 50% 50%,
      color-mix(in srgb, var(--color-accent) 18%, transparent),
      transparent 70%
    );
    border-radius: 50%;
    z-index: -1;
    pointer-events: none;
  }

  /* ── App frame ──────────────────────────────────────────────── */
  .mock-frame {
    border: 1px solid rgba(129, 140, 248, 0.2);
    border-radius: var(--radius-xl);
    overflow: hidden;
    /* Glass layer — lets the folio book bleed through from behind */
    background: rgba(11, 11, 14, 0.72);
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
    box-shadow:
      0 0 80px -20px rgba(129, 140, 248, 0.25),
      0 0 140px -40px rgba(129, 140, 248, 0.15),
      0 24px 80px color-mix(in srgb, var(--color-accent) 18%, transparent),
      0 8px 24px rgba(0, 0, 0, 0.24);
    text-align: left;
    position: relative;
    min-height: 520px;
    display: flex;
    flex-direction: column;
  }
  /* Scanline overlay */
  .mock-frame::after {
    content: '';
    position: absolute;
    inset: 0;
    background-image: repeating-linear-gradient(
      to bottom,
      transparent,
      transparent 3px,
      rgba(0, 0, 0, 0.04) 3px,
      rgba(0, 0, 0, 0.04) 4px
    );
    pointer-events: none;
    z-index: 10;
    border-radius: var(--radius-xl);
  }
  :global([data-theme='dark']) .mock-frame::after {
    background-image: repeating-linear-gradient(
      to bottom,
      transparent,
      transparent 3px,
      rgba(255, 255, 255, 0.02) 3px,
      rgba(255, 255, 255, 0.02) 4px
    );
  }

  /* ── Mac title bar ──────────────────────────────────────────── */
  .mock-titlebar {
    display: flex;
    align-items: center;
    height: 36px;
    padding: 0 var(--space-4);
    background: rgba(22, 22, 28, 0.78);
    border-bottom: 1px solid rgba(129, 140, 248, 0.12);
    flex-shrink: 0;
    gap: var(--space-3);
  }
  .mock-dots {
    display: flex;
    gap: 6px;
    align-items: center;
    flex-shrink: 0;
  }
  .mock-dot {
    width: 11px;
    height: 11px;
    border-radius: 50%;
    opacity: 0.85;
  }
  .mock-title {
    flex: 1;
    text-align: center;
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-text-muted);
    letter-spacing: 0.03em;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .mock-titlebar-right {
    width: 52px;
    flex-shrink: 0;
  }

  /* ── App body (3-col layout) ────────────────────────────────── */
  .mock-body {
    display: flex;
    flex: 1;
    overflow: hidden;
  }

  /* ── Activity bar ───────────────────────────────────────────── */
  .mock-activity {
    width: 40px;
    flex-shrink: 0;
    background: rgba(22, 22, 28, 0.76);
    border-right: 1px solid rgba(129, 140, 248, 0.1);
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: var(--space-2) 0;
    gap: 2px;
  }
  .mock-act-btn {
    width: 36px;
    height: 36px;
    display: flex;
    align-items: center;
    justify-content: center;
    border: none;
    background: transparent;
    border-radius: var(--radius-sm);
    color: var(--color-text-muted);
    cursor: default;
    padding: 0;
    transition: color 0.15s;
  }
  .mock-act-btn.active {
    color: var(--color-accent);
  }
  .mock-act-spacer {
    flex: 1;
  }

  /* ── Sidebar ────────────────────────────────────────────────── */
  .mock-sidebar {
    width: 200px;
    flex-shrink: 0;
    background: rgba(11, 11, 14, 0.74);
    border-right: 1px solid rgba(129, 140, 248, 0.1);
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .mock-sidebar-header {
    font-family: var(--font-mono);
    font-size: 10px;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--color-text-muted);
    padding: var(--space-3) var(--space-3) var(--space-2);
    flex-shrink: 0;
  }
  .mock-tree {
    list-style: none;
    flex: 1;
    overflow-y: auto;
    padding: 0 var(--space-2) var(--space-2);
    display: flex;
    flex-direction: column;
    gap: 1px;
  }
  .mock-tree-folder,
  .mock-tree-file {
    display: flex;
    align-items: center;
    gap: 5px;
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-text-secondary);
    padding: 4px 6px;
    border-radius: var(--radius-sm);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    cursor: default;
    border-left: 2px solid transparent;
  }
  .mock-tree-folder {
    color: var(--color-text-muted);
    font-size: 10px;
    letter-spacing: 0.04em;
    padding-top: var(--space-2);
  }
  .mock-tree-file.indent {
    padding-left: 18px;
  }
  .mock-tree-file.active {
    background: var(--color-accent-subtle);
    color: var(--color-accent);
    border-left-color: var(--color-accent);
    padding-left: 4px;
  }
  .mock-search-stub {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    margin: var(--space-2);
    border-radius: var(--radius-sm);
    border: 1px solid var(--color-border);
    background: var(--color-surface-elevated);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-text-muted);
    flex-shrink: 0;
  }
  .mock-search-stub span {
    flex: 1;
  }
  .mock-search-stub kbd {
    font-family: var(--font-mono);
    font-size: 9px;
    background: var(--color-surface-overlay);
    border: 1px solid var(--color-border);
    border-radius: 3px;
    padding: 1px 4px;
    color: var(--color-text-muted);
  }

  /* ── Editor area ────────────────────────────────────────────── */
  .mock-editor {
    flex: 1;
    display: flex;
    flex-direction: column;
    background: rgba(16, 16, 20, 0.72);
    overflow: hidden;
    min-width: 0;
  }

  /* Tab bar */
  .mock-tabbar {
    display: flex;
    align-items: stretch;
    height: 36px;
    background: rgba(11, 11, 14, 0.7);
    border-bottom: 1px solid rgba(129, 140, 248, 0.1);
    flex-shrink: 0;
    justify-content: space-between;
  }
  .mock-tabs {
    display: flex;
    align-items: stretch;
    overflow: hidden;
  }
  .mock-tab {
    display: flex;
    align-items: center;
    gap: 5px;
    padding: 0 var(--space-3);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-text-muted);
    border-right: 1px solid var(--color-border);
    cursor: default;
    white-space: nowrap;
    border-bottom: 2px solid transparent;
  }
  .mock-tab.active {
    color: var(--color-text-primary);
    background: rgba(16, 16, 20, 0.72);
    border-bottom-color: var(--color-accent);
  }
  .mock-tab-close {
    font-size: 14px;
    line-height: 1;
    color: var(--color-text-muted);
    opacity: 0.5;
  }
  .mock-mode-toggle {
    display: flex;
    align-items: center;
    gap: 1px;
    padding: 0 var(--space-3);
    margin: auto 0;
    background: var(--color-surface-overlay);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    margin-right: var(--space-3);
    height: 24px;
    overflow: hidden;
    align-self: center;
    flex-shrink: 0;
  }
  .mock-mode {
    font-family: var(--font-mono);
    font-size: 10px;
    letter-spacing: 0.04em;
    color: var(--color-text-muted);
    padding: 0 var(--space-2);
    height: 100%;
    display: flex;
    align-items: center;
    border-radius: var(--radius-sm);
  }
  .mock-mode.active {
    background: var(--color-accent);
    color: var(--color-text-inverse);
  }

  /* Document area */
  .mock-doc-area {
    flex: 1;
    overflow-y: auto;
    padding: var(--space-8) var(--space-8) var(--space-6);
    position: relative;
  }

  /* Bubble toolbar */
  .mock-bubble-toolbar {
    position: absolute;
    top: var(--space-6);
    right: var(--space-8);
    display: flex;
    align-items: center;
    gap: 2px;
    background: var(--color-surface-overlay);
    border: 1px solid var(--color-border-strong);
    border-radius: 999px;
    padding: 4px 8px;
    box-shadow: var(--shadow-md);
    z-index: 3;
  }
  .mock-bubble-btn {
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    border: none;
    background: transparent;
    border-radius: 999px;
    font-family: var(--font-sans);
    font-size: var(--font-size-xs);
    color: var(--color-text-secondary);
    cursor: default;
    padding: 0;
  }
  .mock-bubble-btn:first-child {
    background: var(--color-accent-subtle);
    color: var(--color-accent);
  }

  /* Document typography */
  .mock-doc-content {
    font-family: var(--font-sans);
    max-width: 68ch;
  }
  .mock-h1 {
    font-size: 32px;
    font-weight: 700;
    line-height: 1.15;
    color: var(--color-text-primary);
    margin: 0 0 var(--space-2);
    letter-spacing: -0.02em;
  }
  .mock-subtitle {
    font-size: var(--font-size-lg);
    color: var(--color-text-secondary);
    margin: 0 0 var(--space-8);
    line-height: 1.5;
  }
  .mock-h2 {
    font-size: var(--font-size-xl);
    font-weight: 600;
    color: var(--color-text-primary);
    margin: var(--space-8) 0 var(--space-3);
    letter-spacing: -0.01em;
    padding-bottom: var(--space-2);
    border-bottom: 1px solid var(--color-border);
  }
  .mock-p {
    font-size: var(--font-size-base);
    line-height: 1.7;
    color: var(--color-text-secondary);
    margin: 0 0 var(--space-4);
  }
  .mock-inline-code {
    font-family: var(--font-mono);
    font-size: 0.88em;
    background: var(--color-surface-overlay);
    color: var(--color-accent);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 1px 5px;
  }
  .mock-code-block {
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    overflow: hidden;
    margin: var(--space-4) 0;
  }
  .mock-code-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 6px var(--space-4);
    background: var(--color-surface-overlay);
    border-bottom: 1px solid var(--color-border);
  }
  .mock-code-lang {
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--color-text-muted);
    letter-spacing: 0.08em;
    text-transform: uppercase;
  }
  .mock-code-copy {
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--color-text-muted);
    background: transparent;
    border: none;
    cursor: default;
    padding: 0;
  }
  .mock-pre {
    background: var(--color-surface-base);
    padding: var(--space-4);
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    color: var(--color-text-secondary);
    margin: 0;
    line-height: 1.7;
    overflow-x: auto;
  }
  .mock-kw {
    color: var(--color-accent);
    user-select: none;
  }

  /* Status bar */
  .mock-statusbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 28px;
    padding: 0 var(--space-4);
    background: rgba(22, 22, 28, 0.78);
    border-top: 1px solid rgba(129, 140, 248, 0.1);
    flex-shrink: 0;
    gap: var(--space-4);
  }
  .mock-status-left,
  .mock-status-right {
    display: flex;
    align-items: center;
    gap: 4px;
    overflow: hidden;
  }
  .mock-status-item {
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--color-text-muted);
    white-space: nowrap;
    display: flex;
    align-items: center;
    gap: 3px;
  }
  .mock-status-sep {
    font-size: 10px;
    color: var(--color-border-strong);
  }
  .mock-status-saved {
    color: var(--color-success);
  }
  .mock-status-lint {
    color: var(--color-success);
  }
  .mock-status-mode {
    color: var(--color-accent);
  }

  /* Typing cursor */
  :global(.type-cursor) {
    animation: blink 0.7s step-end infinite;
    color: var(--color-accent);
  }
  @keyframes blink {
    50% { opacity: 0; }
  }

  /* ── Responsive ─────────────────────────────────────────────── */
  @media (max-width: 768px) {
    .mock-activity,
    .mock-sidebar {
      display: none;
    }
  }
  @media (max-width: 480px) {
    .mock-tabbar {
      display: none;
    }
    .mock-status-right {
      display: none;
    }
    .mock-bubble-toolbar {
      display: none;
    }
    .mock-doc-area {
      padding: var(--space-4);
    }
  }
</style>
