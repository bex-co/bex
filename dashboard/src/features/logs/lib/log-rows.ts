import { startOfDay } from "date-fns";
import type { AnsiSpan } from "./ansi";
import { LOG_TYPE_REQUEST, type LogLine } from "../types";

/** The order a log explorer shows its lines in: Render's Options ▸ Order by
 *  (w3/m509). Ascending is the newest line at the bottom. */
export type LogOrder = "asc" | "desc";

/** One row of a log list: a line, or the date separator over a new day. */
export type LogListRow =
  | { kind: "line"; key: string; line: LogLine }
  | {
      kind: "day";
      key: string;
      /** The day's local midnight, in epoch milliseconds. */
      day: number;
    };

/** The local day a timestamp falls on (its midnight), or null for none. */
function dayOf(timestamp: string): number | null {
  if (!timestamp) return null;
  const date = new Date(timestamp);
  return Number.isNaN(date.getTime()) ? null : startOfDay(date).getTime();
}

/** The day of the first line from `from` in `step`'s direction that has a
 *  time, or null for none. */
function edgeDay(lines: readonly LogLine[], from: number, step: 1 | -1) {
  for (let i = from; i >= 0 && i < lines.length; i += step) {
    const day = dayOf(lines[i].timestamp);
    if (day !== null) return day;
  }
  return null;
}

/**
 * The rows a log list draws, in the order asked for. A list whose lines span
 * more than one day gets Render's date separator (`Nov 19`) over each day's
 * first line, so every day reads by its date; one day's lines need none, and
 * a line without a time joins the day before it. The lines arrive oldest
 * first, in time order, so the first and last days say whether the list
 * spans several without reading every line; descending reverses them, and
 * each day's separator still leads it.
 */
export function logListRows(
  lines: readonly LogLine[],
  order: LogOrder,
): LogListRow[] {
  const ordered = order === "desc" ? [...lines].reverse() : lines;
  if (edgeDay(lines, 0, 1) === edgeDay(lines, lines.length - 1, -1)) {
    return ordered.map((line) => ({ kind: "line", key: line.key, line }));
  }
  const days = ordered.map((line) => dayOf(line.timestamp));
  const rows: LogListRow[] = [];
  // A separator's key is its day, so it survives pages arriving around it.
  const seen = new Map<number, number>();
  let current: number | null = null;
  ordered.forEach((line, index) => {
    const day = days[index];
    if (day !== null && day !== current) {
      const repeat = seen.get(day) ?? 0;
      seen.set(day, repeat + 1);
      rows.push({
        kind: "day",
        key: repeat === 0 ? `day:${day}` : `day:${day}:${repeat}`,
        day,
      });
      current = day;
    }
    rows.push({ kind: "line", key: line.key, line });
  });
  return rows;
}

/** A long message is clamped past this many lines or characters, until the
 *  row or Options ▸ Expand everything shows it whole (w3/m509). */
export const CLAMP_LINES = 10;
export const CLAMP_CHARS = 1000;

/** What a row shows of its message: the text, as its spans when styled. */
function shownMessage(line: Pick<LogLine, "message" | "spans">) {
  return line.spans
    ? line.spans.map((span) => span.text).join("")
    : line.message;
}

/**
 * A message's first `CLAMP_LINES` lines or `CLAMP_CHARS` characters, as text
 * and as its styled spans, or null when the whole message is within both.
 */
export function clampMessage(
  line: Pick<LogLine, "message" | "spans">,
): { text: string; spans: AnsiSpan[] | null } | null {
  // The raw message is never shorter, nor has fewer lines, than what the row
  // shows of it: one within both limits needs no clamp, and no join of its
  // spans to say so.
  if (!exceedsClamp(line.message)) return null;
  const shown = shownMessage(line);
  let cut = Math.min(shown.length, CLAMP_CHARS);
  let breaks = 0;
  for (let i = 0; i < cut; i += 1) {
    if (shown.charCodeAt(i) === 10 && ++breaks === CLAMP_LINES) {
      cut = i;
      break;
    }
  }
  if (cut >= shown.length) return null;
  const text = shown.slice(0, cut);
  if (!line.spans) return { text, spans: null };
  const spans: AnsiSpan[] = [];
  let left = cut;
  for (const span of line.spans) {
    if (left <= 0) break;
    spans.push(
      span.text.length <= left
        ? span
        : { ...span, text: span.text.slice(0, left) },
    );
    left -= span.text.length;
  }
  return { text, spans };
}

