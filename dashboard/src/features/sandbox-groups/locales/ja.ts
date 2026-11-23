import type { TranslationEntry } from "@/i18n";

const jaSandboxGroups: Record<string, TranslationEntry> = {
  "sandboxGroups.createTitle": {
    message: "新しいサンドボックスグループ",
    description:
      "Title of the new-sandbox-group dialog and its entry in the + New menus",
  },
  "sandboxGroups.createDescription": {
    message:
      "サンドボックスグループは、グループ内で同時にアクティブにできるサンドボックスの数に上限を設けます。グループ名はワークスペース内で一意である必要があります。",
    description: "New-sandbox-group dialog description",
  },
  "sandboxGroups.fieldName": {
    message: "名前",
    description: "New-sandbox-group dialog name field label",
  },
  "sandboxGroups.fieldNamePlaceholder": {
    message: "my-sandbox-group",
    description: "Placeholder for the new sandbox group's name",
  },
  "sandboxGroups.fieldRegion": {
    message: "リージョン",
    description:
      "Label of the region row in the created sandbox group's confirmation",
  },
  "sandboxGroups.createCancel": {
    message: "キャンセル",
    description: "Cancel button of the new-sandbox-group dialog",
  },
  "sandboxGroups.createSubmit": {
    message: "サンドボックスグループを作成",
    description: "Submit button of the new-sandbox-group dialog",
  },
  "sandboxGroups.createError": {
    message:
      "サンドボックスグループ {name} を作成できませんでした。もう一度お試しください。",
    description: "Toast shown when creating a sandbox group fails",
  },
  "sandboxGroups.errorNameTaken": {
    message:
      "「{name}」という名前のサンドボックスグループはこのワークスペースにすでに存在します。別の名前を指定してください。",
    description:
      "Inline error under the name field when the group name is already taken",
  },
  "sandboxGroups.errorNameInvalid": {
    message:
      "グループ名は 1 〜 {max} 文字で、先頭と末尾に空白を含めず、制御文字も使用できません。",
    description: "Inline error under the name field when the name is refused",
  },
  "sandboxGroups.errorLimitReached_other": {
    message:
      "このワークスペースはサンドボックスグループの上限である {count} 個にすでに達しています。",
    description:
      "Inline alert when the workspace is at its sandbox group limit",
  },
  "sandboxGroups.errorLimitReachedGeneric": {
    message: "このワークスペースはサンドボックスグループの上限に達しています。",
    description:
      "Inline alert for the sandbox group limit when the server names no number",
  },
  "sandboxGroups.errorLimitTitle": {
    message: "サンドボックスグループの上限に達しました",
    description: "Heading of the inline sandbox-group-limit alert",
  },
  "sandboxGroups.createdTitle": {
    message: "サンドボックスグループを作成しました",
    description: "Dialog title once a sandbox group has been created",
  },
  "sandboxGroups.createdDescription": {
    message:
      "{name} の準備ができました。このグループにサンドボックスを作成するときは、その ID を sandboxGroupId として渡してください。",
    description: "Dialog description once a sandbox group has been created",
  },
  "sandboxGroups.createdDone": {
    message: "完了",
    description: "Button that closes the created-sandbox-group confirmation",
  },
  "sandboxGroups.fieldId": {
    message: "ID",
    description: "Label of the new sandbox group's ID in the confirmation",
  },
  "sandboxGroups.fieldDefault": {
    message: "デフォルトグループ",
    description:
      "Label of the confirmation row that says whether the group is the workspace's default",
  },
  "sandboxGroups.defaultYes": {
    message: "はい",
    description: "The created group is the workspace's default sandbox group",
  },
  "sandboxGroups.defaultNo": {
    message: "いいえ",
    description:
      "The created group is not the workspace's default sandbox group",
  },
  "sandboxGroups.fieldConcurrencyLimit": {
    message: "同時実行数の上限",
    description:
      "Label of the confirmation row with the most active sandboxes the group allows",
  },
  "sandboxGroups.copyId": {
    message: "サンドボックスグループ ID をコピー",
    description: "Accessible label of the copy button next to the group's ID",
  },
  "sandboxGroups.copied": {
    message: "サンドボックスグループ ID をコピーしました",
    description: "Toast after copying the sandbox group ID",
  },
  "sandboxGroups.copyError": {
    message: "サンドボックスグループ ID をコピーできませんでした",
    description: "Toast when copying the sandbox group ID fails",
  },
  "sandboxGroups.environmentLoading": {
    message: "環境を読み込み中",
    description:
      "Accessible name of the loading placeholder shown where the new-sandbox-group dialog's Environment select will appear",
  },
  "sandboxGroups.environmentsError": {
    message:
      "環境を読み込めませんでした。環境を指定せずにグループを作成することはできます。",
    description:
      "Shown in the new-sandbox-group dialog when the workspace's environments could not be read; the group can still be created without one",
  },
  "sandboxGroups.errorEnvironmentGone": {
    message:
      "その環境はもう存在しません。別の環境を選ぶか、環境なしを選んでください。",
    description:
      "Inline error under the Environment select when the chosen environment was deleted before the group was created",
  },
  "sandboxGroups.pageTitle": {
    message: "サンドボックスグループ",
    description: "Heading for the workspace sandbox groups page",
  },
  "sandboxGroups.pageDescription": {
    message: "各グループのサンドボックス同時実行上限とアウトバウンド認証情報を管理します。",
    description: "Subtitle for the workspace sandbox groups page",
  },
  "sandboxGroups.resourceType": {
    message: "サンドボックスグループ",
    description: "Singular resource type name for a sandbox group",
  },
  "sandboxGroups.backToList": {
    message: "サンドボックスグループ一覧に戻る",
    description: "Accessible label of the back link on a sandbox group detail page",
  },
  "sandboxGroups.defaultBadge": {
    message: "デフォルト",
    description: "Badge indicating the workspace default sandbox group",
  },
  "sandboxGroups.statusReady": {
    message: "準備完了",
    description:
      "Status of a sandbox group row in a project environment's resource table; Render shows every group as Ready",
  },
  "sandboxGroups.environmentUpdated": {
    message: "「{name}」のサンドボックスグループを更新しました。",
    description:
      "Toast after sandbox groups were moved into or out of an environment; {name} is the environment",
  },
  "sandboxGroups.environmentUpdateError": {
    message: "サンドボックスグループ {group} を移動できませんでした。もう一度お試しください。",
    description:
      "Toast when moving a sandbox group into or out of an environment fails and the server gave no reason to show",
  },
  "sandboxGroups.listLoadError": {
    message: "サンドボックスグループを読み込めませんでした。もう一度お試しください。",
    description: "Error state when loading sandbox groups fails",
  },
  "sandboxGroups.listEmptyTitle": {
    message: "サンドボックスグループがまだありません",
    description: "Empty state heading when the workspace has no sandbox groups",
  },
  "sandboxGroups.listEmptyDescription": {
    message: "サンドボックスグループを作成して同時実行上限やアウトバウンド認証情報を設定します。",
    description: "Empty state body when the workspace has no sandbox groups",
  },
  "sandboxGroups.manageCredentialsButton": {
    message: "認証情報",
    description: "Button on each sandbox group row linking to its detail and credentials",
  },
  "sandboxGroups.detailLoadError": {
    message: "このサンドボックスグループを読み込めませんでした。もう一度お試しください。",
    description: "Error state when loading a single sandbox group fails",
  },
  "sandboxGroups.credSectionTitle": {
    message: "アウトバウンド認証情報",
    description: "Card title for the sandbox group credentials panel",
  },
  "sandboxGroups.credSectionDescription": {
    message:
      "このグループ内のサンドボックスからの一致するアウトバウンド HTTPS リクエストに自動注入されるシークレットです。保存された値は書き込み専用で再表示されません。",
    description: "Card subtitle for the sandbox group credentials panel",
  },
  "sandboxGroups.credAddButton": {
    message: "認証情報を追加",
    description: "Button to open the create sandbox group credential dialog",
  },
  "sandboxGroups.credLoadError": {
    message: "サンドボックスグループの認証情報を読み込めませんでした。もう一度お試しください。",
    description: "Error state when fetching sandbox group credentials fails",
  },
  "sandboxGroups.credEmptyTitle": {
    message: "認証情報が設定されていません",
    description: "Empty state heading when a sandbox group has no credentials",
  },
  "sandboxGroups.credEmptyDescriptionAdmin": {
    message:
      "認証情報を追加すると、サンドボックスコードにシークレットを渡さずにアウトバウンドリクエストへ認証を注入できます。",
    description: "Empty state body for admins when no credentials exist",
  },
  "sandboxGroups.credEmptyDescriptionReadOnly": {
    message: "ワークスペース管理者がこのグループのアウトバウンド認証情報を追加できます。",
    description: "Empty state body for non-admins when no credentials exist",
  },
  "sandboxGroups.credColId": {
    message: "認証情報 ID",
    description: "Table column header for credential ID",
  },
  "sandboxGroups.credColDomain": {
    message: "ドメイン",
    description: "Table column header for destination domain",
  },
  "sandboxGroups.credColInjection": {
    message: "注入方式",
    description: "Table column header for injection method and target",
  },
  "sandboxGroups.credColRouting": {
    message: "ルーティング",
    description: "Table column header for HTTP methods and path filters",
  },
  "sandboxGroups.credColEnabled": {
    message: "有効",
    description: "Table column header for enabled toggle or status",
  },
  "sandboxGroups.credColActions": {
    message: "操作",
    description: "Table column header for row actions",
  },
  "sandboxGroups.credAllMethods": {
    message: "すべてのメソッド",
    description: "Badge shown when a credential matches all HTTP methods",
  },
  "sandboxGroups.credAllPaths": {
    message: "すべてのパス",
    description: "Badge shown when a credential matches all request paths",
  },
  "sandboxGroups.credStatusEnabled": {
    message: "有効",
    description: "Read-only badge when a credential is enabled",
  },
  "sandboxGroups.credStatusDisabled": {
    message: "無効",
    description: "Read-only badge when a credential is disabled",
  },
  "sandboxGroups.credToggleEnabledLabel": {
    message: "{id} の有効状態を切り替える",
    description: "Accessible label for the enabled switch on a credential row",
  },
  "sandboxGroups.credVersionsActionLabel": {
    message: "{id} のバージョン履歴を表示",
    description: "Accessible label for the version history icon button",
  },
  "sandboxGroups.credEditActionLabel": {
    message: "認証情報 {id} を編集",
    description: "Accessible label for the edit credential icon button",
  },
  "sandboxGroups.credDeleteActionLabel": {
    message: "認証情報 {id} を削除",
    description: "Accessible label for the delete credential icon button",
  },
  "sandboxGroups.credVersionsButton": {
    message: "履歴",
    description: "Mobile button label to open credential version history",
  },
  "sandboxGroups.credEditButton": {
    message: "編集",
    description: "Mobile button label to edit a credential",
  },
  "sandboxGroups.credDeleteButton": {
    message: "削除",
    description: "Button label to delete a credential",
  },
  "sandboxGroups.credCreateTitle": {
    message: "認証情報を追加",
    description: "Dialog title when creating a sandbox group credential",
  },
  "sandboxGroups.credCreateDescription": {
    message: "このサンドボックスグループに書き込み専用のアウトバウンド認証情報を設定します。",
    description: "Dialog description when creating a sandbox group credential",
  },
  "sandboxGroups.credEditTitle": {
    message: "{id} を編集",
    description: "Dialog title when editing a sandbox group credential",
  },
  "sandboxGroups.credEditDescriptionAdmin": {
    message:
      "ルーティング、注入設定、ドメインの更新やシークレットのローテーションができます。シークレットを空欄にすると現在の値が維持されます。",
    description: "Dialog description when an admin edits a credential",
  },
  "sandboxGroups.credEditDescriptionDeveloper": {
    message:
      "注入設定、メソッド、パス、有効状態を更新できます。ドメインやシークレット値の変更にはワークスペース管理者権限が必要です。",
    description: "Dialog description when a developer edits a credential",
  },
  "sandboxGroups.credFieldId": {
    message: "認証情報 ID",
    description: "Form label for credential ID",
  },
  "sandboxGroups.credFieldIdHelp": {
    message: "大文字小文字を区別し、英数字、.、_、~、- のみ使用できます。`rnd-` では始められません。",
    description: "Help text for credential ID format",
  },
  "sandboxGroups.credFieldDomain": {
    message: "宛先ドメイン",
    description: "Form label for destination hostname",
  },
  "sandboxGroups.credAdminOnlyDomainSecret": {
    message: "ドメインやシークレット値を変更できるのはワークスペース管理者のみです。",
    description: "Helper text shown to developers on read-only domain field",
  },
  "sandboxGroups.credFieldValue": {
    message: "シークレット値",
    description: "Form label for secret value on create",
  },
  "sandboxGroups.credFieldValuePlaceholder": {
    message: "シークレット値を貼り付けてください",
    description: "Placeholder for secret value input on create",
  },
  "sandboxGroups.credFieldReplacementValue": {
    message: "新しいシークレット値（任意）",
    description: "Form label for rotating secret value on edit",
  },
  "sandboxGroups.credFieldReplacementValuePlaceholder": {
    message: "既存のシークレットを保持する場合は空欄のままにします",
    description: "Placeholder for optional secret rotation input on edit",
  },
  "sandboxGroups.credFieldValueWriteOnlyHelp": {
    message: "書き込み専用（最大 5 KiB）です。保存後は再表示されません。",
    description: "Help text explaining write-only secret handling",
  },
  "sandboxGroups.credFieldInjectionType": {
    message: "注入タイプ",
    description: "Form label for injection type select",
  },
  "sandboxGroups.credInjectionHeader": {
    message: "HTTP ヘッダー",
    description: "Injection type option for HTTP header",
  },
  "sandboxGroups.credInjectionBasic": {
    message: "Basic 認証",
    description: "Injection type option for HTTP Basic authentication",
  },
  "sandboxGroups.credInjectionCookie": {
    message: "クッキー",
    description: "Injection type option for HTTP cookie",
  },
  "sandboxGroups.credInjectionQuery": {
    message: "クエリパラメーター",
    description: "Injection type option for URL query parameter",
  },
  "sandboxGroups.credFieldInjectionName": {
    message: "ターゲット名",
    description: "Form label for header, cookie, or query parameter name",
  },
  "sandboxGroups.credBasicHelp": {
    message: "`Authorization` ヘッダーを自動で使用します。`username:password` 形式で指定してください。",
    description: "Helper text shown when basic auth injection is selected",
  },
  "sandboxGroups.credFieldTemplate": {
    message: "値テンプレート（任意）",
    description: "Form label for optional value template",
  },
  "sandboxGroups.credFieldTemplateHelp": {
    message: "既定値は {token} です。{token} を 1〜5 回含めてください。",
    description: "Help text for injection template placeholder syntax",
  },
  "sandboxGroups.credFieldMethods": {
    message: "HTTP メソッド",
    description: "Form label for allowed HTTP methods multi-select",
  },
  "sandboxGroups.credFieldMethodsHelp": {
    message: "何も選択しない場合はすべての HTTP メソッドに一致します。",
    description: "Help text for HTTP methods selection",
  },
  "sandboxGroups.credFieldPaths": {
    message: "パスパターン（1行に1つ）",
    description: "Form label for path patterns textarea",
  },
  "sandboxGroups.credFieldPathsHelp": {
    message: "/ で始まる絶対パスを指定します。セグメント全体の * と ** が使えます。空欄ですべてのパスに一致します。",
    description: "Help text for path pattern syntax",
  },
  "sandboxGroups.credFieldEnabled": {
    message: "有効",
    description: "Form label for credential enabled switch",
  },
  "sandboxGroups.credFieldEnabledHelp": {
    message: "無効な認証情報はグループに残りますが、リクエストには注入されません。",
    description: "Help text for credential enabled switch",
  },
  "sandboxGroups.credCreateSubmit": {
    message: "認証情報を作成",
    description: "Submit button label for creating a credential",
  },
  "sandboxGroups.credEditSubmit": {
    message: "変更を保存",
    description: "Submit button label for editing a credential",
  },
  "sandboxGroups.credCreatedToast": {
    message: "認証情報 {id} を作成しました。",
    description: "Success toast after creating a credential",
  },
  "sandboxGroups.credUpdatedToast": {
    message: "認証情報 {id} を更新しました。",
    description: "Success toast after updating a credential",
  },
  "sandboxGroups.credDeletedToast": {
    message: "認証情報 {id} を削除しました。",
    description: "Success toast after deleting a credential",
  },
  "sandboxGroups.credRestoredToast": {
    message: "{id} をバージョン v{version} から復元しました。",
    description: "Success toast after restoring a credential version",
  },
  "sandboxGroups.credCreateError": {
    message: "認証情報を作成できませんでした。入力内容を確認してください。",
    description: "Fallback error when creating a credential fails",
  },
  "sandboxGroups.credUpdateError": {
    message: "認証情報を更新できませんでした。もう一度お試しください。",
    description: "Fallback error when updating a credential fails",
  },
  "sandboxGroups.credDeleteError": {
    message: "認証情報を削除できませんでした。もう一度お試しください。",
    description: "Fallback error when deleting a credential fails",
  },
  "sandboxGroups.credRestoreError": {
    message: "認証情報のバージョンを復元できませんでした。もう一度お試しください。",
    description: "Fallback error when restoring a credential version fails",
  },
  "sandboxGroups.credErrorExists": {
    message: "このグループには ID「{id}」の認証情報がすでに存在します。",
    description: "Inline error when creating a duplicate credential ID",
  },
  "sandboxGroups.credErrorBusy": {
    message: "このグループで別の認証情報の更新が進行中です。しばらくしてから再試行してください。",
    description: "Error when concurrent credential writes contend on the group lock",
  },
  "sandboxGroups.credErrorUnavailable": {
    message: "認証情報ストレージは現在利用できません。",
    description: "Error when OpenBao credential storage is unconfigured or unreachable",
  },
  "sandboxGroups.credErrorVersionNotFound": {
    message: "その認証情報バージョンは保持されていません。",
    description: "Error when restoring an evicted or unknown version",
  },
  "sandboxGroups.credVersionsTitle": {
    message: "バージョン履歴 · {id}",
    description: "Dialog title for credential version history",
  },
  "sandboxGroups.credVersionsDescription": {
    message: "現在の設定に加えて、直近の異なる過去設定を最大 3 件保持します。",
    description: "Dialog subtitle explaining version retention policy",
  },
  "sandboxGroups.credVersionsError": {
    message: "バージョン履歴を読み込めませんでした。もう一度お試しください。",
    description: "Error state when loading credential versions fails",
  },
  "sandboxGroups.credVersionsEmpty": {
    message: "保持されているバージョンはありません。",
    description: "Empty state for credential versions dialog",
  },
  "sandboxGroups.credVersionCurrent": {
    message: "現在",
    description: "Badge marking the active credential version",
  },
  "sandboxGroups.credVersionRestoredFrom": {
    message: "v{version} から復元",
    description: "Badge showing which prior version was restored",
  },
  "sandboxGroups.credFingerprintLabel": {
    message: "フィンガープリント",
    description: "Label before the truncated SHA-256 fingerprint",
  },
  "sandboxGroups.credUpdatedBy": {
    message: "更新者: {actor}",
    description: "Actor attribution on a credential version row",
  },
  "sandboxGroups.credRestoreAction": {
    message: "復元",
    description: "Button label to restore a prior credential version",
  },
  "sandboxGroups.credRestoreConfirmTitle": {
    message: "{id} を v{version} に復元しますか？",
    description: "Confirmation dialog title when restoring a credential version",
  },
  "sandboxGroups.credRestoreConfirmDescription": {
    message: "v{version} の設定とシークレットを使って {id} の新しい現在バージョンを作成します。",
    description: "Confirmation dialog body when restoring a credential version",
  },
  "sandboxGroups.credDeleteConfirmTitle": {
    message: "認証情報 {id} を削除しますか？",
    description: "Confirmation dialog title when deleting a credential",
  },
  "sandboxGroups.credDeleteConfirmDescription": {
    message: "{id} を削除すると、保存されたシークレットと保持中の全バージョンが完全に消去されます。",
    description: "Confirmation dialog body when deleting a credential",
  },
  "sandboxGroups.credValIdRequired": {
    message: "認証情報 ID は必須です。",
    description: "Validation error when credential ID is empty",
  },
  "sandboxGroups.credValIdInvalid": {
    message: "英数字、ピリオド、アンダースコア、チルダ、ハイフンのみを使用してください。",
    description: "Validation error when credential ID has disallowed characters",
  },
  "sandboxGroups.credValIdReserved": {
    message: "「.」「..」および `rnd-` で始まる認証情報 ID は予約されています。",
    description: "Validation error when credential ID is reserved",
  },
  "sandboxGroups.credValDomainRequired": {
    message: "宛先ドメインは必須です。",
    description: "Validation error when domain is empty",
  },
  "sandboxGroups.credValDomainInvalid": {
    message: "ワイルドカード・ポート・パスを含まない有効な公開 DNS ホスト名を入力してください。",
    description: "Validation error when domain format is invalid",
  },
  "sandboxGroups.credValDomainPrivate": {
    message: "ローカル・プライベート・クラスター内部のホスト名は使用できません。",
    description: "Validation error when domain is private or cluster-local",
  },
  "sandboxGroups.credValValueRequired": {
    message: "シークレット値は必須です。",
    description: "Validation error when secret value is empty on create",
  },
  "sandboxGroups.credValValueTooLong": {
    message: "シークレット値は最大 {maxBytes} バイトまでです。",
    description: "Validation error when secret value exceeds 5 KiB",
  },
  "sandboxGroups.credValValueNul": {
    message: "シークレット値に NUL バイトは含められません。",
    description: "Validation error when secret value contains a NUL byte",
  },
  "sandboxGroups.credValInjectionNameRequired": {
    message: "この注入タイプにはターゲット名が必須です。",
    description: "Validation error when header/cookie/query name is missing",
  },
  "sandboxGroups.credValInjectionNameForbidden": {
    message: "Basic 認証注入ではターゲット名を指定できません。",
    description: "Validation error when basic injection specifies a name",
  },
  "sandboxGroups.credValInjectionHeaderForbidden": {
    message: "ルーティングや接続制御ヘッダーは注入できません。",
    description: "Validation error when a forbidden HTTP header is chosen",
  },
  "sandboxGroups.credValInjectionNameInvalid": {
    message: "有効なターゲット名を入力してください。",
    description: "Validation error when target name has invalid characters",
  },
  "sandboxGroups.credValInjectionTemplateCount": {
    message: "テンプレートには {token} を 1〜{max} 回含める必要があります。",
    description: "Validation error when template placeholder count is out of range",
  },
  "sandboxGroups.credValInjectionBasicFormat": {
    message: "Basic 認証の展開後の値は `username:password` 形式である必要があります。",
    description: "Validation error when basic auth rendered value lacks username:password",
  },
  "sandboxGroups.credValInjectionHeaderControl": {
    message: "展開後のヘッダー値に改行や NUL バイトは含められません。",
    description: "Validation error when header value contains CR/LF/NUL",
  },
  "sandboxGroups.credValInjectionCookieInvalid": {
    message: "展開後のクッキー値にセミコロンや改行は含められません。",
    description: "Validation error when cookie value contains semicolons or CR/LF",
  },
  "sandboxGroups.credValPathsInvalid": {
    message: "各パスは / で始まり、? や # を含まず、セグメント全体の * または ** のみを使用してください。",
    description: "Validation error when a path pattern is invalid",
  },
};

export default jaSandboxGroups;
