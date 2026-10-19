import {
  PushNotificationEvent,
  PushNotificationUrgency,
  PushNotificationWeekday,
  type PushNotificationServiceOverrideInput,
  type PushNotificationSettingsInput,
} from "@/graphql/definitions";

export const pushEvents = [
  PushNotificationEvent.DeployFailed,
  PushNotificationEvent.ServerFailed,
  PushNotificationEvent.CronFailed,
  PushNotificationEvent.DeployStarted,
  PushNotificationEvent.DeploySucceeded,
  PushNotificationEvent.ServerAvailable,
  PushNotificationEvent.ServiceSuspended,
  PushNotificationEvent.ServiceResumed,
  PushNotificationEvent.AgentNeedsDecision,
  PushNotificationEvent.AgentPrReady,
  PushNotificationEvent.AgentFailed,
  PushNotificationEvent.PostgresUnavailable,
  PushNotificationEvent.PostgresAvailable,
  PushNotificationEvent.KeyValueUnhealthy,
  PushNotificationEvent.KeyValueAvailable,
  PushNotificationEvent.PostgresBackupFailed,
  PushNotificationEvent.PostgresRestoreFailed,
  PushNotificationEvent.PostgresUpgradeFailed,
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
const knownEvents = new Set<string>(pushEvents);

function hasValidEvents(events: PushNotificationEvent[]): boolean {
  return (
    events.length <= pushEvents.length &&
    events.every((event) => knownEvents.has(event))
  );
}

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
  if (!hasValidEvents(settings.events)) {
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
  return validateServiceOverrides(settings.serviceOverrides);
}

function validateServiceOverrides(
  overrides: PushNotificationServiceOverrideInput[],
): string | null {
  const serviceIds = new Set<string>();
  for (const override of overrides) {
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
    if (override.events && !hasValidEvents(override.events)) {
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

/**
 * One selectable event as bex-api publishes it: the urgency it is delivered
 * at (w4/m231), and whether anything sends it yet (w4/349).
 */
export interface PublishedPushEvent {
  urgency: PushNotificationUrgency;
  sent: boolean;
}

/** Each selectable event as bex-api publishes it. */
export type PushEventUrgencies = ReadonlyMap<
  PushNotificationEvent,
  PublishedPushEvent
>;

/** bex-api's published rows, keyed by event. */
export function publishedPushEvents(
  rows: readonly ({ event: PushNotificationEvent } & PublishedPushEvent)[],
): PushEventUrgencies {
  return new Map(
    rows.map(({ event, urgency, sent }) => [event, { urgency, sent }]),
  );
}

/**
 * The selected events a minimum urgency drops: those whose published urgency
 * ranks below it (w4/m231). bex-api filters by event and then by urgency, so a
 * selection alone does not make an event push. An event with no published
 * urgency, or one nothing sends yet, is never flagged: the minimum is not what
 * keeps it from pushing. Ordered as the panel lists events.
 */
export function eventsBelowMinimum(
  events: readonly PushNotificationEvent[],
  minimum: PushNotificationUrgency,
  urgencies: PushEventUrgencies,
): PushNotificationEvent[] {
  const floor = pushUrgencies.indexOf(minimum);
  return pushEvents.filter((event) => {
    const published = urgencies.get(event);
    return (
      events.includes(event) &&
      published?.sent === true &&
      pushUrgencies.indexOf(published.urgency) < floor
    );
  });
}

/** The selections bex-api will not push, workspace-wide and per service. */
export interface DroppedSelections {
  workspace: PushNotificationEvent[];
  /** By overridden service id: the minimum that applies there and its drops. */
  services: Map<
    string,
    { minimum: PushNotificationUrgency; events: PushNotificationEvent[] }
  >;
}

/**
 * Everything the urgency filter drops under `settings`, in bex-api's own
 * precedence: push off workspace-wide sends nothing (overrides cannot re-enable
 * it), an override that turns push off sends nothing for its service, and an
 * override resolves each unset field from the workspace. An override that sets
 * neither its own events nor its own minimum adds nothing — the workspace line
 * already covers it.
 */
export function droppedSelections(
  settings: PushNotificationSettingsInput,
  urgencies: PushEventUrgencies,
): DroppedSelections {
  const dropped: DroppedSelections = { workspace: [], services: new Map() };
  if (!settings.enabled) return dropped;
  dropped.workspace = eventsBelowMinimum(
    settings.events,
    settings.minimumUrgency,
    urgencies,
  );
  for (const override of settings.serviceOverrides) {
    if (override.enabled === false) continue;
    if (override.events == null && override.minimumUrgency == null) continue;
    const minimum = override.minimumUrgency ?? settings.minimumUrgency;
    const events = eventsBelowMinimum(
      override.events ?? settings.events,
      minimum,
      urgencies,
    );
    if (events.length > 0)
      dropped.services.set(override.serviceId, { minimum, events });
  }
  return dropped;
}

export function toggleListValue<T>(values: T[], value: T): T[] {
  return values.includes(value)
    ? values.filter((item) => item !== value)
    : [...values, value];
}
