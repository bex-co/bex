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
  "environments.renameAction": {
    message: "Переименовать",
    description: "Environment overflow-menu item that opens the rename dialog",
  },
  "environments.deleteAction": {
    message: "Удалить",
    description:
      "Environment overflow-menu item that opens the delete confirmation, and the confirm button",
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
  "environments.renameFieldLabel": {
    message: "Название окружения",
    description:
      'Accessible/visible label for the environment rename field. Deliberately more specific than environments.fieldName\'s bare "Name" (w4/134)',
  },
  "environments.renameTitle": {
    message: "Переименовать окружение",
    description: "Rename-environment dialog title",
  },
  "environments.renameSubmit": {
    message: "Сохранить",
    description: "Rename-environment dialog submit button",
  },
  "environments.renameSuccess": {
    message: "Окружение переименовано в «{name}».",
    description: "Toast shown after an environment is renamed",
  },
  "environments.renameError": {
    message: "Не удалось переименовать окружение в «{name}».",
    description: "Toast shown when renaming an environment fails",
  },
  "environments.deleteConfirmTitle": {
    message: "Удалить окружение «{name}»?",
    description: "Delete-environment confirmation dialog title",
  },
  "environments.deleteConfirmBody": {
    message:
      "Его группы окружения сохранятся, но станут неназначенными: они больше не относятся к этому окружению или проекту и останутся в разделе «Группы окружения». Это действие нельзя отменить.",
    description: "Delete-environment confirmation dialog body",
  },
  "environments.deleteConfirmBodyResources": {
    message:
      "При удалении этого окружения будут удалены все его сервисы, базы данных и хранилища Key Value: {resources}. Его группы окружения сохранятся, но станут неназначенными: они больше не относятся к этому окружению или проекту и останутся в разделе «Группы окружения». Это действие нельзя отменить.",
    description:
      "Delete-environment confirmation body when services, databases or key value stores would be deleted (w4/m264, w6/m290, Render parity)",
  },
  "environments.deleteSuccess": {
    message: "Окружение «{name}» удалено.",
    description: "Toast shown after an environment is deleted",
  },
  "environments.deleteRefreshFailed": {
    message:
      "Окружение «{name}» удалено, но список не удалось обновить. Повторите попытку или перезагрузите страницу, чтобы обновить его.",
    description:
      "Warning after the environment was deleted but re-reading the list failed; deletion must not be retried or reported as failed (w4/290)",
  },
  "environments.deleteError": {
    message: "Не удалось удалить окружение «{name}».",
    description: "Toast shown when deleting an environment fails",
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
  "environments.settingsTitle": {
    message: "Настройки «{name}»",
    description: "Environment ACL settings dialog title",
  },
  "environments.settingsDescription": {
    message:
      "Управляйте разрешениями, изоляцией частной сети и правилами входящих IP.",
    description: "Environment ACL settings dialog description",
  },
  "environments.protectedBadge": {
    message: "Защищено",
    description: "Badge shown beside a protected environment name",
  },
  "environments.protectedLabel": {
    message: "Защищённое окружение",
    description: "Protected-status setting label",
  },
  "environments.protectedHint": {
    message:
      "Только администраторы рабочего пространства могут удалять, приостанавливать и возобновлять ресурсы этого окружения или включать и выключать для них режим обслуживания.",
    description: "Protected-status setting explanation",
  },
  "environments.isolationLabel": {
    message: "Блокировать подключения между окружениями",
    description: "Private-network isolation setting label",
  },
  "environments.isolationProOnly": {
    message:
      "Для изоляции сети нужен тариф Pro или выше. Отключите её здесь, если она была включена ранее.",
    description: "Isolation setting explanation on a Hobby workspace (w8/m138)",
  },
  "environments.isolationHint": {
    message:
      "Не позволяет трафику частной сети пересекать границу этого окружения, в том числе к его инстансам Postgres и Key Value. Их внешние URL по-прежнему подчиняются их правилам IP.",
    description: "Private-network isolation setting explanation",
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
  "environments.settingsSave": {
    message: "Сохранить",
    description: "Environment ACL settings save button",
  },
  "environments.aclSaveSuccess": {
    message: "Настройки «{name}» сохранены.",
    description: "Toast shown after an Environment ACL is saved",
  },
  "environments.aclSaveError": {
    message: "Не удалось сохранить настройки «{name}».",
    description: "Toast shown when an Environment ACL save fails",
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
};

export default ruEnvironments;
