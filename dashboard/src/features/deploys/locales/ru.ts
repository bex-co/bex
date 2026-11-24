import type { TranslationEntry } from "@/i18n";

const ruDeploys: Record<string, TranslationEntry> = {
  "deploys.statusCreated": {
    message: "Создан",
    description: "Deploy status: created",
  },
  "deploys.statusQueued": {
    message: "В очереди",
    description: "Deploy status: queued",
  },
  "deploys.statusBuildInProgress": {
    message: "Идёт сборка",
    description: "Deploy status: build_in_progress",
  },
  "deploys.statusBuildFailed": {
    message: "Сборка не удалась",
    description: "Deploy status: build_failed",
  },
  "deploys.statusPreDeployInProgress": {
    message: "Идёт предварительный деплой",
    description: "Deploy status: pre_deploy_in_progress",
  },
  "deploys.statusPreDeployFailed": {
    message: "Предварительный деплой не удался",
    description: "Deploy status: pre_deploy_failed",
  },
  "deploys.statusUpdateInProgress": {
    message: "Выполняется",
    description: "Deploy status: update_in_progress",
  },
  "deploys.statusUpdateFailed": {
    message: "Не удался",
    description: "Deploy status: update_failed",
  },
  "deploys.statusLive": {
    message: "Работает",
    description: "Deploy status: live",
  },
  "deploys.statusCanceled": {
    message: "Отменён",
    description: "Deploy status: canceled",
  },
  "deploys.statusDeactivated": {
    message: "Деактивирован",
    description: "Deploy status: deactivated",
  },
  "deploys.statusUnknown": {
    message: "Неизвестно",
    description: "Deploy status: unrecognized backend value",
  },
  "deploys.imageDigestCopy": {
    message: "Копировать дайджест образа",
    description:
      "Deploy header: accessible label of the button copying the image digest the deploy ran",
  },
  "deploys.imageDigestCopied": {
    message: "Дайджест образа скопирован",
    description: "Deploy header: toast after copying the image digest",
  },
  "deploys.imageDigestCopyError": {
    message: "Не удалось скопировать в буфер обмена",
    description: "Deploy header: toast when copying the image digest fails",
  },
  "deploys.duration": {
    message: "Длительность",
    description: "Deploy header: elapsed build/deploy time label",
  },
  "deploys.factStatus": {
    message: "Статус",
    description:
      "Deploy page: label of the deploy's Status fact (Render's STATUS)",
  },
  "deploys.factDeployed": {
    message: "Развёрнут",
    description:
      "Deploy page: label of the fact giving when the deploy was created (Render's DEPLOYED)",
  },
  "deploys.factSource": {
    message: "Источник",
    description:
      "Deploy page: label of the fact naming the commit or image the deploy ran (Render's SOURCE)",
  },
  "deploys.factPreDeploy": {
    message: "Перед деплоем",
    description:
      "Deploy page: label of the fact giving the outcome of the deploy's pre-deploy command",
  },
  "deploys.preDeployRunning": {
    message: "Выполняется",
    description:
      "Deploy page: Pre-deploy fact value while the pre-deploy command runs",
  },
  "deploys.preDeploySucceeded": {
    message: "Успешно",
    description:
      "Deploy page: Pre-deploy fact value once the pre-deploy command succeeded",
  },
  "deploys.preDeployFailed": {
    message: "Ошибка",
    description:
      "Deploy page: Pre-deploy fact value when the pre-deploy command failed",
  },
  "deploys.preDeployCanceled": {
    message: "Отменён",
    description:
      "Deploy page: Pre-deploy fact value when the pre-deploy command was canceled",
  },
  "deploys.idLabel": {
    message: "ID деплоя",
    description:
      "Deploy page: the deploy's id, named in its copy button's accessible name",
  },
  "deploys.idCopied": {
    message: "ID деплоя скопирован",
    description: "Deploy page: toast after copying the deploy's id",
  },
  "deploys.idCopyFailed": {
    message: "Не удалось скопировать ID деплоя",
    description: "Deploy page: toast when copying the deploy's id fails",
  },
  "deploys.loadErrorTitle": {
    message: "Не удалось загрузить этот деплой",
    description:
      "Deploy page: title of the state shown when the deploy itself could not be read",
  },
  "deploys.loadErrorBody": {
    message: "Сейчас не удалось прочитать этот деплой. Сам деплой не затронут.",
    description:
      "Deploy page: body of the state shown when the deploy itself could not be read; says the deploy is unaffected",
  },
  "deploys.durationValue": {
    message: "Длительность {duration}",
    description: "Deploy row: elapsed build/deploy time",
  },
  "deploys.durationInProgress": {
    message: "Длительность: выполняется",
    description: "Deploy row: elapsed time is still running",
  },
  "deploys.durationActive": {
    message: "Выполняется",
    description:
      "Deploys tab Duration column: bare running-elapsed marker (the column header supplies the label)",
  },
  "deploys.deployedAt": {
    message: "Развёрнут {timestamp}",
    description:
      'Deploy row: how long ago a live/deactivated deploy went live — {timestamp} is elapsed text like "2 hours ago" (its finish time; createdAt fallback)',
  },
  "deploys.createdAt": {
    message: "Создан {timestamp}",
    description:
      'Deploy row: how long ago a deploy that hasn\'t finished (created/queued/in-progress) was created — {timestamp} is elapsed text like "2 minutes ago"',
  },
  "deploys.canceledAt": {
    message: "Отменён {timestamp}",
    description:
      'Deploy row: how long ago a canceled deploy was canceled — {timestamp} is elapsed text like "3 days ago" (its finish time; createdAt fallback)',
  },
  "deploys.failedAt": {
    message: "Не удался {timestamp}",
    description:
      'Deploy row: how long ago a build/pre-deploy/update failure landed — {timestamp} is elapsed text like "3 days ago" (its finish time; createdAt fallback)',
  },
  "deploys.notYet": {
    message: "—",
    description:
      "Deploy header: placeholder for a timestamp that hasn't happened yet",
  },
  "deploys.triggerCreate": {
    message: "Первый деплой",
    description: "Deploy header: trigger=create label",
  },
  "deploys.triggerApi": {
    message: "Деплой через API",
    description: "Deploy header: trigger=api label (an API-key deploy, w4/413)",
  },
  "deploys.triggerManual": {
    message: "Ручной деплой",
    description:
      "Deploy header: trigger=manual label (a dashboard click, w4/413)",
  },
  "deploys.triggerRollback": {
    message: "Откат к {deployId}",
    description:
      "Deploy header: trigger label for a rollback deploy, naming the restored deploy (id must stay literal — never wrap in CSS capitalize)",
  },
  "deploys.triggerDeployHook": {
    message: "Deploy Hook",
    description: "Deploy row: trigger=deploy_hook label (secret-URL trigger)",
  },
  "deploys.triggerBlueprint": {
    message: "Синхронизация Blueprint",
    description: "Deploy row: trigger=blueprint label (Blueprint sync deploy)",
  },
  "deploys.triggerNewCommit": {
    message: "Новый коммит",
    description: "Deploy row: trigger=new_commit label (git push redeploy)",
  },
  "deploys.triggerConfigChange": {
    message: "Изменение конфигурации",
    description:
      "Deploy row: trigger=config_change label (an env var, secret file, or env-group edit that deployed)",
  },
  "deploys.triggerSettingsChange": {
    message: "Изменение настроек",
    description:
      "Deploy row: trigger=settings_change label (a Settings save that deployed, or a plan change; w8/m135)",
  },
  "deploys.triggerRestart": {
    message: "Перезапуск",
    description:
      "Deploy row: trigger=restart label (a Restart roll of the live release, w4/379)",
  },
  "deploys.triggerDeployedByRender": {
    message: "Пересборка платформой",
    description:
      "Deploy row: trigger=deployed_by_render label (the platform rebuilt the live commit, e.g. to ship a runtime security patch)",
  },
  "deploys.logSearchPlaceholder": {
    message: "Поиск по логам…",
    description: "Deploy detail page: log search input placeholder",
  },
  "deploys.logTypeFilter": {
    message: "Тип логов",
    description:
      "Deploy log viewer: aria-label of the log-type filter dropdown (Render's All/Application/Build selector)",
  },
  "deploys.logTypeAll": {
    message: "Все логи",
    description: "Deploy log viewer type filter: show every log type",
  },
  "deploys.logTypeApp": {
    message: "Логи приложения",
    description:
      "Deploy log viewer type filter: app + pre-deploy lines, matching Render's Application bucket",
  },
  "deploys.logTypeBuild": {
    message: "Логи сборки",
    description: "Deploy log viewer type filter: build lines only",
  },
  "deploys.buildLogsStoreUnavailable": {
    message: "История логов сборки недоступна",
    description:
      "Deploy detail page: title shown when the durable log store isn't wired",
  },
  "deploys.buildLogsStoreUnavailableBody": {
    message:
      "Эта панель не подключена к постоянному хранилищу логов. Логи приложения для этого деплоя всё равно могут отображаться.",
    description:
      "Deploy detail page: accurate explanation of a storeless build-log response",
  },
  "deploys.detailRefreshError": {
    message:
      "Не удалось обновить этот деплой. Показаны последние загруженные сведения.",
    description:
      "Deploy detail page: a background refresh of an already loaded deploy failed; the page keeps the last good details and offers a retry",
  },
  "deploys.logPartiallyLoaded": {
    message:
      "Загружена только часть этого периода. Загрузите более ранние записи, чтобы искать по ранним строкам.",
    description:
      "Deploy log viewer banner while older history remains unloaded. The panel's search and type/instance filters run over loaded lines only, so it must not claim a number of matches",
  },
  "deploys.logStepEndMissing": {
    message:
      "Этот деплой был отменён до завершения шага предварительного деплоя, а время завершения шага не записано, поэтому его более поздний вывод может выходить за пределы этого окна. Выберите более широкий период в разделе «Параметры логов», чтобы увидеть его.",
    description:
      "Deploy log viewer: a canceled deploy's continued pre-deploy step has no recorded end, so the deploy window may omit its later output",
  },
  "deploys.logStepEndUnavailable": {
    message:
      "Не удалось определить, когда завершился шаг предварительного деплоя, поэтому более поздний вывод шага может выходить за пределы этого окна. Повторите попытку или выберите более широкий период в разделе «Параметры логов».",
    description:
      "Deploy log viewer: reading a canceled deploy's continued pre-deploy step end failed; offers a retry",
  },
  "deploys.timelineTitle": {
    message: "Хронология статусов",
    description: "Deploy detail page: status-timeline card title",
  },
  "deploys.timelineCreated": {
    message: "Деплой создан",
    description: "Deploy timeline: deploy row was created",
  },
  "deploys.timelineStarted": {
    message: "Деплой запущен",
    description: "Deploy timeline: backend-provided startedAt timestamp",
  },
  "deploys.timelineInProgress": {
    message: "Деплой выполняется",
    description: "Deploy timeline: current non-terminal deploy status",
  },
  "deploys.timelineLive": {
    message: "Деплой работает",
    description: "Deploy timeline: successful terminal state",
  },
  "deploys.timelineFailed": {
    message: "Деплой не удался",
    description: "Deploy timeline: failed terminal state",
  },
  "deploys.timelineCanceled": {
    message: "Деплой отменён",
    description: "Deploy timeline: canceled terminal state",
  },
  "deploys.timelineDeactivated": {
    message: "Деплой деактивирован",
    description: "Deploy timeline: deactivated terminal state",
  },
  "deploys.timelineEventsUnavailable": {
    message:
      "События сервиса недоступны; показаны только временные метки деплоя.",
    description:
      "Deploy timeline: graceful fallback when service-events query fails",
  },
  "deploys.autoDeployOffTitle": {
    message: "Автодеплой отключён",
    description:
      "Deploys page banner title when the service's auto-deploy is off (Render, w3/064)",
  },
  "deploys.autoDeployOffBody": {
    message:
      "Пуши в {branch} не запускают деплой этого сервиса. Разверните его вручную или снова включите автодеплой.",
    description: "Deploys page banner body; {branch} is the tracked git branch",
  },
  "deploys.autoDeployOffBodyNoBranch": {
    message:
      "Пуши не запускают деплой этого сервиса. Разверните его вручную или снова включите автодеплой.",
    description: "Deploys page banner body when the branch is unknown",
  },
  "deploys.autoDeployOffSettings": {
    message: "Настройки автодеплоя",
    description: "Deploys page banner link to the service Settings page",
  },
  "deploys.deploysManualTitle": {
    message: "Деплои запускаются вручную",
    description:
      "Deploys page banner title for a repo whose pushes can't reach bex (pushDeliveryMethod unavailable)",
  },
  "deploys.deploysManualBody": {
    message:
      "bex не получает пуши этого репозитория, поэтому они не запускают деплой этого сервиса. Разверните его вручную или через Deploy Hook либо подключите репозиторий через приложение GitHub.",
    description:
      "Deploys page banner body for a repo whose pushes can't reach bex: no Auto-Deploy setting can turn deploys on push on (Render: a public Git URL must be deployed manually)",
  },
  "deploys.columnDeploy": {
    message: "Деплой",
    description:
      "Deploys tab table: first column header (status, id, commit, timestamp)",
  },
  "deploys.columnTrigger": {
    message: "Триггер",
    description: "Deploys tab table: column header for what triggered a deploy",
  },
  "deploys.columnDuration": {
    message: "Длительность",
    description:
      "Deploys tab table: column header for elapsed build/deploy time",
  },
  "deploys.columnActions": {
    message: "Действия",
    description:
      "Deploys tab table: accessible-only header over the Cancel/Rollback action column",
  },
  "deploys.listEmpty": {
    message: "Деплоев пока нет.",
    description: "Deploys tab: empty state with no status filter",
  },
  "deploys.listEmptyFiltered": {
    message: "Нет деплоев с выбранным статусом.",
    description: "Deploys tab: empty state while a status filter is active",
  },
  "deploys.listStatusFilterLabel": {
    message: "Фильтр по статусу",
    description: "Deploys tab: aria-label of the status filter dropdown",
  },
  "deploys.listStatusAll": {
    message: "Все статусы",
    description: "Deploys tab: status filter option matching every deploy",
  },
  "deploys.listSearchPlaceholder": {
    message: "Поиск по деплоям и коммитам",
    description:
      "Placeholder of the deploy list's search field on every service's Deploys page (Render's text, w3/m555)",
  },
  "deploys.listSearchLabel": {
    message: "Поиск по загруженным деплоям и коммитам",
    description: "Deploys tab: accessible label for deploy-history search",
  },
  "deploys.manualBuild": {
    message: "Ручная сборка",
    description:
      "Button beside a repo-backed cron job's deploy search that builds its latest commit and opens the new build (Render's Manual Build, w3/m555)",
  },
  "deploys.listClearSearch": {
    message: "Очистить поиск",
    description:
      "Deploys tab: button under the no-match message that empties the search box",
  },
  "deploys.listMatchCount": {
    message: "Совпадений: {matched} из {total} загруженных",
    description:
      "Deploys tab: count shown while a search is typed; matched = loaded deploys that match, total = loaded deploys",
  },
  "deploys.listLoadErrorTitle": {
    message: "Не удалось загрузить деплои",
    description:
      "Deploys tab: heading of the error state when the deploy history could not be read; a Try again button follows",
  },
  "deploys.listLoadErrorBody": {
    message:
      "На сами деплои это не влияет. Проверьте подключение и повторите попытку.",
    description:
      "Deploys tab: body of the error state when the deploy history could not be read and the server gave no message; a Try again button follows",
  },
  "deploys.listEmptySearch": {
    message: "Среди загруженных деплоев нет подходящих под ваш запрос.",
    description:
      "Deploys tab: empty state when client-side search has no match",
  },
  "deploys.listCount_one": {
    message: "{count} деплой",
    description: "Deploys tab: complete deploy count (singular)",
  },
  "deploys.listCount_few": {
    message: "{count} деплоя",
    description: "Deploys tab: complete deploy count (plural)",
  },
  "deploys.listCount_many": {
    message: "{count} деплоев",
    description: "Deploys tab: complete deploy count (plural)",
  },
  "deploys.listCount_other": {
    message: "{count} деплоя",
    description: "Deploys tab: complete deploy count (plural)",
  },
  "deploys.listCountLoaded_one": {
    message: "Загружен {count} деплой",
    description:
      "Deploys tab: loaded count when more server pages remain (singular)",
  },
  "deploys.listCountLoaded_few": {
    message: "Загружено {count} деплоя",
    description:
      "Deploys tab: loaded count when more server pages remain (plural)",
  },
  "deploys.listCountLoaded_many": {
    message: "Загружено {count} деплоев",
    description:
      "Deploys tab: loaded count when more server pages remain (plural)",
  },
  "deploys.listCountLoaded_other": {
    message: "Загружено {count} деплоя",
    description:
      "Deploys tab: loaded count when more server pages remain (plural)",
  },
  "deploys.listLoadMore": {
    message: "Загрузить ещё",
    description: "Deploys tab: button fetching the next page of deploy history",
  },
  "deploys.listLoadMoreError": {
    message: "Не удалось загрузить более ранние деплои.",
    description:
      "Deploys tab: toast when reading an older page of deploy history fails",
  },
  "deploys.logRangeLabel": {
    message: "Период времени",
    description:
      "Deploy log viewer options menu: heading over the time-range choices",
  },
  "deploys.logRangeDeploy": {
    message: "Окно деплоя",
    description:
      "Deploy log viewer: the default time range — the deploy's own createdAt..finishedAt window",
  },
  "deploys.logRangeLast15m": {
    message: "Последние 15 минут",
    description: "Deploy log viewer: relative time-range option (?r=15m)",
  },
  "deploys.logRangeLast1h": {
    message: "Последний час",
    description: "Deploy log viewer: relative time-range option (?r=1h)",
  },
  "deploys.logRangeLast6h": {
    message: "Последние 6 часов",
    description: "Deploy log viewer: relative time-range option (?r=6h)",
  },
  "deploys.logRangeLast24h": {
    message: "Последние 24 часа",
    description: "Deploy log viewer: relative time-range option (?r=24h)",
  },
  "deploys.logRangeLast7d": {
    message: "Последние 7 дней",
    description: "Deploy log viewer: relative time-range option (?r=7d)",
  },
  "deploys.logWrap": {
    message: "Переносить строки",
    description:
      "Deploy log viewer options menu: toggle wrapping long log lines vs horizontal scroll",
  },
  "deploys.logTimestamps": {
    message: "Показывать временные метки",
    description:
      "Deploy log viewer options menu: toggle the per-line timestamp column",
  },
  "deploys.reason.http_port_waiting": {
    message: "ожидание запуска контейнера перед определением его HTTP-порта",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_waiting, w4/566)",
  },
  "deploys.reason.http_port_detecting": {
    message: "определение HTTP-порта контейнера",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_detecting, w4/566)",
  },
  "deploys.reason.http_port_limit": {
    message:
      "обнаруженный HTTP-порт превышает лимит сервиса в 75 портов; задайте PORT явно",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_limit, w4/566)",
  },
  "deploys.reason.http_port_detected": {
    message: "Обнаружен HTTP на порту {port}; он используется для трафика",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_detected, w4/566); params are the sentence's values",
  },
  "deploys.reason.http_port_not_answering": {
    message:
      "Порт {port} принял подключение, но ещё не ответил по HTTP; сканирование продолжается, пока запускается контейнер",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_not_answering, w4/566); params are the sentence's values",
  },
  "deploys.reason.http_port_none_yet": {
    message:
      "Ни один порт пока не отвечает по HTTP; сканирование продолжается, пока запускается контейнер",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_none_yet, w4/566)",
  },
  "deploys.reason.http_port_fallback": {
    message:
      "Ни один порт не ответил по HTTP; используется резервный порт {port}. Задайте PORT, если HTTP-слушатель привязан к другому порту",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_fallback, w4/566); params are the sentence's values",
  },
  "deploys.reason.http_port_localhost_only": {
    message:
      "На 0.0.0.0 не обнаружено открытых портов, но на localhost порты открыты ({ports}); сервер должен слушать хост 0.0.0.0. Сканирование продолжается, пока запускается контейнер",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_localhost_only, w8/388); params are the sentence's values: ports is the comma-separated list of ports the process listens on only at localhost",
  },
  "deploys.reason.http_port_none_http": {
    message:
      "На 0.0.0.0 открыты порты ({ports}), но ни один пока не отвечает по HTTP; сканирование продолжается, пока запускается контейнер. Если сервису не нужно принимать публичный HTTP-трафик, создайте вместо него Private Service",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_none_http, w8/388); params are the sentence's values: ports is the comma-separated list of open ports that answer no HTTP",
  },
  "deploys.reason.image_not_found": {
    message:
      "образ {image} не найден в его реестре: репозиторий или тег не существует. Проверьте URL и тег образа или разверните существующий дайджест",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_not_found, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_credential_refused": {
    message:
      "{host} отклонил учётные данные реестра этого сервиса для образа {image} (HTTP {status}). Проверьте имя пользователя и токен либо удалите учётные данные из этого сервиса, чтобы скачать публичный образ анонимно",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_credential_refused, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_access_refused": {
    message:
      "реестр отказал в доступе к образу {image}. Если образ приватный, добавьте для этого сервиса учётные данные реестра для {host}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_access_refused, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_read_refused": {
    message:
      "реестр отказался читать образ {image} (HTTP {status}). Проверьте URL и тег образа",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_read_refused, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_registry_unreachable": {
    message:
      "реестр {host} не отвечал на чтение образа {image} в течение {minutes} минут. Проверьте, что реестр доступен, и выполните деплой снова",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_registry_unreachable, w4/692); params are the sentence's values",
  },
  "deploys.reason.build_timed_out": {
    message: "сборка не завершилась в отведённое время; проверьте логи сборки",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason build_timed_out, w4/566)",
  },
  "deploys.reason.pre_deploy_timed_out": {
    message:
      "команда перед деплоем не завершилась в отведённое время; проверьте логи предварительного деплоя",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_timed_out, w4/566)",
  },
  "deploys.reason.health_timed_out": {
    message:
      "деплой не стал работоспособным в отведённое для проверки время; проверьте логи сервиса",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason health_timed_out, w4/566)",
  },
  "deploys.reason.startup_out_of_memory": {
    message:
      "инстанс исчерпал память при запуске (лимит {memory} для его тарифа вычислений; код выхода {code}) — сократите использование памяти или выберите более крупный тариф вычислений",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason startup_out_of_memory, w4/630); params are the sentence's values: memory is the compute plan's memory limit (e.g. 512Mi), code the exit code",
  },
  "deploys.reason.startup_crash_restarting": {
    message:
      "контейнер завершился вскоре после запуска и многократно перезапускается (последний код выхода {code}) — проверьте логи сервиса, чтобы увидеть вывод сбоя.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason startup_crash_restarting, w8/386); params are the sentence's values: code is the exit code, hint the port advice (a part)",
  },
  "deploys.reason.startup_crash_stopped": {
    message:
      "контейнер завершился вскоре после запуска, и его инстанс остановлен (последний код выхода {code}) — проверьте логи сервиса, чтобы увидеть вывод сбоя.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason startup_crash_stopped, w8/386); params are the sentence's values: code is the exit code, hint the port advice (a part)",
  },
  "deploys.reason.startup_early_exit_restarting": {
    message:
      "контейнер завершился вскоре после запуска и многократно перезапускается (последний код выхода 0): процесс завершился «успешно», но слишком рано. Сервис работает непрерывно и не должен завершаться, пока не получит SIGTERM; для задачи, которая завершается по окончании работы, используйте Cron Job. Проверьте в логах сервиса, почему процесс завершился.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason startup_early_exit_restarting, w8/386); the start command exited with code 0; hint is the port advice (a part)",
  },
  "deploys.reason.startup_early_exit_stopped": {
    message:
      "контейнер завершился вскоре после запуска, и его инстанс остановлен (последний код выхода 0): процесс завершился «успешно», но слишком рано. Сервис работает непрерывно и не должен завершаться, пока не получит SIGTERM; для задачи, которая завершается по окончании работы, используйте Cron Job. Проверьте в логах сервиса, почему процесс завершился.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason startup_early_exit_stopped, w8/386); the start command exited with code 0; hint is the port advice (a part)",
  },
  "deploys.reason.probe_http_waiting": {
    message:
      "контейнер запущен, но его проверка {kind} ещё не пройдена, поэтому выкатка ожидает: GET {path} на порту {port}. Убедитесь, что путь возвращает статус 2xx или 3xx на собственном порту сервиса.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_waiting, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_http_waiting_restarted": {
    message:
      "контейнер запущен, но его проверка {kind} ещё не пройдена, поэтому выкатка ожидает: GET {path} на порту {port}. Проверка liveness уже перезапускала контейнер (раз: {restarts}), так что проверка не проходит, а не просто выполняется медленно. Убедитесь, что путь возвращает статус 2xx или 3xx на собственном порту сервиса.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_waiting_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_http_failed": {
    message:
      "контейнер запущен, но его проверка {kind} так и не была пройдена в отведённое для выкатки время, поэтому деплой не удался: GET {path} на порту {port}. Убедитесь, что путь возвращает статус 2xx или 3xx на собственном порту сервиса.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_failed, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_http_failed_restarted": {
    message:
      "контейнер запущен, но его проверка {kind} так и не была пройдена в отведённое для выкатки время, поэтому деплой не удался: GET {path} на порту {port}. Проверка liveness уже перезапускала контейнер (раз: {restarts}), так что проверка не проходит, а не просто выполняется медленно. Убедитесь, что путь возвращает статус 2xx или 3xx на собственном порту сервиса.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_failed_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_waiting": {
    message:
      "контейнер запущен, но его проверка {kind} ещё не пройдена, поэтому выкатка ожидает: TCP-подключение к порту {port}. Путь проверки работоспособности не задан, поэтому используется TCP-подключение, которое лишь проверяет, слушает ли процесс этот порт.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_waiting, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_waiting_restarted": {
    message:
      "контейнер запущен, но его проверка {kind} ещё не пройдена, поэтому выкатка ожидает: TCP-подключение к порту {port}. Проверка liveness уже перезапускала контейнер (раз: {restarts}), так что проверка не проходит, а не просто выполняется медленно. Путь проверки работоспособности не задан, поэтому используется TCP-подключение, которое лишь проверяет, слушает ли процесс этот порт.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_waiting_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_failed": {
    message:
      "контейнер запущен, но его проверка {kind} так и не была пройдена в отведённое для выкатки время, поэтому деплой не удался: TCP-подключение к порту {port}. Путь проверки работоспособности не задан, поэтому используется TCP-подключение, которое лишь проверяет, слушает ли процесс этот порт.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_failed, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_failed_restarted": {
    message:
      "контейнер запущен, но его проверка {kind} так и не была пройдена в отведённое для выкатки время, поэтому деплой не удался: TCP-подключение к порту {port}. Проверка liveness уже перезапускала контейнер (раз: {restarts}), так что проверка не проходит, а не просто выполняется медленно. Путь проверки работоспособности не задан, поэтому используется TCP-подключение, которое лишь проверяет, слушает ли процесс этот порт.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_failed_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.rollout_deadline_exceeded": {
    message: "выкатка не стала работоспособной в пределах допустимого срока",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason rollout_deadline_exceeded, w4/591)",
  },
  "deploys.reason.pre_deploy_window_exceeded": {
    message:
      "команда перед деплоем не завершилась в отведённое время ({minutes} мин); проверьте логи предварительного деплоя",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_window_exceeded, w4/591); params are the sentence's values",
  },
  "deploys.reason.pre_deploy_out_of_memory": {
    message:
      "команде перед деплоем не хватило памяти (код выхода {code}); проверьте логи предварительного деплоя",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_out_of_memory, w4/591); params are the sentence's values",
  },
  "deploys.reason.pre_deploy_exited": {
    message:
      "команда перед деплоем завершилась с кодом {code}; проверьте логи предварительного деплоя",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_exited, w4/591); params are the sentence's values",
  },
  "deploys.reason.pre_deploy_failed": {
    message:
      "команда перед деплоем завершилась с ошибкой; проверьте логи предварительного деплоя",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_failed, w4/591)",
  },
  "deploys.reason.pre_deploy_could_not_launch": {
    message:
      "команду перед деплоем не удалось запустить: среда выполнения контейнера не смогла запустить её программу; программа не выполнялась, поэтому логов предварительного деплоя нет",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_could_not_launch, w4/591)",
  },
  "deploys.reason.build_never_started": {
    message: "сборка не началась: {cause}",
    description:
      "A deploy failure reason by its code (bex-api DeployReason build_never_started, w4/591); {cause} is the operator's wait, whose sentences are worded as parts (w5/m356)",
  },
  "deploys.reason.build_abandoned": {
    message:
      "сборка не завершилась в отведённое время, а запись App этого сервиса не удалось прочитать, чтобы объяснить причину",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason build_abandoned, w4/591)",
  },
  "deploys.reason.deploy_abandoned": {
    message:
      "деплой не завершился в отведённое время, а запись App этого сервиса не удалось прочитать, чтобы объяснить причину",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason deploy_abandoned, w4/591)",
  },
  "deploys.reason.platform_error": {
    message:
      "bex не смог применить этот релиз из-за ошибки платформы, а не из-за вашего кода или настроек. Обратитесь в поддержку, указав ссылку {ref}",
    description:
      "A deploy failure reason by its code (bex-api DeployReason platform_error, w9/090): the platform refused one of bex's own changes; {ref} is the support reference, shown as-is",
  },
  "deploys.reason.canceled_suspended": {
    message: "Отменён: сервис приостановлен",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_suspended, w4/591)",
  },
  "deploys.reason.superseded_by_newer_release": {
    message: "Заменён более новым релизом",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason superseded_by_newer_release, w4/591)",
  },
  "deploys.reason.superseded_by": {
    message: "Заменён деплоем {deploy}",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason superseded_by, w4/591); params are the sentence's values",
  },
  "deploys.reason.canceled_by_restart": {
    message:
      "Отменён перезапуском {deploy}, который сохранил работающий релиз: то, что применял этот деплой, не было развёрнуто",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_by_restart, w4/591); params are the sentence's values",
  },
  "deploys.reason.canceled_along_with": {
    message: "Отменён вместе с {deploy}",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_along_with, w4/591); params are the sentence's values",
  },
  "deploys.reason.canceled_along_with_newer": {
    message: "Отменён вместе с более новым деплоем",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_along_with_newer, w4/591)",
  },
  "deploys.reason.restart_service_gone": {
    message: "сервис больше не существует",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason restart_service_gone, w4/591)",
  },
  "deploys.reason.restart_wait_timed_out": {
    message: "деплой, которого ожидал этот перезапуск, не завершился за {wait}",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason restart_wait_timed_out, w4/591); params are the sentence's values",
  },
  "deploys.reason.deploy_wait_timed_out": {
    message: "деплой, которого ожидал этот, не завершился за {wait}",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason deploy_wait_timed_out, w4/652): a Rollback or another deploy that waited for the deploy in flight; params are the sentence's values",
  },
  "deploys.reason.probe_other_waiting": {
    message:
      "контейнер запущен, но его проверка {kind} ещё не пройдена, поэтому выкатка ожидает: настроенная проверка работоспособности. Убедитесь, что путь возвращает статус 2xx или 3xx на собственном порту сервиса.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_waiting, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.probe_other_waiting_restarted": {
    message:
      "контейнер запущен, но его проверка {kind} ещё не пройдена, поэтому выкатка ожидает: настроенная проверка работоспособности. Проверка liveness уже перезапускала контейнер (раз: {restarts}), так что проверка не проходит, а не просто выполняется медленно. Убедитесь, что путь возвращает статус 2xx или 3xx на собственном порту сервиса.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_waiting_restarted, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.probe_other_failed": {
    message:
      "контейнер запущен, но его проверка {kind} так и не была пройдена в отведённое для выкатки время, поэтому деплой не удался: настроенная проверка работоспособности. Убедитесь, что путь возвращает статус 2xx или 3xx на собственном порту сервиса.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_failed, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.probe_other_failed_restarted": {
    message:
      "контейнер запущен, но его проверка {kind} так и не была пройдена в отведённое для выкатки время, поэтому деплой не удался: настроенная проверка работоспособности. Проверка liveness уже перезапускала контейнер (раз: {restarts}), так что проверка не проходит, а не просто выполняется медленно. Убедитесь, что путь возвращает статус 2xx или 3xx на собственном порту сервиса.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_failed_restarted, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.probe_open_port_waiting": {
    message:
      "контейнер запущен, но его проверка {kind} ещё не пройдена, поэтому выкатка ожидает: TCP-подключение к любому открытому порту экземпляра. Путь проверки работоспособности не задан, поэтому используется TCP-подключение, которое лишь проверяет, слушает ли процесс хотя бы один порт.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_open_port_waiting, w8/384): the stall of Render's default check, a TCP connect to any open port, which a first web image release with no Health Check Path runs; params are the sentence's values",
  },
  "deploys.reason.probe_open_port_waiting_restarted": {
    message:
      "контейнер запущен, но его проверка {kind} ещё не пройдена, поэтому выкатка ожидает: TCP-подключение к любому открытому порту экземпляра. Проверка liveness уже перезапускала контейнер (раз: {restarts}), так что проверка не проходит, а не просто выполняется медленно. Путь проверки работоспособности не задан, поэтому используется TCP-подключение, которое лишь проверяет, слушает ли процесс хотя бы один порт.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_open_port_waiting_restarted, w8/384): the stall of Render's default check, a TCP connect to any open port, which a first web image release with no Health Check Path runs; params are the sentence's values",
  },
  "deploys.reason.probe_open_port_failed": {
    message:
      "контейнер запущен, но его проверка {kind} так и не была пройдена в отведённое для выкатки время, поэтому деплой не удался: TCP-подключение к любому открытому порту экземпляра. Путь проверки работоспособности не задан, поэтому используется TCP-подключение, которое лишь проверяет, слушает ли процесс хотя бы один порт.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_open_port_failed, w8/384): the stall of Render's default check, a TCP connect to any open port, which a first web image release with no Health Check Path runs; params are the sentence's values",
  },
  "deploys.reason.probe_open_port_failed_restarted": {
    message:
      "контейнер запущен, но его проверка {kind} так и не была пройдена в отведённое для выкатки время, поэтому деплой не удался: TCP-подключение к любому открытому порту экземпляра. Проверка liveness уже перезапускала контейнер (раз: {restarts}), так что проверка не проходит, а не просто выполняется медленно. Путь проверки работоспособности не задан, поэтому используется TCP-подключение, которое лишь проверяет, слушает ли процесс хотя бы один порт.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_open_port_failed_restarted, w8/384): the stall of Render's default check, a TCP connect to any open port, which a first web image release with no Health Check Path runs; params are the sentence's values",
  },
  "deploys.reason.hint_image_default_port": {
    message:
      " Ничто не приняло TCP-подключение на порту {port}, который выбран по умолчанию, так как образ не объявляет TCP-портов. Сервер должен слушать хост 0.0.0.0 на порту из PORT ({port}).",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_image_default_port, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_image_declared_ports": {
    message:
      " Ничто не приняло TCP-подключение на порту {port}, основном порту, выбранном из объявленных образом TCP-портов ({ports}). Привяжите сервер к 0.0.0.0 на одном из этих портов или укажите в сервисе порт, который он слушает.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_image_declared_ports, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_env_port_only": {
    message:
      " Ничто не приняло TCP-подключение на порту {port}, заданном переменной окружения PORT сервиса; образ не объявляет других TCP-портов. Привяжите сервер к 0.0.0.0 на порту {port} или измените порт сервиса на тот, который он слушает.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_env_port_only, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_env_port_closed": {
    message:
      " Ничто не приняло TCP-подключение на порту {port}, заданном переменной окружения PORT сервиса. Привяжите сервер к 0.0.0.0 на порту {port} или измените переменную окружения PORT на порт, который сервис слушает.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_env_port_closed, w8/416): a web service's PORT names the one port Render scans; en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_configured_port_only": {
    message:
      " Ничто не приняло TCP-подключение на порту {port}, заданном в настройках порта сервиса; образ не объявляет других TCP-портов. Привяжите сервер к 0.0.0.0 на порту {port} или измените порт сервиса на тот, который он слушает.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_configured_port_only, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_env_port_declared": {
    message:
      " Ничто не приняло TCP-подключение на порту {port}, заданном переменной окружения PORT сервиса; образ объявляет TCP-порты ({ports}). Измените переменную окружения PORT на порт, который сервис слушает, или удалите её, чтобы использовать порт образа.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_env_port_declared, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_configured_port_declared": {
    message:
      " Ничто не приняло TCP-подключение на порту {port}, заданном в настройках порта сервиса; образ объявляет TCP-порты ({ports}). Измените порт сервиса в настройках на тот, который он слушает, или удалите его, чтобы использовать порт образа.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_configured_port_declared, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_background_worker": {
    message:
      " Если сервису не нужно принимать трафик ни на каком порту, создайте вместо него Background Worker.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_background_worker, w5/m356); en starts with the space that joins it to the sentence before",
  },
  "deploys.reason.hint_no_http_discovered": {
    message:
      " Ни один порт не ответил по HTTP при обнаружении во время запуска.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_http_discovered, w5/m356); en starts with the space that joins it to the sentence before",
  },
  "deploys.reason.hint_listening": {
    message: " Процесс слушает порт {port}.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_listening_adopted": {
    message: " Процесс слушает порт {port}, который теперь использует сервис.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening_adopted, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_listening_ports": {
    message:
      " Процесс слушает порты {ports}, а не порт {port}; задайте порт сервиса (или переменную PORT) равным тому, который обслуживает HTTP.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening_ports, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_listening_primary": {
    message:
      " Процесс слушает порт {port}, который теперь использует сервис как основной.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening_primary, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_no_open_port": {
    message:
      " На инстансе не обнаружено открытых портов: сервер должен слушать хост 0.0.0.0 на порту из PORT ({port}).",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_port, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_no_open_port_before": {
    message:
      " На инстансе не обнаружено открытых портов: сервер должен слушать хост 0.0.0.0 на порту из PORT ({port}); ранее он слушал порт {previous}.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_port_before, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_no_open_service_port": {
    message:
      " На инстансе не обнаружено открытых портов: сервер должен слушать хост 0.0.0.0 на порту сервиса ({port}).",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_service_port, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_no_open_port_localhost": {
    message:
      " На 0.0.0.0 не обнаружено открытых портов, но на localhost порты открыты ({ports}): сервер должен слушать хост 0.0.0.0 на порту из PORT ({port}).",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_port_localhost, w8/388); en starts with the space that joins it to the sentence before; params are the sentence's values: ports is the comma-separated list of ports the process listens on only at localhost",
  },
  "deploys.reason.hint_no_open_http_port": {
    message:
      " На 0.0.0.0 открыты порты ({ports}), но ни один не отвечает по HTTP: сервер должен отдавать HTTP на порту из PORT ({port}). Если сервису не нужно принимать публичный HTTP-трафик, создайте вместо него Private Service.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_http_port, w8/388); en starts with the space that joins it to the sentence before; params are the sentence's values: ports is the comma-separated list of open ports that answer no HTTP",
  },
  "deploys.reason.hint_crash_port_bind": {
    message:
      " Если сбой связан с привязкой порта: процесс должен слушать $PORT ({port}).",
    description:
      "A startup crash's port advice, by its code (bex-api DeployReason part hint_crash_port_bind, w8/386); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_early_exit_port": {
    message: " Сервер должен слушать $PORT ({port}), пока не получит SIGTERM.",
    description:
      "A startup crash's port advice, by its code (bex-api DeployReason part hint_early_exit_port, w8/386); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.queued_build_limit": {
    message:
      "{noun}: активно {active} из {limit} одновременных сборок; ожидание свободного слота",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_build_limit, w5/m356): a full concurrent-build cap; {noun} is the cap's scope, worded by deploys.reasonParam.noun.*",
  },
  "deploys.reason.queued_cluster_full": {
    message: "ёмкость кластера для сборок исчерпана; ожидание свободного слота",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_cluster_full, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_slot_taken": {
    message:
      "параллельный запуск занял последний слот сборки; ожидание свободного слота",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_slot_taken, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_database": {
    message:
      "Ожидание, пока связанная база данных {database} станет доступной, перед сборкой",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_database, w5/m356): what the build waited on; params are the sentence's values",
  },
  "deploys.reason.queued_datastore": {
    message:
      "Ожидание данных подключения связанного хранилища данных перед сборкой",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_datastore, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_build_credential": {
    message:
      "Ожидание, пока реестр примет учётные данные сборки этого приложения",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_build_credential, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_pull_credential": {
    message:
      "Ожидание, пока реестр примет учётные данные для скачивания образа этого приложения",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_pull_credential, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_capacity": {
    message: "ожидание ресурсов для сборки: {detail}",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity, w5/m356): waiting for build capacity; {detail} is the build plane's own wait, shown as-is",
  },
  "deploys.reason.queued_capacity_pod": {
    message:
      "ожидание ресурсов для сборки: задание сборки отправлено; ожидание создания пода сборки",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity_pod, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_capacity_no_node": {
    message:
      "ожидание ресурсов для сборки: пока нет узла сборки со свободными ресурсами",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity_no_node, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_capacity_short": {
    message:
      "ожидание ресурсов для сборки: пока нет узла сборки со свободными ресурсами (не хватает свободных ресурсов: {resources})",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity_short, w5/m356): what the build waited on; params are the sentence's values",
  },
  "deploys.reasonParam.kind.startup": {
    message: "запуска",
    description:
      "A deploy reason's {kind} value startup (w4/591): which health check a stalled rollout is waiting on",
  },
  "deploys.reasonParam.kind.readiness": {
    message: "готовности",
    description:
      "A deploy reason's {kind} value readiness (w4/591): which health check a stalled rollout is waiting on",
  },
  "deploys.reasonParam.noun.workspace": {
    message: "рабочее пространство",
    description:
      "A deploy reason's {noun} value workspace (w5/m356): the scope of a full concurrent-build cap",
  },
  "deploys.reasonWait.duration": {
    message: "{units}",
    description:
      "A deferred restart's wait, restart_wait_timed_out's {wait} (w5/m356): {wait} is bex-api's Go duration (3h0m0s), which en keeps so the sentence reads as the API's; {units} is the duration in words (deploys.reasonWait.hours and the rest), which other languages read",
  },
  "deploys.reasonWait.hours": {
    message: "{hours} ч",
    description: "A deferred restart's wait in words (w5/m356): its hours",
  },
  "deploys.reasonWait.minutes": {
    message: "{minutes} мин",
    description: "A deferred restart's wait in words (w5/m356): its minutes",
  },
  "deploys.reasonWait.seconds": {
    message: "{seconds} с",
    description: "A deferred restart's wait in words (w5/m356): its seconds",
  },
  "deploys.factBuild": {
    message: "Сборка",
    description:
      "Deploy page: label of the fact naming the Build Source build ID (bld-…) this deploy ran",
  },
  "deploys.factBuildSource": {
    message: "Источник сборки",
    description:
      "Deploy page: label of the fact naming the linked Build Source (name and bds-… ID)",
  },
  "deploys.buildIdLabel": {
    message: "ID сборки",
    description: "Deploy page: accessible field name for copying the Build ID",
  },
  "deploys.buildIdCopied": {
    message: "ID сборки скопирован",
    description: "Deploy page: toast shown after copying the Build ID",
  },
  "deploys.buildIdCopyFailed": {
    message: "Не удалось скопировать ID сборки",
    description: "Deploy page: toast shown when copying the Build ID fails",
  },
  "deploys.buildSourceIdLabel": {
    message: "ID источника сборки",
    description:
      "Deploy page: accessible field name for copying the Build Source ID",
  },
  "deploys.buildSourceIdCopied": {
    message: "ID источника сборки скопирован",
    description: "Deploy page: toast shown after copying the Build Source ID",
  },
  "deploys.buildSourceIdCopyFailed": {
    message: "Не удалось скопировать ID источника сборки",
    description:
      "Deploy page: toast shown when copying the Build Source ID fails",
  },
};

export default ruDeploys;
