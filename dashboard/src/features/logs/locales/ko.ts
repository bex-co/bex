import type { TranslationEntry } from "@/i18n";

const koLogs: Record<string, TranslationEntry> = {
  "logs.rangeLabel": {
    message: "로그 기록 범위",
    description: "Accessible label for the relative log-history range controls",
  },
  "logs.rangeLiveTail": {
    message: "실시간 보기",
    description:
      "Log explorers (w3/m537): the first entry of the time range picker, Render's Live tail; picking it streams new lines, and every other range is a snapshot",
  },
  "logs.retentionWindow_other": {
    message: "{plan} 요금제에서는 로그를 {count}일 동안 보관합니다.",
    description:
      "Log range picker hint: how many days the workspace plan keeps logs (w9/m389, from bex-api logRetention)",
  },
  "logs.retentionUpgrade_other": {
    message:
      "{plan} 요금제로 업그레이드하면 {count}일 동안 보관할 수 있습니다.",
    description:
      "Log range picker hint: the cheapest plan that keeps logs longer, and its days (w9/m389)",
  },
  "logs.typeLabel": {
    message: "로그 유형",
    description: "Accessible label for the log-type filter dropdown",
  },
  "logs.typeAll": {
    message: "모든 로그",
    description: "Log-type filter option: no type filter",
  },
  "logs.typeApplication": {
    message: "애플리케이션 로그",
    description: "Log-type filter option: application (app) logs",
  },
  "logs.typeRequest": {
    message: "요청 로그",
    description:
      "Log-type filter option: request logs (Traefik access lines, from the durable store)",
  },
  "logs.levelLabel": {
    message: "수준",
    description: "Accessible label for the log-level filter dropdown",
  },
  "logs.levelAll": {
    message: "모든 수준",
    description: "Level filter option: no level filter",
  },
  "logs.methodLabel": {
    message: "메서드",
    description: "Accessible label for the HTTP-method filter dropdown",
  },
  "logs.methodAll": {
    message: "모든 메서드",
    description: "Method filter option: no method filter",
  },
  "logs.statusCodeLabel": {
    message: "상태 코드",
    description: "Accessible label for the HTTP-status filter dropdown",
  },
  "logs.statusCodeAll": {
    message: "모든 상태",
    description: "Status-code filter option: no status filter",
  },
  "logs.instanceLabel": {
    message: "인스턴스",
    description: "Accessible label for the instance (replica) filter dropdown",
  },
  "logs.instanceAll": {
    message: "모든 인스턴스",
    description: "Instance filter option: no instance filter",
  },
  "logs.filterByInstance": {
    message: "{instance} 인스턴스로 로그 필터링",
    description:
      "Accessible label for the clickable instance slug on an application log line",
  },
  "logs.pathLabel": {
    message: "요청 경로",
    description: "Accessible label for the request-path filter input",
  },
  "logs.filtersButton": {
    message: "필터",
    description:
      "Trigger for the structured-filters popover (level/method/status/instance/path)",
  },
  "logs.chipRemove": {
    message: "{label} 필터 제거",
    description:
      "Accessible label for an active-filter chip's clear button; {label} is the filter's field name",
  },
  "logs.pathPlaceholder": {
    message: "경로로 필터링",
    description: "Placeholder for the request-path filter input",
  },
  "logs.hostLabel": {
    message: "요청 호스트",
    description:
      "Accessible label for the request-host filter input and its chip (Render's host log filter, w4/538)",
  },
  "logs.hostPlaceholder": {
    message: "호스트로 필터링",
    description: "Placeholder for the request-host filter input",
  },
  "logs.liveUnsupported": {
    message:
      "실시간 보기를 사용할 수 없습니다. 메서드, 상태, 경로, 호스트는 요청 로그를 필터링하므로 이 조건으로는 애플리케이션 로그를 실시간으로 볼 수 없습니다.",
    description:
      "Note when the chosen log type cannot carry an active filter, so live tail is off (a request filter on app logs)",
  },
  "logs.storeRequiredTitle": {
    message: "요청 로그에는 로그 저장소가 필요합니다",
    description:
      "Empty-state title when a request/structured-filter query hits a deployment with no durable store (503)",
  },
  "logs.storeRequiredBody": {
    message:
      "요청 로그와 수준/상태/메서드/경로 필터는 영구 로그 저장소에서 읽는데, 여기에는 구성되어 있지 않습니다. 저장소가 연결된 환경에서 사용할 수 있습니다.",
    description: "Empty-state body for the store-unavailable (503) state",
  },
  "logs.requestPlanTitle": {
    message: "HTTP 요청 로그는 Pro 요금제가 필요합니다",
    description:
      "Notice title in place of Request logs on a Hobby workspace (PLAN_FEATURE_REQUIRED, w8/m138)",
  },
  "logs.requestPlanBody": {
    message:
      "요청 로그는 Pro 요금제 이상에서 사용할 수 있습니다. 이 워크스페이스는 Hobby 요금제이므로 애플리케이션 로그와 빌드 로그만 표시됩니다.",
    description:
      "Notice body in place of Request logs on a Hobby workspace (w8/m138)",
  },
  "logs.searchPlaceholder": {
    message: "로그 검색",
    description: "Placeholder for the log search box (text filter)",
  },
  "logs.linesLabel": {
    message: "로그 줄",
    description:
      "Accessible name of the scrollable log list, which takes keyboard focus so arrow keys and Page Up/Down scroll it",
  },
  "logs.linesKeysHint": {
    message:
      "위아래 화살표 키로 줄 사이를 이동하고, Enter로 해당 줄의 인스턴스만 표시하며, Shift+F10으로 해당 줄의 메뉴를 엽니다.",
    description:
      "Logs tab: screen-reader description of the focused log list's keys - arrows move the row cursor, Enter shows only that line's instance, Shift+F10 opens the line's menu (w3/m528)",
  },
  "logs.linesKeysShort": {
    message: "↑↓ 줄 이동 · Enter: 이 인스턴스만 보기 · Shift+F10: 메뉴",
    description:
      "Logs tab: the keys of the focused log list, shown in its corner while it has keyboard focus - arrows move between lines, Enter shows only that line's instance, Shift+F10 opens the line's menu",
  },
  "logs.returnToBottom": {
    message: "맨 아래로 돌아가기",
    description:
      "Log list (w3/m528): accessible name and tooltip of Render's icon-only Return to Bottom button, shown while the reader is scrolled away from the newest line (ascending order)",
  },
  "logs.returnToTop": {
    message: "맨 위로 돌아가기",
    description:
      "Log list (w3/m528): accessible name and tooltip of Render's icon-only Return to Top button, shown while the reader is scrolled away from the newest line (descending order)",
  },
  "logs.newLogs": {
    message: "새 로그",
    description:
      "Log list (w3/m528): Render's held-lines button while a live tail holds fewer than 10 new lines back from a reader scrolled away",
  },
  "logs.newLogsCount_other": {
    message: "새 로그 {count}+개",
    description:
      "Log list (w3/m528): Render's held-lines button for 10 or more held lines; count is the held count rounded up to a multiple of 5, at most 100, followed by a plus sign",
  },
  "logs.newLogsInLiveTail": {
    message: "실시간 보기에 새 로그",
    description:
      "Log list (w3/m528): Render's button on a preset range when a new line arrived; clicking it picks Live tail (w3/m537)",
  },
  "logs.switchToLiveTail": {
    message: "실시간 보기로 전환",
    description:
      "Log list (w3/m528): tooltip of the New logs in Live Tail button",
  },
  "logs.dismiss": {
    message: "닫기",
    description:
      "Log list (w3/m528): accessible name of the x that hides the New logs in Live Tail button",
  },
  "logs.lineMenu": {
    message: "로그 줄 옵션",
    description:
      "Log list (w3/m528): accessible name and tooltip of the row menu button (...) on a hovered log line",
  },
  "logs.copyLogLine": {
    message: "로그 줄 복사",
    description:
      "Log row menu (w3/m528): copies the line without its time, as Render's Copy log line",
  },
  "logs.lineCopied": {
    message: "복사했습니다!",
    description:
      "Log row menu (w3/m528): toast after Copy log line, Render's 'Copied!'",
  },
  "logs.lineCopyFailed": {
    message: "로그 줄을 복사하지 못했습니다",
    description:
      "Log row menu (w3/m528): toast when the clipboard refused the copied line",
  },
  "logs.viewInContext": {
    message: "앞뒤 로그 보기",
    description:
      "Log row menu (w3/m528): opens a new tab on the same explorer, unfiltered, 10 minutes either side of the line",
  },
  "logs.sshIntoInstance": {
    message: "인스턴스에 SSH로 접속",
    description:
      "Log row menu (w3/m528): opens the service's Shell on the line's instance in a new tab",
  },
  "logs.sshInstanceNotRunning": {
    message:
      "실행 중인 인스턴스에만 SSH로 접속할 수 있습니다. 이 인스턴스는 사용할 수 없습니다.",
    description:
      "Log row menu (w3/m528): Render's tooltip on a disabled SSH into instance item, for an instance that is no longer running",
  },
  "logs.linesKeysHintMenu": {
    message:
      "위아래 화살표 키로 줄 사이를 이동하고, Shift+F10으로 해당 줄의 메뉴를 엽니다.",
    description:
      "Log list without an instance filter (w3/m528): screen-reader description of its keys - arrows move the row cursor, Shift+F10 opens the line's menu",
  },
  "logs.linesKeysShortMenu": {
    message: "↑↓ 줄 이동 · Shift+F10: 메뉴",
    description:
      "Log list without an instance filter (w3/m528): its keys, shown in its corner while it has keyboard focus",
  },
  "logs.themeSettings": {
    message: "테마 설정",
    description:
      "Log explorer Options menu (w3/m528): Render's last item, a link to the Appearance section of account Settings",
  },
  "logs.endOfLog": {
    message: "로그 끝",
    description:
      "Closing line after the last row of a log that will not grow (a finished deploy's), so a short log reads as complete",
  },
  "logs.loading": {
    message: "로그를 불러오는 중…",
    description: "Shown while the first historical page is loading",
  },
  "logs.loadingOlder": {
    message: "이전 로그를 불러오는 중…",
    description:
      "Shown at the top of the pane while a page-back request is in flight",
  },
  "logs.loadOlder": {
    message: "이전 로그 불러오기",
    description:
      "Button in the truncation notice that fetches the next older page — the keyboard/underfilled-pane path to what scrolling to the top does",
  },
  "logs.truncatedNotice": {
    message:
      "이 시간 범위의 일부만 불러왔습니다. 계속하려면 이전 로그를 불러오세요.",
    description:
      "Banner when hasMore is true: more of the selected range remains unloaded, however many older pages are already shown, so it names no line count (w4/280)",
  },
  "logs.partialSearchNotice": {
    message:
      "지금까지 이 범위의 최신 부분만 검색했습니다. 계속 검색하려면 이전 로그를 불러오세요.",
    description:
      "Banner when a time-boxed search returned less than a page but hasMore is true: the older part of the range is unsearched, not empty (w4/m140)",
  },
  "logs.timeoutTitle": {
    message: "검색 시간이 너무 오래 걸렸습니다",
    description:
      "State title when the logs query ran out of the server's time budget (QUERY_TIMEOUT, w4/m140)",
  },
  "logs.timeoutBody": {
    message: "시간 범위를 줄이거나 검색어를 더 구체적으로 입력해 보세요.",
    description: "State body under logs.timeoutTitle",
  },
  "logs.streaming": {
    message: "실시간 — 새 줄을 스트리밍하는 중",
    description: "Status under the log list when the SSE tail is connected",
  },
  "logs.disconnected": {
    message: "실시간 보기 연결이 끊어졌습니다. 다시 연결하는 중…",
    description: "Banner when the SSE stream drops",
  },
  "logs.emptyTitle": {
    message: "이 시간 범위에 로그가 없습니다",
    description:
      "Empty-state title when the selected (always-bounded) history range holds no lines. Never claims the service has never logged — the range can't establish that (w6/m111)",
  },
  "logs.emptyBody": {
    message: "이 시간 범위에 해당하는 로그 줄이 없습니다.",
    description:
      "Empty-state body with no filters applied — honest about the bounded window, not the service's whole history (w6/m111)",
  },
  "logs.emptyFilteredBody": {
    message: "이 필터와 일치하는 로그가 없습니다.",
    description:
      "Empty-state body when a type/text/structured filter yields nothing",
  },
  "logs.emptyFilteredTitle": {
    message: "일치하는 로그가 없습니다",
    description:
      "Empty-state title when a type/text/structured filter yields nothing",
  },
  "logs.errorTitle": {
    message: "로그를 불러오지 못했습니다",
    description: "Error-state title when the logs query fails",
  },
  "logs.errorBody": {
    message:
      "이 시간 범위에 대한 요청이 실패했습니다. 다시 시도하거나 더 짧은 범위를 선택하세요.",
    description:
      "Logs tab: body when the log history read failed, under logs.errorTitle; the server's own reason follows as a detail line",
  },
  "logs.errorBodyLive": {
    message:
      "새 줄은 도착하는 대로 여기에 계속 표시됩니다. 다시 시도하면 이 범위의 기록을 불러옵니다.",
    description:
      "Logs tab: body when the history read failed while the live tail still streams \u2014 new lines will still land in the panel",
  },
  "logs.filtersDone": {
    message: "완료",
    description: "Logs tab: button closing the Filters panel",
  },
  "logs.emptyBodyRange": {
    message:
      "더 긴 시간 범위를 선택하거나 실시간 보기로 전환해 새 줄을 확인하세요.",
    description:
      "Logs tab: body when the chosen range holds no lines, under logs.emptyTitle \u2014 what to try next",
  },
  "logs.clearFilters": {
    message: "필터 지우기",
    description:
      "Logs tab: button in the no-match state that clears the search and every filter",
  },
  "logs.errorTitleWithReason": {
    message: "로그를 불러오지 못했습니다: {reason}",
    description:
      "Inline banner over loaded log lines when a later read was refused; reason is the server's refusal",
  },
  "logs.observabilityViewEyebrow": {
    message: "관측성 보기",
    description:
      "Eyebrow over a project observability view's name (Render's OBSERVABILITY VIEW, w3/m505)",
  },
  "logs.applicationLogsTitle": {
    message: "애플리케이션 로그",
    description:
      "Document title part for a project's Application Logs view page (w3/m505)",
  },
  "logs.requestLogsTitle": {
    message: "요청 로그",
    description:
      "Document title part for a project's Request Logs view page (w3/m505)",
  },
  "logs.projectScopeEmptyTitle": {
    message: "이 보기에는 아직 읽을 항목이 없습니다",
    description:
      "Project log view state: the view's scope holds no resource to read (w3/m505)",
  },
  "logs.projectScopeEmptyBody": {
    message:
      "이 보기는 프로젝트의 서비스, 크론 작업, Postgres 데이터베이스, Key Value 인스턴스를 읽지만 아직 아무것도 없습니다.",
    description:
      "Project Application Logs view with no service, cron job, Postgres or Key Value in it (w3/m505)",
  },
  "logs.projectRequestScopeEmptyBody": {
    message:
      "요청 로그는 프로젝트의 웹 서비스를 읽지만 이 보기에는 아직 없습니다.",
    description:
      "Project Request Logs view with no web service in it (w3/m505)",
  },
  "logs.projectEmptyTitle": {
    message: "이 기간에는 로그가 없습니다",
    description:
      "Project log view state: its resources logged nothing in the selected range (w3/m505)",
  },
  "logs.projectEmptyBody": {
    message: "이 보기의 리소스가 선택한 기간에 아무것도 기록하지 않았습니다.",
    description:
      "Project log view state body: no line in the range from any of its resources (w3/m505)",
  },
  "logs.viewNotFoundTitle": {
    message: "보기를 찾을 수 없습니다",
    description:
      "Project observability page whose savedSearchId names no view of this project (w3/m505)",
  },
  "logs.viewNotFoundBody": {
    message:
      "이 프로젝트에는 이 링크에 해당하는 보기가 없습니다. 삭제되었을 수 있습니다.",
    description: "Body of the view-not-found state (w3/m505)",
  },
  "logs.viewOpen": {
    message: "{name} 열기",
    description:
      "Link from the view-not-found state to the project's default view of the same type; name is the view's name",
  },
  "logs.viewLoadErrorTitle": {
    message: "이 보기를 불러올 수 없습니다",
    description:
      "Project observability page whose saved searches could not be read (w3/m505)",
  },
  "logs.viewLoadErrorBody": {
    message:
      "프로젝트의 보기를 읽지 못했습니다. 페이지를 새로고침하여 다시 시도하세요.",
    description: "Body of the view load failure (w3/m505)",
  },
  "logs.newView": {
    message: "새 보기",
    description:
      "Project sidebar button that opens the TYPE menu for a new observability view (w3/m507, Render's '+ New View')",
  },
  "logs.newViewTypeHeading": {
    message: "유형",
    description:
      "Heading of the New View menu listing view types (Render's TYPE)",
  },
  "logs.editView": {
    message: "보기 편집",
    description:
      "Observability view header button that opens the Edit view modal (w3/m507)",
  },
  "logs.duplicateView": {
    message: "복제",
    description:
      "Observability view header button that copies the view (w3/m507)",
  },
  "logs.deleteView": {
    message: "보기 삭제",
    description:
      "Accessible name and confirm label of the observability view's trash button (w3/m507)",
  },
  "logs.deleteViewTitle": {
    message: "{name}을(를) 삭제할까요?",
    description:
      "Title of the confirmation before deleting an observability view; name is the view's name",
  },
  "logs.deleteViewBody": {
    message:
      "프로젝트의 모든 사용자에게서 이 보기가 삭제됩니다. 프로젝트의 리소스와 로그는 영향을 받지 않습니다.",
    description:
      "Body of the delete-view confirmation: views are shared by the whole project (w3/m507)",
  },
  "logs.viewDeleted": {
    message: "{name}을(를) 삭제했습니다",
    description:
      "Toast after an observability view was deleted; name is the view's name",
  },
  "logs.deleteViewFailed": {
    message: "보기를 삭제할 수 없습니다",
    description:
      "Toast when deleting an observability view failed without a server reason",
  },
  "logs.viewDuplicated": {
    message: "{name}을(를) 만들었습니다",
    description: "Toast after a view was duplicated; name is the copy's name",
  },
  "logs.duplicateViewFailed": {
    message: "보기를 복제할 수 없습니다",
    description:
      "Toast when duplicating an observability view failed without a server reason",
  },
  "logs.editViewTitle": {
    message: "보기 편집",
    description: "Title of Render's Edit view modal (w3/m507)",
  },
  "logs.newViewTitle": {
    message: "새 보기",
    description: "Title of the view modal when New View opened it (w3/m507)",
  },
  "logs.viewNameLabel": {
    message: "이름",
    description: "Label of the view name field in the Edit view modal",
  },
  "logs.viewNameRequired": {
    message: "보기 이름을 입력하세요.",
    description: "Inline refusal of a blank view name",
  },
  "logs.viewFiltersLabel": {
    message: "필터",
    description: "Heading of the filter rows in the Edit view modal",
  },
  "logs.viewAddFilter": {
    message: "필터 추가",
    description: "Accessible name of the Edit view modal's add (+) button",
  },
  "logs.viewNoFilters": {
    message: "필터 없음: 이 보기는 프로젝트의 모든 리소스를 읽습니다.",
    description:
      "Edit view modal with no filter rows: the view reads the whole project",
  },
  "logs.viewSharedNotice": {
    message:
      "필터를 편집하면 프로젝트의 모든 사용자에게 보기가 업데이트됩니다.",
    description:
      "Render's notice under the Edit view filters: a view is shared by the whole project",
  },
  "logs.viewSaveFailed": {
    message: "보기를 저장할 수 없습니다. 다시 시도하세요.",
    description:
      "Edit view modal error when saving failed without a server reason",
  },
  "logs.viewClose": {
    message: "닫기",
    description:
      "Edit view modal button that discards the changes (Render's Close)",
  },
  "logs.viewDone": {
    message: "완료",
    description: "Edit view modal button that saves the view (Render's Done)",
  },
  "logs.viewFilterField": {
    message: "필드",
    description: "Accessible name of a filter row's Field picker",
  },
  "logs.viewFilterOperator": {
    message: "연산자",
    description: "Accessible name of a filter row's Operator picker",
  },
  "logs.viewFilterValuesFor": {
    message: "{field} 값",
    description:
      "Accessible name of a filter row's Values picker; field is the row's field",
  },
  "logs.viewValuesRequired": {
    message: "값을 하나 이상 선택하세요.",
    description: "Inline refusal of a filter row with no value",
  },
  "logs.viewRemoveFilter": {
    message: "{field} 필터 제거",
    description:
      "Accessible name of a filter row's trash button; field is the row's field",
  },
  "logs.viewOperatorIs": {
    message: "같음",
    description:
      "Filter operator IS for a single-valued field (Environment, Time Range)",
  },
  "logs.viewOperatorIsOneOf": {
    message: "다음 중 하나",
    description: "Filter operator IS for a field that takes several values",
  },
  "logs.viewOperatorIsNotOneOf": {
    message: "다음 중 하나가 아님",
    description: "Filter operator IS_NOT",
  },
  "logs.viewOperatorContains": {
    message: "포함",
    description: "Filter operator CONTAINS, Text Search's only operator",
  },
  "logs.viewValuesPlaceholder": {
    message: "값 선택",
    description:
      "Placeholder of a filter row's Values picker with nothing chosen",
  },
  "logs.viewNoResources": {
    message: "이 프로젝트에 리소스가 없습니다",
    description:
      "Resource picker of a project with no resource the view can read",
  },
  "logs.viewValueTypeHint": {
    message: "값을 입력한 뒤 Enter를 누르세요",
    description: "Placeholder of a typed filter value (Text Search's terms)",
  },
  "logs.viewValueRemove": {
    message: "{value} 제거",
    description:
      "Accessible name of a typed filter value's remove button; value is the value",
  },
  "logs.viewValueSearch": {
    message: "검색하거나 값을 입력하세요",
    description:
      "Placeholder of the search box in a filter's value picker fed by the project's logs (Level, Instance, Method, Status Code, Host, Path)",
  },
  "logs.viewValueAdd": {
    message: "“{value}” 추가",
    description:
      "Picker item that adds the typed text as a filter value; value is the typed text",
  },
  "logs.viewValuesLoading": {
    message: "프로젝트 로그를 읽는 중…",
    description:
      "Value picker status while the project's logs are read for the field's values",
  },
  "logs.viewValuesUnavailable": {
    message: "프로젝트 로그를 읽을 수 없습니다. 값을 입력해 추가하세요.",
    description:
      "Value picker status when the project's logs could not be read for the field's values",
  },
  "logs.viewValuesNoneSeen": {
    message: "최근 7일간 프로젝트 로그에 없습니다. 값을 입력해 추가하세요.",
    description:
      "Value picker status when the project's logs hold no value of the field in the last 7 days",
  },
  "logs.viewFieldEnvironment": {
    message: "환경",
    description: "Filter field Environment in the Edit view modal",
  },
  "logs.viewFieldResource": {
    message: "리소스",
    description: "Filter field Resource in the Edit view modal",
  },
  "logs.viewFieldResourceType": {
    message: "리소스 유형",
    description: "Filter field Resource Type in the Edit view modal",
  },
  "logs.viewFieldText": {
    message: "텍스트 검색",
    description: "Filter field Text Search in the Edit view modal",
  },
  "logs.viewFieldTimeRange": {
    message: "시간 범위",
    description: "Filter field Time Range in the Edit view modal",
  },
  "logs.viewResourceTypeService": {
    message: "서비스",
    description: "Resource Type value Services",
  },
  "logs.viewResourceTypeCronJob": {
    message: "Cron 작업",
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
    message: "수준: {level}",
    description:
      "Log line level gutter (w3/m509): accessible name and hover text of the level icon leading a log row; {level} is the line's level word (info, warning, error, \u2026)",
  },
  "logs.copied": {
    message: "로그를 복사했습니다",
    description:
      "Log explorer (w3/m509): toast after Options > Copy logs wrote the lines shown to the clipboard",
  },
  "logs.copyFailed": {
    message: "로그를 복사할 수 없습니다",
    description:
      "Log explorer (w3/m509): toast when the clipboard refused the copied logs",
  },
  "logs.copyLogs": {
    message: "로그 복사",
    description:
      "Log explorer Options menu (w3/m509): item copying every line shown to the clipboard (Render's Copy logs)",
  },
  "logs.options": {
    message: "로그 옵션",
    description:
      "Log explorer (w3/m509): accessible name and tooltip of the Options (\u2026) menu button",
  },
  "logs.orderBy": {
    message: "정렬 기준",
    description:
      "Log explorer Options menu (w3/m509): heading of the line order choice (Render's Order by)",
  },
  "logs.orderAscending": {
    message: "오름차순",
    description:
      "Log explorer Options menu (w3/m509): order choice, oldest line first and the newest at the bottom",
  },
  "logs.orderDescending": {
    message: "내림차순",
    description:
      "Log explorer Options menu (w3/m509): order choice, newest line first at the top",
  },
  "logs.display": {
    message: "표시:",
    description:
      "Log explorer Options menu (w3/088): the Display submenu's label, before the chosen display in bold (Render's `Display: Expand everything`)",
  },
  "logs.displaySettings": {
    message: "표시 설정",
    description:
      "Log explorer Options menu (w3/088): heading of the Display choices (Render's Display settings)",
  },
  "logs.expandEverything": {
    message: "모두 펼치기",
    description:
      "Log explorer Options ▸ Display (w3/088): every log line shown whole, the default (Render's Expand everything)",
  },
  "logs.expandErrors": {
    message: "오류 펼치기",
    description:
      "Log explorer Options ▸ Display (w3/088): error, critical, alert and emergency lines shown whole, every other line on one line (Render's Expand errors)",
  },
  "logs.collapseEverything": {
    message: "모두 접기",
    description:
      "Log explorer Options ▸ Display (w3/088): every log line on one line, cut off with an ellipsis (Render's Collapse everything)",
  },
  "logs.maximize": {
    message: "최대화",
    description:
      "Log explorer (w3/m509): button and menu item expanding the log to fill the screen (Render's Maximize)",
  },
  "logs.minimize": {
    message: "전체 화면 종료",
    description:
      "Log explorer (w3/m509): button and menu item restoring the maximized log to its place in the page",
  },
  "logs.maximizeHint": {
    message: "최대화 (M)",
    description:
      "Log explorer (w3/m509): tooltip of the Maximize button, naming its M shortcut",
  },
  "logs.minimizeHint": {
    message: "전체 화면 종료 (M 또는 Esc)",
    description:
      "Log explorer (w3/m509): tooltip of the button leaving full screen, naming the M and Escape keys that also leave it",
  },
  "logs.clearedTitle": {
    message: "로그를 지웠습니다",
    description:
      "Log explorer (w3/m509): title over a live tail whose lines the reader cleared with Shift+L",
  },
  "logs.clearedBody": {
    message: "새 줄이 도착하면 여기에 표시됩니다.",
    description:
      "Log explorer (w3/m509): body under the cleared live tail's title",
  },
  "logs.clearedRestore": {
    message: "이전 줄 보기",
    description:
      "Log explorer (w3/m509): button under a cleared live tail bringing back the lines Shift+L cleared",
  },
  "logs.clearLogs": {
    message: "로그 지우기",
    description:
      "Log explorer Options menu (w3/092): item clearing a live tail's lines, as Shift+L does (Render's Clear logs); offered only while the tail is live",
  },
};

export default koLogs;
