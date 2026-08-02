import { describe, expect, it } from "vitest";
import {
  PushNotificationEvent,
  PushNotificationUrgency,
  PushNotificationWeekday,
  type PushNotificationSettingsInput,
} from "@/graphql/definitions";
import { validatePushSettings } from "@/features/notifications/lib/push-policy";

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
});
