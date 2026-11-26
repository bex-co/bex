import type { TranslationEntry } from "@/i18n";

const jaRegistryCredentials: Record<string, TranslationEntry> = {
  "registryCredentials.title": {
    message: "レジストリ認証情報",
    description:
      "Settings Integrations → Registry Credentials section card title",
  },
  "registryCredentials.description": {
    message:
      "プライベートな外部イメージレジストリ（Docker Hub、GHCR、GitLab Container Registry、ECR など）の認証情報です。既存イメージのサービスがイメージを取得できるようになります。",
    description: "Settings Registry Credentials section card description",
  },
  "registryCredentials.colName": {
    message: "名前",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.colUsername": {
    message: "ユーザー名",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.colStatus": {
    message: "有効期限",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.colCreated": {
    message: "作成日時",
    description: "Registry Credentials table column header",
  },
  "registryCredentials.emptyTitle": {
    message: "レジストリ認証情報はありません",
    description: "Registry Credentials empty-state title",
  },
  "registryCredentials.emptyBody": {
    message:
      "認証情報を追加すると、サービスがプライベートレジストリからイメージを取得できます。",
    description: "Registry Credentials empty-state body",
  },
  "registryCredentials.forbiddenTitle": {
    message: "権限がありません",
    description:
      "Registry Credentials state when the caller lacks permission (403)",
  },
  "registryCredentials.forbiddenBody": {
    message:
      "このワークスペースのレジストリ認証情報を管理する権限がありません。",
    description: "Registry Credentials forbidden-state body",
  },
  "registryCredentials.errorTitle": {
    message: "レジストリ認証情報を読み込めませんでした",
    description: "Registry Credentials generic error title",
  },
  "registryCredentials.errorBody": {
    message: "問題が発生しました。もう一度お試しください。",
    description: "Registry Credentials generic error body",
  },
  "registryCredentials.create": {
    message: "認証情報を追加",
    description: "Button that opens the create dialog",
  },
  "registryCredentials.createTitle": {
    message: "認証情報を追加",
    description: "Add Credential dialog title (Render's)",
  },
  "registryCredentials.createDescription": {
    message: "外部の{link}の保存について詳しく見る。",
    description:
      "Add Credential dialog description; {link} is createDescriptionLink, linking bex's private-registry docs",
  },
  "registryCredentials.fieldHost": {
    message: "レジストリホスト",
    description: "Create dialog host field label",
  },
  "registryCredentials.fieldUsername": {
    message: "ユーザー名",
    description: "Create dialog username field label",
  },
  "registryCredentials.fieldAuthToken": {
    message: "パーソナルアクセストークン",
    description: "Credential secret field label",
  },
  "registryCredentials.hostInvalid": {
    message:
      "ghcr.io や registry.example.com:5000 のように、パスを含まないレジストリホストを入力してください。",
    description:
      "Inline error under the Registry host when it is not a bare host[:port] (w21/053)",
  },
  "registryCredentials.hostTooLong": {
    message: "253 バイト以内で入力してください",
    description: "Inline error under an over-long Registry host (w21/053)",
  },
  "registryCredentials.usernameControl": {
    message: "制御文字は使用できません",
    description:
      "Inline error under a registry username with a control character (w21/053)",
  },
  "registryCredentials.usernameTooLong": {
    message: "253 バイト以内で入力してください",
    description: "Inline error under an over-long registry username (w21/053)",
  },
  "registryCredentials.authTokenTooLong": {
    message: "64 KiB 以内で入力してください",
    description:
      "Inline error under a registry password or token over the 64 KiB limit (w21/053)",
  },
  "registryCredentials.fieldName": {
    message: "名前",
    description: "Credential name field label (required)",
  },
  "registryCredentials.createCancel": {
    message: "キャンセル",
    description: "Create dialog cancel button",
  },
  "registryCredentials.createSubmit": {
    message: "認証情報を追加",
    description: "Add Credential dialog submit button",
  },
  "registryCredentials.createSuccess": {
    message: "{host} の認証情報を追加しました",
    description: "Toast on a successful create",
  },
  "registryCredentials.createError": {
    message: "レジストリ認証情報を追加できませんでした",
    description: "Toast on a failed create",
  },
  "registryCredentials.delete": {
    message: "削除",
    description: "Row action / confirmation button to delete a credential",
  },
  "registryCredentials.deleteNamed": {
    message: "レジストリ認証情報 {name} を削除",
    description:
      "Row delete button aria-label identifying the registry credential",
  },
  "registryCredentials.edit": {
    message: "編集",
    description: "Row action to open the edit-credential dialog",
  },
  "registryCredentials.editTitle": {
    message: "レジストリ認証情報を編集",
    description: "Edit-credential dialog title",
  },
  "registryCredentials.editDescription": {
    message:
      "認証情報の名前やユーザー名の変更、トークンのローテーションができます。レジストリは変更できません。",
    description: "Edit-credential dialog description",
  },
  "registryCredentials.editSubmit": {
    message: "変更を保存",
    description: "Edit-credential dialog submit button",
  },
  "registryCredentials.fieldAuthTokenKeep": {
    message: "現在のトークンを維持する場合は空欄のままにしてください",
    description: "Placeholder for the token field in the edit dialog",
  },
  "registryCredentials.fieldAuthTokenKeepHint": {
    message:
      "保存済みのトークンは表示されません。ローテーションする場合のみ新しい値を入力してください。",
    description: "Hint under the token field in the edit dialog",
  },
  "registryCredentials.updateSuccess": {
    message: "レジストリ認証情報を更新しました。",
    description: "Toast after an edit succeeds",
  },
  "registryCredentials.updateError": {
    message: "レジストリ認証情報を更新できませんでした。",
    description: "Toast after an edit fails",
  },
  "registryCredentials.deleteConfirmTitle": {
    message: "{name} を削除しますか？",
    description: "Delete-confirmation dialog title",
  },
  "registryCredentials.deleteConfirmBody": {
    message:
      "削除できるのは、どのサービスからも使われていない認証情報のみです。先にすべてのサービスから外してください。外していない場合、削除は拒否されます。この操作は元に戻せません。",
    description: "Delete-confirmation dialog body",
  },
  "registryCredentials.deleteCancel": {
    message: "キャンセル",
    description: "Delete-confirmation dialog cancel button",
  },
  "registryCredentials.deleteSuccess": {
    message: "{name} を削除しました",
    description: "Toast on a successful delete",
  },
  "registryCredentials.deleteError": {
    message: "{name} を削除できませんでした",
    description: "Toast on a failed delete",
  },
  "registryCredentials.expired": {
    message: "期限切れ",
    description:
      "Expiry-status badge for a past-expiry credential (w2/m14/t007)",
  },
  "registryCredentials.expiringSoon": {
    message: "まもなく期限切れ",
    description: "Expiry-status badge for a credential nearing expiry",
  },
  "registryCredentials.expiresOn": {
    message: "有効期限: {date}",
    description:
      "Expiry-status text for an active credential with a future expiry",
  },
  "registryCredentials.neverExpires": {
    message: "無期限",
    description: "Expiry-status text for a credential with no expiry set",
  },
  "registryCredentials.createDescriptionLink": {
    message: "レジストリ認証情報",
    description: "Link text inside createDescription",
  },
  "registryCredentials.fieldNamePlaceholder": {
    message: "マイ認証情報",
    description: "Credential name field placeholder",
  },
  "registryCredentials.fieldRegistry": {
    message: "レジストリ",
    description: "Add Credential dialog Registry select label",
  },
  "registryCredentials.fieldRegistryPlaceholder": {
    message: "レジストリを選択",
    description: "Registry select placeholder before a registry is chosen",
  },
  "registryCredentials.fieldHostHint": {
    message: "イメージ URL の先頭にあるホスト（パスは含めません）。",
    description:
      "Hint under the Registry host field (Google Artifact and AWS ECR only)",
  },
  "registryCredentials.hostRegistryMismatch": {
    message: "{example} のような {registry} のホストを入力してください。",
    description:
      "Inline error under a Registry host that belongs to another registry; {registry} is the chosen registry's name, {example} an example host",
  },
  "registryCredentials.fieldAuthTokenOrPassword": {
    message: "パーソナルアクセストークンまたはパスワード",
    description:
      "Credential secret field label for Docker, which also takes a password",
  },
  "registryCredentials.helpGithub": {
    message:
      "{link} の {path} で、{permission} 権限を持つパーソナルアクセストークンを作成してください。",
    description:
      "Help under the Registry select for GitHub; {permission} is read:packages, {link} links GitHub's new-token page, {path} is GitHub's menu path",
  },
  "registryCredentials.helpGitlab": {
    message:
      "{link} で {path} に移動し、{permission} 権限を持つパーソナルアクセストークンを作成してください。",
    description:
      "Help under the Registry select for GitLab; {permission} is read registry, {link} links GitLab's token page, {path} is GitLab's menu path",
  },
  "registryCredentials.helpDocker": {
    message:
      "Docker Hub のユーザー名とパスワードを入力するか、{link} の {path} で {permission} アクセス権限を持つアクセストークンを作成し、パスワードの代わりに使用してください。",
    description:
      "Help under the Registry select for Docker; {permission} is read, {link} links Docker Hub's security settings, {path} is Docker Hub's menu path",
  },
  "registryCredentials.helpGoogle": {
    message:
      "{link} で {permission} 権限を持つサービスアカウントを作成してください。",
    description:
      "Help under the Registry select for Google Artifact; {permission} is the reader role, {link} links Google Cloud's service account page",
  },
  "registryCredentials.helpEcr": {
    message:
      "ユーザー名は ECR リポジトリを所有する AWS アカウント ID です。パーソナルアクセストークンは {command} コマンドを実行して取得します。詳しくは{link}を参照してください。",
    description:
      "Help under the Registry select for AWS ECR; {command} is the aws CLI command, {link} is helpEcrLink",
  },
  "registryCredentials.helpEcrLink": {
    message: "AWS のドキュメント",
    description: "Link text to the AWS ECR authentication docs inside helpEcr",
  },
  "registryCredentials.fieldRegistryImmutable": {
    message: "レジストリは作成後に変更できません。",
    description: "Hint under the read-only Registry in the edit dialog",
  },
};

export default jaRegistryCredentials;
