import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetBuildCommandDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseBuildCommandResult {
  setBuildCommand: (id: string, command: string) => Promise<boolean>;
  busy: boolean;
}

/** Wires the Build & Deploy Build Command editor to bex-api. */
export function useBuildCommand(): UseBuildCommandResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(SetBuildCommandDocument);
  const [busy, setBusy] = useState(false);

  const setBuildCommand = useCallback(
    async (id: string, command: string) => {
      setBusy(true);
      try {
        await mutate({ variables: { id, command } });
        toast.success(t("services.buildCommandSuccess"));
        return true;
      } catch {
        toast.error(t("services.buildCommandError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { setBuildCommand, busy };
}
