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
    message: "Outbound egress metered by Traefik",
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
};

export default enUsage;
