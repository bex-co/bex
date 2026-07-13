import type { TranslationEntry } from "@/i18n";

const enDatabases: Record<string, TranslationEntry> = {
  // --- List page stat tiles ---
  "databases.statTotal": {
    message: "Total databases",
    description: "Databases page stat card label",
  },
  "databases.statAvailable": {
    message: "Available",
    description: "Databases page stat card label (healthy databases)",
  },
  "databases.statCreating": {
    message: "Creating",
    description: "Databases page stat card label (provisioning databases)",
  },
  // --- List table ---
  "databases.cardTitle": {
    message: "Databases",
    description: "Databases table card title",
  },
  "databases.colName": {
    message: "Name",
    description: "Databases table column header",
  },
  "databases.colStatus": {
    message: "Status",
    description: "Databases table column header",
  },
  "databases.colPlan": {
    message: "Plan",
    description: "Databases table column header (instance type / tier)",
  },
  "databases.colVersion": {
    message: "Version",
    description: "Databases table column header (PostgreSQL major version)",
  },
  "databases.colStorage": {
    message: "Storage",
    description: "Databases table column header (disk size)",
  },
  "databases.colCreated": {
    message: "Created",
    description: "Databases table column header (relative age from createdAt)",
  },
  "databases.colActions": {
    message: "Actions",
    description: "Databases table actions column header (screen-reader only)",
  },
  // --- Status badges ---
  "databases.statusAvailable": {
    message: "Available",
    description: "Database status badge (CNPG cluster healthy)",
  },
  "databases.statusCreating": {
    message: "Creating",
    description: "Database status badge (provisioning)",
  },
  "databases.statusUnavailable": {
    message: "Unavailable",
    description: "Database status badge (provisioning failed)",
  },
  "databases.statusUnknown": {
    message: "Unknown",
    description: "Database status badge for an unrecognized status",
  },
  // --- List states ---
  "databases.errorTitle": {
    message: "Couldn't load databases",
    description: "Databases list error card title",
  },
  "databases.errorBody": {
    message: "The request to bex-api failed. Check your connection and retry.",
    description: "Databases list error card body",
  },
  "databases.emptyTitle": {
    message: "No databases yet",
    description: "Databases list empty state title",
  },
  "databases.emptyBody": {
    message: "Create your first managed Postgres and it'll show up here.",
    description: "Databases list empty state body",
  },
  // --- Row actions / delete ---
  "databases.actionsMenu": {
    message: "Open actions menu",
    description: "Accessible label for the per-row actions trigger",
  },
  "databases.actionDelete": {
    message: "Delete",
    description: "Row action: permanently delete the database",
  },
  "databases.deleteConfirmTitle": {
    message: "Delete {name}?",
    description: "Delete-confirmation dialog title",
  },
  "databases.deleteConfirmBody": {
    message:
      "This permanently deletes the database and all its data — the Postgres cluster, its storage, and its connection credentials. This cannot be undone.",
    description: "Delete-confirmation dialog body",
  },
  "databases.deleteConfirmPrompt": {
    message: "Type {name} to confirm.",
    description: "Delete-confirmation typed-name prompt label",
  },
  "databases.deleteCancel": {
    message: "Cancel",
    description: "Delete-confirmation dialog cancel button",
  },
  "databases.deleteConfirm": {
    message: "Delete database",
    description: "Delete-confirmation dialog confirm button",
  },
  "databases.deleteSuccess": {
    message: "Deleting {name}…",
    description: "Toast after a delete request is accepted",
  },
  "databases.deleteError": {
    message: "Couldn't delete {name}. Please try again.",
    description: "Toast when a delete request fails",
  },
  // --- Create dialog ---
  "databases.createButton": {
    message: "New Database",
    description: "Button that opens the create-database dialog",
  },
  "databases.createTitle": {
    message: "Create a Postgres database",
    description: "Create-database dialog title",
  },
  "databases.createDescription": {
    message: "Provision a managed PostgreSQL instance.",
    description: "Create-database dialog description",
  },
  "databases.fieldName": {
    message: "Name",
    description: "Create-database form field label (database name)",
  },
  "databases.fieldNamePlaceholder": {
    message: "my-database",
    description: "Create-database name input placeholder",
  },
  "databases.fieldNameError": {
    message:
      "Use lowercase letters, digits and hyphens; must start with a letter.",
    description: "Create-database name validation message",
  },
  "databases.fieldPlan": {
    message: "Instance type",
    description: "Create-database form field label (plan / tier)",
  },
  "databases.fieldPlanPlaceholder": {
    message: "Select an instance type",
    description: "Create-database plan select placeholder",
  },
  "databases.fieldVersion": {
    message: "PostgreSQL version",
    description: "Create-database form field label (major version)",
  },
  "databases.fieldVersionDefault": {
    message: "Default (latest)",
    description: "Create-database version select default option",
  },
  "databases.fieldDisk": {
    message: "Disk size (GB)",
    description: "Create-database form field label (storage size)",
  },
  "databases.fieldPublic": {
    message: "Public access",
    description: "Create-database form field label (external endpoint toggle)",
  },
  "databases.fieldPublicHint": {
    message: "Allow connections from outside the cluster over TLS.",
    description: "Create-database public toggle helper text",
  },
  "databases.createCancel": {
    message: "Cancel",
    description: "Create-database dialog cancel button",
  },
  "databases.createSubmit": {
    message: "Create database",
    description: "Create-database dialog submit button",
  },
  "databases.createSuccess": {
    message: "Creating {name}…",
    description:
      "Toast after a create request is accepted (provisioning is async)",
  },
  "databases.createError": {
    message: "Couldn't create {name}. Please try again.",
    description: "Toast when a create request fails",
  },
  // --- Detail metadata ---
  "databases.metaTitle": {
    message: "Details",
    description: "Database detail metadata card title",
  },
  "databases.metaStatus": {
    message: "Status",
    description: "Database detail metadata row label",
  },
  "databases.metaPlan": {
    message: "Instance type",
    description: "Database detail metadata row label",
  },
  "databases.metaVersion": {
    message: "Version",
    description: "Database detail metadata row label",
  },
  "databases.metaDatabaseName": {
    message: "Database",
    description: "Database detail metadata row label (the normalized db name)",
  },
  "databases.metaDatabaseUser": {
    message: "User",
    description: "Database detail metadata row label (owner role)",
  },
  "databases.metaStorage": {
    message: "Storage",
    description: "Database detail metadata row label (disk size)",
  },
  "databases.metaHighAvailability": {
    message: "High availability",
    description:
      "Database detail metadata row label (HA — single-instance MVP is No)",
  },
  "databases.metaPublic": {
    message: "Public access",
    description: "Database detail metadata row label (external endpoint)",
  },
  "databases.metaExternalHost": {
    message: "External host",
    description: "Database detail metadata row label (SNI hostname)",
  },
  "databases.metaCreated": {
    message: "Created",
    description: "Database detail metadata row label (relative age)",
  },
  "databases.yes": {
    message: "Yes",
    description: "Metadata value for a true boolean field",
  },
  "databases.no": {
    message: "No",
    description: "Metadata value for a false boolean field",
  },
  "databases.notFoundTitle": {
    message: "Database not found",
    description: "Detail page state when database(id) returns nothing",
  },
  "databases.notFoundBody": {
    message: "No database named {name} exists, or you don't have access to it.",
    description: "Detail page not-found body",
  },
  // --- Connection info panel ---
  "databases.connTitle": {
    message: "Connections",
    description: "Connection-info panel card title",
  },
  "databases.connDescription": {
    message:
      "Connection strings and the database password. Revealed only when you ask — never shown automatically.",
    description: "Connection-info panel card description",
  },
  "databases.connReveal": {
    message: "Reveal connection info",
    description: "Button that fetches the connection info on demand",
  },
  "databases.connHide": {
    message: "Hide connection info",
    description: "Button that clears the revealed connection info",
  },
  "databases.connPassword": {
    message: "Password",
    description: "Connection-info field label (database password)",
  },
  "databases.connShowPassword": {
    message: "Show password",
    description: "Accessible label to unmask the password",
  },
  "databases.connHidePassword": {
    message: "Hide password",
    description: "Accessible label to re-mask the password",
  },
  "databases.connInternal": {
    message: "Internal connection string",
    description: "Connection-info field label (in-cluster URL)",
  },
  "databases.connExternal": {
    message: "External connection string",
    description: "Connection-info field label (public URL, TLS)",
  },
  "databases.connPsql": {
    message: "psql command",
    description: "Connection-info field label (ready-to-run psql command)",
  },
  "databases.connErrorTitle": {
    message: "Couldn't load connection info",
    description: "Connection-info panel error title",
  },
  "databases.connErrorBody": {
    message:
      "The database may still be provisioning, or you may not have permission to view its credentials.",
    description: "Connection-info panel error body",
  },
  "databases.copied": {
    message: "Copied to clipboard",
    description: "Toast after copying a connection field",
  },
  "databases.copyError": {
    message: "Couldn't copy to clipboard",
    description: "Toast when clipboard copy fails",
  },
  "databases.loading": {
    message: "Loading…",
    description: "Generic loading placeholder in a detail panel",
  },
  // --- Lifecycle actions (suspend / resume / restart) ---
  "databases.actionSuspend": {
    message: "Suspend",
    description: "Row action: hibernate the database (stop compute, keep data)",
  },
  "databases.actionResume": {
    message: "Resume",
    description: "Row action: wake a suspended database",
  },
  "databases.actionRestart": {
    message: "Restart",
    description: "Row action: rolling restart of the primary",
  },
  "databases.suspendConfirmTitle": {
    message: "Suspend {name}?",
    description: "Suspend-confirmation dialog title",
  },
  "databases.suspendConfirmBody": {
    message:
      "Compute stops and the database becomes unreachable, but its data is preserved. Resume it any time to bring it back.",
    description: "Suspend-confirmation dialog body",
  },
  "databases.restartConfirmTitle": {
    message: "Restart {name}?",
    description: "Restart-confirmation dialog title",
  },
  "databases.restartConfirmBody": {
    message:
      "The primary is restarted. Connections drop briefly while it comes back.",
    description: "Restart-confirmation dialog body",
  },
  "databases.toastSuspendSuccess": {
    message: "Suspending {name}…",
    description: "Toast after a suspend request is accepted",
  },
  "databases.toastResumeSuccess": {
    message: "Resuming {name}…",
    description: "Toast after a resume request is accepted",
  },
  "databases.toastRestartSuccess": {
    message: "Restarting {name}…",
    description: "Toast after a restart request is accepted",
  },
  "databases.toastLifecycleError": {
    message: "Couldn't complete the action on {name}. Please try again.",
    description: "Toast when a lifecycle verb fails",
  },
  // --- Recovery panel ---
  "databases.recoveryTitle": {
    message: "Recovery",
    description: "Recovery panel card title",
  },
  "databases.recoveryDescription": {
    message:
      "Point-in-time recovery and backups. Restore always creates a new database — this one is never modified.",
    description: "Recovery panel card description",
  },
  "databases.recoveryDisabled": {
    message:
      "Backups aren't enabled for this plan, so recovery isn't available. Upgrade to a plan with backups to enable point-in-time recovery.",
    description: "Recovery panel state when the plan has no backups",
  },
  "databases.recoveryEarliest": {
    message: "Earliest restore point",
    description: "Recovery panel field (earliest recoverable time)",
  },
  "databases.recoveryLatest": {
    message: "Latest restore point",
    description: "Recovery panel field (latest recoverable time)",
  },
  "databases.recoveryNoBackupYet": {
    message: "No backup yet",
    description: "Recovery panel value when the first backup hasn't landed",
  },
  "databases.recoveryRestore": {
    message: "Restore to new instance",
    description: "Recovery panel button that opens the restore dialog",
  },
  "databases.recoveryCreateExport": {
    message: "Create export",
    description: "Recovery panel button that triggers an on-demand export",
  },
  "databases.recoveryBackups": {
    message: "Backups",
    description: "Recovery panel backup-list heading",
  },
  "databases.recoveryNoBackups": {
    message: "No backups yet.",
    description: "Recovery panel empty backup list",
  },
  "databases.recoveryExports": {
    message: "Exports",
    description: "Recovery panel export-list heading",
  },
  "databases.recoveryNoExports": {
    message: "No exports yet.",
    description: "Recovery panel empty export list",
  },
  "databases.recoveryRestoreTitle": {
    message: "Restore to a new database",
    description: "Restore dialog title",
  },
  "databases.recoveryRestoreBody": {
    message:
      "Provision a new database restored from this one's backups. The source is left untouched.",
    description: "Restore dialog body",
  },
  "databases.recoveryRestoreName": {
    message: "New database name",
    description: "Restore dialog field label",
  },
  "databases.recoveryRestoreTime": {
    message: "Point in time (optional)",
    description: "Restore dialog target-time field label",
  },
  "databases.recoveryRestoreTimeHint": {
    message: "Leave empty to restore the latest available point.",
    description: "Restore dialog target-time helper text",
  },
  "databases.recoveryRestoreConfirm": {
    message: "Restore",
    description: "Restore dialog confirm button",
  },
  "databases.recoveryExportStarted": {
    message: "Export started.",
    description: "Toast after an export is triggered",
  },
  "databases.recoveryExportError": {
    message: "Couldn't start the export. Please try again.",
    description: "Toast when an export request fails",
  },
  "databases.recoveryRestoreStarted": {
    message: "Restoring into {name}…",
    description: "Toast after a restore request is accepted",
  },
  "databases.recoveryRestoreError": {
    message: "Couldn't restore: {error}",
    description: "Toast when a restore request fails",
  },
  // --- Access control panel ---
  "databases.accessTitle": {
    message: "Access control",
    description: "Access-control panel card title",
  },
  "databases.accessDescription": {
    message:
      "Restrict the external endpoint, pool connections, and manage database users.",
    description: "Access-control panel card description",
  },
  "databases.accessAllowList": {
    message: "IP allowlist",
    description:
      "Access panel section heading (external-endpoint CIDR allowlist)",
  },
  "databases.accessAllowListHint": {
    message:
      "Only these CIDR ranges can reach the external endpoint. Empty means open to all source IPs. The internal endpoint is never affected.",
    description: "Access panel IP allowlist helper text",
  },
  "databases.accessAllowListOpen": {
    message: "Open to all source IPs.",
    description: "Access panel shown when the allowlist is empty",
  },
  "databases.accessAllowListAdd": {
    message: "Add",
    description: "Access panel button to add a CIDR to the draft allowlist",
  },
  "databases.accessAllowListRemove": {
    message: "Remove {cidr}",
    description: "Accessible label to remove a CIDR chip",
  },
  "databases.accessAllowListSave": {
    message: "Save allowlist",
    description: "Access panel button to persist the allowlist",
  },
  "databases.accessAllowListSaved": {
    message: "Allowlist updated.",
    description: "Toast after saving the IP allowlist",
  },
  "databases.accessAllowListError": {
    message: "Couldn't save the allowlist: {error}",
    description: "Toast when saving the allowlist fails",
  },
  "databases.accessUsers": {
    message: "Database users",
    description: "Access panel section heading (Postgres login roles)",
  },
  "databases.accessUsersHint": {
    message:
      "Additional login roles beyond the owner. A new user's password is shown once — copy it now.",
    description: "Access panel users helper text",
  },
  "databases.accessUsersEmpty": {
    message: "No additional users.",
    description: "Access panel empty users list",
  },
  "databases.accessUserAdd": {
    message: "Add user",
    description: "Access panel button to create a database user",
  },
  "databases.accessUserDelete": {
    message: "Delete {name}",
    description: "Accessible label to delete a database user",
  },
  "databases.accessUserPassword": {
    message: "Password",
    description: "Label for a newly-created user's password",
  },
  "databases.accessUserPasswordOnce": {
    message: "Password for {name} — shown once:",
    description: "Access panel one-time password reveal label",
  },
  "databases.accessUserCreated": {
    message: "Created user {name}.",
    description: "Toast after creating a database user",
  },
  "databases.accessUserError": {
    message: "Couldn't create the user: {error}",
    description: "Toast when creating a user fails",
  },
  "databases.accessUserDeleted": {
    message: "Deleted user {name}.",
    description: "Toast after deleting a database user",
  },
  "databases.accessUserDeleteError": {
    message: "Couldn't delete {name}. Please try again.",
    description: "Toast when deleting a user fails",
  },
  "databases.accessPooler": {
    message: "Connection pooling",
    description: "Access panel section heading (PgBouncer pooler strings)",
  },
  "databases.accessPoolerHint": {
    message:
      "Pooled connection strings routed through PgBouncer, for high-connection-count clients.",
    description: "Access panel pooler helper text",
  },
  "databases.accessPoolerReveal": {
    message: "Reveal pooled strings",
    description: "Access panel button to fetch the pooled connection strings",
  },
  "databases.accessPoolerDisabled": {
    message:
      "Connection pooling isn't enabled for this database. Enable a pooler to get pooled connection strings.",
    description: "Access panel shown when no pooler is provisioned",
  },
  "databases.accessPoolerInternal": {
    message: "Internal pooled connection",
    description: "Access panel pooled-string label (in-cluster)",
  },
  "databases.accessPoolerExternal": {
    message: "External pooled connection",
    description: "Access panel pooled-string label (public)",
  },
  "databases.accessPoolerError": {
    message: "Couldn't load the pooled connection strings.",
    description: "Toast when revealing pooled strings fails",
  },
  // --- HA panel (w1/m22) ---
  "databases.haTitle": {
    message: "High Availability",
    description: "HA panel card title",
  },
  "databases.haDescription": {
    message:
      "Replicated cluster with automatic failover. When enabled, a standby is always ready to take over.",
    description: "HA panel card description",
  },
  "databases.haStatus": {
    message: "Status",
    description: "HA panel status row label",
  },
  "databases.haEnabled": {
    message: "Enabled",
    description: "HA panel status value when HA is active",
  },
  "databases.haDisabled": {
    message: "Disabled",
    description: "HA panel status value when HA is off",
  },
  "databases.haFailover": {
    message: "Trigger failover",
    description: "HA panel button to initiate a planned switchover",
  },
  "databases.haFailoverConfirmTitle": {
    message: "Trigger failover on {name}?",
    description: "Failover-confirmation dialog title",
  },
  "databases.haFailoverConfirmBody": {
    message:
      "This promotes a standby to primary. Connections drop briefly during the switchover. The database stays available — no data is lost.",
    description: "Failover-confirmation dialog body",
  },
  "databases.haFailoverCancel": {
    message: "Cancel",
    description: "Failover-confirmation dialog cancel button",
  },
  "databases.haFailoverConfirm": {
    message: "Trigger failover",
    description: "Failover-confirmation dialog confirm button",
  },
  "databases.haFailoverSuccess": {
    message: "Failover triggered on {name}.",
    description: "Toast after a failover request is accepted",
  },
  "databases.haFailoverError": {
    message: "Couldn't trigger failover on {name}. Please try again.",
    description: "Toast when a failover request fails",
  },
  "databases.haReadReplicas": {
    message: "Read replicas",
    description: "HA panel section heading for named read replicas",
  },
  "databases.haReadReplicasEmpty": {
    message: "No named read replicas.",
    description: "HA panel empty state for read replicas",
  },
  "databases.haReplicaInternal": {
    message: "Internal",
    description: "HA panel replica connection label (in-cluster host)",
  },
  "databases.haReplicaExternal": {
    message: "External",
    description: "HA panel replica connection label (public SNI host)",
  },
  "databases.haNotEnabled": {
    message: "High availability is not enabled for this database.",
    description: "HA panel state when HA is off and there are no replicas",
  },
};

export default enDatabases;
