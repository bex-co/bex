import type { TranslationEntry } from "@/i18n";

const enDeploys: Record<string, TranslationEntry> = {
  "deploys.statusCreated": {
    message: "Created",
    description: "Deploy status: created",
  },
  "deploys.statusQueued": {
    message: "Queued",
    description: "Deploy status: queued",
  },
  "deploys.statusBuildInProgress": {
    message: "Building",
    description: "Deploy status: build_in_progress",
  },
  "deploys.statusBuildFailed": {
    message: "Build Failed",
    description: "Deploy status: build_failed",
  },
  "deploys.statusPreDeployInProgress": {
    message: "Pre-Deploy In Progress",
    description: "Deploy status: pre_deploy_in_progress",
  },
  "deploys.statusPreDeployFailed": {
    message: "Pre-Deploy Failed",
    description: "Deploy status: pre_deploy_failed",
  },
  "deploys.statusUpdateInProgress": {
    message: "In Progress",
    description: "Deploy status: update_in_progress",
  },
  "deploys.statusUpdateFailed": {
    message: "Failed",
    description: "Deploy status: update_failed",
  },
  "deploys.statusLive": {
    message: "Live",
    description: "Deploy status: live",
  },
  "deploys.statusCanceled": {
    message: "Canceled",
    description: "Deploy status: canceled",
  },
  "deploys.statusDeactivated": {
    message: "Deactivated",
    description: "Deploy status: deactivated",
  },
  "deploys.statusUnknown": {
    message: "Unknown",
    description: "Deploy status: unrecognized backend value",
  },
  "deploys.statusLabel": {
    message: "Deploy status",
    description:
      "Deploy detail header: explicit label distinguishing deploy status from service phase",
  },
  "deploys.created": {
    message: "Created",
    description: "Deploy header: created-at label",
  },
  "deploys.updated": {
    message: "Updated",
    description: "Label for the deploy's latest stored transition time",
  },
  "deploys.started": {
    message: "Started",
    description: "Deploy header: started-at label",
  },
  "deploys.finished": {
    message: "Finished",
    description: "Deploy header: finished-at label",
  },
  "deploys.duration": {
    message: "Duration",
    description: "Deploy header: elapsed build/deploy time label",
  },
  "deploys.durationValue": {
    message: "Duration {duration}",
    description: "Deploy row: elapsed build/deploy time",
  },
  "deploys.durationInProgress": {
    message: "Duration in progress",
    description: "Deploy row: elapsed time is still running",
  },
  "deploys.deployedAt": {
    message: "Deployed {timestamp}",
    description: "Deploy row: created/deployed timestamp",
  },
  "deploys.notYet": {
    message: "—",
    description:
      "Deploy header: placeholder for a timestamp that hasn't happened yet",
  },
  "deploys.triggerCreate": {
    message: "first deploy",
    description: "Deploy header: trigger=create label",
  },
  "deploys.triggerApi": {
    message: "manual deploy",
    description: "Deploy header: trigger=api label",
  },
  "deploys.triggerRollback": {
    message: "rollback to {deployId}",
    description:
      "Deploy header: trigger label for a rollback deploy, naming the restored deploy",
  },
  "deploys.notFoundTitle": {
    message: "Deploy not found",
    description: "Deploy detail page: not-found state title",
  },
  "deploys.notFoundBody": {
    message: "No deploy {deployId} exists for this service.",
    description: "Deploy detail page: not-found state body",
  },
  "deploys.logSearchPlaceholder": {
    message: "Search logs…",
    description: "Deploy detail page: log search input placeholder",
  },
  "deploys.buildLogsStoreUnavailable": {
    message: "Historical build logs are unavailable",
    description:
      "Deploy detail page: title shown when the durable log store isn't wired",
  },
  "deploys.buildLogsStoreUnavailableBody": {
    message:
      "This dashboard isn't connected to durable log storage. Application logs can still appear for this deploy.",
    description:
      "Deploy detail page: accurate explanation of a storeless build-log response",
  },
  "deploys.timelineTitle": {
    message: "Status timeline",
    description: "Deploy detail page: status-timeline card title",
  },
  "deploys.timelineCreated": {
    message: "Deploy created",
    description: "Deploy timeline: deploy row was created",
  },
  "deploys.timelineStarted": {
    message: "Deploy started",
    description: "Deploy timeline: backend-provided startedAt timestamp",
  },
  "deploys.timelineInProgress": {
    message: "Deploy in progress",
    description: "Deploy timeline: current non-terminal deploy status",
  },
  "deploys.timelineLive": {
    message: "Deploy live",
    description: "Deploy timeline: successful terminal state",
  },
  "deploys.timelineFailed": {
    message: "Deploy failed",
    description: "Deploy timeline: failed terminal state",
  },
  "deploys.timelineCanceled": {
    message: "Deploy canceled",
    description: "Deploy timeline: canceled terminal state",
  },
  "deploys.timelineDeactivated": {
    message: "Deploy deactivated",
    description: "Deploy timeline: deactivated terminal state",
  },
  "deploys.timelineEventsUnavailable": {
    message: "Service events are unavailable; showing deploy timestamps only.",
    description:
      "Deploy timeline: graceful fallback when service-events query fails",
  },
  "deploys.listTitle": {
    message: "Deploys",
    description: "Deploys tab: card title over the deploy-history list",
  },
  "deploys.listEmpty": {
    message: "No deploys yet.",
    description: "Deploys tab: empty state with no status filter",
  },
  "deploys.listEmptyFiltered": {
    message: "No deploys match the selected status.",
    description: "Deploys tab: empty state while a status filter is active",
  },
  "deploys.listStatusFilterLabel": {
    message: "Filter by status",
    description: "Deploys tab: aria-label of the status filter dropdown",
  },
  "deploys.listStatusAll": {
    message: "All statuses",
    description: "Deploys tab: status filter option matching every deploy",
  },
  "deploys.listSearchPlaceholder": {
    message: "Search loaded deploys and commits…",
    description:
      "Deploys tab: placeholder for client-side deploy-history search",
  },
  "deploys.listSearchLabel": {
    message: "Search loaded deploys and commits",
    description: "Deploys tab: accessible label for deploy-history search",
  },
  "deploys.listEmptySearch": {
    message: "No loaded deploys match your search.",
    description:
      "Deploys tab: empty state when client-side search has no match",
  },
  "deploys.listCountOne": {
    message: "{count} deploy",
    description: "Deploys tab: complete singular deploy count",
  },
  "deploys.listCountMany": {
    message: "{count} deploys",
    description: "Deploys tab: complete plural deploy count",
  },
  "deploys.listCountLoadedOne": {
    message: "{count} deploy loaded",
    description:
      "Deploys tab: singular loaded count when more server pages remain",
  },
  "deploys.listCountLoadedMany": {
    message: "{count} deploys loaded",
    description:
      "Deploys tab: plural loaded count when more server pages remain",
  },
  "deploys.listLoadMore": {
    message: "Load more",
    description: "Deploys tab: button fetching the next page of deploy history",
  },
  "deploys.logOptions": {
    message: "Log options",
    description:
      "Deploy log viewer: aria-label/tooltip of the options menu button",
  },
  "deploys.logRangeLabel": {
    message: "Time range",
    description:
      "Deploy log viewer options menu: heading over the time-range choices",
  },
  "deploys.logRangeDeploy": {
    message: "Deploy window",
    description:
      "Deploy log viewer: the default time range — the deploy's own createdAt..finishedAt window",
  },
  "deploys.logRangeLast15m": {
    message: "Last 15 minutes",
    description: "Deploy log viewer: relative time-range option (?r=15m)",
  },
  "deploys.logRangeLast1h": {
    message: "Last hour",
    description: "Deploy log viewer: relative time-range option (?r=1h)",
  },
  "deploys.logRangeLast6h": {
    message: "Last 6 hours",
    description: "Deploy log viewer: relative time-range option (?r=6h)",
  },
  "deploys.logRangeLast24h": {
    message: "Last 24 hours",
    description: "Deploy log viewer: relative time-range option (?r=24h)",
  },
  "deploys.logRangeLast7d": {
    message: "Last 7 days",
    description: "Deploy log viewer: relative time-range option (?r=7d)",
  },
  "deploys.logWrap": {
    message: "Wrap lines",
    description:
      "Deploy log viewer options menu: toggle wrapping long log lines vs horizontal scroll",
  },
  "deploys.logTimestamps": {
    message: "Show timestamps",
    description:
      "Deploy log viewer options menu: toggle the per-line timestamp column",
  },
  "deploys.logMaximize": {
    message: "Maximize",
    description:
      "Deploy log viewer: button expanding the viewer to fill the screen",
  },
  "deploys.logMinimize": {
    message: "Exit full screen",
    description:
      "Deploy log viewer: button restoring the maximized viewer to its inline size",
  },
};

export default enDeploys;
