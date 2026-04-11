/**
 * statusBar.test.ts
 *
 * Tests the word-count derivation logic used by StatusBar.svelte.
 *
 * We test the extraction function directly by reproducing the logic
 * from the component — this keeps the test fast and independent of
 * Svelte compilation.
 */

import { describe, it, expect } from 'vitest';

/**
 * Reproduced from StatusBar.svelte.
 * Counts words in markdown, excluding frontmatter, code blocks, and
 * inline code.
 */
function countWords(content: string): number {
  let text = content;
  text = text.replace(/^---\n[\s\S]*?\n---\n?/, '');
  text = text.replace(/```[\s\S]*?```/g, '');
  text = text.replace(/`[^`]*`/g, '');
  text = text.replace(/[#>*_\[\]()]/g, ' ');
  const words = text.trim().split(/\s+/).filter((w) => w.length > 0);
  return words.length;
}

function readingTimeMinutes(wordCount: number): number {
  return Math.max(1, Math.round(wordCount / 200));
}

describe('StatusBar word count', () => {
  it('counts words in a simple paragraph', () => {
    expect(countWords('Hello world this is a test')).toBe(6);
  });

  it('excludes frontmatter', () => {
    const md = `---
title: Test
author: Me
---

Hello world`;
    expect(countWords(md)).toBe(2);
  });

  it('excludes fenced code blocks', () => {
    const md = `Before code.

\`\`\`js
function foo() { return 42; }
const x = 100;
\`\`\`

After code.`;
    // "Before code. After code." — 4 words
    expect(countWords(md)).toBe(4);
  });

  it('excludes inline code', () => {
    const md = 'Use the `useState` hook for state.';
    // "Use the hook for state." → 5 words
    expect(countWords(md)).toBe(5);
  });

  it('counts a 200-word doc correctly within 10%', () => {
    const words = Array(200).fill('word').join(' ');
    const count = countWords(words);
    expect(count).toBeGreaterThanOrEqual(195);
    expect(count).toBeLessThanOrEqual(205);
  });

  it('strips heading hash marks', () => {
    const md = '# Heading One\n\nBody text here.';
    // Heading One Body text here. → 5 words
    expect(countWords(md)).toBe(5);
  });

  it('empty doc → 0 words', () => {
    expect(countWords('')).toBe(0);
  });

  it('whitespace-only doc → 0 words', () => {
    expect(countWords('   \n\n  \t  ')).toBe(0);
  });
});

describe('StatusBar reading time', () => {
  it('minimum reading time is 1 minute', () => {
    expect(readingTimeMinutes(0)).toBe(1);
    expect(readingTimeMinutes(50)).toBe(1);
  });

  it('200 words → 1 minute', () => {
    expect(readingTimeMinutes(200)).toBe(1);
  });

  it('400 words → 2 minutes', () => {
    expect(readingTimeMinutes(400)).toBe(2);
  });

  it('1000 words → 5 minutes', () => {
    expect(readingTimeMinutes(1000)).toBe(5);
  });

  it('rounds to nearest minute', () => {
    expect(readingTimeMinutes(250)).toBe(1); // 1.25 → 1
    expect(readingTimeMinutes(350)).toBe(2); // 1.75 → 2
  });
});
