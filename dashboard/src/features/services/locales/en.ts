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
  "services.logsComingSoonTitle": {
    message: "Logs are coming soon",
    description: "Logs tab placeholder title (content ships in a later release)",
  },
  "services.logsComingSoonBody": {
    message: "Live log tailing for this service ships in an upcoming release.",
    description: "Logs tab placeholder body",
  },
};

export default enServices;
