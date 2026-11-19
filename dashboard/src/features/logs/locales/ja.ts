import type { TranslationEntry } from "@/i18n";

const jaLogs: Record<string, TranslationEntry> = {
  "logs.rangeLabel": {
    message: "ログ履歴の期間",
    description: "Accessible label for the relative log-history range controls",
  },
  "logs.rangeHistoryNote": {
    message:
      "期間は履歴を制限します。ライブモードでは、新しい行が届くたびに追加されます。",
    description:
      "Explains how the bounded history window interacts with live tail",
  },
  "logs.retentionWindow_other": {
    message: "{plan} プランではログを {count} 日間保持します。",
    description:
      "Log range picker hint: how many days the workspace plan keeps logs (w9/m389, from bex-api logRetention)",
  },
  "logs.retentionUpgrade_other": {
    message: "{plan} にアップグレードすると {count} 日間保持できます。",
    description:
      "Log range picker hint: the cheapest plan that keeps logs longer, and its days (w9/m389)",
  },
  "logs.typeLabel": {
    message: "ログの種類",
    description: "Accessible label for the log-type filter dropdown",
  },
  "logs.typeAll": {
    message: "すべてのログ",
    description: "Log-type filter option: no type filter",
  },
  "logs.typeApplication": {
    message: "アプリケーションログ",
    description: "Log-type filter option: application (app) logs",
  },
  "logs.typeRequest": {
    message: "リクエストログ",
    description:
      "Log-type filter option: request logs (Traefik access lines, from the durable store)",
  },
  "logs.levelLabel": {
    message: "レベル",
    description: "Accessible label for the log-level filter dropdown",
  },
  "logs.levelAll": {
    message: "すべてのレベル",
    description: "Level filter option: no level filter",
  },
  "logs.methodLabel": {
    message: "メソッド",
    description: "Accessible label for the HTTP-method filter dropdown",
  },
  "logs.methodAll": {
    message: "すべてのメソッド",
    description: "Method filter option: no method filter",
  },
  "logs.statusCodeLabel": {
    message: "ステータスコード",
    description: "Accessible label for the HTTP-status filter dropdown",
  },
  "logs.statusCodeAll": {
    message: "すべてのステータス",
    description: "Status-code filter option: no status filter",
  },
  "logs.instanceLabel": {
    message: "インスタンス",
    description: "Accessible label for the instance (replica) filter dropdown",
  },
  "logs.instanceAll": {
    message: "すべてのインスタンス",
    description: "Instance filter option: no instance filter",
  },
  "logs.filterByInstance": {
    message: "インスタンス {instance} でログを絞り込み",
    description:
      "Accessible label for the clickable instance slug on an application log line",
  },
  "logs.pathLabel": {
    message: "リクエストパス",
    description: "Accessible label for the request-path filter input",
  },
  "logs.filtersButton": {
    message: "フィルター",
    description:
      "Trigger for the structured-filters popover (level/method/status/instance/path)",
  },
  "logs.chipRemove": {
    message: "{label} フィルターを削除",
    description:
      "Accessible label for an active-filter chip's clear button; {label} is the filter's field name",
  },
  "logs.pathPlaceholder": {
    message: "パスで絞り込み",
    description: "Placeholder for the request-path filter input",
  },
  "logs.hostLabel": {
    message: "リクエストホスト",
    description:
      "Accessible label for the request-host filter input and its chip (Render's host log filter, w4/538)",
  },
  "logs.hostPlaceholder": {
    message: "ホストで絞り込み",
    description: "Placeholder for the request-host filter input",
  },
  "logs.liveUnsupported": {
    message:
      "ライブテールはオフです。メソッド、ステータス、パス、ホストはリクエストログを絞り込むため、これらを指定するとアプリケーションログはテールできません。",
    description:
      "Note when the chosen log type cannot carry an active filter, so live tail is off (a request filter on app logs)",
  },
  "logs.liveWindowClosed": {
    message:
      "ライブテールはオフです。このカスタム期間はすでに終了しているため、新しい行は届きません。",
    description:
      "Note when the selected custom time range ended before it was applied, so live tail is off",
  },
  "logs.storeRequiredTitle": {
    message: "リクエストログにはログストアが必要です",
    description:
      "Empty-state title when a request/structured-filter query hits a deployment with no durable store (503)",
  },
  "logs.storeRequiredBody": {
    message:
      "リクエストログと、レベル・ステータス・メソッド・パスのフィルターは、永続ログストアから読み取りますが、ここでは設定されていません。ストアが接続されている環境で利用できます。",
    description: "Empty-state body for the store-unavailable (503) state",
  },
  "logs.requestPlanTitle": {
    message: "HTTP リクエストログには Pro プランが必要です",
    description:
      "Notice title in place of Request logs on a Hobby workspace (PLAN_FEATURE_REQUIRED, w8/m138)",
  },
  "logs.requestPlanBody": {
    message:
      "リクエストログは Pro プラン以上で利用できます。このワークスペースは Hobby のため、アプリケーションログとビルドログのみ表示されます。",
    description:
      "Notice body in place of Request logs on a Hobby workspace (w8/m138)",
  },
  "logs.searchPlaceholder": {
    message: "ログを検索",
    description: "Placeholder for the log search box (text filter)",
  },
  "logs.live": {
    message: "ライブ",
    description: "Label for the live-tail on/off toggle",
  },
  "logs.jumpToLatest": {
    message: "最新に移動",
    description: "Button to re-pin autoscroll to the newest log line",
  },
  "logs.linesLabel": {
    message: "ログの行",
    description:
      "Accessible name of the scrollable log list, which takes keyboard focus so arrow keys and Page Up/Down scroll it",
  },
  "logs.endOfLog": {
    message: "ログの終わり",
    description:
      "Closing line after the last row of a log that will not grow (a finished deploy's), so a short log reads as complete",
  },
  "logs.loading": {
    message: "ログを読み込み中…",
    description: "Shown while the first historical page is loading",
  },
  "logs.loadingOlder": {
    message: "古いログを読み込み中…",
    description:
      "Shown at the top of the pane while a page-back request is in flight",
  },
  "logs.loadOlder": {
    message: "古いログを読み込む",
    description:
      "Button in the truncation notice that fetches the next older page — the keyboard/underfilled-pane path to what scrolling to the top does",
  },
  "logs.truncatedNotice": {
    message:
      "この期間の一部のみ読み込まれています。続けるには、古いログを読み込んでください。",
    description:
      "Banner when hasMore is true: more of the selected range remains unloaded, however many older pages are already shown, so it names no line count (w4/280)",
  },
  "logs.partialSearchNotice": {
    message:
      "この期間の最新の部分のみを検索しました。検索を続けるには、古いログを読み込んでください。",
    description:
      "Banner when a time-boxed search returned less than a page but hasMore is true: the older part of the range is unsearched, not empty (w4/m140)",
  },
  "logs.timeoutTitle": {
    message: "この検索に時間がかかりすぎました",
    description:
      "State title when the logs query ran out of the server's time budget (QUERY_TIMEOUT, w4/m140)",
  },
  "logs.timeoutBody": {
    message: "期間を短くするか、より具体的な検索語を試してください。",
    description: "State body under logs.timeoutTitle",
  },
  "logs.streaming": {
    message: "ライブ — 新しい行をストリーミング中",
    description: "Status under the log list when the SSE tail is connected",
  },
  "logs.paused": {
    message: "ライブテールを一時停止中",
    description: "Status under the log list when live tail is toggled off",
  },
  "logs.disconnected": {
    message: "ライブテールが切断されました。再接続しています…",
    description: "Banner when the SSE stream drops",
  },
  "logs.emptyTitle": {
    message: "この期間にログはありません",
    description:
      "Empty-state title when the selected (always-bounded) history range holds no lines. Never claims the service has never logged — the range can't establish that (w6/m111)",
  },
  "logs.emptyBody": {
    message: "この期間に該当するログ行はありません。",
    description:
      "Empty-state body with no filters applied — honest about the bounded window, not the service's whole history (w6/m111)",
  },
  "logs.emptyFilteredBody": {
    message: "これらのフィルターに一致するログはありません。",
    description:
      "Empty-state body when a type/text/structured filter yields nothing",
  },
  "logs.emptyFilteredTitle": {
    message: "一致するログはありません",
    description:
      "Empty-state title when a type/text/structured filter yields nothing",
  },
  "logs.errorTitle": {
    message: "ログを読み込めませんでした",
    description: "Error-state title when the logs query fails",
  },
  "logs.errorTitleWithReason": {
    message: "ログを読み込めませんでした: {reason}",
    description:
      "Inline banner over loaded log lines when a later read was refused; reason is the server's refusal",
  },
  "logs.observabilityViewEyebrow": {
    message: "オブザーバビリティビュー",
    description:
      "Eyebrow over a project observability view's name (Render's OBSERVABILITY VIEW, w3/m505)",
  },
  "logs.applicationLogsTitle": {
    message: "アプリケーションログ",
    description:
      "Document title part for a project's Application Logs view page (w3/m505)",
  },
  "logs.requestLogsTitle": {
    message: "リクエストログ",
    description:
      "Document title part for a project's Request Logs view page (w3/m505)",
  },
  "logs.projectScopeEmptyTitle": {
    message: "このビューにはまだ読み取るものがありません",
    description:
      "Project log view state: the view's scope holds no resource to read (w3/m505)",
  },
  "logs.projectScopeEmptyBody": {
    message:
      "このビューはプロジェクトのサービス、cron ジョブ、Postgres データベース、Key Value インスタンスを読み取りますが、まだ何も含まれていません。",
    description:
      "Project Application Logs view with no service, cron job, Postgres or Key Value in it (w3/m505)",
  },
  "logs.projectRequestScopeEmptyBody": {
    message:
      "リクエストログはプロジェクトの Web サービスを読み取りますが、このビューにはまだありません。",
    description:
      "Project Request Logs view with no web service in it (w3/m505)",
  },
  "logs.projectEmptyTitle": {
    message: "この期間のログはありません",
    description:
      "Project log view state: its resources logged nothing in the selected range (w3/m505)",
  },
  "logs.projectEmptyBody": {
    message:
      "このビューのリソースは、選択した期間に何もログを記録していません。",
    description:
      "Project log view state body: no line in the range from any of its resources (w3/m505)",
  },
  "logs.viewNotFoundTitle": {
    message: "ビューが見つかりません",
    description:
      "Project observability page whose savedSearchId names no view of this project (w3/m505)",
  },
  "logs.viewNotFoundBody": {
    message:
      "このプロジェクトには、このリンクのビューがありません。削除された可能性があります。",
    description: "Body of the view-not-found state (w3/m505)",
  },
  "logs.viewOpen": {
    message: "{name} を開く",
    description:
      "Link from the view-not-found state to the project's default view of the same type; name is the view's name",
  },
  "logs.viewLoadErrorTitle": {
    message: "このビューを読み込めませんでした",
    description:
      "Project observability page whose saved searches could not be read (w3/m505)",
  },
  "logs.viewLoadErrorBody": {
    message:
      "プロジェクトのビューを読み取れませんでした。ページを再読み込みしてもう一度お試しください。",
    description: "Body of the view load failure (w3/m505)",
  },
};

export default jaLogs;
