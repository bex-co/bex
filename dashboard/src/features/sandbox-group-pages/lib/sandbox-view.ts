import type { en } from "@/i18n";

type MessageKey = keyof typeof en;

/**
 * How a sandbox group's pages read what bex-api answers (w1/m658), in the
 * words Render's dashboard uses for the same states
 * (docs/render-artifacts/sandbox-groups.md § The dashboard).
 */

/** bex's sandbox statuses, Render's SandboxStatus in lower case. */
export const SANDBOX_STATUSES = [
  "creating",
  "running",
  "resuming",
  "suspending",
  "suspended",
  "errored",
  "terminated",
] as const;
export type SandboxStatus = (typeof SANDBOX_STATUSES)[number];

export function isSandboxStatus(value: string): value is SandboxStatus {
  return (SANDBOX_STATUSES as readonly string[]).includes(value);
}

export const SANDBOX_STATUS_LABEL: Record<SandboxStatus, MessageKey> = {
  creating: "sandboxGroupPages.statusCreating",
  running: "sandboxGroupPages.statusRunning",
  resuming: "sandboxGroupPages.statusResuming",
  suspending: "sandboxGroupPages.statusSuspending",
  suspended: "sandboxGroupPages.statusSuspended",
  errored: "sandboxGroupPages.statusErrored",
  terminated: "sandboxGroupPages.statusTerminated",
};

/**
 * The statuses that count against a group's concurrency limit and that a
 * sandbox can still change from: Render's "Creating, running, resuming, or
 * suspending now". A drawer and a row keep polling while one holds.
 */
export function isActiveSandbox(status: string): boolean {
  return (
    status === "creating" ||
    status === "running" ||
    status === "resuming" ||
    status === "suspending"
  );
}

/** A sandbox that has ended: nothing about it changes any more. */
export function isEndedSandbox(status: string): boolean {
  return status === "terminated" || status === "errored";
}

/**
 * Render's search box turns a status word into a status filter ("running"
 * lists the running sandboxes). The word may be bex's status or its label in
 * the reader's language.
 */
export function statusFromSearch(
  search: string,
  label: (key: MessageKey) => string,
): SandboxStatus | null {
  const word = search.trim().toLowerCase();
  if (!word) return null;
  return (
    SANDBOX_STATUSES.find(
      (status) =>
        status === word ||
        label(SANDBOX_STATUS_LABEL[status]).toLowerCase() === word,
    ) ?? null
  );
}

/** The time ranges Render's Sandboxes and Snapshots pages offer. */
export const TIME_RANGES = ["1h", "24h", "7d", "30d", "all"] as const;
export type TimeRange = (typeof TIME_RANGES)[number];
/** Render opens the Sandboxes page on the last 7 days. */
export const DEFAULT_TIME_RANGE: TimeRange = "7d";

export const TIME_RANGE_LABEL: Record<TimeRange, MessageKey> = {
  "1h": "sandboxGroupPages.rangeHour",
  "24h": "sandboxGroupPages.rangeDay",
  "7d": "sandboxGroupPages.rangeWeek",
  "30d": "sandboxGroupPages.rangeMonth",
  all: "sandboxGroupPages.rangeAll",
};

const RANGE_MS: Record<Exclude<TimeRange, "all">, number> = {
  "1h": 3_600_000,
  "24h": 86_400_000,
  "7d": 7 * 86_400_000,
  "30d": 30 * 86_400_000,
};

export function isTimeRange(value: unknown): value is TimeRange {
  return (
    typeof value === "string" &&
    (TIME_RANGES as readonly string[]).includes(value)
  );
}

/**
 * The start of a range as bex-api takes it (`createdAfter`), rounded down to
 * the minute so a poll does not ask a new question every second; undefined
 * for All time.
 */
export function rangeStart(range: TimeRange, now: number): string | undefined {
  if (range === "all") return undefined;
  const start = Math.floor((now - RANGE_MS[range]) / 60_000) * 60_000;
  return new Date(start).toISOString();
}

/**
 * Render's Errored share: of the runs that ended in the range, the share
 * that ended in an error, `<0.1%` below a tenth of a percent and none when
 * nothing ended.
 */
export function erroredShare(
  total: number,
  errored: number,
  language: string,
): string | null {
  if (total === 0) return null;
  const share = errored / total;
  if (errored > 0 && share < 0.001) return "<0.1%";
  return share.toLocaleString(language, {
    style: "percent",
    maximumFractionDigits: 1,
  });
}

/**
 * How long a sandbox ran: from when it started (or was created) to when it
 * ended, or to now while it lives. Null before it starts.
 */
export function sandboxDurationMs(
  sandbox: {
    createdAt: string | null;
    startedAt: string | null;
    endedAt: string | null;
  },
  now: number,
): number | null {
  const start = Date.parse(sandbox.startedAt ?? sandbox.createdAt ?? "");
  if (Number.isNaN(start)) return null;
  const end = sandbox.endedAt ? Date.parse(sandbox.endedAt) : now;
  return Math.max(0, end - start);
}

/** A time-out in seconds as Render words it ("Off" for none). */
export function formatSeconds(
  seconds: number | null | undefined,
  off: string,
): string {
  if (seconds == null) return "—";
  if (seconds === 0) return off;
  if (seconds % 3600 === 0) return `${seconds / 3600}h`;
  if (seconds % 60 === 0) return `${seconds / 60}m`;
  return `${seconds}s`;
}

