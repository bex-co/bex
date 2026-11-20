import {
  useCallback,
  useEffect,
  useEffectEvent,
  useMemo,
  useRef,
  useState,
  type Dispatch,
  type SetStateAction,
} from "react";
import { useCopyToClipboard } from "@/common/hooks/use-copy-to-clipboard";
import { useTranslations } from "@/common/hooks/use-translations";
import type { LogLineListHandle } from "../components/log-line-list";
import type { LogOrder } from "../lib/log-rows";

/** What one log explorer's toolbar, options and shortcuts drive (w3/m509). */
export interface LogExplorer {
  order: LogOrder;
  setOrder: (order: LogOrder) => void;
  expandAll: boolean;
  setExpandAll: (expandAll: boolean) => void;
  maximized: boolean;
  setMaximized: Dispatch<SetStateAction<boolean>>;
  /** Options ▸ Copy logs: every line shown, to the clipboard. */
  copy: () => void;
}

// Where a key belongs to something else: a menu, list box, dialog, tab list,
// slider or radio group moves its own focus with these keys.
const OWNS_ITS_KEYS =
  '[role="menu"], [role="menubar"], [role="listbox"], [role="dialog"], [role="alertdialog"], [role="tablist"], [role="slider"], [role="radiogroup"], [role="grid"]';
// Where a key is typed text.
const TYPING =
  'input, textarea, select, [contenteditable]:not([contenteditable="false"]), [role="textbox"], [role="combobox"], [role="searchbox"]';
// The keys the explorer answers, lowercased.
const SHORTCUT_KEYS = new Set(["/", "m", "home", "end", "escape", "c", "l"]);
// A modal open over the page, wherever focus has gone.
const OPEN_MODAL =
  '[role="dialog"][data-state="open"], [role="alertdialog"][data-state="open"]';

/**
 * One log explorer's display options and Render's keyboard shortcuts
 * (render.com/docs/logging): `/` focuses search, `M` toggles full screen and
 * `Esc` leaves it, `Home`/`End` jump to the list's top and bottom,
 * `⌘/Ctrl ⇧ C` copies every line shown, and `Shift+L` clears a live tail's
 * lines (`clear` is only given while the tail is live). No shortcut fires
 * while the reader types in a field, nor while a menu, dialog or other
 * widget that moves its own focus has it; `Esc` alone still leaves full
 * screen from a field.
 */
export function useLogExplorer({ clear }: { clear?: () => void } = {}): {
  explorer: LogExplorer;
  /** Attach the explorer's line list (LogLineList's `handleRef`), for
   *  Home/End and Copy logs. */
  attachList: (handle: LogLineListHandle | null) => void;
  /** Attach the explorer's search box (its `ref`), which `/` focuses. */
  attachSearch: (input: HTMLInputElement | null) => void;
} {
  const { t } = useTranslations();
  const [order, setOrder] = useState<LogOrder>("asc");
  const [expandAll, setExpandAll] = useState(false);
  const [maximized, setMaximized] = useState(false);
  // Callback refs, not ref objects, go to the caller, and beside the
  // explorer rather than in it: an object carrying a ref reads as one to the
  // compiler, and every field of it with it.
  const listRef = useRef<LogLineListHandle | null>(null);
  const searchRef = useRef<HTMLInputElement | null>(null);
  const attachList = useCallback((handle: LogLineListHandle | null) => {
    listRef.current = handle;
  }, []);
  const attachSearch = useCallback((input: HTMLInputElement | null) => {
    searchRef.current = input;
  }, []);
  const { copy: write } = useCopyToClipboard({
    successText: t("logs.copied"),
    errorText: t("logs.copyFailed"),
  });
  const copy = useCallback(() => {
    const text = listRef.current?.text();
    if (text) void write(text);
  }, [write]);

  // The latest state and callbacks, read by one listener per explorer.
  const onKeyDown = useEffectEvent((event: KeyboardEvent) => {
    const key = event.key.toLowerCase();
    // Every other key, typing included, is not the explorer's.
    if (!SHORTCUT_KEYS.has(key)) return;
    if (event.defaultPrevented || event.isComposing) return;
    const target = event.target instanceof Element ? event.target : null;
    if (target?.closest(OWNS_ITS_KEYS) || document.querySelector(OPEN_MODAL))
      return;
    const modified = event.metaKey || event.ctrlKey || event.altKey;
    if (key === "escape") {
      if (!modified && maximized) {
        event.preventDefault();
        setMaximized(false);
      }
      return;
    }
    if (target?.closest(TYPING)) return;
    if ((event.metaKey || event.ctrlKey) && event.shiftKey && key === "c") {
      if (event.altKey) return;
      event.preventDefault();
      copy();
      return;
    }
    if (modified) return;
    if (key === "/") {
      event.preventDefault();
      searchRef.current?.focus();
    } else if (key === "m") {
      event.preventDefault();
      setMaximized((value) => !value);
    } else if (key === "home" || key === "end") {
      if (!listRef.current) return;
      event.preventDefault();
      listRef.current.jump(key === "home" ? "top" : "bottom");
    } else if (key === "l" && event.shiftKey && clear) {
      event.preventDefault();
      clear();
    }
  });
  useEffect(() => {
    const listener = (event: KeyboardEvent) => onKeyDown(event);
    window.addEventListener("keydown", listener);
    return () => window.removeEventListener("keydown", listener);
  }, []);

  const explorer = useMemo(
    () => ({
      order,
      setOrder,
      expandAll,
      setExpandAll,
      maximized,
      setMaximized,
      copy,
    }),
    [order, expandAll, maximized, copy],
  );
  return { explorer, attachList, attachSearch };
}

/**
 * Live tail's `Shift+L` (Render: "Clear logs (live tail view only)"): hides
 * every line shown so far, so only lines newer than the newest one shown
 * then appear. `restore`, pausing the tail or changing the filters
 * (`readerKey`) ends it, and the lines come back.
 */
export function useClearedLines<T extends { timestamp: string }>(
  lines: T[],
  live: boolean,
  readerKey: string,
) {
  const [cut, setCut] = useState<{ at: number; reader: string } | null>(null);
  const active = cut !== null && live && cut.reader === readerKey;
  if (cut && !active) setCut(null);
  const at = active ? cut.at : null;
  // The lines are in time order: the ones after the cut are at the end.
  const shown = useMemo(() => {
    if (at === null) return lines;
    let start = lines.length;
    while (start > 0 && !(Date.parse(lines[start - 1].timestamp) <= at)) {
      start -= 1;
    }
    return lines.slice(start);
  }, [lines, at]);
  const clear = useCallback(() => {
    let newest = -Infinity;
    for (const line of lines) {
      const time = Date.parse(line.timestamp);
      if (time > newest) newest = time;
    }
    if (newest > -Infinity) setCut({ at: newest, reader: readerKey });
  }, [lines, readerKey]);
  const restore = useCallback(() => setCut(null), []);
  // Only a live tail clears: the explorer offers `Shift+L` while this is set.
  return {
    lines: shown,
    cleared: active,
    clear: live ? clear : undefined,
    restore,
  };
}
