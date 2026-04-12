<script lang="ts">
  import { faq } from '$lib/content';
  import { trackFaqExpand } from '$lib/analytics';

  let openIndex = $state<number | null>(null);
  let answerEls: HTMLElement[] = $state([]);

  function toggle(i: number) {
    if (openIndex === i) {
      openIndex = null;
    } else {
      openIndex = i;
      trackFaqExpand(faq.items[i].q);
    }
  }

  function animateHeight(node: HTMLElement, isOpen: boolean) {
    // Cancel any in-flight transition before starting a new one
    node.style.transition = 'none';

    if (isOpen) {
      node.style.overflow = 'hidden';
      node.style.height = '0px';
      // Double-rAF: first frame locks in height:0, second kicks off the transition
      requestAnimationFrame(() => {
        requestAnimationFrame(() => {
          // scrollHeight may be 0 mid-close; clamp to a sane minimum
          const target = node.scrollHeight || 200;
          node.style.transition = 'height 300ms cubic-bezier(0.22, 1, 0.36, 1)';
          node.style.height = target + 'px';
          node.addEventListener(
            'transitionend',
            () => {
              node.style.height = 'auto';
              node.style.overflow = '';
              node.style.transition = '';
            },
            { once: true }
          );
        });
      });
    } else {
      // Capture current rendered height before collapsing
      const current = node.getBoundingClientRect().height || node.scrollHeight;
      node.style.height = current + 'px';
      node.style.overflow = 'hidden';
      requestAnimationFrame(() => {
        requestAnimationFrame(() => {
          node.style.transition = 'height 300ms cubic-bezier(0.22, 1, 0.36, 1)';
          node.style.height = '0px';
          node.addEventListener(
            'transitionend',
            () => {
              node.style.transition = '';
            },
            { once: true }
          );
        });
      });
    }
  }

  function answerAction(node: HTMLElement, params: { open: boolean; i: number }) {
    answerEls[params.i] = node;
    if (!params.open) {
      node.style.height = '0px';
      node.style.overflow = 'hidden';
    }
    return {
      update(newParams: { open: boolean; i: number }) {
        animateHeight(node, newParams.open);
      },
      destroy() {}
    };
  }
</script>

<section id={faq.id} class="faq">
  <!-- Cartographers repo — bottom-right atmospheric texture -->
  <img
    class="faq-bg-img"
    src="/vedox-logo-06-cartographers-repo-final.png"
    alt=""
    aria-hidden="true"
    loading="lazy"
    decoding="async"
  />
  <!-- Aurora ellipse — upper-left complementary light -->
  <svg class="faq-aurora" aria-hidden="true">
    <defs>
      <filter id="faq-aurora-blur"><feGaussianBlur stdDeviation="55"/></filter>
    </defs>
    <ellipse class="faq-aurora-e1" cx="15%" cy="20%" rx="35%" ry="25%" fill="#818cf8" opacity="0.11" filter="url(#faq-aurora-blur)"/>
  </svg>
  <div class="container">
    <p class="kicker">{faq.kicker}</p>
    <h2>{faq.title}</h2>
    <div class="list">
      {#each faq.items as item, i (item.q)}
        <div class="item" class:open={openIndex === i}>
          <button
            type="button"
            class="question"
            aria-expanded={openIndex === i}
            aria-controls="faq-answer-{i}"
            onclick={() => toggle(i)}
          >
            <span>{item.q}</span>
            <svg viewBox="0 0 24 24" width="18" height="18" aria-hidden="true">
              <path
                d="M6 9l6 6l6-6"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
            </svg>
          </button>
          <div
            id="faq-answer-{i}"
            class="answer-wrap"
            use:answerAction={{ open: openIndex === i, i }}
          >
            <p class="answer">{item.a}</p>
          </div>
        </div>
      {/each}
    </div>
  </div>
</section>

<style>
  .faq {
    position: relative;
    overflow: hidden;
  }
  /* Cartographers repo — bottom-right corner, object facing inward */
  .faq-bg-img {
    position: absolute;
    right: -8%;
    bottom: -10%;
    width: 55%;
    height: auto;
    pointer-events: none;
    z-index: 0;
    opacity: 0.07;
    object-fit: contain;
    object-position: right bottom;
    mix-blend-mode: luminosity;
  }
  @media (min-width: 768px) and (max-width: 1023px) {
    .faq-bg-img {
      opacity: 0.04;
    }
  }
  @media (max-width: 767px) {
    .faq-bg-img {
      display: none;
    }
  }
  /* Aurora — upper-left */
  .faq-aurora {
    position: absolute;
    inset: -10% -5%;
    width: 110%;
    height: 120%;
    pointer-events: none;
    z-index: 0;
  }
  .faq-aurora-e1 {
    animation: faq-aurora-drift 22s ease-in-out infinite alternate;
    animation-delay: -15s;
  }
  @keyframes faq-aurora-drift {
    from { transform: translate(0, 0) scale(1); }
    to   { transform: translate(3%, 2%) scale(1.05); }
  }
  @media (prefers-reduced-motion: reduce) {
    .faq-aurora-e1 {
      animation: none;
      transform: translate(1.5%, 1%) scale(1.025);
    }
  }
  .container {
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
  }
  .list {
    max-width: 780px;
  }
  .item {
    border-top: 1px solid var(--color-border);
    border-left: 3px solid transparent;
    padding-left: var(--space-5);
    transition: border-left-color 200ms ease;
  }
  .item.open {
    border-left-color: var(--color-accent);
  }
  .item:last-child {
    border-bottom: 1px solid var(--color-border);
  }
  .question {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-4);
    width: 100%;
    padding: var(--space-5) 0;
    background: none;
    border: none;
    cursor: pointer;
    text-align: left;
    font-weight: 600;
    color: var(--color-text-primary);
    font-size: var(--font-size-lg);
    font-family: var(--font-sans);
    scroll-margin-top: var(--space-8);
  }
  .question svg {
    color: var(--color-text-muted);
    flex-shrink: 0;
    transition: transform 200ms ease;
  }
  .item.open .question svg {
    transform: rotate(180deg);
  }
  .answer-wrap {
    overflow: hidden;
  }
  .answer {
    padding-bottom: var(--space-5);
    color: var(--color-text-secondary);
    max-width: 68ch;
  }
</style>
