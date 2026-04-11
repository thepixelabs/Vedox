<!--
  Breadcrumbs.svelte

  Path breadcrumbs above the editor content. Each segment is clickable
  and navigates to the parent folder (or project home). The separator is
  rendered as an italic Source Serif 4 slash `/` per design-system.md §5.11.

  Props:
    projectId: string
    projectName?: string
    docPath: string       — e.g. "guides/getting-started.md"
-->

<script lang="ts">
  import { goto } from '$app/navigation';

  interface Props {
    projectId: string;
    projectName?: string;
    docPath: string;
  }

  let { projectId, projectName = '', docPath }: Props = $props();

  interface Segment {
    label: string;
    href: string;
    isLast: boolean;
  }

  const segments = $derived.by<Segment[]>(() => {
    const segs: Segment[] = [];
    const displayProject = projectName || projectId;

    // Root (project)
    segs.push({
      label: displayProject,
      href: `/projects/${encodeURIComponent(projectId)}`,
      isLast: false
    });

    if (!docPath) {
      if (segs.length > 0) segs[segs.length - 1].isLast = true;
      return segs;
    }

    // Split doc path and build progressive hrefs
    const parts = docPath.split('/').filter((p) => p.length > 0);
    let accumulated = '';
    for (let i = 0; i < parts.length; i++) {
      accumulated = accumulated ? `${accumulated}/${parts[i]}` : parts[i];
      const isLastPart = i === parts.length - 1;
      // Strip .md extension on the last segment for display
      const label =
        isLastPart && parts[i].endsWith('.md')
          ? parts[i].slice(0, -3)
          : parts[i];
      segs.push({
        label,
        href: `/projects/${encodeURIComponent(projectId)}/docs/${accumulated}`,
        isLast: isLastPart
      });
    }

    return segs;
  });

  function handleClick(e: MouseEvent, href: string, isLast: boolean): void {
    if (isLast) {
      e.preventDefault();
      return;
    }
    e.preventDefault();
    goto(href);
  }
</script>

<nav class="breadcrumbs" aria-label="Document path">
  {#each segments as seg, i (seg.href + i)}
    {#if i > 0}
      <span class="breadcrumbs__sep" aria-hidden="true">/</span>
    {/if}
    {#if seg.isLast}
      <span class="breadcrumbs__segment breadcrumbs__segment--current" aria-current="page">
        {seg.label}
      </span>
    {:else}
      <a
        class="breadcrumbs__segment"
        href={seg.href}
        onclick={(e) => handleClick(e, seg.href, seg.isLast)}
      >
        {seg.label}
      </a>
    {/if}
  {/each}
</nav>

<style>
  .breadcrumbs {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px var(--space-6, 24px);
    font-family: var(--font-body, system-ui, sans-serif);
    font-size: 13px;
    line-height: 1.4;
    color: var(--text-3, rgba(255, 255, 255, 0.5));
    border-bottom: 1px solid var(--border-hairline, rgba(255, 255, 255, 0.06));
    background: var(--surface-1, var(--color-surface-base, #0f0f0f));
    flex-shrink: 0;
    min-height: 36px;
    overflow-x: auto;
    white-space: nowrap;
  }

  .breadcrumbs__segment {
    color: var(--text-3, rgba(255, 255, 255, 0.5));
    text-decoration: none;
    transition: color 120ms var(--ease-out, ease-out);
    padding: 2px 4px;
    border-radius: var(--radius-sm, 4px);
  }

  .breadcrumbs__segment:hover:not(.breadcrumbs__segment--current) {
    color: var(--text-1, rgba(255, 255, 255, 0.95));
    background: var(--surface-3, rgba(255, 255, 255, 0.04));
  }

  .breadcrumbs__segment--current {
    color: var(--text-1, rgba(255, 255, 255, 0.95));
    font-weight: 500;
  }

  .breadcrumbs__sep {
    font-family: var(--font-display, 'Source Serif 4 Variable', 'Source Serif Pro', Georgia, serif);
    font-style: italic;
    font-size: 13px;
    color: var(--text-3, rgba(255, 255, 255, 0.35));
    user-select: none;
    line-height: 1;
  }

  .breadcrumbs__segment:focus-visible {
    outline: 2px solid var(--accent-solid, #3b82f6);
    outline-offset: 2px;
  }
</style>
