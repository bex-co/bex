import type { TranslationEntry } from "@/i18n";

const jaInvites: Record<string, TranslationEntry> = {
  "invites.title": {
    message: "ワークスペースへの招待",
    description: "Invitation review: title",
  },
  "invites.authenticate": {
    message:
      "ワークスペースへの招待を確認するには、アカウントを作成するかログインしてください。",
    description: "Invitation review: authenticate",
  },
  "invites.signUp": {
    message: "登録",
    description: "Invitation review: signUp",
  },
  "invites.signIn": {
    message: "ログイン",
    description: "Invitation review: signIn",
  },
  "invites.joinTitle": {
    message: "{workspace} に参加",
    description: "Invitation review: joinTitle",
  },
  "invites.memberTitle": {
    message: "あなたは {workspace} のメンバーです",
    description: "Invitation review: memberTitle",
  },
  "invites.role": {
    message: "{role} として参加します。",
    description: "Invitation review: role",
  },
  "invites.memberRole": {
    message: "あなたのロールは {role} です。",
    description: "Invitation review: memberRole",
  },
  "invites.inviter": {
    message: "{email} からの招待",
    description: "Invitation review: inviter",
  },
  "invites.account": {
    message: "{email} としてログイン中",
    description: "Invitation review: account",
  },
  "invites.join": {
    message: "ワークスペースに参加",
    description: "Invitation review: join",
  },
  "invites.joining": {
    message: "参加中…",
    description: "Invitation review: joining",
  },
  "invites.open": {
    message: "ワークスペースを開く",
    description: "Invitation review: open",
  },
  "invites.opening": {
    message: "開いています…",
    description: "Invitation review: opening",
  },
  "invites.notNow": {
    message: "今はしない",
    description: "Invitation review: notNow",
  },
  "invites.continue": {
    message: "bex に進む",
    description: "Invitation review: continue",
  },
  "invites.unavailableTitle": {
    message: "招待は利用できません",
    description: "Invitation review: unavailableTitle",
  },
  "invites.invalid": {
    message:
      "この招待リンクは無効か、取り消されています。ワークスペースの管理者に新しい招待を依頼してください。",
    description: "Invitation review: invalid",
  },
  "invites.expired": {
    message:
      "この招待は期限切れです。ワークスペースの管理者に再送を依頼してください。",
    description: "Invitation review: expired",
  },
  "invites.used": {
    message:
      "この招待はすでに使用されています。参加したアカウントでログインするか、新しい招待を依頼してください。",
    description: "Invitation review: used",
  },
  "invites.planLimit": {
    message:
      "このワークスペースは、現在のプランではこれ以上メンバーを受け入れられません。ワークスペースの管理者にプランと席数の確認を依頼してから、もう一度お試しください。",
    description: "Invitation review: planLimit",
  },
  "invites.retryError": {
    message:
      "リクエストを完了できませんでした。招待はこのタブに保存されています。もう一度お試しください。",
    description: "Invitation review: retryError",
  },
  "invites.accessPending": {
    message:
      "メンバーシップは確認されましたが、ワークスペースへのアクセスの準備がまだできていません。もう一度ワークスペースを開いてみてください。",
    description: "Invitation review: accessPending",
  },
  "invites.storageUnavailable": {
    message:
      "このブラウザーに招待を保存できませんでした。サイトのストレージを許可してから、招待リンクをもう一度開いてください。",
    description: "Invitation review: storageUnavailable",
  },
  "invites.retry": {
    message: "もう一度試す",
    description: "Invitation review: retry",
  },
};

export default jaInvites;
