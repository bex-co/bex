import type { TranslationEntry } from "@/i18n";

const ruUsage: Record<string, TranslationEntry> = {
  "usage.pageTitle": {
    message: "Оплата",
    description:
      "Billing page heading and browser title (renamed from Usage in w5/m70)",
  },
  "usage.pageSubtitle": {
    message:
      "Платежи, счета и потребление рабочего пространства с начала месяца",
    description: "Billing page subtitle beneath the heading",
  },
  "usage.sectionNavigation": {
    message: "Разделы оплаты",
    description: "Accessible label for the billing page section navigation",
  },
  "usage.resourceCapsServices": {
    message: "Сервисы",
    description: "Service count label in the resource-cap card",
  },
  "usage.resourceCapsPostgres": {
    message: "Postgres",
    description: "Postgres count label in the resource-cap card",
  },
  "usage.resourceCapsKeyValues": {
    message: "Key Value",
    description: "Key Value count label in the resource-cap card",
  },
  "usage.resourceCapsFree": {
    message: "Бесплатных: {used} из {limit}",
    description: "Free-instance count in the resource-cap card (w4/m241)",
  },
  "usage.freeHoursTitle": {
    message: "Включённое ежемесячное использование",
    description:
      "Title of the Free instance hours card on the usage page (w8/m139)",
  },
  "usage.freeHoursDescription": {
    message:
      "Бесплатные Web Service делят ежемесячный лимит часов работы инстансов на рабочее пространство. Время в остановленном состоянии не учитывается.",
    description: "Explains the monthly Free instance hours allowance (w8/m139)",
  },
  "usage.freeHoursUsed": {
    message:
      "Использовано {used} из {included} бесплатных часов работы инстансов в этом месяце",
    description:
      "Free instance hours used against the monthly allowance (w8/m139)",
  },
  "usage.freeHoursNear": {
    message:
      "Вы близки к лимиту. Когда он будет исчерпан, ваши бесплатные Web Service будут приостановлены до начала следующего месяца.",
    description:
      "Warning once Free instance hours reach 80 percent of the allowance (w8/m139)",
  },
  "usage.freeHoursSpent": {
    message:
      "Лимит исчерпан. Бесплатные Web Service приостановлены до начала следующего месяца; переведите сервис на платный тариф, чтобы запустить его сейчас.",
    description:
      "Notice once the monthly Free instance hours are spent (w8/m139)",
  },
  "usage.buildMinutesTitle": {
    message: "Минуты пайплайна",
    description:
      "Title of the included build minutes card on the billing page (w6/m308)",
  },
  "usage.buildMinutesDescription": {
    message:
      "Сборки и команды перед деплоем расходуют минуты пайплайна. Ваш тариф включает некоторое их количество каждый месяц; минуты сверх него оплачиваются поминутно.",
    description: "Explains the plan's included build minutes (w6/m308)",
  },
  "usage.buildMinutesUsed": {
    message:
      "Использовано {used} из {included} включённых минут сборки в этом месяце",
    description:
      "Build minutes used against the plan's included minutes (w6/m308)",
  },
  "usage.buildMinutesNear": {
    message:
      "Вы близки к лимиту включённых минут сборки. Без способа оплаты новые сборки и команды перед деплоем остановятся после их исчерпания до начала следующего месяца.",
    description:
      "Warning once build minutes reach 80 percent of the included minutes (w6/m308)",
  },
  "usage.buildMinutesSpent": {
    message:
      "Включённые минуты сборки исчерпаны. Дополнительные минуты оплачиваются поминутно; без способа оплаты новые сборки и команды перед деплоем остановятся до начала следующего месяца.",
    description: "Notice once the included build minutes are spent (w6/m308)",
  },
  "usage.bandwidthTitle": {
    message: "Исходящая пропускная способность",
    description:
      "Title of the included bandwidth card on the billing page (w6/m319)",
  },
  "usage.bandwidthDescription": {
    message:
      "Трафик, который ваши сервисы отправляют в интернет, расходует пропускную способность. Ваш тариф включает некоторый объём каждый месяц; трафик сверх него оплачивается за ГБ.",
    description: "Explains the plan's included outbound bandwidth (w6/m319)",
  },
  "usage.bandwidthUsed": {
    message:
      "Использовано {used} из {included} ГБ включённого трафика в этом месяце",
    description:
      "Outbound bandwidth used against the plan's included bandwidth, in GB (w6/m319)",
  },
  "usage.bandwidthNear": {
    message:
      "Вы близки к лимиту включённого трафика. Без способа оплаты сервисы вашего рабочего пространства будут остановлены после его исчерпания до начала следующего месяца.",
    description:
      "Warning once bandwidth reaches 80 percent of the included bandwidth (w6/m319)",
  },
  "usage.bandwidthSpent": {
    message:
      "Включённый трафик исчерпан. Дополнительный трафик оплачивается за ГБ; без способа оплаты сервисы вашего рабочего пространства остановлены до начала следующего месяца.",
    description: "Notice once the included bandwidth is spent (w6/m319)",
  },
  "usage.customDomainsTitle": {
    message: "Пользовательские домены",
    description:
      "Title of the included custom domains card on the billing page (w7/m429)",
  },
  "usage.customDomainsDescription": {
    message:
      "Ваш тариф включает определённое число подтверждённых пользовательских доменов. Каждый домен сверх них оплачивается помесячно за время, пока он подтверждён. Добавление домена никогда не отклоняется из-за их числа.",
    description:
      "Explains the plan's included custom domains and how domains beyond them are billed (w7/m429)",
  },
  "usage.customDomainsUsed": {
    message:
      "Используется {used} из {included} включённых пользовательских доменов",
    description:
      "Verified custom domains now against the count the plan includes (w7/m429)",
  },
  "usage.customDomainsNear": {
    message:
      "Вы приближаетесь к числу пользовательских доменов, включённых в тариф. Каждый домен сверх него оплачивается помесячно.",
    description:
      "Note once the verified custom domains reach 80 percent of the included count (w7/m429)",
  },
  "usage.customDomainsSpent": {
    message:
      "Все пользовательские домены, включённые в тариф, используются. Каждый следующий подтверждённый домен оплачивается помесячно.",
    description: "Notice once every included custom domain is in use (w7/m429)",
  },
  "usage.categoryCustomDomains": {
    message: "Пользовательские домены",
    description:
      "Charge-tree category for a workspace's custom domains beyond its plan (w7/m429)",
  },
  "usage.chargeCronMinimum": {
    message: "Минимальная плата за Cron Job",
    description:
      "Charge line under a cron job: what its runs in a month cost short of the cron job minimum, charged once the month ends (w9/m448)",
  },
  "usage.chargeCronMinimumRate": {
    message: "Минимум ${minimum}/мес.",
    description:
      "Rate column value for the cron job minimum line; minimum is the USD amount without the currency symbol (w9/m448)",
  },
  "usage.chargeCustomDomains": {
    message: "Пользовательские домены сверх тарифа",
    description:
      "Charge line for custom domains beyond the plan's included count, billed per domain-month (w7/m429)",
  },
  "usage.chargeIncluded": {
    message: "{included} включено",
    description:
      "Part of a charge line's quantity that the plan's included build minutes or bandwidth covered (w6/m308, w6/m319)",
  },
  "usage.pipelineTitle": {
    message: "Пайплайн сборки",
    description:
      "Title of the Build Pipeline card in workspace settings (w6/m309)",
  },
  "usage.pipelineDescription": {
    message:
      "Сборки и команды перед деплоем расходуют минуты пайплайна. Минуты сверх включённых в ваш тариф оплачиваются поминутно, до заданного вами месячного лимита расходов. По достижении лимита сборки останавливаются до следующего месяца.",
    description: "Explains the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitValue": {
    message: "Лимит расходов: {limit} в месяц",
    description: "The workspace pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitNone": {
    message: "Без лимита расходов",
    description: "The workspace has no pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitSet": {
    message: "Задать лимит расходов",
    description: "Button that opens the pipeline spend limit dialog (w6/m309)",
  },
  "usage.pipelineSpendLimitEdit": {
    message: "Изменить",
    description: "Button that edits the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitRemove": {
    message: "Убрать лимит",
    description: "Button that removes the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitDialogTitle": {
    message: "Задать лимит расходов",
    description: "Title of the pipeline spend limit dialog (w6/m309)",
  },
  "usage.pipelineSpendLimitDialogDescription": {
    message:
      "Максимум, который это рабочее пространство тратит в месяц на минуты сборки сверх включённых в тариф. При его достижении новые сборки и команды перед деплоем останавливаются до начала следующего месяца.",
    description: "Explains what the pipeline spend limit caps (w6/m309)",
  },
  "usage.pipelineSpendLimitLabel": {
    message: "Месячный лимит (USD)",
    description: "Label of the pipeline spend limit input (w6/m309)",
  },
  "usage.pipelineSpendLimitSave": {
    message: "Сохранить",
    description: "Saves the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitSaved": {
    message: "Лимит расходов установлен: {limit} в месяц",
    description: "Toast after setting the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitRemoved": {
    message: "Лимит расходов убран",
    description: "Toast after removing the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitError": {
    message: "Не удалось обновить лимит расходов",
    description: "Toast when updating the pipeline spend limit fails (w6/m309)",
  },
  "usage.pipelineSpendLimitUnreadable": {
    message:
      "Не удалось загрузить лимит расходов. Перезагрузите страницу и повторите попытку.",
    description: "The pipeline spend limit could not be read (w6/m309)",
  },
  "usage.pipelineSpendLimitForbidden": {
    message: "Видеть и менять лимит расходов могут только менеджеры по оплате.",
    description: "The caller may not read the pipeline spend limit (w6/m309)",
  },
  "usage.resourceCapsNearLimit": {
    message: "Близко к лимиту",
    description:
      "Warning shown once a resource count reaches 80 percent of its cap, below the cap itself",
  },
  "usage.resourceCapsLimitReached": {
    message: "Лимит достигнут",
    description:
      "Shown when a resource count is at or over its cap, so creating another is refused",
  },
  "usage.resourceCapsFinishingDeletion": {
    message: "Завершается удаление: {count}",
    description:
      "Sub-line on a resource-cap tile: how many of the counted resources are mid-deletion and thus hidden from the resource list but still holding quota (w6/m129)",
  },
  "usage.resourceCapsFinishingDeletionHint": {
    message: "Учитывается в лимите, пока удаление не завершится",
    description:
      "Tooltip explaining why resources finishing deletion are included in the used count",
  },
  "usage.billingDetailsDeniedTitle": {
    message: "Сведения об оплате недоступны для вашей роли",
    description:
      "Billing page heading for a Contributor or a Viewer, whose role may not view billing details (Render's View billing details row, w9/m620)",
  },
  "usage.errorTitle": {
    message: "Не удалось загрузить данные об использовании",
    description: "Error state heading on the Usage page",
  },
  "usage.monthPickerLabel": {
    message: "Выберите месяц",
    description: "Aria-label for the month-picker select on the Usage page",
  },
  "usage.currentMonth": {
    message: "Текущий месяц",
    description:
      "Default option in the month picker meaning the current calendar month",
  },
  "usage.creditsTitle": {
    message: "Остаток кредитов",
    description: "Heading of the remaining billing-credit card (w5/m70)",
  },
  "usage.creditsDescription": {
    message:
      "Промокредиты применяются к счетам до списания средств с вашего способа оплаты",
    description: "Subtitle of the remaining billing-credit card",
  },
  "usage.creditsExpiryNote": {
    message: "{amount} из них истекает {date}",
    description:
      "Earliest-expiring grant note beside the credit balance; amount is a $ value, date is YYYY-MM-DD",
  },
  "usage.creditsCardStillRequired": {
    message:
      "Способ оплаты нужен, даже если есть кредиты: сначала счёт покрывают кредиты, а остаток списывается с вашей карты.",
    description:
      "ADR046 clarification on the credit card — credit does not replace payment onboarding",
  },
  "usage.colInvoicePeriod": {
    message: "Период",
    description: "Table column header for an invoice's billing period start",
  },
  "usage.colInvoiceStatus": {
    message: "Статус",
    description: "Table column header for an invoice's status",
  },
  "usage.colInvoiceAmount": {
    message: "Сумма",
    description: "Table column header for an invoice's amount",
  },
  "usage.colInvoiceActions": {
    message: "Действия",
    description: "Screen-reader header of the invoice table's row-menu column",
  },
  "usage.billingSetupTitle": {
    message: "Настройка оплаты",
    description: "Customer billing onboarding card title",
  },
  "usage.billingSetupDescription": {
    message:
      "Приём платежей и готовность налогов для этого рабочего пространства",
    description: "Customer billing onboarding card description",
  },
  "usage.billingTestMode": {
    message: "Тестовый режим Stripe",
    description: "Badge making the non-live Stripe environment explicit",
  },
  "usage.billingMode": {
    message: "Stripe Billing",
    description: "Fallback badge for the Stripe billing environment",
  },
  "usage.billingReady": {
    message: "Готово",
    description: "Billing onboarding status is ready",
  },
  "usage.billingActionNeeded": {
    message: "Требуется действие",
    description: "Billing onboarding status needs action",
  },
  "usage.billingCustomerStatus": {
    message: "Клиент",
    description: "Stripe Customer readiness row",
  },
  "usage.billingSubscriptionStatus": {
    message: "Подписка с учётом потребления",
    description: "Stripe Subscription readiness row",
  },
  "usage.billingPaymentStatus": {
    message: "Способ оплаты",
    description: "Default payment method readiness row",
  },
  "usage.billingTaxStatus": {
    message: "Автоматический расчёт налогов",
    description: "Stripe Tax activation row",
  },
  "usage.billingLifecycleStatus": {
    message: "Жизненный цикл списаний",
    description: "Stripe collection and reversible enforcement state row",
  },
  "usage.billingLifecycleGrace": {
    message:
      "Платёж не прошёл ({reason}). В льготный период рабочее пространство остаётся доступным; обратимая приостановка запланирована после {deadline}.",
    description: "Visible billing grace state and authoritative deadline",
  },
  "usage.billingLifecycleEnforced": {
    message:
      "Действует ограничение из-за оплаты ({reason}). Вычисления приостановлены, но базы данных, данные Key Value, секреты и платёжные записи не удалены.",
    description: "Visible reversible billing enforcement state",
  },
  "usage.billingLifecycleRecovering": {
    message:
      "Платёж восстановлен. bex возвращает только те ресурсы, которые были изменены из-за ограничения по оплате.",
    description: "Visible precise recovery state",
  },
  "usage.billingLifecycleExcluded": {
    message: "Оператор исключил это рабочее пространство из списаний Stripe.",
    description: "Visible structural billing exclusion state",
  },
  "usage.billingLifecycleComped": {
    message:
      "Для этого рабочего пространства ведётся учёт, но оператор полностью освободил его от оплаты.",
    description: "Visible rated-but-free comp state",
  },
  "usage.billingLifecycleUnknown": {
    message:
      "Состояние оплаты: {reason}. Воспользуйтесь порталом оплаты или обратитесь в поддержку.",
    description: "Forward-compatible unknown billing lifecycle state",
  },
  "usage.billingNoDeadline": {
    message: "срок не указан",
    description: "Fallback when a malformed grace state has no deadline",
  },
  "usage.billingOff": {
    message: "Выкл.",
    description:
      "Neutral status for a deliberately disabled billing capability (e.g. tax not activated)",
  },
  "usage.billingAddPayment": {
    message: "Добавить способ оплаты",
    description: "Button opening setup-mode Stripe Checkout (live mode)",
  },
  "usage.billingUpdatePayment": {
    message: "Обновить способ оплаты",
    description: "Button reopening setup-mode Stripe Checkout (live mode)",
  },
  "usage.billingAddPaymentTest": {
    message: "Добавить тестовый способ оплаты",
    description:
      "Button opening setup-mode Stripe Checkout when Stripe is in test mode",
  },
  "usage.billingUpdatePaymentTest": {
    message: "Обновить тестовый способ оплаты",
    description:
      "Button reopening setup-mode Stripe Checkout when Stripe is in test mode",
  },
  "usage.billingOpenPortal": {
    message: "Открыть портал оплаты",
    description: "Button opening Stripe Customer Portal",
  },
  "usage.billingHostedNote": {
    message:
      "Платёжные данные вводятся только на страницах Stripe. bex никогда не передаёт серверный ключ Stripe в ваш браузер.",
    description: "Hosted billing security note",
  },
  "usage.billingUnavailable": {
    message: "Сведения об оплате сейчас недоступны. Повторите попытку позже.",
    description:
      "Billing readiness could not be read (a Stripe outage, the integration off). Every role that reaches the card may view billing details since w9/m620, so this is never a role refusal",
  },
  "usage.billingCheckoutError": {
    message: "Не удалось открыть Stripe Checkout. Повторите попытку.",
    description: "Toast after Checkout session creation fails",
  },
  "usage.billingPortalError": {
    message: "Не удалось открыть портал оплаты Stripe. Повторите попытку.",
    description: "Toast after Portal session creation fails",
  },
  "usage.paymentRequiredTitle": {
    message: "Добавьте способ оплаты, чтобы продолжить",
    description: "Just-in-time paid-intent onboarding dialog title",
  },
  "usage.paymentRequiredDescription": {
    message:
      "Для этого платного тарифа нужен способ оплаты. Завершите Stripe Checkout в новой вкладке; действие возобновится автоматически, когда всё будет готово.",
    description: "Just-in-time paid-intent onboarding explanation",
  },
  "usage.paymentRequiredRetrying": {
    message: "Способ оплаты найден. Повторяем ваше действие…",
    description: "Status while resuming the interrupted paid mutation",
  },
  "usage.paymentRequiredCancel": {
    message: "Отмена",
    description: "Cancel just-in-time payment onboarding",
  },
  "usage.planTitle": {
    message: "Тариф",
    description: "Plan card title on the billing page",
  },
  "usage.planChange": {
    message: "Сменить тариф",
    description:
      "Link from the billing page's plan card to the workspace-settings plan dialog",
  },
  "usage.paymentMethodTitle": {
    message: "Способ оплаты",
    description: "Payment-method card title on the billing page",
  },
  "usage.paymentMethodCard": {
    message: "{brand} •••• {last4}",
    description: "Names the card on file, e.g. 'Visa ending in 4242'",
  },
  "usage.paymentMethodOnFile": {
    message: "Способ оплаты сохранён",
    description:
      "Shown when a payment method exists but is not a card the provider could name",
  },
  "usage.paymentMethodNone": {
    message: "Нет способа оплаты",
    description: "Shown when the workspace has no payment method",
  },
  "usage.billingEmailTitle": {
    message: "Email для счетов",
    description:
      "Billing email card title and section navigation entry on the billing page (w6/m627)",
  },
  "usage.billingEmailDescription": {
    message:
      "Уведомления о расчётах этого рабочего пространства приходят на этот адрес, и счета выставляются на него.",
    description: "Billing email card description: what the address is used for",
  },
  "usage.billingEmailHobbyNote": {
    message:
      "В рабочих пространствах Hobby используется email аккаунта владельца. Отдельный email для счетов доступен на тарифе Pro и выше.",
    description:
      "Note under a Hobby workspace's billing email, which is read-only and is the owner's account email",
  },
  "usage.billingEmailNone": {
    message: "Email для счетов не указан",
    description:
      "Shown when neither a billing email nor the owner's account email is known",
  },
  "usage.billingEmailUnreadable": {
    message: "Не удалось загрузить email для счетов.",
    description: "Shown when the billing email could not be read",
  },
  "usage.billingEmailEdit": {
    message: "Изменить",
    description: "Button that opens the billing email dialog (Pro and higher)",
  },
  "usage.billingEmailDialogTitle": {
    message: "Изменить email для счетов",
    description: "Title of the dialog that sets the billing email",
  },
  "usage.billingEmailDialogDescription": {
    message:
      "Уведомления о расчётах также приходят администраторам рабочего пространства на их собственные адреса.",
    description: "Description in the billing email dialog",
  },
  "usage.billingEmailLabel": {
    message: "Адрес email",
    description: "Label of the billing email input",
  },
  "usage.billingEmailSave": {
    message: "Сохранить",
    description: "Save button of the billing email dialog",
  },
  "usage.billingEmailSaved": {
    message: "Email для счетов изменён на {email}",
    description:
      "Toast after the billing email was saved; {email} is the new address",
  },
  "usage.billingEmailError": {
    message: "Не удалось обновить email для счетов",
    description: "Fallback toast when the billing email could not be saved",
  },
  "usage.billingInfoTitle": {
    message: "Платёжные реквизиты",
    description:
      "Billing info card title and section navigation entry on the billing page (w6/m628)",
  },
  "usage.billingInfoDescription": {
    message:
      "Название компании, адрес и номер плательщика НДС или налоговый номер, которые указываются в каждом счёте.",
    description: "Billing info card description: what the billing info is for",
  },
  "usage.billingInfoBillTo": {
    message: "Плательщик",
    description:
      "Label of the name invoices bill: the company name when set, else the workspace's name",
  },
  "usage.billingInfoCompanyName": {
    message: "Название компании",
    description:
      "Label of the company name, on the billing info card and in its dialog",
  },
  "usage.billingInfoAddress": {
    message: "Адрес",
    description: "Label of the billing address on the billing info card",
  },
  "usage.billingInfoTaxId": {
    message: "Налоговый номер",
    description:
      "Label of the VAT or tax ID on the billing info card, and of its value in the dialog when the country has several types",
  },
  "usage.billingInfoNotSet": {
    message: "Не указано",
    description: "Shown for a part of the billing info that is not set",
  },
  "usage.billingInfoUnreadable": {
    message: "Не удалось загрузить платёжные реквизиты.",
    description: "Shown when the billing info could not be read",
  },
  "usage.billingInfoCustomerNotReady": {
    message:
      "Сначала добавьте способ оплаты: платёжные реквизиты хранятся вместе с ним.",
    description:
      "Note on the billing info card, and error toast, while the workspace has no billing account (no payment method was ever added)",
  },
  "usage.billingInfoAdd": {
    message: "Добавить реквизиты",
    description:
      "Button that opens the billing info dialog when no billing info is set",
  },
  "usage.billingInfoEdit": {
    message: "Изменить",
    description:
      "Button that opens the billing info dialog when billing info is set",
  },
  "usage.billingInfoDialogTitle": {
    message: "Платёжные реквизиты",
    description: "Title of the dialog that sets the billing info",
  },
  "usage.billingInfoDialogDescription": {
    message:
      "Дополнительные платёжные реквизиты, например название компании, адрес или номер плательщика НДС, — с этого момента они указываются во всех счетах.",
    description: "Description in the billing info dialog (Render's wording)",
  },
  "usage.billingInfoCompanyNameHint": {
    message:
      "Оставьте пустым, чтобы в счетах указывалось название рабочего пространства.",
    description:
      "Hint under the company name field: an empty company name bills the workspace's name",
  },
  "usage.billingInfoCountry": {
    message: "Страна",
    description: "Label of the billing address country picker",
  },
  "usage.billingInfoCountryPlaceholder": {
    message: "Выберите страну",
    description: "Placeholder of the country picker when no country is chosen",
  },
  "usage.billingInfoNoCountry": {
    message: "Без страны",
    description:
      "Country picker choice that clears the country (and so the address)",
  },
  "usage.billingInfoCountryRequired": {
    message: "Выберите страну",
    description:
      "Error under the country picker when an address line was typed without a country",
  },
  "usage.billingInfoLine1": {
    message: "Адрес, строка 1",
    description: "Label of the first street address line",
  },
  "usage.billingInfoLine2": {
    message: "Адрес, строка 2",
    description: "Label of the second street address line",
  },
  "usage.billingInfoCity": {
    message: "Город",
    description: "Label of the city field",
  },
  "usage.billingInfoState": {
    message: "Штат / провинция / регион",
    description: "Label of the state, province or region field",
  },
  "usage.billingInfoPostalCode": {
    message: "Почтовый индекс",
    description: "Label of the postal code field",
  },
  "usage.billingInfoTaxIdType": {
    message: "Тип налогового номера",
    description:
      "Label of the tax ID type picker, shown when the country has several types",
  },
  "usage.billingInfoSave": {
    message: "Сохранить",
    description: "Save button of the billing info dialog",
  },
  "usage.billingInfoSaved": {
    message: "Реквизиты сохранены. Новые счета будут выставляться на {name}.",
    description:
      "Toast after the billing info was saved; {name} is the name new invoices bill",
  },
  "usage.billingInfoError": {
    message: "Не удалось сохранить платёжные реквизиты",
    description: "Fallback toast when the billing info could not be saved",
  },
  "usage.includedUsageTitle": {
    message: "Включённое использование",
    description:
      "Plan-allowance card title on the billing page (was 'Resource limits')",
  },
  "usage.includedUsageDescription": {
    message:
      "Что включает тариф этого рабочего пространства и сколько из этого уже использовано.",
    description: "Included-usage card description",
  },
  "usage.chargesTitle": {
    message: "Начисления",
    description: "Charge-tree card title on the billing page",
  },
  "usage.chargesDescriptionEstimate": {
    message:
      "Начислено за текущий период по прайс-листу bex. Это оценка, а не счёт.",
    description:
      "Charge-tree description when no Stripe subscription prices the period",
  },
  "usage.chargesDescriptionInvoiced": {
    message: "Начислено за текущий период по расчёту Stripe.",
    description:
      "Charge-tree description when a real Stripe amount is available. Says the total is Stripe's rating rather than the amount it will invoice: credits and comp discounts can sit between the two, and the amount actually due gets its own line (w6/m98).",
  },
  "usage.amountDueAfterCredits": {
    message: "К оплате после кредитов",
    description:
      "Label for the charge-tree line showing what Stripe actually collects once credits and discounts are applied to the charge above it",
  },
  "usage.chargesEmpty": {
    message: "В этом периоде нет использования.",
    description: "Charge-tree empty state",
  },
  "usage.coveragePartial": {
    message: "Неполные данные",
    description:
      "Amber caveat label above the charges when the metering behind the estimate is degraded/incomplete (w4/048), mirroring the Metrics page's degraded badge",
  },
  "usage.coveragePartialLead": {
    message: "Эта оценка может занижать использование.",
    description: "Lead sentence of the partial-coverage caveat tooltip",
  },
  "usage.coveragePartialThrough": {
    message: "Измерения учтены по {through}.",
    description:
      "Partial-coverage caveat clause naming the date the meters have recorded through (a degraded hour counts as recorded, w4/496); {through} is a YYYY-MM-DD date",
  },
  "usage.coveragePartialSources": {
    message: "Источники измерений со сбоями: {sources}.",
    description:
      "Partial-coverage caveat clause listing the degraded metering source names; {sources} is a comma-separated list",
  },
  "usage.expandAll": {
    message: "Развернуть все",
    description: "Charge-tree button that opens every category",
  },
  "usage.collapseAll": {
    message: "Свернуть все",
    description: "Charge-tree button that closes every category",
  },
  "usage.totalToDate": {
    message: "Итого с начала месяца",
    description: "Charge-tree total row for the month in progress",
  },
  "usage.totalForPeriod": {
    message: "Итого за период",
    description: "Charge-tree total row for a completed past month",
  },
  "usage.totalBillingPeriodWindow": {
    message: "Итого за этот расчётный период ({start} – {end})",
    description:
      "Charge total row when the headline is Stripe's rated amount: it covers the subscription period (anchored on the subscription's day of month), not the calendar month (w4/m139)",
  },
  "usage.totalBillingPeriod": {
    message: "Итого за этот расчётный период",
    description:
      "Charge total row for Stripe's rated amount when its period bounds are unknown (w4/m139)",
  },
  "usage.breakdownMonthToDate": {
    message: "Детализация выше: {month} с начала месяца",
    description:
      "Row under a Stripe-rated total: the charge tree's own sum and window (the calendar month), which differs from the subscription period (w4/m139)",
  },
  "usage.projectedTotal": {
    message: "Прогноз итога за {month}",
    description:
      "Charge-tree straight-line projection of month-end spend, for the estimate fallback whose total accrues over the calendar month",
  },
  "usage.projectedTotalBillingPeriod": {
    message: "Прогноз за этот расчётный период",
    description:
      "Charge-tree straight-line projection of a Stripe-rated total. The subscription period need not align with the calendar month (it can span two), so no month is named (w6/050).",
  },
  "usage.chargeFree": {
    message: "Включено",
    description: "Rate column value for a charge line that is priced at zero",
  },
  "usage.resourceDeleted": {
    message: "(удалён)",
    description:
      "Marker beside a charged resource that no longer exists; the name shown is the one bex retained",
  },
  "usage.categoryServices": {
    message: "Сервисы",
    description: "Charge-tree category for App services",
  },
  "usage.categoryPostgres": {
    message: "Postgres",
    description: "Charge-tree category for managed Postgres instances",
  },
  "usage.categoryKeyValue": {
    message: "Key Value",
    description: "Charge-tree category for managed Key Value instances",
  },
  "usage.categoryObjectStorage": {
    message: "Объектное хранилище",
    description:
      "Charge-tree category for a workspace's object storage (ADR112)",
  },
  "usage.categoryAgentSessions": {
    message: "Сессии агентов",
    description:
      "Charge-tree category for platform model usage of agent sessions",
  },
  "usage.chargeDisk": {
    message: "Диск",
    description: "Charge line for a service's provisioned persistent disk",
  },
  "usage.chargeAgentModelCost": {
    message: "Использование моделей",
    description:
      "Charge line for platform model spend, billed at provider cost plus markup (ADR091)",
  },
  "usage.chargeAgentModelRate": {
    message: "Стоимость провайдера + 30%",
    description: "Rate column value for the platform model spend line",
  },
  "usage.chargeAgentModelInputTokens": {
    message: "Входные токены",
    description: "Breakdown line for platform model input tokens",
  },
  "usage.chargeAgentModelOutputTokens": {
    message: "Выходные токены",
    description: "Breakdown line for platform model output tokens",
  },
  "usage.chargeAgentModelCacheReadTokens": {
    message: "Токены чтения кеша",
    description: "Breakdown line for platform model cache-read tokens",
  },
  "usage.chargeAgentModelCacheWriteTokens": {
    message: "Токены записи кеша",
    description: "Breakdown line for platform model cache-write tokens",
  },
  "usage.categorySandboxes": {
    message: "Песочницы",
    description: "Charge-tree category for hosted agent sandboxes",
  },
  "usage.chargeCompute": {
    message: "Вычисления",
    description: "Charge line for metered instance time",
  },
  "usage.chargeBandwidth": {
    message: "Трафик",
    description: "Charge line for metered outbound bandwidth",
  },
  "usage.chargeBuild": {
    message: "Минуты сборки",
    description: "Charge line for metered build time",
  },
  "usage.chargeStorage": {
    message: "Хранилище",
    description: "Charge line for metered datastore storage",
  },
  "usage.chargeSandboxCompute": {
    message: "Вычисления песочницы",
    description: "Charge line for metered sandbox compute",
  },
  "usage.chargeSandboxMemory": {
    message: "Память песочницы",
    description:
      "Charge line for the memory a running sandbox was allocated, billed per GiB-hour since billing switched to use",
  },
  "usage.chargeSandboxMemoryUnbilled": {
    message: "Память песочницы (входит в вычисления)",
    description:
      "Charge line for the allocated memory a sandbox ran with before billing switched to use; recorded beside sandbox compute, which already includes it, so it is shown and not billed separately",
  },
  "usage.chargeSandboxCpu": {
    message: "CPU песочницы",
    description:
      "Charge line for the CPU a running sandbox used, billed per CPU-hour used",
  },
  "usage.chargeSandboxCpuUnbilled": {
    message: "CPU песочницы (входит в вычисления)",
    description:
      "Charge line for the CPU a sandbox used before billing switched to use; recorded beside sandbox compute, which already charged for it, so it is shown and not billed separately",
  },
  "usage.sandboxUsageBilledFromNote": {
    message:
      "С {date} UTC работающая песочница оплачивается по использованному CPU и выделенной памяти. Более раннее использование оплачивалось как вычисления песочницы.",
    description:
      "Note under the charge tree of a period with sandbox usage; date is the instant billing switched, formatted in UTC",
  },
  "usage.agentSandboxUsageBilledFromNote": {
    message:
      "Песочница сессии агента оплачивается так с {date} UTC; до этого её использование оплачивается как вычисления песочницы.",
    description:
      "Second sentence of the note under the charge tree of a period with sandbox usage; date is the instant an agent session's sandbox switched from sandbox compute to used CPU and allocated memory, formatted in UTC (it may be in the future)",
  },
  "usage.chargeSandboxSnapshotStorage": {
    message: "Хранение снимков песочницы",
    description:
      "Charge line for the image a suspended sandbox retains, billed per GB-month",
  },
  "usage.chargeObjectStorage": {
    message: "Хранилище",
    description:
      "Charge line for the data a workspace keeps in object storage, billed per GB-month (shown at no charge until a price is set, ADR112 D8)",
  },
  "usage.chargeObjectStorageIngress": {
    message: "Входящий трафик",
    description:
      "Charge line for the bytes put into a workspace's object storage (shown at no charge, ADR112 D8)",
  },
  "usage.chargeObjectStorageEgress": {
    message: "Исходящий трафик",
    description:
      "Charge line for the bytes read out of a workspace's object storage, public reads included (shown at no charge, ADR112 D8)",
  },
  "usage.creditsTotalLabel": {
    message: "Общий баланс",
    description: "Label above the credit balance amount",
  },
  "usage.invoiceHistoryTitle": {
    message: "История счетов",
    description: "Invoice-history card title on the billing page",
  },
  "usage.invoiceHistoryDescription": {
    message: "Ваши выставленные счета.",
    description: "Invoice-history card description",
  },
  "usage.invoiceKindAgentModelCharge": {
    message: "Списание за модели агентов",
    description:
      "Label on an invoice-history row for a charge made when agent model spend reached its threshold",
  },
  "usage.invoiceActionsMenu": {
    message: "Действия со счётом за {period}",
    description:
      "Accessible label of an invoice row's three-dot menu; {period} is the invoice's month",
  },
  "usage.invoicePayNow": {
    message: "Оплатить",
    description:
      "Invoice row menu item that opens Stripe's hosted page to pay an unpaid invoice (Render's Pay Now)",
  },
  "usage.invoiceDownloadPdf": {
    message: "Скачать PDF",
    description: "Invoice row menu item that downloads the invoice as a PDF",
  },
  "usage.invoiceStatusOpen": {
    message: "Не оплачен",
    description:
      "Status of an invoice that is finalized and not yet paid (Stripe: open)",
  },
  "usage.invoiceStatusPaid": {
    message: "Оплачен",
    description: "Status of a paid invoice (Stripe: paid)",
  },
  "usage.invoiceStatusVoid": {
    message: "Аннулирован",
    description:
      "Status of a voided invoice, which is owed no longer (Stripe: void)",
  },
  "usage.invoiceStatusUncollectible": {
    message: "Безнадёжный",
    description:
      "Status of an invoice written off as uncollectible, which can still be paid (Stripe: uncollectible)",
  },
  "usage.invoiceHistoryChargesNote": {
    message:
      "Списания за модели агентов, сделанные в течение месяца, вычитаются из счёта за этот месяц.",
    description:
      "Invoice-history note when the list includes agent model charges",
  },
  "usage.creditsAppliedLine": {
    message: "Применено кредитов −{applied} → к оплате {due}",
    description:
      "Credit applied to the current period and the remaining amount due",
  },
  "usage.chargesDescriptionPending": {
    message: "Начислено за текущий период по прайс-листу bex.",
    description:
      "Charges card description while the invoiced total is still loading; deliberately states only what is already true, without claiming the figure is or is not a Stripe invoice.",
  },
  "usage.paymentMethodRejectedPrepaid": {
    message:
      "Предоплаченные карты не принимаются; добавьте кредитную или дебетовую карту.",
    description:
      "Shown when the last card offered was refused at binding because it is prepaid (ADR091 D10.5)",
  },
  "usage.paymentMethodRejectedUnpaidBalance": {
    message:
      "Эта карта связана с неоплаченной задолженностью; добавьте другую карту.",
    description:
      "Shown when the last card offered was refused because it carries an unpaid written-off balance (ADR091 D10.6)",
  },
  "usage.agentModelTitle": {
    message: "Модель агента",
    description:
      "Billing page card and nav entry for agent model spend (ADR091 D6)",
  },
  "usage.agentModelPricing": {
    message: "Оплата по стоимости провайдера модели + 30%.",
    description: "Pricing statement under the Agent model title",
  },
  "usage.agentModelMonthToDate": {
    message: "С начала месяца",
    description: "Label of the current month's agent model total",
  },
  "usage.agentModelPeriodTotal": {
    message: "Итого за месяц",
    description: "Label of a closed month's agent model total",
  },
  "usage.agentModelBreakdown_one": {
    message: "Разбивка по моделям ({count} модель)",
    description: "Toggle that expands the per-model breakdown",
  },
  "usage.agentModelBreakdown_few": {
    message: "Разбивка по моделям ({count} модели)",
    description: "Toggle that expands the per-model breakdown",
  },
  "usage.agentModelBreakdown_many": {
    message: "Разбивка по моделям ({count} моделей)",
    description: "Toggle that expands the per-model breakdown",
  },
  "usage.agentModelBreakdown_other": {
    message: "Разбивка по моделям ({count} модели)",
    description: "Toggle that expands the per-model breakdown",
  },
  "usage.agentModelEmpty": {
    message: "В этом периоде нет использования моделей агентов.",
    description: "Shown when the period has no agent model spend",
  },
  "usage.lessThanAmount": {
    message: "<{amount}",
    description:
      "A charge that rounds below the smallest amount shown, e.g. <$0.01",
  },
};

export default ruUsage;
