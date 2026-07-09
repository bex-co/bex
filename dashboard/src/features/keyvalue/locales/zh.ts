import type { TranslationEntry } from "@/i18n";

const zhKeyValue: Record<string, TranslationEntry> = {
  // --- List page stat tiles ---
  "keyvalue.statTotal": {
    message: "键值存储总数",
    description: "Key Value page stat card label",
  },
  "keyvalue.statAvailable": {
    message: "可用",
    description: "Key Value page stat card label (healthy stores)",
  },
  "keyvalue.statCreating": {
    message: "创建中",
    description: "Key Value page stat card label (provisioning stores)",
  },
  // --- List table ---
  "keyvalue.cardTitle": {
    message: "键值存储",
    description: "Key Value table card title",
  },
  "keyvalue.colName": {
    message: "名称",
    description: "Key Value table column header",
  },
  "keyvalue.colStatus": {
    message: "状态",
    description: "Key Value table column header",
  },
  "keyvalue.colPlan": {
    message: "实例类型",
    description: "Key Value table column header (plan / tier)",
  },
  "keyvalue.colVersion": {
    message: "版本",
    description: "Key Value table column header (Valkey version)",
  },
  "keyvalue.colCreated": {
    message: "创建时间",
    description: "Key Value table column header (relative age from createdAt)",
  },
  "keyvalue.colActions": {
    message: "操作",
    description: "Key Value table actions column header (screen-reader only)",
  },
  // --- Status badges ---
  "keyvalue.statusAvailable": {
    message: "可用",
    description: "Key Value status badge (Valkey instance healthy)",
  },
  "keyvalue.statusCreating": {
    message: "创建中",
    description: "Key Value status badge (provisioning)",
  },
  "keyvalue.statusUnavailable": {
    message: "不可用",
    description: "Key Value status badge (provisioning failed)",
  },
  "keyvalue.statusUnknown": {
    message: "未知",
    description: "Key Value status badge for an unrecognized status",
  },
  // --- List states ---
  "keyvalue.errorTitle": {
    message: "无法加载键值存储",
    description: "Key Value list error card title",
  },
  "keyvalue.errorBody": {
    message: "对 bex-api 的请求失败。请检查网络连接后重试。",
    description: "Key Value list error card body",
  },
  "keyvalue.emptyTitle": {
    message: "还没有键值存储",
    description: "Key Value list empty state title",
  },
  "keyvalue.emptyBody": {
    message: "创建你的第一个托管键值存储，它会显示在这里。",
    description: "Key Value list empty state body",
  },
  // --- Row actions / delete ---
  "keyvalue.actionsMenu": {
    message: "打开操作菜单",
    description: "Accessible label for the per-row actions trigger",
  },
  "keyvalue.actionDelete": {
    message: "删除",
    description: "Row action: permanently delete the Key Value store",
  },
  "keyvalue.deleteConfirmTitle": {
    message: "删除 {name}？",
    description: "Delete-confirmation dialog title",
  },
  "keyvalue.deleteConfirmBody": {
    message:
      "这将永久删除该键值存储及其所有数据——Valkey 实例、存储卷及连接凭据。此操作无法撤销。",
    description: "Delete-confirmation dialog body",
  },
  "keyvalue.deleteConfirmPrompt": {
    message: "输入 {name} 以确认。",
    description: "Delete-confirmation typed-name prompt label",
  },
  "keyvalue.deleteCancel": {
    message: "取消",
    description: "Delete-confirmation dialog cancel button",
  },
  "keyvalue.deleteConfirm": {
    message: "删除键值存储",
    description: "Delete-confirmation dialog confirm button",
  },
  "keyvalue.deleteSuccess": {
    message: "正在删除 {name}…",
    description: "Toast after a delete request is accepted",
  },
  "keyvalue.deleteError": {
    message: "无法删除 {name}。请重试。",
    description: "Toast when a delete request fails",
  },
  // --- Create form ---
  "keyvalue.createButton": {
    message: "新建键值存储",
    description: "Button that navigates to the create-Key-Value page",
  },
  "keyvalue.createTitle": {
    message: "创建键值存储",
    description: "Create-Key-Value page title",
  },
  "keyvalue.createDescription": {
    message: "创建一个托管的 Valkey（兼容 Redis）实例。",
    description: "Create-Key-Value page description",
  },
  "keyvalue.fieldName": {
    message: "名称",
    description: "Create-Key-Value form field label (store name)",
  },
  "keyvalue.fieldNamePlaceholder": {
    message: "example-key-value-name",
    description: "Create-Key-Value name input placeholder",
  },
  "keyvalue.fieldNameError": {
    message: "只能使用小写字母、数字和连字符，且必须以字母开头。",
    description: "Create-Key-Value name validation message",
  },
  "keyvalue.fieldPlan": {
    message: "实例类型",
    description: "Create-Key-Value form field label (plan / tier)",
  },
  "keyvalue.fieldVersion": {
    message: "Valkey 版本",
    description: "Create-Key-Value form field label (major version)",
  },
  "keyvalue.fieldVersionDefault": {
    message: "默认（最新）",
    description: "Create-Key-Value version select default option",
  },
  "keyvalue.fieldPublic": {
    message: "公网访问",
    description: "Create-Key-Value form field label (external endpoint toggle)",
  },
  "keyvalue.fieldPublicHint": {
    message: "允许通过 TLS 从集群外部连接。",
    description: "Create-Key-Value public toggle helper text",
  },
  "keyvalue.createCancel": {
    message: "取消",
    description: "Create-Key-Value page cancel button",
  },
  "keyvalue.createSubmit": {
    message: "创建键值存储实例",
    description: "Create-Key-Value page submit button",
  },
  "keyvalue.createSuccess": {
    message: "正在创建 {name}…",
    description: "Toast after a create request is accepted (provisioning is async)",
  },
  "keyvalue.createError": {
    message: "无法创建 {name}。请重试。",
    description: "Toast when a create request fails",
  },
  // --- Detail metadata ---
  "keyvalue.metaTitle": {
    message: "详情",
    description: "Key Value detail metadata card title",
  },
  "keyvalue.metaStatus": {
    message: "状态",
    description: "Key Value detail metadata row label",
  },
  "keyvalue.metaPlan": {
    message: "实例类型",
    description: "Key Value detail metadata row label",
  },
  "keyvalue.metaVersion": {
    message: "版本",
    description: "Key Value detail metadata row label",
  },
  "keyvalue.metaPublic": {
    message: "公网访问",
    description: "Key Value detail metadata row label (external endpoint)",
  },
  "keyvalue.metaExternalHost": {
    message: "外部主机",
    description: "Key Value detail metadata row label (SNI hostname)",
  },
  "keyvalue.metaCreated": {
    message: "创建时间",
    description: "Key Value detail metadata row label (relative age)",
  },
  "keyvalue.yes": {
    message: "是",
    description: "Metadata value for a true boolean field",
  },
  "keyvalue.no": {
    message: "否",
    description: "Metadata value for a false boolean field",
  },
  "keyvalue.notFoundTitle": {
    message: "未找到键值存储",
    description: "Detail page state when keyValue(id) returns nothing",
  },
  "keyvalue.notFoundBody": {
    message: "不存在名为 {name} 的键值存储，或者你没有访问权限。",
    description: "Detail page not-found body",
  },
  // --- Connection info panel ---
  "keyvalue.connTitle": {
    message: "连接",
    description: "Connection-info panel card title",
  },
  "keyvalue.connDescription": {
    message: "该键值存储的连接字符串。仅在你请求时才会显示——绝不会自动展示。",
    description: "Connection-info panel card description",
  },
  "keyvalue.connReveal": {
    message: "显示连接信息",
    description: "Button that fetches the connection info on demand",
  },
  "keyvalue.connHide": {
    message: "隐藏连接信息",
    description: "Button that clears the revealed connection info",
  },
  "keyvalue.connInternal": {
    message: "内部键值存储 URL",
    description: "Connection-info field label (in-cluster redis:// URL)",
  },
  "keyvalue.connExternal": {
    message: "外部键值存储 URL",
    description: "Connection-info field label (public rediss:// URL)",
  },
  "keyvalue.connExternalUnavailable": {
    message: "尚未公开访问。启用公网访问后可获得外部 URL。",
    description: "Shown instead of the external URL when the store isn't public",
  },
  "keyvalue.connCli": {
    message: "Valkey CLI 命令",
    description: "Connection-info field label (ready-to-run redis-cli command)",
  },
  "keyvalue.connErrorTitle": {
    message: "无法加载连接信息",
    description: "Connection-info panel error title",
  },
  "keyvalue.connErrorBody": {
    message: "该存储可能仍在创建中，或者你没有查看其凭据的权限。",
    description: "Connection-info panel error body",
  },
  "keyvalue.copied": {
    message: "已复制到剪贴板",
    description: "Toast after copying a connection field",
  },
  "keyvalue.copyError": {
    message: "无法复制到剪贴板",
    description: "Toast when clipboard copy fails",
  },
  // --- Suspend / resume ---
  "keyvalue.lifecycleTitle": {
    message: "生命周期",
    description: "Suspend/resume card title",
  },
  "keyvalue.lifecycleDescription": {
    message: "暂停该存储以缩容至零，或将其恢复。",
    description: "Suspend/resume card description",
  },
  "keyvalue.actionSuspend": {
    message: "暂停键值存储实例",
    description: "Suspend action button label",
  },
  "keyvalue.actionResume": {
    message: "恢复键值存储实例",
    description: "Resume action button label",
  },
  "keyvalue.confirmSuspendTitle": {
    message: "暂停 {name}？",
    description: "Suspend-confirmation dialog title",
  },
  "keyvalue.confirmSuspendBody": {
    message: "这会将存储缩容至零并断开所有活动连接。数据会被保留，随时可以恢复。",
    description: "Suspend-confirmation dialog body",
  },
  "keyvalue.confirmCancel": {
    message: "取消",
    description: "Suspend-confirmation dialog cancel button",
  },
  "keyvalue.toastSuspendSuccess": {
    message: "正在暂停 {name}…",
    description: "Toast after a suspend request is accepted",
  },
  "keyvalue.toastSuspendError": {
    message: "无法暂停 {name}。请重试。",
    description: "Toast when a suspend request fails",
  },
  "keyvalue.toastResumeSuccess": {
    message: "正在恢复 {name}…",
    description: "Toast after a resume request is accepted",
  },
  "keyvalue.toastResumeError": {
    message: "无法恢复 {name}。请重试。",
    description: "Toast when a resume request fails",
  },
};

export default zhKeyValue;
