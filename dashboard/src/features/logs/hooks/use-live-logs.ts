import { useEffect, useState } from "react";
import { config } from "@/config/config";
import { fromRenderLog, type RenderLog } from "../lib/map";
import { LOG_TYPE_ALL, type LogLine, type LogTypeFilter } from "../types";

/*
 * DELIBERATE DIVERGENCE FROM RENDER — transport.
 *
 * bex live-tails logs over Server-Sent Events (`GET /v1/logs/subscribe`), where
 * Render upgrades the connection to a WebSocket. Same "stream new lines live"
 * contract, but SSE needs no extra dependency, works with a plain `EventSource`
 * (and `curl -N`), and rides the same bearer/cookie auth as every other bex-api
 * read. This is bex's documented choice — see docs/ADR006-bex-api.md and
 * docs/ADR010-observability.md ("Live tail (SSE)"). If Render parity ever demands a
 * WebSocket, it swaps in *here*, behind this hook, without touching the viewer.
 *
 * Auth: `EventSource` carries the Kratos session cookie via `withCredentials`,
 * exactly as Apollo's HttpLink sends `credentials: "include"` for GraphQL.
 */

export type LiveStatus = "idle" | "connecting" | "open" | "error";

// A minimal EventSource surface — the browser's `EventSource` satisfies it, and
// tests inject a fake to drive append/disconnect paths deterministically (the
// same injected-source pattern the backend uses for PodLogStream).
export interface EventSourceLike {
  onopen: (() => void) | null;
  onmessage: ((ev: { data: string }) => void) | null;
  onerror: ((ev: { data?: unknown }) => void) | null;
  close: () => void;
}

export type EventSourceFactory = (url: string) => EventSourceLike;

// The default stream: the browser EventSource, credentialed so bex-api sees the
// session cookie. Only referenced inside the effect (client-only) — never on the
// SSR pass, where EventSource is undefined.
const defaultCreateEventSource: EventSourceFactory = (url) =>
  new EventSource(url, { withCredentials: true }) as unknown as EventSourceLike;

// bex-api's REST/SSE base lives in config (config.apiBaseUrl) — the same origin
// as GraphQL, minus `/graphql`.
function subscribeUrl(
  resource: string,
  type: LogTypeFilter,
  text: string,
): string {
  const params = new URLSearchParams({ resource });
  if (type !== LOG_TYPE_ALL) params.set("type", type);
  if (text) params.set("text", text);
  return `${config.apiBaseUrl}/v1/logs/subscribe?${params.toString()}`;
}

export interface UseLiveLogsOptions {
  resource: string;
  /** Live toggle — off closes the stream (pause); on (re)opens it (resume). */
  enabled: boolean;
  type: LogTypeFilter;
  text: string;
  /** Ring-buffer cap on retained live lines; oldest drop first. */
  maxLines?: number;
  /** Injectable stream factory (tests). Defaults to a credentialed EventSource. */
  createEventSource?: EventSourceFactory;
}

export interface UseLiveLogsResult {
  lines: LogLine[];
  status: LiveStatus;
}

const DEFAULT_MAX_LINES = 1000;

/**
 * Subscribes to bex-api's SSE log stream for one App and accumulates new lines
 * into a capped, deduped ring buffer. Reopening on any filter change (fresh
 * buffer) keeps the live view consistent with the historical query beside it.
 */
export function useLiveLogs({
  resource,
  enabled,
  type,
  text,
  maxLines = DEFAULT_MAX_LINES,
  createEventSource = defaultCreateEventSource,
}: UseLiveLogsOptions): UseLiveLogsResult {
  // Status while the stream should be live but hasn't opened yet vs. paused.
  const pendingStatus: LiveStatus = enabled ? "connecting" : "idle";
  const [lines, setLines] = useState<LogLine[]>([]);
  const [status, setStatus] = useState<LiveStatus>(pendingStatus);

  // Reset the tail the instant the subscription identity changes (or it
  // pauses) — during render, React's sanctioned "adjust state when a prop
  // changes" pattern, so a stale filter's lines never flash before the effect
  // reconnects, and setState stays out of the effect body.
  const subKey = `${resource}|${enabled}|${type}|${text}`;
  const [prevSubKey, setPrevSubKey] = useState(subKey);
  if (prevSubKey !== subKey) {
    setPrevSubKey(subKey);
    setLines([]);
    setStatus(pendingStatus);
  }

  useEffect(() => {
    if (!enabled) return;

    const es = createEventSource(subscribeUrl(resource, type, text));

    es.onopen = () => setStatus("open");

    es.onmessage = (ev) => {
      let line: LogLine;
      try {
        line = fromRenderLog(JSON.parse(ev.data) as RenderLog);
      } catch {
        return; // a malformed frame shouldn't tear down a long-lived tail
      }
      // Functional updater: dedupe against the current buffer and cap it (oldest
      // lines drop first) without a ref — the buffer *is* the rendered state.
      setLines((prev) => {
        if (prev.some((l) => l.key === line.key)) return prev;
        const next = [...prev, line];
        return next.length > maxLines
          ? next.slice(next.length - maxLines)
          : next;
      });
    };

    es.onerror = (ev) => {
      // bex-api surfaces a terminal reason (e.g. logs source unavailable) as an
      // `event: error` frame carrying `data`; a bare error is a transport drop
      // (EventSource auto-reconnects under the hood — `onopen` clears this).
      setStatus("error");
      if (ev && typeof ev.data !== "undefined") es.close();
    };

    return () => es.close();
  }, [resource, enabled, type, text, maxLines, createEventSource]);

  return { lines, status };
}
