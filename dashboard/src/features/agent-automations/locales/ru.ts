import type { TranslationEntry } from "@/i18n";

const ruAgentAutomations: Record<string, TranslationEntry> = {
  "agentAutomations.pageTitle": {
    message: "Автоматизации",
    description:
      "Page title and sidebar label of the scheduled agent automations page",
  },
  "agentAutomations.detailTitle": {
    message: "Автоматизация",
    description: "Browser tab title of one automation's page",
  },
  "agentAutomations.pageDescription": {
    message:
      "Запускайте сессию агента по расписанию. Каждый запуск использует задачу и настройки, которые вы сохранили здесь.",
    description: "Automations list page description",
  },
  "agentAutomations.create": {
    message: "Создать автоматизацию",
    description:
      "Create automation button on the list page and in the empty state",
  },
  "agentAutomations.createTitle": {
    message: "Создание автоматизации",
    description: "Heading of the create form",
  },
  "agentAutomations.editTitle": {
    message: "Редактирование автоматизации",
    description: "Heading of the edit form",
  },
  "agentAutomations.createSubmit": {
    message: "Создать автоматизацию",
    description: "Create form submit button",
  },
  "agentAutomations.editSubmit": {
    message: "Сохранить изменения",
    description: "Edit form submit button",
  },
  "agentAutomations.createDenied": {
    message:
      "Создавать автоматизации могут только участники с ролью разработчика или выше.",
    description: "Create page notice for a member who cannot create",
  },
  "agentAutomations.backToList": {
    message: "Назад к автоматизациям",
    description: "Accessible label of the back link to the automations list",
  },
  "agentAutomations.backToAutomation": {
    message: "Назад к автоматизации",
    description: "Button returning from the edit page to the automation's page",
  },
  "agentAutomations.tabMine": {
    message: "Мои",
    description: "List tab: automations the viewer created",
  },
  "agentAutomations.tabAll": {
    message: "Все",
    description: "List tab: every automation in the workspace",
  },
  "agentAutomations.emptyMineTitle": {
    message: "У вас пока нет автоматизаций",
    description: "Empty state title on the Mine tab",
  },
  "agentAutomations.emptyAllTitle": {
    message: "В этом рабочем пространстве пока нет автоматизаций",
    description: "Empty state title on the All tab",
  },
  "agentAutomations.emptyBody": {
    message:
      "Автоматизация запускает сессию агента по расписанию, например каждый будний день в 09:00, с выбранными вами задачей, репозиторием и лимитами.",
    description: "Empty state explanation",
  },
  "agentAutomations.loadMore": {
    message: "Показать ещё",
    description: "Load the next page of automations or runs",
  },
  "agentAutomations.loadFailed": {
    message: "Не удалось загрузить автоматизации.",
    description: "Load error",
  },
  "agentAutomations.retry": {
    message: "Повторить",
    description: "Retry button on a load error",
  },
  "agentAutomations.forbidden": {
    message:
      "Использовать автоматизации могут только участники с ролью разработчика или выше.",
    description: "Load error for a member without access",
  },
  "agentAutomations.notFound": {
    message: "Эта автоматизация не существует или была удалена.",
    description: "Load error for a missing automation",
  },
  "agentAutomations.noTrigger": {
    message: "Нет расписания",
    description: "Row text when an automation has no schedule",
  },
  "agentAutomations.nextRun": {
    message: "Следующий запуск",
    description: "Label before the next run time",
  },
  "agentAutomations.lastRun": {
    message: "Последний запуск",
    description: "Label before the last run time",
  },
  "agentAutomations.neverRun": {
    message: "никогда",
    description: "Last run value when it has not run",
  },
  "agentAutomations.lastSession": {
    message: "Открыть сессию",
    description: "Link to the session the last run started",
  },
  "agentAutomations.pauseLabel": {
    message: "Приостановить {name}",
    description: "Accessible name of the switch that pauses an automation",
  },
  "agentAutomations.resumeLabel": {
    message: "Возобновить {name}",
    description: "Accessible name of the switch that resumes an automation",
  },
  "agentAutomations.pausedBadge": {
    message: "Приостановлена",
    description: "Badge on an automation a person paused",
  },
  "agentAutomations.ended": {
    message: "Завершена",
    description: "Badge on an automation with no future run",
  },
  "agentAutomations.active": {
    message: "Активна",
    description: "Status of a running automation",
  },
  "agentAutomations.paused.repeated_failures": {
    message: "Приостановлена: повторяющиеся сбои",
    description: "Badge: paused by the platform after repeated failed runs",
  },
  "agentAutomations.paused.creator_lost_access": {
    message: "Приостановлена: создатель потерял доступ",
    description:
      "Badge: paused because the creator can no longer start sessions",
  },
  "agentAutomations.paused.template_invalid": {
    message: "Приостановлена: шаблон недопустим",
    description:
      "Badge: paused because the session template no longer validates",
  },
  "agentAutomations.paused.unknown": {
    message: "Приостановлена платформой",
    description: "Badge for a pause reason this version does not know",
  },
  "agentAutomations.pausedExplain.repeated_failures": {
    message:
      "Эта автоматизация приостановлена после {failures} неудачных запусков подряд. Проверьте историю запусков, устраните причину и возобновите её.",
    description: "Explanation on the detail page for a repeated-failures pause",
  },
  "agentAutomations.pausedExplain.creator_lost_access": {
    message:
      "У участника, создавшего эту автоматизацию, больше нет доступа к запуску сессий. Администратор рабочего пространства может перехватить её, после чего она возобновится.",
    description:
      "Explanation on the detail page for a creator-lost-access pause",
  },
  "agentAutomations.pausedExplain.template_invalid": {
    message:
      "Шаблон сессии больше не проходит проверку, например из-за изменения секрета или настройки модели. Его создатель может отредактировать шаблон и затем возобновить автоматизацию.",
    description: "Explanation on the detail page for a template-invalid pause",
  },
  "agentAutomations.pausedExplain.unknown": {
    message:
      "Платформа приостановила эту автоматизацию. Возобновите её, когда причина будет устранена.",
    description: "Explanation for a pause reason this version does not know",
  },
  "agentAutomations.pausedToast": {
    message: "{name} приостановлена.",
    description: "Toast after pausing an automation",
  },
  "agentAutomations.resumedToast": {
    message: "{name} возобновлена.",
    description: "Toast after resuming an automation",
  },
  "agentAutomations.runStartedToast": {
    message: "{name} запустила сессию.",
    description: "Toast after Run now started a session",
  },
  "agentAutomations.runSettledToast": {
    message: "Запуск {name}: {outcome}.",
    description: "Toast after Run now ended with an outcome other than started",
  },
  "agentAutomations.tookOverToast": {
    message: "Теперь {name} принадлежит вам.",
    description: "Toast after an admin takes an automation over",
  },
  "agentAutomations.deletedToast": {
    message: "{name} удалена.",
    description: "Toast after deleting an automation",
  },
  "agentAutomations.createdToast": {
    message: "{name} создана.",
    description: "Toast after creating an automation",
  },
  "agentAutomations.updatedToast": {
    message: "{name} сохранена.",
    description: "Toast after editing an automation",
  },
  "agentAutomations.actionFailed": {
    message: "Не получилось. Повторите попытку.",
    description: "Generic failure toast for an automation action",
  },
  "agentAutomations.saveFailed": {
    message: "Не удалось сохранить автоматизацию. Повторите попытку.",
    description: "Generic failure message when saving the form",
  },
  "agentAutomations.actionsLabel": {
    message: "Действия с автоматизацией",
    description: "Accessible label of the action button group",
  },
  "agentAutomations.runNow": {
    message: "Запустить сейчас",
    description: "Button: fire the automation now",
  },
  "agentAutomations.runNowPausedHint": {
    message: "Возобновите эту автоматизацию, прежде чем запускать её.",
    description:
      "Tooltip on Run now while the automation is paused by the platform",
  },
  "agentAutomations.pause": {
    message: "Приостановить",
    description: "Button: pause the automation",
  },
  "agentAutomations.resume": {
    message: "Возобновить",
    description: "Button: resume the automation",
  },
  "agentAutomations.edit": {
    message: "Редактировать",
    description: "Button: edit the automation",
  },
  "agentAutomations.editCreatorOnly": {
    message:
      "Редактировать эту автоматизацию может только создавший её участник.",
    description: "Tooltip and notice: editing is for the creator only",
  },
  "agentAutomations.takeOver": {
    message: "Перехватить",
    description: "Button: an admin becomes the automation's creator",
  },
  "agentAutomations.takeOverAdminOnly": {
    message:
      "Перехватить эту автоматизацию может только администратор рабочего пространства, не создававший её.",
    description: "Tooltip on a disabled Take over button",
  },
  "agentAutomations.takeOverTitle": {
    message: "Перехватить {name}?",
    description: "Take over confirmation title",
  },
  "agentAutomations.takeOverBody": {
    message:
      "Следующие запуски будут создавать сессии от вашего имени и с вашим доступом, а шаблон будет проверен заново с учётом вашего доступа. Автоматизация, приостановленная из-за потери доступа создателем, возобновится.",
    description: "Take over confirmation body",
  },
  "agentAutomations.delete": {
    message: "Удалить",
    description: "Button: delete the automation",
  },
  "agentAutomations.deleteTitle": {
    message: "Удалить {name}?",
    description: "Delete confirmation title",
  },
  "agentAutomations.deleteBody": {
    message:
      "Автоматизация и история её запусков будут удалены. Уже запущенные ею сессии сохранятся.",
    description: "Delete confirmation body",
  },
  "agentAutomations.settingsTitle": {
    message: "Настройки",
    description: "Settings card title on the detail page",
  },
  "agentAutomations.row.status": {
    message: "Статус",
    description: "Settings row label",
  },
  "agentAutomations.row.schedule": {
    message: "Расписание",
    description: "Settings row label",
  },
  "agentAutomations.row.nextRun": {
    message: "Следующий запуск",
    description: "Settings row label",
  },
  "agentAutomations.row.lastRun": {
    message: "Последний запуск",
    description: "Settings row label",
  },
  "agentAutomations.row.createdBy": {
    message: "Создал(а)",
    description: "Settings row label",
  },
  "agentAutomations.row.repo": {
    message: "Репозиторий",
    description: "Settings row label",
  },
  "agentAutomations.row.agent": {
    message: "Агент",
    description: "Settings row label",
  },
  "agentAutomations.row.spendLimit": {
    message: "Лимит расходов на запуск",
    description: "Settings row label",
  },
  "agentAutomations.row.emailNotifications": {
    message: "Уведомления по эл. почте",
    description: "Settings row label",
  },
  "agentAutomations.row.secrets": {
    message: "Секреты",
    description: "Settings row label",
  },
  "agentAutomations.row.skills": {
    message: "Навыки",
    description: "Settings row label",
  },
  "agentAutomations.taskTitle": {
    message: "Задача",
    description: "Task card title on the detail page",
  },
  "agentAutomations.noRepo": {
    message: "Нет (сессия только в чате)",
    description: "Repository value when the template has none",
  },
  "agentAutomations.noSpendLimit": {
    message: "Без лимита",
    description: "Spend limit value when the template has none",
  },
  "agentAutomations.none": {
    message: "Нет",
    description: "Empty value for secrets or skills",
  },
  "agentAutomations.runsTitle": {
    message: "История запусков",
    description: "Run history card title",
  },
  "agentAutomations.runsDescription": {
    message: "Все запуски по расписанию и вручную, сначала новые.",
    description: "Run history card description",
  },
  "agentAutomations.runsEmpty": {
    message: "Запусков пока нет.",
    description: "Run history empty state",
  },
  "agentAutomations.runs.fireAt": {
    message: "Время запуска",
    description: "Run table column",
  },
  "agentAutomations.runs.outcome": {
    message: "Результат",
    description: "Run table column",
  },
  "agentAutomations.runs.session": {
    message: "Сессия",
    description: "Run table column",
  },
  "agentAutomations.runs.missed": {
    message: "Пропущено",
    description: "Run table column: ticks skipped while the scheduler was down",
  },
  "agentAutomations.runs.error": {
    message: "Ошибка",
    description: "Run table column",
  },
  "agentAutomations.runs.openSession": {
    message: "Открыть сессию",
    description: "Link to the session a run started",
  },
  "agentAutomations.outcome.started": {
    message: "Запущена",
    description: "Run outcome: a session was started",
  },
  "agentAutomations.outcome.claimed": {
    message: "Выполняется",
    description: "Run outcome: claimed, not yet settled",
  },
  "agentAutomations.outcome.skipped_overlap": {
    message: "Пропущена",
    description:
      "Run outcome: skipped because the previous session was still running",
  },
  "agentAutomations.outcome.failed_create": {
    message: "Сбой",
    description: "Run outcome: the session could not be created",
  },
  "agentAutomations.outcome.denied": {
    message: "Отклонена",
    description:
      "Run outcome: the creator was no longer allowed to start the session",
  },
  "agentAutomations.outcome.didNotStart": {
    message: "Не запустилась",
    description:
      "Run outcome: the run started a session whose sandbox did not start for a cause on bex's side (w10/m497); the cause shows in the Error column. Not a failed create",
  },
  "agentAutomations.outcome.unknown": {
    message: "Неизвестно",
    description: "Run outcome this version does not know",
  },
  "agentAutomations.section.details": {
    message: "Подробности",
    description: "Form section title",
  },
  "agentAutomations.section.schedule": {
    message: "Расписание",
    description: "Form section title",
  },
  "agentAutomations.section.scheduleDescription": {
    message:
      "Когда запускается автоматизация. Часовой пояс определяет, что означает 09:00.",
    description: "Form section description",
  },
  "agentAutomations.section.session": {
    message: "Сессия",
    description: "Form section title",
  },
  "agentAutomations.section.sessionDescription": {
    message:
      "Сессия агента, которую запускает каждый запуск, с теми же параметрами, что и у новой сессии на странице «Агенты».",
    description: "Form section description",
  },
  "agentAutomations.field.name": {
    message: "Название",
    description: "Form field label",
  },
  "agentAutomations.field.frequency": {
    message: "Повтор",
    description: "Form field label",
  },
  "agentAutomations.field.minute": {
    message: "Минута часа",
    description: "Form field label",
  },
  "agentAutomations.field.at": {
    message: "Время",
    description: "Form field label",
  },
  "agentAutomations.field.weekdays": {
    message: "Дни",
    description: "Form field label",
  },
  "agentAutomations.field.runAt": {
    message: "Дата и время",
    description: "Form field label",
  },
  "agentAutomations.field.cron": {
    message: "Выражение cron",
    description: "Form field label",
  },
  "agentAutomations.field.timezone": {
    message: "Часовой пояс",
    description: "Form field label",
  },
  "agentAutomations.field.endsOn": {
    message: "Завершить (необязательно)",
    description: "Form field label",
  },
  "agentAutomations.field.task": {
    message: "Задача",
    description: "Form field label",
  },
  "agentAutomations.field.repo": {
    message: "Репозиторий",
    description: "Form field label",
  },
  "agentAutomations.field.agent": {
    message: "Агент",
    description: "Form field label",
  },
  "agentAutomations.field.model": {
    message: "Модель",
    description: "Automation form — model field label (w10/m516)",
  },
  "agentAutomations.field.template": {
    message: "Песочница",
    description: "Automation form — sandbox template field label (w10/m519)",
  },
  "agentAutomations.field.spendLimit": {
    message: "Лимит расходов на запуск (USD)",
    description: "Form field label",
  },
  "agentAutomations.field.emailNotifications": {
    message: "Уведомления по эл. почте",
    description: "Form field label",
  },
  "agentAutomations.field.secrets": {
    message: "Секреты",
    description: "Form field label",
  },
  "agentAutomations.field.skills": {
    message: "Навыки",
    description: "Form field label",
  },
  "agentAutomations.frequency.hour": {
    message: "Каждый час",
    description: "Frequency option",
  },
  "agentAutomations.frequency.day": {
    message: "Каждый день",
    description: "Frequency option",
  },
  "agentAutomations.frequency.week": {
    message: "Каждую неделю",
    description: "Frequency option",
  },
  "agentAutomations.frequency.once": {
    message: "Один раз",
    description: "Frequency option",
  },
  "agentAutomations.frequency.custom": {
    message: "Своё (cron)",
    description: "Frequency option",
  },
  "agentAutomations.emailNotifications.always": {
    message: "Всегда",
    description: "Email notification option: every run emails the creator",
  },
  "agentAutomations.emailNotifications.on_failure": {
    message: "При сбое",
    description:
      "Email notification option: only failed runs email the creator",
  },
  "agentAutomations.emailNotifications.on_success": {
    message: "При успехе",
    description:
      "Email notification option: only succeeded runs email the creator",
  },
  "agentAutomations.emailNotifications.never": {
    message: "Никогда",
    description: "Email notification option: no run emails the creator",
  },
  "agentAutomations.weekday.mon": {
    message: "Пн",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.tue": {
    message: "Вт",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.wed": {
    message: "Ср",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.thu": {
    message: "Чт",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.fri": {
    message: "Пт",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.sat": {
    message: "Сб",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.sun": {
    message: "Вс",
    description: "Weekday toggle",
  },
  "agentAutomations.help.name": {
    message: "Строчные буквы, цифры и одиночные дефисы, до 64 символов.",
    description: "Name field help",
  },
  "agentAutomations.help.minute": {
    message: "От 0 до 59.",
    description: "Minute field help",
  },
  "agentAutomations.help.runAt": {
    message: "Учитывается часовой пояс, выбранный ниже ({zone}).",
    description: "Run-at field help",
  },
  "agentAutomations.help.cron": {
    message:
      "Пять полей: минута час день-месяца месяц день-недели. Интервал между запусками не может быть меньше 5 минут.",
    description: "Cron field help",
  },
  "agentAutomations.help.endsOn": {
    message:
      "Последний день, в который возможен запуск. Оставьте пустым, чтобы запускать, пока вы не приостановите автоматизацию.",
    description: "Ends-on field help",
  },
  "agentAutomations.help.task": {
    message:
      "Что агент должен делать при каждом запуске. Укажите ожидаемый результат.",
    description: "Task field help",
  },
  "agentAutomations.help.repo": {
    message:
      "Необязательно. Без репозитория каждый запуск — сессия только в чате. Каждый запуск работает в новой ветке bex-agent.",
    description: "Repository field help",
  },
  "agentAutomations.help.spendLimit": {
    message:
      "Верхний предел расходов на модель за каждый запуск. Обязателен, если запуски используют модель платформы.",
    description: "Spend limit field help",
  },
  "agentAutomations.help.model": {
    message:
      "Модель для каждого запуска. Оставьте значение по умолчанию, чтобы следовать настройке рабочего пространства.",
    description: "Automation form — model field help",
  },
  "agentAutomations.help.modelUnoffered": {
    message:
      "Модель {model} больше не доступна в этом рабочем пространстве: администратор её отключил или она исчезла из каталога платформы. Вместо неё запуски используют модель рабочего пространства по умолчанию {fallback}. Чтобы сохранить изменения, выберите доступную модель.",
    description:
      "Automation form — model field note when the saved model is no longer offered (turned off by an admin or gone from the catalog); {fallback} is the workspace default its runs use (w10/m527)",
  },
  "agentAutomations.help.template": {
    message:
      "Машина, которую получает песочница каждого запуска. Значение по умолчанию для рабочего пространства берётся из «Настройки → Агенты» при каждом запуске.",
    description: "Automation form — sandbox template field help (w10/m519)",
  },
  "agentAutomations.help.githubCli": {
    message:
      "Получают ли её сессии gh. «Наследовать» следует настройке «GitHub CLI для автоматизаций» в «Настройки → Агенты».",
    description: "Automation form — GitHub CLI field help (w10/m561)",
  },
  "agentAutomations.help.emailNotifications": {
    message:
      "О каких запусках вам приходят письма. О приостановке платформой — всегда.",
    description: "Email notifications field help",
  },
  "agentAutomations.help.secrets": {
    message:
      "Секреты рабочего пространства, которые агент получает как переменные окружения. Добавьте их в настройках рабочего пространства.",
    description: "Secrets field help",
  },
  "agentAutomations.noSecrets": {
    message: "Нет секретов",
    description: "Secrets row when none are chosen",
  },
  "agentAutomations.noSkills": {
    message: "Нет навыков",
    description: "Skills row when none are chosen",
  },
  "agentAutomations.placeholder.name": {
    message: "nightly-dependency-check",
    description: "Name field placeholder (an example slug)",
  },
  "agentAutomations.placeholder.repo": {
    message: "owner/repository",
    description: "Repository field placeholder",
  },
  "agentAutomations.placeholder.model": {
    message: "По умолчанию для рабочего пространства",
    description: "Automation form — empty free-form model field",
  },
  "agentAutomations.placeholder.template": {
    message: "По умолчанию для рабочего пространства",
    description:
      "Automation form — the sandbox field while no template list is available: the workspace default",
  },
  "agentAutomations.summary": {
    message: "Запуски: {schedule}",
    description: "Live summary under the schedule picker",
  },
  "agentAutomations.schedule.hour": {
    message: "Каждый час в :{minute}",
    description: "Schedule in words, hourly",
  },
  "agentAutomations.schedule.day": {
    message: "Каждый день в {at}",
    description: "Schedule in words, daily",
  },
  "agentAutomations.schedule.week": {
    message: "Каждую неделю ({days}) в {at}",
    description: "Schedule in words, weekly",
  },
  "agentAutomations.schedule.once": {
    message: "Один раз: {when}",
    description: "Schedule in words, one-off",
  },
  "agentAutomations.schedule.custom": {
    message: "Cron {cron}",
    description: "Schedule in words, cron",
  },
  "agentAutomations.schedule.until": {
    message: "до {date}",
    description: "Schedule end date suffix",
  },
  "agentAutomations.problem.nameRequired": {
    message: "Введите название.",
    description: "Form validation",
  },
  "agentAutomations.problem.nameInvalid": {
    message:
      "Используйте строчные буквы, цифры и одиночные дефисы, до 64 символов.",
    description: "Form validation",
  },
  "agentAutomations.problem.taskRequired": {
    message: "Опишите задачу.",
    description: "Form validation",
  },
  "agentAutomations.problem.spendLimit": {
    message:
      "Введите сумму больше 0 и не более 100000 или оставьте поле пустым.",
    description: "Form validation",
  },
  "agentAutomations.problem.skills": {
    message: "Аргументы навыка должны быть одной строкой не длиннее 4 КБ.",
    description: "Form validation",
  },
  "agentAutomations.problem.githubCliMandatory": {
    message:
      "В этом рабочем пространстве выключение GitHub CLI обязательно, поэтому автоматизация не может включить gh. Выберите «Выкл.» или «Наследовать».",
    description:
      "Automation form — AGENT_AUTOMATION_GITHUB_CLI_LOCKED with reason mandatory (w10/m561)",
  },
  "agentAutomations.problem.githubCliAdminRequired": {
    message:
      "Выбрать или изменить GitHub CLI для автоматизации может только администратор рабочего пространства. Оставьте прежний выбор.",
    description:
      "Automation form — AGENT_AUTOMATION_GITHUB_CLI_LOCKED with reason admin_required: only a workspace admin sets or changes an automation's gh binding (w10/044)",
  },
  "agentAutomations.problem.timezone": {
    message: "Выберите допустимый часовой пояс.",
    description: "Form validation",
  },
  "agentAutomations.problem.minute": {
    message: "Введите минуту от 0 до 59.",
    description: "Form validation",
  },
  "agentAutomations.problem.at": {
    message: "Введите время.",
    description: "Form validation",
  },
  "agentAutomations.problem.weekdays": {
    message: "Выберите хотя бы один день.",
    description: "Form validation",
  },
  "agentAutomations.problem.runAtRequired": {
    message: "Выберите дату и время.",
    description: "Form validation",
  },
  "agentAutomations.problem.runAtPast": {
    message: "Выберите время в будущем.",
    description: "Form validation",
  },
  "agentAutomations.problem.cron": {
    message: "Введите пять полей: минута час день-месяца месяц день-недели.",
    description: "Form validation",
  },
  "agentAutomations.problem.endsOn": {
    message: "Введите допустимую дату.",
    description: "Form validation",
  },
  "agentAutomations.problem.endsOnPast": {
    message: "Выберите дату в будущем.",
    description: "Form validation",
  },
  "agentAutomations.startedBy": {
    message: "Запущена автоматизацией",
    description: "Session page header note linking back to the automation",
  },
};

export default ruAgentAutomations;
