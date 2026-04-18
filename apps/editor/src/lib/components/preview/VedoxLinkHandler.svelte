<!--
  VedoxLinkHandler.svelte — wrapper that detects vedox:// links in rendered
  markdown HTML and attaches hover/click handlers to show CodePreviewCard.

  Usage:
    <VedoxLinkHandler>
      {@html renderedMarkdown}
    </VedoxLinkHandler>

  Detection strategy:
    After the slot content mounts (and on any re-render), we query the wrapper
    div for all <a> elements whose href starts with "vedox://".  We attach
    pointer-based (mouseenter/click) and keyboard (focus/Enter) handlers to
    each so that the preview card surfaces via both input modes.

  A single CodePreviewCard is rendered via Svelte portal (mounted on body) for
  the currently active link.  The portal pattern avoids z-index clipping from
  ancestor overflow:hidden containers.

  Bare vedox:// text (not wrapped in <a>):
    Some markdown renderers leave bare URLs as text nodes — this is handled in a
    second pass that replaces matching text nodes with <a href="vedox://...">
    elements, preserving the renderer's surrounding markup.

  Keyboard:
    Tab into a vedox:// link  → show card
    Escape                     → dismiss card (handled inside CodePreviewCard)
    Enter / Space on link      → show card (or follow href if the renderer set
                                 one — we intercept the click regardless)
-->

