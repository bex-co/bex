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
    description:
      "Services table card title, also used as the metrics page back-link",
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
  "services.statusSleeping": {
    message: "休眠中",
    description:
      "Services status badge: a free-tier App auto-hibernated after idle (bex extension)",
  },
  "services.statusSleepingHint": {
    message: "为节省资源已休眠 —— 下次请求时自动唤醒。",
    description:
      "Hint next to the Sleeping badge explaining free-tier auto-sleep + wake-on-request",
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
    message:
      "服务将缩容至零并停止处理流量。其 URL 与证书会保留，你可以随时恢复。",
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
    description:
      "Environment tab state when the secret store is unconfigured (503)",
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
    description:
      "Environment add-variable validation message for an invalid key",
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
    description:
      "Toast description after an env-var write (bex rolls the pods)",
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
  "services.navSettings": {
    message: "设置",
    description: "Service-detail nav item (settings tab)",
  },
  "services.settingsTitle": {
    message: "设置",
    description: "Settings tab card title",
  },
  "services.settingsDescription": {
    message: "配置该服务的实例规格及其他设置。",
    description: "Settings tab card description",
  },
  "services.settingsInstanceType": {
    message: "实例类型",
    description: "Settings tab row label for the App's current plan/tier",
  },
  "services.settingsNoInstanceType": {
    message: "未设置实例类型",
    description: "Settings tab state for an untiered (bare-CR) App",
  },
  "services.settingsUpdate": {
    message: "更新",
    description: "Settings tab link to the instance-type picker",
  },
  "services.settingsIdleTimeout": {
    message: "空闲超时",
    description:
      "Settings tab: label for the free-tier auto-sleep window control",
  },
  "services.settingsIdleTimeoutHint": {
    message: "免费服务在此空闲时长后休眠，下次请求时自动唤醒。",
    description: "Settings tab: idle-timeout control help text (bex extension)",
  },
  "services.settingsIdleTimeoutPaid": {
    message: "付费服务始终在线，不会休眠。",
    description: "Settings tab: shown instead of the control on a paid plan",
  },
  "services.idleTimeoutDefault": {
    message: "平台默认",
    description: "Idle-timeout option: 0 seconds = the operator's own window",
  },
  "services.idleTimeoutMinutes": {
    message: "{minutes} 分钟",
    description: "Idle-timeout option label in minutes",
  },
  "services.idleTimeoutHours": {
    message: "{hours} 小时",
    description: "Idle-timeout option label in hours",
  },
  "services.idleTimeoutSeconds": {
    message: "{seconds} 秒",
    description: "Idle-timeout option label in seconds (non-round values)",
  },
  "services.idleTimeoutSuccess": {
    message: "空闲超时已更新。",
    description: "Toast after setIdleTimeout succeeds",
  },
  "services.idleTimeoutError": {
    message: "无法更新空闲超时。",
    description: "Toast after setIdleTimeout fails",
  },
  "services.planPickerTitle": {
    message: "选择实例类型",
    description: "Plan-picker page heading",
  },
  "services.planPickerFreeGroup": {
    message: "免费",
    description:
      "Plan-picker section label separating the Free tier from paid tiers",
  },
  "services.planPickerPaidGroup": {
    message: "付费",
    description: "Plan-picker section label for the paid tier ladder",
  },
  "services.planPickerCancel": {
    message: "取消",
    description: "Plan-picker footer button: discard the selection",
  },
  "services.planPickerSave": {
    message: "保存更改",
    description: "Plan-picker footer button: confirm the plan change",
  },
  "services.planPickerConfirmTitle": {
    message: "将实例类型更改为 {name}？",
    description: "Plan-change confirm dialog title",
  },
  "services.planPickerConfirmBody": {
    message:
      "服务将调整规格并无停机滚动更新——进行中的请求会先完成再替换旧实例。",
    description: "Plan-change confirm dialog body",
  },
  "services.planPickerSuccess": {
    message: "实例类型已更新为 {name}",
    description: "Toast on a successful plan change",
  },
  "services.planPickerError": {
    message: "无法更新实例类型，请重试。",
    description: "Toast on a failed plan change",
  },
  "services.planPickerErrorTitle": {
    message: "无法加载实例类型",
    description: "Plan-picker error state title (instanceTypes query failed)",
  },
  "services.planPickerErrorBody": {
    message: "对 bex-api 的请求失败。请检查网络连接后重试。",
    description: "Plan-picker error state body",
  },
  "services.domainsTitle": {
    message: "自定义域名",
    description: "Settings tab custom-domains section title",
  },
  "services.domainsDescription": {
    message: "将你拥有的自定义域名指向此服务。",
    description: "Settings tab custom-domains section description",
  },
  "services.domainColName": {
    message: "名称",
    description: "Custom-domains table column header (the FQDN)",
  },
  "services.domainColVerified": {
    message: "验证状态",
    description:
      "Custom-domains table column header (DNS/ownership verification)",
  },
  "services.domainColCertificate": {
    message: "证书状态",
    description:
      "Custom-domains table column header (TLS certificate serving state)",
  },
  "services.domainColActions": {
    message: "操作",
    description:
      "Custom-domains table actions column header (screen-reader only)",
  },
  "services.domainVerified": {
    message: "已验证",
    description: "Custom-domains status badge: TLS certificate has been issued",
  },
  "services.domainCertActive": {
    message: "已生效",
    description:
      "Custom-domains status badge: certificate issued and serving traffic",
  },
  "services.domainPending": {
    message: "待处理",
    description:
      "Custom-domains status badge: certificate not yet issued/serving",
  },
  "services.domainActionsMenu": {
    message: "打开域名操作菜单",
    description: "Accessible label for the per-domain actions trigger",
  },
  "services.domainDelete": {
    message: "删除",
    description: "Custom-domains row action: remove the domain",
  },
  "services.domainCancel": {
    message: "取消",
    description: "Custom-domains dialog cancel button",
  },
  "services.domainDeleteConfirmTitle": {
    message: "删除 {name}？",
    description: "Custom-domain delete-confirmation dialog title",
  },
  "services.domainDeleteConfirmBody": {
    message:
      "服务将停止为此域名提供服务。其 Ingress 规则会被移除，TLS 证书将被留待过期。此操作无法撤销。",
    description: "Custom-domain delete-confirmation dialog body",
  },
  "services.domainAdd": {
    message: "添加自定义域名",
    description: "Custom-domains button to open the add-domain dialog",
  },
  "services.domainAddTitle": {
    message: "添加自定义域名",
    description: "Add-domain dialog title",
  },
  "services.domainAddDescription": {
    message: "输入你拥有的域名。将其 DNS 指向此服务，bex 会自动签发 TLS 证书。",
    description: "Add-domain dialog description",
  },
  "services.domainPlaceholder": {
    message: "www.example.com",
    description: "Add-domain FQDN input placeholder",
  },
  "services.domainInvalid": {
    message: "请输入有效的域名，例如 www.example.com。",
    description: "Add-domain validation message for a malformed hostname",
  },
  "services.domainAddButton": {
    message: "添加域名",
    description: "Add-domain dialog submit button",
  },
  "services.domainAddSuccess": {
    message: "已添加 {name}",
    description: "Toast on a successful custom-domain add",
  },
  "services.domainAddError": {
    message: "无法添加 {name}",
    description: "Toast on a failed custom-domain add",
  },
  "services.domainDeleteSuccess": {
    message: "已移除 {name}",
    description: "Toast on a successful custom-domain delete",
  },
  "services.domainDeleteError": {
    message: "无法移除 {name}",
    description: "Toast on a failed custom-domain delete",
  },
  "services.domainPropagateNote": {
    message: "DNS 与 TLS 证书会在后台生效。",
    description:
      "Toast description after a custom-domain add (async convergence)",
  },
  "services.domainsEmptyTitle": {
    message: "暂无自定义域名",
    description: "Custom-domains empty-state title",
  },
  "services.domainsEmptyBody": {
    message: "添加一个你拥有的域名，即可通过它访问此服务。",
    description: "Custom-domains empty-state body",
  },
  "services.domainsErrorTitle": {
    message: "无法加载自定义域名",
    description: "Custom-domains generic error title",
  },
  "services.domainsErrorBody": {
    message: "对 bex-api 的请求失败。请检查网络连接后重试。",
    description: "Custom-domains generic error body",
  },
  "services.platformSubdomainTitle": {
    message: "平台子域名",
    description: "Settings tab platform-subdomain section title",
  },
  "services.platformSubdomainDescription": {
    message: "除自定义域名外，你的服务始终可通过其 bex 平台子域名访问。",
    description: "Settings tab platform-subdomain section description",
  },
  "services.platformSubdomainEnabled": {
    message: "始终启用",
    description: "Platform-subdomain badge: the subdomain can't be turned off",
  },
  "services.platformSubdomainPending": {
    message: "服务运行后将分配平台 URL。",
    description: "Platform-subdomain state when the service has no URL yet",
  },
  "services.colType": {
    message: "类型",
    description: "Services table column header (service type)",
  },
  "services.typeWeb": {
    message: "Web 服务",
    description: "Service-type badge: an HTTP service exposed at a URL",
  },
  "services.typePrivate": {
    message: "私有服务",
    description:
      "Service-type badge: an HTTP service reachable only in-cluster",
  },
  "services.typeWorker": {
    message: "后台工作进程",
    description: "Service-type badge: runs with no HTTP port/URL",
  },
  "services.typeCron": {
    message: "定时任务",
    description: "Service-type badge: runs a command on a schedule",
  },
  "services.typeUnknown": {
    message: "服务",
    description: "Service-type badge fallback for an unrecognized type",
  },
  "services.overviewType": {
    message: "类型",
    description: "Overview tab row label for the service type",
  },
  "services.overviewSchedule": {
    message: "计划",
    description: "Overview tab row label for a cron job's schedule",
  },
  "services.cronRunsTitle": {
    message: "最近运行",
    description: "Cron job overview: recent-runs section title",
  },
  "services.cronRunsEmpty": {
    message: "暂无运行记录。",
    description: "Cron job overview: shown when a cron has no run history",
  },
  "services.cronRunColStarted": {
    message: "开始时间",
    description: "Cron runs table column header (run start time)",
  },
  "services.cronRunColStatus": {
    message: "状态",
    description: "Cron runs table column header (run outcome)",
  },
  "services.cronRunStatusRunning": {
    message: "运行中",
    description: "Cron run status badge",
  },
  "services.cronRunStatusSucceeded": {
    message: "成功",
    description: "Cron run status badge",
  },
  "services.cronRunStatusFailed": {
    message: "失败",
    description: "Cron run status badge",
  },
};

export default zhServices;
