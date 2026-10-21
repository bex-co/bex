import { describe, expect, it } from "vitest";
import {
  PushNotificationEvent,
  PushNotificationUrgency,
  PushNotificationWeekday,
  type PushNotificationSettingsInput,
} from "@/graphql/definitions";
import {
  droppedSelections,
  eventsBelowMinimum,
  publishedPushEvents,
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
  it("bounds the max deferral at 0.5 and 168 hours (w4/387)", () => {
    const at = (seconds: number) => {
      const settings = validSettings();
      settings.maxDeferralSeconds = seconds;
      return validatePushSettings(settings);
    };
    expect(at(1800)).toBeNull();
    expect(at(168 * 3600)).toBeNull();
    for (const bad of [-5 * 3600, 0, 900, 1799, 168 * 3600 + 1]) {
      expect(at(bad)).toBe("notifications.pushInvalidDeferral");
    }
  });

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

describe("eventsBelowMinimum (w4/m231)", () => {
  const { DeploySucceeded, DeployFailed, ServerFailed, AgentNeedsDecision } =
    PushNotificationEvent;
  const { Routine, Important, Critical } = PushNotificationUrgency;
  const urgencies = publishedPushEvents([
    { event: DeploySucceeded, urgency: Routine, sent: true },
    { event: DeployFailed, urgency: Important, sent: true },
    { event: ServerFailed, urgency: Critical, sent: true },
    { event: AgentNeedsDecision, urgency: Important, sent: false },
  ]);
  const selected = [ServerFailed, DeploySucceeded, DeployFailed];

  it("returns the selected events ranked below the minimum, in panel order", () => {
    expect(eventsBelowMinimum(selected, Routine, urgencies)).toEqual([]);
    expect(eventsBelowMinimum(selected, Important, urgencies)).toEqual([
      DeploySucceeded,
    ]);
    expect(eventsBelowMinimum(selected, Critical, urgencies)).toEqual([
      DeployFailed,
      DeploySucceeded,
    ]);
  });

  it("never flags an unselected event, one with no published urgency, or one nothing sends yet", () => {
    expect(eventsBelowMinimum([DeployFailed], Critical, urgencies)).toEqual([
      DeployFailed,
    ]);
    // Important under a Critical minimum, but the minimum is not what stops it
    // (w4/349).
    expect(
      eventsBelowMinimum([AgentNeedsDecision], Critical, urgencies),
    ).toEqual([]);
    expect(eventsBelowMinimum(selected, Critical, new Map())).toEqual([]);
  });
});

describe("droppedSelections (w4/m231)", () => {
  const { DeploySucceeded, DeployFailed } = PushNotificationEvent;
  const { Routine, Important, Critical } = PushNotificationUrgency;
  const urgencies = publishedPushEvents([
    { event: DeploySucceeded, urgency: Routine, sent: true },
    { event: DeployFailed, urgency: Important, sent: true },
  ]);
  const serviceId = "srv-c185th5c2rvvnhbfiltg";
  const workspace = (
    overrides: PushNotificationSettingsInput["serviceOverrides"] = [],
  ): PushNotificationSettingsInput => ({
    ...validSettings(),
    events: [DeployFailed, DeploySucceeded],
    minimumUrgency: Important,
    serviceOverrides: overrides,
  });

  it("drops workspace selections below the workspace minimum", () => {
    expect(droppedSelections(workspace(), urgencies).workspace).toEqual([
      DeploySucceeded,
    ]);
  });

  it("resolves an override's unset fields from the workspace", () => {
    const services = (override: object) =>
      droppedSelections(workspace([{ serviceId, ...override }]), urgencies)
        .services;
    expect(services({ events: [DeploySucceeded] }).get(serviceId)).toEqual({
      minimum: Important,
      events: [DeploySucceeded],
    });
    expect(services({ minimumUrgency: Critical }).get(serviceId)).toEqual({
      minimum: Critical,
      events: [DeployFailed, DeploySucceeded],
    });
    expect(
      services({ events: [DeploySucceeded], minimumUrgency: Routine }).size,
    ).toBe(0);
    // Changing neither field leaves the workspace line to cover it.
    expect(services({ enabled: true }).size).toBe(0);
  });

  it("drops nothing where push is off, and an override cannot re-enable it", () => {
    expect(
      droppedSelections(
        workspace([{ serviceId, enabled: false, events: [DeploySucceeded] }]),
        urgencies,
      ).services.size,
    ).toBe(0);
    const off = droppedSelections(
      {
        ...workspace([{ serviceId, enabled: true, events: [DeploySucceeded] }]),
        enabled: false,
      },
      urgencies,
    );
    expect(off.workspace).toEqual([]);
    expect(off.services.size).toBe(0);
  });
});
