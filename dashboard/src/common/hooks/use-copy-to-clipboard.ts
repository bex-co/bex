import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";

export interface UseCopyToClipboardOptions {
  /** Toast shown on a successful copy. */
  successText: string;
  /** Toast shown when the clipboard write fails. */
  errorText: string;
}

export interface UseCopyToClipboardResult {
  /** True for ~2s after a successful copy (drives a Copy→Check icon swap). */
  copied: boolean;
  /** Write a value to the clipboard, toasting the outcome. */
  copy: (value: string) => Promise<void>;
}

/**
 * Copy-to-clipboard with a 2s "copied" tick and a toast — the shared mechanism
 * behind the `CodeBlock` copy button and the Connections panel's `CopyButton`,
 * so the tick window / cleanup / toast behavior can't drift between them. The
 * toast text is passed in so each call site keeps its own (i18n or literal) copy.
 */
export function useCopyToClipboard({
  successText,
  errorText,
}: UseCopyToClipboardOptions): UseCopyToClipboardResult {
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
    },
    [],
  );

  const copy = useCallback(
    async (value: string) => {
      try {
        await navigator.clipboard.writeText(value);
        setCopied(true);
        toast.success(successText);
        if (timer.current) clearTimeout(timer.current);
        timer.current = setTimeout(() => setCopied(false), 2000);
      } catch {
        toast.error(errorText);
      }
    },
    [successText, errorText],
  );

  return { copied, copy };
}
