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
};

export default enAudit;
