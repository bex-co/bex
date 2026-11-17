import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { OrganizationsDocument } from "@/graphql/definitions";
import { useHasVerifiedSession } from "@/common/hooks/use-has-verified-session";
import {
  toOrganization,
  type Organization,
} from "@/features/organizations/types";

const NONE: Organization[] = [];

export interface UseOrganizationsResult {
  /** The orgs the caller is in, with the workspaces each lets them see. */
  organizations: Organization[];
}

/**
 * bex-api's `organizations`: every org the caller belongs to, as Owner, Member
 * or Guest (w7/m464). The workspace switcher lists them, so an Owner who is in
 * none of an org's workspaces still reaches the org; the org pages' skeletons
 * read the caller's role from it. The org page's own reads and mutations
 * update its cached entries.
 */
export function useOrganizations(): UseOrganizationsResult {
  const verified = useHasVerifiedSession();
  const { data } = useQuery(OrganizationsDocument, {
    skip: !verified,
    fetchPolicy: "cache-first",
    errorPolicy: "all",
  });
  const raw = data?.organizations;
  const organizations = useMemo(
    () => (raw ? raw.map(toOrganization) : NONE),
    [raw],
  );
  return { organizations };
}
