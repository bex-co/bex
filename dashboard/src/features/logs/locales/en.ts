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
  "logs.retentionWindow_one": {
    message: "Logs are kept for {count} day on the {plan} plan.",
    description:
      "Log range picker hint: how many days the workspace plan keeps logs (w9/m389, from bex-api logRetention)",
  },
  "logs.retentionWindow_other": {
    message: "Logs are kept for {count} days on the {plan} plan.",
    description:
      "Log range picker hint: how many days the workspace plan keeps logs (w9/m389, from bex-api logRetention)",
  },
  "logs.retentionUpgrade_one": {
    message: "Upgrade to {plan} to keep {count} day.",
    description:
      "Log range picker hint: the cheapest plan that keeps logs longer, and its days (w9/m389)",
  },
  "logs.retentionUpgrade_other": {
    message: "Upgrade to {plan} to keep {count} days.",
    description:
      "Log range picker hint: the cheapest plan that keeps logs longer, and its days (w9/m389)",
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
  "logs.hostLabel": {
    message: "Request host",
    description:
      "Accessible label for the request-host filter input and its chip (Render's host log filter, w4/538)",
  },
  "logs.hostPlaceholder": {
    message: "Filter by host",
    description: "Placeholder for the request-host filter input",
  },
  "logs.liveUnsupported": {
    message:
      "Live tail is off — method, status, path and host filter request logs, so app logs can't be tailed with them.",
    description:
      "Note when the chosen log type cannot carry an active filter, so live tail is off (a request filter on app logs)",
  },
  "logs.liveWindowClosed": {
    message:
      "Live tail is off — this custom range has already ended, so no new lines can arrive in it.",
    description:
      "Note when the selected custom time range ended before it was applied, so live tail is off",
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
  "logs.requestPlanTitle": {
    message: "HTTP request logs require the Pro plan",
    description:
      "Notice title in place of Request logs on a Hobby workspace (PLAN_FEATURE_REQUIRED, w8/m138)",
  },
  "logs.requestPlanBody": {
    message:
      "Request logs are available on the Pro plan and higher. This workspace is on Hobby, so it shows application and build logs only.",
    description:
      "Notice body in place of Request logs on a Hobby workspace (w8/m138)",
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
  "logs.linesLabel": {
    message: "Log lines",
    description:
      "Accessible name of the scrollable log list, which takes keyboard focus so arrow keys and Page Up/Down scroll it",
  },
  "logs.linesKeysHint": {
    message:
      "Use the up and down arrow keys to move between lines, and Enter to show only that line's instance.",
    description:
      "Logs list: screen-reader hint on the focusable log region, naming its row keys",
  },
  "logs.linesKeysShort": {
    message: "↑↓ move between lines · Enter: only this instance",
    description:
      "Logs tab: the keys of the focused log list, shown in its corner while it has keyboard focus — arrows move between lines, Enter shows only that line's instance",
  },
  "logs.endOfLog": {
    message: "End of log",
    description:
      "Closing line after the last row of a log that will not grow (a finished deploy's), so a short log reads as complete",
  },
  "logs.loading": {
    message: "Loading logs…",
    description: "Shown while the first historical page is loading",
  },
  "logs.loadingOlder": {
    message: "Loading older logs…",
    description:
      "Shown at the top of the pane while a page-back request is in flight",
  },
  "logs.loadOlder": {
    message: "Load older",
    description:
      "Button in the truncation notice that fetches the next older page — the keyboard/underfilled-pane path to what scrolling to the top does",
  },
  "logs.truncatedNotice": {
    message: "Only part of this time range is loaded. Load older to continue.",
    description:
      "Banner when hasMore is true: more of the selected range remains unloaded, however many older pages are already shown, so it names no line count (w4/280)",
  },
  "logs.partialSearchNotice": {
    message:
      "Only the newest part of this range has been searched so far — load older to keep searching.",
    description:
      "Banner when a time-boxed search returned less than a page but hasMore is true: the older part of the range is unsearched, not empty (w4/m140)",
  },
  "logs.timeoutTitle": {
    message: "This search took too long",
    description:
      "State title when the logs query ran out of the server's time budget (QUERY_TIMEOUT, w4/m140)",
  },
  "logs.timeoutBody": {
    message: "Try a shorter time range or a more specific search term.",
    description: "State body under logs.timeoutTitle",
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
  "logs.errorBody": {
    message:
      "The request for this range failed. Try again, or pick a shorter range.",
    description:
      "Logs tab: body when the log history read failed, under logs.errorTitle; the server's own reason follows as a detail line",
  },
  "logs.errorBodyLive": {
    message:
      "New lines will still appear here as they arrive. Try again to load this range's history.",
    description:
      "Logs tab: body when the history read failed while the live tail still streams \u2014 new lines will still land in the panel",
  },
  "logs.filtersDone": {
    message: "Done",
    description: "Logs tab: button closing the Filters panel",
  },
  "logs.emptyBodyRange": {
    message: "Try a longer time range, or turn on Live to watch for new lines.",
    description:
      "Logs tab: body when the chosen range holds no lines, under logs.emptyTitle \u2014 what to try next",
  },
  "logs.turnOnLive": {
    message: "Turn on Live",
    description:
      "Logs tab: button in the empty state that switches the live tail on",
  },
  "logs.clearFilters": {
    message: "Clear filters",
    description:
      "Logs tab: button in the no-match state that clears the search and every filter",
  },
  "logs.errorTitleWithReason": {
    message: "Couldn't load logs: {reason}",
    description:
      "Inline banner over loaded log lines when a later read was refused; reason is the server's refusal",
  },
  "logs.observabilityViewEyebrow": {
    message: "Observability view",
    description:
      "Eyebrow over a project observability view's name (Render's OBSERVABILITY VIEW, w3/m505)",
  },
  "logs.applicationLogsTitle": {
    message: "Application Logs",
    description:
      "Document title part for a project's Application Logs view page (w3/m505)",
  },
  "logs.requestLogsTitle": {
    message: "Request Logs",
    description:
      "Document title part for a project's Request Logs view page (w3/m505)",
  },
  "logs.projectScopeEmptyTitle": {
    message: "Nothing to read in this view yet",
    description:
      "Project log view state: the view's scope holds no resource to read (w3/m505)",
  },
  "logs.projectScopeEmptyBody": {
    message:
      "This view reads the project's services, cron jobs, Postgres databases and Key Value instances, and there are none in it yet.",
    description:
      "Project Application Logs view with no service, cron job, Postgres or Key Value in it (w3/m505)",
  },
  "logs.projectRequestScopeEmptyBody": {
    message:
      "Request Logs read the project's web services, and there are none in this view yet.",
    description:
      "Project Request Logs view with no web service in it (w3/m505)",
  },
  "logs.projectEmptyTitle": {
    message: "No logs in this range",
    description:
      "Project log view state: its resources logged nothing in the selected range (w3/m505)",
  },
  "logs.projectEmptyBody": {
    message:
      "None of the resources in this view logged anything in the selected range.",
    description:
      "Project log view state body: no line in the range from any of its resources (w3/m505)",
  },
  "logs.viewNotFoundTitle": {
    message: "View not found",
    description:
      "Project observability page whose savedSearchId names no view of this project (w3/m505)",
  },
  "logs.viewNotFoundBody": {
    message: "This project has no view at this link. It may have been deleted.",
    description: "Body of the view-not-found state (w3/m505)",
  },
  "logs.viewOpen": {
    message: "Open {name}",
    description:
      "Link from the view-not-found state to the project's default view of the same type; name is the view's name",
  },
  "logs.viewLoadErrorTitle": {
    message: "Couldn't load this view",
    description:
      "Project observability page whose saved searches could not be read (w3/m505)",
  },
  "logs.viewLoadErrorBody": {
    message:
      "The project's views could not be read. Reload the page to try again.",
    description: "Body of the view load failure (w3/m505)",
  },
  "logs.newView": {
    message: "New View",
    description:
      "Project sidebar button that opens the TYPE menu for a new observability view (w3/m507, Render's '+ New View')",
  },
  "logs.newViewTypeHeading": {
    message: "Type",
    description:
      "Heading of the New View menu listing view types (Render's TYPE)",
  },
  "logs.editView": {
    message: "Edit view",
    description:
      "Observability view header button that opens the Edit view modal (w3/m507)",
  },
  "logs.duplicateView": {
    message: "Duplicate",
    description:
      "Observability view header button that copies the view (w3/m507)",
  },
  "logs.deleteView": {
    message: "Delete view",
    description:
      "Accessible name and confirm label of the observability view's trash button (w3/m507)",
  },
  "logs.deleteViewTitle": {
    message: "Delete {name}?",
    description:
      "Title of the confirmation before deleting an observability view; name is the view's name",
  },
  "logs.deleteViewBody": {
    message:
      "The view is deleted for everyone on the project. The project's resources and their logs are not affected.",
    description:
      "Body of the delete-view confirmation: views are shared by the whole project (w3/m507)",
  },
  "logs.viewDeleted": {
    message: "Deleted {name}",
    description:
      "Toast after an observability view was deleted; name is the view's name",
  },
  "logs.deleteViewFailed": {
    message: "Couldn't delete the view",
    description:
      "Toast when deleting an observability view failed without a server reason",
  },
  "logs.viewDuplicated": {
    message: "Created {name}",
    description: "Toast after a view was duplicated; name is the copy's name",
  },
  "logs.duplicateViewFailed": {
    message: "Couldn't duplicate the view",
    description:
      "Toast when duplicating an observability view failed without a server reason",
  },
  "logs.editViewTitle": {
    message: "Edit view",
    description: "Title of Render's Edit view modal (w3/m507)",
  },
  "logs.newViewTitle": {
    message: "New view",
    description: "Title of the view modal when New View opened it (w3/m507)",
  },
  "logs.viewNameLabel": {
    message: "Name",
    description: "Label of the view name field in the Edit view modal",
  },
  "logs.viewNameRequired": {
    message: "Name the view.",
    description: "Inline refusal of a blank view name",
  },
  "logs.viewFiltersLabel": {
    message: "Filters",
    description: "Heading of the filter rows in the Edit view modal",
  },
  "logs.viewAddFilter": {
    message: "Add filter",
    description: "Accessible name of the Edit view modal's add (+) button",
  },
  "logs.viewNoFilters": {
    message: "No filters: the view reads every resource in the project.",
    description:
      "Edit view modal with no filter rows: the view reads the whole project",
  },
  "logs.viewSharedNotice": {
    message:
      "Editing filters will update the view for everyone on the project.",
    description:
      "Render's notice under the Edit view filters: a view is shared by the whole project",
  },
  "logs.viewSaveFailed": {
    message: "Couldn't save the view. Try again.",
    description:
      "Edit view modal error when saving failed without a server reason",
  },
  "logs.viewClose": {
    message: "Close",
    description:
      "Edit view modal button that discards the changes (Render's Close)",
  },
  "logs.viewDone": {
    message: "Done",
    description: "Edit view modal button that saves the view (Render's Done)",
  },
  "logs.viewFilterField": {
    message: "Field",
    description: "Accessible name of a filter row's Field picker",
  },
  "logs.viewFilterOperator": {
    message: "Operator",
    description: "Accessible name of a filter row's Operator picker",
  },
  "logs.viewFilterValuesFor": {
    message: "Values for {field}",
    description:
      "Accessible name of a filter row's Values picker; field is the row's field",
  },
  "logs.viewValuesRequired": {
    message: "Choose at least one value.",
    description: "Inline refusal of a filter row with no value",
  },
  "logs.viewRemoveFilter": {
    message: "Remove {field} filter",
    description:
      "Accessible name of a filter row's trash button; field is the row's field",
  },
  "logs.viewOperatorIs": {
    message: "is",
    description:
      "Filter operator IS for a single-valued field (Environment, Time Range)",
  },
  "logs.viewOperatorIsOneOf": {
    message: "is one of",
    description: "Filter operator IS for a field that takes several values",
  },
  "logs.viewOperatorIsNotOneOf": {
    message: "is not one of",
    description: "Filter operator IS_NOT",
  },
  "logs.viewValuesPlaceholder": {
    message: "Select values",
    description:
      "Placeholder of a filter row's Values picker with nothing chosen",
  },
  "logs.viewNoResources": {
    message: "No resources in this project",
    description:
      "Resource picker of a project with no resource the view can read",
  },
  "logs.viewValueTypeHint": {
    message: "Type a value, then press Enter",
    description:
      "Placeholder of a typed filter value (instance, search text, status code, host, path)",
  },
  "logs.viewValueRemove": {
    message: "Remove {value}",
    description:
      "Accessible name of a typed filter value's remove button; value is the value",
  },
  "logs.viewFieldEnvironment": {
    message: "Environment",
    description: "Filter field Environment in the Edit view modal",
  },
  "logs.viewFieldResource": {
    message: "Resource",
    description: "Filter field Resource in the Edit view modal",
  },
  "logs.viewFieldResourceType": {
    message: "Resource Type",
    description: "Filter field Resource Type in the Edit view modal",
  },
  "logs.viewFieldText": {
    message: "Text Search",
    description: "Filter field Text Search in the Edit view modal",
  },
  "logs.viewFieldTimeRange": {
    message: "Time Range",
    description: "Filter field Time Range in the Edit view modal",
  },
  "logs.viewResourceTypeService": {
    message: "Services",
    description: "Resource Type value Services",
  },
  "logs.viewResourceTypeCronJob": {
    message: "Cron jobs",
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
    message: "Level: {level}",
    description:
      "Log line level gutter (w3/m509): accessible name and hover text of the level icon leading a log row; {level} is the line's level word (info, warning, error, \u2026)",
  },
  "logs.showMore": {
    message: "Show more",
    description:
      "Log row (w3/m509): button after a long message cut short, showing the whole message",
  },
  "logs.showLess": {
    message: "Show less",
    description:
      "Log row (w3/m509): button after a long message opened in full, cutting it short again",
  },
  "logs.copied": {
    message: "Logs copied",
    description:
      "Log explorer (w3/m509): toast after Options > Copy logs wrote the lines shown to the clipboard",
  },
  "logs.copyFailed": {
    message: "Couldn't copy the logs",
    description:
      "Log explorer (w3/m509): toast when the clipboard refused the copied logs",
  },
  "logs.copyLogs": {
    message: "Copy logs",
    description:
      "Log explorer Options menu (w3/m509): item copying every line shown to the clipboard (Render's Copy logs)",
  },
  "logs.options": {
    message: "Log options",
    description:
      "Log explorer (w3/m509): accessible name and tooltip of the Options (\u2026) menu button",
  },
  "logs.orderBy": {
    message: "Order by",
    description:
      "Log explorer Options menu (w3/m509): heading of the line order choice (Render's Order by)",
  },
  "logs.orderAscending": {
    message: "Ascending",
    description:
      "Log explorer Options menu (w3/m509): order choice, oldest line first and the newest at the bottom",
  },
  "logs.orderDescending": {
    message: "Descending",
    description:
      "Log explorer Options menu (w3/m509): order choice, newest line first at the top",
  },
  "logs.display": {
    message: "Display",
    description:
      "Log explorer Options menu (w3/m509): heading of the display options (Render's Display)",
  },
  "logs.expandEverything": {
    message: "Expand everything",
    description:
      "Log explorer Options menu (w3/m509): checkbox showing every long message in full (Render's Expand everything)",
  },
  "logs.maximize": {
    message: "Maximize",
    description:
      "Log explorer (w3/m509): button and menu item expanding the log to fill the screen (Render's Maximize)",
  },
  "logs.minimize": {
    message: "Exit full screen",
    description:
      "Log explorer (w3/m509): button and menu item restoring the maximized log to its place in the page",
  },
  "logs.maximizeHint": {
    message: "Maximize (M)",
    description:
      "Log explorer (w3/m509): tooltip of the Maximize button, naming its M shortcut",
  },
  "logs.minimizeHint": {
    message: "Exit full screen (M or Esc)",
    description:
      "Log explorer (w3/m509): tooltip of the button leaving full screen, naming the M and Escape keys that also leave it",
  },
  "logs.clearedTitle": {
    message: "Logs cleared",
    description:
      "Log explorer (w3/m509): title over a live tail whose lines the reader cleared with Shift+L",
  },
  "logs.clearedBody": {
    message: "New lines appear here as they arrive.",
    description:
      "Log explorer (w3/m509): body under the cleared live tail's title",
  },
  "logs.clearedRestore": {
    message: "Show earlier lines",
    description:
      "Log explorer (w3/m509): button under a cleared live tail bringing back the lines Shift+L cleared",
  },
};

export default enLogs;
