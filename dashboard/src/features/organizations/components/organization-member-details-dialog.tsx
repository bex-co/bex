import { Lock } from "lucide-react";
import { Badge } from "@/common/components/ui/badge";
import { ReservedBadge } from "@/common/components/reserved-skeleton";
import { Button } from "@/common/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  UserAvatar,
  WorkspaceAvatar,
} from "@/features/avatars/components/avatar";
import type { OrganizationMember } from "@/features/organizations/hooks/use-organization-members";
import { useOrgUserAuthStatus } from "@/features/organizations/hooks/use-org-user-auth-status";
import {
  LOGIN_METHODS,
  loginMethodLabel,
} from "@/features/organizations/lib/login-methods";
import { ROLES, type Role } from "@/features/team/types";
import { WORKSPACE_PRIVACY_KEYS } from "@/features/workspaces/types";
import {
  toOrganizationRole,
  type OrganizationWorkspace,
} from "@/features/organizations/types";

const isRole = (role: string): role is Role =>
  (ROLES as readonly string[]).includes(role);

const STATUS_ROW =
  "grid grid-cols-[1fr_auto] items-center gap-2 border-x border-t px-4 py-3 last:border-b";

/**
 * Org Members → a member's View details (w1/m657), Render's dialog: the
 * member with "Owner" under an Org Owner's name, the workspaces they belong
 * to with their role in each, and, for an Owner (Render's
 * viewOrgUserAuthStatus), Login & Authentication Status: each login method
 * Signed in or Not signed in, then Two-factor Authentication. Anyone else
 * sees no status section and asks bex-api nothing.
 */
