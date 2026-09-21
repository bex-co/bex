import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetRegistryCredentialDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { useAskForProtectedConfirmation } from "@/common/providers/protected-retry-context";
import {
  ProtectedConfirmationDismissed,
  withProtectedRetry,
} from "@/features/services/lib/protected-confirmation";

export interface UseRegistryCredentialResult {
  setRegistryCredential: (
    serviceId: string,
    registryCredentialId: string,
  ) => Promise<boolean>;
  busy: boolean;
}

/** Binds, changes, or explicitly clears a service's registry credential. */
export function useRegistryCredential(): UseRegistryCredentialResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(SetRegistryCredentialDocument);
  const [busy, setBusy] = useState(false);
  const askForConfirmation = useAskForProtectedConfirmation();

  const setRegistryCredential = useCallback(
    async (serviceId: string, registryCredentialId: string) => {
      setBusy(true);
      try {
        // Rebinding the pull credential repoints where the image comes from,
        // which a protected environment guards (w4/m126).
        await withProtectedRetry(askForConfirmation, (confirm) =>
          mutate({
            variables: { id: serviceId, registryCredentialId, confirm },
          }),
        );
        toast.success(
          registryCredentialId
            ? t("services.registryCredentialSaved")
            : t("services.registryCredentialCleared"),
        );
        return true;
      } catch (err) {
        if (err instanceof ProtectedConfirmationDismissed) return false;
        toast.error(
          mutationErrorMessage(err, t("services.registryCredentialError")),
        );
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t, askForConfirmation],
  );

  return { setRegistryCredential, busy };
}
