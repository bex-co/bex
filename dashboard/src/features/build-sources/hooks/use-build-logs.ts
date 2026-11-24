import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@apollo/client/react";
import { skipPollWhenHidden } from "@/common/lib/polling";
import { LogsDocument } from "@/graphql/definitions";
import {
  compareLogTimestamps,
  mergeLogLines,
  toLogLines,
} from "@/features/logs/lib/map";
import {
  useLiveLogs,
  type EventSourceFactory,
  type LiveStatus,
} from "@/features/logs/hooks/use-live-logs";
import { useOlderLogPages } from "@/features/logs/hooks/use-older-log-pages";
import { isLogStoreUnavailable } from "@/features/logs/lib/log-store";
import { LOG_TYPE_BUILD, type LogLine } from "@/features/logs/types";

const LOGS_LIMIT = 100;
const POLL_INTERVAL_MS = 5000;
const SETTLE_MS = 15000;
const BUILD_RETRY_MS = 5000;

function useBuildLogWindowPolling(endTime: string | undefined): boolean {
  const [settled, setSettled] = useState(false);
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

export interface UseBuildLogsResult {
  lines: LogLine[];
  loading: boolean;
  error: Error | undefined;
  buildStoreUnavailable: boolean;
  buildLiveStatus: LiveStatus;
  hasMore: boolean;
  loadingOlder: boolean;
  loadOlder: () => void;
}

/**
 * Reads historical + live-streaming SSE build logs for one Build Source build
 * (`resource=bds-…`, `build=[bld-…]`, `type="build"`, powered by w2/m611).
 * Polls historical logs while the build is open plus a 15s ingest-lag settle
 * window after completion, and streams live stdout from the build pod via SSE
 * while `followBuild` is true.
 */
export function useBuildLogs({
  buildSourceId,
  buildId,
  startTime,
  endTime,
  followBuild,
  enabled = true,
  createEventSource,
}: {
  buildSourceId: string;
  buildId: string;
  startTime: string | undefined;
  endTime: string | undefined;
  followBuild: boolean;
  enabled?: boolean;
  createEventSource?: EventSourceFactory;
}): UseBuildLogsResult {
  const variables = useMemo(
    () => ({
      resource: buildSourceId,
      type: LOG_TYPE_BUILD,
      build: [buildId],
      startTime,
      endTime,
      limit: LOGS_LIMIT,
    }),
    [buildSourceId, buildId, startTime, endTime],
  );

  const hasWindow = enabled && startTime !== undefined;
  const poll = useBuildLogWindowPolling(endTime);
  const query = useQuery(LogsDocument, {
    variables,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: hasWindow && poll ? POLL_INTERVAL_MS : 0,
    skipPollAttempt: skipPollWhenHidden,
    skip: !hasWindow,
  });

  const pages = useOlderLogPages(variables, query.data?.logs);
  const liveBuild = useLiveLogs({
    resource: buildSourceId,
    build: buildId,
    enabled: enabled && followBuild,
    type: LOG_TYPE_BUILD,
    text: "",
    instance: "",
    retryDelayMs: BUILD_RETRY_MS,
    createEventSource,
  });

  const buildStoreUnavailable = isLogStoreUnavailable(query.error);
  const queryError =
    query.error && !buildStoreUnavailable ? query.error : undefined;

  const { history, seenKeys } = useMemo(() => {
    const merged = [...pages.older, ...toLogLines(query.data?.logs?.logs)];
    merged.sort((a, b) => compareLogTimestamps(a.timestamp, b.timestamp));
    const lines: LogLine[] = [];
    const keys = new Set<string>();
    for (const line of merged) {
      if (keys.has(line.key)) continue;
      keys.add(line.key);
      lines.push(line);
    }
    return { history: lines, seenKeys: keys };
  }, [pages.older, query.data]);

  const lines = useMemo(() => {
    const live = liveBuild.lines;
    if (live.length === 0) return history;
    const last = history[history.length - 1];
    if (!last || compareLogTimestamps(live[0].timestamp, last.timestamp) >= 0) {
      return [...history, ...live.filter((line) => !seenKeys.has(line.key))];
    }
    return mergeLogLines(history, live);
  }, [history, seenKeys, liveBuild.lines]);

  return {
    lines,
    loading: hasWindow && query.loading && !query.data,
    error: queryError,
    buildStoreUnavailable,
    buildLiveStatus: liveBuild.status,
    hasMore: pages.hasMore,
    loadingOlder: pages.loadingOlder,
    loadOlder: pages.loadOlder,
  };
}
