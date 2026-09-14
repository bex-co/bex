import { useCallback } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  KeyValuePersistenceModeDocument,
  SetKeyValuePersistenceModeDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { persistenceModeToUi } from "@/features/keyvalue/lib/labels";

export interface UseSetKeyValuePersistenceModeResult {
  mode: string;
  loading: boolean;
  saving: boolean;
  save: (mode: string) => Promise<boolean>;
}

/**
 * Reads + edits a Key Value store's persistence mode — sibling of
 * useSetKeyValueMaxmemoryPolicy (w4/066 / w6/m127).
 */
export function useSetKeyValuePersistenceMode(
  id: string,
): UseSetKeyValuePersistenceModeResult {
  const { t } = useTranslations();

  const modeQuery = useQuery(KeyValuePersistenceModeDocument, {
    variables: { id },
    fetchPolicy: "cache-and-network",
    errorPolicy: "all",
  });
  const [setModeMut, { loading: saving }] = useMutation(
    SetKeyValuePersistenceModeDocument,
  );

  const mode = persistenceModeToUi(
    modeQuery.data?.keyValue?.persistenceMode ?? "",
  );

  const save = useCallback(
    async (next: string): Promise<boolean> => {
      try {
        await setModeMut({ variables: { id, persistenceMode: next } });
        toast.success(t("keyvalue.persistenceSuccess", { mode: next }));
        void modeQuery.refetch();
        return true;
      } catch {
        toast.error(t("keyvalue.persistenceError"));
        return false;
      }
    },
    [setModeMut, modeQuery, id, t],
  );

  return { mode, loading: modeQuery.loading, saving, save };
}
