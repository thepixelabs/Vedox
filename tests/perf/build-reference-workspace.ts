#!/usr/bin/env tsx
/**
 * build-reference-workspace.ts
 *
 * Generates a deterministic synthetic Vedox workspace for performance testing.
 * Output: tests/perf/reference-workspace/ (gitignored)
 *
 * Run: npx tsx tests/perf/build-reference-workspace.ts
 */

import { writeFileSync, mkdirSync, existsSync } from 'fs';
import { join } from 'path';

const OUT = join(process.cwd(), 'tests/perf/reference-workspace');
const FILE_COUNT = 10_000;
const DIRS = ['docs', 'guides', 'api', 'tutorials', 'reference'];

// Simple seeded pseudo-random for determinism
function seededRand(seed: number) {
  let s = seed;
  return () => {
    s = (s * 1664525 + 1013904223) & 0xffffffff;
    return (s >>> 0) / 0xffffffff;
  };
}

const rand = seededRand(42);

function generateFrontmatter(i: number): string {
  return `---
title: "Document ${i}"
type: doc
status: draft
tags: [${['guide', 'api', 'tutorial', 'reference'][i % 4]}, ${['beginner', 'advanced'][i % 2]}]
date: 2026-0${(i % 9) + 1}-${String((i % 28) + 1).padStart(2, '0')}
---\n\n`;
}

function generateBody(i: number): string {
  const paras = Math.floor(rand() * 8) + 2;
  const lines: string[] = [`# Document ${i}\n`];
  for (let p = 0; p < paras; p++) {
    const words = Math.floor(rand() * 40) + 10;
    const wordList = Array.from({ length: words }, (_, w) =>
      `word${(i * paras * 50 + p * 50 + w) % 2000}`
    );
    lines.push(wordList.join(' ') + '\n');
  }
  if (i % 5 === 0) {
    lines.push(`\`\`\`typescript\nconst x${i} = ${i};\nconsole.log(x${i});\n\`\`\`\n`);
  }
  if (i % 7 === 0) {
    lines.push(`| Column A | Column B | Column C |\n|----------|----------|----------|\n| ${i} | ${i + 1} | ${i + 2} |\n`);
  }
  return lines.join('\n');
}

console.log(`Generating ${FILE_COUNT} files in ${OUT}...`);

// Create top-level dirs
DIRS.forEach(d => mkdirSync(join(OUT, d), { recursive: true }));

for (let i = 0; i < FILE_COUNT; i++) {
  const dir = DIRS[i % DIRS.length];
  const subdir = Math.floor(i / 100);
  const dirPath = join(OUT, dir, `section-${subdir}`);
  if (!existsSync(dirPath)) mkdirSync(dirPath, { recursive: true });

  const content = generateFrontmatter(i) + generateBody(i);
  writeFileSync(join(dirPath, `doc-${i}.md`), content, 'utf8');

  if (i % 1000 === 999) console.log(`  ${i + 1}/${FILE_COUNT} files written`);
}

console.log('Done.');
