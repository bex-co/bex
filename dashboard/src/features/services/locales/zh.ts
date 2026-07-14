import type { TranslationEntry } from "@/i18n";

const zhServices: Record<string, TranslationEntry> = {
  "services.actions": {
    message: "操作",
    description: "Accessible heading for service configuration row actions",
  },
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
  "services.headerServiceId": {
    message: "服务 ID：",
    description: "Service-detail header metadata label for the service id",
  },
  "services.headerSchedule": {
    message: "调度：",
    description:
      "Service-detail header metadata label for a cron job's schedule",
  },
  "services.headerCopyServiceId": {
    message: "复制服务 ID",
    description: "Accessible label for the header's service-id copy button",
  },
  "services.headerCopyUrl": {
    message: "复制服务 URL",
    description: "Accessible label for the header's live-URL copy button",
  },
  "services.headerCopied": {
    message: "已复制到剪贴板",
    description: "Toast after copying a value from the service-detail header",
  },
  "services.headerCopyError": {
    message: "无法复制到剪贴板",
    description: "Toast when a service-detail header copy fails",
  },
  "services.navLogs": {
    message: "日志",
    description: "Service-detail nav item (logs tab)",
  },
  "services.navMetrics": {
    message: "指标",
    description: "Service-detail nav item (metrics tab)",
  },
  "services.navScaling": {
    message: "弹性伸缩",
    description: "Service-detail nav item (autoscaling tab)",
  },
  "services.scalingTitle": {
    message: "自动伸缩",
    description: "Scaling tab section heading",
  },
  "services.scalingEnabled": {
    message: "启用自动伸缩",
    description: "Autoscaling enable/disable toggle label",
  },
  "services.scalingMinInstances": {
    message: "最少实例数",
    description: "Autoscaling min instances input label",
  },
  "services.scalingMaxInstances": {
    message: "最多实例数",
    description: "Autoscaling max instances input label",
  },
  "services.scalingTargetCPU": {
    message: "目标 CPU 使用率 %",
    description: "Autoscaling target CPU utilisation input label",
  },
  "services.scalingTargetMemory": {
    message: "目标内存使用率 %",
    description: "Autoscaling target memory utilisation input label",
  },
  "services.scalingSave": {
    message: "保存",
    description: "Autoscaling form save button",
  },
  "services.scalingSaved": {
    message: "自动伸缩设置已保存。",
    description: "Autoscaling save success toast",
  },
  "services.scalingDisabled": {
    message: "自动伸缩已禁用。",
    description: "Autoscaling disable success toast",
  },
  "services.scalingError": {
    message: "更新自动伸缩设置失败。",
    description: "Autoscaling save error toast",
  },
  "services.scalingDescription": {
    message: "根据 CPU 和内存使用率自动扩缩此服务的实例数量。",
    description: "Autoscaling card description",
  },
  "services.scalingOn": {
    message: "自动伸缩已开启",
    description: "Autoscaling main toggle label when enabled",
  },
  "services.scalingOff": {
    message: "自动伸缩已关闭",
    description: "Autoscaling main toggle label when disabled",
  },
  "services.scalingInstancesTitle": {
    message: "实例数量",
    description: "Autoscaling instances range-slider section heading",
  },
  "services.scalingInstancesHint": {
    message: "bex 将在你指定的范围内自动调整此服务的实例数量。",
    description: "Autoscaling instances range-slider section description",
  },
  "services.scalingCPUTitle": {
    message: "CPU 使用率目标",
    description: "Autoscaling CPU metric section heading",
  },
  "services.scalingCPUHint": {
    message:
      "当平均 CPU 使用率明显高于或低于此值时，bex 会相应地增加或减少实例数。",
    description: "Autoscaling CPU metric section description",
  },
  "services.scalingMemoryTitle": {
    message: "内存使用率目标",
    description: "Autoscaling memory metric section heading",
  },
  "services.scalingMemoryHint": {
    message:
      "当平均内存使用率明显高于或低于此值时，bex 会相应地增加或减少实例数。",
    description: "Autoscaling memory metric section description",
  },
  "services.scalingCancel": {
    message: "取消",
    description: "Autoscaling form cancel button",
  },
  "services.scalingSaveChanges": {
    message: "保存更改",
    description: "Autoscaling form save-changes button",
  },
  "services.notFoundTitle": {
    message: "未找到服务",
    description: "Overview page state when server(id) returns nothing",
  },
  "services.notFoundBody": {
    message: "不存在名为 {name} 的服务，或你没有访问权限。",
    description: "Overview page not-found body",
  },
  "services.notFoundBackToList": {
    message: "返回服务列表",
    description:
      "Link on the service-detail not-found state back to the services list",
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
  "services.secretFilesTitle": {
    message: "密钥文件",
    description: "Environment tab secret-files section title",
  },
  "services.secretFilesDescription": {
    message: "存放包含机密内容（证书、凭据）的文件，部署时挂载到此服务中。",
    description: "Environment tab secret-files section description",
  },
  "services.secretFileColName": {
    message: "文件名",
    description: "Secret-files table column header (file name)",
  },
  "services.secretFileColContent": {
    message: "内容",
    description: "Secret-files table column header (file body)",
  },
  "services.secretFilesEmptyTitle": {
    message: "暂无密钥文件",
    description: "Secret-files empty-state title",
  },
  "services.secretFilesEmptyBody": {
    message: "添加一个文件，将机密内容挂载到此服务中。",
    description: "Secret-files empty-state body",
  },
  "services.secretFilesUnavailableTitle": {
    message: "密钥文件不可用",
    description:
      "Secret-files state when the secret store is unconfigured (503)",
  },
  "services.secretFilesUnavailableBody": {
    message: "此部署未配置密钥存储。",
    description: "Secret-files unavailable-state body",
  },
  "services.secretFilesForbiddenTitle": {
    message: "无权访问",
    description: "Secret-files state when the caller lacks permission (403)",
  },
  "services.secretFilesForbiddenBody": {
    message: "你没有查看此服务密钥文件的权限。",
    description: "Secret-files forbidden-state body",
  },
  "services.secretFilesErrorTitle": {
    message: "无法加载密钥文件",
    description: "Secret-files generic error title",
  },
  "services.secretFilesErrorBody": {
    message: "出错了，请重试。",
    description: "Secret-files generic error body",
  },
  "services.secretFileAdd": {
    message: "添加密钥文件",
    description: "Secret-files button to open the add-file form",
  },
  "services.secretFileNamePlaceholder": {
    message: "文件名.扩展名",
    description: "Secret-files add-file name input placeholder",
  },
  "services.secretFileContentPlaceholder": {
    message: "文件内容",
    description: "Secret-files content input placeholder",
  },
  "services.secretFileInvalidName": {
    message: "只能使用字母、数字、点、短横线和下划线；不能为“.”或“..”。",
    description: "Secret-files add-file validation message for an invalid name",
  },
  "services.secretFileDeleteConfirmTitle": {
    message: "删除 {name}？",
    description: "Secret-file delete-confirmation dialog title",
  },
  "services.secretFileDeleteConfirmBody": {
    message: "服务将在移除该文件后重新部署。",
    description: "Secret-file delete-confirmation dialog body",
  },
  "services.secretFileSaveSuccess": {
    message: "已保存 {name}",
    description: "Toast on a successful secret-file add/update",
  },
  "services.secretFileSaveError": {
    message: "无法保存 {name}",
    description: "Toast on a failed secret-file add/update",
  },
  "services.secretFileDeleteSuccess": {
    message: "已删除 {name}",
    description: "Toast on a successful secret-file delete",
  },
  "services.secretFileDeleteError": {
    message: "无法删除 {name}",
    description: "Toast on a failed secret-file delete",
  },
  "services.envGroupsTitle": {
    message: "环境变量组",
    description: "Environment tab env-groups section title",
  },
  "services.envGroupsDescription": {
    message: "可复用的环境变量与密钥文件集合，可链接到此服务及其他服务。",
    description: "Environment tab env-groups section description",
  },
  "services.envGroupsEmptyTitle": {
    message: "暂无环境变量组",
    description: "Env-groups empty-state title",
  },
  "services.envGroupsEmptyBody": {
    message: "创建一个组，在多个服务间共享配置。",
    description: "Env-groups empty-state body",
  },
  "services.envGroupsUnavailableTitle": {
    message: "环境变量组不可用",
    description: "Env-groups state when the secret store is unconfigured (503)",
  },
  "services.envGroupsUnavailableBody": {
    message: "此部署未配置密钥存储。",
    description: "Env-groups unavailable-state body",
  },
  "services.envGroupsForbiddenTitle": {
    message: "无权访问",
    description: "Env-groups state when the caller lacks permission (403)",
  },
  "services.envGroupsForbiddenBody": {
    message: "你没有查看环境变量组的权限。",
    description: "Env-groups forbidden-state body",
  },
  "services.envGroupsErrorTitle": {
    message: "无法加载环境变量组",
    description: "Env-groups generic error title",
  },
  "services.envGroupsErrorBody": {
    message: "出错了，请重试。",
    description: "Env-groups generic error body",
  },
  "services.envGroupCreate": {
    message: "创建组",
    description: "Env-groups button to open the create-group form",
  },
  "services.envGroupCreateSubmit": {
    message: "创建",
    description: "Env-groups create-group form submit button",
  },
  "services.envGroupNamePlaceholder": {
    message: "组名称",
    description: "Env-groups create-group name input placeholder",
  },
  "services.envGroupNameLabel": {
    message: "组名称",
    description: "Env-groups create-group name input accessible label",
  },
  "services.envGroupInvalidName": {
    message: "请输入组名称。",
    description:
      "Env-groups create-group validation message for an invalid name",
  },
  "services.envGroupLinked": {
    message: "已链接",
    description:
      "Env-groups badge: this group is linked to the current service",
  },
  "services.envGroupEmptyContents": {
    message: "暂无变量或文件。",
    description: "Env-groups: shown when a group has no vars or secret files",
  },
  "services.envGroupLink": {
    message: "链接",
    description: "Env-groups button: attach this group to the current service",
  },
  "services.envGroupUnlink": {
    message: "取消链接",
    description:
      "Env-groups button: detach this group from the current service",
  },
  "services.envGroupDelete": {
    message: "删除",
    description: "Env-groups action: delete the group",
  },
  "services.envGroupDeleteConfirmTitle": {
    message: "删除 {name}？",
    description: "Env-group delete-confirmation dialog title",
  },
  "services.envGroupDeleteConfirmBody": {
    message: "该组将从所有链接到它的服务中移除。此操作无法撤销。",
    description: "Env-group delete-confirmation dialog body",
  },
  "services.envGroupCreateSuccess": {
    message: "已创建 {name}",
    description: "Toast on a successful env-group create",
  },
  "services.envGroupCreateError": {
    message: "无法创建 {name}",
    description: "Toast on a failed env-group create",
  },
  "services.envGroupDeleteSuccess": {
    message: "组已删除",
    description: "Toast on a successful env-group delete",
  },
  "services.envGroupDeleteError": {
    message: "无法删除该组",
    description: "Toast on a failed env-group delete",
  },
  "services.envGroupLinkSuccess": {
    message: "组已链接",
    description: "Toast on a successful env-group link",
  },
  "services.envGroupLinkError": {
    message: "无法链接该组",
    description: "Toast on a failed env-group link",
  },
  "services.envGroupUnlinkSuccess": {
    message: "组已取消链接",
    description: "Toast on a successful env-group unlink",
  },
  "services.envGroupUnlinkError": {
    message: "无法取消链接该组",
    description: "Toast on a failed env-group unlink",
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
    message: "配置该服务的名称、实例规格及其他设置。",
    description: "Settings tab card description",
  },
  "services.displayNameLabel": {
    message: "服务名称",
    description:
      "Settings tab row label for the mutable human-facing service name",
  },
  "services.displayNameHint": {
    message: "服务 ID 仍为 {id}；URL 和基础设施不会改变。",
    description:
      "Settings tab explanation that a display-name change preserves identity",
  },
  "services.displayNameEdit": {
    message: "编辑服务名称",
    description: "Accessible label for the service-name edit button",
  },
  "services.displayNameSave": {
    message: "保存服务名称",
    description: "Accessible label for the service-name save button",
  },
  "services.displayNameCancel": {
    message: "取消编辑服务名称",
    description: "Accessible label for the service-name cancel button",
  },
  "services.displayNameSuccess": {
    message: "服务已重命名为「{name}」。",
    description: "Toast after setDisplayName succeeds",
  },
  "services.displayNameCleared": {
    message: "服务名称已重置为其不可变 ID。",
    description: "Toast after clearing displayName",
  },
  "services.displayNameError": {
    message: "无法重命名服务，请重试。",
    description: "Toast after setDisplayName fails",
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
  "services.settingsHealthChecksTitle": {
    message: "健康检查",
    description: "Settings tab: Health Checks section card title",
  },
  "services.settingsHealthChecksDescription": {
    message: "配置 bex 定期轮询以监控服务的 HTTP 路径。",
    description: "Settings tab: Health Checks section card description",
  },
  "services.settingsHealthCheckPath": {
    message: "健康检查路径",
    description: "Settings tab: health-check path row label",
  },
  "services.settingsHealthCheckPathHint": {
    message: "提供 bex 定期轮询以监控服务的 HTTP 路径。",
    description: "Settings tab: health-check path row hint text",
  },
  "services.settingsHealthCheckPathPlaceholder": {
    message: "/",
    description: "Settings tab: health-check path input placeholder",
  },
  "services.healthCheckPathSuccess": {
    message: "健康检查路径已更新。",
    description: "Toast after setHealthCheckPath succeeds",
  },
  "services.healthCheckPathError": {
    message: "无法更新健康检查路径。",
    description: "Toast after setHealthCheckPath fails",
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
  "services.domainAddConflict": {
    message: "{name} 已被另一个服务使用",
    description:
      "Toast when a custom-domain add is rejected because the host is registered on another service (409)",
  },
  "services.domainAddReserved": {
    message: "{name} 是平台保留的主机名",
    description:
      "Toast when a custom-domain add is rejected because the host is a platform-owned name (400)",
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
  "services.domainDnsToggle": {
    message: "显示 DNS 配置",
    description:
      "aria-label for the per-domain DNS-instructions disclosure toggle",
  },
  "services.domainDnsTitle": {
    message: "DNS 配置",
    description: "Heading of the per-domain DNS-instructions panel",
  },
  "services.domainDnsSubdomainGuidance": {
    message:
      "请在你的 DNS 服务商处创建以下记录，然后重新检查。记录生效后，bex 会自动签发 TLS 证书。",
    description: "Guidance line above the DNS record for a subdomain",
  },
  "services.domainDnsApexGuidance": {
    message:
      "顶级域名无法使用普通 CNAME。若你的服务商支持 ALIAS/ANAME（或 CNAME flattening），请创建此记录；否则请在注册商处将顶级域名重定向到 www 子域名。",
    description: "Guidance line above the DNS record for an apex domain",
  },
  "services.domainRecordType": {
    message: "类型",
    description: "Label for the DNS record type field (CNAME/ALIAS)",
  },
  "services.domainRecordHost": {
    message: "主机",
    description: "Label for the DNS record host/name field",
  },
  "services.domainRecordTarget": {
    message: "目标",
    description: "Label for the DNS record target/value field",
  },
  "services.domainDnsUnavailable": {
    message: "DNS 目标尚不可用——服务运行后请重新检查。",
    description: "Shown when the backend couldn't derive the DNS record target",
  },
  "services.domainRecheck": {
    message: "重新检查",
    description: "Button that re-checks a domain's DNS/certificate status",
  },
  "services.domainCopied": {
    message: "已复制到剪贴板",
    description: "Toast when a DNS record value is copied",
  },
  "services.domainCopyError": {
    message: "无法复制到剪贴板",
    description: "Toast when copying a DNS record value fails",
  },
  "services.domainAddedTitle": {
    message: "域名已添加——请配置 DNS",
    description: "Title of the post-add DNS-record step in the add dialog",
  },
  "services.domainAddedDescription": {
    message: "请在你的 DNS 服务商处创建此记录，以完成域名连接。",
    description: "Subtitle of the post-add DNS-record step in the add dialog",
  },
  "services.domainDone": {
    message: "完成",
    description: "Button closing the post-add DNS-record step",
  },
  "services.domainVerifySuccess": {
    message: "{name} 已验证",
    description: "Toast when a re-check finds the domain verified",
  },
  "services.domainVerifyPending": {
    message: "{name} 尚未验证——DNS 可能仍在生效中。",
    description: "Toast when a re-check finds the domain still pending",
  },
  "services.domainVerifyError": {
    message: "无法重新检查 {name}。",
    description: "Toast when the re-check request fails",
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
  "services.deployTitle": {
    message: "部署",
    description: "Cron job Settings tab: Deploy section title (Render parity)",
  },
  "services.deployDescription": {
    message: "此定时任务的运行方式。",
    description: "Cron job Settings tab: Deploy section description",
  },
  "services.deployEdit": {
    message: "编辑定时任务设置",
    description:
      "Cron job Deploy section: accessible label for the edit-pencil button",
  },
  "services.deploySave": {
    message: "保存",
    description: "Cron job Deploy section: save button",
  },
  "services.deployCancel": {
    message: "取消",
    description: "Cron job Deploy section: cancel edit button",
  },
  "services.deploySuccess": {
    message: "定时任务设置已保存。",
    description: "Toast after updateCronJob succeeds",
  },
  "services.deployConverging": {
    message: "操作器将在下一次协调时应用新计划。",
    description:
      "Toast description after a cron job schedule change (async convergence)",
  },
  "services.deployError": {
    message: "无法保存定时任务设置，请重试。",
    description: "Toast after updateCronJob fails",
  },
  "services.deployScheduleLabel": {
    message: "计划",
    description: "Cron job Settings tab: Deploy section schedule field label",
  },
  "services.deployScheduleHint": {
    message: "按此计划（5 段 crontab 表达式）运行该命令。",
    description: "Cron job Settings tab: Deploy section schedule help text",
  },
  "services.deploySchedulePlaceholder": {
    message: "0 * * * *",
    description: "Cron job Deploy section: schedule input placeholder",
  },
  "services.deployScheduleError": {
    message: "请输入有效的 5 段 cron 表达式，例如 0 * * * *。",
    description: "Cron job Deploy section: schedule validation error",
  },
  "services.deployScheduleRequired": {
    message: "计划表达式为必填项。",
    description: "Cron job Deploy section: schedule required validation error",
  },
  "services.deployCommandLabel": {
    message: "命令",
    description: "Cron job Settings tab: Deploy section command field label",
  },
  "services.deployCommandPlaceholder": {
    message: "例如 python script.py",
    description: "Cron job Deploy section: command input placeholder",
  },
  "services.deployCommandHint": {
    message: "覆盖镜像的默认入口命令。留空则使用镜像自身的命令。",
    description: "Cron job Deploy section: command field help text",
  },
  "services.deployCommandEmpty": {
    message: "使用镜像自身的默认命令。",
    description:
      "Cron job Settings tab: shown when spec.command is unset (no override)",
  },
  "services.buildDeployTitle": {
    message: "构建与部署",
    description:
      "Settings tab: Build & Deploy section title (w5/m13, Render parity)",
  },
  "services.buildDeployDescription": {
    message: "此服务的构建与部署来源。",
    description: "Settings tab: Build & Deploy section description",
  },
  "services.buildDeploySourceLabel": {
    message: "源代码",
    description: "Build & Deploy: repo field label (read-only)",
  },
  "services.buildDeployBranchLabel": {
    message: "分支",
    description: "Build & Deploy: branch field label (read-only)",
  },
  "services.buildDeployRootDirLabel": {
    message: "根目录",
    description: "Build & Deploy: root-directory field label",
  },
  "services.buildDeployRootDirOptional": {
    message: "可选",
    description:
      "Build & Deploy: badge next to the Root Directory label (Render parity)",
  },
  "services.buildDeployRootDirHint": {
    message:
      "如果设置，构建将从此子目录而非仓库根目录运行。此目录之外的代码更改不会触发自动部署。常用于 monorepo。",
    description: "Build & Deploy: root-directory field help text",
  },
  "services.buildDeployRootDirEmpty": {
    message: "仓库根目录",
    description: "Build & Deploy: shown when spec.rootDir is unset",
  },
  "services.buildDeployConfirmRoot": {
    message: "仓库根目录",
    description:
      "Build & Deploy: mid-sentence phrase for the confirm dialog title when clearing rootDir to empty (a dedicated key, not a lowercased buildDeployRootDirEmpty, since that transform doesn't hold in every language)",
  },
  "services.buildDeployRootDirPlaceholder": {
    message: "例如 backend",
    description: "Build & Deploy: root-directory input placeholder",
  },
  "services.buildDeployEdit": {
    message: "编辑根目录",
    description: "Build & Deploy: accessible label for the edit-pencil button",
  },
  "services.buildDeploySave": {
    message: "保存",
    description: "Build & Deploy: root-directory inline-edit save button",
  },
  "services.buildDeployCancel": {
    message: "取消",
    description: "Build & Deploy: root-directory inline-edit cancel button",
  },
  "services.buildDeployConfirmTitle": {
    message: "将根目录更改为 {value}？",
    description: "Build & Deploy: root-directory change confirm dialog title",
  },
  "services.buildDeployConfirmBody": {
    message:
      "服务将重新构建并部署，范围限定为新目录——现有请求会先完成，旧实例才会被替换。",
    description: "Build & Deploy: root-directory change confirm dialog body",
  },
  "services.buildDeploySuccess": {
    message: "根目录已更新。",
    description: "Toast after setRootDir succeeds",
  },
  "services.buildDeployError": {
    message: "无法更新根目录，请重试。",
    description: "Toast after setRootDir fails",
  },
  "services.preDeployLabel": {
    message: "预部署命令",
    description: "Build & Deploy: label for the pre-deploy command field",
  },
  "services.preDeployHint": {
    message:
      "在新镜像开始接收流量前运行一次（例如数据库迁移）。非零退出会使部署失败，并保持上一个版本继续运行。",
    description: "Build & Deploy: help text for the pre-deploy command field",
  },
  "services.preDeployPlaceholder": {
    message: "例如 npm run migrate",
    description: "Build & Deploy: placeholder for the pre-deploy command input",
  },
  "services.preDeployEmpty": {
    message: "无预部署命令",
    description: "Build & Deploy: empty state for the pre-deploy command field",
  },
  "services.preDeployEdit": {
    message: "编辑预部署命令",
    description:
      "Build & Deploy: accessible label for the pre-deploy edit-pencil button",
  },
  "services.preDeploySuccess": {
    message: "预部署命令已更新。",
    description: "Toast after setPreDeployCommand succeeds",
  },
  "services.preDeployError": {
    message: "无法更新预部署命令，请重试。",
    description: "Toast after setPreDeployCommand fails",
  },
  "services.autoDeployLabel": {
    message: "自动部署",
    description: "Build & Deploy: label for the auto-deploy toggle",
  },
  "services.autoDeployViaGitHub": {
    message: "向跟踪分支推送将通过 GitHub 应用自动重新部署。",
    description:
      "Build & Deploy: source indicator when the repo is on the connected GitHub account",
  },
  "services.autoDeployViaWebhook": {
    message:
      "只有在仓库配置了使用 BEX_WEBHOOK_SECRET 的手动 git webhook 时，推送才会重新部署。",
    description:
      "Build & Deploy: source indicator when the repo is not on the connected GitHub account",
  },
  "services.autoDeployOnSuccess": {
    message: "已开启自动部署。",
    description: "Toast after enabling auto-deploy",
  },
  "services.autoDeployOffSuccess": {
    message: "已关闭自动部署。",
    description: "Toast after disabling auto-deploy",
  },
  "services.autoDeployError": {
    message: "无法更改自动部署，请重试。",
    description: "Toast after setAutoDeploy fails",
  },
  "services.deployHookTitle": {
    message: "部署钩子",
    description: "Settings tab: secret Deploy Hook section title",
  },
  "services.deployHookDescription": {
    message: "使用一个机密 URL 从 CI 触发部署。",
    description: "Settings tab: Deploy Hook section description",
  },
  "services.deployHookURLLabel": {
    message: "部署钩子 URL",
    description: "Accessible label for the secret Deploy Hook URL field",
  },
  "services.deployHookReveal": {
    message: "显示部署钩子 URL",
    description: "Accessible label for the reveal-secret button",
  },
  "services.deployHookHide": {
    message: "隐藏部署钩子 URL",
    description: "Accessible label for the hide-secret button",
  },
  "services.deployHookCopy": {
    message: "复制部署钩子 URL",
    description: "Accessible label for the Deploy Hook copy button",
  },
  "services.deployHookCopied": {
    message: "已复制部署钩子 URL。",
    description: "Toast after copying the Deploy Hook URL",
  },
  "services.deployHookCopyError": {
    message: "无法复制部署钩子 URL。",
    description: "Toast after Deploy Hook URL clipboard failure",
  },
  "services.deployHookSecretHint": {
    message: "请保密此 URL。任何持有它的人无需 API 密钥即可部署此服务。",
    description: "Security warning below the Deploy Hook URL",
  },
  "services.deployHookRegenerate": {
    message: "重新生成钩子",
    description: "Deploy Hook rotation button",
  },
  "services.deployHookRegenerateTitle": {
    message: "重新生成部署钩子？",
    description: "Deploy Hook rotation confirmation title",
  },
  "services.deployHookRegenerateWarning": {
    message:
      "当前 URL 将立即失效。请更新使用它的所有 CI 系统、定时任务和集成。",
    description: "Deploy Hook rotation confirmation warning",
  },
  "services.deployHookCancel": {
    message: "取消",
    description: "Deploy Hook rotation confirmation cancel button",
  },
  "services.deployHookRegenerateConfirm": {
    message: "重新生成",
    description: "Deploy Hook rotation confirmation action",
  },
  "services.deployHookRegenerated": {
    message: "部署钩子已重新生成，旧 URL 已失效。",
    description: "Toast after successful Deploy Hook rotation",
  },
  "services.deployHookRegenerateError": {
    message: "无法重新生成部署钩子，请重试。",
    description: "Toast after Deploy Hook rotation fails",
  },
  "services.deployHookLoadError": {
    message: "无法加载部署钩子 URL。",
    description: "Deploy Hook section query error",
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
  "services.typeStatic": {
    message: "静态站点",
    description:
      "Service-type badge: built output served from the object-store origin",
  },
  "services.staticTitle": {
    message: "静态站点",
    description:
      "Settings section title for a static site's publish dir + edge rules",
  },
  "services.staticDescription": {
    message: "发布的输出目录，以及提供服务时应用的边缘规则。",
    description: "Static Site settings section description",
  },
  "services.staticEdit": {
    message: "编辑",
    description: "Edit an inline field",
  },
  "services.staticSave": {
    message: "保存",
    description: "Save an inline field",
  },
  "services.staticCancel": {
    message: "取消",
    description: "Cancel an inline edit",
  },
  "services.publishPathLabel": {
    message: "发布目录",
    description: "Label for a static site's publishPath",
  },
  "services.publishPathPlaceholder": {
    message: "dist",
    description: "Placeholder for the publish directory input",
  },
  "services.publishPathHint": {
    message:
      "作为站点根目录提供服务的构建输出目录（例如 dist、build、public）。修改后将重新发布站点。",
    description: "Help text under the publish directory field",
  },
  "services.publishPathSaved": {
    message: "发布目录已更新",
    description: "Toast after saving a static site's publishPath",
  },
  "services.publishPathRepublishNote": {
    message: "站点将很快重新发布。",
    description: "Toast description after changing publishPath",
  },
  "services.publishPathError": {
    message: "无法更新发布目录",
    description: "Error toast for a failed publishPath change",
  },
  "services.routesTitle": {
    message: "重定向与重写",
    description: "Title for the static-site routes editor",
  },
  "services.routesHint": {
    message:
      "按顺序匹配，首个匹配生效。重定向返回 301；重写提供另一路径的内容（SPA 回退将 /* 重写为 /index.html）。",
    description: "Help text for the routes editor",
  },
  "services.routeAdd": { message: "添加规则", description: "Add a route rule" },
  "services.routeType": { message: "类型", description: "Route type column" },
  "services.routeSource": {
    message: "源路径",
    description: "Route source-path column",
  },
  "services.routeDestination": {
    message: "目标路径",
    description: "Route destination-path column",
  },
  "services.routeRewrite": {
    message: "重写",
    description: "Route type: rewrite",
  },
  "services.routeRedirect": {
    message: "重定向",
    description: "Route type: redirect",
  },
  "services.routeRemove": {
    message: "删除规则",
    description: "Remove a route rule (aria-label)",
  },
  "services.routesSave": {
    message: "保存路由",
    description: "Save the routes list",
  },
  "services.staticRoutesSaved": {
    message: "路由已更新",
    description: "Toast after saving routes",
  },
  "services.staticRoutesError": {
    message: "无法更新路由",
    description: "Error toast for a failed routes save",
  },
  "services.headersTitle": {
    message: "响应头",
    description: "Title for the static-site custom-headers editor",
  },
  "services.headersHint": {
    message: "为路径匹配的响应添加的自定义响应头。",
    description: "Help text for the headers editor",
  },
  "services.headerAdd": {
    message: "添加响应头",
    description: "Add a custom header",
  },
  "services.headerPath": { message: "路径", description: "Header path column" },
  "services.headerName": { message: "名称", description: "Header name column" },
  "services.headerValue": {
    message: "值",
    description: "Header value column",
  },
  "services.headerRemove": {
    message: "删除响应头",
    description: "Remove a header (aria-label)",
  },
  "services.headersSave": {
    message: "保存响应头",
    description: "Save the headers list",
  },
  "services.staticHeadersSaved": {
    message: "响应头已更新",
    description: "Toast after saving headers",
  },
  "services.staticHeadersError": {
    message: "无法更新响应头",
    description: "Error toast for a failed headers save",
  },
  "services.typeUnknown": {
    message: "服务",
    description: "Service-type badge fallback for an unrecognized type",
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
  "services.dangerZoneTitle": {
    message: "危险区域",
    description: "Settings tab delete section title (destructive)",
  },
  "services.dangerZoneDescription": {
    message: "删除服务将永久移除该服务、其部署及其 URL，且无法撤销。",
    description: "Settings tab delete section description",
  },
  "services.deleteButton": {
    message: "删除服务",
    description: "Danger-zone button that opens the delete-confirm dialog",
  },
  "services.deleteConfirmTitle": {
    message: "删除 {name}？",
    description: "Delete-confirm dialog title",
  },
  "services.deleteConfirmBody": {
    message: "此操作将永久移除该服务、其部署及其 URL，且无法撤销。",
    description: "Delete-confirm dialog body",
  },
  "services.deleteConfirmPrompt": {
    message: "输入 {name} 以确认",
    description: "Delete-confirm input label naming the immutable service id",
  },
  "services.deleteCancel": {
    message: "取消",
    description: "Delete-confirm dialog cancel button",
  },
  "services.deleteConfirm": {
    message: "删除服务",
    description:
      "Delete-confirm dialog submit button (armed once the name matches)",
  },
  "services.deleteSuccess": {
    message: "已删除 {name}",
    description: "Toast on a successful service delete",
  },
  "services.deleteError": {
    message: "无法删除 {name}",
    description: "Toast on a failed service delete",
  },
  "services.newServiceButton": {
    message: "新建服务",
    description:
      "Button on the services list page that opens the create wizard",
  },
  "services.createTitle": {
    message: "新建服务",
    description: "Create-wizard page title",
  },
  "services.createDescription": {
    message: "从 Git 仓库或 Docker 镜像部署 Web 服务。",
    description: "Create-wizard page subtitle",
  },
  "services.createSourceTitle": {
    message: "来源",
    description: "Create-wizard source-picker section label",
  },
  "services.createTabGitHub": {
    message: "GitHub",
    description: "Create-wizard source-tab label for connected GitHub repos",
  },
  "services.createTabPublicGit": {
    message: "公开 Git URL",
    description: "Create-wizard source-tab label for a public git URL",
  },
  "services.createTabImage": {
    message: "已有镜像",
    description: "Create-wizard source-tab label for a pre-built Docker image",
  },
  "services.createRepoSearchPlaceholder": {
    message: "搜索仓库…",
    description: "Create-wizard GitHub tab repo-search input placeholder",
  },
  "services.createRepoPrivateBadge": {
    message: "私有",
    description: "Badge on a private GitHub repo row in the repo picker",
  },
  "services.createRepoEmpty": {
    message: "未找到仓库。",
    description:
      "Create-wizard GitHub tab empty state (no repos in the installation)",
  },
  "services.createRepoNoMatch": {
    message: "没有匹配您搜索的仓库。",
    description:
      "Create-wizard GitHub tab empty state when the search filter returns nothing",
  },
  "services.createGitConnectPromptTitle": {
    message: "连接 GitHub",
    description:
      "Create-wizard GitHub tab connect-prompt heading (no GitHub connection yet)",
  },
  "services.createGitConnectPromptBody": {
    message: "连接您的 GitHub 账号，以便从私有或公开仓库部署服务。",
    description:
      "Create-wizard GitHub tab connect-prompt body (no GitHub connection yet)",
  },
  "services.createGitConnectButton": {
    message: "连接 GitHub",
    description:
      "Create-wizard GitHub tab button that opens the GitHub App install flow",
  },
  "services.createPublicUrlLabel": {
    message: "仓库 URL",
    description: "Create-wizard Public Git URL tab input label",
  },
  "services.createPublicUrlPlaceholder": {
    message: "https://github.com/you/your-repo",
    description: "Create-wizard Public Git URL tab input placeholder",
  },
  "services.createPublicUrlError": {
    message: "请输入有效的 https://、git@ 或 git:// URL。",
    description: "Create-wizard Public Git URL tab validation message",
  },
  "services.createImageLabel": {
    message: "Docker 镜像",
    description: "Create-wizard Existing Image tab input label",
  },
  "services.createImagePlaceholder": {
    message: "docker.io/library/nginx:latest",
    description: "Create-wizard Existing Image tab input placeholder",
  },
  "services.createSettingsTitle": {
    message: "设置",
    description: "Create-wizard settings section heading",
  },
  "services.createFieldName": {
    message: "名称",
    description: "Create-wizard name input label",
  },
  "services.createFieldNamePlaceholder": {
    message: "my-service",
    description: "Create-wizard name input placeholder",
  },
  "services.createFieldNameError": {
    message:
      "请使用小写字母、数字和连字符（最多 30 个字符），且不能以连字符开头或结尾。",
    description: "Create-wizard name validation message",
  },
  "services.createFieldNameTaken": {
    message: "该名称已被使用",
    description:
      "Create-wizard inline error when the service name is already taken in the current workspace (w4/m19)",
  },
  "services.createFieldNameUseSuggestion": {
    message: "使用 {name}",
    description:
      "Create-wizard button offering the suggested free name in place of a taken one (w4/m19)",
  },
  "services.createFieldNameChecking": {
    message: "正在检查可用性…",
    description:
      "Create-wizard transient message while the debounced name-availability check is in flight (w4/m19)",
  },
  "services.createFieldBranch": {
    message: "分支",
    description: "Create-wizard branch input label (git sources)",
  },
  "services.createFieldBranchPlaceholder": {
    message: "main",
    description: "Create-wizard branch input placeholder",
  },
  "services.createFieldRootDir": {
    message: "根目录",
    description: "Create-wizard root-directory input label",
  },
  "services.createFieldRootDirPlaceholder": {
    message: "例如：backend",
    description: "Create-wizard root-directory input placeholder",
  },
  "services.createFieldRootDirHint": {
    message: "构建所使用的子目录。留空则使用仓库根目录。",
    description: "Create-wizard root-directory input hint text",
  },
  "services.createFieldRuntime": {
    message: "运行时",
    description: "Create-wizard Render-compatible runtime selector label",
  },
  "services.createFieldBuildCommand": {
    message: "构建命令",
    description: "Create-wizard Render-compatible build command label",
  },
  "services.createFieldStartCommand": {
    message: "启动命令",
    description: "Create-wizard Render-compatible start command label",
  },
  "services.createRuntimeNode": {
    message: "Node",
    description: "Create-wizard Node runtime option",
  },
  "services.createRuntimePython": {
    message: "Python 3",
    description: "Create-wizard Python runtime option",
  },
  "services.createRuntimeGo": {
    message: "Go",
    description: "Create-wizard Go runtime option",
  },
  "services.createRuntimeRuby": {
    message: "Ruby",
    description: "Create-wizard Ruby runtime option",
  },
  "services.createRuntimeRust": {
    message: "Rust",
    description: "Create-wizard Rust runtime option",
  },
  "services.createRuntimeElixir": {
    message: "Elixir",
    description: "Create-wizard Elixir runtime option",
  },
  "services.createRuntimeDocker": {
    message: "Docker",
    description: "Create-wizard Docker runtime option",
  },
  "services.createFieldPlan": {
    message: "实例类型",
    description: "Create-wizard plan-picker section label",
  },
  "services.createFieldAutoDeploy": {
    message: "推送时自动部署",
    description: "Create-wizard auto-deploy toggle label",
  },
  "services.createFieldAutoDeployHint": {
    message: "推送到该分支时自动重新部署。",
    description: "Create-wizard auto-deploy toggle hint text",
  },
  "services.createCancel": {
    message: "取消",
    description: "Create-wizard cancel button",
  },
  "services.createSubmit": {
    message: "部署服务",
    description: "Create-wizard submit button",
  },
  "services.createSuccess": {
    message: "正在部署 {name}…",
    description: "Toast shown after createService succeeds",
  },
  "services.createError": {
    message: "无法创建 {name}，请重试。",
    description: "Toast shown after createService fails",
  },
  "services.scalingInstanceCount": {
    message: "实例数量",
    description:
      "Settings row label for the manual instance-count stepper (w5/m16)",
  },
  "services.scalingInstanceCountHint": {
    message: "同时运行的实例数量。",
    description: "Settings row help text for the manual instance-count stepper",
  },
  "services.scalingDecrement": {
    message: "减少实例数量",
    description: "aria-label for the − stepper button",
  },
  "services.scalingIncrement": {
    message: "增加实例数量",
    description: "aria-label for the + stepper button",
  },
  "services.scalingSaveCount": {
    message: "保存",
    description: "Save button label on the instance-count stepper",
  },
  "services.scaleSuccess": {
    message: "已缩放至 {count} 个实例。",
    description: "Toast shown after scaleService succeeds",
  },
  "services.scaleError": {
    message: "更新实例数量失败。",
    description: "Toast shown after scaleService fails",
  },
  "services.createTypePickerTitle": {
    message: "服务类型",
    description: "Label above the service type picker in the create wizard",
  },
  "services.createTypeWebDesc": {
    message: "在公共 URL 上公开您的服务",
    description:
      "Description shown under the Web Service type card in the create wizard",
  },
  "services.createTypePrivateDesc": {
    message: "仅在平台网络内部可访问",
    description: "Description shown under the Private Service type card",
  },
  "services.createTypeWorkerDesc": {
    message: "无端口或 URL 的后台处理进程",
    description: "Description shown under the Background Worker type card",
  },
  "services.createTypeCronDesc": {
    message: "按定时计划运行命令",
    description: "Description shown under the Cron Job type card",
  },
  "services.createTypeStaticDesc": {
    message: "从对象存储构建并托管静态站点",
    description: "Description shown under the Static Site type card",
  },
  "services.createFieldSchedule": {
    message: "计划表达式",
    description: "Label for the cron schedule field in the create wizard",
  },
  "services.createFieldSchedulePlaceholder": {
    message: "0 0 * * *",
    description: "Placeholder for the cron schedule field",
  },
  "services.createFieldScheduleHint": {
    message: "5 字段 crontab 表达式（分 时 日 月 周）。",
    description: "Hint text under the schedule field",
  },
  "services.createFieldScheduleError": {
    message: "请输入有效的 5 字段 cron 表达式，例如 0 0 * * *。",
    description: "Validation error for an invalid cron expression",
  },
  "services.createFieldCommand": {
    message: "命令",
    description: "Label for the command field in the create wizard",
  },
  "services.createFieldCommandPlaceholder": {
    message: "python script.py",
    description: "Placeholder for the command field",
  },
  "services.createFieldCommandHint": {
    message: "每次计划调用时运行的命令。",
    description: "Hint text under the command field",
  },
  "services.createFieldPublishPath": {
    message: "发布目录",
    description: "Label for the publish directory field in the create wizard",
  },
  "services.createFieldPublishPathPlaceholder": {
    message: "dist",
    description: "Placeholder for the publish directory field",
  },
  "services.createFieldPublishPathHint": {
    message: "作为站点根目录的构建输出目录（如 dist、build、public）。",
    description: "Hint text under the publish directory field",
  },
  "services.createNoPublicUrlNote": {
    message: "此服务类型没有公共 URL。",
    description:
      "Note shown for private/worker types that don't produce a public URL",
  },
  "services.createFieldEnvVarsTitle": {
    message: "环境变量",
    description: "Section heading for env vars in the create wizard",
  },
  "services.createFieldEnvVarsAdd": {
    message: "添加变量",
    description: "Button to add an env var row in the create wizard",
  },
  "services.createFieldEnvVarsRemove": {
    message: "删除",
    description: "Button to remove an env var row in the create wizard",
  },
  "services.createFieldEnvVarsKey": {
    message: "键名",
    description: "Label for the env var key column in the create wizard",
  },
  "services.createFieldEnvVarsValue": {
    message: "值",
    description: "Label for the env var value column in the create wizard",
  },
  "services.createFieldEnvVarsKeyPlaceholder": {
    message: "KEY_NAME",
    description: "Placeholder for the env var key input in the create wizard",
  },
  "services.createFieldEnvVarsValuePlaceholder": {
    message: "值",
    description: "Placeholder for the env var value input in the create wizard",
  },
  "services.createFieldEnvVarsKeyError": {
    message: "键名必须以字母或下划线开头，只能包含字母、数字和下划线。",
    description:
      "Error shown when an env var key is invalid in the create wizard",
  },
  "services.navEvents": {
    message: "事件",
    description: "Service-detail nav item (events tab)",
  },
  "services.eventsTitle": {
    message: "活动",
    description: "Events tab card title",
  },
  "services.eventsEmpty": {
    message: "暂无事件。",
    description: "Events tab empty state",
  },
  "services.eventsManualDeploy": {
    message: "手动部署",
    description: "Button to trigger a new deploy",
  },
  "services.deployMenuLatestCommit": {
    message: "部署最新提交",
    description:
      "Manual Deploy dropdown item, repo-backed service: rebuild and redeploy from the branch's HEAD",
  },
  "services.deployMenuLatestImage": {
    message: "部署最新镜像",
    description:
      "Manual Deploy dropdown item, image-backed service (no repo to rebuild from)",
  },
  "services.deployMenuRestart": {
    message: "重启服务",
    description:
      "Manual Deploy dropdown item: roll the service's pods without rebuilding",
  },
  "services.deployConfirmCommitTitle": {
    message: "部署 {branch} 分支的最新提交？",
    description: "Confirm dialog title for a repo-backed manual deploy",
  },
  "services.deployConfirmCommitBody": {
    message: "将使用 {branch} 分支的最新提交重新构建并部署「{name}」。",
    description: "Confirm dialog body for a repo-backed manual deploy",
  },
  "services.deployConfirmImageTitle": {
    message: "重新部署「{name}」？",
    description: "Confirm dialog title for an image-backed manual deploy",
  },
  "services.deployConfirmImageBody": {
    message: "将使用当前镜像重启「{name}」。没有可重新构建的源代码仓库。",
    description: "Confirm dialog body for an image-backed manual deploy",
  },
  "services.eventsManualDeployConfirmTitle": {
    message: "触发新的部署？",
    description: "Manual deploy confirm dialog title",
  },
  "services.eventsManualDeployConfirmBody": {
    message: "这将从当前镜像或分支重新构建并重新部署该服务。",
    description: "Manual deploy confirm dialog body",
  },
  "services.eventsCancelDeploy": {
    message: "取消",
    description: "Button to cancel an in-progress deploy",
  },
  "services.eventsCancelConfirmTitle": {
    message: "取消此次部署？",
    description: "Cancel deploy confirm dialog title",
  },
  "services.eventsCancelConfirmBody": {
    message: "正在进行的部署将被停止，最近成功的部署仍保持运行。",
    description: "Cancel deploy confirm dialog body",
  },
  "services.eventsRollback": {
    message: "回滚到此次部署",
    description: "Button to roll back to a specific deploy",
  },
  "services.eventsRollbackConfirmTitle": {
    message: "回滚到此次部署？",
    description: "Rollback confirm dialog title",
  },
  "services.eventsRollbackConfirmBody": {
    message: "服务将从此次部署使用的镜像重新部署。",
    description: "Rollback confirm dialog body",
  },
  "services.eventsConfirmProceed": {
    message: "继续",
    description: "Confirm dialog proceed button",
  },
  "services.eventsConfirmCancel": {
    message: "返回",
    description: "Confirm dialog cancel button",
  },
  "services.triggerDeploySuccess": {
    message: "部署已触发。",
    description: "Toast after triggerDeploy succeeds",
  },
  "services.triggerDeployError": {
    message: "无法触发部署。",
    description: "Toast after triggerDeploy fails",
  },
  "services.cancelDeploySuccess": {
    message: "部署已取消。",
    description: "Toast after cancelDeploy succeeds",
  },
  "services.cancelDeployError": {
    message: "无法取消部署。",
    description: "Toast after cancelDeploy fails",
  },
  "services.rollbackSuccess": {
    message: "回滚已触发。",
    description: "Toast after rollbackService succeeds",
  },
  "services.rollbackError": {
    message: "无法回滚。",
    description: "Toast after rollbackService fails",
  },
  "services.eventsStatusLive": {
    message: "运行中",
    description: "Deploy status: live",
  },
  "services.eventsStatusInProgress": {
    message: "进行中",
    description: "Deploy status: update_in_progress",
  },
  "services.eventsStatusFailed": {
    message: "失败",
    description: "Deploy status: update_failed",
  },
  "services.eventsStatusCanceled": {
    message: "已取消",
    description: "Deploy status: canceled",
  },
  "services.eventsPreDeployRunning": {
    message: "预部署命令运行中",
    description: "Deploy row: the pre-deploy step is in progress",
  },
  "services.eventsPreDeploySucceeded": {
    message: "预部署命令成功",
    description: "Deploy row: the pre-deploy step passed",
  },
  "services.eventsPreDeployFailed": {
    message: "预部署命令失败",
    description:
      "Deploy row: the pre-deploy step failed (distinct from a health-check failure)",
  },
  "services.eventsRolledBackFrom": {
    message: "从 {target} 回滚",
    description: "Deploy row: provenance note when trigger=rollback",
  },
  "services.capLimitTitle": {
    message: "已达到服务上限",
    description:
      "Alert title when the workspace's service creation cap is hit (w7/m9)",
  },
  "services.capLimitUpgrade": {
    message: "升级方案",
    description: "Upgrade CTA button inside the cap-limit Alert (w7/m9)",
  },
};

export default zhServices;
