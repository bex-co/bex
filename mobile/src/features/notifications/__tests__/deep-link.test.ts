import { parseNotificationEnvelope } from "../deep-link";

const valid = {
  schema: "bex.notification.v1",
  notificationId: "evt-123",
  event: "deploy_failed",
  route: "/services/srv-abc123",
};

describe("parseNotificationEnvelope", () => {
  it("accepts only the four authenticated detail routes", () => {
    expect(parseNotificationEnvelope(valid)?.route).toBe(
      "/services/srv-abc123",
    );
    expect(
      parseNotificationEnvelope({ ...valid, route: "/databases/dpg-a1" })
        ?.route,
    ).toBe("/databases/dpg-a1");
    expect(
      parseNotificationEnvelope({ ...valid, route: "/key-values/red-a1" })
        ?.route,
    ).toBe("/key-values/red-a1");
    expect(
      parseNotificationEnvelope({ ...valid, route: "/sessions/ags-a1" })?.route,
    ).toBe("/sessions/ags-a1");
  });

  it("rejects absolute, traversal, query, fragment, stale, and mismatched routes", () => {
    for (const route of [
      "https://evil.test/services/srv-a",
      "/services/../sessions/ags-a",
      "/services/srv-a?next=https://evil.test",
      "/services/srv-a#fragment",
      "/service/srv-a",
      "/services/dpg-a",
      "/settings",
    ]) {
      expect(parseNotificationEnvelope({ ...valid, route })).toBe(null);
    }
  });

  it("rejects unknown events, schemas, ids, and extra fields", () => {
    expect(parseNotificationEnvelope({ ...valid, event: "billing_due" })).toBe(
      null,
    );
    expect(parseNotificationEnvelope({ ...valid, schema: "v2" })).toBe(null);
    expect(
      parseNotificationEnvelope({ ...valid, notificationId: "../bad" }),
    ).toBe(null);
    expect(
      parseNotificationEnvelope({ ...valid, url: "https://evil.test" }),
    ).toBe(null);
  });
});
