<script lang="ts">
  import { pillars } from '$lib/content';
  import { reveal } from '$lib/actions/reveal';

  const delays = [0, 100, 200];

  // SVG icon paths keyed by icon name
  const icons: Record<string, string> = {
    git: `<svg viewBox="0 0 24 24" width="32" height="32" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <circle cx="18" cy="18" r="3"/>
      <circle cx="6" cy="6" r="3"/>
      <path d="M13 6h3a2 2 0 0 1 2 2v7"/>
      <line x1="6" y1="9" x2="6" y2="21"/>
    </svg>`,
    disk: `<svg viewBox="0 0 24 24" width="32" height="32" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <ellipse cx="12" cy="5" rx="9" ry="3"/>
      <path d="M3 5v14c0 1.66 4.03 3 9 3s9-1.34 9-3V5"/>
      <path d="M3 12c0 1.66 4.03 3 9 3s9-1.34 9-3"/>
    </svg>`,
    lock: `<svg viewBox="0 0 24 24" width="32" height="32" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>
      <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
    </svg>`,
  };
</script>

<section id={pillars.id} class="pillars">
  <!-- Carved root graph — atmospheric background texture behind all 3 cards -->
  <div class="section-bg" aria-hidden="true">
    <img src="/vedox-logo-02-carved-root-graph-rembg.png" alt="" />
  </div>
  <div class="content container">
    <p class="kicker" use:reveal>{pillars.kicker}</p>
    <h2 use:reveal={{ delay: 60 }}>{pillars.title}</h2>
    <div class="grid">
      {#each pillars.items as p, i (p.title)}
        <article
          class="card"
          style="--card-accent: {p.icon === 'git' ? 'var(--color-accent)' : p.icon === 'disk' ? 'var(--color-warning)' : 'var(--color-success)'}"
          use:reveal={{ delay: delays[i] ?? 0 }}
        >
          <div class="icon" aria-hidden="true">
            {@html icons[p.icon] ?? ''}
          </div>
          <h3>{p.title}</h3>
          <p>{p.body}</p>
        </article>
      {/each}
    </div>
  </div>
</section>

<style>
  .pillars {
    position: relative;
    overflow: hidden;
    background:
      radial-gradient(
        800px 350px at 50% 120%,
        color-mix(in srgb, var(--color-accent) 14%, transparent),
        transparent 60%
      );
    animation: pillars-radial-breathe 14s ease-in-out infinite;
    animation-delay: -5s;
  }
  @keyframes pillars-radial-breathe {
    0%   { background-size: 100% 100%; }
    50%  { background-size: 103% 103%; }
    100% { background-size: 100% 100%; }
  }
  @media (prefers-reduced-motion: reduce) {
    .pillars {
      animation: none;
      /* Freeze radial at mid-breath size */
      background-size: 101.5% 101.5%;
    }
  }
  .section-bg {
    position: absolute;
    inset: 0;
    pointer-events: none;
    z-index: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    opacity: 0.09;
  }
  .section-bg img {
    width: 70%;
    height: auto;
    max-width: none;
    object-fit: contain;
    mix-blend-mode: luminosity;
  }
  @media (max-width: 768px) {
    .section-bg {
      display: none;
    }
  }
  .content {
    position: relative;
    z-index: 1;
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
    max-width: 20ch;
  }
  .grid {
    display: grid;
    gap: var(--space-6);
    grid-template-columns: 1fr;
  }
  @media (min-width: 760px) {
    .grid {
      grid-template-columns: repeat(3, 1fr);
    }
    /* Middle card offset */
    .grid > article:nth-child(2) {
      margin-top: var(--space-8);
    }
  }
  .card {
    padding: var(--space-8);
    background: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-top: 3px solid var(--card-accent, var(--color-accent));
    border-radius: var(--radius-xl);
    transition:
      border-color 200ms var(--ease-standard, ease),
      transform 200ms var(--ease-standard, ease),
      box-shadow 200ms var(--ease-standard, ease);
  }
  .card:hover {
    border-color: var(--color-border-strong);
    border-top-color: var(--card-accent);
    transform: translateY(-4px);
    box-shadow: 0 8px 32px color-mix(in srgb, var(--card-accent) 20%, transparent);
  }
  .icon {
    display: block;
    margin-bottom: var(--space-5);
    line-height: 0;
    color: var(--card-accent, var(--color-accent));
  }
  h3 {
    font-size: var(--font-size-xl);
    margin-bottom: var(--space-3);
  }
  .card p {
    font-size: var(--font-size-base);
    color: var(--color-text-secondary);
  }
</style>
