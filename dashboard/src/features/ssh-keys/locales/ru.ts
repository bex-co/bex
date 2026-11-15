import type { TranslationEntry } from "@/i18n";

const ruSSHKeys: Record<string, TranslationEntry> = {
  "sshKeys.title": {
    message: "Публичные SSH-ключи",
    description: "SSH keys settings card title",
  },
  "sshKeys.description": {
    message:
      "Публичные ключи, зарегистрированные для вашей учётной записи. Приватные ключи никогда не покидают ваше устройство.",
    description: "SSH keys settings card description",
  },
  "sshKeys.add": {
    message: "Добавить SSH-ключ",
    description: "Open add SSH key dialog",
  },
  "sshKeys.addTitle": {
    message: "Добавить публичный SSH-ключ",
    description: "Add SSH key dialog title",
  },
  "sshKeys.addDescription": {
    message:
      "Вставьте один публичный ключ OpenSSH. При сохранении комментарии удаляются.",
    description: "Add SSH key dialog description",
  },
  "sshKeys.name": {
    message: "Название",
    description: "SSH key name field and table column",
  },
  "sshKeys.publicKey": {
    message: "Публичный ключ",
    description: "SSH public key field label",
  },
  "sshKeys.invalid": {
    message:
      "Введите один поддерживаемый публичный ключ OpenSSH (для RSA — не менее 2048 бит).",
    description: "Invalid SSH public key hint",
  },
  "sshKeys.cancel": {
    message: "Отмена",
    description: "Cancel SSH key action",
  },
  "sshKeys.save": {
    message: "Добавить ключ",
    description: "Submit SSH key button",
  },
  "sshKeys.fingerprint": {
    message: "Отпечаток",
    description: "SSH key fingerprint table column",
  },
  "sshKeys.created": {
    message: "Создан",
    description: "SSH key creation table column",
  },
  "sshKeys.actions": {
    message: "Действия",
    description: "SSH key actions table column",
  },
  "sshKeys.emptyTitle": {
    message: "Нет SSH-ключей",
    description: "SSH key empty state title",
  },
  "sshKeys.emptyBody": {
    message:
      "Добавьте публичный ключ, прежде чем подключаться к работающему сервису.",
    description: "SSH key empty state body",
  },
  "sshKeys.errorTitle": {
    message: "Не удалось загрузить SSH-ключи",
    description: "SSH key load error title",
  },
  "sshKeys.errorBody": {
    message:
      "Повторите попытку или обратитесь к администратору рабочего пространства, если доступ запрещён.",
    description: "SSH key load error body",
  },
  "sshKeys.createSuccess": {
    message: "Добавлен ключ {name}",
    description: "SSH key creation success toast",
  },
  "sshKeys.createError": {
    message:
      "Не удалось добавить этот SSH-ключ. Проверьте формат и используйте RSA от 2048 бит или более стойкий ключ.",
    description: "SSH key creation failure toast",
  },
  "sshKeys.duplicateError": {
    message: "Этот публичный ключ уже зарегистрирован",
    description: "Duplicate SSH key toast",
  },
  "sshKeys.delete": {
    message: "Удалить",
    description: "Delete SSH key action",
  },
  "sshKeys.deleteNamed": {
    message: "Удалить SSH-ключ {name}",
    description: "Row delete button aria-label identifying the SSH key",
  },
  "sshKeys.deleteTitle": {
    message: "Удалить {name}?",
    description: "Delete SSH key confirmation title",
  },
  "sshKeys.deleteBody": {
    message:
      "Новые SSH-подключения с этим ключом будут немедленно отклоняться.",
    description: "Delete SSH key confirmation body",
  },
  "sshKeys.deleteSuccess": {
    message: "Удалён ключ {name}",
    description: "Delete SSH key success toast",
  },
  "sshKeys.deleteError": {
    message: "Не удалось удалить этот SSH-ключ",
    description: "Delete SSH key failure toast",
  },
  "sshKeys.gateTitle": {
    message: "Добавьте SSH-ключ для подключения",
    description: "RequiresSshKey gate — CTA title when no key is registered",
  },
  "sshKeys.gateBody": {
    message:
      "Подключение выполняется по SSH с вашим собственным ключом. Добавьте публичный ключ один раз, затем вернитесь и подключитесь.",
    description: "RequiresSshKey gate — CTA body",
  },
  "sshKeys.gateAddKey": {
    message: "Добавить SSH-ключ",
    description: "RequiresSshKey gate — primary action linking to settings",
  },
};

export default ruSSHKeys;
