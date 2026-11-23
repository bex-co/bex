import type { TranslationEntry } from "@/i18n";

const jaMetrics: Record<string, TranslationEntry> = {
  "metrics.cpuCores": {
    message: "{value} コア",
    description:
      "A CPU amount in cores (a limit or the latest usage); value is a decimal such as 0.100 (w4/566)",
  },
  "metrics.eventsTitle": {
    message: "イベントタイムライン",
    description:
      "Metrics-page timeline of service events in the selected range",
  },
  "metrics.eventsFilterLabel": {
    message: "イベントをフィルター",
    description: "Accessible label for the Metrics event category filter",
  },
  "metrics.eventsFilterAll": {
    message: "すべてのイベント",
    description: "All event categories",
  },
  "metrics.eventsFilterDeploys": {
    message: "デプロイ",
    description: "Deploy events category",
  },
  "metrics.eventsFilterLifecycle": {
    message: "ライフサイクル",
    description: "Lifecycle events category",
  },
  "metrics.eventsFilterConfig": {
    message: "構成",
    description: "Configuration events category",
  },
  "metrics.eventsLoading": {
    message: "イベントを読み込み中…",
    description: "Event timeline loading state",
  },
  "metrics.eventsError": {
    message: "イベントを読み込めませんでした。",
    description: "Event timeline error state",
  },
  "metrics.eventsEmptyTitle": {
    message: "イベントなし",
    description: "Event timeline empty-state title",
  },
  "metrics.eventsEmptyBody": {
    message: "この期間は何も起きていません。",
    description: "Event timeline empty-state body",
  },
  "metrics.eventsChanged": {
    message: "サービスが変更されました",
    description: "Fallback event timeline label",
  },
  "metrics.eventMarkersLabel": {
    message: "サービスイベント",
    description: "Accessible label for the event-marker strip above a chart",
  },
  "metrics.eventMarkerCluster_other": {
    message: "{count} 件のイベント",
    description:
      "Accessible label for a clustered chart event marker (count badge, plural)",
  },
  "metrics.lineChartLabel_other": {
    message: "{count} 個のデータポイントを持つ折れ線グラフ",
    description:
      "Accessible name of a metrics line chart; count is the number of distinct sample times (plural)",
  },
  "metrics.barChartLabel_other": {
    message: "{count} 個のデータポイントを持つ棒グラフ",
    description:
      "Accessible name of a metrics bar chart; count is the number of time buckets (plural)",
  },
  "metrics.barValueAt": {
    message: "{time} の値: {value}",
    description:
      "Accessible name of one focusable bar: its formatted value (e.g. 42 or 1.2K) and its local sample time",
  },
  "metrics.chartEmpty": {
    message: "期間内のデータなし",
    description:
      "Default empty state of a metrics chart when the settled query returned no samples in the selected time range",
  },
  "metrics.eventMarkerHint": {
    message: "クリックして詳細を表示",
    description: "Footer line of the chart event-marker hover tooltip",
  },
  "metrics.eventDeployStarted": {
    message: "デプロイ開始",
    description: "Chart event-marker label: a deploy began",
  },
  "metrics.eventDeployLiveFor": {
    message: "{commit} のデプロイが公開",
    description:
      "Chart event-marker label: a deploy went live, with its commit short id",
  },
  "metrics.eventDeployLive": {
    message: "デプロイが公開",
    description: "Chart event-marker label: a deploy went live (no commit id)",
  },
  "metrics.eventDeployFailed": {
    message: "デプロイ失敗",
    description: "Chart event-marker label: a deploy failed",
  },
  "metrics.eventDeployCanceled": {
    message: "デプロイをキャンセル",
    description: "Chart event-marker label: a deploy was canceled",
  },
  "metrics.eventDeployEnded": {
    message: "デプロイ終了",
    description:
      "Chart event-marker label: a deploy finished with an unrecognized status",
  },
  "metrics.eventBuildStarted": {
    message: "ビルド開始",
    description: "Chart event-marker label: an image build began",
  },
  "metrics.eventBuildEnded": {
    message: "ビルド終了",
    description: "Chart event-marker label: an image build finished",
  },
  "metrics.eventPreDeployStarted": {
    message: "プレデプロイ開始",
    description: "Chart event-marker label: the pre-deploy command began",
  },
  "metrics.eventPreDeployEnded": {
    message: "プレデプロイ終了",
    description: "Chart event-marker label: the pre-deploy command finished",
  },
  "metrics.eventJobRunEnded": {
    message: "ジョブ終了",
    description: "Chart event-marker label: a one-off job finished",
  },
  "metrics.eventBranchDeleted": {
    message: "ブランチを削除",
    description: "Chart event-marker label: the tracked Git branch was deleted",
  },
  "metrics.eventServerRestarted": {
    message: "サーバーを再起動",
    description: "Chart event-marker label: the service was restarted",
  },
  "metrics.eventSuspended": {
    message: "サービスを一時停止",
    description: "Chart event-marker label: the service was suspended",
  },
  "metrics.eventResumed": {
    message: "サービスを再開",
    description: "Chart event-marker label: the service was resumed",
  },
  "metrics.eventInstanceCountChanged": {
    message: "インスタンス数を変更",
    description: "Chart event-marker label: manual scaling changed the count",
  },
  "metrics.eventAutoscalingChanged": {
    message: "オートスケーリング設定を変更",
    description: "Chart event-marker label: autoscaling settings changed",
  },
  "metrics.eventPlanChanged": {
    message: "インスタンスタイプを変更",
    description: "Chart event-marker label: the service's plan changed",
  },
  "metrics.networkTitle": {
    message: "ネットワークメトリクス",
    description: "Network metrics card title",
  },
  "metrics.networkDescription": {
    message: "全インスタンスの集計",
    description: "Network metrics card description",
  },
  "metrics.totalRequests": {
    message: "リクエスト合計",
    description: "Network metrics chart section title",
  },
  "metrics.responseTimes": {
    message: "レスポンスタイム",
    description: "Network metrics chart section title",
  },
  "metrics.percentile": {
    message: "パーセンタイル",
    description: "Response Times section control label",
  },
  "metrics.percentileAll": {
    message: "すべて",
    description:
      "Percentile dropdown option that overlays p50/p90/p99 on one chart",
  },
  "metrics.requestsCount_other": {
    message: "{formatted} 件のリクエスト",
    description:
      "Aggregate request count beside the Total Requests section title (plural; {formatted} is the locale-formatted count)",
  },
  "metrics.outboundBandwidth": {
    message: "送信帯域幅",
    description: "Network metrics chart section title",
  },
  "metrics.monthToDateBandwidth": {
    message: "今月の使用量: {amount}",
    description:
      "Footer under the Outbound Bandwidth chart, showing month-to-date egress",
  },
  "metrics.monthToDateBandwidthError": {
    message: "今月の帯域幅を読み込めませんでした。更新してみてください",
    description:
      "Month-to-date bandwidth footer when the query failed with no value — distinct from a known zero and from the chart's bandwidthError (w4/090)",
  },
  "metrics.bandwidthDegraded": {
    message: "一部のデータのみ",
    description:
      "Badge on the Outbound Bandwidth chart when an egress source's health check failed inside the window (w1/m50)",
  },
  "metrics.bandwidthDegradedDetail": {
    message:
      "この期間の一部で {sources} の送信元が異常な状態でした。帯域幅が実際より少なく計測されている可能性があります。",
    description:
      "Tooltip expanding the Partial data badge; {sources} is the comma-joined raw source tokens (http/websocket/direct)",
  },
  "metrics.bandwidthError": {
    message: "帯域幅を読み込めませんでした。更新してみてください",
    description:
      "Bandwidth chart body when the query itself failed (network/server error, not an empty window)",
  },
  "metrics.sourceNotConfigured": {
    message: "メトリクスのソースが設定されていません",
    description: "Shown when bex-api reports no metrics backend is wired up",
  },
  "metrics.chartError": {
    message: "このメトリクスを読み込めませんでした",
    description:
      "Shown when a metrics query fails with a real error (not a 503/no-data) — distinct from the empty 'No data in range' state (w9/m86)",
  },
  "metrics.chartThrottled": {
    message: "追いついています。トラフィックが一時的に制限されました",
    description:
      "Cold-load state when a metrics read was shed (RATE_LIMITED / AUTH_OVERLOADED) with no cached series — transient, not 'metrics unavailable' (w4/m100)",
  },
  "metrics.chartStaleThrottled": {
    message: "キャッシュされたデータを表示中です。一時的な制限後に更新します",
    description:
      "Warm-cache banner when a metrics refetch was throttled; cached series remain visible but must not read as current (w4/m100)",
  },
  "metrics.applicationTitle": {
    message: "アプリケーションメトリクス",
    description: "Application metrics card title",
  },
  "metrics.limitLink": {
    message: "上限",
    description:
      "Link to the Compute page on the Memory/CPU chart headers, followed by the limit value when configured",
  },
  "metrics.manageScaling": {
    message: "スケーリングを管理",
    description:
      "Link to the Compute page's Manual Scaling section on the Total Instances chart header",
  },
  "metrics.noLimitConfigured": {
    message: "上限が設定されていないため、割合を算出できません",
    description:
      "Shown instead of a percentage chart when the App's pods declare no resource limit",
  },
  "metrics.limitsVary": {
    message: "上限が異なります",
    description:
      "Memory/CPU chart header annotation when the selected replicas declare different limits — each replica is percentized against its own limit, so no single limit value applies",
  },
  "metrics.percentageUnavailable": {
    message: "割合を表示できません。この期間には信頼できる上限がありません",
    description:
      "Shown instead of a percentage chart when usage samples exist but every denominator was missing, zero, or otherwise untrustworthy (deleted pods, predated limit history, a mid-window rollout gap) — distinct from no usage samples or a source failure",
  },
  "metrics.targetLabel": {
    message: "目標 {value}",
    description:
      "Autoscale-target annotation on the Memory/CPU chart headers (percentage mode only), e.g. 'Target 70%'",
  },
  "metrics.diskTitle": {
    message: "ディスク",
    description: "Datastore metrics panel: disk usage chart section title",
  },
  "metrics.diskCapacityLabel": {
    message: "容量 {value}",
    description:
      "Disk usage chart header annotation showing the datastore's billed/provisioned disk size",
  },
  "metrics.connectionsTitle": {
    message: "アクティブな接続",
    description:
      "Datastore metrics panel: active-connections chart section title (Postgres client connections; Key Value connected clients), as Render titles both",
  },
  "metrics.memoryTitle": {
    message: "メモリ",
    description:
      "Datastore metrics panel: memory chart section title (Postgres container memory; Key Value used memory)",
  },
  "metrics.datastoreCpuTitle": {
    message: "CPU",
    description: "Datastore metrics panel: CPU usage chart section title",
  },
  "metrics.datastoreLimitLabel": {
    message: "上限 {value}",
    description:
      "Datastore CPU/memory/connections chart header annotation showing the instance's configured limit (a Key Value's connection limit is its plan's)",
  },
  "metrics.replicationLagTitle": {
    message: "レプリケーション遅延",
    description:
      "Datastore metrics panel: Postgres replication-lag chart section title",
  },
  "metrics.diskActivityTitle": {
    message: "ディスクアクティビティ",
    description:
      "Datastore metrics panel: Render's Disk Activity chart title — bytes read from and written to the data volume per bucket (Postgres, Key Value, a service's disk; w7/m430)",
  },
  "metrics.diskOperationsTitle": {
    message: "ディスク操作",
    description:
      "Datastore metrics panel: Render's Disk Operations chart title — read and write operations on the data volume per bucket (Postgres, a service's disk; w7/m430)",
  },
  "metrics.networkActivityTitle": {
    message: "ネットワークアクティビティ",
    description:
      "Datastore metrics panel: Render's Network Activity chart title — bytes a Postgres read from and wrote to the network per bucket (w7/m430)",
  },
  "metrics.transactionVolumeTitle": {
    message: "トランザクション量",
    description:
      "Datastore metrics panel: Render's Transaction Volume chart title — transactions a Postgres executed per bucket (w7/m430)",
  },
  "metrics.lockDelayedQueriesTitle": {
    message: "ロック待ちで遅れたクエリ",
    description:
      "Datastore metrics panel: Render's Lock-Delayed Queries chart title — completed queries that waited a second or more on a lock (w7/m430)",
  },
  "metrics.directionRead": {
    message: "読み取り",
    description:
      "Legend entry of the read half of a two-series chart (data read from disk or the network)",
  },
  "metrics.directionWrite": {
    message: "書き込み",
    description:
      "Legend entry of the write half of a two-series chart (data written to disk or the network)",
  },
  "metrics.seriesInstanceDirection": {
    message: "{instance} · {direction}",
    description:
      "Legend entry of a two-series chart when the datastore has several instances: the instance id, then Read or Write",
  },
  "metrics.lockDelayedStoreUnavailable": {
    message: "ロック待ちで遅れたクエリにはログストアが必要です",
    description:
      "Shown instead of the Lock-Delayed Queries chart on a deployment with no durable log store (503): the count comes from Postgres log lines",
  },
  "metrics.replicationLagPendingHA": {
    message:
      "該当なし: レプリカがありません（レプリケーション遅延を表示するには高可用性を有効にするかリードレプリカを追加してください）",
    description:
      "Shown instead of a chart for replication lag when Postgres has neither HA nor read replicas (w1/m22, w34/011)",
  },
  "metrics.datastoreMetricsTitle": {
    message: "メトリクス",
    description: "Database/Key Value detail page metrics panel card title",
  },
  "metrics.datastoreMetricsDescription": {
    message: "このインスタンスのリソース使用状況（リアルタイム）",
    description:
      "Database/Key Value detail page metrics panel card description",
  },
  "metrics.statusCode": {
    message: "ステータスコード",
    description: "Toolbar filter label for HTTP status code",
  },
  "metrics.statusCodeAll": {
    message: "すべて",
    description: "Status-code filter option meaning no filtering",
  },
  "metrics.host": {
    message: "ホスト",
    description: "Network-card filter label for the request Host",
  },
  "metrics.hostAll": {
    message: "すべて",
    description: "Host filter option meaning no filtering",
  },
  "metrics.path": {
    message: "パス",
    description: "Network-card filter label for the request Path",
  },
  "metrics.pathPlaceholder": {
    message: "パス…",
    description: "Placeholder for the free-text Path filter input",
  },
  "metrics.pathClear": {
    message: "パスのフィルターをクリア",
    description: "Accessible label for the button that clears the Path filter",
  },
  "metrics.hostPathStoreUnavailable": {
    message:
      "ホストとパスのフィルター、およびホスト別の内訳にはログストアが必要です",
    description:
      "Per-section state when a host/path-filtered or host-grouped request metric hits a deployment with no durable log store (503) — served from the request-log store (Loki), unavailable here",
  },
  "metrics.groupBy": {
    message: "グループ化",
    description: "Total Requests chart control label",
  },
  "metrics.groupByAllRequests": {
    message: "すべてのリクエスト",
    description: "Group-by option: one aggregate series",
  },
  "metrics.groupByStatus": {
    message: "ステータスコード",
    description: "Group-by option: one series per HTTP status code",
  },
  "metrics.groupByMethod": {
    message: "メソッド",
    description: "Group-by option: one series per HTTP method",
  },
  "metrics.groupByHost": {
    message: "ホスト",
    description: "Group-by option: one series per request host",
  },
  "metrics.memory": {
    message: "メモリ",
    description: "Application metrics stat tile label",
  },
  "metrics.cpu": {
    message: "CPU",
    description: "Application metrics stat tile label",
  },
  "metrics.totalInstances": {
    message: "インスタンス合計",
    description: "Application metrics stat tile label",
  },
  "metrics.filterPercentage": {
    message: "割合",
    description: "Metrics filter tab: show values as a percentage",
  },
  "metrics.filterTotal": {
    message: "合計",
    description: "Metrics filter tab: show values as an absolute total",
  },
  "metrics.instanceFilter": {
    message: "インスタンス",
    description: "Application metrics instance multi-select label",
  },
  "metrics.instanceFilterAll": {
    message: "すべて",
    description:
      "Instances filter trigger's value when no instance is picked (every instance is shown)",
  },
  "metrics.instanceShowAll": {
    message: "すべて表示",
    description:
      "Explicit action that clears the retained instance selection back to all instances (w5/m91)",
  },
  "metrics.instanceUnavailableSuffix": {
    message: " （利用不可）",
    description:
      "Suffix appended to a retained instance option that is no longer in the window or discovery (w5/m91)",
  },
  "metrics.instancesUnavailable_other": {
    message: "選択した {count} 個のインスタンスはこの期間では利用できません",
    description:
      "Note when retained instance selections have no samples in the window (plural; w5/m91)",
  },
  "metrics.noDataForSelection": {
    message: "この期間には、選択したインスタンスのデータがありません",
    description:
      "Empty-result copy when an explicit instance filter yields no samples — distinct from query errors (w5/m91)",
  },
  "metrics.aggregateFilter": {
    message: "集計",
    description: "Application metrics replica aggregation control",
  },
  "metrics.aggregateRaw": {
    message: "生データ",
    description: "Show one series per selected instance",
  },
  "metrics.aggregateMin": {
    message: "最小",
    description: "Minimum across selected replicas at each timestamp",
  },
  "metrics.aggregateMax": {
    message: "最大",
    description: "Maximum across selected replicas at each timestamp",
  },
  "metrics.aggregateAvg": {
    message: "平均",
    description: "Average across selected replicas at each timestamp",
  },
  "metrics.rangeLast30Minutes": {
    message: "過去 30 分",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLastHour": {
    message: "過去 1 時間",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast4Hours": {
    message: "過去 4 時間",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast12Hours": {
    message: "過去 12 時間",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast24Hours": {
    message: "過去 24 時間",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast2Days": {
    message: "過去 2 日間",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast7Days": {
    message: "過去 7 日間",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast14Days": {
    message: "過去 14 日間",
    description: "Metrics time-range filter option",
  },
  "metrics.rangeLast30Days": {
    message: "過去 30 日間",
    description:
      "Metrics time-range filter option (w5/m56; Render plan-gates it)",
  },
  "metrics.rangeCustom": {
    message: "カスタム…",
    description: "Time-range dropdown option that opens the start/end picker",
  },
  "metrics.rangeCustomEdit": {
    message: "カスタム期間を編集",
    description:
      "Button beside the time-range dropdown that reopens the picker for the active custom range",
  },
  "metrics.rangeCustomTitle": {
    message: "カスタム期間",
    description: "Custom time-range picker dialog title",
  },
  "metrics.rangeCustomDescription": {
    message:
      "開始日時と終了日時を指定してください。30 日を超える期間は指定できません。",
    description: "Custom time-range picker dialog description",
  },
  "metrics.rangeCustomStart": {
    message: "開始",
    description: "Custom range start datetime field label",
  },
  "metrics.rangeCustomEnd": {
    message: "終了",
    description: "Custom range end datetime field label",
  },
  "metrics.rangeCustomApply": {
    message: "適用",
    description: "Custom range apply button",
  },
  "metrics.rangeCustomCancel": {
    message: "キャンセル",
    description: "Custom range cancel button",
  },
  "metrics.rangeCustomErrorOrder": {
    message: "終了は開始より後にしてください。",
    description: "Custom range validation: end not after start",
  },
  "metrics.rangeCustomErrorTooLong": {
    message: "30 日以内の期間を指定してください。",
    description: "Custom range validation: window exceeds the max query window",
  },
  "metrics.rangeCustomErrorTooEarly": {
    message: "範囲の開始は {date} 以降にしてください。",
    description:
      "Custom range validation: the start is before the oldest instant the range may reach (e.g. a log plan's retention)",
  },
  "metrics.retentionWindow_other": {
    message: "{plan} プランではメトリクスを {count} 日間保持します。",
    description:
      "Metrics range picker hint: how many days the workspace plan keeps metrics (w7/m427, from bex-api metricsRetention)",
  },
  "metrics.retentionUpgrade_other": {
    message: "{plan} プランは {count} 日間保持します。",
    description:
      "Metrics range picker hint: a plan that keeps metrics longer, and its days; one sentence per longer window (w7/m427)",
  },
  "metrics.rangeLabel": {
    message: "期間",
    description: "Accessible label for the time-range dropdown",
  },
  "metrics.eventsToggle": {
    message: "イベントタイムラインを表示",
    description: "Accessible label for the Metrics event-timeline toggle",
  },
  "metrics.latencyPlanRequired": {
    message:
      "レスポンスレイテンシのメトリクスには Pro プラン以上が必要です。このワークスペースは Hobby プランです。",
    description:
      "Response Times section on a Hobby workspace (PLAN_FEATURE_REQUIRED, w8/m138)",
  },
  "metrics.serviceMetricsTitle": {
    message: "サービスメトリクス",
    description:
      "A project's Service Metrics view type: New View's menu, a new view's name and its page title (w3/m508)",
  },
  "metrics.projectApplicationDescription": {
    message: "リソースごとに 1 本の線、インスタンスは合算",
    description:
      "Subtitle of a project Service Metrics view's Application Metrics card (w3/m508)",
  },
  "metrics.projectNetworkDescription": {
    message: "各リソースのリクエスト、応答時間、送信トラフィック",
    description:
      "Subtitle of a project Service Metrics view's Network Metrics card (w3/m508)",
  },
  "metrics.groupByResource": {
    message: "リソース",
    description:
      "Total Requests Group by option on a project Service Metrics view: one line per resource (Render's RESOURCE, w3/m508)",
  },
  "metrics.projectLegendLabel": {
    message: "このビューのリソース",
    description:
      "Accessible name of the legend whose buttons show or hide each resource on every chart (w3/m508)",
  },
  "metrics.projectChartFailed": {
    message: "{names} を読み込めませんでした。このグラフには含まれません。",
    description:
      "Under a project chart: resources whose read failed and which the chart leaves out; names is a comma list (w3/m508)",
  },
  "metrics.projectChartNotCharted": {
    message:
      "{names} は表示されません: このグラフにはその種類のデータがありません。",
    description:
      "Under a project chart: resources of a type the chart has no data for (a static site's CPU, a worker's requests); names is a comma list (w3/m508)",
  },
  "metrics.projectChartNoData": {
    message: "{names} はこの期間にデータがありません。",
    description:
      "Under a project chart: resources read with nothing in the range; names is a comma list (w3/m508)",
  },
  "metrics.projectViewTooLargeTitle": {
    message: "このビューはグラフにするにはリソースが多すぎます",
    description:
      "Project Service Metrics view whose filters keep more resources than one view charts (PROJECT_VIEW_TOO_LARGE, w3/m508)",
  },
  "metrics.projectViewTooLargeBody": {
    message:
      "このビューには {resources} 個のリソースがあり、1 つのビューでグラフにできるのは最大 {limit} 個です。ビューを編集し、環境、リソース、リソースタイプのフィルターで絞り込んでください。",
    description:
      "Body of the too-many-resources state; resources is how many the view keeps, limit how many it may chart (w3/m508)",
  },
  "metrics.projectScopeEmptyTitle": {
    message: "このビューにはまだグラフにするものがありません",
    description:
      "Project Service Metrics view with no resource in it (w3/m508)",
  },
  "metrics.projectScopeEmptyBody": {
    message:
      "このビューはプロジェクトのサービス、cron ジョブ、Postgres データベース、Key Value インスタンスをグラフにしますが、まだ何もありません。",
    description: "Body of the empty project Service Metrics view (w3/m508)",
  },
  "metrics.viewActionsFor": {
    message: "{instance} の操作を表示",
    description:
      "Tooltip hint at the bottom of a Memory or CPU chart when hovering an instance's series (w4/717; Render's 'View actions for <slug>')",
  },
  "metrics.instanceActionsMenu": {
    message: "インスタンス {instance} の操作",
    description:
      "Accessible name of the instance action card and legend menu trigger on Memory and CPU charts (w4/717)",
  },
  "metrics.filterToInstance": {
    message: "このインスタンスで絞り込む",
    description:
      "Chart instance menu item: filters the Application Metrics card to this instance (w4/717; Render's 'Filter to instance')",
  },
  "metrics.resetInstanceFilter": {
    message: "インスタンスフィルターをリセット",
    description:
      "Chart instance menu item: removes this instance from the card's Instances filter (w4/717; Render's 'Reset instance filter')",
  },
  "metrics.viewInstanceLogs": {
    message: "インスタンスのログを表示",
    description:
      "Chart instance menu item: opens the service's Logs tab filtered to this instance in a new tab (w4/717; Render's 'View instance logs')",
  },
  "metrics.copyInstanceId": {
    message: "インスタンス ID をコピー",
    description:
      "Chart instance menu item: copies the instance's full ID to the clipboard (w4/717; Render's 'Copy instance ID')",
  },
  "metrics.instanceIdCopied": {
    message: "インスタンス ID をコピーしました",
    description:
      "Toast after Copy instance ID wrote the instance ID to the clipboard (w4/717)",
  },
  "metrics.instanceIdCopyFailed": {
    message: "インスタンス ID をコピーできませんでした",
    description:
      "Toast when the clipboard refused Copy instance ID (w4/717)",
  },
};

export default jaMetrics;
