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
      "A cron job's USD price per minute of run time, on a Change Instance Type card (w4/546): it is billed per second only while a run executes; price is an API-supplied decimal without the currency symbol",
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
  "common.planUnavailable": {
    message: "Недоступно",
    description:
      "Shown in place of the price on a disabled plan card whose instance type is paused (it cannot be selected for now); the server's full reason is the card's tooltip",
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
  "common.navProjectOverview": {
    message: "Обзор",
    description:
      "Contextual project sidebar nav link to the project's own Overview page",
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
  "common.resourceNotFoundToast": {
    message: "Этот ресурс не существует или был удалён.",
    description:
      "Toast shown after a dead resource URL redirects to the home page (w9/m55)",
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
      "Timestamp tooltip: label of the row showing the instant as Unix seconds",
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
};

export default ruCommon;
