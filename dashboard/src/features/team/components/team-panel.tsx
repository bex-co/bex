import { useMemo, useState } from "react";
import { AlertTriangle, Loader2, Search, Users } from "lucide-react";
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
import { Input } from "@/common/components/ui/input";
import {
  PanelCenteredState,
  PanelTableSkeleton,
  TableActionsHead,
} from "@/common/components/panel-states";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useTeam } from "@/features/team/hooks/use-team";
import { useChangeRole } from "@/features/team/hooks/use-change-role";
import { useRemoveMember } from "@/features/team/hooks/use-remove-member";
import { useResendInvite } from "@/features/team/hooks/use-resend-invite";
import { MemberRow } from "@/features/team/components/member-row";
import type { Role } from "@/features/team/types";
import { InviteMemberDialog } from "@/features/team/components/invite-member-dialog";

/**
 * Settings → Team (w4/m12): the workspace's members and their roles, plus (for
 * an admin) invite / role-change / remove and the pending-invite list. The
 * docs/ADR012-auth.md role matrix is enforced server-side; this surface reflects it —
 * a non-admin sees a read-only list (the invite/role/remove controls hide when
 * the admin-only invites query is forbidden).
 *
 * The workspace managed here is the *switcher's* selection (WorkspaceProvider,
 * w6/m14) — the same fix the audit log got (use-audit-log.ts). It used to come
 * from `useCurrentWorkspace()`, which ran its own `workspaces` query and always
 * returned `workspaces[0]` (the account's original auto-provisioned workspace),
 * so after a switch this page kept managing the wrong workspace's members.
 */
export function TeamPanel() {
  const { t } = useTranslations();
  const { currentWorkspaceId: workspaceId } = useWorkspace();
  const { members, invites, seats, loading, error, canManage, refetch } =
    useTeam(workspaceId);
  const { changeRole, changing } = useChangeRole(workspaceId ?? "");
  const { removeMember, revokeInvite, removing } = useRemoveMember(
    workspaceId ?? "",
  );
  const { resendInvite, resending } = useResendInvite(workspaceId ?? "");
  // Seats full on a LIMITED plan disables the invite entry point up front —
  // the same wall the server's cap enforces, seen before the click instead of
  // as the dialog's refusal (which stays as the recovery path).
  const seatsFull =
    seats != null && seats.limit > 0 && seats.used >= seats.limit;
  const [search, setSearch] = useState("");
  const normalizedSearch = search.trim().toLocaleLowerCase();
  const filteredMembers = useMemo(
    () =>
      normalizedSearch
        ? members.filter((member) =>
            [member.email, member.userId, member.subject].some((identity) =>
              identity.toLocaleLowerCase().includes(normalizedSearch),
            ),
          )
        : members,
    [members, normalizedSearch],
  );

  const initialLoading = loading && members.length === 0;

  async function handleChangeRole(subject: string, role: Role) {
    if (await changeRole(subject, role)) await refetch();
  }
  async function handleRemove(subject: string) {
    if (await removeMember(subject)) await refetch();
  }
  async function handleRevoke(inviteId: string) {
    if (await revokeInvite(inviteId)) await refetch();
  }
  // No refetch: the resend mutation returns the whole WorkspaceInvite keyed by
  // id, so the refreshed expiry merges into the row the list already renders.
  const handleResend = resendInvite;

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          {t("team.title")}
          {seats ? (
            <span className="text-sm font-normal text-muted-foreground tabular-nums">
              {seats.limit > 0
                ? t("team.seatUsage", {
                    used: seats.used,
                    limit: seats.limit,
                  })
                : t("team.seatCount", { used: seats.used })}
            </span>
          ) : null}
        </CardTitle>
        <CardDescription>{t("team.description")}</CardDescription>
        {canManage && workspaceId ? (
          <CardAction>
            <InviteMemberDialog
              workspaceId={workspaceId}
              seatsFull={seatsFull}
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
            <div className="relative max-w-sm">
              <Search className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                type="search"
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                aria-label={t("team.searchLabel")}
                placeholder={t("team.searchPlaceholder")}
                className="pl-8"
              />
            </div>

            {members.length === 0 ? (
              <PanelCenteredState
                icon={<Users />}
                title={t("team.emptyTitle")}
                body={t("team.emptyBody")}
              />
            ) : filteredMembers.length === 0 ? (
              <div className="py-8 text-center" role="status">
                <p className="font-medium">{t("team.noMatchesTitle")}</p>
                <p className="mt-1 text-sm text-muted-foreground">
                  {t("team.noMatchesBody")}
                </p>
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("team.colMember")}</TableHead>
                    <TableHead>{t("team.colRole")}</TableHead>
                    <TableActionsHead />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filteredMembers.map((member) => (
                    <MemberRow
                      key={member.subject}
                      member={member}
                      canManage={canManage}
                      changing={changing === member.subject}
                      removing={removing === member.subject}
                      onChangeRole={(subject, role) =>
                        void handleChangeRole(subject, role)
                      }
                      onRemove={(subject) => void handleRemove(subject)}
                    />
                  ))}
                </TableBody>
              </Table>
            )}

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
                      <TableActionsHead />
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {invites.map((invite) => (
                      <TableRow key={invite.id}>
                        <TableCell className="break-all">
                          {invite.email}
                        </TableCell>
                        <TableCell>
                          <Badge variant="secondary">
                            {t(`team.role.${invite.role}`)}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-right">
                          <Button
                            variant="ghost"
                            size="sm"
                            disabled={resending === invite.id}
                            onClick={() => void handleResend(invite.id)}
                          >
                            {resending === invite.id ? (
                              <Loader2 className="animate-spin" />
                            ) : null}
                            {t("team.resendInvite")}
                          </Button>
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
