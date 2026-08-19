import { useState, useCallback } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { UpdateBlueprintDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import type { BlueprintView } from "@/features/blueprints/types";
import { toBlueprintView } from "@/features/blueprints/lib/views";

export interface UseUpdateBlueprintResult {
  update: (
    id: string,
    fields: { name?: string; autoSync?: boolean; path?: string },
  ) => Promise<BlueprintView | null>;
  busy: boolean;
}

export function useUpdateBlueprint(): UseUpdateBlueprintResult {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const [mutate] = useMutation(UpdateBlueprintDocument);
  const [busy, setBusy] = useState(false);

  const update = useCallback(
    async (
      id: string,
      fields: { name?: string; autoSync?: boolean; path?: string },
    ): Promise<BlueprintView | null> => {
      setBusy(true);
      try {
        const res = await mutate({
          variables: { id, ownerId: currentWorkspaceId, ...fields },
        });
        toast.success(t("blueprints.updateSuccess"));
        return res.data?.updateBlueprint
          ? toBlueprintView(res.data.updateBlueprint)
          : null;
      } catch {
        toast.error(t("blueprints.updateError"));
        return null;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t, currentWorkspaceId],
  );

  return { update, busy };
}
