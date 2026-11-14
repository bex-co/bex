// Repository environment builds (ADR111): a build runs a repository's setup
// steps once and keeps the result as the snapshot its sessions boot from. The
// helpers here word a build's status, trigger and failure, and the refusals of
// Build now, from the server's stable codes.

import { ownValue } from "@/common/lib/own-value";
import {
  graphQLErrorCodes,
  graphQLErrorExtensions,
  isForbiddenError,
} from "@/common/lib/graphql-error";
import type { AgentRepoEnvironmentBuildFieldsFragment } from "@/graphql/definitions";

/** A build as the environment list, the history and Build now carry it (no log). */
export type RepoEnvironmentBuildView = AgentRepoEnvironmentBuildFieldsFragment;

/** One setup step of a build's log (ADR111 D1). */
export interface RepoEnvironmentBuildStepView {
  name: string;
  /** Null for a step that was killed or never started. */
  exitCode: number | null;
  durationMs: number;
  /** The step's redacted output tail. */
  output: string;
}

/**
 * How often a queued or building build is re-read. Builds take minutes, so
 * this is slower than the converging cadence; it runs only while one is
 * active (dashboard/AGENTS.md § Polling and loading).
 */
export const BUILD_POLL_INTERVAL_MS = 10_000;

/** ADR111 D2: a build is queued, building, success, failed or cancelled. */
export type RepoEnvironmentBuildStatus =
  "queued" | "building" | "success" | "failed" | "cancelled";

/** True while a build may still change on its own: queued or building. */
export function isActiveBuild(
  build: Pick<RepoEnvironmentBuildView, "status"> | null | undefined,
): boolean {
  return build?.status === "queued" || build?.status === "building";
}

/**
 * Whether a repository's environment has anything build-related to show: it
 * has setup steps (so it builds), or it built before its steps were removed.
 * A checks-only environment never builds (ADR111 D1).
 */
export function hasBuilds(environment: {
  setup: readonly unknown[];
  latestBuild: RepoEnvironmentBuildView | null;
}): boolean {
  return environment.setup.length > 0 || environment.latestBuild != null;
}

/**
 * The instant a build's row names: when it finished, else when it started,
 * else when it was queued.
 */
export function buildInstant(
  build: Pick<
    RepoEnvironmentBuildView,
    "createdAt" | "startedAt" | "finishedAt"
  >,
): string | null {
  return build.finishedAt ?? build.startedAt ?? build.createdAt ?? null;
}

/** "850ms" is not worth showing: whole seconds, then "4m 2s", then "1h 3m". */
export function formatBuildDuration(ms: number): string {
  const totalSeconds = ms > 0 ? Math.max(1, Math.floor(ms / 1000)) : 0;
  if (totalSeconds < 60) return `${totalSeconds}s`;
  const totalMinutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  if (totalMinutes < 60) {
    return seconds === 0 ? `${totalMinutes}m` : `${totalMinutes}m ${seconds}s`;
  }
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  return minutes === 0 ? `${hours}h` : `${hours}h ${minutes}m`;
}

/** A finished build's wall time, or null while it runs or when unrecorded. */
export function buildDurationMs(
  build: Pick<RepoEnvironmentBuildView, "startedAt" | "finishedAt">,
): number | null {
  if (!build.startedAt || !build.finishedAt) return null;
  const started = Date.parse(build.startedAt);
  const finished = Date.parse(build.finishedAt);
  if (Number.isNaN(started) || Number.isNaN(finished) || finished < started)
    return null;
  return finished - started;
}

const STATUS_KEYS: Record<RepoEnvironmentBuildStatus, string> = {
  queued: "agentEnvironment.repoEnvs.builds.status.queued",
  building: "agentEnvironment.repoEnvs.builds.status.building",
  success: "agentEnvironment.repoEnvs.builds.status.success",
  failed: "agentEnvironment.repoEnvs.builds.status.failed",
  cancelled: "agentEnvironment.repoEnvs.builds.status.cancelled",
};

