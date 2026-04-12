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
  const TYPING_TEXT = 'Documentation that lives in your repo.';

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

    // Typing animation for mock editor
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
    <ellipse class="aurora-1" cx={`${30 + mx * 10}%`} cy={`${20 + my * 8}%`} rx="35%" ry="25%" fill="#818cf8" opacity="0.18" filter="url(#aurora-blur)" />
    <ellipse class="aurora-2" cx="72%" cy="40%" rx="28%" ry="20%" fill="#a78bfa" opacity="0.12" filter="url(#aurora-blur)" />
    <ellipse class="aurora-3" cx="50%" cy="-5%" rx="40%" ry="20%" fill="#34d399" opacity="0.06" filter="url(#aurora-blur)" />
  </svg>

  <div class="hero-noise" aria-hidden="true"></div>

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
      <div class="screenshot">
        <div class="chrome">
          <span class="dot r"></span>
          <span class="dot y"></span>
          <span class="dot g"></span>
          <span class="tab">README.md</span>
        </div>
        <div class="body">
          <aside class="files">
            <p class="f-title">docs</p>
            <ul>
              <li class="active">README.md</li>
              <li>getting-started.md</li>
              <li>architecture.md</li>
              <li>contributing.md</li>
              <li>adr/001-...md</li>
            </ul>
          </aside>
          <div class="doc">
            <h3>Vedox</h3>
            <p bind:this={typingTarget} style="visibility:hidden">{TYPING_TEXT}</p>
            <p class="muted">A local-first docs CMS for developers.</p>
            <pre><span class="kw">$</span> {site.runCommand}</pre>
            <p>Edit. Save. <code>git commit</code>. That's the loop.</p>
          </div>
        </div>
      </div>
    </div>
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
  .screenshot-wrap {
    position: relative;
    margin: 0 auto;
    max-width: 960px;
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
  .screenshot {
    border: 1px solid var(--color-border);
    border-radius: var(--radius-xl);
    overflow: hidden;
    background: var(--color-surface-elevated);
    box-shadow:
      0 16px 48px color-mix(in srgb, var(--color-accent) 14%, transparent),
      0 4px 12px rgba(0, 0, 0, 0.08);
    text-align: left;
    position: relative;
  }
  /* Scanline overlay on the screenshot */
  .screenshot::after {
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
    z-index: 2;
    border-radius: var(--radius-xl);
  }
  :global([data-theme='dark']) .screenshot::after {
    background-image: repeating-linear-gradient(
      to bottom,
      transparent,
      transparent 3px,
      rgba(255, 255, 255, 0.02) 3px,
      rgba(255, 255, 255, 0.02) 4px
    );
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
  .tab {
    margin-left: var(--space-4);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-text-muted);
  }
  .body {
    display: grid;
    grid-template-columns: 200px 1fr;
    min-height: 320px;
  }
  .files {
    background: var(--color-surface-base);
    border-right: 1px solid var(--color-border);
    padding: var(--space-4);
  }
  .f-title {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-text-muted);
    text-transform: uppercase;
    letter-spacing: 0.1em;
    margin-bottom: var(--space-3);
  }
  .files ul {
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .files li {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-text-secondary);
    padding: 6px 10px;
    border-radius: var(--radius-sm);
  }
  .files li.active {
    background: var(--color-accent-subtle);
    color: var(--color-accent);
    border-left: 2px solid var(--color-accent);
    padding-left: 8px;
  }
  .doc {
    padding: var(--space-8);
  }
  .doc h3 {
    font-size: 28px;
    margin-bottom: var(--space-3);
  }
  .doc p {
    margin-bottom: var(--space-3);
  }
  .doc .muted {
    color: var(--color-text-muted);
  }
  .doc pre {
    background: var(--color-surface-base);
    border: 1px solid var(--color-border);
    padding: var(--space-3) var(--space-4);
    border-radius: var(--radius-md);
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    margin: var(--space-4) 0;
    overflow-x: auto;
  }
  .doc .kw {
    color: var(--color-accent);
    margin-right: 8px;
  }
  :global(.type-cursor) {
    animation: blink 0.7s step-end infinite;
    color: var(--color-accent);
  }
  @keyframes blink {
    50% { opacity: 0; }
  }
  @media (max-width: 640px) {
    .body { grid-template-columns: 1fr; }
    .files { display: none; }
  }
</style>
