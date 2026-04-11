/**
 * reviewQueue.ts — AI review queue store (v1 stub)
 *
 * In v1, the queue is in-memory only. A future phase will wire this
 * to real AI suggestions from the backend (SSE stream or polling).
 * The store exposes the shape that the UI components consume.
 */
import { writable, derived } from 'svelte/store';

export type ReviewStatus = 'pending' | 'accepted' | 'rejected';

export interface ReviewSuggestion {
  id: string;
  docPath: string;
  type: 'grammar' | 'clarity' | 'structure' | 'style';
  original: string;
  suggested: string;
  reason: string;
  status: ReviewStatus;
  createdAt: string; // ISO timestamp
}

function createReviewQueueStore() {
  // Seed with a few stub suggestions so the UI looks populated
  const initial: ReviewSuggestion[] = [
    {
      id: 'stub-1',
      docPath: 'docs/getting-started.md',
      type: 'clarity',
      original: 'The system then processes the request',
      suggested: 'The system processes the request',
      reason: 'Removing "then" tightens the sentence.',
      status: 'pending',
      createdAt: new Date(Date.now() - 3600000).toISOString(),
    },
    {
      id: 'stub-2',
      docPath: 'docs/getting-started.md',
      type: 'grammar',
      original: 'Click the button, and then confirm',
      suggested: 'Click the button, then confirm',
      reason: 'Unnecessary comma before "and then".',
      status: 'pending',
      createdAt: new Date(Date.now() - 1800000).toISOString(),
    },
  ];

  const suggestions = writable<ReviewSuggestion[]>(initial);

  const pendingCount = derived(suggestions, $s => $s.filter(s => s.status === 'pending').length);

  function accept(id: string): void {
    suggestions.update(ss => ss.map(s => s.id === id ? { ...s, status: 'accepted' as const } : s));
  }

  function reject(id: string): void {
    suggestions.update(ss => ss.map(s => s.id === id ? { ...s, status: 'rejected' as const } : s));
  }

  function dismiss(id: string): void {
    suggestions.update(ss => ss.filter(s => s.id !== id));
  }

  function addSuggestion(s: Omit<ReviewSuggestion, 'id' | 'status' | 'createdAt'>): void {
    const suggestion: ReviewSuggestion = {
      ...s,
      id: Math.random().toString(36).slice(2, 9),
      status: 'pending',
      createdAt: new Date().toISOString(),
    };
    suggestions.update(ss => [suggestion, ...ss]);
  }

  return {
    subscribe: suggestions.subscribe,
    pendingCount,
    accept,
    reject,
    dismiss,
    addSuggestion,
  };
}

export const reviewQueueStore = createReviewQueueStore();
