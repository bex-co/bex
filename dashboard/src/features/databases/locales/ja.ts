import type { TranslationEntry } from "@/i18n";

const jaDatabases: Record<string, TranslationEntry> = {
  "databases.resourceType": {
    message: "データベース",
    description: "Managed Postgres resource type used in document titles",
  },
  "databases.statTotal": {
    message: "データベース合計",
    description: "Databases page stat card label",
  },
  "databases.statAvailable": {
    message: "利用可能",
    description: "Databases page stat card label (healthy databases)",
  },
  "databases.statCreating": {
    message: "作成中",
    description: "Databases page stat card label (provisioning databases)",
  },
  "databases.cardTitle": {
    message: "データベース",
    description: "Databases table card title",
  },
  "databases.colName": {
    message: "名前",
    description: "Databases table column header",
  },
  "databases.colStatus": {
    message: "ステータス",
    description: "Databases table column header",
  },
  "databases.colPlan": {
    message: "プラン",
    description: "Databases table column header (compute plan / tier)",
  },
  "databases.colVersion": {
    message: "バージョン",
    description: "Databases table column header (PostgreSQL major version)",
  },
  "databases.colStorage": {
    message: "ストレージ",
    description: "Databases table column header (disk size)",
  },
  "databases.colCreated": {
    message: "作成日時",
    description: "Databases table column header (relative age from createdAt)",
  },
  "databases.colActions": {
    message: "操作",
    description: "Databases table actions column header (screen-reader only)",
  },
  "databases.statusAvailable": {
    message: "利用可能",
    description: "Database status badge (CNPG cluster healthy)",
  },
  "databases.statusCreating": {
    message: "作成中",
    description: "Database status badge (provisioning)",
  },
  "databases.statusRestarting": {
    message: "再起動中",
    description:
      "Database status badge (Render's config_restart: an established store restarting after a config change, restart, or resume — not a first provision)",
  },
  "databases.statusUpgrading": {
    message: "アップグレード中",
    description: "Database status badge (offline PostgreSQL major upgrade)",
  },
  "databases.statusUnavailable": {
    message: "利用不可",
    description: "Database status badge (provisioning failed)",
  },
  "databases.statusSuspended": {
    message: "一時停止中",
    description:
      "Database status badge (hibernated; suspension wins over the status enum)",
  },
  "databases.statusUnknown": {
    message: "不明",
    description: "Database status badge for an unrecognized status",
  },
  "databases.errorTitle": {
    message: "データベースを読み込めませんでした",
    description: "Databases list error card title",
  },
  "databases.errorBody": {
    message:
      "bex-api へのリクエストに失敗しました。接続を確認して、もう一度お試しください。",
    description: "Databases list error card body",
  },
  "databases.emptyTitle": {
    message: "データベースはまだありません",
    description: "Databases list empty state title",
  },
  "databases.emptyBody": {
    message: "最初のマネージド Postgres を作成すると、ここに表示されます。",
    description: "Databases list empty state body",
  },
  "databases.actionsMenu": {
    message: "{name} の操作",
    description: "Accessible label for the per-row actions trigger",
  },
  "databases.actionDelete": {
    message: "削除",
    description: "Row action: permanently delete the database",
  },
  "databases.deleteConfirmTitle": {
    message: "{name} を削除しますか？",
    description: "Delete-confirmation dialog title",
  },
  "databases.deleteConfirmBody": {
    message:
      "この操作は元に戻せません。この Postgres インスタンスを削除してもよろしいですか？",
    description: "Delete-confirmation dialog body (Render-style copy)",
  },
  "databases.deleteConfirmPrompt": {
    message: "確認のため、下に {phrase} と入力してください。",
    description:
      "Body prompt naming the exact 'sudo delete postgres <name>' phrase (rendered bold by SudoCommandField)",
  },
  "databases.deleteCancel": {
    message: "キャンセル",
    description: "Delete-confirmation dialog cancel button",
  },
  "databases.deleteConfirm": {
    message: "データベースを削除",
    description: "Delete-confirmation dialog confirm button",
  },
  "databases.deleteSuccess": {
    message: "{name} を削除しています…",
    description: "Toast after a delete request is accepted",
  },
  "databases.deleteError": {
    message: "{name} を削除できませんでした。もう一度お試しください。",
    description: "Toast when a delete request fails",
  },
  "databases.createTitle": {
    message: "新規 Postgres",
    description: "Create-database dialog title",
  },
  "databases.createDescription": {
    message: "マネージド PostgreSQL インスタンスをプロビジョニングします。",
    description: "Create-database dialog description",
  },
  "databases.fieldName": {
    message: "名前",
    description: "Create-database form field label (database name)",
  },
  "databases.fieldNamePlaceholder": {
    message: "my-database",
    description: "Create-database name input placeholder",
  },
  "databases.fieldNameError": {
    message: "改行や制御文字を含まない、100 文字以内の名前を入力してください。",
    description: "Create-database name validation message",
  },
  "databases.fieldDatabaseName": {
    message: "データベース",
    description: "Create-database physical PostgreSQL database label",
  },
  "databases.fieldDatabaseNamePlaceholder": {
    message: "orders_data",
    description: "Create-database physical PostgreSQL database placeholder",
  },
  "databases.fieldDatabaseUser": {
    message: "ユーザー",
    description: "Create-database physical PostgreSQL owner-role label",
  },
  "databases.fieldDatabaseUserPlaceholder": {
    message: "orders_owner",
    description: "Create-database physical PostgreSQL owner-role placeholder",
  },
  "databases.fieldPhysicalNameReserved": {
    message: "「{name}」は PostgreSQL の予約語です。別の名前を選んでください。",
    description:
      "Create-database / add-user validation for a name the managed PostgreSQL service reserves (postgres, template0/1, streaming_replica, public, none, pg_*, cnpg_*)",
  },
  "databases.fieldPhysicalNameError": {
    message:
      "小文字の英字、数字、アンダースコアを合わせて 63 文字以内で、英字またはアンダースコアで始まる名前を使用してください。",
    description: "Create-database physical PostgreSQL identifier validation",
  },
  "databases.fieldPlan": {
    message: "コンピュートプラン",
    description: "Create-database form field label (plan / tier)",
  },
  "databases.fieldVersion": {
    message: "PostgreSQL バージョン",
    description: "Create-database form field label (major version)",
  },
  "databases.fieldVersionDefault": {
    message: "デフォルト（最新）",
    description: "Create-database version select default option",
  },
  "databases.fieldDisk": {
    message: "ストレージ",
    description: "Create-database form field label (storage size)",
  },
  "databases.fieldDiskFixed": {
    message:
      "このプランには {size} GB の固定ディスクが含まれます。ストレージを指定するには、より大きなプランを選んでください。",
    description:
      "Create-database hint under a plan's fixed disk size, e.g. Free (w4/376)",
  },
  "databases.fieldDiskError": {
    message: "{min} 〜 {max} GB の整数を入力してください。",
    description:
      "Create-database validation message when the disk size is below the plan floor or above the volume cap",
  },
  "databases.fieldPublic": {
    message: "パブリックアクセス",
    description: "Create-database form field label (external endpoint toggle)",
  },
  "databases.fieldPublicHint": {
    message:
      "アクセス制御で制限するまで、任意の IP アドレスからの有効な認証情報による接続を受け付けます。",
    description:
      "Create-database public toggle helper text; the toggle starts on, as a Render database is reachable by default (w4/553)",
  },
  "databases.fieldPublicHintOff": {
    message:
      "外部からの接続はブロックされます。プライベートネットワーク上のサービスは引き続き接続できます。",
    description:
      "Create-database public toggle helper text once switched off: the database is created private (w4/553)",
  },
  "databases.fieldHighAvailability": {
    message: "高可用性",
    description: "Create-database high availability toggle label (w2/m171)",
  },
  "databases.fieldHighAvailabilityHint": {
    message:
      "プライマリに障害が発生した場合に引き継ぐスタンバイを別のノードで実行します。スタンバイはプライマリと同様に課金されます。",
    description:
      "Create-database HA toggle helper text on a plan that offers HA",
  },
  "databases.fieldHighAvailabilityPlanUnsupported": {
    message:
      "{name} は高可用性に対応していません。1 CPU 以上のプランを選択してください。",
    description:
      "Create-database HA toggle helper text when the selected plan is below 1 CPU (Render's rule)",
  },
  "databases.createCancel": {
    message: "キャンセル",
    description: "Create-database dialog cancel button",
  },
  "databases.createSubmit": {
    message: "データベースを作成",
    description: "Create-database dialog submit button",
  },
  "databases.createSuccess": {
    message: "{name} を作成しています…",
    description:
      "Toast after a create request is accepted (provisioning is async)",
  },
  "databases.createError": {
    message: "{name} を作成できませんでした。もう一度お試しください。",
    description: "Toast when a create request fails",
  },
  "databases.capLimitTitle": {
    message: "Postgres の上限に達しました",
    description:
      "Alert title when the workspace's Postgres creation cap is hit (w7/m9)",
  },
  "databases.capLimitUpgrade": {
    message: "プランをアップグレード",
    description:
      "Upgrade CTA button inside the Postgres cap-limit Alert (w7/m9)",
  },
  "databases.metaTitle": {
    message: "詳細",
    description: "Database detail metadata card title",
  },
  "databases.statusReasonClusterFailed": {
    message:
      "データベースクラスターが設定を受け付けなかったため、最新の変更を適用できませんでした。",
    description:
      "Why an unavailable database is unavailable: the operator's ClusterFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonClusterReadFailed": {
    message: "データベースクラスターの状態を読み取れませんでした。",
    description:
      "Why an unavailable database is unavailable: the operator's ClusterReadFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonNetworkPolicyFailed": {
    message: "データベースのネットワークポリシーを適用できませんでした。",
    description:
      "Why an unavailable database is unavailable: the operator's NetworkPolicyFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonDiskAutoscalingFailed": {
    message: "ディスクのオートスケーリングを適用できませんでした。",
    description:
      "Why an unavailable database is unavailable: the operator's DiskAutoscalingFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonExportFailed": {
    message: "データベースのエクスポートを開始できませんでした。",
    description:
      "Why an unavailable database is unavailable: the operator's ExportFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonPoolerFailed": {
    message: "コネクションプーラーをプロビジョニングできませんでした。",
    description:
      "Why an unavailable database is unavailable: the operator's PoolerFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonExtensionsFailed": {
    message:
      "データベースのマネージド拡張機能をインストールできなかったため、まだ利用できません。",
    description:
      "Why an unavailable database is unavailable: the operator's ExtensionsFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonPostUpgradeBackupFailed": {
    message: "バージョンアップグレード後のバックアップを取得できませんでした。",
    description:
      "Why an unavailable database is unavailable: the operator's PostUpgradeBackupFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonMajorVersionUpgradeFailed": {
    message:
      "メジャーバージョンのアップグレードに失敗しました。データベースは現在のバージョンのままです。",
    description:
      "Why an unavailable database is unavailable: the operator's MajorVersionUpgradeFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonStorageShrinkRejected": {
    message:
      "Postgres のストレージは増やすことしかできません。ディスクを割り当て済みの {size} GB より小さくすることはできません。",
    description:
      "Why an unavailable database is unavailable: the operator's StorageShrinkRejected Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonRecoveryUnavailable": {
    message:
      "バックアップストアが設定されていないため、データベースを復元できません。",
    description:
      "Why an unavailable database is unavailable: the operator's RecoveryUnavailable Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonBackupStoreUnavailable": {
    message:
      "データベースのバックアップストレージをセットアップできませんでした。",
    description:
      "Why an unavailable database is unavailable: the operator's BackupStoreUnavailable Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonScheduledBackupFailed": {
    message: "毎日のバックアップをスケジュールできませんでした。",
    description:
      "Why an unavailable database is unavailable: the operator's ScheduledBackupFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.statusReasonScheduledBackupCleanupFailed": {
    message: "毎日のバックアップのスケジュールを削除できませんでした。",
    description:
      "Why an unavailable database is unavailable: the operator's ScheduledBackupCleanupFailed Ready reason (bex-api statusReasonCode)",
  },
  "databases.metaStatus": {
    message: "ステータス",
    description: "Database detail metadata row label",
  },
  "databases.metaPlan": {
    message: "コンピュートプラン",
    description: "Database detail metadata row label",
  },
  "databases.metaVersion": {
    message: "バージョン",
    description: "Database detail metadata row label",
  },
  "databases.versionLabel": {
    message: "PostgreSQL {version}",
    description: "Formatted PostgreSQL major version label",
  },
  "databases.versionUpgradeDowntime": {
    message:
      "アップグレード中は、データベースとすべてのレプリカが一時的に利用できなくなります。",
    description:
      "Version upgrade page: first sentence of the downtime notice under Upgrade this database",
  },
  "databases.versionUpgradeConfirm": {
    message: "PostgreSQL {version} にアップグレード",
    description:
      "Version upgrade page: the button that starts the upgrade of this database",
  },
  "databases.versionUpgradeAccepted": {
    message: "PostgreSQL {version} へのアップグレードを開始しています",
    description: "Toast after a PostgreSQL major-version upgrade is accepted",
  },
  "databases.versionUpgradeError": {
    message:
      "PostgreSQL のアップグレードを開始できませんでした。もう一度お試しください。",
    description: "Fallback error when the PostgreSQL upgrade mutation fails",
  },
  "databases.versionUpgradeDowntimeDetail": {
    message:
      "通常は 1 時間以内に完了します。アップグレードが終わるまで、データベースとレプリカは利用できません。",
    description: "Version upgrade page: second sentence of the downtime notice",
  },
  "databases.versionUpgradeHeading": {
    message: "Postgres バージョンのアップグレード",
    description:
      "Version upgrade page title and document title (Render: Postgres Version Upgrades)",
  },
  "databases.versionUpgradeCurrent": {
    message: "現在のバージョン: PostgreSQL {version}",
    description:
      "Version upgrade page: badge naming the database's current major version",
  },
  "databases.versionUpgradeLatest": {
    message:
      "このデータベースは、サポートされている最新バージョンの PostgreSQL で動作しています。",
    description:
      "Version upgrade page: shown when the database already runs the latest major",
  },
  "databases.versionUpgradeStepsTitle": {
    message: "アップグレードの手順",
    description: "Version upgrade page: heading over the upgrade steps",
  },
  "databases.versionUpgradeStepsDocs": {
    message: "アップグレードの詳細は{docs}をご覧ください。",
    description:
      "Version upgrade page: sentence after the steps heading; {docs} is the See the docs link",
  },
  "databases.versionUpgradeDocsLink": {
    message: "ドキュメント",
    description: "Version upgrade page: link to the upgrade documentation",
  },
  "databases.versionUpgradeChangesTitle": {
    message:
      "アップグレードでは、データベースに次の取り消せない変更が加えられます:",
    description:
      "Version upgrade page: title of the list of changes an upgrade makes for good",
  },
  "databases.versionUpgradeChangePitr": {
    message:
      "ポイントインタイムリカバリはアップグレード時点から始まり直します。アップグレードより前の時点にはデータベースを復旧できません。",
    description:
      "Version upgrade page: irreversible change — point-in-time recovery restarts at the upgrade",
  },
  "databases.versionUpgradeChangeNoDowngrade": {
    message: "データベースを PostgreSQL {version} に戻すことはできません。",
    description:
      "Version upgrade page: irreversible change — no return to the current major",
  },
  "databases.versionUpgradeBlockedTitle": {
    message: "拡張機能がアップグレードをブロックしています",
    description:
      "Version upgrade page: title of the notice listing extensions that block the upgrade",
  },
  "databases.versionUpgradeBlockedBody": {
    message:
      "これらの拡張機能が有効なデータベースはアップグレードできません。先に削除してください (DROP EXTENSION):",
    description:
      "Version upgrade page: explanation above the list of blocking extensions",
  },
  "databases.versionUpgradeStepsLabel": {
    message: "アップグレードの手順",
    description:
      "Version upgrade page: accessible name of the ordered list of upgrade steps",
  },
  "databases.versionUpgradeCloneStep": {
    message: "このデータベースのクローンをアップグレードする (推奨)",
    description:
      "Version upgrade page: title of the first step (Render: Upgrade a clone of this database (recommended))",
  },
  "databases.versionUpgradeCloneBody": {
    message:
      "使い捨てのコピーでテストアップグレードを行い、正常に完了することを確認します。",
    description: "Version upgrade page: why to clone first",
  },
  "databases.versionUpgradeCloneBilling": {
    message:
      "クローンは、同じスペックの他のデータベースと同じように課金されます。テストが終わったら必ずクローンを削除してください。",
    description: "Version upgrade page: clone billing reminder",
  },
  "databases.versionUpgradeCloneAction": {
    message: "このデータベースをクローン",
    description:
      "Version upgrade page: button that creates the point-in-time clone",
  },
  "databases.versionUpgradeClonePitr": {
    message:
      "ポイントインタイムリカバリの準備ができると、データベースをクローンできます。",
    description:
      "Version upgrade page: tooltip while the clone button waits for point-in-time recovery",
  },
  "databases.versionUpgradeCloneCreated": {
    message: "クローンのデータベースを作成しました: {name}",
    description:
      "Version upgrade page: toast after the clone is created; {name} is the clone",
  },
  "databases.versionUpgradeCloneView": {
    message: "Postgres のクローンを表示",
    description: "Version upgrade page: link to the clone's own upgrade page",
  },
  "databases.versionUpgradeCloneError": {
    message: "クローンを作成できませんでした。もう一度お試しください。",
    description:
      "Version upgrade page: fallback error when the clone cannot be created",
  },
  "databases.versionUpgradeThisStep": {
    message: "このデータベースをアップグレードする",
    description:
      "Version upgrade page: title of the step that upgrades this database",
  },
  "databases.versionUpgradeRunning": {
    message: "アップグレード中...",
    description:
      "Version upgrade page: upgrade button label while the upgrade runs",
  },
  "databases.versionUpgradeNotReady": {
    message:
      "データベースが利用可能になり、ポイントインタイムリカバリが初期化されると、アップグレードできるようになります。",
    description:
      "Version upgrade page: tooltip while the upgrade button waits for the database and point-in-time recovery",
  },
  "databases.versionUpgradeReleaseNotes": {
    message: "リリースノートを読む",
    description:
      "Version upgrade page: link to PostgreSQL's release notes for the target major",
  },
  "databases.versionUpgradeStatusLabel": {
    message: "アップグレードのステータスとログ",
    description:
      "Version upgrade page: accessible name of a run's status and logs section",
  },
  "databases.versionUpgradeStateInProgress": {
    message: "進行中",
    description: "Version upgrade page: run state badge",
  },
  "databases.versionUpgradeStateSucceeded": {
    message: "成功",
    description: "Version upgrade page: run state badge",
  },
  "databases.versionUpgradeStateFailed": {
    message: "失敗",
    description: "Version upgrade page: run state badge",
  },
  "databases.versionUpgradeLogsEmpty": {
    message: "検索条件に一致するログがここに表示されます。",
    description:
      "Version upgrade page: empty logs explanation for a finished run",
  },
  "databases.versionUpgradeLogsEmptyRunning": {
    message:
      "検索条件に一致するログがここに表示されます。アップグレード開始後、ログが表示されるまで数分かかる場合があります。",
    description:
      "Version upgrade page: empty logs explanation while the run is in progress",
  },
  "databases.versionUpgradeLastAttempt": {
    message: "前回のアップグレード",
    description:
      "Version upgrade page: title of the card holding the last finished run",
  },
  "databases.versionUpgradeSucceededTitle": {
    message: "アップグレードが完了しました",
    description: "Toast title when an upgrade the page watched succeeds",
  },
  "databases.versionUpgradeSucceededBody": {
    message: "データベースを Postgres {version} に更新しました。",
    description: "Toast body when an upgrade the page watched succeeds",
  },
  "databases.versionUpgradeFailedTitle": {
    message: "アップグレードに失敗しました",
    description: "Toast title when an upgrade the page watched fails",
  },
  "databases.versionUpgradeFailedBannerTitle": {
    message: "アップグレードに失敗しました。",
    description:
      "Banner title on Info and the upgrade page for a failed upgrade nobody dismissed",
  },
  "databases.versionUpgradeFailedBody": {
    message:
      "データベースは現在のバージョンのまま動作しています。アップグレードのログを確認して問題を解決してから、もう一度お試しください。",
    description: "Banner and toast body for a failed upgrade",
  },
  "databases.versionUpgradeDismiss": {
    message: "エラーを閉じる",
    description:
      "Button that acknowledges a failed upgrade (Render: Dismiss error)",
  },
  "databases.versionUpgradeDismissError": {
    message: "エラーを閉じられませんでした。もう一度お試しください。",
    description: "Fallback error when acknowledging a failed upgrade fails",
  },
  "databases.versionUpgradeAvailableTitle": {
    message: "PostgreSQL {version} を利用できます。",
    description: "Info banner title when a newer PostgreSQL major is available",
  },
  "databases.versionUpgradeAvailableBody": {
    message:
      "アップグレードすると、パフォーマンスが向上し、機能も追加されます。",
    description: "Info banner body when a newer PostgreSQL major is available",
  },
  "databases.versionUpgradeViewOptions": {
    message: "アップグレードのオプションを表示",
    description: "Info banner button that opens the version upgrade page",
  },
  "databases.versionUpgradeAvailableDismiss": {
    message: "閉じる",
    description:
      "Accessible name of the button that dismisses the newer-major banner",
  },
  "databases.versionUpgradeLink": {
    message: "バージョンをアップグレード",
    description:
      "Info Version row link when a newer major is available (Render: Upgrade version)",
  },
  "databases.versionUpgradeViewLink": {
    message: "アップグレードを表示",
    description:
      "Info Version row link while an upgrade runs (Render: View upgrade)",
  },
  "databases.versionUpgradeReviewLink": {
    message: "バージョンの詳細を確認",
    description:
      "Info Version row link on the latest major (Render: Review version details)",
  },
  "databases.versionUpgradeCloneOf": {
    message: "このデータベースは {name} のクローンです。",
    description:
      "Banner title on a clone made for a test upgrade; {name} is its source database",
  },
  "databases.versionUpgradeCloneOfBody": {
    message:
      "クローンは、同じスペックの他のデータベースと同じように課金されます。テストが終わったら必ずこのクローンを削除してください。",
    description: "Banner body on a clone made for a test upgrade",
  },
  "databases.versionUpgradeDeleteClone": {
    message: "このクローンを削除",
    description: "Clone banner button that opens the clone's delete action",
  },
  "databases.versionUpgradeGoToOriginal": {
    message: "元のデータベースに移動",
    description: "Clone banner link to the source database's upgrade page",
  },
  "databases.metaDatabaseName": {
    message: "データベース",
    description: "Database detail metadata row label (the normalized db name)",
  },
  "databases.metaDatabaseUser": {
    message: "ユーザー",
    description: "Database detail metadata row label (owner role)",
  },
  "databases.metaStorage": {
    message: "ストレージ",
    description: "Database detail metadata row label (disk size)",
  },
  "databases.diskAutoscalingLabel": {
    message: "ディスクのオートスケーリング",
    description: "Label beside the database disk-usage chart toggle",
  },
  "databases.diskAutoscalingSize": {
    message: "現在 {current} GB · 最大 {cap} GB",
    description: "Current and maximum Postgres disk size beside autoscaling",
  },
  "databases.diskAutoscalingHint": {
    message:
      "使用率が 90% に達すると、ストレージが 50% 拡張されます（5 GB 単位で切り上げ）。増加は元に戻せず、12 時間に 1 回までです。",
    description: "Accessible explanation of Postgres disk autoscaling",
  },
  "databases.diskAutoscalingCurrentSize": {
    message: "現在 {current} GB",
    description: "Current disk size while plan limits are unavailable",
  },
  "databases.diskAutoscalingPlanUnsupported": {
    message:
      "このプランはディスクのオートスケーリングに対応していません。既存の設定はオフにできます。",
    description: "Plan restriction beside the disk autoscaling switch",
  },
  "databases.diskAutoscalingEnabled": {
    message: "ディスクのオートスケーリングを有効にしました。",
    description: "Toast after enabling Postgres disk autoscaling",
  },
  "databases.diskAutoscalingDisabled": {
    message: "ディスクのオートスケーリングを無効にしました。",
    description: "Toast after disabling Postgres disk autoscaling",
  },
  "databases.diskAutoscalingError": {
    message:
      "ディスクのオートスケーリングを更新できませんでした。もう一度お試しください。",
    description: "Toast when the disk-autoscaling mutation fails",
  },
  "databases.metaHighAvailability": {
    message: "高可用性",
    description:
      "Database detail metadata row label (HA — single-instance MVP is No)",
  },
  "databases.metaPublic": {
    message: "パブリックアクセス",
    description: "Database detail metadata row label (external endpoint)",
  },
  "databases.metaExternalHost": {
    message: "外部ホスト",
    description: "Database detail metadata row label (SNI hostname)",
  },
  "databases.metaRegion": {
    message: "リージョン",
    description: "Database detail metadata row label (platform placement)",
  },
  "databases.metaCreated": {
    message: "作成日時",
    description: "Database detail metadata row label (relative age)",
  },
  "databases.metaExpires": {
    message: "有効期限",
    description:
      "Details row: when a free database expires (Render's expiresAt, w8/m126)",
  },
  "databases.expirySoon": {
    message:
      "この無料データベースは {expiresAt} に期限切れとなり、その後は提供が停止されます。",
    description: "Warning within 7 days of a free database's expiry",
  },
  "databases.expiryExpired": {
    message:
      "この無料データベースは期限切れとなり、一時停止されています。データベースとそのデータは {deleteAt} に削除されます。",
    description:
      "Warning on an expired free database during its 14-day grace period",
  },
  "databases.expiryUpgrade": {
    message: "有料プランにアップグレード",
    description: "Link from the expiry warning to the Plan section",
  },
  "databases.expiryHintSoon": {
    message: "まもなく期限切れ",
    description: "List-row hint on a free database within 7 days of expiry",
  },
  "databases.expiryHintExpired": {
    message: "期限切れ",
    description: "List-row hint on an expired free database",
  },
  "databases.statusReasonFreeExpired": {
    message:
      "この無料データベースは期限切れです。有料プランにアップグレードすると復元されます。",
    description:
      "Why a suspended database is suspended: its free plan expired (bex-api statusReasonCode POSTGRES_FREE_EXPIRED, w8/m126)",
  },
  "databases.yes": {
    message: "はい",
    description: "Metadata value for a true boolean field",
  },
  "databases.no": {
    message: "いいえ",
    description: "Metadata value for a false boolean field",
  },
  "databases.connTitle": {
    message: "接続",
    description: "Connection-info panel card title",
  },
  "databases.connDescription": {
    message:
      "接続文字列とデータベースのパスワードです。ご自身で操作したときにのみ表示され、自動的に表示されることはありません。",
    description: "Connection-info panel card description",
  },
  "databases.connReveal": {
    message: "接続情報を表示",
    description: "Button that fetches the connection info on demand",
  },
  "databases.connHide": {
    message: "接続情報を非表示",
    description: "Button that clears the revealed connection info",
  },
  "databases.connLogicalReplicationTitle": {
    message: "論理レプリケーション",
    description:
      "Heading of the logical-replication note on the Connections card",
  },
  "databases.connLogicalReplicationBody": {
    message:
      "デフォルトでは無効です。Pro 以上のワークスペースでは、ストレージが 10 GB 以上のデータベースが、bex の内外のサブスクライバーに変更を公開できます。有効にするには、このデータベースの ID、サブスクライバーが接続に使う認証情報、読み取るスキーマ、すべてのテーブルを公開するかどうかを添えてサポートに連絡してください。その後、このデータベースのユーザーとして CREATE PUBLICATION … FOR TABLE でパブリケーションを作成し、サブスクライバーはプール URL ではなく外部 URL に接続してください。 bex の内外にある別のデータベースをサブスクライブすることもできます。同じ方法で、サブスクリプションを作成する認証情報を添えて依頼し、テーブルを作成してから CREATE SUBSCRIPTION を実行してください（PostgreSQL 15 以前では、サポートがサブスクリプションを作成します）。",
    description:
      "Logical-replication note on the Connections card: off by default, enabled on request (Render's rules)",
  },
  "databases.connPassword": {
    message: "パスワード",
    description: "Connection-info field label (database password)",
  },
  "databases.connShowPassword": {
    message: "パスワードを表示",
    description: "Accessible label to unmask the password",
  },
  "databases.connHidePassword": {
    message: "パスワードを非表示",
    description: "Accessible label to re-mask the password",
  },
  "databases.connInternal": {
    message: "内部接続文字列",
    description: "Connection-info field label (in-cluster URL)",
  },
  "databases.connExternal": {
    message: "外部接続文字列",
    description: "Connection-info field label (public URL, TLS)",
  },
  "databases.connPsql": {
    message: "psql コマンド",
    description: "Connection-info field label (ready-to-run psql command)",
  },
  "databases.connCaLabel": {
    message: "サーバー CA 証明書",
    description: "Connection-info field label (private server CA bundle)",
  },
  "databases.connCaDownload": {
    message: "CA 証明書をダウンロード",
    description: "Button downloading the server CA PEM file",
  },
  "databases.connCaHelp": {
    message:
      "外部接続では、このデータベースの証明書をプライベートなサーバー CA で検証します（sslmode=verify-full は引き続き必須です）。{file} をダウンロードし、保存したファイルをクライアントに指定してください。psql コマンドまたは外部接続文字列の先頭に次を付けます。",
    description:
      "Explanation above the PGSSLROOTCERT example; {file} is the downloaded CA filename",
  },
  "databases.connCaSnippet": {
    message: "ダウンロードした CA を指す環境変数プレフィックスの例",
    description:
      "Accessible label for the copyable PGSSLROOTCERT example snippet",
  },
  "databases.connCertPendingTitle": {
    message: "公的に信頼される証明書を発行中です",
    description:
      "Connections card note title while a public database's Let's Encrypt certificate is still pending (w4/m357)",
  },
  "databases.connCertPendingBody": {
    message:
      "公的に信頼される証明書を発行中です（通常は数分かかります）。それまでは、下の CA または sslmode=require を使って接続してください。発行が完了すると、Render と同様に外部 URL では CA が不要になります。非表示にしてからもう一度表示すると更新されます。",
    description:
      "Connections card note body while the public database certificate is pending",
  },
  "databases.connCertFailedTitle": {
    message: "公的に信頼される証明書がまだ発行されていません",
    description:
      "Connections card note title when issuing a public database's certificate failed (w4/m357)",
  },
  "databases.connCertFailedBody": {
    message:
      "このデータベースの公的に信頼される証明書の発行に失敗しました。bex は自動的に再試行します。その間も CA を使った接続は可能です。下の CA または sslmode=require を使って接続してください。",
    description:
      "Connections card note body when the public database certificate failed to issue",
  },
  "databases.connErrorTitle": {
    message: "接続情報を読み込めませんでした",
    description: "Connection-info panel error title",
  },
  "databases.connErrorBody": {
    message:
      "データベースのプロビジョニング中か、認証情報を表示する権限がない可能性があります。",
    description: "Connection-info panel error body",
  },
  "databases.copied": {
    message: "クリップボードにコピーしました",
    description: "Toast after copying a connection field",
  },
  "databases.copyError": {
    message: "クリップボードにコピーできませんでした",
    description: "Toast when clipboard copy fails",
  },
  "databases.loading": {
    message: "読み込み中…",
    description: "Generic loading placeholder in a detail panel",
  },
  "databases.dangerDelete": {
    message: "データベースを削除",
    description: "Detail-page bottom action: permanently delete the database",
  },
  "databases.dangerRestart": {
    message: "データベースを再起動",
    description: "Detail-page bottom action: rolling restart of the primary",
  },
  "databases.dangerSuspend": {
    message: "データベースを一時停止",
    description:
      "Detail-page bottom action: hibernate the database (stop compute, keep data)",
  },
  "databases.dangerResume": {
    message: "データベースを再開",
    description: "Detail-page bottom action: wake a suspended database",
  },
  "databases.actionSuspend": {
    message: "一時停止",
    description: "Row action: hibernate the database (stop compute, keep data)",
  },
  "databases.actionResume": {
    message: "再開",
    description: "Row action: wake a suspended database",
  },
  "databases.actionRestart": {
    message: "再起動",
    description: "Row action: rolling restart of the primary",
  },
  "databases.suspendConfirmTitle": {
    message: "{name} を一時停止しますか？",
    description: "Suspend-confirmation dialog title",
  },
  "databases.suspendConfirmBody": {
    message:
      "コンピュートが停止し、データベースに接続できなくなりますが、データは保持されます。いつでも再開して元に戻せます。",
    description: "Suspend-confirmation dialog body",
  },
  "databases.restartConfirmTitle": {
    message: "{name} を再起動しますか？",
    description: "Restart-confirmation dialog title",
  },
  "databases.restartConfirmBody": {
    message:
      "プライマリを再起動します。復帰するまで、接続が一時的に切断されます。",
    description: "Restart-confirmation dialog body",
  },
  "databases.toastSuspendSuccess": {
    message: "{name} を一時停止しています…",
    description: "Toast after a suspend request is accepted",
  },
  "databases.toastResumeSuccess": {
    message: "{name} を再開しています…",
    description: "Toast after a resume request is accepted",
  },
  "databases.toastRestartSuccess": {
    message: "{name} を再起動しています…",
    description: "Toast after a restart request is accepted",
  },
  "databases.toastLifecycleError": {
    message: "{name} の操作を完了できませんでした。もう一度お試しください。",
    description: "Toast when a lifecycle verb fails",
  },
  "databases.recoveryTitle": {
    message: "リカバリ",
    description: "Recovery panel card title",
  },
  "databases.recoveryDescription": {
    message:
      "ポイントインタイムリカバリとバックアップです。復元では常に新しいデータベースが作成され、このデータベースが変更されることはありません。",
    description: "Recovery panel card description",
  },
  "databases.recoveryDisabled": {
    message:
      "このプランではバックアップが有効になっていないため、リカバリは利用できません。ポイントインタイムリカバリを有効にするには、バックアップ付きのプランにアップグレードしてください。",
    description: "Recovery panel state when the plan has no backups",
  },
  "databases.recoveryUnavailable": {
    message: "リカバリ情報を読み取れませんでした。もう一度お試しください。",
    description: "Recovery panel state when backup metadata could not be read",
  },
  "databases.recoveryEarliest": {
    message: "最も古い復元ポイント",
    description: "Recovery panel field (earliest recoverable time)",
  },
  "databases.recoveryLatest": {
    message: "最新の復元ポイント",
    description: "Recovery panel field (latest recoverable time)",
  },
  "databases.recoveryWindowNotEstablished": {
    message: "まだ確立されていません",
    description: "Recovery panel value before the PITR window is established",
  },
  "databases.recoveryRestore": {
    message: "新しいインスタンスに復元",
    description: "Recovery panel button that opens the restore dialog",
  },
  "databases.recoveryRestoreUnavailable": {
    message: "最初のリカバリポイントが確立されると、復元が利用可能になります。",
    description: "Reason restore is disabled before the PITR window opens",
  },
  "databases.recoveryCreateExport": {
    message: "エクスポートを作成",
    description: "Recovery panel button that triggers an on-demand export",
  },
  "databases.recoveryBackups": {
    message: "バックアップ",
    description: "Recovery panel backup-list heading",
  },
  "databases.recoveryNoBackups": {
    message: "バックアップはまだありません。",
    description: "Recovery panel empty backup list",
  },
  "databases.recoveryExportScope": {
    message:
      "エクスポートに含まれるのはデータベース {database} のみです。このインスタンスで作成した他のデータベースは含まれません。",
    description:
      "Recovery panel note naming the one database a logical export contains (w8/258)",
  },
  "databases.recoveryExports": {
    message: "エクスポート",
    description: "Recovery panel export-list heading",
  },
  "databases.recoveryNoExports": {
    message: "エクスポートはまだありません。",
    description: "Recovery panel empty export list",
  },
  "databases.recoveryDownloadExport": {
    message: "ダウンロード",
    description: "Download an available logical database export",
  },
  "databases.recoveryDownloadUnavailable": {
    message: "このエクスポートはダウンロードできなくなりました。",
    description: "Toast when Download finds the export is no longer available",
  },
  "databases.recoveryDownloadError": {
    message:
      "ダウンロードリンクを取得できませんでした。もう一度お試しください。",
    description: "Toast when the download link of an export cannot be fetched",
  },
  "databases.recoveryExportInProgress": {
    message: "エクスポートはすでに実行中です。",
    description: "Reason the create-export button is disabled",
  },
  "databases.recoveryExportPreparing": {
    message: "エクスポートはまだ準備中です。",
    description: "Reason an in-progress export cannot be downloaded",
  },
  "databases.recoveryExportExpired": {
    message: "このエクスポートは期限切れとなり、成果物は削除されました。",
    description: "Reason an expired export cannot be downloaded",
  },
  "databases.recoveryExportUnavailable": {
    message: "このエクスポートはダウンロードできません。",
    description: "Fallback reason an export cannot be downloaded",
  },
  "databases.recoveryExportFailure": {
    message: "エクスポートに失敗しました: {reason}",
    description: "Logical export failure reason",
  },
  "databases.recoveryExportStatusCreated": {
    message: "作成済み",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusRunning": {
    message: "実行中",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusAvailable": {
    message: "利用可能",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusFailed": {
    message: "失敗",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusExpiring": {
    message: "まもなく期限切れ",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusExpired": {
    message: "期限切れ",
    description: "Logical export status",
  },
  "databases.recoveryRestoreTitle": {
    message: "新しいデータベースに復元",
    description: "Restore dialog title",
  },
  "databases.recoveryRestoreBody": {
    message:
      "このデータベースのバックアップから復元した新しいデータベースをプロビジョニングします。復元元は変更されません。",
    description: "Restore dialog body",
  },
  "databases.recoveryRestoreName": {
    message: "新しいデータベース名",
    description: "Restore dialog field label",
  },
  "databases.recoveryRestoreTime": {
    message: "時点（任意）",
    description: "Restore dialog target-time field label",
  },
  "databases.recoveryRestoreTimeHint": {
    message: "空欄にすると、利用可能な最新の時点に復元します。",
    description: "Restore dialog target-time helper text",
  },
  "databases.recoveryRestoreTimeZone": {
    message: "時刻はお使いのローカルタイムゾーン（{zone}）で表示されます。",
    description: "Restore dialog target-time zone note",
  },
  "databases.recoveryRestoreTimeOutOfWindow": {
    message:
      "最も古い復元ポイントから 10 分前までの間の時刻を選択してください。",
    description:
      "Restore dialog target-time out-of-window error (the window ends ten minutes before now, w7/m453)",
  },
  "databases.recoveryRetentionWindow_other": {
    message: "{plan} プランでは過去 {count} 日間の任意の時点に復元できます。",
    description:
      "Recovery panel hint: how far back the workspace plan may restore a Postgres (w7/m453, from bex-api databaseRecoveryInfo.retention)",
  },
  "databases.recoveryRetentionUpgrade_other": {
    message: "{plan} プランでは {count} 日間です。",
    description:
      "Recovery panel hint: the cheapest plan that restores further back, and its days (w7/m453)",
  },
  "databases.recoveryRestoreConfirm": {
    message: "復元",
    description: "Restore dialog confirm button",
  },
  "databases.recoveryExportStarted": {
    message: "エクスポートを開始しました。",
    description: "Toast after an export is triggered",
  },
  "databases.recoveryExportError": {
    message: "エクスポートを開始できませんでした。もう一度お試しください。",
    description: "Toast when an export request fails",
  },
  "databases.recoveryRestoreStarted": {
    message: "{name} に復元しています…",
    description: "Toast after a restore request is accepted",
  },
  "databases.recoveryRestoreError": {
    message: "復元できませんでした。もう一度お試しください。",
    description: "Toast when a restore request fails",
  },
  "databases.accessTitle": {
    message: "アクセス制御",
    description: "Access-control panel card title",
  },
  "databases.accessDescription": {
    message:
      "外部エンドポイントの制限、接続のプーリング、データベースユーザーの管理を行います。",
    description: "Access-control panel card description",
  },
  "databases.accessAllowList": {
    message: "IP 許可リスト",
    description:
      "Access panel section heading (external-endpoint CIDR allowlist)",
  },
  "databases.accessAllowListHint": {
    message:
      "CIDR ルールを保存すると、一致する送信元 IP からの外部アクセスが有効になります。空のリストを保存すると外部接続がブロックされます。内部アクセスは変更されません。",
    description: "Access panel IP allowlist helper text",
  },
  "databases.accessAllowListInternalOnly": {
    message:
      "外部接続は無効です。アクセスを有効にするには、IP ルールを 1 つ以上保存してください。",
    description: "External access is disabled; saving rules enables it",
  },
  "databases.accessAllowListEmpty": {
    message:
      "IP ルールはありません。空のリストを保存すると、外部接続がブロックされます。",
    description: "Empty datastore allowlist draft and the effect of saving it",
  },
  "databases.accessAllowListAdd": {
    message: "追加",
    description: "Access panel button to add a CIDR to the draft allowlist",
  },
  "databases.accessAllowListDescription": {
    message: "説明（任意）",
    description:
      "Placeholder for the optional per-entry allowlist description input",
  },
  "databases.accessAllowListCIDR": {
    message: "新しい CIDR ブロック",
    description: "Accessible label for the add-row CIDR input",
  },
  "databases.accessAllowListCIDRRule": {
    message: "ルール {number} の CIDR ブロック",
    description: "Accessible label for an existing allowlist CIDR row",
  },
  "databases.accessAllowListDescriptionLabel": {
    message: "新しいルールの説明",
    description: "Accessible label for the add-row description input",
  },
  "databases.accessAllowListDescriptionRule": {
    message: "ルール {number} の説明",
    description: "Accessible label for an existing allowlist description row",
  },
  "databases.accessAllowListRemove": {
    message: "{cidr} を削除",
    description: "Accessible label to remove a CIDR chip",
  },
  "databases.accessAllowListMoveUp": {
    message: "{cidr} を上へ移動",
    description: "Accessible label to move an allowlist entry earlier",
  },
  "databases.accessAllowListMoveDown": {
    message: "{cidr} を下へ移動",
    description: "Accessible label to move an allowlist entry later",
  },
  "databases.accessAllowListInvalid": {
    message:
      "重複しない有効な IPv4 または IPv6 の CIDR ブロックを入力してください。",
    description: "Validation error for database allowlist entries",
  },
  "databases.accessAllowListSave": {
    message: "許可リストを保存",
    description: "Access panel button to persist the allowlist",
  },
  "databases.accessAllowListSaved": {
    message: "許可リストを更新しました。",
    description: "Toast after saving the IP allowlist",
  },
  "databases.accessAllowListError": {
    message: "許可リストを保存できませんでした。もう一度お試しください。",
    description: "Toast when saving the allowlist fails",
  },
  "databases.accessCredentials": {
    message: "認証情報",
    description:
      "Access panel section heading (Postgres credentials, Render's Credentials section)",
  },
  "databases.accessCredentialsHint": {
    message:
      "新しいデフォルトの認証情報を追加すると、データベースのパスワードをローテーションできます。接続情報は新しい認証情報に切り替わり、リンクされたサービスは次回の Blueprint 同期で反映されます。古い認証情報は、削除するまで引き続き使用できます。",
    description: "Access panel credentials helper text",
  },
  "databases.accessCredentialDefault": {
    message: "デフォルト",
    description: "Badge on the database's default credential",
  },
  "databases.accessCredentialAdd": {
    message: "新しいデフォルトの認証情報",
    description:
      "Access panel button to create a credential that becomes the default",
  },
  "databases.accessCredentialName": {
    message: "新しい認証情報のユーザー名",
    description: "Accessible label for the new-credential username input",
  },
  "databases.accessCredentialDelete": {
    message: "{name} を削除",
    description: "Accessible label to delete a database credential",
  },
  "databases.accessCredentialDeleteTitle": {
    message: "認証情報 {name} を削除しますか？",
    description:
      "Title of the confirmation before deleting a database credential (w4/551)",
  },
  "databases.accessCredentialDeleteBody": {
    message:
      "{name} を使用しているすべての接続は直ちに失敗します。その接続文字列を環境変数に保持しているサービスも含まれます。この操作は元に戻せません。",
    description:
      "Body of the credential delete confirmation: retiring the login breaks its clients at once (w4/551)",
  },
  "databases.accessCredentialDeleteConfirm": {
    message: "認証情報を削除",
    description:
      "Confirm button of the credential delete confirmation (w4/551)",
  },
  "databases.accessCredentialDefaultLocked": {
    message:
      "デフォルトの認証情報は削除できません。先に新しいデフォルトの認証情報を追加してください。",
    description: "Why the delete button is disabled on the default credential",
  },
  "databases.accessCredentialCreated": {
    message:
      "{name} が新しいデフォルトの認証情報になりました。パスワードは接続情報に表示されます。",
    description: "Toast after creating a credential",
  },
  "databases.accessCredentialError": {
    message: "認証情報を作成できませんでした。もう一度お試しください。",
    description: "Toast when creating a credential fails",
  },
  "databases.accessCredentialDeleted": {
    message: "{name} の失効をリクエストしました。",
    description:
      "Toast after deleting a credential: the login is retired asynchronously on the next reconcile, so this confirms accepted intent, not a completed revocation",
  },
  "databases.accessCredentialDeleteError": {
    message: "{name} を削除できませんでした。もう一度お試しください。",
    description: "Toast when deleting a credential fails",
  },
  "databases.accessPooler": {
    message: "コネクションプーリング",
    description: "Access panel section heading (PgBouncer pooler strings)",
  },
  "databases.accessPoolerHint": {
    message:
      "接続数の多いクライアント向けに、PgBouncer 経由でルーティングされるプール接続文字列です。",
    description: "Access panel pooler helper text",
  },
  "databases.accessPoolerReveal": {
    message: "プール接続文字列を表示",
    description: "Access panel button to fetch the pooled connection strings",
  },
  "databases.accessPoolerDisabled": {
    message:
      "プール接続文字列を取得するには、上のコネクションプーリングをオンにしてください。",
    description:
      "Access panel shown when no pooler is enabled; points at the Connection Pool switch in the section heading (w2/m307)",
  },
  "databases.accessPoolerInternal": {
    message: "内部プール接続",
    description: "Access panel pooled-string label (in-cluster)",
  },
  "databases.accessPoolerPlanUnsupported": {
    message:
      "このプランはコネクションプーリングに対応していません。プーラーを有効にする前に、対応しているプランを選択してください。",
    description: "Pooler empty state on an unsupported plan",
  },
  "databases.accessPoolerExternal": {
    message: "外部プール接続",
    description: "Access panel pooled-string label (public)",
  },
  "databases.accessPoolerReplicaInternal": {
    message: "内部プール接続(リードレプリカ {name})",
    description:
      "Access panel pooled-string label for a read replica's in-cluster pooled connection (w6/m458)",
  },
  "databases.accessPoolerReplicaExternal": {
    message: "外部プール接続(リードレプリカ {name})",
    description:
      "Access panel pooled-string label for a read replica's public pooled connection (w6/m458)",
  },
  "databases.accessPoolerError": {
    message: "プール接続文字列を読み込めませんでした。",
    description: "Toast when revealing pooled strings fails",
  },
  "databases.accessPoolerStarting": {
    message:
      "コネクションプーリングを起動しています。PgBouncer の準備ができると、プール接続文字列がここに表示されます。",
    description:
      "Access panel when pooling is on but the operator has not published the pooler hosts yet (w2/m307)",
  },
  "databases.poolerToggleLabel": {
    message: "コネクションプール",
    description:
      "Accessible label of the Connection Pool switch (Render's Info > Connection Pool, w2/m307)",
  },
  "databases.poolerEnableTitle": {
    message: "{name} のコネクションプーリングを有効にしますか？",
    description:
      "Connection-pool enable confirmation title; {name} is the database",
  },
  "databases.poolerDisableTitle": {
    message: "{name} のコネクションプーリングを無効にしますか？",
    description:
      "Connection-pool disable confirmation title; {name} is the database",
  },
  "databases.poolerEnableBody": {
    message:
      "コネクションプーリングを有効にするとデータベースが再起動され、数分間のダウンタイムが発生します。PgBouncer の準備ができると、プール接続文字列がこのセクションに表示されます。",
    description:
      "Connection-pool enable confirmation body for a database without high availability (Render: enabling restarts the database)",
  },
  "databases.poolerEnableBodyHA": {
    message:
      "コネクションプーリングを有効にすると、データベースは再起動されず、スタンバイにフェイルオーバーします。そのため、開いている接続は一時的に切断されます。PgBouncer の準備ができると、プール接続文字列がこのセクションに表示されます。",
    description:
      "Connection-pool enable confirmation body when high availability is on (Render: a failover instead of a restart)",
  },
  "databases.poolerDisableBody": {
    message:
      "コネクションプーリングを無効にするとデータベースが再起動され、数分間のダウンタイムが発生します。プール接続文字列は使用できなくなります。",
    description:
      "Connection-pool disable confirmation body for a database without high availability",
  },
  "databases.poolerDisableBodyHA": {
    message:
      "コネクションプーリングを無効にすると、データベースは再起動されず、スタンバイにフェイルオーバーします。そのため、開いている接続は一時的に切断されます。プール接続文字列は使用できなくなります。",
    description:
      "Connection-pool disable confirmation body when high availability is on",
  },
  "databases.poolerEnableConfirm": {
    message: "コネクションプールを有効にする",
    description: "Connection-pool enable confirmation button (Render's label)",
  },
  "databases.poolerDisableConfirm": {
    message: "コネクションプールを無効にする",
    description: "Connection-pool disable confirmation button",
  },
  "databases.poolerEnableSuccess": {
    message: "{name} のコネクションプーリングを有効にしています。",
    description: "Toast after the connection-pool enable was accepted",
  },
  "databases.poolerDisableSuccess": {
    message: "{name} のコネクションプーリングを無効にしています。",
    description: "Toast after the connection-pool disable was accepted",
  },
  "databases.poolerEnableError": {
    message: "{name} のコネクションプーリングを有効にできませんでした。",
    description: "Fallback toast when enabling the connection pool fails",
  },
  "databases.poolerDisableError": {
    message: "{name} のコネクションプーリングを無効にできませんでした。",
    description: "Fallback toast when disabling the connection pool fails",
  },
  "databases.haTitle": {
    message: "高可用性",
    description: "HA panel card title",
  },
  "databases.haDescription": {
    message:
      "自動フェイルオーバー付きのレプリケーションクラスターです。有効にすると、常にスタンバイが引き継ぎ可能な状態で待機します。",
    description: "HA panel card description",
  },
  "databases.haStatus": {
    message: "ステータス",
    description: "HA panel status row label",
  },
  "databases.haEnabled": {
    message: "有効",
    description: "HA panel status value when HA is active",
  },
  "databases.haDisabled": {
    message: "無効",
    description: "HA panel status value when HA is off",
  },
  "databases.haFailover": {
    message: "フェイルオーバーを実行",
    description: "HA panel button to initiate a planned switchover",
  },
  "databases.haFailoverConfirmTitle": {
    message: "{name} でフェイルオーバーを実行しますか？",
    description: "Failover-confirmation dialog title",
  },
  "databases.haFailoverConfirmBody": {
    message:
      "スタンバイをプライマリに昇格します。切り替え中は接続が一時的に切断されます。データベースは利用可能なままで、データが失われることはありません。",
    description: "Failover-confirmation dialog body",
  },
  "databases.haFailoverCancel": {
    message: "キャンセル",
    description: "Failover-confirmation dialog cancel button",
  },
  "databases.haFailoverConfirm": {
    message: "フェイルオーバーを実行",
    description: "Failover-confirmation dialog confirm button",
  },
  "databases.haFailoverSuccess": {
    message: "{name} でフェイルオーバーを実行しました。",
    description: "Toast after a failover request is accepted",
  },
  "databases.haFailoverError": {
    message:
      "{name} でフェイルオーバーを実行できませんでした。もう一度お試しください。",
    description: "Toast when a failover request fails",
  },
  "databases.haReadReplicas": {
    message: "リードレプリカ",
    description: "HA panel section heading for named read replicas",
  },
  "databases.haReadReplicasEmpty": {
    message: "名前付きのリードレプリカはありません。",
    description: "HA panel empty state for read replicas",
  },
  "databases.haReplicaInternal": {
    message: "内部",
    description: "HA panel replica connection label (in-cluster host)",
  },
  "databases.haReplicaExternal": {
    message: "外部",
    description: "HA panel replica connection label (public SNI host)",
  },
  "databases.haAddReplica": {
    message: "リードレプリカを追加",
    description:
      "HA panel button that opens the add-read-replica dialog (w5/m138)",
  },
  "databases.haAddReplicaTitle": {
    message: "リードレプリカを追加",
    description: "Add-read-replica dialog title",
  },
  "databases.haAddReplicaBody": {
    message:
      "リードレプリカは、プライマリからストリーミングされる別個の課金対象インスタンスで、独自のアドレスで読み取り専用クエリを処理します。",
    description: "Add-read-replica dialog description",
  },
  "databases.haReplicaNameLabel": {
    message: "名前",
    description: "Add-read-replica dialog name field label",
  },
  "databases.haReplicaNameHint": {
    message:
      "小文字の英字、数字、ハイフンを使用し、{max} 文字以内で入力してください。レプリカのホスト名の一部になります。",
    description: "Add-read-replica name hint; max is the API's length limit",
  },
  "databases.haReplicaNameInvalid": {
    message:
      "小文字の英字、数字、ハイフンを 1〜{max} 文字で入力し、先頭と末尾は英字または数字にしてください。",
    description:
      "Add-read-replica name error; mirrors the API's POSTGRES_READ_REPLICA_NAME_INVALID",
  },
  "databases.haReplicaNameDuplicate": {
    message:
      "このデータベースには、{name} という名前のリードレプリカがすでにあります。",
    description: "Add-read-replica name error for a name already in use",
  },
  "databases.haAddReplicaSubmit": {
    message: "レプリカを追加",
    description: "Add-read-replica dialog submit button",
  },
  "databases.haAddReplicaSuccess": {
    message: "リードレプリカ {name} を作成しています。",
    description: "Toast after a read replica is added",
  },
  "databases.haAddReplicaError": {
    message: "リードレプリカ {name} を追加できませんでした。",
    description: "Toast when adding a read replica fails",
  },
  "databases.haRemoveReplica": {
    message: "削除",
    description: "HA panel per-replica remove button",
  },
  "databases.haRemoveReplicaTitle": {
    message: "リードレプリカ {name} を削除しますか？",
    description: "Remove-read-replica confirmation title",
  },
  "databases.haRemoveReplicaBody": {
    message:
      "インスタンスが削除され、接続文字列は使用できなくなります。プライマリには影響しません。",
    description: "Remove-read-replica confirmation body",
  },
  "databases.haRemoveReplicaConfirm": {
    message: "レプリカを削除",
    description: "Remove-read-replica confirm button",
  },
  "databases.haRemoveReplicaSuccess": {
    message: "リードレプリカ {name} を削除しています。",
    description: "Toast after a read replica is removed",
  },
  "databases.haRemoveReplicaError": {
    message: "リードレプリカ {name} を削除できませんでした。",
    description: "Toast when removing a read replica fails",
  },
  "databases.haReplicaPlanUnsupported": {
    message: "このデータベースのプランでは、リードレプリカを利用できません。",
    description: "Why Add read replica is disabled: the plan allows none",
  },
  "databases.haReplicaLimitReached": {
    message:
      "このデータベースのリードレプリカ数は上限の {max} 個に達しています。",
    description:
      "Why Add read replica is disabled: the plan's replica limit is reached",
  },
  "databases.haReplicaStorageTooSmall": {
    message:
      "リードレプリカには {min} GB 以上のストレージが必要です。このデータベースは {current} GB です。",
    description:
      "Why Add read replica is disabled: storage is below the replica minimum",
  },
  "databases.haNotEnabled": {
    message: "このデータベースでは高可用性が有効になっていません。",
    description: "HA panel line when HA is off on a plan that offers it",
  },
  "databases.haPlanUnsupported": {
    message:
      "高可用性には 1 CPU 以上のプランが必要です。このデータベースのプランでは利用できません。",
    description:
      "HA panel state when HA is off and the current plan is below 1 CPU (w8/m43, Render's rule)",
  },
  "databases.haToggleLabel": {
    message: "高可用性",
    description: "Accessible name of the HA panel's on/off switch (w2/m171)",
  },
  "databases.haStarting": {
    message:
      "別のノードでスタンバイを起動しています。準備ができるとフェイルオーバーを利用できます。",
    description:
      "HA panel line while HA is on but the standby is not ready yet",
  },
  "databases.haStopping": {
    message: "高可用性をオフにしています。スタンバイを削除しています。",
    description: "HA panel line while HA is off but the standby still runs",
  },
  "databases.haEnableTitle": {
    message: "{name} の高可用性を有効にしますか？",
    description: "HA enable confirmation title",
  },
  "databases.haEnableBody": {
    message:
      "別のノードでスタンバイインスタンスが起動し、プライマリに障害が発生すると自動的に引き継ぎます。プライマリと同様に課金されます。",
    description: "HA enable confirmation body",
  },
  "databases.haEnablePrice": {
    message: "スタンバイにより {price} が追加されます。",
    description:
      "HA enable confirmation price note; {price} is a formatted monthly price",
  },
  "databases.haEnableConfirm": {
    message: "高可用性を有効にする",
    description: "HA enable confirmation button",
  },
  "databases.haDisableTitle": {
    message: "{name} の高可用性を無効にしますか？",
    description: "HA disable confirmation title",
  },
  "databases.haDisableBody": {
    message:
      "スタンバイが削除されます。プライマリに障害が発生した場合、復旧するまでデータベースは利用できません。",
    description: "HA disable confirmation body",
  },
  "databases.haDisableConfirm": {
    message: "高可用性を無効にする",
    description: "HA disable confirmation button",
  },
  "databases.haEnableSuccess": {
    message: "{name} の高可用性を有効にしています。",
    description: "Toast after HA was enabled",
  },
  "databases.haDisableSuccess": {
    message: "{name} の高可用性を無効にしています。",
    description: "Toast after HA was disabled",
  },
  "databases.haEnableError": {
    message: "{name} の高可用性を有効にできませんでした。",
    description: "Fallback toast when enabling HA fails",
  },
  "databases.haDisableError": {
    message: "{name} の高可用性を無効にできませんでした。",
    description: "Fallback toast when disabling HA fails",
  },
  "databases.planPickerHAUnsupported": {
    message:
      "{name} は高可用性に対応していません（1 CPU 以上が必要です）。先に高可用性を無効にしてください。",
    description:
      "Plan picker refusal when an HA database selects a plan below 1 CPU; mirrors the API message (w8/m43)",
  },
  "databases.planPickerStorageUnsupported": {
    message:
      "{name} で使用できるストレージは最大 {max} GB です。このデータベースは {current} GB で、縮小できません。十分なストレージのあるプランを選択してください。",
    description: "Plan change blocked by the database's existing storage",
  },
  "databases.planPickerReplicasUnsupported": {
    message:
      "{name} のリードレプリカ数の上限は {max} 個です。このデータベースには {current} 個あります。",
    description: "Plan change blocked by the database's read replicas",
  },
  "databases.planPickerAutoscalingUnsupported": {
    message:
      "{name} はディスクのオートスケーリングに対応していません。先にディスクのオートスケーリングをオフにしてください。",
    description: "Plan change blocked while disk autoscaling is enabled",
  },
  "databases.planPickerPoolerUnsupported": {
    message:
      "{name} はコネクションプーリングに対応していません。先にプーラーを無効にしてください。",
    description: "Plan change blocked while a connection pooler is enabled",
  },
  "databases.planTitle": {
    message: "コンピュートプラン",
    description: "Database detail plan-picker card title",
  },
  "databases.planDescription": {
    message:
      "コンピュートプランを変更します。スタンバイのないデータベースは、適用のために再起動され、1〜2 分間利用できなくなります。データは保持されます。",
    description: "Database detail plan-picker card description",
  },
  "databases.planPickerSave": {
    message: "保存",
    description: "Database plan-picker save button",
  },
  "databases.planPickerCancel": {
    message: "キャンセル",
    description: "Database plan-picker cancel / reset button",
  },
  "databases.reviewComputeLead": {
    message: "コンピュートとストレージの選択内容を確認してください。",
    description:
      "Postgres plan review: the lead of a compute plan change's review (Render's copy)",
  },
  "databases.reviewStorageLead": {
    message: "ストレージの選択内容を確認してください。",
    description:
      "Postgres plan review: the lead of a storage change's review (Render's copy)",
  },
  "databases.reviewEditHighAvailability": {
    message: "高可用性を編集",
    description:
      "Postgres plan review: accessible name of the High availability line's Edit button, which goes to the High Availability section",
  },
  "databases.reviewReplicaCount_other": {
    message: "{count} 個のレプリカ",
    description:
      "Postgres plan review: the Read replicas line's figure, the database's replica count",
  },
  "databases.reviewStorageInstances_other": {
    message: "{size} GB（{count} インスタンス分）",
    description:
      'Postgres plan review: the Storage line\'s figure when the size is billed on several instances (primary, HA standby, read replicas); Render writes "GB (x3 instances)"',
  },
  "databases.reviewUnavailableSeconds": {
    message: "更新中、データベースは数秒間利用できなくなります。",
    description:
      "Postgres plan review warning: a database with high availability fails over to its standby, so the outage is seconds (Render's copy)",
  },
  "databases.reviewUnavailableMinutes": {
    message: "更新中、データベースは数分間利用できなくなります。",
    description:
      "Postgres plan review warning: a database without high availability restarts its only instance, so the outage is minutes (Render's copy)",
  },
  "databases.reviewUnavailableReplicasSeconds": {
    message: "更新中、データベースとそのレプリカは数秒間利用できなくなります。",
    description:
      "Postgres plan review warning: as reviewUnavailableSeconds, for a database with read replicas, which restart too (Render's copy)",
  },
  "databases.reviewUnavailableReplicasMinutes": {
    message: "更新中、データベースとそのレプリカは数分間利用できなくなります。",
    description:
      "Postgres plan review warning: as reviewUnavailableMinutes, for a database with read replicas, which restart too (Render's copy)",
  },
  "databases.reviewUpdateCompute": {
    message: "コンピュートを更新",
    description:
      "Postgres plan review: the button that applies a compute plan change (Render's copy)",
  },
  "databases.reviewUpdateStorage": {
    message: "ストレージを更新",
    description:
      "Postgres plan review: the button that applies a storage change (Render's copy)",
  },
  "databases.planPickerSuccess": {
    message: "プランを {name} に更新しています…",
    description: "Toast after a plan update is accepted",
  },
  "databases.planPickerError": {
    message: "プランを更新できませんでした。もう一度お試しください。",
    description: "Toast when a plan update fails",
  },
  "databases.storageTitle": {
    message: "ストレージ",
    description:
      "Plan section Storage row heading (Render's Plan page Storage section, w2/m307)",
  },
  "databases.storageSize": {
    message: "{size} GB",
    description: "Plan section Storage row: the allocated storage",
  },
  "databases.storageFixed": {
    message:
      "無料データベースのストレージは固定です。ストレージを追加するには、有料プランにアップグレードしてください。",
    description:
      "Why the Storage Edit button is disabled on a free database (bex-api POSTGRES_STORAGE_PLAN_UNSUPPORTED)",
  },
  "databases.storageAlmostFull": {
    message:
      "データベースの空き容量がほとんどありません。データベースの読み書きでエラーが発生したり、自動バックアップが実行できなくなったりする可能性があります。データベースの空き領域を確保するか、{action}ことを検討してください。",
    description:
      "Render's almost-full warning at the top of a Postgres database's Info, Plan and Recovery pages (w3/m656): more than 80% of its storage used and less than 5 GiB left. {action} is databases.storageAlmostFullIncrease or databases.storageAlmostFullUpgrade, drawn as a link to the Plan page, or databases.storageAlmostFullSupport",
  },
  "databases.storageAlmostFullIncrease": {
    message: "ストレージを増やす",
    description:
      "The almost-full warning's ending for storage that can grow: a link to the Plan page's Storage control (Render's copy)",
  },
  "databases.storageAlmostFullUpgrade": {
    message: "ストレージの大きいコンピュートプランにアップグレードする",
    description:
      "The almost-full warning's ending for a plan whose storage is fixed: a link to the Plan page's plan picker (Render's copy)",
  },
  "databases.storageAlmostFullSupport": {
    message: "サポートに連絡してカスタムのコンピュートプランを作成する",
    description:
      "The almost-full warning's ending for storage already at its plan's maximum, which no plan grows further (Render's copy); plain text",
  },
  "databases.storageEdit": {
    message: "編集",
    description:
      "Plan section Storage row button that opens the storage dialog",
  },
  "databases.storageEditAria": {
    message: "ストレージを編集",
    description: "Accessible name of the Storage row's Edit button",
  },
  "databases.storageEditTitle": {
    message: "{name} のストレージを編集",
    description: "Storage dialog title; {name} is the database",
  },
  "databases.storageEditBody": {
    message:
      "現在の {current} GB より大きい、{step} GB の倍数を入力してください。一度増やしたストレージは縮小できません。",
    description:
      "Storage dialog description (Render: storage only grows, in 5 GB steps)",
  },
  "databases.storageEditLabel": {
    message: "ストレージ (GB)",
    description: "Storage dialog size field label",
  },
  "databases.storageEditStep": {
    message: "{step} GB の整数倍を入力してください。",
    description: "Storage dialog validation: not a whole multiple of 5 GB",
  },
  "databases.storageEditNotGreater": {
    message:
      "現在の {current} GB より大きい値を入力してください。ストレージは縮小できません。",
    description:
      "Storage dialog validation: the size is not above the current storage",
  },
  "databases.storageEditTooLarge": {
    message: "{name} で使用できるストレージは最大 {max} GB です。",
    description:
      "Storage dialog validation: the size is above the plan's maximum; {name} is the plan",
  },
  "databases.storageEditSave": {
    message: "保存",
    description: "Storage dialog submit button",
  },
  "databases.storageEditSuccess": {
    message: "ストレージを {size} GB に増やしています…",
    description: "Toast after a storage increase is accepted",
  },
  "databases.storageEditError": {
    message: "ストレージを更新できませんでした。もう一度お試しください。",
    description:
      "Storage dialog fallback error when the server gives no reason",
  },
  "databases.nameEdit": {
    message: "データベース名を編集",
    description: "Accessible label for the database-name edit-in-place pencil",
  },
  "databases.nameDescription": {
    message:
      "表示名を変更します。データベース ID とすべての接続情報は変わりません。",
    description: "Database detail rename row description",
  },
  "databases.nameSuccess": {
    message: "データベース名を {name} に変更しました。",
    description: "Toast after a database rename succeeds",
  },
  "databases.nameError": {
    message: "データベース名を変更できませんでした。もう一度お試しください。",
    description: "Toast when a database rename fails unexpectedly",
  },
  "databases.nameConflict": {
    message: "この名前のデータベースは、このワークスペースにすでに存在します。",
    description: "Toast when a database rename collides with another name",
  },
  "databases.nameInvalid": {
    message: "改行や制御文字を含まない、100 文字以内の名前を入力してください。",
    description: "Database rename validation message",
  },
  "databases.insightsTitle": {
    message: "インサイト",
    description: "Database insights panel card title",
  },
  "databases.insightsDescription": {
    message:
      "プロセス、クエリのパフォーマンス、ストレージ、設定をリアルタイムで確認できます。",
    description: "Database insights panel card description",
  },
  "databases.insightsRefresh": {
    message: "インサイトを更新",
    description: "Accessible label for the insights refresh button",
  },
  "databases.insightsUnavailable": {
    message:
      "利用できません — データベースがまだプロビジョニング中の可能性があります。",
    description: "Insights sub-section error state",
  },
  "databases.insightsSizeTitle": {
    message: "ストレージ",
    description: "Insights panel — database/table sizes sub-section heading",
  },
  "databases.insightsNoSizes": {
    message: "サイズのデータはまだありません。",
    description: "Insights storage empty state",
  },
  "databases.insightsProcessesTitle": {
    message: "アクティブなプロセス",
    description: "Insights panel — pg_stat_activity sub-section heading",
  },
  "databases.insightsNoProcesses": {
    message: "アクティブなプロセスはありません。",
    description: "Insights processes empty state",
  },
  "databases.insightsTopQueriesTitle": {
    message: "上位のクエリ",
    description: "Insights panel — pg_stat_statements sub-section heading",
  },
  "databases.insightsNoTopQueries": {
    message:
      "クエリはまだありません。アプリケーションや SQL コンソールで実行したステートメントがここに表示されます。",
    description:
      "Insights top-queries empty state: the read succeeded and nothing the owner ran is in the list. A failed read, including a missing pg_stat_statements, shows insightsUnavailable instead, so this text must not blame the extension",
  },
  "databases.insightsQueryHidden": {
    message: "非表示",
    description:
      "Insights — query cell for a row whose SQL PostgreSQL hides from the reading role",
  },
  "databases.insightsMaskedNote": {
    message:
      "一部の行は他のデータベースロールに属しているため、PostgreSQL はこのビューで SQL を非表示にしています。それ以外の項目は実際の値です。",
    description:
      "Insights — explanation shown when at least one process or top-query row has masked SQL",
  },
  "databases.insightsTableScansTitle": {
    message: "テーブルスキャン",
    description: "Insights panel — pg_stat_user_tables sub-section heading",
  },
  "databases.insightsNoTableScans": {
    message: "スキャン統計のあるユーザーテーブルはまだありません。",
    description: "Insights table-scans empty state",
  },
  "databases.insightsParamsDeclaredValue": {
    message: "宣言された値",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsObservedValue": {
    message: "観測された設定",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsObservedHint": {
    message:
      "現在の設定です。リクエストした値が適用されたことを確認するものではありません。",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsNotObserved": {
    message:
      "リクエストされたオーバーライドは確認できません。保留中か無効な可能性があります。",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsObservationUnavailable": {
    message: "現在、このオーバーライドを確認できません。",
    description: "Declared PostgreSQL parameter observation diagnostic.",
  },
  "databases.insightsParamsTitle": {
    message: "パラメーターのオーバーライド",
    description:
      "Insights panel — heading for the parameters this database declares (read-only)",
  },
  "databases.insightsParamsDescription": {
    message:
      "パラメーターのオーバーライドが廃止される前に、このデータベースで宣言された設定です。引き続き適用されますが、ここでは変更できません。セッションのデフォルトは、SQL コンソールで ALTER DATABASE … SET を使って設定してください。",
    description:
      "Explains that the declared parameters are read-only legacy settings that still apply, and how to set session defaults instead",
  },
  "databases.insightsEffectiveParamsTitle": {
    message: "デフォルト以外のパラメーター",
    description: "Insights panel — pg_settings sub-section heading",
  },
  "databases.insightsEffectiveParamsDescription": {
    message:
      "プラットフォームが管理するものを含め、現在有効でデフォルトから変更されているすべての設定です。読み取り専用です。",
    description:
      "Explains that the pg_settings table is observed configuration, mostly platform-owned, and cannot be edited",
  },
  "databases.insightsNoParams": {
    message: "すべてのパラメーターがデフォルトです。",
    description: "Insights parameter-overrides empty state",
  },
  "databases.insightsColTable": {
    message: "テーブル",
    description: "Insights table column header (table name)",
  },
  "databases.insightsColRelation": {
    message: "テーブルまたはインデックス",
    description:
      "Insights sizes column header (a table's or an index's name; tables and indexes are listed together)",
  },
  "databases.insightsIndexOn": {
    message: "{table} のインデックス",
    description:
      "Insights sizes: suffix naming the table an index row belongs to; {table} is the quoted schema-qualified table name",
  },
  "databases.insightsColSize": {
    message: "サイズ",
    description: "Insights table column header (relation size)",
  },
  "databases.insightsColUser": {
    message: "ユーザー",
    description: "Insights processes column header (pg role)",
  },
  "databases.insightsColState": {
    message: "状態",
    description: "Insights processes column header (active/idle/…)",
  },
  "databases.insightsColDuration": {
    message: "実行時間",
    description: "Insights processes column header (seconds since query start)",
  },
  "databases.insightsColQuery": {
    message: "クエリ",
    description: "Insights column header (SQL text, truncated)",
  },
  "databases.insightsColCalls": {
    message: "呼び出し回数",
    description: "Insights top-queries column header (invocation count)",
  },
  "databases.insightsColTotalTime": {
    message: "合計時間",
    description: "Insights top-queries column header (total exec time)",
  },
  "databases.insightsColMeanTime": {
    message: "平均時間",
    description: "Insights top-queries column header (mean exec time per call)",
  },
  "databases.insightsColSeqScans": {
    message: "シーケンシャルスキャン",
    description: "Insights table-scans column header (sequential scan count)",
  },
  "databases.insightsColIdxScans": {
    message: "インデックススキャン",
    description: "Insights table-scans column header (index scan count)",
  },
  "databases.insightsColLiveRows": {
    message: "有効行数",
    description:
      "Insights table-scans column header (estimated live row count)",
  },
  "databases.insightsColParam": {
    message: "パラメーター",
    description:
      "Insights parameter-overrides column header (pg_settings.name)",
  },
  "databases.insightsColSetting": {
    message: "設定値",
    description:
      "Insights parameter-overrides column header (pg_settings.setting)",
  },
  "databases.insightsColSource": {
    message: "ソース",
    description:
      "Insights parameter-overrides column header (pg_settings.source)",
  },
  "databases.sqlTitle": {
    message: "SQL コンソール",
    description: "Database detail SQL console card title",
  },
  "databases.sqlDescription": {
    message:
      "このデータベースに対して SQL ステートメントを 1 つ実行します。結果は 500 行までに制限され、クエリは 10 秒でタイムアウトします。",
    description: "Database SQL console safety summary",
  },
  "databases.sqlEditorLabel": {
    message: "SQL クエリ",
    description: "Accessible label for the SQL editor",
  },
  "databases.sqlShortcut": {
    message: "Ctrl+Enter または ⌘+Enter で実行",
    description: "SQL console keyboard shortcut hint",
  },
  "databases.sqlCreating": {
    message:
      "このデータベースはまだ作成中です。SQL は利用可能になると実行できます。クエリと履歴は保持されます。",
    description: "SQL console notice while the database is creating (w4/177)",
  },
  "databases.sqlUpgrading": {
    message:
      "PostgreSQL のオフラインアップグレードを実行中です。データベースが利用可能になると、SQL を再び実行できます。クエリと履歴は保持されます。",
    description: "SQL console notice while the database is upgrading (w4/177)",
  },
  "databases.sqlRestarting": {
    message:
      "このデータベースは再起動中です。利用可能になるまで待ってから SQL を実行してください。クエリと履歴は保持されます。",
    description: "SQL console notice while the database is restarting (w4/177)",
  },
  "databases.sqlSuspended": {
    message:
      "このデータベースは一時停止中です。SQL を実行するには再開してください。クエリと履歴は保持されます。",
    description: "SQL console notice while the database is suspended",
  },
  "databases.sqlRun": {
    message: "クエリを実行",
    description: "SQL console run button",
  },
  "databases.sqlRunning": {
    message: "実行中…",
    description: "SQL console run button while a query is executing",
  },
  "databases.sqlUnknownError": {
    message: "クエリを実行できませんでした。もう一度お試しください。",
    description: "SQL console fallback error",
  },
  "databases.sqlReturnedRows_other": {
    message: "{count} 行が返されました",
    description: "SQL console SELECT result count (plural)",
  },
  "databases.sqlAffectedRows_other": {
    message: "{count} 行が影響を受けました",
    description: "SQL console write result count (plural)",
  },
  "databases.sqlTruncated": {
    message: "結果は {limit} 行までに制限されています",
    description: "SQL console backend row-cap notice",
  },
  "databases.sqlNull": {
    message: "NULL",
    description: "SQL console null cell value",
  },
  "databases.sqlNullLabel": {
    message: "SQL NULL（値なし）",
    description:
      "SQL console accessible meaning of a null cell, distinct from the literal text NULL",
  },
  "databases.sqlNoRows": {
    message: "クエリの結果は 0 行でした。",
    description: "SQL console empty result message",
  },
  "databases.sqlPrevious": {
    message: "前へ",
    description: "SQL result pagination previous button",
  },
  "databases.sqlNext": {
    message: "次へ",
    description: "SQL result pagination next button",
  },
  "databases.sqlPage": {
    message: "{page} / {pages} ページ",
    description: "SQL result pagination position",
  },
  "databases.sqlHistory": {
    message: "クエリ履歴",
    description: "SQL console session history heading",
  },
  "databases.sqlClearHistory": {
    message: "クリア",
    description: "SQL console clear-history button",
  },
  "databases.sqlNoHistory": {
    message: "このブラウザーセッションで実行したクエリがここに表示されます。",
    description: "SQL console empty history message",
  },
  "databases.sqlConfirmTitle": {
    message: "データを変更する可能性のあるステートメントを実行しますか？",
    description: "SQL console write confirmation title",
  },
  "databases.sqlConfirmDescription": {
    message:
      "このステートメントは読み取り専用と認識されませんでした。データベースのデータを変更または削除する可能性があり、bex では元に戻せません。",
    description: "SQL console write confirmation warning",
  },
  "databases.sqlConfirmCancel": {
    message: "キャンセル",
    description: "SQL console write confirmation cancel button",
  },
  "databases.sqlConfirmRun": {
    message: "ステートメントを実行",
    description: "SQL console confirmed write action",
  },
  // --- Sidebar and page headings (w3/m641) ---
  "databases.navInfo": {
    message: "情報",
    description:
      "Postgres sidebar entry and page heading: the database's Info page (general details, connections, access, SQL console)",
  },
  "databases.navLogs": {
    message: "ログ",
    description:
      "Postgres sidebar entry and page heading: the database's Logs page",
  },
  "databases.navMetrics": {
    message: "メトリクス",
    description:
      "Postgres sidebar entry and page heading: the database's Metrics page",
  },
  "databases.navPlan": {
    message: "プラン",
    description:
      "Postgres sidebar entry and page heading: the database's Plan page (compute plan, storage, high availability)",
  },
  "databases.navApps": {
    message: "アプリ",
    description:
      "Postgres sidebar entry and page heading: the database's Apps page (pgAdmin, PgHero)",
  },
  "databases.navRecovery": {
    message: "リカバリ",
    description:
      "Postgres sidebar entry and page heading: the database's Recovery page (point-in-time recovery, exports)",
  },
  "databases.logsRangeLabel": {
    message: "期間",
    description: "Managed Postgres log range filter label",
  },
  "databases.logsInstanceLabel": {
    message: "データベースインスタンス",
    description: "Managed Postgres log instance filter label",
  },
  "databases.logsAllInstances": {
    message: "すべてのインスタンス",
    description: "Managed Postgres log instance filter all option",
  },
  "databases.logsSearchPlaceholder": {
    message: "データベースログを検索",
    description: "Managed Postgres log search placeholder",
  },
  "databases.logsLoading": {
    message: "データベースログを読み込み中…",
    description: "Managed Postgres logs loading state",
  },
  "databases.logsEmptyTitle": {
    message: "データベースログはまだありません",
    description: "Managed Postgres logs empty-state title",
  },
  "databases.logsEmptyBody": {
    message: "この期間の Postgres の出力がここに表示されます。",
    description: "Managed Postgres logs empty-state body",
  },
  "databases.logsUnavailableTitle": {
    message: "データベースログが設定されていません",
    description: "Managed Postgres logs unavailable-state title",
  },
  "databases.logsUnavailableBody": {
    message:
      "このインストールでは、永続的なログ履歴も、CNPG Pod のライブログソースも設定されていません。",
    description: "Managed Postgres logs unavailable-state body",
  },
  "databases.logsUnauthorizedTitle": {
    message: "これらのログを表示できません",
    description: "Managed Postgres logs unauthorized-state title",
  },
  "databases.logsUnauthorizedBody": {
    message:
      "データベースログの表示には、このワークスペースでのコントリビューター権限が必要です。",
    description: "Managed Postgres logs unauthorized-state body",
  },
  "databases.logsErrorTitle": {
    message: "データベースログを読み込めませんでした",
    description: "Managed Postgres logs generic error title",
  },
  "databases.fieldNameDescription": {
    message: "Postgres インスタンスの一意な名前。",
    description: "Create-Postgres Name row description",
  },
  "databases.fieldDatabaseNameDescription": {
    message:
      "Postgres の dbname。作成時にのみ設定できます。空欄の場合は自動で生成されます。",
    description:
      "Create-Postgres Database row description: the physical dbname, create-time only",
  },
  "databases.fieldDatabaseUserDescription": {
    message:
      "データベースを所有する Postgres ユーザー。作成時にのみ設定できます。空欄の場合は自動で生成されます。",
    description:
      "Create-Postgres User row description: the physical owner role, create-time only",
  },
  "databases.fieldPlanDescription": {
    message:
      "データベースのメモリと CPU を設定します。コンピュートは後から変更できます。",
    description: "Create-Postgres plan row description (Render's Compute)",
  },
  "databases.fieldDiskDescription": {
    message:
      "データベースの容量 (GB)。後から増やせますが、減らすことはできません。",
    description:
      "Create-Postgres Storage row description (Render: storage only grows)",
  },
  "databases.fieldDiskUnit": {
    message: "GB",
    description: "Unit after the create-Postgres storage input",
  },
  "databases.fieldVersionDescription": {
    message: "データベースで実行する PostgreSQL のメジャーバージョン。",
    description: "Create-Postgres version row description",
  },
  // --- Apps: admin apps (Render's Postgres Apps page, w4/m551) ---
  "databases.adminAppsTitle": {
    message: "アプリ",
    description:
      "Database page Apps section title and navigation label (Render's Postgres Apps page)",
  },
  "databases.adminAppsDescription": {
    message:
      "このデータベースと同じリージョンと環境に、管理アプリを Web サービスとしてデプロイし、プライベートネットワーク経由で接続します。",
    description: "Database page Apps section description",
  },
  "databases.adminAppsErrorTitle": {
    message: "アプリを読み込めませんでした",
    description: "Database Apps section load error title",
  },
  "databases.adminAppsErrorBody": {
    message:
      "このデータベースのアプリを読み込む際に問題が発生しました。しばらくしてからもう一度お試しください。",
    description: "Database Apps section load error body",
  },
  "databases.adminAppPgAdminDescription": {
    message:
      "汎用の PostgreSQL 管理ツールです。スキーマ、テーブル、インデックスを管理し、単発のクエリを実行してクエリプランを確認できます。",
    description: "pgAdmin card description (Render's copy)",
  },
  "databases.adminAppPgHeroDescription": {
    message:
      "データベースのパフォーマンスダッシュボードです。リソース使用量を監視し、アクティブな接続を分析し、最近のクエリを調べてボトルネックを特定できます。",
    description: "PgHero card description (Render's copy)",
  },
  "databases.adminAppDocs": {
    message: "ドキュメントを読む",
    description: "Admin app card link to the app's documentation",
  },
  "databases.adminAppDeploy": {
    message: "アプリをデプロイ",
    description: "Admin app card button that opens the deploy dialog",
  },
  "databases.adminAppOpen": {
    message: "アプリを開く",
    description:
      "Deployed admin app card button that opens the app in a new tab",
  },
  "databases.adminAppManage": {
    message: "管理",
    description: "Deployed admin app card link to its service page",
  },
  "databases.adminAppDialogTitle": {
    message: "{app} アプリを作成",
    description: "Admin app deploy dialog title; {app} is pgAdmin or PgHero",
  },
  "databases.adminAppDialogDescription": {
    message:
      "データベースと同じリージョンと環境に新しい Web サービスをデプロイします。他の Web サービスと同じように設定できます。",
    description: "Admin app deploy dialog body",
  },
  "databases.adminAppAuthentication": {
    message: "認証",
    description: "Admin app deploy dialog login section heading",
  },
  "databases.adminAppAuthenticationHint": {
    message:
      "{app} インスタンスのユーザー名とパスワードを自動生成しました。以下で変更できます。",
    description:
      "Admin app deploy dialog login section text; {app} is the app name",
  },
  "databases.adminAppUsername": {
    message: "ユーザー名",
    description: "Admin app deploy dialog username field label",
  },
  "databases.adminAppPassword": {
    message: "パスワード",
    description: "Admin app deploy dialog password field label",
  },
  "databases.adminAppShow": {
    message: "{field}を表示",
    description:
      "Accessible label of the button that unmasks a login field; {field} is its label",
  },
  "databases.adminAppHide": {
    message: "{field}を隠す",
    description:
      "Accessible label of the button that masks a login field; {field} is its label",
  },
  "databases.adminAppMonthlyCharges": {
    message: "月額料金",
    description: "Admin app deploy dialog billing section heading",
  },
  "databases.adminAppMonthlyChargesHint": {
    message:
      "Web サービスのインスタンスは秒単位の日割りで請求されます。以下は {app} の料金のみです。",
    description:
      "Admin app deploy dialog billing section text; {app} is the app name",
  },
  "databases.adminAppBillingStarter": {
    message: "Web サービス - Starter インスタンス",
    description:
      "Admin app deploy dialog billing line for a Starter web service",
  },
  "databases.adminAppBillingFree": {
    message: "Web サービス - Free インスタンス",
    description: "Admin app deploy dialog billing line for a Free web service",
  },
  "databases.adminAppBillingDisk": {
    message: "ディスク - {size}GB",
    description:
      "Admin app deploy dialog billing line for its disk; {size} is in GB",
  },
  "databases.adminAppBillingPrice": {
    message: "${usd} / 月",
    description:
      "Admin app deploy dialog billing line price; {usd} is a dollar amount",
  },
  "databases.adminAppDeploySubmit": {
    message: "{app} をデプロイ",
    description: "Admin app deploy dialog submit button; {app} is the app name",
  },
  "databases.adminAppDeployed": {
    message: "{app} を {service} としてデプロイしました。",
    description:
      "Toast after an admin app deploys; {service} is its new service's name",
  },
  "databases.adminAppDeployError": {
    message: "アプリを作成できませんでした。もう一度お試しください。",
    description: "Admin app deploy dialog generic failure",
  },
  "databases.adminAppPaymentRequired": {
    message:
      "{app} をデプロイするには、このワークスペースに支払い方法を追加してください。",
    description:
      "Admin app deploy dialog message when billing onboarding was closed without a payment method",
  },
  "databases.adminAppErrorTitle": {
    message: "エラー",
    description: "Admin app deploy dialog error title",
  },
  "databases.connectInternalTitle": {
    message: "内部データベース URL",
    description:
      "Header Connect menu, Internal tab: the private-network URL's title (Render's copy, w3/m675)",
  },
  "databases.connectExternalTitle": {
    message: "外部データベース URL",
    description:
      "Header Connect menu, External tab: the public URL's title (Render's copy, w3/m675)",
  },
  "databases.connectCliTitle": {
    message: "PSQL コマンド",
    description:
      "Header Connect menu, External tab: the `bex psql <id>` command's title (Render's copy, w3/m675)",
  },
};

export default jaDatabases;
