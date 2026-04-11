<script lang="ts">
  /**
   * FontPicker — horizontal scroll row of font options with radio semantics.
   *
   * Props:
   *   category — "body" | "display" | "mono"
   *   value    — the currently selected CSS family string
   *   onChange — callback when user selects a new font
   */

  interface FontOption {
    label: string;
    cssFamily: string;
    previewText: string;
  }

  const BODY_FONTS: FontOption[] = [
    { label: "Geist", cssFamily: '"Geist Variable", "Geist", system-ui, sans-serif', previewText: "Aa Bb" },
    { label: "Inter", cssFamily: '"Inter Variable", "Inter", system-ui, sans-serif', previewText: "Aa Bb" },
    { label: "Instrument Sans", cssFamily: '"Instrument Sans Variable", "Instrument Sans", system-ui, sans-serif', previewText: "Aa Bb" },
    { label: "DM Sans", cssFamily: '"DM Sans Variable", "DM Sans", system-ui, sans-serif', previewText: "Aa Bb" },
    { label: "System UI", cssFamily: "system-ui, sans-serif", previewText: "Aa Bb" },
  ];

  const DISPLAY_FONTS: FontOption[] = [
    { label: "Fraunces", cssFamily: '"Fraunces Variable", "Fraunces", Georgia, serif', previewText: "Vedox" },
    { label: "Instrument Serif", cssFamily: '"Instrument Serif", Georgia, serif', previewText: "Vedox" },
    { label: "Playfair Display", cssFamily: '"Playfair Display Variable", "Playfair Display", Georgia, serif', previewText: "Vedox" },
    { label: "Lora", cssFamily: '"Lora Variable", "Lora", Georgia, serif', previewText: "Vedox" },
    { label: "Source Serif 4", cssFamily: '"Source Serif 4", Georgia, serif', previewText: "Vedox" },
    { label: "Match Body", cssFamily: "var(--font-body)", previewText: "Vedox" },
  ];

  const MONO_FONTS: FontOption[] = [
    { label: "JetBrains Mono", cssFamily: '"JetBrains Mono Variable", "JetBrains Mono", monospace', previewText: "fn()" },
    { label: "Commit Mono", cssFamily: '"Commit Mono", monospace', previewText: "fn()" },
    { label: "Geist Mono", cssFamily: '"Geist Mono Variable", "Geist Mono", monospace', previewText: "fn()" },
    { label: "Fira Code", cssFamily: '"Fira Code Variable", "Fira Code", monospace', previewText: "fn()" },
    { label: "System Mono", cssFamily: "ui-monospace, monospace", previewText: "fn()" },
  ];

  interface Props {
    category: "body" | "display" | "mono";
    value: string;
    onChange: (newFamily: string) => void;
  }

  let { category, value, onChange }: Props = $props();

  const options = $derived(
    category === "body" ? BODY_FONTS :
    category === "display" ? DISPLAY_FONTS :
    MONO_FONTS
  );

  const label = $derived(
    category === "body" ? "Body & UI" :
    category === "display" ? "Display & Headings" :
    "Monospace & Code"
  );

  const cssVar = $derived(`--font-${category}`);
  const storageKey = $derived(`vedox:font-${category}`);
</script>

<fieldset class="font-picker">
  <legend class="font-picker__legend">{label}</legend>
  <div class="font-picker__scroll" role="radiogroup" aria-label="{label} font">
    {#each options as option (option.label)}
      {@const isSelected = value === option.cssFamily}
      <label class="font-picker__option" class:font-picker__option--selected={isSelected}>
        <input
          type="radio"
          name="font-{category}"
          value={option.cssFamily}
          checked={isSelected}
          onchange={() => onChange(option.cssFamily)}
          class="font-picker__radio"
        />
        <span
          class="font-picker__preview"
          style="font-family: {option.cssFamily};"
          aria-hidden="true"
        >
          {option.previewText}
        </span>
        <span class="font-picker__label">{option.label}</span>
      </label>
    {/each}
  </div>
</fieldset>

<style>
  .font-picker {
    border: none;
    padding: 0;
    margin: 0;
  }

  .font-picker__legend {
    font-size: var(--text-caption);
    font-weight: 600;
    letter-spacing: var(--tracking-wider, 0.06em);
    text-transform: uppercase;
    color: var(--text-3);
    margin-bottom: var(--space-3);
  }

  .font-picker__scroll {
    display: flex;
    gap: var(--space-2);
    overflow-x: auto;
    scroll-snap-type: x mandatory;
    padding-bottom: var(--space-2);
    -webkit-overflow-scrolling: touch;
    scrollbar-width: thin;
    scrollbar-color: var(--border-default) transparent;
  }

  .font-picker__scroll::-webkit-scrollbar {
    height: 4px;
  }
  .font-picker__scroll::-webkit-scrollbar-track { background: transparent; }
  .font-picker__scroll::-webkit-scrollbar-thumb {
    background: var(--border-default);
    border-radius: 2px;
  }

  .font-picker__option {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-1);
    padding: var(--space-3) var(--space-4);
    min-width: 88px;
    border: 1px solid var(--border-default);
    border-radius: var(--radius-lg);
    background: var(--surface-2);
    cursor: pointer;
    scroll-snap-align: start;
    flex-shrink: 0;
    transition:
      border-color var(--duration-fast) var(--ease-out),
      background var(--duration-fast) var(--ease-out);
  }

  .font-picker__option:hover {
    border-color: var(--border-strong);
    background: var(--surface-3);
  }

  .font-picker__option--selected {
    border-color: var(--accent-solid);
    background: var(--accent-subtle);
  }

  .font-picker__option:focus-within {
    outline: 2px solid var(--accent-solid);
    outline-offset: 2px;
  }

  .font-picker__radio {
    position: absolute;
    opacity: 0;
    width: 1px;
    height: 1px;
    pointer-events: none;
  }

  .font-picker__preview {
    font-size: 18px;
    line-height: 1.2;
    color: var(--text-1);
    white-space: nowrap;
  }

  .font-picker__label {
    font-size: var(--text-caption);
    color: var(--text-3);
    white-space: nowrap;
    font-family: var(--font-body);
  }

  .font-picker__option--selected .font-picker__label {
    color: var(--accent-text);
  }
</style>
