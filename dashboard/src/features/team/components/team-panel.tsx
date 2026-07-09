import { AlertTriangle, Loader2 } from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardAction,
  CardContent,
} from "@/common/components/ui/card";
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from "@/common/components/ui/table";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import {
  PanelCenteredState,
  PanelTableSkeleton,
} from "@/common/components/panel-states";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCurrentWorkspace } from "@/features/team/hooks/use-current-workspace";
import { useTeam } from "@/features/team/hooks/use-team";
import { useChangeRole } from "@/features/team/hooks/use-change-role";
import { useRemoveMember } from "@/features/team/hooks/use-remove-member";
import { MemberRow } from "@/features/team/components/member-row";
import { InviteMemberDialog } from "@/features/team/components/invite-member-dialog";

/**
 * Settings → Team (w4/m12): the workspace's members and their roles, plus (for
 * an admin) invite / role-change / remove and the pending-invite list. The
 * docs/auth.md role matrix is enforced server-side; this surface reflects it —
 * a non-admin sees a read-only list (the invite/role/remove controls hide when
 * the admin-only invites query is forbidden).
 */
export function TeamPanel() {
  const { t } = useTranslations();
  const { workspace, loading: workspaceLoading } = useCurrentWorkspace();
  const workspaceId = workspace?.id ?? null;
  const { members, invites, loading, error, canManage, refetch } = useTeam(workspaceId);
  const { changeRole, changing } = useChangeRole(workspaceId ?? "");
  const { removeMember, revokeInvite, removing } = useRemoveMember(workspaceId ?? "");

  const initialLoading =
    (workspaceLoading || loading) && members.length === 0 && !error;

  async function handleChangeRole(subject: string, role: Parameters<typeof changeRole>[1]) {
    if (await changeRole(subject, role)) await refetch();
  }
  async function handleRemove(subject: string) {
    if (await removeMember(subject)) await refetch();
  }
  async function handleRevoke(inviteId: string) {
    if (await revokeInvite(inviteId)) await refetch();
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("team.title")}</CardTitle>
        <CardDescription>{t("team.description")}</CardDescription>
        {canManage && workspaceId ? (
          <CardAction>
            <InviteMemberDialog
              workspaceId={workspaceId}
              onInvited={() => void refetch()}
            />
          </CardAction>
        ) : null}
      </CardHeader>
      <CardContent className="space-y-6">
        {error ? (
          <PanelCenteredState
            icon={<AlertTriangle />}
            title={t("team.errorTitle")}
            body={t("team.errorBody")}
          />
        ) : initialLoading ? (
          <PanelTableSkeleton />
        ) : (
          <>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("team.colMember")}</TableHead>
                  <TableHead>{t("team.colRole")}</TableHead>
                  <TableHead className="sr-only text-right">actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {members.map((member) => (
                  <MemberRow
                    key={member.subject}
                    member={member}
                    canManage={canManage}
                    changing={changing === member.subject}
                    removing={removing === member.subject}
                    onChangeRole={(subject, role) => void handleChangeRole(subject, role)}
                    onRemove={(subject) => void handleRemove(subject)}
                  />
                ))}
              </TableBody>
            </Table>

            {canManage && invites.length > 0 ? (
              <div className="space-y-2">
                <h3 className="text-sm font-medium text-muted-foreground">
                  {t("team.pendingTitle")}
                </h3>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("team.colEmail")}</TableHead>
                      <TableHead>{t("team.colRole")}</TableHead>
                      <TableHead className="sr-only text-right">actions</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {invites.map((invite) => (
                      <TableRow key={invite.id}>
                        <TableCell className="break-all">{invite.email}</TableCell>
                        <TableCell>
                          <Badge variant="secondary">{t(`team.role.${invite.role}`)}</Badge>
                        </TableCell>
                        <TableCell className="text-right">
                          <Button
                            variant="ghost"
                            size="sm"
                            disabled={removing === invite.id}
                            onClick={() => void handleRevoke(invite.id)}
                          >
                            {removing === invite.id ? (
                              <Loader2 className="animate-spin" />
                            ) : null}
                            {t("team.revokeInvite")}
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            ) : null}
          </>
        )}
      </CardContent>
    </Card>
  );
}