<script lang="ts">
  import { onMount, onDestroy, mount as svelteMount, unmount as svelteUnmount, tick } from 'svelte';
  import type { Snippet } from 'svelte';
  import CodePreviewCard from './CodePreviewCard.svelte';

  // ---------------------------------------------------------------------------
  // Props
  // ---------------------------------------------------------------------------

  interface Props {
    children?: Snippet;
  }

  const { children }: Props = $props();

  // ---------------------------------------------------------------------------
  // State
  // ---------------------------------------------------------------------------

  /** Wrapper element that contains the rendered markdown. */
  let wrapperEl: HTMLDivElement | null = $state(null);

  /** Active vedox:// URL being previewed, or null. */
  let activeUrl: string | null = $state(null);

  /** The trigger <a> element that opened the current card. */
  let activeTrigger: HTMLElement | null = $state(null);

  /** Monotonically incrementing id for ARIA linkage. */
  let cardCounter = 0;
  let activeCardId: string = $state('');

  // Portal host for the card — mounted on document.body.
  let portalHost: HTMLDivElement | null = null;
  let cardInstance: Record<string, unknown> | null = null;

  // Mouse-leave dismissal timer (shared between trigger and card).
  let dismissTimer: ReturnType<typeof setTimeout> | null = null;

  // ---------------------------------------------------------------------------
  // Bare-URL rewriting
  // ---------------------------------------------------------------------------

  /**
   * Walk text nodes inside `root` and replace bare "vedox://..." text with
   * an <a href="vedox://..."> so our event delegation picks them up.
   *
   * We only rewrite text nodes that are NOT already inside an <a>.
   */
  function rewriteBareUrls(root: HTMLElement): void {
    const VEDOX_RE = /vedox:\/\/[^\s<>"')]+/g;
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);

    const replacements: { node: Text; replacement: DocumentFragment }[] = [];

    let node: Text | null;
    while ((node = walker.nextNode() as Text | null)) {
      // Skip text nodes already inside <a>.
      if ((node.parentElement as HTMLElement | null)?.closest('a')) continue;

      const text = node.nodeValue ?? '';
      if (!VEDOX_RE.test(text)) continue;
      VEDOX_RE.lastIndex = 0; // reset after .test()

      const frag = document.createDocumentFragment();
      let lastIdx = 0;
      let match: RegExpExecArray | null;
      while ((match = VEDOX_RE.exec(text)) !== null) {
        if (match.index > lastIdx) {
          frag.appendChild(document.createTextNode(text.slice(lastIdx, match.index)));
        }
        const url = match[0];
        const a = document.createElement('a');
        a.href = url;
        a.setAttribute('data-vedox-link', '');
        a.textContent = url;
        a.style.cursor = 'pointer';
        frag.appendChild(a);
        lastIdx = match.index + url.length;
      }
      if (lastIdx < text.length) {
        frag.appendChild(document.createTextNode(text.slice(lastIdx)));
      }

      replacements.push({ node, replacement: frag });
    }

    // Apply replacements in reverse (DOM tree mutation during traversal is unsafe).
    for (const { node: n, replacement } of replacements.reverse()) {
      n.parentNode?.replaceChild(replacement, n);
    }
  }

  // ---------------------------------------------------------------------------
  // Event delegation
  // ---------------------------------------------------------------------------

  /**
   * Find the nearest <a href="vedox://…"> ancestor of an event target.
   */
  function findVedoxAnchor(target: EventTarget | null): HTMLAnchorElement | null {
    if (!(target instanceof Element)) return null;
    const a = target.closest<HTMLAnchorElement>('a[href^="vedox://"]');
    return a ?? null;
  }

  function handleMouseEnter(e: MouseEvent): void {
    const a = findVedoxAnchor(e.target);
    if (!a) return;
    cancelDismiss();
    openCard(a);
  }

  function handleMouseLeave(e: MouseEvent): void {
    const a = findVedoxAnchor(e.target);
    if (!a || a !== activeTrigger) return;
    scheduleDismiss();
  }

  function handleClick(e: MouseEvent): void {
    const a = findVedoxAnchor(e.target);
    if (!a) return;
    e.preventDefault();
    cancelDismiss();
    if (activeUrl === a.href && activeTrigger === a) {
      // Toggle: clicking an already-open card's link closes it.
      dismissCard();
    } else {
      openCard(a);
    }
  }

  function handleFocus(e: FocusEvent): void {
    const a = findVedoxAnchor(e.target);
    if (!a) return;
    cancelDismiss();
    openCard(a);
  }

  function handleBlur(e: FocusEvent): void {
    const a = findVedoxAnchor(e.target);
    if (!a || a !== activeTrigger) return;
    // Blur to outside the card: schedule dismiss.
    // The card's own onmouseenter / focus events will cancel if needed.
    scheduleDismiss();
  }

  // ---------------------------------------------------------------------------
  // Card lifecycle
  // ---------------------------------------------------------------------------

  let cardIdCounter = 0;

  function openCard(trigger: HTMLAnchorElement): void {
    const url = trigger.href;
    if (activeUrl === url && activeTrigger === trigger) return; // already open

    // Close any existing card first.
    destroyCardPortal();

    cardIdCounter += 1;
    activeCardId = `vedox-preview-${cardIdCounter}`;
    activeUrl = url;
    activeTrigger = trigger;

    // Aria-describedby linkage: tell AT users the tooltip describes this link.
    trigger.setAttribute('aria-describedby', activeCardId);
    trigger.setAttribute('aria-expanded', 'true');

    mountCardPortal(url, trigger, activeCardId);
  }

  function dismissCard(): void {
    if (activeTrigger) {
      activeTrigger.removeAttribute('aria-describedby');
      activeTrigger.removeAttribute('aria-expanded');
    }
    destroyCardPortal();
    activeUrl = null;
    activeTrigger = null;
    activeCardId = '';
  }

  // ---------------------------------------------------------------------------
  // Dismiss timer (shared with CodePreviewCard via scheduleHide / cancelHide)
  // ---------------------------------------------------------------------------

  function scheduleDismiss(): void {
    if (dismissTimer !== null) return;
    dismissTimer = setTimeout(() => {
      dismissCard();
    }, 300);
  }

  function cancelDismiss(): void {
    if (dismissTimer !== null) {
      clearTimeout(dismissTimer);
      dismissTimer = null;
    }
  }

  // ---------------------------------------------------------------------------
  // Portal mount / unmount
  // ---------------------------------------------------------------------------

  function mountCardPortal(url: string, trigger: HTMLElement, cardId: string): void {
    if (portalHost) destroyCardPortal();

    portalHost = document.createElement('div');
    portalHost.setAttribute('data-vedox-portal', '');
    document.body.appendChild(portalHost);

    cardInstance = svelteMount(CodePreviewCard, {
      target: portalHost,
      props: {
        vedoxUrl: url,
        triggerEl: trigger,
        cardId,
        onDismiss: dismissCard,
      },
    });
  }

  function destroyCardPortal(): void {
    if (cardInstance) {
      void svelteUnmount(cardInstance);
      cardInstance = null;
    }
    if (portalHost) {
      portalHost.remove();
      portalHost = null;
    }
  }

  // ---------------------------------------------------------------------------
  // Attach / detach handlers
  // ---------------------------------------------------------------------------

  function attachHandlers(root: HTMLElement): void {
    rewriteBareUrls(root);
    // Style all vedox:// anchors consistently.
    root.querySelectorAll<HTMLAnchorElement>('a[href^="vedox://"]').forEach((a) => {
      a.setAttribute('data-vedox-link', '');
    });
    root.addEventListener('mouseenter', handleMouseEnter, true);
    root.addEventListener('mouseleave', handleMouseLeave, true);
    root.addEventListener('click', handleClick, true);
    root.addEventListener('focus', handleFocus, true);
    root.addEventListener('blur', handleBlur, true);
  }

  function detachHandlers(root: HTMLElement): void {
    root.removeEventListener('mouseenter', handleMouseEnter, true);
    root.removeEventListener('mouseleave', handleMouseLeave, true);
    root.removeEventListener('click', handleClick, true);
    root.removeEventListener('focus', handleFocus, true);
    root.removeEventListener('blur', handleBlur, true);
  }

  // ---------------------------------------------------------------------------
  // Lifecycle
  // ---------------------------------------------------------------------------

  onMount(() => {
    if (wrapperEl) attachHandlers(wrapperEl);
  });

  onDestroy(() => {
    if (wrapperEl) detachHandlers(wrapperEl);
    cancelDismiss();
    destroyCardPortal();
  });

  /**
   * When the slot content re-renders (e.g. markdown content changes), re-scan
   * for new vedox:// links. We use a MutationObserver so we don't have to
   * re-run on every keystroke — only when the DOM actually changes.
   */
  let observer: MutationObserver | null = null;

  $effect(() => {
    if (!wrapperEl) return;

    // Initial pass.
    rewriteBareUrls(wrapperEl);

    if (observer) observer.disconnect();

    observer = new MutationObserver(() => {
      if (wrapperEl) rewriteBareUrls(wrapperEl);
    });

    observer.observe(wrapperEl, { childList: true, subtree: true });

    return () => {
      observer?.disconnect();
      observer = null;
    };
  });
</script>

<!--
  Wrapper div. Scoped styles target [data-vedox-link] to add the visual
  affordance on vedox:// anchors without interfering with regular links.
-->
<div bind:this={wrapperEl} class="vlh">
  {#if children}
    {@render children()}
  {/if}
</div>

<style>
  .vlh {
    /* Pass-through: no visual impact on the container itself. */
    display: contents;
  }

  /* Visual affordance for vedox:// links inside rendered markdown. */
  .vlh :global(a[data-vedox-link]) {
    color: var(--accent-text, var(--color-accent));
    text-decoration: underline;
    text-decoration-style: dashed;
    text-underline-offset: 3px;
    cursor: pointer;
    border-radius: var(--radius-sm);
    outline-offset: 2px;
    transition: color var(--duration-fast) var(--ease-out);
  }

  .vlh :global(a[data-vedox-link]:hover),
  .vlh :global(a[data-vedox-link][aria-expanded="true"]) {
    color: var(--accent-solid-hover, var(--color-accent-hover));
    text-decoration-style: solid;
  }

  .vlh :global(a[data-vedox-link]:focus-visible) {
    outline: 2px solid var(--accent-solid, var(--color-accent));
  }
</style>
