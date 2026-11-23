import type { TranslationEntry } from "@/i18n";

const ruAgentEnvironment: Record<string, TranslationEntry> = {
  "agentEnvironment.title": {
    message: "Агенты",
    description: "Settings → Agents card title and section-nav label",
  },
  "agentEnvironment.description": {
    message:
      "Как сессии агентов в этом рабочем пространстве получают доступ к сети и какую модель используют. Применяется к каждой новой сессии.",
    description: "Agents settings card description",
  },
  "agentEnvironment.loadFailed": {
    message: "Не удалось загрузить настройки агентов.",
    description: "Agents card error when the environment query fails",
  },
  "agentEnvironment.retry": {
    message: "Повторить",
    description: "Retry button on the Agents card load error",
  },
  "agentEnvironment.readOnlyNote": {
    message:
      "Изменять настройки агентов могут только администраторы рабочего пространства.",
    description: "Read-only notice shown to non-admin members",
  },
  "agentEnvironment.networkTitle": {
    message: "Доступ к сети",
    description: "Heading (and radiogroup label) of the network tier picker",
  },
  "agentEnvironment.networkDescription": {
    message:
      "GitHub и поставщик модели доступны всегда. Выберите, что ещё доступно песочнице агента.",
    description: "Network tier picker description",
  },
  "agentEnvironment.tierRegistriesTitle": {
    message: "Реестры пакетов (рекомендуется)",
    description: "Network tier option: built-in package registries",
  },
  "agentEnvironment.tierRegistriesDescription": {
    message:
      "Агент может устанавливать зависимости из npm, PyPI, Go, Maven, crates, RubyGems и Docker Hub, чтобы собирать и тестировать ваш код.",
    description: "Description of the registries network tier",
  },
  "agentEnvironment.tierCustomTitle": {
    message: "Свой набор",
    description: "Network tier option: registries plus listed hosts",
  },
  "agentEnvironment.tierCustomDescription": {
    message: "Реестры пакетов и имена хостов, которые вы укажете ниже.",
    description: "Description of the custom network tier",
  },
  "agentEnvironment.tierFullTitle": {
    message: "Полный доступ в интернет",
    description: "Network tier option: any public host over HTTP(S)",
  },
  "agentEnvironment.tierFullDescription": {
    message:
      "Любой публичный сайт по HTTP(S) через egress-прокси bex. Прямые обращения к API поставщиков моделей остаются заблокированными (собственная модель агента продолжает работать), а SSH и другие протоколы, кроме HTTP, недоступны.",
    description:
      "Description of the full-internet tier: HTTP(S) via the egress proxy, direct model-provider API calls blocked, no SSH/non-HTTP",
  },
  "agentEnvironment.registryHostsSummary_one": {
    message: "Включённые реестры пакетов ({count} хост)",
    description: "Disclosure summary listing the built-in registry hosts (one)",
  },
  "agentEnvironment.registryHostsSummary_few": {
    message: "Включённые реестры пакетов ({count} хоста)",
    description:
      "Disclosure summary listing the built-in registry hosts (many)",
  },
  "agentEnvironment.registryHostsSummary_many": {
    message: "Включённые реестры пакетов ({count} хостов)",
    description:
      "Disclosure summary listing the built-in registry hosts (many)",
  },
  "agentEnvironment.registryHostsSummary_other": {
    message: "Включённые реестры пакетов ({count} хоста)",
    description:
      "Disclosure summary listing the built-in registry hosts (many)",
  },
  "agentEnvironment.hostsLabel": {
    message: "Дополнительные имена хостов",
    description: "Label of the custom-tier hostname textarea",
  },
  "agentEnvironment.hostsHelp": {
    message:
      "По одному имени хоста на строку, не более {max}. Только точные имена: без схемы, порта, пути и подстановочных знаков.",
    description: "Help under the custom hostname textarea",
  },
  "agentEnvironment.hostsError.too_many_entries": {
    message: "Укажите не более {max} имён хостов.",
    description: "Custom hosts error: too many entries",
  },
  "agentEnvironment.hostsError.empty": {
    message: "Имена хостов не могут быть пустыми.",
    description: "Custom hosts error: empty entry",
  },
  "agentEnvironment.hostsError.whitespace": {
    message: "«{entry}» содержит пробелы по краям.",
    description: "Custom hosts error: surrounding whitespace",
  },
  "agentEnvironment.hostsError.not_a_hostname": {
    message:
      "«{entry}» не является точным именем хоста. Используйте имя вида api.example.com без схемы, порта, пути и подстановочных знаков.",
    description: "Custom hosts error: not a hostname",
  },
  "agentEnvironment.hostsError.ip_literal": {
    message: "«{entry}» — IP-адрес. Используйте вместо него имя хоста.",
    description: "Custom hosts error: IP literal",
  },
  "agentEnvironment.hostsError.private_name": {
    message:
      "«{entry}» — частное имя или имя внутри кластера, его нельзя разрешить.",
    description: "Custom hosts error: private name",
  },
  "agentEnvironment.hostsError.already_baseline": {
    message:
      "«{entry}» уже доступен (или доступен только через прокси модели). Удалите его из списка.",
    description: "Custom hosts error: already in the baseline",
  },
  "agentEnvironment.hostsError.duplicate": {
    message: "«{entry}» указан более одного раза.",
    description: "Custom hosts error: duplicate",
  },
  "agentEnvironment.hostsError.invalid": {
    message: "Список имён хостов недопустим.",
    description: "Custom hosts error: unknown reason",
  },
  "agentEnvironment.modelLabel": {
    message: "Модель по умолчанию",
    description: "Label of the default model field",
  },
  "agentEnvironment.modelPlaceholder": {
    message: "По умолчанию у поставщика",
    description: "Placeholder of the empty default model field",
  },
  "agentEnvironment.modelHelp": {
    message:
      "Идентификатор модели для новых сессий, например claude-sonnet-4-5. Оставьте пустым, чтобы использовать модель поставщика по умолчанию.",
    description: "Help under the default model field",
  },
  "agentEnvironment.modelPlatformDefault": {
    message: "По умолчанию платформы ({model})",
    description:
      "Default model picker — no default: the platform catalog's first model (w10/m516)",
  },
  "agentEnvironment.modelCatalogHelp": {
    message:
      "Новые сессии и автоматизации без указанной модели используют эту. Участники по-прежнему могут выбрать для сессии другую модель из каталога платформы.",
    description: "Help under the platform model catalog picker (w10/m516)",
  },
  "agentEnvironment.modelInvalid": {
    message:
      "Используйте идентификатор модели без пробелов (не более 255 символов).",
    description: "Default model validation error",
  },
  "agentEnvironment.modelNotOfferedError": {
    message:
      "Модель платформы не предлагает {model}. Выберите модель из списка.",
    description:
      "Server refusal: the default model is not in the platform catalog (w10/m516)",
  },
  "agentEnvironment.templateLabel": {
    message: "Песочница по умолчанию",
    description: "Label of the default sandbox template picker (w10/m519)",
  },
  "agentEnvironment.templateHelp": {
    message:
      "Новые сессии и автоматизации без указанной песочницы запускаются на этой. Действует для сессий, начатых после изменения; участники по-прежнему могут выбрать другую для сессии.",
    description: "Help under the default sandbox template picker (w10/m519)",
  },
  "agentEnvironment.templateInvalidError": {
    message: "{template} — не шаблон песочницы. Выберите один из: {templates}.",
    description:
      "Server refusal: the default sandbox template is not a registered one; {templates} lists those (w10/m519)",
  },
  "agentEnvironment.tierInvalid": {
    message: "Этот вариант доступа к сети не поддерживается.",
    description: "Server refusal: invalid tier",
  },
  "agentEnvironment.forbidden": {
    message:
      "Изменять настройки агентов могут только администраторы рабочего пространства.",
    description: "Server refusal: caller is not an admin",
  },
  "agentEnvironment.saveFailed": {
    message: "Не удалось сохранить настройки агентов. Повторите попытку.",
    description: "Generic save failure",
  },
  "agentEnvironment.saveFailedReason": {
    message: "Не удалось сохранить настройки агентов: {reason}",
    description: "Save failure with the server's reason",
  },
  "agentEnvironment.save": {
    message: "Сохранить",
    description: "Agents card save button",
  },
  "agentEnvironment.saved": {
    message: "Настройки агентов сохранены",
    description: "Success toast after saving",
  },
  "agentEnvironment.modelKeyTitle": {
    message: "Поставщик модели",
    description: "Heading of the model source / provider key section",
  },
  "agentEnvironment.modelKeyDescription": {
    message:
      "Сессии агентов используют модель от bex, пока рабочее пространство не добавит собственный ключ поставщика. С вашим ключом сессии обращаются к поставщику напрямую, а bex взимает плату только за вычисления в песочнице.",
    description: "Explains the model source choice (ADR091 D7)",
  },
  "agentEnvironment.modelSourceLabel": {
    message: "Источник модели:",
    description: "Label before the current model source",
  },
  "agentEnvironment.modelSourcePlatform": {
    message: "от bex (оплата по стоимости поставщика модели + 30%)",
    description: "Model source: the platform model",
  },
  "agentEnvironment.modelSourceByo": {
    message: "Ваш собственный ключ (bex не взимает плату за токены)",
    description: "Model source: the workspace's own key",
  },
  "agentEnvironment.modelSourceNone": {
    message: "Нет: добавьте ключ поставщика, чтобы запускать сессии агентов",
    description: "Model source: no key available",
  },
  "agentEnvironment.modelKeyLabel": {
    message: "API-ключ поставщика",
    description: "Label of the write-only key field when none is set",
  },
  "agentEnvironment.modelKeyReplaceLabel": {
    message: "Заменить API-ключ поставщика",
    description: "Label of the write-only key field when a key is set",
  },
  "agentEnvironment.modelKeyPlaceholder": {
    message: "sk-ant-…",
    description: "Placeholder of the key field when none is set",
  },
  "agentEnvironment.modelKeyPlaceholderSet": {
    message: "Ключ сохранён; введите новый, чтобы заменить его",
    description:
      "Placeholder of the key field when a key is set (the key is never shown)",
  },
  "agentEnvironment.modelKeyHelp": {
    message: "Только запись: ключ надёжно хранится и больше не отображается.",
    description: "Help under the key field",
  },
  "agentEnvironment.modelKeySave": {
    message: "Сохранить ключ",
    description: "Button that saves the model key",
  },
  "agentEnvironment.modelKeyClear": {
    message: "Удалить ключ",
    description: "Button that opens the remove-key confirmation",
  },
  "agentEnvironment.modelKeyClearTitle": {
    message: "Удалить ключ поставщика модели?",
    description: "Remove-key confirmation title",
  },
  "agentEnvironment.modelKeyClearBody": {
    message:
      "Сессии агентов перестанут использовать этот ключ со своего следующего запроса к модели. Ключ нельзя показать снова, поэтому для восстановления его нужно будет вставить заново.",
    description: "Remove-key confirmation body",
  },
  "agentEnvironment.modelKeyClearConfirm": {
    message: "Удалить ключ",
    description: "Remove-key confirmation button",
  },
  "agentEnvironment.modelKeySaved": {
    message: "Ключ модели сохранён",
    description: "Toast after saving the key",
  },
  "agentEnvironment.modelKeyCleared": {
    message: "Ключ модели удалён",
    description: "Toast after removing the key",
  },
  "agentEnvironment.modelKeyInvalid": {
    message: "Введите ключ без пробелов (не более 4096 символов).",
    description: "Client/server refusal of a malformed key",
  },
  "agentEnvironment.modelKeySaveFailed": {
    message: "Не удалось сохранить ключ модели. Повторите попытку.",
    description: "Generic model key write failure",
  },
  "agentEnvironment.spendTitle": {
    message: "Расходы на модель агентов",
    description: "Heading of the agent-model spend section (ADR091 D6)",
  },
  "agentEnvironment.spendPricing": {
    message:
      "Оплата по стоимости поставщика модели + 30%. По достижении лимита новые запросы к модели блокируются до конца месяца.",
    description: "Pricing statement under the spend heading",
  },
  "agentEnvironment.spendSummary": {
    message: "{used} из {cap} в этом месяце",
    description: "Month-to-date spend against the effective monthly cap",
  },
  "agentEnvironment.spendSummaryCeiling": {
    message: "{used} из {cap} сверх кредитов на оплату, {total} в этом месяце",
    description:
      "Spend summary and meter value when the trust tier's ceiling binds on the spend billing credits leave ({used}), which differs from month-to-date ({total})",
  },
  "agentEnvironment.spendMeterValue": {
    message: "{used} из {cap}",
    description: "Screen-reader value of the spend progress bar",
  },
  "agentEnvironment.spendUnavailable": {
    message: "Данные о расходах сейчас недоступны.",
    description: "Shown when the spend fields could not be read",
  },
  "agentEnvironment.spendTierNextCharge": {
    message:
      "Уровень доверия {tier}: расходы на модель агентов сначала покрываются кредитами на оплату; с вашей карты списывается сумма, когда остаток расходов достигает {amount}.",
    description: "Trust tier with its mid-month charge point (ADR091 D10.1)",
  },
  "agentEnvironment.spendTierNextChargeCredit": {
    message:
      "Уровень доверия {tier}: кредиты на оплату покрывают первые {credit} расходов на модель агентов в этом расчётном периоде; с вашей карты списывается сумма, когда расходы сверх этого достигают {amount}.",
    description:
      "Trust tier with its charge point when billing credits cover agent model spend first (ADR091 D10.2a). {credit} is the credit counted in this billing period, {amount} the charge point",
  },
  "agentEnvironment.spendTierMonthEnd": {
    message:
      "Уровень доверия {tier}: расходы на модель агентов списываются в конце месяца.",
    description: "Top trust tier, charged only at month end",
  },
  "agentEnvironment.spendTierNew": {
    message:
      "Для новых рабочих пространств плата списывается, когда расходы на модель агентов сверх кредитов на оплату достигают {amount}: с карты списывается сумма сразу, сессии продолжают работать после успешного списания, а лимит повышается до предела вашего тарифа.",
    description:
      "Explains T0: charged at its threshold, working while the charge settles (ADR091 D10.2a)",
  },
  "agentEnvironment.spendTierLocked": {
    message:
      "Неудачный или оспоренный платёж ограничивает это рабочее пространство; урегулируйте его в разделе «Оплата», чтобы снять ограничение.",
    description: "Explains a payment-locked trust tier",
  },
  "agentEnvironment.spendCapLabel": {
    message: "Месячный лимит расходов (USD)",
    description: "Label of the admin-editable cap field",
  },
  "agentEnvironment.spendCapSave": {
    message: "Сохранить лимит",
    description: "Button that saves the cap",
  },
  "agentEnvironment.spendCapReset": {
    message: "Сбросить до значения тарифа",
    description: "Button that returns the cap to the plan default",
  },
  "agentEnvironment.spendCapHelp": {
    message:
      "По умолчанию для тарифа: {plan}. Администраторы могут задать не более {max}.",
    description: "Help under the cap field",
  },
  "agentEnvironment.spendCapInvalid": {
    message: "Введите сумму больше $0 и не более {max}.",
    description: "Client/server refusal of a cap value",
  },
  "agentEnvironment.spendCapSaved": {
    message: "Лимит расходов сохранён",
    description: "Toast after saving the cap",
  },
  "agentEnvironment.spendCapResetDone": {
    message: "Лимит расходов сброшен до значения тарифа",
    description: "Toast after resetting the cap",
  },
  "agentEnvironment.spendCapSaveFailed": {
    message: "Не удалось сохранить лимит расходов. Повторите попытку.",
    description: "Generic cap write failure",
  },
  "agentEnvironment.secrets.title": {
    message: "Секреты агентов",
    description: "Settings → Agent secrets card title (ADR098)",
  },
  "agentEnvironment.secrets.description": {
    message:
      "API-ключи и токены, которые можно передать сессии как переменные окружения. Значения доступны только для записи: прочитать их обратно нельзя, а в транскриптах они скрываются.",
    description: "Agent secrets card description",
  },
  "agentEnvironment.secrets.loadFailed": {
    message: "Не удалось загрузить секреты агентов.",
    description: "Agent secrets card error when the list query fails",
  },
  "agentEnvironment.secrets.empty": {
    message: "Секретов агентов пока нет.",
    description: "Agent secrets card empty state",
  },
  "agentEnvironment.secrets.deleteLabel": {
    message: "Удалить {name}",
    description: "Accessible label of a secret's delete button",
  },
  "agentEnvironment.secrets.deleteTitle": {
    message: "Удалить {name}?",
    description: "Delete-secret confirmation title",
  },
  "agentEnvironment.secrets.deleteBody": {
    message:
      "Сессии, использующие этот секрет, будут завершаться ошибкой на следующем ходе, пока он не будет задан снова.",
    description: "Delete-secret confirmation body",
  },
  "agentEnvironment.secrets.deleteConfirm": {
    message: "Удалить секрет",
    description: "Delete-secret confirmation button",
  },
  "agentEnvironment.secrets.nameLabel": {
    message: "Название секрета",
    description: "Agent secrets card: name field label",
  },
  "agentEnvironment.secrets.namePlaceholder": {
    message: "STRIPE_API_KEY",
    description: "Agent secrets card: name field placeholder (an env var name)",
  },
  "agentEnvironment.secrets.valueLabel": {
    message: "Значение секрета",
    description: "Agent secrets card: value field label",
  },
  "agentEnvironment.secrets.valuePlaceholder": {
    message: "Значение",
    description: "Agent secrets card: value field placeholder",
  },
  "agentEnvironment.secrets.save": {
    message: "Сохранить",
    description: "Agent secrets card: save (create or replace) button",
  },
  "agentEnvironment.secrets.help": {
    message:
      "Сохранение с существующим названием заменяет его значение. Сессии подхватывают изменения со следующего хода.",
    description: "Agent secrets card help line for admins",
  },
  "agentEnvironment.secrets.readOnly": {
    message:
      "Добавлять, изменять и удалять секреты агентов могут только администраторы рабочего пространства.",
    description: "Agent secrets card help line for non-admins",
  },
  "agentEnvironment.secrets.saved": {
    message: "{name} сохранён",
    description: "Toast after a secret is saved",
  },
  "agentEnvironment.secrets.deleted": {
    message: "{name} удалён",
    description: "Toast after a secret is deleted",
  },
  "agentEnvironment.secrets.nameInvalid": {
    message:
      "Используйте буквы, цифры и _; начинайте с буквы или _ (не более 128 символов).",
    description: "Secret name validation error",
  },
  "agentEnvironment.secrets.nameReserved": {
    message:
      "bex сам задаёт эту переменную окружения, поэтому секрет не может использовать это название.",
    description: "Secret name reserved error",
  },
  "agentEnvironment.secrets.valueRequired": {
    message: "Введите значение.",
    description: "Secret value required error",
  },
  "agentEnvironment.secrets.valueInvalid": {
    message: "Значение может занимать не более 32 КиБ.",
    description: "Secret value too long (or contains a NUL byte)",
  },
  "agentEnvironment.secrets.limitExceeded": {
    message: "Достигнут лимит секретов агентов.",
    description: "Secret count or size limit error",
  },
  "agentEnvironment.secrets.notFound": {
    message: "Этого секрета больше нет; возможно, его только что удалили.",
    description: "AGENT_SECRET_NOT_FOUND error (deleted concurrently)",
  },
  "agentEnvironment.secrets.unavailable": {
    message: "Секреты агентов недоступны в этом развёртывании.",
    description: "AGENT_SECRETS_UNAVAILABLE error",
  },
  "agentEnvironment.secrets.forbidden": {
    message:
      "Изменять секреты агентов могут только администраторы рабочего пространства.",
    description: "Secret write refused for a non-admin",
  },
  "agentEnvironment.secrets.saveFailed": {
    message: "Не удалось сохранить изменение. Повторите попытку.",
    description: "Generic secret write failure",
  },
  "agentEnvironment.secrets.valueKeepPlaceholder": {
    message: "Оставить текущее значение",
    description:
      "Agent secrets card: value placeholder when the name is an existing secret (empty keeps its value)",
  },
  "agentEnvironment.secrets.editLabel": {
    message: "Редактировать {name}",
    description:
      "Accessible label of a secret's edit button (loads it into the form to rotate or rescope)",
  },
  "agentEnvironment.secrets.scopeLabel": {
    message: "Область действия",
    description:
      "Agent secrets card: accessible label of the scope picker (ADR098 D9)",
  },
  "agentEnvironment.secrets.scopeWorkspace": {
    message: "Все сессии",
    description:
      "Scope picker option: every session in the workspace receives the secret",
  },
  "agentEnvironment.secrets.scopeRepo": {
    message: "Один репозиторий",
    description:
      "Scope picker option: sessions on one repository receive the secret",
  },
  "agentEnvironment.secrets.scopeExplicit": {
    message: "Только при выборе",
    description:
      "Scope picker option: only sessions that pick the secret receive it",
  },
  "agentEnvironment.secrets.scopeBadgeRepo": {
    message: "Репозиторий: {repo}",
    description:
      "Secret row badge: repo scope, naming the repository (owner/name)",
  },
  "agentEnvironment.secrets.repoLabel": {
    message: "Репозиторий",
    description:
      "Agent secrets card: accessible label of the repo scope's repository field",
  },
  "agentEnvironment.secrets.repoPlaceholder": {
    message: "owner/name",
    description: "Agent secrets card: repository field placeholder",
  },
  "agentEnvironment.secrets.repoEmpty": {
    message: "Подходящих подключённых репозиториев нет.",
    description:
      "Agent secrets card: repository picker with no matching connected repository",
  },
  "agentEnvironment.secrets.repoInvalid": {
    message: "Введите репозиторий в формате owner/name.",
    description:
      "Repo scope validation error (also AGENT_SECRET_SCOPE_INVALID)",
  },
  "agentEnvironment.repoEnvs.title": {
    message: "Окружения репозиториев",
    description: "Settings → Repository environments card title (ADR109)",
  },
  "agentEnvironment.repoEnvs.description": {
    message:
      "Как сессии агентов подготавливают каждый репозиторий. Шаги настройки выполняются по порядку из корня репозитория перед запуском агента каждой сессии, с секретами этой сессии. Проверки — это команды, которые агент запускает перед сдачей работы, например линтер и тесты.",
    description:
      "Repository environments card description: what setup steps and checks are",
  },
  "agentEnvironment.repoEnvs.loadFailed": {
    message: "Не удалось загрузить окружения репозиториев.",
    description: "Repository environments card error when the list query fails",
  },
  "agentEnvironment.repoEnvs.emptyList": {
    message: "Окружений репозиториев пока нет.",
    description: "Repository environments card empty state",
  },
  "agentEnvironment.repoEnvs.setupSummary": {
    message: "Настройка: {names}",
    description:
      "Repository row summary: the setup step names, comma-separated",
  },
  "agentEnvironment.repoEnvs.checksSummary": {
    message: "Проверки: {names}",
    description: "Repository row summary: the check names, comma-separated",
  },
  "agentEnvironment.repoEnvs.noSetup": {
    message: "Нет шагов настройки",
    description:
      "Repository row summary (and the read-only dialog) when the environment has no setup steps",
  },
  "agentEnvironment.repoEnvs.noChecks": {
    message: "Нет проверок",
    description:
      "Repository row summary (and the read-only dialog) when the environment has no checks",
  },
  "agentEnvironment.repoEnvs.editLabel": {
    message: "Редактировать {repo}",
    description:
      "Accessible label of a repository environment's edit button (repo is owner/name)",
  },
  "agentEnvironment.repoEnvs.viewLabel": {
    message: "Просмотреть {repo}",
    description:
      "Accessible label of a repository environment's view button, for non-admins",
  },
  "agentEnvironment.repoEnvs.deleteLabel": {
    message: "Удалить окружение для {repo}",
    description: "Accessible label of a repository environment's delete button",
  },
  "agentEnvironment.repoEnvs.deleteTitle": {
    message: "Удалить окружение для {repo}?",
    description: "Delete-environment confirmation title",
  },
  "agentEnvironment.repoEnvs.deleteBody": {
    message:
      "Со следующего хода сессии в этом репозитории будут запускаться без его шагов настройки и проверок.",
    description: "Delete-environment confirmation body",
  },
  "agentEnvironment.repoEnvs.deleteConfirm": {
    message: "Удалить окружение",
    description: "Delete-environment confirmation button",
  },
  "agentEnvironment.repoEnvs.add": {
    message: "Добавить окружение",
    description:
      "Repository environments card: button that opens the add-environment dialog",
  },
  "agentEnvironment.repoEnvs.help": {
    message:
      "Сохранение заменяет всё окружение репозитория целиком. Сессии подхватывают изменения со следующего хода.",
    description: "Repository environments card help line for admins",
  },
  "agentEnvironment.repoEnvs.readOnly": {
    message:
      "Добавлять, изменять и удалять окружения репозиториев могут только администраторы рабочего пространства.",
    description: "Repository environments card help line for non-admins",
  },
  "agentEnvironment.repoEnvs.saved": {
    message: "Окружение для {repo} сохранено",
    description: "Toast after an environment is saved",
  },
  "agentEnvironment.repoEnvs.deleted": {
    message: "Окружение для {repo} удалено",
    description: "Toast after an environment is deleted",
  },
  "agentEnvironment.repoEnvs.createTitle": {
    message: "Добавление окружения репозитория",
    description: "Add-environment dialog title",
  },
  "agentEnvironment.repoEnvs.editTitle": {
    message: "Редактирование {repo}",
    description: "Edit-environment dialog title (repo is owner/name)",
  },
  "agentEnvironment.repoEnvs.dialogDescription": {
    message:
      "Шаги настройки выполняются по порядку из корня репозитория перед запуском агента каждой сессии, с секретами этой сессии. Если шаг завершается ошибкой, ход заканчивается до запуска агента. Проверки — это то, что агент запускает перед сдачей работы.",
    description: "Environment dialog description",
  },
  "agentEnvironment.repoEnvs.repoLabel": {
    message: "Репозиторий",
    description: "Environment dialog: repository field label",
  },
  "agentEnvironment.repoEnvs.repoPlaceholder": {
    message: "owner/name",
    description: "Environment dialog: repository field placeholder",
  },
  "agentEnvironment.repoEnvs.repoEmpty": {
    message: "Подходящих подключённых репозиториев нет.",
    description:
      "Environment dialog: repository picker with no matching connected repository",
  },
  "agentEnvironment.repoEnvs.setupTitle": {
    message: "Шаги настройки",
    description: "Environment dialog: setup steps section heading",
  },
  "agentEnvironment.repoEnvs.setupHint": {
    message:
      "Установите инструменты и зависимости. Каждый шаг выполняется через bash по порядку; не более 10 шагов, по 10 минут на каждый.",
    description: "Environment dialog: setup steps section hint",
  },
  "agentEnvironment.repoEnvs.checksTitle": {
    message: "Проверки",
    description: "Environment dialog: checks section heading",
  },
  "agentEnvironment.repoEnvs.checksHint": {
    message:
      "Команды вроде линтера, тестов и сборки. Агент запускает каждую из корня репозитория перед сдачей работы и сообщает результаты; сам bex их не запускает. Не более 10.",
    description: "Environment dialog: checks section hint",
  },
  "agentEnvironment.repoEnvs.addSetup": {
    message: "Добавить шаг настройки",
    description: "Environment dialog: add a setup step row",
  },
  "agentEnvironment.repoEnvs.addCheck": {
    message: "Добавить проверку",
    description: "Environment dialog: add a check row",
  },
  "agentEnvironment.repoEnvs.setupNameLabel": {
    message: "Название шага настройки {position}",
    description:
      "Accessible label of a setup step's name field (position is its 1-based place in the list)",
  },
  "agentEnvironment.repoEnvs.setupRunLabel": {
    message: "Команда шага настройки {position}",
    description: "Accessible label of a setup step's command field",
  },
  "agentEnvironment.repoEnvs.setupMoveUp": {
    message: "Переместить шаг настройки {position} выше",
    description: "Accessible label: move a setup step one place earlier",
  },
  "agentEnvironment.repoEnvs.setupMoveDown": {
    message: "Переместить шаг настройки {position} ниже",
    description: "Accessible label: move a setup step one place later",
  },
  "agentEnvironment.repoEnvs.setupRemove": {
    message: "Удалить шаг настройки {position}",
    description: "Accessible label: remove a setup step row",
  },
  "agentEnvironment.repoEnvs.checkNameLabel": {
    message: "Название проверки {position}",
    description:
      "Accessible label of a check's name field (position is its 1-based place in the list)",
  },
  "agentEnvironment.repoEnvs.checkRunLabel": {
    message: "Команда проверки {position}",
    description: "Accessible label of a check's command field",
  },
  "agentEnvironment.repoEnvs.checkMoveUp": {
    message: "Переместить проверку {position} выше",
    description: "Accessible label: move a check one place earlier",
  },
  "agentEnvironment.repoEnvs.checkMoveDown": {
    message: "Переместить проверку {position} ниже",
    description: "Accessible label: move a check one place later",
  },
  "agentEnvironment.repoEnvs.checkRemove": {
    message: "Удалить проверку {position}",
    description: "Accessible label: remove a check row",
  },
  "agentEnvironment.repoEnvs.setupNamePlaceholder": {
    message: "deps",
    description: "Setup step name placeholder (an example step name)",
  },
  "agentEnvironment.repoEnvs.setupRunPlaceholder": {
    message: "npm ci",
    description: "Setup step command placeholder (an example shell command)",
  },
  "agentEnvironment.repoEnvs.checkNamePlaceholder": {
    message: "test",
    description: "Check name placeholder (an example check name)",
  },
  "agentEnvironment.repoEnvs.checkRunPlaceholder": {
    message: "npm test",
    description: "Check command placeholder (an example shell command)",
  },
  "agentEnvironment.repoEnvs.close": {
    message: "Закрыть",
    description: "Read-only environment dialog: close button",
  },
  "agentEnvironment.repoEnvs.cancel": {
    message: "Отменить",
    description: "Environment dialog: cancel button",
  },
  "agentEnvironment.repoEnvs.save": {
    message: "Сохранить",
    description: "Environment dialog: save (create or replace) button",
  },
  "agentEnvironment.repoEnvs.nameRequired": {
    message: "Введите название.",
    description: "Step or check name required error",
  },
  "agentEnvironment.repoEnvs.nameInvalid": {
    message:
      "Название начинается с буквы или цифры и состоит из букв, цифр, пробелов, «.», «_» или «-» (не более 64 символов).",
    description: "Step or check name format error",
  },
  "agentEnvironment.repoEnvs.nameDuplicate": {
    message: "Другая строка в этом списке уже использует это название.",
    description: "Step or check name repeated within its list",
  },
  "agentEnvironment.repoEnvs.runRequired": {
    message: "Введите команду.",
    description: "Step or check command required error",
  },
  "agentEnvironment.repoEnvs.setupRunTooLong": {
    message: "Команда настройки может занимать не более 8 КиБ.",
    description: "Setup step command too long (or contains a NUL byte)",
  },
  "agentEnvironment.repoEnvs.checkRunTooLong": {
    message: "Команда проверки может занимать не более 2 КиБ.",
    description: "Check command too long (or contains a NUL byte)",
  },
  "agentEnvironment.repoEnvs.setupTooMany": {
    message: "В окружении может быть не более 10 шагов настройки.",
    description: "Too many setup steps",
  },
  "agentEnvironment.repoEnvs.checksTooMany": {
    message: "В окружении может быть не более 10 проверок.",
    description: "Too many checks",
  },
  "agentEnvironment.repoEnvs.environmentEmpty": {
    message:
      "Добавьте хотя бы один шаг настройки или проверку. Чтобы удалить их все, удалите окружение целиком.",
    description: "Environment with neither setup steps nor checks",
  },
  "agentEnvironment.repoEnvs.repoInvalid": {
    message: "Введите репозиторий в формате owner/name.",
    description:
      "Repository validation error (also AGENT_REPO_ENVIRONMENT_INVALID on the repository)",
  },
  "agentEnvironment.repoEnvs.repoExists": {
    message:
      "У этого репозитория уже есть окружение. Отредактируйте его в списке.",
    description: "Adding an environment for a repository that already has one",
  },
  "agentEnvironment.repoEnvs.setupRefused": {
    message:
      "Шаг настройки отклонён. Проверьте, что каждое название допустимо и уникально, а каждая команда не превышает 8 КиБ.",
    description: "AGENT_REPO_ENVIRONMENT_INVALID on the setup steps",
  },
  "agentEnvironment.repoEnvs.checksRefused": {
    message:
      "Проверка отклонена. Проверьте, что каждое название допустимо и уникально, а каждая команда не превышает 2 КиБ.",
    description: "AGENT_REPO_ENVIRONMENT_INVALID on the checks",
  },
  "agentEnvironment.repoEnvs.invalid": {
    message:
      "Это окружение отклонено. Проверьте его репозиторий, шаги настройки и проверки.",
    description: "AGENT_REPO_ENVIRONMENT_INVALID without a recognized field",
  },
  "agentEnvironment.repoEnvs.limitExceeded": {
    message:
      "Это рабочее пространство достигло лимита в 50 окружений репозиториев. Удалите одно, чтобы добавить другое.",
    description:
      "AGENT_REPO_ENVIRONMENT_LIMIT_EXCEEDED (also checked before saving)",
  },
  "agentEnvironment.repoEnvs.notFound": {
    message:
      "У этого репозитория больше нет окружения; возможно, его только что удалили.",
    description:
      "AGENT_REPO_ENVIRONMENT_NOT_FOUND error (deleted concurrently)",
  },
  "agentEnvironment.repoEnvs.unavailable": {
    message: "Окружения репозиториев недоступны в этом развёртывании.",
    description: "AGENT_REPO_ENVIRONMENTS_UNAVAILABLE error",
  },
  "agentEnvironment.repoEnvs.forbidden": {
    message:
      "Изменять окружения репозиториев могут только администраторы рабочего пространства.",
    description: "Environment write refused for a non-admin",
  },
  "agentEnvironment.repoEnvs.saveFailed": {
    message: "Не удалось сохранить изменение. Повторите попытку.",
    description: "Generic environment write failure",
  },
  "agentEnvironment.repoEnvs.builds.status.queued": {
    message: "В очереди",
    description: "Build status badge: waiting to start (ADR111 D2)",
  },
  "agentEnvironment.repoEnvs.builds.status.building": {
    message: "Сборка",
    description: "Build status badge: the setup steps are running",
  },
  "agentEnvironment.repoEnvs.builds.status.success": {
    message: "Готово",
    description:
      "Build status badge: success — sessions can start from this snapshot",
  },
  "agentEnvironment.repoEnvs.builds.status.failed": {
    message: "Сбой",
    description: "Build status badge: the build failed",
  },
  "agentEnvironment.repoEnvs.builds.status.cancelled": {
    message: "Отменена",
    description:
      "Build status badge: a newer build replaced it, or its environment was deleted",
  },
  "agentEnvironment.repoEnvs.builds.trigger.environmentSaved": {
    message: "Шаги настройки изменены",
    description:
      "What queued a build: the environment was saved with changed setup steps",
  },
  "agentEnvironment.repoEnvs.builds.trigger.secretChanged": {
    message: "Секрет репозитория изменён",
    description: "What queued a build: a repo-scoped secret was set or changed",
  },
  "agentEnvironment.repoEnvs.builds.trigger.manual": {
    message: "Ручная сборка",
    description:
      "What queued a build: an admin clicked Build now (or used the API)",
  },
  "agentEnvironment.repoEnvs.builds.trigger.refresh": {
    message: "Ежедневное обновление",
    description: "What queued a build: the daily rebuild",
  },
  "agentEnvironment.repoEnvs.builds.failure.setupFailed": {
    message: "Один из шагов настройки завершился сбоем.",
    description:
      "Failed build, failureCode setup_failed (the reason below names the step and exit code)",
  },
  "agentEnvironment.repoEnvs.builds.failure.stepTimeout": {
    message: "Один из шагов настройки превысил лимит в 3 часа.",
    description: "Failed build, failureCode step_timeout",
  },
  "agentEnvironment.repoEnvs.builds.failure.buildTimeout": {
    message: "Сборка превысила лимит в 3 часа.",
    description: "Failed build, failureCode build_timeout",
  },
  "agentEnvironment.repoEnvs.builds.failure.checkoutFailed": {
    message: "Не удалось получить ветку репозитория по умолчанию.",
    description: "Failed build, failureCode checkout_failed",
  },
  "agentEnvironment.repoEnvs.builds.failure.storageLimitReached": {
    message:
      "Снимок превысил бы лимит хранилища песочниц этого рабочего пространства.",
    description: "Failed build, failureCode storage_limit_reached",
  },
  "agentEnvironment.repoEnvs.builds.failure.internalError": {
    message:
      "bex не смог завершить сборку на своей стороне. Повторите попытку.",
    description: "Failed build, failureCode internal_error",
  },
  "agentEnvironment.repoEnvs.builds.failure.unknown": {
    message: "Сборка завершилась сбоем.",
    description: "Failed build with no recognized failureCode",
  },
  "agentEnvironment.repoEnvs.builds.title": {
    message: "Сборки {repo}",
    description: "Build log dialog title (repo is owner/name)",
  },
  "agentEnvironment.repoEnvs.builds.openLabel": {
    message: "Сборки {repo}",
    description:
      "Accessible label of a repository row's button that opens its builds and logs",
  },
  "agentEnvironment.repoEnvs.builds.description": {
    message:
      "Сессии запускаются из новейшей успешной сборки, шаги настройки которой совпадают с текущими. Если такой сборки нет, они выполняют шаги настройки в начале каждого хода.",
    description:
      "Build log dialog description: how sessions use snapshots and the fallback (ADR111 D4)",
  },
  "agentEnvironment.repoEnvs.builds.buildNow": {
    message: "Собрать сейчас",
    description:
      "Button that queues a build of the repository's setup steps (admins only)",
  },
  "agentEnvironment.repoEnvs.builds.snapshotNote": {
    message:
      "Файлы, которые записывают шаги настройки, сохраняются в снимке, поэтому не записывайте учётные данные в файлы. Секреты передаются шагам как переменные окружения, которые не сохраняются.",
    description: "Note beside Build now (Devin's warning, ADR111 D6)",
  },
  "agentEnvironment.repoEnvs.builds.readOnly": {
    message:
      "Запускать сборку могут только администраторы рабочего пространства.",
    description: "Shown under a disabled Build now for non-admins",
  },
  "agentEnvironment.repoEnvs.builds.forbidden": {
    message:
      "Запускать сборку могут только администраторы рабочего пространства.",
    description: "Build now refused with FORBIDDEN",
  },
  "agentEnvironment.repoEnvs.builds.nothingToBuild": {
    message:
      "В этом окружении нет шагов настройки, поэтому собирать нечего. Проверкам сборка не нужна.",
    description:
      "Build now on a checks-only environment (also AGENT_REPO_ENVIRONMENT_NOTHING_TO_BUILD)",
  },
  "agentEnvironment.repoEnvs.builds.queued": {
    message: "Сборка {repo} поставлена в очередь",
    description: "Toast after Build now succeeds",
  },
  "agentEnvironment.repoEnvs.builds.rateLimited": {
    message:
      "Этот репозиторий уже запустил 12 сборок за последние 24 часа. Повторите попытку позже.",
    description:
      "AGENT_REPO_ENVIRONMENT_BUILD_RATE_LIMITED without a usable nextBuildAt",
  },
  "agentEnvironment.repoEnvs.builds.rateLimitedUntil": {
    message:
      "Этот репозиторий уже запустил 12 сборок за последние 24 часа. Следующую сборку можно запустить в {time}.",
    description:
      "AGENT_REPO_ENVIRONMENT_BUILD_RATE_LIMITED; time is nextBuildAt in the viewer's local date and time",
  },
  "agentEnvironment.repoEnvs.builds.unavailable": {
    message: "Сборки окружений недоступны в этом развёртывании.",
    description: "AGENT_REPO_ENVIRONMENT_BUILDS_UNAVAILABLE",
  },
  "agentEnvironment.repoEnvs.builds.buildFailed": {
    message: "Не удалось запустить сборку. Повторите попытку.",
    description: "Generic Build now failure",
  },
  "agentEnvironment.repoEnvs.builds.loadFailed": {
    message: "Не удалось загрузить сборки.",
    description: "Build history query failed",
  },
  "agentEnvironment.repoEnvs.builds.none": {
    message: "Сборок пока нет.",
    description: "Build history list with no builds",
  },
  "agentEnvironment.repoEnvs.builds.noneLog": {
    message:
      "Этот репозиторий ещё не собирался. «Собрать сейчас» один раз выполнит его шаги настройки и сохранит результат как снимок.",
    description: "Build log panel when the repository has no builds",
  },
  "agentEnvironment.repoEnvs.builds.historyLabel": {
    message: "История сборок",
    description: "Accessible label of the build history list",
  },
  "agentEnvironment.repoEnvs.builds.logTitle": {
    message: "Лог сборки",
    description: "Build log panel header when no build is selected",
  },
  "agentEnvironment.repoEnvs.builds.logFailed": {
    message: "Не удалось загрузить лог этой сборки.",
    description: "Single build (log) query failed",
  },
  "agentEnvironment.repoEnvs.builds.took": {
    message: "заняла {duration}",
    description:
      "A finished build's wall time, e.g. 'took 3m 12s' (joined with ' · ')",
  },
  "agentEnvironment.repoEnvs.builds.imageRemoved": {
    message: "снимок удалён",
    description:
      "A successful build whose snapshot image was since deleted (older builds keep no image)",
  },
  "agentEnvironment.repoEnvs.builds.notBuilt": {
    message: "Ещё не собрано",
    description: "Repository row: has setup steps but no build yet",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.queued": {
    message:
      "Эта сборка ожидает запуска. Сборки делят с сессиями лимит песочниц агентов рабочего пространства.",
    description: "Build log of a queued build",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.building": {
    message:
      "Шаги настройки выполняются. Их лог появится здесь, когда сборка завершится.",
    description: "Build log of a running build",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.cancelled": {
    message:
      "Эта сборка отменена: её заменила более новая сборка или её окружение было удалено.",
    description: "Build log of a cancelled build",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.failed": {
    message: "Ни один шаг настройки не выполнялся.",
    description:
      "Build log of a build that failed before any step ran (e.g. the checkout failed)",
  },
  "agentEnvironment.repoEnvs.builds.noSteps.other": {
    message: "Для этой сборки не записано ни одного шага.",
    description: "Build log with no steps, any other status",
  },
  "agentEnvironment.repoEnvs.builds.stepExit": {
    message: "код выхода {code}",
    description: "A setup step's exit code badge",
  },
  "agentEnvironment.repoEnvs.builds.stepKilled": {
    message: "остановлен",
    description: "A setup step with no exit code: killed or never started",
  },
  "agentEnvironment.repoEnvs.builds.noOutput": {
    message: "Нет вывода.",
    description: "A setup step that printed nothing",
  },
  "agentEnvironment.skills.title": {
    message: "Навыки агентов",
    description: "Settings → Agent skills card title (ADR099)",
  },
  "agentEnvironment.skills.description": {
    message:
      "Повторно используемые инструкции в виде файлов SKILL.md, которые получает каждая сессия агента в этом рабочем пространстве. Агент применяет навык, когда он подходит к задаче, или только по запросу.",
    description: "Agent skills card description",
  },
  "agentEnvironment.skills.loadFailed": {
    message: "Не удалось загрузить навыки агентов.",
    description: "Agent skills card error when the list query fails",
  },
  "agentEnvironment.skills.empty": {
    message: "Навыков агентов пока нет.",
    description: "Agent skills card empty state",
  },
  "agentEnvironment.skills.onlyWhenAsked": {
    message: "Только по запросу",
    description:
      "Badge on a skill the agent applies only when a prompt invokes it",
  },
  "agentEnvironment.skills.enabledLabel": {
    message: "Включить {name}",
    description: "Accessible label of a skill's enabled switch",
  },
  "agentEnvironment.skills.editLabel": {
    message: "Редактировать {name}",
    description: "Accessible label of a skill's edit button",
  },
  "agentEnvironment.skills.viewLabel": {
    message: "Просмотреть {name}",
    description: "Accessible label of a skill's view button (non-admins)",
  },
  "agentEnvironment.skills.exportLabel": {
    message: "Скачать {name} как SKILL.md",
    description: "Accessible label of a skill's export button",
  },
  "agentEnvironment.skills.deleteLabel": {
    message: "Удалить {name}",
    description: "Accessible label of a skill's delete button",
  },
  "agentEnvironment.skills.deleteTitle": {
    message: "Удалить навык {name}?",
    description: "Delete skill confirm title",
  },
  "agentEnvironment.skills.deleteBody": {
    message:
      "Новые ходы агента перестанут его получать. Уже выполненные сессии сохранят свой транскрипт.",
    description: "Delete skill confirm body",
  },
  "agentEnvironment.skills.deleteConfirm": {
    message: "Удалить навык",
    description: "Delete skill confirm button",
  },
  "agentEnvironment.skills.new": {
    message: "Новый навык",
    description: "Button that opens the create-skill dialog",
  },
  "agentEnvironment.skills.import": {
    message: "Импортировать SKILL.md",
    description: "Button that imports a SKILL.md file",
  },
  "agentEnvironment.skills.help": {
    message:
      "Участники могут использовать навыки; добавлять и изменять их могут только администраторы.",
    description: "Agent skills help line for admins",
  },
  "agentEnvironment.skills.readOnly": {
    message:
      "Добавлять и изменять навыки агентов могут только администраторы рабочего пространства.",
    description: "Agent skills help line for non-admins",
  },
  "agentEnvironment.skills.createTitle": {
    message: "Новый навык",
    description: "Create-skill dialog title",
  },
  "agentEnvironment.skills.editTitle": {
    message: "Редактирование {name}",
    description: "Edit-skill dialog title",
  },
  "agentEnvironment.skills.viewTitle": {
    message: "{name}",
    description: "View-skill dialog title (read-only)",
  },
  "agentEnvironment.skills.dialogDescription": {
    message:
      "Агент читает описание, чтобы решить, когда применять навык, и следует инструкциям, когда он применяется.",
    description: "Skill dialog description",
  },
  "agentEnvironment.skills.nameLabel": {
    message: "Название",
    description: "Skill name field label",
  },
  "agentEnvironment.skills.namePlaceholder": {
    message: "run-integration-tests",
    description: "Skill name field placeholder",
  },
  "agentEnvironment.skills.descriptionLabel": {
    message: "Описание",
    description: "Skill description field label",
  },
  "agentEnvironment.skills.descriptionPlaceholder": {
    message: "Когда использовать этот навык, одной строкой",
    description: "Skill description field placeholder",
  },
  "agentEnvironment.skills.onlyWhenAskedLabel": {
    message: "Только по моему запросу",
    description: "Toggle: the agent applies the skill only when invoked",
  },
  "agentEnvironment.skills.onlyWhenAskedHint": {
    message: "Вызывайте его из поля ввода или через @skills:name в запросе.",
    description: "Hint under the only-when-asked toggle",
  },
  "agentEnvironment.skills.bodyLabel": {
    message: "Инструкции",
    description: "Skill body field label",
  },
  "agentEnvironment.skills.bodyPlaceholder": {
    message: "Шаги, которым следует агент, в формате Markdown.",
    description: "Skill body field placeholder",
  },
  "agentEnvironment.skills.contentLabel": {
    message: "SKILL.md",
    description: "Whole-file editor label",
  },
  "agentEnvironment.skills.editAsFile": {
    message: "Редактировать как SKILL.md",
    description: "Switch the dialog to the whole-file editor",
  },
  "agentEnvironment.skills.editAsFields": {
    message: "Редактировать по полям",
    description: "Switch the dialog back to separate fields",
  },
  "agentEnvironment.skills.history.tab": {
    message: "История версий",
    description: "Skill dialog tab listing the saved versions",
  },
  "agentEnvironment.skills.history.editTab": {
    message: "Редактирование",
    description: "Skill dialog tab with the editing form",
  },
  "agentEnvironment.skills.history.current": {
    message: "Текущая",
    description: "Badge on the newest saved version",
  },
  "agentEnvironment.skills.history.changes": {
    message: "Изменения",
    description: "Version view: line diff against the previous version",
  },
  "agentEnvironment.skills.history.content": {
    message: "Содержимое",
    description: "Version view: the full SKILL.md",
  },
  "agentEnvironment.skills.history.load": {
    message: "Загрузить в редактор",
    description: "Button that fills the editor with the selected version",
  },
  "agentEnvironment.skills.history.loadMore": {
    message: "Показать ещё",
    description: "Button that loads the next page of versions",
  },
  "agentEnvironment.skills.history.noChanges": {
    message: "В этой версии ничего не изменилось.",
    description: "Changes view of a version identical to the previous one",
  },
  "agentEnvironment.skills.history.loadFailed": {
    message: "Не удалось загрузить историю версий.",
    description: "Version history list query failed",
  },
  "agentEnvironment.skills.history.versionFailed": {
    message: "Не удалось загрузить эту версию.",
    description: "Single version query failed",
  },
  "agentEnvironment.skills.history.loadedNotice": {
    message:
      "Загружена версия {version}. Сохраните, чтобы оставить её как новую версию.",
    description: "Notice in the editor after Load into editor",
  },
  "agentEnvironment.skills.viewTab": {
    message: "Просмотр",
    description: "Read-only skill dialog tab showing the skill",
  },
  "agentEnvironment.skills.save": {
    message: "Сохранить навык",
    description: "Skill dialog save button",
  },
  "agentEnvironment.skills.cancel": {
    message: "Отменить",
    description: "Skill dialog cancel button",
  },
  "agentEnvironment.skills.close": {
    message: "Закрыть",
    description: "Skill dialog close button (read-only)",
  },
  "agentEnvironment.skills.saved": {
    message: "{name} сохранён",
    description: "Toast after a skill is saved",
  },
  "agentEnvironment.skills.deleted": {
    message: "{name} удалён",
    description: "Toast after a skill is deleted",
  },
  "agentEnvironment.skills.readFailed": {
    message: "Не удалось открыть навык. Повторите попытку.",
    description: "Error when a single skill read fails",
  },
  "agentEnvironment.skills.importFailed": {
    message: "Не удалось прочитать этот файл.",
    description: "Error when an imported file can't be read",
  },
  "agentEnvironment.skills.nameRequired": {
    message: "Введите название.",
    description: "Skill name empty",
  },
  "agentEnvironment.skills.nameInvalid": {
    message:
      "Используйте строчные буквы, цифры и одиночные дефисы, до 64 символов.",
    description: "Skill name fails the Agent Skills format",
  },
  "agentEnvironment.skills.descriptionRequired": {
    message: "Введите описание в одну строку.",
    description: "Skill description empty",
  },
  "agentEnvironment.skills.descriptionInvalid": {
    message: "Описание должно быть одной строкой не длиннее 1024 символов.",
    description: "Skill description too long or multi-line",
  },
  "agentEnvironment.skills.bodyInvalid": {
    message: "Инструкции ограничены 64 КиБ.",
    description: "Skill body too large",
  },
  "agentEnvironment.skills.invalid": {
    message: "Навык недопустим. Проверьте его название, описание и инструкции.",
    description: "AGENT_SKILL_INVALID error",
  },
  "agentEnvironment.skills.limitExceeded": {
    message:
      "Достигнут лимит навыков агентов (100 на рабочее пространство, 1 МиБ включённых в сумме).",
    description: "AGENT_SKILL_LIMIT_EXCEEDED error",
  },
  "agentEnvironment.skills.conflict": {
    message:
      "Этот навык изменился после того, как вы его открыли, или название занято. Закройте и откройте его снова, чтобы увидеть актуальную версию.",
    description: "AGENT_SKILL_CONFLICT error",
  },
  "agentEnvironment.skills.notFound": {
    message: "Этого навыка больше нет.",
    description: "AGENT_SKILL_NOT_FOUND error",
  },
  "agentEnvironment.skills.unavailable": {
    message: "Навыки агентов недоступны в этом развёртывании.",
    description: "AGENT_SKILLS_UNAVAILABLE error",
  },
  "agentEnvironment.skills.forbidden": {
    message:
      "Изменять навыки агентов могут только администраторы рабочего пространства.",
    description: "Skill write refused for a non-admin",
  },
  "agentEnvironment.skills.saveFailed": {
    message: "Не удалось сохранить изменение. Повторите попытку.",
    description: "Generic skill write failure",
  },
  "agentEnvironment.agentsTitle": {
    message: "Доступные агенты",
    description: "Settings → Agents: availability section heading",
  },
  "agentEnvironment.agentsDescription": {
    message:
      "Выберите агентов, с которыми участники могут запускать сессии. Выполняющиеся сессии продолжат работу, если вы отключите агента.",
    description: "Settings → Agents: availability section description",
  },
  "agentEnvironment.agentsDataUseNote": {
    message:
      "Его поставщик модели может обучаться на данных, которые отправляют сессии.",
    description:
      "Settings → Agents: note under an agent whose provider trains on session data (Muse Code)",
  },
  "agentEnvironment.agentsDataUseTitle": {
    message: "Включить {agent}?",
    description:
      "Confirm title before turning on an agent whose provider trains on session data",
  },
  "agentEnvironment.agentsDataUseBody.muse": {
    message:
      "Этот агент работает на уровне для контрибьюторов Meta. Meta может использовать запросы, код, файлы и контекст, которые отправляют его сессии, а также полученные ответы для обучения своих моделей. Включение фиксирует, что вы как администратор подтверждаете это для рабочего пространства.",
    description: "Confirm body: Muse Code's data-use acknowledgement (ADR108)",
  },
  "agentEnvironment.agentsDataUseConfirm": {
    message: "Принять и включить",
    description: "Confirm button: acknowledge data use and enable the agent",
  },
  "agentEnvironment.agentsSaved": {
    message: "Доступные агенты обновлены",
    description: "Toast after an availability change",
  },
  "agentEnvironment.agentsSaveFailed": {
    message: "Не удалось обновить доступных агентов. Повторите попытку.",
    description: "Error after a failed availability change",
  },
  "agentEnvironment.githubCliLabel": {
    message: "GitHub CLI (gh)",
    description:
      "Label of the switch that gives the workspace's agent sessions gh, the GitHub CLI (w10/m524)",
  },
  "agentEnvironment.githubCliHelp": {
    message:
      "Если выключить, gh станет недоступен во всех сессиях агентов этого рабочего пространства, включая выполняющиеся; git push в ветку сессии и инструменты bex для pull request продолжат работать.",
    description:
      "Help under the GitHub CLI switch: what turning it off does (w10/m524)",
  },
  "agentEnvironment.githubCliTurnedOn": {
    message: "GitHub CLI включён для сессий агентов",
    description:
      "Toast after an admin turns the sessions' GitHub CLI on (w10/m524)",
  },
  "agentEnvironment.githubCliTurnedOff": {
    message: "GitHub CLI выключен для сессий агентов",
    description:
      "Toast after an admin turns the sessions' GitHub CLI off (w10/m524)",
  },
  "agentEnvironment.githubCliSaveFailed": {
    message: "Не удалось изменить настройку GitHub CLI. Повторите попытку.",
    description: "Error after a failed GitHub CLI switch (w10/m524)",
  },
  "agentEnvironment.githubCliMandatoryLabel": {
    message: "Сделать выключение обязательным",
    description:
      "Label of the switch that makes the workspace's GitHub CLI off a floor no automation or session can turn back on (w10/m561)",
  },
  "agentEnvironment.githubCliMandatoryHelp": {
    message:
      "Действует, пока GitHub CLI выключен: тогда ни одна автоматизация и ни одна сессия не смогут снова включить gh.",
    description:
      "Help under the mandatory switch: it applies only while GitHub CLI is off (w10/m561)",
  },
  "agentEnvironment.githubCliMandatoryTurnedOn": {
    message: "Выключение GitHub CLI теперь обязательно",
    description:
      "Toast after an admin makes the workspace's GitHub CLI off mandatory (w10/m561)",
  },
  "agentEnvironment.githubCliMandatoryTurnedOff": {
    message: "Выключение GitHub CLI больше не обязательно",
    description:
      "Toast after an admin makes the workspace's GitHub CLI off no longer mandatory (w10/m561)",
  },
  "agentEnvironment.githubCliMandatorySaveFailed": {
    message:
      "Не удалось изменить обязательность выключения GitHub CLI. Повторите попытку.",
    description: "Error after a failed mandatory switch (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsLabel": {
    message: "GitHub CLI для автоматизаций",
    description:
      "Label of the select that binds the gh of the sessions automations start: inherit, on or off (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsHelp": {
    message:
      "gh в сессиях, которые запускают автоматизации, если автоматизация не задаёт свой. «Наследовать» следует переключателю выше.",
    description:
      "Help under the automations GitHub CLI select: an automation's own binding wins, and inherit follows the workspace switch (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsInherited": {
    message: "Автоматизации теперь наследуют настройку GitHub CLI",
    description:
      "Toast after an admin returns automations to inheriting the workspace's GitHub CLI switch (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsTurnedOn": {
    message: "GitHub CLI включён для автоматизаций",
    description:
      "Toast after an admin turns GitHub CLI on for the sessions automations start (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsTurnedOff": {
    message: "GitHub CLI выключен для автоматизаций",
    description:
      "Toast after an admin turns GitHub CLI off for the sessions automations start (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsSaveFailed": {
    message:
      "Не удалось изменить GitHub CLI для автоматизаций. Повторите попытку.",
    description:
      "Error after a failed automations GitHub CLI choice (w10/m561)",
  },
  "agentEnvironment.githubCliAutomationsLocked": {
    message:
      "Выключение GitHub CLI обязательно, поэтому автоматизации не могут включить gh.",
    description:
      "Mapped message for AGENT_ENVIRONMENT_GITHUB_CLI_LOCKED: automations cannot be bound on under a mandatory off (w10/m561)",
  },
  "agentEnvironment.modelsTitle": {
    message: "Модели, доступные участникам",
    description:
      "Settings → Agents: heading of the list of platform models members can choose (w10/m527)",
  },
  "agentEnvironment.modelsDescription": {
    message:
      "Выберите, какие модели платформы участники могут выбирать для сессий, сообщений и автоматизаций: все модели или только некоторые, по модели или по провайдеру. Сессия или автоматизация, которая уже использует отключённую вами модель, со следующего хода перейдёт на модель по умолчанию.",
    description:
      "Settings → Agents: description of the model list — what turning a model off does (w10/m527), and that it allows all models (w10/m592) or filters them by model or by provider (w10/m573)",
  },
  "agentEnvironment.modelsDefaultLocked": {
    message:
      "Модель платформы по умолчанию всегда остаётся включённой: на неё переключаются сессии.",
    description:
      "Note under the platform catalog's default model, whose switch is locked on (w10/m527)",
  },
  "agentEnvironment.modelsNew": {
    message: "Новая в каталоге",
    description:
      "Note under a platform model that is off because it joined the catalog after an admin narrowed the list; an admin turns it on with its switch (w10/m568)",
  },
  "agentEnvironment.modelsTurnedOn": {
    message: "Модель {model} включена для участников",
    description:
      "Toast after an admin turns a model on for members; {model} is its id (w10/m527)",
  },
  "agentEnvironment.modelsTurnedOff": {
    message: "Модель {model} отключена для участников",
    description:
      "Toast after an admin turns a model off for members; {model} is its id (w10/m527)",
  },
  "agentEnvironment.modelsInvalid": {
    message:
      "Не удалось обновить модели: модель платформы по умолчанию должна оставаться включённой, а в списке могут быть только модели из каталога платформы. Перезагрузите страницу, чтобы увидеть текущий каталог.",
    description:
      "Server refusal AGENT_ENVIRONMENT_MODELS_INVALID of a model list change (w10/m527)",
  },
  "agentEnvironment.modelsSaveFailed": {
    message:
      "Не удалось обновить модели, доступные участникам. Повторите попытку.",
    description: "Error after a failed model switch (w10/m527)",
  },
  "agentEnvironment.modelsFilterLabel": {
    message: "Как отбирать модели, доступные участникам",
    description:
      "Settings → Agents: accessible name of the choice between every platform model (w10/m592) and filtering them by model or by provider (w10/m573)",
  },
  "agentEnvironment.modelsAll": {
    message: "Все модели",
    description:
      "Settings → Agents: option that lets members choose every platform model, one added to the catalog later included — Devin's “Allow all models”, every workspace's default (w10/m592). Keep it short: the three options share one line on a 390 px phone",
  },
  "agentEnvironment.modelsByModel": {
    message: "По модели",
    description:
      "Settings → Agents: option that filters the platform models members can choose one model at a time (w10/m573)",
  },
  "agentEnvironment.modelsByProvider": {
    message: "По провайдеру",
    description:
      "Settings → Agents: option that filters the platform models members can choose by provider, the prefix of a model id before / (w10/m573)",
  },
  "agentEnvironment.modelsAllHelp": {
    message:
      "Участники могут выбрать любую модель платформы, в том числе модели, которые платформа добавит позже. Чтобы отключить модели, отбирайте по модели или по провайдеру.",
    description:
      "Settings → Agents: help under the choice while it is All models — every platform model, a later one included; filtering by model or by provider turns models off (w10/m592)",
  },
  "agentEnvironment.modelsByModelHelp": {
    message:
      "Участники могут выбрать только модели, включённые здесь. Модель, которую платформа добавит позже, остаётся отключённой, пока вы её не включите, даже если здесь включены все модели.",
    description:
      "Settings → Agents: help under the choice while it is by model — an allowlist: members pick only the models on, and a model added to the catalog later stays off even while every listed model is on (w10/m568, w10/m573, w10/m592)",
  },
  "agentEnvironment.modelsByProviderHelp": {
    message:
      "Участники могут выбрать любую модель разрешённых провайдеров, в том числе модели, которые платформа добавит позже. Модели других провайдеров остаются отключёнными.",
    description:
      "Settings → Agents: help under the filter choice while it is by provider — every model of an allowed provider, a later one included, and other providers' models off (w10/m573)",
  },
  "agentEnvironment.modelsProviderDefault": {
    message: "{model} (по умолчанию, всегда включена)",
    description:
      "Settings → Agents, by provider: the catalog's default model in the list under its provider's switch; {model} is its id (w10/m573)",
  },
  "agentEnvironment.modelsProviderLastLocked": {
    message:
      "Хотя бы один провайдер остаётся разрешённым. Чтобы оставить только модель по умолчанию, отбирайте по модели.",
    description:
      "Settings → Agents, by provider: note under the one provider still allowed, whose switch is locked on (w10/m573)",
  },
  "agentEnvironment.modelsProviderTurnedOn": {
    message: "Модели {provider} включены для участников",
    description:
      "Toast after an admin allows a provider's models for members; {provider} is the provider id, e.g. anthropic (w10/m573)",
  },
  "agentEnvironment.modelsProviderTurnedOff": {
    message: "Модели {provider} отключены для участников",
    description:
      "Toast after an admin stops allowing a provider's models for members; {provider} is the provider id (w10/m573)",
  },
  "agentEnvironment.modelsAllowedAll": {
    message:
      "Теперь участники выбирают из всех моделей, в том числе из тех, что платформа добавит позже",
    description:
      "Toast after an admin switches the model list to All models (w10/m592)",
  },
  "agentEnvironment.modelsFilteredByProvider": {
    message: "Теперь участники выбирают из моделей разрешённых провайдеров",
    description:
      "Toast after an admin switches the model list to filtering by provider (w10/m573)",
  },
  "agentEnvironment.modelsFilteredByModel": {
    message: "Теперь участники выбирают из разрешённых моделей",
    description:
      "Toast after an admin switches the model list to filtering by model, from All models or from by provider (w10/m573, w10/m592)",
  },
  "agentEnvironment.modelsProvidersInvalid": {
    message:
      "Не удалось обновить провайдеров: в списке должен быть хотя бы один провайдер из каталога платформы. Перезагрузите страницу, чтобы увидеть текущий каталог.",
    description:
      "Server refusal AGENT_ENVIRONMENT_MODELS_INVALID of a provider filter change (field enabledProviders, w10/m573)",
  },
  "agentEnvironment.modelsAllowAllInvalid": {
    message:
      "Не удалось изменить модели, доступные участникам: это рабочее пространство больше не использует каталог моделей платформы. Перезагрузите страницу, чтобы увидеть текущие настройки.",
    description:
      "Server refusal AGENT_ENVIRONMENT_MODELS_INVALID of a switch to or from All models (field allowAllModels): the workspace no longer uses the platform model catalog (w10/m592)",
  },
  "agentEnvironment.modelOffOption": {
    message: "{model} (отключена)",
    description:
      "Default model picker: the saved default, which an admin turned off (w10/m527)",
  },
  "agentEnvironment.modelDefaultOffNote": {
    message:
      "Модель {model} отключена, поэтому сессии и автоматизации без указанной модели используют модель платформы по умолчанию {fallback}, пока администратор не выберет включённую модель или снова не включит {model}.",
    description:
      "Note under the default model picker when the saved default is turned off; {fallback} is the platform default that runs instead (w10/m527)",
  },
  "agentEnvironment.modelDefaultRetiredNote": {
    message:
      "Модели {model} больше нет в каталоге платформы, поэтому сессии и автоматизации без указанной модели используют модель платформы по умолчанию {fallback}, пока администратор не выберет другую модель.",
    description:
      "Note under the default model picker when the saved default left the platform catalog; {fallback} is the platform default that runs instead (w10/m527)",
  },
  "agentEnvironment.idleTimeoutTitle": {
    message: "Тайм-аут простоя",
    description: "Heading of the Enterprise idle timeout section (w6/m563)",
  },
  "agentEnvironment.idleTimeoutDescription": {
    message:
      "Сколько времени песочница сессии остаётся запущенной после окончания хода или отключения последнего редактора. Сообщение в закрытую сессию запускает новую песочницу, которая продолжает разговор. Песочницы автоматизаций и сессий, запущенных другой сессией, закрываются сразу после окончания хода.",
    description:
      "Description of the idle timeout section: what it bounds, and that automations' and child sessions' sandboxes close at once",
  },
  "agentEnvironment.idleTimeoutLabel": {
    message: "Минуты",
    description: "Label of the idle timeout's minutes field",
  },
  "agentEnvironment.idleTimeoutSave": {
    message: "Сохранить тайм-аут",
    description: "Button that saves the idle timeout",
  },
  "agentEnvironment.idleTimeoutHelp": {
    message:
      "От {min} до {max} минут. Рабочие пространства без своего значения используют {min}.",
    description: "Help under the idle timeout field; {min} is also the default",
  },
  "agentEnvironment.idleTimeoutInvalid": {
    message: "Введите целое число минут от {min} до {max}.",
    description:
      "Client/server refusal of an idle timeout outside {min}-{max} minutes",
  },
  "agentEnvironment.idleTimeoutEnterpriseOnly": {
    message:
      "Изменять тайм-аут простоя могут только рабочие пространства Enterprise.",
    description:
      "Server refusal of an idle timeout on a workspace no longer on Enterprise",
  },
  "agentEnvironment.idleTimeoutSaved": {
    message: "Тайм-аут простоя: {minutes} мин.",
    description: "Toast after the idle timeout saved",
  },
  "agentEnvironment.idleTimeoutSaveFailed": {
    message: "Не удалось сохранить тайм-аут простоя. Попробуйте ещё раз.",
    description: "Generic idle timeout write failure",
  },
};

export default ruAgentEnvironment;
