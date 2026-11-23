import type { TranslationEntry } from "@/i18n";

const esMetrics: Record<string, TranslationEntry> = {
  "metrics.cpuCores": {
    message: "{value} núcleos",
    description:
      "A CPU amount in cores (a limit or the latest usage); value is a decimal such as 0.100 (w4/566)",
  },
  "metrics.eventsTitle": {
    message: "Cronología de eventos",
    description:
      "Metrics-page timeline of service events in the selected range",
  },
  "metrics.eventsFilterLabel": {
    message: "Filtrar eventos",
    description: "Accessible label for the Metrics event category filter",
  },
  "metrics.eventsFilterAll": {
    message: "Todos los eventos",
    description: "All event categories",
  },
  "metrics.eventsFilterDeploys": {
    message: "Despliegues",
    description: "Deploy events category",
  },
  "metrics.eventsFilterLifecycle": {
    message: "Ciclo de vida",
    description: "Lifecycle events category",
  },
  "metrics.eventsFilterConfig": {
    message: "Configuración",
    description: "Configuration events category",
  },
  "metrics.eventsLoading": {
    message: "Cargando eventos…",
    description: "Event timeline loading state",
  },
  "metrics.eventsError": {
    message: "No se pudieron cargar los eventos.",
    description: "Event timeline error state",
  },
  "metrics.eventsEmptyTitle": {
    message: "Sin eventos",
    description: "Event timeline empty-state title",
  },
  "metrics.eventsEmptyBody": {
    message: "Ha habido calma en este intervalo de tiempo.",
    description: "Event timeline empty-state body",
  },
  "metrics.eventsChanged": {
    message: "Servicio modificado",
    description: "Fallback event timeline label",
  },
  "metrics.eventMarkersLabel": {
    message: "Eventos del servicio",
    description: "Accessible label for the event-marker strip above a chart",
  },
  "metrics.eventMarkerCluster_one": {
    message: "{count} evento",
    description:
      "Accessible label for a clustered chart event marker (count badge, singular)",
  },
  "metrics.eventMarkerCluster_many": {
    message: "{count} de eventos",
    description:
      "Accessible label for a clustered chart event marker (count badge, plural)",
  },
  "metrics.eventMarkerCluster_other": {
    message: "{count} eventos",
    description:
      "Accessible label for a clustered chart event marker (count badge, plural)",
  },
  "metrics.lineChartLabel_one": {
    message: "Gráfico de líneas con {count} punto de datos",
    description:
      "Accessible name of a metrics line chart; count is the number of distinct sample times (singular)",
  },
  "metrics.lineChartLabel_many": {
    message: "Gráfico de líneas con {count} de puntos de datos",
    description:
      "Accessible name of a metrics line chart; count is the number of distinct sample times (plural)",
  },
  "metrics.lineChartLabel_other": {
    message: "Gráfico de líneas con {count} puntos de datos",
    description:
      "Accessible name of a metrics line chart; count is the number of distinct sample times (plural)",
  },
  "metrics.barChartLabel_one": {
    message: "Gráfico de barras con {count} punto de datos",
    description:
      "Accessible name of a metrics bar chart; count is the number of time buckets (singular)",
  },
  "metrics.barChartLabel_many": {
    message: "Gráfico de barras con {count} de puntos de datos",
    description:
      "Accessible name of a metrics bar chart; count is the number of time buckets (plural)",
  },
  "metrics.barChartLabel_other": {
    message: "Gráfico de barras con {count} puntos de datos",
    description:
      "Accessible name of a metrics bar chart; count is the number of time buckets (plural)",
  },
  "metrics.barValueAt": {
    message: "{value} a las {time}",
    description:
      "Accessible name of one focusable bar: its formatted value (e.g. 42 or 1.2K) and its local sample time",
  },
  "metrics.chartEmpty": {
    message: "Sin datos en el intervalo",
    description:
      "Default empty state of a metrics chart when the settled query returned no samples in the selected time range",
  },
  "metrics.eventMarkerHint": {
    message: "Haz clic para ver los detalles",
    description: "Footer line of the chart event-marker hover tooltip",
  },
  "metrics.eventDeployStarted": {
    message: "Despliegue iniciado",
    description: "Chart event-marker label: a deploy began",
  },
  "metrics.eventDeployLiveFor": {
    message: "Despliegue activo para {commit}",
    description:
      "Chart event-marker label: a deploy went live, with its commit short id",
  },
  "metrics.eventDeployLive": {
    message: "Despliegue activo",
    description: "Chart event-marker label: a deploy went live (no commit id)",
  },
  "metrics.eventDeployFailed": {
    message: "Despliegue fallido",
    description: "Chart event-marker label: a deploy failed",
  },
  "metrics.eventDeployCanceled": {
    message: "Despliegue cancelado",
    description: "Chart event-marker label: a deploy was canceled",
  },
  "metrics.eventDeployEnded": {
    message: "Despliegue finalizado",
    description:
      "Chart event-marker label: a deploy finished with an unrecognized status",
  },
  "metrics.eventBuildStarted": {
    message: "Build iniciado",
    description: "Chart event-marker label: an image build began",
  },
  "metrics.eventBuildEnded": {
    message: "Build finalizado",
    description: "Chart event-marker label: an image build finished",
  },
  "metrics.eventPreDeployStarted": {
    message: "Comando previo al despliegue iniciado",
    description: "Chart event-marker label: the pre-deploy command began",
  },
  "metrics.eventPreDeployEnded": {
    message: "Comando previo al despliegue finalizado",
    description: "Chart event-marker label: the pre-deploy command finished",
  },
  "metrics.eventJobRunEnded": {
    message: "Tarea finalizada",
    description: "Chart event-marker label: a one-off job finished",
  },
  "metrics.eventBranchDeleted": {
    message: "Rama eliminada",
    description: "Chart event-marker label: the tracked Git branch was deleted",
  },
  "metrics.eventServerRestarted": {
    message: "Servidor reiniciado",
    description: "Chart event-marker label: the service was restarted",
  },
  "metrics.eventSuspended": {
    message: "Servicio suspendido",
    description: "Chart event-marker label: the service was suspended",
  },
  "metrics.eventResumed": {
    message: "Servicio reanudado",
    description: "Chart event-marker label: the service was resumed",
  },
  "metrics.eventInstanceCountChanged": {
    message: "Número de instancias modificado",
    description: "Chart event-marker label: manual scaling changed the count",
  },
  "metrics.eventAutoscalingChanged": {
    message: "Configuración de escalado automático modificada",
    description: "Chart event-marker label: autoscaling settings changed",
  },
  "metrics.eventPlanChanged": {
    message: "Tipo de instancia modificado",
    description: "Chart event-marker label: the service's plan changed",
  },
  "metrics.networkTitle": {
    message: "Métricas de red",
    description: "Network metrics card title",
  },
  "metrics.networkDescription": {
    message: "Agregado de todas las instancias",
    description: "Network metrics card description",
  },
  "metrics.totalRequests": {
    message: "Total de solicitudes",
    description: "Network metrics chart section title",
  },
  "metrics.responseTimes": {
    message: "Tiempos de respuesta",
    description: "Network metrics chart section title",
  },
  "metrics.percentile": {
    message: "Percentil",
    description: "Response Times section control label",
  },
  "metrics.percentileAll": {
    message: "Todos",
    description:
      "Percentile dropdown option that overlays p50/p90/p99 on one chart",
  },
  "metrics.requestsCount_one": {
    message: "{formatted} solicitud",
    description:
      "Aggregate request count beside the Total Requests section title (singular; {formatted} is the locale-formatted count)",
  },
  "metrics.requestsCount_many": {
    message: "{formatted} de solicitudes",
    description:
      "Aggregate request count beside the Total Requests section title (plural; {formatted} is the locale-formatted count)",
  },
  "metrics.requestsCount_other": {
    message: "{formatted} solicitudes",
    description:
      "Aggregate request count beside the Total Requests section title (plural; {formatted} is the locale-formatted count)",
  },
  "metrics.outboundBandwidth": {
    message: "Ancho de banda saliente",
    description: "Network metrics chart section title",
  },
  "metrics.monthToDateBandwidth": {
    message: "{amount} usados este mes",
    description:
      "Footer under the Outbound Bandwidth chart, showing month-to-date egress",
  },
  "metrics.monthToDateBandwidthError": {
    message:
      "No se pudo cargar el ancho de banda de este mes; prueba a actualizar",
    description:
      "Month-to-date bandwidth footer when the query failed with no value — distinct from a known zero and from the chart's bandwidthError (w4/090)",
  },
  "metrics.bandwidthDegraded": {
    message: "Datos parciales",
    description:
      "Badge on the Outbound Bandwidth chart when an egress source's health check failed inside the window (w1/m50)",
  },
  "metrics.bandwidthDegradedDetail": {
    message:
      "La fuente de salida {sources} no estuvo disponible durante parte de este intervalo, por lo que el ancho de banda podría estar subestimado.",
    description:
      "Tooltip expanding the Partial data badge; {sources} is the comma-joined raw source tokens (http/websocket/direct)",
  },
  "metrics.bandwidthError": {
    message: "No se pudo cargar el ancho de banda; prueba a actualizar",
    description:
      "Bandwidth chart body when the query itself failed (network/server error, not an empty window)",
  },
  "metrics.sourceNotConfigured": {
    message: "Fuente de métricas no configurada",
    description: "Shown when bex-api reports no metrics backend is wired up",
  },
  "metrics.chartError": {
    message: "No se pudo cargar esta métrica",
    description:
      "Shown when a metrics query fails with a real error (not a 503/no-data) — distinct from the empty 'No data in range' state (w9/m86)",
  },
  "metrics.chartThrottled": {
    message: "Poniéndose al día: el tráfico se limitó brevemente",
    description:
      "Cold-load state when a metrics read was shed (RATE_LIMITED / AUTH_OVERLOADED) with no cached series — transient, not 'metrics unavailable' (w4/m100)",
  },
  "metrics.chartStaleThrottled": {
    message:
      "Mostrando datos en caché; se actualizarán tras una breve limitación",
    description:
      "Warm-cache banner when a metrics refetch was throttled; cached series remain visible but must not read as current (w4/m100)",
  },
  "metrics.applicationTitle": {
    message: "Métricas de la aplicación",
    description: "Application metrics card title",
  },
  "metrics.limitLink": {
    message: "Límite",
    description:
      "Link to the Compute page on the Memory/CPU chart headers, followed by the limit value when configured",
  },
  "metrics.manageScaling": {
    message: "Gestionar escalado",
    description:
      "Link to the Compute page's Manual Scaling section on the Total Instances chart header",
  },
  "metrics.noLimitConfigured": {
    message: "Sin límite configurado: el porcentaje no está definido",
    description:
      "Shown instead of a percentage chart when the App's pods declare no resource limit",
  },
  "metrics.limitsVary": {
    message: "Los límites varían",
    description:
      "Memory/CPU chart header annotation when the selected replicas declare different limits — each replica is percentized against its own limit, so no single limit value applies",
  },
  "metrics.percentageUnavailable": {
    message:
      "Porcentajes no disponibles: no hay un límite fiable para este intervalo",
    description:
      "Shown instead of a percentage chart when usage samples exist but every denominator was missing, zero, or otherwise untrustworthy (deleted pods, predated limit history, a mid-window rollout gap) — distinct from no usage samples or a source failure",
  },
  "metrics.targetLabel": {
    message: "Objetivo {value}",
    description:
      "Autoscale-target annotation on the Memory/CPU chart headers (percentage mode only), e.g. 'Target 70%'",
  },
  "metrics.diskTitle": {
    message: "Disco",
    description: "Datastore metrics panel: disk usage chart section title",
  },
  "metrics.diskCapacityLabel": {
    message: "Capacidad {value}",
    description:
      "Disk usage chart header annotation showing the datastore's billed/provisioned disk size",
  },
  "metrics.connectionsTitle": {
    message: "Conexiones activas",
    description:
      "Datastore metrics panel: active-connections chart section title (Postgres client connections; Key Value connected clients), as Render titles both",
  },
  "metrics.memoryTitle": {
    message: "Memoria",
    description:
      "Datastore metrics panel: memory chart section title (Postgres container memory; Key Value used memory)",
  },
  "metrics.datastoreCpuTitle": {
    message: "CPU",
    description: "Datastore metrics panel: CPU usage chart section title",
  },
  "metrics.datastoreLimitLabel": {
    message: "Límite {value}",
    description:
      "Datastore CPU/memory/connections chart header annotation showing the instance's configured limit (a Key Value's connection limit is its plan's)",
  },
  "metrics.replicationLagTitle": {
    message: "Retraso de replicación",
    description:
      "Datastore metrics panel: Postgres replication-lag chart section title",
  },
  "metrics.diskActivityTitle": {
    message: "Actividad de disco",
    description:
      "Datastore metrics panel: Render's Disk Activity chart title — bytes read from and written to the data volume per bucket (Postgres, Key Value, a service's disk; w7/m430)",
  },
  "metrics.diskOperationsTitle": {
    message: "Operaciones de disco",
    description:
      "Datastore metrics panel: Render's Disk Operations chart title — read and write operations on the data volume per bucket (Postgres, a service's disk; w7/m430)",
  },
  "metrics.networkActivityTitle": {
    message: "Actividad de red",
    description:
      "Datastore metrics panel: Render's Network Activity chart title — bytes a Postgres read from and wrote to the network per bucket (w7/m430)",
  },
  "metrics.transactionVolumeTitle": {
    message: "Volumen de transacciones",
    description:
      "Datastore metrics panel: Render's Transaction Volume chart title — transactions a Postgres executed per bucket (w7/m430)",
  },
  "metrics.lockDelayedQueriesTitle": {
    message: "Consultas retrasadas por bloqueos",
    description:
      "Datastore metrics panel: Render's Lock-Delayed Queries chart title — completed queries that waited a second or more on a lock (w7/m430)",
  },
  "metrics.directionRead": {
    message: "Lectura",
    description:
      "Legend entry of the read half of a two-series chart (data read from disk or the network)",
  },
  "metrics.directionWrite": {
    message: "Escritura",
    description:
      "Legend entry of the write half of a two-series chart (data written to disk or the network)",
  },
  "metrics.seriesInstanceDirection": {
    message: "{instance} · {direction}",
    description:
      "Legend entry of a two-series chart when the datastore has several instances: the instance id, then Read or Write",
  },
  "metrics.lockDelayedStoreUnavailable": {
    message:
      "Las consultas retrasadas por bloqueos requieren el almacén de logs",
    description:
      "Shown instead of the Lock-Delayed Queries chart on a deployment with no durable log store (503): the count comes from Postgres log lines",
  },
  "metrics.replicationLagPendingHA": {
    message:
      "N/D: no hay réplica (activa la alta disponibilidad para ver el retraso de replicación)",
    description:
      "Shown instead of a chart for replication lag before Postgres HA is enabled (w1/m22)",
  },
  "metrics.datastoreMetricsTitle": {
    message: "Métricas",
    description: "Database/Key Value detail page metrics panel card title",
  },
  "metrics.datastoreMetricsDescription": {
    message: "Uso de recursos en directo de esta instancia",
    description:
      "Database/Key Value detail page metrics panel card description",
  },
  "metrics.statusCode": {
    message: "Código de estado",
    description: "Toolbar filter label for HTTP status code",
  },
  "metrics.statusCodeAll": {
    message: "Todos",
    description: "Status-code filter option meaning no filtering",
  },
  "metrics.host": {
    message: "Host",
    description: "Network-card filter label for the request Host",
  },
  "metrics.hostAll": {
    message: "Todos",
    description: "Host filter option meaning no filtering",
  },
  "metrics.path": {
    message: "Ruta",
    description: "Network-card filter label for the request Path",
  },
  "metrics.pathPlaceholder": {
    message: "Ruta…",
    description: "Placeholder for the free-text Path filter input",
  },
  "metrics.pathClear": {
    message: "Borrar filtro de ruta",
    description: "Accessible label for the button that clears the Path filter",
  },
  "metrics.hostPathStoreUnavailable": {
    message:
      "Los filtros de host y ruta, y el desglose por host, requieren el almacén de logs",
    description:
      "Per-section state when a host/path-filtered or host-grouped request metric hits a deployment with no durable log store (503) — served from the request-log store (Loki), unavailable here",
  },
  "metrics.groupBy": {
    message: "Agrupar por",
    description: "Total Requests chart control label",
  },
  "metrics.groupByAllRequests": {
    message: "Todas las solicitudes",
    description: "Group-by option: one aggregate series",
  },
  "metrics.groupByStatus": {
    message: "Código de estado",
    description: "Group-by option: one series per HTTP status code",
  },
  "metrics.groupByMethod": {
    message: "Método",
    description: "Group-by option: one series per HTTP method",
  },
  "metrics.groupByHost": {
    message: "Host",
    description: "Group-by option: one series per request host",
  },
  "metrics.memory": {
    message: "Memoria",
    description: "Application metrics stat tile label",
  },
  "metrics.cpu": {
    message: "CPU",
    description: "Application metrics stat tile label",
  },
  "metrics.totalInstances": {
    message: "Total de instancias",
    description: "Application metrics stat tile label",
  },
  "metrics.filterPercentage": {
    message: "Porcentaje",
    description: "Metrics filter tab: show values as a percentage",
  },
  "metrics.filterTotal": {
    message: "Total",
    description: "Metrics filter tab: show values as an absolute total",
  },
  "metrics.instanceFilter": {
    message: "Instancias",
    description: "Application metrics instance multi-select label",
  },
  "metrics.instanceFilterAll": {
    message: "Todas",
    description:
      "Instances filter trigger's value when no instance is picked (every instance is shown)",
  },
  "metrics.instanceShowAll": {
    message: "Mostrar todas",
    description:
      "Explicit action that clears the retained instance selection back to all instances (w5/m91)",
  },
  "metrics.instanceUnavailableSuffix": {
    message: " (no disponible)",
    description:
      "Suffix appended to a retained instance option that is no longer in the window or discovery (w5/m91)",
  },
  "metrics.instancesUnavailable_one": {
    message:
      "{count} instancia seleccionada no está disponible en este intervalo",
    description:
      "Note when one retained instance selection has no samples in the window (singular; w5/m91)",
  },
  "metrics.instancesUnavailable_many": {
    message:
      "{count} de instancias seleccionadas no están disponibles en este intervalo",
    description:
      "Note when retained instance selections have no samples in the window (plural; w5/m91)",
  },
  "metrics.instancesUnavailable_other": {
    message:
      "{count} instancias seleccionadas no están disponibles en este intervalo",
    description:
      "Note when retained instance selections have no samples in the window (plural; w5/m91)",
  },
  "metrics.noDataForSelection": {
    message: "Sin datos para las instancias seleccionadas en este intervalo",
    description:
      "Empty-result copy when an explicit instance filter yields no samples — distinct from query errors (w5/m91)",
  },
  "metrics.aggregateFilter": {
    message: "Agregación",
    description: "Application metrics replica aggregation control",
  },
  "metrics.aggregateRaw": {
    message: "Sin procesar",
    description: "Show one series per selected instance",
  },
  "metrics.aggregateMin": {
    message: "Mín.",
    description: "Minimum across selected replicas at each timestamp",
  },
  "metrics.aggregateMax": {
    message: "Máx.",
    description: "Maximum across selected replicas at each timestamp",
  },
  "metrics.aggregateAvg": {
    message: "Prom.",
    description: "Average across selected replicas at each timestamp",
  },
  "metrics.rangeLast30Minutes": {
    message: "Últimos 30 minutos",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLastHour": {
    message: "Última hora",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast4Hours": {
    message: "Últimas 4 horas",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast12Hours": {
    message: "Últimas 12 horas",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast24Hours": {
    message: "Últimas 24 horas",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast2Days": {
    message: "Últimos 2 días",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast7Days": {
    message: "Últimos 7 días",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast14Days": {
    message: "Últimos 14 días",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast30Days": {
    message: "Últimos 30 días",
    description:
      "Metrics time-range filter option (w5/m56; Render plan-gates it)",
  },
  "metrics.rangeCustom": {
    message: "Personalizado…",
    description: "Time-range dropdown option that opens the start/end picker",
  },
  "metrics.rangeCustomEdit": {
    message: "Editar intervalo personalizado",
    description:
      "Button beside the time-range dropdown that reopens the picker for the active custom range",
  },
  "metrics.rangeCustomTitle": {
    message: "Intervalo personalizado",
    description: "Custom time-range picker dialog title",
  },
  "metrics.rangeCustomDescription": {
    message:
      "Elige un inicio y un fin absolutos. No se admiten intervalos de más de 30 días.",
    description: "Custom time-range picker dialog description",
  },
  "metrics.rangeCustomStart": {
    message: "Inicio",
    description: "Custom range start datetime field label",
  },
  "metrics.rangeCustomEnd": {
    message: "Fin",
    description: "Custom range end datetime field label",
  },
  "metrics.rangeCustomApply": {
    message: "Aplicar",
    description: "Custom range apply button",
  },
  "metrics.rangeCustomCancel": {
    message: "Cancelar",
    description: "Custom range cancel button",
  },
  "metrics.rangeCustomErrorOrder": {
    message: "El fin debe ser posterior al inicio.",
    description: "Custom range validation: end not after start",
  },
  "metrics.rangeCustomErrorTooLong": {
    message: "Elige un intervalo de 30 días o menos.",
    description: "Custom range validation: window exceeds the max query window",
  },
  "metrics.rangeCustomErrorTooEarly": {
    message: "El rango debe empezar el {date} o después.",
    description:
      "Custom range validation: the start is before the oldest instant the range may reach (e.g. a log plan's retention)",
  },
  "metrics.retentionWindow_one": {
    message: "Las métricas se conservan {count} día en el plan {plan}.",
    description:
      "Metrics range picker hint: how many days the workspace plan keeps metrics (w7/m427, from bex-api metricsRetention)",
  },
  "metrics.retentionWindow_many": {
    message: "Las métricas se conservan {count} de días en el plan {plan}.",
    description:
      "Metrics range picker hint: how many days the workspace plan keeps metrics (w7/m427, from bex-api metricsRetention)",
  },
  "metrics.retentionWindow_other": {
    message: "Las métricas se conservan {count} días en el plan {plan}.",
    description:
      "Metrics range picker hint: how many days the workspace plan keeps metrics (w7/m427, from bex-api metricsRetention)",
  },
  "metrics.retentionUpgrade_one": {
    message: "El plan {plan} las conserva {count} día.",
    description:
      "Metrics range picker hint: a plan that keeps metrics longer, and its days; one sentence per longer window (w7/m427)",
  },
  "metrics.retentionUpgrade_many": {
    message: "El plan {plan} las conserva {count} de días.",
    description:
      "Metrics range picker hint: a plan that keeps metrics longer, and its days; one sentence per longer window (w7/m427)",
  },
  "metrics.retentionUpgrade_other": {
    message: "El plan {plan} las conserva {count} días.",
    description:
      "Metrics range picker hint: a plan that keeps metrics longer, and its days; one sentence per longer window (w7/m427)",
  },
  "metrics.rangeLabel": {
    message: "Intervalo de tiempo",
    description: "Accessible label for the time-range dropdown",
  },
  "metrics.eventsToggle": {
    message: "Mostrar cronología de eventos",
    description: "Accessible label for the Metrics event-timeline toggle",
  },
  "metrics.latencyPlanRequired": {
    message:
      "Las métricas de latencia de respuesta requieren el plan Pro o superior. Este espacio de trabajo tiene el plan Hobby.",
    description:
      "Response Times section on a Hobby workspace (PLAN_FEATURE_REQUIRED, w8/m138)",
  },
  "metrics.serviceMetricsTitle": {
    message: "Métricas de servicios",
    description:
      "A project's Service Metrics view type: New View's menu, a new view's name and its page title (w3/m508)",
  },
  "metrics.projectApplicationDescription": {
    message: "Una línea por recurso, con sus instancias sumadas",
    description:
      "Subtitle of a project Service Metrics view's Application Metrics card (w3/m508)",
  },
  "metrics.projectNetworkDescription": {
    message:
      "Solicitudes, tiempos de respuesta y tráfico saliente de cada recurso",
    description:
      "Subtitle of a project Service Metrics view's Network Metrics card (w3/m508)",
  },
  "metrics.groupByResource": {
    message: "Recurso",
    description:
      "Total Requests Group by option on a project Service Metrics view: one line per resource (Render's RESOURCE, w3/m508)",
  },
  "metrics.projectLegendLabel": {
    message: "Recursos de esta vista",
    description:
      "Accessible name of the legend whose buttons show or hide each resource on every chart (w3/m508)",
  },
  "metrics.projectChartFailed": {
    message: "No se pudo cargar {names}; este gráfico los deja fuera.",
    description:
      "Under a project chart: resources whose read failed and which the chart leaves out; names is a comma list (w3/m508)",
  },
  "metrics.projectChartNotCharted": {
    message:
      "Sin gráfico para {names}: este gráfico no tiene datos para su tipo.",
    description:
      "Under a project chart: resources of a type the chart has no data for (a static site's CPU, a worker's requests); names is a comma list (w3/m508)",
  },
  "metrics.projectChartNoData": {
    message: "Sin datos en este intervalo para {names}.",
    description:
      "Under a project chart: resources read with nothing in the range; names is a comma list (w3/m508)",
  },
  "metrics.projectViewTooLargeTitle": {
    message: "Esta vista tiene demasiados recursos para graficar",
    description:
      "Project Service Metrics view whose filters keep more resources than one view charts (PROJECT_VIEW_TOO_LARGE, w3/m508)",
  },
  "metrics.projectViewTooLargeBody": {
    message:
      "Tiene {resources} recursos, y una vista grafica como máximo {limit}. Edita la vista para acotarla con un filtro de entorno, recurso o tipo de recurso.",
    description:
      "Body of the too-many-resources state; resources is how many the view keeps, limit how many it may chart (w3/m508)",
  },
  "metrics.projectScopeEmptyTitle": {
    message: "Todavía no hay nada que graficar en esta vista",
    description:
      "Project Service Metrics view with no resource in it (w3/m508)",
  },
  "metrics.projectScopeEmptyBody": {
    message:
      "Esta vista grafica los servicios, cron jobs, bases de datos Postgres e instancias Key Value del proyecto, y todavía no hay ninguno.",
    description: "Body of the empty project Service Metrics view (w3/m508)",
  },
  "metrics.viewActionsFor": {
    message: "Ver acciones para {instance}",
    description:
      "Tooltip hint at the bottom of a Memory or CPU chart when hovering an instance's series (w4/717; Render's 'View actions for <slug>')",
  },
  "metrics.instanceActionsMenu": {
    message: "Acciones para la instancia {instance}",
    description:
      "Accessible name of the instance action card and legend menu trigger on Memory and CPU charts (w4/717)",
  },
  "metrics.filterToInstance": {
    message: "Filtrar por instancia",
    description:
      "Chart instance menu item: filters the Application Metrics card to this instance (w4/717; Render's 'Filter to instance')",
  },
  "metrics.resetInstanceFilter": {
    message: "Restablecer filtro de instancia",
    description:
      "Chart instance menu item: removes this instance from the card's Instances filter (w4/717; Render's 'Reset instance filter')",
  },
  "metrics.viewInstanceLogs": {
    message: "Ver logs de la instancia",
    description:
      "Chart instance menu item: opens the service's Logs tab filtered to this instance in a new tab (w4/717; Render's 'View instance logs')",
  },
  "metrics.copyInstanceId": {
    message: "Copiar ID de instancia",
    description:
      "Chart instance menu item: copies the instance's full ID to the clipboard (w4/717; Render's 'Copy instance ID')",
  },
  "metrics.instanceIdCopied": {
    message: "ID de instancia copiado",
    description:
      "Toast after Copy instance ID wrote the instance ID to the clipboard (w4/717)",
  },
  "metrics.instanceIdCopyFailed": {
    message: "No se pudo copiar el ID de la instancia",
    description:
      "Toast when the clipboard refused Copy instance ID (w4/717)",
  },
};

export default esMetrics;
