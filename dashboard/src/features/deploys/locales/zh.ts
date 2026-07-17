import type { TranslationEntry } from "@/i18n";

const zhDeploys: Record<string, TranslationEntry> = {
  "deploys.statusCreated": {
    message: "已创建",
    description: "Deploy status: created",
  },
  "deploys.statusQueued": {
    message: "排队中",
    description: "Deploy status: queued",
  },
  "deploys.statusBuildInProgress": {
    message: "构建中",
    description: "Deploy status: build_in_progress",
  },
  "deploys.statusBuildFailed": {
    message: "构建失败",
    description: "Deploy status: build_failed",
  },
  "deploys.statusPreDeployInProgress": {
    message: "预部署进行中",
    description: "Deploy status: pre_deploy_in_progress",
  },
  "deploys.statusPreDeployFailed": {
    message: "预部署失败",
    description: "Deploy status: pre_deploy_failed",
  },
  "deploys.statusUpdateInProgress": {
    message: "进行中",
    description: "Deploy status: update_in_progress",
  },
  "deploys.statusUpdateFailed": {
    message: "失败",
    description: "Deploy status: update_failed",
  },
  "deploys.statusLive": {
    message: "已上线",
    description: "Deploy status: live",
  },
  "deploys.statusCanceled": {
    message: "已取消",
    description: "Deploy status: canceled",
  },
  "deploys.statusDeactivated": {
    message: "已停用",
    description: "Deploy status: deactivated",
  },
  "deploys.statusUnknown": {
    message: "未知",
    description: "Deploy status: unrecognized backend value",
  },
  "deploys.statusLabel": {
    message: "部署状态",
    description:
      "Deploy detail header: explicit label distinguishing deploy status from service phase",
  },
  "deploys.created": {
    message: "创建于",
    description: "Deploy header: created-at label",
  },
  "deploys.updated": {
    message: "更新时间",
    description: "部署最近一次已存储状态转换时间的标签",
  },
  "deploys.started": {
    message: "开始于",
    description: "Deploy header: started-at label",
  },
  "deploys.finished": {
    message: "完成于",
    description: "Deploy header: finished-at label",
  },
  "deploys.duration": {
    message: "耗时",
    description: "Deploy header: elapsed build/deploy time label",
  },
  "deploys.durationValue": {
    message: "耗时 {duration}",
    description: "Deploy row: elapsed build/deploy time",
  },
  "deploys.durationInProgress": {
    message: "计时中",
    description: "Deploy row: elapsed time is still running",
  },
  "deploys.deployedAt": {
    message: "部署于 {timestamp}",
    description: "Deploy row: created/deployed timestamp",
  },
  "deploys.notYet": {
    message: "—",
    description:
      "Deploy header: placeholder for a timestamp that hasn't happened yet",
  },
  "deploys.triggerCreate": {
    message: "首次部署",
    description: "Deploy header: trigger=create label",
  },
  "deploys.triggerApi": {
    message: "手动部署",
    description: "Deploy header: trigger=api label",
  },
  "deploys.triggerRollback": {
    message: "回滚至 {deployId}",
    description:
      "Deploy header: trigger label for a rollback deploy, naming the restored deploy",
  },
  "deploys.notFoundTitle": {
    message: "未找到部署",
    description: "Deploy detail page: not-found state title",
  },
  "deploys.notFoundBody": {
    message: "此服务不存在部署 {deployId}。",
    description: "Deploy detail page: not-found state body",
  },
  "deploys.logSearchPlaceholder": {
    message: "搜索日志…",
    description: "Deploy detail page: log search input placeholder",
  },
  "deploys.buildLogsStoreUnavailable": {
    message: "历史构建日志不可用",
    description:
      "Deploy detail page: title shown when the durable log store isn't wired",
  },
  "deploys.buildLogsStoreUnavailableBody": {
    message: "此控制台尚未连接持久日志存储。该部署的应用日志仍可能显示在这里。",
    description:
      "Deploy detail page: accurate explanation of a storeless build-log response",
  },
  "deploys.timelineTitle": {
    message: "状态时间线",
    description: "Deploy detail page: status-timeline card title",
  },
  "deploys.timelineCreated": {
    message: "已创建部署",
    description: "Deploy timeline: deploy row was created",
  },
  "deploys.timelineStarted": {
    message: "部署已开始",
    description: "Deploy timeline: backend-provided startedAt timestamp",
  },
  "deploys.timelineInProgress": {
    message: "部署进行中",
    description: "Deploy timeline: current non-terminal deploy status",
  },
  "deploys.timelineLive": {
    message: "部署已上线",
    description: "Deploy timeline: successful terminal state",
  },
  "deploys.timelineFailed": {
    message: "部署失败",
    description: "Deploy timeline: failed terminal state",
  },
  "deploys.timelineCanceled": {
    message: "部署已取消",
    description: "Deploy timeline: canceled terminal state",
  },
  "deploys.timelineDeactivated": {
    message: "部署已停用",
    description: "Deploy timeline: deactivated terminal state",
  },
  "deploys.timelineEventsUnavailable": {
    message: "服务事件不可用；仅显示部署时间戳。",
    description:
      "Deploy timeline: graceful fallback when service-events query fails",
  },
  "deploys.listTitle": {
    message: "部署",
    description: "Deploys tab: card title over the deploy-history list",
  },
  "deploys.listEmpty": {
    message: "暂无部署。",
    description: "Deploys tab: empty state with no status filter",
  },
  "deploys.listEmptyFiltered": {
    message: "没有符合所选状态的部署。",
    description: "Deploys tab: empty state while a status filter is active",
  },
  "deploys.listStatusFilterLabel": {
    message: "按状态筛选",
    description: "Deploys tab: aria-label of the status filter dropdown",
  },
  "deploys.listStatusAll": {
    message: "全部状态",
    description: "Deploys tab: status filter option matching every deploy",
  },
  "deploys.listSearchPlaceholder": {
    message: "搜索已加载的部署和提交…",
    description:
      "Deploys tab: placeholder for client-side deploy-history search",
  },
  "deploys.listSearchLabel": {
    message: "搜索已加载的部署和提交",
    description: "Deploys tab: accessible label for deploy-history search",
  },
  "deploys.listEmptySearch": {
    message: "已加载的部署中没有匹配项。",
    description:
      "Deploys tab: empty state when client-side search has no match",
  },
  "deploys.listCountOne": {
    message: "{count} 个部署",
    description: "Deploys tab: complete singular deploy count",
  },
  "deploys.listCountMany": {
    message: "{count} 个部署",
    description: "Deploys tab: complete plural deploy count",
  },
  "deploys.listCountLoadedOne": {
    message: "已加载 {count} 个部署",
    description:
      "Deploys tab: singular loaded count when more server pages remain",
  },
  "deploys.listCountLoadedMany": {
    message: "已加载 {count} 个部署",
    description:
      "Deploys tab: plural loaded count when more server pages remain",
  },
  "deploys.listLoadMore": {
    message: "加载更多",
    description: "Deploys tab: button fetching the next page of deploy history",
  },
  "deploys.logOptions": {
    message: "日志选项",
    description:
      "Deploy log viewer: aria-label/tooltip of the options menu button",
  },
  "deploys.logRangeLabel": {
    message: "时间范围",
    description:
      "Deploy log viewer options menu: heading over the time-range choices",
  },
  "deploys.logRangeDeploy": {
    message: "部署时间窗",
    description:
      "Deploy log viewer: the default time range — the deploy's own createdAt..finishedAt window",
  },
  "deploys.logRangeLast15m": {
    message: "最近 15 分钟",
    description: "Deploy log viewer: relative time-range option (?r=15m)",
  },
  "deploys.logRangeLast1h": {
    message: "最近 1 小时",
    description: "Deploy log viewer: relative time-range option (?r=1h)",
  },
  "deploys.logRangeLast6h": {
    message: "最近 6 小时",
    description: "Deploy log viewer: relative time-range option (?r=6h)",
  },
  "deploys.logRangeLast24h": {
    message: "最近 24 小时",
    description: "Deploy log viewer: relative time-range option (?r=24h)",
  },
  "deploys.logRangeLast7d": {
    message: "最近 7 天",
    description: "Deploy log viewer: relative time-range option (?r=7d)",
  },
  "deploys.logWrap": {
    message: "自动换行",
    description:
      "Deploy log viewer options menu: toggle wrapping long log lines vs horizontal scroll",
  },
  "deploys.logTimestamps": {
    message: "显示时间戳",
    description:
      "Deploy log viewer options menu: toggle the per-line timestamp column",
  },
  "deploys.logMaximize": {
    message: "最大化",
    description:
      "Deploy log viewer: button expanding the viewer to fill the screen",
  },
  "deploys.logMinimize": {
    message: "退出全屏",
    description:
      "Deploy log viewer: button restoring the maximized viewer to its inline size",
  },
};

export default zhDeploys;
