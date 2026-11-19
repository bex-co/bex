import type { TranslationEntry } from "@/i18n";

const koWebhooks: Record<string, TranslationEntry> = {
  "webhooks.title": {
    message: "웹훅",
    description: "Settings Integrations Webhooks section card title",
  },
  "webhooks.description": {
    message:
      "서명된 이벤트 알림(배포, 일시 중지, 다시 시작, 확장)을 폴링 없이 내 엔드포인트로 보냅니다. 각 웹훅의 서명 시크릿은 해당 웹훅의 설정에 있습니다.",
    description: "Settings Webhooks section card description",
  },
  "webhooks.colEndpoint": {
    message: "엔드포인트",
    description: "Webhooks table column header — name + destination URL",
  },
  "webhooks.colEvents": {
    message: "이벤트",
    description: "Webhooks table column header — subscribed event types",
  },
  "webhooks.colEnabled": {
    message: "사용",
    description: "Webhooks table column header — the enable/disable switch",
  },
  "webhooks.colLatest": {
    message: "최근 전송",
    description: "Webhooks table column header — newest immutable attempt",
  },
  "webhooks.searchEndpoints": {
    message: "웹훅 검색",
    description: "Endpoint list search label and placeholder",
  },
  "webhooks.searchEndpointsHelp": {
    message: "이름, 대상 또는 이벤트로 검색",
    description: "Endpoint list search helper text",
  },
  "webhooks.searchEndpointsEmpty": {
    message: "검색과 일치하는 웹훅이 없습니다.",
    description: "Endpoint list empty state after local search",
  },
  "webhooks.emptyTitle": {
    message: "웹훅 없음",
    description: "Webhooks empty-state title",
  },
  "webhooks.emptyBody": {
    message:
      "서비스가 배포, 다시 시작 또는 확장될 때 알림을 받으려면 엔드포인트를 추가하세요.",
    description: "Webhooks empty-state body",
  },
  "webhooks.forbiddenTitle": {
    message: "권한 없음",
    description: "Webhooks state when the caller lacks permission (403)",
  },
  "webhooks.forbiddenBody": {
    message: "이 워크스페이스의 웹훅을 볼 권한이 없습니다.",
    description: "Webhooks forbidden-state body",
  },
  "webhooks.errorTitle": {
    message: "웹훅을 불러오지 못했습니다",
    description: "Webhooks generic error title",
  },
  "webhooks.errorBody": {
    message: "문제가 발생했습니다. 다시 시도하세요.",
    description: "Webhooks generic error body",
  },
  "webhooks.proOnly": {
    message:
      "웹훅은 Pro 요금제 이상에서 사용할 수 있습니다. 기존 엔드포인트는 계속 전송됩니다.",
    description: "Webhooks card description on a Hobby workspace (w8/m138)",
  },
  "webhooks.create": {
    message: "웹훅 추가",
    description: "Button that opens the create dialog",
  },
  "webhooks.createTitle": {
    message: "웹훅 추가",
    description: "Create dialog title",
  },
  "webhooks.createDescription": {
    message:
      "구독한 이벤트가 발생할 때마다 bex가 이 URL로 서명된 JSON 페이로드를 POST합니다.",
    description: "Create dialog description",
  },
  "webhooks.createEnabledHelp": {
    message: "즉시 전송을 시작합니다. 나중에 설정에서 활성화할 수도 있습니다.",
    description: "Create page initial enabled-state helper copy",
  },
  "webhooks.fieldName": {
    message: "이름",
    description: "Create dialog name field label",
  },
  "webhooks.fieldNamePlaceholder": {
    message: "예: deploy-alerts-slack-bot",
    description: "Create dialog name field placeholder",
  },
  "webhooks.fieldUrl": {
    message: "대상 URL",
    description: "Create dialog URL field label",
  },
  "webhooks.fieldEvents": {
    message: "전송할 이벤트",
    description: "Create dialog event-type checklist label",
  },
  "webhooks.eventsLoading": {
    message: "이벤트 유형을 불러오는 중…",
    description: "Create dialog while the event-type vocabulary loads",
  },
  "webhooks.eventsLoadError": {
    message:
      "이벤트 유형을 불러오지 못했습니다. 저장하기 전에 다시 시도하세요.",
    description: "Actionable event-catalog query failure",
  },
  "webhooks.eventsRetry": {
    message: "이벤트 유형 다시 시도",
    description: "Retry button for the event-catalog query",
  },
  "webhooks.eventsEmpty": {
    message: "현재 구독할 수 있는 이벤트 유형이 없습니다.",
    description: "Event picker state when the served catalog is empty",
  },
  "webhooks.formUnavailableEvents": {
    message: "이벤트 유형을 불러오기 전에는 생성할 수 없습니다.",
    description:
      "Create submit disabled reason while the catalog is unavailable",
  },
  "webhooks.formErrorsSummary": {
    message: "강조 표시된 필드를 수정한 뒤 다시 시도하세요.",
    description: "Accessible webhook form validation summary",
  },
  "webhooks.validation.nameRequired": {
    message: "웹훅 이름을 입력하세요.",
    description: "Field-local required-name validation",
  },
  "webhooks.validation.nameDuplicate": {
    message: "같은 이름의 웹훅이 이미 있습니다.",
    description: "Field-local duplicate-name validation",
  },
  "webhooks.validation.urlRequired": {
    message: "대상 URL을 입력하세요.",
    description: "Field-local required URL validation",
  },
  "webhooks.validation.urlHTTPS": {
    message: "자격 증명이 포함되지 않은 절대 HTTPS URL을 입력하세요.",
    description: "Field-local webhook URL validation",
  },
  "webhooks.validation.eventsRequired": {
    message: "이벤트를 하나 이상 선택하세요.",
    description: "Field-local event selection validation",
  },
  "webhooks.validation.eventsInvalid": {
    message:
      "이벤트 선택이 변경되었습니다. 카탈로그를 다시 불러와 다시 선택하세요.",
    description: "Server-coded stale/invalid event filter",
  },
  "webhooks.errors.planLimit": {
    message:
      "웹훅은 Pro 요금제 이상에서 사용할 수 있습니다. 이 워크스페이스는 Hobby 요금제이므로 엔드포인트를 추가할 수 없습니다.",
    description:
      "Server-coded plan refusal of a webhook create on a Hobby workspace (WEBHOOK_PLAN_LIMIT, w4/425)",
  },
  "webhooks.planGate": {
    message:
      "웹훅은 Pro 요금제 이상에서 사용할 수 있습니다. 엔드포인트를 추가하려면 이 워크스페이스를 업그레이드하세요.",
    description:
      "Notice in place of the create form and the empty list on a Hobby workspace (w4/425)",
  },
  "webhooks.errors.endpointLimit": {
    message: "이 워크스페이스는 웹훅 엔드포인트 한도에 도달했습니다.",
    description: "Server-coded endpoint quota refusal",
  },
  "webhooks.errors.endpointDisabled": {
    message: "전송을 다시 보내기 전에 이 엔드포인트를 활성화하세요.",
    description: "Server-coded resend refusal for a disabled endpoint",
  },
  "webhooks.errors.deliveryPending": {
    message: "전송 시도가 이미 대기 중입니다. 완료될 때까지 기다리세요.",
    description: "Server-coded duplicate pending resend refusal",
  },
  "webhooks.errors.staleState": {
    message:
      "이 웹훅이 변경되었거나 삭제되었습니다. 새로 고친 뒤 다시 시도하세요.",
    description: "Safe stale endpoint/attempt refusal",
  },
  "webhooks.manageRequired": {
    message:
      "이 웹훅을 조회할 수는 있지만 변경은 워크스페이스 관리자만 할 수 있습니다.",
    description: "Read-only member notice on webhook management surfaces",
  },
  "webhooks.latestNever": {
    message: "전송된 적 없음",
    description: "Endpoint with no completed delivery attempt",
  },
  "webhooks.latestDelivered": {
    message: "성공",
    description: "Latest endpoint attempt succeeded",
  },
  "webhooks.latestFailed": {
    message: "실패",
    description: "Latest endpoint attempt failed terminally",
  },
  "webhooks.latestRetrying": {
    message: "재시도 중",
    description: "Latest endpoint attempt failed with a retry pending",
  },
  "webhooks.createCancel": {
    message: "취소",
    description: "Create dialog cancel button",
  },
  "webhooks.createSubmit": {
    message: "웹훅 생성",
    description: "Create page submit button (Render: 'Create Webhook')",
  },
  "webhooks.createSuccess": {
    message: "웹훅을 생성했습니다",
    description: "Toast after a successful create",
  },
  "webhooks.createError": {
    message: "웹훅을 생성하지 못했습니다",
    description: "Toast after a failed create",
  },
  "webhooks.copy": {
    message: "서명 시크릿 복사",
    description: "Copy-secret button label",
  },
  "webhooks.copied": {
    message: "시크릿을 클립보드에 복사했습니다",
    description: "Toast after copying the secret",
  },
  "webhooks.copyError": {
    message: "복사하지 못했습니다. 텍스트를 선택하여 직접 복사하세요",
    description: "Toast when clipboard write fails",
  },
  "webhooks.toggle": {
    message: "사용",
    description: "Accessible label of the per-endpoint enable/disable switch",
  },
  "webhooks.enableSuccess": {
    message: "{name} 활성화됨",
    description: "Toast after enabling an endpoint",
  },
  "webhooks.disableSuccess": {
    message: "{name} 비활성화됨",
    description: "Toast after disabling an endpoint",
  },
  "webhooks.toggleError": {
    message: "{name} 업데이트에 실패했습니다",
    description: "Toast after a failed enable/disable",
  },
  "webhooks.delete": {
    message: "삭제",
    description: "Delete action label (icon button + confirm button)",
  },
  "webhooks.deleteConfirmTitle": {
    message: "{name} 웹훅을 삭제할까요?",
    description: "Delete confirmation dialog title",
  },
  "webhooks.deleteConfirmBody": {
    message:
      "이 엔드포인트로 더 이상 이벤트가 전송되지 않으며 전송 기록도 삭제됩니다.",
    description: "Delete confirmation dialog body",
  },
  "webhooks.deleteCancel": {
    message: "취소",
    description: "Delete confirmation cancel button",
  },
  "webhooks.deleteSuccess": {
    message: "{name} 삭제됨",
    description: "Toast after a successful delete",
  },
  "webhooks.deleteError": {
    message: "{name} 삭제에 실패했습니다",
    description: "Toast after a failed delete",
  },
  "webhooks.history": {
    message: "전송 기록",
    description: "Accessible label of the per-endpoint history button",
  },
  "webhooks.historyTitle": {
    message: "전송 기록 — {name}",
    description: "Delivery-history dialog title",
  },
  "webhooks.historyBody": {
    message:
      "이 엔드포인트로 보낸 각 이벤트를 최신순으로 표시합니다. 실패한 전송은 지수 백오프로 재시도하며, 반복해서 실패하면 엔드포인트가 비활성화됩니다.",
    description: "Delivery-history dialog description",
  },
  "webhooks.historyEmptyTitle": {
    message: "아직 전송 내역이 없습니다",
    description: "Delivery-history empty-state title",
  },
  "webhooks.historyEmptyBody": {
    message: "구독한 이벤트(예: 배포)를 발생시키면 여기에 표시됩니다.",
    description: "Delivery-history empty-state body",
  },
  "webhooks.historyFilteredEmptySuccessfulTitle": {
    message: "성공한 전송 없음",
    description:
      "Delivery-history state when the Successful tab (and any date range) matches nothing (w4/146)",
  },
  "webhooks.historyFilteredEmptyFailedTitle": {
    message: "실패한 전송 없음",
    description:
      "Delivery-history state when the Failed tab (and any date range) matches nothing (w4/146)",
  },
  "webhooks.historyFilteredEmptyRangeTitle": {
    message: "이 범위에 전송 내역 없음",
    description:
      "Delivery-history state when the Sent after/before range matches nothing (w4/146)",
  },
  "webhooks.historyFilteredEmptyBody": {
    message: "다른 상태나 기간에는 일치하는 전송이 있을 수 있습니다.",
    description: "Body under a filtered-empty delivery history (w4/146)",
  },
  "webhooks.historyClearFilters": {
    message: "필터 지우기",
    description:
      "Resets the delivery-history status tab and date range (w4/146)",
  },
  "webhooks.historyErrorTitle": {
    message: "전송 기록을 불러오지 못했습니다",
    description: "Delivery-history error-state title",
  },
  "webhooks.historyErrorBody": {
    message: "문제가 발생했습니다. 다시 시도하세요.",
    description: "Delivery-history error-state body",
  },
  "webhooks.colEvent": {
    message: "이벤트",
    description: "Delivery-history table column header",
  },
  "webhooks.colService": {
    message: "서비스",
    description: "Delivery-history table column header",
  },
  "webhooks.serviceDeleted": {
    message: "(삭제됨)",
    description:
      "Marker beside a delivery's recorded service name when that service no longer exists, so the name cannot be linked",
  },
  "webhooks.colStatus": {
    message: "상태",
    description: "Delivery-history table column header",
  },
  "webhooks.colAttempts": {
    message: "시도",
    description:
      "Delivery-history table column header — one-based attempt number",
  },
  "webhooks.colResponse": {
    message: "응답",
    description: "Delivery-history table column header — last HTTP status",
  },
  "webhooks.colWhen": {
    message: "시간",
    description:
      "Delivery-history table column header — relative and exact send time",
  },
  "webhooks.colActions": {
    message: "작업",
    description: "Accessible delivery-history table actions column header",
  },
  "webhooks.status.pending": {
    message: "대기 중",
    description: "Delivery status badge — queued or between retries",
  },
  "webhooks.status.delivered": {
    message: "전송됨",
    description: "Delivery status badge — endpoint answered 2xx",
  },
  "webhooks.status.failed": {
    message: "실패",
    description: "Delivery status badge — this individual attempt failed",
  },
  "webhooks.httpStatus": {
    message: "HTTP {status}",
    description: "Delivery attempt HTTP status summary",
  },
  "webhooks.openSourceEvent": {
    message: "{label} — 소스 이벤트 {id} 열기",
    description:
      "Accessible label for the event hydration link; it starts with the link's visible text, the event type (WCAG 2.5.3)",
  },
  "webhooks.requestPayload": {
    message: "요청 페이로드",
    description: "Expanded delivery-attempt request evidence heading",
  },
  "webhooks.endpointResponse": {
    message: "엔드포인트 응답",
    description: "Expanded delivery-attempt response evidence heading",
  },
  "webhooks.noRequestEvidence": {
    message: "사용할 수 있는 요청 기록이 없습니다.",
    description: "Expanded attempt with no retained request body",
  },
  "webhooks.noResponseEvidence": {
    message: "기록된 응답이 없습니다.",
    description: "Expanded pending/transport attempt with no endpoint body",
  },
  "webhooks.attemptIdentity": {
    message: "시도 {attemptId} · 소스 이벤트 {eventId}",
    description: "Immutable attempt and stable source-event identity",
  },
  "webhooks.parentDeliveryStatus": {
    message: "알림 상태: {status}",
    description:
      "Logical notification state shown separately from attempt outcome",
  },
  "webhooks.retryScheduled": {
    message: "다음 자동 시도: {date}",
    description: "Parent notification retry schedule",
  },
  "webhooks.resend": {
    message: "다시 보내기",
    description: "Manual resend action for a failed attempt",
  },
  "webhooks.resending": {
    message: "다시 보내는 중…",
    description: "Manual resend action in progress",
  },
  "webhooks.resendConfirmTitle": {
    message: "이 웹훅을 다시 보낼까요?",
    description: "Manual resend confirmation title",
  },
  "webhooks.resendConfirmBody": {
    message:
      "동일한 소스 이벤트와 요청 페이로드로 서명된 새 시도를 대기열에 추가합니다.",
    description: "Manual resend confirmation consequence",
  },
  "webhooks.resendCancel": {
    message: "취소",
    description: "Manual resend confirmation cancel action",
  },
  "webhooks.resendSuccess": {
    message: "웹훅 재전송을 대기열에 추가했습니다",
    description: "Toast after a manual resend reservation succeeds",
  },
  "webhooks.resendError": {
    message: "이 웹훅을 다시 보내지 못했습니다",
    description: "Toast after a manual resend is refused or fails",
  },
  "webhooks.loadMore": {
    message: "더 보기",
    description: "Delivery-history pagination button",
  },
  "webhooks.selectedCount_other": {
    message: "이벤트 {count}개 선택됨",
    description: "Event-picker live selection counter (plural)",
  },
  "webhooks.searchEvents": {
    message: "이벤트 검색",
    description: "Event-picker search box placeholder + accessible label",
  },
  "webhooks.searchNoMatches": {
    message: "검색과 일치하는 이벤트가 없습니다",
    description:
      "Event-picker empty state while a search filters everything out",
  },
  "webhooks.allEvents": {
    message: "모든 이벤트",
    description: "Event-picker tri-state master checkbox label",
  },
  "webhooks.groupToggle": {
    message: "{group} 이벤트 펼치기/접기",
    description: "Accessible label of a group's expand/collapse chevron",
  },
  "webhooks.group.deploy": {
    message: "배포",
    description: "Event-picker group — deploy lifecycle events",
  },
  "webhooks.group.autoDeploy": {
    message: "자동 배포",
    description: "Event-picker group — automatic deploy setting events",
  },
  "webhooks.group.serviceAvailability": {
    message: "서비스 가용성",
    description: "Event-picker group — observed service availability events",
  },
  "webhooks.group.scaling": {
    message: "확장",
    description: "Event-picker group — service scaling events",
  },
  "webhooks.group.cronJobRun": {
    message: "Cron Job 실행",
    description: "Event-picker group — cron job run events",
  },
  "webhooks.group.maintenance": {
    message: "유지 보수",
    description:
      "Event-picker group — Render maintenance events (shown only when already stored on an edited webhook, w4/709)",
  },
  "webhooks.group.maintenanceMode": {
    message: "유지 보수 모드",
    description: "Event-picker group — maintenance mode events",
  },
  "webhooks.group.disk": {
    message: "디스크",
    description: "Event-picker group — persistent disk events",
  },
  "webhooks.group.postgres": {
    message: "Postgres",
    description: "Event-picker group — managed Postgres events",
  },
  "webhooks.group.keyValue": {
    message: "Key Value",
    description: "Event-picker group — managed Key Value events",
  },
  "webhooks.group.suspension": {
    message: "일시 중지",
    description: "Event-picker group — suspend/resume events",
  },
  "webhooks.group.agentSession": {
    message: "클라우드 에이전트",
    description: "Event-picker group — cloud coding agent session events",
  },
  "webhooks.group.other": {
    message: "기타",
    description:
      "Event-picker fallback group for served keys the catalog doesn't know yet",
  },
  "webhooks.event.deploy_started": {
    message: "배포 시작됨",
    description: "Event label — deploy_started",
  },
  "webhooks.event.branch_deleted": {
    message: "브랜치 삭제됨",
    description: "Webhook event label",
  },
  "webhooks.event.build_started": {
    message: "빌드 시작됨",
    description: "Webhook event label",
  },
  "webhooks.event.build_ended": {
    message: "빌드 종료됨",
    description: "Webhook event label",
  },
  "webhooks.event.pre_deploy_started": {
    message: "사전 배포 시작됨",
    description: "Webhook event label",
  },
  "webhooks.event.pre_deploy_ended": {
    message: "사전 배포 종료됨",
    description: "Webhook event label",
  },
  "webhooks.event.job_run_ended": {
    message: "작업 실행 종료됨",
    description: "Webhook event label",
  },
  "webhooks.event.auto_deploy_enabled": {
    message: "자동 배포 활성화됨",
    description: "Webhook event label",
  },
  "webhooks.event.auto_deploy_disabled": {
    message: "자동 배포 비활성화됨",
    description: "Webhook event label",
  },
  "webhooks.event.deploy_ended": {
    message: "배포 종료됨",
    description: "Event label — deploy_ended",
  },
  "webhooks.event.zero_downtime_redeploy_started": {
    message: "무중단 재배포 시작됨",
    description:
      "Event label — zero_downtime_redeploy_started: a Bex-initiated zero-downtime deploy started for a service",
  },
  "webhooks.event.zero_downtime_redeploy_ended": {
    message: "무중단 재배포 종료됨",
    description:
      "Event label — zero_downtime_redeploy_ended: a Bex-initiated zero-downtime deploy completed for a service",
  },
  "webhooks.event.image_pull_failed": {
    message: "이미지 가져오기 실패",
    description: "Event label — image_pull_failed",
  },
  "webhooks.event.artifact_fetch_failed": {
    message: "아티팩트 가져오기 실패",
    description:
      "Event label — artifact_fetch_failed: a build published by a build source could not be fetched",
  },
  "webhooks.event.artifact_source_changed": {
    message: "빌드 소스 변경됨",
    description:
      "Event label — artifact_source_changed: a service's linked build source changed",
  },
  "webhooks.event.pipeline_minutes_exhausted": {
    message: "파이프라인 시간 소진",
    description: "Event label — pipeline_minutes_exhausted",
  },
  "webhooks.event.edge_cache_enabled": {
    message: "엣지 캐시 활성화됨",
    description:
      "Event label — edge_cache_enabled (API-only on Render; shown on subscriptions and deliveries, not in the picker)",
  },
  "webhooks.event.edge_cache_disabled": {
    message: "엣지 캐시 비활성화됨",
    description:
      "Event label — edge_cache_disabled (API-only on Render; shown on subscriptions and deliveries, not in the picker)",
  },
  "webhooks.event.edge_cache_purged": {
    message: "엣지 캐시 삭제됨",
    description:
      "Event label — edge_cache_purged (API-only on Render; shown on subscriptions and deliveries, not in the picker)",
  },
  "webhooks.event.commit_ignored": {
    message: "커밋 무시됨",
    description: "Event label — commit_ignored",
  },
  "webhooks.event.server_failed": {
    message: "서버 실패",
    description: "Event label — server_failed",
  },
  "webhooks.event.server_hardware_failure": {
    message: "서버 하드웨어 장애",
    description: "Event label — server_hardware_failure",
  },
  "webhooks.event.server_available": {
    message: "서버 사용 가능",
    description: "Event label — server_available",
  },
  "webhooks.event.autoscaling_started": {
    message: "자동 확장 시작됨",
    description: "Event label — autoscaling_started",
  },
  "webhooks.event.autoscaling_ended": {
    message: "자동 확장 종료됨",
    description: "Event label — autoscaling_ended",
  },
  "webhooks.event.branch_changed": {
    message: "브랜치 변경됨",
    description: "Event label — branch_changed bex extension",
  },
  "webhooks.event.cron_job_run_started": {
    message: "Cron Job 실행 시작됨",
    description: "Event label — cron_job_run_started",
  },
  "webhooks.event.cron_job_run_ended": {
    message: "Cron Job 실행 종료됨",
    description: "Event label — cron_job_run_ended",
  },
  "webhooks.event.maintenance_started": {
    message: "유지 보수 시작됨",
    description:
      "Event label — maintenance_started (Render enum value accepted for compatibility, never emitted by bex, w4/709)",
  },
  "webhooks.event.maintenance_ended": {
    message: "유지 보수 종료됨",
    description:
      "Event label — maintenance_ended (Render enum value accepted for compatibility, never emitted by bex, w4/709)",
  },
  "webhooks.event.maintenance_mode_enabled": {
    message: "유지 보수 모드 활성화됨",
    description: "Event label — maintenance_mode_enabled",
  },
  "webhooks.event.maintenance_mode_uri_updated": {
    message: "유지 보수 모드 URI 업데이트됨",
    description: "Event label — maintenance_mode_uri_updated",
  },
  "webhooks.event.disk_created": {
    message: "디스크 생성됨",
    description: "Event label — disk_created",
  },
  "webhooks.event.disk_updated": {
    message: "디스크 업데이트됨",
    description: "Event label — disk_updated",
  },
  "webhooks.event.disk_deleted": {
    message: "디스크 삭제됨",
    description: "Event label — disk_deleted",
  },
  "webhooks.event.disk_snapshot_restored": {
    message: "디스크 스냅샷 복원됨",
    description: "Event label — disk_snapshot_restored",
  },
  "webhooks.event.postgres_created": {
    message: "Postgres 생성됨",
    description: "Event label — postgres_created",
  },
  "webhooks.event.postgres_restarted": {
    message: "Postgres 다시 시작됨",
    description: "Event label — postgres_restarted",
  },
  "webhooks.event.postgres_credentials_created": {
    message: "Postgres 자격 증명 생성됨",
    description: "Event label — postgres_credentials_created",
  },
  "webhooks.event.postgres_credentials_deleted": {
    message: "Postgres 자격 증명 삭제됨",
    description: "Event label — postgres_credentials_deleted",
  },
  "webhooks.event.postgres_backup_started": {
    message: "Postgres 백업 시작됨",
    description: "Event label — postgres_backup_started",
  },
  "webhooks.event.postgres_available": {
    message: "Postgres 사용 가능",
    description: "Event label — postgres_available",
  },
  "webhooks.event.postgres_unavailable": {
    message: "Postgres 사용 불가",
    description: "Event label — postgres_unavailable",
  },
  "webhooks.event.postgres_ha_status_changed": {
    message: "Postgres 고가용성 변경됨",
    description: "Event label — postgres_ha_status_changed",
  },
  "webhooks.event.postgres_cluster_leader_changed": {
    message: "Postgres 클러스터 리더 변경됨",
    description: "Event label — postgres_cluster_leader_changed",
  },
  "webhooks.event.postgres_connection_pool_changed": {
    message: "Postgres 커넥션 풀 설정 변경됨",
    description:
      "Event label — postgres_connection_pool_changed (Render enum value accepted for compatibility, never emitted by bex, w4/709)",
  },
  "webhooks.event.postgres_connection_pool_enabled_changed": {
    message: "Postgres 커넥션 풀 변경됨",
    description: "Event label — postgres_connection_pool_enabled_changed",
  },
  "webhooks.event.postgres_disk_size_changed": {
    message: "Postgres 디스크 크기 변경됨",
    description: "Event label — postgres_disk_size_changed",
  },
  "webhooks.event.postgres_disk_autoscaling_enabled_changed": {
    message: "Postgres 디스크 자동 확장 변경됨",
    description: "Event label — postgres_disk_autoscaling_enabled_changed",
  },
  "webhooks.event.postgres_read_replicas_changed": {
    message: "Postgres 읽기 복제본 변경됨",
    description: "Event label — postgres_read_replicas_changed",
  },
  "webhooks.event.postgres_read_replica_stale": {
    message: "Postgres 읽기 복제본 동기화 지연",
    description: "Event label — postgres_read_replica_stale",
  },
  "webhooks.event.postgres_backup_completed": {
    message: "Postgres 백업 완료됨",
    description: "Event label — postgres_backup_completed",
  },
  "webhooks.event.postgres_backup_failed": {
    message: "Postgres 백업 실패",
    description: "Event label — postgres_backup_failed",
  },
  "webhooks.event.postgres_pitr_checkpoint_started": {
    message: "Postgres PITR 체크포인트 시작됨",
    description: "Event label — postgres_pitr_checkpoint_started",
  },
  "webhooks.event.postgres_pitr_checkpoint_completed": {
    message: "Postgres PITR 체크포인트 완료됨",
    description: "Event label — postgres_pitr_checkpoint_completed",
  },
  "webhooks.event.postgres_pitr_checkpoint_failed": {
    message: "Postgres PITR 체크포인트 실패",
    description: "Event label — postgres_pitr_checkpoint_failed",
  },
  "webhooks.event.postgres_wal_archive_failed": {
    message: "Postgres WAL 아카이브 실패",
    description: "Event label — postgres_wal_archive_failed",
  },
  "webhooks.event.postgres_restore_succeeded": {
    message: "Postgres 복원 성공",
    description: "Event label — postgres_restore_succeeded",
  },
  "webhooks.event.postgres_restore_failed": {
    message: "Postgres 복원 실패",
    description: "Event label — postgres_restore_failed",
  },
  "webhooks.event.postgres_upgrade_started": {
    message: "Postgres 업그레이드 시작됨",
    description: "Event label — postgres_upgrade_started",
  },
  "webhooks.event.postgres_upgrade_succeeded": {
    message: "Postgres 업그레이드 성공",
    description: "Event label — postgres_upgrade_succeeded",
  },
  "webhooks.event.postgres_upgrade_failed": {
    message: "Postgres 업그레이드 실패",
    description: "Event label — postgres_upgrade_failed",
  },
  "webhooks.event.key_value_available": {
    message: "Key Value 사용 가능",
    description: "Event label — key_value_available",
  },
  "webhooks.event.key_value_unhealthy": {
    message: "Key Value 비정상",
    description: "Event label — key_value_unhealthy",
  },
  "webhooks.event.key_value_config_restart": {
    message: "구성 변경으로 Key Value 다시 시작됨",
    description: "Event label — key_value_config_restart",
  },
  "webhooks.event.service_suspended": {
    message: "서비스 일시 중지됨",
    description: "Event label — service_suspended",
  },
  "webhooks.event.service_resumed": {
    message: "서비스 재개됨",
    description: "Event label — service_resumed",
  },
  "webhooks.event.service_hibernated": {
    message: "서비스가 절전 상태가 됨",
    description: "Event label — service_hibernated",
  },
  "webhooks.event.service_woken": {
    message: "서비스가 절전에서 깨어남",
    description: "Event label — service_woken",
  },
  "webhooks.event.agent_session_turn_succeeded": {
    message: "에이전트 세션 턴 성공",
    description: "Event label — agent_session_turn_succeeded",
  },
  "webhooks.event.agent_session_turn_failed": {
    message: "에이전트 세션 턴 실패",
    description: "Event label — agent_session_turn_failed",
  },
  "webhooks.event.agent_session_needs_input": {
    message: "에이전트 세션 입력 필요",
    description: "Event label — agent_session_needs_input",
  },
  "webhooks.event.service_moved": {
    message: "서비스 이동됨",
    description:
      "Event label — service_moved (bex extension: project/environment placement change)",
  },
  "webhooks.event.server_restarted": {
    message: "서버 다시 시작됨",
    description: "Event label — server_restarted",
  },
  "webhooks.event.instance_count_changed": {
    message: "인스턴스 수 변경됨",
    description: "Event label — instance_count_changed",
  },
  "webhooks.event.autoscaling_config_changed": {
    message: "자동 확장 구성 변경됨",
    description: "Event label — autoscaling_config_changed",
  },
  "webhooks.event.plan_changed": {
    message: "인스턴스 유형 변경됨",
    description:
      "Event label — plan_changed (bex's slot for Render's Instance Type Changed)",
  },
  "webhooks.newTitle": {
    message: "새 웹훅 만들기",
    description: "Create page heading (Render: 'Create a new Webhook')",
  },
  "webhooks.newMenuLabel": {
    message: "새 웹훅",
    description:
      "+ New menu entry; worded like the other entries, unlike the create page heading",
  },
  "webhooks.backToList": {
    message: "웹훅 목록으로 돌아가기",
    description: "Accessible label of the create/detail pages' back link",
  },
  "webhooks.fieldNameHelp": {
    message: "이 웹훅의 고유한 이름입니다.",
    description: "Create page name-field helper copy",
  },
  "webhooks.fieldUrlHelp": {
    message: "bex가 각 알림을 이 URL로 POST 요청으로 보냅니다.",
    description: "Create page URL-field helper copy",
  },
  "webhooks.fieldUrlPlaceholder": {
    message: "https://example.com/webhooks/bex",
    description: "Create page URL-field placeholder",
  },
  "webhooks.fieldEventsHelp": {
    message:
      "워크스페이스에서 어떤 이벤트가 웹훅 알림을 발생시킬지 선택하세요.",
    description: "Create page events-field helper copy",
  },
  "webhooks.detailKicker": {
    message: "웹훅",
    description: "Detail page header kicker above the endpoint name",
  },
  "webhooks.idLabel": {
    message: "웹훅 ID:",
    description: "Detail header id row label",
  },
  "webhooks.copyId": {
    message: "웹훅 ID 복사",
    description: "Accessible label of the header id copy button",
  },
  "webhooks.copyUrl": {
    message: "엔드포인트 URL 복사",
    description: "Accessible label of the header URL copy button",
  },
  "webhooks.copiedGeneric": {
    message: "클립보드에 복사했습니다",
    description: "Toast after copying the id or URL",
  },
  "webhooks.showMore": {
    message: "{count}개 더 보기",
    description: "Event-chip expander on the detail header",
  },
  "webhooks.showLess": {
    message: "간단히 보기",
    description: "Event-chip collapser on the detail header",
  },
  "webhooks.createdByOn": {
    message: "{creator}님이 {date}에 생성",
    description: "Detail header provenance line when the creator is known",
  },
  "webhooks.createdOn": {
    message: "{date}에 생성",
    description: "Detail header provenance line without a creator",
  },
  "webhooks.tabActivity": {
    message: "활동",
    description: "Detail page tab — delivery history",
  },
  "webhooks.tabSettings": {
    message: "설정",
    description: "Detail page tab — edit + delete",
  },
  "webhooks.recentDeliveries": {
    message: "최근 전송",
    description: "Activity tab heading",
  },
  "webhooks.recentDeliveriesHint": {
    message:
      "각 행은 전송 시도 한 건입니다. 이 탭이 표시되는 동안 새 시도가 자동으로 나타납니다.",
    description: "Activity tab heading hint for immutable attempts + polling",
  },
  "webhooks.refresh": {
    message: "새로 고침",
    description: "Accessible label of the Activity refresh button",
  },
  "webhooks.filterAll": {
    message: "전체",
    description: "Activity delivery filter tab",
  },
  "webhooks.filterSuccessful": {
    message: "성공",
    description: "Activity delivery filter tab — delivered only",
  },
  "webhooks.filterFailed": {
    message: "실패",
    description: "Activity delivery filter tab — failed only",
  },
  "webhooks.sentAfter": {
    message: "이후에 전송",
    description: "Activity delivery-history lower timestamp bound",
  },
  "webhooks.sentBefore": {
    message: "이전에 전송",
    description: "Activity delivery-history upper timestamp bound",
  },
  "webhooks.destinationBlocked": {
    message: "허용되지 않는 대상",
    description:
      "Delivery result label when bex refused to dial the destination: it resolves to a loopback, private, link-local or cluster-internal address (w21/046)",
  },
  "webhooks.destinationUnreachableNotice": {
    message:
      "이 주소는 인터넷에서 접근할 수 없으므로 이 주소로의 모든 전송이 실패합니다. 공개 HTTPS 주소를 사용하세요.",
    description:
      "Non-blocking notice under the webhook Destination URL when its host is a loopback, private, link-local or cluster-internal address (w21/046)",
  },
  "webhooks.transportError": {
    message: "전송 오류",
    description: "Delivery evidence label when no HTTP response was received",
  },
  "webhooks.enabledBadge": {
    message: "활성",
    description: "Detail header status badge while enabled",
  },
  "webhooks.disabledBadge": {
    message: "비활성",
    description: "Detail header status badge while disabled",
  },
  "webhooks.view": {
    message: "웹훅 보기",
    description: "Accessible label of the list row's open-detail link",
  },
  "webhooks.settingsGeneral": {
    message: "일반",
    description: "Settings tab first section heading",
  },
  "webhooks.disabledReasonAuto": {
    message: "전송이 반복해서 실패하여 자동으로 비활성화되었습니다.",
    description:
      "Reason shown beside a webhook the platform disabled after repeated delivery failures",
  },
  "webhooks.settingsStatus": {
    message: "상태",
    description: "Settings status-toggle label",
  },
  "webhooks.settingsStatusHelp": {
    message: "비활성 상태인 웹훅은 알림을 보내지 않습니다.",
    description: "Settings status-toggle helper copy",
  },
  "webhooks.settingsEvents": {
    message: "구독 중인 이벤트",
    description: "Settings events section heading",
  },
  "webhooks.saveChanges": {
    message: "변경 사항 저장",
    description: "Settings submit button (disabled until dirty)",
  },
  "webhooks.updateSuccess": {
    message: "웹훅을 업데이트했습니다",
    description: "Toast after a successful settings save",
  },
  "webhooks.updateError": {
    message: "웹훅을 업데이트하지 못했습니다",
    description: "Toast after a failed settings save",
  },
  "webhooks.secretLabel": {
    message: "서명 시크릿",
    description: "Settings signing-secret row label",
  },
  "webhooks.secretHelp": {
    message:
      "요청 페이로드에 서명하고 검증하는 데 사용하는 토큰입니다. 워크스페이스 관리자만 볼 수 있습니다.",
    description: "Settings signing-secret row help (Render's wording)",
  },
  "webhooks.secretShow": {
    message: "시크릿 표시",
    description: "Settings button that reveals the masked signing secret",
  },
  "webhooks.secretHide": {
    message: "시크릿 숨기기",
    description: "Settings button that masks the revealed signing secret",
  },
  "webhooks.deleteSection": {
    message: "웹훅 삭제",
    description: "Settings danger-zone heading",
  },
  "webhooks.deleteTypeToConfirm": {
    message: "확인하려면 아래에 다음을 입력하세요: {command}",
    description: "Type-to-confirm instruction in the delete dialog",
  },
  "webhooks.deleteCommandLabel": {
    message: "확인",
    description: "Accessible label of the type-to-confirm input",
  },
};

export default koWebhooks;
