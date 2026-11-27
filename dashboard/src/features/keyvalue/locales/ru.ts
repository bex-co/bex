import type { TranslationEntry } from "@/i18n";

const ruKeyValue: Record<string, TranslationEntry> = {
  "keyvalue.resourceType": {
    message: "Key Value",
    description: "Key Value resource type used in document titles",
  },
  "keyvalue.statTotal": {
    message: "Всего хранилищ Key Value",
    description: "Key Value page stat card label",
  },
  "keyvalue.statAvailable": {
    message: "Доступны",
    description: "Key Value page stat card label (healthy stores)",
  },
  "keyvalue.statCreating": {
    message: "Создаются",
    description: "Key Value page stat card label (provisioning stores)",
  },
  "keyvalue.cardTitle": {
    message: "Key Value",
    description: "Key Value table card title",
  },
  "keyvalue.colName": {
    message: "Название",
    description: "Key Value table column header",
  },
  "keyvalue.colStatus": {
    message: "Статус",
    description: "Key Value table column header",
  },
  "keyvalue.colPlan": {
    message: "Тариф вычислений",
    description: "Key Value table column header (plan / tier)",
  },
  "keyvalue.colVersion": {
    message: "Версия",
    description: "Key Value table column header (Valkey version)",
  },
  "keyvalue.colCreated": {
    message: "Создано",
    description: "Key Value table column header (relative age from createdAt)",
  },
  "keyvalue.colActions": {
    message: "Действия",
    description: "Key Value table actions column header (screen-reader only)",
  },
  "keyvalue.statusAvailable": {
    message: "Доступно",
    description: "Key Value status badge (Valkey instance healthy)",
  },
  "keyvalue.statusCreating": {
    message: "Создаётся",
    description: "Key Value status badge (provisioning)",
  },
  "keyvalue.statusRestarting": {
    message: "Перезапускается",
    description:
      "Key Value status badge (Render's config_restart: an established store restarting after a config change, restart, or resume — not a first provision)",
  },
  "keyvalue.statusUnavailable": {
    message: "Недоступно",
    description: "Key Value status badge (provisioning failed)",
  },
  "keyvalue.statusSuspended": {
    message: "Приостановлено",
    description:
      "Key Value status badge (hibernated; suspension wins over the status enum)",
  },
  "keyvalue.statusUnknown": {
    message: "Неизвестно",
    description: "Key Value status badge for an unrecognized status",
  },
  "keyvalue.statusReasonStatefulSetReadFailed": {
    message: "Не удалось проверить состояние рабочей нагрузки Key Value.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StatefulSetReadFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStatefulSetFailed": {
    message:
      "Не удалось обновить рабочую нагрузку Key Value, поэтому последнее изменение не применено.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StatefulSetFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonPVCReadFailed": {
    message: "Не удалось проверить состояние тома хранилища.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's PVCReadFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonPVCResizeFailed": {
    message: "Не удалось изменить размер тома хранилища Key Value.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's PVCResizeFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonSecretFailed": {
    message: "Не удалось сохранить данные подключения Key Value.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's SecretFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonCredentialSecretFailed": {
    message: "Не удалось сохранить учётные данные Key Value.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's CredentialSecretFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonSecretRecreateFailed": {
    message: "Не удалось пересоздать данные подключения Key Value.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's SecretRecreateFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonServiceFailed": {
    message: "Не удалось настроить внутренний адрес Key Value.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's ServiceFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonNetworkPolicyFailed": {
    message: "Не удалось применить сетевую политику Key Value.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's NetworkPolicyFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonBackupNetworkPolicyFailed": {
    message:
      "Не удалось применить сетевую политику задания резервного копирования.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's BackupNetworkPolicyFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonBackupCronJobFailed": {
    message:
      "Не удалось обновить расписание ежедневного резервного копирования.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's BackupCronJobFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonTLSIssuerMissing": {
    message:
      "Публичный доступ нельзя включить: не настроен издатель сертификатов.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's TLSIssuerMissing Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonCertificateFailed": {
    message: "Не удалось выпустить сертификат для публичного доступа.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's CertificateFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonCertificateCleanupFailed": {
    message: "Не удалось удалить сертификат для публичного доступа.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's CertificateCleanupFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageClassMissing": {
    message:
      "У тома хранилища нет класса хранилища, поэтому он не может расти.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageClassMissing Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageClassNotFound": {
    message: "Класс хранилища для тома не найден, поэтому он не может расти.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageClassNotFound Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageClassNotExpandable": {
    message: "Класс хранилища тома не позволяет ему расти.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageClassNotExpandable Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageBlockedByQuota": {
    message:
      "Том хранилища не может расти: в квоте хранилища рабочего пространства нет места.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageBlockedByQuota Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonPersistenceTransitionFailed": {
    message:
      "Не удалось подготовить сохранённые данные к запуску; данные сохранены, попытка будет повторена.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's PersistenceTransitionFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonPersistenceSourceUnknown": {
    message:
      "Режим сохранения хранимых данных неизвестен, поэтому Key Value не запустится, пока оператор платформы не подтвердит его. Данные не изменялись.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's PersistenceSourceUnknown Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageShrinkRejected": {
    message:
      "Хранилище Key Value можно только увеличивать: том не может быть меньше текущего размера.",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageShrinkRejected Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.errorTitle": {
    message: "Не удалось загрузить хранилища Key Value",
    description: "Key Value list error card title",
  },
  "keyvalue.errorBody": {
    message:
      "Запрос к bex-api не выполнен. Проверьте подключение и повторите попытку.",
    description: "Key Value list error card body",
  },
  "keyvalue.emptyTitle": {
    message: "Хранилищ Key Value пока нет",
    description: "Key Value list empty state title",
  },
  "keyvalue.emptyBody": {
    message:
      "Создайте первое управляемое хранилище Key Value, и оно появится здесь.",
    description: "Key Value list empty state body",
  },
  "keyvalue.actionsMenu": {
    message: "Действия для {name}",
    description: "Accessible label for the per-row actions trigger",
  },
  "keyvalue.actionDelete": {
    message: "Удалить",
    description: "Row action: permanently delete the Key Value store",
  },
  "keyvalue.deleteConfirmTitle": {
    message: "Удалить инстанс Key Value",
    description: "Delete-confirmation dialog title",
  },
  "keyvalue.deleteConfirmBody": {
    message:
      "Это действие нельзя отменить. Вы уверены, что хотите удалить этот инстанс Key Value?",
    description:
      "Delete-confirmation dialog body (verbatim Render copy, docs/render-artifacts/key-value.md)",
  },
  "keyvalue.deleteConfirmPrompt": {
    message: "Введите {phrase} ниже для подтверждения.",
    description:
      "Body prompt naming Render's exact sudo delete phrase (rendered bold by SudoCommandField)",
  },
  "keyvalue.deleteCancel": {
    message: "Отмена",
    description: "Delete-confirmation dialog cancel button",
  },
  "keyvalue.deleteConfirm": {
    message: "Удалить инстанс Key Value",
    description: "Delete-confirmation dialog confirm button",
  },
  "keyvalue.deleteSuccess": {
    message: "Удаление {name}…",
    description: "Toast after a delete request is accepted",
  },
  "keyvalue.deleteError": {
    message: "Не удалось удалить {name}. Попробуйте ещё раз.",
    description: "Toast when a delete request fails",
  },
  "keyvalue.createButton": {
    message: "Новый Key Value",
    description: "Button that navigates to the create-Key-Value page",
  },
  "keyvalue.createTitle": {
    message: "Новый Key Value",
    description: "Create-Key-Value page title",
  },
  "keyvalue.createDescription": {
    message: "Создайте управляемый инстанс Valkey (совместимый с Redis).",
    description: "Create-Key-Value page description",
  },
  "keyvalue.fieldName": {
    message: "Название",
    description: "Create-Key-Value form field label (store name)",
  },
  "keyvalue.fieldNamePlaceholder": {
    message: "example-key-value-name",
    description: "Create-Key-Value name input placeholder",
  },
  "keyvalue.fieldNameError": {
    message:
      "Введите название длиной до 100 символов без переводов строк и управляющих символов.",
    description: "Create-Key-Value name validation message",
  },
  "keyvalue.fieldPlan": {
    message: "Тариф вычислений",
    description: "Create-Key-Value form field label (plan / tier)",
  },
  "keyvalue.fieldVersion": {
    message: "Версия Valkey",
    description: "Create-Key-Value form field label (major version)",
  },
  "keyvalue.fieldVersionDefault": {
    message: "По умолчанию (последняя)",
    description: "Create-Key-Value version select default option",
  },
  "keyvalue.fieldMaxmemoryPolicy": {
    message: "Политика maxmemory",
    description: "Create-Key-Value form field label (eviction policy)",
  },
  "keyvalue.fieldMaxmemoryRecommended": {
    message: "(рекомендуется для кэшей)",
    description:
      "Suffix on the recommended maxmemory policy option (allkeys-lru)",
  },
  "keyvalue.fieldMaxmemoryPolicyHint": {
    message: "Как вытесняются ключи, когда хранилище достигает лимита памяти.",
    description: "Create-Key-Value maxmemory policy helper text",
  },
  "keyvalue.fieldPersistenceMode": {
    message: "Режим сохранения данных",
    description: "Create-Key-Value form field label (persistence)",
  },
  "keyvalue.fieldPersistenceModeHint": {
    message: "Как данные сохраняются на диск, чтобы пережить перезапуск.",
    description: "Create-Key-Value persistence mode helper text",
  },
  "keyvalue.persistenceJournalSnapshot": {
    message: "Журнал + снимок",
    description: "Persistence mode option: AOF journal plus RDB snapshots",
  },
  "keyvalue.persistenceSnapshot": {
    message: "Только снимок",
    description: "Persistence mode option: RDB snapshots only",
  },
  "keyvalue.persistenceOff": {
    message: "Выключено",
    description: "Persistence mode option: no persistence",
  },
  "keyvalue.fieldPublic": {
    message: "Публичный доступ",
    description: "Create-Key-Value form field label (external endpoint toggle)",
  },
  "keyvalue.fieldPublicHint": {
    message: "Разрешить подключения извне вашей частной сети по TLS.",
    description: "Create-Key-Value public toggle helper text",
  },
  "keyvalue.createCancel": {
    message: "Отмена",
    description: "Create-Key-Value page cancel button",
  },
  "keyvalue.createSubmit": {
    message: "Создать инстанс Key Value",
    description: "Create-Key-Value page submit button",
  },
  "keyvalue.createSuccess": {
    message: "Создание {name}…",
    description:
      "Toast after a create request is accepted (provisioning is async)",
  },
  "keyvalue.createError": {
    message: "Не удалось создать {name}. Попробуйте ещё раз.",
    description: "Toast when a create request fails",
  },
  "keyvalue.capLimitTitle": {
    message: "Достигнут лимит Key Value",
    description:
      "Alert title when the workspace's KV creation cap is hit (w7/m9)",
  },
  "keyvalue.capLimitUpgrade": {
    message: "Перейти на другой тариф",
    description: "Upgrade CTA button inside the KV cap-limit Alert (w7/m9)",
  },
  "keyvalue.metaTitle": {
    message: "Сведения",
    description: "Key Value detail metadata card title",
  },
  "keyvalue.metaId": {
    message: "ID",
    description: "Key Value detail metadata row label (immutable red- id)",
  },
  "keyvalue.metaStatus": {
    message: "Статус",
    description: "Key Value detail metadata row label",
  },
  "keyvalue.metaPlan": {
    message: "Тариф вычислений",
    description: "Key Value detail metadata row label",
  },
  "keyvalue.metaConnectionLimit": {
    message: "Лимит подключений",
    description:
      "Key Value detail Details card row label: the plan's client connection limit, as Render's Key Value Instance card shows it",
  },
  "keyvalue.metaVersion": {
    message: "Версия",
    description: "Key Value detail metadata row label",
  },
  "keyvalue.metaPublic": {
    message: "Публичный доступ",
    description: "Key Value detail metadata row label (external endpoint)",
  },
  "keyvalue.metaExternalHost": {
    message: "Внешний хост",
    description: "Key Value detail metadata row label (SNI hostname)",
  },
  "keyvalue.metaRegion": {
    message: "Регион",
    description: "Key Value detail metadata row label (platform placement)",
  },
  "keyvalue.metaCreated": {
    message: "Создано",
    description: "Key Value detail metadata row label (relative age)",
  },
  "keyvalue.yes": {
    message: "Да",
    description: "Metadata value for a true boolean field",
  },
  "keyvalue.publicPending": {
    message: "Да (внешняя конечная точка открывается: сертификат ожидается)",
    description:
      "Details value for Public access while the store's certificate is pending, so it does not read as reachable (w4/m357)",
  },
  "keyvalue.publicFailed": {
    message: "Да (пока недоступно: не удалось выпустить сертификат)",
    description:
      "Details value for Public access when the store's certificate failed to issue (w4/m357)",
  },
  "keyvalue.no": {
    message: "Нет",
    description: "Metadata value for a false boolean field",
  },
  "keyvalue.connTitle": {
    message: "Подключения",
    description: "Connection-info panel card title",
  },
  "keyvalue.connDescription": {
    message:
      "Строки подключения к этому хранилищу Key Value. Показываются только по запросу, автоматически никогда.",
    description: "Connection-info panel card description",
  },
  "keyvalue.internalAuthOff": {
    message: "Внутренним подключениям пароль не нужен.",
    description: "Connections panel: internal auth off",
  },
  "keyvalue.internalAuthOn": {
    message: "Внутренним подключениям нужен пароль.",
    description: "Connections panel: internal auth on",
  },
  "keyvalue.internalAuthEnable": {
    message: "Включить внутреннюю аутентификацию",
    description: "Button and confirm label (Render's wording)",
  },
  "keyvalue.internalAuthConfirmTitle": {
    message: "Включить внутреннюю аутентификацию?",
    description: "Confirm dialog title",
  },
  "keyvalue.internalAuthConfirmBody": {
    message:
      "Всем внутренним подключениям потребуется пароль. Существующие подключения, использующие URL без аутентификации, перестанут работать, а связанные сервисы получат новый URL при следующем деплое; обновите их, указав URL из данных подключения. Это действие нельзя отменить.",
    description: "Confirm dialog body",
  },
  "keyvalue.internalAuthEnabled": {
    message: "Внутренняя аутентификация для {name} включена.",
    description: "Toast after enabling",
  },
  "keyvalue.internalAuthError": {
    message:
      "Не удалось включить внутреннюю аутентификацию. Попробуйте ещё раз.",
    description: "Toast when enabling fails",
  },
  "keyvalue.connReveal": {
    message: "Показать данные подключения",
    description: "Button that fetches the connection info on demand",
  },
  "keyvalue.connHide": {
    message: "Скрыть данные подключения",
    description: "Button that clears the revealed connection info",
  },
  "keyvalue.connInternal": {
    message: "Внутренний URL Key Value",
    description: "Connection-info field label (in-cluster redis:// URL)",
  },
  "keyvalue.connExternal": {
    message: "Внешний URL Key Value",
    description: "Connection-info field label (public rediss:// URL)",
  },
  "keyvalue.connExternalPending": {
    message:
      "Внешний доступ включается: выпускается публично доверенный сертификат (обычно несколько минут). Внешний URL и команда CLI подключатся, когда хранилище начнёт принимать TLS-подключения; до этого оно доступно только вашим сервисам по внутреннему URL. Скройте и покажите снова, чтобы обновить.",
    description:
      "Shown under the external URL while a public store's TLS listener is still opening (w9/m521)",
  },
  "keyvalue.connExternalFailed": {
    message:
      "Внешний URL и команда CLI пока не подключаются: не удалось выпустить публично доверенный сертификат этого хранилища, и bex повторяет попытку автоматически. Пока это не удастся, хранилище доступно только вашим сервисам по внутреннему URL.",
    description:
      "Shown under the external URL when a public store's certificate failed to issue (w4/m357, w9/m521)",
  },
  "keyvalue.connExternalUnavailable": {
    message:
      "Не публично. Добавьте правило входящих IP в разделе «Сеть», чтобы включить внешний доступ.",
    description:
      "Shown instead of the external URL and the CLI command when the store isn't public (w9/134)",
  },
  "keyvalue.connCli": {
    message: "Команда Valkey CLI",
    description:
      "Connection-info field label (ready-to-run valkey-cli command)",
  },
  "keyvalue.connErrorTitle": {
    message: "Не удалось загрузить данные подключения",
    description: "Connection-info panel error title",
  },
  "keyvalue.connErrorBody": {
    message:
      "Возможно, хранилище ещё создаётся или у вас нет прав на просмотр его учётных данных.",
    description: "Connection-info panel error body",
  },
  "keyvalue.copied": {
    message: "Скопировано в буфер обмена",
    description: "Toast after copying a connection field",
  },
  "keyvalue.copyError": {
    message: "Не удалось скопировать в буфер обмена",
    description: "Toast when clipboard copy fails",
  },
  "keyvalue.dangerDelete": {
    message: "Удалить инстанс Key Value",
    description: "Discoverable delete action at the bottom of Key Value Info",
  },
  "keyvalue.actionSuspend": {
    message: "Приостановить инстанс Key Value",
    description: "Suspend action button label",
  },
  "keyvalue.actionResume": {
    message: "Возобновить инстанс Key Value",
    description: "Resume action button label",
  },
  "keyvalue.confirmSuspendTitle": {
    message: "Приостановить инстанс Key Value",
    description: "Suspend-confirmation dialog title",
  },
  "keyvalue.confirmSuspendBody": {
    message: "Вы уверены, что хотите приостановить этот инстанс Key Value?",
    description: "Suspend-confirmation dialog description",
  },
  "keyvalue.confirmSuspendDetail": {
    message:
      "Подтвердите, что хотите приостановить {name}. Активные подключения будут разорваны, а возобновить инстанс можно в любой момент. Сохранность данных зависит от режима сохранения: «Выключено» теряет все данные; «Только снимок» сохраняет данные до последнего снимка; «Журнал + снимок» сохраняет записи примерно до последней секунды.",
    description: "Suspend-confirmation consequences and named instance",
  },
  "keyvalue.confirmSuspendPrompt": {
    message: "Введите {phrase} ниже для подтверждения.",
    description:
      "Body prompt naming Render's exact Key Value suspend phrase (rendered bold by SudoCommandField)",
  },
  "keyvalue.confirmCancel": {
    message: "Отмена",
    description: "Suspend-confirmation dialog cancel button",
  },
  "keyvalue.toastSuspendSuccess": {
    message: "Приостановка {name}…",
    description: "Toast after a suspend request is accepted",
  },
  "keyvalue.toastSuspendError": {
    message: "Не удалось приостановить {name}. Попробуйте ещё раз.",
    description: "Toast when a suspend request fails",
  },
  "keyvalue.toastResumeSuccess": {
    message: "Возобновление {name}…",
    description: "Toast after a resume request is accepted",
  },
  "keyvalue.toastResumeError": {
    message: "Не удалось возобновить {name}. Попробуйте ещё раз.",
    description: "Toast when a resume request fails",
  },
  "keyvalue.planTitle": {
    message: "Тариф вычислений",
    description: "Key Value detail plan-picker card title",
  },
  "keyvalue.planDescription": {
    message:
      "Перейдите на больший тариф вычислений; на меньший инстанс Key Value перейти нельзя. При изменении хранилище перезапускается и будет недоступно минуту-две. Сохранность данных зависит от режима сохранения; хранилище с режимом «Выключено» теряет все ключи.",
    description: "Key Value detail plan-picker card description",
  },
  "keyvalue.planDowngradeUnavailable": {
    message:
      "Инстанс Key Value нельзя перевести на меньший тариф вычислений. Создайте новый инстанс этого тарифа и перенесите в него данные.",
    description:
      "Tooltip on a disabled plan card smaller than the store's current plan: Render does not downgrade a Key Value instance (w8/249)",
  },
  "keyvalue.planPausedReason": {
    message:
      "Тариф Key Value «{name}» временно недоступен; выберите {largest} или меньший тариф.",
    description:
      "A paused Key Value plan's tooltip and screen-reader reason, composed in the dashboard's language; matches bex-api's English unavailableReason (w4/552)",
  },
  "keyvalue.planPickerSave": {
    message: "Сохранить",
    description: "Key Value plan-picker save button",
  },
  "keyvalue.planPickerCancel": {
    message: "Отмена",
    description: "Key Value plan-picker cancel / reset button",
  },
  "keyvalue.reviewComputeLead": {
    message: "Проверьте выбранные параметры вычислений и хранилища.",
    description:
      "Key Value plan review: the lead of a compute plan change's review (compute and storage, as the review lists both)",
  },
  "keyvalue.reviewUnavailable": {
    message: "Во время обновления инстанс Key Value будет недоступен.",
    description:
      "Key Value plan review warning: the store restarts to apply the plan (Render's copy); the data-loss sentence follows it when persistence is Off",
  },
  "keyvalue.reviewUpgradeCompute": {
    message: "Повысить тариф вычислений",
    description:
      "Key Value plan review: the button that applies a plan change (Render's copy; Key Value plans only go up)",
  },
  "keyvalue.storageTitle": {
    message: "Хранилище",
    description:
      "Key Value plan review: the Storage line's label (the store's billed storage)",
  },
  "keyvalue.storageSize": {
    message: "{size} ГБ",
    description:
      "Key Value plan review: the Storage line's figure, the billed size in GB",
  },
  "keyvalue.restartConfirmDataLoss": {
    message:
      "Сохранение данных выключено, поэтому при перезапуске будут удалены все ключи в {name}. Это действие нельзя отменить.",
    description:
      "Restart confirm (plan change): a Persistence Off store loses all data on the restart (w4/m263)",
  },
  "keyvalue.restartConfirmDataLossUnknown": {
    message:
      "Режим сохранения данных этого хранилища неизвестен. Если сохранение выключено, при перезапуске будут удалены все ключи в {name}. Это действие нельзя отменить.",
    description:
      "Restart confirm when the persistence mode could not be read: the typed phrase is still required",
  },
  "keyvalue.planPickerSuccess": {
    message: "Обновление тарифа до {name}…",
    description: "Toast after a plan update is accepted",
  },
  "keyvalue.planPickerError": {
    message: "Не удалось обновить тариф. Попробуйте ещё раз.",
    description: "Toast when a plan update fails",
  },
  "keyvalue.maxmemoryTitle": {
    message: "Политика maxmemory",
    description: "Key Value detail eviction-policy card title",
  },
  "keyvalue.maxmemoryDescription": {
    message:
      "Измените способ вытеснения ключей, когда хранилище достигает лимита памяти. Новая политика применяется к работающему хранилищу без перезапуска.",
    description: "Key Value detail eviction-policy card description",
  },
  "keyvalue.maxmemoryLabel": {
    message: "Политика maxmemory",
    description: "Accessible label for the eviction-policy select",
  },
  "keyvalue.maxmemoryEdit": {
    message: "Изменить политику maxmemory",
    description:
      "Accessible label for the maxmemory-policy edit-in-place pencil",
  },
  "keyvalue.maxmemorySuccess": {
    message: "Обновление политики maxmemory до {policy}…",
    description: "Toast after an eviction-policy update is accepted",
  },
  "keyvalue.maxmemoryError": {
    message: "Не удалось обновить политику maxmemory. Попробуйте ещё раз.",
    description: "Toast when an eviction-policy update fails",
  },
  "keyvalue.persistenceTitle": {
    message: "Режим сохранения данных",
    description: "Key Value detail persistence-mode card title",
  },
  "keyvalue.persistenceDescription": {
    message:
      "Смена режима перезапускает хранилище и прерывает подключения, пока готовятся данные. Переключение между «Журнал + снимок» и «Только снимок» сохраняет данные. Переключение на «Выключено» или с него удаляет все данные.",
    description: "Key Value detail persistence-mode card description",
  },
  "keyvalue.persistenceLabel": {
    message: "Режим сохранения данных",
    description: "Accessible label for the persistence-mode select",
  },
  "keyvalue.persistenceEdit": {
    message: "Изменить режим сохранения данных",
    description:
      "Accessible label for the persistence-mode edit-in-place pencil",
  },
  "keyvalue.persistenceSuccess": {
    message: "Обновление режима сохранения данных до {mode}…",
    description: "Toast after a persistence-mode update is accepted",
  },
  "keyvalue.persistenceConfirmTitle": {
    message: "Удалить все данные и изменить режим сохранения?",
    description: "Destructive persistence-mode confirmation title",
  },
  "keyvalue.persistenceConfirmDescription": {
    message:
      "Переключение на «Выключено» или с него удаляет все данные в {name} и перезапускает хранилище. Это действие нельзя отменить.",
    description: "Destructive persistence-mode confirmation description",
  },
  "keyvalue.persistenceConfirmAction": {
    message: "Удалить данные и изменить режим",
    description: "Destructive persistence-mode confirmation action",
  },
  "keyvalue.persistenceError": {
    message: "Не удалось обновить режим сохранения данных. Попробуйте ещё раз.",
    description: "Toast when a persistence-mode update fails",
  },
  "keyvalue.networkingTitle": {
    message: "Сеть",
    description: "Detail-page Networking card title (IP allowlist)",
  },
  "keyvalue.networkingDescription": {
    message:
      "Ограничьте, с каких исходных IP можно обращаться к внешней точке доступа.",
    description: "Networking card subtitle",
  },
  "keyvalue.networkingHint": {
    message:
      "Сохранение правил CIDR включает внешний доступ для подходящих исходных IP. Сохранение пустого списка блокирует внешние подключения. Внутренний доступ не меняется.",
    description: "Networking IP allowlist helper text",
  },
  "keyvalue.networkingHintRestart": {
    message:
      "Сохранение правил CIDR включает внешний доступ для подходящих исходных IP. У этого хранилища ещё никогда не было внешнего доступа, поэтому первое сохранение перезапустит его, и подключения прервутся до минуты. Сохранение пустого списка блокирует внешние подключения.",
    description:
      "Networking IP allowlist helper text when enabling external access restarts the store (its first enable)",
  },
  "keyvalue.networkingRestartConfirmTitle": {
    message: "Удалить все данные и включить внешний доступ?",
    description:
      "Confirmation title when a first external-access enable restarts a persistence-Off store",
  },
  "keyvalue.networkingRestartConfirmDescription": {
    message:
      "Включение внешнего доступа впервые перезапускает {name}. Его режим сохранения данных — «Выключено», поэтому при перезапуске будут удалены все данные. Это действие нельзя отменить.",
    description:
      "Confirmation description when a first external-access enable restarts a persistence-Off store",
  },
  "keyvalue.networkingRestartConfirmAction": {
    message: "Удалить данные и сохранить правила",
    description:
      "Confirmation action when a first external-access enable restarts a persistence-Off store",
  },
  "keyvalue.networkingInternalOnly": {
    message:
      "Внешние подключения отключены. Сохраните одно или несколько правил IP, чтобы включить доступ.",
    description: "External access is disabled; saving rules enables it",
  },
  "keyvalue.networkingEmpty": {
    message:
      "Правил IP нет. Сохраните пустой список, чтобы заблокировать внешние подключения.",
    description: "Empty datastore allowlist draft and the effect of saving it",
  },
  "keyvalue.networkingAdd": {
    message: "Добавить",
    description: "Networking button to add a CIDR to the draft allowlist",
  },
  "keyvalue.networkingEntryDescription": {
    message: "Описание (необязательно)",
    description:
      "Placeholder for the optional per-entry allowlist description input",
  },
  "keyvalue.networkingCIDR": {
    message: "Новый блок CIDR",
    description: "Accessible label for the add-row CIDR input",
  },
  "keyvalue.networkingCIDRRule": {
    message: "Блок CIDR для правила {number}",
    description: "Accessible label for an existing allowlist CIDR row",
  },
  "keyvalue.networkingDescriptionLabel": {
    message: "Описание нового правила",
    description: "Accessible label for the add-row description input",
  },
  "keyvalue.networkingDescriptionRule": {
    message: "Описание для правила {number}",
    description: "Accessible label for an existing allowlist description row",
  },
  "keyvalue.networkingRemove": {
    message: "Удалить {cidr}",
    description: "Accessible label to remove a CIDR chip",
  },
  "keyvalue.networkingMoveUp": {
    message: "Переместить {cidr} выше",
    description: "Accessible label to move an allowlist entry earlier",
  },
  "keyvalue.networkingMoveDown": {
    message: "Переместить {cidr} ниже",
    description: "Accessible label to move an allowlist entry later",
  },
  "keyvalue.networkingInvalid": {
    message: "Введите уникальные корректные блоки CIDR IPv4 или IPv6.",
    description: "Validation error for Key Value allowlist entries",
  },
  "keyvalue.networkingSave": {
    message: "Сохранить список",
    description: "Networking button to persist the allowlist",
  },
  "keyvalue.networkingSaved": {
    message: "Список обновлён.",
    description: "Toast after saving the IP allowlist",
  },
  "keyvalue.networkingError": {
    message: "Не удалось сохранить список. Попробуйте ещё раз.",
    description: "Toast when saving the allowlist fails",
  },
  "keyvalue.nameEdit": {
    message: "Изменить название Key Value",
    description: "Accessible label for the Key Value name edit-in-place pencil",
  },
  "keyvalue.nameDescription": {
    message:
      "Измените отображаемое название. ID Key Value и все данные подключения остаются прежними.",
    description: "Key Value detail rename row description",
  },
  "keyvalue.nameSuccess": {
    message: "Хранилище Key Value переименовано в {name}.",
    description: "Toast after a Key Value rename succeeds",
  },
  "keyvalue.nameError": {
    message:
      "Не удалось переименовать хранилище Key Value. Попробуйте ещё раз.",
    description: "Toast when a Key Value rename fails unexpectedly",
  },
  "keyvalue.nameConflict": {
    message:
      "Хранилище Key Value с таким названием уже существует в этом рабочем пространстве.",
    description: "Toast when a Key Value rename collides with another name",
  },
  "keyvalue.nameInvalid": {
    message:
      "Введите название длиной до 100 символов без переводов строк и управляющих символов.",
    description: "Key Value rename validation message",
  },
  // --- Sidebar and page headings (w3/m649) ---
  "keyvalue.navInfo": {
    message: "Информация",
    description:
      "Key Value sidebar entry and page heading: the instance's Info page (general details, plan, connections, networking)",
  },
  "keyvalue.navLogs": {
    message: "Логи",
    description:
      "Key Value sidebar entry and page heading: the instance's Logs page",
  },
  "keyvalue.navMetrics": {
    message: "Метрики",
    description:
      "Key Value sidebar entry and page heading: the instance's Metrics page",
  },
  "keyvalue.logsRangeLabel": {
    message: "Период времени",
    description: "Accessible label for the log time-range select",
  },
  "keyvalue.logsAllInstances": {
    message: "Все инстансы",
    description: "Default option in the instance filter (no filter applied)",
  },
  "keyvalue.logsSearchPlaceholder": {
    message: "Поиск по логам…",
    description: "Placeholder for the log text-search input",
  },
  "keyvalue.logsLoading": {
    message: "Загрузка логов…",
    description: "Loading state message while fetching logs",
  },
  "keyvalue.logsEmptyTitle": {
    message: "Строк логов нет",
    description: "Empty state title when no log lines are returned",
  },
  "keyvalue.logsEmptyBody": {
    message:
      "За этот период нет строк логов. Valkey логирует события пространства ключей, медленные запросы и сообщения при запуске.",
    description: "Empty state body when no log lines and no filters active",
  },
  "keyvalue.logsUnavailableTitle": {
    message: "Логи недоступны",
    description: "Empty state title when the logs source is not configured",
  },
  "keyvalue.logsUnavailableBody": {
    message:
      "Источник логов платформы не настроен. Обратитесь к администратору платформы, чтобы включить BEX_LOKI_URL.",
    description: "Empty state body when BEX_LOKI_URL is not set",
  },
  "keyvalue.logsUnauthorizedTitle": {
    message: "Доступ запрещён",
    description: "Empty state title when the caller lacks can_view_logs",
  },
  "keyvalue.logsUnauthorizedBody": {
    message: "У вас нет прав на просмотр логов этого хранилища Key Value.",
    description: "Empty state body for a 403 on the logs query",
  },
  "keyvalue.fieldNameDescription": {
    message: "Уникальное название инстанса Key Value.",
    description: "Create-Key-Value Name row description",
  },
  "keyvalue.fieldVersionDescription": {
    message: "Основная версия Valkey, на которой работает инстанс.",
    description: "Create-Key-Value Version row description",
  },
  "keyvalue.fieldPlanDescription": {
    message: "Определяет доступную память и лимит подключений инстанса.",
    description:
      "Create-Key-Value plan row description (Render: the plan determines RAM and connection limit)",
  },
  "keyvalue.connectInternalTitle": {
    message: "Внутренний URL Redis",
    description:
      "Header Connect menu, Internal tab: the private-network URL's title (Render's copy, which names Key Value by its type name Redis, w3/m675)",
  },
  "keyvalue.connectExternalTitle": {
    message: "Внешний URL Redis",
    description:
      "Header Connect menu, External tab: the public URL's title (Render's copy, w3/m675)",
  },
  "keyvalue.connectCliTitle": {
    message: "Команда Redis CLI",
    description:
      "Header Connect menu, External tab: the `bex kv-cli <id>` command's title (Render's copy, w3/m675)",
  },
};

export default ruKeyValue;
