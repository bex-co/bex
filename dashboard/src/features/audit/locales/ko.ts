import type { TranslationEntry } from "@/i18n";

const koAudit: Record<string, TranslationEntry> = {
  "audit.title": {
    message: "감사 로그",
    description: "Settings Audit Log section card title",
  },
  "audit.description": {
    message:
      "이 워크스페이스에서 누가 무엇을 했는지 허용 및 거부 여부와 함께 최신순으로 표시합니다. 워크스페이스 관리자만 볼 수 있습니다.",
    description: "Settings Audit Log section card description",
  },
  "audit.columnTimestamp": {
    message: "타임스탬프",
    description: "Audit Log table column header",
  },
  "audit.columnActor": {
    message: "수행자",
    description: "Audit Log table column header — who performed the action",
  },
  "audit.columnSource": {
    message: "출처",
    description:
      "Audit Log table column header — the client address and user agent the actor acted from (w9/m450)",
  },
  "audit.columnAction": {
    message: "작업",
    description: "Audit Log table column header — the verb performed",
  },
  "audit.columnStatus": {
    message: "상태",
    description: "Audit Log table column header — allowed or denied",
  },
  "audit.columnResource": {
    message: "리소스",
    description: "Audit Log table column header — the object acted on",
  },
  "audit.changeRename": {
    message: "이름 {from} → {to}",
    description:
      "Audit log Resource cell: a service rename's label before and after (w5/183)",
  },
  "audit.changeDomain": {
    message: "도메인 {domain}",
    description:
      "Audit log Resource cell: the custom domain a domain verb added, removed or verified (w5/183)",
  },
  "audit.changeEnvGroup": {
    message: "환경 그룹 {name}",
    description:
      "Audit log Resource cell: the environment group a link or unlink changed (w5/183)",
  },
  "audit.changeEnvVars": {
    message: "변수 {keys}",
    description:
      "Audit log Resource cell: the environment variable names an env var row created, changed or deleted, never their values (w9/m449)",
  },
  "audit.changeGrantScopes": {
    message: "스코프 {scopes}",
    description:
      "Audit log Resource cell: the scopes a member granted an OAuth client (GrantOAuthAccessEvent, w4/m454)",
  },
  "audit.statusAllowed": {
    message: "허용됨",
    description: "Audit Log status badge for a successful action",
  },
  "audit.statusDenied": {
    message: "거부됨",
    description: "Audit Log status badge for a denied action",
  },
  "audit.actorUnknown": {
    message: "알 수 없음",
    description:
      "Audit Log actor cell placeholder for an unauthenticated caller",
  },
  "audit.oauthDelegation": {
    message: "{client} · {scopes}",
    description:
      "Audit Log actor subtitle for a third-party OAuth grant (client id and canonical scopes)",
  },
  "audit.loadMore": {
    message: "더 보기",
    description: "Button that fetches the next page of audit events",
  },
  "audit.emptyTitle": {
    message: "아직 감사 이벤트가 없습니다",
    description: "Audit Log empty-state title",
  },
  "audit.emptyBody": {
    message: "이 워크스페이스의 쓰기 작업이 여기에 표시됩니다.",
    description: "Audit Log empty-state body",
  },
  "audit.errorTitle": {
    message: "감사 로그를 불러오지 못했습니다",
    description: "Audit Log generic error title",
  },
  "audit.errorBody": {
    message:
      "이 워크스페이스의 감사 기록을 불러오는 중 문제가 발생했습니다. 다시 시도하세요.",
    description: "Audit Log generic error body",
  },
  "audit.unavailableTitle": {
    message: "감사 로그가 구성되지 않았습니다",
    description:
      "Audit Log state when the control-plane store isn't wired (503)",
  },
  "audit.unavailableBody": {
    message: "이 bex 배포에는 감사 로그 저장소가 구성되어 있지 않습니다.",
    description: "Audit Log unavailable-state body",
  },
  "audit.planTitle": {
    message: "감사 로그는 Pro 요금제가 필요합니다",
    description:
      "Audit Log title on a Hobby workspace (PLAN_FEATURE_REQUIRED, w8/m138)",
  },
  "audit.planBody": {
    message:
      "감사 로그는 Pro 요금제 이상에서 사용할 수 있습니다. 이 워크스페이스는 Hobby 요금제입니다.",
    description: "Audit Log plan notice body on a Hobby workspace (w8/m138)",
  },
  "audit.exportStart": {
    message: "시작일 (UTC)",
    description: "Audit Log CSV export start-date label",
  },
  "audit.exportEnd": {
    message: "종료일 (UTC)",
    description: "Audit Log CSV export end-date label",
  },
  "audit.exportCSV": {
    message: "CSV로 내보내기",
    description: "Audit Log CSV export button (Render's label)",
  },
  "audit.exportHint": {
    message:
      "최대 {days}일 분량의 이벤트(타임스탬프, 수행자, 이벤트, 상태, 메타데이터)를 내보냅니다.",
    description: "Audit Log CSV export help text; {days} is the longest range",
  },
  "audit.exportRangeOrder": {
    message: "종료일은 시작일과 같거나 그 이후여야 합니다.",
    description: "Audit Log CSV export: end before start",
  },
  "audit.exportRangeTooLong": {
    message: "내보내기는 최대 {days}일까지 가능합니다.",
    description:
      "Audit Log CSV export: range over the limit; {days} is the limit",
  },
  "audit.exportTooLarge": {
    message:
      "이 기간에는 한 번에 내보낼 이벤트가 너무 많습니다. 더 짧은 기간을 선택하세요.",
    description: "Audit Log CSV export: row budget exceeded",
  },
  "audit.exportFailed": {
    message: "내보내기에 실패했습니다. 다시 시도하세요.",
    description: "Audit Log CSV export: request failed",
  },
  "audit.exportRangeMissing": {
    message: "시작일과 종료일을 선택하세요.",
    description: "Audit Log CSV export: a date is empty",
  },
};

export default koAudit;
