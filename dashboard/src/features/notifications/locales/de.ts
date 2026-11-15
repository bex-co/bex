import type { TranslationEntry } from "@/i18n";

const deNotifications: Record<string, TranslationEntry> = {
  "notifications.title": {
    message: "Benachrichtigungen",
    description: "Settings Notifications section card title",
  },
  "notifications.description": {
    message:
      "Lass dir eine E-Mail schicken, wenn ein Deployment eines deiner Services startet, erfolgreich ist oder fehlschlägt und wenn zur Laufzeit etwas fehlschlägt. Das sind deine eigenen Einstellungen — jedes Workspace-Mitglied legt sie unabhängig fest.",
    description: "Settings Notifications section card description",
  },
  "notifications.deployStarted": {
    message: "Deployment gestartet",
    description: "Toggle label",
  },
  "notifications.deployStartedHint": {
    message:
      "Schick mir eine E-Mail, wenn ein Deployment startet. Wird nur für Services gesendet, die alle Benachrichtigungen senden.",
    description: "Toggle hint",
  },
  "notifications.deploySucceeded": {
    message: "Deployment erfolgreich",
    description: "Toggle label",
  },
  "notifications.deploySucceededHint": {
    message:
      "Schick mir eine E-Mail, wenn ein Deployment live geht. Wird nur für Services gesendet, die alle Benachrichtigungen senden.",
    description: "Toggle hint",
  },
  "notifications.deployFailed": {
    message: "Fehler",
    description: "Toggle label",
  },
  "notifications.deployFailedHint": {
    message:
      "Schick mir eine E-Mail, wenn ein Deployment fehlschlägt, ein Service fehlerhaft wird, ein Cron-Lauf fehlschlägt, ein Image-Pull fehlschlägt oder eine Blueprint-Synchronisierung fehlschlägt.",
    description: "Toggle hint",
  },
  "notifications.errorTitle": {
    message: "Benachrichtigungseinstellungen konnten nicht geladen werden",
    description: "Generic error title",
  },
  "notifications.errorBody": {
    message: "Etwas ist schiefgelaufen. Bitte versuche es erneut.",
    description: "Generic error body",
  },
  "notifications.updateError": {
    message: "Deine Einstellung konnte nicht gespeichert werden",
    description: "Toast on a failed update",
  },
  "notifications.pushTitle": {
    message: "Mobile Push",
    description: "Push settings card title",
  },
  "notifications.bexExtension": {
    message: "bex-Erweiterung",
    description: "Badge distinguishing native push from Render parity",
  },
  "notifications.pushDescription": {
    message:
      "Steuere bex-Mobilbenachrichtigungen, Dringlichkeit, Zustellzeiten und Ausnahmen für einzelne Services. Die E-Mail-Einstellungen oben bleiben unverändert.",
    description: "Push settings description",
  },
  "notifications.pushEnabled": {
    message: "Mobile Push aktivieren",
    description: "Push enabled toggle",
  },
  "notifications.pushEnabledHint": {
    message: "Sende Warnungen an deine registrierten bex-Mobilgeräte.",
    description: "Push enabled hint",
  },
  "notifications.pushUnavailable": {
    message: "Push-Zustellung ist auf diesem bex-Server nicht konfiguriert",
    description: "Disabled server status",
  },
  "notifications.pushUnavailableHint": {
    message:
      "Du kannst die Richtlinie schon jetzt vorbereiten und speichern, aber registrierte Geräte erhalten erst Benachrichtigungen, wenn ein Operator den Push-Anbieter konfiguriert.",
    description: "Disabled server status hint",
  },
  "notifications.webPushTitle": {
    message: "Browser-Benachrichtigungen",
    description: "Web push settings card title",
  },
  "notifications.webPushDescription": {
    message:
      "Erhalte dieselben Warnungen in diesem Browser. Ereignisfilter und Ruhezeiten sind die, die du unter Mobile Push festlegst — es gibt keine zweite Richtlinie.",
    description: "Web push settings description",
  },
  "notifications.webPushUnsupported": {
    message: "Dieser Browser unterstützt kein Web-Push.",
    description: "Unsupported browser",
  },
  "notifications.webPushUnconfigured": {
    message: "Dieser bex-Server hat Browser-Push nicht aktiviert.",
    description: "Server VAPID unset",
  },
  "notifications.webPushDenied": {
    message:
      "Benachrichtigungen sind für diese Website blockiert. Erlaube sie in den Browser-Einstellungen und versuche es dann erneut.",
    description: "Permission denied",
  },
  "notifications.webPushError": {
    message:
      "Browser-Benachrichtigungen konnten nicht aktualisiert werden. Versuche es erneut.",
    description: "Subscribe error",
  },
  "notifications.webPushOn": {
    message: "Dieser Browser erhält bex-Warnungen.",
    description: "Subscribed state",
  },
  "notifications.webPushOff": {
    message: "Dieser Browser ist nicht abonniert.",
    description: "Unsubscribed state",
  },
  "notifications.webPushEnable": {
    message: "In diesem Browser aktivieren",
    description: "Subscribe button",
  },
  "notifications.webPushDisable": {
    message: "In diesem Browser deaktivieren",
    description: "Unsubscribe button",
  },
  "notifications.pushEvents": {
    message: "Ereignisfilter",
    description: "Push event section",
  },
  "notifications.pushEventsHint": {
    message:
      "Nur ausgewählte Ereignistypen ab der Mindest-Dringlichkeit können eine Push-Benachrichtigung auslösen.",
    description: "Push events hint",
  },
  "notifications.pushEventNotSentYet": {
    message:
      "Noch nicht gesendet: Dieses Ereignis wird derzeit von nichts gesendet.",
    description:
      "Under a push event that no part of bex emits yet (it is reserved for an upcoming feature); selecting it is kept but has no effect",
  },
  "notifications.pushEventNotSentYetShort": {
    message: "Noch nicht gesendet",
    description:
      "Compact form of the not-sent-yet note, under a push event in a per-service override's event grid",
  },
  "notifications.pushEventFiltered": {
    message: "Kein Push, solange die Mindest-Dringlichkeit {minimum} ist.",
    description:
      "Under a selected push event whose urgency is below the minimum ({minimum} = urgency name, e.g. 'Important')",
  },
  "notifications.pushFilteredTitle": {
    message: "Einige ausgewählte Ereignisse lösen keinen Push aus",
    description:
      "Heading of the save-area summary listing selected push events the minimum urgency drops",
  },
  "notifications.pushFilteredEvent": {
    message:
      "{event} ist {urgency} und löst keinen Push aus, solange die Mindest-Dringlichkeit {minimum} ist.",
    description:
      "One summary line ({event} = event name, {urgency} = its urgency, {minimum} = the minimum urgency)",
  },
  "notifications.pushFilteredServiceEvent": {
    message:
      "{service}: {event} ist {urgency} und löst keinen Push aus, solange die Mindest-Dringlichkeit dieses Service {minimum} ist.",
    description:
      "One summary line for a service override ({service} = service name; others as pushFilteredEvent)",
  },
  "notifications.pushOverrideFiltered_one": {
    message:
      "{count} ausgewähltes Ereignis löst für diesen Service keinen Push aus, solange seine Mindest-Dringlichkeit {minimum} ist.",
    description:
      "Under a service override whose own policy drops selected events ({minimum} = urgency name)",
  },
  "notifications.pushOverrideFiltered_other": {
    message:
      "{count} ausgewählte Ereignisse lösen für diesen Service keinen Push aus, solange seine Mindest-Dringlichkeit {minimum} ist.",
    description:
      "Under a service override whose own policy drops selected events ({minimum} = urgency name)",
  },
  "notifications.pushEventDeployFailed": {
    message: "Deployment fehlgeschlagen",
    description: "Push event",
  },
  "notifications.pushEventServerFailed": {
    message: "Service abgestürzt",
    description: "Push event",
  },
  "notifications.pushEventCronFailed": {
    message: "Cron-Lauf fehlgeschlagen",
    description: "Push event",
  },
  "notifications.pushEventDeployStarted": {
    message: "Deployment gestartet",
    description: "Push event",
  },
  "notifications.pushEventDeploySucceeded": {
    message: "Deployment erfolgreich",
    description: "Push event",
  },
  "notifications.pushEventServerAvailable": {
    message: "Service wiederhergestellt",
    description: "Push event",
  },
  "notifications.pushEventServiceSuspended": {
    message: "Service pausiert",
    description: "Push event",
  },
  "notifications.pushEventServiceResumed": {
    message: "Service fortgesetzt",
    description: "Push event",
  },
  "notifications.pushEventAgentNeedsDecision": {
    message: "Agent braucht eine Entscheidung",
    description: "Push event",
  },
  "notifications.pushEventAgentPrReady": {
    message: "Agent-PR bereit",
    description: "Push event",
  },
  "notifications.pushEventAgentFailed": {
    message: "Agent-Sitzung fehlgeschlagen",
    description: "Push event",
  },
  "notifications.pushEventAgentAutomationPaused": {
    message: "Automatisierung pausiert",
    description: "Push event: the platform paused an agent automation",
  },
  "notifications.pushEventPostgresUnavailable": {
    message: "Datenbank nicht verfügbar",
    description: "Push event",
  },
  "notifications.pushEventPostgresAvailable": {
    message: "Datenbank wiederhergestellt",
    description: "Push event",
  },
  "notifications.pushEventKeyValueUnhealthy": {
    message: "Key Value fehlerhaft",
    description: "Push event",
  },
  "notifications.pushEventKeyValueAvailable": {
    message: "Key Value wiederhergestellt",
    description: "Push event",
  },
  "notifications.pushEventPostgresBackupFailed": {
    message: "Datenbank-Backup fehlgeschlagen",
    description: "Push event",
  },
  "notifications.pushEventPostgresRestoreFailed": {
    message: "Datenbank-Wiederherstellung fehlgeschlagen",
    description: "Push event",
  },
  "notifications.pushEventPostgresUpgradeFailed": {
    message: "Datenbank-Upgrade fehlgeschlagen",
    description: "Push event",
  },
  "notifications.pushMinimumUrgency": {
    message: "Mindest-Dringlichkeit",
    description: "Urgency field",
  },
  "notifications.pushUrgencyRoutine": {
    message: "Routine",
    description: "Urgency option",
  },
  "notifications.pushUrgencyImportant": {
    message: "Wichtig",
    description: "Urgency option",
  },
  "notifications.pushUrgencyCritical": {
    message: "Kritisch",
    description: "Urgency option",
  },
  "notifications.pushTimezone": {
    message: "IANA-Zeitzone",
    description: "Timezone field",
  },
  "notifications.pushTimezonePlaceholder": {
    message: "America/Los_Angeles",
    description: "Timezone placeholder",
  },
  "notifications.pushMaxDeferral": {
    message: "Max. Verzögerung (Stunden)",
    description: "Deferral field",
  },
  "notifications.pushWorkingHours": {
    message: "Arbeitszeiten",
    description: "Working hours heading",
  },
  "notifications.pushWorkingHoursHint": {
    message:
      "Wenn festgelegt, warten nicht kritische Warnungen bis zu einem dieser Zeiträume.",
    description: "Working hours hint",
  },
  "notifications.pushQuietHours": {
    message: "Ruhezeiten",
    description: "Quiet hours heading",
  },
  "notifications.pushQuietHoursHint": {
    message:
      "Nicht kritische Warnungen warten, solange ein Ruhezeitraum aktiv ist.",
    description: "Quiet hours hint",
  },
  "notifications.weekdayMonday": {
    message: "Mo",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayTuesday": {
    message: "Di",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayWednesday": {
    message: "Mi",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayThursday": {
    message: "Do",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayFriday": {
    message: "Fr",
    description: "Weekday abbreviation",
  },
  "notifications.weekdaySaturday": {
    message: "Sa",
    description: "Weekday abbreviation",
  },
  "notifications.weekdaySunday": {
    message: "So",
    description: "Weekday abbreviation",
  },
  "notifications.pushRangeStart": {
    message: "Beginn des Zeitraums",
    description: "Accessible time input label",
  },
  "notifications.pushRangeEnd": {
    message: "Ende des Zeitraums",
    description: "Accessible time input label",
  },
  "notifications.pushRemoveRange": {
    message: "Zeitraum entfernen",
    description: "Accessible remove button label",
  },
  "notifications.pushAddRange": {
    message: "Zeitraum hinzufügen",
    description: "Add range button",
  },
  "notifications.pushOverrides": {
    message: "Ausnahmen pro Service",
    description: "Overrides heading",
  },
  "notifications.pushOverridesHint": {
    message:
      "Nicht angegebene Felder übernehmen die obigen Einstellungen; eine ausdrücklich leere Ereignisliste schaltet diesen Service stumm.",
    description: "Override semantics hint",
  },
  "notifications.pushRemoveOverride": {
    message: "Service-Ausnahme entfernen",
    description: "Accessible remove override label",
  },
  "notifications.pushOverrideEnabled": {
    message: "Zustellung",
    description: "Override enabled field",
  },
  "notifications.pushInherit": {
    message: "Übernehmen",
    description: "Override option",
  },
  "notifications.pushOn": {
    message: "An",
    description: "Override option",
  },
  "notifications.pushOff": {
    message: "Aus",
    description: "Override option",
  },
  "notifications.pushExactEvents": {
    message: "Mit einer eigenen Ereignisliste überschreiben",
    description: "Exact event filter toggle",
  },
  "notifications.pushAddOverride": {
    message: "Service-Ausnahme hinzufügen…",
    description: "Add override selector",
  },
  "notifications.pushSave": {
    message: "Push-Einstellungen speichern",
    description: "Save button",
  },
  "notifications.pushUnsaved": {
    message: "Nicht gespeicherte Änderungen",
    description:
      "Hint beside Save push settings while the push form differs from the saved settings (w4/435)",
  },
  "notifications.pushDiscardTitle": {
    message: "Änderungen an den Push-Einstellungen verwerfen?",
    description:
      "Navigation guard title when the push form has unsaved changes (w4/435)",
  },
  "notifications.pushDiscardBody": {
    message:
      "Diese Änderungen wurden nicht gespeichert und gehen verloren, wenn du die Seite verlässt.",
    description: "Navigation guard explanation for unsaved push settings",
  },
  "notifications.pushKeepEditing": {
    message: "Weiter bearbeiten",
    description: "Cancel the navigation and stay on the push form",
  },
  "notifications.pushDiscard": {
    message: "Verwerfen und verlassen",
    description: "Confirm leaving and dropping unsaved push settings",
  },
  "notifications.pushSaved": {
    message: "Push-Einstellungen gespeichert",
    description: "Success toast",
  },
  "notifications.pushUpdateError": {
    message: "Push-Einstellungen konnten nicht gespeichert werden",
    description: "Failure toast",
  },
  "notifications.pushErrorTitle": {
    message: "Push-Einstellungen konnten nicht geladen werden",
    description: "Push load error title",
  },
  "notifications.pushInvalidTimezone": {
    message: "Gib eine gültige IANA-Zeitzone ein, z. B. America/Los_Angeles.",
    description: "Timezone validation",
  },
  "notifications.pushInvalidDeferral": {
    message:
      "Die maximale Verzögerung muss zwischen 0,5 und 168 Stunden liegen.",
    description: "Deferral validation",
  },
  "notifications.pushInvalidEvents": {
    message: "Der Ereignisfilter enthält einen nicht unterstützten Wert.",
    description: "Event validation",
  },
  "notifications.pushInvalidUrgency": {
    message: "Wähle eine gültige Dringlichkeit.",
    description: "Urgency validation",
  },
  "notifications.pushInvalidRange": {
    message:
      "Jeder Zeitraum braucht Wochentage, gültige Start- und Endzeiten und unterschiedliche Endpunkte.",
    description: "Range validation",
  },
  "notifications.pushTooManyRules": {
    message: "Es gibt zu viele Zeitplan-Zeiträume oder Service-Ausnahmen.",
    description: "Rule count validation",
  },
  "notifications.pushInvalidService": {
    message:
      "Service-Ausnahmen müssen eindeutig sein und auf einen gültigen Service verweisen.",
    description: "Service validation",
  },
  "notifications.pushEmptyOverride": {
    message:
      "Jede Service-Ausnahme muss Zustellung, Dringlichkeit oder Ereignisse ändern.",
    description: "Empty override validation",
  },
  "notifications.workspaceTitle": {
    message: "Benachrichtigungen",
    description:
      "Workspace Settings → Notifications card title (the workspace notification policy)",
  },
  "notifications.workspaceDescription": {
    message:
      "Standardwerte für jeden Service. Ein Service kann sie in seinen Einstellungen überschreiben.",
    description: "Workspace Settings → Notifications card description",
  },
  "notifications.workspaceReadOnly": {
    message: "Nur Workspace-Admins können diese Standardwerte ändern.",
    description: "Workspace notification policy: read-only note for non-admins",
  },
  "notifications.workspaceEmailLabel": {
    message: "E-Mail-Benachrichtigungen",
    description: "Workspace notification policy: email destination switch",
  },
  "notifications.workspaceEmailHint": {
    message:
      "Bei „Aus“ werden keine Benachrichtigungs-E-Mails gesendet, unabhängig von der Einstellung eines Service.",
    description: "Workspace notification policy: email destination hint",
  },
  "notifications.workspaceLevelLabel": {
    message: "Standard-Benachrichtigungen für Services",
    description: "Workspace notification policy: level radio group label",
  },
  "notifications.workspaceLevelHint": {
    message: "Was Services mit dem Workspace-Standard senden.",
    description: "Workspace notification policy: level radio group hint",
  },
  "notifications.levelAll": {
    message: "Alle Benachrichtigungen",
    description: "Notification level: every notification",
  },
  "notifications.levelAllHint": {
    message:
      "Fehler sowie Start und Live-Gang von Deployments und Wiederherstellungen.",
    description: "Notification level hint: all",
  },
  "notifications.levelFailure": {
    message: "Nur Fehlerbenachrichtigungen",
    description: "Notification level: failures only (Render's default)",
  },
  "notifications.levelFailureHint": {
    message:
      "Fehlgeschlagene Deployments, fehlerhafte Services, fehlgeschlagene Cron-Läufe, volle Disks.",
    description: "Notification level hint: failure",
  },
  "notifications.levelNone": {
    message: "Keine",
    description: "Notification level: nothing",
  },
  "notifications.levelNoneHint": {
    message: "Für diese Services keine Benachrichtigungen senden.",
    description: "Notification level hint: none",
  },
  "notifications.workspaceSave": {
    message: "Speichern",
    description: "Workspace notification policy: save button",
  },
  "notifications.workspaceSaved": {
    message: "Benachrichtigungs-Standardwerte gespeichert",
    description: "Workspace notification policy: success toast",
  },
  "notifications.workspaceSaveFailed": {
    message:
      "Die Benachrichtigungs-Standardwerte konnten nicht gespeichert werden. Versuche es erneut.",
    description: "Workspace notification policy: save error",
  },
  "notifications.workspaceLoadFailed": {
    message:
      "Die Benachrichtigungs-Standardwerte des Workspace konnten nicht geladen werden.",
    description: "Workspace notification policy: load error",
  },
  "notifications.workspaceRetry": {
    message: "Erneut versuchen",
    description: "Workspace notification policy: retry button",
  },
  "notifications.overridesTitle": {
    message: "Benachrichtigungs-Ausnahmen",
    description:
      "Workspace notifications: overrides list title (Render's label)",
  },
  "notifications.overridesDescription": {
    message:
      "Services, deren Benachrichtigungseinstellung vom Workspace-Standard abweicht.",
    description: "Workspace notifications: overrides list description",
  },
  "notifications.overridesEmpty": {
    message: "Kein Service überschreibt den Workspace-Standard.",
    description: "Workspace notifications: no overrides",
  },
  "notifications.overridesError": {
    message:
      "Die Services, die den Standard überschreiben, konnten nicht geladen werden.",
    description: "Workspace notifications: overrides failed to load",
  },
};

export default deNotifications;
