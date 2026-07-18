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
  "databases.statusUpgrading": {
    message: "Upgrading",
    description: "Database status badge (offline PostgreSQL major upgrade)",
  },
  "databases.statusUnavailable": {
    message: "Unavailable",
    description: "Database status badge (provisioning failed)",
  },
  "databases.statusSuspended": {
    message: "Suspended",
    description:
      "Database status badge (hibernated; suspension wins over the status enum)",
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
      "This action cannot be undone. Are you sure you want to delete this Postgres instance?",
    description: "Delete-confirmation dialog body (Render-style copy)",
  },
  "databases.deleteConfirmPrompt": {
    message: "Type {phrase} below to confirm.",
    description:
      "Body prompt naming the exact 'sudo delete postgres <name>' phrase (rendered bold by SudoCommandField)",
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
      "Use lowercase letters, digits, and hyphens (up to 30 characters); can't start or end with a hyphen.",
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
  "databases.capLimitTitle": {
    message: "Postgres limit reached",
    description:
      "Alert title when the workspace's Postgres creation cap is hit (w7/m9)",
  },
  "databases.capLimitUpgrade": {
    message: "Upgrade plan",
    description:
      "Upgrade CTA button inside the Postgres cap-limit Alert (w7/m9)",
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
  "databases.versionLabel": {
    message: "PostgreSQL {version}",
    description: "Formatted PostgreSQL major version label",
  },
  "databases.versionUpgradeAction": {
    message: "Upgrade",
    description: "Button opening the PostgreSQL major-version upgrade dialog",
  },
  "databases.versionUpgrading": {
    message: "Upgrade in progress",
    description:
      "Version-row status while the database is offline for pg_upgrade",
  },
  "databases.versionUpgradeTitle": {
    message: "Upgrade PostgreSQL",
    description: "PostgreSQL major-version upgrade dialog title",
  },
  "databases.versionUpgradeDescription": {
    message:
      "Upgrade this database from PostgreSQL {version} to a newer supported major version.",
    description: "PostgreSQL major-version upgrade dialog description",
  },
  "databases.versionUpgradeTarget": {
    message: "Target version",
    description: "PostgreSQL major-version upgrade target selector label",
  },
  "databases.versionUpgradeDowntime": {
    message:
      "The database will be unavailable while the offline upgrade runs. Test compatibility first and schedule downtime; large databases can take up to an hour.",
    description:
      "Downtime warning in the PostgreSQL major-version upgrade dialog",
  },
  "databases.versionUpgradeErrorTitle": {
    message: "Upgrade blocked",
    description:
      "Inline backend-guard error title in the version upgrade dialog",
  },
  "databases.versionUpgradeCancel": {
    message: "Cancel",
    description: "PostgreSQL major-version upgrade dialog cancel button",
  },
  "databases.versionUpgradeConfirm": {
    message: "Upgrade to PostgreSQL {version}",
    description: "PostgreSQL major-version upgrade dialog confirmation button",
  },
  "databases.versionUpgradeAccepted": {
    message: "Upgrade to PostgreSQL {version} started.",
    description: "Toast after a PostgreSQL major-version upgrade is accepted",
  },
  "databases.versionUpgradeError": {
    message: "Couldn't start the PostgreSQL upgrade. Please try again.",
    description: "Fallback error when the PostgreSQL upgrade mutation fails",
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
  "databases.diskAutoscalingLabel": {
    message: "Disk autoscaling",
    description: "Label beside the database disk-usage chart toggle",
  },
  "databases.diskAutoscalingSize": {
    message: "{current} GB current · {cap} GB max",
    description: "Current and maximum Postgres disk size beside autoscaling",
  },
  "databases.diskAutoscalingHint": {
    message:
      "At 90% full, storage grows by 50%, rounded up to 5 GB. Increases are permanent and limited to once every 12 hours.",
    description: "Accessible explanation of Postgres disk autoscaling",
  },
  "databases.diskAutoscalingEnabled": {
    message: "Disk autoscaling enabled.",
    description: "Toast after enabling Postgres disk autoscaling",
  },
  "databases.diskAutoscalingDisabled": {
    message: "Disk autoscaling disabled.",
    description: "Toast after disabling Postgres disk autoscaling",
  },
  "databases.diskAutoscalingError": {
    message: "Couldn't update disk autoscaling. Please try again.",
    description: "Toast when the disk-autoscaling mutation fails",
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
  "databases.metaRegion": {
    message: "Region",
    description: "Database detail metadata row label (platform placement)",
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
  // --- Bottom action row (Render parity: Info page footer buttons) ---
  "databases.dangerDelete": {
    message: "Delete Database",
    description: "Detail-page bottom action: permanently delete the database",
  },
  "databases.dangerRestart": {
    message: "Restart Database",
    description: "Detail-page bottom action: rolling restart of the primary",
  },
  "databases.dangerSuspend": {
    message: "Suspend Database",
    description:
      "Detail-page bottom action: hibernate the database (stop compute, keep data)",
  },
  "databases.dangerResume": {
    message: "Resume Database",
    description: "Detail-page bottom action: wake a suspended database",
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
  "databases.recoveryDownloadExport": {
    message: "Download",
    description: "Download an available logical database export",
  },
  "databases.recoveryExportInProgress": {
    message: "An export is already in progress.",
    description: "Reason the create-export button is disabled",
  },
  "databases.recoveryExportPreparing": {
    message: "The export is still being prepared.",
    description: "Reason an in-progress export cannot be downloaded",
  },
  "databases.recoveryExportExpired": {
    message: "This export has expired and its artifact was removed.",
    description: "Reason an expired export cannot be downloaded",
  },
  "databases.recoveryExportUnavailable": {
    message: "This export isn't available for download.",
    description: "Fallback reason an export cannot be downloaded",
  },
  "databases.recoveryExportFailure": {
    message: "Export failed: {reason}",
    description: "Logical export failure reason",
  },
  "databases.recoveryExportStatusCreated": {
    message: "Created",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusRunning": {
    message: "Running",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusAvailable": {
    message: "Available",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusFailed": {
    message: "Failed",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusExpiring": {
    message: "Expiring",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusExpired": {
    message: "Expired",
    description: "Logical export status",
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
  "databases.accessAllowListDescription": {
    message: "Description (optional)",
    description:
      "Placeholder for the optional per-entry allowlist description input",
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
  // --- Plan section (m16) ---
  "databases.planTitle": {
    message: "Instance type",
    description: "Database detail plan-picker card title",
  },
  "databases.planDescription": {
    message:
      "Change the instance type. The operator resizes resources on the next reconcile — a rolling update that preserves data.",
    description: "Database detail plan-picker card description",
  },
  "databases.planPickerSave": {
    message: "Save",
    description: "Database plan-picker save button",
  },
  "databases.planPickerCancel": {
    message: "Cancel",
    description: "Database plan-picker cancel / reset button",
  },
  "databases.planPickerConfirmTitle": {
    message: "Change to {name}?",
    description: "Database plan-picker confirm dialog title",
  },
  "databases.planPickerConfirmBody": {
    message:
      "The operator will resize the database's compute resources on the next reconcile. Connections drop briefly during the rolling restart.",
    description: "Database plan-picker confirm dialog body",
  },
  "databases.planPickerSuccess": {
    message: "Updating plan to {name}…",
    description: "Toast after a plan update is accepted",
  },
  "databases.planPickerError": {
    message: "Couldn't update the plan. Please try again.",
    description: "Toast when a plan update fails",
  },
  // --- Name section (w9/m3) ---
  "databases.nameTitle": {
    message: "Database name",
    description: "Database detail rename card title",
  },
  "databases.nameDescription": {
    message:
      "Change the display name. The database ID and all connection details stay the same.",
    description: "Database detail rename card description",
  },
  "databases.nameSave": {
    message: "Save name",
    description: "Database rename save button",
  },
  "databases.nameSuccess": {
    message: "Renamed database to {name}.",
    description: "Toast after a database rename succeeds",
  },
  "databases.nameError": {
    message: "Couldn't rename the database. Please try again.",
    description: "Toast when a database rename fails unexpectedly",
  },
  "databases.nameConflict": {
    message: "A database with that name already exists in this workspace.",
    description: "Toast when a database rename collides with another name",
  },
  "databases.nameInvalid": {
    message:
      "Use lowercase letters, digits, and hyphens (up to 30 characters); don't start or end with a hyphen.",
    description: "Database rename validation message",
  },
  // --- Insights panel (w2/m25) ---
  "databases.insightsTitle": {
    message: "Insights",
    description: "Database insights panel card title",
  },
  "databases.insightsDescription": {
    message:
      "Live introspection into processes, query performance, storage, and configuration.",
    description: "Database insights panel card description",
  },
  "databases.insightsRefresh": {
    message: "Refresh insights",
    description: "Accessible label for the insights refresh button",
  },
  "databases.insightsUnavailable": {
    message: "Unavailable — the database may still be provisioning.",
    description: "Insights sub-section error state",
  },
  "databases.insightsSizeTitle": {
    message: "Storage",
    description: "Insights panel — database/table sizes sub-section heading",
  },
  "databases.insightsNoSizes": {
    message: "No size data yet.",
    description: "Insights storage empty state",
  },
  "databases.insightsProcessesTitle": {
    message: "Active processes",
    description: "Insights panel — pg_stat_activity sub-section heading",
  },
  "databases.insightsNoProcesses": {
    message: "No active processes.",
    description: "Insights processes empty state",
  },
  "databases.insightsTopQueriesTitle": {
    message: "Top queries",
    description: "Insights panel — pg_stat_statements sub-section heading",
  },
  "databases.insightsNoTopQueries": {
    message:
      "No query stats yet — pg_stat_statements may not be enabled on this cluster.",
    description: "Insights top-queries empty state",
  },
  "databases.insightsTableScansTitle": {
    message: "Table scans",
    description: "Insights panel — pg_stat_user_tables sub-section heading",
  },
  "databases.insightsNoTableScans": {
    message: "No user tables with scan stats yet.",
    description: "Insights table-scans empty state",
  },
  "databases.insightsParamsTitle": {
    message: "Non-default parameters",
    description: "Insights panel — pg_settings sub-section heading",
  },
  "databases.insightsNoParams": {
    message: "All parameters are at their defaults.",
    description: "Insights parameter-overrides empty state",
  },
  "databases.insightsParamsAdd": {
    message: "Add override",
    description: "Button that adds a Postgres parameter override row",
  },
  "databases.insightsParamsSave": {
    message: "Save overrides",
    description: "Button that replaces the Postgres parameter overrides",
  },
  "databases.insightsParamsSaving": {
    message: "Saving…",
    description: "Postgres parameter-overrides save button while saving",
  },
  "databases.insightsParamsDiscard": {
    message: "Discard changes",
    description: "Button that restores the last saved parameter draft",
  },
  "databases.insightsParamsActions": {
    message: "Actions",
    description: "Screen-reader-only header for parameter-row actions",
  },
  "databases.insightsParamNameLabel": {
    message: "Parameter {index} name",
    description: "Accessible label for a parameter override name input",
  },
  "databases.insightsParamValueLabel": {
    message: "Parameter {index} value",
    description: "Accessible label for a parameter override value input",
  },
  "databases.insightsParamsRemove": {
    message: "Remove {name}",
    description: "Accessible label for a parameter override remove button",
  },
  "databases.insightsParamsUnnamed": {
    message: "parameter {index}",
    description: "Fallback name in the remove label for an empty draft row",
  },
  "databases.insightsParamsPending": {
    message: "Pending",
    description: "Source shown for a newly added, not-yet-applied override",
  },
  "databases.insightsParamsBlank": {
    message: "Every override needs both a parameter name and a value.",
    description: "Inline validation for an incomplete parameter row",
  },
  "databases.insightsParamsDuplicate": {
    message: "{name} appears more than once.",
    description: "Inline validation for duplicate parameter names",
  },
  "databases.insightsParamsManaged": {
    message:
      "shared_preload_libraries is managed by bex and can't be overridden.",
    description:
      "Inline validation for the platform-managed Postgres parameter",
  },
  "databases.insightsParamsSaveError": {
    message: "Couldn't save parameter overrides. Please try again.",
    description: "Fallback error when the parameter mutation fails",
  },
  // Table column headers shared across insight sub-sections
  "databases.insightsColTable": {
    message: "Table",
    description: "Insights table column header (table name)",
  },
  "databases.insightsColSize": {
    message: "Size",
    description: "Insights table column header (relation size)",
  },
  "databases.insightsColUser": {
    message: "User",
    description: "Insights processes column header (pg role)",
  },
  "databases.insightsColState": {
    message: "State",
    description: "Insights processes column header (active/idle/…)",
  },
  "databases.insightsColDuration": {
    message: "Duration",
    description: "Insights processes column header (seconds since query start)",
  },
  "databases.insightsColQuery": {
    message: "Query",
    description: "Insights column header (SQL text, truncated)",
  },
  "databases.insightsColCalls": {
    message: "Calls",
    description: "Insights top-queries column header (invocation count)",
  },
  "databases.insightsColTotalTime": {
    message: "Total time",
    description: "Insights top-queries column header (total exec time)",
  },
  "databases.insightsColMeanTime": {
    message: "Mean time",
    description: "Insights top-queries column header (mean exec time per call)",
  },
  "databases.insightsColSeqScans": {
    message: "Seq scans",
    description: "Insights table-scans column header (sequential scan count)",
  },
  "databases.insightsColIdxScans": {
    message: "Index scans",
    description: "Insights table-scans column header (index scan count)",
  },
  "databases.insightsColLiveRows": {
    message: "Live rows",
    description:
      "Insights table-scans column header (estimated live row count)",
  },
  "databases.insightsColParam": {
    message: "Parameter",
    description:
      "Insights parameter-overrides column header (pg_settings.name)",
  },
  "databases.insightsColSetting": {
    message: "Setting",
    description:
      "Insights parameter-overrides column header (pg_settings.setting)",
  },
  "databases.insightsColSource": {
    message: "Source",
    description:
      "Insights parameter-overrides column header (pg_settings.source)",
  },
  "databases.sqlTitle": {
    message: "SQL console",
    description: "Database detail SQL console card title",
  },
  "databases.sqlDescription": {
    message:
      "Run a single SQL statement against this database. Results are limited to 500 rows and queries time out after 10 seconds.",
    description: "Database SQL console safety summary",
  },
  "databases.sqlEditorLabel": {
    message: "SQL query",
    description: "Accessible label for the SQL editor",
  },
  "databases.sqlShortcut": {
    message: "Press Ctrl+Enter or ⌘+Enter to run",
    description: "SQL console keyboard shortcut hint",
  },
  "databases.sqlRun": {
    message: "Run query",
    description: "SQL console run button",
  },
  "databases.sqlRunning": {
    message: "Running…",
    description: "SQL console run button while a query is executing",
  },
  "databases.sqlUnknownError": {
    message: "The query failed. Check the statement and try again.",
    description: "SQL console fallback error",
  },
  "databases.sqlReturnedRows": {
    message: "{count} rows returned",
    description: "SQL console SELECT result count",
  },
  "databases.sqlAffectedRows": {
    message: "{count} rows affected",
    description: "SQL console write result count",
  },
  "databases.sqlTruncated": {
    message: "Result capped at {limit} rows",
    description: "SQL console backend row-cap notice",
  },
  "databases.sqlNull": {
    message: "NULL",
    description: "SQL console null cell value",
  },
  "databases.sqlNoRows": {
    message: "The query returned no rows.",
    description: "SQL console empty result message",
  },
  "databases.sqlPrevious": {
    message: "Previous",
    description: "SQL result pagination previous button",
  },
  "databases.sqlNext": {
    message: "Next",
    description: "SQL result pagination next button",
  },
  "databases.sqlPage": {
    message: "Page {page} of {pages}",
    description: "SQL result pagination position",
  },
  "databases.sqlHistory": {
    message: "Query history",
    description: "SQL console session history heading",
  },
  "databases.sqlClearHistory": {
    message: "Clear",
    description: "SQL console clear-history button",
  },
  "databases.sqlNoHistory": {
    message: "Queries run in this browser session appear here.",
    description: "SQL console empty history message",
  },
  "databases.sqlConfirmTitle": {
    message: "Run a statement that can modify data?",
    description: "SQL console write confirmation title",
  },
  "databases.sqlConfirmDescription": {
    message:
      "This statement is not recognized as read-only. It can change or delete database data and cannot be undone by bex.",
    description: "SQL console write confirmation warning",
  },
  "databases.sqlConfirmCancel": {
    message: "Cancel",
    description: "SQL console write confirmation cancel button",
  },
  "databases.sqlConfirmRun": {
    message: "Run statement",
    description: "SQL console confirmed write action",
  },
  "databases.detailNavLabel": {
    message: "Database details",
    description: "Accessible label for the managed Postgres detail tabs",
  },
  "databases.overviewTab": {
    message: "Overview",
    description: "Managed Postgres overview tab",
  },
  "databases.logsTab": {
    message: "Logs",
    description: "Managed Postgres logs tab",
  },
  "databases.logsRangeLabel": {
    message: "Time range",
    description: "Managed Postgres log range filter label",
  },
  "databases.logsRange1h": {
    message: "Last hour",
    description: "Managed Postgres one-hour log range",
  },
  "databases.logsRange6h": {
    message: "Last 6 hours",
    description: "Managed Postgres six-hour log range",
  },
  "databases.logsRange24h": {
    message: "Last 24 hours",
    description: "Managed Postgres 24-hour log range",
  },
  "databases.logsInstanceLabel": {
    message: "Database instance",
    description: "Managed Postgres log instance filter label",
  },
  "databases.logsAllInstances": {
    message: "All instances",
    description: "Managed Postgres log instance filter all option",
  },
  "databases.logsSearchPlaceholder": {
    message: "Search database logs",
    description: "Managed Postgres log search placeholder",
  },
  "databases.logsLoading": {
    message: "Loading database logs…",
    description: "Managed Postgres logs loading state",
  },
  "databases.logsEmptyTitle": {
    message: "No database logs yet",
    description: "Managed Postgres logs empty-state title",
  },
  "databases.logsEmptyBody": {
    message: "Postgres output in this time range will appear here.",
    description: "Managed Postgres logs empty-state body",
  },
  "databases.logsEmptyFilteredBody": {
    message: "No database logs match the current filters.",
    description: "Managed Postgres logs filtered empty-state body",
  },
  "databases.logsUnavailableTitle": {
    message: "Database logs aren't configured",
    description: "Managed Postgres logs unavailable-state title",
  },
  "databases.logsUnavailableBody": {
    message:
      "This installation has neither durable log history nor a live CNPG pod-log source configured.",
    description: "Managed Postgres logs unavailable-state body",
  },
  "databases.logsUnauthorizedTitle": {
    message: "You can't view these logs",
    description: "Managed Postgres logs unauthorized-state title",
  },
  "databases.logsUnauthorizedBody": {
    message: "Database logs require contributor access in this workspace.",
    description: "Managed Postgres logs unauthorized-state body",
  },
  "databases.logsErrorTitle": {
    message: "Couldn't load database logs",
    description: "Managed Postgres logs generic error title",
  },
};

export default enDatabases;
