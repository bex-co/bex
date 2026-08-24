import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { CreateProjectDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { conflictOrGenericMessage } from "@/common/lib/graphql-error";

export interface UseCreateProjectResult {
  /** Fires createProject; resolves the new id on success, null on failure. */
  create: (name: string) => Promise<string | null>;
  busy: boolean;
}

/**
 * Wires the "New Project" dialog to bex-api's `createProject` (w1/m31, bex
 * extension). Scoped to the switcher's selected workspace, mirroring
 * useCreateDatabase/useCreateKeyValue.
 */
export function useCreateProject(): UseCreateProjectResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const [mutate] = useMutation(CreateProjectDocument);
  const [busy, setBusy] = useState(false);

  const create = useCallback(
    async (name: string) => {
      if (currentWorkspaceId == null) {
        toast.error(t("projects.createError", { name }));
        return null;
      }
      setBusy(true);
      try {
        const res = await mutate({
          variables: { name, ownerId: currentWorkspaceId },
        });
        const id = res.data?.createProject?.id;
        if (!id) throw new Error("createProject returned no id");
        toast.success(t("projects.createSuccess", { name }));
        return id;
      } catch (err) {
        toast.error(
          conflictOrGenericMessage(err, t("projects.createError", { name })),
        );
        return null;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t, currentWorkspaceId],
  );

  return { create, busy };
}
