import { describe, it, expect } from "vitest";
import {
  toServiceView,
  deriveStatus,
  computeStats,
  isSuspended,
  isSleeping,
} from "@/features/services/lib/status";
import type { ServicesQuery, ServerQuery } from "@/graphql/definitions";
import type { ServiceView } from "@/features/services/types";

type ServiceNode = NonNullable<NonNullable<ServicesQuery["services"]>[number]>;
type ServerNode = NonNullable<ServerQuery["server"]>;

function node(overrides: Partial<ServiceNode> = {}): ServiceNode {
  return {
    __typename: "Service",
    id: "app",
    name: "app",
    displayName: null,
    type: "web_service",
    suspended: "not_suspended",
    dashboardUrl: "https://app.onbex.co",
    url: "https://app.onbex.co",
    createdAt: "2026-01-01T00:00:00Z",
    phase: "Running",
    replicas: 1,
    revision: "abc123",
    plan: null,
    idleTTLSeconds: 0,
    ...overrides,
  };
}

function svc(overrides: Partial<ServiceView> = {}): ServiceView {
  return {
    id: "app",
    name: "app",
    type: "web_service",
    suspended: false,
    phase: "Running",
    url: "https://app.onbex.co",
    createdAt: "2026-01-01T00:00:00Z",
    replicas: 1,
    revision: "abc123",
    plan: null,
    idleTTLSeconds: null,
    schedule: null,
    command: null,
    runs: [],
    repo: null,
    branch: null,
    rootDir: null,
    ...overrides,
  };
}

describe("isSuspended", () => {
  it("reads Render's string enum, not a boolean", () => {
    expect(isSuspended("suspended")).toBe(true);
    expect(isSuspended("not_suspended")).toBe(false);
    expect(isSuspended(null)).toBe(false);
    // guards against a boolean-shaped value sneaking in
    expect(isSuspended("true")).toBe(false);
  });
});

describe("toServiceView", () => {
  it("normalizes the wire Service, decoding the suspended enum to a bool", () => {
    expect(
      toServiceView(node({ suspended: "suspended", phase: "Hibernated" })),
    ).toEqual({
      id: "app",
      name: "app",
      slug: null,
      displayName: null,
      type: "web_service",
      suspended: true,
      phase: "Hibernated",
      url: "https://app.onbex.co",
      createdAt: "2026-01-01T00:00:00Z",
      replicas: 1,
      revision: "abc123",
      plan: null,
      idleTTLSeconds: 0,
      schedule: null,
      command: null,
      runs: [],
      repo: null,
      branch: null,
      rootDir: null,
      buildFilter: null,
      autoDeploy: null,
      notifyOnFail: null,
      healthCheckPath: null,
      preDeployCommand: null,
      publishPath: null,
      routes: [],
      headers: [],
    });
  });

  it("carries the plan through when the wire Service has one", () => {
    expect(toServiceView(node({ plan: "pro_plus" })).plan).toBe("pro_plus");
  });

  it("falls back to id for a missing name and null for a missing url", () => {
    const v = toServiceView(node({ name: null, url: null, id: "svc-1" }));
    expect(v.name).toBe("svc-1");
    expect(v.url).toBeNull();
  });

  it("uses displayName for humans while preserving the immutable id", () => {
    const v = toServiceView(
      node({ id: "stable-id", name: "stable-id", displayName: "Customer API" }),
    );
    expect(v.id).toBe("stable-id");
    expect(v.name).toBe("Customer API");
    expect(v.displayName).toBe("Customer API");
  });

  it("leaves schedule/command null / runs empty for a list node (no cron fields selected)", () => {
    const v = toServiceView(node({ type: "web_service" }));
    expect(v.type).toBe("web_service");
    expect(v.schedule).toBeNull();
    expect(v.command).toBeNull();
    expect(v.runs).toEqual([]);
  });

  it("maps a build-from-git server node's repo/branch/rootDir", () => {
    const serverNode: ServerNode = {
      __typename: "Service",
      id: "mono",
      name: "mono",
      type: "web_service",
      suspended: "not_suspended",
      dashboardUrl: null,
      url: null,
      createdAt: null,
      phase: "Running",
      replicas: 1,
      revision: null,
      plan: null,
      idleTTLSeconds: 0,
      repo: "https://github.com/x/mono",
      branch: "main",
      rootDir: "backend",
      schedule: null,
      command: null,
      runs: [],
    };
    const v = toServiceView(serverNode);
    expect(v.repo).toBe("https://github.com/x/mono");
    expect(v.branch).toBe("main");
    expect(v.rootDir).toBe("backend");
  });

  it("leaves repo/branch/rootDir null for a list node (no build fields selected)", () => {
    const v = toServiceView(node());
    expect(v.repo).toBeNull();
    expect(v.branch).toBeNull();
    expect(v.rootDir).toBeNull();
  });

  it("reads slug from a detail server node, incl. the random-suffix case", () => {
    const serverNode: ServerNode = {
      __typename: "Service",
      id: "web",
      name: "web",
      slug: "web-a1b2",
      type: "web_service",
      suspended: "not_suspended",
      dashboardUrl: null,
      url: null,
      createdAt: null,
      phase: "Running",
      replicas: 1,
      revision: null,
      plan: null,
      idleTTLSeconds: 0,
      repo: null,
      branch: null,
      rootDir: null,
      schedule: null,
      command: null,
      runs: [],
    };
    expect(toServiceView(serverNode).slug).toBe("web-a1b2");
  });

  it("leaves slug null for a list node (not selected by the services query)", () => {
    expect(toServiceView(node()).slug).toBeNull();
  });

  it("maps a cron server node's schedule and run history", () => {
    const serverNode: ServerNode = {
      __typename: "Service",
      id: "nightly",
      name: "nightly",
      type: "cron_job",
      suspended: "not_suspended",
      dashboardUrl: null,
      url: null,
      createdAt: null,
      phase: "Running",
      replicas: null,
      revision: null,
      plan: null,
      idleTTLSeconds: 0,
      repo: null,
      branch: null,
      rootDir: null,
      schedule: "*/5 * * * *",
      command: "npm run report",
      runs: [
        {
          __typename: "CronRun",
          name: "nightly-run-1",
          startedAt: "2026-07-09T10:00:00Z",
          finishedAt: "2026-07-09T10:00:05Z",
          status: "Succeeded",
        },
      ],
    };
    const v = toServiceView(serverNode);
    expect(v.type).toBe("cron_job");
    expect(v.schedule).toBe("*/5 * * * *");
    expect(v.command).toBe("npm run report");
    expect(v.runs).toHaveLength(1);
    expect(v.runs[0]).toEqual({
      name: "nightly-run-1",
      startedAt: "2026-07-09T10:00:00Z",
      finishedAt: "2026-07-09T10:00:05Z",
      status: "Succeeded",
    });
  });
});

