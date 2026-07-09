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
};

export default enDatabases;
