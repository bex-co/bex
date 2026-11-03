import { describe, expect, it } from "vitest";

import {
  SCOPE_API_COMPAT,
  SCOPE_CLOUD_AGENTS,
  SCOPE_DESCRIPTION_KEYS,
  grantableScopes,
  hasGranularCapability,
} from "../oauth-scopes";

describe("bex.cloud_agents scope (ADR094 D9)", () => {
  it("counts as a granular capability on its own", () => {
    expect(hasGranularCapability([SCOPE_CLOUD_AGENTS])).toBe(true);
    expect(hasGranularCapability(["openid"])).toBe(false);
  });

  it("is kept for third-party clients, which still lose the bex.api alias", () => {
    expect(
      grantableScopes(["openid", SCOPE_CLOUD_AGENTS, SCOPE_API_COMPAT], false),
    ).toEqual(["openid", SCOPE_CLOUD_AGENTS]);
  });

  it("has a consent description", () => {
    expect(SCOPE_DESCRIPTION_KEYS[SCOPE_CLOUD_AGENTS]).toBe(
      "auth.consentScopeCloudAgents",
    );
  });
});
