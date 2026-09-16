import * as React from "react";

import { cn } from "@/common/lib/utils/utils.ts";

/**
 * A one-row textarea that grows with its content, shaped to look exactly like
 * `Input` while single-line.
 *
 * Env values are the reason it exists (w2/m95 t002): every value field used to
 * be an `<input>`, and the browser flattens a pasted line break into a space
 * before the value ever reaches React state — so a pasted PEM key or JSON blob
 * was silently saved changed. A textarea keeps the newlines, and growing to fit
 * is what makes a multi-line value readable instead of a one-line smear.
 *
 * Masking uses `-webkit-text-security`, not `type="password"` (a textarea has no
 * type). It is supported by Chromium, WebKit and Firefox 137+; the value is
 * still selectable and copyable, exactly as a password input's is not — callers
 * that need true secrecy should not render the value at all.
 */
export const AUTO_TEXTAREA_MAX_ROWS = 10;

export function AutoTextarea({
  className,
  masked = false,
  maxRows = AUTO_TEXTAREA_MAX_ROWS,
  value,
  onChange,
  ...props
}: Omit<React.ComponentProps<"textarea">, "rows"> & {
  /** Render the value as dots, the closest a textarea gets to `type="password"`. */
  masked?: boolean;
  /** Rows to grow to before the field starts scrolling instead. */
  maxRows?: number;
}) {
  const ref = React.useRef<HTMLTextAreaElement>(null);

  const fit = React.useCallback(() => {
    const element = ref.current;
    if (!element) return;
    // Collapse first so shrinking is measured, not just growing.
    element.style.height = "auto";
    const content = element.scrollHeight;
    if (!content) {
      // jsdom (and any layout-less environment) reports 0 — leave the element
      // alone rather than pinning it to a zero height.
      element.style.height = "";
      return;
    }
    const styles = window.getComputedStyle(element);
    const lineHeight = Number.parseFloat(styles.lineHeight);
    const chrome =
      Number.parseFloat(styles.paddingTop) +
      Number.parseFloat(styles.paddingBottom) +
      Number.parseFloat(styles.borderTopWidth) +
      Number.parseFloat(styles.borderBottomWidth);
    const ceiling =
      Number.isFinite(lineHeight) && Number.isFinite(chrome)
        ? lineHeight * maxRows + chrome
        : Number.POSITIVE_INFINITY;
    element.style.height = `${Math.min(content, ceiling)}px`;
    element.style.overflowY = content > ceiling ? "auto" : "hidden";
  }, [maxRows]);

  // Re-fit on every value change, including ones the component did not cause
  // (an import that rewrites the draft, an undo, a generated secret).
  React.useLayoutEffect(fit, [fit, value, masked]);

  return (
    <textarea
      {...props}
      ref={ref}
      rows={1}
      value={value}
      onChange={(event) => {
        onChange?.(event);
        fit();
      }}
      data-slot="auto-textarea"
      style={{
        ...props.style,
        ...(masked
          ? ({ WebkitTextSecurity: "disc" } as React.CSSProperties)
          : null),
      }}
      className={cn(
        "placeholder:text-muted-foreground selection:bg-primary selection:text-primary-foreground dark:bg-input/30 border-input flex min-h-9 w-full min-w-0 resize-none rounded-md border bg-transparent px-3 py-1.5 text-base shadow-xs transition-[color,box-shadow] outline-none disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 md:text-sm",
        "focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px]",
        "aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive",
        className,
      )}
    />
  );
}
