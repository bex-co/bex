import {
  getCookie,
  setCookie,
  removeCookie,
} from "@/common/hooks/use-cookie-storage-state/cookie";
import {
  organizationPlanOwner,
  type OrganizationPlanOwner,
  type WorkspaceOrganizationRef,
  type WorkspaceView,
} from "@/features/workspaces/types";

export const WORKSPACE_SELECTION_KEY = "bex.selectedWorkspaceId";
/** The selected workspace's shape, beside the selection (w3/073). */
export const WORKSPACE_SHAPE_KEY = "bex.selectedWorkspaceShape";
const WORKSPACE_SHAPE_VERSION = 1;

const SELECTION_COOKIE_OPTIONS = {
  expires: 365,
  sameSite: "lax",
  path: "/",
} as const;

export function getPersistedWorkspaceId(): string | null {
  const value = getCookie(WORKSPACE_SELECTION_KEY)?.trim();
  return value ? value : null;
}

export function persistWorkspaceId(id: string): void {
  setCookie(WORKSPACE_SELECTION_KEY, id, SELECTION_COOKIE_OPTIONS);
}

/** Clears the cookie after confirmed last-workspace removal (w6/m144), and
 *  the selected workspace's shape stored beside it (w3/073). */
export function clearPersistedWorkspaceId(): void {
  removeCookie(WORKSPACE_SELECTION_KEY, { path: "/" });
  removeCookie(WORKSPACE_SHAPE_KEY, { path: "/" });
}

/**
 * What a pending frame reads from the workspace list about the selected
 * workspace: which sections its pages have. A cold load has only the
 * selection cookie until the list arrives, so the provider stores this beside
 * it whenever the list is known, and the next cold frame draws the sections
 * the last visit saw (w3/073). It holds no copy the frame would print: the
 * workspace's name stays a placeholder until the list loads.
 */
export interface WorkspaceShape {
  role: string;
  plan: string;
  /** The org the workspace belongs to: Workspace privacy and the org's login
   *  requirement notes (w7/m464, w7/m466). */
  organization: WorkspaceOrganizationRef | null;
  /** The org-plan note of a secondary workspace (w7/m464). */
  planOwner: OrganizationPlanOwner | null;
  /** The caller has another workspace to land in: the Delete card. */
  otherWorkspaces: boolean;
}

export function workspaceShape(
  workspace: WorkspaceView,
  workspaces: readonly WorkspaceView[],
): WorkspaceShape {
  return {
    role: workspace.role,
    plan: workspace.plan,
    organization: workspace.organization,
    planOwner: organizationPlanOwner(workspace, workspaces),
    otherWorkspaces: workspaces.length > 1,
  };
}

/** Stores `shape` for workspace `id`; an unchanged shape is not rewritten. */
export function persistWorkspaceShape(id: string, shape: WorkspaceShape): void {
  const value = JSON.stringify({ v: WORKSPACE_SHAPE_VERSION, id, ...shape });
  if (getCookie(WORKSPACE_SHAPE_KEY) === value) return;
  setCookie(WORKSPACE_SHAPE_KEY, value, SELECTION_COOKIE_OPTIONS);
}

/** The stored shape of workspace `id`, or null when none is stored for it
 *  (a first visit, another workspace's, or an unreadable value). */
export function getPersistedWorkspaceShape(
  id: string | null | undefined,
): WorkspaceShape | null {
  if (!id) return null;
  const raw = getCookie(WORKSPACE_SHAPE_KEY);
  if (!raw) return null;
  let value: unknown;
  try {
    value = JSON.parse(raw);
  } catch {
    return null;
  }
  return readWorkspaceShape(value, id);
}

// The readers below return undefined for a value that is not what
// persistWorkspaceShape writes, so a hand-edited cookie draws the default.
type Fields = Record<string, unknown>;

const isFields = (value: unknown): value is Fields =>
  typeof value === "object" && value !== null;

function readOrganization(
  value: unknown,
): WorkspaceOrganizationRef | null | undefined {
  if (value === null) return null;
  if (
    !isFields(value) ||
    typeof value.id !== "string" ||
    typeof value.name !== "string" ||
    typeof value.primaryWorkspaceId !== "string" ||
    typeof value.requireTwoFactor !== "boolean" ||
    typeof value.requireGoogleLogin !== "boolean"
  ) {
    return undefined;
  }
  return {
    id: value.id,
    name: value.name,
    primaryWorkspaceId: value.primaryWorkspaceId,
    requireTwoFactor: value.requireTwoFactor,
    requireGoogleLogin: value.requireGoogleLogin,
  };
}

function readPlanOwner(
  value: unknown,
): OrganizationPlanOwner | null | undefined {
  if (value === null) return null;
  const primary = isFields(value) ? value.primaryWorkspaceId : undefined;
  return primary === null || typeof primary === "string"
    ? { primaryWorkspaceId: primary }
    : undefined;
}

function readWorkspaceShape(value: unknown, id: string): WorkspaceShape | null {
  if (
    !isFields(value) ||
    value.v !== WORKSPACE_SHAPE_VERSION ||
    value.id !== id ||
    typeof value.role !== "string" ||
    typeof value.plan !== "string" ||
    typeof value.otherWorkspaces !== "boolean"
  ) {
    return null;
  }
  const organization = readOrganization(value.organization);
  const planOwner = readPlanOwner(value.planOwner);
  if (organization === undefined || planOwner === undefined) return null;
  return {
    role: value.role,
    plan: value.plan,
    organization,
    planOwner,
    otherWorkspaces: value.otherWorkspaces,
  };
}
