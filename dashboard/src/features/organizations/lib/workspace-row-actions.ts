import type {
  OrganizationRole,
  OrganizationWorkspace,
} from "@/features/organizations/types";

/** A workspace row's actions, in the order the row shows them. */
export type WorkspaceRowAction = "open" | "join" | "joinAsAdmin";

export const WORKSPACE_ROW_ACTION_KEYS: Record<WorkspaceRowAction, string> = {
  open: "organizations.open",
  join: "organizations.join",
  joinAsAdmin: "organizations.joinAsAdmin",
};

/**
 * What the caller can do with one org workspace (ADR115 D3, Render's
 * `JoinTeamEvent`): open one they are in; otherwise a Member or an Owner may
 * join it when it is Public, and an Owner may join any as admin. A Guest is in
 * their one workspace and joins none.
 */
export function workspaceRowActions(
  role: OrganizationRole | null,
  workspace: Pick<OrganizationWorkspace, "privacy" | "viewerIsMember">,
): WorkspaceRowAction[] {
  if (workspace.viewerIsMember) return ["open"];
  const actions: WorkspaceRowAction[] = [];
  if (
    workspace.privacy === "public" &&
    (role === "MEMBER" || role === "OWNER")
  ) {
    actions.push("join");
  }
  if (role === "OWNER") actions.push("joinAsAdmin");
  return actions;
}
