/**
 * onboarding.ts — Static copy for first-run and contextual onboarding hints.
 *
 * Not rendered yet. A future phase will wire these into tooltip/popover
 * components. Written now so the copy is reviewed alongside the UX polish pass.
 */

export const onboarding = {
  firstRun: {
    headline: "Start with a folder",
    body: "Vedox works like a local-first notebook. Open any folder and start writing — your docs stay on disk as plain Markdown.",
    cta: "Open folder",
  },
  firstDoc: {
    headline: "Your first document",
    body: "Write in rich mode for prose, or switch to source mode for raw Markdown. ⌘Shift+M toggles between them.",
  },
  commandPalette: {
    headline: "⌘K does most things",
    body: "Search docs, switch themes, split panes. The fastest way to do anything in Vedox.",
  },
  splitPane: {
    headline: "Split the view",
    body: "⌘\\ opens a second pane. Compare docs, reference while writing, or review a diff side-by-side.",
  },
  aiReview: {
    headline: "AI writing review",
    body: "The AI reads your writing and flags clarity issues, grammar nits, and structural suggestions. Accept or reject each one.",
  },
} as const;
