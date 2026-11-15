import type { TranslationEntry } from "@/i18n";

const ruEnvGroups: Record<string, TranslationEntry> = {
  "envGroups.resourceType": {
    message: "Группа окружения",
    description: "Environment Group resource type used in document titles",
  },
  "envGroups.pageTitle": {
    message: "Группы окружения",
    description: "Workspace env-groups page title",
  },
  "envGroups.pageDescription": {
    message:
      "Делитесь переменными окружения и секретными файлами между сервисами.",
    description: "Workspace env-groups page subtitle",
  },
  "envGroups.newButton": {
    message: "Новая группа окружения",
    description: "Open the create env-group dialog",
  },
  "envGroups.createTitle": {
    message: "Создать группу окружения",
    description: "Create env-group dialog title",
  },
  "envGroups.createDescription": {
    message:
      "Добавьте переменные, секретные файлы и связи с сервисами сейчас или оставьте их пустыми.",
    description: "Create env-group dialog description",
  },
  "envGroups.createVarsTitle": {
    message: "Переменные окружения",
    description: "Initial environment variables section title",
  },
  "envGroups.createVarsDescription": {
    message:
      "Задайте значения вручную или сгенерируйте надёжные секреты при создании.",
    description: "Initial environment variables section description",
  },
  "envGroups.varKeyLabel": {
    message: "Ключ",
    description: "Initial environment variable key label",
  },
  "envGroups.varKeyPlaceholder": {
    message: "API_TOKEN",
    description: "Initial environment variable key placeholder",
  },
  "envGroups.varValueLabel": {
    message: "Значение",
    description: "Initial environment variable value label",
  },
  "envGroups.varValuePlaceholder": {
    message: "Секретное значение",
    description: "Initial environment variable value placeholder",
  },
  "envGroups.generateValue": {
    message: "Сгенерировать",
    description: "Generate an initial environment variable value",
  },
  "envGroups.addInitialVar": {
    message: "Добавить переменную окружения",
    description: "Add an initial environment variable row",
  },
  "envGroups.importEnv": {
    message: "Импорт из .env",
    description:
      "Import initial environment variables from dotenv text or file",
  },
  "envGroups.createFilesTitle": {
    message: "Секретные файлы",
    description: "Initial secret files section title",
  },
  "envGroups.createFilesDescription": {
    message: "Файлы шифруются и монтируются в /etc/secrets.",
    description: "Initial secret files section description",
  },
  "envGroups.fileNameLabel": {
    message: "Имя файла",
    description: "Initial secret file name label",
  },
  "envGroups.fileNamePlaceholder": {
    message: "credentials.json",
    description: "Initial secret file name placeholder",
  },
  "envGroups.fileContentLabel": {
    message: "Содержимое",
    description: "Initial secret file contents label",
  },
  "envGroups.fileContentPlaceholder": {
    message: "Содержимое файла",
    description: "Initial secret file contents placeholder",
  },
  "envGroups.addInitialFile": {
    message: "Добавить секретный файл",
    description: "Add an initial secret file row",
  },
  "envGroups.createServicesTitle": {
    message: "Связанные сервисы",
    description: "Initial service links section title",
  },
  "envGroups.createServicesDescription": {
    message: "Выбранные сервисы получат эту группу сразу после её создания.",
    description: "Initial service links section description",
  },
  "envGroups.servicesLoading": {
    message: "Загрузка сервисов рабочего пространства…",
    description: "Initial service links loading state",
  },
  "envGroups.createScopeLoading": {
    message: "Загрузка окружений и сервисов…",
    description:
      "Create dialog waits for a confirmed scope and link candidates",
  },
  "envGroups.createScopeError": {
    message:
      "Не удалось загрузить окружение или сервисы. Повторите попытку, прежде чем создавать группу.",
    description:
      "Create dialog blocks submission when scope or service lookup fails",
  },
  "envGroups.noServicesToLink": {
    message: "В этом рабочем пространстве пока нет сервисов для связывания.",
    description: "Initial service links empty state",
  },
  "envGroups.noServicesInScope": {
    message:
      "В этом окружении нет сервисов. Сервисы можно связывать только с группой из их собственного окружения.",
    description: "Initial service links empty state for the picked scope",
  },
  "envGroups.invalidInitialContents": {
    message:
      "Исправьте недопустимые ключи переменных или имена файлов, прежде чем создавать группу.",
    description: "Invalid initial env-group contents message",
  },
  "envGroups.nameLabel": {
    message: "Название группы",
    description: "Env-group name field label",
  },
  "envGroups.namePlaceholder": {
    message: "shared-production",
    description: "Env-group name placeholder",
  },
  "envGroups.invalidName": {
    message: "Введите название группы.",
    description: "Invalid env-group name message",
  },
  "envGroups.cancel": {
    message: "Отмена",
    description: "Cancel an env-group dialog",
  },
  "envGroups.createSubmit": {
    message: "Создать группу окружения",
    description: "Create env-group submit button",
  },
  "envGroups.emptyTitle": {
    message: "Групп окружения нет",
    description: "Workspace env-groups empty-state title",
  },
  "envGroups.emptyBody": {
    message:
      "Создайте группу, чтобы управлять общей конфигурацией, прежде чем связывать её с сервисом.",
    description: "Workspace env-groups empty-state body",
  },
  "envGroups.serviceCount_one": {
    message: "{count} связанный сервис",
    description: "Env-group linked-service count (singular)",
  },
  "envGroups.serviceCount_few": {
    message: "{count} связанных сервиса",
    description: "Env-group linked-service count (plural)",
  },
  "envGroups.serviceCount_many": {
    message: "{count} связанных сервисов",
    description: "Env-group linked-service count (plural)",
  },
  "envGroups.serviceCount_other": {
    message: "{count} связанных сервиса",
    description: "Env-group linked-service count (plural)",
  },
  "envGroups.metadataTitle": {
    message: "Метаданные группы",
    description: "Env-group metadata card title",
  },
  "envGroups.ownerLabel": {
    message: "Владелец",
    description: "Env-group owner id label",
  },
  "envGroups.environmentLabel": {
    message: "Окружение",
    description: "Environment Group scope label",
  },
  "envGroups.workspaceLabel": {
    message: "Рабочее пространство",
    description: "Destination workspace label",
  },
  "envGroups.workspaceScope": {
    message: "Рабочее пространство (без окружения)",
    description: "Standalone Environment Group scope",
  },
  "envGroups.environmentLookupLoading": {
    message: "Загрузка окружения…",
    description:
      "Group metadata Environment value while the scope lookup is still loading",
  },
  "envGroups.environmentLookupFailed": {
    message: "Не удалось загрузить окружение",
    description:
      "Group metadata Environment value when the scope lookup request failed",
  },
  "envGroups.unknownEnvironment": {
    message: "Неизвестное окружение ({id})",
    description: "Deleted or inaccessible Environment fallback",
  },
  "envGroups.createdAtLabel": {
    message: "Создана",
    description: "Env-group created timestamp label",
  },
  "envGroups.updatedAtLabel": {
    message: "Обновлена",
    description: "Env-group updated timestamp label",
  },
  "envGroups.availabilityLabel": {
    message: "Статус",
    description: "Env-group availability label when busy or repair-required",
  },
  "envGroups.availabilityBusy": {
    message: "Идёт сохранение — повторите чуть позже",
    description: "Busy environment-group availability message",
  },
  "envGroups.availabilityRepairRequired": {
    message:
      "Требуется восстановление — если проблема сохранится, обратитесь в поддержку",
    description: "Repair-required environment-group availability message",
  },
  "envGroups.unavailableTitle": {
    message: "Группы окружения недоступны",
    description: "Secret store unavailable state title",
  },
  "envGroups.unavailableBody": {
    message: "Хранилище секретов не настроено для этой установки.",
    description: "Secret store unavailable state body",
  },
  "envGroups.forbiddenTitle": {
    message: "Нет доступа",
    description: "Env-group forbidden state title",
  },
  "envGroups.forbiddenBody": {
    message: "У вас нет прав на просмотр или управление группами окружения.",
    description: "Env-group forbidden state body",
  },
  "envGroups.genericTitle": {
    message: "Не удалось загрузить группы окружения",
    description: "Env-group generic query error title",
  },
  "envGroups.genericBody": {
    message: "Что-то пошло не так. Попробуйте ещё раз.",
    description: "Env-group generic query error body",
  },
  "envGroups.errorTitle": {
    message: "Не удалось загрузить содержимое группы",
    description: "Env-group editor error title",
  },
  "envGroups.errorBody": {
    message: "Что-то пошло не так. Попробуйте ещё раз.",
    description: "Env-group editor error body",
  },
  "envGroups.backToList": {
    message: "Назад к группам окружения",
    description: "Detail-page back button label",
  },
  "envGroups.renameButton": {
    message: "Переименовать",
    description: "Open rename env-group dialog",
  },
  "envGroups.manageButton": {
    message: "Управление",
    description: "Open Environment Group management menu",
  },
  "envGroups.moveButton": {
    message: "Переместить группу",
    description: "Open Environment Group move dialog",
  },
  "envGroups.moveTitle": {
    message: "Переместить группу окружения",
    description: "Environment Group move dialog title",
  },
  "envGroups.moveDescription": {
    message:
      "Выберите подходящее окружение или верните эту группу на уровень рабочего пространства.",
    description: "Environment Group move dialog description",
  },
  "envGroups.moveSubmit": {
    message: "Переместить группу",
    description: "Submit Environment Group move",
  },
  "envGroups.moveSuccess": {
    message: "Группа окружения перемещена",
    description: "Environment Group move success toast",
  },
  "envGroups.moveError": {
    message: "Не удалось переместить группу",
    description: "Environment Group move error toast",
  },
  "envGroups.cloneButton": {
    message: "Клонировать группу",
    description: "Open Environment Group clone dialog",
  },
  "envGroups.cloneTitle": {
    message: "Клонировать группу окружения",
    description: "Environment Group clone dialog title",
  },
  "envGroups.cloneDescription": {
    message:
      "Скопируйте переменные и файлы в новую группу. Связи с сервисами не копируются.",
    description: "Environment Group clone dialog description",
  },
  "envGroups.cloneSubmit": {
    message: "Клонировать группу",
    description: "Submit Environment Group clone",
  },
  "envGroups.cloneSuccess": {
    message: "Клонировано: {name}",
    description: "Environment Group clone success toast",
  },
  "envGroups.cloneError": {
    message: "Не удалось клонировать группу",
    description: "Environment Group clone error toast",
  },
  "envGroups.environmentPlaceholder": {
    message: "Выберите окружение",
    description: "Environment scope selector placeholder",
  },
  "envGroups.renameTitle": {
    message: "Переименовать группу окружения",
    description: "Rename env-group dialog title",
  },
  "envGroups.renameDescription": {
    message:
      "При переименовании переменные, файлы и связи с сервисами сохраняются.",
    description: "Rename env-group dialog description",
  },
  "envGroups.renameSubmit": {
    message: "Сохранить название",
    description: "Rename env-group submit button",
  },
  "envGroups.deleteButton": {
    message: "Удалить",
    description: "Open delete env-group dialog",
  },
  "envGroups.deleteTitle": {
    message: "Удалить {name}?",
    description: "Delete env-group dialog title",
  },
  "envGroups.deleteDescription": {
    message:
      "Это действие нельзя отменить. Вы уверены, что хотите удалить эту группу окружения? Она будет отвязана от всех сервисов.",
    description: "Delete env-group dialog warning (Render-style copy)",
  },
  "envGroups.deletePrompt": {
    message: "Введите {phrase} ниже для подтверждения.",
    description:
      "Body prompt naming the exact 'sudo delete env group <name>' phrase (rendered bold by SudoCommandField)",
  },
  "envGroups.deleteConfirm": {
    message: "Удалить группу окружения",
    description: "Delete env-group confirm button",
  },
  "envGroups.varsTitle": {
    message: "Переменные окружения",
    description: "Env-group variable editor title",
  },
  "envGroups.varsDescription": {
    message: "Значения шифруются и применяются ко всем связанным сервисам.",
    description: "Env-group variable editor description",
  },
  "envGroups.varsEmptyTitle": {
    message: "Переменных окружения нет",
    description: "Env-group variable editor empty title",
  },
  "envGroups.varsEmptyBody": {
    message:
      "Добавьте переменную сейчас; группу не нужно сначала связывать с сервисом.",
    description: "Env-group variable editor empty body",
  },
  "envGroups.varDeleteConfirmBody": {
    message:
      "Связанные сервисы будут повторно развёрнуты без этой переменной, кроме сервиса из репозитория с отключённым автодеплоем: он продолжит работать на текущем релизе и применит изменение при следующем деплое.",
    description: "Delete env-group variable warning",
  },
  "envGroups.filesTitle": {
    message: "Секретные файлы",
    description: "Env-group secret-file editor title",
  },
  "envGroups.filesDescription": {
    message:
      "Зашифрованные файлы монтируются в каждый связанный сервис в /etc/secrets.",
    description: "Env-group secret-file editor description",
  },
  "envGroups.filesEmptyTitle": {
    message: "Секретных файлов нет",
    description: "Env-group secret-file editor empty title",
  },
  "envGroups.filesEmptyBody": {
    message:
      "Добавьте файл сейчас; группу не нужно сначала связывать с сервисом.",
    description: "Env-group secret-file editor empty body",
  },
  "envGroups.fileDeleteConfirmBody": {
    message:
      "Связанные сервисы будут повторно развёрнуты без этого файла, кроме сервиса из репозитория с отключённым автодеплоем: он продолжит работать на текущем релизе и применит изменение при следующем деплое.",
    description: "Delete env-group secret-file warning",
  },
  "envGroups.servicesTitle": {
    message: "Связанные сервисы",
    description: "Env-group linked-services card title",
  },
  "envGroups.servicesDescription": {
    message:
      "Связывание и отвязывание меняют конфигурацию сервиса. Сервис из репозитория с отключённым автодеплоем применит изменение при следующем деплое; остальные сервисы развернутся сейчас.",
    description: "Env-group linked-services card description",
  },
  "envGroups.selectService": {
    message: "Выберите сервис",
    description: "Link-service selector placeholder",
  },
  "envGroups.linkButton": {
    message: "Связать сервис",
    description: "Link selected service to env group",
  },
  "envGroups.unlinkButton": {
    message: "Отвязать",
    description: "Unlink service from env group",
  },
  "envGroups.unlinkServiceAria": {
    message: "Отвязать {name}",
    description:
      "Accessible name of a linked-service row's Unlink control, naming the service (w4/150)",
  },
  "envGroups.noLinkedServices": {
    message:
      "Эта группа пока не связана ни с одним сервисом. Её по-прежнему можно полностью редактировать.",
    description: "No linked services state",
  },
  "envGroups.servicesLoadError": {
    message:
      "Не удалось загрузить сервисы рабочего пространства. Существующие связи по-прежнему доступны ниже.",
    description: "Linked-services inventory error",
  },
  "envGroups.incompatibleLink": {
    message: "Связана вне окружения этой группы",
    description: "Warning on an existing incompatible Environment Group link",
  },
  "envGroups.searchPlaceholder": {
    message: "Поиск групп окружения",
    description: "Environment Group list search placeholder",
  },
  "envGroups.searchLabel": {
    message: "Поиск групп окружения по названию",
    description: "Accessible Environment Group search label",
  },
  "envGroups.resetSearch": {
    message: "Сбросить поиск",
    description: "Clear Environment Group search",
  },
  "envGroups.noMatchesTitle": {
    message: "Подходящих групп окружения нет",
    description: "Environment Group search empty-state title",
  },
  "envGroups.noMatchesBody": {
    message: "Попробуйте другое название или сбросьте поиск.",
    description: "Environment Group search empty-state body",
  },
  "envGroups.colName": {
    message: "Название группы",
    description: "Environment Group table name column",
  },
  "envGroups.colEnvironment": {
    message: "Окружение",
    description: "Environment Group table Environment column",
  },
  "envGroups.colEnvVars": {
    message: "Переменные",
    description: "Environment Group table variable-count column",
  },
  "envGroups.colSecretFiles": {
    message: "Секретные файлы",
    description: "Environment Group table secret-file-count column",
  },
  "envGroups.colUpdated": {
    message: "Обновлена",
    description: "Environment Group table updated-time column",
  },
  "envGroups.rolloutNote": {
    message:
      "Связанные сервисы повторно развёртываются для применения изменения, кроме сервиса из репозитория с отключённым автодеплоем: он применит его при следующем деплое.",
    description: "Env-group write rollout toast detail",
  },
  "envGroups.linkChangeNote": {
    message:
      "Конфигурация сервиса изменена. Сервис из репозитория с отключённым автодеплоем применит её при следующем деплое; любой другой сервис развернётся сейчас.",
    description: "Env-group link/unlink success toast detail",
  },
  "envGroups.createSuccess": {
    message: "Создано: {name}",
    description: "Env-group create success toast",
  },
  "envGroups.createError": {
    message: "Не удалось создать {name}",
    description: "Env-group create error toast",
  },
  "envGroups.renameSuccess": {
    message: "Переименовано в {name}",
    description: "Env-group rename success toast",
  },
  "envGroups.renameError": {
    message: "Не удалось переименовать группу",
    description: "Env-group rename error toast",
  },
  "envGroups.deleteSuccess": {
    message: "Группа окружения удалена",
    description: "Env-group delete success toast",
  },
  "envGroups.deleteError": {
    message: "Не удалось удалить группу",
    description: "Env-group delete error toast",
  },
  "envGroups.linkSuccess": {
    message: "Сервис связан",
    description: "Env-group link success toast",
  },
  "envGroups.linkError": {
    message: "Не удалось связать сервис",
    description: "Env-group link error toast",
  },
  "envGroups.unlinkSuccess": {
    message: "Сервис отвязан",
    description: "Env-group unlink success toast",
  },
  "envGroups.unlinkError": {
    message: "Не удалось отвязать сервис",
    description: "Env-group unlink error toast",
  },
  "envGroups.varSaveSuccess": {
    message: "Сохранено: {key}",
    description: "Env-group variable save success toast",
  },
  "envGroups.varsSaveSuccess": {
    message: "Переменные окружения сохранены",
    description: "Env-group replace-all variables success toast",
  },
  "envGroups.varsSaveError": {
    message: "Не удалось сохранить переменные окружения",
    description: "Env-group replace-all variables error toast",
  },
  "envGroups.varSaveError": {
    message: "Не удалось сохранить {key}",
    description: "Env-group variable save error toast",
  },
  "envGroups.varDeleteSuccess": {
    message: "Удалено: {key}",
    description: "Env-group variable delete success toast",
  },
  "envGroups.varDeleteError": {
    message: "Не удалось удалить {key}",
    description: "Env-group variable delete error toast",
  },
};

export default ruEnvGroups;