describe("deriveStatus", () => {
  it("maps each operator phase (capitalized) to a status + variant", () => {
    expect(deriveStatus(svc({ phase: "Running" }))).toEqual({
      key: "running",
      variant: "default",
    });
    expect(deriveStatus(svc({ phase: "Failed" })).key).toBe("failed");
    expect(deriveStatus(svc({ phase: "Deploying" })).key).toBe("deploying");
    expect(deriveStatus(svc({ phase: "Pending" })).key).toBe("pending");
  });

  it("is case-insensitive so an operator casing change won't fall through", () => {
    expect(deriveStatus(svc({ phase: "running" })).key).toBe("running");
  });

  it("lets suspension win over phase (manually suspended App => Suspended, not Sleeping)", () => {
    const s = svc({ suspended: true, phase: "Hibernated" });
    expect(deriveStatus(s)).toEqual({ key: "suspended", variant: "secondary" });
  });

  it("shows a Hibernated-but-not-suspended App as Sleeping (auto-slept free tier)", () => {
    const s = svc({ suspended: false, phase: "Hibernated" });
    expect(deriveStatus(s)).toEqual({ key: "sleeping", variant: "secondary" });
    expect(isSleeping(s)).toBe(true);
  });

  it("does not treat a suspended or running App as sleeping", () => {
    expect(isSleeping(svc({ suspended: true, phase: "Hibernated" }))).toBe(
      false,
    );
    expect(isSleeping(svc({ phase: "Running" }))).toBe(false);
  });

  it("falls back to unknown for an unrecognized phase", () => {
    expect(deriveStatus(svc({ phase: "WeirdNewPhase" })).key).toBe("unknown");
  });
});

describe("computeStats", () => {
  it("counts total / running / suspended from the live list", () => {
    const list = [
      svc({ id: "a", phase: "Running" }),
      svc({ id: "b", phase: "Running" }),
      svc({ id: "c", phase: "Failed" }),
      svc({ id: "d", suspended: true, phase: "Hibernated" }),
    ];
    expect(computeStats(list)).toEqual({ total: 4, running: 2, suspended: 1 });
  });

  it("counts a suspended-but-Running-phase App as suspended, not running", () => {
    const list = [svc({ suspended: true, phase: "Running" })];
    expect(computeStats(list)).toEqual({ total: 1, running: 0, suspended: 1 });
  });
});
