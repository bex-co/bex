import type { TranslationEntry } from "@/i18n";

const enNotifications: Record<string, TranslationEntry> = {
  "notifications.title": {
    message: "Notifications",
    description: "Settings Notifications section card title",
  },
  "notifications.description": {
    message:
      "Get emailed when a deploy of one of your services starts, succeeds, or fails. These are your own preferences — every workspace member sets theirs independently.",
    description: "Settings Notifications section card description",
  },
  "notifications.deployStarted": {
    message: "Deploy started",
    description: "Toggle label",
  },
  "notifications.deployStartedHint": {
    message: "Email me when a deploy starts.",
    description: "Toggle hint",
  },
  "notifications.deploySucceeded": {
    message: "Deploy succeeded",
    description: "Toggle label",
  },
  "notifications.deploySucceededHint": {
    message: "Email me when a deploy goes live.",
    description: "Toggle hint",
  },
  "notifications.deployFailed": {
    message: "Deploy failed",
    description: "Toggle label",
  },
  "notifications.deployFailedHint": {
    message: "Email me when a deploy fails.",
    description: "Toggle hint",
  },
  "notifications.errorTitle": {
    message: "Couldn't load notification settings",
    description: "Generic error title",
  },
  "notifications.errorBody": {
    message: "Something went wrong. Please try again.",
    description: "Generic error body",
  },
  "notifications.updateError": {
    message: "Couldn't save your preference",
    description: "Toast on a failed update",
  },
  "notifications.pushTitle": {
    message: "Mobile push",
    description: "Push settings card title",
  },
  "notifications.bexExtension": {
    message: "bex extension",
    description: "Badge distinguishing native push from Render parity",
  },
  "notifications.pushDescription": {
    message:
      "Control bex mobile alerts, urgency, delivery hours, and exact service exceptions. Email preferences above are unchanged.",
    description: "Push settings description",
  },
  "notifications.pushEnabled": {
    message: "Enable mobile push",
    description: "Push enabled toggle",
  },
  "notifications.pushEnabledHint": {
    message: "Send alerts to your registered bex mobile devices.",
    description: "Push enabled hint",
  },
  "notifications.pushUnavailable": {
    message: "Push delivery is not configured on this bex server",
    description: "Disabled server status",
  },
  "notifications.pushUnavailableHint": {
    message:
      "You can prepare and save policy now, but registered devices cannot receive notifications until an operator configures the push provider.",
    description: "Disabled server status hint",
  },
  "notifications.pushEvents": {
    message: "Event filters",
    description: "Push event section",
  },
  "notifications.pushEventsHint": {
    message: "Only selected event types can produce a push notification.",
    description: "Push events hint",
  },
  "notifications.pushEventDeployFailed": {
    message: "Deploy failed",
    description: "Push event",
  },
  "notifications.pushEventServerFailed": {
    message: "Service crashed",
    description: "Push event",
  },
  "notifications.pushEventCronFailed": {
    message: "Cron run failed",
    description: "Push event",
  },
  "notifications.pushEventDeployStarted": {
    message: "Deploy started",
    description: "Push event",
  },
  "notifications.pushEventDeploySucceeded": {
    message: "Deploy succeeded",
    description: "Push event",
  },
  "notifications.pushEventUsageThreshold": {
    message: "Usage threshold",
    description: "Push event",
  },
  "notifications.pushEventAgentNeedsDecision": {
    message: "Agent needs a decision",
    description: "Push event",
  },
  "notifications.pushEventAgentPrReady": {
    message: "Agent PR ready",
    description: "Push event",
  },
  "notifications.pushMinimumUrgency": {
    message: "Minimum urgency",
    description: "Urgency field",
  },
  "notifications.pushUrgencyRoutine": {
    message: "Routine",
    description: "Urgency option",
  },
  "notifications.pushUrgencyImportant": {
    message: "Important",
    description: "Urgency option",
  },
  "notifications.pushUrgencyCritical": {
    message: "Critical",
    description: "Urgency option",
  },
  "notifications.pushTimezone": {
    message: "IANA timezone",
    description: "Timezone field",
  },
  "notifications.pushTimezonePlaceholder": {
    message: "America/Los_Angeles",
    description: "Timezone placeholder",
  },
  "notifications.pushMaxDeferral": {
    message: "Max deferral (hours)",
    description: "Deferral field",
  },
  "notifications.pushWorkingHours": {
    message: "Working hours",
    description: "Working hours heading",
  },
  "notifications.pushWorkingHoursHint": {
    message: "When set, non-critical alerts wait until one of these ranges.",
    description: "Working hours hint",
  },
  "notifications.pushQuietHours": {
    message: "Quiet hours",
    description: "Quiet hours heading",
  },
  "notifications.pushQuietHoursHint": {
    message: "Non-critical alerts wait while a quiet range is active.",
    description: "Quiet hours hint",
  },
  "notifications.weekdayMonday": {
    message: "Mon",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayTuesday": {
    message: "Tue",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayWednesday": {
    message: "Wed",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayThursday": {
    message: "Thu",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayFriday": {
    message: "Fri",
    description: "Weekday abbreviation",
  },
  "notifications.weekdaySaturday": {
    message: "Sat",
    description: "Weekday abbreviation",
  },
  "notifications.weekdaySunday": {
    message: "Sun",
    description: "Weekday abbreviation",
  },
  "notifications.pushRangeStart": {
    message: "Range start",
    description: "Accessible time input label",
  },
  "notifications.pushRangeEnd": {
    message: "Range end",
    description: "Accessible time input label",
  },
  "notifications.pushRemoveRange": {
    message: "Remove time range",
    description: "Accessible remove button label",
  },
  "notifications.pushAddRange": {
    message: "Add time range",
    description: "Add range button",
  },
  "notifications.pushOverrides": {
    message: "Per-service overrides",
    description: "Overrides heading",
  },
  "notifications.pushOverridesHint": {
    message:
      "Omitted fields inherit the settings above; an exact empty event list mutes that service.",
    description: "Override semantics hint",
  },
  "notifications.pushRemoveOverride": {
    message: "Remove service override",
    description: "Accessible remove override label",
  },
  "notifications.pushOverrideEnabled": {
    message: "Delivery",
    description: "Override enabled field",
  },
  "notifications.pushInherit": {
    message: "Inherit",
    description: "Override option",
  },
  "notifications.pushOn": { message: "On", description: "Override option" },
  "notifications.pushOff": { message: "Off", description: "Override option" },
  "notifications.pushExactEvents": {
    message: "Override with an exact event list",
    description: "Exact event filter toggle",
  },
  "notifications.pushAddOverride": {
    message: "Add a service override…",
    description: "Add override selector",
  },
  "notifications.pushSave": {
    message: "Save push settings",
    description: "Save button",
  },
  "notifications.pushSaved": {
    message: "Push settings saved",
    description: "Success toast",
  },
  "notifications.pushUpdateError": {
    message: "Couldn't save push settings",
    description: "Failure toast",
  },
  "notifications.pushErrorTitle": {
    message: "Couldn't load push settings",
    description: "Push load error title",
  },
  "notifications.pushInvalidTimezone": {
    message: "Enter a valid IANA timezone such as America/Los_Angeles.",
    description: "Timezone validation",
  },
  "notifications.pushInvalidDeferral": {
    message: "Maximum deferral must be between 1 second and 168 hours.",
    description: "Deferral validation",
  },
  "notifications.pushInvalidEvents": {
    message: "The event filter contains an unsupported value.",
    description: "Event validation",
  },
  "notifications.pushInvalidUrgency": {
    message: "Select a valid urgency.",
    description: "Urgency validation",
  },
  "notifications.pushInvalidRange": {
    message:
      "Every time range needs weekdays, valid start/end times, and different endpoints.",
    description: "Range validation",
  },
  "notifications.pushTooManyRules": {
    message: "There are too many schedule ranges or service overrides.",
    description: "Rule count validation",
  },
  "notifications.pushInvalidService": {
    message: "Service overrides must be unique and reference a valid service.",
    description: "Service validation",
  },
  "notifications.pushEmptyOverride": {
    message: "Each service override must change delivery, urgency, or events.",
    description: "Empty override validation",
  },
};

export default enNotifications;
