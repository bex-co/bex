import type { HistoryState } from "@tanstack/history";
import { safeNext as sameOriginPath } from "@/common/lib/safe-next";
import type { BillingReadiness } from "@/features/usage/hooks/use-billing-onboarding";
import { isPaidWorkspacePlan } from "@/features/workspaces/lib/create-workspace-payment";
import {
  WORKSPACE_PLAN_IDS,
  organizationPlan,
  organizationPrimary,
  type WorkspacePlanId,
  type WorkspaceView,
} from "@/features/workspaces/types";

/**
 * Render's plan pages (w3/m708, docs/render-artifacts/workspace-plan-change.md):
 * `/billing/update-plan` chooses the plan, `/billing/update-plan-confirm`
 * reviews it and asks for the typed phrase before a downgrade, and
 * `/billing/update-plan-preview` chooses which custom domains a capped plan
 * keeps. Their URL grammar, read from Render's dashboard bundle
 * (`billing-confirm-plan-utils`, 2026-11-27), lives here so the three pages and
 * every link into them agree.
 */

declare module "@tanstack/history" {
  interface HistoryState {
    /**
     * Where the plan pages return to when the URL names no `next`, handed
     * over in the navigation's state as Render's `afterPlanUpdate` is: the
     * Billing page for a link from Billing, Workspace Settings or Render's
     * `/w/<id>/billing/update-plan`.
     */
    afterPlanUpdate?: string;
  }
}

/** The navigation state a link from Billing or Workspace Settings hands the
 *  plan page, so Cancel and success return to Billing as Render's do. */
export const AFTER_PLAN_UPDATE_BILLING: HistoryState = {
  afterPlanUpdate: "/billing",
};

/** The plan pages' search: Render's `plan` alias and its `next` path. */
export type UpdatePlanSearch = {
  plan?: string;
  next?: string;
};

/**
 * The plan a `?plan=` value names: Render's aliases (`team`, `pro` and
 * `professional` for Pro; `org`, `scale` and `organization` for Scale).
 * Enterprise has none, as on Render, where it is Get in touch.
 */
export function planFromSearch(value: unknown): WorkspacePlanId | undefined {
  switch (value) {
    case "hobby":
      return "hobby";
    case "team":
    case "pro":
    case "professional":
      return "pro";
    case "org":
    case "scale":
    case "organization":
      return "scale";
    default:
      return undefined;
  }
}

/** The `?plan=` value Render's links write for a plan. */
export function planSearchValue(plan: WorkspacePlanId): string {
  switch (plan) {
    case "pro":
      return "professional";
    case "scale":
      return "organization";
    default:
      return plan;
  }
}

/**
 * A `next` worth returning to: a path on this dashboard, by the login's own
 * same-origin check (`common/lib/safe-next`). Anything else is dropped rather
 * than sent home, as Render's pages drop a `next` they cannot read.
 */
export function readNext(value: unknown): string | undefined {
  if (typeof value !== "string" || value === "") return undefined;
  const path = sameOriginPath(value);
  return path === "/" && value !== "/" ? undefined : path;
}

/**
 * Reads the plan pages' search, keeping only values they can use. Both keys
 * come back, undefined when absent or rejected: the router merges this over
 * the raw search, so an omitted `?next=https://…` reached Cancel and success
 * raw, an open redirect (w3/m708's production walk, w5/078's rule).
 */
export function validateUpdatePlanSearch(
  search: Record<string, unknown>,
): UpdatePlanSearch {
  const plan = planFromSearch(search.plan);
  return {
    plan: plan && planSearchValue(plan),
    next: readNext(search.next),
  };
}

/** The search a link into the plan pages writes, naming only what it has. */
export function updatePlanSearch(
  plan?: WorkspacePlanId,
  next?: unknown,
): UpdatePlanSearch {
  const search: UpdatePlanSearch = {};
  if (plan) search.plan = planSearchValue(plan);
  const path = readNext(next);
  if (path) search.next = path;
  return search;
}

/**
 * Where an upgrade link goes when it is followed: the plan page on `plan`,
 * coming back to the page the link was on (Render's
 * `?plan=professional&next=${window.location.pathname}`). Read at click time,
 * in the browser.
 */
export function upgradePlanTarget(plan: WorkspacePlanId) {
  return {
    to: "/billing/update-plan" as const,
    search: updatePlanSearch(
      plan,
      typeof window === "undefined" ? undefined : window.location.pathname,
    ),
  };
}

/**
 * Where Cancel, Back and success go: `next`, else what the link handed over,
 * else the dashboard's home, as Render's pages go. Render keeps the plan
 * page's own hash on `next`.
 */
export function updatePlanReturnPath(
  next: string | undefined,
  afterPlanUpdate: unknown,
  hash = "",
): string {
  if (next) return hash ? `${next}#${hash}` : next;
  return readNext(afterPlanUpdate) ?? "/";
}

/**
 * Whether a card is still needed for `plan`: a paid plan while the platform
 * requires a payment method (BEX_REQUIRE_PAYMENT_METHOD) and none is on file,
 * Render's "requires a card" rule. Unknown (still loading) reads as not
 * needed: bex-api's RequirePlanBilling refuses the change either way.
 */
export function cardMissing(
  plan: WorkspacePlanId,
  readiness: Pick<
    BillingReadiness,
    "paymentMethodRequired" | "paymentMethodReady"
  > | null,
): boolean {
  return (
    isPaidWorkspacePlan(plan) &&
    readiness !== null &&
    readiness.paymentMethodRequired &&
    !readiness.paymentMethodReady
  );
}

/** Whether moving from one plan to another is a downgrade (the catalog is
 *  in ladder order). */
export function isDowngrade(from: string, to: WorkspacePlanId): boolean {
  return (
    WORKSPACE_PLAN_IDS.indexOf(to) <
    WORKSPACE_PLAN_IDS.indexOf(from as WorkspacePlanId)
  );
}

/** Whether moving `workspace` to `plan` ends its organization: an org
 *  primary going below Scale (ADR115 D6). */
export function removesOrganization(
  workspace: Pick<WorkspaceView, "organization" | "primary" | "plan">,
  plan: WorkspacePlanId,
): boolean {
  return (
    isDowngrade(workspace.plan, plan) &&
    organizationPrimary(workspace) &&
    !organizationPlan(plan)
  );
}

/** Render's whitespace rule for the downgrade phrase: trimmed, each run of
 *  whitespace one space. */
function normalizePhrase(value: string): string {
  return value.trim().replace(/\s+/g, " ");
}

/** The phrase Render asks for before a downgrade:
 *  `sudo downgrade workspace <name>`, the name normalized. */
export function downgradeConfirmationPhrase(name: string): string {
  return `sudo downgrade workspace ${normalizePhrase(name)}`;
}

/** Whether what was typed is the phrase, under the same whitespace rule. */
export function downgradeConfirmationMatches(
  typed: string,
  phrase: string,
): boolean {
  return normalizePhrase(typed) === phrase;
}