function exceedsClamp(text: string): boolean {
  if (text.length > CLAMP_CHARS) return true;
  let breaks = 0;
  for (let i = 0; i < text.length; i += 1) {
    if (text.charCodeAt(i) === 10 && ++breaks === CLAMP_LINES) return true;
  }
  return false;
}

/**
 * Public instance ids are `<service-id>-<opaque>` (20-char base32-hex
 * suffix). A row shows a compact suffix when present, otherwise the full id;
 * filtering always uses the full value from the line.
 */
export function shortInstance(instance: string): string {
  const suffix = instance.split("-").at(-1) ?? instance;
  if (/^[0-9a-v]{20}$/.test(suffix)) {
    return suffix.slice(0, 5);
  }
  return suffix.length === 5 ? suffix : instance;
}

/** How the copied text names what a row shows beside its message. */
export interface LogRowsTextFormat {
  /** The row's clock, or null when the list hides timestamps. */
  time: ((timestamp: string) => string) | null;
  day: (day: number) => string;
  /** A project view's resource name for a line, when the list names them. */
  resource?: (id: string) => string | undefined;
}

/**
 * The rows as Options ▸ Copy logs writes them (Render's "Copy all currently
 * displayed logs", `⌘/Ctrl ⇧ C`): one text line per row, in the order shown,
 * as the row reads — date separator, time, `[resource]`, `[instance]`, a
 * request's method, status, host and path, then the whole message without
 * its terminal styling, however much of it the row has clamped.
 */
export function logRowsText(
  rows: readonly LogListRow[],
  format: LogRowsTextFormat,
): string {
  return rows
    .map((row) => {
      if (row.kind === "day") return format.day(row.day);
      const { line } = row;
      const parts: string[] = [];
      const time = format.time?.(line.timestamp);
      if (time) parts.push(time);
      if (format.resource && line.resource) {
        parts.push(`[${format.resource(line.resource) ?? line.resource}]`);
      }
      if (line.instance) parts.push(`[${shortInstance(line.instance)}]`);
      parts.push(...lineBody(line));
      return parts.join(" ");
    })
    .join("\n");
}

/** A line's request method, status, host and path, then its whole message
 *  without its terminal styling: what both copies write after the tags. */
function lineBody(line: LogLine): string[] {
  const parts: string[] = [];
  if (line.type === LOG_TYPE_REQUEST) {
    if (line.method) parts.push(line.method);
    if (line.statusCode) parts.push(line.statusCode);
    if (line.host || line.path) parts.push(`${line.host}${line.path}`);
  }
  parts.push(shownMessage(line));
  return parts;
}

/**
 * One line as the row menu's Copy log line writes it (Render's `En`, w3/m528):
 * `[resource] METHOD STATUS host+path`, then the message without its terminal
 * styling, with the parts the line has. No time and no instance: Render's
 * label is a project view's resource name, absent on one resource's log.
 */
export function logLineText(line: LogLine, resource?: string): string {
  return [resource ? `[${resource}]` : "", ...lineBody(line)]
    .filter(Boolean)
    .join(" ");
}

/** The held-lines button's count for `count` held lines (Render's
 *  `BufferedLogsButton`, w3/m528): null below 10, which reads `New logs`,
 *  else the count rounded up to a multiple of 5, at most 100. */
export function heldLinesCount(count: number): number | null {
  return count < 10 ? null : Math.ceil(Math.min(count, 100) / 5) * 5;
}
