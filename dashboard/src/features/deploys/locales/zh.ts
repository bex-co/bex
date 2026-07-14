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
};

export default zhDeploys;
