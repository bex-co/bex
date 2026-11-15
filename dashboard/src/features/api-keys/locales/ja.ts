import type { TranslationEntry } from "@/i18n";

const jaApiKeys: Record<string, TranslationEntry> = {
  "apiKeys.title": {
    message: "API キー",
    description: "Settings API Keys section card title",
  },
  "apiKeys.description": {
    message:
      "スクリプトやエージェント向けのマシン認証情報です。ワークスペース全体で共有され、キーを管理できるメンバーは、自分のキーだけでなくすべてのキーを確認できます。",
    description: "Settings API Keys section card description",
  },
  "apiKeys.cliGuide": {
    message: "CLI をセットアップする。",
    description: "Link from the API Keys card to the bex CLI setup guide",
  },
  "apiKeys.colName": {
    message: "名前",
    description: "API Keys table column header",
  },
  "apiKeys.colClientId": {
    message: "クライアント ID",
    description:
      "API Keys table column header — OAuth2 client_id paired with the secret",
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
    description:
      "API Keys table column header — when a token for the key was last used",
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
    description: "Mint dialog title (secret-shown step)",
  },
  "apiKeys.createdWarning": {
    message:
      "クライアント ID とシークレットを今すぐコピーしてください。シークレットは再表示できません。",
    description: "Mint dialog warning (secret-shown step)",
  },
  "apiKeys.fieldClientId": {
    message: "クライアント ID",
    description: "Mint dialog label for the OAuth2 client_id",
  },
  "apiKeys.fieldSecret": {
    message: "クライアントシークレット",
    description: "Mint dialog label for the one-time client_secret",
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
      "これはベアラートークンではなく、OAuth2 の client_credentials クライアントです。{tokenEndpoint} で交換してください。\n\ngrant_type=client_credentials&client_id=…&client_secret=…\n\nスコープは送信しないでください。このキーは、接続文字列を含め、このワークスペースで完全な権限を持つため、パスワードと同様に扱ってください。返された access_token を Authorization: Bearer として使用します。",
    description:
      "Mint dialog usage copy for the client_credentials exchange (token endpoint from config)",
  },
  "apiKeys.createdDone": {
    message: "完了",
    description: "Mint dialog close button (secret-shown step)",
  },
  "apiKeys.copy": {
    message: "シークレットをコピー",
    description: "Copy-to-clipboard icon button label",
  },
  "apiKeys.copied": {
    message: "クリップボードにコピーしました",
    description: "Toast on a successful secret copy",
  },
  "apiKeys.copyError": {
    message: "クリップボードにコピーできませんでした",
    description: "Toast on a failed secret copy",
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
};

export default jaApiKeys;
