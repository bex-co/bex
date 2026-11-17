import type { TranslationEntry } from "@/i18n";

const ptUsage: Record<string, TranslationEntry> = {
  "usage.pageTitle": {
    message: "Faturamento",
    description:
      "Billing page heading and browser title (renamed from Usage in w5/m70)",
  },
  "usage.pageSubtitle": {
    message: "Pagamento, faturas e consumo do workspace no mês até agora",
    description: "Billing page subtitle beneath the heading",
  },
  "usage.sectionNavigation": {
    message: "Seções de faturamento",
    description: "Accessible label for the billing page section navigation",
  },
  "usage.resourceCapsServices": {
    message: "Serviços",
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
    message: "Gratuitas: {used} de {limit}",
    description: "Free-instance count in the resource-cap card (w4/m241)",
  },
  "usage.freeHoursTitle": {
    message: "Uso incluído mensal",
    description:
      "Title of the Free instance hours card on the usage page (w8/m139)",
  },
  "usage.freeHoursDescription": {
    message:
      "Web Services gratuitos compartilham uma cota mensal de horas de instância por workspace. O tempo em que ficam desligados não conta.",
    description: "Explains the monthly Free instance hours allowance (w8/m139)",
  },
  "usage.freeHoursUsed": {
    message:
      "{used} de {included} horas de instância gratuitas usadas neste mês",
    description:
      "Free instance hours used against the monthly allowance (w8/m139)",
  },
  "usage.freeHoursNear": {
    message:
      "Você está perto da cota. Quando ela acabar, seus Web Services gratuitos serão suspensos até o início do próximo mês.",
    description:
      "Warning once Free instance hours reach 80 percent of the allowance (w8/m139)",
  },
  "usage.freeHoursSpent": {
    message:
      "A cota acabou. Os Web Services gratuitos ficam suspensos até o início do próximo mês; faça upgrade de um serviço para um plano pago para executá-lo agora.",
    description:
      "Notice once the monthly Free instance hours are spent (w8/m139)",
  },
  "usage.buildMinutesTitle": {
    message: "Minutos de pipeline",
    description:
      "Title of the included build minutes card on the billing page (w6/m308)",
  },
  "usage.buildMinutesDescription": {
    message:
      "Builds e comandos de pré-deploy usam minutos de pipeline. Seu plano inclui alguns por mês; os minutos além deles são cobrados por minuto.",
    description: "Explains the plan's included build minutes (w6/m308)",
  },
  "usage.buildMinutesUsed": {
    message: "{used} de {included} minutos de build incluídos usados neste mês",
    description:
      "Build minutes used against the plan's included minutes (w6/m308)",
  },
  "usage.buildMinutesNear": {
    message:
      "Você está perto dos seus minutos de build incluídos. Sem uma forma de pagamento, novos builds e comandos de pré-deploy param quando eles acabarem, até o início do próximo mês.",
    description:
      "Warning once build minutes reach 80 percent of the included minutes (w6/m308)",
  },
  "usage.buildMinutesSpent": {
    message:
      "Os minutos de build incluídos acabaram. Os minutos adicionais são cobrados por minuto; sem uma forma de pagamento, novos builds e comandos de pré-deploy param até o início do próximo mês.",
    description: "Notice once the included build minutes are spent (w6/m308)",
  },
  "usage.bandwidthTitle": {
    message: "Largura de banda de saída",
    description:
      "Title of the included bandwidth card on the billing page (w6/m319)",
  },
  "usage.bandwidthDescription": {
    message:
      "O tráfego que seus serviços enviam para a internet usa largura de banda. Seu plano inclui uma parte por mês; a largura de banda além dela é cobrada por GB.",
    description: "Explains the plan's included outbound bandwidth (w6/m319)",
  },
  "usage.bandwidthUsed": {
    message:
      "{used} de {included} GB de largura de banda incluída usados neste mês",
    description:
      "Outbound bandwidth used against the plan's included bandwidth, in GB (w6/m319)",
  },
  "usage.bandwidthNear": {
    message:
      "Você está perto da largura de banda incluída. Sem uma forma de pagamento, os serviços do seu workspace são desligados quando ela acabar, até o início do próximo mês.",
    description:
      "Warning once bandwidth reaches 80 percent of the included bandwidth (w6/m319)",
  },
  "usage.bandwidthSpent": {
    message:
      "A largura de banda incluída acabou. A largura de banda adicional é cobrada por GB; sem uma forma de pagamento, os serviços do seu workspace ficam desligados até o início do próximo mês.",
    description: "Notice once the included bandwidth is spent (w6/m319)",
  },
  "usage.customDomainsTitle": {
    message: "Domínios personalizados",
    description:
      "Title of the included custom domains card on the billing page (w7/m429)",
  },
  "usage.customDomainsDescription": {
    message:
      "Seu plano inclui uma quantidade de domínios personalizados verificados. Cada domínio além deles é cobrado por mês, pelo tempo em que fica verificado. Adicionar um domínio nunca é recusado pela quantidade.",
    description:
      "Explains the plan's included custom domains and how domains beyond them are billed (w7/m429)",
  },
  "usage.customDomainsUsed": {
    message: "{used} de {included} domínios personalizados incluídos em uso",
    description:
      "Verified custom domains now against the count the plan includes (w7/m429)",
  },
  "usage.customDomainsNear": {
    message:
      "Você está perto dos domínios personalizados incluídos no seu plano. Cada domínio além deles é cobrado por mês.",
    description:
      "Note once the verified custom domains reach 80 percent of the included count (w7/m429)",
  },
  "usage.customDomainsSpent": {
    message:
      "Os domínios personalizados incluídos no seu plano estão em uso. Cada domínio verificado adicional é cobrado por mês.",
    description: "Notice once every included custom domain is in use (w7/m429)",
  },
  "usage.categoryCustomDomains": {
    message: "Domínios personalizados",
    description:
      "Charge-tree category for a workspace's custom domains beyond its plan (w7/m429)",
  },
  "usage.chargeCronMinimum": {
    message: "Mínimo do Cron Job",
    description:
      "Charge line under a cron job: what its runs in a month cost short of the cron job minimum, charged once the month ends (w9/m448)",
  },
  "usage.chargeCronMinimumRate": {
    message: "Mínimo de ${minimum}/mês",
    description:
      "Rate column value for the cron job minimum line; minimum is the USD amount without the currency symbol (w9/m448)",
  },
  "usage.chargeCustomDomains": {
    message: "Domínios personalizados além do plano",
    description:
      "Charge line for custom domains beyond the plan's included count, billed per domain-month (w7/m429)",
  },
  "usage.chargeIncluded": {
    message: "{included} incluídos",
    description:
      "Part of a charge line's quantity that the plan's included build minutes or bandwidth covered (w6/m308, w6/m319)",
  },
  "usage.pipelineTitle": {
    message: "Pipeline de build",
    description:
      "Title of the Build Pipeline card in workspace settings (w6/m309)",
  },
  "usage.pipelineDescription": {
    message:
      "Builds e comandos de pré-deploy usam minutos de pipeline. Os minutos além dos incluídos no seu plano são cobrados por minuto, até o limite de gastos mensal que você definir. Quando o limite é atingido, os builds param até o próximo mês.",
    description: "Explains the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitValue": {
    message: "Limite de gastos: {limit} por mês",
    description: "The workspace pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitNone": {
    message: "Sem limite de gastos",
    description: "The workspace has no pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitSet": {
    message: "Definir limite de gastos",
    description: "Button that opens the pipeline spend limit dialog (w6/m309)",
  },
  "usage.pipelineSpendLimitEdit": {
    message: "Editar",
    description: "Button that edits the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitRemove": {
    message: "Remover limite",
    description: "Button that removes the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitDialogTitle": {
    message: "Definir limite de gastos",
    description: "Title of the pipeline spend limit dialog (w6/m309)",
  },
  "usage.pipelineSpendLimitDialogDescription": {
    message:
      "O máximo que este workspace gasta por mês com minutos de build além dos incluídos no plano. Quando ele é atingido, novos builds e comandos de pré-deploy param até o início do próximo mês.",
    description: "Explains what the pipeline spend limit caps (w6/m309)",
  },
  "usage.pipelineSpendLimitLabel": {
    message: "Limite mensal (USD)",
    description: "Label of the pipeline spend limit input (w6/m309)",
  },
  "usage.pipelineSpendLimitSave": {
    message: "Salvar",
    description: "Saves the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitSaved": {
    message: "Limite de gastos definido em {limit} por mês",
    description: "Toast after setting the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitRemoved": {
    message: "Limite de gastos removido",
    description: "Toast after removing the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitError": {
    message: "Não foi possível atualizar o limite de gastos",
    description: "Toast when updating the pipeline spend limit fails (w6/m309)",
  },
  "usage.pipelineSpendLimitUnreadable": {
    message:
      "Não foi possível carregar o limite de gastos. Recarregue para tentar novamente.",
    description: "The pipeline spend limit could not be read (w6/m309)",
  },
  "usage.pipelineSpendLimitForbidden": {
    message:
      "Somente quem gerencia o faturamento pode ver e alterar o limite de gastos.",
    description: "The caller may not read the pipeline spend limit (w6/m309)",
  },
  "usage.resourceCapsNearLimit": {
    message: "Perto do limite",
    description:
      "Warning shown once a resource count reaches 80 percent of its cap, below the cap itself",
  },
  "usage.resourceCapsLimitReached": {
    message: "Limite atingido",
    description:
      "Shown when a resource count is at or over its cap, so creating another is refused",
  },
  "usage.resourceCapsFinishingDeletion": {
    message: "{count} concluindo a exclusão",
    description:
      "Sub-line on a resource-cap tile: how many of the counted resources are mid-deletion and thus hidden from the resource list but still holding quota (w6/m129)",
  },
  "usage.resourceCapsFinishingDeletionHint": {
    message: "Continua contando para o seu limite até a exclusão ser concluída",
    description:
      "Tooltip explaining why resources finishing deletion are included in the used count",
  },
  "usage.errorTitle": {
    message: "Não foi possível carregar o uso",
    description: "Error state heading on the Usage page",
  },
  "usage.monthPickerLabel": {
    message: "Selecionar mês",
    description: "Aria-label for the month-picker select on the Usage page",
  },
  "usage.currentMonth": {
    message: "Mês atual",
    description:
      "Default option in the month picker meaning the current calendar month",
  },
  "usage.creditsTitle": {
    message: "Créditos restantes",
    description: "Heading of the remaining billing-credit card (w5/m70)",
  },
  "usage.creditsDescription": {
    message:
      "Crédito promocional aplicado às faturas antes da cobrança da sua forma de pagamento",
    description: "Subtitle of the remaining billing-credit card",
  },
  "usage.creditsExpiryNote": {
    message: "{amount} expiram em {date}",
    description:
      "Earliest-expiring grant note beside the credit balance; amount is a $ value, date is YYYY-MM-DD",
  },
  "usage.creditsCardStillRequired": {
    message:
      "Uma forma de pagamento ainda é necessária mesmo com crédito: o crédito cobre as faturas primeiro, e seu cartão paga o que sobrar.",
    description:
      "ADR046 clarification on the credit card — credit does not replace payment onboarding",
  },
  "usage.colInvoicePeriod": {
    message: "Período",
    description: "Table column header for an invoice's billing period start",
  },
  "usage.colInvoiceStatus": {
    message: "Status",
    description: "Table column header for an invoice's status",
  },
  "usage.colInvoiceAmount": {
    message: "Valor",
    description: "Table column header for an invoice's amount",
  },
  "usage.billingSetupTitle": {
    message: "Configuração de faturamento",
    description: "Customer billing onboarding card title",
  },
  "usage.billingSetupDescription": {
    message: "Cobrança de pagamentos e preparo fiscal deste workspace",
    description: "Customer billing onboarding card description",
  },
  "usage.billingTestMode": {
    message: "Stripe em modo de teste",
    description: "Badge making the non-live Stripe environment explicit",
  },
  "usage.billingMode": {
    message: "Stripe Billing",
    description: "Fallback badge for the Stripe billing environment",
  },
  "usage.billingReady": {
    message: "Pronto",
    description: "Billing onboarding status is ready",
  },
  "usage.billingActionNeeded": {
    message: "Ação necessária",
    description: "Billing onboarding status needs action",
  },
  "usage.billingCustomerStatus": {
    message: "Cliente",
    description: "Stripe Customer readiness row",
  },
  "usage.billingSubscriptionStatus": {
    message: "Assinatura medida",
    description: "Stripe Subscription readiness row",
  },
  "usage.billingPaymentStatus": {
    message: "Forma de pagamento",
    description: "Default payment method readiness row",
  },
  "usage.billingTaxStatus": {
    message: "Imposto automático",
    description: "Stripe Tax activation row",
  },
  "usage.billingLifecycleStatus": {
    message: "Ciclo de cobrança",
    description: "Stripe collection and reversible enforcement state row",
  },
  "usage.billingLifecycleGrace": {
    message:
      "O pagamento falhou ({reason}). O workspace continua disponível durante o período de carência; a suspensão reversível está agendada para depois de {deadline}.",
    description: "Visible billing grace state and authoritative deadline",
  },
  "usage.billingLifecycleEnforced": {
    message:
      "A aplicação do faturamento está ativa ({reason}). A computação está suspensa, mas bancos de dados, dados Key Value, segredos e registros de faturamento não foram excluídos.",
    description: "Visible reversible billing enforcement state",
  },
  "usage.billingLifecycleRecovering": {
    message:
      "Pagamento recuperado. O bex está restaurando somente os recursos alterados pela aplicação do faturamento.",
    description: "Visible precise recovery state",
  },
  "usage.billingLifecycleExcluded": {
    message:
      "Este workspace foi excluído da cobrança do Stripe por um operador.",
    description: "Visible structural billing exclusion state",
  },
  "usage.billingLifecycleComped": {
    message:
      "Este workspace é tarifado, mas totalmente isento por um operador.",
    description: "Visible rated-but-free comp state",
  },
  "usage.billingLifecycleUnknown": {
    message:
      "O estado do faturamento é {reason}. Use o portal de faturamento ou entre em contato com o suporte.",
    description: "Forward-compatible unknown billing lifecycle state",
  },
  "usage.billingNoDeadline": {
    message: "nenhum prazo informado",
    description: "Fallback when a malformed grace state has no deadline",
  },
  "usage.billingOff": {
    message: "Desativado",
    description:
      "Neutral status for a deliberately disabled billing capability (e.g. tax not activated)",
  },
  "usage.billingAddPayment": {
    message: "Adicionar forma de pagamento",
    description: "Button opening setup-mode Stripe Checkout (live mode)",
  },
  "usage.billingUpdatePayment": {
    message: "Atualizar forma de pagamento",
    description: "Button reopening setup-mode Stripe Checkout (live mode)",
  },
  "usage.billingAddPaymentTest": {
    message: "Adicionar forma de pagamento de teste",
    description:
      "Button opening setup-mode Stripe Checkout when Stripe is in test mode",
  },
  "usage.billingUpdatePaymentTest": {
    message: "Atualizar forma de pagamento de teste",
    description:
      "Button reopening setup-mode Stripe Checkout when Stripe is in test mode",
  },
  "usage.billingOpenPortal": {
    message: "Abrir portal de faturamento",
    description: "Button opening Stripe Customer Portal",
  },
  "usage.billingHostedNote": {
    message:
      "Os dados de pagamento são informados somente em páginas hospedadas pelo Stripe. O bex nunca envia uma chave de servidor do Stripe ao seu navegador.",
    description: "Hosted billing security note",
  },
  "usage.billingUnavailable": {
    message:
      "A configuração de faturamento está indisponível ou você não tem acesso ao faturamento (função de faturamento ou administrador).",
    description: "Degraded or unauthorized billing onboarding state",
  },
  "usage.billingCheckoutError": {
    message: "Não foi possível abrir o Stripe Checkout. Tente novamente.",
    description: "Toast after Checkout session creation fails",
  },
  "usage.billingPortalError": {
    message:
      "Não foi possível abrir o portal de faturamento do Stripe. Tente novamente.",
    description: "Toast after Portal session creation fails",
  },
  "usage.paymentRequiredTitle": {
    message: "Adicione uma forma de pagamento para continuar",
    description: "Just-in-time paid-intent onboarding dialog title",
  },
  "usage.paymentRequiredDescription": {
    message:
      "Este plano pago precisa de uma forma de pagamento. Conclua o Stripe Checkout na nova aba; esta ação será retomada automaticamente quando estiver pronta.",
    description: "Just-in-time paid-intent onboarding explanation",
  },
  "usage.paymentRequiredRetrying": {
    message: "Forma de pagamento encontrada. Tentando sua ação novamente…",
    description: "Status while resuming the interrupted paid mutation",
  },
  "usage.paymentRequiredCancel": {
    message: "Cancelar",
    description: "Cancel just-in-time payment onboarding",
  },
  "usage.planTitle": {
    message: "Plano",
    description: "Plan card title on the billing page",
  },
  "usage.planChange": {
    message: "Alterar plano",
    description:
      "Link from the billing page's plan card to the workspace-settings plan dialog",
  },
  "usage.paymentMethodTitle": {
    message: "Forma de pagamento",
    description: "Payment-method card title on the billing page",
  },
  "usage.paymentMethodCard": {
    message: "{brand} com final {last4}",
    description: "Names the card on file, e.g. 'Visa ending in 4242'",
  },
  "usage.paymentMethodOnFile": {
    message: "Forma de pagamento cadastrada",
    description:
      "Shown when a payment method exists but is not a card the provider could name",
  },
  "usage.paymentMethodNone": {
    message: "Nenhuma forma de pagamento",
    description: "Shown when the workspace has no payment method",
  },
  "usage.includedUsageTitle": {
    message: "Uso incluído",
    description:
      "Plan-allowance card title on the billing page (was 'Resource limits')",
  },
  "usage.includedUsageDescription": {
    message: "O que o plano deste workspace inclui e quanto dele está em uso.",
    description: "Included-usage card description",
  },
  "usage.chargesTitle": {
    message: "Cobranças",
    description: "Charge-tree card title on the billing page",
  },
  "usage.chargesDescriptionEstimate": {
    message:
      "Acumulado até agora neste período, calculado pela tabela de preços do bex. Uma estimativa, não uma fatura.",
    description:
      "Charge-tree description when no Stripe subscription prices the period",
  },
  "usage.chargesDescriptionInvoiced": {
    message:
      "Acumulado até agora neste período, conforme tarifado pelo Stripe.",
    description:
      "Charge-tree description when a real Stripe amount is available. Says the total is Stripe's rating rather than the amount it will invoice: credits and comp discounts can sit between the two, and the amount actually due gets its own line (w6/m98).",
  },
  "usage.amountDueAfterCredits": {
    message: "Valor a pagar após créditos",
    description:
      "Label for the charge-tree line showing what Stripe actually collects once credits and discounts are applied to the charge above it",
  },
  "usage.chargesEmpty": {
    message: "Nenhum uso neste período.",
    description: "Charge-tree empty state",
  },
  "usage.coveragePartial": {
    message: "Dados parciais",
    description:
      "Amber caveat label above the charges when the metering behind the estimate is degraded/incomplete (w4/048), mirroring the Metrics page's degraded badge",
  },
  "usage.coveragePartialLead": {
    message: "Esta estimativa pode subestimar o uso.",
    description: "Lead sentence of the partial-coverage caveat tooltip",
  },
  "usage.coveragePartialThrough": {
    message: "Medido até {through}.",
    description:
      "Partial-coverage caveat clause naming the date the meters have recorded through (a degraded hour counts as recorded, w4/496); {through} is a YYYY-MM-DD date",
  },
  "usage.coveragePartialSources": {
    message: "Fontes de medição degradadas: {sources}.",
    description:
      "Partial-coverage caveat clause listing the degraded metering source names; {sources} is a comma-separated list",
  },
  "usage.expandAll": {
    message: "Expandir tudo",
    description: "Charge-tree button that opens every category",
  },
  "usage.collapseAll": {
    message: "Recolher tudo",
    description: "Charge-tree button that closes every category",
  },
  "usage.totalToDate": {
    message: "Total do mês até agora",
    description: "Charge-tree total row for the month in progress",
  },
  "usage.totalForPeriod": {
    message: "Total do período",
    description: "Charge-tree total row for a completed past month",
  },
  "usage.totalBillingPeriodWindow": {
    message: "Total neste período de faturamento ({start} – {end})",
    description:
      "Charge total row when the headline is Stripe's rated amount: it covers the subscription period (anchored on the subscription's day of month), not the calendar month (w4/m139)",
  },
  "usage.totalBillingPeriod": {
    message: "Total neste período de faturamento",
    description:
      "Charge total row for Stripe's rated amount when its period bounds are unknown (w4/m139)",
  },
  "usage.breakdownMonthToDate": {
    message: "Detalhamento acima: {month} até agora",
    description:
      "Row under a Stripe-rated total: the charge tree's own sum and window (the calendar month), which differs from the subscription period (w4/m139)",
  },
  "usage.projectedTotal": {
    message: "Total projetado para {month}",
    description:
      "Charge-tree straight-line projection of month-end spend, for the estimate fallback whose total accrues over the calendar month",
  },
  "usage.projectedTotalBillingPeriod": {
    message: "Projetado para este período de faturamento",
    description:
      "Charge-tree straight-line projection of a Stripe-rated total. The subscription period need not align with the calendar month (it can span two), so no month is named (w6/050).",
  },
  "usage.chargeFree": {
    message: "Incluído",
    description: "Rate column value for a charge line that is priced at zero",
  },
  "usage.resourceDeleted": {
    message: "(excluído)",
    description:
      "Marker beside a charged resource that no longer exists; the name shown is the one bex retained",
  },
  "usage.categoryServices": {
    message: "Serviços",
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
    message: "Armazenamento de objetos",
    description:
      "Charge-tree category for a workspace's object storage (ADR112)",
  },
  "usage.categoryAgentSessions": {
    message: "Sessões de agente",
    description:
      "Charge-tree category for platform model usage of agent sessions",
  },
  "usage.chargeDisk": {
    message: "Disco",
    description: "Charge line for a service's provisioned persistent disk",
  },
  "usage.chargeAgentModelCost": {
    message: "Uso de modelo",
    description:
      "Charge line for platform model spend, billed at provider cost plus markup (ADR091)",
  },
  "usage.chargeAgentModelRate": {
    message: "Custo do provedor + 30%",
    description: "Rate column value for the platform model spend line",
  },
  "usage.chargeAgentModelInputTokens": {
    message: "Tokens de entrada",
    description: "Breakdown line for platform model input tokens",
  },
  "usage.chargeAgentModelOutputTokens": {
    message: "Tokens de saída",
    description: "Breakdown line for platform model output tokens",
  },
  "usage.chargeAgentModelCacheReadTokens": {
    message: "Tokens de leitura de cache",
    description: "Breakdown line for platform model cache-read tokens",
  },
  "usage.chargeAgentModelCacheWriteTokens": {
    message: "Tokens de gravação de cache",
    description: "Breakdown line for platform model cache-write tokens",
  },
  "usage.categorySandboxes": {
    message: "Sandboxes",
    description: "Charge-tree category for hosted agent sandboxes",
  },
  "usage.chargeCompute": {
    message: "Computação",
    description: "Charge line for metered instance time",
  },
  "usage.chargeBandwidth": {
    message: "Largura de banda",
    description: "Charge line for metered outbound bandwidth",
  },
  "usage.chargeBuild": {
    message: "Minutos de build",
    description: "Charge line for metered build time",
  },
  "usage.chargeStorage": {
    message: "Armazenamento",
    description: "Charge line for metered datastore storage",
  },
  "usage.chargeSandboxCompute": {
    message: "Computação de sandbox",
    description: "Charge line for metered sandbox compute",
  },
  "usage.chargeSandboxMemory": {
    message: "Memória de sandbox",
    description:
      "Charge line for the memory a running sandbox was allocated, billed per GiB-hour since billing switched to use",
  },
  "usage.chargeSandboxMemoryUnbilled": {
    message: "Memória de sandbox (incluída na computação)",
    description:
      "Charge line for the allocated memory a sandbox ran with before billing switched to use; recorded beside sandbox compute, which already includes it, so it is shown and not billed separately",
  },
  "usage.chargeSandboxCpu": {
    message: "CPU de sandbox",
    description:
      "Charge line for the CPU a running sandbox used, billed per CPU-hour used",
  },
  "usage.chargeSandboxCpuUnbilled": {
    message: "CPU de sandbox (incluída na computação)",
    description:
      "Charge line for the CPU a sandbox used before billing switched to use; recorded beside sandbox compute, which already charged for it, so it is shown and not billed separately",
  },
  "usage.sandboxUsageBilledFromNote": {
    message:
      "Desde {date} UTC, um sandbox em execução é cobrado pela CPU que usa e pela memória alocada. O uso anterior foi cobrado como computação de sandbox.",
    description:
      "Note under the charge tree of a period with sandbox usage; date is the instant billing switched, formatted in UTC",
  },
  "usage.chargeSandboxSnapshotStorage": {
    message: "Armazenamento de snapshots de sandbox",
    description:
      "Charge line for the image a suspended sandbox retains, billed per GB-month",
  },
  "usage.chargeObjectStorage": {
    message: "Armazenamento",
    description:
      "Charge line for the data a workspace keeps in object storage, billed per GB-month (shown at no charge until a price is set, ADR112 D8)",
  },
  "usage.chargeObjectStorageIngress": {
    message: "Largura de banda de entrada",
    description:
      "Charge line for the bytes put into a workspace's object storage (shown at no charge, ADR112 D8)",
  },
  "usage.chargeObjectStorageEgress": {
    message: "Largura de banda de saída",
    description:
      "Charge line for the bytes read out of a workspace's object storage, public reads included (shown at no charge, ADR112 D8)",
  },
  "usage.creditsTotalLabel": {
    message: "Saldo total",
    description: "Label above the credit balance amount",
  },
  "usage.invoiceHistoryTitle": {
    message: "Histórico de faturas",
    description: "Invoice-history card title on the billing page",
  },
  "usage.invoiceHistoryDescription": {
    message: "Suas faturas finalizadas.",
    description: "Invoice-history card description",
  },
  "usage.invoiceKindAgentModelCharge": {
    message: "Cobrança de modelo de agente",
    description:
      "Label on an invoice-history row for a charge made when agent model spend reached its threshold",
  },
  "usage.invoiceHistoryChargesNote": {
    message:
      "As cobranças de modelo de agente feitas durante um mês são deduzidas da fatura desse mês.",
    description:
      "Invoice-history note when the list includes agent model charges",
  },
  "usage.creditsAppliedLine": {
    message: "Créditos aplicados −{applied} → valor a pagar {due}",
    description:
      "Credit applied to the current period and the remaining amount due",
  },
  "usage.chargesDescriptionPending": {
    message:
      "Acumulado até agora neste período, calculado pela tabela de preços do bex.",
    description:
      "Charges card description while the invoiced total is still loading; deliberately states only what is already true, without claiming the figure is or is not a Stripe invoice.",
  },
  "usage.paymentMethodRejectedPrepaid": {
    message:
      "Cartões pré-pagos não são aceitos; adicione um cartão de crédito ou débito.",
    description:
      "Shown when the last card offered was refused at binding because it is prepaid (ADR091 D10.5)",
  },
  "usage.paymentMethodRejectedUnpaidBalance": {
    message:
      "Este cartão está vinculado a um saldo não pago; adicione outro cartão.",
    description:
      "Shown when the last card offered was refused because it carries an unpaid written-off balance (ADR091 D10.6)",
  },
  "usage.agentModelTitle": {
    message: "Modelo de agente",
    description:
      "Billing page card and nav entry for agent model spend (ADR091 D6)",
  },
  "usage.agentModelPricing": {
    message: "Cobrado ao custo do provedor do modelo + 30%.",
    description: "Pricing statement under the Agent model title",
  },
  "usage.agentModelMonthToDate": {
    message: "Mês até agora",
    description: "Label of the current month's agent model total",
  },
  "usage.agentModelPeriodTotal": {
    message: "Total do mês",
    description: "Label of a closed month's agent model total",
  },
  "usage.agentModelBreakdown_one": {
    message: "Detalhamento por modelo ({count} modelo)",
    description: "Toggle that expands the per-model breakdown",
  },
  "usage.agentModelBreakdown_many": {
    message: "Detalhamento por modelo ({count} de modelos)",
    description: "Toggle that expands the per-model breakdown",
  },
  "usage.agentModelBreakdown_other": {
    message: "Detalhamento por modelo ({count} modelos)",
    description: "Toggle that expands the per-model breakdown",
  },
  "usage.agentModelEmpty": {
    message: "Nenhum uso de modelo de agente neste período.",
    description: "Shown when the period has no agent model spend",
  },
  "usage.lessThanAmount": {
    message: "<{amount}",
    description:
      "A charge that rounds below the smallest amount shown, e.g. <$0.01",
  },
};

export default ptUsage;
