import {
  forwardRef,
  useEffect,
  useId,
  useImperativeHandle,
  useState,
} from "react";
import type { Range } from "@tiptap/core";
import Mention from "@tiptap/extension-mention";
import type { MentionOptions } from "@tiptap/extension-mention";
import Document from "@tiptap/extension-document";
import Paragraph from "@tiptap/extension-paragraph";
import Text from "@tiptap/extension-text";
import { Placeholder, UndoRedo } from "@tiptap/extensions";
import { PluginKey, TextSelection } from "@tiptap/pm/state";
import { EditorContent, ReactRenderer, useEditor } from "@tiptap/react";
import type {
  SuggestionKeyDownProps,
  SuggestionOptions,
  SuggestionProps,
} from "@tiptap/suggestion";
import { useTranslations } from "@/common/hooks/use-translations";
import { MentionPicker } from "@/features/agent-sessions/components/mention-picker";
import { AGENT_COMPOSER_FOCUS_EVENT } from "@/features/agent-sessions/lib/composer-focus";
import {
  isRepoMentionId,
  readComposerDocument,
  repoMentionId,
  sessionMentionId,
} from "@/features/agent-sessions/lib/composer-document";
import type { ComposerDocument } from "@/features/agent-sessions/lib/composer-document";
import { sessionTitle } from "@/features/agent-sessions/lib/mapper";
import {
  mentionEmptyText,
  mentionOptions,
  mentionStateFromQuery,
  mentionToken,
} from "@/features/agent-sessions/lib/mention";
import type {
  MentionOption,
  MentionSource,
} from "@/features/agent-sessions/lib/mention";

export interface InlineMentionEditorProps {
  source: MentionSource;
  reposLoading: boolean;
  ariaLabel: string;
  placeholder: string;
  onChange: (document: ComposerDocument) => void;
  /** Enter (without Shift) submits the prompt, chat-composer style. */
  onSubmit: () => void;
}

export interface InlineMentionEditorHandle {
  openMention: () => void;
}

interface MentionMenuHandle {
  onKeyDown: (props: SuggestionKeyDownProps) => boolean;
}

type BaseSuggestionProps = SuggestionProps<MentionOption, MentionOption>;
type MentionMenuProps = BaseSuggestionProps & {
  source: MentionSource;
  reposLoading: boolean;
};

const mentionPluginKey = new PluginKey("agent-composer-mention");

type Translate = ReturnType<typeof useTranslations>["t"];

/** Mutable editor inputs live outside React's render data flow. */
class EditorRuntime {
  constructor(
    public source: MentionSource,
    public reposLoading: boolean,
    public placeholder: string,
    public onChange: (document: ComposerDocument) => void,
    public onSubmit: () => void,
    public t: Translate,
  ) {}

  update(
    source: MentionSource,
    reposLoading: boolean,
    placeholder: string,
    onChange: (document: ComposerDocument) => void,
    onSubmit: () => void,
    t: Translate,
  ) {
    this.source = source;
    this.reposLoading = reposLoading;
    this.placeholder = placeholder;
    this.onChange = onChange;
    this.onSubmit = onSubmit;
    this.t = t;
  }
}

/**
 * Tiptap-backed prompt surface. Repositories and prior sessions are atomic
 * inline nodes, so normal caret movement, selection, undo, and Backspace all
 * come from ProseMirror instead of a parallel chip state machine.
 */
export const InlineMentionEditor = forwardRef<
  InlineMentionEditorHandle,
  InlineMentionEditorProps
