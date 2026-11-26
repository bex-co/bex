import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import {
  RegistryCredentialsDocument,
  type RegistryCredentialsQuery,
} from "@/graphql/definitions";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
import type { RegistryCredentialView } from "@/features/registry-credentials/types";
import { isRenderRegistry } from "@/features/registry-credentials/lib/registries";

type RawCredential = NonNullable<
  NonNullable<RegistryCredentialsQuery["registryCredentials"]>[number]
>;

/** One GraphQL `RegistryCredential` row (list or detail read) as the view. */
export function toCredentialView(
  c: RawCredential & { id: string },
): RegistryCredentialView {
  return {
    id: c.id,
    name: c.name ?? c.host ?? "",
    registry: isRenderRegistry(c.registry) ? c.registry : null,
    host: c.host ?? "",
    username: c.username ?? "",
    expiresAt: c.expiresAt ?? null,
    status: c.status ?? "active",
    createdAt: c.createdAt ?? null,
  };
}

function toViews(
  raw: RegistryCredentialsQuery["registryCredentials"] | undefined,
): RegistryCredentialView[] {
  return (raw ?? [])
    .filter((c): c is RawCredential & { id: string } => !!c?.id)
    .map(toCredentialView);
}

export interface UseRegistryCredentialsResult {
  credentials: RegistryCredentialView[];
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
}

/**
 * Reads bex-api's `registryCredentials` — the workspace's stored external
 * image-registry credentials (w2/m14). Secrets are never requested (see the
 * feature's `.graphql` file); the server's read queries have no secret field
 * to return in the first place.
 */
export function useRegistryCredentials(): UseRegistryCredentialsResult {
  const { currentWorkspaceId } = useWorkspace();
  const resolved = currentWorkspaceId != null;
  const { data, loading, error, refetch } = useQuery(
    RegistryCredentialsDocument,
    {
      variables: { ownerId: currentWorkspaceId },
      skip: !resolved,
      fetchPolicy: "cache-and-network",
      errorPolicy: "all",
      pollInterval: RESOURCE_POLL_INTERVAL_MS,
      skipPollAttempt: skipPollWhenHidden,
    },
  );

  const credentials = useMemo(() => toViews(data?.registryCredentials), [data]);

  return { credentials, loading: !resolved || loading, error, refetch };
}
