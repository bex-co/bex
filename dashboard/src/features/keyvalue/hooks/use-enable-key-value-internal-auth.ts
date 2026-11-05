import { useCallback } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { EnableKeyValueInternalAuthDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";

/**
 * Render's "Enable Internal Authentication" (w5/m268): the mutation answers
 * `{id, internalAuth}`, which Apollo's normalized cache writes onto the
 * detail query's KeyValue, so the panel flips without a refetch.
 */
export function useEnableKeyValueInternalAuth() {
  const { t } = useTranslations();
  const [mutate, { loading }] = useMutation(EnableKeyValueInternalAuthDocument);
  const enable = useCallback(
    async (id: string, name: string): Promise<boolean> => {
      try {
        await mutate({ variables: { id } });
        toast.success(t("keyvalue.internalAuthEnabled", { name }));
        return true;
      } catch (err) {
        toast.error(mutationErrorMessage(err, t("keyvalue.internalAuthError")));
        return false;
      }
    },
    [mutate, t],
  );
  return { enable, loading };
}
