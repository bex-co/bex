// bex-native projection of bex-api's workspace-lifecycle GraphQL `Workspace`
// (backend/internal/workspaces/graphql.go, w6/m1). A workspace is Render's
// "owner" — the tenant a session's Apps/Databases/API keys belong to.

export interface WorkspaceView {
  id: string;
  /** Render's freeform workspace name (w3/m246). */
  name: string;
  /** The unique DNS-label key (`BEX_WORKSPACE`, `X-Bex-Workspace`). */
  slug: string;
  /** Workspace plan id — one of WORKSPACE_PLAN_IDS (store.WorkspacePlans). */
  plan: string;
  /** The caller's role in this workspace ("admin" for every m1 membership). */
  role: string;
  createdAt: string | null;
  /**
   * Whether an admin requires every member to use two-factor authentication
   * (w22/m402, docs/ADR012-auth.md 9a). Readable by every member, and the list
   * it rides on is exempt from enforcement, so a blocked member can read it.
   */
  requireTwoFactor: boolean;
  /**
   * The organization the workspace belongs to (w7/m464,
   * docs/ADR115-organizations.md): Scale and Enterprise workspaces are an
   * org's primary or one of its secondaries; null for a standalone workspace.
   */
  organization: WorkspaceOrganizationRef | null;
  /** Whether this workspace is its org's primary (holds the org's plan). */
  primary: boolean;
  /** An org workspace's privacy, "public" or "invite_only"; null outside an org. */
  privacy: WorkspacePrivacy | null;
}

/** The org a workspace belongs to, as the workspaces list carries it. */
export interface WorkspaceOrganizationRef {
  id: string;
  name: string;
  primaryWorkspaceId: string;
}

/** An org workspace's privacy setting (Render: Public or Invite-only). */
export type WorkspacePrivacy = "public" | "invite_only";

/** Render's two privacy settings, in its order: Public, then Invite-only. */
export const WORKSPACE_PRIVACY_OPTIONS: readonly WorkspacePrivacy[] = [
  "public",
  "invite_only",
];

/** Render's default: a new org workspace starts Invite-only. */
export const DEFAULT_WORKSPACE_PRIVACY: WorkspacePrivacy = "invite_only";

/** Each privacy setting's Render name and what it allows. */
export const WORKSPACE_PRIVACY_KEYS: Record<
  WorkspacePrivacy,
  { label: string; description: string }
> = {
  public: {
    label: "workspaces.privacyPublic",
    description: "workspaces.privacyPublicDescription",
  },
  invite_only: {
    label: "workspaces.privacyInviteOnly",
    description: "workspaces.privacyInviteOnlyDescription",
  },
};

/** Reads bex-api's privacy string; anything unknown is Invite-only, Render's default. */
export function toWorkspacePrivacy(
  value: string | null | undefined,
): WorkspacePrivacy | null {
  if (value == null) return null;
  return value === "public" ? "public" : DEFAULT_WORKSPACE_PRIVACY;
}

/**
 * Where a secondary workspace's plan is managed (w7/m464, ADR115 D2): Render's
 * "The org plan is managed from the primary workspace". Null when the
 * workspace holds its own plan, standalone or its org's primary.
 * `primaryWorkspaceId` is null when the caller is not in the primary, so there
 * is no page of it to link to (the `/w/$` alias refuses a non-member).
 */
export interface OrganizationPlanOwner {
  primaryWorkspaceId: string | null;
}

export function organizationPlanOwner(
  workspace: Pick<WorkspaceView, "organization" | "primary"> | null | undefined,
  workspaces: readonly Pick<WorkspaceView, "id">[],
): OrganizationPlanOwner | null {
  const organization = workspace?.organization;
  if (!organization || workspace.primary) return null;
  const primaryId = organization.primaryWorkspaceId;
  return {
    primaryWorkspaceId: workspaces.some((w) => w.id === primaryId)
      ? primaryId
      : null,
  };
}

/** Whether a workspace is its org's primary, where the org's plan changes. */
export function organizationPrimary(
  workspace: Pick<WorkspaceView, "organization" | "primary">,
): boolean {
  return workspace.organization !== null && workspace.primary;
}

