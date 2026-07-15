import { describe, expect, it } from "vitest";
import type { DeployView } from "@/features/deploys/hooks/use-deploy";
import type { DeployTimelineEvent } from "@/features/deploys/hooks/use-deploy-timeline";
import { buildDeployTimeline } from "../deploy-timeline";

function deploy(over: Partial<DeployView> = {}): DeployView {
  return {
    id: "dep-1",
    status: "live",
    trigger: "api",
    image: "",
    rollbackOf: "",
    createdAt: "2026-07-14T00:00:00Z",
    startedAt: null,
    finishedAt: "2026-07-14T00:02:00Z",
    preDeployStatus: "",
    ...over,
  };
}

function event(over: Partial<DeployTimelineEvent>): DeployTimelineEvent {
  return {
    id: "evt-1",
    type: "deploy_started",
    timestamp: "2026-07-14T00:00:01Z",
    deployId: "dep-1",
    deployStatus: "",
    ...over,
  };
}

describe("buildDeployTimeline", () => {
  it("uses matching service-event timestamps with row timestamps as fallback", () => {
    const steps = buildDeployTimeline(deploy(), [
      event({ type: "deploy_started", timestamp: "2026-07-14T00:00:01Z" }),
      event({
        id: "evt-2",
        type: "deploy_ended",
        timestamp: "2026-07-14T00:01:59Z",
        deployStatus: "succeeded",
      }),
    ]);

    expect(steps).toEqual([
      {
        id: "created",
        kind: "created",
        timestamp: "2026-07-14T00:00:01Z",
      },
      {
        id: "terminal",
        kind: "live",
        timestamp: "2026-07-14T00:01:59Z",
        status: "live",
      },
    ]);
  });

  it.each([
    ["update_failed", "failed"],
    ["canceled", "canceled"],
  ] as const)("renders %s as its honest terminal state", (status, kind) => {
    expect(buildDeployTimeline(deploy({ status }), [])).toEqual([
      {
        id: "created",
        kind: "created",
        timestamp: "2026-07-14T00:00:00Z",
      },
      {
        id: "terminal",
        kind,
        timestamp: "2026-07-14T00:02:00Z",
        status,
      },
    ]);
  });

  it("does not fabricate build or started phases when bex did not record them", () => {
    const steps = buildDeployTimeline(
      deploy({
        status: "update_in_progress",
        startedAt: null,
        finishedAt: null,
      }),
      [],
    );

    expect(steps.map((step) => step.kind)).toEqual(["created", "in_progress"]);
    expect(steps.some((step) => step.kind === "started")).toBe(false);
  });

  it("shows started only when the deploy row has a real startedAt", () => {
    const steps = buildDeployTimeline(
      deploy({
        status: "update_in_progress",
        startedAt: "2026-07-14T00:00:05Z",
        finishedAt: null,
      }),
      [],
    );

    expect(steps.map((step) => step.kind)).toEqual([
      "created",
      "started",
      "in_progress",
    ]);
  });
});
