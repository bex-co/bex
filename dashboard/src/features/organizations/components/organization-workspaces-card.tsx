import { useId } from "react";
import { WorkspaceAvatar } from "@/features/avatars/components/avatar";
import { useNavigate } from "@tanstack/react-router";
import { Boxes, Globe, Loader2, Lock } from "lucide-react";
import { Card, CardContent } from "@/common/components/ui/card";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableRow,
} from "@/common/components/ui/table";
import {
  PanelCenteredState,
  PanelTableHeader,
} from "@/common/components/panel-states";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  WORKSPACE_PRIVACY_KEYS,
  type WorkspacePrivacy,
} from "@/features/workspaces/types";
import { useJoinOrganizationWorkspace } from "@/features/organizations/hooks/use-organization-workspace-actions";
import { CreateOrganizationWorkspaceDialog } from "@/features/organizations/components/create-organization-workspace-dialog";
import {
  OrganizationWorkspacesCardHeader,
  WORKSPACE_ROW_CLASS,
} from "@/features/organizations/components/organization-workspaces-frame";
import {
  WORKSPACE_ROW_ACTION_KEYS,
  workspaceRowActions,
  type WorkspaceRowAction,
} from "@/features/organizations/lib/workspace-row-actions";
import {
  WORKSPACE_COLUMN_KEYS,
  type Organization,
  type OrganizationRole,
  type OrganizationWorkspace,
} from "@/features/organizations/types";

/** Public or Invite-only, with the icon the create dialog's choice implies. */
function PrivacyLabel({ privacy }: { privacy: WorkspacePrivacy }) {
  const { t } = useTranslations();
  const Icon = privacy === "public" ? Globe : Lock;
  return (
    <span className="inline-flex items-center gap-1.5 whitespace-nowrap text-sm">
      <Icon aria-hidden className="size-4 text-muted-foreground" />
      {t(WORKSPACE_PRIVACY_KEYS[privacy].label)}
    </span>
  );
}

function WorkspaceRow({
  workspace,
  role,
  joining,
  onAction,
}: {
  workspace: OrganizationWorkspace;
  role: OrganizationRole | null;
  joining: boolean;
  onAction: (action: WorkspaceRowAction) => void;
}) {
  const { t } = useTranslations();
  // Each row's buttons are described by its workspace's name, so "Join"
  // says which workspace it joins.
  const nameId = useId();
  return (
    <TableRow className={WORKSPACE_ROW_CLASS}>
      <TableCell className="w-full max-w-0">
        <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
          {/* The workspace's avatar, as Render's org workspaces list (w1/m648). */}
          <WorkspaceAvatar
            id={workspace.id}
            name={workspace.name}
            imageURL={workspace.avatarDownload?.url}
            size="small"
          />
          <span
            id={nameId}
            className="min-w-0 max-w-full break-words font-medium"
          >
            {workspace.name}
          </span>
          {workspace.primary ? (
            <Badge variant="outline" className="shrink-0">
              {t("organizations.primary")}
            </Badge>
          ) : null}
        </div>
      </TableCell>
      <TableCell>
        <PrivacyLabel privacy={workspace.privacy} />
      </TableCell>
      <TableCell className="text-right whitespace-nowrap">
        <div className="inline-flex gap-2">
          {workspaceRowActions(role, workspace).map((action) => (
            <Button
              key={action}
              variant="outline"
              size="sm"
              disabled={joining}
              aria-describedby={nameId}
              onClick={() => onAction(action)}
            >
              {joining && action !== "open" ? (
                <Loader2 className="animate-spin" />
              ) : null}
              {t(WORKSPACE_ROW_ACTION_KEYS[action])}
            </Button>
          ))}
        </div>
      </TableCell>
    </TableRow>
  );
}

/**
 * Org → Workspaces (w7/m464): every workspace of the org the caller may see
 * (a Guest sees their one), each with its privacy and the Primary label, and
 * the caller's way in: Open, Join (Public), or Join as admin (Owners). An
 * Owner creates workspaces from the card's header.
 */
export function OrganizationWorkspacesCard({
  organization,
  role,
}: {
  organization: Organization;
  role: OrganizationRole | null;
}) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { setCurrentWorkspaceId } = useWorkspace();
  const { join, joinAsAdmin, joining } = useJoinOrganizationWorkspace();

  function act(workspace: OrganizationWorkspace, action: WorkspaceRowAction) {
    if (action === "open") {
      setCurrentWorkspaceId(workspace.id);
      void navigate({ to: "/" });
    } else if (action === "join") {
      void join(workspace.id, workspace.name);
    } else {
      void joinAsAdmin(workspace.id, workspace.name);
    }
  }

  return (
    <Card>
      <OrganizationWorkspacesCardHeader
        action={
          role === "OWNER" ? (
            <CreateOrganizationWorkspaceDialog
              organizationId={organization.id}
              organizationName={organization.name}
              plan={organization.plan}
            />
          ) : null
        }
      />
      <CardContent>
        {organization.workspaces.length === 0 ? (
          <PanelCenteredState
            icon={<Boxes />}
            title={t("organizations.workspacesEmptyTitle")}
            body={t("organizations.workspacesEmptyBody")}
          />
        ) : (
          <Table>
            <PanelTableHeader
              columns={WORKSPACE_COLUMN_KEYS.map((key) => t(key))}
            />
            <TableBody>
              {organization.workspaces.map((workspace) => (
                <WorkspaceRow
                  key={workspace.id}
                  workspace={workspace}
                  role={role}
                  joining={joining === workspace.id}
                  onAction={(action) => act(workspace, action)}
                />
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}
