<script lang="ts">
  import { onMount } from 'svelte';
  import { pitch } from '$lib/content';
  import { reveal } from '$lib/actions/reveal';

  let terminalEl: HTMLElement;
  let linesEl: HTMLElement;
  let hasPlayed = false;

  onMount(() => {
    const prefersReduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (prefersReduced || !linesEl) return;

    // Clear static lines for animation
    linesEl.innerHTML = '';

    const obs = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting && !hasPlayed) {
          hasPlayed = true;
          obs.disconnect();
          playTerminal();
        }
      },
      { threshold: 0.4 }
    );
    obs.observe(terminalEl);

    return () => obs.disconnect();
  });

  async function playTerminal() {
    const lines = pitch.terminal;
    for (let li = 0; li < lines.length; li++) {
      const line = lines[li];
      const span = document.createElement('span');
      span.className = line.kind;

      if (line.kind === 'prompt') {
        const arrow = document.createElement('span');
        arrow.className = 'arrow';
        arrow.textContent = '› ';
        span.appendChild(arrow);
        linesEl.appendChild(span);
        linesEl.appendChild(document.createTextNode('\n'));

        // Type character by character
        for (let ci = 0; ci < line.text.length; ci++) {
          span.appendChild(document.createTextNode(line.text[ci]));
          await sleep(32);
        }
      } else {
        // Output block — appear after 180ms pause
        await sleep(180);
        span.textContent = line.text;
        linesEl.appendChild(span);
        linesEl.appendChild(document.createTextNode('\n'));
      }

      // Pause between lines
      if (li < lines.length - 1) {
        await sleep(400);
      }
    }

    // Blinking cursor after completion
    const cursor = document.createElement('span');
    cursor.className = 'term-cursor';
    cursor.textContent = '▌';
    linesEl.appendChild(cursor);
  }

  function sleep(ms: number) {
    return new Promise<void>((r) => setTimeout(r, ms));
  }
</script>

<section id={pitch.id} class="pitch">
  <!-- Terminal hearth — atmospheric CRT warmth on the right, desktop only -->
  <div class="section-bg" aria-hidden="true">
    <img
      src="/vedox-logo-10-terminal-hearth-rembg.png"
      alt=""
      loading="lazy"
      decoding="async"
    />
  </div>
  <div class="container grid">
    <div class="copy">
      <p class="kicker" use:reveal>{pitch.kicker}</p>
      <h2 use:reveal={{ delay: 60 }}>{pitch.title}</h2>
      <p class="body" use:reveal={{ delay: 120 }}>{pitch.body}</p>
    </div>
    <div class="terminal" role="img" aria-label="Terminal running vedox dev" bind:this={terminalEl}>
      <div class="chrome">
        <span class="dot r"></span>
        <span class="dot y"></span>
        <span class="dot g"></span>
        <span class="tab">zsh</span>
      </div>
      <pre bind:this={linesEl}>
{#each pitch.terminal as line, i (i)}<span class={line.kind}>{#if line.kind === 'prompt'}<span class="arrow">› </span>{/if}{line.text}</span>
{/each}</pre>
    </div>
  </div>
</section>

<style>
  .pitch {
    padding-top: var(--mkt-section-pad);
    position: relative;
    overflow: hidden;
  }
  /* Terminal hearth — left side behind copy column, CRT screen faces inward */
  .section-bg {
    position: absolute;
    left: -6%;
    top: 0;
    bottom: 0;
    width: 44%;
    pointer-events: none;
    z-index: 0;
    display: flex;
    align-items: center;
    justify-content: flex-start;
    opacity: 0.09;
  }
  .section-bg img {
    width: 100%;
    height: auto;
    max-width: none;
    object-fit: contain;
    object-position: right center;
    mix-blend-mode: luminosity;
  }
  @media (min-width: 769px) and (max-width: 1024px) {
    .section-bg {
      opacity: 0.06;
    }
  }
  @media (max-width: 768px) {
    .section-bg {
      display: none;
    }
  }
  /* Subtle engineering grid */
  .pitch::before {
    content: '';
    position: absolute;
    inset: 0;
    background-image: repeating-linear-gradient(
      to bottom,
      transparent,
      transparent 47px,
      color-mix(in srgb, var(--color-border) 60%, transparent) 47px,
      color-mix(in srgb, var(--color-border) 60%, transparent) 48px
    );
    opacity: 0.015;
    pointer-events: none;
    z-index: 0;
  }
  .grid {
    position: relative;
    z-index: 1;
    display: grid;
    gap: var(--space-12);
    grid-template-columns: 1fr;
    align-items: center;
  }
  @media (min-width: 880px) {
    .grid {
      grid-template-columns: 1fr 1.1fr;
    }
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
    margin-bottom: var(--space-5);
    max-width: 18ch;
  }
  .body {
    font-size: var(--font-size-lg);
    max-width: 48ch;
  }
  /* Left accent rail on copy block (desktop only) */
  @media (min-width: 880px) {
    .copy {
      border-left: 2px solid var(--color-accent);
      padding-left: var(--space-8);
    }
  }
  .terminal {
    border-radius: var(--radius-xl);
    background: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    box-shadow:
      0 12px 40px rgba(0, 0, 0, 0.12),
      -20px 0 60px -20px color-mix(in srgb, var(--color-accent) 10%, transparent);
    overflow: hidden;
  }
  .chrome {
    display: flex;
    gap: var(--space-2);
    padding: 10px var(--space-4);
    background: var(--color-surface-overlay);
    border-bottom: 1px solid var(--color-border);
    align-items: center;
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
  pre {
    padding: var(--space-6);
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    line-height: 1.8;
    color: var(--color-text-primary);
    white-space: pre-wrap;
  }
  :global(.pitch pre .prompt) {
    color: var(--color-text-primary);
  }
  :global(.pitch pre .arrow) {
    color: var(--color-accent);
  }
  :global(.pitch pre .output) {
    color: var(--color-text-muted);
  }
  :global(.pitch pre .term-cursor) {
    color: var(--color-accent);
    animation: term-blink 1s step-end infinite;
  }
  @keyframes term-blink {
    50% { opacity: 0; }
  }
</style>
