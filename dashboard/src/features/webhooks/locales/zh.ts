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
    message: "每当订阅的事件发生时，bex 会向此 URL POST 一个带签名的 JSON 负载。",
    description: "Create dialog description",
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
    message: "创建",
    description: "Create dialog submit button",
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
};

export default zhWebhooks;
