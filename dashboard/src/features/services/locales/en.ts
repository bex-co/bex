import type { TranslationEntry } from "@/i18n";

const enServices: Record<string, TranslationEntry> = {
  "services.connect": {
    message: "Connect",
    description: "Open the service connection menu",
  },
  "services.connectSSH": {
    message: "SSH",
    description: "SSH section label in the service connection menu",
  },
  "services.sshCopy": {
    message: "Copy SSH command",
    description: "Copy service SSH command button",
  },
  "services.sshCopied": {
    message: "SSH command copied",
    description: "Successful SSH command copy",
  },
  "services.sshCopyError": {
    message: "Couldn't copy SSH command",
    description: "Failed SSH command copy",
  },
  "services.sshUnavailable": {
    message: "SSH isn't available",
    description: "Service header state when no SSH address is advertised",
  },
  "services.sshUnavailableHint": {
    message:
      "SSH requires a running paid web, private, or background service and an active gateway.",
    description: "Explanation for a service without an SSH address",
  },
  "services.actions": {
    message: "Actions",
    description: "Accessible heading for service configuration row actions",
  },
  "services.statTotal": {
    message: "Total services",
    description: "Services page stat card label",
  },
  "services.statRunning": {
    message: "Running",
    description: "Services page stat card label",
  },
  "services.statSuspended": {
    message: "Suspended",
    description: "Services page stat card label",
  },
  "services.cardTitle": {
    message: "Services",
    description:
      "Services table card title, also used as the metrics page back-link",
  },
  "services.colName": {
    message: "Name",
    description: "Services table column header",
  },
  "services.colStatus": {
    message: "Status",
    description: "Services table column header",
  },
  "services.colUrl": {
    message: "URL",
    description: "Services table column header",
  },
  "services.colSlug": {
    message: "Slug",
    description:
      "Service detail header fact (globally-unique platform-host segment, Render's slug field)",
  },
  "services.colInstances": {
    message: "Instances",
    description: "Services table column header (replica count — bex-native)",
  },
  "services.colRevision": {
    message: "Revision",
    description: "Services table column header (active revision — bex-native)",
  },
  "services.colCreated": {
    message: "Created",
    description: "Services table column header (relative age from createdAt)",
  },
  "services.colActions": {
    message: "Actions",
    description: "Services table actions column header (screen-reader only)",
  },
  "services.statusRunning": {
    message: "Running",
    description: "Services table status badge",
  },
  "services.statusSuspended": {
    message: "Suspended",
    description: "Services table status badge",
  },
  "services.statusSleeping": {
    message: "Sleeping",
    description:
      "Services status badge: a free-tier App auto-hibernated after idle (bex extension)",
  },
  "services.statusSleepingHint": {
    message: "Sleeping to save resources — wakes on the next request.",
    description:
      "Hint next to the Sleeping badge explaining free-tier auto-sleep + wake-on-request",
  },
  "services.statusPending": {
    message: "Pending",
    description: "Services table status badge",
  },
  "services.statusBuilding": {
    message: "Building",
    description: "Services table status badge",
  },
  "services.statusDeploying": {
    message: "Deploying",
    description: "Services table status badge",
  },
  "services.statusFailed": {
    message: "Failed",
    description: "Services table status badge",
  },
  "services.statusUnknown": {
    message: "Unknown",
    description: "Services table status badge for an unrecognized phase",
  },
  "services.actionsMenu": {
    message: "Open actions menu",
    description: "Accessible label for the per-row actions trigger",
  },
  "services.actionSuspend": {
    message: "Suspend",
    description: "Row action: park the service",
  },
  "services.actionResume": {
    message: "Resume",
    description: "Row action: bring a suspended service back",
  },
  "services.actionRestart": {
    message: "Restart",
    description: "Row action: roll the service's pods",
  },
  "services.confirmSuspendTitle": {
    message: "Suspend {name}?",
    description: "Suspend confirmation dialog title",
  },
  "services.confirmSuspendBody": {
    message:
      "The service scales to zero and stops serving traffic. Its URL and certificates are kept, and you can resume it at any time.",
    description: "Suspend confirmation dialog body",
  },
  "services.confirmRestartTitle": {
    message: "Restart {name}?",
    description: "Restart confirmation dialog title",
  },
  "services.confirmRestartBody": {
    message:
      "The service's pods roll with no downtime. In-flight requests finish before old instances are replaced.",
    description: "Restart confirmation dialog body",
  },
  "services.confirmCancel": {
    message: "Cancel",
    description: "Confirmation dialog cancel button",
  },
  "services.toastSuspendSuccess": {
    message: "Suspending {name}…",
    description: "Toast shown after a suspend request is accepted",
  },
  "services.toastResumeSuccess": {
    message: "Resuming {name}…",
    description: "Toast shown after a resume request is accepted",
  },
  "services.toastRestartSuccess": {
    message: "Restarting {name}…",
    description: "Toast shown after a restart request is accepted",
  },
  "services.toastError": {
    message: "Could not update {name}. Please try again.",
    description: "Toast shown when a lifecycle action fails",
  },
  "services.errorTitle": {
    message: "Couldn't load services",
    description: "Services list error card title",
  },
  "services.errorBody": {
    message: "The request to bex-api failed. Check your connection and retry.",
    description: "Services list error card body",
  },
  "services.emptyTitle": {
    message: "No services yet",
    description: "Services list empty state title",
  },
  "services.emptyBody": {
    message: "Deploy your first App and it'll show up here.",
    description: "Services list empty state body",
  },
  "services.navLabel": {
    message: "Service navigation",
    description: "Accessible label for the service-detail tab nav",
  },
  "services.headerServiceId": {
    message: "Service ID:",
    description: "Service-detail header metadata label for the service id",
  },
  "services.headerSchedule": {
    message: "Schedule:",
    description:
      "Service-detail header metadata label for a cron job's schedule",
  },
  "services.headerCopyServiceId": {
    message: "Copy service ID",
    description: "Accessible label for the header's service-id copy button",
  },
  "services.headerCopyUrl": {
    message: "Copy service URL",
    description: "Accessible label for the header's live-URL copy button",
  },
  "services.headerCopied": {
    message: "Copied to clipboard",
    description: "Toast after copying a value from the service-detail header",
  },
  "services.headerCopyError": {
    message: "Couldn't copy to clipboard",
    description: "Toast when a service-detail header copy fails",
  },
  "services.navLogs": {
    message: "Logs",
    description: "Service-detail nav item (logs tab)",
  },
  "services.navMetrics": {
    message: "Metrics",
    description: "Service-detail nav item (metrics tab)",
  },
  "services.navScaling": {
    message: "Scaling",
    description: "Service-detail nav item (autoscaling tab)",
  },
  "services.scalingTitle": {
    message: "Autoscaling",
    description: "Scaling tab section heading",
  },
  "services.scalingEnabled": {
    message: "Enable autoscaling",
    description: "Autoscaling enable/disable toggle label",
  },
  "services.scalingMinInstances": {
    message: "Min instances",
    description: "Autoscaling min instances input label",
  },
  "services.scalingMaxInstances": {
    message: "Max instances",
    description: "Autoscaling max instances input label",
  },
  "services.scalingTargetCPU": {
    message: "Target CPU %",
    description: "Autoscaling target CPU utilisation input label",
  },
  "services.scalingTargetMemory": {
    message: "Target memory %",
    description: "Autoscaling target memory utilisation input label",
  },
  "services.scalingSave": {
    message: "Save",
    description: "Autoscaling form save button",
  },
  "services.scalingSaved": {
    message: "Autoscaling settings saved.",
    description: "Autoscaling save success toast",
  },
  "services.scalingDisabled": {
    message: "Autoscaling disabled.",
    description: "Autoscaling disable success toast",
  },
  "services.scalingError": {
    message: "Failed to update autoscaling settings.",
    description: "Autoscaling save error toast",
  },
  "services.scalingDescription": {
    message:
      "Automatically scale this service up or down based on CPU and memory utilization.",
    description: "Autoscaling card description",
  },
  "services.scalingOn": {
    message: "Autoscaling on",
    description: "Autoscaling main toggle label when enabled",
  },
  "services.scalingOff": {
    message: "Autoscaling off",
    description: "Autoscaling main toggle label when disabled",
  },
  "services.scalingInstancesTitle": {
    message: "Number of Instances",
    description: "Autoscaling instances range-slider section heading",
  },
  "services.scalingInstancesHint": {
    message:
      "Render scales the number of instances for this service within the range you specify.",
    description: "Autoscaling instances range-slider section description",
  },
  "services.scalingCPUTitle": {
    message: "Target CPU Utilization",
    description: "Autoscaling CPU metric section heading",
  },
  "services.scalingCPUHint": {
    message:
      "If average CPU utilization is significantly above or below this value, bex adds or removes instances accordingly.",
    description: "Autoscaling CPU metric section description",
  },
  "services.scalingMemoryTitle": {
    message: "Target Memory Utilization",
    description: "Autoscaling memory metric section heading",
  },
  "services.scalingMemoryHint": {
    message:
      "If average memory utilization is significantly above or below this value, bex adds or removes instances accordingly.",
    description: "Autoscaling memory metric section description",
  },
  "services.scalingCancel": {
    message: "Cancel",
    description: "Autoscaling form cancel button",
  },
  "services.scalingSaveChanges": {
    message: "Save Changes",
    description: "Autoscaling form save-changes button",
  },
  "services.notFoundTitle": {
    message: "Service not found",
    description: "Overview page state when server(id) returns nothing",
  },
  "services.notFoundBody": {
    message: "No service named {name} exists, or you don't have access to it.",
    description: "Overview page not-found body",
  },
  "services.notFoundBackToList": {
    message: "Back to services",
    description:
      "Link on the service-detail not-found state back to the services list",
  },
  "services.navEnvironment": {
    message: "Environment",
    description: "Service-detail nav item (environment variables tab)",
  },
  "services.envTitle": {
    message: "Environment Variables",
    description: "Environment tab card title",
  },
  "services.envDescription": {
    message:
      "Set environment-specific config and secrets, then read those values from your code.",
    description: "Environment tab card description",
  },
  "services.envColKey": {
    message: "Key",
    description: "Environment table column header (variable name)",
  },
  "services.envColValue": {
    message: "Value",
    description: "Environment table column header (variable value)",
  },
  "services.envShowSecret": {
    message: "Show value",
    description: "Environment row button to reveal a masked value",
  },
  "services.envHideSecret": {
    message: "Hide value",
    description: "Environment row button to re-mask a revealed value",
  },
  "services.envRevealError": {
    message: "Couldn't load the value.",
    description: "Environment row inline error when a value reveal fails",
  },
  "services.envEmptyTitle": {
    message: "No environment variables",
    description: "Environment tab empty-state title",
  },
  "services.envEmptyBody": {
    message: "Add a variable to configure this service.",
    description: "Environment tab empty-state body",
  },
  "services.envUnavailableTitle": {
    message: "Environment variables unavailable",
    description:
      "Environment tab state when the secret store is unconfigured (503)",
  },
  "services.envUnavailableBody": {
    message: "The secret store isn't configured for this deployment.",
    description: "Environment tab unavailable-state body",
  },
  "services.envForbiddenTitle": {
    message: "Not authorized",
    description: "Environment tab state when the caller lacks permission (403)",
  },
  "services.envForbiddenBody": {
    message:
      "You don't have permission to view this service's environment variables.",
    description: "Environment tab forbidden-state body",
  },
  "services.envErrorTitle": {
    message: "Couldn't load environment variables",
    description: "Environment tab generic error title",
  },
  "services.envErrorBody": {
    message: "Something went wrong. Please try again.",
    description: "Environment tab generic error body",
  },
  "services.envAdd": {
    message: "Add variable",
    description: "Environment tab button to open the add-variable form",
  },
  "services.envEdit": {
    message: "Edit",
    description: "Environment row button to edit a variable's value",
  },
  "services.envDelete": {
    message: "Delete",
    description: "Environment row button to remove a variable",
  },
  "services.envSave": {
    message: "Save",
    description: "Environment add/edit form save button",
  },
  "services.envCancel": {
    message: "Cancel",
    description: "Environment add/edit form cancel button",
  },
  "services.envKeyPlaceholder": {
    message: "NAME_OF_VARIABLE",
    description: "Environment add-variable key input placeholder",
  },
  "services.envValuePlaceholder": {
    message: "value",
    description: "Environment value input placeholder",
  },
  "services.envGenerate": {
    message: "Generate",
    description: "Generate a cryptographically random environment value",
  },
  "services.envGeneratePlaceholder": {
    message: "Generated securely when saved",
    description:
      "Environment value placeholder while server generation is selected",
  },
  "services.envInvalidKey": {
    message: "Use letters, digits and underscores; can't start with a digit.",
    description:
      "Environment add-variable validation message for an invalid key",
  },
  "services.envDeleteConfirmTitle": {
    message: "Remove {key}?",
    description: "Environment delete-confirmation dialog title",
  },
  "services.envDeleteConfirmBody": {
    message: "The service will redeploy without this variable.",
    description: "Environment delete-confirmation dialog body",
  },
  "services.envRolloutNote": {
    message: "The service is redeploying to apply the change.",
    description:
      "Toast description after an env-var write (bex rolls the pods)",
  },
  "services.envSaveSuccess": {
    message: "Saved {key}",
    description: "Toast on a successful env-var add/update",
  },
  "services.envSaveError": {
    message: "Couldn't save {key}",
    description: "Toast on a failed env-var add/update",
  },
  "services.envDeleteSuccess": {
    message: "Removed {key}",
    description: "Toast on a successful env-var delete",
  },
  "services.envDeleteError": {
    message: "Couldn't remove {key}",
    description: "Toast on a failed env-var delete",
  },
  "services.secretFilesTitle": {
    message: "Secret Files",
    description: "Environment tab secret-files section title",
  },
  "services.secretFilesDescription": {
    message:
      "Store files with secret contents (certificates, credentials) mounted into this service at deploy time.",
    description: "Environment tab secret-files section description",
  },
  "services.secretFileColName": {
    message: "File name",
    description: "Secret-files table column header (file name)",
  },
  "services.secretFileColContent": {
    message: "Contents",
    description: "Secret-files table column header (file body)",
  },
  "services.secretFilesEmptyTitle": {
    message: "No secret files",
    description: "Secret-files empty-state title",
  },
  "services.secretFilesEmptyBody": {
    message: "Add a file to mount secret contents into this service.",
    description: "Secret-files empty-state body",
  },
  "services.secretFilesUnavailableTitle": {
    message: "Secret files unavailable",
    description:
      "Secret-files state when the secret store is unconfigured (503)",
  },
  "services.secretFilesUnavailableBody": {
    message: "The secret store isn't configured for this deployment.",
    description: "Secret-files unavailable-state body",
  },
  "services.secretFilesForbiddenTitle": {
    message: "Not authorized",
    description: "Secret-files state when the caller lacks permission (403)",
  },
  "services.secretFilesForbiddenBody": {
    message: "You don't have permission to view this service's secret files.",
    description: "Secret-files forbidden-state body",
  },
  "services.secretFilesErrorTitle": {
    message: "Couldn't load secret files",
    description: "Secret-files generic error title",
  },
  "services.secretFilesErrorBody": {
    message: "Something went wrong. Please try again.",
    description: "Secret-files generic error body",
  },
  "services.secretFileAdd": {
    message: "Add secret file",
    description: "Secret-files button to open the add-file form",
  },
  "services.secretFileNamePlaceholder": {
    message: "filename.ext",
    description: "Secret-files add-file name input placeholder",
  },
  "services.secretFileContentPlaceholder": {
    message: "file contents",
    description: "Secret-files content input placeholder",
  },
  "services.secretFileInvalidName": {
    message: "Use letters, digits, dot, dash and underscore; not '.' or '..'.",
    description: "Secret-files add-file validation message for an invalid name",
  },
  "services.secretFileDeleteConfirmTitle": {
    message: "Remove {name}?",
    description: "Secret-file delete-confirmation dialog title",
  },
  "services.secretFileDeleteConfirmBody": {
    message: "The service will redeploy without this file.",
    description: "Secret-file delete-confirmation dialog body",
  },
  "services.secretFileSaveSuccess": {
    message: "Saved {name}",
    description: "Toast on a successful secret-file add/update",
  },
  "services.secretFileSaveError": {
    message: "Couldn't save {name}",
    description: "Toast on a failed secret-file add/update",
  },
  "services.secretFileDeleteSuccess": {
    message: "Removed {name}",
    description: "Toast on a successful secret-file delete",
  },
  "services.secretFileDeleteError": {
    message: "Couldn't remove {name}",
    description: "Toast on a failed secret-file delete",
  },
  "services.envGroupsTitle": {
    message: "Environment Groups",
    description: "Environment tab env-groups section title",
  },
  "services.envGroupsDescription": {
    message:
      "Reusable bundles of environment variables and secret files you can link to this and other services.",
    description: "Environment tab env-groups section description",
  },
  "services.envGroupsEmptyTitle": {
    message: "No environment groups",
    description: "Env-groups empty-state title",
  },
  "services.envGroupsEmptyBody": {
    message: "Create a group to share config across services.",
    description: "Env-groups empty-state body",
  },
  "services.envGroupsUnavailableTitle": {
    message: "Environment groups unavailable",
    description: "Env-groups state when the secret store is unconfigured (503)",
  },
  "services.envGroupsUnavailableBody": {
    message: "The secret store isn't configured for this deployment.",
    description: "Env-groups unavailable-state body",
  },
  "services.envGroupsForbiddenTitle": {
    message: "Not authorized",
    description: "Env-groups state when the caller lacks permission (403)",
  },
  "services.envGroupsForbiddenBody": {
    message: "You don't have permission to view environment groups.",
    description: "Env-groups forbidden-state body",
  },
  "services.envGroupsErrorTitle": {
    message: "Couldn't load environment groups",
    description: "Env-groups generic error title",
  },
  "services.envGroupsErrorBody": {
    message: "Something went wrong. Please try again.",
    description: "Env-groups generic error body",
  },
  "services.envGroupCreate": {
    message: "Create group",
    description: "Env-groups button to open the create-group form",
  },
  "services.envGroupCreateSubmit": {
    message: "Create",
    description: "Env-groups create-group form submit button",
  },
  "services.envGroupNamePlaceholder": {
    message: "group-name",
    description: "Env-groups create-group name input placeholder",
  },
  "services.envGroupNameLabel": {
    message: "Group name",
    description: "Env-groups create-group name input accessible label",
  },
  "services.envGroupInvalidName": {
    message: "Enter a group name.",
    description:
      "Env-groups create-group validation message for an invalid name",
  },
  "services.envGroupLinked": {
    message: "Linked",
    description:
      "Env-groups badge: this group is linked to the current service",
  },
  "services.envGroupEmptyContents": {
    message: "No variables or files yet.",
    description: "Env-groups: shown when a group has no vars or secret files",
  },
  "services.envGroupLink": {
    message: "Link",
    description: "Env-groups button: attach this group to the current service",
  },
  "services.envGroupUnlink": {
    message: "Unlink",
    description:
      "Env-groups button: detach this group from the current service",
  },
  "services.envGroupDelete": {
    message: "Delete",
    description: "Env-groups action: delete the group",
  },
  "services.envGroupDeleteConfirmTitle": {
    message: "Delete {name}?",
    description: "Env-group delete-confirmation dialog title",
  },
  "services.envGroupDeleteConfirmBody": {
    message:
      "The group is removed from every service it's linked to. This can't be undone.",
    description: "Env-group delete-confirmation dialog body",
  },
  "services.envGroupCreateSuccess": {
    message: "Created {name}",
    description: "Toast on a successful env-group create",
  },
  "services.envGroupCreateError": {
    message: "Couldn't create {name}",
    description: "Toast on a failed env-group create",
  },
  "services.envGroupDeleteSuccess": {
    message: "Group deleted",
    description: "Toast on a successful env-group delete",
  },
  "services.envGroupDeleteError": {
    message: "Couldn't delete the group",
    description: "Toast on a failed env-group delete",
  },
  "services.envGroupLinkSuccess": {
    message: "Group linked",
    description: "Toast on a successful env-group link",
  },
  "services.envGroupLinkError": {
    message: "Couldn't link the group",
    description: "Toast on a failed env-group link",
  },
  "services.envGroupUnlinkSuccess": {
    message: "Group unlinked",
    description: "Toast on a successful env-group unlink",
  },
  "services.envGroupUnlinkError": {
    message: "Couldn't unlink the group",
    description: "Toast on a failed env-group unlink",
  },
  "services.navSettings": {
    message: "Settings",
    description: "Service-detail nav item (settings tab)",
  },
  "services.settingsTitle": {
    message: "Settings",
    description: "Settings tab card title",
  },
  "services.settingsDescription": {
    message:
      "Configure this service's name, instance size, and other settings.",
    description: "Settings tab card description",
  },
  "services.displayNameLabel": {
    message: "Service Name",
    description:
      "Settings tab row label for the mutable human-facing service name",
  },
  "services.displayNameHint": {
    message:
      "The service ID remains {id}; URLs and infrastructure do not change.",
    description:
      "Settings tab explanation that a display-name change preserves identity",
  },
  "services.displayNameEdit": {
    message: "Edit service name",
    description: "Accessible label for the service-name edit button",
  },
  "services.displayNameSave": {
    message: "Save service name",
    description: "Accessible label for the service-name save button",
  },
  "services.displayNameCancel": {
    message: "Cancel service name edit",
    description: "Accessible label for the service-name cancel button",
  },
  "services.displayNameSuccess": {
    message: 'Service renamed to "{name}".',
    description: "Toast after setDisplayName succeeds",
  },
  "services.displayNameCleared": {
    message: "Service name reset to its immutable ID.",
    description: "Toast after clearing displayName",
  },
  "services.displayNameError": {
    message: "Couldn't rename the service. Please try again.",
    description: "Toast after setDisplayName fails",
  },
  "services.settingsInstanceType": {
    message: "Instance Type",
    description: "Settings tab row label for the App's current plan/tier",
  },
  "services.settingsNoInstanceType": {
    message: "No instance type set",
    description: "Settings tab state for an untiered (bare-CR) App",
  },
  "services.settingsUpdate": {
    message: "Update",
    description: "Settings tab link to the instance-type picker",
  },
  "services.settingsIdleTimeout": {
    message: "Idle timeout",
    description:
      "Settings tab: label for the free-tier auto-sleep window control",
  },
  "services.settingsIdleTimeoutHint": {
    message:
      "Free services sleep after this idle window, then wake on the next request.",
    description: "Settings tab: idle-timeout control help text (bex extension)",
  },
  "services.settingsIdleTimeoutPaid": {
    message: "Paid services stay always-on and never sleep.",
    description: "Settings tab: shown instead of the control on a paid plan",
  },
  "services.idleTimeoutDefault": {
    message: "Platform default",
    description: "Idle-timeout option: 0 seconds = the operator's own window",
  },
  "services.idleTimeoutMinutes": {
    message: "{minutes} min",
    description: "Idle-timeout option label in minutes",
  },
  "services.idleTimeoutHours": {
    message: "{hours} hr",
    description: "Idle-timeout option label in hours",
  },
  "services.idleTimeoutSeconds": {
    message: "{seconds} sec",
    description: "Idle-timeout option label in seconds (non-round values)",
  },
  "services.idleTimeoutSuccess": {
    message: "Idle timeout updated.",
    description: "Toast after setIdleTimeout succeeds",
  },
  "services.idleTimeoutError": {
    message: "Couldn't update the idle timeout.",
    description: "Toast after setIdleTimeout fails",
  },
  "services.settingsMaxShutdownDelay": {
    message: "Max shutdown delay",
    description: "Settings row label for the graceful SIGTERM window",
  },
  "services.settingsMaxShutdownDelayHint": {
    message:
      "Wait 1–300 seconds after SIGTERM before force-stopping the process (default 30).",
    description: "Settings row help for the shutdown-delay range",
  },
  "services.maxShutdownDelaySeconds": {
    message: "{seconds} sec",
    description: "Current graceful-shutdown delay in seconds",
  },
  "services.maxShutdownDelayEdit": {
    message: "Edit max shutdown delay",
    description: "Accessible label for the shutdown-delay edit button",
  },
  "services.maxShutdownDelaySave": {
    message: "Save max shutdown delay",
    description: "Accessible label for the shutdown-delay save button",
  },
  "services.maxShutdownDelayCancel": {
    message: "Cancel max shutdown delay edit",
    description: "Accessible label for the shutdown-delay cancel button",
  },
  "services.maxShutdownDelaySuccess": {
    message: "Max shutdown delay updated.",
    description: "Toast after setMaxShutdownDelay succeeds",
  },
  "services.maxShutdownDelayError": {
    message: "Couldn't update the max shutdown delay.",
    description: "Toast after setMaxShutdownDelay fails",
  },
  "services.settingsHealthChecksTitle": {
    message: "Health Checks",
    description: "Settings tab: Health Checks section card title",
  },
  "services.settingsHealthChecksDescription": {
    message:
      "Configure the HTTP path bex polls periodically to monitor your service.",
    description: "Settings tab: Health Checks section card description",
  },
  "services.settingsHealthCheckPath": {
    message: "Health Check Path",
    description: "Settings tab: health-check path row label",
  },
  "services.settingsHealthCheckPathHint": {
    message:
      "Provide an HTTP endpoint path that bex polls periodically to monitor your service.",
    description: "Settings tab: health-check path row hint text",
  },
  "services.settingsHealthCheckPathPlaceholder": {
    message: "/",
    description: "Settings tab: health-check path input placeholder",
  },
  "services.settingsHealthCheckPathEdit": {
    message: "Edit Health Check Path",
    description:
      "Settings tab: accessible label for the health-check path edit-pencil button",
  },
  "services.healthCheckPathSuccess": {
    message: "Health check path updated.",
    description: "Toast after setHealthCheckPath succeeds",
  },
  "services.healthCheckPathError": {
    message: "Couldn't update the health check path.",
    description: "Toast after setHealthCheckPath fails",
  },
  "services.settingsNotificationsTitle": {
    message: "Notifications",
    description: "Settings tab: Notifications section card title",
  },
  "services.settingsNotificationsDescription": {
    message: "Choose who gets emailed when a deploy of this service fails.",
    description: "Settings tab: Notifications section card description",
  },
  "services.notifyOnFailLabel": {
    message: "Deploy Failure Notifications",
    description: "Settings tab: notifyOnFail row label",
  },
  "services.notifyOnFailHint": {
    message:
      "Default defers to each member's own notification preference; you can force it on or off for just this service.",
    description: "Settings tab: notifyOnFail row hint text",
  },
  "services.notifyOnFailOptionDefault": {
    message: "Use member preference",
    description: "notifyOnFail select option: default",
  },
  "services.notifyOnFailOptionNotify": {
    message: "Always notify",
    description: "notifyOnFail select option: notify",
  },
  "services.notifyOnFailOptionIgnore": {
    message: "Never notify",
    description: "notifyOnFail select option: ignore",
  },
  "services.notifyOnFailSuccess": {
    message: "Notification setting updated.",
    description: "Toast after setNotifyOnFail succeeds",
  },
  "services.notifyOnFailError": {
    message: "Couldn't update the notification setting.",
    description: "Toast after setNotifyOnFail fails",
  },
  "services.planPickerTitle": {
    message: "Pick an Instance Type",
    description: "Plan-picker page heading",
  },
  "services.planPickerFreeGroup": {
    message: "Free",
    description:
      "Plan-picker section label separating the Free tier from paid tiers",
  },
  "services.planPickerPaidGroup": {
    message: "Paid",
    description: "Plan-picker section label for the paid tier ladder",
  },
  "services.planPickerCancel": {
    message: "Cancel",
    description: "Plan-picker footer button: discard the selection",
  },
  "services.planPickerSave": {
    message: "Save Changes",
    description: "Plan-picker footer button: confirm the plan change",
  },
  "services.planPickerConfirmTitle": {
    message: "Change instance type to {name}?",
    description: "Plan-change confirm dialog title",
  },
  "services.planPickerConfirmBody": {
    message:
      "The service resizes and rolls with no downtime — in-flight requests finish before old instances are replaced.",
    description: "Plan-change confirm dialog body",
  },
  "services.planPickerSuccess": {
    message: "Instance type updated to {name}",
    description: "Toast on a successful plan change",
  },
  "services.planPickerError": {
    message: "Couldn't update the instance type. Please try again.",
    description: "Toast on a failed plan change",
  },
  "services.planPickerErrorTitle": {
    message: "Couldn't load instance types",
    description: "Plan-picker error state title (instanceTypes query failed)",
  },
  "services.planPickerErrorBody": {
    message: "The request to bex-api failed. Check your connection and retry.",
    description: "Plan-picker error state body",
  },
  "services.domainsTitle": {
    message: "Custom Domains",
    description: "Settings tab custom-domains section title",
  },
  "services.domainsDescription": {
    message: "Point custom domains you own to this service.",
    description: "Settings tab custom-domains section description",
  },
  "services.domainColName": {
    message: "Name",
    description: "Custom-domains table column header (the FQDN)",
  },
  "services.domainColVerified": {
    message: "Verified Status",
    description:
      "Custom-domains table column header (DNS/ownership verification)",
  },
  "services.domainColCertificate": {
    message: "Certificate Status",
    description:
      "Custom-domains table column header (TLS certificate serving state)",
  },
  "services.domainColActions": {
    message: "Actions",
    description:
      "Custom-domains table actions column header (screen-reader only)",
  },
  "services.domainVerified": {
    message: "Verified",
    description: "Custom-domains status badge: TLS certificate has been issued",
  },
  "services.domainCertActive": {
    message: "Active",
    description:
      "Custom-domains status badge: certificate issued and serving traffic",
  },
  "services.domainPending": {
    message: "Pending",
    description:
      "Custom-domains status badge: certificate not yet issued/serving",
  },
  "services.domainActionsMenu": {
    message: "Open domain actions menu",
    description: "Accessible label for the per-domain actions trigger",
  },
  "services.domainDelete": {
    message: "Delete",
    description: "Custom-domains row action: remove the domain",
  },
  "services.domainCancel": {
    message: "Cancel",
    description: "Custom-domains dialog cancel button",
  },
  "services.domainDeleteConfirmTitle": {
    message: "Delete {name}?",
    description: "Custom-domain delete-confirmation dialog title",
  },
  "services.domainDeleteConfirmBody": {
    message:
      "The service stops serving this domain. Its Ingress rule is removed and the TLS certificate is left to expire. This can't be undone.",
    description: "Custom-domain delete-confirmation dialog body",
  },
  "services.domainAdd": {
    message: "Add Custom Domain",
    description: "Custom-domains button to open the add-domain dialog",
  },
  "services.domainAddTitle": {
    message: "Add a custom domain",
    description: "Add-domain dialog title",
  },
  "services.domainAddDescription": {
    message:
      "Enter a domain you own. Point its DNS to this service, and bex issues a TLS certificate automatically.",
    description: "Add-domain dialog description",
  },
  "services.domainPlaceholder": {
    message: "www.example.com",
    description: "Add-domain FQDN input placeholder",
  },
  "services.domainInvalid": {
    message: "Enter a valid domain, e.g. www.example.com.",
    description: "Add-domain validation message for a malformed hostname",
  },
  "services.domainAddButton": {
    message: "Add Domain",
    description: "Add-domain dialog submit button",
  },
  "services.domainAddSuccess": {
    message: "Added {name}",
    description: "Toast on a successful custom-domain add",
  },
  "services.domainAddError": {
    message: "Couldn't add {name}",
    description: "Toast on a failed custom-domain add",
  },
  "services.domainAddConflict": {
    message: "{name} is already in use by another service",
    description:
      "Toast when a custom-domain add is rejected because the host is registered on another service (409)",
  },
  "services.domainAddReserved": {
    message: "{name} is a reserved platform hostname",
    description:
      "Toast when a custom-domain add is rejected because the host is a platform-owned name (400)",
  },
  "services.domainDeleteSuccess": {
    message: "Removed {name}",
    description: "Toast on a successful custom-domain delete",
  },
  "services.domainDeleteError": {
    message: "Couldn't remove {name}",
    description: "Toast on a failed custom-domain delete",
  },
  "services.domainPropagateNote": {
    message: "DNS and the TLS certificate propagate in the background.",
    description:
      "Toast description after a custom-domain add (async convergence)",
  },
  "services.domainDnsToggle": {
    message: "Show DNS setup",
    description:
      "aria-label for the per-domain DNS-instructions disclosure toggle",
  },
  "services.domainDnsTitle": {
    message: "DNS setup",
    description: "Heading of the per-domain DNS-instructions panel",
  },
  "services.domainDnsSubdomainGuidance": {
    message:
      "Create the following record at your DNS provider, then re-check. bex issues the TLS certificate automatically once it resolves.",
    description: "Guidance line above the DNS record for a subdomain",
  },
  "services.domainDnsApexGuidance": {
    message:
      "Apex domains can't use a plain CNAME. Create this record if your provider supports ALIAS/ANAME (or CNAME flattening); otherwise redirect the apex to your www subdomain at your registrar.",
    description: "Guidance line above the DNS record for an apex domain",
  },
  "services.domainRecordType": {
    message: "Type",
    description: "Label for the DNS record type field (CNAME/ALIAS)",
  },
  "services.domainRecordHost": {
    message: "Host",
    description: "Label for the DNS record host/name field",
  },
  "services.domainRecordTarget": {
    message: "Target",
    description: "Label for the DNS record target/value field",
  },
  "services.domainDnsUnavailable": {
    message:
      "The DNS target isn't available yet — re-check once the service is running.",
    description: "Shown when the backend couldn't derive the DNS record target",
  },
  "services.domainRecheck": {
    message: "Re-check",
    description: "Button that re-checks a domain's DNS/certificate status",
  },
  "services.domainCopied": {
    message: "Copied to clipboard",
    description: "Toast when a DNS record value is copied",
  },
  "services.domainCopyError": {
    message: "Couldn't copy to clipboard",
    description: "Toast when copying a DNS record value fails",
  },
  "services.domainAddedTitle": {
    message: "Domain added — set up DNS",
    description: "Title of the post-add DNS-record step in the add dialog",
  },
  "services.domainAddedDescription": {
    message:
      "Create this record at your DNS provider to finish connecting your domain.",
    description: "Subtitle of the post-add DNS-record step in the add dialog",
  },
  "services.domainPairedWith": {
    message: "Paired with {sibling} — bex added it automatically",
    description:
      "Note under a domain row's name when bex auto-added its www<->apex sibling (w6/m23)",
  },
  "services.domainPairedDnsTitle": {
    message: "{sibling} was added automatically — set up its DNS too",
    description:
      "Heading of the second DNS-record block in the add dialog when the add auto-paired a www<->apex sibling (w6/m23)",
  },
  "services.domainDone": {
    message: "Done",
    description: "Button closing the post-add DNS-record step",
  },
  "services.domainVerifySuccess": {
    message: "{name} verified",
    description: "Toast when a re-check finds the domain verified",
  },
  "services.domainVerifyPending": {
    message: "{name} isn't verified yet — DNS may still be propagating.",
    description: "Toast when a re-check finds the domain still pending",
  },
  "services.domainVerifyError": {
    message: "Couldn't re-check {name}.",
    description: "Toast when the re-check request fails",
  },
  "services.domainsEmptyTitle": {
    message: "No custom domains",
    description: "Custom-domains empty-state title",
  },
  "services.domainsEmptyBody": {
    message: "Add a domain you own to serve this service from it.",
    description: "Custom-domains empty-state body",
  },
  "services.domainsErrorTitle": {
    message: "Couldn't load custom domains",
    description: "Custom-domains generic error title",
  },
  "services.domainsErrorBody": {
    message: "The request to bex-api failed. Check your connection and retry.",
    description: "Custom-domains generic error body",
  },
  "services.platformSubdomainTitle": {
    message: "Platform Subdomain",
    description: "Settings tab platform-subdomain section title",
  },
  "services.platformSubdomainDescription": {
    message:
      "Control whether your service answers on its bex platform subdomain in addition to any custom domains you've configured.",
    description: "Settings tab platform-subdomain section description",
  },
  "services.platformSubdomainEnabled": {
    message: "Enabled",
    description: "Platform-subdomain toggle label when the subdomain is active",
  },
  "services.platformSubdomainDisabled": {
    message: "Disabled",
    description:
      "Platform-subdomain toggle label when the subdomain is disabled",
  },
  "services.platformSubdomainPending": {
    message: "The platform URL is assigned once the service is running.",
    description: "Platform-subdomain state when the service has no URL yet",
  },
  "services.platformSubdomainDisabledNote": {
    message:
      "Platform subdomain is disabled. Your service is only reachable via custom domains.",
    description:
      "Platform-subdomain note shown when the policy is set to disabled",
  },
  "services.platformSubdomainToggleLabel": {
    message: "Toggle platform subdomain",
    description: "Accessible label for the platform-subdomain Switch",
  },
  "services.subdomainPolicySuccess": {
    message: "Platform subdomain setting updated.",
    description: "Toast on successful setSubdomainPolicy mutation",
  },
  "services.subdomainPolicyError": {
    message: "Could not update platform subdomain setting. Try again.",
    description: "Toast on failed setSubdomainPolicy mutation",
  },
  "services.maintenanceModeTitle": {
    message: "Maintenance Mode",
    description: "Settings tab maintenance-mode section title",
  },
  "services.maintenanceModeDescription": {
    message:
      "Take this service offline behind a maintenance page without suspending it — its pods keep running, only public traffic is redirected.",
    description: "Settings tab maintenance-mode section description",
  },
  "services.maintenanceModeEnabled": {
    message: "Enabled",
    description: "Maintenance-mode toggle label when active",
  },
  "services.maintenanceModeDisabled": {
    message: "Disabled",
    description: "Maintenance-mode toggle label when inactive",
  },
  "services.maintenanceModeSwitchHint": {
    message:
      "Every host this service serves will answer with the maintenance page.",
    description: "Maintenance-mode section hint text next to the switch",
  },
  "services.maintenanceModeToggleLabel": {
    message: "Toggle maintenance mode",
    description: "Accessible label for the maintenance-mode Switch",
  },
  "services.maintenanceModeUriLabel": {
    message: "Custom maintenance page URL",
    description: "Label for the maintenance-mode custom-page URL field",
  },
  "services.maintenanceModeUriPlaceholder": {
    message: "https://status.example.com/maintenance (optional)",
    description: "Placeholder for the maintenance-mode custom-page URL field",
  },
  "services.maintenanceModeUriHint": {
    message:
      "Fetched and served in place of the default page. Leave empty to use bex's default maintenance page. Must not point at this service's own URL.",
    description: "Hint text under the maintenance-mode custom-page URL field",
  },
  "services.maintenanceModeSaveUri": {
    message: "Save",
    description: "Save button for the maintenance-mode custom-page URL field",
  },
  "services.maintenanceModeEnableAction": {
    message: "Enable maintenance mode",
    description: "Confirm-dialog action button for enabling maintenance mode",
  },
  "services.confirmMaintenanceModeTitle": {
    message: "Enable maintenance mode?",
    description: "Confirm-dialog title for enabling maintenance mode",
  },
  "services.confirmMaintenanceModeBody": {
    message:
      "{name} will show a maintenance page to every visitor until you disable this. The service's pods keep running.",
    description: "Confirm-dialog body for enabling maintenance mode",
  },
  "services.maintenanceModeEnabledSuccess": {
    message: "Maintenance mode enabled.",
    description: "Toast on successfully enabling maintenance mode",
  },
  "services.maintenanceModeDisabledSuccess": {
    message: "Maintenance mode disabled.",
    description: "Toast on successfully disabling maintenance mode",
  },
  "services.maintenanceModeError": {
    message: "Could not update maintenance mode. Try again.",
    description: "Toast on failed setMaintenanceMode mutation",
  },
  "services.maintenanceModeBannerTitle": {
    message: "Maintenance mode is on",
    description: "Service-detail header banner title while in maintenance",
  },
  "services.maintenanceModeBannerBody": {
    message:
      "Visitors see a maintenance page instead of this service. Pods are still running — disable maintenance mode in Settings to resume normal serving.",
    description: "Service-detail header banner body while in maintenance",
  },
  "services.deployTitle": {
    message: "Deploy",
    description: "Cron job Settings tab: Deploy section title (Render parity)",
  },
  "services.deployDescription": {
    message: "How this cron job runs.",
    description: "Cron job Settings tab: Deploy section description",
  },
  "services.deployEdit": {
    message: "Edit cron settings",
    description:
      "Cron job Deploy section: accessible label for the edit-pencil button",
  },
  "services.deploySave": {
    message: "Save",
    description: "Cron job Deploy section: save button",
  },
  "services.deployCancel": {
    message: "Cancel",
    description: "Cron job Deploy section: cancel edit button",
  },
  "services.deploySuccess": {
    message: "Cron job settings saved.",
    description: "Toast after updateCronJob succeeds",
  },
  "services.deployConverging": {
    message:
      "The operator will apply the new schedule on its next reconcile pass.",
    description:
      "Toast description after a cron job schedule change (async convergence)",
  },
  "services.deployError": {
    message: "Couldn't save cron job settings. Please try again.",
    description: "Toast after updateCronJob fails",
  },
  "services.deployScheduleLabel": {
    message: "Schedule",
    description: "Cron job Settings tab: Deploy section schedule field label",
  },
  "services.deployScheduleHint": {
    message: "Runs this command on this schedule (5-field crontab).",
    description: "Cron job Settings tab: Deploy section schedule help text",
  },
  "services.deploySchedulePlaceholder": {
    message: "0 * * * *",
    description: "Cron job Deploy section: schedule input placeholder",
  },
  "services.deployScheduleError": {
    message: "Enter a valid 5-field cron expression, e.g. 0 * * * *.",
    description: "Cron job Deploy section: schedule validation error",
  },
  "services.deployScheduleRequired": {
    message: "Schedule is required.",
    description: "Cron job Deploy section: schedule required validation error",
  },
  "services.deployCommandLabel": {
    message: "Command",
    description: "Cron job Settings tab: Deploy section command field label",
  },
  "services.deployCommandPlaceholder": {
    message: "e.g. python script.py",
    description: "Cron job Deploy section: command input placeholder",
  },
  "services.deployCommandHint": {
    message:
      "Overrides the image's default entry point. Leave blank to run the image's own command.",
    description: "Cron job Deploy section: command field help text",
  },
  "services.deployCommandEmpty": {
    message: "Uses the image's own default command.",
    description:
      "Cron job Settings tab: shown when spec.command is unset (no override)",
  },
  "services.buildDeployTitle": {
    message: "Build & Deploy",
    description:
      "Settings tab: Build & Deploy section title (w5/m13, Render parity)",
  },
  "services.buildDeployDescription": {
    message: "Where this service builds and deploys from.",
    description: "Settings tab: Build & Deploy section description",
  },
  "services.buildDeploySourceLabel": {
    message: "Source",
    description: "Build & Deploy: repo field label (read-only)",
  },
  "services.buildDeployBranchLabel": {
    message: "Branch",
    description: "Build & Deploy: branch field label (read-only)",
  },
  "services.buildDeployRootDirLabel": {
    message: "Root Directory",
    description: "Build & Deploy: root-directory field label",
  },
  "services.buildDeployRootDirOptional": {
    message: "Optional",
    description:
      "Build & Deploy: badge next to the Root Directory label (Render parity)",
  },
  "services.buildDeployRootDirHint": {
    message:
      "If set, builds run from this subdirectory instead of the repository root. Code changes outside of it don't trigger an auto-deploy. Most commonly used with a monorepo.",
    description: "Build & Deploy: root-directory field help text",
  },
  "services.buildDeployRootDirEmpty": {
    message: "Repository root",
    description: "Build & Deploy: shown when spec.rootDir is unset",
  },
  "services.buildDeployConfirmRoot": {
    message: "the repository root",
    description:
      "Build & Deploy: mid-sentence phrase for the confirm dialog title when clearing rootDir to empty (a dedicated key, not a lowercased buildDeployRootDirEmpty, since that transform doesn't hold in every language)",
  },
  "services.buildDeployRootDirPlaceholder": {
    message: "e.g. backend",
    description: "Build & Deploy: root-directory input placeholder",
  },
  "services.buildDeployEdit": {
    message: "Edit Root Directory",
    description: "Build & Deploy: accessible label for the edit-pencil button",
  },
  "services.buildDeploySave": {
    message: "Save",
    description: "Build & Deploy: root-directory inline-edit save button",
  },
  "services.buildDeployCancel": {
    message: "Cancel",
    description: "Build & Deploy: root-directory inline-edit cancel button",
  },
  "services.buildDeployConfirmTitle": {
    message: "Change Root Directory to {value}?",
    description: "Build & Deploy: root-directory change confirm dialog title",
  },
  "services.buildDeployConfirmBody": {
    message:
      "The service rebuilds and redeploys, scoped to the new directory — in-flight requests finish before old instances are replaced.",
    description: "Build & Deploy: root-directory change confirm dialog body",
  },
  "services.buildDeploySuccess": {
    message: "Root Directory updated.",
    description: "Toast after setRootDir succeeds",
  },
  "services.buildDeployError": {
    message: "Couldn't update the Root Directory. Please try again.",
    description: "Toast after setRootDir fails",
  },
  "services.startCommandLabel": {
    message: "Start Command",
    description: "Build & Deploy: native-runtime start-command label",
  },
  "services.dockerCommandLabel": {
    message: "Docker Command",
    description: "Build & Deploy: Docker CMD override label (Render wording)",
  },
  "services.startCommandHint": {
    message: "The command that starts this service after a successful build.",
    description: "Build & Deploy: native start-command help text",
  },
  "services.dockerCommandHint": {
    message:
      "Overrides the CMD in the Dockerfile. Leave blank to use the image's default command.",
    description: "Build & Deploy: Docker Command help text",
  },
  "services.startCommandEmpty": {
    message: "Uses the runtime or image default command",
    description: "Build & Deploy: empty start/Docker Command state",
  },
  "services.startCommandConfirmEmpty": {
    message: "the default command",
    description: "Build & Deploy: empty command phrase in confirmation title",
  },
  "services.startCommandPlaceholder": {
    message: "e.g. npm start",
    description: "Build & Deploy: start-command input placeholder",
  },
  "services.startCommandEdit": {
    message: "Edit Start Command",
    description: "Build & Deploy: accessible command edit button label",
  },
  "services.dockerCommandEdit": {
    message: "Edit Docker Command",
    description: "Build & Deploy: accessible Docker Command edit button label",
  },
  "services.dockerCommandPlaceholder": {
    message: "e.g. bin/server",
    description: "Build & Deploy: Docker Command input placeholder",
  },
  "services.startCommandConfirmTitle": {
    message: "Change the start command to {value}?",
    description: "Build & Deploy: command-change confirmation title",
  },
  "services.dockerCommandConfirmTitle": {
    message: "Change the Docker Command to {value}?",
    description: "Build & Deploy: Docker Command confirmation title",
  },
  "services.startCommandConfirmBody": {
    message:
      "The service redeploys with the new command. In-flight requests finish before old instances are replaced.",
    description: "Build & Deploy: command-change confirmation body",
  },
  "services.startCommandSuccess": {
    message: "Start Command updated.",
    description: "Toast after setStartCommand succeeds",
  },
  "services.startCommandError": {
    message: "Couldn't update the Start Command. Please try again.",
    description: "Toast after setStartCommand fails",
  },
  "services.dockerfilePathLabel": {
    message: "Dockerfile Path",
    description: "Build & Deploy: Dockerfile-path field label",
  },
  "services.dockerfilePathHint": {
    message:
      "Path to the Dockerfile, relative to the Root Directory. Leave blank to use Dockerfile.",
    description: "Build & Deploy: Dockerfile-path help text",
  },
  "services.dockerfilePathEmpty": {
    message: "Dockerfile",
    description: "Build & Deploy: default Dockerfile-path state",
  },
  "services.dockerfilePathConfirmEmpty": {
    message: "the default Dockerfile",
    description: "Build & Deploy: empty Dockerfile-path confirmation phrase",
  },
  "services.dockerfilePathPlaceholder": {
    message: "e.g. docker/Dockerfile.prod",
    description: "Build & Deploy: Dockerfile-path input placeholder",
  },
  "services.dockerfilePathEdit": {
    message: "Edit Dockerfile Path",
    description: "Build & Deploy: accessible Dockerfile-path edit label",
  },
  "services.dockerfilePathConfirmTitle": {
    message: "Change Dockerfile Path to {value}?",
    description: "Build & Deploy: Dockerfile-path confirmation title",
  },
  "services.dockerfilePathConfirmBody": {
    message:
      "The service rebuilds from the selected Dockerfile and deploys the resulting image.",
    description: "Build & Deploy: Dockerfile-path confirmation body",
  },
  "services.dockerfilePathSuccess": {
    message: "Dockerfile Path updated.",
    description: "Toast after setDockerfilePath succeeds",
  },
  "services.dockerfilePathError": {
    message: "Couldn't update the Dockerfile Path. Please try again.",
    description: "Toast after setDockerfilePath fails",
  },
  "services.buildFilterLabel": {
    message: "Build Filters",
    description: "Build & Deploy: label for the build-filters editor",
  },
  "services.buildFilterHint": {
    message:
      "Only trigger a deploy when a git push changes matching files. Paths are glob patterns relative to the repository root (e.g. src/**, **/*.md).",
    description: "Build & Deploy: help text for the build-filters editor",
  },
  "services.buildFilterIncludedTitle": {
    message: "Included Paths",
    description: "Build & Deploy: title for the included-paths list",
  },
  "services.buildFilterIncludedHint": {
    message:
      "A push deploys only when a changed file matches one of these. Empty means every path is included.",
    description: "Build & Deploy: help text for the included-paths list",
  },
  "services.buildFilterIncludedPlaceholder": {
    message: "e.g. src/**",
    description: "Build & Deploy: placeholder for an included-path input",
  },
  "services.buildFilterAddIncluded": {
    message: "Add included path",
    description: "Build & Deploy: add-row button for the included-paths list",
  },
  "services.buildFilterRemoveIncluded": {
    message: "Remove included path",
    description: "Build & Deploy: remove-row label for an included path",
  },
  "services.buildFilterIgnoredTitle": {
    message: "Ignored Paths",
    description: "Build & Deploy: title for the ignored-paths list",
  },
  "services.buildFilterIgnoredHint": {
    message:
      "A changed file matching one of these never triggers a deploy, even if it also matches an included path.",
    description: "Build & Deploy: help text for the ignored-paths list",
  },
  "services.buildFilterIgnoredPlaceholder": {
    message: "e.g. docs/**",
    description: "Build & Deploy: placeholder for an ignored-path input",
  },
  "services.buildFilterAddIgnored": {
    message: "Add ignored path",
    description: "Build & Deploy: add-row button for the ignored-paths list",
  },
  "services.buildFilterRemoveIgnored": {
    message: "Remove ignored path",
    description: "Build & Deploy: remove-row label for an ignored path",
  },
  "services.buildFilterSave": {
    message: "Save Build Filters",
    description: "Build & Deploy: save button for the build-filters editor",
  },
  "services.buildFilterSuccess": {
    message: "Build Filters updated.",
    description: "Toast after setBuildFilter succeeds",
  },
  "services.buildFilterError": {
    message: "Couldn't update the Build Filters. Please try again.",
    description: "Toast after setBuildFilter fails",
  },
  "services.preDeployLabel": {
    message: "Pre-Deploy Command",
    description: "Build & Deploy: label for the pre-deploy command field",
  },
  "services.preDeployHint": {
    message:
      "Runs once against the new image before it serves traffic (e.g. a database migration). A non-zero exit fails the deploy and keeps the previous version live.",
    description: "Build & Deploy: help text for the pre-deploy command field",
  },
  "services.preDeployPlaceholder": {
    message: "e.g. npm run migrate",
    description: "Build & Deploy: placeholder for the pre-deploy command input",
  },
  "services.preDeployEmpty": {
    message: "No pre-deploy command",
    description: "Build & Deploy: empty state for the pre-deploy command field",
  },
  "services.preDeployEdit": {
    message: "Edit Pre-Deploy Command",
    description:
      "Build & Deploy: accessible label for the pre-deploy edit-pencil button",
  },
  "services.preDeploySuccess": {
    message: "Pre-Deploy Command updated.",
    description: "Toast after setPreDeployCommand succeeds",
  },
  "services.preDeployError": {
    message: "Couldn't update the Pre-Deploy Command. Please try again.",
    description: "Toast after setPreDeployCommand fails",
  },
  "services.autoDeployLabel": {
    message: "Auto-Deploy",
    description: "Build & Deploy: label for the auto-deploy toggle",
  },
  "services.autoDeployViaGitHub": {
    message:
      "A push to the tracked branch redeploys automatically via the GitHub app.",
    description:
      "Build & Deploy: source indicator when the repo is on the connected GitHub account",
  },
  "services.autoDeployViaWebhook": {
    message:
      "A push redeploys only if the repo's manual git webhook is configured with your BEX_WEBHOOK_SECRET.",
    description:
      "Build & Deploy: source indicator when the repo is not on the connected GitHub account",
  },
  "services.autoDeployOnSuccess": {
    message: "Auto-Deploy turned on.",
    description: "Toast after enabling auto-deploy",
  },
  "services.autoDeployOffSuccess": {
    message: "Auto-Deploy turned off.",
    description: "Toast after disabling auto-deploy",
  },
  "services.autoDeployError": {
    message: "Couldn't change Auto-Deploy. Please try again.",
    description: "Toast after setAutoDeploy fails",
  },
  "services.deployHookTitle": {
    message: "Deploy Hook",
    description: "Settings tab: secret Deploy Hook section title",
  },
  "services.deployHookDescription": {
    message: "Trigger a deploy from CI with a single secret URL.",
    description: "Settings tab: Deploy Hook section description",
  },
  "services.deployHookURLLabel": {
    message: "Deploy Hook URL",
    description: "Accessible label for the secret Deploy Hook URL field",
  },
  "services.deployHookReveal": {
    message: "Reveal Deploy Hook URL",
    description: "Accessible label for the reveal-secret button",
  },
  "services.deployHookHide": {
    message: "Hide Deploy Hook URL",
    description: "Accessible label for the hide-secret button",
  },
  "services.deployHookCopy": {
    message: "Copy Deploy Hook URL",
    description: "Accessible label for the Deploy Hook copy button",
  },
  "services.deployHookCopied": {
    message: "Deploy Hook URL copied.",
    description: "Toast after copying the Deploy Hook URL",
  },
  "services.deployHookCopyError": {
    message: "Couldn't copy the Deploy Hook URL.",
    description: "Toast after Deploy Hook URL clipboard failure",
  },
  "services.deployHookSecretHint": {
    message:
      "Keep this URL secret. Anyone who has it can deploy this service without an API key.",
    description: "Security warning below the Deploy Hook URL",
  },
  "services.deployHookRegenerate": {
    message: "Regenerate Hook",
    description: "Deploy Hook rotation button",
  },
  "services.deployHookRegenerateTitle": {
    message: "Regenerate the Deploy Hook?",
    description: "Deploy Hook rotation confirmation title",
  },
  "services.deployHookRegenerateWarning": {
    message:
      "The current URL will stop working immediately. Update every CI system, cron job, and integration that uses it.",
    description: "Deploy Hook rotation confirmation warning",
  },
  "services.deployHookCancel": {
    message: "Cancel",
    description: "Deploy Hook rotation confirmation cancel button",
  },
  "services.deployHookRegenerateConfirm": {
    message: "Regenerate",
    description: "Deploy Hook rotation confirmation action",
  },
  "services.deployHookRegenerated": {
    message: "Deploy Hook regenerated. The old URL no longer works.",
    description: "Toast after successful Deploy Hook rotation",
  },
  "services.deployHookRegenerateError": {
    message: "Couldn't regenerate the Deploy Hook. Please try again.",
    description: "Toast after Deploy Hook rotation fails",
  },
  "services.deployHookLoadError": {
    message: "Couldn't load the Deploy Hook URL.",
    description: "Deploy Hook section query error",
  },
  "services.colType": {
    message: "Type",
    description: "Services table column header (service type)",
  },
  "services.typeWeb": {
    message: "Web Service",
    description: "Service-type badge: an HTTP service exposed at a URL",
  },
  "services.typePrivate": {
    message: "Private Service",
    description:
      "Service-type badge: an HTTP service reachable only in-cluster",
  },
  "services.typeWorker": {
    message: "Background Worker",
    description: "Service-type badge: runs with no HTTP port/URL",
  },
  "services.typeCron": {
    message: "Cron Job",
    description: "Service-type badge: runs a command on a schedule",
  },
  "services.typeStatic": {
    message: "Static Site",
    description:
      "Service-type badge: built output served from the object-store origin",
  },
  "services.staticTitle": {
    message: "Static Site",
    description:
      "Settings section title for a static site's publish dir + edge rules",
  },
  "services.staticDescription": {
    message:
      "The published output directory and the edge rules applied when serving it.",
    description: "Static Site settings section description",
  },
  "services.staticEdit": {
    message: "Edit",
    description: "Edit an inline field",
  },
  "services.staticSave": {
    message: "Save",
    description: "Save an inline field",
  },
  "services.staticCancel": {
    message: "Cancel",
    description: "Cancel an inline edit",
  },
  "services.publishPathLabel": {
    message: "Publish directory",
    description: "Label for a static site's publishPath",
  },
  "services.publishPathPlaceholder": {
    message: "dist",
    description: "Placeholder for the publish directory input",
  },
  "services.publishPathHint": {
    message:
      "The built output directory served as the site root (e.g. dist, build, public). Changing it republishes the site.",
    description: "Help text under the publish directory field",
  },
  "services.publishPathSaved": {
    message: "Publish directory updated",
    description: "Toast after saving a static site's publishPath",
  },
  "services.publishPathRepublishNote": {
    message: "The site will republish shortly.",
    description: "Toast description after changing publishPath",
  },
  "services.publishPathError": {
    message: "Couldn't update the publish directory",
    description: "Error toast for a failed publishPath change",
  },
  "services.routesTitle": {
    message: "Redirects & rewrites",
    description: "Title for the static-site routes editor",
  },
  "services.routesHint": {
    message:
      "Matched in order, first match wins. A redirect returns 301; a rewrite serves another path (an SPA fallback rewrites /* to /index.html).",
    description: "Help text for the routes editor",
  },
  "services.routeAdd": { message: "Add rule", description: "Add a route rule" },
  "services.routeType": { message: "Type", description: "Route type column" },
  "services.routeSource": {
    message: "Source",
    description: "Route source-path column",
  },
  "services.routeDestination": {
    message: "Destination",
    description: "Route destination-path column",
  },
  "services.routeRewrite": {
    message: "Rewrite",
    description: "Route type: rewrite",
  },
  "services.routeRedirect": {
    message: "Redirect",
    description: "Route type: redirect",
  },
  "services.routeRemove": {
    message: "Remove rule",
    description: "Remove a route rule (aria-label)",
  },
  "services.routesSave": {
    message: "Save routes",
    description: "Save the routes list",
  },
  "services.staticRoutesSaved": {
    message: "Routes updated",
    description: "Toast after saving routes",
  },
  "services.staticRoutesError": {
    message: "Couldn't update the routes",
    description: "Error toast for a failed routes save",
  },
  "services.headersTitle": {
    message: "Response headers",
    description: "Title for the static-site custom-headers editor",
  },
  "services.headersHint": {
    message: "Custom response headers added to responses whose path matches.",
    description: "Help text for the headers editor",
  },
  "services.headerAdd": {
    message: "Add header",
    description: "Add a custom header",
  },
  "services.headerPath": { message: "Path", description: "Header path column" },
  "services.headerName": { message: "Name", description: "Header name column" },
  "services.headerValue": {
    message: "Value",
    description: "Header value column",
  },
  "services.headerRemove": {
    message: "Remove header",
    description: "Remove a header (aria-label)",
  },
  "services.headersSave": {
    message: "Save headers",
    description: "Save the headers list",
  },
  "services.staticHeadersSaved": {
    message: "Headers updated",
    description: "Toast after saving headers",
  },
  "services.staticHeadersError": {
    message: "Couldn't update the headers",
    description: "Error toast for a failed headers save",
  },
  "services.typeUnknown": {
    message: "Service",
    description: "Service-type badge fallback for an unrecognized type",
  },
  "services.cronRunsTitle": {
    message: "Recent Runs",
    description: "Cron job overview: recent-runs section title",
  },
  "services.cronRunsEmpty": {
    message: "No runs yet.",
    description: "Cron job overview: shown when a cron has no run history",
  },
  "services.cronRunColStarted": {
    message: "Started",
    description: "Cron runs table column header (run start time)",
  },
  "services.cronRunColDuration": {
    message: "Duration",
    description: "Cron runs table column header (elapsed run time)",
  },
  "services.cronRunColStatus": {
    message: "Status",
    description: "Cron runs table column header (run outcome)",
  },
  "services.cronRunColActions": {
    message: "Actions",
    description: "Cron runs table column header (row actions)",
  },
  "services.cronRunStatusRunning": {
    message: "Running",
    description: "Cron run status badge",
  },
  "services.cronRunStatusSucceeded": {
    message: "Succeeded",
    description: "Cron run status badge",
  },
  "services.cronRunStatusFailed": {
    message: "Failed",
    description: "Cron run status badge",
  },
  "services.cronRunStatusCanceled": {
    message: "Canceled",
    description: "Cron run status badge",
  },
  "services.cronRunCancel": {
    message: "Cancel",
    description: "Cancel an in-flight cron run",
  },
  "services.cronRunCancelConfirmTitle": {
    message: "Cancel this run?",
    description: "Cron run cancellation confirmation title",
  },
  "services.cronRunCancelConfirmBody": {
    message: "The running job will be terminated. This can't be undone.",
    description: "Cron run cancellation confirmation body",
  },
  "services.cronRunCancelSuccess": {
    message: "Cron run canceled.",
    description: "Toast after cron run cancellation is accepted",
  },
  "services.cronRunCancelError": {
    message: "Couldn't cancel the cron run.",
    description: "Toast after cron run cancellation fails",
  },
  "services.cronRunsLoadMore": {
    message: "Load more",
    description: "Cron run history pagination button",
  },
  "services.cronRunsLoadingMore": {
    message: "Loading…",
    description: "Cron run history pagination busy label",
  },
  "services.cronRunsLoadError": {
    message: "Couldn't load cron runs.",
    description: "Cron run history read error",
  },
  "services.dangerZoneTitle": {
    message: "Danger Zone",
    description: "Settings tab delete section title (destructive)",
  },
  "services.dangerZoneDescription": {
    message:
      "Deleting a service permanently removes it, its deployment, and its URL. This can't be undone.",
    description: "Settings tab delete section description",
  },
  "services.deleteButton": {
    message: "Delete Service",
    description: "Danger-zone button that opens the delete-confirm dialog",
  },
  "services.deleteConfirmTitle": {
    message: "Delete {name}?",
    description: "Delete-confirm dialog title",
  },
  "services.deleteConfirmBody": {
    message:
      "This permanently removes the service, its deployment, and its URL. This can't be undone.",
    description: "Delete-confirm dialog body",
  },
  "services.deleteConfirmPrompt": {
    message: "Type {name} to confirm",
    description: "Delete-confirm input label naming the immutable service id",
  },
  "services.deleteCancel": {
    message: "Cancel",
    description: "Delete-confirm dialog cancel button",
  },
  "services.deleteConfirm": {
    message: "Delete Service",
    description:
      "Delete-confirm dialog submit button (armed once the name matches)",
  },
  "services.deleteSuccess": {
    message: "Deleted {name}",
    description: "Toast on a successful service delete",
  },
  "services.deleteError": {
    message: "Couldn't delete {name}",
    description: "Toast on a failed service delete",
  },
  "services.newServiceButton": {
    message: "New Service",
    description:
      "Button on the services list page that opens the create wizard",
  },
  "services.createTitle": {
    message: "New Service",
    description: "Create-wizard page title",
  },
  "services.createDescription": {
    message: "Deploy a web service from a Git repo or Docker image.",
    description: "Create-wizard page subtitle",
  },
  "services.createSourceTitle": {
    message: "Source",
    description: "Create-wizard source-picker section label",
  },
  "services.createTabGitHub": {
    message: "GitHub",
    description: "Create-wizard source-tab label for connected GitHub repos",
  },
  "services.createTabPublicGit": {
    message: "Public Git URL",
    description: "Create-wizard source-tab label for a public git URL",
  },
  "services.createTabImage": {
    message: "Existing Image",
    description: "Create-wizard source-tab label for a pre-built Docker image",
  },
  "services.createRepoSearchPlaceholder": {
    message: "Search repositories…",
    description: "Create-wizard GitHub tab repo-search input placeholder",
  },
  "services.createRepoPrivateBadge": {
    message: "Private",
    description: "Badge on a private GitHub repo row in the repo picker",
  },
  "services.createRepoEmpty": {
    message: "No repositories found.",
    description:
      "Create-wizard GitHub tab empty state (no repos in the installation)",
  },
  "services.createRepoNoMatch": {
    message: "No repositories match your search.",
    description:
      "Create-wizard GitHub tab empty state when the search filter returns nothing",
  },
  "services.createGitConnectPromptTitle": {
    message: "Connect GitHub",
    description:
      "Create-wizard GitHub tab connect-prompt heading (no GitHub connection yet)",
  },
  "services.createGitConnectPromptBody": {
    message:
      "Connect your GitHub account to deploy from private or public repositories.",
    description:
      "Create-wizard GitHub tab connect-prompt body (no GitHub connection yet)",
  },
  "services.createGitConnectButton": {
    message: "Connect GitHub",
    description:
      "Create-wizard GitHub tab button that opens the GitHub App install flow",
  },
  "services.createPublicUrlLabel": {
    message: "Repository URL",
    description: "Create-wizard Public Git URL tab input label",
  },
  "services.createPublicUrlPlaceholder": {
    message: "https://github.com/you/your-repo",
    description: "Create-wizard Public Git URL tab input placeholder",
  },
  "services.createPublicUrlError": {
    message: "Enter a valid https://, git@, or git:// URL.",
    description: "Create-wizard Public Git URL tab validation message",
  },
  "services.createImageLabel": {
    message: "Docker Image",
    description: "Create-wizard Existing Image tab input label",
  },
  "services.createImagePlaceholder": {
    message: "docker.io/library/nginx:latest",
    description: "Create-wizard Existing Image tab input placeholder",
  },
  "services.createSettingsTitle": {
    message: "Settings",
    description: "Create-wizard settings section heading",
  },
  "services.createFieldName": {
    message: "Name",
    description: "Create-wizard name input label",
  },
  "services.createFieldNamePlaceholder": {
    message: "my-service",
    description: "Create-wizard name input placeholder",
  },
  "services.createFieldNameError": {
    message:
      "Use lowercase letters, digits, and hyphens (up to 30 characters); can't start or end with a hyphen.",
    description: "Create-wizard name validation message",
  },
  "services.createFieldNameTaken": {
    message: "Name is already in use",
    description:
      "Create-wizard inline error when the service name is already taken in the current workspace (w4/m19)",
  },
  "services.createFieldNameUseSuggestion": {
    message: "Use {name}",
    description:
      "Create-wizard button offering the suggested free name in place of a taken one (w4/m19)",
  },
  "services.createFieldNameChecking": {
    message: "Checking availability…",
    description:
      "Create-wizard transient message while the debounced name-availability check is in flight (w4/m19)",
  },
  "services.createFieldBranch": {
    message: "Branch",
    description: "Create-wizard branch input label (git sources)",
  },
  "services.createFieldBranchPlaceholder": {
    message: "main",
    description: "Create-wizard branch input placeholder",
  },
  "services.createFieldRootDir": {
    message: "Root Directory",
    description: "Create-wizard root-directory input label",
  },
  "services.createFieldRootDirPlaceholder": {
    message: "e.g. backend",
    description: "Create-wizard root-directory input placeholder",
  },
  "services.createFieldRootDirHint": {
    message: "Subdirectory to build from. Leave blank to use the repo root.",
    description: "Create-wizard root-directory input hint text",
  },
  "services.createFieldRuntime": {
    message: "Runtime",
    description: "Create-wizard Render-compatible runtime selector label",
  },
  "services.createFieldBuildCommand": {
    message: "Build Command",
    description: "Create-wizard Render-compatible build command label",
  },
  "services.createFieldStartCommand": {
    message: "Start Command",
    description: "Create-wizard Render-compatible start command label",
  },
  "services.createFieldDockerfilePath": {
    message: "Dockerfile Path",
    description: "Create-wizard Dockerfile-path label for the Docker runtime",
  },
  "services.createFieldDockerfilePathPlaceholder": {
    message: "Dockerfile",
    description: "Create-wizard Dockerfile-path placeholder",
  },
  "services.createFieldDockerfilePathHint": {
    message: "Path relative to the Root Directory. Leave blank for Dockerfile.",
    description: "Create-wizard Dockerfile-path help text",
  },
  "services.createFieldDockerCommand": {
    message: "Docker Command",
    description: "Create-wizard Docker CMD override label (Render wording)",
  },
  "services.createFieldDockerCommandPlaceholder": {
    message: "Use the Dockerfile CMD",
    description: "Create-wizard optional Docker Command placeholder",
  },
  "services.createRuntimeNode": {
    message: "Node",
    description: "Create-wizard Node runtime option",
  },
  "services.createRuntimePython": {
    message: "Python 3",
    description: "Create-wizard Python runtime option",
  },
  "services.createRuntimeGo": {
    message: "Go",
    description: "Create-wizard Go runtime option",
  },
  "services.createRuntimeRuby": {
    message: "Ruby",
    description: "Create-wizard Ruby runtime option",
  },
  "services.createRuntimeRust": {
    message: "Rust",
    description: "Create-wizard Rust runtime option",
  },
  "services.createRuntimeElixir": {
    message: "Elixir",
    description: "Create-wizard Elixir runtime option",
  },
  "services.createRuntimeDocker": {
    message: "Docker",
    description: "Create-wizard Docker runtime option",
  },
  "services.createFieldPlan": {
    message: "Instance Type",
    description: "Create-wizard plan-picker section label",
  },
  "services.createFieldAutoDeploy": {
    message: "Auto-deploy on push",
    description: "Create-wizard auto-deploy toggle label",
  },
  "services.createFieldAutoDeployHint": {
    message: "Automatically redeploy when you push to this branch.",
    description: "Create-wizard auto-deploy toggle hint text",
  },
  "services.createCancel": {
    message: "Cancel",
    description: "Create-wizard cancel button",
  },
  "services.createSubmit": {
    message: "Deploy Service",
    description: "Create-wizard submit button",
  },
  "services.createSuccess": {
    message: "Deploying {name}…",
    description: "Toast shown after createService succeeds",
  },
  "services.createError": {
    message: "Couldn't create {name}. Please try again.",
    description: "Toast shown after createService fails",
  },
  "services.scalingInstanceCount": {
    message: "Instance count",
    description:
      "Settings row label for the manual instance-count stepper (w5/m16)",
  },
  "services.scalingInstanceCountHint": {
    message: "Number of instances to run simultaneously.",
    description: "Settings row help text for the manual instance-count stepper",
  },
  "services.scalingDecrement": {
    message: "Decrease instance count",
    description: "aria-label for the − stepper button",
  },
  "services.scalingIncrement": {
    message: "Increase instance count",
    description: "aria-label for the + stepper button",
  },
  "services.scalingSaveCount": {
    message: "Save",
    description: "Save button label on the instance-count stepper",
  },
  "services.scaleSuccess": {
    message: "Scaled to {count} instance(s).",
    description: "Toast shown after scaleService succeeds",
  },
  "services.scaleError": {
    message: "Failed to update instance count.",
    description: "Toast shown after scaleService fails",
  },
  "services.createTypePickerTitle": {
    message: "Service Type",
    description: "Label above the service type picker in the create wizard",
  },
  "services.createTypeWebDesc": {
    message: "Expose your service on a public URL",
    description:
      "Description shown under the Web Service type card in the create wizard",
  },
  "services.createTypePrivateDesc": {
    message: "Accessible only within the platform network",
    description: "Description shown under the Private Service type card",
  },
  "services.createTypeWorkerDesc": {
    message: "Run background processing with no port or URL",
    description: "Description shown under the Background Worker type card",
  },
  "services.createTypeCronDesc": {
    message: "Run a command on a recurring schedule",
    description: "Description shown under the Cron Job type card",
  },
  "services.createTypeStaticDesc": {
    message: "Build and serve a static site from object storage",
    description: "Description shown under the Static Site type card",
  },
  "services.createFieldSchedule": {
    message: "Schedule",
    description: "Label for the cron schedule field in the create wizard",
  },
  "services.createFieldSchedulePlaceholder": {
    message: "0 0 * * *",
    description: "Placeholder for the cron schedule field",
  },
  "services.createFieldScheduleHint": {
    message: "A 5-field crontab expression (minute hour day month weekday).",
    description: "Hint text under the schedule field",
  },
  "services.createFieldScheduleError": {
    message: "Enter a valid 5-field cron expression, e.g. 0 0 * * *.",
    description: "Validation error for an invalid cron expression",
  },
  "services.createFieldCommand": {
    message: "Command",
    description: "Label for the command field in the create wizard",
  },
  "services.createFieldCommandPlaceholder": {
    message: "python script.py",
    description: "Placeholder for the command field",
  },
  "services.createFieldCommandHint": {
    message: "The command to run on each scheduled invocation.",
    description: "Hint text under the command field",
  },
  "services.createFieldPublishPath": {
    message: "Publish Directory",
    description: "Label for the publish directory field in the create wizard",
  },
  "services.createFieldPublishPathPlaceholder": {
    message: "dist",
    description: "Placeholder for the publish directory field",
  },
  "services.createFieldPublishPathHint": {
    message:
      "The built output directory to serve as the site root (e.g. dist, build, public).",
    description: "Hint text under the publish directory field",
  },
  "services.createNoPublicUrlNote": {
    message: "This service type has no public URL.",
    description:
      "Note shown for private/worker types that don't produce a public URL",
  },
  "services.createFieldEnvVarsTitle": {
    message: "Environment Variables",
    description: "Section heading for env vars in the create wizard",
  },
  "services.createFieldEnvVarsAdd": {
    message: "Add Variable",
    description: "Button to add an env var row in the create wizard",
  },
  "services.createFieldEnvVarsRemove": {
    message: "Remove",
    description: "Button to remove an env var row in the create wizard",
  },
  "services.createFieldEnvVarsKey": {
    message: "Key",
    description: "Label for the env var key column in the create wizard",
  },
  "services.createFieldEnvVarsValue": {
    message: "Value",
    description: "Label for the env var value column in the create wizard",
  },
  "services.createFieldEnvVarsKeyPlaceholder": {
    message: "KEY_NAME",
    description: "Placeholder for the env var key input in the create wizard",
  },
  "services.createFieldEnvVarsValuePlaceholder": {
    message: "value",
    description: "Placeholder for the env var value input in the create wizard",
  },
  "services.createFieldEnvVarsKeyError": {
    message:
      "Keys must start with a letter or underscore and contain only letters, digits, and underscores.",
    description:
      "Error shown when an env var key is invalid in the create wizard",
  },
  "services.createFieldSecretFilesTitle": {
    message: "Secret Files",
    description: "Section heading for secret files in the create wizard",
  },
  "services.createFieldSecretFilesHint": {
    message: "Mounted read-only under /etc/secrets from the first deploy.",
    description: "Hint for create-time secret files",
  },
  "services.createFieldSecretFilesAdd": {
    message: "Add Secret File",
    description: "Button to add a create-time secret file",
  },
  "services.createFieldSecretFilesRemove": {
    message: "Remove secret file",
    description: "Accessible label for removing a secret file row",
  },
  "services.createFieldSecretFilesName": {
    message: "Secret file name",
    description: "Accessible label for a secret file name",
  },
  "services.createFieldSecretFilesContent": {
    message: "Secret file contents",
    description: "Accessible label for secret file contents",
  },
  "services.createFieldSecretFilesNamePlaceholder": {
    message: "credentials.json",
    description: "Placeholder for a secret file name",
  },
  "services.createFieldSecretFilesContentPlaceholder": {
    message: "Paste secret contents",
    description: "Placeholder for secret file contents",
  },
  "services.createFieldSecretFilesNameError": {
    message:
      "Use only letters, digits, dots, dashes, and underscores; . and .. are not allowed.",
    description: "Invalid secret file name error",
  },
  "services.createFieldEnvironmentTitle": {
    message: "Project and Environment",
    description: "Create wizard grouping section title",
  },
  "services.createFieldProject": {
    message: "Project",
    description: "Accessible label for the create project picker",
  },
  "services.createFieldProjectNone": {
    message: "No project",
    description: "Unassigned option in the project picker",
  },
  "services.createFieldEnvironment": {
    message: "Environment",
    description: "Accessible label for the create environment picker",
  },
  "services.createFieldEnvironmentNone": {
    message: "No environment",
    description: "Unassigned option in the environment picker",
  },
  "services.createFieldEnvironmentHint": {
    message:
      "Selecting an environment also adds the service to its parent project.",
    description: "Hint for create-time environment assignment",
  },
  "services.navEvents": {
    message: "Events",
    description: "Service-detail nav item (events tab)",
  },
  "services.navDeploys": {
    message: "Deploys",
    description:
      "Service-detail nav item (dedicated deploy-history tab, w9/002)",
  },
  "services.eventsTitle": {
    message: "Activity",
    description: "Events tab card title",
  },
  "services.eventsDescription": {
    message: "Recent deploys and service changes.",
    description: "Events tab card description",
  },
  "services.eventsCount": {
    message: "{count} recent events",
    description: "Accessible label for the number of visible service events",
  },
  "services.eventsEmptyTitle": {
    message: "No activity yet",
    description: "Events tab empty-state title",
  },
  "services.eventsEmpty": {
    message: "Deploys and service changes will appear here.",
    description: "Events tab empty-state description",
  },
  "services.eventsErrorTitle": {
    message: "Activity is unavailable",
    description: "Events tab query-error title",
  },
  "services.eventsErrorDescription": {
    message: "We couldn't load recent activity. Try again in a moment.",
    description: "Events tab query-error description",
  },
  "services.eventsRetry": {
    message: "Try again",
    description: "Events tab query-error retry button",
  },
  "services.eventsActor": {
    message: "by {actor}",
    description: "Actor attribution shown on a service event",
  },
  "services.eventsDeployReference": {
    message: "Deploy {id}",
    description: "Deploy identifier shown on a deploy activity row",
  },
  "services.eventsTriggerRollback": {
    message: "Rollback",
    description: "Deploy event trigger: rollback",
  },
  "services.eventsTriggerFirstBuild": {
    message: "First build",
    description: "Deploy event trigger: initial build",
  },
  "services.eventsTriggerManual": {
    message: "Manual deploy",
    description: "Deploy event trigger: manual",
  },
  "services.eventsTriggerEnvUpdated": {
    message: "Environment updated",
    description: "Deploy event trigger: environment update",
  },
  "services.eventsTriggerClearCache": {
    message: "Cache cleared",
    description: "Deploy event trigger: build-cache clear",
  },
  "services.eventsTriggerDeployedByRender": {
    message: "Platform deploy",
    description: "Deploy event trigger: platform initiated",
  },
  "services.eventsTypeDeployStarted": {
    message: "Deploy started",
    description: "Service activity type: deploy started",
  },
  "services.eventsTypeDeployFinished": {
    message: "Deploy finished",
    description: "Service activity type: deploy finished",
  },
  "services.eventsTypeSuspended": {
    message: "Service suspended",
    description: "Service activity type: service suspended",
  },
  "services.eventsTypeResumed": {
    message: "Service resumed",
    description: "Service activity type: service resumed",
  },
  "services.eventsTypeRestarted": {
    message: "Service restarted",
    description: "Service activity type: service restarted",
  },
  "services.eventsTypePlanChanged": {
    message: "Instance type changed",
    description: "Service activity type: plan changed",
  },
  "services.eventsTypeInstanceCountChanged": {
    message: "Instance count changed",
    description: "Service activity type: manual scale",
  },
  "services.eventsTypeAutoscalingChanged": {
    message: "Autoscaling updated",
    description: "Service activity type: autoscaling configuration changed",
  },
  "services.eventsTypeCronRunStarted": {
    message: "Cron run started",
    description: "Service activity type: cron run started",
  },
  "services.eventsTypeCronRunFinished": {
    message: "Cron run finished",
    description: "Service activity type: cron run finished",
  },
  "services.eventsTypeEnvVarsChanged": {
    message: "Environment variables changed",
    description: "Service activity type: environment variables changed",
  },
  "services.eventsTypeEnvGroupLinked": {
    message: "Environment group linked",
    description: "Service activity type: environment group linked",
  },
  "services.eventsTypeEnvGroupUnlinked": {
    message: "Environment group unlinked",
    description: "Service activity type: environment group unlinked",
  },
  "services.eventsTypeAutoDeployChanged": {
    message: "Auto-deploy updated",
    description: "Service activity type: auto-deploy setting changed",
  },
  "services.eventsTypeIdleTimeoutChanged": {
    message: "Idle timeout updated",
    description: "Service activity type: idle timeout changed",
  },
  "services.eventsTypeDisplayNameChanged": {
    message: "Display name changed",
    description: "Service activity type: display name changed",
  },
  "services.eventsTypeCustomDomainAdded": {
    message: "Custom domain added",
    description: "Service activity type: custom domain added",
  },
  "services.eventsTypeCustomDomainRemoved": {
    message: "Custom domain removed",
    description: "Service activity type: custom domain removed",
  },
  "services.eventsTypeNotificationsChanged": {
    message: "Failure notifications updated",
    description: "Service activity type: failure notification setting changed",
  },
  "services.eventsTypeSubdomainPolicyChanged": {
    message: "Platform subdomain updated",
    description: "Service activity type: platform subdomain policy changed",
  },
  "services.eventsTypeStaticSiteChanged": {
    message: "Static site settings changed",
    description: "Service activity type: static-site configuration changed",
  },
  "services.eventsTypeBuildSettingsChanged": {
    message: "Build and deploy settings changed",
    description: "Service activity type: build or deploy configuration changed",
  },
  "services.eventsTypeServiceChanged": {
    message: "Service settings changed",
    description: "Fallback service activity type",
  },
  "services.eventsManualDeploy": {
    message: "Manual Deploy",
    description: "Button to trigger a new deploy",
  },
  "services.deployMenuLatestCommit": {
    message: "Deploy latest commit",
    description:
      "Manual Deploy dropdown item, repo-backed service: rebuild and redeploy from the branch's HEAD",
  },
  "services.deployMenuLatestImage": {
    message: "Deploy latest image",
    description:
      "Manual Deploy dropdown item, image-backed service (no repo to rebuild from)",
  },
  "services.deployMenuRestart": {
    message: "Restart service",
    description:
      "Manual Deploy dropdown item: roll the service's pods without rebuilding",
  },
  "services.deployConfirmCommitTitle": {
    message: "Deploy the latest commit on {branch}?",
    description: "Confirm dialog title for a repo-backed manual deploy",
  },
  "services.deployConfirmCommitBody": {
    message:
      "Rebuilds {name} from the latest commit on {branch} and redeploys it.",
    description: "Confirm dialog body for a repo-backed manual deploy",
  },
  "services.deployConfirmImageTitle": {
    message: "Redeploy {name}?",
    description: "Confirm dialog title for an image-backed manual deploy",
  },
  "services.deployConfirmImageBody": {
    message:
      "Restarts {name} using its current image. There's no source repo to rebuild from.",
    description: "Confirm dialog body for an image-backed manual deploy",
  },
  "services.eventsManualDeployConfirmTitle": {
    message: "Trigger a new deploy?",
    description: "Manual deploy confirm dialog title",
  },
  "services.eventsManualDeployConfirmBody": {
    message:
      "This will rebuild and redeploy the service from its current image or branch.",
    description: "Manual deploy confirm dialog body",
  },
  "services.eventsCancelDeploy": {
    message: "Cancel",
    description: "Button to cancel an in-progress deploy",
  },
  "services.eventsCancelConfirmTitle": {
    message: "Cancel this deploy?",
    description: "Cancel deploy confirm dialog title",
  },
  "services.eventsCancelConfirmBody": {
    message:
      "The in-progress deploy will be stopped. The last successful deploy remains live.",
    description: "Cancel deploy confirm dialog body",
  },
  "services.eventsRollback": {
    message: "Roll Back to This Deploy",
    description: "Button to roll back to a specific deploy",
  },
  "services.eventsRollbackConfirmTitle": {
    message: "Roll back to this deploy?",
    description: "Rollback confirm dialog title",
  },
  "services.eventsRollbackConfirmBody": {
    message: "The service will redeploy from the image used in this deploy.",
    description: "Rollback confirm dialog body",
  },
  "services.eventsConfirmProceed": {
    message: "Proceed",
    description: "Confirm dialog proceed button",
  },
  "services.eventsConfirmCancel": {
    message: "Go Back",
    description: "Confirm dialog cancel button",
  },
  "services.triggerDeploySuccess": {
    message: "Deploy triggered.",
    description: "Toast after triggerDeploy succeeds",
  },
  "services.triggerDeployError": {
    message: "Couldn't trigger deploy.",
    description: "Toast after triggerDeploy fails",
  },
  "services.cancelDeploySuccess": {
    message: "Deploy cancelled.",
    description: "Toast after cancelDeploy succeeds",
  },
  "services.cancelDeployError": {
    message: "Couldn't cancel deploy.",
    description: "Toast after cancelDeploy fails",
  },
  "services.rollbackSuccess": {
    message: "Rollback triggered.",
    description: "Toast after rollbackService succeeds",
  },
  "services.rollbackError": {
    message: "Couldn't roll back.",
    description: "Toast after rollbackService fails",
  },
  "services.eventsStatusLive": {
    message: "Live",
    description: "Deploy status: live",
  },
  "services.eventsStatusInProgress": {
    message: "In Progress",
    description: "Deploy status: update_in_progress",
  },
  "services.eventsStatusFailed": {
    message: "Failed",
    description: "Deploy status: update_failed",
  },
  "services.eventsStatusCanceled": {
    message: "Canceled",
    description: "Deploy status: canceled",
  },
  "services.eventsPreDeployRunning": {
    message: "Pre-deploy command running",
    description: "Deploy row: the pre-deploy step is in progress",
  },
  "services.eventsPreDeploySucceeded": {
    message: "Pre-deploy command succeeded",
    description: "Deploy row: the pre-deploy step passed",
  },
  "services.eventsPreDeployFailed": {
    message: "Pre-deploy command failed",
    description:
      "Deploy row: the pre-deploy step failed (distinct from a health-check failure)",
  },
  "services.eventsRolledBackFrom": {
    message: "Rolled back from {target}",
    description: "Deploy row: provenance note when trigger=rollback",
  },
  "services.capLimitTitle": {
    message: "Service limit reached",
    description:
      "Alert title when the workspace's service creation cap is hit (w7/m9)",
  },
  "services.capLimitUpgrade": {
    message: "Upgrade plan",
    description: "Upgrade CTA button inside the cap-limit Alert (w7/m9)",
  },
  "services.networkingTitle": {
    message: "Networking",
    description: "Settings Networking card title (w7/m32)",
  },
  "services.networkingDescription": {
    message: "Restrict inbound HTTP traffic to these source CIDRs.",
    description: "Settings Networking card description (w7/m32)",
  },
  "services.networkingHint": {
    message:
      "Enter a CIDR block (e.g. 203.0.113.0/24) and press Add. Empty list opens the service to all source IPs.",
    description: "Hint below the CIDR list in the Networking card (w7/m32)",
  },
  "services.networkingOpen": {
    message: "Open to all source IPs",
    description:
      "Placeholder shown when the allow list is empty (Render default, w7/m32)",
  },
  "services.networkingAdd": {
    message: "Add",
    description: "Button to add a CIDR to the draft list (w7/m32)",
  },
  "services.networkingSave": {
    message: "Save",
    description: "Button to persist the CIDR list (w7/m32)",
  },
  "services.networkingRemove": {
    message: "Remove {cidr}",
    description:
      "Accessible label on the trash icon next to a CIDR tag (w7/m32)",
  },
  "services.networkingSaved": {
    message: "IP allowlist updated",
    description: "Toast on successful setServiceIpAllowList mutation (w7/m32)",
  },
  "services.networkingError": {
    message: "Failed to update IP allowlist: {error}",
    description: "Toast on failed setServiceIpAllowList mutation (w7/m32)",
  },
};

export default enServices;
