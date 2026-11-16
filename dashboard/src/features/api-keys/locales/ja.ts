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
      "すべてのリクエストで Authorization: Bearer として送信してください。このキーは、接続文字列を含め、このワークスペースで完全な権限を持つため、パスワードと同様に扱ってください。",
    description:
      "Mint dialog copy under the key: how to send it, and that it carries full authority in the workspace",
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
