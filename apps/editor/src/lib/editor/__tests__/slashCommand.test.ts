/**
 * slashCommand.test.ts
 *
 * Unit tests for the slash command registry and filtering logic.
 * The popover itself is DOM-heavy and exercised via manual QA.
 */

import { describe, it, expect } from 'vitest';
import {
  slashCommands,
  filterCommands,
  type SlashCommand
} from '../slash-commands/registry.js';

describe('Slash command registry', () => {
  it('exposes at least 10 commands', () => {
    expect(slashCommands.length).toBeGreaterThanOrEqual(10);
  });

  it('every command has required fields', () => {
    for (const cmd of slashCommands) {
      expect(cmd.id).toBeTruthy();
      expect(cmd.label).toBeTruthy();
      expect(cmd.group).toBeTruthy();
      expect(cmd.description).toBeTruthy();
      expect(cmd.icon).toContain('<svg');
      expect(typeof cmd.action).toBe('function');
      expect(Array.isArray(cmd.keywords)).toBe(true);
    }
  });

  it('includes core block types', () => {
    const ids = slashCommands.map((c) => c.id);
    expect(ids).toContain('heading1');
    expect(ids).toContain('heading2');
    expect(ids).toContain('bulletList');
    expect(ids).toContain('orderedList');
    expect(ids).toContain('codeBlock');
    expect(ids).toContain('blockquote');
    expect(ids).toContain('divider');
    expect(ids).toContain('table');
    expect(ids).toContain('mermaid');
    expect(ids).toContain('callout');
    expect(ids).toContain('math');
    expect(ids).toContain('image');
  });

  it('command IDs are unique', () => {
    const ids = slashCommands.map((c) => c.id);
    const unique = new Set(ids);
    expect(unique.size).toBe(ids.length);
  });
});

describe('filterCommands', () => {
  it('returns all commands when query is empty', () => {
    expect(filterCommands('').length).toBe(slashCommands.length);
  });

  it('filters "cod" to code block', () => {
    const results = filterCommands('cod');
    expect(results.some((c: SlashCommand) => c.id === 'codeBlock')).toBe(true);
  });

  it('filters "h1" to heading1', () => {
    const results = filterCommands('h1');
    expect(results.some((c: SlashCommand) => c.id === 'heading1')).toBe(true);
  });

  it('filters "table" to the table command', () => {
    const results = filterCommands('table');
    expect(results.some((c: SlashCommand) => c.id === 'table')).toBe(true);
  });

  it('filters "math" to the math command', () => {
    const results = filterCommands('math');
    expect(results.some((c: SlashCommand) => c.id === 'math')).toBe(true);
  });

  it('is case-insensitive', () => {
    const lower = filterCommands('code');
    const upper = filterCommands('CODE');
    expect(lower.length).toBe(upper.length);
  });

  it('returns empty array for nonsense query', () => {
    const results = filterCommands('xyznonexistentcmd');
    expect(results.length).toBe(0);
  });

  it('keyword "alert" finds callout', () => {
    const results = filterCommands('alert');
    expect(results.some((c: SlashCommand) => c.id === 'callout')).toBe(true);
  });
});
