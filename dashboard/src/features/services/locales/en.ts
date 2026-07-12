import type { TranslationEntry } from "@/i18n";

const enServices: Record<string, TranslationEntry> = {
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
  "services.navOverview": {
    message: "Overview",
    description: "Service-detail nav item + overview panel title",
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
  "services.overviewPhase": {
    message: "Phase",
    description: "Overview panel field label (operator phase, verbatim)",
  },
  "services.overviewSuspended": {
    message: "Suspended",
    description: "Overview panel field label (suspend state)",
  },
  "services.overviewYes": {
    message: "Yes",
    description: "Overview panel value for a true boolean field",
  },
  "services.overviewNo": {
    message: "No",
    description: "Overview panel value for a false boolean field",
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
    message: "Use letters, digits, dot, dash and underscore.",
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
    message: "Configure this service's instance size and other settings.",
    description: "Settings tab card description",
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
      "Your service is always reachable at its bex platform subdomain, in addition to any custom domains.",
    description: "Settings tab platform-subdomain section description",
  },
  "services.platformSubdomainEnabled": {
    message: "Always enabled",
    description: "Platform-subdomain badge: the subdomain can't be turned off",
  },
  "services.platformSubdomainPending": {
    message: "The platform URL is assigned once the service is running.",
    description: "Platform-subdomain state when the service has no URL yet",
  },
  "services.deployTitle": {
    message: "Deploy",
    description: "Cron job Settings tab: Deploy section title (Render parity)",
  },
  "services.deployDescription": {
    message: "How this cron job runs — read-only for now.",
    description: "Cron job Settings tab: Deploy section description",
  },
  "services.deployScheduleLabel": {
    message: "Schedule",
    description: "Cron job Settings tab: Deploy section schedule field label",
  },
  "services.deployScheduleHint": {
    message: "Runs this command on this schedule (5-field crontab).",
    description: "Cron job Settings tab: Deploy section schedule help text",
  },
  "services.deployCommandLabel": {
    message: "Command",
    description: "Cron job Settings tab: Deploy section command field label",
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
  "services.typeUnknown": {
    message: "Service",
    description: "Service-type badge fallback for an unrecognized type",
  },
  "services.overviewType": {
    message: "Type",
    description: "Overview tab row label for the service type",
  },
  "services.overviewSchedule": {
    message: "Schedule",
    description: "Overview tab row label for a cron job's schedule",
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
  "services.cronRunColStatus": {
    message: "Status",
    description: "Cron runs table column header (run outcome)",
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
    description: "Delete-confirm input label naming the exact service name",
  },
  "services.deleteCancel": {
    message: "Cancel",
    description: "Delete-confirm dialog cancel button",
  },
  "services.deleteConfirm": {
    message: "Delete Service",
    description: "Delete-confirm dialog submit button (armed once the name matches)",
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
    description: "Button on the services list page that opens the create wizard",
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
      "Use lowercase letters, digits, and hyphens; can't start or end with a hyphen.",
    description: "Create-wizard name validation message",
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
    message:
      "Subdirectory to build from. Leave blank to use the repo root.",
    description: "Create-wizard root-directory input hint text",
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
};

export default enServices;
