<script lang="ts">
  import { pillars } from '$lib/content';
  import { reveal } from '$lib/actions/reveal';

  const icons: Record<string, string> = {
    git: 'M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22',
    disk: 'M4 6a8 3 0 1 0 16 0a8 3 0 1 0 -16 0 M4 6v12a8 3 0 0 0 16 0V6 M4 12a8 3 0 0 0 16 0',
    lock: 'M5 11h14a1 1 0 0 1 1 1v8a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1v-8a1 1 0 0 1 1-1z M7 11V7a5 5 0 0 1 10 0v4',
  };

  const delays = [0, 100, 200];
</script>

<section id={pillars.id} class="pillars">
  <div class="container">
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
            <svg viewBox="0 0 24 24" width="22" height="22">
              <path
                d={icons[p.icon]}
                fill="none"
                stroke="currentColor"
                stroke-width="1.8"
                stroke-linecap="round"
                stroke-linejoin="round"
              />
            </svg>
          </div>
          <h3>{p.title}</h3>
          <p>{p.body}</p>
        </article>
      {/each}
    </div>
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
  .card:hover .icon svg {
    transform: rotate(8deg);
  }
  .icon {
    width: 42px;
    height: 42px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    background: color-mix(in srgb, var(--card-accent) 12%, transparent);
    color: var(--card-accent);
    border-radius: var(--radius-md);
    margin-bottom: var(--space-5);
  }
  .icon svg {
    transition: transform 200ms var(--ease-standard, ease);
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
