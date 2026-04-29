<script lang="ts">
  import { onMount } from 'svelte';
  import { waitlist } from '$lib/content';
  import InstallCommand from './InstallCommand.svelte';

  let h2El: HTMLElement;

  onMount(() => {
    const prefersReduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    if (!prefersReduced && h2El) {
      const words = h2El.textContent?.split(' ') ?? [];
      h2El.innerHTML = words
        .map((w, i) => `<span class="wlw" style="--i:${i}">${w}</span>`)
        .join(' ');
      // Double-rAF ensures opacity:0 is painted before the observer wires up
      requestAnimationFrame(() => {
        requestAnimationFrame(() => {
          const obs = new IntersectionObserver(
            ([entry]) => {
              if (entry.isIntersecting) {
                h2El.querySelectorAll('.wlw').forEach((el) => el.classList.add('visible'));
                obs.disconnect();
              }
            },
            { threshold: 0.3 }
          );
          obs.observe(h2El);
        });
      });
    }
  });
</script>

<section id={waitlist.id} class="waitlist">
  <img
    class="waitlist-bg"
    src="/images/waitlist-bg.webp"
    alt=""
    aria-hidden="true"
    loading="lazy"
    width="1400"
    height="700"
  />
  <div class="shimmer-line shimmer-1" aria-hidden="true"></div>
  <div class="shimmer-line shimmer-2" aria-hidden="true"></div>
  <div class="shimmer-line shimmer-3" aria-hidden="true"></div>
  <div class="waitlist-noise" aria-hidden="true"></div>
  <div class="container inner">
    <p class="kicker">{waitlist.kicker}</p>
    <h2 bind:this={h2El}>{waitlist.title}</h2>
    <p class="body">{waitlist.body}</p>

    <div class="install">
      <InstallCommand location="waitlist" />
    </div>
  </div>
</section>

<style>
  .waitlist-bg {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
    object-position: center;
    opacity: 0.08;
    pointer-events: none;
    z-index: 0;
    /* Darken in dark mode for contrast safety */
  }
  :global([data-theme='dark']) .waitlist-bg {
    opacity: 0.06;
  }
  .waitlist {
    text-align: center;
    position: relative;
    overflow: hidden;
    background:
      radial-gradient(
        1100px 600px at 50% 100%,
        color-mix(in srgb, var(--color-accent) 18%, transparent),
        transparent 65%
      ),
      radial-gradient(
        600px 400px at 20% 60%,
        color-mix(in srgb, var(--color-accent) 11%, transparent),
        transparent 60%
      ),
      radial-gradient(
        500px 300px at 80% 40%,
        color-mix(in srgb, var(--color-accent) 8%, transparent),
        transparent 60%
      );
  }
  /* Shimmer lines — wider, slightly blurred for volumetric feel */
  .shimmer-line {
    position: absolute;
    top: -100%;
    width: 1.5px;
    height: 100%;
    background: linear-gradient(
      to bottom,
      transparent,
      var(--color-accent),
      transparent
    );
    opacity: 0.38;
    filter: blur(0.5px);
    pointer-events: none;
  }
  .shimmer-1 {
    left: 15%;
    animation: shimmer-fall 6s ease-in-out infinite;
  }
  .shimmer-2 {
    left: 50%;
    animation: shimmer-fall 8s ease-in-out infinite 1.5s;
  }
  .shimmer-3 {
    left: 85%;
    animation: shimmer-fall 10s ease-in-out infinite 3s;
  }
  @keyframes shimmer-fall {
    0% { transform: translateY(0); opacity: 0; }
    10% { opacity: 0.38; }
    90% { opacity: 0.38; }
    100% { transform: translateY(200%); opacity: 0; }
  }
  @media (prefers-reduced-motion: reduce) {
    .shimmer-line {
      animation: none;
      display: none;
    }
  }
  .waitlist-noise {
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
  :global([data-theme='dark']) .waitlist-noise {
    opacity: 0.07;
  }
  .inner {
    position: relative;
    z-index: 1;
    max-width: 680px;
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
    margin-bottom: var(--space-4);
    overflow: hidden;
  }
  :global(.wlw) {
    display: inline-block;
    opacity: 0;
    transform: translateY(12px);
    transition:
      opacity var(--duration-entrance, 600ms) var(--ease-expo-out, ease) calc(var(--i) * 70ms),
      transform var(--duration-entrance, 600ms) var(--ease-expo-out, ease) calc(var(--i) * 70ms);
  }
  :global(.wlw.visible) {
    opacity: 1;
    transform: translateY(0);
  }
  .body {
    margin-bottom: var(--space-8);
  }
  .install {
    display: flex;
    justify-content: center;
  }
</style>
