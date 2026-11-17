import { useCallback } from "react";
import { useMutation } from "@apollo/client/react";
import {
  CreateOrganizationWorkspaceDocument,
  JoinWorkspaceAsAdminDocument,
  JoinWorkspaceDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import type { WorkspacePrivacy } from "@/features/workspaces/types";
import {
  useKeyedOrganizationAction,
  useOrganizationDialogAction,
} from "@/features/organizations/hooks/use-organization-action";

/**
 * What a create or a join changes: the caller's workspace list (the switcher
 * and every workspace-scoped page) and the org page, whose read also updates
 * the switcher's cached entry for the org. Named queries refetch only where
 * mounted.
 */
const ORGANIZATION_WORKSPACE_QUERIES = {
  refetchQueries: ["Workspaces", "Organization"],
  awaitRefetchQueries: true,
};

export interface CreateOrganizationWorkspaceResult {
  /** Resolves true once the workspace exists and the lists are fresh. */
  create: (name: string, privacy: WorkspacePrivacy) => Promise<boolean>;
  busy: boolean;
  /** The refusal to show in the dialog, in the user's words. */
  refusal: string | null;
  clearRefusal: () => void;
}

/**
 * An Owner creates a workspace in the org (ADR115 D3): a name and Public or
 * Invite-only, no plan and no payment step. The creator becomes its admin.
 */
export function useCreateOrganizationWorkspace(
  organizationId: string,
): CreateOrganizationWorkspaceResult {
  const { t } = useTranslations();
  const [mutate, { loading }] = useMutation(
    CreateOrganizationWorkspaceDocument,
    ORGANIZATION_WORKSPACE_QUERIES,
  );
  const { run, refusal, clearRefusal } = useOrganizationDialogAction();

  const create = useCallback(
    (name: string, privacy: WorkspacePrivacy) =>
      run(async () => {
        const { data } = await mutate({
          variables: { organizationId, name, privacy },
        });
        return t("organizations.workspaceCreated", {
          workspace: data?.createOrganizationWorkspace?.name ?? name,
        });
      }, t("organizations.createFailed")),
    [run, mutate, organizationId, t],
  );

  return { create, busy: loading, refusal, clearRefusal };
}

export interface JoinOrganizationWorkspaceResult {
  /** Join a Public workspace (Member or Owner; Developer role). */
  join: (workspaceId: string, workspaceName: string) => Promise<boolean>;
  /** An Owner joins any org workspace as its admin. */
  joinAsAdmin: (workspaceId: string, workspaceName: string) => Promise<boolean>;
  /** The workspace a join is in flight for, or null. */
  joining: string | null;
}

/**
 * Render's two self-joins (audit-logs, `JoinTeamEvent`): "Org Owners can add
 * themselves to any workspace as an admin. Other org members can add
 * themselves to public workspaces only (they receive the Developer role)."
 */
export function useJoinOrganizationWorkspace(): JoinOrganizationWorkspaceResult {
  const { t } = useTranslations();
  const [joinMutation] = useMutation(
    JoinWorkspaceDocument,
    ORGANIZATION_WORKSPACE_QUERIES,
  );
  const [joinAsAdminMutation] = useMutation(
    JoinWorkspaceAsAdminDocument,
    ORGANIZATION_WORKSPACE_QUERIES,
  );
  const { busy, run } = useKeyedOrganizationAction();

  const join = useCallback(
    (workspaceId: string, workspaceName: string) =>
      run(
        workspaceId,
        () => joinMutation({ variables: { workspaceId } }),
        t("organizations.joined", { workspace: workspaceName }),
        t("organizations.joinFailed", { workspace: workspaceName }),
      ),
    [run, joinMutation, t],
  );
  const joinAsAdmin = useCallback(
    (workspaceId: string, workspaceName: string) =>
      run(
        workspaceId,
        () => joinAsAdminMutation({ variables: { workspaceId } }),
        t("organizations.joinedAsAdmin", { workspace: workspaceName }),
        t("organizations.joinFailed", { workspace: workspaceName }),
      ),
    [run, joinAsAdminMutation, t],
  );

  return { join, joinAsAdmin, joining: busy };
}
