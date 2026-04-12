<script lang="ts">
  import { socialProof } from '$lib/content';
  // Double the array for seamless loop
  const items = [...socialProof.commits, ...socialProof.commits];
</script>

<section class="social-proof" aria-label={socialProof.kicker}>
  <div class="container header-row">
    <p class="kicker">{socialProof.kicker}</p>
  </div>
  <div class="ticker-wrap" role="marquee" aria-live="off">
    <div class="ticker">
      {#each items as commit, i (`${commit.hash}-${i}`)}
        <span class="commit">
          <span class="hash">{commit.hash}</span>
          <span class="msg">{commit.message}</span>
        </span>
      {/each}
    </div>
  </div>
</section>

<style>
  .social-proof {
    padding-top: var(--space-12);
    padding-bottom: var(--space-12);
    overflow: hidden;
    border-top: 1px solid var(--color-border);
  }
  .header-row {
    margin-bottom: var(--space-6);
  }
  .kicker {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    text-transform: uppercase;
    letter-spacing: 0.14em;
    color: var(--color-text-muted);
  }
  .ticker-wrap {
    overflow: hidden;
    mask-image: linear-gradient(to right, transparent, black 10%, black 90%, transparent);
    -webkit-mask-image: linear-gradient(to right, transparent, black 10%, black 90%, transparent);
  }
  .ticker-wrap:hover .ticker,
  .ticker-wrap:focus-within .ticker {
    animation-play-state: paused;
  }
  .ticker {
    display: flex;
    gap: var(--space-8);
    width: max-content;
    animation: ticker-scroll 32s linear infinite;
  }
  @keyframes ticker-scroll {
    from { transform: translateX(0); }
    to { transform: translateX(-50%); }
  }
  @media (prefers-reduced-motion: reduce) {
    .ticker {
      animation: none;
    }
    /* Stack commits vertically when motion is off so they're still visible */
    .ticker-wrap {
      mask-image: none;
      -webkit-mask-image: none;
    }
    .ticker {
      flex-wrap: wrap;
      width: auto;
    }
  }
  .commit {
    display: inline-flex;
    align-items: center;
    gap: var(--space-3);
    white-space: nowrap;
    padding: var(--space-2) var(--space-4);
    background: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
  }
  .hash {
    color: var(--color-accent);
    font-size: var(--font-size-xs);
    opacity: 0.8;
  }
  .msg {
    color: var(--color-text-secondary);
  }
</style>
