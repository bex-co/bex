import type { TranslationEntry } from "@/i18n";

const ruWorkspaces: Record<string, TranslationEntry> = {
  "workspaces.switcherEmpty": {
    message: "Выберите рабочее пространство",
    description: "Switcher trigger label before any workspace is selected",
  },
  "workspaces.switcherLabel": {
    message: "Рабочие пространства",
    description: "Switcher dropdown label above the workspace list",
  },
  "workspaces.switcherBilling": {
    message: "Оплата",
    description: "Switcher menu item linking to /billing",
  },
  "workspaces.switcherSettings": {
    message: "Настройки рабочего пространства",
    description: "Switcher menu item linking to /workspace/settings",
  },
  "workspaces.switcherNew": {
    message: "+ Новое рабочее пространство",
    description: "Switcher menu item linking to /new/workspace",
  },
  "workspaces.newTitle": {
    message: "Новое рабочее пространство",
    description: "New-workspace page document title",
  },
  "workspaces.planPickerLabel": {
    message: "Тариф",
    description: "Accessible label for the plan-card radiogroup",
  },
  "workspaces.planHobbyName": {
    message: "Hobby",
    description: "Workspace plan card name",
  },
  "workspaces.planHobbyBilling": {
    message: "$0/мес.",
    description:
      "Workspace plan card billing label (pricing.yaml hobby usdPerMonth)",
  },
  "workspaces.planHobbyDescription": {
    message: "Для тех, кто только начинает.",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planHobbyBulletMembers": {
    message: "1 участник",
    description: "Hobby plan LimitsFor.MaxMembers bullet",
  },
  "workspaces.planHobbyBulletServices": {
    message: "До 25 сервисов",
    description: "Hobby plan LimitsFor.MaxServices bullet",
  },
  "workspaces.planHobbyBulletWorkspaces": {
    message: "5 рабочих пространств Hobby на пользователя",
    description: "Hobby plan LimitsFor.MaxWorkspacesPerUser bullet",
  },
  "workspaces.planProName": {
    message: "Pro",
    description: "Workspace plan card name",
  },
  "workspaces.planProBilling": {
    message: "$17.50/мес.",
    description:
      "Workspace plan card billing label (pricing.yaml pro usdPerMonth)",
  },
  "workspaces.planProDescription": {
    message: "Для небольших команд, которые выпускают продукт вместе.",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planProBulletMembers": {
    message: "Неограниченное число участников",
    description: "Pro plan unlimited MaxMembers bullet",
  },
  "workspaces.planProBulletServices": {
    message: "Неограниченное число сервисов",
    description: "Pro plan unlimited MaxServices bullet",
  },
  "workspaces.planScaleName": {
    message: "Scale",
    description: "Workspace plan card name",
  },
  "workspaces.planScaleBilling": {
    message: "$349.30/мес.",
    description:
      "Workspace plan card billing label (pricing.yaml scale usdPerMonth)",
  },
  "workspaces.planScaleDescription": {
    message: "Для растущих команд, которым нужны дополнительные роли.",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planScaleBulletMembers": {
    message: "Неограниченное число участников",
    description: "Scale plan unlimited MaxMembers bullet",
  },
  "workspaces.planScaleBulletServices": {
    message: "Неограниченное число сервисов",
    description: "Scale plan unlimited MaxServices bullet",
  },
  "workspaces.planScaleBulletRoles": {
    message: "Дополнительные роли (участник-исполнитель, наблюдатель, оплата)",
    description: "Scale plan AllowedRoles beyond Pro bullet",
  },
  "workspaces.planEnterpriseName": {
    message: "Enterprise",
    description: "Workspace plan card name",
  },
  "workspaces.planEnterpriseBilling": {
    message: "Индивидуальные условия",
    description: "Workspace plan card billing label",
  },
  "workspaces.planEnterpriseDescription": {
    message: "Индивидуальные лимиты и поддержка.",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planEnterpriseBulletLimits": {
    message: "Индивидуальные лимиты",
    description: "Enterprise plan custom-limits bullet",
  },
  "workspaces.planEnterpriseBulletSupport": {
    message: "Индивидуальная поддержка",
    description: "Enterprise plan custom-support bullet",
  },
  "workspaces.planSelect": {
    message: "Выбрать тариф",
    description: "Unselected plan card action label",
  },
  "workspaces.planSelected": {
    message: "Тариф выбран",
    description: "Selected plan card action label",
  },
  "workspaces.planUsageBillingNote": {
    message:
      "Использование сервисов и хранилищ данных оплачивается отдельно в зависимости от уровня ресурса.",
    description: "Billing note below the workspace plan picker",
  },
  "workspaces.createTitle": {
    message: "Создать рабочее пространство",
    description: "/new/workspace page heading",
  },
  "workspaces.createDescription": {
    message: "Укажите данные, платёжный контакт, тариф и способ оплаты.",
    description: "/new/workspace page subtitle",
  },
  "workspaces.detailsTitle": {
    message: "Данные рабочего пространства",
    description: "/new/workspace details section heading",
  },
  "workspaces.billingEmail": {
    message: "Email для оплаты",
    description: "/new/workspace billing email label",
  },
  "workspaces.billingEmailHelp": {
    message:
      "Сюда приходят квитанции и уведомления об оплате этого рабочего пространства.",
    description: "Editable paid-plan billing email help",
  },
  "workspaces.billingEmailHobbyHelp": {
    message:
      "Для рабочих пространств Hobby email для оплаты совпадает с email вашей учётной записи.",
    description: "Read-only Hobby billing email help",
  },
  "workspaces.billingEmailError": {
    message: "Введите корректный email для оплаты.",
    description: "Billing email validation error",
  },
  "workspaces.fieldSlug": {
    message: "Слаг рабочего пространства",
    description: "/new/workspace slug field label",
  },
  "workspaces.fieldSlugHelp": {
    message:
      "Уникальный ключ для CLI и API. Строчные буквы, цифры и дефисы, 1–30 символов.",
    description: "/new/workspace slug helper text",
  },
  "workspaces.fieldSlugError": {
    message:
      "Только строчные буквы, цифры и дефисы, 1–30 символов, без дефиса в начале и в конце.",
    description: "Workspace slug validation error",
  },
  "workspaces.paymentTitle": {
    message: "Способ оплаты",
    description: "/new/workspace payment panel heading",
  },
  "workspaces.paymentDescription": {
    message:
      "Оплата отдельная для каждого рабочего пространства. Этот способ оплаты относится только к новому рабочему пространству.",
    description: "/new/workspace workspace-specific payment copy",
  },
  "workspaces.paymentRequired": {
    message: "Для этого рабочего пространства нужен способ оплаты.",
    description: "Required payment policy copy",
  },
  "workspaces.paymentOptional": {
    message: "Добавьте способ оплаты сейчас или продолжите без него.",
    description: "Optional payment policy copy",
  },
  "workspaces.paymentSelfHosted": {
    message:
      "В этой самостоятельно размещённой установке приём платежей отключён.",
    description: "Billing-off create-flow copy",
  },
  "workspaces.paymentAdd": {
    message: "Добавить способ оплаты",
    description: "Open Payment Element action",
  },
  "workspaces.paymentSave": {
    message: "Сохранить способ оплаты",
    description: "Confirm SetupIntent action",
  },
  "workspaces.paymentAdded": {
    message: "Способ оплаты подтверждён для этого рабочего пространства.",
    description: "Successful payment setup state",
  },
  "workspaces.paymentError": {
    message: "Не удалось подтвердить этот способ оплаты. Повторите попытку.",
    description: "Payment Element fallback error",
  },
  "workspaces.fieldName": {
    message: "Название",
    description: "Workspace name field label (shared by create + settings)",
  },
  "workspaces.fieldDisplayNamePlaceholder": {
    message: "напр. Acme Staging",
    description: "/new/workspace freeform name placeholder",
  },
  "workspaces.fieldNamePlaceholder": {
    message: "напр. acme-staging",
    description: "Workspace name field placeholder",
  },
  "workspaces.fieldNameError": {
    message: "Введите название длиной 1–100 символов без переводов строки.",
    description: "Workspace name validation error",
  },
  "workspaces.fieldPlan": {
    message: "Тариф",
    description: "Workspace plan field label (create picker + settings badge)",
  },
  "workspaces.createErrorTitle": {
    message: "Не удалось создать рабочее пространство",
    description: "/new/workspace inline error alert title",
  },
  "workspaces.createCancel": {
    message: "Отмена",
    description: "/new/workspace cancel button",
  },
  "workspaces.createCancelError": {
    message:
      "Не удалось отменить создание рабочего пространства. Повторите попытку.",
    description:
      "/new/workspace toast when cancelling the pending creation fails",
  },
  "workspaces.createSubmit": {
    message: "Создать рабочее пространство",
    description: "/new/workspace submit button",
  },
  "workspaces.createSuccess": {
    message: "Создано: {name}",
    description: "Toast on a successful workspace create",
  },
  "workspaces.createError": {
    message: "Не удалось создать рабочее пространство",
    description: "Fallback toast/inline message on a failed create",
  },
  "workspaces.creationLoadError": {
    message:
      "Не удалось загрузить настройку рабочего пространства. Обновите страницу и повторите попытку.",
    description:
      "Inline message when the workspace-creation plan policy or a resumed setup attempt fails to load",
  },
  "workspaces.settingsTitle": {
    message: "Настройки рабочего пространства",
    description: "Workspace settings page and card title",
  },
  "workspaces.settingsDescription": {
    message:
      "Переименуйте рабочее пространство или посмотрите его тариф и метаданные.",
    description: "Workspace settings card description",
  },
  "workspaces.settingsEmpty": {
    message: "Рабочее пространство не выбрано.",
    description: "Workspace settings page empty state",
  },
  "workspaces.settingsNavigation": {
    message: "Разделы настроек",
    description:
      "Accessible label for the workspace settings section navigation",
  },
  "workspaces.renameSubmit": {
    message: "Сохранить",
    description: "Workspace rename form submit button",
  },
  "workspaces.renameErrorTitle": {
    message: "Не удалось переименовать рабочее пространство",
    description: "Workspace rename inline error alert title",
  },
  "workspaces.renameSuccess": {
    message: "Переименовано в {name}",
    description: "Toast on a successful rename",
  },
  "workspaces.renameError": {
    message: "Не удалось переименовать рабочее пространство",
    description: "Fallback toast/inline message on a failed rename",
  },
  "workspaces.fieldId": {
    message: "ID рабочего пространства",
    description: "Workspace settings metadata field label",
  },
  "workspaces.fieldCreatedAt": {
    message: "Создано",
    description: "Workspace settings metadata field label",
  },
  "workspaces.dangerZoneTitle": {
    message: "Опасная зона",
    description: "Workspace settings delete section title",
  },
  "workspaces.dangerZoneDescription": {
    message:
      "Это удалит все ресурсы и данные вашего рабочего пространства. Все сервисы, хранилища данных и переменные окружения будут потеряны. Это действие нельзя отменить.",
    description: "Workspace settings delete section description",
  },
  "workspaces.deleteConfirmLabel": {
    message: "Введите {phrase} ниже для подтверждения.",
    description:
      "Body prompt naming the exact 'sudo delete workspace <name>' phrase (rendered bold by SudoCommandField)",
  },
  "workspaces.deleteErrorTitle": {
    message: "Не удалось удалить рабочее пространство",
    description: "Delete danger-zone inline error alert title",
  },
  "workspaces.deleteSubmit": {
    message: "Удалить рабочее пространство",
    description: "Delete danger-zone submit button",
  },
  "workspaces.deleteSuccess": {
    message: "Удалено: {name}",
    description: "Toast on a successful delete",
  },
  "workspaces.deleteError": {
    message: "Не удалось удалить рабочее пространство",
    description: "Fallback toast/inline message on a failed delete",
  },
  "workspaces.changePlanTrigger": {
    message: "Сменить тариф",
    description:
      "Workspace settings plan-badge link opening the change-plan dialog",
  },
  "workspaces.changePlanTitle": {
    message: "Смена тарифа",
    description: "Change-plan dialog title",
  },
  "workspaces.changePlanDescription": {
    message:
      "Выберите новый тариф для этого рабочего пространства. Оплата не требуется — тариф меняется сразу.",
    description: "Change-plan dialog description",
  },
  "workspaces.changePlanCancel": {
    message: "Отмена",
    description: "Change-plan dialog cancel button",
  },
  "workspaces.changePlanSubmit": {
    message: "Сменить тариф",
    description: "Change-plan dialog submit button",
  },
  "workspaces.changePlanErrorTitle": {
    message: "Не удалось сменить тариф",
    description: "Change-plan dialog inline error alert title",
  },
  "workspaces.changePlanSuccess": {
    message: "Тариф изменён на {plan}",
    description: "Toast on a successful plan change",
  },
  "workspaces.changePlanError": {
    message: "Не удалось сменить тариф рабочего пространства",
    description: "Fallback toast/inline message on a failed plan change",
  },
  "workspaces.generalTitle": {
    message: "Общие",
    description: "Title of the general card on the workspace settings page",
  },
  "workspaces.ipRulesTitle": {
    message: "Правила входящих IP",
    description:
      "Workspace Settings → Inbound IP Rules card title and section link (w9/m388)",
  },
  "workspaces.ipRulesDescription": {
    message:
      "Выберите, какие IP-адреса могут обращаться к каждому Web Service, Static Site, базе данных Postgres и экземпляру Key Value в этом рабочем пространстве. Эти правила действуют вместе с правилами окружения и сервиса, поэтому источник должен быть разрешён на каждом уровне. Private Service, Background Worker и Cron Job это не затрагивает.",
    description:
      "Inbound IP Rules card description: scope and layering of workspace rules (Render's workspace-level inbound IP rules, w9/m388)",
  },
  "workspaces.ipRulesHint": {
    message:
      "Введите блок CIDR (напр. 203.0.113.0/24) и нажмите «Добавить источник». Без правил весь входящий трафик блокируется; 0.0.0.0/0 и ::/0 разрешают все источники IPv4 и IPv6.",
    description: "Hint above the editable workspace inbound IP rules (w9/m388)",
  },
  "workspaces.ipRulesBlocked": {
    message:
      "Нет правил: весь входящий трафик к сервисам и хранилищам данных этого рабочего пространства заблокирован. Добавьте 0.0.0.0/0 и ::/0, чтобы разрешить все источники.",
    description:
      "Warning shown when the workspace rule list is empty (deny-all), saved or about to be saved (w9/m388)",
  },
  "workspaces.ipRulesOpen": {
    message: "Открыто для всех исходных IP",
    description:
      "Shown in the read-only view when the workspace rules restrict nothing (Render's allow-all default, w9/m388)",
  },
  "workspaces.ipRulesScalePlanOnly": {
    message:
      "Правила входящих IP для рабочего пространства доступны на тарифе Scale и выше. Уже заданные правила продолжают действовать и могут быть сброшены до «разрешить всё».",
    description:
      "Notice replacing the editable workspace rules below the Scale plan (w9/m388)",
  },
  "workspaces.ipRulesAdminOnly": {
    message:
      "Изменять эти правила могут только администраторы рабочего пространства.",
    description:
      "Notice for a member who cannot edit the workspace inbound IP rules, and the forbidden-save explanation (w9/m388)",
  },
  "workspaces.ipRulesPlanRefused": {
    message:
      "Тариф этого рабочего пространства не включает правила входящих IP. Перейдите на Scale или выше, чтобы ограничивать трафик; правила всегда можно сбросить до «разрешить всё».",
    description:
      "Inline explanation when bex-api refuses a workspace rule change with PLAN_FEATURE_REQUIRED (w9/m388)",
  },
  "workspaces.ipRulesUnreadable": {
    message:
      "Не удалось загрузить правила входящих IP этого рабочего пространства.",
    description:
      "Shown in place of the rules when the workspace inbound IP rules read fails (w9/m388)",
  },
  "workspaces.ipRulesResetAllowAll": {
    message: "Сбросить до «разрешить всё»",
    description:
      "Button replacing the workspace inbound IP rules with the allow-all pair below the Scale plan (w9/m388)",
  },
  "workspaces.ipRulesCIDR": {
    message: "Новый блок CIDR",
    description:
      "Accessible label for the add-row CIDR input of the workspace inbound IP rules",
  },
  "workspaces.ipRulesCIDRRule": {
    message: "Блок CIDR для правила {number}",
    description:
      "Accessible label for an existing workspace inbound IP rule CIDR",
  },
  "workspaces.ipRulesDescriptionLabel": {
    message: "Описание нового правила",
    description:
      "Accessible label for the add-row description input of the workspace inbound IP rules",
  },
  "workspaces.ipRulesDescriptionRule": {
    message: "Описание для правила {number}",
    description:
      "Accessible label for an existing workspace inbound IP rule description",
  },
  "workspaces.ipRulesDescriptionPlaceholder": {
    message: "Описание (необязательно)",
    description: "Placeholder for a workspace inbound IP rule description",
  },
  "workspaces.ipRulesAdd": {
    message: "Добавить источник",
    description: "Button adding a CIDR to the draft workspace inbound IP rules",
  },
  "workspaces.ipRulesCancel": {
    message: "Отмена",
    description:
      "Button discarding unsaved changes to the workspace inbound IP rules",
  },
  "workspaces.ipRulesSave": {
    message: "Сохранить правила",
    description:
      "Button saving the workspace inbound IP rules (setWorkspaceIPAllowList)",
  },
  "workspaces.ipRulesInvalid": {
    message: "Введите уникальные корректные блоки CIDR IPv4 или IPv6.",
    description:
      "Inline validation when a workspace rule block is not a unique, valid CIDR",
  },
  "workspaces.ipRulesRemove": {
    message: "Удалить {cidr}",
    description:
      "Accessible label for a workspace inbound IP rule remove button",
  },
  "workspaces.ipRulesMoveUp": {
    message: "Переместить {cidr} вверх",
    description: "Accessible label to move a workspace inbound IP rule earlier",
  },
  "workspaces.ipRulesMoveDown": {
    message: "Переместить {cidr} вниз",
    description: "Accessible label to move a workspace inbound IP rule later",
  },
  "workspaces.ipRulesSaved": {
    message: "Правила входящих IP сохранены",
    description: "Toast after setWorkspaceIPAllowList succeeds (w9/m388)",
  },
  "workspaces.ipRulesSaveErrorTitle": {
    message: "Не удалось сохранить правила входящих IP",
    description:
      "Title of the inline alert when a workspace rule change is refused (w9/m388)",
  },
  "workspaces.ipRulesSaveError": {
    message: "Не удалось сохранить правила входящих IP. Повторите попытку.",
    description:
      "Fallback inline message when a workspace rule change fails without a server reason (w9/m388)",
  },
  "workspaces.twoFactorTitle": {
    message: "Двухфакторная аутентификация",
    description:
      "Title of the two-factor card on the workspace settings page, and its quick-nav label (w22/m402)",
  },
  "workspaces.twoFactorDescription": {
    message:
      "Участники без двухфакторной аутентификации (2FA) не могут получить доступ к ресурсам и настройкам этого рабочего пространства, пока не включат её. Их API-ключи и SSH-ключи до этого момента тоже блокируются.",
    description:
      "Description under the two-factor card title; follows Render's login-settings wording",
  },
  "workspaces.twoFactorSwitchLabel": {
    message: "Требовать двухфакторную аутентификацию (2FA)",
    description:
      "Label of the switch that requires 2FA of every member (Render's control name)",
  },
  "workspaces.twoFactorStatusRequired": {
    message: "Требуется",
    description:
      "Read-only status shown to a non-admin when the workspace requires 2FA",
  },
  "workspaces.twoFactorStatusNotRequired": {
    message: "Не требуется",
    description:
      "Read-only status shown to a non-admin when the workspace does not require 2FA",
  },
  "workspaces.twoFactorAdminOnly": {
    message:
      "Изменить эту настройку могут только администраторы рабочего пространства.",
    description: "Hint shown to a non-admin in place of the switch",
  },
  "workspaces.twoFactorEnabledToast": {
    message: "В {name} теперь требуется двухфакторная аутентификация",
    description: "Toast after an admin turns the 2FA requirement on",
  },
  "workspaces.twoFactorDisabledToast": {
    message: "В {name} двухфакторная аутентификация больше не требуется",
    description: "Toast after an admin turns the 2FA requirement off",
  },
  "workspaces.twoFactorErrorTitle": {
    message: "Не удалось изменить настройку двухфакторной аутентификации",
    description: "Title of the inline error when the 2FA switch is refused",
  },
  "workspaces.twoFactorError": {
    message:
      "Не удалось изменить настройку двухфакторной аутентификации. Повторите попытку.",
    description:
      "Generic message when the 2FA switch fails without a recognised code",
  },
  "workspaces.twoFactorErrorAdminNotEnrolled": {
    message:
      "Включите двухфакторную аутентификацию для своей учётной записи, прежде чем требовать её в этом рабочем пространстве, иначе вы потеряете к нему доступ.",
    description:
      "Refusal for code WORKSPACE_2FA_ADMIN_NOT_ENROLLED: the acting admin has no second factor",
  },
  "workspaces.twoFactorErrorUnavailable": {
    message:
      "Обязательная двухфакторная аутентификация недоступна в этой установке, поэтому включить её нельзя.",
    description:
      "Refusal for code WORKSPACE_2FA_UNAVAILABLE: the deployment has no identity lookup",
  },
  "workspaces.twoFactorEnrollLink": {
    message: "Настроить двухфакторную аутентификацию",
    description:
      "Link to account security, in the admin-not-enrolled refusal and on the blocked page",
  },
  "workspaces.twoFactorBlockedTitle": {
    message: "{workspace} требует двухфакторную аутентификацию",
    description: "Heading of the page a blocked member sees (w22/m402)",
  },
  "workspaces.twoFactorBlockedBody": {
    message:
      "Администратор этого рабочего пространства требует, чтобы все участники использовали двухфакторную аутентификацию (2FA). Пока вы её не включите, вы не сможете получить доступ к ресурсам и настройкам этого рабочего пространства, а ваши API-ключи и SSH-ключи нельзя будет с ним использовать.",
    description: "Why the blocked page is shown",
  },
  "workspaces.twoFactorBlockedRecheck": {
    message: "Я включил(а) её, проверить снова",
    description:
      "Button on the blocked page that retries the workspace after enrolling",
  },
  "workspaces.twoFactorBlockedRecheckHint": {
    message:
      "После включения 2FA восстановление доступа может занять до минуты.",
    description: "Hint under the blocked page's check-again button",
  },
  "workspaces.twoFactorBlockedSwitchLabel": {
    message: "Перейти в другое рабочее пространство",
    description: "Label of the workspace picker on the blocked page",
  },
  "workspaces.twoFactorBlockedSwitchPlaceholder": {
    message: "Выберите рабочее пространство",
    description: "Placeholder of the workspace picker on the blocked page",
  },
  "workspaces.twoFactorBlockedNoOtherWorkspaces": {
    message: "Других рабочих пространств нет",
    description:
      "Placeholder of the blocked page's workspace picker when the member belongs to no other workspace",
  },
  "workspaces.overlappingDeployPolicyTitle": {
    message: "Политика перекрывающихся деплоев",
    description:
      "Workspace Settings section title, Render's name for the setting (w4/653)",
  },
  "workspaces.overlappingDeployPolicyDescription": {
    message:
      "Выберите, что произойдёт, если деплой сервиса запускается, пока другой его деплой ещё выполняется.",
    description:
      "Workspace Settings → Overlapping Deploy Policy description (w4/653)",
  },
  "workspaces.overlappingDeployPolicyWait": {
    message: "Ожидание",
    description: "The Wait policy's label, Render's wording (w4/653)",
  },
  "workspaces.overlappingDeployPolicyWaitDescription": {
    message:
      "Дождаться завершения выполняющегося деплоя, а затем сразу перейти к последнему из запущенных деплоев.",
    description:
      "The Wait policy's sentence, Render's wording (render.com/docs/deploys, w4/653)",
  },
  "workspaces.overlappingDeployPolicyOverride": {
    message: "Переопределение",
    description: "The Override policy's label, Render's wording (w4/653)",
  },
  "workspaces.overlappingDeployPolicyOverrideDescription": {
    message: "Сразу отменить выполняющийся деплой и запустить новый.",
    description:
      "The Override policy's sentence, Render's wording (render.com/docs/deploys, w4/653)",
  },
  "workspaces.overlappingDeployPolicySave": {
    message: "Сохранить",
    description: "Saves the selected Overlapping Deploy Policy (w4/653)",
  },
  "workspaces.overlappingDeployPolicySaving": {
    message: "Сохранение…",
    description: "The Save button while the policy saves (w4/653)",
  },
  "workspaces.overlappingDeployPolicySaved": {
    message: "Политика перекрывающихся деплоев сохранена",
    description:
      "Toast after setWorkspaceOverlappingDeployPolicy succeeds (w4/653)",
  },
  "workspaces.overlappingDeployPolicyError": {
    message:
      "Не удалось сохранить политику перекрывающихся деплоев. Повторите попытку.",
    description:
      "Fallback toast when a policy save fails without a server reason (w4/653)",
  },
  "workspaces.overlappingDeployPolicyAdminOnly": {
    message:
      "Изменить эту политику могут только администраторы рабочего пространства.",
    description:
      "Shown in place of Save to a member who cannot manage the workspace (w4/653)",
  },
};

export default ruWorkspaces;
