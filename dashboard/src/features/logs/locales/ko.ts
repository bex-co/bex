import type { TranslationEntry } from "@/i18n";

const koLogs: Record<string, TranslationEntry> = {
  "logs.rangeLabel": {
    message: "로그 기록 범위",
    description: "Accessible label for the relative log-history range controls",
  },
  "logs.rangeHistoryNote": {
    message:
      "범위가 기록을 제한합니다. 실시간 모드에서는 새 줄이 도착하는 대로 추가됩니다.",
    description:
      "Explains how the bounded history window interacts with live tail",
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
      "실시간 보기가 꺼져 있습니다. 메서드, 상태, 경로, 호스트는 요청 로그를 필터링하므로 이 조건으로는 애플리케이션 로그를 실시간으로 볼 수 없습니다.",
    description:
      "Note when the chosen log type cannot carry an active filter, so live tail is off (a request filter on app logs)",
  },
  "logs.liveWindowClosed": {
    message:
      "실시간 보기가 꺼져 있습니다. 이 사용자 지정 범위는 이미 끝났으므로 새 줄이 도착할 수 없습니다.",
    description:
      "Note when the selected custom time range ended before it was applied, so live tail is off",
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
  "logs.live": {
    message: "실시간",
    description: "Label for the live-tail on/off toggle",
  },
  "logs.jumpToLatest": {
    message: "최신으로 이동",
    description: "Button to re-pin autoscroll to the newest log line",
  },
  "logs.linesLabel": {
    message: "로그 줄",
    description:
      "Accessible name of the scrollable log list, which takes keyboard focus so arrow keys and Page Up/Down scroll it",
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
  "logs.paused": {
    message: "실시간 보기 일시 중지됨",
    description: "Status under the log list when live tail is toggled off",
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
};

export default koLogs;
