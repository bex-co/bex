import { useCallback } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import {
  OwnerNotificationSettingsDocument,
  UpdateOwnerNotificationSettingsDocument,
  type OwnerNotificationSettingsFieldsFragment,
  type OwnerNotificationSettingsInput,
} from "@/graphql/definitions";

export type OwnerNotificationSettingsView =
  OwnerNotificationSettingsFieldsFragment;

export interface UseOwnerNotificationSettingsResult {
  settings: OwnerNotificationSettingsView | null;
  /** Loading with no data yet — the skeleton gate (dashboard/AGENTS.md). */
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
  /** Sends a partial update; resolves the saved policy or throws. */
  update: (
    input: OwnerNotificationSettingsInput,
  ) => Promise<OwnerNotificationSettingsView>;
  saving: boolean;
}

/**
 * Reads and writes a workspace's notification policy (w3/m271). The type is
 * keyed by `ownerId` in the Apollo cache, so a save also refreshes every
 * service page's "Use workspace default" label.
 */
export function useOwnerNotificationSettings(
  ownerId: string | null | undefined,
): UseOwnerNotificationSettingsResult {
  const { data, loading, error, refetch } = useQuery(
    OwnerNotificationSettingsDocument,
    {
      variables: { ownerId: ownerId ?? "" },
      skip: !ownerId,
      fetchPolicy: "cache-and-network",
    },
  );
  const [mutate, { loading: saving }] = useMutation(
    UpdateOwnerNotificationSettingsDocument,
  );

  const update = useCallback(
    async (input: OwnerNotificationSettingsInput) => {
      if (!ownerId) throw new Error("no workspace selected");
      const result = await mutate({ variables: { ownerId, input } });
      const saved = result.data?.updateOwnerNotificationSettings;
      if (!saved) {
        throw new Error("updateOwnerNotificationSettings returned no data");
      }
      return saved;
    },
    [mutate, ownerId],
  );

  const settings = data?.ownerNotificationSettings ?? null;
  return {
    settings,
    loading: !ownerId || (loading && settings == null),
    error,
    refetch,
    update,
    saving,
  };
}
