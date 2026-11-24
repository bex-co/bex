import { useState } from "react";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import type { SavedSearch } from "@/features/projects/hooks/use-saved-searches";
import { useSavedSearchMutations } from "@/features/projects/hooks/use-saved-search-mutations";
import { useOpenView } from "../hooks/use-open-view";
import { EditViewDialog } from "./edit-view-dialog";
import { ViewActionButtons } from "./observability-view-header";

/**
 * A view's header buttons (w3/m507), Render's three, which every view has,
 * the defaults too: **Edit view** opens the modal, **Duplicate** copies the
 * view and opens the copy, and the trash button deletes the view after a
 * typed confirmation and lands on the project's first remaining view, or its
 * overview when none is left. A member who may not edit the project sees
 * them disabled with the reason (docs/ADR018/surface-pins-and-corrections.md, Role-aware
 * dashboard controls);
 * bex-api refuses them too.
 */
export function ObservabilityViewActions({
  projectId,
  view,
  onLeaving,
}: {
  projectId: string;
  view: SavedSearch;
  /** The view is deleted and about to leave the list: the page holds its
   *  pending state until the next view opens, not "View not found". */
  onLeaving: (id: string | null) => void;
}) {
  const { t } = useTranslations();
  const capabilities = useCapabilities();
  const reasonKey = capabilities.reasonKey("can_create");
  const { duplicate, remove, busy } = useSavedSearchMutations(projectId);
  const openView = useOpenView(projectId);
  const [editing, setEditing] = useState(false);
  const [confirming, setConfirming] = useState(false);

  async function duplicateView() {
    try {
      const copy = await duplicate(view.id);
      toast.success(t("logs.viewDuplicated", { name: copy.name }));
      await openView(copy);
    } catch (err) {
      toast.error(mutationErrorMessage(err, t("logs.duplicateViewFailed")));
    }
  }

  async function deleteView() {
    let left: SavedSearch[];
    try {
      left = await remove(view.id, () => onLeaving(view.id));
    } catch (err) {
      toast.error(mutationErrorMessage(err, t("logs.deleteViewFailed")));
      return;
    }
    toast.success(t("logs.viewDeleted", { name: view.name }));
    try {
      await openView(left[0] ?? null);
    } finally {
      onLeaving(null);
    }
  }

  return (
    <>
      <ViewActionButtons
        disabled={!capabilities.canCreate || busy !== null}
        reason={capabilities.canCreate || !reasonKey ? undefined : t(reasonKey)}
        onEdit={() => setEditing(true)}
        onDuplicate={() => void duplicateView()}
        onDelete={() => setConfirming(true)}
      />
      <EditViewDialog
        projectId={projectId}
        target={editing ? { kind: "edit", view } : null}
        onOpenChange={setEditing}
      />
      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        title={t("logs.deleteViewTitle", { name: view.name })}
        description={t("logs.deleteViewBody")}
        phrase={`sudo delete view ${view.name}`}
        confirmLabel={
          busy === "delete" ? (
            <>
              <Loader2 className="animate-spin" />
              {t("logs.deleteView")}
            </>
          ) : (
            t("logs.deleteView")
          )
        }
        pending={busy === "delete"}
        closeOnConfirm={false}
        onConfirm={() => void deleteView()}
      />
    </>
  );
}
