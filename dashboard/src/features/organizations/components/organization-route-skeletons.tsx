import type { ReactNode } from "react";
import { useParams } from "@tanstack/react-router";
import { Plus, UserPlus } from "lucide-react";
import { PageFrame } from "@/common/components/page-frame";
import { Region } from "@/common/components/route-skeletons";
import { Card, CardContent } from "@/common/components/ui/card";
import { Label } from "@/common/components/ui/label";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableRow,
} from "@/common/components/ui/table";
import {
  PanelTableFrame,
  PanelTableHeader,
} from "@/common/components/panel-states";
import {
  ReservedBadge,
  ReservedButton,
  ReservedText,
} from "@/common/components/reserved-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  DEFAULT_WORKSPACE_PRIVACY,
  WORKSPACE_PRIVACY_KEYS,
} from "@/features/workspaces/types";
import { AuditLogExportFrame } from "@/features/audit/components/audit-log-panel-skeleton";
import { useCachedOrganization } from "@/features/organizations/hooks/use-cached-organization";
import { OrganizationPageHeader } from "@/features/organizations/components/organization-page-header";
import {
  OrganizationWorkspacesCardHeader,
  WORKSPACE_ROW_CLASS,
} from "@/features/organizations/components/organization-workspaces-frame";
import {
  WORKSPACE_ROW_ACTION_KEYS,
  workspaceRowActions,
} from "@/features/organizations/lib/workspace-row-actions";
import {
  MEMBER_ROW_CLASS,
  OrganizationMembersCardHeader,
  OrganizationMembersForbidden,
} from "@/features/organizations/components/organization-members-frame";
import {
  NAME_ROW_CLASS,
  OrganizationAuditLogsCardHeader,
  OrganizationLoginRequirementSection,
  OrganizationNameCardHeader,
  OrganizationSecurityCardHeader,
  OrganizationSecurityOwnerOnly,
} from "@/features/organizations/components/organization-settings-frame";
import { OrganizationLoginBlocked } from "@/features/organizations/components/organization-login-blocked";
import { useOrganizationLoginBlock } from "@/features/workspaces/hooks/use-workspace-login-block";
import {
  LOGIN_REQUIREMENTS,
  LOGIN_REQUIREMENT_SETTINGS,
} from "@/features/workspaces/lib/login-requirements";
import {
  MEMBER_COLUMN_KEYS,
  WORKSPACE_COLUMN_KEYS,
  toOrganizationRole,
  type Organization,
  type OrganizationRole,
  type OrganizationSection,
  type OrganizationWorkspace,
} from "@/features/organizations/types";

/**
 * The org pages' pending frames (w7/m464), kept out of the pages' modules so
 * the route tree loads them up front (dashboard/AGENTS.md, w3/066). Each is
 * the ready page's own frame: the header with the name and role as bars, the
 * static navigation and card headers, and the ready rows' boxes. What depends
 * on the caller's role (create, invite, rename, the audit log) is read from
 * the cache: the org's own entry, else the caller's `organizations` list the
 * shell fetches at every width (w4/706); with neither, the frame draws only
 * what every role sees.
 */

const PLACEHOLDER_WORKSPACE = "workspace-name";
const PLACEHOLDER_EMAIL = "member@example.com";

type SkeletonWorkspace = Pick<
  OrganizationWorkspace,
  "primary" | "privacy" | "viewerIsMember"
>;

/** With nothing cached, the one row every org shows: its primary. */
const DEFAULT_WORKSPACES: SkeletonWorkspace[] = [
  { primary: true, privacy: DEFAULT_WORKSPACE_PRIVACY, viewerIsMember: true },
];

function useSkeletonOrganization(organizationId: string): {
  organization: Organization | undefined;
  role: OrganizationRole | null;
  workspaces: readonly SkeletonWorkspace[];
} {
  const organization = useCachedOrganization(organizationId);
  return {
    organization,
    role: toOrganizationRole(organization?.viewerRole),
    workspaces: organization?.workspaces.length
      ? organization.workspaces
      : DEFAULT_WORKSPACES,
  };
}

