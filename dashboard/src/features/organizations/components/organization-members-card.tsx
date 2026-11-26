import { useState } from "react";
import { Loader2, MoreHorizontal } from "lucide-react";
import { Card, CardContent } from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import {
  Table,
  TableBody,
  TableCell,
  TableRow,
} from "@/common/components/ui/table";
import { PanelTableHeader } from "@/common/components/panel-states";
import { useTranslations } from "@/common/hooks/use-translations";
import { UserAvatar } from "@/features/avatars/components/avatar";
import {
  MemberIdentity,
  PendingInvitesTable,
  RoleSelect,
} from "@/features/team/components/member-controls";
import {
  useOrganizationMembers,
  type OrganizationMember,
} from "@/features/organizations/hooks/use-organization-members";
import { useOrganizationMemberActions } from "@/features/organizations/hooks/use-organization-member-actions";
import { InviteOrganizationMemberDialog } from "@/features/organizations/components/invite-organization-member-dialog";
import { OrganizationMemberDetailsDialog } from "@/features/organizations/components/organization-member-details-dialog";
import {
  MEMBER_ROW_CLASS,
  OrganizationMembersCardHeader,
  OrganizationMembersFailed,
  OrganizationMembersForbidden,
} from "@/features/organizations/components/organization-members-frame";
import { OrganizationMembersTableFrame } from "@/features/organizations/components/organization-route-skeletons";
import {
  MEMBER_COLUMN_KEYS,
  ORGANIZATION_ROLES,
  ORGANIZATION_ROLE_LABEL_KEYS,
  toOrganizationRole,
  type OrganizationRole,
  type OrganizationWorkspace,
} from "@/features/organizations/types";

function MemberRow({
  member,
  canManage,
  busy,
  workspaceName,
  onChangeRole,
  onViewDetails,
  onRemove,
}: {
  member: OrganizationMember;
  canManage: boolean;
  busy: boolean;
  workspaceName: (id: string) => string | undefined;
  onChangeRole: (role: OrganizationRole) => void;
  onViewDetails: () => void;
  onRemove: () => void;
}) {
  const { t } = useTranslations();
  const identity = member.email || member.userId || member.subject;
  const role = toOrganizationRole(member.role);
  // The caller's own role and membership are another Owner's to change
  // (CANNOT_CHANGE_OWN_ROLE, CANNOT_REMOVE_SELF), as on a workspace.
  const manageable = canManage && !member.isSelf;
  const guestWorkspace = member.workspaces[0];

  return (
    <TableRow className={MEMBER_ROW_CLASS}>
      <TableCell className="w-full max-w-0">
        <MemberIdentity
          identity={identity}
          isSelf={member.isSelf}
          avatar={
            <UserAvatar
              email={member.email}
              imageURL={member.avatarDownload?.url}
              size="small"
            />
          }
        />
      </TableCell>
      <TableCell>
        {manageable && role ? (
          <RoleSelect
            value={role}
            roles={ORGANIZATION_ROLES}
            label={(option) => t(ORGANIZATION_ROLE_LABEL_KEYS[option])}
            ariaLabel={t("organizations.roleFor", { identity })}
            disabled={busy}
            onChange={onChangeRole}
          />
        ) : (
          <span className="whitespace-nowrap text-sm">
            {role ? t(ORGANIZATION_ROLE_LABEL_KEYS[role]) : member.role}
          </span>
        )}
        {role === "GUEST" ? (
          <span className="mt-1 block text-xs text-muted-foreground">
            {guestWorkspace
              ? t("organizations.guestWorkspace", {
                  workspace: workspaceName(guestWorkspace) ?? guestWorkspace,
                })
              : t("organizations.guestNoWorkspace")}
          </span>
        ) : null}
      </TableCell>
      <TableCell className="text-right">
        <MemberRowMenu
          identity={identity}
          busy={busy}
          removable={manageable}
          onViewDetails={onViewDetails}
          onRemove={onRemove}
        />
      </TableCell>
    </TableRow>
  );
}

/**
 * Render's row menu: View details first, for every row, then Remove member
 * on anyone but the caller, for an Owner. The org role stays in its own
 * column's select, which also offers Guest.
 */
