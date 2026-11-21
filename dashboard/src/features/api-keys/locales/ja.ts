import type { TranslationEntry } from "@/i18n";

const jaApiKeys: Record<string, TranslationEntry> = {
  "apiKeys.title": {
    message: "API キー",
    description: "Settings API Keys section card title",
  },
  "apiKeys.description": {
    message:
      "スクリプトやエージェント向けのマシン認証情報です。各キーは選択された権限だけを使え、作成したメンバーの現在のロールで許可される範囲を超えることはなく、このワークスペースでのみ動作します。自分が作成したキーを確認・管理でき、Admin はすべてのメンバーのキーを確認して失効させることもできます。",
    description:
      "Settings API Keys section card description: what a key is, that it uses only its chosen permissions within its creator's current role in this workspace (w9/m496), and who sees which keys: every member their own, as on Render's Account Settings, and an Admin every member's too (w9/m539)",
  },
  "apiKeys.cliGuide": {
    message: "CLI をセットアップする。",
    description: "Link from the API Keys card to the bex CLI setup guide",
  },
  "apiKeys.scopeLabel": {
    message: "表示する API キー",
    description:
      "Accessible name of an Admin's switch between their own API keys and every member's (w9/m539)",
  },
  "apiKeys.scopeOwn": {
    message: "自分のキー",
    description: "API Keys switch option: the keys the caller created",
  },
  "apiKeys.scopeAll": {
    message: "全メンバーのキー",
    description:
      "API Keys switch option, an Admin's only: every member's keys in the workspace",
  },
  "apiKeys.colName": {
    message: "名前",
    description: "API Keys table column header",
  },
  "apiKeys.colClientId": {
    message: "クライアント ID",
    description:
      "API Keys table column header: the key's id, which is also its OAuth2 client_id",
  },
  "apiKeys.colCreated": {
    message: "作成日時",
    description: "API Keys table column header",
  },
  "apiKeys.colCreatedBy": {
    message: "作成者",
    description: "API Keys table column header — who minted the key",
  },
  "apiKeys.colLastUsed": {
    message: "最終使用",
    description: "API Keys table column header: when the key was last used",
  },
  "apiKeys.neverUsed": {
    message: "未使用",
    description: "API Keys last-used cell when the key has never been used",
  },
  "apiKeys.emptyTitle": {
    message: "API キーはありません",
    description: "API Keys empty-state title",
  },
  "apiKeys.emptyBody": {
    message: "スクリプトやエージェントを認証するためのキーを作成してください。",
    description: "API Keys empty-state body",
  },
  "apiKeys.forbiddenTitle": {
    message: "権限がありません",
    description: "API Keys state when the caller lacks permission (403)",
  },
  "apiKeys.forbiddenBody": {
    message: "このワークスペースの API キーを管理する権限がありません。",
    description: "API Keys forbidden-state body",
  },
  "apiKeys.errorTitle": {
    message: "API キーを読み込めませんでした",
    description: "API Keys generic error title",
  },
  "apiKeys.errorBody": {
    message: "問題が発生しました。もう一度お試しください。",
    description: "API Keys generic error body",
  },
  "apiKeys.create": {
    message: "API キーを作成",
    description: "Button that opens the mint dialog",
  },
  "apiKeys.createTitle": {
    message: "API キーを作成",
    description: "Mint dialog title (name step)",
  },
  "apiKeys.createDescription": {
    message: "後で見分けられるように、このキーに名前を付けてください。",
    description: "Mint dialog description (name step)",
  },
  "apiKeys.fieldName": {
    message: "名前",
    description: "Mint dialog name field label",
  },
  "apiKeys.fieldNamePlaceholder": {
    message: "例: deploy-agent",
    description: "Mint dialog name field placeholder",
  },
  "apiKeys.createCancel": {
    message: "キャンセル",
    description: "Mint dialog cancel button (name step)",
  },
  "apiKeys.createSubmit": {
    message: "作成",
    description: "Mint dialog submit button (name step)",
  },
  "apiKeys.createdTitle": {
    message: "API キーを作成しました",
    description: "Mint dialog title (key-shown step)",
  },
  "apiKeys.createdWarning": {
    message: "キーを今すぐコピーしてください。再表示はできません。",
    description:
      "Mint dialog warning (key-shown step): the key is shown only once",
  },
  "apiKeys.fieldKey": {
    message: "API キー",
    description:
      "Mint dialog label for the key itself, the one string sent as Authorization: Bearer",
  },
  "apiKeys.copyKey": {
    message: "API キーをコピー",
    description: "Copy button aria-label for the key",
  },
  "apiKeys.keyHelp": {
    message:
      "すべてのリクエストで Authorization: Bearer として送信してください。このキーは選択した権限だけを使え、このワークスペースでのあなたの現在のメンバーロールで許可される範囲を超えることはありません。パスワードと同様に扱ってください。",
    description:
      "Mint dialog copy under the key: how to send it, and that it uses only the chosen permissions, capped by the creator's current member role in this workspace (w9/m496)",
  },
  "apiKeys.curlTitle": {
    message: "リクエスト例",
    description:
      "Mint dialog label for the curl example that lists services with the key",
  },
  "apiKeys.copyCurl": {
    message: "リクエスト例をコピー",
    description: "Copy button aria-label for the curl example",
  },
  "apiKeys.oauthDetails": {
    message: "OAuth クライアント認証情報",
    description:
      "Mint dialog title of the collapsed detail holding the client ID, the client secret and the optional token exchange",
  },
  "apiKeys.fieldClientId": {
    message: "クライアント ID",
    description:
      "Mint dialog label for the OAuth2 client_id, inside the OAuth client credentials detail",
  },
  "apiKeys.fieldSecret": {
    message: "クライアントシークレット",
    description:
      "Mint dialog label for the one-time client_secret, inside the OAuth client credentials detail",
  },
  "apiKeys.copyClientId": {
    message: "クライアント ID をコピー",
    description: "Copy button aria-label for the client_id",
  },
  "apiKeys.copyClientIdNamed": {
    message: "{name} のクライアント ID をコピー",
    description: "Copy button aria-label for one API client row's client_id",
  },
  "apiKeys.copySecret": {
    message: "クライアントシークレットをコピー",
    description: "Copy button aria-label for the client_secret",
  },
  "apiKeys.exchangeHelp": {
    message:
      "このキーは OAuth2 クライアントとしても使えます。有効期間の短いトークンを使いたい場合は、{tokenEndpoint} でクライアント ID とクライアントシークレットをアクセストークンに交換してください。\n\ngrant_type=client_credentials&client_id=…&client_secret=…\n\nスコープは送信しないでください。返された access_token を、キーの代わりに Authorization: Bearer として使用します。",
    description:
      "Mint dialog copy inside the OAuth client credentials detail: the optional client_credentials exchange (token endpoint from config)",
  },
  "apiKeys.createdDone": {
    message: "完了",
    description: "Mint dialog close button (key-shown step)",
  },
  "apiKeys.copied": {
    message: "クリップボードにコピーしました",
    description:
      "Toast on a successful copy (the key, the example request, the client ID or the secret)",
  },
  "apiKeys.copyError": {
    message: "クリップボードにコピーできませんでした",
    description: "Toast on a failed copy",
  },
  "apiKeys.createSuccess": {
    message: "{name} を作成しました",
    description: "Toast on a successful mint",
  },
  "apiKeys.createError": {
    message: "{name} を作成できませんでした",
    description: "Toast on a failed mint",
  },
  "apiKeys.revoke": {
    message: "失効",
    description: "Row action / confirmation button to revoke a key",
  },
  "apiKeys.revokeNamed": {
    message: "API キー {name} を失効",
    description: "Row revoke button aria-label identifying the API key",
  },
  "apiKeys.revokeConfirmTitle": {
    message: "{name} を失効しますか？",
    description: "Revoke-confirmation dialog title",
  },
  "apiKeys.revokeConfirmBody": {
    message:
      "このキーで認証しているものはすべて、直ちに動作しなくなります。この操作は元に戻せません。",
    description: "Revoke-confirmation dialog body",
  },
  "apiKeys.revokeConfirmBodyOther": {
    message:
      "このキーは {creator} が作成しました。このキーで認証しているものはすべて、直ちに動作しなくなります。この操作は元に戻せません。",
    description:
      "Revoke-confirmation dialog body for another member's key in an Admin's every-member view; {creator} is who created it",
  },
  "apiKeys.revokeCancel": {
    message: "キャンセル",
    description: "Revoke-confirmation dialog cancel button",
  },
  "apiKeys.revokeSuccess": {
    message: "{name} を失効しました",
    description: "Toast on a successful revoke",
  },
  "apiKeys.revokeError": {
    message: "{name} を失効できませんでした",
    description: "Toast on a failed revoke",
  },
  "apiKeys.colPermissions": {
    message: "権限",
    description: "API Keys table column header: what each key may do (w9/m496)",
  },
  "apiKeys.permSummaryAll": {
    message: "すべて",
    description:
      "Key row permissions summary: the key has every permission its creator's member role allows (Render's All)",
  },
  "apiKeys.permSummaryReadOnly": {
    message: "読み取り専用",
    description:
      "Key row permissions summary: every permission the key has is at Read, so it can change nothing",
  },
  "apiKeys.permSummaryNone": {
    message: "権限なし",
    description:
      "Key row permissions summary: the key has no permissions (Clear all)",
  },
  "apiKeys.permSummaryCustom": {
    message: "カスタム",
    description:
      "Key row permissions summary: the key has a chosen set of permissions; followed by their count",
  },
  "apiKeys.editPermissionsNamed": {
    message: "{name} の権限を変更",
    description:
      "Gear button aria-label on a key row; opens the permissions editor",
  },
  "apiKeys.editPermissionsTitle": {
    message: "権限を編集",
    description: "Permissions editor dialog title",
  },
  "apiKeys.editPermissionsDescription": {
    message:
      "{name} が使える権限を変更し、「変更を保存」をクリックしてください。新しい権限はすぐに適用されます。",
    description:
      "Permissions editor dialog description; {name} is the key's name",
  },
  "apiKeys.editPermissionsCancel": {
    message: "キャンセル",
    description: "Permissions editor dialog cancel button",
  },
  "apiKeys.editPermissionsSave": {
    message: "変更を保存",
    description: "Permissions editor dialog save button (Render's words)",
  },
  "apiKeys.permissionsSaved": {
    message: "{name} の権限を更新しました",
    description: "Toast after a key's permissions are saved",
  },
  "apiKeys.permissionsSaveError": {
    message: "{name} の権限を更新できませんでした",
    description:
      "Toast when saving a key's permissions fails (the server's reason is shown instead when it gives one)",
  },
  "apiKeys.permIntro": {
    message: "この API キーが使える権限を選択してください。",
    description:
      "Permission picker heading in the create dialog and the permissions editor (Render's words)",
  },
  "apiKeys.permPresetAll": {
    message: "すべて",
    description:
      "Picker preset button: grant every permission the member role allows (Render's All)",
  },
  "apiKeys.permPresetReadOnly": {
    message: "読み取り専用",
    description:
      "Picker preset button: grant Read on every permission that offers it (Render's Read only)",
  },
  "apiKeys.permPresetClear": {
    message: "すべてクリア",
    description:
      "Picker preset button: remove every permission from the key (Render's Clear all)",
  },
  "apiKeys.permAllHelp": {
    message:
      "「すべて」は、作成者のメンバーロールで許可されるすべての権限をすべてのリソースに付与し、ロールが変わるとそれに従います。",
    description: "Picker note while the All preset is selected: what All means",
  },
  "apiKeys.permNoneHelp": {
    message: "権限が選択されていません。このキーは自身の識別しかできません。",
    description: "Picker note when every permission is None (Clear all)",
  },
  "apiKeys.permAreaToggle": {
    message: "{area} の権限を付与",
    description:
      "Switch aria-label on one area of the picker; {area} is Render's area name, e.g. Services (kept in English)",
  },
  "apiKeys.permLevelNone": {
    message: "なし",
    description: "Picker option on a permission: the key gets none of it",
  },
  "apiKeys.permLevelGrants": {
    message: "{level} で許可される操作:",
    description:
      "Picker label over the actions a permission's chosen level grants; {level} is Render's level name, e.g. Read & write (kept in English)",
  },
  "apiKeys.permRequiresRole.viewer": {
    message: "{level} には 閲覧者 ロールが必要です。",
    description:
      "Reason beside a level the viewer's role cannot grant; {level} is Render's level name (kept in English); the lowest role that can is Viewer",
  },
  "apiKeys.permRequiresRole.contributor": {
    message: "{level} には コントリビューター ロールが必要です。",
    description:
      "Reason beside a level the viewer's role cannot grant; {level} is Render's level name (kept in English); the lowest role that can is Contributor",
  },
  "apiKeys.permRequiresRole.developer": {
    message: "{level} には 開発者 ロールが必要です。",
    description:
      "Reason beside a level the viewer's role cannot grant; {level} is Render's level name (kept in English); the lowest role that can is Developer",
  },
  "apiKeys.permRequiresRole.admin": {
    message: "{level} には 管理者 ロールが必要です。",
    description:
      "Reason beside a level the viewer's role cannot grant; {level} is Render's level name (kept in English); the lowest role that can is Admin",
  },
  "apiKeys.permRequiresHigherRole": {
    message: "{level} にはより上位のメンバーロールが必要です。",
    description:
      "Reason beside a level the viewer's role cannot grant, when bex-api names a role the dashboard does not know",
  },
  "apiKeys.permCatalogLoading": {
    message: "権限を読み込んでいます…",
    description:
      "Accessible name of the picker while its permission list loads",
  },
  "apiKeys.permCatalogErrorCreate": {
    message:
      "権限の一覧を読み込めませんでした。今作成するキーにはすべての権限が付与されます。",
    description:
      "Create dialog note when the permission list fails to load: creating still works and grants All",
  },
  "apiKeys.permCatalogErrorEdit": {
    message: "権限の一覧を読み込めませんでした。",
    description:
      "Permissions editor note when the permission list fails to load, beside a Try again button",
  },
  "apiKeys.permSummaryCount_other": {
    message: "{count} 件の権限",
    description:
      "Key row permissions summary beside Custom: how many permissions the key has",
  },
};

export default jaApiKeys;
