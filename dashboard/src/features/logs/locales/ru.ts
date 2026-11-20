import type { TranslationEntry } from "@/i18n";

const ruLogs: Record<string, TranslationEntry> = {
  "logs.rangeLabel": {
    message: "Период истории логов",
    description: "Accessible label for the relative log-history range controls",
  },
  "logs.rangeHistoryNote": {
    message:
      "Период ограничивает историю. В режиме реального времени новые строки добавляются по мере поступления.",
    description:
      "Explains how the bounded history window interacts with live tail",
  },
  "logs.retentionWindow_one": {
    message: "На тарифе {plan} логи хранятся {count} день.",
    description:
      "Log range picker hint: how many days the workspace plan keeps logs (w9/m389, from bex-api logRetention)",
  },
  "logs.retentionWindow_few": {
    message: "На тарифе {plan} логи хранятся {count} дня.",
    description:
      "Log range picker hint: how many days the workspace plan keeps logs (w9/m389, from bex-api logRetention)",
  },
  "logs.retentionWindow_many": {
    message: "На тарифе {plan} логи хранятся {count} дней.",
    description:
      "Log range picker hint: how many days the workspace plan keeps logs (w9/m389, from bex-api logRetention)",
  },
  "logs.retentionWindow_other": {
    message: "На тарифе {plan} логи хранятся {count} дня.",
    description:
      "Log range picker hint: how many days the workspace plan keeps logs (w9/m389, from bex-api logRetention)",
  },
  "logs.retentionUpgrade_one": {
    message: "Перейдите на {plan}, чтобы хранить логи {count} день.",
    description:
      "Log range picker hint: the cheapest plan that keeps logs longer, and its days (w9/m389)",
  },
  "logs.retentionUpgrade_few": {
    message: "Перейдите на {plan}, чтобы хранить логи {count} дня.",
    description:
      "Log range picker hint: the cheapest plan that keeps logs longer, and its days (w9/m389)",
  },
  "logs.retentionUpgrade_many": {
    message: "Перейдите на {plan}, чтобы хранить логи {count} дней.",
    description:
      "Log range picker hint: the cheapest plan that keeps logs longer, and its days (w9/m389)",
  },
  "logs.retentionUpgrade_other": {
    message: "Перейдите на {plan}, чтобы хранить логи {count} дня.",
    description:
      "Log range picker hint: the cheapest plan that keeps logs longer, and its days (w9/m389)",
  },
  "logs.typeLabel": {
    message: "Тип логов",
    description: "Accessible label for the log-type filter dropdown",
  },
  "logs.typeAll": {
    message: "Все логи",
    description: "Log-type filter option: no type filter",
  },
  "logs.typeApplication": {
    message: "Логи приложения",
    description: "Log-type filter option: application (app) logs",
  },
  "logs.typeRequest": {
    message: "Логи запросов",
    description:
      "Log-type filter option: request logs (Traefik access lines, from the durable store)",
  },
  "logs.levelLabel": {
    message: "Уровень",
    description: "Accessible label for the log-level filter dropdown",
  },
  "logs.levelAll": {
    message: "Все уровни",
    description: "Level filter option: no level filter",
  },
  "logs.methodLabel": {
    message: "Метод",
    description: "Accessible label for the HTTP-method filter dropdown",
  },
  "logs.methodAll": {
    message: "Все методы",
    description: "Method filter option: no method filter",
  },
  "logs.statusCodeLabel": {
    message: "Код статуса",
    description: "Accessible label for the HTTP-status filter dropdown",
  },
  "logs.statusCodeAll": {
    message: "Все статусы",
    description: "Status-code filter option: no status filter",
  },
  "logs.instanceLabel": {
    message: "Инстанс",
    description: "Accessible label for the instance (replica) filter dropdown",
  },
  "logs.instanceAll": {
    message: "Все инстансы",
    description: "Instance filter option: no instance filter",
  },
  "logs.filterByInstance": {
    message: "Фильтровать логи по инстансу {instance}",
    description:
      "Accessible label for the clickable instance slug on an application log line",
  },
  "logs.pathLabel": {
    message: "Путь запроса",
    description: "Accessible label for the request-path filter input",
  },
  "logs.filtersButton": {
    message: "Фильтры",
    description:
      "Trigger for the structured-filters popover (level/method/status/instance/path)",
  },
  "logs.chipRemove": {
    message: "Убрать фильтр {label}",
    description:
      "Accessible label for an active-filter chip's clear button; {label} is the filter's field name",
  },
  "logs.pathPlaceholder": {
    message: "Фильтр по пути",
    description: "Placeholder for the request-path filter input",
  },
  "logs.hostLabel": {
    message: "Хост запроса",
    description:
      "Accessible label for the request-host filter input and its chip (Render's host log filter, w4/538)",
  },
  "logs.hostPlaceholder": {
    message: "Фильтр по хосту",
    description: "Placeholder for the request-host filter input",
  },
  "logs.liveUnsupported": {
    message:
      "Просмотр в реальном времени отключён — метод, статус, путь и хост фильтруют логи запросов, поэтому логи приложения с ними отслеживать нельзя.",
    description:
      "Note when the chosen log type cannot carry an active filter, so live tail is off (a request filter on app logs)",
  },
  "logs.liveWindowClosed": {
    message:
      "Просмотр в реальном времени отключён — этот пользовательский период уже закончился, поэтому новые строки в нём появиться не могут.",
    description:
      "Note when the selected custom time range ended before it was applied, so live tail is off",
  },
  "logs.storeRequiredTitle": {
    message: "Для логов запросов нужно хранилище логов",
    description:
      "Empty-state title when a request/structured-filter query hits a deployment with no durable store (503)",
  },
  "logs.storeRequiredBody": {
    message:
      "Логи запросов и фильтры по уровню, статусу, методу и пути читаются из постоянного хранилища логов, которое здесь не настроено. Они доступны там, где хранилище подключено.",
    description: "Empty-state body for the store-unavailable (503) state",
  },
  "logs.requestPlanTitle": {
    message: "Для HTTP-логов запросов нужен тариф Pro",
    description:
      "Notice title in place of Request logs on a Hobby workspace (PLAN_FEATURE_REQUIRED, w8/m138)",
  },
  "logs.requestPlanBody": {
    message:
      "Логи запросов доступны на тарифе Pro и выше. Это рабочее пространство на тарифе Hobby, поэтому показываются только логи приложения и сборки.",
    description:
      "Notice body in place of Request logs on a Hobby workspace (w8/m138)",
  },
  "logs.searchPlaceholder": {
    message: "Поиск по логам",
    description: "Placeholder for the log search box (text filter)",
  },
  "logs.live": {
    message: "В реальном времени",
    description: "Label for the live-tail on/off toggle",
  },
  "logs.linesLabel": {
    message: "Строки логов",
    description:
      "Accessible name of the scrollable log list, which takes keyboard focus so arrow keys and Page Up/Down scroll it",
  },
  "logs.linesKeysHint": {
    message:
      "Стрелками вверх и вниз переходите между строками, клавишей Enter показывайте только экземпляр этой строки, а Shift+F10 открывайте её меню.",
    description:
      "Logs tab: screen-reader description of the focused log list's keys - arrows move the row cursor, Enter shows only that line's instance, Shift+F10 opens the line's menu (w3/m528)",
  },
  "logs.linesKeysShort": {
    message:
      "↑↓ переход по строкам · Enter: только этот экземпляр · Shift+F10: меню",
    description:
      "Logs tab: the keys of the focused log list, shown in its corner while it has keyboard focus - arrows move between lines, Enter shows only that line's instance, Shift+F10 opens the line's menu",
  },
  "logs.returnToBottom": {
    message: "Вернуться вниз",
    description:
      "Log list (w3/m528): accessible name and tooltip of Render's icon-only Return to Bottom button, shown while the reader is scrolled away from the newest line (ascending order)",
  },
  "logs.returnToTop": {
    message: "Вернуться наверх",
    description:
      "Log list (w3/m528): accessible name and tooltip of Render's icon-only Return to Top button, shown while the reader is scrolled away from the newest line (descending order)",
  },
  "logs.newLogs": {
    message: "Новые логи",
    description:
      "Log list (w3/m528): Render's held-lines button while a live tail holds fewer than 10 new lines back from a reader scrolled away",
  },
  "logs.newLogsCount_one": {
    message: "{count}+ новый лог",
    description:
      "Log list (w3/m528): Render's held-lines button for 10 or more held lines; count is the held count rounded up to a multiple of 5, at most 100, followed by a plus sign",
  },
  "logs.newLogsCount_few": {
    message: "{count}+ новых лога",
    description:
      "Log list (w3/m528): Render's held-lines button for 10 or more held lines; count is the held count rounded up to a multiple of 5, at most 100, followed by a plus sign",
  },
  "logs.newLogsCount_many": {
    message: "{count}+ новых логов",
    description:
      "Log list (w3/m528): Render's held-lines button for 10 or more held lines; count is the held count rounded up to a multiple of 5, at most 100, followed by a plus sign",
  },
  "logs.newLogsCount_other": {
    message: "{count}+ нового лога",
    description:
      "Log list (w3/m528): Render's held-lines button for 10 or more held lines; count is the held count rounded up to a multiple of 5, at most 100, followed by a plus sign",
  },
  "logs.newLogsInLiveTail": {
    message: "Новые логи в реальном времени",
    description:
      "Log list (w3/m528): Render's button on a paused explorer when a new line arrived; clicking it turns Live on",
  },
  "logs.switchToLiveTail": {
    message: "Перейти в режим реального времени",
    description:
      "Log list (w3/m528): tooltip of the New logs in Live Tail button",
  },
  "logs.dismiss": {
    message: "Скрыть",
    description:
      "Log list (w3/m528): accessible name of the x that hides the New logs in Live Tail button",
  },
  "logs.lineMenu": {
    message: "Действия со строкой",
    description:
      "Log list (w3/m528): accessible name and tooltip of the row menu button (...) on a hovered log line",
  },
  "logs.copyLogLine": {
    message: "Копировать строку лога",
    description:
      "Log row menu (w3/m528): copies the line without its time, as Render's Copy log line",
  },
  "logs.lineCopied": {
    message: "Скопировано!",
    description:
      "Log row menu (w3/m528): toast after Copy log line, Render's 'Copied!'",
  },
  "logs.lineCopyFailed": {
    message: "Не удалось скопировать строку лога",
    description:
      "Log row menu (w3/m528): toast when the clipboard refused the copied line",
  },
  "logs.viewInContext": {
    message: "Показать в контексте",
    description:
      "Log row menu (w3/m528): opens a new tab on the same explorer, unfiltered, 10 minutes either side of the line",
  },
  "logs.sshIntoInstance": {
    message: "Подключиться к экземпляру по SSH",
    description:
      "Log row menu (w3/m528): opens the service's Shell on the line's instance in a new tab",
  },
  "logs.sshInstanceNotRunning": {
    message:
      "По SSH можно подключиться только к работающим экземплярам. Этот недоступен.",
    description:
      "Log row menu (w3/m528): Render's tooltip on a disabled SSH into instance item, for an instance that is no longer running",
  },
  "logs.linesKeysHintMenu": {
    message:
      "Стрелками вверх и вниз переходите между строками, а Shift+F10 открывайте меню строки.",
    description:
      "Log list without an instance filter (w3/m528): screen-reader description of its keys - arrows move the row cursor, Shift+F10 opens the line's menu",
  },
  "logs.linesKeysShortMenu": {
    message: "↑↓ переход по строкам · Shift+F10: меню",
    description:
      "Log list without an instance filter (w3/m528): its keys, shown in its corner while it has keyboard focus",
  },
  "logs.themeSettings": {
    message: "Настройки темы",
    description:
      "Log explorer Options menu (w3/m528): Render's last item, a link to the Appearance section of account Settings",
  },
  "logs.endOfLog": {
    message: "Конец лога",
    description:
      "Closing line after the last row of a log that will not grow (a finished deploy's), so a short log reads as complete",
  },
  "logs.loading": {
    message: "Загрузка логов…",
    description: "Shown while the first historical page is loading",
  },
  "logs.loadingOlder": {
    message: "Загрузка более ранних логов…",
    description:
      "Shown at the top of the pane while a page-back request is in flight",
  },
  "logs.loadOlder": {
    message: "Загрузить более ранние",
    description:
      "Button in the truncation notice that fetches the next older page — the keyboard/underfilled-pane path to what scrolling to the top does",
  },
  "logs.truncatedNotice": {
    message:
      "Загружена только часть этого периода. Загрузите более ранние записи, чтобы продолжить.",
    description:
      "Banner when hasMore is true: more of the selected range remains unloaded, however many older pages are already shown, so it names no line count (w4/280)",
  },
  "logs.partialSearchNotice": {
    message:
      "Пока просмотрена только самая новая часть этого периода — загрузите более ранние записи, чтобы продолжить поиск.",
    description:
      "Banner when a time-boxed search returned less than a page but hasMore is true: the older part of the range is unsearched, not empty (w4/m140)",
  },
  "logs.timeoutTitle": {
    message: "Этот поиск занял слишком много времени",
    description:
      "State title when the logs query ran out of the server's time budget (QUERY_TIMEOUT, w4/m140)",
  },
  "logs.timeoutBody": {
    message:
      "Попробуйте выбрать более короткий период или более точный поисковый запрос.",
    description: "State body under logs.timeoutTitle",
  },
  "logs.streaming": {
    message: "В реальном времени — поступают новые строки",
    description: "Status under the log list when the SSE tail is connected",
  },
  "logs.paused": {
    message: "Просмотр в реальном времени приостановлен",
    description: "Status under the log list when live tail is toggled off",
  },
  "logs.disconnected": {
    message: "Просмотр в реальном времени отключён — повторное подключение…",
    description: "Banner when the SSE stream drops",
  },
  "logs.emptyTitle": {
    message: "В этом периоде нет логов",
    description:
      "Empty-state title when the selected (always-bounded) history range holds no lines. Never claims the service has never logged — the range can't establish that (w6/m111)",
  },
  "logs.emptyBody": {
    message: "В этом периоде нет строк логов.",
    description:
      "Empty-state body with no filters applied — honest about the bounded window, not the service's whole history (w6/m111)",
  },
  "logs.emptyFilteredBody": {
    message: "Нет логов, соответствующих этим фильтрам.",
    description:
      "Empty-state body when a type/text/structured filter yields nothing",
  },
  "logs.emptyFilteredTitle": {
    message: "Подходящих логов нет",
    description:
      "Empty-state title when a type/text/structured filter yields nothing",
  },
  "logs.errorTitle": {
    message: "Не удалось загрузить логи",
    description: "Error-state title when the logs query fails",
  },
  "logs.errorBody": {
    message:
      "Запрос за этот период не выполнен. Повторите попытку или выберите более короткий период.",
    description:
      "Logs tab: body when the log history read failed, under logs.errorTitle; the server's own reason follows as a detail line",
  },
  "logs.errorBodyLive": {
    message:
      "Новые строки по-прежнему будут появляться здесь по мере поступления. Повторите попытку, чтобы загрузить историю за этот период.",
    description:
      "Logs tab: body when the history read failed while the live tail still streams \u2014 new lines will still land in the panel",
  },
  "logs.filtersDone": {
    message: "Готово",
    description: "Logs tab: button closing the Filters panel",
  },
  "logs.emptyBodyRange": {
    message:
      "Выберите более длинный период или включите режим «В реальном времени», чтобы видеть новые строки.",
    description:
      "Logs tab: body when the chosen range holds no lines, under logs.emptyTitle \u2014 what to try next",
  },
  "logs.turnOnLive": {
    message: "Включить режим реального времени",
    description:
      "Logs tab: button in the empty state that switches the live tail on",
  },
  "logs.clearFilters": {
    message: "Сбросить фильтры",
    description:
      "Logs tab: button in the no-match state that clears the search and every filter",
  },
  "logs.errorTitleWithReason": {
    message: "Не удалось загрузить логи: {reason}",
    description:
      "Inline banner over loaded log lines when a later read was refused; reason is the server's refusal",
  },
  "logs.observabilityViewEyebrow": {
    message: "Представление наблюдаемости",
    description:
      "Eyebrow over a project observability view's name (Render's OBSERVABILITY VIEW, w3/m505)",
  },
  "logs.applicationLogsTitle": {
    message: "Логи приложений",
    description:
      "Document title part for a project's Application Logs view page (w3/m505)",
  },
  "logs.requestLogsTitle": {
    message: "Логи запросов",
    description:
      "Document title part for a project's Request Logs view page (w3/m505)",
  },
  "logs.projectScopeEmptyTitle": {
    message: "В этом представлении пока нечего читать",
    description:
      "Project log view state: the view's scope holds no resource to read (w3/m505)",
  },
  "logs.projectScopeEmptyBody": {
    message:
      "Это представление читает сервисы, cron-задания, базы данных Postgres и экземпляры Key Value проекта, но их пока нет.",
    description:
      "Project Application Logs view with no service, cron job, Postgres or Key Value in it (w3/m505)",
  },
  "logs.projectRequestScopeEmptyBody": {
    message:
      "Логи запросов читают веб-сервисы проекта, но в этом представлении их пока нет.",
    description:
      "Project Request Logs view with no web service in it (w3/m505)",
  },
  "logs.projectRoleLimitTitle": {
    message: "Вы не можете просматривать логи",
    description:
      "Project log view state: the caller's workspace role reads no logs, so the view reads nothing (w3/078)",
  },
  "logs.projectRoleLimitBody": {
    message:
      "Для просмотра логов в этом рабочем пространстве нужна роль «Участник-исполнитель» или выше. Попросите администратора рабочего пространства изменить вашу роль.",
    description:
      "Project log view state body: logs need the Contributor role or higher, as on Render (w3/078)",
  },
  "logs.projectEmptyTitle": {
    message: "Нет логов за этот период",
    description:
      "Project log view state: its resources logged nothing in the selected range (w3/m505)",
  },
  "logs.projectEmptyBody": {
    message:
      "Ни один ресурс этого представления ничего не записал за выбранный период.",
    description:
      "Project log view state body: no line in the range from any of its resources (w3/m505)",
  },
  "logs.viewNotFoundTitle": {
    message: "Представление не найдено",
    description:
      "Project observability page whose savedSearchId names no view of this project (w3/m505)",
  },
  "logs.viewNotFoundBody": {
    message:
      "В этом проекте нет представления по этой ссылке. Возможно, оно было удалено.",
    description: "Body of the view-not-found state (w3/m505)",
  },
  "logs.viewOpen": {
    message: "Открыть {name}",
    description:
      "Link from the view-not-found state to the project's default view of the same type; name is the view's name",
  },
  "logs.viewLoadErrorTitle": {
    message: "Не удалось загрузить это представление",
    description:
      "Project observability page whose saved searches could not be read (w3/m505)",
  },
  "logs.viewLoadErrorBody": {
    message:
      "Не удалось прочитать представления проекта. Перезагрузите страницу, чтобы повторить попытку.",
    description: "Body of the view load failure (w3/m505)",
  },
  "logs.newView": {
    message: "Новое представление",
    description:
      "Project sidebar button that opens the TYPE menu for a new observability view (w3/m507, Render's '+ New View')",
  },
  "logs.newViewTypeHeading": {
    message: "Тип",
    description:
      "Heading of the New View menu listing view types (Render's TYPE)",
  },
  "logs.editView": {
    message: "Изменить представление",
    description:
      "Observability view header button that opens the Edit view modal (w3/m507)",
  },
  "logs.duplicateView": {
    message: "Дублировать",
    description:
      "Observability view header button that copies the view (w3/m507)",
  },
  "logs.deleteView": {
    message: "Удалить представление",
    description:
      "Accessible name and confirm label of the observability view's trash button (w3/m507)",
  },
  "logs.deleteViewTitle": {
    message: "Удалить {name}?",
    description:
      "Title of the confirmation before deleting an observability view; name is the view's name",
  },
  "logs.deleteViewBody": {
    message:
      "Представление будет удалено для всех участников проекта. Ресурсы проекта и их логи не затрагиваются.",
    description:
      "Body of the delete-view confirmation: views are shared by the whole project (w3/m507)",
  },
  "logs.viewDeleted": {
    message: "{name} удалено",
    description:
      "Toast after an observability view was deleted; name is the view's name",
  },
  "logs.deleteViewFailed": {
    message: "Не удалось удалить представление",
    description:
      "Toast when deleting an observability view failed without a server reason",
  },
  "logs.viewDuplicated": {
    message: "{name} создано",
    description: "Toast after a view was duplicated; name is the copy's name",
  },
  "logs.duplicateViewFailed": {
    message: "Не удалось дублировать представление",
    description:
      "Toast when duplicating an observability view failed without a server reason",
  },
  "logs.editViewTitle": {
    message: "Изменить представление",
    description: "Title of Render's Edit view modal (w3/m507)",
  },
  "logs.newViewTitle": {
    message: "Новое представление",
    description: "Title of the view modal when New View opened it (w3/m507)",
  },
  "logs.viewNameLabel": {
    message: "Название",
    description: "Label of the view name field in the Edit view modal",
  },
  "logs.viewNameRequired": {
    message: "Укажите название представления.",
    description: "Inline refusal of a blank view name",
  },
  "logs.viewFiltersLabel": {
    message: "Фильтры",
    description: "Heading of the filter rows in the Edit view modal",
  },
  "logs.viewAddFilter": {
    message: "Добавить фильтр",
    description: "Accessible name of the Edit view modal's add (+) button",
  },
  "logs.viewNoFilters": {
    message: "Без фильтров: представление читает все ресурсы проекта.",
    description:
      "Edit view modal with no filter rows: the view reads the whole project",
  },
  "logs.viewSharedNotice": {
    message:
      "Изменение фильтров обновит представление для всех участников проекта.",
    description:
      "Render's notice under the Edit view filters: a view is shared by the whole project",
  },
  "logs.viewSaveFailed": {
    message: "Не удалось сохранить представление. Попробуйте ещё раз.",
    description:
      "Edit view modal error when saving failed without a server reason",
  },
  "logs.viewClose": {
    message: "Закрыть",
    description:
      "Edit view modal button that discards the changes (Render's Close)",
  },
  "logs.viewDone": {
    message: "Готово",
    description: "Edit view modal button that saves the view (Render's Done)",
  },
  "logs.viewFilterField": {
    message: "Поле",
    description: "Accessible name of a filter row's Field picker",
  },
  "logs.viewFilterOperator": {
    message: "Оператор",
    description: "Accessible name of a filter row's Operator picker",
  },
  "logs.viewFilterValuesFor": {
    message: "Значения для {field}",
    description:
      "Accessible name of a filter row's Values picker; field is the row's field",
  },
  "logs.viewValuesRequired": {
    message: "Выберите хотя бы одно значение.",
    description: "Inline refusal of a filter row with no value",
  },
  "logs.viewRemoveFilter": {
    message: "Удалить фильтр {field}",
    description:
      "Accessible name of a filter row's trash button; field is the row's field",
  },
  "logs.viewOperatorIs": {
    message: "равно",
    description:
      "Filter operator IS for a single-valued field (Environment, Time Range)",
  },
  "logs.viewOperatorIsOneOf": {
    message: "одно из",
    description: "Filter operator IS for a field that takes several values",
  },
  "logs.viewOperatorIsNotOneOf": {
    message: "не одно из",
    description: "Filter operator IS_NOT",
  },
  "logs.viewValuesPlaceholder": {
    message: "Выберите значения",
    description:
      "Placeholder of a filter row's Values picker with nothing chosen",
  },
  "logs.viewNoResources": {
    message: "В этом проекте нет ресурсов",
    description:
      "Resource picker of a project with no resource the view can read",
  },
  "logs.viewValueTypeHint": {
    message: "Введите значение и нажмите Enter",
    description:
      "Placeholder of a typed filter value (instance, search text, status code, host, path)",
  },
  "logs.viewValueRemove": {
    message: "Удалить {value}",
    description:
      "Accessible name of a typed filter value's remove button; value is the value",
  },
  "logs.viewValueSearch": {
    message: "Найдите или введите значение",
    description:
      "Placeholder of the search box in a filter's value picker fed by the project's logs (Instance)",
  },
  "logs.viewValueAdd": {
    message: "Добавить «{value}»",
    description:
      "Picker item that adds the typed text as a filter value; value is the typed text",
  },
  "logs.viewValuesLoading": {
    message: "Чтение логов проекта…",
    description:
      "Value picker status while the project's logs are read for the field's values",
  },
  "logs.viewValuesUnavailable": {
    message:
      "Не удалось прочитать логи проекта. Введите значение, чтобы добавить его.",
    description:
      "Value picker status when the project's logs could not be read for the field's values",
  },
  "logs.viewValuesNoneSeen": {
    message:
      "В логах проекта за последние 7 дней ничего нет. Введите значение, чтобы добавить его.",
    description:
      "Value picker status when the project's logs hold no value of the field in the last 7 days",
  },
  "logs.viewFieldEnvironment": {
    message: "Окружение",
    description: "Filter field Environment in the Edit view modal",
  },
  "logs.viewFieldResource": {
    message: "Ресурс",
    description: "Filter field Resource in the Edit view modal",
  },
  "logs.viewFieldResourceType": {
    message: "Тип ресурса",
    description: "Filter field Resource Type in the Edit view modal",
  },
  "logs.viewFieldText": {
    message: "Поиск по тексту",
    description: "Filter field Text Search in the Edit view modal",
  },
  "logs.viewFieldTimeRange": {
    message: "Диапазон времени",
    description: "Filter field Time Range in the Edit view modal",
  },
  "logs.viewResourceTypeService": {
    message: "Сервисы",
    description: "Resource Type value Services",
  },
  "logs.viewResourceTypeCronJob": {
    message: "Cron-задачи",
    description: "Resource Type value Cron jobs",
  },
  "logs.viewResourceTypePostgres": {
    message: "Postgres",
    description: "Resource Type value Postgres",
  },
  "logs.viewResourceTypeKeyValue": {
    message: "Key Value",
    description: "Resource Type value Key Value (the product name)",
  },
  "logs.lineLevel": {
    message: "Уровень: {level}",
    description:
      "Log line level gutter (w3/m509): accessible name and hover text of the level icon leading a log row; {level} is the line's level word (info, warning, error, \u2026)",
  },
  "logs.showMore": {
    message: "Показать полностью",
    description:
      "Log row (w3/m509): button after a long message cut short, showing the whole message",
  },
  "logs.showLess": {
    message: "Свернуть",
    description:
      "Log row (w3/m509): button after a long message opened in full, cutting it short again",
  },
  "logs.copied": {
    message: "Логи скопированы",
    description:
      "Log explorer (w3/m509): toast after Options > Copy logs wrote the lines shown to the clipboard",
  },
  "logs.copyFailed": {
    message: "Не удалось скопировать логи",
    description:
      "Log explorer (w3/m509): toast when the clipboard refused the copied logs",
  },
  "logs.copyLogs": {
    message: "Скопировать логи",
    description:
      "Log explorer Options menu (w3/m509): item copying every line shown to the clipboard (Render's Copy logs)",
  },
  "logs.options": {
    message: "Параметры логов",
    description:
      "Log explorer (w3/m509): accessible name and tooltip of the Options (\u2026) menu button",
  },
  "logs.orderBy": {
    message: "Порядок",
    description:
      "Log explorer Options menu (w3/m509): heading of the line order choice (Render's Order by)",
  },
  "logs.orderAscending": {
    message: "По возрастанию",
    description:
      "Log explorer Options menu (w3/m509): order choice, oldest line first and the newest at the bottom",
  },
  "logs.orderDescending": {
    message: "По убыванию",
    description:
      "Log explorer Options menu (w3/m509): order choice, newest line first at the top",
  },
  "logs.display": {
    message: "Отображение",
    description:
      "Log explorer Options menu (w3/m509): heading of the display options (Render's Display)",
  },
  "logs.expandEverything": {
    message: "Развернуть всё",
    description:
      "Log explorer Options menu (w3/m509): checkbox showing every long message in full (Render's Expand everything)",
  },
  "logs.maximize": {
    message: "Развернуть",
    description:
      "Log explorer (w3/m509): button and menu item expanding the log to fill the screen (Render's Maximize)",
  },
  "logs.minimize": {
    message: "Выйти из полноэкранного режима",
    description:
      "Log explorer (w3/m509): button and menu item restoring the maximized log to its place in the page",
  },
  "logs.maximizeHint": {
    message: "Развернуть (M)",
    description:
      "Log explorer (w3/m509): tooltip of the Maximize button, naming its M shortcut",
  },
  "logs.minimizeHint": {
    message: "Выйти из полноэкранного режима (M или Esc)",
    description:
      "Log explorer (w3/m509): tooltip of the button leaving full screen, naming the M and Escape keys that also leave it",
  },
  "logs.clearedTitle": {
    message: "Логи очищены",
    description:
      "Log explorer (w3/m509): title over a live tail whose lines the reader cleared with Shift+L",
  },
  "logs.clearedBody": {
    message: "Новые строки появятся здесь по мере поступления.",
    description:
      "Log explorer (w3/m509): body under the cleared live tail's title",
  },
  "logs.clearedRestore": {
    message: "Показать прежние строки",
    description:
      "Log explorer (w3/m509): button under a cleared live tail bringing back the lines Shift+L cleared",
  },
};

export default ruLogs;
