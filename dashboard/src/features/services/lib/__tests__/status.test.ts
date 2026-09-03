import { describe, it, expect } from "vitest";
import {
  toServiceView,
  deriveStatus,
  computeStats,
  isSuspended,
  isSleeping,
  isConvergingPhase,
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
    updatedAt: null,
    runtime: null,
    region: null,
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
    slug: null,
    type: "web_service",
    suspended: false,
    phase: "Running",
    url: "https://app.onbex.co",
    internalAddress: null,
    createdAt: "2026-01-01T00:00:00Z",
    sshAddress: null,
    replicas: 1,
    revision: "abc123",
    plan: null,
    idleTTLSeconds: null,
    maintenanceMode: null,
    schedule: null,
    command: null,
    runs: [],
    repo: null,
    branch: null,
    imagePath: null,
    rootDir: null,
    runtime: null,
    builder: null,
    buildCommand: null,
    startCommand: null,
    dockerfilePath: null,
    registryCredentialId: null,
    buildFilter: null,
    autoDeploy: null,
    notifyOnFail: null,
    notificationsToSend: null,
    renderSubdomainPolicy: null,
    healthCheckPath: null,
    maxShutdownDelaySeconds: null,
    preDeployCommand: null,
    publishPath: null,
    routes: [],
    headers: [],
    ipAllowList: null,
    ipAllowListEntries: null,
    ...overrides,
  };
}

