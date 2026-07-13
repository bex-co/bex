import { useState } from "react";
import { Loader2 } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/common/components/ui/dialog";
import { Button } from "@/common/components/ui/button";
import {
  Alert,
  AlertTitle,
  AlertDescription,
} from "@/common/components/ui/alert";
import { useTranslations } from "@/common/hooks/use-translations";
import { useChangeWorkspacePlan } from "@/features/workspaces/hooks/use-change-workspace-plan";
import { PlanPicker } from "@/features/workspaces/components/plan-picker";
import type { WorkspacePlanId, WorkspaceView } from "@/features/workspaces/types";

export interface ChangePlanDialogProps {
  workspace: WorkspaceView;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Called after a successful change so the caller can refetch the workspace. */
  onChanged: () => void;
}

/**
 * The workspace settings plan section's change-plan dialog (w6/m12/t005):
 * Render's plan cards (reusing `/new/workspace`'s `PlanPicker`) plus the
 * `changeWorkspacePlan` mutation. A blocked downgrade (member/service/
 * per-user-cap/role-set guard) surfaces the backend's exact copy inline —
 * not just a toast — so the user knows what to remove before retrying.
 */
export function ChangePlanDialog({
  workspace,
  open,
  onOpenChange,
  onChanged,
}: ChangePlanDialogProps) {
  const { t } = useTranslations();
  const { changePlan, busy, error } = useChangeWorkspacePlan();
  const [plan, setPlan] = useState<WorkspacePlanId>(
    workspace.plan as WorkspacePlanId,
  );

  const canSubmit = plan !== workspace.plan && !busy;

  async function handleSubmit() {
    if (!canSubmit) return;
    const ok = await changePlan(workspace.id, plan);
    if (!ok) return;
    onChanged();
    onOpenChange(false);
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!busy) onOpenChange(next);
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("workspaces.changePlanTitle")}</DialogTitle>
          <DialogDescription>
            {t("workspaces.changePlanDescription")}
          </DialogDescription>
        </DialogHeader>

        <PlanPicker selected={plan} onSelect={setPlan} />

        {error ? (
          <Alert variant="destructive">
            <AlertTitle>{t("workspaces.changePlanErrorTitle")}</AlertTitle>
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        ) : null}

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={busy}
          >
            {t("workspaces.changePlanCancel")}
          </Button>
          <Button onClick={() => void handleSubmit()} disabled={!canSubmit}>
            {busy ? <Loader2 className="animate-spin" /> : null}
            {t("workspaces.changePlanSubmit")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
