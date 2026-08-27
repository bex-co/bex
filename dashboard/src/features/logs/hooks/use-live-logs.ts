import { useEffect, useRef, useState } from "react";
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
// as GraphQL, minus `/graphql`. Only the tail-honorable filters (type app/all,
// standalone build, text, instance) reach the stream — the store-only ones can't
// be tailed, so the viewer keeps live tail off while any is active
// (docs/ADR010-observability.md).
function subscribeUrl(
  resource: string,
  type: LogTypeFilter,
  text: string,
  instance: string,
  startTime: string,
): string {
  const params = new URLSearchParams({ resource });
  if (type !== LOG_TYPE_ALL) params.set("type", type);
  if (text) params.set("text", text);
  if (instance) params.set("instance", instance);
  // The tail's lower bound: the selected history window's start (w6/m111). The
  // FIRST connect then follows the pod from the window edge instead of replaying
  // its whole log from kubelet's offset 0 — which is what made a "Last hour"
  // range show lines hours older than it. The browser EventSource's own
  // invisible reconnects still resume PAST this via Last-Event-ID (w6/m93); the
  // server takes the later of the two bounds, so the window is a floor, not a
  // re-read. Empty for a caller with no window (the deploy page's build tail).
  if (startTime) params.set("startTime", startTime);
  return `${config.apiBaseUrl}/v1/logs/subscribe?${params.toString()}`;
}

export interface UseLiveLogsOptions {
  resource: string;
  /** Live toggle — off closes the stream (pause); on (re)opens it (resume). */
  enabled: boolean;
  type: LogTypeFilter;
  text: string;
  /** Replica filter — the one structured filter the tail honors (a pod name). */
  instance: string;
  /**
   * Lower bound for the tail — the selected history window's start (ISO), sent
   * as `startTime` so kubelet follows the pod from the window edge, not offset 0,
   * on the first connect (w6/m111). Deliberately NOT part of the subscription
   * identity: the window slides with wall-clock time, and rebinding the stream
   * on every slide would reopen it constantly. It is read via a ref when a
   * (re)subscribe actually happens (a filter change), so a mere slide is ignored
   * and an established tail keeps streaming. Omitted (build tail) => no bound.
   */
  startTime?: string;
  /** Ring-buffer cap on retained live lines; oldest drop first. */
  maxLines?: number;
  /** Reopen a server-terminated stream after this many ms while still enabled —
   *  for tails whose terminal reason can be transient, like a build tail opened
   *  before the build pod exists. 0/omitted = a terminal error closes for good. */
  retryDelayMs?: number;
  /** Injectable stream factory (tests). Defaults to a credentialed EventSource. */
  createEventSource?: EventSourceFactory;
}

export interface UseLiveLogsResult {
  lines: LogLine[];
  status: LiveStatus;
}

// Ring-buffer cap on retained live lines (oldest drop first). Raised from 1,000
// to 5,000 by w9/m83: the log list is now virtualized, so the retained buffer no
// longer maps 1:1 to DOM rows — only the visible window is mounted. The bound
// that made 1,000 the ceiling (whole-buffer DOM reconciliation on every tail
// frame) is gone, so a longer scrollback costs memory + one ANSI parse per line
// at ingest, not per-frame rendering work.
const DEFAULT_MAX_LINES = 5000;

