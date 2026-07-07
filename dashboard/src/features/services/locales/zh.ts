import type { TranslationEntry } from "@/i18n";

const zhServices: Record<string, TranslationEntry> = {
  "services.statTotal": {
    message: "服务总数",
    description: "Services page stat card label",
  },
  "services.statRunning": {
    message: "运行中",
    description: "Services page stat card label",
  },
  "services.statSuspended": {
    message: "已暂停",
    description: "Services page stat card label",
  },
  "services.cardTitle": {
    message: "服务",
    description: "Services table card title, also used as the metrics page back-link",
  },
  "services.colName": {
    message: "名称",
    description: "Services table column header",
  },
  "services.colStatus": {
    message: "状态",
    description: "Services table column header",
  },
  "services.colUrl": {
    message: "URL",
    description: "Services table column header",
  },
  "services.colInstances": {
    message: "实例数",
    description: "Services table column header (replica count — bex-native)",
  },
  "services.colRevision": {
    message: "版本",
    description: "Services table column header (active revision — bex-native)",
  },
  "services.colCreated": {
    message: "创建于",
    description: "Services table column header (relative age from createdAt)",
  },
  "services.colActions": {
    message: "操作",
    description: "Services table actions column header (screen-reader only)",
  },
  "services.statusRunning": {
    message: "运行中",
    description: "Services table status badge",
  },
  "services.statusSuspended": {
    message: "已暂停",
    description: "Services table status badge",
  },
  "services.statusHibernated": {
    message: "已休眠",
    description: "Services table status badge (App scaled to zero)",
  },
  "services.statusPending": {
    message: "等待中",
    description: "Services table status badge",
  },
  "services.statusBuilding": {
    message: "构建中",
    description: "Services table status badge",
  },
  "services.statusDeploying": {
    message: "部署中",
    description: "Services table status badge",
  },
  "services.statusFailed": {
    message: "失败",
    description: "Services table status badge",
  },
  "services.statusUnknown": {
    message: "未知",
    description: "Services table status badge for an unrecognized phase",
  },
  "services.actionsMenu": {
    message: "打开操作菜单",
    description: "Accessible label for the per-row actions trigger",
  },
  "services.actionSuspend": {
    message: "暂停",
    description: "Row action: park the service",
  },
  "services.actionResume": {
    message: "恢复",
    description: "Row action: bring a suspended service back",
  },
  "services.actionRestart": {
    message: "重启",
    description: "Row action: roll the service's pods",
  },
  "services.confirmSuspendTitle": {
    message: "暂停 {name}？",
    description: "Suspend confirmation dialog title",
  },
  "services.confirmSuspendBody": {
    message: "服务将缩容至零并停止处理流量。其 URL 与证书会保留，你可以随时恢复。",
    description: "Suspend confirmation dialog body",
  },
  "services.confirmRestartTitle": {
    message: "重启 {name}？",
    description: "Restart confirmation dialog title",
  },
  "services.confirmRestartBody": {
    message: "服务的 Pod 将无停机滚动更新，进行中的请求会先完成再替换旧实例。",
    description: "Restart confirmation dialog body",
  },
  "services.confirmCancel": {
    message: "取消",
    description: "Confirmation dialog cancel button",
  },
  "services.toastSuspendSuccess": {
    message: "正在暂停 {name}……",
    description: "Toast shown after a suspend request is accepted",
  },
  "services.toastResumeSuccess": {
    message: "正在恢复 {name}……",
    description: "Toast shown after a resume request is accepted",
  },
  "services.toastRestartSuccess": {
    message: "正在重启 {name}……",
    description: "Toast shown after a restart request is accepted",
  },
  "services.toastError": {
    message: "无法更新 {name}，请重试。",
    description: "Toast shown when a lifecycle action fails",
  },
  "services.errorTitle": {
    message: "无法加载服务",
    description: "Services list error card title",
  },
  "services.errorBody": {
    message: "对 bex-api 的请求失败。请检查网络连接后重试。",
    description: "Services list error card body",
  },
  "services.emptyTitle": {
    message: "还没有服务",
    description: "Services list empty state title",
  },
  "services.emptyBody": {
    message: "部署你的第一个 App，它就会出现在这里。",
    description: "Services list empty state body",
  },
};

export default zhServices;
