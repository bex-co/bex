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
};

export default jaBuildSources;
