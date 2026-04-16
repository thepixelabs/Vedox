/**
 * onboarding.svelte.ts — Svelte 5 $state store for the 5-step onboarding flow.
 *
 * Steps:
 *   1. detect-projects  — scan filesystem for existing git repos with docs
 *   2. setup-repos      — create/register a doc repo (or skip to inbox fallback)
 *   3. install-agent    — pick providers, show install progress
 *   4. configure-voice  — opt-in, pick trigger method (coming soon)
 *   5. all-done         — summary + first doc suggestion
 *
 * All steps are skippable per founder override OQ-E.
 * State persists to localStorage so the user can resume after a reload.
 *
 * Usage:
 *   import { onboardingStore } from '$lib/stores/onboarding.svelte';
 *   onboardingStore.step        // reactive step index 1-5
 *   onboardingStore.next()
 *   onboardingStore.skip()
 *   onboardingStore.goTo(3)
 *   onboardingStore.complete()
 *   onboardingStore.reset()
 */

import { browser } from '$app/environment';

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

export const STEP_COUNT = 5;

export type OnboardingStepId =
  | 'detect-projects'
  | 'setup-repos'
  | 'install-agent'
  | 'configure-voice'
  | 'all-done';

export interface OnboardingStepDef {
  id: OnboardingStepId;
  index: number;   // 1-based
  title: string;
  description: string;
  skippable: boolean;
}

export const STEPS: OnboardingStepDef[] = [
  {
    id: 'detect-projects',
    index: 1,
    title: 'detect projects',
    description: 'find existing git repos that contain docs',
    skippable: true,
  },
  {
    id: 'setup-repos',
    index: 2,
    title: 'create or register a doc repo',
    description: 'point vedox at a dedicated docs folder',
    skippable: true,
  },
  {
    id: 'install-agent',
    index: 3,
    title: 'install doc agent',
    description: 'connect your ai providers',
    skippable: true,
  },
  {
    id: 'configure-voice',
    index: 4,
    title: 'configure voice',
    description: 'set a push-to-talk trigger (optional)',
    skippable: true,
  },
  {
    id: 'all-done',
    index: 5,
    title: "you're ready",
    description: 'vedox is set up and ready to use',
    skippable: false,
  },
];

// ---------------------------------------------------------------------------
// Persistence
// ---------------------------------------------------------------------------

const STORAGE_KEY = 'vedox:onboarding';

interface PersistedState {
  step: number;
  completed: boolean;
  skippedSteps: number[];
  /** Which providers were selected during install-agent step */
  selectedProviders: string[];
  /** Repos registered in setup-repos step */
  registeredRepos: string[];
  /** Whether voice was configured */
  voiceConfigured: boolean;
  /** Detected projects from step 1 */
  detectedProjects: DetectedProject[];
}

export interface DetectedProject {
  path: string;
  name: string;
  hasGit: boolean;
  docCount: number;
}

function loadState(): PersistedState {
  if (!browser) {
    return defaultState();
  }
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      return JSON.parse(raw) as PersistedState;
    }
  } catch {
    // ignore parse errors
  }
  return defaultState();
}

function defaultState(): PersistedState {
  return {
    step: 1,
    completed: false,
    skippedSteps: [],
    selectedProviders: [],
    registeredRepos: [],
    voiceConfigured: false,
    detectedProjects: [],
  };
}

function saveState(state: PersistedState): void {
  if (!browser) return;
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  } catch {
    // localStorage unavailable — non-fatal
  }
}

// ---------------------------------------------------------------------------
// Store (Svelte 5 $state rune)
// ---------------------------------------------------------------------------

function createOnboardingStore() {
  let _state = $state<PersistedState>(loadState());

  function persist() {
    saveState(_state);
  }

  return {
    // ── Reactive reads ────────────────────────────────────────────────────
    get step() { return _state.step; },
    get completed() { return _state.completed; },
    get skippedSteps() { return _state.skippedSteps; },
    get selectedProviders() { return _state.selectedProviders; },
    get registeredRepos() { return _state.registeredRepos; },
    get voiceConfigured() { return _state.voiceConfigured; },
    get detectedProjects() { return _state.detectedProjects; },

    get currentStepDef(): OnboardingStepDef {
      return STEPS[_state.step - 1] ?? STEPS[STEPS.length - 1];
    },

    get isFirstStep(): boolean {
      return _state.step === 1;
    },

    get isLastStep(): boolean {
      return _state.step === STEP_COUNT;
    },

    get progressPercent(): number {
      return Math.round(((_state.step - 1) / (STEP_COUNT - 1)) * 100);
    },

    // ── Navigation ────────────────────────────────────────────────────────

    next(): void {
      if (_state.step < STEP_COUNT) {
        _state = { ..._state, step: _state.step + 1 };
        persist();
      }
    },

    back(): void {
      if (_state.step > 1) {
        _state = { ..._state, step: _state.step - 1 };
        persist();
      }
    },

    skip(): void {
      const skipped = [..._state.skippedSteps, _state.step];
      if (_state.step < STEP_COUNT) {
        _state = { ..._state, step: _state.step + 1, skippedSteps: skipped };
      } else {
        _state = { ..._state, skippedSteps: skipped };
      }
      persist();
    },

    goTo(targetStep: number): void {
      if (targetStep >= 1 && targetStep <= STEP_COUNT) {
        _state = { ..._state, step: targetStep };
        persist();
      }
    },

    complete(): void {
      _state = { ..._state, step: STEP_COUNT, completed: true };
      persist();
    },

    reset(): void {
      _state = defaultState();
      persist();
    },

    // ── Data mutations ────────────────────────────────────────────────────

    setDetectedProjects(projects: DetectedProject[]): void {
      _state = { ..._state, detectedProjects: projects };
      persist();
    },

    addRegisteredRepo(repoPath: string): void {
      const repos = [..._state.registeredRepos, repoPath];
      _state = { ..._state, registeredRepos: repos };
      persist();
    },

    setSelectedProviders(providers: string[]): void {
      _state = { ..._state, selectedProviders: providers };
      persist();
    },

    setVoiceConfigured(value: boolean): void {
      _state = { ..._state, voiceConfigured: value };
      persist();
    },
  };
}

export const onboardingStore = createOnboardingStore();
