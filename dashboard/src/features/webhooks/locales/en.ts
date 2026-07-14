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
    message: "Create",
    description: "Create dialog submit button",
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
    message: "Trigger a subscribed event — a deploy, for example — to see it here.",
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
    message: "Attempts",
    description: "Delivery-history table column header",
  },
  "webhooks.colResponse": {
    message: "Response",
    description: "Delivery-history table column header — last HTTP status",
  },
  "webhooks.colWhen": {
    message: "When",
    description: "Delivery-history table column header — event age",
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
    description: "Delivery status badge — retries exhausted",
  },
  "webhooks.loadMore": {
    message: "Load more",
    description: "Delivery-history pagination button",
  },
};

export default enWebhooks;
