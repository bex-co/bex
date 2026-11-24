import type { TranslationEntry } from "@/i18n";

const ruBuildSources: Record<string, TranslationEntry> = {
  "buildSources.pageTitle": {
    message: "Источники сборки",
    description: "Title of the Build Sources list page",
  },
  "buildSources.pageDescription": {
    message:
      "Собирайте один раз из репозитория Git или образа Docker и разворачивайте артефакт сразу на нескольких сервисах.",
    description: "Subtitle of the Build Sources list page",
  },
  "buildSources.resourceType": {
    message: "Источник сборки",
    description: "Singular resource type label shown in global search results",
  },
  "buildSources.newButton": {
    message: "Новый источник сборки",
    description: "Primary CTA button to create a new Build Source",
  },
  "buildSources.backToList": {
    message: "Назад к источникам сборки",
    description: "Back button label returning to /build-sources",
  },
  "buildSources.viewAll": {
    message: "Показать все",
    description: "Link from Project Overview Build Sources section to /build-sources",
  },
  "buildSources.viewBuildSources": {
    message: "Открыть источники сборки",
    description: "Button in partial-link alert navigating to /build-sources",
  },
  "buildSources.emptyTitle": {
    message: "Источников сборки пока нет",
    description: "Empty state heading when the workspace has no build sources",
  },
  "buildSources.emptyDescription": {
    message:
      "Создайте источник сборки, чтобы использовать одну сборку Git или образ Docker сразу для Web Service, Private Service, Background Worker и Cron Job.",
    description: "Empty state explanation on /build-sources",
  },
  "buildSources.refreshError": {
    message:
      "Не удалось обновить источники сборки. Показаны кэшированные данные.",
    description: "Inline error banner when a background refresh fails over cached rows",
  },
  "buildSources.searchPlaceholder": {
    message: "Поиск по имени, репозиторию, образу или проекту…",
    description: "Placeholder for the /build-sources search input",
  },
  "buildSources.searchLabel": {
    message: "Поиск источников сборки",
    description: "Accessible label for the /build-sources search input",
  },
  "buildSources.resetFilters": {
    message: "Сбросить фильтры",
    description: "Button to clear active search and kind filters on /build-sources",
  },
  "buildSources.filterKindLabel": {
    message: "Фильтр по типу источника",
    description: "Accessible label for the source-kind filter button group",
  },
  "buildSources.filterAll": {
    message: "Все",
    description: "Filter button showing both Git and Docker image build sources",
  },
  "buildSources.kindGit": {
    message: "Git",
    description: "Source kind badge and filter label for Git-backed build sources",
  },
  "buildSources.kindImage": {
    message: "Образ Docker",
    description: "Source kind badge and filter label for Docker-image-backed build sources",
  },
  "buildSources.noMatchesTitle": {
    message: "Подходящих источников сборки нет",
    description: "Heading when search or kind filter matches zero build sources",
  },
  "buildSources.noMatchesDescription": {
    message: "Попробуйте изменить поисковый запрос или сбросить фильтр типа.",
    description: "Body copy when search or kind filter matches zero build sources",
  },
  "buildSources.colName": {
    message: "Имя",
    description: "Table column header for Build Source name",
  },
  "buildSources.colSource": {
    message: "Источник",
    description: "Table column header for Git repo/branch or Docker image",
  },
  "buildSources.colRegion": {
    message: "Регион",
    description: "Table column header for Build Source region",
  },
  "buildSources.colLinkedServices": {
    message: "Связанные сервисы",
    description: "Table column header for linked service count",
  },
  "buildSources.colProject": {
    message: "Проект",
    description: "Table column header for optional project scope",
  },
  "buildSources.colUpdated": {
    message: "Обновлено",
    description: "Table column header for last updated timestamp",
  },
  "buildSources.copyId": {
    message: "Скопировать ID источника сборки",
    description: "Accessible label for copying a bds- ID",
  },
  "buildSources.copied": {
    message: "Скопировано",
    description: "Copy button confirmation tooltip",
  },
  "buildSources.copyError": {
    message: "Не удалось скопировать",
    description: "Copy button failure tooltip",
  },
  "buildSources.linkedServicesCount_one": {
    message: "{count} сервис",
    description: "One linked service",
  },
  "buildSources.linkedServicesCount_few": {
    message: "{count} сервиса",
    description: "Few linked services",
  },
  "buildSources.linkedServicesCount_many": {
    message: "{count} сервисов",
    description: "Many linked services",
  },
  "buildSources.linkedServicesCount_other": {
    message: "{count} сервиса",
    description: "Other linked services",
  },
  "buildSources.workspaceScope": {
    message: "Рабочее пространство",
    description: "Label when a build source is workspace-wide (not scoped to a project)",
  },
  "buildSources.createTitle": {
    message: "Новый источник сборки",
    description: "Page heading of /build-sources/new",
  },
  "buildSources.createDescription": {
    message:
      "Настройте общую сборку из репозитория Git или образ Docker, из которого смогут разворачиваться несколько сервисов.",
    description: "Subtitle of /build-sources/new",
  },
  "buildSources.createSubmit": {
    message: "Создать источник сборки",
    description: "Submit button label on /build-sources/new",
  },
  "buildSources.createSuccess": {
    message: 'Источник сборки «{name}» создан',
    description: "Toast shown after creating a Build Source",
  },
  "buildSources.createError": {
    message: "Не удалось создать источник сборки. Попробуйте снова.",
    description: "Fallback error toast when createBuildSource fails",
  },
  "buildSources.updateSuccess": {
    message: 'Источник сборки «{name}» обновлён',
    description: "Toast shown after updating a Build Source",
  },
  "buildSources.updateError": {
    message: "Не удалось обновить источник сборки. Попробуйте снова.",
    description: "Fallback error toast when updateBuildSource fails",
  },
  "buildSources.deleteSuccess": {
    message: 'Источник сборки «{name}» удалён',
    description: "Toast shown after deleting a Build Source",
  },
  "buildSources.deleteError": {
    message: "Не удалось удалить источник сборки. Попробуйте снова.",
    description: "Fallback error toast when deleteBuildSource fails",
  },
  "buildSources.triggerBuildSuccess": {
    message: "Сборка запущена",
    description: "Toast shown after triggering a Build Source build",
  },
  "buildSources.triggerBuildError": {
    message: "Не удалось запустить сборку. Попробуйте снова.",
    description: "Fallback error toast when triggerBuild fails",
  },
  "buildSources.createPermissionDenied": {
    message:
      "Для создания источника сборки требуются права Developer или Admin в этом рабочем пространстве.",
    description: "Read-only notice shown on /build-sources/new when can_create is false",
  },
  "buildSources.createBlockedSource": {
    message: "Выберите репозиторий Git или образ Docker, чтобы продолжить.",
    description: "Action bar hint while no source is selected",
  },
  "buildSources.partiallyLinkedTitle": {
    message: "Источник сборки создан с частичной привязкой сервисов",
    description: "Alert heading when createBuildSource returns 409 BUILD_SOURCE_PARTIALLY_LINKED",
  },
  "buildSources.partiallyLinkedDescription": {
    message:
      "Источник сборки {id} создан, но привязать все выбранные сервисы не удалось.",
    description: "Alert summary naming the newly created bds- ID on partial link conflict",
  },
  "buildSources.partiallyLinkedSucceeded": {
    message: "Привязаны: {services}",
    description: "List of services that were successfully linked before the conflict",
  },
  "buildSources.partiallyLinkedFailed": {
    message: "Ошибка привязки: {service}",
    description: "The service whose link failed during create",
  },
  "buildSources.fieldSource": {
    message: "Источник",
    description: "Create form label for Git repository vs Docker image picker",
  },
  "buildSources.fieldSourceDescription": {
    message:
      "Выберите подключённый репозиторий GitHub, публичный URL Git или существующий образ Docker.",
    description: "Help text for the source picker row on /build-sources/new",
  },
  "buildSources.fieldName": {
    message: "Имя",
    description: "Create form label for Build Source name",
  },
  "buildSources.fieldNameDescription": {
    message: "Уникальное имя в пределах этого рабочего пространства.",
    description: "Help text for Build Source name",
  },
  "buildSources.fieldNamePlaceholder": {
    message: "my-shared-api",
    description: "Placeholder for Build Source name input",
  },
  "buildSources.errorNameTaken": {
    message:
      'Источник сборки с именем «{name}» уже существует в этом рабочем пространстве.',
    description: "Inline error when BUILD_SOURCE_NAME_TAKEN is returned",
  },
  "buildSources.fieldProject": {
    message: "Проект",
    description: "Create form label for optional Project scope",
  },
  "buildSources.fieldProjectDescription": {
    message: "При необходимости ограничьте источник сборки одним проектом.",
    description: "Help text for optional Project selector",
  },
  "buildSources.fieldProjectNone": {
    message: "Нет (на всё рабочее пространство)",
    description: "Option leaving the build source unscoped at workspace level",
  },
  "buildSources.fieldRuntime": {
    message: "Среда выполнения",
    description: "Create form label for Git build source runtime",
  },
  "buildSources.fieldRuntimeDescription": {
    message: "Выберите встроенную среду выполнения или Docker.",
    description: "Help text for Git build source runtime selector",
  },
  "buildSources.fieldBranch": {
    message: "Ветка",
    description: "Create form label for Git branch",
  },
  "buildSources.fieldBranchDescription": {
    message:
      "Ветка Git для сборки. По умолчанию используется основная ветка репозитория.",
    description: "Help text for Git branch input",
  },
  "buildSources.fieldRegion": {
    message: "Регион",
    description: "Create form label for Git build source region",
  },
  "buildSources.fieldRegionDescription": {
    message:
      "Регион источника сборки Git должен совпадать с регионом связанных сервисов.",
    description: "Help text for Git build source region selector",
  },
  "buildSources.fieldRegionDefault": {
    message: "Регион рабочего пространства по умолчанию",
    description: "Option leaving the build source region unset to use the platform default",
  },
  "buildSources.fieldGitRegistryDescription": {
    message:
      "Необязательные учётные данные для загрузки приватных базовых образов при сборке Docker.",
    description: "Help text for registry credential on Docker Git build sources",
  },
  "buildSources.fieldEnvGroups": {
    message: "Группы окружения для сборки",
    description: "Label for selecting workspace env groups in build-time environment",
  },
  "buildSources.fieldEnvGroupsDescription": {
    message:
      "Подключите группы окружения уровня рабочего пространства к среде сборки. Группы, привязанные к окружению, недоступны.",
    description: "Help text for build-time env group checkboxes",
  },
  "buildSources.fieldEnvGroupsEmpty": {
    message: "Групп окружения уровня рабочего пространства нет.",
    description: "Empty message when no workspace-level env groups exist",
  },
  "buildSources.linkServicesTitle": {
    message: "Привязать существующие сервисы",
    description: "Section heading for optional initial serviceIds linking",
  },
  "buildSources.linkServicesDescription": {
    message:
      "При необходимости привяжите существующие Web Service, Private Service, Background Worker или Cron Job. Static Site не поддерживают источники сборки.",
    description: "Section description for optional initial serviceIds linking",
  },
  "buildSources.linkServicesEmpty": {
    message: "В этом рабочем пространстве пока нет сервисов для привязки.",
    description: "Empty message when workspace has no linkable services",
  },
  "buildSources.projectSectionTitle": {
    message: "Источники сборки",
    description: "Heading of the Build Sources section on Project Overview",
  },
  "buildSources.projectLoadError": {
    message: "Не удалось загрузить источники сборки для этого проекта.",
    description: "Error text inside Project Overview Build Sources section",
  },
  "buildSources.projectEmptyDescription": {
    message: "К этому проекту пока не привязаны источники сборки.",
    description: "Empty state text inside Project Overview Build Sources section",
  },
  "buildSources.viewBuildSource": {
    message: "Открыть источник сборки",
    description: "Button navigating to a newly created Build Source detail page",
  },
  "buildSources.detailTabsLabel": {
    message: "Разделы источника сборки",
    description: "Accessible label for the Build Source detail tab navigation",
  },
  "buildSources.tabBuilds": {
    message: "Сборки",
    description: "Tab label for Build Source build history",
  },
  "buildSources.tabServices": {
    message: "Сервисы",
    description: "Tab label for Build Source linked services",
  },
  "buildSources.tabEnvironment": {
    message: "Окружение",
    description: "Tab label for Build Source build-time environment",
  },
  "buildSources.tabSettings": {
    message: "Настройки",
    description: "Tab label for Build Source settings and danger zone",
  },
  "buildSources.triggerBuildButton": {
    message: "Запустить сборку",
    description: "Primary CTA to trigger a new build on a Git build source",
  },
  "buildSources.triggerSubmitting": {
    message: "Запуск…",
    description: "Submit button label while triggerBuild mutation is in flight",
  },
  "buildSources.triggerDialogTitle": {
    message: "Запустить сборку",
    description: "Title of the Trigger Build modal dialog",
  },
  "buildSources.triggerDialogDescription": {
    message: "Запустите новую сборку для «{name}» и выберите, какие связанные сервисы развернуть после завершения.",
    description: "Description of the Trigger Build modal dialog",
  },
  "buildSources.triggerScopeLegend": {
    message: "Развернуть после успешной сборки",
    description: "Legend for the deploy.serviceScope radio group",
  },
  "buildSources.triggerScopeAutoDeploy": {
    message: "Авторазвёртывание связанных сервисов",
    description: "Radio option for deploy.serviceScope=autodeploy_only",
  },
  "buildSources.triggerScopeAutoDeployHint": {
    message: "Развернуть все связанные сервисы с включённым авторазвёртыванием.",
    description: "Hint text for deploy.serviceScope=autodeploy_only",
  },
  "buildSources.triggerScopeNone": {
    message: "Только сборка (без развёртывания)",
    description: "Radio option for deploy.serviceScope=none",
  },
  "buildSources.triggerScopeNoneHint": {
    message: "Собрать артефакт без развёртывания связанных сервисов.",
    description: "Hint text for deploy.serviceScope=none",
  },
  "buildSources.triggerScopeCustom": {
    message: "Выбранные сервисы",
    description: "Radio option for deploy.serviceScope=custom",
  },
  "buildSources.triggerScopeCustomHint": {
    message: "Выберите конкретные связанные сервисы для развёртывания после успешной сборки.",
    description: "Hint text for deploy.serviceScope=custom",
  },
  "buildSources.triggerScopeCustomNoServices": {
    message: "К этому источнику сборки ещё не привязаны сервисы.",
    description: "Message inside custom deploy scope when serviceLinks is empty",
  },
  "buildSources.triggerScopeCustomSelectOne": {
    message: "Выберите хотя бы один сервис для развёртывания.",
    description: "Validation hint when custom scope has zero checked services",
  },
  "buildSources.triggerClearCache": {
    message: "Очистить кеш сборки",
    description: "Checkbox label for clearCache=clear",
  },
  "buildSources.triggerClearCacheHint": {
    message: "Выполнить чистую сборку без повторного использования кешированных слоёв.",
    description: "Hint text for clearCache checkbox",
  },
  "buildSources.triggerResultTitle": {
    message: "Сборка запущена",
    description: "Confirmation heading in Trigger Build dialog after success",
  },
  "buildSources.triggerResultDescription": {
    message: "Сборка {id} поставлена в очередь.",
    description: "Confirmation summary naming the new bld- ID",
  },
  "buildSources.triggerResultDeploysTitle_one": {
    message: "В очереди {count} развёртывание сервиса",
    description: "Singular heading listing fan-out deploys queued by triggerBuild",
  },
  "buildSources.triggerResultDeploysTitle_few": {
    message: "В очереди {count} развёртывания сервисов",
    description: "Few heading listing fan-out deploys queued by triggerBuild",
  },
  "buildSources.triggerResultDeploysTitle_many": {
    message: "В очереди {count} развёртываний сервисов",
    description: "Many heading listing fan-out deploys queued by triggerBuild",
  },
  "buildSources.triggerResultDeploysTitle_other": {
    message: "В очереди {count} развёртывания сервисов",
    description: "Other heading listing fan-out deploys queued by triggerBuild",
  },
  "buildSources.triggerResultNoDeploys": {
    message: "Для этой сборки не запланировано ни одного развёртывания сервиса.",
    description: "Message when triggerBuild returned an empty deploys array",
  },
  "buildSources.triggerResultSkippedTitle_one": {
    message: "Пропущен {count} связанный сервис",
    description: "Singular alert heading when triggerBuild returns skippedServices",
  },
  "buildSources.triggerResultSkippedTitle_few": {
    message: "Пропущено {count} связанных сервиса",
    description: "Few alert heading when triggerBuild returns skippedServices",
  },
  "buildSources.triggerResultSkippedTitle_many": {
    message: "Пропущено {count} связанных сервисов",
    description: "Many alert heading when triggerBuild returns skippedServices",
  },
  "buildSources.triggerResultSkippedTitle_other": {
    message: "Пропущено {count} связанных сервисов",
    description: "Other alert heading when triggerBuild returns skippedServices",
  },
  "buildSources.triggerResultViewLogs": {
    message: "Открыть логи сборки",
    description: "Primary action navigating from Trigger Build confirmation to Build Detail",
  },
  "buildSources.skippedReasonSuspended": {
    message: "Приостановлен",
    description: "Badge label when a service was skipped because it is suspended",
  },
  "buildSources.buildStatusSucceeded": {
    message: "Успешно",
    description: "Build status badge for succeeded builds",
  },
  "buildSources.buildStatusFailed": {
    message: "Ошибка",
    description: "Build status badge for failed builds",
  },
  "buildSources.buildStatusCanceled": {
    message: "Отменена",
    description: "Build status badge for canceled builds",
  },
  "buildSources.buildStatusInProgress": {
    message: "Сборка",
    description: "Build status badge for in-progress builds",
  },
  "buildSources.buildStatusCreated": {
    message: "В очереди",
    description: "Build status badge for newly queued builds",
  },
  "buildSources.buildStatusUnknown": {
    message: "Неизвестно",
    description: "Fallback build status badge",
  },
  "buildSources.buildsEmptyTitle": {
    message: "Сборок пока нет",
    description: "Empty state heading on the Builds tab",
  },
  "buildSources.buildsEmptyGitDescription": {
    message: "Запустите сборку вручную или отправьте изменения в отслеживаемую ветку, чтобы получить первый артефакт.",
    description: "Empty state description on Builds tab for Git build sources",
  },
  "buildSources.buildsEmptyImageDescription": {
    message: "Источники сборки из Docker-образа фиксируют версии образов при развёртывании связанных сервисов.",
    description: "Empty state description on Builds tab for image build sources",
  },
  "buildSources.buildsRefreshError": {
    message: "Не удалось обновить список сборок. Показаны кешированные данные.",
    description: "Inline error banner when polling builds fails over cached rows",
  },
  "buildSources.buildsLoadMore": {
    message: "Загрузить ещё сборки",
    description: "Pagination button loading older builds",
  },
  "buildSources.colBuildId": {
    message: "ID сборки",
    description: "Table header for build ID column",
  },
  "buildSources.colStatus": {
    message: "Статус",
    description: "Table header for build status column",
  },
  "buildSources.colArtifact": {
    message: "Коммит / Образ",
    description: "Table header for commit SHA or image digest column",
  },
  "buildSources.colDuration": {
    message: "Длительность",
    description: "Table header for build duration column",
  },
  "buildSources.colDeploys": {
    message: "Развёртывания",
    description: "Table header for fan-out deploys count column",
  },
  "buildSources.colStarted": {
    message: "Запущена",
    description: "Table header for build start timestamp column",
  },
  "buildSources.buildDeploysCount_one": {
    message: "{count} развёртывание",
    description: "Singular badge count of fan-out deploys on a build",
  },
  "buildSources.buildDeploysCount_few": {
    message: "{count} развёртывания",
    description: "Few badge count of fan-out deploys on a build",
  },
  "buildSources.buildDeploysCount_many": {
    message: "{count} развёртываний",
    description: "Many badge count of fan-out deploys on a build",
  },
  "buildSources.buildDeploysCount_other": {
    message: "{count} развёртывания",
    description: "Other badge count of fan-out deploys on a build",
  },
  "buildSources.buildSkippedCount_one": {
    message: "пропущен {count}",
    description: "Singular badge count of skipped services on a build",
  },
  "buildSources.buildSkippedCount_few": {
    message: "пропущено {count}",
    description: "Few badge count of skipped services on a build",
  },
  "buildSources.buildSkippedCount_many": {
    message: "пропущено {count}",
    description: "Many badge count of skipped services on a build",
  },
  "buildSources.buildSkippedCount_other": {
    message: "пропущено {count}",
    description: "Other badge count of skipped services on a build",
  },
  "buildSources.backToBuilds": {
    message: "Назад к сборкам",
    description: "Back link from Build Detail to the Builds tab",
  },
  "buildSources.copyBuildId": {
    message: "Скопировать ID сборки",
    description: "Accessible label for copying a bld- ID",
  },
  "buildSources.buildDetailSubtitle": {
    message: "Сборка для {name}",
    description: "Subtitle under the bld- ID on Build Detail",
  },
  "buildSources.buildDetailRefreshError": {
    message: "Не удалось обновить сведения о сборке. Показано последнее загруженное состояние.",
    description: "Inline error banner when build detail polling fails",
  },
  "buildSources.factStarted": {
    message: "Запущена",
    description: "Fact label for build start timestamp",
  },
  "buildSources.factCommit": {
    message: "Коммит",
    description: "Fact label for build commit SHA",
  },
  "buildSources.factArtifact": {
    message: "Артефакт",
    description: "Fact label for built image reference and digest",
  },
  "buildSources.fanoutTitle": {
    message: "Развёртывания связанных сервисов",
    description: "Card heading listing fan-out deploys on Build Detail",
  },
  "buildSources.fanoutEmpty": {
    message: "Из этой сборки ещё не развёрнут ни один сервис.",
    description: "Empty message inside fan-out deploys card",
  },
  "buildSources.buildLogsSearchLabel": {
    message: "Фильтровать логи сборки",
    description: "Accessible label for search input in BuildLogPanel",
  },
  "buildSources.buildLogsEmptyTitle": {
    message: "Логов сборки пока нет",
    description: "Empty state heading in BuildLogPanel",
  },
  "buildSources.buildLogsEmptyBody": {
    message: "Логи этой сборки появятся здесь, как только сборка начнёт выводить сообщения.",
    description: "Empty state body in BuildLogPanel",
  },
  "buildSources.imageBuildLogsNotice": {
    message: "Источники сборки из Docker-образа используют готовый образ и не создают логов сборки.",
    description: "Notice shown on Build Detail for image-backed build sources",
  },
  "buildSources.servicesTabTitle": {
    message: "Связанные сервисы",
    description: "Heading of the Linked Services tab",
  },
  "buildSources.servicesTabDescription": {
    message: "Сервисы, привязанные к этому источнику сборки, развёртывают артефакты его сборок.",
    description: "Description of the Linked Services tab",
  },
  "buildSources.linkExistingService": {
    message: "Привязать существующий сервис",
    description: "Button opening the Link Existing Service dialog",
  },
  "buildSources.servicesEmptyTitle": {
    message: "Нет связанных сервисов",
    description: "Empty state heading on Linked Services tab",
  },
  "buildSources.servicesEmptyDescription": {
    message: "Привяжите существующие Web Service, Private Service, Background Worker или Cron Job, чтобы использовать общие сборки.",
    description: "Empty state description on Linked Services tab",
  },
  "buildSources.autoDeployOn": {
    message: "Авторазвёртывание вкл.",
    description: "Badge indicating a linked service auto-deploys new builds",
  },
  "buildSources.autoDeployOff": {
    message: "Авторазвёртывание выкл.",
    description: "Badge indicating a linked service requires manual deploys",
  },
  "buildSources.changeOrUnlinkSource": {
    message: "Изменить источник",
    description: "Link to a service's settings page to switch or unlink its source",
  },
  "buildSources.linkDialogTitle": {
    message: "Привязать существующий сервис",
    description: "Heading of the Link Existing Service dialog",
  },
  "buildSources.linkDialogDescription": {
    message: "Привяжите существующий сервис рабочего пространства к «{name}». Привязка сохраняет источник без запуска развёртывания.",
    description: "Description of the Link Existing Service dialog",
  },
  "buildSources.linkDialogNoEligible": {
    message: "Все подходящие сервисы в этом рабочем пространстве уже привязаны или ещё не созданы.",
    description: "Message when no linkable unlinked services exist",
  },
  "buildSources.linkDialogServiceLabel": {
    message: "Сервис",
    description: "Select label in Link Existing Service dialog",
  },
  "buildSources.linkDialogServicePlaceholder": {
    message: "Выберите сервис…",
    description: "Select placeholder in Link Existing Service dialog",
  },
  "buildSources.linkDialogRegionMismatch": {
    message: "Регионы не совпадают: этот Git-источник сборки находится в {sourceRegion}, а выбранный сервис — в {serviceRegion}.",
    description: "Inline error when linking a Git build source to a service in another region",
  },
  "buildSources.linkDialogStartCommandHint": {
    message: "Обязательно для источников с нативной средой выполнения, чтобы сервис знал, как запускаться.",
    description: "Help text for startCommand input in Link Existing Service dialog",
  },
  "buildSources.linkDialogSubmit": {
    message: "Привязать сервис",
    description: "Submit button label in Link Existing Service dialog",
  },
  "buildSources.envImageUnsupportedTitle": {
    message: "Окружение сборки недоступно для источников из Docker-образа",
    description: "Fallback heading if an image build source visits /environment",
  },
  "buildSources.envImageUnsupportedBody": {
    message: "Переменные окружения и секретные файлы сборки поддерживаются только для Git-источников сборки.",
    description: "Fallback body if an image build source visits /environment",
  },
  "buildSources.envSaveOnlyNote": {
    message: "Изменения сохраняются без запуска сборки и применяются при следующей сборке.",
    description: "Top note on Build-time Environment tab",
  },
  "buildSources.envVarsDescription": {
    message: "Доступны во время сборок этого источника. Изменения вступят в силу при следующей сборке.",
    description: "Environment Variables card description on Build-time Environment tab",
  },
  "buildSources.secretFilesDescription": {
    message: "Монтируются во время сборок этого источника. Изменения вступят в силу при следующей сборке.",
    description: "Secret Files card description on Build-time Environment tab",
  },
  "buildSources.envGroupsPanelDescription": {
    message: "Группы окружения уровня рабочего пространства для этапа сборки. Изменения применяются при следующей сборке без развёртывания.",
    description: "Environment Groups card description on Build-time Environment tab",
  },
  "buildSources.envGroupsNoneLinked": {
    message: "К этому источнику сборки не привязана ни одна группа окружения.",
    description: "Empty state when no env groups are linked to a build source",
  },
  "buildSources.settingsIdentityTitle": {
    message: "Идентификация и область",
    description: "Heading of read-only identity card on Settings tab",
  },
  "buildSources.settingsIdentityDescription": {
    message: "Неизменяемые идентификаторы и область действия этого источника сборки.",
    description: "Description of read-only identity card on Settings tab",
  },
  "buildSources.settingsFieldId": {
    message: "ID источника сборки",
    description: "Label for read-only bds- ID field on Settings tab",
  },
  "buildSources.settingsConfigTitle": {
    message: "Конфигурация сборки",
    description: "Heading of editable settings card on Settings tab",
  },
  "buildSources.settingsNoBuildNote": {
    message: "Сохранение настроек обновляет источник сборки без запуска сборки.",
    description: "Inline note explaining Settings edits do not rebuild",
  },
  "buildSources.settingsFieldRepoUrl": {
    message: "URL репозитория",
    description: "Input label for Git repository URL on Settings tab",
  },
  "buildSources.settingsInvalidRepoUrl": {
    message: "Введите корректный URL Git-репозитория.",
    description: "Validation error for malformed repo URL on Settings tab",
  },
  "buildSources.settingsInvalidImageUrl": {
    message: "Введите корректную ссылку на Docker-образ.",
    description: "Validation error for malformed image URL on Settings tab",
  },
  "buildSources.settingsSaveButton": {
    message: "Сохранить изменения",
    description: "Submit button label on Settings tab",
  },
  "buildSources.settingsSaving": {
    message: "Сохранение…",
    description: "Submit button label while updateBuildSource is in flight",
  },
  "buildSources.dangerZoneTitle": {
    message: "Опасная зона",
    description: "Heading of Delete Build Source card on Settings tab",
  },
  "buildSources.dangerZoneDescription": {
    message: "Безвозвратно удалите этот источник сборки и его конфигурацию сборки. Это действие нельзя отменить.",
    description: "Description of Delete Build Source card",
  },
  "buildSources.deleteBlockedTitle_one": {
    message: "Отвяжите {count} сервис перед удалением",
    description: "Singular alert heading blocking delete when services remain linked",
  },
  "buildSources.deleteBlockedTitle_few": {
    message: "Отвяжите {count} сервиса перед удалением",
    description: "Few alert heading blocking delete when services remain linked",
  },
  "buildSources.deleteBlockedTitle_many": {
    message: "Отвяжите {count} сервисов перед удалением",
    description: "Many alert heading blocking delete when services remain linked",
  },
  "buildSources.deleteBlockedTitle_other": {
    message: "Отвяжите {count} сервисов перед удалением",
    description: "Other alert heading blocking delete when services remain linked",
  },
  "buildSources.deleteBlockedDescription": {
    message: "Источник сборки нельзя удалить, пока на него ссылаются сервисы. Сначала переключите каждый связанный сервис на другой источник:",
    description: "Explanation under delete-blocked alert",
  },
  "buildSources.deleteButton": {
    message: "Удалить источник сборки",
    description: "Destructive button opening the delete confirmation dialog",
  },
  "buildSources.deleteConfirmTitle": {
    message: "Удалить «{name}»?",
    description: "Title of Delete Build Source confirmation dialog",
  },
  "buildSources.deleteConfirmBody": {
    message: "Источник сборки и его окружение сборки будут безвозвратно удалены. Историю сборок и настройки восстановить не удастся.",
    description: "Body of Delete Build Source confirmation dialog",
  },
  "buildSources.deleteConfirmPrompt": {
    message: "Введите {phrase} ниже для подтверждения.",
    description: "Sudo command prompt inside Delete Build Source confirmation dialog",
  },
  "buildSources.deleteConfirmButton": {
    message: "Удалить источник сборки",
    description: "Confirm action label inside Delete Build Source confirmation dialog",
  },
};

export default ruBuildSources;
