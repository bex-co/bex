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
  "logs.linesKeysHint": {
    message:
      "上下の矢印キーで行を移動し、Enter でその行のインスタンスだけを表示します。",
    description:
      "Logs list: screen-reader hint on the focusable log region, naming its row keys",
  },
  "logs.linesKeysShort": {
    message: "↑↓ 行を移動 · Enter: このインスタンスのみ表示",
    description:
      "Logs tab: the keys of the focused log list, shown in its corner while it has keyboard focus — arrows move between lines, Enter shows only that line's instance",
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
  "logs.errorBody": {
    message:
      "この期間のリクエストに失敗しました。もう一度試すか、より短い期間を選んでください。",
    description:
      "Logs tab: body when the log history read failed, under logs.errorTitle; the server's own reason follows as a detail line",
  },
  "logs.errorBodyLive": {
    message:
      "新しい行は届きしだいここに表示されます。もう一度試すと、この期間の履歴を読み込みます。",
    description:
      "Logs tab: body when the history read failed while the live tail still streams \u2014 new lines will still land in the panel",
  },
  "logs.filtersDone": {
    message: "完了",
    description: "Logs tab: button closing the Filters panel",
  },
  "logs.emptyBodyRange": {
    message: "期間を長くするか、ライブをオンにして新しい行を待ちます。",
    description:
      "Logs tab: body when the chosen range holds no lines, under logs.emptyTitle \u2014 what to try next",
  },
  "logs.turnOnLive": {
    message: "ライブをオンにする",
    description:
      "Logs tab: button in the empty state that switches the live tail on",
  },
  "logs.clearFilters": {
    message: "フィルターをクリア",
    description:
      "Logs tab: button in the no-match state that clears the search and every filter",
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
  "logs.projectRoleLimitTitle": {
    message: "ログを表示できません",
    description:
      "Project log view state: the caller's workspace role reads no logs, so the view reads nothing (w3/078)",
  },
  "logs.projectRoleLimitBody": {
    message:
      "このワークスペースのログを表示するには、コントリビューター以上のロールが必要です。ワークスペースの管理者にロールの変更を依頼してください。",
    description:
      "Project log view state body: logs need the Contributor role or higher, as on Render (w3/078)",
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
  "logs.newView": {
    message: "新しいビュー",
    description:
      "Project sidebar button that opens the TYPE menu for a new observability view (w3/m507, Render's '+ New View')",
  },
  "logs.newViewTypeHeading": {
    message: "タイプ",
    description:
      "Heading of the New View menu listing view types (Render's TYPE)",
  },
  "logs.editView": {
    message: "ビューを編集",
    description:
      "Observability view header button that opens the Edit view modal (w3/m507)",
  },
  "logs.duplicateView": {
    message: "複製",
    description:
      "Observability view header button that copies the view (w3/m507)",
  },
  "logs.deleteView": {
    message: "ビューを削除",
    description:
      "Accessible name and confirm label of the observability view's trash button (w3/m507)",
  },
  "logs.deleteViewTitle": {
    message: "{name} を削除しますか？",
    description:
      "Title of the confirmation before deleting an observability view; name is the view's name",
  },
  "logs.deleteViewBody": {
    message:
      "このビューはプロジェクトの全員に対して削除されます。プロジェクトのリソースとそのログには影響しません。",
    description:
      "Body of the delete-view confirmation: views are shared by the whole project (w3/m507)",
  },
  "logs.viewDeleted": {
    message: "{name} を削除しました",
    description:
      "Toast after an observability view was deleted; name is the view's name",
  },
  "logs.deleteViewFailed": {
    message: "ビューを削除できませんでした",
    description:
      "Toast when deleting an observability view failed without a server reason",
  },
  "logs.viewDuplicated": {
    message: "{name} を作成しました",
    description: "Toast after a view was duplicated; name is the copy's name",
  },
  "logs.duplicateViewFailed": {
    message: "ビューを複製できませんでした",
    description:
      "Toast when duplicating an observability view failed without a server reason",
  },
  "logs.editViewTitle": {
    message: "ビューを編集",
    description: "Title of Render's Edit view modal (w3/m507)",
  },
  "logs.newViewTitle": {
    message: "新しいビュー",
    description: "Title of the view modal when New View opened it (w3/m507)",
  },
  "logs.viewNameLabel": {
    message: "名前",
    description: "Label of the view name field in the Edit view modal",
  },
  "logs.viewNameRequired": {
    message: "ビューの名前を入力してください。",
    description: "Inline refusal of a blank view name",
  },
  "logs.viewFiltersLabel": {
    message: "フィルター",
    description: "Heading of the filter rows in the Edit view modal",
  },
  "logs.viewAddFilter": {
    message: "フィルターを追加",
    description: "Accessible name of the Edit view modal's add (+) button",
  },
  "logs.viewNoFilters": {
    message:
      "フィルターなし：このビューはプロジェクトのすべてのリソースを読み取ります。",
    description:
      "Edit view modal with no filter rows: the view reads the whole project",
  },
  "logs.viewSharedNotice": {
    message:
      "フィルターを編集すると、プロジェクトの全員に対してビューが更新されます。",
    description:
      "Render's notice under the Edit view filters: a view is shared by the whole project",
  },
  "logs.viewSaveFailed": {
    message: "ビューを保存できませんでした。もう一度お試しください。",
    description:
      "Edit view modal error when saving failed without a server reason",
  },
  "logs.viewClose": {
    message: "閉じる",
    description:
      "Edit view modal button that discards the changes (Render's Close)",
  },
  "logs.viewDone": {
    message: "完了",
    description: "Edit view modal button that saves the view (Render's Done)",
  },
  "logs.viewFilterField": {
    message: "フィールド",
    description: "Accessible name of a filter row's Field picker",
  },
  "logs.viewFilterOperator": {
    message: "演算子",
    description: "Accessible name of a filter row's Operator picker",
  },
  "logs.viewFilterValuesFor": {
    message: "{field} の値",
    description:
      "Accessible name of a filter row's Values picker; field is the row's field",
  },
  "logs.viewValuesRequired": {
    message: "値を 1 つ以上選択してください。",
    description: "Inline refusal of a filter row with no value",
  },
  "logs.viewRemoveFilter": {
    message: "{field} フィルターを削除",
    description:
      "Accessible name of a filter row's trash button; field is the row's field",
  },
  "logs.viewOperatorIs": {
    message: "等しい",
    description:
      "Filter operator IS for a single-valued field (Environment, Time Range)",
  },
  "logs.viewOperatorIsOneOf": {
    message: "次のいずれか",
    description: "Filter operator IS for a field that takes several values",
  },
  "logs.viewOperatorIsNotOneOf": {
    message: "次のいずれでもない",
    description: "Filter operator IS_NOT",
  },
  "logs.viewValuesPlaceholder": {
    message: "値を選択",
    description:
      "Placeholder of a filter row's Values picker with nothing chosen",
  },
  "logs.viewNoResources": {
    message: "このプロジェクトにリソースはありません",
    description:
      "Resource picker of a project with no resource the view can read",
  },
  "logs.viewValueTypeHint": {
    message: "値を入力して Enter を押してください",
    description:
      "Placeholder of a typed filter value (instance, search text, status code, host, path)",
  },
  "logs.viewValueRemove": {
    message: "{value} を削除",
    description:
      "Accessible name of a typed filter value's remove button; value is the value",
  },
  "logs.viewValueSearch": {
    message: "検索するか値を入力",
    description:
      "Placeholder of the search box in a filter's value picker fed by the project's logs (Instance)",
  },
  "logs.viewValueAdd": {
    message: "「{value}」を追加",
    description:
      "Picker item that adds the typed text as a filter value; value is the typed text",
  },
  "logs.viewValuesLoading": {
    message: "プロジェクトのログを読み込み中…",
    description:
      "Value picker status while the project's logs are read for the field's values",
  },
  "logs.viewValuesUnavailable": {
    message:
      "プロジェクトのログを読み込めませんでした。値を入力して追加してください。",
    description:
      "Value picker status when the project's logs could not be read for the field's values",
  },
  "logs.viewValuesNoneSeen": {
    message:
      "過去 7 日間のプロジェクトのログにはありません。値を入力して追加してください。",
    description:
      "Value picker status when the project's logs hold no value of the field in the last 7 days",
  },
  "logs.viewFieldEnvironment": {
    message: "環境",
    description: "Filter field Environment in the Edit view modal",
  },
  "logs.viewFieldResource": {
    message: "リソース",
    description: "Filter field Resource in the Edit view modal",
  },
  "logs.viewFieldResourceType": {
    message: "リソースタイプ",
    description: "Filter field Resource Type in the Edit view modal",
  },
  "logs.viewFieldText": {
    message: "テキスト検索",
    description: "Filter field Text Search in the Edit view modal",
  },
  "logs.viewFieldTimeRange": {
    message: "時間範囲",
    description: "Filter field Time Range in the Edit view modal",
  },
  "logs.viewResourceTypeService": {
    message: "サービス",
    description: "Resource Type value Services",
  },
  "logs.viewResourceTypeCronJob": {
    message: "Cron ジョブ",
    description: "Resource Type value Cron jobs",
  },
  "logs.viewResourceTypePostgres": {
    message: "Postgres",
    description: "Resource Type value Postgres",
  },
  "logs.viewResourceTypeKeyValue": {
    message: "Key Value",
    description: "Resource Type value Key Value (the product name)",
  },
  "logs.lineLevel": {
    message: "レベル: {level}",
    description:
      "Log line level gutter (w3/m509): accessible name and hover text of the level icon leading a log row; {level} is the line's level word (info, warning, error, \u2026)",
  },
  "logs.showMore": {
    message: "もっと見る",
    description:
      "Log row (w3/m509): button after a long message cut short, showing the whole message",
  },
  "logs.showLess": {
    message: "折りたたむ",
    description:
      "Log row (w3/m509): button after a long message opened in full, cutting it short again",
  },
  "logs.copied": {
    message: "ログをコピーしました",
    description:
      "Log explorer (w3/m509): toast after Options > Copy logs wrote the lines shown to the clipboard",
  },
  "logs.copyFailed": {
    message: "ログをコピーできませんでした",
    description:
      "Log explorer (w3/m509): toast when the clipboard refused the copied logs",
  },
  "logs.copyLogs": {
    message: "ログをコピー",
    description:
      "Log explorer Options menu (w3/m509): item copying every line shown to the clipboard (Render's Copy logs)",
  },
  "logs.options": {
    message: "ログのオプション",
    description:
      "Log explorer (w3/m509): accessible name and tooltip of the Options (\u2026) menu button",
  },
  "logs.orderBy": {
    message: "並び順",
    description:
      "Log explorer Options menu (w3/m509): heading of the line order choice (Render's Order by)",
  },
  "logs.orderAscending": {
    message: "昇順",
    description:
      "Log explorer Options menu (w3/m509): order choice, oldest line first and the newest at the bottom",
  },
  "logs.orderDescending": {
    message: "降順",
    description:
      "Log explorer Options menu (w3/m509): order choice, newest line first at the top",
  },
  "logs.display": {
    message: "表示",
    description:
      "Log explorer Options menu (w3/m509): heading of the display options (Render's Display)",
  },
  "logs.expandEverything": {
    message: "すべて展開",
    description:
      "Log explorer Options menu (w3/m509): checkbox showing every long message in full (Render's Expand everything)",
  },
  "logs.maximize": {
    message: "最大化",
    description:
      "Log explorer (w3/m509): button and menu item expanding the log to fill the screen (Render's Maximize)",
  },
  "logs.minimize": {
    message: "全画面表示を終了",
    description:
      "Log explorer (w3/m509): button and menu item restoring the maximized log to its place in the page",
  },
  "logs.maximizeHint": {
    message: "最大化 (M)",
    description:
      "Log explorer (w3/m509): tooltip of the Maximize button, naming its M shortcut",
  },
  "logs.minimizeHint": {
    message: "全画面表示を終了 (M または Esc)",
    description:
      "Log explorer (w3/m509): tooltip of the button leaving full screen, naming the M and Escape keys that also leave it",
  },
  "logs.clearedTitle": {
    message: "ログをクリアしました",
    description:
      "Log explorer (w3/m509): title over a live tail whose lines the reader cleared with Shift+L",
  },
  "logs.clearedBody": {
    message: "新しい行は届きしだいここに表示されます。",
    description:
      "Log explorer (w3/m509): body under the cleared live tail's title",
  },
  "logs.clearedRestore": {
    message: "以前の行を表示",
    description:
      "Log explorer (w3/m509): button under a cleared live tail bringing back the lines Shift+L cleared",
  },
};

export default jaLogs;
