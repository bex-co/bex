import type { TranslationEntry } from "@/i18n";

const es: Record<string, TranslationEntry> = {
  "activeSessions.title": {
    message: "Sesiones activas",
    description: "Active Sessions settings card title",
  },
  "activeSessions.description": {
    message:
      "Navegadores y dispositivos que tienen la sesión iniciada con tu cuenta.",
    description: "Active Sessions settings card description",
  },
  "activeSessions.signOutOthers": {
    message: "Cerrar las demás sesiones",
    description: "Active Sessions sign-out-others button label",
  },
  "activeSessions.signOutOthersConfirmTitle": {
    message: "¿Cerrar todas las demás sesiones?",
    description: "Active Sessions sign-out-others confirmation dialog title",
  },
  "activeSessions.signOutOthersConfirmBody": {
    message:
      "Esto cierra de inmediato la sesión de todos los navegadores y dispositivos excepto el que estás usando ahora.",
    description: "Active Sessions sign-out-others confirmation dialog body",
  },
  "activeSessions.signOutOthersSuccess": {
    message: "Se cerraron las demás sesiones",
    description: "Active Sessions sign-out-others success toast",
  },
  "activeSessions.signOutOthersError": {
    message: "No se pudieron cerrar las demás sesiones",
    description: "Active Sessions sign-out-others failure toast",
  },
  "activeSessions.colDevice": {
    message: "Dispositivo",
    description: "Active Sessions table column: device/browser",
  },
  "activeSessions.colLocation": {
    message: "Ubicación",
    description: "Active Sessions table column: location or IP",
  },
  "activeSessions.colSignedIn": {
    message: "Sesión iniciada",
    description:
      "Active Sessions table column: when the session was authenticated (Kratos has no last-activity time)",
  },
  "activeSessions.current": {
    message: "Este dispositivo",
    description: "Active Sessions badge marking the current session's row",
  },
  "activeSessions.unknownDevice": {
    message: "Dispositivo desconocido",
    description: "Active Sessions fallback when no user agent is recorded",
  },
  "activeSessions.emptyTitle": {
    message: "No hay sesiones activas",
    description: "Active Sessions empty state title",
  },
  "activeSessions.emptyBody": {
    message: "No encontramos ninguna sesión activa de tu cuenta.",
    description: "Active Sessions empty state body",
  },
  "activeSessions.errorTitle": {
    message: "No se pudieron cargar las sesiones activas",
    description: "Active Sessions generic error state title",
  },
  "activeSessions.errorBody": {
    message: "Algo salió mal. Inténtalo de nuevo en un momento.",
    description: "Active Sessions generic error state body",
  },
  "activeSessions.revoke": {
    message: "Cerrar sesión",
    description: "Active Sessions row revoke confirmation button",
  },
  "activeSessions.revokeNamed": {
    message: "Cerrar la sesión de {device}, {location}, iniciada {signedIn}",
    description:
      "Active Sessions row revoke button aria-label identifying the session",
  },
  "activeSessions.revokeConfirmTitle": {
    message: "¿Cerrar esta sesión?",
    description: "Active Sessions revoke confirmation dialog title",
  },
  "activeSessions.revokeConfirmBody": {
    message:
      "Esto cierra de inmediato la sesión de ese navegador o dispositivo.",
    description: "Active Sessions revoke confirmation dialog body",
  },
  "activeSessions.revokeCancel": {
    message: "Cancelar",
    description: "Active Sessions confirmation dialog cancel button",
  },
  "activeSessions.revokeSuccess": {
    message: "Sesión cerrada",
    description: "Active Sessions revoke success toast",
  },
  "activeSessions.revokeError": {
    message: "No se pudo cerrar esa sesión",
    description: "Active Sessions revoke failure toast",
  },
};

export default es;
