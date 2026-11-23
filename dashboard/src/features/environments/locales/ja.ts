import type { TranslationEntry } from "@/i18n";

const jaEnvironments: Record<string, TranslationEntry> = {
  "environments.heading": {
    message: "環境",
    description: "Section heading above a project page's environments list",
  },
  "environments.selectorLabel": {
    message: "選択中の環境",
    description:
      "Accessible label for the Project overview Environment selector",
  },
  "environments.unassignedOption": {
    message: "未割り当て",
    description: "Project resources that belong to no Environment",
  },
  "environments.newButton": {
    message: "新規環境",
    description: "Button that opens the new-environment dialog",
  },
  "environments.emptyBody": {
    message:
      "環境はまだありません。このプロジェクトのリソースをグループ化するには、staging や production などの環境を作成してください。",
    description: "Empty state shown when a project has no environments",
  },
  "environments.errorBody": {
    message:
      "環境の読み込み中に問題が発生しました。しばらくしてからもう一度お試しください。",
    description: "Error state shown when the environments query fails",
  },
  "environments.refreshErrorBody": {
    message:
      "環境を更新できませんでした。最後に読み込んだデータを表示しています。",
    description:
      "Inline notice when a background refresh of environments fails while the previously loaded environments stay on screen",
  },
  "environments.resourceCount_other": {
    message: "{count} 件のリソース",
    description:
      "Count of visible services+databases+key-value instances assigned to an environment, next to its name (plural)",
  },
  "environments.manageButton": {
    message: "リソースを管理",
    description:
      "Button on an environment card that opens the manage-resources dialog",
  },
  "environments.moreActions": {
    message: "その他の操作",
    description:
      "Accessible label for an environment card's overflow (•••) menu",
  },
  "environments.renameAction": {
    message: "名前を変更",
    description: "Environment overflow-menu item that opens the rename dialog",
  },
  "environments.deleteAction": {
    message: "削除",
    description:
      "Environment overflow-menu item that opens the delete confirmation, and the confirm button",
  },
  "environments.cardEmpty": {
    message: "この環境には、まだリソースがありません。",
    description:
      "Shown inside an environment card when it has no assigned resources",
  },
  "environments.createTitle": {
    message: "新規環境",
    description: "New-environment dialog title",
  },
  "environments.createDescription": {
    message:
      "このプロジェクトのリソースの一部に、staging や production などの名前を付けてグループ化します。",
    description: "New-environment dialog description",
  },
  "environments.fieldName": {
    message: "名前",
    description: "New-environment dialog name field label",
  },
  "environments.fieldNamePlaceholder": {
    message: "staging",
    description: "New-environment dialog name field placeholder",
  },
  "environments.cancel": {
    message: "キャンセル",
    description: "Cancel button shared by the environment dialogs",
  },
  "environments.createSubmit": {
    message: "環境を作成",
    description: "New-environment dialog submit button",
  },
  "environments.createSuccess": {
    message: "環境「{name}」を作成しました。",
    description: "Toast shown after an environment is created",
  },
  "environments.createRefreshFailed": {
    message:
      "環境「{name}」を作成しましたが、一覧を更新できませんでした。もう一度お試しいただくか、再読み込みして確認してください。",
    description:
      "Warning after the environment was created but re-reading the list failed; creation must not be retried or reported as failed (w4/231)",
  },
  "environments.createError": {
    message: "環境「{name}」を作成できませんでした。",
    description: "Toast shown when creating an environment fails",
  },
  "environments.renameFieldLabel": {
    message: "環境名",
    description:
      'Accessible/visible label for the environment rename field. Deliberately more specific than environments.fieldName\'s bare "Name" (w4/134)',
  },
  "environments.renameTitle": {
    message: "環境の名前を変更",
    description: "Rename-environment dialog title",
  },
  "environments.renameSubmit": {
    message: "保存",
    description: "Rename-environment dialog submit button",
  },
  "environments.renameSuccess": {
    message: "環境の名前を「{name}」に変更しました。",
    description: "Toast shown after an environment is renamed",
  },
  "environments.renameError": {
    message: "環境の名前を「{name}」に変更できませんでした。",
    description: "Toast shown when renaming an environment fails",
  },
  "environments.deleteConfirmTitle": {
    message: "環境「{name}」を削除しますか？",
    description: "Delete-environment confirmation dialog title",
  },
  "environments.deleteConfirmBody": {
    message:
      "この環境の環境グループは削除されず、未割り当てになります。この環境やプロジェクトには属さなくなり、環境グループに残ります。この操作は元に戻せません。",
    description: "Delete-environment confirmation dialog body",
  },
  "environments.deleteConfirmBodyResources": {
    message:
      "この環境を削除すると、そのすべてのサービス、データベース、Key Value ストアが削除されます: {resources}。この環境の環境グループは削除されず、未割り当てになります。この環境やプロジェクトには属さなくなり、環境グループに残ります。この操作は元に戻せません。",
    description:
      "Delete-environment confirmation body when services, databases or key value stores would be deleted (w4/m264, w6/m290, Render parity)",
  },
  "environments.deleteSuccess": {
    message: "環境「{name}」を削除しました。",
    description: "Toast shown after an environment is deleted",
  },
  "environments.deleteRefreshFailed": {
    message:
      "環境「{name}」を削除しましたが、一覧を更新できませんでした。もう一度お試しいただくか、再読み込みして更新してください。",
    description:
      "Warning after the environment was deleted but re-reading the list failed; deletion must not be retried or reported as failed (w4/290)",
  },
  "environments.deleteError": {
    message: "環境「{name}」を削除できませんでした。",
    description: "Toast shown when deleting an environment fails",
  },
  "environments.manageTitle": {
    message: "「{name}」のリソースを管理",
    description: "Manage-resources dialog title",
  },
  "environments.manageDescription": {
    message:
      "この環境に属するリソースをチェックしてください。リソースを割り当てると、このプロジェクトにも追加されます。",
    description: "Manage-resources dialog description",
  },
  "environments.tabServices": {
    message: "サービス",
    description: "Manage-resources dialog tab label for the services checklist",
  },
  "environments.tabDatabases": {
    message: "データベース",
    description:
      "Manage-resources dialog tab label for the databases checklist",
  },
  "environments.tabKeyValues": {
    message: "Key Value",
    description:
      "Manage-resources dialog tab label for the key-value checklist",
  },
  "environments.manageNoServices": {
    message: "このワークスペースには、割り当てられるサービスがまだありません。",
    description:
      "Manage-resources dialog empty state when the workspace has no services",
  },
  "environments.manageNoDatabases": {
    message:
      "このワークスペースには、割り当てられるデータベースがまだありません。",
    description:
      "Manage-resources dialog empty state when the workspace has no databases",
  },
  "environments.manageNoKeyValues": {
    message:
      "このワークスペースには、割り当てられる Key Value インスタンスがまだありません。",
    description:
      "Manage-resources dialog empty state when the workspace has no key-value instances",
  },
  "environments.manageResourceIn": {
    message: "{environment} 内",
    description:
      "Under a Manage resources row: the other environment the resource is in now ({environment} = its name)",
  },
  "environments.markerProtected": {
    message: "保護",
    description:
      "Marker: the environment the resource would leave is protected",
  },
  "environments.markerIsolated": {
    message: "分離",
    description:
      "Marker: the environment the resource would leave has network isolation",
  },
  "environments.markerAllowList": {
    message: "IP 許可リスト",
    description:
      "Marker: the environment the resource would leave restricts inbound IPs",
  },
  "environments.manageMovesOut": {
    message: "{environment} から移動: {resources}",
    description:
      "Before Save: the selected resources this save moves out of another environment ({resources} = comma-separated names)",
  },
  "environments.manageNeedsAdminToMove": {
    message: "{environment} から移動できるのは管理者のみです。",
    description:
      "Why a Manage resources row is disabled: its current environment is admin-managed",
  },
  "environments.manageUnsaved": {
    message: "未保存の変更",
    description:
      "Accessible description of a Manage resources tab whose selection differs from the saved membership",
  },
  "environments.manageSubmit": {
    message: "保存",
    description: "Manage-resources dialog submit button",
  },
  "environments.assignSuccess": {
    message: "「{name}」のサービスを更新しました。",
    description: "Toast shown after an environment's services are updated",
  },
  "environments.strandedMoveTitle": {
    message: "この移動により、リンクされたグループが取り残されます",
    description:
      "Confirmation title shown before a move that strands an environment-scoped group link (w4/m238)",
  },
  "environments.strandedMoveBody": {
    message:
      "これらの環境グループは、移動するサービスにリンクされたままとなり、グループのリンクを解除するか移動するまで、変数とファイルが引き続き適用されます:",
    description:
      "Confirmation body: the stranded groups keep feeding the service after the move",
  },
  "environments.strandedMoveConfirm": {
    message: "このまま移動",
    description: "Confirm button on the stranded-groups move confirmation",
  },
  "environments.strandedScopeWorkspace": {
    message: "ワークスペース",
    description:
      "Scope label for a stranded group that is scoped to the whole workspace",
  },
  "environments.strandedScopeUnknown": {
    message:
      "リンクされた環境グループをまだ確認できませんでした。一覧の読み込みを待ってから、もう一度お試しください。",
    description:
      "Toast shown when a move is attempted before the group/scope reads resolve",
  },
  "environments.assignError": {
    message: "「{name}」のサービスを更新できませんでした。",
    description: "Toast shown when updating an environment's services fails",
  },
  "environments.assignDatabasesSuccess": {
    message: "「{name}」のデータベースを更新しました。",
    description: "Toast shown after an environment's databases are updated",
  },
  "environments.assignDatabasesError": {
    message: "「{name}」のデータベースを更新できませんでした。",
    description: "Toast shown when updating an environment's databases fails",
  },
  "environments.assignDatabasesRefreshFailed": {
    message:
      "「{name}」のデータベースを更新しましたが、一覧を更新できませんでした。もう一度お試しいただくか、再読み込みして確認してください。",
    description:
      "Warning after the database membership was saved but re-reading the environments list failed; the write must not be retried or reported as failed (w4/325)",
  },
  "environments.assignKeyValuesSuccess": {
    message: "「{name}」の Key Value インスタンスを更新しました。",
    description:
      "Toast shown after an environment's key-value instances are updated",
  },
  "environments.assignKeyValuesRefreshFailed": {
    message:
      "「{name}」の Key Value インスタンスを更新しましたが、一覧を更新できませんでした。もう一度お試しいただくか、再読み込みして確認してください。",
    description:
      "Warning after the key-value membership was saved but re-reading the environments list failed; the write must not be retried or reported as failed (w4/263)",
  },
  "environments.assignKeyValuesError": {
    message: "「{name}」の Key Value インスタンスを更新できませんでした。",
    description:
      "Toast shown when updating an environment's key-value instances fails",
  },
  "environments.tabEnvGroups": {
    message: "環境グループ",
    description: "Manage-resources dialog tab label for environment groups",
  },
  "environments.manageNoEnvGroups": {
    message:
      "このワークスペースには、割り当てられる環境グループがまだありません。",
    description: "Environment-groups checklist empty state",
  },
  "environments.assignEnvGroupsSuccess": {
    message: "「{name}」の環境グループを更新しました。",
    description: "Toast shown after environment-group membership is updated",
  },
  "environments.assignEnvGroupsPlacementUnread": {
    message:
      "「{name}」のグループが現在どこにあるかを確認できませんでした。もう一度移動する前に、再試行するか再読み込みしてください。",
    description:
      "Warning after an environment-group move failed and re-reading the groups' placement failed too; its action re-reads, it never moves again (w4/m206)",
  },
  "environments.assignEnvGroupsError": {
    message: "「{name}」の環境グループを更新できませんでした。",
    description:
      "Toast shown when environment-group membership fails to update",
  },
  "environments.settingsAction": {
    message: "すべての設定",
    description: "Environment overflow-menu action opening ACL settings",
  },
  "environments.settingsTitle": {
    message: "「{name}」の設定",
    description: "Environment ACL settings dialog title",
  },
  "environments.settingsDescription": {
    message:
      "権限、プライベートネットワークの分離、受信 IP ルールを管理します。",
    description: "Environment ACL settings dialog description",
  },
  "environments.protectedBadge": {
    message: "保護",
    description: "Badge shown beside a protected environment name",
  },
  "environments.protectedLabel": {
    message: "保護された環境",
    description: "Protected-status setting label",
  },
  "environments.protectedHint": {
    message:
      "この環境のリソースを削除、一時停止、再開したり、メンテナンスモードを切り替えたりできるのは、ワークスペースの管理者のみです。",
    description: "Protected-status setting explanation",
  },
  "environments.isolationLabel": {
    message: "環境間の接続をブロック",
    description: "Private-network isolation setting label",
  },
  "environments.isolationProOnly": {
    message:
      "ネットワークの分離には Pro プラン以上が必要です。以前に有効にしていた場合は、ここでオフにできます。",
    description: "Isolation setting explanation on a Hobby workspace (w8/m138)",
  },
  "environments.isolationHint": {
    message:
      "この環境の Postgres や Key Value インスタンスを含め、この環境の境界を越えるプライベートネットワークのトラフィックを防ぎます。外部 URL は引き続き、それぞれの IP ルールに従います。",
    description: "Private-network isolation setting explanation",
  },
  "environments.ipAllowListLabel": {
    message: "受信 IP の制限",
    description: "Environment inbound-IP allowlist heading",
  },
  "environments.ipAllowListHint": {
    message:
      "一覧にある CIDR 範囲のみが、この環境のパブリックなサービス、Static Site、データストアにアクセスできます。送信元は、各リソース自身の IP ルールにも合格する必要があります。",
    description: "Environment inbound-IP allowlist explanation",
  },
  "environments.ipAllowListOpen": {
    message:
      "ルールがありません。この環境のパブリックなメンバーは、すべてのトラフィックを拒否します。すべてを許可するには、0.0.0.0/0 と ::/0 を追加してください。",
    description: "Empty environment inbound-IP allowlist state (deny-all)",
  },
  "environments.ipAllowListRemove": {
    message: "{cidr} を削除",
    description: "Accessible label for an inbound-IP rule remove button",
  },
  "environments.ipAllowListRuleCIDR": {
    message: "ルール {number} の CIDR ブロック",
    description: "Accessible label for an existing inbound-IP rule CIDR",
  },
  "environments.ipAllowListRuleDescription": {
    message: "ルール {number} の説明",
    description: "Accessible label for an existing inbound-IP rule description",
  },
  "environments.ipAllowListNewCIDR": {
    message: "新しい CIDR ブロック",
    description: "Accessible label for a new inbound-IP rule CIDR",
  },
  "environments.ipAllowListNewDescription": {
    message: "新しいルールの説明",
    description: "Accessible label for a new inbound-IP rule description",
  },
  "environments.ipAllowListDescriptionPlaceholder": {
    message: "説明（任意）",
    description: "Placeholder for an inbound-IP rule description",
  },
  "environments.ipAllowListScalePlanOnly": {
    message:
      "環境の受信 IP ルールには Scale プラン以上が必要です。すでに設定されているルールは引き続き適用され、リセットできます。",
    description:
      "Hint replacing the editable IP rules below the Scale plan (w8/m137)",
  },
  "environments.ipAllowListResetAllowAll": {
    message: "すべて許可にリセット",
    description:
      "Button resetting an environment's inbound IP rules to allow-all below the Scale plan (w8/m137)",
  },
  "environments.ipAllowListInvalid": {
    message:
      "重複しない有効な IPv4 または IPv6 の CIDR ブロックを入力してください。",
    description:
      "Inline validation when an environment allow list block is not a unique, valid CIDR",
  },
  "environments.ipAllowListAdd": {
    message: "ソースを追加",
    description: "Button adding an inbound-IP CIDR rule",
  },
  "environments.settingsSave": {
    message: "保存",
    description: "Environment ACL settings save button",
  },
  "environments.aclSaveSuccess": {
    message: "「{name}」の設定を保存しました。",
    description: "Toast shown after an Environment ACL is saved",
  },
  "environments.aclSaveError": {
    message: "「{name}」の設定を保存できませんでした。",
    description: "Toast shown when an Environment ACL save fails",
  },
  "environments.resourceCountLoading": {
    message: "集計中…",
    description:
      "Environment card header while its resource lists are still loading, instead of a count",
  },
  "environments.resourceCountUnknown": {
    message: "件数を取得できません",
    description:
      "Environment card header when a resource list has not answered, instead of a count",
  },
  "environments.resourcesUnavailable": {
    message:
      "一部のリソースを読み込めなかったため、この環境の内容が不完全または古い可能性があります。",
    description:
      "Inline alert on a Project environment card when a resource list failed to load; offers Try again",
  },
  "environments.resourcesLoading": {
    message: "リソースを読み込み中…",
    description:
      "Environment card line while its resource lists are still loading, instead of the empty message",
  },
  "environments.assignmentTitle": {
    message: "プロジェクト",
    description: "Shared create-form Project/Environment selector heading",
  },
  "environments.assignmentProject": {
    message: "プロジェクト",
    description: "Accessible label for a create-form Project selector",
  },
  "environments.assignmentProjectNone": {
    message: "プロジェクトなし",
    description: "Unassigned Project selector option",
  },
  "environments.assignmentEnvironment": {
    message: "環境",
    description: "Accessible label for a create-form Environment selector",
  },
  "environments.assignmentEnvironmentNone": {
    message: "環境なし",
    description: "Unassigned Environment selector option",
  },
  "environments.assignmentSelectedLoading": {
    message: "読み込み中…",
    description:
      "Label for a kept Project/Environment selection while its list is still loading",
  },
  "environments.assignmentSelectedUnavailable": {
    message: "選択済み（名前を取得できません）",
    description:
      "Label for a kept Project/Environment selection whose list failed to load",
  },
  "environments.assignmentReadError": {
    message:
      "プロジェクトまたは環境を読み込めませんでした。もう一度お試しください。選択内容は保持されます。",
    description:
      "Inline alert when the create form's Project or Environment list fails to load; the selection is kept",
  },
  "environments.assignmentHint": {
    message:
      "リソースの作成時にプロジェクトを選択すると、環境が絞り込まれます。リソースをそのプロジェクトに追加するには、環境を選択してください。「環境なし」のままにすると、リソースはどのプロジェクトにも属さずに作成されます。プロジェクトにまだ環境がない場合は、先にプロジェクトのページで作成してください。",
    description: "Shared create-form Environment-assignment hint",
  },
  "environments.protectedPlacementAdminOnly": {
    message:
      "保護された環境にリソースを作成したり、その環境へ、またはその環境から移動したりできるのは、管理者のみです。",
    description:
      "Why a create or move control that targets (or leaves) a protected environment is disabled for a non-admin, and the message for the same server refusal (w3/m281)",
  },
  "environments.protectedOptionAdminOnly": {
    message: "{environment}（管理者のみ）",
    description:
      "A protected environment in a create or move picker that a non-admin cannot choose ({environment} = its name)",
  },
  "environments.assignmentDescription": {
    message: "作成後にプロジェクトと環境へ追加します。",
    description: "Shared create-form Project row description",
  },
  "environments.assignmentNoEnvironments": {
    message:
      "このプロジェクトにはまだ環境がありません。先にプロジェクトのページで作成してください。",
    description:
      "Shared create-form note when the chosen Project has no Environment to add the resource to",
  },
  "environments.assignmentPickEnvironment": {
    message:
      "このプロジェクトに追加するには環境を選択してください。「環境なし」のままだと、どのプロジェクトにも属さずに作成されます。",
    description:
      "Shared create-form note when a Project is chosen but No environment is: only an Environment adds the resource to the Project",
  },
  "environments.assignmentEnvironmentPickProject": {
    message: "先にプロジェクトを選択",
    description:
      "Shared create-form Environment select while no Project is chosen (it is disabled until one is)",
  },
};

export default jaEnvironments;
