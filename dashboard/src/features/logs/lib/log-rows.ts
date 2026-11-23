import { startOfDay } from "date-fns";
import { LOG_TYPE_REQUEST, type LogLine } from "../types";

/** The order a log explorer shows its lines in: Render's Options ▸ Order by
 *  (w3/m509). Ascending is the newest line at the bottom. */
export type LogOrder = "asc" | "desc";

/** One row of a log list: a line, or the date separator over a new day. */
export type LogListRow =
  | {
      kind: "line";
      key: string;
      line: LogLine;
      /** The time of the line before it in time order, whichever order the
       *  list shows: Render's From previous (w4/716). Empty for the first. */
      previous: string;
    }
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
  const lineRow = (index: number): LogListRow => ({
    kind: "line",
    key: lines[index].key,
    line: lines[index],
    previous: index > 0 ? lines[index - 1].timestamp : "",
  });
  const indices = lines.map((_, index) => index);
  if (order === "desc") indices.reverse();
  if (edgeDay(lines, 0, 1) === edgeDay(lines, lines.length - 1, -1)) {
    return indices.map(lineRow);
  }
  const rows: LogListRow[] = [];
  // A separator's key is its day, so it survives pages arriving around it.
  const seen = new Map<number, number>();
  let current: number | null = null;
  for (const index of indices) {
    const day = dayOf(lines[index].timestamp);
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
    rows.push(lineRow(index));
  }
  return rows;
}

/** What a row shows of its message: the text, as its spans when styled. */
function shownMessage(line: Pick<LogLine, "message" | "spans">) {
  return line.spans
    ? line.spans.map((span) => span.text).join("")
    : line.message;
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

/**
 * The rows as Options ▸ Copy logs writes them (Render's "Copy all currently
 * displayed logs", `⌘/Ctrl ⇧ C`; its `Dn`, read from Render's dashboard bundle
 * by w3/m528): one text line per record, in the order shown, each the
 * record's own RFC 3339 timestamp and then what Copy log line writes
 * (`logLineText`). No date rows and no instance, and the timestamp whether or
 * not the list shows its clock. `resource` names a project view's resource
 * for a line, by id.
 */
export function logRowsText(
  rows: readonly LogListRow[],
  resource?: (id: string) => string | undefined,
): string {
  return rows
    .flatMap((row) => {
      if (row.kind === "day") return [];
      const { line } = row;
      const name =
        resource && line.resource
          ? (resource(line.resource) ?? line.resource)
          : undefined;
      return [
        [line.timestamp, logLineText(line, name)].filter(Boolean).join(" "),
      ];
    })
    .join("\n");
}

/**
 * One line as the row menu's Copy log line writes it (Render's `En`, w3/m528):
 * `[resource] METHOD STATUS host+path`, then the whole message without its
 * terminal styling, with the parts the line has. No time and no instance:
 * Render's label is a project view's resource name, absent on one resource's
 * log.
 */
export function logLineText(line: LogLine, resource?: string): string {
  const parts = [resource ? `[${resource}]` : ""];
  if (line.type === LOG_TYPE_REQUEST) {
    parts.push(line.method, line.statusCode, `${line.host}${line.path}`);
  }
  parts.push(shownMessage(line));
  return parts.filter(Boolean).join(" ");
}

/** The held-lines button's count for `count` held lines (Render's
 *  `BufferedLogsButton`, w3/m528): null below 10, which reads `New logs`,
 *  else the count rounded up to a multiple of 5, at most 100. */
export function heldLinesCount(count: number): number | null {
  return count < 10 ? null : Math.ceil(Math.min(count, 100) / 5) * 5;
}
