import { describe, it, expect } from "vitest";
import {
  bulkLifecycleReason,
  capabilitiesFromState,
  lifecycleTargets,
  runBounded,
  suspendPhrase,
  targetsFor,
  type LifecycleTarget,
} from "@/features/projects/lib/bulk-lifecycle";
import { toSnapshot } from "@/features/capabilities/lib/capability-policy";
import { mockCapabilities } from "@/test/mocks/capabilities";
import type { ResourceRow } from "@/features/projects/types";
import type { ServiceView } from "@/features/services/types";

const t = (key: string) => key;

function serviceRow(id: string, suspended: boolean): ResourceRow {
  return {
    kind: "service",
    id,
    name: id,
    createdAt: null,
    updatedAt: null,
    runtime: null,
    region: null,
    service: { id, name: id, suspended } as ServiceView,
  };
}

function target(
  kind: LifecycleTarget["kind"],
  id: string,
  suspended = false,
): LifecycleTarget {
  return { key: `${kind}:${id}`, kind, id, name: id, suspended };
}

describe("lifecycleTargets", () => {
  it("acts on services, Postgres and Key Value, never an environment group", () => {
    const rows: ResourceRow[] = [
      serviceRow("web", false),
      {
        ...serviceRow("db", false),
        kind: "database",
        service: undefined,
        database: {
          id: "db",
          name: "db",
          suspended: "suspended",
        } as ResourceRow["database"],
      },
      {
        ...serviceRow("cache", false),
        kind: "keyvalue",
        service: undefined,
        keyValue: {
          id: "cache",
          name: "cache",
          suspended: false,
        } as ResourceRow["keyValue"],
      },
      { ...serviceRow("shared", false), kind: "envgroup", service: undefined },
    ];
    expect(lifecycleTargets(rows)).toEqual([
      target("service", "web", false),
      target("database", "db", true),
      target("keyvalue", "cache", false),
    ]);
  });
});

describe("targetsFor", () => {
  // Render's dialog lists only the rows an action changes; a mixed selection
  // offers both actions over disjoint halves.
  it("splits a mixed selection between Suspend and Resume", () => {
    const mixed = [
      target("service", "web"),
      target("service", "worker", true),
      target("service", "site"),
    ];
    expect(targetsFor("suspend", mixed).map((t) => t.id)).toEqual([
      "web",
      "site",
    ]);
    expect(targetsFor("resume", mixed).map((t) => t.id)).toEqual(["worker"]);
  });
});

describe("suspendPhrase", () => {
  it("counts databases apart and Key Value as a service, as Render does", () => {
    expect(suspendPhrase([target("service", "a")])).toBe(
      "sudo suspend 1 service",
    );
    expect(
      suspendPhrase([target("service", "a"), target("keyvalue", "b")]),
    ).toBe("sudo suspend 2 services");
    expect(
      suspendPhrase([target("service", "a"), target("database", "b")]),
    ).toBe("sudo suspend 1 service and 1 database");
    expect(
      suspendPhrase([target("database", "a"), target("database", "b")]),
    ).toBe("sudo suspend 2 databases");
  });
});

describe("bulkLifecycleReason", () => {
  const developer = mockCapabilities({ role: "DEVELOPER", canManage: false });
  const viewer = mockCapabilities({
    role: "VIEWER",
    canOperate: false,
    canCreate: false,
    canManage: false,
  });

  it("lets an Admin act anywhere and a Developer outside protected environments", () => {
    expect(bulkLifecycleReason(mockCapabilities(), true, t)).toBeUndefined();
    expect(bulkLifecycleReason(developer, false, t)).toBeUndefined();
  });

  it("limits a protected environment to Admins with Render's copy", () => {
    expect(bulkLifecycleReason(developer, true, t)).toBe(
      "projects.bulkAdminOnly",
    );
  });

  it("gives a Viewer the role reason in every environment", () => {
    expect(bulkLifecycleReason(viewer, false, t)).toBe(
      "capabilities.reasonCanOperate",
    );
    expect(bulkLifecycleReason(viewer, true, t)).toBe(
      "capabilities.reasonCanOperate",
    );
  });

  it("never enables while the role is still unknown", () => {
    const checking = mockCapabilities({
      loaded: false,
      canOperate: false,
      canManage: false,
    });
    expect(bulkLifecycleReason(checking, false, t)).toBe(
      "capabilities.actionChecking",
    );
  });

  it("reads a fresh answer by the same rule (capabilitiesFromState)", () => {
    const answer = (grants: Record<string, string>) =>
      capabilitiesFromState(
        {
          status: "ready",
          snapshot: toSnapshot(
            "tea-1",
            Object.entries(grants).map(([action, outcome]) => ({
              action,
              outcome,
              reason: null,
            })),
          ),
        },
        "tea-1",
      );
    const demoted = answer({ can_operate: "allowed", can_manage: "denied" });
    expect(bulkLifecycleReason(demoted, false, t)).toBeUndefined();
    expect(bulkLifecycleReason(demoted, true, t)).toBe(
      "projects.bulkAdminOnly",
    );
    expect(
      bulkLifecycleReason(answer({ can_operate: "denied" }), false, t),
    ).toBe("capabilities.reasonCanOperate");
    expect(
      bulkLifecycleReason(
        capabilitiesFromState({ status: "unavailable" }, "tea-1"),
        false,
        t,
      ),
    ).toBe("capabilities.grantUnavailable");
  });
});

describe("runBounded", () => {
  it("runs every item with at most the limit in flight", async () => {
    let inFlight = 0;
    let peak = 0;
    const seen: number[] = [];
    const items = Array.from({ length: 25 }, (_, i) => i);
    await runBounded(items, 10, async (item) => {
      inFlight++;
      peak = Math.max(peak, inFlight);
      await new Promise((resolve) => setTimeout(resolve, item % 3));
      seen.push(item);
      inFlight--;
    });
    expect(peak).toBe(10);
    expect(seen.sort((a, b) => a - b)).toEqual(items);
  });

  it("does nothing for an empty batch", async () => {
    let calls = 0;
    await runBounded([], 10, async () => {
      calls++;
    });
    expect(calls).toBe(0);
  });
});
