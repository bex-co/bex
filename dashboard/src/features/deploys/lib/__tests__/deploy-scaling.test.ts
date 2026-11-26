import { describe, expect, it } from "vitest";
import {
  deployScaling,
  isInProgressDeployStatus,
  scalesOut,
  scalingShows,
  type ScalingInstance,
} from "../deploy-scaling";

// Render's Scaling count (index's P$, docs/render-artifacts/
// deploy-detail-page.md § Scaling): only this deploy's running or succeeded
// instances count, out of the autoscaling minimum or the instance count.
const instance = (
  deployId: string | null,
  status: string | null,
): ScalingInstance => ({
  deployId,
  status,
});

describe("deployScaling", () => {
  it.each([
    [
      "no new instance yet",
      [instance("dep-old", "running"), instance("dep-old", "running")],
      0,
    ],
    [
      "the first new instance running",
      [instance("dep-new", "running"), instance("dep-old", "running")],
      1,
    ],
    [
      "both new instances running",
      [instance("dep-new", "running"), instance("dep-new", "running")],
      2,
    ],
  ])(
    "counts %s of a 2-instance service's rollout",
    (_, instances, deployed) => {
      expect(
        deployScaling({
          deployId: "dep-new",
          replicas: 2,
          autoscaling: null,
          instances,
        }),
      ).toEqual({ deployed, total: 2 });
    },
  );

  it("counts only running and succeeded instances of this deploy", () => {
    const instances = [
      instance("dep-new", "running"),
      instance("dep-new", "succeeded"),
      instance("dep-new", "pending"),
      instance("dep-new", "failed"),
      instance("dep-new", "unknown"),
      instance("dep-old", "running"),
      instance(null, "running"),
      instance("dep-new", null),
    ];
    expect(
      deployScaling({
        deployId: "dep-new",
        replicas: 4,
        autoscaling: null,
        instances,
      }),
    ).toEqual({ deployed: 2, total: 4 });
  });

  it("counts an autoscaled service against its minimum, never fewer than run", () => {
    const autoscaling = { enabled: true, minInstances: 3 };
    const three = [instance("dep-1", "running"), instance("dep-1", "running")];
    expect(
      deployScaling({
        deployId: "dep-1",
        replicas: 1,
        autoscaling,
        instances: three,
      }),
    ).toEqual({ deployed: 2, total: 3 });
    const scaledUp = Array.from({ length: 5 }, () =>
      instance("dep-1", "running"),
    );
    expect(
      deployScaling({
        deployId: "dep-1",
        replicas: 1,
        autoscaling,
        instances: scaledUp,
      }),
    ).toEqual({ deployed: 5, total: 5 });
    // Render's minimum defaults to one.
    expect(
      deployScaling({
        deployId: "dep-1",
        replicas: 4,
        autoscaling: { enabled: true, minInstances: null },
        instances: [],
      }),
    ).toEqual({ deployed: 0, total: 1 });
  });

  it("is null for a service that does not scale out, or before its instances load", () => {
    const instances = [instance("dep-1", "running")];
    expect(
      deployScaling({
        deployId: "dep-1",
        replicas: 1,
        autoscaling: null,
        instances,
      }),
    ).toBeNull();
    expect(
      deployScaling({
        deployId: "dep-1",
        replicas: 1,
        autoscaling: { enabled: false, minInstances: 3 },
        instances,
      }),
    ).toBeNull();
    expect(
      deployScaling({
        deployId: "dep-1",
        replicas: 2,
        autoscaling: null,
        instances: null,
      }),
    ).toBeNull();
  });
});

describe("scalesOut", () => {
  it.each([
    [1, null, false],
    [null, null, false],
    [2, null, true],
    [1, { enabled: true, minInstances: 1 }, true],
    [1, { enabled: false, minInstances: 2 }, false],
  ] as const)(
    "replicas %s, autoscaling %j: %s",
    (replicas, autoscaling, want) => {
      expect(scalesOut(replicas, autoscaling)).toBe(want);
    },
  );
});

describe("scalingShows", () => {
  it.each([
    ["live", true],
    ["queued", true],
    ["created", true],
    ["build_in_progress", true],
    ["pre_deploy_in_progress", true],
    ["update_in_progress", true],
    ["deactivated", false],
    ["update_failed", false],
    ["build_failed", false],
    ["pre_deploy_failed", false],
    ["canceled", false],
  ])("on a %s deploy opened afresh: %s", (status, want) => {
    expect(scalingShows(status, false)).toBe(want);
  });

  it("keeps showing on a deploy the page watched in progress", () => {
    expect(scalingShows("update_failed", true)).toBe(true);
    expect(scalingShows("canceled", true)).toBe(true);
  });

  it("watches every open status but the queue", () => {
    expect(isInProgressDeployStatus("queued")).toBe(false);
    expect(isInProgressDeployStatus("update_in_progress")).toBe(true);
    expect(isInProgressDeployStatus("created")).toBe(true);
    expect(isInProgressDeployStatus("live")).toBe(false);
  });
});
