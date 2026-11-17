import type { TranslationEntry } from "@/i18n";

const jaEnvGroups: Record<string, TranslationEntry> = {
  "envGroups.resourceType": {
    message: "環境グループ",
    description: "Environment Group resource type used in document titles",
  },
  "envGroups.pageTitle": {
    message: "環境グループ",
    description: "Workspace env-groups page title",
  },
  "envGroups.pageDescription": {
    message: "環境変数とシークレットファイルを複数のサービスで共有します。",
    description: "Workspace env-groups page subtitle",
  },
  "envGroups.newButton": {
    message: "新規環境グループ",
    description: "Open the create env-group dialog",
  },
  "envGroups.createTitle": {
    message: "環境グループを作成",
    description: "Create env-group dialog title",
  },
  "envGroups.createDescription": {
    message:
      "変数、シークレットファイル、サービスのリンクを今すぐ追加することも、空のままにすることもできます。",
    description: "Create env-group dialog description",
  },
  "envGroups.createVarsTitle": {
    message: "環境変数",
    description: "Initial environment variables section title",
  },
  "envGroups.createVarsDescription": {
    message: "リテラル値を設定するか、作成時に強力なシークレットを生成します。",
    description: "Initial environment variables section description",
  },
  "envGroups.varKeyLabel": {
    message: "キー",
    description: "Initial environment variable key label",
  },
  "envGroups.varKeyPlaceholder": {
    message: "API_TOKEN",
    description: "Initial environment variable key placeholder",
  },
  "envGroups.varValueLabel": {
    message: "値",
    description: "Initial environment variable value label",
  },
  "envGroups.varValuePlaceholder": {
    message: "シークレット値",
    description: "Initial environment variable value placeholder",
  },
  "envGroups.generateValue": {
    message: "生成",
    description: "Generate an initial environment variable value",
  },
  "envGroups.addInitialVar": {
    message: "環境変数を追加",
    description: "Add an initial environment variable row",
  },
  "envGroups.importEnv": {
    message: ".env からインポート",
    description:
      "Import initial environment variables from dotenv text or file",
  },
  "envGroups.createFilesTitle": {
    message: "シークレットファイル",
    description: "Initial secret files section title",
  },
  "envGroups.createFilesDescription": {
    message: "ファイルは暗号化され、/etc/secrets 配下にマウントされます。",
    description: "Initial secret files section description",
  },
  "envGroups.fileNameLabel": {
    message: "ファイル名",
    description: "Initial secret file name label",
  },
  "envGroups.fileNamePlaceholder": {
    message: "credentials.json",
    description: "Initial secret file name placeholder",
  },
  "envGroups.fileContentLabel": {
    message: "内容",
    description: "Initial secret file contents label",
  },
  "envGroups.fileContentPlaceholder": {
    message: "ファイルの内容",
    description: "Initial secret file contents placeholder",
  },
  "envGroups.addInitialFile": {
    message: "シークレットファイルを追加",
    description: "Add an initial secret file row",
  },
  "envGroups.createServicesTitle": {
    message: "リンクされたサービス",
    description: "Initial service links section title",
  },
  "envGroups.createServicesDescription": {
    message:
      "選択したサービスは、グループの作成と同時にこのグループを受け取ります。",
    description: "Initial service links section description",
  },
  "envGroups.servicesLoading": {
    message: "ワークスペースのサービスを読み込み中…",
    description: "Initial service links loading state",
  },
  "envGroups.createScopeLoading": {
    message: "環境とサービスを読み込み中…",
    description:
      "Create dialog waits for a confirmed scope and link candidates",
  },
  "envGroups.createScopeError": {
    message:
      "環境またはサービスを読み込めませんでした。グループを作成する前に再試行してください。",
    description:
      "Create dialog blocks submission when scope or service lookup fails",
  },
  "envGroups.noServicesToLink": {
    message: "このワークスペースには、リンクできるサービスがまだありません。",
    description: "Initial service links empty state",
  },
  "envGroups.noServicesInScope": {
    message:
      "この環境にサービスはありません。サービスは、自身の環境にあるグループにのみリンクできます。",
    description: "Initial service links empty state for the picked scope",
  },
  "envGroups.invalidInitialContents": {
    message:
      "グループを作成する前に、無効な変数キーまたはファイル名を修正してください。",
    description: "Invalid initial env-group contents message",
  },
  "envGroups.nameLabel": {
    message: "グループ名",
    description: "Env-group name field label",
  },
  "envGroups.namePlaceholder": {
    message: "shared-production",
    description: "Env-group name placeholder",
  },
  "envGroups.invalidName": {
    message: "グループ名を入力してください。",
    description: "Invalid env-group name message",
  },
  "envGroups.cancel": {
    message: "キャンセル",
    description: "Cancel an env-group dialog",
  },
  "envGroups.createSubmit": {
    message: "環境グループを作成",
    description: "Create env-group submit button",
  },
  "envGroups.emptyTitle": {
    message: "環境グループはありません",
    description: "Workspace env-groups empty-state title",
  },
  "envGroups.emptyBody": {
    message:
      "サービスにリンクする前に、共有設定を管理するグループを作成してください。",
    description: "Workspace env-groups empty-state body",
  },
  "envGroups.serviceCount_other": {
    message: "{count} 件のリンク済みサービス",
    description: "Env-group linked-service count (plural)",
  },
  "envGroups.metadataTitle": {
    message: "グループのメタデータ",
    description: "Env-group metadata card title",
  },
  "envGroups.ownerLabel": {
    message: "オーナー",
    description: "Env-group owner id label",
  },
  "envGroups.environmentLabel": {
    message: "環境",
    description: "Environment Group scope label",
  },
  "envGroups.workspaceLabel": {
    message: "ワークスペース",
    description: "Destination workspace label",
  },
  "envGroups.workspaceScope": {
    message: "ワークスペース（環境なし）",
    description: "Standalone Environment Group scope",
  },
  "envGroups.environmentLookupLoading": {
    message: "環境を読み込み中…",
    description:
      "Group metadata Environment value while the scope lookup is still loading",
  },
  "envGroups.environmentLookupFailed": {
    message: "環境を読み込めませんでした",
    description:
      "Group metadata Environment value when the scope lookup request failed",
  },
  "envGroups.unknownEnvironment": {
    message: "不明な環境 ({id})",
    description: "Deleted or inaccessible Environment fallback",
  },
  "envGroups.createdAtLabel": {
    message: "作成日時",
    description: "Env-group created timestamp label",
  },
  "envGroups.updatedAtLabel": {
    message: "更新日時",
    description: "Env-group updated timestamp label",
  },
  "envGroups.availabilityLabel": {
    message: "ステータス",
    description: "Env-group availability label when busy or repair-required",
  },
  "envGroups.availabilityBusy": {
    message: "保存中です。しばらくしてからもう一度お試しください",
    description: "Busy environment-group availability message",
  },
  "envGroups.availabilityRepairRequired": {
    message: "復旧が必要です。問題が続く場合はサポートにお問い合わせください",
    description: "Repair-required environment-group availability message",
  },
  "envGroups.unavailableTitle": {
    message: "環境グループは利用できません",
    description: "Secret store unavailable state title",
  },
  "envGroups.unavailableBody": {
    message: "このデプロイメントでは、シークレットストアが設定されていません。",
    description: "Secret store unavailable state body",
  },
  "envGroups.forbiddenTitle": {
    message: "権限がありません",
    description: "Env-group forbidden state title",
  },
  "envGroups.forbiddenBody": {
    message: "環境グループを表示または管理する権限がありません。",
    description: "Env-group forbidden state body",
  },
  "envGroups.genericTitle": {
    message: "環境グループを読み込めませんでした",
    description: "Env-group generic query error title",
  },
  "envGroups.genericBody": {
    message: "問題が発生しました。もう一度お試しください。",
    description: "Env-group generic query error body",
  },
  "envGroups.errorTitle": {
    message: "グループの内容を読み込めませんでした",
    description: "Env-group editor error title",
  },
  "envGroups.errorBody": {
    message: "問題が発生しました。もう一度お試しください。",
    description: "Env-group editor error body",
  },
  "envGroups.backToList": {
    message: "環境グループに戻る",
    description: "Detail-page back button label",
  },
  "envGroups.renameButton": {
    message: "名前を変更",
    description: "Open rename env-group dialog",
  },
  "envGroups.manageButton": {
    message: "管理",
    description: "Open Environment Group management menu",
  },
  "envGroups.moveButton": {
    message: "グループを移動",
    description: "Open Environment Group move dialog",
  },
  "envGroups.moveTitle": {
    message: "環境グループを移動",
    description: "Environment Group move dialog title",
  },
  "envGroups.moveDescription": {
    message:
      "互換性のある環境を選択するか、このグループをワークスペースのスコープに戻します。",
    description: "Environment Group move dialog description",
  },
  "envGroups.moveSubmit": {
    message: "グループを移動",
    description: "Submit Environment Group move",
  },
  "envGroups.moveSuccess": {
    message: "環境グループを移動しました",
    description: "Environment Group move success toast",
  },
  "envGroups.moveError": {
    message: "グループを移動できませんでした",
    description: "Environment Group move error toast",
  },
  "envGroups.cloneButton": {
    message: "グループを複製",
    description: "Open Environment Group clone dialog",
  },
  "envGroups.cloneTitle": {
    message: "環境グループを複製",
    description: "Environment Group clone dialog title",
  },
  "envGroups.cloneDescription": {
    message:
      "変数とファイルを新しいグループにコピーします。サービスのリンクはコピーされません。",
    description: "Environment Group clone dialog description",
  },
  "envGroups.cloneSubmit": {
    message: "グループを複製",
    description: "Submit Environment Group clone",
  },
  "envGroups.cloneSuccess": {
    message: "{name} を複製しました",
    description: "Environment Group clone success toast",
  },
  "envGroups.cloneError": {
    message: "グループを複製できませんでした",
    description: "Environment Group clone error toast",
  },
  "envGroups.environmentPlaceholder": {
    message: "環境を選択",
    description: "Environment scope selector placeholder",
  },
  "envGroups.renameTitle": {
    message: "環境グループの名前を変更",
    description: "Rename env-group dialog title",
  },
  "envGroups.renameDescription": {
    message:
      "名前を変更しても、グループの変数、ファイル、サービスのリンクは保持されます。",
    description: "Rename env-group dialog description",
  },
  "envGroups.renameSubmit": {
    message: "名前を保存",
    description: "Rename env-group submit button",
  },
  "envGroups.deleteButton": {
    message: "削除",
    description: "Open delete env-group dialog",
  },
  "envGroups.deleteTitle": {
    message: "{name} を削除しますか？",
    description: "Delete env-group dialog title",
  },
  "envGroups.deleteDescription": {
    message:
      "この操作は元に戻せません。この環境グループを削除してもよろしいですか？すべてのサービスからのリンクが解除されます。",
    description: "Delete env-group dialog warning (Render-style copy)",
  },
  "envGroups.deletePrompt": {
    message: "確認のため、下に {phrase} と入力してください。",
    description:
      "Body prompt naming the exact 'sudo delete env group <name>' phrase (rendered bold by SudoCommandField)",
  },
  "envGroups.deleteConfirm": {
    message: "環境グループを削除",
    description: "Delete env-group confirm button",
  },
  "envGroups.varsTitle": {
    message: "環境変数",
    description: "Env-group variable editor title",
  },
  "envGroups.varsDescription": {
    message: "値は暗号化され、リンクされたすべてのサービスに適用されます。",
    description: "Env-group variable editor description",
  },
  "envGroups.varsEmptyTitle": {
    message: "環境変数はありません",
    description: "Env-group variable editor empty title",
  },
  "envGroups.varsEmptyBody": {
    message:
      "今すぐ変数を追加できます。先にグループをリンクする必要はありません。",
    description: "Env-group variable editor empty body",
  },
  "envGroups.varDeleteConfirmBody": {
    message:
      "リンクされたサービスは、この変数なしで再デプロイされます。ただし、自動デプロイがオフのリポジトリ連携サービスは現在のリリースを提供し続け、次回のデプロイで変更が反映されます。",
    description: "Delete env-group variable warning",
  },
  "envGroups.filesTitle": {
    message: "シークレットファイル",
    description: "Env-group secret-file editor title",
  },
  "envGroups.filesDescription": {
    message:
      "暗号化されたファイルは、リンクされたすべてのサービスの /etc/secrets にマウントされます。",
    description: "Env-group secret-file editor description",
  },
  "envGroups.filesEmptyTitle": {
    message: "シークレットファイルはありません",
    description: "Env-group secret-file editor empty title",
  },
  "envGroups.filesEmptyBody": {
    message:
      "今すぐファイルを追加できます。先にグループをリンクする必要はありません。",
    description: "Env-group secret-file editor empty body",
  },
  "envGroups.fileDeleteConfirmBody": {
    message:
      "リンクされたサービスは、このファイルなしで再デプロイされます。ただし、自動デプロイがオフのリポジトリ連携サービスは現在のリリースを提供し続け、次回のデプロイで変更が反映されます。",
    description: "Delete env-group secret-file warning",
  },
  "envGroups.servicesTitle": {
    message: "リンクされたサービス",
    description: "Env-group linked-services card title",
  },
  "envGroups.servicesDescription": {
    message:
      "リンクすると、自動デプロイの設定にかかわらず、サービスはグループの値ですぐにデプロイされます。リンクを解除するとサービスの設定が変更されます。自動デプロイがオフのリポジトリ連携サービスには次回のデプロイで反映され、それ以外のサービスはすぐにデプロイされます。",
    description: "Env-group linked-services card description",
  },
  "envGroups.selectService": {
    message: "サービスを選択",
    description: "Link-service selector placeholder",
  },
  "envGroups.linkButton": {
    message: "サービスをリンク",
    description: "Link selected service to env group",
  },
  "envGroups.unlinkButton": {
    message: "リンク解除",
    description: "Unlink service from env group",
  },
  "envGroups.unlinkServiceAria": {
    message: "{name} のリンクを解除",
    description:
      "Accessible name of a linked-service row's Unlink control, naming the service (w4/150)",
  },
  "envGroups.noLinkedServices": {
    message:
      "このグループは、まだどのサービスにもリンクされていません。それでも自由に編集できます。",
    description: "No linked services state",
  },
  "envGroups.servicesLoadError": {
    message:
      "ワークスペースのサービスを読み込めませんでした。既存のリンクは下に引き続き表示されます。",
    description: "Linked-services inventory error",
  },
  "envGroups.incompatibleLink": {
    message: "このグループの環境の外にリンクされています",
    description: "Warning on an existing incompatible Environment Group link",
  },
  "envGroups.searchPlaceholder": {
    message: "環境グループを検索",
    description: "Environment Group list search placeholder",
  },
  "envGroups.searchLabel": {
    message: "名前で環境グループを検索",
    description: "Accessible Environment Group search label",
  },
  "envGroups.resetSearch": {
    message: "検索をリセット",
    description: "Clear Environment Group search",
  },
  "envGroups.noMatchesTitle": {
    message: "一致する環境グループはありません",
    description: "Environment Group search empty-state title",
  },
  "envGroups.noMatchesBody": {
    message: "別の名前を試すか、検索をリセットしてください。",
    description: "Environment Group search empty-state body",
  },
  "envGroups.colName": {
    message: "環境グループ名",
    description: "Environment Group table name column",
  },
  "envGroups.colEnvironment": {
    message: "環境",
    description: "Environment Group table Environment column",
  },
  "envGroups.colEnvVars": {
    message: "環境変数",
    description: "Environment Group table variable-count column",
  },
  "envGroups.colSecretFiles": {
    message: "シークレットファイル",
    description: "Environment Group table secret-file-count column",
  },
  "envGroups.colUpdated": {
    message: "更新日時",
    description: "Environment Group table updated-time column",
  },
  "envGroups.rolloutNote": {
    message:
      "リンクされたサービスは変更を適用するために再デプロイされています。ただし、自動デプロイがオフのリポジトリ連携サービスには次回のデプロイで反映されます。",
    description: "Env-group write rollout toast detail",
  },
  "envGroups.linkChangeNote": {
    message:
      "自動デプロイの設定にかかわらず、サービスはグループの値ですぐにデプロイされます。",
    description: "Env-group link success toast detail",
  },
  "envGroups.unlinkChangeNote": {
    message:
      "サービスの設定が変更されました。自動デプロイがオフのリポジトリ連携サービスには次回のデプロイで反映され、それ以外のサービスはすぐにデプロイされます。",
    description: "Env-group unlink success toast detail",
  },
  "envGroups.createSuccess": {
    message: "{name} を作成しました",
    description: "Env-group create success toast",
  },
  "envGroups.createError": {
    message: "{name} を作成できませんでした",
    description: "Env-group create error toast",
  },
  "envGroups.renameSuccess": {
    message: "名前を {name} に変更しました",
    description: "Env-group rename success toast",
  },
  "envGroups.renameError": {
    message: "グループの名前を変更できませんでした",
    description: "Env-group rename error toast",
  },
  "envGroups.deleteSuccess": {
    message: "環境グループを削除しました",
    description: "Env-group delete success toast",
  },
  "envGroups.deleteError": {
    message: "グループを削除できませんでした",
    description: "Env-group delete error toast",
  },
  "envGroups.linkSuccess": {
    message: "サービスをリンクしました",
    description: "Env-group link success toast",
  },
  "envGroups.linkError": {
    message: "サービスをリンクできませんでした",
    description: "Env-group link error toast",
  },
  "envGroups.unlinkSuccess": {
    message: "サービスのリンクを解除しました",
    description: "Env-group unlink success toast",
  },
  "envGroups.unlinkError": {
    message: "サービスのリンクを解除できませんでした",
    description: "Env-group unlink error toast",
  },
  "envGroups.varSaveSuccess": {
    message: "{key} を保存しました",
    description: "Env-group variable save success toast",
  },
  "envGroups.varsSaveSuccess": {
    message: "環境変数を保存しました",
    description: "Env-group replace-all variables success toast",
  },
  "envGroups.varsSaveError": {
    message: "環境変数を保存できませんでした",
    description: "Env-group replace-all variables error toast",
  },
  "envGroups.varSaveError": {
    message: "{key} を保存できませんでした",
    description: "Env-group variable save error toast",
  },
  "envGroups.varDeleteSuccess": {
    message: "{key} を削除しました",
    description: "Env-group variable delete success toast",
  },
  "envGroups.varDeleteError": {
    message: "{key} を削除できませんでした",
    description: "Env-group variable delete error toast",
  },
};

export default jaEnvGroups;
