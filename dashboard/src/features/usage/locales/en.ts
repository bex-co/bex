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
  "usage.resourceCapsNearLimit": {
    message: "Near limit",
    description: "Warning shown once a resource count reaches 80 percent",
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
    message: "${amount} of it expires {date}",
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
    message: "Complete only through {through}.",
    description:
      "Partial-coverage caveat clause naming the date the estimate is complete through; {through} is a YYYY-MM-DD date",
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
  "usage.creditsAppliedLine": {
    message: "Credits applied \u2212${applied} \u2192 amount due ${due}",
    description:
      "Credit applied to the current period and the remaining amount due",
  },
  "usage.chargesDescriptionPending": {
    message: "Accrued so far this period, priced from the bex rate sheet.",
    description:
      "Charges card description while the invoiced total is still loading; deliberately states only what is already true, without claiming the figure is or is not a Stripe invoice.",
  },
};

export default enUsage;
