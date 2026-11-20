import type { TranslationEntry } from "@/i18n";

const deLogs: Record<string, TranslationEntry> = {
  "logs.rangeLabel": {
    message: "Zeitraum des Log-Verlaufs",
    description: "Accessible label for the relative log-history range controls",
  },
  "logs.rangeHistoryNote": {
    message:
      "Der Zeitraum begrenzt den Verlauf. Im Live-Modus werden neue Zeilen angehängt, sobald sie eintreffen.",
    description:
      "Explains how the bounded history window interacts with live tail",
  },
  "logs.retentionWindow_one": {
    message: "Logs werden im {plan}-Tarif {count} Tag aufbewahrt.",
    description:
      "Log range picker hint: how many days the workspace plan keeps logs (w9/m389, from bex-api logRetention)",
  },
  "logs.retentionWindow_other": {
    message: "Logs werden im {plan}-Tarif {count} Tage aufbewahrt.",
    description:
      "Log range picker hint: how many days the workspace plan keeps logs (w9/m389, from bex-api logRetention)",
  },
  "logs.retentionUpgrade_one": {
    message: "Wechsle zu {plan}, um {count} Tag aufzubewahren.",
    description:
      "Log range picker hint: the cheapest plan that keeps logs longer, and its days (w9/m389)",
  },
  "logs.retentionUpgrade_other": {
    message: "Wechsle zu {plan}, um {count} Tage aufzubewahren.",
    description:
      "Log range picker hint: the cheapest plan that keeps logs longer, and its days (w9/m389)",
  },
  "logs.typeLabel": {
    message: "Log-Typ",
    description: "Accessible label for the log-type filter dropdown",
  },
  "logs.typeAll": {
    message: "Alle Logs",
    description: "Log-type filter option: no type filter",
  },
  "logs.typeApplication": {
    message: "Anwendungs-Logs",
    description: "Log-type filter option: application (app) logs",
  },
  "logs.typeRequest": {
    message: "Request-Logs",
    description:
      "Log-type filter option: request logs (Traefik access lines, from the durable store)",
  },
  "logs.levelLabel": {
    message: "Level",
    description: "Accessible label for the log-level filter dropdown",
  },
  "logs.levelAll": {
    message: "Alle Level",
    description: "Level filter option: no level filter",
  },
  "logs.methodLabel": {
    message: "Methode",
    description: "Accessible label for the HTTP-method filter dropdown",
  },
  "logs.methodAll": {
    message: "Alle Methoden",
    description: "Method filter option: no method filter",
  },
  "logs.statusCodeLabel": {
    message: "Statuscode",
    description: "Accessible label for the HTTP-status filter dropdown",
  },
  "logs.statusCodeAll": {
    message: "Alle Status",
    description: "Status-code filter option: no status filter",
  },
  "logs.instanceLabel": {
    message: "Instanz",
    description: "Accessible label for the instance (replica) filter dropdown",
  },
  "logs.instanceAll": {
    message: "Alle Instanzen",
    description: "Instance filter option: no instance filter",
  },
  "logs.filterByInstance": {
    message: "Logs nach Instanz {instance} filtern",
    description:
      "Accessible label for the clickable instance slug on an application log line",
  },
  "logs.pathLabel": {
    message: "Request-Pfad",
    description: "Accessible label for the request-path filter input",
  },
  "logs.filtersButton": {
    message: "Filter",
    description:
      "Trigger for the structured-filters popover (level/method/status/instance/path)",
  },
  "logs.chipRemove": {
    message: "Filter {label} entfernen",
    description:
      "Accessible label for an active-filter chip's clear button; {label} is the filter's field name",
  },
  "logs.pathPlaceholder": {
    message: "Nach Pfad filtern",
    description: "Placeholder for the request-path filter input",
  },
  "logs.hostLabel": {
    message: "Request-Host",
    description:
      "Accessible label for the request-host filter input and its chip (Render's host log filter, w4/538)",
  },
  "logs.hostPlaceholder": {
    message: "Nach Host filtern",
    description: "Placeholder for the request-host filter input",
  },
  "logs.liveUnsupported": {
    message:
      "Live-Tail ist aus – Methode, Status, Pfad und Host filtern Request-Logs, daher können App-Logs damit nicht live verfolgt werden.",
    description:
      "Note when the chosen log type cannot carry an active filter, so live tail is off (a request filter on app logs)",
  },
  "logs.liveWindowClosed": {
    message:
      "Live-Tail ist aus – dieser benutzerdefinierte Zeitraum ist bereits beendet, daher können darin keine neuen Zeilen eintreffen.",
    description:
      "Note when the selected custom time range ended before it was applied, so live tail is off",
  },
  "logs.storeRequiredTitle": {
    message: "Request-Logs benötigen den Log-Speicher",
    description:
      "Empty-state title when a request/structured-filter query hits a deployment with no durable store (503)",
  },
  "logs.storeRequiredBody": {
    message:
      "Request-Logs und die Filter für Level, Status, Methode und Pfad lesen aus dem dauerhaften Log-Speicher, der hier nicht konfiguriert ist. Sie sind verfügbar, wo der Speicher angebunden ist.",
    description: "Empty-state body for the store-unavailable (503) state",
  },
  "logs.requestPlanTitle": {
    message: "HTTP-Request-Logs erfordern den Pro-Tarif",
    description:
      "Notice title in place of Request logs on a Hobby workspace (PLAN_FEATURE_REQUIRED, w8/m138)",
  },
  "logs.requestPlanBody": {
    message:
      "Request-Logs sind im Pro-Tarif und höher verfügbar. Dieser Workspace nutzt Hobby und zeigt daher nur Anwendungs- und Build-Logs.",
    description:
      "Notice body in place of Request logs on a Hobby workspace (w8/m138)",
  },
  "logs.searchPlaceholder": {
    message: "Logs durchsuchen",
    description: "Placeholder for the log search box (text filter)",
  },
  "logs.live": {
    message: "Live",
    description: "Label for the live-tail on/off toggle",
  },
  "logs.linesLabel": {
    message: "Logzeilen",
    description:
      "Accessible name of the scrollable log list, which takes keyboard focus so arrow keys and Page Up/Down scroll it",
  },
  "logs.linesKeysHint": {
    message:
      "Mit den Pfeiltasten nach oben und unten zwischen Zeilen wechseln, mit Enter nur die Instanz dieser Zeile anzeigen und mit Umschalt+F10 ihr Menü öffnen.",
    description:
      "Logs tab: screen-reader description of the focused log list's keys - arrows move the row cursor, Enter shows only that line's instance, Shift+F10 opens the line's menu (w3/m528)",
  },
  "logs.linesKeysShort": {
    message:
      "↑↓ zwischen Zeilen wechseln · Enter: nur diese Instanz · Umschalt+F10: Menü",
    description:
      "Logs tab: the keys of the focused log list, shown in its corner while it has keyboard focus - arrows move between lines, Enter shows only that line's instance, Shift+F10 opens the line's menu",
  },
  "logs.returnToBottom": {
    message: "Zurück nach unten",
    description:
      "Log list (w3/m528): accessible name and tooltip of Render's icon-only Return to Bottom button, shown while the reader is scrolled away from the newest line (ascending order)",
  },
  "logs.returnToTop": {
    message: "Zurück nach oben",
    description:
      "Log list (w3/m528): accessible name and tooltip of Render's icon-only Return to Top button, shown while the reader is scrolled away from the newest line (descending order)",
  },
  "logs.newLogs": {
    message: "Neue Logs",
    description:
      "Log list (w3/m528): Render's held-lines button while a live tail holds fewer than 10 new lines back from a reader scrolled away",
  },
  "logs.newLogsCount_one": {
    message: "{count}+ neuer Log",
    description:
      "Log list (w3/m528): Render's held-lines button for 10 or more held lines; count is the held count rounded up to a multiple of 5, at most 100, followed by a plus sign",
  },
  "logs.newLogsCount_other": {
    message: "{count}+ neue Logs",
    description:
      "Log list (w3/m528): Render's held-lines button for 10 or more held lines; count is the held count rounded up to a multiple of 5, at most 100, followed by a plus sign",
  },
  "logs.newLogsInLiveTail": {
    message: "Neue Logs im Live-Tail",
    description:
      "Log list (w3/m528): Render's button on a paused explorer when a new line arrived; clicking it turns Live on",
  },
  "logs.switchToLiveTail": {
    message: "Zum Live-Tail wechseln",
    description:
      "Log list (w3/m528): tooltip of the New logs in Live Tail button",
  },
  "logs.dismiss": {
    message: "Ausblenden",
    description:
      "Log list (w3/m528): accessible name of the x that hides the New logs in Live Tail button",
  },
  "logs.lineMenu": {
    message: "Zeilenoptionen",
    description:
      "Log list (w3/m528): accessible name and tooltip of the row menu button (...) on a hovered log line",
  },
  "logs.copyLogLine": {
    message: "Logzeile kopieren",
    description:
      "Log row menu (w3/m528): copies the line without its time, as Render's Copy log line",
  },
  "logs.lineCopied": {
    message: "Kopiert!",
    description:
      "Log row menu (w3/m528): toast after Copy log line, Render's 'Copied!'",
  },
  "logs.lineCopyFailed": {
    message: "Logzeile konnte nicht kopiert werden",
    description:
      "Log row menu (w3/m528): toast when the clipboard refused the copied line",
  },
  "logs.viewInContext": {
    message: "Im Kontext anzeigen",
    description:
      "Log row menu (w3/m528): opens a new tab on the same explorer, unfiltered, 10 minutes either side of the line",
  },
  "logs.sshIntoInstance": {
    message: "Per SSH mit der Instanz verbinden",
    description:
      "Log row menu (w3/m528): opens the service's Shell on the line's instance in a new tab",
  },
  "logs.sshInstanceNotRunning": {
    message:
      "Du kannst dich nur per SSH mit laufenden Instanzen verbinden. Diese ist nicht verfügbar.",
    description:
      "Log row menu (w3/m528): Render's tooltip on a disabled SSH into instance item, for an instance that is no longer running",
  },
  "logs.linesKeysHintMenu": {
    message:
      "Mit den Pfeiltasten nach oben und unten zwischen Zeilen wechseln und mit Umschalt+F10 das Menü einer Zeile öffnen.",
    description:
      "Log list without an instance filter (w3/m528): screen-reader description of its keys - arrows move the row cursor, Shift+F10 opens the line's menu",
  },
  "logs.linesKeysShortMenu": {
    message: "↑↓ zwischen Zeilen wechseln · Umschalt+F10: Menü",
    description:
      "Log list without an instance filter (w3/m528): its keys, shown in its corner while it has keyboard focus",
  },
  "logs.themeSettings": {
    message: "Theme-Einstellungen",
    description:
      "Log explorer Options menu (w3/m528): Render's last item, a link to the Appearance section of account Settings",
  },
  "logs.endOfLog": {
    message: "Ende des Logs",
    description:
      "Closing line after the last row of a log that will not grow (a finished deploy's), so a short log reads as complete",
  },
  "logs.loading": {
    message: "Logs werden geladen…",
    description: "Shown while the first historical page is loading",
  },
  "logs.loadingOlder": {
    message: "Ältere Logs werden geladen…",
    description:
      "Shown at the top of the pane while a page-back request is in flight",
  },
  "logs.loadOlder": {
    message: "Ältere laden",
    description:
      "Button in the truncation notice that fetches the next older page — the keyboard/underfilled-pane path to what scrolling to the top does",
  },
  "logs.truncatedNotice": {
    message:
      "Nur ein Teil dieses Zeitraums ist geladen. Lade ältere Einträge, um fortzufahren.",
    description:
      "Banner when hasMore is true: more of the selected range remains unloaded, however many older pages are already shown, so it names no line count (w4/280)",
  },
  "logs.partialSearchNotice": {
    message:
      "Bisher wurde nur der neueste Teil dieses Zeitraums durchsucht – lade ältere Einträge, um weiterzusuchen.",
    description:
      "Banner when a time-boxed search returned less than a page but hasMore is true: the older part of the range is unsearched, not empty (w4/m140)",
  },
  "logs.timeoutTitle": {
    message: "Diese Suche hat zu lange gedauert",
    description:
      "State title when the logs query ran out of the server's time budget (QUERY_TIMEOUT, w4/m140)",
  },
  "logs.timeoutBody": {
    message:
      "Versuche einen kürzeren Zeitraum oder einen genaueren Suchbegriff.",
    description: "State body under logs.timeoutTitle",
  },
  "logs.streaming": {
    message: "Live – neue Zeilen werden gestreamt",
    description: "Status under the log list when the SSE tail is connected",
  },
  "logs.paused": {
    message: "Live-Tail pausiert",
    description: "Status under the log list when live tail is toggled off",
  },
  "logs.disconnected": {
    message: "Live-Tail getrennt – Verbindung wird neu aufgebaut…",
    description: "Banner when the SSE stream drops",
  },
  "logs.emptyTitle": {
    message: "Keine Logs in diesem Zeitraum",
    description:
      "Empty-state title when the selected (always-bounded) history range holds no lines. Never claims the service has never logged — the range can't establish that (w6/m111)",
  },
  "logs.emptyBody": {
    message: "In diesem Zeitraum liegen keine Log-Zeilen.",
    description:
      "Empty-state body with no filters applied — honest about the bounded window, not the service's whole history (w6/m111)",
  },
  "logs.emptyFilteredBody": {
    message: "Keine Logs entsprechen diesen Filtern.",
    description:
      "Empty-state body when a type/text/structured filter yields nothing",
  },
  "logs.emptyFilteredTitle": {
    message: "Keine passenden Logs",
    description:
      "Empty-state title when a type/text/structured filter yields nothing",
  },
  "logs.errorTitle": {
    message: "Logs konnten nicht geladen werden",
    description: "Error-state title when the logs query fails",
  },
  "logs.errorBody": {
    message:
      "Die Anfrage für diesen Zeitraum ist fehlgeschlagen. Versuche es erneut oder wähle einen kürzeren Zeitraum.",
    description:
      "Logs tab: body when the log history read failed, under logs.errorTitle; the server's own reason follows as a detail line",
  },
  "logs.errorBodyLive": {
    message:
      "Neue Zeilen erscheinen hier weiterhin, sobald sie eintreffen. Versuche es erneut, um den Verlauf dieses Zeitraums zu laden.",
    description:
      "Logs tab: body when the history read failed while the live tail still streams \u2014 new lines will still land in the panel",
  },
  "logs.filtersDone": {
    message: "Fertig",
    description: "Logs tab: button closing the Filters panel",
  },
  "logs.emptyBodyRange": {
    message:
      "Wähle einen längeren Zeitraum oder schalte Live ein, um auf neue Zeilen zu warten.",
    description:
      "Logs tab: body when the chosen range holds no lines, under logs.emptyTitle \u2014 what to try next",
  },
  "logs.turnOnLive": {
    message: "Live einschalten",
    description:
      "Logs tab: button in the empty state that switches the live tail on",
  },
  "logs.clearFilters": {
    message: "Filter zurücksetzen",
    description:
      "Logs tab: button in the no-match state that clears the search and every filter",
  },
  "logs.errorTitleWithReason": {
    message: "Logs konnten nicht geladen werden: {reason}",
    description:
      "Inline banner over loaded log lines when a later read was refused; reason is the server's refusal",
  },
  "logs.observabilityViewEyebrow": {
    message: "Observability-Ansicht",
    description:
      "Eyebrow over a project observability view's name (Render's OBSERVABILITY VIEW, w3/m505)",
  },
  "logs.applicationLogsTitle": {
    message: "Anwendungs-Logs",
    description:
      "Document title part for a project's Application Logs view page (w3/m505)",
  },
  "logs.requestLogsTitle": {
    message: "Request-Logs",
    description:
      "Document title part for a project's Request Logs view page (w3/m505)",
  },
  "logs.projectScopeEmptyTitle": {
    message: "In dieser Ansicht gibt es noch nichts zu lesen",
    description:
      "Project log view state: the view's scope holds no resource to read (w3/m505)",
  },
  "logs.projectScopeEmptyBody": {
    message:
      "Diese Ansicht liest die Dienste, Cron-Jobs, Postgres-Datenbanken und Key-Value-Instanzen des Projekts, aber noch keine davon gehört dazu.",
    description:
      "Project Application Logs view with no service, cron job, Postgres or Key Value in it (w3/m505)",
  },
  "logs.projectRequestScopeEmptyBody": {
    message:
      "Request-Logs lesen die Webdienste des Projekts, aber in dieser Ansicht gibt es noch keinen.",
    description:
      "Project Request Logs view with no web service in it (w3/m505)",
  },
  "logs.projectRoleLimitTitle": {
    message: "Du kannst keine Logs ansehen",
    description:
      "Project log view state: the caller's workspace role reads no logs, so the view reads nothing (w3/078)",
  },
  "logs.projectRoleLimitBody": {
    message:
      "Logs in diesem Workspace erfordern die Rolle Contributor oder höher. Frag einen Workspace-Admin, deine Rolle zu ändern.",
    description:
      "Project log view state body: logs need the Contributor role or higher, as on Render (w3/078)",
  },
  "logs.projectEmptyTitle": {
    message: "Keine Logs in diesem Zeitraum",
    description:
      "Project log view state: its resources logged nothing in the selected range (w3/m505)",
  },
  "logs.projectEmptyBody": {
    message:
      "Keine Ressource dieser Ansicht hat im gewählten Zeitraum etwas protokolliert.",
    description:
      "Project log view state body: no line in the range from any of its resources (w3/m505)",
  },
  "logs.viewNotFoundTitle": {
    message: "Ansicht nicht gefunden",
    description:
      "Project observability page whose savedSearchId names no view of this project (w3/m505)",
  },
  "logs.viewNotFoundBody": {
    message:
      "Dieses Projekt hat unter diesem Link keine Ansicht. Sie wurde möglicherweise gelöscht.",
    description: "Body of the view-not-found state (w3/m505)",
  },
  "logs.viewOpen": {
    message: "{name} öffnen",
    description:
      "Link from the view-not-found state to the project's default view of the same type; name is the view's name",
  },
  "logs.viewLoadErrorTitle": {
    message: "Diese Ansicht konnte nicht geladen werden",
    description:
      "Project observability page whose saved searches could not be read (w3/m505)",
  },
  "logs.viewLoadErrorBody": {
    message:
      "Die Ansichten des Projekts konnten nicht gelesen werden. Laden Sie die Seite neu, um es erneut zu versuchen.",
    description: "Body of the view load failure (w3/m505)",
  },
  "logs.newView": {
    message: "Neue Ansicht",
    description:
      "Project sidebar button that opens the TYPE menu for a new observability view (w3/m507, Render's '+ New View')",
  },
  "logs.newViewTypeHeading": {
    message: "Typ",
    description:
      "Heading of the New View menu listing view types (Render's TYPE)",
  },
  "logs.editView": {
    message: "Ansicht bearbeiten",
    description:
      "Observability view header button that opens the Edit view modal (w3/m507)",
  },
  "logs.duplicateView": {
    message: "Duplizieren",
    description:
      "Observability view header button that copies the view (w3/m507)",
  },
  "logs.deleteView": {
    message: "Ansicht löschen",
    description:
      "Accessible name and confirm label of the observability view's trash button (w3/m507)",
  },
  "logs.deleteViewTitle": {
    message: "{name} löschen?",
    description:
      "Title of the confirmation before deleting an observability view; name is the view's name",
  },
  "logs.deleteViewBody": {
    message:
      "Die Ansicht wird für alle im Projekt gelöscht. Die Ressourcen des Projekts und ihre Logs bleiben unberührt.",
    description:
      "Body of the delete-view confirmation: views are shared by the whole project (w3/m507)",
  },
  "logs.viewDeleted": {
    message: "{name} gelöscht",
    description:
      "Toast after an observability view was deleted; name is the view's name",
  },
  "logs.deleteViewFailed": {
    message: "Die Ansicht konnte nicht gelöscht werden",
    description:
      "Toast when deleting an observability view failed without a server reason",
  },
  "logs.viewDuplicated": {
    message: "{name} erstellt",
    description: "Toast after a view was duplicated; name is the copy's name",
  },
  "logs.duplicateViewFailed": {
    message: "Die Ansicht konnte nicht dupliziert werden",
    description:
      "Toast when duplicating an observability view failed without a server reason",
  },
  "logs.editViewTitle": {
    message: "Ansicht bearbeiten",
    description: "Title of Render's Edit view modal (w3/m507)",
  },
  "logs.newViewTitle": {
    message: "Neue Ansicht",
    description: "Title of the view modal when New View opened it (w3/m507)",
  },
  "logs.viewNameLabel": {
    message: "Name",
    description: "Label of the view name field in the Edit view modal",
  },
  "logs.viewNameRequired": {
    message: "Gib der Ansicht einen Namen.",
    description: "Inline refusal of a blank view name",
  },
  "logs.viewFiltersLabel": {
    message: "Filter",
    description: "Heading of the filter rows in the Edit view modal",
  },
  "logs.viewAddFilter": {
    message: "Filter hinzufügen",
    description: "Accessible name of the Edit view modal's add (+) button",
  },
  "logs.viewNoFilters": {
    message: "Keine Filter: Die Ansicht liest alle Ressourcen des Projekts.",
    description:
      "Edit view modal with no filter rows: the view reads the whole project",
  },
  "logs.viewSharedNotice": {
    message:
      "Wenn du die Filter bearbeitest, ändert sich die Ansicht für alle im Projekt.",
    description:
      "Render's notice under the Edit view filters: a view is shared by the whole project",
  },
  "logs.viewSaveFailed": {
    message:
      "Die Ansicht konnte nicht gespeichert werden. Versuch es noch einmal.",
    description:
      "Edit view modal error when saving failed without a server reason",
  },
  "logs.viewClose": {
    message: "Schließen",
    description:
      "Edit view modal button that discards the changes (Render's Close)",
  },
  "logs.viewDone": {
    message: "Fertig",
    description: "Edit view modal button that saves the view (Render's Done)",
  },
  "logs.viewFilterField": {
    message: "Feld",
    description: "Accessible name of a filter row's Field picker",
  },
  "logs.viewFilterOperator": {
    message: "Operator",
    description: "Accessible name of a filter row's Operator picker",
  },
  "logs.viewFilterValuesFor": {
    message: "Werte für {field}",
    description:
      "Accessible name of a filter row's Values picker; field is the row's field",
  },
  "logs.viewValuesRequired": {
    message: "Wähle mindestens einen Wert.",
    description: "Inline refusal of a filter row with no value",
  },
  "logs.viewRemoveFilter": {
    message: "Filter {field} entfernen",
    description:
      "Accessible name of a filter row's trash button; field is the row's field",
  },
  "logs.viewOperatorIs": {
    message: "ist",
    description:
      "Filter operator IS for a single-valued field (Environment, Time Range)",
  },
  "logs.viewOperatorIsOneOf": {
    message: "ist eines von",
    description: "Filter operator IS for a field that takes several values",
  },
  "logs.viewOperatorIsNotOneOf": {
    message: "ist keines von",
    description: "Filter operator IS_NOT",
  },
  "logs.viewValuesPlaceholder": {
    message: "Werte wählen",
    description:
      "Placeholder of a filter row's Values picker with nothing chosen",
  },
  "logs.viewNoResources": {
    message: "Keine Ressourcen in diesem Projekt",
    description:
      "Resource picker of a project with no resource the view can read",
  },
  "logs.viewValueTypeHint": {
    message: "Wert eingeben, dann Enter drücken",
    description:
      "Placeholder of a typed filter value (instance, search text, status code, host, path)",
  },
  "logs.viewValueRemove": {
    message: "{value} entfernen",
    description:
      "Accessible name of a typed filter value's remove button; value is the value",
  },
  "logs.viewValueSearch": {
    message: "Suchen oder einen Wert eingeben",
    description:
      "Placeholder of the search box in a filter's value picker fed by the project's logs (Instance)",
  },
  "logs.viewValueAdd": {
    message: "„{value}“ hinzufügen",
    description:
      "Picker item that adds the typed text as a filter value; value is the typed text",
  },
  "logs.viewValuesLoading": {
    message: "Logs des Projekts werden gelesen…",
    description:
      "Value picker status while the project's logs are read for the field's values",
  },
  "logs.viewValuesUnavailable": {
    message:
      "Die Logs des Projekts konnten nicht gelesen werden. Gib einen Wert ein, um ihn hinzuzufügen.",
    description:
      "Value picker status when the project's logs could not be read for the field's values",
  },
  "logs.viewValuesNoneSeen": {
    message:
      "Keine in den Logs des Projekts der letzten 7 Tage. Gib einen Wert ein, um ihn hinzuzufügen.",
    description:
      "Value picker status when the project's logs hold no value of the field in the last 7 days",
  },
  "logs.viewFieldEnvironment": {
    message: "Umgebung",
    description: "Filter field Environment in the Edit view modal",
  },
  "logs.viewFieldResource": {
    message: "Ressource",
    description: "Filter field Resource in the Edit view modal",
  },
  "logs.viewFieldResourceType": {
    message: "Ressourcentyp",
    description: "Filter field Resource Type in the Edit view modal",
  },
  "logs.viewFieldText": {
    message: "Textsuche",
    description: "Filter field Text Search in the Edit view modal",
  },
  "logs.viewFieldTimeRange": {
    message: "Zeitraum",
    description: "Filter field Time Range in the Edit view modal",
  },
  "logs.viewResourceTypeService": {
    message: "Services",
    description: "Resource Type value Services",
  },
  "logs.viewResourceTypeCronJob": {
    message: "Cron-Jobs",
    description: "Resource Type value Cron jobs",
  },
  "logs.viewResourceTypePostgres": {
    message: "Postgres",
    description: "Resource Type value Postgres",
  },
  "logs.viewResourceTypeKeyValue": {
    message: "Key Value",
    description: "Resource Type value Key Value (the product name)",
  },
  "logs.lineLevel": {
    message: "Stufe: {level}",
    description:
      "Log line level gutter (w3/m509): accessible name and hover text of the level icon leading a log row; {level} is the line's level word (info, warning, error, \u2026)",
  },
  "logs.showMore": {
    message: "Mehr anzeigen",
    description:
      "Log row (w3/m509): button after a long message cut short, showing the whole message",
  },
  "logs.showLess": {
    message: "Weniger anzeigen",
    description:
      "Log row (w3/m509): button after a long message opened in full, cutting it short again",
  },
  "logs.copied": {
    message: "Logs kopiert",
    description:
      "Log explorer (w3/m509): toast after Options > Copy logs wrote the lines shown to the clipboard",
  },
  "logs.copyFailed": {
    message: "Die Logs konnten nicht kopiert werden",
    description:
      "Log explorer (w3/m509): toast when the clipboard refused the copied logs",
  },
  "logs.copyLogs": {
    message: "Logs kopieren",
    description:
      "Log explorer Options menu (w3/m509): item copying every line shown to the clipboard (Render's Copy logs)",
  },
  "logs.options": {
    message: "Log-Optionen",
    description:
      "Log explorer (w3/m509): accessible name and tooltip of the Options (\u2026) menu button",
  },
  "logs.orderBy": {
    message: "Sortieren nach",
    description:
      "Log explorer Options menu (w3/m509): heading of the line order choice (Render's Order by)",
  },
  "logs.orderAscending": {
    message: "Aufsteigend",
    description:
      "Log explorer Options menu (w3/m509): order choice, oldest line first and the newest at the bottom",
  },
  "logs.orderDescending": {
    message: "Absteigend",
    description:
      "Log explorer Options menu (w3/m509): order choice, newest line first at the top",
  },
  "logs.display": {
    message: "Anzeige",
    description:
      "Log explorer Options menu (w3/m509): heading of the display options (Render's Display)",
  },
  "logs.expandEverything": {
    message: "Alles ausklappen",
    description:
      "Log explorer Options menu (w3/m509): checkbox showing every long message in full (Render's Expand everything)",
  },
  "logs.maximize": {
    message: "Maximieren",
    description:
      "Log explorer (w3/m509): button and menu item expanding the log to fill the screen (Render's Maximize)",
  },
  "logs.minimize": {
    message: "Vollbild beenden",
    description:
      "Log explorer (w3/m509): button and menu item restoring the maximized log to its place in the page",
  },
  "logs.maximizeHint": {
    message: "Maximieren (M)",
    description:
      "Log explorer (w3/m509): tooltip of the Maximize button, naming its M shortcut",
  },
  "logs.minimizeHint": {
    message: "Vollbild beenden (M oder Esc)",
    description:
      "Log explorer (w3/m509): tooltip of the button leaving full screen, naming the M and Escape keys that also leave it",
  },
  "logs.clearedTitle": {
    message: "Logs geleert",
    description:
      "Log explorer (w3/m509): title over a live tail whose lines the reader cleared with Shift+L",
  },
  "logs.clearedBody": {
    message: "Neue Zeilen erscheinen hier, sobald sie eintreffen.",
    description:
      "Log explorer (w3/m509): body under the cleared live tail's title",
  },
  "logs.clearedRestore": {
    message: "Frühere Zeilen anzeigen",
    description:
      "Log explorer (w3/m509): button under a cleared live tail bringing back the lines Shift+L cleared",
  },
};

export default deLogs;
