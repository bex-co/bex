import { useQuery } from "@apollo/client/react";
import { SshKeysDocument } from "@/graphql/definitions";

export interface HasSSHKeyState {
  /** True only when the caller is confirmed to have ≥1 registered key. */
  hasKey: boolean;
  /** The shared query is still resolving (no cached answer yet). */
  loading: boolean;
  /** The shared query errored. */
  error: boolean;
}

/**
 * Shared, app-cached "does the caller have any SSH key?" selector for the
 * `RequiresSshKey` gate (w2/m66). It reuses the exact `SshKeys` query the
 * settings panel drives (`use-ssh-keys.ts`), read `cache-first`, so every gated
 * affordance shares one cache entry instead of issuing a query per button — and
 * a key added on the settings page flips every gate the moment that query's
 * cache updates.
 *
 * Fail-open by contract: `loading` and `error` are surfaced so the gate can
 * treat an unknown answer as "assume yes" and never hide a working feature on
 * the guard's own trouble.
 */
export function useHasSSHKey(): HasSSHKeyState {
  const { data, loading, error } = useQuery(SshKeysDocument, {
    fetchPolicy: "cache-first",
    errorPolicy: "all",
  });
  const hasKey = (data?.sshKeys ?? []).some((key) => key != null && !!key.id);
  return { hasKey, loading, error: error != null };
}
