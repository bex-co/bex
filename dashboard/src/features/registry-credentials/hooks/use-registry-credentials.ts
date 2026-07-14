import { useMemo } from "react";
import { useQuery } from "@apollo/client/react";
import {
  RegistryCredentialsDocument,
  type RegistryCredentialsQuery,
} from "@/graphql/definitions";
import type { RegistryCredentialView } from "@/features/registry-credentials/types";

type RawCredential = NonNullable<
  RegistryCredentialsQuery["registryCredentials"]
>[number];

function toViews(
  raw: RegistryCredentialsQuery["registryCredentials"] | undefined,
): RegistryCredentialView[] {
  return (raw ?? [])
    .filter((c): c is NonNullable<RawCredential> & { id: string } => !!c?.id)
    .map((c) => ({
      id: c.id,
      name: c.name ?? c.host ?? "",
      host: c.host ?? "",
      username: c.username ?? "",
      expiresAt: c.expiresAt ?? null,
      status: c.status ?? "active",
      createdAt: c.createdAt ?? null,
    }));
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
  const { data, loading, error, refetch } = useQuery(
    RegistryCredentialsDocument,
    { fetchPolicy: "cache-and-network", errorPolicy: "all" },
  );

  const credentials = useMemo(
    () => toViews(data?.registryCredentials),
    [data],
  );

  return { credentials, loading, error, refetch };
}
