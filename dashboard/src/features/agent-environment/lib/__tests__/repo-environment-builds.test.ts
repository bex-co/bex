import { describe, expect, it } from "vitest";
import { codedGraphQLError } from "@/test/mocks/apollo";
import {
  buildDurationMs,
  buildFailureKey,
  buildInstant,
  buildNowRefusal,
  buildStatusKey,
  buildTriggerKey,
  formatBuildDuration,
  hasBuilds,
  isActiveBuild,
  type RepoEnvironmentBuildView,
} from "@/features/agent-environment/lib/repo-environment-builds";

function build(over: Partial<RepoEnvironmentBuildView> = {}) {
  return {
    __typename: "AgentRepoEnvironmentBuild",
    id: "arb-1",
    ownerId: "tea-1",
    repo: "acme/api",
    version: "v1",
    trigger: "manual",
    status: "success",
    failureCode: null,
    failureReason: null,
    headSha: null,
    sizeBytes: null,
    imageAvailable: true,
    createdAt: "2026-11-14T09:00:00Z",
    startedAt: "2026-11-14T09:01:00Z",
    finishedAt: "2026-11-14T09:05:30Z",
    ...over,
  } satisfies RepoEnvironmentBuildView;
}

describe("repository environment builds (ADR111)", () => {
  it("counts only queued and building builds as active", () => {
    expect(isActiveBuild(build({ status: "queued" }))).toBe(true);
    expect(isActiveBuild(build({ status: "building" }))).toBe(true);
    for (const status of ["success", "failed", "cancelled", "partial"])
      expect(isActiveBuild(build({ status }))).toBe(false);
    expect(isActiveBuild(null)).toBe(false);
  });

  // A checks-only environment never builds (ADR111 D1).
  it("shows builds for setup steps, or for a build made before they were removed", () => {
    expect(hasBuilds({ setup: [{}], latestBuild: null })).toBe(true);
    expect(hasBuilds({ setup: [], latestBuild: null })).toBe(false);
    expect(hasBuilds({ setup: [], latestBuild: build() })).toBe(true);
  });

  it("dates a build by its finish, else its start, else its queueing", () => {
    expect(buildInstant(build())).toBe("2026-11-14T09:05:30Z");
    expect(buildInstant(build({ finishedAt: null }))).toBe(
      "2026-11-14T09:01:00Z",
    );
    expect(buildInstant(build({ finishedAt: null, startedAt: null }))).toBe(
      "2026-11-14T09:00:00Z",
    );
  });

  it("measures a finished build and formats durations compactly", () => {
    expect(buildDurationMs(build())).toBe(270_000);
    expect(buildDurationMs(build({ finishedAt: null }))).toBeNull();
    expect(
      buildDurationMs(build({ finishedAt: "2026-11-14T09:00:00Z" })),
    ).toBeNull();
    expect(formatBuildDuration(0)).toBe("0s");
    expect(formatBuildDuration(400)).toBe("1s");
    expect(formatBuildDuration(59_999)).toBe("59s");
    expect(formatBuildDuration(60_000)).toBe("1m");
    expect(formatBuildDuration(270_000)).toBe("4m 30s");
    expect(formatBuildDuration(3 * 3_600_000)).toBe("3h");
    expect(formatBuildDuration(3_660_000 + 5_000)).toBe("1h 1m");
  });

  it("words every status, trigger and failure code, and nothing inherited", () => {
    expect(
      ["queued", "building", "success", "failed", "cancelled"].map(
        buildStatusKey,
      ),
    ).toEqual([
      "agentEnvironment.repoEnvs.builds.status.queued",
      "agentEnvironment.repoEnvs.builds.status.building",
      "agentEnvironment.repoEnvs.builds.status.success",
      "agentEnvironment.repoEnvs.builds.status.failed",
      "agentEnvironment.repoEnvs.builds.status.cancelled",
    ]);
    expect(buildStatusKey("toString")).toBeNull();
    expect(
      ["environment_saved", "secret_changed", "manual", "refresh"].map(
        buildTriggerKey,
      ),
    ).toEqual([
      "agentEnvironment.repoEnvs.builds.trigger.environmentSaved",
      "agentEnvironment.repoEnvs.builds.trigger.secretChanged",
      "agentEnvironment.repoEnvs.builds.trigger.manual",
      "agentEnvironment.repoEnvs.builds.trigger.refresh",
    ]);
    expect(buildTriggerKey("constructor")).toBeNull();
    expect(
      [
        "setup_failed",
        "step_timeout",
        "build_timeout",
        "checkout_failed",
        "storage_limit_reached",
        "internal_error",
        "something_new",
        null,
      ].map(buildFailureKey),
    ).toEqual([
      "agentEnvironment.repoEnvs.builds.failure.setupFailed",
      "agentEnvironment.repoEnvs.builds.failure.stepTimeout",
      "agentEnvironment.repoEnvs.builds.failure.buildTimeout",
      "agentEnvironment.repoEnvs.builds.failure.checkoutFailed",
      "agentEnvironment.repoEnvs.builds.failure.storageLimitReached",
      "agentEnvironment.repoEnvs.builds.failure.internalError",
      "agentEnvironment.repoEnvs.builds.failure.unknown",
      "agentEnvironment.repoEnvs.builds.failure.unknown",
    ]);
  });

  it("maps each Build now refusal by its code", () => {
    expect(
      buildNowRefusal(
        codedGraphQLError("AGENT_REPO_ENVIRONMENT_BUILD_RATE_LIMITED", {
          nextBuildAt: "2026-11-15T08:30:00Z",
        }),
      ),
    ).toEqual({
      key: "agentEnvironment.repoEnvs.builds.rateLimitedUntil",
      nextBuildAt: "2026-11-15T08:30:00Z",
    });
    // An unusable instant falls back to the copy that names no time.
    expect(
      buildNowRefusal(
        codedGraphQLError("AGENT_REPO_ENVIRONMENT_BUILD_RATE_LIMITED", {
          nextBuildAt: "soon",
        }),
      ),
    ).toEqual({
      key: "agentEnvironment.repoEnvs.builds.rateLimited",
      nextBuildAt: null,
    });
    const key = (err: unknown) => buildNowRefusal(err).key;
    expect(
      key(codedGraphQLError("AGENT_REPO_ENVIRONMENT_NOTHING_TO_BUILD")),
    ).toBe("agentEnvironment.repoEnvs.builds.nothingToBuild");
    expect(key(codedGraphQLError("AGENT_REPO_ENVIRONMENT_NOT_FOUND"))).toBe(
      "agentEnvironment.repoEnvs.notFound",
    );
    expect(
      key(codedGraphQLError("AGENT_REPO_ENVIRONMENT_BUILDS_UNAVAILABLE")),
    ).toBe("agentEnvironment.repoEnvs.builds.unavailable");
    expect(key(codedGraphQLError("AGENT_REPO_ENVIRONMENTS_UNAVAILABLE"))).toBe(
      "agentEnvironment.repoEnvs.builds.unavailable",
    );
    expect(key(codedGraphQLError("FORBIDDEN"))).toBe(
      "agentEnvironment.repoEnvs.builds.forbidden",
    );
    expect(key(new Error("Failed to fetch"))).toBe(
      "agentEnvironment.repoEnvs.builds.buildFailed",
    );
  });
});
