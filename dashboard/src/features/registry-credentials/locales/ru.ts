import type { TranslationEntry } from "@/i18n";

const ruRegistryCredentials: Record<string, TranslationEntry> = {
  "registryCredentials.title": {
    message: "Учётные данные реестра",
    description:
      "Settings Integrations → Registry Credentials section card title",
  },
  "registryCredentials.description": {
    message:
      "Учётные данные для приватных внешних реестров образов (Docker Hub, GHCR, GitLab Container Registry, ECR и др.), чтобы сервис с готовым образом мог загружать их из реестра.",
    description: "Settings Registry Credentials section card description",
  },
  "registryCredentials.colName": {
    message: "Название",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.colUsername": {
    message: "Имя пользователя",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.colStatus": {
    message: "Срок действия",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.colCreated": {
    message: "Создано",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.emptyTitle": {
    message: "Нет учётных данных реестра",
    description: "Registry Credentials empty-state title",
  },
  "registryCredentials.emptyBody": {
    message:
      "Добавьте учётные данные, чтобы сервис мог загружать образ из приватного реестра.",
    description: "Registry Credentials empty-state body",
  },
  "registryCredentials.forbiddenTitle": {
    message: "Нет доступа",
    description:
      "Registry Credentials state when the caller lacks permission (403)",
  },
  "registryCredentials.forbiddenBody": {
    message:
      "У вас нет прав управлять учётными данными реестра этого рабочего пространства.",
    description: "Registry Credentials forbidden-state body",
  },
  "registryCredentials.errorTitle": {
    message: "Не удалось загрузить учётные данные реестра",
    description: "Registry Credentials generic error title",
  },
  "registryCredentials.errorBody": {
    message: "Что-то пошло не так. Попробуйте ещё раз.",
    description: "Registry Credentials generic error body",
  },
  "registryCredentials.create": {
    message: "Добавить учётные данные",
    description: "Button that opens the create dialog",
  },
  "registryCredentials.createTitle": {
    message: "Добавить учётные данные",
    description: "Add Credential dialog title (Render's)",
  },
  "registryCredentials.createDescription": {
    message: "Подробнее о хранении внешних {link}.",
    description:
      "Add Credential dialog description; {link} is createDescriptionLink, linking bex's private-registry docs",
  },
  "registryCredentials.fieldHost": {
    message: "Хост реестра",
    description: "Create dialog host field label",
  },
  "registryCredentials.fieldUsername": {
    message: "Имя пользователя",
    description: "Create dialog username field label",
  },
  "registryCredentials.fieldAuthToken": {
    message: "Персональный токен доступа",
    description: "Credential secret field label",
  },
  "registryCredentials.hostInvalid": {
    message:
      "Введите хост реестра, например ghcr.io или registry.example.com:5000, без пути.",
    description:
      "Inline error under the Registry host when it is not a bare host[:port] (w21/053)",
  },
  "registryCredentials.hostTooLong": {
    message: "Не более 253 байт",
    description: "Inline error under an over-long Registry host (w21/053)",
  },
  "registryCredentials.usernameControl": {
    message: "Не должно содержать управляющих символов",
    description:
      "Inline error under a registry username with a control character (w21/053)",
  },
  "registryCredentials.usernameTooLong": {
    message: "Не более 253 байт",
    description: "Inline error under an over-long registry username (w21/053)",
  },
  "registryCredentials.authTokenTooLong": {
    message: "Не более 64 KiB",
    description:
      "Inline error under a registry password or token over the 64 KiB limit (w21/053)",
  },
  "registryCredentials.fieldName": {
    message: "Название",
    description: "Credential name field label (required)",
  },
  "registryCredentials.createCancel": {
    message: "Отменить",
    description: "Create dialog cancel button",
  },
  "registryCredentials.createSubmit": {
    message: "Добавить учётные данные",
    description: "Add Credential dialog submit button",
  },
  "registryCredentials.createSuccess": {
    message: "Учётные данные для {host} добавлены",
    description: "Toast on a successful create",
  },
  "registryCredentials.createError": {
    message: "Не удалось добавить учётные данные реестра",
    description: "Toast on a failed create",
  },
  "registryCredentials.delete": {
    message: "Удалить",
    description: "Row action / confirmation button to delete a credential",
  },
  "registryCredentials.deleteNamed": {
    message: "Удалить учётные данные реестра {name}",
    description:
      "Row delete button aria-label identifying the registry credential",
  },
  "registryCredentials.edit": {
    message: "Изменить",
    description: "Row action to open the edit-credential dialog",
  },
  "registryCredentials.editTitle": {
    message: "Изменить учётные данные реестра",
    description: "Edit-credential dialog title",
  },
  "registryCredentials.editDescription": {
    message:
      "Переименуйте учётные данные, измените имя пользователя или смените токен. Реестр изменить нельзя.",
    description: "Edit-credential dialog description",
  },
  "registryCredentials.editSubmit": {
    message: "Сохранить изменения",
    description: "Edit-credential dialog submit button",
  },
  "registryCredentials.fieldAuthTokenKeep": {
    message: "Оставьте пустым, чтобы сохранить текущий токен",
    description: "Placeholder for the token field in the edit dialog",
  },
  "registryCredentials.fieldAuthTokenKeepHint": {
    message:
      "Сохранённый токен не показывается. Введите новое значение, только чтобы сменить токен.",
    description: "Hint under the token field in the edit dialog",
  },
  "registryCredentials.updateSuccess": {
    message: "Учётные данные реестра обновлены.",
    description: "Toast after an edit succeeds",
  },
  "registryCredentials.updateError": {
    message: "Не удалось обновить учётные данные реестра.",
    description: "Toast after an edit fails",
  },
  "registryCredentials.deleteConfirmTitle": {
    message: "Удалить {name}?",
    description: "Delete-confirmation dialog title",
  },
  "registryCredentials.deleteConfirmBody": {
    message:
      "Удалить можно только учётные данные, которые не использует ни один сервис — сначала уберите их из каждого сервиса, иначе удаление будет отклонено. Это действие нельзя отменить.",
    description: "Delete-confirmation dialog body",
  },
  "registryCredentials.deleteCancel": {
    message: "Отменить",
    description: "Delete-confirmation dialog cancel button",
  },
  "registryCredentials.deleteSuccess": {
    message: "{name} удалены",
    description: "Toast on a successful delete",
  },
  "registryCredentials.deleteError": {
    message: "Не удалось удалить {name}",
    description: "Toast on a failed delete",
  },
  "registryCredentials.expired": {
    message: "Срок действия истёк",
    description:
      "Expiry-status badge for a past-expiry credential (w2/m14/t007)",
  },
  "registryCredentials.expiringSoon": {
    message: "Скоро истекает",
    description: "Expiry-status badge for a credential nearing expiry",
  },
  "registryCredentials.expiresOn": {
    message: "Истекает {date}",
    description:
      "Expiry-status text for an active credential with a future expiry",
  },
  "registryCredentials.neverExpires": {
    message: "Бессрочно",
    description: "Expiry-status text for a credential with no expiry set",
  },
  "registryCredentials.createDescriptionLink": {
    message: "учётных данных реестров",
    description: "Link text inside createDescription",
  },
  "registryCredentials.fieldNamePlaceholder": {
    message: "Мои учётные данные",
    description: "Credential name field placeholder",
  },
  "registryCredentials.fieldRegistry": {
    message: "Реестр",
    description: "Add Credential dialog Registry select label",
  },
  "registryCredentials.fieldRegistryPlaceholder": {
    message: "Выберите реестр",
    description: "Registry select placeholder before a registry is chosen",
  },
  "registryCredentials.fieldHostHint": {
    message: "Хост в начале URL образа, без пути.",
    description:
      "Hint under the Registry host field (Google Artifact and AWS ECR only)",
  },
  "registryCredentials.hostRegistryMismatch": {
    message: "Введите хост {registry}, например {example}.",
    description:
      "Inline error under a Registry host that belongs to another registry; {registry} is the chosen registry's name, {example} an example host",
  },
  "registryCredentials.fieldAuthTokenOrPassword": {
    message: "Персональный токен доступа или пароль",
    description:
      "Credential secret field label for Docker, which also takes a password",
  },
  "registryCredentials.helpGithub": {
    message:
      "Создайте персональный токен доступа с разрешением {permission} в {link} {path}.",
    description:
      "Help under the Registry select for GitHub; {permission} is read:packages, {link} links GitHub's new-token page, {path} is GitHub's menu path",
  },
  "registryCredentials.helpGitlab": {
    message:
      "Создайте персональный токен доступа с разрешением {permission} в {link}: откройте {path}.",
    description:
      "Help under the Registry select for GitLab; {permission} is read registry, {link} links GitLab's token page, {path} is GitLab's menu path",
  },
  "registryCredentials.helpDocker": {
    message:
      "Введите имя пользователя и пароль Docker Hub или создайте токен доступа с разрешением {permission}, чтобы использовать его вместо пароля: {link} {path}.",
    description:
      "Help under the Registry select for Docker; {permission} is read, {link} links Docker Hub's security settings, {path} is Docker Hub's menu path",
  },
  "registryCredentials.helpGoogle": {
    message: "Создайте сервисный аккаунт с разрешением {permission} в {link}.",
    description:
      "Help under the Registry select for Google Artifact; {permission} is the reader role, {link} links Google Cloud's service account page",
  },
  "registryCredentials.helpEcr": {
    message:
      "Имя пользователя — это ID аккаунта AWS, которому принадлежит репозиторий ECR. Персональный токен доступа — это вывод команды {command}. Подробнее — в {link}.",
    description:
      "Help under the Registry select for AWS ECR; {command} is the aws CLI command, {link} is helpEcrLink",
  },
  "registryCredentials.helpEcrLink": {
    message: "документации AWS",
    description: "Link text to the AWS ECR authentication docs inside helpEcr",
  },
  "registryCredentials.fieldRegistryImmutable": {
    message: "Реестр нельзя изменить после создания.",
    description: "Hint under the read-only Registry in the edit dialog",
  },
};

export default ruRegistryCredentials;
