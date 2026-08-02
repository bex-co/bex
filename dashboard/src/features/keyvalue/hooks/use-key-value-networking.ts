import { useCallback } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  KeyValueIpAllowListDocument,
  SetKeyValueIpAllowListDocument,
  type IpAllowListEntry,
} from "@/features/keyvalue/api/operations";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * Reads + edits a Key Value store's external-endpoint IP allowlist — the
 * Networking control, the allowlist half of databases' useAccessControl (Key
 * Value has no extra login roles or pooler, so this hook stays allowlist-only).
 * The save refetches so the panel reflects what the operator will project.
 */
export function useKeyValueNetworking(id: string) {
  const { t } = useTranslations();

  const allowListQuery = useQuery(KeyValueIpAllowListDocument, {
    variables: { id },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
    pollInterval: RESOURCE_POLL_INTERVAL_MS,
    skipPollAttempt: skipPollWhenHidden,
  });
  const [setAllowListMut, { loading: savingAllowList }] = useMutation(
    SetKeyValueIpAllowListDocument,
  );

  // Entries, not bare CIDRs: each carries the operator-facing description a
  // human gave it, which persists end to end (w4/m24).
  const allowList: IpAllowListEntry[] = (
    allowListQuery.data?.keyValue?.ipAllowListEntries ?? []
  )
    .filter((e): e is NonNullable<typeof e> => e != null)
    .map((e) => ({ cidrBlock: e.cidrBlock, description: e.description ?? "" }));

  const saveAllowList = useCallback(
    async (entries: IpAllowListEntry[]): Promise<boolean> => {
      try {
        await setAllowListMut({ variables: { id, entries } });
        toast.success(t("keyvalue.networkingSaved"));
        void allowListQuery.refetch();
        return true;
      } catch (e) {
        toast.error(
          t("keyvalue.networkingError", { error: (e as Error).message }),
        );
        return false;
      }
    },
    [setAllowListMut, allowListQuery, id, t],
  );

  return {
    allowList,
    loading: allowListQuery.loading,
    savingAllowList,
    saveAllowList,
  };
}
