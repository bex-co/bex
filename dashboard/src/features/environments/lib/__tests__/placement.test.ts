import { describe, expect, it } from "vitest";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import { aclBearing, restrictionMarkers } from "../placement";

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
