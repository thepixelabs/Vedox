#!/usr/bin/env npx tsx
/**
 * fetch-fonts.ts — download self-hosted woff2 files for the Vedox editor.
 *
 * Usage:
 *   npx tsx scripts/fetch-fonts.ts
 *
 * Idempotent: skips any file that already exists in the target directory.
 * Network errors are logged as warnings — the script never throws so CI
 * can continue even if a CDN is temporarily unreachable.
 *
 * Target directory: apps/editor/static/fonts/
 */

import { existsSync, mkdirSync, createWriteStream } from "node:fs";
import { join, resolve } from "node:path";
import { pipeline } from "node:stream/promises";
import { Readable } from "node:stream";

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

const ROOT = resolve(import.meta.dirname ?? ".", "..");
const FONTS_DIR = join(ROOT, "apps", "editor", "static", "fonts");

interface FontSpec {
  filename: string;
  url: string;
  /** Optional: try copying from a local npm package first. */
  localPath?: string;
}

const FONTS: FontSpec[] = [
  // Geist Variable — try local @vercel/font first, fall back to GitHub release
  {
    filename: "GeistVariable.woff2",
    localPath: join(
      ROOT,
      "node_modules",
      "@vercel",
      "font",
      "dist",
      "geist",
      "Geist-Variable.woff2",
    ),
    url: "https://github.com/vercel/geist-font/releases/download/1.4.01/Geist-Variable.woff2",
  },
  // Source Serif 4 Variable — Google Fonts static woff2
  {
    filename: "SourceSerif4Variable-Roman.woff2",
    url: "https://github.com/google/fonts/raw/main/ofl/sourceserif4/SourceSerif4%5Bopsz%2Cwght%5D.woff2",
  },
  {
    filename: "SourceSerif4Variable-Italic.woff2",
    url: "https://github.com/google/fonts/raw/main/ofl/sourceserif4/SourceSerif4-Italic%5Bopsz%2Cwght%5D.woff2",
  },
  // Fraunces Variable — Google Fonts repo
  {
    filename: "FrauncesVariable-Roman.woff2",
    url: "https://github.com/google/fonts/raw/main/ofl/fraunces/Fraunces%5BSOFT%2CWONK%2Copsz%2Cwght%5D.woff2",
  },
  {
    filename: "FrauncesVariable-Italic.woff2",
    url: "https://github.com/google/fonts/raw/main/ofl/fraunces/Fraunces-Italic%5BSOFT%2CWONK%2Copsz%2Cwght%5D.woff2",
  },
  // JetBrains Mono Variable — GitHub releases
  {
    filename: "JetBrainsMonoVariable.woff2",
    url: "https://github.com/JetBrains/JetBrainsMono/raw/master/fonts/variable/JetBrainsMono%5Bwght%5D.woff2",
  },
  {
    filename: "JetBrainsMonoVariable-Italic.woff2",
    url: "https://github.com/JetBrains/JetBrainsMono/raw/master/fonts/variable/JetBrainsMono-Italic%5Bwght%5D.woff2",
  },
];

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function ensureDir(dir: string): void {
  if (!existsSync(dir)) {
    mkdirSync(dir, { recursive: true });
    console.log(`  Created ${dir}`);
  }
}

async function downloadFile(url: string, dest: string): Promise<boolean> {
  try {
    const res = await fetch(url, { redirect: "follow" });
    if (!res.ok) {
      console.warn(`  WARN: ${url} responded ${res.status} — skipping`);
      return false;
    }
    if (!res.body) {
      console.warn(`  WARN: ${url} — empty body — skipping`);
      return false;
    }
    const ws = createWriteStream(dest);
    await pipeline(Readable.fromWeb(res.body as any), ws);
    return true;
  } catch (err: any) {
    console.warn(`  WARN: Failed to fetch ${url} — ${err.message}`);
    return false;
  }
}

function copyLocalFile(src: string, dest: string): boolean {
  try {
    if (!existsSync(src)) return false;
    const { copyFileSync } = require("node:fs");
    copyFileSync(src, dest);
    return true;
  } catch {
    return false;
  }
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

async function main() {
  // Allow CI to skip font downloads entirely (e.g. VEDOX_SKIP_FONTS=1)
  if (process.env.VEDOX_SKIP_FONTS === "1") {
    console.log("VEDOX_SKIP_FONTS=1 — skipping font fetch.");
    return;
  }

  console.log("Vedox font fetcher");
  console.log(`Target: ${FONTS_DIR}\n`);

  ensureDir(FONTS_DIR);

  let fetched = 0;
  let skipped = 0;
  let failed = 0;

  for (const font of FONTS) {
    const dest = join(FONTS_DIR, font.filename);

    if (existsSync(dest)) {
      console.log(`  SKIP ${font.filename} (already exists)`);
      skipped++;
      continue;
    }

    // Try local copy first
    if (font.localPath && copyLocalFile(font.localPath, dest)) {
      console.log(`  COPY ${font.filename} (from node_modules)`);
      fetched++;
      continue;
    }

    // Download from URL
    console.log(`  FETCH ${font.filename}...`);
    const ok = await downloadFile(font.url, dest);
    if (ok) {
      console.log(`  OK   ${font.filename}`);
      fetched++;
    } else {
      failed++;
    }
  }

  console.log(
    `\nDone: ${fetched} fetched, ${skipped} skipped, ${failed} failed`,
  );

  if (failed > 0) {
    console.log(
      "Some fonts could not be downloaded. Run again when network is available.",
    );
  }
}

main();
