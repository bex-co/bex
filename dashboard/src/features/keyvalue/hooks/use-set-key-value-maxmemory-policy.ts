import { useCallback } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  KeyValueMaxmemoryPolicyDocument,
  SetKeyValueMaxmemoryPolicyDocument,
} from "@/features/keyvalue/api/operations";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseSetKeyValueMaxmemoryPolicyResult {
  /** The store's current eviction policy, or "" until the read resolves. */
  policy: string;
  /** True while the current policy is still being read. */
  loading: boolean;
  /** True while a change is in flight. */
  saving: boolean;
  /** Persist a new policy; resolves true on success, false on failure. */
  save: (policy: string) => Promise<boolean>;
}

/**
 * Reads + edits a Key Value store's maxmemory (eviction) policy — the detail
 * page's Maxmemory Policy control, mirroring useKeyValueNetworking (a focused
 * read paired with its setter, refetching on save so the card reflects what the
 * operator will apply). The setter rides `setKeyValueMaxmemoryPolicy`, the
 * GraphQL mirror of the REST PATCH's maxmemoryPolicy field (w7/m45); the policy
 * is already updatable on every programmatic surface, so this only closes the
 * dashboard gap (w7/007).
 */
export function useSetKeyValueMaxmemoryPolicy(
  id: string,
): UseSetKeyValueMaxmemoryPolicyResult {
  const { t } = useTranslations();

  const policyQuery = useQuery(KeyValueMaxmemoryPolicyDocument, {
    variables: { id },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });
  const [setPolicyMut, { loading: saving }] = useMutation(
    SetKeyValueMaxmemoryPolicyDocument,
  );

  const policy = policyQuery.data?.keyValue?.maxmemoryPolicy ?? "";

  const save = useCallback(
    async (next: string): Promise<boolean> => {
      try {
        await setPolicyMut({ variables: { id, maxmemoryPolicy: next } });
        toast.success(t("keyvalue.maxmemorySuccess", { policy: next }));
        void policyQuery.refetch();
        return true;
      } catch {
        toast.error(t("keyvalue.maxmemoryError"));
        return false;
      }
    },
    [setPolicyMut, policyQuery, id, t],
  );

  return { policy, loading: policyQuery.loading, saving, save };
}
