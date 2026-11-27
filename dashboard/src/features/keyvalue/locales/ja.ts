import type { TranslationEntry } from "@/i18n";

const jaKeyValue: Record<string, TranslationEntry> = {
  "keyvalue.resourceType": {
    message: "Key Value",
    description: "Key Value resource type used in document titles",
  },
  "keyvalue.statTotal": {
    message: "Key Value ストア合計",
    description: "Key Value page stat card label",
  },
  "keyvalue.statAvailable": {
    message: "利用可能",
    description: "Key Value page stat card label (healthy stores)",
  },
  "keyvalue.statCreating": {
    message: "作成中",
    description: "Key Value page stat card label (provisioning stores)",
  },
  "keyvalue.cardTitle": {
    message: "Key Value",
    description: "Key Value table card title",
  },
  "keyvalue.colName": {
    message: "名前",
    description: "Key Value table column header",
  },
  "keyvalue.colStatus": {
    message: "ステータス",
    description: "Key Value table column header",
  },
  "keyvalue.colPlan": {
    message: "コンピュートプラン",
    description: "Key Value table column header (plan / tier)",
  },
  "keyvalue.colVersion": {
    message: "バージョン",
    description: "Key Value table column header (Valkey version)",
  },
  "keyvalue.colCreated": {
    message: "作成日時",
    description: "Key Value table column header (relative age from createdAt)",
  },
  "keyvalue.colActions": {
    message: "操作",
    description: "Key Value table actions column header (screen-reader only)",
  },
  "keyvalue.statusAvailable": {
    message: "利用可能",
    description: "Key Value status badge (Valkey instance healthy)",
  },
  "keyvalue.statusCreating": {
    message: "作成中",
    description: "Key Value status badge (provisioning)",
  },
  "keyvalue.statusRestarting": {
    message: "再起動中",
    description:
      "Key Value status badge (Render's config_restart: an established store restarting after a config change, restart, or resume — not a first provision)",
  },
  "keyvalue.statusUnavailable": {
    message: "利用不可",
    description: "Key Value status badge (provisioning failed)",
  },
  "keyvalue.statusSuspended": {
    message: "一時停止中",
    description:
      "Key Value status badge (hibernated; suspension wins over the status enum)",
  },
  "keyvalue.statusUnknown": {
    message: "不明",
    description: "Key Value status badge for an unrecognized status",
  },
  "keyvalue.statusReasonStatefulSetReadFailed": {
    message: "Key Value のワークロードの状態を確認できませんでした。",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StatefulSetReadFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStatefulSetFailed": {
    message:
      "Key Value のワークロードを更新できなかったため、最新の変更は適用されませんでした。",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StatefulSetFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonPVCReadFailed": {
    message: "ストレージボリュームの状態を確認できませんでした。",
    description:
      "Why an unavailable Key Value is unavailable: the operator's PVCReadFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonPVCResizeFailed": {
    message: "Key Value のストレージボリュームのサイズを変更できませんでした。",
    description:
      "Why an unavailable Key Value is unavailable: the operator's PVCResizeFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonSecretFailed": {
    message: "Key Value の接続情報を保存できませんでした。",
    description:
      "Why an unavailable Key Value is unavailable: the operator's SecretFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonCredentialSecretFailed": {
    message: "Key Value の認証情報を保存できませんでした。",
    description:
      "Why an unavailable Key Value is unavailable: the operator's CredentialSecretFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonSecretRecreateFailed": {
    message: "Key Value の接続情報を再構築できませんでした。",
    description:
      "Why an unavailable Key Value is unavailable: the operator's SecretRecreateFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonServiceFailed": {
    message: "Key Value の内部アドレスをセットアップできませんでした。",
    description:
      "Why an unavailable Key Value is unavailable: the operator's ServiceFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonNetworkPolicyFailed": {
    message: "Key Value のネットワークポリシーを適用できませんでした。",
    description:
      "Why an unavailable Key Value is unavailable: the operator's NetworkPolicyFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonBackupNetworkPolicyFailed": {
    message: "バックアップジョブのネットワークポリシーを適用できませんでした。",
    description:
      "Why an unavailable Key Value is unavailable: the operator's BackupNetworkPolicyFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonBackupCronJobFailed": {
    message: "毎日のバックアップのスケジュールを更新できませんでした。",
    description:
      "Why an unavailable Key Value is unavailable: the operator's BackupCronJobFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonTLSIssuerMissing": {
    message:
      "証明書の発行元が設定されていないため、パブリックアクセスを有効にできません。",
    description:
      "Why an unavailable Key Value is unavailable: the operator's TLSIssuerMissing Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonCertificateFailed": {
    message: "パブリックアクセス用の証明書を発行できませんでした。",
    description:
      "Why an unavailable Key Value is unavailable: the operator's CertificateFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonCertificateCleanupFailed": {
    message: "パブリックアクセス用の証明書を削除できませんでした。",
    description:
      "Why an unavailable Key Value is unavailable: the operator's CertificateCleanupFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageClassMissing": {
    message:
      "ストレージボリュームにストレージクラスがないため、拡張できません。",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageClassMissing Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageClassNotFound": {
    message:
      "ストレージボリュームのストレージクラスが見つからないため、拡張できません。",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageClassNotFound Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageClassNotExpandable": {
    message:
      "ストレージボリュームのストレージクラスでは、拡張が許可されていません。",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageClassNotExpandable Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageBlockedByQuota": {
    message:
      "ワークスペースのストレージ割り当てに空きがないため、ストレージボリュームを拡張できません。",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageBlockedByQuota Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonPersistenceTransitionFailed": {
    message:
      "起動のための保存データの準備に失敗しました。データは保持されており、再試行されます。",
    description:
      "Why an unavailable Key Value is unavailable: the operator's PersistenceTransitionFailed Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonPersistenceSourceUnknown": {
    message:
      "保存データの永続化モードが不明なため、プラットフォームの運用担当者が確認するまで Key Value を起動できません。データは変更されていません。",
    description:
      "Why an unavailable Key Value is unavailable: the operator's PersistenceSourceUnknown Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.statusReasonStorageShrinkRejected": {
    message:
      "Key Value のストレージは増やすことしかできません。ボリュームを現在のサイズより小さくすることはできません。",
    description:
      "Why an unavailable Key Value is unavailable: the operator's StorageShrinkRejected Ready reason (bex-api statusReasonCode)",
  },
  "keyvalue.errorTitle": {
    message: "Key Value ストアを読み込めませんでした",
    description: "Key Value list error card title",
  },
  "keyvalue.errorBody": {
    message:
      "bex-api へのリクエストに失敗しました。接続を確認して、もう一度お試しください。",
    description: "Key Value list error card body",
  },
  "keyvalue.emptyTitle": {
    message: "Key Value ストアはまだありません",
    description: "Key Value list empty state title",
  },
  "keyvalue.emptyBody": {
    message:
      "最初のマネージド Key Value ストアを作成すると、ここに表示されます。",
    description: "Key Value list empty state body",
  },
  "keyvalue.actionsMenu": {
    message: "{name} の操作",
    description: "Accessible label for the per-row actions trigger",
  },
  "keyvalue.actionDelete": {
    message: "削除",
    description: "Row action: permanently delete the Key Value store",
  },
  "keyvalue.deleteConfirmTitle": {
    message: "Key Value インスタンスを削除",
    description: "Delete-confirmation dialog title",
  },
  "keyvalue.deleteConfirmBody": {
    message:
      "この操作は元に戻せません。この Key Value インスタンスを削除してもよろしいですか？",
    description:
      "Delete-confirmation dialog body (verbatim Render copy, docs/render-artifacts/key-value.md)",
  },
  "keyvalue.deleteConfirmPrompt": {
    message: "確認のため、下に {phrase} と入力してください。",
    description:
      "Body prompt naming Render's exact sudo delete phrase (rendered bold by SudoCommandField)",
  },
  "keyvalue.deleteCancel": {
    message: "キャンセル",
    description: "Delete-confirmation dialog cancel button",
  },
  "keyvalue.deleteConfirm": {
    message: "Key Value インスタンスを削除",
    description: "Delete-confirmation dialog confirm button",
  },
  "keyvalue.deleteSuccess": {
    message: "{name} を削除しています…",
    description: "Toast after a delete request is accepted",
  },
  "keyvalue.deleteError": {
    message: "{name} を削除できませんでした。もう一度お試しください。",
    description: "Toast when a delete request fails",
  },
  "keyvalue.createButton": {
    message: "新規 Key Value",
    description: "Button that navigates to the create-Key-Value page",
  },
  "keyvalue.createTitle": {
    message: "新規 Key Value",
    description: "Create-Key-Value page title",
  },
  "keyvalue.createDescription": {
    message:
      "マネージド Valkey (Redis 互換) インスタンスをプロビジョニングします。",
    description: "Create-Key-Value page description",
  },
  "keyvalue.fieldName": {
    message: "名前",
    description: "Create-Key-Value form field label (store name)",
  },
  "keyvalue.fieldNamePlaceholder": {
    message: "example-key-value-name",
    description: "Create-Key-Value name input placeholder",
  },
  "keyvalue.fieldNameError": {
    message: "改行や制御文字を含まない、100 文字以内の名前を入力してください。",
    description: "Create-Key-Value name validation message",
  },
  "keyvalue.fieldPlan": {
    message: "コンピュートプラン",
    description: "Create-Key-Value form field label (plan / tier)",
  },
  "keyvalue.fieldVersion": {
    message: "Valkey バージョン",
    description: "Create-Key-Value form field label (major version)",
  },
  "keyvalue.fieldVersionDefault": {
    message: "デフォルト（最新）",
    description: "Create-Key-Value version select default option",
  },
  "keyvalue.fieldMaxmemoryPolicy": {
    message: "Maxmemory ポリシー",
    description: "Create-Key-Value form field label (eviction policy)",
  },
  "keyvalue.fieldMaxmemoryRecommended": {
    message: "（キャッシュ向けに推奨）",
    description:
      "Suffix on the recommended maxmemory policy option (allkeys-lru)",
  },
  "keyvalue.fieldMaxmemoryPolicyHint": {
    message:
      "ストアがメモリ上限に達したときに、キーを削除（エビクト）する方法です。",
    description: "Create-Key-Value maxmemory policy helper text",
  },
  "keyvalue.fieldPersistenceMode": {
    message: "永続化モード",
    description: "Create-Key-Value form field label (persistence)",
  },
  "keyvalue.fieldPersistenceModeHint": {
    message:
      "再起動後もデータが残るように、ディスクへデータを永続化する方法です。",
    description: "Create-Key-Value persistence mode helper text",
  },
  "keyvalue.persistenceJournalSnapshot": {
    message: "ジャーナル + スナップショット",
    description: "Persistence mode option: AOF journal plus RDB snapshots",
  },
  "keyvalue.persistenceSnapshot": {
    message: "スナップショットのみ",
    description: "Persistence mode option: RDB snapshots only",
  },
  "keyvalue.persistenceOff": {
    message: "オフ",
    description: "Persistence mode option: no persistence",
  },
  "keyvalue.fieldPublic": {
    message: "パブリックアクセス",
    description: "Create-Key-Value form field label (external endpoint toggle)",
  },
  "keyvalue.fieldPublicHint": {
    message:
      "プライベートネットワークの外部から TLS 経由で接続できるようにします。",
    description: "Create-Key-Value public toggle helper text",
  },
  "keyvalue.createCancel": {
    message: "キャンセル",
    description: "Create-Key-Value page cancel button",
  },
  "keyvalue.createSubmit": {
    message: "Key Value インスタンスを作成",
    description: "Create-Key-Value page submit button",
  },
  "keyvalue.createSuccess": {
    message: "{name} を作成しています…",
    description:
      "Toast after a create request is accepted (provisioning is async)",
  },
  "keyvalue.createError": {
    message: "{name} を作成できませんでした。もう一度お試しください。",
    description: "Toast when a create request fails",
  },
  "keyvalue.capLimitTitle": {
    message: "Key Value の上限に達しました",
    description:
      "Alert title when the workspace's KV creation cap is hit (w7/m9)",
  },
  "keyvalue.capLimitUpgrade": {
    message: "プランをアップグレード",
    description: "Upgrade CTA button inside the KV cap-limit Alert (w7/m9)",
  },
  "keyvalue.metaTitle": {
    message: "詳細",
    description: "Key Value detail metadata card title",
  },
  "keyvalue.metaId": {
    message: "ID",
    description: "Key Value detail metadata row label (immutable red- id)",
  },
  "keyvalue.metaStatus": {
    message: "ステータス",
    description: "Key Value detail metadata row label",
  },
  "keyvalue.metaPlan": {
    message: "コンピュートプラン",
    description: "Key Value detail metadata row label",
  },
  "keyvalue.metaConnectionLimit": {
    message: "接続数の上限",
    description:
      "Key Value detail Details card row label: the plan's client connection limit, as Render's Key Value Instance card shows it",
  },
  "keyvalue.metaVersion": {
    message: "バージョン",
    description: "Key Value detail metadata row label",
  },
  "keyvalue.metaPublic": {
    message: "パブリックアクセス",
    description: "Key Value detail metadata row label (external endpoint)",
  },
  "keyvalue.metaExternalHost": {
    message: "外部ホスト",
    description: "Key Value detail metadata row label (SNI hostname)",
  },
  "keyvalue.metaRegion": {
    message: "リージョン",
    description: "Key Value detail metadata row label (platform placement)",
  },
  "keyvalue.metaCreated": {
    message: "作成日時",
    description: "Key Value detail metadata row label (relative age)",
  },
  "keyvalue.yes": {
    message: "はい",
    description: "Metadata value for a true boolean field",
  },
  "keyvalue.publicPending": {
    message: "はい（外部エンドポイントを開放中: 証明書の発行待ち）",
    description:
      "Details value for Public access while the store's certificate is pending, so it does not read as reachable (w4/m357)",
  },
  "keyvalue.publicFailed": {
    message: "はい（まだ到達できません: 証明書の発行に失敗）",
    description:
      "Details value for Public access when the store's certificate failed to issue (w4/m357)",
  },
  "keyvalue.no": {
    message: "いいえ",
    description: "Metadata value for a false boolean field",
  },
  "keyvalue.connTitle": {
    message: "接続",
    description: "Connection-info panel card title",
  },
  "keyvalue.connDescription": {
    message:
      "この Key Value ストアの接続文字列です。ご自身で操作したときにのみ表示され、自動的に表示されることはありません。",
    description: "Connection-info panel card description",
  },
  "keyvalue.internalAuthOff": {
    message: "内部接続にパスワードは不要です。",
    description: "Connections panel: internal auth off",
  },
  "keyvalue.internalAuthOn": {
    message: "内部接続にはパスワードが必要です。",
    description: "Connections panel: internal auth on",
  },
  "keyvalue.internalAuthEnable": {
    message: "内部認証を有効にする",
    description: "Button and confirm label (Render's wording)",
  },
  "keyvalue.internalAuthConfirmTitle": {
    message: "内部認証を有効にしますか？",
    description: "Confirm dialog title",
  },
  "keyvalue.internalAuthConfirmBody": {
    message:
      "すべての内部接続でパスワードが必要になります。認証なしの URL を使用している既存の接続は切断され、リンクされたサービスは次回のデプロイで新しい URL に切り替わります。接続情報に表示される URL に更新してください。この操作は元に戻せません。",
    description: "Confirm dialog body",
  },
  "keyvalue.internalAuthEnabled": {
    message: "{name} の内部認証を有効にしました。",
    description: "Toast after enabling",
  },
  "keyvalue.internalAuthError": {
    message: "内部認証を有効にできませんでした。もう一度お試しください。",
    description: "Toast when enabling fails",
  },
  "keyvalue.connReveal": {
    message: "接続情報を表示",
    description: "Button that fetches the connection info on demand",
  },
  "keyvalue.connHide": {
    message: "接続情報を非表示",
    description: "Button that clears the revealed connection info",
  },
  "keyvalue.connInternal": {
    message: "内部 Key Value URL",
    description: "Connection-info field label (in-cluster redis:// URL)",
  },
  "keyvalue.connExternal": {
    message: "外部 Key Value URL",
    description: "Connection-info field label (public rediss:// URL)",
  },
  "keyvalue.connExternalPending": {
    message:
      "外部アクセスを有効にしています。公的に信頼される証明書を発行中です（通常は数分かかります）。ストアが TLS 接続を受け付けるようになると、外部 URL と CLI コマンドで接続できます。それまでは、内部 URL を使って、お使いのサービスからのみ到達できます。非表示にしてからもう一度表示すると更新されます。",
    description:
      "Shown under the external URL while a public store's TLS listener is still opening (w9/m521)",
  },
  "keyvalue.connExternalFailed": {
    message:
      "外部 URL と CLI コマンドではまだ接続できません。このストアの公的に信頼される証明書の発行に失敗しており、bex は自動的に再試行します。成功するまでは、内部 URL を使って、お使いのサービスからのみ到達できます。",
    description:
      "Shown under the external URL when a public store's certificate failed to issue (w4/m357, w9/m521)",
  },
  "keyvalue.connExternalUnavailable": {
    message:
      "パブリックではありません。外部アクセスを有効にするには、ネットワークで受信 IP ルールを追加してください。",
    description:
      "Shown instead of the external URL and the CLI command when the store isn't public (w9/134)",
  },
  "keyvalue.connCli": {
    message: "Valkey CLI コマンド",
    description:
      "Connection-info field label (ready-to-run valkey-cli command)",
  },
  "keyvalue.connErrorTitle": {
    message: "接続情報を読み込めませんでした",
    description: "Connection-info panel error title",
  },
  "keyvalue.connErrorBody": {
    message:
      "ストアのプロビジョニング中か、認証情報を表示する権限がない可能性があります。",
    description: "Connection-info panel error body",
  },
  "keyvalue.copied": {
    message: "クリップボードにコピーしました",
    description: "Toast after copying a connection field",
  },
  "keyvalue.copyError": {
    message: "クリップボードにコピーできませんでした",
    description: "Toast when clipboard copy fails",
  },
  "keyvalue.dangerDelete": {
    message: "Key Value インスタンスを削除",
    description: "Discoverable delete action at the bottom of Key Value Info",
  },
  "keyvalue.actionSuspend": {
    message: "Key Value インスタンスを一時停止",
    description: "Suspend action button label",
  },
  "keyvalue.actionResume": {
    message: "Key Value インスタンスを再開",
    description: "Resume action button label",
  },
  "keyvalue.confirmSuspendTitle": {
    message: "Key Value インスタンスを一時停止",
    description: "Suspend-confirmation dialog title",
  },
  "keyvalue.confirmSuspendBody": {
    message: "この Key Value インスタンスを一時停止してもよろしいですか？",
    description: "Suspend-confirmation dialog description",
  },
  "keyvalue.confirmSuspendDetail": {
    message:
      "{name} を一時停止することを確認してください。アクティブな接続は切断されますが、いつでも再開できます。データの保持は永続化モードに従います。オフの場合はすべてのデータが失われ、スナップショットのみの場合は最後のスナップショットまでのデータが保持され、ジャーナル + スナップショットの場合は約 1 秒前までの書き込みが保持されます。",
    description: "Suspend-confirmation consequences and named instance",
  },
  "keyvalue.confirmSuspendPrompt": {
    message: "確認のため、下に {phrase} と入力してください。",
    description:
      "Body prompt naming Render's exact Key Value suspend phrase (rendered bold by SudoCommandField)",
  },
  "keyvalue.confirmCancel": {
    message: "キャンセル",
    description: "Suspend-confirmation dialog cancel button",
  },
  "keyvalue.toastSuspendSuccess": {
    message: "{name} を一時停止しています…",
    description: "Toast after a suspend request is accepted",
  },
  "keyvalue.toastSuspendError": {
    message: "{name} を一時停止できませんでした。もう一度お試しください。",
    description: "Toast when a suspend request fails",
  },
  "keyvalue.toastResumeSuccess": {
    message: "{name} を再開しています…",
    description: "Toast after a resume request is accepted",
  },
  "keyvalue.toastResumeError": {
    message: "{name} を再開できませんでした。もう一度お試しください。",
    description: "Toast when a resume request fails",
  },
  "keyvalue.planTitle": {
    message: "コンピュートプラン",
    description: "Key Value detail plan-picker card title",
  },
  "keyvalue.planDescription": {
    message:
      "より大きいコンピュートプランに変更できます。Key Value インスタンスは、より小さいものには変更できません。変更するとストアが再起動され、1〜2 分間利用できなくなります。データは永続化モードに従って保持されます。永続化がオフのストアでは、すべてのキーが失われます。",
    description: "Key Value detail plan-picker card description",
  },
  "keyvalue.planDowngradeUnavailable": {
    message:
      "Key Value インスタンスは、より小さいコンピュートプランには変更できません。このプランで新しいインスタンスを作成し、データを移行してください。",
    description:
      "Tooltip on a disabled plan card smaller than the store's current plan: Render does not downgrade a Key Value instance (w8/249)",
  },
  "keyvalue.planPausedReason": {
    message:
      "Key Value プラン「{name}」は一時的に利用できません。{largest} かそれより小さいプランを選択してください。",
    description:
      "A paused Key Value plan's tooltip and screen-reader reason, composed in the dashboard's language; matches bex-api's English unavailableReason (w4/552)",
  },
  "keyvalue.planPickerSave": {
    message: "保存",
    description: "Key Value plan-picker save button",
  },
  "keyvalue.planPickerCancel": {
    message: "キャンセル",
    description: "Key Value plan-picker cancel / reset button",
  },
  "keyvalue.reviewComputeLead": {
    message: "コンピュートの選択内容を確認してください。",
    description:
      "Key Value plan review: the lead of a compute plan change's review (Render's copy)",
  },
  "keyvalue.reviewUnavailable": {
    message: "更新中、Key Value インスタンスは利用できなくなります。",
    description:
      "Key Value plan review warning: the store restarts to apply the plan (Render's copy); the data-loss sentence follows it when persistence is Off",
  },
  "keyvalue.reviewUpgradeCompute": {
    message: "コンピュートをアップグレード",
    description:
      "Key Value plan review: the button that applies a plan change (Render's copy; Key Value plans only go up)",
  },
  "keyvalue.restartConfirmDataLoss": {
    message:
      "永続化がオフのため、再起動すると {name} のすべてのキーが削除されます。この操作は元に戻せません。",
    description:
      "Restart confirm (plan change): a Persistence Off store loses all data on the restart (w4/m263)",
  },
  "keyvalue.restartConfirmDataLossUnknown": {
    message:
      "このストアの永続化モードは不明です。オフの場合、再起動すると {name} のすべてのキーが削除されます。この操作は元に戻せません。",
    description:
      "Restart confirm when the persistence mode could not be read: the typed phrase is still required",
  },
  "keyvalue.planPickerSuccess": {
    message: "プランを {name} に更新しています…",
    description: "Toast after a plan update is accepted",
  },
  "keyvalue.planPickerError": {
    message: "プランを更新できませんでした。もう一度お試しください。",
    description: "Toast when a plan update fails",
  },
  "keyvalue.maxmemoryTitle": {
    message: "Maxmemory ポリシー",
    description: "Key Value detail eviction-policy card title",
  },
  "keyvalue.maxmemoryDescription": {
    message:
      "ストアがメモリ上限に達したときに、キーを削除（エビクト）する方法を変更します。新しいポリシーは、再起動せずに実行中のストアに適用されます。",
    description: "Key Value detail eviction-policy card description",
  },
  "keyvalue.maxmemoryLabel": {
    message: "Maxmemory ポリシー",
    description: "Accessible label for the eviction-policy select",
  },
  "keyvalue.maxmemoryEdit": {
    message: "Maxmemory ポリシーを編集",
    description:
      "Accessible label for the maxmemory-policy edit-in-place pencil",
  },
  "keyvalue.maxmemorySuccess": {
    message: "Maxmemory ポリシーを {policy} に更新しています…",
    description: "Toast after an eviction-policy update is accepted",
  },
  "keyvalue.maxmemoryError": {
    message:
      "Maxmemory ポリシーを更新できませんでした。もう一度お試しください。",
    description: "Toast when an eviction-policy update fails",
  },
  "keyvalue.persistenceTitle": {
    message: "永続化モード",
    description: "Key Value detail persistence-mode card title",
  },
  "keyvalue.persistenceDescription": {
    message:
      "モードを変更するとストアが再起動され、データの準備中は接続が中断されます。ジャーナル + スナップショットとスナップショットのみの間で切り替えても、データは保持されます。オフへの切り替え、またはオフからの切り替えでは、すべてのデータが破棄されます。",
    description: "Key Value detail persistence-mode card description",
  },
  "keyvalue.persistenceLabel": {
    message: "永続化モード",
    description: "Accessible label for the persistence-mode select",
  },
  "keyvalue.persistenceEdit": {
    message: "永続化モードを編集",
    description:
      "Accessible label for the persistence-mode edit-in-place pencil",
  },
  "keyvalue.persistenceSuccess": {
    message: "永続化モードを {mode} に更新しています…",
    description: "Toast after a persistence-mode update is accepted",
  },
  "keyvalue.persistenceConfirmTitle": {
    message: "すべてのデータを削除して永続化を変更しますか？",
    description: "Destructive persistence-mode confirmation title",
  },
  "keyvalue.persistenceConfirmDescription": {
    message:
      "オフへの切り替え、またはオフからの切り替えを行うと、{name} のすべてのデータが削除され、ストアが再起動されます。この操作は元に戻せません。",
    description: "Destructive persistence-mode confirmation description",
  },
  "keyvalue.persistenceConfirmAction": {
    message: "データを削除してモードを変更",
    description: "Destructive persistence-mode confirmation action",
  },
  "keyvalue.persistenceError": {
    message: "永続化モードを更新できませんでした。もう一度お試しください。",
    description: "Toast when a persistence-mode update fails",
  },
  "keyvalue.networkingTitle": {
    message: "ネットワーク",
    description: "Detail-page Networking card title (IP allowlist)",
  },
  "keyvalue.networkingDescription": {
    message: "外部エンドポイントにアクセスできる送信元 IP を制限します。",
    description: "Networking card subtitle",
  },
  "keyvalue.networkingHint": {
    message:
      "CIDR ルールを保存すると、一致する送信元 IP からの外部アクセスが有効になります。空のリストを保存すると外部接続がブロックされます。内部アクセスは変更されません。",
    description: "Networking IP allowlist helper text",
  },
  "keyvalue.networkingHintRestart": {
    message:
      "CIDR ルールを保存すると、一致する送信元 IP からの外部アクセスが有効になります。このストアで外部アクセスを有効にするのは初めてのため、最初の保存でストアが再起動され、最大 1 分間接続が切断されます。空のリストを保存すると外部接続がブロックされます。",
    description:
      "Networking IP allowlist helper text when enabling external access restarts the store (its first enable)",
  },
  "keyvalue.networkingRestartConfirmTitle": {
    message: "すべてのデータを削除して外部アクセスを有効にしますか？",
    description:
      "Confirmation title when a first external-access enable restarts a persistence-Off store",
  },
  "keyvalue.networkingRestartConfirmDescription": {
    message:
      "外部アクセスを有効にすると、{name} が初めて再起動されます。永続化モードがオフのため、再起動するとすべてのデータが削除されます。この操作は元に戻せません。",
    description:
      "Confirmation description when a first external-access enable restarts a persistence-Off store",
  },
  "keyvalue.networkingRestartConfirmAction": {
    message: "データを削除してルールを保存",
    description:
      "Confirmation action when a first external-access enable restarts a persistence-Off store",
  },
  "keyvalue.networkingInternalOnly": {
    message:
      "外部接続は無効です。アクセスを有効にするには、IP ルールを 1 つ以上保存してください。",
    description: "External access is disabled; saving rules enables it",
  },
  "keyvalue.networkingEmpty": {
    message:
      "IP ルールはありません。空のリストを保存すると、外部接続がブロックされます。",
    description: "Empty datastore allowlist draft and the effect of saving it",
  },
  "keyvalue.networkingAdd": {
    message: "追加",
    description: "Networking button to add a CIDR to the draft allowlist",
  },
  "keyvalue.networkingEntryDescription": {
    message: "説明（任意）",
    description:
      "Placeholder for the optional per-entry allowlist description input",
  },
  "keyvalue.networkingCIDR": {
    message: "新しい CIDR ブロック",
    description: "Accessible label for the add-row CIDR input",
  },
  "keyvalue.networkingCIDRRule": {
    message: "ルール {number} の CIDR ブロック",
    description: "Accessible label for an existing allowlist CIDR row",
  },
  "keyvalue.networkingDescriptionLabel": {
    message: "新しいルールの説明",
    description: "Accessible label for the add-row description input",
  },
  "keyvalue.networkingDescriptionRule": {
    message: "ルール {number} の説明",
    description: "Accessible label for an existing allowlist description row",
  },
  "keyvalue.networkingRemove": {
    message: "{cidr} を削除",
    description: "Accessible label to remove a CIDR chip",
  },
  "keyvalue.networkingMoveUp": {
    message: "{cidr} を上へ移動",
    description: "Accessible label to move an allowlist entry earlier",
  },
  "keyvalue.networkingMoveDown": {
    message: "{cidr} を下へ移動",
    description: "Accessible label to move an allowlist entry later",
  },
  "keyvalue.networkingInvalid": {
    message:
      "重複しない有効な IPv4 または IPv6 の CIDR ブロックを入力してください。",
    description: "Validation error for Key Value allowlist entries",
  },
  "keyvalue.networkingSave": {
    message: "許可リストを保存",
    description: "Networking button to persist the allowlist",
  },
  "keyvalue.networkingSaved": {
    message: "許可リストを更新しました。",
    description: "Toast after saving the IP allowlist",
  },
  "keyvalue.networkingError": {
    message: "許可リストを保存できませんでした。もう一度お試しください。",
    description: "Toast when saving the allowlist fails",
  },
  "keyvalue.nameEdit": {
    message: "Key Value の名前を編集",
    description: "Accessible label for the Key Value name edit-in-place pencil",
  },
  "keyvalue.nameDescription": {
    message:
      "表示名を変更します。Key Value の ID とすべての接続情報は変わりません。",
    description: "Key Value detail rename row description",
  },
  "keyvalue.nameSuccess": {
    message: "Key Value ストアの名前を {name} に変更しました。",
    description: "Toast after a Key Value rename succeeds",
  },
  "keyvalue.nameError": {
    message:
      "Key Value ストアの名前を変更できませんでした。もう一度お試しください。",
    description: "Toast when a Key Value rename fails unexpectedly",
  },
  "keyvalue.nameConflict": {
    message:
      "この名前の Key Value ストアは、このワークスペースにすでに存在します。",
    description: "Toast when a Key Value rename collides with another name",
  },
  "keyvalue.nameInvalid": {
    message: "改行や制御文字を含まない、100 文字以内の名前を入力してください。",
    description: "Key Value rename validation message",
  },
  // --- Sidebar and page headings (w3/m649) ---
  "keyvalue.navInfo": {
    message: "情報",
    description:
      "Key Value sidebar entry and page heading: the instance's Info page (general details, plan, connections, networking)",
  },
  "keyvalue.navLogs": {
    message: "ログ",
    description:
      "Key Value sidebar entry and page heading: the instance's Logs page",
  },
  "keyvalue.navMetrics": {
    message: "メトリクス",
    description:
      "Key Value sidebar entry and page heading: the instance's Metrics page",
  },
  "keyvalue.logsRangeLabel": {
    message: "期間",
    description: "Accessible label for the log time-range select",
  },
  "keyvalue.logsAllInstances": {
    message: "すべてのインスタンス",
    description: "Default option in the instance filter (no filter applied)",
  },
  "keyvalue.logsSearchPlaceholder": {
    message: "ログを検索…",
    description: "Placeholder for the log text-search input",
  },
  "keyvalue.logsLoading": {
    message: "ログを読み込み中…",
    description: "Loading state message while fetching logs",
  },
  "keyvalue.logsEmptyTitle": {
    message: "ログ行がありません",
    description: "Empty state title when no log lines are returned",
  },
  "keyvalue.logsEmptyBody": {
    message:
      "この期間にログ行はありません。Valkey は、キースペースのイベント、スロークエリ、起動メッセージをログに記録します。",
    description: "Empty state body when no log lines and no filters active",
  },
  "keyvalue.logsUnavailableTitle": {
    message: "ログを利用できません",
    description: "Empty state title when the logs source is not configured",
  },
  "keyvalue.logsUnavailableBody": {
    message:
      "プラットフォームのログソースが設定されていません。BEX_LOKI_URL を有効にするには、プラットフォームの管理者にお問い合わせください。",
    description: "Empty state body when BEX_LOKI_URL is not set",
  },
  "keyvalue.logsUnauthorizedTitle": {
    message: "アクセスが拒否されました",
    description: "Empty state title when the caller lacks can_view_logs",
  },
  "keyvalue.logsUnauthorizedBody": {
    message: "この Key Value ストアのログを表示する権限がありません。",
    description: "Empty state body for a 403 on the logs query",
  },
  "keyvalue.fieldNameDescription": {
    message: "Key Value インスタンスの一意な名前。",
    description: "Create-Key-Value Name row description",
  },
  "keyvalue.fieldVersionDescription": {
    message: "インスタンスで実行する Valkey のメジャーバージョン。",
    description: "Create-Key-Value Version row description",
  },
  "keyvalue.fieldPlanDescription": {
    message: "インスタンスで使えるメモリと接続数の上限を決めます。",
    description:
      "Create-Key-Value plan row description (Render: the plan determines RAM and connection limit)",
  },
  "keyvalue.connectInternalTitle": {
    message: "内部 Redis URL",
    description:
      "Header Connect menu, Internal tab: the private-network URL's title (Render's copy, which names Key Value by its type name Redis, w3/m675)",
  },
  "keyvalue.connectExternalTitle": {
    message: "外部 Redis URL",
    description:
      "Header Connect menu, External tab: the public URL's title (Render's copy, w3/m675)",
  },
  "keyvalue.connectCliTitle": {
    message: "Redis CLI コマンド",
    description:
      "Header Connect menu, External tab: the `bex kv-cli <id>` command's title (Render's copy, w3/m675)",
  },
};

export default jaKeyValue;
