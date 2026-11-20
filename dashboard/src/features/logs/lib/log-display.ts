import type { LineLevel } from "./log-levels";
import { createLocalSetting } from "./local-setting";

/**
 * Options ▸ Display, Render's three ways to show a log's lines (its
 * truncation mode, w3/088): every line whole (the default), only the error
 * levels whole with every other line on one, or every line on one. A line
 * on one line is cut off at the panel's edge with an ellipsis.
 */
export type LogDisplay = "expand-all" | "expand-errors" | "collapse-all";

/** The choices in the order Render's Display submenu lists them. */
export const LOG_DISPLAYS: readonly LogDisplay[] = [
  "expand-all",
  "expand-errors",
  "collapse-all",
];

function parseLogDisplay(raw: string | null | undefined): LogDisplay {
  return raw === "expand-errors" || raw === "collapse-all" ? raw : "expand-all";
}

/**
 * The reader's Display choice, one for every explorer and kept across visits
 * in `localStorage`, as Render keeps its own (`render-log-truncation-mode`).
 */
export const logDisplaySetting = createLocalSetting<LogDisplay>(
  "bex.logs.display",
  "expand-all",
  parseLogDisplay,
);

/**
 * Whether a line at `level` shows on one line. Under Expand errors only
 * Render's error levels (`error`, `critical`, `alert`, `emergency`, which the
 * level gutter folds into `error`) show whole.
 */
export function collapsesLine(
  display: LogDisplay,
  level: LineLevel | null,
): boolean {
  return (
    display === "collapse-all" ||
    (display === "expand-errors" && level !== "error")
  );
}
