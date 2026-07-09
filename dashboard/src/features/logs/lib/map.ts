import type { LogsQuery } from "@/graphql/definitions";
import type { LogLine } from "../types";
import { LOG_TYPE_APP } from "../types";
import { formatLogTimestamp } from "./format";

// map.ts is the log data layer: it turns bex-api's two log wire shapes — the
// GraphQL `LogEntry` (history) and the SSE `renderLog` frame (live tail) — into
// the one flat `LogLine` the viewer draws, and derives the dedupe `key` both
// share so a line seen on both paths is drawn once.

// logLineKey mirrors the backend's stable id derivation (instance + timestamp +
// message, internal/logs/render.go) with a client-computable key. The GraphQL
// projection carries no id, so this is the only key both sources agree on.
export function logLineKey(
  timestamp: string,
  instance: string,
  message: string,
): string {
  return `${timestamp}|${instance}|${message}`;
}

// makeLogLine is the single place a LogLine is built — both wire shapes converge
// here so the key derivation, the once-computed formatted `time`, and the `app`
// type fallback live in one spot.
function makeLogLine(
  timestamp: string,
  instance: string,
  message: string,
  type: string,
): LogLine {
  return {
    key: logLineKey(timestamp, instance, message),
    timestamp,
    time: formatLogTimestamp(timestamp),
    instance,
    message,
    type: type || LOG_TYPE_APP,
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

// fromRenderLog maps one SSE frame onto a LogLine, reading `instance`/`type`
// out of the labels array (Render's shape) rather than flat fields.
export function fromRenderLog(log: RenderLog): LogLine {
  const labels = log.labels ?? [];
  const label = (name: string) =>
    labels.find((l) => l?.name === name)?.value ?? "";
  return makeLogLine(
    log.timestamp ?? "",
    label("instance"),
    log.message ?? "",
    label("type"),
  );
}

// mergeLogLines appends live-tail lines after the historical page, dropping any
// live line whose key already appears in history — a line straddling the last
// historical page and the first streamed frames is drawn once, in order.
export function mergeLogLines(history: LogLine[], live: LogLine[]): LogLine[] {
  const seen = new Set(history.map((l) => l.key));
  const merged = [...history];
  for (const line of live) {
    if (seen.has(line.key)) continue;
    seen.add(line.key);
    merged.push(line);
  }
  return merged;
}
