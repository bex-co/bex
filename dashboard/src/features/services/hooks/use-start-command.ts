import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetStartCommandDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseStartCommandResult {
  setStartCommand: (id: string, command: string) => Promise<boolean>;
  busy: boolean;
}

/** Wires the Build & Deploy Start/Docker Command editor to bex-api. */
export function useStartCommand(): UseStartCommandResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(SetStartCommandDocument);
  const [busy, setBusy] = useState(false);

  const setStartCommand = useCallback(
    async (id: string, command: string) => {
      setBusy(true);
      try {
        await mutate({ variables: { id, command } });
        toast.success(t("services.startCommandSuccess"));
        return true;
      } catch {
        toast.error(t("services.startCommandError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { setStartCommand, busy };
}
