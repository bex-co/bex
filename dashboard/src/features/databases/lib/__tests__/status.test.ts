import { describe, it, expect } from "vitest";
import {
  toDatabaseView,
  toDatabaseViews,
  toDatabaseDetailView,
  deriveStatus,
  isConverging,
  computeStats,
  sqlConsoleBlock,
} from "@/features/databases/lib/status";
import { statusLabel } from "@/features/databases/lib/labels";

describe("toDatabaseViews", () => {
  it("maps wire Database nodes onto normalized views and drops nulls", () => {
    const views = toDatabaseViews([
      {
        __typename: "Database",
        expiresAt: null,
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
      expiresAt: null,
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
      expiresAt: null,
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
      expiresAt: null,
      statusReason: null,
      statusReasonCode: null,
      ownerId: "tea-1",
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
      highAvailabilityRequested: null,
      highAvailabilityReady: null,
      suspended: "not_suspended",
      createdAt: null,
      updatedAt: null,
      externalHost: "db1.db.bex.co",
      public: true,
      poolerEnabled: true,
      backupsEnabled: null,
      ipAllowList: null,
      region: null,
      readReplicas: null,
    });
    expect(d.ownerId).toBe("tea-1");
    expect(d.databaseName).toBe("db1");
    expect(d.databaseUser).toBe("db1_user");
    expect(d.externalHost).toBe("db1.db.bex.co");
    expect(d.highAvailabilityEnabled).toBe(false);
    // An older bex-api with no spec-intent field reads as the observed state.
    expect(d.highAvailabilityRequested).toBe(false);
    expect(d.poolerEnabled).toBe(true);
    expect(d.region).toBeNull();
  });

  it("normalizes the private-database empty-string externalHost to null (w6/052)", () => {
    const d = toDatabaseDetailView({
      __typename: "Database",
      expiresAt: null,
      statusReason: null,
      statusReasonCode: null,
      ownerId: "tea-1",
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
      highAvailabilityRequested: null,
      highAvailabilityReady: null,
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
      expiresAt: null,
      statusReason: null,
      statusReasonCode: null,
      ownerId: "tea-1",
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
      highAvailabilityRequested: null,
      highAvailabilityReady: null,
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

  // w5/079: the card translates by code, so dropping it here would silently
  // show bex-api's English sentence in every language.
  it("carries an unavailable database's reason and its code", () => {
    const d = toDatabaseDetailView({
      __typename: "Database",
      expiresAt: null,
      statusReason: "The connection pooler could not be provisioned.",
      statusReasonCode: "PoolerFailed",
      ownerId: "tea-1",
      id: "db4",
      name: "db4",
      plan: "free",
      version: "18",
      status: "unavailable",
      databaseName: "db4",
      databaseUser: "db4_user",
      diskSizeGB: 1,
      diskAutoscalingEnabled: false,
      highAvailabilityEnabled: false,
      highAvailabilityRequested: null,
      highAvailabilityReady: null,
      suspended: "not_suspended",
      createdAt: null,
      updatedAt: null,
      externalHost: null,
      public: false,
      poolerEnabled: true,
      backupsEnabled: null,
      ipAllowList: null,
      region: null,
      readReplicas: null,
    });
    expect(d.statusReason).toBe(
      "The connection pooler could not be provisioned.",
    );
    expect(d.statusReasonCode).toBe("PoolerFailed");
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

  it("lets suspension win over the status enum — via suspended flag or wire status", () => {
    expect(
      deriveStatus({ status: "available", suspended: "suspended" }),
    ).toEqual({
      key: "suspended",
      variant: "secondary",
    });
    expect(
      deriveStatus({ status: "suspended", suspended: "not_suspended" }),
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

describe("config_restart (w4/m137)", () => {
  it("reads Restarting, never Unknown, and keeps the page polling", () => {
    const db = { status: "config_restart", suspended: "not_suspended" };
    expect(deriveStatus(db)).toEqual({ key: "restarting", variant: "outline" });
    expect(statusLabel(db)).toBe("databases.statusRestarting");
    expect(isConverging(db)).toBe(true);
    expect(isConverging({ status: "upgrading" })).toBe(true);
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

describe("sqlConsoleBlock (w4/177)", () => {
  it("blocks only the known not-serving lifecycle states", () => {
    expect(sqlConsoleBlock("suspended")).toBe("suspended");
    expect(sqlConsoleBlock("creating")).toBe("creating");
    expect(sqlConsoleBlock("upgrading")).toBe("upgrading");
    expect(sqlConsoleBlock("restarting")).toBe("restarting");
    // Available runs; unavailable/unknown are left to the server's answer.
    expect(sqlConsoleBlock("available")).toBeNull();
    expect(sqlConsoleBlock("unavailable")).toBeNull();
    expect(sqlConsoleBlock("unknown")).toBeNull();
  });
});

describe("toDatabaseDetailView high availability (w2/059)", () => {
  const node = (
    enabled: boolean | null,
    requested: boolean | null,
    ready: boolean | null,
  ) =>
    toDatabaseDetailView({
      __typename: "Database",
      expiresAt: null,
      statusReason: null,
      statusReasonCode: null,
      ownerId: "tea-1",
      id: "db",
      name: "db",
      plan: "1c-2g",
      version: "18",
      status: "creating",
      databaseName: "db",
      databaseUser: "db_user",
      diskSizeGB: 10,
      diskAutoscalingEnabled: false,
      highAvailabilityEnabled: enabled,
      highAvailabilityRequested: requested,
      highAvailabilityReady: ready,
      suspended: "not_suspended",
      createdAt: null,
      updatedAt: null,
      externalHost: null,
      public: true,
      poolerEnabled: false,
      backupsEnabled: null,
      ipAllowList: null,
      region: null,
      readReplicas: null,
    });

  it("treats the wire's enabled field as the request and ready as the standby", () => {
    // Enabling: the API already says enabled (as on Render); no standby yet.
    const starting = node(true, true, false);
    expect(starting.highAvailabilityRequested).toBe(true);
    expect(starting.highAvailabilityEnabled).toBe(false);
    // Disabling: the request is off while the standby still runs.
    const stopping = node(false, false, true);
    expect(stopping.highAvailabilityRequested).toBe(false);
    expect(stopping.highAvailabilityEnabled).toBe(true);
  });

  it("still reads an older API, whose enabled field was the observed value", () => {
    // Requested on, standby not up yet: older API said enabled=false.
    const old = node(false, true, null);
    expect(old.highAvailabilityEnabled).toBe(false);
    expect(old.highAvailabilityRequested).toBe(true);
  });
});
