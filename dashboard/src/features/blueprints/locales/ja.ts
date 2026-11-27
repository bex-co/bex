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
    description:
      "Generate Blueprint: the selection bar's, a resource row menu's and a project environment menu's entry that opens the Create Blueprint page (w3/m681)",
  },
  "blueprints.generatePageTitle": {
    message: "サービスから新しい Blueprint を作成",
    description:
      "Create Blueprint page title and document title, opened from a selection bar or a resource row's menu (w3/m681)",
  },
  "blueprints.generatePageTitleEnvironment": {
    message: "{project} / {environment} から新しい Blueprint を作成",
    description:
      "Create Blueprint page title and document title, opened from a project environment's menu: the project and environment names",
  },
  "blueprints.generateBreadcrumb": {
    message: "Blueprint を作成",
    description:
      "Topbar breadcrumb of the Create Blueprint page (/blueprint/generate)",
  },
  "blueprints.generateStepDownload": {
    message: "生成された render.yaml をダウンロード",
    description: "Create Blueprint page: step 1 title",
  },
  "blueprints.generateIncludes_other": {
    message: "{count} 件のサービスを含みます:",
    description:
      "Create Blueprint page: step 1 sentence before the list of exported resource names (services, databases and Key Value all count)",
  },
  "blueprints.generating": {
    message: "render.yaml を生成しています...",
    description: "Create Blueprint page: step 1 while render.yaml is generated",
  },
  "blueprints.generateCopiedLabel": {
    message: "コピーしました!",
    description:
      "Create Blueprint page: the Copy button's label for 2 seconds after copying render.yaml",
  },
  "blueprints.generateStepCommit": {
    message: "render.yaml を Git に追加",
    description: "Create Blueprint page: step 2 title",
  },
  "blueprints.generateStepCommitBody": {
    message:
      "render.yaml をリポジトリのルートディレクトリにコミットし、Git プロバイダーにプッシュしてください。",
    description: "Create Blueprint page: step 2 text above the git commands",
  },
  "blueprints.generateStepConnect": {
    message: "新しい Blueprint を接続",
    description: "Create Blueprint page: step 3 title",
  },
  "blueprints.generateStepConnectBody": {
    message:
      "新しい Blueprint インスタンスを作成し、リポジトリを接続してください。",
    description: "Create Blueprint page: step 3 text, followed by a docs link",
  },
  "blueprints.generateSeeDocs": {
    message: "ドキュメントをご覧ください。",
    description: "Create Blueprint page: step 3 link to the Blueprint docs",
  },
  "blueprints.generateConnect": {
    message: "新しい Blueprint を接続する",
    description:
      "Create Blueprint page: step 3 button, opens the New Blueprint Instance page in a new tab",
  },
  "blueprints.generateHelpfulResources": {
    message: "参考リソース",
    description:
      "Create Blueprint page: heading of the links card beside the steps",
  },
  "blueprints.generateLearnBlueprints": {
    message: "Blueprint について",
    description:
      "Create Blueprint page: link to the Blueprint docs in the Helpful Resources card",
  },
  "blueprints.copied": {
    message: "コピーしました",
    description: "Toast after copying a code snippet on a Blueprint page",
  },
  "blueprints.generateCopy": {
    message: "コピー",
    description: "Generate dialog — copy the yaml to the clipboard",
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
  "blueprints.metaCreated": {
    message: "作成日時",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaUpdated": {
    message: "最終同期",
    description: "Blueprint detail metadata label",
  },
  "blueprints.detailsTitle": {
    message: "詳細",
    description:
      "Blueprint Settings page: title of bex's card of read-only facts (repository, created, last synced) under Render's Settings card (w3/m679)",
  },
  "blueprints.settingsNameHint": {
    message: "Blueprint の一意の名前です。",
    description:
      "Blueprint Settings card: help under Blueprint Name (Render's copy, w3/m679)",
  },
  "blueprints.settingsRequired": {
    message: "必須",
    description:
      "Blueprint Settings card: shown under a row left empty (Render's copy, w3/m679)",
  },
  "blueprints.settingsNameTaken": {
    message: "この名前はすでに使用されています",
    description:
      "Blueprint Settings card: shown under Blueprint Name when another Blueprint of the workspace has the name (Render's copy, w3/m679)",
  },
  "blueprints.settingsAutoSync": {
    message: "自動同期",
    description:
      "Blueprint Settings card: the Auto Sync row's label (Render's copy, w3/m679)",
  },
  "blueprints.settingsAutoSyncHint": {
    message:
      "Blueprint ファイルの変更を自動的に同期しますか？同期を手動で行う場合は「いいえ」を選択してください。",
    description:
      "Blueprint Settings card: help under Auto Sync; quotes the No option (Render's copy, w3/m679)",
  },
  "blueprints.settingsYes": {
    message: "はい",
    description:
      "Blueprint Settings card: Auto Sync option, on (Render's copy, w3/m679)",
  },
  "blueprints.settingsNo": {
    message: "いいえ",
    description:
      "Blueprint Settings card: Auto Sync option, off (Render's copy, w3/m679)",
  },
  "blueprints.settingsBranch": {
    message: "ブランチ",
    description: "Blueprint Settings card: the Branch row's label (w3/m679)",
  },
  "blueprints.settingsBranchHint": {
    message: "ビルドしてデプロイする Git ブランチです。",
    description:
      "Blueprint Settings card: help under Branch (Render's copy, w3/m679)",
  },
  "blueprints.settingsBranchNotFound": {
    message:
      "このリポジトリにブランチが見つかりません。別のブランチを選択してください。",
    description:
      "Blueprint Settings card: shown under Branch when the repository has no such branch (Render's copy, w3/m679)",
  },
  "blueprints.settingsPath": {
    message: "Blueprint パス",
    description:
      "Blueprint Settings card: the Blueprint Path row's label (Render's copy, w3/m679)",
  },
  "blueprints.settingsPathHint": {
    message:
      "リポジトリ内の Blueprint ファイルへのパスです（例: infra/render.yaml）。デフォルトは、リポジトリのルートにある render.yaml です。",
    description:
      "Blueprint Settings card: help under Blueprint Path (Render's copy, w3/m679)",
  },
  "blueprints.settingsFileNotFound": {
    message:
      "Blueprint ファイル「{path}」が「{branch}」ブランチに見つかりません。このブランチにファイルが存在することを確認してください。",
    description:
      "Blueprint Settings card: shown under Branch or Blueprint Path when the Blueprint file is not at that path on that branch (Render's copy, w3/m679)",
  },
  "blueprints.settingsUpdateFailed": {
    message: "Blueprint を更新できませんでした。もう一度お試しください。",
    description:
      "Blueprint Settings card: shown under a row whose save failed with no reason from the server (Render's copy, w3/m679)",
  },
  "blueprints.disconnectNoSync": {
    message: "リソースは Blueprint ファイルから自動的に同期されなくなります。",
    description:
      "Disconnect Blueprint modal: first sentence (Render's copy, w3/m679)",
  },
  "blueprints.disconnectKeeps": {
    message:
      "ファイル自体は削除されません。管理対象のリソースも削除されません。ファイルは後からいつでも Blueprints ページで接続できます。",
    description:
      "Disconnect Blueprint modal: second sentence (Render's copy, w3/m679)",
  },
  "blueprints.disconnectConfirm": {
    message: "Blueprint 仕様 {name} の接続を解除してもよろしいですか？",
    description:
      "Disconnect Blueprint modal: the question, with the Blueprint's name in bold (Render's copy, w3/m679)",
  },
  "blueprints.resourcesEmpty": {
    message: "この Blueprint が管理するリソースはありません",
    description:
      "A Blueprint's Resources page when it manages nothing, Render's words (w3/m680)",
  },
  "blueprints.resourcesFileProblems_other": {
    message: "Blueprint ファイルとの同期中に問題が発生しました:",
    description:
      "A Blueprint's Resources page when its newest sync failed on its file: the line above the file's errors, Render's 'There was a problem' / 'There were a few problems' (w3/m680)",
  },
  "blueprints.colBuildSourceName": {
    message: "ビルドソース名",
    description:
      "A Blueprint's Resources page: the Build Sources table's name column, Render's 'Build Source Name' (w3/m680)",
  },
  "blueprints.buildSourceUnlinked": {
    message: "未リンク",
    description:
      "A Blueprint's Build Sources table: badge on a build source no service builds from, Render's 'Unlinked' (w3/m680)",
  },
  "blueprints.buildSourcesUngrouped": {
    message: "未グループ",
    description:
      "A Blueprint's Build Sources table: the row over the build sources in no project, Render's 'Ungrouped' (w3/m680)",
  },
  "blueprints.syncHistoryEmpty": {
    message: "同期はまだありません。",
    description: "Syncs page: no syncs yet, Render's wording (w3/m678)",
  },
  "blueprints.detachPreviewTitle": {
    message: "リソースは管理対象外になります",
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
  "blueprints.manifestTitle": {
    message: "render.yaml マニフェスト",
    description: "Blueprint detail section heading for the stored manifest",
  },
  "blueprints.plannedActions": {
    message: "計画された変更",
    description:
      "A sync's page: accessible label of its list of planned actions (w3/m678)",
  },
  "blueprints.plannedAction": {
    message: "{operation} {kind} {name}",
    description:
      "Planned-sync card: one plan action, e.g. 'create service api'",
  },
  "blueprints.plannedManualEnvVars": {
    message:
      "これらの値は Blueprint ファイル（sync: false）にありません。承認するには入力してください。",
    description:
      "A waiting sync's page: intro to the fields for the sync: false values approving it needs (w3/m678)",
  },
  "blueprints.plannedManualEnvVarLabel": {
    message: "{service}: {key}",
    description:
      "A waiting sync's page: label of one sync: false value's field (w3/m678)",
  },
  "blueprints.plannedApprove": {
    message: "承認",
    description:
      "A waiting sync's page: the button that applies the planned sync, Render's Approve (w3/m678)",
  },
  "blueprints.plannedChanged": {
    message:
      "他の誰かがこの Blueprint を再計画しました。最新の計画を確認してから、承認してください。",
    description:
      "A sync's page: inline alert when the plan on screen was replaced before it was approved (BLUEPRINT_PLAN_CHANGED)",
  },
  "blueprints.plannedReplanFailed": {
    message: "新しい同期を計画できませんでした",
    description:
      "Sync Error banner: Manual sync could not plan a sync and the server gave no reason (w3/m678)",
  },
  "blueprints.syncButton": {
    message: "手動同期",
    description:
      "The Blueprint header's one action, Render's Manual sync: plans a sync and opens its page (w3/m678)",
  },
  "blueprints.editField": {
    message: "{field} を編集",
    description: "Aria label for the inline-edit pencil button",
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
  "blueprints.disconnectButton": {
    message: "Blueprint の接続を解除",
    description:
      "Blueprint Settings page: the critical button that opens the Disconnect Blueprint modal (Render's copy, w3/m679)",
  },
  "blueprints.disconnectTitle": {
    message: "Blueprint の接続を解除",
    description: "Disconnect Blueprint modal title (Render's copy, w3/m679)",
  },
  "blueprints.disconnectAction": {
    message: "Blueprint の接続を解除",
    description:
      "Disconnect Blueprint modal: the critical confirm button (Render's copy, w3/m679)",
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
  "blueprints.navResources": {
    message: "リソース",
    description:
      "Blueprint sidebar entry and page heading for the resources the Blueprint manages (Render's Resources, /blueprints/<id>/resources, w3/m677)",
  },
  "blueprints.navSyncs": {
    message: "同期履歴",
    description:
      "Blueprint sidebar entry and page heading for the Blueprint's syncs (Render's Syncs, /blueprints/<id>/syncs, w3/m677)",
  },
  "blueprints.navSettings": {
    message: "設定",
    description:
      "Blueprint sidebar entry and page heading for the Blueprint's settings (Render's Settings, /blueprints/<id>/settings, w3/m677)",
  },
  "blueprints.resourcesViewSyncs": {
    message: "同期履歴へ移動",
    description:
      "Link from a Blueprint's empty Resources page to its Syncs page, where the waiting plan or the failed sync is (w3/m677)",
  },
  "blueprints.syncViewDetails": {
    message: "詳細を表示",
    description:
      "Sync card: link to the sync's own page (Render's View details, w3/m678)",
  },
  "blueprints.syncsLoadMore": {
    message: "さらに読み込む",
    description:
      "Syncs page: button reading the next page of older syncs (w3/m678)",
  },
  "blueprints.syncsLoadMoreError": {
    message: "以前の同期を読み込めませんでした。",
    description:
      "Syncs page: toast when reading an older page of syncs fails (w3/m678)",
  },
  "blueprints.syncPageTitle": {
    message: "同期: {sha}",
    description:
      "A sync's page heading, Render's 'Sync: <short commit id>'; {sha} is the 7-character commit id (w3/m678)",
  },
  "blueprints.syncPageTitleNoCommit": {
    message: "同期",
    description:
      "A sync's page heading when the sync read no commit, Render's 'Sync' (w3/m678)",
  },
  "blueprints.syncDocumentTitle": {
    message: "同期",
    description:
      "Document title's page name on a sync's page: 'Sync ・ <name> ・ Blueprint' (w3/m678)",
  },
  "blueprints.syncUpToDate": {
    message: "リソースはすでに最新です",
    description:
      "A sync's page when its plan has no actions, Render's wording (w3/m678)",
  },
  "blueprints.syncNotFullySynced": {
    message: "これらのリソースを完全には同期できませんでした。",
    description:
      "A failed sync's page with no actions and no errors, Render's critical line (w3/m678)",
  },
  "blueprints.syncNotFullySyncedBody": {
    message: "このエラーが続く場合は、サポートにお問い合わせください。",
    description:
      "Under blueprints.syncNotFullySynced: what to do if it keeps happening (w3/m678)",
  },
  "blueprints.syncActionsNotRecorded": {
    message:
      "この同期はアクションが保存されるようになる前に記録されたため、アクションを表示できません。",
    description:
      "A sync's page for a sync recorded before every sync kept its actions (w3/m678)",
  },
  "blueprints.syncActionCreate": {
    message: "{kind} {name} を作成",
    description:
      "A sync's planned action: create a resource; {kind} is its kind label, {name} its name, drawn bold (w3/m678)",
  },
  "blueprints.syncActionUpdate": {
    message: "{kind} {name} を更新",
    description:
      "A sync's planned action: change a resource; {kind} is its kind label, {name} its name (w3/m678)",
  },
  "blueprints.syncActionDetach": {
    message: "{kind} {name} の管理を停止",
    description:
      "A sync's planned action: stop managing a resource the file no longer names; {kind}, {name} (w3/m678)",
  },
  "blueprints.syncActionError": {
    message: "{kind} {name} を適用できません",
    description:
      "A sync's planned action the planner refused; {kind}, {name}; the reason follows (w3/m678)",
  },
  "blueprints.syncActionDetachNote": {
    message: "引き続き実行され、料金が発生する場合があります。",
    description:
      "Under a 'stop managing' action: the resource is not deleted (w3/m678)",
  },
  "blueprints.syncActionStateUnknown": {
    message: "状態不明",
    description:
      "Accessible name of a planned action's state icon when bex can't say which state it reached (a failed sync's actions, w3/m678)",
  },
  "blueprints.syncApproveFailed": {
    message: "この同期を承認できませんでした。もう一度お試しください。",
    description:
      "A sync's page: inline alert when Approve fails without the server's reason, Render's wording (w3/m678)",
  },
  "blueprints.syncNotFound": {
    message:
      "この同期は存在しません。24 時間以内に承認されなかった計画済みの同期は削除されます。",
    description:
      "A sync's page for an id that is not one of this Blueprint's syncs, e.g. a plan deleted after 24 hours (w3/m678)",
  },
  "blueprints.syncErrorBannerTitle": {
    message: "同期エラー",
    description:
      "Title of Render's dismissible banner when Manual sync could not plan a sync (w3/m678)",
  },
  "blueprints.syncErrorBannerDismiss": {
    message: "閉じる",
    description:
      "Accessible name of the Sync Error banner's close button (w3/m678)",
  },
  "blueprints.syncErrorsIntro_other": {
    message: "Blueprint との同期中に問題が発生しました:",
    description:
      "A sync's page: the line above its errors, Render's 'There was a problem' / 'There were a few problems' (w3/m678)",
  },
};

export default jaBlueprints;
