import type { TranslationEntry } from "@/i18n";

const de: Record<string, TranslationEntry> = {
  "sandboxGroupPages.navSandboxes": {
    message: "Sandboxes",
    description: "Sidebar entry and heading: a sandbox group's sandboxes",
  },
  "sandboxGroupPages.navSnapshots": {
    message: "Snapshots",
    description: "Sidebar entry and heading: a sandbox group's snapshots",
  },
  "sandboxGroupPages.navImages": {
    message: "Benutzerdefinierte Images",
    description:
      "Sidebar entry and heading: the images imported into a sandbox group",
  },
  "sandboxGroupPages.navSettings": {
    message: "Einstellungen",
    description: "Sidebar entry and heading: a sandbox group's settings",
  },
  "sandboxGroupPages.statusCreating": {
    message: "Wird erstellt",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusRunning": {
    message: "Läuft",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusResuming": {
    message: "Wird fortgesetzt",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusSuspending": {
    message: "Wird pausiert",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusSuspended": {
    message: "Pausiert",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusErrored": {
    message: "Fehlerhaft",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusTerminated": {
    message: "Beendet",
    description: "Sandbox status",
  },
  "sandboxGroupPages.rangeLabel": {
    message: "Zeitraum",
    description: "Accessible name of the time range picker",
  },
  "sandboxGroupPages.rangeHour": {
    message: "Letzte Stunde",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeDay": {
    message: "Letzte 24 Stunden",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeWeek": {
    message: "Letzte 7 Tage",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeMonth": {
    message: "Letzte 30 Tage",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeAll": {
    message: "Gesamter Zeitraum",
    description: "Time range option",
  },
  "sandboxGroupPages.searchSandboxes": {
    message: "Nach Name, Sandbox-ID oder Status suchen",
    description: "Placeholder of the Sandboxes search",
  },
  "sandboxGroupPages.searchSnapshots": {
    message: "Snapshots durchsuchen",
    description: "Placeholder of the Snapshots search",
  },
  "sandboxGroupPages.searchImages": {
    message: "Images durchsuchen",
    description: "Placeholder of the Custom images search",
  },
  "sandboxGroupPages.pauseLive": {
    message: "Live-Updates pausieren",
    description: "Button that stops the list refreshing",
  },
  "sandboxGroupPages.resumeLive": {
    message: "Live-Updates fortsetzen",
    description: "Button that restarts the list refreshing",
  },
  "sandboxGroupPages.showHideColumns": {
    message: "Spalten ein- oder ausblenden",
    description: "Accessible name of the columns menu",
  },
  "sandboxGroupPages.summaryTotal": {
    message: "Gesamt",
    description: "Summary: runs that ended in the time range",
  },
  "sandboxGroupPages.summaryErrored": {
    message: "Fehlerhaft",
    description: "Summary: share of runs that ended in an error",
  },
  "sandboxGroupPages.summaryErroredTip": {
    message: "Anteil der Sandboxes im Zeitraum, die mit einem Fehler endeten.",
    description: "Tooltip of the Errored summary",
  },
  "sandboxGroupPages.summaryRunning": {
    message: "Laufend",
    description: "Summary: sandboxes active now",
  },
  "sandboxGroupPages.summaryRunningTip": {
    message:
      "Sandboxes, die gerade erstellt werden, laufen, fortgesetzt oder pausiert werden, unabhängig vom Zeitraum. Sie zählen zum Parallelitätslimit.",
    description: "Tooltip of the Running summary",
  },
  "sandboxGroupPages.loadRunsError": {
    message: "Sandbox-Läufe konnten nicht geladen werden",
    description: "The run chart failed to load",
  },
  "sandboxGroupPages.runsChartLabel": {
    message:
      "Beendete Sandbox-Läufe: {succeeded} erfolgreich, {errored} fehlerhaft",
    description: "Accessible name of the run chart",
  },
  "sandboxGroupPages.runsSucceeded": {
    message: "Erfolgreich",
    description: "Chart legend: runs that ended without an error",
  },
  "sandboxGroupPages.runsErrored": {
    message: "Fehlerhaft",
    description: "Chart legend: runs that ended in an error",
  },
  "sandboxGroupPages.selectedCount_one": {
    message: "{count} Sandbox ausgewählt",
    description: "How many rows are selected",
  },
  "sandboxGroupPages.selectedCount_other": {
    message: "{count} Sandboxes ausgewählt",
    description: "How many rows are selected",
  },
  "sandboxGroupPages.terminateSelected": {
    message: "Beenden",
    description: "Button: terminate the selected sandboxes",
  },
  "sandboxGroupPages.terminateTitle_one": {
    message: "{count} Sandbox beenden?",
    description: "Terminate confirmation title",
  },
  "sandboxGroupPages.terminateTitle_other": {
    message: "{count} Sandboxes beenden?",
    description: "Terminate confirmation title",
  },
  "sandboxGroupPages.terminateDescription_one": {
    message:
      "{count} Sandbox wird gestoppt, und alles darin, was nicht in einem Snapshot gesichert ist, geht verloren.",
    description: "Terminate confirmation body",
  },
  "sandboxGroupPages.terminateDescription_other": {
    message:
      "Die {count} Sandboxes werden gestoppt, und alles darin, was nicht in einem Snapshot gesichert ist, geht verloren.",
    description: "Terminate confirmation body",
  },
  "sandboxGroupPages.terminateConfirm": {
    message: "Beenden",
    description: "Terminate confirmation button",
  },
  "sandboxGroupPages.terminateFailed_one": {
    message:
      "{count} Sandbox konnte nicht beendet werden. Bitte versuche es erneut.",
    description: "Toast: a terminate failed",
  },
  "sandboxGroupPages.terminateFailed_other": {
    message:
      "{count} Sandboxes konnten nicht beendet werden. Bitte versuche es erneut.",
    description: "Toast: terminates failed",
  },
  "sandboxGroupPages.terminateStarted_one": {
    message: "{count} Sandbox wird beendet",
    description: "Toast: a terminate was accepted",
  },
  "sandboxGroupPages.terminateStarted_other": {
    message: "{count} Sandboxes werden beendet",
    description: "Toast: terminates were accepted",
  },
  "sandboxGroupPages.terminateSandbox": {
    message: "Sandbox beenden",
    description: "Drawer menu item",
  },
  "sandboxGroupPages.sandboxActions": {
    message: "Sandbox-Aktionen",
    description: "Accessible name of the drawer's menu",
  },
  "sandboxGroupPages.loadSandboxError": {
    message: "Die Sandbox konnte nicht geladen werden",
    description: "A sandbox's drawer failed to load",
  },
  "sandboxGroupPages.loadSandboxesError": {
    message: "Sandboxes konnten nicht geladen werden",
    description: "The list failed to load",
  },
  "sandboxGroupPages.noMatchingSandboxes": {
    message: "Keine passenden Sandboxes",
    description: "Empty state with filters",
  },
  "sandboxGroupPages.noMatchingSandboxesHelp": {
    message: "Versuche eine andere Suche oder erweitere den Zeitraum.",
    description: "Empty state with filters",
  },
  "sandboxGroupPages.clearFilters": {
    message: "Filter zurücksetzen",
    description: "Button: reset the search and time range",
  },
  "sandboxGroupPages.firstSandboxTitle": {
    message: "Erstelle deine erste Sandbox",
    description: "Empty state title of the Sandboxes page",
  },
  "sandboxGroupPages.firstSandboxDescription": {
    message:
      "Starte eine Sandbox über die CLI oder richte deinen Code so ein, dass er Sandboxes programmatisch mit den SDKs für TypeScript und Python erstellt.",
    description: "Empty state body of the Sandboxes page",
  },
  "sandboxGroupPages.starterCli": {
    message: "CLI",
    description: "Starter tab: the command line",
  },
  "sandboxGroupPages.starterSdk": {
    message: "SDK",
    description: "Starter tab: the SDKs",
  },
  "sandboxGroupPages.starterInstallCli": {
    message: "Neueste CLI installieren oder aktualisieren",
    description: "Starter step",
  },
  "sandboxGroupPages.starterHomebrew": {
    message: "Homebrew",
    description: "Install method",
  },
  "sandboxGroupPages.starterDirect": {
    message: "Direkter Download",
    description: "Install method",
  },
  "sandboxGroupPages.starterLogin": {
    message: "Anmelden und diesen Workspace verwenden",
    description: "Starter step",
  },
  "sandboxGroupPages.starterCreateSandbox": {
    message: "Sandbox mit den Standardeinstellungen erstellen",
    description: "Starter step",
  },
  "sandboxGroupPages.starterApiKey": {
    message: "API-Schlüssel festlegen",
    description: "Starter step",
  },
  "sandboxGroupPages.starterApiKeyHelp": {
    message:
      "Führe dies im Terminal aus, in dem du deinen Code ausführst. Das SDK liest deinen Schlüssel aus der Umgebungsvariable RENDER_API_KEY. Ersetze den Platzhalter durch einen bex-API-Schlüssel.",
    description: "Starter step help",
  },
  "sandboxGroupPages.starterInstallSdk": {
    message: "Projekt einrichten und SDK installieren",
    description: "Starter step",
  },
  "sandboxGroupPages.starterPythonNeeds": {
    message:
      "Erfordert Python 3.10 oder neuer. -U aktualisiert ein älteres SDK; 1.4.0 und älter unterstützen keine Sandboxes.",
    description: "Starter step help",
  },
  "sandboxGroupPages.starterNodeNeeds": {
    message:
      "Erfordert Node.js 18 oder neuer. @latest aktualisiert auch ein bereits installiertes älteres SDK.",
    description: "Starter step help",
  },
  "sandboxGroupPages.starterFirstSandboxFile": {
    message: "Erste Sandbox initialisieren",
    description: "Starter step",
  },
  "sandboxGroupPages.starterRun": {
    message: "Code lokal ausführen",
    description: "Starter step",
  },
  "sandboxGroupPages.starterTypeScript": {
    message: "TypeScript",
    description: "SDK language",
  },
  "sandboxGroupPages.starterPython": {
    message: "Python",
    description: "SDK language",
  },
  "sandboxGroupPages.colSandbox": { message: "Sandbox", description: "Column" },
  "sandboxGroupPages.colStatus": { message: "Status", description: "Column" },
  "sandboxGroupPages.colCreated": {
    message: "Erstellt",
    description: "Column",
  },
  "sandboxGroupPages.colDuration": { message: "Dauer", description: "Column" },
  "sandboxGroupPages.colPlan": { message: "Tarif", description: "Column" },
  "sandboxGroupPages.colSource": {
    message: "Gestartet aus",
    description: "Column: the image or snapshot a sandbox started from",
  },
  "sandboxGroupPages.selectAll": {
    message: "Alle Sandboxes auf dieser Seite auswählen",
    description: "Accessible name of the select-all checkbox",
  },
  "sandboxGroupPages.selectSandbox": {
    message: "{name} auswählen",
    description: "Accessible name of a row's checkbox",
  },
  "sandboxGroupPages.baseImage": {
    message: "Basis-Image",
    description: "A sandbox started from the default image",
  },
  "sandboxGroupPages.importedImage": {
    message: "Importiertes Image",
    description: "A sandbox started from an imported image",
  },
  "sandboxGroupPages.previousPage": {
    message: "Zurück",
    description: "Paging button",
  },
  "sandboxGroupPages.nextPage": {
    message: "Weiter",
    description: "Paging button",
  },
  "sandboxGroupPages.switchedToRunning": {
    message:
      "Deine ausgewählte Sandbox wurde gestoppt. Es wurde zu einer laufenden gewechselt.",
    description: "Toast when the open sandbox stops",
  },
  "sandboxGroupPages.copied": { message: "Kopiert", description: "Copy toast" },
  "sandboxGroupPages.copyError": {
    message: "Kopieren fehlgeschlagen",
    description: "Copy failed toast",
  },
  "sandboxGroupPages.copySandboxId": {
    message: "Sandbox-ID kopieren",
    description: "Copy button",
  },
  "sandboxGroupPages.copySnapshotId": {
    message: "Snapshot-ID kopieren",
    description: "Copy button",
  },
  "sandboxGroupPages.tabEvents": {
    message: "Ereignisse",
    description: "Drawer tab",
  },
  "sandboxGroupPages.tabSnapshots": {
    message: "Snapshots",
    description: "Drawer tab",
  },
  "sandboxGroupPages.tabDetails": {
    message: "Details",
    description: "Drawer tab and section",
  },
  "sandboxGroupPages.tabVersions": {
    message: "Versionen",
    description: "Image drawer tab",
  },
  "sandboxGroupPages.sandboxGoneTitle": {
    message: "Sandbox nicht gefunden",
    description: "A sandbox that does not exist here",
  },
  "sandboxGroupPages.sandboxGoneDescription": {
    message: "{id} ist keine Sandbox, die du in dieser Gruppe sehen kannst.",
    description: "A sandbox that does not exist here",
  },
  "sandboxGroupPages.loadingEvents": {
    message: "Ereignisse werden geladen",
    description: "Events tab loading",
  },
  "sandboxGroupPages.loadEventsError": {
    message:
      "Ereignisse konnten nicht geladen werden. Versuche es gleich noch einmal.",
    description: "Events tab error",
  },
  "sandboxGroupPages.noEvents": {
    message: "Noch keine Ereignisse.",
    description: "Events tab empty",
  },
  "sandboxGroupPages.loadEarlierEvents": {
    message: "Frühere Ereignisse laden",
    description: "Button: the next page of events",
  },
  "sandboxGroupPages.loadingEarlierEvents": {
    message: "Frühere Ereignisse werden geladen",
    description: "Loading the next page of events",
  },
  "sandboxGroupPages.eventCreated": {
    message: "Erstellt",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventReady": {
    message: "Bereit",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventSuspending": {
    message: "Wird pausiert",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventSuspended": {
    message: "Pausiert",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventResuming": {
    message: "Wird fortgesetzt",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventTerminated": {
    message: "Beendet",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventErrored": {
    message: "Fehler",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventCommand": {
    message: "Befehl ({operation})",
    description: "An execution event that ran a command",
  },
  "sandboxGroupPages.eventFileTransfer": {
    message: "Dateiübertragung ({operation})",
    description: "An execution event that moved a file",
  },
  "sandboxGroupPages.exitCode": {
    message: "Exit-Code {code}",
    description: "A command's exit code",
  },
  "sandboxGroupPages.eventSnapshot": {
    message: "Snapshot",
    description: "A snapshot event",
  },
  "sandboxGroupPages.viewSnapshot": {
    message: "Snapshot ansehen",
    description: "Link to a snapshot's drawer",
  },
  "sandboxGroupPages.triggerSdk": {
    message: "SDK",
    description: "Trigger: the SDK",
  },
  "sandboxGroupPages.triggerCli": {
    message: "CLI",
    description: "Trigger: the CLI",
  },
  "sandboxGroupPages.triggerApi": {
    message: "API",
    description: "Trigger: the API",
  },
  "sandboxGroupPages.triggerDashboard": {
    message: "Dashboard",
    description: "Trigger: the dashboard",
  },
  "sandboxGroupPages.triggerIdleTimeout": {
    message: "Leerlauf-Timeout",
    description: "Trigger: the idle timeout",
  },
  "sandboxGroupPages.triggerSystem": {
    message: "bex",
    description: "Trigger: the platform itself",
  },
  "sandboxGroupPages.triggerConnection": {
    message: "Neuer Befehl oder neue Dateiübertragung",
    description: "Trigger: a resume a command started",
  },
  "sandboxGroupPages.loading": {
    message: "Wird geladen",
    description: "Loading text",
  },
  "sandboxGroupPages.loadSnapshotsError": {
    message: "Snapshots konnten nicht geladen werden",
    description: "Snapshots failed to load",
  },
  "sandboxGroupPages.noSnapshotsRunning": {
    message:
      "Noch keine Snapshots dieser Sandbox. Erstelle einen mit der CLI oder einem SDK.",
    description: "Snapshots tab of a running sandbox",
  },
  "sandboxGroupPages.noSnapshotsEnded": {
    message: "Von dieser Sandbox wurden keine Snapshots erstellt.",
    description: "Snapshots tab of an ended sandbox",
  },
  "sandboxGroupPages.detailSandboxId": {
    message: "Sandbox-ID",
    description: "Detail label",
  },
  "sandboxGroupPages.detailGroup": {
    message: "Gruppe",
    description: "Detail label",
  },
  "sandboxGroupPages.detailPlan": {
    message: "Tarif",
    description: "Detail label",
  },
  "sandboxGroupPages.planSize": {
    message: "{plan} ({cpus} CPU-Kerne, {memory} GB RAM)",
    description: "A plan with its size",
  },
  "sandboxGroupPages.detailCreated": {
    message: "Erstellt",
    description: "Detail label",
  },
  "sandboxGroupPages.detailEnded": {
    message: "Beendet",
    description: "Detail label",
  },
  "sandboxGroupPages.detailDuration": {
    message: "Dauer",
    description: "Detail label",
  },
  "sandboxGroupPages.detailTimeout": {
    message: "Timeout",
    description: "Detail label",
  },
  "sandboxGroupPages.detailIdleTimeout": {
    message: "Leerlauf-Timeout",
    description: "Detail label",
  },
  "sandboxGroupPages.off": {
    message: "Aus",
    description: "A timeout that is off",
  },
  "sandboxGroupPages.detailNetwork": {
    message: "Netzwerk",
    description: "Detail label",
  },
  "sandboxGroupPages.networkAllowAll": {
    message: "Alles zulassen",
    description: "Network policy",
  },
  "sandboxGroupPages.networkDenyAll": {
    message: "Alles blockieren",
    description: "Network policy",
  },
  "sandboxGroupPages.networkAllowList": {
    message: "Zulassungsliste",
    description: "Network policy",
  },
  "sandboxGroupPages.detailRegion": {
    message: "Region",
    description: "Detail label",
  },
  "sandboxGroupPages.detailSource": {
    message: "Gestartet aus",
    description: "Detail label",
  },
  "sandboxGroupPages.goToSnapshot": {
    message: "Zum Snapshot",
    description: "Link to the snapshot a sandbox started from",
  },
  "sandboxGroupPages.goToImage": {
    message: "Zum Image",
    description: "Link to the image a sandbox started from",
  },
  "sandboxGroupPages.detailTrigger": {
    message: "Auslöser",
    description: "Detail label: what started the sandbox",
  },
  "sandboxGroupPages.detailUser": {
    message: "Benutzer",
    description: "Detail label: who started the sandbox",
  },
  "sandboxGroupPages.detailError": {
    message: "Fehler",
    description: "Detail label",
  },
  "sandboxGroupPages.errorUnknown": {
    message:
      "bex konnte nicht feststellen, warum bei dieser Sandbox ein Fehler aufgetreten ist. Versuche es erneut oder wende dich mit der Sandbox-ID an den Support.",
    description: "An errored sandbox with no known reason",
  },
  "sandboxGroupPages.snapshotCapturing": {
    message: "Wird erstellt",
    description: "Snapshot status",
  },
  "sandboxGroupPages.snapshotAvailable": {
    message: "Verfügbar",
    description: "Snapshot status",
  },
  "sandboxGroupPages.snapshotFailed": {
    message: "Fehlgeschlagen",
    description: "Snapshot status",
  },
  "sandboxGroupPages.snapshotFailureInternal": {
    message: "Ein interner Fehler hat die Erstellung abgebrochen.",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailureTimeout": {
    message: "Bei dieser Erstellung ist eine Zeitüberschreitung aufgetreten.",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailureSourceTerminated": {
    message:
      "Die Quell-Sandbox wurde beendet, bevor die Erstellung abgeschlossen war.",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailureSourceErrored": {
    message:
      "Bei der Quell-Sandbox ist ein Fehler aufgetreten, bevor die Erstellung abgeschlossen war.",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailedHelp": {
    message:
      "Erstelle einen neuen Snapshot, um es erneut zu versuchen. Neue Sandboxes können nicht aus diesem starten.",
    description: "After a snapshot failure",
  },
  "sandboxGroupPages.noMatchingSnapshots": {
    message: "Keine passenden Snapshots",
    description: "Empty state with filters",
  },
  "sandboxGroupPages.firstSnapshotTitle": {
    message: "Erstelle deinen ersten Snapshot",
    description: "Empty state title of the Snapshots page",
  },
  "sandboxGroupPages.firstSnapshotDescription": {
    message:
      "Erstelle einen Snapshot, um die Dateien einer Sandbox zu sichern. Starte neue Sandboxes vom selben Stand, ohne alles neu einzurichten.",
    description: "Empty state body of the Snapshots page",
  },
  "sandboxGroupPages.firstSnapshotNeedsRunning": {
    message:
      "Du brauchst eine laufende Sandbox, um einen Snapshot zu erstellen.",
    description: "Empty state note",
  },
  "sandboxGroupPages.starterSnapshotSandbox": {
    message: "Snapshot einer laufenden Sandbox erstellen",
    description: "Starter step",
  },
  "sandboxGroupPages.starterStartFromSnapshot": {
    message: "Sandboxes aus dem Snapshot erstellen",
    description: "Starter step",
  },
  "sandboxGroupPages.starterSnapshotCode": {
    message: "Code einrichten",
    description: "Starter step",
  },
  "sandboxGroupPages.colSnapshot": {
    message: "Snapshot",
    description: "Column",
  },
  "sandboxGroupPages.colSourceSandbox": {
    message: "Quell-Sandbox",
    description: "Column and detail label",
  },
  "sandboxGroupPages.colSize": {
    message: "Größe",
    description: "Column and detail label",
  },
  "sandboxGroupPages.colCaptured": {
    message: "Erfasst",
    description: "Column and detail label",
  },
  "sandboxGroupPages.colExpires": {
    message: "Läuft ab",
    description: "Column and detail label",
  },
  "sandboxGroupPages.never": {
    message: "Nie",
    description: "A snapshot that never expires",
  },
  "sandboxGroupPages.snapshotGoneTitle": {
    message: "Snapshot nicht gefunden",
    description: "A snapshot that does not exist here",
  },
  "sandboxGroupPages.snapshotGoneDescription": {
    message:
      "{id} ist kein Snapshot, den du in diesem Workspace sehen kannst. Er wurde möglicherweise gelöscht oder ist abgelaufen.",
    description: "A snapshot that does not exist here",
  },
  "sandboxGroupPages.detailType": {
    message: "Typ",
    description: "Detail label: snapshot kind",
  },
  "sandboxGroupPages.kindFilesystem": {
    message: "Dateisystem",
    description: "Snapshot kind",
  },
  "sandboxGroupPages.kindRuntime": {
    message: "Runtime",
    description: "Snapshot kind",
  },
  "sandboxGroupPages.detailResourceConstraint": {
    message: "Ressourcenbeschränkung",
    description: "Detail label: the plan a snapshot needs",
  },
  "sandboxGroupPages.detailLifecycle": {
    message: "Lebenszyklus",
    description: "Detail label",
  },
  "sandboxGroupPages.lifecycleKept": {
    message: "Bleibt bis zur Löschung erhalten",
    description: "A snapshot with no time to live",
  },
  "sandboxGroupPages.lifecycleAfterUse": {
    message: "Läuft {days} Tage nach der letzten Nutzung ab",
    description: "A snapshot's time to live",
  },
  "sandboxGroupPages.noSandboxesFromSnapshot": {
    message: "Aus diesem Snapshot wurden noch keine Sandboxes gestartet.",
    description: "Snapshot drawer's sandboxes",
  },
  "sandboxGroupPages.restoreTitle": {
    message: "Sandbox aus diesem Snapshot starten",
    description: "Snapshot drawer starter code",
  },
  "sandboxGroupPages.deleteSnapshot": {
    message: "Snapshot löschen",
    description: "Button",
  },
  "sandboxGroupPages.deleteSnapshotTitle": {
    message: "Snapshot löschen?",
    description: "Confirmation title",
  },
  "sandboxGroupPages.deleteSnapshotDescription": {
    message:
      "Dadurch wird {name} dauerhaft gelöscht. Neue Sandboxes können nicht mehr daraus starten. Bereits daraus gestartete Sandboxes laufen weiter.",
    description: "Confirmation body",
  },
  "sandboxGroupPages.deleteSnapshotError": {
    message: "Der Snapshot konnte nicht gelöscht werden. Versuche es erneut.",
    description: "Toast",
  },
  "sandboxGroupPages.snapshotDeleted": {
    message: "Snapshot {name} gelöscht",
    description: "Toast",
  },
  "sandboxGroupPages.imageQueued": {
    message: "In Warteschlange",
    description: "Image status",
  },
  "sandboxGroupPages.imageImporting": {
    message: "Wird importiert",
    description: "Image status",
  },
  "sandboxGroupPages.imageAvailable": {
    message: "Verfügbar",
    description: "Image status",
  },
  "sandboxGroupPages.imageUnavailable": {
    message: "Nicht verfügbar",
    description: "Image status: no version can start a sandbox",
  },
  "sandboxGroupPages.imageFailed": {
    message: "Fehlgeschlagen",
    description: "Image version status",
  },
  "sandboxGroupPages.imageLatest": {
    message: "Neueste",
    description: "The newest available version of an image",
  },
  "sandboxGroupPages.imageImportingHelp": {
    message:
      "Bis diese Version fertig ist, starten Sandboxes aus der neuesten verfügbaren Version.",
    description: "A version still importing",
  },
  "sandboxGroupPages.imageFailureTimeout": {
    message: "Der Import hat zu lange gedauert.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureRetries": {
    message: "Der Import ist nach mehreren Versuchen fehlgeschlagen.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureCredentials": {
    message:
      "bex konnte das Image nicht abrufen. Prüfe die Registry-Zugangsdaten.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureNotFound": {
    message: "bex konnte das Image nicht finden. Prüfe Image-Name und Tag.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailurePull": {
    message: "bex konnte das Image nicht abrufen.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailurePlatform": {
    message:
      "Das Image ist für {platform}. Sandboxes unterstützen nur linux/amd64.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureMissing": {
    message:
      "Dem Image fehlt {what}, was Sandboxes zum Starten brauchen. Verwende ein Image mit einer Shell, etwa eine slim- oder Alpine-Variante.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureConvert": {
    message:
      "bex konnte das Image nicht konvertieren. Versuche, es erneut zu importieren.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureLayers": {
    message: "bex konnte die Layer des Images nicht lesen.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureStart": {
    message: "Der Import konnte nicht gestartet werden.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureStopped": {
    message: "Der Import wurde vor dem Abschluss gestoppt.",
    description: "Image failure",
  },
  "sandboxGroupPages.loadImagesError": {
    message: "Images konnten nicht geladen werden",
    description: "Images failed to load",
  },
  "sandboxGroupPages.noMatchingImages": {
    message: "Keine passenden Images",
    description: "Empty state with a search",
  },
  "sandboxGroupPages.firstImageTitle": {
    message: "Importiere dein erstes Image",
    description: "Empty state title of the Custom images page",
  },
  "sandboxGroupPages.firstImageDescription": {
    message:
      "Importiere ein Image aus einer Registry, damit Sandboxes daraus starten können. Importiere es mit der CLI oder dem SDK und starte dann Sandboxes daraus.",
    description: "Empty state body of the Custom images page",
  },
  "sandboxGroupPages.starterImportImage": {
    message: "Image importieren (linux/amd64)",
    description: "Starter step",
  },
  "sandboxGroupPages.starterStartFromImage": {
    message: "Sandboxes aus dem Image erstellen",
    description: "Starter step",
  },
  "sandboxGroupPages.starterImportCode": {
    message: "Images importieren",
    description: "Starter step",
  },
  "sandboxGroupPages.colImage": { message: "Image", description: "Column" },
  "sandboxGroupPages.colSourceImage": {
    message: "Quelle",
    description: "Column: the registry image",
  },
  "sandboxGroupPages.colLatestAvailable": {
    message: "Neueste verfügbare",
    description: "Column: the newest available version's digest",
  },
  "sandboxGroupPages.colUpdated": {
    message: "Aktualisiert",
    description: "Column: the newest import",
  },
  "sandboxGroupPages.imageGoneTitle": {
    message: "Image nicht gefunden",
    description: "An image name with no versions",
  },
  "sandboxGroupPages.imageGoneDescription": {
    message: "Diese Gruppe hat kein Image namens {name}.",
    description: "An image name with no versions",
  },
  "sandboxGroupPages.detailLatestAvailable": {
    message: "Neueste verfügbare Version",
    description: "Detail label",
  },
  "sandboxGroupPages.detailImageSource": {
    message: "Quelle",
    description: "Detail label: the registry",
  },
  "sandboxGroupPages.detailImage": {
    message: "Image",
    description: "Detail label",
  },
  "sandboxGroupPages.detailDigest": {
    message: "Digest",
    description: "Detail label",
  },
  "sandboxGroupPages.detailVersionId": {
    message: "Versions-ID",
    description: "Detail label",
  },
  "sandboxGroupPages.noSandboxesFromImage": {
    message: "Aus diesem Image wurden noch keine Sandboxes gestartet.",
    description: "Image drawer's sandboxes",
  },
  "sandboxGroupPages.general": {
    message: "Allgemein",
    description: "Settings section",
  },
  "sandboxGroupPages.makeDefault": {
    message: "Als Standard festlegen",
    description: "Button: make this the workspace's default group",
  },
  "sandboxGroupPages.madeDefault": {
    message: "{name} ist jetzt die Standard-Sandbox-Gruppe",
    description: "Toast",
  },
  "sandboxGroupPages.save": { message: "Speichern", description: "Button" },
  "sandboxGroupPages.limitSaved": {
    message: "Parallelitätslimit gespeichert",
    description: "Toast",
  },
  "sandboxGroupPages.brokeredCredentials": {
    message: "Vermittelte Zugangsdaten",
    description: "What a group delete removes",
  },
  "sandboxGroupPages.deleteGroup": {
    message: "Sandbox-Gruppe löschen",
    description: "Button and section",
  },
  "sandboxGroupPages.deleteGroupHelp": {
    message:
      "Lösche diese Gruppe mit ihren Sandboxes, Snapshots, Images und Zugangsdaten.",
    description: "Delete section help",
  },
  "sandboxGroupPages.deleteGroupDefault": {
    message:
      "Die Standardgruppe des Workspace kann nicht gelöscht werden. Lege zuerst eine andere Gruppe als Standard fest.",
    description: "Delete section help for the default group",
  },
  "sandboxGroupPages.deleteGroupTitle": {
    message: "Sandbox-Gruppe löschen?",
    description: "Confirmation title",
  },
  "sandboxGroupPages.deleteGroupDescription": {
    message:
      "Diese Sandbox-Gruppe, {name}, wird dauerhaft gelöscht, zusammen mit ihrem Laufverlauf, ihren Logs und Einstellungen:",
    description: "Confirmation body",
  },
  "sandboxGroupPages.deleteGroupStops": {
    message:
      "Alles, was diese Gruppe über SDK, CLI oder API nutzt, funktioniert dann nicht mehr.",
    description: "Confirmation body",
  },
  "sandboxGroupPages.groupDeleted": {
    message: "Sandbox-Gruppe {name} gelöscht",
    description: "Toast",
  },
  "sandboxGroupPages.linkLoadError": {
    message:
      "Der Link konnte nicht geöffnet werden. Versuche es gleich noch einmal.",
    description: "A link by id failed to resolve",
  },
  "sandboxGroupPages.backToGroups": {
    message: "Zu den Sandbox-Gruppen",
    description: "Button",
  },
};

export default de;
