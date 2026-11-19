import type { TranslationEntry } from "@/i18n";

const jaWebhooks: Record<string, TranslationEntry> = {
  "webhooks.title": {
    message: "Webhook",
    description: "Settings Integrations Webhooks section card title",
  },
  "webhooks.description": {
    message:
      "署名付きのイベント通知（デプロイ、一時停止、再起動、スケーリング）を、ポーリングなしで独自のエンドポイントにプッシュします。各 Webhook の署名シークレットは、その設定にあります。",
    description: "Settings Webhooks section card description",
  },
  "webhooks.colEndpoint": {
    message: "エンドポイント",
    description: "Webhooks table column header — name + destination URL",
  },
  "webhooks.colEvents": {
    message: "イベント",
    description: "Webhooks table column header — subscribed event types",
  },
  "webhooks.colEnabled": {
    message: "有効",
    description: "Webhooks table column header — the enable/disable switch",
  },
  "webhooks.colLatest": {
    message: "最新の配信",
    description: "Webhooks table column header — newest immutable attempt",
  },
  "webhooks.searchEndpoints": {
    message: "Webhook を検索",
    description: "Endpoint list search label and placeholder",
  },
  "webhooks.searchEndpointsHelp": {
    message: "名前、宛先、イベントで検索",
    description: "Endpoint list search helper text",
  },
  "webhooks.searchEndpointsEmpty": {
    message: "検索に一致する Webhook がありません。",
    description: "Endpoint list empty state after local search",
  },
  "webhooks.emptyTitle": {
    message: "Webhook がありません",
    description: "Webhooks empty-state title",
  },
  "webhooks.emptyBody": {
    message:
      "エンドポイントを追加すると、サービスのデプロイ、再起動、スケーリング時に通知を受け取れます。",
    description: "Webhooks empty-state body",
  },
  "webhooks.forbiddenTitle": {
    message: "権限がありません",
    description: "Webhooks state when the caller lacks permission (403)",
  },
  "webhooks.forbiddenBody": {
    message: "このワークスペースの Webhook を表示する権限がありません。",
    description: "Webhooks forbidden-state body",
  },
  "webhooks.errorTitle": {
    message: "Webhook を読み込めませんでした",
    description: "Webhooks generic error title",
  },
  "webhooks.errorBody": {
    message: "問題が発生しました。もう一度お試しください。",
    description: "Webhooks generic error body",
  },
  "webhooks.proOnly": {
    message:
      "Webhook には Pro プラン以上が必要です。既存のエンドポイントへの配信は引き続き行われます。",
    description: "Webhooks card description on a Hobby workspace (w8/m138)",
  },
  "webhooks.create": {
    message: "Webhook を追加",
    description: "Button that opens the create dialog",
  },
  "webhooks.createTitle": {
    message: "Webhook を追加",
    description: "Create dialog title",
  },
  "webhooks.createDescription": {
    message:
      "購読中のイベントが発生するたびに、bex がこの URL へ署名付きの JSON ペイロードを POST します。",
    description: "Create dialog description",
  },
  "webhooks.createEnabledHelp": {
    message: "ただちに配信を開始します。後から設定で有効にすることもできます。",
    description: "Create page initial enabled-state helper copy",
  },
  "webhooks.fieldName": {
    message: "名前",
    description: "Create dialog name field label",
  },
  "webhooks.fieldNamePlaceholder": {
    message: "例: deploy-alerts-slack-bot",
    description: "Create dialog name field placeholder",
  },
  "webhooks.fieldUrl": {
    message: "宛先 URL",
    description: "Create dialog URL field label",
  },
  "webhooks.fieldEvents": {
    message: "送信するイベント",
    description: "Create dialog event-type checklist label",
  },
  "webhooks.eventsLoading": {
    message: "イベントタイプを読み込み中…",
    description: "Create dialog while the event-type vocabulary loads",
  },
  "webhooks.eventsLoadError": {
    message:
      "イベントタイプを読み込めませんでした。保存する前に再試行してください。",
    description: "Actionable event-catalog query failure",
  },
  "webhooks.eventsRetry": {
    message: "イベントタイプを再試行",
    description: "Retry button for the event-catalog query",
  },
  "webhooks.eventsEmpty": {
    message: "現在、購読できるイベントタイプはありません。",
    description: "Event picker state when the served catalog is empty",
  },
  "webhooks.formUnavailableEvents": {
    message: "イベントタイプが読み込まれるまで、作成できません。",
    description:
      "Create submit disabled reason while the catalog is unavailable",
  },
  "webhooks.formErrorsSummary": {
    message: "強調表示されたフィールドを修正して、もう一度お試しください。",
    description: "Accessible webhook form validation summary",
  },
  "webhooks.validation.nameRequired": {
    message: "Webhook の名前を入力してください。",
    description: "Field-local required-name validation",
  },
  "webhooks.validation.nameDuplicate": {
    message: "この名前の Webhook はすでに存在します。",
    description: "Field-local duplicate-name validation",
  },
  "webhooks.validation.urlRequired": {
    message: "宛先 URL を入力してください。",
    description: "Field-local required URL validation",
  },
  "webhooks.validation.urlHTTPS": {
    message: "認証情報を含まない、絶対 HTTPS URL を入力してください。",
    description: "Field-local webhook URL validation",
  },
  "webhooks.validation.eventsRequired": {
    message: "イベントを 1 つ以上選択してください。",
    description: "Field-local event selection validation",
  },
  "webhooks.validation.eventsInvalid": {
    message:
      "イベントの選択が変更されました。カタログを再読み込みして、もう一度選択してください。",
    description: "Server-coded stale/invalid event filter",
  },
  "webhooks.errors.planLimit": {
    message:
      "Webhook は Pro プラン以上で利用できます。このワークスペースは Hobby プランのため、エンドポイントを追加できません。",
    description:
      "Server-coded plan refusal of a webhook create on a Hobby workspace (WEBHOOK_PLAN_LIMIT, w4/425)",
  },
  "webhooks.planGate": {
    message:
      "Webhook は Pro プラン以上で利用できます。エンドポイントを追加するには、このワークスペースをアップグレードしてください。",
    description:
      "Notice in place of the create form and the empty list on a Hobby workspace (w4/425)",
  },
  "webhooks.errors.endpointLimit": {
    message: "このワークスペースは、Webhook エンドポイントの上限に達しました。",
    description: "Server-coded endpoint quota refusal",
  },
  "webhooks.errors.endpointDisabled": {
    message: "配信を再送するには、先にこのエンドポイントを有効にしてください。",
    description: "Server-coded resend refusal for a disabled endpoint",
  },
  "webhooks.errors.deliveryPending": {
    message:
      "配信の試行がすでにキューに入っています。完了するまでお待ちください。",
    description: "Server-coded duplicate pending resend refusal",
  },
  "webhooks.errors.staleState": {
    message:
      "この Webhook は変更または削除されました。更新してもう一度お試しください。",
    description: "Safe stale endpoint/attempt refusal",
  },
  "webhooks.manageRequired": {
    message:
      "この Webhook は確認できますが、変更できるのはワークスペースの管理者のみです。",
    description: "Read-only member notice on webhook management surfaces",
  },
  "webhooks.latestNever": {
    message: "未送信",
    description: "Endpoint with no completed delivery attempt",
  },
  "webhooks.latestDelivered": {
    message: "成功",
    description: "Latest endpoint attempt succeeded",
  },
  "webhooks.latestFailed": {
    message: "失敗",
    description: "Latest endpoint attempt failed terminally",
  },
  "webhooks.latestRetrying": {
    message: "再試行中",
    description: "Latest endpoint attempt failed with a retry pending",
  },
  "webhooks.createCancel": {
    message: "キャンセル",
    description: "Create dialog cancel button",
  },
  "webhooks.createSubmit": {
    message: "Webhook を作成",
    description: "Create page submit button (Render: 'Create Webhook')",
  },
  "webhooks.createSuccess": {
    message: "Webhook を作成しました",
    description: "Toast after a successful create",
  },
  "webhooks.createError": {
    message: "Webhook を作成できませんでした",
    description: "Toast after a failed create",
  },
  "webhooks.copy": {
    message: "署名シークレットをコピー",
    description: "Copy-secret button label",
  },
  "webhooks.copied": {
    message: "シークレットをクリップボードにコピーしました",
    description: "Toast after copying the secret",
  },
  "webhooks.copyError": {
    message:
      "コピーできませんでした。テキストを選択して手動でコピーしてください",
    description: "Toast when clipboard write fails",
  },
  "webhooks.toggle": {
    message: "有効",
    description: "Accessible label of the per-endpoint enable/disable switch",
  },
  "webhooks.enableSuccess": {
    message: "{name} を有効にしました",
    description: "Toast after enabling an endpoint",
  },
  "webhooks.disableSuccess": {
    message: "{name} を無効にしました",
    description: "Toast after disabling an endpoint",
  },
  "webhooks.toggleError": {
    message: "{name} を更新できませんでした",
    description: "Toast after a failed enable/disable",
  },
  "webhooks.delete": {
    message: "削除",
    description: "Delete action label (icon button + confirm button)",
  },
  "webhooks.deleteConfirmTitle": {
    message: "{name} を削除しますか？",
    description: "Delete confirmation dialog title",
  },
  "webhooks.deleteConfirmBody": {
    message:
      "このエンドポイントには今後イベントが送信されず、配信履歴も削除されます。",
    description: "Delete confirmation dialog body",
  },
  "webhooks.deleteCancel": {
    message: "キャンセル",
    description: "Delete confirmation cancel button",
  },
  "webhooks.deleteSuccess": {
    message: "{name} を削除しました",
    description: "Toast after a successful delete",
  },
  "webhooks.deleteError": {
    message: "{name} を削除できませんでした",
    description: "Toast after a failed delete",
  },
  "webhooks.history": {
    message: "配信履歴",
    description: "Accessible label of the per-endpoint history button",
  },
  "webhooks.historyTitle": {
    message: "配信 — {name}",
    description: "Delivery-history dialog title",
  },
  "webhooks.historyBody": {
    message:
      "このエンドポイントに送信された各イベントを、新しい順に表示します。失敗した配信は指数バックオフで再試行され、失敗が続くとエンドポイントは無効になります。",
    description: "Delivery-history dialog description",
  },
  "webhooks.historyEmptyTitle": {
    message: "配信はまだありません",
    description: "Delivery-history empty-state title",
  },
  "webhooks.historyEmptyBody": {
    message:
      "購読中のイベント（たとえばデプロイ）を発生させると、ここに表示されます。",
    description: "Delivery-history empty-state body",
  },
  "webhooks.historyFilteredEmptySuccessfulTitle": {
    message: "成功した配信はありません",
    description:
      "Delivery-history state when the Successful tab (and any date range) matches nothing (w4/146)",
  },
  "webhooks.historyFilteredEmptyFailedTitle": {
    message: "失敗した配信はありません",
    description:
      "Delivery-history state when the Failed tab (and any date range) matches nothing (w4/146)",
  },
  "webhooks.historyFilteredEmptyRangeTitle": {
    message: "この期間の配信はありません",
    description:
      "Delivery-history state when the Sent after/before range matches nothing (w4/146)",
  },
  "webhooks.historyFilteredEmptyBody": {
    message: "別のステータスや期間では、一致する配信がある可能性があります。",
    description: "Body under a filtered-empty delivery history (w4/146)",
  },
  "webhooks.historyClearFilters": {
    message: "フィルターをクリア",
    description:
      "Resets the delivery-history status tab and date range (w4/146)",
  },
  "webhooks.historyErrorTitle": {
    message: "配信を読み込めませんでした",
    description: "Delivery-history error-state title",
  },
  "webhooks.historyErrorBody": {
    message: "問題が発生しました。もう一度お試しください。",
    description: "Delivery-history error-state body",
  },
  "webhooks.colEvent": {
    message: "イベント",
    description: "Delivery-history table column header",
  },
  "webhooks.colService": {
    message: "サービス",
    description: "Delivery-history table column header",
  },
  "webhooks.serviceDeleted": {
    message: "（削除済み）",
    description:
      "Marker beside a delivery's recorded service name when that service no longer exists, so the name cannot be linked",
  },
  "webhooks.colStatus": {
    message: "ステータス",
    description: "Delivery-history table column header",
  },
  "webhooks.colAttempts": {
    message: "試行",
    description:
      "Delivery-history table column header — one-based attempt number",
  },
  "webhooks.colResponse": {
    message: "レスポンス",
    description: "Delivery-history table column header — last HTTP status",
  },
  "webhooks.colWhen": {
    message: "日時",
    description:
      "Delivery-history table column header — relative and exact send time",
  },
  "webhooks.colActions": {
    message: "操作",
    description: "Accessible delivery-history table actions column header",
  },
  "webhooks.status.pending": {
    message: "保留中",
    description: "Delivery status badge — queued or between retries",
  },
  "webhooks.status.delivered": {
    message: "配信済み",
    description: "Delivery status badge — endpoint answered 2xx",
  },
  "webhooks.status.failed": {
    message: "失敗",
    description: "Delivery status badge — this individual attempt failed",
  },
  "webhooks.httpStatus": {
    message: "HTTP {status}",
    description: "Delivery attempt HTTP status summary",
  },
  "webhooks.openSourceEvent": {
    message: "{label} — ソースイベント {id} を開く",
    description:
      "Accessible label for the event hydration link; it starts with the link's visible text, the event type (WCAG 2.5.3)",
  },
  "webhooks.requestPayload": {
    message: "リクエストペイロード",
    description: "Expanded delivery-attempt request evidence heading",
  },
  "webhooks.endpointResponse": {
    message: "エンドポイントのレスポンス",
    description: "Expanded delivery-attempt response evidence heading",
  },
  "webhooks.noRequestEvidence": {
    message: "リクエストの記録はありません。",
    description: "Expanded attempt with no retained request body",
  },
  "webhooks.noResponseEvidence": {
    message: "レスポンスは記録されていません。",
    description: "Expanded pending/transport attempt with no endpoint body",
  },
  "webhooks.attemptIdentity": {
    message: "試行 {attemptId} · ソースイベント {eventId}",
    description: "Immutable attempt and stable source-event identity",
  },
  "webhooks.parentDeliveryStatus": {
    message: "通知の状態: {status}",
    description:
      "Logical notification state shown separately from attempt outcome",
  },
  "webhooks.retryScheduled": {
    message: "次の自動試行: {date}",
    description: "Parent notification retry schedule",
  },
  "webhooks.resend": {
    message: "再送信",
    description: "Manual resend action for a failed attempt",
  },
  "webhooks.resending": {
    message: "再送信中…",
    description: "Manual resend action in progress",
  },
  "webhooks.resendConfirmTitle": {
    message: "この Webhook を再送信しますか？",
    description: "Manual resend confirmation title",
  },
  "webhooks.resendConfirmBody": {
    message:
      "同じソースイベントとリクエストペイロードで、署名付きの新しい試行をキューに追加します。",
    description: "Manual resend confirmation consequence",
  },
  "webhooks.resendCancel": {
    message: "キャンセル",
    description: "Manual resend confirmation cancel action",
  },
  "webhooks.resendSuccess": {
    message: "Webhook の再送信をキューに追加しました",
    description: "Toast after a manual resend reservation succeeds",
  },
  "webhooks.resendError": {
    message: "この Webhook を再送信できませんでした",
    description: "Toast after a manual resend is refused or fails",
  },
  "webhooks.loadMore": {
    message: "さらに読み込む",
    description: "Delivery-history pagination button",
  },
  "webhooks.selectedCount_other": {
    message: "{count} 件のイベントを選択中",
    description: "Event-picker live selection counter (plural)",
  },
  "webhooks.searchEvents": {
    message: "イベントを検索",
    description: "Event-picker search box placeholder + accessible label",
  },
  "webhooks.searchNoMatches": {
    message: "検索に一致するイベントがありません",
    description:
      "Event-picker empty state while a search filters everything out",
  },
  "webhooks.allEvents": {
    message: "すべてのイベント",
    description: "Event-picker tri-state master checkbox label",
  },
  "webhooks.groupToggle": {
    message: "{group} のイベントを切り替え",
    description: "Accessible label of a group's expand/collapse chevron",
  },
  "webhooks.group.deploy": {
    message: "デプロイ",
    description: "Event-picker group — deploy lifecycle events",
  },
  "webhooks.group.autoDeploy": {
    message: "自動デプロイ",
    description: "Event-picker group — automatic deploy setting events",
  },
  "webhooks.group.serviceAvailability": {
    message: "サービスの可用性",
    description: "Event-picker group — observed service availability events",
  },
  "webhooks.group.scaling": {
    message: "スケーリング",
    description: "Event-picker group — service scaling events",
  },
  "webhooks.group.cronJobRun": {
    message: "Cron Job の実行",
    description: "Event-picker group — cron job run events",
  },
  "webhooks.group.maintenanceMode": {
    message: "メンテナンスモード",
    description: "Event-picker group — maintenance mode events",
  },
  "webhooks.group.disk": {
    message: "ディスク",
    description: "Event-picker group — persistent disk events",
  },
  "webhooks.group.postgres": {
    message: "Postgres",
    description: "Event-picker group — managed Postgres events",
  },
  "webhooks.group.keyValue": {
    message: "Key Value",
    description: "Event-picker group — managed Key Value events",
  },
  "webhooks.group.suspension": {
    message: "一時停止",
    description: "Event-picker group — suspend/resume events",
  },
  "webhooks.group.agentSession": {
    message: "クラウドエージェント",
    description: "Event-picker group — cloud coding agent session events",
  },
  "webhooks.group.other": {
    message: "その他",
    description:
      "Event-picker fallback group for served keys the catalog doesn't know yet",
  },
  "webhooks.event.deploy_started": {
    message: "デプロイ開始",
    description: "Event label — deploy_started",
  },
  "webhooks.event.branch_deleted": {
    message: "ブランチ削除",
    description: "Webhook event label",
  },
  "webhooks.event.build_started": {
    message: "ビルド開始",
    description: "Webhook event label",
  },
  "webhooks.event.build_ended": {
    message: "ビルド終了",
    description: "Webhook event label",
  },
  "webhooks.event.pre_deploy_started": {
    message: "プレデプロイ開始",
    description: "Webhook event label",
  },
  "webhooks.event.pre_deploy_ended": {
    message: "プレデプロイ終了",
    description: "Webhook event label",
  },
  "webhooks.event.job_run_ended": {
    message: "ジョブ実行終了",
    description: "Webhook event label",
  },
  "webhooks.event.auto_deploy_enabled": {
    message: "自動デプロイ有効化",
    description: "Webhook event label",
  },
  "webhooks.event.auto_deploy_disabled": {
    message: "自動デプロイ無効化",
    description: "Webhook event label",
  },
  "webhooks.event.deploy_ended": {
    message: "デプロイ終了",
    description: "Event label — deploy_ended",
  },
  "webhooks.event.zero_downtime_redeploy_started": {
    message: "ゼロダウンタイム再デプロイ開始",
    description:
      "Event label — zero_downtime_redeploy_started: a Bex-initiated zero-downtime deploy started for a service",
  },
  "webhooks.event.zero_downtime_redeploy_ended": {
    message: "ゼロダウンタイム再デプロイ終了",
    description:
      "Event label — zero_downtime_redeploy_ended: a Bex-initiated zero-downtime deploy completed for a service",
  },
  "webhooks.event.image_pull_failed": {
    message: "イメージ取得失敗",
    description: "Event label — image_pull_failed",
  },
  "webhooks.event.pipeline_minutes_exhausted": {
    message: "パイプライン時間の枯渇",
    description: "Event label — pipeline_minutes_exhausted",
  },
  "webhooks.event.edge_cache_enabled": {
    message: "エッジキャッシュ有効化",
    description:
      "Event label — edge_cache_enabled (API-only on Render; shown on subscriptions and deliveries, not in the picker)",
  },
  "webhooks.event.edge_cache_disabled": {
    message: "エッジキャッシュ無効化",
    description:
      "Event label — edge_cache_disabled (API-only on Render; shown on subscriptions and deliveries, not in the picker)",
  },
  "webhooks.event.edge_cache_purged": {
    message: "エッジキャッシュパージ",
    description:
      "Event label — edge_cache_purged (API-only on Render; shown on subscriptions and deliveries, not in the picker)",
  },
  "webhooks.event.commit_ignored": {
    message: "コミット無視",
    description: "Event label — commit_ignored",
  },
  "webhooks.event.server_failed": {
    message: "サーバー障害",
    description: "Event label — server_failed",
  },
  "webhooks.event.server_hardware_failure": {
    message: "サーバーのハードウェア障害",
    description: "Event label — server_hardware_failure",
  },
  "webhooks.event.server_available": {
    message: "サーバー利用可能",
    description: "Event label — server_available",
  },
  "webhooks.event.autoscaling_started": {
    message: "オートスケーリング開始",
    description: "Event label — autoscaling_started",
  },
  "webhooks.event.autoscaling_ended": {
    message: "オートスケーリング終了",
    description: "Event label — autoscaling_ended",
  },
  "webhooks.event.branch_changed": {
    message: "ブランチ変更",
    description: "Event label — branch_changed bex extension",
  },
  "webhooks.event.cron_job_run_started": {
    message: "Cron Job 実行開始",
    description: "Event label — cron_job_run_started",
  },
  "webhooks.event.cron_job_run_ended": {
    message: "Cron Job 実行終了",
    description: "Event label — cron_job_run_ended",
  },
  "webhooks.event.maintenance_mode_enabled": {
    message: "メンテナンスモード有効化",
    description: "Event label — maintenance_mode_enabled",
  },
  "webhooks.event.maintenance_mode_uri_updated": {
    message: "メンテナンスモードの URI 更新",
    description: "Event label — maintenance_mode_uri_updated",
  },
  "webhooks.event.disk_created": {
    message: "ディスク作成",
    description: "Event label — disk_created",
  },
  "webhooks.event.disk_updated": {
    message: "ディスク更新",
    description: "Event label — disk_updated",
  },
  "webhooks.event.disk_deleted": {
    message: "ディスク削除",
    description: "Event label — disk_deleted",
  },
  "webhooks.event.disk_snapshot_restored": {
    message: "ディスクスナップショット復元",
    description: "Event label — disk_snapshot_restored",
  },
  "webhooks.event.postgres_created": {
    message: "Postgres 作成",
    description: "Event label — postgres_created",
  },
  "webhooks.event.postgres_restarted": {
    message: "Postgres 再起動",
    description: "Event label — postgres_restarted",
  },
  "webhooks.event.postgres_credentials_created": {
    message: "Postgres 認証情報作成",
    description: "Event label — postgres_credentials_created",
  },
  "webhooks.event.postgres_credentials_deleted": {
    message: "Postgres 認証情報削除",
    description: "Event label — postgres_credentials_deleted",
  },
  "webhooks.event.postgres_backup_started": {
    message: "Postgres バックアップ開始",
    description: "Event label — postgres_backup_started",
  },
  "webhooks.event.postgres_available": {
    message: "Postgres 利用可能",
    description: "Event label — postgres_available",
  },
  "webhooks.event.postgres_unavailable": {
    message: "Postgres 利用不可",
    description: "Event label — postgres_unavailable",
  },
  "webhooks.event.postgres_ha_status_changed": {
    message: "Postgres 高可用性の変更",
    description: "Event label — postgres_ha_status_changed",
  },
  "webhooks.event.postgres_cluster_leader_changed": {
    message: "Postgres クラスターリーダーの変更",
    description: "Event label — postgres_cluster_leader_changed",
  },
  "webhooks.event.postgres_connection_pool_enabled_changed": {
    message: "Postgres コネクションプールの変更",
    description: "Event label — postgres_connection_pool_enabled_changed",
  },
  "webhooks.event.postgres_disk_size_changed": {
    message: "Postgres ディスクサイズの変更",
    description: "Event label — postgres_disk_size_changed",
  },
  "webhooks.event.postgres_disk_autoscaling_enabled_changed": {
    message: "Postgres ディスクオートスケーリングの変更",
    description: "Event label — postgres_disk_autoscaling_enabled_changed",
  },
  "webhooks.event.postgres_read_replicas_changed": {
    message: "Postgres リードレプリカの変更",
    description: "Event label — postgres_read_replicas_changed",
  },
  "webhooks.event.postgres_backup_completed": {
    message: "Postgres バックアップ完了",
    description: "Event label — postgres_backup_completed",
  },
  "webhooks.event.postgres_backup_failed": {
    message: "Postgres バックアップ失敗",
    description: "Event label — postgres_backup_failed",
  },
  "webhooks.event.postgres_pitr_checkpoint_started": {
    message: "Postgres PITR チェックポイント開始",
    description: "Event label — postgres_pitr_checkpoint_started",
  },
  "webhooks.event.postgres_pitr_checkpoint_completed": {
    message: "Postgres PITR チェックポイント完了",
    description: "Event label — postgres_pitr_checkpoint_completed",
  },
  "webhooks.event.postgres_pitr_checkpoint_failed": {
    message: "Postgres PITR チェックポイント失敗",
    description: "Event label — postgres_pitr_checkpoint_failed",
  },
  "webhooks.event.postgres_wal_archive_failed": {
    message: "Postgres WAL アーカイブ失敗",
    description: "Event label — postgres_wal_archive_failed",
  },
  "webhooks.event.postgres_restore_succeeded": {
    message: "Postgres 復元成功",
    description: "Event label — postgres_restore_succeeded",
  },
  "webhooks.event.postgres_restore_failed": {
    message: "Postgres 復元失敗",
    description: "Event label — postgres_restore_failed",
  },
  "webhooks.event.postgres_upgrade_started": {
    message: "Postgres アップグレード開始",
    description: "Event label — postgres_upgrade_started",
  },
  "webhooks.event.postgres_upgrade_succeeded": {
    message: "Postgres アップグレード成功",
    description: "Event label — postgres_upgrade_succeeded",
  },
  "webhooks.event.postgres_upgrade_failed": {
    message: "Postgres アップグレード失敗",
    description: "Event label — postgres_upgrade_failed",
  },
  "webhooks.event.key_value_available": {
    message: "Key Value 利用可能",
    description: "Event label — key_value_available",
  },
  "webhooks.event.key_value_unhealthy": {
    message: "Key Value 異常",
    description: "Event label — key_value_unhealthy",
  },
  "webhooks.event.key_value_config_restart": {
    message: "設定変更による Key Value 再起動",
    description: "Event label — key_value_config_restart",
  },
  "webhooks.event.service_suspended": {
    message: "サービス一時停止",
    description: "Event label — service_suspended",
  },
  "webhooks.event.service_resumed": {
    message: "サービス再開",
    description: "Event label — service_resumed",
  },
  "webhooks.event.service_hibernated": {
    message: "サービスがスリープ状態に移行",
    description: "Event label — service_hibernated",
  },
  "webhooks.event.service_woken": {
    message: "サービスが起動",
    description: "Event label — service_woken",
  },
  "webhooks.event.agent_session_turn_succeeded": {
    message: "エージェントセッションのターン成功",
    description: "Event label — agent_session_turn_succeeded",
  },
  "webhooks.event.agent_session_turn_failed": {
    message: "エージェントセッションのターン失敗",
    description: "Event label — agent_session_turn_failed",
  },
  "webhooks.event.agent_session_needs_input": {
    message: "エージェントセッションが入力待ち",
    description: "Event label — agent_session_needs_input",
  },
  "webhooks.event.service_moved": {
    message: "サービス移動",
    description:
      "Event label — service_moved (bex extension: project/environment placement change)",
  },
  "webhooks.event.server_restarted": {
    message: "サーバー再起動",
    description: "Event label — server_restarted",
  },
  "webhooks.event.instance_count_changed": {
    message: "インスタンス数の変更",
    description: "Event label — instance_count_changed",
  },
  "webhooks.event.autoscaling_config_changed": {
    message: "オートスケーリング設定の変更",
    description: "Event label — autoscaling_config_changed",
  },
  "webhooks.event.plan_changed": {
    message: "プランの変更",
    description:
      "Event label — plan_changed (bex's slot for Render's Instance Type Changed)",
  },
  "webhooks.newTitle": {
    message: "新しい Webhook を作成",
    description: "Create page heading (Render: 'Create a new Webhook')",
  },
  "webhooks.newMenuLabel": {
    message: "新規 Webhook",
    description:
      "+ New menu entry; worded like the other entries, unlike the create page heading",
  },
  "webhooks.backToList": {
    message: "Webhook 一覧に戻る",
    description: "Accessible label of the create/detail pages' back link",
  },
  "webhooks.fieldNameHelp": {
    message: "この Webhook の一意の名前です。",
    description: "Create page name-field helper copy",
  },
  "webhooks.fieldUrlHelp": {
    message: "bex は各通知を、この URL に POST リクエストとして送信します。",
    description: "Create page URL-field helper copy",
  },
  "webhooks.fieldUrlPlaceholder": {
    message: "https://example.com/webhooks/bex",
    description: "Create page URL-field placeholder",
  },
  "webhooks.fieldEventsHelp": {
    message:
      "ワークスペース内のどのイベントで Webhook 通知を送信するかを選択してください。",
    description: "Create page events-field helper copy",
  },
  "webhooks.detailKicker": {
    message: "Webhook",
    description: "Detail page header kicker above the endpoint name",
  },
  "webhooks.idLabel": {
    message: "Webhook ID:",
    description: "Detail header id row label",
  },
  "webhooks.copyId": {
    message: "Webhook ID をコピー",
    description: "Accessible label of the header id copy button",
  },
  "webhooks.copyUrl": {
    message: "エンドポイント URL をコピー",
    description: "Accessible label of the header URL copy button",
  },
  "webhooks.copiedGeneric": {
    message: "クリップボードにコピーしました",
    description: "Toast after copying the id or URL",
  },
  "webhooks.showMore": {
    message: "さらに {count} 件を表示",
    description: "Event-chip expander on the detail header",
  },
  "webhooks.showLess": {
    message: "表示を減らす",
    description: "Event-chip collapser on the detail header",
  },
  "webhooks.createdByOn": {
    message: "{creator} が {date} に作成",
    description: "Detail header provenance line when the creator is known",
  },
  "webhooks.createdOn": {
    message: "{date} に作成",
    description: "Detail header provenance line without a creator",
  },
  "webhooks.tabActivity": {
    message: "アクティビティ",
    description: "Detail page tab — delivery history",
  },
  "webhooks.tabSettings": {
    message: "設定",
    description: "Detail page tab — edit + delete",
  },
  "webhooks.recentDeliveries": {
    message: "最近の配信",
    description: "Activity tab heading",
  },
  "webhooks.recentDeliveriesHint": {
    message:
      "各行は 1 回の配信試行です。このタブを表示している間は、新しい試行が自動的に表示されます。",
    description: "Activity tab heading hint for immutable attempts + polling",
  },
  "webhooks.refresh": {
    message: "更新",
    description: "Accessible label of the Activity refresh button",
  },
  "webhooks.filterAll": {
    message: "すべて",
    description: "Activity delivery filter tab",
  },
  "webhooks.filterSuccessful": {
    message: "成功",
    description: "Activity delivery filter tab — delivered only",
  },
  "webhooks.filterFailed": {
    message: "失敗",
    description: "Activity delivery filter tab — failed only",
  },
  "webhooks.sentAfter": {
    message: "送信日時（以降）",
    description: "Activity delivery-history lower timestamp bound",
  },
  "webhooks.sentBefore": {
    message: "送信日時（以前）",
    description: "Activity delivery-history upper timestamp bound",
  },
  "webhooks.destinationBlocked": {
    message: "許可されていない宛先",
    description:
      "Delivery result label when bex refused to dial the destination: it resolves to a loopback, private, link-local or cluster-internal address (w21/046)",
  },
  "webhooks.destinationUnreachableNotice": {
    message:
      "このアドレスにはインターネットから到達できないため、このアドレスへの配信はすべて失敗します。公開されている HTTPS アドレスを使用してください。",
    description:
      "Non-blocking notice under the webhook Destination URL when its host is a loopback, private, link-local or cluster-internal address (w21/046)",
  },
  "webhooks.transportError": {
    message: "トランスポートエラー",
    description: "Delivery evidence label when no HTTP response was received",
  },
  "webhooks.enabledBadge": {
    message: "有効",
    description: "Detail header status badge while enabled",
  },
  "webhooks.disabledBadge": {
    message: "無効",
    description: "Detail header status badge while disabled",
  },
  "webhooks.view": {
    message: "Webhook を表示",
    description: "Accessible label of the list row's open-detail link",
  },
  "webhooks.settingsGeneral": {
    message: "一般",
    description: "Settings tab first section heading",
  },
  "webhooks.disabledReasonAuto": {
    message: "配信の失敗が繰り返されたため、自動的に無効になりました。",
    description:
      "Reason shown beside a webhook the platform disabled after repeated delivery failures",
  },
  "webhooks.settingsStatus": {
    message: "ステータス",
    description: "Settings status-toggle label",
  },
  "webhooks.settingsStatusHelp": {
    message: "無効の間、Webhook は通知を送信しません。",
    description: "Settings status-toggle helper copy",
  },
  "webhooks.settingsEvents": {
    message: "購読中のイベント",
    description: "Settings events section heading",
  },
  "webhooks.saveChanges": {
    message: "変更を保存",
    description: "Settings submit button (disabled until dirty)",
  },
  "webhooks.updateSuccess": {
    message: "Webhook を更新しました",
    description: "Toast after a successful settings save",
  },
  "webhooks.updateError": {
    message: "Webhook を更新できませんでした",
    description: "Toast after a failed settings save",
  },
  "webhooks.secretLabel": {
    message: "署名シークレット",
    description: "Settings signing-secret row label",
  },
  "webhooks.secretHelp": {
    message:
      "リクエストのペイロードの署名と検証に使用するトークンです。表示できるのはワークスペースの管理者のみです。",
    description: "Settings signing-secret row help (Render's wording)",
  },
  "webhooks.secretShow": {
    message: "シークレットを表示",
    description: "Settings button that reveals the masked signing secret",
  },
  "webhooks.secretHide": {
    message: "シークレットを非表示",
    description: "Settings button that masks the revealed signing secret",
  },
  "webhooks.deleteSection": {
    message: "Webhook を削除",
    description: "Settings danger-zone heading",
  },
  "webhooks.deleteTypeToConfirm": {
    message: "確認のため、下に {command} と入力してください。",
    description: "Type-to-confirm instruction in the delete dialog",
  },
  "webhooks.deleteCommandLabel": {
    message: "確認",
    description: "Accessible label of the type-to-confirm input",
  },
};

export default jaWebhooks;
