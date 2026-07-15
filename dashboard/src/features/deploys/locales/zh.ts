import type { TranslationEntry } from "@/i18n";

const zhDeploys: Record<string, TranslationEntry> = {
  "deploys.created": {
    message: "创建于",
    description: "Deploy header: created-at label",
  },
  "deploys.started": {
    message: "开始于",
    description: "Deploy header: started-at label",
  },
  "deploys.finished": {
    message: "完成于",
    description: "Deploy header: finished-at label",
  },
  "deploys.notYet": {
    message: "—",
    description: "Deploy header: placeholder for a timestamp that hasn't happened yet",
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
    description: "Deploy header: trigger label for a rollback deploy, naming the restored deploy",
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
    message: "构建日志需要日志存储。",
    description: "Deploy detail page: shown when the durable log store isn't wired, so build-log lines can't be fetched",
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
  "deploys.listLoadMore": {
    message: "加载更多",
    description: "Deploys tab: button fetching the next page of deploy history",
  },
  "deploys.logOptions": {
    message: "日志选项",
    description: "Deploy log viewer: aria-label/tooltip of the options menu button",
  },
  "deploys.logRangeLabel": {
    message: "时间范围",
    description: "Deploy log viewer options menu: heading over the time-range choices",
  },
  "deploys.logRangeDeploy": {
    message: "部署时间窗",
    description: "Deploy log viewer: the default time range — the deploy's own createdAt..finishedAt window",
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
    description: "Deploy log viewer options menu: toggle wrapping long log lines vs horizontal scroll",
  },
  "deploys.logTimestamps": {
    message: "显示时间戳",
    description: "Deploy log viewer options menu: toggle the per-line timestamp column",
  },
  "deploys.logMaximize": {
    message: "最大化",
    description: "Deploy log viewer: button expanding the viewer to fill the screen",
  },
  "deploys.logMinimize": {
    message: "退出全屏",
    description: "Deploy log viewer: button restoring the maximized viewer to its inline size",
  },
};

export default zhDeploys;
