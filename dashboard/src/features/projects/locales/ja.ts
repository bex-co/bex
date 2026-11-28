import type { TranslationEntry } from "@/i18n";

const jaProjects: Record<string, TranslationEntry> = {
  "projects.overviewTitle": {
    message: "概要",
    description: "Page title of the Overview page (workspace homepage)",
  },
  "projects.projectsHeading": {
    message: "プロジェクト",
    description: "Section heading above the Overview page's project grid",
  },
  "projects.ungroupedHeading": {
    message: "未グループのリソース",
    description:
      "Section heading above the Overview page's ungrouped resource table",
  },
  "projects.allResourcesHeading": {
    message: "すべてのリソース",
    description:
      "Heading above a project page's full resource table, below the environments section",
  },
  "projects.filterTypeLabel": {
    message: "リソースタイプ",
    description: "Accessible label for project environment type filters",
  },
  "projects.filterAll": {
    message: "すべて",
    description: "All project environment resource types",
  },
  "projects.filterServices": {
    message: "サービス",
    description: "Service resource filter",
  },
  "projects.filterDatabases": {
    message: "データベース",
    description: "Database resource filter",
  },
  "projects.filterKeyValues": {
    message: "Key Value",
    description: "Key-value resource filter",
  },
  "projects.filterEnvGroups": {
    message: "環境グループ",
    description: "Environment-group resource filter",
  },
  "projects.searchLabel": {
    message: "{environment} 内のリソースを検索",
    description: "Accessible project environment resource-search label",
  },
  "projects.searchPlaceholder": {
    message: "{environment} 内のリソースを検索",
    description: "Project environment resource-search placeholder",
  },
  "projects.noMatchesTitle": {
    message: "一致するリソースがありません",
    description: "Project environment filtered no-results title",
  },
  "projects.noMatchesBody": {
    message: "別の検索語またはリソースタイプをお試しください。",
    description: "Project environment filtered no-results body",
  },
  "projects.eyebrow": {
    message: "プロジェクト",
    description: "Small uppercase label above a project page's name heading",
  },
  "projects.cardEmpty": {
    message: "リソースはまだありません",
    description: "Project card status line when the project has no resources",
  },
  "projects.cardHealthy": {
    message: "すべてのリソースが稼働中",
    description: "Project card status line when every resource is healthy",
  },
  "projects.cardInProgress_other": {
    message: "{count} 件のリソースが処理中",
    description:
      "Project card status line when resources are provisioning or deploying without a failure (plural)",
  },
  "projects.cardUnhealthy_other": {
    message: "{count} 件のリソースに対応が必要です",
    description:
      "Project card status line when more than one resource is unhealthy",
  },
  "projects.newProjectCard": {
    message: "新しいプロジェクトを作成",
    description:
      'The dashed "+ create project" tile at the end of the project grid',
  },
  "projects.errorTitle": {
    message: "リソースを読み込めませんでした",
    description: "Error state heading on the unified Projects page",
  },
  "projects.errorBody": {
    message:
      "リソースの読み込み中に問題が発生しました。しばらくしてからもう一度お試しください。",
    description: "Error state body on the unified Projects page",
  },
  "projects.emptyTitle": {
    message: "リソースはまだありません",
    description: "Empty state heading on the unified Projects page",
  },
  "projects.emptyBody": {
    message: "サービス、データベース、Key Value ストアを作成して始めましょう。",
    description: "Empty state body on the unified Projects page",
  },
  "projects.emptyCreateService": {
    message: "サービスを作成",
    description: "Primary action of the empty Projects page (ADR075 D1)",
  },
  "projects.newButton": {
    message: "新規",
    description: "Button that opens the new-resource menu",
  },
  "projects.newService": {
    message: "新しいサービス",
    description: "New-resource menu item that starts the service create wizard",
  },
  "projects.newDatabase": {
    message: "新しいデータベース",
    description: "New-resource menu item that opens the database create dialog",
  },
  "projects.newKeyValue": {
    message: "新しい Key Value",
    description: "New-resource menu item that starts the key value create form",
  },
  "projects.colName": {
    message: "名前",
    description: "Merged resource table column header",
  },
  "projects.colType": {
    message: "タイプ",
    description: "Merged resource table column header",
  },
  "projects.colStatus": {
    message: "ステータス",
    description: "Merged resource table column header",
  },
  "projects.colCreated": {
    message: "作成日時",
    description: "Merged resource table column header",
  },
  "projects.colRuntime": {
    message: "ランタイム",
    description:
      "Authoritative runtime/product column in a Project resource table",
  },
  "projects.colRegion": {
    message: "リージョン",
    description:
      "Explicit installation placement column in a Project resource table",
  },
  "projects.colUpdated": {
    message: "更新日時",
    description:
      "Authoritative resource update-time column in a Project resource table",
  },
  "projects.colActions": {
    message: "操作",
    description: "Merged resource table column header (screen-reader only)",
  },
  "projects.typeService": {
    message: "サービス",
    description: "Type badge for a service row in the merged resource table",
  },
  "projects.typeDatabase": {
    message: "データベース",
    description: "Type badge for a database row in the merged resource table",
  },
  "projects.typeKeyValue": {
    message: "Key Value",
    description: "Type badge for a key value row in the merged resource table",
  },
  "projects.typeEnvGroup": {
    message: "環境グループ",
    description: "Type badge for an environment-group row",
  },
  "projects.metadataNotApplicable": {
    message: "該当なし",
    description:
      "Accessible label for a status, runtime or region cell on an environment group row, which has none of them (w4/549)",
  },
  "projects.metadataUnavailable": {
    message: "利用不可",
    description:
      "Accessible label for an unavailable runtime, region, status, or timestamp",
  },
  "projects.selectAllVisible": {
    message: "表示中のリソースをすべて選択",
    description: "Accessible label for the Project table header checkbox",
  },
  "projects.selectResource": {
    message: "{name} を選択",
    description: "Accessible label for one Project resource row checkbox",
  },
  "projects.selectedCount": {
    message: "選択中: {count}",
    description: "Bulk action toolbar selection count",
  },
  "projects.moveTargetLabel": {
    message: "選択したリソースの移動先の環境",
    description: "Accessible label for the bulk Move target selector",
  },
  "projects.moveTargetPlaceholder": {
    message: "環境を選択",
    description: "Placeholder for the bulk Move target selector",
  },
  "projects.moveSelected": {
    message: "移動",
    description:
      "Bulk action that moves selected resources to another Environment",
  },
  "projects.bulkResume": {
    message: "再開",
    description:
      "Selection toolbar action that resumes every selected suspended resource (Render's bulk Resume, w4/m550)",
  },
  "projects.bulkSuspend": {
    message: "一時停止",
    description:
      "Selection toolbar action that suspends every selected running resource (Render's bulk Suspend, w4/m550)",
  },
  "projects.bulkSuspendTitle": {
    message: "サービスを一時停止",
    description: "Title of the bulk Suspend dialog",
  },
  "projects.bulkSuspendBody": {
    message:
      "次のサービスは一時停止され、すぐに動作を停止します。サービスを再開すると元に戻せます。",
    description:
      "Body of the bulk Suspend dialog, above the list of resources it suspends",
  },
  "projects.bulkSuspendConfirm": {
    message: "サービスを一時停止",
    description: "Confirm button of the bulk Suspend dialog",
  },
  "projects.bulkResumeTitle": {
    message: "サービスを再開",
    description: "Title of the bulk Resume dialog",
  },
  "projects.bulkResumeBody": {
    message: "次のサービスを再開します:",
    description:
      "Body of the bulk Resume dialog, above the list of resources it resumes",
  },
  "projects.bulkResumeConfirm": {
    message: "サービスを再開",
    description: "Confirm button of the bulk Resume dialog",
  },
  "projects.bulkRetry": {
    message: "再試行",
    description:
      "Button on a bulk Suspend/Resume row whose call failed; runs that row again",
  },
  "projects.bulkRetryRow": {
    message: "{name} を再試行",
    description:
      "Accessible label of a row's Retry button ({name} = the resource)",
  },
  "projects.bulkRowSuspending": {
    message: "一時停止中",
    description:
      "Screen-reader state of a bulk Suspend row whose call is in flight",
  },
  "projects.bulkRowSuspended": {
    message: "一時停止済み",
    description: "Screen-reader state of a bulk Suspend row that succeeded",
  },
  "projects.bulkRowResuming": {
    message: "再開中",
    description:
      "Screen-reader state of a bulk Resume row whose call is in flight",
  },
  "projects.bulkRowResumed": {
    message: "再開済み",
    description: "Screen-reader state of a bulk Resume row that succeeded",
  },
  "projects.bulkRowError": {
    message: "リクエストに失敗しました。もう一度お試しください。",
    description:
      "Fallback reason on a bulk Suspend/Resume row whose call failed without a server reason",
  },
  "projects.newProjectButton": {
    message: "新しいプロジェクト",
    description: "Button that opens the new-project dialog",
  },
  "projects.createTitle": {
    message: "新しいプロジェクト",
    description: "New-project dialog title",
  },
  "projects.createDescription": {
    message:
      "サービス、データベース、Key Value ストアをまとめて管理します。すべてのプロジェクトは 1 つの環境から始まります。",
    description: "New-project dialog description",
  },
  "projects.fieldName": {
    message: "名前",
    description: "New-project dialog name field label",
  },
  "projects.fieldFirstEnvironment": {
    message: "最初の環境の名前",
    description:
      "New-project dialog: name of the environment created with the project (w4/m264, Render parity)",
  },
  "projects.fieldNamePlaceholder": {
    message: "my-project",
    description: "New-project dialog name field placeholder",
  },
  "projects.createCancel": {
    message: "キャンセル",
    description: "New-project dialog cancel button",
  },
  "projects.createSubmit": {
    message: "プロジェクトを作成",
    description: "New-project dialog submit button",
  },
  "projects.createSuccess": {
    message: "プロジェクト「{name}」を作成しました。",
    description: "Toast shown after a project is created",
  },
  "projects.createError": {
    message: "プロジェクト「{name}」を作成できませんでした。",
    description: "Toast shown when creating a project fails",
  },
  "projects.editNameButton": {
    message: "編集",
    description:
      "Accessible label for the project page's inline pencil-edit button, and the Settings page's Project Name \"Edit\" button",
  },
  "projects.renameFieldLabel": {
    message: "プロジェクト名",
    description:
      'Accessible/visible label for the project rename field, on both the overview dialog and the Settings card. Deliberately more specific than projects.fieldName\'s bare "Name": these controls are identified by value alone otherwise (w4/134)',
  },
  "projects.renameTitle": {
    message: "プロジェクト名を変更",
    description: "Rename-project dialog title",
  },
  "projects.renameSubmit": {
    message: "保存",
    description: "Rename-project dialog submit button",
  },
  "projects.renameSuccess": {
    message: "プロジェクト名を「{name}」に変更しました。",
    description: "Toast shown after a project is renamed",
  },
  "projects.renameRefreshFailed": {
    message:
      "プロジェクト名を「{name}」に変更しましたが、このページを更新できませんでした。もう一度試すか、再読み込みして保存された名前を確認してください。",
    description:
      "Warning after the rename saved but re-reading the project failed; the rename must not be retried or reported as failed (w4/m186)",
  },
  "projects.renameError": {
    message: "プロジェクト名を「{name}」に変更できませんでした。",
    description: "Toast shown when renaming a project fails",
  },
  "projects.settingsTitle": {
    message: "プロジェクトの設定",
    description: "Project settings page heading",
  },
  "projects.nameCardTitle": {
    message: "プロジェクト名",
    description: "Project settings page's name card title",
  },
  "projects.nameCardDescription": {
    message: "プロジェクトの一意な名前",
    description: "Project settings page's name card description",
  },
  "projects.deleteCardTitle": {
    message: "プロジェクトを削除",
    description: "Project settings page's danger-zone card title",
  },
  "projects.deleteCardDescription": {
    message:
      "環境と、その中のすべてのサービス、データベース、Key Value ストアも一緒に削除されます。環境グループは未割り当てになります。この操作は元に戻せません。",
    description: "Project settings page's danger-zone card description",
  },
  "projects.deleteCardButton": {
    message: "プロジェクトを削除",
    description:
      "Project settings page's danger-zone card button, and the delete-confirmation dialog's confirm button",
  },
  "projects.deleteConfirmTitle": {
    message: "プロジェクト「{name}」を削除しますか？",
    description: "Delete-project confirmation dialog title",
  },
  "projects.deleteConfirmBody": {
    message:
      "環境とその構成が削除されます。環境グループは削除されず、未割り当てになります。",
    description: "Delete-project confirmation dialog body",
  },
  "projects.deleteConfirmBodyResources": {
    message:
      "このプロジェクトを削除すると、すべての環境と、その中のサービス、データベース、Key Value ストアが削除されます: {resources}。環境グループは削除されず、未割り当てになります。この操作は元に戻せません。",
    description:
      "Delete-project confirmation body naming every service, database and key value store it deletes, as the environment delete does (w4/m264, w6/m290, w4/542)",
  },
  "projects.deleteSuccess": {
    message: "プロジェクト「{name}」を削除しました。",
    description: "Toast shown after a project is deleted",
  },
  "projects.deleteError": {
    message: "プロジェクト「{name}」を削除できませんでした。",
    description: "Toast shown when deleting a project fails",
  },
  "projects.moveToProject": {
    message: "プロジェクトに移動",
    description:
      "Row-actions submenu label for assigning a resource to a project",
  },
  "projects.removeFromProject": {
    message: "プロジェクトから削除",
    description:
      "Row-actions submenu item that unassigns a resource from its project",
  },
  "projects.moveSuccess": {
    message: "「{name}」を「{project}」に移動しました。",
    description: "Toast shown after moving a resource to a project",
  },
  "projects.moveError": {
    message: "「{name}」を移動できませんでした。",
    description: "Toast shown when moving a resource to a project fails",
  },
  "projects.removeSuccess": {
    message: "「{name}」をプロジェクトから削除しました。",
    description: "Toast shown after unassigning a resource from its project",
  },
  "projects.removeError": {
    message: "「{name}」をプロジェクトから削除できませんでした。",
    description: "Toast shown when unassigning a resource fails",
  },
  "projects.editProjectName": {
    message: "プロジェクト名を編集",
    description:
      "Screen-reader text of the project page's name link, which opens Project Settings with the Name card editing (Render's words, w1/m682)",
  },
  "projects.addEnvironment": {
    message: "環境を追加",
    description:
      "The project page header's button and the card under a project's only environment that open the new-environment dialog (Render's words, w1/m682)",
  },
  "projects.environmentLimitTitle": {
    message: "{plan} で環境を増やしましょう",
    description:
      "Title of the paywall Add environment opens at the plan's environments per project; {plan} is the plan name (Render's words, w1/m682)",
  },
  "projects.environmentLimitBody": {
    message:
      "アップグレードすると、環境を無制限に作成でき、{plan} プランのすべての機能を利用できます。",
    description:
      "Body of the environments-per-project paywall; {plan} is the plan name (Render's words, w1/m682)",
  },
  "projects.environmentLimitUpgrade": {
    message: "アップグレード",
    description:
      "The environments paywall's button to Render's plan page (w3/m708)",
  },
  "projects.environmentLimitLearnMore": {
    message: "詳細を見る",
    description:
      "The environments paywall's link to the projects and environments docs",
  },
};

export default jaProjects;
