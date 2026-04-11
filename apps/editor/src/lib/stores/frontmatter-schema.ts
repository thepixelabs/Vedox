/**
 * frontmatter-schema.ts — FrontmatterStore
 *
 * Holds the parsed frontmatter fields of the currently-open document and emits
 * a typed field list for the MetadataSidecar to render.
 *
 * The store is updated in two steps:
 *   1. setFromFrontmatter() — called when the editor parses a document.
 *   2. setGitMetadata()     — called when the API returns git file metadata.
 *
 * Both are idempotent and can be called in any order.
 */

import { writable } from "svelte/store";

export interface FrontmatterField {
  key: string;
  value: string | string[] | boolean | number | null;
  type: "string" | "array" | "boolean" | "number" | "date";
}

export interface DocMetadata {
  path: string | null;
  fields: FrontmatterField[];
  lastModifiedRaw: string | null; // ISO timestamp from git
  contributors: string[]; // display names from git log
}

function createFrontmatterStore() {
  const metadata = writable<DocMetadata>({
    path: null,
    fields: [],
    lastModifiedRaw: null,
    contributors: [],
  });

  function setFromFrontmatter(
    path: string,
    raw: Record<string, unknown>,
  ): void {
    const fields: FrontmatterField[] = Object.entries(raw).map(
      ([key, value]) => {
        if (Array.isArray(value))
          return { key, value: value.map(String), type: "array" as const };
        if (typeof value === "boolean")
          return { key, value, type: "boolean" as const };
        if (typeof value === "number")
          return { key, value, type: "number" as const };
        if (typeof value === "string" && /^\d{4}-\d{2}-\d{2}/.test(value))
          return { key, value, type: "date" as const };
        return { key, value: String(value ?? ""), type: "string" as const };
      },
    );
    metadata.update((m) => ({ ...m, path, fields }));
  }

  function setGitMetadata(
    lastModifiedRaw: string | null,
    contributors: string[],
  ): void {
    metadata.update((m) => ({ ...m, lastModifiedRaw, contributors }));
  }

  function clear(): void {
    metadata.set({
      path: null,
      fields: [],
      lastModifiedRaw: null,
      contributors: [],
    });
  }

  return {
    subscribe: metadata.subscribe,
    setFromFrontmatter,
    setGitMetadata,
    clear,
  };
}

export const frontmatterStore = createFrontmatterStore();