/** The locale key of a build's status, or null for a status this dashboard does not know. */
export function buildStatusKey(status: string): string | null {
  return ownValue(STATUS_KEYS, status) ?? null;
}

const TRIGGER_KEYS: Record<string, string> = {
  environment_saved:
    "agentEnvironment.repoEnvs.builds.trigger.environmentSaved",
  secret_changed: "agentEnvironment.repoEnvs.builds.trigger.secretChanged",
  manual: "agentEnvironment.repoEnvs.builds.trigger.manual",
  refresh: "agentEnvironment.repoEnvs.builds.trigger.refresh",
};

/** The locale key of what queued a build (ADR111 D3), or null for an unknown trigger. */
export function buildTriggerKey(trigger: string): string | null {
  return ownValue(TRIGGER_KEYS, trigger) ?? null;
}

const FAILURE_KEYS: Record<string, string> = {
  setup_failed: "agentEnvironment.repoEnvs.builds.failure.setupFailed",
  step_timeout: "agentEnvironment.repoEnvs.builds.failure.stepTimeout",
  build_timeout: "agentEnvironment.repoEnvs.builds.failure.buildTimeout",
  checkout_failed: "agentEnvironment.repoEnvs.builds.failure.checkoutFailed",
  storage_limit_reached:
    "agentEnvironment.repoEnvs.builds.failure.storageLimitReached",
  internal_error: "agentEnvironment.repoEnvs.builds.failure.internalError",
};

/** The locale key wording a failed build's failureCode (ADR111 D2). */
export function buildFailureKey(code: string | null | undefined): string {
  return (
    (code ? ownValue(FAILURE_KEYS, code) : undefined) ??
    "agentEnvironment.repoEnvs.builds.failure.unknown"
  );
}

/** A refused Build now: what to say, and for the rate limit when to try again. */
export interface BuildNowRefusal {
  key: string;
  /** RFC 3339; set on AGENT_REPO_ENVIRONMENT_BUILD_RATE_LIMITED. */
  nextBuildAt: string | null;
}

const BUILD_NOW_ERROR_KEYS: Record<string, string> = {
  AGENT_REPO_ENVIRONMENT_NOTHING_TO_BUILD:
    "agentEnvironment.repoEnvs.builds.nothingToBuild",
  AGENT_REPO_ENVIRONMENT_NOT_FOUND: "agentEnvironment.repoEnvs.notFound",
  AGENT_REPO_ENVIRONMENT_BUILDS_UNAVAILABLE:
    "agentEnvironment.repoEnvs.builds.unavailable",
  AGENT_REPO_ENVIRONMENTS_UNAVAILABLE:
    "agentEnvironment.repoEnvs.builds.unavailable",
};

/** Words a refused Build now by the server's stable code (ADR111 D3, D10). */
export function buildNowRefusal(err: unknown): BuildNowRefusal {
  const limited = graphQLErrorExtensions(
    err,
    "AGENT_REPO_ENVIRONMENT_BUILD_RATE_LIMITED",
  );
  if (limited) {
    const next = limited["nextBuildAt"];
    const nextBuildAt =
      typeof next === "string" && !Number.isNaN(Date.parse(next)) ? next : null;
    return {
      key: nextBuildAt
        ? "agentEnvironment.repoEnvs.builds.rateLimitedUntil"
        : "agentEnvironment.repoEnvs.builds.rateLimited",
      nextBuildAt,
    };
  }
  for (const code of graphQLErrorCodes(err)) {
    const key = ownValue(BUILD_NOW_ERROR_KEYS, code);
    if (key) return { key, nextBuildAt: null };
  }
  return {
    key: isForbiddenError(err)
      ? "agentEnvironment.repoEnvs.builds.forbidden"
      : "agentEnvironment.repoEnvs.builds.buildFailed",
    nextBuildAt: null,
  };
}
