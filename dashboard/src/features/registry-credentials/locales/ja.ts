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
    message: "レジストリ認証情報を追加",
    description: "Create dialog title",
  },
  "registryCredentials.createDescription": {
    message:
      "安全に保存されます。パスワードとトークンは、この後は二度と表示されません。",
    description: "Create dialog description",
  },
  "registryCredentials.fieldHost": {
    message: "レジストリホスト",
    description: "Create dialog host field label",
  },
  "registryCredentials.fieldHostPlaceholder": {
    message: "例: ghcr.io, docker.io, registry.gitlab.com",
    description: "Create dialog host field placeholder",
  },
  "registryCredentials.fieldUsername": {
    message: "ユーザー名",
    description: "Create dialog username field label",
  },
  "registryCredentials.fieldAuthToken": {
    message: "パスワードまたはアクセストークン",
    description: "Create dialog secret field label",
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
    message: "表示名",
    description: "Create dialog optional display-name field label",
  },
  "registryCredentials.fieldNameOptional": {
    message: "（任意。省略するとホスト名になります）",
    description: "Create dialog display-name field's optional hint",
  },
  "registryCredentials.createCancel": {
    message: "キャンセル",
    description: "Create dialog cancel button",
  },
  "registryCredentials.createSubmit": {
    message: "追加",
    description: "Create dialog submit button",
  },
  "registryCredentials.createSuccess": {
    message: "{host} の認証情報を追加しました",
    description: "Toast on a successful create",
  },
  "registryCredentials.createError": {
    message: "{host} の認証情報を追加できませんでした",
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
      "認証情報の名前やユーザー名の変更、トークンのローテーションができます。レジストリホストは変更できません。",
    description: "Edit-credential dialog description",
  },
  "registryCredentials.editSubmit": {
    message: "変更を保存",
    description: "Edit-credential dialog submit button",
  },
  "registryCredentials.fieldHostImmutable": {
    message: "レジストリホストは作成後に変更できません。",
    description: "Hint under the read-only host field in the edit dialog",
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
};

export default jaRegistryCredentials;
