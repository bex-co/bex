import type { ReactNode } from "react";
import { useParams } from "@tanstack/react-router";
import { Plus, UserPlus } from "lucide-react";
import { PageFrame } from "@/common/components/page-frame";
import { Region } from "@/common/components/route-skeletons";
import { Badge } from "@/common/components/ui/badge";
import { Card, CardContent } from "@/common/components/ui/card";
import { Label } from "@/common/components/ui/label";
import { Skeleton, SkeletonSpan } from "@/common/components/ui/skeleton";
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
  ReservedButton,
  ReservedText,
} from "@/common/components/reserved-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  DEFAULT_WORKSPACE_PRIVACY,
  WORKSPACE_PRIVACY_KEYS,
} from "@/features/workspaces/types";
import { AuditLogExportFrame } from "@/features/audit/components/audit-log-panel-skeleton";
import { useOrganizations } from "@/features/organizations/hooks/use-organizations";
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
  OrganizationNameCardHeader,
} from "@/features/organizations/components/organization-settings-frame";
import {
  MEMBER_COLUMN_KEYS,
  WORKSPACE_COLUMN_KEYS,
  toOrganizationRole,
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
 * the switcher's `organizations` list, which the chrome has already loaded;
 * with no entry for the org, the frame draws only what every role sees.
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

function useCachedOrganization(organizationId: string): {
  role: OrganizationRole | null;
  workspaces: readonly SkeletonWorkspace[];
} {
  const { organizations } = useOrganizations();
  const organization = organizations.find((org) => org.id === organizationId);
  return {
    role: toOrganizationRole(organization?.viewerRole),
    workspaces: organization?.workspaces.length
      ? organization.workspaces
      : DEFAULT_WORKSPACES,
  };
}

/** A Badge's exact box, as a bar. */
function ReservedBadge({
  variant,
  children,
}: {
  variant: "secondary" | "outline";
  children: ReactNode;
}) {
  return (
    <span className="relative inline-flex shrink-0">
      <Badge variant={variant} className="invisible">
        {children}
      </Badge>
      <SkeletonSpan />
    </span>
  );
}

/** The Workspaces card while the org loads: its rows' boxes and actions. */
export function OrganizationWorkspacesCardSkeleton({
  organizationId,
}: {
  organizationId: string;
}) {
  const { t } = useTranslations();
  const { role, workspaces } = useCachedOrganization(organizationId);
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
  const { role } = useCachedOrganization(organizationId);
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
 * the Owners-only note otherwise) and, for an Owner, Audit Logs with the
 * export row's frame.
 */
export function OrganizationSettingsSkeleton({
  organizationId,
}: {
  organizationId: string;
}) {
  const { t } = useTranslations();
  const { role } = useCachedOrganization(organizationId);
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
            {role && role !== "OWNER" ? (
              <p className="text-xs text-muted-foreground">
                {t("organizations.nameOwnerOnly")}
              </p>
            ) : null}
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

/** An org route's pending frame: the page's header, then `children`. */
function OrganizationRouteSkeleton({
  section,
  children,
}: {
  section: OrganizationSection;
  children: (organizationId: string) => ReactNode;
}) {
  const params: { orgId?: string } = useParams({ strict: false });
  const organizationId = params.orgId ?? "";
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
      {children(organizationId)}
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
