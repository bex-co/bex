// The dashboard's view of bex-api's organizations (w7/m464,
// docs/ADR115-organizations.md): a Scale or Enterprise workspace is the primary
// of an org that holds unlimited workspaces on its plan. Roles are Render's
// Owner, Member and Guest; an org workspace is Public or Invite-only.

import type { OrganizationFieldsFragment } from "@/graphql/definitions";
import {
  DEFAULT_WORKSPACE_PRIVACY,
  toWorkspacePrivacy,
  type WorkspacePrivacy,
} from "@/features/workspaces/types";

type RawOrganizationWorkspace =
  OrganizationFieldsFragment["workspaces"][number];

/** An org workspace, its privacy read once (`toOrganization`). */
export type OrganizationWorkspace = Omit<
  RawOrganizationWorkspace,
  "privacy"
> & {
  privacy: WorkspacePrivacy;
};

export type Organization = Omit<OrganizationFieldsFragment, "workspaces"> & {
  workspaces: OrganizationWorkspace[];
};

/** bex-api's org, each workspace's privacy string read as a setting. */
export function toOrganization(raw: OrganizationFieldsFragment): Organization {
  return {
    ...raw,
    workspaces: raw.workspaces.map((workspace) => ({
      ...workspace,
      privacy:
        toWorkspacePrivacy(workspace.privacy) ?? DEFAULT_WORKSPACE_PRIVACY,
    })),
  };
}

/** Render's org roles, in the order the role select lists them. */
export const ORGANIZATION_ROLES = ["OWNER", "MEMBER", "GUEST"] as const;
export type OrganizationRole = (typeof ORGANIZATION_ROLES)[number];

/** The roles an org invite may carry: a Guest is made by a role change. */
export const ORGANIZATION_INVITE_ROLES = [
  "OWNER",
  "MEMBER",
] as const satisfies readonly OrganizationRole[];

/** bex-api's role string (either case), or null for one it does not send. */
export function toOrganizationRole(
  value: string | null | undefined,
): OrganizationRole | null {
  const upper = value?.toUpperCase();
  return ORGANIZATION_ROLES.find((role) => role === upper) ?? null;
}

export const ORGANIZATION_ROLE_LABEL_KEYS: Record<OrganizationRole, string> = {
  OWNER: "organizations.roleOwner",
  MEMBER: "organizations.roleMember",
  GUEST: "organizations.roleGuest",
};

/** The org tables' columns, before each one's row-actions head. */
export const WORKSPACE_COLUMN_KEYS = [
  "organizations.colWorkspace",
  "organizations.colPrivacy",
] as const;
export const MEMBER_COLUMN_KEYS = [
  "organizations.colMember",
  "organizations.colRole",
] as const;

/** The org's three pages, in the order its navigation lists them. */
export const ORGANIZATION_SECTIONS = [
  "workspaces",
  "members",
  "settings",
] as const;
export type OrganizationSection = (typeof ORGANIZATION_SECTIONS)[number];
