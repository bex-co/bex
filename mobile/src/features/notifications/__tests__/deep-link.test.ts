import {
  notificationMatchesBinding,
  parseNotificationEnvelope,
} from "../deep-link";

const valid = {
  schema: "bex.notification.v1",
  notificationId: "evt-123",
  event: "deploy_failed",
  route: "/services/srv-abc123",
  subject: "identity-1",
  workspaceId: "tea-1",
  sessionId: "session-1",
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

  it("accepts the observed-lifecycle events (w3/m78)", () => {
    for (const event of [
      "server_available",
      "service_suspended",
      "service_resumed",
    ] as const) {
      expect(parseNotificationEnvelope({ ...valid, event })?.event).toBe(event);
    }
  });

  it("rejects unknown events, schemas, ids, and extra fields", () => {
    expect(parseNotificationEnvelope({ ...valid, event: "billing_due" })).toBe(
      null,
    );
    expect(
      parseNotificationEnvelope({ ...valid, event: "usage_threshold" }),
    ).toBe(null);
    expect(parseNotificationEnvelope({ ...valid, schema: "v2" })).toBe(null);
    expect(
      parseNotificationEnvelope({ ...valid, notificationId: "../bad" }),
    ).toBe(null);
    expect(
      parseNotificationEnvelope({ ...valid, url: "https://evil.test" }),
    ).toBe(null);
  });

  it("accepts only the exact authenticated account, workspace, and session epoch", () => {
    const envelope = parseNotificationEnvelope(valid);
    expect(envelope).not.toBe(null);
    if (!envelope) return;
    expect(
      notificationMatchesBinding(envelope, {
        subject: "identity-1",
        workspaceId: "tea-1",
        sessionId: "session-1",
      }),
    ).toBe(true);
    for (const binding of [
      null,
      { subject: "identity-old", workspaceId: "tea-1", sessionId: "session-1" },
      { subject: "identity-1", workspaceId: "tea-old", sessionId: "session-1" },
      { subject: "identity-1", workspaceId: "tea-1", sessionId: "session-old" },
    ]) {
      expect(notificationMatchesBinding(envelope, binding)).toBe(false);
    }
  });
});
