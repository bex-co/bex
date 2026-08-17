import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
} from "react";
import type {
  InlineMentionEditorHandle,
  InlineMentionEditorProps,
} from "@/features/agent-sessions/components/inline-mention-editor";

export type { InlineMentionEditorHandle, InlineMentionEditorProps };

// The client-only entry point for the Tiptap-backed prompt editor. The whole
// @tiptap stack (~100 KB gzip) lives in `inline-mention-editor`, which this
// wrapper loads via a dynamic import in an effect — the session-conversation /
// web-shell-terminal precedent. On the server (and the first client render) it
// renders only a min-height placeholder, so neither the SSR bundle nor the
// /agents route chunk pays for Tiptap until the combined page's composer
// actually mounts in the browser.

type InlineMentionEditorImpl =
  typeof import("@/features/agent-sessions/components/inline-mention-editor").InlineMentionEditor;

export const InlineMentionEditor = forwardRef<
  InlineMentionEditorHandle,
  InlineMentionEditorProps
>(function InlineMentionEditor(props, ref) {
  const [Impl, setImpl] = useState<InlineMentionEditorImpl | null>(null);
  const innerRef = useRef<InlineMentionEditorHandle | null>(null);

  useEffect(() => {
    let live = true;
    void import("@/features/agent-sessions/components/inline-mention-editor").then(
      (module) => {
        if (live) setImpl(() => module.InlineMentionEditor);
      },
    );
    return () => {
      live = false;
    };
  }, []);

  // The composer's @ toolbar button can fire before the impl chunk resolves;
  // bridge the handle so the call lands on the real editor once it has.
  useImperativeHandle(ref, () => ({
    openMention: () => innerRef.current?.openMention(),
  }));

  if (!Impl) {
    // Same box metrics (min-h-16 + px-3 py-2 text-sm) and muted placeholder as
    // the impl's editorProps, so the SSR/first-render swap is layout-stable.
    return (
      <div
        className="text-muted-foreground min-h-16 px-3 py-2 text-sm"
        data-testid="agent-composer-editor"
      >
        {props.placeholder}
      </div>
    );
  }

  return <Impl ref={innerRef} {...props} />;
});
