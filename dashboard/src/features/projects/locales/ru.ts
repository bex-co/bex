import type { TranslationEntry } from "@/i18n";

const ruProjects: Record<string, TranslationEntry> = {
  "projects.overviewTitle": {
    message: "Обзор",
    description: "Page title of the Overview page (workspace homepage)",
  },
  "projects.projectsHeading": {
    message: "Проекты",
    description: "Section heading above the Overview page's project grid",
  },
  "projects.ungroupedHeading": {
    message: "Ресурсы без проекта",
    description:
      "Section heading above the Overview page's ungrouped resource table",
  },
  "projects.allResourcesHeading": {
    message: "Все ресурсы",
    description:
      "Heading above a project page's full resource table, below the environments section",
  },
  "projects.filterTypeLabel": {
    message: "Тип ресурса",
    description: "Accessible label for project environment type filters",
  },
  "projects.filterAll": {
    message: "Все",
    description: "All project environment resource types",
  },
  "projects.filterServices": {
    message: "Сервисы",
    description: "Service resource filter",
  },
  "projects.filterDatabases": {
    message: "Базы данных",
    description: "Database resource filter",
  },
  "projects.filterKeyValues": {
    message: "Key Value",
    description: "Key-value resource filter",
  },
  "projects.filterEnvGroups": {
    message: "Группы окружения",
    description: "Environment-group resource filter",
  },
  "projects.searchLabel": {
    message: "Поиск ресурсов в окружении {environment}",
    description: "Accessible project environment resource-search label",
  },
  "projects.searchPlaceholder": {
    message: "Поиск ресурсов в окружении {environment}",
    description: "Project environment resource-search placeholder",
  },
  "projects.noMatchesTitle": {
    message: "Подходящих ресурсов нет",
    description: "Project environment filtered no-results title",
  },
  "projects.noMatchesBody": {
    message: "Попробуйте другой запрос или тип ресурса.",
    description: "Project environment filtered no-results body",
  },
  "projects.eyebrow": {
    message: "Проект",
    description: "Small uppercase label above a project page's name heading",
  },
  "projects.cardEmpty": {
    message: "Ресурсов пока нет",
    description: "Project card status line when the project has no resources",
  },
  "projects.cardHealthy": {
    message: "Все ресурсы работают",
    description: "Project card status line when every resource is healthy",
  },
  "projects.cardInProgress_one": {
    message: "{count} ресурс в процессе",
    description:
      "Project card status line when resources are provisioning or deploying without a failure (singular)",
  },
  "projects.cardInProgress_few": {
    message: "{count} ресурса в процессе",
    description:
      "Project card status line when resources are provisioning or deploying without a failure (plural)",
  },
  "projects.cardInProgress_many": {
    message: "{count} ресурсов в процессе",
    description:
      "Project card status line when resources are provisioning or deploying without a failure (plural)",
  },
  "projects.cardInProgress_other": {
    message: "{count} ресурса в процессе",
    description:
      "Project card status line when resources are provisioning or deploying without a failure (plural)",
  },
  "projects.cardUnhealthy_one": {
    message: "{count} ресурс требует внимания",
    description: "Project card status line when one resource is unhealthy",
  },
  "projects.cardUnhealthy_few": {
    message: "{count} ресурса требуют внимания",
    description:
      "Project card status line when more than one resource is unhealthy",
  },
  "projects.cardUnhealthy_many": {
    message: "{count} ресурсов требуют внимания",
    description:
      "Project card status line when more than one resource is unhealthy",
  },
  "projects.cardUnhealthy_other": {
    message: "{count} ресурса требуют внимания",
    description:
      "Project card status line when more than one resource is unhealthy",
  },
  "projects.newProjectCard": {
    message: "Создать новый проект",
    description:
      'The dashed "+ create project" tile at the end of the project grid',
  },
  "projects.errorTitle": {
    message: "Не удалось загрузить ресурсы",
    description: "Error state heading on the unified Projects page",
  },
  "projects.errorBody": {
    message:
      "Не удалось загрузить ваши ресурсы. Попробуйте ещё раз чуть позже.",
    description: "Error state body on the unified Projects page",
  },
  "projects.emptyTitle": {
    message: "Ресурсов пока нет",
    description: "Empty state heading on the unified Projects page",
  },
  "projects.emptyBody": {
    message:
      "Создайте сервис, базу данных или хранилище Key Value, чтобы начать.",
    description: "Empty state body on the unified Projects page",
  },
  "projects.emptyCreateService": {
    message: "Создать сервис",
    description: "Primary action of the empty Projects page (ADR075 D1)",
  },
  "projects.newButton": {
    message: "Создать",
    description: "Button that opens the new-resource menu",
  },
  "projects.newService": {
    message: "Новый сервис",
    description: "New-resource menu item that starts the service create wizard",
  },
  "projects.newDatabase": {
    message: "Новая база данных",
    description: "New-resource menu item that opens the database create dialog",
  },
  "projects.newKeyValue": {
    message: "Новый Key Value",
    description: "New-resource menu item that starts the key value create form",
  },
  "projects.colName": {
    message: "Название",
    description: "Merged resource table column header",
  },
  "projects.colType": {
    message: "Тип",
    description: "Merged resource table column header",
  },
  "projects.colStatus": {
    message: "Статус",
    description: "Merged resource table column header",
  },
  "projects.colCreated": {
    message: "Создан",
    description: "Merged resource table column header",
  },
  "projects.colRuntime": {
    message: "Среда выполнения",
    description:
      "Authoritative runtime/product column in a Project resource table",
  },
  "projects.colRegion": {
    message: "Регион",
    description:
      "Explicit installation placement column in a Project resource table",
  },
  "projects.colUpdated": {
    message: "Обновлён",
    description:
      "Authoritative resource update-time column in a Project resource table",
  },
  "projects.colActions": {
    message: "Действия",
    description: "Merged resource table column header (screen-reader only)",
  },
  "projects.typeService": {
    message: "Сервис",
    description: "Type badge for a service row in the merged resource table",
  },
  "projects.typeDatabase": {
    message: "База данных",
    description: "Type badge for a database row in the merged resource table",
  },
  "projects.typeKeyValue": {
    message: "Key Value",
    description: "Type badge for a key value row in the merged resource table",
  },
  "projects.typeEnvGroup": {
    message: "Группа окружения",
    description: "Type badge for an environment-group row",
  },
  "projects.metadataNotApplicable": {
    message: "Неприменимо",
    description:
      "Accessible label for a status, runtime or region cell on an environment group row, which has none of them (w4/549)",
  },
  "projects.metadataUnavailable": {
    message: "Недоступно",
    description:
      "Accessible label for an unavailable runtime, region, status, or timestamp",
  },
  "projects.selectAllVisible": {
    message: "Выбрать все видимые ресурсы",
    description: "Accessible label for the Project table header checkbox",
  },
  "projects.selectResource": {
    message: "Выбрать {name}",
    description: "Accessible label for one Project resource row checkbox",
  },
  "projects.selectedCount": {
    message: "Выбрано: {count}",
    description: "Bulk action toolbar selection count",
  },
  "projects.moveTargetLabel": {
    message: "Переместить выбранные ресурсы в окружение",
    description: "Accessible label for the bulk Move target selector",
  },
  "projects.moveTargetPlaceholder": {
    message: "Выберите окружение",
    description: "Placeholder for the bulk Move target selector",
  },
  "projects.moveSelected": {
    message: "Переместить",
    description:
      "Bulk action that moves selected resources to another Environment",
  },
  "projects.bulkResume": {
    message: "Возобновить",
    description:
      "Selection toolbar action that resumes every selected suspended resource (Render's bulk Resume, w4/m550)",
  },
  "projects.bulkSuspend": {
    message: "Приостановить",
    description:
      "Selection toolbar action that suspends every selected running resource (Render's bulk Suspend, w4/m550)",
  },
  "projects.bulkSuspendTitle": {
    message: "Приостановить сервисы",
    description: "Title of the bulk Suspend dialog",
  },
  "projects.bulkSuspendBody": {
    message:
      "Следующие сервисы будут приостановлены и сразу перестанут работать. Это можно отменить, возобновив сервисы.",
    description:
      "Body of the bulk Suspend dialog, above the list of resources it suspends",
  },
  "projects.bulkSuspendConfirm": {
    message: "Приостановить сервисы",
    description: "Confirm button of the bulk Suspend dialog",
  },
  "projects.bulkResumeTitle": {
    message: "Возобновить сервисы",
    description: "Title of the bulk Resume dialog",
  },
  "projects.bulkResumeBody": {
    message: "Следующие сервисы будут возобновлены:",
    description:
      "Body of the bulk Resume dialog, above the list of resources it resumes",
  },
  "projects.bulkResumeConfirm": {
    message: "Возобновить сервисы",
    description: "Confirm button of the bulk Resume dialog",
  },
  "projects.bulkRetry": {
    message: "Повторить",
    description:
      "Button on a bulk Suspend/Resume row whose call failed; runs that row again",
  },
  "projects.bulkRetryRow": {
    message: "Повторить для {name}",
    description:
      "Accessible label of a row's Retry button ({name} = the resource)",
  },
  "projects.bulkRowSuspending": {
    message: "Приостанавливается",
    description:
      "Screen-reader state of a bulk Suspend row whose call is in flight",
  },
  "projects.bulkRowSuspended": {
    message: "Приостановлен",
    description: "Screen-reader state of a bulk Suspend row that succeeded",
  },
  "projects.bulkRowResuming": {
    message: "Возобновляется",
    description:
      "Screen-reader state of a bulk Resume row whose call is in flight",
  },
  "projects.bulkRowResumed": {
    message: "Возобновлён",
    description: "Screen-reader state of a bulk Resume row that succeeded",
  },
  "projects.bulkRowError": {
    message: "Запрос не выполнен. Повторите попытку.",
    description:
      "Fallback reason on a bulk Suspend/Resume row whose call failed without a server reason",
  },
  "projects.newProjectButton": {
    message: "Новый проект",
    description: "Button that opens the new-project dialog",
  },
  "projects.createTitle": {
    message: "Новый проект",
    description: "New-project dialog title",
  },
  "projects.createDescription": {
    message:
      "Объединяйте сервисы, базы данных и хранилища Key Value. Каждый проект начинается с окружения.",
    description: "New-project dialog description",
  },
  "projects.fieldName": {
    message: "Название",
    description: "New-project dialog name field label",
  },
  "projects.fieldFirstEnvironment": {
    message: "Название первого окружения",
    description:
      "New-project dialog: name of the environment created with the project (w4/m264, Render parity)",
  },
  "projects.fieldNamePlaceholder": {
    message: "my-project",
    description: "New-project dialog name field placeholder",
  },
  "projects.createCancel": {
    message: "Отменить",
    description: "New-project dialog cancel button",
  },
  "projects.createSubmit": {
    message: "Создать проект",
    description: "New-project dialog submit button",
  },
  "projects.createSuccess": {
    message: "Проект «{name}» создан.",
    description: "Toast shown after a project is created",
  },
  "projects.createError": {
    message: "Не удалось создать проект «{name}».",
    description: "Toast shown when creating a project fails",
  },
  "projects.editNameButton": {
    message: "Изменить",
    description:
      "Accessible label for the project page's inline pencil-edit button, and the Settings page's Project Name \"Edit\" button",
  },
  "projects.renameFieldLabel": {
    message: "Название проекта",
    description:
      'Accessible/visible label for the project rename field, on both the overview dialog and the Settings card. Deliberately more specific than projects.fieldName\'s bare "Name": these controls are identified by value alone otherwise (w4/134)',
  },
  "projects.renameTitle": {
    message: "Переименовать проект",
    description: "Rename-project dialog title",
  },
  "projects.renameSubmit": {
    message: "Сохранить",
    description: "Rename-project dialog submit button",
  },
  "projects.renameSuccess": {
    message: "Проект переименован в «{name}».",
    description: "Toast shown after a project is renamed",
  },
  "projects.renameRefreshFailed": {
    message:
      "Проект переименован в «{name}», но эту страницу не удалось обновить. Повторите попытку или перезагрузите страницу, чтобы увидеть сохранённое название.",
    description:
      "Warning after the rename saved but re-reading the project failed; the rename must not be retried or reported as failed (w4/m186)",
  },
  "projects.renameError": {
    message: "Не удалось переименовать проект в «{name}».",
    description: "Toast shown when renaming a project fails",
  },
  "projects.settingsTitle": {
    message: "Настройки проекта",
    description: "Project settings page heading",
  },
  "projects.nameCardTitle": {
    message: "Название проекта",
    description: "Project settings page's name card title",
  },
  "projects.nameCardDescription": {
    message: "Уникальное название вашего проекта",
    description: "Project settings page's name card description",
  },
  "projects.deleteCardTitle": {
    message: "Удалить проект",
    description: "Project settings page's danger-zone card title",
  },
  "projects.deleteCardDescription": {
    message:
      "Вместе с ним удаляются его окружения и все их сервисы, базы данных и хранилища Key Value. Его группы окружения становятся неназначенными. Это действие нельзя отменить.",
    description: "Project settings page's danger-zone card description",
  },
  "projects.deleteCardButton": {
    message: "Удалить проект",
    description:
      "Project settings page's danger-zone card button, and the delete-confirmation dialog's confirm button",
  },
  "projects.deleteConfirmTitle": {
    message: "Удалить проект «{name}»?",
    description: "Delete-project confirmation dialog title",
  },
  "projects.deleteConfirmBody": {
    message:
      "Его окружения и их конфигурация будут удалены. Его группы окружения становятся неназначенными, но не удаляются.",
    description: "Delete-project confirmation dialog body",
  },
  "projects.deleteConfirmBodyResources": {
    message:
      "Удаление этого проекта удалит все его окружения и их сервисы, базы данных и хранилища Key Value: {resources}. Его группы окружения станут неназначенными, но не будут удалены. Это действие нельзя отменить.",
    description:
      "Delete-project confirmation body naming every service, database and key value store it deletes, as the environment delete does (w4/m264, w6/m290, w4/542)",
  },
  "projects.deleteSuccess": {
    message: "Проект «{name}» удалён.",
    description: "Toast shown after a project is deleted",
  },
  "projects.deleteError": {
    message: "Не удалось удалить проект «{name}».",
    description: "Toast shown when deleting a project fails",
  },
  "projects.moveToProject": {
    message: "Переместить в проект",
    description:
      "Row-actions submenu label for assigning a resource to a project",
  },
  "projects.removeFromProject": {
    message: "Убрать из проекта",
    description:
      "Row-actions submenu item that unassigns a resource from its project",
  },
  "projects.moveSuccess": {
    message: "«{name}» перемещён в «{project}».",
    description: "Toast shown after moving a resource to a project",
  },
  "projects.moveError": {
    message: "Не удалось переместить «{name}».",
    description: "Toast shown when moving a resource to a project fails",
  },
  "projects.removeSuccess": {
    message: "«{name}» убран из своего проекта.",
    description: "Toast shown after unassigning a resource from its project",
  },
  "projects.removeError": {
    message: "Не удалось убрать «{name}» из его проекта.",
    description: "Toast shown when unassigning a resource fails",
  },
  "projects.editProjectName": {
    message: "Изменить название проекта",
    description:
      "Screen-reader text of the project page's name link, which opens Project Settings with the Name card editing (Render's words, w1/m682)",
  },
  "projects.addEnvironment": {
    message: "Добавить окружение",
    description:
      "The project page header's button and the card under a project's only environment that open the new-environment dialog (Render's words, w1/m682)",
  },
  "projects.environmentLimitTitle": {
    message: "Больше окружений с тарифом {plan}",
    description:
      "Title of the paywall Add environment opens at the plan's environments per project; {plan} is the plan name (Render's words, w1/m682)",
  },
  "projects.environmentLimitBody": {
    message:
      "Повысьте тариф, чтобы получить неограниченное число окружений и все возможности тарифа {plan}.",
    description:
      "Body of the environments-per-project paywall; {plan} is the plan name (Render's words, w1/m682)",
  },
  "projects.environmentLimitUpgrade": {
    message: "Повысить тариф",
    description:
      "The environments paywall's button to the workspace's change-plan dialog",
  },
  "projects.environmentLimitLearnMore": {
    message: "Подробнее",
    description:
      "The environments paywall's link to the projects and environments docs",
  },
};

export default ruProjects;
