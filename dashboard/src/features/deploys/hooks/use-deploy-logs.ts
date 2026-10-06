import { useCallback, useEffect, useMemo, useState } from "react";
import { useQuery } from "@apollo/client/react";
import { skipPollWhenHidden } from "@/common/lib/polling";
import { LogsDocument } from "@/graphql/definitions";
import { toLogLines, dedupeLogLines } from "@/features/logs/lib/map";
import {
  useLiveLogs,
  type EventSourceFactory,
  type LiveStatus,
} from "@/features/logs/hooks/use-live-logs";
import { useOlderLogPages } from "@/features/logs/hooks/use-older-log-pages";
import { LOG_TYPE_BUILD } from "@/features/logs/types";
import type { LogLine } from "@/features/logs/types";
import { isLogStoreUnavailable } from "@/features/logs/lib/log-store";

// bex-api caps a single logs() page at 100 rows (Render's paging range,
// internal/logs/service.go) — same limit the Logs-tab history hook uses.
const LOGS_LIMIT = 100;

// Poll cadence while the deploy is still open (no endTime yet) — for
// predeploy/app lines. Build lines are streamed live via SSE (w3/m14), so
// the build GraphQL leg only polls on completion to pick up any store-flushed
// lines once the deploy finishes.
const POLL_INTERVAL_MS = 5000;

// How long the windowed queries keep polling after the deploy closes its
// window (finishedAt set). Loki ingests a build pod's lines seconds behind the
// pod writing them, and a build that fails the moment it speaks closes the
// window immediately — stopping on the very first closed-window fetch can
// permanently miss those lines until a manual reload.
const SETTLE_MS = 15000;

// Reopen delay for the build SSE tail after a server-terminated subscription.
// The tail can terminate transiently — subscribing the instant a deploy opens
// can race the build Job's pod into existence — so while the deploy is still
// build_in_progress a dead tail retries instead of staying silent.
const BUILD_RETRY_MS = 5000;

/** The deploy viewer's type buckets; Application is everything but build. */
type LogBucket = "build" | "app";

// A record's buckets as bits: a pre-deploy line the durable build leg also
// carries is in both.
const IN_BUILD = 1;
const IN_OTHER = 2;

function bucketBit(type: string): number {
  return type === LOG_TYPE_BUILD ? IN_BUILD : IN_OTHER;
}

export interface UseDeployLogsResult {
  /** build + predeploy + app lines inside the deploy's window, chronological. */
  lines: LogLine[];
  /**
   * Whether a line belongs in the Build or Application bucket. A deduped line
   * may stand for copies from several legs (a pre-deploy line is also durable
   * build output), so its own `type` alone would drop it from one bucket.
   */
  inLogBucket: (line: LogLine, bucket: LogBucket) => boolean;
  loading: boolean;
  error: Error | undefined;
  /** True when the build-log leg 503'd because no durable store is wired. */
  buildStoreUnavailable: boolean;
  /** SSE state while the active build pod is being followed. */
  buildLiveStatus: LiveStatus;
  /** True while any leg reports history older than what's loaded. */
  hasMore: boolean;
  loadingOlder: boolean;
  /** Fetch the next older page of every leg that has more. */
  loadOlder: () => void;
}

interface LogsWindow {
  resource: string;
  startTime: string | undefined;
  endTime: string | undefined;
  limit: number;
}

// One windowed logs() query for a single type — the shared shape build/
// predeploy/app below all issue, so the three calls differ only in `type` and
// `skip`. Still three separate hook calls (GraphQL's `type` arg is single-
// valued and `predeploy` must be requested alone, internal/logs/service.go's
// validate()) — this just removes the per-call options boilerplate.
//
// Each leg pages backwards on its own (w4/m136): a long build overflows the
// 100-row page while its app leg may not, so each keeps its own cursor and the
// merge below interleaves whatever each has loaded.
function useTypedDeployLogs(
  type: string,
  window: LogsWindow,
  poll: boolean,
  skip?: boolean,
) {
  const variables = useMemo(() => ({ ...window, type }), [window, type]);
  const query = useQuery(LogsDocument, {
    variables,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: poll ? POLL_INTERVAL_MS : 0,
    skipPollAttempt: skipPollWhenHidden,
    skip,
  });
  const pages = useOlderLogPages(variables, query.data?.logs);
  return { ...query, pages };
}

// True while the windowed queries should still poll: always for an open window,
// and for SETTLE_MS after a closed one first renders — the ingest-lag grace.
function useWindowPolling(endTime: string | undefined): boolean {
  const [settled, setSettled] = useState(false);
  // Reset during render when the window identity changes (endTime cleared by a
  // range switch, or set by the deploy finishing) — the same sanctioned
  // adjust-state-on-prop-change pattern as use-live-logs' subKey reset.
  const [prevEnd, setPrevEnd] = useState(endTime);
  if (prevEnd !== endTime) {
    setPrevEnd(endTime);
    setSettled(false);
  }
  useEffect(() => {
    if (!endTime) return;
    const timer = setTimeout(() => setSettled(true), SETTLE_MS);
    return () => clearTimeout(timer);
  }, [endTime]);
  return !endTime || !settled;
}

