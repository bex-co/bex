import type { TranslationEntry } from "@/i18n";

const enApiKeys: Record<string, TranslationEntry> = {
  "apiKeys.title": {
    message: "API Keys",
    description: "Settings API Keys section card title",
  },
  "apiKeys.description": {
    message:
      "Machine credentials for scripts and agents. Shared across the workspace — anyone who can manage keys sees every key here, not just their own.",
    description: "Settings API Keys section card description",
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
      "Send it as Authorization: Bearer on every request. The key acts with full authority in this workspace, including connection strings, so treat it like a password.",
    description:
      "Mint dialog copy under the key: how to send it, and that it carries full authority in the workspace",
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
};

export default enApiKeys;
