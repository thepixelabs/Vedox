/**
 * SlashCommand.ts
 *
 * Tiptap extension that triggers a slash-command popover on `/`.
 *
 * Approach: we use a plain ProseMirror plugin listening for transactions
 * and dispatching custom DOM events when `/` appears at the start of an
 * empty block. SlashCommandPopover.svelte listens for the events and
 * renders the UI.
 *
 * Events dispatched on window:
 *   - vedox-slash-open  { query, items, coords, onSelect, onClose }
 *   - vedox-slash-update { query, items }
 *   - vedox-slash-close
 *   - vedox-slash-nav { key, accept() }
 */

import { Extension, type Editor } from '@tiptap/core';
import { Plugin, PluginKey } from '@tiptap/pm/state';
import type { EditorView } from '@tiptap/pm/view';
import { filterCommands, type SlashCommand as SlashCommandItem } from '../slash-commands/registry.js';

export const SlashCommandPluginKey = new PluginKey('slashCommand');

interface SlashState {
  active: boolean;
  query: string;
  from: number;
  to: number;
}

export interface SlashCommandEventDetail {
  query: string;
  items: SlashCommandItem[];
  coords: { top: number; left: number };
  onSelect: (cmd: SlashCommandItem) => void;
  onClose: () => void;
}

function openPopover(
  view: EditorView,
  editor: Editor,
  pos: number,
  query: string
): void {
  const coords = view.coordsAtPos(pos);
  const detail: SlashCommandEventDetail = {
    query,
    items: filterCommands(query),
    coords: { top: coords.bottom + 4, left: coords.left },
    onSelect: (cmd) => {
      const state = SlashCommandPluginKey.getState(view.state) as SlashState;
      if (state.active) {
        view.dispatch(
          view.state.tr
            .delete(state.from, state.to)
            .setMeta(SlashCommandPluginKey, {
              active: false,
              query: '',
              from: 0,
              to: 0
            } as SlashState)
        );
      }
      cmd.action(editor);
      closePopover();
    },
    onClose: () => {
      const state = SlashCommandPluginKey.getState(view.state) as SlashState;
      if (state.active) {
        view.dispatch(
          view.state.tr.setMeta(SlashCommandPluginKey, {
            active: false,
            query: '',
            from: 0,
            to: 0
          } as SlashState)
        );
      }
      closePopover();
    }
  };
  window.dispatchEvent(new CustomEvent('vedox-slash-open', { detail }));
}

function updatePopover(query: string): void {
  window.dispatchEvent(
    new CustomEvent('vedox-slash-update', {
      detail: { query, items: filterCommands(query) }
    })
  );
}

function closePopover(): void {
  window.dispatchEvent(new CustomEvent('vedox-slash-close'));
}

function dispatchNavEvent(key: string): boolean {
  let handled = false;
  window.dispatchEvent(
    new CustomEvent('vedox-slash-nav', {
      detail: {
        key,
        accept: () => {
          handled = true;
        }
      }
    })
  );
  return handled;
}

export const SlashCommand = Extension.create({
  name: 'slashCommand',

  addProseMirrorPlugins() {
    const editor = this.editor;

    return [
      new Plugin<SlashState>({
        key: SlashCommandPluginKey,

        state: {
          init(): SlashState {
            return { active: false, query: '', from: 0, to: 0 };
          },
          apply(tr, prev): SlashState {
            const meta = tr.getMeta(SlashCommandPluginKey);
            if (meta) return meta as SlashState;

            if (!prev.active) return prev;

            const { from } = tr.selection;
            if (from < prev.from) {
              return { active: false, query: '', from: 0, to: 0 };
            }

            const textBetween = tr.doc.textBetween(prev.from, from, '\n', '\0');
            if (!textBetween.startsWith('/')) {
              return { active: false, query: '', from: 0, to: 0 };
            }
            const query = textBetween.slice(1);
            if (/\s/.test(query)) {
              return { active: false, query: '', from: 0, to: 0 };
            }
            return { active: true, query, from: prev.from, to: from };
          }
        },

        props: {
          handleKeyDown(view: EditorView, event: KeyboardEvent): boolean {
            const state = SlashCommandPluginKey.getState(view.state) as SlashState;

            if (event.key === '/') {
              const { $from, empty } = view.state.selection;
              if (!empty) return false;
              if ($from.parentOffset !== 0) return false;
              if ($from.parent.type.name !== 'paragraph') return false;

              // Defer activation until after the "/" character is inserted
              setTimeout(() => {
                const pos = view.state.selection.from - 1; // position of the "/"
                const tr = view.state.tr.setMeta(SlashCommandPluginKey, {
                  active: true,
                  query: '',
                  from: pos,
                  to: pos + 1
                } as SlashState);
                view.dispatch(tr);
              }, 0);
              return false;
            }

            if (!state.active) return false;

            if (
              event.key === 'ArrowDown' ||
              event.key === 'ArrowUp' ||
              event.key === 'Enter' ||
              event.key === 'Escape'
            ) {
              const handled = dispatchNavEvent(event.key);
              if (handled) {
                event.preventDefault();
                return true;
              }
            }

            return false;
          }
        },

        view(_editorView: EditorView) {
          let lastActive = false;
          let lastQuery = '';
          return {
            update(view: EditorView) {
              const state = SlashCommandPluginKey.getState(view.state) as SlashState;
              if (state.active && !lastActive) {
                openPopover(view, editor, state.from, state.query);
                lastQuery = state.query;
              } else if (!state.active && lastActive) {
                closePopover();
              } else if (state.active && lastActive && state.query !== lastQuery) {
                updatePopover(state.query);
                lastQuery = state.query;
              }
              lastActive = state.active;
            },
            destroy() {
              closePopover();
            }
          };
        }
      })
    ];
  }
});
