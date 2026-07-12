import { useCallback } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  KeyValueIpAllowListDocument,
  SetKeyValueIpAllowListDocument,
} from "@/features/keyvalue/api/operations";
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
  });
  const [setAllowListMut, { loading: savingAllowList }] = useMutation(
    SetKeyValueIpAllowListDocument,
  );

  const allowList: string[] = (
    allowListQuery.data?.keyValueIpAllowList ?? []
  ).filter((c): c is string => typeof c === "string");

  const saveAllowList = useCallback(
    async (cidrs: string[]): Promise<boolean> => {
      try {
        await setAllowListMut({ variables: { id, cidrs } });
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
