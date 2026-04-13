<script lang="ts">
  import { workflow } from '$lib/content';
  import { reveal } from '$lib/actions/reveal';

  const delays = [0, 80, 160];

  function mdInline(text: string): string {
    return text.replace(/`([^`]+)`/g, '<code>$1</code>');
  }

  const terminalBlocks = [
    {
      lines: [
        { kind: 'prompt', text: 'vedox dev' },
        { kind: 'output', text: 'scanning docs/ \u2026 24 files indexed in 340ms' },
        { kind: 'output', text: 'opening http://localhost:4321' },
      ],
    },
    {
      lines: [
        { kind: 'tree', text: 'docs/' },
        { kind: 'tree-file-active', text: '  architecture/adr-012.md', marker: '\u2190 editing' },
        { kind: 'tree-file', text: '  api/endpoints.md' },
        { kind: 'tree-file', text: '  runbooks/deploy.md' },
      ],
    },
    {
      lines: [
        { kind: 'prompt', text: 'git diff docs/architecture/adr-012.md' },
        { kind: 'prompt', text: 'git commit -m "docs: add ADR 012 \u2014 async queue"' },
        { kind: 'success', text: '[main 3f8a2b1] docs: add ADR 012 \u2014 async queue' },
      ],
    },
  ] as const;
</script>

<section id={workflow.id} class="workflow">
  <!-- Map image — full-bleed atmospheric background -->
  <img
    class="map-bg"
    src="/vedox-logo-06-cartographers-repo-final.png"
    alt=""
    aria-hidden="true"
    loading="lazy"
    decoding="async"
  />
  <!-- Aurora SVG — indigo upper-center + green lower-left -->
  <svg class="aurora" aria-hidden="true">
    <defs>
      <filter id="workflow-aurora-blur"><feGaussianBlur stdDeviation="60"/></filter>
    </defs>
    <ellipse class="wf-aurora-1" cx="55%" cy="10%" rx="50%" ry="18%" fill="#818cf8" opacity="0.14" filter="url(#workflow-aurora-blur)"/>
    <ellipse class="wf-aurora-2" cx="20%" cy="60%" rx="30%" ry="15%" fill="#34d399" opacity="0.08" filter="url(#workflow-aurora-blur)"/>
  </svg>

  <div class="container">
    <p class="kicker" use:reveal>{workflow.kicker}</p>
    <h2 use:reveal={{ delay: 60 }}>{workflow.title}</h2>

    <!-- Horizontal timeline spine (desktop) -->
    <div class="spine-wrap" aria-hidden="true">
      <div class="spine-line"></div>
      {#each workflow.steps as s, i (s.n)}
        <div class="spine-marker" style="left: calc({i} * 33.333% + 16.666%)">
          <span class="spine-num">{s.n}</span>
        </div>
      {/each}
    </div>

    <ol>
      {#each workflow.steps as s, i (s.n)}
        <li use:reveal={{ delay: delays[i] ?? 0 }}>
          <!-- Zone A: terminal / file tree output -->
          <div class="terminal-zone">
            <pre class="terminal-pre" aria-label="Terminal output for step {s.n}">{#each terminalBlocks[i].lines as line}{#if line.kind === 'prompt'}<span class="t-prompt">$</span> <span class="t-cmd">{line.text}</span>{'\n'}{:else if line.kind === 'success'}<span class="t-success">{line.text}</span>{'\n'}{:else if line.kind === 'tree'}<span class="t-output">{line.text}</span>{'\n'}{:else if line.kind === 'tree-file-active'}<span class="t-output">{line.text}</span>  <span class="t-accent">{line.marker}</span>{'\n'}{:else if line.kind === 'tree-file'}<span class="t-output">{line.text}</span>{'\n'}{:else}<span class="t-output">{line.text}</span>{'\n'}{/if}{/each}</pre>
          </div>
          <!-- Zone B: title + body -->
          <div class="card-body">
            <span class="step-n">{s.n}</span>
            <h3>{s.title}</h3>
            <p>{@html mdInline(s.body)}</p>
          </div>
        </li>
      {/each}
    </ol>
  </div>
</section>

<style>
  .workflow {
    position: relative;
    overflow: hidden;
  }

  /* ---- Map background ---- */
  .map-bg {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
    object-position: center;
    opacity: 0.045;
    filter: grayscale(100%);
    mix-blend-mode: luminosity;
    mask-image: radial-gradient(ellipse 80% 80% at 50% 50%, black 40%, transparent 100%);
    -webkit-mask-image: radial-gradient(ellipse 80% 80% at 50% 50%, black 40%, transparent 100%);
    pointer-events: none;
    z-index: 0;
  }
  @media (max-width: 767px) {
    .map-bg { display: none; }
  }
  @media (min-width: 768px) and (max-width: 1023px) {
    .map-bg { opacity: 0.03; }
  }

  /* ---- Aurora layer (z-index: 1) ---- */
  .aurora {
    position: absolute;
    inset: -10% -5%;
    width: 110%;
    height: 120%;
    pointer-events: none;
    z-index: 1;
  }
  .wf-aurora-1 {
    animation: wf-drift-1 24s ease-in-out infinite alternate;
    animation-delay: -7s;
  }
  .wf-aurora-2 {
    animation: wf-drift-2 20s ease-in-out infinite alternate;
    animation-delay: -3s;
  }
  @keyframes wf-drift-1 {
    from { transform: translate(0, 0) scale(1); }
    to   { transform: translate(3%, -2%) scale(1.04); }
  }
  @keyframes wf-drift-2 {
    from { transform: translate(0, 0) scale(1); }
    to   { transform: translate(-3%, 2%) scale(1.03); }
  }
  @media (prefers-reduced-motion: reduce) {
    .wf-aurora-1,
    .wf-aurora-2 {
      animation: none;
      transform: translate(1.5%, -1%) scale(1.02);
    }
  }
  @media (max-width: 767px) {
    .aurora {
      display: none;
    }
  }

  /* ---- Container (z-index: 2) ---- */
  .container {
    position: relative;
    z-index: 2;
  }

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
    margin-bottom: var(--space-10);
    max-width: 22ch;
  }

  /* ---- Horizontal spine (desktop >= 880px) ---- */
  .spine-wrap {
    position: relative;
    height: 32px;
    margin-bottom: var(--space-6);
    display: none;
  }
  @media (min-width: 880px) {
    .spine-wrap {
      display: block;
    }
  }
  .spine-line {
    position: absolute;
    top: 50%;
    left: 0;
    right: 0;
    height: 1px;
    background: var(--color-border);
    transform: translateY(-50%);
  }
  .spine-marker {
    position: absolute;
    top: 50%;
    transform: translate(-50%, -50%);
    width: 32px;
    height: 32px;
    border-radius: 50%;
    background: var(--color-surface-elevated);
    border: 1px solid var(--color-accent);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .spine-num {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--color-accent);
    font-variant-numeric: tabular-nums;
    line-height: 1;
  }

  /* ---- Step list (z-index: 2, above spine) ---- */
  ol {
    list-style: none;
    display: grid;
    gap: var(--space-6);
    grid-template-columns: 1fr;
  }
  @media (min-width: 880px) {
    ol {
      grid-template-columns: repeat(3, 1fr);
    }
  }

  /* ---- Step card ---- */
  li {
    background: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-left: 3px solid var(--color-accent);
    border-radius: var(--radius-xl);
    overflow: hidden;
    transition: border-color 200ms ease, transform 200ms ease;
    display: flex;
    flex-direction: column;
  }
  li:hover {
    border-color: color-mix(in srgb, var(--color-accent) 50%, var(--color-border));
    border-left-color: var(--color-accent);
    transform: translateY(-2px);
  }

  /* ---- Zone A: terminal ---- */
  .terminal-zone {
    background: #0d0d10;
    border-bottom: 1px solid var(--color-border);
    padding: var(--space-4) var(--space-5);
  }
  .terminal-pre {
    font-family: var(--font-mono);
    font-size: 12px;
    line-height: 1.55;
    white-space: pre;
    overflow: hidden;
    margin: 0;
    color: var(--color-text-secondary);
  }
  .t-prompt {
    color: var(--color-text-muted);
  }
  .t-cmd {
    color: var(--color-text-primary);
  }
  .t-output {
    color: var(--color-text-secondary);
  }
  .t-success {
    color: var(--color-success);
  }
  .t-accent {
    color: var(--color-accent);
  }

  /* ---- Zone B: card body ---- */
  .card-body {
    padding: var(--space-6) var(--space-8);
    flex: 1;
  }
  .step-n {
    display: inline-block;
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    font-variant-numeric: tabular-nums;
    color: var(--color-accent);
    background: var(--color-accent-subtle);
    padding: 4px 10px;
    border-radius: 999px;
    margin-bottom: var(--space-4);
  }
  h3 {
    font-size: var(--font-size-xl);
    margin-bottom: var(--space-3);
  }
  .card-body p {
    color: var(--color-text-secondary);
    line-height: 1.65;
    font-size: var(--font-size-sm);
  }
  .card-body p :global(code) {
    font-family: var(--font-mono);
    font-size: 0.85em;
    padding: 1px 6px;
    background: color-mix(in srgb, var(--color-accent) 8%, transparent);
    border: 1px solid var(--color-border);
    border-radius: 3px;
    color: var(--color-text-primary);
  }

  /* ---- iPad: vertical spine ---- */
  @media (min-width: 768px) and (max-width: 879px) {
    ol {
      position: relative;
      padding-left: var(--space-8);
    }
    ol::before {
      content: '';
      position: absolute;
      top: 0;
      bottom: 0;
      left: 0;
      width: 3px;
      background: var(--color-border);
      border-radius: 2px;
    }
  }
</style>
