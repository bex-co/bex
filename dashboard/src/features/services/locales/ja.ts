import type { TranslationEntry } from "@/i18n";

const jaServices: Record<string, TranslationEntry> = {
  "services.protectedConfirmationTitle": {
    message: "保護された環境の確認",
    description: "Title of a protected-service destructive-action retry dialog",
  },
  "services.protectedConfirmationBody": {
    message:
      "「{name}」は保護された環境に属しています。続行するには、bex-api が返した sudo コマンドをそのまま入力してください。",
    description: "Body of a protected-service destructive-action retry dialog",
  },
  "services.protectedConfirmationPrompt": {
    message: "確認のため、下に {phrase} と入力してください。",
    description:
      "Body prompt naming the exact protected-action confirmation phrase (rendered bold by SudoCommandField)",
  },
  "services.connect": {
    message: "接続",
    description: "Open the service connection menu",
  },
  "services.connectSSH": {
    message: "SSH",
    description: "SSH section label in the service connection menu",
  },
  "services.openBrowserTerminal": {
    message: "ブラウザーターミナルを開く",
    description:
      "RequiresSshKey gate — zero-setup second door to the Web Shell (no SSH key)",
  },
  "services.connectInternal": {
    message: "内部",
    description:
      "Internal-address section label in the service connection menu (Render's Connect → Internal tab)",
  },
  "services.connectInternalFreeNote": {
    message:
      "無料の Web サービスはプライベートネットワークのトラフィックを受信できません。内部アドレスを使うにはアップグレードしてください。",
    description:
      "Connect → Internal note for a free public web service: Render's Free plan does not support receiving private network traffic, so a sibling's internal call never reaches it (w8/m130)",
  },
  "services.headerServiceAddress": {
    message: "サービスアドレス",
    description:
      "Label before a private service's internal address in the detail header (Render's private-service wording)",
  },
  "services.openPorts": {
    message: "公開ポート",
    description:
      "Label before the TCP ports a private service's hostname serves, beside its Service Address and in Connect → Internal (Render's serviceDetails.openPorts, w6/m148)",
  },
  "services.openPortsPrimary": {
    message: "プライマリ",
    description:
      "Marker on the open port that is the service's primary port — the <slug>:<port> suffix of its Service Address (w6/m148)",
  },
  "services.internalCopy": {
    message: "内部アドレスをコピー",
    description: "Copy service internal address button",
  },
  "services.internalCopied": {
    message: "内部アドレスをコピーしました",
    description: "Successful internal address copy",
  },
  "services.internalCopyError": {
    message: "内部アドレスをコピーできませんでした",
    description: "Failed internal address copy",
  },
  "services.sshCopy": {
    message: "SSH コマンドをコピー",
    description: "Copy service SSH command button",
  },
  "services.sshCopied": {
    message: "SSH コマンドをコピーしました",
    description: "Successful SSH command copy",
  },
  "services.sshCopyError": {
    message: "SSH コマンドをコピーできませんでした",
    description: "Failed SSH command copy",
  },
  "services.sshUnavailable": {
    message: "SSH は利用できません",
    description: "Service header state when no SSH address is advertised",
  },
  "services.sshUnavailableHint": {
    message:
      "SSH を使うには、稼働中の有料の Web、プライベート、またはバックグラウンドサービスと、有効なゲートウェイが必要です。",
    description: "Explanation for a service without an SSH address",
  },
  "services.sshUnavailableSuspended": {
    message:
      "このサービスは一時停止中です。SSH で接続するには再開してください。",
    description: "Why SSH is unavailable on a suspended service (w4/143)",
  },
  "services.sshUnavailableSuspendedFree": {
    message:
      "このサービスは一時停止中で、SSH には有料のコンピュートプランが必要です。再開するとトラフィックは復旧しますが、Free サービスにはシェルアクセスが含まれません。",
    description:
      "Why SSH is unavailable on a suspended Free service: both blockers, without implying a resume grants SSH (w4/m183)",
  },
  "services.sshUnavailableFree": {
    message:
      "SSH には有料のコンピュートプランが必要です。Free サービスにはシェルアクセスが含まれません。",
    description: "Why SSH is unavailable on a Free-plan service (w4/143)",
  },
  "services.sshRemedyResume": {
    message: "サービスを再開",
    description:
      "Link from an SSH-unavailable state to the service's suspend/resume setting (w4/143)",
  },
  "services.sshRemedyChangePlan": {
    message: "コンピュートプランを変更",
    description:
      "Link from an SSH-unavailable state on a Free service to its compute-plan page (w4/143)",
  },
  "services.shellDescription": {
    message: "稼働中のサービスインスタンスのシェルを、ターミナルから開きます。",
    description: "Running-instance SSH connection page description",
  },
  "services.shellConnectionTitle": {
    message: "SSH で接続",
    description: "SSH connection card title",
  },
  "services.shellConnectionDescription": {
    message:
      "bex は既存の準備完了インスタンスに接続します。シェル専用のインスタンスは作成しません。",
    description: "SSH running-instance behavior explanation",
  },
  "services.shellCommand": {
    message: "SSH コマンド",
    description: "Label for a copy-ready service SSH command",
  },
  "services.shellManageKeys": {
    message: "SSH 公開鍵を管理",
    description: "Link from a service shell page to account SSH key settings",
  },
  "services.shellSessionLifecycle": {
    message:
      "このコマンドは準備完了のインスタンスを選択します。サービスを再起動、再デプロイ、または一時停止すると、セッションは終了します。",
    description: "SSH session selection and lifecycle guidance",
  },
  "services.shellUnavailableTitle": {
    message: "シェルアクセスは利用できません",
    description: "Unavailable SSH connection card title",
  },
  "services.shellUnavailableBody": {
    message:
      "シェルアクセスには、稼働中の有料の Web、プライベート、またはバックグラウンドサービスと、有効な SSH ゲートウェイが必要です。",
    description: "Unavailable SSH connection card explanation",
  },
  "services.shellWebTitle": {
    message: "Web シェル",
    description: "In-browser terminal card title",
  },
  "services.shellWebDescription": {
    message: "稼働中のインスタンス内でコマンドをブラウザーから実行します。",
    description: "In-browser terminal card description",
  },
  "services.shellConnecting": {
    message: "稼働中のインスタンスに接続しています…",
    description: "Web shell status while the terminal is connecting",
  },
  "services.shellConnected": {
    message: "接続済み",
    description: "Web shell status when the terminal stream is live",
  },
  "services.shellClosed": {
    message: "セッションが終了しました",
    description: "Web shell status when the terminal stream has ended",
  },
  "services.shellErrorStatus": {
    message: "接続エラー",
    description:
      "Web shell status when the session failed to connect or errored",
  },
  "services.shellEnded": {
    message: "セッションが終了しました",
    description:
      "Web shell status when the server ended a live session for a stated reason (access, suspension, instance stopped, time limit)",
  },
  "services.shellEndRevoked": {
    message: "このサービスへのアクセス権がなくなりました。",
    description:
      "Web shell end reason: the caller lost access to the service mid-session (deliberately unspecific)",
  },
  "services.shellEndSuspended": {
    message:
      "サービスが一時停止されたため、このセッションは終了しました。再開すると新しいセッションを開けます。",
    description: "Web shell end reason: the service was suspended",
  },
  "services.shellEndIneligible": {
    message:
      "このサービスはシェルアクセスの対象外になった（プランまたはタイプが変更された）ため、このセッションは終了しました。",
    description:
      "Web shell end reason: the service moved to the Free plan or a type without instances",
  },
  "services.shellEndInstanceGone": {
    message:
      "このセッションが動作していたインスタンスが停止しました。再接続すると、稼働中のインスタンスでシェルを開けます。",
    description:
      "Web shell end reason: the attached instance stopped (restart, or the old instance leaving after a deploy went live)",
  },
  "services.shellEndTimeout": {
    message:
      "セッションが時間制限に達しました。再接続して作業を続けてください。",
    description: "Web shell end reason: the session hit its maximum duration",
  },
  "services.shellEndUnverified": {
    message:
      "アクセス権を再確認できなかったため、セッションを終了しました。再接続してもう一度お試しください。",
    description:
      "Web shell end reason: the periodic access check could not complete",
  },
  "services.shellReconnect": {
    message: "再接続",
    description: "Button that re-opens a closed web shell session",
  },
  "services.shellConnect": {
    message: "シェルを開始",
    description: "Button that opens a web shell session",
  },
  "services.shellErrorGeneric": {
    message: "シェルセッションを開始できませんでした。",
    description: "Web shell generic connection error",
  },
  "services.shellErrorUnavailable": {
    message:
      "このプラットフォームではブラウザー内シェルが有効になっていません。代わりに下の SSH コマンドを使用してください。",
    description:
      "Web shell error when the browser transport is unconfigured (503)",
  },
  "services.shellInstanceLabel": {
    message: "インスタンス",
    description: "Label for the web shell instance picker",
  },
  "services.shellInstanceAny": {
    message: "準備完了のインスタンスのいずれか",
    description:
      "Web shell instance picker option that selects a random ready replica",
  },
  "services.shellInstanceSelect": {
    message: "インスタンスを選択",
    description: "Placeholder for the web shell instance picker",
  },
  "services.shellSshFallback": {
    message:
      "お使いのターミナルを使いたい場合は、下の SSH コマンドを使用してください。",
    description: "Hint pointing web shell users to the copy-ready SSH command",
  },
  "services.actions": {
    message: "操作",
    description: "Accessible heading for service configuration row actions",
  },
  "services.statTotal": {
    message: "サービス合計",
    description: "Services page stat card label",
  },
  "services.statRunning": {
    message: "稼働中",
    description: "Services page stat card label",
  },
  "services.statSuspended": {
    message: "一時停止中",
    description: "Services page stat card label",
  },
  "services.cardTitle": {
    message: "サービス",
    description:
      "Services table card title, also used as the metrics page back-link",
  },
  "services.colName": {
    message: "名前",
    description: "Services table column header",
  },
  "services.colStatus": {
    message: "ステータス",
    description: "Services table column header",
  },
  "services.colUrl": {
    message: "URL",
    description: "Services table column header",
  },
  "services.colBlueprintName": {
    message: "Blueprint 名",
    description: "Immutable service name used to match Blueprint resources.",
  },
  "services.blueprintNameHelp": {
    message:
      "Blueprint マニフェストの services[].name には、この変更できない名前を使用してください。表示名や公開ホスト名は異なる場合があります。",
    description: "Immutable service name used to match Blueprint resources.",
  },
  "services.colSlug": {
    message: "スラッグ",
    description:
      "Service detail header fact (globally-unique platform-host segment, Render's slug field)",
  },
  "services.instancesSuspended": {
    message: "0（再開すると {n}）",
    description:
      "The header's Instances fact for a suspended service, which runs none; n is its configured count (w4/547)",
  },
  "services.instancesSleeping": {
    message: "0（復帰すると {n}）",
    description:
      "The header's Instances fact for a sleeping Free service, which runs none; n is its configured count (w4/547)",
  },
  "services.colInstances": {
    message: "インスタンス",
    description: "Services table column header (replica count — bex-native)",
  },
  "services.colRevision": {
    message: "リビジョン",
    description: "Services table column header (active revision — bex-native)",
  },
  "services.colCreated": {
    message: "作成日時",
    description: "Services table column header (relative age from createdAt)",
  },
  "services.colActions": {
    message: "操作",
    description: "Services table actions column header (screen-reader only)",
  },
  "services.statusRunning": {
    message: "稼働中",
    description: "Services table status badge",
  },
  "services.statusSuspended": {
    message: "一時停止中",
    description: "Services table status badge",
  },
  "services.statusSleeping": {
    message: "スリープ中",
    description:
      "Services status badge: a free-tier App auto-hibernated after idle (bex extension)",
  },
  "services.undeployedChangesSuspendedHint": {
    message:
      "保存した変更はまだ反映されていません。サービスを再開してからデプロイすると反映されます。",
    description:
      "Notice when a suspended service has saved settings that differ from the release it serves: no deploy can run until it resumes",
  },
  "services.undeployedChangesHint": {
    message:
      "保存した変更はまだ反映されていません。標準のデプロイを実行すると反映されます。",
    description:
      "Notice when saved settings differ from the running release, including Save only, cancellation, rollback or restart",
  },
  "services.undeployedChangesApplyingHint": {
    message: "保存した変更は、現在のデプロイが完了すると反映されます。",
    description:
      "Notice while an in-flight deploy already carries the saved settings, so no further deploy is needed (w4/m184)",
  },
  "services.undeployedChangesOverrideHint": {
    message:
      "このデプロイでは、このデプロイ用に選択した {running} を実行します。保存されているイメージは {saved} で、標準のデプロイを実行すると元に戻ります。",
    description:
      "Notice when a per-deploy image override (imageUrl/imgURL) runs a different image than the saved one: nothing was saved, and a standard deploy reverts to the saved image (w4/414)",
  },
  "services.statusSleepingHint": {
    message:
      "リソース節約のためスリープ中です。公開 URL への次のリクエストで起動します。",
    description:
      "Hint next to the Sleeping badge explaining free-tier auto-sleep + wake-on-request",
  },
  "services.statusPending": {
    message: "保留中",
    description: "Services table status badge",
  },
  "services.statusBuilding": {
    message: "ビルド中",
    description: "Services table status badge",
  },
  "services.statusDeploying": {
    message: "デプロイ中",
    description: "Services table status badge",
  },
  "services.statusCanceled": {
    message: "キャンセル済み",
    description:
      "Services table status badge for a service whose first-ever deploy the user canceled",
  },
  "services.statusDeleting": {
    message: "削除中",
    description:
      "Services status badge for a service whose deletion is finalizing",
  },
  "services.statusFailed": {
    message: "失敗",
    description: "Services table status badge",
  },
  "services.statusUnknown": {
    message: "不明",
    description: "Services table status badge for an unrecognized phase",
  },
  "services.actionsMenu": {
    message: "{name} の操作",
    description: "Accessible label for the per-row actions trigger",
  },
  "services.actionSuspend": {
    message: "一時停止",
    description: "Row action: park the service",
  },
  "services.actionResume": {
    message: "再開",
    description: "Row action: bring a suspended service back",
  },
  "services.actionRestart": {
    message: "再起動",
    description:
      "Row action: restart the service on the commit or image it is running",
  },
  "services.confirmSuspendTitle": {
    message: "{name} を一時停止しますか？",
    description: "Suspend confirmation dialog title",
  },
  "services.confirmSuspendBody": {
    message:
      "サービスはゼロにスケールダウンし、トラフィックの処理を停止します。URL と証明書は保持され、いつでも再開できます。",
    description:
      "Suspend confirmation dialog body, for the types served at a public host",
  },
  "services.confirmSuspendBodyNoUrl": {
    message:
      "サービスはゼロにスケールダウンし、実行を停止します。いつでも再開できます。",
    description:
      "Suspend confirmation dialog body, for private services and background workers — neither has a public URL or certificates to keep",
  },
  "services.confirmSuspendBodyStatic": {
    message:
      "一時停止中、サイトは配信を停止し、503 を返します。URL と証明書は保持され、いつでも再開できます。",
    description:
      "Suspend confirmation dialog body for a static site, which has no instances to scale to zero (w4/380)",
  },
  "services.suspendCronDescription": {
    message:
      "この Cron Job を一時停止すると、今後のスケジュール実行が停止します。実行中のものは継続します。停止するには、「実行」ページで「キャンセル」を選択してください。",
    description:
      "Cron suspension explanation in Settings and suspend confirmations; suspension does not cancel an active run",
  },
  "services.confirmRestartTitle": {
    message: "{name} を再起動しますか？",
    description: "Restart confirmation dialog title",
  },
  "services.confirmRestartBody": {
    message:
      "{name} は、現在実行中のコミットまたはイメージで再起動します。その後にプッシュされたコミットはデプロイされません。新しいインスタンスは、正常になった時点で古いものと入れ替わります。",
    description:
      "Restart confirmation dialog body (service header and services list). A restart keeps the running release, as on Render; it never picks up newer commits",
  },
  "services.confirmRestartBodyStatic": {
    message:
      "{name} は、現在配信中のコミットから再公開されます。その後にプッシュされたコミットはデプロイされません。",
    description:
      "Restart confirmation body for a static site, which has no instances to replace (w4/m141)",
  },
  "services.confirmCancel": {
    message: "キャンセル",
    description: "Confirmation dialog cancel button",
  },
  "services.toastSuspendSuccess": {
    message: "{name} を一時停止しています…",
    description: "Toast shown after a suspend request is accepted",
  },
  "services.toastResumeSuccess": {
    message: "{name} を再開しています…",
    description: "Toast shown after a resume request is accepted",
  },
  "services.toastRestartSuccess": {
    message: "{name} を再起動しています…",
    description: "Toast shown after a restart request is accepted",
  },
  "services.toastError": {
    message: "{name} を更新できませんでした。もう一度お試しください。",
    description: "Toast shown when a lifecycle action fails",
  },
  "services.errorTitle": {
    message: "サービスを読み込めませんでした",
    description: "Services list error card title",
  },
  "services.errorBody": {
    message:
      "bex-api へのリクエストが失敗しました。接続を確認して、もう一度お試しください。",
    description: "Services list error card body",
  },
  "services.detailErrorTitle": {
    message: "サービスを読み込めませんでした",
    description: "Service detail query error card title",
  },
  "services.detailErrorBody": {
    message:
      "bex-api へのリクエストが失敗しました。サービスは存在している可能性があります。",
    description: "Service detail query error card body",
  },
  "services.emptyTitle": {
    message: "サービスはまだありません",
    description: "Services list empty state title",
  },
  "services.emptyBody": {
    message: "最初の App をデプロイすると、ここに表示されます。",
    description: "Services list empty state body",
  },
  "services.headerServiceId": {
    message: "サービス ID:",
    description: "Service-detail header metadata label for the service id",
  },
  "services.headerServicePhase": {
    message: "サービス",
    description:
      "Service-detail header label distinguishing operator/App phase from deploy status",
  },
  "services.headerLatestDeploy": {
    message: "最新のデプロイ",
    description: "Service-detail header label for the newest deploy status",
  },
  "services.headerRuntime": {
    message: "ランタイム",
    description: "Service-detail header label for the selected runtime",
  },
  "services.headerMoreFacts": {
    message: "詳細情報",
    description:
      "Service-detail header toggle, phones only, that shows or hides the slug, instances, revision and age line",
  },
  "services.headerLastRun": {
    message: "最後に成功した実行",
    description:
      "Service-detail header: a cron job's second fact line, before the last successful run's date (Render's \"Last successful run <date>\", w3/m555)",
  },
  "services.headerRuns": {
    message: "実行",
    description:
      "Service-detail header: the word before a cron job's command on its first fact line (Render's \"Runs <command> <schedule>\", w3/m555)",
  },
  "services.headerRunsDockerCmd": {
    message: "{dockerfile} の {cmd}",
    description:
      "Service-detail header: what a Docker or image cron job with no command override runs; {cmd} is the code word CMD and {dockerfile} the Dockerfile's path, both shown as code (Render's \"CMD in Dockerfile\", w3/m555)",
  },
  "services.headerNoSuccessfulRuns": {
    message: "成功した実行はまだありません。",
    description:
      "Service-detail header: a cron job's second fact line before its first successful run (Render's text, w3/m555)",
  },
  "services.suspendedNoticeManual": {
    message: "手動で一時停止中",
    description:
      "Title of the notice under a suspended service's header when its owner suspended it (Render's \"Manually suspended\", w3/m555, w3/107)",
  },
  "services.suspendedNoticeTitle": {
    message: "一時停止中",
    description:
      "Title of the notice under a suspended service's header when no suspender is known (Render's fallback \"Suspended\", w3/m555, w3/107)",
  },
  "services.suspendedNoticeByPlatform": {
    message: "bex により一時停止中",
    description:
      "Title of the notice under a suspended service's header when the platform suspended it for spent Free instance hours or bandwidth (the billing suspender; Render's \"Suspended by Render\", w3/107)",
  },
  "services.suspendedNoticeCrashLoop": {
    message: "繰り返しクラッシュしたため、このサービスは一時停止されました",
    description:
      "Title of the notice under a suspended service's header when the platform suspended it for a crash loop (stuck_crashlooping; Render's title, w3/107)",
  },
  "services.suspendedNoticeBody": {
    message: "一時停止中のサービスには料金がかかりません。",
    description:
      "Body of the notice under a suspended service's header (Render's text, w3/m555, w3/107)",
  },
  "services.suspendedNoticeResume": {
    message: "サービスを再開",
    description:
      "The notice's inline action that resumes a suspended service (Render's \"Resume service\", w3/m555, w3/107)",
  },
  "services.headerCopyServiceId": {
    message: "サービス ID をコピー",
    description: "Accessible label for the header's service-id copy button",
  },
  "services.headerCopyUrl": {
    message: "サービス URL をコピー",
    description: "Accessible label for the header's live-URL copy button",
  },
  "services.headerCopied": {
    message: "クリップボードにコピーしました",
    description: "Toast after copying a value from the service-detail header",
  },
  "services.headerCopyError": {
    message: "クリップボードにコピーできませんでした",
    description: "Toast when a service-detail header copy fails",
  },
  "services.navLogs": {
    message: "ログ",
    description: "Service-detail nav item (logs tab)",
  },
  "services.navMetrics": {
    message: "メトリクス",
    description: "Service-detail nav item (metrics tab)",
  },
  "services.navRedirects": {
    message: "リダイレクト/リライト",
    description:
      "Service-detail nav item (static-site route rules page, Render's label)",
  },
  "services.navHeaders": {
    message: "ヘッダー",
    description:
      "Service-detail nav item (static-site response headers page, Render's label)",
  },
  "services.scalingTitle": {
    message: "オートスケーリング",
    description: "Scaling tab section heading",
  },
  "services.autoscaleMinInstances": {
    message: "最小インスタンス数",
    description:
      "Accessible name of the minimum-instances number input beside the autoscaling range slider",
  },
  "services.autoscaleMaxInstances": {
    message: "最大インスタンス数",
    description:
      "Accessible name of the maximum-instances number input beside the autoscaling range slider",
  },
  "services.scalingEnabled": {
    message: "オートスケーリングを有効にする",
    description: "Autoscaling enable/disable toggle label",
  },
  "services.scalingMinInstances": {
    message: "最小インスタンス数",
    description: "Autoscaling min instances input label",
  },
  "services.scalingMaxInstances": {
    message: "最大インスタンス数",
    description: "Autoscaling max instances input label",
  },
  "services.scalingTargetCPU": {
    message: "目標 CPU %",
    description: "Autoscaling target CPU utilization input label",
  },
  "services.scalingTargetMemory": {
    message: "目標メモリ %",
    description: "Autoscaling target memory utilization input label",
  },
  "services.scalingValidationMinMax": {
    message: "最小インスタンス数は最大インスタンス数以下にしてください。",
    description: "Autoscaling form validation when min exceeds max",
  },
  "services.scalingValidationTargets": {
    message:
      "使用率の目標（CPU またはメモリ）を少なくとも 1 つ有効にしてください。",
    description:
      "Autoscaling form validation when both CPU and memory targets are off",
  },
  "services.scalingSave": {
    message: "保存",
    description: "Autoscaling form save button",
  },
  "services.scalingSaved": {
    message: "オートスケーリングの設定を保存しました。",
    description: "Autoscaling save success toast",
  },
  "services.scalingDisabled": {
    message: "オートスケーリングを無効にしました。",
    description: "Autoscaling disable success toast",
  },
  "services.scalingError": {
    message: "オートスケーリングの設定を更新できませんでした。",
    description: "Autoscaling save error toast",
  },
  "services.scalingDescription": {
    message:
      "CPU とメモリの使用率に基づいて、このサービスを自動的にスケールアップまたはスケールダウンします。",
    description: "Autoscaling card description",
  },
  "services.scalingOn": {
    message: "オートスケーリング オン",
    description: "Autoscaling main toggle label when enabled",
  },
  "services.scalingPaidOnly": {
    message:
      "オートスケーリングは有料プランで利用できます。有効にするにはアップグレードしてください。",
    description: "Autoscaling card explanation for a free service",
  },
  "services.scalingPaidOnlyInactive": {
    message:
      "オートスケーリングの設定は保存されていますが、Free プランでは無効です。このサービスが有料プランに移行すると有効になります。保存した設定を削除するには、オートスケーリングをオフにしてください。",
    description:
      "Autoscaling card explanation for a free service carrying a stored autoscaling config (w4/365)",
  },
  "services.scalingProPlanOnly": {
    message:
      "オートスケーリングは Pro 以上のワークスペースプランで利用できます。このワークスペースは Hobby です。",
    description:
      "Autoscaling card notice on a Hobby workspace, followed by an Open Billing link (w7/m429)",
  },
  "services.scalingProPlanOnlyRunning": {
    message:
      "オートスケーリングは Pro 以上のワークスペースプランで利用できます。このワークスペースは Hobby のため、下のポリシーは動作し続け、オフにできますが、変更や再度オンにするには Pro が必要です。",
    description:
      "Autoscaling card notice on a Hobby workspace whose service already autoscales, followed by an Open Billing link (w7/m429)",
  },
  "services.scalingMetricsUnavailableTitle": {
    message:
      "オートスケーリングは一時停止中です: 使用率メトリクスを取得できません",
    description:
      "Autoscaling card notice title while the operator cannot read the service's CPU and memory utilization (w4/m364)",
  },
  "services.scalingMetricsUnavailableBody": {
    message:
      "CPU とメモリの使用率を再び読み取れるようになるまで、サービスは現在のインスタンス数を維持します。オートスケーリングの設定は保存されています。",
    description:
      "Autoscaling card notice body while utilization metrics are unavailable (w4/m364)",
  },
  "services.scalingOff": {
    message: "オートスケーリング オフ",
    description: "Autoscaling main toggle label when disabled",
  },
  "services.scalingInstancesTitle": {
    message: "インスタンス数",
    description: "Autoscaling instances range-slider section heading",
  },
  "services.scalingInstancesHint": {
    message:
      "bex は、指定した範囲内でこのサービスのインスタンス数を調整します。",
    description: "Autoscaling instances range-slider section description",
  },
  "services.scalingCPUTitle": {
    message: "目標 CPU 使用率",
    description: "Autoscaling CPU metric section heading",
  },
  "services.scalingCPUHint": {
    message:
      "平均 CPU 使用率がこの値を大きく上回る、または下回る場合、bex はそれに応じてインスタンスを追加または削除します。",
    description: "Autoscaling CPU metric section description",
  },
  "services.scalingMemoryTitle": {
    message: "目標メモリ使用率",
    description: "Autoscaling memory metric section heading",
  },
  "services.scalingMemoryHint": {
    message:
      "平均メモリ使用率がこの値を大きく上回る、または下回る場合、bex はそれに応じてインスタンスを追加または削除します。",
    description: "Autoscaling memory metric section description",
  },
  "services.scalingCancel": {
    message: "キャンセル",
    description: "Autoscaling form cancel button",
  },
  "services.scalingSaveChanges": {
    message: "変更を保存",
    description: "Autoscaling form save-changes button",
  },
  "services.scalingDisableConfirmTitle": {
    message: "オートスケーリングを無効にしますか？",
    description: "Autoscaling disable confirmation dialog title (w7/m43)",
  },
  "services.scalingDisableConfirmBody": {
    message:
      "サービスは、手動スケーリングで指定した固定数のインスタンスで実行されます。",
    description: "Autoscaling disable confirmation dialog body",
  },
  "services.scalingDisableConfirmAction": {
    message: "無効にする",
    description: "Autoscaling disable confirmation dialog confirm button",
  },
  "services.scalingManualTitle": {
    message: "手動スケーリング",
    description: "Scaling page: manual instance-count card title (w7/m43)",
  },
  "services.scalingManualDescription": {
    message:
      "自動的に負荷分散される複数のインスタンスを実行します。すべてのインスタンスは同じコンピュートプランを使用し、それに応じて課金されます。",
    description: "Scaling page: manual instance-count card description",
  },
  "services.scalingManualDescriptionFree": {
    message:
      "Free サービスは 1 つのインスタンスで実行されます。負荷分散された複数のインスタンスを実行するには、有料プランにアップグレードしてください。",
    description:
      "Scaling page: manual instance-count card description on the free plan",
  },
  "services.scalingManualInstances": {
    message: "インスタンス",
    description: "Scaling page: manual instance-count slider label",
  },
  "services.scalingMetricsTitle": {
    message: "最近のメトリクス",
    description: "Scaling page: recent-metrics section title (w7/m43)",
  },
  "services.scalingMetricsNote": {
    message: "過去 48 時間のメトリクスを表示しています。",
    description: "Scaling page: recent-metrics window note",
  },
  "services.scalingMetricsViewAll": {
    message: "すべてのメトリクスを表示",
    description: "Scaling page: link to the full Metrics tab",
  },
  "services.scalingMetricsMemory": {
    message: "平均メモリ使用率",
    description: "Scaling page: averaged memory utilization chart title",
  },
  "services.scalingMetricsCPU": {
    message: "平均 CPU 使用率",
    description: "Scaling page: averaged CPU utilization chart title",
  },
  "services.scalingMetricsAcross": {
    message: "全インスタンス",
    description: "Scaling page: recent-metrics chart subtitle",
  },
  "services.scalingMetricsEmpty": {
    message: "過去 48 時間のデータはありません",
    description: "Scaling page: recent-metrics empty state",
  },
  "services.navEnvironment": {
    message: "環境",
    description: "Service-detail nav item (environment variables tab)",
  },
  "services.envTitle": {
    message: "環境変数",
    description: "Environment tab card title",
  },
  "services.envDescription": {
    message:
      "環境ごとの設定とシークレットを設定し、コードからその値を読み取ります。",
    description: "Environment tab card description",
  },
  "services.envColKey": {
    message: "キー",
    description: "Environment table column header (variable name)",
  },
  "services.envColValue": {
    message: "値",
    description: "Environment table column header (variable value)",
  },
  "services.envManifestManaged": {
    message: "Blueprint で管理",
    description:
      "Badge on an environment row whose value the service's render.yaml manifest owns",
  },
  "services.envManifestManagedBody": {
    message:
      "この変数はサービスの render.yaml マニフェストで宣言されており、値はマニフェストが管理します。変更するにはマニフェストを編集して Blueprint を同期するか、`sync: false` で宣言してダッシュボードで管理するようにしてください。",
    description:
      "Explanation under a manifest-managed environment row: why it cannot be edited here and what to do instead",
  },
  "services.envShowSecret": {
    message: "値を表示",
    description: "Environment row button to reveal a masked value",
  },
  "services.envHideSecret": {
    message: "値を非表示",
    description: "Environment row button to re-mask a revealed value",
  },
  "services.envRevealError": {
    message: "値を読み込めませんでした。",
    description: "Environment row inline error when a value reveal fails",
  },
  "services.envEmptyTitle": {
    message: "環境変数はありません",
    description: "Environment tab empty-state title",
  },
  "services.envEmptyBody": {
    message: "変数を追加して、このサービスを設定します。",
    description: "Environment tab empty-state body",
  },
  "services.envUnavailableTitle": {
    message: "環境変数は利用できません",
    description:
      "Environment tab state when the secret store is unconfigured (503)",
  },
  "services.envUnavailableBody": {
    message: "このデプロイ環境ではシークレットストアが設定されていません。",
    description: "Environment tab unavailable-state body",
  },
  "services.envForbiddenTitle": {
    message: "権限がありません",
    description: "Environment tab state when the caller lacks permission (403)",
  },
  "services.envForbiddenBody": {
    message: "このサービスの環境変数を表示する権限がありません。",
    description: "Environment tab forbidden-state body",
  },
  "services.envErrorTitle": {
    message: "環境変数を読み込めませんでした",
    description: "Environment tab generic error title",
  },
  "services.envErrorBody": {
    message: "問題が発生しました。もう一度お試しください。",
    description: "Environment tab generic error body",
  },
  "services.envAdd": {
    message: "変数を追加",
    description: "Environment tab button to open the add-variable form",
  },
  "services.envExport": {
    message: "エクスポート",
    description:
      "Environment tab button that downloads all service variables as dotenv",
  },
  "services.envExportSuccess": {
    message: "環境変数をエクスポートしました",
    description:
      "Toast after every current environment value was safely exported",
  },
  "services.envExportError": {
    message:
      "環境変数をすべてエクスポートできませんでした。ファイルはダウンロードされていません。",
    description:
      "Fail-closed toast when any environment value cannot be revealed",
  },
  "services.envEdit": {
    message: "編集",
    description: "Environment row button to edit a variable's value",
  },
  "services.envDelete": {
    message: "削除",
    description: "Environment row button to remove a variable",
  },
  "services.envValueFor": {
    message: "{name} の値",
    description:
      "Accessible name for a Value textbox once its Key is set (m101/t013)",
  },
  "services.envDeleteOne": {
    message: "{name} を削除",
    description:
      "Accessible name for a Delete button once its Key is set (m101/t013)",
  },
  "services.envSave": {
    message: "保存",
    description: "Environment add/edit form save button",
  },
  "services.envCancel": {
    message: "キャンセル",
    description: "Environment add/edit form cancel button",
  },
  "services.envKeyPlaceholder": {
    message: "NAME_OF_VARIABLE",
    description: "Environment add-variable key input placeholder",
  },
  "services.envValuePlaceholder": {
    message: "値",
    description: "Environment value input placeholder",
  },
  "services.envGenerate": {
    message: "生成",
    description: "Generate a cryptographically random environment value",
  },
  "services.envGeneratePlaceholder": {
    message: "保存時に安全に生成されます",
    description:
      "Environment value placeholder while server generation is selected",
  },
  "services.envInvalidKey": {
    message:
      "英数字、ドット、ハイフン、アンダースコアを 253 文字以内で使用してください。「.」のみ、または「..」で始まる名前は使えません。",
    description:
      "Environment add-variable validation message for an invalid key",
  },
  "services.envDeleteConfirmTitle": {
    message: "{key} を削除しますか？",
    description: "Environment delete-confirmation dialog title",
  },
  "services.envDeleteConfirmBody": {
    message: "この変数なしでサービスが再デプロイされます。",
    description: "Environment delete-confirmation dialog body",
  },
  "services.envRolloutNote": {
    message: "変更を反映するため、サービスを再デプロイしています。",
    description:
      "Toast description after an env-var write (bex rolls the pods)",
  },
  "services.envSaveSuccess": {
    message: "{key} を保存しました",
    description: "Toast on a successful env-var add/update",
  },
  "services.envSaveError": {
    message: "{key} を保存できませんでした",
    description: "Toast on a failed env-var add/update",
  },
  "services.envDeleteSuccess": {
    message: "{key} を削除しました",
    description: "Toast on a successful env-var delete",
  },
  "services.envDeleteError": {
    message: "{key} を削除できませんでした",
    description: "Toast on a failed env-var delete",
  },
  "services.secretFilesTitle": {
    message: "シークレットファイル",
    description: "Environment tab secret-files section title",
  },
  "services.secretFilesDescription": {
    message:
      "秘密鍵や .env ファイルなど、シークレットの内容を持つファイルを保存します。次のデプロイ後、サービスは各ファイルを /etc/secrets/<ファイル名> で読み取れます。",
    description: "Environment tab secret-files section description",
  },
  "services.secretFileColName": {
    message: "ファイル名",
    description: "Secret-files table column header (file name)",
  },
  "services.secretFileColContent": {
    message: "内容",
    description: "Secret-files table column header (file body)",
  },
  "services.secretFileRevealError": {
    message:
      "このシークレットファイルの内容を読み込めませんでした。ダイアログを閉じて、もう一度お試しください。",
    description:
      "Shown in the secret-file dialog when reading the stored contents fails.",
  },
  "services.secretFilesEmptyTitle": {
    message: "シークレットファイルはありません",
    description: "Secret-files empty-state title",
  },
  "services.secretFilesEmptyBody": {
    message:
      "ファイルを追加して、シークレットの内容をこのサービスにマウントします。",
    description: "Secret-files empty-state body",
  },
  "services.secretFilesUnavailableTitle": {
    message: "シークレットファイルは利用できません",
    description:
      "Secret-files state when the secret store is unconfigured (503)",
  },
  "services.secretFilesUnavailableBody": {
    message: "このデプロイ環境ではシークレットストアが設定されていません。",
    description: "Secret-files unavailable-state body",
  },
  "services.secretFilesForbiddenTitle": {
    message: "権限がありません",
    description: "Secret-files state when the caller lacks permission (403)",
  },
  "services.secretFilesForbiddenBody": {
    message: "このサービスのシークレットファイルを表示する権限がありません。",
    description: "Secret-files forbidden-state body",
  },
  "services.secretFilesErrorTitle": {
    message: "シークレットファイルを読み込めませんでした",
    description: "Secret-files generic error title",
  },
  "services.secretFilesErrorBody": {
    message: "問題が発生しました。もう一度お試しください。",
    description: "Secret-files generic error body",
  },
  "services.secretFileAdd": {
    message: "シークレットファイルを追加",
    description: "Secret-files button to open the add-file form",
  },
  "services.secretFileNamePlaceholder": {
    message: "filename.ext",
    description: "Secret-files add-file name input placeholder",
  },
  "services.secretFileContentPlaceholder": {
    message: "ファイルの内容",
    description: "Secret-files content input placeholder",
  },
  "services.secretFileInvalidName": {
    message:
      "英数字、ドット、ハイフン、アンダースコアを 253 文字以内で使用してください。「.」や「..」は使えません。",
    description: "Secret-files add-file validation message for an invalid name",
  },
  "services.secretFileNameHint": {
    message:
      "英数字、ドット、ハイフン、アンダースコアを 253 文字以内で使用してください。「.」や「..」は使えません。",
    description:
      "Neutral file-name rule under a new secret-file row before the user types or blurs — same rule as secretFileInvalidName but helper styling, not an error",
  },
  "services.secretFileDeleteConfirmTitle": {
    message: "{name} を削除しますか？",
    description: "Secret-file delete-confirmation dialog title",
  },
  "services.secretFileDeleteConfirmBody": {
    message: "このファイルなしでサービスが再デプロイされます。",
    description: "Secret-file delete-confirmation dialog body",
  },
  "services.secretFileSaveSuccess": {
    message: "{name} を保存しました",
    description: "Toast on a successful secret-file add/update",
  },
  "services.secretFileSaveError": {
    message: "{name} を保存できませんでした",
    description: "Toast on a failed secret-file add/update",
  },
  "services.secretFileDeleteSuccess": {
    message: "{name} を削除しました",
    description: "Toast on a successful secret-file delete",
  },
  "services.secretFileDeleteError": {
    message: "{name} を削除できませんでした",
    description: "Toast on a failed secret-file delete",
  },
  "services.envGroupsTitle": {
    message: "環境グループ",
    description: "Environment tab env-groups section title",
  },
  "services.envGroupsDescription": {
    message:
      "このサービスや他のサービスにリンクできる、環境変数とシークレットファイルの再利用可能なまとまりです。",
    description: "Environment tab env-groups section description",
  },
  "services.envGroupsEmptyTitle": {
    message: "環境グループはありません",
    description: "Env-groups empty-state title",
  },
  "services.envGroupsEmptyBody": {
    message: "グループを作成して、サービス間で設定を共有します。",
    description: "Env-groups empty-state body",
  },
  "services.envGroupsUnavailableTitle": {
    message: "環境グループは利用できません",
    description: "Env-groups state when the secret store is unconfigured (503)",
  },
  "services.envGroupsUnavailableBody": {
    message: "このデプロイ環境ではシークレットストアが設定されていません。",
    description: "Env-groups unavailable-state body",
  },
  "services.envGroupsForbiddenTitle": {
    message: "権限がありません",
    description: "Env-groups state when the caller lacks permission (403)",
  },
  "services.envGroupsForbiddenBody": {
    message: "環境グループを表示する権限がありません。",
    description: "Env-groups forbidden-state body",
  },
  "services.envGroupsErrorTitle": {
    message: "環境グループを読み込めませんでした",
    description: "Env-groups generic error title",
  },
  "services.envGroupsErrorBody": {
    message: "問題が発生しました。もう一度お試しください。",
    description: "Env-groups generic error body",
  },
  "services.envGroupCreate": {
    message: "グループを作成",
    description: "Env-groups button to open the create-group form",
  },
  "services.envGroupCreateSubmit": {
    message: "作成",
    description: "Env-groups create-group form submit button",
  },
  "services.envGroupNamePlaceholder": {
    message: "group-name",
    description: "Env-groups create-group name input placeholder",
  },
  "services.envGroupNameLabel": {
    message: "グループ名",
    description: "Env-groups create-group name input accessible label",
  },
  "services.envGroupInvalidName": {
    message: "グループ名を入力してください。",
    description:
      "Env-groups create-group validation message for an invalid name",
  },
  "services.envGroupLinked": {
    message: "リンク済み",
    description:
      "Env-groups badge: this group is linked to the current service",
  },
  "services.envGroupWorkspaceScope": {
    message: "ワークスペース",
    description: "Scope label for a workspace-scoped environment group",
  },
  "services.envGroupOutOfScope": {
    message:
      "スコープは {groupScope} ですが、このサービスは {serviceScope} にあります。変数とファイルは引き続き適用されます。",
    description:
      "Notice on a linked group whose scope no longer matches the service (w4/m238)",
  },
  "services.envGroupOutOfScopeNoEnvironment": {
    message:
      "スコープは {groupScope} ですが、このサービスには環境がありません。変数とファイルは引き続き適用されます。",
    description:
      "Notice on a linked group scoped to an environment while the service has none (w4/m238)",
  },
  "services.envGroupEmptyContents": {
    message: "変数やファイルはまだありません。",
    description: "Env-groups: shown when a group has no vars or secret files",
  },
  "services.envGroupKeyOverridden": {
    message: "このサービス自身の環境変数 {key} で上書きされています",
    description:
      "Tooltip on a linked group's key badge when a service variable of the same name shadows it at runtime (service wins)",
  },
  "services.envGroupFileOverridden": {
    message:
      "このサービス自身のシークレットファイル {name} で上書きされています",
    description:
      "Tooltip on a linked group's secret-file badge when a service secret file of the same name shadows it at runtime (service wins)",
  },
  "services.envGroupKeyShadowedByGroup": {
    message: "後から作成された {group} によって上書きされています",
    description:
      "Tooltip on a linked group's key badge when a MORE recently created linked group defines the same key and wins it",
  },
  "services.envGroupFileShadowedByGroup": {
    message: "後から作成された {group} によって上書きされています",
    description:
      "Tooltip on a linked group's secret-file badge when a MORE recently created linked group defines the same file name and wins it",
  },
  "services.envGroupOverriddenNote": {
    message:
      "取り消し線のキーは、このサービス自身の環境変数で上書きされています。",
    description:
      "Legend under a linked group's key list when at least one key is shadowed by a service variable",
  },
  "services.envGroupLink": {
    message: "リンク",
    description: "Env-groups button: attach this group to the current service",
  },
  "services.envGroupUnlink": {
    message: "リンク解除",
    description:
      "Env-groups button: detach this group from the current service",
  },
  "services.envGroupLinkAria": {
    message: "{name} をリンク",
    description:
      "Accessible name of an env-group row's Link button on the service Environment tab, naming the group (w4/537)",
  },
  "services.envGroupUnlinkAria": {
    message: "{name} のリンクを解除",
    description:
      "Accessible name of an env-group row's Unlink button on the service Environment tab, naming the group (w4/537)",
  },
  "services.envGroupDelete": {
    message: "削除",
    description: "Env-groups action: delete the group",
  },
  "services.envGroupDeleteConfirmTitle": {
    message: "{name} を削除しますか？",
    description: "Env-group delete-confirmation dialog title",
  },
  "services.envGroupDeleteConfirmBody": {
    message:
      "グループは、リンクされているすべてのサービスから外されます。この操作は元に戻せません。",
    description: "Env-group delete-confirmation dialog body",
  },
  "services.envGroupCreateSuccess": {
    message: "{name} を作成しました",
    description: "Toast on a successful env-group create",
  },
  "services.envGroupCreateError": {
    message: "{name} を作成できませんでした",
    description: "Toast on a failed env-group create",
  },
  "services.envGroupDeleteSuccess": {
    message: "グループを削除しました",
    description: "Toast on a successful env-group delete",
  },
  "services.envGroupDeleteError": {
    message: "グループを削除できませんでした",
    description: "Toast on a failed env-group delete",
  },
  "services.envGroupLinkSuccess": {
    message: "グループをリンクしました",
    description: "Toast on a successful env-group link",
  },
  "services.envGroupLinkError": {
    message: "グループをリンクできませんでした",
    description: "Toast on a failed env-group link",
  },
  "services.envGroupUnlinkSuccess": {
    message: "グループのリンクを解除しました",
    description: "Toast on a successful env-group unlink",
  },
  "services.envGroupUnlinkError": {
    message: "グループのリンクを解除できませんでした",
    description: "Toast on a failed env-group unlink",
  },
  "services.navSettings": {
    message: "設定",
    description: "Service-detail nav item (settings tab)",
  },
  "services.settingsNavigation": {
    message: "設定セクション",
    description: "Accessible label for the service settings section navigation",
  },
  "services.settingsDescription": {
    message:
      "このサービスの名前、インスタンスサイズ、その他の設定を構成します。",
    description: "Settings tab card description",
  },
  "services.generalTitle": {
    message: "一般",
    description:
      "Settings tab: first card heading, Render's 'General' section (w5/m52)",
  },
  "services.regionLabel": {
    message: "リージョン",
    description:
      "Settings General card: read-only platform placement row label",
  },
  "services.regionHint": {
    message:
      "サービスが実行されるリージョンです。同じリージョンのサービス同士は、プライベートネットワークで通信できます。",
    description: "Settings General card: Region row explanatory copy",
  },
  "services.editRowSave": {
    message: "変更を保存",
    description:
      "Shared editable-field row: save button shown while editing (Render parity)",
  },
  "services.editRowCancel": {
    message: "キャンセル",
    description: "Shared editable-field row: cancel button shown while editing",
  },
  "services.editRowOptional": {
    message: "任意",
    description:
      "Shared editable-field row: badge next to a label for an optional field",
  },
  "services.displayNameLabel": {
    message: "サービス名",
    description:
      "Settings tab row label for the mutable human-facing service name",
  },
  "services.displayNameHint": {
    message: "サービス ID は {id} のままで、URL やインフラは変わりません。",
    description:
      "Settings tab explanation that a display-name change preserves identity",
  },
  "services.displayNameEdit": {
    message: "サービス名を編集",
    description: "Accessible label for the service-name edit button",
  },
  "services.displayNameSuccess": {
    message: "サービス名を「{name}」に変更しました。",
    description: "Toast after setDisplayName succeeds",
  },
  "services.displayNameCleared": {
    message: "カスタムのサービス名をクリアしました。",
    description: "Toast after clearing displayName",
  },
  "services.displayNameRefreshFailed": {
    message:
      "名前を「{name}」に変更しましたが、このページを更新できませんでした。もう一度試すか、再読み込みして保存された名前を確認してください。",
    description:
      "Warning after a service rename committed but the page could not re-read the saved name; offers Try again",
  },
  "services.displayNameUnpublished": {
    message:
      "新しい名前は保存されましたが、このページにはまだ表示できません。再度編集する前に、通知の「再試行」を使うか、再読み込みしてください。",
    description:
      "Why the service name row is locked after a rename saved but its re-read failed",
  },
  "services.displayNameError": {
    message: "サービス名を変更できませんでした。もう一度お試しください。",
    description: "Toast after setDisplayName fails",
  },
  "services.settingsInstanceType": {
    message: "コンピュートプラン",
    description: "Settings tab row label for the App's current plan/tier",
  },
  "services.settingsNoInstanceType": {
    message: "コンピュートプランが設定されていません",
    description: "Settings tab state for an untiered (bare-CR) App",
  },
  "services.settingsUpdate": {
    message: "更新",
    description: "Settings tab link to the compute-plan picker",
  },
  "services.settingsIdleTimeout": {
    message: "アイドルタイムアウト",
    description:
      "Settings tab: label for the free-tier auto-sleep window control",
  },
  "services.settingsIdleTimeoutHint": {
    message:
      "Free サービスは、公開 HTTP リクエストや WebSocket メッセージがない状態がこの時間続くとスリープし、次の公開リクエストで起動します。他のサービスからの内部リクエストでは、スリープの回避も起動もされません。",
    description: "Settings tab: idle-timeout control help text (bex extension)",
  },
  "services.settingsIdleTimeoutPaid": {
    message: "有料サービスは常時稼働し、スリープしません。",
    description: "Settings tab: shown instead of the control on a paid plan",
  },
  "services.idleTimeoutDefault": {
    message: "プラットフォームのデフォルト",
    description: "Idle-timeout option: 0 seconds = the operator's own window",
  },
  "services.idleTimeoutMinutes": {
    message: "{minutes} 分",
    description: "Idle-timeout option label in minutes",
  },
  "services.idleTimeoutHours": {
    message: "{hours} 時間",
    description: "Idle-timeout option label in hours",
  },
  "services.idleTimeoutSeconds": {
    message: "{seconds} 秒",
    description: "Idle-timeout option label in seconds (non-round values)",
  },
  "services.idleTimeoutSuccess": {
    message: "アイドルタイムアウトを更新しました。",
    description: "Toast after setIdleTimeout succeeds",
  },
  "services.idleTimeoutError": {
    message: "アイドルタイムアウトを更新できませんでした。",
    description: "Toast after setIdleTimeout fails",
  },
  "services.settingsMaxShutdownDelay": {
    message: "最大シャットダウン遅延",
    description: "Settings row label for the graceful SIGTERM window",
  },
  "services.settingsMaxShutdownDelayHint": {
    message:
      "SIGTERM の後、プロセスを強制停止するまで 1 〜 300 秒待機します（デフォルトは 30）。",
    description: "Settings row help for the shutdown-delay range",
  },
  "services.maxShutdownDelayDiskReason": {
    message: "永続ディスクがあるサービスのシャットダウン遅延は変更できません。",
    description:
      "Settings hint replacing the range on a service with a disk (Render's rule, render.com/docs/disks); a link to the Disk tab follows",
  },
  "services.maxShutdownDelayDiskLink": {
    message: "ディスクを表示",
    description:
      "Link after maxShutdownDelayDiskReason to the service's Disk tab",
  },
  "services.maxShutdownDelayEdit": {
    message: "最大シャットダウン遅延を編集",
    description: "Accessible label for the shutdown-delay edit button",
  },
  "services.maxShutdownDelaySuccess": {
    message:
      "最大シャットダウン遅延を更新しました。新しいデプロイが開始されました。",
    description: "Toast after setMaxShutdownDelay succeeds",
  },
  "services.maxShutdownDelayRangeError": {
    message: "{min} 〜 {max} の整数の秒数を入力してください。",
    description:
      "Inline validation message for a max shutdown delay outside 1–300 or not a whole number (w4/494)",
  },
  "services.maxShutdownDelayError": {
    message: "最大シャットダウン遅延を更新できませんでした。",
    description: "Toast after setMaxShutdownDelay fails",
  },
  "services.settingsHealthChecksTitle": {
    message: "ヘルスチェック",
    description: "Settings tab: Health Checks section card title",
  },
  "services.settingsHealthChecksDescription": {
    message:
      "サービスを監視するために bex が定期的にポーリングする HTTP パスを設定します。",
    description: "Settings tab: Health Checks section card description",
  },
  "services.settingsHealthCheckPath": {
    message: "ヘルスチェックパス",
    description: "Settings tab: health-check path row label",
  },
  "services.settingsHealthCheckPathHint": {
    message:
      "サービスが正常かどうかを判断するために bex がポーリングする HTTP パスです。5 秒以内に 2xx または 3xx を返す必要があり、60 秒間連続して失敗するとインスタンスが再起動されるため、負荷の小さいルートを指定してください。空欄にすると、プロセスがポートで待ち受けているかどうかだけを確認します。2xx を返す負荷の小さいルートがない場合は、こちらが適切です。",
    description: "Settings tab: health-check path row hint text",
  },
  "services.settingsHealthCheckPathPlaceholder": {
    message: "空欄: ポートのみ確認",
    description: "Settings tab: health-check path input placeholder",
  },
  "services.settingsHealthCheckPathEdit": {
    message: "ヘルスチェックパスを編集",
    description:
      "Settings tab: accessible label for the health-check path edit-pencil button",
  },
  "services.healthCheckPathSuccess": {
    message:
      "ヘルスチェックパスを更新しました。新しいデプロイが開始されました。",
    description: "Toast after setHealthCheckPath succeeds",
  },
  "services.healthCheckPathError": {
    message: "ヘルスチェックパスを更新できませんでした。",
    description: "Toast after setHealthCheckPath fails",
  },
  "services.settingsPortTitle": {
    message: "ポート",
    description: "Settings tab: Port section card title",
  },
  "services.settingsPortDescription": {
    message:
      "コンテナが待ち受けるポートです。bex はこのポートにトラフィックをルーティングし、$PORT として渡します。",
    description: "Settings tab: Port section card description",
  },
  "services.settingsPort": {
    message: "ポート",
    description: "Settings tab: service port row label",
  },
  "services.settingsPortHint": {
    message:
      "コンテナが待ち受けるポートです（デフォルトは {port}）。bex はこのポートにトラフィックをルーティングし、$PORT として渡します。Render と同様に、サービスの PORT 環境変数がある場合はそちらが優先されます。保存すると新しいデプロイが開始され、インスタンスが再起動されます。",
    description: "Settings tab: service port row hint text",
  },
  "services.settingsPortDetected": {
    message: "検出: {port}",
    description:
      "Settings tab: port placeholder for a service whose serving port bex detected from the listener its process binds, while it names no port of its own (w8/338)",
  },
  "services.settingsPortDetectedHint": {
    message:
      "bex はサービスがポート {port} で待ち受けていることを検出し、そのポートにトラフィックをルーティングしています。Render と同様に、$PORT は {defaultPort} のままです。ポートを入力すると、そのポートを設定できます。bex はそのポートにトラフィックをルーティングし、$PORT として渡します。保存すると新しいデプロイが開始され、インスタンスが再起動されます。",
    description:
      "Settings tab: service port row hint for a service serving on a detected port; PORT stays the default (w8/177, w8/267, w8/338)",
  },
  "services.settingsPortImageHint": {
    message:
      "このサービスは、イメージの EXPOSE 命令からポートを取得します。ここでポートを設定すると、bex がルーティング、ヘルスチェック、$PORT として渡す対象のプライマリリスナーを選択します。イメージで宣言されている他の TCP ポートは引き続き公開されます。クリアすると、再びイメージのポートを使用します。保存すると新しいデプロイが開始され、インスタンスが再起動されます。",
    description:
      "Settings tab: service port row hint for an image-port service (a Docker-built or prebuilt-image web or private service) whose ports come from its image's EXPOSE instructions; a blank port returns it to the image's (w6/m148, w4/m243, w4/m323)",
  },
  "services.settingsPortEdit": {
    message: "ポートを編集",
    description:
      "Settings tab: accessible label for the service port edit-pencil button",
  },
  "services.portRangeError": {
    message: "{min} 〜 {max} のポートを入力してください。",
    description:
      "Validation message for a port outside the range the container can bind",
  },
  "services.portSuccess": {
    message: "ポートを更新しました。新しいデプロイをロールアウトしています。",
    description: "Toast after setPort succeeds",
  },
  "services.portError": {
    message: "ポートを更新できませんでした。",
    description: "Toast after setPort fails",
  },
  "services.settingsNotificationsTitle": {
    message: "通知",
    description: "Settings tab: Notifications section card title",
  },
  "services.settingsNotificationsDescription": {
    message: "このサービスについて受け取るデプロイ通知を選択します。",
    description: "Settings tab: Notifications section card description",
  },
  "services.notificationsLabel": {
    message: "サービスの通知",
    description: "Settings tab: service notification policy label",
  },
  "services.notificationsHint": {
    message:
      "このサービスについて、ワークスペースの既定値を上書きします。ワークスペースの既定値は、ワークスペース設定の「通知」で設定します。",
    description: "Settings tab: service notification policy explanation",
  },
  "services.notificationsEdit": {
    message: "サービスの通知を編集",
    description: "Accessible label for the service-notifications edit button",
  },
  "services.notificationsOptionDefault": {
    message: "ワークスペースの既定値を使用（{level}）",
    description:
      "Service notification option: inherit workspace default; {level} is the workspace's saved level (e.g. Only failure notifications)",
  },
  "services.notificationsOptionDefaultUnknown": {
    message: "ワークスペースの既定値を使用",
    description:
      "Service notification option: inherit workspace default, while the workspace level is loading or unreadable",
  },
  "services.notificationsOptionAll": {
    message: "すべての通知",
    description: "Service notification option: all lifecycle mail",
  },
  "services.notificationsOptionFailure": {
    message: "失敗の通知のみ",
    description: "Service notification option: failures only",
  },
  "services.notificationsOptionNone": {
    message: "なし",
    description: "Service notification option: no mail",
  },
  "services.notificationsSuccess": {
    message: "サービスの通知設定を更新しました。",
    description: "Toast after notification policy update succeeds",
  },
  "services.notificationsError": {
    message: "サービスの通知設定を更新できませんでした。",
    description: "Toast after notification policy update fails",
  },
  "services.notifyOnFailLabel": {
    message: "デプロイ失敗の通知",
    description: "Settings tab: notifyOnFail row label",
  },
  "services.notifyOnFailHint": {
    message:
      "デフォルトでは各メンバー自身の通知設定に従います。このサービスだけ、強制的にオンまたはオフにすることもできます。",
    description: "Settings tab: notifyOnFail row hint text",
  },
  "services.notifyOnFailOptionDefault": {
    message: "メンバーの設定を使用",
    description: "notifyOnFail select option: default",
  },
  "services.notifyOnFailOptionNotify": {
    message: "常に通知",
    description: "notifyOnFail select option: notify",
  },
  "services.notifyOnFailOptionIgnore": {
    message: "通知しない",
    description: "notifyOnFail select option: ignore",
  },
  "services.notifyOnFailSuccess": {
    message: "通知設定を更新しました。",
    description: "Toast after setNotifyOnFail succeeds",
  },
  "services.notifyOnFailError": {
    message: "通知設定を更新できませんでした。",
    description: "Toast after setNotifyOnFail fails",
  },
  "services.planPickerTitle": {
    message: "コンピュートプラン",
    description:
      "Accessible name of the Compute section's plan cards, shown after Edit (w6/m483)",
  },
  "services.computeTitle": {
    message: "コンピュート",
    description:
      "Compute page: title of the section holding the service's compute plan (Render: 'Under Compute, click Edit', w6/m483)",
  },
  "services.computePlanEdit": {
    message: "編集",
    description:
      "Compute section button that opens the compute plan cards (w6/m483)",
  },
  "services.planPickerFreeGroup": {
    message: "Free",
    description:
      "Plan-picker section label separating the Free tier from paid tiers",
  },
  "services.planPickerFreeSmtp": {
    message:
      "Free サービスは、SMTP ポート 25、465、587 でメールを送信できません。",
    description:
      "Plan picker note under the Free group: free instances are refused outbound SMTP, as on Render (w8/m127)",
  },
  "services.planPickerPaidSmtp": {
    message:
      "有料サービスは、STARTTLS を使用してポート 587 または 2525 でメールを送信できます。ポート 25 と 465 は、どのプランでも利用できません。",
    description:
      "Plan picker note under the Paid group: outbound SMTP ports paid services may use (w8/m127)",
  },
  "services.planPickerLegacyFreeWorker": {
    message:
      "このバックグラウンドワーカーは Free プランですが、バックグラウンドワーカーは Free を利用できなくなりました。引き続き稼働します。有料のコンピュートプランを選択すると Free から移行し、元には戻せません。",
    description:
      "Plan picker notice: this legacy service type is still on Free, which is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerUntieredWorker": {
    message:
      "このバックグラウンドワーカーにはコンピュートプランがなく、バックグラウンドワーカーは Free を利用できなくなりました。引き続き稼働します。有料のコンピュートプランを選択して設定してください。",
    description:
      "Plan picker notice: this service type has no compute plan, and Free is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerLegacyFreePrivateService": {
    message:
      "このプライベートサービスは Free プランですが、プライベートサービスは Free を利用できなくなりました。引き続き稼働します。有料のコンピュートプランを選択すると Free から移行し、元には戻せません。",
    description:
      "Plan picker notice: this legacy service type is still on Free, which is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerUntieredPrivateService": {
    message:
      "このプライベートサービスにはコンピュートプランがなく、プライベートサービスは Free を利用できなくなりました。引き続き稼働します。有料のコンピュートプランを選択して設定してください。",
    description:
      "Plan picker notice: this service type has no compute plan, and Free is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerLegacyFreeCronJob": {
    message:
      "この Cron Job は Free プランですが、Cron Job は Free を利用できなくなりました。引き続き稼働します。有料のコンピュートプランを選択すると Free から移行し、元には戻せません。",
    description:
      "Plan picker notice: this legacy service type is still on Free, which is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerUntieredCronJob": {
    message:
      "この Cron Job にはコンピュートプランがなく、Cron Job は Free を利用できなくなりました。引き続き稼働します。有料のコンピュートプランを選択して設定してください。",
    description:
      "Plan picker notice: this service type has no compute plan, and Free is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerPaidGroup": {
    message: "有料",
    description: "Plan-picker section label for the paid tier ladder",
  },
  "services.planCurrent": {
    message: "現在",
    description:
      "Plan picker: badge on the instance type the service is on now",
  },
  "services.planPickerCancel": {
    message: "キャンセル",
    description: "Plan-picker footer button: discard the selection",
  },
  "services.planPickerSave": {
    message: "保存",
    description:
      'Compute section footer button: confirm the plan change ("Select a new compute plan and click Save", render.com/docs/compute-plans)',
  },
  "services.planPickerConfirmTitle": {
    message: "コンピュートプランを {name} に変更しますか？",
    description: "Plan-change confirm dialog title",
  },
  "services.planPickerConfirmBody": {
    message:
      "サービスはダウンタイムなしでリサイズされ、ローリング更新されます。処理中のリクエストは、古いインスタンスが置き換えられる前に完了します。",
    description: "Plan-change confirm dialog body",
  },
  "services.instanceTypePausedReason": {
    message:
      "「{name}」は一時的に利用できません。{largest} またはそれより小さいコンピュートプランを選択してください。",
    description:
      "A paused compute plan's tooltip and screen-reader reason, composed in the dashboard's language from the tier and the largest still offered; matches bex-api's English unavailableReason (w4/552)",
  },
  "services.planPickerConfirmBodyCron": {
    message:
      "変更後に開始される実行では、新しいコンピュートプランが使用されます。",
    description:
      "Plan-change confirm body for a cron job, which serves no requests: its next run takes the new size (w4/548)",
  },
  "services.planPickerConfirmBodyNeverLive": {
    message:
      "サービスは新しいコンピュートプランで実行されます。まだ何も配信されておらず、デプロイが成功すると公開されます。",
    description:
      "Plan-change confirm body for a service that has never gone live (w4/548)",
  },
  "services.planPickerConfirmRunPrice": {
    message:
      "このコンピュートプランは、実行時間 1 分あたり ${price} で、実行中は秒単位で課金されます。Cron Job 1 件あたりの最低料金は月額 ${minimum} です。",
    description:
      "Plan-change confirm for a cron job: the new compute plan's run-time rate (w4/546); minimum is the cron job minimum in USD without the currency symbol (w9/m448)",
  },
  "services.createFieldPlanCronHint": {
    message:
      "Cron Job は、実行中は秒単位で課金されます。Cron Job 1 件あたりの最低料金は月額 ${minimum} です。",
    description:
      "Under the create form's compute plans for a cron job, which are priced per minute of run time (w4/546); minimum is the cron job minimum in USD without the currency symbol (w9/m448)",
  },
  "services.planPickerConfirmPrice": {
    message: "このコンピュートプランは月額 ${price} です。",
    description:
      "Plan-change confirm: the new compute plan's monthly price, from the API's price sheet (w4/400)",
  },
  "services.planPickerSuccess": {
    message: "コンピュートプランを {name} に更新しました",
    description: "Toast on a successful plan change",
  },
  "services.planPickerError": {
    message:
      "コンピュートプランを更新できませんでした。もう一度お試しください。",
    description: "Toast on a failed plan change",
  },
  "services.planPickerErrorTitle": {
    message: "コンピュートプランを読み込めませんでした",
    description: "Plan-picker error state title (instanceTypes query failed)",
  },
  "services.planPickerErrorBody": {
    message:
      "bex-api へのリクエストが失敗しました。接続を確認して、もう一度お試しください。",
    description: "Plan-picker error state body",
  },
  "services.domainsTitle": {
    message: "カスタムドメイン",
    description: "Settings tab custom-domains section title",
  },
  "services.domainsDescription": {
    message: "お持ちのカスタムドメインをこのサービスに向けます。",
    description: "Settings tab custom-domains section description",
  },
  "services.domainColName": {
    message: "名前",
    description: "Custom-domains table column header (the FQDN)",
  },
  "services.domainColVerified": {
    message: "検証ステータス",
    description: "Custom-domains table column header (DNS verification)",
  },
  "services.domainColCertificate": {
    message: "証明書ステータス",
    description:
      "Custom-domains table column header (TLS certificate serving state)",
  },
  "services.domainColActions": {
    message: "操作",
    description:
      "Custom-domains table actions column header (screen-reader only)",
  },
  "services.domainVerified": {
    message: "検証済み",
    description:
      "Custom-domains status badge: DNS or TLS verification is complete",
  },
  "services.domainCertActive": {
    message: "有効",
    description:
      "Custom-domains status badge: certificate issued and serving traffic",
  },
  "services.domainCertBlockedLabel": {
    message: "証明書がブロックされています:",
    description:
      "Label before cert-manager's verbatim reason a custom domain's TLS certificate has not been issued",
  },
  "services.domainCertBlockedHint": {
    message:
      "DNS レコードは DNS のみ（プロキシなし）にして、サービスの bex ホスト名を指す必要があります。そうしないと、証明書の確認が到達できません。",
    description:
      "Fix-it guidance shown with a blocked-certificate reason: proxied records (e.g. Cloudflare's orange cloud) break the ACME HTTP-01 challenge",
  },
  "services.domainPending": {
    message: "保留中",
    description:
      "Custom-domains status badge: DNS or certificate is not complete",
  },
  "services.domainActionsMenu": {
    message: "{domain} の操作",
    description: "Accessible label for the per-domain actions trigger",
  },
  "services.domainDelete": {
    message: "削除",
    description: "Custom-domains row action: remove the domain",
  },
  "services.domainCancel": {
    message: "キャンセル",
    description: "Custom-domains dialog cancel button",
  },
  "services.domainDeleteConfirmTitle": {
    message: "{name} を削除しますか？",
    description: "Custom-domain delete-confirmation dialog title",
  },
  "services.domainDeleteConfirmBody": {
    message:
      "サービスはこのドメインの配信を停止します。Ingress ルールは削除され、TLS 証明書は期限切れまで残ります。この操作は元に戻せません。",
    description: "Custom-domain delete-confirmation dialog body",
  },
  "services.domainDeletePairConfirmBody": {
    message:
      "{name} と、自動的に追加されたリダイレクト {sibling} を削除します。両方の Ingress ルールが削除され、TLS 証明書は期限切れまで残ります。この操作は元に戻せません。",
    description:
      "Custom-domain delete confirmation when deleting a canonical host also removes its generated redirecting sibling",
  },
  "services.domainDeleteGeneratedConfirmBody": {
    message:
      "{name} のみを削除します。{canonical} は引き続き正規のドメインで、リダイレクトされません。削除したドメインの Ingress ルールは削除され、TLS 証明書は期限切れまで残ります。この操作は元に戻せません。",
    description:
      "Custom-domain delete confirmation when deleting only the generated redirecting sibling",
  },
  "services.domainDeletePendingConfirmBody": {
    message:
      "{name} の保留中の申請を削除します。検証されていないため、配信は行われておらず、証明書も発行されていません。",
    description:
      "Custom-domain delete confirmation for a domain still unverified: it was never routed (w4/142)",
  },
  "services.domainDeletePendingPairConfirmBody": {
    message:
      "{name} と {sibling} の保留中の申請を削除します。どちらも検証されていないため、配信は行われておらず、証明書も発行されていません。",
    description:
      "Custom-domain delete confirmation for a still-pending domain and its auto-paired sibling (w4/142)",
  },
  "services.domainDeletePendingGeneratedConfirmBody": {
    message:
      "{name} の保留中の申請のみを削除します。{canonical} は正規のドメインのまま残ります。検証されていないため、配信は行われておらず、証明書も発行されていません。",
    description:
      "Custom-domain delete confirmation for a still-pending generated sibling: only it is removed, its canonical claim stays (w4/539)",
  },
  "services.domainAdd": {
    message: "カスタムドメインを追加",
    description: "Custom-domains button to open the add-domain dialog",
  },
  "services.domainAddTitle": {
    message: "カスタムドメインを追加",
    description: "Add-domain dialog title",
  },
  "services.domainAddDescription": {
    message:
      "お持ちのドメインを入力してください。DNS をこのサービスに向けると、bex が TLS 証明書を自動的に発行します。",
    description: "Add-domain dialog description",
  },
  "services.domainPlaceholder": {
    message: "www.example.com",
    description: "Add-domain FQDN input placeholder",
  },
  "services.domainInvalid": {
    message: "有効なドメインを入力してください。例: www.example.com",
    description: "Add-domain validation message for a malformed hostname",
  },
  "services.domainAddButton": {
    message: "ドメインを追加",
    description: "Add-domain dialog submit button",
  },
  "services.domainAddPairSuccess": {
    message: "{name} と {sibling} を追加しました",
    description:
      "Toast on a custom-domain add that auto-paired its www/apex sibling (w4/618)",
  },
  "services.domainAddSuccess": {
    message: "{name} を追加しました",
    description: "Toast on a successful custom-domain add",
  },
  "services.domainAddError": {
    message: "{name} を追加できませんでした",
    description: "Toast on a failed custom-domain add",
  },
  "services.domainAddConflict": {
    message: "{name} は別のサービスですでに使用されています",
    description:
      "Toast when a custom-domain add is rejected because the host is registered on another service (409)",
  },
  "services.domainAddReserved": {
    message: "{name} はプラットフォームの予約済みホスト名です",
    description:
      "Toast when a custom-domain add is rejected because the host is a platform-owned name (400)",
  },
  "services.domainAddOwnPlatformHost": {
    message:
      "{name} はすでにこのサービスのプラットフォームサブドメインのため、カスタムドメインとして追加できません",
    description:
      "Toast when a custom-domain add names the service's own platform subdomain (400 CUSTOM_DOMAIN_OWN_PLATFORM_HOST, w4/558)",
  },
  "services.domainDeletePairSuccess": {
    message: "{name} と {sibling} を削除しました",
    description:
      "Toast on a custom-domain delete that also removed its auto-paired sibling (w4/618)",
  },
  "services.domainDeleteSuccess": {
    message: "{name} を削除しました",
    description: "Toast on a successful custom-domain delete",
  },
  "services.domainDeleteError": {
    message: "{name} を削除できませんでした",
    description: "Toast on a failed custom-domain delete",
  },
  "services.domainPropagateNote": {
    message: "DNS と TLS 証明書はバックグラウンドで反映されます。",
    description:
      "Toast description after a custom-domain add (async convergence)",
  },
  "services.domainDnsToggle": {
    message: "{domain} の DNS 設定を表示",
    description:
      "aria-label for the per-domain DNS-instructions disclosure toggle",
  },
  "services.domainDnsTitle": {
    message: "DNS 設定",
    description: "Heading of the per-domain DNS-instructions panel",
  },
  "services.domainDnsSubdomainGuidance": {
    message:
      "DNS プロバイダーの {zone} ゾーンに、このレコードを作成してください。ホストはそのゾーンからの相対指定です。名前解決できるようになると、検証と TLS 証明書の発行は自動的に行われます。",
    description:
      "Guidance line above the DNS record for a subdomain — names the zone Host is relative to (w4/092)",
  },
  "services.domainDnsWildcardChallenge": {
    message:
      "ワイルドカードドメインには、このレコードも必要です。bex がドメインの TLS 証明書を発行・更新できるようになります。",
    description:
      "Above a wildcard custom domain's second DNS record, its _acme-challenge CNAME (w4/m326; Render's docs say it enables certificate issuance and renewal)",
  },
  "services.domainDnsApexFallback": {
    message:
      "ALIAS/ANAME や CNAME フラットニングが使えない場合は、代わりに下記の現在のエッジアドレスのいずれかを使って、@ に A レコードを作成してください。これらのアドレスは変更される可能性がありますが、ALIAS/ANAME レコードなら変更後も動作し続けます。",
    description:
      "A-record fallback under an apex domain's DNS instructions, shown with the resolved edge addresses (w4/378)",
  },
  "services.domainDnsCaaHint": {
    message:
      "ドメインに CAA レコードがある場合は、bex の認証局である Let's Encrypt（letsencrypt.org）と Google Trust Services（pki.goog）のレコードを追加してください。ワイルドカードドメインには、それぞれの issuewild レコードも必要です。",
    description:
      "Under every custom domain's DNS records: the CAA values bex's two certificate authorities need, as Render's custom-domains page names its own (w21/m618; the bex-domains CLI skill says the same)",
  },
  "services.domainDnsApexGuidance": {
    message:
      "Apex ドメインでは通常の CNAME は使えません。{zone} ゾーンに、この ALIAS/ANAME（または CNAME フラットニング）レコードを作成してください（@ はゾーンの apex を意味します）。プロバイダーがどちらにも対応していない場合は、ターゲットが解決される IP アドレスを使って @ に A レコードを作成してください。名前解決できるようになると、検証と TLS は自動的に行われ、bex が www ↔ apex のリダイレクトも設定します。",
    description:
      "Guidance line above the DNS record for an apex domain — names the zone Host is relative to (w4/092)",
  },
  "services.domainRecordType": {
    message: "タイプ",
    description: "Label for the DNS record type field (CNAME/ALIAS)",
  },
  "services.domainRecordHost": {
    message: "ホスト",
    description: "Label for the DNS record host/name field",
  },
  "services.domainRecordTarget": {
    message: "ターゲット",
    description: "Label for the DNS record target/value field",
  },
  "services.domainCopyHost": {
    message: "{domain} の {type} ホストをコピー",
    description: "Accessible label for copying a DNS host field",
  },
  "services.domainCopyTarget": {
    message: "{domain} の {type} ターゲットをコピー",
    description: "Accessible label for copying a DNS target/value field",
  },
  "services.domainDnsUnavailable": {
    message:
      "DNS ターゲットはまだ利用できません。サービスが稼働してからもう一度確認してください。",
    description: "Shown when the backend couldn't derive the DNS record target",
  },
  "services.domainRecheck": {
    message: "再確認",
    description: "Button that re-checks a domain's DNS/certificate status",
  },
  "services.domainRecheckAria": {
    message: "{domain} を再確認",
    description:
      "Accessible name of a custom domain's DNS-panel Re-check button, naming the domain (w4/540)",
  },
  "services.domainCopied": {
    message: "クリップボードにコピーしました",
    description: "Toast when a DNS record value is copied",
  },
  "services.domainCopyError": {
    message: "クリップボードにコピーできませんでした",
    description: "Toast when copying a DNS record value fails",
  },
  "services.domainAddedTitle": {
    message: "ドメインを追加しました。DNS を設定してください",
    description: "Title of the post-add DNS-record step in the add dialog",
  },
  "services.domainAddedDescription": {
    message:
      "プロバイダーで DNS レコードを作成してください。名前解決できるようになると、検証は自動的に完了します。",
    description: "Subtitle of the post-add DNS-record step in the add dialog",
  },
  "services.domainRedirectsTo": {
    message: "{canonical} にリダイレクト",
    description:
      "Note under an auto-paired sibling domain showing its canonical redirect target",
  },
  "services.domainRedirectsHere": {
    message: "{sibling} がここにリダイレクト",
    description:
      "Note under a canonical domain showing which auto-paired sibling redirects to it",
  },
  "services.domainPairedDnsTitle": {
    message:
      "{sibling} が自動的に追加され、{canonical} にリダイレクトされます。こちらの DNS も設定してください",
    description:
      "Heading of the second DNS-record block in the add dialog when the add auto-paired a www<->apex sibling (w6/m23)",
  },
  "services.domainDone": {
    message: "完了",
    description: "Button closing the post-add DNS-record step",
  },
  "services.domainVerifySuccess": {
    message: "{name} を検証しました",
    description: "Toast when a re-check finds the domain verified",
  },
  "services.domainVerifyPending": {
    message:
      "{name} はまだ検証されていません。DNS の反映に時間がかかっている可能性があります。",
    description: "Toast when a re-check finds the domain still pending",
  },
  "services.domainVerifyError": {
    message: "{name} を再確認できませんでした。",
    description: "Toast when the re-check request fails",
  },
  "services.domainsEmptyTitle": {
    message: "カスタムドメインはありません",
    description: "Custom-domains empty-state title",
  },
  "services.domainsEmptyBody": {
    message:
      "お持ちのドメインを追加すると、そのドメインでこのサービスを配信できます。",
    description: "Custom-domains empty-state body",
  },
  "services.domainsErrorTitle": {
    message: "カスタムドメインを読み込めませんでした",
    description: "Custom-domains generic error title",
  },
  "services.domainsErrorBody": {
    message:
      "bex-api へのリクエストが失敗しました。接続を確認して、もう一度お試しください。",
    description: "Custom-domains generic error body",
  },
  "services.platformSubdomainTitle": {
    message: "プラットフォームのサブドメイン",
    description: "Settings tab platform-subdomain section title",
  },
  "services.platformSubdomainDescription": {
    message:
      "設定済みのカスタムドメインに加えて、サービスが bex のプラットフォームサブドメインでも応答するかどうかを設定します。",
    description: "Settings tab platform-subdomain section description",
  },
  "services.platformSubdomainEnabled": {
    message: "有効",
    description: "Platform-subdomain toggle label when the subdomain is active",
  },
  "services.platformSubdomainDisabled": {
    message: "無効",
    description:
      "Platform-subdomain toggle label when the subdomain is disabled",
  },
  "services.platformSubdomainPending": {
    message: "プラットフォーム URL は、サービスが稼働すると割り当てられます。",
    description: "Platform-subdomain state when the service has no URL yet",
  },
  "services.platformSubdomainDisabledNote": {
    message:
      "プラットフォームのサブドメインは無効です。サービスにはカスタムドメインからのみアクセスできます。",
    description:
      "Platform-subdomain note shown when the policy is set to disabled",
  },
  "services.platformSubdomainToggleLabel": {
    message: "プラットフォームのサブドメインを切り替え",
    description: "Accessible label for the platform-subdomain Switch",
  },
  "services.platformSubdomainNeedsVerifiedDomain": {
    message:
      "サービスのアドレスを維持するため、プラットフォームのサブドメインをオフにする前にカスタムドメインを検証してください。",
    description:
      "Hint beside the platform-subdomain switch when no custom domain is verified yet (w4/142)",
  },
  "services.platformSubdomainNeedsVerifiedDomainPending": {
    message:
      "プラットフォームのサブドメインをオフにするには、{names} の検証を完了する必要があります。",
    description:
      "Hint beside the platform-subdomain switch when custom domains exist but are all still pending verification (w4/142)",
  },
  "services.subdomainPolicySuccess": {
    message: "プラットフォームのサブドメイン設定を更新しました。",
    description: "Toast on successful setSubdomainPolicy mutation",
  },
  "services.subdomainPolicyError": {
    message:
      "プラットフォームのサブドメイン設定を更新できませんでした。もう一度お試しください。",
    description: "Toast on failed setSubdomainPolicy mutation",
  },
  "services.maintenanceModeTitle": {
    message: "メンテナンスモード",
    description: "Settings tab maintenance-mode section title",
  },
  "services.maintenanceModeDescription": {
    message:
      "サービスを一時停止せずに、メンテナンスページの背後でオフラインにします。Pod は稼働し続け、公開トラフィックだけがリダイレクトされます。",
    description: "Settings tab maintenance-mode section description",
  },
  "services.maintenanceModeEnabled": {
    message: "有効",
    description: "Maintenance-mode toggle label when active",
  },
  "services.maintenanceModeDisabled": {
    message: "無効",
    description: "Maintenance-mode toggle label when inactive",
  },
  "services.maintenanceModeSwitchHint": {
    message:
      "このサービスが配信するすべてのホストが、メンテナンスページで応答します。",
    description: "Maintenance-mode section hint text next to the switch",
  },
  "services.maintenanceModePaidOnly": {
    message: "メンテナンスモードは、有料の Web Service プランで利用できます。",
    description: "Maintenance mode free-plan eligibility note",
  },
  "services.maintenanceModeToggleLabel": {
    message: "メンテナンスモードを切り替え",
    description: "Accessible label for the maintenance-mode Switch",
  },
  "services.maintenanceModeUriLabel": {
    message: "カスタムメンテナンスページの URL",
    description: "Label for the maintenance-mode custom-page URL field",
  },
  "services.maintenanceModeUriPlaceholder": {
    message: "https://status.example.com/maintenance （任意）",
    description: "Placeholder for the maintenance-mode custom-page URL field",
  },
  "services.maintenanceModeUriHint": {
    message:
      "デフォルトのページの代わりに、取得して配信されます。空欄にすると bex のデフォルトのメンテナンスページを使用します。このサービス自身の URL は指定できません。",
    description: "Hint text under the maintenance-mode custom-page URL field",
  },
  "services.maintenanceModeUriEdit": {
    message: "メンテナンスページの URL を編集",
    description:
      "Accessible label for the maintenance-page URL edit-pencil button",
  },
  "services.maintenanceModeEnableAction": {
    message: "メンテナンスモードを有効にする",
    description: "Confirm-dialog action button for enabling maintenance mode",
  },
  "services.confirmMaintenanceModeTitle": {
    message: "メンテナンスモードを有効にしますか？",
    description: "Confirm-dialog title for enabling maintenance mode",
  },
  "services.confirmMaintenanceModeBody": {
    message:
      "無効にするまで、{name} はすべての訪問者にメンテナンスページを表示します。サービスの Pod は稼働し続けます。",
    description: "Confirm-dialog body for enabling maintenance mode",
  },
  "services.maintenanceModeEnabledSuccess": {
    message: "メンテナンスモードを有効にしました。",
    description: "Toast on successfully enabling maintenance mode",
  },
  "services.maintenanceModeDisabledSuccess": {
    message: "メンテナンスモードを無効にしました。",
    description: "Toast on successfully disabling maintenance mode",
  },
  "services.maintenancePageSavedSuccess": {
    message: "メンテナンスページを保存しました。",
    description: "Toast on saving only the custom maintenance page URL",
  },
  "services.maintenancePageClearedSuccess": {
    message: "デフォルトのメンテナンスページに戻しました。",
    description: "Toast on clearing the custom maintenance page URL",
  },
  "services.maintenanceModeError": {
    message:
      "メンテナンスモードを更新できませんでした。もう一度お試しください。",
    description: "Toast on failed setMaintenanceMode mutation",
  },
  "services.edgeCachingTitle": {
    message: "エッジキャッシュ",
    description: "Settings tab edge-caching section title (Render parity)",
  },
  "services.edgeCachingDescription": {
    message:
      "レスポンスをエッジにキャッシュし、同じリクエストをこのサービスに到達させずに配信します。",
    description: "Settings tab edge-caching section description",
  },
  "services.edgeCachingFileTypesLabel": {
    message: "キャッシュ可能なファイルタイプ",
    description: "Edge-caching radio group label",
  },
  "services.edgeCachingOptionNone": {
    message: "なし",
    description: "Edge-caching option: no-cache profile",
  },
  "services.edgeCachingOptionNoneDesc": {
    message: "すべてのレスポンスをサービスから配信します。",
    description: "Edge-caching no-cache option description",
  },
  "services.edgeCachingOptionStatic": {
    message: "一般的な静的ファイル",
    description: "Edge-caching option: origin-controlled profile",
  },
  "services.edgeCachingOptionStaticDesc": {
    message:
      "CSS、JavaScript、画像、フォントなどの一般的な静的ファイルタイプをキャッシュします。ただし、レスポンスの Cache-Control ヘッダーで別の指定がある場合はそれに従います。",
    description: "Edge-caching origin-controlled option description",
  },
  "services.edgeCachingOptionAll": {
    message: "すべてのファイル",
    description: "Edge-caching option: origin-controlled-all profile",
  },
  "services.edgeCachingOptionAllDesc": {
    message:
      "Cache-Control ヘッダーで別の指定がない限り、対象となるすべてのレスポンスをキャッシュします。動的コンテンツがキャッシュされないよう、必ず Cache-Control を設定してください。",
    description:
      "Edge-caching origin-controlled-all option description, with the dynamic-content warning",
  },
  "services.edgeCachingPurgeOnDeploy": {
    message: "キャッシュはデプロイが成功するたびに消去されます。",
    description: "Note under the edge-caching options",
  },
  "services.edgeCachingPaidOnly": {
    message: "エッジキャッシュは、無料の Web サービスでは利用できません。",
    description: "Edge-caching free-plan eligibility note",
  },
  "services.edgeCachingSave": {
    message: "エッジキャッシュを保存",
    description: "Edge-caching save button",
  },
  "services.edgeCachingSaving": {
    message: "保存中…",
    description: "Edge-caching save button while the mutation is in flight",
  },
  "services.edgeCachingSaved": {
    message: "エッジキャッシュを更新しました。",
    description: "Toast on successful setCacheProfile mutation",
  },
  "services.edgeCachingError": {
    message: "エッジキャッシュを更新できませんでした。もう一度お試しください。",
    description: "Toast on failed setCacheProfile mutation",
  },
  "services.edgeCachingPurgeAction": {
    message: "キャッシュを消去",
    description: "Edge-caching purge button and confirm action",
  },
  "services.edgeCachingPurgeConfirmTitle": {
    message: "エッジキャッシュを消去しますか？",
    description: "Confirm-dialog title for purging the edge cache",
  },
  "services.edgeCachingPurgeConfirmBody": {
    message:
      "キャッシュされたすべてのレスポンスが破棄されます。キャッシュが再び温まるまで、リクエストはサービスに送られます。",
    description: "Confirm-dialog body for purging the edge cache",
  },
  "services.edgeCachingPurged": {
    message: "キャッシュの消去を開始しました。",
    description: "Toast on successful purgeServiceCache mutation",
  },
  "services.edgeCachingPurgeError": {
    message: "キャッシュを消去できませんでした。もう一度お試しください。",
    description: "Toast on failed purgeServiceCache mutation",
  },
  "services.runtimeTitle": {
    message: "ランタイム",
    description: "Settings tab runtime section title (Render parity)",
  },
  "services.runtimeDescription": {
    message: "bex がこのサービスをビルドして実行する環境です。",
    description: "Settings tab runtime section description",
  },
  "services.runtimeLabel": {
    message: "ランタイム",
    description: "Settings runtime select label",
  },
  "services.runtimeNote": {
    message:
      "ランタイムを変更すると、サービスが再ビルドされて再デプロイされます。ネイティブランタイムでは、下のビルドコマンドと開始コマンドを使用します。",
    description: "Note beside the Settings runtime select",
  },
  "services.runtimePlaceholder": {
    message: "ランタイムを選択",
    description:
      "Settings runtime select placeholder for a legacy service with no runtime recorded",
  },
  "services.runtimeSave": {
    message: "ランタイムを保存",
    description: "Settings runtime save button",
  },
  "services.runtimeSaving": {
    message: "保存中…",
    description: "Settings runtime save button while the mutation is in flight",
  },
  "services.runtimeConfirmTitle": {
    message: "ランタイムを {runtime} に変更しますか？",
    description: "Confirm-dialog title for changing a service's runtime",
  },
  "services.runtimeConfirmBody": {
    message:
      "サービスは {runtime} ランタイムで再ビルドされ、再デプロイされます。新しいデプロイが開始されます。",
    description: "Confirm-dialog body for changing a service's runtime",
  },
  "services.runtimeConfirmAction": {
    message: "ランタイムを変更",
    description: "Confirm-dialog action for changing a service's runtime",
  },
  "services.runtimeSaved": {
    message: "ランタイムを更新しました。新しいデプロイが開始されました。",
    description: "Toast on successful setRuntime mutation",
  },
  "services.runtimeError": {
    message: "ランタイムを変更できませんでした。もう一度お試しください。",
    description: "Toast on failed setRuntime mutation without a server message",
  },
  "services.maintenanceModeBannerTitle": {
    message: "メンテナンスモードがオンです",
    description: "Service-detail header banner title while in maintenance",
  },
  "services.maintenanceModeBannerBody": {
    message:
      "訪問者には、このサービスの代わりにメンテナンスページが表示されます。Pod は稼働し続けています。通常の配信に戻すには、設定でメンテナンスモードを無効にしてください。",
    description: "Service-detail header banner body while in maintenance",
  },
  "services.deployTitle": {
    message: "デプロイ",
    description: "Cron job Settings tab: Deploy section title (Render parity)",
  },
  "services.deployDescription": {
    message: "この Cron Job の実行方法です。",
    description: "Cron job Settings tab: Deploy section description",
  },
  "services.deploySuccess": {
    message: "Cron Job の設定を保存しました。",
    description: "Toast after updateCronJob succeeds",
  },
  "services.cronScheduleSaved": {
    message: "次回の実行は新しいスケジュールに従います。",
    description:
      "Toast description after saving a cron job's schedule alone: the next run follows it, nothing deploys (w4/593)",
  },
  "services.cronCommandSaved": {
    message:
      "新しいコマンドを適用するデプロイが実行中です。それ以降の実行で使用されます。",
    description:
      "Toast description after saving a cron job's command: the save rolls a deploy that applies it (w4/593)",
  },
  "services.deployError": {
    message: "Cron Job の設定を保存できませんでした。もう一度お試しください。",
    description: "Toast after updateCronJob fails",
  },
  "services.deployRefreshFailed": {
    message:
      "Cron Job の設定を保存しましたが、このページを更新できませんでした。もう一度試すか、再読み込みして保存された設定を確認してください。",
    description:
      "Warning after a cron save committed but its authoritative re-read failed; the save must not be retried or reported as failed (w4/276)",
  },
  "services.deployRefreshPending": {
    message:
      "設定を保存しました。さらに編集するには、最新の状態の読み込みをお待ちください。",
    description:
      "Reason shown while cron rows stay unwritable after a committed save whose refresh failed (w4/276)",
  },
  "services.deployScheduleLabel": {
    message: "スケジュール",
    description: "Cron job Settings tab: Deploy section schedule field label",
  },
  "services.deployScheduleHint": {
    message:
      "このスケジュールでこのコマンドを実行します（5 フィールドの crontab、UTC）。",
    description: "Cron job Settings tab: Deploy section schedule help text",
  },
  "services.deploySchedulePreview": {
    message: "{description} · UTC で実行",
    description:
      "Cron job Settings tab: live human-readable description of the schedule",
  },
  "services.deploySchedulePlaceholder": {
    message: "0 * * * *",
    description: "Cron job Deploy section: schedule input placeholder",
  },
  "services.deployScheduleEdit": {
    message: "スケジュールを編集",
    description: "Accessible label for the cron schedule edit button",
  },
  "services.deployCommandEdit": {
    message: "コマンドを編集",
    description: "Accessible label for the cron command edit button",
  },
  "services.cronScheduleNeverFires": {
    message:
      "このスケジュールは実行されません。日と月に一致する日付がありません（例: 31 2 は 2 月 31 日）。",
    description:
      "Cron schedule validation error when the schedule parses but no date ever matches (w4/197)",
  },
  "services.deployScheduleError": {
    message: "有効な 5 フィールドの cron 式を入力してください。例: 0 * * * *",
    description: "Cron job Deploy section: schedule validation error",
  },
  "services.deployScheduleRequired": {
    message: "スケジュールは必須です。",
    description: "Cron job Deploy section: schedule required validation error",
  },
  "services.deployCommandLabel": {
    message: "コマンド",
    description: "Cron job Settings tab: Deploy section command field label",
  },
  "services.deployCommandPlaceholder": {
    message: "例: python script.py",
    description: "Cron job Deploy section: command input placeholder",
  },
  "services.deployCommandHint": {
    message:
      "イメージのデフォルトのエントリーポイントを上書きします。空欄にすると、イメージ自身のコマンドを実行します。",
    description: "Cron job Deploy section: command field help text",
  },
  "services.deployCommandNativeHint": {
    message:
      "スケジュールごとにジョブが実行するコマンドで、ランタイムの開始コマンドとして使われます。",
    description:
      "Cron job Deploy section: command field help text for a native-runtime cron, whose command is its start command (w8/m69)",
  },
  "services.deployCommandRequired": {
    message: "コマンドは必須です。",
    description:
      "Cron job Deploy section: command required validation error for a native-runtime cron (w8/m69)",
  },
  "services.buildTitle": {
    message: "ビルド",
    description:
      "Settings tab: Build card title, Render's split Build section (w5/m52)",
  },
  "services.buildDeployNav": {
    message: "ビルドとデプロイ",
    description:
      "Settings section index: the one entry for the Build and Deploy cards, which the page renders as one section (Render names it Build & Deploy)",
  },
  "services.buildDescription": {
    message: "bex がリポジトリからサービスをビルドする方法を設定します。",
    description: "Settings tab: Build card description",
  },
  "services.deploySectionTitle": {
    message: "デプロイ",
    description:
      "Settings tab: Deploy card title, Render's split Deploy section (w5/m52)",
  },
  "services.deploySectionDescription": {
    message: "bex がサービスをデプロイして実行する方法を設定します。",
    description: "Settings tab: Deploy card description",
  },
  "services.buildDeploySourceLabel": {
    message: "ソース",
    description: "Build & Deploy: repo field label",
  },
  "services.buildDeploySourceHint": {
    message:
      "このサービスのビルド元の Git リポジトリです。連携済みのリポジトリから選択するか、リポジトリの URL を貼り付けてください。",
    description: "Build & Deploy: Source (repo) field help text (w5/m54)",
  },
  "services.buildDeploySourcePlaceholder": {
    message: "https://github.com/owner/repo",
    description: "Build & Deploy: Source combobox input placeholder",
  },
  "services.buildDeploySourceEdit": {
    message: "ソースを編集",
    description: "Accessible label for the Source edit-pencil button",
  },
  "services.buildDeploySourceConfirmTitle": {
    message: "ソースを {value} に切り替えますか？",
    description: "Build & Deploy: Source-change confirm dialog title",
  },
  "services.buildDeployBranchLabel": {
    message: "ブランチ",
    description: "Build & Deploy: branch field label",
  },
  "services.buildDeployBranchHint": {
    message: "ビルドしてデプロイする Git ブランチです。",
    description: "Build & Deploy: branch field hint (Render's copy)",
  },
  "services.buildDeployBranchEmpty": {
    message: "main",
    description:
      "Build & Deploy: branch value shown when unset (the backend default)",
  },
  "services.buildDeployBranchPlaceholder": {
    message: "main",
    description: "Build & Deploy: branch input placeholder",
  },
  "services.buildDeployBranchEdit": {
    message: "ブランチを編集",
    description: "Build & Deploy: branch edit-button aria label",
  },
  "services.buildDeployBranchConfirmTitle": {
    message: "ブランチを「{value}」に変更しますか？",
    description: "Build & Deploy: branch confirm-dialog title",
  },
  "services.buildDeployRootDirLabel": {
    message: "ルートディレクトリ",
    description: "Build & Deploy: root-directory field label",
  },
  "services.buildDeployRootDirOptional": {
    message: "任意",
    description:
      "Build & Deploy: badge next to the Root Directory label (Render parity)",
  },
  "services.buildDeployRootDirHint": {
    message:
      "設定すると、リポジトリのルートではなくこのサブディレクトリからビルドを実行します。その外側のコードの変更では、ビルドフィルターの含めるパスに一致しない限り自動デプロイは開始されません。モノレポで最もよく使われます。",
    description: "Build & Deploy: root-directory field help text",
  },
  "services.buildDeployConfirmRoot": {
    message: "リポジトリのルート",
    description:
      "Build & Deploy: mid-sentence phrase for the confirm dialog title when clearing rootDir to empty (a dedicated key, not a lowercased buildDeployRootDirEmpty, since that transform doesn't hold in every language)",
  },
  "services.buildDeployRootDirPlaceholder": {
    message: "例: backend",
    description: "Build & Deploy: root-directory input placeholder",
  },
  "services.buildDeployEdit": {
    message: "ルートディレクトリを編集",
    description: "Build & Deploy: accessible label for the edit-pencil button",
  },
  "services.buildDeployCancel": {
    message: "キャンセル",
    description: "Build & Deploy: root-directory inline-edit cancel button",
  },
  "services.buildDeployConfirmTitle": {
    message: "ルートディレクトリを {value} に変更しますか？",
    description: "Build & Deploy: root-directory change confirm dialog title",
  },
  "services.buildDeployConfirmBody": {
    message:
      "サービスは新しいディレクトリを対象に再ビルドされ、再デプロイされます。処理中のリクエストは、古いインスタンスが置き換えられる前に完了します。",
    description: "Build & Deploy: root-directory change confirm dialog body",
  },
  "services.buildDeployConfirmBodyStatic": {
    message:
      "新しいディレクトリから再ビルドを開始します。新しいビルドが正常に公開されるまで、サイトは現在公開中のバージョンを配信し続けます。",
    description:
      "Build & Deploy: a live static site's root-directory change confirm body (atomic publish, no instances; w4/548)",
  },
  "services.buildDeployConfirmBodyNeverLive": {
    message:
      "サービスは新しいディレクトリからビルドされ、デプロイされます。まだ何も配信されておらず、デプロイが成功すると公開されます。",
    description:
      "Build & Deploy: root-directory change confirm body for a service that has never gone live, so no current version keeps serving (w4/548)",
  },
  "services.buildDeploySuccess": {
    message: "ルートディレクトリを更新しました。",
    description: "Toast after setRootDir succeeds",
  },
  "services.buildDeployError": {
    message:
      "ルートディレクトリを更新できませんでした。もう一度お試しください。",
    description: "Toast after setRootDir fails",
  },
  "services.sourceUpdateSuccess": {
    message: "ソースを更新しました。",
    description:
      "Toast after a repository, branch, or image source update saves without triggering a deploy (e.g. suspended service or unbuilt build source)",
  },
  "services.sourceUpdateDeploying": {
    message: "ソースを更新しました。新しいデプロイを開始します。",
    description:
      "Toast after Update Source saves and immediately triggers a new deploy",
  },
  "services.sourceUpdateError": {
    message: "ソースを更新できませんでした。もう一度お試しください。",
    description:
      "Fallback toast when a source update fails without a server reason",
  },
  "services.sourceTitle": {
    message: "ソース",
    description: "Source card title",
  },
  "services.sourceRepoDescription": {
    message: "このサービスのビルド元の Git リポジトリとブランチです。",
    description: "Source card description for a repository-backed service",
  },
  "services.sourceImageDescription": {
    message: "このサービスがデプロイするビルド済みのコンテナイメージです。",
    description: "Source card description for an image-backed service",
  },
  "services.sourceImageLabel": {
    message: "イメージ",
    description: "Source card: container image path field label",
  },
  "services.sourceDialogDescription": {
    message:
      "Git リポジトリ、既存のビルドソース、または Docker イメージを選択してください。送信すると新しいデプロイが開始されます。",
    description:
      "Update Source dialog description for non-static services (Render parity)",
  },
  "services.sourceDialogDescriptionStatic": {
    message:
      "この Static Site の元となる Git リポジトリを選択してください。送信すると新しいデプロイが開始されます。",
    description:
      "Update Source dialog description for static sites (Render parity)",
  },
  "services.sourceMissing": {
    message: "未設定",
    description: "Source card fallback when no source value is available",
  },
  "services.sourceRepoAccessLostTitle": {
    message: "bex の GitHub アプリはこのリポジトリを読み取れません",
    description:
      "Settings → Source warning title: the GitHub App connected this service's repository and can no longer read it (repoAccessLost, w10/m490)",
  },
  "services.sourceRepoAccessLostBody": {
    message:
      "プッシュでこのサービスはデプロイされなくなりました。リポジトリが別の GitHub アカウントに移動した場合は、そのアカウントに GitHub アプリをインストールしてください。そうでない場合は、アプリにリポジトリへのアクセスを再度許可してください。",
    description:
      "Settings → Source warning body: pushes stop reaching bex once the GitHub App loses the repository (moved to an account without the app, or the grant was revoked); what to do (w10/m490)",
  },
  "services.sourceEdit": {
    message: "編集",
    description: "Button opening the Update Source dialog",
  },
  "services.sourceDialogTitle": {
    message: "ソースを更新",
    description: "Update Source dialog title",
  },
  "services.sourcePickerLabel": {
    message: "ソースを選択",
    description: "Label above the source kind picker in Update Source",
  },
  "services.sourceBranchHint": {
    message: "ブランチを選択するか、一覧にないブランチを入力してください。",
    description: "Branch picker free-text fallback hint",
  },
  "services.sourceBranchEmpty": {
    message: "一致するブランチがありません。ブランチ名を入力してください。",
    description: "Empty state for the Update Source branch picker",
  },
  "services.sourceTabBuildSource": {
    message: "ビルドソース",
    description: "Update Source tab: build the service from a build source",
  },
  "services.sourceBuildSourceDescription": {
    message:
      "このサービスがデプロイするビルドソースです。そのビルドは、リンクされているすべてのサービスで共有されます。",
    description:
      "Source card description for a service linked to a build source",
  },
  "services.sourceBuildSourceLabel": {
    message: "ビルドソース",
    description: "Source card and Update Source: build source field label",
  },
  "services.sourceBuildSourceBuildsFrom": {
    message: "ビルド元",
    description:
      "Source card: the repository or image the linked build source builds from",
  },
  "services.sourceBuildSourceEmpty": {
    message:
      "このワークスペースにはまだビルドソースがありません。API または MCP で作成してから、ここでサービスをリンクしてください。",
    description: "Update Source build source tab: empty state",
  },
  "services.sourceBuildSourceLoadError": {
    message:
      "このワークスペースのビルドソースを読み込めませんでした。ダイアログを閉じて、もう一度お試しください。",
    description: "Update Source build source tab: the list failed to load",
  },
  "services.sourceBuildSourcePlaceholder": {
    message: "ビルドソースを選択",
    description: "Update Source build source picker placeholder",
  },
  "services.sourceBuildSourceStartHint": {
    message:
      "このビルドソースはネイティブランタイムを使用するため、サービスには独自の開始コマンドが必要です。",
    description:
      "Update Source: why a native-runtime build source needs a start command",
  },
  "services.sourceBuildCommandOptionalHint": {
    message:
      "任意です。空のままにするとリポジトリのファイルがそのまま公開されます。",
    description:
      "Update Source static-site Build Command helper: build command is optional",
  },
  "services.sourcePublishDirectoryRequired": {
    message: "公開ディレクトリを入力してください。",
    description:
      "Update Source static-site Publish Directory validation error when blank",
  },
  "services.sourceDeploy": {
    message: "デプロイ",
    description:
      "Primary submit button in Update Source for Git and Build Source tabs",
  },
  "services.sourceDeploying": {
    message: "デプロイ中",
    description:
      "In-flight submit button label in Update Source for Git and Build Source tabs",
  },
  "services.sourceConnect": {
    message: "接続",
    description:
      "Primary submit button in Update Source for the Existing Image tab",
  },
  "services.sourceConnecting": {
    message: "接続中",
    description:
      "In-flight submit button label in Update Source for the Existing Image tab",
  },
  "services.startCommandLabel": {
    message: "開始コマンド",
    description: "Build & Deploy: native-runtime start-command label",
  },
  "services.dockerCommandLabel": {
    message: "Docker コマンド",
    description: "Build & Deploy: Docker CMD override label (Render wording)",
  },
  "services.startCommandHint": {
    message: "ビルドが成功した後にこのサービスを開始するコマンドです。",
    description: "Build & Deploy: native start-command help text",
  },
  "services.dockerCommandHint": {
    message:
      "Dockerfile の CMD を上書きします。空欄にすると、イメージのデフォルトのコマンドを使用します。",
    description: "Build & Deploy: Docker Command help text",
  },
  "services.startCommandConfirmEmpty": {
    message: "デフォルトのコマンド",
    description: "Build & Deploy: empty command phrase in confirmation title",
  },
  "services.startCommandPlaceholder": {
    message: "例: npm start",
    description: "Build & Deploy: start-command input placeholder",
  },
  "services.startCommandEdit": {
    message: "開始コマンドを編集",
    description: "Build & Deploy: accessible command edit button label",
  },
  "services.dockerCommandEdit": {
    message: "Docker コマンドを編集",
    description: "Build & Deploy: accessible Docker Command edit button label",
  },
  "services.dockerCommandPlaceholder": {
    message: "例: bin/server",
    description: "Build & Deploy: Docker Command input placeholder",
  },
  "services.startCommandConfirmTitle": {
    message: "開始コマンドを {value} に変更しますか？",
    description: "Build & Deploy: command-change confirmation title",
  },
  "services.dockerCommandConfirmTitle": {
    message: "Docker コマンドを {value} に変更しますか？",
    description: "Build & Deploy: Docker Command confirmation title",
  },
  "services.startCommandConfirmBody": {
    message:
      "サービスは新しいコマンドで再デプロイされます。処理中のリクエストは、古いインスタンスが置き換えられる前に完了します。",
    description: "Build & Deploy: command-change confirmation body",
  },
  "services.startCommandConfirmBodyNeverLive": {
    message:
      "サービスは新しいコマンドでデプロイされます。まだ何も配信されておらず、デプロイが成功すると公開されます。",
    description:
      "Deploy: command change confirm body for a service that has never gone live (w4/548)",
  },
  "services.startCommandSuccess": {
    message: "開始コマンドを更新しました。",
    description: "Toast after setStartCommand succeeds",
  },
  "services.startCommandError": {
    message: "開始コマンドを更新できませんでした。もう一度お試しください。",
    description: "Toast after setStartCommand fails",
  },
  "services.dockerCommandSuccess": {
    message: "Docker コマンドを更新しました。",
    description:
      "Toast after a Dockerfile or Existing Image service's Docker Command is saved",
  },
  "services.dockerCommandError": {
    message: "Docker コマンドを更新できませんでした。もう一度お試しください。",
    description:
      "Toast when saving a Dockerfile or Existing Image service's Docker Command fails",
  },
  "services.buildCommandLabel": {
    message: "ビルドコマンド",
    description:
      "Build & Deploy: build-command field label (all service types' settings + the New Service form)",
  },
  "services.buildCommandHint": {
    message:
      "サービスをビルドするコマンドです（例: npm run build）。空欄にすると、ランタイムのデフォルトを使用します。",
    description:
      "Build & Deploy: build-command help text — shown for every service type, so it must not assume static-site output",
  },
  "services.buildCommandConfirmEmpty": {
    message: "ランタイムのデフォルト",
    description:
      "Build & Deploy: empty build-command phrase in confirmation title",
  },
  "services.buildCommandPlaceholder": {
    message: "npm run build",
    description: "Build & Deploy: build-command input placeholder",
  },
  "services.buildCommandEdit": {
    message: "ビルドコマンドを編集",
    description: "Build & Deploy: accessible build-command edit button label",
  },
  "services.buildCommandConfirmTitle": {
    message: "ビルドコマンドを {value} に変更しますか？",
    description: "Build & Deploy: build-command change confirmation title",
  },
  "services.buildCommandConfirmBody": {
    message:
      "サービスは新しいビルドコマンドで再デプロイされます。既存のリクエストは、古いインスタンスが置き換えられる前に処理を完了します。",
    description: "Build & Deploy: build-command change confirmation body",
  },
  "services.buildCommandConfirmBodyNeverLive": {
    message:
      "サービスは新しいビルドコマンドでビルドされ、デプロイされます。まだ何も配信されておらず、デプロイが成功すると公開されます。",
    description:
      "Build & Deploy: build-command change confirm body for a service that has never gone live (w4/548)",
  },
  "services.buildCommandConfirmBodyStatic": {
    message:
      "新しいビルドコマンドで再ビルドを開始します。新しいビルドが正常に公開されるまで、サイトは現在公開中のバージョンを配信し続けます。",
    description:
      "Build & Deploy: static-site build-command change confirmation body (atomic publish, no instances)",
  },
  "services.staticBuildConfirmBodyNeverLive": {
    message:
      "この変更でビルドを開始します。サイトはまだ公開されていません。このビルドが成功すると公開されます。",
    description:
      "Build & Deploy: a static site's build-command or root-directory change confirm body when the site has never been published (w4/548)",
  },
  "services.buildCommandSuccess": {
    message: "ビルドコマンドを更新しました。",
    description: "Toast after setBuildCommand succeeds",
  },
  "services.buildCommandError": {
    message: "ビルドコマンドを更新できませんでした。もう一度お試しください。",
    description: "Toast after setBuildCommand fails",
  },
  "services.dockerfilePathLabel": {
    message: "Dockerfile のパス",
    description: "Build & Deploy: Dockerfile-path field label",
  },
  "services.dockerfilePathHint": {
    message:
      "ルートディレクトリからの相対パスで Dockerfile を指定します。空欄にすると Dockerfile を使用します。",
    description: "Build & Deploy: Dockerfile-path help text",
  },
  "services.dockerfilePathConfirmEmpty": {
    message: "デフォルトの Dockerfile",
    description: "Build & Deploy: empty Dockerfile-path confirmation phrase",
  },
  "services.dockerfilePathPlaceholder": {
    message: "例: docker/Dockerfile.prod",
    description: "Build & Deploy: Dockerfile-path input placeholder",
  },
  "services.dockerfilePathEdit": {
    message: "Dockerfile のパスを編集",
    description: "Build & Deploy: accessible Dockerfile-path edit label",
  },
  "services.dockerfilePathConfirmTitle": {
    message: "Dockerfile のパスを {value} に変更しますか？",
    description: "Build & Deploy: Dockerfile-path confirmation title",
  },
  "services.dockerfilePathConfirmBody": {
    message:
      "サービスは選択した Dockerfile から再ビルドされ、生成されたイメージがデプロイされます。",
    description: "Build & Deploy: Dockerfile-path confirmation body",
  },
  "services.dockerfilePathSuccess": {
    message: "Dockerfile のパスを更新しました。",
    description: "Toast after setDockerfilePath succeeds",
  },
  "services.dockerfilePathError": {
    message:
      "Dockerfile のパスを更新できませんでした。もう一度お試しください。",
    description: "Toast after setDockerfilePath fails",
  },
  "services.dockerContextLabel": {
    message: "Docker ビルドコンテキストのディレクトリ",
    description: "Build & Deploy: Docker build context field label (w8/m128)",
  },
  "services.dockerContextHint": {
    message:
      "ルートディレクトリからの相対パスでビルドコンテキストを指定します。空欄にするとルートディレクトリになります。",
    description: "Build & Deploy: Docker build context help text",
  },
  "services.dockerContextConfirmEmpty": {
    message: "ルートディレクトリ",
    description:
      "Build & Deploy: empty Docker build context confirmation phrase",
  },
  "services.dockerContextPlaceholder": {
    message: "例: .",
    description: "Build & Deploy: Docker build context input placeholder",
  },
  "services.dockerContextEdit": {
    message: "Docker ビルドコンテキストのディレクトリを編集",
    description: "Build & Deploy: accessible Docker build context edit label",
  },
  "services.dockerContextConfirmTitle": {
    message:
      "Docker ビルドコンテキストのディレクトリを {value} に変更しますか？",
    description: "Build & Deploy: Docker build context confirmation title",
  },
  "services.dockerContextConfirmBody": {
    message:
      "サービスは新しいビルドコンテキストで再ビルドされ、生成されたイメージがデプロイされます。",
    description: "Build & Deploy: Docker build context confirmation body",
  },
  "services.dockerContextSuccess": {
    message: "Docker ビルドコンテキストのディレクトリを更新しました。",
    description: "Toast after setDockerContext succeeds",
  },
  "services.dockerContextError": {
    message:
      "Docker ビルドコンテキストのディレクトリを更新できませんでした。もう一度お試しください。",
    description: "Toast after setDockerContext fails",
  },
  "services.buildFilterLabel": {
    message: "ビルドフィルター",
    description: "Build & Deploy: label for the build-filters editor",
  },
  "services.buildFilterHint": {
    message:
      "git push で一致するファイルが変更されたときだけデプロイを開始します。パスは、ルートディレクトリを設定していてもリポジトリのルートからの相対 glob パターンです（例: src/**, **/*.md）。",
    description: "Build & Deploy: help text for the build-filters editor",
  },
  "services.buildFilterIncludedTitle": {
    message: "含めるパス",
    description: "Build & Deploy: title for the included-paths list",
  },
  "services.buildFilterIncludedHint": {
    message:
      "変更されたファイルがいずれかに一致する場合のみ、push でデプロイされます。ルートディレクトリの外側のファイルも同様です。空の場合は、ルートディレクトリ内のすべてのパスが対象です。",
    description: "Build & Deploy: help text for the included-paths list",
  },
  "services.buildFilterIncludedPlaceholder": {
    message: "例: src/**",
    description: "Build & Deploy: placeholder for an included-path input",
  },
  "services.buildFilterAddIncluded": {
    message: "含めるパスを追加",
    description: "Build & Deploy: add-row button for the included-paths list",
  },
  "services.buildFilterRemoveIncluded": {
    message: "含めるパスを削除",
    description: "Build & Deploy: remove-row label for an included path",
  },
  "services.buildFilterIgnoredTitle": {
    message: "無視するパス",
    description: "Build & Deploy: title for the ignored-paths list",
  },
  "services.buildFilterIgnoredHint": {
    message:
      "いずれかに一致する変更ファイルは、含めるパスにも一致していても、デプロイを開始しません。",
    description: "Build & Deploy: help text for the ignored-paths list",
  },
  "services.buildFilterIgnoredPlaceholder": {
    message: "例: docs/**",
    description: "Build & Deploy: placeholder for an ignored-path input",
  },
  "services.buildFilterAddIgnored": {
    message: "無視するパスを追加",
    description: "Build & Deploy: add-row button for the ignored-paths list",
  },
  "services.buildFilterRemoveIgnored": {
    message: "無視するパスを削除",
    description: "Build & Deploy: remove-row label for an ignored path",
  },
  "services.buildPathTooLong": {
    message: "512 バイト以内で入力してください",
    description:
      "Inline error under a Root Directory, Dockerfile Path, Docker Build Context or Build Filter pattern over the server's 512-byte limit; a Chinese character is 3 bytes (w21/045)",
  },
  "services.buildPathControlChar": {
    message: "制御文字やタブは使用できません",
    description:
      "Inline error under a build path or Build Filter pattern with a control character (w21/045)",
  },
  "services.buildPathBackslash": {
    message: "パスの区切りには \\ ではなく / を使用してください",
    description:
      "Inline error under a build path or Build Filter pattern with a backslash (w21/045)",
  },
  "services.buildPathAbsolute": {
    message:
      "リポジトリからの相対パスにしてください。/ で始めることはできません",
    description:
      "Inline error under a build path or Build Filter pattern that starts with / (w21/045)",
  },
  "services.buildPathDotDot": {
    message:
      "リポジトリの外に出ることはできません。.. セグメントは使用できません",
    description:
      "Inline error under a build path or Build Filter pattern with a .. segment (w21/045)",
  },
  "services.buildPathEmptySegment": {
    message: "空のセグメントは使用できません。// や末尾の / は指定できません",
    description:
      "Inline error under a Dockerfile Path or Docker Build Context with an empty path segment; the Root Directory normalizes these away (w21/045)",
  },
  "services.buildFilterUnclosedClass": {
    message: "[ が ] で閉じられていません",
    description:
      "Inline error under a Build Filter glob with an unclosed character class (w21/045)",
  },
  "services.healthCheckPathSlash": {
    message: "/ で始めるか、ポートのみの確認にする場合は空にしてください",
    description:
      "Inline error under a Health Check Path that does not start with / (w21/045)",
  },
  "services.buildFilterSave": {
    message: "ビルドフィルターを保存",
    description: "Build & Deploy: save button for the build-filters editor",
  },
  "services.buildFilterSuccess": {
    message: "ビルドフィルターを更新しました。",
    description: "Toast after setBuildFilter succeeds",
  },
  "services.buildFilterError": {
    message: "ビルドフィルターを更新できませんでした。もう一度お試しください。",
    description: "Toast after setBuildFilter fails",
  },
  "services.preDeployLabel": {
    message: "プレデプロイコマンド",
    description: "Build & Deploy: label for the pre-deploy command field",
  },
  "services.preDeployHint": {
    message:
      "トラフィックを受け付ける前に、新しいイメージに対して一度だけ実行されます（例: データベースのマイグレーション）。ゼロ以外の終了コードでデプロイは失敗し、以前のバージョンが公開されたままになります。",
    description: "Build & Deploy: help text for the pre-deploy command field",
  },
  "services.preDeployPaidOnly": {
    message: "プレデプロイコマンドは有料プランで利用できます。",
    description:
      "Build & Deploy: why the pre-deploy command field is disabled on a Free service (Render: paid web services, private services and background workers; w8/m131)",
  },
  "services.preDeployPlaceholder": {
    message: "例: npm run migrate",
    description: "Build & Deploy: placeholder for the pre-deploy command input",
  },
  "services.preDeployEdit": {
    message: "プレデプロイコマンドを編集",
    description:
      "Build & Deploy: accessible label for the pre-deploy edit-pencil button",
  },
  "services.healthCheckPathConfirmTitle": {
    message: "ヘルスチェックパスを {value} に変更しますか？",
    description:
      "Settings: Health Check Path change confirmation title (w4/616)",
  },
  "services.healthCheckPathConfirmEmpty": {
    message: "TCP チェック",
    description:
      "Settings: empty Health Check Path phrase in its confirmation title",
  },
  "services.maxShutdownDelayConfirmTitle": {
    message: "最大シャットダウン遅延を {value} 秒に変更しますか？",
    description:
      "Settings: Max shutdown delay change confirmation title (w4/616)",
  },
  "services.preDeployConfirmTitle": {
    message: "プレデプロイコマンドを {value} に変更しますか？",
    description:
      "Settings: Pre-Deploy Command change confirmation title (w4/616)",
  },
  "services.preDeployConfirmEmpty": {
    message: "プレデプロイコマンドなし",
    description:
      "Settings: empty Pre-Deploy Command phrase in its confirmation title",
  },
  "services.settingRedeployConfirmBody": {
    message:
      "サービスは新しい設定で再デプロイされます。処理中のリクエストは、古いインスタンスが置き換えられる前に完了します。",
    description:
      "Settings: confirmation body for a setting whose save starts a deploy (w4/616)",
  },
  "services.settingRedeployConfirmBodyNeverLive": {
    message:
      "サービスは新しい設定でデプロイされます。まだ何も配信されておらず、デプロイが成功すると公開されます。",
    description:
      "Settings: deploying-setting confirmation body for a service that has never gone live (w4/616)",
  },
  "services.preDeployConfirmBody": {
    message:
      "サービスは再デプロイされ、新しいリリースが公開される前に新しいコマンドが実行されます。処理中のリクエストは、古いインスタンスが置き換えられる前に完了します。",
    description:
      "Settings: Pre-Deploy Command change confirmation body (w4/616)",
  },
  "services.preDeployConfirmBodyNeverLive": {
    message:
      "サービスはデプロイされ、リリースが公開される前に新しいコマンドが実行されます。まだ何も配信されておらず、デプロイが成功すると公開されます。",
    description:
      "Settings: Pre-Deploy Command confirmation body for a service that has never gone live (w4/616)",
  },
  "services.preDeploySuccess": {
    message:
      "プレデプロイコマンドを更新しました。新しいデプロイが開始されました。",
    description: "Toast after setPreDeployCommand succeeds",
  },
  "services.preDeployError": {
    message:
      "プレデプロイコマンドを更新できませんでした。もう一度お試しください。",
    description: "Toast after setPreDeployCommand fails",
  },
  "services.autoDeployLabel": {
    message: "自動デプロイ",
    description: "Build & Deploy: label for the auto-deploy toggle",
  },
  "services.autoDeployOnCommit": {
    message: "コミット時",
    description:
      "Auto-Deploy select option: redeploy on every matching git push (Render's autoDeployTrigger 'commit', w5/m53)",
  },
  "services.autoDeployAfterChecks": {
    message: "CI チェックの成功後",
    description:
      "Auto-Deploy select option: redeploy a pushed commit once all its GitHub checks pass (Render's autoDeployTrigger 'checksPass', w1/m181)",
  },
  "services.autoDeployOff": {
    message: "オフ",
    description:
      "Auto-Deploy select option: never redeploy on push (Render's autoDeployTrigger 'off')",
  },
  "services.autoDeployEdit": {
    message: "自動デプロイを編集",
    description:
      "Accessible label for the Auto-Deploy select's edit-pencil button",
  },
  "services.autoDeployViaGitHub": {
    message:
      "追跡中のブランチへの push があると、GitHub アプリ経由で自動的に再デプロイされます。",
    description:
      "Build & Deploy: source indicator when the connected GitHub App installation grants THIS repo (pushDeliveryMethod github_app)",
  },
  "services.autoDeployViaWebhook": {
    message:
      "リポジトリの手動 git Webhook にこのインストールの BEX_WEBHOOK_SECRET が設定されている場合のみ、push で再デプロイされます。",
    description:
      "Build & Deploy: source indicator when the GitHub App cannot deliver a push for this repo — no connection, another git host, or a repo the installation does not grant (pushDeliveryMethod manual_webhook)",
  },
  "services.autoDeployUnavailable": {
    message:
      "このリポジトリでは自動デプロイを実行できません。GitHub が push を bex に送信していません。GitHub アプリでリポジトリを連携するか、Deploy Hook でデプロイを開始してください。",
    description:
      "Build & Deploy: source indicator when no push can reach bex for this repo — the GitHub App does not cover it and this install rejects the shared manual webhook key (pushDeliveryMethod unavailable, w4/483)",
  },
  "services.autoDeployDeliveryUnknown": {
    message:
      "追跡中のブランチへの push が bex にどう届くかを確認しています…。この状態が続く場合は、GitHub に接続できず、配信方法を確認できません。",
    description:
      "Build & Deploy: source indicator while the per-repo push-delivery check is loading or returned unknown (GitHub unreachable) — states the uncertainty instead of naming a mechanism (w6/m99)",
  },
  "services.autoDeployOnSuccess": {
    message: "自動デプロイをオンにしました。",
    description: "Toast after enabling auto-deploy",
  },
  "services.autoDeployChecksSuccess": {
    message: "自動デプロイは CI チェックの成功を待つようになりました。",
    description: "Toast after choosing After CI Checks Pass",
  },
  "services.autoDeployOffSuccess": {
    message: "自動デプロイをオフにしました。",
    description: "Toast after disabling auto-deploy",
  },
  "services.autoDeployError": {
    message: "自動デプロイを変更できませんでした。もう一度お試しください。",
    description: "Toast after setAutoDeploy fails",
  },
  "services.deployHookTitle": {
    message: "Deploy Hook",
    description: "Settings tab: secret Deploy Hook section title",
  },
  "services.deployHookDescription": {
    message: "1 つのシークレット URL で、CI からデプロイを開始します。",
    description: "Settings tab: Deploy Hook section description",
  },
  "services.deployHookURLLabel": {
    message: "Deploy Hook の URL",
    description: "Accessible label for the secret Deploy Hook URL field",
  },
  "services.deployHookReveal": {
    message: "Deploy Hook の URL を表示",
    description: "Accessible label for the reveal-secret button",
  },
  "services.deployHookHide": {
    message: "Deploy Hook の URL を非表示",
    description: "Accessible label for the hide-secret button",
  },
  "services.deployHookCopy": {
    message: "Deploy Hook の URL をコピー",
    description: "Accessible label for the Deploy Hook copy button",
  },
  "services.deployHookCopied": {
    message: "Deploy Hook の URL をコピーしました。",
    description: "Toast after copying the Deploy Hook URL",
  },
  "services.deployHookCopyError": {
    message: "Deploy Hook の URL をコピーできませんでした。",
    description: "Toast after Deploy Hook URL clipboard failure",
  },
  "services.deployHookSecretHint": {
    message:
      "この URL は秘密にしてください。URL を知っている人は誰でも、API キーなしでこのサービスをデプロイできます。",
    description: "Security warning below the Deploy Hook URL",
  },
  "services.deployHookRegenerate": {
    message: "フックを再生成",
    description: "Deploy Hook rotation button",
  },
  "services.deployHookRegenerateTitle": {
    message: "Deploy Hook を再生成しますか？",
    description: "Deploy Hook rotation confirmation title",
  },
  "services.deployHookRegenerateWarning": {
    message:
      "現在の URL はすぐに使用できなくなります。この URL を使用しているすべての CI システム、cron ジョブ、連携を更新してください。",
    description: "Deploy Hook rotation confirmation warning",
  },
  "services.deployHookCancel": {
    message: "キャンセル",
    description: "Deploy Hook rotation confirmation cancel button",
  },
  "services.deployHookRegenerateConfirm": {
    message: "再生成",
    description: "Deploy Hook rotation confirmation action",
  },
  "services.deployHookRegenerated": {
    message:
      "Deploy Hook を再生成しました。古い URL は使用できなくなりました。",
    description: "Toast after successful Deploy Hook rotation",
  },
  "services.deployHookRegenerateError": {
    message: "Deploy Hook を再生成できませんでした。もう一度お試しください。",
    description: "Toast after Deploy Hook rotation fails",
  },
  "services.deployHookLoadError": {
    message: "Deploy Hook の URL を読み込めませんでした。",
    description: "Deploy Hook section query error",
  },
  "services.colType": {
    message: "タイプ",
    description: "Services table column header (service type)",
  },
  "services.typeWeb": {
    message: "Web Service",
    description: "Service-type badge: an HTTP service exposed at a URL",
  },
  "services.typePrivate": {
    message: "Private Service",
    description:
      "Service-type badge: an HTTP service reachable only in-cluster",
  },
  "services.typeWorker": {
    message: "Background Worker",
    description: "Service-type badge: runs with no HTTP port/URL",
  },
  "services.typeCron": {
    message: "Cron Job",
    description: "Service-type badge: runs a command on a schedule",
  },
  "services.typeStatic": {
    message: "Static Site",
    description:
      "Service-type badge: built output served from the object-store origin",
  },
  "services.staticTitle": {
    message: "Static Site",
    description:
      "Settings section title for a static site's publish dir + edge rules",
  },
  "services.staticDescription": {
    message:
      "公開される出力ディレクトリと、配信時に適用されるエッジルールです。",
    description: "Static Site settings section description",
  },
  "services.staticEdgeRulesHint": {
    message:
      "リダイレクト/リライトとレスポンスヘッダーには、それぞれ専用のページがあります:",
    description:
      "Settings hint pointing at the dedicated Redirects/Rewrites + Headers pages",
  },
  "services.staticCancel": {
    message: "キャンセル",
    description: "Cancel an inline edit",
  },
  "services.publishPathEscapes": {
    message:
      "このパスはリポジトリの外に出ています。公開ディレクトリはルートディレクトリを基準に解決されるため、.. セグメントを削除してください。",
    description:
      "Inline error under a repo-backed static site's publish directory whose .. segments leave the checkout (w21/050)",
  },
  "services.publishPathControlChar": {
    message: "制御文字や改行は使用できません",
    description:
      "Inline error under a static site's publish directory with a control character (w21/050)",
  },
  "services.publishPathLabel": {
    message: "公開ディレクトリ",
    description: "Label for a static site's publishPath",
  },
  "services.publishPathPlaceholder": {
    message: "dist",
    description: "Placeholder for the publish directory input",
  },
  "services.publishPathEdit": {
    message: "公開ディレクトリを編集",
    description:
      "Accessible label for the publish-directory edit-pencil button",
  },
  "services.publishPathHint": {
    message:
      "サイトのルートとして配信されるビルド出力ディレクトリです（例: dist, build, public）。変更するとサイトが再公開されます。",
    description: "Help text under the publish directory field",
  },
  "services.publishPathSaved": {
    message: "公開ディレクトリを更新しました",
    description: "Toast after saving a static site's publishPath",
  },
  "services.publishPathRepublishNote": {
    message: "まもなくサイトが再公開されます。",
    description: "Toast description after changing publishPath",
  },
  "services.publishPathError": {
    message: "公開ディレクトリを更新できませんでした",
    description: "Error toast for a failed publishPath change",
  },
  "services.routesTitle": {
    message: "リダイレクトとリライト",
    description: "Title for the static-site routes editor",
  },
  "services.routesHint": {
    message:
      "順番に照合され、最初に一致したものが適用されます。リダイレクトは 301 を返し、リライトは別のパスを配信します（SPA のフォールバックは /* を /index.html にリライトします）。:name セグメントは 1 つのパスセグメントを取得します。例: /blog/posts/:postid → /blog/:postid",
    description: "Help text for the routes editor",
  },
  "services.routeAdd": {
    message: "ルールを追加",
    description: "Add a route rule",
  },
  "services.routeType": {
    message: "タイプ",
    description: "Route type column",
  },
  "services.routeSource": {
    message: "ソース",
    description: "Route source-path column",
  },
  "services.routeDestination": {
    message: "宛先",
    description: "Route destination-path column",
  },
  "services.routeRewrite": {
    message: "リライト",
    description: "Route type: rewrite",
  },
  "services.routeRedirect": {
    message: "リダイレクト",
    description: "Route type: redirect",
  },
  "services.routeTypeAria": {
    message: "ルール {n} のタイプ",
    description:
      "A route rule's type select (aria-label), named by its row as Remove rule {n} is (w4/541)",
  },
  "services.routeSourceAria": {
    message: "ルール {n} のソース",
    description:
      "A route rule's source field (aria-label), named by its row (w4/541)",
  },
  "services.routeDestinationAria": {
    message: "ルール {n} の宛先",
    description:
      "A route rule's destination field (aria-label), named by its row (w4/541)",
  },
  "services.routeRemove": {
    message: "ルール {n} を削除",
    description: "Remove a route rule (aria-label)",
  },
  "services.routesSave": {
    message: "ルートを保存",
    description: "Save the routes list",
  },
  "services.staticRuleRequired": {
    message: "必須",
    description: "Inline error under an empty static-site rule field (w4/145)",
  },
  "services.staticRulePathSlash": {
    message: "/ で始まるパスにしてください",
    description:
      "Inline error under a static-site rule path that doesn't start with / (w4/145)",
  },
  "services.staticRuleExternalHost": {
    message:
      "固定ホストを持つ通常の http(s) URL を使用してください。例: https://example.org/new",
    description:
      "Inline error under a redirect destination that is an absolute URL without a literal host (w8/162)",
  },
  "services.staticRuleLocalPath": {
    message: "//host ではなく、このサイト内のパスにしてください",
    description:
      "Inline error under a redirect/rewrite destination that is a network-path reference (w4/145)",
  },
  "services.staticRuleDestinationPathOrUrl": {
    message: "パスまたは http(s) URL にしてください",
    description:
      "Inline error under a redirect/rewrite destination whose first segment reads as a scheme other than http(s), such as javascript: (w33/007)",
  },
  "services.staticRuleFixRows": {
    message: "保存するには、マークされた行を修正してください。",
    description:
      "Note beside a disabled Save while a static-site rule row is invalid (w4/145)",
  },
  "services.staticRuleEmptyRow": {
    message: "保存するには、空の行を入力するか削除してください。",
    description:
      "Note beside a disabled Save while a static-site rule row is still completely empty (w4/145)",
  },
  "services.staticRuleRow": {
    message: "{row} 行目:",
    description:
      "Prefix naming the 1-based rule row in a server refusal, replacing its zero-based wire index (w4/145)",
  },
  "services.staticRulePathTooLong": {
    message: "2048 バイト以内で入力してください",
    description:
      "Inline error under an over-long static-site rule path; the server counts UTF-8 bytes, so a Chinese character is 3 (w4/402, w21/043)",
  },
  "services.staticRuleSourceRepeatsPlaceholder": {
    message: "各 :name はソース内で 1 回だけ使用できます",
    description:
      "Inline error under a redirect/rewrite source that declares the same :name placeholder twice (w21/043)",
  },
  "services.staticRuleUndeclaredPlaceholder": {
    message: "ソースで宣言されていない :name を使用しています",
    description:
      "Inline error under a redirect/rewrite destination whose :name placeholder (other than :splat) the source never declares (w21/043)",
  },
  "services.staticRuleHeaderNameTooLong": {
    message: "256 文字以内で入力してください",
    description: "Inline error under an over-long header name (w4/402)",
  },
  "services.staticRuleHeaderNameInvalid": {
    message: "有効な HTTP ヘッダー名ではありません",
    description:
      "Inline error under a header name with characters a header name can't hold (w4/402)",
  },
  "services.staticRuleHeaderNameReserved": {
    message: "このヘッダーはサーバーが設定するため、上書きできません",
    description:
      "Inline error under a reserved header name such as Content-Length (w4/402)",
  },
  "services.staticRuleHeaderValueTooLong": {
    message: "4096 バイト以内で入力してください",
    description: "Inline error under an over-long header value (w4/402)",
  },
  "services.staticRuleHeaderValueInvalid": {
    message: "ヘッダー値に使用できない文字が含まれています",
    description:
      "Inline error under a header value with a control character (w4/402)",
  },
  "services.staticRuleCookieDomain": {
    message: "共有ホスティングでは、Set-Cookie で Domain を設定できません",
    description:
      "Inline error under a Set-Cookie value carrying a Domain attribute (w4/402)",
  },
  "services.staticRoutesSaved": {
    message: "ルートを更新しました",
    description: "Toast after saving routes",
  },
  "services.staticRoutesError": {
    message: "ルートを更新できませんでした",
    description: "Error toast for a failed routes save",
  },
  "services.headersTitle": {
    message: "レスポンスヘッダー",
    description: "Title for the static-site custom-headers editor",
  },
  "services.headersHint": {
    message:
      "パスが一致するレスポンスに、カスタムのレスポンスヘッダーを追加します。Content-Length などのボディのフレーミングや接続に関するヘッダーはサーバーが設定するため、上書きできません。Content-Encoding は、実際に圧縮して保存されているファイルにのみ設定してください。",
    description: "Help text for the headers editor",
  },
  "services.headerAdd": {
    message: "ヘッダーを追加",
    description: "Add a custom header",
  },
  "services.headerPath": {
    message: "パス",
    description: "Header path column",
  },
  "services.headerName": {
    message: "名前",
    description: "Header name column",
  },
  "services.headerValue": {
    message: "値",
    description: "Header value column",
  },
  "services.headerPathAria": {
    message: "ヘッダー {n} のパス",
    description:
      "A header rule's path field (aria-label), named by its row as Remove header {n} is (w4/541)",
  },
  "services.headerNameAria": {
    message: "ヘッダー {n} の名前",
    description:
      "A header rule's name field (aria-label), named by its row (w4/541)",
  },
  "services.headerValueAria": {
    message: "ヘッダー {n} の値",
    description:
      "A header rule's value field (aria-label), named by its row (w4/541)",
  },
  "services.headerRemove": {
    message: "ヘッダー {n} を削除",
    description: "Remove a header (aria-label)",
  },
  "services.headersSave": {
    message: "ヘッダーを保存",
    description: "Save the headers list",
  },
  "services.staticHeadersSaved": {
    message: "ヘッダーを更新しました",
    description: "Toast after saving headers",
  },
  "services.staticHeadersError": {
    message: "ヘッダーを更新できませんでした",
    description: "Error toast for a failed headers save",
  },
  "services.typeUnknown": {
    message: "サービス",
    description: "Service-type badge fallback for an unrecognized type",
  },
  "services.cronRunColDuration": {
    message: "所要時間",
    description:
      "Cron Runs page: the Duration column's header, and a row's duration for assistive tech (Render's DURATION, w3/m556)",
  },
  "services.cronRunStatusSucceeded": {
    message: "実行成功",
    description:
      "Cron Runs page: a successful run's status, in its icon's tooltip (Render's text, w3/m556)",
  },
  "services.cronRunStatusFailed": {
    message: "実行失敗",
    description:
      "Cron Runs page: a failed run's status, in its icon's tooltip (Render's text, w3/m556)",
  },
  "services.cronRunStatusCanceled": {
    message: "キャンセル済み",
    description:
      "Cron Runs page: a canceled run's status in its icon's tooltip, and its line when no one is named as canceling it (Render's text, w3/m556)",
  },
  "services.cronRunStatusInProgress": {
    message: "進行中",
    description:
      "Cron Runs page: the status of a run that has not ended, started or not, in its icon's tooltip (Render's text, w3/m556)",
  },
  "services.cronRunStatusLastSuccessful": {
    message: "最後に成功した実行",
    description:
      "Cron Runs page: the newest successful run's status, in its icon's tooltip (Render's text, w3/m556)",
  },
  "services.cronRunCancel": {
    message: "キャンセル",
    description:
      "Cron Runs page: a run in progress's Cancel button, which cancels it at once (Render's text, w3/m556)",
  },
  "services.cronRunCancelError": {
    message: "Cron の実行をキャンセルできませんでした。",
    description: "Toast after cron run cancellation fails",
  },
  "services.cronTriggerRun": {
    message: "実行をトリガー",
    description: "Button that fires an immediate cron run (Render parity)",
  },
  "services.cronTriggerError": {
    message:
      "実行をトリガーできませんでした。しばらくしてからもう一度お試しください。",
    description:
      "Toast, kept until dismissed, when Trigger Run fails and the server gave no message (Render's text, w3/m555)",
  },
  "services.cronTriggerForbidden": {
    message: "この Cron Job を実行する権限がありません。",
    description:
      "Toast, kept until dismissed, when Trigger Run is refused for permission (Render's text, w3/m555)",
  },
  "services.cronTriggerNoBuild": {
    message: "成功したビルドが見つかりません。{deploys}を確認してください。",
    description:
      "Tooltip of a disabled Trigger Run on a repo-backed cron job whose build never succeeded; {deploys} is a link to its Deploys page, named by services.navDeploys (Render's text, w3/m555)",
  },
  "services.cronTriggerSuspended": {
    message: "実行をトリガーするには、この Cron Job を再開してください。",
    description:
      "Tooltip of a disabled Trigger Run while the cron job is suspended, shown in the Runs page's empty states (Render's text, w3/m556)",
  },
  "services.cronRunsLoadMore": {
    message: "さらに読み込む",
    description: "Cron run history pagination button",
  },
  "services.cronRunsLoadingMore": {
    message: "読み込み中…",
    description: "Cron run history pagination busy label",
  },
  "services.cronRunsLoadError": {
    message: "Cron の実行履歴を読み込めませんでした。",
    description: "Cron run history read error",
  },
  "services.cronRunsLoadFailed": {
    message:
      "申し訳ありません。この Cron Job の実行を読み込めませんでした。ページを更新してもう一度お試しください。",
    description:
      "Cron Runs page: the warning shown in place of the list when the runs could not be read (Render's text, w3/m556)",
  },
  "services.cronRunsSearchPlaceholder": {
    message: "Cron Job の実行を検索",
    description:
      "Cron Runs page: the search field's placeholder and name (Render's text, w3/m556)",
  },
  "services.cronRunsSearchClear": {
    message: "検索をクリア",
    description:
      "Cron Runs page: the search field's clear button, for assistive tech (w3/m556)",
  },
  "services.cronRunsColRun": {
    message: "実行",
    description:
      "Cron Runs page: the list's first column header, before the count (Render's RUN, w3/m556)",
  },
  "services.cronRunsColTrigger": {
    message: "トリガー",
    description:
      "Cron Runs page: the Trigger column's header, and a row's trigger for assistive tech (Render's TRIGGER, w3/m556)",
  },
  "services.cronRunTitle": {
    message: "Cron Job の実行",
    description: "Cron Runs page: a run row's title (Render's text, w3/m556)",
  },
  "services.cronRunStartedAgo": {
    message: "{timestamp} に実行",
    description:
      'Cron Runs page: when a run started, under its title; {timestamp} is elapsed text like "2 hours ago" (Render\'s "Run 21h ago", w3/m556)',
  },
  "services.cronRunBy": {
    message: "{user} が実行",
    description:
      'Cron Runs page: after when a run started, the email of the user who triggered it (Render\'s "by <user>", w3/m556)',
  },
  "services.cronRunByYou": {
    message: "あなたが実行",
    description:
      'Cron Runs page: after when a run started, when the viewer triggered it (Render\'s "by you", w3/m556)',
  },
  "services.cronRunCanceledBy": {
    message: "{user} がキャンセル",
    description:
      "Cron Runs page: a canceled run's line naming the email of the user who canceled it (Render's text, w3/m556)",
  },
  "services.cronRunCanceledByYou": {
    message: "あなたがキャンセル",
    description:
      "Cron Runs page: a canceled run's line when the viewer canceled it (Render's \"Canceled by you\", w3/m556)",
  },
  "services.cronRunTriggerScheduled": {
    message: "スケジュール",
    description:
      "Cron Runs page: the trigger of a run the schedule started (Render's text, w3/m556)",
  },
  "services.cronRunTriggerManual": {
    message: "手動",
    description:
      "Cron Runs page: the trigger of a run a user started (Render's text, w3/m556)",
  },
  "services.cronRunsNoMatchTitle": {
    message: "一致する結果はありません",
    description:
      "Cron Runs page: title when the search matches none of the loaded runs (Render's text, w3/m556)",
  },
  "services.cronRunsNoMatchBody": {
    message:
      "お探しのものが見つかりませんか？検索語を変えるか、入力ミスがないか確認してください。",
    description:
      "Cron Runs page: body when the search matches none of the loaded runs (Render's text, w3/m556)",
  },
  "services.cronRunsResetSearch": {
    message: "検索をリセット",
    description:
      "Cron Runs page: the button that clears a search that matched nothing (Render's text, w3/m556)",
  },
  "services.cronRunsEmptyRecentTitle": {
    message: "この Cron Job は過去 90 日間実行されていません。",
    description:
      "Cron Runs page: title when a cron job whose build succeeded lists no runs (Render's text, w3/m556)",
  },
  "services.cronRunsEmptyRecentBody": {
    message:
      "手動で実行をトリガーするか、次のスケジュール実行を待ってください。",
    description:
      "Cron Runs page: body when a cron job whose build succeeded lists no runs, above Trigger Run (Render's text, w3/m556)",
  },
  "services.cronRunsEmptyNeverTitle": {
    message: "この Cron Job はまだ実行されていません。",
    description:
      "Cron Runs page: title when a cron job whose build never succeeded has no runs (Render's text, w3/m556)",
  },
  "services.cronRunsEmptyNeverBody": {
    message:
      "最初のビルドが完了すると実行されます。手動で実行をトリガーすることもできます。",
    description:
      "Cron Runs page: body when a cron job whose build never succeeded has no runs, above Trigger Run (Render's text, w3/m556)",
  },
  "services.cronRunsCount_other": {
    message: "{count} 件の実行",
    description:
      "Cron Runs page: the list's count, for assistive tech, beside RUN (Render's \"30 runs\", w3/m556)",
  },
  "services.suspendCardTitle": {
    message: "サービスを一時停止",
    description: "Settings tab suspend section title",
  },
  "services.suspendCardDescription": {
    message:
      "サービスを一時停止すると、サービスがシャットダウンし、トラフィックの処理を停止します。URL と証明書は保持され、いつでも再開できます。",
    description:
      "Settings tab suspend section description, for the types served at a public host",
  },
  "services.suspendCardDescriptionNoUrl": {
    message:
      "サービスを一時停止すると、サービスがシャットダウンし、実行を停止します。いつでも再開できます。",
    description:
      "Settings tab suspend section description, for private services and background workers — neither has a public URL or certificates to keep",
  },
  "services.resumeCardTitle": {
    message: "サービスを再開",
    description:
      "Settings tab resume section title (shown when service is suspended)",
  },
  "services.resumeCardDescription": {
    message:
      "サービスを再開すると、再びオンラインになり、トラフィックの処理を再開します。",
    description: "Settings tab resume section description",
  },
  "services.resumeCardDescriptionCrashLoop": {
    message:
      "このサービスは、24 時間にわたって起動と正常化に繰り返し失敗したため、一時停止されました。設定の誤り、アプリケーションコードのバグ、コンピュートプランのサイズ不足が原因であることがよくあります。原因を修正してから再開して、オンラインに戻してください。",
    description:
      "Settings tab resume section description for a service the platform suspended for a crash loop (stuck_crashlooping)",
  },
  "services.resumeCardDescriptionCron": {
    message: "この Cron Job を再開すると、スケジュール実行が再開されます。",
    description: "Settings tab resume section description for a cron job",
  },
  "services.dangerZoneTitle": {
    message: "危険ゾーン",
    description: "Settings tab delete section title (destructive)",
  },
  "services.dangerZoneDescription": {
    message:
      "サービスを削除すると、サービス、そのデプロイ、URL が完全に削除されます。この操作は元に戻せません。",
    description:
      "Settings tab delete section description, for the types served at a public host",
  },
  "services.dangerZoneDescriptionNoUrl": {
    message:
      "サービスを削除すると、サービスとそのデプロイが完全に削除されます。この操作は元に戻せません。",
    description:
      "Settings tab delete section description, for private services, background workers and cron jobs — none of which have a URL to remove",
  },
  "services.deleteButton": {
    message: "サービスを削除",
    description: "Danger-zone button that opens the delete-confirm dialog",
  },
  "services.deleteConfirmTitle": {
    message: "{name} を削除しますか？",
    description: "Delete-confirm dialog title",
  },
  "services.deleteConfirmBody": {
    message:
      "この操作は元に戻せません。この {type} を削除してもよろしいですか？",
    description:
      "Delete-confirm dialog body ({type} = lowercase Render type words, e.g. 'web service')",
  },
  "services.deleteConfirmPrompt": {
    message: "確認のため、下に {phrase} と入力してください。",
    description:
      "Body prompt naming Render's exact 'sudo delete <type> <name>' phrase (rendered bold by SudoCommandField)",
  },
  "services.deleteCancel": {
    message: "キャンセル",
    description: "Delete-confirm dialog cancel button",
  },
  "services.deleteConfirm": {
    message: "サービスを削除",
    description:
      "Delete-confirm dialog submit button (armed once the name matches)",
  },
  "services.deleteSuccess": {
    message: "{name} を削除しました",
    description: "Toast on a successful service delete",
  },
  "services.deleteError": {
    message: "{name} を削除できませんでした",
    description: "Toast on a failed service delete",
  },
  "services.newServiceButton": {
    message: "新しいサービス",
    description:
      "Button on the services list page that opens the create wizard",
  },
  "services.createTitle": {
    message: "新しいサービス",
    description: "Create-wizard page title",
  },
  "services.createWebTitle": {
    message: "新しい Web Service",
    description:
      "Create-wizard page title when the web_service type is selected",
  },
  "services.createWebDescription": {
    message:
      "Git リポジトリまたは Docker イメージから Web Service をデプロイします。",
    description:
      "Create-wizard page subtitle when the web_service type is selected",
  },
  "services.createPrivateTitle": {
    message: "新しい Private Service",
    description:
      "Create-wizard page title when the private_service type is selected",
  },
  "services.createPrivateDescription": {
    message:
      "Git リポジトリまたは Docker イメージから Private Service をデプロイします。",
    description:
      "Create-wizard page subtitle when the private_service type is selected",
  },
  "services.createWorkerTitle": {
    message: "新しい Background Worker",
    description:
      "Create-wizard page title when the background_worker type is selected",
  },
  "services.createWorkerDescription": {
    message:
      "Git リポジトリまたは Docker イメージから Background Worker をデプロイします。",
    description:
      "Create-wizard page subtitle when the background_worker type is selected",
  },
  "services.createStaticTitle": {
    message: "新しい Static Site",
    description:
      "Create-wizard page title when the static_site type is selected",
  },
  "services.createStaticDescription": {
    message: "Git リポジトリから Static Site をビルドしてデプロイします。",
    description:
      "Create-wizard page subtitle when the static_site type is selected",
  },
  "services.createCronTitle": {
    message: "新しい Cron Job",
    description: "Create-wizard page title when the cron_job type is selected",
  },
  "services.createCronDescription": {
    message: "コマンドを定期的なスケジュールで実行します。",
    description:
      "Create-wizard page subtitle when the cron_job type is selected",
  },
  "services.createSourceTitle": {
    message: "ソースコード",
    description: "Create-wizard source-picker section label",
  },
  "services.createTabGitHub": {
    message: "GitHub",
    description: "Create-wizard source-tab label for connected GitHub repos",
  },
  "services.createTabPublicGit": {
    message: "公開 Git URL",
    description: "Create-wizard source-tab label for a public git URL",
  },
  "services.createTabImage": {
    message: "既存のイメージ",
    description: "Create-wizard source-tab label for a pre-built Docker image",
  },
  "services.createRepoSearchShort": {
    message: "検索…",
    description:
      "Repository search placeholder where the field is narrow (a phone); the field keeps its full accessible name",
  },
  "services.createRepoCurrent": {
    message: "現在",
    description:
      "Update Source repository list: tag on the repository the service uses now, shown once another one is chosen",
  },
  "services.createRepoSearchPlaceholder": {
    message: "リポジトリを検索…",
    description: "Create-wizard GitHub tab repo-search input placeholder",
  },
  "services.createRepoPrivateBadge": {
    message: "プライベート",
    description: "Badge on a private GitHub repo row in the repo picker",
  },
  "services.createRepoEmpty": {
    message: "リポジトリが見つかりません。",
    description:
      "Create-wizard GitHub tab empty state (no repos in the installation)",
  },
  "services.createRepoNoMatch": {
    message: "検索に一致するリポジトリがありません。",
    description:
      "Create-wizard GitHub tab empty state when the search filter returns nothing",
  },
  "services.createRepoEmptyHint": {
    message:
      "GitHub で bex にリポジトリへのアクセスを許可するか、別のアカウントを接続してください。",
    description:
      "Source picker GitHub tab, under the empty state: what to do when the connected accounts grant no repository. Credentials is the menu beside the search field",
  },
  "services.createRepoLoadError": {
    message: "リポジトリを読み込めませんでした。",
    description:
      "Source picker GitHub tab: the repository list could not be read, shown with a Try again button. Not the same as having no repositories",
  },
  "services.createRepoListLabel": {
    message: "リポジトリ",
    description:
      "Accessible name of the repository list in the source picker's GitHub tab",
  },
  "services.createRepoClearSearch": {
    message: "検索をクリア",
    description:
      "Source picker GitHub tab: button under the no-match message that empties the repository search field",
  },
  "services.sourceNoChanges": {
    message: "保存する変更はありません。",
    description:
      "Update Source dialog footer: why Update source is disabled when the dialog still shows the service's current source",
  },
  "services.sourceSelectRepo": {
    message: "リポジトリを選択してください。",
    description:
      "Update Source dialog footer: why Update source is disabled on the GitHub tab before a repository is picked",
  },
  "services.sourceSelectBuildSource": {
    message: "ビルドソースを選択してください。",
    description:
      "Update Source dialog footer: why Update source is disabled on the Build source tab before one is picked",
  },
  "services.sourceEnterRepoUrl": {
    message: "リポジトリの URL を入力してください。",
    description:
      "Update Source dialog footer: why Update source is disabled on the Public Git URL tab while the URL field is empty",
  },
  "services.sourceEnterImage": {
    message: "イメージを入力してください。",
    description:
      "Update Source dialog footer: why Update source is disabled on the Existing Image tab while the image field is empty",
  },
  "services.createRepoOpenCredentials": {
    message: "認証情報を開く",
    description:
      "Source picker GitHub tab: button under the empty repository list that opens the Credentials menu beside the search field",
  },
  "services.sourcePending": {
    message: "新しいソース: {source}",
    description:
      "Update Source dialog footer: names what Update source will save. {source} is a repository and branch, an image, or a build source's name",
  },
  "services.sourceBranchLocked": {
    message: "リポジトリを選ぶと使えます。",
    description:
      "Update Source dialog: helper under the disabled Branch field before a repository is picked or a valid URL entered",
  },
  "services.sourceNoBuildSources": {
    message: "選択できるビルドソースがありません。",
    description:
      "Update Source dialog footer: why Update source is disabled on the Build source tab when the workspace has no build sources",
  },
  "services.sourceFixField": {
    message: "続けるには、上の項目を入力してください。",
    description:
      "Update Source dialog footer: why Update source is disabled while a field is malformed (its own message is under it) or a required one is empty",
  },
  "services.createRepoNoMatchHint": {
    message:
      "一覧にありませんか？GitHub で bex にアクセスを許可するか、別のアカウントを接続してください。",
    description:
      "Source picker GitHub tab, under the no-match message: the usual cause is a repository the connected accounts do not grant. Shown with Clear search and Open Credentials buttons",
  },
  "services.sourceBranchLockedUrl": {
    message: "リポジトリの URL を入力すると使えます。",
    description:
      "Update Source dialog, Public Git URL tab: helper under the disabled Branch field before a valid URL is entered",
  },
  "services.createGitConnectPromptTitle": {
    message: "GitHub に接続",
    description:
      "Create-wizard GitHub tab connect-prompt heading (no GitHub connection yet)",
  },
  "services.createGitConnectPromptBody": {
    message:
      "GitHub アカウントを接続すると、プライベートまたは公開リポジトリからデプロイできます。",
    description:
      "Create-wizard GitHub tab connect-prompt body (no GitHub connection yet)",
  },
  "services.createPublicUrlLabel": {
    message: "リポジトリ URL",
    description: "Create-wizard Public Git URL tab input label",
  },
  "services.createPublicUrlPlaceholder": {
    message: "https://github.com/you/your-repo",
    description: "Create-wizard Public Git URL tab input placeholder",
  },
  "services.createPublicUrlError": {
    message:
      "https:// の URL、または GitHub の SSH URL（git@github.com:owner/repo.git）を入力してください。",
    description: "Create-wizard Public Git URL tab validation message",
  },
  "services.createPublicUrlFileError": {
    message: "ファイルのページではなく、リポジトリの URL を使用してください。",
    description:
      "Create-wizard Public Git URL: a GitHub file (blob) page was pasted, which names no repository to build (w4/576)",
  },
  "services.createImageLabel": {
    message: "Docker イメージ",
    description: "Create-wizard Existing Image tab input label",
  },
  "services.createImagePlaceholder": {
    message: "docker.io/library/nginx:latest",
    description: "Create-wizard Existing Image tab input placeholder",
  },
  "services.createImagePortHint": {
    message:
      "bex はイメージが宣言したポート（EXPOSE）を使用します。イメージが宣言していないポートで待ち受ける場合のみ、下でポートを設定してください。",
    description:
      "Create-wizard Existing Image tab hint: set the port below to the one the image listens on (w9/011, w6/m149)",
  },
  "services.createImageMalformed": {
    message:
      "Docker イメージの参照を指定してください。例: docker.io/library/nginx:latest — 小文字のパスで、タグまたはダイジェストは任意です。",
    description: "Inline error under a malformed Docker Image field (w4/406)",
  },
  "services.createImageTooLong": {
    message: "最大 512 バイトまで",
    description: "Inline error under an over-long Docker Image field (w4/406)",
  },
  "services.createImagePrivateHost": {
    message:
      "レジストリのホストは公開されている必要があります。localhost、プライベートアドレス、リンクローカルアドレスは使用できません。",
    description:
      "Inline error under a Docker Image field naming a private registry host (w4/406)",
  },
  "services.createFieldPort": {
    message: "ポート",
    description: "Create wizard: service port field label",
  },
  "services.createFieldPortFromImage": {
    message: "イメージから取得",
    description:
      "Create wizard: port placeholder for an image-port service (a Docker-built or prebuilt-image web or private service), whose ports come from its image (w6/m148, w4/m243, w4/m323)",
  },
  "services.createFieldPortImageHint": {
    message: "空欄ならイメージが宣言したポートが $PORT になります。",
    description:
      "Create wizard: port hint for an image-port private service (Docker-built or prebuilt image), whose primary is also its $PORT — blank takes the image's declared ports (w6/m148, w4/m323, w4/615)",
  },
  "services.createFieldPortImageHintWeb": {
    message:
      "空欄なら、イメージが宣言したポートへトラフィックを送ります。ポートを設定しない限り、$PORT 変数は 10000 のままです。",
    description:
      "Create wizard: port hint for an image-port web service (Docker-built or prebuilt image) — the selected port gets the traffic, while $PORT stays Render's 10000 unless set (w4/m243, w4/615)",
  },
  "services.createFieldPortHint": {
    message:
      "コンテナはポート {port} で待ち受ける必要があります。bex はそこへトラフィックを送り、$PORT として設定します。",
    description: "Create wizard: service port field hint",
  },
  "services.createRegistryCredentialLabel": {
    message: "レジストリ認証情報",
    description: "Create-wizard private-image registry credential label",
  },
  "services.createRegistryCredentialPlaceholder": {
    message: "レジストリ認証情報を選択",
    description: "Create-wizard registry credential picker placeholder",
  },
  "services.createRegistryCredentialNone": {
    message: "認証情報なし — 公開イメージ",
    description: "Create-wizard registry credential picker empty option",
  },
  "services.createRegistryCredentialDescription": {
    message:
      "イメージまたは Dockerfile のベースイメージがプライベートの場合は、保存済みのワークスペース認証情報を選択してください。",
    description: "Create-wizard registry credential picker help text",
  },
  "services.registryCredentialEmpty": {
    message: "利用できる保存済みのレジストリ認証情報がありません。",
    description: "Registry credential selector empty state",
  },
  "services.registryCredentialListError": {
    message: "レジストリ認証情報を読み込めませんでした。",
    description: "Registry credential selector load error",
  },
  "services.registryCredentialManage": {
    message: "レジストリ認証情報を管理",
    description: "Link from a service credential selector to account settings",
  },
  "services.registryCredentialSettingsTitle": {
    message: "イメージレジストリ認証情報",
    description: "Service Settings private-image credential card title",
  },
  "services.registryCredentialSettingsDescription": {
    message:
      "このプライベートイメージまたは Dockerfile のベースイメージの取得に使用する、保存済みの認証情報を選択してください。",
    description: "Service Settings private-image credential card description",
  },
  "services.registryCredentialSave": {
    message: "認証情報を保存",
    description: "Service Settings registry credential save action",
  },
  "services.registryCredentialSaved": {
    message: "レジストリ認証情報を更新しました。",
    description: "Toast after attaching or changing a registry credential",
  },
  "services.registryCredentialCleared": {
    message: "レジストリ認証情報を解除しました。",
    description: "Toast after clearing a registry credential",
  },
  "services.registryCredentialError": {
    message: "レジストリ認証情報を更新できませんでした。",
    description: "Toast after a registry credential update fails",
  },
  "services.headerRegistryCredential": {
    message: "レジストリ認証情報:",
    description: "Service detail header label for a bound registry credential",
  },
  "services.createSettingsTitle": {
    message: "設定",
    description: "Create-wizard settings section heading",
  },
  "services.createFieldName": {
    message: "名前",
    description: "Create-wizard name input label",
  },
  "services.createFieldNamePlaceholder": {
    message: "my-service",
    description: "Create-wizard name input placeholder",
  },
  "services.createFieldNameError": {
    message: "改行や制御文字を含まない、100 文字以内の名前を入力してください。",
    description: "Create-wizard name validation message",
  },
  "services.createFieldNameResourceName": {
    message: "リソース名: {name}",
    description:
      "Create-wizard preview of the resource name (slug) bex derives from a free-form service name (w5/m153)",
  },
  "services.createFieldNameTaken": {
    message: "この名前はすでに使用されています",
    description:
      "Create-wizard inline error when the service name is already taken in the current workspace (w4/m19)",
  },
  "services.createFieldNameUseSuggestion": {
    message: "{name} を使用",
    description:
      "Create-wizard button offering the suggested free name in place of a taken one (w4/m19)",
  },
  "services.createFieldNameChecking": {
    message: "利用可否を確認中…",
    description:
      "Create-wizard transient message while the debounced name-availability check is in flight (w4/m19)",
  },
  "services.createFieldBranch": {
    message: "ブランチ",
    description: "Create-wizard branch input label (git sources)",
  },
  "services.createFieldBranchPlaceholder": {
    message: "リポジトリのデフォルトブランチ",
    description:
      "Create-wizard branch input placeholder when the repo's default is not known client-side: a blank branch builds the repository's default branch (w8/m76)",
  },
  "services.createFieldRootDir": {
    message: "ルートディレクトリ",
    description: "Create-wizard root-directory input label",
  },
  "services.createFieldRootDirPlaceholder": {
    message: "例: backend",
    description: "Create-wizard root-directory input placeholder",
  },
  "services.createFieldRootDirHint": {
    message:
      "ビルドを実行するサブディレクトリです。空欄の場合はリポジトリのルートを使用します。",
    description: "Create-wizard root-directory input hint text",
  },
  "services.createFieldRuntime": {
    message: "ランタイム",
    description: "Create-wizard Render-compatible runtime selector label",
  },
  "services.createFieldBuildCommand": {
    message: "ビルドコマンド",
    description: "Create-wizard Render-compatible build command label",
  },
  "services.createFieldStartCommand": {
    message: "開始コマンド",
    description: "Create-wizard Render-compatible start command label",
  },
  "services.createFieldDockerfilePath": {
    message: "Dockerfile のパス",
    description: "Create-wizard Dockerfile-path label for the Docker runtime",
  },
  "services.createFieldDockerfilePathPlaceholder": {
    message: "Dockerfile",
    description: "Create-wizard Dockerfile-path placeholder",
  },
  "services.createFieldDockerfilePathHint": {
    message:
      "ルートディレクトリからの相対パスです。空欄の場合は Dockerfile を使用します。",
    description: "Create-wizard Dockerfile-path help text",
  },
  "services.createFieldDockerCommand": {
    message: "Docker コマンド",
    description: "Create-wizard Docker CMD override label (Render wording)",
  },
  "services.createFieldDockerCommandPlaceholder": {
    message: "Dockerfile の CMD を使用",
    description: "Create-wizard optional Docker Command placeholder",
  },
  "services.imageCommandPlaceholder": {
    message: "イメージのデフォルトコマンドを使用",
    description:
      "Create-wizard and settings placeholder for an Existing Image service's optional Docker Command",
  },
  "services.imageCommandHint": {
    message:
      "イメージのデフォルトコマンド（CMD）を上書きします。空欄の場合はデフォルトを使用します。",
    description: "Existing Image service Settings: Docker Command help text",
  },
  "services.createRuntimeNode": {
    message: "Node",
    description: "Create-wizard Node runtime option",
  },
  "services.createRuntimePython": {
    message: "Python 3",
    description: "Create-wizard Python runtime option",
  },
  "services.createRuntimeGo": {
    message: "Go",
    description: "Create-wizard Go runtime option",
  },
  "services.createRuntimeRuby": {
    message: "Ruby",
    description: "Create-wizard Ruby runtime option",
  },
  "services.createRuntimeRust": {
    message: "Rust",
    description: "Create-wizard Rust runtime option",
  },
  "services.createRuntimeElixir": {
    message: "Elixir",
    description: "Create-wizard Elixir runtime option",
  },
  "services.createRuntimeDocker": {
    message: "Docker",
    description: "Create-wizard Docker runtime option",
  },
  "services.createFieldPlan": {
    message: "コンピュートプラン",
    description: "Create-wizard plan-picker section label",
  },
  "services.createFieldAutoDeploy": {
    message: "プッシュ時に自動デプロイ",
    description: "Create-wizard auto-deploy toggle label",
  },
  "services.createFieldAutoDeployHint": {
    message: "このブランチにプッシュすると自動的に再デプロイします。",
    description: "Create-wizard auto-deploy toggle hint text",
  },
  "services.createCancel": {
    message: "キャンセル",
    description: "Create-wizard cancel button",
  },
  "services.createSubmit": {
    message: "サービスをデプロイ",
    description: "Create-wizard submit button",
  },
  "services.createSuccess": {
    message: "{name} をデプロイしています…",
    description: "Toast shown after createService succeeds",
  },
  "services.createError": {
    message: "{name} を作成できませんでした。もう一度お試しください。",
    description: "Toast shown after createService fails",
  },
  "services.scaleSuccess_other": {
    message: "{count} 個のインスタンスにスケールしています…",
    description:
      "Toast acknowledging that scaleService accepted the desired count; convergence is still asynchronous (plural)",
  },
  "services.scaleError": {
    message: "インスタンス数を更新できませんでした。",
    description: "Toast shown after scaleService fails",
  },
  "services.createTypePickerTitle": {
    message: "サービスタイプ",
    description: "Label above the service type picker in the create wizard",
  },
  "services.createTypeWebDesc": {
    message: "サービスを公開 URL で公開します",
    description:
      "Description shown under the Web Service type card in the create wizard",
  },
  "services.createTypePrivateDesc": {
    message: "プラットフォームのネットワーク内からのみアクセスできます",
    description: "Description shown under the Private Service type card",
  },
  "services.createTypeWorkerDesc": {
    message: "ポートや URL を持たないバックグラウンド処理を実行します",
    description: "Description shown under the Background Worker type card",
  },
  "services.createTypeCronDesc": {
    message: "コマンドを定期的なスケジュールで実行します",
    description: "Description shown under the Cron Job type card",
  },
  "services.createTypeStaticDesc": {
    message: "静的サイトをビルドし、オブジェクトストレージから配信します",
    description: "Description shown under the Static Site type card",
  },
  "services.createFieldSchedule": {
    message: "スケジュール",
    description: "Label for the cron schedule field in the create wizard",
  },
  "services.createFieldSchedulePlaceholder": {
    message: "0 0 * * *",
    description: "Placeholder for the cron schedule field",
  },
  "services.createFieldScheduleHint": {
    message:
      "5 フィールドの crontab 式（分 時 日 月 曜日）です。UTC で実行されます。",
    description: "Hint text under the schedule field",
  },
  "services.createFieldSchedulePreview": {
    message: "{description} · UTC で実行",
    description:
      "Live human-readable description of a valid schedule, shown under the field",
  },
  "services.createFieldScheduleError": {
    message: "有効な 5 フィールドの cron 式を入力してください。例: 0 0 * * *",
    description: "Validation error for an invalid cron expression",
  },
  "services.createFieldCommand": {
    message: "コマンド",
    description: "Label for the command field in the create wizard",
  },
  "services.createFieldCommandPlaceholder": {
    message: "python script.py",
    description: "Placeholder for the command field",
  },
  "services.createFieldCommandHint": {
    message: "スケジュールされた実行のたびに実行するコマンドです。",
    description: "Hint text under the command field",
  },
  "services.createFieldPublishPath": {
    message: "公開ディレクトリ",
    description: "Label for the publish directory field in the create wizard",
  },
  "services.createFieldPublishPathPlaceholder": {
    message: "例: dist",
    description: "Placeholder for the publish directory field",
  },
  "services.createFieldPublishPathHint": {
    message:
      "ビルドしたサイトを置くフォルダー。ルートディレクトリからの相対パスです（例: dist、ルートディレクトリ自体なら .）。",
    description: "Hint text under the publish directory field",
  },
  "services.createFieldPublishPathRequired": {
    message:
      "必須です。サイトのルートとして配信するビルド出力ディレクトリを、ルートディレクトリからの相対パスで入力してください（例: dist、build、public、またはルートディレクトリ自体を指す . — ルートディレクトリが空欄の場合はリポジトリのルート）。",
    description:
      "Hint under the publish directory field while it is blank: the field is required, the placeholder is only an example",
  },
  "services.createNoPublicUrlNote": {
    message: "このサービスタイプには公開 URL がありません。",
    description:
      "Note shown for private/worker types that don't produce a public URL",
  },
  "services.createFieldEnvVarsTitle": {
    message: "環境変数",
    description: "Section heading for env vars in the create wizard",
  },
  "services.createFieldEnvVarsAdd": {
    message: "変数を追加",
    description: "Button to add an env var row in the create wizard",
  },
  "services.envRowKey": {
    message: "変数 {n} のキー",
    description:
      "A create-flow variable row's key field (aria-label), named by its row as its Remove button is (w4/544)",
  },
  "services.envRowValue": {
    message: "変数 {n} の値",
    description:
      "A create-flow variable row's value field (aria-label), named by its row (w4/544)",
  },
  "services.envRowGenerate": {
    message: "変数 {n} の値を生成",
    description:
      "A create-flow variable row's Generate button (aria-label), named by its row (w4/544)",
  },
  "services.envRowRemove": {
    message: "変数 {n} を削除",
    description:
      "A create-flow variable row's Remove button (aria-label), named by its row (w4/544)",
  },
  "services.fileRowName": {
    message: "シークレットファイル {n} のファイル名",
    description:
      "A create-flow secret file row's name field (aria-label), named by its row and starting with the dialog's visible \"File name\" label (w4/544)",
  },
  "services.fileRowContent": {
    message: "シークレットファイル {n} の内容",
    description:
      "A create-flow secret file row's contents field (aria-label), named by its row (w4/544)",
  },
  "services.fileRowRemove": {
    message: "シークレットファイル {n} を削除",
    description:
      "A create-flow secret file row's Remove button (aria-label), named by its row (w4/544)",
  },
  "services.createFieldEnvVarsKey": {
    message: "キー",
    description: "Label for the env var key column in the create wizard",
  },
  "services.createFieldEnvVarsValue": {
    message: "値",
    description: "Label for the env var value column in the create wizard",
  },
  "services.createFieldEnvVarsKeyPlaceholder": {
    message: "KEY_NAME",
    description: "Placeholder for the env var key input in the create wizard",
  },
  "services.createFieldEnvVarsValuePlaceholder": {
    message: "値",
    description: "Placeholder for the env var value input in the create wizard",
  },
  "services.createFieldEnvVarsKeyError": {
    message:
      "英字、数字、ドット、ハイフン、アンダースコアを 253 文字以内で使用してください。'.' のみ、または '..' で始まるキーは使用できません。",
    description:
      "Error shown when an env var key is invalid in the create wizard",
  },
  "services.createFieldSecretFilesTitle": {
    message: "シークレットファイル",
    description: "Section heading for secret files in the create wizard",
  },
  "services.createFieldSecretFilesHint": {
    message:
      "最初のデプロイから /etc/secrets 配下に読み取り専用でマウントされます。",
    description: "Hint for create-time secret files",
  },
  "services.createFieldSecretFilesAdd": {
    message: "シークレットファイルを追加",
    description: "Button to add a create-time secret file",
  },
  "services.createFieldSecretFilesNamePlaceholder": {
    message: "credentials.json",
    description: "Placeholder for a secret file name",
  },
  "services.createFieldSecretFilesContentPlaceholder": {
    message: "シークレットの内容を貼り付け",
    description: "Placeholder for secret file contents",
  },
  "services.createFieldSecretFilesNameError": {
    message:
      "英字、数字、ドット、ハイフン、アンダースコアのみ使用できます。. と .. は使用できません。",
    description: "Invalid secret file name error",
  },
  "services.createFieldEnvironmentTitle": {
    message: "プロジェクトと環境",
    description: "Create wizard grouping section title",
  },
  "services.createFieldProject": {
    message: "プロジェクト",
    description: "Accessible label for the create project picker",
  },
  "services.createFieldProjectNone": {
    message: "プロジェクトなし",
    description: "Unassigned option in the project picker",
  },
  "services.createFieldEnvironment": {
    message: "環境",
    description: "Accessible label for the create environment picker",
  },
  "services.createFieldEnvironmentNone": {
    message: "環境なし",
    description: "Unassigned option in the environment picker",
  },
  "services.createFieldEnvironmentHint": {
    message: "環境を選択すると、サービスはその親プロジェクトにも追加されます。",
    description: "Hint for create-time environment assignment",
  },
  "services.navEvents": {
    message: "イベント",
    description: "Service-detail nav item (events tab)",
  },
  "services.navDeploys": {
    message: "デプロイ",
    description:
      "Service-detail nav item (dedicated deploy-history tab, w9/002)",
  },
  "services.navRuns": {
    message: "実行",
    description:
      "Service-detail nav item for a cron job: its run history, Render's Runs page (w2/m360)",
  },
  "services.navDisk": {
    message: "ディスク",
    description:
      "Service-detail nav item (persistent disk page); Render's label is singular",
  },
  "services.diskDescription": {
    message:
      "SSD をアタッチすると、デプロイをまたいでサービスのファイルシステムのデータを保持できます。ディスクの料金は月額 $0.175/GB です。",
    description:
      "Disk tab card description; bex's rate is 30% below Render's $0.25",
  },
  "services.diskEmptyTitle": {
    message: "ディスクはアタッチされていません",
    description: "Disk tab empty state title",
  },
  "services.diskEmptyBody": {
    message:
      "デプロイや再起動をまたいで保持されるのは、ディスクのマウントパス配下のファイルのみです。",
    description: "Disk tab empty state body",
  },
  "services.diskPaidOnly": {
    message:
      "ディスクには有料のコンピュートプランが必要です。ディスクをアタッチするには、このサービスのプランをアップグレードしてください。",
    description: "Shown instead of the add button on a free service",
  },
  "services.diskAddAction": {
    message: "ディスクを追加",
    description: "Button that opens and submits the add-disk form",
  },
  "services.diskLoadErrorTitle": {
    message: "このサービスのディスクを読み込めませんでした",
    description: "Disk tab error state title",
  },
  "services.diskWarningsTitle": {
    message: "以下の点にご注意ください。",
    description: "Heading above the five add-disk warnings (Render's wording)",
  },
  "services.diskWarningZeroDowntime": {
    message:
      "ディスクをアタッチすると、サービスのゼロダウンタイムデプロイは無効になります。",
    description: "Add-disk warning 1",
  },
  "services.diskWarningSingleInstance": {
    message:
      "ディスクがアタッチされたサービスは、複数のインスタンスにスケールできません。",
    description: "Add-disk warning 2",
  },
  "services.diskWarningOnePerService": {
    message: "1 つのサービスにアタッチできるディスクは最大 1 つです。",
    description: "Add-disk warning 3",
  },
  "services.diskWarningMountPathOnly": {
    message: "ディスクのマウントパス配下のファイルのみが永続化されます。",
    description: "Add-disk warning 4",
  },
  "services.diskWarningNoSharing": {
    message: "他のサービスからこのサービスのディスクにはアクセスできません。",
    description: "Add-disk warning 5",
  },
  "services.diskMountPathLabel": {
    message: "マウントパス",
    description: "Add-disk form field label",
  },
  "services.diskMountPathHint": {
    message:
      "ディスクの絶対マウントパスです。このパス配下のファイルのみがデプロイをまたいで永続化されます。ルートディレクトリ（/）は指定できません。",
    description: "Add-disk mount path helper text",
  },
  "services.diskMountPathRequired": {
    message: "マウントパスを入力してください。",
    description: "Client-side validation: empty mount path",
  },
  "services.diskMountPathAbsolute": {
    message: "マウントパスは絶対パス（/ で始まる）である必要があります。",
    description: "Client-side validation: relative mount path",
  },
  "services.diskMountPathNotRoot": {
    message:
      "マウントパスにルートディレクトリは指定できず、末尾にスラッシュも付けられません。",
    description: "Client-side validation: root or trailing slash",
  },
  "services.diskMountPathReserved": {
    message:
      "このパスはプラットフォームで予約されています。代わりにそのサブディレクトリをマウントしてください。",
    description: "Client-side validation: reserved mount path",
  },
  "services.diskSizeLabel": {
    message: "サイズ",
    description: "Add-disk size field label",
  },
  "services.diskSizeHint": {
    message:
      "サイズは後から増やせますが、減らすことはできません。用途に合う最小の値から始めることをおすすめします。",
    description: "Add-disk size helper text (Render's wording)",
  },
  "services.diskSizeChip": {
    message: "{size} GB",
    description: "Size quick-select chip and size display",
  },
  "services.diskAddSuccess": {
    message: "ディスクをアタッチしました。サービスを再デプロイしています。",
    description: "Toast after a disk is attached",
  },
  "services.diskGrowLabel": {
    message: "サイズを増やす（GB）",
    description: "Grow control label on an attached disk",
  },
  "services.diskGrowAction": {
    message: "サイズを増やす",
    description: "Grow button",
  },
  "services.diskGrowHint": {
    message:
      "サイズを増やしてもダウンタイムは発生せず、追加したストレージは数秒以内に利用可能になります。ディスクを小さくすることはできません。",
    description: "Grow helper text in Render's wording",
  },
  "services.diskResizeSuccess": {
    message: "ディスクのサイズを増やしました。",
    description: "Toast after a successful grow",
  },
  "services.diskDeleteAction": {
    message: "ディスクを削除",
    description: "Destructive action on an attached disk",
  },
  "services.diskDeleteTitle": {
    message: "このディスクを削除しますか？",
    description: "Delete confirmation title",
  },
  "services.diskDeleteWarning": {
    message:
      "ディスク上のすべてのデータが失われ、サービスはただちにディスクにアクセスできなくなります。スナップショットも削除されます。この操作は元に戻せません。",
    description: "Delete confirmation body",
  },
  "services.diskDeleteConfirm": {
    message: "ディスクを削除",
    description: "Delete confirmation action",
  },
  "services.diskDeleteSuccess": {
    message: "ディスクを削除しました。",
    description: "Toast after a disk is deleted",
  },
  "services.diskSnapshotsTitle": {
    message: "スナップショット",
    description: "Snapshots card title",
  },
  "services.diskSnapshotsDescription": {
    message:
      "このディスクのスナップショットは毎日作成され、少なくとも 7 日間保持されます。",
    description: "Snapshots card description",
  },
  "services.diskSnapshotsNotConfiguredTitle": {
    message: "スナップショットが設定されていません",
    description:
      "Snapshots card heading when the deployment has no snapshot object store configured (not a failure).",
  },
  "services.diskSnapshotsNotConfiguredBody": {
    message:
      "この bex 環境にはディスクスナップショット用のオブジェクトストアが設定されていないため、スナップショットは作成されていません。データには影響ありません。有効化するには運用担当者に依頼してください。",
    description:
      "Explains that snapshots are unconfigured rather than broken, and that an operator enables them.",
  },
  "services.diskSnapshotsEmptyTitle": {
    message: "スナップショットはまだありません",
    description: "Snapshots empty state title",
  },
  "services.diskSnapshotsEmptyBody": {
    message: "最初のスナップショットは、次回の夜間実行時に作成されます。",
    description: "Snapshots empty state body",
  },
  "services.diskSnapshotsFailedTitle": {
    message: "前回のスナップショットに失敗しました",
    description: "A disk's most recent nightly snapshot attempt failed",
  },
  "services.diskSnapshotsFailedReadBody": {
    message:
      "このディスクの一部を読み取れなかったため、スナップショットは作成されず、最近の変更はバックアップされていません。既存のスナップショットには影響ありません。再度発生する場合は、サポートにお問い合わせください。",
    description: "Failed snapshot: the volume could not be read",
  },
  "services.diskSnapshotsFailedUploadBody": {
    message:
      "スナップショットをストレージに保存できなかったため、最近の変更はバックアップされていません。既存のスナップショットには影響ありません。次回の夜間実行で再試行します。",
    description: "Failed snapshot: the object store did not accept it",
  },
  "services.diskSnapshotsFailedConfigBody": {
    message:
      "スナップショットのジョブを正しく開始できなかったため、最近の変更はバックアップされていません。既存のスナップショットには影響ありません。サポートにお問い合わせください。",
    description: "Failed snapshot: the job was misconfigured",
  },
  "services.diskSnapshotsFailedOtherBody": {
    message:
      "前回のスナップショットの試行に失敗したため、最近の変更はバックアップされていません。既存のスナップショットには影響ありません。再度発生する場合は、サポートにお問い合わせください。",
    description: "Failed snapshot: an unrecognized stage",
  },
  "services.diskSnapshotsOverdueTitle": {
    message: "最近のスナップショットがありません",
    description: "A disk's nightly snapshot did not arrive when expected",
  },
  "services.diskSnapshotsOverdueBody": {
    message:
      "{dueBy} までに作成されるはずの夜間スナップショットが作成されていないため、このディスクの最近の変更はバックアップされていません。既存のスナップショットには影響ありません。解消しない場合は、サポートにお問い合わせください。",
    description:
      "Explains an overdue nightly snapshot; {dueBy} is a local date/time",
  },
  "services.diskSnapshotsNotRunningTitle": {
    message: "スナップショットが実行されていません",
    description:
      "The platform could not schedule this disk's nightly snapshots",
  },
  "services.diskSnapshotsNotRunningBody": {
    message:
      "bex がこのディスクの夜間スナップショットをスケジュールできなかったため、新しい変更はバックアップされていません。既存のスナップショットには影響ありません。サポートにお問い合わせください。",
    description:
      "Explains that nightly snapshots cannot be scheduled and whom to contact",
  },
  "services.diskSnapshotsUnavailableTitle": {
    message: "スナップショットを利用できません",
    description: "Snapshots error state title",
  },
  "services.diskRestoreDatabaseWarning": {
    message:
      "ディスク上で稼働しているデータベースを復旧するためにディスクを復元しないでください。稼働中のデータベースの下でファイルを復元すると、データベースが破損するおそれがあります。代わりに、そのデータベース自体のバックアップと復元を使用してください。",
    description:
      "Render's database-recovery warning, carried verbatim in meaning",
  },
  "services.diskRestoreAction": {
    message: "復元",
    description: "Restore button on a snapshot row",
  },
  "services.diskRestoreTitle": {
    message: "このスナップショットを復元しますか？",
    description: "Restore confirmation title",
  },
  "services.diskRestoreWarning": {
    message:
      "サービスが停止し、ディスクがこのスナップショットに置き換えられてから、サービスが再起動します。スナップショット以降に書き込まれた内容はすべて失われます。この操作は元に戻せません。",
    description: "Restore confirmation body",
  },
  "services.diskRestoreConfirm": {
    message: "ディスクを復元",
    description: "Restore confirmation action",
  },
  "services.diskUnsupportedType": {
    message:
      "永続ディスクはマウント先として常時稼働するインスタンスが必要なため、Web Service、Private Service、Background Worker で利用できます。",
    description:
      "Empty-state body on the Disk tab for a service type that cannot have a disk.",
  },
  "services.diskSudoPrompt": {
    message: "続行するには {phrase} と入力してください",
    description:
      "Label above the typed-confirmation input for an irreversible disk action.",
  },
  "services.diskConfigTitle": {
    message: "ディスク設定",
    description:
      "Heading of the card holding the disk's mount path and size settings.",
  },
  "services.diskConfigDescription": {
    message: "このディスクのマウントパスとサイズを設定します。",
    description: "Subtitle of the disk configuration card.",
  },
  "services.diskSizeUnit": {
    message: "GB",
    description: "Unit suffix shown beside the disk size input.",
  },
  "services.diskEditAction": {
    message: "編集",
    description: "Button that unlocks the disk size field for editing.",
  },
  "services.diskProvisionedLabel": {
    message: "プロビジョニング済み {size} GB",
    description:
      "Provisioned disk size shown beside the usage chart heading; this is the size billing meters.",
  },
  "services.diskDeleteCardTitle": {
    message: "ディスクを削除",
    description:
      "Heading of the delete-disk card. Distinct from diskDeleteTitle, which is the confirmation dialog's question.",
  },
  "services.diskDeleteCardDescription": {
    message:
      "ディスクはこのサービスから切り離され、ディスク上のすべてのデータが完全に削除されます。",
    description: "Subtitle of the delete-disk card, stating what is lost.",
  },
  "services.diskUsageTitle": {
    message: "ディスク使用量",
    description: "Heading of the disk usage chart on a service's Disk tab.",
  },
  "services.diskUsageDescription": {
    message:
      "マウントされたボリュームに書き込まれたバイト数を、プロビジョニング済みの容量と比較して表示します。課金対象は使用バイト数ではなく、プロビジョニングされたサイズです。",
    description:
      "Subtitle of the disk usage chart, stating that billing uses provisioned size.",
  },
  "services.diskRestoreStarted": {
    message: "ディスクを復元しています。完了するまでサービスは停止します。",
    description: "Toast after a restore is requested",
  },
  "services.navShell": {
    message: "シェル",
    description: "Service-detail nav item (running-instance SSH page)",
  },
  "services.navCompute": {
    message: "コンピュート",
    description:
      "Service sidebar link to the Compute page: the compute plan, manual scaling and autoscaling, as on Render (w6/m483)",
  },
  "services.eventsCount_other": {
    message: "イベント {count} 件",
    description:
      "Accessible label for the number of visible service events (plural)",
  },
  "services.eventsCountAtLeast_other": {
    message: "イベント {count} 件以上",
    description:
      "Accessible label for the event count badge while older events remain unread (plural)",
  },
  "services.eventsEmptyTitle": {
    message: "イベントはまだありません",
    description: "Events tab empty-state title",
  },
  "services.eventsEmpty": {
    message: "デプロイとサービスの変更がここに表示されます。",
    description: "Events tab empty-state description",
  },
  "services.eventsErrorTitle": {
    message: "イベントを表示できません",
    description: "Events tab query-error title",
  },
  "services.eventsErrorDescription": {
    message: "このサービスのイベントを読み込めませんでした。",
    description: "Events tab query-error description",
  },
  "services.eventsRetry": {
    message: "再試行",
    description: "Events tab query-error retry button",
  },
  "services.eventsLoadOlder": {
    message: "古いイベントを読み込む",
    description: "Button that loads the next page or earlier event window",
  },
  "services.eventsLoadingOlder": {
    message: "古いイベントを読み込み中…",
    description: "Busy label while an older service-event page is loading",
  },
  "services.eventsHistoryEnd": {
    message: "このサービスの履歴の最初まで到達しました。",
    description: "Terminal label after all retained service events are loaded",
  },
  "services.eventsFilter": {
    message: "イベントを絞り込む",
    description: "Button opening the service event-type filter",
  },
  "services.eventsFilterSelected": {
    message: "イベントの種類: {count}",
    description: "Event filter button with selected-type count",
  },
  "services.eventsFilterSearch": {
    message: "イベントの種類を検索",
    description: "Search input for service event types",
  },
  "services.eventsFilterShowAll": {
    message: "すべてのイベントを表示",
    description:
      "Events tab: button in the filtered-to-nothing state that shows every event type again",
  },
  "services.eventsShownOf": {
    message: "{total} 件中 {shown} 件を表示",
    description:
      "Events tab: count over the feed while a filter hides some rows; {total} is the loaded count, '20+' while older events remain",
  },
  "services.eventsFilterAll": {
    message: "すべてのイベント",
    description: "Master service event filter option",
  },
  "services.eventsFilterNoMatches": {
    message: "検索に一致するイベントタイプがありません。",
    description: "Empty search result in the event filter",
  },
  "services.eventsFilterEmptyTitle": {
    message: "一致するイベントがありません",
    description: "Title when loaded history has no selected event types",
  },
  "services.eventsFilterEmpty": {
    message: "イベントタイプをさらに選択するか、古い履歴を読み込んでください。",
    description: "Body when loaded history has no selected event types",
  },
  "services.eventsFilterNoneTitle": {
    message: "イベントの種類が選択されていません",
    description:
      "Events tab: title when the filter has every event type deselected",
  },
  "services.eventsFilterNone": {
    message:
      "イベントを表示するには、少なくとも 1 つの種類を選択してください。",
    description:
      "Events tab: body when the filter has every event type deselected",
  },
  "services.eventsFilterGroup.deploy": {
    message: "デプロイ",
    description: "Deploy event filter group",
  },
  "services.eventsFilterGroup.serviceStatus": {
    message: "サービスの状態",
    description: "Service-status event filter group",
  },
  "services.eventsFilterGroup.scaling": {
    message: "スケーリング",
    description: "Scaling event filter group",
  },
  "services.eventsFilterGroup.maintenanceMode": {
    message: "メンテナンスモード",
    description: "Maintenance-mode event filter group",
  },
  "services.eventsFilterGroup.edgeCache": {
    message: "エッジキャッシュ",
    description: "Edge-cache event filter group",
  },
  "services.eventsFilterGroup.configuration": {
    message: "構成",
    description: "bex configuration event filter group",
  },
  "services.eventsFilterGroup.other": {
    message: "その他",
    description:
      "Event filter group holding types the API returned that the dashboard catalog does not know yet",
  },
  "services.eventsActor": {
    message: "{actor} による操作",
    description: "Actor attribution shown on a service event",
  },
  "services.eventsDeployReference": {
    message: "デプロイ {id}",
    description: "Deploy identifier shown on a deploy activity row",
  },
  "services.eventsTriggerRollback": {
    message: "ロールバック",
    description: "Deploy event trigger: rollback",
  },
  "services.eventsTriggerFirstBuild": {
    message: "初回デプロイ",
    description: "Deploy event trigger: the service's first deploy",
  },
  "services.eventsTriggerManual": {
    message: "手動デプロイ",
    description: "Deploy event trigger: manual",
  },
  "services.eventsTriggerEnvUpdated": {
    message: "設定の変更",
    description:
      "Deploy event trigger: an environment change (env var, secret file, or env-group edit); a Settings save no longer sets envUpdated (w8/m135)",
  },
  "services.eventsTriggerDeployHook": {
    message: "Deploy Hook",
    description:
      "Deploy event trigger: a POST to the service's secret deploy-hook URL",
  },
  "services.eventsTriggerClearCache": {
    message: "キャッシュのクリア",
    description: "Deploy event trigger: build-cache clear",
  },
  "services.eventsTriggerDeployedByRender": {
    message: "プラットフォームによるデプロイ",
    description: "Deploy event trigger: platform initiated",
  },
  "services.eventsTypeDeployStarted": {
    message: "デプロイ開始",
    description: "Service activity type: deploy started",
  },
  "services.eventsTypeDeployFinished": {
    message: "デプロイ終了",
    description: "Service activity type: deploy finished",
  },
  "services.eventsTypeZeroDowntimeRedeployStarted": {
    message: "ゼロダウンタイム再デプロイ開始",
    description:
      "Service activity type: a Bex-initiated zero-downtime deploy started for a service",
  },
  "services.eventsTypeZeroDowntimeRedeployEnded": {
    message: "ゼロダウンタイム再デプロイ終了",
    description:
      "Service activity type: a Bex-initiated zero-downtime deploy completed for a service",
  },
  "services.eventsRedeployTrigger.node_replacement": {
    message: "ノードの入れ替え",
    description:
      "zero_downtime_redeploy_started trigger: drain-guard rolled the service off a node being replaced",
  },
  "services.eventsRedeployTrigger.node_consolidation": {
    message: "オートスケーリングによるノード削除",
    description:
      "zero_downtime_redeploy_started trigger: drain-guard rolled the service off a node marked for Cluster Autoscaler scale-down",
  },
  "services.eventsTypePipelineMinutesExhausted": {
    message: "パイプライン時間を使い切りました",
    description:
      "Service activity type: a build refused because the workspace used its pipeline minutes",
  },
  "services.eventsTypeEdgeCacheEnabled": {
    message: "エッジキャッシュを有効化",
    description: "Service activity type: edge caching turned on",
  },
  "services.eventsTypeEdgeCacheDisabled": {
    message: "エッジキャッシュを無効化",
    description: "Service activity type: edge caching turned off",
  },
  "services.eventsTypeEdgeCachePurged": {
    message: "エッジキャッシュをパージ",
    description: "Service activity type: edge cache purged",
  },
  "services.eventsTypeImagePullFailed": {
    message: "イメージの取得に失敗",
    description: "Service activity type: image pull failed",
  },
  "services.eventsTypeArtifactFetchFailed": {
    message: "アーティファクトの取得に失敗",
    description:
      "Service activity type: a build published by a build source could not be fetched",
  },
  "services.eventsTypeArtifactSourceChanged": {
    message: "ビルドソースを変更",
    description:
      "Service activity type: the service's linked build source changed",
  },
  "services.eventsArtifactSourceChanged": {
    message: "ビルドソース {from} → {to}",
    description:
      "Detail line on artifact_source_changed when switching between two build sources",
  },
  "services.eventsArtifactSourceLinked": {
    message: "ビルドソース {to} にリンク",
    description:
      "Detail line on artifact_source_changed when newly linked to a build source",
  },
  "services.eventsArtifactSourceUnlinked": {
    message: "ビルドソース {from} のリンクを解除",
    description:
      "Detail line on artifact_source_changed when detached from a build source",
  },
  "services.eventsBuildReference": {
    message: "ビルド {id}",
    description:
      "Build identifier shown on an artifact_fetch_failed activity row",
  },
  "services.eventsTypeSuspended": {
    message: "一時停止",
    description: "Service activity type: service suspended",
  },
  "services.eventsTypeResumed": {
    message: "再開",
    description: "Service activity type: service resumed",
  },
  "services.eventsTypeRestarted": {
    message: "サーバーを再起動",
    description: "Service activity type: service restarted",
  },
  "services.eventsTypeInstanceFailed": {
    message: "インスタンスの障害",
    description: "Service activity type: running instance became unhealthy",
  },
  "services.eventsTypeDiskUsageHigh": {
    message: "ディスク使用量が高い状態",
    description:
      "Service activity type: a service disk is over 80% full (service_disk_usage_high)",
  },
  "services.eventsTypeDiskUsageRecovered": {
    message: "ディスク使用量が回復",
    description:
      "Service activity type: a service disk dropped back below 80% (service_disk_usage_recovered)",
  },
  "services.eventsDiskUsage": {
    message:
      "ディスク {disk}（{mountPath}）: {usage}% 使用中（しきい値 {threshold}%）",
    description: "Disk usage event detail line",
  },
  "services.eventsTypeHardwareFailure": {
    message: "ハードウェア障害",
    description:
      "Service activity type: an instance was lost with the machine it ran on",
  },
  "services.eventsTypeServiceSuspended": {
    message: "サービスを一時停止",
    description: "Service activity type: suspension converged",
  },
  "services.eventsTypeServiceResumed": {
    message: "サービスを再開",
    description: "Service activity type: resume converged",
  },
  "services.eventsTypeServiceHibernated": {
    message: "サービスがスリープ状態になりました",
    description:
      "Service activity type: free-tier idle auto-hibernation — deliberately worded apart from a user-driven suspension",
  },
  "services.eventsTypeServiceWoken": {
    message: "サービスが起動しました",
    description:
      "Service activity type: a sleeping free-tier service woke on an incoming request",
  },
  "services.eventsTypeServiceRecovered": {
    message: "サービスが復旧しました",
    description: "Service activity type: unhealthy service recovered",
  },
  "services.eventsTypePlanChanged": {
    message: "インスタンスタイプを変更",
    description: "Service activity type: plan changed",
  },
  "services.eventsTypeInstanceCountChanged": {
    message: "インスタンス数を変更",
    description: "Service activity type: manual scale",
  },
  "services.eventsTypeAutoscalingChanged": {
    message: "オートスケーリング設定を変更",
    description: "Service activity type: autoscaling configuration changed",
  },
  "services.eventsTypeAutoscalingStarted": {
    message: "オートスケーリング開始",
    description: "Service activity type: autoscaler began a replica change",
  },
  "services.eventsTypeAutoscalingEnded": {
    message: "オートスケーリング終了",
    description: "Service activity type: autoscaler replica change converged",
  },
  "services.eventsTypeBranchChanged": {
    message: "ブランチを変更",
    description: "Service activity type: tracked Git branch changed",
  },
  "services.eventsTypeCommitIgnored": {
    message: "コミットを無視",
    description: "Service activity type: matching Git push did not deploy",
  },
  "services.eventsTypeJobStarted": {
    message: "ジョブ開始",
    description: "Service activity type: one-off job started",
  },
  "services.eventsTypeJobCanceled": {
    message: "ジョブをキャンセル",
    description: "Service activity type: one-off job canceled",
  },
  "services.eventsTypeJobRunEnded": {
    message: "ジョブ終了",
    description: "Service activity type: one-off job finished",
  },
  "services.eventsTypeBuildStarted": {
    message: "ビルド開始",
    description: "Service activity type: image build started",
  },
  "services.eventsTypeBuildEnded": {
    message: "ビルド終了",
    description: "Service activity type: image build finished",
  },
  "services.eventsTypePreDeployStarted": {
    message: "プレデプロイ開始",
    description: "Service activity type: pre-deploy command started",
  },
  "services.eventsTypePreDeployEnded": {
    message: "プレデプロイ終了",
    description: "Service activity type: pre-deploy command finished",
  },
  "services.eventsTypeBranchDeleted": {
    message: "ブランチを削除",
    description: "Service activity type: tracked Git branch deleted",
  },
  "services.eventsTypeMaintenanceModeChanged": {
    message: "メンテナンスモード設定を変更",
    description: "Service activity type: maintenance mode setting changed",
  },
  "services.eventsTypeMaintenanceModeUriUpdated": {
    message: "メンテナンスモードの URI を更新",
    description: "Service activity type: maintenance mode URI changed",
  },
  "services.eventsInstanceReference": {
    message: "インスタンス {id}",
    description: "Instance identifier shown on a service event",
  },
  "services.eventsReplicaChange": {
    message: "インスタンス {from} → {to}",
    description:
      "Instance count from/to summary: an autoscaling start/end's replicas or an instance_count_changed event's counts",
  },
  "services.eventsAutoscalingChange": {
    message: "オートスケーリング {from} → {to}",
    description:
      "autoscaling_config_changed detail line; from/to are services.eventsAutoscalingOff or services.eventsAutoscalingRange (w4/568)",
  },
  "services.eventsAutoscalingOff": {
    message: "オフ",
    description: "One side of an autoscaling change: autoscaling off (w4/568)",
  },
  "services.eventsAutoscalingRange": {
    message: "{min}～{max} インスタンス",
    description:
      "One side of an autoscaling change: its minimum–maximum instances (w4/568)",
  },
  "services.eventsAutoscalingRangeCPU": {
    message: "{min}～{max} インスタンス、CPU {cpu}%",
    description:
      "One side of an autoscaling change: its instance range and CPU target (w4/m335)",
  },
  "services.eventsAutoscalingRangeMemory": {
    message: "{min}～{max} インスタンス、メモリ {memory}%",
    description:
      "One side of an autoscaling change: its instance range and memory target (w4/m335)",
  },
  "services.eventsAutoscalingRangeCPUMemory": {
    message: "{min}～{max} インスタンス、CPU {cpu}%、メモリ {memory}%",
    description:
      "One side of an autoscaling change: its instance range and both targets (w4/m335)",
  },
  "services.eventsPlanChange": {
    message: "コンピュートプラン {from} → {to}",
    description:
      "plan_changed detail line, by the compute plans' names (w4/568, w6/m483)",
  },
  "services.eventsBranchChange": {
    message: "ブランチ {from} → {to}",
    description: "Tracked branch from/to summary",
  },
  "services.eventsAutoDeployDisabledReason.setting_change": {
    message: "設定でオフにしました",
    description:
      "auto_deploy_disabled reason: the auto-deploy setting was turned off (w4/m431)",
  },
  "services.eventsAutoDeployDisabledReason.rollback": {
    message:
      "ロールバックによりオフになりました。以降のプッシュで置き換えられません",
    description:
      "auto_deploy_disabled reason: a dashboard rollback turned auto-deploy off (w4/m431)",
  },
  "services.eventsAutoDeployDisabledReason.manual_deploy": {
    message:
      "特定のコミットのデプロイによりオフになりました。以降のプッシュで置き換えられません",
    description:
      "auto_deploy_disabled reason: a deploy of a specific commit from the dashboard or a deploy hook turned auto-deploy off (w4/m431)",
  },
  "services.eventsBranchDeleted": {
    message: "ブランチ {branch} を削除しました — 自動デプロイは無効です",
    description: "Deleted tracked branch summary",
  },
  "services.eventsStatus.succeeded": {
    message: "成功",
    description: "Lifecycle-step outcome badge: succeeded",
  },
  "services.eventsStatus.failed": {
    message: "失敗",
    description: "Lifecycle-step outcome badge: failed",
  },
  "services.eventsStatus.canceled": {
    message: "キャンセル済み",
    description: "Lifecycle-step outcome badge: canceled",
  },
  "services.eventsReason.superseded": {
    message: "より新しいデプロイにこのデプロイが置き換えられました。",
    description:
      "Deploy/build ended reason: a newer deploy superseded this one",
  },
  "services.eventsReason.image_pull_backoff": {
    message: "ランタイムが設定されたイメージを取得できませんでした。",
    description: "Bounded public image-pull failure reason",
  },
  "services.eventsReason.readiness_failed": {
    message: "実行中のインスタンスがヘルスチェックに失敗しました。",
    description:
      "Bounded public readiness failure reason; names the health check, as Settings and REST do (w4/m327)",
  },
  "services.eventsReason.root_directory": {
    message:
      "サービスのルートディレクトリに一致する変更ファイルがありませんでした。",
    description: "Commit ignored because of root-directory scoping",
  },
  "services.eventsReason.build_filter": {
    message:
      "変更されたファイルがサービスのビルドフィルターを通過しませんでした。",
    description: "Commit ignored because of build filters",
  },
  "services.eventsReason.skip_phrase": {
    message:
      "コミットメッセージにより、このデプロイのスキップが要求されました。",
    description: "Commit ignored because of a Render skip phrase",
  },
  "services.eventsReason.non_zero_exit": {
    message: "実行はステータス {exitCode} で終了しました。",
    description:
      "Cron run failure reason: the container exited non-zero (w4/m114)",
  },
  "services.eventsReason.oom_killed": {
    message: "実行中にメモリが不足し、停止されました。",
    description:
      "Cron run failure reason: the container was OOM-killed (w4/m114)",
  },
  "services.eventsReason.instance.non_zero_exit": {
    message:
      "インスタンスのプロセスがステータス {exitCode} で終了したため、再起動しました。",
    description:
      "Instance failed reason for a running service: its process crashed (w8/m158)",
  },
  "services.eventsReason.instance.oom_killed": {
    message:
      "インスタンスのメモリが不足したため（上限 {memory}）、再起動しました。",
    description:
      "Instance failed reason for a running service: killed at its plan's memory limit (w8/m158)",
  },
  "services.eventsReason.instance.readiness_failed_result": {
    message: "実行中のインスタンスがヘルスチェックに失敗しました: {result}。",
    description:
      'Instance failed reason for a running service whose health check failed, naming what the check got back (w6/m634); {result} is Render\'s English message ("HTTP health check failed with status code 503"), shown verbatim in every language as Render shows it',
  },
  "services.eventsReason.instance.oom_killed_no_limit": {
    message: "インスタンスのメモリが不足したため、再起動しました。",
    description:
      "Instance failed reason for a running service: killed for memory, limit unknown (w8/m158)",
  },
  "services.eventsReason.early_exit": {
    message:
      "インスタンスのプロセスが終了したため、再起動しました。サービスは動き続けることが想定されています。",
    description:
      "Instance failed reason for a running service: its process ended with status 0 (w8/m158)",
  },
  "services.eventsReason.evicted": {
    message: "実行が完了する前に、ノードから退避されました。",
    description: "Cron run failure reason: the run's pod was evicted (w4/m114)",
  },
  "services.eventsReason.timed_out": {
    message: "実行が 12 時間の上限に達したため、停止されました。",
    description:
      "Cron run failure reason: the run exceeded its deadline (w4/m114)",
  },
  "services.eventsTypeCronRunStarted": {
    message: "Cron 実行開始",
    description: "Service activity type: cron run started",
  },
  "services.eventsTypeCronRunFinished": {
    message: "Cron 実行完了",
    description: "Service activity type: cron run finished",
  },
  "services.eventsTypeEnvVarsChanged": {
    message: "環境変数を変更",
    description: "Service activity type: environment variables changed",
  },
  "services.eventsTypeEnvironmentChanged": {
    message: "環境設定を変更",
    description:
      "Service activity type: environment variables and secret files changed",
  },
  "services.eventsTypeServiceMoved": {
    message: "サービスを移動",
    description:
      "Service activity type: the service was reassigned between projects or environments (service_moved) — distinct from the env-var fact above",
  },
  "services.eventsProjectChange": {
    message: "プロジェクト {from} → {to}",
    description:
      "service_moved detail line: previous and new project name (its id once it no longer exists), or the placement-none placeholder (w4/561)",
  },
  "services.eventsEnvironmentChange": {
    message: "環境 {from} → {to}",
    description:
      "service_moved detail line: previous and new environment name (its id once it no longer exists), or the placement-none placeholder (w4/561)",
  },
  "services.eventsPlacementNone": {
    message: "なし",
    description:
      "Placeholder for an absent side of a service_moved event (assigned from / removed to nothing) or of a plan_changed event (no plan before)",
  },
  "services.eventsTypeEnvGroupLinked": {
    message: "環境グループをリンク",
    description: "Service activity type: environment group linked",
  },
  "services.eventsTypeEnvGroupUnlinked": {
    message: "環境グループのリンクを解除",
    description: "Service activity type: environment group unlinked",
  },
  "services.eventsEnvGroupLinkedNamed": {
    message: "環境グループ {name} をリンクしました",
    description:
      "Service activity title for env_group_linked naming the group, by its name at the time (w4/563)",
  },
  "services.eventsEnvGroupUnlinkedNamed": {
    message: "環境グループ {name} のリンクを解除しました",
    description:
      "Service activity title for env_group_unlinked naming the group, by its name at the time (w4/563)",
  },
  "services.eventsCustomDomainAddedNamed": {
    message: "カスタムドメイン {name} を追加しました",
    description:
      "Service activity title for custom_domain_added naming the domain's hostname (w4/m337)",
  },
  "services.eventsCustomDomainRemovedNamed": {
    message: "カスタムドメイン {name} を削除しました",
    description:
      "Service activity title for custom_domain_removed naming the domain's hostname (w4/m337)",
  },
  "services.eventsCustomDomainVerifiedNamed": {
    message: "カスタムドメイン {name} を検証しました",
    description:
      "Service activity title for custom_domain_verified naming the domain's hostname (w4/m337)",
  },
  "services.eventsDisplayNameChange": {
    message: "名前 {from} → {to}",
    description:
      "display_name_changed detail line: the service's name before and after the rename (w4/m337)",
  },
  "services.eventsTypeAutoDeployChanged": {
    message: "自動デプロイを更新",
    description: "Service activity type: auto-deploy setting changed",
  },
  "services.eventsTypeIdleTimeoutChanged": {
    message: "アイドルタイムアウトを更新",
    description: "Service activity type: idle timeout changed",
  },
  "services.eventsTypeDisplayNameChanged": {
    message: "表示名を変更",
    description: "Service activity type: display name changed",
  },
  "services.eventsTypeCustomDomainAdded": {
    message: "カスタムドメインを追加",
    description: "Service activity type: custom domain added",
  },
  "services.eventsTypeCustomDomainRemoved": {
    message: "カスタムドメインを削除",
    description: "Service activity type: custom domain removed",
  },
  "services.eventsTypeCustomDomainVerified": {
    message: "カスタムドメインを検証",
    description: "Service activity type: custom domain verified",
  },
  "services.eventsTypeDiskCreated": {
    message: "ディスクを作成",
    description: "Service activity type: persistent disk created",
  },
  "services.eventsTypeDiskUpdated": {
    message: "ディスクを更新",
    description: "Service activity type: persistent disk updated",
  },
  "services.eventsTypeDiskDeleted": {
    message: "ディスクを削除",
    description: "Service activity type: persistent disk deleted",
  },
  "services.eventsTypeDiskSnapshotRestored": {
    message: "ディスクのスナップショットを復元",
    description:
      "Service activity type: persistent disk restored from a snapshot",
  },
  "services.eventsTypeNotificationsChanged": {
    message: "障害通知を更新",
    description: "Service activity type: failure notification setting changed",
  },
  "services.eventsTypeSubdomainPolicyChanged": {
    message: "プラットフォームのサブドメインを更新",
    description: "Service activity type: platform subdomain policy changed",
  },
  "services.eventsTypeStaticSiteChanged": {
    message: "Static Site の設定を変更",
    description: "Service activity type: static-site configuration changed",
  },
  "services.eventsFilterType.autoDeployEnabled": {
    message: "自動デプロイを有効化",
    description: "Events filter option for the auto deploy enabled event type",
  },
  "services.eventsFilterType.autoDeployDisabled": {
    message: "自動デプロイを無効化",
    description: "Events filter option for the auto deploy disabled event type",
  },
  "services.eventsFilterType.autoDeployChanged": {
    message: "自動デプロイを更新（旧形式）",
    description: "Events filter option for the auto deploy changed event type",
  },
  "services.eventsFilterType.publishPathChanged": {
    message: "公開ディレクトリを変更",
    description: "Events filter option for the publish path changed event type",
  },
  "services.eventsFilterType.routesChanged": {
    message: "リダイレクトとリライトを変更",
    description: "Events filter option for the routes changed event type",
  },
  "services.eventsFilterType.headersChanged": {
    message: "ヘッダーを変更",
    description: "Events filter option for the headers changed event type",
  },
  "services.eventsFilterType.rootDirectoryChanged": {
    message: "ルートディレクトリを変更",
    description:
      "Events filter option for the root directory changed event type",
  },
  "services.eventsFilterType.dockerfilePathChanged": {
    message: "Dockerfile のパスを変更",
    description:
      "Events filter option for the dockerfile path changed event type",
  },
  "services.eventsFilterType.dockerContextChanged": {
    message: "Docker のビルドコンテキストを変更",
    description:
      "Events filter option for the docker context changed event type (w8/m128)",
  },
  "services.eventsFilterType.buildFilterChanged": {
    message: "ビルドフィルターを変更",
    description: "Events filter option for the build filter changed event type",
  },
  "services.eventsFilterType.commandsChanged": {
    message: "コマンドを変更",
    description: "Events filter option for the commands changed event type",
  },
  "services.eventsFilterType.sourceChanged": {
    message: "ソースを変更",
    description: "Events filter option for the source changed event type",
  },
  "services.eventsFilterType.preDeployCommandChanged": {
    message: "プレデプロイコマンドを変更",
    description:
      "Events filter option for the pre deploy command changed event type",
  },
  "services.eventsFilterType.maxShutdownDelayChanged": {
    message: "最大シャットダウン遅延を変更",
    description:
      "Events filter option for the max shutdown delay changed event type",
  },
  "services.eventsFilterType.deployHookRegenerated": {
    message: "Deploy Hook を再生成",
    description:
      "Events filter option for the deploy hook regenerated event type",
  },
  "services.eventsTypeBuildSettingsChanged": {
    message: "ビルドとデプロイの設定を変更",
    description: "Service activity type: build or deploy configuration changed",
  },
  "services.eventsTypePortChanged": {
    message: "ポートを変更",
    description: "Service activity type: the service's listening port changed",
  },
  "services.eventsTypeServiceChanged": {
    message: "サービス設定を変更",
    description: "Fallback service activity type",
  },
  "services.eventsManualDeploy": {
    message: "手動デプロイ",
    description: "Button to trigger a new deploy",
  },
  "services.deployMenuLatestCommit": {
    message: "最新のコミットをデプロイ",
    description:
      "Manual Deploy dropdown item, repo-backed service: rebuild and redeploy from the branch's HEAD",
  },
  "services.deployMenuLatestImage": {
    message: "最新のイメージをデプロイ",
    description:
      "Manual Deploy dropdown item, image-backed service (no repo to rebuild from)",
  },
  "services.deployMenuClearCache": {
    message: "ビルドキャッシュをクリアしてデプロイ",
    description:
      "Manual Deploy dropdown item (Render parity): rebuild without importing prior build-cache layers when registry caching is enabled; still exports a fresh cache for later deploys",
  },
  "services.deployMenuRestart": {
    message: "サービスを再起動",
    description:
      "Manual Deploy dropdown item: restart on the commit or image that is running; never deploys newer commits",
  },
  "services.deployMenuSpecificCommit": {
    message: "特定のコミットをデプロイ",
    description:
      "Manual Deploy dropdown item, repo-backed service (Render parity): deploy a commit by SHA and turn auto-deploy off",
  },
  "services.deployCommitTitle": {
    message: "特定のコミットをデプロイ",
    description: "Title of the dialog that deploys a commit by SHA",
  },
  "services.deployCommitBody": {
    message:
      "入力したコミットをデプロイし、自動デプロイをオフにします。以降のプッシュでこのデプロイが置き換えられることはありません。自動デプロイは設定でオンに戻せます。",
    description:
      "Body of the deploy-a-specific-commit dialog. Render's dashboard disables automatic deploys when a specific commit is deployed",
  },
  "services.deployCommitLabel": {
    message: "コミット SHA",
    description:
      "Label of the commit SHA input in the deploy-a-specific-commit dialog",
  },
  "services.deployCommitInvalid": {
    message: "コミット SHA を入力してください（16 進数の 7～40 文字）。",
    description: "Inline error when the entered commit SHA is malformed",
  },
  "services.deployCommitConfirm": {
    message: "コミットをデプロイ",
    description: "Confirm button of the deploy-a-specific-commit dialog",
  },
  "services.restartServiceSuccess": {
    message: "実行中のリリースで再起動を開始しました。",
    description: "Toast after the header's Restart service is accepted",
  },
  "services.restartServiceError": {
    message: "サービスを再起動できませんでした。",
    description:
      "Toast when the header's Restart service fails and the server gave no reason",
  },
  "services.eventsCancelDeploy": {
    message: "キャンセル",
    description: "Button to cancel an in-progress deploy",
  },
  "services.eventsCancelConfirmTitle": {
    message: "このデプロイをキャンセルしますか？",
    description: "Cancel deploy confirm dialog title",
  },
  "services.eventsCancelConfirmBody": {
    message:
      "進行中のデプロイを停止します。以前のデプロイが成功している場合はそれが引き続き公開され、そうでない場合は何も提供されなくなります。保存した変更は保持され、次回のデプロイで反映されます。",
    description:
      "Cancel deploy confirm dialog body when it is unknown whether an earlier deploy is live, so both outcomes are stated (w4/334)",
  },
  "services.eventsCancelConfirmBodyFirstDeploy": {
    message:
      "進行中のデプロイを停止します。これはサービスの初回デプロイのため、公開中のバージョンがまだありません。キャンセルすると、サービスは何も提供しない状態になります。",
    description:
      "Cancel deploy confirm dialog body — the service's first deploy, nothing is serving yet (w4/103)",
  },
  "services.eventsRollback": {
    message: "ロールバック",
    description: "Button to roll back to a specific deploy",
  },
  "services.eventsRollbackConfirmTitle": {
    message: "このデプロイにロールバックしますか？",
    description: "Rollback confirm dialog title",
  },
  "services.eventsRollbackConfirmBody": {
    message:
      "サービスは、このデプロイのイメージと保持されている設定で実行されます。保存済みの設定は変更されず、次回の通常のデプロイで適用されます。自動デプロイはオフになります。保持されている設定がない古いデプロイでは、イメージのみが復元されます。",
    description: "Rollback confirm dialog body",
  },
  "services.eventsRollbackConfirmBodyCommit": {
    message:
      "サービスは、このデプロイのイメージ（{commit} からビルド）と保持されている設定で実行されます。保存済みの設定は変更されず、次回の通常のデプロイで適用されます。自動デプロイはオフになります。保持されている設定がない古いデプロイでは、イメージのみが復元されます。",
    description:
      "Rollback confirm dialog body naming the commit being restored",
  },
  "services.eventsRollbackConfirmBodyNoAutoDeploy": {
    message:
      "サービスは、このデプロイのイメージと保持されている設定で実行されます。保存済みの設定は変更されず、次回の通常のデプロイで適用されます。保持されている設定がない古いデプロイでは、イメージのみが復元されます。",
    description:
      "Rollback confirm dialog body when the service's auto-deploy is already off (always so for an image-backed service), so no auto-deploy sentence (w4/554)",
  },
  "services.eventsRollbackConfirmBodyCommitNoAutoDeploy": {
    message:
      "サービスは、このデプロイのイメージ（{commit} からビルド）と保持されている設定で実行されます。保存済みの設定は変更されず、次回の通常のデプロイで適用されます。保持されている設定がない古いデプロイでは、イメージのみが復元されます。",
    description:
      "Rollback confirm dialog body naming the commit, when the service's auto-deploy is already off (w4/554)",
  },
  "services.eventsRollbackConfirmBodyStatic": {
    message:
      "このデプロイからサイトを再公開します。保存済みの設定は変更されず、自動デプロイはオフになります。",
    description:
      "Rollback confirm dialog body for a static site: it re-publishes the deploy's files, it has no image or instances (w4/m141)",
  },
  "services.eventsRollbackConfirmBodyStaticCommit": {
    message:
      "このデプロイ（{commit}）からサイトを再公開します。保存済みの設定は変更されず、自動デプロイはオフになります。",
    description:
      "Static-site rollback confirm dialog body naming the commit being re-published (w4/m141)",
  },
  "services.eventsRollbackConfirmTagPull": {
    message:
      "{image} はタグです。ロールバックはこのタグが現在指すイメージを取得します。このデプロイで実行したイメージと異なる場合があり、タグが存在しなくなっているとロールバックは失敗します。",
    description:
      "Rollback confirm sentence for a deploy whose image is a registry tag, not a digest: Render pulls the tag again (render.com/docs/rollbacks, w4/m432)",
  },
  "services.eventsRollbackAria": {
    message: "{target} にロールバック",
    description:
      "Accessible name of a deploy row's Rollback control, naming the deploy it restores (w4/m141)",
  },
  "services.eventsCancelDeployAria": {
    message: "デプロイ {target} をキャンセル",
    description:
      "Accessible name of a deploy row's Cancel control, naming the deploy (w4/m141)",
  },
  "services.eventsConfirmProceed": {
    message: "続行",
    description: "Confirm dialog proceed button",
  },
  "services.eventsConfirmCancel": {
    message: "戻る",
    description: "Confirm dialog cancel button",
  },
  "services.triggerDeploySuccess": {
    message: "デプロイを開始しました。",
    description: "Toast after triggerDeploy succeeds",
  },
  "services.triggerDeployError": {
    message: "デプロイを開始できませんでした。",
    description: "Toast after triggerDeploy fails",
  },
  "services.cancelDeploySuccess": {
    message: "デプロイをキャンセルしました。",
    description: "Toast after cancelDeploy succeeds",
  },
  "services.cancelDeployError": {
    message: "デプロイをキャンセルできませんでした。",
    description: "Toast after cancelDeploy fails",
  },
  "services.rollbackSuccess": {
    message: "ロールバックを開始しました。",
    description: "Toast after rollbackService succeeds",
  },
  "services.rollbackError": {
    message: "ロールバックできませんでした。",
    description: "Toast after rollbackService fails",
  },
  "services.eventsStatusLive": {
    message: "公開中",
    description: "Deploy status: live",
  },
  "services.eventsStatusInProgress": {
    message: "進行中",
    description: "Deploy status: update_in_progress",
  },
  "services.eventsStatusFailed": {
    message: "失敗",
    description: "Deploy status: update_failed",
  },
  "services.eventsStatusCanceled": {
    message: "キャンセル済み",
    description: "Deploy status: canceled",
  },
  "services.eventsPreDeployRunning": {
    message: "プレデプロイコマンドを実行中",
    description: "Deploy row: the pre-deploy step is in progress",
  },
  "services.eventsPreDeploySucceeded": {
    message: "プレデプロイコマンドが成功しました",
    description: "Deploy row: the pre-deploy step passed",
  },
  "services.eventsPreDeployCanceled": {
    message: "プレデプロイコマンドがキャンセルされました",
    description:
      "Deploy row: the deploy ended (superseded or canceled) while its pre-deploy step was still running",
  },
  "services.eventsPreDeployFailed": {
    message: "プレデプロイコマンドが失敗しました",
    description:
      "Deploy row: the pre-deploy step failed (distinct from a health-check failure)",
  },
  "services.eventsRolledBackFrom": {
    message: "{target} からロールバック",
    description: "Deploy row: provenance note when trigger=rollback",
  },
  "services.capLimitTitle": {
    message: "サービス数の上限に達しました",
    description:
      "Alert title when the workspace's service creation cap is hit (w7/m9)",
  },
  "services.capLimitUpgrade": {
    message: "プランをアップグレード",
    description: "Upgrade CTA button inside the cap-limit Alert (w7/m9)",
  },
  "services.networkingTitle": {
    message: "ネットワーク",
    description: "Settings Networking card title (w7/m32)",
  },
  "services.networkingDescription": {
    message:
      "インバウンド HTTP トラフィックを、指定した送信元 CIDR に制限します。",
    description: "Settings Networking card description (w7/m32)",
  },
  "services.networkingScalePlanOnly": {
    message:
      "Web Service と Static Site のインバウンド IP ルールには、Scale プラン以上が必要です。すでに設定されているルールは引き続き適用され、すべて許可のルールに置き換えることができます。",
    description:
      "Hint replacing the editable IP rules below the Scale plan (w8/m137)",
  },
  "services.networkingClearAll": {
    message: "すべてのトラフィックを許可",
    description:
      "Button replacing every inbound IP rule with the allow-all rule below the Scale plan (w8/m137, w8/m157)",
  },
  "services.networkingHint": {
    message:
      "CIDR ブロック（例: 203.0.113.0/24）を入力して「追加」を押してください。ルールがない場合、すべてのインバウンドトラフィックがブロックされます。ルール 0.0.0.0/0 はすべての送信元 IP を許可します。",
    description: "Hint below the CIDR list in the Networking card (w7/m32)",
  },
  "services.networkingOpen": {
    message: "すべての送信元 IP に開放",
    description:
      "Shown below the Scale plan when the service has only the allow-all rule (Render default, w7/m32, w8/m157)",
  },
  "services.networkingBlocked": {
    message:
      "ルールなし: このサービスへのすべてのインバウンドトラフィックがブロックされています。すべての送信元 IP を許可するには 0.0.0.0/0 を追加してください。",
    description:
      "Warning shown when a service's inbound IP rule list is empty, saved or about to be saved (w8/m157)",
  },
  "services.networkingAdd": {
    message: "許可リストのエントリを追加",
    description: "Button to add a CIDR to the draft list (w7/m32)",
  },
  "services.networkingEntryDescription": {
    message: "説明（任意）",
    description: "Placeholder for a service allowlist entry description",
  },
  "services.networkingCIDR": {
    message: "新しい CIDR ブロック",
    description: "Accessible label for the add-row CIDR input",
  },
  "services.networkingCIDRRule": {
    message: "ルール {number} の CIDR ブロック",
    description: "Accessible label for an existing allowlist CIDR row",
  },
  "services.networkingDescriptionLabel": {
    message: "新しいルールの説明",
    description: "Accessible label for the add-row description input",
  },
  "services.networkingDescriptionRule": {
    message: "ルール {number} の説明",
    description: "Accessible label for an existing allowlist description row",
  },
  "services.networkingInvalid": {
    message: "有効な IPv4 または IPv6 の CIDR ブロックを入力してください。",
    description: "Validation error for an invalid service allowlist CIDR",
  },
  "services.networkingSave": {
    message: "許可リストを保存",
    description: "Button to persist the CIDR list (w7/m32)",
  },
  "services.networkingRemove": {
    message: "{cidr} を削除",
    description:
      "Accessible label on the trash icon next to a CIDR tag (w7/m32)",
  },
  "services.networkingMoveUp": {
    message: "{cidr} を上に移動",
    description: "Accessible label to move an allowlist entry earlier",
  },
  "services.networkingMoveDown": {
    message: "{cidr} を下に移動",
    description: "Accessible label to move an allowlist entry later",
  },
  "services.networkingSaved": {
    message: "IP 許可リストを更新しました",
    description: "Toast on successful setServiceIpAllowList mutation (w7/m32)",
  },
  "services.networkingError": {
    message: "IP 許可リストを更新できませんでした。もう一度お試しください。",
    description: "Toast on failed setServiceIpAllowList mutation (w7/m32)",
  },
  "services.outboundIpsTitle": {
    message: "アウトバウンド IP",
    description: "Settings card title for service egress IPs (w8/010)",
  },
  "services.outboundIpsDescription": {
    message:
      "このサービスの共有エグレスアドレスです。テナントプールのノードの現在のパブリック IP であり、オートスケーラーによるノードの追加や削除に伴って変わることがあります。",
    description:
      "Outbound IPs card helper — truthful shared-pool semantics (w8/010)",
  },
  "services.outboundIpsEmpty": {
    message:
      "アウトバウンド IP はまだ報告されていません（ノードに ExternalIP がないローカルクラスターではよくあります）。",
    description: "Honest empty state when outboundIps.ips is empty (w8/010)",
  },
  "services.outboundIpsCopy": {
    message: "{ip} をコピー",
    description:
      "Accessible label on the copy button for one outbound IP (w8/010)",
  },
  "services.outboundIpsCopied": {
    message: "コピーしました",
    description: "Toast after copying an outbound IP (w8/010)",
  },
  "services.outboundIpsCopyError": {
    message: "コピーできませんでした",
    description: "Toast when clipboard write fails for an outbound IP (w8/010)",
  },
  "services.environmentPageDescription": {
    message:
      "このサービスが実行時に読み込むすべての値です。値は表示するまで伏せられます。",
    description: "Service Environment page introduction",
  },
  "services.environmentEdit": {
    message: "編集",
    description: "Enter the coherent service environment draft",
  },
  "services.environmentMaskedValue": {
    message: "マスクされたシークレット値",
    description: "Accessible label for an unrevealed secret",
  },
  "services.environmentUnchangedMasked": {
    message: "変更なし（マスク済み）",
    description: "Placeholder for an opaque unchanged draft value",
  },
  "services.environmentDuplicateKey": {
    message: "各変数のキーは一意である必要があります。",
    description: "Draft validation for a duplicate environment key",
  },
  "services.environmentValueRequired": {
    message: "この新しい変数の値を入力してください。",
    description: "Draft validation for a new variable without a value",
  },
  "services.environmentValueNul": {
    message:
      "値に NUL バイトを含めることはできません。プロセスの環境変数では扱えないためです。バイナリの内容にはシークレットファイルを使用してください。",
    description:
      "Draft validation under an environment variable value with a NUL byte (w21/047)",
  },
  "services.environmentValueTooLarge": {
    message:
      "KEY=value の 1 エントリは最大 131,071 バイトです。これを超えるとプロセスを起動できません。大きなデータにはシークレットファイルを使用してください。",
    description:
      "Draft validation under an environment variable whose KEY=value entry is over the process limit; the server counts UTF-8 bytes, so a Chinese character is 3 (w21/047)",
  },
  "services.environmentPortInvalid": {
    message:
      "PORT は 1 から 65535 までの整数にしてください。プラットフォームが予約している 18012、18013、19099 は使用できません。",
    description:
      "Draft validation under a PORT environment variable the operator would ignore (w21/047)",
  },
  "services.environmentLimit": {
    message:
      "環境変数は、サービスまたはグループごとに最大 500 個、合計 512 KiB までです。",
    description:
      "Draft validation when the environment variables would exceed bex-api's per-service quota",
  },
  "services.environmentStagedDelete": {
    message: "削除されます",
    description: "Badge on a staged environment deletion",
  },
  "services.environmentUndo": {
    message: "元に戻す",
    description: "Undo a staged environment deletion",
  },
  "services.environmentUnsavedTitle": {
    message: "未保存の変更",
    description: "Combined draft save bar heading",
  },
  "services.environmentRestoredTitle": {
    message: "未保存の変更を復元しました",
    description:
      "Heading of the banner shown when an unsaved env draft is recovered in this tab, after a reload or a session-expiry sign-in (w3/m80)",
  },
  "services.environmentRestoredBody": {
    message:
      "未保存だった環境の編集内容を復元しました。内容を確認して、保存または破棄してください。",
    description:
      "Body of the restored-draft banner. Names no cause: an ordinary reload restores the draft as a sign-in does (w4/337)",
  },
  "services.environmentRestoredDiscard": {
    message: "復元した変更を破棄",
    description: "Button that throws away a recovered env draft",
  },
  "services.environmentUnsavedVariables_other": {
    message: "変数 {count} 件を変更",
    description:
      "Draft variable-operation count, composed with the file count at the call site (plural)",
  },
  "services.environmentUnsavedFiles_other": {
    message: "ファイル {count} 件を変更",
    description:
      "Draft file-operation count, composed with the variable count at the call site (plural)",
  },
  "services.environmentSaveOptions": {
    message: "環境の保存オプション",
    description: "Accessible label for the split save menu",
  },
  "services.environmentSaveOnly": {
    message: "保存のみ",
    description: "Persist and project without rolling the service",
  },
  "services.environmentSaveDeploy": {
    message: "保存してデプロイ",
    description: "Persist and roll the current image once",
  },
  "services.environmentSaveRebuild": {
    message: "保存、再ビルド、デプロイ",
    description: "Persist then start one source build and deploy",
  },
  "services.environmentSaveOnlySuccess": {
    message:
      "デプロイせずに環境を保存しました。保留中の変更を反映するには、通常のデプロイを実行してください。",
    description:
      "Save-only success feedback while the operator checks saved/runtime differences; does not assert that a no-op or revert has pending changes",
  },
  "services.environmentSaveDeploySuccess": {
    message: "環境を保存し、デプロイを開始しました",
    description: "Toast after save-and-deploy succeeds",
  },
  "services.environmentSavedRefreshFailed": {
    message:
      "環境を保存しましたが、このページを更新できませんでした。最新の値とデプロイの状態を確認するには、再読み込みしてください。",
    description:
      "A committed environment patch must not be retried because a follow-up read failed",
  },
  "services.environmentSaveError": {
    message: "環境を保存できませんでした。下書きはそのまま残っています。",
    description: "Batch environment save failure",
  },
  "services.environmentDraftStale": {
    message:
      "下書きを作成してから、この環境グループが変更された可能性があるため、下書きは保存されませんでした。必要な内容をコピーして下書きを破棄し、最新の状態から編集し直してください。",
    description:
      "Refusal when a group draft's base revision is no longer current (ENV_GROUP_REVISION_CONFLICT) or predates revision tracking (w4/m161)",
  },
  "services.environmentSavedDeployFailedTitle": {
    message: "設定は保存されましたが、ロールアウトが完了していません",
    description: "Partial rebuild failure heading",
  },
  "services.environmentSavedDeployFailedBody": {
    message:
      "環境の変更は保存されています。準備ができたら、完了していないロールアウトのみを再試行してください。",
    description: "Partial rebuild failure recovery explanation",
  },
  "services.environmentRetryDeploy": {
    message: "ロールアウトを再試行",
    description: "Retry only the failed deploy phase",
  },
  "services.environmentDiscardTitle": {
    message: "環境の変更を破棄しますか？",
    description: "Dirty navigation guard title",
  },
  "services.environmentDiscardBody": {
    message: "これらの変更は保存されておらず、このページを離れると失われます。",
    description: "Dirty navigation guard explanation",
  },
  "services.environmentKeepEditing": {
    message: "編集を続ける",
    description: "Cancel dirty navigation",
  },
  "services.environmentDiscard": {
    message: "破棄して移動",
    description: "Confirm dirty navigation",
  },
  "services.envReveal": {
    message: "表示",
    description: "Reveal one masked environment value",
  },
  "services.envRevealOne": {
    message: "{name} を表示",
    description:
      'aria-label of the per-row Reveal button for one env var or secret file; starts with the visible text "Reveal" (w4/499)',
  },
  "services.envHideOne": {
    message: "{name} を非表示",
    description:
      'aria-label of the per-row Hide button for one env var or secret file; starts with the visible text "Hide" (w4/499)',
  },
  "services.envHide": {
    message: "非表示",
    description: "Mask one revealed environment value",
  },
  "services.envCopy": {
    message: "環境変数をコピー",
    description: "Copy the complete dotenv export",
  },
  "services.envDownload": {
    message: ".env をダウンロード",
    description: "Download the complete dotenv export",
  },
  "services.envCopySuccess": {
    message: "環境をコピーしました",
    description: "Toast after the complete dotenv export is copied",
  },
  "services.envCopyOne": {
    message: "{name} をコピー",
    description:
      "aria-label of the per-row copy button for one env var or secret file — distinct from the bulk envCopy dotenv export",
  },
  "services.envCopyOneSuccess": {
    message: "{name} をコピーしました",
    description:
      "Toast after one env var value or secret file's contents is copied — distinct from the bulk envCopySuccess",
  },
  "services.envAddVariable": {
    message: "変数を追加",
    description: "Add one blank draft variable",
  },
  "services.envAddGenerated": {
    message: "生成されたシークレット",
    description: "Add one previewable generated secret",
  },
  "services.envImport": {
    message: ".env からインポート",
    description: "Open the dotenv import dialog",
  },
  "services.envImportTitle": {
    message: "環境変数をインポート",
    description: "Dotenv import dialog title",
  },
  "services.envImportDescription": {
    message:
      "dotenv の代入を貼り付けるか、テキストファイルを選択してください。インポートした値は、保存するまでこの下書きに保持されます。",
    description: "Dotenv import dialog explanation",
  },
  "services.envImportTextLabel": {
    message: "dotenv の内容",
    description: "Accessible label for the dotenv import textarea",
  },
  "services.envImportPlaceholder": {
    message: "API_URL=https://example.com\nSECRET=replace-me",
    description: "Dotenv import textarea placeholder",
  },
  "services.envImportChooseFile": {
    message: "ファイルを選択",
    description: "Dotenv file picker button",
  },
  "services.envImportAdd": {
    message: "変数を追加",
    description: "Stage parsed dotenv variables",
  },
  "services.envImportLineError": {
    message: "{line} 行目は KEY=value 形式の代入ではありません。",
    description: "Line-numbered dotenv parse error",
  },
  "services.envImportKeyError": {
    message:
      "{line} 行目: 名前は英字、数字、ドット、ハイフン、アンダースコアで 253 文字以内にしてください。",
    description:
      "Dotenv import error: the variable name on a line is invalid (w4/159)",
  },
  "services.envImportQuoteError": {
    message: "{line} 行目で開いた引用符が閉じられていません。",
    description: "Dotenv import error: a quoted value never closes (w4/159)",
  },
  "services.envImportTrailingError": {
    message: "{line} 行目の閉じ引用符の後にテキストがあります。",
    description:
      "Dotenv import error: text after a quoted value's closing quote (w4/159)",
  },
  "services.envImportReading": {
    message: "ファイルを読み込み中…",
    description:
      "Status while a chosen dotenv file is being read; Add waits for it",
  },
  "services.envImportFileError": {
    message: "1 MiB 以下の読み取り可能なテキストファイルを選択してください。",
    description: "Dotenv file read or size error",
  },
  "services.secretFileUpload": {
    message: "ファイルをアップロード",
    description: "Stage multiple local text files",
  },
  "services.secretFileSkippedTitle_other": {
    message: "{count} 件のファイルをスキップしました:",
    description: "Secret-file upload: heading over the skipped files (w4/628)",
  },
  "services.secretFileSkippedName": {
    message:
      "名前に使用できるのは英字、数字、'.'、'-'、'_' のみです（253 文字以内）。",
    description: "Secret-file upload: why a file was skipped — its name",
  },
  "services.secretFileSkippedSize": {
    message: "1 MiB を超えています。",
    description: "Secret-file upload: why a file was skipped — its size",
  },
  "services.secretFileSkippedEncoding": {
    message:
      "UTF-8 のテキストではありません。バイナリファイル（キーストア、.p12、DER 証明書）は、base64 でエンコードしてアップロードし、起動時にデコードしてください。例: base64 -d /etc/secrets/keystore.b64 > /tmp/keystore.p12",
    description: "Secret-file upload: why a file was skipped — not UTF-8 text",
  },
  "services.secretFileSkippedDuplicate": {
    message: "同じ名前のファイルがすでにリストにあります。",
    description: "Secret-file upload: why a file was skipped — duplicate name",
  },
  "services.secretFileDuplicateName": {
    message: "各シークレットファイルの名前は一意である必要があります。",
    description: "Draft validation for a duplicate file name",
  },
  "services.secretFileContentRequired": {
    message: "この新しいファイルの内容を入力してください。",
    description: "Draft validation for missing new-file content",
  },
  "services.secretFileLimit": {
    message:
      "シークレットファイルはサービスまたはグループごとに 500 個まで、合計サイズは 1 MiB までです。",
    description:
      "Draft validation when the secret files would exceed bex-api's per-service quota: 500 files, 1 MiB of contents (names not counted), Render's 1 MB (w4/m363)",
  },
  "services.secretFileViewContent": {
    message: "内容を表示",
    description: "Reveal or inspect secret-file contents in a dialog",
  },
  "services.secretFileViewContentOne": {
    message: "{name} の内容を表示",
    description:
      "Accessible name for View contents once the file has a name (m101/t013)",
  },
  "services.secretFileEditContent": {
    message: "内容を編集",
    description: "Edit staged secret-file contents in a dialog",
  },
  "services.secretFileEditContentOne": {
    message: "{name} の内容を編集",
    description:
      "Accessible name for Edit contents once the file has a name (m101/t013)",
  },
  "services.secretFileContentDialogTitle": {
    message: "{name} の内容",
    description: "Secret-file content dialog title",
  },
  "services.secretFileContentDialogDescription": {
    message:
      "このファイルは、次回のデプロイ後に /etc/secrets へ読み取り専用でマウントされます。",
    description: "Secret-file content dialog explanation",
  },
  "services.secretFileContentDialogDescriptionDocker": {
    message:
      "このファイルは、次回のデプロイ後に /etc/secrets へ読み取り専用でマウントされます。Docker ビルドでも、ビルドコンテキストのルートにこのファイルが含まれます。イメージに含めないようにするには、.dockerignore に記載してください。",
    description:
      "Secret-file content dialog explanation for a Docker-runtime service: the build context also receives the file, as on Render (w8/253)",
  },
  "services.secretFileLoadingContent": {
    message: "内容を読み込み中…",
    description: "Secret-file content reveal progress",
  },
  "services.secretFileContentDone": {
    message: "完了",
    description: "Close and stage secret-file content edits",
  },
  "services.secretFileUntitled": {
    message: "名前のないファイル",
    description: "Fallback title for a new unnamed secret file",
  },
  "services.secretFileDelete": {
    message: "シークレットファイルを削除",
    description: "Accessible label for a staged secret-file deletion",
  },
  "services.secretFileDeleteOne": {
    message: "シークレットファイル {name} を削除",
    description:
      "Accessible name for Delete secret file once the file has a name (m101/t013)",
  },
  "services.envGroupsLinkedTitle": {
    message: "リンクされた環境グループ",
    description: "Service-side environment group card title",
  },
  "services.envGroupsLinkedCount": {
    message: "リンク済み（{count}）",
    description: "Count of groups linked to this service",
  },
  "services.envGroupsAvailableCount": {
    message: "リンク可能（{count}）",
    description: "Count of groups available to link",
  },
  "services.envGroupsNoneLinked": {
    message: "このサービスにリンクされている環境グループはありません。",
    description: "Service group empty linked state",
  },
  "services.envGroupsAvailableTitle": {
    message: "リンク可能",
    description:
      "Available-groups heading while the service's environment is still resolving (no count yet)",
  },
  "services.envGroupsScopeLoading": {
    message: "このサービスの環境を共有しているグループを確認中…",
    description:
      "Available groups wait for the service's environment scope (w4/217)",
  },
  "services.envGroupsOrderReading": {
    message: "優先されるリンク済みグループを確認中…",
    description:
      "Link order is being re-read after a link change; no winner is claimed until it lands (w4/322)",
  },
  "services.envGroupsOrderFailed": {
    message:
      "リンクしましたが、どのグループが優先されるかを確認できませんでした。順序を再読み込みして確認してください。",
    description:
      "The post-write Server read failed; the write stands, precedence stays unclaimed until a retry succeeds (w4/322)",
  },
  "services.envGroupsScopeError": {
    message: "このサービスにリンクできるグループを確認できませんでした。",
    description:
      "Scope index failed; nothing is offered until a retry succeeds (w4/217)",
  },
  "services.envGroupsScopeEnvironmentMissing": {
    message:
      "このサービスの環境が読み込まれた環境リストにないため、リンクできるグループを表示できません。もう一度試して更新してください。",
    description:
      "The service reports an Environment the authorized scope index lacks even after a refresh; never falls back to Workspace (w4/220)",
  },
  "services.envGroupsNoneInScope": {
    message:
      "このサービスの環境を共有している、未リンクのグループはありません。ここで作成するか、グループをこの環境に移動してください。",
    description:
      "Unlinked groups exist, but none in this service's environment scope (w4/217)",
  },
  "services.envGroupsNoneAvailable": {
    message: "ワークスペースのすべての環境グループがすでにリンクされています。",
    description: "Service group empty available state",
  },
  "services.envGroupsNoneAvailableCreate": {
    message:
      "ワークスペースの環境グループはまだありません。変数とファイルを設定し、このサービスを選択済みの状態で作成してください。",
    description: "Service group create-first empty state",
  },
  "services.cronEveryMinute": {
    message: "毎分",
    description: "Cron schedule preview phrase (w4/m188): fires every minute",
  },
  "services.cronEveryNMinutes": {
    message: "{n} 分ごと",
    description:
      "Cron schedule preview phrase (w4/m188): uniform minute step; n is the step",
  },
  "services.cronEveryHour": {
    message: "毎時",
    description:
      "Cron schedule preview phrase (w4/m188): fires at minute 0 of every hour",
  },
  "services.cronEveryNHours": {
    message: "{n} 時間ごと",
    description:
      "Cron schedule preview phrase (w4/m188): uniform hour step; n is the step",
  },
  "services.cronHourlyAtMinute": {
    message: "毎時 {minute} 分",
    description:
      "Cron schedule preview phrase (w4/m188): fires once an hour at the given minute",
  },
  "services.cronDailyAt": {
    message: "毎日 {time}",
    description: "Cron schedule preview phrase (w4/m188): daily at HH:MM (UTC)",
  },
  "services.cronWeeklyAt": {
    message: "毎週{day} {time}",
    description:
      "Cron schedule preview phrase (w4/m188): weekly; day is a services.cronDay* weekday name, time is HH:MM (UTC)",
  },
  "services.cronMonthlyAt": {
    message: "毎月 {day} 日 {time}",
    description:
      "Cron schedule preview phrase (w4/m188): monthly on a day of the month at HH:MM (UTC)",
  },
  "services.cronRunsEveryMinute": {
    message: "毎分",
    description:
      'Cron job header, after "Runs <command>": the schedule every minute (Render\'s cronstrue phrase, w3/m555)',
  },
  "services.cronRunsEveryNMinutes": {
    message: "{n} 分ごと",
    description:
      'Cron job header, after "Runs <command>": every {n} minutes (Render\'s cronstrue phrase, w3/m555)',
  },
  "services.cronRunsEveryHour": {
    message: "毎時",
    description:
      'Cron job header, after "Runs <command>": every hour on the hour (Render\'s cronstrue phrase, w3/m555)',
  },
  "services.cronRunsHourlyAtMinute": {
    message: "毎時 {minute} 分",
    description:
      'Cron job header, after "Runs <command>": hourly at minute {minute} (Render\'s cronstrue phrase, w3/m555)',
  },
  "services.cronRunsEveryNHours": {
    message: "{n} 時間ごとの {minute} 分",
    description:
      'Cron job header, after "Runs <command>": at minute {minute}, every {n} hours (Render\'s cronstrue phrase, w3/m555)',
  },
  "services.cronRunsDailyAt": {
    message: "毎日 UTC {time}",
    description:
      'Cron job header, after "Runs <command>": daily at {time} ("09:00 AM" in English, the 24-hour clock elsewhere), UTC (Render\'s cronstrue phrase, w3/m555)',
  },
  "services.cronRunsWeeklyAt": {
    message: "毎週{day} UTC {time}",
    description:
      'Cron job header, after "Runs <command>": weekly on {day} (a services.cronDay* name) at {time}, UTC (Render\'s cronstrue phrase, w3/m555)',
  },
  "services.cronRunsMonthlyAt": {
    message: "毎月 {day} 日 UTC {time}",
    description:
      'Cron job header, after "Runs <command>": monthly on day {day} at {time}, UTC (Render\'s cronstrue phrase, w3/m555)',
  },
  "services.cronDaySunday": {
    message: "日曜日",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDayMonday": {
    message: "月曜日",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDayTuesday": {
    message: "火曜日",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDayWednesday": {
    message: "水曜日",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDayThursday": {
    message: "木曜日",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDayFriday": {
    message: "金曜日",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDaySaturday": {
    message: "土曜日",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.environmentEditingTitle": {
    message: "環境を編集中",
    description:
      "Edit bar title while the environment draft is open and holds no change yet (the bar says Unsaved changes once it does)",
  },
  "services.environmentNoChanges": {
    message: "まだ変更はありません。",
    description:
      "Edit bar line while the open environment draft holds no change: what to do next",
  },
  "services.envAddOptions": {
    message: "変数を追加するその他の方法",
    description:
      "Accessible name of the chevron beside Add variable that opens the other ways to add (generated secret, .env import)",
  },
  "services.secretFilesReloadWithVariables": {
    message: "上で再試行すると、変数とあわせて再読み込みされます。",
    description:
      "Secret Files card line when both lists failed to load: the variables card's Try again reloads the files too, so this card has no button of its own",
  },
  "services.createSubmitWeb": {
    message: "Web Service をデプロイ",
    description:
      "Create-wizard submit button for a web service (Render's 'Deploy web service')",
  },
  "services.createSubmitPrivate": {
    message: "Private Service をデプロイ",
    description: "Create-wizard submit button for a private service",
  },
  "services.createSubmitWorker": {
    message: "Background Worker をデプロイ",
    description: "Create-wizard submit button for a background worker",
  },
  "services.createSubmitCron": {
    message: "Cron Job をデプロイ",
    description:
      "Create-wizard submit button for a cron job (Render's 'Deploy cron job')",
  },
  "services.createSubmitStatic": {
    message: "Static Site をデプロイ",
    description: "Create-wizard submit button for a static site",
  },
  "services.createSourceEdit": {
    message: "編集",
    description:
      "Button on the chosen repository's summary row that reopens the source picker",
  },
  "services.createSourceEditLabel": {
    message: "ソースを変更",
    description:
      "Accessible name of the Edit button on the chosen repository's summary row ({repo} = owner/name)",
  },
  "services.createBlockedSource": {
    message: "続行するにはソースを選択してください。",
    description:
      "Create-wizard action bar: why Deploy is held while no source is chosen",
  },
  "services.createFieldNameDescription": {
    message: "このサービスの一意な名前。",
    description: "Create-wizard Name row description",
  },
  "services.createFieldRuntimeDescription": {
    message: "このサービスのランタイム環境を選択します。",
    description: "Create-wizard Language/runtime row description",
  },
  "services.createFieldBranchDescription": {
    message: "ビルドしてデプロイする Git ブランチ。",
    description: "Create-wizard Branch row description",
  },
  "services.createFieldBuildCommandDescription": {
    message: "各デプロイの前に、このコマンドでアプリをビルドします。",
    description:
      "Create-wizard Build Command row description (native runtimes)",
  },
  "services.createFieldStartCommandDescription": {
    message: "各デプロイで、このコマンドによりアプリを起動します。",
    description: "Create-wizard Start Command row description",
  },
  "services.createFieldDockerCommandDescription": {
    message:
      "任意：イメージの CMD と ENTRYPOINT を別のコマンドで上書きしてサービスを起動します。",
    description: "Create-wizard Docker Command row description (Advanced)",
  },
  "services.createFieldPlanDescription": {
    message: "サービスを実行するインスタンス。いつでも変更できます。",
    description: "Create-wizard compute plan row description",
  },
  "services.createEnvVarsDescription": {
    message:
      "環境ごとの設定やシークレット（API キーなど）を設定し、コードから読み取ります。",
    description: "Create-wizard Environment Variables card description",
  },
  "services.createBlockedPublishPath": {
    message: "続行するには公開ディレクトリを入力してください。",
    description:
      "Create-wizard action bar: Deploy is held until a static site has a publish directory",
  },
  "services.createBlockedSchedule": {
    message: "続行するには有効なスケジュールを入力してください。",
    description:
      "Create-wizard action bar: Deploy is held until a cron job has a valid schedule",
  },
  "services.createBlockedCommands": {
    message: "続行するにはビルドコマンドと起動コマンドを入力してください。",
    description:
      "Create-wizard action bar: Deploy is held until the native runtime's build and start commands are filled",
  },
  "services.createBlockedPort": {
    message: "続行するには有効なポートを入力してください。",
    description:
      "Create-wizard action bar: Deploy is held while the port is out of range",
  },
  "services.createBuildSourceRegionNotice": {
    message:
      "この Git ビルドソースにリンクされたサービスは、ビルドソースに合わせて {region} に作成されます。",
    description:
      "Create service wizard: notice explaining that a git Build Source determines the new service's region; {region} is the Render region name",
  },
  "services.createBuildSourceAutoDeployHint": {
    message:
      "リンクされたビルドソースの新しいビルドが成功するたびに、このサービスを自動的にデプロイします。",
    description:
      "Create service wizard: Auto-Deploy toggle description when creating from a Build Source",
  },
  "services.buildSourceKindGit": {
    message: "Git リポジトリ",
    description: "Badge or label for a git-backed Build Source",
  },
  "services.buildSourceKindImage": {
    message: "コンテナイメージ",
    description: "Badge or label for an image-backed Build Source",
  },
  "services.sourceBuildSourceIdLabel": {
    message: "ビルドソース ID",
    description: "Accessible field name for copying a Build Source ID",
  },
  "services.sourceBuildSourceIdCopied": {
    message: "ビルドソース ID をコピーしました",
    description: "Toast shown after copying a Build Source ID",
  },
  "services.sourceBuildSourceIdCopyFailed": {
    message: "ビルドソース ID をコピーできませんでした",
    description: "Toast shown when copying a Build Source ID fails",
  },
  "services.deployMenuLatestBuild": {
    message: "最新のビルドをデプロイ",
    description:
      "Manual Deploy dropdown item for a Build-Source-linked service: deploy the latest succeeded build from the Build Source",
  },
  "services.deployMenuSpecificBuild": {
    message: "特定のビルドをデプロイ…",
    description:
      "Manual Deploy dropdown item for a Build-Source-linked service: pick a specific succeeded build (bld-…) to deploy",
  },
  "services.deployBuildTitle": {
    message: "特定のビルドをデプロイ",
    description: "Title of the dialog to pick a specific Build Source build",
  },
  "services.deployBuildBody": {
    message:
      "リンクされたビルドソースから成功したビルドを選択して、このサービスにデプロイします。",
    description:
      "Description in the dialog to pick a specific Build Source build",
  },
  "services.deployBuildConfirm": {
    message: "ビルドをデプロイ",
    description: "Confirm button label in the deploy-a-specific-build dialog",
  },
  "services.deployBuildListLabel": {
    message: "リンクされたビルドソースのビルド一覧",
    description:
      "Accessible label for the build list inside the deploy-a-specific-build dialog",
  },
  "services.deployBuildEmpty": {
    message:
      "このビルドソースにはまだビルドがありません。先にビルドソースでビルドを実行してください。",
    description:
      "Empty state in the deploy-a-specific-build dialog when no builds exist",
  },
  "services.deployBuildLoadError": {
    message: "このビルドソースのビルド一覧を読み込めませんでした。",
    description:
      "Error message in the deploy-a-specific-build dialog when fetching builds fails",
  },
  "services.deployBuildNoSucceeded": {
    message: "デプロイ可能な成功済みビルドはまだありません。",
    description:
      "Notice in the deploy-a-specific-build dialog when builds exist but none succeeded",
  },
  "services.buildStatusCreated": {
    message: "待機中",
    description: "Build Source build status badge: created/queued",
  },
  "services.buildStatusInProgress": {
    message: "ビルド中",
    description: "Build Source build status badge: in_progress",
  },
  "services.buildStatusSucceeded": {
    message: "成功",
    description: "Build Source build status badge: succeeded",
  },
  "services.buildStatusFailed": {
    message: "失敗",
    description: "Build Source build status badge: failed",
  },
  "services.buildStatusCanceled": {
    message: "キャンセル済み",
    description: "Build Source build status badge: canceled",
  },
};

export default jaServices;
