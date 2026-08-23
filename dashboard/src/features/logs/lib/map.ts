import type { LogsQuery } from "@/graphql/definitions";
import type { LogLine } from "../types";
import { LOG_TYPE_APP } from "../types";
import { formatLogTimestamp } from "./format";
import { needsAnsiParse, parseAnsi } from "./ansi";

// map.ts is the log data layer: it turns bex-api's two log wire shapes — the
// GraphQL `LogEntry` (history) and the SSE `renderLog` frame (live tail) — into
// the one flat `LogLine` the viewer draws, and derives the dedupe `key` both
// share so a line seen on both paths is drawn once.

// logLineKey mirrors the backend's stable id derivation (instance + timestamp +
// message, internal/logs/render.go) with a client-computable key. The GraphQL
// projection carries no id, so this is the only key both sources agree on.
//
// The timestamp enters the key at MILLISECOND resolution (JS's native clock
// precision, and finer than the viewer's second-precision clock display), not
// as the raw wire string: a record redelivered with its timestamp at a coarser
// sub-second precision (an SSE reconnect replay whose stamp was re-rendered
// without its nanoseconds) is the same line and must not render twice — that
// drift double-rendered a cron run's lines in w9/053. Two genuinely distinct
// records share a key only when same pod + same message + same millisecond,
// which the viewer already treats as one record. An empty/unparseable stamp
// keeps the raw string so stamp-less lines key exactly as before.
export function logLineKey(
  timestamp: string,
  instance: string,
  message: string,
): string {
  const ms = Date.parse(timestamp);
  return `${Number.isNaN(ms) ? timestamp : ms}|${instance}|${message}`;
}

// makeLogLine is the single place a LogLine is built — both wire shapes converge
// here so the key derivation, the once-computed formatted `time`, and the `app`
// type fallback live in one spot. level/method/statusCode come from the entry's
// labels (empty for an app line without them, populated for a request line).
function makeLogLine(
  timestamp: string,
  instance: string,
  message: string,
  type: string,
  labels: { level?: string; method?: string; statusCode?: string } = {},
): LogLine {
  return {
    key: logLineKey(timestamp, instance, message),
    timestamp,
    time: formatLogTimestamp(timestamp),
    instance,
    message,
    // ANSI parsed once here at ingest, not per render (w9/m63 t001).
    spans: needsAnsiParse(message) ? parseAnsi(message) : null,
    type: type || LOG_TYPE_APP,
    level: labels.level ?? "",
    method: labels.method ?? "",
    statusCode: labels.statusCode ?? "",
  };
}

type GraphQLLogEntry = NonNullable<NonNullable<LogsQuery["logs"]>[number]>;

// toLogLine maps one GraphQL LogEntry row onto a LogLine.
export function toLogLine(e: GraphQLLogEntry): LogLine {
  return makeLogLine(
    e.timestamp ?? "",
    e.instance ?? "",
    e.message ?? "",
    e.type ?? "",
    {
      level: e.level ?? "",
      method: e.method ?? "",
      statusCode: e.statusCode ?? "",
    },
  );
}

// toLogLines maps a full GraphQL `logs` result, dropping the nullable holes.
export function toLogLines(entries: LogsQuery["logs"] | undefined): LogLine[] {
  return (entries ?? [])
    .filter((e): e is GraphQLLogEntry => e != null)
    .map(toLogLine);
}

// renderLog is bex-api's REST/SSE log object (internal/logs/render.go): id +
// message + timestamp + a Render `[{name,value}]` labels array. The SSE stream
// sends one such JSON object per `data:` frame.
export interface RenderLog {
  id?: string;
  message?: string;
  timestamp?: string;
  labels?: Array<{ name?: string; value?: string }>;
}

// fromRenderLog maps one SSE frame onto a LogLine, reading the labels out of the
// array (Render's shape) rather than flat fields. The live tail reads app pod
// logs, so level/method/statusCode are usually empty here — but they're mapped
// the same way so a shipper that does attach them renders identically.
export function fromRenderLog(log: RenderLog): LogLine {
  const labels = log.labels ?? [];
  const label = (name: string) =>
    labels.find((l) => l?.name === name)?.value ?? "";
  return makeLogLine(
    log.timestamp ?? "",
    label("instance"),
    log.message ?? "",
    label("type"),
    {
      level: label("level"),
      method: label("method"),
      statusCode: label("statusCode"),
    },
  );
}

// dedupeLogLines drops any line whose key already appeared earlier in the
// array, keeping the first occurrence — the key-based unique-by-key building
// block any multi-source merge (mergeLogLines below, or a fan-out across
// several typed queries) shares.
export function dedupeLogLines(lines: LogLine[]): LogLine[] {
  const seen = new Set<string>();
  return lines.filter((line) => {
    if (seen.has(line.key)) return false;
    seen.add(line.key);
    return true;
  });
}

// mergeLogLines appends live-tail lines after the historical page, dropping any
// live line whose key already appears in history — a line straddling the last
// historical page and the first streamed frames is drawn once, in order.
export function mergeLogLines(history: LogLine[], live: LogLine[]): LogLine[] {
  return dedupeLogLines([...history, ...live]);
}
