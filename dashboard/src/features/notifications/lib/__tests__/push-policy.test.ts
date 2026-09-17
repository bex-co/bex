import { describe, expect, it } from "vitest";
import {
  PushNotificationEvent,
  PushNotificationUrgency,
  PushNotificationWeekday,
  type PushNotificationSettingsInput,
} from "@/graphql/definitions";
import {
  pushEvents,
  validatePushSettings,
} from "@/features/notifications/lib/push-policy";

function validSettings(): PushNotificationSettingsInput {
  return {
    enabled: true,
    events: [PushNotificationEvent.DeployFailed],
    minimumUrgency: PushNotificationUrgency.Important,
    timeZone: "America/Los_Angeles",
    workingHours: [
      {
        weekdays: [PushNotificationWeekday.Monday],
        start: "09:00",
        end: "17:00",
      },
    ],
    quietHours: [],
    maxDeferralSeconds: 28_800,
    serviceOverrides: [],
  };
}

describe("validatePushSettings", () => {
  it("accepts an IANA timezone and a DST-aware overnight range", () => {
    const settings = validSettings();
    settings.quietHours = [
      {
        weekdays: [PushNotificationWeekday.Sunday],
        start: "22:00",
        end: "06:00",
      },
    ];
    expect(validatePushSettings(settings)).toBeNull();
  });

  it("rejects invalid local input before it reaches the mutation", () => {
    const settings = validSettings();
    settings.timeZone = "Local";
    expect(validatePushSettings(settings)).toBe(
      "notifications.pushInvalidTimezone",
    );

    settings.timeZone = "UTC";
    settings.workingHours[0].end = "09:00";
    expect(validatePushSettings(settings)).toBe(
      "notifications.pushInvalidRange",
    );
  });

  it("preserves inheritance but rejects an override that changes nothing", () => {
    const settings = validSettings();
    settings.serviceOverrides = [{ serviceId: "srv-c185th5c2rvvnhbfiltg" }];
    expect(validatePushSettings(settings)).toBe(
      "notifications.pushEmptyOverride",
    );

    settings.serviceOverrides[0].events = [];
    expect(validatePushSettings(settings)).toBeNull();
  });

  it.each([
    ["unknown", ["future_event" as PushNotificationEvent]],
    [
      "too many",
      Array.from(
        { length: pushEvents.length + 1 },
        () => PushNotificationEvent.DeployFailed,
      ),
    ],
  ])("rejects %s events at both policy levels", (_label, events) => {
    const settings = validSettings();
    settings.events = events;
    expect(validatePushSettings(settings)).toBe(
      "notifications.pushInvalidEvents",
    );

    settings.events = [];
    settings.serviceOverrides = [
      { serviceId: "srv-c185th5c2rvvnhbfiltg", events },
    ];
    expect(validatePushSettings(settings)).toBe(
      "notifications.pushInvalidEvents",
    );
  });

  it("allows duplicate known events within the existing count limit", () => {
    const settings = validSettings();
    const events = [
      PushNotificationEvent.DeployFailed,
      PushNotificationEvent.DeployFailed,
    ];
    settings.events = events;
    settings.serviceOverrides = [
      { serviceId: "srv-c185th5c2rvvnhbfiltg", events },
    ];
    expect(validatePushSettings(settings)).toBeNull();
  });

  it.each([
    ["invalid id", { serviceId: "not-a-service", enabled: false }],
    ["duplicate id", { serviceId: "srv-c185th5c2rvvnhbfiltg", events: [] }],
  ])("rejects an override with an %s", (_label, override) => {
    const settings = validSettings();
    settings.serviceOverrides = [
      { serviceId: "srv-c185th5c2rvvnhbfiltg", enabled: true },
      override,
    ];
    expect(validatePushSettings(settings)).toBe(
      "notifications.pushInvalidService",
    );
  });

  it("treats null fields as inherited and false as an explicit override", () => {
    const settings = validSettings();
    settings.serviceOverrides = [
      {
        serviceId: "srv-c185th5c2rvvnhbfiltg",
        enabled: null,
        events: null,
        minimumUrgency: null,
      },
    ];
    expect(validatePushSettings(settings)).toBe(
      "notifications.pushEmptyOverride",
    );
    settings.serviceOverrides[0].enabled = false;
    expect(validatePushSettings(settings)).toBeNull();
  });

  it("validates an override urgency even when its event filter is inherited", () => {
    const settings = validSettings();
    settings.serviceOverrides = [
      {
        serviceId: "srv-c185th5c2rvvnhbfiltg",
        minimumUrgency: "future_urgency" as PushNotificationUrgency,
      },
    ];
    expect(validatePushSettings(settings)).toBe(
      "notifications.pushInvalidUrgency",
    );
    settings.serviceOverrides[0].minimumUrgency =
      PushNotificationUrgency.Critical;
    expect(validatePushSettings(settings)).toBeNull();
  });

  it("reports rule limits before inspecting service overrides", () => {
    const settings = validSettings();
    settings.serviceOverrides = Array.from({ length: 101 }, () => ({
      serviceId: "invalid",
    }));
    expect(validatePushSettings(settings)).toBe(
      "notifications.pushTooManyRules",
    );
  });

  it("reports the first invalid override and preserves field precedence", () => {
    const settings = validSettings();
    settings.serviceOverrides = [
      {
        serviceId: "srv-c185th5c2rvvnhbfiltg",
        events: ["future_event" as PushNotificationEvent],
        minimumUrgency: "future_urgency" as PushNotificationUrgency,
      },
      { serviceId: "invalid" },
    ];
    expect(validatePushSettings(settings)).toBe(
      "notifications.pushInvalidEvents",
    );
    settings.serviceOverrides[0].events = [];
    expect(validatePushSettings(settings)).toBe(
      "notifications.pushInvalidUrgency",
    );
    settings.serviceOverrides[0].minimumUrgency =
      PushNotificationUrgency.Routine;
    expect(validatePushSettings(settings)).toBe(
      "notifications.pushInvalidService",
    );
  });
});
