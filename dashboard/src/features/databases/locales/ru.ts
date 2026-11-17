import type { TranslationEntry } from "@/i18n";

const ruDatabases: Record<string, TranslationEntry> = {
  "databases.resourceType": {
    message: "База данных",
    description: "Managed Postgres resource type used in document titles",
  },
  "databases.statTotal": {
    message: "Всего баз данных",
    description: "Databases page stat card label",
  },
  "databases.statAvailable": {
    message: "Доступны",
    description: "Databases page stat card label (healthy databases)",
  },
  "databases.statCreating": {
    message: "Создаются",
    description: "Databases page stat card label (provisioning databases)",
  },
  "databases.cardTitle": {
    message: "Базы данных",
    description: "Databases table card title",
  },
  "databases.colName": {
    message: "Название",
    description: "Databases table column header",
  },
  "databases.colStatus": {
    message: "Статус",
    description: "Databases table column header",
  },
  "databases.colPlan": {
    message: "Тариф",
    description: "Databases table column header (instance type / tier)",
  },
  "databases.colVersion": {
    message: "Версия",
    description: "Databases table column header (PostgreSQL major version)",
  },
  "databases.colStorage": {
    message: "Хранилище",
    description: "Databases table column header (disk size)",
  },
  "databases.colCreated": {
    message: "Создана",
    description: "Databases table column header (relative age from createdAt)",
  },
  "databases.colActions": {
    message: "Действия",
    description: "Databases table actions column header (screen-reader only)",
  },
  "databases.statusAvailable": {
    message: "Доступна",
    description: "Database status badge (CNPG cluster healthy)",
  },
  "databases.statusCreating": {
    message: "Создаётся",
    description: "Database status badge (provisioning)",
  },
  "databases.statusRestarting": {
    message: "Перезапускается",
    description:
      "Database status badge (Render's config_restart: an established store restarting after a config change, restart, or resume — not a first provision)",
  },
  "databases.statusUpgrading": {
    message: "Обновляется",
    description: "Database status badge (offline PostgreSQL major upgrade)",
  },
  "databases.statusUnavailable": {
    message: "Недоступна",
    description: "Database status badge (provisioning failed)",
  },
  "databases.statusSuspended": {
    message: "Приостановлена",
    description:
      "Database status badge (hibernated; suspension wins over the status enum)",
  },
  "databases.statusUnknown": {
    message: "Неизвестно",
    description: "Database status badge for an unrecognized status",
  },
  "databases.errorTitle": {
    message: "Не удалось загрузить базы данных",
    description: "Databases list error card title",
  },
  "databases.errorBody": {
    message:
      "Запрос к bex-api не выполнен. Проверьте подключение и повторите попытку.",
    description: "Databases list error card body",
  },
  "databases.emptyTitle": {
    message: "Баз данных пока нет",
    description: "Databases list empty state title",
  },
  "databases.emptyBody": {
    message: "Создайте первую управляемую базу Postgres, и она появится здесь.",
    description: "Databases list empty state body",
  },
  "databases.actionsMenu": {
    message: "Действия для {name}",
    description: "Accessible label for the per-row actions trigger",
  },
  "databases.actionDelete": {
    message: "Удалить",
    description: "Row action: permanently delete the database",
  },
  "databases.deleteConfirmTitle": {
    message: "Удалить {name}?",
    description: "Delete-confirmation dialog title",
  },
  "databases.deleteConfirmBody": {
    message:
      "Это действие нельзя отменить. Вы уверены, что хотите удалить этот инстанс Postgres?",
    description: "Delete-confirmation dialog body (Render-style copy)",
  },
  "databases.deleteConfirmPrompt": {
    message: "Введите {phrase} ниже для подтверждения.",
    description:
      "Body prompt naming the exact 'sudo delete postgres <name>' phrase (rendered bold by SudoCommandField)",
  },
  "databases.deleteCancel": {
    message: "Отмена",
    description: "Delete-confirmation dialog cancel button",
  },
  "databases.deleteConfirm": {
    message: "Удалить базу данных",
    description: "Delete-confirmation dialog confirm button",
  },
  "databases.deleteSuccess": {
    message: "Удаление {name}…",
    description: "Toast after a delete request is accepted",
  },
  "databases.deleteError": {
    message: "Не удалось удалить {name}. Попробуйте ещё раз.",
    description: "Toast when a delete request fails",
  },
  "databases.createButton": {
    message: "Новая база данных",
    description: "Button that opens the create-database dialog",
  },
  "databases.createTitle": {
    message: "Новая база Postgres",
    description: "Create-database dialog title",
  },
  "databases.createDescription": {
    message: "Создайте управляемый инстанс PostgreSQL.",
    description: "Create-database dialog description",
  },
  "databases.fieldName": {
    message: "Название",
    description: "Create-database form field label (database name)",
  },
  "databases.fieldNamePlaceholder": {
    message: "my-database",
    description: "Create-database name input placeholder",
  },
  "databases.fieldNameError": {
    message:
      "Введите название длиной до 100 символов без переводов строк и управляющих символов.",
    description: "Create-database name validation message",
  },
  "databases.physicalNamesTitle": {
    message: "Имена PostgreSQL (необязательно)",
    description: "Create-database advanced physical-name section title",
  },
  "databases.physicalNamesHint": {
    message:
      "Задаются только при создании. Оставьте пустыми, чтобы сгенерировать стабильные имена.",
    description: "Create-database physical-name helper text",
  },
  "databases.fieldDatabaseName": {
    message: "Имя базы данных",
    description: "Create-database physical PostgreSQL database label",
  },
  "databases.fieldDatabaseNamePlaceholder": {
    message: "orders_data",
    description: "Create-database physical PostgreSQL database placeholder",
  },
  "databases.fieldDatabaseUser": {
    message: "Пользователь базы данных",
    description: "Create-database physical PostgreSQL owner-role label",
  },
  "databases.fieldDatabaseUserPlaceholder": {
    message: "orders_owner",
    description: "Create-database physical PostgreSQL owner-role placeholder",
  },
  "databases.fieldPhysicalNameReserved": {
    message: "«{name}» зарезервировано PostgreSQL. Выберите другое имя.",
    description:
      "Create-database / add-user validation for a name the managed PostgreSQL service reserves (postgres, template0/1, streaming_replica, public, none, pg_*, cnpg_*)",
  },
  "databases.fieldPhysicalNameError": {
    message:
      "Используйте до 63 строчных букв, цифр и символов подчёркивания; имя должно начинаться с буквы или подчёркивания.",
    description: "Create-database physical PostgreSQL identifier validation",
  },
  "databases.fieldPlan": {
    message: "Тип инстанса",
    description: "Create-database form field label (plan / tier)",
  },
  "databases.fieldPlanPlaceholder": {
    message: "Выберите тип инстанса",
    description: "Create-database plan select placeholder",
  },
  "databases.fieldVersion": {
    message: "Версия PostgreSQL",
    description: "Create-database form field label (major version)",
  },
  "databases.fieldVersionDefault": {
    message: "По умолчанию (последняя)",
    description: "Create-database version select default option",
  },
  "databases.fieldDisk": {
    message: "Размер диска (ГБ)",
    description: "Create-database form field label (storage size)",
  },
  "databases.fieldDiskFixed": {
    message: "У этого тарифа фиксированный диск {size} ГБ.",
    description:
      "Create-database hint under a plan's fixed disk size, e.g. Free (w4/376)",
  },
  "databases.fieldDiskError": {
    message: "Введите целое число от {min} до {max} ГБ.",
    description:
      "Create-database validation message when the disk size is below the plan floor or above the volume cap",
  },
  "databases.fieldPublic": {
    message: "Публичный доступ",
    description: "Create-database form field label (external endpoint toggle)",
  },
  "databases.fieldPublicHint": {
    message:
      "Принимает подключения с верными учётными данными с любого IP-адреса, пока вы не ограничите их в разделе «Контроль доступа».",
    description:
      "Create-database public toggle helper text; the toggle starts on, as a Render database is reachable by default (w4/553)",
  },
  "databases.fieldPublicHintOff": {
    message:
      "Внешние подключения заблокированы. Сервисы в частной сети по-прежнему могут подключаться.",
    description:
      "Create-database public toggle helper text once switched off: the database is created private (w4/553)",
  },
  "databases.fieldHighAvailability": {
    message: "Высокая доступность",
    description: "Create-database high availability toggle label (w2/m171)",
  },
  "databases.fieldHighAvailabilityHint": {
    message:
      "Запускает резервный инстанс на другом узле, который берёт нагрузку на себя при сбое основного. Резервный инстанс оплачивается так же, как основной.",
    description:
      "Create-database HA toggle helper text on a plan that offers HA",
  },
  "databases.fieldHighAvailabilityPlanUnsupported": {
    message:
      "{name} не поддерживает высокую доступность. Выберите тариф минимум с 1 CPU.",
    description:
      "Create-database HA toggle helper text when the selected plan is below 1 CPU (Render's rule)",
  },
  "databases.createCancel": {
    message: "Отмена",
    description: "Create-database dialog cancel button",
  },
  "databases.createSubmit": {
    message: "Создать базу данных",
    description: "Create-database dialog submit button",
  },
  "databases.createSuccess": {
    message: "Создание {name}…",
    description:
      "Toast after a create request is accepted (provisioning is async)",
  },
  "databases.createError": {
    message: "Не удалось создать {name}. Попробуйте ещё раз.",
    description: "Toast when a create request fails",
  },
  "databases.capLimitTitle": {
    message: "Достигнут лимит Postgres",
    description:
      "Alert title when the workspace's Postgres creation cap is hit (w7/m9)",
  },
  "databases.capLimitUpgrade": {
    message: "Перейти на другой тариф",
    description:
      "Upgrade CTA button inside the Postgres cap-limit Alert (w7/m9)",
  },
  "databases.metaTitle": {
    message: "Сведения",
    description: "Database detail metadata card title",
  },
  "databases.statusReasonClusterFailed": {
    message:
      "Кластер базы данных отклонил конфигурацию, поэтому последнее изменение не удалось применить.",
    description:
      "Why an unavailable database is unavailable: the operator's ClusterFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonClusterReadFailed": {
    message: "Не удалось прочитать состояние кластера базы данных.",
    description:
      "Why an unavailable database is unavailable: the operator's ClusterReadFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonNetworkPolicyFailed": {
    message: "Не удалось применить сетевую политику базы данных.",
    description:
      "Why an unavailable database is unavailable: the operator's NetworkPolicyFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonDiskAutoscalingFailed": {
    message: "Не удалось применить автомасштабирование диска.",
    description:
      "Why an unavailable database is unavailable: the operator's DiskAutoscalingFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonExportFailed": {
    message: "Не удалось запустить экспорт базы данных.",
    description:
      "Why an unavailable database is unavailable: the operator's ExportFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonPoolerFailed": {
    message: "Не удалось создать пулер подключений.",
    description:
      "Why an unavailable database is unavailable: the operator's PoolerFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonExtensionsFailed": {
    message:
      "Не удалось установить управляемые расширения базы данных, поэтому она пока недоступна.",
    description:
      "Why an unavailable database is unavailable: the operator's ExtensionsFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonPostUpgradeBackupFailed": {
    message: "Не удалось создать резервную копию после обновления версии.",
    description:
      "Why an unavailable database is unavailable: the operator's PostUpgradeBackupFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonMajorVersionUpgradeFailed": {
    message:
      "Обновление основной версии не удалось; база данных остаётся на текущей версии.",
    description:
      "Why an unavailable database is unavailable: the operator's MajorVersionUpgradeFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonStorageShrinkRejected": {
    message:
      "Хранилище Postgres можно только увеличивать: диск не может быть меньше выделенных {size} ГБ.",
    description:
      "Why an unavailable database is unavailable: the operator's StorageShrinkRejected Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonRecoveryUnavailable": {
    message:
      "Базу данных нельзя восстановить: хранилище резервных копий не настроено.",
    description:
      "Why an unavailable database is unavailable: the operator's RecoveryUnavailable Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonBackupStoreUnavailable": {
    message: "Не удалось настроить хранилище резервных копий базы данных.",
    description:
      "Why an unavailable database is unavailable: the operator's BackupStoreUnavailable Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonScheduledBackupFailed": {
    message: "Не удалось запланировать ежедневное резервное копирование.",
    description:
      "Why an unavailable database is unavailable: the operator's ScheduledBackupFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonScheduledBackupCleanupFailed": {
    message:
      "Не удалось удалить расписание ежедневного резервного копирования.",
    description:
      "Why an unavailable database is unavailable: the operator's ScheduledBackupCleanupFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.metaStatus": {
    message: "Статус",
    description: "Database detail metadata row label",
  },
  "databases.metaPlan": {
    message: "Тип инстанса",
    description: "Database detail metadata row label",
  },
  "databases.metaVersion": {
    message: "Версия",
    description: "Database detail metadata row label",
  },
  "databases.versionLabel": {
    message: "PostgreSQL {version}",
    description: "Formatted PostgreSQL major version label",
  },
  "databases.versionUpgradeAction": {
    message: "Обновить",
    description: "Button opening the PostgreSQL major-version upgrade dialog",
  },
  "databases.versionUpgrading": {
    message: "Идёт обновление",
    description:
      "Version-row status while the database is offline for pg_upgrade",
  },
  "databases.versionUpgradeTitle": {
    message: "Обновить PostgreSQL",
    description: "PostgreSQL major-version upgrade dialog title",
  },
  "databases.versionUpgradeDescription": {
    message:
      "Обновите эту базу данных с PostgreSQL {version} до более новой поддерживаемой основной версии.",
    description: "PostgreSQL major-version upgrade dialog description",
  },
  "databases.versionUpgradeTarget": {
    message: "Целевая версия",
    description: "PostgreSQL major-version upgrade target selector label",
  },
  "databases.versionUpgradeDowntime": {
    message:
      "Пока выполняется офлайн-обновление, база данных будет недоступна. Сначала проверьте совместимость и запланируйте простой; большие базы данных могут обновляться до часа.",
    description:
      "Downtime warning in the PostgreSQL major-version upgrade dialog",
  },
  "databases.versionUpgradeErrorTitle": {
    message: "Обновление заблокировано",
    description:
      "Inline backend-guard error title in the version upgrade dialog",
  },
  "databases.versionUpgradeCancel": {
    message: "Отмена",
    description: "PostgreSQL major-version upgrade dialog cancel button",
  },
  "databases.versionUpgradeConfirm": {
    message: "Обновить до PostgreSQL {version}",
    description: "PostgreSQL major-version upgrade dialog confirmation button",
  },
  "databases.versionUpgradeAccepted": {
    message: "Обновление до PostgreSQL {version} запущено.",
    description: "Toast after a PostgreSQL major-version upgrade is accepted",
  },
  "databases.versionUpgradeError": {
    message: "Не удалось запустить обновление PostgreSQL. Попробуйте ещё раз.",
    description: "Fallback error when the PostgreSQL upgrade mutation fails",
  },
  "databases.metaDatabaseName": {
    message: "База данных",
    description: "Database detail metadata row label (the normalized db name)",
  },
  "databases.metaDatabaseUser": {
    message: "Пользователь",
    description: "Database detail metadata row label (owner role)",
  },
  "databases.metaStorage": {
    message: "Хранилище",
    description: "Database detail metadata row label (disk size)",
  },
  "databases.diskAutoscalingLabel": {
    message: "Автомасштабирование диска",
    description: "Label beside the database disk-usage chart toggle",
  },
  "databases.diskAutoscalingSize": {
    message: "Сейчас {current} ГБ · максимум {cap} ГБ",
    description: "Current and maximum Postgres disk size beside autoscaling",
  },
  "databases.diskAutoscalingHint": {
    message:
      "При заполнении на 90% хранилище увеличивается на 50% с округлением вверх до 5 ГБ. Увеличение необратимо и выполняется не чаще одного раза в 12 часов.",
    description: "Accessible explanation of Postgres disk autoscaling",
  },
  "databases.diskAutoscalingCurrentSize": {
    message: "Сейчас {current} ГБ",
    description: "Current disk size while plan limits are unavailable",
  },
  "databases.diskAutoscalingPlanUnsupported": {
    message:
      "Этот тариф не поддерживает автомасштабирование диска. Существующую настройку можно отключить.",
    description: "Plan restriction beside the disk autoscaling switch",
  },
  "databases.diskAutoscalingEnabled": {
    message: "Автомасштабирование диска включено.",
    description: "Toast after enabling Postgres disk autoscaling",
  },
  "databases.diskAutoscalingDisabled": {
    message: "Автомасштабирование диска отключено.",
    description: "Toast after disabling Postgres disk autoscaling",
  },
  "databases.diskAutoscalingError": {
    message:
      "Не удалось обновить автомасштабирование диска. Попробуйте ещё раз.",
    description: "Toast when the disk-autoscaling mutation fails",
  },
  "databases.metaHighAvailability": {
    message: "Высокая доступность",
    description:
      "Database detail metadata row label (HA — single-instance MVP is No)",
  },
  "databases.metaPublic": {
    message: "Публичный доступ",
    description: "Database detail metadata row label (external endpoint)",
  },
  "databases.metaExternalHost": {
    message: "Внешний хост",
    description: "Database detail metadata row label (SNI hostname)",
  },
  "databases.metaRegion": {
    message: "Регион",
    description: "Database detail metadata row label (platform placement)",
  },
  "databases.metaCreated": {
    message: "Создана",
    description: "Database detail metadata row label (relative age)",
  },
  "databases.metaExpires": {
    message: "Истекает",
    description:
      "Details row: when a free database expires (Render's expiresAt, w8/m126)",
  },
  "databases.expirySoon": {
    message:
      "Срок действия этой бесплатной базы данных истекает {expiresAt}, после чего она перестанет работать.",
    description: "Warning within 7 days of a free database's expiry",
  },
  "databases.expiryExpired": {
    message:
      "Срок действия этой бесплатной базы данных истёк, она приостановлена. Она и её данные будут удалены {deleteAt}.",
    description:
      "Warning on an expired free database during its 14-day grace period",
  },
  "databases.expiryUpgrade": {
    message: "Перейти на платный тариф",
    description: "Link from the expiry warning to the Plan section",
  },
  "databases.expiryHintSoon": {
    message: "Скоро истекает",
    description: "List-row hint on a free database within 7 days of expiry",
  },
  "databases.expiryHintExpired": {
    message: "Истекла",
    description: "List-row hint on an expired free database",
  },
  "databases.statusReasonFreeExpired": {
    message:
      "Срок действия этой бесплатной базы данных истёк. Перейдите на платный тариф, чтобы восстановить её.",
    description:
      "Why a suspended database is suspended: its free plan expired (bex-api statusReasonCode POSTGRES_FREE_EXPIRED, w8/m126)",
  },
  "databases.yes": {
    message: "Да",
    description: "Metadata value for a true boolean field",
  },
  "databases.no": {
    message: "Нет",
    description: "Metadata value for a false boolean field",
  },
  "databases.connTitle": {
    message: "Подключения",
    description: "Connection-info panel card title",
  },
  "databases.connDescription": {
    message:
      "Строки подключения и пароль базы данных. Показываются только по запросу, автоматически никогда.",
    description: "Connection-info panel card description",
  },
  "databases.connReveal": {
    message: "Показать данные подключения",
    description: "Button that fetches the connection info on demand",
  },
  "databases.connHide": {
    message: "Скрыть данные подключения",
    description: "Button that clears the revealed connection info",
  },
  "databases.connPassword": {
    message: "Пароль",
    description: "Connection-info field label (database password)",
  },
  "databases.connShowPassword": {
    message: "Показать пароль",
    description: "Accessible label to unmask the password",
  },
  "databases.connHidePassword": {
    message: "Скрыть пароль",
    description: "Accessible label to re-mask the password",
  },
  "databases.connInternal": {
    message: "Внутренняя строка подключения",
    description: "Connection-info field label (in-cluster URL)",
  },
  "databases.connExternal": {
    message: "Внешняя строка подключения",
    description: "Connection-info field label (public URL, TLS)",
  },
  "databases.connPsql": {
    message: "Команда psql",
    description: "Connection-info field label (ready-to-run psql command)",
  },
  "databases.connCaLabel": {
    message: "Сертификат CA сервера",
    description: "Connection-info field label (private server CA bundle)",
  },
  "databases.connCaDownload": {
    message: "Скачать сертификат CA",
    description: "Button downloading the server CA PEM file",
  },
  "databases.connCaHelp": {
    message:
      "Внешние подключения проверяют сертификат этой базы данных по её закрытому серверному CA (sslmode=verify-full по-прежнему обязателен). Скачайте {file}, затем укажите клиенту путь к сохранённому файлу: для команды psql или внешней строки подключения добавьте в начало:",
    description:
      "Explanation above the PGSSLROOTCERT example; {file} is the downloaded CA filename",
  },
  "databases.connCaSnippet": {
    message:
      "Пример префикса переменной окружения, указывающей на скачанный CA",
    description:
      "Accessible label for the copyable PGSSLROOTCERT example snippet",
  },
  "databases.connCertPendingTitle": {
    message: "Публично доверенный сертификат выпускается",
    description:
      "Connections card note title while a public database's Let's Encrypt certificate is still pending (w4/m357)",
  },
  "databases.connCertPendingBody": {
    message:
      "Публично доверенный сертификат выпускается (обычно несколько минут). До этого подключайтесь с CA ниже или с sslmode=require. После выпуска внешнему URL CA не нужен, как на Render; скройте и покажите снова, чтобы обновить.",
    description:
      "Connections card note body while the public database certificate is pending",
  },
  "databases.connCertFailedTitle": {
    message: "Публично доверенный сертификат ещё не выпущен",
    description:
      "Connections card note title when issuing a public database's certificate failed (w4/m357)",
  },
  "databases.connCertFailedBody": {
    message:
      "Не удалось выпустить публично доверенный сертификат этой базы данных, и bex повторяет попытку автоматически. Пока что работает способ с CA: подключайтесь с CA ниже или с sslmode=require.",
    description:
      "Connections card note body when the public database certificate failed to issue",
  },
  "databases.connErrorTitle": {
    message: "Не удалось загрузить данные подключения",
    description: "Connection-info panel error title",
  },
  "databases.connErrorBody": {
    message:
      "Возможно, база данных ещё создаётся или у вас нет прав на просмотр её учётных данных.",
    description: "Connection-info panel error body",
  },
  "databases.copied": {
    message: "Скопировано в буфер обмена",
    description: "Toast after copying a connection field",
  },
  "databases.copyError": {
    message: "Не удалось скопировать в буфер обмена",
    description: "Toast when clipboard copy fails",
  },
  "databases.loading": {
    message: "Загрузка…",
    description: "Generic loading placeholder in a detail panel",
  },
  "databases.dangerDelete": {
    message: "Удалить базу данных",
    description: "Detail-page bottom action: permanently delete the database",
  },
  "databases.dangerRestart": {
    message: "Перезапустить базу данных",
    description: "Detail-page bottom action: rolling restart of the primary",
  },
  "databases.dangerSuspend": {
    message: "Приостановить базу данных",
    description:
      "Detail-page bottom action: hibernate the database (stop compute, keep data)",
  },
  "databases.dangerResume": {
    message: "Возобновить базу данных",
    description: "Detail-page bottom action: wake a suspended database",
  },
  "databases.actionSuspend": {
    message: "Приостановить",
    description: "Row action: hibernate the database (stop compute, keep data)",
  },
  "databases.actionResume": {
    message: "Возобновить",
    description: "Row action: wake a suspended database",
  },
  "databases.actionRestart": {
    message: "Перезапустить",
    description: "Row action: rolling restart of the primary",
  },
  "databases.suspendConfirmTitle": {
    message: "Приостановить {name}?",
    description: "Suspend-confirmation dialog title",
  },
  "databases.suspendConfirmBody": {
    message:
      "Вычисления останавливаются, и база данных становится недоступной, но её данные сохраняются. Возобновите её в любой момент, чтобы вернуть в работу.",
    description: "Suspend-confirmation dialog body",
  },
  "databases.restartConfirmTitle": {
    message: "Перезапустить {name}?",
    description: "Restart-confirmation dialog title",
  },
  "databases.restartConfirmBody": {
    message:
      "Основной инстанс будет перезапущен. Подключения ненадолго прервутся, пока он возвращается в работу.",
    description: "Restart-confirmation dialog body",
  },
  "databases.toastSuspendSuccess": {
    message: "Приостановка {name}…",
    description: "Toast after a suspend request is accepted",
  },
  "databases.toastResumeSuccess": {
    message: "Возобновление {name}…",
    description: "Toast after a resume request is accepted",
  },
  "databases.toastRestartSuccess": {
    message: "Перезапуск {name}…",
    description: "Toast after a restart request is accepted",
  },
  "databases.toastLifecycleError": {
    message: "Не удалось выполнить действие для {name}. Попробуйте ещё раз.",
    description: "Toast when a lifecycle verb fails",
  },
  "databases.recoveryTitle": {
    message: "Восстановление",
    description: "Recovery panel card title",
  },
  "databases.recoveryDescription": {
    message:
      "Восстановление на момент времени и резервные копии. Восстановление всегда создаёт новую базу данных, а эта никогда не изменяется.",
    description: "Recovery panel card description",
  },
  "databases.recoveryDisabled": {
    message:
      "Для этого тарифа резервное копирование не включено, поэтому восстановление недоступно. Перейдите на тариф с резервными копиями, чтобы включить восстановление на момент времени.",
    description: "Recovery panel state when the plan has no backups",
  },
  "databases.recoveryUnavailable": {
    message:
      "Не удалось прочитать данные о восстановлении. Попробуйте ещё раз.",
    description: "Recovery panel state when backup metadata could not be read",
  },
  "databases.recoveryEarliest": {
    message: "Самая ранняя точка восстановления",
    description: "Recovery panel field (earliest recoverable time)",
  },
  "databases.recoveryLatest": {
    message: "Самая поздняя точка восстановления",
    description: "Recovery panel field (latest recoverable time)",
  },
  "databases.recoveryWindowNotEstablished": {
    message: "Ещё не определена",
    description: "Recovery panel value before the PITR window is established",
  },
  "databases.recoveryRestore": {
    message: "Восстановить в новый инстанс",
    description: "Recovery panel button that opens the restore dialog",
  },
  "databases.recoveryRestoreUnavailable": {
    message:
      "Восстановление станет доступно после появления первой точки восстановления.",
    description: "Reason restore is disabled before the PITR window opens",
  },
  "databases.recoveryCreateExport": {
    message: "Создать экспорт",
    description: "Recovery panel button that triggers an on-demand export",
  },
  "databases.recoveryBackups": {
    message: "Резервные копии",
    description: "Recovery panel backup-list heading",
  },
  "databases.recoveryNoBackups": {
    message: "Резервных копий пока нет.",
    description: "Recovery panel empty backup list",
  },
  "databases.recoveryExportScope": {
    message:
      "Экспорт содержит только базу данных {database}. Другие базы данных, созданные в этом инстансе, не включаются.",
    description:
      "Recovery panel note naming the one database a logical export contains (w8/258)",
  },
  "databases.recoveryExports": {
    message: "Экспорты",
    description: "Recovery panel export-list heading",
  },
  "databases.recoveryNoExports": {
    message: "Экспортов пока нет.",
    description: "Recovery panel empty export list",
  },
  "databases.recoveryDownloadExport": {
    message: "Скачать",
    description: "Download an available logical database export",
  },
  "databases.recoveryDownloadUnavailable": {
    message: "Этот экспорт больше недоступен для скачивания.",
    description: "Toast when Download finds the export is no longer available",
  },
  "databases.recoveryDownloadError": {
    message: "Не удалось получить ссылку для скачивания. Повторите попытку.",
    description: "Toast when the download link of an export cannot be fetched",
  },
  "databases.recoveryExportInProgress": {
    message: "Экспорт уже выполняется.",
    description: "Reason the create-export button is disabled",
  },
  "databases.recoveryExportPreparing": {
    message: "Экспорт ещё готовится.",
    description: "Reason an in-progress export cannot be downloaded",
  },
  "databases.recoveryExportExpired": {
    message: "Срок действия этого экспорта истёк, его файл удалён.",
    description: "Reason an expired export cannot be downloaded",
  },
  "databases.recoveryExportUnavailable": {
    message: "Этот экспорт недоступен для скачивания.",
    description: "Fallback reason an export cannot be downloaded",
  },
  "databases.recoveryExportFailure": {
    message: "Экспорт не удался: {reason}",
    description: "Logical export failure reason",
  },
  "databases.recoveryExportStatusCreated": {
    message: "Создан",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusRunning": {
    message: "Выполняется",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusAvailable": {
    message: "Доступен",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusFailed": {
    message: "Не удался",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusExpiring": {
    message: "Скоро истекает",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusExpired": {
    message: "Истёк",
    description: "Logical export status",
  },
  "databases.recoveryRestoreTitle": {
    message: "Восстановить в новую базу данных",
    description: "Restore dialog title",
  },
  "databases.recoveryRestoreBody": {
    message:
      "Создайте новую базу данных, восстановленную из резервных копий этой. Источник остаётся нетронутым.",
    description: "Restore dialog body",
  },
  "databases.recoveryRestoreName": {
    message: "Название новой базы данных",
    description: "Restore dialog field label",
  },
  "databases.recoveryRestoreTime": {
    message: "Момент времени (необязательно)",
    description: "Restore dialog target-time field label",
  },
  "databases.recoveryRestoreTimeHint": {
    message: "Оставьте пустым, чтобы восстановить последнюю доступную точку.",
    description: "Restore dialog target-time helper text",
  },
  "databases.recoveryRestoreTimeZone": {
    message: "Время указано в вашем часовом поясе ({zone}).",
    description: "Restore dialog target-time zone note",
  },
  "databases.recoveryRestoreTimeOutOfWindow": {
    message:
      "Выберите время между самой ранней точкой восстановления и моментом десять минут назад.",
    description:
      "Restore dialog target-time out-of-window error (the window ends ten minutes before now, w7/m453)",
  },
  "databases.recoveryRetentionWindow_one": {
    message:
      "На тарифе {plan} базу можно восстановить на любой момент за последний {count} день.",
    description:
      "Recovery panel hint: how far back the workspace plan may restore a Postgres (w7/m453, from bex-api databaseRecoveryInfo.retention)",
  },
  "databases.recoveryRetentionWindow_few": {
    message:
      "На тарифе {plan} базу можно восстановить на любой момент за последние {count} дня.",
    description:
      "Recovery panel hint: how far back the workspace plan may restore a Postgres (w7/m453, from bex-api databaseRecoveryInfo.retention)",
  },
  "databases.recoveryRetentionWindow_many": {
    message:
      "На тарифе {plan} базу можно восстановить на любой момент за последние {count} дней.",
    description:
      "Recovery panel hint: how far back the workspace plan may restore a Postgres (w7/m453, from bex-api databaseRecoveryInfo.retention)",
  },
  "databases.recoveryRetentionWindow_other": {
    message:
      "На тарифе {plan} базу можно восстановить на любой момент за последние {count} дня.",
    description:
      "Recovery panel hint: how far back the workspace plan may restore a Postgres (w7/m453, from bex-api databaseRecoveryInfo.retention)",
  },
  "databases.recoveryRetentionUpgrade_one": {
    message: "На {plan} — {count} день.",
    description:
      "Recovery panel hint: the cheapest plan that restores further back, and its days (w7/m453)",
  },
  "databases.recoveryRetentionUpgrade_few": {
    message: "На {plan} — {count} дня.",
    description:
      "Recovery panel hint: the cheapest plan that restores further back, and its days (w7/m453)",
  },
  "databases.recoveryRetentionUpgrade_many": {
    message: "На {plan} — {count} дней.",
    description:
      "Recovery panel hint: the cheapest plan that restores further back, and its days (w7/m453)",
  },
  "databases.recoveryRetentionUpgrade_other": {
    message: "На {plan} — {count} дня.",
    description:
      "Recovery panel hint: the cheapest plan that restores further back, and its days (w7/m453)",
  },
  "databases.recoveryRestoreConfirm": {
    message: "Восстановить",
    description: "Restore dialog confirm button",
  },
  "databases.recoveryExportStarted": {
    message: "Экспорт запущен.",
    description: "Toast after an export is triggered",
  },
  "databases.recoveryExportError": {
    message: "Не удалось запустить экспорт. Попробуйте ещё раз.",
    description: "Toast when an export request fails",
  },
  "databases.recoveryRestoreStarted": {
    message: "Восстановление в {name}…",
    description: "Toast after a restore request is accepted",
  },
  "databases.recoveryRestoreError": {
    message: "Не удалось выполнить восстановление. Попробуйте ещё раз.",
    description: "Toast when a restore request fails",
  },
  "databases.accessTitle": {
    message: "Контроль доступа",
    description: "Access-control panel card title",
  },
  "databases.accessDescription": {
    message:
      "Ограничивайте внешнюю точку доступа, настраивайте пул подключений и управляйте пользователями базы данных.",
    description: "Access-control panel card description",
  },
  "databases.accessAllowList": {
    message: "Список разрешённых IP",
    description:
      "Access panel section heading (external-endpoint CIDR allowlist)",
  },
  "databases.accessAllowListHint": {
    message:
      "Сохранение правил CIDR включает внешний доступ для подходящих исходных IP. Сохранение пустого списка блокирует внешние подключения. Внутренний доступ не меняется.",
    description: "Access panel IP allowlist helper text",
  },
  "databases.accessAllowListInternalOnly": {
    message:
      "Внешние подключения отключены. Сохраните одно или несколько правил IP, чтобы включить доступ.",
    description: "External access is disabled; saving rules enables it",
  },
  "databases.accessAllowListEmpty": {
    message:
      "Правил IP нет. Сохраните пустой список, чтобы заблокировать внешние подключения.",
    description: "Empty datastore allowlist draft and the effect of saving it",
  },
  "databases.accessAllowListAdd": {
    message: "Добавить",
    description: "Access panel button to add a CIDR to the draft allowlist",
  },
  "databases.accessAllowListDescription": {
    message: "Описание (необязательно)",
    description:
      "Placeholder for the optional per-entry allowlist description input",
  },
  "databases.accessAllowListCIDR": {
    message: "Новый блок CIDR",
    description: "Accessible label for the add-row CIDR input",
  },
  "databases.accessAllowListCIDRRule": {
    message: "Блок CIDR для правила {number}",
    description: "Accessible label for an existing allowlist CIDR row",
  },
  "databases.accessAllowListDescriptionLabel": {
    message: "Описание нового правила",
    description: "Accessible label for the add-row description input",
  },
  "databases.accessAllowListDescriptionRule": {
    message: "Описание для правила {number}",
    description: "Accessible label for an existing allowlist description row",
  },
  "databases.accessAllowListRemove": {
    message: "Удалить {cidr}",
    description: "Accessible label to remove a CIDR chip",
  },
  "databases.accessAllowListMoveUp": {
    message: "Переместить {cidr} выше",
    description: "Accessible label to move an allowlist entry earlier",
  },
  "databases.accessAllowListMoveDown": {
    message: "Переместить {cidr} ниже",
    description: "Accessible label to move an allowlist entry later",
  },
  "databases.accessAllowListInvalid": {
    message: "Введите уникальные корректные блоки CIDR IPv4 или IPv6.",
    description: "Validation error for database allowlist entries",
  },
  "databases.accessAllowListSave": {
    message: "Сохранить список",
    description: "Access panel button to persist the allowlist",
  },
  "databases.accessAllowListSaved": {
    message: "Список обновлён.",
    description: "Toast after saving the IP allowlist",
  },
  "databases.accessAllowListError": {
    message: "Не удалось сохранить список. Попробуйте ещё раз.",
    description: "Toast when saving the allowlist fails",
  },
  "databases.accessCredentials": {
    message: "Учётные данные",
    description:
      "Access panel section heading (Postgres credentials, Render's Credentials section)",
  },
  "databases.accessCredentialsHint": {
    message:
      "Добавьте новые учётные данные по умолчанию, чтобы сменить пароль базы данных. Данные подключения переключатся на них, а связанные сервисы подхватят их при следующей синхронизации Blueprint. Старые учётные данные работают, пока вы их не удалите.",
    description: "Access panel credentials helper text",
  },
  "databases.accessCredentialDefault": {
    message: "По умолчанию",
    description: "Badge on the database's default credential",
  },
  "databases.accessCredentialAdd": {
    message: "Новые учётные данные по умолчанию",
    description:
      "Access panel button to create a credential that becomes the default",
  },
  "databases.accessCredentialName": {
    message: "Имя пользователя для новых учётных данных",
    description: "Accessible label for the new-credential username input",
  },
  "databases.accessCredentialDelete": {
    message: "Удалить {name}",
    description: "Accessible label to delete a database credential",
  },
  "databases.accessCredentialDeleteTitle": {
    message: "Удалить учётные данные {name}?",
    description:
      "Title of the confirmation before deleting a database credential (w4/551)",
  },
  "databases.accessCredentialDeleteBody": {
    message:
      "Все подключения, использующие {name}, немедленно перестанут работать, включая сервисы, в переменных окружения которых хранится строка подключения с ними. Это действие нельзя отменить.",
    description:
      "Body of the credential delete confirmation: retiring the login breaks its clients at once (w4/551)",
  },
  "databases.accessCredentialDeleteConfirm": {
    message: "Удалить учётные данные",
    description:
      "Confirm button of the credential delete confirmation (w4/551)",
  },
  "databases.accessCredentialDefaultLocked": {
    message:
      "Учётные данные по умолчанию удалить нельзя. Сначала добавьте новые учётные данные по умолчанию.",
    description: "Why the delete button is disabled on the default credential",
  },
  "databases.accessCredentialCreated": {
    message:
      "{name} теперь используется по умолчанию. Пароль указан в данных подключения.",
    description: "Toast after creating a credential",
  },
  "databases.accessCredentialError": {
    message: "Не удалось создать учётные данные. Попробуйте ещё раз.",
    description: "Toast when creating a credential fails",
  },
  "databases.accessCredentialDeleted": {
    message: "Отзыв {name} запрошен.",
    description:
      "Toast after deleting a credential: the login is retired asynchronously on the next reconcile, so this confirms accepted intent, not a completed revocation",
  },
  "databases.accessCredentialDeleteError": {
    message: "Не удалось удалить {name}. Попробуйте ещё раз.",
    description: "Toast when deleting a credential fails",
  },
  "databases.accessPooler": {
    message: "Пул подключений",
    description: "Access panel section heading (PgBouncer pooler strings)",
  },
  "databases.accessPoolerHint": {
    message:
      "Строки подключения через пул (PgBouncer) для клиентов с большим числом подключений.",
    description: "Access panel pooler helper text",
  },
  "databases.accessPoolerReveal": {
    message: "Показать строки пула",
    description: "Access panel button to fetch the pooled connection strings",
  },
  "databases.accessPoolerDisabled": {
    message:
      "Включите пул подключений выше, чтобы получить строки подключения через пул.",
    description:
      "Access panel shown when no pooler is enabled; points at the Connection Pool switch in the section heading (w2/m307)",
  },
  "databases.accessPoolerInternal": {
    message: "Внутреннее подключение через пул",
    description: "Access panel pooled-string label (in-cluster)",
  },
  "databases.accessPoolerPlanUnsupported": {
    message:
      "Этот тариф не поддерживает пул подключений. Выберите тариф с такой поддержкой, прежде чем включать пулер.",
    description: "Pooler empty state on an unsupported plan",
  },
  "databases.accessPoolerExternal": {
    message: "Внешнее подключение через пул",
    description: "Access panel pooled-string label (public)",
  },
  "databases.accessPoolerReplicaInternal": {
    message: "Внутреннее подключение через пул, реплика для чтения {name}",
    description:
      "Access panel pooled-string label for a read replica's in-cluster pooled connection (w6/m458)",
  },
  "databases.accessPoolerReplicaExternal": {
    message: "Внешнее подключение через пул, реплика для чтения {name}",
    description:
      "Access panel pooled-string label for a read replica's public pooled connection (w6/m458)",
  },
  "databases.accessPoolerError": {
    message: "Не удалось загрузить строки подключения через пул.",
    description: "Toast when revealing pooled strings fails",
  },
  "databases.accessPoolerStarting": {
    message:
      "Пул подключений запускается. Строки подключения через пул появятся здесь, когда PgBouncer будет готов.",
    description:
      "Access panel when pooling is on but the operator has not published the pooler hosts yet (w2/m307)",
  },
  "databases.poolerToggleLabel": {
    message: "Пул подключений",
    description:
      "Accessible label of the Connection Pool switch (Render's Info > Connection Pool, w2/m307)",
  },
  "databases.poolerEnableTitle": {
    message: "Включить пул подключений для {name}?",
    description:
      "Connection-pool enable confirmation title; {name} is the database",
  },
  "databases.poolerDisableTitle": {
    message: "Отключить пул подключений для {name}?",
    description:
      "Connection-pool disable confirmation title; {name} is the database",
  },
  "databases.poolerEnableBody": {
    message:
      "При включении пула подключений база данных перезапускается, что приводит к простою на несколько минут. Строки подключения через пул появятся в этом разделе, когда PgBouncer будет готов.",
    description:
      "Connection-pool enable confirmation body for a database without high availability (Render: enabling restarts the database)",
  },
  "databases.poolerEnableBodyHA": {
    message:
      "При включении пула подключений происходит переключение на резервный инстанс вместо перезапуска базы данных, поэтому открытые подключения ненадолго прервутся. Строки подключения через пул появятся в этом разделе, когда PgBouncer будет готов.",
    description:
      "Connection-pool enable confirmation body when high availability is on (Render: a failover instead of a restart)",
  },
  "databases.poolerDisableBody": {
    message:
      "При отключении пула подключений база данных перезапускается, что приводит к простою на несколько минут. Строки подключения через пул перестанут работать.",
    description:
      "Connection-pool disable confirmation body for a database without high availability",
  },
  "databases.poolerDisableBodyHA": {
    message:
      "При отключении пула подключений происходит переключение на резервный инстанс вместо перезапуска базы данных, поэтому открытые подключения ненадолго прервутся. Строки подключения через пул перестанут работать.",
    description:
      "Connection-pool disable confirmation body when high availability is on",
  },
  "databases.poolerEnableConfirm": {
    message: "Включить пул подключений",
    description: "Connection-pool enable confirmation button (Render's label)",
  },
  "databases.poolerDisableConfirm": {
    message: "Отключить пул подключений",
    description: "Connection-pool disable confirmation button",
  },
  "databases.poolerEnableSuccess": {
    message: "Пул подключений для {name} включается.",
    description: "Toast after the connection-pool enable was accepted",
  },
  "databases.poolerDisableSuccess": {
    message: "Пул подключений для {name} отключается.",
    description: "Toast after the connection-pool disable was accepted",
  },
  "databases.poolerEnableError": {
    message: "Не удалось включить пул подключений для {name}.",
    description: "Fallback toast when enabling the connection pool fails",
  },
  "databases.poolerDisableError": {
    message: "Не удалось отключить пул подключений для {name}.",
    description: "Fallback toast when disabling the connection pool fails",
  },
  "databases.haTitle": {
    message: "Высокая доступность",
    description: "HA panel card title",
  },
  "databases.haDescription": {
    message:
      "Реплицируемый кластер с автоматическим переключением при сбое. Когда функция включена, резервный инстанс всегда готов взять нагрузку на себя.",
    description: "HA panel card description",
  },
  "databases.haStatus": {
    message: "Статус",
    description: "HA panel status row label",
  },
  "databases.haEnabled": {
    message: "Включена",
    description: "HA panel status value when HA is active",
  },
  "databases.haDisabled": {
    message: "Отключена",
    description: "HA panel status value when HA is off",
  },
  "databases.haFailover": {
    message: "Запустить переключение",
    description: "HA panel button to initiate a planned switchover",
  },
  "databases.haFailoverConfirmTitle": {
    message: "Запустить переключение для {name}?",
    description: "Failover-confirmation dialog title",
  },
  "databases.haFailoverConfirmBody": {
    message:
      "Резервный инстанс станет основным. Во время переключения подключения ненадолго прервутся. База данных остаётся доступной, данные не теряются.",
    description: "Failover-confirmation dialog body",
  },
  "databases.haFailoverCancel": {
    message: "Отмена",
    description: "Failover-confirmation dialog cancel button",
  },
  "databases.haFailoverConfirm": {
    message: "Запустить переключение",
    description: "Failover-confirmation dialog confirm button",
  },
  "databases.haFailoverSuccess": {
    message: "Переключение для {name} запущено.",
    description: "Toast after a failover request is accepted",
  },
  "databases.haFailoverError": {
    message:
      "Не удалось запустить переключение для {name}. Попробуйте ещё раз.",
    description: "Toast when a failover request fails",
  },
  "databases.haReadReplicas": {
    message: "Реплики для чтения",
    description: "HA panel section heading for named read replicas",
  },
  "databases.haReadReplicasEmpty": {
    message: "Именованных реплик для чтения нет.",
    description: "HA panel empty state for read replicas",
  },
  "databases.haReplicaInternal": {
    message: "Внутренний",
    description: "HA panel replica connection label (in-cluster host)",
  },
  "databases.haReplicaExternal": {
    message: "Внешний",
    description: "HA panel replica connection label (public SNI host)",
  },
  "databases.haAddReplica": {
    message: "Добавить реплику для чтения",
    description:
      "HA panel button that opens the add-read-replica dialog (w5/m138)",
  },
  "databases.haAddReplicaTitle": {
    message: "Добавить реплику для чтения",
    description: "Add-read-replica dialog title",
  },
  "databases.haAddReplicaBody": {
    message:
      "Реплика для чтения — это отдельный оплачиваемый инстанс, который получает данные от основного и обрабатывает запросы только на чтение по собственному адресу.",
    description: "Add-read-replica dialog description",
  },
  "databases.haReplicaNameLabel": {
    message: "Название",
    description: "Add-read-replica dialog name field label",
  },
  "databases.haReplicaNameHint": {
    message:
      "Строчные буквы, цифры и дефисы, до {max} символов. Название становится частью имени хоста реплики.",
    description: "Add-read-replica name hint; max is the API's length limit",
  },
  "databases.haReplicaNameInvalid": {
    message:
      "Используйте от 1 до {max} строчных букв, цифр или дефисов; имя должно начинаться и заканчиваться буквой или цифрой.",
    description:
      "Add-read-replica name error; mirrors the API's POSTGRES_READ_REPLICA_NAME_INVALID",
  },
  "databases.haReplicaNameDuplicate": {
    message:
      "У этой базы данных уже есть реплика для чтения с названием {name}.",
    description: "Add-read-replica name error for a name already in use",
  },
  "databases.haAddReplicaSubmit": {
    message: "Добавить реплику",
    description: "Add-read-replica dialog submit button",
  },
  "databases.haAddReplicaSuccess": {
    message: "Реплика для чтения {name} создаётся.",
    description: "Toast after a read replica is added",
  },
  "databases.haAddReplicaError": {
    message: "Не удалось добавить реплику для чтения {name}.",
    description: "Toast when adding a read replica fails",
  },
  "databases.haRemoveReplica": {
    message: "Удалить",
    description: "HA panel per-replica remove button",
  },
  "databases.haRemoveReplicaTitle": {
    message: "Удалить реплику для чтения {name}?",
    description: "Remove-read-replica confirmation title",
  },
  "databases.haRemoveReplicaBody": {
    message:
      "Её инстанс будет удалён, а строки подключения перестанут работать. Основной инстанс не затрагивается.",
    description: "Remove-read-replica confirmation body",
  },
  "databases.haRemoveReplicaConfirm": {
    message: "Удалить реплику",
    description: "Remove-read-replica confirm button",
  },
  "databases.haRemoveReplicaSuccess": {
    message: "Реплика для чтения {name} удаляется.",
    description: "Toast after a read replica is removed",
  },
  "databases.haRemoveReplicaError": {
    message: "Не удалось удалить реплику для чтения {name}.",
    description: "Toast when removing a read replica fails",
  },
  "databases.haReplicaPlanUnsupported": {
    message: "Реплики для чтения недоступны на тарифе этой базы данных.",
    description: "Why Add read replica is disabled: the plan allows none",
  },
  "databases.haReplicaLimitReached": {
    message: "У этой базы данных максимальное число реплик для чтения: {max}.",
    description:
      "Why Add read replica is disabled: the plan's replica limit is reached",
  },
  "databases.haReplicaStorageTooSmall": {
    message:
      "Для реплик для чтения нужно минимум {min} ГБ хранилища; у этой базы данных {current} ГБ.",
    description:
      "Why Add read replica is disabled: storage is below the replica minimum",
  },
  "databases.haNotEnabled": {
    message: "Высокая доступность для этой базы данных не включена.",
    description: "HA panel line when HA is off on a plan that offers it",
  },
  "databases.haPlanUnsupported": {
    message:
      "Для высокой доступности нужен тариф минимум с 1 CPU. Тариф этой базы данных её не поддерживает.",
    description:
      "HA panel state when HA is off and the current plan is below 1 CPU (w8/m43, Render's rule)",
  },
  "databases.haToggleLabel": {
    message: "Высокая доступность",
    description: "Accessible name of the HA panel's on/off switch (w2/m171)",
  },
  "databases.haStarting": {
    message:
      "Резервный инстанс запускается на другом узле. Переключение станет доступно, когда он будет готов.",
    description:
      "HA panel line while HA is on but the standby is not ready yet",
  },
  "databases.haStopping": {
    message: "Высокая доступность отключается. Резервный инстанс удаляется.",
    description: "HA panel line while HA is off but the standby still runs",
  },
  "databases.haEnableTitle": {
    message: "Включить высокую доступность для {name}?",
    description: "HA enable confirmation title",
  },
  "databases.haEnableBody": {
    message:
      "Резервный инстанс запускается на другом узле и автоматически берёт нагрузку на себя при сбое основного. Он оплачивается так же, как основной.",
    description: "HA enable confirmation body",
  },
  "databases.haEnablePrice": {
    message: "Резервный инстанс добавит {price}.",
    description:
      "HA enable confirmation price note; {price} is a formatted monthly price",
  },
  "databases.haEnableConfirm": {
    message: "Включить высокую доступность",
    description: "HA enable confirmation button",
  },
  "databases.haDisableTitle": {
    message: "Отключить высокую доступность для {name}?",
    description: "HA disable confirmation title",
  },
  "databases.haDisableBody": {
    message:
      "Резервный инстанс удаляется. Если основной откажет, база данных будет недоступна до восстановления.",
    description: "HA disable confirmation body",
  },
  "databases.haDisableConfirm": {
    message: "Отключить высокую доступность",
    description: "HA disable confirmation button",
  },
  "databases.haEnableSuccess": {
    message: "Высокая доступность для {name} включается.",
    description: "Toast after HA was enabled",
  },
  "databases.haDisableSuccess": {
    message: "Высокая доступность для {name} отключается.",
    description: "Toast after HA was disabled",
  },
  "databases.haEnableError": {
    message: "Не удалось включить высокую доступность для {name}.",
    description: "Fallback toast when enabling HA fails",
  },
  "databases.haDisableError": {
    message: "Не удалось отключить высокую доступность для {name}.",
    description: "Fallback toast when disabling HA fails",
  },
  "databases.planPickerHAUnsupported": {
    message:
      "{name} не поддерживает высокую доступность (нужно минимум 1 CPU). Сначала отключите высокую доступность.",
    description:
      "Plan picker refusal when an HA database selects a plan below 1 CPU; mirrors the API message (w8/m43)",
  },
  "databases.planPickerStorageUnsupported": {
    message:
      "{name} допускает до {max} ГБ хранилища. У этой базы данных {current} ГБ, и уменьшить их нельзя. Выберите тариф с достаточным объёмом хранилища.",
    description: "Plan change blocked by the database's existing storage",
  },
  "databases.planPickerReplicasUnsupported": {
    message:
      "Лимит реплик для чтения на {name}: {max}. У этой базы данных их {current}.",
    description: "Plan change blocked by the database's read replicas",
  },
  "databases.planPickerAutoscalingUnsupported": {
    message:
      "{name} не поддерживает автомасштабирование диска. Сначала отключите автомасштабирование диска.",
    description: "Plan change blocked while disk autoscaling is enabled",
  },
  "databases.planPickerPoolerUnsupported": {
    message: "{name} не поддерживает пул подключений. Сначала отключите пулер.",
    description: "Plan change blocked while a connection pooler is enabled",
  },
  "databases.planTitle": {
    message: "Тип инстанса",
    description: "Database detail plan-picker card title",
  },
  "databases.planDescription": {
    message:
      "Измените тип инстанса. База данных без резервного инстанса перезапускается для применения изменения и будет недоступна минуту-две. Ваши данные сохраняются.",
    description: "Database detail plan-picker card description",
  },
  "databases.planPickerSave": {
    message: "Сохранить",
    description: "Database plan-picker save button",
  },
  "databases.planPickerCancel": {
    message: "Отмена",
    description: "Database plan-picker cancel / reset button",
  },
  "databases.planPickerConfirmTitle": {
    message: "Перейти на {name}?",
    description: "Database plan-picker confirm dialog title",
  },
  "databases.planPickerConfirmBody": {
    message:
      "База данных перезапустится для применения нового типа инстанса и будет недоступна минуту-две. Подключения прервутся. Ваши данные сохранятся.",
    description: "Database plan-picker confirm dialog body",
  },
  "databases.planPickerConfirmPrice": {
    message: "Новый тариф стоит ${price}/месяц.",
    description:
      "Plan-change confirm: the new plan's monthly price, from the API's price sheet (w4/156)",
  },
  "databases.planPickerSuccess": {
    message: "Обновление тарифа до {name}…",
    description: "Toast after a plan update is accepted",
  },
  "databases.planPickerError": {
    message: "Не удалось обновить тариф. Попробуйте ещё раз.",
    description: "Toast when a plan update fails",
  },
  "databases.storageTitle": {
    message: "Хранилище",
    description:
      "Plan section Storage row heading (Render's Plan page Storage section, w2/m307)",
  },
  "databases.storageSize": {
    message: "{size} ГБ",
    description: "Plan section Storage row: the allocated storage",
  },
  "databases.storageFixed": {
    message:
      "У бесплатных баз данных фиксированное хранилище. Перейдите на платный тариф, чтобы увеличить его.",
    description:
      "Why the Storage Edit button is disabled on a free database (bex-api POSTGRES_STORAGE_PLAN_UNSUPPORTED)",
  },
  "databases.storageEdit": {
    message: "Изменить",
    description:
      "Plan section Storage row button that opens the storage dialog",
  },
  "databases.storageEditAria": {
    message: "Изменить хранилище",
    description: "Accessible name of the Storage row's Edit button",
  },
  "databases.storageEditTitle": {
    message: "Изменить хранилище для {name}",
    description: "Storage dialog title; {name} is the database",
  },
  "databases.storageEditBody": {
    message:
      "Введите любое число, кратное {step} ГБ и превышающее текущие {current} ГБ. После увеличения хранилище нельзя уменьшить.",
    description:
      "Storage dialog description (Render: storage only grows, in 5 GB steps)",
  },
  "databases.storageEditLabel": {
    message: "Хранилище (ГБ)",
    description: "Storage dialog size field label",
  },
  "databases.storageEditStep": {
    message: "Введите целое число, кратное {step} ГБ.",
    description: "Storage dialog validation: not a whole multiple of 5 GB",
  },
  "databases.storageEditNotGreater": {
    message:
      "Введите значение больше текущих {current} ГБ. Хранилище нельзя уменьшить.",
    description:
      "Storage dialog validation: the size is not above the current storage",
  },
  "databases.storageEditTooLarge": {
    message: "{name} допускает до {max} ГБ хранилища.",
    description:
      "Storage dialog validation: the size is above the plan's maximum; {name} is the plan",
  },
  "databases.storageEditSave": {
    message: "Сохранить",
    description: "Storage dialog submit button",
  },
  "databases.storageEditSuccess": {
    message: "Увеличение хранилища до {size} ГБ…",
    description: "Toast after a storage increase is accepted",
  },
  "databases.storageEditError": {
    message: "Не удалось обновить хранилище. Попробуйте ещё раз.",
    description:
      "Storage dialog fallback error when the server gives no reason",
  },
  "databases.nameEdit": {
    message: "Изменить название базы данных",
    description: "Accessible label for the database-name edit-in-place pencil",
  },
  "databases.nameDescription": {
    message:
      "Измените отображаемое название. ID базы данных и все данные подключения остаются прежними.",
    description: "Database detail rename row description",
  },
  "databases.nameSuccess": {
    message: "База данных переименована в {name}.",
    description: "Toast after a database rename succeeds",
  },
  "databases.nameError": {
    message: "Не удалось переименовать базу данных. Попробуйте ещё раз.",
    description: "Toast when a database rename fails unexpectedly",
  },
  "databases.nameConflict": {
    message:
      "База данных с таким названием уже существует в этом рабочем пространстве.",
    description: "Toast when a database rename collides with another name",
  },
  "databases.nameInvalid": {
    message:
      "Введите название длиной до 100 символов без переводов строк и управляющих символов.",
    description: "Database rename validation message",
  },
  "databases.insightsTitle": {
    message: "Аналитика",
    description: "Database insights panel card title",
  },
  "databases.insightsDescription": {
    message:
      "Данные о процессах, производительности запросов, хранилище и конфигурации в реальном времени.",
    description: "Database insights panel card description",
  },
  "databases.insightsRefresh": {
    message: "Обновить аналитику",
    description: "Accessible label for the insights refresh button",
  },
  "databases.insightsUnavailable": {
    message: "Недоступно: возможно, база данных ещё создаётся.",
    description: "Insights sub-section error state",
  },
  "databases.insightsSizeTitle": {
    message: "Хранилище",
    description: "Insights panel — database/table sizes sub-section heading",
  },
  "databases.insightsNoSizes": {
    message: "Данных о размерах пока нет.",
    description: "Insights storage empty state",
  },
  "databases.insightsProcessesTitle": {
    message: "Активные процессы",
    description: "Insights panel — pg_stat_activity sub-section heading",
  },
  "databases.insightsNoProcesses": {
    message: "Активных процессов нет.",
    description: "Insights processes empty state",
  },
  "databases.insightsTopQueriesTitle": {
    message: "Самые затратные запросы",
    description: "Insights panel — pg_stat_statements sub-section heading",
  },
  "databases.insightsNoTopQueries": {
    message:
      "Запросов пока нет. Здесь появятся инструкции, которые выполняют ваше приложение и консоль SQL.",
    description:
      "Insights top-queries empty state: the read succeeded and nothing the owner ran is in the list. A failed read, including a missing pg_stat_statements, shows insightsUnavailable instead, so this text must not blame the extension",
  },
  "databases.insightsQueryHidden": {
    message: "Скрыто",
    description:
      "Insights — query cell for a row whose SQL PostgreSQL hides from the reading role",
  },
  "databases.insightsMaskedNote": {
    message:
      "Некоторые строки принадлежат другим ролям базы данных, поэтому PostgreSQL скрывает их SQL в этом представлении. Всё остальное в строке настоящее.",
    description:
      "Insights — explanation shown when at least one process or top-query row has masked SQL",
  },
  "databases.insightsTableScansTitle": {
    message: "Сканирования таблиц",
    description: "Insights panel — pg_stat_user_tables sub-section heading",
  },
  "databases.insightsNoTableScans": {
    message: "Статистики сканирования пользовательских таблиц пока нет.",
    description: "Insights table-scans empty state",
  },
  "databases.insightsParamsDeclaredValue": {
    message: "Заданное значение",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsObservedValue": {
    message: "Наблюдаемое значение",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsObservedHint": {
    message:
      "Текущее значение; оно не подтверждает, что запрошенное значение применено.",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsNotObserved": {
    message:
      "Запрошенное переопределение не наблюдается. Возможно, оно ещё не применено или недопустимо.",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsObservationUnavailable": {
    message: "Сейчас не удаётся проверить это переопределение.",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsTitle": {
    message: "Переопределения параметров",
    description:
      "Insights panel — heading for the parameters this database declares (read-only)",
  },
  "databases.insightsParamsDescription": {
    message:
      "Настройки, заданные для этой базы данных до отмены переопределений параметров. Они по-прежнему действуют, но изменить их здесь больше нельзя; задавайте значения по умолчанию для сеанса через ALTER DATABASE … SET в консоли SQL.",
    description:
      "Explains that the declared parameters are read-only legacy settings that still apply, and how to set session defaults instead",
  },
  "databases.insightsEffectiveParamsTitle": {
    message: "Параметры не по умолчанию",
    description: "Insights panel — pg_settings sub-section heading",
  },
  "databases.insightsEffectiveParamsDescription": {
    message:
      "Все действующие сейчас настройки, отличающиеся от значений по умолчанию, включая управляемые платформой. Только для чтения.",
    description:
      "Explains that the pg_settings table is observed configuration, mostly platform-owned, and cannot be edited",
  },
  "databases.insightsNoParams": {
    message: "Все параметры имеют значения по умолчанию.",
    description: "Insights parameter-overrides empty state",
  },
  "databases.insightsColTable": {
    message: "Таблица",
    description: "Insights table column header (table name)",
  },
  "databases.insightsColRelation": {
    message: "Таблица или индекс",
    description:
      "Insights sizes column header (a table's or an index's name; tables and indexes are listed together)",
  },
  "databases.insightsIndexOn": {
    message: "индекс таблицы {table}",
    description:
      "Insights sizes: suffix naming the table an index row belongs to; {table} is the quoted schema-qualified table name",
  },
  "databases.insightsColSize": {
    message: "Размер",
    description: "Insights table column header (relation size)",
  },
  "databases.insightsColUser": {
    message: "Пользователь",
    description: "Insights processes column header (pg role)",
  },
  "databases.insightsColState": {
    message: "Состояние",
    description: "Insights processes column header (active/idle/…)",
  },
  "databases.insightsColDuration": {
    message: "Длительность",
    description: "Insights processes column header (seconds since query start)",
  },
  "databases.insightsColQuery": {
    message: "Запрос",
    description: "Insights column header (SQL text, truncated)",
  },
  "databases.insightsColCalls": {
    message: "Вызовы",
    description: "Insights top-queries column header (invocation count)",
  },
  "databases.insightsColTotalTime": {
    message: "Общее время",
    description: "Insights top-queries column header (total exec time)",
  },
  "databases.insightsColMeanTime": {
    message: "Среднее время",
    description: "Insights top-queries column header (mean exec time per call)",
  },
  "databases.insightsColSeqScans": {
    message: "Послед. сканирования",
    description: "Insights table-scans column header (sequential scan count)",
  },
  "databases.insightsColIdxScans": {
    message: "Индексные сканирования",
    description: "Insights table-scans column header (index scan count)",
  },
  "databases.insightsColLiveRows": {
    message: "Активные строки",
    description:
      "Insights table-scans column header (estimated live row count)",
  },
  "databases.insightsColParam": {
    message: "Параметр",
    description:
      "Insights parameter-overrides column header (pg_settings.name)",
  },
  "databases.insightsColSetting": {
    message: "Значение",
    description:
      "Insights parameter-overrides column header (pg_settings.setting)",
  },
  "databases.insightsColSource": {
    message: "Источник",
    description:
      "Insights parameter-overrides column header (pg_settings.source)",
  },
  "databases.sqlTitle": {
    message: "Консоль SQL",
    description: "Database detail SQL console card title",
  },
  "databases.sqlDescription": {
    message:
      "Выполните одну инструкцию SQL в этой базе данных. Результаты ограничены 500 строками, а запросы прерываются по тайм-ауту через 10 секунд.",
    description: "Database SQL console safety summary",
  },
  "databases.sqlEditorLabel": {
    message: "SQL-запрос",
    description: "Accessible label for the SQL editor",
  },
  "databases.sqlShortcut": {
    message: "Нажмите Ctrl+Enter или ⌘+Enter для запуска",
    description: "SQL console keyboard shortcut hint",
  },
  "databases.sqlCreating": {
    message:
      "Эта база данных ещё создаётся. SQL заработает, когда она станет доступной; ваш запрос и история сохраняются.",
    description: "SQL console notice while the database is creating (w4/177)",
  },
  "databases.sqlUpgrading": {
    message:
      "Выполняется офлайн-обновление PostgreSQL. SQL заработает снова, когда база данных станет доступной; ваш запрос и история сохраняются.",
    description: "SQL console notice while the database is upgrading (w4/177)",
  },
  "databases.sqlRestarting": {
    message:
      "Эта база данных перезапускается. Дождитесь, пока она станет доступной, чтобы выполнять SQL; ваш запрос и история сохраняются.",
    description: "SQL console notice while the database is restarting (w4/177)",
  },
  "databases.sqlSuspended": {
    message:
      "Эта база данных приостановлена. Возобновите её, чтобы выполнять SQL; ваш запрос и история сохраняются.",
    description: "SQL console notice while the database is suspended",
  },
  "databases.sqlRun": {
    message: "Выполнить запрос",
    description: "SQL console run button",
  },
  "databases.sqlRunning": {
    message: "Выполняется…",
    description: "SQL console run button while a query is executing",
  },
  "databases.sqlUnknownError": {
    message: "Не удалось выполнить запрос. Попробуйте ещё раз.",
    description: "SQL console fallback error",
  },
  "databases.sqlReturnedRows_one": {
    message: "Возвращена {count} строка",
    description: "SQL console SELECT result count (singular)",
  },
  "databases.sqlReturnedRows_few": {
    message: "Возвращено {count} строки",
    description: "SQL console SELECT result count (plural)",
  },
  "databases.sqlReturnedRows_many": {
    message: "Возвращено {count} строк",
    description: "SQL console SELECT result count (plural)",
  },
  "databases.sqlReturnedRows_other": {
    message: "Возвращено {count} строки",
    description: "SQL console SELECT result count (plural)",
  },
  "databases.sqlAffectedRows_one": {
    message: "Затронута {count} строка",
    description: "SQL console write result count (singular)",
  },
  "databases.sqlAffectedRows_few": {
    message: "Затронуто {count} строки",
    description: "SQL console write result count (plural)",
  },
  "databases.sqlAffectedRows_many": {
    message: "Затронуто {count} строк",
    description: "SQL console write result count (plural)",
  },
  "databases.sqlAffectedRows_other": {
    message: "Затронуто {count} строки",
    description: "SQL console write result count (plural)",
  },
  "databases.sqlTruncated": {
    message: "Результат ограничен {limit} строками",
    description: "SQL console backend row-cap notice",
  },
  "databases.sqlNull": {
    message: "NULL",
    description: "SQL console null cell value",
  },
  "databases.sqlNullLabel": {
    message: "SQL NULL (нет значения)",
    description:
      "SQL console accessible meaning of a null cell, distinct from the literal text NULL",
  },
  "databases.sqlNoRows": {
    message: "Запрос не вернул строк.",
    description: "SQL console empty result message",
  },
  "databases.sqlPrevious": {
    message: "Назад",
    description: "SQL result pagination previous button",
  },
  "databases.sqlNext": {
    message: "Далее",
    description: "SQL result pagination next button",
  },
  "databases.sqlPage": {
    message: "Страница {page} из {pages}",
    description: "SQL result pagination position",
  },
  "databases.sqlHistory": {
    message: "История запросов",
    description: "SQL console session history heading",
  },
  "databases.sqlClearHistory": {
    message: "Очистить",
    description: "SQL console clear-history button",
  },
  "databases.sqlNoHistory": {
    message: "Здесь появятся запросы, выполненные в этом сеансе браузера.",
    description: "SQL console empty history message",
  },
  "databases.sqlConfirmTitle": {
    message: "Выполнить инструкцию, которая может изменить данные?",
    description: "SQL console write confirmation title",
  },
  "databases.sqlConfirmDescription": {
    message:
      "Эта инструкция не распознана как инструкция только для чтения. Она может изменить или удалить данные базы данных, и bex не сможет это отменить.",
    description: "SQL console write confirmation warning",
  },
  "databases.sqlConfirmCancel": {
    message: "Отмена",
    description: "SQL console write confirmation cancel button",
  },
  "databases.sqlConfirmRun": {
    message: "Выполнить инструкцию",
    description: "SQL console confirmed write action",
  },
  "databases.detailNavLabel": {
    message: "Сведения о базе данных",
    description: "Accessible label for the managed Postgres detail tabs",
  },
  "databases.sectionNavigation": {
    message: "Разделы базы данных",
    description:
      "Accessible label for the database detail page section navigation",
  },
  "databases.dangerZoneTitle": {
    message: "Опасная зона",
    description:
      "Database detail section-nav label for the destructive actions",
  },
  "databases.overviewTab": {
    message: "Обзор",
    description: "Managed Postgres overview tab",
  },
  "databases.logsTab": {
    message: "Логи",
    description: "Managed Postgres logs tab",
  },
  "databases.logsRangeLabel": {
    message: "Период времени",
    description: "Managed Postgres log range filter label",
  },
  "databases.logsInstanceLabel": {
    message: "Инстанс базы данных",
    description: "Managed Postgres log instance filter label",
  },
  "databases.logsAllInstances": {
    message: "Все инстансы",
    description: "Managed Postgres log instance filter all option",
  },
  "databases.logsSearchPlaceholder": {
    message: "Поиск по логам базы данных",
    description: "Managed Postgres log search placeholder",
  },
  "databases.logsLoading": {
    message: "Загрузка логов базы данных…",
    description: "Managed Postgres logs loading state",
  },
  "databases.logsEmptyTitle": {
    message: "Логов базы данных пока нет",
    description: "Managed Postgres logs empty-state title",
  },
  "databases.logsEmptyBody": {
    message: "Здесь появится вывод Postgres за выбранный период.",
    description: "Managed Postgres logs empty-state body",
  },
  "databases.logsUnavailableTitle": {
    message: "Логи базы данных не настроены",
    description: "Managed Postgres logs unavailable-state title",
  },
  "databases.logsUnavailableBody": {
    message:
      "В этой установке не настроены ни постоянная история логов, ни источник живых логов подов CNPG.",
    description: "Managed Postgres logs unavailable-state body",
  },
  "databases.logsUnauthorizedTitle": {
    message: "У вас нет доступа к этим логам",
    description: "Managed Postgres logs unauthorized-state title",
  },
  "databases.logsUnauthorizedBody": {
    message:
      "Для просмотра логов базы данных нужен доступ участника-разработчика в этом рабочем пространстве.",
    description: "Managed Postgres logs unauthorized-state body",
  },
  "databases.logsErrorTitle": {
    message: "Не удалось загрузить логи базы данных",
    description: "Managed Postgres logs generic error title",
  },
};

export default ruDatabases;
