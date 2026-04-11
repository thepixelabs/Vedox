<!--
  FrontmatterPanel.svelte

  Structured metadata panel displayed above the Tiptap editor body in
  WYSIWYG mode. Shows the five canonical frontmatter fields (title, type,
  status, date, tags) as editable form controls — never as raw YAML.

  Validation runs on blur (Zod warn-only — never hard-blocks saving).
  Additional/unknown frontmatter keys are preserved opaque and re-injected
  verbatim on serialize; they are not shown in this panel.

  Props:
    frontmatter: FrontmatterFields (bound — parent tracks changes)
    readonly?: boolean (default false)

  Events:
    onchange: dispatched when any field is committed (blur / Enter / tag removal)
-->

<script lang="ts">
  import type { FrontmatterFields } from './utils/frontmatter.js';
  import {
    validateFrontmatter,
    type FrontmatterValidationResult
  } from './utils/frontmatter.js';

  // ---------------------------------------------------------------------------
  // Props
  // ---------------------------------------------------------------------------

  interface Props {
    frontmatter: FrontmatterFields;
    readonly?: boolean;
    onchange?: (fm: FrontmatterFields) => void;
  }

  let { frontmatter = $bindable(), readonly = false, onchange }: Props = $props();

  // ---------------------------------------------------------------------------
  // Local state
  // ---------------------------------------------------------------------------

  let validation: FrontmatterValidationResult = $state({ success: true, errors: {} });
  let tagInput = $state('');
  let tagInputEl: HTMLInputElement | undefined = $state(undefined);

  // ---------------------------------------------------------------------------
  // Field options
  // ---------------------------------------------------------------------------

  const TYPE_OPTIONS = [
    { value: '', label: '— select type —' },
    { value: 'adr', label: 'ADR' },
    { value: 'how-to', label: 'How-To' },
    { value: 'runbook', label: 'Runbook' },
    { value: 'readme', label: 'README' },
    { value: 'api-reference', label: 'API Reference' },
    { value: 'explanation', label: 'Explanation' },
    { value: 'issue', label: 'Issue' },
    { value: 'platform', label: 'Platform' },
    { value: 'infrastructure', label: 'Infrastructure' },
    { value: 'network', label: 'Network' },
    { value: 'logging', label: 'Logging' }
  ];

  const STATUS_OPTIONS = [
    { value: '', label: '— select status —' },
    { value: 'draft', label: 'Draft' },
    { value: 'review', label: 'In Review' },
    { value: 'published', label: 'Published' },
    { value: 'deprecated', label: 'Deprecated' },
    { value: 'superseded', label: 'Superseded' }
  ];

  const SLUG_PATTERN = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

  function slugify(input: string): string {
    return input
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/-+/g, '-')
      .replace(/^-+|-+$/g, '');
  }

  let slugError = $state('');

  // Auto-derive slug from title while the slug field is empty. Once the user
  // (or a saved document) has a slug, we never overwrite it.
  $effect(() => {
    const title = frontmatter.title;
    if (!frontmatter.slug) {
      const derived = slugify(title ?? '');
      if (derived && frontmatter.slug !== derived) {
        frontmatter.slug = derived;
      }
    }
  });

  function handleSlugInput(): void {
    const v = frontmatter.slug ?? '';
    slugError = v === '' || SLUG_PATTERN.test(v)
      ? ''
      : 'Slug must be lowercase kebab-case (e.g. my-document)';
  }

  // ---------------------------------------------------------------------------
  // Handlers
  // ---------------------------------------------------------------------------

  function handleBlur(): void {
    validation = validateFrontmatter(frontmatter);
    onchange?.(frontmatter);
  }

  function handleTagKeydown(e: KeyboardEvent): void {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault();
      addTag();
    } else if (e.key === 'Backspace' && tagInput === '' && frontmatter.tags.length > 0) {
      removeTag(frontmatter.tags.length - 1);
    }
  }

  function addTag(): void {
    const tag = tagInput.trim().replace(/^,+|,+$/g, '');
    if (tag && !frontmatter.tags.includes(tag)) {
      frontmatter.tags = [...frontmatter.tags, tag];
      onchange?.(frontmatter);
    }
    tagInput = '';
  }

  function removeTag(index: number): void {
    frontmatter.tags = frontmatter.tags.filter((_, i) => i !== index);
    onchange?.(frontmatter);
    tagInputEl?.focus();
  }
