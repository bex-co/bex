import type { TranslationEntry } from "@/i18n";

const enWebhooks: Record<string, TranslationEntry> = {
  "webhooks.title": {
    message: "Webhooks",
    description: "Settings Integrations Webhooks section card title",
  },
  "webhooks.description": {
    message:
      "Push signed event notifications (deploys, suspends, restarts, scaling) to your own endpoints — no polling. The signing secret is shown once at creation.",
    description: "Settings Webhooks section card description",
  },
  "webhooks.colEndpoint": {
    message: "Endpoint",
    description: "Webhooks table column header — name + destination URL",
  },
  "webhooks.colEvents": {
    message: "Events",
    description: "Webhooks table column header — subscribed event types",
  },
  "webhooks.colEnabled": {
    message: "Enabled",
    description: "Webhooks table column header — the enable/disable switch",
  },
  "webhooks.emptyTitle": {
    message: "No webhooks",
    description: "Webhooks empty-state title",
  },
  "webhooks.emptyBody": {
    message:
      "Add an endpoint to get notified when your services deploy, restart, or scale.",
    description: "Webhooks empty-state body",
  },
  "webhooks.forbiddenTitle": {
    message: "Not authorized",
    description: "Webhooks state when the caller lacks permission (403)",
  },
  "webhooks.forbiddenBody": {
    message: "You don't have permission to view this workspace's webhooks.",
    description: "Webhooks forbidden-state body",
  },
  "webhooks.errorTitle": {
    message: "Couldn't load webhooks",
    description: "Webhooks generic error title",
  },
  "webhooks.errorBody": {
    message: "Something went wrong. Please try again.",
    description: "Webhooks generic error body",
  },
  "webhooks.create": {
    message: "Add webhook",
    description: "Button that opens the create dialog",
  },
  "webhooks.createTitle": {
    message: "Add a webhook",
    description: "Create dialog title",
  },
  "webhooks.createDescription": {
    message:
      "bex will POST a signed JSON payload to this URL whenever a subscribed event happens.",
    description: "Create dialog description",
  },
  "webhooks.createEnabledHelp": {
    message:
      "Start delivering immediately. You can enable it later from Settings.",
    description: "Create page initial enabled-state helper copy",
  },
  "webhooks.fieldName": {
    message: "Name",
    description: "Create dialog name field label",
  },
  "webhooks.fieldNamePlaceholder": {
    message: "e.g. deploy-alerts-slack-bot",
    description: "Create dialog name field placeholder",
  },
  "webhooks.fieldUrl": {
    message: "Destination URL",
    description: "Create dialog URL field label",
  },
  "webhooks.fieldEvents": {
    message: "Events to send",
    description: "Create dialog event-type checklist label",
  },
  "webhooks.eventsLoading": {
    message: "Loading event types…",
    description: "Create dialog while the event-type vocabulary loads",
  },
  "webhooks.createCancel": {
    message: "Cancel",
    description: "Create dialog cancel button",
  },
  "webhooks.createSubmit": {
    message: "Create webhook",
    description: "Create page submit button (Render: 'Create Webhook')",
  },
  "webhooks.createSuccess": {
    message: "Webhook created",
    description: "Toast after a successful create",
  },
  "webhooks.createError": {
    message: "Couldn't create the webhook",
    description: "Toast after a failed create",
  },
  "webhooks.createdTitle": {
    message: "Webhook created",
    description: "Secret-reveal step title",
  },
  "webhooks.createdWarning": {
    message:
      "Copy the signing secret now — you won't be able to see it again. Use it to verify the webhook-signature header on every delivery.",
    description: "Secret-reveal step warning (shown exactly once)",
  },
  "webhooks.copy": {
    message: "Copy",
    description: "Copy-secret button label",
  },
  "webhooks.copied": {
    message: "Secret copied to clipboard",
    description: "Toast after copying the secret",
  },
  "webhooks.copyError": {
    message: "Couldn't copy — select the text and copy it manually",
    description: "Toast when clipboard write fails",
  },
  "webhooks.createdDone": {
    message: "Done",
    description: "Secret-reveal step dismiss button",
  },
  "webhooks.toggle": {
    message: "Enabled",
    description: "Accessible label of the per-endpoint enable/disable switch",
  },
  "webhooks.enableSuccess": {
    message: "{name} enabled",
    description: "Toast after enabling an endpoint",
  },
  "webhooks.disableSuccess": {
    message: "{name} disabled",
    description: "Toast after disabling an endpoint",
  },
  "webhooks.toggleError": {
    message: "Couldn't update {name}",
    description: "Toast after a failed enable/disable",
  },
  "webhooks.delete": {
    message: "Delete",
    description: "Delete action label (icon button + confirm button)",
  },
  "webhooks.deleteConfirmTitle": {
    message: "Delete {name}?",
    description: "Delete confirmation dialog title",
  },
  "webhooks.deleteConfirmBody": {
    message:
      "No further events will be sent to this endpoint, and its delivery history will be removed.",
    description: "Delete confirmation dialog body",
  },
  "webhooks.deleteCancel": {
    message: "Cancel",
    description: "Delete confirmation cancel button",
  },
  "webhooks.deleteSuccess": {
    message: "{name} deleted",
    description: "Toast after a successful delete",
  },
  "webhooks.deleteError": {
    message: "Couldn't delete {name}",
    description: "Toast after a failed delete",
  },
  "webhooks.history": {
    message: "Delivery history",
    description: "Accessible label of the per-endpoint history button",
  },
  "webhooks.historyTitle": {
    message: "Deliveries — {name}",
    description: "Delivery-history dialog title",
  },
  "webhooks.historyBody": {
    message:
      "Each event sent to this endpoint, newest first. Failed deliveries retry on an exponential backoff; repeated failure disables the endpoint.",
    description: "Delivery-history dialog description",
  },
  "webhooks.historyEmptyTitle": {
    message: "No deliveries yet",
    description: "Delivery-history empty-state title",
  },
  "webhooks.historyEmptyBody": {
    message:
      "Trigger a subscribed event — a deploy, for example — to see it here.",
    description: "Delivery-history empty-state body",
  },
  "webhooks.historyErrorTitle": {
    message: "Couldn't load deliveries",
    description: "Delivery-history error-state title",
  },
  "webhooks.historyErrorBody": {
    message: "Something went wrong. Please try again.",
    description: "Delivery-history error-state body",
  },
  "webhooks.colEvent": {
    message: "Event",
    description: "Delivery-history table column header",
  },
  "webhooks.colService": {
    message: "Service",
    description: "Delivery-history table column header",
  },
  "webhooks.colStatus": {
    message: "Status",
    description: "Delivery-history table column header",
  },
  "webhooks.colAttempts": {
    message: "Attempt",
    description:
      "Delivery-history table column header — one-based attempt number",
  },
  "webhooks.colResponse": {
    message: "Response",
    description: "Delivery-history table column header — last HTTP status",
  },
  "webhooks.colWhen": {
    message: "When",
    description:
      "Delivery-history table column header — relative and exact send time",
  },
  "webhooks.colActions": {
    message: "Actions",
    description: "Accessible delivery-history table actions column header",
  },
  "webhooks.status.pending": {
    message: "Pending",
    description: "Delivery status badge — queued or between retries",
  },
  "webhooks.status.delivered": {
    message: "Delivered",
    description: "Delivery status badge — endpoint answered 2xx",
  },
  "webhooks.status.failed": {
    message: "Failed",
    description: "Delivery status badge — this individual attempt failed",
  },
  "webhooks.httpStatus": {
    message: "HTTP {status}",
    description: "Delivery attempt HTTP status summary",
  },
  "webhooks.openSourceEvent": {
    message: "Open source event {id}",
    description: "Accessible label for the event hydration link",
  },
  "webhooks.requestPayload": {
    message: "Request payload",
    description: "Expanded delivery-attempt request evidence heading",
  },
  "webhooks.endpointResponse": {
    message: "Endpoint response",
    description: "Expanded delivery-attempt response evidence heading",
  },
  "webhooks.noRequestEvidence": {
    message: "No request evidence is available.",
    description: "Expanded attempt with no retained request body",
  },
  "webhooks.noResponseEvidence": {
    message: "No response was recorded.",
    description: "Expanded pending/transport attempt with no endpoint body",
  },
  "webhooks.attemptIdentity": {
    message: "Attempt {attemptId} · source event {eventId}",
    description: "Immutable attempt and stable source-event identity",
  },
  "webhooks.parentDeliveryStatus": {
    message: "Notification state: {status}",
    description:
      "Logical notification state shown separately from attempt outcome",
  },
  "webhooks.retryScheduled": {
    message: "Next automatic attempt: {date}",
    description: "Parent notification retry schedule",
  },
  "webhooks.resend": {
    message: "Resend",
    description: "Manual resend action for a failed attempt",
  },
  "webhooks.resending": {
    message: "Resending…",
    description: "Manual resend action in progress",
  },
  "webhooks.resendConfirmTitle": {
    message: "Resend this webhook?",
    description: "Manual resend confirmation title",
  },
  "webhooks.resendConfirmBody": {
    message:
      "This queues a new signed attempt with the same source event and request payload.",
    description: "Manual resend confirmation consequence",
  },
  "webhooks.resendCancel": {
    message: "Cancel",
    description: "Manual resend confirmation cancel action",
  },
  "webhooks.resendSuccess": {
    message: "Webhook resend queued",
    description: "Toast after a manual resend reservation succeeds",
  },
  "webhooks.resendError": {
    message: "Couldn't resend this webhook",
    description: "Toast after a manual resend is refused or fails",
  },
  "webhooks.loadMore": {
    message: "Load more",
    description: "Delivery-history pagination button",
  },
  // --- event picker (w1/m49/t002) ---
  "webhooks.selectedCount": {
    message: "{count} events selected",
    description: "Event-picker live selection counter",
  },
  "webhooks.searchEvents": {
    message: "Search for events",
    description: "Event-picker search box placeholder + accessible label",
  },
  "webhooks.searchNoMatches": {
    message: "No events match your search",
    description:
      "Event-picker empty state while a search filters everything out",
  },
  "webhooks.allEvents": {
    message: "All events",
    description: "Event-picker tri-state master checkbox label",
  },
  "webhooks.groupToggle": {
    message: "Toggle {group} events",
    description: "Accessible label of a group's expand/collapse chevron",
  },
  "webhooks.group.deploy": {
    message: "Deploy",
    description: "Event-picker group — deploy lifecycle events",
  },
  "webhooks.group.autoDeploy": {
    message: "Auto-Deploy",
    description: "Event-picker group — automatic deploy setting events",
  },
  "webhooks.group.serviceAvailability": {
    message: "Service Availability",
    description: "Event-picker group — observed service availability events",
  },
  "webhooks.group.scaling": {
    message: "Scaling",
    description: "Event-picker group — service scaling events",
  },
  "webhooks.group.cronJobRun": {
    message: "Cron Job Run",
    description: "Event-picker group — cron job run events",
  },
  "webhooks.group.maintenanceMode": {
    message: "Maintenance Mode",
    description: "Event-picker group — maintenance mode events",
  },
  "webhooks.group.postgres": {
    message: "Postgres",
    description: "Event-picker group — managed Postgres events",
  },
  "webhooks.group.suspension": {
    message: "Suspension",
    description: "Event-picker group — suspend/resume events",
  },
  "webhooks.group.other": {
    message: "Other",
    description:
      "Event-picker fallback group for served keys the catalog doesn't know yet",
  },
  "webhooks.event.deploy_started": {
    message: "Deploy Started",
    description: "Event label — deploy_started",
  },
  "webhooks.event.branch_deleted": {
    message: "Branch Deleted",
    description: "Webhook event label",
  },
  "webhooks.event.build_started": {
    message: "Build Started",
    description: "Webhook event label",
  },
  "webhooks.event.build_ended": {
    message: "Build Ended",
    description: "Webhook event label",
  },
  "webhooks.event.pre_deploy_started": {
    message: "Pre-Deploy Started",
    description: "Webhook event label",
  },
  "webhooks.event.pre_deploy_ended": {
    message: "Pre-Deploy Ended",
    description: "Webhook event label",
  },
  "webhooks.event.job_run_ended": {
    message: "Job Run Ended",
    description: "Webhook event label",
  },
  "webhooks.event.auto_deploy_enabled": {
    message: "Auto-Deploy Enabled",
    description: "Webhook event label",
  },
  "webhooks.event.auto_deploy_disabled": {
    message: "Auto-Deploy Disabled",
    description: "Webhook event label",
  },
  "webhooks.event.deploy_ended": {
    message: "Deploy Ended",
    description: "Event label — deploy_ended",
  },
  "webhooks.event.image_pull_failed": {
    message: "Image Pull Failed",
    description: "Event label — image_pull_failed",
  },
  "webhooks.event.commit_ignored": {
    message: "Commit Ignored",
    description: "Event label — commit_ignored",
  },
  "webhooks.event.server_failed": {
    message: "Server Failed",
    description: "Event label — server_failed",
  },
  "webhooks.event.server_available": {
    message: "Server Available",
    description: "Event label — server_available",
  },
  "webhooks.event.autoscaling_started": {
    message: "Autoscaling Started",
    description: "Event label — autoscaling_started",
  },
  "webhooks.event.autoscaling_ended": {
    message: "Autoscaling Ended",
    description: "Event label — autoscaling_ended",
  },
  "webhooks.event.branch_changed": {
    message: "Branch Changed",
    description: "Event label — branch_changed bex extension",
  },
  "webhooks.event.cron_job_run_started": {
    message: "Cron Job Run Started",
    description: "Event label — cron_job_run_started",
  },
  "webhooks.event.cron_job_run_ended": {
    message: "Cron Job Run Ended",
    description: "Event label — cron_job_run_ended",
  },
  "webhooks.event.maintenance_mode_enabled": {
    message: "Maintenance Mode Enabled",
    description: "Event label — maintenance_mode_enabled",
  },
  "webhooks.event.maintenance_mode_uri_updated": {
    message: "Maintenance Mode URI Updated",
    description: "Event label — maintenance_mode_uri_updated",
  },
  "webhooks.event.postgres_created": {
    message: "Postgres Created",
    description: "Event label — postgres_created",
  },
  "webhooks.event.postgres_restarted": {
    message: "Postgres Restarted",
    description: "Event label — postgres_restarted",
  },
  "webhooks.event.postgres_credentials_created": {
    message: "Postgres Credentials Created",
    description: "Event label — postgres_credentials_created",
  },
  "webhooks.event.postgres_credentials_deleted": {
    message: "Postgres Credentials Deleted",
    description: "Event label — postgres_credentials_deleted",
  },
  "webhooks.event.postgres_backup_started": {
    message: "Postgres Backup Started",
    description: "Event label — postgres_backup_started",
  },
  "webhooks.event.service_suspended": {
    message: "Service Suspended",
    description: "Event label — service_suspended",
  },
  "webhooks.event.service_resumed": {
    message: "Service Resumed",
    description: "Event label — service_resumed",
  },
  "webhooks.event.server_restarted": {
    message: "Server Restarted",
    description: "Event label — server_restarted",
  },
  "webhooks.event.instance_count_changed": {
    message: "Instance Count Changed",
    description: "Event label — instance_count_changed",
  },
  "webhooks.event.autoscaling_config_changed": {
    message: "Autoscaling Config Changed",
    description: "Event label — autoscaling_config_changed",
  },
  "webhooks.event.plan_changed": {
    message: "Plan Changed",
    description:
      "Event label — plan_changed (bex's slot for Render's Instance Type Changed)",
  },
  // --- /webhooks/new create page (w1/m49/t003) ---
  "webhooks.newTitle": {
    message: "Create a new webhook",
    description: "Create page heading (Render: 'Create a new Webhook')",
  },
  "webhooks.backToList": {
    message: "Back to webhooks",
    description: "Accessible label of the create/detail pages' back link",
  },
  "webhooks.fieldNameHelp": {
    message: "A unique name for this webhook.",
    description: "Create page name-field helper copy",
  },
  "webhooks.fieldUrlHelp": {
    message: "bex sends each notification to this URL as a POST request.",
    description: "Create page URL-field helper copy",
  },
  "webhooks.fieldUrlPlaceholder": {
    message: "https://example.com/webhooks/bex",
    description: "Create page URL-field placeholder",
  },
  "webhooks.fieldEventsHelp": {
    message:
      "Choose which events in your workspace will trigger a webhook notification.",
    description: "Create page events-field helper copy",
  },
  "webhooks.createdView": {
    message: "View webhook",
    description: "Secret step button that opens the new webhook's page",
  },
  // --- /webhook/$id detail page (w1/m49/t004) ---
  "webhooks.detailKicker": {
    message: "Webhook",
    description: "Detail page header kicker above the endpoint name",
  },
  "webhooks.idLabel": {
    message: "Webhook ID:",
    description: "Detail header id row label",
  },
  "webhooks.copyId": {
    message: "Copy webhook ID",
    description: "Accessible label of the header id copy button",
  },
  "webhooks.copyUrl": {
    message: "Copy endpoint URL",
    description: "Accessible label of the header URL copy button",
  },
  "webhooks.copiedGeneric": {
    message: "Copied to clipboard",
    description: "Toast after copying the id or URL",
  },
  "webhooks.showMore": {
    message: "Show {count} more",
    description: "Event-chip expander on the detail header",
  },
  "webhooks.showLess": {
    message: "Show less",
    description: "Event-chip collapser on the detail header",
  },
  "webhooks.createdByOn": {
    message: "Created by {creator} on {date}",
    description: "Detail header provenance line when the creator is known",
  },
  "webhooks.createdOn": {
    message: "Created on {date}",
    description: "Detail header provenance line without a creator",
  },
  "webhooks.tabActivity": {
    message: "Activity",
    description: "Detail page tab — delivery history",
  },
  "webhooks.tabSettings": {
    message: "Settings",
    description: "Detail page tab — edit + delete",
  },
  "webhooks.recentDeliveries": {
    message: "Recent deliveries",
    description: "Activity tab heading",
  },
  "webhooks.recentDeliveriesHint": {
    message:
      "Each row is one delivery attempt. New attempts appear automatically while this tab is visible.",
    description: "Activity tab heading hint for immutable attempts + polling",
  },
  "webhooks.refresh": {
    message: "Refresh",
    description: "Accessible label of the Activity refresh button",
  },
  "webhooks.filterAll": {
    message: "All",
    description: "Activity delivery filter tab",
  },
  "webhooks.filterSuccessful": {
    message: "Successful",
    description: "Activity delivery filter tab — delivered only",
  },
  "webhooks.filterFailed": {
    message: "Failed",
    description: "Activity delivery filter tab — failed only",
  },
  "webhooks.sentAfter": {
    message: "Sent after",
    description: "Activity delivery-history lower timestamp bound",
  },
  "webhooks.sentBefore": {
    message: "Sent before",
    description: "Activity delivery-history upper timestamp bound",
  },
  "webhooks.transportError": {
    message: "Transport error",
    description: "Delivery evidence label when no HTTP response was received",
  },
  "webhooks.enabledBadge": {
    message: "Enabled",
    description: "Detail header status badge while enabled",
  },
  "webhooks.disabledBadge": {
    message: "Disabled",
    description: "Detail header status badge while disabled",
  },
  "webhooks.view": {
    message: "View webhook",
    description: "Accessible label of the list row's open-detail link",
  },
  // --- Settings tab (w1/m49/t005 + t006) ---
  "webhooks.settingsGeneral": {
    message: "General",
    description: "Settings tab first section heading",
  },
  "webhooks.settingsStatus": {
    message: "Status",
    description: "Settings status-toggle label",
  },
  "webhooks.settingsStatusHelp": {
    message: "Webhooks do not send any notifications while disabled.",
    description: "Settings status-toggle helper copy",
  },
  "webhooks.settingsEvents": {
    message: "Subscribed events",
    description: "Settings events section heading",
  },
  "webhooks.saveChanges": {
    message: "Save changes",
    description: "Settings submit button (disabled until dirty)",
  },
  "webhooks.updateSuccess": {
    message: "Webhook updated",
    description: "Toast after a successful settings save",
  },
  "webhooks.updateError": {
    message: "Couldn't update the webhook",
    description: "Toast after a failed settings save",
  },
  "webhooks.secretLabel": {
    message: "Signing secret",
    description: "Settings signing-secret row label",
  },
  "webhooks.secretMintOnceNote": {
    message:
      "The signing secret was shown once when this webhook was created and can't be retrieved again. Delete and recreate the webhook to mint a new one.",
    description:
      "Settings note documenting bex's mint-once secret contract (w1/m49/t006 decision — deliberate Render divergence)",
  },
  "webhooks.deleteSection": {
    message: "Delete webhook",
    description: "Settings danger-zone heading",
  },
  "webhooks.deleteTypeToConfirm": {
    message: "Type {command} below to confirm.",
    description: "Type-to-confirm instruction in the delete dialog",
  },
  "webhooks.deleteCommandLabel": {
    message: "Confirmation",
    description: "Accessible label of the type-to-confirm input",
  },
};

export default enWebhooks;
