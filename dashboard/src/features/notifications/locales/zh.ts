import type { TranslationEntry } from "@/i18n";

const zhNotifications: Record<string, TranslationEntry> = {
  "notifications.title": {
    message: "通知",
    description: "Settings Notifications section card title",
  },
  "notifications.description": {
    message:
      "当你的某个服务开始部署、部署成功或失败时通过邮件通知你。这些是你的个人偏好设置——工作区内每位成员都可独立设置自己的通知偏好。",
    description: "Settings Notifications section card description",
  },
  "notifications.deployStarted": {
    message: "部署开始",
    description: "Toggle label",
  },
  "notifications.deployStartedHint": {
    message: "部署开始时通过邮件通知我。",
    description: "Toggle hint",
  },
  "notifications.deploySucceeded": {
    message: "部署成功",
    description: "Toggle label",
  },
  "notifications.deploySucceededHint": {
    message: "部署上线时通过邮件通知我。",
    description: "Toggle hint",
  },
  "notifications.deployFailed": {
    message: "部署失败",
    description: "Toggle label",
  },
  "notifications.deployFailedHint": {
    message: "部署失败时通过邮件通知我。",
    description: "Toggle hint",
  },
  "notifications.errorTitle": {
    message: "无法加载通知设置",
    description: "Generic error title",
  },
  "notifications.errorBody": {
    message: "出现问题，请重试。",
    description: "Generic error body",
  },
  "notifications.updateError": {
    message: "无法保存你的偏好设置",
    description: "Toast on a failed update",
  },
};

export default zhNotifications;