/** The Workspaces card while the org loads: its rows' boxes and actions. */
export function OrganizationWorkspacesCardSkeleton({
  organizationId,
}: {
  organizationId: string;
}) {
  const { t } = useTranslations();
  const { role, workspaces } = useSkeletonOrganization(organizationId);
  return (
    <Card aria-hidden="true" data-testid="organization-workspaces-skeleton">
      <OrganizationWorkspacesCardHeader
        action={
          role === "OWNER" ? (
            <ReservedButton variant="default" size="sm">
              <Plus />
              {t("organizations.newWorkspace")}
            </ReservedButton>
          ) : null
        }
      />
      <CardContent>
        <Table>
          <PanelTableHeader
            columns={WORKSPACE_COLUMN_KEYS.map((key) => t(key))}
          />
          <TableBody>
            {workspaces.map((workspace, index) => (
              <TableRow key={index} className={WORKSPACE_ROW_CLASS}>
                <TableCell className="w-full max-w-0">
                  <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                    <Skeleton className="size-5 shrink-0 rounded-full" />
                    <span className="relative min-w-0 max-w-full break-words font-medium">
                      <ReservedText>{PLACEHOLDER_WORKSPACE}</ReservedText>
                    </span>
                    {workspace.primary ? (
                      <ReservedBadge variant="outline">
                        {t("organizations.primary")}
                      </ReservedBadge>
                    ) : null}
                  </div>
                </TableCell>
                <TableCell>
                  <span className="relative inline-flex items-center gap-1.5 whitespace-nowrap text-sm">
                    <span className="invisible size-4" />
                    <ReservedText>
                      {t(WORKSPACE_PRIVACY_KEYS[workspace.privacy].label)}
                    </ReservedText>
                  </span>
                </TableCell>
                <TableCell className="text-right whitespace-nowrap">
                  <div className="inline-flex gap-2">
                    {workspaceRowActions(role, workspace).map((action) => (
                      <ReservedButton key={action} variant="outline" size="sm">
                        {t(WORKSPACE_ROW_ACTION_KEYS[action])}
                      </ReservedButton>
                    ))}
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}

/**
 * The member table while it loads: its header and the one row every org's
 * list holds for the caller, their own (the "You" badge, the role as text,
 * no controls, since nobody changes their own role or removes themselves).
 */
export function OrganizationMembersTableFrame() {
  const { t } = useTranslations();
  return (
    <PanelTableFrame columns={MEMBER_COLUMN_KEYS.map((key) => t(key))}>
      <TableRow className={MEMBER_ROW_CLASS}>
        <TableCell className="w-full max-w-0">
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <Skeleton className="size-5 shrink-0 rounded-full" />
            <span className="relative min-w-0 max-w-full break-words">
              <ReservedText>{PLACEHOLDER_EMAIL}</ReservedText>
            </span>
            <ReservedBadge variant="secondary">{t("team.you")}</ReservedBadge>
          </div>
        </TableCell>
        <TableCell>
          <span className="relative whitespace-nowrap text-sm">
            <ReservedText>{t("organizations.roleMember")}</ReservedText>
          </span>
        </TableCell>
        <TableCell className="text-right" />
      </TableRow>
    </PanelTableFrame>
  );
}

/** The Members card while the org loads, in the caller's role's shape. */
export function OrganizationMembersCardSkeleton({
  organizationId,
}: {
  organizationId: string;
}) {
  const { t } = useTranslations();
  const { role } = useSkeletonOrganization(organizationId);
  return (
    <Card aria-hidden="true" data-testid="organization-members-skeleton">
      <OrganizationMembersCardHeader
        action={
          role === "OWNER" ? (
            <ReservedButton variant="outline" size="sm">
              <UserPlus />
              {t("organizations.invite")}
            </ReservedButton>
          ) : null
        }
      />
      <CardContent className="space-y-6">
        {role === "GUEST" ? (
          <div inert>
            <OrganizationMembersForbidden />
          </div>
        ) : (
          <OrganizationMembersTableFrame />
        )}
      </CardContent>
    </Card>
  );
}

/**
 * The Settings cards while the org loads: the name card (Save for an Owner,
 * the Owners-only note otherwise), Security (w7/m466) with each setting's
 * label and description as themselves and the control's box, and, for an
 * Owner, Audit Logs with the export row's frame. The control is the switch's
 * box for an Owner or a role not yet known; for a Member or Guest it is the
 * state badge's ("Required" or "Not required", from the cached org), which
 * is wider than a switch and at phone width wraps under the label (w7/089).
 * A Member or Guest also gets the Owners-only note.
 */
export function OrganizationSettingsSkeleton({
  organizationId,
}: {
  organizationId: string;
}) {
  const { t } = useTranslations();
  const { organization, role } = useSkeletonOrganization(organizationId);
  const readOnly = role !== null && role !== "OWNER";
  return (
    <>
      <Region name="organization-name">
        <Card aria-hidden="true" data-testid="organization-name-skeleton">
          <OrganizationNameCardHeader />
          <CardContent className="space-y-2">
            <div className={NAME_ROW_CLASS}>
              <div className="min-w-0 flex-1 space-y-2">
                <Label>{t("organizations.fieldName")}</Label>
                <Skeleton className="h-9 w-full" />
              </div>
              {role === "OWNER" ? (
                <ReservedButton variant="default">
                  {t("organizations.nameSave")}
                </ReservedButton>
              ) : null}
            </div>
            {readOnly ? (
              <p className="text-xs text-muted-foreground">
                {t("organizations.nameOwnerOnly")}
              </p>
            ) : null}
          </CardContent>
        </Card>
      </Region>
      <Region name="organization-security">
        <Card aria-hidden="true" data-testid="organization-security-skeleton">
          <OrganizationSecurityCardHeader />
          <CardContent className="space-y-6">
            {LOGIN_REQUIREMENTS.map((requirement) => (
              <OrganizationLoginRequirementSection
                key={requirement}
                requirement={requirement}
                control={readOnly ? "pendingStatus" : "pending"}
                required={
                  organization?.[LOGIN_REQUIREMENT_SETTINGS[requirement].field]
                }
              />
            ))}
            {readOnly ? <OrganizationSecurityOwnerOnly /> : null}
          </CardContent>
        </Card>
      </Region>
      {role === "OWNER" ? (
        <Region name="audit-logs">
          <Card aria-hidden="true" data-testid="organization-audit-skeleton">
            <OrganizationAuditLogsCardHeader />
            <CardContent>
              <AuditLogExportFrame />
            </CardContent>
          </Card>
        </Region>
      ) : null}
    </>
  );
}

/**
 * An org route's pending frame: the page's header, then `children`; or, when
 * the org's login requirement is already known to refuse the caller
 * (w7/m466), the blocked card the page will show instead.
 */
function OrganizationRouteSkeleton({
  section,
  children,
}: {
  section: OrganizationSection;
  children: (organizationId: string) => ReactNode;
}) {
  const params: { orgId?: string } = useParams({ strict: false });
  const organizationId = params.orgId ?? "";
  const block = useOrganizationLoginBlock(organizationId);
  return (
    <PageFrame
      pending={`organization-${section}`}
      header={
        <OrganizationPageHeader
          organizationId={organizationId}
          section={section}
          pending
        />
      }
    >
      {block ? (
        <OrganizationLoginBlocked block={block} />
      ) : (
        children(organizationId)
      )}
    </PageFrame>
  );
}

export function OrganizationWorkspacesRouteSkeleton() {
  return (
    <OrganizationRouteSkeleton section="workspaces">
      {(organizationId) => (
        <Region name="workspaces-table">
          <OrganizationWorkspacesCardSkeleton organizationId={organizationId} />
        </Region>
      )}
    </OrganizationRouteSkeleton>
  );
}

export function OrganizationMembersRouteSkeleton() {
  return (
    <OrganizationRouteSkeleton section="members">
      {(organizationId) => (
        <Region name="members-table">
          <OrganizationMembersCardSkeleton organizationId={organizationId} />
        </Region>
      )}
    </OrganizationRouteSkeleton>
  );
}

export function OrganizationSettingsRouteSkeleton() {
  return (
    <OrganizationRouteSkeleton section="settings">
      {(organizationId) => (
        <OrganizationSettingsSkeleton organizationId={organizationId} />
      )}
    </OrganizationRouteSkeleton>
  );
}