export function OrganizationMemberDetailsDialog({
  organizationId,
  member,
  canViewAuthStatus,
  workspace,
  onClose,
}: {
  organizationId: string;
  member: OrganizationMember;
  canViewAuthStatus: boolean;
  workspace: (id: string) => OrganizationWorkspace | undefined;
  onClose: () => void;
}) {
  const { t } = useTranslations();
  const identity = member.email || member.userId || member.subject;
  const isOwner = toOrganizationRole(member.role) === "OWNER";
  const { status, loading } = useOrgUserAuthStatus(
    organizationId,
    member.userId || member.subject,
    canViewAuthStatus,
  );

  return (
    <Dialog open onOpenChange={(open) => (open ? undefined : onClose())}>
      <DialogContent className="grid-cols-[minmax(0,1fr)]">
        <DialogHeader>
          <DialogTitle className="grid grid-cols-[auto_minmax(0,1fr)] items-center gap-x-3 gap-y-1 text-left">
            <UserAvatar
              email={member.email}
              imageURL={member.avatarDownload?.url}
              size="large"
              className={isOwner ? "row-span-2" : undefined}
            />
            <span className="truncate" title={identity}>
              {identity}
            </span>
            {isOwner ? (
              <span className="text-muted-foreground col-start-2 text-sm font-normal">
                {t("organizations.detailsOwner")}
              </span>
            ) : null}
          </DialogTitle>
          <DialogDescription className="sr-only">
            {t("organizations.viewDetails")}
          </DialogDescription>
        </DialogHeader>
        <div className="min-h-0 space-y-6 overflow-y-auto">
          <section className="space-y-2">
            <h2 className="text-sm font-semibold">
              {t("organizations.detailsWorkspaces")}
            </h2>
            <MemberWorkspaces member={member} workspace={workspace} />
          </section>
          {loading || status ? (
            <section className="space-y-2">
              <h2 className="text-sm font-semibold">
                {t("organizations.detailsLoginStatus")}
              </h2>
              {status ? (
                <ul className="text-sm">
                  {status.loginMethodStatuses.map((entry) => (
                    <li key={entry.method} className={STATUS_ROW}>
                      <span>
                        {loginMethodLabel(
                          entry.method,
                          t("organizations.loginMethodPassword"),
                        )}
                      </span>
                      <StatusBadge
                        ok={entry.signedIn}
                        yes={t("organizations.loginSignedIn")}
                        no={t("organizations.loginNotSignedIn")}
                      />
                    </li>
                  ))}
                  <li className={STATUS_ROW}>
                    <span>{t("organizations.loginTwoFactor")}</span>
                    <StatusBadge
                      ok={status.is2FAEnabled}
                      yes={t("organizations.loginTwoFactorEnabled")}
                      no={t("organizations.loginTwoFactorNotEnabled")}
                    />
                  </li>
                </ul>
              ) : (
                <LoginStatusFrame />
              )}
            </section>
          ) : null}
        </div>
        <DialogFooter>
          <DialogClose asChild>
            <Button>{t("organizations.detailsClose")}</Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/**
 * The status rows while orgUserAuthStatus is read (w1/m657 t007): each row's
 * label as itself and its badge's box, so the dialog keeps its height when the
 * answer lands. The methods are the ones bex-api lists, in its order
 * (`members.OrgUserAuthStatus`).
 */
function LoginStatusFrame() {
  const { t } = useTranslations();
  const password = t("organizations.loginMethodPassword");
  return (
    <>
      <span role="status" className="sr-only">
        {t("organizations.detailsLoginStatusLoading")}
      </span>
      <ul className="text-sm" aria-hidden="true">
        {LOGIN_METHODS.map((method) => (
          <li key={method} className={STATUS_ROW}>
            <span>{loginMethodLabel(method, password)}</span>
            <ReservedBadge variant="outline">
              {t("organizations.loginNotSignedIn")}
            </ReservedBadge>
          </li>
        ))}
        <li className={STATUS_ROW}>
          <span>{t("organizations.loginTwoFactor")}</span>
          <ReservedBadge variant="outline">
            {t("organizations.loginTwoFactorNotEnabled")}
          </ReservedBadge>
        </li>
      </ul>
    </>
  );
}

/** Render's success or warning badge: the dashboard's success badge, or an
 *  amber outline (there is no warning variant). */
function StatusBadge({
  ok,
  yes,
  no,
}: {
  ok: boolean;
  yes: string;
  no: string;
}) {
  return ok ? (
    <Badge variant="success">{yes}</Badge>
  ) : (
    <Badge
      variant="outline"
      className="border-amber-600/40 text-amber-700 dark:text-amber-400"
    >
      {no}
    </Badge>
  );
}

/** The member's org workspaces, each with its privacy and their role. */
function MemberWorkspaces({
  member,
  workspace,
}: {
  member: OrganizationMember;
  workspace: (id: string) => OrganizationWorkspace | undefined;
}) {
  const { t } = useTranslations();
  if (member.workspaceRoles.length === 0) {
    return (
      <div className="rounded-lg border p-4 text-center text-sm">
        <p className="font-semibold">
          {t("organizations.detailsNoWorkspaces")}
        </p>
        <p>{t("organizations.detailsNoWorkspacesBody")}</p>
      </div>
    );
  }
  return (
    <ul className="max-h-50 divide-y overflow-y-auto rounded-lg border">
      {member.workspaceRoles.map(({ workspaceId, role }) => {
        const w = workspace(workspaceId);
        const name = w?.name ?? workspaceId;
        return (
          <li
            key={workspaceId}
            className="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-2 px-3 py-2 text-sm"
          >
            <WorkspaceAvatar
              id={workspaceId}
              name={name}
              imageURL={w?.avatarDownload?.url}
              size="small"
            />
            <span className="flex min-w-0 items-center gap-1">
              <span className="truncate" title={name}>
                {name}
              </span>
              {w?.privacy === "invite_only" ? (
                <Lock
                  role="img"
                  className="text-muted-foreground size-3.5 shrink-0"
                  aria-label={t(WORKSPACE_PRIVACY_KEYS.invite_only.label)}
                />
              ) : null}
            </span>
            <span className="text-muted-foreground">
              {isRole(role) ? t(`team.role.${role}`) : role}
            </span>
          </li>
        );
      })}
    </ul>
  );
}
