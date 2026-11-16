import type { TranslationEntry } from "@/i18n";

const ptMetrics: Record<string, TranslationEntry> = {
  "metrics.cpuCores": {
    message: "{value} núcleos",
    description:
      "A CPU amount in cores (a limit or the latest usage); value is a decimal such as 0.100 (w4/566)",
  },
  "metrics.eventsTitle": {
    message: "Linha do tempo de eventos",
    description:
      "Metrics-page timeline of service events in the selected range",
  },
  "metrics.eventsFilterLabel": {
    message: "Filtrar eventos",
    description: "Accessible label for the Metrics event category filter",
  },
  "metrics.eventsFilterAll": {
    message: "Todos os eventos",
    description: "All event categories",
  },
  "metrics.eventsFilterDeploys": {
    message: "Deploys",
    description: "Deploy events category",
  },
  "metrics.eventsFilterLifecycle": {
    message: "Ciclo de vida",
    description: "Lifecycle events category",
  },
  "metrics.eventsFilterConfig": {
    message: "Configuração",
    description: "Configuration events category",
  },
  "metrics.eventsLoading": {
    message: "Carregando eventos…",
    description: "Event timeline loading state",
  },
  "metrics.eventsError": {
    message: "Não foi possível carregar os eventos.",
    description: "Event timeline error state",
  },
  "metrics.eventsEmptyTitle": {
    message: "Nenhum evento",
    description: "Event timeline empty-state title",
  },
  "metrics.eventsEmptyBody": {
    message: "Está tudo calmo neste intervalo de tempo.",
    description: "Event timeline empty-state body",
  },
  "metrics.eventsChanged": {
    message: "Serviço alterado",
    description: "Fallback event timeline label",
  },
  "metrics.eventMarkersLabel": {
    message: "Eventos do serviço",
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
    message: "Gráfico de linhas com {count} ponto de dados",
    description:
      "Accessible name of a metrics line chart; count is the number of distinct sample times (singular)",
  },
  "metrics.lineChartLabel_many": {
    message: "Gráfico de linhas com {count} de pontos de dados",
    description:
      "Accessible name of a metrics line chart; count is the number of distinct sample times (plural)",
  },
  "metrics.lineChartLabel_other": {
    message: "Gráfico de linhas com {count} pontos de dados",
    description:
      "Accessible name of a metrics line chart; count is the number of distinct sample times (plural)",
  },
  "metrics.barChartLabel_one": {
    message: "Gráfico de barras com {count} ponto de dados",
    description:
      "Accessible name of a metrics bar chart; count is the number of time buckets (singular)",
  },
  "metrics.barChartLabel_many": {
    message: "Gráfico de barras com {count} de pontos de dados",
    description:
      "Accessible name of a metrics bar chart; count is the number of time buckets (plural)",
  },
  "metrics.barChartLabel_other": {
    message: "Gráfico de barras com {count} pontos de dados",
    description:
      "Accessible name of a metrics bar chart; count is the number of time buckets (plural)",
  },
  "metrics.barValueAt": {
    message: "{value} em {time}",
    description:
      "Accessible name of one focusable bar: its formatted value (e.g. 42 or 1.2K) and its local sample time",
  },
  "metrics.chartEmpty": {
    message: "Sem dados no intervalo",
    description:
      "Default empty state of a metrics chart when the settled query returned no samples in the selected time range",
  },
  "metrics.eventMarkerHint": {
    message: "Clique para ver os detalhes",
    description: "Footer line of the chart event-marker hover tooltip",
  },
  "metrics.eventDeployStarted": {
    message: "Deploy iniciado",
    description: "Chart event-marker label: a deploy began",
  },
  "metrics.eventDeployLiveFor": {
    message: "Deploy no ar para {commit}",
    description:
      "Chart event-marker label: a deploy went live, with its commit short id",
  },
  "metrics.eventDeployLive": {
    message: "Deploy no ar",
    description: "Chart event-marker label: a deploy went live (no commit id)",
  },
  "metrics.eventDeployFailed": {
    message: "Deploy com falha",
    description: "Chart event-marker label: a deploy failed",
  },
  "metrics.eventDeployCanceled": {
    message: "Deploy cancelado",
    description: "Chart event-marker label: a deploy was canceled",
  },
  "metrics.eventDeployEnded": {
    message: "Deploy encerrado",
    description:
      "Chart event-marker label: a deploy finished with an unrecognized status",
  },
  "metrics.eventBuildStarted": {
    message: "Build iniciado",
    description: "Chart event-marker label: an image build began",
  },
  "metrics.eventBuildEnded": {
    message: "Build encerrado",
    description: "Chart event-marker label: an image build finished",
  },
  "metrics.eventPreDeployStarted": {
    message: "Pré-deploy iniciado",
    description: "Chart event-marker label: the pre-deploy command began",
  },
  "metrics.eventPreDeployEnded": {
    message: "Pré-deploy encerrado",
    description: "Chart event-marker label: the pre-deploy command finished",
  },
  "metrics.eventJobRunEnded": {
    message: "Job encerrado",
    description: "Chart event-marker label: a one-off job finished",
  },
  "metrics.eventBranchDeleted": {
    message: "Branch excluído",
    description: "Chart event-marker label: the tracked Git branch was deleted",
  },
  "metrics.eventServerRestarted": {
    message: "Servidor reiniciado",
    description: "Chart event-marker label: the service was restarted",
  },
  "metrics.eventSuspended": {
    message: "Serviço suspenso",
    description: "Chart event-marker label: the service was suspended",
  },
  "metrics.eventResumed": {
    message: "Serviço retomado",
    description: "Chart event-marker label: the service was resumed",
  },
  "metrics.eventInstanceCountChanged": {
    message: "Número de instâncias alterado",
    description: "Chart event-marker label: manual scaling changed the count",
  },
  "metrics.eventAutoscalingChanged": {
    message: "Configuração de escalonamento automático alterada",
    description: "Chart event-marker label: autoscaling settings changed",
  },
  "metrics.eventPlanChanged": {
    message: "Tipo de instância alterado",
    description: "Chart event-marker label: the service's plan changed",
  },
  "metrics.networkTitle": {
    message: "Métricas de rede",
    description: "Network metrics card title",
  },
  "metrics.networkDescription": {
    message: "Agregadas de todas as instâncias",
    description: "Network metrics card description",
  },
  "metrics.totalRequests": {
    message: "Total de requisições",
    description: "Network metrics chart section title",
  },
  "metrics.responseTimes": {
    message: "Tempos de resposta",
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
    message: "{formatted} requisição",
    description:
      "Aggregate request count beside the Total Requests section title (singular; {formatted} is the locale-formatted count)",
  },
  "metrics.requestsCount_many": {
    message: "{formatted} de requisições",
    description:
      "Aggregate request count beside the Total Requests section title (plural; {formatted} is the locale-formatted count)",
  },
  "metrics.requestsCount_other": {
    message: "{formatted} requisições",
    description:
      "Aggregate request count beside the Total Requests section title (plural; {formatted} is the locale-formatted count)",
  },
  "metrics.outboundBandwidth": {
    message: "Largura de banda de saída",
    description: "Network metrics chart section title",
  },
  "metrics.monthToDateBandwidth": {
    message: "{amount} usados neste mês",
    description:
      "Footer under the Outbound Bandwidth chart, showing month-to-date egress",
  },
  "metrics.monthToDateBandwidthError": {
    message:
      "Não foi possível carregar a largura de banda deste mês — tente atualizar",
    description:
      "Month-to-date bandwidth footer when the query failed with no value — distinct from a known zero and from the chart's bandwidthError (w4/090)",
  },
  "metrics.bandwidthDegraded": {
    message: "Dados parciais",
    description:
      "Badge on the Outbound Bandwidth chart when an egress source's health check failed inside the window (w1/m50)",
  },
  "metrics.bandwidthDegradedDetail": {
    message:
      "A origem de saída {sources} esteve instável durante parte deste período — a largura de banda pode estar subcontada.",
    description:
      "Tooltip expanding the Partial data badge; {sources} is the comma-joined raw source tokens (http/websocket/direct)",
  },
  "metrics.bandwidthError": {
    message: "Não foi possível carregar a largura de banda — tente atualizar",
    description:
      "Bandwidth chart body when the query itself failed (network/server error, not an empty window)",
  },
  "metrics.sourceNotConfigured": {
    message: "Fonte de métricas não configurada",
    description: "Shown when bex-api reports no metrics backend is wired up",
  },
  "metrics.chartError": {
    message: "Não foi possível carregar esta métrica",
    description:
      "Shown when a metrics query fails with a real error (not a 503/no-data) — distinct from the empty 'No data in range' state (w9/m86)",
  },
  "metrics.chartThrottled": {
    message: "Recuperando o atraso — o tráfego foi limitado por um instante",
    description:
      "Cold-load state when a metrics read was shed (RATE_LIMITED / AUTH_OVERLOADED) with no cached series — transient, not 'metrics unavailable' (w4/m100)",
  },
  "metrics.chartStaleThrottled": {
    message: "Exibindo dados em cache — atualizando após um limite breve",
    description:
      "Warm-cache banner when a metrics refetch was throttled; cached series remain visible but must not read as current (w4/m100)",
  },
  "metrics.applicationTitle": {
    message: "Métricas da aplicação",
    description: "Application metrics card title",
  },
  "metrics.limitLink": {
    message: "Limite",
    description:
      "Link to the Instance Type tab on the Memory/CPU chart headers, followed by the limit value when configured",
  },
  "metrics.manageScaling": {
    message: "Gerenciar escalonamento",
    description: "Link to the Scaling tab on the Total Instances chart header",
  },
  "metrics.noLimitConfigured": {
    message: "Nenhum limite configurado — a porcentagem é indefinida",
    description:
      "Shown instead of a percentage chart when the App's pods declare no resource limit",
  },
  "metrics.limitsVary": {
    message: "Os limites variam",
    description:
      "Memory/CPU chart header annotation when the selected replicas declare different limits — each replica is percentized against its own limit, so no single limit value applies",
  },
  "metrics.percentageUnavailable": {
    message:
      "Porcentagens indisponíveis — não há um limite confiável para este período",
    description:
      "Shown instead of a percentage chart when usage samples exist but every denominator was missing, zero, or otherwise untrustworthy (deleted pods, predated limit history, a mid-window rollout gap) — distinct from no usage samples or a source failure",
  },
  "metrics.targetLabel": {
    message: "Meta {value}",
    description:
      "Autoscale-target annotation on the Memory/CPU chart headers (percentage mode only), e.g. 'Target 70%'",
  },
  "metrics.diskTitle": {
    message: "Disco",
    description: "Datastore metrics panel: disk usage chart section title",
  },
  "metrics.diskCapacityLabel": {
    message: "Capacidade {value}",
    description:
      "Disk usage chart header annotation showing the datastore's billed/provisioned disk size",
  },
  "metrics.connectionsTitle": {
    message: "Conexões ativas",
    description:
      "Datastore metrics panel: Postgres active-connections chart section title",
  },
  "metrics.memoryTitle": {
    message: "Memória",
    description:
      "Datastore metrics panel: memory chart section title (Postgres container memory; Key Value used memory)",
  },
  "metrics.datastoreCpuTitle": {
    message: "CPU",
    description: "Datastore metrics panel: CPU usage chart section title",
  },
  "metrics.datastoreLimitLabel": {
    message: "Limite {value}",
    description:
      "Datastore CPU/memory chart header annotation showing the instance's configured limit",
  },
  "metrics.kvConnectionsTitle": {
    message: "Conexões",
    description:
      "Datastore metrics panel: Key Value connected-clients chart section title",
  },
  "metrics.replicationLagTitle": {
    message: "Atraso de replicação",
    description:
      "Datastore metrics panel: Postgres replication-lag chart section title",
  },
  "metrics.replicationLagPendingHA": {
    message:
      "N/D — sem réplica (ative a Alta Disponibilidade para ver o atraso de replicação)",
    description:
      "Shown instead of a chart for replication lag before Postgres HA is enabled (w1/m22)",
  },
  "metrics.datastoreMetricsTitle": {
    message: "Métricas",
    description: "Database/Key Value detail page metrics panel card title",
  },
  "metrics.datastoreMetricsDescription": {
    message: "Uso de recursos em tempo real desta instância",
    description:
      "Database/Key Value detail page metrics panel card description",
  },
  "metrics.statusCode": {
    message: "Código de status",
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
    message: "Caminho",
    description: "Network-card filter label for the request Path",
  },
  "metrics.pathPlaceholder": {
    message: "Caminho…",
    description: "Placeholder for the free-text Path filter input",
  },
  "metrics.pathClear": {
    message: "Limpar filtro de caminho",
    description: "Accessible label for the button that clears the Path filter",
  },
  "metrics.hostPathStoreUnavailable": {
    message:
      "Os filtros de Host e Caminho e o detalhamento por Host exigem o armazenamento de logs",
    description:
      "Per-section state when a host/path-filtered or host-grouped request metric hits a deployment with no durable log store (503) — served from the request-log store (Loki), unavailable here",
  },
  "metrics.groupBy": {
    message: "Agrupar por",
    description: "Total Requests chart control label",
  },
  "metrics.groupByAllRequests": {
    message: "Todas as requisições",
    description: "Group-by option: one aggregate series",
  },
  "metrics.groupByStatus": {
    message: "Código de status",
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
    message: "Memória",
    description: "Application metrics stat tile label",
  },
  "metrics.cpu": {
    message: "CPU",
    description: "Application metrics stat tile label",
  },
  "metrics.totalInstances": {
    message: "Total de instâncias",
    description: "Application metrics stat tile label",
  },
  "metrics.filterPercentage": {
    message: "Porcentagem",
    description: "Metrics filter tab: show values as a percentage",
  },
  "metrics.filterTotal": {
    message: "Total",
    description: "Metrics filter tab: show values as an absolute total",
  },
  "metrics.instanceFilter": {
    message: "Instâncias",
    description: "Application metrics instance multi-select label",
  },
  "metrics.instanceShowAll": {
    message: "Mostrar todas",
    description:
      "Explicit action that clears the retained instance selection back to all instances (w5/m91)",
  },
  "metrics.instanceUnavailableSuffix": {
    message: " (indisponível)",
    description:
      "Suffix appended to a retained instance option that is no longer in the window or discovery (w5/m91)",
  },
  "metrics.instancesUnavailable_one": {
    message: "{count} instância selecionada está indisponível neste período",
    description:
      "Note when one retained instance selection has no samples in the window (singular; w5/m91)",
  },
  "metrics.instancesUnavailable_many": {
    message:
      "{count} de instâncias selecionadas estão indisponíveis neste período",
    description:
      "Note when retained instance selections have no samples in the window (plural; w5/m91)",
  },
  "metrics.instancesUnavailable_other": {
    message:
      "{count} instâncias selecionadas estão indisponíveis neste período",
    description:
      "Note when retained instance selections have no samples in the window (plural; w5/m91)",
  },
  "metrics.noDataForSelection": {
    message: "Sem dados para as instâncias selecionadas neste intervalo",
    description:
      "Empty-result copy when an explicit instance filter yields no samples — distinct from query errors (w5/m91)",
  },
  "metrics.aggregateFilter": {
    message: "Agregação",
    description: "Application metrics replica aggregation control",
  },
  "metrics.aggregateRaw": {
    message: "Bruto",
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
    message: "Média",
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
    message: "Últimos 2 dias",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast7Days": {
    message: "Últimos 7 dias",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast14Days": {
    message: "Últimos 14 dias",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast30Days": {
    message: "Últimos 30 dias",
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
      "Escolha um início e um fim absolutos. Períodos acima de 30 dias não estão disponíveis.",
    description: "Custom time-range picker dialog description",
  },
  "metrics.rangeCustomStart": {
    message: "Início",
    description: "Custom range start datetime field label",
  },
  "metrics.rangeCustomEnd": {
    message: "Fim",
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
    message: "O fim deve ser posterior ao início.",
    description: "Custom range validation: end not after start",
  },
  "metrics.rangeCustomErrorTooLong": {
    message: "Escolha um período de até 30 dias.",
    description: "Custom range validation: window exceeds the max query window",
  },
  "metrics.rangeCustomErrorTooEarly": {
    message: "O intervalo deve começar em {date} ou depois.",
    description:
      "Custom range validation: the start is before the oldest instant the range may reach (e.g. a log plan's retention)",
  },
  "metrics.retentionWindow_one": {
    message: "As métricas são mantidas por {count} dia no plano {plan}.",
    description:
      "Metrics range picker hint: how many days the workspace plan keeps metrics (w7/m427, from bex-api metricsRetention)",
  },
  "metrics.retentionWindow_many": {
    message: "As métricas são mantidas por {count} de dias no plano {plan}.",
    description:
      "Metrics range picker hint: how many days the workspace plan keeps metrics (w7/m427, from bex-api metricsRetention)",
  },
  "metrics.retentionWindow_other": {
    message: "As métricas são mantidas por {count} dias no plano {plan}.",
    description:
      "Metrics range picker hint: how many days the workspace plan keeps metrics (w7/m427, from bex-api metricsRetention)",
  },
  "metrics.retentionUpgrade_one": {
    message: "O plano {plan} as mantém por {count} dia.",
    description:
      "Metrics range picker hint: a plan that keeps metrics longer, and its days; one sentence per longer window (w7/m427)",
  },
  "metrics.retentionUpgrade_many": {
    message: "O plano {plan} as mantém por {count} de dias.",
    description:
      "Metrics range picker hint: a plan that keeps metrics longer, and its days; one sentence per longer window (w7/m427)",
  },
  "metrics.retentionUpgrade_other": {
    message: "O plano {plan} as mantém por {count} dias.",
    description:
      "Metrics range picker hint: a plan that keeps metrics longer, and its days; one sentence per longer window (w7/m427)",
  },
  "metrics.rangeLabel": {
    message: "Intervalo de tempo",
    description: "Accessible label for the time-range dropdown",
  },
  "metrics.eventsToggle": {
    message: "Mostrar linha do tempo de eventos",
    description: "Accessible label for the Metrics event-timeline toggle",
  },
  "metrics.latencyPlanRequired": {
    message:
      "As métricas de latência de resposta exigem o plano Pro ou superior. Este workspace está no plano Hobby.",
    description:
      "Response Times section on a Hobby workspace (PLAN_FEATURE_REQUIRED, w8/m138)",
  },
};

export default ptMetrics;
