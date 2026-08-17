import type { TranslationEntry } from "@/i18n";

const zhWebhooks: Record<string, TranslationEntry> = {
  "webhooks.title": {
    message: "Webhooks",
    description: "Settings Integrations Webhooks section card title",
  },
  "webhooks.description": {
    message:
      "将签名的事件通知（部署、暂停、重启、扩缩容）推送到你自己的端点——无需轮询。签名密钥仅在创建时显示一次。",
    description: "Settings Webhooks section card description",
  },
  "webhooks.colEndpoint": {
    message: "端点",
    description: "Webhooks table column header — name + destination URL",
  },
  "webhooks.colEvents": {
    message: "事件",
    description: "Webhooks table column header — subscribed event types",
  },
  "webhooks.colEnabled": {
    message: "启用",
    description: "Webhooks table column header — the enable/disable switch",
  },
  "webhooks.emptyTitle": {
    message: "暂无 Webhook",
    description: "Webhooks empty-state title",
  },
  "webhooks.emptyBody": {
    message: "添加一个端点，在服务部署、重启或扩缩容时收到通知。",
    description: "Webhooks empty-state body",
  },
  "webhooks.forbiddenTitle": {
    message: "无权限",
    description: "Webhooks state when the caller lacks permission (403)",
  },
  "webhooks.forbiddenBody": {
    message: "你没有权限查看此工作区的 Webhook。",
    description: "Webhooks forbidden-state body",
  },
  "webhooks.errorTitle": {
    message: "无法加载 Webhook",
    description: "Webhooks generic error title",
  },
  "webhooks.errorBody": {
    message: "出了点问题，请重试。",
    description: "Webhooks generic error body",
  },
  "webhooks.create": {
    message: "添加 Webhook",
    description: "Button that opens the create dialog",
  },
  "webhooks.createTitle": {
    message: "添加 Webhook",
    description: "Create dialog title",
  },
  "webhooks.createDescription": {
    message:
      "每当订阅的事件发生时，bex 会向此 URL POST 一个带签名的 JSON 负载。",
    description: "Create dialog description",
  },
  "webhooks.createEnabledHelp": {
    message: "立即开始投递。你也可以稍后在设置中启用。",
    description: "Create page initial enabled-state helper copy",
  },
  "webhooks.fieldName": {
    message: "名称",
    description: "Create dialog name field label",
  },
  "webhooks.fieldNamePlaceholder": {
    message: "例如 deploy-alerts-slack-bot",
    description: "Create dialog name field placeholder",
  },
  "webhooks.fieldUrl": {
    message: "目标 URL",
    description: "Create dialog URL field label",
  },
  "webhooks.fieldEvents": {
    message: "发送的事件",
    description: "Create dialog event-type checklist label",
  },
  "webhooks.eventsLoading": {
    message: "正在加载事件类型…",
    description: "Create dialog while the event-type vocabulary loads",
  },
  "webhooks.createCancel": {
    message: "取消",
    description: "Create dialog cancel button",
  },
  "webhooks.createSubmit": {
    message: "创建 Webhook",
    description: "Create page submit button (Render: 'Create Webhook')",
  },
  "webhooks.createSuccess": {
    message: "Webhook 已创建",
    description: "Toast after a successful create",
  },
  "webhooks.createError": {
    message: "无法创建 Webhook",
    description: "Toast after a failed create",
  },
  "webhooks.createdTitle": {
    message: "Webhook 已创建",
    description: "Secret-reveal step title",
  },
  "webhooks.createdWarning": {
    message:
      "请立即复制签名密钥——之后将无法再次查看。用它来校验每次投递的 webhook-signature 请求头。",
    description: "Secret-reveal step warning (shown exactly once)",
  },
  "webhooks.copy": {
    message: "复制",
    description: "Copy-secret button label",
  },
  "webhooks.copied": {
    message: "密钥已复制到剪贴板",
    description: "Toast after copying the secret",
  },
  "webhooks.copyError": {
    message: "复制失败——请手动选中文本复制",
    description: "Toast when clipboard write fails",
  },
  "webhooks.createdDone": {
    message: "完成",
    description: "Secret-reveal step dismiss button",
  },
  "webhooks.toggle": {
    message: "启用",
    description: "Accessible label of the per-endpoint enable/disable switch",
  },
  "webhooks.enableSuccess": {
    message: "{name} 已启用",
    description: "Toast after enabling an endpoint",
  },
  "webhooks.disableSuccess": {
    message: "{name} 已停用",
    description: "Toast after disabling an endpoint",
  },
  "webhooks.toggleError": {
    message: "无法更新 {name}",
    description: "Toast after a failed enable/disable",
  },
  "webhooks.delete": {
    message: "删除",
    description: "Delete action label (icon button + confirm button)",
  },
  "webhooks.deleteConfirmTitle": {
    message: "删除 {name}？",
    description: "Delete confirmation dialog title",
  },
  "webhooks.deleteConfirmBody": {
    message: "此端点将不再收到任何事件，其投递历史也会被删除。",
    description: "Delete confirmation dialog body",
  },
  "webhooks.deleteCancel": {
    message: "取消",
    description: "Delete confirmation cancel button",
  },
  "webhooks.deleteSuccess": {
    message: "{name} 已删除",
    description: "Toast after a successful delete",
  },
  "webhooks.deleteError": {
    message: "无法删除 {name}",
    description: "Toast after a failed delete",
  },
  "webhooks.history": {
    message: "投递历史",
    description: "Accessible label of the per-endpoint history button",
  },
  "webhooks.historyTitle": {
    message: "投递记录 — {name}",
    description: "Delivery-history dialog title",
  },
  "webhooks.historyBody": {
    message:
      "发送到此端点的每个事件，最新在前。失败的投递会按指数退避重试；持续失败会停用该端点。",
    description: "Delivery-history dialog description",
  },
  "webhooks.historyEmptyTitle": {
    message: "暂无投递记录",
    description: "Delivery-history empty-state title",
  },
  "webhooks.historyEmptyBody": {
    message: "触发一个已订阅的事件（例如一次部署）后即可在这里看到。",
    description: "Delivery-history empty-state body",
  },
  "webhooks.historyErrorTitle": {
    message: "无法加载投递记录",
    description: "Delivery-history error-state title",
  },
  "webhooks.historyErrorBody": {
    message: "出了点问题，请重试。",
    description: "Delivery-history error-state body",
  },
  "webhooks.colEvent": {
    message: "事件",
    description: "Delivery-history table column header",
  },
  "webhooks.colService": {
    message: "服务",
    description: "Delivery-history table column header",
  },
  "webhooks.colStatus": {
    message: "状态",
    description: "Delivery-history table column header",
  },
  "webhooks.colAttempts": {
    message: "尝试次数",
    description: "Delivery-history table column header",
  },
  "webhooks.colResponse": {
    message: "响应",
    description: "Delivery-history table column header — last HTTP status",
  },
  "webhooks.colWhen": {
    message: "时间",
    description: "Delivery-history table column header — event age",
  },
  "webhooks.status.pending": {
    message: "等待中",
    description: "Delivery status badge — queued or between retries",
  },
  "webhooks.status.delivered": {
    message: "已送达",
    description: "Delivery status badge — endpoint answered 2xx",
  },
  "webhooks.status.failed": {
    message: "已失败",
    description: "Delivery status badge — retries exhausted",
  },
  "webhooks.loadMore": {
    message: "加载更多",
    description: "Delivery-history pagination button",
  },
  // --- event picker (w1/m49/t002) ---
  "webhooks.selectedCount": {
    message: "已选择 {count} 个事件",
    description: "Event-picker live selection counter",
  },
  "webhooks.searchEvents": {
    message: "搜索事件",
    description: "Event-picker search box placeholder + accessible label",
  },
  "webhooks.searchNoMatches": {
    message: "没有匹配的事件",
    description:
      "Event-picker empty state while a search filters everything out",
  },
  "webhooks.allEvents": {
    message: "全部事件",
    description: "Event-picker tri-state master checkbox label",
  },
  "webhooks.groupToggle": {
    message: "展开/收起 {group} 事件",
    description: "Accessible label of a group's expand/collapse chevron",
  },
  "webhooks.group.deploy": {
    message: "部署",
    description: "Event-picker group — deploy lifecycle events",
  },
  "webhooks.group.autoDeploy": {
    message: "自动部署",
    description: "Event-picker group — automatic deploy setting events",
  },
  "webhooks.group.serviceAvailability": {
    message: "服务可用性",
    description: "Event-picker group — observed service availability events",
  },
  "webhooks.group.scaling": {
    message: "伸缩",
    description: "Event-picker group — service scaling events",
  },
  "webhooks.group.cronJobRun": {
    message: "定时任务运行",
    description: "Event-picker group — cron job run events",
  },
  "webhooks.group.maintenanceMode": {
    message: "维护模式",
    description: "Event-picker group — maintenance mode events",
  },
  "webhooks.group.postgres": {
    message: "Postgres",
    description: "Event-picker group — managed Postgres events",
  },
  "webhooks.group.suspension": {
    message: "暂停",
    description: "Event-picker group — suspend/resume events",
  },
  "webhooks.group.other": {
    message: "其他",
    description:
      "Event-picker fallback group for served keys the catalog doesn't know yet",
  },
  "webhooks.event.deploy_started": {
    message: "部署开始",
    description: "Event label — deploy_started",
  },
  "webhooks.event.branch_deleted": {
    message: "分支已删除",
    description: "Webhook event label",
  },
  "webhooks.event.build_started": {
    message: "构建已开始",
    description: "Webhook event label",
  },
  "webhooks.event.build_ended": {
    message: "构建已结束",
    description: "Webhook event label",
  },
  "webhooks.event.pre_deploy_started": {
    message: "预部署已开始",
    description: "Webhook event label",
  },
  "webhooks.event.pre_deploy_ended": {
    message: "预部署已结束",
    description: "Webhook event label",
  },
  "webhooks.event.job_run_ended": {
    message: "任务运行已结束",
    description: "Webhook event label",
  },
  "webhooks.event.auto_deploy_enabled": {
    message: "自动部署已启用",
    description: "Webhook event label",
  },
  "webhooks.event.auto_deploy_disabled": {
    message: "自动部署已停用",
    description: "Webhook event label",
  },
  "webhooks.event.deploy_ended": {
    message: "部署结束",
    description: "Event label — deploy_ended",
  },
  "webhooks.event.image_pull_failed": {
    message: "镜像拉取失败",
    description: "Event label — image_pull_failed",
  },
  "webhooks.event.commit_ignored": {
    message: "提交已忽略",
    description: "Event label — commit_ignored",
  },
  "webhooks.event.server_failed": {
    message: "服务不可用",
    description: "Event label — server_failed",
  },
  "webhooks.event.server_available": {
    message: "服务恢复可用",
    description: "Event label — server_available",
  },
  "webhooks.event.autoscaling_started": {
    message: "自动伸缩已开始",
    description: "Event label — autoscaling_started",
  },
  "webhooks.event.autoscaling_ended": {
    message: "自动伸缩已结束",
    description: "Event label — autoscaling_ended",
  },
  "webhooks.event.branch_changed": {
    message: "分支已更改",
    description: "Event label — branch_changed bex extension",
  },
  "webhooks.event.cron_job_run_started": {
    message: "定时任务开始",
    description: "Event label — cron_job_run_started",
  },
  "webhooks.event.cron_job_run_ended": {
    message: "定时任务结束",
    description: "Event label — cron_job_run_ended",
  },
  "webhooks.event.maintenance_mode_enabled": {
    message: "维护模式已启用",
    description: "Event label — maintenance_mode_enabled",
  },
  "webhooks.event.maintenance_mode_uri_updated": {
    message: "维护模式 URI 已更新",
    description: "Event label — maintenance_mode_uri_updated",
  },
  "webhooks.event.postgres_created": {
    message: "Postgres 已创建",
    description: "Event label — postgres_created",
  },
  "webhooks.event.postgres_restarted": {
    message: "Postgres 已重启",
    description: "Event label — postgres_restarted",
  },
  "webhooks.event.postgres_credentials_created": {
    message: "Postgres 凭据已创建",
    description: "Event label — postgres_credentials_created",
  },
  "webhooks.event.postgres_credentials_deleted": {
    message: "Postgres 凭据已删除",
    description: "Event label — postgres_credentials_deleted",
  },
  "webhooks.event.postgres_backup_started": {
    message: "Postgres 备份开始",
    description: "Event label — postgres_backup_started",
  },
  "webhooks.event.service_suspended": {
    message: "服务已暂停",
    description: "Event label — service_suspended",
  },
  "webhooks.event.service_resumed": {
    message: "服务已恢复",
    description: "Event label — service_resumed",
  },
  "webhooks.event.server_restarted": {
    message: "服务器已重启",
    description: "Event label — server_restarted",
  },
  "webhooks.event.instance_count_changed": {
    message: "实例数量已变更",
    description: "Event label — instance_count_changed",
  },
  "webhooks.event.autoscaling_config_changed": {
    message: "自动扩缩容配置已变更",
    description: "Event label — autoscaling_config_changed",
  },
  "webhooks.event.plan_changed": {
    message: "套餐已变更",
    description:
      "Event label — plan_changed (bex's slot for Render's Instance Type Changed)",
  },
  // --- /webhooks/new create page (w1/m49/t003) ---
  "webhooks.newTitle": {
    message: "创建新 Webhook",
    description: "Create page heading (Render: 'Create a new Webhook')",
  },
  "webhooks.backToList": {
    message: "返回 Webhook 列表",
    description: "Accessible label of the create/detail pages' back link",
  },
  "webhooks.fieldNameHelp": {
    message: "此 Webhook 的唯一名称。",
    description: "Create page name-field helper copy",
  },
  "webhooks.fieldUrlHelp": {
    message: "bex 将每条通知以 POST 请求发送到此 URL。",
    description: "Create page URL-field helper copy",
  },
  "webhooks.fieldUrlPlaceholder": {
    message: "https://example.com/webhooks/bex",
    description: "Create page URL-field placeholder",
  },
  "webhooks.fieldEventsHelp": {
    message: "选择工作区中哪些事件会触发 Webhook 通知。",
    description: "Create page events-field helper copy",
  },
  "webhooks.createdView": {
    message: "查看 Webhook",
    description: "Secret step button that opens the new webhook's page",
  },
  // --- /webhook/$id detail page (w1/m49/t004) ---
  "webhooks.detailKicker": {
    message: "Webhook",
    description: "Detail page header kicker above the endpoint name",
  },
  "webhooks.idLabel": {
    message: "Webhook ID:",
    description: "Detail header id row label",
  },
  "webhooks.copyId": {
    message: "复制 Webhook ID",
    description: "Accessible label of the header id copy button",
  },
  "webhooks.copyUrl": {
    message: "复制端点 URL",
    description: "Accessible label of the header URL copy button",
  },
  "webhooks.copiedGeneric": {
    message: "已复制到剪贴板",
    description: "Toast after copying the id or URL",
  },
  "webhooks.showMore": {
    message: "再显示 {count} 个",
    description: "Event-chip expander on the detail header",
  },
  "webhooks.showLess": {
    message: "收起",
    description: "Event-chip collapser on the detail header",
  },
  "webhooks.createdByOn": {
    message: "由 {creator} 创建于 {date}",
    description: "Detail header provenance line when the creator is known",
  },
  "webhooks.createdOn": {
    message: "创建于 {date}",
    description: "Detail header provenance line without a creator",
  },
  "webhooks.tabActivity": {
    message: "活动",
    description: "Detail page tab — delivery history",
  },
  "webhooks.tabSettings": {
    message: "设置",
    description: "Detail page tab — edit + delete",
  },
  "webhooks.recentDeliveries": {
    message: "最近投递",
    description: "Activity tab heading",
  },
  "webhooks.recentDeliveriesHint": {
    message: "刷新表格以获取最新事件",
    description: "Activity tab heading hint",
  },
  "webhooks.refresh": {
    message: "刷新",
    description: "Accessible label of the Activity refresh button",
  },
  "webhooks.filterAll": {
    message: "全部",
    description: "Activity delivery filter tab",
  },
  "webhooks.filterSuccessful": {
    message: "成功",
    description: "Activity delivery filter tab — delivered only",
  },
  "webhooks.filterFailed": {
    message: "失败",
    description: "Activity delivery filter tab — failed only",
  },
  "webhooks.sentAfter": {
    message: "发送时间晚于",
    description: "Activity delivery-history lower timestamp bound",
  },
  "webhooks.sentBefore": {
    message: "发送时间早于",
    description: "Activity delivery-history upper timestamp bound",
  },
  "webhooks.transportError": {
    message: "传输错误",
    description: "Delivery evidence label when no HTTP response was received",
  },
  "webhooks.enabledBadge": {
    message: "已启用",
    description: "Detail header status badge while enabled",
  },
  "webhooks.disabledBadge": {
    message: "已禁用",
    description: "Detail header status badge while disabled",
  },
  "webhooks.view": {
    message: "查看 Webhook",
    description: "Accessible label of the list row's open-detail link",
  },
  // --- Settings tab (w1/m49/t005 + t006) ---
  "webhooks.settingsGeneral": {
    message: "常规",
    description: "Settings tab first section heading",
  },
  "webhooks.settingsStatus": {
    message: "状态",
    description: "Settings status-toggle label",
  },
  "webhooks.settingsStatusHelp": {
    message: "禁用期间不会发送任何通知。",
    description: "Settings status-toggle helper copy",
  },
  "webhooks.settingsEvents": {
    message: "订阅的事件",
    description: "Settings events section heading",
  },
  "webhooks.saveChanges": {
    message: "保存更改",
    description: "Settings submit button (disabled until dirty)",
  },
  "webhooks.updateSuccess": {
    message: "Webhook 已更新",
    description: "Toast after a successful settings save",
  },
  "webhooks.updateError": {
    message: "无法更新 Webhook",
    description: "Toast after a failed settings save",
  },
  "webhooks.secretLabel": {
    message: "签名密钥",
    description: "Settings signing-secret row label",
  },
  "webhooks.secretMintOnceNote": {
    message:
      "签名密钥仅在创建 Webhook 时显示一次，无法再次获取。如需新密钥，请删除并重新创建 Webhook。",
    description:
      "Settings note documenting bex's mint-once secret contract (w1/m49/t006 decision — deliberate Render divergence)",
  },
  "webhooks.deleteSection": {
    message: "删除 Webhook",
    description: "Settings danger-zone heading",
  },
  "webhooks.deleteTypeToConfirm": {
    message: "在下方输入 {command} 以确认。",
    description: "Type-to-confirm instruction in the delete dialog",
  },
  "webhooks.deleteCommandLabel": {
    message: "确认命令",
    description: "Accessible label of the type-to-confirm input",
  },
};

export default zhWebhooks;
