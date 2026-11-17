import type { WorkspaceView } from "@/features/workspaces/types";

export interface SwitcherOrganization {
  id: string;
  name: string;
  /** The caller's own workspaces in the org, the primary first. */
  workspaces: WorkspaceView[];
}

export interface SwitcherGroups {
  organizations: SwitcherOrganization[];
  /** Workspaces in no organization, in the list's own order. */
  standalone: WorkspaceView[];
}

/**
 * The workspace dropdown's sections (w7/m464, Render: "open the workspace
 * dropdown at the top of the left pane and select your org"): each org the
 * caller is in, with the workspaces they belong to under it, then the
 * standalone workspaces.
 *
 * `organizations` is bex-api's `organizations` list, which names an org even
 * where the caller is in none of its workspaces (an Owner is not added to any
 * by being one). A workspace whose org that list does not carry (still loading,
 * or failed) is grouped under its own `organization` reference. A Guest's lists
 * hold only their one workspace, so that is all their org shows.
 */
export function groupSwitcherWorkspaces(
  workspaces: readonly WorkspaceView[],
  organizations: readonly { id: string; name: string }[],
): SwitcherGroups {
  const groups = new Map<string, SwitcherOrganization>();
  for (const org of organizations) {
    groups.set(org.id, { id: org.id, name: org.name, workspaces: [] });
  }
  const standalone: WorkspaceView[] = [];
  for (const workspace of workspaces) {
    const ref = workspace.organization;
    if (!ref) {
      standalone.push(workspace);
      continue;
    }
    let group = groups.get(ref.id);
    if (!group) {
      group = { id: ref.id, name: ref.name, workspaces: [] };
      groups.set(ref.id, group);
    }
    group.workspaces.push(workspace);
  }
  for (const group of groups.values()) {
    // Stable: the primary moves up, the rest keep the list's order.
    group.workspaces.sort((a, b) => Number(b.primary) - Number(a.primary));
  }
  return { organizations: [...groups.values()], standalone };
}
