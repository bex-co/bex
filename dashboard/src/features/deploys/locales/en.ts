import type { TranslationEntry } from "@/i18n";

const enDeploys: Record<string, TranslationEntry> = {
  "deploys.created": {
    message: "Created",
    description: "Deploy header: created-at label",
  },
  "deploys.started": {
    message: "Started",
    description: "Deploy header: started-at label",
  },
  "deploys.finished": {
    message: "Finished",
    description: "Deploy header: finished-at label",
  },
  "deploys.notYet": {
    message: "—",
    description: "Deploy header: placeholder for a timestamp that hasn't happened yet",
  },
  "deploys.triggerCreate": {
    message: "first deploy",
    description: "Deploy header: trigger=create label",
  },
  "deploys.triggerApi": {
    message: "manual deploy",
    description: "Deploy header: trigger=api label",
  },
  "deploys.triggerRollback": {
    message: "rollback to {deployId}",
    description: "Deploy header: trigger label for a rollback deploy, naming the restored deploy",
  },
  "deploys.notFoundTitle": {
    message: "Deploy not found",
    description: "Deploy detail page: not-found state title",
  },
  "deploys.notFoundBody": {
    message: "No deploy {deployId} exists for this service.",
    description: "Deploy detail page: not-found state body",
  },
  "deploys.logSearchPlaceholder": {
    message: "Search logs…",
    description: "Deploy detail page: log search input placeholder",
  },
  "deploys.buildLogsStoreUnavailable": {
    message: "Build logs need the log store.",
    description: "Deploy detail page: shown when the durable log store isn't wired, so build-log lines can't be fetched",
  },
};

export default enDeploys;
