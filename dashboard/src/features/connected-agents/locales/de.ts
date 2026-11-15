import type { TranslationEntry } from "@/i18n";

const de: Record<string, TranslationEntry> = {
  "connectedAgents.title": {
    message: "Verbundene Agents",
    description: "Connected Agents settings card title",
  },
  "connectedAgents.description": {
    message:
      "OAuth-Clients, die du autorisiert hast, in deinem Namen zu handeln. Beim Widerrufen werden ihre Access-Tokens sofort ungültig.",
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
    message: "Erteilt",
    description: "Connected Agents table column: grant date",
  },
  "connectedAgents.emptyTitle": {
    message: "Keine verbundenen Agents",
    description: "Connected Agents empty state title",
  },
  "connectedAgents.emptyBody": {
    message: "Du hast noch keine OAuth-Clients autorisiert.",
    description: "Connected Agents empty state body",
  },
  "connectedAgents.errorTitle": {
    message: "Verbundene Agents konnten nicht geladen werden",
    description: "Connected Agents generic error state title",
  },
  "connectedAgents.errorBody": {
    message: "Etwas ist schiefgelaufen. Versuche es gleich noch einmal.",
    description: "Connected Agents generic error state body",
  },
  "connectedAgents.revoke": {
    message: "Widerrufen",
    description: "Connected Agents row revoke confirmation button",
  },
  "connectedAgents.revokeNamed": {
    message: "Verbundenen Agent {name} widerrufen",
    description:
      "Connected Agents row revoke button aria-label identifying the client",
  },
  "connectedAgents.revokeConfirmTitle": {
    message: "Zugriff für „{name}“ widerrufen?",
    description: "Connected Agents revoke confirmation dialog title",
  },
  "connectedAgents.revokeConfirmBody": {
    message:
      "Dadurch werden sofort alle Access-Tokens dieses Clients ungültig. Er muss erneut autorisiert werden, um wieder in deinem Namen zu handeln.",
    description: "Connected Agents revoke confirmation dialog body",
  },
  "connectedAgents.revokeCancel": {
    message: "Abbrechen",
    description: "Connected Agents revoke confirmation dialog cancel button",
  },
  "connectedAgents.revokeSuccess": {
    message: "Zugriff für „{name}“ widerrufen",
    description: "Connected Agents revoke success toast",
  },
  "connectedAgents.revokeError": {
    message: "Zugriff für „{name}“ konnte nicht widerrufen werden",
    description: "Connected Agents revoke failure toast",
  },
};

export default de;
