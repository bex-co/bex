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
  "audit.oauthDelegation": {
    message: "{client} · {scopes}",
    description:
      "Audit Log actor subtitle for a third-party OAuth grant (client id and canonical scopes)",
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
  "audit.planTitle": {
    message: "审计日志需要 Pro 套餐",
    description:
      "Audit Log title on a Hobby workspace (PLAN_FEATURE_REQUIRED, w8/m138)",
  },
  "audit.planBody": {
    message: "审计日志仅在 Pro 及更高套餐中提供。此工作区为 Hobby 套餐。",
    description: "Audit Log plan notice body on a Hobby workspace (w8/m138)",
  },
  "audit.exportStart": {
    message: "开始日期（UTC）",
    description: "Audit Log CSV export start-date label",
  },
  "audit.exportEnd": {
    message: "结束日期（UTC）",
    description: "Audit Log CSV export end-date label",
  },
  "audit.exportCSV": {
    message: "导出为 CSV",
    description: "Audit Log CSV export button (Render's label)",
  },
  "audit.exportHint": {
    message: "最多导出 {days} 天的事件：时间、操作者、事件、状态和元数据。",
    description: "Audit Log CSV export help text; {days} is the longest range",
  },
  "audit.exportRangeOrder": {
    message: "结束日期不能早于开始日期。",
    description: "Audit Log CSV export: end before start",
  },
  "audit.exportRangeTooLong": {
    message: "单次导出最多 {days} 天。",
    description:
      "Audit Log CSV export: range over the limit; {days} is the limit",
  },
  "audit.exportTooLarge": {
    message: "该时间范围内的事件过多，无法一次导出，请缩短范围。",
    description: "Audit Log CSV export: row budget exceeded",
  },
  "audit.exportFailed": {
    message: "导出失败，请重试。",
    description: "Audit Log CSV export: request failed",
  },
  "audit.exportRangeMissing": {
    message: "请选择开始日期和结束日期。",
    description: "Audit Log CSV export: a date is empty",
  },
};

export default zhAudit;
