import { useCallback } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  KeyValueIpAllowListDocument,
  SetKeyValueIpAllowListDocument,
} from "@/graphql/definitions";
import type { IPAllowListEntryDraft } from "@/common/lib/ip-allow-list";
import {
  RESOURCE_POLL_INTERVAL_MS,
  skipPollWhenHidden,
} from "@/common/lib/polling";
import { useTranslations } from "@/common/hooks/use-translations";
import { graphQLErrorExtensions } from "@/common/lib/graphql-error";

/**
 * bex-api refused the save until the user confirms the restart it causes: a
 * Persistence Off store's first external-access enable, which empties it
 * (w4/m233). confirm is the server's retry phrase; the dashboard never builds
 * it.
 */
export interface RestartRefusal {
  confirm: string;
  name: string;
}

const RESTART_REFUSAL = "KEY_VALUE_RESTART_CONFIRMATION_REQUIRED";

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
  const allowList: IPAllowListEntryDraft[] = (
    allowListQuery.data?.keyValue?.ipAllowListEntries ?? []
  )
    .filter((e): e is NonNullable<typeof e> => e != null)
    .map((e) => ({ cidrBlock: e.cidrBlock, description: e.description ?? "" }));

  const saveAllowList = useCallback(
    async (
      entries: IPAllowListEntryDraft[],
      confirm?: string,
    ): Promise<boolean | RestartRefusal> => {
      try {
        await setAllowListMut({ variables: { id, entries, confirm } });
        toast.success(t("keyvalue.networkingSaved"));
        void allowListQuery.refetch();
        return true;
      } catch (e) {
        const refusal = graphQLErrorExtensions(e, RESTART_REFUSAL);
        if (typeof refusal?.["confirm"] === "string") {
          const name = refusal["name"];
          return {
            confirm: refusal["confirm"],
            name: typeof name === "string" ? name : "",
          };
        }
        toast.error(
          t("keyvalue.networkingError", { error: (e as Error).message }),
        );
        return false;
      }
    },
    [setAllowListMut, allowListQuery, id, t],
  );

  const keyValue = allowListQuery.data?.keyValue;
  return {
    allowList,
    isPublic: keyValue?.public,
    // Enabling external access restarts the store (its first enable, w4/m233).
    externalAccessRestarts: keyValue?.externalAccessRestarts === true,
    loading: allowListQuery.loading,
    savingAllowList,
    saveAllowList,
  };
}
