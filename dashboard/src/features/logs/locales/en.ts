import type { TranslationEntry } from "@/i18n";

const enLogs: Record<string, TranslationEntry> = {
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
      "Log-type filter option: request logs (empty on bex — no backend)",
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
    message: "No logs yet",
    description: "Empty-state title when the service has produced no logs",
  },
  "logs.emptyBody": {
    message: "This service hasn't produced any logs yet.",
    description: "Empty-state body with no filters applied",
  },
  "logs.emptyFilteredBody": {
    message:
      "No logs match this filter. bex sources application logs only — request logs are empty.",
    description:
      "Empty-state body when a type/text filter yields nothing (honest about bex's app-only contract)",
  },
  "logs.errorTitle": {
    message: "Couldn't load logs",
    description: "Error-state title when the logs query fails",
  },
};

export default enLogs;
