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
  "deploys.duration": {
    message: "Duration",
    description: "Deploy header: elapsed build/deploy time label",
  },
  "deploys.factStatus": {
    message: "Status",
    description:
      "Deploy page: label of the deploy's Status fact (Render's STATUS)",
  },
  "deploys.factDeployed": {
    message: "Deployed",
    description:
      "Deploy page: label of the fact giving when the deploy was created (Render's DEPLOYED)",
  },
  "deploys.factSource": {
    message: "Source",
    description:
      "Deploy page: label of the fact naming the commit or image the deploy ran (Render's SOURCE)",
  },
  "deploys.factScaling": {
    message: "Scaling",
    description:
      "Deploy page (w1/m661, Render's DeployDetailsHeader): label of the fact counting the service's instances that run this deploy",
  },
  "deploys.scalingInstances_one": {
    message: "{deployed} of {total} instance",
    description:
      "Deploy page (w1/m661): the Scaling fact's value; {deployed} is how many of the service's instances run this deploy and {total} how many it needs, both shown as numbers (one)",
  },
  "deploys.scalingInstances_other": {
    message: "{deployed} of {total} instances",
    description:
      "Deploy page (w1/m661): the Scaling fact's value; {deployed} is how many of the service's instances run this deploy and {total} how many it needs, both shown as numbers (other)",
  },
  "deploys.factPreDeploy": {
    message: "Pre-deploy",
    description:
      "Deploy page: label of the fact giving the outcome of the deploy's pre-deploy command",
  },
  "deploys.preDeployRunning": {
    message: "Running",
    description:
      "Deploy page: Pre-deploy fact value while the pre-deploy command runs",
  },
  "deploys.preDeploySucceeded": {
    message: "Succeeded",
    description:
      "Deploy page: Pre-deploy fact value once the pre-deploy command succeeded",
  },
  "deploys.preDeployFailed": {
    message: "Failed",
    description:
      "Deploy page: Pre-deploy fact value when the pre-deploy command failed",
  },
  "deploys.preDeployCanceled": {
    message: "Canceled",
    description:
      "Deploy page: Pre-deploy fact value when the pre-deploy command was canceled",
  },
  "deploys.idLabel": {
    message: "Deploy ID",
    description:
      "Deploy page: the deploy's id, named in its copy button's accessible name",
  },
  "deploys.idCopied": {
    message: "Deploy ID copied",
    description: "Deploy page: toast after copying the deploy's id",
  },
  "deploys.idCopyFailed": {
    message: "Couldn't copy the deploy ID",
    description: "Deploy page: toast when copying the deploy's id fails",
  },
  "deploys.loadErrorTitle": {
    message: "Couldn't load this deploy",
    description:
      "Deploy page: title of the state shown when the deploy itself could not be read",
  },
  "deploys.loadErrorBody": {
    message:
      "This deploy couldn't be read just now. The deploy itself isn't affected.",
    description:
      "Deploy page: body of the state shown when the deploy itself could not be read; says the deploy is unaffected",
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
  "deploys.triggerServiceResumed": {
    message: "Service Resumed",
    description:
      "Deploy row: trigger=service_resumed label (the deploy a Resume opened on the release the service was serving, Render's service_resumed, w3/m643)",
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
  "deploys.autoDeployOffTitle": {
    message: "Auto-deploy is off",
    description:
      "Deploys page banner title when the service's auto-deploy is off (Render, w3/064)",
  },
  "deploys.autoDeployOffBody": {
    message:
      "Pushes to {branch} don't deploy this service. Deploy it manually, or turn auto-deploy back on.",
    description: "Deploys page banner body; {branch} is the tracked git branch",
  },
  "deploys.autoDeployOffBodyNoBranch": {
    message:
      "Pushes don't deploy this service. Deploy it manually, or turn auto-deploy back on.",
    description: "Deploys page banner body when the branch is unknown",
  },
  "deploys.autoDeployOffSettings": {
    message: "Auto-Deploy settings",
    description: "Deploys page banner link to the service Settings page",
  },
  "deploys.deploysManualTitle": {
    message: "Deploys are manual",
    description:
      "Deploys page banner title for a repo whose pushes can't reach bex (pushDeliveryMethod unavailable)",
  },
  "deploys.deploysManualBody": {
    message:
      "bex doesn't receive this repo's pushes, so they don't deploy this service. Deploy it manually or with the Deploy Hook, or connect the repo through the GitHub app.",
    description:
      "Deploys page banner body for a repo whose pushes can't reach bex: no Auto-Deploy setting can turn deploys on push on (Render: a public Git URL must be deployed manually)",
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
    message: "Search deploys and commits",
    description:
      "Placeholder of the deploy list's search field on every service's Deploys page (Render's text, w3/m555)",
  },
  "deploys.listSearchLabel": {
    message: "Search loaded deploys and commits",
    description: "Deploys tab: accessible label for deploy-history search",
  },
  "deploys.manualBuild": {
    message: "Manual Build",
    description:
      "Button beside a repo-backed cron job's deploy search that builds its latest commit and opens the new build (Render's Manual Build, w3/m555)",
  },
  "deploys.listClearSearch": {
    message: "Clear search",
    description:
      "Deploys tab: button under the no-match message that empties the search box",
  },
  "deploys.listMatchCount": {
    message: "{matched} of {total} loaded deploys match",
    description:
      "Deploys tab: count shown while a search is typed; matched = loaded deploys that match, total = loaded deploys",
  },
  "deploys.listLoadErrorTitle": {
    message: "Couldn't load deploys",
    description:
      "Deploys tab: heading of the error state when the deploy history could not be read; a Try again button follows",
  },
  "deploys.listLoadErrorBody": {
    message:
      "Your deploys are not affected. Check your connection and try again.",
    description:
      "Deploys tab: body of the error state when the deploy history could not be read and the server gave no message; a Try again button follows",
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
  "deploys.reason.http_port_localhost_only": {
    message:
      "No open port was detected on 0.0.0.0, but ports are open on localhost ({ports}); bind the server to host 0.0.0.0. Continuing to scan while the container starts",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_localhost_only, w8/388); params are the sentence's values: ports is the comma-separated list of ports the process listens on only at localhost",
  },
  "deploys.reason.http_port_none_http": {
    message:
      "Ports are open on 0.0.0.0 ({ports}) but none answers HTTP yet; continuing to scan while the container starts. If the service does not need to receive public HTTP traffic, create a private service instead",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_none_http, w8/388); params are the sentence's values: ports is the comma-separated list of open ports that answer no HTTP",
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
  "deploys.reason.image_registry_unreachable": {
    message:
      "the registry {host} did not answer the read of the image {image} for {minutes} minutes. Wait and deploy again, or check the registry provider's status page",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_registry_unreachable, w4/692); params are the sentence's values",
  },
  "deploys.reason.image_registry_unavailable": {
    message:
      "the registry {host} is unavailable and could not serve the image {image}. Wait and deploy again, or check the registry provider's status page",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_registry_unavailable, w4/m669): Render's Registry unavailable cause; params are the sentence's values",
  },
  "deploys.reason.image_registry_rate_limited": {
    message:
      "the registry {host} rate-limited the pull of the image {image} (HTTP 429). Wait for the rate limit to reset, use a registry credential for an account with a higher rate limit, or mirror the image to another registry",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_registry_rate_limited, w4/m669): Render's Rate limited (429 error) cause with its three remedies; params are the sentence's values",
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
  "deploys.reason.startup_out_of_memory": {
    message:
      "the instance ran out of memory during startup ({memory} limit on its compute plan; exit code {code}) — reduce memory use or choose a larger compute plan",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason startup_out_of_memory, w4/630); params are the sentence's values: memory is the compute plan's memory limit (e.g. 512Mi), code the exit code",
  },
  "deploys.reason.startup_crash_restarting": {
    message:
      "container exited shortly after start and is restarting repeatedly (last exit code {code}) — check the service logs for the crash output.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason startup_crash_restarting, w8/386); params are the sentence's values: code is the exit code, hint the port advice (a part)",
  },
  "deploys.reason.startup_crash_stopped": {
    message:
      "container exited shortly after start and its instance has stopped (last exit code {code}) — check the service logs for the crash output.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason startup_crash_stopped, w8/386); params are the sentence's values: code is the exit code, hint the port advice (a part)",
  },
  "deploys.reason.startup_early_exit_restarting": {
    message:
      'container exited shortly after start and is restarting repeatedly (last exit code 0): the process exited "successfully" but early. A service runs continuously and must not exit until it receives SIGTERM; use a cron job for a task that ends when its work is done. Check the service logs for why it ended.{hint}',
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason startup_early_exit_restarting, w8/386); the start command exited with code 0; hint is the port advice (a part)",
  },
  "deploys.reason.startup_early_exit_stopped": {
    message:
      'container exited shortly after start and its instance has stopped (last exit code 0): the process exited "successfully" but early. A service runs continuously and must not exit until it receives SIGTERM; use a cron job for a task that ends when its work is done. Check the service logs for why it ended.{hint}',
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason startup_early_exit_stopped, w8/386); the start command exited with code 0; hint is the port advice (a part)",
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
  "deploys.reason.deploy_wait_timed_out": {
    message: "the deploy this one waited for did not finish within {wait}",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason deploy_wait_timed_out, w4/652): a Rollback or another deploy that waited for the deploy in flight; params are the sentence's values",
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
  "deploys.reason.probe_open_port_waiting": {
    message:
      "the container is running but its {kind} health check has not succeeded, so the rollout is waiting: a TCP connect to any open port on the instance. No Health Check Path is set, so the check is a TCP connect, which only asks whether the process is listening on at least one port.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_open_port_waiting, w8/384): the stall of Render's default check, a TCP connect to any open port, which a first web image release with no Health Check Path runs; params are the sentence's values",
  },
  "deploys.reason.probe_open_port_waiting_restarted": {
    message:
      "the container is running but its {kind} health check has not succeeded, so the rollout is waiting: a TCP connect to any open port on the instance. The liveness check has already restarted the container {restarts} time(s), so the check is failing rather than merely slow. No Health Check Path is set, so the check is a TCP connect, which only asks whether the process is listening on at least one port.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_open_port_waiting_restarted, w8/384): the stall of Render's default check, a TCP connect to any open port, which a first web image release with no Health Check Path runs; params are the sentence's values",
  },
  "deploys.reason.probe_open_port_failed": {
    message:
      "the container is running but its {kind} health check never succeeded within the rollout window, so the deploy failed: a TCP connect to any open port on the instance. No Health Check Path is set, so the check is a TCP connect, which only asks whether the process is listening on at least one port.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_open_port_failed, w8/384): the stall of Render's default check, a TCP connect to any open port, which a first web image release with no Health Check Path runs; params are the sentence's values",
  },
  "deploys.reason.probe_open_port_failed_restarted": {
    message:
      "the container is running but its {kind} health check never succeeded within the rollout window, so the deploy failed: a TCP connect to any open port on the instance. The liveness check has already restarted the container {restarts} time(s), so the check is failing rather than merely slow. No Health Check Path is set, so the check is a TCP connect, which only asks whether the process is listening on at least one port.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_open_port_failed_restarted, w8/384): the stall of Render's default check, a TCP connect to any open port, which a first web image release with no Health Check Path runs; params are the sentence's values",
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
  "deploys.reason.hint_env_port_closed": {
    message:
      " Nothing accepted a TCP connection on port {port}, set by the service's PORT environment variable. Bind the server to 0.0.0.0 on port {port}, or update the PORT environment variable to the port it listens on.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_env_port_closed, w8/416): a web service's PORT names the one port Render scans; en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_last_result": {
    message: " Last result: {result}.",
    description:
      'What the failing health check last got back, which the operator appends to a health-check stall (bex-api DeployReason part hint_last_result, w6/m634); {result} is Render\'s English message ("HTTP health check failed with status code 503"), shown verbatim in every language as Render shows it; en starts with the space that joins it to the sentence before',
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
  "deploys.reason.hint_no_open_port_localhost": {
    message:
      " No open port was detected on 0.0.0.0, but ports are open on localhost ({ports}): the server must bind host 0.0.0.0 on the port in PORT ({port}).",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_port_localhost, w8/388); en starts with the space that joins it to the sentence before; params are the sentence's values: ports is the comma-separated list of ports the process listens on only at localhost",
  },
  "deploys.reason.hint_no_open_http_port": {
    message:
      " Ports are open on 0.0.0.0 ({ports}) but none answers HTTP: the server must serve HTTP on the port in PORT ({port}). If the service does not need to receive public HTTP traffic, create a private service instead.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_http_port, w8/388); en starts with the space that joins it to the sentence before; params are the sentence's values: ports is the comma-separated list of open ports that answer no HTTP",
  },
  "deploys.reason.hint_reserved_port": {
    message:
      " The process serves HTTP on port {reserved}, which is reserved: bind the server to 0.0.0.0 on the port in PORT ({port}) instead. The reserved ports are 18012, 18013 and 19099.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_reserved_port, w1/m659); en starts with the space that joins it to the sentence before; params are the sentence's values: reserved is the reserved port the process serves HTTP on, port the one in PORT",
  },
  "deploys.reason.hint_crash_port_bind": {
    message:
      " If the crash is a port bind: the process must listen on $PORT ({port}).",
    description:
      "A startup crash's port advice, by its code (bex-api DeployReason part hint_crash_port_bind, w8/386); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_early_exit_port": {
    message:
      " The server must keep listening on $PORT ({port}) until it receives SIGTERM.",
    description:
      "A startup crash's port advice, by its code (bex-api DeployReason part hint_early_exit_port, w8/386); en starts with the space that joins it to the sentence before; params are the sentence's values",
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
  "deploys.factBuild": {
    message: "Build",
    description:
      "Deploy page: label of the fact naming the Build Source build ID (bld-…) this deploy ran",
  },
  "deploys.factBuildSource": {
    message: "Build Source",
    description:
      "Deploy page: label of the fact naming the linked Build Source (name and bds-… ID)",
  },
  "deploys.buildIdLabel": {
    message: "Build ID",
    description: "Deploy page: accessible field name for copying the Build ID",
  },
  "deploys.buildIdCopied": {
    message: "Build ID copied",
    description: "Deploy page: toast shown after copying the Build ID",
  },
  "deploys.buildIdCopyFailed": {
    message: "Couldn't copy the Build ID",
    description: "Deploy page: toast shown when copying the Build ID fails",
  },
  "deploys.buildSourceIdLabel": {
    message: "Build Source ID",
    description:
      "Deploy page: accessible field name for copying the Build Source ID",
  },
  "deploys.buildSourceIdCopied": {
    message: "Build Source ID copied",
    description: "Deploy page: toast shown after copying the Build Source ID",
  },
  "deploys.buildSourceIdCopyFailed": {
    message: "Couldn't copy the Build Source ID",
    description:
      "Deploy page: toast shown when copying the Build Source ID fails",
  },
  "deploys.noticesTitle": {
    message: "Notices",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): the section heading over the list, shown in small capitals",
  },
  "deploys.noticeTroubleshoot": {
    message: "Read our {link} for common ways to troubleshoot your deploy.",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): the line under a failed deploy's reason; {link} is the noticeTroubleshootLink link to bex's troubleshooting docs",
  },
  "deploys.noticeTroubleshootLink": {
    message: "docs",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): the link text inside noticeTroubleshoot",
  },
  "deploys.noticeBuildCacheCleared": {
    message: "Build cache cleared",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): the deploy was opened with Clear build cache & deploy",
  },
  "deploys.noticeFreePlan": {
    message: "Updated to a free compute plan",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): a settings deploy that moved the service from a paid plan to the free one",
  },
  "deploys.noticeMaintenance": {
    message:
      "bex periodically runs zero-downtime maintenance to ensure platform reliability.",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): a deploy the platform itself started (trigger deployed_by_render)",
  },
  "deploys.noticeRolledBackTo": {
    message: "Rolled back to {deploy}",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): a rollback deploy; {deploy} is the target deploy's id, a link",
  },
  "deploys.noticeBuildGone": {
    message: "Rollback is not available because this build no longer exists.",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): a past deploy whose build is no longer stored",
  },
  "deploys.noticeSyncedFrom": {
    message: "Synced from {blueprint}",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): a deploy a Blueprint sync opened; {blueprint} is the Blueprint's id, a link",
  },
  "deploys.noticeInProgress": {
    message: "{deploy} is in progress",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): a queued deploy; {deploy} is the noticeAnotherDeploy link to the deploy in progress",
  },
  "deploys.noticeAnotherDeploy": {
    message: "Another deploy",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): the link text inside noticeInProgress",
  },
  "deploys.noticeCancelPolicy": {
    message:
      "To avoid canceling active deploys when new ones are triggered, adjust your {policy}",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): under a deploy a newer one canceled, when the workspace's Overlapping Deploy Policy is Override; {policy} is the noticePolicyLink link to Workspace Settings",
  },
  "deploys.noticeQueuePolicy": {
    message: "To avoid queuing deploys, adjust your {policy}",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): under a queued deploy, when the workspace's Overlapping Deploy Policy is Wait; {policy} is the noticePolicyLink link to Workspace Settings",
  },
  "deploys.noticePolicyLink": {
    message: "overlapping deploy policy.",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): the link text inside noticeCancelPolicy and noticeQueuePolicy, the workspace's Overlapping Deploy Policy",
  },
  "deploys.noticeHealthWaiting": {
    message: "Waiting for internal {link} to return a successful response code",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): a deploy rolling out on a service with a Health Check Path; {link} is the noticeHealthLink link to the health check docs",
  },
  "deploys.noticeHealthWaitingAt": {
    message:
      "Waiting for internal {link} to return a successful response code at: {address}",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): noticeHealthWaiting with the address checked; {address} is host:port/path in code type",
  },
  "deploys.noticeHealthTimedOut": {
    message:
      "Timed out after waiting for internal {link} to return a successful response code",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): a failed deploy that ran out of its rollout window, on a service with a Health Check Path; {link} as in noticeHealthWaiting",
  },
  "deploys.noticeHealthTimedOutAt": {
    message:
      "Timed out after waiting for internal {link} to return a successful response code at: {address}",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): noticeHealthTimedOut with the address checked; {address} is host:port/path in code type",
  },
  "deploys.noticeHealthLink": {
    message: "health check",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): the link text inside the health check notices",
  },
  "deploys.reservedPortWarning": {
    message:
      "It looks like your service might be listening on port {port}, which is a reserved port for {link} Please configure your service to listen on a port that is not part of our reserved ports: {ports}.",
    description:
      "Deploy page banner (w1/m659, Render's free-plan warning): the instance serves HTTP on a reserved port; {port} is that port, {link} the reservedPortWarningLink link to bex's pricing, {ports} the reserved ports",
  },
  "deploys.reservedPortWarningLink": {
    message: "free compute plans.",
    description: "The link text inside reservedPortWarning (w1/m659)",
  },
  "deploys.rollbackTitle": {
    message: "Confirm Rollback",
    description:
      "Rollback page (w1/m646): the block's title, Render's \"Confirm Rollback\"",
  },
  "deploys.rollbackLead": {
    message: "Rolling back reverts certain configuration changes.",
    description:
      "Rollback page: the block's lead sentence, before the Learn more link",
  },
  "deploys.rollbackLearnMore": {
    message: "Learn more.",
    description:
      "Rollback page: link to the rollbacks documentation after the lead sentence",
  },
  "deploys.rollbackLoading": {
    message: "Loading the configuration changes",
    description:
      "Rollback page: accessible name of the spinner while the comparison loads",
  },
  "deploys.rollbackUnavailable": {
    message:
      "Unable to load configuration for the current live deploy. Rolling back to this deploy could cause unanticipated configuration changes.",
    description:
      "Rollback page: Render's sentence when the comparison failed or either deploy's configuration is not recorded",
  },
  "deploys.rollbackNoChanges": {
    message: "No configuration changes since this deploy.",
    description:
      "Rollback page: nothing differs between the live deploy and this one",
  },
  "deploys.rollbackRowStartCommand": {
    message: "Start Command",
    description: "Rollback page: row label for a changed start command",
  },
  "deploys.rollbackRowPreDeployCommand": {
    message: "Pre-Deploy Command",
    description: "Rollback page: row label for a changed pre-deploy command",
  },
  "deploys.rollbackRowDockerCommand": {
    message: "Docker Command",
    description: "Rollback page: row label for a changed Docker command",
  },
  "deploys.rollbackRowHealthCheckPath": {
    message: "Health Check Path",
    description: "Rollback page: row label for a changed health check path",
  },
  "deploys.rollbackRowInstances": {
    message: "Instances",
    description: "Rollback page: row label for a changed instance count",
  },
  "deploys.rollbackRowEnvVars": {
    message: "Environment Variables",
    description:
      "Rollback page: row label for the changed environment variables",
  },
  "deploys.rollbackRowEnvGroup": {
    message: "Environment Group",
    description: "Rollback page: row label for environment group changes",
  },
  "deploys.rollbackRowSecretFiles": {
    message: "Secret Files",
    description: "Rollback page: row label for changed secret files",
  },
  "deploys.rollbackRowPlan": {
    message: "Plan",
    description: "Rollback page: row label for the plan warning",
  },
  "deploys.rollbackNone": {
    message: "none",
    description:
      "Rollback page: a command or path that is not set, on either side of a change",
  },
  "deploys.rollbackChange": {
    message: "Configuration change",
    description: "Rollback page: accessible name of one before/after change",
  },
  "deploys.rollbackRemoved": {
    message: "Removed",
    description:
      "Rollback page: accessible name of a change's minus line (the live value)",
  },
  "deploys.rollbackAdded": {
    message: "Added",
    description:
      "Rollback page: accessible name of a change's plus line (the value after the rollback)",
  },
  "deploys.rollbackEnvKey": {
    message: "Key",
    description:
      "Rollback page: environment variable table's key column header",
  },
  "deploys.rollbackEnvValue": {
    message: "Value",
    description:
      "Rollback page: environment variable table's value column header; every value is masked",
  },
  "deploys.rollbackSecretFileAdded": {
    message:
      "Secret file {name} was added after this deploy. The rollback will ignore it.",
    description:
      "Rollback page: a secret file that did not exist at this deploy; {name} is the file name",
  },
  "deploys.rollbackSecretFileDeleted": {
    message:
      "Secret file {name} was deleted after this deploy. The rollback will restore the file.",
    description:
      "Rollback page: a secret file deleted since this deploy; {name} is the file name",
  },
  "deploys.rollbackSecretFileChanged": {
    message:
      "Secret file {name} was changed after this deploy. The rollback will revert to the original contents.",
    description:
      "Rollback page: a secret file whose contents changed since this deploy; {name} is the file name",
  },
  "deploys.rollbackGroupUnlink": {
    message:
      "The rollback will unlink environment group {name} which was added to the service after this deploy.",
    description:
      "Rollback page: an environment group linked since this deploy; {name} is the group name",
  },
  "deploys.rollbackGroupLink": {
    message:
      "The rollback will link environment group {name} which was removed from the service after this deploy.",
    description:
      "Rollback page: an environment group unlinked since this deploy; {name} is the group name",
  },
  "deploys.rollbackGroupDeleted_one": {
    message:
      "This deploy was linked to {n} environment group that has since been deleted. It cannot be restored.",
    description:
      "Rollback page: one group this deploy linked was deleted; {n} is the count",
  },
  "deploys.rollbackGroupDeleted_other": {
    message:
      "This deploy was linked to {n} environment groups that have since been deleted. They cannot be restored.",
    description:
      "Rollback page: several groups this deploy linked were deleted; {n} is the count",
  },
  "deploys.rollbackGroupCurrentValues": {
    message:
      "The rollback will use current keys and values for environment groups linked to this deploy.",
    description:
      "Rollback page: shown whenever this deploy linked any environment group",
  },
  "deploys.rollbackStaticGroupNotUse": {
    message:
      "The rollback will not use environment group {name} which was added to the site after this build.",
    description:
      "Rollback page, static site: an environment group linked since this build, which the rollback's files never used; {name} is the group name",
  },
  "deploys.rollbackStaticGroupUse": {
    message:
      "The rollback will use environment group {name} which was removed from the site after this build.",
    description:
      "Rollback page, static site: an environment group unlinked since this build, whose values the rollback's files hold; {name} is the group name",
  },
  "deploys.rollbackStaticGroupDeleted_one": {
    message:
      "This build used {n} environment group that has since been deleted. The rollback will use its original values.",
    description:
      "Rollback page, static site: one group this build used was deleted; {n} is the count",
  },
  "deploys.rollbackStaticGroupDeleted_other": {
    message:
      "This build used {n} environment groups that have since been deleted. The rollback will use their original values.",
    description:
      "Rollback page, static site: several groups this build used were deleted; {n} is the count",
  },
  "deploys.rollbackStaticGroupOriginalValues": {
    message:
      "The rollback will use the original values for environment groups used for this build.",
    description:
      "Rollback page, static site: shown whenever this build used any environment group",
  },
  "deploys.rollbackPlanWarning": {
    message:
      "Your current plan has lower CPU and memory allocations than the plan originally used for this deploy. You may need to switch to a different plan for the rollback to succeed.",
    description:
      "Rollback page: the saved plan is smaller than the one this deploy ran on",
  },
  "deploys.rollbackStaticNotice": {
    message: "Changes made to {what} will not be rolled back.",
    description:
      "Rollback page, static site: {what} is deploys.rollbackStaticNoticeWhat, in bold",
  },
  "deploys.rollbackStaticNoticeWhat": {
    message: "redirects, rewrites, and headers",
    description: "Rollback page, static site: what a rollback keeps current",
  },
  "deploys.rollbackAutoDeployTitle": {
    message: "Auto-Deploy",
    description:
      "Rollback page: title of the warning that the rollback turns auto-deploy off",
  },
  "deploys.rollbackAutoDeployBody": {
    message: "will be disabled for this service to prevent accidental deploys.",
    description:
      "Rollback page: the auto-deploy warning's text, after its title",
  },
  "deploys.rollbackConfirm": {
    message: "Rollback to this deploy",
    description: "Rollback page: the button that starts the rollback",
  },
  "deploys.rollbackCancel": {
    message: "Cancel",
    description:
      "Rollback page: the button that goes back to the deploy without rolling back",
  },
  "deploys.rollbackErrorTitle": {
    message: "Error:",
    description: "Rollback page: label before the reason a rollback failed",
  },
};

export default enDeploys;
