import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { ApiKeysDocument, type ApiKeysQuery } from "@/graphql/definitions";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
import { isPermissionLevel } from "@/features/api-keys/lib/permissions";
import type { ApiKeyView } from "@/features/api-keys/types";
import { useWorkspace } from "@/features/workspaces/context/hooks";

// Derived from the generated query so new fields can't drift from the schema.
type RawKey = NonNullable<ApiKeysQuery["apiKeys"]>[number];

function toApiKeyViews(raw: ApiKeysQuery["apiKeys"] | undefined): ApiKeyView[] {
  return (raw ?? [])
    .filter((k): k is NonNullable<RawKey> & { id: string } => !!k?.id)
    .map((k) => ({
      id: k.id,
      name: k.name ?? "",
      createdAt: k.createdAt,
      createdBy: k.createdBy,
      lastUsedAt: k.lastUsedAt,
      permissions: {
        all: k.permissions.all,
        // bex-api checks every grant against the catalog, so each level is
        // one of Render's; narrowing drops nothing in practice.
        grants: k.permissions.grants.flatMap(({ permission, level }) =>
          isPermissionLevel(level) ? [{ permission, level }] : [],
        ),
      },
    }));
}

export interface UseApiKeysResult {
  keys: ApiKeyView[];
  loading: boolean;
  error: Error | undefined;
  /** Re-run the list query (callers refresh after mint/revoke). */
  refetch: () => Promise<unknown>;
}

/**
 * Reads bex-api's `apiKeys` — the bex-minted machine credentials the caller
 * created in the workspace, whatever their role, as on Render's Account
 * Settings; `allMembers` asks for every member's keys, a workspace Admin's
 * view (w9/m539, docs/ADR024-members.md). Neither a key nor its secret is ever requested
 * here (see the feature's `.graphql` file); only the create mutation returns them.
 * Scoped to the switcher's selected workspace (w6/m18): skipped until the
 * selection resolves to an id, mirroring useServices.
 */
export function useApiKeys(allMembers = false): UseApiKeysResult {
  const { currentWorkspaceId } = useWorkspace();
  const resolved = currentWorkspaceId != null;
  const { data, loading, error, refetch } = useQuery(ApiKeysDocument, {
    variables: allMembers
      ? { ownerId: currentWorkspaceId, allMembers: true }
      : { ownerId: currentWorkspaceId },
    skip: !resolved,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });

  const keys = useMemo(() => toApiKeyViews(data?.apiKeys), [data]);

  return { keys, loading: !resolved || loading, error, refetch };
}
