import type { TranslationEntry } from "@/i18n";

const ruWorkspaces: Record<string, TranslationEntry> = {
  "workspaces.idCopied": {
    message: "ID рабочего пространства скопирован",
    description: "Toast after copying the workspace ID",
  },
  "workspaces.idCopyFailed": {
    message: "Не удалось скопировать ID рабочего пространства",
    description: "Toast when copying the workspace ID fails",
  },
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
    message: "$349.30/мес. за организацию",
    description:
      "Workspace plan card billing label (pricing.yaml scale usdPerMonth, per: organization; w7/m464)",
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
    message: "Индивидуальные условия для организации",
    description:
      "Workspace plan card billing label: Enterprise is charged once per organization like Scale (w7/m464)",
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
      "Plan link on Workspace Settings, the plan gates and the webhooks gate, opening Render's plan page (w3/m708)",
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
  "workspaces.loginRequirementStatusRequired": {
    message: "Требуется",
    description:
      "Read-only status shown to a non-admin when the workspace requires 2FA",
  },
  "workspaces.loginRequirementStatusNotRequired": {
    message: "Не требуется",
    description:
      "Read-only status shown to a non-admin when the workspace does not require 2FA",
  },
  "workspaces.loginRequirementAdminOnly": {
    message:
      "Изменить эту настройку могут только администраторы рабочего пространства.",
    description: "Hint shown to a non-admin in place of the switch",
  },
  "workspaces.twoFactorProPlanOnly": {
    message:
      "Обязательная двухфакторная аутентификация доступна на тарифе Pro и выше. Рабочее пространство, где она уже включена, сохраняет её, пока администратор её не отключит.",
    description:
      "Notice on the two-factor card below the Pro plan, and the refusal for code PLAN_FEATURE_REQUIRED (Render's plan table, w7/070)",
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
  "workspaces.loginBlockedSwitchLabel": {
    message: "Перейти в другое рабочее пространство",
    description: "Label of the workspace picker on the blocked page",
  },
  "workspaces.loginBlockedSwitchPlaceholder": {
    message: "Выберите рабочее пространство",
    description: "Placeholder of the workspace picker on the blocked page",
  },
  "workspaces.loginBlockedNoOtherWorkspaces": {
    message: "Других рабочих пространств нет",
    description:
      "Placeholder of the blocked page's workspace picker when the member belongs to no other workspace",
  },
  "workspaces.googleLoginTitle": {
    message: "Вход через Google",
    description:
      "Title of the Google login card on the workspace settings page, and its quick-nav label (w7/m465)",
  },
  "workspaces.googleLoginDescription": {
    message:
      "Участники, которые не входят через аккаунт Google своего адреса электронной почты в bex, не могут получить доступ к ресурсам и настройкам этого рабочего пространства. API-ключи и SSH-ключи, созданные при входе другим способом, тоже блокируются.",
    description:
      "Description under the Google login card title; follows Render's login-settings wording",
  },
  "workspaces.googleLoginSwitchLabel": {
    message: "Требовать вход через Google",
    description:
      "Label of the switch that requires every member to sign in with Google (Render's control name)",
  },
  "workspaces.googleLoginProPlanOnly": {
    message:
      "Обязательный вход через Google доступен на тарифе Pro и выше. Рабочее пространство, где он уже включён, сохраняет его, пока администратор его не отключит.",
    description:
      "Notice on the Google login card below the Pro plan, and the refusal for code PLAN_FEATURE_REQUIRED (Render's plan table)",
  },
  "workspaces.googleLoginEnabledToast": {
    message: "В {name} теперь требуется вход через Google",
    description: "Toast after an admin turns the Google login requirement on",
  },
  "workspaces.googleLoginDisabledToast": {
    message: "В {name} вход через Google больше не требуется",
    description: "Toast after an admin turns the Google login requirement off",
  },
  "workspaces.googleLoginErrorTitle": {
    message: "Не удалось изменить настройку входа через Google",
    description:
      "Title of the inline error when the Google login switch is refused",
  },
  "workspaces.googleLoginError": {
    message:
      "Не удалось изменить настройку входа через Google. Повторите попытку.",
    description:
      "Generic message when the Google login switch fails without a recognised code",
  },
  "workspaces.googleLoginErrorAdminNotSignedIn": {
    message:
      "Войдите через аккаунт Google своего адреса электронной почты в bex, прежде чем требовать вход через Google в этом рабочем пространстве, иначе вы потеряете к нему доступ.",
    description:
      "Refusal for code WORKSPACE_GOOGLE_LOGIN_ADMIN_NOT_SIGNED_IN: the acting admin is not signed in with Google",
  },
  "workspaces.googleLoginErrorUnavailable": {
    message:
      "Обязательный вход через Google недоступен в этой установке, поэтому включить его нельзя.",
    description:
      "Refusal for code WORKSPACE_GOOGLE_LOGIN_UNAVAILABLE: the deployment cannot look up Google accounts",
  },
  "workspaces.googleLoginSignIn": {
    message: "Войти через Google",
    description:
      "Button that signs the session in again with Google and returns to the page: on the blocked page and in the admin's lock-out refusal",
  },
  "workspaces.googleLoginSignInError": {
    message: "Не удалось начать вход через Google. Повторите попытку.",
    description: "Shown when Kratos did not start the Google sign-in",
  },
  "workspaces.googleLoginBlockedTitle": {
    message: "{workspace} требует вход через Google",
    description:
      "Heading of the page a member who did not sign in with Google sees (w7/m465)",
  },
  "workspaces.googleLoginBlockedBody": {
    message:
      "Администратор этого рабочего пространства требует, чтобы все участники входили через Google. Пока вы не войдёте через аккаунт Google своего адреса электронной почты в bex, вы не сможете получить доступ к ресурсам и настройкам этого рабочего пространства, а API-ключи и SSH-ключи, созданные вами при входе другим способом, нельзя будет с ним использовать.",
    description: "Why the Google login blocked page is shown",
  },
  "workspaces.googleLoginBlockedHint": {
    message: "После входа вы вернётесь на эту страницу.",
    description: "Hint under the blocked page's Sign in with Google button",
  },
  "workspaces.requireTwoFactorConfirmTitle": {
    message: "Требовать двухфакторную аутентификацию в {workspace}?",
    description:
      "Title of the confirmation before an admin requires 2FA in the workspace (w7/m471)",
  },
  "workspaces.requireTwoFactorConfirm": {
    message:
      "Участники без двухфакторной аутентификации потеряют доступ к этому рабочему пространству, пока не включат её, а их API-ключи и SSH-ключи перестанут в нём работать.",
    description:
      "Body of the confirmation before an admin requires 2FA in the workspace (w7/m471)",
  },
  "workspaces.requireGoogleLoginConfirmTitle": {
    message: "Требовать вход через Google в {workspace}?",
    description:
      "Title of the confirmation before an admin requires Google login in the workspace (w7/m471)",
  },
  "workspaces.requireGoogleLoginConfirm": {
    message:
      "Участники, которые не вошли через аккаунт Google своего адреса электронной почты в bex, потеряют доступ к этому рабочему пространству, пока не войдут так, а API-ключи и SSH-ключи, созданные при входе другим способом, перестанут в нём работать.",
    description:
      "Body of the confirmation before an admin requires Google login in the workspace (w7/m471)",
  },
  "workspaces.requireConfirmAction": {
    message: "Требовать",
    description:
      "Confirm button of the workspace's require-2FA and require-Google-login confirmations (w7/m471)",
  },
  "workspaces.loginImpactRegionLabel": {
    message: "Кто потеряет доступ",
    description:
      "Accessible name of the confirmation's list of members and keys a login requirement would block (w7/m471)",
  },
  "workspaces.loginImpactChecking": {
    message: "Проверяем, кто потеряет доступ…",
    description:
      "Screen-reader text while the confirmation checks who a login requirement would block",
  },
  "workspaces.loginImpactError": {
    message:
      "Не удалось проверить, кто потеряет доступ. Вы всё равно можете включить требование.",
    description:
      "Shown in the confirmation when bex could not check who a login requirement would block; confirming stays possible",
  },
  "workspaces.loginImpactNobody": {
    message:
      "Никто не потеряет доступ: все участники соответствуют требованию, и ни один API-ключ или SSH-ключ не перестанет работать.",
    description:
      "Shown in the confirmation when a login requirement would block nobody",
  },
  "workspaces.loginImpactMembersTwoFactor": {
    message:
      "Каждый участник из списка потеряет доступ, пока не включит двухфакторную аутентификацию. Каждый API-ключ и SSH-ключ из списка перестанет работать, пока его владелец её не включит.",
    description:
      "Sentence above the members and keys requiring 2FA would block, after Render's SSO enforcement list",
  },
  "workspaces.loginImpactMembersGoogleLogin": {
    message:
      "Каждый участник из списка потеряет доступ, пока не войдёт через аккаунт Google своего адреса электронной почты в bex. Каждый API-ключ и SSH-ключ из списка перестанет работать; владелец может создать новый после входа через Google.",
    description:
      "Sentence above the members and keys requiring Google login would block, after Render's SSO enforcement list",
  },
  "workspaces.loginImpactLosesAccess": {
    message: "Теряет доступ",
    description: "Badge on a listed member who would lose access",
  },
  "workspaces.loginImpactKeepsAccess": {
    message: "Сохраняет доступ",
    description:
      "Badge on a listed member who keeps access while their listed keys stop working",
  },
  "workspaces.loginImpactApiKey": {
    message: "API-ключ {name}",
    description:
      "A listed API key that would stop working; {name} is the key's name",
  },
  "workspaces.loginImpactApiKeyInWorkspace": {
    message: "API-ключ {name} в {workspace}",
    description:
      "A listed API key on an org's confirmation; {workspace} is the workspace it belongs to",
  },
  "workspaces.loginImpactSshKey": {
    message: "SSH-ключ {name}",
    description:
      "A listed SSH key that would stop working; {name} is the key's name",
  },
  "workspaces.loginImpactUnattributed": {
    message:
      "API-ключи без записанного создателя или с создателем, который больше не участник, перестанут работать:",
    description:
      "Heading of the API keys no current member created, which would stop working",
  },
  "workspaces.loginImpactGoogleSessions": {
    message:
      "Участников, которые соответствуют требованию, но сейчас вошли другим способом, попросят войти через Google.",
    description:
      "One line under the Google login confirmation's list: who the list cannot show",
  },
  "workspaces.loginNoticeMembers_one": {
    message: "{count} участник",
    description:
      "The bold count in the notice under an enforced login requirement: how many members do not meet it (w1/m647, Render's wording)",
  },
  "workspaces.loginNoticeMembers_few": {
    message: "{count} участника",
    description:
      "The bold count in the notice under an enforced login requirement: how many members do not meet it (w1/m647, Render's wording)",
  },
  "workspaces.loginNoticeMembers_many": {
    message: "{count} участников",
    description:
      "The bold count in the notice under an enforced login requirement: how many members do not meet it (w1/m647, Render's wording)",
  },
  "workspaces.loginNoticeMembers_other": {
    message: "{count} участника",
    description:
      "The bold count in the notice under an enforced login requirement: how many members do not meet it (w1/m647, Render's wording)",
  },
  "workspaces.loginNoticeTwoFactor_one": {
    message:
      "{members} вашей команды ещё не включил двухфакторную аутентификацию.",
    description:
      "Notice under enforced 2FA when members have not enabled it; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeTwoFactor_few": {
    message:
      "{members} вашей команды ещё не включили двухфакторную аутентификацию.",
    description:
      "Notice under enforced 2FA when members have not enabled it; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeTwoFactor_many": {
    message:
      "{members} вашей команды ещё не включили двухфакторную аутентификацию.",
    description:
      "Notice under enforced 2FA when members have not enabled it; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeTwoFactor_other": {
    message:
      "{members} вашей команды ещё не включили двухфакторную аутентификацию.",
    description:
      "Notice under enforced 2FA when members have not enabled it; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeGoogleLogin_one": {
    message: "{members} вашей команды ещё не вошёл через Google.",
    description:
      "Notice under enforced Google login when members have not signed in with Google; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeGoogleLogin_few": {
    message: "{members} вашей команды ещё не вошли через Google.",
    description:
      "Notice under enforced Google login when members have not signed in with Google; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeGoogleLogin_many": {
    message: "{members} вашей команды ещё не вошли через Google.",
    description:
      "Notice under enforced Google login when members have not signed in with Google; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeGoogleLogin_other": {
    message: "{members} вашей команды ещё не вошли через Google.",
    description:
      "Notice under enforced Google login when members have not signed in with Google; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeViewMembers_one": {
    message: "Показать участника",
    description:
      "Button on the notice that opens the list of members who do not meet the requirement (Render's wording)",
  },
  "workspaces.loginNoticeViewMembers_few": {
    message: "Показать участников",
    description:
      "Button on the notice that opens the list of members who do not meet the requirement (Render's wording)",
  },
  "workspaces.loginNoticeViewMembers_many": {
    message: "Показать участников",
    description:
      "Button on the notice that opens the list of members who do not meet the requirement (Render's wording)",
  },
  "workspaces.loginNoticeViewMembers_other": {
    message: "Показать участников",
    description:
      "Button on the notice that opens the list of members who do not meet the requirement (Render's wording)",
  },
  "workspaces.loginNoticeAllMembers": {
    message: "Все участники",
    description:
      "The bold words of the all-compliant notice (Render's wording)",
  },
  "workspaces.loginNoticeAllTwoFactor": {
    message: "{members} вашей команды включили двухфакторную аутентификацию!",
    description:
      "Notice under enforced 2FA when every member has enabled it; {members} is the bold 'All members' (Render's wording)",
  },
  "workspaces.loginNoticeAllGoogleLogin": {
    message: "{members} вашей команды вошли через Google!",
    description:
      "Notice under enforced Google login when every member has signed in with Google; {members} is the bold 'All members' (Render's wording)",
  },
  "workspaces.loginMembersTitleTwoFactor": {
    message: "Участники без двухфакторной аутентификации",
    description:
      "Title of the dialog listing members without 2FA (Render's wording)",
  },
  "workspaces.loginMembersTitleGoogleLogin": {
    message: "Участники без входа через Google",
    description:
      "Title of the dialog listing members who have not signed in with Google (Render's wording)",
  },
  "workspaces.loginMembersDescription_one": {
    message:
      "Этому участнику сейчас запрещён доступ к этому рабочему пространству команды и его ресурсам.",
    description:
      "Description of the dialog listing members who do not meet the enforced requirement (Render's wording)",
  },
  "workspaces.loginMembersDescription_few": {
    message:
      "Этим участникам сейчас запрещён доступ к этому рабочему пространству команды и его ресурсам.",
    description:
      "Description of the dialog listing members who do not meet the enforced requirement (Render's wording)",
  },
  "workspaces.loginMembersDescription_many": {
    message:
      "Этим участникам сейчас запрещён доступ к этому рабочему пространству команды и его ресурсам.",
    description:
      "Description of the dialog listing members who do not meet the enforced requirement (Render's wording)",
  },
  "workspaces.loginMembersDescription_other": {
    message:
      "Этим участникам сейчас запрещён доступ к этому рабочему пространству команды и его ресурсам.",
    description:
      "Description of the dialog listing members who do not meet the enforced requirement (Render's wording)",
  },
  "workspaces.twoFactorRemindTitle_one": {
    message: "Напомнить этому пользователю включить 2FA?",
    description:
      "Title of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindTitle_few": {
    message: "Напомнить этим пользователям включить 2FA?",
    description:
      "Title of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindTitle_many": {
    message: "Напомнить этим пользователям включить 2FA?",
    description:
      "Title of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindTitle_other": {
    message: "Напомнить этим пользователям включить 2FA?",
    description:
      "Title of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindBody_one": {
    message: "Отправить этому пользователю напоминание по электронной почте.",
    description:
      "Body of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindBody_few": {
    message: "Отправить этим пользователям напоминания по электронной почте.",
    description:
      "Body of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindBody_many": {
    message: "Отправить этим пользователям напоминания по электронной почте.",
    description:
      "Body of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindBody_other": {
    message: "Отправить этим пользователям напоминания по электронной почте.",
    description:
      "Body of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindAction_one": {
    message: "Отправить напоминание",
    description: "Button that emails the 2FA reminders (Render's wording)",
  },
  "workspaces.twoFactorRemindAction_few": {
    message: "Отправить напоминания",
    description: "Button that emails the 2FA reminders (Render's wording)",
  },
  "workspaces.twoFactorRemindAction_many": {
    message: "Отправить напоминания",
    description: "Button that emails the 2FA reminders (Render's wording)",
  },
  "workspaces.twoFactorRemindAction_other": {
    message: "Отправить напоминания",
    description: "Button that emails the 2FA reminders (Render's wording)",
  },
  "workspaces.twoFactorRemindError": {
    message:
      "Извините, нам не удалось отправить напоминания этим участникам команды. Повторить попытку?",
    description:
      "Shown in the members dialog when the reminders could not be sent (Render's wording)",
  },
  "workspaces.twoFactorRemindSentToast": {
    message:
      "Напоминания отправлены по электронной почте участникам без двухфакторной аутентификации!",
    description: "Toast after the 2FA reminders are sent (Render's wording)",
  },
  "workspaces.twoFactorRemindLimitTitle": {
    message: "Достигнут лимит напоминаний!",
    description:
      "Title of the dialog shown when reminders were already sent today (Render's wording)",
  },
  "workspaces.twoFactorRemindLimitBody": {
    message:
      "Похоже, сегодня мы уже отправили напоминания. Вы сможете отправить их снова завтра.",
    description:
      "Body of the dialog shown when reminders were already sent today; the day is UTC (Render's wording)",
  },
  "workspaces.twoFactorConfirmReminder_one": {
    message: "Этому участнику будет отправлено напоминание включить 2FA.",
    description:
      "Line in the require-2FA confirmation: turning it on emails a reminder to each member listed (Render's wording)",
  },
  "workspaces.twoFactorConfirmReminder_few": {
    message: "Этим участникам будут отправлены напоминания включить 2FA.",
    description:
      "Line in the require-2FA confirmation: turning it on emails a reminder to each member listed (Render's wording)",
  },
  "workspaces.twoFactorConfirmReminder_many": {
    message: "Этим участникам будут отправлены напоминания включить 2FA.",
    description:
      "Line in the require-2FA confirmation: turning it on emails a reminder to each member listed (Render's wording)",
  },
  "workspaces.twoFactorConfirmReminder_other": {
    message: "Этим участникам будут отправлены напоминания включить 2FA.",
    description:
      "Line in the require-2FA confirmation: turning it on emails a reminder to each member listed (Render's wording)",
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
  "workspaces.orgDowngradePersonRole": {
    message: "{person} ({role})",
    description:
      "A member or invitee in a downgrade precondition: {person} is their id or email, {role} the role to change (w7/m464)",
  },
  "workspaces.orgDowngradeIpRuleWorkspace": {
    message: "это рабочее пространство",
    description:
      "The workspace's own inbound IP rules, in the inbound_ip_rules precondition (w7/m464)",
  },
  "workspaces.orgDowngradeIpRuleEnvironment": {
    message: "окружение {name}",
    description:
      "An environment with inbound IP rules, in the inbound_ip_rules precondition (w7/m464)",
  },
  "workspaces.orgDowngradeIpRuleService": {
    message: "сервис {name}",
    description:
      "A service with inbound IP rules, in the inbound_ip_rules precondition (w7/m464)",
  },
  "workspaces.orgPlanOnPrimaryError": {
    message:
      "Тариф этого рабочего пространства — тариф его организации. Измените его в основном рабочем пространстве организации.",
    description:
      "Refusal for code ORGANIZATION_PLAN_ON_PRIMARY: a secondary workspace's plan changes only on the org's primary (w7/m464)",
  },
  "workspaces.orgPlanSecondaryNote": {
    message:
      "Входит в тариф вашей организации. Тарифом и оплатой управляет основное рабочее пространство организации.",
    description:
      "Workspace Settings and Billing on an org's secondary workspace: its plan is the org's, managed on the primary (ADR115, w7/m464)",
  },
  "workspaces.orgPlanPrimarySettingsLink": {
    message: "Перейти в основное рабочее пространство",
    description:
      "Link from a secondary workspace's settings to the org's primary workspace settings (w7/m464)",
  },
  "workspaces.orgPlanPrimaryBillingLink": {
    message: "Перейти к оплате основного рабочего пространства",
    description:
      "Button on a secondary workspace's Billing plan card linking to the org's primary workspace's Billing (w7/m464)",
  },
  "workspaces.privacyTitle": {
    message: "Доступ к рабочему пространству",
    description:
      "Workspace Settings → Workspace privacy card title, radio group name and section link (w7/m464)",
  },
  "workspaces.privacyDescription": {
    message:
      "Выберите, кто из вашей организации может присоединиться к этому рабочему пространству.",
    description: "Description under the Workspace privacy card title (w7/m464)",
  },
  "workspaces.privacyPublic": {
    message: "Открытое",
    description: "The Public privacy setting's label, Render's name (w7/m464)",
  },
  "workspaces.privacyPublicDescription": {
    message:
      "Любой участник вашей организации может сам добавить себя в это рабочее пространство.",
    description:
      "What the Public privacy setting allows, Render's wording (render.com/docs/organizations, w7/m464)",
  },
  "workspaces.privacyInviteOnly": {
    message: "Только по приглашению",
    description:
      "The Invite-only privacy setting's label, Render's name (w7/m464)",
  },
  "workspaces.privacyInviteOnlyDescription": {
    message:
      "Приглашать людей в это рабочее пространство могут только его администраторы.",
    description:
      "What the Invite-only privacy setting allows, Render's wording (render.com/docs/organizations, w7/m464)",
  },
  "workspaces.privacySave": {
    message: "Сохранить",
    description: "Saves the selected workspace privacy (w7/m464)",
  },
  "workspaces.privacySaving": {
    message: "Сохранение…",
    description: "The Save button while the workspace privacy saves (w7/m464)",
  },
  "workspaces.privacySaved": {
    message: "Настройка доступа к рабочему пространству сохранена",
    description: "Toast after setWorkspacePrivacy succeeds (w7/m464)",
  },
  "workspaces.privacyError": {
    message:
      "Не удалось сохранить настройку доступа к рабочему пространству. Повторите попытку.",
    description:
      "Fallback toast when a privacy save fails without a server reason (w7/m464)",
  },
  "workspaces.loginRequirementRequiredByOrganization": {
    message:
      "Этого требует {organization} во всех своих рабочих пространствах.",
    description:
      "Workspace Settings, under the 2FA or Google login switch: why it is on and locked, the workspace's organization requires it in all of its workspaces (w7/m466)",
  },
  "workspaces.loginRequirementOrganizationSettingsLink": {
    message: "Изменить в настройках организации",
    description:
      "Link for an org Owner after the required-by-organization note, to the organization's Settings page (w7/m466)",
  },
  "workspaces.twoFactorBlockedOrganizationTitle": {
    message:
      "{organization} требует двухфакторную аутентификацию во всех своих рабочих пространствах",
    description:
      "Heading of the 2FA blocked page when the organization's setting refuses the member, on a workspace or on the org's own pages (w7/m466)",
  },
  "workspaces.twoFactorBlockedOrganizationBody": {
    message:
      "Владелец этой организации требует, чтобы все участники использовали двухфакторную аутентификацию (2FA) в организации и во всех её рабочих пространствах. Пока вы её не включите, вы не сможете получить к ним доступ, а ваши API-ключи и SSH-ключи нельзя будет с ними использовать.",
    description:
      "Why the 2FA blocked page is shown when the organization's setting refuses the member (w7/m466)",
  },
  "workspaces.googleLoginBlockedOrganizationTitle": {
    message:
      "{organization} требует вход через Google во всех своих рабочих пространствах",
    description:
      "Heading of the Google login blocked page when the organization's setting refuses the member (w7/m466)",
  },
  "workspaces.googleLoginBlockedOrganizationBody": {
    message:
      "Владелец этой организации требует, чтобы все участники входили через Google в организации и во всех её рабочих пространствах. Пока вы не войдёте через аккаунт Google своего адреса электронной почты в bex, вы не сможете получить к ним доступ, а API-ключи и SSH-ключи, созданные вами при входе другим способом, нельзя будет с ними использовать.",
    description:
      "Why the Google login blocked page is shown when the organization's setting refuses the member (w7/m466)",
  },
  "workspaces.updatePlanPageTitle": {
    message: "Изменить тариф",
    description: "Document title of the plan pages (w3/m708)",
  },
  "workspaces.updatePlanTitle": {
    message: "Изменить тариф для {name}",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): page title with the workspace's name",
  },
  "workspaces.updatePlanConfirmTitle": {
    message: "Изменить тариф для {name}",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): page title with the workspace's name (Render capitalizes Plan here)",
  },
  "workspaces.updatePlanBack": {
    message: "Назад",
    description:
      "Accessible label of the plan pages' Back button, and the confirmation's Back action (w3/m708)",
  },
  "workspaces.updatePlanSelectTitle": {
    message: "Выберите тариф",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): heading of the plan cards",
  },
  "workspaces.updatePlanCompare": {
    message: "Подробнее о тарифах и их сравнение — на нашей {link}.",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): line under Select a plan; {link} is the updatePlanPricingLink link to bex's pricing page",
  },
  "workspaces.updatePlanPricingLink": {
    message: "странице цен",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): the link in updatePlanCompare",
  },
  "workspaces.updatePlanPaymentTitle": {
    message: "Способ оплаты",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): heading of the payment method section",
  },
  "workspaces.updatePlanCardRequired": {
    message:
      "Для этого тарифа рабочего пространства нужна карта для платных сервисов и вычислений.",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): notice while a plan other than Hobby is selected",
  },
  "workspaces.updatePlanHowBillingWorks": {
    message: "Как устроена оплата",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): link beside updatePlanCardRequired to bex's billing docs",
  },
  "workspaces.updatePlanWorkspaceBillingTitle": {
    message: "Оплата рабочего пространства",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): heading beside the card on file",
  },
  "workspaces.updatePlanWorkspaceBillingBody": {
    message:
      "Оплата ведётся отдельно для каждого рабочего пространства. С вашей карты будут списываться платежи за платные сервисы и вычисления в этом рабочем пространстве.",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): line under updatePlanWorkspaceBillingTitle",
  },
  "workspaces.updatePlanErrorsSummary": {
    message:
      "В выбранных параметрах есть ошибки. Проверьте и исправьте их, чтобы продолжить.",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): summary under the plan rules a selection breaks",
  },
  "workspaces.updatePlanErrorPrefix": {
    message: "Произошла ошибка. {message}",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): a refused change; {message} is the reason",
  },
  "workspaces.updatePlanPaymentRequired": {
    message: "Добавьте действующую карту, чтобы перейти на платный тариф.",
    description:
      "The plan pages' message when bex-api refuses a paid plan with PAYMENT_REQUIRED (w3/m708)",
  },
  "workspaces.updatePlanServerError": {
    message: "Ошибка сервера при отправке. Попробуйте ещё раз",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): fallback reason when an upgrade fails without one",
  },
  "workspaces.updatePlanSubmit": {
    message: "Выбрать тариф",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): submit button",
  },
  "workspaces.updatePlanCancel": {
    message: "Отмена",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): cancel button, back to where the page was opened from",
  },
  "workspaces.updatePlanNoPermission": {
    message:
      "Ваша роль не позволяет менять тариф этого рабочего пространства. Обратитесь к администратору.",
    description:
      "The plan pages' notice for a member who cannot change the plan (can_manage) (w3/m708)",
  },
  "workspaces.planCurrentBadge": {
    message: "Текущий тариф",
    description:
      "Badge on the workspace's own plan card on the plan page (Render's TeamPlanPicker, w3/m708)",
  },
  "workspaces.planNewBadge": {
    message: "Новый тариф",
    description:
      "Badge on the chosen plan card on the plan page (Render's TeamPlanPicker, w3/m708)",
  },
  "workspaces.planContact": {
    message: "Связаться с нами",
    description:
      "Enterprise card's contact action on the plan page, Render's Get in touch (w3/m708)",
  },
  "workspaces.updatePlanSummaryTitle": {
    message: "Сводка нового тарифа",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): heading of the plan comparison and the card on file",
  },
  "workspaces.updatePlanConfirmDowngradeTitle": {
    message: "Подтвердите понижение тарифа",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): heading of the typed confirmation a downgrade asks for",
  },
  "workspaces.updatePlanConfirmChangesIncluding": {
    message: "Чтобы начать изменение тарифа, включая:",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): lead-in above the list of what a downgrade changes",
  },
  "workspaces.updatePlanConfirmTypeAfterList": {
    message:
      "Введите {phrase} в текстовое поле и нажмите кнопку подтверждения.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): the typed phrase after the list of changes; {phrase} is 'sudo downgrade workspace <name>' (rendered bold)",
  },
  "workspaces.updatePlanConfirmType": {
    message:
      "Чтобы начать изменение тарифа, введите {phrase} в текстовое поле и нажмите кнопку подтверждения.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): the typed phrase when the downgrade lists no changes; {phrase} is 'sudo downgrade workspace <name>' (rendered bold)",
  },
  "workspaces.updatePlanConfirmSubmit": {
    message: "Подтвердить",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): confirm button",
  },
  "workspaces.updatePlanErrorTitle": {
    message: "Произошла ошибка.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): title of a refused change",
  },
  "workspaces.updatePlanError": {
    message: "Ошибка при обновлении тарифа.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): fallback reason when the change fails without one",
  },
  "workspaces.updatePlanRemovesOrganization": {
    message: "Организация будет удалена.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): bold start of the change an org primary's downgrade below Scale makes (ADR115 D6)",
  },
  "workspaces.updatePlanRemovesOrganizationDetail": {
    message:
      "Функции уровня организации отключатся, а это рабочее пространство продолжит работать как самостоятельное.",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): rest of the org removal line after updatePlanRemovesOrganization",
  },
  "workspaces.planUpdated": {
    message: "Теперь ваше рабочее пространство на тарифе {plan}.",
    description:
      "Toast after a plan change, Render's plan-updated toast; {plan} is the plan's name (w3/m708)",
  },
  "workspaces.planUpdatedWelcome": {
    message: "Добро пожаловать на тариф {plan}.",
    description:
      "Toast after a change to Scale, Render's welcome; {plan} is the plan's name (w3/m708)",
  },
  "workspaces.planComparisonCurrent": {
    message: "Текущий тариф",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): badge on the current plan's side of the comparison",
  },
  "workspaces.planComparisonNew": {
    message: "Новый тариф",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): badge on the new plan's side of the comparison",
  },
  "workspaces.planComparisonHighlights": {
    message: "Основное",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): label above each side's highlights (shown in capitals)",
  },
  "workspaces.planHighlightJustForMe": {
    message: "Только для меня",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): Hobby's one seat",
  },
  "workspaces.planHighlightUnlimitedMembers": {
    message: "Неограниченное число участников",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): unlimited seats",
  },
  "workspaces.planHighlightIncludedDomains": {
    message:
      "{included} пользовательских доменов включено, далее {price} за домен",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): custom domains included before they are metered; {included} is the count, {price} bex's price per domain (ADR030 §16)",
  },
  "workspaces.planHighlightUnlimitedDomains": {
    message: "Неограниченное число пользовательских доменов",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): every custom domain included (Enterprise)",
  },
  "workspaces.planHighlightTwoEnvironments": {
    message: "2 окружения на проект",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): Hobby's environments per project",
  },
  "workspaces.planHighlightUnlimitedEnvironments": {
    message: "Неограниченное число проектов и окружений",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): unlimited projects and environments",
  },
  "workspaces.planHighlightRetention": {
    message: "Логи и метрики за {days} дн.",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): how far back logs and metrics reach; {days} is the number of days",
  },
  "workspaces.planHighlightBasicLogs": {
    message: "Базовые логи и метрики",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): Hobby's logs and metrics, without request logs",
  },
  "workspaces.planHighlightRequestLogs": {
    message: "Логи HTTP-запросов и метрики времени ответа",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): Pro's HTTP request logs and response time metrics",
  },
  "workspaces.planHighlightKeptBuilds": {
    message: "Хранится сборок для мгновенного отката: {builds}",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): builds kept for rollback; {builds} is the count (core.KeptBuilds)",
  },
  "workspaces.planHighlightAuditLogs": {
    message: "Журнал аудита рабочего пространства",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): workspace audit logs (Pro and higher)",
  },
  "workspaces.planHighlightOrganization": {
    message:
      "Организация: дополнительные рабочие пространства, роли и правила входящих IP",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): what an organization adds (Scale and higher, ADR115)",
  },
  "workspaces.planBlockerMemberLimitTitle": {
    message: "Слишком много пользователей в команде.",
    description:
      "Render's plan rule on its plan page (w3/m708): title when the workspace has more people than the plan seats",
  },
  "workspaces.planBlockerMemberLimitOne": {
    message: "В рабочем пространстве {plan} может быть только 1 пользователь.",
    description:
      "Render's plan rule on its plan page (w3/m708): a plan with one seat; {plan} is the plan's name",
  },
  "workspaces.planBlockerMemberLimitMany": {
    message: "В рабочем пространстве {plan} может быть не более {limit} польз.",
    description:
      "Render's plan rule on its plan page (w3/m708): a plan with several seats; {plan} is the plan's name, {limit} its seats",
  },
  "workspaces.planBlockerUpdateTeamMembers": {
    message: "Изменить участников",
    description:
      "Render's plan rule on its plan page (w3/m708): link to Workspace Settings' Team section",
  },
  "workspaces.planBlockerServiceLimitTitle": {
    message: "Слишком много сервисов в рабочем пространстве.",
    description:
      "Render's plan rule on its plan page (w3/m708): title when the workspace has more services than the plan allows",
  },
  "workspaces.planBlockerServiceLimit": {
    message:
      "В рабочем пространстве {plan} может быть не более {limit} сервисов.",
    description:
      "Render's plan rule on its plan page (w3/m708): the plan's service cap; {plan} is the plan's name, {limit} its cap",
  },
  "workspaces.planBlockerUpdateServices": {
    message: "Изменить сервисы",
    description:
      "Render's plan rule on its plan page (w3/m708): link to the workspace's services",
  },
  "workspaces.planBlockerWorkspaceLimitTitle": {
    message: "Слишком много рабочих пространств Hobby.",
    description:
      "Render's plan rule on its plan page (w3/m708): title when the caller already has the most Hobby workspaces allowed",
  },
  "workspaces.planBlockerWorkspaceLimit": {
    message:
      "Нельзя иметь больше {limit} рабочих пространств Hobby. Перейдите на другой тариф или удалите другое пространство Hobby.",
    description:
      "Render's plan rule on its plan page (w3/m708): the cap on a person's Hobby workspaces; {limit} is the cap",
  },
  "workspaces.planBlockerWorkspaceLimitsDocs": {
    message: "Лимиты рабочих пространств",
    description:
      "Render's plan rule on its plan page (w3/m708): link to bex's docs on workspace limits",
  },
  "workspaces.planBlockerWorkspacesTitle": {
    message: "Слишком много рабочих пространств.",
    description:
      "Render's plan rule on its plan page (w3/m708): title when an org primary's downgrade leaves secondary workspaces",
  },
  "workspaces.planBlockerWorkspaces": {
    message: "{plan} не поддерживает больше одного рабочего пространства.",
    description:
      "Render's plan rule on its plan page (w3/m708): a plan without an organization; {plan} is the plan's name",
  },
  "workspaces.planBlockerUpdateWorkspaces": {
    message: "Изменить рабочие пространства",
    description:
      "Render's plan rule on its plan page (w3/m708): link to the organization's workspaces",
  },
  "workspaces.planBlockerRolesTitle": {
    message: "Нельзя понизить тариф, пока используются недоступные роли",
    description:
      "Render's plan rule on its plan page (w3/m708): title when members or invites hold roles the plan does not offer",
  },
  "workspaces.planBlockerRolesOne": {
    message:
      "{plan} не поддерживает роль {roles}. Чтобы понизить тариф, сначала смените роль затронутых участников и ожидающих приглашений на Developer или Admin.",
    description:
      "Render's plan rule on its plan page (w3/m708): one role the plan lacks; {plan} is the plan's name, {roles} the role",
  },
  "workspaces.planBlockerRolesMany": {
    message:
      "{plan} не поддерживает роли {roles}. Чтобы понизить тариф, сначала смените роль затронутых участников и ожидающих приглашений на Developer или Admin.",
    description:
      "Render's plan rule on its plan page (w3/m708): several roles the plan lacks; {plan} is the plan's name, {roles} the roles",
  },
  "workspaces.planBlockerIpRulesTitle": {
    message: "Перед понижением тарифа нужно удалить списки разрешённых IP.",
    description:
      "Render's plan rule on its plan page (w3/m708): title when inbound IP rules stand in the way (Render's words for them on its confirmation)",
  },
  "workspaces.planBlockerIpRules": {
    message:
      "Перед сменой тарифа удалите все настройки списков разрешённых IP из рабочего пространства, окружений и сервисов.",
    description:
      "Render's plan rule on its plan page (w3/m708): what to remove for planBlockerIpRulesTitle",
  },
  "workspaces.planBlockerUpdateIpRules": {
    message: "Изменить правила входящих IP",
    description:
      "Render's plan rule on its plan page (w3/m708): link to Workspace Settings' inbound IP rules",
  },
  "workspaces.planBlockerOtherTitle": {
    message: "Этот тариф пока нельзя выбрать.",
    description:
      "Render's plan rule on its plan page (w3/m708): title of a refusal kind this dashboard does not know yet",
  },
  "workspaces.planBlockerOther": {
    message:
      "Это рабочее пространство пока не укладывается в лимиты тарифа {plan}.",
    description:
      "Render's plan rule on its plan page (w3/m708): line under planBlockerOtherTitle; {plan} is the plan's name",
  },
};

export default ruWorkspaces;
