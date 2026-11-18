import { useQuery } from "@apollo/client/react";
import { OrganizationMembersDocument } from "@/graphql/definitions";
import { PRIMED_FETCH_POLICY } from "@/common/lib/fetch-policy";
import { useOrganizationLoginBlock } from "@/features/workspaces/hooks/use-workspace-login-block";
import type { OrganizationLoginBlock } from "@/features/workspaces/lib/workspace-login-block";
import type { Organization } from "@/features/organizations/types";

/**
 * Whether the org's own login requirements refuse the caller (w7/m466,
 * docs/ADR012-auth.md 9c), on every org page, including the two whose reads
 * the server exempts so a blocked person can still see why (the org itself:
 * Workspaces, Settings). Returns the recorded block, or null.
 *
 * While the org requires a way of signing in and no block is recorded, it
 * reads the org's member list: the one org read every role reaches that the
 * server holds to the requirement (a Guest who meets it is refused FORBIDDEN,
 * which is not a block). A refusal reaches the Apollo auth error link like
 * any other, which records the block; the dashboard never decides it. The
 * Members page reads the same list, so there it costs nothing more, and an
 * org that requires nothing is not asked.
 *
 * While blocked it does not ask again. Clearing the block ("check again")
 * lets it ask: a refusal leaves nothing in the cache (the list is non-null,
 * so the refused answer's data is null), so the read goes to the network.
 */
export function useOrganizationLoginCheck(
  organizationId: string,
  organization: Pick<
    Organization,
    "requireTwoFactor" | "requireGoogleLogin"
  > | null,
): OrganizationLoginBlock | null {
  const block = useOrganizationLoginBlock(organizationId);
  const requires =
    organization !== null &&
    (organization.requireTwoFactor || organization.requireGoogleLogin);
  useQuery(OrganizationMembersDocument, {
    variables: { organizationId },
    skip: !requires || block !== null,
    fetchPolicy: PRIMED_FETCH_POLICY,
    errorPolicy: "all",
  });
  return block;
}
