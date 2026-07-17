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
      externalHost: null,
      public: null,
    });
    expect(v.name).toBe("kv1");
    expect(v.status).toBe("");
    expect(v.plan).toBeNull();
    expect(v.public).toBe(false);
    expect(v.suspended).toBe(true);
  });
});

describe("deriveStatus", () => {
  it("maps Render's status enum to a badge key + variant, case-insensitively", () => {
    expect(deriveStatus({ status: "available" })).toEqual({
      key: "available",
      variant: "default",
    });
    expect(deriveStatus({ status: "Creating" })).toEqual({
      key: "creating",
      variant: "outline",
    });
    expect(deriveStatus({ status: "unavailable" })).toEqual({
      key: "unavailable",
      variant: "destructive",
    });
  });

  it("falls back to unknown for an unrecognized status", () => {
    expect(deriveStatus({ status: "weird" })).toEqual({
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
  it("counts total / available / creating", () => {
    const stats = computeStats([
      { status: "available" },
      { status: "available" },
      { status: "creating" },
      { status: "unavailable" },
    ] as unknown as Parameters<typeof computeStats>[0]);
    expect(stats).toEqual({ total: 4, available: 2, creating: 1 });
  });
});
