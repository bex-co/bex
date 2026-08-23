import type { TranslationEntry } from "@/i18n";

const zhLogs: Record<string, TranslationEntry> = {
  "logs.rangeLabel": {
    message: "日志历史范围",
    description: "Accessible label for the relative log-history range controls",
  },
  "logs.rangeHistoryNote": {
    message: "时间范围仅限制历史记录；实时模式会继续追加新日志。",
    description:
      "Explains how the bounded history window interacts with live tail",
  },
  "logs.typeLabel": {
    message: "日志类型",
    description: "Accessible label for the log-type filter dropdown",
  },
  "logs.typeAll": {
    message: "全部日志",
    description: "Log-type filter option: no type filter",
  },
  "logs.typeApplication": {
    message: "应用日志",
    description: "Log-type filter option: application (app) logs",
  },
  "logs.typeRequest": {
    message: "请求日志",
    description:
      "Log-type filter option: request logs (Traefik access lines, from the durable store)",
  },
  "logs.levelLabel": {
    message: "级别",
    description: "Accessible label for the log-level filter dropdown",
  },
  "logs.levelAll": {
    message: "全部级别",
    description: "Level filter option: no level filter",
  },
  "logs.methodLabel": {
    message: "方法",
    description: "Accessible label for the HTTP-method filter dropdown",
  },
  "logs.methodAll": {
    message: "全部方法",
    description: "Method filter option: no method filter",
  },
  "logs.statusCodeLabel": {
    message: "状态码",
    description: "Accessible label for the HTTP-status filter dropdown",
  },
  "logs.statusCodeAll": {
    message: "全部状态",
    description: "Status-code filter option: no status filter",
  },
  "logs.instanceLabel": {
    message: "实例",
    description: "Accessible label for the instance (replica) filter dropdown",
  },
  "logs.instanceAll": {
    message: "全部实例",
    description: "Instance filter option: no instance filter",
  },
  "logs.filterByInstance": {
    message: "按实例 {instance} 筛选日志",
    description:
      "Accessible label for the clickable instance slug on an application log line",
  },
  "logs.pathLabel": {
    message: "请求路径",
    description: "Accessible label for the request-path filter input",
  },
  "logs.filtersButton": {
    message: "筛选",
    description:
      "Trigger for the structured-filters popover (level/method/status/instance/path)",
  },
  "logs.chipRemove": {
    message: "移除{label}筛选",
    description:
      "Accessible label for an active-filter chip's clear button; {label} is the filter's field name",
  },
  "logs.pathPlaceholder": {
    message: "按路径筛选",
    description: "Placeholder for the request-path filter input",
  },
  "logs.liveUnsupported": {
    message: "实时日志已关闭——请求日志与结构化筛选无法实时跟踪。",
    description:
      "Note when a store-only filter disables live tail (the tail reads pod stdout)",
  },
  "logs.storeRequiredTitle": {
    message: "请求日志需要日志存储",
    description:
      "Empty-state title when a request/structured-filter query hits a deployment with no durable store (503)",
  },
  "logs.storeRequiredBody": {
    message:
      "请求日志以及级别/状态/方法/路径筛选需从持久化日志存储读取，而此环境未配置该存储。在已接入存储的环境中可用。",
    description: "Empty-state body for the store-unavailable (503) state",
  },
  "logs.searchPlaceholder": {
    message: "搜索日志",
    description: "Placeholder for the log search box (text filter)",
  },
  "logs.live": {
    message: "实时",
    description: "Label for the live-tail on/off toggle",
  },
  "logs.jumpToLatest": {
    message: "跳到最新",
    description: "Button to re-pin autoscroll to the newest log line",
  },
  "logs.loading": {
    message: "正在加载日志……",
    description: "Shown while the first historical page is loading",
  },
  "logs.streaming": {
    message: "实时——正在接收新日志",
    description: "Status under the log list when the SSE tail is connected",
  },
  "logs.paused": {
    message: "已暂停实时日志",
    description: "Status under the log list when live tail is toggled off",
  },
  "logs.disconnected": {
    message: "实时日志已断开——正在重连……",
    description: "Banner when the SSE stream drops",
  },
  "logs.emptyTitle": {
    message: "暂无日志",
    description: "Empty-state title when the service has produced no logs",
  },
  "logs.emptyBody": {
    message: "该服务尚未产生任何日志。",
    description: "Empty-state body with no filters applied",
  },
  "logs.emptyFilteredBody": {
    message: "没有匹配这些筛选条件的日志。",
    description:
      "Empty-state body when a type/text/structured filter yields nothing",
  },
  "logs.emptyFilteredTitle": {
    message: "没有匹配的日志",
    description:
      "Empty-state title when a type/text/structured filter yields nothing",
  },
  "logs.errorTitle": {
    message: "无法加载日志",
    description: "Error-state title when the logs query fails",
  },
};

export default zhLogs;
