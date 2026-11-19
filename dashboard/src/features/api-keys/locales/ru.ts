import type { TranslationEntry } from "@/i18n";

const ruApiKeys: Record<string, TranslationEntry> = {
  "apiKeys.title": {
    message: "API-ключи",
    description: "Settings API Keys section card title",
  },
  "apiKeys.description": {
    message:
      "Машинные учётные данные для скриптов и агентов. Каждый ключ может использовать только выбранные для него разрешения, не больше, чем допускает текущая роль участника, который его создал, и только в этом рабочем пространстве. Каждый участник видит созданные им ключи и управляет ими; Developer и Admin видят все ключи рабочего пространства.",
    description:
      "Settings API Keys section card description: what a key is, that it uses only its chosen permissions within its creator's current role in this workspace (w9/m496), and who sees which keys: every member their own, Developers and Admins every key (w9/m514)",
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
      "Передавайте его в каждом запросе как Authorization: Bearer. Ключ может использовать только выбранные вами разрешения, не больше, чем допускает ваша текущая роль участника в этом рабочем пространстве, поэтому обращайтесь с ним как с паролем.",
    description:
      "Mint dialog copy under the key: how to send it, and that it uses only the chosen permissions, capped by the creator's current member role in this workspace (w9/m496)",
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
  "apiKeys.colPermissions": {
    message: "Разрешения",
    description: "API Keys table column header: what each key may do (w9/m496)",
  },
  "apiKeys.permSummaryAll": {
    message: "Все",
    description:
      "Key row permissions summary: the key has every permission its creator's member role allows (Render's All)",
  },
  "apiKeys.permSummaryReadOnly": {
    message: "Только чтение",
    description:
      "Key row permissions summary: every permission the key has is at Read, so it can change nothing",
  },
  "apiKeys.permSummaryNone": {
    message: "Нет разрешений",
    description:
      "Key row permissions summary: the key has no permissions (Clear all)",
  },
  "apiKeys.permSummaryCustom": {
    message: "Выборочные",
    description:
      "Key row permissions summary: the key has a chosen set of permissions; followed by their count",
  },
  "apiKeys.editPermissionsNamed": {
    message: "Изменить разрешения ключа {name}",
    description:
      "Gear button aria-label on a key row; opens the permissions editor",
  },
  "apiKeys.editPermissionsTitle": {
    message: "Изменение разрешений",
    description: "Permissions editor dialog title",
  },
  "apiKeys.editPermissionsDescription": {
    message:
      "Измените, что может использовать {name}, и нажмите «Сохранить изменения». Новые разрешения действуют сразу.",
    description:
      "Permissions editor dialog description; {name} is the key's name",
  },
  "apiKeys.editPermissionsCancel": {
    message: "Отменить",
    description: "Permissions editor dialog cancel button",
  },
  "apiKeys.editPermissionsSave": {
    message: "Сохранить изменения",
    description: "Permissions editor dialog save button (Render's words)",
  },
  "apiKeys.permissionsSaved": {
    message: "Разрешения ключа {name} обновлены",
    description: "Toast after a key's permissions are saved",
  },
  "apiKeys.permissionsSaveError": {
    message: "Не удалось обновить разрешения ключа {name}",
    description:
      "Toast when saving a key's permissions fails (the server's reason is shown instead when it gives one)",
  },
  "apiKeys.permIntro": {
    message: "Выберите разрешения, которые может использовать этот API-ключ.",
    description:
      "Permission picker heading in the create dialog and the permissions editor (Render's words)",
  },
  "apiKeys.permPresetAll": {
    message: "Все",
    description:
      "Picker preset button: grant every permission the member role allows (Render's All)",
  },
  "apiKeys.permPresetReadOnly": {
    message: "Только чтение",
    description:
      "Picker preset button: grant Read on every permission that offers it (Render's Read only)",
  },
  "apiKeys.permPresetClear": {
    message: "Снять все",
    description:
      "Picker preset button: remove every permission from the key (Render's Clear all)",
  },
  "apiKeys.permAllHelp": {
    message:
      "«Все» даёт каждое разрешение, которое допускает роль участника, создавшего ключ, для всех ресурсов, и меняется вместе с этой ролью.",
    description: "Picker note while the All preset is selected: what All means",
  },
  "apiKeys.permNoneHelp": {
    message: "Разрешения не выбраны. Ключ может только идентифицировать себя.",
    description: "Picker note when every permission is None (Clear all)",
  },
  "apiKeys.permAreaToggle": {
    message: "Выдать разрешения раздела {area}",
    description:
      "Switch aria-label on one area of the picker; {area} is Render's area name, e.g. Services (kept in English)",
  },
  "apiKeys.permLevelNone": {
    message: "Нет",
    description: "Picker option on a permission: the key gets none of it",
  },
  "apiKeys.permLevelGrants": {
    message: "{level} даёт:",
    description:
      "Picker label over the actions a permission's chosen level grants; {level} is Render's level name, e.g. Read & write (kept in English)",
  },
  "apiKeys.permRequiresRole.viewer": {
    message: "Для уровня {level} нужна роль «Наблюдатель».",
    description:
      "Reason beside a level the viewer's role cannot grant; {level} is Render's level name (kept in English); the lowest role that can is Viewer",
  },
  "apiKeys.permRequiresRole.contributor": {
    message: "Для уровня {level} нужна роль «Участник-исполнитель».",
    description:
      "Reason beside a level the viewer's role cannot grant; {level} is Render's level name (kept in English); the lowest role that can is Contributor",
  },
  "apiKeys.permRequiresRole.developer": {
    message: "Для уровня {level} нужна роль «Разработчик».",
    description:
      "Reason beside a level the viewer's role cannot grant; {level} is Render's level name (kept in English); the lowest role that can is Developer",
  },
  "apiKeys.permRequiresRole.admin": {
    message: "Для уровня {level} нужна роль «Администратор».",
    description:
      "Reason beside a level the viewer's role cannot grant; {level} is Render's level name (kept in English); the lowest role that can is Admin",
  },
  "apiKeys.permRequiresHigherRole": {
    message: "Для уровня {level} нужна более высокая роль участника.",
    description:
      "Reason beside a level the viewer's role cannot grant, when bex-api names a role the dashboard does not know",
  },
  "apiKeys.permCatalogLoading": {
    message: "Загрузка разрешений…",
    description:
      "Accessible name of the picker while its permission list loads",
  },
  "apiKeys.permCatalogErrorCreate": {
    message:
      "Не удалось загрузить список разрешений. Ключ, созданный сейчас, получит все разрешения.",
    description:
      "Create dialog note when the permission list fails to load: creating still works and grants All",
  },
  "apiKeys.permCatalogErrorEdit": {
    message: "Не удалось загрузить список разрешений.",
    description:
      "Permissions editor note when the permission list fails to load, beside a Try again button",
  },
  "apiKeys.permSummaryCount_one": {
    message: "{count} разрешение",
    description:
      "Key row permissions summary beside Custom: how many permissions the key has",
  },
  "apiKeys.permSummaryCount_few": {
    message: "{count} разрешения",
    description:
      "Key row permissions summary beside Custom: how many permissions the key has",
  },
  "apiKeys.permSummaryCount_many": {
    message: "{count} разрешений",
    description:
      "Key row permissions summary beside Custom: how many permissions the key has",
  },
  "apiKeys.permSummaryCount_other": {
    message: "{count} разрешения",
    description:
      "Key row permissions summary beside Custom: how many permissions the key has",
  },
};

export default ruApiKeys;
