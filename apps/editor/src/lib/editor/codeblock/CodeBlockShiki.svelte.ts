/**
 * CodeBlockShiki.svelte.ts — Tiptap code block extension backed by Shiki.
 *
 * This extension replaces the default @tiptap/starter-kit CodeBlock with a
 * node view that renders through CodeBlockView.svelte. The node schema is
 * identical to Tiptap's built-in code_block so the markdown round-trip that
 * tiptap-markdown expects still works:
 *
 *   ```<lang>
 *   <code>
 *   ```
 *
 *   ↔ { type: 'codeBlock', attrs: { language }, content: [{ type: 'text', text }] }
 *
 * Why a `.svelte.ts` extension
 * ----------------------------
 * Svelte 5's `$state` rune is a compiler primitive — it only exists inside
 * `.svelte` and `.svelte.ts` / `.svelte.js` modules. We need a reactive
 * props bag so that patching `props.code` triggers a re-render inside the
 * mounted component. If we used a plain `.ts` file the rune would not be
 * available.
 */

import CodeBlock from '@tiptap/extension-code-block';
import type { Node as ProseMirrorNode } from '@tiptap/pm/model';
import type { NodeView } from '@tiptap/pm/view';
import { mount, unmount } from 'svelte';
import CodeBlockView from './CodeBlockView.svelte';

/**
 * Tiptap extension that swaps the default code block for a Shiki-rendered
 * Svelte node view. Accepts the same options as @tiptap/extension-code-block.
 */
export const CodeBlockShiki = CodeBlock.extend({
  name: 'codeBlock',

  addOptions() {
    return {
      ...this.parent?.(),
      exitOnTripleEnter: false,
      exitOnArrowDown: true,
      HTMLAttributes: {
        class: 'code-block',
      },
    };
  },

  addNodeView() {
    return ({ node }: { node: ProseMirrorNode }): NodeView => {
      // Outer wrapper owned by ProseMirror. Svelte mounts *inside* this dom.
      const dom = document.createElement('div');
      dom.setAttribute('data-code-block', '');
      dom.classList.add('tiptap-code-block-host');

      // Reactive props proxy. Each field is a separate `$state` cell so that
      // patching it from the ProseMirror update() callback triggers a
      // component re-render without a full remount.
      const props = $state({
        code: node.textContent,
        language: (node.attrs.language as string | null) ?? 'text',
      });

      const view = mount(CodeBlockView, {
        target: dom,
        props,
      });

      return {
        dom,

        update(updatedNode: ProseMirrorNode): boolean {
          if (updatedNode.type.name !== 'codeBlock') return false;
          // Patch in place so the Svelte view re-renders without a full remount.
          props.code = updatedNode.textContent;
          props.language = (updatedNode.attrs.language as string | null) ?? 'text';
          return true;
        },

        destroy(): void {
          // Svelte 5 unmount cleans up the child DOM tree for us.
          void unmount(view);
        },

        // The code block is a leaf-ish island for interaction purposes:
        // ProseMirror handles cursor placement around the block, but clicks
        // inside the Svelte view (copy button, text selection on the shiki
        // output) should not be treated as editor events.
        stopEvent(): boolean {
          return true;
        },

        ignoreMutation(): boolean {
          // Shiki rewrites our DOM on every re-render; ProseMirror should ignore it.
          return true;
        },
      };
    };
  },
});
