import type { TranslationEntry } from "@/i18n";

const ru: Record<string, TranslationEntry> = {
  "sandboxGroupPages.navSandboxes": {
    message: "Песочницы",
    description: "Sidebar entry and heading: a sandbox group's sandboxes",
  },
  "sandboxGroupPages.navSnapshots": {
    message: "Снимки",
    description: "Sidebar entry and heading: a sandbox group's snapshots",
  },
  "sandboxGroupPages.navImages": {
    message: "Пользовательские образы",
    description:
      "Sidebar entry and heading: the images imported into a sandbox group",
  },
  "sandboxGroupPages.navSettings": {
    message: "Настройки",
    description: "Sidebar entry and heading: a sandbox group's settings",
  },
  "sandboxGroupPages.statusCreating": {
    message: "Создаётся",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusRunning": {
    message: "Выполняется",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusResuming": {
    message: "Возобновляется",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusSuspending": {
    message: "Приостанавливается",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusSuspended": {
    message: "Приостановлена",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusErrored": {
    message: "Ошибка",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusTerminated": {
    message: "Завершена",
    description: "Sandbox status",
  },
  "sandboxGroupPages.rangeLabel": {
    message: "Период времени",
    description: "Accessible name of the time range picker",
  },
  "sandboxGroupPages.rangeHour": {
    message: "Последний час",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeDay": {
    message: "Последние 24 часа",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeWeek": {
    message: "Последние 7 дней",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeMonth": {
    message: "Последние 30 дней",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeAll": {
    message: "За всё время",
    description: "Time range option",
  },
  "sandboxGroupPages.searchSandboxes": {
    message: "Поиск по имени, ID песочницы или статусу",
    description: "Placeholder of the Sandboxes search",
  },
  "sandboxGroupPages.searchSnapshots": {
    message: "Поиск снимков",
    description: "Placeholder of the Snapshots search",
  },
  "sandboxGroupPages.searchImages": {
    message: "Поиск образов",
    description: "Placeholder of the Custom images search",
  },
  "sandboxGroupPages.pauseLive": {
    message: "Приостановить обновление в реальном времени",
    description: "Button that stops the list refreshing",
  },
  "sandboxGroupPages.resumeLive": {
    message: "Возобновить обновление в реальном времени",
    description: "Button that restarts the list refreshing",
  },
  "sandboxGroupPages.showHideColumns": {
    message: "Показать или скрыть столбцы",
    description: "Accessible name of the columns menu",
  },
  "sandboxGroupPages.summaryTotal": {
    message: "Всего",
    description: "Summary: runs that ended in the time range",
  },
  "sandboxGroupPages.summaryErrored": {
    message: "С ошибкой",
    description: "Summary: share of runs that ended in an error",
  },
  "sandboxGroupPages.summaryErroredTip": {
    message: "Доля песочниц за выбранный период, завершившихся с ошибкой.",
    description: "Tooltip of the Errored summary",
  },
  "sandboxGroupPages.summaryRunning": {
    message: "Выполняются",
    description: "Summary: sandboxes active now",
  },
  "sandboxGroupPages.summaryRunningTip": {
    message:
      "Песочницы, которые сейчас создаются, выполняются, возобновляются или приостанавливаются, независимо от периода. Они учитываются в лимите параллельных песочниц.",
    description: "Tooltip of the Running summary",
  },
  "sandboxGroupPages.loadRunsError": {
    message: "Не удалось загрузить запуски песочниц",
    description: "The run chart failed to load",
  },
  "sandboxGroupPages.runsChartLabel": {
    message:
      "Завершённые запуски песочниц: успешно — {succeeded}, с ошибкой — {errored}",
    description: "Accessible name of the run chart",
  },
  "sandboxGroupPages.runsSucceeded": {
    message: "Успешно",
    description: "Chart legend: runs that ended without an error",
  },
  "sandboxGroupPages.runsErrored": {
    message: "С ошибкой",
    description: "Chart legend: runs that ended in an error",
  },
  "sandboxGroupPages.selectedCount_one": {
    message: "Выбрана {count} песочница",
    description: "How many rows are selected",
  },
  "sandboxGroupPages.selectedCount_few": {
    message: "Выбрано {count} песочницы",
    description: "How many rows are selected",
  },
  "sandboxGroupPages.selectedCount_many": {
    message: "Выбрано {count} песочниц",
    description: "How many rows are selected",
  },
  "sandboxGroupPages.selectedCount_other": {
    message: "Выбрано {count} песочницы",
    description: "How many rows are selected",
  },
  "sandboxGroupPages.terminateSelected": {
    message: "Завершить",
    description: "Button: terminate the selected sandboxes",
  },
  "sandboxGroupPages.terminateTitle_one": {
    message: "Завершить {count} песочницу?",
    description: "Terminate confirmation title",
  },
  "sandboxGroupPages.terminateTitle_few": {
    message: "Завершить {count} песочницы?",
    description: "Terminate confirmation title",
  },
  "sandboxGroupPages.terminateTitle_many": {
    message: "Завершить {count} песочниц?",
    description: "Terminate confirmation title",
  },
  "sandboxGroupPages.terminateTitle_other": {
    message: "Завершить {count} песочницы?",
    description: "Terminate confirmation title",
  },
  "sandboxGroupPages.terminateDescription_one": {
    message:
      "{count} песочница будет остановлена, и всё, что в ней не сохранено в снимке, будет потеряно.",
    description: "Terminate confirmation body",
  },
  "sandboxGroupPages.terminateDescription_few": {
    message:
      "{count} песочницы будут остановлены, и всё, что в них не сохранено в снимке, будет потеряно.",
    description: "Terminate confirmation body",
  },
  "sandboxGroupPages.terminateDescription_many": {
    message:
      "{count} песочниц будут остановлены, и всё, что в них не сохранено в снимке, будет потеряно.",
    description: "Terminate confirmation body",
  },
  "sandboxGroupPages.terminateDescription_other": {
    message:
      "{count} песочницы будут остановлены, и всё, что в них не сохранено в снимке, будет потеряно.",
    description: "Terminate confirmation body",
  },
  "sandboxGroupPages.terminateConfirm": {
    message: "Завершить",
    description: "Terminate confirmation button",
  },
  "sandboxGroupPages.terminateFailed_one": {
    message: "Не удалось завершить {count} песочницу. Попробуйте ещё раз.",
    description: "Toast: a terminate failed",
  },
  "sandboxGroupPages.terminateFailed_few": {
    message: "Не удалось завершить {count} песочницы. Попробуйте ещё раз.",
    description: "Toast: terminates failed",
  },
  "sandboxGroupPages.terminateFailed_many": {
    message: "Не удалось завершить {count} песочниц. Попробуйте ещё раз.",
    description: "Toast: terminates failed",
  },
  "sandboxGroupPages.terminateFailed_other": {
    message: "Не удалось завершить {count} песочницы. Попробуйте ещё раз.",
    description: "Toast: terminates failed",
  },
  "sandboxGroupPages.terminateStarted_one": {
    message: "Завершается {count} песочница",
    description: "Toast: a terminate was accepted",
  },
  "sandboxGroupPages.terminateStarted_few": {
    message: "Завершаются {count} песочницы",
    description: "Toast: terminates were accepted",
  },
  "sandboxGroupPages.terminateStarted_many": {
    message: "Завершается {count} песочниц",
    description: "Toast: terminates were accepted",
  },
  "sandboxGroupPages.terminateStarted_other": {
    message: "Завершаются {count} песочницы",
    description: "Toast: terminates were accepted",
  },
  "sandboxGroupPages.terminateSandbox": {
    message: "Завершить песочницу",
    description: "Drawer menu item",
  },
  "sandboxGroupPages.sandboxActions": {
    message: "Действия с песочницей",
    description: "Accessible name of the drawer's menu",
  },
  "sandboxGroupPages.loadSandboxError": {
    message: "Не удалось загрузить песочницу",
    description: "A sandbox's drawer failed to load",
  },
  "sandboxGroupPages.loadSandboxesError": {
    message: "Не удалось загрузить песочницы",
    description: "The list failed to load",
  },
  "sandboxGroupPages.noMatchingSandboxes": {
    message: "Нет подходящих песочниц",
    description: "Empty state with filters",
  },
  "sandboxGroupPages.noMatchingSandboxesHelp": {
    message: "Попробуйте другой запрос или расширьте период.",
    description: "Empty state with filters",
  },
  "sandboxGroupPages.clearFilters": {
    message: "Сбросить фильтры",
    description: "Button: reset the search and time range",
  },
  "sandboxGroupPages.firstSandboxTitle": {
    message: "Создайте первую песочницу",
    description: "Empty state title of the Sandboxes page",
  },
  "sandboxGroupPages.firstSandboxDescription": {
    message:
      "Запустите песочницу из CLI или настройте код, чтобы создавать песочницы программно с помощью SDK для TypeScript и Python.",
    description: "Empty state body of the Sandboxes page",
  },
  "sandboxGroupPages.starterCli": {
    message: "CLI",
    description: "Starter tab: the command line",
  },
  "sandboxGroupPages.starterSdk": {
    message: "SDK",
    description: "Starter tab: the SDKs",
  },
  "sandboxGroupPages.starterInstallCli": {
    message: "Установите или обновите CLI до последней версии",
    description: "Starter step",
  },
  "sandboxGroupPages.starterHomebrew": {
    message: "Homebrew",
    description: "Install method",
  },
  "sandboxGroupPages.starterDirect": {
    message: "Прямая загрузка",
    description: "Install method",
  },
  "sandboxGroupPages.starterLogin": {
    message: "Войдите и выберите это рабочее пространство",
    description: "Starter step",
  },
  "sandboxGroupPages.starterCreateSandbox": {
    message: "Создайте песочницу с настройками по умолчанию",
    description: "Starter step",
  },
  "sandboxGroupPages.starterApiKey": {
    message: "Задайте API-ключ",
    description: "Starter step",
  },
  "sandboxGroupPages.starterApiKeyHelp": {
    message:
      "Выполните это в терминале, из которого будете запускать код. SDK читает ключ из переменной окружения RENDER_API_KEY. Замените значение-заглушку на API-ключ bex.",
    description: "Starter step help",
  },
  "sandboxGroupPages.starterInstallSdk": {
    message: "Настройте проект и установите SDK",
    description: "Starter step",
  },
  "sandboxGroupPages.starterPythonNeeds": {
    message:
      "Нужен Python 3.10 или новее. -U обновляет старую версию SDK; версии 1.4.0 и ниже не поддерживают песочницы.",
    description: "Starter step help",
  },
  "sandboxGroupPages.starterNodeNeeds": {
    message:
      "Нужен Node.js 18 или новее. @latest также обновляет уже установленную старую версию SDK.",
    description: "Starter step help",
  },
  "sandboxGroupPages.starterFirstSandboxFile": {
    message: "Инициализируйте первую песочницу",
    description: "Starter step",
  },
  "sandboxGroupPages.starterRun": {
    message: "Запустите код локально",
    description: "Starter step",
  },
  "sandboxGroupPages.starterTypeScript": {
    message: "TypeScript",
    description: "SDK language",
  },
  "sandboxGroupPages.starterPython": {
    message: "Python",
    description: "SDK language",
  },
  "sandboxGroupPages.colSandbox": {
    message: "Песочница",
    description: "Column",
  },
  "sandboxGroupPages.colStatus": { message: "Статус", description: "Column" },
  "sandboxGroupPages.colCreated": { message: "Создана", description: "Column" },
  "sandboxGroupPages.colDuration": {
    message: "Длительность",
    description: "Column",
  },
  "sandboxGroupPages.colPlan": { message: "Тариф", description: "Column" },
  "sandboxGroupPages.colSource": {
    message: "Источник запуска",
    description: "Column: the image or snapshot a sandbox started from",
  },
  "sandboxGroupPages.selectAll": {
    message: "Выбрать все песочницы на этой странице",
    description: "Accessible name of the select-all checkbox",
  },
  "sandboxGroupPages.selectSandbox": {
    message: "Выбрать {name}",
    description: "Accessible name of a row's checkbox",
  },
  "sandboxGroupPages.baseImage": {
    message: "Базовый образ",
    description: "A sandbox started from the default image",
  },
  "sandboxGroupPages.importedImage": {
    message: "Импортированный образ",
    description: "A sandbox started from an imported image",
  },
  "sandboxGroupPages.previousPage": {
    message: "Назад",
    description: "Paging button",
  },
  "sandboxGroupPages.nextPage": {
    message: "Далее",
    description: "Paging button",
  },
  "sandboxGroupPages.switchedToRunning": {
    message:
      "Выбранная песочница остановилась. Открыта другая, работающая песочница.",
    description: "Toast when the open sandbox stops",
  },
  "sandboxGroupPages.copied": {
    message: "Скопировано",
    description: "Copy toast",
  },
  "sandboxGroupPages.copyError": {
    message: "Не удалось скопировать",
    description: "Copy failed toast",
  },
  "sandboxGroupPages.copySandboxId": {
    message: "Копировать ID песочницы",
    description: "Copy button",
  },
  "sandboxGroupPages.copySnapshotId": {
    message: "Копировать ID снимка",
    description: "Copy button",
  },
  "sandboxGroupPages.tabEvents": {
    message: "События",
    description: "Drawer tab",
  },
  "sandboxGroupPages.tabSnapshots": {
    message: "Снимки",
    description: "Drawer tab",
  },
  "sandboxGroupPages.tabDetails": {
    message: "Сведения",
    description: "Drawer tab and section",
  },
  "sandboxGroupPages.tabVersions": {
    message: "Версии",
    description: "Image drawer tab",
  },
  "sandboxGroupPages.sandboxGoneTitle": {
    message: "Песочница не найдена",
    description: "A sandbox that does not exist here",
  },
  "sandboxGroupPages.sandboxGoneDescription": {
    message: "Песочница {id} недоступна вам в этой группе.",
    description: "A sandbox that does not exist here",
  },
  "sandboxGroupPages.loadingEvents": {
    message: "Загрузка событий",
    description: "Events tab loading",
  },
  "sandboxGroupPages.loadEventsError": {
    message: "Не удалось загрузить события. Попробуйте ещё раз чуть позже.",
    description: "Events tab error",
  },
  "sandboxGroupPages.noEvents": {
    message: "Событий пока нет.",
    description: "Events tab empty",
  },
  "sandboxGroupPages.loadEarlierEvents": {
    message: "Загрузить более ранние события",
    description: "Button: the next page of events",
  },
  "sandboxGroupPages.loadingEarlierEvents": {
    message: "Загрузка более ранних событий",
    description: "Loading the next page of events",
  },
  "sandboxGroupPages.eventCreated": {
    message: "Создана",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventReady": {
    message: "Готова",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventSuspending": {
    message: "Приостанавливается",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventSuspended": {
    message: "Приостановлена",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventResuming": {
    message: "Возобновляется",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventTerminated": {
    message: "Завершена",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventErrored": {
    message: "Ошибка",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventCommand": {
    message: "Команда ({operation})",
    description: "An execution event that ran a command",
  },
  "sandboxGroupPages.eventFileTransfer": {
    message: "Передача файлов ({operation})",
    description: "An execution event that moved a file",
  },
  "sandboxGroupPages.exitCode": {
    message: "Код выхода {code}",
    description: "A command's exit code",
  },
  "sandboxGroupPages.eventSnapshot": {
    message: "Снимок",
    description: "A snapshot event",
  },
  "sandboxGroupPages.viewSnapshot": {
    message: "Открыть снимок",
    description: "Link to a snapshot's drawer",
  },
  "sandboxGroupPages.triggerSdk": {
    message: "SDK",
    description: "Trigger: the SDK",
  },
  "sandboxGroupPages.triggerCli": {
    message: "CLI",
    description: "Trigger: the CLI",
  },
  "sandboxGroupPages.triggerApi": {
    message: "API",
    description: "Trigger: the API",
  },
  "sandboxGroupPages.triggerDashboard": {
    message: "Панель",
    description: "Trigger: the dashboard",
  },
  "sandboxGroupPages.triggerIdleTimeout": {
    message: "Тайм-аут простоя",
    description: "Trigger: the idle timeout",
  },
  "sandboxGroupPages.triggerSystem": {
    message: "bex",
    description: "Trigger: the platform itself",
  },
  "sandboxGroupPages.triggerConnection": {
    message: "Новая команда или передача файлов",
    description: "Trigger: a resume a command started",
  },
  "sandboxGroupPages.loading": {
    message: "Загрузка",
    description: "Loading text",
  },
  "sandboxGroupPages.loadSnapshotsError": {
    message: "Не удалось загрузить снимки",
    description: "Snapshots failed to load",
  },
  "sandboxGroupPages.noSnapshotsRunning": {
    message:
      "У этой песочницы пока нет снимков. Создайте снимок с помощью CLI или SDK.",
    description: "Snapshots tab of a running sandbox",
  },
  "sandboxGroupPages.noSnapshotsEnded": {
    message: "Снимки этой песочницы не создавались.",
    description: "Snapshots tab of an ended sandbox",
  },
  "sandboxGroupPages.detailSandboxId": {
    message: "ID песочницы",
    description: "Detail label",
  },
  "sandboxGroupPages.detailGroup": {
    message: "Группа",
    description: "Detail label",
  },
  "sandboxGroupPages.detailPlan": {
    message: "Тариф",
    description: "Detail label",
  },
  "sandboxGroupPages.planSize": {
    message: "{plan} ({cpus} CPU, {memory} ГБ памяти)",
    description: "A plan with its size",
  },
  "sandboxGroupPages.detailCreated": {
    message: "Создана",
    description: "Detail label",
  },
  "sandboxGroupPages.detailEnded": {
    message: "Завершена",
    description: "Detail label",
  },
  "sandboxGroupPages.detailDuration": {
    message: "Длительность",
    description: "Detail label",
  },
  "sandboxGroupPages.detailTimeout": {
    message: "Тайм-аут",
    description: "Detail label",
  },
  "sandboxGroupPages.detailIdleTimeout": {
    message: "Тайм-аут простоя",
    description: "Detail label",
  },
  "sandboxGroupPages.off": {
    message: "Выкл.",
    description: "A timeout that is off",
  },
  "sandboxGroupPages.detailNetwork": {
    message: "Сеть",
    description: "Detail label",
  },
  "sandboxGroupPages.networkAllowAll": {
    message: "Разрешить всё",
    description: "Network policy",
  },
  "sandboxGroupPages.networkDenyAll": {
    message: "Запретить всё",
    description: "Network policy",
  },
  "sandboxGroupPages.networkAllowList": {
    message: "Список разрешённых",
    description: "Network policy",
  },
  "sandboxGroupPages.detailRegion": {
    message: "Регион",
    description: "Detail label",
  },
  "sandboxGroupPages.detailSource": {
    message: "Источник запуска",
    description: "Detail label",
  },
  "sandboxGroupPages.goToSnapshot": {
    message: "Перейти к снимку",
    description: "Link to the snapshot a sandbox started from",
  },
  "sandboxGroupPages.goToImage": {
    message: "Перейти к образу",
    description: "Link to the image a sandbox started from",
  },
  "sandboxGroupPages.detailTrigger": {
    message: "Триггер",
    description: "Detail label: what started the sandbox",
  },
  "sandboxGroupPages.detailUser": {
    message: "Пользователь",
    description: "Detail label: who started the sandbox",
  },
  "sandboxGroupPages.detailError": {
    message: "Ошибка",
    description: "Detail label",
  },
  "sandboxGroupPages.errorUnknown": {
    message:
      "bex не удалось определить, почему в этой песочнице произошла ошибка. Попробуйте ещё раз или обратитесь в поддержку, указав ID песочницы.",
    description: "An errored sandbox with no known reason",
  },
  "sandboxGroupPages.snapshotCapturing": {
    message: "Создаётся",
    description: "Snapshot status",
  },
  "sandboxGroupPages.snapshotAvailable": {
    message: "Доступен",
    description: "Snapshot status",
  },
  "sandboxGroupPages.snapshotFailed": {
    message: "Сбой",
    description: "Snapshot status",
  },
  "sandboxGroupPages.snapshotFailureInternal": {
    message: "Создание снимка прервано внутренней ошибкой.",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailureTimeout": {
    message: "Время создания снимка истекло.",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailureSourceTerminated": {
    message: "Исходная песочница была завершена до окончания создания снимка.",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailureSourceErrored": {
    message:
      "В исходной песочнице произошла ошибка до окончания создания снимка.",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailedHelp": {
    message:
      "Чтобы повторить попытку, создайте новый снимок. Из этого снимка нельзя запускать новые песочницы.",
    description: "After a snapshot failure",
  },
  "sandboxGroupPages.noMatchingSnapshots": {
    message: "Нет подходящих снимков",
    description: "Empty state with filters",
  },
  "sandboxGroupPages.firstSnapshotTitle": {
    message: "Создайте первый снимок",
    description: "Empty state title of the Snapshots page",
  },
  "sandboxGroupPages.firstSnapshotDescription": {
    message:
      "Создайте снимок, чтобы сохранить файлы песочницы. Запускайте новые песочницы с того же состояния, не настраивая всё заново.",
    description: "Empty state body of the Snapshots page",
  },
  "sandboxGroupPages.firstSnapshotNeedsRunning": {
    message: "Чтобы создать снимок, нужна работающая песочница.",
    description: "Empty state note",
  },
  "sandboxGroupPages.starterSnapshotSandbox": {
    message: "Создайте снимок работающей песочницы",
    description: "Starter step",
  },
  "sandboxGroupPages.starterStartFromSnapshot": {
    message: "Создайте песочницы из снимка",
    description: "Starter step",
  },
  "sandboxGroupPages.starterSnapshotCode": {
    message: "Настройте код",
    description: "Starter step",
  },
  "sandboxGroupPages.colSnapshot": { message: "Снимок", description: "Column" },
  "sandboxGroupPages.colSourceSandbox": {
    message: "Исходная песочница",
    description: "Column and detail label",
  },
  "sandboxGroupPages.colSize": {
    message: "Размер",
    description: "Column and detail label",
  },
  "sandboxGroupPages.colCaptured": {
    message: "Создан",
    description: "Column and detail label",
  },
  "sandboxGroupPages.colExpires": {
    message: "Истекает",
    description: "Column and detail label",
  },
  "sandboxGroupPages.never": {
    message: "Никогда",
    description: "A snapshot that never expires",
  },
  "sandboxGroupPages.snapshotGoneTitle": {
    message: "Снимок не найден",
    description: "A snapshot that does not exist here",
  },
  "sandboxGroupPages.snapshotGoneDescription": {
    message:
      "Снимок {id} недоступен вам в этом рабочем пространстве. Возможно, он был удалён или истёк его срок хранения.",
    description: "A snapshot that does not exist here",
  },
  "sandboxGroupPages.detailType": {
    message: "Тип",
    description: "Detail label: snapshot kind",
  },
  "sandboxGroupPages.kindFilesystem": {
    message: "Файловая система",
    description: "Snapshot kind",
  },
  "sandboxGroupPages.kindRuntime": {
    message: "Среда выполнения",
    description: "Snapshot kind",
  },
  "sandboxGroupPages.detailResourceConstraint": {
    message: "Ограничение ресурсов",
    description: "Detail label: the plan a snapshot needs",
  },
  "sandboxGroupPages.detailLifecycle": {
    message: "Жизненный цикл",
    description: "Detail label",
  },
  "sandboxGroupPages.lifecycleKept": {
    message: "Хранится до удаления",
    description: "A snapshot with no time to live",
  },
  "sandboxGroupPages.lifecycleAfterUse": {
    message: "Истекает через {days} дн. после последнего использования",
    description: "A snapshot's time to live",
  },
  "sandboxGroupPages.noSandboxesFromSnapshot": {
    message: "Из этого снимка ещё не запускались песочницы.",
    description: "Snapshot drawer's sandboxes",
  },
  "sandboxGroupPages.restoreTitle": {
    message: "Запустите песочницу из этого снимка",
    description: "Snapshot drawer starter code",
  },
  "sandboxGroupPages.deleteSnapshot": {
    message: "Удалить снимок",
    description: "Button",
  },
  "sandboxGroupPages.deleteSnapshotTitle": {
    message: "Удалить снимок?",
    description: "Confirmation title",
  },
  "sandboxGroupPages.deleteSnapshotDescription": {
    message:
      "Снимок {name} будет удалён безвозвратно. Новые песочницы нельзя будет запустить из него. Песочницы, уже запущенные из него, продолжат работать.",
    description: "Confirmation body",
  },
  "sandboxGroupPages.deleteSnapshotError": {
    message: "Не удалось удалить снимок. Попробуйте ещё раз.",
    description: "Toast",
  },
  "sandboxGroupPages.snapshotDeleted": {
    message: "Снимок {name} удалён",
    description: "Toast",
  },
  "sandboxGroupPages.imageQueued": {
    message: "В очереди",
    description: "Image status",
  },
  "sandboxGroupPages.imageImporting": {
    message: "Импортируется",
    description: "Image status",
  },
  "sandboxGroupPages.imageAvailable": {
    message: "Доступен",
    description: "Image status",
  },
  "sandboxGroupPages.imageUnavailable": {
    message: "Недоступен",
    description: "Image status: no version can start a sandbox",
  },
  "sandboxGroupPages.imageFailed": {
    message: "Сбой",
    description: "Image version status",
  },
  "sandboxGroupPages.imageLatest": {
    message: "Последняя",
    description: "The newest available version of an image",
  },
  "sandboxGroupPages.imageImportingHelp": {
    message:
      "Пока импорт этой версии не завершится, песочницы запускаются из последней доступной версии.",
    description: "A version still importing",
  },
  "sandboxGroupPages.imageFailureTimeout": {
    message: "Импорт занял слишком много времени.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureRetries": {
    message: "Импорт не удался после нескольких попыток.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureCredentials": {
    message:
      "bex не удалось загрузить образ. Проверьте учётные данные реестра.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureNotFound": {
    message: "bex не удалось найти образ. Проверьте имя и тег образа.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailurePull": {
    message: "bex не удалось загрузить образ.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailurePlatform": {
    message:
      "Образ предназначен для {platform}. Песочницы поддерживают только linux/amd64.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureMissing": {
    message:
      "В образе нет {what}, без чего песочницы не запускаются. Используйте образ с оболочкой, например вариант slim или Alpine.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureConvert": {
    message:
      "bex не удалось преобразовать образ. Попробуйте импортировать его ещё раз.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureLayers": {
    message: "bex не удалось прочитать слои образа.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureStart": {
    message: "Не удалось начать импорт.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureStopped": {
    message: "Импорт остановился, не завершившись.",
    description: "Image failure",
  },
  "sandboxGroupPages.loadImagesError": {
    message: "Не удалось загрузить образы",
    description: "Images failed to load",
  },
  "sandboxGroupPages.noMatchingImages": {
    message: "Нет подходящих образов",
    description: "Empty state with a search",
  },
  "sandboxGroupPages.firstImageTitle": {
    message: "Импортируйте первый образ",
    description: "Empty state title of the Custom images page",
  },
  "sandboxGroupPages.firstImageDescription": {
    message:
      "Импортируйте образ из реестра, чтобы запускать из него песочницы. Импортируйте его с помощью CLI или SDK, а затем запускайте из него песочницы.",
    description: "Empty state body of the Custom images page",
  },
  "sandboxGroupPages.starterImportImage": {
    message: "Импортируйте образ (linux/amd64)",
    description: "Starter step",
  },
  "sandboxGroupPages.starterStartFromImage": {
    message: "Создайте песочницы из образа",
    description: "Starter step",
  },
  "sandboxGroupPages.starterImportCode": {
    message: "Импортируйте образы",
    description: "Starter step",
  },
  "sandboxGroupPages.colImage": { message: "Образ", description: "Column" },
  "sandboxGroupPages.colSourceImage": {
    message: "Источник",
    description: "Column: the registry image",
  },
  "sandboxGroupPages.colLatestAvailable": {
    message: "Последняя доступная",
    description: "Column: the newest available version's digest",
  },
  "sandboxGroupPages.colUpdated": {
    message: "Обновлён",
    description: "Column: the newest import",
  },
  "sandboxGroupPages.imageGoneTitle": {
    message: "Образ не найден",
    description: "An image name with no versions",
  },
  "sandboxGroupPages.imageGoneDescription": {
    message: "В этой группе нет образа с именем {name}.",
    description: "An image name with no versions",
  },
  "sandboxGroupPages.detailLatestAvailable": {
    message: "Последняя доступная версия",
    description: "Detail label",
  },
  "sandboxGroupPages.detailImageSource": {
    message: "Источник",
    description: "Detail label: the registry",
  },
  "sandboxGroupPages.detailImage": {
    message: "Образ",
    description: "Detail label",
  },
  "sandboxGroupPages.detailDigest": {
    message: "Дайджест",
    description: "Detail label",
  },
  "sandboxGroupPages.detailVersionId": {
    message: "ID версии",
    description: "Detail label",
  },
  "sandboxGroupPages.noSandboxesFromImage": {
    message: "Из этого образа ещё не запускались песочницы.",
    description: "Image drawer's sandboxes",
  },
  "sandboxGroupPages.general": {
    message: "Общие",
    description: "Settings section",
  },
  "sandboxGroupPages.makeDefault": {
    message: "Сделать группой по умолчанию",
    description: "Button: make this the workspace's default group",
  },
  "sandboxGroupPages.madeDefault": {
    message: "{name} теперь группа песочниц по умолчанию",
    description: "Toast",
  },
  "sandboxGroupPages.save": { message: "Сохранить", description: "Button" },
  "sandboxGroupPages.limitSaved": {
    message: "Лимит параллельных песочниц сохранён",
    description: "Toast",
  },
  "sandboxGroupPages.brokeredCredentials": {
    message: "Исходящие учётные данные",
    description: "What a group delete removes",
  },
  "sandboxGroupPages.deleteGroup": {
    message: "Удалить группу песочниц",
    description: "Button and section",
  },
  "sandboxGroupPages.deleteGroupHelp": {
    message:
      "Удаляет эту группу вместе с её песочницами, снимками, образами и учётными данными.",
    description: "Delete section help",
  },
  "sandboxGroupPages.deleteGroupDefault": {
    message:
      "Группу по умолчанию рабочего пространства нельзя удалить. Сначала сделайте группой по умолчанию другую группу.",
    description: "Delete section help for the default group",
  },
  "sandboxGroupPages.deleteGroupTitle": {
    message: "Удалить группу песочниц?",
    description: "Confirmation title",
  },
  "sandboxGroupPages.deleteGroupDescription": {
    message:
      "Группа песочниц {name} будет удалена безвозвратно вместе с историей запусков, логами и настройками:",
    description: "Confirmation body",
  },
  "sandboxGroupPages.deleteGroupStops": {
    message:
      "Всё, что использует эту группу через SDK, CLI или API, перестанет работать.",
    description: "Confirmation body",
  },
  "sandboxGroupPages.groupDeleted": {
    message: "Группа песочниц {name} удалена",
    description: "Toast",
  },
  "sandboxGroupPages.linkLoadError": {
    message: "Не удалось открыть ссылку. Попробуйте ещё раз чуть позже.",
    description: "A link by id failed to resolve",
  },
  "sandboxGroupPages.backToGroups": {
    message: "Перейти к группам песочниц",
    description: "Button",
  },
};

export default ru;
