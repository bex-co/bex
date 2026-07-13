import type { TranslationEntry } from "@/i18n";

const enNotifications: Record<string, TranslationEntry> = {
  "notifications.title": {
    message: "Notifications",
    description: "Settings Notifications section card title",
  },
  "notifications.description": {
    message:
      "Get emailed when a deploy of one of your services succeeds or fails. These are your own preferences — every workspace member sets theirs independently.",
    description: "Settings Notifications section card description",
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
};

export default enNotifications;
