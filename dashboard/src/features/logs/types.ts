// A single rendered log line — the flat shape the viewer draws, mapped from
// either bex-api's GraphQL `LogEntry` (history) or an SSE `renderLog` frame
// (live tail). `key` is a client-side dedupe key (timestamp|instance|message)
// so a line that appears in both the last historical page and the live stream
// is drawn once — the GraphQL projection carries no id to dedupe on.
export interface LogLine {
  key: string;
  timestamp: string; // RFC3339Nano, or "" when the source omitted it
  time: string; // `timestamp` formatted as Render's line clock, computed once
  instance: string; // replica id (Render's `[bv612]`), or ""
  message: string;
  type: string; // Render's `type` label value; bex sources only `app`
}

// The type filter, matching Render's Logs dropdown (All / Application / Request).
// bex-api sources application logs only: `request` (and `build`) resolve to an
// empty page per its contract (docs/observability.md), never faked. `all` is the
// no-filter sentinel — sent as an absent `type` arg.
export const LOG_TYPE_ALL = "all";
export const LOG_TYPE_APP = "app";
export const LOG_TYPE_REQUEST = "request";

export type LogTypeFilter =
  | typeof LOG_TYPE_ALL
  | typeof LOG_TYPE_APP
  | typeof LOG_TYPE_REQUEST;

export const LOG_TYPE_FILTERS: LogTypeFilter[] = [
  LOG_TYPE_ALL,
  LOG_TYPE_APP,
  LOG_TYPE_REQUEST,
];
