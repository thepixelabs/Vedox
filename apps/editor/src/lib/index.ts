/**
 * $lib/index.ts — barrel export for public library surface.
 *
 * Import directly from $lib/stores/theme, $lib/stores/projects, etc.
 * for the full API. This file re-exports only the most commonly used items.
 */

export { themeStore } from "./stores/theme";
export type { Theme } from "./stores/theme";
export { projectsStore, hasProjects } from "./stores/projects";
export type { Project, DocEntry } from "./stores/projects";
export { sidebarStore } from "./stores/sidebar";
export type { SidebarState } from "./stores/sidebar";
