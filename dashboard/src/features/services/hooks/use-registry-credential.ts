import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetRegistryCredentialDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

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

  const setRegistryCredential = useCallback(
    async (serviceId: string, registryCredentialId: string) => {
      setBusy(true);
      try {
        await mutate({ variables: { id: serviceId, registryCredentialId } });
        toast.success(
          registryCredentialId
            ? t("services.registryCredentialSaved")
            : t("services.registryCredentialCleared"),
        );
        return true;
      } catch {
        toast.error(t("services.registryCredentialError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { setRegistryCredential, busy };
}
