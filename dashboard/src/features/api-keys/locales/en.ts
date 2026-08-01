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
    description:
      "API Keys table column header — when a token for the key was last used",
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
    description: "Mint dialog title (secret-shown step)",
  },
  "apiKeys.createdWarning": {
    message: "Copy this key now — you won't be able to see it again.",
    description: "Mint dialog warning (secret-shown step)",
  },
  "apiKeys.createdDone": {
    message: "Done",
    description: "Mint dialog close button (secret-shown step)",
  },
  "apiKeys.copy": {
    message: "Copy",
    description: "Copy-to-clipboard icon button label",
  },
  "apiKeys.copied": {
    message: "Copied to clipboard",
    description: "Toast on a successful secret copy",
  },
  "apiKeys.copyError": {
    message: "Couldn't copy to clipboard",
    description: "Toast on a failed secret copy",
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
