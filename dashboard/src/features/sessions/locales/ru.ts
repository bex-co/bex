import type { TranslationEntry } from "@/i18n";

const ru: Record<string, TranslationEntry> = {
  "activeSessions.title": {
    message: "Активные сессии",
    description: "Active Sessions settings card title",
  },
  "activeSessions.description": {
    message:
      "Браузеры и устройства, в которых сейчас выполнен вход под вашей учётной записью.",
    description: "Active Sessions settings card description",
  },
  "activeSessions.signOutOthers": {
    message: "Завершить другие сессии",
    description: "Active Sessions sign-out-others button label",
  },
  "activeSessions.signOutOthersConfirmTitle": {
    message: "Завершить все остальные сессии?",
    description: "Active Sessions sign-out-others confirmation dialog title",
  },
  "activeSessions.signOutOthersConfirmBody": {
    message:
      "Это немедленно завершит сессии во всех браузерах и на всех устройствах, кроме того, которым вы пользуетесь сейчас.",
    description: "Active Sessions sign-out-others confirmation dialog body",
  },
  "activeSessions.signOutOthersSuccess": {
    message: "Другие сессии завершены",
    description: "Active Sessions sign-out-others success toast",
  },
  "activeSessions.signOutOthersError": {
    message: "Не удалось завершить другие сессии",
    description: "Active Sessions sign-out-others failure toast",
  },
  "activeSessions.colDevice": {
    message: "Устройство",
    description: "Active Sessions table column: device/browser",
  },
  "activeSessions.colLocation": {
    message: "Местоположение",
    description: "Active Sessions table column: location or IP",
  },
  "activeSessions.colSignedIn": {
    message: "Вход выполнен",
    description:
      "Active Sessions table column: when the session was authenticated (Kratos has no last-activity time)",
  },
  "activeSessions.current": {
    message: "Это устройство",
    description: "Active Sessions badge marking the current session's row",
  },
  "activeSessions.unknownDevice": {
    message: "Неизвестное устройство",
    description: "Active Sessions fallback when no user agent is recorded",
  },
  "activeSessions.emptyTitle": {
    message: "Нет активных сессий",
    description: "Active Sessions empty state title",
  },
  "activeSessions.emptyBody": {
    message: "Не удалось найти активные сессии для вашей учётной записи.",
    description: "Active Sessions empty state body",
  },
  "activeSessions.errorTitle": {
    message: "Не удалось загрузить активные сессии",
    description: "Active Sessions generic error state title",
  },
  "activeSessions.errorBody": {
    message: "Что-то пошло не так. Повторите попытку чуть позже.",
    description: "Active Sessions generic error state body",
  },
  "activeSessions.revoke": {
    message: "Выйти",
    description: "Active Sessions row revoke confirmation button",
  },
  "activeSessions.revokeNamed": {
    message: "Завершить сессию: {device}, {location}, вход выполнен {signedIn}",
    description:
      "Active Sessions row revoke button aria-label identifying the session",
  },
  "activeSessions.revokeConfirmTitle": {
    message: "Завершить эту сессию?",
    description: "Active Sessions revoke confirmation dialog title",
  },
  "activeSessions.revokeConfirmBody": {
    message:
      "Это немедленно завершит сессию в этом браузере или на этом устройстве.",
    description: "Active Sessions revoke confirmation dialog body",
  },
  "activeSessions.revokeCancel": {
    message: "Отмена",
    description: "Active Sessions confirmation dialog cancel button",
  },
  "activeSessions.revokeSuccess": {
    message: "Сессия завершена",
    description: "Active Sessions revoke success toast",
  },
  "activeSessions.revokeError": {
    message: "Не удалось завершить эту сессию",
    description: "Active Sessions revoke failure toast",
  },
};

export default ru;
