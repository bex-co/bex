import type { TranslationEntry } from "@/i18n";

const enMetrics: Record<string, TranslationEntry> = {
  "metrics.subtitle": {
    message: "Metrics — live from bex-api",
    description: "Subtitle on the service metrics page",
  },
  "metrics.networkTitle": {
    message: "Network Metrics",
    description: "Network metrics card title",
  },
  "metrics.networkDescription": {
    message: "Aggregated across all instances",
    description: "Network metrics card description",
  },
  "metrics.totalRequests": {
    message: "Total Requests",
    description: "Network metrics chart section title",
  },
  "metrics.responseTimes": {
    message: "Response Times ({quantile})",
    description: "Network metrics chart section title, with the selected quantile",
  },
  "metrics.outboundBandwidth": {
    message: "Outbound Bandwidth",
    description: "Network metrics chart section title",
  },
  "metrics.sourceNotConfigured": {
    message: "Metrics source not configured",
    description: "Shown when bex-api reports no metrics backend is wired up",
  },
  "metrics.applicationTitle": {
    message: "Application Metrics",
    description: "Application metrics card title",
  },
  "metrics.applicationDescription": {
    message: "Per-instance resource usage, from metrics-server",
    description: "Application metrics card description",
  },
  "metrics.memory": {
    message: "Memory",
    description: "Application metrics stat tile label",
  },
  "metrics.cpu": {
    message: "CPU",
    description: "Application metrics stat tile label",
  },
  "metrics.totalInstances": {
    message: "Total Instances",
    description: "Application metrics stat tile label",
  },
  "metrics.filterPercentage": {
    message: "Percentage",
    description: "Metrics filter tab: show values as a percentage",
  },
  "metrics.filterTotal": {
    message: "Total",
    description: "Metrics filter tab: show values as an absolute total",
  },
  "metrics.rangeLast30Minutes": {
    message: "Last 30 minutes",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLastHour": {
    message: "Last hour",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast12Hours": {
    message: "Last 12 hours",
    description: "Metrics time-range filter option",
  },
};

export default enMetrics;
