import type { TranslationEntry } from "@/i18n";

const deMetrics: Record<string, TranslationEntry> = {
  "metrics.cpuCores": {
    message: "{value} Kerne",
    description:
      "A CPU amount in cores (a limit or the latest usage); value is a decimal such as 0.100 (w4/566)",
  },
  "metrics.eventsTitle": {
    message: "Ereignis-Timeline",
    description:
      "Metrics-page timeline of service events in the selected range",
  },
  "metrics.eventsFilterLabel": {
    message: "Ereignisse filtern",
    description: "Accessible label for the Metrics event category filter",
  },
  "metrics.eventsFilterAll": {
    message: "Alle Ereignisse",
    description: "All event categories",
  },
  "metrics.eventsFilterDeploys": {
    message: "Deployments",
    description: "Deploy events category",
  },
  "metrics.eventsFilterLifecycle": {
    message: "Lebenszyklus",
    description: "Lifecycle events category",
  },
  "metrics.eventsFilterConfig": {
    message: "Konfiguration",
    description: "Configuration events category",
  },
  "metrics.eventsLoading": {
    message: "Ereignisse werden geladen…",
    description: "Event timeline loading state",
  },
  "metrics.eventsError": {
    message: "Ereignisse konnten nicht geladen werden.",
    description: "Event timeline error state",
  },
  "metrics.eventsEmptyTitle": {
    message: "Keine Ereignisse",
    description: "Event timeline empty-state title",
  },
  "metrics.eventsEmptyBody": {
    message: "In diesem Zeitraum war es ruhig.",
    description: "Event timeline empty-state body",
  },
  "metrics.eventsChanged": {
    message: "Service geändert",
    description: "Fallback event timeline label",
  },
  "metrics.eventMarkersLabel": {
    message: "Service-Ereignisse",
    description: "Accessible label for the event-marker strip above a chart",
  },
  "metrics.eventMarkerCluster_one": {
    message: "{count} Ereignis",
    description:
      "Accessible label for a clustered chart event marker (count badge, singular)",
  },
  "metrics.eventMarkerCluster_other": {
    message: "{count} Ereignisse",
    description:
      "Accessible label for a clustered chart event marker (count badge, plural)",
  },
  "metrics.lineChartLabel_one": {
    message: "Liniendiagramm mit {count} Datenpunkt",
    description:
      "Accessible name of a metrics line chart; count is the number of distinct sample times (singular)",
  },
  "metrics.lineChartLabel_other": {
    message: "Liniendiagramm mit {count} Datenpunkten",
    description:
      "Accessible name of a metrics line chart; count is the number of distinct sample times (plural)",
  },
  "metrics.barChartLabel_one": {
    message: "Balkendiagramm mit {count} Datenpunkt",
    description:
      "Accessible name of a metrics bar chart; count is the number of time buckets (singular)",
  },
  "metrics.barChartLabel_other": {
    message: "Balkendiagramm mit {count} Datenpunkten",
    description:
      "Accessible name of a metrics bar chart; count is the number of time buckets (plural)",
  },
  "metrics.barValueAt": {
    message: "{value} um {time}",
    description:
      "Accessible name of one focusable bar: its formatted value (e.g. 42 or 1.2K) and its local sample time",
  },
  "metrics.chartEmpty": {
    message: "Keine Daten im Zeitraum",
    description:
      "Default empty state of a metrics chart when the settled query returned no samples in the selected time range",
  },
  "metrics.eventMarkerHint": {
    message: "Klicke für Details",
    description: "Footer line of the chart event-marker hover tooltip",
  },
  "metrics.eventDeployStarted": {
    message: "Deployment gestartet",
    description: "Chart event-marker label: a deploy began",
  },
  "metrics.eventDeployLiveFor": {
    message: "Deployment live für {commit}",
    description:
      "Chart event-marker label: a deploy went live, with its commit short id",
  },
  "metrics.eventDeployLive": {
    message: "Deployment live",
    description: "Chart event-marker label: a deploy went live (no commit id)",
  },
  "metrics.eventDeployFailed": {
    message: "Deployment fehlgeschlagen",
    description: "Chart event-marker label: a deploy failed",
  },
  "metrics.eventDeployCanceled": {
    message: "Deployment abgebrochen",
    description: "Chart event-marker label: a deploy was canceled",
  },
  "metrics.eventDeployEnded": {
    message: "Deployment beendet",
    description:
      "Chart event-marker label: a deploy finished with an unrecognized status",
  },
  "metrics.eventBuildStarted": {
    message: "Build gestartet",
    description: "Chart event-marker label: an image build began",
  },
  "metrics.eventBuildEnded": {
    message: "Build beendet",
    description: "Chart event-marker label: an image build finished",
  },
  "metrics.eventPreDeployStarted": {
    message: "Pre-Deploy gestartet",
    description: "Chart event-marker label: the pre-deploy command began",
  },
  "metrics.eventPreDeployEnded": {
    message: "Pre-Deploy beendet",
    description: "Chart event-marker label: the pre-deploy command finished",
  },
  "metrics.eventJobRunEnded": {
    message: "Job beendet",
    description: "Chart event-marker label: a one-off job finished",
  },
  "metrics.eventBranchDeleted": {
    message: "Branch gelöscht",
    description: "Chart event-marker label: the tracked Git branch was deleted",
  },
  "metrics.eventServerRestarted": {
    message: "Server neu gestartet",
    description: "Chart event-marker label: the service was restarted",
  },
  "metrics.eventSuspended": {
    message: "Service pausiert",
    description: "Chart event-marker label: the service was suspended",
  },
  "metrics.eventResumed": {
    message: "Service fortgesetzt",
    description: "Chart event-marker label: the service was resumed",
  },
  "metrics.eventInstanceCountChanged": {
    message: "Instanzanzahl geändert",
    description: "Chart event-marker label: manual scaling changed the count",
  },
  "metrics.eventAutoscalingChanged": {
    message: "Autoscaling-Konfiguration geändert",
    description: "Chart event-marker label: autoscaling settings changed",
  },
  "metrics.eventPlanChanged": {
    message: "Instanztyp geändert",
    description: "Chart event-marker label: the service's plan changed",
  },
  "metrics.networkTitle": {
    message: "Netzwerkmetriken",
    description: "Network metrics card title",
  },
  "metrics.networkDescription": {
    message: "Über alle Instanzen aggregiert",
    description: "Network metrics card description",
  },
  "metrics.totalRequests": {
    message: "Anfragen gesamt",
    description: "Network metrics chart section title",
  },
  "metrics.responseTimes": {
    message: "Antwortzeiten",
    description: "Network metrics chart section title",
  },
  "metrics.percentile": {
    message: "Perzentil",
    description: "Response Times section control label",
  },
  "metrics.percentileAll": {
    message: "Alle",
    description:
      "Percentile dropdown option that overlays p50/p90/p99 on one chart",
  },
  "metrics.requestsCount_one": {
    message: "{formatted} Anfrage",
    description:
      "Aggregate request count beside the Total Requests section title (singular; {formatted} is the locale-formatted count)",
  },
  "metrics.requestsCount_other": {
    message: "{formatted} Anfragen",
    description:
      "Aggregate request count beside the Total Requests section title (plural; {formatted} is the locale-formatted count)",
  },
  "metrics.outboundBandwidth": {
    message: "Ausgehende Bandbreite",
    description: "Network metrics chart section title",
  },
  "metrics.monthToDateBandwidth": {
    message: "{amount} diesen Monat verbraucht",
    description:
      "Footer under the Outbound Bandwidth chart, showing month-to-date egress",
  },
  "metrics.monthToDateBandwidthError": {
    message:
      "Bandbreite dieses Monats konnte nicht geladen werden — versuche es mit Neuladen",
    description:
      "Month-to-date bandwidth footer when the query failed with no value — distinct from a known zero and from the chart's bandwidthError (w4/090)",
  },
  "metrics.bandwidthDegraded": {
    message: "Teilweise Daten",
    description:
      "Badge on the Outbound Bandwidth chart when an egress source's health check failed inside the window (w1/m50)",
  },
  "metrics.bandwidthDegradedDetail": {
    message:
      "Die Egress-Quelle {sources} war in einem Teil dieses Zeitraums fehlerhaft — die Bandbreite wurde möglicherweise zu niedrig erfasst.",
    description:
      "Tooltip expanding the Partial data badge; {sources} is the comma-joined raw source tokens (http/websocket/direct)",
  },
  "metrics.bandwidthError": {
    message:
      "Bandbreite konnte nicht geladen werden — versuche es mit Neuladen",
    description:
      "Bandwidth chart body when the query itself failed (network/server error, not an empty window)",
  },
  "metrics.sourceNotConfigured": {
    message: "Metrikquelle nicht konfiguriert",
    description: "Shown when bex-api reports no metrics backend is wired up",
  },
  "metrics.chartError": {
    message: "Diese Metrik konnte nicht geladen werden",
    description:
      "Shown when a metrics query fails with a real error (not a 503/no-data) — distinct from the empty 'No data in range' state (w9/m86)",
  },
  "metrics.chartThrottled": {
    message: "Wird aufgeholt — der Traffic war kurzzeitig begrenzt",
    description:
      "Cold-load state when a metrics read was shed (RATE_LIMITED / AUTH_OVERLOADED) with no cached series — transient, not 'metrics unavailable' (w4/m100)",
  },
  "metrics.chartStaleThrottled": {
    message:
      "Zwischengespeicherte Daten werden angezeigt — Aktualisierung nach kurzer Begrenzung",
    description:
      "Warm-cache banner when a metrics refetch was throttled; cached series remain visible but must not read as current (w4/m100)",
  },
  "metrics.applicationTitle": {
    message: "Anwendungsmetriken",
    description: "Application metrics card title",
  },
  "metrics.limitLink": {
    message: "Limit",
    description:
      "Link to the Compute page on the Memory/CPU chart headers, followed by the limit value when configured",
  },
  "metrics.manageScaling": {
    message: "Skalierung verwalten",
    description:
      "Link to the Compute page's Manual Scaling section on the Total Instances chart header",
  },
  "metrics.noLimitConfigured": {
    message: "Kein Limit konfiguriert — Prozentwert nicht definiert",
    description:
      "Shown instead of a percentage chart when the App's pods declare no resource limit",
  },
  "metrics.limitsVary": {
    message: "Limits unterscheiden sich",
    description:
      "Memory/CPU chart header annotation when the selected replicas declare different limits — each replica is percentized against its own limit, so no single limit value applies",
  },
  "metrics.percentageUnavailable": {
    message:
      "Prozentwerte nicht verfügbar — kein verlässliches Limit für diesen Zeitraum",
    description:
      "Shown instead of a percentage chart when usage samples exist but every denominator was missing, zero, or otherwise untrustworthy (deleted pods, predated limit history, a mid-window rollout gap) — distinct from no usage samples or a source failure",
  },
  "metrics.targetLabel": {
    message: "Ziel {value}",
    description:
      "Autoscale-target annotation on the Memory/CPU chart headers (percentage mode only), e.g. 'Target 70%'",
  },
  "metrics.diskTitle": {
    message: "Disk",
    description: "Datastore metrics panel: disk usage chart section title",
  },
  "metrics.diskCapacityLabel": {
    message: "Kapazität {value}",
    description:
      "Disk usage chart header annotation showing the datastore's billed/provisioned disk size",
  },
  "metrics.connectionsTitle": {
    message: "Aktive Verbindungen",
    description:
      "Datastore metrics panel: active-connections chart section title (Postgres client connections; Key Value connected clients), as Render titles both",
  },
  "metrics.memoryTitle": {
    message: "Arbeitsspeicher",
    description:
      "Datastore metrics panel: memory chart section title (Postgres container memory; Key Value used memory)",
  },
  "metrics.datastoreCpuTitle": {
    message: "CPU",
    description: "Datastore metrics panel: CPU usage chart section title",
  },
  "metrics.datastoreLimitLabel": {
    message: "Limit {value}",
    description:
      "Datastore CPU/memory/connections chart header annotation showing the instance's configured limit (a Key Value's connection limit is its plan's)",
  },
  "metrics.replicationLagTitle": {
    message: "Replikationsverzögerung",
    description:
      "Datastore metrics panel: Postgres replication-lag chart section title",
  },
  "metrics.diskActivityTitle": {
    message: "Disk-Aktivität",
    description:
      "Datastore metrics panel: Render's Disk Activity chart title — bytes read from and written to the data volume per bucket (Postgres, Key Value, a service's disk; w7/m430)",
  },
  "metrics.diskOperationsTitle": {
    message: "Disk-Operationen",
    description:
      "Datastore metrics panel: Render's Disk Operations chart title — read and write operations on the data volume per bucket (Postgres, a service's disk; w7/m430)",
  },
  "metrics.networkActivityTitle": {
    message: "Netzwerkaktivität",
    description:
      "Datastore metrics panel: Render's Network Activity chart title — bytes a Postgres read from and wrote to the network per bucket (w7/m430)",
  },
  "metrics.transactionVolumeTitle": {
    message: "Transaktionsvolumen",
    description:
      "Datastore metrics panel: Render's Transaction Volume chart title — transactions a Postgres executed per bucket (w7/m430)",
  },
  "metrics.lockDelayedQueriesTitle": {
    message: "Durch Sperren verzögerte Abfragen",
    description:
      "Datastore metrics panel: Render's Lock-Delayed Queries chart title — completed queries that waited a second or more on a lock (w7/m430)",
  },
  "metrics.directionRead": {
    message: "Lesen",
    description:
      "Legend entry of the read half of a two-series chart (data read from disk or the network)",
  },
  "metrics.directionWrite": {
    message: "Schreiben",
    description:
      "Legend entry of the write half of a two-series chart (data written to disk or the network)",
  },
  "metrics.seriesInstanceDirection": {
    message: "{instance} · {direction}",
    description:
      "Legend entry of a two-series chart when the datastore has several instances: the instance id, then Read or Write",
  },
  "metrics.lockDelayedStoreUnavailable": {
    message: "Durch Sperren verzögerte Abfragen benötigen den Log-Speicher",
    description:
      "Shown instead of the Lock-Delayed Queries chart on a deployment with no durable log store (503): the count comes from Postgres log lines",
  },
  "metrics.replicationLagPendingHA": {
    message:
      "Nicht verfügbar — kein Replikat (aktiviere High Availability, um die Replikationsverzögerung zu sehen)",
    description:
      "Shown instead of a chart for replication lag before Postgres HA is enabled (w1/m22)",
  },
  "metrics.datastoreMetricsTitle": {
    message: "Metriken",
    description: "Database/Key Value detail page metrics panel card title",
  },
  "metrics.datastoreMetricsDescription": {
    message: "Aktuelle Ressourcennutzung dieser Instanz",
    description:
      "Database/Key Value detail page metrics panel card description",
  },
  "metrics.statusCode": {
    message: "Statuscode",
    description: "Toolbar filter label for HTTP status code",
  },
  "metrics.statusCodeAll": {
    message: "Alle",
    description: "Status-code filter option meaning no filtering",
  },
  "metrics.host": {
    message: "Host",
    description: "Network-card filter label for the request Host",
  },
  "metrics.hostAll": {
    message: "Alle",
    description: "Host filter option meaning no filtering",
  },
  "metrics.path": {
    message: "Pfad",
    description: "Network-card filter label for the request Path",
  },
  "metrics.pathPlaceholder": {
    message: "Pfad…",
    description: "Placeholder for the free-text Path filter input",
  },
  "metrics.pathClear": {
    message: "Pfadfilter löschen",
    description: "Accessible label for the button that clears the Path filter",
  },
  "metrics.hostPathStoreUnavailable": {
    message:
      "Host- und Pfadfilter sowie die Host-Aufschlüsselung benötigen den Log-Speicher",
    description:
      "Per-section state when a host/path-filtered or host-grouped request metric hits a deployment with no durable log store (503) — served from the request-log store (Loki), unavailable here",
  },
  "metrics.groupBy": {
    message: "Gruppieren nach",
    description: "Total Requests chart control label",
  },
  "metrics.groupByAllRequests": {
    message: "Alle Anfragen",
    description: "Group-by option: one aggregate series",
  },
  "metrics.groupByStatus": {
    message: "Statuscode",
    description: "Group-by option: one series per HTTP status code",
  },
  "metrics.groupByMethod": {
    message: "Methode",
    description: "Group-by option: one series per HTTP method",
  },
  "metrics.groupByHost": {
    message: "Host",
    description: "Group-by option: one series per request host",
  },
  "metrics.memory": {
    message: "Arbeitsspeicher",
    description: "Application metrics stat tile label",
  },
  "metrics.cpu": {
    message: "CPU",
    description: "Application metrics stat tile label",
  },
  "metrics.totalInstances": {
    message: "Instanzen gesamt",
    description: "Application metrics stat tile label",
  },
  "metrics.filterPercentage": {
    message: "Prozent",
    description: "Metrics filter tab: show values as a percentage",
  },
  "metrics.filterTotal": {
    message: "Gesamt",
    description: "Metrics filter tab: show values as an absolute total",
  },
  "metrics.instanceFilter": {
    message: "Instanzen",
    description: "Application metrics instance multi-select label",
  },
  "metrics.instanceFilterAll": {
    message: "Alle",
    description:
      "Instances filter trigger's value when no instance is picked (every instance is shown)",
  },
  "metrics.instanceShowAll": {
    message: "Alle anzeigen",
    description:
      "Explicit action that clears the retained instance selection back to all instances (w5/m91)",
  },
  "metrics.instanceUnavailableSuffix": {
    message: " (nicht verfügbar)",
    description:
      "Suffix appended to a retained instance option that is no longer in the window or discovery (w5/m91)",
  },
  "metrics.instancesUnavailable_one": {
    message:
      "{count} ausgewählte Instanz ist in diesem Zeitraum nicht verfügbar",
    description:
      "Note when one retained instance selection has no samples in the window (singular; w5/m91)",
  },
  "metrics.instancesUnavailable_other": {
    message:
      "{count} ausgewählte Instanzen sind in diesem Zeitraum nicht verfügbar",
    description:
      "Note when retained instance selections have no samples in the window (plural; w5/m91)",
  },
  "metrics.noDataForSelection": {
    message: "Keine Daten für die ausgewählten Instanzen in diesem Zeitraum",
    description:
      "Empty-result copy when an explicit instance filter yields no samples — distinct from query errors (w5/m91)",
  },
  "metrics.aggregateFilter": {
    message: "Aggregation",
    description: "Application metrics replica aggregation control",
  },
  "metrics.aggregateRaw": {
    message: "Roh",
    description: "Show one series per selected instance",
  },
  "metrics.aggregateMin": {
    message: "Min",
    description: "Minimum across selected replicas at each timestamp",
  },
  "metrics.aggregateMax": {
    message: "Max",
    description: "Maximum across selected replicas at each timestamp",
  },
  "metrics.aggregateAvg": {
    message: "Durchschn.",
    description: "Average across selected replicas at each timestamp",
  },
  "metrics.rangeLast30Minutes": {
    message: "Letzte 30 Minuten",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLastHour": {
    message: "Letzte Stunde",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast4Hours": {
    message: "Letzte 4 Stunden",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast12Hours": {
    message: "Letzte 12 Stunden",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast24Hours": {
    message: "Letzte 24 Stunden",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast2Days": {
    message: "Letzte 2 Tage",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast7Days": {
    message: "Letzte 7 Tage",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast14Days": {
    message: "Letzte 14 Tage",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast30Days": {
    message: "Letzte 30 Tage",
    description:
      "Metrics time-range filter option (w5/m56; Render plan-gates it)",
  },
  "metrics.rangeCustom": {
    message: "Benutzerdefiniert…",
    description: "Time-range dropdown option that opens the start/end picker",
  },
  "metrics.rangeCustomEdit": {
    message: "Benutzerdefinierten Zeitraum bearbeiten",
    description:
      "Button beside the time-range dropdown that reopens the picker for the active custom range",
  },
  "metrics.rangeCustomTitle": {
    message: "Benutzerdefinierter Zeitraum",
    description: "Custom time-range picker dialog title",
  },
  "metrics.rangeCustomDescription": {
    message:
      "Wähle einen absoluten Start- und Endzeitpunkt. Zeiträume über 30 Tage sind nicht verfügbar.",
    description: "Custom time-range picker dialog description",
  },
  "metrics.rangeCustomStart": {
    message: "Start",
    description: "Custom range start datetime field label",
  },
  "metrics.rangeCustomEnd": {
    message: "Ende",
    description: "Custom range end datetime field label",
  },
  "metrics.rangeCustomApply": {
    message: "Anwenden",
    description: "Custom range apply button",
  },
  "metrics.rangeCustomCancel": {
    message: "Abbrechen",
    description: "Custom range cancel button",
  },
  "metrics.rangeCustomErrorOrder": {
    message: "Das Ende muss nach dem Start liegen.",
    description: "Custom range validation: end not after start",
  },
  "metrics.rangeCustomErrorTooLong": {
    message: "Wähle einen Zeitraum von maximal 30 Tagen.",
    description: "Custom range validation: window exceeds the max query window",
  },
  "metrics.rangeCustomErrorTooEarly": {
    message: "Der Zeitraum muss am oder nach dem {date} beginnen.",
    description:
      "Custom range validation: the start is before the oldest instant the range may reach (e.g. a log plan's retention)",
  },
  "metrics.retentionWindow_one": {
    message: "Metriken werden im {plan}-Tarif {count} Tag aufbewahrt.",
    description:
      "Metrics range picker hint: how many days the workspace plan keeps metrics (w7/m427, from bex-api metricsRetention)",
  },
  "metrics.retentionWindow_other": {
    message: "Metriken werden im {plan}-Tarif {count} Tage aufbewahrt.",
    description:
      "Metrics range picker hint: how many days the workspace plan keeps metrics (w7/m427, from bex-api metricsRetention)",
  },
  "metrics.retentionUpgrade_one": {
    message: "Der {plan}-Tarif bewahrt sie {count} Tag auf.",
    description:
      "Metrics range picker hint: a plan that keeps metrics longer, and its days; one sentence per longer window (w7/m427)",
  },
  "metrics.retentionUpgrade_other": {
    message: "Der {plan}-Tarif bewahrt sie {count} Tage auf.",
    description:
      "Metrics range picker hint: a plan that keeps metrics longer, and its days; one sentence per longer window (w7/m427)",
  },
  "metrics.rangeLabel": {
    message: "Zeitraum",
    description: "Accessible label for the time-range dropdown",
  },
  "metrics.eventsToggle": {
    message: "Ereignis-Timeline anzeigen",
    description: "Accessible label for the Metrics event-timeline toggle",
  },
  "metrics.latencyPlanRequired": {
    message:
      "Antwortzeit-Metriken erfordern den Pro-Tarif oder höher. Dieser Workspace nutzt Hobby.",
    description:
      "Response Times section on a Hobby workspace (PLAN_FEATURE_REQUIRED, w8/m138)",
  },
  "metrics.serviceMetricsTitle": {
    message: "Service-Metriken",
    description:
      "A project's Service Metrics view type: New View's menu, a new view's name and its page title (w3/m508)",
  },
  "metrics.projectApplicationDescription": {
    message: "Eine Linie pro Ressource, ihre Instanzen zusammengezählt",
    description:
      "Subtitle of a project Service Metrics view's Application Metrics card (w3/m508)",
  },
  "metrics.projectNetworkDescription": {
    message:
      "Anfragen, Antwortzeiten und ausgehender Datenverkehr jeder Ressource",
    description:
      "Subtitle of a project Service Metrics view's Network Metrics card (w3/m508)",
  },
  "metrics.groupByResource": {
    message: "Ressource",
    description:
      "Total Requests Group by option on a project Service Metrics view: one line per resource (Render's RESOURCE, w3/m508)",
  },
  "metrics.projectLegendLabel": {
    message: "Ressourcen in dieser Ansicht",
    description:
      "Accessible name of the legend whose buttons show or hide each resource on every chart (w3/m508)",
  },
  "metrics.projectChartFailed": {
    message:
      "{names} konnte nicht geladen werden; dieses Diagramm lässt sie aus.",
    description:
      "Under a project chart: resources whose read failed and which the chart leaves out; names is a comma list (w3/m508)",
  },
  "metrics.projectChartNotCharted": {
    message:
      "Nicht dargestellt für {names}: Dieses Diagramm hat für ihren Typ keine Daten.",
    description:
      "Under a project chart: resources of a type the chart has no data for (a static site's CPU, a worker's requests); names is a comma list (w3/m508)",
  },
  "metrics.projectChartNoData": {
    message: "Keine Daten in diesem Zeitraum für {names}.",
    description:
      "Under a project chart: resources read with nothing in the range; names is a comma list (w3/m508)",
  },
  "metrics.projectViewTooLargeTitle": {
    message: "Diese Ansicht enthält zu viele Ressourcen für Diagramme",
    description:
      "Project Service Metrics view whose filters keep more resources than one view charts (PROJECT_VIEW_TOO_LARGE, w3/m508)",
  },
  "metrics.projectViewTooLargeBody": {
    message:
      "Sie enthält {resources} Ressourcen, und eine Ansicht stellt höchstens {limit} dar. Bearbeite die Ansicht und grenze sie mit einem Filter für Umgebung, Ressource oder Ressourcentyp ein.",
    description:
      "Body of the too-many-resources state; resources is how many the view keeps, limit how many it may chart (w3/m508)",
  },
  "metrics.projectScopeEmptyTitle": {
    message: "In dieser Ansicht gibt es noch nichts darzustellen",
    description:
      "Project Service Metrics view with no resource in it (w3/m508)",
  },
  "metrics.projectScopeEmptyBody": {
    message:
      "Diese Ansicht zeigt die Services, Cronjobs, Postgres-Datenbanken und Key-Value-Instanzen des Projekts, und sie enthält noch keine.",
    description: "Body of the empty project Service Metrics view (w3/m508)",
  },
};

export default deMetrics;
