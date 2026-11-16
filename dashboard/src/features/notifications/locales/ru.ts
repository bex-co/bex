import type { TranslationEntry } from "@/i18n";

const ruNotifications: Record<string, TranslationEntry> = {
  "notifications.pageDescription": {
    message:
      "Выберите, о каких событиях и где получать уведомления: по электронной почте, в мобильном приложении bex или в этом браузере.",
    description: "Notifications page header description",
  },
  "notifications.emailTitle": {
    message: "Электронная почта",
    description:
      "Notifications page: email preferences card title and section link",
  },
  "notifications.sectionNavigation": {
    message: "Разделы уведомлений",
    description: "Accessible name of the Notifications page section navigation",
  },
  "notifications.title": {
    message: "Уведомления",
    description: "Settings Notifications section card title",
  },
  "notifications.description": {
    message:
      "Получайте письма, когда деплой одного из ваших сервисов начинается, проходит успешно или завершается ошибкой, а также когда что-то ломается во время работы. Это ваши личные настройки — каждый участник рабочего пространства задаёт их независимо.",
    description: "Settings Notifications section card description",
  },
  "notifications.deployStarted": {
    message: "Деплой начался",
    description: "Toggle label",
  },
  "notifications.deployStartedHint": {
    message:
      "Присылать письмо, когда начинается деплой. Отправляется только для сервисов, которые отправляют все уведомления.",
    description: "Toggle hint",
  },
  "notifications.deploySucceeded": {
    message: "Деплой прошёл успешно",
    description: "Toggle label",
  },
  "notifications.deploySucceededHint": {
    message:
      "Присылать письмо, когда деплой запущен. Отправляется только для сервисов, которые отправляют все уведомления.",
    description: "Toggle hint",
  },
  "notifications.deployFailed": {
    message: "Сбои",
    description: "Toggle label",
  },
  "notifications.deployFailedHint": {
    message:
      "Присылать письмо, когда деплой завершается ошибкой, сервис становится неработоспособным, запуск Cron Job завершается ошибкой, не удаётся загрузить образ или не удаётся синхронизировать Blueprint.",
    description: "Toggle hint",
  },
  "notifications.errorTitle": {
    message: "Не удалось загрузить настройки уведомлений",
    description: "Generic error title",
  },
  "notifications.errorBody": {
    message: "Что-то пошло не так. Попробуйте ещё раз.",
    description: "Generic error body",
  },
  "notifications.updateError": {
    message: "Не удалось сохранить ваши настройки",
    description: "Toast on a failed update",
  },
  "notifications.pushTitle": {
    message: "Мобильные push-уведомления",
    description: "Push settings card title",
  },
  "notifications.bexExtension": {
    message: "расширение bex",
    description: "Badge distinguishing native push from Render parity",
  },
  "notifications.pushDescription": {
    message:
      "Управляйте мобильными оповещениями bex, их срочностью, часами доставки и исключениями для отдельных сервисов. Настройки почты выше не меняются.",
    description: "Push settings description",
  },
  "notifications.pushEnabled": {
    message: "Включить мобильные push-уведомления",
    description: "Push enabled toggle",
  },
  "notifications.pushEnabledHint": {
    message:
      "Отправлять оповещения на ваши зарегистрированные мобильные устройства bex.",
    description: "Push enabled hint",
  },
  "notifications.pushUnavailable": {
    message: "На этом сервере bex не настроена доставка push-уведомлений",
    description: "Disabled server status",
  },
  "notifications.pushUnavailableHint": {
    message:
      "Вы можете подготовить и сохранить политику уже сейчас, но зарегистрированные устройства не смогут получать уведомления, пока оператор не настроит push-провайдера.",
    description: "Disabled server status hint",
  },
  "notifications.webPushTitle": {
    message: "Уведомления в браузере",
    description: "Web push settings card title",
  },
  "notifications.webPushDescription": {
    message:
      "Получайте те же оповещения в этом браузере. Фильтры событий и тихие часы — те, что вы задали в разделе «Мобильные push-уведомления»; отдельной политики нет.",
    description: "Web push settings description",
  },
  "notifications.webPushUnsupported": {
    message: "Этот браузер не поддерживает web push.",
    description: "Unsupported browser",
  },
  "notifications.webPushUnconfigured": {
    message: "На этом сервере bex не включены push-уведомления в браузере.",
    description: "Server VAPID unset",
  },
  "notifications.webPushDenied": {
    message:
      "Уведомления для этого сайта заблокированы. Разрешите их в настройках браузера и повторите попытку.",
    description: "Permission denied",
  },
  "notifications.webPushError": {
    message: "Не удалось обновить уведомления в браузере. Попробуйте ещё раз.",
    description: "Subscribe error",
  },
  "notifications.webPushOn": {
    message: "Этот браузер будет получать оповещения bex.",
    description: "Subscribed state",
  },
  "notifications.webPushOff": {
    message: "Этот браузер не подписан.",
    description: "Unsubscribed state",
  },
  "notifications.webPushEnable": {
    message: "Включить в этом браузере",
    description: "Subscribe button",
  },
  "notifications.webPushDisable": {
    message: "Отключить в этом браузере",
    description: "Unsubscribe button",
  },
  "notifications.pushEvents": {
    message: "Фильтры событий",
    description: "Push event section",
  },
  "notifications.pushEventsHint": {
    message:
      "Push-уведомление могут вызвать только выбранные типы событий с срочностью не ниже минимальной.",
    description: "Push events hint",
  },
  "notifications.pushEventNotSentYet": {
    message: "Пока не отправляется: сейчас ничто не отправляет это событие.",
    description:
      "Under a push event that no part of bex emits yet (it is reserved for an upcoming feature); selecting it is kept but has no effect",
  },
  "notifications.pushEventNotSentYetShort": {
    message: "Пока не отправляется",
    description:
      "Compact form of the not-sent-yet note, under a push event in a per-service override's event grid",
  },
  "notifications.pushEventFiltered": {
    message: "Не отправляется при минимальной срочности «{minimum}».",
    description:
      "Under a selected push event whose urgency is below the minimum ({minimum} = urgency name, e.g. 'Important')",
  },
  "notifications.pushFilteredTitle": {
    message: "Некоторые выбранные события не будут отправлены",
    description:
      "Heading of the save-area summary listing selected push events the minimum urgency drops",
  },
  "notifications.pushFilteredEvent": {
    message:
      "Событие «{event}» имеет срочность «{urgency}» и не будет отправлено при минимальной срочности «{minimum}».",
    description:
      "One summary line ({event} = event name, {urgency} = its urgency, {minimum} = the minimum urgency)",
  },
  "notifications.pushFilteredServiceEvent": {
    message:
      "{service}: событие «{event}» имеет срочность «{urgency}» и не будет отправлено при минимальной срочности «{minimum}» для этого сервиса.",
    description:
      "One summary line for a service override ({service} = service name; others as pushFilteredEvent)",
  },
  "notifications.pushOverrideFiltered_one": {
    message:
      "{count} выбранное событие не будет отправлено для этого сервиса при его минимальной срочности «{minimum}».",
    description:
      "Under a service override whose own policy drops selected events ({minimum} = urgency name)",
  },
  "notifications.pushOverrideFiltered_few": {
    message:
      "{count} выбранных события не будут отправлены для этого сервиса при его минимальной срочности «{minimum}».",
    description:
      "Under a service override whose own policy drops selected events ({minimum} = urgency name)",
  },
  "notifications.pushOverrideFiltered_many": {
    message:
      "{count} выбранных событий не будут отправлены для этого сервиса при его минимальной срочности «{minimum}».",
    description:
      "Under a service override whose own policy drops selected events ({minimum} = urgency name)",
  },
  "notifications.pushOverrideFiltered_other": {
    message:
      "{count} выбранного события не будут отправлены для этого сервиса при его минимальной срочности «{minimum}».",
    description:
      "Under a service override whose own policy drops selected events ({minimum} = urgency name)",
  },
  "notifications.pushEventDeployFailed": {
    message: "Деплой завершился с ошибкой",
    description: "Push event",
  },
  "notifications.pushEventServerFailed": {
    message: "Сервис упал",
    description: "Push event",
  },
  "notifications.pushEventCronFailed": {
    message: "Запуск Cron Job завершился с ошибкой",
    description: "Push event",
  },
  "notifications.pushEventDeployStarted": {
    message: "Деплой начался",
    description: "Push event",
  },
  "notifications.pushEventDeploySucceeded": {
    message: "Деплой прошёл успешно",
    description: "Push event",
  },
  "notifications.pushEventServerAvailable": {
    message: "Сервис восстановлен",
    description: "Push event",
  },
  "notifications.pushEventServiceSuspended": {
    message: "Сервис приостановлен",
    description: "Push event",
  },
  "notifications.pushEventServiceResumed": {
    message: "Сервис возобновлён",
    description: "Push event",
  },
  "notifications.pushEventAgentNeedsDecision": {
    message: "Агенту нужно решение",
    description: "Push event",
  },
  "notifications.pushEventAgentPrReady": {
    message: "PR агента готов",
    description: "Push event",
  },
  "notifications.pushEventAgentFailed": {
    message: "Сессия агента завершилась ошибкой",
    description: "Push event",
  },
  "notifications.pushEventAgentAutomationPaused": {
    message: "Автоматизация приостановлена",
    description: "Push event: the platform paused an agent automation",
  },
  "notifications.pushEventPostgresUnavailable": {
    message: "База данных недоступна",
    description: "Push event",
  },
  "notifications.pushEventPostgresAvailable": {
    message: "База данных восстановлена",
    description: "Push event",
  },
  "notifications.pushEventKeyValueUnhealthy": {
    message: "Key Value неработоспособен",
    description: "Push event",
  },
  "notifications.pushEventKeyValueAvailable": {
    message: "Key Value восстановлен",
    description: "Push event",
  },
  "notifications.pushEventPostgresBackupFailed": {
    message: "Не удалось создать резервную копию базы данных",
    description: "Push event",
  },
  "notifications.pushEventPostgresRestoreFailed": {
    message: "Не удалось восстановить базу данных",
    description: "Push event",
  },
  "notifications.pushEventPostgresUpgradeFailed": {
    message: "Не удалось обновить базу данных",
    description: "Push event",
  },
  "notifications.pushMinimumUrgency": {
    message: "Минимальная срочность",
    description: "Urgency field",
  },
  "notifications.pushUrgencyRoutine": {
    message: "Обычная",
    description: "Urgency option",
  },
  "notifications.pushUrgencyImportant": {
    message: "Важная",
    description: "Urgency option",
  },
  "notifications.pushUrgencyCritical": {
    message: "Критическая",
    description: "Urgency option",
  },
  "notifications.pushTimezone": {
    message: "Часовой пояс IANA",
    description: "Timezone field",
  },
  "notifications.pushTimezonePlaceholder": {
    message: "America/Los_Angeles",
    description: "Timezone placeholder",
  },
  "notifications.pushMaxDeferral": {
    message: "Макс. отсрочка (часы)",
    description: "Deferral field",
  },
  "notifications.pushWorkingHours": {
    message: "Рабочие часы",
    description: "Working hours heading",
  },
  "notifications.pushWorkingHoursHint": {
    message:
      "Если задано, некритичные оповещения ждут начала одного из этих интервалов.",
    description: "Working hours hint",
  },
  "notifications.pushQuietHours": {
    message: "Тихие часы",
    description: "Quiet hours heading",
  },
  "notifications.pushQuietHoursHint": {
    message: "Некритичные оповещения ждут, пока действует тихий интервал.",
    description: "Quiet hours hint",
  },
  "notifications.weekdayMonday": {
    message: "Пн",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayTuesday": {
    message: "Вт",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayWednesday": {
    message: "Ср",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayThursday": {
    message: "Чт",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayFriday": {
    message: "Пт",
    description: "Weekday abbreviation",
  },
  "notifications.weekdaySaturday": {
    message: "Сб",
    description: "Weekday abbreviation",
  },
  "notifications.weekdaySunday": {
    message: "Вс",
    description: "Weekday abbreviation",
  },
  "notifications.pushRangeStart": {
    message: "Начало интервала",
    description: "Accessible time input label",
  },
  "notifications.pushRangeEnd": {
    message: "Конец интервала",
    description: "Accessible time input label",
  },
  "notifications.pushRemoveRange": {
    message: "Удалить интервал",
    description: "Accessible remove button label",
  },
  "notifications.pushAddRange": {
    message: "Добавить интервал",
    description: "Add range button",
  },
  "notifications.pushOverrides": {
    message: "Переопределения для сервисов",
    description: "Overrides heading",
  },
  "notifications.pushOverridesHint": {
    message:
      "Незаданные поля наследуют настройки выше; пустой список событий отключает этот сервис.",
    description: "Override semantics hint",
  },
  "notifications.pushRemoveOverride": {
    message: "Удалить переопределение для сервиса",
    description: "Accessible remove override label",
  },
  "notifications.pushOverrideEnabled": {
    message: "Доставка",
    description: "Override enabled field",
  },
  "notifications.pushInherit": {
    message: "Наследовать",
    description: "Override option",
  },
  "notifications.pushOn": {
    message: "Вкл.",
    description: "Override option",
  },
  "notifications.pushOff": {
    message: "Выкл.",
    description: "Override option",
  },
  "notifications.pushExactEvents": {
    message: "Переопределить точным списком событий",
    description: "Exact event filter toggle",
  },
  "notifications.pushAddOverride": {
    message: "Добавить переопределение для сервиса…",
    description: "Add override selector",
  },
  "notifications.pushSave": {
    message: "Сохранить настройки push",
    description: "Save button",
  },
  "notifications.pushUnsaved": {
    message: "Есть несохранённые изменения",
    description:
      "Hint beside Save push settings while the push form differs from the saved settings (w4/435)",
  },
  "notifications.pushDiscardTitle": {
    message: "Отменить изменения настроек push?",
    description:
      "Navigation guard title when the push form has unsaved changes (w4/435)",
  },
  "notifications.pushDiscardBody": {
    message: "Эти изменения не сохранены и будут потеряны, если вы уйдёте.",
    description: "Navigation guard explanation for unsaved push settings",
  },
  "notifications.pushKeepEditing": {
    message: "Продолжить редактирование",
    description: "Cancel the navigation and stay on the push form",
  },
  "notifications.pushDiscard": {
    message: "Отменить и уйти",
    description: "Confirm leaving and dropping unsaved push settings",
  },
  "notifications.pushSaved": {
    message: "Настройки push сохранены",
    description: "Success toast",
  },
  "notifications.pushUpdateError": {
    message: "Не удалось сохранить настройки push",
    description: "Failure toast",
  },
  "notifications.pushErrorTitle": {
    message: "Не удалось загрузить настройки push",
    description: "Push load error title",
  },
  "notifications.pushInvalidTimezone": {
    message:
      "Введите корректный часовой пояс IANA, например America/Los_Angeles.",
    description: "Timezone validation",
  },
  "notifications.pushInvalidDeferral": {
    message: "Максимальная отсрочка должна быть от 0.5 до 168 часов.",
    description: "Deferral validation",
  },
  "notifications.pushInvalidEvents": {
    message: "Фильтр событий содержит неподдерживаемое значение.",
    description: "Event validation",
  },
  "notifications.pushInvalidUrgency": {
    message: "Выберите корректную срочность.",
    description: "Urgency validation",
  },
  "notifications.pushInvalidRange": {
    message:
      "Для каждого интервала нужны дни недели, корректное время начала и конца, и они не должны совпадать.",
    description: "Range validation",
  },
  "notifications.pushTooManyRules": {
    message:
      "Слишком много интервалов расписания или переопределений для сервисов.",
    description: "Rule count validation",
  },
  "notifications.pushInvalidService": {
    message:
      "Переопределения для сервисов должны быть уникальными и ссылаться на существующий сервис.",
    description: "Service validation",
  },
  "notifications.pushEmptyOverride": {
    message:
      "Каждое переопределение для сервиса должно менять доставку, срочность или события.",
    description: "Empty override validation",
  },
  "notifications.workspaceTitle": {
    message: "Уведомления",
    description:
      "Workspace Settings → Notifications card title (the workspace notification policy)",
  },
  "notifications.workspaceDescription": {
    message:
      "Значения по умолчанию для всех сервисов. Сервис может переопределить их в своих настройках.",
    description: "Workspace Settings → Notifications card description",
  },
  "notifications.workspaceReadOnly": {
    message:
      "Изменять эти значения по умолчанию могут только администраторы рабочего пространства.",
    description: "Workspace notification policy: read-only note for non-admins",
  },
  "notifications.workspaceEmailLabel": {
    message: "Уведомления по почте",
    description: "Workspace notification policy: email destination switch",
  },
  "notifications.workspaceEmailHint": {
    message:
      "Если выключено, письма с уведомлениями не отправляются независимо от настроек сервиса.",
    description: "Workspace notification policy: email destination hint",
  },
  "notifications.workspaceLevelLabel": {
    message: "Уведомления сервисов по умолчанию",
    description: "Workspace notification policy: level radio group label",
  },
  "notifications.workspaceLevelHint": {
    message: "Что отправляют сервисы рабочего пространства по умолчанию.",
    description: "Workspace notification policy: level radio group hint",
  },
  "notifications.levelAll": {
    message: "Все уведомления",
    description: "Notification level: every notification",
  },
  "notifications.levelAllHint": {
    message: "Сбои, а также начало и запуск деплоев и восстановления.",
    description: "Notification level hint: all",
  },
  "notifications.levelFailure": {
    message: "Только уведомления о сбоях",
    description: "Notification level: failures only (Render's default)",
  },
  "notifications.levelFailureHint": {
    message:
      "Неудачные деплои, неработоспособные сервисы, неудачные запуски Cron Job, переполненные диски.",
    description: "Notification level hint: failure",
  },
  "notifications.levelNone": {
    message: "Нет",
    description: "Notification level: nothing",
  },
  "notifications.levelNoneHint": {
    message: "Не отправлять уведомления для этих сервисов.",
    description: "Notification level hint: none",
  },
  "notifications.workspaceSave": {
    message: "Сохранить",
    description: "Workspace notification policy: save button",
  },
  "notifications.workspaceSaved": {
    message: "Значения по умолчанию для уведомлений сохранены",
    description: "Workspace notification policy: success toast",
  },
  "notifications.workspaceSaveFailed": {
    message:
      "Не удалось сохранить значения по умолчанию для уведомлений. Попробуйте ещё раз.",
    description: "Workspace notification policy: save error",
  },
  "notifications.workspaceLoadFailed": {
    message:
      "Не удалось загрузить значения по умолчанию для уведомлений рабочего пространства.",
    description: "Workspace notification policy: load error",
  },
  "notifications.workspaceRetry": {
    message: "Повторить",
    description: "Workspace notification policy: retry button",
  },
  "notifications.overridesTitle": {
    message: "Переопределения уведомлений",
    description:
      "Workspace notifications: overrides list title (Render's label)",
  },
  "notifications.overridesDescription": {
    message:
      "Сервисы, у которых настройка уведомлений отличается от значения по умолчанию для рабочего пространства.",
    description: "Workspace notifications: overrides list description",
  },
  "notifications.overridesEmpty": {
    message:
      "Ни один сервис не переопределяет значение по умолчанию для рабочего пространства.",
    description: "Workspace notifications: no overrides",
  },
  "notifications.overridesError": {
    message:
      "Не удалось загрузить сервисы, переопределяющие значение по умолчанию.",
    description: "Workspace notifications: overrides failed to load",
  },
};

export default ruNotifications;