</script>

<div class="frontmatter-panel" aria-label="Document metadata">
  <!-- Title -->
  <div class="fm-field fm-field--title">
    <label class="fm-label" for="fm-title">Title</label>
    <input
      id="fm-title"
      type="text"
      class="fm-input fm-input--title"
      class:fm-input--error={validation.errors.title}
      bind:value={frontmatter.title}
      onblur={handleBlur}
      disabled={readonly}
      placeholder="Document title"
      autocomplete="off"
    />
    {#if validation.errors.title}
      <span class="fm-error" role="alert">{validation.errors.title}</span>
    {/if}
  </div>

  <!-- Slug -->
  <div class="fm-field">
    <label class="fm-label" for="fm-slug">Slug</label>
    <input
      id="fm-slug"
      type="text"
      class="fm-input"
      class:fm-input--error={slugError}
      bind:value={frontmatter.slug}
      oninput={handleSlugInput}
      onblur={handleBlur}
      disabled={readonly}
      placeholder="my-document"
      autocomplete="off"
      spellcheck="false"
    />
    {#if slugError}
      <span class="fm-error" role="alert">{slugError}</span>
    {/if}
  </div>

  <!-- Row: type + status + date -->
  <div class="fm-row">
    <!-- Type -->
    <div class="fm-field">
      <label class="fm-label" for="fm-type">Type</label>
      <select
        id="fm-type"
        class="fm-select"
        class:fm-input--error={validation.errors.type}
        bind:value={frontmatter.type}
        onblur={handleBlur}
        onchange={handleBlur}
        disabled={readonly}
      >
        {#each TYPE_OPTIONS as opt (opt.value)}
          <option value={opt.value}>{opt.label}</option>
        {/each}
      </select>
    </div>

    <!-- Status -->
    <div class="fm-field">
      <label class="fm-label" for="fm-status">Status</label>
      <select
        id="fm-status"
        class="fm-select"
        bind:value={frontmatter.status}
        onblur={handleBlur}
        onchange={handleBlur}
        disabled={readonly}
      >
        {#each STATUS_OPTIONS as opt (opt.value)}
          <option value={opt.value}>{opt.label}</option>
        {/each}
      </select>
      <!-- Status badge preview -->
      {#if frontmatter.status}
        <span class="fm-status-badge fm-status-badge--{frontmatter.status}">
          {frontmatter.status}
        </span>
      {/if}
    </div>

    <!-- Date -->
    <div class="fm-field">
      <label class="fm-label" for="fm-date">Date</label>
      <input
        id="fm-date"
        type="date"
        class="fm-input"
        class:fm-input--error={validation.errors.date}
        bind:value={frontmatter.date}
        onblur={handleBlur}
        disabled={readonly}
      />
      {#if validation.errors.date}
        <span class="fm-error" role="alert">{validation.errors.date}</span>
      {/if}
    </div>
  </div>

  <!-- Tags -->
  <div class="fm-field">
    <label class="fm-label" for="fm-tag-input">Tags</label>
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div
      class="fm-tags-container"
      onclick={() => tagInputEl?.focus()}
    >
      {#each frontmatter.tags as tag, i (tag)}
        <span class="fm-tag">
          {tag}
          {#if !readonly}
            <button
              type="button"
              class="fm-tag__remove"
              aria-label="Remove tag {tag}"
              onclick={() => removeTag(i)}
            >&#x2715;</button>
          {/if}
        </span>
      {/each}
      {#if !readonly}
        <input
          bind:this={tagInputEl}
          id="fm-tag-input"
          type="text"
          class="fm-tag-input"
          bind:value={tagInput}
          onkeydown={handleTagKeydown}
          onblur={() => { if (tagInput.trim()) addTag(); else handleBlur(); }}
          placeholder={frontmatter.tags.length === 0 ? 'Add tags…' : ''}
          autocomplete="off"
        />
      {/if}
    </div>
  </div>
</div>

<style>
  .frontmatter-panel {
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 16px 20px;
    border-bottom: 1px solid var(--color-border);
    background: var(--color-surface-elevated);
  }

  .fm-field {
    display: flex;
    flex-direction: column;
    gap: 4px;
    position: relative;
  }

  .fm-field--title {
    flex: 1;
  }

  .fm-row {
    display: flex;
    gap: 16px;
    flex-wrap: wrap;
  }

  .fm-row .fm-field {
    flex: 1;
    min-width: 140px;
  }

  .fm-label {
    font-size: 11px;
    font-weight: 600;
    color: var(--color-text-muted);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    user-select: none;
  }

  .fm-input {
    background: var(--color-surface-base);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-text-primary);
    font-size: 14px;
    padding: 6px 8px;
    outline: none;
    transition: border-color var(--duration-fast) var(--ease-out);
    width: 100%;
    box-sizing: border-box;
    font-family: inherit;
  }

  .fm-input--title {
    font-size: 16px;
    font-weight: 600;
    padding: 8px 10px;
  }

  .fm-input:focus {
    border-color: var(--color-accent);
  }

  .fm-input:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .fm-input--error {
    border-color: var(--color-error) !important;
  }

  .fm-select {
    background: var(--color-surface-base);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-text-primary);
    font-size: 13px;
    padding: 6px 8px;
    outline: none;
    transition: border-color var(--duration-fast) var(--ease-out);
    width: 100%;
    cursor: pointer;
    appearance: none;
    background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='10' height='6'%3E%3Cpath d='M0 0l5 6 5-6z' fill='%23a6adc8'/%3E%3C/svg%3E");
    background-repeat: no-repeat;
    background-position: right 8px center;
    padding-right: 28px;
  }

  .fm-select:focus {
    border-color: var(--color-accent);
  }

  .fm-select:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .fm-error {
    font-size: 11px;
    color: var(--color-error);
    margin-top: 2px;
  }

  /* Status badge */
  .fm-status-badge {
    display: inline-block;
    font-size: 10px;
    font-weight: 700;
    padding: 2px 6px;
    border-radius: 100px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    margin-top: 4px;
    width: fit-content;
  }

  .fm-status-badge--draft {
    background: var(--color-surface-overlay);
    color: var(--color-text-muted);
  }

  .fm-status-badge--review {
    background: color-mix(in srgb, var(--color-warning) 20%, transparent);
    color: var(--color-warning);
  }

  .fm-status-badge--published {
    background: color-mix(in srgb, var(--color-success) 20%, transparent);
    color: var(--color-success);
  }

  .fm-status-badge--deprecated,
  .fm-status-badge--superseded {
    background: color-mix(in srgb, var(--color-error) 15%, transparent);
    color: var(--color-error);
  }

  /* Tags */
  .fm-tags-container {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    align-items: center;
    background: var(--color-surface-base);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 5px 8px;
    min-height: 36px;
    cursor: text;
    transition: border-color var(--duration-fast) var(--ease-out);
  }

  .fm-tags-container:focus-within {
    border-color: var(--color-accent);
  }

  .fm-tag {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    background: var(--color-accent-subtle);
    color: var(--color-accent);
    font-size: 12px;
    font-weight: 500;
    padding: 2px 8px 2px 8px;
    border-radius: 100px;
    line-height: 1.4;
  }

  .fm-tag__remove {
    background: none;
    border: none;
    color: var(--color-accent);
    cursor: pointer;
    padding: 0;
    font-size: 10px;
    line-height: 1;
    opacity: 0.7;
    transition: opacity var(--duration-fast) var(--ease-out);
    display: flex;
    align-items: center;
  }

  .fm-tag__remove:hover {
    opacity: 1;
  }

  .fm-tag-input {
    background: none;
    border: none;
    color: var(--color-text-primary);
    font-size: 13px;
    outline: none;
    flex: 1;
    min-width: 80px;
    padding: 0;
    font-family: inherit;
  }

  .fm-tag-input::placeholder {
    color: var(--color-text-subtle);
  }
</style>
