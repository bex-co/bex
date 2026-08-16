import type { TranslationEntry } from "@/i18n";

const zhAudit: Record<string, TranslationEntry> = {
  "audit.title": {
    message: "审计日志",
    description: "Settings Audit Log section card title",
  },
  "audit.description": {
    message:
      "此工作区中谁做了什么，是否被允许——按时间从新到旧排列。仅工作区管理员可见。",
    description: "Settings Audit Log section card description",
  },
  "audit.columnTimestamp": {
    message: "时间",
    description: "Audit Log table column header",
  },
  "audit.columnActor": {
    message: "操作者",
    description: "Audit Log table column header — who performed the action",
  },
  "audit.columnAction": {
    message: "操作",
    description: "Audit Log table column header — the verb performed",
  },
  "audit.columnStatus": {
    message: "状态",
    description: "Audit Log table column header — allowed or denied",
  },
  "audit.columnResource": {
    message: "资源",
    description: "Audit Log table column header — the object acted on",
  },
  "audit.statusAllowed": {
    message: "已允许",
    description: "Audit Log status badge for a successful action",
  },
  "audit.statusDenied": {
    message: "已拒绝",
    description: "Audit Log status badge for a denied action",
  },
  "audit.actorUnknown": {
    message: "未知",
    description:
      "Audit Log actor cell placeholder for an unauthenticated caller",
  },
  "audit.loadMore": {
    message: "加载更多",
    description: "Button that fetches the next page of audit events",
  },
  "audit.emptyTitle": {
    message: "暂无审计事件",
    description: "Audit Log empty-state title",
  },
  "audit.emptyBody": {
    message: "此工作区中的写操作将显示在这里。",
    description: "Audit Log empty-state body",
  },
  "audit.errorTitle": {
    message: "无法加载审计日志",
    description: "Audit Log generic error title",
  },
  "audit.errorBody": {
    message: "加载此工作区的审计记录时出错，请重试。",
    description: "Audit Log generic error body",
  },
  "audit.unavailableTitle": {
    message: "审计日志未配置",
    description:
      "Audit Log state when the control-plane store isn't wired (503)",
  },
  "audit.unavailableBody": {
    message: "此 bex 部署尚未配置审计日志存储。",
    description: "Audit Log unavailable-state body",
  },
};

export default zhAudit;
