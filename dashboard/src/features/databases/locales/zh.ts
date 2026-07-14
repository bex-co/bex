import type { TranslationEntry } from "@/i18n";

const zhDatabases: Record<string, TranslationEntry> = {
  // --- List page stat tiles ---
  "databases.statTotal": {
    message: "数据库总数",
    description: "Databases page stat card label",
  },
  "databases.statAvailable": {
    message: "可用",
    description: "Databases page stat card label (healthy databases)",
  },
  "databases.statCreating": {
    message: "创建中",
    description: "Databases page stat card label (provisioning databases)",
  },
  // --- List table ---
  "databases.cardTitle": {
    message: "数据库",
    description: "Databases table card title",
  },
  "databases.colName": {
    message: "名称",
    description: "Databases table column header",
  },
  "databases.colStatus": {
    message: "状态",
    description: "Databases table column header",
  },
  "databases.colPlan": {
    message: "实例类型",
    description: "Databases table column header (instance type / tier)",
  },
  "databases.colVersion": {
    message: "版本",
    description: "Databases table column header (PostgreSQL major version)",
  },
  "databases.colStorage": {
    message: "存储",
    description: "Databases table column header (disk size)",
  },
  "databases.colCreated": {
    message: "创建时间",
    description: "Databases table column header (relative age from createdAt)",
  },
  "databases.colActions": {
    message: "操作",
    description: "Databases table actions column header (screen-reader only)",
  },
  // --- Status badges ---
  "databases.statusAvailable": {
    message: "可用",
    description: "Database status badge (CNPG cluster healthy)",
  },
  "databases.statusCreating": {
    message: "创建中",
    description: "Database status badge (provisioning)",
  },
  "databases.statusUnavailable": {
    message: "不可用",
    description: "Database status badge (provisioning failed)",
  },
  "databases.statusUnknown": {
    message: "未知",
    description: "Database status badge for an unrecognized status",
  },
  // --- List states ---
  "databases.errorTitle": {
    message: "无法加载数据库",
    description: "Databases list error card title",
  },
  "databases.errorBody": {
    message: "对 bex-api 的请求失败。请检查网络连接后重试。",
    description: "Databases list error card body",
  },
  "databases.emptyTitle": {
    message: "还没有数据库",
    description: "Databases list empty state title",
  },
  "databases.emptyBody": {
    message: "创建你的第一个托管 Postgres，它会显示在这里。",
    description: "Databases list empty state body",
  },
  // --- Row actions / delete ---
  "databases.actionsMenu": {
    message: "打开操作菜单",
    description: "Accessible label for the per-row actions trigger",
  },
  "databases.actionDelete": {
    message: "删除",
    description: "Row action: permanently delete the database",
  },
  "databases.deleteConfirmTitle": {
    message: "删除 {name}？",
    description: "Delete-confirmation dialog title",
  },
  "databases.deleteConfirmBody": {
    message:
      "这将永久删除该数据库及其所有数据——Postgres 集群、存储卷及连接凭据。此操作无法撤销。",
    description: "Delete-confirmation dialog body",
  },
  "databases.deleteConfirmPrompt": {
    message: "输入 {name} 以确认。",
    description: "Delete-confirmation typed-name prompt label",
  },
  "databases.deleteCancel": {
    message: "取消",
    description: "Delete-confirmation dialog cancel button",
  },
  "databases.deleteConfirm": {
    message: "删除数据库",
    description: "Delete-confirmation dialog confirm button",
  },
  "databases.deleteSuccess": {
    message: "正在删除 {name}…",
    description: "Toast after a delete request is accepted",
  },
  "databases.deleteError": {
    message: "无法删除 {name}。请重试。",
    description: "Toast when a delete request fails",
  },
  // --- Create dialog ---
  "databases.createButton": {
    message: "新建数据库",
    description: "Button that opens the create-database dialog",
  },
  "databases.createTitle": {
    message: "创建 Postgres 数据库",
    description: "Create-database dialog title",
  },
  "databases.createDescription": {
    message: "创建一个托管的 PostgreSQL 实例。",
    description: "Create-database dialog description",
  },
  "databases.fieldName": {
    message: "名称",
    description: "Create-database form field label (database name)",
  },
  "databases.fieldNamePlaceholder": {
    message: "my-database",
    description: "Create-database name input placeholder",
  },
  "databases.fieldNameError": {
    message: "只能使用小写字母、数字和连字符，且必须以字母开头。",
    description: "Create-database name validation message",
  },
  "databases.fieldPlan": {
    message: "实例类型",
    description: "Create-database form field label (plan / tier)",
  },
  "databases.fieldPlanPlaceholder": {
    message: "选择实例类型",
    description: "Create-database plan select placeholder",
  },
  "databases.fieldVersion": {
    message: "PostgreSQL 版本",
    description: "Create-database form field label (major version)",
  },
  "databases.fieldVersionDefault": {
    message: "默认（最新）",
    description: "Create-database version select default option",
  },
  "databases.fieldDisk": {
    message: "磁盘大小（GB）",
    description: "Create-database form field label (storage size)",
  },
  "databases.fieldPublic": {
    message: "公网访问",
    description: "Create-database form field label (external endpoint toggle)",
  },
  "databases.fieldPublicHint": {
    message: "允许通过 TLS 从集群外部连接。",
    description: "Create-database public toggle helper text",
  },
  "databases.createCancel": {
    message: "取消",
    description: "Create-database dialog cancel button",
  },
  "databases.createSubmit": {
    message: "创建数据库",
    description: "Create-database dialog submit button",
  },
  "databases.createSuccess": {
    message: "正在创建 {name}…",
    description:
      "Toast after a create request is accepted (provisioning is async)",
  },
  "databases.createError": {
    message: "无法创建 {name}。请重试。",
    description: "Toast when a create request fails",
  },
  "databases.capLimitTitle": {
    message: "已达到 Postgres 上限",
    description: "Alert title when the workspace's Postgres creation cap is hit (w7/m9)",
  },
  "databases.capLimitUpgrade": {
    message: "升级方案",
    description: "Upgrade CTA button inside the Postgres cap-limit Alert (w7/m9)",
  },
  // --- Detail metadata ---
  "databases.metaTitle": {
    message: "详情",
    description: "Database detail metadata card title",
  },
  "databases.metaStatus": {
    message: "状态",
    description: "Database detail metadata row label",
  },
  "databases.metaPlan": {
    message: "实例类型",
    description: "Database detail metadata row label",
  },
  "databases.metaVersion": {
    message: "版本",
    description: "Database detail metadata row label",
  },
  "databases.metaDatabaseName": {
    message: "数据库",
    description: "Database detail metadata row label (the normalized db name)",
  },
  "databases.metaDatabaseUser": {
    message: "用户",
    description: "Database detail metadata row label (owner role)",
  },
  "databases.metaStorage": {
    message: "存储",
    description: "Database detail metadata row label (disk size)",
  },
  "databases.metaHighAvailability": {
    message: "高可用",
    description:
      "Database detail metadata row label (HA — single-instance MVP is No)",
  },
  "databases.metaPublic": {
    message: "公网访问",
    description: "Database detail metadata row label (external endpoint)",
  },
  "databases.metaExternalHost": {
    message: "外部主机",
    description: "Database detail metadata row label (SNI hostname)",
  },
  "databases.metaCreated": {
    message: "创建时间",
    description: "Database detail metadata row label (relative age)",
  },
  "databases.yes": {
    message: "是",
    description: "Metadata value for a true boolean field",
  },
  "databases.no": {
    message: "否",
    description: "Metadata value for a false boolean field",
  },
  "databases.notFoundTitle": {
    message: "未找到数据库",
    description: "Detail page state when database(id) returns nothing",
  },
  "databases.notFoundBody": {
    message: "不存在名为 {name} 的数据库，或者你没有访问权限。",
    description: "Detail page not-found body",
  },
  // --- Connection info panel ---
  "databases.connTitle": {
    message: "连接",
    description: "Connection-info panel card title",
  },
  "databases.connDescription": {
    message: "连接字符串与数据库密码。仅在你请求时才会显示——绝不会自动展示。",
    description: "Connection-info panel card description",
  },
  "databases.connReveal": {
    message: "显示连接信息",
    description: "Button that fetches the connection info on demand",
  },
  "databases.connHide": {
    message: "隐藏连接信息",
    description: "Button that clears the revealed connection info",
  },
  "databases.connPassword": {
    message: "密码",
    description: "Connection-info field label (database password)",
  },
  "databases.connShowPassword": {
    message: "显示密码",
    description: "Accessible label to unmask the password",
  },
  "databases.connHidePassword": {
    message: "隐藏密码",
    description: "Accessible label to re-mask the password",
  },
  "databases.connInternal": {
    message: "内部连接字符串",
    description: "Connection-info field label (in-cluster URL)",
  },
  "databases.connExternal": {
    message: "外部连接字符串",
    description: "Connection-info field label (public URL, TLS)",
  },
  "databases.connPsql": {
    message: "psql 命令",
    description: "Connection-info field label (ready-to-run psql command)",
  },
  "databases.connErrorTitle": {
    message: "无法加载连接信息",
    description: "Connection-info panel error title",
  },
  "databases.connErrorBody": {
    message: "数据库可能仍在创建中，或者你没有查看其凭据的权限。",
    description: "Connection-info panel error body",
  },
  "databases.copied": {
    message: "已复制到剪贴板",
    description: "Toast after copying a connection field",
  },
  "databases.copyError": {
    message: "无法复制到剪贴板",
    description: "Toast when clipboard copy fails",
  },
  "databases.loading": {
    message: "加载中…",
    description: "Generic loading placeholder in a detail panel",
  },
  // --- Lifecycle actions (suspend / resume / restart) ---
  "databases.actionSuspend": {
    message: "暂停",
    description: "Row action: hibernate the database (stop compute, keep data)",
  },
  "databases.actionResume": {
    message: "恢复",
    description: "Row action: wake a suspended database",
  },
  "databases.actionRestart": {
    message: "重启",
    description: "Row action: rolling restart of the primary",
  },
  "databases.suspendConfirmTitle": {
    message: "暂停 {name}？",
    description: "Suspend-confirmation dialog title",
  },
  "databases.suspendConfirmBody": {
    message: "计算将停止、数据库将不可访问，但数据会保留。可随时恢复。",
    description: "Suspend-confirmation dialog body",
  },
  "databases.restartConfirmTitle": {
    message: "重启 {name}？",
    description: "Restart-confirmation dialog title",
  },
  "databases.restartConfirmBody": {
    message: "主实例将重启，恢复期间连接会短暂中断。",
    description: "Restart-confirmation dialog body",
  },
  "databases.toastSuspendSuccess": {
    message: "正在暂停 {name}…",
    description: "Toast after a suspend request is accepted",
  },
  "databases.toastResumeSuccess": {
    message: "正在恢复 {name}…",
    description: "Toast after a resume request is accepted",
  },
  "databases.toastRestartSuccess": {
    message: "正在重启 {name}…",
    description: "Toast after a restart request is accepted",
  },
  "databases.toastLifecycleError": {
    message: "无法对 {name} 执行该操作，请重试。",
    description: "Toast when a lifecycle verb fails",
  },
  // --- Recovery panel ---
  "databases.recoveryTitle": {
    message: "恢复",
    description: "Recovery panel card title",
  },
  "databases.recoveryDescription": {
    message: "时间点恢复与备份。恢复始终创建新数据库，本数据库不会被修改。",
    description: "Recovery panel card description",
  },
  "databases.recoveryDisabled": {
    message:
      "当前套餐未启用备份，无法恢复。升级到含备份的套餐以启用时间点恢复。",
    description: "Recovery panel state when the plan has no backups",
  },
  "databases.recoveryEarliest": {
    message: "最早恢复点",
    description: "Recovery panel field (earliest recoverable time)",
  },
  "databases.recoveryLatest": {
    message: "最新恢复点",
    description: "Recovery panel field (latest recoverable time)",
  },
  "databases.recoveryNoBackupYet": {
    message: "尚无备份",
    description: "Recovery panel value when the first backup hasn't landed",
  },
  "databases.recoveryRestore": {
    message: "恢复到新实例",
    description: "Recovery panel button that opens the restore dialog",
  },
  "databases.recoveryCreateExport": {
    message: "创建导出",
    description: "Recovery panel button that triggers an on-demand export",
  },
  "databases.recoveryBackups": {
    message: "备份",
    description: "Recovery panel backup-list heading",
  },
  "databases.recoveryNoBackups": {
    message: "尚无备份。",
    description: "Recovery panel empty backup list",
  },
  "databases.recoveryExports": {
    message: "导出",
    description: "Recovery panel export-list heading",
  },
  "databases.recoveryNoExports": {
    message: "尚无导出。",
    description: "Recovery panel empty export list",
  },
  "databases.recoveryRestoreTitle": {
    message: "恢复到新数据库",
    description: "Restore dialog title",
  },
  "databases.recoveryRestoreBody": {
    message: "从本数据库的备份创建一个新数据库进行恢复，源数据库保持不变。",
    description: "Restore dialog body",
  },
  "databases.recoveryRestoreName": {
    message: "新数据库名称",
    description: "Restore dialog field label",
  },
  "databases.recoveryRestoreTime": {
    message: "时间点（可选）",
    description: "Restore dialog target-time field label",
  },
  "databases.recoveryRestoreTimeHint": {
    message: "留空则恢复到最新可用时间点。",
    description: "Restore dialog target-time helper text",
  },
  "databases.recoveryRestoreConfirm": {
    message: "恢复",
    description: "Restore dialog confirm button",
  },
  "databases.recoveryExportStarted": {
    message: "导出已开始。",
    description: "Toast after an export is triggered",
  },
  "databases.recoveryExportError": {
    message: "无法开始导出，请重试。",
    description: "Toast when an export request fails",
  },
  "databases.recoveryRestoreStarted": {
    message: "正在恢复到 {name}…",
    description: "Toast after a restore request is accepted",
  },
  "databases.recoveryRestoreError": {
    message: "无法恢复：{error}",
    description: "Toast when a restore request fails",
  },
  // --- Access control panel ---
  "databases.accessTitle": {
    message: "访问控制",
    description: "Access-control panel card title",
  },
  "databases.accessDescription": {
    message: "限制外部访问、连接池，以及管理数据库用户。",
    description: "Access-control panel card description",
  },
  "databases.accessAllowList": {
    message: "IP 允许列表",
    description:
      "Access panel section heading (external-endpoint CIDR allowlist)",
  },
  "databases.accessAllowListHint": {
    message:
      "仅这些 CIDR 网段可访问外部端点。留空表示对所有来源 IP 开放。内部端点始终不受影响。",
    description: "Access panel IP allowlist helper text",
  },
  "databases.accessAllowListOpen": {
    message: "对所有来源 IP 开放。",
    description: "Access panel shown when the allowlist is empty",
  },
  "databases.accessAllowListAdd": {
    message: "添加",
    description: "Access panel button to add a CIDR to the draft allowlist",
  },
  "databases.accessAllowListRemove": {
    message: "移除 {cidr}",
    description: "Accessible label to remove a CIDR chip",
  },
  "databases.accessAllowListSave": {
    message: "保存允许列表",
    description: "Access panel button to persist the allowlist",
  },
  "databases.accessAllowListSaved": {
    message: "允许列表已更新。",
    description: "Toast after saving the IP allowlist",
  },
  "databases.accessAllowListError": {
    message: "无法保存允许列表：{error}",
    description: "Toast when saving the allowlist fails",
  },
  "databases.accessUsers": {
    message: "数据库用户",
    description: "Access panel section heading (Postgres login roles)",
  },
  "databases.accessUsersHint": {
    message: "所有者之外的额外登录角色。新用户的密码只显示一次，请立即复制。",
    description: "Access panel users helper text",
  },
  "databases.accessUsersEmpty": {
    message: "暂无额外用户。",
    description: "Access panel empty users list",
  },
  "databases.accessUserAdd": {
    message: "添加用户",
    description: "Access panel button to create a database user",
  },
  "databases.accessUserDelete": {
    message: "删除 {name}",
    description: "Accessible label to delete a database user",
  },
  "databases.accessUserPassword": {
    message: "密码",
    description: "Label for a newly-created user's password",
  },
  "databases.accessUserPasswordOnce": {
    message: "{name} 的密码（仅显示一次）：",
    description: "Access panel one-time password reveal label",
  },
  "databases.accessUserCreated": {
    message: "已创建用户 {name}。",
    description: "Toast after creating a database user",
  },
  "databases.accessUserError": {
    message: "无法创建用户：{error}",
    description: "Toast when creating a user fails",
  },
  "databases.accessUserDeleted": {
    message: "已删除用户 {name}。",
    description: "Toast after deleting a database user",
  },
  "databases.accessUserDeleteError": {
    message: "无法删除 {name}，请重试。",
    description: "Toast when deleting a user fails",
  },
  "databases.accessPooler": {
    message: "连接池",
    description: "Access panel section heading (PgBouncer pooler strings)",
  },
  "databases.accessPoolerHint": {
    message: "通过 PgBouncer 路由的连接池字符串，适用于高连接数客户端。",
    description: "Access panel pooler helper text",
  },
  "databases.accessPoolerReveal": {
    message: "显示连接池字符串",
    description: "Access panel button to fetch the pooled connection strings",
  },
  "databases.accessPoolerDisabled": {
    message: "该数据库未启用连接池。启用连接池以获取连接池字符串。",
    description: "Access panel shown when no pooler is provisioned",
  },
  "databases.accessPoolerInternal": {
    message: "内部连接池连接",
    description: "Access panel pooled-string label (in-cluster)",
  },
  "databases.accessPoolerExternal": {
    message: "外部连接池连接",
    description: "Access panel pooled-string label (public)",
  },
  "databases.accessPoolerError": {
    message: "无法加载连接池字符串。",
    description: "Toast when revealing pooled strings fails",
  },
  // --- HA panel (w1/m22) ---
  "databases.haTitle": {
    message: "高可用",
    description: "HA panel card title",
  },
  "databases.haDescription": {
    message: "具备自动故障切换的复制集群。启用后，备节点随时准备接管主节点。",
    description: "HA panel card description",
  },
  "databases.haStatus": {
    message: "状态",
    description: "HA panel status row label",
  },
  "databases.haEnabled": {
    message: "已启用",
    description: "HA panel status value when HA is active",
  },
  "databases.haDisabled": {
    message: "未启用",
    description: "HA panel status value when HA is off",
  },
  "databases.haFailover": {
    message: "触发故障切换",
    description: "HA panel button to initiate a planned switchover",
  },
  "databases.haFailoverConfirmTitle": {
    message: "触发 {name} 的故障切换？",
    description: "Failover-confirmation dialog title",
  },
  "databases.haFailoverConfirmBody": {
    message:
      "这将把备节点提升��主节点。切换期间连接会短暂中断，数据库保持可用，不会丢失数据。",
    description: "Failover-confirmation dialog body",
  },
  "databases.haFailoverCancel": {
    message: "取消",
    description: "Failover-confirmation dialog cancel button",
  },
  "databases.haFailoverConfirm": {
    message: "触发故障切换",
    description: "Failover-confirmation dialog confirm button",
  },
  "databases.haFailoverSuccess": {
    message: "已触发 {name} 的故障切换。",
    description: "Toast after a failover request is accepted",
  },
  "databases.haFailoverError": {
    message: "无法触发 {name} 的故障切换，请重试。",
    description: "Toast when a failover request fails",
  },
  "databases.haReadReplicas": {
    message: "只读副本",
    description: "HA panel section heading for named read replicas",
  },
  "databases.haReadReplicasEmpty": {
    message: "暂无命名只读副本。",
    description: "HA panel empty state for read replicas",
  },
  "databases.haReplicaInternal": {
    message: "内部",
    description: "HA panel replica connection label (in-cluster host)",
  },
  "databases.haReplicaExternal": {
    message: "外部",
    description: "HA panel replica connection label (public SNI host)",
  },
  "databases.haNotEnabled": {
    message: "该数据库未启用高可用。",
    description: "HA panel state when HA is off and there are no replicas",
  },
  // --- Insights panel (w2/m25) ---
  "databases.insightsTitle": {
    message: "洞察",
    description: "Database insights panel card title",
  },
  "databases.insightsDescription": {
    message: "进程、查询性能、存储及配置的实时内省。",
    description: "Database insights panel card description",
  },
  "databases.insightsRefresh": {
    message: "刷新洞察",
    description: "Accessible label for the insights refresh button",
  },
  "databases.insightsUnavailable": {
    message: "不可用 — 数据库可能仍在配置中。",
    description: "Insights sub-section error state",
  },
  "databases.insightsSizeTitle": {
    message: "存储",
    description: "Insights panel — database/table sizes sub-section heading",
  },
  "databases.insightsProcessesTitle": {
    message: "活跃进程",
    description: "Insights panel — pg_stat_activity sub-section heading",
  },
  "databases.insightsNoProcesses": {
    message: "没有活跃进程。",
    description: "Insights processes empty state",
  },
  "databases.insightsTopQueriesTitle": {
    message: "热门查询",
    description: "Insights panel — pg_stat_statements sub-section heading",
  },
  "databases.insightsNoTopQueries": {
    message: "暂无查询统计 — 此集群可能未启用 pg_stat_statements。",
    description: "Insights top-queries empty state",
  },
  "databases.insightsTableScansTitle": {
    message: "表扫描",
    description: "Insights panel — pg_stat_user_tables sub-section heading",
  },
  "databases.insightsNoTableScans": {
    message: "暂无用户表扫描统计。",
    description: "Insights table-scans empty state",
  },
  "databases.insightsParamsTitle": {
    message: "非默认参数",
    description: "Insights panel — pg_settings sub-section heading",
  },
  "databases.insightsNoParams": {
    message: "所有参数均为默认值。",
    description: "Insights parameter-overrides empty state",
  },
  "databases.insightsColTable": {
    message: "表",
    description: "Insights table column header (table name)",
  },
  "databases.insightsColSize": {
    message: "大小",
    description: "Insights table column header (relation size)",
  },
  "databases.insightsColUser": {
    message: "用户",
    description: "Insights processes column header (pg role)",
  },
  "databases.insightsColState": {
    message: "状态",
    description: "Insights processes column header (active/idle/…)",
  },
  "databases.insightsColDuration": {
    message: "持续时间",
    description: "Insights processes column header (seconds since query start)",
  },
  "databases.insightsColQuery": {
    message: "查询",
    description: "Insights column header (SQL text, truncated)",
  },
  "databases.insightsColCalls": {
    message: "调用次数",
    description: "Insights top-queries column header (invocation count)",
  },
  "databases.insightsColTotalTime": {
    message: "总耗时",
    description: "Insights top-queries column header (total exec time)",
  },
  "databases.insightsColMeanTime": {
    message: "平均耗时",
    description: "Insights top-queries column header (mean exec time per call)",
  },
  "databases.insightsColSeqScans": {
    message: "顺序扫描",
    description: "Insights table-scans column header (sequential scan count)",
  },
  "databases.insightsColIdxScans": {
    message: "索引扫描",
    description: "Insights table-scans column header (index scan count)",
  },
  "databases.insightsColLiveRows": {
    message: "活跃行数",
    description: "Insights table-scans column header (estimated live row count)",
  },
  "databases.insightsColParam": {
    message: "参数",
    description: "Insights parameter-overrides column header (pg_settings.name)",
  },
  "databases.insightsColSetting": {
    message: "值",
    description: "Insights parameter-overrides column header (pg_settings.setting)",
  },
  "databases.insightsColSource": {
    message: "来源",
    description: "Insights parameter-overrides column header (pg_settings.source)",
  },
};

export default zhDatabases;
