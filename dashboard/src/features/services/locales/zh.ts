import type { TranslationEntry } from "@/i18n";

const zhServices: Record<string, TranslationEntry> = {
  "services.protectedConfirmationTitle": {
    message: "受保护环境确认",
    description: "Title of a protected-service destructive-action retry dialog",
  },
  "services.protectedConfirmationBody": {
    message:
      "“{name}”属于受保护环境。请输入 bex-api 返回的完整 sudo 命令以继续。",
    description: "Body of a protected-service destructive-action retry dialog",
  },
  "services.protectedConfirmationPrompt": {
    message: "在下方输入 {phrase} 以确认。",
    description:
      "Body prompt naming the exact protected-action confirmation phrase (rendered bold by SudoCommandField)",
  },
  "services.connect": {
    message: "连接",
    description: "Open the service connection menu",
  },
  "services.connectSSH": {
    message: "SSH",
    description: "SSH section label in the service connection menu",
  },
  "services.sshCopy": {
    message: "复制 SSH 命令",
    description: "Copy service SSH command button",
  },
  "services.sshCopied": {
    message: "已复制 SSH 命令",
    description: "Successful SSH command copy",
  },
  "services.sshCopyError": {
    message: "无法复制 SSH 命令",
    description: "Failed SSH command copy",
  },
  "services.sshUnavailable": {
    message: "SSH 不可用",
    description: "服务未公布 SSH 地址时的标题状态",
  },
  "services.sshUnavailableHint": {
    message: "SSH 需要正在运行的付费 Web、私有或后台服务，以及已启用的网关。",
    description: "服务没有 SSH 地址时的说明",
  },
  "services.shellTitle": {
    message: "Shell",
    description: "Running-instance SSH connection page title",
  },
  "services.shellDescription": {
    message: "从本地终端打开正在运行的服务实例 Shell。",
    description: "Running-instance SSH connection page description",
  },
  "services.shellConnectionTitle": {
    message: "通过 SSH 连接",
    description: "SSH connection card title",
  },
  "services.shellConnectionDescription": {
    message: "bex 会连接到现有的就绪实例，不会创建单独的 Shell 实例。",
    description: "SSH running-instance behavior explanation",
  },
  "services.shellCommand": {
    message: "SSH 命令",
    description: "Label for a copy-ready service SSH command",
  },
  "services.shellManageKeys": {
    message: "管理 SSH 公钥",
    description: "Link from a service shell page to account SSH key settings",
  },
  "services.shellSessionLifecycle": {
    message: "该命令会选择一个就绪实例。重启、重新部署或暂停服务会关闭会话。",
    description: "SSH session selection and lifecycle guidance",
  },
  "services.shellUnavailableTitle": {
    message: "Shell 访问不可用",
    description: "Unavailable SSH connection card title",
  },
  "services.shellUnavailableBody": {
    message:
      "Shell 访问需要正在运行的付费 Web、私有或后台服务，以及已启用的 SSH 网关。",
    description: "Unavailable SSH connection card explanation",
  },
  "services.shellWebTitle": {
    message: "Web Shell",
    description: "In-browser terminal card title",
  },
  "services.shellWebDescription": {
    message: "在浏览器中直接进入正在运行的实例执行命令。",
    description: "In-browser terminal card description",
  },
  "services.shellConnecting": {
    message: "正在连接到运行中的实例…",
    description: "Web shell status while the terminal is connecting",
  },
  "services.shellConnected": {
    message: "已连接",
    description: "Web shell status when the terminal stream is live",
  },
  "services.shellClosed": {
    message: "会话已关闭",
    description: "Web shell status when the terminal stream has ended",
  },
  "services.shellErrorStatus": {
    message: "连接错误",
    description: "Web shell status when the session failed to connect or errored",
  },
  "services.shellReconnect": {
    message: "重新连接",
    description: "Button that re-opens a closed web shell session",
  },
  "services.shellConnect": {
    message: "启动 Shell",
    description: "Button that opens a web shell session",
  },
  "services.shellErrorGeneric": {
    message: "无法启动 Shell 会话。",
    description: "Web shell generic connection error",
  },
  "services.shellErrorUnavailable": {
    message: "此平台未启用浏览器内 Shell。请改用下方的 SSH 命令。",
    description: "Web shell error when the browser transport is unconfigured (503)",
  },
  "services.shellInstanceLabel": {
    message: "实例",
    description: "Label for the web shell instance picker",
  },
  "services.shellInstanceAny": {
    message: "任意就绪实例",
    description: "Web shell instance picker option that selects a random ready replica",
  },
  "services.shellInstanceSelect": {
    message: "选择实例",
    description: "Placeholder for the web shell instance picker",
  },
  "services.shellSshFallback": {
    message: "想用自己的终端？请使用下方的 SSH 命令。",
    description: "Hint pointing web shell users to the copy-ready SSH command",
  },
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
  "services.colSlug": {
    message: "Slug",
    description:
      "Service detail header fact (globally-unique platform-host segment, Render's slug field)",
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
  "services.detailErrorTitle": {
    message: "无法加载服务",
    description: "Service detail query error card title",
  },
  "services.detailErrorBody": {
    message: "对 bex-api 的请求失败。该服务可能仍然存在。",
    description: "Service detail query error card body",
  },
  "services.emptyTitle": {
    message: "还没有服务",
    description: "Services list empty state title",
  },
  "services.emptyBody": {
    message: "部署你的第一个 App，它就会出现在这里。",
    description: "Services list empty state body",
  },
  "services.navPlan": {
    message: "套餐",
    description: "Service sidebar link to the instance-type (plan) page",
  },
  "services.headerServiceId": {
    message: "服务 ID：",
    description: "Service-detail header metadata label for the service id",
  },
  "services.headerServicePhase": {
    message: "服务",
    description:
      "Service-detail header label distinguishing operator/App phase from deploy status",
  },
  "services.headerLatestDeploy": {
    message: "最新部署",
    description: "Service-detail header label for the newest deploy status",
  },
  "services.headerRuntime": {
    message: "运行时",
    description: "Service-detail header label for the selected runtime",
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
  "services.scalingDisableConfirmTitle": {
    message: "关闭自动扩缩容？",
    description: "Autoscaling disable confirmation dialog title (w7/m43)",
  },
  "services.scalingDisableConfirmBody": {
    message: "服务将按「手动扩缩容」中指定的固定实例数量运行。",
    description: "Autoscaling disable confirmation dialog body",
  },
  "services.scalingDisableConfirmAction": {
    message: "关闭",
    description: "Autoscaling disable confirmation dialog confirm button",
  },
  "services.scalingManualTitle": {
    message: "手动扩缩容",
    description: "Scaling page: manual instance-count card title (w7/m43)",
  },
  "services.scalingManualDescription": {
    message:
      "运行多个自动负载均衡的实例。所有实例使用相同的实例类型并按此计费。",
    description: "Scaling page: manual instance-count card description",
  },
  "services.scalingManualInstances": {
    message: "实例",
    description: "Scaling page: manual instance-count slider label",
  },
  "services.scalingMetricsTitle": {
    message: "近期指标",
    description: "Scaling page: recent-metrics section title (w7/m43)",
  },
  "services.scalingMetricsNote": {
    message: "显示过去 48 小时的指标。",
    description: "Scaling page: recent-metrics window note",
  },
  "services.scalingMetricsViewAll": {
    message: "查看全部指标。",
    description: "Scaling page: link to the full Metrics tab",
  },
  "services.scalingMetricsMemory": {
    message: "平均内存利用率",
    description: "Scaling page: averaged memory utilization chart title",
  },
  "services.scalingMetricsCPU": {
    message: "平均 CPU 利用率",
    description: "Scaling page: averaged CPU utilization chart title",
  },
  "services.scalingMetricsAcross": {
    message: "所有实例平均",
    description: "Scaling page: recent-metrics chart subtitle",
  },
  "services.scalingMetricsEmpty": {
    message: "过去 48 小时未采集到数据",
    description: "Scaling page: recent-metrics empty state",
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
  "services.envExport": {
    message: "导出",
    description:
      "Environment tab button that downloads all service variables as dotenv",
  },
  "services.envExportSuccess": {
    message: "环境变量已导出",
    description:
      "Toast after every current environment value was safely exported",
  },
  "services.envExportError": {
    message: "无法导出完整环境，未下载任何文件。",
    description:
      "Fail-closed toast when any environment value cannot be revealed",
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
  "services.envGenerate": {
    message: "生成",
    description: "Generate a cryptographically random environment value",
  },
  "services.envGeneratePlaceholder": {
    message: "保存时安全生成",
    description:
      "Environment value placeholder while server generation is selected",
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
  "services.settingsMaxShutdownDelay": {
    message: "最大关闭延迟",
    description: "Settings row label for the graceful SIGTERM window",
  },
  "services.settingsMaxShutdownDelayHint": {
    message: "收到 SIGTERM 后等待 1–300 秒再强制停止进程（默认 30 秒）。",
    description: "Settings row help for the shutdown-delay range",
  },
  "services.maxShutdownDelaySeconds": {
    message: "{seconds} 秒",
    description: "Current graceful-shutdown delay in seconds",
  },
  "services.maxShutdownDelayEdit": {
    message: "编辑最大关闭延迟",
    description: "Accessible label for the shutdown-delay edit button",
  },
  "services.maxShutdownDelaySave": {
    message: "保存最大关闭延迟",
    description: "Accessible label for the shutdown-delay save button",
  },
  "services.maxShutdownDelayCancel": {
    message: "取消编辑最大关闭延迟",
    description: "Accessible label for the shutdown-delay cancel button",
  },
  "services.maxShutdownDelaySuccess": {
    message: "最大关闭延迟已更新。",
    description: "Toast after setMaxShutdownDelay succeeds",
  },
  "services.maxShutdownDelayError": {
    message: "无法更新最大关闭延迟。",
    description: "Toast after setMaxShutdownDelay fails",
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
  "services.settingsHealthCheckPathEdit": {
    message: "编辑健康检查路径",
    description:
      "Settings tab: accessible label for the health-check path edit-pencil button",
  },
  "services.healthCheckPathSuccess": {
    message: "健康检查路径已更新。",
    description: "Toast after setHealthCheckPath succeeds",
  },
  "services.healthCheckPathError": {
    message: "无法更新健康检查路径。",
    description: "Toast after setHealthCheckPath fails",
  },
  "services.settingsNotificationsTitle": {
    message: "通知",
    description: "Settings tab: Notifications section card title",
  },
  "services.settingsNotificationsDescription": {
    message: "选择要接收的此服务部署通知。",
    description: "Settings tab: Notifications section card description",
  },
  "services.notificationsLabel": {
    message: "服务通知",
    description: "Settings tab: service notification policy label",
  },
  "services.notificationsHint": {
    message: "覆盖此服务的工作区默认设置。工作区默认仅发送失败通知。",
    description: "Settings tab: service notification policy explanation",
  },
  "services.notificationsOptionDefault": {
    message: "使用工作区默认设置（仅失败通知）",
    description: "Service notification option: inherit workspace default",
  },
  "services.notificationsOptionAll": {
    message: "所有通知",
    description: "Service notification option: all lifecycle mail",
  },
  "services.notificationsOptionFailure": {
    message: "仅失败通知",
    description: "Service notification option: failures only",
  },
  "services.notificationsOptionNone": {
    message: "无",
    description: "Service notification option: no mail",
  },
  "services.notificationsSuccess": {
    message: "服务通知设置已更新。",
    description: "Toast after notification policy update succeeds",
  },
  "services.notificationsError": {
    message: "无法更新服务通知设置。",
    description: "Toast after notification policy update fails",
  },
  "services.notifyOnFailLabel": {
    message: "部署失败通知",
    description: "Settings tab: notifyOnFail row label",
  },
  "services.notifyOnFailHint": {
    message:
      "默认遵循每位成员自己的通知偏好；你可以为此服务单独强制开启或关闭。",
    description: "Settings tab: notifyOnFail row hint text",
  },
  "services.notifyOnFailOptionDefault": {
    message: "使用成员偏好",
    description: "notifyOnFail select option: default",
  },
  "services.notifyOnFailOptionNotify": {
    message: "始终通知",
    description: "notifyOnFail select option: notify",
  },
  "services.notifyOnFailOptionIgnore": {
    message: "从不通知",
    description: "notifyOnFail select option: ignore",
  },
  "services.notifyOnFailSuccess": {
    message: "通知设置已更新。",
    description: "Toast after setNotifyOnFail succeeds",
  },
  "services.notifyOnFailError": {
    message: "无法更新通知设置。",
    description: "Toast after setNotifyOnFail fails",
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
  "services.domainRedirectsTo": {
    message: "重定向到 {canonical}",
    description:
      "Note under an auto-paired sibling domain showing its canonical redirect target",
  },
  "services.domainRedirectsHere": {
    message: "{sibling} 重定向到此域名",
    description:
      "Note under a canonical domain showing which auto-paired sibling redirects to it",
  },
  "services.domainPairedDnsTitle": {
    message: "{sibling} 已自动添加并重定向到 {canonical} — 也请为它配置 DNS",
    description:
      "Heading of the second DNS-record block in the add dialog when the add auto-paired a www<->apex sibling (w6/m23)",
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
    message: "控制你的服务是否在平台子域名上响应，可与自定义域名同时生效。",
    description: "Settings tab platform-subdomain section description",
  },
  "services.platformSubdomainEnabled": {
    message: "已启用",
    description: "Platform-subdomain toggle label when the subdomain is active",
  },
  "services.platformSubdomainDisabled": {
    message: "已禁用",
    description:
      "Platform-subdomain toggle label when the subdomain is disabled",
  },
  "services.platformSubdomainPending": {
    message: "服务运行后将分配平台 URL。",
    description: "Platform-subdomain state when the service has no URL yet",
  },
  "services.platformSubdomainDisabledNote": {
    message: "平台子域名已禁用。你的服务只能通过自定义域名访问。",
    description:
      "Platform-subdomain note shown when the policy is set to disabled",
  },
  "services.platformSubdomainToggleLabel": {
    message: "切换平台子域名",
    description: "Accessible label for the platform-subdomain Switch",
  },
  "services.subdomainPolicySuccess": {
    message: "平台子域名设置已更新。",
    description: "Toast on successful setSubdomainPolicy mutation",
  },
  "services.subdomainPolicyError": {
    message: "无法更新平台子域名设置，请重试。",
    description: "Toast on failed setSubdomainPolicy mutation",
  },
  "services.maintenanceModeTitle": {
    message: "维护模式",
    description: "Settings tab maintenance-mode section title",
  },
  "services.maintenanceModeDescription": {
    message:
      "让此服务在维护页面后下线，而不暂停它——其 Pod 保持运行，只是公共流量被重定向。",
    description: "Settings tab maintenance-mode section description",
  },
  "services.maintenanceModeEnabled": {
    message: "已启用",
    description: "Maintenance-mode toggle label when active",
  },
  "services.maintenanceModeDisabled": {
    message: "已禁用",
    description: "Maintenance-mode toggle label when inactive",
  },
  "services.maintenanceModeSwitchHint": {
    message: "此服务提供的所有主机都将显示维护页面。",
    description: "Maintenance-mode section hint text next to the switch",
  },
  "services.maintenanceModePaidOnly": {
    message: "维护模式仅适用于付费 Web 服务套餐。",
    description: "Maintenance mode free-plan eligibility note",
  },
  "services.maintenanceModeToggleLabel": {
    message: "切换维护模式",
    description: "Accessible label for the maintenance-mode Switch",
  },
  "services.maintenanceModeUriLabel": {
    message: "自定义维护页面 URL",
    description: "Label for the maintenance-mode custom-page URL field",
  },
  "services.maintenanceModeUriPlaceholder": {
    message: "https://status.example.com/maintenance（可选）",
    description: "Placeholder for the maintenance-mode custom-page URL field",
  },
  "services.maintenanceModeUriHint": {
    message:
      "将被获取并代替默认页面提供。留空以使用 bex 的默认维护页面。不能指向此服务自身的 URL。",
    description: "Hint text under the maintenance-mode custom-page URL field",
  },
  "services.maintenanceModeSaveUri": {
    message: "保存",
    description: "Save button for the maintenance-mode custom-page URL field",
  },
  "services.maintenanceModeEnableAction": {
    message: "启用维护模式",
    description: "Confirm-dialog action button for enabling maintenance mode",
  },
  "services.confirmMaintenanceModeTitle": {
    message: "启用维护模式？",
    description: "Confirm-dialog title for enabling maintenance mode",
  },
  "services.confirmMaintenanceModeBody": {
    message:
      "{name} 将向每位访问者显示维护页面，直到你禁用此设置。服务的 Pod 将继续运行。",
    description: "Confirm-dialog body for enabling maintenance mode",
  },
  "services.maintenanceModeEnabledSuccess": {
    message: "维护模式已启用。",
    description: "Toast on successfully enabling maintenance mode",
  },
  "services.maintenanceModeDisabledSuccess": {
    message: "维护模式已禁用。",
    description: "Toast on successfully disabling maintenance mode",
  },
  "services.maintenanceModeError": {
    message: "无法更新维护模式，请重试。",
    description: "Toast on failed setMaintenanceMode mutation",
  },
  "services.maintenanceModeBannerTitle": {
    message: "维护模式已开启",
    description: "Service-detail header banner title while in maintenance",
  },
  "services.maintenanceModeBannerBody": {
    message:
      "访问者将看到维护页面而不是此服务。Pod 仍在运行——在设置中禁用维护模式以恢复正常服务。",
    description: "Service-detail header banner body while in maintenance",
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
  "services.startCommandLabel": {
    message: "启动命令",
    description: "Build & Deploy: native-runtime start-command label",
  },
  "services.dockerCommandLabel": {
    message: "Docker 命令",
    description: "Build & Deploy: Docker CMD override label (Render wording)",
  },
  "services.startCommandHint": {
    message: "成功构建后用于启动此服务的命令。",
    description: "Build & Deploy: native start-command help text",
  },
  "services.dockerCommandHint": {
    message: "覆盖 Dockerfile 中的 CMD。留空则使用镜像的默认命令。",
    description: "Build & Deploy: Docker Command help text",
  },
  "services.startCommandEmpty": {
    message: "使用运行时或镜像的默认命令",
    description: "Build & Deploy: empty start/Docker Command state",
  },
  "services.startCommandConfirmEmpty": {
    message: "默认命令",
    description: "Build & Deploy: empty command phrase in confirmation title",
  },
  "services.startCommandPlaceholder": {
    message: "例如 npm start",
    description: "Build & Deploy: start-command input placeholder",
  },
  "services.startCommandEdit": {
    message: "编辑启动命令",
    description: "Build & Deploy: accessible command edit button label",
  },
  "services.dockerCommandEdit": {
    message: "编辑 Docker 命令",
    description: "Build & Deploy: accessible Docker Command edit button label",
  },
  "services.dockerCommandPlaceholder": {
    message: "例如 bin/server",
    description: "Build & Deploy: Docker Command input placeholder",
  },
  "services.startCommandConfirmTitle": {
    message: "将启动命令更改为 {value}？",
    description: "Build & Deploy: command-change confirmation title",
  },
  "services.dockerCommandConfirmTitle": {
    message: "将 Docker 命令更改为 {value}？",
    description: "Build & Deploy: Docker Command confirmation title",
  },
  "services.startCommandConfirmBody": {
    message: "服务将使用新命令重新部署。现有请求完成后才会替换旧实例。",
    description: "Build & Deploy: command-change confirmation body",
  },
  "services.startCommandSuccess": {
    message: "启动命令已更新。",
    description: "Toast after setStartCommand succeeds",
  },
  "services.startCommandError": {
    message: "无法更新启动命令，请重试。",
    description: "Toast after setStartCommand fails",
  },
  "services.buildCommandLabel": {
    message: "构建命令",
    description:
      "Build & Deploy: build-command field label (static_site settings)",
  },
  "services.buildCommandHint": {
    message:
      "生成静态输出的命令（例如 npm run build）。留空则使用运行时默认值。",
    description: "Build & Deploy: build-command help text",
  },
  "services.buildCommandEmpty": {
    message: "使用运行时默认值",
    description: "Build & Deploy: empty build-command state label",
  },
  "services.buildCommandConfirmEmpty": {
    message: "运行时默认值",
    description:
      "Build & Deploy: empty build-command phrase in confirmation title",
  },
  "services.buildCommandPlaceholder": {
    message: "npm run build",
    description: "Build & Deploy: build-command input placeholder",
  },
  "services.buildCommandEdit": {
    message: "编辑构建命令",
    description: "Build & Deploy: accessible build-command edit button label",
  },
  "services.buildCommandConfirmTitle": {
    message: "将构建命令更改为 {value}？",
    description: "Build & Deploy: build-command change confirmation title",
  },
  "services.buildCommandConfirmBody": {
    message: "服务将使用新的构建命令重新部署。现有请求完成后才会替换旧实例。",
    description: "Build & Deploy: build-command change confirmation body",
  },
  "services.buildCommandSuccess": {
    message: "构建命令已更新。",
    description: "Toast after setBuildCommand succeeds",
  },
  "services.buildCommandError": {
    message: "无法更新构建命令，请重试。",
    description: "Toast after setBuildCommand fails",
  },
  "services.dockerfilePathLabel": {
    message: "Dockerfile 路径",
    description: "Build & Deploy: Dockerfile-path field label",
  },
  "services.dockerfilePathHint": {
    message: "相对于根目录的 Dockerfile 路径。留空则使用 Dockerfile。",
    description: "Build & Deploy: Dockerfile-path help text",
  },
  "services.dockerfilePathEmpty": {
    message: "Dockerfile",
    description: "Build & Deploy: default Dockerfile-path state",
  },
  "services.dockerfilePathConfirmEmpty": {
    message: "默认 Dockerfile",
    description: "Build & Deploy: empty Dockerfile-path confirmation phrase",
  },
  "services.dockerfilePathPlaceholder": {
    message: "例如 docker/Dockerfile.prod",
    description: "Build & Deploy: Dockerfile-path input placeholder",
  },
  "services.dockerfilePathEdit": {
    message: "编辑 Dockerfile 路径",
    description: "Build & Deploy: accessible Dockerfile-path edit label",
  },
  "services.dockerfilePathConfirmTitle": {
    message: "将 Dockerfile 路径更改为 {value}？",
    description: "Build & Deploy: Dockerfile-path confirmation title",
  },
  "services.dockerfilePathConfirmBody": {
    message: "服务将使用所选 Dockerfile 重新构建并部署生成的镜像。",
    description: "Build & Deploy: Dockerfile-path confirmation body",
  },
  "services.dockerfilePathSuccess": {
    message: "Dockerfile 路径已更新。",
    description: "Toast after setDockerfilePath succeeds",
  },
  "services.dockerfilePathError": {
    message: "无法更新 Dockerfile 路径，请重试。",
    description: "Toast after setDockerfilePath fails",
  },
  "services.buildFilterLabel": {
    message: "构建过滤器",
    description: "Build & Deploy: label for the build-filters editor",
  },
  "services.buildFilterHint": {
    message:
      "仅当 git 推送改动了匹配的文件时才触发部署。路径为相对仓库根目录的通配模式（如 src/**、**/*.md）。",
    description: "Build & Deploy: help text for the build-filters editor",
  },
  "services.buildFilterIncludedTitle": {
    message: "包含路径",
    description: "Build & Deploy: title for the included-paths list",
  },
  "services.buildFilterIncludedHint": {
    message: "仅当改动的文件匹配其中之一时才部署。留空表示包含所有路径。",
    description: "Build & Deploy: help text for the included-paths list",
  },
  "services.buildFilterIncludedPlaceholder": {
    message: "例如 src/**",
    description: "Build & Deploy: placeholder for an included-path input",
  },
  "services.buildFilterAddIncluded": {
    message: "添加包含路径",
    description: "Build & Deploy: add-row button for the included-paths list",
  },
  "services.buildFilterRemoveIncluded": {
    message: "移除包含路径",
    description: "Build & Deploy: remove-row label for an included path",
  },
  "services.buildFilterIgnoredTitle": {
    message: "忽略路径",
    description: "Build & Deploy: title for the ignored-paths list",
  },
  "services.buildFilterIgnoredHint": {
    message: "匹配其中之一的改动文件永不触发部署，即使它同时匹配包含路径。",
    description: "Build & Deploy: help text for the ignored-paths list",
  },
  "services.buildFilterIgnoredPlaceholder": {
    message: "例如 docs/**",
    description: "Build & Deploy: placeholder for an ignored-path input",
  },
  "services.buildFilterAddIgnored": {
    message: "添加忽略路径",
    description: "Build & Deploy: add-row button for the ignored-paths list",
  },
  "services.buildFilterRemoveIgnored": {
    message: "移除忽略路径",
    description: "Build & Deploy: remove-row label for an ignored path",
  },
  "services.buildFilterSave": {
    message: "保存构建过滤器",
    description: "Build & Deploy: save button for the build-filters editor",
  },
  "services.buildFilterSuccess": {
    message: "构建过滤器已更新。",
    description: "Toast after setBuildFilter succeeds",
  },
  "services.buildFilterError": {
    message: "无法更新构建过滤器，请重试。",
    description: "Toast after setBuildFilter fails",
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
  "services.cronRunColDuration": {
    message: "持续时间",
    description: "Cron runs table column header (elapsed run time)",
  },
  "services.cronRunColStatus": {
    message: "状态",
    description: "Cron runs table column header (run outcome)",
  },
  "services.cronRunColActions": {
    message: "操作",
    description: "Cron runs table column header (row actions)",
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
  "services.cronRunStatusCanceled": {
    message: "已取消",
    description: "Cron run status badge",
  },
  "services.cronRunCancel": {
    message: "取消",
    description: "Cancel an in-flight cron run",
  },
  "services.cronRunCancelConfirmTitle": {
    message: "取消此次运行？",
    description: "Cron run cancellation confirmation title",
  },
  "services.cronRunCancelConfirmBody": {
    message: "正在运行的任务将被终止，且无法撤销。",
    description: "Cron run cancellation confirmation body",
  },
  "services.cronRunCancelSuccess": {
    message: "定时任务运行已取消。",
    description: "Toast after cron run cancellation is accepted",
  },
  "services.cronRunCancelError": {
    message: "无法取消定时任务运行。",
    description: "Toast after cron run cancellation fails",
  },
  "services.cronRunsLoadMore": {
    message: "加载更多",
    description: "Cron run history pagination button",
  },
  "services.cronRunsLoadingMore": {
    message: "加载中…",
    description: "Cron run history pagination busy label",
  },
  "services.cronRunsLoadError": {
    message: "无法加载定时任务运行记录。",
    description: "Cron run history read error",
  },
  "services.suspendCardTitle": {
    message: "暂停服务",
    description: "Settings tab suspend section title",
  },
  "services.suspendCardDescription": {
    message:
      "暂停服务将关闭它并停止流量服务。服务的 URL 和证书将保留，您可以随时恢复。",
    description: "Settings tab suspend section description",
  },
  "services.resumeCardTitle": {
    message: "恢复服务",
    description:
      "Settings tab resume section title (shown when service is suspended)",
  },
  "services.resumeCardDescription": {
    message: "恢复服务将使其重新上线并开始处理流量。",
    description: "Settings tab resume section description",
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
    message: "此操作无法撤销。确定要删除此 {type} 吗？",
    description:
      "Delete-confirm dialog body ({type} = lowercase Render type words, e.g. 'web service')",
  },
  "services.deleteConfirmPrompt": {
    message: "在下方输入 {phrase} 以确认。",
    description:
      "Body prompt naming Render's exact 'sudo delete <type> <name>' phrase (rendered bold by SudoCommandField)",
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
  "services.createImagePortHint": {
    message: "容器必须监听 $PORT（默认 3000），且无法绑定 1024 以下的端口。",
    description:
      "Create-wizard Existing Image tab hint about bex's routed port and the no-privileged-ports hardening (w9/011)",
  },
  "services.createRegistryCredentialLabel": {
    message: "镜像仓库凭据",
    description: "Create-wizard private-image registry credential label",
  },
  "services.createRegistryCredentialPlaceholder": {
    message: "选择镜像仓库凭据",
    description: "Create-wizard registry credential picker placeholder",
  },
  "services.createRegistryCredentialNone": {
    message: "无凭据 — 公开镜像",
    description: "Create-wizard registry credential picker empty option",
  },
  "services.createRegistryCredentialDescription": {
    message:
      "如果镜像或 Dockerfile 基础镜像是私有的，请选择工作区中已存储的凭据。",
    description: "Create-wizard registry credential picker help text",
  },
  "services.registryCredentialEmpty": {
    message: "没有可用的已存储镜像仓库凭据。",
    description: "Registry credential selector empty state",
  },
  "services.registryCredentialListError": {
    message: "无法加载镜像仓库凭据。",
    description: "Registry credential selector load error",
  },
  "services.registryCredentialManage": {
    message: "管理镜像仓库凭据",
    description: "Link from a service credential selector to account settings",
  },
  "services.registryCredentialSettingsTitle": {
    message: "镜像仓库凭据",
    description: "Service Settings private-image credential card title",
  },
  "services.registryCredentialSettingsDescription": {
    message: "选择用于拉取此私有镜像或 Dockerfile 基础镜像的已存储凭据。",
    description: "Service Settings private-image credential card description",
  },
  "services.registryCredentialSave": {
    message: "保存凭据",
    description: "Service Settings registry credential save action",
  },
  "services.registryCredentialSaved": {
    message: "镜像仓库凭据已更新。",
    description: "Toast after attaching or changing a registry credential",
  },
  "services.registryCredentialCleared": {
    message: "镜像仓库凭据已清除。",
    description: "Toast after clearing a registry credential",
  },
  "services.registryCredentialError": {
    message: "无法更新镜像仓库凭据。",
    description: "Toast after a registry credential update fails",
  },
  "services.headerRegistryCredential": {
    message: "镜像仓库凭据：",
    description: "Service detail header label for a bound registry credential",
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
  "services.createFieldDockerfilePath": {
    message: "Dockerfile 路径",
    description: "Create-wizard Dockerfile-path label for the Docker runtime",
  },
  "services.createFieldDockerfilePathPlaceholder": {
    message: "Dockerfile",
    description: "Create-wizard Dockerfile-path placeholder",
  },
  "services.createFieldDockerfilePathHint": {
    message: "相对于根目录的路径。留空则使用 Dockerfile。",
    description: "Create-wizard Dockerfile-path help text",
  },
  "services.createFieldDockerCommand": {
    message: "Docker 命令",
    description: "Create-wizard Docker CMD override label (Render wording)",
  },
  "services.createFieldDockerCommandPlaceholder": {
    message: "使用 Dockerfile CMD",
    description: "Create-wizard optional Docker Command placeholder",
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
  "services.scaleSuccess": {
    message: "正在缩放至 {count} 个实例…",
    description:
      "Toast acknowledging that scaleService accepted the desired count; convergence is still asynchronous",
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
  "services.createFieldSecretFilesTitle": {
    message: "机密文件",
    description: "Section heading for secret files in the create wizard",
  },
  "services.createFieldSecretFilesHint": {
    message: "从首次部署起以只读方式挂载到 /etc/secrets。",
    description: "Hint for create-time secret files",
  },
  "services.createFieldSecretFilesAdd": {
    message: "添加机密文件",
    description: "Button to add a create-time secret file",
  },
  "services.createFieldSecretFilesRemove": {
    message: "删除机密文件",
    description: "Accessible label for removing a secret file row",
  },
  "services.createFieldSecretFilesName": {
    message: "机密文件名",
    description: "Accessible label for a secret file name",
  },
  "services.createFieldSecretFilesContent": {
    message: "机密文件内容",
    description: "Accessible label for secret file contents",
  },
  "services.createFieldSecretFilesNamePlaceholder": {
    message: "credentials.json",
    description: "Placeholder for a secret file name",
  },
  "services.createFieldSecretFilesContentPlaceholder": {
    message: "粘贴机密内容",
    description: "Placeholder for secret file contents",
  },
  "services.createFieldSecretFilesNameError": {
    message: "只能使用字母、数字、点、短横线和下划线；不能使用 . 或 ..。",
    description: "Invalid secret file name error",
  },
  "services.createFieldEnvironmentTitle": {
    message: "项目和环境",
    description: "Create wizard grouping section title",
  },
  "services.createFieldProject": {
    message: "项目",
    description: "Accessible label for the create project picker",
  },
  "services.createFieldProjectNone": {
    message: "无项目",
    description: "Unassigned option in the project picker",
  },
  "services.createFieldEnvironment": {
    message: "环境",
    description: "Accessible label for the create environment picker",
  },
  "services.createFieldEnvironmentNone": {
    message: "无环境",
    description: "Unassigned option in the environment picker",
  },
  "services.createFieldEnvironmentHint": {
    message: "选择环境也会将服务加入其所属项目。",
    description: "Hint for create-time environment assignment",
  },
  "services.navEvents": {
    message: "事件",
    description: "Service-detail nav item (events tab)",
  },
  "services.navDeploys": {
    message: "部署",
    description:
      "Service-detail nav item (dedicated deploy-history tab, w9/002)",
  },
  "services.navShell": {
    message: "Shell",
    description: "Service-detail nav item (running-instance SSH page)",
  },
  "services.eventsTitle": {
    message: "活动",
    description: "Events tab card title",
  },
  "services.eventsDescription": {
    message: "最近的部署和服务变更。",
    description: "Events tab card description",
  },
  "services.eventsCount": {
    message: "最近 {count} 条事件",
    description: "Accessible label for the number of visible service events",
  },
  "services.eventsEmptyTitle": {
    message: "暂无活动",
    description: "Events tab empty-state title",
  },
  "services.eventsEmpty": {
    message: "部署和服务变更将显示在这里。",
    description: "Events tab empty-state description",
  },
  "services.eventsErrorTitle": {
    message: "活动暂不可用",
    description: "Events tab query-error title",
  },
  "services.eventsErrorDescription": {
    message: "无法加载最近活动，请稍后重试。",
    description: "Events tab query-error description",
  },
  "services.eventsRetry": {
    message: "重试",
    description: "Events tab query-error retry button",
  },
  "services.eventsActor": {
    message: "由 {actor} 操作",
    description: "Actor attribution shown on a service event",
  },
  "services.eventsDeployReference": {
    message: "部署 {id}",
    description: "Deploy identifier shown on a deploy activity row",
  },
  "services.eventsTriggerRollback": {
    message: "回滚",
    description: "Deploy event trigger: rollback",
  },
  "services.eventsTriggerFirstBuild": {
    message: "首次构建",
    description: "Deploy event trigger: initial build",
  },
  "services.eventsTriggerManual": {
    message: "手动部署",
    description: "Deploy event trigger: manual",
  },
  "services.eventsTriggerEnvUpdated": {
    message: "环境已更新",
    description: "Deploy event trigger: environment update",
  },
  "services.eventsTriggerClearCache": {
    message: "已清除缓存",
    description: "Deploy event trigger: build-cache clear",
  },
  "services.eventsTriggerDeployedByRender": {
    message: "平台部署",
    description: "Deploy event trigger: platform initiated",
  },
  "services.eventsTypeDeployStarted": {
    message: "部署已开始",
    description: "Service activity type: deploy started",
  },
  "services.eventsTypeDeployFinished": {
    message: "部署已完成",
    description: "Service activity type: deploy finished",
  },
  "services.eventsTypeSuspended": {
    message: "服务已暂停",
    description: "Service activity type: service suspended",
  },
  "services.eventsTypeResumed": {
    message: "服务已恢复",
    description: "Service activity type: service resumed",
  },
  "services.eventsTypeRestarted": {
    message: "服务已重启",
    description: "Service activity type: service restarted",
  },
  "services.eventsTypePlanChanged": {
    message: "实例类型已更改",
    description: "Service activity type: plan changed",
  },
  "services.eventsTypeInstanceCountChanged": {
    message: "实例数量已更改",
    description: "Service activity type: manual scale",
  },
  "services.eventsTypeAutoscalingChanged": {
    message: "自动伸缩已更新",
    description: "Service activity type: autoscaling configuration changed",
  },
  "services.eventsTypeCronRunStarted": {
    message: "定时任务已开始",
    description: "Service activity type: cron run started",
  },
  "services.eventsTypeCronRunFinished": {
    message: "定时任务已完成",
    description: "Service activity type: cron run finished",
  },
  "services.eventsTypeEnvVarsChanged": {
    message: "环境变量已更改",
    description: "Service activity type: environment variables changed",
  },
  "services.eventsTypeEnvironmentChanged": {
    message: "环境配置已更改",
    description:
      "Service activity type: environment variables and secret files changed",
  },
  "services.eventsTypeEnvGroupLinked": {
    message: "已关联环境组",
    description: "Service activity type: environment group linked",
  },
  "services.eventsTypeEnvGroupUnlinked": {
    message: "已取消关联环境组",
    description: "Service activity type: environment group unlinked",
  },
  "services.eventsTypeAutoDeployChanged": {
    message: "自动部署已更新",
    description: "Service activity type: auto-deploy setting changed",
  },
  "services.eventsTypeIdleTimeoutChanged": {
    message: "空闲超时已更新",
    description: "Service activity type: idle timeout changed",
  },
  "services.eventsTypeDisplayNameChanged": {
    message: "显示名称已更改",
    description: "Service activity type: display name changed",
  },
  "services.eventsTypeCustomDomainAdded": {
    message: "已添加自定义域名",
    description: "Service activity type: custom domain added",
  },
  "services.eventsTypeCustomDomainRemoved": {
    message: "已移除自定义域名",
    description: "Service activity type: custom domain removed",
  },
  "services.eventsTypeNotificationsChanged": {
    message: "失败通知已更新",
    description: "Service activity type: failure notification setting changed",
  },
  "services.eventsTypeSubdomainPolicyChanged": {
    message: "平台子域名已更新",
    description: "Service activity type: platform subdomain policy changed",
  },
  "services.eventsTypeStaticSiteChanged": {
    message: "静态站点设置已更改",
    description: "Service activity type: static-site configuration changed",
  },
  "services.eventsTypeBuildSettingsChanged": {
    message: "构建和部署设置已更改",
    description: "Service activity type: build or deploy configuration changed",
  },
  "services.eventsTypeServiceChanged": {
    message: "服务设置已更改",
    description: "Fallback service activity type",
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
  "services.networkingTitle": {
    message: "网络",
    description: "Settings Networking card title (w7/m32)",
  },
  "services.networkingDescription": {
    message: "将入站 HTTP 流量限制为这些源 CIDR。",
    description: "Settings Networking card description (w7/m32)",
  },
  "services.networkingHint": {
    message:
      "输入 CIDR（例如 203.0.113.0/24）并点击添加。列表为空时向所有源 IP 开放。",
    description: "Hint below the CIDR list in the Networking card (w7/m32)",
  },
  "services.networkingOpen": {
    message: "向所有源 IP 开放",
    description:
      "Placeholder shown when the allow list is empty (Render default, w7/m32)",
  },
  "services.networkingAdd": {
    message: "添加",
    description: "Button to add a CIDR to the draft list (w7/m32)",
  },
  "services.networkingEntryDescription": {
    message: "描述（可选）",
    description: "Placeholder for a service allowlist entry description",
  },
  "services.networkingInvalid": {
    message: "请输入有效的 IPv4 或 IPv6 CIDR。",
    description: "Validation error for an invalid service allowlist CIDR",
  },
  "services.networkingSave": {
    message: "保存",
    description: "Button to persist the CIDR list (w7/m32)",
  },
  "services.networkingRemove": {
    message: "移除 {cidr}",
    description:
      "Accessible label on the trash icon next to a CIDR tag (w7/m32)",
  },
  "services.networkingMoveUp": {
    message: "上移 {cidr}",
    description: "Accessible label to move an allowlist entry earlier",
  },
  "services.networkingMoveDown": {
    message: "下移 {cidr}",
    description: "Accessible label to move an allowlist entry later",
  },
  "services.networkingSaved": {
    message: "IP 允许列表已更新",
    description: "Toast on successful setServiceIpAllowList mutation (w7/m32)",
  },
  "services.networkingError": {
    message: "更新 IP 允许列表失败：{error}",
    description: "Toast on failed setServiceIpAllowList mutation (w7/m32)",
  },
  "services.environmentPageTitle": {
    message: "环境",
    description: "Service Environment page heading",
  },
  "services.environmentPageDescription": {
    message: "管理此服务的环境变量、机密文件和已关联的环境组。",
    description: "Service Environment page introduction",
  },
  "services.environmentEdit": {
    message: "编辑",
    description: "Enter the coherent service environment draft",
  },
  "services.environmentMaskedValue": {
    message: "已遮蔽的机密值",
    description: "Accessible label for an unrevealed secret",
  },
  "services.environmentUnchangedMasked": {
    message: "未更改（已遮蔽）",
    description: "Placeholder for an opaque unchanged draft value",
  },
  "services.environmentDuplicateKey": {
    message: "每个变量键必须唯一。",
    description: "Draft validation for a duplicate environment key",
  },
  "services.environmentValueRequired": {
    message: "请为此新变量输入值。",
    description: "Draft validation for a new variable without a value",
  },
  "services.environmentStagedDelete": {
    message: "将被移除",
    description: "Badge on a staged environment deletion",
  },
  "services.environmentUndo": {
    message: "撤销",
    description: "Undo a staged environment deletion",
  },
  "services.environmentUnsavedTitle": {
    message: "未保存的环境更改",
    description: "Combined draft save bar heading",
  },
  "services.environmentUnsavedSummary": {
    message: "{variables} 个变量操作 · {files} 个文件操作",
    description: "Combined draft operation count",
  },
  "services.environmentSaveOptions": {
    message: "环境保存选项",
    description: "Accessible label for the split save menu",
  },
  "services.environmentSaveOnly": {
    message: "仅保存",
    description: "Persist and project without rolling the service",
  },
  "services.environmentSaveDeploy": {
    message: "保存并部署",
    description: "Persist and roll the current image once",
  },
  "services.environmentSaveRebuild": {
    message: "保存、重新构建并部署",
    description: "Persist then start one source build and deploy",
  },
  "services.environmentSaveOnlySuccess": {
    message: "环境已保存，未部署",
    description: "Toast after save-only succeeds",
  },
  "services.environmentSaveDeploySuccess": {
    message: "环境已保存，部署已开始",
    description: "Toast after save-and-deploy succeeds",
  },
  "services.environmentSaveError": {
    message: "无法保存环境。草稿仍保留。",
    description: "Batch environment save failure",
  },
  "services.environmentSavedDeployFailedTitle": {
    message: "配置已保存，但部署未启动",
    description: "Partial rebuild failure heading",
  },
  "services.environmentSavedDeployFailedBody": {
    message: "环境更改已存储。准备好后只需重试部署。",
    description: "Partial rebuild failure recovery explanation",
  },
  "services.environmentRetryDeploy": {
    message: "重试部署",
    description: "Retry only the failed deploy phase",
  },
  "services.environmentDiscardTitle": {
    message: "放弃环境更改？",
    description: "Dirty navigation guard title",
  },
  "services.environmentDiscardBody": {
    message: "这些更改尚未保存，离开后将丢失。",
    description: "Dirty navigation guard explanation",
  },
  "services.environmentKeepEditing": {
    message: "继续编辑",
    description: "Cancel dirty navigation",
  },
  "services.environmentDiscard": {
    message: "放弃并离开",
    description: "Confirm dirty navigation",
  },
  "services.envReveal": {
    message: "显示",
    description: "Reveal one masked environment value",
  },
  "services.envHide": {
    message: "隐藏",
    description: "Mask one revealed environment value",
  },
  "services.envCopy": {
    message: "复制环境变量",
    description: "Copy the complete dotenv export",
  },
  "services.envDownload": {
    message: "下载 .env",
    description: "Download the complete dotenv export",
  },
  "services.envCopySuccess": {
    message: "环境已复制",
    description: "Toast after the complete dotenv export is copied",
  },
  "services.envAddVariable": {
    message: "添加变量",
    description: "Add one blank draft variable",
  },
  "services.envAddGenerated": {
    message: "生成机密",
    description: "Add one previewable generated secret",
  },
  "services.envImport": {
    message: "从 .env 导入",
    description: "Open the dotenv import dialog",
  },
  "services.envImportTitle": {
    message: "导入环境变量",
    description: "Dotenv import dialog title",
  },
  "services.envImportDescription": {
    message:
      "粘贴 dotenv 赋值或选择文本文件。导入的值会保留在草稿中，直到保存。",
    description: "Dotenv import dialog explanation",
  },
  "services.envImportTextLabel": {
    message: "Dotenv 内容",
    description: "Accessible label for the dotenv import textarea",
  },
  "services.envImportPlaceholder": {
    message: "API_URL=https://example.com\nSECRET=replace-me",
    description: "Dotenv import textarea placeholder",
  },
  "services.envImportChooseFile": {
    message: "选择文件",
    description: "Dotenv file picker button",
  },
  "services.envImportAdd": {
    message: "添加变量",
    description: "Stage parsed dotenv variables",
  },
  "services.envImportLineError": {
    message: "第 {line} 行不是有效的 dotenv 赋值。",
    description: "Line-numbered dotenv parse error",
  },
  "services.envImportFileError": {
    message: "请选择不超过 1 MiB 的可读文本文件。",
    description: "Dotenv file read or size error",
  },
  "services.secretFileUpload": {
    message: "上传文件",
    description: "Stage multiple local text files",
  },
  "services.secretFileUploadError": {
    message:
      "部分文件已跳过。请使用唯一且安全的名称、文本内容，并确保文件不超过 1 MiB。",
    description: "Secret-file upload validation summary",
  },
  "services.secretFileDuplicateName": {
    message: "每个机密文件名必须唯一。",
    description: "Draft validation for a duplicate file name",
  },
  "services.secretFileContentRequired": {
    message: "请为此新文件添加内容。",
    description: "Draft validation for missing new-file content",
  },
  "services.secretFileViewContent": {
    message: "查看内容",
    description: "Reveal or inspect secret-file contents in a dialog",
  },
  "services.secretFileEditContent": {
    message: "编辑内容",
    description: "Edit staged secret-file contents in a dialog",
  },
  "services.secretFileContentDialogTitle": {
    message: "{name} 的内容",
    description: "Secret-file content dialog title",
  },
  "services.secretFileContentDialogDescription": {
    message: "下次部署后，此文件将以只读方式挂载到 /etc/secrets。",
    description: "Secret-file content dialog explanation",
  },
  "services.secretFileLoadingContent": {
    message: "正在加载内容…",
    description: "Secret-file content reveal progress",
  },
  "services.secretFileContentDone": {
    message: "完成",
    description: "Close and stage secret-file content edits",
  },
  "services.secretFileUntitled": {
    message: "未命名文件",
    description: "Fallback title for a new unnamed secret file",
  },
  "services.secretFileDelete": {
    message: "删除机密文件",
    description: "Accessible label for a staged secret-file deletion",
  },
  "services.envGroupsLinkedTitle": {
    message: "已关联的环境组",
    description: "Service-side environment group card title",
  },
  "services.envGroupsLinkedCount": {
    message: "已关联（{count}）",
    description: "Count of groups linked to this service",
  },
  "services.envGroupsAvailableCount": {
    message: "可关联（{count}）",
    description: "Count of groups available to link",
  },
  "services.envGroupsNoneLinked": {
    message: "此服务尚未关联环境组。",
    description: "Service group empty linked state",
  },
  "services.envGroupsNoneAvailable": {
    message: "工作区中的所有环境组都已关联。",
    description: "Service group empty available state",
  },
  "services.envGroupsNoneAvailableCreate": {
    message:
      "工作区中还没有环境组。创建一个包含变量和文件、并预先选择此服务的环境组。",
    description: "Service group create-first empty state",
  },
};

export default zhServices;
