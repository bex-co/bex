import type { TranslationEntry } from "@/i18n";

const enAudit: Record<string, TranslationEntry> = {
  "audit.title": {
    message: "Audit Log",
    description: "Settings Audit Log section card title",
  },
  "audit.description": {
    message:
      "Who did what in this workspace, allowed or denied — newest first. Visible to workspace admins only.",
    description: "Settings Audit Log section card description",
  },
  "audit.columnTimestamp": {
    message: "Timestamp",
    description: "Audit Log table column header",
  },
  "audit.columnActor": {
    message: "Actor",
    description: "Audit Log table column header — who performed the action",
  },
  "audit.columnAction": {
    message: "Action",
    description: "Audit Log table column header — the verb performed",
  },
  "audit.columnStatus": {
    message: "Status",
    description: "Audit Log table column header — allowed or denied",
  },
  "audit.columnResource": {
    message: "Resource",
    description: "Audit Log table column header — the object acted on",
  },
  "audit.changeRename": {
    message: "Name {from} → {to}",
    description:
      "Audit log Resource cell: a service rename's label before and after (w5/183)",
  },
  "audit.changeDomain": {
    message: "Domain {domain}",
    description:
      "Audit log Resource cell: the custom domain a domain verb added, removed or verified (w5/183)",
  },
  "audit.changeEnvGroup": {
    message: "Environment group {name}",
    description:
      "Audit log Resource cell: the environment group a link or unlink changed (w5/183)",
  },
  "audit.statusAllowed": {
    message: "Allowed",
    description: "Audit Log status badge for a successful action",
  },
  "audit.statusDenied": {
    message: "Denied",
    description: "Audit Log status badge for a denied action",
  },
  "audit.actorUnknown": {
    message: "Unknown",
    description:
      "Audit Log actor cell placeholder for an unauthenticated caller",
  },
  "audit.oauthDelegation": {
    message: "{client} · {scopes}",
    description:
      "Audit Log actor subtitle for a third-party OAuth grant (client id and canonical scopes)",
  },
  "audit.loadMore": {
    message: "Load more",
    description: "Button that fetches the next page of audit events",
  },
  "audit.emptyTitle": {
    message: "No audit events yet",
    description: "Audit Log empty-state title",
  },
  "audit.emptyBody": {
    message: "Write actions in this workspace will show up here.",
    description: "Audit Log empty-state body",
  },
  "audit.errorTitle": {
    message: "Couldn't load the audit log",
    description: "Audit Log generic error title",
  },
  "audit.errorBody": {
    message:
      "Something went wrong loading this workspace's audit trail. Try again.",
    description: "Audit Log generic error body",
  },
  "audit.unavailableTitle": {
    message: "Audit log not configured",
    description:
      "Audit Log state when the control-plane store isn't wired (503)",
  },
  "audit.unavailableBody": {
    message: "This bex deployment doesn't have an audit-log store configured.",
    description: "Audit Log unavailable-state body",
  },
  "audit.planTitle": {
    message: "Audit logs require the Pro plan",
    description:
      "Audit Log title on a Hobby workspace (PLAN_FEATURE_REQUIRED, w8/m138)",
  },
  "audit.planBody": {
    message:
      "Audit logs are available on the Pro plan and higher. This workspace is on Hobby.",
    description: "Audit Log plan notice body on a Hobby workspace (w8/m138)",
  },
  "audit.exportStart": {
    message: "Start date (UTC)",
    description: "Audit Log CSV export start-date label",
  },
  "audit.exportEnd": {
    message: "End date (UTC)",
    description: "Audit Log CSV export end-date label",
  },
  "audit.exportCSV": {
    message: "Export as CSV",
    description: "Audit Log CSV export button (Render's label)",
  },
  "audit.exportHint": {
    message:
      "Exports up to {days} days of events: timestamp, actor, event, status and metadata.",
    description: "Audit Log CSV export help text; {days} is the longest range",
  },
  "audit.exportRangeOrder": {
    message: "The end date must be on or after the start date.",
    description: "Audit Log CSV export: end before start",
  },
  "audit.exportRangeTooLong": {
    message: "An export can span at most {days} days.",
    description:
      "Audit Log CSV export: range over the limit; {days} is the limit",
  },
  "audit.exportTooLarge": {
    message:
      "This range has too many events to export at once. Choose a shorter range.",
    description: "Audit Log CSV export: row budget exceeded",
  },
  "audit.exportFailed": {
    message: "The export failed. Try again.",
    description: "Audit Log CSV export: request failed",
  },
  "audit.exportRangeMissing": {
    message: "Choose a start and an end date.",
    description: "Audit Log CSV export: a date is empty",
  },
};

export default enAudit;