/**
 * The exact phrase the user must type to arm a workspace delete, cloning
 * Render's live dashboard guard verbatim (docs/render-artifacts/workspace-lifecycle.md,
 * captured 2026-07-11): "sudo delete workspace <name>", not the bare name. Kept
 * in lockstep with the backend's `DeleteConfirmation` helper
 * (backend/internal/workspaces/service.go), which re-validates the same phrase.
 */
export function workspaceDeleteConfirmation(name: string): string {
  return `sudo delete workspace ${name}`;
}

/**
 * The bex workspace capability lineup mirrors Render's plan ids (verified
 * 2026-07-08, .pm/w6/RESEARCH-workspaces.md finding 1 + 4): Hobby is capped
 * (1 member, 25 services, 5 workspaces/user); Pro/Scale/Enterprise lift caps
 * and add roles. Monthly fees are Render × 0.70 (Hobby $0, Pro $17.50, Scale
 * $349.30; Enterprise custom) — keep the locale billing strings in lockstep
 * with lego/backend/internal/pricing/pricing.yaml, where Scale's fee is
 * `per: organization` (w7/m464): its billing label says so, as Enterprise's
 * does (`plan-billing-labels.test.ts`). Resource-tier usage is
 * billed separately (ADR040, ADR046). Kept in sync by hand with the backend
 * catalog (store.WorkspacePlans) — this module has no schema-introspection
 * path to the Go constants.
 */
export const WORKSPACE_PLAN_IDS = [
  "hobby",
  "pro",
  "scale",
  "enterprise",
] as const;
export type WorkspacePlanId = (typeof WORKSPACE_PLAN_IDS)[number];

export interface WorkspacePlanCatalogEntry {
  id: WorkspacePlanId;
  nameKey: string;
  billingKey: string;
  descriptionKey: string;
  bulletKeys: readonly string[];
}

export const WORKSPACE_PLAN_CATALOG: WorkspacePlanCatalogEntry[] = [
  {
    id: "hobby",
    nameKey: "workspaces.planHobbyName",
    billingKey: "workspaces.planHobbyBilling",
    descriptionKey: "workspaces.planHobbyDescription",
    bulletKeys: [
      "workspaces.planHobbyBulletMembers",
      "workspaces.planHobbyBulletServices",
      "workspaces.planHobbyBulletWorkspaces",
    ],
  },
  {
    id: "pro",
    nameKey: "workspaces.planProName",
    billingKey: "workspaces.planProBilling",
    descriptionKey: "workspaces.planProDescription",
    bulletKeys: [
      "workspaces.planProBulletMembers",
      "workspaces.planProBulletServices",
    ],
  },
  {
    id: "scale",
    nameKey: "workspaces.planScaleName",
    billingKey: "workspaces.planScaleBilling",
    descriptionKey: "workspaces.planScaleDescription",
    bulletKeys: [
      "workspaces.planScaleBulletMembers",
      "workspaces.planScaleBulletServices",
      "workspaces.planScaleBulletRoles",
    ],
  },
  {
    id: "enterprise",
    nameKey: "workspaces.planEnterpriseName",
    billingKey: "workspaces.planEnterpriseBilling",
    descriptionKey: "workspaces.planEnterpriseDescription",
    bulletKeys: [
      "workspaces.planEnterpriseBulletLimits",
      "workspaces.planEnterpriseBulletSupport",
    ],
  },
];

/** Whether a plan holds an organization (Render: Scale and Enterprise). */
export function organizationPlan(plan: WorkspacePlanId): boolean {
  return plan === "scale" || plan === "enterprise";
}

export function workspacePlanNameKey(plan: string): string {
  return WORKSPACE_PLAN_CATALOG.find((p) => p.id === plan)?.nameKey ?? "";
}

/**
 * Whether a workspace plan includes Render's Pro-and-higher features (network
 * isolation, webhooks, HTTP request logs). An unknown plan (still loading) is
 * treated as included; the API enforces the rule either way.
 */
export function proFeaturesPlan(plan: string | null | undefined): boolean {
  return plan !== "hobby";
}

/**
 * Whether a workspace plan may set inbound IP rules on services, static sites,
 * environments and the workspace itself (Render: Scale and Enterprise). An
 * unknown plan (still loading) is treated as allowed; the API enforces the
 * rule either way.
 */
export function inboundIpRulesPlan(plan: string | null | undefined): boolean {
  return !plan || plan === "scale" || plan === "enterprise";
}
