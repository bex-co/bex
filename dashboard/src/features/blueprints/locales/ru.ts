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
      "Preview alert body on /blueprint/new when the only problem is a takeover conflict (replaces the raw API message)",
  },
  "blueprints.previewConflict": {
    message: "Уже управляется — развёртывание перехватит его",
    description:
      "Alert title on /blueprint/new when the only preview problem is a resource another Blueprint manages, which an explicit takeover confirmation resolves (w4/m125)",
  },
  "blueprints.previewMatchTitle": {
    message: "Совпадает с существующими ресурсами",
    description:
      "Notice title on /blueprint/new when the Blueprint file names resources that already exist in the workspace (w8/m153, Render parity)",
  },
  "blueprints.previewMatchBody": {
    message:
      "Этот Blueprint совпадает с ресурсами, которые уже есть в вашем рабочем пространстве. Новые ресурсы будут созданы с суффиксом «-{suffix}» в названии. Существующие ресурсы не изменятся.",
    description:
      "Notice body on /blueprint/new for matched resources; {suffix} is the four-character resource name suffix, shown after a hyphen as it appears in a name",
  },
  "blueprints.previewMatchListLabel": {
    message: "Уже есть в вашем рабочем пространстве:",
    description:
      "Notice on /blueprint/new — label above the list of existing resources the Blueprint file matches",
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
    description:
      "Generate Blueprint: the selection bar's, a resource row menu's and a project environment menu's entry that opens the Create Blueprint page (w3/m681)",
  },
  "blueprints.generatePageTitle": {
    message: "Создать новый Blueprint из ваших сервисов",
    description:
      "Create Blueprint page title and document title, opened from a selection bar or a resource row's menu (w3/m681)",
  },
  "blueprints.generatePageTitleEnvironment": {
    message: "Создать новый Blueprint из {project} / {environment}",
    description:
      "Create Blueprint page title and document title, opened from a project environment's menu: the project and environment names",
  },
  "blueprints.generateBreadcrumb": {
    message: "Создание Blueprint",
    description:
      "Topbar breadcrumb of the Create Blueprint page (/blueprint/generate)",
  },
  "blueprints.generateStepDownload": {
    message: "Скачайте сгенерированный render.yaml",
    description: "Create Blueprint page: step 1 title",
  },
  "blueprints.generateIncludes_one": {
    message: "Включает {count} сервис:",
    description:
      "Create Blueprint page: step 1 sentence before the list of exported resource names (services, databases and Key Value all count)",
  },
  "blueprints.generateIncludes_few": {
    message: "Включает {count} сервиса:",
    description:
      "Create Blueprint page: step 1 sentence before the list of exported resource names (services, databases and Key Value all count)",
  },
  "blueprints.generateIncludes_many": {
    message: "Включает {count} сервисов:",
    description:
      "Create Blueprint page: step 1 sentence before the list of exported resource names (services, databases and Key Value all count)",
  },
  "blueprints.generateIncludes_other": {
    message: "Включает {count} сервиса:",
    description:
      "Create Blueprint page: step 1 sentence before the list of exported resource names (services, databases and Key Value all count)",
  },
  "blueprints.generating": {
    message: "Генерация render.yaml...",
    description: "Create Blueprint page: step 1 while render.yaml is generated",
  },
  "blueprints.generateCopiedLabel": {
    message: "Скопировано!",
    description:
      "Create Blueprint page: the Copy button's label for 2 seconds after copying render.yaml",
  },
  "blueprints.generateStepCommit": {
    message: "Добавьте render.yaml в Git",
    description: "Create Blueprint page: step 2 title",
  },
  "blueprints.generateStepCommitBody": {
    message:
      "Закоммитьте render.yaml в корневой каталог репозитория и отправьте изменения своему Git-провайдеру.",
    description: "Create Blueprint page: step 2 text above the git commands",
  },
  "blueprints.generateStepConnect": {
    message: "Подключите новый Blueprint",
    description: "Create Blueprint page: step 3 title",
  },
  "blueprints.generateStepConnectBody": {
    message:
      "Создайте новый экземпляр Blueprint и подключите свой репозиторий.",
    description: "Create Blueprint page: step 3 text, followed by a docs link",
  },
  "blueprints.generateSeeDocs": {
    message: "См. документацию.",
    description: "Create Blueprint page: step 3 link to the Blueprint docs",
  },
  "blueprints.generateConnect": {
    message: "Подключить новый Blueprint",
    description:
      "Create Blueprint page: step 3 button, opens the New Blueprint Instance page in a new tab",
  },
  "blueprints.generateHelpfulResources": {
    message: "Полезные ресурсы",
    description:
      "Create Blueprint page: heading of the links card beside the steps",
  },
  "blueprints.generateLearnBlueprints": {
    message: "Подробнее о Blueprint",
    description:
      "Create Blueprint page: link to the Blueprint docs in the Helpful Resources card",
  },
  "blueprints.copied": {
    message: "Скопировано",
    description: "Toast after copying a code snippet on a Blueprint page",
  },
  "blueprints.generateCopy": {
    message: "Копировать",
    description: "Generate dialog — copy the yaml to the clipboard",
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
  "blueprints.metaCreated": {
    message: "Создан",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaUpdated": {
    message: "Последняя синхронизация",
    description: "Blueprint detail metadata label",
  },
  "blueprints.detailsTitle": {
    message: "Сведения",
    description:
      "Blueprint Settings page: title of bex's card of read-only facts (repository, created, last synced) under Render's Settings card (w3/m679)",
  },
  "blueprints.settingsNameHint": {
    message: "Уникальное название вашего Blueprint.",
    description:
      "Blueprint Settings card: help under Blueprint Name (Render's copy, w3/m679)",
  },
  "blueprints.settingsRequired": {
    message: "Обязательно",
    description:
      "Blueprint Settings card: shown under a row left empty (Render's copy, w3/m679)",
  },
  "blueprints.settingsNameTaken": {
    message: "Название уже используется",
    description:
      "Blueprint Settings card: shown under Blueprint Name when another Blueprint of the workspace has the name (Render's copy, w3/m679)",
  },
  "blueprints.settingsAutoSync": {
    message: "Автосинхронизация",
    description:
      "Blueprint Settings card: the Auto Sync row's label (Render's copy, w3/m679)",
  },
  "blueprints.settingsAutoSyncHint": {
    message:
      "Автоматически синхронизировать изменения файла Blueprint? Выберите «Нет», чтобы запускать синхронизацию вручную.",
    description:
      "Blueprint Settings card: help under Auto Sync; quotes the No option (Render's copy, w3/m679)",
  },
  "blueprints.settingsYes": {
    message: "Да",
    description:
      "Blueprint Settings card: Auto Sync option, on (Render's copy, w3/m679)",
  },
  "blueprints.settingsNo": {
    message: "Нет",
    description:
      "Blueprint Settings card: Auto Sync option, off (Render's copy, w3/m679)",
  },
  "blueprints.settingsBranch": {
    message: "Ветка",
    description: "Blueprint Settings card: the Branch row's label (w3/m679)",
  },
  "blueprints.settingsBranchHint": {
    message: "Git-ветка для сборки и деплоя.",
    description:
      "Blueprint Settings card: help under Branch (Render's copy, w3/m679)",
  },
  "blueprints.settingsBranchNotFound": {
    message: "Ветка не найдена в этом репозитории. Выберите другую ветку.",
    description:
      "Blueprint Settings card: shown under Branch when the repository has no such branch (Render's copy, w3/m679)",
  },
  "blueprints.settingsPath": {
    message: "Путь к Blueprint",
    description:
      "Blueprint Settings card: the Blueprint Path row's label (Render's copy, w3/m679)",
  },
  "blueprints.settingsPathHint": {
    message:
      "Путь к файлу Blueprint в вашем репозитории (напр. infra/render.yaml). По умолчанию — render.yaml в корне репозитория.",
    description:
      "Blueprint Settings card: help under Blueprint Path (Render's copy, w3/m679)",
  },
  "blueprints.settingsFileNotFound": {
    message:
      "Файл Blueprint «{path}» не найден в ветке «{branch}». Проверьте, что файл есть в этой ветке.",
    description:
      "Blueprint Settings card: shown under Branch or Blueprint Path when the Blueprint file is not at that path on that branch (Render's copy, w3/m679)",
  },
  "blueprints.settingsUpdateFailed": {
    message: "Не удалось обновить Blueprint. Попробуйте ещё раз.",
    description:
      "Blueprint Settings card: shown under a row whose save failed with no reason from the server (Render's copy, w3/m679)",
  },
  "blueprints.disconnectNoSync": {
    message:
      "Ресурсы больше не будут автоматически синхронизироваться из файла Blueprint.",
    description:
      "Disconnect Blueprint modal: first sentence (Render's copy, w3/m679)",
  },
  "blueprints.disconnectKeeps": {
    message:
      "Сам файл не будет удалён. Управляемые ресурсы не будут удалены. Вы всегда сможете снова подключить файл позже на странице Blueprints.",
    description:
      "Disconnect Blueprint modal: second sentence (Render's copy, w3/m679)",
  },
  "blueprints.disconnectConfirm": {
    message: "Вы уверены, что хотите отключить спецификацию Blueprint {name}?",
    description:
      "Disconnect Blueprint modal: the question, with the Blueprint's name in bold (Render's copy, w3/m679)",
  },
  "blueprints.resourcesEmpty": {
    message: "Этот Blueprint не управляет ни одним ресурсом",
    description:
      "A Blueprint's Resources page when it manages nothing, Render's words (w3/m680)",
  },
  "blueprints.resourcesFileProblems_one": {
    message: "При синхронизации с файлом Blueprint возникла проблема:",
    description:
      "A Blueprint's Resources page when its newest sync failed on its file: the line above the file's errors, Render's 'There was a problem' / 'There were a few problems' (w3/m680)",
  },
  "blueprints.resourcesFileProblems_few": {
    message: "При синхронизации с файлом Blueprint возникло несколько проблем:",
    description:
      "A Blueprint's Resources page when its newest sync failed on its file: the line above the file's errors, Render's 'There was a problem' / 'There were a few problems' (w3/m680)",
  },
  "blueprints.resourcesFileProblems_many": {
    message: "При синхронизации с файлом Blueprint возникло несколько проблем:",
    description:
      "A Blueprint's Resources page when its newest sync failed on its file: the line above the file's errors, Render's 'There was a problem' / 'There were a few problems' (w3/m680)",
  },
  "blueprints.resourcesFileProblems_other": {
    message: "При синхронизации с файлом Blueprint возникло несколько проблем:",
    description:
      "A Blueprint's Resources page when its newest sync failed on its file: the line above the file's errors, Render's 'There was a problem' / 'There were a few problems' (w3/m680)",
  },
  "blueprints.colBuildSourceName": {
    message: "Название источника сборки",
    description:
      "A Blueprint's Resources page: the Build Sources table's name column, Render's 'Build Source Name' (w3/m680)",
  },
  "blueprints.buildSourceUnlinked": {
    message: "Не связан",
    description:
      "A Blueprint's Build Sources table: badge on a build source no service builds from, Render's 'Unlinked' (w3/m680)",
  },
  "blueprints.buildSourcesUngrouped": {
    message: "Без проекта",
    description:
      "A Blueprint's Build Sources table: the row over the build sources in no project, Render's 'Ungrouped' (w3/m680)",
  },
  "blueprints.syncHistoryEmpty": {
    message: "Синхронизаций пока нет.",
    description: "Syncs page: no syncs yet, Render's wording (w3/m678)",
  },
  "blueprints.detachPreviewTitle": {
    message: "Ресурсы перестанут управляться",
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
  "blueprints.manifestTitle": {
    message: "Манифест render.yaml",
    description: "Blueprint detail section heading for the stored manifest",
  },
  "blueprints.plannedActions": {
    message: "Запланированные изменения",
    description:
      "A sync's page: accessible label of its list of planned actions (w3/m678)",
  },
  "blueprints.plannedAction": {
    message: "{operation} {kind} {name}",
    description:
      "Planned-sync card: one plan action, e.g. 'create service api'",
  },
  "blueprints.plannedManualEnvVars": {
    message:
      "Этих значений нет в файле Blueprint (sync: false). Введите их, чтобы подтвердить.",
    description:
      "A waiting sync's page: intro to the fields for the sync: false values approving it needs (w3/m678)",
  },
  "blueprints.plannedManualEnvVarLabel": {
    message: "{service}: {key}",
    description:
      "A waiting sync's page: label of one sync: false value's field (w3/m678)",
  },
  "blueprints.plannedApprove": {
    message: "Подтвердить",
    description:
      "A waiting sync's page: the button that applies the planned sync, Render's Approve (w3/m678)",
  },
  "blueprints.plannedChanged": {
    message:
      "Кто-то заново составил план для этого Blueprint. Проверьте новейший план, затем подтвердите его.",
    description:
      "A sync's page: inline alert when the plan on screen was replaced before it was approved (BLUEPRINT_PLAN_CHANGED)",
  },
  "blueprints.plannedReplanFailed": {
    message: "Не удалось запланировать новую синхронизацию",
    description:
      "Sync Error banner: Manual sync could not plan a sync and the server gave no reason (w3/m678)",
  },
  "blueprints.syncButton": {
    message: "Ручная синхронизация",
    description:
      "The Blueprint header's one action, Render's Manual sync: plans a sync and opens its page (w3/m678)",
  },
  "blueprints.editField": {
    message: "Редактировать: {field}",
    description: "Aria label for the inline-edit pencil button",
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
  "blueprints.disconnectButton": {
    message: "Отключить Blueprint",
    description:
      "Blueprint Settings page: the critical button that opens the Disconnect Blueprint modal (Render's copy, w3/m679)",
  },
  "blueprints.disconnectTitle": {
    message: "Отключить Blueprint",
    description: "Disconnect Blueprint modal title (Render's copy, w3/m679)",
  },
  "blueprints.disconnectAction": {
    message: "Отключить Blueprint",
    description:
      "Disconnect Blueprint modal: the critical confirm button (Render's copy, w3/m679)",
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
  "blueprints.createNameDescription": {
    message: "Уникальное название этого экземпляра Blueprint.",
    description: "Create-Blueprint Name row description",
  },
  "blueprints.createBlockedSource": {
    message: "Выберите репозиторий, чтобы продолжить.",
    description:
      "Create-Blueprint action bar: why Deploy is held while no repository is chosen",
  },
  "blueprints.createBlockedBranch": {
    message: "Укажите ветку, чтобы продолжить.",
    description:
      "Create-Blueprint action bar: why Deploy is held while the branch is empty",
  },
  "blueprints.createBlockedReviewing": {
    message: "Проверяем файл Blueprint…",
    description:
      "Create-Blueprint action bar: why Deploy is held while the Blueprint file is being reviewed",
  },
  "blueprints.createBlockedReview": {
    message: "Устраните проблемы, найденные при проверке, чтобы продолжить.",
    description:
      "Create-Blueprint action bar: why Deploy is held while the review found problems in the Blueprint file",
  },
  "blueprints.navResources": {
    message: "Ресурсы",
    description:
      "Blueprint sidebar entry and page heading for the resources the Blueprint manages (Render's Resources, /blueprints/<id>/resources, w3/m677)",
  },
  "blueprints.navSyncs": {
    message: "Синхронизации",
    description:
      "Blueprint sidebar entry and page heading for the Blueprint's syncs (Render's Syncs, /blueprints/<id>/syncs, w3/m677)",
  },
  "blueprints.navSettings": {
    message: "Настройки",
    description:
      "Blueprint sidebar entry and page heading for the Blueprint's settings (Render's Settings, /blueprints/<id>/settings, w3/m677)",
  },
  "blueprints.resourcesViewSyncs": {
    message: "Перейти к синхронизациям",
    description:
      "Link from a Blueprint's empty Resources page to its Syncs page, where the waiting plan or the failed sync is (w3/m677)",
  },
  "blueprints.syncViewDetails": {
    message: "Подробнее",
    description:
      "Sync card: link to the sync's own page (Render's View details, w3/m678)",
  },
  "blueprints.syncsLoadMore": {
    message: "Загрузить ещё",
    description:
      "Syncs page: button reading the next page of older syncs (w3/m678)",
  },
  "blueprints.syncsLoadMoreError": {
    message: "Не удалось загрузить более ранние синхронизации.",
    description:
      "Syncs page: toast when reading an older page of syncs fails (w3/m678)",
  },
  "blueprints.syncPageTitle": {
    message: "Синхронизация: {sha}",
    description:
      "A sync's page heading, Render's 'Sync: <short commit id>'; {sha} is the 7-character commit id (w3/m678)",
  },
  "blueprints.syncPageTitleNoCommit": {
    message: "Синхронизация",
    description:
      "A sync's page heading when the sync read no commit, Render's 'Sync' (w3/m678)",
  },
  "blueprints.syncDocumentTitle": {
    message: "Синхронизация",
    description:
      "Document title's page name on a sync's page: 'Sync ・ <name> ・ Blueprint' (w3/m678)",
  },
  "blueprints.syncUpToDate": {
    message: "Ресурсы уже в актуальном состоянии",
    description:
      "A sync's page when its plan has no actions, Render's wording (w3/m678)",
  },
  "blueprints.syncNotFullySynced": {
    message: "Не удалось полностью синхронизировать эти ресурсы.",
    description:
      "A failed sync's page with no actions and no errors, Render's critical line (w3/m678)",
  },
  "blueprints.syncNotFullySyncedBody": {
    message: "Если ошибка повторяется, обратитесь в поддержку.",
    description:
      "Under blueprints.syncNotFullySynced: what to do if it keeps happening (w3/m678)",
  },
  "blueprints.syncActionsNotRecorded": {
    message:
      "Эта синхронизация записана до того, как синхронизации стали сохранять свои действия, поэтому их нельзя показать.",
    description:
      "A sync's page for a sync recorded before every sync kept its actions (w3/m678)",
  },
  "blueprints.syncActionCreate": {
    message: "Создать {kind} {name}",
    description:
      "A sync's planned action: create a resource; {kind} is its kind label, {name} its name, drawn bold (w3/m678)",
  },
  "blueprints.syncActionUpdate": {
    message: "Обновить {kind} {name}",
    description:
      "A sync's planned action: change a resource; {kind} is its kind label, {name} its name (w3/m678)",
  },
  "blueprints.syncActionDetach": {
    message: "Прекратить управление: {kind} {name}",
    description:
      "A sync's planned action: stop managing a resource the file no longer names; {kind}, {name} (w3/m678)",
  },
  "blueprints.syncActionError": {
    message: "Невозможно применить {kind} {name}",
    description:
      "A sync's planned action the planner refused; {kind}, {name}; the reason follows (w3/m678)",
  },
  "blueprints.syncActionDetachNote": {
    message: "Ресурс продолжит работать, и за него может взиматься плата.",
    description:
      "Under a 'stop managing' action: the resource is not deleted (w3/m678)",
  },
  "blueprints.syncActionStateUnknown": {
    message: "Состояние неизвестно",
    description:
      "Accessible name of a planned action's state icon when bex can't say which state it reached (a failed sync's actions, w3/m678)",
  },
  "blueprints.syncApproveFailed": {
    message: "Не удалось одобрить эту синхронизацию. Попробуйте ещё раз.",
    description:
      "A sync's page: inline alert when Approve fails without the server's reason, Render's wording (w3/m678)",
  },
  "blueprints.syncNotFound": {
    message:
      "Такой синхронизации нет. Запланированная синхронизация, не одобренная в течение 24 часов, удаляется.",
    description:
      "A sync's page for an id that is not one of this Blueprint's syncs, e.g. a plan deleted after 24 hours (w3/m678)",
  },
  "blueprints.syncErrorBannerTitle": {
    message: "Ошибка синхронизации",
    description:
      "Title of Render's dismissible banner when Manual sync could not plan a sync (w3/m678)",
  },
  "blueprints.syncErrorBannerDismiss": {
    message: "Закрыть",
    description:
      "Accessible name of the Sync Error banner's close button (w3/m678)",
  },
  "blueprints.syncErrorsIntro_one": {
    message: "При синхронизации с вашим Blueprint возникла проблема:",
    description:
      "A sync's page: the line above its errors, Render's 'There was a problem' / 'There were a few problems' (w3/m678)",
  },
  "blueprints.syncErrorsIntro_few": {
    message: "При синхронизации с вашим Blueprint возникло несколько проблем:",
    description:
      "A sync's page: the line above its errors, Render's 'There was a problem' / 'There were a few problems' (w3/m678)",
  },
  "blueprints.syncErrorsIntro_many": {
    message: "При синхронизации с вашим Blueprint возникло несколько проблем:",
    description:
      "A sync's page: the line above its errors, Render's 'There was a problem' / 'There were a few problems' (w3/m678)",
  },
  "blueprints.syncErrorsIntro_other": {
    message: "При синхронизации с вашим Blueprint возникло несколько проблем:",
    description:
      "A sync's page: the line above its errors, Render's 'There was a problem' / 'There were a few problems' (w3/m678)",
  },
};

export default ruBlueprints;
