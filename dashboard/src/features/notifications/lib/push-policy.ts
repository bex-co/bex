import {
  PushNotificationEvent,
  PushNotificationUrgency,
  PushNotificationWeekday,
  type PushNotificationSettingsInput,
} from "@/graphql/definitions";

export const pushEvents = [
  PushNotificationEvent.DeployFailed,
  PushNotificationEvent.ServerFailed,
  PushNotificationEvent.CronFailed,
  PushNotificationEvent.DeployStarted,
  PushNotificationEvent.DeploySucceeded,
  PushNotificationEvent.UsageThreshold,
  PushNotificationEvent.AgentNeedsDecision,
  PushNotificationEvent.AgentPrReady,
] as const;

export const pushUrgencies = [
  PushNotificationUrgency.Routine,
  PushNotificationUrgency.Important,
  PushNotificationUrgency.Critical,
] as const;

export const pushWeekdays = [
  PushNotificationWeekday.Monday,
  PushNotificationWeekday.Tuesday,
  PushNotificationWeekday.Wednesday,
  PushNotificationWeekday.Thursday,
  PushNotificationWeekday.Friday,
  PushNotificationWeekday.Saturday,
  PushNotificationWeekday.Sunday,
] as const;

const clockPattern = /^(?:[01]\d|2[0-3]):[0-5]\d$/;
const servicePattern = /^srv-[0-9a-v]{20}$/;

export function validatePushSettings(
  settings: PushNotificationSettingsInput,
): string | null {
  if (
    settings.timeZone.length === 0 ||
    settings.timeZone.length > 64 ||
    settings.timeZone === "Local"
  ) {
    return "notifications.pushInvalidTimezone";
  }
  try {
    new Intl.DateTimeFormat("en", { timeZone: settings.timeZone }).format();
  } catch {
    return "notifications.pushInvalidTimezone";
  }
  if (
    !Number.isInteger(settings.maxDeferralSeconds) ||
    settings.maxDeferralSeconds <= 0 ||
    settings.maxDeferralSeconds > 7 * 24 * 60 * 60
  ) {
    return "notifications.pushInvalidDeferral";
  }
  const knownEvents = new Set<string>(pushEvents);
  if (
    settings.events.length > pushEvents.length ||
    settings.events.some((event) => !knownEvents.has(event))
  ) {
    return "notifications.pushInvalidEvents";
  }
  if (!pushUrgencies.includes(settings.minimumUrgency)) {
    return "notifications.pushInvalidUrgency";
  }
  for (const range of [...settings.workingHours, ...settings.quietHours]) {
    if (
      range.weekdays.length === 0 ||
      range.weekdays.length > 7 ||
      !clockPattern.test(range.start) ||
      !clockPattern.test(range.end) ||
      range.start === range.end
    ) {
      return "notifications.pushInvalidRange";
    }
  }
  if (
    settings.workingHours.length > 32 ||
    settings.quietHours.length > 32 ||
    settings.serviceOverrides.length > 100
  ) {
    return "notifications.pushTooManyRules";
  }
  const serviceIds = new Set<string>();
  for (const override of settings.serviceOverrides) {
    if (
      !servicePattern.test(override.serviceId) ||
      serviceIds.has(override.serviceId)
    ) {
      return "notifications.pushInvalidService";
    }
    serviceIds.add(override.serviceId);
    if (
      override.enabled == null &&
      override.events == null &&
      override.minimumUrgency == null
    ) {
      return "notifications.pushEmptyOverride";
    }
    if (
      override.events &&
      (override.events.length > pushEvents.length ||
        override.events.some((event) => !knownEvents.has(event)))
    ) {
      return "notifications.pushInvalidEvents";
    }
    if (
      override.minimumUrgency != null &&
      !pushUrgencies.includes(override.minimumUrgency)
    ) {
      return "notifications.pushInvalidUrgency";
    }
  }
  return null;
}

export function toggleListValue<T>(values: T[], value: T): T[] {
  return values.includes(value)
    ? values.filter((item) => item !== value)
    : [...values, value];
}
