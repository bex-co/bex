/**
 * Static GrowthBook feature definitions for dashboard-only flags.
 *
 * These definitions mirror GrowthBook's feature JSON shape so they can be
 * uploaded to a remote GrowthBook project later. Until then, the dashboard
 * evaluates them locally from this file.
 */

export interface GrowthBookFeatureRule {
  condition: Record<string, unknown>;
  force?: boolean;
}

export interface GrowthBookFeatureDefinition {
  defaultValue: boolean;
  rules: GrowthBookFeatureRule[];
}

export type GrowthBookFeatures = Record<string, GrowthBookFeatureDefinition>;

export const ROUTER_DASHBOARD_BETA_WORKSPACE_ID = "tea-d98210cbbpdc73dcrkvg";

export const growthbookFeatures = {
  "dashboard-router": {
    defaultValue: false,
    rules: [
      {
        condition: { workspaceId: ROUTER_DASHBOARD_BETA_WORKSPACE_ID },
        force: true,
      },
    ],
  },
  // Agent sessions and automations are open to every workspace; the flag is
  // kept as a kill switch (set defaultValue false to hide them everywhere).
  "dashboard-agents": {
    defaultValue: true,
    rules: [],
  },
} satisfies GrowthBookFeatures;

export const GROWTHBOOK_FEATURE_KEYS = {
  agents: "dashboard-agents",
  router: "dashboard-router",
} as const;

export type GrowthBookFeatureKey =
  (typeof GROWTHBOOK_FEATURE_KEYS)[keyof typeof GROWTHBOOK_FEATURE_KEYS];

export type GrowthBookAttributes = {
  workspaceId?: string | null;
};

function ruleMatches(
  condition: Record<string, unknown>,
  attributes: GrowthBookAttributes,
): boolean {
  return Object.entries(condition).every(([key, expected]) => {
    const actual = attributes[key as keyof GrowthBookAttributes];
    return (actual ?? "") === expected;
  });
}

/** Evaluate a boolean GrowthBook feature against the supplied attributes. */
export function isGrowthBookFeatureEnabled(
  featureKey: GrowthBookFeatureKey,
  attributes: GrowthBookAttributes,
): boolean {
  const feature = growthbookFeatures[featureKey];
  if (!feature) return false;

  for (const rule of feature.rules) {
    if (ruleMatches(rule.condition, attributes)) {
      return rule.force ?? feature.defaultValue;
    }
  }

  return feature.defaultValue;
}

/**
 * Whether the current workspace may access dashboard `/agents` and
 * `/automations` routes. A workspace must be selected.
 */
export function isAgentsDashboardEnabled(
  workspaceId: string | null | undefined,
): boolean {
  if (!workspaceId) return false;
  return isGrowthBookFeatureEnabled(GROWTHBOOK_FEATURE_KEYS.agents, {
    workspaceId,
  });
}

export function isRouterDashboardEnabled(
  workspaceId: string | null | undefined,
): boolean {
  return isGrowthBookFeatureEnabled(GROWTHBOOK_FEATURE_KEYS.router, {
    workspaceId,
  });
}
