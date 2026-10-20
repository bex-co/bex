import { describe, expect, it } from "vitest";
import { readRepoVectors, repeated } from "@/test/repo-vectors";
import {
  READ_REPLICA_NAME_MAX_LENGTH,
  readReplicaAddBlock,
  readReplicaNameProblem,
} from "@/features/databases/lib/read-replicas";
import type { DatabaseInstanceTypeView } from "@/features/databases/types";

// The one table of read replica names. bex-api's checkReadReplicaNames is
// tested against the same file (lego/backend/internal/postgres/
// read_replica_names_test.go), and its answers are the source of truth (w5/m138).
const VECTORS = readRepoVectors<{
  name: string;
  repeat?: number;
  code: string | null;
}>("lego/backend/internal/postgres/testdata/read-replica-name-vectors.json");

describe("the shared read replica name vectors", () => {
  it.each(VECTORS.map((v) => [repeated(v.name, v.repeat), v.code] as const))(
    "%j answers %s",
    (name, code) => {
      expect(readReplicaNameProblem(name, [])).toBe(code ? "invalid" : null);
    },
  );

  it("bounds names so <database id>-ro-<name> stays one DNS label", () => {
    expect(READ_REPLICA_NAME_MAX_LENGTH).toBe(63 - "dpg-".length - 20 - 4);
  });
});

describe("readReplicaNameProblem", () => {
  it("refuses a name the database already uses", () => {
    expect(readReplicaNameProblem("r1", ["r1", "r2"])).toBe("duplicate");
    expect(readReplicaNameProblem("r3", ["r1", "r2"])).toBeNull();
  });
});

const PLAN: DatabaseInstanceTypeView = {
  id: "basic-1gb",
  name: "Basic 1 GB",
  cpu: "500m",
  memory: "1Gi",
  storageGB: 5,
  maxStorageGB: 16384,
  supportsDiskAutoscaling: true,
  supportsConnectionPooling: true,
  maxReadReplicas: 2,
  readReplicaMinStorageGB: 10,
  supportsHighAvailability: false,
  monthlyUsd: "13.30",
};

describe("readReplicaAddBlock", () => {
  const replicas = (n: number) =>
    Array.from({ length: n }, (_, i) => ({
      name: `r${i}`,
      connectionInfo: null,
    }));

  it("never blocks while the plan catalog is unknown", () => {
    expect(
      readReplicaAddBlock({ diskSizeGB: 1, readReplicas: [] }, undefined),
    ).toBeNull();
  });

  it("blocks on a plan that allows no replicas", () => {
    expect(
      readReplicaAddBlock(
        { diskSizeGB: 20, readReplicas: [] },
        { ...PLAN, maxReadReplicas: 0 },
      ),
    ).toEqual({ kind: "plan" });
  });

  it("blocks at the plan's limit, not before", () => {
    expect(
      readReplicaAddBlock({ diskSizeGB: 20, readReplicas: replicas(1) }, PLAN),
    ).toBeNull();
    expect(
      readReplicaAddBlock({ diskSizeGB: 20, readReplicas: replicas(2) }, PLAN),
    ).toEqual({ kind: "limit", max: 2 });
  });

  it("blocks below the minimum storage, reading the plan's floor when the size is unknown", () => {
    expect(
      readReplicaAddBlock({ diskSizeGB: 9, readReplicas: [] }, PLAN),
    ).toEqual({ kind: "storage", min: 10, current: 9 });
    expect(
      readReplicaAddBlock({ diskSizeGB: null, readReplicas: [] }, PLAN),
    ).toEqual({ kind: "storage", min: 10, current: 5 });
    expect(
      readReplicaAddBlock({ diskSizeGB: 10, readReplicas: [] }, PLAN),
    ).toBeNull();
  });
});
