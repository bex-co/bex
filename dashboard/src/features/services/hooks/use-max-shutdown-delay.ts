import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetMaxShutdownDelayDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseMaxShutdownDelayResult {
  setMaxShutdownDelay: (id: string, seconds: number) => Promise<boolean>;
  busy: boolean;
}

/** Wires Settings to the shared maxShutdownDelaySeconds service verb. */
export function useMaxShutdownDelay(): UseMaxShutdownDelayResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(SetMaxShutdownDelayDocument);
  const [busy, setBusy] = useState(false);

  const setMaxShutdownDelay = useCallback(
    async (id: string, seconds: number) => {
      setBusy(true);
      try {
        await mutate({ variables: { id, seconds } });
        toast.success(t("services.maxShutdownDelaySuccess"));
        return true;
      } catch {
        toast.error(t("services.maxShutdownDelayError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { setMaxShutdownDelay, busy };
}
