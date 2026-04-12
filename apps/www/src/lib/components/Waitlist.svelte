<script lang="ts">
  import { onMount } from 'svelte';
  import { waitlist } from '$lib/content';
  import { track } from '$lib/analytics';
  import { env } from '$env/dynamic/public';

  const PUBLIC_WAITLIST_ENDPOINT = env.PUBLIC_WAITLIST_ENDPOINT;
  import InstallCommand from './InstallCommand.svelte';

  let email = $state('');
  let status = $state<'idle' | 'loading' | 'ok' | 'err' | 'disabled'>(
    PUBLIC_WAITLIST_ENDPOINT ? 'idle' : 'disabled'
  );
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

  async function onSubmit(e: SubmitEvent) {
    e.preventDefault();
    if (!PUBLIC_WAITLIST_ENDPOINT) {
      status = 'disabled';
      return;
    }
    if (!email || !email.includes('@')) {
      status = 'err';
      return;
    }
    status = 'loading';
    try {
      const res = await fetch(PUBLIC_WAITLIST_ENDPOINT, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({ email, source: 'vedox.dev' }),
      });
      if (!res.ok) throw new Error(String(res.status));
      status = 'ok';
      track('Waitlist Submit');
      email = '';
    } catch {
      status = 'err';
    }
  }
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

    <form onsubmit={onSubmit} novalidate>
      <label class="sr-only" for="email">Email</label>
      <input
        id="email"
        type="email"
        autocomplete="email"
        placeholder={waitlist.placeholder}
        bind:value={email}
        required
        disabled={status === 'disabled' || status === 'loading'}
      />
      <button type="submit" disabled={status === 'disabled' || status === 'loading'}>
        {status === 'loading' ? waitlist.sending : waitlist.button}
      </button>
    </form>

    <p class="msg" class:ok={status === 'ok'} class:err={status === 'err'} aria-live="polite">
      {#if status === 'ok'}
        {waitlist.success}
      {:else if status === 'err'}
        {waitlist.failure}
      {:else if status === 'disabled'}
        {waitlist.disabled}
      {:else}
        &nbsp;
      {/if}
    </p>
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
        900px 500px at 50% 100%,
        color-mix(in srgb, var(--color-accent) 14%, transparent),
        transparent 65%
      ),
      radial-gradient(
        600px 400px at 20% 60%,
        color-mix(in srgb, var(--color-accent) 8%, transparent),
        transparent 60%
      ),
      radial-gradient(
        500px 300px at 80% 40%,
        color-mix(in srgb, var(--color-accent) 6%, transparent),
        transparent 60%
      );
  }
  /* Shimmer lines */
  .shimmer-line {
    position: absolute;
    top: -100%;
    width: 1px;
    height: 100%;
    background: linear-gradient(
      to bottom,
      transparent,
      var(--color-accent),
      transparent
    );
    opacity: 0.25;
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
    10% { opacity: 0.25; }
    90% { opacity: 0.25; }
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
    margin-bottom: var(--space-8);
  }
  form {
    display: flex;
    gap: var(--space-3);
    max-width: 480px;
    margin: 0 auto;
    flex-wrap: wrap;
  }
  input {
    flex: 1 1 220px;
    padding: 14px 16px;
    border-radius: var(--radius-lg);
    border: 1px solid var(--color-border);
    background: var(--color-surface-elevated);
    color: var(--color-text-primary);
    font-family: var(--font-sans);
    font-size: var(--font-size-base);
  }
  input:focus-visible {
    border-color: var(--color-accent);
  }
  button {
    padding: 14px 20px;
    border-radius: var(--radius-lg);
    border: 1px solid var(--color-accent);
    background: var(--color-accent);
    color: var(--color-text-inverse);
    font-weight: 600;
    font-size: var(--font-size-base);
    cursor: pointer;
    box-shadow: 0 0 20px color-mix(in srgb, var(--color-accent) 25%, transparent);
  }
  button:hover:not(:disabled) {
    background: var(--color-accent-hover);
    border-color: var(--color-accent-hover);
    box-shadow: 0 0 30px color-mix(in srgb, var(--color-accent) 35%, transparent);
  }
  button:disabled,
  input:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }
  .msg {
    margin-top: var(--space-4);
    font-size: var(--font-size-sm);
    color: var(--color-text-muted);
    min-height: 1.5em;
  }
  .msg.ok {
    color: var(--color-success);
  }
  .msg.err {
    color: var(--color-error);
  }
</style>
