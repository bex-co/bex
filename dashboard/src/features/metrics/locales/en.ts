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
    description:
      "Network metrics chart section title, with the selected quantile",
  },
  "metrics.outboundBandwidth": {
    message: "Outbound Bandwidth",
    description: "Network metrics chart section title",
  },
  "metrics.monthToDateBandwidth": {
    message: "{amount} used this month",
    description:
      "Footer under the Outbound Bandwidth chart, showing month-to-date egress",
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
    message: "Per-instance resource usage over the selected range",
    description: "Application metrics card description",
  },
  "metrics.limitLabel": {
    message: "Limit {value}",
    description:
      "Resource limit annotation on the Memory/CPU chart headers, e.g. 'Limit 512 MiB'",
  },
  "metrics.noLimitConfigured": {
    message: "No limit configured — percentage is undefined",
    description:
      "Shown instead of a percentage chart when the App's pods declare no resource limit",
  },
  "metrics.targetLabel": {
    message: "Target {value}",
    description:
      "Autoscale-target annotation on the Memory/CPU chart headers (percentage mode only), e.g. 'Target 70%'",
  },
  "metrics.diskTitle": {
    message: "Disk",
    description: "Datastore metrics panel: disk usage chart section title",
  },
  "metrics.diskCapacityLabel": {
    message: "Capacity {value}",
    description:
      "Disk usage chart header annotation showing the PVC's total capacity",
  },
  "metrics.connectionsTitle": {
    message: "Active Connections",
    description:
      "Datastore metrics panel: Postgres active-connections chart section title",
  },
  "metrics.memoryTitle": {
    message: "Memory",
    description:
      "Datastore metrics panel: Key Value used-memory chart section title",
  },
  "metrics.kvConnectionsTitle": {
    message: "Connections",
    description:
      "Datastore metrics panel: Key Value connected-clients chart section title",
  },
  "metrics.replicationLagTitle": {
    message: "Replication Lag",
    description:
      "Datastore metrics panel: Postgres replication-lag chart section title",
  },
  "metrics.replicationLagPendingHA": {
    message:
      "N/A — no replica (enable High Availability to see replication lag)",
    description:
      "Shown instead of a chart for replication lag before Postgres HA is enabled (w1/m22)",
  },
  "metrics.datastoreMetricsTitle": {
    message: "Metrics",
    description: "Database/Key Value detail page metrics panel card title",
  },
  "metrics.datastoreMetricsDescription": {
    message: "Live resource usage for this instance",
    description:
      "Database/Key Value detail page metrics panel card description",
  },
  "metrics.statusCode": {
    message: "Status Code",
    description: "Toolbar filter label for HTTP status code",
  },
  "metrics.statusCodeAll": {
    message: "All",
    description: "Status-code filter option meaning no filtering",
  },
  "metrics.groupBy": {
    message: "Group by",
    description: "Total Requests chart control label",
  },
  "metrics.groupByAllRequests": {
    message: "All requests",
    description: "Group-by option: one aggregate series",
  },
  "metrics.groupByStatus": {
    message: "Status code",
    description: "Group-by option: one series per HTTP status code",
  },
  "metrics.groupByMethod": {
    message: "Method",
    description: "Group-by option: one series per HTTP method",
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
  "metrics.rangeLast3Hours": {
    message: "Last 3 hours",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast6Hours": {
    message: "Last 6 hours",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast12Hours": {
    message: "Last 12 hours",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLastDay": {
    message: "Last day",
    description: "Metrics time-range filter option",
  },
};

export default enMetrics;
