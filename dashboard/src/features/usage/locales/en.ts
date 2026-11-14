import type { TranslationEntry } from "@/i18n";

const enUsage: Record<string, TranslationEntry> = {
  "usage.pageTitle": {
    message: "Billing",
    description:
      "Billing page heading and browser title (renamed from Usage in w5/m70)",
  },
  "usage.pageSubtitle": {
    message: "Payment, invoices, and month-to-date workspace consumption",
    description: "Billing page subtitle beneath the heading",
  },
  "usage.sectionNavigation": {
    message: "Billing sections",
    description: "Accessible label for the billing page section navigation",
  },
  "usage.resourceCapsServices": {
    message: "Services",
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
    message: "Free: {used} of {limit}",
    description: "Free-instance count in the resource-cap card (w4/m241)",
  },
  "usage.freeHoursTitle": {
    message: "Monthly included usage",
    description:
      "Title of the Free instance hours card on the usage page (w8/m139)",
  },
  "usage.freeHoursDescription": {
    message:
      "Free web services share a monthly allowance of instance hours per workspace. Time spent spun down does not count.",
    description: "Explains the monthly Free instance hours allowance (w8/m139)",
  },
  "usage.freeHoursUsed": {
    message: "{used} of {included} Free instance hours used this month",
    description:
      "Free instance hours used against the monthly allowance (w8/m139)",
  },
  "usage.freeHoursNear": {
    message:
      "You are close to the allowance. When it is spent, your Free web services are suspended until the start of next month.",
    description:
      "Warning once Free instance hours reach 80 percent of the allowance (w8/m139)",
  },
  "usage.freeHoursSpent": {
    message:
      "The allowance is spent. Free web services are suspended until the start of next month; upgrade a service to a paid plan to run it now.",
    description:
      "Notice once the monthly Free instance hours are spent (w8/m139)",
  },
  "usage.buildMinutesTitle": {
    message: "Pipeline minutes",
    description:
      "Title of the included build minutes card on the billing page (w6/m308)",
  },
  "usage.buildMinutesDescription": {
    message:
      "Builds and pre-deploy commands use pipeline minutes. Your plan includes some every month; minutes beyond them are billed per minute.",
    description: "Explains the plan's included build minutes (w6/m308)",
  },
  "usage.buildMinutesUsed": {
    message: "{used} of {included} included build minutes used this month",
    description:
      "Build minutes used against the plan's included minutes (w6/m308)",
  },
  "usage.buildMinutesNear": {
    message:
      "You are close to your included build minutes. Without a payment method, new builds and pre-deploy commands stop when they run out, until the start of next month.",
    description:
      "Warning once build minutes reach 80 percent of the included minutes (w6/m308)",
  },
  "usage.buildMinutesSpent": {
    message:
      "The included build minutes are spent. Further minutes are billed per minute; without a payment method, new builds and pre-deploy commands stop until the start of next month.",
    description: "Notice once the included build minutes are spent (w6/m308)",
  },
  "usage.bandwidthTitle": {
    message: "Outbound bandwidth",
    description:
      "Title of the included bandwidth card on the billing page (w6/m319)",
  },
  "usage.bandwidthDescription": {
    message:
      "Traffic your services send to the internet uses bandwidth. Your plan includes some every month; bandwidth beyond it is billed per GB.",
    description: "Explains the plan's included outbound bandwidth (w6/m319)",
  },
  "usage.bandwidthUsed": {
    message: "{used} of {included} GB included bandwidth used this month",
    description:
      "Outbound bandwidth used against the plan's included bandwidth, in GB (w6/m319)",
  },
  "usage.bandwidthNear": {
    message:
      "You are close to your included bandwidth. Without a payment method, your workspace's services spin down when it runs out, until the start of next month.",
    description:
      "Warning once bandwidth reaches 80 percent of the included bandwidth (w6/m319)",
  },
  "usage.bandwidthSpent": {
    message:
      "The included bandwidth is spent. Further bandwidth is billed per GB; without a payment method, your workspace's services are spun down until the start of next month.",
    description: "Notice once the included bandwidth is spent (w6/m319)",
  },
  "usage.chargeIncluded": {
    message: "{included} included",
    description:
      "Part of a charge line's quantity that the plan's included build minutes or bandwidth covered (w6/m308, w6/m319)",
  },
  "usage.pipelineTitle": {
    message: "Build Pipeline",
    description:
      "Title of the Build Pipeline card in workspace settings (w6/m309)",
  },
  "usage.pipelineDescription": {
    message:
      "Builds and pre-deploy commands use pipeline minutes. Minutes beyond your plan's included ones are billed per minute, up to the monthly spend limit you set. When the limit is reached, builds stop until next month.",
    description: "Explains the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitValue": {
    message: "Spend limit: {limit} a month",
    description: "The workspace pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitNone": {
    message: "No spend limit",
    description: "The workspace has no pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitSet": {
    message: "Set spend limit",
    description: "Button that opens the pipeline spend limit dialog (w6/m309)",
  },
  "usage.pipelineSpendLimitEdit": {
    message: "Edit",
    description: "Button that edits the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitRemove": {
    message: "Remove limit",
    description: "Button that removes the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitDialogTitle": {
    message: "Set spend limit",
    description: "Title of the pipeline spend limit dialog (w6/m309)",
  },
  "usage.pipelineSpendLimitDialogDescription": {
    message:
      "The most this workspace spends a month on build minutes beyond its plan's included ones. When it is reached, new builds and pre-deploy commands stop until the start of next month.",
    description: "Explains what the pipeline spend limit caps (w6/m309)",
  },
  "usage.pipelineSpendLimitLabel": {
    message: "Monthly limit (USD)",
    description: "Label of the pipeline spend limit input (w6/m309)",
  },
  "usage.pipelineSpendLimitSave": {
    message: "Save",
    description: "Saves the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitSaved": {
    message: "Spend limit set to {limit} a month",
    description: "Toast after setting the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitRemoved": {
    message: "Spend limit removed",
    description: "Toast after removing the pipeline spend limit (w6/m309)",
  },
  "usage.pipelineSpendLimitError": {
    message: "Couldn't update the spend limit",
    description: "Toast when updating the pipeline spend limit fails (w6/m309)",
  },
  "usage.pipelineSpendLimitUnreadable": {
    message: "Couldn't load the spend limit. Reload to try again.",
    description: "The pipeline spend limit could not be read (w6/m309)",
  },
  "usage.pipelineSpendLimitForbidden": {
    message: "Only billing managers can see and change the spend limit.",
    description: "The caller may not read the pipeline spend limit (w6/m309)",
  },
  "usage.resourceCapsNearLimit": {
    message: "Near limit",
    description:
      "Warning shown once a resource count reaches 80 percent of its cap, below the cap itself",
  },
  "usage.resourceCapsLimitReached": {
    message: "Limit reached",
    description:
      "Shown when a resource count is at or over its cap, so creating another is refused",
  },
  "usage.resourceCapsFinishingDeletion": {
    message: "{count} finishing deletion",
    description:
      "Sub-line on a resource-cap tile: how many of the counted resources are mid-deletion and thus hidden from the resource list but still holding quota (w6/m129)",
  },
  "usage.resourceCapsFinishingDeletionHint": {
    message: "Still counts toward your limit until deletion completes",
    description:
      "Tooltip explaining why resources finishing deletion are included in the used count",
  },
  "usage.errorTitle": {
    message: "Could not load usage",
    description: "Error state heading on the Usage page",
  },
  "usage.monthPickerLabel": {
    message: "Select month",
    description: "Aria-label for the month-picker select on the Usage page",
  },
  "usage.currentMonth": {
    message: "Current month",
    description:
      "Default option in the month picker meaning the current calendar month",
  },
  "usage.creditsTitle": {
    message: "Credits remaining",
    description: "Heading of the remaining billing-credit card (w5/m70)",
  },
  "usage.creditsDescription": {
    message:
      "Promotional credit applied to invoices before your payment method is charged",
    description: "Subtitle of the remaining billing-credit card",
  },
  "usage.creditsExpiryNote": {
    message: "{amount} of it expires {date}",
    description:
      "Earliest-expiring grant note beside the credit balance; amount is a $ value, date is YYYY-MM-DD",
  },
  "usage.creditsCardStillRequired": {
    message:
      "A payment method is still required even with credit: credit covers invoices first, and your card pays any remainder.",
    description:
      "ADR046 clarification on the credit card — credit does not replace payment onboarding",
  },
  "usage.colInvoicePeriod": {
    message: "Period",
    description: "Table column header for an invoice's billing period start",
  },
  "usage.colInvoiceStatus": {
    message: "Status",
    description: "Table column header for an invoice's status",
  },
  "usage.colInvoiceAmount": {
    message: "Amount",
    description: "Table column header for an invoice's amount",
  },
  "usage.billingSetupTitle": {
    message: "Billing setup",
    description: "Customer billing onboarding card title",
  },
  "usage.billingSetupDescription": {
    message: "Payment collection and tax readiness for this workspace",
    description: "Customer billing onboarding card description",
  },
  "usage.billingTestMode": {
    message: "Stripe Test Mode",
    description: "Badge making the non-live Stripe environment explicit",
  },
  "usage.billingMode": {
    message: "Stripe Billing",
    description: "Fallback badge for the Stripe billing environment",
  },
  "usage.billingReady": {
    message: "Ready",
    description: "Billing onboarding status is ready",
  },
  "usage.billingActionNeeded": {
    message: "Action needed",
    description: "Billing onboarding status needs action",
  },
  "usage.billingCustomerStatus": {
    message: "Customer",
    description: "Stripe Customer readiness row",
  },
  "usage.billingSubscriptionStatus": {
    message: "Metered subscription",
    description: "Stripe Subscription readiness row",
  },
  "usage.billingPaymentStatus": {
    message: "Payment method",
    description: "Default payment method readiness row",
  },
  "usage.billingTaxStatus": {
    message: "Automatic tax",
    description: "Stripe Tax activation row",
  },
  "usage.billingLifecycleStatus": {
    message: "Collection lifecycle",
    description: "Stripe collection and reversible enforcement state row",
  },
  "usage.billingLifecycleGrace": {
    message:
      "Payment failed ({reason}). The workspace remains available during grace; reversible suspension is scheduled after {deadline}.",
    description: "Visible billing grace state and authoritative deadline",
  },
  "usage.billingLifecycleEnforced": {
    message:
      "Billing enforcement is active ({reason}). Compute is suspended, but databases, key-value data, secrets, and billing evidence have not been deleted.",
    description: "Visible reversible billing enforcement state",
  },
  "usage.billingLifecycleRecovering": {
    message:
      "Payment recovered. bex is restoring only resources changed by billing enforcement.",
    description: "Visible precise recovery state",
  },
  "usage.billingLifecycleExcluded": {
    message:
      "This workspace is excluded from Stripe collection by an operator.",
    description: "Visible structural billing exclusion state",
  },
  "usage.billingLifecycleComped": {
    message: "This workspace is rated but fully comped by an operator.",
    description: "Visible rated-but-free comp state",
  },
  "usage.billingLifecycleUnknown": {
    message:
      "Billing state is {reason}. Use the billing portal or contact support.",
    description: "Forward-compatible unknown billing lifecycle state",
  },
  "usage.billingNoDeadline": {
    message: "no deadline reported",
    description: "Fallback when a malformed grace state has no deadline",
  },
  "usage.billingOff": {
    message: "Off",
    description:
      "Neutral status for a deliberately disabled billing capability (e.g. tax not activated)",
  },
  "usage.billingAddPayment": {
    message: "Add payment method",
    description: "Button opening setup-mode Stripe Checkout (live mode)",
  },
  "usage.billingUpdatePayment": {
    message: "Update payment method",
    description: "Button reopening setup-mode Stripe Checkout (live mode)",
  },
  "usage.billingAddPaymentTest": {
    message: "Add test payment method",
    description:
      "Button opening setup-mode Stripe Checkout when Stripe is in test mode",
  },
  "usage.billingUpdatePaymentTest": {
    message: "Update test payment method",
    description:
      "Button reopening setup-mode Stripe Checkout when Stripe is in test mode",
  },
  "usage.billingOpenPortal": {
    message: "Open billing portal",
    description: "Button opening Stripe Customer Portal",
  },
  "usage.billingHostedNote": {
    message:
      "Payment details are entered only on Stripe-hosted pages. bex never sends a Stripe server key to your browser.",
    description: "Hosted billing security note",
  },
  "usage.billingUnavailable": {
    message:
      "Billing onboarding is unavailable or you do not have billing access (billing role or admin).",
    description: "Degraded or unauthorized billing onboarding state",
  },
  "usage.billingCheckoutError": {
    message: "Could not open Stripe Checkout. Try again.",
    description: "Toast after Checkout session creation fails",
  },
  "usage.billingPortalError": {
    message: "Could not open the Stripe billing portal. Try again.",
    description: "Toast after Portal session creation fails",
  },
  "usage.paymentRequiredTitle": {
    message: "Add a payment method to continue",
    description: "Just-in-time paid-intent onboarding dialog title",
  },
  "usage.paymentRequiredDescription": {
    message:
      "This paid plan needs a payment method. Complete Stripe Checkout in the new tab; this action will resume automatically when it is ready.",
    description: "Just-in-time paid-intent onboarding explanation",
  },
  "usage.paymentRequiredRetrying": {
    message: "Payment method found. Retrying your action…",
    description: "Status while resuming the interrupted paid mutation",
  },
  "usage.paymentRequiredCancel": {
    message: "Cancel",
    description: "Cancel just-in-time payment onboarding",
  },
  "usage.planTitle": {
    message: "Plan",
    description: "Plan card title on the billing page",
  },
  "usage.planChange": {
    message: "Change plan",
    description:
      "Link from the billing page's plan card to the workspace-settings plan dialog",
  },
  "usage.paymentMethodTitle": {
    message: "Payment method",
    description: "Payment-method card title on the billing page",
  },
  "usage.paymentMethodCard": {
    message: "{brand} ending in {last4}",
    description: "Names the card on file, e.g. 'Visa ending in 4242'",
  },
  "usage.paymentMethodOnFile": {
    message: "Payment method on file",
    description:
      "Shown when a payment method exists but is not a card the provider could name",
  },
  "usage.paymentMethodNone": {
    message: "No payment method",
    description: "Shown when the workspace has no payment method",
  },
  "usage.includedUsageTitle": {
    message: "Included usage",
    description:
      "Plan-allowance card title on the billing page (was 'Resource limits')",
  },
  "usage.includedUsageDescription": {
    message:
      "What this workspace's plan includes, and how much of it is in use.",
    description: "Included-usage card description",
  },
  "usage.chargesTitle": {
    message: "Charges",
    description: "Charge-tree card title on the billing page",
  },
  "usage.chargesDescriptionEstimate": {
    message:
      "Accrued so far this period, priced from the bex rate sheet. An estimate, not an invoice.",
    description:
      "Charge-tree description when no Stripe subscription prices the period",
  },
  "usage.chargesDescriptionInvoiced": {
    message: "Accrued so far this period, as rated by Stripe.",
    description:
      "Charge-tree description when a real Stripe amount is available. Says the total is Stripe's rating rather than the amount it will invoice: credits and comp discounts can sit between the two, and the amount actually due gets its own line (w6/m98).",
  },
  "usage.amountDueAfterCredits": {
    message: "Amount due after credits",
    description:
      "Label for the charge-tree line showing what Stripe actually collects once credits and discounts are applied to the charge above it",
  },
  "usage.chargesEmpty": {
    message: "No usage in this period.",
    description: "Charge-tree empty state",
  },
  "usage.coveragePartial": {
    message: "Partial data",
    description:
      "Amber caveat label above the charges when the metering behind the estimate is degraded/incomplete (w4/048), mirroring the Metrics page's degraded badge",
  },
  "usage.coveragePartialLead": {
    message: "This estimate may undercount usage.",
    description: "Lead sentence of the partial-coverage caveat tooltip",
  },
  "usage.coveragePartialThrough": {
    message: "Metered through {through}.",
    description:
      "Partial-coverage caveat clause naming the date the meters have recorded through (a degraded hour counts as recorded, w4/496); {through} is a YYYY-MM-DD date",
  },
  "usage.coveragePartialSources": {
    message: "Degraded metering sources: {sources}.",
    description:
      "Partial-coverage caveat clause listing the degraded metering source names; {sources} is a comma-separated list",
  },
  "usage.expandAll": {
    message: "Expand all",
    description: "Charge-tree button that opens every category",
  },
  "usage.collapseAll": {
    message: "Collapse all",
    description: "Charge-tree button that closes every category",
  },
  "usage.totalToDate": {
    message: "Total month to date",
    description: "Charge-tree total row for the month in progress",
  },
  "usage.totalForPeriod": {
    message: "Total for the period",
    description: "Charge-tree total row for a completed past month",
  },
  "usage.totalBillingPeriodWindow": {
    message: "Total this billing period ({start} – {end})",
    description:
      "Charge total row when the headline is Stripe's rated amount: it covers the subscription period (anchored on the subscription's day of month), not the calendar month (w4/m139)",
  },
  "usage.totalBillingPeriod": {
    message: "Total this billing period",
    description:
      "Charge total row for Stripe's rated amount when its period bounds are unknown (w4/m139)",
  },
  "usage.breakdownMonthToDate": {
    message: "Breakdown above: {month} to date",
    description:
      "Row under a Stripe-rated total: the charge tree's own sum and window (the calendar month), which differs from the subscription period (w4/m139)",
  },
  "usage.projectedTotal": {
    message: "Projected total for {month}",
    description:
      "Charge-tree straight-line projection of month-end spend, for the estimate fallback whose total accrues over the calendar month",
  },
  "usage.projectedTotalBillingPeriod": {
    message: "Projected for this billing period",
    description:
      "Charge-tree straight-line projection of a Stripe-rated total. The subscription period need not align with the calendar month (it can span two), so no month is named (w6/050).",
  },
  "usage.chargeFree": {
    message: "Included",
    description: "Rate column value for a charge line that is priced at zero",
  },
  "usage.resourceDeleted": {
    message: "(deleted)",
    description:
      "Marker beside a charged resource that no longer exists; the name shown is the one bex retained",
  },
  "usage.categoryServices": {
    message: "Services",
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
  "usage.categoryAgentSessions": {
    message: "Agent sessions",
    description:
      "Charge-tree category for platform model usage of agent sessions",
  },
  "usage.chargeDisk": {
    message: "Disk",
    description: "Charge line for a service's provisioned persistent disk",
  },
  "usage.chargeAgentModelCost": {
    message: "Model usage",
    description:
      "Charge line for platform model spend, billed at provider cost plus markup (ADR091)",
  },
  "usage.chargeAgentModelRate": {
    message: "Provider cost + 30%",
    description: "Rate column value for the platform model spend line",
  },
  "usage.chargeAgentModelInputTokens": {
    message: "Input tokens",
    description: "Breakdown line for platform model input tokens",
  },
  "usage.chargeAgentModelOutputTokens": {
    message: "Output tokens",
    description: "Breakdown line for platform model output tokens",
  },
  "usage.chargeAgentModelCacheReadTokens": {
    message: "Cache read tokens",
    description: "Breakdown line for platform model cache-read tokens",
  },
  "usage.chargeAgentModelCacheWriteTokens": {
    message: "Cache write tokens",
    description: "Breakdown line for platform model cache-write tokens",
  },
  "usage.categorySandboxes": {
    message: "Sandboxes",
    description: "Charge-tree category for hosted agent sandboxes",
  },
  "usage.chargeCompute": {
    message: "Compute",
    description: "Charge line for metered instance time",
  },
  "usage.chargeBandwidth": {
    message: "Bandwidth",
    description: "Charge line for metered outbound bandwidth",
  },
  "usage.chargeBuild": {
    message: "Build minutes",
    description: "Charge line for metered build time",
  },
  "usage.chargeStorage": {
    message: "Storage",
    description: "Charge line for metered datastore storage",
  },
  "usage.chargeSandboxCompute": {
    message: "Sandbox compute",
    description: "Charge line for metered sandbox compute",
  },
  "usage.chargeSandboxMemory": {
    message: "Sandbox memory (included in compute)",
    description:
      "Charge line for the allocated memory a sandbox ran with; recorded beside sandbox compute, which already includes it, so it is shown and not billed separately",
  },
  "usage.chargeSandboxSnapshotStorage": {
    message: "Sandbox snapshot storage",
    description:
      "Charge line for the image a suspended sandbox retains, billed per GB-month",
  },
  "usage.creditsTotalLabel": {
    message: "Total balance",
    description: "Label above the credit balance amount",
  },
  "usage.invoiceHistoryTitle": {
    message: "Invoice history",
    description: "Invoice-history card title on the billing page",
  },
  "usage.invoiceHistoryDescription": {
    message: "Your finalized invoices.",
    description: "Invoice-history card description",
  },
  "usage.invoiceKindAgentModelCharge": {
    message: "Agent model charge",
    description:
      "Label on an invoice-history row for a charge made when agent model spend reached its threshold",
  },
  "usage.invoiceHistoryChargesNote": {
    message:
      "Agent model charges made during a month are deducted from that month's invoice.",
    description:
      "Invoice-history note when the list includes agent model charges",
  },
  "usage.creditsAppliedLine": {
    message: "Credits applied −{applied} → amount due {due}",
    description:
      "Credit applied to the current period and the remaining amount due",
  },
  "usage.chargesDescriptionPending": {
    message: "Accrued so far this period, priced from the bex rate sheet.",
    description:
      "Charges card description while the invoiced total is still loading; deliberately states only what is already true, without claiming the figure is or is not a Stripe invoice.",
  },
  "usage.paymentMethodRejectedPrepaid": {
    message: "Prepaid cards aren't accepted; add a credit or debit card.",
    description:
      "Shown when the last card offered was refused at binding because it is prepaid (ADR091 D10.5)",
  },
  "usage.paymentMethodRejectedUnpaidBalance": {
    message: "This card is linked to an unpaid balance; add a different card.",
    description:
      "Shown when the last card offered was refused because it carries an unpaid written-off balance (ADR091 D10.6)",
  },
  "usage.agentModelTitle": {
    message: "Agent model",
    description:
      "Billing page card and nav entry for agent model spend (ADR091 D6)",
  },
  "usage.agentModelPricing": {
    message: "Billed at the model provider's cost + 30%.",
    description: "Pricing statement under the Agent model title",
  },
  "usage.agentModelMonthToDate": {
    message: "Month to date",
    description: "Label of the current month's agent model total",
  },
  "usage.agentModelPeriodTotal": {
    message: "Total for the month",
    description: "Label of a closed month's agent model total",
  },
  "usage.agentModelBreakdown_one": {
    message: "Breakdown by model ({count} model)",
    description: "Toggle that expands the per-model breakdown",
  },
  "usage.agentModelBreakdown_other": {
    message: "Breakdown by model ({count} models)",
    description: "Toggle that expands the per-model breakdown",
  },
  "usage.agentModelEmpty": {
    message: "No agent model usage in this period.",
    description: "Shown when the period has no agent model spend",
  },
  "usage.lessThanAmount": {
    message: "<{amount}",
    description:
      "A charge that rounds below the smallest amount shown, e.g. <$0.01",
  },
};

export default enUsage;
