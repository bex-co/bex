import type { TranslationEntry } from "@/i18n";

const ruCommon: Record<string, TranslationEntry> = {
  "common.navRouter": {
    message: "Маршрутизатор",
    description: "Router navigation item",
  },
  "common.protectedConfirmationTitle": {
    message: "Требуется подтверждение для защищённого окружения",
    description:
      "Title for the retry dialog shown after a protected environment blocks a destructive action",
  },
  "common.protectedConfirmationBody": {
    message:
      "{name} относится к защищённому окружению. Чтобы продолжить, введите точную команду ниже.",
    description:
      "Resource-neutral explanation for a protected environment confirmation gate",
  },
  "common.protectedConfirmationPrompt": {
    message: "Введите {phrase} ниже для подтверждения.",
    description:
      "Prompt naming the server-issued protected environment confirmation phrase",
  },
  "common.confirmPhrasePrompt": {
    message: "Чтобы продолжить, введите {phrase}",
    description:
      "Label above the typed-confirmation input in a phrase-gated confirm dialog.",
  },
  "common.close": {
    message: "Закрыть",
    description:
      "Accessible name of the shared dialog's built-in close (X) button.",
  },
  "common.sidebarTitle": {
    message: "Боковая панель",
    description: "Screen-reader title of the mobile navigation drawer.",
  },
  "common.sidebarMobileDescription": {
    message: "Показывает мобильную боковую панель.",
    description: "Screen-reader description of the mobile navigation drawer.",
  },
  "common.skipToMainContent": {
    message: "Перейти к основному содержимому",
    description:
      "Label of the first-tab skip link that moves keyboard focus past the sidebar and header to the page content.",
  },
  "common.sidebarToggle": {
    message: "Показать или скрыть боковую панель",
    description:
      "Accessible name of the button that opens or collapses the navigation sidebar.",
  },
  "common.sidebarResize": {
    message: "Изменить размер боковой панели",
    description: "Accessible name of the sidebar's draggable resize separator.",
  },
  "common.sidebarResizeHint": {
    message:
      "Перетащите, чтобы изменить размер · дважды щёлкните, чтобы переключить",
    description: "Hover tooltip on the sidebar's resize separator.",
  },
  "common.commandSuggestions": {
    message: "Подсказки",
    description:
      "Accessible name of a search or picker results list (the command menu's listbox).",
  },
  "common.comboboxPlaceholder": {
    message: "Выберите вариант",
    description:
      "Fallback placeholder of a searchable picker when its caller supplies none.",
  },
  "common.comboboxEmpty": {
    message: "Ничего не найдено",
    description:
      "Fallback text when a searchable picker's filter matches no option.",
  },
  "common.comboboxUseCustom": {
    message: "Нажмите Enter, чтобы использовать «{value}»",
    description:
      "Hint under a picker's empty results: Enter keeps the typed value as entered. {value} is the typed text.",
  },
  "common.comboboxClear": {
    message: "Очистить поле",
    description: "Accessible name of a searchable picker's clear (X) button.",
  },
  "common.cancel": {
    message: "Отменить",
    description:
      "Generic Cancel button — dismisses a dialog or an inline form without saving.",
  },
  "common.pricePerRunMinute": {
    message: "${price}/мин времени выполнения",
    description:
      "A cron job's USD price per minute of run time, on a Change Compute Plan card (w4/546): it is billed per second only while a run executes; price is an API-supplied decimal without the currency symbol",
  },
  "common.pricePerMinute": {
    message: "${price}/мин",
    description:
      "A cron job's USD price per minute of run time on a create-form plan card, short to fit beside the plan name; the form's hint says it is billed only while a run executes (w4/546). price is an API-supplied decimal without the currency symbol",
  },
  "common.pricePerMonth": {
    message: "${price}/мес.",
    description:
      "Monthly USD price shown on a resource plan card; price is an API-supplied decimal without the currency symbol",
  },
  "common.reviewChangesTitle": {
    message: "Применить изменения?",
    description:
      "Title of Render's plan review dialog, shown before a compute or storage change applies (w3/m674)",
  },
  "common.reviewChangesTotal": {
    message: "Итого",
    description:
      "Plan review dialog: the label of the monthly total under the priced lines",
  },
  "common.reviewEditComputePlan": {
    message: "Изменить тариф вычислений",
    description:
      "Plan review dialog: accessible name of the Compute plan line's Edit button, which returns to the plan picker",
  },
  "common.planUnavailable": {
    message: "Недоступно",
    description:
      "Shown in place of the price on a disabled plan card whose compute plan is paused (it cannot be selected for now); the server's full reason is the card's tooltip",
  },
  "common.planConnections_one": {
    message: "{formatted} подключение",
    description:
      "A plan card's connection-limit line, under its RAM: the client connections the plan accepts (Render's Key Value cards read \"50 Connections\"); {formatted} is the locale-formatted count",
  },
  "common.planConnections_few": {
    message: "{formatted} подключения",
    description:
      "A plan card's connection-limit line, under its RAM: the client connections the plan accepts (Render's Key Value cards read \"50 Connections\"); {formatted} is the locale-formatted count",
  },
  "common.planConnections_many": {
    message: "{formatted} подключений",
    description:
      "A plan card's connection-limit line, under its RAM: the client connections the plan accepts (Render's Key Value cards read \"50 Connections\"); {formatted} is the locale-formatted count",
  },
  "common.planConnections_other": {
    message: "{formatted} подключения",
    description:
      "A plan card's connection-limit line, under its RAM: the client connections the plan accepts (Render's Key Value cards read \"50 Connections\"); {formatted} is the locale-formatted count",
  },
  "common.protectedConfirmationRetry": {
    message: "Подтвердить и сохранить",
    description:
      "Confirm button in the protected environment retry dialog raised by a settings save",
  },
  "common.protectedConfirmationCancel": {
    message: "Отменить",
    description:
      "Cancel button in the protected environment confirmation dialog",
  },
  "common.navEnvGroups": {
    message: "Группы окружения",
    description: "Workspace sidebar link to environment groups",
  },
  "common.navBuildSources": {
    message: "Источники сборки",
    description: "Workspace sidebar link to build sources",
  },
  "common.navSandboxGroups": {
    message: "Группы песочниц",
    description: "Workspace sidebar link to sandbox groups",
  },
  "common.appName": {
    message: "bex",
    description: "Product name shown in the dashboard chrome",
  },
  "common.headDescription": {
    message:
      "Развёртывайте приложения и управляйте ими на инфраструктуре, которую вы контролируете, с помощью bex — открытой AI-native альтернативы Render.",
    description:
      "Generic dashboard description used in description, Open Graph, and Twitter metadata",
  },
  "common.headImageAlt": {
    message: "Логотип панели bex",
    description: "Alternative text for the dashboard's generic social image",
  },
  "common.loading": {
    message: "Загрузка…",
    description: "Generic loading state label",
  },
  "common.topbarBreadcrumbs": {
    message: "Навигационная цепочка",
    description: "Accessible label for the dashboard topbar hierarchy",
  },
  "common.topbarSearch": {
    message: "Поиск",
    description: "Open the workspace-wide topbar search",
  },
  "common.copyCode": {
    message: "Копировать код",
    description: "Accessible name of the copy button on a code block",
  },
  "common.topbarSearchPlaceholder": {
    message: "Поиск страниц и ресурсов…",
    description: "Placeholder in the workspace-wide topbar command search",
  },
  "common.topbarSearchDescription": {
    message: "Поиск по страницам панели и ресурсам рабочего пространства.",
    description: "Accessible description for the topbar command search",
  },
  "common.topbarSearchEmpty": {
    message: "Подходящих страниц или ресурсов нет.",
    description: "Empty state in the workspace-wide topbar command search",
  },
  "common.copyField": {
    message: "Копировать: {field}",
    description:
      "Accessible name for a copy control when the caller only has a field label (m101/t014)",
  },
  "common.topbarNavigation": {
    message: "Навигация",
    description: "Page-links group in the workspace-wide topbar search",
  },
  "common.topbarResources": {
    message: "Ресурсы",
    description: "Workspace-resources group in the topbar search",
  },
  "common.topbarNew": {
    message: "Создать",
    description: "Persistent topbar menu for creating a resource",
  },
  "common.topbarCreate": {
    message: "Создать ресурс",
    description: "Heading in the persistent topbar create menu",
  },
  "common.topbarHelp": {
    message: "Справка и ресурсы",
    description: "Accessible label for the topbar help menu",
  },
  "common.topbarDocumentation": {
    message: "Документация",
    description: "Topbar help-menu link to bex documentation",
  },
  "common.topbarCliGuide": {
    message: "Руководство по CLI",
    description: "Topbar help-menu link to the bex CLI guide",
  },
  "common.topbarRepository": {
    message: "Репозиторий на GitHub",
    description: "Topbar help-menu link to the bex source repository",
  },
  "common.topbarWorkspaceSettings": {
    message: "Настройки рабочего пространства",
    description: "Workspace-settings label in topbar navigation and search",
  },
  "common.searchShortcutEnvVarsDocs": {
    message: "Переменные окружения",
    description:
      "Search shortcut to the docs page on environment variables and secrets (Render's 'Environment variables' shortcut)",
  },
  "common.searchShortcutEnvVarsDocsDescription": {
    message: "Задайте переменные окружения и секреты для сервиса",
    description:
      "One-line description beside the environment variables docs search shortcut",
  },
  "common.searchShortcutCustomDomainsDocs": {
    message: "Пользовательские домены",
    description: "Search shortcut to the custom domains docs page",
  },
  "common.searchShortcutCustomDomainsDocsDescription": {
    message: "Добавьте пользовательский домен к сервису",
    description:
      "One-line description beside the custom domains docs search shortcut",
  },
  "common.searchShortcutGitCredentials": {
    message: "Учётные данные Git",
    description:
      "Search shortcut to Account Settings → Integrations, where GitHub is connected",
  },
  "common.searchShortcutGitCredentialsDescription": {
    message: "Подключите GitHub",
    description:
      "One-line description beside the Git credentials search shortcut; bex connects GitHub only",
  },
  "common.searchShortcutAuditLogs": {
    message: "Журналы аудита",
    description: "Search shortcut to Workspace Settings → Audit Logs",
  },
  "common.searchShortcutAuditLogsDescription": {
    message: "Экспортируйте журналы аудита",
    description: "One-line description beside the Audit Logs search shortcut",
  },
  "common.searchShortcutApiKeys": {
    message: "API-ключи",
    description: "Search shortcut to Account Settings → API Keys",
  },
  "common.searchShortcutApiKeysDescription": {
    message: "Создавайте API-ключи",
    description: "One-line description beside the API Keys search shortcut",
  },
  "common.searchShortcutSshKeys": {
    message: "SSH-ключи",
    description: "Search shortcut to Account Settings → SSH Public Keys",
  },
  "common.searchShortcutSshKeysDescription": {
    message: "Добавляйте публичные SSH-ключи",
    description: "One-line description beside the SSH Keys search shortcut",
  },
  "common.searchShortcutCliTokens": {
    message: "Токены CLI",
    description:
      "Search shortcut to Account Settings → Connected agents, which lists the CLI's tokens",
  },
  "common.searchShortcutCliTokensDescription": {
    message: "Просматривайте и отзывайте токены CLI",
    description: "One-line description beside the CLI Tokens search shortcut",
  },
  "common.searchShortcutRegistryCredentials": {
    message: "Учётные данные реестров контейнеров",
    description: "Search shortcut to Workspace Settings → Registry Credentials",
  },
  "common.searchShortcutRegistryCredentialsDescription": {
    message: "Управляйте учётными данными для внешних образов",
    description:
      "One-line description beside the container registry credentials search shortcut",
  },
  "common.searchShortcutMcpServer": {
    message: "MCP-сервер",
    description: "Search shortcut to the MCP server docs page",
  },
  "common.searchShortcutMcpServerDescription": {
    message: "Управляйте сервисами bex из ИИ-приложений",
    description: "One-line description beside the MCP Server search shortcut",
  },
  "common.topbarSwitchProject": {
    message: "Сменить проект",
    description: "Heading in a project breadcrumb dropdown",
  },
  "common.topbarSwitchEnvironment": {
    message: "Сменить окружение",
    description: "Heading in an environment breadcrumb dropdown",
  },
  "common.topbarSwitchService": {
    message: "Сменить сервис",
    description: "Heading in a service breadcrumb dropdown",
  },
  "common.topbarAllResources": {
    message: "Все ресурсы",
    description: "Service breadcrumb menu link back to workspace resources",
  },
  "common.topbarProjectResource": {
    message: "Проект",
    description: "Project kind label in workspace-wide search results",
  },
  "common.topbarServiceResource": {
    message: "Сервис",
    description: "Service kind label in workspace-wide search results",
  },
  "common.sudoCommandLabel": {
    message: "Команда sudo",
    description:
      "Input label on Render-style destructive type-to-confirm gates (live capture: docs/render-artifacts/workspace-lifecycle.md)",
  },
  "common.navIntegrationsGroup": {
    message: "Интеграции",
    description:
      "Sidebar nav section label (webhooks, notifications) — Render's grouping",
  },
  "common.navWorkspaceGroup": {
    message: "Рабочее пространство",
    description:
      "Sidebar nav section label (usage, settings) — Render's grouping",
  },
  "common.navMonitorGroup": {
    message: "Мониторинг",
    description:
      "Service sidebar section label (logs, metrics) — Render's grouping",
  },
  "common.navWebhooks": {
    message: "Вебхуки",
    description: "Sidebar nav link to the outbound event webhooks page",
  },
  "common.labelInvalid": {
    message:
      "Введите название длиной до 100 символов без переносов строк и управляющих символов.",
    description:
      "Inline validation under a free-form name field (project, environment, group, webhook, credential, Blueprint)",
  },
  "common.navNotifications": {
    message: "Уведомления",
    description: "Sidebar nav link to the notification settings page",
  },
  "common.navProjects": {
    message: "Проекты",
    description:
      "Sidebar nav link to the unified projects page (services, databases, key value grouped together)",
  },
  "common.navUsage": {
    message: "Оплата",
    description:
      "Sidebar nav link to the workspace billing page (/billing, renamed from Usage in w5/m70; key kept to avoid churn)",
  },
  "common.navBlueprints": {
    message: "Blueprints",
    description:
      "Sidebar nav link to the blueprints management page (IaC stacks auto-registered on deploy)",
  },
  "common.navAgents": {
    message: "Агенты",
    description:
      "Sidebar nav link to the cloud coding-agent sessions page (/agents)",
  },
  "common.navAutomations": {
    message: "Автоматизации",
    description:
      "Sidebar nav link to the scheduled agent automations page (/automations)",
  },
  "common.navSettings": {
    message: "Настройки",
    description:
      "Sidebar nav link to the workspace settings page (the sidebar is workspace-scoped; account settings hang off the user menu)",
  },
  "common.navBackToDashboard": {
    message: "Панель",
    description:
      "Contextual project sidebar's back link label, shown on a project's Overview page (links up to the workspace Overview)",
  },
  "common.navBackToProject": {
    message: "Проект",
    description:
      "Contextual project sidebar's back link label, shown on a project's Settings page (links up to the project's Overview)",
  },
  "common.navBackToEnvironment": {
    message: "Окружение",
    description:
      "Contextual project sidebar's back link label, shown on an environment's settings page (links up to the environment's section on the project page, Render's words, w1/m683)",
  },
  "common.navProjectOverview": {
    message: "Обзор",
    description:
      "Contextual project sidebar nav link to the project's own Overview page",
  },
  "common.navObservabilityGroup": {
    message: "Наблюдаемость",
    description:
      "Contextual project sidebar section label above the project's log views (Render's OBSERVABILITY group, w3/m505)",
  },
  "common.navManageGroup": {
    message: "Управление",
    description: "Contextual project sidebar section label above Settings",
  },
  "common.changeLanguage": {
    message: "Сменить язык",
    description: "Accessible label for the language switcher button",
  },
  "common.userMenuSettings": {
    message: "Настройки аккаунта",
    description:
      "User menu item that navigates to account settings (/settings)",
  },
  "common.userMenuTheme": {
    message: "Тема",
    description: "User menu submenu label for theme selection",
  },
  "common.userMenuLanguage": {
    message: "Язык",
    description: "User menu submenu label for language selection",
  },
  "common.userMenuThemeLight": {
    message: "Светлая",
    description: "Light theme option label",
  },
  "common.userMenuThemeDark": {
    message: "Тёмная",
    description: "Dark theme option label",
  },
  "common.userMenuThemeSystem": {
    message: "Системная",
    description: "System theme option label",
  },
  "common.themeNotSaved": {
    message: "Тема не сохранена. Восстановлена предыдущая тема.",
    description:
      "Toast when a theme change could not be saved to the account (w3/m531); the previous theme is put back",
  },
  "common.userMenuLogOut": {
    message: "Выйти",
    description: "User menu item that signs the user out",
  },
  "common.userMenuOpen": {
    message: "Меню аккаунта",
    description:
      "Accessible name of the avatar button that opens the account menu (w4/407)",
  },
  "common.userMenuOpenFor": {
    message: "Меню аккаунта {name}",
    description:
      "Accessible name of the avatar button that opens the account menu, naming the signed-in identity (w4/407)",
  },
  "common.goHome": {
    message: "На главную",
    description: "Button label that navigates back to the home page",
  },
  "common.goBack": {
    message: "Назад",
    description: "Button label that navigates to the previous page",
  },
  "common.planCatalogUnavailable": {
    message:
      "Не удалось загрузить тарифы, поэтому пока выбрать их нельзя. Остальные введённые данные сохранены.",
    description:
      "A plan picker whose catalog read failed or returned no plans; shown with a Try again button that re-reads only the catalog (w4/m218)",
  },
  "common.tryAgain": {
    message: "Повторить",
    description: "Button label that retries after an error",
  },
  "common.notFoundTitle": {
    message: "Страница не найдена",
    description: "404 page heading",
  },
  "common.notFoundDescription": {
    message: "Страница, которую вы ищете, не существует или была перемещена.",
    description: "404 page explanatory text",
  },
  "common.errorTitle": {
    message: "Что-то пошло не так",
    description: "Generic error page heading",
  },
  "common.errorDefaultMessage": {
    message: "Что-то пошло не так.",
    description: "Fallback error message when none is provided",
  },
  "common.loadErrorRetry": {
    message:
      "При загрузке что-то пошло не так. Обновите страницу и повторите попытку.",
    description:
      "Body of a panel whose read failed with no server reason to show (a network failure or throttling)",
  },
  "common.serviceNotFoundTitle": {
    message: "Сервис не найден",
    description:
      "Title of a service page whose id leads nowhere (Render's ResourceNotFound, w3/m706)",
  },
  "common.serviceDeletedTitle": {
    message: "Сервис удалён",
    description:
      "Title of a service page whose service the dashboard had read and is now gone (w3/m706)",
  },
  "common.cronJobNotFoundTitle": {
    message: "Cron job не найден",
    description: "Title of a cron job page whose id leads nowhere (w3/m706)",
  },
  "common.cronJobDeletedTitle": {
    message: "Cron job удалён",
    description:
      "Title of a cron job page whose cron job the dashboard had read and is now gone (w3/m706)",
  },
  "common.sandboxGroupNotFoundTitle": {
    message: "Группа песочниц не найдена",
    description:
      "Title of a sandbox group page whose id leads nowhere (w3/m706)",
  },
  "common.sandboxGroupDeletedTitle": {
    message: "Группа песочниц удалена",
    description:
      "Title of a sandbox group page whose group the dashboard had read and is now gone (w3/m706)",
  },
  "common.deployNotFoundTitle": {
    message: "Деплой не найден",
    description:
      "Title of a deploy page whose deploy id leads nowhere under a live service (w3/m706)",
  },
  "common.buildNotFoundTitle": {
    message: "Сборка не найдена",
    description:
      "Title of a build page whose build id leads nowhere under a live build source (w3/m706)",
  },
  "common.resourceNotFoundBody": {
    message:
      "Не удалось найти {id}. Возможно, он был удалён или ссылка неверна.",
    description:
      "Body of a resource page whose id leads nowhere; {id} is the id, drawn as a code chip (w3/m706)",
  },
  "common.resourceDeletedBody": {
    message: "{name} ({id}) удалён.",
    description:
      "Body of a resource page whose resource the dashboard had read and is now gone; {name} is its name, {id} its id (w3/m706)",
  },
  "common.resourceNotFoundForBody": {
    message: "Не удалось найти {id} для {parent}. Возможно, ссылка неверна.",
    description:
      "Body of a deploy or build page whose id leads nowhere; {id} is the id, {parent} the service or build source it was opened under (w3/m706)",
  },
  "common.goToProject": {
    message: "Перейти к проекту",
    description:
      "Button on a missing resource's page that opens the project it was in (w3/m706)",
  },
  "common.returnHome": {
    message: "Вернуться на главную",
    description:
      "Button on a missing, denied or failed resource's page that opens the home page (w3/m706)",
  },
  "common.viewDeploys": {
    message: "Смотреть деплои",
    description:
      "Button on a missing deploy's page that opens its service's deploys (w3/m706)",
  },
  "common.viewBuilds": {
    message: "Смотреть сборки",
    description:
      "Button on a missing build's page that opens its build source's builds (w3/m706)",
  },
  "common.goToWebhooks": {
    message: "Перейти к вебхукам",
    description:
      "Button on a missing or denied webhook's page that opens the webhooks list (w3/m706)",
  },
  "common.resourceDoesNotExist": {
    message: "{id} не существует.",
    description:
      "Body of the Page not found state a resource page draws for a dead id; {id} is the id (w3/m706)",
  },
  "common.accessDeniedTitle": {
    message: "Доступ запрещён",
    description: "Title of a resource page whose read was refused (w3/m706)",
  },
  "common.accessDeniedBody": {
    message: "У вас нет доступа к {id}.",
    description:
      "Body of a resource page whose read was refused; {id} is the id (w3/m706)",
  },
  "common.accessDeniedBodyNoId": {
    message: "У вас нет доступа к этому ресурсу.",
    description: "Body of a refused page that names no id (w3/m706)",
  },
  "common.resourceLoadFailedTitle": {
    message: "Произошла ошибка",
    description: "Title of a resource page whose read failed (w3/m706)",
  },
  "common.resourceLoadFailedBody": {
    message: "Не удалось загрузить {id}.",
    description:
      "Body of a resource page whose read failed; {id} is the id (w3/m706)",
  },
  "common.copyId": {
    message: "Скопировать ID",
    description:
      "Accessible name of the copy button beside an id in a resource state (w3/m706)",
  },
  "common.copyIdError": {
    message: "Не удалось скопировать ID",
    description:
      "Toast when copying an id from a resource state fails (w3/m706)",
  },
  "common.resourceErrorBody": {
    message: "Не удалось загрузить ресурс. Проверьте API и повторите попытку.",
    description:
      "Body text of a detail page's inline error state when the resource query failed",
  },
  "common.sessionExpiredTitle": {
    message: "Ваша сессия истекла",
    description:
      "Heading of the inline state shown when a resource query fails with a 401 (expired session)",
  },
  "common.sessionExpiredBody": {
    message:
      "Войдите снова, чтобы продолжить — вы сразу вернётесь на эту страницу.",
    description:
      "Body text of the expired-session inline state, reassuring the user their place is preserved",
  },
  "common.signIn": {
    message: "Войти",
    description:
      "Button label that sends an expired session back to the login page",
  },
  "common.colActions": {
    message: "Действия",
    description:
      "Screen-reader-only header for a table's trailing row-actions column",
  },
  "common.justNow": {
    message: "только что",
    description:
      'Elapsed-time text for an instant under a minute old ("Deployed just now")',
  },
  "common.timeLocal": {
    message: "Местное",
    description:
      "Timestamp tooltip: label of the row showing the instant in the viewer's local timezone",
  },
  "common.timeUtc": {
    message: "UTC",
    description:
      "Timestamp tooltip: label of the row showing the instant in UTC",
  },
  "common.timeUnix": {
    message: "Метка времени",
    description:
      "Timestamp tooltip: label of the row showing the instant as Unix milliseconds",
  },
  "common.timeFromPrevious": {
    message: "С предыдущей строки",
    description:
      "Timestamp tooltip (w4/716): label of a log line's row showing the time since the line before it, Render's 'From previous'",
  },
  "common.timeFromStart": {
    message: "С начала",
    description:
      "Timestamp tooltip (w4/716): label of a deploy log line's row showing the time since the deploy began, Render's 'From start'",
  },
  "common.timeRowLabel": {
    message: "{label}:",
    description:
      "Timestamp tooltip (w4/716): a row's label followed by Render's colon; {label} is the zone ('PST'), UTC, Timestamp, From previous or From start",
  },
  "common.timeCopied": {
    message: "Скопировано",
    description:
      "Timestamp tooltip (w4/716): toast after a row's copy button copied its value, Render's 'Copied!'",
  },
  "common.timeCopyFailed": {
    message: "Не удалось скопировать время",
    description:
      "Timestamp tooltip (w4/716): toast when the clipboard refused a row's value",
  },
  "common.ipAllowListProxiedTitle_one": {
    message:
      "Список разрешённых адресов не видит клиентов на {count} пользовательском домене",
    description:
      "Title of the warning shown under an inbound IP allowlist when one of the custom domains it covers is proxied by Cloudflare (w1/m171)",
  },
  "common.ipAllowListProxiedTitle_few": {
    message:
      "Список разрешённых адресов не видит клиентов на {count} пользовательских доменах",
    description:
      "Title of the warning shown under an inbound IP allowlist when several custom domains it covers are proxied by Cloudflare (w1/m171)",
  },
  "common.ipAllowListProxiedTitle_many": {
    message:
      "Список разрешённых адресов не видит клиентов на {count} пользовательских доменах",
    description:
      "Title of the warning shown under an inbound IP allowlist when several custom domains it covers are proxied by Cloudflare (w1/m171)",
  },
  "common.ipAllowListProxiedTitle_other": {
    message:
      "Список разрешённых адресов не видит клиентов на {count} пользовательских доменах",
    description:
      "Title of the warning shown under an inbound IP allowlist when several custom domains it covers are proxied by Cloudflare (w1/m171)",
  },
  "common.ipAllowListProxiedBody_one": {
    message:
      "Этот домен проксируется через Cloudflare, поэтому список разрешённых адресов сопоставляется с адресом Cloudflare, а не посетителя. Список сохранён, но на этом домене он заблокирует посетителей, которых вы хотели пропустить:",
    description:
      "Body of the Cloudflare-proxied allowlist warning, followed by the one proxied hostname (w1/m171)",
  },
  "common.ipAllowListProxiedBody_few": {
    message:
      "Эти домены проксируются через Cloudflare, поэтому список разрешённых адресов сопоставляется с адресом Cloudflare, а не посетителя. Список сохранён, но на этих доменах он заблокирует посетителей, которых вы хотели пропустить:",
    description:
      "Body of the Cloudflare-proxied allowlist warning, followed by the list of proxied hostnames (w1/m171)",
  },
  "common.ipAllowListProxiedBody_many": {
    message:
      "Эти домены проксируются через Cloudflare, поэтому список разрешённых адресов сопоставляется с адресом Cloudflare, а не посетителя. Список сохранён, но на этих доменах он заблокирует посетителей, которых вы хотели пропустить:",
    description:
      "Body of the Cloudflare-proxied allowlist warning, followed by the list of proxied hostnames (w1/m171)",
  },
  "common.ipAllowListProxiedBody_other": {
    message:
      "Эти домены проксируются через Cloudflare, поэтому список разрешённых адресов сопоставляется с адресом Cloudflare, а не посетителя. Список сохранён, но на этих доменах он заблокирует посетителей, которых вы хотели пропустить:",
    description:
      "Body of the Cloudflare-proxied allowlist warning, followed by the list of proxied hostnames (w1/m171)",
  },
  "common.ipAllowListProxiedFix": {
    message:
      "Чтобы фильтровать реальные адреса клиентов, переключите DNS-запись в Cloudflare в режим DNS only (серое облако).",
    description:
      "How to fix the Cloudflare-proxied allowlist warning: switch the record from proxied (orange cloud) to DNS only (w1/m171)",
  },
  "common.inboundIpAllowedRanges_one": {
    message: "диапазон IP разрешён извне вашей частной сети.",
    description:
      "Status line of a Networking panel after a count badge (Render's, w4/m710): how many IPv4 ranges every level of inbound IP rules (workspace, environment, the resource's own) allows together",
  },
  "common.inboundIpAllowedRanges_few": {
    message: "диапазона IP разрешены извне вашей частной сети.",
    description:
      "Status line of a Networking panel after a count badge (Render's, w4/m710): how many IPv4 ranges every level of inbound IP rules (workspace, environment, the resource's own) allows together",
  },
  "common.inboundIpAllowedRanges_many": {
    message: "диапазонов IP разрешено извне вашей частной сети.",
    description:
      "Status line of a Networking panel after a count badge (Render's, w4/m710): how many IPv4 ranges every level of inbound IP rules (workspace, environment, the resource's own) allows together",
  },
  "common.inboundIpAllowedRanges_other": {
    message: "диапазона IP разрешено извне вашей частной сети.",
    description:
      "Status line of a Networking panel after a count badge (Render's, w4/m710): how many IPv4 ranges every level of inbound IP rules (workspace, environment, the resource's own) allows together",
  },
  "common.inboundIpValidateLabel": {
    message: "Проверить IP-адрес",
    description:
      "Label of the Networking panel's address check field (Render's Validate IP address, w4/m710)",
  },
  "common.inboundIpCheckEmpty": {
    message: "Введите IP-адрес",
    description:
      "Validate IP address field, nothing typed yet (Render's tooltip, w4/m710)",
  },
  "common.inboundIpCheckInvalid": {
    message: "Этот IP-адрес недействителен",
    description:
      "Validate IP address field: the text is not an IPv4 address (Render's, w4/m710)",
  },
  "common.inboundIpCheckAllowed": {
    message: "Доступ с этого IP-адреса разрешён",
    description:
      "Validate IP address field: every level of inbound IP rules admits the address (Render's, w4/m710)",
  },
  "common.inboundIpRefusedWorkspace": {
    message:
      "Доступ с этого IP-адреса запрещён правилами входящих IP вашего рабочего пространства.",
    description:
      "Validate IP address field: the workspace's inbound IP rules refuse the address (Render's, w4/m710)",
  },
  "common.inboundIpRefusedEnvironment": {
    message:
      "Доступ с этого IP-адреса запрещён правилами входящих IP вашего окружения.",
    description:
      "Validate IP address field: the environment's inbound IP rules refuse the address (Render's, w4/m710)",
  },
  "common.inboundIpRefusedOwn": {
    message:
      "Доступ с этого IP-адреса запрещён правилами входящих IP вашего ресурса {type}.",
    description:
      "Validate IP address field: the resource's own (unsaved) inbound IP rules refuse the address; {type} is its type, e.g. Web Service, PostgreSQL, Key Value, Environment (Render's, w4/m710)",
  },
  "common.inboundIpAffectedBy": {
    message: "Также действуют:",
    description:
      "Label before the breadcrumb of a higher level of inbound IP rules that also applies to this resource (Render's 'Affected by:', w4/m710)",
  },
  "common.inboundIpLevelWorkspace": {
    message: "Рабочее пространство",
    description:
      "Breadcrumb item naming the workspace level of inbound IP rules (Render's, w4/m710)",
  },
  "common.inboundIpLevelEnvironment": {
    message: "Окружение",
    description:
      "Breadcrumb item naming the environment level of inbound IP rules, and an environment's type in the own-rules heading and refusal (Render's, w4/m710)",
  },
  "common.inboundIpViewRules": {
    message: "Открыть правила",
    description:
      "Link, opening in a new tab, to where a higher level of inbound IP rules is edited (Render's View rules, w4/m710)",
  },
  "common.inboundIpSource": {
    message: "Источник",
    description:
      "Column header of a read-only inbound IP rules table: the CIDR range (Render's Source, w4/m710)",
  },
  "common.inboundIpDescription": {
    message: "Описание",
    description:
      "Column header of a read-only inbound IP rules table: the rule's description (Render's, w4/m710)",
  },
  "common.inboundIpBlockedWorkspace": {
    message:
      "Правила входящих IP рабочего пространства блокируют весь интернет-трафик.",
    description:
      "Warning in place of the table when the workspace has no inbound IP rule (Render's, w4/m710)",
  },
  "common.inboundIpBlockedEnvironment": {
    message: "Правила входящих IP окружения блокируют весь интернет-трафик.",
    description:
      "Warning in place of the table when the environment has no inbound IP rule (Render's, w4/m710)",
  },
  "common.inboundIpOwnRules": {
    message: "Правила входящих IP: {type}",
    description:
      "Heading above a resource's own editable inbound IP rules; {type} is its type, e.g. Web Service, PostgreSQL, Key Value, Environment (Render's, w4/m710)",
  },
  "common.inboundIpTypePostgres": {
    message: "PostgreSQL",
    description:
      "A Postgres database's type as the inbound IP rules heading and refusal name it (Render's userFacingType, w4/m710)",
  },
  "common.inboundIpLoadError": {
    message:
      "Не удалось загрузить правила входящих IP рабочего пространства и окружения, которые тоже действуют. Обновите страницу и попробуйте снова.",
    description:
      "Shown in place of the workspace and environment inbound IP rules when one of them could not be read (w4/m710)",
  },
  "common.relativeAgeNow": {
    message: "сейчас",
    description: "Compact relative age under one minute (w4/m188)",
  },
  "common.relativeAgeMinutes": {
    message: "{n} мин",
    description: "Compact relative age in whole minutes, e.g. 4m (w4/m188)",
  },
  "common.relativeAgeHours": {
    message: "{n} ч",
    description: "Compact relative age in whole hours, e.g. 3h (w4/m188)",
  },
  "common.relativeAgeDays": {
    message: "{n} д",
    description: "Compact relative age in whole days, e.g. 5d (w4/m188)",
  },
  "common.relativeAgeMonths": {
    message: "{n} мес.",
    description: "Compact relative age in 30-day months, e.g. 2mo (w4/m188)",
  },
  "common.relativeAgeYears": {
    message: "{n} г.",
    description: "Compact relative age in 365-day years, e.g. 1y (w4/m188)",
  },
  "common.relativeUntilNow": {
    message: "сейчас",
    description:
      "Countdown to a future instant less than one minute away (w4/m188)",
  },
  "common.relativeUntilMinutes": {
    message: "через {n} мин",
    description: "Compact countdown in whole minutes, e.g. in 5m (w4/m188)",
  },
  "common.relativeUntilHours": {
    message: "через {n} ч",
    description: "Compact countdown in whole hours, e.g. in 3h (w4/m188)",
  },
  "common.relativeUntilDays": {
    message: "через {n} д",
    description: "Compact countdown in whole days, e.g. in 83d (w4/m188)",
  },
  "common.openBilling": {
    message: "Открыть оплату",
    description:
      "Link from a plan notice (a Pro-and-higher feature on Hobby) to the Billing page (w8/m138)",
  },
  "common.billingSettlingTitle": {
    message: "Расчёт итогового использования…",
    description:
      "Title while a deletion waits for the workspace's final usage to be charged (BILLING_SETTLING); retried automatically",
  },
  "common.billingSettlingDescription": {
    message:
      "bex списывает плату за итоговое использование перед удалением. Это может занять минуту или две; попытка повторяется автоматически, поэтому не закрывайте эту страницу.",
    description:
      "Body while a deletion waits for the final usage to be charged (BILLING_SETTLING)",
  },
  "common.billingSettlingTimeout": {
    message:
      "Итоговое использование всё ещё обрабатывается. Повторите удаление через несколько минут.",
    description:
      "Deletion error after BILLING_SETTLING retries ran out (about three minutes)",
  },
  "common.balanceDueTitle": {
    message: "Неоплаченный баланс",
    description:
      "Title of the alert when a deletion is refused because the workspace has an unpaid balance (BILLING_BALANCE_DUE)",
  },
  "common.balanceDueAmount": {
    message:
      "В этом рабочем пространстве есть неоплаченный баланс: {amount}. Оплатите счёт и повторите удаление.",
    description: "Balance-due deletion refusal with the amount",
  },
  "common.balanceDue": {
    message:
      "В этом рабочем пространстве есть неоплаченный баланс. Оплатите счёт и повторите удаление.",
    description: "Balance-due deletion refusal without an amount",
  },
  "common.balanceDueNamedAmount": {
    message:
      "В рабочем пространстве {name} есть неоплаченный баланс: {amount}. Оплатите счёт и повторите удаление аккаунта.",
    description:
      "Account-deletion refusal naming the workspace with the unpaid balance and its amount",
  },
  "common.balanceDueNamed": {
    message:
      "В рабочем пространстве {name} есть неоплаченный баланс. Оплатите счёт и повторите удаление аккаунта.",
    description:
      "Account-deletion refusal naming the workspace with the unpaid balance",
  },
  "common.balanceDuePay": {
    message: "Оплатить счёт",
    description: "Link to Stripe's hosted invoice page, opens in a new tab",
  },
  "common.regionFrankfurt": {
    message: "Франкфурт, Германия",
    description:
      "Display name of Render's `frankfurt` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionOhio": {
    message: "Огайо, США",
    description:
      "Display name of Render's `ohio` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionOregon": {
    message: "Орегон, США",
    description:
      "Display name of Render's `oregon` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionSingapore": {
    message: "Сингапур",
    description:
      "Display name of Render's `singapore` region, shown where a resource's region is read (w4/693)",
  },
  "common.regionVirginia": {
    message: "Вирджиния, США",
    description:
      "Display name of Render's `virginia` region, shown where a resource's region is read (w4/693)",
  },
  "common.createOptional": {
    message: "Необязательно",
    description:
      "Mark beside a create-page row label whose field may be left blank (Render's 'Optional')",
  },
  "common.createAdvanced": {
    message: "Дополнительно",
    description: "Heading of the collapsed Advanced card on a create page",
  },
  "common.planListShowAll": {
    message: "Показать все планы",
    description: "Button under a create page's plan list that shows every plan",
  },
  "common.planListShowFewer": {
    message: "Показать меньше планов",
    description:
      "Button under an expanded plan list that folds it back to the first plans",
  },
  "common.createBlockedName": {
    message: "Введите название, чтобы продолжить.",
    description:
      "Create page action bar: why Submit is held while the name is empty or invalid",
  },
  "common.createBlockedFields": {
    message: "Заполните или исправьте поля выше, чтобы продолжить.",
    description:
      "Create page action bar: why Submit is held while a field is missing or invalid",
  },
  "common.createTypePostgres": {
    message: "Postgres",
    description:
      "Postgres in the New menu and a create page's type switcher (Render's product name)",
  },
  "common.createTypeSwitcher": {
    message: "{type}: выбрать тип сервиса",
    description:
      "Accessible name of the breadcrumb menu that switches a create page to another resource type ({type} = the current page's heading)",
  },
  "common.createTypeProject": {
    message: "Проект",
    description: "Project in the New menu (Render's bare 'Project')",
  },
  "common.createTypeBlueprint": {
    message: "Blueprint",
    description: "Blueprint in the New menu (Render's bare 'Blueprint')",
  },
  "common.createTypeBuildSource": {
    message: "Источник сборки",
    description: "Build Source in the New menu",
  },
  "common.createTypeSandboxGroup": {
    message: "Группа песочниц",
    description:
      "Sandbox Group in the New menu (Render's bare 'Sandbox Group')",
  },
  "common.createTypeWebhook": {
    message: "Вебхук",
    description:
      "Webhook in the New menu (bex's own create, after Render's entries)",
  },
  "common.createBlockedNameTaken": {
    message: "Выберите незанятое название, чтобы продолжить.",
    description:
      "Create page action bar: why Submit is held while the name is already in use",
  },
  "common.createRequired": {
    message: "Обязательно",
    description:
      "Mark beside a create-page row label whose field must be filled before Submit",
  },
  "common.createBlockedLoadingPlans": {
    message: "Загрузка тарифов…",
    description:
      "Create page action bar: Submit waits only for the plan catalog to load (not the user's fault)",
  },
  "common.datastoreViewDocs": {
    message: "Документация",
    description:
      "A Postgres's or Key Value's header link to bex's docs for it, opening in a new tab (Render's View docs, w3/m675)",
  },
  "common.datastoreConnect": {
    message: "Подключиться",
    description:
      "A Postgres's or Key Value's header button opening its connection strings (Render's Connect, w3/m675)",
  },
  "common.datastoreConnectInternal": {
    message: "Внутренний",
    description:
      "Connect menu tab: the private-network URL (Render's Internal tab)",
  },
  "common.datastoreConnectExternal": {
    message: "Внешний",
    description:
      "Connect menu tab: the public URL and the CLI command (Render's External tab)",
  },
  "common.datastoreConnectInternalBody": {
    message:
      "Все ваши сервисы bex в том же регионе могут {link} по частной сети.",
    description:
      "Connect menu's Internal line; {link} is common.datastoreConnectInternalLink, linking the docs",
  },
  "common.datastoreConnectInternalLink": {
    message: "обмениваться данными внутри",
    description: "Linked words inside common.datastoreConnectInternalBody",
  },
  "common.datastoreConnectExternalBody": {
    message: "Подключайтесь из {link} bex.",
    description:
      "Connect menu's External line once an IP rule admits traffic; {link} is common.datastoreConnectExternalLink, linking the docs",
  },
  "common.datastoreConnectExternalLink": {
    message: "сервисов за пределами",
    description: "Linked words inside common.datastoreConnectExternalBody",
  },
  "common.datastoreConnectExternalClosed": {
    message:
      "Внешний трафик не разрешён. Добавьте IP-адреса в разделе «{link}».",
    description:
      "Connect menu's External tab while no IP rule admits traffic (Render's note); {link} is the Info card holding the IP rules, named as that card is",
  },
  "common.datastoreConnectCliBody": {
    message: "Подключайтесь с помощью {link}",
    description:
      "Connect menu's line above the CLI command; {link} is common.datastoreConnectCliLink, linking the CLI docs",
  },
  "common.datastoreConnectCliLink": {
    message: "CLI bex",
    description:
      "Linked words inside common.datastoreConnectCliBody: the bex command-line tool",
  },
  "common.datastoreConnectRetry": {
    message: "Не удалось загрузить. Попробовать снова?",
    description:
      "Accessible name and tooltip of the retry button on a Connect menu value that failed to load",
  },
};

export default ruCommon;