function server(overrides: Partial<ServerNode> = {}): ServerNode {
  return {
    __typename: "Service",
    id: "app",
    name: "app",
    slug: null,
    displayName: null,
    type: "web_service",
    suspended: "not_suspended",
    dashboardUrl: null,
    region: null,
    url: null,
    publicRoutingNotice: null,
    internalAddress: null,
    createdAt: null,
    sshAddress: null,
    phase: "Running",
    replicas: 1,
    revision: null,
    plan: null,
    idleTTLSeconds: 0,
    repo: null,
    branch: null,
    imagePath: null,
    rootDir: null,
    runtime: null,
    builder: null,
    buildCommand: null,
    startCommand: null,
    dockerfilePath: null,
    registryCredentialId: null,
    autoDeploy: null,
    pushDeliveryMethod: null,
    notifyOnFail: null,
    notificationsToSend: null,
    renderSubdomainPolicy: null,
    healthCheckPath: null,
    maxShutdownDelaySeconds: null,
    preDeployCommand: null,
    schedule: null,
    command: null,
    lastSuccessfulRunAt: null,
    nextRunAt: null,
    publishPath: null,
    ipAllowList: null,
    maintenanceMode: { __typename: "MaintenanceMode", enabled: false, uri: "" },
    buildFilter: null,
    runs: null,
    routes: null,
    headers: null,
    ipAllowListEntries: null,
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
      internalAddress: null,
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: null,
      region: null,
      sshAddress: null,
      replicas: 1,
      revision: "abc123",
      plan: null,
      idleTTLSeconds: 0,
      schedule: null,
      command: null,
      lastSuccessfulRunAt: null,
      nextRunAt: null,
      runs: [],
      repo: null,
      branch: null,
      imagePath: null,
      rootDir: null,
      runtime: null,
      builder: null,
      buildCommand: null,
      startCommand: null,
      dockerfilePath: null,
      registryCredentialId: null,
      buildFilter: null,
      autoDeploy: null,
      // The list node doesn't select push deliverability, so the mapper reports
      // null rather than guessing a delivery mechanism (w6/m99).
      pushDeliveryMethod: null,
      notifyOnFail: null,
      renderSubdomainPolicy: null,
      notificationsToSend: null,
      healthCheckPath: null,
      maxShutdownDelaySeconds: null,
      preDeployCommand: null,
      publishPath: null,
      routes: [],
      headers: [],
      ipAllowList: null,
      ipAllowListEntries: null,
      maintenanceMode: null,
    });
  });

  it("carries the plan through when the wire Service has one", () => {
    expect(toServiceView(node({ plan: "pro_plus" })).plan).toBe("pro_plus");
  });

  it("keeps server runtime/region/update facts and normalizes an omitted region", () => {
    const populated = toServiceView(
      node({
        runtime: "node",
        region: "fsn1",
        updatedAt: "2026-07-02T00:00:00Z",
      }),
    );
    expect(populated.runtime).toBe("node");
    expect(populated.region).toBe("fsn1");
    expect(populated.updatedAt).toBe("2026-07-02T00:00:00Z");
    expect(toServiceView(node({ region: "" })).region).toBeNull();
  });

  it("carries a copy-ready SSH address only when the detail query selected it", () => {
    expect(
      toServiceView(server({ sshAddress: "srv-example@ssh.bex.co" }))
        .sshAddress,
    ).toBe("srv-example@ssh.bex.co");
  });

  // w6/m99: only the detail query computes push deliverability. A list row must
  // report null — "not computed on this projection" — rather than let the
  // Auto-Deploy hint read an absent field as a mechanism it can name.
  it("carries push deliverability only from the detail query", () => {
    expect(
      toServiceView(server({ pushDeliveryMethod: "manual_webhook" }))
        .pushDeliveryMethod,
    ).toBe("manual_webhook");
    expect(toServiceView(node()).pushDeliveryMethod).toBeNull();
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

  it("maps a build-from-git server node's source and build settings", () => {
    const serverNode: ServerNode = server({
      id: "mono",
      name: "mono",
      repo: "https://github.com/x/mono",
      branch: "main",
      rootDir: "backend",
      runtime: "docker",
      builder: "dockerfile",
      startCommand: "bin/server",
      dockerfilePath: "docker/Dockerfile.prod",
      registryCredentialId: "rgc-private",
    });
    const v = toServiceView(serverNode);
    expect(v.repo).toBe("https://github.com/x/mono");
    expect(v.branch).toBe("main");
    expect(v.rootDir).toBe("backend");
    expect(v.runtime).toBe("docker");
    expect(v.builder).toBe("dockerfile");
    expect(v.startCommand).toBe("bin/server");
    expect(v.dockerfilePath).toBe("docker/Dockerfile.prod");
    expect(v.registryCredentialId).toBe("rgc-private");
  });

  it("leaves build settings null for a list node (not selected)", () => {
    const v = toServiceView(node());
    expect(v.repo).toBeNull();
    expect(v.branch).toBeNull();
    expect(v.rootDir).toBeNull();
    expect(v.runtime).toBeNull();
    expect(v.builder).toBeNull();
    expect(v.startCommand).toBeNull();
    expect(v.dockerfilePath).toBeNull();
    expect(v.registryCredentialId).toBeNull();
  });

  it("reads slug from a detail server node, incl. the random-suffix case", () => {
    const serverNode: ServerNode = server({
      id: "web",
      name: "web",
      slug: "web-a1b2",
    });
    expect(toServiceView(serverNode).slug).toBe("web-a1b2");
  });

  it("leaves slug null for a list node (not selected by the services query)", () => {
    expect(toServiceView(node()).slug).toBeNull();
  });

  it("maps a cron server node's schedule and run history", () => {
    const serverNode: ServerNode = server({
      id: "nightly",
      name: "nightly",
      type: "cron_job",
      replicas: null,
      schedule: "*/5 * * * *",
      command: "npm run report",
      runs: [
        {
          __typename: "CronRun",
          id: "crr-d2g9h41kc86ots6qg9s0",
          name: "nightly-run-1",
          startedAt: "2026-07-09T10:00:00Z",
          finishedAt: "2026-07-09T10:00:05Z",
          status: "Succeeded",
        },
      ],
    });
    const v = toServiceView(serverNode);
    expect(v.type).toBe("cron_job");
    expect(v.schedule).toBe("*/5 * * * *");
    expect(v.command).toBe("npm run report");
    expect(v.runs).toHaveLength(1);
    expect(v.runs[0]).toEqual({
      id: "crr-d2g9h41kc86ots6qg9s0",
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

  // w6/m52: canceling a service's very first deploy used to report phase Failed
  // — a red error badge for something the user did on purpose, contradicting the
  // deploy's own "canceled" status. It is its own non-destructive state now.
  it("shows a canceled first-ever deploy as Canceled, not the red Failed badge", () => {
    expect(deriveStatus(svc({ phase: "Canceled" }))).toEqual({
      key: "canceled",
      variant: "secondary",
    });
  });

  it("keeps a genuine failure on the destructive Failed badge", () => {
    expect(deriveStatus(svc({ phase: "Failed" }))).toEqual({
      key: "failed",
      variant: "destructive",
    });
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

  // w3/m81: a service observed mid-teardown must read as a muted "Deleting", not
  // fall through to the generic "Unknown" the m81 fixture showed for 2+ hours.
  it("shows a deleting service as Deleting, never Unknown", () => {
    expect(deriveStatus(svc({ phase: "Deleting" }))).toEqual({
      key: "deleting",
      variant: "secondary",
    });
    expect(deriveStatus(svc({ phase: "deleting" })).key).toBe("deleting");
  });

  // Only a free web service has the public activator route that makes Sleeping
  // and its wake-on-request promise true. Other Hibernated services are either
  // resuming from manual suspension or stale data from before that eligibility
  // rule was enforced.
  describe("Hibernated resolves by service type, not by phase alone", () => {
    it("keeps Sleeping for a free web service", () => {
      const s = svc({
        type: "web_service",
        plan: "free",
        suspended: false,
        phase: "Hibernated",
      });
      expect(deriveStatus(s)).toEqual({
        key: "sleeping",
        variant: "secondary",
      });
      expect(isSleeping(s)).toBe(true);
    });

    it("reports every non-web type as pending, never Sleeping", () => {
      for (const type of [
        "private_service",
        "background_worker",
        "cron_job",
        "static_site",
      ]) {
        const s = svc({ type, suspended: false, phase: "Hibernated" });
        expect(deriveStatus(s)).toEqual({ key: "pending", variant: "outline" });
        expect(isSleeping(s)).toBe(false);
      }
    });

    it("does not call a paid web service an auto-sleeper", () => {
      const s = svc({
        type: "web_service",
        plan: "starter",
        suspended: false,
        phase: "Hibernated",
      });
      expect(deriveStatus(s)).toEqual({ key: "pending", variant: "outline" });
      expect(isSleeping(s)).toBe(false);
    });

    it("still lets manual suspension win for a worker", () => {
      const s = svc({
        type: "background_worker",
        suspended: true,
        phase: "Hibernated",
      });
      expect(deriveStatus(s).key).toBe("suspended");
      expect(isSleeping(s)).toBe(false);
    });

    it("leaves every other phase type-independent", () => {
      for (const type of ["web_service", "background_worker", "cron_job"]) {
        expect(deriveStatus(svc({ type, phase: "Running" })).key).toBe(
          "running",
        );
        expect(deriveStatus(svc({ type, phase: "Failed" })).key).toBe("failed");
        expect(deriveStatus(svc({ type, phase: "Building" })).key).toBe(
          "building",
        );
      }
    });
  });
});

describe("isConvergingPhase", () => {
  it("keeps polling through the moving phases, including deleting", () => {
    for (const phase of ["", "pending", "building", "deploying", "deleting"]) {
      expect(isConvergingPhase({ phase })).toBe(true);
    }
  });

  // w3/m81: "deleting" is converging so a service seen mid-teardown keeps
  // polling until the by-id read resolves to not-found (which redirects), rather
  // than freezing on the stale cached row the fixture did for 2+ hours.
  it("treats a settled phase as done so the poll can stop", () => {
    for (const phase of ["running", "failed", "canceled", "hibernated"]) {
      expect(isConvergingPhase({ phase })).toBe(false);
    }
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
