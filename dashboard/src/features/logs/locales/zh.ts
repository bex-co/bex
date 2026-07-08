import type { TranslationEntry } from "@/i18n";

const zhLogs: Record<string, TranslationEntry> = {
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
      "Log-type filter option: request logs (empty on bex — no backend)",
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
    message: "没有匹配此筛选条件的日志。bex 仅提供应用日志——请求日志为空。",
    description:
      "Empty-state body when a type/text filter yields nothing (honest about bex's app-only contract)",
  },
  "logs.errorTitle": {
    message: "无法加载日志",
    description: "Error-state title when the logs query fails",
  },
};

export default zhLogs;
