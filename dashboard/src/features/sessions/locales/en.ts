import type { TranslationEntry } from "@/i18n";

const en: Record<string, TranslationEntry> = {
  "activeSessions.title": {
    message: "Active sessions",
    description: "Active Sessions settings card title",
  },
  "activeSessions.description": {
    message: "Browsers and devices currently signed in as you.",
    description: "Active Sessions settings card description",
  },
  "activeSessions.signOutOthers": {
    message: "Sign out other sessions",
    description: "Active Sessions sign-out-others button label",
  },
  "activeSessions.signOutOthersConfirmTitle": {
    message: "Sign out every other session?",
    description: "Active Sessions sign-out-others confirmation dialog title",
  },
  "activeSessions.signOutOthersConfirmBody": {
    message:
      "This immediately signs out every browser and device except the one you're using right now.",
    description: "Active Sessions sign-out-others confirmation dialog body",
  },
  "activeSessions.signOutOthersSuccess": {
    message: "Signed out other sessions",
    description: "Active Sessions sign-out-others success toast",
  },
  "activeSessions.signOutOthersError": {
    message: "Couldn't sign out other sessions",
    description: "Active Sessions sign-out-others failure toast",
  },
  "activeSessions.colDevice": {
    message: "Device",
    description: "Active Sessions table column: device/browser",
  },
  "activeSessions.colLocation": {
    message: "Location",
    description: "Active Sessions table column: location or IP",
  },
  "activeSessions.colLastActive": {
    message: "Last active",
    description: "Active Sessions table column: last authenticated",
  },
  "activeSessions.current": {
    message: "This device",
    description: "Active Sessions badge marking the current session's row",
  },
  "activeSessions.unknownDevice": {
    message: "Unknown device",
    description: "Active Sessions fallback when no user agent is recorded",
  },
  "activeSessions.emptyTitle": {
    message: "No active sessions",
    description: "Active Sessions empty state title",
  },
  "activeSessions.emptyBody": {
    message: "We couldn't find any active sessions for your account.",
    description: "Active Sessions empty state body",
  },
  "activeSessions.errorTitle": {
    message: "Couldn't load active sessions",
    description: "Active Sessions generic error state title",
  },
  "activeSessions.errorBody": {
    message: "Something went wrong. Try again in a moment.",
    description: "Active Sessions generic error state body",
  },
  "activeSessions.revoke": {
    message: "Sign out",
    description: "Active Sessions row revoke button label",
  },
  "activeSessions.revokeConfirmTitle": {
    message: "Sign out this session?",
    description: "Active Sessions revoke confirmation dialog title",
  },
  "activeSessions.revokeConfirmBody": {
    message: "This immediately signs that browser or device out.",
    description: "Active Sessions revoke confirmation dialog body",
  },
  "activeSessions.revokeCancel": {
    message: "Cancel",
    description: "Active Sessions confirmation dialog cancel button",
  },
  "activeSessions.revokeSuccess": {
    message: "Session signed out",
    description: "Active Sessions revoke success toast",
  },
  "activeSessions.revokeError": {
    message: "Couldn't sign out that session",
    description: "Active Sessions revoke failure toast",
  },
};

export default en;
