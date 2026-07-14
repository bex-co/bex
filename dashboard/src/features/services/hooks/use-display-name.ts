import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetDisplayNameDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseDisplayNameResult {
  setDisplayName: (id: string, displayName: string) => Promise<boolean>;
  busy: boolean;
}

/** Wires the Settings name control to the App's mutable spec.displayName. */
export function useDisplayName(): UseDisplayNameResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(SetDisplayNameDocument);
  const [busy, setBusy] = useState(false);

  const setDisplayName = useCallback(
    async (id: string, displayName: string) => {
      setBusy(true);
      try {
        await mutate({ variables: { id, displayName } });
        toast.success(
          displayName
            ? t("services.displayNameSuccess", { name: displayName })
            : t("services.displayNameCleared"),
        );
        return true;
      } catch {
        toast.error(t("services.displayNameError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { setDisplayName, busy };
}
