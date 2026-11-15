import type { TranslationEntry } from "@/i18n";

const ruMetrics: Record<string, TranslationEntry> = {
  "metrics.cpuCores": {
    message: "{value} ядер",
    description:
      "A CPU amount in cores (a limit or the latest usage); value is a decimal such as 0.100 (w4/566)",
  },
  "metrics.eventsTitle": {
    message: "Лента событий",
    description:
      "Metrics-page timeline of service events in the selected range",
  },
  "metrics.eventsFilterLabel": {
    message: "Фильтр событий",
    description: "Accessible label for the Metrics event category filter",
  },
  "metrics.eventsFilterAll": {
    message: "Все события",
    description: "All event categories",
  },
  "metrics.eventsFilterDeploys": {
    message: "Деплои",
    description: "Deploy events category",
  },
  "metrics.eventsFilterLifecycle": {
    message: "Жизненный цикл",
    description: "Lifecycle events category",
  },
  "metrics.eventsFilterConfig": {
    message: "Конфигурация",
    description: "Configuration events category",
  },
  "metrics.eventsLoading": {
    message: "Загрузка событий…",
    description: "Event timeline loading state",
  },
  "metrics.eventsError": {
    message: "Не удалось загрузить события.",
    description: "Event timeline error state",
  },
  "metrics.eventsEmptyTitle": {
    message: "Нет событий",
    description: "Event timeline empty-state title",
  },
  "metrics.eventsEmptyBody": {
    message: "За этот период ничего не происходило.",
    description: "Event timeline empty-state body",
  },
  "metrics.eventsChanged": {
    message: "Сервис изменён",
    description: "Fallback event timeline label",
  },
  "metrics.eventMarkersLabel": {
    message: "События сервиса",
    description: "Accessible label for the event-marker strip above a chart",
  },
  "metrics.eventMarkerCluster_one": {
    message: "{count} событие",
    description:
      "Accessible label for a clustered chart event marker (count badge, singular)",
  },
  "metrics.eventMarkerCluster_few": {
    message: "{count} события",
    description:
      "Accessible label for a clustered chart event marker (count badge, plural)",
  },
  "metrics.eventMarkerCluster_many": {
    message: "{count} событий",
    description:
      "Accessible label for a clustered chart event marker (count badge, plural)",
  },
  "metrics.eventMarkerCluster_other": {
    message: "{count} события",
    description:
      "Accessible label for a clustered chart event marker (count badge, plural)",
  },
  "metrics.lineChartLabel_one": {
    message: "Линейный график с {count} точкой данных",
    description:
      "Accessible name of a metrics line chart; count is the number of distinct sample times (singular)",
  },
  "metrics.lineChartLabel_few": {
    message: "Линейный график с {count} точками данных",
    description:
      "Accessible name of a metrics line chart; count is the number of distinct sample times (plural)",
  },
  "metrics.lineChartLabel_many": {
    message: "Линейный график с {count} точками данных",
    description:
      "Accessible name of a metrics line chart; count is the number of distinct sample times (plural)",
  },
  "metrics.lineChartLabel_other": {
    message: "Линейный график с {count} точки данных",
    description:
      "Accessible name of a metrics line chart; count is the number of distinct sample times (plural)",
  },
  "metrics.barChartLabel_one": {
    message: "Столбчатая диаграмма с {count} точкой данных",
    description:
      "Accessible name of a metrics bar chart; count is the number of time buckets (singular)",
  },
  "metrics.barChartLabel_few": {
    message: "Столбчатая диаграмма с {count} точками данных",
    description:
      "Accessible name of a metrics bar chart; count is the number of time buckets (plural)",
  },
  "metrics.barChartLabel_many": {
    message: "Столбчатая диаграмма с {count} точками данных",
    description:
      "Accessible name of a metrics bar chart; count is the number of time buckets (plural)",
  },
  "metrics.barChartLabel_other": {
    message: "Столбчатая диаграмма с {count} точки данных",
    description:
      "Accessible name of a metrics bar chart; count is the number of time buckets (plural)",
  },
  "metrics.barValueAt": {
    message: "{value} в {time}",
    description:
      "Accessible name of one focusable bar: its formatted value (e.g. 42 or 1.2K) and its local sample time",
  },
  "metrics.chartEmpty": {
    message: "Нет данных за период",
    description:
      "Default empty state of a metrics chart when the settled query returned no samples in the selected time range",
  },
  "metrics.eventMarkerHint": {
    message: "Нажмите, чтобы посмотреть подробности",
    description: "Footer line of the chart event-marker hover tooltip",
  },
  "metrics.eventDeployStarted": {
    message: "Деплой начался",
    description: "Chart event-marker label: a deploy began",
  },
  "metrics.eventDeployLiveFor": {
    message: "Деплой запущен для {commit}",
    description:
      "Chart event-marker label: a deploy went live, with its commit short id",
  },
  "metrics.eventDeployLive": {
    message: "Деплой запущен",
    description: "Chart event-marker label: a deploy went live (no commit id)",
  },
  "metrics.eventDeployFailed": {
    message: "Деплой завершился с ошибкой",
    description: "Chart event-marker label: a deploy failed",
  },
  "metrics.eventDeployCanceled": {
    message: "Деплой отменён",
    description: "Chart event-marker label: a deploy was canceled",
  },
  "metrics.eventDeployEnded": {
    message: "Деплой завершён",
    description:
      "Chart event-marker label: a deploy finished with an unrecognized status",
  },
  "metrics.eventBuildStarted": {
    message: "Сборка началась",
    description: "Chart event-marker label: an image build began",
  },
  "metrics.eventBuildEnded": {
    message: "Сборка завершена",
    description: "Chart event-marker label: an image build finished",
  },
  "metrics.eventPreDeployStarted": {
    message: "Pre-deploy начался",
    description: "Chart event-marker label: the pre-deploy command began",
  },
  "metrics.eventPreDeployEnded": {
    message: "Pre-deploy завершён",
    description: "Chart event-marker label: the pre-deploy command finished",
  },
  "metrics.eventJobRunEnded": {
    message: "Задание завершено",
    description: "Chart event-marker label: a one-off job finished",
  },
  "metrics.eventBranchDeleted": {
    message: "Ветка удалена",
    description: "Chart event-marker label: the tracked Git branch was deleted",
  },
  "metrics.eventServerRestarted": {
    message: "Сервер перезапущен",
    description: "Chart event-marker label: the service was restarted",
  },
  "metrics.eventSuspended": {
    message: "Сервис приостановлен",
    description: "Chart event-marker label: the service was suspended",
  },
  "metrics.eventResumed": {
    message: "Сервис возобновлён",
    description: "Chart event-marker label: the service was resumed",
  },
  "metrics.eventInstanceCountChanged": {
    message: "Количество инстансов изменено",
    description: "Chart event-marker label: manual scaling changed the count",
  },
  "metrics.eventAutoscalingChanged": {
    message: "Конфигурация автомасштабирования изменена",
    description: "Chart event-marker label: autoscaling settings changed",
  },
  "metrics.eventPlanChanged": {
    message: "Тип инстанса изменён",
    description: "Chart event-marker label: the service's plan changed",
  },
  "metrics.networkTitle": {
    message: "Сетевые метрики",
    description: "Network metrics card title",
  },
  "metrics.networkDescription": {
    message: "Суммарно по всем инстансам",
    description: "Network metrics card description",
  },
  "metrics.totalRequests": {
    message: "Всего запросов",
    description: "Network metrics chart section title",
  },
  "metrics.responseTimes": {
    message: "Время ответа",
    description: "Network metrics chart section title",
  },
  "metrics.percentile": {
    message: "Перцентиль",
    description: "Response Times section control label",
  },
  "metrics.percentileAll": {
    message: "Все",
    description:
      "Percentile dropdown option that overlays p50/p90/p99 on one chart",
  },
  "metrics.requestsCount_one": {
    message: "{formatted} запрос",
    description:
      "Aggregate request count beside the Total Requests section title (singular; {formatted} is the locale-formatted count)",
  },
  "metrics.requestsCount_few": {
    message: "{formatted} запроса",
    description:
      "Aggregate request count beside the Total Requests section title (plural; {formatted} is the locale-formatted count)",
  },
  "metrics.requestsCount_many": {
    message: "{formatted} запросов",
    description:
      "Aggregate request count beside the Total Requests section title (plural; {formatted} is the locale-formatted count)",
  },
  "metrics.requestsCount_other": {
    message: "{formatted} запроса",
    description:
      "Aggregate request count beside the Total Requests section title (plural; {formatted} is the locale-formatted count)",
  },
  "metrics.outboundBandwidth": {
    message: "Исходящий трафик",
    description: "Network metrics chart section title",
  },
  "metrics.monthToDateBandwidth": {
    message: "Использовано в этом месяце: {amount}",
    description:
      "Footer under the Outbound Bandwidth chart, showing month-to-date egress",
  },
  "metrics.monthToDateBandwidthError": {
    message:
      "Не удалось загрузить трафик за этот месяц — попробуйте обновить страницу",
    description:
      "Month-to-date bandwidth footer when the query failed with no value — distinct from a known zero and from the chart's bandwidthError (w4/090)",
  },
  "metrics.bandwidthDegraded": {
    message: "Неполные данные",
    description:
      "Badge on the Outbound Bandwidth chart when an egress source's health check failed inside the window (w1/m50)",
  },
  "metrics.bandwidthDegradedDetail": {
    message:
      "Источник исходящего трафика {sources} был недоступен в части этого периода — трафик мог быть занижен.",
    description:
      "Tooltip expanding the Partial data badge; {sources} is the comma-joined raw source tokens (http/websocket/direct)",
  },
  "metrics.bandwidthError": {
    message: "Не удалось загрузить трафик — попробуйте обновить страницу",
    description:
      "Bandwidth chart body when the query itself failed (network/server error, not an empty window)",
  },
  "metrics.sourceNotConfigured": {
    message: "Источник метрик не настроен",
    description: "Shown when bex-api reports no metrics backend is wired up",
  },
  "metrics.chartError": {
    message: "Не удалось загрузить эту метрику",
    description:
      "Shown when a metrics query fails with a real error (not a 503/no-data) — distinct from the empty 'No data in range' state (w9/m86)",
  },
  "metrics.chartThrottled": {
    message: "Обновляем данные — трафик был ненадолго ограничен",
    description:
      "Cold-load state when a metrics read was shed (RATE_LIMITED / AUTH_OVERLOADED) with no cached series — transient, not 'metrics unavailable' (w4/m100)",
  },
  "metrics.chartStaleThrottled": {
    message:
      "Показаны кешированные данные — обновление после краткого ограничения",
    description:
      "Warm-cache banner when a metrics refetch was throttled; cached series remain visible but must not read as current (w4/m100)",
  },
  "metrics.applicationTitle": {
    message: "Метрики приложения",
    description: "Application metrics card title",
  },
  "metrics.limitLink": {
    message: "Лимит",
    description:
      "Link to the Instance Type tab on the Memory/CPU chart headers, followed by the limit value when configured",
  },
  "metrics.manageScaling": {
    message: "Управление масштабированием",
    description: "Link to the Scaling tab on the Total Instances chart header",
  },
  "metrics.noLimitConfigured": {
    message: "Лимит не задан — процент не определён",
    description:
      "Shown instead of a percentage chart when the App's pods declare no resource limit",
  },
  "metrics.limitsVary": {
    message: "Лимиты различаются",
    description:
      "Memory/CPU chart header annotation when the selected replicas declare different limits — each replica is percentized against its own limit, so no single limit value applies",
  },
  "metrics.percentageUnavailable": {
    message: "Проценты недоступны — для этого периода нет надёжного лимита",
    description:
      "Shown instead of a percentage chart when usage samples exist but every denominator was missing, zero, or otherwise untrustworthy (deleted pods, predated limit history, a mid-window rollout gap) — distinct from no usage samples or a source failure",
  },
  "metrics.targetLabel": {
    message: "Цель {value}",
    description:
      "Autoscale-target annotation on the Memory/CPU chart headers (percentage mode only), e.g. 'Target 70%'",
  },
  "metrics.diskTitle": {
    message: "Диск",
    description: "Datastore metrics panel: disk usage chart section title",
  },
  "metrics.diskCapacityLabel": {
    message: "Объём {value}",
    description:
      "Disk usage chart header annotation showing the datastore's billed/provisioned disk size",
  },
  "metrics.connectionsTitle": {
    message: "Активные подключения",
    description:
      "Datastore metrics panel: Postgres active-connections chart section title",
  },
  "metrics.memoryTitle": {
    message: "Память",
    description:
      "Datastore metrics panel: memory chart section title (Postgres container memory; Key Value used memory)",
  },
  "metrics.datastoreCpuTitle": {
    message: "CPU",
    description: "Datastore metrics panel: CPU usage chart section title",
  },
  "metrics.datastoreLimitLabel": {
    message: "Лимит {value}",
    description:
      "Datastore CPU/memory chart header annotation showing the instance's configured limit",
  },
  "metrics.kvConnectionsTitle": {
    message: "Подключения",
    description:
      "Datastore metrics panel: Key Value connected-clients chart section title",
  },
  "metrics.replicationLagTitle": {
    message: "Задержка репликации",
    description:
      "Datastore metrics panel: Postgres replication-lag chart section title",
  },
  "metrics.replicationLagPendingHA": {
    message:
      "Н/д — нет реплики (включите высокую доступность, чтобы видеть задержку репликации)",
    description:
      "Shown instead of a chart for replication lag before Postgres HA is enabled (w1/m22)",
  },
  "metrics.datastoreMetricsTitle": {
    message: "Метрики",
    description: "Database/Key Value detail page metrics panel card title",
  },
  "metrics.datastoreMetricsDescription": {
    message: "Текущее использование ресурсов этим инстансом",
    description:
      "Database/Key Value detail page metrics panel card description",
  },
  "metrics.statusCode": {
    message: "Код статуса",
    description: "Toolbar filter label for HTTP status code",
  },
  "metrics.statusCodeAll": {
    message: "Все",
    description: "Status-code filter option meaning no filtering",
  },
  "metrics.host": {
    message: "Хост",
    description: "Network-card filter label for the request Host",
  },
  "metrics.hostAll": {
    message: "Все",
    description: "Host filter option meaning no filtering",
  },
  "metrics.path": {
    message: "Путь",
    description: "Network-card filter label for the request Path",
  },
  "metrics.pathPlaceholder": {
    message: "Путь…",
    description: "Placeholder for the free-text Path filter input",
  },
  "metrics.pathClear": {
    message: "Сбросить фильтр по пути",
    description: "Accessible label for the button that clears the Path filter",
  },
  "metrics.hostPathStoreUnavailable": {
    message:
      "Фильтры по хосту и пути и разбивка по хостам требуют хранилища логов",
    description:
      "Per-section state when a host/path-filtered or host-grouped request metric hits a deployment with no durable log store (503) — served from the request-log store (Loki), unavailable here",
  },
  "metrics.groupBy": {
    message: "Группировать по",
    description: "Total Requests chart control label",
  },
  "metrics.groupByAllRequests": {
    message: "Все запросы",
    description: "Group-by option: one aggregate series",
  },
  "metrics.groupByStatus": {
    message: "Код статуса",
    description: "Group-by option: one series per HTTP status code",
  },
  "metrics.groupByMethod": {
    message: "Метод",
    description: "Group-by option: one series per HTTP method",
  },
  "metrics.groupByHost": {
    message: "Хост",
    description: "Group-by option: one series per request host",
  },
  "metrics.memory": {
    message: "Память",
    description: "Application metrics stat tile label",
  },
  "metrics.cpu": {
    message: "CPU",
    description: "Application metrics stat tile label",
  },
  "metrics.totalInstances": {
    message: "Всего инстансов",
    description: "Application metrics stat tile label",
  },
  "metrics.filterPercentage": {
    message: "Проценты",
    description: "Metrics filter tab: show values as a percentage",
  },
  "metrics.filterTotal": {
    message: "Всего",
    description: "Metrics filter tab: show values as an absolute total",
  },
  "metrics.instanceFilter": {
    message: "Инстансы",
    description: "Application metrics instance multi-select label",
  },
  "metrics.instanceShowAll": {
    message: "Показать все",
    description:
      "Explicit action that clears the retained instance selection back to all instances (w5/m91)",
  },
  "metrics.instanceUnavailableSuffix": {
    message: " (недоступен)",
    description:
      "Suffix appended to a retained instance option that is no longer in the window or discovery (w5/m91)",
  },
  "metrics.instancesUnavailable_one": {
    message: "{count} выбранный инстанс недоступен в этом периоде",
    description:
      "Note when one retained instance selection has no samples in the window (singular; w5/m91)",
  },
  "metrics.instancesUnavailable_few": {
    message: "{count} выбранных инстанса недоступны в этом периоде",
    description:
      "Note when retained instance selections have no samples in the window (plural; w5/m91)",
  },
  "metrics.instancesUnavailable_many": {
    message: "{count} выбранных инстансов недоступны в этом периоде",
    description:
      "Note when retained instance selections have no samples in the window (plural; w5/m91)",
  },
  "metrics.instancesUnavailable_other": {
    message: "{count} выбранных инстанса недоступны в этом периоде",
    description:
      "Note when retained instance selections have no samples in the window (plural; w5/m91)",
  },
  "metrics.noDataForSelection": {
    message: "Нет данных по выбранным инстансам за этот период",
    description:
      "Empty-result copy when an explicit instance filter yields no samples — distinct from query errors (w5/m91)",
  },
  "metrics.aggregateFilter": {
    message: "Агрегация",
    description: "Application metrics replica aggregation control",
  },
  "metrics.aggregateRaw": {
    message: "Без агрегации",
    description: "Show one series per selected instance",
  },
  "metrics.aggregateMin": {
    message: "Мин.",
    description: "Minimum across selected replicas at each timestamp",
  },
  "metrics.aggregateMax": {
    message: "Макс.",
    description: "Maximum across selected replicas at each timestamp",
  },
  "metrics.aggregateAvg": {
    message: "Сред.",
    description: "Average across selected replicas at each timestamp",
  },
  "metrics.rangeLast30Minutes": {
    message: "Последние 30 минут",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLastHour": {
    message: "Последний час",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast4Hours": {
    message: "Последние 4 часа",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast12Hours": {
    message: "Последние 12 часов",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast24Hours": {
    message: "Последние 24 часа",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast2Days": {
    message: "Последние 2 дня",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast7Days": {
    message: "Последние 7 дней",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast14Days": {
    message: "Последние 14 дней",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast30Days": {
    message: "Последние 30 дней",
    description:
      "Metrics time-range filter option (w5/m56; Render plan-gates it)",
  },
  "metrics.rangeCustom": {
    message: "Свой период…",
    description: "Time-range dropdown option that opens the start/end picker",
  },
  "metrics.rangeCustomEdit": {
    message: "Изменить свой период",
    description:
      "Button beside the time-range dropdown that reopens the picker for the active custom range",
  },
  "metrics.rangeCustomTitle": {
    message: "Свой период",
    description: "Custom time-range picker dialog title",
  },
  "metrics.rangeCustomDescription": {
    message:
      "Укажите начало и конец периода. Периоды длиннее 30 дней недоступны.",
    description: "Custom time-range picker dialog description",
  },
  "metrics.rangeCustomStart": {
    message: "Начало",
    description: "Custom range start datetime field label",
  },
  "metrics.rangeCustomEnd": {
    message: "Конец",
    description: "Custom range end datetime field label",
  },
  "metrics.rangeCustomApply": {
    message: "Применить",
    description: "Custom range apply button",
  },
  "metrics.rangeCustomCancel": {
    message: "Отменить",
    description: "Custom range cancel button",
  },
  "metrics.rangeCustomErrorOrder": {
    message: "Конец должен быть позже начала.",
    description: "Custom range validation: end not after start",
  },
  "metrics.rangeCustomErrorTooLong": {
    message: "Выберите период не длиннее 30 дней.",
    description: "Custom range validation: window exceeds the max query window",
  },
  "metrics.rangeCustomErrorTooEarly": {
    message: "Диапазон должен начинаться не раньше {date}.",
    description:
      "Custom range validation: the start is before the oldest instant the range may reach (e.g. a log plan's retention)",
  },
  "metrics.rangeLabel": {
    message: "Период",
    description: "Accessible label for the time-range dropdown",
  },
  "metrics.eventsToggle": {
    message: "Показать ленту событий",
    description: "Accessible label for the Metrics event-timeline toggle",
  },
  "metrics.latencyPlanRequired": {
    message:
      "Метрики времени ответа доступны на тарифе Pro и выше. Это рабочее пространство использует тариф Hobby.",
    description:
      "Response Times section on a Hobby workspace (PLAN_FEATURE_REQUIRED, w8/m138)",
  },
};

export default ruMetrics;
