import { useState } from "react";
import { Loader2 } from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/common/components/ui/card";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { Button } from "@/common/components/ui/button";
import { Badge } from "@/common/components/ui/badge";
import {
  Alert,
  AlertTitle,
  AlertDescription,
} from "@/common/components/ui/alert";
import { useTranslations } from "@/common/hooks/use-translations";
import { useRenameWorkspace } from "@/features/workspaces/hooks/use-rename-workspace";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { ChangePlanDialog } from "@/features/workspaces/components/change-plan-dialog";
import {
  WORKSPACE_NAME_RE,
  WORKSPACE_PLAN_CATALOG,
  type WorkspaceView,
} from "@/features/workspaces/types";

function planLabelKey(plan: string): string {
  return (
    WORKSPACE_PLAN_CATALOG.find((p) => p.id === plan)?.nameKey ?? "" // unknown plan: fall through to the raw id below
  );
}

export interface WorkspaceDetailsCardProps {
  workspace: WorkspaceView;
  /**
   * Whether the change-plan dialog is open. Owned by the route, which keeps it
   * in the URL (`/workspace/settings?plan=change`) — that's how the blocked-
   * invite CTA opens it from another page (w6/m15/t001). Controlled rather than
   * seeded at mount so the URL stays the single source of truth: closing the
   * dialog clears the param instead of leaving it to re-open on the next visit.
   */
  changePlanOpen: boolean;
  onChangePlanOpenChange: (open: boolean) => void;
}

/**
 * Workspace settings' primary card (w6/m3/t003, plan section w6/m12/t005):
 * rename, the plan as a badge with a change-plan dialog (upgrade/downgrade —
 * no payment step, bex has no billing system yet, .pm/w6/README.md "Not in
 * w6"), and the id/created-at metadata Render's own settings page shows
 * alongside the name.
 */
export function WorkspaceDetailsCard({
  workspace,
  changePlanOpen,
  onChangePlanOpenChange,
}: WorkspaceDetailsCardProps) {
  const { t } = useTranslations();
  const { rename, busy, error } = useRenameWorkspace();
  const { refetch } = useWorkspace();

  const [name, setName] = useState(workspace.name);

  const nameValid = WORKSPACE_NAME_RE.test(name);
  const showNameError = name.length > 0 && !nameValid;
  const canSave = nameValid && name !== workspace.name && !busy;

  async function handleSave() {
    if (!canSave) return;
    const ok = await rename(workspace.id, name);
    if (ok) await refetch();
  }

  const planNameKey = planLabelKey(workspace.plan);

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("workspaces.settingsTitle")}</CardTitle>
        <CardDescription>{t("workspaces.settingsDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="space-y-2">
          <Label htmlFor="workspace-settings-name">
            {t("workspaces.fieldName")}
          </Label>
          <div className="flex gap-2">
            <Input
              id="workspace-settings-name"
              value={name}
              onChange={(e) => setName(e.target.value.toLowerCase())}
              autoComplete="off"
              aria-invalid={showNameError}
              className="max-w-sm"
            />
            <Button onClick={() => void handleSave()} disabled={!canSave}>
              {busy ? <Loader2 className="animate-spin" /> : null}
              {t("workspaces.renameSubmit")}
            </Button>
          </div>
          {showNameError ? (
            <p className="text-sm text-destructive">
              {t("workspaces.fieldNameError")}
            </p>
          ) : null}
        </div>

        {error ? (
          <Alert variant="destructive">
            <AlertTitle>{t("workspaces.renameErrorTitle")}</AlertTitle>
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        ) : null}

        <dl className="grid grid-cols-1 gap-4 border-t pt-4 text-sm sm:grid-cols-3">
          <div className="space-y-1">
            <dt className="text-muted-foreground">
              {t("workspaces.fieldPlan")}
            </dt>
            <dd className="flex items-center gap-2">
              <Badge variant="secondary">
                {planNameKey ? t(planNameKey) : workspace.plan}
              </Badge>
              <Button
                variant="link"
                className="h-auto p-0"
                onClick={() => onChangePlanOpenChange(true)}
              >
                {t("workspaces.changePlanTrigger")}
              </Button>
            </dd>
          </div>
          <div className="space-y-1">
            <dt className="text-muted-foreground">{t("workspaces.fieldId")}</dt>
            <dd className="font-mono text-xs">{workspace.id}</dd>
          </div>
          <div className="space-y-1">
            <dt className="text-muted-foreground">
              {t("workspaces.fieldCreatedAt")}
            </dt>
            <dd>{workspace.createdAt ?? "—"}</dd>
          </div>
        </dl>
      </CardContent>

      <ChangePlanDialog
        workspace={workspace}
        open={changePlanOpen}
        onOpenChange={onChangePlanOpenChange}
        onChanged={() => void refetch()}
      />
    </Card>
  );
}
