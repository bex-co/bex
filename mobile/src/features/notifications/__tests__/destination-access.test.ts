import { notificationActions } from "../destination-access";

describe("notificationActions", () => {
  it("asks for can_create on an automation pause, by event or by route", () => {
    const route = "/automations/aut-a1";
    expect(
      notificationActions({ event: "agent_automation_paused", route }),
    ).toEqual(["can_view", "can_create"]);
    // A mismatched pairing still guards the destination it opens.
    expect(notificationActions({ event: "deploy_failed", route })).toEqual([
      "can_view",
      "can_create",
    ]);
  });

  it("asks for can_operate on a session and only can_view on a service", () => {
    expect(
      notificationActions({ event: "agent_failed", route: "/sessions/ags-a1" }),
    ).toEqual(["can_view", "can_operate"]);
    expect(
      notificationActions({
        event: "deploy_failed",
        route: "/services/srv-a1",
      }),
    ).toEqual(["can_view"]);
  });
});
