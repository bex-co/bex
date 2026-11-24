import type { TranslationEntry } from "@/i18n";

const jaBuildSources: Record<string, TranslationEntry> = {
  "buildSources.pageTitle": {
    message: "ビルドソース",
    description: "Title of the Build Sources list page",
  },
  "buildSources.pageDescription": {
    message:
      "Git リポジトリまたは Docker イメージから一度ビルドし、複数のサービスにデプロイできます。",
    description: "Subtitle of the Build Sources list page",
  },
  "buildSources.resourceType": {
    message: "ビルドソース",
    description: "Singular resource type label shown in global search results",
  },
  "buildSources.newButton": {
    message: "新しいビルドソース",
    description: "Primary CTA button to create a new Build Source",
  },
  "buildSources.backToList": {
    message: "ビルドソース一覧に戻る",
    description: "Back button label returning to /build-sources",
  },
  "buildSources.viewAll": {
    message: "すべて表示",
    description: "Link from Project Overview Build Sources section to /build-sources",
  },
  "buildSources.viewBuildSources": {
    message: "ビルドソースを表示",
    description: "Button in partial-link alert navigating to /build-sources",
  },
  "buildSources.emptyTitle": {
    message: "ビルドソースはまだありません",
    description: "Empty state heading when the workspace has no build sources",
  },
  "buildSources.emptyDescription": {
    message:
      "ビルドソースを作成すると、Web Service・Private Service・Background Worker・Cron Job 間で 1 つの Git ビルドや Docker イメージを共有できます。",
    description: "Empty state explanation on /build-sources",
  },
  "buildSources.refreshError": {
    message:
      "ビルドソースを更新できませんでした。キャッシュ済みの結果を表示しています。",
    description: "Inline error banner when a background refresh fails over cached rows",
  },
  "buildSources.searchPlaceholder": {
    message: "名前、リポジトリ、イメージ、またはプロジェクトで検索…",
    description: "Placeholder for the /build-sources search input",
  },
  "buildSources.searchLabel": {
    message: "ビルドソースを検索",
    description: "Accessible label for the /build-sources search input",
  },
  "buildSources.resetFilters": {
    message: "フィルターをリセット",
    description: "Button to clear active search and kind filters on /build-sources",
  },
  "buildSources.filterKindLabel": {
    message: "ソース種別で絞り込む",
    description: "Accessible label for the source-kind filter button group",
  },
  "buildSources.filterAll": {
    message: "すべて",
    description: "Filter button showing both Git and Docker image build sources",
  },
  "buildSources.kindGit": {
    message: "Git",
    description: "Source kind badge and filter label for Git-backed build sources",
  },
  "buildSources.kindImage": {
    message: "Docker イメージ",
    description: "Source kind badge and filter label for Docker-image-backed build sources",
  },
  "buildSources.noMatchesTitle": {
    message: "一致するビルドソースがありません",
    description: "Heading when search or kind filter matches zero build sources",
  },
  "buildSources.noMatchesDescription": {
    message: "検索条件を変更するか、ソース種別フィルターを解除してください。",
    description: "Body copy when search or kind filter matches zero build sources",
  },
  "buildSources.colName": {
    message: "名前",
    description: "Table column header for Build Source name",
  },
  "buildSources.colSource": {
    message: "ソース",
    description: "Table column header for Git repo/branch or Docker image",
  },
  "buildSources.colRegion": {
    message: "リージョン",
    description: "Table column header for Build Source region",
  },
  "buildSources.colLinkedServices": {
    message: "リンク済みサービス",
    description: "Table column header for linked service count",
  },
  "buildSources.colProject": {
    message: "プロジェクト",
    description: "Table column header for optional project scope",
  },
  "buildSources.colUpdated": {
    message: "更新日時",
    description: "Table column header for last updated timestamp",
  },
  "buildSources.copyId": {
    message: "ビルドソース ID をコピー",
    description: "Accessible label for copying a bds- ID",
  },
  "buildSources.copied": {
    message: "コピーしました",
    description: "Copy button confirmation tooltip",
  },
  "buildSources.copyError": {
    message: "コピーできませんでした",
    description: "Copy button failure tooltip",
  },
  "buildSources.linkedServicesCount_other": {
    message: "{count} サービス",
    description: "Count of services linked to a build source",
  },
  "buildSources.workspaceScope": {
    message: "ワークスペース",
    description: "Label when a build source is workspace-wide (not scoped to a project)",
  },
  "buildSources.createTitle": {
    message: "新しいビルドソース",
    description: "Page heading of /build-sources/new",
  },
  "buildSources.createDescription": {
    message:
      "複数のサービスからデプロイできる共有 Git リポジトリビルドまたは Docker イメージを設定します。",
    description: "Subtitle of /build-sources/new",
  },
  "buildSources.createSubmit": {
    message: "ビルドソースを作成",
    description: "Submit button label on /build-sources/new",
  },
  "buildSources.createSuccess": {
    message: "ビルドソース「{name}」を作成しました",
    description: "Toast shown after creating a Build Source",
  },
  "buildSources.createError": {
    message: "ビルドソースを作成できませんでした。もう一度お試しください。",
    description: "Fallback error toast when createBuildSource fails",
  },
  "buildSources.updateSuccess": {
    message: "ビルドソース「{name}」を更新しました",
    description: "Toast shown after updating a Build Source",
  },
  "buildSources.updateError": {
    message: "ビルドソースを更新できませんでした。もう一度お試しください。",
    description: "Fallback error toast when updateBuildSource fails",
  },
  "buildSources.deleteSuccess": {
    message: "ビルドソース「{name}」を削除しました",
    description: "Toast shown after deleting a Build Source",
  },
  "buildSources.deleteError": {
    message: "ビルドソースを削除できませんでした。もう一度お試しください。",
    description: "Fallback error toast when deleteBuildSource fails",
  },
  "buildSources.triggerBuildSuccess": {
    message: "ビルドを開始しました",
    description: "Toast shown after triggering a Build Source build",
  },
  "buildSources.triggerBuildError": {
    message: "ビルドを開始できませんでした。もう一度お試しください。",
    description: "Fallback error toast when triggerBuild fails",
  },
  "buildSources.createPermissionDenied": {
    message:
      "ビルドソースを作成するには、このワークスペースでの Developer または Admin 権限が必要です。",
    description: "Read-only notice shown on /build-sources/new when can_create is false",
  },
  "buildSources.createBlockedSource": {
    message: "続行するには Git リポジトリまたは Docker イメージを選択してください。",
    description: "Action bar hint while no source is selected",
  },
  "buildSources.partiallyLinkedTitle": {
    message: "ビルドソースは作成されましたが、一部サービスのリンクに失敗しました",
    description: "Alert heading when createBuildSource returns 409 BUILD_SOURCE_PARTIALLY_LINKED",
  },
  "buildSources.partiallyLinkedDescription": {
    message:
      "ビルドソース {id} は作成されましたが、選択したすべてのサービスをリンクできませんでした。",
    description: "Alert summary naming the newly created bds- ID on partial link conflict",
  },
  "buildSources.partiallyLinkedSucceeded": {
    message: "リンク済み: {services}",
    description: "List of services that were successfully linked before the conflict",
  },
  "buildSources.partiallyLinkedFailed": {
    message: "リンク失敗: {service}",
    description: "The service whose link failed during create",
  },
  "buildSources.fieldSource": {
    message: "ソース",
    description: "Create form label for Git repository vs Docker image picker",
  },
  "buildSources.fieldSourceDescription": {
    message:
      "連携済みの GitHub リポジトリ、公開 Git URL、または既存の Docker イメージを選択します。",
    description: "Help text for the source picker row on /build-sources/new",
  },
  "buildSources.fieldName": {
    message: "名前",
    description: "Create form label for Build Source name",
  },
  "buildSources.fieldNameDescription": {
    message: "このワークスペース内で一意の名前を指定します。",
    description: "Help text for Build Source name",
  },
  "buildSources.fieldNamePlaceholder": {
    message: "my-shared-api",
    description: "Placeholder for Build Source name input",
  },
  "buildSources.errorNameTaken": {
    message: "「{name}」という名前のビルドソースはこのワークスペースにすでに存在します。",
    description: "Inline error when BUILD_SOURCE_NAME_TAKEN is returned",
  },
  "buildSources.fieldProject": {
    message: "プロジェクト",
    description: "Create form label for optional Project scope",
  },
  "buildSources.fieldProjectDescription": {
    message: "必要に応じてこのビルドソースを単一のプロジェクトに紐づけます。",
    description: "Help text for optional Project selector",
  },
  "buildSources.fieldProjectNone": {
    message: "なし（ワークスペース全体）",
    description: "Option leaving the build source unscoped at workspace level",
  },
  "buildSources.fieldRuntime": {
    message: "ランタイム",
    description: "Create form label for Git build source runtime",
  },
  "buildSources.fieldRuntimeDescription": {
    message: "ネイティブランタイムまたは Docker を選択します。",
    description: "Help text for Git build source runtime selector",
  },
  "buildSources.fieldBranch": {
    message: "ブランチ",
    description: "Create form label for Git branch",
  },
  "buildSources.fieldBranchDescription": {
    message:
      "ビルド対象の Git ブランチです。未指定の場合はリポジトリのデフォルトブランチが使われます。",
    description: "Help text for Git branch input",
  },
  "buildSources.fieldRegion": {
    message: "リージョン",
    description: "Create form label for Git build source region",
  },
  "buildSources.fieldRegionDescription": {
    message: "Git ビルドソースはリンク先サービスと同じリージョンである必要があります。",
    description: "Help text for Git build source region selector",
  },
  "buildSources.fieldRegionDefault": {
    message: "ワークスペースのデフォルトリージョン",
    description: "Option leaving the build source region unset to use the platform default",
  },
  "buildSources.fieldGitRegistryDescription": {
    message: "Docker ビルド中にプライベートベースイメージを取得するための任意認証情報です。",
    description: "Help text for registry credential on Docker Git build sources",
  },
  "buildSources.fieldEnvGroups": {
    message: "ビルド時環境グループ",
    description: "Label for selecting workspace env groups in build-time environment",
  },
  "buildSources.fieldEnvGroupsDescription": {
    message:
      "ワークスペース単位の環境グループをビルド時にリンクします。特定環境に紐づくグループは選択できません。",
    description: "Help text for build-time env group checkboxes",
  },
  "buildSources.fieldEnvGroupsEmpty": {
    message: "利用可能なワークスペース単位の環境グループはありません。",
    description: "Empty message when no workspace-level env groups exist",
  },
  "buildSources.linkServicesTitle": {
    message: "既存サービスをリンク",
    description: "Section heading for optional initial serviceIds linking",
  },
  "buildSources.linkServicesDescription": {
    message:
      "既存の Web Service・Private Service・Background Worker・Cron Job を任意でリンクできます。Static Site はビルドソースを利用できません。",
    description: "Section description for optional initial serviceIds linking",
  },
  "buildSources.linkServicesEmpty": {
    message: "このワークスペースにリンク可能なサービスはまだありません。",
    description: "Empty message when workspace has no linkable services",
  },
  "buildSources.projectSectionTitle": {
    message: "ビルドソース",
    description: "Heading of the Build Sources section on Project Overview",
  },
  "buildSources.projectLoadError": {
    message: "このプロジェクトのビルドソースを読み込めませんでした。",
    description: "Error text inside Project Overview Build Sources section",
  },
  "buildSources.projectEmptyDescription": {
    message: "このプロジェクトに紐づくビルドソースはまだありません。",
    description: "Empty state text inside Project Overview Build Sources section",
  },
  "buildSources.viewBuildSource": {
    message: "ビルドソースを開く",
    description: "Button navigating to a newly created Build Source detail page",
  },
  "buildSources.detailTabsLabel": {
    message: "ビルドソースのセクション",
    description: "Accessible label for the Build Source detail tab navigation",
  },
  "buildSources.tabBuilds": {
    message: "ビルド",
    description: "Tab label for Build Source build history",
  },
  "buildSources.tabServices": {
    message: "サービス",
    description: "Tab label for Build Source linked services",
  },
  "buildSources.tabEnvironment": {
    message: "環境",
    description: "Tab label for Build Source build-time environment",
  },
  "buildSources.tabSettings": {
    message: "設定",
    description: "Tab label for Build Source settings and danger zone",
  },
  "buildSources.triggerBuildButton": {
    message: "ビルドを実行",
    description: "Primary CTA to trigger a new build on a Git build source",
  },
  "buildSources.triggerSubmitting": {
    message: "実行中…",
    description: "Submit button label while triggerBuild mutation is in flight",
  },
  "buildSources.triggerDialogTitle": {
    message: "ビルドを実行",
    description: "Title of the Trigger Build modal dialog",
  },
  "buildSources.triggerDialogDescription": {
    message: "「{name}」の新しいビルドを開始し、成功時にデプロイする連携サービスを選択します。",
    description: "Description of the Trigger Build modal dialog",
  },
  "buildSources.triggerScopeLegend": {
    message: "ビルド成功後のデプロイ対象",
    description: "Legend for the deploy.serviceScope radio group",
  },
  "buildSources.triggerScopeAutoDeploy": {
    message: "自動デプロイ有効な連携サービス",
    description: "Radio option for deploy.serviceScope=autodeploy_only",
  },
  "buildSources.triggerScopeAutoDeployHint": {
    message: "自動デプロイが有効なすべての連携サービスをデプロイします。",
    description: "Hint text for deploy.serviceScope=autodeploy_only",
  },
  "buildSources.triggerScopeNone": {
    message: "ビルドのみ（デプロイしない）",
    description: "Radio option for deploy.serviceScope=none",
  },
  "buildSources.triggerScopeNoneHint": {
    message: "連携サービスをデプロイせず、ビルド成果物のみを作成します。",
    description: "Hint text for deploy.serviceScope=none",
  },
  "buildSources.triggerScopeCustom": {
    message: "選択したサービス",
    description: "Radio option for deploy.serviceScope=custom",
  },
  "buildSources.triggerScopeCustomHint": {
    message: "ビルド成功後にデプロイする特定の連携サービスを選択します。",
    description: "Hint text for deploy.serviceScope=custom",
  },
  "buildSources.triggerScopeCustomNoServices": {
    message: "このビルドソースにはまだ連携サービスがありません。",
    description: "Message inside custom deploy scope when serviceLinks is empty",
  },
  "buildSources.triggerScopeCustomSelectOne": {
    message: "デプロイするサービスを1つ以上選択してください。",
    description: "Validation hint when custom scope has zero checked services",
  },
  "buildSources.triggerClearCache": {
    message: "ビルドキャッシュをクリア",
    description: "Checkbox label for clearCache=clear",
  },
  "buildSources.triggerClearCacheHint": {
    message: "キャッシュ済みレイヤーを再利用せず、最初からビルドします。",
    description: "Hint text for clearCache checkbox",
  },
  "buildSources.triggerResultTitle": {
    message: "ビルドを開始しました",
    description: "Confirmation heading in Trigger Build dialog after success",
  },
  "buildSources.triggerResultDescription": {
    message: "ビルド {id} をキューに追加しました。",
    description: "Confirmation summary naming the new bld- ID",
  },
  "buildSources.triggerResultDeploysTitle_other": {
    message: "{count} 件のサービスデプロイをキューに追加しました",
    description: "Plural heading listing fan-out deploys queued by triggerBuild",
  },
  "buildSources.triggerResultNoDeploys": {
    message: "このビルドではサービスデプロイはキューに追加されませんでした。",
    description: "Message when triggerBuild returned an empty deploys array",
  },
  "buildSources.triggerResultSkippedTitle_other": {
    message: "{count} 件の連携サービスをスキップしました",
    description: "Plural alert heading when triggerBuild returns skippedServices",
  },
  "buildSources.triggerResultViewLogs": {
    message: "ビルドログを見る",
    description: "Primary action navigating from Trigger Build confirmation to Build Detail",
  },
  "buildSources.skippedReasonSuspended": {
    message: "一時停止中",
    description: "Badge label when a service was skipped because it is suspended",
  },
  "buildSources.buildStatusSucceeded": {
    message: "成功",
    description: "Build status badge for succeeded builds",
  },
  "buildSources.buildStatusFailed": {
    message: "失敗",
    description: "Build status badge for failed builds",
  },
  "buildSources.buildStatusCanceled": {
    message: "キャンセル済み",
    description: "Build status badge for canceled builds",
  },
  "buildSources.buildStatusInProgress": {
    message: "ビルド中",
    description: "Build status badge for in-progress builds",
  },
  "buildSources.buildStatusCreated": {
    message: "待機中",
    description: "Build status badge for newly queued builds",
  },
  "buildSources.buildStatusUnknown": {
    message: "不明",
    description: "Fallback build status badge",
  },
  "buildSources.buildsEmptyTitle": {
    message: "まだビルドがありません",
    description: "Empty state heading on the Builds tab",
  },
  "buildSources.buildsEmptyGitDescription": {
    message: "手動でビルドを実行するか、対象ブランチにプッシュして最初の成果物を作成してください。",
    description: "Empty state description on Builds tab for Git build sources",
  },
  "buildSources.buildsEmptyImageDescription": {
    message: "Docker イメージビルドソースは、連携サービスのデプロイ時にイメージリビジョンを記録します。",
    description: "Empty state description on Builds tab for image build sources",
  },
  "buildSources.buildsRefreshError": {
    message: "ビルド一覧を更新できませんでした。キャッシュ済みの結果を表示しています。",
    description: "Inline error banner when polling builds fails over cached rows",
  },
  "buildSources.buildsLoadMore": {
    message: "さらにビルドを読み込む",
    description: "Pagination button loading older builds",
  },
  "buildSources.colBuildId": {
    message: "ビルド ID",
    description: "Table header for build ID column",
  },
  "buildSources.colStatus": {
    message: "ステータス",
    description: "Table header for build status column",
  },
  "buildSources.colArtifact": {
    message: "コミット / イメージ",
    description: "Table header for commit SHA or image digest column",
  },
  "buildSources.colDuration": {
    message: "所要時間",
    description: "Table header for build duration column",
  },
  "buildSources.colDeploys": {
    message: "デプロイ",
    description: "Table header for fan-out deploys count column",
  },
  "buildSources.colStarted": {
    message: "開始日時",
    description: "Table header for build start timestamp column",
  },
  "buildSources.buildDeploysCount_other": {
    message: "{count} 件のデプロイ",
    description: "Plural badge count of fan-out deploys on a build",
  },
  "buildSources.buildSkippedCount_other": {
    message: "{count} 件スキップ",
    description: "Plural badge count of skipped services on a build",
  },
  "buildSources.backToBuilds": {
    message: "ビルド一覧へ戻る",
    description: "Back link from Build Detail to the Builds tab",
  },
  "buildSources.copyBuildId": {
    message: "ビルド ID をコピー",
    description: "Accessible label for copying a bld- ID",
  },
  "buildSources.buildDetailSubtitle": {
    message: "{name} のビルド",
    description: "Subtitle under the bld- ID on Build Detail",
  },
  "buildSources.buildDetailRefreshError": {
    message: "ビルド詳細を更新できませんでした。最後に読み込んだ状態を表示しています。",
    description: "Inline error banner when build detail polling fails",
  },
  "buildSources.factStarted": {
    message: "開始日時",
    description: "Fact label for build start timestamp",
  },
  "buildSources.factCommit": {
    message: "コミット",
    description: "Fact label for build commit SHA",
  },
  "buildSources.factArtifact": {
    message: "成果物",
    description: "Fact label for built image reference and digest",
  },
  "buildSources.fanoutTitle": {
    message: "連携サービスのデプロイ",
    description: "Card heading listing fan-out deploys on Build Detail",
  },
  "buildSources.fanoutEmpty": {
    message: "このビルドからデプロイされたサービスはまだありません。",
    description: "Empty message inside fan-out deploys card",
  },
  "buildSources.buildLogsSearchLabel": {
    message: "ビルドログを絞り込む",
    description: "Accessible label for search input in BuildLogPanel",
  },
  "buildSources.buildLogsEmptyTitle": {
    message: "まだビルドログがありません",
    description: "Empty state heading in BuildLogPanel",
  },
  "buildSources.buildLogsEmptyBody": {
    message: "ビルドが出力を開始すると、ここにログが表示されます。",
    description: "Empty state body in BuildLogPanel",
  },
  "buildSources.imageBuildLogsNotice": {
    message: "Docker イメージビルドソースは既存のイメージを再利用するため、ビルドログは生成されません。",
    description: "Notice shown on Build Detail for image-backed build sources",
  },
  "buildSources.servicesTabTitle": {
    message: "連携サービス",
    description: "Heading of the Linked Services tab",
  },
  "buildSources.servicesTabDescription": {
    message: "このビルドソースに連携されたサービスは、ビルドで生成された成果物をデプロイします。",
    description: "Description of the Linked Services tab",
  },
  "buildSources.linkExistingService": {
    message: "既存サービスを連携",
    description: "Button opening the Link Existing Service dialog",
  },
  "buildSources.servicesEmptyTitle": {
    message: "連携サービスがありません",
    description: "Empty state heading on Linked Services tab",
  },
  "buildSources.servicesEmptyDescription": {
    message: "既存の Web Service、Private Service、Background Worker、または Cron Job を連携して、このソースのビルドを共有できます。",
    description: "Empty state description on Linked Services tab",
  },
  "buildSources.autoDeployOn": {
    message: "自動デプロイ ON",
    description: "Badge indicating a linked service auto-deploys new builds",
  },
  "buildSources.autoDeployOff": {
    message: "自動デプロイ OFF",
    description: "Badge indicating a linked service requires manual deploys",
  },
  "buildSources.changeOrUnlinkSource": {
    message: "ソースを変更",
    description: "Link to a service's settings page to switch or unlink its source",
  },
  "buildSources.linkDialogTitle": {
    message: "既存サービスを連携",
    description: "Heading of the Link Existing Service dialog",
  },
  "buildSources.linkDialogDescription": {
    message: "ワークスペース内の既存サービスを「{name}」に連携します。連携はデプロイを開始せずに保存されます。",
    description: "Description of the Link Existing Service dialog",
  },
  "buildSources.linkDialogNoEligible": {
    message: "このワークスペースの対象サービスはすべて連携済みか、まだ存在しません。",
    description: "Message when no linkable unlinked services exist",
  },
  "buildSources.linkDialogServiceLabel": {
    message: "サービス",
    description: "Select label in Link Existing Service dialog",
  },
  "buildSources.linkDialogServicePlaceholder": {
    message: "サービスを選択…",
    description: "Select placeholder in Link Existing Service dialog",
  },
  "buildSources.linkDialogRegionMismatch": {
    message: "リージョンが一致しません：この Git ビルドソースは {sourceRegion} ですが、選択したサービスは {serviceRegion} です。",
    description: "Inline error when linking a Git build source to a service in another region",
  },
  "buildSources.linkDialogStartCommandHint": {
    message: "ネイティブランタイムのビルドソースでは、サービスの起動コマンドが必須です。",
    description: "Help text for startCommand input in Link Existing Service dialog",
  },
  "buildSources.linkDialogSubmit": {
    message: "サービスを連携",
    description: "Submit button label in Link Existing Service dialog",
  },
  "buildSources.envImageUnsupportedTitle": {
    message: "Docker イメージソースではビルド時環境を利用できません",
    description: "Fallback heading if an image build source visits /environment",
  },
  "buildSources.envImageUnsupportedBody": {
    message: "ビルド時の環境変数とシークレットファイルは Git ベースのビルドソースでのみ利用できます。",
    description: "Fallback body if an image build source visits /environment",
  },
  "buildSources.envSaveOnlyNote": {
    message: "変更はビルドを開始せずに保存され、次回のビルドから適用されます。",
    description: "Top note on Build-time Environment tab",
  },
  "buildSources.envVarsDescription": {
    message: "このビルドソースのビルド時に利用されます。変更は次回のビルドから反映されます。",
    description: "Environment Variables card description on Build-time Environment tab",
  },
  "buildSources.secretFilesDescription": {
    message: "このビルドソースのビルド時にマウントされます。変更は次回のビルドから反映されます。",
    description: "Secret Files card description on Build-time Environment tab",
  },
  "buildSources.envGroupsPanelDescription": {
    message: "ビルド時に連携するワークスペースレベルの環境グループです。変更はデプロイを行わずに次回ビルドから適用されます。",
    description: "Environment Groups card description on Build-time Environment tab",
  },
  "buildSources.envGroupsNoneLinked": {
    message: "このビルドソースに連携されている環境グループはありません。",
    description: "Empty state when no env groups are linked to a build source",
  },
  "buildSources.settingsIdentityTitle": {
    message: "ID とスコープ",
    description: "Heading of read-only identity card on Settings tab",
  },
  "buildSources.settingsIdentityDescription": {
    message: "このビルドソースの不変な識別子とスコープです。",
    description: "Description of read-only identity card on Settings tab",
  },
  "buildSources.settingsFieldId": {
    message: "ビルドソース ID",
    description: "Label for read-only bds- ID field on Settings tab",
  },
  "buildSources.settingsConfigTitle": {
    message: "ビルド設定",
    description: "Heading of editable settings card on Settings tab",
  },
  "buildSources.settingsNoBuildNote": {
    message: "設定を保存しても、ビルドは自動的に開始されません。",
    description: "Inline note explaining Settings edits do not rebuild",
  },
  "buildSources.settingsFieldRepoUrl": {
    message: "リポジトリ URL",
    description: "Input label for Git repository URL on Settings tab",
  },
  "buildSources.settingsInvalidRepoUrl": {
    message: "有効な Git リポジトリ URL を入力してください。",
    description: "Validation error for malformed repo URL on Settings tab",
  },
  "buildSources.settingsInvalidImageUrl": {
    message: "有効な Docker イメージ参照を入力してください。",
    description: "Validation error for malformed image URL on Settings tab",
  },
  "buildSources.settingsSaveButton": {
    message: "変更を保存",
    description: "Submit button label on Settings tab",
  },
  "buildSources.settingsSaving": {
    message: "保存中…",
    description: "Submit button label while updateBuildSource is in flight",
  },
  "buildSources.dangerZoneTitle": {
    message: "危険な操作",
    description: "Heading of Delete Build Source card on Settings tab",
  },
  "buildSources.dangerZoneDescription": {
    message: "このビルドソースとビルド時設定を完全に削除します。この操作は取り消せません。",
    description: "Description of Delete Build Source card",
  },
  "buildSources.deleteBlockedTitle_other": {
    message: "削除する前に {count} 件のサービスの連携を解除してください",
    description: "Plural alert heading blocking delete when services remain linked",
  },
  "buildSources.deleteBlockedDescription": {
    message: "サービスが参照している間はビルドソースを削除できません。まず各連携サービスを別のソースに切り替えてください：",
    description: "Explanation under delete-blocked alert",
  },
  "buildSources.deleteButton": {
    message: "ビルドソースを削除",
    description: "Destructive button opening the delete confirmation dialog",
  },
  "buildSources.deleteConfirmTitle": {
    message: "「{name}」を削除しますか？",
    description: "Title of Delete Build Source confirmation dialog",
  },
  "buildSources.deleteConfirmBody": {
    message: "このビルドソースとビルド時環境が完全に削除されます。ビルド履歴や設定は復元できません。",
    description: "Body of Delete Build Source confirmation dialog",
  },
  "buildSources.deleteConfirmPrompt": {
    message: "確認のため、以下に {phrase} と入力してください。",
    description: "Sudo command prompt inside Delete Build Source confirmation dialog",
  },
  "buildSources.deleteConfirmButton": {
    message: "ビルドソースを削除",
    description: "Confirm action label inside Delete Build Source confirmation dialog",
  },
};

export default jaBuildSources;
