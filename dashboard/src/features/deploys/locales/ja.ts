import type { TranslationEntry } from "@/i18n";

const jaDeploys: Record<string, TranslationEntry> = {
  "deploys.statusCreated": {
    message: "作成済み",
    description: "Deploy status: created",
  },
  "deploys.statusQueued": {
    message: "待機中",
    description: "Deploy status: queued",
  },
  "deploys.statusBuildInProgress": {
    message: "ビルド中",
    description: "Deploy status: build_in_progress",
  },
  "deploys.statusBuildFailed": {
    message: "ビルド失敗",
    description: "Deploy status: build_failed",
  },
  "deploys.statusPreDeployInProgress": {
    message: "プレデプロイ実行中",
    description: "Deploy status: pre_deploy_in_progress",
  },
  "deploys.statusPreDeployFailed": {
    message: "プレデプロイ失敗",
    description: "Deploy status: pre_deploy_failed",
  },
  "deploys.statusUpdateInProgress": {
    message: "実行中",
    description: "Deploy status: update_in_progress",
  },
  "deploys.statusUpdateFailed": {
    message: "失敗",
    description: "Deploy status: update_failed",
  },
  "deploys.statusLive": {
    message: "公開中",
    description: "Deploy status: live",
  },
  "deploys.statusCanceled": {
    message: "キャンセル済み",
    description: "Deploy status: canceled",
  },
  "deploys.statusDeactivated": {
    message: "無効化済み",
    description: "Deploy status: deactivated",
  },
  "deploys.statusUnknown": {
    message: "不明",
    description: "Deploy status: unrecognized backend value",
  },
  "deploys.imageDigestCopy": {
    message: "イメージダイジェストをコピー",
    description:
      "Deploy header: accessible label of the button copying the image digest the deploy ran",
  },
  "deploys.imageDigestCopied": {
    message: "イメージダイジェストをコピーしました",
    description: "Deploy header: toast after copying the image digest",
  },
  "deploys.imageDigestCopyError": {
    message: "クリップボードにコピーできませんでした",
    description: "Deploy header: toast when copying the image digest fails",
  },
  "deploys.duration": {
    message: "所要時間",
    description: "Deploy header: elapsed build/deploy time label",
  },
  "deploys.factStatus": {
    message: "ステータス",
    description:
      "Deploy page: label of the deploy's Status fact (Render's STATUS)",
  },
  "deploys.factDeployed": {
    message: "デプロイ日時",
    description:
      "Deploy page: label of the fact giving when the deploy was created (Render's DEPLOYED)",
  },
  "deploys.factSource": {
    message: "ソース",
    description:
      "Deploy page: label of the fact naming the commit or image the deploy ran (Render's SOURCE)",
  },
  "deploys.factScaling": {
    message: "スケーリング",
    description:
      "Deploy page (w1/m661, Render's DeployDetailsHeader): label of the fact counting the service's instances that run this deploy",
  },
  "deploys.scalingInstances_other": {
    message: "{deployed} / {total} インスタンス",
    description:
      "Deploy page (w1/m661): the Scaling fact's value; {deployed} is how many of the service's instances run this deploy and {total} how many it needs, both shown as numbers (other)",
  },
  "deploys.factPreDeploy": {
    message: "プレデプロイ",
    description:
      "Deploy page: label of the fact giving the outcome of the deploy's pre-deploy command",
  },
  "deploys.preDeployRunning": {
    message: "実行中",
    description:
      "Deploy page: Pre-deploy fact value while the pre-deploy command runs",
  },
  "deploys.preDeploySucceeded": {
    message: "成功",
    description:
      "Deploy page: Pre-deploy fact value once the pre-deploy command succeeded",
  },
  "deploys.preDeployFailed": {
    message: "失敗",
    description:
      "Deploy page: Pre-deploy fact value when the pre-deploy command failed",
  },
  "deploys.preDeployCanceled": {
    message: "キャンセル済み",
    description:
      "Deploy page: Pre-deploy fact value when the pre-deploy command was canceled",
  },
  "deploys.idLabel": {
    message: "デプロイID",
    description:
      "Deploy page: the deploy's id, named in its copy button's accessible name",
  },
  "deploys.idCopied": {
    message: "デプロイIDをコピーしました",
    description: "Deploy page: toast after copying the deploy's id",
  },
  "deploys.idCopyFailed": {
    message: "デプロイIDをコピーできませんでした",
    description: "Deploy page: toast when copying the deploy's id fails",
  },
  "deploys.loadErrorTitle": {
    message: "このデプロイを読み込めませんでした",
    description:
      "Deploy page: title of the state shown when the deploy itself could not be read",
  },
  "deploys.loadErrorBody": {
    message:
      "現在このデプロイを読み込めません。デプロイ自体には影響ありません。",
    description:
      "Deploy page: body of the state shown when the deploy itself could not be read; says the deploy is unaffected",
  },
  "deploys.durationValue": {
    message: "所要時間 {duration}",
    description: "Deploy row: elapsed build/deploy time",
  },
  "deploys.durationInProgress": {
    message: "所要時間 計測中",
    description: "Deploy row: elapsed time is still running",
  },
  "deploys.durationActive": {
    message: "実行中",
    description:
      "Deploys tab Duration column: bare running-elapsed marker (the column header supplies the label)",
  },
  "deploys.deployedAt": {
    message: "{timestamp} にデプロイ",
    description:
      'Deploy row: how long ago a live/deactivated deploy went live — {timestamp} is elapsed text like "2 hours ago" (its finish time; createdAt fallback)',
  },
  "deploys.createdAt": {
    message: "{timestamp} に作成",
    description:
      'Deploy row: how long ago a deploy that hasn\'t finished (created/queued/in-progress) was created — {timestamp} is elapsed text like "2 minutes ago"',
  },
  "deploys.canceledAt": {
    message: "{timestamp} にキャンセル",
    description:
      'Deploy row: how long ago a canceled deploy was canceled — {timestamp} is elapsed text like "3 days ago" (its finish time; createdAt fallback)',
  },
  "deploys.failedAt": {
    message: "{timestamp} に失敗",
    description:
      'Deploy row: how long ago a build/pre-deploy/update failure landed — {timestamp} is elapsed text like "3 days ago" (its finish time; createdAt fallback)',
  },
  "deploys.notYet": {
    message: "—",
    description:
      "Deploy header: placeholder for a timestamp that hasn't happened yet",
  },
  "deploys.triggerCreate": {
    message: "初回デプロイ",
    description: "Deploy header: trigger=create label",
  },
  "deploys.triggerApi": {
    message: "API デプロイ",
    description: "Deploy header: trigger=api label (an API-key deploy, w4/413)",
  },
  "deploys.triggerManual": {
    message: "手動デプロイ",
    description:
      "Deploy header: trigger=manual label (a dashboard click, w4/413)",
  },
  "deploys.triggerRollback": {
    message: "{deployId} へのロールバック",
    description:
      "Deploy header: trigger label for a rollback deploy, naming the restored deploy (id must stay literal — never wrap in CSS capitalize)",
  },
  "deploys.triggerDeployHook": {
    message: "Deploy Hook",
    description: "Deploy row: trigger=deploy_hook label (secret-URL trigger)",
  },
  "deploys.triggerBlueprint": {
    message: "Blueprint 同期",
    description: "Deploy row: trigger=blueprint label (Blueprint sync deploy)",
  },
  "deploys.triggerNewCommit": {
    message: "新しいコミット",
    description: "Deploy row: trigger=new_commit label (git push redeploy)",
  },
  "deploys.triggerConfigChange": {
    message: "設定の変更",
    description:
      "Deploy row: trigger=config_change label (an env var, secret file, or env-group edit that deployed)",
  },
  "deploys.triggerSettingsChange": {
    message: "設定の変更",
    description:
      "Deploy row: trigger=settings_change label (a Settings save that deployed, or a plan change; w8/m135)",
  },
  "deploys.triggerRestart": {
    message: "再起動",
    description:
      "Deploy row: trigger=restart label (a Restart roll of the live release, w4/379)",
  },
  "deploys.triggerServiceResumed": {
    message: "サービス再開",
    description:
      "Deploy row: trigger=service_resumed label (the deploy a Resume opened on the release the service was serving, Render's service_resumed, w3/m643)",
  },
  "deploys.triggerDeployedByRender": {
    message: "プラットフォームによる再ビルド",
    description:
      "Deploy row: trigger=deployed_by_render label (the platform rebuilt the live commit, e.g. to ship a runtime security patch)",
  },
  "deploys.logSearchPlaceholder": {
    message: "ログを検索…",
    description: "Deploy detail page: log search input placeholder",
  },
  "deploys.logTypeFilter": {
    message: "ログの種類",
    description:
      "Deploy log viewer: aria-label of the log-type filter dropdown (Render's All/Application/Build selector)",
  },
  "deploys.logTypeAll": {
    message: "すべてのログ",
    description: "Deploy log viewer type filter: show every log type",
  },
  "deploys.logTypeApp": {
    message: "アプリケーションログ",
    description:
      "Deploy log viewer type filter: app + pre-deploy lines, matching Render's Application bucket",
  },
  "deploys.logTypeBuild": {
    message: "ビルドログ",
    description: "Deploy log viewer type filter: build lines only",
  },
  "deploys.buildLogsStoreUnavailable": {
    message: "過去のビルドログは利用できません",
    description:
      "Deploy detail page: title shown when the durable log store isn't wired",
  },
  "deploys.buildLogsStoreUnavailableBody": {
    message:
      "このダッシュボードは永続ログストレージに接続されていません。このデプロイのアプリケーションログは引き続き表示される場合があります。",
    description:
      "Deploy detail page: accurate explanation of a storeless build-log response",
  },
  "deploys.detailRefreshError": {
    message:
      "このデプロイを更新できませんでした。最後に読み込んだ詳細を表示しています。",
    description:
      "Deploy detail page: a background refresh of an already loaded deploy failed; the page keeps the last good details and offers a retry",
  },
  "deploys.logPartiallyLoaded": {
    message:
      "この期間の一部のみ読み込まれています。さらに前の行を検索するには、古いログを読み込んでください。",
    description:
      "Deploy log viewer banner while older history remains unloaded. The panel's search and type/instance filters run over loaded lines only, so it must not claim a number of matches",
  },
  "deploys.logStepEndMissing": {
    message:
      "このデプロイはプレデプロイのステップが完了する前にキャンセルされ、ステップの完了時刻が記録されなかったため、その後の出力がこの範囲外になる場合があります。ログのオプションでより広い範囲を選択すると確認できます。",
    description:
      "Deploy log viewer: a canceled deploy's continued pre-deploy step has no recorded end, so the deploy window may omit its later output",
  },
  "deploys.logStepEndUnavailable": {
    message:
      "このデプロイのプレデプロイのステップがいつ完了したかを確認できなかったため、その後のステップの出力がこの範囲外になる場合があります。もう一度お試しいただくか、ログのオプションでより広い範囲を選択してください。",
    description:
      "Deploy log viewer: reading a canceled deploy's continued pre-deploy step end failed; offers a retry",
  },
  "deploys.timelineTitle": {
    message: "ステータスのタイムライン",
    description: "Deploy detail page: status-timeline card title",
  },
  "deploys.timelineCreated": {
    message: "デプロイを作成",
    description: "Deploy timeline: deploy row was created",
  },
  "deploys.timelineStarted": {
    message: "デプロイを開始",
    description: "Deploy timeline: backend-provided startedAt timestamp",
  },
  "deploys.timelineInProgress": {
    message: "デプロイ実行中",
    description: "Deploy timeline: current non-terminal deploy status",
  },
  "deploys.timelineLive": {
    message: "デプロイが公開中",
    description: "Deploy timeline: successful terminal state",
  },
  "deploys.timelineFailed": {
    message: "デプロイに失敗",
    description: "Deploy timeline: failed terminal state",
  },
  "deploys.timelineCanceled": {
    message: "デプロイをキャンセル",
    description: "Deploy timeline: canceled terminal state",
  },
  "deploys.timelineDeactivated": {
    message: "デプロイを無効化",
    description: "Deploy timeline: deactivated terminal state",
  },
  "deploys.timelineEventsUnavailable": {
    message:
      "サービスのイベントを利用できないため、デプロイのタイムスタンプのみを表示しています。",
    description:
      "Deploy timeline: graceful fallback when service-events query fails",
  },
  "deploys.autoDeployOffTitle": {
    message: "自動デプロイはオフです",
    description:
      "Deploys page banner title when the service's auto-deploy is off (Render, w3/064)",
  },
  "deploys.autoDeployOffBody": {
    message:
      "{branch} へのプッシュではこのサービスはデプロイされません。手動でデプロイするか、自動デプロイをオンに戻してください。",
    description: "Deploys page banner body; {branch} is the tracked git branch",
  },
  "deploys.autoDeployOffBodyNoBranch": {
    message:
      "プッシュしてもこのサービスはデプロイされません。手動でデプロイするか、自動デプロイをオンに戻してください。",
    description: "Deploys page banner body when the branch is unknown",
  },
  "deploys.autoDeployOffSettings": {
    message: "自動デプロイの設定",
    description: "Deploys page banner link to the service Settings page",
  },
  "deploys.deploysManualTitle": {
    message: "デプロイは手動です",
    description:
      "Deploys page banner title for a repo whose pushes can't reach bex (pushDeliveryMethod unavailable)",
  },
  "deploys.deploysManualBody": {
    message:
      "bex はこのリポジトリのプッシュを受信しないため、プッシュしてもこのサービスはデプロイされません。手動または Deploy Hook でデプロイするか、GitHub アプリ経由でリポジトリを接続してください。",
    description:
      "Deploys page banner body for a repo whose pushes can't reach bex: no Auto-Deploy setting can turn deploys on push on (Render: a public Git URL must be deployed manually)",
  },
  "deploys.columnDeploy": {
    message: "デプロイ",
    description:
      "Deploys tab table: first column header (status, id, commit, timestamp)",
  },
  "deploys.columnTrigger": {
    message: "トリガー",
    description: "Deploys tab table: column header for what triggered a deploy",
  },
  "deploys.columnDuration": {
    message: "所要時間",
    description:
      "Deploys tab table: column header for elapsed build/deploy time",
  },
  "deploys.columnActions": {
    message: "操作",
    description:
      "Deploys tab table: accessible-only header over the Cancel/Rollback action column",
  },
  "deploys.listEmpty": {
    message: "デプロイはまだありません。",
    description: "Deploys tab: empty state with no status filter",
  },
  "deploys.listEmptyFiltered": {
    message: "選択したステータスに一致するデプロイはありません。",
    description: "Deploys tab: empty state while a status filter is active",
  },
  "deploys.listStatusFilterLabel": {
    message: "ステータスで絞り込み",
    description: "Deploys tab: aria-label of the status filter dropdown",
  },
  "deploys.listStatusAll": {
    message: "すべてのステータス",
    description: "Deploys tab: status filter option matching every deploy",
  },
  "deploys.listSearchPlaceholder": {
    message: "デプロイとコミットを検索",
    description:
      "Placeholder of the deploy list's search field on every service's Deploys page (Render's text, w3/m555)",
  },
  "deploys.listSearchLabel": {
    message: "読み込み済みのデプロイとコミットを検索",
    description: "Deploys tab: accessible label for deploy-history search",
  },
  "deploys.manualBuild": {
    message: "手動ビルド",
    description:
      "Button beside a repo-backed cron job's deploy search that builds its latest commit and opens the new build (Render's Manual Build, w3/m555)",
  },
  "deploys.listClearSearch": {
    message: "検索をクリア",
    description:
      "Deploys tab: button under the no-match message that empties the search box",
  },
  "deploys.listMatchCount": {
    message: "読み込み済み {total} 件中 {matched} 件が一致",
    description:
      "Deploys tab: count shown while a search is typed; matched = loaded deploys that match, total = loaded deploys",
  },
  "deploys.listLoadErrorTitle": {
    message: "デプロイを読み込めませんでした",
    description:
      "Deploys tab: heading of the error state when the deploy history could not be read; a Try again button follows",
  },
  "deploys.listLoadErrorBody": {
    message:
      "デプロイ自体に影響はありません。接続を確認して、もう一度お試しください。",
    description:
      "Deploys tab: body of the error state when the deploy history could not be read and the server gave no message; a Try again button follows",
  },
  "deploys.listEmptySearch": {
    message: "検索に一致する読み込み済みのデプロイはありません。",
    description:
      "Deploys tab: empty state when client-side search has no match",
  },
  "deploys.listCount_other": {
    message: "{count} 件のデプロイ",
    description: "Deploys tab: complete deploy count (plural)",
  },
  "deploys.listCountLoaded_other": {
    message: "{count} 件のデプロイを読み込み済み",
    description:
      "Deploys tab: loaded count when more server pages remain (plural)",
  },
  "deploys.listLoadMore": {
    message: "さらに読み込む",
    description: "Deploys tab: button fetching the next page of deploy history",
  },
  "deploys.listLoadMoreError": {
    message: "古いデプロイを読み込めませんでした。",
    description:
      "Deploys tab: toast when reading an older page of deploy history fails",
  },
  "deploys.logRangeLabel": {
    message: "期間",
    description:
      "Deploy log viewer options menu: heading over the time-range choices",
  },
  "deploys.logRangeDeploy": {
    message: "デプロイの期間",
    description:
      "Deploy log viewer: the default time range — the deploy's own createdAt..finishedAt window",
  },
  "deploys.logRangeLast15m": {
    message: "過去 15 分",
    description: "Deploy log viewer: relative time-range option (?r=15m)",
  },
  "deploys.logRangeLast1h": {
    message: "過去 1 時間",
    description: "Deploy log viewer: relative time-range option (?r=1h)",
  },
  "deploys.logRangeLast6h": {
    message: "過去 6 時間",
    description: "Deploy log viewer: relative time-range option (?r=6h)",
  },
  "deploys.logRangeLast24h": {
    message: "過去 24 時間",
    description: "Deploy log viewer: relative time-range option (?r=24h)",
  },
  "deploys.logRangeLast7d": {
    message: "過去 7 日間",
    description: "Deploy log viewer: relative time-range option (?r=7d)",
  },
  "deploys.logWrap": {
    message: "行を折り返す",
    description:
      "Deploy log viewer options menu: toggle wrapping long log lines vs horizontal scroll",
  },
  "deploys.logTimestamps": {
    message: "タイムスタンプを表示",
    description:
      "Deploy log viewer options menu: toggle the per-line timestamp column",
  },
  "deploys.reason.http_port_waiting": {
    message: "コンテナーの起動を待ってから HTTP ポートを検出します",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_waiting, w4/566)",
  },
  "deploys.reason.http_port_detecting": {
    message: "コンテナーの HTTP ポートを検出しています",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_detecting, w4/566)",
  },
  "deploys.reason.http_port_limit": {
    message:
      "検出された HTTP ポートがサービスの 75 ポートの上限を超えています。PORT を明示的に設定してください",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_limit, w4/566)",
  },
  "deploys.reason.http_port_detected": {
    message: "ポート {port} で HTTP を検出しました。このポートで提供します",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_detected, w4/566); params are the sentence's values",
  },
  "deploys.reason.http_port_not_answering": {
    message:
      "ポート {port} は接続を受け付けましたが、まだ HTTP に応答していません。コンテナーの起動中はスキャンを続けます",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_not_answering, w4/566); params are the sentence's values",
  },
  "deploys.reason.http_port_none_yet": {
    message:
      "HTTP に応答するポートはまだありません。コンテナーの起動中はスキャンを続けます",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_none_yet, w4/566)",
  },
  "deploys.reason.http_port_fallback": {
    message:
      "HTTP に応答するポートがなかったため、フォールバックのポート {port} を使用します。HTTP リスナーが別のポートにバインドされている場合は、PORT をそのポートに設定してください",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_fallback, w4/566); params are the sentence's values",
  },
  "deploys.reason.http_port_localhost_only": {
    message:
      "0.0.0.0 で開いているポートは検出されませんでしたが、localhost でポートが開いています ({ports})。サーバーをホスト 0.0.0.0 にバインドしてください。コンテナーの起動中はスキャンを続けます",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_localhost_only, w8/388); params are the sentence's values: ports is the comma-separated list of ports the process listens on only at localhost",
  },
  "deploys.reason.http_port_none_http": {
    message:
      "0.0.0.0 でポートが開いています ({ports}) が、まだどれも HTTP に応答しません。コンテナーの起動中はスキャンを続けます。サービスが公開 HTTP トラフィックを受信する必要がない場合は、代わりに Private Service を作成してください",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_none_http, w8/388); params are the sentence's values: ports is the comma-separated list of open ports that answer no HTTP",
  },
  "deploys.reason.image_not_found": {
    message:
      "イメージ {image} がレジストリに見つかりませんでした。リポジトリまたはタグが存在しません。イメージの URL とタグを確認するか、存在するダイジェストをデプロイしてください",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_not_found, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_credential_refused": {
    message:
      "{host} がイメージ {image} に対するこのサービスのレジストリ認証情報を拒否しました (HTTP {status})。認証情報のユーザー名とトークンを確認するか、このサービスから認証情報を削除して、パブリックイメージを匿名で取得してください",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_credential_refused, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_access_refused": {
    message:
      "レジストリがイメージ {image} へのアクセスを拒否しました。イメージがプライベートの場合は、{host} のレジストリ認証情報をこのサービスに追加してください",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_access_refused, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_read_refused": {
    message:
      "レジストリがイメージ {image} の読み取りを拒否しました (HTTP {status})。イメージの URL とタグを確認してください",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_read_refused, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_registry_unreachable": {
    message:
      "レジストリ {host} が {minutes} 分間イメージ {image} の読み取りに応答しませんでした。しばらく待ってからもう一度デプロイするか、レジストリ提供元のステータスページを確認してください",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_registry_unreachable, w4/692); params are the sentence's values",
  },
  "deploys.reason.image_registry_unavailable": {
    message:
      "レジストリ {host} が利用できず、イメージ {image} を提供できませんでした。しばらく待ってからもう一度デプロイするか、レジストリ提供元のステータスページを確認してください",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_registry_unavailable, w4/m669): Render's Registry unavailable cause; params are the sentence's values",
  },
  "deploys.reason.image_registry_rate_limited": {
    message:
      "レジストリ {host} がレート制限によりイメージ {image} の pull を拒否しました (HTTP 429)。レート制限がリセットされるまで待つか、レート制限の高いアカウントのレジストリ認証情報を使うか、イメージを別のレジストリにミラーしてください",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_registry_rate_limited, w4/m669): Render's Rate limited (429 error) cause with its three remedies; params are the sentence's values",
  },
  "deploys.reason.build_timed_out": {
    message:
      "ビルドがビルド時間の上限内に完了しませんでした。ビルドログを確認してください",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason build_timed_out, w4/566)",
  },
  "deploys.reason.pre_deploy_timed_out": {
    message:
      "プレデプロイコマンドが制限時間内に完了しませんでした。プレデプロイのログを確認してください",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_timed_out, w4/566)",
  },
  "deploys.reason.health_timed_out": {
    message:
      "デプロイがヘルスゲートの制限時間内に正常になりませんでした。サービスのログを確認してください",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason health_timed_out, w4/566)",
  },
  "deploys.reason.startup_out_of_memory": {
    message:
      "起動中にインスタンスのメモリが不足しました（コンピュートプランの上限は {memory}、終了コード {code}）。メモリ使用量を減らすか、より大きなコンピュートプランを選択してください",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason startup_out_of_memory, w4/630); params are the sentence's values: memory is the compute plan's memory limit (e.g. 512Mi), code the exit code",
  },
  "deploys.reason.startup_crash_restarting": {
    message:
      "コンテナが起動直後に終了し、再起動を繰り返しています（最後の終了コード {code}）。サービスのログでクラッシュ時の出力を確認してください。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason startup_crash_restarting, w8/386); params are the sentence's values: code is the exit code, hint the port advice (a part)",
  },
  "deploys.reason.startup_crash_stopped": {
    message:
      "コンテナが起動直後に終了し、そのインスタンスは停止しました（最後の終了コード {code}）。サービスのログでクラッシュ時の出力を確認してください。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason startup_crash_stopped, w8/386); params are the sentence's values: code is the exit code, hint the port advice (a part)",
  },
  "deploys.reason.startup_early_exit_restarting": {
    message:
      "コンテナが起動直後に終了し、再起動を繰り返しています（最後の終了コード 0）。プロセスは「正常に」終了しましたが、終了が早すぎます。サービスは継続して実行されるもので、SIGTERM を受け取るまで終了してはいけません。作業が終わると終了するタスクには Cron Job を使用してください。終了した理由をサービスのログで確認してください。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason startup_early_exit_restarting, w8/386); the start command exited with code 0; hint is the port advice (a part)",
  },
  "deploys.reason.startup_early_exit_stopped": {
    message:
      "コンテナが起動直後に終了し、そのインスタンスは停止しました（最後の終了コード 0）。プロセスは「正常に」終了しましたが、終了が早すぎます。サービスは継続して実行されるもので、SIGTERM を受け取るまで終了してはいけません。作業が終わると終了するタスクには Cron Job を使用してください。終了した理由をサービスのログで確認してください。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason startup_early_exit_stopped, w8/386); the start command exited with code 0; hint is the port advice (a part)",
  },
  "deploys.reason.probe_http_waiting": {
    message:
      "コンテナーは実行中ですが、{kind} ヘルスチェックが成功していないため、ロールアウトは待機中です: ポート {port} への GET {path}。そのパスがサービス自身のポートで 2xx または 3xx のステータスを返すことを確認してください。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_waiting, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_http_waiting_restarted": {
    message:
      "コンテナーは実行中ですが、{kind} ヘルスチェックが成功していないため、ロールアウトは待機中です: ポート {port} への GET {path}。ライブネスチェックによってコンテナーはすでに {restarts} 回再起動されており、単に遅いのではなくチェックが失敗しています。そのパスがサービス自身のポートで 2xx または 3xx のステータスを返すことを確認してください。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_waiting_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_http_failed": {
    message:
      "コンテナーは実行中ですが、ロールアウトの制限時間内に {kind} ヘルスチェックが一度も成功しなかったため、デプロイに失敗しました: ポート {port} への GET {path}。そのパスがサービス自身のポートで 2xx または 3xx のステータスを返すことを確認してください。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_failed, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_http_failed_restarted": {
    message:
      "コンテナーは実行中ですが、ロールアウトの制限時間内に {kind} ヘルスチェックが一度も成功しなかったため、デプロイに失敗しました: ポート {port} への GET {path}。ライブネスチェックによってコンテナーはすでに {restarts} 回再起動されており、単に遅いのではなくチェックが失敗しています。そのパスがサービス自身のポートで 2xx または 3xx のステータスを返すことを確認してください。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_failed_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_waiting": {
    message:
      "コンテナーは実行中ですが、{kind} ヘルスチェックが成功していないため、ロールアウトは待機中です: ポート {port} への TCP 接続。ヘルスチェックパスが設定されていないため、チェックは TCP 接続となり、そのポートでプロセスがリッスンしているかどうかだけを確認します。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_waiting, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_waiting_restarted": {
    message:
      "コンテナーは実行中ですが、{kind} ヘルスチェックが成功していないため、ロールアウトは待機中です: ポート {port} への TCP 接続。ライブネスチェックによってコンテナーはすでに {restarts} 回再起動されており、単に遅いのではなくチェックが失敗しています。ヘルスチェックパスが設定されていないため、チェックは TCP 接続となり、そのポートでプロセスがリッスンしているかどうかだけを確認します。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_waiting_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_failed": {
    message:
      "コンテナーは実行中ですが、ロールアウトの制限時間内に {kind} ヘルスチェックが一度も成功しなかったため、デプロイに失敗しました: ポート {port} への TCP 接続。ヘルスチェックパスが設定されていないため、チェックは TCP 接続となり、そのポートでプロセスがリッスンしているかどうかだけを確認します。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_failed, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_failed_restarted": {
    message:
      "コンテナーは実行中ですが、ロールアウトの制限時間内に {kind} ヘルスチェックが一度も成功しなかったため、デプロイに失敗しました: ポート {port} への TCP 接続。ライブネスチェックによってコンテナーはすでに {restarts} 回再起動されており、単に遅いのではなくチェックが失敗しています。ヘルスチェックパスが設定されていないため、チェックは TCP 接続となり、そのポートでプロセスがリッスンしているかどうかだけを確認します。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_failed_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.rollout_deadline_exceeded": {
    message: "ロールアウトが進行の制限時間内に正常になりませんでした",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason rollout_deadline_exceeded, w4/591)",
  },
  "deploys.reason.pre_deploy_window_exceeded": {
    message:
      "プレデプロイコマンドが {minutes} 分の制限時間内に完了しませんでした。プレデプロイのログを確認してください",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_window_exceeded, w4/591); params are the sentence's values",
  },
  "deploys.reason.pre_deploy_out_of_memory": {
    message:
      "プレデプロイコマンドでメモリ不足が発生しました (終了コード {code})。プレデプロイのログを確認してください",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_out_of_memory, w4/591); params are the sentence's values",
  },
  "deploys.reason.pre_deploy_exited": {
    message:
      "プレデプロイコマンドが終了コード {code} で終了しました。プレデプロイのログを確認してください",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_exited, w4/591); params are the sentence's values",
  },
  "deploys.reason.pre_deploy_failed": {
    message:
      "プレデプロイコマンドが失敗しました。プレデプロイのログを確認してください",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_failed, w4/591)",
  },
  "deploys.reason.pre_deploy_could_not_launch": {
    message:
      "プレデプロイコマンドを開始できませんでした: コンテナーランタイムがそのプログラムを起動できず、プログラムは一度も実行されなかったため、プレデプロイのログはありません",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_could_not_launch, w4/591)",
  },
  "deploys.reason.build_never_started": {
    message: "ビルドが開始されませんでした: {cause}",
    description:
      "A deploy failure reason by its code (bex-api DeployReason build_never_started, w4/591); {cause} is the operator's wait, whose sentences are worded as parts (w5/m356)",
  },
  "deploys.reason.build_abandoned": {
    message:
      "ビルドがビルド時間の上限内に完了せず、その理由を説明するためのこのサービスの App レコードも読み取れませんでした",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason build_abandoned, w4/591)",
  },
  "deploys.reason.deploy_abandoned": {
    message:
      "デプロイが制限時間内に完了せず、その理由を説明するためのこのサービスの App レコードも読み取れませんでした",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason deploy_abandoned, w4/591)",
  },
  "deploys.reason.platform_error": {
    message:
      "コードや設定の問題ではなく、プラットフォームのエラーにより、bex はこのリリースを適用できませんでした。参照番号 {ref} を添えてサポートにお問い合わせください",
    description:
      "A deploy failure reason by its code (bex-api DeployReason platform_error, w9/090): the platform refused one of bex's own changes; {ref} is the support reference, shown as-is",
  },
  "deploys.reason.canceled_suspended": {
    message: "キャンセル: サービスが一時停止されました",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_suspended, w4/591)",
  },
  "deploys.reason.superseded_by_newer_release": {
    message: "新しいリリースに置き換えられました",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason superseded_by_newer_release, w4/591)",
  },
  "deploys.reason.superseded_by": {
    message: "{deploy} に置き換えられました",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason superseded_by, w4/591); params are the sentence's values",
  },
  "deploys.reason.canceled_by_restart": {
    message:
      "実行中のリリースを維持した再起動 {deploy} によりキャンセルされました。このデプロイが適用しようとしていた内容はデプロイされていません",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_by_restart, w4/591); params are the sentence's values",
  },
  "deploys.reason.canceled_along_with": {
    message: "{deploy} とともにキャンセルされました",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_along_with, w4/591); params are the sentence's values",
  },
  "deploys.reason.canceled_along_with_newer": {
    message: "新しいデプロイとともにキャンセルされました",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_along_with_newer, w4/591)",
  },
  "deploys.reason.restart_service_gone": {
    message: "サービスがすでに存在しません",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason restart_service_gone, w4/591)",
  },
  "deploys.reason.restart_wait_timed_out": {
    message:
      "この再起動が待機していたデプロイが {wait} 以内に完了しませんでした",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason restart_wait_timed_out, w4/591); params are the sentence's values",
  },
  "deploys.reason.deploy_wait_timed_out": {
    message:
      "このデプロイが待っていたデプロイが {wait} 以内に完了しませんでした",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason deploy_wait_timed_out, w4/652): a Rollback or another deploy that waited for the deploy in flight; params are the sentence's values",
  },
  "deploys.reason.probe_other_waiting": {
    message:
      "コンテナーは実行中ですが、{kind} ヘルスチェックが成功していないため、ロールアウトは待機中です: 設定されたヘルスチェック。そのパスがサービス自身のポートで 2xx または 3xx のステータスを返すことを確認してください。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_waiting, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.probe_other_waiting_restarted": {
    message:
      "コンテナーは実行中ですが、{kind} ヘルスチェックが成功していないため、ロールアウトは待機中です: 設定されたヘルスチェック。ライブネスチェックによってコンテナーはすでに {restarts} 回再起動されており、単に遅いのではなくチェックが失敗しています。そのパスがサービス自身のポートで 2xx または 3xx のステータスを返すことを確認してください。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_waiting_restarted, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.probe_other_failed": {
    message:
      "コンテナーは実行中ですが、ロールアウトの制限時間内に {kind} ヘルスチェックが一度も成功しなかったため、デプロイに失敗しました: 設定されたヘルスチェック。そのパスがサービス自身のポートで 2xx または 3xx のステータスを返すことを確認してください。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_failed, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.probe_other_failed_restarted": {
    message:
      "コンテナーは実行中ですが、ロールアウトの制限時間内に {kind} ヘルスチェックが一度も成功しなかったため、デプロイに失敗しました: 設定されたヘルスチェック。ライブネスチェックによってコンテナーはすでに {restarts} 回再起動されており、単に遅いのではなくチェックが失敗しています。そのパスがサービス自身のポートで 2xx または 3xx のステータスを返すことを確認してください。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_failed_restarted, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.probe_open_port_waiting": {
    message:
      "コンテナーは実行中ですが、{kind} ヘルスチェックが成功していないため、ロールアウトは待機中です: インスタンスで開いている任意のポートへの TCP 接続。ヘルスチェックパスが設定されていないため、チェックは TCP 接続となり、プロセスが少なくとも 1 つのポートでリッスンしているかどうかだけを確認します。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_open_port_waiting, w8/384): the stall of Render's default check, a TCP connect to any open port, which a first web image release with no Health Check Path runs; params are the sentence's values",
  },
  "deploys.reason.probe_open_port_waiting_restarted": {
    message:
      "コンテナーは実行中ですが、{kind} ヘルスチェックが成功していないため、ロールアウトは待機中です: インスタンスで開いている任意のポートへの TCP 接続。ライブネスチェックによってコンテナーはすでに {restarts} 回再起動されており、単に遅いのではなくチェックが失敗しています。ヘルスチェックパスが設定されていないため、チェックは TCP 接続となり、プロセスが少なくとも 1 つのポートでリッスンしているかどうかだけを確認します。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_open_port_waiting_restarted, w8/384): the stall of Render's default check, a TCP connect to any open port, which a first web image release with no Health Check Path runs; params are the sentence's values",
  },
  "deploys.reason.probe_open_port_failed": {
    message:
      "コンテナーは実行中ですが、ロールアウトの制限時間内に {kind} ヘルスチェックが一度も成功しなかったため、デプロイに失敗しました: インスタンスで開いている任意のポートへの TCP 接続。ヘルスチェックパスが設定されていないため、チェックは TCP 接続となり、プロセスが少なくとも 1 つのポートでリッスンしているかどうかだけを確認します。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_open_port_failed, w8/384): the stall of Render's default check, a TCP connect to any open port, which a first web image release with no Health Check Path runs; params are the sentence's values",
  },
  "deploys.reason.probe_open_port_failed_restarted": {
    message:
      "コンテナーは実行中ですが、ロールアウトの制限時間内に {kind} ヘルスチェックが一度も成功しなかったため、デプロイに失敗しました: インスタンスで開いている任意のポートへの TCP 接続。ライブネスチェックによってコンテナーはすでに {restarts} 回再起動されており、単に遅いのではなくチェックが失敗しています。ヘルスチェックパスが設定されていないため、チェックは TCP 接続となり、プロセスが少なくとも 1 つのポートでリッスンしているかどうかだけを確認します。{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_open_port_failed_restarted, w8/384): the stall of Render's default check, a TCP connect to any open port, which a first web image release with no Health Check Path runs; params are the sentence's values",
  },
  "deploys.reason.hint_image_default_port": {
    message:
      " ポート {port} で TCP 接続を受け付けたものはありませんでした。イメージが TCP ポートを宣言していないため、これがデフォルトのポートです。サーバーは PORT ({port}) のポートで、ホスト 0.0.0.0 にバインドする必要があります。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_image_default_port, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_image_declared_ports": {
    message:
      " ポート {port} で TCP 接続を受け付けたものはありませんでした。これはイメージが宣言した TCP ポート ({ports}) から選ばれたプライマリポートです。サーバーをそのいずれかのポートで 0.0.0.0 にバインドするか、サービスのポートをサーバーがリッスンしているポートに設定してください。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_image_declared_ports, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_env_port_only": {
    message:
      " ポート {port} で TCP 接続を受け付けたものはありませんでした。これはサービスの環境変数 PORT で設定されており、イメージは他の TCP ポートを宣言していません。サーバーをポート {port} で 0.0.0.0 にバインドするか、サービスのポートをサーバーがリッスンしているポートに変更してください。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_env_port_only, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_env_port_closed": {
    message:
      " ポート {port} で TCP 接続を受け付けたものはありませんでした。これはサービスの環境変数 PORT で設定されています。サーバーをポート {port} でホスト 0.0.0.0 にバインドするか、環境変数 PORT をサーバーがリッスンしているポートに変更してください。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_env_port_closed, w8/416): a web service's PORT names the one port Render scans; en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_last_result": {
    message: " 最後の結果: {result}。",
    description:
      'What the failing health check last got back, which the operator appends to a health-check stall (bex-api DeployReason part hint_last_result, w6/m634); {result} is Render\'s English message ("HTTP health check failed with status code 503"), shown verbatim in every language as Render shows it; en starts with the space that joins it to the sentence before',
  },
  "deploys.reason.hint_configured_port_only": {
    message:
      " ポート {port} で TCP 接続を受け付けたものはありませんでした。これはサービスに設定されたポートで、イメージは他の TCP ポートを宣言していません。サーバーをポート {port} で 0.0.0.0 にバインドするか、サービスのポートをサーバーがリッスンしているポートに変更してください。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_configured_port_only, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_env_port_declared": {
    message:
      " ポート {port} で TCP 接続を受け付けたものはありませんでした。これはサービスの環境変数 PORT で設定されており、イメージは TCP ポート ({ports}) を宣言しています。環境変数 PORT をサーバーがリッスンしているポートに変更するか、削除してイメージのポートを使用してください。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_env_port_declared, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_configured_port_declared": {
    message:
      " ポート {port} で TCP 接続を受け付けたものはありませんでした。これはサービスに設定されたポートで、イメージは TCP ポート ({ports}) を宣言しています。設定でサービスのポートをサーバーがリッスンしているポートに変更するか、削除してイメージのポートを使用してください。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_configured_port_declared, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_background_worker": {
    message:
      " サービスがどのポートでもトラフィックを受信する必要がない場合は、代わりに Background Worker を作成してください。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_background_worker, w5/m356); en starts with the space that joins it to the sentence before",
  },
  "deploys.reason.hint_no_http_discovered": {
    message: " 起動時の検出で HTTP に応答したポートはありませんでした。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_http_discovered, w5/m356); en starts with the space that joins it to the sentence before",
  },
  "deploys.reason.hint_listening": {
    message: " プロセスはポート {port} でリッスンしています。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_listening_adopted": {
    message:
      " プロセスはポート {port} でリッスンしており、サービスはこのポートを使用するようになりました。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening_adopted, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_listening_ports": {
    message:
      " プロセスはポート {port} ではなく、ポート {ports} でリッスンしています。サービスのポート（または PORT 変数）を、HTTP を提供するポートに設定してください。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening_ports, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_listening_primary": {
    message:
      " プロセスはポート {port} でリッスンしており、サービスはこのポートをプライマリポートとして使用するようになりました。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening_primary, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_no_open_port": {
    message:
      " インスタンスで開いているポートが検出されませんでした。サーバーは PORT ({port}) のポートで、ホスト 0.0.0.0 にバインドする必要があります。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_port, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_no_open_port_before": {
    message:
      " インスタンスで開いているポートが検出されませんでした。サーバーは PORT ({port}) のポートで、ホスト 0.0.0.0 にバインドする必要があります。以前はポート {previous} でリッスンしていました。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_port_before, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_no_open_service_port": {
    message:
      " インスタンスで開いているポートが検出されませんでした。サーバーはサービスのポート ({port}) で、ホスト 0.0.0.0 にバインドする必要があります。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_service_port, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_no_open_port_localhost": {
    message:
      " 0.0.0.0 で開いているポートは検出されませんでしたが、localhost でポートが開いています ({ports})。サーバーは PORT ({port}) のポートで、ホスト 0.0.0.0 にバインドする必要があります。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_port_localhost, w8/388); en starts with the space that joins it to the sentence before; params are the sentence's values: ports is the comma-separated list of ports the process listens on only at localhost",
  },
  "deploys.reason.hint_no_open_http_port": {
    message:
      " 0.0.0.0 でポートが開いています ({ports}) が、どれも HTTP に応答しません。サーバーは PORT ({port}) のポートで HTTP を提供する必要があります。サービスが公開 HTTP トラフィックを受信する必要がない場合は、代わりに Private Service を作成してください。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_http_port, w8/388); en starts with the space that joins it to the sentence before; params are the sentence's values: ports is the comma-separated list of open ports that answer no HTTP",
  },
  "deploys.reason.hint_reserved_port": {
    message:
      " プロセスは予約ポート {reserved} で HTTP を提供しています。代わりに PORT ({port}) のポートで 0.0.0.0 にサーバーをバインドしてください。予約ポートは 18012、18013、19099 です。",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_reserved_port, w1/m659); en starts with the space that joins it to the sentence before; params are the sentence's values: reserved is the reserved port the process serves HTTP on, port the one in PORT",
  },
  "deploys.reason.hint_crash_port_bind": {
    message:
      " クラッシュの原因がポートのバインドの場合: プロセスは $PORT ({port}) でリッスンする必要があります。",
    description:
      "A startup crash's port advice, by its code (bex-api DeployReason part hint_crash_port_bind, w8/386); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_early_exit_port": {
    message:
      " サーバーは SIGTERM を受け取るまで $PORT ({port}) でリッスンし続ける必要があります。",
    description:
      "A startup crash's port advice, by its code (bex-api DeployReason part hint_early_exit_port, w8/386); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.queued_build_limit": {
    message:
      "{noun}では同時ビルドが {active}/{limit} 件実行中です。空きスロットを待機しています",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_build_limit, w5/m356): a full concurrent-build cap; {noun} is the cap's scope, worded by deploys.reasonParam.noun.*",
  },
  "deploys.reason.queued_cluster_full": {
    message:
      "クラスターのビルド容量が上限に達しています。空きスロットを待機しています",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_cluster_full, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_slot_taken": {
    message:
      "同時ディスパッチにより最後のビルドスロットが使用されました。空きスロットを待機しています",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_slot_taken, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_database": {
    message:
      "ビルド前に、リンクされたデータベース {database} が利用可能になるのを待機しています",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_database, w5/m356): what the build waited on; params are the sentence's values",
  },
  "deploys.reason.queued_datastore": {
    message: "ビルド前に、リンクされたデータストアの接続情報を待機しています",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_datastore, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_build_credential": {
    message:
      "レジストリがこのアプリのビルド認証情報を受け付けるのを待機しています",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_build_credential, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_pull_credential": {
    message:
      "レジストリがこのアプリのプル認証情報を受け付けるのを待機しています",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_pull_credential, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_capacity": {
    message: "ビルド容量を待機しています: {detail}",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity, w5/m356): waiting for build capacity; {detail} is the build plane's own wait, shown as-is",
  },
  "deploys.reason.queued_capacity_pod": {
    message:
      "ビルド容量を待機しています: ビルドジョブをディスパッチしました。ビルド Pod が作成されるのを待機しています",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity_pod, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_capacity_no_node": {
    message:
      "ビルド容量を待機しています: 空きのあるビルドノードがまだありません",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity_no_node, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_capacity_short": {
    message:
      "ビルド容量を待機しています: 空きのあるビルドノードがまだありません ({resources} の空きが不足しています)",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity_short, w5/m356): what the build waited on; params are the sentence's values",
  },
  "deploys.reasonParam.kind.startup": {
    message: "スタートアップ",
    description:
      "A deploy reason's {kind} value startup (w4/591): which health check a stalled rollout is waiting on",
  },
  "deploys.reasonParam.kind.readiness": {
    message: "レディネス",
    description:
      "A deploy reason's {kind} value readiness (w4/591): which health check a stalled rollout is waiting on",
  },
  "deploys.reasonParam.noun.workspace": {
    message: "ワークスペース",
    description:
      "A deploy reason's {noun} value workspace (w5/m356): the scope of a full concurrent-build cap",
  },
  "deploys.reasonWait.duration": {
    message: "{units}",
    description:
      "A deferred restart's wait, restart_wait_timed_out's {wait} (w5/m356): {wait} is bex-api's Go duration (3h0m0s), which en keeps so the sentence reads as the API's; {units} is the duration in words (deploys.reasonWait.hours and the rest), which other languages read",
  },
  "deploys.reasonWait.hours": {
    message: "{hours} 時間",
    description: "A deferred restart's wait in words (w5/m356): its hours",
  },
  "deploys.reasonWait.minutes": {
    message: "{minutes} 分",
    description: "A deferred restart's wait in words (w5/m356): its minutes",
  },
  "deploys.reasonWait.seconds": {
    message: "{seconds} 秒",
    description: "A deferred restart's wait in words (w5/m356): its seconds",
  },
  "deploys.factBuild": {
    message: "ビルド",
    description:
      "Deploy page: label of the fact naming the Build Source build ID (bld-…) this deploy ran",
  },
  "deploys.factBuildSource": {
    message: "ビルドソース",
    description:
      "Deploy page: label of the fact naming the linked Build Source (name and bds-… ID)",
  },
  "deploys.buildIdLabel": {
    message: "ビルド ID",
    description: "Deploy page: accessible field name for copying the Build ID",
  },
  "deploys.buildIdCopied": {
    message: "ビルド ID をコピーしました",
    description: "Deploy page: toast shown after copying the Build ID",
  },
  "deploys.buildIdCopyFailed": {
    message: "ビルド ID をコピーできませんでした",
    description: "Deploy page: toast shown when copying the Build ID fails",
  },
  "deploys.buildSourceIdLabel": {
    message: "ビルドソース ID",
    description:
      "Deploy page: accessible field name for copying the Build Source ID",
  },
  "deploys.buildSourceIdCopied": {
    message: "ビルドソース ID をコピーしました",
    description: "Deploy page: toast shown after copying the Build Source ID",
  },
  "deploys.buildSourceIdCopyFailed": {
    message: "ビルドソース ID をコピーできませんでした",
    description:
      "Deploy page: toast shown when copying the Build Source ID fails",
  },
  "deploys.noticesTitle": {
    message: "お知らせ",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): the section heading over the list, shown in small capitals",
  },
  "deploys.noticeTroubleshoot": {
    message:
      "デプロイのよくあるトラブルシューティング方法は{link}をご覧ください。",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): the line under a failed deploy's reason; {link} is the noticeTroubleshootLink link to bex's troubleshooting docs",
  },
  "deploys.noticeTroubleshootLink": {
    message: "ドキュメント",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): the link text inside noticeTroubleshoot",
  },
  "deploys.noticeBuildCacheCleared": {
    message: "ビルドキャッシュを消去しました",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): the deploy was opened with Clear build cache & deploy",
  },
  "deploys.noticeFreePlan": {
    message: "無料のコンピュートプランに変更しました",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): a settings deploy that moved the service from a paid plan to the free one",
  },
  "deploys.noticeMaintenance": {
    message:
      "bex はプラットフォームの信頼性を保つため、ダウンタイムのないメンテナンスを定期的に行っています。",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): a deploy the platform itself started (trigger deployed_by_render)",
  },
  "deploys.noticeRolledBackTo": {
    message: "{deploy} にロールバックしました",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): a rollback deploy; {deploy} is the target deploy's id, a link",
  },
  "deploys.noticeBuildGone": {
    message: "このビルドはもう存在しないため、ロールバックできません。",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): a past deploy whose build is no longer stored",
  },
  "deploys.noticeSyncedFrom": {
    message: "{blueprint} から同期しました",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): a deploy a Blueprint sync opened; {blueprint} is the Blueprint's id, a link",
  },
  "deploys.noticeInProgress": {
    message: "{deploy}が進行中です",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): a queued deploy; {deploy} is the noticeAnotherDeploy link to the deploy in progress",
  },
  "deploys.noticeAnotherDeploy": {
    message: "別のデプロイ",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): the link text inside noticeInProgress",
  },
  "deploys.noticeCancelPolicy": {
    message:
      "新しいデプロイの開始時に実行中のデプロイがキャンセルされないようにするには、{policy}を変更してください。",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): under a deploy a newer one canceled, when the workspace's Overlapping Deploy Policy is Override; {policy} is the noticePolicyLink link to Workspace Settings",
  },
  "deploys.noticeQueuePolicy": {
    message: "デプロイが待機しないようにするには、{policy}を変更してください。",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): under a queued deploy, when the workspace's Overlapping Deploy Policy is Wait; {policy} is the noticePolicyLink link to Workspace Settings",
  },
  "deploys.noticePolicyLink": {
    message: "重複デプロイポリシー",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): the link text inside noticeCancelPolicy and noticeQueuePolicy, the workspace's Overlapping Deploy Policy",
  },
  "deploys.noticeHealthWaiting": {
    message: "内部の{link}が成功のレスポンスコードを返すのを待っています",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): a deploy rolling out on a service with a Health Check Path; {link} is the noticeHealthLink link to the health check docs",
  },
  "deploys.noticeHealthWaitingAt": {
    message:
      "内部の{link}が成功のレスポンスコードを返すのを待っています: {address}",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): noticeHealthWaiting with the address checked; {address} is host:port/path in code type",
  },
  "deploys.noticeHealthTimedOut": {
    message:
      "内部の{link}が成功のレスポンスコードを返すのを待ちましたが、タイムアウトしました",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): a failed deploy that ran out of its rollout window, on a service with a Health Check Path; {link} as in noticeHealthWaiting",
  },
  "deploys.noticeHealthTimedOutAt": {
    message:
      "内部の{link}が成功のレスポンスコードを返すのを待ちましたが、タイムアウトしました: {address}",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): noticeHealthTimedOut with the address checked; {address} is host:port/path in code type",
  },
  "deploys.noticeHealthLink": {
    message: "ヘルスチェック",
    description:
      "Deploy page Notices (w1/m659, Render's DeployContainer): the link text inside the health check notices",
  },
  "deploys.reservedPortWarning": {
    message:
      "サービスがポート {port} で待ち受けている可能性があります。このポートは{link}の予約ポートです。予約ポート ({ports}) 以外のポートで待ち受けるようにサービスを設定してください。",
    description:
      "Deploy page banner (w1/m659, Render's free-plan warning): the instance serves HTTP on a reserved port; {port} is that port, {link} the reservedPortWarningLink link to bex's pricing, {ports} the reserved ports",
  },
  "deploys.reservedPortWarningLink": {
    message: "無料のコンピュートプラン",
    description: "The link text inside reservedPortWarning (w1/m659)",
  },
  "deploys.rollbackTitle": {
    message: "ロールバックの確認",
    description:
      "Rollback page (w1/m646): the block's title, Render's \"Confirm Rollback\"",
  },
  "deploys.rollbackLead": {
    message: "ロールバックすると、一部の設定変更が元に戻ります。",
    description:
      "Rollback page: the block's lead sentence, before the Learn more link",
  },
  "deploys.rollbackLearnMore": {
    message: "詳細を見る",
    description:
      "Rollback page: link to the rollbacks documentation after the lead sentence",
  },
  "deploys.rollbackLoading": {
    message: "設定の変更を読み込んでいます",
    description:
      "Rollback page: accessible name of the spinner while the comparison loads",
  },
  "deploys.rollbackUnavailable": {
    message:
      "現在公開中のデプロイの設定を読み込めませんでした。このデプロイにロールバックすると、予期しない設定変更が発生する可能性があります。",
    description:
      "Rollback page: Render's sentence when the comparison failed or either deploy's configuration is not recorded",
  },
  "deploys.rollbackNoChanges": {
    message: "このデプロイ以降、設定の変更はありません。",
    description:
      "Rollback page: nothing differs between the live deploy and this one",
  },
  "deploys.rollbackRowStartCommand": {
    message: "開始コマンド",
    description: "Rollback page: row label for a changed start command",
  },
  "deploys.rollbackRowPreDeployCommand": {
    message: "プレデプロイコマンド",
    description: "Rollback page: row label for a changed pre-deploy command",
  },
  "deploys.rollbackRowDockerCommand": {
    message: "Docker コマンド",
    description: "Rollback page: row label for a changed Docker command",
  },
  "deploys.rollbackRowHealthCheckPath": {
    message: "ヘルスチェックパス",
    description: "Rollback page: row label for a changed health check path",
  },
  "deploys.rollbackRowInstances": {
    message: "インスタンス",
    description: "Rollback page: row label for a changed instance count",
  },
  "deploys.rollbackRowEnvVars": {
    message: "環境変数",
    description:
      "Rollback page: row label for the changed environment variables",
  },
  "deploys.rollbackRowEnvGroup": {
    message: "環境グループ",
    description: "Rollback page: row label for environment group changes",
  },
  "deploys.rollbackRowSecretFiles": {
    message: "シークレットファイル",
    description: "Rollback page: row label for changed secret files",
  },
  "deploys.rollbackRowPlan": {
    message: "プラン",
    description: "Rollback page: row label for the plan warning",
  },
  "deploys.rollbackNone": {
    message: "なし",
    description:
      "Rollback page: a command or path that is not set, on either side of a change",
  },
  "deploys.rollbackChange": {
    message: "設定の変更",
    description: "Rollback page: accessible name of one before/after change",
  },
  "deploys.rollbackRemoved": {
    message: "削除",
    description:
      "Rollback page: accessible name of a change's minus line (the live value)",
  },
  "deploys.rollbackAdded": {
    message: "追加",
    description:
      "Rollback page: accessible name of a change's plus line (the value after the rollback)",
  },
  "deploys.rollbackEnvKey": {
    message: "キー",
    description:
      "Rollback page: environment variable table's key column header",
  },
  "deploys.rollbackEnvValue": {
    message: "値",
    description:
      "Rollback page: environment variable table's value column header; every value is masked",
  },
  "deploys.rollbackSecretFileAdded": {
    message:
      "シークレットファイル {name} はこのデプロイの後に追加されました。ロールバックではこのファイルは無視されます。",
    description:
      "Rollback page: a secret file that did not exist at this deploy; {name} is the file name",
  },
  "deploys.rollbackSecretFileDeleted": {
    message:
      "シークレットファイル {name} はこのデプロイの後に削除されました。ロールバックによりファイルが復元されます。",
    description:
      "Rollback page: a secret file deleted since this deploy; {name} is the file name",
  },
  "deploys.rollbackSecretFileChanged": {
    message:
      "シークレットファイル {name} はこのデプロイの後に変更されました。ロールバックにより元の内容に戻ります。",
    description:
      "Rollback page: a secret file whose contents changed since this deploy; {name} is the file name",
  },
  "deploys.rollbackGroupUnlink": {
    message:
      "このデプロイの後にサービスに追加された環境グループ {name} は、ロールバックによりリンクが解除されます。",
    description:
      "Rollback page: an environment group linked since this deploy; {name} is the group name",
  },
  "deploys.rollbackGroupLink": {
    message:
      "このデプロイの後にサービスから削除された環境グループ {name} は、ロールバックにより再びリンクされます。",
    description:
      "Rollback page: an environment group unlinked since this deploy; {name} is the group name",
  },
  "deploys.rollbackGroupDeleted_other": {
    message:
      "このデプロイにリンクされていた {n} 個の環境グループは、その後削除されています。復元することはできません。",
    description:
      "Rollback page: several groups this deploy linked were deleted; {n} is the count",
  },
  "deploys.rollbackGroupCurrentValues": {
    message:
      "ロールバックでは、このデプロイにリンクされている環境グループの現在のキーと値が使用されます。",
    description:
      "Rollback page: shown whenever this deploy linked any environment group",
  },
  "deploys.rollbackStaticGroupNotUse": {
    message:
      "このビルドの後にサイトに追加された環境グループ {name} は、ロールバックでは使用されません。",
    description:
      "Rollback page, static site: an environment group linked since this build, which the rollback's files never used; {name} is the group name",
  },
  "deploys.rollbackStaticGroupUse": {
    message:
      "このビルドの後にサイトから削除された環境グループ {name} は、ロールバックで使用されます。",
    description:
      "Rollback page, static site: an environment group unlinked since this build, whose values the rollback's files hold; {name} is the group name",
  },
  "deploys.rollbackStaticGroupDeleted_other": {
    message:
      "このビルドで使用された {n} 個の環境グループは、その後削除されています。ロールバックでは元の値が使用されます。",
    description:
      "Rollback page, static site: several groups this build used were deleted; {n} is the count",
  },
  "deploys.rollbackStaticGroupOriginalValues": {
    message:
      "ロールバックでは、このビルドで使用された環境グループの元の値が使用されます。",
    description:
      "Rollback page, static site: shown whenever this build used any environment group",
  },
  "deploys.rollbackPlanWarning": {
    message:
      "現在のプランは、このデプロイで当初使用されていたプランよりも CPU とメモリの割り当てが少なくなっています。ロールバックを成功させるには、別のプランに変更する必要がある場合があります。",
    description:
      "Rollback page: the saved plan is smaller than the one this deploy ran on",
  },
  "deploys.rollbackStaticNotice": {
    message: "{what}への変更はロールバックされません。",
    description:
      "Rollback page, static site: {what} is deploys.rollbackStaticNoticeWhat, in bold",
  },
  "deploys.rollbackStaticNoticeWhat": {
    message: "リダイレクト、リライト、ヘッダー",
    description: "Rollback page, static site: what a rollback keeps current",
  },
  "deploys.rollbackAutoDeployTitle": {
    message: "自動デプロイ",
    description:
      "Rollback page: title of the warning that the rollback turns auto-deploy off",
  },
  "deploys.rollbackAutoDeployBody": {
    message: "意図しないデプロイを防ぐため、このサービスでは無効になります。",
    description:
      "Rollback page: the auto-deploy warning's text, after its title",
  },
  "deploys.rollbackConfirm": {
    message: "このデプロイにロールバック",
    description: "Rollback page: the button that starts the rollback",
  },
  "deploys.rollbackCancel": {
    message: "キャンセル",
    description:
      "Rollback page: the button that goes back to the deploy without rolling back",
  },
  "deploys.rollbackErrorTitle": {
    message: "エラー:",
    description: "Rollback page: label before the reason a rollback failed",
  },
};

export default jaDeploys;
