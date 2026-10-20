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

  it("flags groups scoped anywhere but the destination", () => {
    expect(
      strandedGroupsForDestination(groups, ["srv-1"], "evm-staging").map(
        (g) => g.id,
      ),
    ).toEqual(["evg-prod", "evg-workspace"]);
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
