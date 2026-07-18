import type { TranslationEntry } from "@/i18n";

const enKeyValue: Record<string, TranslationEntry> = {
  // --- List page stat tiles ---
  "keyvalue.statTotal": {
    message: "Total Key Value stores",
    description: "Key Value page stat card label",
  },
  "keyvalue.statAvailable": {
    message: "Available",
    description: "Key Value page stat card label (healthy stores)",
  },
  "keyvalue.statCreating": {
    message: "Creating",
    description: "Key Value page stat card label (provisioning stores)",
  },
  // --- List table ---
  "keyvalue.cardTitle": {
    message: "Key Value",
    description: "Key Value table card title",
  },
  "keyvalue.colName": {
    message: "Name",
    description: "Key Value table column header",
  },
  "keyvalue.colStatus": {
    message: "Status",
    description: "Key Value table column header",
  },
  "keyvalue.colPlan": {
    message: "Instance type",
    description: "Key Value table column header (plan / tier)",
  },
  "keyvalue.colVersion": {
    message: "Version",
    description: "Key Value table column header (Valkey version)",
  },
  "keyvalue.colCreated": {
    message: "Created",
    description: "Key Value table column header (relative age from createdAt)",
  },
  "keyvalue.colActions": {
    message: "Actions",
    description: "Key Value table actions column header (screen-reader only)",
  },
  // --- Status badges ---
  "keyvalue.statusAvailable": {
    message: "Available",
    description: "Key Value status badge (Valkey instance healthy)",
  },
  "keyvalue.statusCreating": {
    message: "Creating",
    description: "Key Value status badge (provisioning)",
  },
  "keyvalue.statusUnavailable": {
    message: "Unavailable",
    description: "Key Value status badge (provisioning failed)",
  },
  "keyvalue.statusSuspended": {
    message: "Suspended",
    description:
      "Key Value status badge (hibernated; suspension wins over the status enum)",
  },
  "keyvalue.statusUnknown": {
    message: "Unknown",
    description: "Key Value status badge for an unrecognized status",
  },
  // --- List states ---
  "keyvalue.errorTitle": {
    message: "Couldn't load Key Value stores",
    description: "Key Value list error card title",
  },
  "keyvalue.errorBody": {
    message: "The request to bex-api failed. Check your connection and retry.",
    description: "Key Value list error card body",
  },
  "keyvalue.emptyTitle": {
    message: "No Key Value stores yet",
    description: "Key Value list empty state title",
  },
  "keyvalue.emptyBody": {
    message:
      "Create your first managed Key Value store and it'll show up here.",
    description: "Key Value list empty state body",
  },
  // --- Row actions / delete ---
  "keyvalue.actionsMenu": {
    message: "Open actions menu",
    description: "Accessible label for the per-row actions trigger",
  },
  "keyvalue.actionDelete": {
    message: "Delete",
    description: "Row action: permanently delete the Key Value store",
  },
  "keyvalue.deleteConfirmTitle": {
    message: "Delete Key Value Instance",
    description: "Delete-confirmation dialog title",
  },
  "keyvalue.deleteConfirmBody": {
    message:
      "This permanently deletes the Key Value store and all its data — the Valkey instance, its storage, and its connection credentials. This cannot be undone.",
    description: "Delete-confirmation dialog body",
  },
  "keyvalue.deleteConfirmPrompt": {
    message: "Type {phrase} below to confirm",
    description: "Render-style exact sudo delete confirmation prompt",
  },
  "keyvalue.deleteCancel": {
    message: "Cancel",
    description: "Delete-confirmation dialog cancel button",
  },
  "keyvalue.deleteConfirm": {
    message: "Delete Key Value Instance",
    description: "Delete-confirmation dialog confirm button",
  },
  "keyvalue.deleteSuccess": {
    message: "Deleting {name}…",
    description: "Toast after a delete request is accepted",
  },
  "keyvalue.deleteError": {
    message: "Couldn't delete {name}. Please try again.",
    description: "Toast when a delete request fails",
  },
  // --- Create form ---
  "keyvalue.createButton": {
    message: "New Key Value",
    description: "Button that navigates to the create-Key-Value page",
  },
  "keyvalue.createTitle": {
    message: "Create a Key Value store",
    description: "Create-Key-Value page title",
  },
  "keyvalue.createDescription": {
    message: "Provision a managed Valkey (Redis-compatible) instance.",
    description: "Create-Key-Value page description",
  },
  "keyvalue.fieldName": {
    message: "Name",
    description: "Create-Key-Value form field label (store name)",
  },
  "keyvalue.fieldNamePlaceholder": {
    message: "example-key-value-name",
    description: "Create-Key-Value name input placeholder",
  },
  "keyvalue.fieldNameError": {
    message:
      "Use lowercase letters, digits, and hyphens (up to 30 characters); can't start or end with a hyphen.",
    description: "Create-Key-Value name validation message",
  },
  "keyvalue.fieldPlan": {
    message: "Instance type",
    description: "Create-Key-Value form field label (plan / tier)",
  },
  "keyvalue.fieldVersion": {
    message: "Valkey version",
    description: "Create-Key-Value form field label (major version)",
  },
  "keyvalue.fieldVersionDefault": {
    message: "Default (latest)",
    description: "Create-Key-Value version select default option",
  },
  "keyvalue.fieldMaxmemoryPolicy": {
    message: "Maxmemory Policy",
    description: "Create-Key-Value form field label (eviction policy)",
  },
  "keyvalue.fieldMaxmemoryRecommended": {
    message: "(recommended for caches)",
    description:
      "Suffix on the recommended maxmemory policy option (allkeys-lru)",
  },
  "keyvalue.fieldMaxmemoryPolicyHint": {
    message: "How keys are evicted once the store reaches its memory limit.",
    description: "Create-Key-Value maxmemory policy helper text",
  },
  "keyvalue.fieldPersistenceMode": {
    message: "Persistence Mode",
    description: "Create-Key-Value form field label (persistence)",
  },
  "keyvalue.fieldPersistenceModeHint": {
    message: "How data is persisted to disk so it survives a restart.",
    description: "Create-Key-Value persistence mode helper text",
  },
  "keyvalue.fieldPersistenceFreeHint": {
    message: "The Free plan has no persistent disk, so persistence is off.",
    description:
      "Create-Key-Value persistence helper text when Free plan is selected",
  },
  "keyvalue.persistenceJournalSnapshot": {
    message: "Journal + Snapshot",
    description: "Persistence mode option: AOF journal plus RDB snapshots",
  },
  "keyvalue.persistenceSnapshot": {
    message: "Snapshot only",
    description: "Persistence mode option: RDB snapshots only",
  },
  "keyvalue.persistenceOff": {
    message: "Off",
    description: "Persistence mode option: no persistence",
  },
  "keyvalue.fieldPublic": {
    message: "Public access",
    description: "Create-Key-Value form field label (external endpoint toggle)",
  },
  "keyvalue.fieldPublicHint": {
    message: "Allow connections from outside the cluster over TLS.",
    description: "Create-Key-Value public toggle helper text",
  },
  "keyvalue.createCancel": {
    message: "Cancel",
    description: "Create-Key-Value page cancel button",
  },
  "keyvalue.createSubmit": {
    message: "Create Key Value Instance",
    description: "Create-Key-Value page submit button",
  },
  "keyvalue.createSuccess": {
    message: "Creating {name}…",
    description:
      "Toast after a create request is accepted (provisioning is async)",
  },
  "keyvalue.createError": {
    message: "Couldn't create {name}. Please try again.",
    description: "Toast when a create request fails",
  },
  "keyvalue.capLimitTitle": {
    message: "Key-value limit reached",
    description:
      "Alert title when the workspace's KV creation cap is hit (w7/m9)",
  },
  "keyvalue.capLimitUpgrade": {
    message: "Upgrade plan",
    description: "Upgrade CTA button inside the KV cap-limit Alert (w7/m9)",
  },
  // --- Detail metadata ---
  "keyvalue.metaTitle": {
    message: "Details",
    description: "Key Value detail metadata card title",
  },
  "keyvalue.metaId": {
    message: "ID",
    description: "Key Value detail metadata row label (immutable red- id)",
  },
  "keyvalue.metaStatus": {
    message: "Status",
    description: "Key Value detail metadata row label",
  },
  "keyvalue.metaPlan": {
    message: "Instance type",
    description: "Key Value detail metadata row label",
  },
  "keyvalue.metaVersion": {
    message: "Version",
    description: "Key Value detail metadata row label",
  },
  "keyvalue.metaPublic": {
    message: "Public access",
    description: "Key Value detail metadata row label (external endpoint)",
  },
  "keyvalue.metaExternalHost": {
    message: "External host",
    description: "Key Value detail metadata row label (SNI hostname)",
  },
  "keyvalue.metaRegion": {
    message: "Region",
    description: "Key Value detail metadata row label (platform placement)",
  },
  "keyvalue.metaCreated": {
    message: "Created",
    description: "Key Value detail metadata row label (relative age)",
  },
  "keyvalue.yes": {
    message: "Yes",
    description: "Metadata value for a true boolean field",
  },
  "keyvalue.no": {
    message: "No",
    description: "Metadata value for a false boolean field",
  },
  "keyvalue.notFoundTitle": {
    message: "Key Value store not found",
    description: "Detail page state when keyValue(id) returns nothing",
  },
  "keyvalue.notFoundBody": {
    message:
      "No Key Value store named {name} exists, or you don't have access to it.",
    description: "Detail page not-found body",
  },
  // --- Connection info panel ---
  "keyvalue.connTitle": {
    message: "Connections",
    description: "Connection-info panel card title",
  },
  "keyvalue.connDescription": {
    message:
      "Connection strings for this Key Value store. Revealed only when you ask — never shown automatically.",
    description: "Connection-info panel card description",
  },
  "keyvalue.connReveal": {
    message: "Reveal connection info",
    description: "Button that fetches the connection info on demand",
  },
  "keyvalue.connHide": {
    message: "Hide connection info",
    description: "Button that clears the revealed connection info",
  },
  "keyvalue.connInternal": {
    message: "Internal Key Value URL",
    description: "Connection-info field label (in-cluster redis:// URL)",
  },
  "keyvalue.connExternal": {
    message: "External Key Value URL",
    description: "Connection-info field label (public rediss:// URL)",
  },
  "keyvalue.connExternalUnavailable": {
    message: "Not public. Enable public access to get an external URL.",
    description:
      "Shown instead of the external URL when the store isn't public",
  },
  "keyvalue.connCli": {
    message: "Valkey CLI command",
    description: "Connection-info field label (ready-to-run redis-cli command)",
  },
  "keyvalue.connErrorTitle": {
    message: "Couldn't load connection info",
    description: "Connection-info panel error title",
  },
  "keyvalue.connErrorBody": {
    message:
      "The store may still be provisioning, or you may not have permission to view its credentials.",
    description: "Connection-info panel error body",
  },
  "keyvalue.copied": {
    message: "Copied to clipboard",
    description: "Toast after copying a connection field",
  },
  "keyvalue.copyError": {
    message: "Couldn't copy to clipboard",
    description: "Toast when clipboard copy fails",
  },
  // --- Suspend / resume ---
  "keyvalue.dangerDelete": {
    message: "Delete Key Value Instance",
    description: "Discoverable delete action at the bottom of Key Value Info",
  },
  "keyvalue.actionSuspend": {
    message: "Suspend Key Value Instance",
    description: "Suspend action button label",
  },
  "keyvalue.actionResume": {
    message: "Resume Key Value Instance",
    description: "Resume action button label",
  },
  "keyvalue.confirmSuspendTitle": {
    message: "Suspend Key Value Instance",
    description: "Suspend-confirmation dialog title",
  },
  "keyvalue.confirmSuspendBody": {
    message: "Are you sure you want to suspend this Key Value instance?",
    description: "Suspend-confirmation dialog description",
  },
  "keyvalue.confirmSuspendDetail": {
    message:
      "Please confirm you would like to suspend {name}. Active connections will be dropped; data is preserved and you can resume it at any time.",
    description: "Suspend-confirmation consequences and named instance",
  },
  "keyvalue.confirmSuspendPrompt": {
    message: "Type {phrase} below to confirm",
    description:
      "Label for Render's exact Key Value suspend confirmation phrase",
  },
  "keyvalue.confirmCancel": {
    message: "Cancel",
    description: "Suspend-confirmation dialog cancel button",
  },
  "keyvalue.toastSuspendSuccess": {
    message: "Suspending {name}…",
    description: "Toast after a suspend request is accepted",
  },
  "keyvalue.toastSuspendError": {
    message: "Couldn't suspend {name}. Please try again.",
    description: "Toast when a suspend request fails",
  },
  "keyvalue.toastResumeSuccess": {
    message: "Resuming {name}…",
    description: "Toast after a resume request is accepted",
  },
  "keyvalue.toastResumeError": {
    message: "Couldn't resume {name}. Please try again.",
    description: "Toast when a resume request fails",
  },
  // --- Plan section (m16) ---
  "keyvalue.planTitle": {
    message: "Instance type",
    description: "Key Value detail plan-picker card title",
  },
  "keyvalue.planDescription": {
    message:
      "Change the instance type. The operator resizes resources on the next reconcile — a rolling update that preserves data.",
    description: "Key Value detail plan-picker card description",
  },
  "keyvalue.planPickerSave": {
    message: "Save",
    description: "Key Value plan-picker save button",
  },
  "keyvalue.planPickerCancel": {
    message: "Cancel",
    description: "Key Value plan-picker cancel / reset button",
  },
  "keyvalue.planPickerConfirmTitle": {
    message: "Change to {name}?",
    description: "Key Value plan-picker confirm dialog title",
  },
  "keyvalue.planPickerConfirmBody": {
    message:
      "The operator will resize the store's compute resources on the next reconcile. Connections drop briefly during the rolling restart.",
    description: "Key Value plan-picker confirm dialog body",
  },
  "keyvalue.planPickerSuccess": {
    message: "Updating plan to {name}…",
    description: "Toast after a plan update is accepted",
  },
  "keyvalue.planPickerError": {
    message: "Couldn't update the plan. Please try again.",
    description: "Toast when a plan update fails",
  },
  // --- Networking (external-endpoint IP allowlist) ---
  "keyvalue.networkingTitle": {
    message: "Networking",
    description: "Detail-page Networking card title (IP allowlist)",
  },
  "keyvalue.networkingDescription": {
    message: "Restrict which source IPs can reach the external endpoint.",
    description: "Networking card subtitle",
  },
  "keyvalue.networkingHint": {
    message:
      "Only these CIDR ranges can reach the external endpoint. Empty means open to all source IPs. The internal endpoint is never affected.",
    description: "Networking IP allowlist helper text",
  },
  "keyvalue.networkingInternalOnly": {
    message:
      "This store has no external endpoint; the allowlist takes effect once the store is public.",
    description: "Networking note shown for an internal-only store",
  },
  "keyvalue.networkingOpen": {
    message: "Open to all source IPs.",
    description: "Networking panel shown when the allowlist is empty",
  },
  "keyvalue.networkingAdd": {
    message: "Add",
    description: "Networking button to add a CIDR to the draft allowlist",
  },
  "keyvalue.networkingEntryDescription": {
    message: "Description (optional)",
    description:
      "Placeholder for the optional per-entry allowlist description input",
  },
  "keyvalue.networkingRemove": {
    message: "Remove {cidr}",
    description: "Accessible label to remove a CIDR chip",
  },
  "keyvalue.networkingSave": {
    message: "Save allowlist",
    description: "Networking button to persist the allowlist",
  },
  "keyvalue.networkingSaved": {
    message: "Allowlist updated.",
    description: "Toast after saving the IP allowlist",
  },
  "keyvalue.networkingError": {
    message: "Couldn't save the allowlist: {error}",
    description: "Toast when saving the allowlist fails",
  },
  // --- Name section (rename control) ---
  "keyvalue.nameTitle": {
    message: "Key Value name",
    description: "Key Value detail rename card title",
  },
  "keyvalue.nameDescription": {
    message:
      "Change the display name. The Key Value ID and all connection details stay the same.",
    description: "Key Value detail rename card description",
  },
  "keyvalue.nameSave": {
    message: "Save name",
    description: "Key Value rename save button",
  },
  "keyvalue.nameSuccess": {
    message: "Renamed Key Value store to {name}.",
    description: "Toast after a Key Value rename succeeds",
  },
  "keyvalue.nameError": {
    message: "Couldn't rename the Key Value store. Please try again.",
    description: "Toast when a Key Value rename fails unexpectedly",
  },
  "keyvalue.nameConflict": {
    message:
      "A Key Value store with that name already exists in this workspace.",
    description: "Toast when a Key Value rename collides with another name",
  },
  "keyvalue.nameInvalid": {
    message:
      "Use lowercase letters, digits, and hyphens (up to 30 characters); don't start or end with a hyphen.",
    description: "Key Value rename validation message",
  },
  // --- Tab navigation ---
  "keyvalue.detailNavLabel": {
    message: "Key Value detail navigation",
    description: "aria-label for the detail-page tab nav",
  },
  "keyvalue.overviewTab": {
    message: "Overview",
    description: "Detail-page Overview tab label",
  },
  "keyvalue.logsTab": {
    message: "Logs",
    description: "Detail-page Logs tab label",
  },
  // --- Logs viewer ---
  "keyvalue.logsRangeLabel": {
    message: "Time range",
    description: "Accessible label for the log time-range select",
  },
  "keyvalue.logsRange1h": {
    message: "Last 1 hour",
    description: "Log time range option",
  },
  "keyvalue.logsRange6h": {
    message: "Last 6 hours",
    description: "Log time range option",
  },
  "keyvalue.logsRange24h": {
    message: "Last 24 hours",
    description: "Log time range option",
  },
  "keyvalue.logsInstanceLabel": {
    message: "Instance",
    description: "Accessible label for the log instance (pod) select",
  },
  "keyvalue.logsAllInstances": {
    message: "All instances",
    description: "Default option in the instance filter (no filter applied)",
  },
  "keyvalue.logsSearchPlaceholder": {
    message: "Search logs…",
    description: "Placeholder for the log text-search input",
  },
  "keyvalue.logsLoading": {
    message: "Loading logs…",
    description: "Loading state message while fetching logs",
  },
  "keyvalue.logsEmptyTitle": {
    message: "No log lines",
    description: "Empty state title when no log lines are returned",
  },
  "keyvalue.logsEmptyBody": {
    message:
      "No log lines in this time range. Valkey logs key-space events, slow queries, and startup messages.",
    description: "Empty state body when no log lines and no filters active",
  },
  "keyvalue.logsEmptyFilteredBody": {
    message: "No log lines match the active filters.",
    description: "Empty state body when filters are active and nothing matches",
  },
  "keyvalue.logsUnavailableTitle": {
    message: "Logs not available",
    description: "Empty state title when the logs source is not configured",
  },
  "keyvalue.logsUnavailableBody": {
    message:
      "The platform's log source is not configured. Contact your platform admin to enable BEX_LOKI_URL.",
    description: "Empty state body when BEX_LOKI_URL is not set",
  },
  "keyvalue.logsUnauthorizedTitle": {
    message: "Access denied",
    description: "Empty state title when the caller lacks can_view_logs",
  },
  "keyvalue.logsUnauthorizedBody": {
    message: "You don't have permission to view logs for this Key Value store.",
    description: "Empty state body for a 403 on the logs query",
  },
  "keyvalue.logsErrorTitle": {
    message: "Couldn't load logs",
    description: "Empty state title for an unexpected logs fetch error",
  },
};

export default enKeyValue;
