import type { TranslationEntry } from "@/i18n";

const ruWebhooks: Record<string, TranslationEntry> = {
  "webhooks.title": {
    message: "Вебхуки",
    description: "Settings Integrations Webhooks section card title",
  },
  "webhooks.description": {
    message:
      "Отправляйте подписанные уведомления о событиях (деплои, приостановки, перезапуски, масштабирование) на ваши эндпоинты — без опроса. Секрет подписи каждого вебхука находится в его настройках.",
    description: "Settings Webhooks section card description",
  },
  "webhooks.colEndpoint": {
    message: "Эндпоинт",
    description: "Webhooks table column header — name + destination URL",
  },
  "webhooks.colEvents": {
    message: "События",
    description: "Webhooks table column header — subscribed event types",
  },
  "webhooks.colEnabled": {
    message: "Включён",
    description: "Webhooks table column header — the enable/disable switch",
  },
  "webhooks.colLatest": {
    message: "Последняя доставка",
    description: "Webhooks table column header — newest immutable attempt",
  },
  "webhooks.searchEndpoints": {
    message: "Поиск вебхуков",
    description: "Endpoint list search label and placeholder",
  },
  "webhooks.searchEndpointsHelp": {
    message: "Поиск по названию, адресу назначения или событию",
    description: "Endpoint list search helper text",
  },
  "webhooks.searchEndpointsEmpty": {
    message: "Нет вебхуков, соответствующих вашему запросу.",
    description: "Endpoint list empty state after local search",
  },
  "webhooks.emptyTitle": {
    message: "Нет вебхуков",
    description: "Webhooks empty-state title",
  },
  "webhooks.emptyBody": {
    message:
      "Добавьте эндпоинт, чтобы получать уведомления, когда ваши сервисы разворачиваются, перезапускаются или масштабируются.",
    description: "Webhooks empty-state body",
  },
  "webhooks.forbiddenTitle": {
    message: "Нет доступа",
    description: "Webhooks state when the caller lacks permission (403)",
  },
  "webhooks.forbiddenBody": {
    message: "У вас нет прав на просмотр вебхуков этого рабочего пространства.",
    description: "Webhooks forbidden-state body",
  },
  "webhooks.errorTitle": {
    message: "Не удалось загрузить вебхуки",
    description: "Webhooks generic error title",
  },
  "webhooks.errorBody": {
    message: "Что-то пошло не так. Повторите попытку.",
    description: "Webhooks generic error body",
  },
  "webhooks.proOnly": {
    message:
      "Вебхуки доступны на тарифе Pro и выше. Существующие эндпоинты продолжают получать доставки.",
    description: "Webhooks card description on a Hobby workspace (w8/m138)",
  },
  "webhooks.create": {
    message: "Добавить вебхук",
    description: "Button that opens the create dialog",
  },
  "webhooks.createTitle": {
    message: "Добавить вебхук",
    description: "Create dialog title",
  },
  "webhooks.createDescription": {
    message:
      "bex будет отправлять POST-запрос с подписанным JSON на этот URL при каждом событии, на которое оформлена подписка.",
    description: "Create dialog description",
  },
  "webhooks.createEnabledHelp": {
    message: "Начать доставку сразу. Включить её можно позже в настройках.",
    description: "Create page initial enabled-state helper copy",
  },
  "webhooks.fieldName": {
    message: "Название",
    description: "Create dialog name field label",
  },
  "webhooks.fieldNamePlaceholder": {
    message: "напр. deploy-alerts-slack-bot",
    description: "Create dialog name field placeholder",
  },
  "webhooks.fieldUrl": {
    message: "URL назначения",
    description: "Create dialog URL field label",
  },
  "webhooks.fieldEvents": {
    message: "События для отправки",
    description: "Create dialog event-type checklist label",
  },
  "webhooks.eventsLoading": {
    message: "Загрузка типов событий…",
    description: "Create dialog while the event-type vocabulary loads",
  },
  "webhooks.eventsLoadError": {
    message:
      "Не удалось загрузить типы событий. Повторите попытку перед сохранением.",
    description: "Actionable event-catalog query failure",
  },
  "webhooks.eventsRetry": {
    message: "Повторить загрузку типов событий",
    description: "Retry button for the event-catalog query",
  },
  "webhooks.eventsEmpty": {
    message: "Сейчас нет типов событий, на которые можно подписаться.",
    description: "Event picker state when the served catalog is empty",
  },
  "webhooks.formUnavailableEvents": {
    message: "Создание недоступно, пока не загружены типы событий.",
    description:
      "Create submit disabled reason while the catalog is unavailable",
  },
  "webhooks.formErrorsSummary": {
    message: "Исправьте выделенные поля и повторите попытку.",
    description: "Accessible webhook form validation summary",
  },
  "webhooks.validation.nameRequired": {
    message: "Введите название вебхука.",
    description: "Field-local required-name validation",
  },
  "webhooks.validation.nameDuplicate": {
    message: "Вебхук с таким названием уже существует.",
    description: "Field-local duplicate-name validation",
  },
  "webhooks.validation.urlRequired": {
    message: "Введите URL назначения.",
    description: "Field-local required URL validation",
  },
  "webhooks.validation.urlHTTPS": {
    message: "Введите абсолютный HTTPS-URL без встроенных учётных данных.",
    description: "Field-local webhook URL validation",
  },
  "webhooks.validation.eventsRequired": {
    message: "Выберите хотя бы одно событие.",
    description: "Field-local event selection validation",
  },
  "webhooks.validation.eventsInvalid": {
    message: "Выбор событий изменился. Перезагрузите каталог и выберите снова.",
    description: "Server-coded stale/invalid event filter",
  },
  "webhooks.errors.planLimit": {
    message:
      "Вебхуки доступны на тарифе Pro и выше. Это рабочее пространство на тарифе Hobby, поэтому добавить эндпоинт нельзя.",
    description:
      "Server-coded plan refusal of a webhook create on a Hobby workspace (WEBHOOK_PLAN_LIMIT, w4/425)",
  },
  "webhooks.planGate": {
    message:
      "Вебхуки доступны на тарифе Pro и выше. Перейдите на другой тариф, чтобы добавить эндпоинт.",
    description:
      "Notice in place of the create form and the empty list on a Hobby workspace (w4/425)",
  },
  "webhooks.errors.endpointLimit": {
    message: "Это рабочее пространство достигло лимита эндпоинтов вебхуков.",
    description: "Server-coded endpoint quota refusal",
  },
  "webhooks.errors.endpointDisabled": {
    message: "Включите этот эндпоинт, прежде чем повторно отправлять доставку.",
    description: "Server-coded resend refusal for a disabled endpoint",
  },
  "webhooks.errors.deliveryPending": {
    message: "Попытка доставки уже в очереди. Дождитесь её завершения.",
    description: "Server-coded duplicate pending resend refusal",
  },
  "webhooks.errors.staleState": {
    message:
      "Этот вебхук был изменён или удалён. Обновите страницу и повторите попытку.",
    description: "Safe stale endpoint/attempt refusal",
  },
  "webhooks.manageRequired": {
    message:
      "Вы можете просматривать этот вебхук, но изменять его могут только менеджеры рабочего пространства.",
    description: "Read-only member notice on webhook management surfaces",
  },
  "webhooks.latestNever": {
    message: "Не отправлялся",
    description: "Endpoint with no completed delivery attempt",
  },
  "webhooks.latestDelivered": {
    message: "Успешно",
    description: "Latest endpoint attempt succeeded",
  },
  "webhooks.latestFailed": {
    message: "Сбой",
    description: "Latest endpoint attempt failed terminally",
  },
  "webhooks.latestRetrying": {
    message: "Повторная попытка",
    description: "Latest endpoint attempt failed with a retry pending",
  },
  "webhooks.createCancel": {
    message: "Отмена",
    description: "Create dialog cancel button",
  },
  "webhooks.createSubmit": {
    message: "Создать вебхук",
    description: "Create page submit button (Render: 'Create Webhook')",
  },
  "webhooks.createSuccess": {
    message: "Вебхук создан",
    description: "Toast after a successful create",
  },
  "webhooks.createError": {
    message: "Не удалось создать вебхук",
    description: "Toast after a failed create",
  },
  "webhooks.copy": {
    message: "Копировать секрет подписи",
    description: "Copy-secret button label",
  },
  "webhooks.copied": {
    message: "Секрет скопирован в буфер обмена",
    description: "Toast after copying the secret",
  },
  "webhooks.copyError": {
    message: "Не удалось скопировать — выделите текст и скопируйте вручную",
    description: "Toast when clipboard write fails",
  },
  "webhooks.toggle": {
    message: "Включён",
    description: "Accessible label of the per-endpoint enable/disable switch",
  },
  "webhooks.enableSuccess": {
    message: "{name} включён",
    description: "Toast after enabling an endpoint",
  },
  "webhooks.disableSuccess": {
    message: "{name} отключён",
    description: "Toast after disabling an endpoint",
  },
  "webhooks.toggleError": {
    message: "Не удалось обновить {name}",
    description: "Toast after a failed enable/disable",
  },
  "webhooks.delete": {
    message: "Удалить",
    description: "Delete action label (icon button + confirm button)",
  },
  "webhooks.deleteConfirmTitle": {
    message: "Удалить {name}?",
    description: "Delete confirmation dialog title",
  },
  "webhooks.deleteConfirmBody": {
    message:
      "Новые события на этот эндпоинт отправляться не будут, а история его доставок будет удалена.",
    description: "Delete confirmation dialog body",
  },
  "webhooks.deleteCancel": {
    message: "Отмена",
    description: "Delete confirmation cancel button",
  },
  "webhooks.deleteSuccess": {
    message: "{name} удалён",
    description: "Toast after a successful delete",
  },
  "webhooks.deleteError": {
    message: "Не удалось удалить {name}",
    description: "Toast after a failed delete",
  },
  "webhooks.history": {
    message: "История доставок",
    description: "Accessible label of the per-endpoint history button",
  },
  "webhooks.historyTitle": {
    message: "Доставки — {name}",
    description: "Delivery-history dialog title",
  },
  "webhooks.historyBody": {
    message:
      "Каждое событие, отправленное на этот эндпоинт, сначала новые. Неудачные доставки повторяются с экспоненциальной задержкой; при повторяющихся сбоях эндпоинт отключается.",
    description: "Delivery-history dialog description",
  },
  "webhooks.historyEmptyTitle": {
    message: "Доставок пока нет",
    description: "Delivery-history empty-state title",
  },
  "webhooks.historyEmptyBody": {
    message:
      "Вызовите событие, на которое оформлена подписка, например деплой, и оно появится здесь.",
    description: "Delivery-history empty-state body",
  },
  "webhooks.historyFilteredEmptySuccessfulTitle": {
    message: "Нет успешных доставок",
    description:
      "Delivery-history state when the Successful tab (and any date range) matches nothing (w4/146)",
  },
  "webhooks.historyFilteredEmptyFailedTitle": {
    message: "Нет неудачных доставок",
    description:
      "Delivery-history state when the Failed tab (and any date range) matches nothing (w4/146)",
  },
  "webhooks.historyFilteredEmptyRangeTitle": {
    message: "Нет доставок в этом диапазоне",
    description:
      "Delivery-history state when the Sent after/before range matches nothing (w4/146)",
  },
  "webhooks.historyFilteredEmptyBody": {
    message:
      "Другие доставки могут относиться к другому статусу или периоду времени.",
    description: "Body under a filtered-empty delivery history (w4/146)",
  },
  "webhooks.historyClearFilters": {
    message: "Сбросить фильтры",
    description:
      "Resets the delivery-history status tab and date range (w4/146)",
  },
  "webhooks.historyErrorTitle": {
    message: "Не удалось загрузить доставки",
    description: "Delivery-history error-state title",
  },
  "webhooks.historyErrorBody": {
    message: "Что-то пошло не так. Повторите попытку.",
    description: "Delivery-history error-state body",
  },
  "webhooks.colEvent": {
    message: "Событие",
    description: "Delivery-history table column header",
  },
  "webhooks.colService": {
    message: "Сервис",
    description: "Delivery-history table column header",
  },
  "webhooks.serviceDeleted": {
    message: "(удалён)",
    description:
      "Marker beside a delivery's recorded service name when that service no longer exists, so the name cannot be linked",
  },
  "webhooks.colStatus": {
    message: "Статус",
    description: "Delivery-history table column header",
  },
  "webhooks.colAttempts": {
    message: "Попытка",
    description:
      "Delivery-history table column header — one-based attempt number",
  },
  "webhooks.colResponse": {
    message: "Ответ",
    description: "Delivery-history table column header — last HTTP status",
  },
  "webhooks.colWhen": {
    message: "Когда",
    description:
      "Delivery-history table column header — relative and exact send time",
  },
  "webhooks.colActions": {
    message: "Действия",
    description: "Accessible delivery-history table actions column header",
  },
  "webhooks.status.pending": {
    message: "Ожидает",
    description: "Delivery status badge — queued or between retries",
  },
  "webhooks.status.delivered": {
    message: "Доставлено",
    description: "Delivery status badge — endpoint answered 2xx",
  },
  "webhooks.status.failed": {
    message: "Сбой",
    description: "Delivery status badge — this individual attempt failed",
  },
  "webhooks.httpStatus": {
    message: "HTTP {status}",
    description: "Delivery attempt HTTP status summary",
  },
  "webhooks.openSourceEvent": {
    message: "{label} — открыть исходное событие {id}",
    description:
      "Accessible label for the event hydration link; it starts with the link's visible text, the event type (WCAG 2.5.3)",
  },
  "webhooks.requestPayload": {
    message: "Тело запроса",
    description: "Expanded delivery-attempt request evidence heading",
  },
  "webhooks.endpointResponse": {
    message: "Ответ эндпоинта",
    description: "Expanded delivery-attempt response evidence heading",
  },
  "webhooks.noRequestEvidence": {
    message: "Данных о запросе нет.",
    description: "Expanded attempt with no retained request body",
  },
  "webhooks.noResponseEvidence": {
    message: "Ответ не был записан.",
    description: "Expanded pending/transport attempt with no endpoint body",
  },
  "webhooks.attemptIdentity": {
    message: "Попытка {attemptId} · исходное событие {eventId}",
    description: "Immutable attempt and stable source-event identity",
  },
  "webhooks.parentDeliveryStatus": {
    message: "Состояние уведомления: {status}",
    description:
      "Logical notification state shown separately from attempt outcome",
  },
  "webhooks.retryScheduled": {
    message: "Следующая автоматическая попытка: {date}",
    description: "Parent notification retry schedule",
  },
  "webhooks.resend": {
    message: "Отправить повторно",
    description: "Manual resend action for a failed attempt",
  },
  "webhooks.resending": {
    message: "Повторная отправка…",
    description: "Manual resend action in progress",
  },
  "webhooks.resendConfirmTitle": {
    message: "Отправить этот вебхук повторно?",
    description: "Manual resend confirmation title",
  },
  "webhooks.resendConfirmBody": {
    message:
      "В очередь будет поставлена новая подписанная попытка с тем же исходным событием и телом запроса.",
    description: "Manual resend confirmation consequence",
  },
  "webhooks.resendCancel": {
    message: "Отмена",
    description: "Manual resend confirmation cancel action",
  },
  "webhooks.resendSuccess": {
    message: "Повторная отправка вебхука поставлена в очередь",
    description: "Toast after a manual resend reservation succeeds",
  },
  "webhooks.resendError": {
    message: "Не удалось повторно отправить этот вебхук",
    description: "Toast after a manual resend is refused or fails",
  },
  "webhooks.loadMore": {
    message: "Загрузить ещё",
    description: "Delivery-history pagination button",
  },
  "webhooks.selectedCount_one": {
    message: "Выбрано {count} событие",
    description: "Event-picker live selection counter (singular)",
  },
  "webhooks.selectedCount_few": {
    message: "Выбрано {count} события",
    description: "Event-picker live selection counter (plural)",
  },
  "webhooks.selectedCount_many": {
    message: "Выбрано {count} событий",
    description: "Event-picker live selection counter (plural)",
  },
  "webhooks.selectedCount_other": {
    message: "Выбрано {count} события",
    description: "Event-picker live selection counter (plural)",
  },
  "webhooks.searchEvents": {
    message: "Поиск событий",
    description: "Event-picker search box placeholder + accessible label",
  },
  "webhooks.searchNoMatches": {
    message: "Нет событий, соответствующих запросу",
    description:
      "Event-picker empty state while a search filters everything out",
  },
  "webhooks.allEvents": {
    message: "Все события",
    description: "Event-picker tri-state master checkbox label",
  },
  "webhooks.groupToggle": {
    message: "Переключить события: {group}",
    description: "Accessible label of a group's expand/collapse chevron",
  },
  "webhooks.group.deploy": {
    message: "Деплой",
    description: "Event-picker group — deploy lifecycle events",
  },
  "webhooks.group.autoDeploy": {
    message: "Автодеплой",
    description: "Event-picker group — automatic deploy setting events",
  },
  "webhooks.group.serviceAvailability": {
    message: "Доступность сервиса",
    description: "Event-picker group — observed service availability events",
  },
  "webhooks.group.scaling": {
    message: "Масштабирование",
    description: "Event-picker group — service scaling events",
  },
  "webhooks.group.cronJobRun": {
    message: "Запуск Cron Job",
    description: "Event-picker group — cron job run events",
  },
  "webhooks.group.maintenance": {
    message: "Обслуживание",
    description:
      "Event-picker group — Render maintenance events (shown only when already stored on an edited webhook, w4/709)",
  },
  "webhooks.group.maintenanceMode": {
    message: "Режим обслуживания",
    description: "Event-picker group — maintenance mode events",
  },
  "webhooks.group.disk": {
    message: "Диск",
    description: "Event-picker group — persistent disk events",
  },
  "webhooks.group.postgres": {
    message: "Postgres",
    description: "Event-picker group — managed Postgres events",
  },
  "webhooks.group.keyValue": {
    message: "Key Value",
    description: "Event-picker group — managed Key Value events",
  },
  "webhooks.group.suspension": {
    message: "Приостановка",
    description: "Event-picker group — suspend/resume events",
  },
  "webhooks.group.agentSession": {
    message: "Облачные агенты",
    description: "Event-picker group — cloud coding agent session events",
  },
  "webhooks.group.other": {
    message: "Другое",
    description:
      "Event-picker fallback group for served keys the catalog doesn't know yet",
  },
  "webhooks.event.deploy_started": {
    message: "Деплой начат",
    description: "Event label — deploy_started",
  },
  "webhooks.event.branch_deleted": {
    message: "Ветка удалена",
    description: "Webhook event label",
  },
  "webhooks.event.build_started": {
    message: "Сборка начата",
    description: "Webhook event label",
  },
  "webhooks.event.build_ended": {
    message: "Сборка завершена",
    description: "Webhook event label",
  },
  "webhooks.event.pre_deploy_started": {
    message: "Команда перед деплоем начата",
    description: "Webhook event label",
  },
  "webhooks.event.pre_deploy_ended": {
    message: "Команда перед деплоем завершена",
    description: "Webhook event label",
  },
  "webhooks.event.job_run_ended": {
    message: "Запуск задачи завершён",
    description: "Webhook event label",
  },
  "webhooks.event.auto_deploy_enabled": {
    message: "Автодеплой включён",
    description: "Webhook event label",
  },
  "webhooks.event.auto_deploy_disabled": {
    message: "Автодеплой отключён",
    description: "Webhook event label",
  },
  "webhooks.event.deploy_ended": {
    message: "Деплой завершён",
    description: "Event label — deploy_ended",
  },
  "webhooks.event.zero_downtime_redeploy_started": {
    message: "Начат повторный деплой без простоя",
    description:
      "Event label — zero_downtime_redeploy_started: a Bex-initiated zero-downtime deploy started for a service",
  },
  "webhooks.event.zero_downtime_redeploy_ended": {
    message: "Завершён повторный деплой без простоя",
    description:
      "Event label — zero_downtime_redeploy_ended: a Bex-initiated zero-downtime deploy completed for a service",
  },
  "webhooks.event.image_pull_failed": {
    message: "Не удалось загрузить образ",
    description: "Event label — image_pull_failed",
  },
  "webhooks.event.artifact_fetch_failed": {
    message: "Ошибка получения артефакта",
    description:
      "Event label — artifact_fetch_failed: a build published by a build source could not be fetched",
  },
  "webhooks.event.artifact_source_changed": {
    message: "Источник сборки изменён",
    description:
      "Event label — artifact_source_changed: a service's linked build source changed",
  },
  "webhooks.event.pipeline_minutes_exhausted": {
    message: "Минуты пайплайна исчерпаны",
    description: "Event label — pipeline_minutes_exhausted",
  },
  "webhooks.event.edge_cache_enabled": {
    message: "Edge-кеш включён",
    description:
      "Event label — edge_cache_enabled (API-only on Render; shown on subscriptions and deliveries, not in the picker)",
  },
  "webhooks.event.edge_cache_disabled": {
    message: "Edge-кеш отключён",
    description:
      "Event label — edge_cache_disabled (API-only on Render; shown on subscriptions and deliveries, not in the picker)",
  },
  "webhooks.event.edge_cache_purged": {
    message: "Edge-кеш очищен",
    description:
      "Event label — edge_cache_purged (API-only on Render; shown on subscriptions and deliveries, not in the picker)",
  },
  "webhooks.event.commit_ignored": {
    message: "Коммит проигнорирован",
    description: "Event label — commit_ignored",
  },
  "webhooks.event.server_failed": {
    message: "Сбой сервера",
    description: "Event label — server_failed",
  },
  "webhooks.event.server_hardware_failure": {
    message: "Аппаратный сбой сервера",
    description: "Event label — server_hardware_failure",
  },
  "webhooks.event.server_available": {
    message: "Сервер доступен",
    description: "Event label — server_available",
  },
  "webhooks.event.autoscaling_started": {
    message: "Автомасштабирование началось",
    description: "Event label — autoscaling_started",
  },
  "webhooks.event.autoscaling_ended": {
    message: "Автомасштабирование завершено",
    description: "Event label — autoscaling_ended",
  },
  "webhooks.event.branch_changed": {
    message: "Ветка изменена",
    description: "Event label — branch_changed bex extension",
  },
  "webhooks.event.cron_job_run_started": {
    message: "Запуск Cron Job начат",
    description: "Event label — cron_job_run_started",
  },
  "webhooks.event.cron_job_run_ended": {
    message: "Запуск Cron Job завершён",
    description: "Event label — cron_job_run_ended",
  },
  "webhooks.event.maintenance_started": {
    message: "Обслуживание начато",
    description:
      "Event label — maintenance_started (Render enum value accepted for compatibility, never emitted by bex, w4/709)",
  },
  "webhooks.event.maintenance_ended": {
    message: "Обслуживание завершено",
    description:
      "Event label — maintenance_ended (Render enum value accepted for compatibility, never emitted by bex, w4/709)",
  },
  "webhooks.event.maintenance_mode_enabled": {
    message: "Режим обслуживания включён",
    description: "Event label — maintenance_mode_enabled",
  },
  "webhooks.event.maintenance_mode_uri_updated": {
    message: "URI режима обслуживания обновлён",
    description: "Event label — maintenance_mode_uri_updated",
  },
  "webhooks.event.disk_created": {
    message: "Диск создан",
    description: "Event label — disk_created",
  },
  "webhooks.event.disk_updated": {
    message: "Диск обновлён",
    description: "Event label — disk_updated",
  },
  "webhooks.event.disk_deleted": {
    message: "Диск удалён",
    description: "Event label — disk_deleted",
  },
  "webhooks.event.disk_snapshot_restored": {
    message: "Снимок диска восстановлен",
    description: "Event label — disk_snapshot_restored",
  },
  "webhooks.event.postgres_created": {
    message: "Postgres создан",
    description: "Event label — postgres_created",
  },
  "webhooks.event.postgres_restarted": {
    message: "Postgres перезапущен",
    description: "Event label — postgres_restarted",
  },
  "webhooks.event.postgres_credentials_created": {
    message: "Учётные данные Postgres созданы",
    description: "Event label — postgres_credentials_created",
  },
  "webhooks.event.postgres_credentials_deleted": {
    message: "Учётные данные Postgres удалены",
    description: "Event label — postgres_credentials_deleted",
  },
  "webhooks.event.postgres_backup_started": {
    message: "Резервное копирование Postgres начато",
    description: "Event label — postgres_backup_started",
  },
  "webhooks.event.postgres_available": {
    message: "Postgres доступен",
    description: "Event label — postgres_available",
  },
  "webhooks.event.postgres_unavailable": {
    message: "Postgres недоступен",
    description: "Event label — postgres_unavailable",
  },
  "webhooks.event.postgres_ha_status_changed": {
    message: "Высокая доступность Postgres изменена",
    description: "Event label — postgres_ha_status_changed",
  },
  "webhooks.event.postgres_cluster_leader_changed": {
    message: "Лидер кластера Postgres изменён",
    description: "Event label — postgres_cluster_leader_changed",
  },
  "webhooks.event.postgres_connection_pool_changed": {
    message: "Конфигурация пула подключений Postgres изменена",
    description:
      "Event label — postgres_connection_pool_changed (Render enum value accepted for compatibility, never emitted by bex, w4/709)",
  },
  "webhooks.event.postgres_connection_pool_enabled_changed": {
    message: "Пул подключений Postgres изменён",
    description: "Event label — postgres_connection_pool_enabled_changed",
  },
  "webhooks.event.postgres_disk_size_changed": {
    message: "Размер диска Postgres изменён",
    description: "Event label — postgres_disk_size_changed",
  },
  "webhooks.event.postgres_disk_autoscaling_enabled_changed": {
    message: "Автомасштабирование диска Postgres изменено",
    description: "Event label — postgres_disk_autoscaling_enabled_changed",
  },
  "webhooks.event.postgres_read_replicas_changed": {
    message: "Реплики Postgres для чтения изменены",
    description: "Event label — postgres_read_replicas_changed",
  },
  "webhooks.event.postgres_read_replica_stale": {
    message: "Реплика Postgres для чтения устарела",
    description: "Event label — postgres_read_replica_stale",
  },
  "webhooks.event.postgres_backup_completed": {
    message: "Резервное копирование Postgres завершено",
    description: "Event label — postgres_backup_completed",
  },
  "webhooks.event.postgres_backup_failed": {
    message: "Сбой резервного копирования Postgres",
    description: "Event label — postgres_backup_failed",
  },
  "webhooks.event.postgres_pitr_checkpoint_started": {
    message: "Контрольная точка PITR Postgres начата",
    description: "Event label — postgres_pitr_checkpoint_started",
  },
  "webhooks.event.postgres_pitr_checkpoint_completed": {
    message: "Контрольная точка PITR Postgres завершена",
    description: "Event label — postgres_pitr_checkpoint_completed",
  },
  "webhooks.event.postgres_pitr_checkpoint_failed": {
    message: "Сбой контрольной точки PITR Postgres",
    description: "Event label — postgres_pitr_checkpoint_failed",
  },
  "webhooks.event.postgres_wal_archive_failed": {
    message: "Сбой архивации WAL Postgres",
    description: "Event label — postgres_wal_archive_failed",
  },
  "webhooks.event.postgres_restore_succeeded": {
    message: "Восстановление Postgres выполнено",
    description: "Event label — postgres_restore_succeeded",
  },
  "webhooks.event.postgres_restore_failed": {
    message: "Сбой восстановления Postgres",
    description: "Event label — postgres_restore_failed",
  },
  "webhooks.event.postgres_upgrade_started": {
    message: "Обновление Postgres начато",
    description: "Event label — postgres_upgrade_started",
  },
  "webhooks.event.postgres_upgrade_succeeded": {
    message: "Обновление Postgres выполнено",
    description: "Event label — postgres_upgrade_succeeded",
  },
  "webhooks.event.postgres_upgrade_failed": {
    message: "Сбой обновления Postgres",
    description: "Event label — postgres_upgrade_failed",
  },
  "webhooks.event.key_value_available": {
    message: "Key Value доступен",
    description: "Event label — key_value_available",
  },
  "webhooks.event.key_value_unhealthy": {
    message: "Key Value неработоспособен",
    description: "Event label — key_value_unhealthy",
  },
  "webhooks.event.key_value_config_restart": {
    message: "Key Value перезапущен из-за изменения конфигурации",
    description: "Event label — key_value_config_restart",
  },
  "webhooks.event.service_suspended": {
    message: "Сервис приостановлен",
    description: "Event label — service_suspended",
  },
  "webhooks.event.service_resumed": {
    message: "Сервис возобновлён",
    description: "Event label — service_resumed",
  },
  "webhooks.event.service_hibernated": {
    message: "Сервис уснул",
    description: "Event label — service_hibernated",
  },
  "webhooks.event.service_woken": {
    message: "Сервис проснулся",
    description: "Event label — service_woken",
  },
  "webhooks.event.agent_session_turn_succeeded": {
    message: "Ход сессии агента выполнен успешно",
    description: "Event label — agent_session_turn_succeeded",
  },
  "webhooks.event.agent_session_turn_failed": {
    message: "Сбой хода сессии агента",
    description: "Event label — agent_session_turn_failed",
  },
  "webhooks.event.agent_session_needs_input": {
    message: "Сессии агента нужен ввод",
    description: "Event label — agent_session_needs_input",
  },
  "webhooks.event.service_moved": {
    message: "Сервис перемещён",
    description:
      "Event label — service_moved (bex extension: project/environment placement change)",
  },
  "webhooks.event.server_restarted": {
    message: "Сервер перезапущен",
    description: "Event label — server_restarted",
  },
  "webhooks.event.instance_count_changed": {
    message: "Число инстансов изменено",
    description: "Event label — instance_count_changed",
  },
  "webhooks.event.autoscaling_config_changed": {
    message: "Конфигурация автомасштабирования изменена",
    description: "Event label — autoscaling_config_changed",
  },
  "webhooks.event.plan_changed": {
    message: "Тип инстанса изменён",
    description:
      "Event label — plan_changed (bex's slot for Render's Instance Type Changed)",
  },
  "webhooks.newTitle": {
    message: "Создать новый вебхук",
    description: "Create page heading (Render: 'Create a new Webhook')",
  },
  "webhooks.newMenuLabel": {
    message: "Новый вебхук",
    description:
      "+ New menu entry; worded like the other entries, unlike the create page heading",
  },
  "webhooks.backToList": {
    message: "Назад к вебхукам",
    description: "Accessible label of the create/detail pages' back link",
  },
  "webhooks.fieldNameHelp": {
    message: "Уникальное название этого вебхука.",
    description: "Create page name-field helper copy",
  },
  "webhooks.fieldUrlHelp": {
    message: "bex отправляет каждое уведомление на этот URL POST-запросом.",
    description: "Create page URL-field helper copy",
  },
  "webhooks.fieldUrlPlaceholder": {
    message: "https://example.com/webhooks/bex",
    description: "Create page URL-field placeholder",
  },
  "webhooks.fieldEventsHelp": {
    message:
      "Выберите, какие события в вашем рабочем пространстве будут вызывать уведомление вебхука.",
    description: "Create page events-field helper copy",
  },
  "webhooks.detailKicker": {
    message: "Вебхук",
    description: "Detail page header kicker above the endpoint name",
  },
  "webhooks.idLabel": {
    message: "ID вебхука:",
    description: "Detail header id row label",
  },
  "webhooks.copyId": {
    message: "Копировать ID вебхука",
    description: "Accessible label of the header id copy button",
  },
  "webhooks.copyUrl": {
    message: "Копировать URL эндпоинта",
    description: "Accessible label of the header URL copy button",
  },
  "webhooks.copiedGeneric": {
    message: "Скопировано в буфер обмена",
    description: "Toast after copying the id or URL",
  },
  "webhooks.showMore": {
    message: "Показать ещё {count}",
    description: "Event-chip expander on the detail header",
  },
  "webhooks.showLess": {
    message: "Показать меньше",
    description: "Event-chip collapser on the detail header",
  },
  "webhooks.createdByOn": {
    message: "Создал {creator}, {date}",
    description: "Detail header provenance line when the creator is known",
  },
  "webhooks.createdOn": {
    message: "Создан {date}",
    description: "Detail header provenance line without a creator",
  },
  "webhooks.tabActivity": {
    message: "Активность",
    description: "Detail page tab — delivery history",
  },
  "webhooks.tabSettings": {
    message: "Настройки",
    description: "Detail page tab — edit + delete",
  },
  "webhooks.recentDeliveries": {
    message: "Недавние доставки",
    description: "Activity tab heading",
  },
  "webhooks.recentDeliveriesHint": {
    message:
      "Каждая строка — одна попытка доставки. Новые попытки появляются автоматически, пока эта вкладка открыта.",
    description: "Activity tab heading hint for immutable attempts + polling",
  },
  "webhooks.refresh": {
    message: "Обновить",
    description: "Accessible label of the Activity refresh button",
  },
  "webhooks.filterAll": {
    message: "Все",
    description: "Activity delivery filter tab",
  },
  "webhooks.filterSuccessful": {
    message: "Успешные",
    description: "Activity delivery filter tab — delivered only",
  },
  "webhooks.filterFailed": {
    message: "Неудачные",
    description: "Activity delivery filter tab — failed only",
  },
  "webhooks.sentAfter": {
    message: "Отправлено после",
    description: "Activity delivery-history lower timestamp bound",
  },
  "webhooks.sentBefore": {
    message: "Отправлено до",
    description: "Activity delivery-history upper timestamp bound",
  },
  "webhooks.destinationBlocked": {
    message: "Адрес назначения не разрешён",
    description:
      "Delivery result label when bex refused to dial the destination: it resolves to a loopback, private, link-local or cluster-internal address (w21/046)",
  },
  "webhooks.destinationUnreachableNotice": {
    message:
      "Этот адрес недоступен из интернета, поэтому все доставки на него будут завершаться сбоем. Используйте публичный HTTPS-адрес.",
    description:
      "Non-blocking notice under the webhook Destination URL when its host is a loopback, private, link-local or cluster-internal address (w21/046)",
  },
  "webhooks.transportError": {
    message: "Ошибка транспорта",
    description: "Delivery evidence label when no HTTP response was received",
  },
  "webhooks.enabledBadge": {
    message: "Включён",
    description: "Detail header status badge while enabled",
  },
  "webhooks.disabledBadge": {
    message: "Отключён",
    description: "Detail header status badge while disabled",
  },
  "webhooks.view": {
    message: "Открыть вебхук",
    description: "Accessible label of the list row's open-detail link",
  },
  "webhooks.settingsGeneral": {
    message: "Общие",
    description: "Settings tab first section heading",
  },
  "webhooks.disabledReasonAuto": {
    message: "Отключён автоматически после повторяющихся сбоев доставки.",
    description:
      "Reason shown beside a webhook the platform disabled after repeated delivery failures",
  },
  "webhooks.settingsStatus": {
    message: "Статус",
    description: "Settings status-toggle label",
  },
  "webhooks.settingsStatusHelp": {
    message: "Пока вебхук отключён, он не отправляет уведомления.",
    description: "Settings status-toggle helper copy",
  },
  "webhooks.settingsEvents": {
    message: "События подписки",
    description: "Settings events section heading",
  },
  "webhooks.saveChanges": {
    message: "Сохранить изменения",
    description: "Settings submit button (disabled until dirty)",
  },
  "webhooks.updateSuccess": {
    message: "Вебхук обновлён",
    description: "Toast after a successful settings save",
  },
  "webhooks.updateError": {
    message: "Не удалось обновить вебхук",
    description: "Toast after a failed settings save",
  },
  "webhooks.secretLabel": {
    message: "Секрет подписи",
    description: "Settings signing-secret row label",
  },
  "webhooks.secretHelp": {
    message:
      "Токен для подписи и проверки тел запросов. Просматривать его могут только администраторы рабочего пространства.",
    description: "Settings signing-secret row help (Render's wording)",
  },
  "webhooks.secretShow": {
    message: "Показать секрет",
    description: "Settings button that reveals the masked signing secret",
  },
  "webhooks.secretHide": {
    message: "Скрыть секрет",
    description: "Settings button that masks the revealed signing secret",
  },
  "webhooks.deleteSection": {
    message: "Удалить вебхук",
    description: "Settings danger-zone heading",
  },
  "webhooks.deleteTypeToConfirm": {
    message: "Введите {command} ниже для подтверждения.",
    description: "Type-to-confirm instruction in the delete dialog",
  },
  "webhooks.deleteCommandLabel": {
    message: "Подтверждение",
    description: "Accessible label of the type-to-confirm input",
  },
};

export default ruWebhooks;
