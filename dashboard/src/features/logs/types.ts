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
  type: string; // Render's `type` label value: `app` or `request`
  // Request/HTTP-line labels (populated for `type=request`; empty for app
  // lines). The Logs viewer renders a request line as method/status chips
  // instead of the raw Traefik JSON (w5/008, docs/ADR010-observability.md).
  level: string; // app-line severity (info/warn/error/…), or ""
  method: string; // request HTTP method, or ""
  statusCode: string; // request HTTP status, or ""
}

// The type filter, matching Render's Logs dropdown (All / Application / Request).
// With the durable store wired (prod) `request` returns real Traefik access
// lines; without it (local dev) it 503s — an honest state the viewer renders as
// "request logs need the log store" (docs/ADR010-observability.md). `all` is the
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

// The discoverable log labels the filter dropdowns query values for
// (`logLabelValues`) — Render's filter vocabulary. `host` is answered from the
// App's own URLs even without the store; the rest are stream labels the shipper
// attaches (docs/ADR010-observability.md § Log filters).
export const LOG_LABEL_LEVEL = "level";
export const LOG_LABEL_INSTANCE = "instance";
export const LOG_LABEL_METHOD = "method";
export const LOG_LABEL_STATUS_CODE = "statusCode";

// The Logs-tab filter state. Each structured filter is single-valued in the UI
// ("" = no filter) and sent to bex-api as a single-element list — the same
// single-select + "all"-sentinel shape the metrics filter bar uses. `path` is a
// free-text request-path filter (too high-cardinality for a dropdown, so Render
// keeps it a text input); the rest are dropdowns fed by `logLabelValues`.
export interface LogFilters {
  type: LogTypeFilter;
  text: string;
  level: string;
  instance: string;
  method: string;
  statusCode: string;
  path: string;
}

export const EMPTY_LOG_FILTERS: LogFilters = {
  type: LOG_TYPE_ALL,
  text: "",
  level: "",
  instance: "",
  method: "",
  statusCode: "",
  path: "",
};

// The structured filters only the durable store can honor. The live tail reads
// pod stdout, so it structurally cannot serve request logs or these
// label/request filters (bex-api answers them 400, not silently) — the viewer
// disables live tail while any is active (docs/ADR010-observability.md § The tail
// reads pod logs). `instance` is NOT here: a pod name is a pod name, so the tail
// honors it.
export function usesStoreOnlyFilters(f: LogFilters): boolean {
  return (
    f.type === LOG_TYPE_REQUEST ||
    f.level !== "" ||
    f.method !== "" ||
    f.statusCode !== "" ||
    f.path !== ""
  );
}
