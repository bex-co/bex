import type { TranslationEntry } from "@/i18n";

const zh: Record<string, TranslationEntry> = {
  "sandboxGroupPages.navSandboxes": {
    message: "沙箱",
    description: "Sidebar entry and heading: a sandbox group's sandboxes",
  },
  "sandboxGroupPages.navSnapshots": {
    message: "快照",
    description: "Sidebar entry and heading: a sandbox group's snapshots",
  },
  "sandboxGroupPages.navImages": {
    message: "自定义镜像",
    description:
      "Sidebar entry and heading: the images imported into a sandbox group",
  },
  "sandboxGroupPages.navSettings": {
    message: "设置",
    description: "Sidebar entry and heading: a sandbox group's settings",
  },
  "sandboxGroupPages.statusCreating": {
    message: "创建中",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusRunning": {
    message: "运行中",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusResuming": {
    message: "恢复中",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusSuspending": {
    message: "正在暂停",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusSuspended": {
    message: "已暂停",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusErrored": {
    message: "出错",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusTerminated": {
    message: "已终止",
    description: "Sandbox status",
  },
  "sandboxGroupPages.rangeLabel": {
    message: "时间范围",
    description: "Accessible name of the time range picker",
  },
  "sandboxGroupPages.rangeHour": {
    message: "最近 1 小时",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeDay": {
    message: "最近 24 小时",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeWeek": {
    message: "最近 7 天",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeMonth": {
    message: "最近 30 天",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeAll": {
    message: "全部时间",
    description: "Time range option",
  },
  "sandboxGroupPages.searchSandboxes": {
    message: "按名称、沙箱 ID 或状态搜索",
    description: "Placeholder of the Sandboxes search",
  },
  "sandboxGroupPages.searchSnapshots": {
    message: "搜索快照",
    description: "Placeholder of the Snapshots search",
  },
  "sandboxGroupPages.searchImages": {
    message: "搜索镜像",
    description: "Placeholder of the Custom images search",
  },
  "sandboxGroupPages.pauseLive": {
    message: "暂停实时更新",
    description: "Button that stops the list refreshing",
  },
  "sandboxGroupPages.resumeLive": {
    message: "恢复实时更新",
    description: "Button that restarts the list refreshing",
  },
  "sandboxGroupPages.showHideColumns": {
    message: "显示或隐藏列",
    description: "Accessible name of the columns menu",
  },
  "sandboxGroupPages.summaryTotal": {
    message: "总计",
    description: "Summary: runs that ended in the time range",
  },
  "sandboxGroupPages.summaryErrored": {
    message: "出错",
    description: "Summary: share of runs that ended in an error",
  },
  "sandboxGroupPages.summaryErroredTip": {
    message: "时间范围内以出错结束的沙箱所占的比例。",
    description: "Tooltip of the Errored summary",
  },
  "sandboxGroupPages.summaryRunning": {
    message: "运行中",
    description: "Summary: sandboxes active now",
  },
  "sandboxGroupPages.summaryRunningTip": {
    message:
      "当前正在创建、运行、恢复或暂停的沙箱，与时间范围无关。它们计入并发上限。",
    description: "Tooltip of the Running summary",
  },
  "sandboxGroupPages.loadRunsError": {
    message: "无法加载沙箱运行记录",
    description: "The run chart failed to load",
  },
  "sandboxGroupPages.runsChartLabel": {
    message: "已结束的沙箱运行：{succeeded} 次成功，{errored} 次出错",
    description: "Accessible name of the run chart",
  },
  "sandboxGroupPages.runsSucceeded": {
    message: "成功",
    description: "Chart legend: runs that ended without an error",
  },
  "sandboxGroupPages.runsErrored": {
    message: "出错",
    description: "Chart legend: runs that ended in an error",
  },
  "sandboxGroupPages.selectedCount_other": {
    message: "已选择 {count} 个沙箱",
    description: "How many rows are selected",
  },
  "sandboxGroupPages.terminateSelected": {
    message: "终止",
    description: "Button: terminate the selected sandboxes",
  },
  "sandboxGroupPages.terminateTitle_other": {
    message: "终止 {count} 个沙箱？",
    description: "Terminate confirmation title",
  },
  "sandboxGroupPages.terminateDescription_other": {
    message: "这 {count} 个沙箱将停止，其中未保存到快照的内容都会丢失。",
    description: "Terminate confirmation body",
  },
  "sandboxGroupPages.terminateConfirm": {
    message: "终止",
    description: "Terminate confirmation button",
  },
  "sandboxGroupPages.terminateFailed_other": {
    message: "无法终止 {count} 个沙箱，请重试。",
    description: "Toast: terminates failed",
  },
  "sandboxGroupPages.terminateStarted_other": {
    message: "正在终止 {count} 个沙箱",
    description: "Toast: terminates were accepted",
  },
  "sandboxGroupPages.terminateSandbox": {
    message: "终止沙箱",
    description: "Drawer menu item",
  },
  "sandboxGroupPages.sandboxActions": {
    message: "沙箱操作",
    description: "Accessible name of the drawer's menu",
  },
  "sandboxGroupPages.loadSandboxError": {
    message: "无法加载该沙箱",
    description: "A sandbox's drawer failed to load",
  },
  "sandboxGroupPages.loadSandboxesError": {
    message: "无法加载沙箱",
    description: "The list failed to load",
  },
  "sandboxGroupPages.noMatchingSandboxes": {
    message: "没有匹配的沙箱",
    description: "Empty state with filters",
  },
  "sandboxGroupPages.noMatchingSandboxesHelp": {
    message: "换个搜索词，或扩大时间范围。",
    description: "Empty state with filters",
  },
  "sandboxGroupPages.clearFilters": {
    message: "清除筛选",
    description: "Button: reset the search and time range",
  },
  "sandboxGroupPages.firstSandboxTitle": {
    message: "创建你的第一个沙箱",
    description: "Empty state title of the Sandboxes page",
  },
  "sandboxGroupPages.firstSandboxDescription": {
    message:
      "用 CLI 启动一个沙箱，或者配置你的代码，通过 TypeScript 和 Python SDK 以编程方式创建沙箱。",
    description: "Empty state body of the Sandboxes page",
  },
  "sandboxGroupPages.starterCli": {
    message: "CLI",
    description: "Starter tab: the command line",
  },
  "sandboxGroupPages.starterSdk": {
    message: "SDK",
    description: "Starter tab: the SDKs",
  },
  "sandboxGroupPages.starterInstallCli": {
    message: "安装或更新到最新版 CLI",
    description: "Starter step",
  },
  "sandboxGroupPages.starterHomebrew": {
    message: "Homebrew",
    description: "Install method",
  },
  "sandboxGroupPages.starterDirect": {
    message: "直接下载",
    description: "Install method",
  },
  "sandboxGroupPages.starterLogin": {
    message: "登录并使用此工作区",
    description: "Starter step",
  },
  "sandboxGroupPages.starterCreateSandbox": {
    message: "使用默认设置创建沙箱",
    description: "Starter step",
  },
  "sandboxGroupPages.starterApiKey": {
    message: "设置你的 API 密钥",
    description: "Starter step",
  },
  "sandboxGroupPages.starterApiKeyHelp": {
    message:
      "在你将要运行代码的终端中执行此命令。SDK 会从 RENDER_API_KEY 环境变量读取你的密钥。请将占位值替换为 bex API 密钥。",
    description: "Starter step help",
  },
  "sandboxGroupPages.starterInstallSdk": {
    message: "创建项目并安装 SDK",
    description: "Starter step",
  },
  "sandboxGroupPages.starterPythonNeeds": {
    message:
      "需要 Python 3.10 或更高版本。-U 会升级旧版 SDK；1.4.0 及更早版本不支持沙箱。",
    description: "Starter step help",
  },
  "sandboxGroupPages.starterNodeNeeds": {
    message: "需要 Node.js 18 或更高版本。@latest 也会升级已安装的旧版 SDK。",
    description: "Starter step help",
  },
  "sandboxGroupPages.starterFirstSandboxFile": {
    message: "初始化你的第一个沙箱",
    description: "Starter step",
  },
  "sandboxGroupPages.starterRun": {
    message: "在本地运行你的代码",
    description: "Starter step",
  },
  "sandboxGroupPages.starterTypeScript": {
    message: "TypeScript",
    description: "SDK language",
  },
  "sandboxGroupPages.starterPython": {
    message: "Python",
    description: "SDK language",
  },
  "sandboxGroupPages.colSandbox": { message: "沙箱", description: "Column" },
  "sandboxGroupPages.colStatus": { message: "状态", description: "Column" },
  "sandboxGroupPages.colCreated": {
    message: "创建时间",
    description: "Column",
  },
  "sandboxGroupPages.colDuration": {
    message: "持续时间",
    description: "Column",
  },
  "sandboxGroupPages.colPlan": { message: "套餐", description: "Column" },
  "sandboxGroupPages.colSource": {
    message: "启动来源",
    description: "Column: the image or snapshot a sandbox started from",
  },
  "sandboxGroupPages.selectAll": {
    message: "选择此页上的所有沙箱",
    description: "Accessible name of the select-all checkbox",
  },
  "sandboxGroupPages.selectSandbox": {
    message: "选择 {name}",
    description: "Accessible name of a row's checkbox",
  },
  "sandboxGroupPages.baseImage": {
    message: "基础镜像",
    description: "A sandbox started from the default image",
  },
  "sandboxGroupPages.importedImage": {
    message: "导入的镜像",
    description: "A sandbox started from an imported image",
  },
  "sandboxGroupPages.previousPage": {
    message: "上一页",
    description: "Paging button",
  },
  "sandboxGroupPages.nextPage": {
    message: "下一页",
    description: "Paging button",
  },
  "sandboxGroupPages.switchedToRunning": {
    message: "你选中的沙箱已停止，已切换到一个正在运行的沙箱。",
    description: "Toast when the open sandbox stops",
  },
  "sandboxGroupPages.copied": { message: "已复制", description: "Copy toast" },
  "sandboxGroupPages.copyError": {
    message: "复制失败",
    description: "Copy failed toast",
  },
  "sandboxGroupPages.copySandboxId": {
    message: "复制沙箱 ID",
    description: "Copy button",
  },
  "sandboxGroupPages.copySnapshotId": {
    message: "复制快照 ID",
    description: "Copy button",
  },
  "sandboxGroupPages.tabEvents": { message: "事件", description: "Drawer tab" },
  "sandboxGroupPages.tabSnapshots": {
    message: "快照",
    description: "Drawer tab",
  },
  "sandboxGroupPages.tabDetails": {
    message: "详情",
    description: "Drawer tab and section",
  },
  "sandboxGroupPages.tabVersions": {
    message: "版本",
    description: "Image drawer tab",
  },
  "sandboxGroupPages.sandboxGoneTitle": {
    message: "未找到沙箱",
    description: "A sandbox that does not exist here",
  },
  "sandboxGroupPages.sandboxGoneDescription": {
    message: "{id} 不是你在此组中可以查看的沙箱。",
    description: "A sandbox that does not exist here",
  },
  "sandboxGroupPages.loadingEvents": {
    message: "正在加载事件",
    description: "Events tab loading",
  },
  "sandboxGroupPages.loadEventsError": {
    message: "无法加载事件，请稍后再试。",
    description: "Events tab error",
  },
  "sandboxGroupPages.noEvents": {
    message: "暂无事件。",
    description: "Events tab empty",
  },
  "sandboxGroupPages.loadEarlierEvents": {
    message: "加载更早的事件",
    description: "Button: the next page of events",
  },
  "sandboxGroupPages.loadingEarlierEvents": {
    message: "正在加载更早的事件",
    description: "Loading the next page of events",
  },
  "sandboxGroupPages.eventCreated": {
    message: "已创建",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventReady": {
    message: "就绪",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventSuspending": {
    message: "正在暂停",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventSuspended": {
    message: "已暂停",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventResuming": {
    message: "恢复中",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventTerminated": {
    message: "已终止",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventErrored": {
    message: "出错",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventCommand": {
    message: "命令（{operation}）",
    description: "An execution event that ran a command",
  },
  "sandboxGroupPages.eventFileTransfer": {
    message: "文件传输（{operation}）",
    description: "An execution event that moved a file",
  },
  "sandboxGroupPages.exitCode": {
    message: "退出码 {code}",
    description: "A command's exit code",
  },
  "sandboxGroupPages.eventSnapshot": {
    message: "快照",
    description: "A snapshot event",
  },
  "sandboxGroupPages.viewSnapshot": {
    message: "查看快照",
    description: "Link to a snapshot's drawer",
  },
  "sandboxGroupPages.triggerSdk": {
    message: "SDK",
    description: "Trigger: the SDK",
  },
  "sandboxGroupPages.triggerCli": {
    message: "CLI",
    description: "Trigger: the CLI",
  },
  "sandboxGroupPages.triggerApi": {
    message: "API",
    description: "Trigger: the API",
  },
  "sandboxGroupPages.triggerDashboard": {
    message: "控制台",
    description: "Trigger: the dashboard",
  },
  "sandboxGroupPages.triggerIdleTimeout": {
    message: "空闲超时",
    description: "Trigger: the idle timeout",
  },
  "sandboxGroupPages.triggerSystem": {
    message: "bex",
    description: "Trigger: the platform itself",
  },
  "sandboxGroupPages.triggerConnection": {
    message: "新命令或文件传输",
    description: "Trigger: a resume a command started",
  },
  "sandboxGroupPages.loading": {
    message: "正在加载",
    description: "Loading text",
  },
  "sandboxGroupPages.loadSnapshotsError": {
    message: "无法加载快照",
    description: "Snapshots failed to load",
  },
  "sandboxGroupPages.noSnapshotsRunning": {
    message: "此沙箱还没有快照。可以用 CLI 或 SDK 捕获一个。",
    description: "Snapshots tab of a running sandbox",
  },
  "sandboxGroupPages.noSnapshotsEnded": {
    message: "此沙箱没有创建过快照。",
    description: "Snapshots tab of an ended sandbox",
  },
  "sandboxGroupPages.detailSandboxId": {
    message: "沙箱 ID",
    description: "Detail label",
  },
  "sandboxGroupPages.detailGroup": {
    message: "组",
    description: "Detail label",
  },
  "sandboxGroupPages.detailPlan": {
    message: "套餐",
    description: "Detail label",
  },
  "sandboxGroupPages.planSize": {
    message: "{plan}（{cpus} 核 CPU，{memory} GB 内存）",
    description: "A plan with its size",
  },
  "sandboxGroupPages.detailCreated": {
    message: "创建时间",
    description: "Detail label",
  },
  "sandboxGroupPages.detailEnded": {
    message: "结束时间",
    description: "Detail label",
  },
  "sandboxGroupPages.detailDuration": {
    message: "持续时间",
    description: "Detail label",
  },
  "sandboxGroupPages.detailTimeout": {
    message: "超时",
    description: "Detail label",
  },
  "sandboxGroupPages.detailIdleTimeout": {
    message: "空闲超时",
    description: "Detail label",
  },
  "sandboxGroupPages.off": {
    message: "关闭",
    description: "A timeout that is off",
  },
  "sandboxGroupPages.detailNetwork": {
    message: "网络",
    description: "Detail label",
  },
  "sandboxGroupPages.networkAllowAll": {
    message: "全部允许",
    description: "Network policy",
  },
  "sandboxGroupPages.networkDenyAll": {
    message: "全部拒绝",
    description: "Network policy",
  },
  "sandboxGroupPages.networkAllowList": {
    message: "允许列表",
    description: "Network policy",
  },
  "sandboxGroupPages.detailRegion": {
    message: "区域",
    description: "Detail label",
  },
  "sandboxGroupPages.detailSource": {
    message: "启动来源",
    description: "Detail label",
  },
  "sandboxGroupPages.goToSnapshot": {
    message: "前往快照",
    description: "Link to the snapshot a sandbox started from",
  },
  "sandboxGroupPages.goToImage": {
    message: "前往镜像",
    description: "Link to the image a sandbox started from",
  },
  "sandboxGroupPages.detailTrigger": {
    message: "触发方式",
    description: "Detail label: what started the sandbox",
  },
  "sandboxGroupPages.detailUser": {
    message: "用户",
    description: "Detail label: who started the sandbox",
  },
  "sandboxGroupPages.detailError": {
    message: "错误",
    description: "Detail label",
  },
  "sandboxGroupPages.errorUnknown": {
    message:
      "bex 无法确定此沙箱出错的原因。请重试，或带上沙箱 ID 联系支持团队。",
    description: "An errored sandbox with no known reason",
  },
  "sandboxGroupPages.snapshotCapturing": {
    message: "捕获中",
    description: "Snapshot status",
  },
  "sandboxGroupPages.snapshotAvailable": {
    message: "可用",
    description: "Snapshot status",
  },
  "sandboxGroupPages.snapshotFailed": {
    message: "失败",
    description: "Snapshot status",
  },
  "sandboxGroupPages.snapshotFailureInternal": {
    message: "内部错误导致捕获中止。",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailureTimeout": {
    message: "此次捕获超时。",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailureSourceTerminated": {
    message: "源沙箱在捕获完成前已被终止。",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailureSourceErrored": {
    message: "源沙箱在捕获完成前出错。",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailedHelp": {
    message: "请创建新快照重试。新沙箱无法从此快照启动。",
    description: "After a snapshot failure",
  },
  "sandboxGroupPages.noMatchingSnapshots": {
    message: "没有匹配的快照",
    description: "Empty state with filters",
  },
  "sandboxGroupPages.firstSnapshotTitle": {
    message: "创建你的第一个快照",
    description: "Empty state title of the Snapshots page",
  },
  "sandboxGroupPages.firstSnapshotDescription": {
    message:
      "捕获快照以保存沙箱的文件。之后可以从同一状态启动新沙箱，无需重新完成所有设置。",
    description: "Empty state body of the Snapshots page",
  },
  "sandboxGroupPages.firstSnapshotNeedsRunning": {
    message: "你需要一个正在运行的沙箱才能创建快照。",
    description: "Empty state note",
  },
  "sandboxGroupPages.starterSnapshotSandbox": {
    message: "为正在运行的沙箱创建快照",
    description: "Starter step",
  },
  "sandboxGroupPages.starterStartFromSnapshot": {
    message: "从快照创建沙箱",
    description: "Starter step",
  },
  "sandboxGroupPages.starterSnapshotCode": {
    message: "配置你的代码",
    description: "Starter step",
  },
  "sandboxGroupPages.colSnapshot": { message: "快照", description: "Column" },
  "sandboxGroupPages.colSourceSandbox": {
    message: "源沙箱",
    description: "Column and detail label",
  },
  "sandboxGroupPages.colSize": {
    message: "大小",
    description: "Column and detail label",
  },
  "sandboxGroupPages.colCaptured": {
    message: "捕获时间",
    description: "Column and detail label",
  },
  "sandboxGroupPages.colExpires": {
    message: "到期时间",
    description: "Column and detail label",
  },
  "sandboxGroupPages.never": {
    message: "永不",
    description: "A snapshot that never expires",
  },
  "sandboxGroupPages.snapshotGoneTitle": {
    message: "未找到快照",
    description: "A snapshot that does not exist here",
  },
  "sandboxGroupPages.snapshotGoneDescription": {
    message: "{id} 不是你在此工作区中可以查看的快照。它可能已被删除或已过期。",
    description: "A snapshot that does not exist here",
  },
  "sandboxGroupPages.detailType": {
    message: "类型",
    description: "Detail label: snapshot kind",
  },
  "sandboxGroupPages.kindFilesystem": {
    message: "文件系统",
    description: "Snapshot kind",
  },
  "sandboxGroupPages.kindRuntime": {
    message: "运行时",
    description: "Snapshot kind",
  },
  "sandboxGroupPages.detailResourceConstraint": {
    message: "资源约束",
    description: "Detail label: the plan a snapshot needs",
  },
  "sandboxGroupPages.detailLifecycle": {
    message: "生命周期",
    description: "Detail label",
  },
  "sandboxGroupPages.lifecycleKept": {
    message: "保留至删除",
    description: "A snapshot with no time to live",
  },
  "sandboxGroupPages.lifecycleAfterUse": {
    message: "最后一次使用 {days} 天后过期",
    description: "A snapshot's time to live",
  },
  "sandboxGroupPages.noSandboxesFromSnapshot": {
    message: "还没有从此快照启动的沙箱。",
    description: "Snapshot drawer's sandboxes",
  },
  "sandboxGroupPages.restoreTitle": {
    message: "从此快照启动沙箱",
    description: "Snapshot drawer starter code",
  },
  "sandboxGroupPages.deleteSnapshot": {
    message: "删除快照",
    description: "Button",
  },
  "sandboxGroupPages.deleteSnapshotTitle": {
    message: "删除快照？",
    description: "Confirmation title",
  },
  "sandboxGroupPages.deleteSnapshotDescription": {
    message:
      "这将永久删除 {name}。新沙箱将无法从它启动，已从它启动的沙箱会继续运行。",
    description: "Confirmation body",
  },
  "sandboxGroupPages.deleteSnapshotError": {
    message: "无法删除快照，请重试。",
    description: "Toast",
  },
  "sandboxGroupPages.snapshotDeleted": {
    message: "已删除快照 {name}",
    description: "Toast",
  },
  "sandboxGroupPages.imageQueued": {
    message: "排队中",
    description: "Image status",
  },
  "sandboxGroupPages.imageImporting": {
    message: "导入中",
    description: "Image status",
  },
  "sandboxGroupPages.imageAvailable": {
    message: "可用",
    description: "Image status",
  },
  "sandboxGroupPages.imageUnavailable": {
    message: "不可用",
    description: "Image status: no version can start a sandbox",
  },
  "sandboxGroupPages.imageFailed": {
    message: "失败",
    description: "Image version status",
  },
  "sandboxGroupPages.imageLatest": {
    message: "最新",
    description: "The newest available version of an image",
  },
  "sandboxGroupPages.imageImportingHelp": {
    message: "在此版本导入完成前，沙箱会从最新的可用版本启动。",
    description: "A version still importing",
  },
  "sandboxGroupPages.imageFailureTimeout": {
    message: "导入耗时过长。",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureRetries": {
    message: "多次尝试后导入仍然失败。",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureCredentials": {
    message: "bex 无法拉取该镜像。请检查镜像仓库凭据。",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureNotFound": {
    message: "bex 找不到该镜像。请检查镜像名称和标签。",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailurePull": {
    message: "bex 无法拉取该镜像。",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailurePlatform": {
    message: "该镜像适用于 {platform}。沙箱仅支持 linux/amd64。",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureMissing": {
    message:
      "该镜像缺少 {what}，而沙箱启动需要它。请使用带 shell 的镜像，例如 slim 或 Alpine 变体。",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureConvert": {
    message: "bex 无法转换该镜像。请尝试重新导入。",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureLayers": {
    message: "bex 无法读取该镜像的层。",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureStart": {
    message: "导入无法启动。",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureStopped": {
    message: "导入在完成前停止了。",
    description: "Image failure",
  },
  "sandboxGroupPages.loadImagesError": {
    message: "无法加载镜像",
    description: "Images failed to load",
  },
  "sandboxGroupPages.noMatchingImages": {
    message: "没有匹配的镜像",
    description: "Empty state with a search",
  },
  "sandboxGroupPages.firstImageTitle": {
    message: "导入你的第一个镜像",
    description: "Empty state title of the Custom images page",
  },
  "sandboxGroupPages.firstImageDescription": {
    message:
      "从镜像仓库导入镜像，沙箱即可从它启动。用 CLI 或 SDK 导入一个镜像，然后从它启动沙箱。",
    description: "Empty state body of the Custom images page",
  },
  "sandboxGroupPages.starterImportImage": {
    message: "导入镜像（linux/amd64）",
    description: "Starter step",
  },
  "sandboxGroupPages.starterStartFromImage": {
    message: "从镜像创建沙箱",
    description: "Starter step",
  },
  "sandboxGroupPages.starterImportCode": {
    message: "导入镜像",
    description: "Starter step",
  },
  "sandboxGroupPages.colImage": { message: "镜像", description: "Column" },
  "sandboxGroupPages.colSourceImage": {
    message: "来源",
    description: "Column: the registry image",
  },
  "sandboxGroupPages.colLatestAvailable": {
    message: "最新可用版本",
    description: "Column: the newest available version's digest",
  },
  "sandboxGroupPages.colUpdated": {
    message: "更新时间",
    description: "Column: the newest import",
  },
  "sandboxGroupPages.imageGoneTitle": {
    message: "未找到镜像",
    description: "An image name with no versions",
  },
  "sandboxGroupPages.imageGoneDescription": {
    message: "此组中没有名为 {name} 的镜像。",
    description: "An image name with no versions",
  },
  "sandboxGroupPages.detailLatestAvailable": {
    message: "最新可用版本",
    description: "Detail label",
  },
  "sandboxGroupPages.detailImageSource": {
    message: "来源",
    description: "Detail label: the registry",
  },
  "sandboxGroupPages.detailImage": {
    message: "镜像",
    description: "Detail label",
  },
  "sandboxGroupPages.detailDigest": {
    message: "摘要",
    description: "Detail label",
  },
  "sandboxGroupPages.detailVersionId": {
    message: "版本 ID",
    description: "Detail label",
  },
  "sandboxGroupPages.noSandboxesFromImage": {
    message: "还没有从此镜像启动的沙箱。",
    description: "Image drawer's sandboxes",
  },
  "sandboxGroupPages.general": {
    message: "常规",
    description: "Settings section",
  },
  "sandboxGroupPages.makeDefault": {
    message: "设为默认",
    description: "Button: make this the workspace's default group",
  },
  "sandboxGroupPages.madeDefault": {
    message: "{name} 现在是默认沙箱组",
    description: "Toast",
  },
  "sandboxGroupPages.save": { message: "保存", description: "Button" },
  "sandboxGroupPages.limitSaved": {
    message: "已保存并发上限",
    description: "Toast",
  },
  "sandboxGroupPages.brokeredCredentials": {
    message: "代理凭据",
    description: "What a group delete removes",
  },
  "sandboxGroupPages.deleteGroup": {
    message: "删除沙箱组",
    description: "Button and section",
  },
  "sandboxGroupPages.deleteGroupHelp": {
    message: "删除此组及其沙箱、快照、镜像和凭据。",
    description: "Delete section help",
  },
  "sandboxGroupPages.deleteGroupDefault": {
    message: "工作区的默认组无法删除。请先将另一个组设为默认组。",
    description: "Delete section help for the default group",
  },
  "sandboxGroupPages.deleteGroupTitle": {
    message: "删除沙箱组？",
    description: "Confirmation title",
  },
  "sandboxGroupPages.deleteGroupDescription": {
    message: "沙箱组 {name} 将被永久删除，其运行历史、日志和设置也会一并删除：",
    description: "Confirmation body",
  },
  "sandboxGroupPages.deleteGroupStops": {
    message: "通过 SDK、CLI 或 API 使用此组的一切都将停止工作。",
    description: "Confirmation body",
  },
  "sandboxGroupPages.groupDeleted": {
    message: "已删除沙箱组 {name}",
    description: "Toast",
  },
  "sandboxGroupPages.linkLoadError": {
    message: "无法打开链接，请稍后再试。",
    description: "A link by id failed to resolve",
  },
  "sandboxGroupPages.backToGroups": {
    message: "前往沙箱组",
    description: "Button",
  },
};

export default zh;
