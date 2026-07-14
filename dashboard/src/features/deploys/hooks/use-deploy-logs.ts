import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { LogsDocument } from "@/graphql/definitions";
import { toLogLines, dedupeLogLines } from "@/features/logs/lib/map";
import type { LogLine } from "@/features/logs/types";

// bex-api caps a single logs() page at 100 rows (Render's paging range,
// internal/logs/service.go) — same limit the Logs-tab history hook uses.
const LOGS_LIMIT = 100;

// Poll cadence while the deploy is still open (no endTime yet) — new
// build/predeploy/app lines can land between renders.
const POLL_INTERVAL_MS = 5000;

// The message bex-api returns when a type=build query hits a deployment with
// no durable store wired (core.ErrLogStoreUnavailable → 503) — build logs are
// store-only (predeploy and app both have a live pod-log fallback).
const STORE_UNAVAILABLE_MARKER = "durable log store";

export interface UseDeployLogsResult {
  /** build + predeploy + app lines inside the deploy's window, chronological. */
  lines: LogLine[];
  loading: boolean;
  error: Error | undefined;
  /** True when the build-log leg 503'd because no durable store is wired. */
  buildStoreUnavailable: boolean;
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
function useTypedDeployLogs(
  type: string,
  window: LogsWindow,
  skip?: boolean,
) {
  return useQuery(LogsDocument, {
    variables: { ...window, type },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: window.endTime ? 0 : POLL_INTERVAL_MS,
    skip,
  });
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
 */
export function useDeployLogs(
  resource: string,
  startTime: string | undefined,
  endTime: string | undefined,
  hasPreDeploy: boolean,
): UseDeployLogsResult {
  const window = useMemo(
    () => ({ resource, startTime, endTime, limit: LOGS_LIMIT }),
    [resource, startTime, endTime],
  );

  const build = useTypedDeployLogs("build", window);
  const predeploy = useTypedDeployLogs("predeploy", window, !hasPreDeploy);
  const app = useTypedDeployLogs("app", window);

  const buildStoreUnavailable = !!(
    build.error && build.error.message.includes(STORE_UNAVAILABLE_MARKER)
  );

  const lines = useMemo(() => {
    const merged = [build.data, predeploy.data, app.data].flatMap((d) =>
      toLogLines(d?.logs),
    );
    merged.sort((a, b) => a.timestamp.localeCompare(b.timestamp));
    return dedupeLogLines(merged);
  }, [build.data, predeploy.data, app.data]);

  return {
    lines,
    loading: [build, predeploy, app].some((r) => r.loading && !r.data),
    error: app.error && !buildStoreUnavailable ? app.error : undefined,
    buildStoreUnavailable,
  };
}
