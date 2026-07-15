import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetDockerfilePathDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseDockerfilePathResult {
  setDockerfilePath: (id: string, dockerfilePath: string) => Promise<boolean>;
  busy: boolean;
}

/** Wires the Dockerfile Path editor to bex-api's existing-service setter. */
export function useDockerfilePath(): UseDockerfilePathResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(SetDockerfilePathDocument);
  const [busy, setBusy] = useState(false);

  const setDockerfilePath = useCallback(
    async (id: string, dockerfilePath: string) => {
      setBusy(true);
      try {
        await mutate({ variables: { id, dockerfilePath } });
        toast.success(t("services.dockerfilePathSuccess"));
        return true;
      } catch {
        toast.error(t("services.dockerfilePathError"));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { setDockerfilePath, busy };
}
