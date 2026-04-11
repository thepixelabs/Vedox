<script lang="ts">
  /**
   * / — root page.
   *
   * Redirects immediately to /projects.
   * The redirect happens in +page.ts via `redirect()` so there is no
   * flash of content, but this component is the fallback in case the
   * load function is somehow bypassed.
   */
  import { goto } from "$app/navigation";
  import { onMount } from "svelte";

  onMount(() => {
    goto("/projects", { replaceState: true });
  });
</script>

<!-- Fallback: if JS hasn't run yet, provide a visible link -->
<noscript>
  <meta http-equiv="refresh" content="0; url=/projects" />
  <a href="/projects">Go to projects</a>
</noscript>