/** Render's snapshot statuses: Capturing, Available, Failed. */
export const SNAPSHOT_STATUS_LABEL: Record<string, MessageKey> = {
  creating: "sandboxGroupPages.snapshotCapturing",
  available: "sandboxGroupPages.snapshotAvailable",
  failed: "sandboxGroupPages.snapshotFailed",
};

/** Render's failure text for each snapshot failureCode. */
export function snapshotFailureKey(code: string | null): MessageKey {
  switch (code) {
    case "DEADLINE_EXCEEDED":
      return "sandboxGroupPages.snapshotFailureTimeout";
    case "SOURCE_TERMINATED":
      return "sandboxGroupPages.snapshotFailureSourceTerminated";
    case "SOURCE_ERRORED":
      return "sandboxGroupPages.snapshotFailureSourceErrored";
    default:
      return "sandboxGroupPages.snapshotFailureInternal";
  }
}

/** A snapshot kept until it is deleted: `ttlSeconds: 0`, expiring 9999. */
export function snapshotNeverExpires(snapshot: {
  ttlSeconds: number | null;
  expiresAt: string | null;
}): boolean {
  return (
    snapshot.ttlSeconds === 0 || (snapshot.expiresAt ?? "").startsWith("9999-")
  );
}

/** Render's image statuses, by bex's build status. */
export const BUILD_STATUS_LABEL: Record<string, MessageKey> = {
  pending: "sandboxGroupPages.imageQueued",
  running: "sandboxGroupPages.imageImporting",
  succeeded: "sandboxGroupPages.imageAvailable",
  failed: "sandboxGroupPages.imageFailed",
};

export function isBuildInProgress(status: string): boolean {
  return status === "pending" || status === "running";
}

/**
 * An image name's status on the Custom images page, as Render reads it from
 * the name's builds: its newest import while that is in flight, else
 * Available once any version is, else Unavailable.
 */
export function imageSourceStatus(source: {
  latestBuild: { status: string };
  latestSucceededBuild: unknown;
}): MessageKey {
  const latest = source.latestBuild.status;
  if (isBuildInProgress(latest)) {
    return BUILD_STATUS_LABEL[latest];
  }
  return source.latestSucceededBuild
    ? "sandboxGroupPages.imageAvailable"
    : "sandboxGroupPages.imageUnavailable";
}

/**
 * Render's words for an import's failure reason, matched on the reason's
 * text as Render's dashboard matches it; null when none fits, and the
 * reason is shown as it is.
 */
export function imageFailureKey(reason: string | null): {
  key: MessageKey;
  params?: Record<string, string>;
} | null {
  if (!reason) return null;
  const text = reason.trim().replace(/\.$/, "");
  const lower = text.toLowerCase();
  if (lower.startsWith("deadlineexceeded"))
    return { key: "sandboxGroupPages.imageFailureTimeout" };
  if (lower.startsWith("backofflimitexceeded"))
    return { key: "sandboxGroupPages.imageFailureRetries" };
  if (lower.startsWith("fetch source image")) {
    if (/unauthorized|authentication required|denied/.test(lower))
      return { key: "sandboxGroupPages.imageFailureCredentials" };
    if (/manifest unknown|not found|name unknown/.test(lower))
      return { key: "sandboxGroupPages.imageFailureNotFound" };
    return { key: "sandboxGroupPages.imageFailurePull" };
  }
  const platform =
    /^source image is (\S+); only linux\/amd64 is supported/i.exec(text);
  if (platform)
    return {
      key: "sandboxGroupPages.imageFailurePlatform",
      params: { platform: platform[1] },
    };
  const missing = /^image lacks (.+), which sandbox boot requires/i.exec(text);
  if (missing)
    return {
      key: "sandboxGroupPages.imageFailureMissing",
      params: { what: missing[1] },
    };
  if (lower === "internal error converting the image")
    return { key: "sandboxGroupPages.imageFailureConvert" };
  if (lower.startsWith("read image tar"))
    return { key: "sandboxGroupPages.imageFailureLayers" };
  if (lower === "failed to start build")
    return { key: "sandboxGroupPages.imageFailureStart" };
  if (lower === "build did not finish")
    return { key: "sandboxGroupPages.imageFailureStopped" };
  return null;
}

/** A digest as Render shortens it: the first 7 hex digits of sha256. */
export function shortDigest(digest: string): string {
  const [algorithm, hex] = digest.split(":");
  if (!hex) return digest.slice(0, 14);
  return algorithm === "sha256"
    ? hex.slice(0, 7)
    : `${algorithm}:${hex.slice(0, 7)}`;
}

/** Render's lifecycle event labels, by lifecycleType. */
export const LIFECYCLE_LABEL: Record<string, MessageKey> = {
  CREATED: "sandboxGroupPages.eventCreated",
  READY: "sandboxGroupPages.eventReady",
  SUSPENDING: "sandboxGroupPages.eventSuspending",
  SUSPENDED: "sandboxGroupPages.eventSuspended",
  RESUMING: "sandboxGroupPages.eventResuming",
  TERMINATED: "sandboxGroupPages.eventTerminated",
  ERRORED: "sandboxGroupPages.eventErrored",
};

/** Render's trigger labels. */
export const TRIGGER_LABEL: Record<string, MessageKey> = {
  SDK: "sandboxGroupPages.triggerSdk",
  CLI: "sandboxGroupPages.triggerCli",
  API: "sandboxGroupPages.triggerApi",
  DASHBOARD: "sandboxGroupPages.triggerDashboard",
  IDLE_TIMEOUT: "sandboxGroupPages.triggerIdleTimeout",
  SYSTEM: "sandboxGroupPages.triggerSystem",
  CONNECTION: "sandboxGroupPages.triggerConnection",
};
