import type { TranslationEntry } from "@/i18n";

const de: Record<string, TranslationEntry> = {
  "activeSessions.title": {
    message: "Aktive Sitzungen",
    description: "Active Sessions settings card title",
  },
  "activeSessions.description": {
    message: "Browser und Geräte, die aktuell als du angemeldet sind.",
    description: "Active Sessions settings card description",
  },
  "activeSessions.signOutOthers": {
    message: "Andere Sitzungen abmelden",
    description: "Active Sessions sign-out-others button label",
  },
  "activeSessions.signOutOthersConfirmTitle": {
    message: "Alle anderen Sitzungen abmelden?",
    description: "Active Sessions sign-out-others confirmation dialog title",
  },
  "activeSessions.signOutOthersConfirmBody": {
    message:
      "Dadurch werden sofort alle Browser und Geräte abgemeldet, außer dem, das du gerade verwendest.",
    description: "Active Sessions sign-out-others confirmation dialog body",
  },
  "activeSessions.signOutOthersSuccess": {
    message: "Andere Sitzungen abgemeldet",
    description: "Active Sessions sign-out-others success toast",
  },
  "activeSessions.signOutOthersError": {
    message: "Andere Sitzungen konnten nicht abgemeldet werden",
    description: "Active Sessions sign-out-others failure toast",
  },
  "activeSessions.colDevice": {
    message: "Gerät",
    description: "Active Sessions table column: device/browser",
  },
  "activeSessions.colLocation": {
    message: "Standort",
    description: "Active Sessions table column: location or IP",
  },
  "activeSessions.colSignedIn": {
    message: "Angemeldet",
    description:
      "Active Sessions table column: when the session was authenticated (Kratos has no last-activity time)",
  },
  "activeSessions.current": {
    message: "Dieses Gerät",
    description: "Active Sessions badge marking the current session's row",
  },
  "activeSessions.unknownDevice": {
    message: "Unbekanntes Gerät",
    description: "Active Sessions fallback when no user agent is recorded",
  },
  "activeSessions.emptyTitle": {
    message: "Keine aktiven Sitzungen",
    description: "Active Sessions empty state title",
  },
  "activeSessions.emptyBody": {
    message: "Für dein Konto wurden keine aktiven Sitzungen gefunden.",
    description: "Active Sessions empty state body",
  },
  "activeSessions.errorTitle": {
    message: "Aktive Sitzungen konnten nicht geladen werden",
    description: "Active Sessions generic error state title",
  },
  "activeSessions.errorBody": {
    message: "Etwas ist schiefgelaufen. Versuche es gleich noch einmal.",
    description: "Active Sessions generic error state body",
  },
  "activeSessions.revoke": {
    message: "Abmelden",
    description: "Active Sessions row revoke confirmation button",
  },
  "activeSessions.revokeNamed": {
    message: "{device}, {location}, angemeldet {signedIn} abmelden",
    description:
      "Active Sessions row revoke button aria-label identifying the session",
  },
  "activeSessions.revokeConfirmTitle": {
    message: "Diese Sitzung abmelden?",
    description: "Active Sessions revoke confirmation dialog title",
  },
  "activeSessions.revokeConfirmBody": {
    message: "Dadurch wird dieser Browser bzw. dieses Gerät sofort abgemeldet.",
    description: "Active Sessions revoke confirmation dialog body",
  },
  "activeSessions.revokeCancel": {
    message: "Abbrechen",
    description: "Active Sessions confirmation dialog cancel button",
  },
  "activeSessions.revokeSuccess": {
    message: "Sitzung abgemeldet",
    description: "Active Sessions revoke success toast",
  },
  "activeSessions.revokeError": {
    message: "Diese Sitzung konnte nicht abgemeldet werden",
    description: "Active Sessions revoke failure toast",
  },
};

export default de;