>(function InlineMentionEditor(
  { source, reposLoading, ariaLabel, placeholder, onChange, onSubmit },
  ref,
) {
  const { t } = useTranslations();
  const [runtime] = useState(
    () =>
      new EditorRuntime(
        source,
        reposLoading,
        placeholder,
        onChange,
        onSubmit,
        t,
      ),
  );

  useEffect(() => {
    runtime.update(source, reposLoading, placeholder, onChange, onSubmit, t);
  }, [onChange, onSubmit, placeholder, reposLoading, runtime, source, t]);

  const suggestion: Omit<
    SuggestionOptions<MentionOption, MentionOption>,
    "editor"
  > = {
    pluginKey: mentionPluginKey,
    char: "@",
    allowedPrefixes: [" ", "\n"],
    items: ({ query, editor }) => {
      const state = mentionStateFromQuery(query);
      // Sessions surface at both the universal level and the `@sessions:`
      // second level, so drop already-mentioned ones wherever they'd appear —
      // a selected session should never be offered again.
      const selectedSessions = new Set(
        readComposerDocument(editor.getJSON()).sessionIds,
      );
      const availableSource: MentionSource = {
        ...runtime.source,
        sessions: runtime.source.sessions.filter(
          (session) => !selectedSessions.has(session.id),
        ),
      };
      return mentionOptions(state, availableSource, runtime.t);
    },
    command: ({ editor, range, props: option }) => {
      if (option.kind === "category") {
        editor
          .chain()
          .focus()
          .insertContentAt(range, mentionToken(option.category))
          .run();
        return;
      }
      insertMention(editor, range, option);
    },
    render: () => {
      let component:
        | ReactRenderer<MentionMenuHandle, MentionMenuProps>
        | undefined;
      let unmount: (() => void) | undefined;

      const menuProps = (props: BaseSuggestionProps): MentionMenuProps => ({
        ...props,
        source: runtime.source,
        reposLoading: runtime.reposLoading,
      });

      return {
        onStart: (props) => {
          component = new ReactRenderer(MentionSuggestionMenu, {
            editor: props.editor,
            props: menuProps(props),
          });
          unmount = props.mount(component.element);
        },
        onUpdate: (props) => component?.updateProps(menuProps(props)),
        onKeyDown: (props) => component?.ref?.onKeyDown(props) ?? false,
        onExit: () => {
          unmount?.();
          component?.destroy();
          unmount = undefined;
          component = undefined;
        },
      };
    },
  };

  const editor = useEditor(
    {
      extensions: [
        Document,
        Paragraph,
        Text,
        UndoRedo,
        Placeholder.configure({
          placeholder: () => runtime.placeholder,
        }),
        Mention.configure({
          HTMLAttributes: {
            class:
              "bg-muted text-foreground mx-0.5 inline rounded-md px-1.5 py-0.5 font-medium",
          },
          deleteTriggerWithBackspace: true,
          // Mention's exported singleton fixes selected props to node attrs,
          // while Suggestion intentionally supports richer picker rows. The
          // command always converts our row into attrs before insertion.
          suggestion: suggestion as unknown as MentionOptions["suggestion"],
        }),
      ],
      autofocus: true,
      immediatelyRender: false,
      editorProps: {
        attributes: {
          "aria-label": ariaLabel,
          "aria-autocomplete": "list",
          "aria-expanded": "false",
          role: "combobox",
          class:
            "min-h-16 px-3 py-2 text-sm outline-none [&_p]:min-h-6 [&_p.is-editor-empty:first-child]:before:pointer-events-none [&_p.is-editor-empty:first-child]:before:float-left [&_p.is-editor-empty:first-child]:before:h-0 [&_p.is-editor-empty:first-child]:before:text-muted-foreground [&_p.is-editor-empty:first-child]:before:content-[attr(data-placeholder)]",
        },
        // Enter submits; Shift+Enter inserts a newline (chat-composer style).
        // When the @-mention picker is open, defer to its own Enter handling.
        handleKeyDown: (view, event) => {
          if (event.key !== "Enter" || event.shiftKey || event.isComposing) {
            return false;
          }
          const mention = mentionPluginKey.getState(view.state) as
            | { active?: boolean }
            | undefined;
          if (mention?.active) return false;
          event.preventDefault();
          runtime.onSubmit();
          return true;
        },
      },
      onCreate: ({ editor: nextEditor }) => {
        runtime.onChange(readComposerDocument(nextEditor.getJSON()));
      },
      onUpdate: ({ editor: nextEditor }) => {
        runtime.onChange(readComposerDocument(nextEditor.getJSON()));
      },
    },
    [],
  );

  useImperativeHandle(ref, () => ({
    openMention: () => {
      if (!editor) return;
      const { from } = editor.state.selection;
      const previous =
        from > 1
          ? editor.state.doc.textBetween(from - 1, from, "\n", "\uFFFC")
          : "";
      editor
        .chain()
        .focus()
        .insertContent(previous && !/\s/.test(previous) ? " @" : "@")
        .run();
    },
  }));

  useEffect(() => {
    function focusEditor() {
      editor?.commands.focus("end");
    }
    window.addEventListener(AGENT_COMPOSER_FOCUS_EVENT, focusEditor);
    return () =>
      window.removeEventListener(AGENT_COMPOSER_FOCUS_EVENT, focusEditor);
  }, [editor]);

  useEffect(() => {
    if (!editor) return;
    editor.view.dom.setAttribute("aria-label", ariaLabel);
  }, [ariaLabel, editor]);

  return (
    <div className="min-h-16" data-testid="agent-composer-editor">
      <EditorContent editor={editor} />
    </div>
  );
});

