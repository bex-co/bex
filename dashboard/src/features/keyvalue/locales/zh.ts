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
  "keyvalue.statusSuspended": {
    message: "已暂停",
    description:
      "Key Value status badge (hibernated; suspension wins over the status enum)",
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
    message:
      "请使用小写字母、数字和连字符（最多 30 个字符），且不能以连字符开头或结尾。",
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
  "keyvalue.fieldMaxmemoryPolicy": {
    message: "内存淘汰策略",
    description: "Create-Key-Value form field label (eviction policy)",
  },
  "keyvalue.fieldMaxmemoryRecommended": {
    message: "（缓存推荐）",
    description:
      "Suffix on the recommended maxmemory policy option (allkeys-lru)",
  },
  "keyvalue.fieldMaxmemoryPolicyHint": {
    message: "当存储达到内存上限时如何淘汰键。",
    description: "Create-Key-Value maxmemory policy helper text",
  },
  "keyvalue.fieldPersistenceMode": {
    message: "持久化模式",
    description: "Create-Key-Value form field label (persistence)",
  },
  "keyvalue.fieldPersistenceModeHint": {
    message: "如何将数据持久化到磁盘，以便在重启后保留。",
    description: "Create-Key-Value persistence mode helper text",
  },
  "keyvalue.fieldPersistenceFreeHint": {
    message: "免费套餐没有持久化磁盘，因此持久化处于关闭状态。",
    description:
      "Create-Key-Value persistence helper text when Free plan is selected",
  },
  "keyvalue.persistenceJournalSnapshot": {
    message: "日志 + 快照",
    description: "Persistence mode option: AOF journal plus RDB snapshots",
  },
  "keyvalue.persistenceSnapshot": {
    message: "仅快照",
    description: "Persistence mode option: RDB snapshots only",
  },
  "keyvalue.persistenceOff": {
    message: "关闭",
    description: "Persistence mode option: no persistence",
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
    description:
      "Toast after a create request is accepted (provisioning is async)",
  },
  "keyvalue.createError": {
    message: "无法创建 {name}。请重试。",
    description: "Toast when a create request fails",
  },
  "keyvalue.capLimitTitle": {
    message: "已达到键值存储上限",
    description:
      "Alert title when the workspace's KV creation cap is hit (w7/m9)",
  },
  "keyvalue.capLimitUpgrade": {
    message: "升级方案",
    description: "Upgrade CTA button inside the KV cap-limit Alert (w7/m9)",
  },
  // --- Detail metadata ---
  "keyvalue.metaTitle": {
    message: "详情",
    description: "Key Value detail metadata card title",
  },
  "keyvalue.metaId": {
    message: "ID",
    description: "Key Value detail metadata row label (immutable red- id)",
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
  "keyvalue.metaRegion": {
    message: "区域",
    description: "Key Value detail metadata row label (platform placement)",
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
    description:
      "Shown instead of the external URL when the store isn't public",
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
    message:
      "这会将存储缩容至零并断开所有活动连接。数据会被保留，随时可以恢复。",
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
  // --- Plan section (m16) ---
  "keyvalue.planTitle": {
    message: "实例规格",
    description: "Key Value detail plan-picker card title",
  },
  "keyvalue.planDescription": {
    message:
      "更改实例规格。操作员将在下次协调时调整资源——这是保留数据的滚动更新。",
    description: "Key Value detail plan-picker card description",
  },
  "keyvalue.planPickerSave": {
    message: "保存",
    description: "Key Value plan-picker save button",
  },
  "keyvalue.planPickerCancel": {
    message: "取消",
    description: "Key Value plan-picker cancel / reset button",
  },
  "keyvalue.planPickerConfirmTitle": {
    message: "切换至 {name}？",
    description: "Key Value plan-picker confirm dialog title",
  },
  "keyvalue.planPickerConfirmBody": {
    message:
      "操作员将在下次协调时调整存储的计算资源，滚动重启期间连接会短暂中断。",
    description: "Key Value plan-picker confirm dialog body",
  },
  "keyvalue.planPickerSuccess": {
    message: "正在更新规格至 {name}……",
    description: "Toast after a plan update is accepted",
  },
  "keyvalue.planPickerError": {
    message: "无法更新规格，请重试。",
    description: "Toast when a plan update fails",
  },
  // --- Networking (external-endpoint IP allowlist) ---
  "keyvalue.networkingTitle": {
    message: "网络",
    description: "Detail-page Networking card title (IP allowlist)",
  },
  "keyvalue.networkingDescription": {
    message: "限制可访问外部端点的来源 IP。",
    description: "Networking card subtitle",
  },
  "keyvalue.networkingHint": {
    message:
      "仅这些 CIDR 网段可访问外部端点。留空表示对所有来源 IP 开放。内部端点始终不受影响。",
    description: "Networking IP allowlist helper text",
  },
  "keyvalue.networkingInternalOnly": {
    message: "该存储没有外部端点；允许列表将在存储设为公开后生效。",
    description: "Networking note shown for an internal-only store",
  },
  "keyvalue.networkingOpen": {
    message: "对所有来源 IP 开放。",
    description: "Networking panel shown when the allowlist is empty",
  },
  "keyvalue.networkingAdd": {
    message: "添加",
    description: "Networking button to add a CIDR to the draft allowlist",
  },
  "keyvalue.networkingEntryDescription": {
    message: "描述（可选）",
    description:
      "Placeholder for the optional per-entry allowlist description input",
  },
  "keyvalue.networkingRemove": {
    message: "移除 {cidr}",
    description: "Accessible label to remove a CIDR chip",
  },
  "keyvalue.networkingSave": {
    message: "保存允许列表",
    description: "Networking button to persist the allowlist",
  },
  "keyvalue.networkingSaved": {
    message: "允许列表已更新。",
    description: "Toast after saving the IP allowlist",
  },
  "keyvalue.networkingError": {
    message: "无法保存允许列表：{error}",
    description: "Toast when saving the allowlist fails",
  },
  // --- Name section (rename control) ---
  "keyvalue.nameTitle": {
    message: "Key Value 名称",
    description: "Key Value detail rename card title",
  },
  "keyvalue.nameDescription": {
    message: "更改显示名称。Key Value ID 和所有连接信息保持不变。",
    description: "Key Value detail rename card description",
  },
  "keyvalue.nameSave": {
    message: "保存名称",
    description: "Key Value rename save button",
  },
  "keyvalue.nameSuccess": {
    message: "Key Value 存储已重命名为 {name}。",
    description: "Toast after a Key Value rename succeeds",
  },
  "keyvalue.nameError": {
    message: "无法重命名 Key Value 存储，请重试。",
    description: "Toast when a Key Value rename fails unexpectedly",
  },
  "keyvalue.nameConflict": {
    message: "此工作区中已存在同名的 Key Value 存储。",
    description: "Toast when a Key Value rename collides with another name",
  },
  "keyvalue.nameInvalid": {
    message:
      "请使用小写字母、数字和连字符（最多 30 个字符），且不能以连字符开头或结尾。",
    description: "Key Value rename validation message",
  },
  // --- Tab navigation ---
  "keyvalue.detailNavLabel": {
    message: "Key Value 详情导航",
    description: "aria-label for the detail-page tab nav",
  },
  "keyvalue.overviewTab": {
    message: "概览",
    description: "Detail-page Overview tab label",
  },
  "keyvalue.logsTab": {
    message: "日志",
    description: "Detail-page Logs tab label",
  },
  // --- Logs viewer ---
  "keyvalue.logsRangeLabel": {
    message: "时间范围",
    description: "Accessible label for the log time-range select",
  },
  "keyvalue.logsRange1h": {
    message: "最近 1 小时",
    description: "Log time range option",
  },
  "keyvalue.logsRange6h": {
    message: "最近 6 小时",
    description: "Log time range option",
  },
  "keyvalue.logsRange24h": {
    message: "最近 24 小时",
    description: "Log time range option",
  },
  "keyvalue.logsInstanceLabel": {
    message: "实例",
    description: "Accessible label for the log instance (pod) select",
  },
  "keyvalue.logsAllInstances": {
    message: "所有实例",
    description: "Default option in the instance filter (no filter applied)",
  },
  "keyvalue.logsSearchPlaceholder": {
    message: "搜索日志…",
    description: "Placeholder for the log text-search input",
  },
  "keyvalue.logsLoading": {
    message: "加载日志中…",
    description: "Loading state message while fetching logs",
  },
  "keyvalue.logsEmptyTitle": {
    message: "暂无日志",
    description: "Empty state title when no log lines are returned",
  },
  "keyvalue.logsEmptyBody": {
    message:
      "此时间范围内没有日志。Valkey 会记录键空间事件、慢查询和启动消息。",
    description: "Empty state body when no log lines and no filters active",
  },
  "keyvalue.logsEmptyFilteredBody": {
    message: "没有匹配当前过滤条件的日志。",
    description: "Empty state body when filters are active and nothing matches",
  },
  "keyvalue.logsUnavailableTitle": {
    message: "日志不可用",
    description: "Empty state title when the logs source is not configured",
  },
  "keyvalue.logsUnavailableBody": {
    message: "平台日志源未配置。请联系平台管理员启用 BEX_LOKI_URL。",
    description: "Empty state body when BEX_LOKI_URL is not set",
  },
  "keyvalue.logsUnauthorizedTitle": {
    message: "访问被拒绝",
    description: "Empty state title when the caller lacks can_view_logs",
  },
  "keyvalue.logsUnauthorizedBody": {
    message: "您没有权限查看此 Key Value 存储的日志。",
    description: "Empty state body for a 403 on the logs query",
  },
  "keyvalue.logsErrorTitle": {
    message: "无法加载日志",
    description: "Empty state title for an unexpected logs fetch error",
  },
};

export default zhKeyValue;
