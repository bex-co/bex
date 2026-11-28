import type { TranslationEntry } from "@/i18n";

const ruEnvironments: Record<string, TranslationEntry> = {
  "environments.heading": {
    message: "Окружения",
    description: "Section heading above a project page's environments list",
  },
  "environments.selectorLabel": {
    message: "Выбранное окружение",
    description:
      "Accessible label for the Project overview Environment selector",
  },
  "environments.unassignedOption": {
    message: "Не назначено",
    description: "Project resources that belong to no Environment",
  },
  "environments.newButton": {
    message: "Новое окружение",
    description: "Button that opens the new-environment dialog",
  },
  "environments.emptyBody": {
    message:
      "Окружений пока нет. Создайте одно (например, staging или production), чтобы сгруппировать ресурсы этого проекта.",
    description: "Empty state shown when a project has no environments",
  },
  "environments.errorBody": {
    message: "Не удалось загрузить окружения. Повторите попытку чуть позже.",
    description: "Error state shown when the environments query fails",
  },
  "environments.refreshErrorBody": {
    message:
      "Не удалось обновить окружения. Показаны последние загруженные данные.",
    description:
      "Inline notice when a background refresh of environments fails while the previously loaded environments stay on screen",
  },
  "environments.resourceCount_one": {
    message: "{count} ресурс",
    description:
      "Count of visible services+databases+key-value instances assigned to an environment, next to its name (singular)",
  },
  "environments.resourceCount_few": {
    message: "{count} ресурса",
    description:
      "Count of visible services+databases+key-value instances assigned to an environment, next to its name (plural)",
  },
  "environments.resourceCount_many": {
    message: "{count} ресурсов",
    description:
      "Count of visible services+databases+key-value instances assigned to an environment, next to its name (plural)",
  },
  "environments.resourceCount_other": {
    message: "{count} ресурса",
    description:
      "Count of visible services+databases+key-value instances assigned to an environment, next to its name (plural)",
  },
  "environments.manageButton": {
    message: "Управление ресурсами",
    description:
      "Button on an environment card that opens the manage-resources dialog",
  },
  "environments.moreActions": {
    message: "Другие действия",
    description:
      "Accessible label for an environment card's overflow (•••) menu",
  },
  "environments.cardEmpty": {
    message: "В этом окружении пока нет ресурсов.",
    description:
      "Shown inside an environment card when it has no assigned resources",
  },
  "environments.createTitle": {
    message: "Новое окружение",
    description: "New-environment dialog title",
  },
  "environments.createDescription": {
    message:
      "Сгруппируйте часть ресурсов этого проекта под одним названием, например staging или production.",
    description: "New-environment dialog description",
  },
  "environments.fieldName": {
    message: "Название",
    description: "New-environment dialog name field label",
  },
  "environments.fieldNamePlaceholder": {
    message: "staging",
    description: "New-environment dialog name field placeholder",
  },
  "environments.cancel": {
    message: "Отмена",
    description: "Cancel button shared by the environment dialogs",
  },
  "environments.createSubmit": {
    message: "Создать окружение",
    description: "New-environment dialog submit button",
  },
  "environments.createSuccess": {
    message: "Окружение «{name}» создано.",
    description: "Toast shown after an environment is created",
  },
  "environments.createRefreshFailed": {
    message:
      "Окружение «{name}» создано, но список не удалось обновить. Повторите попытку или перезагрузите страницу, чтобы увидеть его.",
    description:
      "Warning after the environment was created but re-reading the list failed; creation must not be retried or reported as failed (w4/231)",
  },
  "environments.createError": {
    message: "Не удалось создать окружение «{name}».",
    description: "Toast shown when creating an environment fails",
  },
  "environments.deleteSuccess": {
    message: "Окружение {name} успешно удалено.",
    description:
      "Toast shown after an environment is deleted (Render's words, w1/m683)",
  },
  "environments.deleteRefreshFailed": {
    message:
      "Окружение «{name}» удалено, но список не удалось обновить. Повторите попытку или перезагрузите страницу, чтобы обновить его.",
    description:
      "Warning after the environment was deleted but re-reading the list failed; deletion must not be retried or reported as failed (w4/290)",
  },
  "environments.manageTitle": {
    message: "Управление ресурсами в «{name}»",
    description: "Manage-resources dialog title",
  },
  "environments.manageDescription": {
    message:
      "Отметьте ресурсы, которые относятся к этому окружению. Назначение ресурса также добавляет его в этот проект.",
    description: "Manage-resources dialog description",
  },
  "environments.tabServices": {
    message: "Сервисы",
    description: "Manage-resources dialog tab label for the services checklist",
  },
  "environments.tabDatabases": {
    message: "Базы данных",
    description:
      "Manage-resources dialog tab label for the databases checklist",
  },
  "environments.tabKeyValues": {
    message: "Key Value",
    description:
      "Manage-resources dialog tab label for the key-value checklist",
  },
  "environments.manageNoServices": {
    message: "В этом рабочем пространстве пока нет сервисов для назначения.",
    description:
      "Manage-resources dialog empty state when the workspace has no services",
  },
  "environments.manageNoDatabases": {
    message: "В этом рабочем пространстве пока нет баз данных для назначения.",
    description:
      "Manage-resources dialog empty state when the workspace has no databases",
  },
  "environments.manageNoKeyValues": {
    message:
      "В этом рабочем пространстве пока нет инстансов Key Value для назначения.",
    description:
      "Manage-resources dialog empty state when the workspace has no key-value instances",
  },
  "environments.manageResourceIn": {
    message: "В окружении {environment}",
    description:
      "Under a Manage resources row: the other environment the resource is in now ({environment} = its name)",
  },
  "environments.markerProtected": {
    message: "Защищено",
    description:
      "Marker: the environment the resource would leave is protected",
  },
  "environments.markerIsolated": {
    message: "Изолировано",
    description:
      "Marker: the environment the resource would leave has network isolation",
  },
  "environments.markerAllowList": {
    message: "Список разрешённых IP",
    description:
      "Marker: the environment the resource would leave restricts inbound IPs",
  },
  "environments.manageMovesOut": {
    message: "Будет перемещено из {environment}: {resources}",
    description:
      "Before Save: the selected resources this save moves out of another environment ({resources} = comma-separated names)",
  },
  "environments.manageNeedsAdminToMove": {
    message: "Вывести его из {environment} может только администратор.",
    description:
      "Why a Manage resources row is disabled: its current environment is admin-managed",
  },
  "environments.manageUnsaved": {
    message: "Несохранённые изменения",
    description:
      "Accessible description of a Manage resources tab whose selection differs from the saved membership",
  },
  "environments.manageSubmit": {
    message: "Сохранить",
    description: "Manage-resources dialog submit button",
  },
  "environments.assignSuccess": {
    message: "Сервисы для «{name}» обновлены.",
    description: "Toast shown after an environment's services are updated",
  },
  "environments.strandedMoveTitle": {
    message: "После этого перемещения останутся связанные группы",
    description:
      "Confirmation title shown before a move that strands an environment-scoped group link (w4/m238)",
  },
  "environments.strandedMoveBody": {
    message:
      "Эти группы окружения остаются связанными с перемещаемым сервисом, и их переменные и файлы продолжают применяться к нему, пока группа не будет отвязана или перемещена:",
    description:
      "Confirmation body: the stranded groups keep feeding the service after the move",
  },
  "environments.strandedMoveConfirm": {
    message: "Всё равно переместить",
    description: "Confirm button on the stranded-groups move confirmation",
  },
  "environments.strandedScopeWorkspace": {
    message: "Рабочее пространство",
    description:
      "Scope label for a stranded group that is scoped to the whole workspace",
  },
  "environments.strandedScopeUnknown": {
    message:
      "Пока не удалось проверить связанные группы окружения — дождитесь загрузки списка и повторите попытку.",
    description:
      "Toast shown when a move is attempted before the group/scope reads resolve",
  },
  "environments.assignError": {
    message: "Не удалось обновить сервисы для «{name}».",
    description: "Toast shown when updating an environment's services fails",
  },
  "environments.assignDatabasesSuccess": {
    message: "Базы данных для «{name}» обновлены.",
    description: "Toast shown after an environment's databases are updated",
  },
  "environments.assignDatabasesError": {
    message: "Не удалось обновить базы данных для «{name}».",
    description: "Toast shown when updating an environment's databases fails",
  },
  "environments.assignDatabasesRefreshFailed": {
    message:
      "Базы данных для «{name}» обновлены, но список не удалось обновить. Повторите попытку или перезагрузите страницу, чтобы увидеть их.",
    description:
      "Warning after the database membership was saved but re-reading the environments list failed; the write must not be retried or reported as failed (w4/325)",
  },
  "environments.assignKeyValuesSuccess": {
    message: "Инстансы Key Value для «{name}» обновлены.",
    description:
      "Toast shown after an environment's key-value instances are updated",
  },
  "environments.assignKeyValuesRefreshFailed": {
    message:
      "Инстансы Key Value для «{name}» обновлены, но список не удалось обновить. Повторите попытку или перезагрузите страницу, чтобы увидеть их.",
    description:
      "Warning after the key-value membership was saved but re-reading the environments list failed; the write must not be retried or reported as failed (w4/263)",
  },
  "environments.assignKeyValuesError": {
    message: "Не удалось обновить инстансы Key Value для «{name}».",
    description:
      "Toast shown when updating an environment's key-value instances fails",
  },
  "environments.tabEnvGroups": {
    message: "Группы окружения",
    description: "Manage-resources dialog tab label for environment groups",
  },
  "environments.manageNoEnvGroups": {
    message:
      "В этом рабочем пространстве пока нет групп окружения для назначения.",
    description: "Environment-groups checklist empty state",
  },
  "environments.tabSandboxGroups": {
    message: "Группы песочниц",
    description:
      "Manage-resources dialog tab label for the workspace's sandbox groups",
  },
  "environments.manageNoSandboxGroups": {
    message:
      "В этом рабочем пространстве пока нет групп песочниц для назначения.",
    description: "Sandbox-groups checklist empty state",
  },
  "environments.assignEnvGroupsSuccess": {
    message: "Группы окружения для «{name}» обновлены.",
    description: "Toast shown after environment-group membership is updated",
  },
  "environments.assignEnvGroupsPlacementUnread": {
    message:
      "Не удалось подтвердить, где сейчас находятся группы для «{name}». Повторите попытку или перезагрузите страницу, прежде чем перемещать их снова.",
    description:
      "Warning after an environment-group move failed and re-reading the groups' placement failed too; its action re-reads, it never moves again (w4/m206)",
  },
  "environments.assignEnvGroupsError": {
    message: "Не удалось обновить группы окружения для «{name}».",
    description:
      "Toast shown when environment-group membership fails to update",
  },
  "environments.settingsAction": {
    message: "Все настройки",
    description: "Environment overflow-menu action opening ACL settings",
  },
  "environments.protectedBadge": {
    message: "Защищено",
    description: "Badge shown beside a protected environment name",
  },
  "environments.ipAllowListLabel": {
    message: "Ограничения входящих IP",
    description: "Environment inbound-IP allowlist heading",
  },
  "environments.ipAllowListHint": {
    message:
      "Только перечисленные диапазоны CIDR могут обращаться к публичным сервисам, Static Site и хранилищам данных этого окружения. Источник также должен соответствовать собственным правилам IP каждого ресурса.",
    description: "Environment inbound-IP allowlist explanation",
  },
  "environments.ipAllowListOpen": {
    message:
      "Правил нет — публичные ресурсы этого окружения будут БЛОКИРОВАТЬ весь трафик. Добавьте 0.0.0.0/0 и ::/0, чтобы разрешить всё.",
    description: "Empty environment inbound-IP allowlist state (deny-all)",
  },
  "environments.ipAllowListRemove": {
    message: "Удалить {cidr}",
    description: "Accessible label for an inbound-IP rule remove button",
  },
  "environments.ipAllowListRuleCIDR": {
    message: "Блок CIDR для правила {number}",
    description: "Accessible label for an existing inbound-IP rule CIDR",
  },
  "environments.ipAllowListRuleDescription": {
    message: "Описание для правила {number}",
    description: "Accessible label for an existing inbound-IP rule description",
  },
  "environments.ipAllowListNewCIDR": {
    message: "Новый блок CIDR",
    description: "Accessible label for a new inbound-IP rule CIDR",
  },
  "environments.ipAllowListNewDescription": {
    message: "Описание нового правила",
    description: "Accessible label for a new inbound-IP rule description",
  },
  "environments.ipAllowListDescriptionPlaceholder": {
    message: "Описание (необязательно)",
    description: "Placeholder for an inbound-IP rule description",
  },
  "environments.ipAllowListScalePlanOnly": {
    message:
      "Правила входящих IP для окружений требуют тарифа Scale или выше. Уже заданные правила продолжают действовать и могут быть сброшены.",
    description:
      "Hint replacing the editable IP rules below the Scale plan (w8/m137)",
  },
  "environments.ipAllowListResetAllowAll": {
    message: "Сбросить и разрешить всё",
    description:
      "Button resetting an environment's inbound IP rules to allow-all below the Scale plan (w8/m137)",
  },
  "environments.ipAllowListInvalid": {
    message: "Введите уникальные корректные блоки CIDR IPv4 или IPv6.",
    description:
      "Inline validation when an environment allow list block is not a unique, valid CIDR",
  },
  "environments.ipAllowListAdd": {
    message: "Добавить источник",
    description: "Button adding an inbound-IP CIDR rule",
  },
  "environments.resourceCountLoading": {
    message: "Подсчёт…",
    description:
      "Environment card header while its resource lists are still loading, instead of a count",
  },
  "environments.resourceCountUnknown": {
    message: "Количество недоступно",
    description:
      "Environment card header when a resource list has not answered, instead of a count",
  },
  "environments.resourcesUnavailable": {
    message:
      "Некоторые ресурсы не удалось загрузить, поэтому это окружение может быть неполным или устаревшим.",
    description:
      "Inline alert on a Project environment card when a resource list failed to load; offers Try again",
  },
  "environments.resourcesLoading": {
    message: "Загрузка ресурсов…",
    description:
      "Environment card line while its resource lists are still loading, instead of the empty message",
  },
  "environments.assignmentTitle": {
    message: "Проект",
    description: "Shared create-form Project/Environment selector heading",
  },
  "environments.assignmentProject": {
    message: "Проект",
    description: "Accessible label for a create-form Project selector",
  },
  "environments.assignmentProjectNone": {
    message: "Без проекта",
    description: "Unassigned Project selector option",
  },
  "environments.assignmentEnvironment": {
    message: "Окружение",
    description: "Accessible label for a create-form Environment selector",
  },
  "environments.assignmentEnvironmentNone": {
    message: "Без окружения",
    description: "Unassigned Environment selector option",
  },
  "environments.assignmentSelectedLoading": {
    message: "Загрузка…",
    description:
      "Label for a kept Project/Environment selection while its list is still loading",
  },
  "environments.assignmentSelectedUnavailable": {
    message: "Выбрано (название недоступно)",
    description:
      "Label for a kept Project/Environment selection whose list failed to load",
  },
  "environments.assignmentReadError": {
    message:
      "Не удалось загрузить проекты или окружения. Повторите попытку; сделанный вами выбор сохранён.",
    description:
      "Inline alert when the create form's Project or Environment list fails to load; the selection is kept",
  },
  "environments.assignmentHint": {
    message:
      "При создании ресурса выбор проекта отфильтровывает его окружения. Выберите окружение, чтобы добавить ресурс в этот проект. Если оставить «Без окружения», ресурс будет создан вне проектов. Если в проекте ещё нет окружений, сначала создайте одно на странице проекта.",
    description: "Shared create-form Environment-assignment hint",
  },
  "environments.protectedPlacementAdminOnly": {
    message:
      "Только администратор может создавать ресурсы в защищённом окружении и перемещать их в него или из него.",
    description:
      "Why a create or move control that targets (or leaves) a protected environment is disabled for a non-admin, and the message for the same server refusal (w3/m281)",
  },
  "environments.protectedOptionAdminOnly": {
    message: "{environment} (только для администраторов)",
    description:
      "A protected environment in a create or move picker that a non-admin cannot choose ({environment} = its name)",
  },
  "environments.assignmentDescription": {
    message: "После создания он попадёт в выбранные проект и окружение.",
    description: "Shared create-form Project row description",
  },
  "environments.assignmentNoEnvironments": {
    message:
      "В этом проекте пока нет окружений. Сначала создайте окружение на странице проекта.",
    description:
      "Shared create-form note when the chosen Project has no Environment to add the resource to",
  },
  "environments.assignmentPickEnvironment": {
    message:
      "Выберите окружение, чтобы добавить ресурс в этот проект. С вариантом «Без окружения» он будет создан вне проектов.",
    description:
      "Shared create-form note when a Project is chosen but No environment is: only an Environment adds the resource to the Project",
  },
  "environments.assignmentEnvironmentPickProject": {
    message: "Сначала выберите проект",
    description:
      "Shared create-form Environment select while no Project is chosen (it is disabled until one is)",
  },
  "environments.protectedDescription": {
    message: "Защищённое окружение",
    description:
      "Line under a protected environment's section heading on the project page (Render's words, w1/m682)",
  },
  "environments.emptyTitle": {
    message: "Окружение {name} пусто",
    description:
      "Heading of an environment section with no services, Postgres or Key Value; {name} is the environment (Render's words, w1/m682)",
  },
  "environments.emptyBodyCreate": {
    message: "Начните работу с окружением: создайте новый сервис.",
    description:
      "An empty environment's sentence when the workspace has nothing to move into it (Render's words, w1/m682)",
  },
  "environments.emptyBodyCreateOrMove": {
    message:
      "Начните работу с окружением: создайте новый сервис или перенесите существующий.",
    description:
      "An empty environment's sentence when the workspace has resources that could move into it (Render's words, w1/m682)",
  },
  "environments.createNewService": {
    message: "Создать новый сервис",
    description:
      "An empty environment's button to the create flow with the project and environment chosen (Render's words, w1/m682)",
  },
  "environments.moveExistingServices": {
    message: "Перенести существующие сервисы",
    description:
      "An empty environment's button that moves existing resources into it (Render's words, w1/m682)",
  },
  "environments.noEnvGroups": {
    message: "В этом окружении нет групп окружения.",
    description:
      "An environment section's Env Groups view when the environment has none (Render's words, w1/m682)",
  },
  "environments.unassignedDescription": {
    message: "Ресурсы этого проекта, не входящие ни в одно окружение.",
    description:
      "Line under the project page's Unassigned section, which lists the project's resources in no environment (a bex section; Render requires an environment)",
  },
  "environments.settingsPageTitle": {
    message: "Настройки окружения",
    description:
      "Heading of an environment's settings page, /project/<id>/environment/<envId>/settings (Render's words, w1/m683)",
  },
  "environments.settingsNavigation": {
    message: "Разделы настроек окружения",
    description:
      "Accessible name of the environment settings page's section navigation (w1/m683)",
  },
  "environments.settingsEdit": {
    message: "Изменить",
    description:
      "Button on an environment settings card that makes its field editable (Render's words, w1/m683)",
  },
  "environments.settingsCardSave": {
    message: "Сохранить",
    description:
      "Button that saves an environment settings card's change (Render's words, w1/m683)",
  },
  "environments.settingsUpdateError": {
    message: "При обновлении окружения произошла ошибка. Попробуйте ещё раз.",
    description:
      "Inline error on an environment settings card when the save failed for a reason the server did not name (Render's words, w1/m683)",
  },
  "environments.nameCardTitle": {
    message: "Название",
    description:
      "Title of the environment settings page's Name card, and its navigation entry (Render's words, w1/m683)",
  },
  "environments.nameCardDescription": {
    message: "Уникальное название вашего окружения",
    description:
      "Description of the environment settings page's Name card (Render's words, w1/m683)",
  },
  "environments.nameFieldLabel": {
    message: "Название окружения",
    description:
      "Accessible label of the Name card's field (Render's words, w1/m683)",
  },
  "environments.nameRequired": {
    message: "Название окружения обязательно",
    description:
      "Error under the Name card's field when it is saved empty (Render's words, w1/m683)",
  },
  "environments.nameInUse": {
    message: "Это название уже используется другим окружением в этом проекте",
    description:
      "Error under the Name card's field when another environment of the project has the name (Render's words, w1/m683)",
  },
  "environments.permissionsCardTitle": {
    message: "Разрешения",
    description:
      "Title of the environment settings page's Permissions card, and its navigation entry (Render's words, w1/m683)",
  },
  "environments.permissionsDefaultBody": {
    message:
      "По умолчанию все участники команды могут удалять и приостанавливать ресурсы, перемещать сервисы и управлять сетью баз данных.",
    description:
      "First paragraph of the Permissions card (Render's words, w1/m683)",
  },
  "environments.permissionsProtectedBody": {
    message:
      "Если пометить окружение как {protectedEnvironment}, удалять и приостанавливать ресурсы, перемещать сервисы и управлять сетью баз данных смогут только администраторы.",
    description:
      "Second paragraph of the Permissions card; {protectedEnvironment} is a link whose text is environments.permissionsProtectedLink (Render's words, w1/m683)",
  },
  "environments.permissionsProtectedLink": {
    message: "защищённое окружение",
    description:
      "Link text inside environments.permissionsProtectedBody, to the projects and environments docs (Render's words, w1/m683)",
  },
  "environments.permissionsLegend": {
    message: "Разрешения окружения",
    description:
      "Accessible name of the Permissions card's Default / Protected choice (Render's words, w1/m683)",
  },
  "environments.permissionsDefault": {
    message: "По умолчанию",
    description:
      "The Permissions card's option for an unprotected environment (Render's words, w1/m683)",
  },
  "environments.permissionsDefaultHint": {
    message: "Для staging и разработки",
    description:
      "Line under the Permissions card's Default option (Render's words, w1/m683)",
  },
  "environments.permissionsProtected": {
    message: "Защищённое",
    description:
      "The Permissions card's option for a protected environment (Render's words, w1/m683)",
  },
  "environments.permissionsProtectedHint": {
    message: "Лучше всего для продакшена",
    description:
      "Line under the Permissions card's Protected option (Render's words, w1/m683)",
  },
  "environments.removeRestrictionsTitle": {
    message: "Снять ограничения",
    description:
      "Title of the dialog that confirms unprotecting an environment, or allowing its cross-environment connections again (Render's words, w1/m683)",
  },
  "environments.removeProtectionBody": {
    message:
      "Если снять ограничения для окружения {name}, все участники команды смогут выполнять деструктивные действия. Вы уверены, что хотите это сделать?",
    description:
      "Body of the dialog that confirms unprotecting an environment; {name} is the environment (Render's words, w1/m683)",
  },
  "environments.removeRestrictionsConfirm": {
    message: "Да, снять ограничения",
    description:
      "Confirm button that unprotects an environment, or allows its cross-environment connections again (Render's words, w1/m683)",
  },
  "environments.connectionsNavLabel": {
    message: "Подключения",
    description:
      "Navigation entry of the environment settings page's Cross-Environment Connections card (Render's words, w1/m683)",
  },
  "environments.connectionsCardTitle": {
    message: "Подключения между окружениями",
    description:
      "Title of the environment settings page's connections card (Render's words, w1/m683)",
  },
  "environments.connectionsBody": {
    message:
      "Запретите трафику частной сети пересекать границу окружения, сохранив трафик между сервисами этого окружения.",
    description:
      "First paragraph of the Cross-Environment Connections card (Render's words, w1/m683)",
  },
  "environments.connectionsBodyOutside": {
    message:
      "Сервисы в этом окружении не смогут взаимодействовать с сервисами bex за его пределами.",
    description:
      "Second paragraph of the Cross-Environment Connections card (Render's words, with bex for Render, w1/m683)",
  },
  "environments.connectionsLegend": {
    message: "Подключения между окружениями",
    description:
      "Accessible name of the Cross-Environment Connections card's Allowed / Blocked choice (w1/m683)",
  },
  "environments.connectionsAllowed": {
    message: "Разрешены",
    description:
      "The Cross-Environment Connections card's option for an environment whose private traffic may cross its boundary (Render's words, w1/m683)",
  },
  "environments.connectionsAllowedHint": {
    message: "По умолчанию для новых окружений",
    description:
      "Line under the Cross-Environment Connections card's Allowed option (Render's words, w1/m683)",
  },
  "environments.connectionsBlocked": {
    message: "Заблокированы",
    description:
      "The Cross-Environment Connections card's option for an isolated environment (Render's words, w1/m683)",
  },
  "environments.connectionsBlockedHint": {
    message: "Рекомендуется для продакшен-окружений",
    description:
      "Line under the Cross-Environment Connections card's Blocked option (Render's words, w1/m683)",
  },
  "environments.connectionsPaywall": {
    message:
      "Перейдите на тариф {plan}, чтобы блокировать подключения между окружениями.",
    description:
      "Notice on the Cross-Environment Connections card below Pro; {plan} is the Pro plan's name (Render's words, w1/m683)",
  },
  "environments.upgrade": {
    message: "Повысить тариф",
    description:
      "Button or link that opens Render's plan page from an environment's plan gate (Render's words, w1/m683, w3/m708)",
  },
  "environments.isolateTitle": {
    message: "Ограничить подключения",
    description:
      "Title of the dialog that confirms blocking an environment's cross-environment connections, and its confirm button (Render's words, w1/m683)",
  },
  "environments.isolateDescription": {
    message:
      "Подтвердите, что хотите изолировать подключения для {environment}",
    description:
      "Description of the dialog that confirms blocking or allowing an environment's cross-environment connections; {environment} is <project> / <environment> (Render's words, w1/m683)",
  },
  "environments.isolateRestrictBody": {
    message:
      "Трафик частной сети между этим окружением и вашими сервисами bex за его пределами будет заблокирован. Это может нарушить работу сервисов, которые зависят от трафика между окружениями.",
    description:
      "First paragraph of the dialog that confirms blocking cross-environment connections (Render's words, with bex for Render, w1/m683)",
  },
  "environments.isolateRestrictRestart": {
    message:
      "Уже открытые сетевые подключения не затрагиваются. Перезапустите сервисы в этом окружении, чтобы гарантированно закрыть все существующие подключения.",
    description:
      "Second paragraph of the dialog that confirms blocking cross-environment connections (Render's words, w1/m683)",
  },
  "environments.isolateRemoveBody": {
    message:
      "Трафик частной сети между этим окружением и вашими сервисами bex за его пределами больше не будет блокироваться.",
    description:
      "Body of the dialog that confirms allowing cross-environment connections again (Render's words, with bex for Render, w1/m683)",
  },
  "environments.isolateQuestion": {
    message: "Вы уверены, что хотите это сделать?",
    description:
      "Last line of the dialog that confirms blocking or allowing cross-environment connections (Render's words, w1/m683)",
  },
  "environments.isolationBlockedToast": {
    message: "Подключения между окружениями для {name} заблокированы.",
    description:
      "Toast after the section menu's toggle blocks an environment's cross-environment connections (Render's words, w1/m683)",
  },
  "environments.isolationAllowedToast": {
    message: "Подключения между окружениями для {name} разрешены.",
    description:
      "Toast after the section menu's toggle allows an environment's cross-environment connections again (Render's words, w1/m683)",
  },
  "environments.menuBlockConnections": {
    message: "Блокировать подключения между окружениями",
    description:
      "The environment section's ••• menu toggle that isolates the environment (Render's words, w1/m683)",
  },
  "environments.menuBlockConnectionsHint": {
    message: "Запретить трафику частной сети пересекать границу окружения.",
    description:
      "Line under the ••• menu's Block cross-environment connections toggle (Render's words, w1/m683)",
  },
  "environments.menuUpgradeToUnlock": {
    message: "{upgrade}, чтобы разблокировать эту функцию",
    description:
      "The ••• menu's plan gate on Block cross-environment connections below Pro; {upgrade} is a link whose text is environments.upgrade (Render's words, w1/m683)",
  },
  "environments.envGroupsCardTitle": {
    message: "Группы окружения",
    description:
      "Title of the environment settings page's Environment Groups card, and its navigation entry (Render's words, w1/m683)",
  },
  "environments.envGroupsNew": {
    message: "Новая группа окружения",
    description:
      "Button on the Environment Groups card that creates an env group in the environment (Render's words, w1/m683)",
  },
  "environments.envGroupsSearch": {
    message: "Поиск групп окружения",
    description:
      "Placeholder and accessible name of the Environment Groups card's search (Render's words, w1/m683)",
  },
  "environments.envGroupsNone": {
    message: "Группы окружения не найдены",
    description:
      "The Environment Groups card when the environment has no env groups (Render's words, w1/m683)",
  },
  "environments.networkingCardTitle": {
    message: "Сеть",
    description:
      "Title of the environment settings page's Networking card (inbound IP rules), and its navigation entry (Render's words, w1/m683)",
  },
  "environments.ipAllowListMoveUp": {
    message: "Переместить {cidr} вверх",
    description:
      "Accessible name of an environment IP rule's move-up button; {cidr} is the rule's CIDR block",
  },
  "environments.ipAllowListMoveDown": {
    message: "Переместить {cidr} вниз",
    description:
      "Accessible name of an environment IP rule's move-down button; {cidr} is the rule's CIDR block",
  },
  "environments.ipAllowListAllowsAll": {
    message: "Открыто для всех исходных IP",
    description:
      "The Networking card below Scale when the environment's rules are the allow-all pair",
  },
  "environments.ipAllowListSaved": {
    message: "Правила входящих IP для {name} сохранены.",
    description:
      "Toast after the Networking card saves an environment's inbound IP rules",
  },
  "environments.deleteCardTitle": {
    message: "Удалить окружение",
    description:
      "Title of the environment settings page's delete card, and its navigation entry (Render's words, w1/m683)",
  },
  "environments.deleteCardDescription": {
    message:
      "Это окружение вместе с его сервисами будет удалено навсегда. Это действие нельзя отменить.",
    description:
      "Description of the Delete Environment card (Render's words, w1/m683)",
  },
  "environments.deleteLastEnvironment": {
    message:
      "Это окружение нельзя удалить: в проекте должно быть хотя бы одно окружение. Вместо этого можно удалить {project}.",
    description:
      "Notice on the Delete Environment card for a project's only environment; {project} is a link whose text is environments.deleteLastEnvironmentProject (Render's words, w1/m683)",
  },
  "environments.deleteLastEnvironmentProject": {
    message: "проект",
    description:
      "Link text inside environments.deleteLastEnvironment, to the project's settings (Render's words, w1/m683)",
  },
  "environments.deleteButton": {
    message: "Удалить окружение",
    description:
      "The Delete Environment card's button, the confirmation dialog's title and its confirm button (Render's words, w1/m683)",
  },
  "environments.deleteDialogBody": {
    message:
      "Это окружение будет удалено навсегда вместе со всеми его сервисами. Чтобы сохранить какие-либо сервисы, сначала перенесите их из окружения.",
    description:
      "Body of the dialog that confirms deleting an environment (Render's words, w1/m683)",
  },
  "environments.deleteDialogEnvGroupsKept": {
    message:
      "Его группы окружения сохранятся, но станут неназначенными: они больше не относятся к этому окружению или проекту и останутся в разделе «Группы окружения».",
    description:
      "Added to the delete dialog's body when the environment holds env groups, which bex keeps (w1/m683)",
  },
  "environments.deleteFailed": {
    message:
      "Не удалось удалить окружение. Попробуйте ещё раз чуть позже. Извините!",
    description:
      "Error in the delete dialog when the delete failed for a reason the server did not name (Render's words, w1/m683)",
  },
  "environments.deleteCardDenied": {
    message: "Удаление окружения",
    description:
      "Title of the card that replaces Delete Environment for a member who may not delete the environment (Render's words, w1/m683)",
  },
  "environments.protectedDeleteAdminOnly": {
    message:
      "Только администратор может удалить защищённое окружение или окружение с изоляцией сети или правилами входящих IP.",
    description:
      "Why a non-Admin cannot delete an environment that is protected, isolated or has inbound IP rules (w1/m683)",
  },
};

export default ruEnvironments;
