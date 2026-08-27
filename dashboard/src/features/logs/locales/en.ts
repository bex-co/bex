import type { TranslationEntry } from "@/i18n";

const enLogs: Record<string, TranslationEntry> = {
  "logs.rangeLabel": {
    message: "Log history range",
    description: "Accessible label for the relative log-history range controls",
  },
  "logs.rangeHistoryNote": {
    message:
      "The range limits history. Live mode appends new lines as they arrive.",
    description:
      "Explains how the bounded history window interacts with live tail",
  },
  "logs.typeLabel": {
    message: "Log type",
    description: "Accessible label for the log-type filter dropdown",
  },
  "logs.typeAll": {
    message: "All logs",
    description: "Log-type filter option: no type filter",
  },
  "logs.typeApplication": {
    message: "Application logs",
    description: "Log-type filter option: application (app) logs",
  },
  "logs.typeRequest": {
    message: "Request logs",
    description:
      "Log-type filter option: request logs (Traefik access lines, from the durable store)",
  },
  "logs.levelLabel": {
    message: "Level",
    description: "Accessible label for the log-level filter dropdown",
  },
  "logs.levelAll": {
    message: "All levels",
    description: "Level filter option: no level filter",
  },
  "logs.methodLabel": {
    message: "Method",
    description: "Accessible label for the HTTP-method filter dropdown",
  },
  "logs.methodAll": {
    message: "All methods",
    description: "Method filter option: no method filter",
  },
  "logs.statusCodeLabel": {
    message: "Status code",
    description: "Accessible label for the HTTP-status filter dropdown",
  },
  "logs.statusCodeAll": {
    message: "All statuses",
    description: "Status-code filter option: no status filter",
  },
  "logs.instanceLabel": {
    message: "Instance",
    description: "Accessible label for the instance (replica) filter dropdown",
  },
  "logs.instanceAll": {
    message: "All instances",
    description: "Instance filter option: no instance filter",
  },
  "logs.filterByInstance": {
    message: "Filter logs by instance {instance}",
    description:
      "Accessible label for the clickable instance slug on an application log line",
  },
  "logs.pathLabel": {
    message: "Request path",
    description: "Accessible label for the request-path filter input",
  },
  "logs.filtersButton": {
    message: "Filters",
    description:
      "Trigger for the structured-filters popover (level/method/status/instance/path)",
  },
  "logs.chipRemove": {
    message: "Remove {label} filter",
    description:
      "Accessible label for an active-filter chip's clear button; {label} is the filter's field name",
  },
  "logs.pathPlaceholder": {
    message: "Filter by path",
    description: "Placeholder for the request-path filter input",
  },
  "logs.liveUnsupported": {
    message:
      "Live tail is off — request-log and structured filters can't be tailed.",
    description:
      "Note when a store-only filter disables live tail (the tail reads pod stdout)",
  },
  "logs.storeRequiredTitle": {
    message: "Request logs need the log store",
    description:
      "Empty-state title when a request/structured-filter query hits a deployment with no durable store (503)",
  },
  "logs.storeRequiredBody": {
    message:
      "Request logs and level/status/method/path filters read from the durable log store, which isn't configured here. They're available where the store is wired.",
    description: "Empty-state body for the store-unavailable (503) state",
  },
  "logs.searchPlaceholder": {
    message: "Search logs",
    description: "Placeholder for the log search box (text filter)",
  },
  "logs.live": {
    message: "Live",
    description: "Label for the live-tail on/off toggle",
  },
  "logs.jumpToLatest": {
    message: "Jump to latest",
    description: "Button to re-pin autoscroll to the newest log line",
  },
  "logs.loading": {
    message: "Loading logs…",
    description: "Shown while the first historical page is loading",
  },
  "logs.streaming": {
    message: "Live — streaming new lines",
    description: "Status under the log list when the SSE tail is connected",
  },
  "logs.paused": {
    message: "Live tail paused",
    description: "Status under the log list when live tail is toggled off",
  },
  "logs.disconnected": {
    message: "Live tail disconnected — reconnecting…",
    description: "Banner when the SSE stream drops",
  },
  "logs.emptyTitle": {
    message: "No logs in this time range",
    description:
      "Empty-state title when the selected (always-bounded) history range holds no lines. Never claims the service has never logged — the range can't establish that (w6/m111)",
  },
  "logs.emptyBody": {
    message: "No log lines fall within this time range.",
    description:
      "Empty-state body with no filters applied — honest about the bounded window, not the service's whole history (w6/m111)",
  },
  "logs.emptyFilteredBody": {
    message: "No logs match these filters.",
    description:
      "Empty-state body when a type/text/structured filter yields nothing",
  },
  "logs.emptyFilteredTitle": {
    message: "No matching logs",
    description:
      "Empty-state title when a type/text/structured filter yields nothing",
  },
  "logs.errorTitle": {
    message: "Couldn't load logs",
    description: "Error-state title when the logs query fails",
  },
};

export default enLogs;
