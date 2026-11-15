import type { TranslationEntry } from "@/i18n";

const ko: Record<string, TranslationEntry> = {
  "activeSessions.title": {
    message: "활성 세션",
    description: "Active Sessions settings card title",
  },
  "activeSessions.description": {
    message: "현재 내 계정으로 로그인되어 있는 브라우저와 기기입니다.",
    description: "Active Sessions settings card description",
  },
  "activeSessions.signOutOthers": {
    message: "다른 세션 로그아웃",
    description: "Active Sessions sign-out-others button label",
  },
  "activeSessions.signOutOthersConfirmTitle": {
    message: "다른 모든 세션을 로그아웃할까요?",
    description: "Active Sessions sign-out-others confirmation dialog title",
  },
  "activeSessions.signOutOthersConfirmBody": {
    message:
      "지금 사용 중인 브라우저와 기기를 제외한 모든 브라우저와 기기가 즉시 로그아웃됩니다.",
    description: "Active Sessions sign-out-others confirmation dialog body",
  },
  "activeSessions.signOutOthersSuccess": {
    message: "다른 세션을 로그아웃했습니다",
    description: "Active Sessions sign-out-others success toast",
  },
  "activeSessions.signOutOthersError": {
    message: "다른 세션을 로그아웃하지 못했습니다",
    description: "Active Sessions sign-out-others failure toast",
  },
  "activeSessions.colDevice": {
    message: "기기",
    description: "Active Sessions table column: device/browser",
  },
  "activeSessions.colLocation": {
    message: "위치",
    description: "Active Sessions table column: location or IP",
  },
  "activeSessions.colSignedIn": {
    message: "로그인 시간",
    description:
      "Active Sessions table column: when the session was authenticated (Kratos has no last-activity time)",
  },
  "activeSessions.current": {
    message: "이 기기",
    description: "Active Sessions badge marking the current session's row",
  },
  "activeSessions.unknownDevice": {
    message: "알 수 없는 기기",
    description: "Active Sessions fallback when no user agent is recorded",
  },
  "activeSessions.emptyTitle": {
    message: "활성 세션 없음",
    description: "Active Sessions empty state title",
  },
  "activeSessions.emptyBody": {
    message: "내 계정의 활성 세션을 찾을 수 없습니다.",
    description: "Active Sessions empty state body",
  },
  "activeSessions.errorTitle": {
    message: "활성 세션을 불러오지 못했습니다",
    description: "Active Sessions generic error state title",
  },
  "activeSessions.errorBody": {
    message: "문제가 발생했습니다. 잠시 후 다시 시도하세요.",
    description: "Active Sessions generic error state body",
  },
  "activeSessions.revoke": {
    message: "로그아웃",
    description: "Active Sessions row revoke confirmation button",
  },
  "activeSessions.revokeNamed": {
    message: "{device}, {location}, {signedIn}에 로그인한 세션 로그아웃",
    description:
      "Active Sessions row revoke button aria-label identifying the session",
  },
  "activeSessions.revokeConfirmTitle": {
    message: "이 세션을 로그아웃할까요?",
    description: "Active Sessions revoke confirmation dialog title",
  },
  "activeSessions.revokeConfirmBody": {
    message: "해당 브라우저 또는 기기가 즉시 로그아웃됩니다.",
    description: "Active Sessions revoke confirmation dialog body",
  },
  "activeSessions.revokeCancel": {
    message: "취소",
    description: "Active Sessions confirmation dialog cancel button",
  },
  "activeSessions.revokeSuccess": {
    message: "세션을 로그아웃했습니다",
    description: "Active Sessions revoke success toast",
  },
  "activeSessions.revokeError": {
    message: "해당 세션을 로그아웃하지 못했습니다",
    description: "Active Sessions revoke failure toast",
  },
};

export default ko;
