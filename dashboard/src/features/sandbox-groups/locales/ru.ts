import type { TranslationEntry } from "@/i18n";

const ruSandboxGroups: Record<string, TranslationEntry> = {
  "sandboxGroups.createTitle": {
    message: "Новая группа песочниц",
    description:
      "Title of the new-sandbox-group dialog and its entry in the + New menus",
  },
  "sandboxGroups.createDescription": {
    message:
      "Группа песочниц ограничивает, сколько её песочниц могут быть активны одновременно. Названия групп должны быть уникальными в вашем рабочем пространстве.",
    description: "New-sandbox-group dialog description",
  },
  "sandboxGroups.fieldName": {
    message: "Название",
    description: "New-sandbox-group dialog name field label",
  },
  "sandboxGroups.fieldNamePlaceholder": {
    message: "my-sandbox-group",
    description: "Placeholder for the new sandbox group's name",
  },
  "sandboxGroups.fieldRegion": {
    message: "Регион",
    description:
      "Label of the region row in the created sandbox group's confirmation",
  },
  "sandboxGroups.createCancel": {
    message: "Отменить",
    description: "Cancel button of the new-sandbox-group dialog",
  },
  "sandboxGroups.createSubmit": {
    message: "Создать группу песочниц",
    description: "Submit button of the new-sandbox-group dialog",
  },
  "sandboxGroups.createError": {
    message: "Не удалось создать группу песочниц {name}. Попробуйте ещё раз.",
    description: "Toast shown when creating a sandbox group fails",
  },
  "sandboxGroups.errorNameTaken": {
    message:
      "Группа песочниц с названием «{name}» уже существует в этом рабочем пространстве. Выберите другое название.",
    description:
      "Inline error under the name field when the group name is already taken",
  },
  "sandboxGroups.errorNameInvalid": {
    message:
      "Название группы должно содержать от 1 до {max} символов, без пробелов в начале и в конце и без управляющих символов.",
    description: "Inline error under the name field when the name is refused",
  },
  "sandboxGroups.errorLimitReached_one": {
    message:
      "В этом рабочем пространстве уже максимальное число групп песочниц: {count}.",
    description:
      "Inline alert when the workspace is at its sandbox group limit (singular)",
  },
  "sandboxGroups.errorLimitReached_few": {
    message:
      "В этом рабочем пространстве уже максимальное число групп песочниц: {count}.",
    description:
      "Inline alert when the workspace is at its sandbox group limit",
  },
  "sandboxGroups.errorLimitReached_many": {
    message:
      "В этом рабочем пространстве уже максимальное число групп песочниц: {count}.",
    description:
      "Inline alert when the workspace is at its sandbox group limit",
  },
  "sandboxGroups.errorLimitReached_other": {
    message:
      "В этом рабочем пространстве уже максимальное число групп песочниц: {count}.",
    description:
      "Inline alert when the workspace is at its sandbox group limit",
  },
  "sandboxGroups.errorLimitReachedGeneric": {
    message: "Это рабочее пространство достигло лимита групп песочниц.",
    description:
      "Inline alert for the sandbox group limit when the server names no number",
  },
  "sandboxGroups.errorLimitTitle": {
    message: "Достигнут лимит групп песочниц",
    description: "Heading of the inline sandbox-group-limit alert",
  },
  "sandboxGroups.createdTitle": {
    message: "Группа песочниц создана",
    description: "Dialog title once a sandbox group has been created",
  },
  "sandboxGroups.createdDescription": {
    message:
      "{name} готова. Передайте её ID как sandboxGroupId при создании песочницы в этой группе.",
    description: "Dialog description once a sandbox group has been created",
  },
  "sandboxGroups.createdDone": {
    message: "Готово",
    description: "Button that closes the created-sandbox-group confirmation",
  },
  "sandboxGroups.fieldId": {
    message: "ID",
    description: "Label of the new sandbox group's ID in the confirmation",
  },
  "sandboxGroups.fieldDefault": {
    message: "Группа по умолчанию",
    description:
      "Label of the confirmation row that says whether the group is the workspace's default",
  },
  "sandboxGroups.defaultYes": {
    message: "Да",
    description: "The created group is the workspace's default sandbox group",
  },
  "sandboxGroups.defaultNo": {
    message: "Нет",
    description:
      "The created group is not the workspace's default sandbox group",
  },
  "sandboxGroups.fieldConcurrencyLimit": {
    message: "Лимит параллельных песочниц",
    description:
      "Label of the confirmation row with the most active sandboxes the group allows",
  },
  "sandboxGroups.copyId": {
    message: "Копировать ID группы песочниц",
    description: "Accessible label of the copy button next to the group's ID",
  },
  "sandboxGroups.copied": {
    message: "ID группы песочниц скопирован",
    description: "Toast after copying the sandbox group ID",
  },
  "sandboxGroups.copyError": {
    message: "Не удалось скопировать ID группы песочниц",
    description: "Toast when copying the sandbox group ID fails",
  },
  "sandboxGroups.environmentLoading": {
    message: "Загрузка окружений",
    description:
      "Accessible name of the loading placeholder shown where the new-sandbox-group dialog's Environment select will appear",
  },
  "sandboxGroups.environmentsError": {
    message:
      "Не удалось загрузить ваши окружения. Группу всё равно можно создать без окружения.",
    description:
      "Shown in the new-sandbox-group dialog when the workspace's environments could not be read; the group can still be created without one",
  },
  "sandboxGroups.errorEnvironmentGone": {
    message:
      "Это окружение больше не существует. Выберите другое или создайте группу без окружения.",
    description:
      "Inline error under the Environment select when the chosen environment was deleted before the group was created",
  },
};

export default ruSandboxGroups;
