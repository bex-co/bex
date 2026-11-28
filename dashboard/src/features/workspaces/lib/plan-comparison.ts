import {
  WORKSPACE_PLAN_IDS,
  environmentsPerProjectLimit,
  organizationPlan,
  proFeaturesPlan,
  type WorkspacePlanId,
} from "@/features/workspaces/types";

/**
 * The plan facts the confirmation's comparison card (w3/m708) sets side by
 * side, as Render's PlanComparisonCard does under "New plan summary". Each is
 * bex's own number, kept in step by hand with the backend that enforces it:
 * the seats (`store.LimitsFor`), the custom domains included before they are
 * metered at $0.175 a month (`core.IncludedCustomDomains`, ADR030 §16), the
 * environments per project (`core.MaxEnvironmentsPerProject`), the log and
 * metrics window (`core.LogRetention`), the builds kept for rollback
 * (`core.KeptBuilds`) and the features Pro and Scale add (`core` plan gates).
 */
interface PlanFacts {
  /** One seat, or unlimited. */
  justForMe: boolean;
  /** Included custom domains; null when all are included. */
  includedDomains: number | null;
  /** Two environments per project, or unlimited projects and environments. */
  twoEnvironments: boolean;
  retentionDays: number;
  /** HTTP request logs and response time metrics (Pro and higher). */
  requestLogs: boolean;
  keptBuilds: number;
  /** Workspace audit logs (Pro and higher). */
  auditLogs: boolean;
  /** An organization: more workspaces, extra roles, inbound IP rules. */
  organization: boolean;
}

/** The numbers the dashboard has no other copy of, by plan. */
const PLAN_NUMBERS: Record<
  WorkspacePlanId,
  Pick<
    PlanFacts,
    "justForMe" | "includedDomains" | "retentionDays" | "keptBuilds"
  >
> = {
  hobby: {
    justForMe: true,
    includedDomains: 2,
    retentionDays: 7,
    keptBuilds: 5,
  },
  pro: {
    justForMe: false,
    includedDomains: 15,
    retentionDays: 14,
    keptBuilds: 15,
  },
  scale: {
    justForMe: false,
    includedDomains: 25,
    retentionDays: 30,
    keptBuilds: 30,
  },
  enterprise: {
    justForMe: false,
    includedDomains: null,
    retentionDays: 30,
    keptBuilds: 30,
  },
};

/** A plan's facts: its numbers, and the plan gates the dashboard already
 *  reads elsewhere (`types.ts`). */
function planFacts(plan: WorkspacePlanId): PlanFacts {
  return {
    ...PLAN_NUMBERS[plan],
    twoEnvironments: environmentsPerProjectLimit(plan) === 2,
    requestLogs: proFeaturesPlan(plan),
    auditLogs: proFeaturesPlan(plan),
    organization: organizationPlan(plan),
  };
}

/** bex's price for a custom domain above the included count (ADR030 §16). */
const CUSTOM_DOMAIN_PRICE = "$0.175";

/** One side of a highlight: a translation key, its values, and whether the
 *  plan lacks the feature (drawn crossed out, as Render draws it). */
export interface PlanHighlight {
  key: string;
  values?: Record<string, string | number>;
  missing?: boolean;
}

export interface PlanComparisonRow {
  id: keyof PlanFacts;
  current: PlanHighlight;
  next: PlanHighlight;
}

function highlight(id: keyof PlanFacts, facts: PlanFacts): PlanHighlight {
  switch (id) {
    case "justForMe":
      return {
        key: facts.justForMe
          ? "workspaces.planHighlightJustForMe"
          : "workspaces.planHighlightUnlimitedMembers",
      };
    case "includedDomains":
      return facts.includedDomains === null
        ? { key: "workspaces.planHighlightUnlimitedDomains" }
        : {
            key: "workspaces.planHighlightIncludedDomains",
            values: {
              included: facts.includedDomains,
              price: CUSTOM_DOMAIN_PRICE,
            },
          };
    case "twoEnvironments":
      return {
        key: facts.twoEnvironments
          ? "workspaces.planHighlightTwoEnvironments"
          : "workspaces.planHighlightUnlimitedEnvironments",
      };
    case "retentionDays":
      return {
        key: "workspaces.planHighlightRetention",
        values: { days: facts.retentionDays },
      };
    case "requestLogs":
      return {
        key: facts.requestLogs
          ? "workspaces.planHighlightRequestLogs"
          : "workspaces.planHighlightBasicLogs",
      };
    case "keptBuilds":
      return {
        key: "workspaces.planHighlightKeptBuilds",
        values: { builds: facts.keptBuilds },
      };
    case "auditLogs":
      return {
        key: "workspaces.planHighlightAuditLogs",
        missing: !facts.auditLogs,
      };
    case "organization":
      return {
        key: "workspaces.planHighlightOrganization",
        missing: !facts.organization,
      };
  }
}

const ROW_ORDER: readonly (keyof PlanFacts)[] = [
  "justForMe",
  "includedDomains",
  "twoEnvironments",
  "retentionDays",
  "requestLogs",
  "keptBuilds",
  "auditLogs",
  "organization",
];

/**
 * The highlights that differ between two plans, current beside new, in the
 * order Render lists them: people, domains, projects, the log window, logs and
 * metrics, then what the higher plan adds. A plan unknown to the catalog has
 * no row to compare.
 */
export function planComparisonRows(
  current: string,
  next: WorkspacePlanId,
): PlanComparisonRow[] {
  if (!(WORKSPACE_PLAN_IDS as readonly string[]).includes(current)) return [];
  const from = planFacts(current as WorkspacePlanId);
  const to = planFacts(next);
  return ROW_ORDER.filter((id) => from[id] !== to[id]).map((id) => ({
    id,
    current: highlight(id, from),
    next: highlight(id, to),
  }));
}
