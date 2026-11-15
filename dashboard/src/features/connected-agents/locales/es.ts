import type { TranslationEntry } from "@/i18n";

const es: Record<string, TranslationEntry> = {
  "connectedAgents.title": {
    message: "Agentes conectados",
    description: "Connected Agents settings card title",
  },
  "connectedAgents.description": {
    message:
      "Clientes OAuth que has autorizado para actuar en tu nombre. Al revocar uno, sus tokens de acceso dejan de ser válidos de inmediato.",
    description: "Connected Agents settings card description",
  },
  "connectedAgents.colClient": {
    message: "Cliente",
    description: "Connected Agents table column: client name",
  },
  "connectedAgents.colScopes": {
    message: "Scopes",
    description: "Connected Agents table column: granted scopes",
  },
  "connectedAgents.colGranted": {
    message: "Concedido",
    description: "Connected Agents table column: grant date",
  },
  "connectedAgents.emptyTitle": {
    message: "No hay agentes conectados",
    description: "Connected Agents empty state title",
  },
  "connectedAgents.emptyBody": {
    message: "Aún no has autorizado ningún cliente OAuth.",
    description: "Connected Agents empty state body",
  },
  "connectedAgents.errorTitle": {
    message: "No se pudieron cargar los agentes conectados",
    description: "Connected Agents generic error state title",
  },
  "connectedAgents.errorBody": {
    message: "Algo salió mal. Inténtalo de nuevo en un momento.",
    description: "Connected Agents generic error state body",
  },
  "connectedAgents.revoke": {
    message: "Revocar",
    description: "Connected Agents row revoke confirmation button",
  },
  "connectedAgents.revokeNamed": {
    message: "Revocar el agente conectado {name}",
    description:
      "Connected Agents row revoke button aria-label identifying the client",
  },
  "connectedAgents.revokeConfirmTitle": {
    message: "¿Revocar el acceso de “{name}”?",
    description: "Connected Agents revoke confirmation dialog title",
  },
  "connectedAgents.revokeConfirmBody": {
    message:
      "Esto invalida de inmediato todos los tokens de acceso de este cliente. Tendrá que volver a autorizarse para actuar en tu nombre.",
    description: "Connected Agents revoke confirmation dialog body",
  },
  "connectedAgents.revokeCancel": {
    message: "Cancelar",
    description: "Connected Agents revoke confirmation dialog cancel button",
  },
  "connectedAgents.revokeSuccess": {
    message: "Acceso de “{name}” revocado",
    description: "Connected Agents revoke success toast",
  },
  "connectedAgents.revokeError": {
    message: "No se pudo revocar el acceso de “{name}”",
    description: "Connected Agents revoke failure toast",
  },
};

export default es;
