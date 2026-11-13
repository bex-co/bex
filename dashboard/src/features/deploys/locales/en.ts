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
  "deploys.imageDigestCopy": {
    message: "Copy image digest",
    description:
      "Deploy header: accessible label of the button copying the image digest the deploy ran",
  },
  "deploys.imageDigestCopied": {
    message: "Image digest copied",
    description: "Deploy header: toast after copying the image digest",
  },
  "deploys.imageDigestCopyError": {
    message: "Couldn't copy to clipboard",
    description: "Deploy header: toast when copying the image digest fails",
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
  "deploys.durationActive": {
    message: "In progress",
    description:
      "Deploys tab Duration column: bare running-elapsed marker (the column header supplies the label)",
  },
  "deploys.deployedAt": {
    message: "Deployed {timestamp}",
    description:
      'Deploy row: how long ago a live/deactivated deploy went live — {timestamp} is elapsed text like "2 hours ago" (its finish time; createdAt fallback)',
  },
  "deploys.createdAt": {
    message: "Created {timestamp}",
    description:
      'Deploy row: how long ago a deploy that hasn\'t finished (created/queued/in-progress) was created — {timestamp} is elapsed text like "2 minutes ago"',
  },
  "deploys.canceledAt": {
    message: "Canceled {timestamp}",
    description:
      'Deploy row: how long ago a canceled deploy was canceled — {timestamp} is elapsed text like "3 days ago" (its finish time; createdAt fallback)',
  },
  "deploys.failedAt": {
    message: "Failed {timestamp}",
    description:
      'Deploy row: how long ago a build/pre-deploy/update failure landed — {timestamp} is elapsed text like "3 days ago" (its finish time; createdAt fallback)',
  },
  "deploys.notYet": {
    message: "—",
    description:
      "Deploy header: placeholder for a timestamp that hasn't happened yet",
  },
  "deploys.triggerCreate": {
    message: "First Deploy",
    description: "Deploy header: trigger=create label",
  },
  "deploys.triggerApi": {
    message: "API Deploy",
    description: "Deploy header: trigger=api label (an API-key deploy, w4/413)",
  },
  "deploys.triggerManual": {
    message: "Manual Deploy",
    description:
      "Deploy header: trigger=manual label (a dashboard click, w4/413)",
  },
  "deploys.triggerRollback": {
    message: "Rollback to {deployId}",
    description:
      "Deploy header: trigger label for a rollback deploy, naming the restored deploy (id must stay literal — never wrap in CSS capitalize)",
  },
  "deploys.triggerDeployHook": {
    message: "Deploy Hook",
    description: "Deploy row: trigger=deploy_hook label (secret-URL trigger)",
  },
  "deploys.triggerBlueprint": {
    message: "Blueprint Sync",
    description: "Deploy row: trigger=blueprint label (Blueprint sync deploy)",
  },
  "deploys.triggerNewCommit": {
    message: "New Commit",
    description: "Deploy row: trigger=new_commit label (git push redeploy)",
  },
  "deploys.triggerConfigChange": {
    message: "Config Change",
    description:
      "Deploy row: trigger=config_change label (an env var, secret file, or env-group edit that deployed)",
  },
  "deploys.triggerSettingsChange": {
    message: "Settings Change",
    description:
      "Deploy row: trigger=settings_change label (a Settings save that deployed, or a plan change; w8/m135)",
  },
  "deploys.triggerRestart": {
    message: "Restart",
    description:
      "Deploy row: trigger=restart label (a Restart roll of the live release, w4/379)",
  },
  "deploys.triggerDeployedByRender": {
    message: "Platform Rebuild",
    description:
      "Deploy row: trigger=deployed_by_render label (the platform rebuilt the live commit, e.g. to ship a runtime security patch)",
  },
  "deploys.logSearchPlaceholder": {
    message: "Search logs…",
    description: "Deploy detail page: log search input placeholder",
  },
  "deploys.logTypeFilter": {
    message: "Log type",
    description:
      "Deploy log viewer: aria-label of the log-type filter dropdown (Render's All/Application/Build selector)",
  },
  "deploys.logTypeAll": {
    message: "All logs",
    description: "Deploy log viewer type filter: show every log type",
  },
  "deploys.logTypeApp": {
    message: "Application logs",
    description:
      "Deploy log viewer type filter: app + pre-deploy lines, matching Render's Application bucket",
  },
  "deploys.logTypeBuild": {
    message: "Build logs",
    description: "Deploy log viewer type filter: build lines only",
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
  "deploys.detailRefreshError": {
    message: "Couldn't refresh this deploy. Showing the last loaded details.",
    description:
      "Deploy detail page: a background refresh of an already loaded deploy failed; the page keeps the last good details and offers a retry",
  },
  "deploys.logPartiallyLoaded": {
    message:
      "Only part of this time range is loaded. Load older to search earlier lines.",
    description:
      "Deploy log viewer banner while older history remains unloaded. The panel's search and type/instance filters run over loaded lines only, so it must not claim a number of matches",
  },
  "deploys.logStepEndMissing": {
    message:
      "This deploy was canceled before its pre-deploy step finished, and the step's completion time wasn't recorded, so its later output may fall outside this window. Choose a wider range in Log options to see it.",
    description:
      "Deploy log viewer: a canceled deploy's continued pre-deploy step has no recorded end, so the deploy window may omit its later output",
  },
  "deploys.logStepEndUnavailable": {
    message:
      "Couldn't check when this deploy's pre-deploy step finished, so later step output may fall outside this window. Try again, or choose a wider range in Log options.",
    description:
      "Deploy log viewer: reading a canceled deploy's continued pre-deploy step end failed; offers a retry",
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
  "deploys.buildsTitle": {
    message: "Builds",
    description:
      "A cron job's Builds tab: card title over its build history, Render's Builds page (w2/m360)",
  },
  "deploys.columnDeploy": {
    message: "Deploy",
    description:
      "Deploys tab table: first column header (status, id, commit, timestamp)",
  },
  "deploys.columnTrigger": {
    message: "Trigger",
    description: "Deploys tab table: column header for what triggered a deploy",
  },
  "deploys.columnDuration": {
    message: "Duration",
    description:
      "Deploys tab table: column header for elapsed build/deploy time",
  },
  "deploys.columnActions": {
    message: "Actions",
    description:
      "Deploys tab table: accessible-only header over the Cancel/Rollback action column",
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
  "deploys.listCount_one": {
    message: "{count} deploy",
    description: "Deploys tab: complete deploy count (singular)",
  },
  "deploys.listCount_other": {
    message: "{count} deploys",
    description: "Deploys tab: complete deploy count (plural)",
  },
  "deploys.listCountLoaded_one": {
    message: "{count} deploy loaded",
    description:
      "Deploys tab: loaded count when more server pages remain (singular)",
  },
  "deploys.listCountLoaded_other": {
    message: "{count} deploys loaded",
    description:
      "Deploys tab: loaded count when more server pages remain (plural)",
  },
  "deploys.listLoadMore": {
    message: "Load more",
    description: "Deploys tab: button fetching the next page of deploy history",
  },
  "deploys.listLoadMoreError": {
    message: "Couldn't load older deploys.",
    description:
      "Deploys tab: toast when reading an older page of deploy history fails",
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
  "deploys.reason.http_port_waiting": {
    message:
      "Waiting for the container to start before detecting its HTTP port",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_waiting, w4/566)",
  },
  "deploys.reason.http_port_detecting": {
    message: "Detecting the container's HTTP port",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_detecting, w4/566)",
  },
  "deploys.reason.http_port_limit": {
    message:
      "The detected HTTP port exceeds the service's 75-port limit; set PORT explicitly",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_limit, w4/566)",
  },
  "deploys.reason.http_port_detected": {
    message: "Detected HTTP on port {port}; serving it",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_detected, w4/566); params are the sentence's values",
  },
  "deploys.reason.http_port_not_answering": {
    message:
      "Port {port} accepted a connection but has not answered HTTP yet; continuing to scan while the container starts",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_not_answering, w4/566); params are the sentence's values",
  },
  "deploys.reason.http_port_none_yet": {
    message:
      "No port answers HTTP yet; continuing to scan while the container starts",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_none_yet, w4/566)",
  },
  "deploys.reason.http_port_fallback": {
    message:
      "No port answered HTTP; using fallback port {port}. Set PORT to the HTTP listener if it binds elsewhere",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_fallback, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_not_found": {
    message:
      "the image {image} was not found in its registry: the repository or tag does not exist. Check the image URL and tag, or deploy a digest that exists",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_not_found, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_credential_refused": {
    message:
      "{host} refused this service's registry credential for the image {image} (HTTP {status}). Check the credential's username and token, or remove the credential from this service to pull a public image anonymously",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_credential_refused, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_access_refused": {
    message:
      "the registry refused access to the image {image}. If the image is private, add a registry credential for {host} to this service",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_access_refused, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_read_refused": {
    message:
      "the registry refused to read the image {image} (HTTP {status}). Check the image URL and tag",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_read_refused, w4/566); params are the sentence's values",
  },
  "deploys.reason.build_timed_out": {
    message:
      "the build did not finish within the build window; check the build logs",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason build_timed_out, w4/566)",
  },
  "deploys.reason.pre_deploy_timed_out": {
    message:
      "the pre-deploy command did not finish within its window; check the pre-deploy logs",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_timed_out, w4/566)",
  },
  "deploys.reason.health_timed_out": {
    message:
      "the deploy did not become healthy within the health-gate window; check the service logs",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason health_timed_out, w4/566)",
  },
  "deploys.reason.probe_http_waiting": {
    message:
      "the container is running but its {kind} health check has not succeeded, so the rollout is waiting: GET {path} on port {port}. Check that the path returns a 2xx or 3xx status on the service's own port.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_waiting, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_http_waiting_restarted": {
    message:
      "the container is running but its {kind} health check has not succeeded, so the rollout is waiting: GET {path} on port {port}. The liveness check has already restarted the container {restarts} time(s), so the check is failing rather than merely slow. Check that the path returns a 2xx or 3xx status on the service's own port.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_waiting_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_http_failed": {
    message:
      "the container is running but its {kind} health check never succeeded within the rollout window, so the deploy failed: GET {path} on port {port}. Check that the path returns a 2xx or 3xx status on the service's own port.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_failed, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_http_failed_restarted": {
    message:
      "the container is running but its {kind} health check never succeeded within the rollout window, so the deploy failed: GET {path} on port {port}. The liveness check has already restarted the container {restarts} time(s), so the check is failing rather than merely slow. Check that the path returns a 2xx or 3xx status on the service's own port.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_failed_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_waiting": {
    message:
      "the container is running but its {kind} health check has not succeeded, so the rollout is waiting: a TCP connect to port {port}. No Health Check Path is set, so the check is a TCP connect, which only asks whether the process is listening on that port.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_waiting, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_waiting_restarted": {
    message:
      "the container is running but its {kind} health check has not succeeded, so the rollout is waiting: a TCP connect to port {port}. The liveness check has already restarted the container {restarts} time(s), so the check is failing rather than merely slow. No Health Check Path is set, so the check is a TCP connect, which only asks whether the process is listening on that port.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_waiting_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_failed": {
    message:
      "the container is running but its {kind} health check never succeeded within the rollout window, so the deploy failed: a TCP connect to port {port}. No Health Check Path is set, so the check is a TCP connect, which only asks whether the process is listening on that port.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_failed, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_failed_restarted": {
    message:
      "the container is running but its {kind} health check never succeeded within the rollout window, so the deploy failed: a TCP connect to port {port}. The liveness check has already restarted the container {restarts} time(s), so the check is failing rather than merely slow. No Health Check Path is set, so the check is a TCP connect, which only asks whether the process is listening on that port.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_failed_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.rollout_deadline_exceeded": {
    message: "rollout did not become healthy within the progress deadline",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason rollout_deadline_exceeded, w4/591)",
  },
  "deploys.reason.pre_deploy_window_exceeded": {
    message:
      "the pre-deploy command did not finish within its {minutes}-minute window; check the pre-deploy logs",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_window_exceeded, w4/591); params are the sentence's values",
  },
  "deploys.reason.pre_deploy_out_of_memory": {
    message:
      "the pre-deploy command ran out of memory (exit code {code}); check the pre-deploy logs",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_out_of_memory, w4/591); params are the sentence's values",
  },
  "deploys.reason.pre_deploy_exited": {
    message:
      "the pre-deploy command exited with code {code}; check the pre-deploy logs",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_exited, w4/591); params are the sentence's values",
  },
  "deploys.reason.pre_deploy_failed": {
    message: "the pre-deploy command failed; check the pre-deploy logs",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_failed, w4/591)",
  },
  "deploys.reason.pre_deploy_could_not_launch": {
    message:
      "the pre-deploy command could not start: the container runtime could not launch its program; the program never ran, so there are no pre-deploy logs",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_could_not_launch, w4/591)",
  },
  "deploys.reason.build_never_started": {
    message: "the build never started: {cause}",
    description:
      "A deploy failure reason by its code (bex-api DeployReason build_never_started, w4/591); {cause} is the operator's wait, whose sentences are worded as parts (w5/m356)",
  },
  "deploys.reason.build_abandoned": {
    message:
      "the build did not finish within the build window, and this service's App record could not be read to explain why",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason build_abandoned, w4/591)",
  },
  "deploys.reason.deploy_abandoned": {
    message:
      "the deploy did not finish within its window, and this service's App record could not be read to explain why",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason deploy_abandoned, w4/591)",
  },
  "deploys.reason.platform_error": {
    message:
      "bex could not apply this release because of a platform error, not a problem with your code or settings. Contact support with reference {ref}",
    description:
      "A deploy failure reason by its code (bex-api DeployReason platform_error, w9/090): the platform refused one of bex's own changes; {ref} is the support reference, shown as-is",
  },
  "deploys.reason.canceled_suspended": {
    message: "Canceled: the service was suspended",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_suspended, w4/591)",
  },
  "deploys.reason.superseded_by_newer_release": {
    message: "Superseded by a newer release",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason superseded_by_newer_release, w4/591)",
  },
  "deploys.reason.superseded_by": {
    message: "Superseded by {deploy}",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason superseded_by, w4/591); params are the sentence's values",
  },
  "deploys.reason.canceled_by_restart": {
    message:
      "Canceled by restart {deploy}, which kept the running release: what this deploy was applying was not deployed",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_by_restart, w4/591); params are the sentence's values",
  },
  "deploys.reason.canceled_along_with": {
    message: "Canceled along with {deploy}",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_along_with, w4/591); params are the sentence's values",
  },
  "deploys.reason.canceled_along_with_newer": {
    message: "Canceled along with a newer deploy",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_along_with_newer, w4/591)",
  },
  "deploys.reason.restart_service_gone": {
    message: "the service no longer exists",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason restart_service_gone, w4/591)",
  },
  "deploys.reason.restart_wait_timed_out": {
    message: "the deploy this restart waited for did not finish within {wait}",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason restart_wait_timed_out, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_other_waiting": {
    message:
      "the container is running but its {kind} health check has not succeeded, so the rollout is waiting: the configured health check. Check that the path returns a 2xx or 3xx status on the service's own port.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_waiting, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.probe_other_waiting_restarted": {
    message:
      "the container is running but its {kind} health check has not succeeded, so the rollout is waiting: the configured health check. The liveness check has already restarted the container {restarts} time(s), so the check is failing rather than merely slow. Check that the path returns a 2xx or 3xx status on the service's own port.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_waiting_restarted, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.probe_other_failed": {
    message:
      "the container is running but its {kind} health check never succeeded within the rollout window, so the deploy failed: the configured health check. Check that the path returns a 2xx or 3xx status on the service's own port.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_failed, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.probe_other_failed_restarted": {
    message:
      "the container is running but its {kind} health check never succeeded within the rollout window, so the deploy failed: the configured health check. The liveness check has already restarted the container {restarts} time(s), so the check is failing rather than merely slow. Check that the path returns a 2xx or 3xx status on the service's own port.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_failed_restarted, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.hint_image_default_port": {
    message:
      " Nothing accepted a TCP connection on port {port}, the default because the image declares no TCP ports. The server must bind host 0.0.0.0 on the port in PORT ({port}).",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_image_default_port, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_image_declared_ports": {
    message:
      " Nothing accepted a TCP connection on port {port}, the primary port selected from the image's declared TCP ports ({ports}). Bind the server to 0.0.0.0 on one of those ports, or set the service's port to the one it listens on.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_image_declared_ports, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_env_port_only": {
    message:
      " Nothing accepted a TCP connection on port {port}, set by the service's PORT environment variable; the image declares no other TCP port. Bind the server to 0.0.0.0 on port {port}, or change the service's port to the one it listens on.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_env_port_only, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_configured_port_only": {
    message:
      " Nothing accepted a TCP connection on port {port}, set by the service's configured port; the image declares no other TCP port. Bind the server to 0.0.0.0 on port {port}, or change the service's port to the one it listens on.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_configured_port_only, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_env_port_declared": {
    message:
      " Nothing accepted a TCP connection on port {port}, set by the service's PORT environment variable; the image declares TCP ports ({ports}). Change the PORT environment variable to the port it listens on, or remove it to use the image's port.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_env_port_declared, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_configured_port_declared": {
    message:
      " Nothing accepted a TCP connection on port {port}, set by the service's configured port; the image declares TCP ports ({ports}). Change the service's port in Settings to the one it listens on, or remove it to use the image's port.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_configured_port_declared, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_background_worker": {
    message:
      " If the service does not need to receive traffic on any port, create a background worker instead.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_background_worker, w5/m356); en starts with the space that joins it to the sentence before",
  },
  "deploys.reason.hint_no_http_discovered": {
    message: " No port answered HTTP during startup discovery.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_http_discovered, w5/m356); en starts with the space that joins it to the sentence before",
  },
  "deploys.reason.hint_listening": {
    message: " The process is listening on port {port}.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_listening_adopted": {
    message:
      " The process is listening on port {port}, which the service now uses.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening_adopted, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_listening_ports": {
    message:
      " The process is listening on ports {ports}, not on port {port}; set the service's port (or its PORT variable) to the one that serves HTTP.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening_ports, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_listening_primary": {
    message:
      " The process is listening on port {port}, which the service now uses as its primary port.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening_primary, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_no_open_port": {
    message:
      " No open port was detected on the instance: the server must bind host 0.0.0.0 on the port in PORT ({port}).",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_port, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_no_open_port_before": {
    message:
      " No open port was detected on the instance: the server must bind host 0.0.0.0 on the port in PORT ({port}); it listened on port {previous} before.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_port_before, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_no_open_service_port": {
    message:
      " No open port was detected on the instance: the server must bind host 0.0.0.0 on the service's port ({port}).",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_service_port, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.queued_build_limit": {
    message:
      "{noun} has {active}/{limit} concurrent builds active; waiting for a slot",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_build_limit, w5/m356): a full concurrent-build cap; {noun} is the cap's scope, worded by deploys.reasonParam.noun.*",
  },
  "deploys.reason.queued_cluster_full": {
    message: "the cluster's build capacity is full; waiting for a slot",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_cluster_full, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_slot_taken": {
    message:
      "a concurrent dispatch took the last build slot; waiting for a slot",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_slot_taken, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_database": {
    message:
      "Waiting for linked database {database} to become available before building",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_database, w5/m356): what the build waited on; params are the sentence's values",
  },
  "deploys.reason.queued_datastore": {
    message:
      "Waiting for a linked datastore's connection details before building",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_datastore, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_build_credential": {
    message: "Waiting for the registry to accept this app's build credential",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_build_credential, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_pull_credential": {
    message: "Waiting for the registry to accept this app's pull credential",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_pull_credential, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_capacity": {
    message: "waiting for build capacity: {detail}",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity, w5/m356): waiting for build capacity; {detail} is the build plane's own wait, shown as-is",
  },
  "deploys.reason.queued_capacity_pod": {
    message:
      "waiting for build capacity: build job dispatched; waiting for its build pod to be created",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity_pod, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_capacity_no_node": {
    message: "waiting for build capacity: no build node has room for it yet",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity_no_node, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_capacity_short": {
    message:
      "waiting for build capacity: no build node has room for it yet (not enough {resources} free)",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity_short, w5/m356): what the build waited on; params are the sentence's values",
  },
  "deploys.reasonParam.kind.startup": {
    message: "startup",
    description:
      "A deploy reason's {kind} value startup (w4/591): which health check a stalled rollout is waiting on",
  },
  "deploys.reasonParam.kind.readiness": {
    message: "readiness",
    description:
      "A deploy reason's {kind} value readiness (w4/591): which health check a stalled rollout is waiting on",
  },
  "deploys.reasonParam.noun.workspace": {
    message: "workspace",
    description:
      "A deploy reason's {noun} value workspace (w5/m356): the scope of a full concurrent-build cap",
  },
  "deploys.reasonWait.duration": {
    message: "{wait}",
    description:
      "A deferred restart's wait, restart_wait_timed_out's {wait} (w5/m356): {wait} is bex-api's Go duration (3h0m0s), which en keeps so the sentence reads as the API's; {units} is the duration in words (deploys.reasonWait.hours and the rest), which other languages read",
  },
  "deploys.reasonWait.hours": {
    message: "{hours} h",
    description: "A deferred restart's wait in words (w5/m356): its hours",
  },
  "deploys.reasonWait.minutes": {
    message: "{minutes} min",
    description: "A deferred restart's wait in words (w5/m356): its minutes",
  },
  "deploys.reasonWait.seconds": {
    message: "{seconds} s",
    description: "A deferred restart's wait in words (w5/m356): its seconds",
  },
};

export default enDeploys;
