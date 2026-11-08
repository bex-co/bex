import { describe, expect, it } from "vitest";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import type { Translate } from "@/common/hooks/use-translations";
import {
  PROTECTED_PLACEMENT_CODE,
  aclBearing,
  placementErrorMessage,
  restrictionMarkers,
} from "../placement";

const env = (over: Partial<EnvironmentView>) =>
  ({
    protectedStatus: "unprotected",
    networkIsolationEnabled: false,
    ipAllowListEntries: [
      { cidrBlock: "0.0.0.0/0", description: "" },
      { cidrBlock: "::/0", description: "" },
    ],
    ...over,
  }) as EnvironmentView;

// w4/355: mirrors bex-api's aclBearing.
describe("aclBearing", () => {
  it("ignores the seeded allow-all default", () => {
    expect(aclBearing(env({}))).toBe(false);
    expect(restrictionMarkers(env({}))).toEqual([]);
  });
  it("counts an emptied (deny-all) list and a narrowing list", () => {
    expect(aclBearing(env({ ipAllowListEntries: [] }))).toBe(true);
    expect(restrictionMarkers(env({ ipAllowListEntries: [] }))).toEqual([
      "environments.markerAllowList",
    ]);
    expect(
      aclBearing(
        env({
          ipAllowListEntries: [{ cidrBlock: "10.0.0.0/8", description: "" }],
        }),
      ),
    ).toBe(true);
  });
  it("counts protected and isolated environments", () => {
    expect(aclBearing(env({ protectedStatus: "protected" }))).toBe(true);
    expect(aclBearing(env({ networkIsolationEnabled: true }))).toBe(true);
  });
});

// w3/m281: a raced protected-placement 403 reads as the reason the disabled
// control shows, in the user's language, not the server's English.
describe("placementErrorMessage", () => {
  const t = ((key: string) => `t:${key}`) as Translate;
  const refusal = (code: string, message: string) =>
    new CombinedGraphQLErrors({
      errors: [{ message, extensions: { code } }],
    });

  it("localizes the protected placement refusal", () => {
    expect(
      placementErrorMessage(
        refusal(PROTECTED_PLACEMENT_CODE, "only workspace Admins can …"),
        "generic",
        t,
      ),
    ).toBe("t:environments.protectedPlacementAdminOnly");
  });

  it("keeps any other refusal's own words, and the generic copy otherwise", () => {
    expect(
      placementErrorMessage(refusal("CONFLICT", "name taken"), "generic", t),
    ).toBe("Name taken");
    expect(
      placementErrorMessage(new Error("Failed to fetch"), "generic", t),
    ).toBe("generic");
  });
});
