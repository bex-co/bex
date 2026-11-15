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
  "logs.jumpToLatest": {
    message: "Zum Neuesten springen",
    description: "Button to re-pin autoscroll to the newest log line",
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
  "logs.errorTitleWithReason": {
    message: "Logs konnten nicht geladen werden: {reason}",
    description:
      "Inline banner over loaded log lines when a later read was refused; reason is the server's refusal",
  },
};

export default deLogs;
