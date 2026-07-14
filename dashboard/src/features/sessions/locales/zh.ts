import type { TranslationEntry } from "@/i18n";

const zh: Record<string, TranslationEntry> = {
  "activeSessions.title": {
    message: "活跃会话",
    description: "Active Sessions settings card title",
  },
  "activeSessions.description": {
    message: "当前以您身份登录的浏览器和设备。",
    description: "Active Sessions settings card description",
  },
  "activeSessions.signOutOthers": {
    message: "退出其他会话",
    description: "Active Sessions sign-out-others button label",
  },
  "activeSessions.signOutOthersConfirmTitle": {
    message: "退出所有其他会话？",
    description: "Active Sessions sign-out-others confirmation dialog title",
  },
  "activeSessions.signOutOthersConfirmBody": {
    message: "此操作将立即退出除当前正在使用的浏览器或设备之外的所有会话。",
    description: "Active Sessions sign-out-others confirmation dialog body",
  },
  "activeSessions.signOutOthersSuccess": {
    message: "已退出其他会话",
    description: "Active Sessions sign-out-others success toast",
  },
  "activeSessions.signOutOthersError": {
    message: "无法退出其他会话",
    description: "Active Sessions sign-out-others failure toast",
  },
  "activeSessions.colDevice": {
    message: "设备",
    description: "Active Sessions table column: device/browser",
  },
  "activeSessions.colLocation": {
    message: "位置",
    description: "Active Sessions table column: location or IP",
  },
  "activeSessions.colLastActive": {
    message: "最近活动",
    description: "Active Sessions table column: last authenticated",
  },
  "activeSessions.current": {
    message: "当前设备",
    description: "Active Sessions badge marking the current session's row",
  },
  "activeSessions.unknownDevice": {
    message: "未知设备",
    description: "Active Sessions fallback when no user agent is recorded",
  },
  "activeSessions.emptyTitle": {
    message: "暂无活跃会话",
    description: "Active Sessions empty state title",
  },
  "activeSessions.emptyBody": {
    message: "未找到您账户的活跃会话。",
    description: "Active Sessions empty state body",
  },
  "activeSessions.errorTitle": {
    message: "无法加载活跃会话",
    description: "Active Sessions generic error state title",
  },
  "activeSessions.errorBody": {
    message: "出了点问题，请稍后再试。",
    description: "Active Sessions generic error state body",
  },
  "activeSessions.revoke": {
    message: "退出登录",
    description: "Active Sessions row revoke button label",
  },
  "activeSessions.revokeConfirmTitle": {
    message: "退出此会话？",
    description: "Active Sessions revoke confirmation dialog title",
  },
  "activeSessions.revokeConfirmBody": {
    message: "此操作将立即使该浏览器或设备退出登录。",
    description: "Active Sessions revoke confirmation dialog body",
  },
  "activeSessions.revokeCancel": {
    message: "取消",
    description: "Active Sessions confirmation dialog cancel button",
  },
  "activeSessions.revokeSuccess": {
    message: "已退出该会话",
    description: "Active Sessions revoke success toast",
  },
  "activeSessions.revokeError": {
    message: "无法退出该会话",
    description: "Active Sessions revoke failure toast",
  },
};

export default zh;
