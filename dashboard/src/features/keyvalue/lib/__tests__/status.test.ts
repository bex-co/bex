import { describe, it, expect } from "vitest";
import {
  toKeyValueView,
  toKeyValueViews,
  deriveStatus,
  isConverging,
  computeStats,
} from "@/features/keyvalue/lib/status";

describe("toKeyValueViews", () => {
  it("maps wire KeyValue nodes onto normalized views and drops nulls", () => {
    const views = toKeyValueViews([
      {
        __typename: "KeyValue",
        id: "sessions-cache",
        name: "sessions-cache",
        plan: "starter",
        version: "8",
        status: "available",
        suspended: "not_suspended",
        createdAt: "2026-07-01T00:00:00Z",
        updatedAt: "2026-07-02T00:00:00Z",
        region: null,
        externalHost: "sessions-cache.kv.bex.co",
        public: true,
      },
      null,
    ]);

    expect(views).toHaveLength(1);
    expect(views[0]).toEqual({
      id: "sessions-cache",
      name: "sessions-cache",
      status: "available",
      plan: "starter",
      version: "8",
      createdAt: "2026-07-01T00:00:00Z",
      updatedAt: "2026-07-02T00:00:00Z",
      externalHost: "sessions-cache.kv.bex.co",
      public: true,
      suspended: false,
      region: null,
    });
  });

  it("returns an empty list (not a crash) when data is undefined", () => {
    expect(toKeyValueViews(undefined)).toEqual([]);
  });

  it("falls back name->id, coerces missing nullable fields, and derives suspended from the string enum", () => {
    const v = toKeyValueView({
      __typename: "KeyValue",
      id: "kv1",
      name: null,
      plan: null,
      version: null,
      status: null,
      suspended: "suspended",
      createdAt: null,
      updatedAt: null,
      region: null,
      externalHost: null,
      public: null,
    });
    expect(v.name).toBe("kv1");
    expect(v.status).toBe("");
    expect(v.plan).toBeNull();
    expect(v.public).toBe(false);
    expect(v.suspended).toBe(true);
    expect(v.updatedAt).toBeNull();
  });
});

describe("deriveStatus", () => {
  it("maps Render's status enum to a badge key + variant, case-insensitively", () => {
    expect(deriveStatus({ status: "available", suspended: false })).toEqual({
      key: "available",
      variant: "default",
    });
    expect(deriveStatus({ status: "Creating", suspended: false })).toEqual({
      key: "creating",
      variant: "outline",
    });
    expect(deriveStatus({ status: "unavailable", suspended: false })).toEqual({
      key: "unavailable",
      variant: "destructive",
    });
  });

  it("lets suspension win over the status enum — a suspended store still reports status available", () => {
    expect(deriveStatus({ status: "available", suspended: true })).toEqual({
      key: "suspended",
      variant: "secondary",
    });
  });

  it("falls back to unknown for an unrecognized status", () => {
    expect(deriveStatus({ status: "weird", suspended: false })).toEqual({
      key: "unknown",
      variant: "outline",
    });
  });
});

describe("isConverging", () => {
  it("is true only while creating", () => {
    expect(isConverging({ status: "creating" })).toBe(true);
    expect(isConverging({ status: "available" })).toBe(false);
    expect(isConverging({ status: "unavailable" })).toBe(false);
  });
});

describe("computeStats", () => {
  it("counts total / available / creating, excluding suspended from available", () => {
    const stats = computeStats([
      { status: "available", suspended: false },
      { status: "available", suspended: false },
      { status: "available", suspended: true },
      { status: "creating", suspended: false },
      { status: "unavailable", suspended: false },
    ] as unknown as Parameters<typeof computeStats>[0]);
    expect(stats).toEqual({ total: 5, available: 2, creating: 1 });
  });
});
