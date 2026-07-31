import { useQuery } from "@apollo/client/react";
import { RegistryCredentialDocument } from "@/graphql/definitions";
import type { RegistryCredentialView } from "@/features/registry-credentials/types";

export interface UseRegistryCredentialResult {
  credential: RegistryCredentialView | null;
  loading: boolean;
  error: boolean;
}

/**
 * A single credential's detail via the dedicated `registryCredential(id)` read
 * (w5/m60) — the edit dialog's readback source. Mount it only when the detail is
 * actually needed (the edit dialog mounts it on open); the secret is never part
 * of the read (structurally absent), so a prefill can only ever carry
 * name/host/username/expiry, never the stored token.
 */
export function useRegistryCredential(id: string): UseRegistryCredentialResult {
  const { data, loading, error } = useQuery(RegistryCredentialDocument, {
    variables: { id },
    fetchPolicy: "cache-and-network",
  });
  const raw = data?.registryCredential;
  const credential: RegistryCredentialView | null =
    raw && raw.id
      ? {
          id: raw.id,
          name: raw.name ?? raw.host ?? "",
          host: raw.host ?? "",
          username: raw.username ?? "",
          expiresAt: raw.expiresAt ?? null,
          status: raw.status ?? "active",
          createdAt: raw.createdAt ?? null,
        }
      : null;
  return { credential, loading, error: !!error };
}
