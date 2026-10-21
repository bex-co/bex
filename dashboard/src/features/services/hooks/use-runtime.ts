import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetRuntimeDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { useAskForProtectedConfirmation } from "@/common/providers/protected-retry-context";
import {
  ProtectedConfirmationDismissed,
  withProtectedRetry,
} from "@/features/services/lib/protected-confirmation";

export interface UseRuntimeResult {
  setRuntime: (id: string, runtime: string) => Promise<boolean>;
  saving: boolean;
}

/**
 * Wires the Settings Runtime section to bex-api's `setRuntime` (Render's
 * runtime change), which rebuilds and redeploys the service. The mutation
 * returns the Service's `runtime` and `builder`, which Apollo merges into the
 * cached `server` document, so the page re-reads both on success. A refusal
 * (a native runtime without build/start commands, an image-backed service)
 * toasts the server's message.
 */
export function useRuntime(): UseRuntimeResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(SetRuntimeDocument);
  const [saving, setSaving] = useState(false);
  const askForConfirmation = useAskForProtectedConfirmation();

  const setRuntime = useCallback(
    async (id: string, runtime: string) => {
      setSaving(true);
      try {
        await withProtectedRetry(askForConfirmation, (confirm) =>
          mutate({ variables: { id, runtime, confirm } }),
        );
        toast.success(t("services.runtimeSaved"));
        return true;
      } catch (err) {
        if (err instanceof ProtectedConfirmationDismissed) return false;
        toast.error(mutationErrorMessage(err, t("services.runtimeError")));
        return false;
      } finally {
        setSaving(false);
      }
    },
    [mutate, t, askForConfirmation],
  );

  return { setRuntime, saving };
}
