import { describe, expect, it } from "vitest";
import { OrganizationDocument } from "@/graphql/definitions";
import { loadOrganizationRoute } from "@/features/organizations/lib/organization-route-loader";
import {
  organization,
  scriptedClient,
} from "@/features/organizations/__tests__/org-fixtures";
import type { RouterContext } from "@/common/types/router-context";
import type { OrganizationRole } from "@/features/organizations/types";

/**
 * The Members loader primes the list (but for a Guest) and an Owner's invites
 * by the role the cache already holds (w4/706): the org's own entry, which
 * the previous org tab read, counts even when the caller's org list was
 * never fetched (a phone, where the switcher is not mounted).
 */
async function membersEntry(cachedEntry: OrganizationRole | null) {
  const { client, sent } = scriptedClient({
    Organization: () => ({ organization: organization("OWNER") }),
    OrganizationMembers: () => ({ organizationMembers: [] }),
    OrganizationInvites: () => ({ organizationInvites: [] }),
  });
  if (cachedEntry) {
    client.writeQuery({
      query: OrganizationDocument,
      variables: { id: "org-1" },
      data: { organization: organization(cachedEntry) },
    });
  }
  await loadOrganizationRoute(
    {
      context: { client } as unknown as RouterContext,
      params: { orgId: "org-1" },
      cause: "enter",
    },
    "members",
  );
  return sent.map((operation) => operation.name).sort();
}

describe("loadOrganizationRoute members (w4/706)", () => {
  it("primes an Owner's invites from the org's own cached entry", async () => {
    expect(await membersEntry("OWNER")).toEqual([
      "Organization",
      "OrganizationInvites",
      "OrganizationMembers",
    ]);
  });

  it("primes no invites for a Member, and no list for a Guest", async () => {
    expect(await membersEntry("MEMBER")).toEqual([
      "Organization",
      "OrganizationMembers",
    ]);
    expect(await membersEntry("GUEST")).toEqual(["Organization"]);
  });

  it("primes only the list while no role is known", async () => {
    expect(await membersEntry(null)).toEqual([
      "Organization",
      "OrganizationMembers",
    ]);
  });
});
