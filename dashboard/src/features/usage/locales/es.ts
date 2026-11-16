import type { TranslationEntry } from "@/i18n";

const esUsage: Record<string, TranslationEntry> = {
  "usage.pageTitle": {
    message: "Facturación",
    description:
      "Billing page heading and browser title (renamed from Usage in w5/m70)",
  },
  "usage.pageSubtitle": {
    message:
      "Pago, facturas y consumo del espacio de trabajo en lo que va de mes",
    description: "Billing page subtitle beneath the heading",
  },
  "usage.sectionNavigation": {
    message: "Secciones de facturación",
    description: "Accessible label for the billing page section navigation",
  },
  "usage.resourceCapsServices": {
    message: "Servicios",
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
    message: "Gratis: {used} de {limit}",
    description: "Free-instance count in the resource-cap card (w4/m241)",
  },
  "usage.freeHoursTitle": {
    message: "Uso mensual incluido",
    description:
      "Title of the Free instance hours card on the usage page (w8/m139)",
  },
  "usage.freeHoursDescription": {
    message:
      "Los Web Services gratuitos comparten una asignación mensual de horas de instancia por espacio de trabajo. El tiempo en reposo no cuenta.",
    description: "Explains the monthly Free instance hours allowance (w8/m139)",
  },
  "usage.freeHoursUsed": {
    message:
      "{used} de {included} horas de instancia gratuitas usadas este mes",
    description:
      "Free instance hours used against the monthly allowance (w8/m139)",
  },
  "usage.freeHoursNear": {
    message:
      "Estás cerca de la asignación. Cuando se agote, tus Web Services gratuitos se suspenden hasta el inicio del próximo mes.",
    description:
      "Warning once Free instance hours reach 80 percent of the allowance (w8/m139)",
  },
  "usage.freeHoursSpent": {
    message:
      "La asignación se ha agotado. Los Web Services gratuitos están suspendidos hasta el inicio del próximo mes; mejora un servicio a un plan de pago para ejecutarlo ahora.",
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
      "Los builds y los comandos previos al despliegue consumen minutos de pipeline. Tu plan incluye algunos cada mes; los minutos adicionales se facturan por minuto.",
    description: "Explains the plan's included build minutes (w6/m308)",
  },
  "usage.buildMinutesUsed": {
    message: "{used} de {included} minutos de build incluidos usados este mes",
    description:
      "Build minutes used against the plan's included minutes (w6/m308)",
  },
  "usage.buildMinutesNear": {
    message:
      "Estás cerca de tus minutos de build incluidos. Sin un método de pago, los nuevos builds y comandos previos al despliegue se detienen cuando se agoten, hasta el inicio del próximo mes.",
    description:
      "Warning once build minutes reach 80 percent of the included minutes (w6/m308)",
  },
  "usage.buildMinutesSpent": {
    message:
      "Los minutos de build incluidos se han agotado. Los minutos adicionales se facturan por minuto; sin un método de pago, los nuevos builds y comandos previos al despliegue se detienen hasta el inicio del próximo mes.",
    description: "Notice once the included build minutes are spent (w6/m308)",
  },
  "usage.bandwidthTitle": {
    message: "Ancho de banda de salida",
    description:
      "Title of the included bandwidth card on the billing page (w6/m319)",
  },
  "usage.bandwidthDescription": {
    message:
      "El tráfico que tus servicios envían a internet consume ancho de banda. Tu plan incluye algo cada mes; el ancho de banda adicional se factura por GB.",
    description: "Explains the plan's included outbound bandwidth (w6/m319)",
  },
  "usage.bandwidthUsed": {
    message:
      "{used} de {included} GB de ancho de banda incluido usados este mes",
    description:
      "Outbound bandwidth used against the plan's included bandwidth, in GB (w6/m319)",
  },
  "usage.bandwidthNear": {
    message:
      "Estás cerca de tu ancho de banda incluido. Sin un método de pago, los servicios de tu espacio de trabajo pasan a reposo cuando se agote, hasta el inicio del próximo mes.",
    description:
      "Warning once bandwidth reaches 80 percent of the included bandwidth (w6/m319)",
  },
  "usage.bandwidthSpent": {
    message:
      "El ancho de banda incluido se ha agotado. El ancho de banda adicional se factura por GB; sin un método de pago, los servicios de tu espacio de trabajo pasan a reposo hasta el inicio del próximo mes.",
    description: "Notice once the included bandwidth is spent (w6/m319)",
  },
  "usage.chargeIncluded": {
    message: "{included} incluidos",
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
      "Los builds y los comandos previos al despliegue consumen minutos de pipeline. Los minutos que superan los incluidos en tu plan se facturan por minuto, hasta el límite de gasto mensual que definas. Cuando se alcanza el límite, los builds se detienen hasta el próximo mes.",
    description: "Explains the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitValue": {
    message: "Límite de gasto: {limit} al mes",
    description: "The workspace pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitNone": {
    message: "Sin límite de gasto",
    description: "The workspace has no pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitSet": {
    message: "Definir límite de gasto",
    description: "Button that opens the pipeline spend limit dialog (w6/m309)",
  },
  "usage.pipelineSpendLimitEdit": {
    message: "Editar",
    description: "Button that edits the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitRemove": {
    message: "Quitar límite",
    description: "Button that removes the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitDialogTitle": {
    message: "Definir límite de gasto",
    description: "Title of the pipeline spend limit dialog (w6/m309)",
  },
  "usage.pipelineSpendLimitDialogDescription": {
    message:
      "Lo máximo que este espacio de trabajo gasta al mes en minutos de build que superan los incluidos en su plan. Cuando se alcanza, los nuevos builds y comandos previos al despliegue se detienen hasta el inicio del próximo mes.",
    description: "Explains what the pipeline spend limit caps (w6/m309)",
  },
  "usage.pipelineSpendLimitLabel": {
    message: "Límite mensual (USD)",
    description: "Label of the pipeline spend limit input (w6/m309)",
  },
  "usage.pipelineSpendLimitSave": {
    message: "Guardar",
    description: "Saves the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitSaved": {
    message: "Límite de gasto definido en {limit} al mes",
    description: "Toast after setting the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitRemoved": {
    message: "Límite de gasto quitado",
    description: "Toast after removing the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitError": {
    message: "No se pudo actualizar el límite de gasto",
    description: "Toast when updating the pipeline spend limit fails (w6/m309)",
  },
  "usage.pipelineSpendLimitUnreadable": {
    message:
      "No se pudo cargar el límite de gasto. Recarga para intentarlo de nuevo.",
    description: "The pipeline spend limit could not be read (w6/m309)",
  },
  "usage.pipelineSpendLimitForbidden": {
    message:
      "Solo los gestores de facturación pueden ver y cambiar el límite de gasto.",
    description: "The caller may not read the pipeline spend limit (w6/m309)",
  },
  "usage.resourceCapsNearLimit": {
    message: "Cerca del límite",
    description:
      "Warning shown once a resource count reaches 80 percent of its cap, below the cap itself",
  },
  "usage.resourceCapsLimitReached": {
    message: "Límite alcanzado",
    description:
      "Shown when a resource count is at or over its cap, so creating another is refused",
  },
  "usage.resourceCapsFinishingDeletion": {
    message: "{count} terminando de eliminarse",
    description:
      "Sub-line on a resource-cap tile: how many of the counted resources are mid-deletion and thus hidden from the resource list but still holding quota (w6/m129)",
  },
  "usage.resourceCapsFinishingDeletionHint": {
    message:
      "Sigue contando para tu límite hasta que se complete la eliminación",
    description:
      "Tooltip explaining why resources finishing deletion are included in the used count",
  },
  "usage.errorTitle": {
    message: "No se pudo cargar el uso",
    description: "Error state heading on the Usage page",
  },
  "usage.monthPickerLabel": {
    message: "Seleccionar mes",
    description: "Aria-label for the month-picker select on the Usage page",
  },
  "usage.currentMonth": {
    message: "Mes actual",
    description:
      "Default option in the month picker meaning the current calendar month",
  },
  "usage.creditsTitle": {
    message: "Crédito restante",
    description: "Heading of the remaining billing-credit card (w5/m70)",
  },
  "usage.creditsDescription": {
    message:
      "Crédito promocional que se aplica a las facturas antes de cobrar tu método de pago",
    description: "Subtitle of the remaining billing-credit card",
  },
  "usage.creditsExpiryNote": {
    message: "{amount} caduca el {date}",
    description:
      "Earliest-expiring grant note beside the credit balance; amount is a $ value, date is YYYY-MM-DD",
  },
  "usage.creditsCardStillRequired": {
    message:
      "Sigue siendo necesario un método de pago aunque tengas crédito: el crédito cubre primero las facturas y tu tarjeta paga el resto.",
    description:
      "ADR046 clarification on the credit card — credit does not replace payment onboarding",
  },
  "usage.colInvoicePeriod": {
    message: "Periodo",
    description: "Table column header for an invoice's billing period start",
  },
  "usage.colInvoiceStatus": {
    message: "Estado",
    description: "Table column header for an invoice's status",
  },
  "usage.colInvoiceAmount": {
    message: "Importe",
    description: "Table column header for an invoice's amount",
  },
  "usage.billingSetupTitle": {
    message: "Configuración de facturación",
    description: "Customer billing onboarding card title",
  },
  "usage.billingSetupDescription": {
    message: "Cobro de pagos y preparación fiscal de este espacio de trabajo",
    description: "Customer billing onboarding card description",
  },
  "usage.billingTestMode": {
    message: "Stripe en modo de prueba",
    description: "Badge making the non-live Stripe environment explicit",
  },
  "usage.billingMode": {
    message: "Stripe Billing",
    description: "Fallback badge for the Stripe billing environment",
  },
  "usage.billingReady": {
    message: "Listo",
    description: "Billing onboarding status is ready",
  },
  "usage.billingActionNeeded": {
    message: "Se requiere acción",
    description: "Billing onboarding status needs action",
  },
  "usage.billingCustomerStatus": {
    message: "Cliente",
    description: "Stripe Customer readiness row",
  },
  "usage.billingSubscriptionStatus": {
    message: "Suscripción medida",
    description: "Stripe Subscription readiness row",
  },
  "usage.billingPaymentStatus": {
    message: "Método de pago",
    description: "Default payment method readiness row",
  },
  "usage.billingTaxStatus": {
    message: "Impuestos automáticos",
    description: "Stripe Tax activation row",
  },
  "usage.billingLifecycleStatus": {
    message: "Ciclo de cobro",
    description: "Stripe collection and reversible enforcement state row",
  },
  "usage.billingLifecycleGrace": {
    message:
      "El pago falló ({reason}). El espacio de trabajo sigue disponible durante el periodo de gracia; la suspensión reversible está programada después de {deadline}.",
    description: "Visible billing grace state and authoritative deadline",
  },
  "usage.billingLifecycleEnforced": {
    message:
      "La restricción por facturación está activa ({reason}). El cómputo está suspendido, pero las bases de datos, los datos de Key Value, los secretos y la evidencia de facturación no se han eliminado.",
    description: "Visible reversible billing enforcement state",
  },
  "usage.billingLifecycleRecovering": {
    message:
      "Pago recuperado. bex está restaurando solo los recursos modificados por la restricción de facturación.",
    description: "Visible precise recovery state",
  },
  "usage.billingLifecycleExcluded": {
    message:
      "Un operador ha excluido este espacio de trabajo del cobro de Stripe.",
    description: "Visible structural billing exclusion state",
  },
  "usage.billingLifecycleComped": {
    message:
      "Este espacio de trabajo se tarifica, pero un operador lo ha dejado totalmente gratuito.",
    description: "Visible rated-but-free comp state",
  },
  "usage.billingLifecycleUnknown": {
    message:
      "El estado de facturación es {reason}. Usa el portal de facturación o contacta con soporte.",
    description: "Forward-compatible unknown billing lifecycle state",
  },
  "usage.billingNoDeadline": {
    message: "no se informó ningún plazo",
    description: "Fallback when a malformed grace state has no deadline",
  },
  "usage.billingOff": {
    message: "Desactivado",
    description:
      "Neutral status for a deliberately disabled billing capability (e.g. tax not activated)",
  },
  "usage.billingAddPayment": {
    message: "Añadir método de pago",
    description: "Button opening setup-mode Stripe Checkout (live mode)",
  },
  "usage.billingUpdatePayment": {
    message: "Actualizar método de pago",
    description: "Button reopening setup-mode Stripe Checkout (live mode)",
  },
  "usage.billingAddPaymentTest": {
    message: "Añadir método de pago de prueba",
    description:
      "Button opening setup-mode Stripe Checkout when Stripe is in test mode",
  },
  "usage.billingUpdatePaymentTest": {
    message: "Actualizar método de pago de prueba",
    description:
      "Button reopening setup-mode Stripe Checkout when Stripe is in test mode",
  },
  "usage.billingOpenPortal": {
    message: "Abrir portal de facturación",
    description: "Button opening Stripe Customer Portal",
  },
  "usage.billingHostedNote": {
    message:
      "Los datos de pago se introducen solo en páginas alojadas por Stripe. bex nunca envía una clave de servidor de Stripe a tu navegador.",
    description: "Hosted billing security note",
  },
  "usage.billingUnavailable": {
    message:
      "La configuración de facturación no está disponible o no tienes acceso a la facturación (rol de facturación o administrador).",
    description: "Degraded or unauthorized billing onboarding state",
  },
  "usage.billingCheckoutError": {
    message: "No se pudo abrir Stripe Checkout. Inténtalo de nuevo.",
    description: "Toast after Checkout session creation fails",
  },
  "usage.billingPortalError": {
    message:
      "No se pudo abrir el portal de facturación de Stripe. Inténtalo de nuevo.",
    description: "Toast after Portal session creation fails",
  },
  "usage.paymentRequiredTitle": {
    message: "Añade un método de pago para continuar",
    description: "Just-in-time paid-intent onboarding dialog title",
  },
  "usage.paymentRequiredDescription": {
    message:
      "Este plan de pago necesita un método de pago. Completa Stripe Checkout en la nueva pestaña; esta acción se reanudará automáticamente cuando esté listo.",
    description: "Just-in-time paid-intent onboarding explanation",
  },
  "usage.paymentRequiredRetrying": {
    message: "Método de pago encontrado. Reintentando tu acción…",
    description: "Status while resuming the interrupted paid mutation",
  },
  "usage.paymentRequiredCancel": {
    message: "Cancelar",
    description: "Cancel just-in-time payment onboarding",
  },
  "usage.planTitle": {
    message: "Plan",
    description: "Plan card title on the billing page",
  },
  "usage.planChange": {
    message: "Cambiar de plan",
    description:
      "Link from the billing page's plan card to the workspace-settings plan dialog",
  },
  "usage.paymentMethodTitle": {
    message: "Método de pago",
    description: "Payment-method card title on the billing page",
  },
  "usage.paymentMethodCard": {
    message: "{brand} terminada en {last4}",
    description: "Names the card on file, e.g. 'Visa ending in 4242'",
  },
  "usage.paymentMethodOnFile": {
    message: "Método de pago registrado",
    description:
      "Shown when a payment method exists but is not a card the provider could name",
  },
  "usage.paymentMethodNone": {
    message: "Sin método de pago",
    description: "Shown when the workspace has no payment method",
  },
  "usage.includedUsageTitle": {
    message: "Uso incluido",
    description:
      "Plan-allowance card title on the billing page (was 'Resource limits')",
  },
  "usage.includedUsageDescription": {
    message:
      "Lo que incluye el plan de este espacio de trabajo y cuánto se está usando.",
    description: "Included-usage card description",
  },
  "usage.chargesTitle": {
    message: "Cargos",
    description: "Charge-tree card title on the billing page",
  },
  "usage.chargesDescriptionEstimate": {
    message:
      "Acumulado en lo que va de este periodo, con precios del tarifario de bex. Es una estimación, no una factura.",
    description:
      "Charge-tree description when no Stripe subscription prices the period",
  },
  "usage.chargesDescriptionInvoiced": {
    message:
      "Acumulado en lo que va de este periodo, según la tarificación de Stripe.",
    description:
      "Charge-tree description when a real Stripe amount is available. Says the total is Stripe's rating rather than the amount it will invoice: credits and comp discounts can sit between the two, and the amount actually due gets its own line (w6/m98).",
  },
  "usage.amountDueAfterCredits": {
    message: "Importe a pagar tras créditos",
    description:
      "Label for the charge-tree line showing what Stripe actually collects once credits and discounts are applied to the charge above it",
  },
  "usage.chargesEmpty": {
    message: "No hay uso en este periodo.",
    description: "Charge-tree empty state",
  },
  "usage.coveragePartial": {
    message: "Datos parciales",
    description:
      "Amber caveat label above the charges when the metering behind the estimate is degraded/incomplete (w4/048), mirroring the Metrics page's degraded badge",
  },
  "usage.coveragePartialLead": {
    message: "Esta estimación puede subestimar el uso.",
    description: "Lead sentence of the partial-coverage caveat tooltip",
  },
  "usage.coveragePartialThrough": {
    message: "Medido hasta {through}.",
    description:
      "Partial-coverage caveat clause naming the date the meters have recorded through (a degraded hour counts as recorded, w4/496); {through} is a YYYY-MM-DD date",
  },
  "usage.coveragePartialSources": {
    message: "Fuentes de medición degradadas: {sources}.",
    description:
      "Partial-coverage caveat clause listing the degraded metering source names; {sources} is a comma-separated list",
  },
  "usage.expandAll": {
    message: "Expandir todo",
    description: "Charge-tree button that opens every category",
  },
  "usage.collapseAll": {
    message: "Contraer todo",
    description: "Charge-tree button that closes every category",
  },
  "usage.totalToDate": {
    message: "Total del mes hasta la fecha",
    description: "Charge-tree total row for the month in progress",
  },
  "usage.totalForPeriod": {
    message: "Total del periodo",
    description: "Charge-tree total row for a completed past month",
  },
  "usage.totalBillingPeriodWindow": {
    message: "Total de este periodo de facturación ({start} – {end})",
    description:
      "Charge total row when the headline is Stripe's rated amount: it covers the subscription period (anchored on the subscription's day of month), not the calendar month (w4/m139)",
  },
  "usage.totalBillingPeriod": {
    message: "Total de este periodo de facturación",
    description:
      "Charge total row for Stripe's rated amount when its period bounds are unknown (w4/m139)",
  },
  "usage.breakdownMonthToDate": {
    message: "Desglose anterior: {month} hasta la fecha",
    description:
      "Row under a Stripe-rated total: the charge tree's own sum and window (the calendar month), which differs from the subscription period (w4/m139)",
  },
  "usage.projectedTotal": {
    message: "Total proyectado para {month}",
    description:
      "Charge-tree straight-line projection of month-end spend, for the estimate fallback whose total accrues over the calendar month",
  },
  "usage.projectedTotalBillingPeriod": {
    message: "Proyectado para este periodo de facturación",
    description:
      "Charge-tree straight-line projection of a Stripe-rated total. The subscription period need not align with the calendar month (it can span two), so no month is named (w6/050).",
  },
  "usage.chargeFree": {
    message: "Incluido",
    description: "Rate column value for a charge line that is priced at zero",
  },
  "usage.resourceDeleted": {
    message: "(eliminado)",
    description:
      "Marker beside a charged resource that no longer exists; the name shown is the one bex retained",
  },
  "usage.categoryServices": {
    message: "Servicios",
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
    message: "Almacenamiento de objetos",
    description:
      "Charge-tree category for a workspace's object storage (ADR112)",
  },
  "usage.categoryAgentSessions": {
    message: "Sesiones de agente",
    description:
      "Charge-tree category for platform model usage of agent sessions",
  },
  "usage.chargeDisk": {
    message: "Disco",
    description: "Charge line for a service's provisioned persistent disk",
  },
  "usage.chargeAgentModelCost": {
    message: "Uso de modelos",
    description:
      "Charge line for platform model spend, billed at provider cost plus markup (ADR091)",
  },
  "usage.chargeAgentModelRate": {
    message: "Coste del proveedor + 30%",
    description: "Rate column value for the platform model spend line",
  },
  "usage.chargeAgentModelInputTokens": {
    message: "Tokens de entrada",
    description: "Breakdown line for platform model input tokens",
  },
  "usage.chargeAgentModelOutputTokens": {
    message: "Tokens de salida",
    description: "Breakdown line for platform model output tokens",
  },
  "usage.chargeAgentModelCacheReadTokens": {
    message: "Tokens de lectura de caché",
    description: "Breakdown line for platform model cache-read tokens",
  },
  "usage.chargeAgentModelCacheWriteTokens": {
    message: "Tokens de escritura de caché",
    description: "Breakdown line for platform model cache-write tokens",
  },
  "usage.categorySandboxes": {
    message: "Sandboxes",
    description: "Charge-tree category for hosted agent sandboxes",
  },
  "usage.chargeCompute": {
    message: "Cómputo",
    description: "Charge line for metered instance time",
  },
  "usage.chargeBandwidth": {
    message: "Ancho de banda",
    description: "Charge line for metered outbound bandwidth",
  },
  "usage.chargeBuild": {
    message: "Minutos de build",
    description: "Charge line for metered build time",
  },
  "usage.chargeStorage": {
    message: "Almacenamiento",
    description: "Charge line for metered datastore storage",
  },
  "usage.chargeSandboxCompute": {
    message: "Cómputo de sandbox",
    description: "Charge line for metered sandbox compute",
  },
  "usage.chargeSandboxMemory": {
    message: "Memoria de sandbox (incluida en el cómputo)",
    description:
      "Charge line for the allocated memory a sandbox ran with; recorded beside sandbox compute, which already includes it, so it is shown and not billed separately",
  },
  "usage.chargeSandboxSnapshotStorage": {
    message: "Almacenamiento de snapshots de sandbox",
    description:
      "Charge line for the image a suspended sandbox retains, billed per GB-month",
  },
  "usage.chargeObjectStorage": {
    message: "Almacenamiento",
    description:
      "Charge line for the data a workspace keeps in object storage, billed per GB-month (shown at no charge until a price is set, ADR112 D8)",
  },
  "usage.chargeObjectStorageIngress": {
    message: "Ancho de banda de entrada",
    description:
      "Charge line for the bytes put into a workspace's object storage (shown at no charge, ADR112 D8)",
  },
  "usage.chargeObjectStorageEgress": {
    message: "Ancho de banda de salida",
    description:
      "Charge line for the bytes read out of a workspace's object storage, public reads included (shown at no charge, ADR112 D8)",
  },
  "usage.creditsTotalLabel": {
    message: "Saldo total",
    description: "Label above the credit balance amount",
  },
  "usage.invoiceHistoryTitle": {
    message: "Historial de facturas",
    description: "Invoice-history card title on the billing page",
  },
  "usage.invoiceHistoryDescription": {
    message: "Tus facturas finalizadas.",
    description: "Invoice-history card description",
  },
  "usage.invoiceKindAgentModelCharge": {
    message: "Cargo por modelo de agente",
    description:
      "Label on an invoice-history row for a charge made when agent model spend reached its threshold",
  },
  "usage.invoiceHistoryChargesNote": {
    message:
      "Los cargos por modelo de agente realizados durante un mes se descuentan de la factura de ese mes.",
    description:
      "Invoice-history note when the list includes agent model charges",
  },
  "usage.creditsAppliedLine": {
    message: "Créditos aplicados −{applied} → importe a pagar {due}",
    description:
      "Credit applied to the current period and the remaining amount due",
  },
  "usage.chargesDescriptionPending": {
    message:
      "Acumulado en lo que va de este periodo, con precios del tarifario de bex.",
    description:
      "Charges card description while the invoiced total is still loading; deliberately states only what is already true, without claiming the figure is or is not a Stripe invoice.",
  },
  "usage.paymentMethodRejectedPrepaid": {
    message:
      "No se aceptan tarjetas prepago; añade una tarjeta de crédito o débito.",
    description:
      "Shown when the last card offered was refused at binding because it is prepaid (ADR091 D10.5)",
  },
  "usage.paymentMethodRejectedUnpaidBalance": {
    message:
      "Esta tarjeta está vinculada a un saldo impagado; añade otra tarjeta.",
    description:
      "Shown when the last card offered was refused because it carries an unpaid written-off balance (ADR091 D10.6)",
  },
  "usage.agentModelTitle": {
    message: "Modelo de agente",
    description:
      "Billing page card and nav entry for agent model spend (ADR091 D6)",
  },
  "usage.agentModelPricing": {
    message: "Se factura al coste del proveedor del modelo + 30%.",
    description: "Pricing statement under the Agent model title",
  },
  "usage.agentModelMonthToDate": {
    message: "Mes hasta la fecha",
    description: "Label of the current month's agent model total",
  },
  "usage.agentModelPeriodTotal": {
    message: "Total del mes",
    description: "Label of a closed month's agent model total",
  },
  "usage.agentModelBreakdown_one": {
    message: "Desglose por modelo ({count} modelo)",
    description: "Toggle that expands the per-model breakdown",
  },
  "usage.agentModelBreakdown_many": {
    message: "Desglose por modelo ({count} de modelos)",
    description: "Toggle that expands the per-model breakdown",
  },
  "usage.agentModelBreakdown_other": {
    message: "Desglose por modelo ({count} modelos)",
    description: "Toggle that expands the per-model breakdown",
  },
  "usage.agentModelEmpty": {
    message: "No hay uso de modelos de agente en este periodo.",
    description: "Shown when the period has no agent model spend",
  },
  "usage.lessThanAmount": {
    message: "<{amount}",
    description:
      "A charge that rounds below the smallest amount shown, e.g. <$0.01",
  },
};

export default esUsage;
