import type { TranslationEntry } from "@/i18n";

// A sandbox group's pages (w1/m658): Render's Sandboxes list, sandbox drawer,
// Snapshots, Custom images and Settings. Render's own words where its
// dashboard has them (docs/render-artifacts/sandbox-groups.md § The dashboard).
const en: Record<string, TranslationEntry> = {
  "sandboxGroupPages.navSandboxes": {
    message: "Sandboxes",
    description: "Sidebar entry and heading: a sandbox group's sandboxes",
  },
  "sandboxGroupPages.navSnapshots": {
    message: "Snapshots",
    description: "Sidebar entry and heading: a sandbox group's snapshots",
  },
  "sandboxGroupPages.navImages": {
    message: "Custom images",
    description:
      "Sidebar entry and heading: the images imported into a sandbox group",
  },
  "sandboxGroupPages.navSettings": {
    message: "Settings",
    description: "Sidebar entry and heading: a sandbox group's settings",
  },
  "sandboxGroupPages.statusCreating": {
    message: "Creating",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusRunning": {
    message: "Running",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusResuming": {
    message: "Resuming",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusSuspending": {
    message: "Suspending",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusSuspended": {
    message: "Suspended",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusErrored": {
    message: "Errored",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusTerminated": {
    message: "Terminated",
    description: "Sandbox status",
  },
  "sandboxGroupPages.rangeLabel": {
    message: "Time range",
    description: "Accessible name of the time range picker",
  },
  "sandboxGroupPages.rangeHour": {
    message: "Last hour",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeDay": {
    message: "Last 24 hours",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeWeek": {
    message: "Last 7 days",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeMonth": {
    message: "Last 30 days",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeAll": {
    message: "All time",
    description: "Time range option",
  },
  "sandboxGroupPages.searchSandboxes": {
    message: "Search by name, sandbox ID, or status",
    description: "Placeholder of the Sandboxes search",
  },
  "sandboxGroupPages.searchSnapshots": {
    message: "Search snapshots",
    description: "Placeholder of the Snapshots search",
  },
  "sandboxGroupPages.searchImages": {
    message: "Search images",
    description: "Placeholder of the Custom images search",
  },
  "sandboxGroupPages.pauseLive": {
    message: "Pause live updates",
    description: "Button that stops the list refreshing",
  },
  "sandboxGroupPages.resumeLive": {
    message: "Resume live updates",
    description: "Button that restarts the list refreshing",
  },
  "sandboxGroupPages.showHideColumns": {
    message: "Show or hide columns",
    description: "Accessible name of the columns menu",
  },
  "sandboxGroupPages.summaryTotal": {
    message: "Total",
    description: "Summary: runs that ended in the time range",
  },
  "sandboxGroupPages.summaryErrored": {
    message: "Errored",
    description: "Summary: share of runs that ended in an error",
  },
  "sandboxGroupPages.summaryErroredTip": {
    message: "Share of sandboxes in the time range that ended in an error.",
    description: "Tooltip of the Errored summary",
  },
  "sandboxGroupPages.summaryRunning": {
    message: "Running",
    description: "Summary: sandboxes active now",
  },
  "sandboxGroupPages.summaryRunningTip": {
    message:
      "Creating, running, resuming, or suspending now, whatever the time range. These count against the concurrency limit.",
    description: "Tooltip of the Running summary",
  },
  "sandboxGroupPages.loadRunsError": {
    message: "Couldn't load sandbox runs",
    description: "The run chart failed to load",
  },
  "sandboxGroupPages.runsChartLabel": {
    message:
      "Sandbox runs that ended: {succeeded} succeeded, {errored} errored",
    description: "Accessible name of the run chart",
  },
  "sandboxGroupPages.runsSucceeded": {
    message: "Succeeded",
    description: "Chart legend: runs that ended without an error",
  },
  "sandboxGroupPages.runsErrored": {
    message: "Errored",
    description: "Chart legend: runs that ended in an error",
  },
  "sandboxGroupPages.selectedCount_one": {
    message: "{count} sandbox selected",
    description: "How many rows are selected",
  },
  "sandboxGroupPages.selectedCount_other": {
    message: "{count} sandboxes selected",
    description: "How many rows are selected",
  },
  "sandboxGroupPages.terminateSelected": {
    message: "Terminate",
    description: "Button: terminate the selected sandboxes",
  },
  "sandboxGroupPages.terminateTitle_one": {
    message: "Terminate {count} sandbox?",
    description: "Terminate confirmation title",
  },
  "sandboxGroupPages.terminateTitle_other": {
    message: "Terminate {count} sandboxes?",
    description: "Terminate confirmation title",
  },
  "sandboxGroupPages.terminateDescription_one": {
    message:
      "{count} sandbox stops, and anything in it that isn't in a snapshot is lost.",
    description: "Terminate confirmation body",
  },
  "sandboxGroupPages.terminateDescription_other": {
    message:
      "The {count} sandboxes stop, and anything in them that isn't in a snapshot is lost.",
    description: "Terminate confirmation body",
  },
  "sandboxGroupPages.terminateConfirm": {
    message: "Terminate",
    description: "Terminate confirmation button",
  },
  "sandboxGroupPages.terminateFailed_one": {
    message: "Failed to terminate {count} sandbox. Please try again.",
    description: "Toast: a terminate failed",
  },
  "sandboxGroupPages.terminateFailed_other": {
    message: "Failed to terminate {count} sandboxes. Please try again.",
    description: "Toast: terminates failed",
  },
  "sandboxGroupPages.terminateStarted_one": {
    message: "Terminating {count} sandbox",
    description: "Toast: a terminate was accepted",
  },
  "sandboxGroupPages.terminateStarted_other": {
    message: "Terminating {count} sandboxes",
    description: "Toast: terminates were accepted",
  },
  "sandboxGroupPages.terminateSandbox": {
    message: "Terminate sandbox",
    description: "Drawer menu item",
  },
  "sandboxGroupPages.sandboxActions": {
    message: "Sandbox actions",
    description: "Accessible name of the drawer's menu",
  },
  "sandboxGroupPages.loadSandboxError": {
    message: "Couldn't load the sandbox",
    description: "A sandbox's drawer failed to load",
  },
  "sandboxGroupPages.loadSandboxesError": {
    message: "Couldn't load sandboxes",
    description: "The list failed to load",
  },
  "sandboxGroupPages.noMatchingSandboxes": {
    message: "No matching sandboxes",
    description: "Empty state with filters",
  },
  "sandboxGroupPages.noMatchingSandboxesHelp": {
    message: "Try another search, or widen the time range.",
    description: "Empty state with filters",
  },
  "sandboxGroupPages.clearFilters": {
    message: "Clear filters",
    description: "Button: reset the search and time range",
  },
  "sandboxGroupPages.firstSandboxTitle": {
    message: "Create your first sandbox",
    description: "Empty state title of the Sandboxes page",
  },
  "sandboxGroupPages.firstSandboxDescription": {
    message:
      "Spin up a sandbox from the CLI, or set up your code to create sandboxes programmatically with the TypeScript and Python SDKs.",
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
    message: "Install or update to the latest CLI",
    description: "Starter step",
  },
  "sandboxGroupPages.starterHomebrew": {
    message: "Homebrew",
    description: "Install method",
  },
  "sandboxGroupPages.starterDirect": {
    message: "Direct download",
    description: "Install method",
  },
  "sandboxGroupPages.starterLogin": {
    message: "Log in and use this workspace",
    description: "Starter step",
  },
  "sandboxGroupPages.starterCreateSandbox": {
    message: "Create a sandbox with the defaults",
    description: "Starter step",
  },
  "sandboxGroupPages.starterApiKey": {
    message: "Set your API key",
    description: "Starter step",
  },
  "sandboxGroupPages.starterApiKeyHelp": {
    message:
      "Run this in the terminal you'll run your code from. The SDK reads your key from the RENDER_API_KEY environment variable. Replace the placeholder with a bex API key.",
    description: "Starter step help",
  },
  "sandboxGroupPages.starterInstallSdk": {
    message: "Set up a project and install the SDK",
    description: "Starter step",
  },
  "sandboxGroupPages.starterPythonNeeds": {
    message:
      "Needs Python 3.10 or later. -U upgrades an older SDK; 1.4.0 and earlier have no sandbox support.",
    description: "Starter step help",
  },
  "sandboxGroupPages.starterNodeNeeds": {
    message:
      "Needs Node.js 18 or later. @latest also upgrades an older SDK that is already installed.",
    description: "Starter step help",
  },
  "sandboxGroupPages.starterFirstSandboxFile": {
    message: "Initialize your first sandbox",
    description: "Starter step",
  },
  "sandboxGroupPages.starterRun": {
    message: "Run your code locally",
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
  "sandboxGroupPages.colCreated": { message: "Created", description: "Column" },
  "sandboxGroupPages.colDuration": {
    message: "Duration",
    description: "Column",
  },
  "sandboxGroupPages.colPlan": { message: "Plan", description: "Column" },
  "sandboxGroupPages.colSource": {
    message: "Started from",
    description: "Column: the image or snapshot a sandbox started from",
  },
  "sandboxGroupPages.selectAll": {
    message: "Select every sandbox on this page",
    description: "Accessible name of the select-all checkbox",
  },
  "sandboxGroupPages.selectSandbox": {
    message: "Select {name}",
    description: "Accessible name of a row's checkbox",
  },
  "sandboxGroupPages.baseImage": {
    message: "Base image",
    description: "A sandbox started from the default image",
  },
  "sandboxGroupPages.importedImage": {
    message: "Imported image",
    description: "A sandbox started from an imported image",
  },
  "sandboxGroupPages.previousPage": {
    message: "Previous",
    description: "Paging button",
  },
  "sandboxGroupPages.nextPage": {
    message: "Next",
    description: "Paging button",
  },
  "sandboxGroupPages.switchedToRunning": {
    message: "Your selected sandbox stopped. Switched to a running one.",
    description: "Toast when the open sandbox stops",
  },
  "sandboxGroupPages.copied": { message: "Copied", description: "Copy toast" },
  "sandboxGroupPages.copyError": {
    message: "Couldn't copy",
    description: "Copy failed toast",
  },
  "sandboxGroupPages.copySandboxId": {
    message: "Copy sandbox ID",
    description: "Copy button",
  },
  "sandboxGroupPages.copySnapshotId": {
    message: "Copy snapshot ID",
    description: "Copy button",
  },
  "sandboxGroupPages.tabEvents": {
    message: "Events",
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
    message: "Versions",
    description: "Image drawer tab",
  },
  "sandboxGroupPages.sandboxGoneTitle": {
    message: "Sandbox not found",
    description: "A sandbox that does not exist here",
  },
  "sandboxGroupPages.sandboxGoneDescription": {
    message: "{id} isn't a sandbox you can see in this group.",
    description: "A sandbox that does not exist here",
  },
  "sandboxGroupPages.loadingEvents": {
    message: "Loading events",
    description: "Events tab loading",
  },
  "sandboxGroupPages.loadEventsError": {
    message: "Couldn't load events. Try again shortly.",
    description: "Events tab error",
  },
  "sandboxGroupPages.noEvents": {
    message: "No events yet.",
    description: "Events tab empty",
  },
  "sandboxGroupPages.loadEarlierEvents": {
    message: "Load earlier events",
    description: "Button: the next page of events",
  },
  "sandboxGroupPages.loadingEarlierEvents": {
    message: "Loading earlier events",
    description: "Loading the next page of events",
  },
  "sandboxGroupPages.eventCreated": {
    message: "Created",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventReady": {
    message: "Ready",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventSuspending": {
    message: "Suspending",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventSuspended": {
    message: "Suspended",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventResuming": {
    message: "Resuming",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventTerminated": {
    message: "Terminated",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventErrored": {
    message: "Errored",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventCommand": {
    message: "Command ({operation})",
    description: "An execution event that ran a command",
  },
  "sandboxGroupPages.eventFileTransfer": {
    message: "File transfer ({operation})",
    description: "An execution event that moved a file",
  },
  "sandboxGroupPages.exitCode": {
    message: "Exit code {code}",
    description: "A command's exit code",
  },
  "sandboxGroupPages.eventSnapshot": {
    message: "Snapshot",
    description: "A snapshot event",
  },
  "sandboxGroupPages.viewSnapshot": {
    message: "View snapshot",
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
    message: "Idle timeout",
    description: "Trigger: the idle timeout",
  },
  "sandboxGroupPages.triggerSystem": {
    message: "bex",
    description: "Trigger: the platform itself",
  },
  "sandboxGroupPages.triggerConnection": {
    message: "New command or file transfer",
    description: "Trigger: a resume a command started",
  },
  "sandboxGroupPages.loading": {
    message: "Loading",
    description: "Loading text",
  },
  "sandboxGroupPages.loadSnapshotsError": {
    message: "Couldn't load snapshots",
    description: "Snapshots failed to load",
  },
  "sandboxGroupPages.noSnapshotsRunning": {
    message:
      "No snapshots of this sandbox yet. Capture one with the CLI or an SDK.",
    description: "Snapshots tab of a running sandbox",
  },
  "sandboxGroupPages.noSnapshotsEnded": {
    message: "No snapshots were taken of this sandbox.",
    description: "Snapshots tab of an ended sandbox",
  },
  "sandboxGroupPages.detailSandboxId": {
    message: "Sandbox ID",
    description: "Detail label",
  },
  "sandboxGroupPages.detailGroup": {
    message: "Group",
    description: "Detail label",
  },
  "sandboxGroupPages.detailPlan": {
    message: "Plan",
    description: "Detail label",
  },
  "sandboxGroupPages.planSize": {
    message: "{plan} ({cpus} CPU, {memory} GB)",
    description: "A plan with its size",
  },
  "sandboxGroupPages.detailCreated": {
    message: "Created",
    description: "Detail label",
  },
  "sandboxGroupPages.detailEnded": {
    message: "Ended",
    description: "Detail label",
  },
  "sandboxGroupPages.detailDuration": {
    message: "Duration",
    description: "Detail label",
  },
  "sandboxGroupPages.detailTimeout": {
    message: "Timeout",
    description: "Detail label",
  },
  "sandboxGroupPages.detailIdleTimeout": {
    message: "Idle timeout",
    description: "Detail label",
  },
  "sandboxGroupPages.off": {
    message: "Off",
    description: "A timeout that is off",
  },
  "sandboxGroupPages.detailNetwork": {
    message: "Network",
    description: "Detail label",
  },
  "sandboxGroupPages.networkAllowAll": {
    message: "Allow all",
    description: "Network policy",
  },
  "sandboxGroupPages.networkDenyAll": {
    message: "Deny all",
    description: "Network policy",
  },
  "sandboxGroupPages.networkAllowList": {
    message: "Allow list",
    description: "Network policy",
  },
  "sandboxGroupPages.detailRegion": {
    message: "Region",
    description: "Detail label",
  },
  "sandboxGroupPages.detailSource": {
    message: "Started from",
    description: "Detail label",
  },
  "sandboxGroupPages.goToSnapshot": {
    message: "Go to snapshot",
    description: "Link to the snapshot a sandbox started from",
  },
  "sandboxGroupPages.goToImage": {
    message: "Go to image",
    description: "Link to the image a sandbox started from",
  },
  "sandboxGroupPages.detailTrigger": {
    message: "Trigger",
    description: "Detail label: what started the sandbox",
  },
  "sandboxGroupPages.detailUser": {
    message: "User",
    description: "Detail label: who started the sandbox",
  },
  "sandboxGroupPages.detailError": {
    message: "Error",
    description: "Detail label",
  },
  "sandboxGroupPages.errorUnknown": {
    message:
      "bex couldn't tell why this sandbox errored. Try again, or contact support with the sandbox ID.",
    description: "An errored sandbox with no known reason",
  },
  "sandboxGroupPages.snapshotCapturing": {
    message: "Capturing",
    description: "Snapshot status",
  },
  "sandboxGroupPages.snapshotAvailable": {
    message: "Available",
    description: "Snapshot status",
  },
  "sandboxGroupPages.snapshotFailed": {
    message: "Failed",
    description: "Snapshot status",
  },
  "sandboxGroupPages.snapshotFailureInternal": {
    message: "An internal error stopped the capture.",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailureTimeout": {
    message: "This capture timed out.",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailureSourceTerminated": {
    message: "The source sandbox was terminated before the capture finished.",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailureSourceErrored": {
    message: "The source sandbox errored before the capture finished.",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailedHelp": {
    message:
      "Create a new snapshot to try again. New sandboxes can't start from it.",
    description: "After a snapshot failure",
  },
  "sandboxGroupPages.noMatchingSnapshots": {
    message: "No matching snapshots",
    description: "Empty state with filters",
  },
  "sandboxGroupPages.firstSnapshotTitle": {
    message: "Create your first snapshot",
    description: "Empty state title of the Snapshots page",
  },
  "sandboxGroupPages.firstSnapshotDescription": {
    message:
      "Capture a snapshot to save a sandbox's files. Start new sandboxes from the same point without setting everything up again.",
    description: "Empty state body of the Snapshots page",
  },
  "sandboxGroupPages.firstSnapshotNeedsRunning": {
    message: "You need a running sandbox to create a snapshot.",
    description: "Empty state note",
  },
  "sandboxGroupPages.starterSnapshotSandbox": {
    message: "Snapshot a running sandbox",
    description: "Starter step",
  },
  "sandboxGroupPages.starterStartFromSnapshot": {
    message: "Create sandboxes from the snapshot",
    description: "Starter step",
  },
  "sandboxGroupPages.starterSnapshotCode": {
    message: "Set up your code",
    description: "Starter step",
  },
  "sandboxGroupPages.colSnapshot": {
    message: "Snapshot",
    description: "Column",
  },
  "sandboxGroupPages.colSourceSandbox": {
    message: "Source sandbox",
    description: "Column and detail label",
  },
  "sandboxGroupPages.colSize": {
    message: "Size",
    description: "Column and detail label",
  },
  "sandboxGroupPages.colCaptured": {
    message: "Captured",
    description: "Column and detail label",
  },
  "sandboxGroupPages.colExpires": {
    message: "Expires",
    description: "Column and detail label",
  },
  "sandboxGroupPages.never": {
    message: "Never",
    description: "A snapshot that never expires",
  },
  "sandboxGroupPages.snapshotGoneTitle": {
    message: "Snapshot not found",
    description: "A snapshot that does not exist here",
  },
  "sandboxGroupPages.snapshotGoneDescription": {
    message:
      "{id} isn't a snapshot you can see in this workspace. It may have been deleted or expired.",
    description: "A snapshot that does not exist here",
  },
  "sandboxGroupPages.detailType": {
    message: "Type",
    description: "Detail label: snapshot kind",
  },
  "sandboxGroupPages.kindFilesystem": {
    message: "Filesystem",
    description: "Snapshot kind",
  },
  "sandboxGroupPages.kindRuntime": {
    message: "Runtime",
    description: "Snapshot kind",
  },
  "sandboxGroupPages.detailResourceConstraint": {
    message: "Resource constraint",
    description: "Detail label: the plan a snapshot needs",
  },
  "sandboxGroupPages.detailLifecycle": {
    message: "Lifecycle",
    description: "Detail label",
  },
  "sandboxGroupPages.lifecycleKept": {
    message: "Kept until deleted",
    description: "A snapshot with no time to live",
  },
  "sandboxGroupPages.lifecycleAfterUse": {
    message: "Expires {days} days after its last use",
    description: "A snapshot's time to live",
  },
  "sandboxGroupPages.noSandboxesFromSnapshot": {
    message: "No sandboxes have started from this snapshot.",
    description: "Snapshot drawer's sandboxes",
  },
  "sandboxGroupPages.restoreTitle": {
    message: "Start a sandbox from this snapshot",
    description: "Snapshot drawer starter code",
  },
  "sandboxGroupPages.deleteSnapshot": {
    message: "Delete snapshot",
    description: "Button",
  },
  "sandboxGroupPages.deleteSnapshotTitle": {
    message: "Delete snapshot?",
    description: "Confirmation title",
  },
  "sandboxGroupPages.deleteSnapshotDescription": {
    message:
      "This permanently deletes {name}. New sandboxes can't start from it. Sandboxes already started from it keep running.",
    description: "Confirmation body",
  },
  "sandboxGroupPages.deleteSnapshotError": {
    message: "Couldn't delete the snapshot. Try again.",
    description: "Toast",
  },
  "sandboxGroupPages.snapshotDeleted": {
    message: "Deleted snapshot {name}",
    description: "Toast",
  },
  "sandboxGroupPages.imageQueued": {
    message: "Queued",
    description: "Image status",
  },
  "sandboxGroupPages.imageImporting": {
    message: "Importing",
    description: "Image status",
  },
  "sandboxGroupPages.imageAvailable": {
    message: "Available",
    description: "Image status",
  },
  "sandboxGroupPages.imageUnavailable": {
    message: "Unavailable",
    description: "Image status: no version can start a sandbox",
  },
  "sandboxGroupPages.imageFailed": {
    message: "Failed",
    description: "Image version status",
  },
  "sandboxGroupPages.imageLatest": {
    message: "Latest",
    description: "The newest available version of an image",
  },
  "sandboxGroupPages.imageImportingHelp": {
    message:
      "Sandboxes start from the latest available version until this one finishes.",
    description: "A version still importing",
  },
  "sandboxGroupPages.imageFailureTimeout": {
    message: "The import took too long.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureRetries": {
    message: "The import failed after several tries.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureCredentials": {
    message: "bex couldn't pull the image. Check the registry credentials.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureNotFound": {
    message: "bex couldn't find the image. Check the image name and tag.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailurePull": {
    message: "bex couldn't pull the image.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailurePlatform": {
    message: "The image is for {platform}. Sandboxes support only linux/amd64.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureMissing": {
    message:
      "The image is missing {what}, which sandboxes need to start. Use an image with a shell, like a slim or Alpine variant.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureConvert": {
    message: "bex couldn't convert the image. Try importing it again.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureLayers": {
    message: "bex couldn't read the image's layers.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureStart": {
    message: "The import couldn't start.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureStopped": {
    message: "The import stopped before it finished.",
    description: "Image failure",
  },
  "sandboxGroupPages.loadImagesError": {
    message: "Couldn't load images",
    description: "Images failed to load",
  },
  "sandboxGroupPages.noMatchingImages": {
    message: "No matching images",
    description: "Empty state with a search",
  },
  "sandboxGroupPages.firstImageTitle": {
    message: "Import your first image",
    description: "Empty state title of the Custom images page",
  },
  "sandboxGroupPages.firstImageDescription": {
    message:
      "Import an image from a registry so sandboxes can start from it. Import one with the CLI or the SDK, then start sandboxes from it.",
    description: "Empty state body of the Custom images page",
  },
  "sandboxGroupPages.starterImportImage": {
    message: "Import an image (linux/amd64)",
    description: "Starter step",
  },
  "sandboxGroupPages.starterStartFromImage": {
    message: "Create sandboxes from the image",
    description: "Starter step",
  },
  "sandboxGroupPages.starterImportCode": {
    message: "Import images",
    description: "Starter step",
  },
  "sandboxGroupPages.colImage": { message: "Image", description: "Column" },
  "sandboxGroupPages.colSourceImage": {
    message: "Source",
    description: "Column: the registry image",
  },
  "sandboxGroupPages.colLatestAvailable": {
    message: "Latest available",
    description: "Column: the newest available version's digest",
  },
  "sandboxGroupPages.colUpdated": {
    message: "Updated",
    description: "Column: the newest import",
  },
  "sandboxGroupPages.imageGoneTitle": {
    message: "Image not found",
    description: "An image name with no versions",
  },
  "sandboxGroupPages.imageGoneDescription": {
    message: "This group has no image named {name}.",
    description: "An image name with no versions",
  },
  "sandboxGroupPages.detailLatestAvailable": {
    message: "Latest available version",
    description: "Detail label",
  },
  "sandboxGroupPages.detailImageSource": {
    message: "Source",
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
    message: "Version ID",
    description: "Detail label",
  },
  "sandboxGroupPages.noSandboxesFromImage": {
    message: "No sandboxes have started from this image.",
    description: "Image drawer's sandboxes",
  },
  "sandboxGroupPages.general": {
    message: "General",
    description: "Settings section",
  },
  "sandboxGroupPages.makeDefault": {
    message: "Make default",
    description: "Button: make this the workspace's default group",
  },
  "sandboxGroupPages.madeDefault": {
    message: "{name} is now the default sandbox group",
    description: "Toast",
  },
  "sandboxGroupPages.save": { message: "Save", description: "Button" },
  "sandboxGroupPages.limitSaved": {
    message: "Saved the concurrency limit",
    description: "Toast",
  },
  "sandboxGroupPages.brokeredCredentials": {
    message: "Brokered credentials",
    description: "What a group delete removes",
  },
  "sandboxGroupPages.deleteGroup": {
    message: "Delete sandbox group",
    description: "Button and section",
  },
  "sandboxGroupPages.deleteGroupHelp": {
    message:
      "Delete this group, with its sandboxes, snapshots, images and credentials.",
    description: "Delete section help",
  },
  "sandboxGroupPages.deleteGroupDefault": {
    message:
      "The workspace's default group can't be deleted. Make another group the default first.",
    description: "Delete section help for the default group",
  },
  "sandboxGroupPages.deleteGroupTitle": {
    message: "Delete sandbox group?",
    description: "Confirmation title",
  },
  "sandboxGroupPages.deleteGroupDescription": {
    message:
      "This sandbox group, {name}, will be permanently deleted, along with its run history, logs, and settings:",
    description: "Confirmation body",
  },
  "sandboxGroupPages.deleteGroupStops": {
    message:
      "Anything using this group via SDK, CLI, or API will stop working.",
    description: "Confirmation body",
  },
  "sandboxGroupPages.groupDeleted": {
    message: "Deleted sandbox group {name}",
    description: "Toast",
  },
  "sandboxGroupPages.linkLoadError": {
    message: "Couldn't open the link. Try again shortly.",
    description: "A link by id failed to resolve",
  },
  "sandboxGroupPages.backToGroups": {
    message: "Go to sandbox groups",
    description: "Button",
  },
};

export default en;
