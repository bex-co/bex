import type { TranslationEntry } from "@/i18n";

const enMetrics: Record<string, TranslationEntry> = {
  "metrics.eventsTitle": {
    message: "Event timeline",
    description:
      "Metrics-page timeline of service events in the selected range",
  },
  "metrics.eventsFilterLabel": {
    message: "Filter events",
    description: "Accessible label for the Metrics event category filter",
  },
  "metrics.eventsFilterAll": {
    message: "All events",
    description: "All event categories",
  },
  "metrics.eventsFilterDeploys": {
    message: "Deploys",
    description: "Deploy events category",
  },
  "metrics.eventsFilterLifecycle": {
    message: "Lifecycle",
    description: "Lifecycle events category",
  },
  "metrics.eventsFilterConfig": {
    message: "Configuration",
    description: "Configuration events category",
  },
  "metrics.eventsLoading": {
    message: "Loading events…",
    description: "Event timeline loading state",
  },
  "metrics.eventsError": {
    message: "Couldn't load events.",
    description: "Event timeline error state",
  },
  "metrics.eventsEmptyTitle": {
    message: "No events",
    description: "Event timeline empty-state title",
  },
  "metrics.eventsEmptyBody": {
    message: "It's been quiet in this time range.",
    description: "Event timeline empty-state body",
  },
  "metrics.eventsChanged": {
    message: "Service changed",
    description: "Fallback event timeline label",
  },
  "metrics.eventMarkersLabel": {
    message: "Service events",
    description: "Accessible label for the event-marker strip above a chart",
  },
  "metrics.eventMarkerCluster": {
    message: "{count} events",
    description:
      "Accessible label for a clustered chart event marker (count badge)",
  },
  "metrics.eventMarkerHint": {
    message: "Click to view details",
    description: "Footer line of the chart event-marker hover tooltip",
  },
  "metrics.eventDeployStarted": {
    message: "Deploy started",
    description: "Chart event-marker label: a deploy began",
  },
  "metrics.eventDeployLiveFor": {
    message: "Deploy live for {commit}",
    description:
      "Chart event-marker label: a deploy went live, with its commit short id",
  },
  "metrics.eventDeployLive": {
    message: "Deploy live",
    description: "Chart event-marker label: a deploy went live (no commit id)",
  },
  "metrics.eventDeployFailed": {
    message: "Deploy failed",
    description: "Chart event-marker label: a deploy failed",
  },
  "metrics.eventDeployCanceled": {
    message: "Deploy canceled",
    description: "Chart event-marker label: a deploy was canceled",
  },
  "metrics.eventDeployEnded": {
    message: "Deploy ended",
    description:
      "Chart event-marker label: a deploy finished with an unrecognized status",
  },
  "metrics.eventServerRestarted": {
    message: "Server restarted",
    description: "Chart event-marker label: the service was restarted",
  },
  "metrics.eventSuspended": {
    message: "Service suspended",
    description: "Chart event-marker label: the service was suspended",
  },
  "metrics.eventResumed": {
    message: "Service resumed",
    description: "Chart event-marker label: the service was resumed",
  },
  "metrics.eventInstanceCountChanged": {
    message: "Instance count changed",
    description: "Chart event-marker label: manual scaling changed the count",
  },
  "metrics.eventAutoscalingChanged": {
    message: "Autoscaling config changed",
    description: "Chart event-marker label: autoscaling settings changed",
  },
  "metrics.eventPlanChanged": {
    message: "Instance type changed",
    description: "Chart event-marker label: the service's plan changed",
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
    message: "Response Times",
    description: "Network metrics chart section title",
  },
  "metrics.percentile": {
    message: "Percentile",
    description: "Response Times section control label",
  },
  "metrics.percentileAll": {
    message: "All",
    description:
      "Percentile dropdown option that overlays p50/p90/p99 on one chart",
  },
  "metrics.requestsCount": {
    message: "{count} requests",
    description:
      "Aggregate request count beside the Total Requests section title",
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
  "metrics.bandwidthDegraded": {
    message: "Partial data",
    description:
      "Badge on the Outbound Bandwidth chart when an egress source's health check failed inside the window (w1/m50)",
  },
  "metrics.bandwidthDegradedDetail": {
    message:
      "The {sources} egress source was unhealthy for part of this window — bandwidth may be undercounted.",
    description:
      "Tooltip expanding the Partial data badge; {sources} is the comma-joined raw source tokens (http/websocket/direct)",
  },
  "metrics.bandwidthError": {
    message: "Couldn't load bandwidth — try refreshing",
    description:
      "Bandwidth chart body when the query itself failed (network/server error, not an empty window)",
  },
  "metrics.sourceNotConfigured": {
    message: "Metrics source not configured",
    description: "Shown when bex-api reports no metrics backend is wired up",
  },
  "metrics.applicationTitle": {
    message: "Application Metrics",
    description: "Application metrics card title",
  },
  "metrics.limitLink": {
    message: "Limit",
    description:
      "Link to the Instance Type tab on the Memory/CPU chart headers, followed by the limit value when configured",
  },
  "metrics.manageScaling": {
    message: "Manage scaling",
    description: "Link to the Scaling tab on the Total Instances chart header",
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
  "metrics.rangeLast4Hours": {
    message: "Last 4 hours",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast12Hours": {
    message: "Last 12 hours",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast24Hours": {
    message: "Last 24 hours",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast2Days": {
    message: "Last 2 days",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast7Days": {
    message: "Last 7 days",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast14Days": {
    message: "Last 14 days",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast30Days": {
    message: "Last 30 days",
    description: "Metrics time-range filter option (w5/m56; Render plan-gates it)",
  },
  "metrics.rangeCustom": {
    message: "Custom…",
    description: "Time-range dropdown option that opens the start/end picker",
  },
  "metrics.rangeCustomTitle": {
    message: "Custom range",
    description: "Custom time-range picker dialog title",
  },
  "metrics.rangeCustomDescription": {
    message: "Choose an absolute start and end. Windows over 30 days aren't available.",
    description: "Custom time-range picker dialog description",
  },
  "metrics.rangeCustomStart": {
    message: "Start",
    description: "Custom range start datetime field label",
  },
  "metrics.rangeCustomEnd": {
    message: "End",
    description: "Custom range end datetime field label",
  },
  "metrics.rangeCustomApply": {
    message: "Apply",
    description: "Custom range apply button",
  },
  "metrics.rangeCustomCancel": {
    message: "Cancel",
    description: "Custom range cancel button",
  },
  "metrics.rangeCustomErrorOrder": {
    message: "The end must be after the start.",
    description: "Custom range validation: end not after start",
  },
  "metrics.rangeCustomErrorTooLong": {
    message: "Choose a window of 30 days or less.",
    description: "Custom range validation: window exceeds the max query window",
  },
  "metrics.rangeLabel": {
    message: "Time range",
    description: "Accessible label for the time-range dropdown",
  },
  "metrics.eventsToggle": {
    message: "Show event timeline",
    description: "Accessible label for the Metrics event-timeline toggle",
  },
};

export default enMetrics;
