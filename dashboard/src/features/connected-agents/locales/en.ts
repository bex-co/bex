import type { TranslationEntry } from "@/i18n";

const en: Record<string, TranslationEntry> = {
  "connectedAgents.title": {
    message: "Connected agents",
    description: "Connected Agents settings card title",
  },
  "connectedAgents.description": {
    message:
      "OAuth clients you've authorized to act on your behalf. Revoking one immediately invalidates its access tokens.",
    description: "Connected Agents settings card description",
  },
  "connectedAgents.colClient": {
    message: "Client",
    description: "Connected Agents table column: client name",
  },
  "connectedAgents.colScopes": {
    message: "Scopes",
    description: "Connected Agents table column: granted scopes",
  },
  "connectedAgents.colGranted": {
    message: "Granted",
    description: "Connected Agents table column: grant date",
  },
  "connectedAgents.emptyTitle": {
    message: "No connected agents",
    description: "Connected Agents empty state title",
  },
  "connectedAgents.emptyBody": {
    message: "You haven't authorized any OAuth clients yet.",
    description: "Connected Agents empty state body",
  },
  "connectedAgents.errorTitle": {
    message: "Couldn't load connected agents",
    description: "Connected Agents generic error state title",
  },
  "connectedAgents.errorBody": {
    message: "Something went wrong. Try again in a moment.",
    description: "Connected Agents generic error state body",
  },
  "connectedAgents.revoke": {
    message: "Revoke",
    description: "Connected Agents row revoke button label",
  },
  "connectedAgents.revokeConfirmTitle": {
    message: 'Revoke access for "{name}"?',
    description: "Connected Agents revoke confirmation dialog title",
  },
  "connectedAgents.revokeConfirmBody": {
    message:
      "This immediately invalidates every access token this client holds. It will need to be re-authorized to act as you again.",
    description: "Connected Agents revoke confirmation dialog body",
  },
  "connectedAgents.revokeCancel": {
    message: "Cancel",
    description: "Connected Agents revoke confirmation dialog cancel button",
  },
  "connectedAgents.revokeSuccess": {
    message: 'Revoked access for "{name}"',
    description: "Connected Agents revoke success toast",
  },
  "connectedAgents.revokeError": {
    message: 'Couldn\'t revoke access for "{name}"',
    description: "Connected Agents revoke failure toast",
  },
};

export default en;
