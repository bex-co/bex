import type { MobileWorkspacesQuery } from "@/generated-graphql";

export type MobileWorkspace = {
  id: string;
  name: string;
  plan: string | null;
  role: string | null;
};

const workspaceIdPattern = /^tea-[a-z0-9]+$/;

export function normalizeWorkspaces(
  rows: MobileWorkspacesQuery["workspaces"],
): MobileWorkspace[] {
  return (rows ?? []).flatMap((row) => {
    if (!row?.id || !workspaceIdPattern.test(row.id)) return [];
    return [
      {
        id: row.id,
        name: row.name?.trim() || row.id,
        plan: row.plan ?? null,
        role: row.role ?? null,
      },
    ];
  });
}

export function chooseWorkspace(
  workspaces: MobileWorkspace[],
  persistedId: string | null,
): MobileWorkspace | null {
  return (
    workspaces.find((workspace) => workspace.id === persistedId) ??
    workspaces[0] ??
    null
  );
}

export function workspaceSelectionKey(sessionId: string): string {
  return `bex.mobile.workspace.${sessionId}`;
}
