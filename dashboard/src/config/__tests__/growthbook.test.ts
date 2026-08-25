import { describe, expect, it } from "vitest";
import {
  AGENTS_DASHBOARD_BETA_WORKSPACE_ID,
  GROWTHBOOK_FEATURE_KEYS,
  isAgentsDashboardEnabled,
  isGrowthBookFeatureEnabled,
} from "../growthbook";

describe("growthbook config", () => {
  it("enables dashboard agents only for the beta workspace", () => {
    expect(isAgentsDashboardEnabled(AGENTS_DASHBOARD_BETA_WORKSPACE_ID)).toBe(
      true,
    );
    expect(isAgentsDashboardEnabled("tea-other")).toBe(false);
    expect(isAgentsDashboardEnabled(null)).toBe(false);
    expect(isAgentsDashboardEnabled(undefined)).toBe(false);
  });

  it("returns false for unknown feature keys", () => {
    expect(
      isGrowthBookFeatureEnabled(
        "missing-feature" as (typeof GROWTHBOOK_FEATURE_KEYS)["agents"],
        { workspaceId: AGENTS_DASHBOARD_BETA_WORKSPACE_ID },
      ),
    ).toBe(false);
  });
});
