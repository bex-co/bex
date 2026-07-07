import { describe, it, expect } from "vitest";
import {
  toServiceView,
  deriveStatus,
  computeStats,
  isSuspended,
} from "@/features/services/lib/status";
import type { ServicesQuery } from "@/graphql/definitions";
import type { ServiceView } from "@/features/services/types";

type ServiceNode = NonNullable<NonNullable<ServicesQuery["services"]>[number]>;

function node(overrides: Partial<ServiceNode> = {}): ServiceNode {
  return {
    __typename: "Service",
    id: "app",
    name: "app",
    type: "web_service",
    suspended: "not_suspended",
    dashboardUrl: "https://app.onbex.co",
    url: "https://app.onbex.co",
    createdAt: "2026-01-01T00:00:00Z",
    phase: "Running",
    replicas: 1,
    revision: "abc123",
    ...overrides,
  };
}

function svc(overrides: Partial<ServiceView> = {}): ServiceView {
  return {
    id: "app",
    name: "app",
    suspended: false,
    phase: "Running",
    url: "https://app.onbex.co",
    createdAt: "2026-01-01T00:00:00Z",
    replicas: 1,
    revision: "abc123",
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
    expect(toServiceView(node({ suspended: "suspended", phase: "Hibernated" }))).toEqual({
      id: "app",
      name: "app",
      suspended: true,
      phase: "Hibernated",
      url: "https://app.onbex.co",
      createdAt: "2026-01-01T00:00:00Z",
      replicas: 1,
      revision: "abc123",
    });
  });

  it("falls back to id for a missing name and null for a missing url", () => {
    const v = toServiceView(node({ name: null, url: null, id: "svc-1" }));
    expect(v.name).toBe("svc-1");
    expect(v.url).toBeNull();
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

  it("lets suspension win over phase (suspended App reports Hibernated)", () => {
    const s = svc({ suspended: true, phase: "Hibernated" });
    expect(deriveStatus(s)).toEqual({ key: "suspended", variant: "secondary" });
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
