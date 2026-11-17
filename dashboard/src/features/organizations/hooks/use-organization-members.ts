import { useQuery } from "@apollo/client/react";
import {
  OrganizationInvitesDocument,
  OrganizationMembersDocument,
  type OrganizationInvitesQuery,
  type OrganizationMembersQuery,
} from "@/graphql/definitions";
import { isForbiddenError } from "@/common/lib/graphql-error";
import { PRIMED_FETCH_POLICY } from "@/common/lib/fetch-policy";

export type OrganizationMember =
  OrganizationMembersQuery["organizationMembers"][number];
export type OrganizationInvite =
  OrganizationInvitesQuery["organizationInvites"][number];

const NO_MEMBERS: OrganizationMember[] = [];
const NO_INVITES: OrganizationInvite[] = [];

export interface UseOrganizationMembersResult {
  members: OrganizationMember[];
  /** Pending org invites; read only for an Owner. */
  invites: OrganizationInvite[];
  /** True only while nothing has been read yet. */
  loading: boolean;
  /** FORBIDDEN: a Guest does not see the members (ADR115 D4). */
  forbidden: boolean;
  /** Any other failure, with no list on screen. */
  failed: boolean;
}

/**
 * The org's people with their org role and the workspaces each belongs to
 * (Owners and Members read it; a Guest is refused FORBIDDEN), and, for an
 * Owner, its pending invites. The members route's loader primes both.
 */
export function useOrganizationMembers(
  organizationId: string,
  isOwner: boolean,
): UseOrganizationMembersResult {
  const variables = { organizationId };
  const membersQuery = useQuery(OrganizationMembersDocument, {
    variables,
    fetchPolicy: PRIMED_FETCH_POLICY,
    errorPolicy: "all",
  });
  const invitesQuery = useQuery(OrganizationInvitesDocument, {
    variables,
    skip: !isOwner,
    fetchPolicy: PRIMED_FETCH_POLICY,
    errorPolicy: "all",
  });
  const members = membersQuery.data?.organizationMembers;
  const settled = !membersQuery.loading || membersQuery.data !== undefined;
  const forbidden = !members && isForbiddenError(membersQuery.error);

  return {
    members: members ?? NO_MEMBERS,
    invites: isOwner
      ? (invitesQuery.data?.organizationInvites ?? NO_INVITES)
      : NO_INVITES,
    loading: !settled,
    forbidden: settled && forbidden,
    failed: settled && !members && !forbidden,
  };
}