/**
 * Reads a deploy's logs — build, pre-deploy, and the service's own app lines —
 * scoped to `[startTime, endTime]` (the deploy's createdAt..finishedAt window,
 * open-ended while the deploy is still running). bex-api's GraphQL `type` arg
 * is single-valued and `predeploy` must be requested on its own
 * (internal/logs/service.go's validate()), so this issues three separate
 * windowed queries and merges them chronologically client-side — the same
 * `LogEntry` → `LogLine` mapping the Logs tab uses (toLogLines), just fanned
 * out across the three deploy-relevant types instead of the tab's single
 * type filter. `hasPreDeploy` skips the predeploy leg entirely for the common
 * case (a deploy with no pre-deploy command configured) instead of polling a
 * query that can only ever come back empty.
 *
 * While the deploy is in-flight (!endTime), build lines are ALSO streamed live
 * from the build Job's pod stdout via SSE (w3/m14) and merged with the
 * historical query result so new lines appear in real time.
 */
export function useDeployLogs(
  resource: string,
  startTime: string | undefined,
  endTime: string | undefined,
  hasPreDeploy: boolean,
  followBuild: boolean,
  createEventSource?: EventSourceFactory,
): UseDeployLogsResult {
  const window = useMemo(
    () => ({ resource, startTime, endTime, limit: LOGS_LIMIT }),
    [resource, startTime, endTime],
  );

  // Skip every leg until the deploy's window (its `startTime`) is known — the
  // page mounts the panel in parallel with the header query (w9/m62 t002), and
  // a windowless logs query would read the wrong range. A `?r=` range supplies
  // `startTime` directly, so a ranged panel queries immediately.
  const hasWindow = startTime !== undefined;
  const poll = useWindowPolling(endTime);
  const build = useTypedDeployLogs("build", window, poll, !hasWindow);
  const predeploy = useTypedDeployLogs(
    "predeploy",
    window,
    poll,
    !hasPreDeploy || !hasWindow,
  );
  const app = useTypedDeployLogs("app", window, poll, !hasWindow);
  const liveBuild = useLiveLogs({
    resource,
    enabled: followBuild,
    type: LOG_TYPE_BUILD,
    text: "",
    instance: "",
    retryDelayMs: BUILD_RETRY_MS,
    createEventSource,
  });

  // Build-log history lives only in the durable store; the live SSE tail reads
  // the build pod's stdout, so a store-less deployment still streams it.
  const buildStoreUnavailable = isLogStoreUnavailable(build.error);
  const queryError = [build.error, predeploy.error, app.error].find(
    (candidate) => candidate && !isLogStoreUnavailable(candidate),
  );

  // History is the expensive leg — mapping, sorting, and deduping three
  // windowed query results. Memoize it on the query data identities so a
  // streamed live line never re-maps or re-sorts it.
  const { history, bucketsByKey } = useMemo(() => {
    const merged = [
      build.pages.older,
      predeploy.pages.older,
      app.pages.older,
      ...[build.data, predeploy.data, app.data].map((d) =>
        toLogLines(d?.logs?.logs),
      ),
    ].flat();
    merged.sort((a, b) => a.timestamp.localeCompare(b.timestamp));
    // Keep one copy of a record two legs returned (a pre-deploy line the
    // durable build leg also carries), in the bucket of every type it came as.
    const lines: LogLine[] = [];
    const buckets = new Map<string, number>();
    for (const line of merged) {
      const seen = buckets.get(line.key);
      if (seen === undefined) lines.push(line);
      buckets.set(line.key, (seen ?? 0) | bucketBit(line.type));
    }
    return { history: lines, bucketsByKey: buckets };
  }, [
    build.data,
    predeploy.data,
    app.data,
    build.pages.older,
    predeploy.pages.older,
    app.pages.older,
  ]);

  const legs = [build.pages, predeploy.pages, app.pages];
  const hasMore = legs.some((leg) => leg.hasMore);
  const loadingOlder = legs.some((leg) => leg.loadingOlder);
  const loadBuild = build.pages.loadOlder;
  const loadPredeploy = predeploy.pages.loadOlder;
  const loadApp = app.pages.loadOlder;
  // Each leg's loadOlder is a no-op while that leg has nothing older.
  const loadOlder = useCallback(() => {
    loadBuild();
    loadPredeploy();
    loadApp();
  }, [loadBuild, loadPredeploy, loadApp]);

  // A live line is build output (the live tail reads only build pods), and a
  // history line is in every bucket one of its copies belongs to.
  const inLogBucket = useCallback(
    (line: LogLine, bucket: LogBucket): boolean => {
      const buckets = bucketsByKey.get(line.key) ?? bucketBit(line.type);
      return (buckets & (bucket === "build" ? IN_BUILD : IN_OTHER)) !== 0;
    },
    [bucketsByKey],
  );

  const lines = useMemo(() => {
    const live = liveBuild.lines;
    if (live.length === 0) return history;
    const last = history[history.length - 1];
    // Fast path: the live tail arrives chronologically after history (it
    // streams the same build pod the windowed query reads behind ingest lag),
    // so merging is an append plus a key filter for the poll/stream straddle
    // — O(live) per flush, no re-sort of history per streamed line.
    if (!last || live[0].timestamp >= last.timestamp) {
      return [
        ...history,
        ...live.filter((line) => !bucketsByKey.has(line.key)),
      ];
    }
    // Correctness fallback: a live line predates the tail of history (the
    // query won the race against the stream) — full chronological merge.
    const merged = [...history, ...live];
    merged.sort((a, b) => a.timestamp.localeCompare(b.timestamp));
    return dedupeLogLines(merged);
  }, [history, bucketsByKey, liveBuild.lines]);

  return {
    lines,
    inLogBucket,
    loading: [build, predeploy, app].some((r) => r.loading && !r.data),
    error: queryError,
    buildStoreUnavailable,
    buildLiveStatus: liveBuild.status,
    hasMore,
    loadingOlder,
    loadOlder,
  };
}
