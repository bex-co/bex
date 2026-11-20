import type { TranslationEntry } from "@/i18n";

const zhLogs: Record<string, TranslationEntry> = {
  "logs.rangeLabel": {
    message: "日志历史范围",
    description: "Accessible label for the relative log-history range controls",
  },
  "logs.rangeHistoryNote": {
    message: "时间范围仅限制历史记录；实时模式会继续追加新日志。",
    description:
      "Explains how the bounded history window interacts with live tail",
  },
  "logs.retentionWindow_other": {
    message: "{plan} 套餐保留 {count} 天的日志。",
    description:
      "Log range picker hint: how many days the workspace plan keeps logs (w9/m389, from bex-api logRetention)",
  },
  "logs.retentionUpgrade_other": {
    message: "升级到 {plan} 可保留 {count} 天。",
    description:
      "Log range picker hint: the cheapest plan that keeps logs longer, and its days (w9/m389)",
  },
  "logs.typeLabel": {
    message: "日志类型",
    description: "Accessible label for the log-type filter dropdown",
  },
  "logs.typeAll": {
    message: "全部日志",
    description: "Log-type filter option: no type filter",
  },
  "logs.typeApplication": {
    message: "应用日志",
    description: "Log-type filter option: application (app) logs",
  },
  "logs.typeRequest": {
    message: "请求日志",
    description:
      "Log-type filter option: request logs (Traefik access lines, from the durable store)",
  },
  "logs.levelLabel": {
    message: "级别",
    description: "Accessible label for the log-level filter dropdown",
  },
  "logs.levelAll": {
    message: "全部级别",
    description: "Level filter option: no level filter",
  },
  "logs.methodLabel": {
    message: "方法",
    description: "Accessible label for the HTTP-method filter dropdown",
  },
  "logs.methodAll": {
    message: "全部方法",
    description: "Method filter option: no method filter",
  },
  "logs.statusCodeLabel": {
    message: "状态码",
    description: "Accessible label for the HTTP-status filter dropdown",
  },
  "logs.statusCodeAll": {
    message: "全部状态",
    description: "Status-code filter option: no status filter",
  },
  "logs.instanceLabel": {
    message: "实例",
    description: "Accessible label for the instance (replica) filter dropdown",
  },
  "logs.instanceAll": {
    message: "全部实例",
    description: "Instance filter option: no instance filter",
  },
  "logs.filterByInstance": {
    message: "按实例 {instance} 筛选日志",
    description:
      "Accessible label for the clickable instance slug on an application log line",
  },
  "logs.pathLabel": {
    message: "请求路径",
    description: "Accessible label for the request-path filter input",
  },
  "logs.filtersButton": {
    message: "筛选",
    description:
      "Trigger for the structured-filters popover (level/method/status/instance/path)",
  },
  "logs.chipRemove": {
    message: "移除{label}筛选",
    description:
      "Accessible label for an active-filter chip's clear button; {label} is the filter's field name",
  },
  "logs.pathPlaceholder": {
    message: "按路径筛选",
    description: "Placeholder for the request-path filter input",
  },
  "logs.hostLabel": {
    message: "请求主机",
    description:
      "Accessible label for the request-host filter input and its chip (Render's host log filter, w4/538)",
  },
  "logs.hostPlaceholder": {
    message: "按主机筛选",
    description: "Placeholder for the request-host filter input",
  },
  "logs.liveWindowClosed": {
    message: "实时日志已关闭——该自定义时间范围已经结束，不会再有新日志进入。",
    description:
      "Note when the selected custom time range ended before it was applied, so live tail is off",
  },
  "logs.liveUnsupported": {
    message:
      "实时日志已关闭——方法、状态码、路径和主机只筛选请求日志，应用日志无法按这些条件实时跟踪。",
    description:
      "Note when the chosen log type cannot carry an active filter, so live tail is off (a request filter on app logs)",
  },
  "logs.storeRequiredTitle": {
    message: "请求日志需要日志存储",
    description:
      "Empty-state title when a request/structured-filter query hits a deployment with no durable store (503)",
  },
  "logs.storeRequiredBody": {
    message:
      "请求日志以及级别/状态/方法/路径筛选需从持久化日志存储读取，而此环境未配置该存储。在已接入存储的环境中可用。",
    description: "Empty-state body for the store-unavailable (503) state",
  },
  "logs.requestPlanTitle": {
    message: "HTTP 请求日志需要 Pro 套餐",
    description:
      "Notice title in place of Request logs on a Hobby workspace (PLAN_FEATURE_REQUIRED, w8/m138)",
  },
  "logs.requestPlanBody": {
    message:
      "请求日志仅在 Pro 及更高套餐中提供。此工作区为 Hobby 套餐，因此只显示应用日志和构建日志。",
    description:
      "Notice body in place of Request logs on a Hobby workspace (w8/m138)",
  },
  "logs.searchPlaceholder": {
    message: "搜索日志",
    description: "Placeholder for the log search box (text filter)",
  },
  "logs.live": {
    message: "实时",
    description: "Label for the live-tail on/off toggle",
  },
  "logs.linesLabel": {
    message: "日志行",
    description:
      "Accessible name of the scrollable log list, which takes keyboard focus so arrow keys and Page Up/Down scroll it",
  },
  "logs.linesKeysHint": {
    message:
      "使用上下方向键在行间移动，按 Enter 仅显示该行的实例，按 Shift+F10 打开该行的菜单。",
    description:
      "Logs tab: screen-reader description of the focused log list's keys - arrows move the row cursor, Enter shows only that line's instance, Shift+F10 opens the line's menu (w3/m528)",
  },
  "logs.linesKeysShort": {
    message: "↑↓ 在行间移动 · Enter：仅显示此实例 · Shift+F10：菜单",
    description:
      "Logs tab: the keys of the focused log list, shown in its corner while it has keyboard focus - arrows move between lines, Enter shows only that line's instance, Shift+F10 opens the line's menu",
  },
  "logs.returnToBottom": {
    message: "返回底部",
    description:
      "Log list (w3/m528): accessible name and tooltip of Render's icon-only Return to Bottom button, shown while the reader is scrolled away from the newest line (ascending order)",
  },
  "logs.returnToTop": {
    message: "返回顶部",
    description:
      "Log list (w3/m528): accessible name and tooltip of Render's icon-only Return to Top button, shown while the reader is scrolled away from the newest line (descending order)",
  },
  "logs.newLogs": {
    message: "新日志",
    description:
      "Log list (w3/m528): Render's held-lines button while a live tail holds fewer than 10 new lines back from a reader scrolled away",
  },
  "logs.newLogsCount_other": {
    message: "{count}+ 条新日志",
    description:
      "Log list (w3/m528): Render's held-lines button for 10 or more held lines; count is the held count rounded up to a multiple of 5, at most 100, followed by a plus sign",
  },
  "logs.newLogsInLiveTail": {
    message: "实时日志中有新日志",
    description:
      "Log list (w3/m528): Render's button on a paused explorer when a new line arrived; clicking it turns Live on",
  },
  "logs.switchToLiveTail": {
    message: "切换到实时",
    description:
      "Log list (w3/m528): tooltip of the New logs in Live Tail button",
  },
  "logs.dismiss": {
    message: "关闭",
    description:
      "Log list (w3/m528): accessible name of the x that hides the New logs in Live Tail button",
  },
  "logs.lineMenu": {
    message: "日志行选项",
    description:
      "Log list (w3/m528): accessible name and tooltip of the row menu button (...) on a hovered log line",
  },
  "logs.copyLogLine": {
    message: "复制日志行",
    description:
      "Log row menu (w3/m528): copies the line without its time, as Render's Copy log line",
  },
  "logs.lineCopied": {
    message: "已复制！",
    description:
      "Log row menu (w3/m528): toast after Copy log line, Render's 'Copied!'",
  },
  "logs.lineCopyFailed": {
    message: "无法复制日志行",
    description:
      "Log row menu (w3/m528): toast when the clipboard refused the copied line",
  },
  "logs.viewInContext": {
    message: "查看上下文",
    description:
      "Log row menu (w3/m528): opens a new tab on the same explorer, unfiltered, 10 minutes either side of the line",
  },
  "logs.sshIntoInstance": {
    message: "SSH 连接到实例",
    description:
      "Log row menu (w3/m528): opens the service's Shell on the line's instance in a new tab",
  },
  "logs.sshInstanceNotRunning": {
    message: "只能 SSH 连接到正在运行的实例。这个实例不可用。",
    description:
      "Log row menu (w3/m528): Render's tooltip on a disabled SSH into instance item, for an instance that is no longer running",
  },
  "logs.linesKeysHintMenu": {
    message: "使用上下方向键在行间移动，按 Shift+F10 打开该行的菜单。",
    description:
      "Log list without an instance filter (w3/m528): screen-reader description of its keys - arrows move the row cursor, Shift+F10 opens the line's menu",
  },
  "logs.linesKeysShortMenu": {
    message: "↑↓ 在行间移动 · Shift+F10：菜单",
    description:
      "Log list without an instance filter (w3/m528): its keys, shown in its corner while it has keyboard focus",
  },
  "logs.themeSettings": {
    message: "主题设置",
    description:
      "Log explorer Options menu (w3/m528): Render's last item, a link to the Appearance section of account Settings",
  },
  "logs.endOfLog": {
    message: "日志结束",
    description:
      "Closing line after the last row of a log that will not grow (a finished deploy's), so a short log reads as complete",
  },
  "logs.loading": {
    message: "正在加载日志……",
    description: "Shown while the first historical page is loading",
  },
  "logs.loadingOlder": {
    message: "正在加载更早的日志……",
    description:
      "Shown at the top of the pane while a page-back request is in flight",
  },
  "logs.loadOlder": {
    message: "加载更早的日志",
    description:
      "Button in the truncation notice that fetches the next older page — the keyboard/underfilled-pane path to what scrolling to the top does",
  },
  "logs.truncatedNotice": {
    message: "此时间范围仅加载了一部分。加载更早的日志以继续。",
    description:
      "Banner when hasMore is true: more of the selected range remains unloaded, however many older pages are already shown, so it names no line count (w4/280)",
  },
  "logs.partialSearchNotice": {
    message: "目前只搜索了此范围内最新的一部分——加载更早的日志以继续搜索。",
    description:
      "Banner when a time-boxed search returned less than a page but hasMore is true: the older part of the range is unsearched, not empty (w4/m140)",
  },
  "logs.timeoutTitle": {
    message: "此搜索耗时过长",
    description:
      "State title when the logs query ran out of the server's time budget (QUERY_TIMEOUT, w4/m140)",
  },
  "logs.timeoutBody": {
    message: "请缩短时间范围，或使用更具体的搜索词。",
    description: "State body under logs.timeoutTitle",
  },
  "logs.streaming": {
    message: "实时——正在接收新日志",
    description: "Status under the log list when the SSE tail is connected",
  },
  "logs.paused": {
    message: "已暂停实时日志",
    description: "Status under the log list when live tail is toggled off",
  },
  "logs.disconnected": {
    message: "实时日志已断开——正在重连……",
    description: "Banner when the SSE stream drops",
  },
  "logs.emptyTitle": {
    message: "此时间范围内暂无日志",
    description:
      "Empty-state title when the selected (always-bounded) history range holds no lines. Never claims the service has never logged — the range can't establish that (w6/m111)",
  },
  "logs.emptyBody": {
    message: "所选时间范围内没有日志。",
    description:
      "Empty-state body with no filters applied — honest about the bounded window, not the service's whole history (w6/m111)",
  },
  "logs.emptyFilteredBody": {
    message: "没有匹配这些筛选条件的日志。",
    description:
      "Empty-state body when a type/text/structured filter yields nothing",
  },
  "logs.emptyFilteredTitle": {
    message: "没有匹配的日志",
    description:
      "Empty-state title when a type/text/structured filter yields nothing",
  },
  "logs.errorTitle": {
    message: "无法加载日志",
    description: "Error-state title when the logs query fails",
  },
  "logs.errorBody": {
    message: "此时间范围的请求失败。请重试，或选择更短的时间范围。",
    description:
      "Logs tab: body when the log history read failed, under logs.errorTitle; the server's own reason follows as a detail line",
  },
  "logs.errorBodyLive": {
    message: "新日志到达后仍会显示在这里。重试即可加载此时间范围的历史记录。",
    description:
      "Logs tab: body when the history read failed while the live tail still streams \u2014 new lines will still land in the panel",
  },
  "logs.filtersDone": {
    message: "完成",
    description: "Logs tab: button closing the Filters panel",
  },
  "logs.emptyBodyRange": {
    message: "请选择更长的时间范围，或开启实时以查看新日志。",
    description:
      "Logs tab: body when the chosen range holds no lines, under logs.emptyTitle \u2014 what to try next",
  },
  "logs.turnOnLive": {
    message: "开启实时",
    description:
      "Logs tab: button in the empty state that switches the live tail on",
  },
  "logs.clearFilters": {
    message: "清除筛选",
    description:
      "Logs tab: button in the no-match state that clears the search and every filter",
  },
  "logs.errorTitleWithReason": {
    message: "无法加载日志：{reason}",
    description:
      "Inline banner over loaded log lines when a later read was refused; reason is the server's refusal",
  },
  "logs.observabilityViewEyebrow": {
    message: "可观测性视图",
    description:
      "Eyebrow over a project observability view's name (Render's OBSERVABILITY VIEW, w3/m505)",
  },
  "logs.applicationLogsTitle": {
    message: "应用日志",
    description:
      "Document title part for a project's Application Logs view page (w3/m505)",
  },
  "logs.requestLogsTitle": {
    message: "请求日志",
    description:
      "Document title part for a project's Request Logs view page (w3/m505)",
  },
  "logs.projectScopeEmptyTitle": {
    message: "此视图中暂无可读取的内容",
    description:
      "Project log view state: the view's scope holds no resource to read (w3/m505)",
  },
  "logs.projectScopeEmptyBody": {
    message:
      "此视图读取项目的服务、定时任务、Postgres 数据库和 Key Value 实例，但目前其中还没有任何资源。",
    description:
      "Project Application Logs view with no service, cron job, Postgres or Key Value in it (w3/m505)",
  },
  "logs.projectRequestScopeEmptyBody": {
    message: "请求日志读取项目的 Web 服务，但此视图中目前还没有。",
    description:
      "Project Request Logs view with no web service in it (w3/m505)",
  },
  "logs.projectRoleLimitTitle": {
    message: "你无法查看日志",
    description:
      "Project log view state: the caller's workspace role reads no logs, so the view reads nothing (w3/078)",
  },
  "logs.projectRoleLimitBody": {
    message:
      "查看此工作区的日志需要贡献者或更高的角色。请联系工作区管理员更改你的角色。",
    description:
      "Project log view state body: logs need the Contributor role or higher, as on Render (w3/078)",
  },
  "logs.projectEmptyTitle": {
    message: "此时间范围内没有日志",
    description:
      "Project log view state: its resources logged nothing in the selected range (w3/m505)",
  },
  "logs.projectEmptyBody": {
    message: "此视图中的资源在所选时间范围内没有记录任何日志。",
    description:
      "Project log view state body: no line in the range from any of its resources (w3/m505)",
  },
  "logs.viewNotFoundTitle": {
    message: "未找到视图",
    description:
      "Project observability page whose savedSearchId names no view of this project (w3/m505)",
  },
  "logs.viewNotFoundBody": {
    message: "此项目在该链接下没有视图，它可能已被删除。",
    description: "Body of the view-not-found state (w3/m505)",
  },
  "logs.viewOpen": {
    message: "打开 {name}",
    description:
      "Link from the view-not-found state to the project's default view of the same type; name is the view's name",
  },
  "logs.viewLoadErrorTitle": {
    message: "无法加载此视图",
    description:
      "Project observability page whose saved searches could not be read (w3/m505)",
  },
  "logs.viewLoadErrorBody": {
    message: "无法读取项目的视图。请刷新页面重试。",
    description: "Body of the view load failure (w3/m505)",
  },
  "logs.newView": {
    message: "新建视图",
    description:
      "Project sidebar button that opens the TYPE menu for a new observability view (w3/m507, Render's '+ New View')",
  },
  "logs.newViewTypeHeading": {
    message: "类型",
    description:
      "Heading of the New View menu listing view types (Render's TYPE)",
  },
  "logs.editView": {
    message: "编辑视图",
    description:
      "Observability view header button that opens the Edit view modal (w3/m507)",
  },
  "logs.duplicateView": {
    message: "复制",
    description:
      "Observability view header button that copies the view (w3/m507)",
  },
  "logs.deleteView": {
    message: "删除视图",
    description:
      "Accessible name and confirm label of the observability view's trash button (w3/m507)",
  },
  "logs.deleteViewTitle": {
    message: "删除 {name}？",
    description:
      "Title of the confirmation before deleting an observability view; name is the view's name",
  },
  "logs.deleteViewBody": {
    message: "该视图会对项目中的所有人删除。项目的资源及其日志不受影响。",
    description:
      "Body of the delete-view confirmation: views are shared by the whole project (w3/m507)",
  },
  "logs.viewDeleted": {
    message: "已删除 {name}",
    description:
      "Toast after an observability view was deleted; name is the view's name",
  },
  "logs.deleteViewFailed": {
    message: "无法删除视图",
    description:
      "Toast when deleting an observability view failed without a server reason",
  },
  "logs.viewDuplicated": {
    message: "已创建 {name}",
    description: "Toast after a view was duplicated; name is the copy's name",
  },
  "logs.duplicateViewFailed": {
    message: "无法复制视图",
    description:
      "Toast when duplicating an observability view failed without a server reason",
  },
  "logs.editViewTitle": {
    message: "编辑视图",
    description: "Title of Render's Edit view modal (w3/m507)",
  },
  "logs.newViewTitle": {
    message: "新建视图",
    description: "Title of the view modal when New View opened it (w3/m507)",
  },
  "logs.viewNameLabel": {
    message: "名称",
    description: "Label of the view name field in the Edit view modal",
  },
  "logs.viewNameRequired": {
    message: "请为视图命名。",
    description: "Inline refusal of a blank view name",
  },
  "logs.viewFiltersLabel": {
    message: "筛选条件",
    description: "Heading of the filter rows in the Edit view modal",
  },
  "logs.viewAddFilter": {
    message: "添加筛选条件",
    description: "Accessible name of the Edit view modal's add (+) button",
  },
  "logs.viewNoFilters": {
    message: "没有筛选条件：该视图读取项目中的所有资源。",
    description:
      "Edit view modal with no filter rows: the view reads the whole project",
  },
  "logs.viewSharedNotice": {
    message: "编辑筛选条件会为项目中的所有人更新该视图。",
    description:
      "Render's notice under the Edit view filters: a view is shared by the whole project",
  },
  "logs.viewSaveFailed": {
    message: "无法保存视图。请重试。",
    description:
      "Edit view modal error when saving failed without a server reason",
  },
  "logs.viewClose": {
    message: "关闭",
    description:
      "Edit view modal button that discards the changes (Render's Close)",
  },
  "logs.viewDone": {
    message: "完成",
    description: "Edit view modal button that saves the view (Render's Done)",
  },
  "logs.viewFilterField": {
    message: "字段",
    description: "Accessible name of a filter row's Field picker",
  },
  "logs.viewFilterOperator": {
    message: "运算符",
    description: "Accessible name of a filter row's Operator picker",
  },
  "logs.viewFilterValuesFor": {
    message: "{field} 的值",
    description:
      "Accessible name of a filter row's Values picker; field is the row's field",
  },
  "logs.viewValuesRequired": {
    message: "请至少选择一个值。",
    description: "Inline refusal of a filter row with no value",
  },
  "logs.viewRemoveFilter": {
    message: "移除 {field} 筛选条件",
    description:
      "Accessible name of a filter row's trash button; field is the row's field",
  },
  "logs.viewOperatorIs": {
    message: "是",
    description:
      "Filter operator IS for a single-valued field (Environment, Time Range)",
  },
  "logs.viewOperatorIsOneOf": {
    message: "属于",
    description: "Filter operator IS for a field that takes several values",
  },
  "logs.viewOperatorIsNotOneOf": {
    message: "不属于",
    description: "Filter operator IS_NOT",
  },
  "logs.viewOperatorContains": {
    message: "包含",
    description: "Filter operator CONTAINS, Text Search's only operator",
  },
  "logs.viewValuesPlaceholder": {
    message: "选择值",
    description:
      "Placeholder of a filter row's Values picker with nothing chosen",
  },
  "logs.viewNoResources": {
    message: "此项目中没有资源",
    description:
      "Resource picker of a project with no resource the view can read",
  },
  "logs.viewValueTypeHint": {
    message: "输入一个值，然后按 Enter",
    description: "Placeholder of a typed filter value (Text Search's terms)",
  },
  "logs.viewValueRemove": {
    message: "移除 {value}",
    description:
      "Accessible name of a typed filter value's remove button; value is the value",
  },
  "logs.viewValueSearch": {
    message: "搜索或输入一个值",
    description:
      "Placeholder of the search box in a filter's value picker fed by the project's logs (Level, Instance, Method, Status Code, Host, Path)",
  },
  "logs.viewValueAdd": {
    message: "添加“{value}”",
    description:
      "Picker item that adds the typed text as a filter value; value is the typed text",
  },
  "logs.viewValuesLoading": {
    message: "正在读取项目日志…",
    description:
      "Value picker status while the project's logs are read for the field's values",
  },
  "logs.viewValuesUnavailable": {
    message: "无法读取项目日志。输入一个值即可添加。",
    description:
      "Value picker status when the project's logs could not be read for the field's values",
  },
  "logs.viewValuesNoneSeen": {
    message: "过去 7 天的项目日志中没有。输入一个值即可添加。",
    description:
      "Value picker status when the project's logs hold no value of the field in the last 7 days",
  },
  "logs.viewFieldEnvironment": {
    message: "环境",
    description: "Filter field Environment in the Edit view modal",
  },
  "logs.viewFieldResource": {
    message: "资源",
    description: "Filter field Resource in the Edit view modal",
  },
  "logs.viewFieldResourceType": {
    message: "资源类型",
    description: "Filter field Resource Type in the Edit view modal",
  },
  "logs.viewFieldText": {
    message: "文本搜索",
    description: "Filter field Text Search in the Edit view modal",
  },
  "logs.viewFieldTimeRange": {
    message: "时间范围",
    description: "Filter field Time Range in the Edit view modal",
  },
  "logs.viewResourceTypeService": {
    message: "服务",
    description: "Resource Type value Services",
  },
  "logs.viewResourceTypeCronJob": {
    message: "Cron 任务",
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
    message: "级别：{level}",
    description:
      "Log line level gutter (w3/m509): accessible name and hover text of the level icon leading a log row; {level} is the line's level word (info, warning, error, \u2026)",
  },
  "logs.copied": {
    message: "已复制日志",
    description:
      "Log explorer (w3/m509): toast after Options > Copy logs wrote the lines shown to the clipboard",
  },
  "logs.copyFailed": {
    message: "无法复制日志",
    description:
      "Log explorer (w3/m509): toast when the clipboard refused the copied logs",
  },
  "logs.copyLogs": {
    message: "复制日志",
    description:
      "Log explorer Options menu (w3/m509): item copying every line shown to the clipboard (Render's Copy logs)",
  },
  "logs.options": {
    message: "日志选项",
    description:
      "Log explorer (w3/m509): accessible name and tooltip of the Options (\u2026) menu button",
  },
  "logs.orderBy": {
    message: "排序",
    description:
      "Log explorer Options menu (w3/m509): heading of the line order choice (Render's Order by)",
  },
  "logs.orderAscending": {
    message: "升序",
    description:
      "Log explorer Options menu (w3/m509): order choice, oldest line first and the newest at the bottom",
  },
  "logs.orderDescending": {
    message: "降序",
    description:
      "Log explorer Options menu (w3/m509): order choice, newest line first at the top",
  },
  "logs.display": {
    message: "显示：",
    description:
      "Log explorer Options menu (w3/088): the Display submenu's label, before the chosen display in bold (Render's `Display: Expand everything`)",
  },
  "logs.displaySettings": {
    message: "显示设置",
    description:
      "Log explorer Options menu (w3/088): heading of the Display choices (Render's Display settings)",
  },
  "logs.expandEverything": {
    message: "全部展开",
    description:
      "Log explorer Options ▸ Display (w3/088): every log line shown whole, the default (Render's Expand everything)",
  },
  "logs.expandErrors": {
    message: "展开错误",
    description:
      "Log explorer Options ▸ Display (w3/088): error, critical, alert and emergency lines shown whole, every other line on one line (Render's Expand errors)",
  },
  "logs.collapseEverything": {
    message: "全部收起",
    description:
      "Log explorer Options ▸ Display (w3/088): every log line on one line, cut off with an ellipsis (Render's Collapse everything)",
  },
  "logs.maximize": {
    message: "最大化",
    description:
      "Log explorer (w3/m509): button and menu item expanding the log to fill the screen (Render's Maximize)",
  },
  "logs.minimize": {
    message: "退出全屏",
    description:
      "Log explorer (w3/m509): button and menu item restoring the maximized log to its place in the page",
  },
  "logs.maximizeHint": {
    message: "最大化 (M)",
    description:
      "Log explorer (w3/m509): tooltip of the Maximize button, naming its M shortcut",
  },
  "logs.minimizeHint": {
    message: "退出全屏 (M 或 Esc)",
    description:
      "Log explorer (w3/m509): tooltip of the button leaving full screen, naming the M and Escape keys that also leave it",
  },
  "logs.clearedTitle": {
    message: "日志已清空",
    description:
      "Log explorer (w3/m509): title over a live tail whose lines the reader cleared with Shift+L",
  },
  "logs.clearedBody": {
    message: "新的日志行到达后会显示在这里。",
    description:
      "Log explorer (w3/m509): body under the cleared live tail's title",
  },
  "logs.clearedRestore": {
    message: "显示之前的日志行",
    description:
      "Log explorer (w3/m509): button under a cleared live tail bringing back the lines Shift+L cleared",
  },
};

export default zhLogs;
