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
    description: "Services table card title, also used as the metrics page back-link",
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
  "services.statusHibernated": {
    message: "Hibernated",
    description: "Services table status badge (App scaled to zero)",
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
    description: "Environment tab state when the secret store is unconfigured (503)",
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
    message: "You don't have permission to view this service's environment variables.",
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
    description: "Environment add-variable validation message for an invalid key",
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
    description: "Toast description after an env-var write (bex rolls the pods)",
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
  "services.planPickerTitle": {
    message: "Pick an Instance Type",
    description: "Plan-picker page heading",
  },
  "services.planPickerFreeGroup": {
    message: "Free",
    description: "Plan-picker section label separating the Free tier from paid tiers",
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
};

export default enServices;