// How long incoming frames accumulate before one flush to state. A busy tail
// can deliver a frame per log line; flushing each one re-renders the whole
// ~1k-row list per line, so frames batch on a short timer — at most one
// re-render per FLUSH_MS no matter how fast the stream speaks.
const FLUSH_MS = 100;

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
  instance,
  startTime = "",
  maxLines = DEFAULT_MAX_LINES,
  retryDelayMs = 0,
  createEventSource = defaultCreateEventSource,
}: UseLiveLogsOptions): UseLiveLogsResult {
  // Status while the stream should be live but hasn't opened yet vs. paused.
  const pendingStatus: LiveStatus = enabled ? "connecting" : "idle";
  const [status, setStatus] = useState<LiveStatus>(pendingStatus);
  // Bumped to reopen a terminated stream (retryDelayMs); deliberately NOT part
  // of subKey below — a retry continues the same subscription, so the buffer
  // survives and dedupe absorbs any replayed lines.
  const [attempt, setAttempt] = useState(0);

  // Reset the tail the instant the subscription identity changes (or it
  // pauses) — during render, React's sanctioned "adjust state when a prop
  // changes" pattern, so a stale filter's lines never flash before the effect
  // reconnects, and setState stays out of the effect body. The buffer carries
  // its subKey so a stale effect's final flush (cleanup ordering: the reset
  // lands first) can tell it no longer owns the buffer and drop its batch.
  const subKey = `${resource}|${enabled}|${type}|${text}|${instance}`;
  const [buffer, setBuffer] = useState<{ key: string; lines: LogLine[] }>({
    key: subKey,
    lines: [],
  });
  if (buffer.key !== subKey) {
    setBuffer({ key: subKey, lines: [] });
    setStatus(pendingStatus);
  }

  // O(1) dedupe index for the merge — the keys of every line currently in the
  // buffer, replacing a per-frame linear scan of up to maxLines entries. The
  // cap's evictions delete their keys (see flushPending), so a replayed
  // evicted line re-appends exactly as the old scan semantics allowed. It is
  // tagged with the subscription epoch below rather than subKey itself: a
  // disable/enable round trip reproduces the same subKey string while the
  // buffer was reset in between, and the set must reset with it.
  const keysRef = useRef<{ epoch: number; set: Set<string> }>({
    epoch: 0,
    set: new Set(),
  });
  // Bumped after every subscription-identity change (a retry bumps `attempt`,
  // not subKey, so a retried stream correctly keeps its dedupe set).
  const epochRef = useRef(0);
  useEffect(() => {
    epochRef.current += 1;
  }, [subKey]);

  // Latest window start, held out of subKey so the sliding window (a new value
  // every resolution tick) never reopens the stream. The subscribe effect reads
  // this ref only when it actually (re)runs — i.e. on a real filter change — so
  // each fresh subscription follows the pod from the current window edge while
  // an established tail streams on undisturbed. Declared before the subscribe
  // effect so it is current by the time that effect reads it in the same flush.
  const startTimeRef = useRef(startTime);
  useEffect(() => {
    startTimeRef.current = startTime;
  });

  useEffect(() => {
    if (!enabled) return;

    let retryTimer: ReturnType<typeof setTimeout> | undefined;
    // Frames received since the last flush; a short timer (same pattern as
    // retryTimer below) batches a burst of frames into one state update.
    let pending: LogLine[] = [];
    let flushTimer: ReturnType<typeof setTimeout> | undefined;
    const es = createEventSource(
      subscribeUrl(resource, type, text, instance, startTimeRef.current),
    );

    // Flush the pending batch into the ring buffer: dedupe against the key
    // set (O(1) per line), append, and cap oldest-first — the same merge the
    // pre-batch per-frame updater did, once per batch instead of per frame.
    const flushPending = () => {
      if (pending.length === 0) return;
      if (keysRef.current.epoch !== epochRef.current) {
        keysRef.current = { epoch: epochRef.current, set: new Set() };
      }
      const keys = keysRef.current.set;
      const batch = pending.filter((line) => {
        if (keys.has(line.key)) return false;
        keys.add(line.key);
        return true;
      });
      pending = [];
      if (batch.length === 0) return;
      setBuffer((prev) => {
        // A stale tail flushing after a filter switch: the render-phase reset
        // already swapped in the new subscription's empty buffer — drop the
        // batch instead of resurrecting the old filter's lines.
        if (prev.key !== subKey) return prev;
        const next = [...prev.lines, ...batch];
        if (next.length <= maxLines) return { key: prev.key, lines: next };
        const kept = next.slice(next.length - maxLines);
        // Keep the dedupe set in sync with the ring buffer: evicted lines no
        // longer dedupe (idempotent under StrictMode's double-invoked
        // updater — same prev computes the same evictions).
        for (const line of next.slice(0, next.length - maxLines))
          keys.delete(line.key);
        return { key: prev.key, lines: kept };
      });
    };

    const scheduleFlush = () => {
      if (flushTimer !== undefined) return;
      flushTimer = setTimeout(() => {
        flushTimer = undefined;
        flushPending();
      }, FLUSH_MS);
    };

    es.onopen = () => setStatus("open");

    es.onmessage = (ev) => {
      let line: LogLine;
      try {
        line = fromRenderLog(JSON.parse(ev.data) as RenderLog);
      } catch {
        return; // a malformed frame shouldn't tear down a long-lived tail
      }
      pending.push(line);
      scheduleFlush();
    };

    es.onerror = (ev) => {
      // bex-api surfaces a terminal reason (e.g. logs source unavailable) as an
      // `event: error` frame carrying `data`; a bare error is a transport drop
      // (EventSource auto-reconnects under the hood — `onopen` clears this).
      setStatus("error");
      if (ev && typeof ev.data !== "undefined") {
        es.close();
        // A terminal reason can be transient while the caller still wants the
        // tail (a build subscription racing its own pod's creation) — reopen
        // after a delay rather than staying dead for the rest of the deploy.
        if (retryDelayMs > 0) {
          retryTimer = setTimeout(() => setAttempt((a) => a + 1), retryDelayMs);
        }
      }
    };

    return () => {
      if (retryTimer !== undefined) clearTimeout(retryTimer);
      if (flushTimer !== undefined) clearTimeout(flushTimer);
      // Don't lose the trailing batch: on a retry it belongs to the same
      // subscription and must land; on a filter switch flushPending's stale
      // guard drops it (the buffer was already reset during render).
      flushPending();
      es.close();
    };
  }, [
    resource,
    enabled,
    type,
    text,
    instance,
    // Derived from the five above — listed for exhaustive-deps; it changes
    // exactly when they do.
    subKey,
    maxLines,
    retryDelayMs,
    createEventSource,
    attempt,
  ]);

  return { lines: buffer.lines, status };
}
