#!/usr/bin/env tsx
/**
 * run-budgets.ts — performance budget checker
 *
 * Requires a running Vedox dev server at http://localhost:5173.
 * Run: npx tsx tests/perf/run-budgets.ts
 *
 * Exit code 0 = all budgets met. Non-zero = at least one regression.
 */

import { readFileSync } from 'fs';
import { join } from 'path';

interface Budgets {
  coldLoadP95: number;
  cmdKFirstResultP95: number;
  fileOpenP95: number;
  modeToggleP95: number;
  fileSwitchP95: number;
}

const budgets: Budgets = JSON.parse(
  readFileSync(join(process.cwd(), 'tests/perf/budgets.json'), 'utf8')
);

const BASE_URL = process.env.PERF_BASE_URL || 'http://localhost:5173';
const SAMPLES = 10;

async function measureFetch(url: string): Promise<number> {
  const start = performance.now();
  const res = await fetch(url);
  await res.text();
  return performance.now() - start;
}

async function p95(samples: number[]): Promise<number> {
  const sorted = [...samples].sort((a, b) => a - b);
  const idx = Math.ceil(0.95 * sorted.length) - 1;
  return sorted[Math.max(0, idx)];
}

async function runMeasurements(): Promise<void> {
  console.log(`\nVedox Performance Budget Check`);
  console.log(`   Base URL: ${BASE_URL}`);
  console.log(`   Samples per measurement: ${SAMPLES}\n`);

  let failed = false;

  // Cold load: fetch the editor root
  const coldSamples: number[] = [];
  for (let i = 0; i < SAMPLES; i++) {
    coldSamples.push(await measureFetch(`${BASE_URL}/`));
  }
  const coldP95 = await p95(coldSamples);
  const coldOk = coldP95 <= budgets.coldLoadP95;
  console.log(`  Cold load p95:       ${coldP95.toFixed(0)}ms  (budget: ${budgets.coldLoadP95}ms)  ${coldOk ? 'PASS' : 'FAIL'}`);
  if (!coldOk) failed = true;

  // API search (proxy for Cmd+K first result)
  const searchSamples: number[] = [];
  for (let i = 0; i < SAMPLES; i++) {
    searchSamples.push(await measureFetch(`${BASE_URL}/api/search?q=doc`));
  }
  const searchP95 = await p95(searchSamples);
  const searchOk = searchP95 <= budgets.cmdKFirstResultP95;
  console.log(`  Cmd+K first result:  ${searchP95.toFixed(0)}ms  (budget: ${budgets.cmdKFirstResultP95}ms)  ${searchOk ? 'PASS' : 'FAIL'}`);
  if (!searchOk) failed = true;

  // Stub measurements for the remaining budgets (no UI automation yet)
  console.log(`  File open p95:       [stub — needs Playwright in v2]  (budget: ${budgets.fileOpenP95}ms)`);
  console.log(`  Mode toggle p95:     [stub — needs Playwright in v2]  (budget: ${budgets.modeToggleP95}ms)`);
  console.log(`  File switch p95:     [stub — needs Playwright in v2]  (budget: ${budgets.fileSwitchP95}ms)`);

  if (failed) {
    console.error('\nFAIL: One or more perf budgets exceeded.\n');
    process.exit(1);
  } else {
    console.log('\nPASS: All measured budgets met.\n');
    process.exit(0);
  }
}

runMeasurements().catch(err => {
  console.error('Perf measurement error:', err);
  process.exit(1);
});
