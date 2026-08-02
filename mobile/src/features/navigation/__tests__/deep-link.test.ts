import {
  validAgentSessionDeepLink,
  validDatabaseDeepLink,
  validKeyValueDeepLink,
  validServiceDeepLink,
} from "../deep-link";

describe("native deep links", () => {
  it("accepts only canonical opaque service IDs", () => {
    expect(validServiceDeepLink("srv-abc123")).toBe(true);
    expect(validServiceDeepLink("srv-../other")).toBe(false);
    expect(validServiceDeepLink(["srv-one", "srv-two"])).toBe(false);
  });

  it("accepts only canonical opaque agent-session IDs", () => {
    expect(validAgentSessionDeepLink("ags-abc123")).toBe(true);
    expect(validAgentSessionDeepLink("srv-abc123")).toBe(false);
    expect(validAgentSessionDeepLink("ags-ABC123")).toBe(false);
  });

  it("accepts only canonical datastore IDs", () => {
    expect(validDatabaseDeepLink("dpg-abc123")).toBe(true);
    expect(validDatabaseDeepLink("red-abc123")).toBe(false);
    expect(validKeyValueDeepLink("red-abc123")).toBe(true);
    expect(validKeyValueDeepLink("red-../other")).toBe(false);
  });
});
