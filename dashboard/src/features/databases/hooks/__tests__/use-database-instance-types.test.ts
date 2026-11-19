import { describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import { useDatabaseInstanceTypes } from "@/features/databases/hooks/use-database-instance-types";

const mockUseQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useQuery: (...args: unknown[]) => mockUseQuery(...args),
}));

function tier(id: string, maxConnections?: number) {
  return {
    __typename: "DatabaseInstanceType" as const,
    id,
    name: id,
    cpu: "1",
    memory: "1Gi",
    storageGB: 1,
    maxStorageGB: 16384,
    supportsDiskAutoscaling: true,
    supportsConnectionPooling: true,
    maxReadReplicas: 5,
    readReplicaMinStorageGB: 10,
    supportsHighAvailability: true,
    monthlyUsd: "19.00",
    ...(maxConnections === undefined ? {} : { maxConnections }),
  };
}

// w6/138: each plan's connection limit, the "Limit" beside the Metrics tab's
// Active Connections chart, comes from the catalog, never a frontend ladder; a
// catalog that sends none (or a non-positive figure) leaves it undefined, so no
// chart reads "Limit 0".
describe("useDatabaseInstanceTypes connection limit", () => {
  it("passes each plan's limit through and drops a missing or zero one", () => {
    mockUseQuery.mockReturnValue({
      data: {
        databaseInstanceTypes: [
          tier("basic-1gb", 100),
          tier("pro-16gb", 400),
          tier("older-api"),
          tier("zero", 0),
        ],
      },
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    });
    const { result } = renderHook(() => useDatabaseInstanceTypes());
    expect(
      result.current.instanceTypes.map((it) => [it.id, it.maxConnections]),
    ).toEqual([
      ["basic-1gb", 100],
      ["pro-16gb", 400],
      ["older-api", undefined],
      ["zero", undefined],
    ]);
  });
});
