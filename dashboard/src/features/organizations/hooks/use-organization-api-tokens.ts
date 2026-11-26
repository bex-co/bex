import { useCallback, useMemo } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import {
  OrganizationApiTokensDocument,
  ProvisionOrganizationAuditLogTokenDocument,
  RevokeOrganizationApiTokenDocument,
  type OrganizationApiTokensQuery,
} from "@/graphql/definitions";
import {
  hasGraphQLErrorCode,
  isThrottledError,
} from "@/common/lib/graphql-error";

export type OrganizationApiToken = NonNullable<
  OrganizationApiTokensQuery["organization"]
>["organizationAPITokens"][number];

const NO_TOKENS: OrganizationApiToken[] = [];

/** bex-api's refusal of a key past the org's quota or creation rate. */
const KEY_LIMIT_CODE = "ORGANIZATION_API_KEY_LIMIT";

/**
 * A create refused for too many keys (the quota or the creation rate), the
 * case Render words "You have created too many keys".
 */
export function isKeyLimitRefusal(err: unknown): boolean {
  return hasGraphQLErrorCode(err, KEY_LIMIT_CODE) || isThrottledError(err);
}

export interface UseOrganizationApiTokensResult {
  tokens: OrganizationApiToken[];
  /** True while nothing has been read yet. */
  loading: boolean;
  /** The list could not be read, and none is on screen. */
  failed: boolean;
  refetch: () => void;
}

/**
 * Render's organizationAPITokens (w1/m657): the org's audit-log keys. Only an
 * Owner asks, as only Render's manageOrganization does; anyone else sends no
 * query and sees the no-permission text instead.
 */
export function useOrganizationApiTokens(
  organizationId: string,
  canManage: boolean,
): UseOrganizationApiTokensResult {
  const { data, loading, error, refetch } = useQuery(
    OrganizationApiTokensDocument,
    {
      variables: { organizationId },
      skip: !canManage,
      fetchPolicy: "cache-and-network",
      notifyOnNetworkStatusChange: true,
      errorPolicy: "all",
    },
  );
  const tokens = data?.organization?.organizationAPITokens;
  const sorted = useMemo(
    () => (tokens ? byLastUse(tokens) : NO_TOKENS),
    [tokens],
  );
  return {
    tokens: sorted,
    loading: canManage && loading && !tokens,
    failed: canManage && !loading && !tokens && Boolean(error),
    refetch: useCallback(() => void refetch(), [refetch]),
  };
}

/**
 * Render's table opens sorted by last use, newest first; a key never used
 * comes after every used one, the newest created first among them.
 */
function byLastUse(tokens: OrganizationApiToken[]): OrganizationApiToken[] {
  return [...tokens].sort(
    (a, b) =>
      (b.lastUsedAt ?? "").localeCompare(a.lastUsedAt ?? "") ||
      b.createdAt.localeCompare(a.createdAt),
  );
}

/**
 * Render's provisionOrganizationAuditLogToken: the key, shown once. The
 * answer is never cached (`no-cache`), so closing the dialog is the last time
 * the key exists in the dashboard.
 */
export function useProvisionOrganizationApiToken(organizationId: string) {
  const [mutate, { loading }] = useMutation(
    ProvisionOrganizationAuditLogTokenDocument,
    { fetchPolicy: "no-cache" },
  );
  const provision = useCallback(
    async (label: string) => {
      const { data } = await mutate({ variables: { organizationId, label } });
      const token = data?.provisionOrganizationAuditLogToken.token;
      if (!token) throw new Error("Missing provisioned organization API key");
      return token;
    },
    [mutate, organizationId],
  );
  return { provision, busy: loading };
}

/** Render's revokeOrganizationAPIToken. */
export function useRevokeOrganizationApiToken(organizationId: string) {
  const [mutate, { loading }] = useMutation(RevokeOrganizationApiTokenDocument);
  const revoke = useCallback(
    async (tokenId: string) => {
      const { data } = await mutate({ variables: { organizationId, tokenId } });
      if (!data?.revokeOrganizationAPIToken) {
        throw new Error("Organization API key was not revoked");
      }
    },
    [mutate, organizationId],
  );
  return { revoke, busy: loading };
}
