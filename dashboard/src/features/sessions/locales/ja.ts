import type { TranslationEntry } from "@/i18n";

const ja: Record<string, TranslationEntry> = {
  "activeSessions.title": {
    message: "アクティブなセッション",
    description: "Active Sessions settings card title",
  },
  "activeSessions.description": {
    message: "現在あなたとしてログインしているブラウザーとデバイスです。",
    description: "Active Sessions settings card description",
  },
  "activeSessions.signOutOthers": {
    message: "他のセッションをログアウト",
    description: "Active Sessions sign-out-others button label",
  },
  "activeSessions.signOutOthersConfirmTitle": {
    message: "他のすべてのセッションをログアウトしますか？",
    description: "Active Sessions sign-out-others confirmation dialog title",
  },
  "activeSessions.signOutOthersConfirmBody": {
    message:
      "現在使用中のブラウザーとデバイスを除くすべてのブラウザーとデバイスを、ただちにログアウトします。",
    description: "Active Sessions sign-out-others confirmation dialog body",
  },
  "activeSessions.signOutOthersSuccess": {
    message: "他のセッションをログアウトしました",
    description: "Active Sessions sign-out-others success toast",
  },
  "activeSessions.signOutOthersError": {
    message: "他のセッションをログアウトできませんでした",
    description: "Active Sessions sign-out-others failure toast",
  },
  "activeSessions.colDevice": {
    message: "デバイス",
    description: "Active Sessions table column: device/browser",
  },
  "activeSessions.colLocation": {
    message: "場所",
    description: "Active Sessions table column: location or IP",
  },
  "activeSessions.colSignedIn": {
    message: "ログイン日時",
    description:
      "Active Sessions table column: when the session was authenticated (Kratos has no last-activity time)",
  },
  "activeSessions.current": {
    message: "このデバイス",
    description: "Active Sessions badge marking the current session's row",
  },
  "activeSessions.unknownDevice": {
    message: "不明なデバイス",
    description: "Active Sessions fallback when no user agent is recorded",
  },
  "activeSessions.emptyTitle": {
    message: "アクティブなセッションはありません",
    description: "Active Sessions empty state title",
  },
  "activeSessions.emptyBody": {
    message: "アカウントのアクティブなセッションが見つかりませんでした。",
    description: "Active Sessions empty state body",
  },
  "activeSessions.errorTitle": {
    message: "アクティブなセッションを読み込めませんでした",
    description: "Active Sessions generic error state title",
  },
  "activeSessions.errorBody": {
    message: "問題が発生しました。しばらくしてからもう一度お試しください。",
    description: "Active Sessions generic error state body",
  },
  "activeSessions.revoke": {
    message: "ログアウト",
    description: "Active Sessions row revoke confirmation button",
  },
  "activeSessions.revokeNamed": {
    message: "{device}（{location}、ログイン日時 {signedIn}）をログアウト",
    description:
      "Active Sessions row revoke button aria-label identifying the session",
  },
  "activeSessions.revokeConfirmTitle": {
    message: "このセッションをログアウトしますか？",
    description: "Active Sessions revoke confirmation dialog title",
  },
  "activeSessions.revokeConfirmBody": {
    message: "そのブラウザーまたはデバイスをただちにログアウトします。",
    description: "Active Sessions revoke confirmation dialog body",
  },
  "activeSessions.revokeCancel": {
    message: "キャンセル",
    description: "Active Sessions confirmation dialog cancel button",
  },
  "activeSessions.revokeSuccess": {
    message: "セッションをログアウトしました",
    description: "Active Sessions revoke success toast",
  },
  "activeSessions.revokeError": {
    message: "そのセッションをログアウトできませんでした",
    description: "Active Sessions revoke failure toast",
  },
};

export default ja;
