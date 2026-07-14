import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { CreateEnvironmentDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

export interface UseCreateEnvironmentResult {
  /** Fires createEnvironment; resolves the new id on success, null on failure. */
  create: (name: string) => Promise<string | null>;
  busy: boolean;
}

/**
 * Wires the "New Environment" dialog to bex-api's `createEnvironment` (w1/m32,
 * bex extension). Scoped to a project — the workspace is inherited from the
 * project server-side, so the caller only supplies the name.
 */
export function useCreateEnvironment(projectId: string): UseCreateEnvironmentResult {
  const { t } = useTranslations();
  // Refetch the active Environments list by name (the panel's query), the same
  // pattern useTriggerDeploy uses — so the new environment appears without any
  // caller threading a refetch callback back down.
  const [mutate] = useMutation(CreateEnvironmentDocument, {
    refetchQueries: ["Environments"],
    awaitRefetchQueries: true,
  });
  const [busy, setBusy] = useState(false);

  const create = useCallback(
    async (name: string) => {
      setBusy(true);
      try {
        const res = await mutate({ variables: { name, projectId } });
        const id = res.data?.createEnvironment?.id ?? null;
        toast.success(t("environments.createSuccess", { name }));
        return id;
      } catch {
        toast.error(t("environments.createError", { name }));
        return null;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t, projectId],
  );

  return { create, busy };
}
