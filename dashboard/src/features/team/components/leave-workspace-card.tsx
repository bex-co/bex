import { Loader2, LogOut } from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import {
  Alert,
  AlertTitle,
  AlertDescription,
} from "@/common/components/ui/alert";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useTranslations } from "@/common/hooks/use-translations";
import { useTeam } from "@/features/team/hooks/use-team";
import { LeaveWorkspaceCardSkeleton } from "@/features/team/components/team-panel-skeleton";
import { useLeaveWorkspace } from "@/features/team/hooks/use-leave-workspace";
import { leaveEligibility } from "@/features/team/lib/leave-eligibility";
import type { WorkspaceView } from "@/features/workspaces/types";

export interface LeaveWorkspaceCardProps {
  workspace: WorkspaceView;
}

/**
 * Leave workspace (w5/m102) — the deliberate exit that w5/m101's
 * CANNOT_REMOVE_SELF refusal points at. It lives in the danger zone rather than
 * on the caller's own Team row, because leaving is a decision about your place
 * in the workspace, not a row-level edit; the Team row's disabled Remove
 * control names this action as the way out.
 *
 * Renders nothing when the caller has no membership here (nothing to leave).
 * For the last admin the button stays rendered but disabled with the reason —
 * the same server refusal, seen before the click —
 * and a genuine server refusal still renders inline, for the race where
 * somebody else's role changes while this page is open.
 *
 * On success `useLeaveWorkspace` lands the caller in a workspace they still
 * belong to (the DeleteWorkspaceCard ordering, for the same m13 reason).
 */
export function LeaveWorkspaceCard({ workspace }: LeaveWorkspaceCardProps) {
  const { t } = useTranslations();
  const { members, loading } = useTeam(workspace.id);
  const { leave, busy, error } = useLeaveWorkspace(workspace.id);

  const { isMember, blockedReason } = leaveEligibility(members);
  // Until the members arrive, hold the card's place rather than render nothing
  // and pop in (w9/082): almost every caller here is a member.
  if (loading && members.length === 0) {
    return <LeaveWorkspaceCardSkeleton workspaceName={workspace.name} />;
  }
  if (!isMember) return null;

  // No onClick: when the action is available this button is the confirm
  // dialog's trigger (the dialog's confirm runs the leave), and when it is
  // blocked it is disabled. Wiring both would leave and open the dialog.
  const leaveButton = (
    <Button
      variant="outline"
      className="text-destructive"
      disabled={busy || blockedReason !== null}
    >
      {busy ? <Loader2 className="animate-spin" /> : <LogOut />}
      {t("team.leaveAction")}
    </Button>
  );

  return (
    <Card className="border-destructive/50">
      <CardHeader>
        <CardTitle className="text-destructive">
          {t("team.leaveTitle")}
        </CardTitle>
        <CardDescription>
          {t("team.leaveDescription", { workspace: workspace.name })}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {error ? (
          <Alert variant="destructive">
            <AlertTitle>{t("team.leaveErrorTitle")}</AlertTitle>
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        ) : null}
        {blockedReason ? (
          <PermissionTooltip reason={t(blockedReason)}>
            {leaveButton}
          </PermissionTooltip>
        ) : (
          <ConfirmDialog
            trigger={leaveButton}
            title={t("team.leaveConfirmTitle")}
            description={t("team.leaveConfirm", { workspace: workspace.name })}
            cancelLabel={t("team.leaveCancel")}
            confirmLabel={t("team.leaveAction")}
            onConfirm={() => void leave()}
          />
        )}
      </CardContent>
    </Card>
  );
}
