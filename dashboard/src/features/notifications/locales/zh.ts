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
  "notifications.pushTitle": {
    message: "移动推送",
    description: "Push settings card title",
  },
  "notifications.bexExtension": {
    message: "bex 扩展",
    description: "Native extension badge",
  },
  "notifications.pushDescription": {
    message:
      "管理 bex 移动端提醒、紧急程度、投递时段和服务例外；不会更改上方的邮件偏好。",
    description: "Push settings description",
  },
  "notifications.pushEnabled": {
    message: "启用移动推送",
    description: "Push enabled toggle",
  },
  "notifications.pushEnabledHint": {
    message: "向你已注册的 bex 移动设备发送提醒。",
    description: "Push enabled hint",
  },
  "notifications.pushUnavailable": {
    message: "此 bex 服务器尚未配置推送投递",
    description: "Disabled server status",
  },
  "notifications.pushUnavailableHint": {
    message:
      "你仍可预先保存策略，但在运维人员配置推送服务之前，已注册设备不会收到通知。",
    description: "Disabled server status hint",
  },
  "notifications.pushEvents": {
    message: "事件筛选",
    description: "Push event section",
  },
  "notifications.pushEventsHint": {
    message: "只有选中的事件类型会生成推送通知。",
    description: "Push event hint",
  },
  "notifications.pushEventDeployFailed": {
    message: "部署失败",
    description: "Push event",
  },
  "notifications.pushEventServerFailed": {
    message: "服务崩溃",
    description: "Push event",
  },
  "notifications.pushEventCronFailed": {
    message: "定时任务失败",
    description: "Push event",
  },
  "notifications.pushEventDeployStarted": {
    message: "部署开始",
    description: "Push event",
  },
  "notifications.pushEventDeploySucceeded": {
    message: "部署成功",
    description: "Push event",
  },
  "notifications.pushEventUsageThreshold": {
    message: "用量阈值",
    description: "Push event",
  },
  "notifications.pushEventAgentNeedsDecision": {
    message: "智能体需要决策",
    description: "Push event",
  },
  "notifications.pushEventAgentPrReady": {
    message: "智能体 PR 已就绪",
    description: "Push event",
  },
  "notifications.pushMinimumUrgency": {
    message: "最低紧急程度",
    description: "Urgency field",
  },
  "notifications.pushUrgencyRoutine": {
    message: "常规",
    description: "Urgency option",
  },
  "notifications.pushUrgencyImportant": {
    message: "重要",
    description: "Urgency option",
  },
  "notifications.pushUrgencyCritical": {
    message: "紧急",
    description: "Urgency option",
  },
  "notifications.pushTimezone": {
    message: "IANA 时区",
    description: "Timezone field",
  },
  "notifications.pushTimezonePlaceholder": {
    message: "Asia/Shanghai",
    description: "Timezone placeholder",
  },
  "notifications.pushMaxDeferral": {
    message: "最长延迟（小时）",
    description: "Deferral field",
  },
  "notifications.pushWorkingHours": {
    message: "工作时段",
    description: "Working hours heading",
  },
  "notifications.pushWorkingHoursHint": {
    message: "设置后，非紧急提醒会等待到这些时段。",
    description: "Working hours hint",
  },
  "notifications.pushQuietHours": {
    message: "免打扰时段",
    description: "Quiet hours heading",
  },
  "notifications.pushQuietHoursHint": {
    message: "免打扰时段内会延迟非紧急提醒。",
    description: "Quiet hours hint",
  },
  "notifications.weekdayMonday": { message: "周一", description: "Weekday" },
  "notifications.weekdayTuesday": { message: "周二", description: "Weekday" },
  "notifications.weekdayWednesday": { message: "周三", description: "Weekday" },
  "notifications.weekdayThursday": { message: "周四", description: "Weekday" },
  "notifications.weekdayFriday": { message: "周五", description: "Weekday" },
  "notifications.weekdaySaturday": { message: "周六", description: "Weekday" },
  "notifications.weekdaySunday": { message: "周日", description: "Weekday" },
  "notifications.pushRangeStart": {
    message: "开始时间",
    description: "Accessible label",
  },
  "notifications.pushRangeEnd": {
    message: "结束时间",
    description: "Accessible label",
  },
  "notifications.pushRemoveRange": {
    message: "删除时段",
    description: "Accessible label",
  },
  "notifications.pushAddRange": {
    message: "添加时段",
    description: "Add range",
  },
  "notifications.pushOverrides": {
    message: "按服务覆盖",
    description: "Overrides heading",
  },
  "notifications.pushOverridesHint": {
    message: "未设置的字段继承上方配置；明确的空事件列表会静音该服务。",
    description: "Override semantics",
  },
  "notifications.pushRemoveOverride": {
    message: "删除服务覆盖",
    description: "Accessible label",
  },
  "notifications.pushOverrideEnabled": {
    message: "投递",
    description: "Override field",
  },
  "notifications.pushInherit": {
    message: "继承",
    description: "Override option",
  },
  "notifications.pushOn": { message: "开启", description: "Override option" },
  "notifications.pushOff": { message: "关闭", description: "Override option" },
  "notifications.pushExactEvents": {
    message: "使用精确事件列表覆盖",
    description: "Exact events toggle",
  },
  "notifications.pushAddOverride": {
    message: "添加服务覆盖…",
    description: "Add override",
  },
  "notifications.pushSave": {
    message: "保存推送设置",
    description: "Save button",
  },
  "notifications.pushSaved": {
    message: "推送设置已保存",
    description: "Success toast",
  },
  "notifications.pushUpdateError": {
    message: "无法保存推送设置",
    description: "Failure toast",
  },
  "notifications.pushErrorTitle": {
    message: "无法加载推送设置",
    description: "Load error",
  },
  "notifications.pushInvalidTimezone": {
    message: "请输入有效的 IANA 时区，例如 Asia/Shanghai。",
    description: "Validation",
  },
  "notifications.pushInvalidDeferral": {
    message: "最长延迟必须介于 1 秒和 168 小时之间。",
    description: "Validation",
  },
  "notifications.pushInvalidEvents": {
    message: "事件筛选包含不支持的值。",
    description: "Validation",
  },
  "notifications.pushInvalidUrgency": {
    message: "请选择有效的紧急程度。",
    description: "Validation",
  },
  "notifications.pushInvalidRange": {
    message: "每个时段都需要星期、有效的开始和结束时间，且两者不能相同。",
    description: "Validation",
  },
  "notifications.pushTooManyRules": {
    message: "时段或服务覆盖规则过多。",
    description: "Validation",
  },
  "notifications.pushInvalidService": {
    message: "服务覆盖必须唯一且引用有效服务。",
    description: "Validation",
  },
  "notifications.pushEmptyOverride": {
    message: "每个服务覆盖都必须更改投递、紧急程度或事件。",
    description: "Validation",
  },
};

export default zhNotifications;
