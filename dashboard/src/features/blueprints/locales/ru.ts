import type { TranslationEntry } from "@/i18n";

const ruBlueprints: Record<string, TranslationEntry> = {
  "blueprints.resourceType": {
    message: "Blueprint",
    description: "Blueprint resource type used in document titles",
  },
  "blueprints.pageTitle": {
    message: "Blueprints",
    description: "Blueprints list page heading and document title",
  },
  "blueprints.cardTitle": {
    message: "Blueprints",
    description: "Blueprints table card title",
  },
  "blueprints.colName": {
    message: "Название",
    description:
      "Blueprints table column header — blueprint name (derived from repo)",
  },
  "blueprints.colRepo": {
    message: "Репозиторий",
    description: "Blueprints table column header — source repo URL",
  },
  "blueprints.colBranch": {
    message: "Ветка",
    description: "Blueprints table column header — tracked branch",
  },
  "blueprints.colStatus": {
    message: "Статус",
    description: "Blueprints table column header",
  },
  "blueprints.colUpdated": {
    message: "Последнее обновление",
    description: "Blueprints table column header — updatedAt relative age",
  },
  "blueprints.createButton": {
    message: "Новый экземпляр Blueprint",
    description: "Button label that opens the New Blueprint dialog",
  },
  "blueprints.createTitle": {
    message: "Новый экземпляр Blueprint",
    description: "New Blueprint page title",
  },
  "blueprints.createDescription": {
    message:
      "Подключите репозиторий с Blueprint-файлом render.yaml, чтобы создать и синхронизировать целый набор сервисов.",
    description: "New Blueprint page subtitle under the title",
  },
  "blueprints.createSourceTitle": {
    message: "Подключите репозиторий",
    description:
      "New Blueprint page — heading above the GitHub/Public Git source tabs (Render parity)",
  },
  "blueprints.createBranchLabel": {
    message: "Ветка",
    description: "New Blueprint page — branch combobox label",
  },
  "blueprints.createBranchPlaceholder": {
    message: "main",
    description: "New Blueprint page — branch combobox placeholder",
  },
  "blueprints.createBranchHint": {
    message: "Ветка репозитория с файлом render.yaml.",
    description: "New Blueprint page — branch field helper text",
  },
  "blueprints.createPathLabel": {
    message: "Путь к Blueprint",
    description: "New Blueprint page — render.yaml path input label",
  },
  "blueprints.createNamePlaceholder": {
    message: "my-stack",
    description: "New Blueprint page — blueprint name placeholder",
  },
  "blueprints.createPathPlaceholder": {
    message: "render.yaml",
    description: "New Blueprint page — manifest path placeholder",
  },
  "blueprints.createPathHint": {
    message:
      "Путь к файлу Blueprint в вашем репозитории — подойдёт любой файл .yaml/.yml (напр. infra/bex/stack.yaml). По умолчанию — render.yaml в корне репозитория.",
    description: "New Blueprint page — manifest path helper text",
  },
  "blueprints.createNameLabel": {
    message: "Название Blueprint",
    description: "New Blueprint page — name input label",
  },
  "blueprints.createNameTaken": {
    message:
      "Blueprint с названием {name} уже существует в этом рабочем пространстве. Выберите другое название.",
    description:
      "New Blueprint page — shown under the name field when Deploy was refused because an approved Blueprint already has the name (w8/250)",
  },
  "blueprints.createAction": {
    message: "Развернуть Blueprint",
    description: "New Blueprint dialog submit button label",
  },
  "blueprints.createCancel": {
    message: "Отменить",
    description: "New Blueprint dialog cancel button label",
  },
  "blueprints.createSuccess": {
    message: "Blueprint создан",
    description: "Toast shown after a successful createBlueprint call",
  },
  "blueprints.createError": {
    message: "Не удалось создать Blueprint",
    description: "Toast shown when createBlueprint returns an error",
  },
  "blueprints.previewTitle": {
    message: "Проверка конфигурации Blueprint",
    description:
      "New Blueprint page — heading of the pre-create fetch + validate section",
  },
  "blueprints.previewSelectSource": {
    message:
      "Выберите репозиторий и ветку, чтобы проверить файл Blueprint перед развёртыванием.",
    description: "Review section placeholder before a source is chosen",
  },
  "blueprints.previewLoading": {
    message: "Получение {path} из вашего репозитория…",
    description: "Review section loading text while the manifest is fetched",
  },
  "blueprints.previewNotFoundTitle": {
    message: "Файл Blueprint не найден",
    description: "Review section error title when the manifest fetch fails",
  },
  "blueprints.previewNotFoundBody": {
    message: "Файл Blueprint {path} не найден в ветке {branch}.",
    description:
      "Review section fallback error body when the backend returns no message",
  },
  "blueprints.previewFileNotFoundTitle": {
    message: "Файл Blueprint не найден",
    description:
      "Preview failure title: the repo and branch resolved, the file did not",
  },
  "blueprints.previewFileNotFoundBody": {
    message:
      "{path} не существует в ветке {branch}. Проверьте путь или закоммитьте файл в эту ветку.",
    description: "Preview failure body for file_not_found",
  },
  "blueprints.previewBranchNotFoundTitle": {
    message: "Ветка не найдена",
    description: "Preview failure title for branch_not_found",
  },
  "blueprints.previewBranchNotFoundBody": {
    message: "Ветка {branch} не существует в этом репозитории.",
    description: "Preview failure body for branch_not_found",
  },
  "blueprints.previewRepoNotFoundTitle": {
    message: "Репозиторий не найден",
    description: "Preview failure title for repo_not_found_or_no_access",
  },
  "blueprints.previewRepoNotFoundBody": {
    message:
      "Этот репозиторий не существует, или у GitHub App bex нет к нему доступа. Проверьте репозиторий и доступ приложения к репозиториям.",
    description: "Preview failure body for repo_not_found_or_no_access",
  },
  "blueprints.previewAccessDeniedTitle": {
    message: "Доступ запрещён",
    description: "Preview failure title for access_denied",
  },
  "blueprints.previewAccessDeniedBody": {
    message:
      "GitHub App bex не имеет права читать этот репозиторий. Проверьте установку и репозитории, к которым предоставлен доступ.",
    description: "Preview failure body for access_denied",
  },
  "blueprints.previewPublicRepoNotFoundBody": {
    message:
      "Этот репозиторий не существует на {host} или не является публичным. bex читает Blueprint оттуда только из публичного репозитория.",
    description:
      "Preview failure body for repo_not_found_or_no_access on gitlab.com or bitbucket.org, read anonymously; {host} is that host",
  },
  "blueprints.previewPublicAccessDeniedBody": {
    message:
      "{host} не разрешил bex прочитать этот репозиторий. bex читает Blueprint оттуда только из публичного репозитория.",
    description:
      "Preview failure body for access_denied on gitlab.com or bitbucket.org, read anonymously; {host} is that host",
  },
  "blueprints.previewHostUnsupportedTitle": {
    message: "Git-хостинг не поддерживается",
    description:
      "Preview failure title for a repository on a Git host bex reads no Blueprint from",
  },
  "blueprints.previewHostUnsupportedBody": {
    message:
      "Как и Render, bex читает Blueprint из публичного репозитория на github.com, gitlab.com или bitbucket.org. {host} к ним не относится.",
    description:
      "Preview failure body for a repository on a Git host bex reads no Blueprint from; {host} is that host",
  },
  "blueprints.previewRateLimitedTitle": {
    message: "{host} ограничивает запросы от bex",
    description:
      "Preview failure title for rate_limited; {host} is GitHub or the repository's host",
  },
  "blueprints.previewRateLimitedBody": {
    message:
      "Сейчас слишком много запросов к {host}. Повторите попытку через несколько минут.",
    description:
      "Preview failure body for rate_limited; {host} is GitHub or the repository's host",
  },
  "blueprints.previewAmbiguousTitle": {
    message: "Два файла Blueprint",
    description: "Preview failure title for ambiguous_filename",
  },
  "blueprints.previewAmbiguousBody": {
    message:
      "В этом репозитории в ветке {branch} есть и render.yaml, и bex.yml. Удалите один из них или явно укажите файл выше.",
    description: "Preview failure body for ambiguous_filename",
  },
  "blueprints.previewUnavailableTitle": {
    message: "Не удалось связаться с {host}",
    description:
      "Preview failure title for unavailable; {host} is GitHub or the repository's host",
  },
  "blueprints.previewUnavailableBody": {
    message:
      "bex не удалось прочитать файл Blueprint из {host}. Повторите попытку через несколько минут.",
    description:
      "Preview failure body for unavailable; {host} is GitHub or the repository's host",
  },
  "blueprints.previewInvalidPathExtension": {
    message: "Путь к Blueprint должен указывать на файл .yaml или .yml.",
    description: "Inline path-field error for invalid_path, wrong extension",
  },
  "blueprints.previewInvalidPathShape": {
    message:
      "Путь к Blueprint должен быть чистым путём относительно репозитория (без начального слеша и без ..).",
    description: "Inline path-field error for invalid_path, bad shape",
  },
  "blueprints.previewRetry": {
    message: "Повторить",
    description: "Review section retry button after a failed manifest fetch",
  },
  "blueprints.takeoverResourceTitle": {
    message: "Перехватить «{name}»?",
    description:
      "Takeover dialog title when a resource is managed by another Blueprint (BLUEPRINT_RESOURCE_CONFLICT)",
  },
  "blueprints.takeoverResourceBody": {
    message:
      "{kind} «{name}» управляется Blueprint {owner}. Если продолжить, он будет передан этому Blueprint. Введите команду ниже, чтобы продолжить.",
    description: "Takeover dialog body for a Blueprint resource conflict",
  },
  "blueprints.takeoverResourceBodyOtherOwner": {
    message:
      "{kind} «{name}» управляется другим Blueprint. Если продолжить, он будет передан этому Blueprint. Введите команду ниже, чтобы продолжить.",
    description:
      "Takeover dialog body for a Blueprint resource conflict whose owning Blueprint bex-api could not name",
  },
  "blueprints.previewConflictBody": {
    message:
      "Некоторыми из этих ресурсов уже управляет другой Blueprint. Развёртывание передаст их этому Blueprint; на следующем шаге вы подтвердите, что именно будет перехвачено.",
    description:
      "Preview alert body on /blueprints/new when the only problem is a takeover conflict (replaces the raw API message)",
  },
  "blueprints.previewConflict": {
    message: "Уже управляется — развёртывание перехватит его",
    description:
      "Alert title on /blueprints/new when the only preview problem is a resource another Blueprint manages, which an explicit takeover confirmation resolves (w4/m125)",
  },
  "blueprints.previewMatchTitle": {
    message: "Совпадает с существующими ресурсами",
    description:
      "Notice title on /blueprints/new when the Blueprint file names resources that already exist in the workspace (w8/m153, Render parity)",
  },
  "blueprints.previewMatchBody": {
    message:
      "Этот Blueprint совпадает с ресурсами, которые уже есть в вашем рабочем пространстве. Новые ресурсы будут созданы с суффиксом «-{suffix}» в названии. Существующие ресурсы не изменятся.",
    description:
      "Notice body on /blueprints/new for matched resources; {suffix} is the four-character resource name suffix, shown after a hyphen as it appears in a name",
  },
  "blueprints.previewMatchListLabel": {
    message: "Уже есть в вашем рабочем пространстве:",
    description:
      "Notice on /blueprints/new — label above the list of existing resources the Blueprint file matches",
  },
  "blueprints.previewInvalid": {
    message: "В файле Blueprint есть ошибки",
    description:
      "Review section error title when the fetched manifest fails validation",
  },
  "blueprints.previewValid_one": {
    message: "Файл Blueprint успешно разобран — изменится {count} ресурс.",
    description:
      "Review section success line, with the plan's total resource count (singular)",
  },
  "blueprints.previewValid_few": {
    message: "Файл Blueprint успешно разобран — изменятся {count} ресурса.",
    description:
      "Review section success line, with the plan's total resource count (plural)",
  },
  "blueprints.previewValid_many": {
    message: "Файл Blueprint успешно разобран — изменятся {count} ресурсов.",
    description:
      "Review section success line, with the plan's total resource count (plural)",
  },
  "blueprints.previewValid_other": {
    message: "Файл Blueprint успешно разобран — изменятся {count} ресурса.",
    description:
      "Review section success line, with the plan's total resource count (plural)",
  },
  "blueprints.previewValidEnvironments_one": {
    message: "Файл Blueprint успешно разобран — изменится {count} окружение.",
    description:
      "Review section success line when only environments change (w9/095, singular)",
  },
  "blueprints.previewValidEnvironments_few": {
    message: "Файл Blueprint успешно разобран — изменятся {count} окружения.",
    description:
      "Review section success line when only environments change (w9/095, plural)",
  },
  "blueprints.previewValidEnvironments_many": {
    message: "Файл Blueprint успешно разобран — изменятся {count} окружений.",
    description:
      "Review section success line when only environments change (w9/095, plural)",
  },
  "blueprints.previewValidEnvironments_other": {
    message: "Файл Blueprint успешно разобран — изменятся {count} окружения.",
    description:
      "Review section success line when only environments change (w9/095, plural)",
  },
  "blueprints.previewValidWithEnvironments_one": {
    message:
      "Файл Blueprint успешно разобран — будут затронуты: {count} ресурс и {environments}.",
    description:
      "Review section success line: changing resources (singular) and blueprints.previewEnvironmentCount (w9/095)",
  },
  "blueprints.previewValidWithEnvironments_few": {
    message:
      "Файл Blueprint успешно разобран — будут затронуты: {count} ресурса и {environments}.",
    description:
      "Review section success line: changing resources (plural) and blueprints.previewEnvironmentCount (w9/095)",
  },
  "blueprints.previewValidWithEnvironments_many": {
    message:
      "Файл Blueprint успешно разобран — будут затронуты: {count} ресурсов и {environments}.",
    description:
      "Review section success line: changing resources (plural) and blueprints.previewEnvironmentCount (w9/095)",
  },
  "blueprints.previewValidWithEnvironments_other": {
    message:
      "Файл Blueprint успешно разобран — будут затронуты: {count} ресурса и {environments}.",
    description:
      "Review section success line: changing resources (plural) and blueprints.previewEnvironmentCount (w9/095)",
  },
  "blueprints.previewEnvironmentCount_one": {
    message: "{count} окружение",
    description:
      "The changing environments inside blueprints.previewValidWithEnvironments (singular)",
  },
  "blueprints.previewEnvironmentCount_few": {
    message: "{count} окружения",
    description:
      "The changing environments inside blueprints.previewValidWithEnvironments (plural)",
  },
  "blueprints.previewEnvironmentCount_many": {
    message: "{count} окружений",
    description:
      "The changing environments inside blueprints.previewValidWithEnvironments (plural)",
  },
  "blueprints.previewEnvironmentCount_other": {
    message: "{count} окружения",
    description:
      "The changing environments inside blueprints.previewValidWithEnvironments (plural)",
  },
  "blueprints.errorAtLineColumn": {
    message: "строка {line}, столбец {column}",
    description: "Source location beside a Blueprint validation error (w4/151)",
  },
  "blueprints.errorAtLine": {
    message: "строка {line}",
    description:
      "Source location beside a Blueprint validation error when only the line is known (w4/151)",
  },
  "blueprints.previewNoChanges": {
    message:
      "Файл Blueprint успешно разобран — изменений нет. Все ресурсы уже соответствуют этому файлу.",
    description:
      "Review section success line when every planned action is a no-op (w4/m138)",
  },
  "blueprints.previewOpCreate": {
    message: "Создать",
    description:
      "Plan row badge: the resource does not exist yet and will be created",
  },
  "blueprints.previewOpUpdate": {
    message: "Обновить",
    description: "Plan row badge: an existing resource will be changed",
  },
  "blueprints.previewOpNoop": {
    message: "Без изменений",
    description: "Plan row badge: the resource already matches the file",
  },
  "blueprints.previewOpError": {
    message: "Нельзя применить",
    description:
      "Plan row badge: the planner refused this resource (reason below)",
  },
  "blueprints.previewChangedFields": {
    message: "Изменения: {fields}",
    description:
      "Plan row detail for an update: the changed field paths (never values)",
  },
  "blueprints.previewKindService": {
    message: "Сервис",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindPostgres": {
    message: "Postgres",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindKeyValue": {
    message: "Key Value",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindEnvGroup": {
    message: "Группа окружения",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindBuildSource": {
    message: "Источник сборки",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindEnvironment": {
    message: "Окружение",
    description: "Plan row resource kind: a projects[].environments[] entry",
  },
  "blueprints.previewServices": {
    message: "Сервисы",
    description: "Review section plan group label — services in the manifest",
  },
  "blueprints.previewDatabases": {
    message: "Базы данных",
    description: "Review section plan group label — databases in the manifest",
  },
  "blueprints.previewKeyValue": {
    message: "Key Value",
    description:
      "Review section plan group label — key-value instances in the manifest",
  },
  "blueprints.previewEnvGroups": {
    message: "Группы окружения",
    description: "Review section plan group label — env groups in the manifest",
  },
  "blueprints.previewManualSyncNote": {
    message:
      "bex не получает push-события этого репозитория, поэтому последующие изменения файла Blueprint не будут синхронизироваться автоматически. Синхронизируйте Blueprint после каждого изменения или подключите репозиторий через GitHub App.",
    description:
      "Review section note for a repo whose pushes can't reach bex (the preview's autoSync is false): the Blueprint is created without auto-sync",
  },
  "blueprints.previewAutoSyncNote": {
    message:
      "Все будущие изменения файла Blueprint в этой ветке будут синхронизироваться автоматически. Автосинхронизацию можно приостановить после создания.",
    description:
      "Review section note about auto-sync being enabled by default (Render parity)",
  },
  "blueprints.previewError": {
    message: "Не удалось загрузить предварительный просмотр Blueprint.",
    description:
      "Review section error shown when the preview query itself fails (network)",
  },
  "blueprints.promptTitle": {
    message: "Значения секретов",
    description:
      "Review section — heading for the sync:false env var prompt inputs",
  },
  "blueprints.promptHint": {
    message:
      "Этот Blueprint объявляет переменные окружения с sync: false — их значения хранятся только в bex, а не в репозитории. Задайте их сейчас; они записываются один раз, и последующие синхронизации их не перезаписывают.",
    description: "sync:false prompt section helper text",
  },
  "blueprints.promptEmptyWarning": {
    message:
      "Пустые значения разворачиваются как незаданные — сервис может не запуститься, пока вы не зададите их на вкладке «Окружение».",
    description:
      "Warning below the sync:false inputs when at least one is blank",
  },
  "blueprints.pricingTitle": {
    message: "Ориентировочная стоимость",
    description: "Review section — estimated-pricing panel heading",
  },
  "blueprints.pricingSubtitle": {
    message:
      "Стоимость вычислений рассчитывается посекундно. Оплата списывается в начале месяца.",
    description:
      "Estimated-pricing panel subtitle explaining billing semantics (Render parity)",
  },
  "blueprints.pricingLineAmount": {
    message: "({tier}) ${amount} / мес.",
    description:
      "Estimated-pricing row amount — plan label + monthly dollar cost",
  },
  "blueprints.pricingLineBreakdown": {
    message: "Инстанс ${instance} + диск ({gb} ГБ) ${storage}",
    description:
      "Estimated-pricing datastore row sub-line — instance vs provisioned-storage cost breakdown",
  },
  "blueprints.pricingVariable": {
    message: "Переменная",
    description:
      "Estimated-pricing row amount for resources whose cost depends on runtime behavior",
  },
  "blueprints.pricingReasonAutoscaling": {
    message: "Автомасштабирование",
    description:
      "Estimated-pricing variable-row badge — service has autoscaling enabled",
  },
  "blueprints.pricingReasonMultiInstance": {
    message: "Несколько инстансов",
    description:
      "Estimated-pricing variable-row badge — service declares numInstances > 1",
  },
  "blueprints.pricingReasonCron": {
    message: "Cron Job",
    description:
      "Estimated-pricing variable-row badge — cron jobs bill only while runs execute",
  },
  "blueprints.pricingTotalLabel": {
    message: "Итого",
    description: "Estimated-pricing panel total row label",
  },
  "blueprints.pricingTotalAmount": {
    message: "${amount}{marker} в месяц",
    description:
      "Estimated-pricing panel total amount; marker is an asterisk when variable costs are excluded",
  },
  "blueprints.pricingExclusions": {
    message: "* Без учёта: {items}.",
    description:
      "Estimated-pricing footnote listing the variable costs excluded from the total",
  },
  "blueprints.pricingExcludeAutoscaling": {
    message: "автомасштабирования",
    description: "Excluded-cost phrase in the estimated-pricing footnote",
  },
  "blueprints.pricingExcludeMultiInstance": {
    message: "дополнительных фиксированных инстансов",
    description: "Excluded-cost phrase in the estimated-pricing footnote",
  },
  "blueprints.pricingExcludeCron": {
    message: "Cron Job",
    description: "Excluded-cost phrase in the estimated-pricing footnote",
  },
  "blueprints.pricingEstimateNote": {
    message:
      "Только оценка — хранилище оценивается по выделенному размеру; в счёте учитывается фактическое использование.",
    description:
      "Estimated-pricing disclaimer: forward estimate is an upper bound of metered billing",
  },
  "blueprints.generateButton": {
    message: "Сгенерировать Blueprint",
    description: "Blueprints list header — opens the export dialog (w8/m22)",
  },
  "blueprints.generateTitle": {
    message: "Генерация Blueprint",
    description: "Generate dialog title",
  },
  "blueprints.generateDescription": {
    message:
      "Выберите существующие ресурсы, чтобы экспортировать их в render.yaml, который можно закоммитить в репозиторий и подключить как Blueprint.",
    description: "Generate dialog description",
  },
  "blueprints.generateEmptyHint": {
    message: "Выберите хотя бы один ресурс для генерации.",
    description: "Generate dialog hint when nothing is selected",
  },
  "blueprints.generateAction": {
    message: "Сгенерировать",
    description: "Generate dialog primary action",
  },
  "blueprints.generateBack": {
    message: "Назад",
    description: "Generate dialog — return from preview to selection",
  },
  "blueprints.generateCopy": {
    message: "Копировать",
    description: "Generate dialog — copy the yaml to the clipboard",
  },
  "blueprints.generateCopied": {
    message: "render.yaml скопирован",
    description: "Toast after copying the generated manifest",
  },
  "blueprints.generateCopyFailed": {
    message:
      "Не удалось скопировать render.yaml. Разрешите доступ к буферу обмена и повторите попытку или скачайте файл.",
    description:
      "Toast when the browser refuses to copy the generated manifest to the clipboard",
  },
  "blueprints.generateDownload": {
    message: "Скачать render.yaml",
    description: "Generate dialog — download the manifest file",
  },
  "blueprints.generateSecretsNote": {
    message:
      "Сохранённые значения никогда не экспортируются. Для переменных сервиса с sync: false значения запрашиваются при первом создании Blueprint. Переменные с generateValue — включая экспортированные ключи групп окружения — сохраняют существующее значение или получают сгенерированное, если его нет.",
    description: "Generate dialog note under the yaml preview",
  },
  "blueprints.generateError": {
    message: "Не удалось сгенерировать Blueprint",
    description: "Toast when the generate query fails",
  },
  "blueprints.emptyTitle": {
    message: "Вы ещё не создали ни одного экземпляра Blueprint.",
    description: "Blueprints list empty-state heading",
  },
  "blueprints.emptyBody": {
    message:
      "Blueprints регистрируются автоматически при каждом развёртывании render.yaml из репозитория. После развёртывания ваш набор сервисов появится здесь, и вы сможете синхронизировать или проверить его.",
    description:
      "Blueprints list empty-state body explaining auto-registration",
  },
  "blueprints.loadingBody": {
    message: "Загрузка Blueprints…",
    description: "Blueprints list loading state label",
  },
  "blueprints.errorTitle": {
    message: "Не удалось загрузить Blueprints",
    description: "Blueprints list error state heading",
  },
  "blueprints.statusActive": {
    message: "Активен",
    description: "Blueprint status badge — row is active",
  },
  "blueprints.statusInSync": {
    message: "Синхронизирован",
    description: "Blueprint status badge — git-connected and in sync",
  },
  "blueprints.statusSyncing": {
    message: "Синхронизация",
    description: "Blueprint status badge — sync in progress",
  },
  "blueprints.statusError": {
    message: "Ошибка",
    description: "Blueprint status badge — last sync failed",
  },
  "blueprints.statusPaused": {
    message: "Приостановлен",
    description: "Blueprint status badge — auto-sync paused",
  },
  "blueprints.statusUnknown": {
    message: "{status}",
    description: "Blueprint status badge fallback — raw status value",
  },
  "blueprints.resourceSuffixLabel": {
    message: "Суффикс названия ресурса",
    description:
      "Tooltip and screen-reader label of the suffix shown beside a Blueprint's name in the list and on its detail page (w8/m153)",
  },
  "blueprints.detailTitle": {
    message: "{name} · Blueprints",
    description: "Blueprint detail page document title",
  },
  "blueprints.metaRepo": {
    message: "Репозиторий",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaBranch": {
    message: "Ветка",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaPath": {
    message: "Путь к манифесту",
    description:
      "Blueprint detail metadata label — path to render.yaml in repo",
  },
  "blueprints.metaAutoSync": {
    message: "Автосинхронизация",
    description: "Blueprint detail metadata label — auto-sync on push toggle",
  },
  "blueprints.metaCreated": {
    message: "Создан",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaUpdated": {
    message: "Последняя синхронизация",
    description: "Blueprint detail metadata label",
  },
  "blueprints.autoSyncOn": {
    message: "Вкл.",
    description: "Auto-sync toggle label — enabled",
  },
  "blueprints.autoSyncOff": {
    message: "Выкл.",
    description: "Auto-sync toggle label — disabled",
  },
  "blueprints.resourcesTitle": {
    message: "Управляемые ресурсы",
    description:
      "Blueprint detail section heading listing services/databases managed by this blueprint",
  },
  "blueprints.resourcesEmpty": {
    message:
      "Ресурсов пока нет — синхронизируйте Blueprint, чтобы применить ваш render.yaml.",
    description: "Blueprint managed-resources empty state (never synced)",
  },
  "blueprints.resourcesEmptyAwaitingApproval": {
    message:
      "Ресурсов пока нет — подтвердите запланированную синхронизацию, чтобы создать ресурсы из вашего render.yaml.",
    description:
      "Blueprint managed-resources empty state while the Blueprint awaits approval of its planned sync",
  },
  "blueprints.resourcesEmptyAfterError": {
    message:
      "Нет закреплённых ресурсов. Последняя синхронизация завершилась ошибкой — посмотрите описание ошибки в разделе «История синхронизаций», затем повторите попытку после исправления Blueprint.",
    description:
      "Blueprint managed-resources empty state when status is error after a sync",
  },
  "blueprints.syncHistoryTitle": {
    message: "История синхронизаций",
    description: "Blueprint detail section heading for the sync run table",
  },
  "blueprints.syncHistoryEmpty": {
    message: "Синхронизаций пока нет.",
    description: "Blueprint sync history empty state",
  },
  "blueprints.syncColCommit": {
    message: "Коммит",
    description: "Sync history table column — commit SHA",
  },
  "blueprints.syncColState": {
    message: "Состояние",
    description: "Sync history table column — sync run state",
  },
  "blueprints.syncColStarted": {
    message: "Начата",
    description: "Sync history table column — sync run start time",
  },
  "blueprints.syncColCompleted": {
    message: "Завершена",
    description: "Sync history table column — sync run completion time",
  },
  "blueprints.detachPreviewTitle": {
    message: "Ресурсы перестанут управляться",
    description: "Blueprint resource detachment notice.",
  },
  "blueprints.detachSuccessTitle": {
    message: "Синхронизация завершена — ресурсы отсоединены",
    description: "Blueprint resource detachment notice.",
  },
  "blueprints.detachEstimateNote": {
    message: "Их стоимость не включена в оценку ниже.",
    description:
      "Detached resources are outside the proposed manifest estimate.",
  },
  "blueprints.detachWarning": {
    message:
      "Эти ресурсы продолжат работать и могут продолжать начислять плату. Этот Blueprint больше не будет ими управлять.",
    description: "Blueprint resource detachment notice.",
  },
  "blueprints.syncColError": {
    message: "Подробности",
    description: "Sync history table column — errors and durable sync notes",
  },
  "blueprints.manifestTitle": {
    message: "Манифест render.yaml",
    description: "Blueprint detail section heading for the stored manifest",
  },
  "blueprints.plannedTitle": {
    message: "Запланированная синхронизация",
    description:
      "Blueprint detail: heading of the planned-sync card a created Blueprint shows",
  },
  "blueprints.plannedBody": {
    message:
      "Этот Blueprint создан с планом. Проверьте, что изменится при подтверждении в коммите {commit}, затем подтвердите, чтобы применить его.",
    description:
      "Planned-sync card: explains plan-then-approve; {commit} is a short commit SHA",
  },
  "blueprints.plannedActions": {
    message: "Запланированные изменения",
    description: "Planned-sync card: accessible label of the action list",
  },
  "blueprints.plannedAction": {
    message: "{operation} {kind} {name}",
    description:
      "Planned-sync card: one plan action, e.g. 'create service api'",
  },
  "blueprints.plannedActionsUnavailable": {
    message:
      "Этот план записан до того, как планы стали сохранять свои изменения. Составьте план заново, чтобы увидеть их, или подтвердите, чтобы применить его.",
    description:
      "Planned-sync card: a plan stored without its actions (recorded before migration 0187)",
  },
  "blueprints.plannedManualEnvVars": {
    message:
      "Этих значений нет в файле Blueprint (sync: false). Введите их, чтобы подтвердить.",
    description:
      "Planned-sync card: intro to the required manual env var fields",
  },
  "blueprints.plannedManualEnvVarLabel": {
    message: "{service}: {key}",
    description: "Planned-sync card: label of one manual env var field",
  },
  "blueprints.plannedApprove": {
    message: "Подтвердить",
    description: "Planned-sync card: button that applies the planned sync",
  },
  "blueprints.plannedReplan": {
    message: "Составить план заново",
    description:
      "Planned-sync card: button that plans a new sync at the branch head",
  },
  "blueprints.plannedNone": {
    message:
      "Нет плана, ожидающего подтверждения. Составьте план заново, чтобы прочитать актуальное состояние ветки.",
    description:
      "Planned-sync card: shown when a created Blueprint has no approvable plan",
  },
  "blueprints.plannedApproved": {
    message: "Запланированная синхронизация подтверждена",
    description: "Toast after approving a planned sync",
  },
  "blueprints.plannedApproveFailed": {
    message: "Не удалось подтвердить запланированную синхронизацию",
    description: "Toast when approving fails",
  },
  "blueprints.plannedChanged": {
    message:
      "Кто-то заново составил план для этого Blueprint. Проверьте новейший план, затем подтвердите его.",
    description:
      "Toast when the plan on screen was replaced before it was approved (BLUEPRINT_PLAN_CHANGED)",
  },
  "blueprints.plannedReplanFailed": {
    message: "Не удалось запланировать новую синхронизацию",
    description: "Toast when re-planning fails",
  },
  "blueprints.syncButton": {
    message: "Синхронизировать",
    description: "Button label that triggers an idempotent blueprint re-apply",
  },
  "blueprints.syncConfirmTitle": {
    message: "Синхронизировать Blueprint?",
    description: "Sync confirm dialog title",
  },
  "blueprints.syncConfirmBody": {
    message:
      "Будет применён render.yaml ровно в состоянии коммита, показанного ниже, а не более нового push в ветку. Ресурсы, которые уже ему соответствуют, останутся без изменений.",
    description: "Sync confirm dialog description",
  },
  "blueprints.syncConfirmAction": {
    message: "Синхронизировать",
    description: "Sync confirm dialog primary action button label",
  },
  "blueprints.syncCancel": {
    message: "Отменить",
    description: "Sync confirm dialog cancel button label",
  },
  "blueprints.syncPreviewLoading": {
    message: "Вычисление плана синхронизации по репозиторию…",
    description: "Pre-sync dialog — loading line while blueprintPreview runs",
  },
  "blueprints.syncPreviewInvalid": {
    message:
      "В файле Blueprint, который сейчас лежит в ветке, есть проблемы — синхронизация сейчас может завершиться ошибкой или откатиться к сохранённому манифесту:",
    description:
      "Pre-sync dialog — heading above validation errors from the fetched manifest",
  },
  "blueprints.syncPreviewUnavailable": {
    message:
      "Не удалось вычислить план синхронизации (предварительный просмотр недоступен). Обновите страницу после успешного просмотра — для синхронизации нужен проверенный коммит.",
    description:
      "Pre-sync dialog — preview transport failure; sync is disabled until a valid preview pins a commit (w8/m41)",
  },
  "blueprints.syncPreviewCommit": {
    message: "Проверенный коммит {sha}",
    description: "Pre-sync dialog — short SHA of the previewed Git revision",
  },
  "blueprints.syncRefreshPreview": {
    message: "Обновить предварительный просмотр",
    description:
      "Pre-sync dialog — refetch Git + re-validate after BLUEPRINT_SOURCE_CHANGED or an invalid preview",
  },
  "blueprints.syncSourceChanged": {
    message:
      "Источник Blueprint изменился после вашей проверки — обновите предварительный просмотр и подтвердите снова",
    description:
      "Toast when syncBlueprint refuses with BLUEPRINT_SOURCE_CHANGED (path/repo drift or stale pin)",
  },
  "blueprints.syncSuccess": {
    message: "Blueprint синхронизирован",
    description: "Toast shown after a successful syncBlueprint call",
  },
  "blueprints.syncError": {
    message: "Синхронизация не удалась",
    description: "Toast shown when syncBlueprint returns an error",
  },
  "blueprints.syncBusy": {
    message:
      "Эта синхронизация больше не управляет выполнением (занята, прервана или заменена) — запустите новую синхронизацию; частично выполненная работа никогда не повторяется",
    description:
      "Toast shown when syncBlueprint refuses with BLUEPRINT_SYNC_BUSY (admission race, mid-apply fence, or disconnect-busy)",
  },
  "blueprints.editField": {
    message: "Редактировать: {field}",
    description: "Aria label for the inline-edit pencil button",
  },
  "blueprints.saveField": {
    message: "Сохранить: {field}",
    description: "Aria label for the inline-edit save button",
  },
  "blueprints.cancelEdit": {
    message: "Отменить редактирование",
    description: "Aria label for the inline-edit cancel button",
  },
  "blueprints.pathInvalid": {
    message:
      "Должен быть чистым путём к файлу .yaml или .yml относительно репозитория (без начального слеша и без '..').",
    description: "Inline path editor client-side validation message",
  },
  "blueprints.updateSuccess": {
    message: "Blueprint обновлён",
    description: "Toast shown after a successful updateBlueprint call",
  },
  "blueprints.updateError": {
    message: "Не удалось обновить",
    description: "Toast shown when updateBlueprint returns an error",
  },
  "blueprints.disconnectButton": {
    message: "Отключить",
    description:
      "Button that disconnects a blueprint from its Git repo (stops auto-sync, keeps resources)",
  },
  "blueprints.disconnectTitle": {
    message: "Отключить Blueprint?",
    description: "Disconnect confirm dialog title",
  },
  "blueprints.disconnectBody": {
    message:
      "Автосинхронизация при push будет остановлена, а Blueprint исчезнет из вашего списка. Уже развёрнутые ресурсы останутся без изменений.",
    description: "Disconnect confirm dialog description",
  },
  "blueprints.disconnectAction": {
    message: "Отключить",
    description: "Disconnect confirm dialog primary action button label",
  },
  "blueprints.disconnectCancel": {
    message: "Отмена",
    description: "Disconnect confirm dialog cancel button label",
  },
  "blueprints.disconnectSuccess": {
    message: "Blueprint отключён",
    description: "Toast shown after a successful disconnectBlueprint call",
  },
  "blueprints.disconnectError": {
    message: "Не удалось отключить",
    description: "Toast shown when disconnectBlueprint returns an error",
  },
  "blueprints.disconnectBusy": {
    message:
      "Сейчас применяется синхронизация — повторите отключение после её завершения",
    description:
      "Toast shown when disconnectBlueprint refuses with BLUEPRINT_SYNC_BUSY",
  },
  "blueprints.validateTitle": {
    message: "Проверка",
    description: "Validate panel section heading",
  },
  "blueprints.validateRun": {
    message: "Запустить проверку",
    description: "Validate panel button label",
  },
  "blueprints.validateValid": {
    message: "Манифест корректен — ошибок не найдено.",
    description: "Validate result: manifest parsed successfully",
  },
  "blueprints.validateInvalid": {
    message: "В манифесте есть ошибки:",
    description:
      "Validate result: manifest has parse errors — followed by the error list",
  },
  "blueprints.validateNoResult": {
    message: "Результатов пока нет.",
    description: "Validate panel placeholder before first run",
  },
  "blueprints.stateCreated": {
    message: "Ожидает подтверждения",
    description:
      "Sync state (and Blueprint status) created: a planned sync waiting for approval (w2/m256); never a queued run.",
  },
  "blueprints.stateRunning": {
    message: "Выполняется",
    description: "Sync-run state: the run is in progress.",
  },
  "blueprints.stateSuccess": {
    message: "Успешно",
    description: "Sync-run state: the run completed successfully.",
  },
};

export default ruBlueprints;
