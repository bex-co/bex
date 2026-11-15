import type { TranslationEntry } from "@/i18n";

const koNotifications: Record<string, TranslationEntry> = {
  "notifications.title": {
    message: "알림",
    description: "Settings Notifications section card title",
  },
  "notifications.description": {
    message:
      "서비스 배포가 시작되거나 성공하거나 실패할 때, 그리고 런타임에 문제가 생겼을 때 이메일로 알림을 받습니다. 이 설정은 개인 설정이며, 워크스페이스 멤버마다 따로 지정합니다.",
    description: "Settings Notifications section card description",
  },
  "notifications.deployStarted": {
    message: "배포 시작됨",
    description: "Toggle label",
  },
  "notifications.deployStartedHint": {
    message:
      "배포가 시작되면 이메일을 보냅니다. 모든 알림을 보내도록 설정된 서비스에만 전송됩니다.",
    description: "Toggle hint",
  },
  "notifications.deploySucceeded": {
    message: "배포 성공",
    description: "Toggle label",
  },
  "notifications.deploySucceededHint": {
    message:
      "배포가 라이브 상태가 되면 이메일을 보냅니다. 모든 알림을 보내도록 설정된 서비스에만 전송됩니다.",
    description: "Toggle hint",
  },
  "notifications.deployFailed": {
    message: "실패",
    description: "Toggle label",
  },
  "notifications.deployFailedHint": {
    message:
      "배포가 실패하거나, 서비스가 비정상 상태가 되거나, Cron 실행이 실패하거나, 이미지 가져오기가 실패하거나, Blueprint 동기화가 실패하면 이메일을 보냅니다.",
    description: "Toggle hint",
  },
  "notifications.errorTitle": {
    message: "알림 설정을 불러오지 못했습니다",
    description: "Generic error title",
  },
  "notifications.errorBody": {
    message: "문제가 발생했습니다. 다시 시도하세요.",
    description: "Generic error body",
  },
  "notifications.updateError": {
    message: "설정을 저장하지 못했습니다",
    description: "Toast on a failed update",
  },
  "notifications.pushTitle": {
    message: "모바일 푸시",
    description: "Push settings card title",
  },
  "notifications.bexExtension": {
    message: "bex 확장",
    description: "Badge distinguishing native push from Render parity",
  },
  "notifications.pushDescription": {
    message:
      "bex 모바일 알림, 긴급도, 전송 시간대, 서비스별 예외를 설정합니다. 위의 이메일 설정은 변경되지 않습니다.",
    description: "Push settings description",
  },
  "notifications.pushEnabled": {
    message: "모바일 푸시 사용",
    description: "Push enabled toggle",
  },
  "notifications.pushEnabledHint": {
    message: "등록된 bex 모바일 기기로 알림을 보냅니다.",
    description: "Push enabled hint",
  },
  "notifications.pushUnavailable": {
    message: "이 bex 서버에는 푸시 전송이 구성되어 있지 않습니다",
    description: "Disabled server status",
  },
  "notifications.pushUnavailableHint": {
    message:
      "정책은 지금 준비하고 저장할 수 있지만, 운영자가 푸시 제공자를 구성하기 전까지는 등록된 기기가 알림을 받을 수 없습니다.",
    description: "Disabled server status hint",
  },
  "notifications.webPushTitle": {
    message: "브라우저 알림",
    description: "Web push settings card title",
  },
  "notifications.webPushDescription": {
    message:
      "이 브라우저에서도 같은 알림을 받습니다. 이벤트 필터와 방해 금지 시간은 모바일 푸시에서 설정한 값이 그대로 적용되며, 별도의 정책은 없습니다.",
    description: "Web push settings description",
  },
  "notifications.webPushUnsupported": {
    message: "이 브라우저는 웹 푸시를 지원하지 않습니다.",
    description: "Unsupported browser",
  },
  "notifications.webPushUnconfigured": {
    message:
      "이 bex 서버에서는 브라우저 푸시를 사용하도록 설정하지 않았습니다.",
    description: "Server VAPID unset",
  },
  "notifications.webPushDenied": {
    message:
      "이 사이트의 알림이 차단되어 있습니다. 브라우저 설정에서 허용한 후 다시 시도하세요.",
    description: "Permission denied",
  },
  "notifications.webPushError": {
    message: "브라우저 알림을 업데이트하지 못했습니다. 다시 시도하세요.",
    description: "Subscribe error",
  },
  "notifications.webPushOn": {
    message: "이 브라우저에서 bex 알림을 받습니다.",
    description: "Subscribed state",
  },
  "notifications.webPushOff": {
    message: "이 브라우저는 구독 중이 아닙니다.",
    description: "Unsubscribed state",
  },
  "notifications.webPushEnable": {
    message: "이 브라우저에서 사용",
    description: "Subscribe button",
  },
  "notifications.webPushDisable": {
    message: "이 브라우저에서 사용 안 함",
    description: "Unsubscribe button",
  },
  "notifications.pushEvents": {
    message: "이벤트 필터",
    description: "Push event section",
  },
  "notifications.pushEventsHint": {
    message:
      "최소 긴급도 이상인 선택한 이벤트 유형만 푸시 알림을 보낼 수 있습니다.",
    description: "Push events hint",
  },
  "notifications.pushEventNotSentYet": {
    message: "아직 전송되지 않음: 현재는 이 이벤트를 보내는 기능이 없습니다.",
    description:
      "Under a push event that no part of bex emits yet (it is reserved for an upcoming feature); selecting it is kept but has no effect",
  },
  "notifications.pushEventNotSentYetShort": {
    message: "아직 전송되지 않음",
    description:
      "Compact form of the not-sent-yet note, under a push event in a per-service override's event grid",
  },
  "notifications.pushEventFiltered": {
    message: "최소 긴급도가 {minimum}인 동안에는 푸시되지 않습니다.",
    description:
      "Under a selected push event whose urgency is below the minimum ({minimum} = urgency name, e.g. 'Important')",
  },
  "notifications.pushFilteredTitle": {
    message: "선택한 일부 이벤트는 푸시되지 않습니다",
    description:
      "Heading of the save-area summary listing selected push events the minimum urgency drops",
  },
  "notifications.pushFilteredEvent": {
    message:
      "{event}의 긴급도는 {urgency}이므로 최소 긴급도가 {minimum}인 동안에는 푸시되지 않습니다.",
    description:
      "One summary line ({event} = event name, {urgency} = its urgency, {minimum} = the minimum urgency)",
  },
  "notifications.pushFilteredServiceEvent": {
    message:
      "{service}: {event}의 긴급도는 {urgency}이므로 이 서비스의 최소 긴급도가 {minimum}인 동안에는 푸시되지 않습니다.",
    description:
      "One summary line for a service override ({service} = service name; others as pushFilteredEvent)",
  },
  "notifications.pushOverrideFiltered_other": {
    message:
      "이 서비스의 최소 긴급도가 {minimum}인 동안에는 선택한 이벤트 {count}개가 푸시되지 않습니다.",
    description:
      "Under a service override whose own policy drops selected events ({minimum} = urgency name)",
  },
  "notifications.pushEventDeployFailed": {
    message: "배포 실패",
    description: "Push event",
  },
  "notifications.pushEventServerFailed": {
    message: "서비스 비정상 종료",
    description: "Push event",
  },
  "notifications.pushEventCronFailed": {
    message: "Cron 실행 실패",
    description: "Push event",
  },
  "notifications.pushEventDeployStarted": {
    message: "배포 시작됨",
    description: "Push event",
  },
  "notifications.pushEventDeploySucceeded": {
    message: "배포 성공",
    description: "Push event",
  },
  "notifications.pushEventServerAvailable": {
    message: "서비스 복구됨",
    description: "Push event",
  },
  "notifications.pushEventServiceSuspended": {
    message: "서비스 일시 중지됨",
    description: "Push event",
  },
  "notifications.pushEventServiceResumed": {
    message: "서비스 재개됨",
    description: "Push event",
  },
  "notifications.pushEventAgentNeedsDecision": {
    message: "에이전트가 결정을 요청함",
    description: "Push event",
  },
  "notifications.pushEventAgentPrReady": {
    message: "에이전트 PR 준비됨",
    description: "Push event",
  },
  "notifications.pushEventAgentFailed": {
    message: "에이전트 세션 실패",
    description: "Push event",
  },
  "notifications.pushEventAgentAutomationPaused": {
    message: "자동화 일시 중지됨",
    description: "Push event: the platform paused an agent automation",
  },
  "notifications.pushEventPostgresUnavailable": {
    message: "데이터베이스 사용 불가",
    description: "Push event",
  },
  "notifications.pushEventPostgresAvailable": {
    message: "데이터베이스 복구됨",
    description: "Push event",
  },
  "notifications.pushEventKeyValueUnhealthy": {
    message: "Key Value 비정상",
    description: "Push event",
  },
  "notifications.pushEventKeyValueAvailable": {
    message: "Key Value 복구됨",
    description: "Push event",
  },
  "notifications.pushEventPostgresBackupFailed": {
    message: "데이터베이스 백업 실패",
    description: "Push event",
  },
  "notifications.pushEventPostgresRestoreFailed": {
    message: "데이터베이스 복원 실패",
    description: "Push event",
  },
  "notifications.pushEventPostgresUpgradeFailed": {
    message: "데이터베이스 업그레이드 실패",
    description: "Push event",
  },
  "notifications.pushMinimumUrgency": {
    message: "최소 긴급도",
    description: "Urgency field",
  },
  "notifications.pushUrgencyRoutine": {
    message: "일반",
    description: "Urgency option",
  },
  "notifications.pushUrgencyImportant": {
    message: "중요",
    description: "Urgency option",
  },
  "notifications.pushUrgencyCritical": {
    message: "긴급",
    description: "Urgency option",
  },
  "notifications.pushTimezone": {
    message: "IANA 시간대",
    description: "Timezone field",
  },
  "notifications.pushTimezonePlaceholder": {
    message: "America/Los_Angeles",
    description: "Timezone placeholder",
  },
  "notifications.pushMaxDeferral": {
    message: "최대 지연 시간(시간)",
    description: "Deferral field",
  },
  "notifications.pushWorkingHours": {
    message: "근무 시간",
    description: "Working hours heading",
  },
  "notifications.pushWorkingHoursHint": {
    message:
      "설정하면 긴급이 아닌 알림은 이 시간대 중 하나가 시작될 때까지 보류됩니다.",
    description: "Working hours hint",
  },
  "notifications.pushQuietHours": {
    message: "방해 금지 시간",
    description: "Quiet hours heading",
  },
  "notifications.pushQuietHoursHint": {
    message: "방해 금지 시간대에는 긴급이 아닌 알림이 보류됩니다.",
    description: "Quiet hours hint",
  },
  "notifications.weekdayMonday": {
    message: "월",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayTuesday": {
    message: "화",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayWednesday": {
    message: "수",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayThursday": {
    message: "목",
    description: "Weekday abbreviation",
  },
  "notifications.weekdayFriday": {
    message: "금",
    description: "Weekday abbreviation",
  },
  "notifications.weekdaySaturday": {
    message: "토",
    description: "Weekday abbreviation",
  },
  "notifications.weekdaySunday": {
    message: "일",
    description: "Weekday abbreviation",
  },
  "notifications.pushRangeStart": {
    message: "시작 시간",
    description: "Accessible time input label",
  },
  "notifications.pushRangeEnd": {
    message: "종료 시간",
    description: "Accessible time input label",
  },
  "notifications.pushRemoveRange": {
    message: "시간대 제거",
    description: "Accessible remove button label",
  },
  "notifications.pushAddRange": {
    message: "시간대 추가",
    description: "Add range button",
  },
  "notifications.pushOverrides": {
    message: "서비스별 재정의",
    description: "Overrides heading",
  },
  "notifications.pushOverridesHint": {
    message:
      "비워 둔 항목은 위의 설정을 따르며, 이벤트 목록을 빈 값으로 지정하면 해당 서비스의 알림이 꺼집니다.",
    description: "Override semantics hint",
  },
  "notifications.pushRemoveOverride": {
    message: "서비스 재정의 제거",
    description: "Accessible remove override label",
  },
  "notifications.pushOverrideEnabled": {
    message: "전송",
    description: "Override enabled field",
  },
  "notifications.pushInherit": {
    message: "상속",
    description: "Override option",
  },
  "notifications.pushOn": {
    message: "켜짐",
    description: "Override option",
  },
  "notifications.pushOff": {
    message: "꺼짐",
    description: "Override option",
  },
  "notifications.pushExactEvents": {
    message: "이벤트 목록을 직접 지정하여 재정의",
    description: "Exact event filter toggle",
  },
  "notifications.pushAddOverride": {
    message: "서비스 재정의 추가…",
    description: "Add override selector",
  },
  "notifications.pushSave": {
    message: "푸시 설정 저장",
    description: "Save button",
  },
  "notifications.pushUnsaved": {
    message: "저장되지 않은 변경 사항",
    description:
      "Hint beside Save push settings while the push form differs from the saved settings (w4/435)",
  },
  "notifications.pushDiscardTitle": {
    message: "푸시 설정 변경 사항을 취소하시겠습니까?",
    description:
      "Navigation guard title when the push form has unsaved changes (w4/435)",
  },
  "notifications.pushDiscardBody": {
    message: "변경 사항이 저장되지 않았으며 페이지를 나가면 사라집니다.",
    description: "Navigation guard explanation for unsaved push settings",
  },
  "notifications.pushKeepEditing": {
    message: "계속 편집",
    description: "Cancel the navigation and stay on the push form",
  },
  "notifications.pushDiscard": {
    message: "취소하고 나가기",
    description: "Confirm leaving and dropping unsaved push settings",
  },
  "notifications.pushSaved": {
    message: "푸시 설정이 저장되었습니다",
    description: "Success toast",
  },
  "notifications.pushUpdateError": {
    message: "푸시 설정을 저장하지 못했습니다",
    description: "Failure toast",
  },
  "notifications.pushErrorTitle": {
    message: "푸시 설정을 불러오지 못했습니다",
    description: "Push load error title",
  },
  "notifications.pushInvalidTimezone": {
    message: "America/Los_Angeles와 같은 올바른 IANA 시간대를 입력하세요.",
    description: "Timezone validation",
  },
  "notifications.pushInvalidDeferral": {
    message: "최대 지연 시간은 0.5시간에서 168시간 사이여야 합니다.",
    description: "Deferral validation",
  },
  "notifications.pushInvalidEvents": {
    message: "이벤트 필터에 지원되지 않는 값이 있습니다.",
    description: "Event validation",
  },
  "notifications.pushInvalidUrgency": {
    message: "올바른 긴급도를 선택하세요.",
    description: "Urgency validation",
  },
  "notifications.pushInvalidRange": {
    message:
      "모든 시간대에는 요일, 올바른 시작/종료 시간이 필요하며 시작과 종료가 달라야 합니다.",
    description: "Range validation",
  },
  "notifications.pushTooManyRules": {
    message: "스케줄 시간대 또는 서비스 재정의가 너무 많습니다.",
    description: "Rule count validation",
  },
  "notifications.pushInvalidService": {
    message:
      "서비스 재정의는 중복될 수 없으며 올바른 서비스를 가리켜야 합니다.",
    description: "Service validation",
  },
  "notifications.pushEmptyOverride": {
    message:
      "각 서비스 재정의는 전송 여부, 긴급도, 이벤트 중 하나 이상을 변경해야 합니다.",
    description: "Empty override validation",
  },
  "notifications.workspaceTitle": {
    message: "알림",
    description:
      "Workspace Settings → Notifications card title (the workspace notification policy)",
  },
  "notifications.workspaceDescription": {
    message:
      "모든 서비스의 기본값입니다. 서비스는 설정에서 이를 재정의할 수 있습니다.",
    description: "Workspace Settings → Notifications card description",
  },
  "notifications.workspaceReadOnly": {
    message: "워크스페이스 관리자만 이 기본값을 변경할 수 있습니다.",
    description: "Workspace notification policy: read-only note for non-admins",
  },
  "notifications.workspaceEmailLabel": {
    message: "이메일 알림",
    description: "Workspace notification policy: email destination switch",
  },
  "notifications.workspaceEmailHint": {
    message: "끄면 서비스 설정과 관계없이 알림 이메일을 보내지 않습니다.",
    description: "Workspace notification policy: email destination hint",
  },
  "notifications.workspaceLevelLabel": {
    message: "서비스 기본 알림",
    description: "Workspace notification policy: level radio group label",
  },
  "notifications.workspaceLevelHint": {
    message: "워크스페이스 기본값을 사용하는 서비스가 보내는 알림입니다.",
    description: "Workspace notification policy: level radio group hint",
  },
  "notifications.levelAll": {
    message: "모든 알림",
    description: "Notification level: every notification",
  },
  "notifications.levelAllHint": {
    message: "실패 알림과 함께 배포 시작, 배포 라이브, 복구 알림을 보냅니다.",
    description: "Notification level hint: all",
  },
  "notifications.levelFailure": {
    message: "실패 알림만",
    description: "Notification level: failures only (Render's default)",
  },
  "notifications.levelFailureHint": {
    message: "배포 실패, 서비스 비정상, Cron 실행 실패, 디스크 가득 참.",
    description: "Notification level hint: failure",
  },
  "notifications.levelNone": {
    message: "없음",
    description: "Notification level: nothing",
  },
  "notifications.levelNoneHint": {
    message: "이 서비스에는 알림을 보내지 않습니다.",
    description: "Notification level hint: none",
  },
  "notifications.workspaceSave": {
    message: "저장",
    description: "Workspace notification policy: save button",
  },
  "notifications.workspaceSaved": {
    message: "알림 기본값이 저장되었습니다",
    description: "Workspace notification policy: success toast",
  },
  "notifications.workspaceSaveFailed": {
    message: "알림 기본값을 저장하지 못했습니다. 다시 시도하세요.",
    description: "Workspace notification policy: save error",
  },
  "notifications.workspaceLoadFailed": {
    message: "워크스페이스 알림 기본값을 불러오지 못했습니다.",
    description: "Workspace notification policy: load error",
  },
  "notifications.workspaceRetry": {
    message: "다시 시도",
    description: "Workspace notification policy: retry button",
  },
  "notifications.overridesTitle": {
    message: "알림 재정의",
    description:
      "Workspace notifications: overrides list title (Render's label)",
  },
  "notifications.overridesDescription": {
    message: "알림 설정이 워크스페이스 기본값과 다른 서비스입니다.",
    description: "Workspace notifications: overrides list description",
  },
  "notifications.overridesEmpty": {
    message: "워크스페이스 기본값을 재정의하는 서비스가 없습니다.",
    description: "Workspace notifications: no overrides",
  },
  "notifications.overridesError": {
    message: "기본값을 재정의하는 서비스를 불러오지 못했습니다.",
    description: "Workspace notifications: overrides failed to load",
  },
};

export default koNotifications;
