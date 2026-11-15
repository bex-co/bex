import type { TranslationEntry } from "@/i18n";

const ru: Record<string, TranslationEntry> = {
  "connectedAgents.title": {
    message: "Подключённые агенты",
    description: "Connected Agents settings card title",
  },
  "connectedAgents.description": {
    message:
      "Клиенты OAuth, которым вы разрешили действовать от вашего имени. Отзыв сразу делает недействительными их токены доступа.",
    description: "Connected Agents settings card description",
  },
  "connectedAgents.colClient": {
    message: "Клиент",
    description: "Connected Agents table column: client name",
  },
  "connectedAgents.colScopes": {
    message: "Права",
    description: "Connected Agents table column: granted scopes",
  },
  "connectedAgents.colGranted": {
    message: "Предоставлено",
    description: "Connected Agents table column: grant date",
  },
  "connectedAgents.emptyTitle": {
    message: "Нет подключённых агентов",
    description: "Connected Agents empty state title",
  },
  "connectedAgents.emptyBody": {
    message: "Вы пока не авторизовали ни одного клиента OAuth.",
    description: "Connected Agents empty state body",
  },
  "connectedAgents.errorTitle": {
    message: "Не удалось загрузить подключённых агентов",
    description: "Connected Agents generic error state title",
  },
  "connectedAgents.errorBody": {
    message: "Что-то пошло не так. Повторите попытку чуть позже.",
    description: "Connected Agents generic error state body",
  },
  "connectedAgents.revoke": {
    message: "Отозвать",
    description: "Connected Agents row revoke confirmation button",
  },
  "connectedAgents.revokeNamed": {
    message: "Отозвать подключённого агента {name}",
    description:
      "Connected Agents row revoke button aria-label identifying the client",
  },
  "connectedAgents.revokeConfirmTitle": {
    message: "Отозвать доступ для «{name}»?",
    description: "Connected Agents revoke confirmation dialog title",
  },
  "connectedAgents.revokeConfirmBody": {
    message:
      "Все токены доступа этого клиента сразу станут недействительными. Чтобы он снова мог действовать от вашего имени, его нужно будет авторизовать заново.",
    description: "Connected Agents revoke confirmation dialog body",
  },
  "connectedAgents.revokeCancel": {
    message: "Отменить",
    description: "Connected Agents revoke confirmation dialog cancel button",
  },
  "connectedAgents.revokeSuccess": {
    message: "Доступ для «{name}» отозван",
    description: "Connected Agents revoke success toast",
  },
  "connectedAgents.revokeError": {
    message: "Не удалось отозвать доступ для «{name}»",
    description: "Connected Agents revoke failure toast",
  },
};

export default ru;
