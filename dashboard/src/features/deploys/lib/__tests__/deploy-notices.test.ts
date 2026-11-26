import { describe, expect, it } from "vitest";
import type { DeployView } from "@/features/deploys/hooks/use-deploy";
import {
  deployNotices,
  reservedPortWarning,
  type DeployNoticeFacts,
} from "../deploy-notices";

// The deploy page's Notices (w1/m659), as Render's DeployContainer lists
// them: docs/render-artifacts/deploy-detail-page.md § Notices.

function deploy(over: Partial<DeployView> = {}): DeployView {
  return {
    id: "dep-1",
    status: "live",
    trigger: "manual",
    image: "",
    imageSha: "",
    rollbackOf: "",
    buildAvailable: null,
    commitId: "",
    commitMessage: "",
    commitCreatedAt: null,
    createdAt: "2026-11-26T00:00:00Z",
    updatedAt: "2026-11-26T00:02:00Z",
    startedAt: null,
    finishedAt: "2026-11-26T00:02:00Z",
    preDeployStatus: "",
    initialDeployHookStatus: "",
    initialDeployHookFinishedAt: null,
    failureReason: "",
    cancelReason: "",
    stallReason: "",
    failureReasonTemplate: null,
    stallReasonTemplate: null,
    cancelReasonTemplate: null,
    ...over,
  };
}

const none: DeployNoticeFacts = { policy: null };
const kinds = (d: DeployView, facts: DeployNoticeFacts = none) =>
  deployNotices(d, facts).map((n) => n.kind);
const supersede = {
  code: "superseded_by",
  params: { deploy: "dep-newer" },
  parts: [],
  remainder: "",
};

describe("deployNotices", () => {
  it("says nothing about a plain live deploy", () => {
    expect(deployNotices(deploy(), none)).toEqual([]);
  });

  it("lists every fact that holds, in Render's order", () => {
    const d = deploy({
      status: "update_failed",
      trigger: "blueprint",
      failureReason: "the build failed",
      clearCache: true,
      isTransitionToFreeTier: true,
      rollbackOf: "dep-target",
      blueprintId: "exs-abc",
      stallReason: "waiting",
      timedOutSeconds: 900,
    });
    expect(
      kinds(d, { healthCheckPath: "/healthz", port: 10000, policy: null }),
    ).toEqual([
      "failure",
      "buildCacheCleared",
      "freePlan",
      "rolledBackTo",
      "syncedFrom",
      "healthCheck",
      "stall",
    ]);
  });

  it("names a platform rebuild as maintenance", () => {
    expect(kinds(deploy({ trigger: "deployed_by_render" }))).toEqual([
      "maintenance",
    ]);
  });

  it("leaves the troubleshooting docs off a platform error", () => {
    const failure = (code: string) =>
      deployNotices(
        deploy({
          status: "update_failed",
          failureReason: "x",
          failureReasonTemplate: { code, params: {}, parts: [], remainder: "" },
        }),
        none,
      )[0];
    expect(failure("platform_error")).toEqual({
      kind: "failure",
      troubleshoot: false,
    });
    expect(failure("health_timed_out")).toEqual({
      kind: "failure",
      troubleshoot: true,
    });
  });

  it("names a gone build only on a past deploy", () => {
    expect(
      kinds(deploy({ status: "deactivated", buildAvailable: false })),
    ).toEqual(["buildGone"]);
    expect(
      kinds(deploy({ status: "deactivated", buildAvailable: true })),
    ).toEqual([]);
    expect(
      kinds(deploy({ status: "deactivated", buildAvailable: null })),
    ).toEqual([]);
    expect(kinds(deploy({ status: "live", buildAvailable: false }))).toEqual(
      [],
    );
  });

  it("names a sync only on a sync's deploy that recorded its Blueprint", () => {
    expect(kinds(deploy({ trigger: "blueprint" }))).toEqual([]);
    expect(kinds(deploy({ trigger: "manual", blueprintId: "exs-a" }))).toEqual(
      [],
    );
    expect(
      deployNotices(
        deploy({ trigger: "blueprint", blueprintId: "exs-a" }),
        none,
      ),
    ).toEqual([{ kind: "syncedFrom", blueprintId: "exs-a" }]);
  });

  it("links the overlapping deploy policy only under the policy that caused it", () => {
    const canceled = deploy({
      status: "canceled",
      cancelReason: "Superseded by dep-newer",
      cancelReasonTemplate: supersede,
    });
    expect(deployNotices(canceled, { policy: "override" })).toEqual([
      { kind: "cancel", supersede: true, policyLink: true },
    ]);
    expect(deployNotices(canceled, { policy: "wait" })).toEqual([
      { kind: "cancel", supersede: true, policyLink: false },
    ]);
    const suspended = deploy({
      status: "canceled",
      cancelReason: "Canceled: the service was suspended",
      cancelReasonTemplate: { ...supersede, code: "canceled_suspended" },
    });
    expect(deployNotices(suspended, { policy: "override" })).toEqual([
      { kind: "cancel", supersede: false, policyLink: false },
    ]);

    const queued = deploy({ status: "queued" });
    expect(
      deployNotices(queued, {
        inProgressDeployId: "dep-ahead",
        policy: "wait",
      }),
    ).toEqual([{ kind: "queued", deployId: "dep-ahead", policyLink: true }]);
    expect(
      deployNotices(queued, {
        inProgressDeployId: "dep-ahead",
        policy: "override",
      }),
    ).toEqual([{ kind: "queued", deployId: "dep-ahead", policyLink: false }]);
    // No deploy in progress found: Render shows nothing either.
    expect(deployNotices(queued, { policy: "wait" })).toEqual([]);
    // A deploy no longer queued no longer waits.
    expect(
      kinds(deploy({ status: "build_in_progress" }), {
        inProgressDeployId: "dep-ahead",
        policy: "wait",
      }),
    ).toEqual([]);
  });

  it("names the health check while rolling out and once it timed out, only with a path", () => {
    const facts = { healthCheckPath: "/healthz", port: 8080, policy: null };
    expect(
      deployNotices(deploy({ status: "update_in_progress" }), facts),
    ).toEqual([
      { kind: "healthCheck", timedOut: false, address: "0.0.0.0:8080/healthz" },
    ]);
    expect(
      deployNotices(deploy({ status: "update_failed", timedOutSeconds: 900 }), {
        ...facts,
        port: null,
      }),
    ).toEqual([{ kind: "healthCheck", timedOut: true, address: null }]);
    expect(kinds(deploy({ status: "update_failed" }), facts)).toEqual([]);
    expect(kinds(deploy({ status: "build_in_progress" }), facts)).toEqual([]);
    expect(
      kinds(deploy({ status: "update_in_progress" }), {
        ...facts,
        healthCheckPath: "",
      }),
    ).toEqual([]);
  });
});

describe("reservedPortWarning", () => {
  it("warns on a free plan while the deploy rolls out or once it failed", () => {
    const d = (status: string) => deploy({ status, reservedPort: 18012 });
    expect(reservedPortWarning(d("update_in_progress"), "free")).toBe(18012);
    expect(reservedPortWarning(d("update_failed"), "free")).toBe(18012);
    expect(reservedPortWarning(d("build_failed"), "free")).toBe(18012);
    expect(reservedPortWarning(d("live"), "free")).toBeNull();
    expect(reservedPortWarning(d("update_in_progress"), "starter")).toBeNull();
    expect(reservedPortWarning(d("update_in_progress"), null)).toBeNull();
    expect(
      reservedPortWarning(deploy({ status: "update_failed" }), "free"),
    ).toBeNull();
  });
});
