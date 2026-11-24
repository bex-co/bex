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
};

export default ruBuildSources;
