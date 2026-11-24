import { formatDuration } from "@/common/lib/format";
import type { BuildSourceBuildView } from "@/features/build-sources/types";

const TERMINAL_BUILD_STATUSES = new Set([
  "succeeded",
  "failed",
  "canceled",
  "build_failed",
  "live",
  "deactivated",
]);

export function isTerminalBuildStatus(status: string): boolean {
  return TERMINAL_BUILD_STATUSES.has(status);
}

export function isActiveBuildStatus(status: string): boolean {
  return status !== "" && !isTerminalBuildStatus(status);
}

export type BuildStatusTone = "success" | "destructive" | "secondary" | "outline";

export function buildStatusVariant(status: string): BuildStatusTone {
  switch (status) {
    case "succeeded":
    case "live":
      return "success";
    case "failed":
    case "build_failed":
      return "destructive";
    case "canceled":
      return "outline";
    default:
      return "secondary";
  }
}

export function buildStatusLabelKey(status: string): string {
  switch (status) {
    case "succeeded":
    case "live":
      return "buildSources.buildStatusSucceeded";
    case "failed":
    case "build_failed":
      return "buildSources.buildStatusFailed";
    case "canceled":
      return "buildSources.buildStatusCanceled";
    case "in_progress":
    case "build_in_progress":
      return "buildSources.buildStatusInProgress";
    case "created":
    case "queued":
      return "buildSources.buildStatusCreated";
    default:
      return "buildSources.buildStatusUnknown";
  }
}

export function buildDurationValue(
  build: Pick<BuildSourceBuildView, "buildStartedAt" | "buildFinishedAt">,
): string | null {
  if (!build.buildStartedAt || !build.buildFinishedAt) return null;
  const start = Date.parse(build.buildStartedAt);
  const end = Date.parse(build.buildFinishedAt);
  if (!Number.isFinite(start) || !Number.isFinite(end) || end < start) {
    return null;
  }
  return formatDuration(end - start);
}

/** "sha256:" plus the first 12 hex digits, Docker's short image id length. */
export function shortBuildDigest(digest: string): string {
  const [algorithm, hex] = digest.split(":");
  return hex ? `${algorithm}:${hex.slice(0, 12)}` : digest;
}

/** Standard destructive-confirmation phrase (dashboard/AGENTS.md § Destructive confirmations). */
export function buildSourceSudoDeletePhrase(name: string): string {
  return `sudo delete build source ${name}`;
}

/** Localizes known skipped-service reasons from TriggerBuild / BuildDetail. */
export function skippedReasonLabel(
  reason: string,
  t: (key: "buildSources.skippedReasonSuspended") => string,
): string {
  return reason === "suspended"
    ? t("buildSources.skippedReasonSuspended")
    : reason;
}
