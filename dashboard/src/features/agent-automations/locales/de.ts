import type { TranslationEntry } from "@/i18n";

const deAgentAutomations: Record<string, TranslationEntry> = {
  "agentAutomations.pageTitle": {
    message: "Automatisierungen",
    description:
      "Page title and sidebar label of the scheduled agent automations page",
  },
  "agentAutomations.detailTitle": {
    message: "Automatisierung",
    description: "Browser tab title of one automation's page",
  },
  "agentAutomations.pageDescription": {
    message:
      "Starte nach Zeitplan eine Agent-Sitzung. Jeder Lauf verwendet die Aufgabe und die Einstellungen, die du hier speicherst.",
    description: "Automations list page description",
  },
  "agentAutomations.create": {
    message: "Automatisierung erstellen",
    description:
      "Create automation button on the list page and in the empty state",
  },
  "agentAutomations.createTitle": {
    message: "Automatisierung erstellen",
    description: "Heading of the create form",
  },
  "agentAutomations.editTitle": {
    message: "Automatisierung bearbeiten",
    description: "Heading of the edit form",
  },
  "agentAutomations.createSubmit": {
    message: "Automatisierung erstellen",
    description: "Create form submit button",
  },
  "agentAutomations.editSubmit": {
    message: "Änderungen speichern",
    description: "Edit form submit button",
  },
  "agentAutomations.createDenied": {
    message:
      "Nur Mitglieder mit der Rolle Entwickler oder höher können Automatisierungen erstellen.",
    description: "Create page notice for a member who cannot create",
  },
  "agentAutomations.backToList": {
    message: "Zurück zu den Automatisierungen",
    description: "Accessible label of the back link to the automations list",
  },
  "agentAutomations.backToAutomation": {
    message: "Zurück zur Automatisierung",
    description: "Button returning from the edit page to the automation's page",
  },
  "agentAutomations.tabMine": {
    message: "Meine",
    description: "List tab: automations the viewer created",
  },
  "agentAutomations.tabAll": {
    message: "Alle",
    description: "List tab: every automation in the workspace",
  },
  "agentAutomations.emptyMineTitle": {
    message: "Du hast noch keine Automatisierungen",
    description: "Empty state title on the Mine tab",
  },
  "agentAutomations.emptyAllTitle": {
    message: "Dieser Workspace hat noch keine Automatisierungen",
    description: "Empty state title on the All tab",
  },
  "agentAutomations.emptyBody": {
    message:
      "Eine Automatisierung startet nach Zeitplan eine Agent-Sitzung, z. B. jeden Werktag um 09:00, mit der Aufgabe, dem Repository und den Limits, die du wählst.",
    description: "Empty state explanation",
  },
  "agentAutomations.loadMore": {
    message: "Mehr laden",
    description: "Load the next page of automations or runs",
  },
  "agentAutomations.loadFailed": {
    message: "Automatisierungen konnten nicht geladen werden.",
    description: "Load error",
  },
  "agentAutomations.retry": {
    message: "Erneut versuchen",
    description: "Retry button on a load error",
  },
  "agentAutomations.forbidden": {
    message:
      "Nur Mitglieder mit der Rolle Entwickler oder höher können Automatisierungen nutzen.",
    description: "Load error for a member without access",
  },
  "agentAutomations.notFound": {
    message: "Diese Automatisierung existiert nicht oder wurde gelöscht.",
    description: "Load error for a missing automation",
  },
  "agentAutomations.noTrigger": {
    message: "Kein Zeitplan",
    description: "Row text when an automation has no schedule",
  },
  "agentAutomations.nextRun": {
    message: "Nächster Lauf",
    description: "Label before the next run time",
  },
  "agentAutomations.lastRun": {
    message: "Letzter Lauf",
    description: "Label before the last run time",
  },
  "agentAutomations.neverRun": {
    message: "nie",
    description: "Last run value when it has not run",
  },
  "agentAutomations.lastSession": {
    message: "Sitzung ansehen",
    description: "Link to the session the last run started",
  },
  "agentAutomations.pauseLabel": {
    message: "{name} pausieren",
    description: "Accessible name of the switch that pauses an automation",
  },
  "agentAutomations.resumeLabel": {
    message: "{name} fortsetzen",
    description: "Accessible name of the switch that resumes an automation",
  },
  "agentAutomations.pausedBadge": {
    message: "Pausiert",
    description: "Badge on an automation a person paused",
  },
  "agentAutomations.ended": {
    message: "Beendet",
    description: "Badge on an automation with no future run",
  },
  "agentAutomations.active": {
    message: "Aktiv",
    description: "Status of a running automation",
  },
  "agentAutomations.paused.repeated_failures": {
    message: "Pausiert: wiederholte Fehler",
    description: "Badge: paused by the platform after repeated failed runs",
  },
  "agentAutomations.paused.creator_lost_access": {
    message: "Pausiert: Ersteller hat keinen Zugriff mehr",
    description:
      "Badge: paused because the creator can no longer start sessions",
  },
  "agentAutomations.paused.template_invalid": {
    message: "Pausiert: Vorlage ungültig",
    description:
      "Badge: paused because the session template no longer validates",
  },
  "agentAutomations.paused.unknown": {
    message: "Von der Plattform pausiert",
    description: "Badge for a pause reason this version does not know",
  },
  "agentAutomations.pausedExplain.repeated_failures": {
    message:
      "Diese Automatisierung wurde nach {failures} fehlgeschlagenen Läufen in Folge pausiert. Prüfe den Laufverlauf, behebe die Ursache und setze sie dann fort.",
    description: "Explanation on the detail page for a repeated-failures pause",
  },
  "agentAutomations.pausedExplain.creator_lost_access": {
    message:
      "Das Mitglied, das diese Automatisierung erstellt hat, darf keine Sitzungen mehr starten. Ein Workspace-Admin kann sie übernehmen, wodurch sie fortgesetzt wird.",
    description:
      "Explanation on the detail page for a creator-lost-access pause",
  },
  "agentAutomations.pausedExplain.template_invalid": {
    message:
      "Die Sitzungsvorlage besteht die Validierung nicht mehr, zum Beispiel weil sich ein Secret oder eine Modelleinstellung geändert hat. Ihr Ersteller kann sie bearbeiten und dann fortsetzen.",
    description: "Explanation on the detail page for a template-invalid pause",
  },
  "agentAutomations.pausedExplain.unknown": {
    message:
      "Die Plattform hat diese Automatisierung pausiert. Setze sie fort, sobald die Ursache behoben ist.",
    description: "Explanation for a pause reason this version does not know",
  },
  "agentAutomations.pausedToast": {
    message: "{name} pausiert.",
    description: "Toast after pausing an automation",
  },
  "agentAutomations.resumedToast": {
    message: "{name} fortgesetzt.",
    description: "Toast after resuming an automation",
  },
  "agentAutomations.runStartedToast": {
    message: "{name} hat eine Sitzung gestartet.",
    description: "Toast after Run now started a session",
  },
  "agentAutomations.runSettledToast": {
    message: "{name} ausgeführt: {outcome}.",
    description: "Toast after Run now ended with an outcome other than started",
  },
  "agentAutomations.tookOverToast": {
    message: "{name} gehört jetzt dir.",
    description: "Toast after an admin takes an automation over",
  },
  "agentAutomations.deletedToast": {
    message: "{name} gelöscht.",
    description: "Toast after deleting an automation",
  },
  "agentAutomations.createdToast": {
    message: "{name} erstellt.",
    description: "Toast after creating an automation",
  },
  "agentAutomations.updatedToast": {
    message: "{name} gespeichert.",
    description: "Toast after editing an automation",
  },
  "agentAutomations.actionFailed": {
    message: "Das hat nicht geklappt. Versuche es erneut.",
    description: "Generic failure toast for an automation action",
  },
  "agentAutomations.saveFailed": {
    message:
      "Die Automatisierung konnte nicht gespeichert werden. Versuche es erneut.",
    description: "Generic failure message when saving the form",
  },
  "agentAutomations.actionsLabel": {
    message: "Aktionen der Automatisierung",
    description: "Accessible label of the action button group",
  },
  "agentAutomations.runNow": {
    message: "Jetzt ausführen",
    description: "Button: fire the automation now",
  },
  "agentAutomations.runNowPausedHint": {
    message: "Setze diese Automatisierung fort, bevor du sie ausführst.",
    description:
      "Tooltip on Run now while the automation is paused by the platform",
  },
  "agentAutomations.pause": {
    message: "Pausieren",
    description: "Button: pause the automation",
  },
  "agentAutomations.resume": {
    message: "Fortsetzen",
    description: "Button: resume the automation",
  },
  "agentAutomations.edit": {
    message: "Bearbeiten",
    description: "Button: edit the automation",
  },
  "agentAutomations.editCreatorOnly": {
    message:
      "Nur das Mitglied, das diese Automatisierung erstellt hat, kann sie bearbeiten.",
    description: "Tooltip and notice: editing is for the creator only",
  },
  "agentAutomations.takeOver": {
    message: "Übernehmen",
    description: "Button: an admin becomes the automation's creator",
  },
  "agentAutomations.takeOverAdminOnly": {
    message:
      "Nur ein Workspace-Admin, der diese Automatisierung nicht erstellt hat, kann sie übernehmen.",
    description: "Tooltip on a disabled Take over button",
  },
  "agentAutomations.takeOverTitle": {
    message: "{name} übernehmen?",
    description: "Take over confirmation title",
  },
  "agentAutomations.takeOverBody": {
    message:
      "Künftige Läufe starten Sitzungen als du, mit deinem Zugriff, und die Vorlage wird mit deinem Zugriff erneut geprüft. Eine Automatisierung, die pausiert wurde, weil ihr Ersteller den Zugriff verloren hat, wird fortgesetzt.",
    description: "Take over confirmation body",
  },
  "agentAutomations.delete": {
    message: "Löschen",
    description: "Button: delete the automation",
  },
  "agentAutomations.deleteTitle": {
    message: "{name} löschen?",
    description: "Delete confirmation title",
  },
  "agentAutomations.deleteBody": {
    message:
      "Die Automatisierung und ihr Laufverlauf werden gelöscht. Bereits gestartete Sitzungen bleiben erhalten.",
    description: "Delete confirmation body",
  },
  "agentAutomations.settingsTitle": {
    message: "Einstellungen",
    description: "Settings card title on the detail page",
  },
  "agentAutomations.row.status": {
    message: "Status",
    description: "Settings row label",
  },
  "agentAutomations.row.schedule": {
    message: "Zeitplan",
    description: "Settings row label",
  },
  "agentAutomations.row.nextRun": {
    message: "Nächster Lauf",
    description: "Settings row label",
  },
  "agentAutomations.row.lastRun": {
    message: "Letzter Lauf",
    description: "Settings row label",
  },
  "agentAutomations.row.createdBy": {
    message: "Erstellt von",
    description: "Settings row label",
  },
  "agentAutomations.row.repo": {
    message: "Repository",
    description: "Settings row label",
  },
  "agentAutomations.row.agent": {
    message: "Agent",
    description: "Settings row label",
  },
  "agentAutomations.row.spendLimit": {
    message: "Ausgabenlimit pro Lauf",
    description: "Settings row label",
  },
  "agentAutomations.row.emailNotifications": {
    message: "E-Mail-Benachrichtigungen",
    description: "Settings row label",
  },
  "agentAutomations.row.secrets": {
    message: "Secrets",
    description: "Settings row label",
  },
  "agentAutomations.row.skills": {
    message: "Skills",
    description: "Settings row label",
  },
  "agentAutomations.taskTitle": {
    message: "Aufgabe",
    description: "Task card title on the detail page",
  },
  "agentAutomations.noRepo": {
    message: "Keines (reine Chat-Sitzung)",
    description: "Repository value when the template has none",
  },
  "agentAutomations.noSpendLimit": {
    message: "Kein Limit",
    description: "Spend limit value when the template has none",
  },
  "agentAutomations.none": {
    message: "Keine",
    description: "Empty value for secrets or skills",
  },
  "agentAutomations.runsTitle": {
    message: "Laufverlauf",
    description: "Run history card title",
  },
  "agentAutomations.runsDescription": {
    message: "Jeder geplante und manuelle Lauf, neueste zuerst.",
    description: "Run history card description",
  },
  "agentAutomations.runsEmpty": {
    message: "Noch keine Läufe.",
    description: "Run history empty state",
  },
  "agentAutomations.runs.fireAt": {
    message: "Auslösezeit",
    description: "Run table column",
  },
  "agentAutomations.runs.outcome": {
    message: "Ergebnis",
    description: "Run table column",
  },
  "agentAutomations.runs.session": {
    message: "Sitzung",
    description: "Run table column",
  },
  "agentAutomations.runs.missed": {
    message: "Verpasst",
    description: "Run table column: ticks skipped while the scheduler was down",
  },
  "agentAutomations.runs.error": {
    message: "Fehler",
    description: "Run table column",
  },
  "agentAutomations.runs.openSession": {
    message: "Sitzung öffnen",
    description: "Link to the session a run started",
  },
  "agentAutomations.outcome.started": {
    message: "Gestartet",
    description: "Run outcome: a session was started",
  },
  "agentAutomations.outcome.claimed": {
    message: "In Bearbeitung",
    description: "Run outcome: claimed, not yet settled",
  },
  "agentAutomations.outcome.skipped_overlap": {
    message: "Übersprungen",
    description:
      "Run outcome: skipped because the previous session was still running",
  },
  "agentAutomations.outcome.failed_create": {
    message: "Fehlgeschlagen",
    description: "Run outcome: the session could not be created",
  },
  "agentAutomations.outcome.denied": {
    message: "Abgelehnt",
    description:
      "Run outcome: the creator was no longer allowed to start the session",
  },
  "agentAutomations.outcome.didNotStart": {
    message: "Nicht gestartet",
    description:
      "Run outcome: the run started a session whose sandbox did not start for a cause on bex's side (w10/m497); the cause shows in the Error column. Not a failed create",
  },
  "agentAutomations.outcome.unknown": {
    message: "Unbekannt",
    description: "Run outcome this version does not know",
  },
  "agentAutomations.section.details": {
    message: "Details",
    description: "Form section title",
  },
  "agentAutomations.section.schedule": {
    message: "Zeitplan",
    description: "Form section title",
  },
  "agentAutomations.section.scheduleDescription": {
    message:
      "Wann die Automatisierung läuft. Die Zeitzone legt fest, was 09:00 bedeutet.",
    description: "Form section description",
  },
  "agentAutomations.section.session": {
    message: "Sitzung",
    description: "Form section title",
  },
  "agentAutomations.section.sessionDescription": {
    message:
      "Die Agent-Sitzung, die jeder Lauf startet, mit denselben Optionen wie eine neue Sitzung auf der Seite „Agents“.",
    description: "Form section description",
  },
  "agentAutomations.field.name": {
    message: "Name",
    description: "Form field label",
  },
  "agentAutomations.field.frequency": {
    message: "Wiederholung",
    description: "Form field label",
  },
  "agentAutomations.field.minute": {
    message: "Minute nach der vollen Stunde",
    description: "Form field label",
  },
  "agentAutomations.field.at": {
    message: "Uhrzeit",
    description: "Form field label",
  },
  "agentAutomations.field.weekdays": {
    message: "Tage",
    description: "Form field label",
  },
  "agentAutomations.field.runAt": {
    message: "Datum und Uhrzeit",
    description: "Form field label",
  },
  "agentAutomations.field.cron": {
    message: "Cron-Ausdruck",
    description: "Form field label",
  },
  "agentAutomations.field.timezone": {
    message: "Zeitzone",
    description: "Form field label",
  },
  "agentAutomations.field.endsOn": {
    message: "Endet am (optional)",
    description: "Form field label",
  },
  "agentAutomations.field.task": {
    message: "Aufgabe",
    description: "Form field label",
  },
  "agentAutomations.field.repo": {
    message: "Repository",
    description: "Form field label",
  },
  "agentAutomations.field.agent": {
    message: "Agent",
    description: "Form field label",
  },
  "agentAutomations.field.model": {
    message: "Modell",
    description: "Automation form — model field label (w10/m516)",
  },
  "agentAutomations.field.template": {
    message: "Sandbox",
    description: "Automation form — sandbox template field label (w10/m519)",
  },
  "agentAutomations.field.spendLimit": {
    message: "Ausgabenlimit pro Lauf (USD)",
    description: "Form field label",
  },
  "agentAutomations.field.emailNotifications": {
    message: "E-Mail-Benachrichtigungen",
    description: "Form field label",
  },
  "agentAutomations.field.secrets": {
    message: "Secrets",
    description: "Form field label",
  },
  "agentAutomations.field.skills": {
    message: "Skills",
    description: "Form field label",
  },
  "agentAutomations.frequency.hour": {
    message: "Jede Stunde",
    description: "Frequency option",
  },
  "agentAutomations.frequency.day": {
    message: "Jeden Tag",
    description: "Frequency option",
  },
  "agentAutomations.frequency.week": {
    message: "Jede Woche",
    description: "Frequency option",
  },
  "agentAutomations.frequency.once": {
    message: "Einmalig",
    description: "Frequency option",
  },
  "agentAutomations.frequency.custom": {
    message: "Benutzerdefiniert (Cron)",
    description: "Frequency option",
  },
  "agentAutomations.emailNotifications.always": {
    message: "Immer",
    description: "Email notification option: every run emails the creator",
  },
  "agentAutomations.emailNotifications.on_failure": {
    message: "Bei Fehlschlag",
    description:
      "Email notification option: only failed runs email the creator",
  },
  "agentAutomations.emailNotifications.on_success": {
    message: "Bei Erfolg",
    description:
      "Email notification option: only succeeded runs email the creator",
  },
  "agentAutomations.emailNotifications.never": {
    message: "Nie",
    description: "Email notification option: no run emails the creator",
  },
  "agentAutomations.weekday.mon": {
    message: "Mo",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.tue": {
    message: "Di",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.wed": {
    message: "Mi",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.thu": {
    message: "Do",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.fri": {
    message: "Fr",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.sat": {
    message: "Sa",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.sun": {
    message: "So",
    description: "Weekday toggle",
  },
  "agentAutomations.help.name": {
    message:
      "Kleinbuchstaben, Ziffern und einzelne Bindestriche, bis zu 64 Zeichen.",
    description: "Name field help",
  },
  "agentAutomations.help.minute": {
    message: "0 bis 59.",
    description: "Minute field help",
  },
  "agentAutomations.help.runAt": {
    message: "Wird in der unten gewählten Zeitzone gelesen ({zone}).",
    description: "Run-at field help",
  },
  "agentAutomations.help.cron": {
    message:
      "Fünf Felder: Minute Stunde Tag-des-Monats Monat Wochentag. Läufe dürfen nicht näher als 5 Minuten beieinander liegen.",
    description: "Cron field help",
  },
  "agentAutomations.help.endsOn": {
    message:
      "Der letzte Tag, an dem sie laufen kann. Leer lassen, damit sie läuft, bis du sie pausierst.",
    description: "Ends-on field help",
  },
  "agentAutomations.help.task": {
    message:
      "Was der Agent bei jedem Lauf tun soll. Nenne das erwartete Ergebnis.",
    description: "Task field help",
  },
  "agentAutomations.help.repo": {
    message:
      "Optional. Ohne Repository ist jeder Lauf eine reine Chat-Sitzung. Jeder Lauf arbeitet auf einem neuen bex-agent-Branch.",
    description: "Repository field help",
  },
  "agentAutomations.help.spendLimit": {
    message:
      "Eine Obergrenze für die Modellkosten pro Lauf. Erforderlich, wenn Läufe das Plattformmodell verwenden.",
    description: "Spend limit field help",
  },
  "agentAutomations.help.model": {
    message:
      "Das Modell, das jeder Lauf nutzt. Beim Standard folgt es dem Workspace.",
    description: "Automation form — model field help",
  },
  "agentAutomations.help.template": {
    message:
      "Die Maschine, die die Sandbox jedes Laufs bekommt. Der Workspace-Standard folgt bei jedem Lauf Einstellungen → Agents.",
    description: "Automation form — sandbox template field help (w10/m519)",
  },
  "agentAutomations.help.emailNotifications": {
    message:
      "Bei welchen Läufen du eine E-Mail bekommst. Bei einer Pausierung durch die Plattform immer.",
    description: "Email notifications field help",
  },
  "agentAutomations.help.secrets": {
    message:
      "Workspace-Secrets, die der Agent als Umgebungsvariablen erhält. Füge sie in den Workspace-Einstellungen hinzu.",
    description: "Secrets field help",
  },
  "agentAutomations.noSecrets": {
    message: "Keine Secrets",
    description: "Secrets row when none are chosen",
  },
  "agentAutomations.noSkills": {
    message: "Keine Skills",
    description: "Skills row when none are chosen",
  },
  "agentAutomations.placeholder.name": {
    message: "nightly-dependency-check",
    description: "Name field placeholder (an example slug)",
  },
  "agentAutomations.placeholder.repo": {
    message: "owner/repository",
    description: "Repository field placeholder",
  },
  "agentAutomations.placeholder.model": {
    message: "Workspace-Standard",
    description: "Automation form — empty free-form model field",
  },
  "agentAutomations.placeholder.template": {
    message: "Workspace-Standard",
    description:
      "Automation form — the sandbox field while no template list is available: the workspace default",
  },
  "agentAutomations.summary": {
    message: "Läuft: {schedule}",
    description: "Live summary under the schedule picker",
  },
  "agentAutomations.schedule.hour": {
    message: "Jede Stunde um :{minute}",
    description: "Schedule in words, hourly",
  },
  "agentAutomations.schedule.day": {
    message: "Jeden Tag um {at}",
    description: "Schedule in words, daily",
  },
  "agentAutomations.schedule.week": {
    message: "Jeden {days} um {at}",
    description: "Schedule in words, weekly",
  },
  "agentAutomations.schedule.once": {
    message: "Einmalig am {when}",
    description: "Schedule in words, one-off",
  },
  "agentAutomations.schedule.custom": {
    message: "Cron {cron}",
    description: "Schedule in words, cron",
  },
  "agentAutomations.schedule.until": {
    message: "bis {date}",
    description: "Schedule end date suffix",
  },
  "agentAutomations.problem.nameRequired": {
    message: "Gib einen Namen ein.",
    description: "Form validation",
  },
  "agentAutomations.problem.nameInvalid": {
    message:
      "Verwende Kleinbuchstaben, Ziffern und einzelne Bindestriche, bis zu 64 Zeichen.",
    description: "Form validation",
  },
  "agentAutomations.problem.taskRequired": {
    message: "Beschreibe die Aufgabe.",
    description: "Form validation",
  },
  "agentAutomations.problem.spendLimit": {
    message:
      "Gib einen Betrag über 0 und bis zu 100000 ein oder lass das Feld leer.",
    description: "Form validation",
  },
  "agentAutomations.problem.skills": {
    message:
      "Die Argumente eines Skills müssen eine einzelne Zeile mit höchstens 4 KB sein.",
    description: "Form validation",
  },
  "agentAutomations.problem.timezone": {
    message: "Wähle eine gültige Zeitzone.",
    description: "Form validation",
  },
  "agentAutomations.problem.minute": {
    message: "Gib eine Minute von 0 bis 59 ein.",
    description: "Form validation",
  },
  "agentAutomations.problem.at": {
    message: "Gib eine Uhrzeit ein.",
    description: "Form validation",
  },
  "agentAutomations.problem.weekdays": {
    message: "Wähle mindestens einen Tag.",
    description: "Form validation",
  },
  "agentAutomations.problem.runAtRequired": {
    message: "Wähle ein Datum und eine Uhrzeit.",
    description: "Form validation",
  },
  "agentAutomations.problem.runAtPast": {
    message: "Wähle einen Zeitpunkt in der Zukunft.",
    description: "Form validation",
  },
  "agentAutomations.problem.cron": {
    message:
      "Gib fünf Felder ein: Minute Stunde Tag-des-Monats Monat Wochentag.",
    description: "Form validation",
  },
  "agentAutomations.problem.endsOn": {
    message: "Gib ein gültiges Datum ein.",
    description: "Form validation",
  },
  "agentAutomations.problem.endsOnPast": {
    message: "Wähle ein Datum in der Zukunft.",
    description: "Form validation",
  },
  "agentAutomations.startedBy": {
    message: "Von Automatisierung gestartet",
    description: "Session page header note linking back to the automation",
  },
};

export default deAgentAutomations;