function MemberRowMenu({
  identity,
  busy,
  removable,
  onViewDetails,
  onRemove,
}: {
  identity: string;
  busy: boolean;
  removable: boolean;
  onViewDetails: () => void;
  onRemove: () => void;
}) {
  const { t } = useTranslations();
  const [confirming, setConfirming] = useState(false);
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            disabled={busy}
            aria-label={t("organizations.memberActions", { identity })}
          >
            {busy ? <Loader2 className="animate-spin" /> : <MoreHorizontal />}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem onSelect={onViewDetails}>
            {t("organizations.viewDetails")}
          </DropdownMenuItem>
          {removable ? (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                variant="destructive"
                onSelect={() => setConfirming(true)}
              >
                {t("organizations.removeMemberItem")}
              </DropdownMenuItem>
            </>
          ) : null}
        </DropdownMenuContent>
      </DropdownMenu>
      {removable ? (
        <ConfirmDialog
          open={confirming}
          onOpenChange={setConfirming}
          title={t("organizations.removeTitle")}
          description={t("organizations.removeConfirm", { identity })}
          cancelLabel={t("organizations.cancel")}
          confirmLabel={t("organizations.remove")}
          pending={busy}
          onConfirm={onRemove}
        />
      ) : null}
    </>
  );
}

/**
 * Org → Members (w7/m464): Owners, Members and Guests with their org role, a
 * Guest with the one workspace they belong to. An Owner invites (as Owner or
 * Member), changes anyone else's role to Owner, Member or Guest, removes people
 * from the org, and revokes pending invites. A Guest is refused the list.
 * Each row's menu opens the member's details (w1/m657): their workspaces and,
 * for an Owner, their login methods and 2FA.
 */
export function OrganizationMembersCard({
  organizationId,
  role,
  workspaceName,
  workspace,
}: {
  organizationId: string;
  role: OrganizationRole | null;
  workspaceName: (id: string) => string | undefined;
  workspace: (id: string) => OrganizationWorkspace | undefined;
}) {
  const { t } = useTranslations();
  const isOwner = role === "OWNER";
  const [details, setDetails] = useState<OrganizationMember | null>(null);
  const { members, invites, loading, forbidden, failed } =
    useOrganizationMembers(organizationId, isOwner);
  const { changeRole, remove, revokeInvite, busy } =
    useOrganizationMemberActions(organizationId, workspaceName);

  return (
    <Card>
      <OrganizationMembersCardHeader
        action={
          isOwner ? (
            <InviteOrganizationMemberDialog organizationId={organizationId} />
          ) : null
        }
      />
      <CardContent className="space-y-6">
        {forbidden ? (
          <OrganizationMembersForbidden />
        ) : failed ? (
          <OrganizationMembersFailed />
        ) : loading ? (
          <OrganizationMembersTableFrame />
        ) : (
          <>
            <Table>
              <PanelTableHeader
                columns={MEMBER_COLUMN_KEYS.map((key) => t(key))}
              />
              <TableBody>
                {members.map((member) => {
                  const identity =
                    member.email || member.userId || member.subject;
                  return (
                    <MemberRow
                      key={member.subject}
                      member={member}
                      canManage={isOwner}
                      busy={busy === member.subject}
                      workspaceName={workspaceName}
                      onChangeRole={(next) =>
                        void changeRole(member.subject, identity, next)
                      }
                      onViewDetails={() => setDetails(member)}
                      onRemove={() => void remove(member.subject, identity)}
                    />
                  );
                })}
              </TableBody>
            </Table>
            {isOwner && invites.length > 0 ? (
              <PendingInvitesTable
                title={t("organizations.pendingTitle")}
                columns={[
                  t("organizations.colEmail"),
                  t("organizations.colRole"),
                ]}
                invites={invites}
                roleLabel={(invite) => {
                  const inviteRole = toOrganizationRole(invite.role);
                  return inviteRole
                    ? t(ORGANIZATION_ROLE_LABEL_KEYS[inviteRole])
                    : invite.role;
                }}
                // A pending invite's Revoke is described by the invite's email.
                actions={(invite, emailId) => (
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={busy === invite.id}
                    aria-describedby={emailId}
                    onClick={() => void revokeInvite(invite.id, invite.email)}
                  >
                    {busy === invite.id ? (
                      <Loader2 className="animate-spin" />
                    ) : null}
                    {t("organizations.revokeInvite")}
                  </Button>
                )}
              />
            ) : null}
          </>
        )}
      </CardContent>
      {details ? (
        <OrganizationMemberDetailsDialog
          organizationId={organizationId}
          member={details}
          canViewAuthStatus={isOwner}
          workspace={workspace}
          onClose={() => setDetails(null)}
        />
      ) : null}
    </Card>
  );
}
