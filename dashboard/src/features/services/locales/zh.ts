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
  "services.navLabel": {
    message: "服务导航",
    description: "Accessible label for the service-detail tab nav",
  },
  "services.navOverview": {
    message: "概览",
    description: "Service-detail nav item + overview panel title",
  },
  "services.navLogs": {
    message: "日志",
    description: "Service-detail nav item (logs tab)",
  },
  "services.navMetrics": {
    message: "指标",
    description: "Service-detail nav item (metrics tab)",
  },
  "services.overviewPhase": {
    message: "阶段",
    description: "Overview panel field label (operator phase, verbatim)",
  },
  "services.overviewSuspended": {
    message: "已暂停",
    description: "Overview panel field label (suspend state)",
  },
  "services.overviewYes": {
    message: "是",
    description: "Overview panel value for a true boolean field",
  },
  "services.overviewNo": {
    message: "否",
    description: "Overview panel value for a false boolean field",
  },
  "services.notFoundTitle": {
    message: "未找到服务",
    description: "Overview page state when server(id) returns nothing",
  },
  "services.notFoundBody": {
    message: "不存在名为 {name} 的服务，或你没有访问权限。",
    description: "Overview page not-found body",
  },
  "services.logsComingSoonTitle": {
    message: "日志功能即将上线",
    description: "Logs tab placeholder title (content ships in a later release)",
  },
  "services.logsComingSoonBody": {
    message: "该服务的实时日志跟踪将在后续版本中提供。",
    description: "Logs tab placeholder body",
  },
  "services.navEnvironment": {
    message: "环境变量",
    description: "Service-detail nav item (environment variables tab)",
  },
  "services.envTitle": {
    message: "环境变量",
    description: "Environment tab card title",
  },
  "services.envDescription": {
    message: "设置该环境的配置和密钥，然后在代码中读取这些值。",
    description: "Environment tab card description",
  },
  "services.envColKey": {
    message: "键",
    description: "Environment table column header (variable name)",
  },
  "services.envColValue": {
    message: "值",
    description: "Environment table column header (variable value)",
  },
  "services.envShowSecret": {
    message: "显示值",
    description: "Environment row button to reveal a masked value",
  },
  "services.envHideSecret": {
    message: "隐藏值",
    description: "Environment row button to re-mask a revealed value",
  },
  "services.envRevealError": {
    message: "无法加载该值。",
    description: "Environment row inline error when a value reveal fails",
  },
  "services.envEmptyTitle": {
    message: "暂无环境变量",
    description: "Environment tab empty-state title",
  },
  "services.envEmptyBody": {
    message: "添加一个变量来配置该服务。",
    description: "Environment tab empty-state body",
  },
  "services.envUnavailableTitle": {
    message: "环境变量不可用",
    description: "Environment tab state when the secret store is unconfigured (503)",
  },
  "services.envUnavailableBody": {
    message: "此部署未配置密钥存储。",
    description: "Environment tab unavailable-state body",
  },
  "services.envForbiddenTitle": {
    message: "无权访问",
    description: "Environment tab state when the caller lacks permission (403)",
  },
  "services.envForbiddenBody": {
    message: "你没有查看此服务环境变量的权限。",
    description: "Environment tab forbidden-state body",
  },
  "services.envErrorTitle": {
    message: "无法加载环境变量",
    description: "Environment tab generic error title",
  },
  "services.envErrorBody": {
    message: "出错了，请重试。",
    description: "Environment tab generic error body",
  },
  "services.envAdd": {
    message: "添加变量",
    description: "Environment tab button to open the add-variable form",
  },
  "services.envEdit": {
    message: "编辑",
    description: "Environment row button to edit a variable's value",
  },
  "services.envDelete": {
    message: "删除",
    description: "Environment row button to remove a variable",
  },
  "services.envSave": {
    message: "保存",
    description: "Environment add/edit form save button",
  },
  "services.envCancel": {
    message: "取消",
    description: "Environment add/edit form cancel button",
  },
  "services.envKeyPlaceholder": {
    message: "变量名称",
    description: "Environment add-variable key input placeholder",
  },
  "services.envValuePlaceholder": {
    message: "值",
    description: "Environment value input placeholder",
  },
  "services.envInvalidKey": {
    message: "只能使用字母、数字和下划线，且不能以数字开头。",
    description: "Environment add-variable validation message for an invalid key",
  },
  "services.envDeleteConfirmTitle": {
    message: "删除 {key}？",
    description: "Environment delete-confirmation dialog title",
  },
  "services.envDeleteConfirmBody": {
    message: "服务将在移除该变量后重新部署。",
    description: "Environment delete-confirmation dialog body",
  },
  "services.envRolloutNote": {
    message: "服务正在重新部署以应用更改。",
    description: "Toast description after an env-var write (bex rolls the pods)",
  },
  "services.envSaveSuccess": {
    message: "已保存 {key}",
    description: "Toast on a successful env-var add/update",
  },
  "services.envSaveError": {
    message: "无法保存 {key}",
    description: "Toast on a failed env-var add/update",
  },
  "services.envDeleteSuccess": {
    message: "已删除 {key}",
    description: "Toast on a successful env-var delete",
  },
  "services.envDeleteError": {
    message: "无法删除 {key}",
    description: "Toast on a failed env-var delete",
  },
};

export default zhServices;
