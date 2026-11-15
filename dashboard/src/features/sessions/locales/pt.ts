import type { TranslationEntry } from "@/i18n";

const pt: Record<string, TranslationEntry> = {
  "activeSessions.title": {
    message: "Sessões ativas",
    description: "Active Sessions settings card title",
  },
  "activeSessions.description": {
    message:
      "Navegadores e dispositivos que estão conectados como você no momento.",
    description: "Active Sessions settings card description",
  },
  "activeSessions.signOutOthers": {
    message: "Sair das outras sessões",
    description: "Active Sessions sign-out-others button label",
  },
  "activeSessions.signOutOthersConfirmTitle": {
    message: "Sair de todas as outras sessões?",
    description: "Active Sessions sign-out-others confirmation dialog title",
  },
  "activeSessions.signOutOthersConfirmBody": {
    message:
      "Isso desconecta imediatamente todos os navegadores e dispositivos, exceto o que você está usando agora.",
    description: "Active Sessions sign-out-others confirmation dialog body",
  },
  "activeSessions.signOutOthersSuccess": {
    message: "Outras sessões encerradas",
    description: "Active Sessions sign-out-others success toast",
  },
  "activeSessions.signOutOthersError": {
    message: "Não foi possível sair das outras sessões",
    description: "Active Sessions sign-out-others failure toast",
  },
  "activeSessions.colDevice": {
    message: "Dispositivo",
    description: "Active Sessions table column: device/browser",
  },
  "activeSessions.colLocation": {
    message: "Local",
    description: "Active Sessions table column: location or IP",
  },
  "activeSessions.colSignedIn": {
    message: "Conectado",
    description:
      "Active Sessions table column: when the session was authenticated (Kratos has no last-activity time)",
  },
  "activeSessions.current": {
    message: "Este dispositivo",
    description: "Active Sessions badge marking the current session's row",
  },
  "activeSessions.unknownDevice": {
    message: "Dispositivo desconhecido",
    description: "Active Sessions fallback when no user agent is recorded",
  },
  "activeSessions.emptyTitle": {
    message: "Nenhuma sessão ativa",
    description: "Active Sessions empty state title",
  },
  "activeSessions.emptyBody": {
    message: "Não encontramos nenhuma sessão ativa para a sua conta.",
    description: "Active Sessions empty state body",
  },
  "activeSessions.errorTitle": {
    message: "Não foi possível carregar as sessões ativas",
    description: "Active Sessions generic error state title",
  },
  "activeSessions.errorBody": {
    message: "Algo deu errado. Tente novamente em instantes.",
    description: "Active Sessions generic error state body",
  },
  "activeSessions.revoke": {
    message: "Sair",
    description: "Active Sessions row revoke confirmation button",
  },
  "activeSessions.revokeNamed": {
    message: "Sair de {device}, {location}, conectado {signedIn}",
    description:
      "Active Sessions row revoke button aria-label identifying the session",
  },
  "activeSessions.revokeConfirmTitle": {
    message: "Sair desta sessão?",
    description: "Active Sessions revoke confirmation dialog title",
  },
  "activeSessions.revokeConfirmBody": {
    message: "Isso desconecta imediatamente esse navegador ou dispositivo.",
    description: "Active Sessions revoke confirmation dialog body",
  },
  "activeSessions.revokeCancel": {
    message: "Cancelar",
    description: "Active Sessions confirmation dialog cancel button",
  },
  "activeSessions.revokeSuccess": {
    message: "Sessão encerrada",
    description: "Active Sessions revoke success toast",
  },
  "activeSessions.revokeError": {
    message: "Não foi possível encerrar essa sessão",
    description: "Active Sessions revoke failure toast",
  },
};

export default pt;
