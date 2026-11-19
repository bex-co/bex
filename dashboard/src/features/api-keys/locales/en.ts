import type { TranslationEntry } from "@/i18n";

const enApiKeys: Record<string, TranslationEntry> = {
  "apiKeys.title": {
    message: "API Keys",
    description: "Settings API Keys section card title",
  },
  "apiKeys.description": {
    message:
      "Machine credentials for scripts and agents. Each key can use only the permissions chosen for it, never more than its creator's current member role allows, and only in this workspace. Shared across the workspace — anyone who can manage keys sees every key here, not just their own.",
    description:
      "Settings API Keys section card description: what a key is, that it uses only its chosen permissions within its creator's current role in this workspace (w9/m496), and that the list is shared",
  },
  "apiKeys.cliGuide": {
    message: "Set up the CLI.",
    description: "Link from the API Keys card to the bex CLI setup guide",
  },
  "apiKeys.colName": {
    message: "Name",
    description: "API Keys table column header",
  },
  "apiKeys.colClientId": {
    message: "Client ID",
    description:
      "API Keys table column header: the key's id, which is also its OAuth2 client_id",
  },
  "apiKeys.colCreated": {
    message: "Created",
    description: "API Keys table column header",
  },
  "apiKeys.colCreatedBy": {
    message: "Created by",
    description: "API Keys table column header — who minted the key",
  },
  "apiKeys.colLastUsed": {
    message: "Last used",
    description: "API Keys table column header: when the key was last used",
  },
  "apiKeys.neverUsed": {
    message: "Never",
    description: "API Keys last-used cell when the key has never been used",
  },
  "apiKeys.emptyTitle": {
    message: "No API keys",
    description: "API Keys empty-state title",
  },
  "apiKeys.emptyBody": {
    message: "Create a key to authenticate a script or agent.",
    description: "API Keys empty-state body",
  },
  "apiKeys.forbiddenTitle": {
    message: "Not authorized",
    description: "API Keys state when the caller lacks permission (403)",
  },
  "apiKeys.forbiddenBody": {
    message: "You don't have permission to manage this workspace's API keys.",
    description: "API Keys forbidden-state body",
  },
  "apiKeys.errorTitle": {
    message: "Couldn't load API keys",
    description: "API Keys generic error title",
  },
  "apiKeys.errorBody": {
    message: "Something went wrong. Please try again.",
    description: "API Keys generic error body",
  },
  "apiKeys.create": {
    message: "Create API key",
    description: "Button that opens the mint dialog",
  },
  "apiKeys.createTitle": {
    message: "Create API key",
    description: "Mint dialog title (name step)",
  },
  "apiKeys.createDescription": {
    message: "Name this key so you can recognize it later.",
    description: "Mint dialog description (name step)",
  },
  "apiKeys.fieldName": {
    message: "Name",
    description: "Mint dialog name field label",
  },
  "apiKeys.fieldNamePlaceholder": {
    message: "e.g. deploy-agent",
    description: "Mint dialog name field placeholder",
  },
  "apiKeys.createCancel": {
    message: "Cancel",
    description: "Mint dialog cancel button (name step)",
  },
  "apiKeys.createSubmit": {
    message: "Create",
    description: "Mint dialog submit button (name step)",
  },
  "apiKeys.createdTitle": {
    message: "API key created",
    description: "Mint dialog title (key-shown step)",
  },
  "apiKeys.createdWarning": {
    message: "Copy the key now. You won't be able to see it again.",
    description:
      "Mint dialog warning (key-shown step): the key is shown only once",
  },
  "apiKeys.fieldKey": {
    message: "API key",
    description:
      "Mint dialog label for the key itself, the one string sent as Authorization: Bearer",
  },
  "apiKeys.copyKey": {
    message: "Copy API key",
    description: "Copy button aria-label for the key",
  },
  "apiKeys.keyHelp": {
    message:
      "Send it as Authorization: Bearer on every request. The key can use only the permissions you chose for it, never more than your current member role in this workspace allows, so treat it like a password.",
    description:
      "Mint dialog copy under the key: how to send it, and that it uses only the chosen permissions, capped by the creator's current member role in this workspace (w9/m496)",
  },
  "apiKeys.curlTitle": {
    message: "Example request",
    description:
      "Mint dialog label for the curl example that lists services with the key",
  },
  "apiKeys.copyCurl": {
    message: "Copy example request",
    description: "Copy button aria-label for the curl example",
  },
  "apiKeys.oauthDetails": {
    message: "OAuth client credentials",
    description:
      "Mint dialog title of the collapsed detail holding the client ID, the client secret and the optional token exchange",
  },
  "apiKeys.fieldClientId": {
    message: "Client ID",
    description:
      "Mint dialog label for the OAuth2 client_id, inside the OAuth client credentials detail",
  },
  "apiKeys.fieldSecret": {
    message: "Client secret",
    description:
      "Mint dialog label for the one-time client_secret, inside the OAuth client credentials detail",
  },
  "apiKeys.copyClientId": {
    message: "Copy client ID",
    description: "Copy button aria-label for the client_id",
  },
  "apiKeys.copyClientIdNamed": {
    message: "Copy client ID for {name}",
    description: "Copy button aria-label for one API client row's client_id",
  },
  "apiKeys.copySecret": {
    message: "Copy client secret",
    description: "Copy button aria-label for the client_secret",
  },
  "apiKeys.exchangeHelp": {
    message:
      "This key also works as an OAuth2 client. If you prefer short-lived tokens, exchange the client ID and client secret for an access token at {tokenEndpoint}:\n\ngrant_type=client_credentials&client_id=…&client_secret=…\n\nDo not send a scope. Use the returned access_token as Authorization: Bearer in place of the key.",
    description:
      "Mint dialog copy inside the OAuth client credentials detail: the optional client_credentials exchange (token endpoint from config)",
  },
  "apiKeys.createdDone": {
    message: "Done",
    description: "Mint dialog close button (key-shown step)",
  },
  "apiKeys.copied": {
    message: "Copied to clipboard",
    description:
      "Toast on a successful copy (the key, the example request, the client ID or the secret)",
  },
  "apiKeys.copyError": {
    message: "Couldn't copy to clipboard",
    description: "Toast on a failed copy",
  },
  "apiKeys.createSuccess": {
    message: "Created {name}",
    description: "Toast on a successful mint",
  },
  "apiKeys.createError": {
    message: "Couldn't create {name}",
    description: "Toast on a failed mint",
  },
  "apiKeys.revoke": {
    message: "Revoke",
    description: "Row action / confirmation button to revoke a key",
  },
  "apiKeys.revokeNamed": {
    message: "Revoke API key {name}",
    description: "Row revoke button aria-label identifying the API key",
  },
  "apiKeys.revokeConfirmTitle": {
    message: "Revoke {name}?",
    description: "Revoke-confirmation dialog title",
  },
  "apiKeys.revokeConfirmBody": {
    message:
      "Anything authenticating with this key will stop working immediately. This can't be undone.",
    description: "Revoke-confirmation dialog body",
  },
  "apiKeys.revokeCancel": {
    message: "Cancel",
    description: "Revoke-confirmation dialog cancel button",
  },
  "apiKeys.revokeSuccess": {
    message: "Revoked {name}",
    description: "Toast on a successful revoke",
  },
  "apiKeys.revokeError": {
    message: "Couldn't revoke {name}",
    description: "Toast on a failed revoke",
  },
  "apiKeys.colPermissions": {
    message: "Permissions",
    description: "API Keys table column header: what each key may do (w9/m496)",
  },
  "apiKeys.permSummaryAll": {
    message: "All",
    description:
      "Key row permissions summary: the key has every permission its creator's member role allows (Render's All)",
  },
  "apiKeys.permSummaryReadOnly": {
    message: "Read only",
    description:
      "Key row permissions summary: every permission the key has is at Read, so it can change nothing",
  },
  "apiKeys.permSummaryNone": {
    message: "No permissions",
    description:
      "Key row permissions summary: the key has no permissions (Clear all)",
  },
  "apiKeys.permSummaryCustom": {
    message: "Custom",
    description:
      "Key row permissions summary: the key has a chosen set of permissions; followed by their count",
  },
  "apiKeys.editPermissionsNamed": {
    message: "Change permissions for {name}",
    description:
      "Gear button aria-label on a key row; opens the permissions editor",
  },
  "apiKeys.editPermissionsTitle": {
    message: "Edit permissions",
    description: "Permissions editor dialog title",
  },
  "apiKeys.editPermissionsDescription": {
    message:
      "Change what {name} can use, then click Save changes. The new permissions apply right away.",
    description:
      "Permissions editor dialog description; {name} is the key's name",
  },
  "apiKeys.editPermissionsCancel": {
    message: "Cancel",
    description: "Permissions editor dialog cancel button",
  },
  "apiKeys.editPermissionsSave": {
    message: "Save changes",
    description: "Permissions editor dialog save button (Render's words)",
  },
  "apiKeys.permissionsSaved": {
    message: "Updated permissions for {name}",
    description: "Toast after a key's permissions are saved",
  },
  "apiKeys.permissionsSaveError": {
    message: "Couldn't update permissions for {name}",
    description:
      "Toast when saving a key's permissions fails (the server's reason is shown instead when it gives one)",
  },
  "apiKeys.permIntro": {
    message: "Select the permissions this API key can use.",
    description:
      "Permission picker heading in the create dialog and the permissions editor (Render's words)",
  },
  "apiKeys.permPresetAll": {
    message: "All",
    description:
      "Picker preset button: grant every permission the member role allows (Render's All)",
  },
  "apiKeys.permPresetReadOnly": {
    message: "Read only",
    description:
      "Picker preset button: grant Read on every permission that offers it (Render's Read only)",
  },
  "apiKeys.permPresetClear": {
    message: "Clear all",
    description:
      "Picker preset button: remove every permission from the key (Render's Clear all)",
  },
  "apiKeys.permAllHelp": {
    message:
      "All grants every permission the creator's member role allows, for every resource, and follows that role if it changes.",
    description: "Picker note while the All preset is selected: what All means",
  },
  "apiKeys.permNoneHelp": {
    message: "No permissions selected. The key can only identify itself.",
    description: "Picker note when every permission is None (Clear all)",
  },
  "apiKeys.permAreaToggle": {
    message: "Grant {area} permissions",
    description:
      "Switch aria-label on one area of the picker; {area} is Render's area name, e.g. Services (kept in English)",
  },
  "apiKeys.permLevelNone": {
    message: "None",
    description: "Picker option on a permission: the key gets none of it",
  },
  "apiKeys.permLevelGrants": {
    message: "{level} grants:",
    description:
      "Picker label over the actions a permission's chosen level grants; {level} is Render's level name, e.g. Read & write (kept in English)",
  },
  "apiKeys.permRequiresRole.viewer": {
    message: "{level} requires the Viewer role.",
    description:
      "Reason beside a level the viewer's role cannot grant; {level} is Render's level name (kept in English); the lowest role that can is Viewer",
  },
  "apiKeys.permRequiresRole.contributor": {
    message: "{level} requires the Contributor role.",
    description:
      "Reason beside a level the viewer's role cannot grant; {level} is Render's level name (kept in English); the lowest role that can is Contributor",
  },
  "apiKeys.permRequiresRole.developer": {
    message: "{level} requires the Developer role.",
    description:
      "Reason beside a level the viewer's role cannot grant; {level} is Render's level name (kept in English); the lowest role that can is Developer",
  },
  "apiKeys.permRequiresRole.admin": {
    message: "{level} requires the Admin role.",
    description:
      "Reason beside a level the viewer's role cannot grant; {level} is Render's level name (kept in English); the lowest role that can is Admin",
  },
  "apiKeys.permRequiresHigherRole": {
    message: "{level} requires a higher member role.",
    description:
      "Reason beside a level the viewer's role cannot grant, when bex-api names a role the dashboard does not know",
  },
  "apiKeys.permCatalogLoading": {
    message: "Loading permissions…",
    description:
      "Accessible name of the picker while its permission list loads",
  },
  "apiKeys.permCatalogErrorCreate": {
    message:
      "Couldn't load the permission list. A key you create now gets All permissions.",
    description:
      "Create dialog note when the permission list fails to load: creating still works and grants All",
  },
  "apiKeys.permCatalogErrorEdit": {
    message: "Couldn't load the permission list.",
    description:
      "Permissions editor note when the permission list fails to load, beside a Try again button",
  },
  "apiKeys.permSummaryCount_one": {
    message: "{count} permission",
    description:
      "Key row permissions summary beside Custom: how many permissions the key has",
  },
  "apiKeys.permSummaryCount_other": {
    message: "{count} permissions",
    description:
      "Key row permissions summary beside Custom: how many permissions the key has",
  },
};

export default enApiKeys;
