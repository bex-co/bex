import { LOG_TYPE_REQUEST, type LogLine } from "../types";

/**
 * The mark a log row's level gutter draws (w3/m509). Render documents eight
 * levels (`debug`, `info`, `notice`, `warning`, `error`, `critical`, `alert`,
 * `emergency`, render.com/docs/logging); the gutter folds the last four into
 * the error mark and tint, as their rows read the same at a glance.
 */
export type LineLevel = "debug" | "info" | "notice" | "warning" | "error";

// The level labels a line can carry: Render's eight, and the spellings
// application loggers and the shipper's buckets use for the same levels.
const LEVELS: Record<string, LineLevel> = {
  trace: "debug",
  debug: "debug",
  info: "info",
  information: "info",
  informational: "info",
  notice: "notice",
  warn: "warning",
  warning: "warning",
  error: "error",
  err: "error",
  fatal: "error",
  panic: "error",
  critical: "error",
  crit: "error",
  alert: "error",
  emergency: "error",
  emerg: "error",
};

/**
 * A line's level mark, or null for a line that names none. A request line
 * stored before its `level` label existed (w4/584) takes the level Render
 * gives its status class: `error` for 5xx, `warning` for 4xx, `info`
 * otherwise (docs/render-artifacts/request-logs.md).
 */
export function lineLevel(
  line: Pick<LogLine, "level" | "type" | "statusCode">,
): LineLevel | null {
  const named = LEVELS[line.level.toLowerCase()];
  if (named) return named;
  if (line.type !== LOG_TYPE_REQUEST || !/^[1-5]/.test(line.statusCode)) {
    return null;
  }
  const status = line.statusCode.charAt(0);
  return status === "5" ? "error" : status === "4" ? "warning" : "info";
}

/** The word a level mark's hover shows: the line's own label when it has
 *  one (`critical`, not the error mark it shares), else the mark's. */
export function lineLevelText(
  line: Pick<LogLine, "level">,
  level: LineLevel,
): string {
  return line.level ? line.level.toLowerCase() : level;
}
