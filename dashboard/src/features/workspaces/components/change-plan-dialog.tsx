import { useState } from "react";
import { Loader2, TriangleAlert } from "lucide-react";
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
import {
  useTranslations,
  type Translate,
} from "@/common/hooks/use-translations";
import { conjunctionList } from "@/common/lib/format/language";
import {
  useChangeWorkspacePlan,
  type DowngradePrecondition,
} from "@/features/workspaces/hooks/use-change-workspace-plan";
import { PlanPicker } from "@/features/workspaces/components/plan-picker";
import {
  organizationPlan,
  organizationPrimary,
  type WorkspacePlanId,
  type WorkspaceView,
} from "@/features/workspaces/types";
import { ROLES } from "@/features/team/types";

export interface ChangePlanDialogProps {
  workspace: WorkspaceView;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Called after a successful change so the caller can refetch the workspace. */
  onChanged: () => void;
}

const PRECONDITION_LABELS: Record<DowngradePrecondition["kind"], string> = {
  secondary_workspaces: "workspaces.orgDowngradeSecondaryWorkspaces",
  member_roles: "workspaces.orgDowngradeMemberRoles",
  invite_roles: "workspaces.orgDowngradeInviteRoles",
  inbound_ip_rules: "workspaces.orgDowngradeInboundIpRules",
};

/** A "subject:role" or "email:role" pair as "who (Role)". */
function personWithRole(item: string, t: Translate): string {
  const at = item.lastIndexOf(":");
  if (at <= 0) return item;
  const role = item.slice(at + 1);
  const known = ROLES.find((r) => r === role.toUpperCase());
  return t("workspaces.orgDowngradePersonRole", {
    person: item.slice(0, at),
    role: known ? t(`team.role.${known}`) : role,
  });
}

/** "workspace", "environment:<name>" or "service:<name>" in words. */
function ipRuleHolder(item: string, t: Translate): string {
  if (item === "workspace") return t("workspaces.orgDowngradeIpRuleWorkspace");
  const at = item.indexOf(":");
  const kind = item.slice(0, at);
  const name = item.slice(at + 1);
  if (kind === "environment") {
    return t("workspaces.orgDowngradeIpRuleEnvironment", { name });
  }
  if (kind === "service") {
    return t("workspaces.orgDowngradeIpRuleService", { name });
  }
  return item;
}

/** One unmet precondition as one line: what to do, then what to do it to. */
function preconditionLine(
  precondition: DowngradePrecondition,
  t: Translate,
  list: Intl.ListFormat,
): string {
  const label = t(PRECONDITION_LABELS[precondition.kind]);
  if (precondition.items.length === 0) return label;
  const items = precondition.items.map((item) => {
    switch (precondition.kind) {
      case "member_roles":
      case "invite_roles":
        return personWithRole(item, t);
      case "inbound_ip_rules":
        return ipRuleHolder(item, t);
      default:
        return item;
    }
  });
  return t("workspaces.orgDowngradeItems", {
    label,
    items: list.format(items),
  });
}

/**
 * The workspace settings plan section's change-plan dialog (w6/m12/t005):
 * Render's plan cards (reusing `/new/workspace`'s `PlanPicker`) plus the
 * `changeWorkspacePlan` mutation. A blocked downgrade (member/service/
 * per-user-cap/role-set guard) surfaces the backend's exact copy inline —
 * not just a toast — so the user knows what to remove before retrying.
 *
 * On an organization's primary (w7/m464, ADR115 D6) the plan is the org's: a
 * plan below Scale is warned about before it is sent, since it deletes the
 * org, and ORGANIZATION_DOWNGRADE_BLOCKED's unmet preconditions are listed one
 * line per kind. A secondary never opens this dialog: its plan is managed on
 * the primary (`WorkspaceDetailsCard`).
 */
export function ChangePlanDialog({
  workspace,
  open,
  onOpenChange,
  onChanged,
}: ChangePlanDialogProps) {
  const { t, i18n } = useTranslations();
  const { changePlan, busy, error, preconditions } = useChangeWorkspacePlan();
  const [plan, setPlan] = useState<WorkspacePlanId>(
    workspace.plan as WorkspacePlanId,
  );

  const canSubmit = plan !== workspace.plan && !busy;
  const deletesOrganization =
    organizationPrimary(workspace) && !organizationPlan(plan);
  const list = preconditions.length > 0 ? conjunctionList(i18n.language) : null;

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

        {deletesOrganization ? (
          <Alert data-testid="organization-downgrade-warning">
            <TriangleAlert />
            <AlertTitle>{t("workspaces.orgDowngradeWarningTitle")}</AlertTitle>
            <AlertDescription>
              {t("workspaces.orgDowngradeWarning")}
            </AlertDescription>
          </Alert>
        ) : null}

        {error ? (
          <Alert variant="destructive">
            <AlertTitle>{t("workspaces.changePlanErrorTitle")}</AlertTitle>
            <AlertDescription>
              <p>{error}</p>
              {list ? (
                <ul className="list-disc space-y-1 pl-4">
                  {preconditions.map((precondition) => (
                    <li key={precondition.kind}>
                      {preconditionLine(precondition, t, list)}
                    </li>
                  ))}
                </ul>
              ) : null}
            </AlertDescription>
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
