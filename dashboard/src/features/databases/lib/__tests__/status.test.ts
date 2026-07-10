import { describe, it, expect } from "vitest";
import {
  toDatabaseView,
  toDatabaseViews,
  toDatabaseDetailView,
  deriveStatus,
  isConverging,
  computeStats,
} from "@/features/databases/lib/status";

describe("toDatabaseViews", () => {
  it("maps wire Database nodes onto normalized views and drops nulls", () => {
    const views = toDatabaseViews([
      {
        __typename: "Database",
        id: "shop-db",
        name: "shop-db",
        plan: "basic-1gb",
        version: "16",
        status: "available",
        diskSizeGB: 5,
        suspended: "not_suspended",
        createdAt: "2026-07-01T00:00:00Z",
        public: true,
      },
      null,
    ]);

    expect(views).toHaveLength(1);
    expect(views[0]).toEqual({
      id: "shop-db",
      name: "shop-db",
      status: "available",
      plan: "basic-1gb",
      version: "16",
      diskSizeGB: 5,
      createdAt: "2026-07-01T00:00:00Z",
      public: true,
      suspended: "not_suspended",
    });
  });

  it("returns an empty list (not a crash) when data is undefined", () => {
    expect(toDatabaseViews(undefined)).toEqual([]);
  });

  it("falls back name->id and coerces missing nullable fields", () => {
    const v = toDatabaseView({
      __typename: "Database",
      id: "db1",
      name: null,
      plan: null,
      version: null,
      status: null,
      diskSizeGB: null,
      suspended: null,
      createdAt: null,
      public: null,
    });
    expect(v.name).toBe("db1");
    expect(v.status).toBe("");
    expect(v.plan).toBeNull();
    expect(v.public).toBe(false);
  });
});

describe("toDatabaseDetailView", () => {
  it("extends the list view with the detail-only fields", () => {
    const d = toDatabaseDetailView({
      __typename: "Database",
      id: "db1",
      name: "db1",
      plan: "free",
      version: "18",
      status: "available",
      databaseName: "db1",
      databaseUser: "db1_user",
      diskSizeGB: 1,
      highAvailabilityEnabled: false,
      suspended: "not_suspended",
      createdAt: null,
      externalHost: "db1.db.bex.co",
      public: true,
    });
    expect(d.databaseName).toBe("db1");
    expect(d.databaseUser).toBe("db1_user");
    expect(d.externalHost).toBe("db1.db.bex.co");
    expect(d.highAvailabilityEnabled).toBe(false);
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
