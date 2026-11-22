import type { TranslationEntry } from "@/i18n";

const jaBlueprints: Record<string, TranslationEntry> = {
  "blueprints.resourceType": {
    message: "Blueprint",
    description: "Blueprint resource type used in document titles",
  },
  "blueprints.pageTitle": {
    message: "Blueprints",
    description: "Blueprints list page heading and document title",
  },
  "blueprints.cardTitle": {
    message: "Blueprints",
    description: "Blueprints table card title",
  },
  "blueprints.colName": {
    message: "名前",
    description:
      "Blueprints table column header — blueprint name (derived from repo)",
  },
  "blueprints.colRepo": {
    message: "リポジトリ",
    description: "Blueprints table column header — source repo URL",
  },
  "blueprints.colBranch": {
    message: "ブランチ",
    description: "Blueprints table column header — tracked branch",
  },
  "blueprints.colStatus": {
    message: "ステータス",
    description: "Blueprints table column header",
  },
  "blueprints.colUpdated": {
    message: "最終更新",
    description: "Blueprints table column header — updatedAt relative age",
  },
  "blueprints.createButton": {
    message: "新しい Blueprint インスタンス",
    description: "Button label that opens the New Blueprint dialog",
  },
  "blueprints.createTitle": {
    message: "新しい Blueprint インスタンス",
    description: "New Blueprint page title",
  },
  "blueprints.createDescription": {
    message:
      "render.yaml Blueprint を含むリポジトリを接続して、サービスのスタック全体を作成し、同期します。",
    description: "New Blueprint page subtitle under the title",
  },
  "blueprints.createSourceTitle": {
    message: "リポジトリを接続",
    description:
      "New Blueprint page — heading above the GitHub/Public Git source tabs (Render parity)",
  },
  "blueprints.createBranchLabel": {
    message: "ブランチ",
    description: "New Blueprint page — branch combobox label",
  },
  "blueprints.createBranchPlaceholder": {
    message: "main",
    description: "New Blueprint page — branch combobox placeholder",
  },
  "blueprints.createBranchHint": {
    message: "render.yaml ファイルがあるリポジトリのブランチです。",
    description: "New Blueprint page — branch field helper text",
  },
  "blueprints.createPathLabel": {
    message: "Blueprint のパス",
    description: "New Blueprint page — render.yaml path input label",
  },
  "blueprints.createNamePlaceholder": {
    message: "my-stack",
    description: "New Blueprint page — blueprint name placeholder",
  },
  "blueprints.createPathPlaceholder": {
    message: "render.yaml",
    description: "New Blueprint page — manifest path placeholder",
  },
  "blueprints.createPathHint": {
    message:
      "リポジトリ内の Blueprint ファイルへのパスです。.yaml / .yml ファイルであれば使用できます（例: infra/bex/stack.yaml）。デフォルトは、リポジトリのルートにある render.yaml です。",
    description: "New Blueprint page — manifest path helper text",
  },
  "blueprints.createNameLabel": {
    message: "Blueprint 名",
    description: "New Blueprint page — name input label",
  },
  "blueprints.createNameTaken": {
    message:
      "{name} という名前の Blueprint は、このワークスペースにすでに存在します。別の名前を選択してください。",
    description:
      "New Blueprint page — shown under the name field when Deploy was refused because an approved Blueprint already has the name (w8/250)",
  },
  "blueprints.createAction": {
    message: "Blueprint をデプロイ",
    description: "New Blueprint dialog submit button label",
  },
  "blueprints.createCancel": {
    message: "キャンセル",
    description: "New Blueprint dialog cancel button label",
  },
  "blueprints.createSuccess": {
    message: "Blueprint を作成しました",
    description: "Toast shown after a successful createBlueprint call",
  },
  "blueprints.createError": {
    message: "Blueprint の作成に失敗しました",
    description: "Toast shown when createBlueprint returns an error",
  },
  "blueprints.previewTitle": {
    message: "Blueprint の設定を確認",
    description:
      "New Blueprint page — heading of the pre-create fetch + validate section",
  },
  "blueprints.previewSelectSource": {
    message:
      "デプロイする前に Blueprint ファイルを確認するには、リポジトリとブランチを選択してください。",
    description: "Review section placeholder before a source is chosen",
  },
  "blueprints.previewLoading": {
    message: "リポジトリから {path} を取得しています…",
    description: "Review section loading text while the manifest is fetched",
  },
  "blueprints.previewNotFoundTitle": {
    message: "Blueprint ファイルが見つかりません",
    description: "Review section error title when the manifest fetch fails",
  },
  "blueprints.previewNotFoundBody": {
    message: "Blueprint ファイル {path} が {branch} ブランチに見つかりません。",
    description:
      "Review section fallback error body when the backend returns no message",
  },
  "blueprints.previewFileNotFoundTitle": {
    message: "Blueprint ファイルが見つかりません",
    description:
      "Preview failure title: the repo and branch resolved, the file did not",
  },
  "blueprints.previewFileNotFoundBody": {
    message:
      "{path} はブランチ {branch} に存在しません。パスを確認するか、そのブランチにファイルをコミットしてください。",
    description: "Preview failure body for file_not_found",
  },
  "blueprints.previewBranchNotFoundTitle": {
    message: "ブランチが見つかりません",
    description: "Preview failure title for branch_not_found",
  },
  "blueprints.previewBranchNotFoundBody": {
    message: "ブランチ {branch} は、このリポジトリに存在しません。",
    description: "Preview failure body for branch_not_found",
  },
  "blueprints.previewRepoNotFoundTitle": {
    message: "リポジトリが見つかりません",
    description: "Preview failure title for repo_not_found_or_no_access",
  },
  "blueprints.previewRepoNotFoundBody": {
    message:
      "このリポジトリは存在しないか、bex の GitHub App がアクセスできません。リポジトリと、App のリポジトリアクセスを確認してください。",
    description: "Preview failure body for repo_not_found_or_no_access",
  },
  "blueprints.previewAccessDeniedTitle": {
    message: "アクセスが拒否されました",
    description: "Preview failure title for access_denied",
  },
  "blueprints.previewAccessDeniedBody": {
    message:
      "bex の GitHub App には、このリポジトリを読み取る権限がありません。インストールと、付与されているリポジトリを再確認してください。",
    description: "Preview failure body for access_denied",
  },
  "blueprints.previewPublicRepoNotFoundBody": {
    message:
      "このリポジトリは {host} 上に存在しないか、公開されていません。bex はそこでは公開リポジトリから Blueprint を読み取ります。",
    description:
      "Preview failure body for repo_not_found_or_no_access on gitlab.com or bitbucket.org, read anonymously; {host} is that host",
  },
  "blueprints.previewPublicAccessDeniedBody": {
    message:
      "{host} は bex にこのリポジトリの読み取りを許可しませんでした。bex はそこでは公開リポジトリから Blueprint を読み取ります。",
    description:
      "Preview failure body for access_denied on gitlab.com or bitbucket.org, read anonymously; {host} is that host",
  },
  "blueprints.previewHostUnsupportedTitle": {
    message: "サポートされていない Git ホスト",
    description:
      "Preview failure title for a repository on a Git host bex reads no Blueprint from",
  },
  "blueprints.previewHostUnsupportedBody": {
    message:
      "bex は Render と同様に、github.com、gitlab.com、bitbucket.org 上の公開リポジトリから Blueprint を読み取ります。{host} はそのいずれでもありません。",
    description:
      "Preview failure body for a repository on a Git host bex reads no Blueprint from; {host} is that host",
  },
  "blueprints.previewRateLimitedTitle": {
    message: "{host} が bex を制限しています",
    description:
      "Preview failure title for rate_limited; {host} is GitHub or the repository's host",
  },
  "blueprints.previewRateLimitedBody": {
    message:
      "現在、{host} へのリクエストが多すぎます。数分後にもう一度お試しください。",
    description:
      "Preview failure body for rate_limited; {host} is GitHub or the repository's host",
  },
  "blueprints.previewAmbiguousTitle": {
    message: "Blueprint ファイルが 2 つあります",
    description: "Preview failure title for ambiguous_filename",
  },
  "blueprints.previewAmbiguousBody": {
    message:
      "このリポジトリのブランチ {branch} には、render.yaml と bex.yml の両方があります。どちらかを削除するか、上でファイルを明示的に指定してください。",
    description: "Preview failure body for ambiguous_filename",
  },
  "blueprints.previewUnavailableTitle": {
    message: "{host} に接続できませんでした",
    description:
      "Preview failure title for unavailable; {host} is GitHub or the repository's host",
  },
  "blueprints.previewUnavailableBody": {
    message:
      "bex は {host} から Blueprint ファイルを読み取れませんでした。数分後にもう一度お試しください。",
    description:
      "Preview failure body for unavailable; {host} is GitHub or the repository's host",
  },
  "blueprints.previewInvalidPathExtension": {
    message: "Blueprint のパスは、.yaml または .yml ファイルにしてください。",
    description: "Inline path-field error for invalid_path, wrong extension",
  },
  "blueprints.previewInvalidPathShape": {
    message:
      "Blueprint のパスは、リポジトリからの相対パスで、余計な要素を含まない形式にしてください（先頭のスラッシュや .. は不可）。",
    description: "Inline path-field error for invalid_path, bad shape",
  },
  "blueprints.previewRetry": {
    message: "再試行",
    description: "Review section retry button after a failed manifest fetch",
  },
  "blueprints.takeoverResourceTitle": {
    message: "「{name}」を引き継ぎますか？",
    description:
      "Takeover dialog title when a resource is managed by another Blueprint (BLUEPRINT_RESOURCE_CONFLICT)",
  },
  "blueprints.takeoverResourceBody": {
    message:
      "{kind}「{name}」は Blueprint {owner} によって管理されています。続行すると、この Blueprint に移管されます。続行するには、以下のコマンドを入力してください。",
    description: "Takeover dialog body for a Blueprint resource conflict",
  },
  "blueprints.takeoverResourceBodyOtherOwner": {
    message:
      "{kind}「{name}」は別の Blueprint によって管理されています。続行すると、この Blueprint に移管されます。続行するには、以下のコマンドを入力してください。",
    description:
      "Takeover dialog body for a Blueprint resource conflict whose owning Blueprint bex-api could not name",
  },
  "blueprints.previewConflictBody": {
    message:
      "別の Blueprint が、これらのリソースの一部をすでに管理しています。デプロイすると、それらがこの Blueprint に移管されます。引き継がれる内容は、次のステップで正確に確認できます。",
    description:
      "Preview alert body on /blueprint/new when the only problem is a takeover conflict (replaces the raw API message)",
  },
  "blueprints.previewConflict": {
    message: "すでに管理されています。デプロイすると引き継がれます",
    description:
      "Alert title on /blueprint/new when the only preview problem is a resource another Blueprint manages, which an explicit takeover confirmation resolves (w4/m125)",
  },
  "blueprints.previewMatchTitle": {
    message: "既存のリソースと一致しています",
    description:
      "Notice title on /blueprint/new when the Blueprint file names resources that already exist in the workspace (w8/m153, Render parity)",
  },
  "blueprints.previewMatchBody": {
    message:
      "この Blueprint は、ワークスペースにすでに存在するリソースと一致します。新しいリソースは、各名前に接尾辞「-{suffix}」を付けて作成されます。既存のリソースは変更されません。",
    description:
      "Notice body on /blueprint/new for matched resources; {suffix} is the four-character resource name suffix, shown after a hyphen as it appears in a name",
  },
  "blueprints.previewMatchListLabel": {
    message: "ワークスペースにすでにあるもの:",
    description:
      "Notice on /blueprint/new — label above the list of existing resources the Blueprint file matches",
  },
  "blueprints.previewInvalid": {
    message: "Blueprint ファイルにエラーがあります",
    description:
      "Review section error title when the fetched manifest fails validation",
  },
  "blueprints.previewValid_other": {
    message:
      "Blueprint ファイルを正常に解析しました。{count} 個のリソースが変更されます。",
    description:
      "Review section success line, with the plan's total resource count (plural)",
  },
  "blueprints.previewValidEnvironments_other": {
    message:
      "Blueprint ファイルを正常に解析しました。{count} 個の環境が変更されます。",
    description:
      "Review section success line when only environments change (w9/095, plural)",
  },
  "blueprints.previewValidWithEnvironments_other": {
    message:
      "Blueprint ファイルを正常に解析しました。{count} 個のリソースと {environments} が変更されます。",
    description:
      "Review section success line: changing resources (plural) and blueprints.previewEnvironmentCount (w9/095)",
  },
  "blueprints.previewEnvironmentCount_other": {
    message: "{count} 個の環境",
    description:
      "The changing environments inside blueprints.previewValidWithEnvironments (plural)",
  },
  "blueprints.errorAtLineColumn": {
    message: "{line} 行目、{column} 列目",
    description: "Source location beside a Blueprint validation error (w4/151)",
  },
  "blueprints.errorAtLine": {
    message: "{line} 行目",
    description:
      "Source location beside a Blueprint validation error when only the line is known (w4/151)",
  },
  "blueprints.previewNoChanges": {
    message:
      "Blueprint ファイルを正常に解析しました。変更はありません。すべてのリソースが、すでにこのファイルと一致しています。",
    description:
      "Review section success line when every planned action is a no-op (w4/m138)",
  },
  "blueprints.previewOpCreate": {
    message: "作成",
    description:
      "Plan row badge: the resource does not exist yet and will be created",
  },
  "blueprints.previewOpUpdate": {
    message: "更新",
    description: "Plan row badge: an existing resource will be changed",
  },
  "blueprints.previewOpNoop": {
    message: "変更なし",
    description: "Plan row badge: the resource already matches the file",
  },
  "blueprints.previewOpError": {
    message: "適用不可",
    description:
      "Plan row badge: the planner refused this resource (reason below)",
  },
  "blueprints.previewChangedFields": {
    message: "変更: {fields}",
    description:
      "Plan row detail for an update: the changed field paths (never values)",
  },
  "blueprints.previewKindService": {
    message: "サービス",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindPostgres": {
    message: "Postgres",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindKeyValue": {
    message: "Key Value",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindEnvGroup": {
    message: "環境グループ",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindBuildSource": {
    message: "ビルドソース",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindEnvironment": {
    message: "環境",
    description: "Plan row resource kind: a projects[].environments[] entry",
  },
  "blueprints.previewServices": {
    message: "サービス",
    description: "Review section plan group label — services in the manifest",
  },
  "blueprints.previewDatabases": {
    message: "データベース",
    description: "Review section plan group label — databases in the manifest",
  },
  "blueprints.previewKeyValue": {
    message: "Key Value",
    description:
      "Review section plan group label — key-value instances in the manifest",
  },
  "blueprints.previewEnvGroups": {
    message: "環境グループ",
    description: "Review section plan group label — env groups in the manifest",
  },
  "blueprints.previewManualSyncNote": {
    message:
      "bex はこのリポジトリへのプッシュを受信しないため、Blueprint ファイルを後で変更しても自動的には同期されません。変更のたびに Blueprint を同期するか、GitHub App 経由でリポジトリを接続してください。",
    description:
      "Review section note for a repo whose pushes can't reach bex (the preview's autoSync is false): the Blueprint is created without auto-sync",
  },
  "blueprints.previewAutoSyncNote": {
    message:
      "このブランチ上の Blueprint ファイルへの今後の更新は、すべて自動的に同期されます。作成後に、自動同期を一時停止できます。",
    description:
      "Review section note about auto-sync being enabled by default (Render parity)",
  },
  "blueprints.previewError": {
    message: "Blueprint のプレビューを読み込めませんでした。",
    description:
      "Review section error shown when the preview query itself fails (network)",
  },
  "blueprints.promptTitle": {
    message: "シークレットの値",
    description:
      "Review section — heading for the sync:false env var prompt inputs",
  },
  "blueprints.promptHint": {
    message:
      "この Blueprint は、sync: false の環境変数を宣言しています。その値は bex にのみ保存され、リポジトリには保存されません。今すぐ設定してください。値は 1 回だけ設定され、以降の同期で上書きされることはありません。",
    description: "sync:false prompt section helper text",
  },
  "blueprints.promptEmptyWarning": {
    message:
      "空の値は未設定としてデプロイされます。サービスの「環境」タブで設定するまで、サービスが失敗する可能性があります。",
    description:
      "Warning below the sync:false inputs when at least one is blank",
  },
  "blueprints.pricingTitle": {
    message: "料金の見積もり",
    description: "Review section — estimated-pricing panel heading",
  },
  "blueprints.pricingSubtitle": {
    message:
      "コンピュートの料金は秒単位の日割りで計算されます。請求は月初に行われます。",
    description:
      "Estimated-pricing panel subtitle explaining billing semantics (Render parity)",
  },
  "blueprints.pricingLineAmount": {
    message: "（{tier}）${amount} / 月",
    description:
      "Estimated-pricing row amount — plan label + monthly dollar cost",
  },
  "blueprints.pricingLineBreakdown": {
    message: "インスタンス ${instance} + ディスク（{gb} GB）${storage}",
    description:
      "Estimated-pricing datastore row sub-line — instance vs provisioned-storage cost breakdown",
  },
  "blueprints.pricingVariable": {
    message: "変動",
    description:
      "Estimated-pricing row amount for resources whose cost depends on runtime behavior",
  },
  "blueprints.pricingReasonAutoscaling": {
    message: "オートスケーリング",
    description:
      "Estimated-pricing variable-row badge — service has autoscaling enabled",
  },
  "blueprints.pricingReasonMultiInstance": {
    message: "複数のインスタンス",
    description:
      "Estimated-pricing variable-row badge — service declares numInstances > 1",
  },
  "blueprints.pricingReasonCron": {
    message: "Cron Job",
    description:
      "Estimated-pricing variable-row badge — cron jobs bill only while runs execute",
  },
  "blueprints.pricingTotalLabel": {
    message: "合計",
    description: "Estimated-pricing panel total row label",
  },
  "blueprints.pricingTotalAmount": {
    message: "月額 ${amount}{marker}",
    description:
      "Estimated-pricing panel total amount; marker is an asterisk when variable costs are excluded",
  },
  "blueprints.pricingExclusions": {
    message: "* {items} は含まれません。",
    description:
      "Estimated-pricing footnote listing the variable costs excluded from the total",
  },
  "blueprints.pricingExcludeAutoscaling": {
    message: "オートスケーリング",
    description: "Excluded-cost phrase in the estimated-pricing footnote",
  },
  "blueprints.pricingExcludeMultiInstance": {
    message: "追加の固定インスタンス",
    description: "Excluded-cost phrase in the estimated-pricing footnote",
  },
  "blueprints.pricingExcludeCron": {
    message: "Cron Job",
    description: "Excluded-cost phrase in the estimated-pricing footnote",
  },
  "blueprints.pricingEstimateNote": {
    message:
      "あくまで見積もりです。ストレージはプロビジョニングされたサイズで計算されますが、請求書では実際の使用量が計測されます。",
    description:
      "Estimated-pricing disclaimer: forward estimate is an upper bound of metered billing",
  },
  "blueprints.generateButton": {
    message: "Blueprint を生成",
    description: "Blueprints list header — opens the export dialog (w8/m22)",
  },
  "blueprints.generateTitle": {
    message: "Blueprint を生成",
    description: "Generate dialog title",
  },
  "blueprints.generateDescription": {
    message:
      "既存のリソースを選択して、render.yaml としてエクスポートします。これをリポジトリにコミットして、Blueprint として接続できます。",
    description: "Generate dialog description",
  },
  "blueprints.generateEmptyHint": {
    message: "生成元のリソースを 1 つ以上選択してください。",
    description: "Generate dialog hint when nothing is selected",
  },
  "blueprints.generateAction": {
    message: "生成",
    description: "Generate dialog primary action",
  },
  "blueprints.generateBack": {
    message: "戻る",
    description: "Generate dialog — return from preview to selection",
  },
  "blueprints.generateCopy": {
    message: "コピー",
    description: "Generate dialog — copy the yaml to the clipboard",
  },
  "blueprints.generateCopied": {
    message: "render.yaml をコピーしました",
    description: "Toast after copying the generated manifest",
  },
  "blueprints.generateCopyFailed": {
    message:
      "render.yaml をコピーできませんでした。クリップボードへのアクセスを許可してもう一度お試しになるか、ダウンロードしてください。",
    description:
      "Toast when the browser refuses to copy the generated manifest to the clipboard",
  },
  "blueprints.generateDownload": {
    message: "render.yaml をダウンロード",
    description: "Generate dialog — download the manifest file",
  },
  "blueprints.generateSecretsNote": {
    message:
      "保存された値はエクスポートされません。sync: false が指定されたサービスの変数は、Blueprint を最初に作成するときに入力を求められます。generateValue が指定された変数（エクスポートされた環境グループのキーを含む）は、既存の値が維持され、存在しない場合は生成されます。",
    description: "Generate dialog note under the yaml preview",
  },
  "blueprints.generateError": {
    message: "Blueprint を生成できませんでした",
    description: "Toast when the generate query fails",
  },
  "blueprints.emptyTitle": {
    message: "Blueprint インスタンスはまだ作成されていません。",
    description: "Blueprints list empty-state heading",
  },
  "blueprints.emptyBody": {
    message:
      "リポジトリを参照する render.yaml をデプロイすると、Blueprint が自動的に登録されます。デプロイ後は、スタックがここに表示され、同期や検証ができます。",
    description:
      "Blueprints list empty-state body explaining auto-registration",
  },
  "blueprints.loadingBody": {
    message: "Blueprints を読み込み中…",
    description: "Blueprints list loading state label",
  },
  "blueprints.errorTitle": {
    message: "Blueprints を読み込めませんでした",
    description: "Blueprints list error state heading",
  },
  "blueprints.statusActive": {
    message: "有効",
    description: "Blueprint status badge — row is active",
  },
  "blueprints.statusInSync": {
    message: "同期済み",
    description: "Blueprint status badge — git-connected and in sync",
  },
  "blueprints.statusSyncing": {
    message: "同期中",
    description: "Blueprint status badge — sync in progress",
  },
  "blueprints.statusError": {
    message: "エラー",
    description: "Blueprint status badge — last sync failed",
  },
  "blueprints.statusPaused": {
    message: "一時停止中",
    description: "Blueprint status badge — auto-sync paused",
  },
  "blueprints.statusUnknown": {
    message: "{status}",
    description: "Blueprint status badge fallback — raw status value",
  },
  "blueprints.resourceSuffixLabel": {
    message: "リソース名の接尾辞",
    description:
      "Tooltip and screen-reader label of the suffix shown beside a Blueprint's name in the list and on its detail page (w8/m153)",
  },
  "blueprints.detailTitle": {
    message: "{name} · Blueprints",
    description: "Blueprint detail page document title",
  },
  "blueprints.metaRepo": {
    message: "リポジトリ",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaBranch": {
    message: "ブランチ",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaPath": {
    message: "マニフェストのパス",
    description:
      "Blueprint detail metadata label — path to render.yaml in repo",
  },
  "blueprints.metaAutoSync": {
    message: "自動同期",
    description: "Blueprint detail metadata label — auto-sync on push toggle",
  },
  "blueprints.metaCreated": {
    message: "作成日時",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaUpdated": {
    message: "最終同期",
    description: "Blueprint detail metadata label",
  },
  "blueprints.autoSyncOn": {
    message: "オン",
    description: "Auto-sync toggle label — enabled",
  },
  "blueprints.autoSyncOff": {
    message: "オフ",
    description: "Auto-sync toggle label — disabled",
  },
  "blueprints.resourcesTitle": {
    message: "管理対象のリソース",
    description:
      "Blueprint detail section heading listing services/databases managed by this blueprint",
  },
  "blueprints.resourcesEmpty": {
    message:
      "リソースはまだありません。render.yaml を適用するには、Blueprint を同期してください。",
    description: "Blueprint managed-resources empty state (never synced)",
  },
  "blueprints.resourcesEmptyAwaitingApproval": {
    message:
      "リソースはまだありません。render.yaml のリソースを作成するには、計画された同期を承認してください。",
    description:
      "Blueprint managed-resources empty state while the Blueprint awaits approval of its planned sync",
  },
  "blueprints.resourcesEmptyAfterError": {
    message:
      "管理対象のリソースはありません。前回の同期が失敗しました。同期履歴でエラーを確認し、Blueprint を修正してから再試行してください。",
    description:
      "Blueprint managed-resources empty state when status is error after a sync",
  },
  "blueprints.syncHistoryTitle": {
    message: "同期履歴",
    description: "Blueprint detail section heading for the sync run table",
  },
  "blueprints.syncHistoryEmpty": {
    message: "同期はまだありません。",
    description: "Blueprint sync history empty state",
  },
  "blueprints.syncColCommit": {
    message: "コミット",
    description: "Sync history table column — commit SHA",
  },
  "blueprints.syncColState": {
    message: "状態",
    description: "Sync history table column — sync run state",
  },
  "blueprints.syncColStarted": {
    message: "開始",
    description: "Sync history table column — sync run start time",
  },
  "blueprints.syncColCompleted": {
    message: "完了",
    description: "Sync history table column — sync run completion time",
  },
  "blueprints.detachPreviewTitle": {
    message: "リソースは管理対象外になります",
    description: "Blueprint resource detachment notice.",
  },
  "blueprints.detachSuccessTitle": {
    message: "同期が完了しました。リソースは切り離されました",
    description: "Blueprint resource detachment notice.",
  },
  "blueprints.detachEstimateNote": {
    message: "これらの料金は、以下の見積もりに含まれていません。",
    description:
      "Detached resources are outside the proposed manifest estimate.",
  },
  "blueprints.detachWarning": {
    message:
      "これらのリソースは引き続き実行され、料金が発生し続ける可能性があります。この Blueprint は、これらを管理しなくなります。",
    description: "Blueprint resource detachment notice.",
  },
  "blueprints.syncColError": {
    message: "詳細",
    description: "Sync history table column — errors and durable sync notes",
  },
  "blueprints.manifestTitle": {
    message: "render.yaml マニフェスト",
    description: "Blueprint detail section heading for the stored manifest",
  },
  "blueprints.plannedTitle": {
    message: "計画された同期",
    description:
      "Blueprint detail: heading of the planned-sync card a Blueprint shows while a plan waits for approval",
  },
  "blueprints.plannedBody": {
    message:
      "この計画は承認待ちです。コミット {commit} で承認すると何が変更されるかを確認し、承認して適用してください。",
    description:
      "Planned-sync card: explains plan-then-approve; {commit} is a short commit SHA",
  },
  "blueprints.plannedActions": {
    message: "計画された変更",
    description: "Planned-sync card: accessible label of the action list",
  },
  "blueprints.plannedAction": {
    message: "{operation} {kind} {name}",
    description:
      "Planned-sync card: one plan action, e.g. 'create service api'",
  },
  "blueprints.plannedActionsUnavailable": {
    message:
      "この計画は、計画が変更内容を保持する前に記録されたものです。変更内容を確認するにはもう一度計画するか、承認して適用してください。",
    description:
      "Planned-sync card: a plan stored without its actions (recorded before migration 0187)",
  },
  "blueprints.plannedManualEnvVars": {
    message:
      "これらの値は Blueprint ファイル（sync: false）にありません。承認するには入力してください。",
    description:
      "Planned-sync card: intro to the required manual env var fields",
  },
  "blueprints.plannedManualEnvVarLabel": {
    message: "{service}: {key}",
    description: "Planned-sync card: label of one manual env var field",
  },
  "blueprints.plannedApprove": {
    message: "承認",
    description: "Planned-sync card: button that applies the planned sync",
  },
  "blueprints.plannedReplan": {
    message: "もう一度計画",
    description:
      "Planned-sync card: button that plans a new sync at the branch head",
  },
  "blueprints.plannedNone": {
    message:
      "承認待ちの計画はありません。ブランチの先頭を読み取るには、もう一度計画してください。",
    description:
      "Planned-sync card: shown when a created Blueprint has no approvable plan",
  },
  "blueprints.plannedApproved": {
    message: "計画された同期を承認しました",
    description: "Toast after approving a planned sync",
  },
  "blueprints.plannedApproveFailed": {
    message: "計画された同期を承認できませんでした",
    description: "Toast when approving fails",
  },
  "blueprints.plannedChanged": {
    message:
      "他の誰かがこの Blueprint を再計画しました。最新の計画を確認してから、承認してください。",
    description:
      "Toast when the plan on screen was replaced before it was approved (BLUEPRINT_PLAN_CHANGED)",
  },
  "blueprints.plannedReplanFailed": {
    message: "新しい同期を計画できませんでした",
    description: "Toast when re-planning fails",
  },
  "blueprints.syncButton": {
    message: "同期",
    description: "Button label that triggers an idempotent blueprint re-apply",
  },
  "blueprints.syncConfirmTitle": {
    message: "Blueprint を同期しますか？",
    description: "Sync confirm dialog title",
  },
  "blueprints.syncConfirmBody": {
    message:
      "以下に表示されているコミット時点の render.yaml をそのまま適用します。ブランチへの新しいプッシュは含まれません。すでに一致しているリソースは、そのまま残されます。",
    description: "Sync confirm dialog description",
  },
  "blueprints.syncConfirmAction": {
    message: "同期",
    description: "Sync confirm dialog primary action button label",
  },
  "blueprints.syncCancel": {
    message: "キャンセル",
    description: "Sync confirm dialog cancel button label",
  },
  "blueprints.syncPreviewLoading": {
    message: "リポジトリから同期計画を計算しています…",
    description: "Pre-sync dialog — loading line while blueprintPreview runs",
  },
  "blueprints.syncPreviewInvalid": {
    message:
      "現在ブランチにある Blueprint ファイルに問題があります。今すぐ同期すると、失敗するか、保存済みのマニフェストにフォールバックする可能性があります:",
    description:
      "Pre-sync dialog — heading above validation errors from the fetched manifest",
  },
  "blueprints.syncPreviewUnavailable": {
    message:
      "同期計画を計算できませんでした（プレビューを利用できません）。プレビューが成功した後に更新してください。同期には、確認済みのコミットが必要です。",
    description:
      "Pre-sync dialog — preview transport failure; sync is disabled until a valid preview pins a commit (w8/m41)",
  },
  "blueprints.syncPreviewCommit": {
    message: "確認済みのコミット {sha}",
    description: "Pre-sync dialog — short SHA of the previewed Git revision",
  },
  "blueprints.syncRefreshPreview": {
    message: "プレビューを更新",
    description:
      "Pre-sync dialog — refetch Git + re-validate after BLUEPRINT_SOURCE_CHANGED or an invalid preview",
  },
  "blueprints.syncSourceChanged": {
    message:
      "確認後に Blueprint のソースが変更されました。プレビューを更新して、もう一度確認してください",
    description:
      "Toast when syncBlueprint refuses with BLUEPRINT_SOURCE_CHANGED (path/repo drift or stale pin)",
  },
  "blueprints.syncSuccess": {
    message: "Blueprint を同期しました",
    description: "Toast shown after a successful syncBlueprint call",
  },
  "blueprints.syncError": {
    message: "同期に失敗しました",
    description: "Toast shown when syncBlueprint returns an error",
  },
  "blueprints.syncBusy": {
    message:
      "この同期はもう実行を所有していません（ビジー、中断、または置き換え済み）。新しい同期を開始してください。部分的な処理が再実行されることはありません",
    description:
      "Toast shown when syncBlueprint refuses with BLUEPRINT_SYNC_BUSY (admission race, mid-apply fence, or disconnect-busy)",
  },
  "blueprints.editField": {
    message: "{field} を編集",
    description: "Aria label for the inline-edit pencil button",
  },
  "blueprints.saveField": {
    message: "{field} を保存",
    description: "Aria label for the inline-edit save button",
  },
  "blueprints.cancelEdit": {
    message: "編集をキャンセル",
    description: "Aria label for the inline-edit cancel button",
  },
  "blueprints.pathInvalid": {
    message:
      "リポジトリからの相対パスで、余計な要素を含まない .yaml または .yml のパスにしてください（先頭のスラッシュや '..' は不可）。",
    description: "Inline path editor client-side validation message",
  },
  "blueprints.updateSuccess": {
    message: "Blueprint を更新しました",
    description: "Toast shown after a successful updateBlueprint call",
  },
  "blueprints.updateError": {
    message: "更新に失敗しました",
    description: "Toast shown when updateBlueprint returns an error",
  },
  "blueprints.disconnectButton": {
    message: "接続解除",
    description:
      "Button that disconnects a blueprint from its Git repo (stops auto-sync, keeps resources)",
  },
  "blueprints.disconnectTitle": {
    message: "Blueprint の接続を解除しますか？",
    description: "Disconnect confirm dialog title",
  },
  "blueprints.disconnectBody": {
    message:
      "プッシュ時の自動同期が停止し、Blueprint が一覧から削除されます。すでにデプロイされたリソースには影響しません。",
    description: "Disconnect confirm dialog description",
  },
  "blueprints.disconnectAction": {
    message: "接続解除",
    description: "Disconnect confirm dialog primary action button label",
  },
  "blueprints.disconnectCancel": {
    message: "キャンセル",
    description: "Disconnect confirm dialog cancel button label",
  },
  "blueprints.disconnectSuccess": {
    message: "Blueprint の接続を解除しました",
    description: "Toast shown after a successful disconnectBlueprint call",
  },
  "blueprints.disconnectError": {
    message: "接続の解除に失敗しました",
    description: "Toast shown when disconnectBlueprint returns an error",
  },
  "blueprints.disconnectBusy": {
    message:
      "現在、同期を適用中です。完了してから、もう一度接続を解除してください",
    description:
      "Toast shown when disconnectBlueprint refuses with BLUEPRINT_SYNC_BUSY",
  },
  "blueprints.validateTitle": {
    message: "検証",
    description: "Validate panel section heading",
  },
  "blueprints.validateRun": {
    message: "検証を実行",
    description: "Validate panel button label",
  },
  "blueprints.validateValid": {
    message: "マニフェストは有効です。エラーは見つかりませんでした。",
    description: "Validate result: manifest parsed successfully",
  },
  "blueprints.validateInvalid": {
    message: "マニフェストにエラーがあります:",
    description:
      "Validate result: manifest has parse errors — followed by the error list",
  },
  "blueprints.validateNoResult": {
    message: "結果はまだありません。",
    description: "Validate panel placeholder before first run",
  },
  "blueprints.stateCreated": {
    message: "承認待ち",
    description:
      "Sync state (and Blueprint status) created: a planned sync waiting for approval (w2/m256); never a queued run.",
  },
  "blueprints.stateRunning": {
    message: "実行中",
    description: "Sync-run state: the run is in progress.",
  },
  "blueprints.stateSuccess": {
    message: "成功",
    description: "Sync-run state: the run completed successfully.",
  },
  "blueprints.createNameDescription": {
    message: "この Blueprint インスタンスの一意な名前。",
    description: "Create-Blueprint Name row description",
  },
  "blueprints.createBlockedSource": {
    message: "続行するにはリポジトリを選択してください。",
    description:
      "Create-Blueprint action bar: why Deploy is held while no repository is chosen",
  },
  "blueprints.createBlockedBranch": {
    message: "続行するにはブランチを入力してください。",
    description:
      "Create-Blueprint action bar: why Deploy is held while the branch is empty",
  },
  "blueprints.createBlockedReviewing": {
    message: "Blueprint ファイルを確認しています…",
    description:
      "Create-Blueprint action bar: why Deploy is held while the Blueprint file is being reviewed",
  },
  "blueprints.createBlockedReview": {
    message: "続行するには、確認で見つかった問題を解決してください。",
    description:
      "Create-Blueprint action bar: why Deploy is held while the review found problems in the Blueprint file",
  },
};

export default jaBlueprints;
