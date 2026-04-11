/**
 * callout.test.ts
 *
 * Unit tests for the Callout Tiptap extension.
 * Tests creation, serialization, and the five callout types.
 */

import { describe, it, expect, beforeEach } from 'vitest';
import { Editor } from '@tiptap/core';
import StarterKit from '@tiptap/starter-kit';
import { Markdown } from 'tiptap-markdown';
import { Callout, CALLOUT_COLORS, CALLOUT_ICONS, type CalloutType } from '../extensions/Callout.js';

function createTestEditor(content = ''): Editor {
  const el = document.createElement('div');
  document.body.appendChild(el);
  return new Editor({
    element: el,
    extensions: [
      StarterKit.configure({ history: false, heading: { levels: [1, 2, 3, 4] } }),
      Markdown.configure({ html: false, bulletListMarker: '-' }),
      Callout
    ],
    content
  });
}

describe('Callout extension', () => {
  describe('constants', () => {
    it('defines all 5 callout types with colors', () => {
      const types: CalloutType[] = ['NOTE', 'TIP', 'WARNING', 'DANGER', 'INFO'];
      for (const t of types) {
        expect(CALLOUT_COLORS[t]).toBeDefined();
        expect(CALLOUT_COLORS[t]).toContain('var(--');
      }
    });

    it('defines SVG icons for all 5 types', () => {
      const types: CalloutType[] = ['NOTE', 'TIP', 'WARNING', 'DANGER', 'INFO'];
      for (const t of types) {
        expect(CALLOUT_ICONS[t]).toContain('<svg');
        expect(CALLOUT_ICONS[t]).toContain('</svg>');
      }
    });
  });

  describe('insertion via command', () => {
    let editor: Editor;
    beforeEach(() => {
      editor = createTestEditor();
    });

    it('inserts a NOTE callout with default content', () => {
      editor.commands.insertCallout('NOTE');
      const json = editor.getJSON();
      const callout = findNode(json, 'callout');
      expect(callout).toBeDefined();
      expect(callout?.attrs?.calloutType).toBe('NOTE');
    });

    it('inserts a TIP callout with custom title', () => {
      editor.commands.insertCallout('TIP', 'Pro tip');
      const json = editor.getJSON();
      const callout = findNode(json, 'callout');
      expect(callout?.attrs?.calloutType).toBe('TIP');
      expect(callout?.attrs?.title).toBe('Pro tip');
    });

    it('supports all 5 callout types', () => {
      const types: CalloutType[] = ['NOTE', 'TIP', 'WARNING', 'DANGER', 'INFO'];
      for (const t of types) {
        const ed = createTestEditor();
        ed.commands.insertCallout(t);
        const json = ed.getJSON();
        const callout = findNode(json, 'callout');
        expect(callout?.attrs?.calloutType, `${t} callout should be created`).toBe(t);
        ed.destroy();
      }
    });
  });

  describe('markdown serialization', () => {
    it('serializes a NOTE callout without title', () => {
      const editor = createTestEditor();
      editor.commands.insertCallout('NOTE');
      const md = editor.storage.markdown.getMarkdown() as string;
      expect(md).toContain('[!NOTE]');
    });

    it('serializes a WARNING callout with title', () => {
      const editor = createTestEditor();
      editor.commands.insertCallout('WARNING', 'Heads up');
      const md = editor.storage.markdown.getMarkdown() as string;
      expect(md).toContain('[!WARNING]');
      expect(md).toContain('Heads up');
    });

    it('each line of body is prefixed with >', () => {
      const editor = createTestEditor();
      editor.commands.insertCallout('TIP');
      const md = editor.storage.markdown.getMarkdown() as string;
      const lines = md.split('\n').filter((l) => l.length > 0);
      for (const line of lines) {
        expect(line.startsWith('>')).toBe(true);
      }
    });
  });
});

// ---- helpers ----

interface JSONNode {
  type?: string;
  attrs?: Record<string, unknown>;
  content?: JSONNode[];
}

function findNode(doc: JSONNode, typeName: string): JSONNode | undefined {
  if (doc.type === typeName) return doc;
  if (!doc.content) return undefined;
  for (const child of doc.content) {
    const found = findNode(child, typeName);
    if (found) return found;
  }
  return undefined;
}
