import type { TranslationEntry } from "@/i18n";

const enUsage: Record<string, TranslationEntry> = {
  "usage.pageTitle": {
    message: "Usage",
    description: "Usage page heading and browser title",
  },
  "usage.pageSubtitle": {
    message: "Month-to-date workspace consumption",
    description: "Usage page subtitle beneath the heading",
  },
  "usage.resourceCapsTitle": {
    message: "Resource limits",
    description: "Workspace creation-cap card title on the Usage page",
  },
  "usage.resourceCapsDescription": {
    message: "Current resource counts for this workspace",
    description: "Workspace creation-cap card description",
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
  "usage.resourceCapsValue": {
    message: "{used} of {limit} used",
    description: "Used-versus-limit resource count",
  },
  "usage.resourceCapsNearLimit": {
    message: "Near limit",
    description: "Warning shown once a resource count reaches 80 percent",
  },
  "usage.computeTitle": {
    message: "Compute",
    description: "Compute section heading on the Usage page",
  },
  "usage.computeDescription": {
    message: "Instance-hours by service and plan",
    description: "Compute section description",
  },
  "usage.bandwidthTitle": {
    message: "Bandwidth",
    description: "Bandwidth section heading on the Usage page",
  },
  "usage.bandwidthDescription": {
    message:
      "HTTP, WebSocket, direct public, and public datastore response egress",
    description: "Bandwidth section description",
  },
  "usage.buildTitle": {
    message: "Build Minutes",
    description: "Build minutes section heading on the Usage page",
  },
  "usage.buildDescription": {
    message: "Pipeline minutes consumed by builds",
    description: "Build section description",
  },
  "usage.storageTitle": {
    message: "Storage",
    description: "Managed datastore storage section heading on the Usage page",
  },
  "usage.storageDescription": {
    message: "Actual Postgres and Key Value volume usage",
    description: "Storage section description",
  },
  "usage.colService": {
    message: "Service",
    description: "Table column header: service name",
  },
  "usage.colKind": {
    message: "Kind",
    description:
      "Table column header: resource kind (service/postgres/key_value)",
  },
  "usage.colPlan": {
    message: "Plan",
    description: "Table column header: service plan/tier",
  },
  "usage.colHours": {
    message: "Hours",
    description: "Table column header: compute hours",
  },
  "usage.colBandwidth": {
    message: "Bandwidth",
    description: "Table column header: egress bandwidth",
  },
  "usage.colMinutes": {
    message: "Minutes",
    description: "Table column header: build minutes",
  },
  "usage.colGBHours": {
    message: "GB-hours",
    description: "Table column header: storage gigabyte-hours",
  },
  "usage.totalRow": {
    message: "Total",
    description: "Summary row label in usage tables",
  },
  "usage.empty": {
    message: "No usage recorded this month.",
    description: "Empty-state message when a section has no data",
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
  "usage.trendTitle": {
    message: "3-Month Trend",
    description: "Heading for the trend view showing last 3 months of usage",
  },
  "usage.trendDescription": {
    message: "Total per meter over the last three calendar months",
    description: "Subtitle for the trend charts on the Usage page",
  },
  "usage.estimatedCostTitle": {
    message: "Estimated Cost",
    description: "Estimated cost section heading on the Usage page",
  },
  "usage.estimatedCostDescription": {
    message:
      "30% below Render on compute, Postgres, key-value, and Postgres storage; 90% below on bandwidth. Estimate only — not an invoice.",
    description:
      "Estimated cost section description explaining the pricing policy",
  },
  "usage.estimatedCostNote": {
    message: "Estimate only — not an invoice",
    description: "Short disclaimer shown next to the estimated cost total",
  },
  "usage.colMeter": {
    message: "Meter",
    description: "Table column header for the usage meter kind",
  },
  "usage.colEstimate": {
    message: "Estimate",
    description: "Table column header for the estimated USD cost per meter",
  },
  "usage.estimatedCostUnavailable": {
    message: "No billable usage this period.",
    description:
      "Empty-state message when there is no billable usage to estimate",
  },
  "usage.currentSpendTitle": {
    message: "Current Spend",
    description:
      "Heading for the real billing section (actual Stripe cost + invoices)",
  },
  "usage.currentSpendBadge": {
    message: "Invoice",
    description:
      "Badge distinguishing the real billing card from the estimate-only card",
  },
  "usage.currentSpendDescription": {
    message:
      "Your actual billed cost and finalized invoices — not an estimate.",
    description: "Description clarifying this card shows real charges",
  },
  "usage.currentSpendNote": {
    message: "Current billing period, so far",
    description: "Short note beside the current-period real cost total",
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
  "usage.billingTaxUnconfigured": {
    message:
      "Tax is not configured. Collection stays off until an operator confirms a canonical product tax code and an active test registration.",
    description: "Fail-closed tax setup explanation",
  },
  "usage.billingAddPayment": {
    message: "Add test payment method",
    description: "Button opening setup-mode Stripe Checkout",
  },
  "usage.billingUpdatePayment": {
    message: "Update test payment method",
    description: "Button reopening setup-mode Stripe Checkout",
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
};

export default enUsage;
