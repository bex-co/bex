import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { PlanBlueprintSyncDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";

/** Where Manual sync's plan sends the user, or why it could not plan. */
export type PlanBlueprintSyncResult =
  | { status: "planned"; syncId: string }
  | { status: "no_changes" }
  | { status: "error"; reason: string };

/**
 * Manual sync, as Render's (w3/m678, docs/render-artifacts/blueprint-pages.md
 * § Manual sync): plan a sync at the branch head (`planBlueprintSync`), whose
 * page the caller opens. A plan that saves a sync, planned or failed, names
 * it; a plan with no changes saves none. A refused plan's reason is the
 * server's, for Render's Sync Error banner.
 */
export function usePlanBlueprintSync() {
  const { t } = useTranslations();
  // A plan with no changes takes a created Blueprint out of created. Syncs
  // reads its list again when it mounts.
  const [mutate] = useMutation(PlanBlueprintSyncDocument, {
    refetchQueries: ["Blueprint"],
  });
  const [busy, setBusy] = useState(false);
  const plan = useCallback(
    async (blueprintId: string): Promise<PlanBlueprintSyncResult> => {
      setBusy(true);
      try {
        const res = await mutate({ variables: { id: blueprintId } });
        const syncId = res.data?.planBlueprintSync?.sync?.id;
        if (syncId) return { status: "planned", syncId };
        if (res.data?.planBlueprintSync?.planningOutcome === "no_changes") {
          return { status: "no_changes" };
        }
        return {
          status: "error",
          reason: t("blueprints.plannedReplanFailed"),
        };
      } catch (err) {
        return {
          status: "error",
          reason: mutationErrorMessage(
            err,
            t("blueprints.plannedReplanFailed"),
          ),
        };
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );
  return { plan, busy };
}
