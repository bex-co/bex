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
        diskAutoscalingEnabled: false,
        suspended: "not_suspended",
        createdAt: "2026-07-01T00:00:00Z",
        updatedAt: "2026-07-02T00:00:00Z",
        region: "fsn1",
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
      updatedAt: "2026-07-02T00:00:00Z",
      region: "fsn1",
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
      diskAutoscalingEnabled: null,
      suspended: null,
      createdAt: null,
      updatedAt: null,
      region: "",
      public: null,
    });
    expect(v.name).toBe("db1");
    expect(v.status).toBe("");
    expect(v.plan).toBeNull();
    expect(v.public).toBe(false);
    expect(v.updatedAt).toBeNull();
    expect(v.region).toBeNull();
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
      diskAutoscalingEnabled: false,
      highAvailabilityEnabled: false,
      suspended: "not_suspended",
      createdAt: null,
      updatedAt: null,
      externalHost: "db1.db.bex.co",
      public: true,
      poolerEnabled: null,
      backupsEnabled: null,
      ipAllowList: null,
      region: null,
      readReplicas: null,
    });
    expect(d.databaseName).toBe("db1");
    expect(d.databaseUser).toBe("db1_user");
    expect(d.externalHost).toBe("db1.db.bex.co");
    expect(d.highAvailabilityEnabled).toBe(false);
    expect(d.region).toBeNull();
  });

  it("normalizes the private-database empty-string externalHost to null (w6/052)", () => {
    const d = toDatabaseDetailView({
      __typename: "Database",
      id: "db3",
      name: "db3",
      plan: "free",
      version: "18",
      status: "available",
      databaseName: "db3",
      databaseUser: "db3_user",
      diskSizeGB: 1,
      diskAutoscalingEnabled: false,
      highAvailabilityEnabled: false,
      suspended: "not_suspended",
      createdAt: null,
      updatedAt: null,
      // bex-api sends "" (not null) when public access is off.
      externalHost: "",
      public: false,
      poolerEnabled: null,
      backupsEnabled: null,
      ipAllowList: null,
      region: null,
      readReplicas: null,
    });
    expect(d.externalHost).toBeNull();
  });

  it("carries region through from the wire type when configured (w9/m42/t004)", () => {
    const d = toDatabaseDetailView({
      __typename: "Database",
      id: "db2",
      name: "db2",
      plan: "free",
      version: "18",
      status: "available",
      databaseName: "db2",
      databaseUser: "db2_user",
      diskSizeGB: 1,
      diskAutoscalingEnabled: false,
      highAvailabilityEnabled: false,
      suspended: "not_suspended",
      createdAt: null,
      updatedAt: null,
      externalHost: null,
      public: false,
      poolerEnabled: null,
      backupsEnabled: null,
      ipAllowList: null,
      region: "fsn1",
      readReplicas: null,
    });
    expect(d.region).toBe("fsn1");
  });
});

describe("deriveStatus", () => {
  it("maps Render's status enum to a badge key + variant, case-insensitively", () => {
    expect(
      deriveStatus({ status: "available", suspended: "not_suspended" }),
    ).toEqual({
      key: "available",
      variant: "default",
    });
    expect(
      deriveStatus({ status: "Creating", suspended: "not_suspended" }),
    ).toEqual({
      key: "creating",
      variant: "outline",
    });
    expect(
      deriveStatus({ status: "unavailable", suspended: "not_suspended" }),
    ).toEqual({
      key: "unavailable",
      variant: "destructive",
    });
  });

  it("lets suspension win over the status enum — a suspended database still reports status available", () => {
    expect(
      deriveStatus({ status: "available", suspended: "suspended" }),
    ).toEqual({
      key: "suspended",
      variant: "secondary",
    });
  });

  it("falls back to unknown for an unrecognized status", () => {
    expect(
      deriveStatus({ status: "weird", suspended: "not_suspended" }),
    ).toEqual({
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
      { status: "available", suspended: "not_suspended" },
      { status: "available", suspended: "not_suspended" },
      { status: "available", suspended: "suspended" },
      { status: "creating", suspended: "not_suspended" },
      { status: "unavailable", suspended: "not_suspended" },
    ] as unknown as Parameters<typeof computeStats>[0]);
    expect(stats).toEqual({ total: 5, available: 2, creating: 1 });
  });
});
