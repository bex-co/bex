import type { TranslationEntry } from "@/i18n";

const ruApiKeys: Record<string, TranslationEntry> = {
  "apiKeys.title": {
    message: "API-ключи",
    description: "Settings API Keys section card title",
  },
  "apiKeys.description": {
    message:
      "Машинные учётные данные для скриптов и агентов. Общие для всего рабочего пространства — любой, кто может управлять ключами, видит здесь все ключи, а не только свои.",
    description: "Settings API Keys section card description",
  },
  "apiKeys.cliGuide": {
    message: "Настройте CLI.",
    description: "Link from the API Keys card to the bex CLI setup guide",
  },
  "apiKeys.colName": {
    message: "Название",
    description: "API Keys table column header",
  },
  "apiKeys.colClientId": {
    message: "ID клиента",
    description:
      "API Keys table column header: the key's id, which is also its OAuth2 client_id",
  },
  "apiKeys.colCreated": {
    message: "Создан",
    description: "API Keys table column header",
  },
  "apiKeys.colCreatedBy": {
    message: "Создал(а)",
    description: "API Keys table column header — who minted the key",
  },
  "apiKeys.colLastUsed": {
    message: "Последнее использование",
    description: "API Keys table column header: when the key was last used",
  },
  "apiKeys.neverUsed": {
    message: "Никогда",
    description: "API Keys last-used cell when the key has never been used",
  },
  "apiKeys.emptyTitle": {
    message: "Нет API-ключей",
    description: "API Keys empty-state title",
  },
  "apiKeys.emptyBody": {
    message: "Создайте ключ для аутентификации скрипта или агента.",
    description: "API Keys empty-state body",
  },
  "apiKeys.forbiddenTitle": {
    message: "Нет доступа",
    description: "API Keys state when the caller lacks permission (403)",
  },
  "apiKeys.forbiddenBody": {
    message:
      "У вас нет прав на управление API-ключами этого рабочего пространства.",
    description: "API Keys forbidden-state body",
  },
  "apiKeys.errorTitle": {
    message: "Не удалось загрузить API-ключи",
    description: "API Keys generic error title",
  },
  "apiKeys.errorBody": {
    message: "Что-то пошло не так. Повторите попытку.",
    description: "API Keys generic error body",
  },
  "apiKeys.create": {
    message: "Создать API-ключ",
    description: "Button that opens the mint dialog",
  },
  "apiKeys.createTitle": {
    message: "Создание API-ключа",
    description: "Mint dialog title (name step)",
  },
  "apiKeys.createDescription": {
    message: "Назовите ключ, чтобы потом его узнать.",
    description: "Mint dialog description (name step)",
  },
  "apiKeys.fieldName": {
    message: "Название",
    description: "Mint dialog name field label",
  },
  "apiKeys.fieldNamePlaceholder": {
    message: "напр. deploy-agent",
    description: "Mint dialog name field placeholder",
  },
  "apiKeys.createCancel": {
    message: "Отменить",
    description: "Mint dialog cancel button (name step)",
  },
  "apiKeys.createSubmit": {
    message: "Создать",
    description: "Mint dialog submit button (name step)",
  },
  "apiKeys.createdTitle": {
    message: "API-ключ создан",
    description: "Mint dialog title (key-shown step)",
  },
  "apiKeys.createdWarning": {
    message: "Скопируйте ключ сейчас. Позже увидеть его будет нельзя.",
    description:
      "Mint dialog warning (key-shown step): the key is shown only once",
  },
  "apiKeys.fieldKey": {
    message: "API-ключ",
    description:
      "Mint dialog label for the key itself, the one string sent as Authorization: Bearer",
  },
  "apiKeys.copyKey": {
    message: "Копировать API-ключ",
    description: "Copy button aria-label for the key",
  },
  "apiKeys.keyHelp": {
    message:
      "Передавайте его в каждом запросе как Authorization: Bearer. Ключ действует с полными правами в этом рабочем пространстве, включая строки подключения, поэтому обращайтесь с ним как с паролем.",
    description:
      "Mint dialog copy under the key: how to send it, and that it carries full authority in the workspace",
  },
  "apiKeys.curlTitle": {
    message: "Пример запроса",
    description:
      "Mint dialog label for the curl example that lists services with the key",
  },
  "apiKeys.copyCurl": {
    message: "Копировать пример запроса",
    description: "Copy button aria-label for the curl example",
  },
  "apiKeys.oauthDetails": {
    message: "Учётные данные клиента OAuth",
    description:
      "Mint dialog title of the collapsed detail holding the client ID, the client secret and the optional token exchange",
  },
  "apiKeys.fieldClientId": {
    message: "ID клиента",
    description:
      "Mint dialog label for the OAuth2 client_id, inside the OAuth client credentials detail",
  },
  "apiKeys.fieldSecret": {
    message: "Секрет клиента",
    description:
      "Mint dialog label for the one-time client_secret, inside the OAuth client credentials detail",
  },
  "apiKeys.copyClientId": {
    message: "Копировать ID клиента",
    description: "Copy button aria-label for the client_id",
  },
  "apiKeys.copyClientIdNamed": {
    message: "Копировать ID клиента для {name}",
    description: "Copy button aria-label for one API client row's client_id",
  },
  "apiKeys.copySecret": {
    message: "Копировать секрет клиента",
    description: "Copy button aria-label for the client_secret",
  },
  "apiKeys.exchangeHelp": {
    message:
      "Этот ключ работает и как клиент OAuth2. Если вы предпочитаете краткосрочные токены, обменяйте ID клиента и секрет клиента на токен доступа по адресу {tokenEndpoint}:\n\ngrant_type=client_credentials&client_id=…&client_secret=…\n\nНе передавайте scope. Используйте полученный access_token как Authorization: Bearer вместо ключа.",
    description:
      "Mint dialog copy inside the OAuth client credentials detail: the optional client_credentials exchange (token endpoint from config)",
  },
  "apiKeys.createdDone": {
    message: "Готово",
    description: "Mint dialog close button (key-shown step)",
  },
  "apiKeys.copied": {
    message: "Скопировано в буфер обмена",
    description:
      "Toast on a successful copy (the key, the example request, the client ID or the secret)",
  },
  "apiKeys.copyError": {
    message: "Не удалось скопировать в буфер обмена",
    description: "Toast on a failed copy",
  },
  "apiKeys.createSuccess": {
    message: "{name} создан",
    description: "Toast on a successful mint",
  },
  "apiKeys.createError": {
    message: "Не удалось создать {name}",
    description: "Toast on a failed mint",
  },
  "apiKeys.revoke": {
    message: "Отозвать",
    description: "Row action / confirmation button to revoke a key",
  },
  "apiKeys.revokeNamed": {
    message: "Отозвать API-ключ {name}",
    description: "Row revoke button aria-label identifying the API key",
  },
  "apiKeys.revokeConfirmTitle": {
    message: "Отозвать {name}?",
    description: "Revoke-confirmation dialog title",
  },
  "apiKeys.revokeConfirmBody": {
    message:
      "Всё, что аутентифицируется с этим ключом, немедленно перестанет работать. Это действие нельзя отменить.",
    description: "Revoke-confirmation dialog body",
  },
  "apiKeys.revokeCancel": {
    message: "Отменить",
    description: "Revoke-confirmation dialog cancel button",
  },
  "apiKeys.revokeSuccess": {
    message: "{name} отозван",
    description: "Toast on a successful revoke",
  },
  "apiKeys.revokeError": {
    message: "Не удалось отозвать {name}",
    description: "Toast on a failed revoke",
  },
};

export default ruApiKeys;
