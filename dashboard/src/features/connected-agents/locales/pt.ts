import type { TranslationEntry } from "@/i18n";

const pt: Record<string, TranslationEntry> = {
  "connectedAgents.title": {
    message: "Agentes conectados",
    description: "Connected Agents settings card title",
  },
  "connectedAgents.description": {
    message:
      "Clientes OAuth que você autorizou a agir em seu nome. Revogar um deles invalida imediatamente os tokens de acesso dele.",
    description: "Connected Agents settings card description",
  },
  "connectedAgents.colClient": {
    message: "Cliente",
    description: "Connected Agents table column: client name",
  },
  "connectedAgents.colScopes": {
    message: "Escopos",
    description: "Connected Agents table column: granted scopes",
  },
  "connectedAgents.colGranted": {
    message: "Concedido em",
    description: "Connected Agents table column: grant date",
  },
  "connectedAgents.emptyTitle": {
    message: "Nenhum agente conectado",
    description: "Connected Agents empty state title",
  },
  "connectedAgents.emptyBody": {
    message: "Você ainda não autorizou nenhum cliente OAuth.",
    description: "Connected Agents empty state body",
  },
  "connectedAgents.errorTitle": {
    message: "Não foi possível carregar os agentes conectados",
    description: "Connected Agents generic error state title",
  },
  "connectedAgents.errorBody": {
    message: "Algo deu errado. Tente novamente em instantes.",
    description: "Connected Agents generic error state body",
  },
  "connectedAgents.revoke": {
    message: "Revogar",
    description: "Connected Agents row revoke confirmation button",
  },
  "connectedAgents.revokeNamed": {
    message: "Revogar o agente conectado {name}",
    description:
      "Connected Agents row revoke button aria-label identifying the client",
  },
  "connectedAgents.revokeConfirmTitle": {
    message: "Revogar o acesso de “{name}”?",
    description: "Connected Agents revoke confirmation dialog title",
  },
  "connectedAgents.revokeConfirmBody": {
    message:
      "Isso invalida imediatamente todos os tokens de acesso deste cliente. Ele precisará ser autorizado novamente para agir como você.",
    description: "Connected Agents revoke confirmation dialog body",
  },
  "connectedAgents.revokeCancel": {
    message: "Cancelar",
    description: "Connected Agents revoke confirmation dialog cancel button",
  },
  "connectedAgents.revokeSuccess": {
    message: "Acesso de “{name}” revogado",
    description: "Connected Agents revoke success toast",
  },
  "connectedAgents.revokeError": {
    message: "Não foi possível revogar o acesso de “{name}”",
    description: "Connected Agents revoke failure toast",
  },
};

export default pt;
