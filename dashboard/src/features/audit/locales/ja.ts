import type { TranslationEntry } from "@/i18n";

const jaAudit: Record<string, TranslationEntry> = {
  "audit.title": {
    message: "監査ログ",
    description: "Settings Audit Log section card title",
  },
  "audit.description": {
    message:
      "このワークスペースで、誰が何をしたか（許可・拒否を問わず）を新しい順に表示します。ワークスペースの管理者のみが閲覧できます。",
    description: "Settings Audit Log section card description",
  },
  "audit.columnTimestamp": {
    message: "タイムスタンプ",
    description: "Audit Log table column header",
  },
  "audit.columnActor": {
    message: "実行者",
    description: "Audit Log table column header — who performed the action",
  },
  "audit.columnSource": {
    message: "送信元",
    description:
      "Audit Log table column header — the client address and user agent the actor acted from (w9/m450)",
  },
  "audit.columnAction": {
    message: "操作",
    description: "Audit Log table column header — the verb performed",
  },
  "audit.columnStatus": {
    message: "ステータス",
    description: "Audit Log table column header — allowed or denied",
  },
  "audit.columnResource": {
    message: "リソース",
    description: "Audit Log table column header — the object acted on",
  },
  "audit.changeRename": {
    message: "名前 {from} → {to}",
    description:
      "Audit log Resource cell: a service rename's label before and after (w5/183)",
  },
  "audit.changeDomain": {
    message: "ドメイン {domain}",
    description:
      "Audit log Resource cell: the custom domain a domain verb added, removed or verified (w5/183)",
  },
  "audit.changeEnvGroup": {
    message: "環境グループ {name}",
    description:
      "Audit log Resource cell: the environment group a link or unlink changed (w5/183)",
  },
  "audit.changeEnvVars": {
    message: "変数 {keys}",
    description:
      "Audit log Resource cell: the environment variable names an env var row created, changed or deleted, never their values (w9/m449)",
  },
  "audit.changeGrantScopes": {
    message: "スコープ {scopes}",
    description:
      "Audit log Resource cell: the scopes a member granted an OAuth client (GrantOAuthAccessEvent, w4/m454)",
  },
  "audit.changeInstance": {
    message: "インスタンス {instance}",
    description:
      "Audit log Resource cell: the service instance a shell session (StartShellEvent / EndShellEvent) ran on (w9/m451)",
  },
  "audit.statusAllowed": {
    message: "許可",
    description: "Audit Log status badge for a successful action",
  },
  "audit.statusDenied": {
    message: "拒否",
    description: "Audit Log status badge for a denied action",
  },
  "audit.actorUnknown": {
    message: "不明",
    description:
      "Audit Log actor cell placeholder for an unauthenticated caller",
  },
  "audit.oauthDelegation": {
    message: "{client} · {scopes}",
    description:
      "Audit Log actor subtitle for a third-party OAuth grant (client id and canonical scopes)",
  },
  "audit.loadMore": {
    message: "さらに読み込む",
    description: "Button that fetches the next page of audit events",
  },
  "audit.emptyTitle": {
    message: "監査イベントはまだありません",
    description: "Audit Log empty-state title",
  },
  "audit.emptyBody": {
    message: "このワークスペースでの書き込み操作がここに表示されます。",
    description: "Audit Log empty-state body",
  },
  "audit.errorTitle": {
    message: "監査ログを読み込めませんでした",
    description: "Audit Log generic error title",
  },
  "audit.errorBody": {
    message:
      "このワークスペースの監査証跡の読み込み中に問題が発生しました。もう一度お試しください。",
    description: "Audit Log generic error body",
  },
  "audit.unavailableTitle": {
    message: "監査ログが設定されていません",
    description:
      "Audit Log state when the control-plane store isn't wired (503)",
  },
  "audit.unavailableBody": {
    message:
      "この bex デプロイメントには、監査ログのストアが設定されていません。",
    description: "Audit Log unavailable-state body",
  },
  "audit.planTitle": {
    message: "監査ログには Pro プランが必要です",
    description:
      "Audit Log title on a Hobby workspace (PLAN_FEATURE_REQUIRED, w8/m138)",
  },
  "audit.planBody": {
    message:
      "監査ログは Pro プラン以上で利用できます。このワークスペースは Hobby プランです。",
    description: "Audit Log plan notice body on a Hobby workspace (w8/m138)",
  },
  "audit.exportStart": {
    message: "開始日（UTC）",
    description: "Audit Log CSV export start-date label",
  },
  "audit.exportEnd": {
    message: "終了日（UTC）",
    description: "Audit Log CSV export end-date label",
  },
  "audit.exportCSV": {
    message: "CSV でエクスポート",
    description: "Audit Log CSV export button (Render's label)",
  },
  "audit.exportHint": {
    message:
      "最大 {days} 日分のイベント（タイムスタンプ、実行者、イベント、ステータス、メタデータ）をエクスポートします。",
    description: "Audit Log CSV export help text; {days} is the longest range",
  },
  "audit.exportRangeOrder": {
    message: "終了日は開始日と同じかそれ以降にしてください。",
    description: "Audit Log CSV export: end before start",
  },
  "audit.exportRangeTooLong": {
    message: "1 回のエクスポートで指定できる期間は最大 {days} 日です。",
    description:
      "Audit Log CSV export: range over the limit; {days} is the limit",
  },
  "audit.exportTooLarge": {
    message:
      "この期間はイベントが多すぎて、一度にエクスポートできません。より短い期間を選択してください。",
    description: "Audit Log CSV export: row budget exceeded",
  },
  "audit.exportFailed": {
    message: "エクスポートに失敗しました。もう一度お試しください。",
    description: "Audit Log CSV export: request failed",
  },
  "audit.exportRangeMissing": {
    message: "開始日と終了日を選択してください。",
    description: "Audit Log CSV export: a date is empty",
  },
};

export default jaAudit;
