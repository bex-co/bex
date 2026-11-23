import type { TranslationEntry } from "@/i18n";

const enSandboxGroups: Record<string, TranslationEntry> = {
  "sandboxGroups.createTitle": {
    message: "New Sandbox Group",
    description:
      "Title of the new-sandbox-group dialog and its entry in the + New menus",
  },
  "sandboxGroups.createDescription": {
    message:
      "A sandbox group caps how many of its sandboxes can be active at once. Group names must be unique within your workspace.",
    description: "New-sandbox-group dialog description",
  },
  "sandboxGroups.fieldName": {
    message: "Name",
    description: "New-sandbox-group dialog name field label",
  },
  "sandboxGroups.fieldNamePlaceholder": {
    message: "my-sandbox-group",
    description: "Placeholder for the new sandbox group's name",
  },
  "sandboxGroups.fieldRegion": {
    message: "Region",
    description:
      "Label of the region row in the created sandbox group's confirmation",
  },
  "sandboxGroups.createCancel": {
    message: "Cancel",
    description: "Cancel button of the new-sandbox-group dialog",
  },
  "sandboxGroups.createSubmit": {
    message: "Create Sandbox Group",
    description: "Submit button of the new-sandbox-group dialog",
  },
  "sandboxGroups.createError": {
    message: "Couldn't create sandbox group {name}. Please try again.",
    description: "Toast shown when creating a sandbox group fails",
  },
  "sandboxGroups.errorNameTaken": {
    message:
      'A sandbox group named "{name}" already exists in this workspace. Choose a different name.',
    description:
      "Inline error under the name field when the group name is already taken",
  },
  "sandboxGroups.errorNameInvalid": {
    message:
      "A group name must be 1 to {max} characters, with no leading or trailing space and no control characters.",
    description: "Inline error under the name field when the name is refused",
  },
  "sandboxGroups.errorLimitReached_one": {
    message: "This workspace already has the maximum of {count} sandbox group.",
    description:
      "Inline alert when the workspace is at its sandbox group limit (singular)",
  },
  "sandboxGroups.errorLimitReached_other": {
    message:
      "This workspace already has the maximum of {count} sandbox groups.",
    description:
      "Inline alert when the workspace is at its sandbox group limit",
  },
  "sandboxGroups.errorLimitReachedGeneric": {
    message: "This workspace has reached its sandbox group limit.",
    description:
      "Inline alert for the sandbox group limit when the server names no number",
  },
  "sandboxGroups.errorLimitTitle": {
    message: "Sandbox group limit reached",
    description: "Heading of the inline sandbox-group-limit alert",
  },
  "sandboxGroups.createdTitle": {
    message: "Sandbox group created",
    description: "Dialog title once a sandbox group has been created",
  },
  "sandboxGroups.createdDescription": {
    message:
      "{name} is ready. Pass its ID as sandboxGroupId when you create a sandbox in it.",
    description: "Dialog description once a sandbox group has been created",
  },
  "sandboxGroups.createdDone": {
    message: "Done",
    description: "Button that closes the created-sandbox-group confirmation",
  },
  "sandboxGroups.fieldId": {
    message: "ID",
    description: "Label of the new sandbox group's ID in the confirmation",
  },
  "sandboxGroups.fieldDefault": {
    message: "Default group",
    description:
      "Label of the confirmation row that says whether the group is the workspace's default",
  },
  "sandboxGroups.defaultYes": {
    message: "Yes",
    description: "The created group is the workspace's default sandbox group",
  },
  "sandboxGroups.defaultNo": {
    message: "No",
    description:
      "The created group is not the workspace's default sandbox group",
  },
  "sandboxGroups.fieldConcurrencyLimit": {
    message: "Concurrency limit",
    description:
      "Label of the confirmation row with the most active sandboxes the group allows",
  },
  "sandboxGroups.copyId": {
    message: "Copy sandbox group ID",
    description: "Accessible label of the copy button next to the group's ID",
  },
  "sandboxGroups.copied": {
    message: "Sandbox group ID copied",
    description: "Toast after copying the sandbox group ID",
  },
  "sandboxGroups.copyError": {
    message: "Couldn't copy the sandbox group ID",
    description: "Toast when copying the sandbox group ID fails",
  },
  "sandboxGroups.environmentLoading": {
    message: "Loading environments",
    description:
      "Accessible name of the loading placeholder shown where the new-sandbox-group dialog's Environment select will appear",
  },
  "sandboxGroups.environmentsError": {
    message:
      "Couldn't load your environments. You can still create the group without one.",
    description:
      "Shown in the new-sandbox-group dialog when the workspace's environments could not be read; the group can still be created without one",
  },
  "sandboxGroups.errorEnvironmentGone": {
    message: "That environment no longer exists. Choose another or none.",
    description:
      "Inline error under the Environment select when the chosen environment was deleted before the group was created",
  },
  "sandboxGroups.pageTitle": {
    message: "Sandbox Groups",
    description: "Heading for the workspace sandbox groups page",
  },
  "sandboxGroups.pageDescription": {
    message:
      "Manage sandbox concurrency limits and outbound brokered credentials for each group.",
    description: "Subtitle for the workspace sandbox groups page",
  },
  "sandboxGroups.resourceType": {
    message: "Sandbox Group",
    description: "Singular resource type name for a sandbox group",
  },
  "sandboxGroups.backToList": {
    message: "Back to sandbox groups",
    description: "Accessible label of the back link on a sandbox group detail page",
  },
  "sandboxGroups.defaultBadge": {
    message: "Default",
    description: "Badge indicating the workspace default sandbox group",
  },
  "sandboxGroups.statusReady": {
    message: "Ready",
    description:
      "Status of a sandbox group row in a project environment's resource table; Render shows every group as Ready",
  },
  "sandboxGroups.environmentUpdated": {
    message: 'Sandbox groups for "{name}" updated.',
    description:
      "Toast after sandbox groups were moved into or out of an environment; {name} is the environment",
  },
  "sandboxGroups.environmentUpdateError": {
    message: "Couldn't move sandbox group {group}. Please try again.",
    description:
      "Toast when moving a sandbox group into or out of an environment fails and the server gave no reason to show",
  },
  "sandboxGroups.listLoadError": {
    message: "Couldn't load sandbox groups. Please try again.",
    description: "Error state when loading sandbox groups fails",
  },
  "sandboxGroups.listEmptyTitle": {
    message: "No sandbox groups yet",
    description: "Empty state heading when the workspace has no sandbox groups",
  },
  "sandboxGroups.listEmptyDescription": {
    message:
      "Create a sandbox group to set concurrency limits and configure brokered outbound credentials.",
    description: "Empty state body when the workspace has no sandbox groups",
  },
  "sandboxGroups.manageCredentialsButton": {
    message: "Credentials",
    description: "Button on each sandbox group row linking to its detail and credentials",
  },
  "sandboxGroups.detailLoadError": {
    message: "Couldn't load this sandbox group. Please try again.",
    description: "Error state when loading a single sandbox group fails",
  },
  "sandboxGroups.credSectionTitle": {
    message: "Outbound Credentials",
    description: "Card title for the sandbox group credentials panel",
  },
  "sandboxGroups.credSectionDescription": {
    message:
      "Brokered secrets injected into matching outbound HTTPS requests from sandboxes in this group. Stored secret values are write-only and never displayed.",
    description: "Card subtitle for the sandbox group credentials panel",
  },
  "sandboxGroups.credAddButton": {
    message: "Add Credential",
    description: "Button to open the create sandbox group credential dialog",
  },
  "sandboxGroups.credLoadError": {
    message: "Couldn't load sandbox group credentials. Please try again.",
    description: "Error state when fetching sandbox group credentials fails",
  },
  "sandboxGroups.credEmptyTitle": {
    message: "No credentials configured",
    description: "Empty state heading when a sandbox group has no credentials",
  },
  "sandboxGroups.credEmptyDescriptionAdmin": {
    message:
      "Add a credential to inject authentication into outbound requests without exposing the secret to sandbox code.",
    description: "Empty state body for admins when no credentials exist",
  },
  "sandboxGroups.credEmptyDescriptionReadOnly": {
    message:
      "A workspace admin can add outbound credentials for sandboxes in this group.",
    description: "Empty state body for non-admins when no credentials exist",
  },
  "sandboxGroups.credColId": {
    message: "Credential ID",
    description: "Table column header for credential ID",
  },
  "sandboxGroups.credColDomain": {
    message: "Domain",
    description: "Table column header for destination domain",
  },
  "sandboxGroups.credColInjection": {
    message: "Injection",
    description: "Table column header for injection method and target",
  },
  "sandboxGroups.credColRouting": {
    message: "Routing",
    description: "Table column header for HTTP methods and path filters",
  },
  "sandboxGroups.credColEnabled": {
    message: "Enabled",
    description: "Table column header for enabled toggle or status",
  },
  "sandboxGroups.credColActions": {
    message: "Actions",
    description: "Table column header for row actions",
  },
  "sandboxGroups.credAllMethods": {
    message: "All methods",
    description: "Badge shown when a credential matches all HTTP methods",
  },
  "sandboxGroups.credAllPaths": {
    message: "All paths",
    description: "Badge shown when a credential matches all request paths",
  },
  "sandboxGroups.credStatusEnabled": {
    message: "Enabled",
    description: "Read-only badge when a credential is enabled",
  },
  "sandboxGroups.credStatusDisabled": {
    message: "Disabled",
    description: "Read-only badge when a credential is disabled",
  },
  "sandboxGroups.credToggleEnabledLabel": {
    message: "Toggle enabled state for {id}",
    description: "Accessible label for the enabled switch on a credential row",
  },
  "sandboxGroups.credVersionsActionLabel": {
    message: "View versions for {id}",
    description: "Accessible label for the version history icon button",
  },
  "sandboxGroups.credEditActionLabel": {
    message: "Edit credential {id}",
    description: "Accessible label for the edit credential icon button",
  },
  "sandboxGroups.credDeleteActionLabel": {
    message: "Delete credential {id}",
    description: "Accessible label for the delete credential icon button",
  },
  "sandboxGroups.credVersionsButton": {
    message: "Versions",
    description: "Mobile button label to open credential version history",
  },
  "sandboxGroups.credEditButton": {
    message: "Edit",
    description: "Mobile button label to edit a credential",
  },
  "sandboxGroups.credDeleteButton": {
    message: "Delete",
    description: "Button label to delete a credential",
  },
  "sandboxGroups.credCreateTitle": {
    message: "Add Credential",
    description: "Dialog title when creating a sandbox group credential",
  },
  "sandboxGroups.credCreateDescription": {
    message:
      "Configure a write-only outbound credential for this sandbox group.",
    description: "Dialog description when creating a sandbox group credential",
  },
  "sandboxGroups.credEditTitle": {
    message: "Edit {id}",
    description: "Dialog title when editing a sandbox group credential",
  },
  "sandboxGroups.credEditDescriptionAdmin": {
    message:
      "Update routing, injection, domain, or rotate the secret value. Leave the secret blank to keep the current value.",
    description: "Dialog description when an admin edits a credential",
  },
  "sandboxGroups.credEditDescriptionDeveloper": {
    message:
      "Update injection, methods, paths, or enabled state. Changing the domain or secret value requires a workspace admin.",
    description: "Dialog description when a developer edits a credential",
  },
  "sandboxGroups.credFieldId": {
    message: "Credential ID",
    description: "Form label for credential ID",
  },
  "sandboxGroups.credFieldIdHelp": {
    message:
      "Case-sensitive identifier using letters, numbers, ., _, ~, or -. Cannot start with rnd-.",
    description: "Help text for credential ID format",
  },
  "sandboxGroups.credFieldDomain": {
    message: "Destination domain",
    description: "Form label for destination hostname",
  },
  "sandboxGroups.credAdminOnlyDomainSecret": {
    message: "Only workspace admins can change the domain or secret value.",
    description: "Helper text shown to developers on read-only domain field",
  },
  "sandboxGroups.credFieldValue": {
    message: "Secret value",
    description: "Form label for secret value on create",
  },
  "sandboxGroups.credFieldValuePlaceholder": {
    message: "Paste secret value",
    description: "Placeholder for secret value input on create",
  },
  "sandboxGroups.credFieldReplacementValue": {
    message: "New secret value (optional)",
    description: "Form label for rotating secret value on edit",
  },
  "sandboxGroups.credFieldReplacementValuePlaceholder": {
    message: "Leave blank to keep existing secret",
    description: "Placeholder for optional secret rotation input on edit",
  },
  "sandboxGroups.credFieldValueWriteOnlyHelp": {
    message: "Write-only (up to 5 KiB). Never shown again after saving.",
    description: "Help text explaining write-only secret handling",
  },
  "sandboxGroups.credFieldInjectionType": {
    message: "Injection type",
    description: "Form label for injection type select",
  },
  "sandboxGroups.credInjectionHeader": {
    message: "HTTP Header",
    description: "Injection type option for HTTP header",
  },
  "sandboxGroups.credInjectionBasic": {
    message: "Basic Auth",
    description: "Injection type option for HTTP Basic authentication",
  },
  "sandboxGroups.credInjectionCookie": {
    message: "Cookie",
    description: "Injection type option for HTTP cookie",
  },
  "sandboxGroups.credInjectionQuery": {
    message: "Query Parameter",
    description: "Injection type option for URL query parameter",
  },
  "sandboxGroups.credFieldInjectionName": {
    message: "Target name",
    description: "Form label for header, cookie, or query parameter name",
  },
  "sandboxGroups.credBasicHelp": {
    message:
      "Uses the Authorization header automatically. Provide username:password.",
    description: "Helper text shown when basic auth injection is selected",
  },
  "sandboxGroups.credFieldTemplate": {
    message: "Value template (optional)",
    description: "Form label for optional value template",
  },
  "sandboxGroups.credFieldTemplateHelp": {
    message: "Defaults to {token}. Include 1 to 5 occurrences of {token}.",
    description: "Help text for injection template placeholder syntax",
  },
  "sandboxGroups.credFieldMethods": {
    message: "HTTP methods",
    description: "Form label for allowed HTTP methods multi-select",
  },
  "sandboxGroups.credFieldMethodsHelp": {
    message: "Leave all unselected to match any HTTP method.",
    description: "Help text for HTTP methods selection",
  },
  "sandboxGroups.credFieldPaths": {
    message: "Path patterns (one per line)",
    description: "Form label for path patterns textarea",
  },
  "sandboxGroups.credFieldPathsHelp": {
    message:
      "Absolute paths starting with /. Supports whole-segment * and ** wildcards. Leave empty for all paths.",
    description: "Help text for path pattern syntax",
  },
  "sandboxGroups.credFieldEnabled": {
    message: "Enabled",
    description: "Form label for credential enabled switch",
  },
  "sandboxGroups.credFieldEnabledHelp": {
    message: "Disabled credentials are kept in the group but never injected.",
    description: "Help text for credential enabled switch",
  },
  "sandboxGroups.credCreateSubmit": {
    message: "Create Credential",
    description: "Submit button label for creating a credential",
  },
  "sandboxGroups.credEditSubmit": {
    message: "Save Changes",
    description: "Submit button label for editing a credential",
  },
  "sandboxGroups.credCreatedToast": {
    message: "Credential {id} created.",
    description: "Success toast after creating a credential",
  },
  "sandboxGroups.credUpdatedToast": {
    message: "Credential {id} updated.",
    description: "Success toast after updating a credential",
  },
  "sandboxGroups.credDeletedToast": {
    message: "Credential {id} deleted.",
    description: "Success toast after deleting a credential",
  },
  "sandboxGroups.credRestoredToast": {
    message: "Restored {id} from version v{version}.",
    description: "Success toast after restoring a credential version",
  },
  "sandboxGroups.credCreateError": {
    message: "Couldn't create credential. Please check your inputs.",
    description: "Fallback error when creating a credential fails",
  },
  "sandboxGroups.credUpdateError": {
    message: "Couldn't update credential. Please try again.",
    description: "Fallback error when updating a credential fails",
  },
  "sandboxGroups.credDeleteError": {
    message: "Couldn't delete credential. Please try again.",
    description: "Fallback error when deleting a credential fails",
  },
  "sandboxGroups.credRestoreError": {
    message: "Couldn't restore credential version. Please try again.",
    description: "Fallback error when restoring a credential version fails",
  },
  "sandboxGroups.credErrorExists": {
    message: 'A credential with ID "{id}" already exists in this group.',
    description: "Inline error when creating a duplicate credential ID",
  },
  "sandboxGroups.credErrorBusy": {
    message:
      "Another credential update is in progress for this group. Please retry shortly.",
    description: "Error when concurrent credential writes contend on the group lock",
  },
  "sandboxGroups.credErrorUnavailable": {
    message: "Credential storage is currently unavailable.",
    description: "Error when OpenBao credential storage is unconfigured or unreachable",
  },
  "sandboxGroups.credErrorVersionNotFound": {
    message: "That credential version is no longer retained.",
    description: "Error when restoring an evicted or unknown version",
  },
  "sandboxGroups.credVersionsTitle": {
    message: "Version history · {id}",
    description: "Dialog title for credential version history",
  },
  "sandboxGroups.credVersionsDescription": {
    message:
      "Retains the current configuration plus up to 3 most recent distinct prior configurations.",
    description: "Dialog subtitle explaining version retention policy",
  },
  "sandboxGroups.credVersionsError": {
    message: "Couldn't load version history. Please try again.",
    description: "Error state when loading credential versions fails",
  },
  "sandboxGroups.credVersionsEmpty": {
    message: "No retained versions found.",
    description: "Empty state for credential versions dialog",
  },
  "sandboxGroups.credVersionCurrent": {
    message: "Current",
    description: "Badge marking the active credential version",
  },
  "sandboxGroups.credVersionRestoredFrom": {
    message: "Restored from v{version}",
    description: "Badge showing which prior version was restored",
  },
  "sandboxGroups.credFingerprintLabel": {
    message: "Fingerprint",
    description: "Label before the truncated SHA-256 fingerprint",
  },
  "sandboxGroups.credUpdatedBy": {
    message: "by {actor}",
    description: "Actor attribution on a credential version row",
  },
  "sandboxGroups.credRestoreAction": {
    message: "Restore",
    description: "Button label to restore a prior credential version",
  },
  "sandboxGroups.credRestoreConfirmTitle": {
    message: "Restore {id} to v{version}?",
    description: "Confirmation dialog title when restoring a credential version",
  },
  "sandboxGroups.credRestoreConfirmDescription": {
    message:
      "This creates a new current version for {id} using the configuration and secret from v{version}.",
    description: "Confirmation dialog body when restoring a credential version",
  },
  "sandboxGroups.credDeleteConfirmTitle": {
    message: "Delete credential {id}?",
    description: "Confirmation dialog title when deleting a credential",
  },
  "sandboxGroups.credDeleteConfirmDescription": {
    message:
      "Deleting {id} permanently purges its stored secret and all retained versions.",
    description: "Confirmation dialog body when deleting a credential",
  },
  "sandboxGroups.credValIdRequired": {
    message: "Credential ID is required.",
    description: "Validation error when credential ID is empty",
  },
  "sandboxGroups.credValIdInvalid": {
    message: "Use only letters, numbers, periods, underscores, tildes, and hyphens.",
    description: "Validation error when credential ID has disallowed characters",
  },
  "sandboxGroups.credValIdReserved": {
    message: 'Credential IDs ".", "..", and values starting with "rnd-" are reserved.',
    description: "Validation error when credential ID is reserved",
  },
  "sandboxGroups.credValDomainRequired": {
    message: "Destination domain is required.",
    description: "Validation error when domain is empty",
  },
  "sandboxGroups.credValDomainInvalid": {
    message: "Enter a valid public DNS hostname without wildcards, ports, or paths.",
    description: "Validation error when domain format is invalid",
  },
  "sandboxGroups.credValDomainPrivate": {
    message: "Local, private, and cluster hostnames are not allowed.",
    description: "Validation error when domain is private or cluster-local",
  },
  "sandboxGroups.credValValueRequired": {
    message: "Secret value is required.",
    description: "Validation error when secret value is empty on create",
  },
  "sandboxGroups.credValValueTooLong": {
    message: "Secret value must be at most {maxBytes} bytes.",
    description: "Validation error when secret value exceeds 5 KiB",
  },
  "sandboxGroups.credValValueNul": {
    message: "Secret value cannot contain NUL bytes.",
    description: "Validation error when secret value contains a NUL byte",
  },
  "sandboxGroups.credValInjectionNameRequired": {
    message: "Target name is required for this injection type.",
    description: "Validation error when header/cookie/query name is missing",
  },
  "sandboxGroups.credValInjectionNameForbidden": {
    message: "Basic auth injection cannot specify a target name.",
    description: "Validation error when basic injection specifies a name",
  },
  "sandboxGroups.credValInjectionHeaderForbidden": {
    message: "Routing and connection headers cannot be injected.",
    description: "Validation error when a forbidden HTTP header is chosen",
  },
  "sandboxGroups.credValInjectionNameInvalid": {
    message: "Enter a valid target name.",
    description: "Validation error when target name has invalid characters",
  },
  "sandboxGroups.credValInjectionTemplateCount": {
    message: "Template must include between 1 and {max} occurrences of {token}.",
    description: "Validation error when template placeholder count is out of range",
  },
  "sandboxGroups.credValInjectionBasicFormat": {
    message: "Basic auth rendered value must have the form username:password.",
    description: "Validation error when basic auth rendered value lacks username:password",
  },
  "sandboxGroups.credValInjectionHeaderControl": {
    message: "Rendered header value cannot contain line breaks or NUL bytes.",
    description: "Validation error when header value contains CR/LF/NUL",
  },
  "sandboxGroups.credValInjectionCookieInvalid": {
    message: "Rendered cookie value cannot contain semicolons or line breaks.",
    description: "Validation error when cookie value contains semicolons or CR/LF",
  },
  "sandboxGroups.credValPathsInvalid": {
    message:
      "Each path must start with /, omit ? or #, and use whole-segment * or ** wildcards.",
    description: "Validation error when a path pattern is invalid",
  },
};

export default enSandboxGroups;
