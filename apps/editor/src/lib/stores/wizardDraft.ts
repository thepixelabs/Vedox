/**
 * wizardDraft.ts — Singleton store for the in-flight /projects/new wizard state.
 *
 * Survives client-side navigation so users can return to their draft after
 * jumping to another page. In-memory only — intentionally not persisted to
 * localStorage (the draft lives only for the session).
 */

import { writable, get } from 'svelte/store';

export interface WizardAiPanelDraft {
  open: boolean;
  selectedCategories: string[];
  selectedPlatform: string;
  selectedOS: string;
  selectedInterface: string;
  selectedAudience: string;
  selectedTone: string;
  selectedLength: string;
  selectedLanguageStyle: string;
  selectedProvider: string;
  selectedAccount: string;
  nameCount: number;
  generatedNames: string[];
  selectedName: string;
  phase: 'idle' | 'loading' | 'done' | 'error';
}

export interface WizardDraft {
  projectName: string;
  tagline: string;
  description: string;
  aiPanel: WizardAiPanelDraft;
}

/** The active draft. null = no draft (card not shown). */
export const draft = writable<WizardDraft | null>(null);

/** Save the current wizard state before navigating away. */
export function saveDraft(d: WizardDraft): void {
  draft.set(d);
}

/** Discard the draft — called on successful project creation or manual dismiss. */
export function clearDraft(): void {
  draft.set(null);
}

/** Returns true when the draft has at least one non-empty field worth showing. */
export function hasMeaningfulDraft(): boolean {
  const d = get(draft);
  return (
    d !== null &&
    (d.projectName.trim().length > 0 ||
      d.tagline.trim().length > 0 ||
      d.description.trim().length > 0 ||
      d.aiPanel.generatedNames.length > 0)
  );
}

/** Default AI panel state — used to initialise both the panel and the draft. */
export function defaultAiPanelDraft(): WizardAiPanelDraft {
  return {
    open: false,
    selectedCategories: [],
    selectedPlatform: 'any',
    selectedOS: 'any',
    selectedInterface: 'any',
    selectedAudience: 'general',
    selectedTone: 'professional',
    selectedLength: 'medium',
    selectedLanguageStyle: 'modern',
    selectedProvider: '',
    selectedAccount: '',
    nameCount: 12,
    generatedNames: [],
    selectedName: '',
    phase: 'idle',
  };
}