/** Insert an item atomically and replace the prior repo mention, if any. */
function insertMention(
  editor: BaseSuggestionProps["editor"],
  range: Range,
  option: Exclude<MentionOption, { kind: "category" }>,
) {
  const { state, view } = editor;
  const id =
    option.kind === "repo"
      ? repoMentionId(option.repo.fullName)
      : sessionMentionId(option.session.id);
  const label =
    option.kind === "repo"
      ? option.repo.fullName
      : sessionTitle(option.session);

  if (
    option.kind === "session" &&
    readComposerDocument(editor.getJSON()).sessionIds.includes(
      option.session.id,
    )
  ) {
    return;
  }

  const mentionType = state.schema.nodes.mention;
  if (!mentionType) return;

  const transaction = state.tr;
  if (option.kind === "repo") {
    const oldRepos: Array<{ position: number; size: number }> = [];
    state.doc.descendants((node, position) => {
      if (node.type === mentionType && isRepoMentionId(node.attrs.id)) {
        oldRepos.push({ position, size: node.nodeSize });
      }
    });
    for (const oldRepo of oldRepos.reverse()) {
      transaction.delete(oldRepo.position, oldRepo.position + oldRepo.size);
    }
  }

  const from = transaction.mapping.map(range.from);
  const to = transaction.mapping.map(range.to);
  const node = mentionType.create({
    id,
    label,
    mentionSuggestionChar: "@",
  });
  transaction.replaceWith(from, to, node);
  transaction.insertText(" ", from + node.nodeSize);
  transaction.setSelection(
    TextSelection.create(transaction.doc, from + node.nodeSize + 1),
  );
  view.dispatch(transaction);
  view.focus();
}

const MentionSuggestionMenu = forwardRef<MentionMenuHandle, MentionMenuProps>(
  function MentionSuggestionMenu(
    { editor, query, items, command, source, reposLoading },
    ref,
  ) {
    const { t } = useTranslations();
    const idBase = useId();
    const [highlightState, setHighlightState] = useState({ query, index: 0 });
    const highlight =
      highlightState.query === query
        ? Math.min(highlightState.index, Math.max(items.length - 1, 0))
        : 0;
    const state = mentionStateFromQuery(query);

    function setHighlight(index: number) {
      setHighlightState({ query, index });
    }

    useEffect(() => {
      const element = editor.view.dom;
      element.setAttribute("aria-expanded", "true");
      element.setAttribute("aria-controls", idBase);
      return () => {
        element.setAttribute("aria-expanded", "false");
        element.removeAttribute("aria-controls");
        element.removeAttribute("aria-activedescendant");
      };
    }, [editor, idBase]);

    useEffect(() => {
      const element = editor.view.dom;
      if (items.length === 0) {
        element.removeAttribute("aria-activedescendant");
        return;
      }
      element.setAttribute(
        "aria-activedescendant",
        `${idBase}-option-${highlight}`,
      );
    }, [editor, highlight, idBase, items.length]);

    useImperativeHandle(ref, () => ({
      onKeyDown: ({ event }) => {
        if (event.key === "ArrowUp") {
          setHighlight(
            items.length === 0
              ? 0
              : (highlight - 1 + items.length) % items.length,
          );
          return true;
        }
        if (event.key === "ArrowDown") {
          setHighlight(items.length === 0 ? 0 : (highlight + 1) % items.length);
          return true;
        }
        if (event.key === "Enter") {
          const option = items[highlight];
          if (option) command(option);
          return true;
        }
        return false;
      },
    }));

    return (
      <MentionPicker
        idBase={idBase}
        options={items}
        highlight={highlight}
        emptyText={t(mentionEmptyText(state, source, reposLoading))}
        onHighlight={setHighlight}
        onSelect={command}
      />
    );
  },
);
