import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import { ApiKeysDocument, type ApiKeysQuery } from "@/graphql/definitions";
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
 * Reads bex-api's `apiKeys` — the workspace's bex-minted machine credentials,
 * visible to any session with `can_manage_keys` (docs/ADR012-auth.md; not "my keys",
 * there's no per-user owner). Secrets are never requested here (see the
 * feature's `.graphql` file); only the create mutation ever returns one.
 * Scoped to the switcher's selected workspace (w6/m18): skipped until the
 * selection resolves to an id, mirroring useServices.
 */
export function useApiKeys(): UseApiKeysResult {
  const { currentWorkspaceId } = useWorkspace();
  const resolved = currentWorkspaceId != null;
  const { data, loading, error, refetch } = useQuery(ApiKeysDocument, {
    variables: { ownerId: currentWorkspaceId },
    skip: !resolved,
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });

  const keys = useMemo(() => toApiKeyViews(data?.apiKeys), [data]);

  return { keys, loading: !resolved || loading, error, refetch };
}
