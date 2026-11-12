import { describe, it, expect } from "vitest";
import { strandedGroupsForDestination } from "@/features/env-groups/lib/stranded";
import type { EnvGroupView } from "@/features/env-groups/types";

function group(
  id: string,
  environmentId: string | null,
  serviceLinks: string[],
): EnvGroupView {
  return {
    id,
    name: id,
    ownerId: "tea-1",
    environmentId,
    createdAt: null,
    updatedAt: null,
    revision: null,
    availability: null,
    serviceLinks,
    envVarKeys: [],
    secretFileNames: [],
  } as EnvGroupView;
}

describe("strandedGroupsForDestination (w4/m238)", () => {
  const groups = [
    group("evg-prod", "evm-prod", ["srv-1"]),
    group("evg-staging", "evm-staging", ["srv-1"]),
    group("evg-workspace", null, ["srv-1"]),
    group("evg-other-service", "evm-prod", ["srv-2"]),
    group("evg-unlinked", "evm-prod", []),
  ];

  // w4/564: a group moving to the destination with its services is in scope
  // after the move, so only the groups that stay behind are listed — unless
  // the moving group also links a service that stays, which bex-api refuses.
  it("leaves out a group that moves with every service it links", () => {
    const comoving = (
      groupIds: string[],
      destinationServiceIds: string[] = [],
    ) => ({
      groupIds,
      destinationServiceIds,
    });
    expect(
      strandedGroupsForDestination(
        groups,
        ["srv-1"],
        "evm-staging",
        comoving(["evg-prod"]),
      ),
    ).toEqual([]);
    const twoGroups = [
      group("evg-a", "evm-prod", ["srv-1"]),
      group("evg-b", "evm-prod", ["srv-1"]),
    ];
    expect(
      strandedGroupsForDestination(
        twoGroups,
        ["srv-1"],
        "evm-staging",
        comoving(["evg-a"]),
      ).map((g) => g.id),
    ).toEqual(["evg-b"]);
    const shared = [group("evg-shared", "evm-prod", ["srv-1", "srv-2"])];
    expect(
      strandedGroupsForDestination(
        shared,
        ["srv-1"],
        "evm-staging",
        comoving(["evg-shared"]),
      ).map((g) => g.id),
    ).toEqual(["evg-shared"]);
    // …though not when that other service is already at the destination.
    expect(
      strandedGroupsForDestination(
        shared,
        ["srv-1"],
        "evm-staging",
        comoving(["evg-shared"], ["srv-2"]),
      ),
    ).toEqual([]);
  });

  it("flags environment groups scoped anywhere but the destination", () => {
    expect(
      strandedGroupsForDestination(groups, ["srv-1"], "evm-staging").map(
        (g) => g.id,
      ),
    ).toEqual(["evg-prod"]);
  });

  it("never strands a workspace group (w8/179: it links to any service)", () => {
    expect(
      strandedGroupsForDestination(groups, ["srv-1"], "evm-prod").map(
        (g) => g.id,
      ),
    ).not.toContain("evg-workspace");
  });

  it("treats leaving every environment as the null destination", () => {
    expect(
      strandedGroupsForDestination(groups, ["srv-1"], null).map((g) => g.id),
    ).toEqual(["evg-prod", "evg-staging"]);
  });

  it("ignores other services' links and unlinked groups", () => {
    expect(strandedGroupsForDestination(groups, ["srv-9"], null)).toEqual([]);
    expect(
      strandedGroupsForDestination(groups, ["srv-2"], "evm-staging").map(
        (g) => g.id,
      ),
    ).toEqual(["evg-other-service"]);
  });
});
