// bex-native projection of bex-api's workspace-lifecycle GraphQL `Workspace`
// (backend/internal/workspaces/graphql.go, w6/m1). A workspace is Render's
// "owner" — the tenant a session's Apps/Databases/API keys belong to.

export interface WorkspaceView {
  id: string;
  name: string;
  /** Workspace plan id — one of WORKSPACE_PLAN_IDS (store.WorkspacePlans). */
  plan: string;
  /** The caller's role in this workspace ("admin" for every m1 membership). */
  role: string;
  createdAt: string | null;
}

// Mirrors the backend's workspace-name validation exactly (nameRE,
// backend/internal/workspaces/service.go): a DNS label of 1-30 chars, because
// a bex workspace name becomes part of every App CR name
// ("<workspace>-<app>") — unlike Render's freeform names (documented parity
// drift, w6/m1/t007). Validating client-side here only pre-empts a round
// trip; the backend re-validates regardless.
export const WORKSPACE_NAME_RE = /^[a-z0-9]([a-z0-9-]{0,28}[a-z0-9])?$/;

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
 * and add roles. Unlike Render, bex bills resource-tier usage rather than flat
 * workspace subscriptions (ADR040, ADR046). Kept in sync by hand with the
 * backend catalog (store.WorkspacePlans) — this module has no schema-
 * introspection path to the Go constants.
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
}

export const WORKSPACE_PLAN_CATALOG: WorkspacePlanCatalogEntry[] = [
  {
    id: "hobby",
    nameKey: "workspaces.planHobbyName",
    billingKey: "workspaces.planHobbyBilling",
    descriptionKey: "workspaces.planHobbyDescription",
  },
  {
    id: "pro",
    nameKey: "workspaces.planProName",
    billingKey: "workspaces.planProBilling",
    descriptionKey: "workspaces.planProDescription",
  },
  {
    id: "scale",
    nameKey: "workspaces.planScaleName",
    billingKey: "workspaces.planScaleBilling",
    descriptionKey: "workspaces.planScaleDescription",
  },
  {
    id: "enterprise",
    nameKey: "workspaces.planEnterpriseName",
    billingKey: "workspaces.planEnterpriseBilling",
    descriptionKey: "workspaces.planEnterpriseDescription",
  },
];
