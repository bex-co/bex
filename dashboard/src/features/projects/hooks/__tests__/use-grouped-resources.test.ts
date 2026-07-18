import { describe, expect, it } from "vitest";
import {
  toDatabaseRow,
  toEnvGroupRow,
  toKeyValueRow,
  toServiceRow,
} from "../use-grouped-resources";

describe("Project resource metadata normalization", () => {
  it("uses authoritative service facts and never substitutes createdAt", () => {
    const row = toServiceRow({
      id: "srv-api",
      name: "API",
      runtime: "node",
      region: "fsn1",
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-07-01T00:00:00Z",
    } as never);

    expect(row.runtime).toBe("Node");
    expect(row.region).toBe("fsn1");
    expect(row.updatedAt).toBe("2026-07-01T00:00:00Z");
    expect(row.updatedAt).not.toBe(row.createdAt);
  });

  it("keeps missing service runtime, region, and updatedAt unavailable", () => {
    const row = toServiceRow({
      id: "srv-legacy",
      name: "Legacy",
      runtime: null,
      region: null,
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: null,
    } as never);

    expect(row.runtime).toBeNull();
    expect(row.region).toBeNull();
    expect(row.updatedAt).toBeNull();
  });

  it("maps datastore product versions and leaves Env Group placement unknown", () => {
    expect(
      toDatabaseRow({
        id: "dpg-main",
        name: "Main DB",
        version: "16",
        createdAt: null,
        updatedAt: null,
        region: "fsn1",
      } as never),
    ).toMatchObject({ runtime: "PostgreSQL 16", region: "fsn1" });
    expect(
      toKeyValueRow({
        id: "red-cache",
        name: "Cache",
        version: "8.1",
        createdAt: null,
        updatedAt: null,
        region: "fsn1",
      } as never),
    ).toMatchObject({ runtime: "Valkey 8.1", region: "fsn1" });
    expect(
      toEnvGroupRow({
        id: "evg-shared",
        name: "Shared",
        createdAt: null,
        updatedAt: null,
      } as never),
    ).toMatchObject({ runtime: null, region: null });
  });
});
