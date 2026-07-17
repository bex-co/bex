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
  "databases.statusUpgrading": {
    message: "升级中",
    description: "Database status badge (offline PostgreSQL major upgrade)",
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
    message:
      "请使用小写字母、数字和连字符（最多 30 个字符），且不能以连字符开头或结尾。",
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
    description:
      "Alert title when the workspace's Postgres creation cap is hit (w7/m9)",
  },
  "databases.capLimitUpgrade": {
    message: "升级方案",
    description:
      "Upgrade CTA button inside the Postgres cap-limit Alert (w7/m9)",
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
  "databases.versionLabel": {
    message: "PostgreSQL {version}",
    description: "Formatted PostgreSQL major version label",
  },
  "databases.versionUpgradeAction": {
    message: "升级",
    description: "Button opening the PostgreSQL major-version upgrade dialog",
  },
  "databases.versionUpgrading": {
    message: "升级正在进行",
    description:
      "Version-row status while the database is offline for pg_upgrade",
  },
  "databases.versionUpgradeTitle": {
    message: "升级 PostgreSQL",
    description: "PostgreSQL major-version upgrade dialog title",
  },
  "databases.versionUpgradeDescription": {
    message: "将此数据库从 PostgreSQL {version} 升级到更新的受支持主版本。",
    description: "PostgreSQL major-version upgrade dialog description",
  },
  "databases.versionUpgradeTarget": {
    message: "目标版本",
    description: "PostgreSQL major-version upgrade target selector label",
  },
  "databases.versionUpgradeDowntime": {
    message:
      "离线升级期间数据库将不可用。请先测试兼容性并安排停机时间；大型数据库可能需要长达一小时。",
    description:
      "Downtime warning in the PostgreSQL major-version upgrade dialog",
  },
  "databases.versionUpgradeErrorTitle": {
    message: "升级被阻止",
    description:
      "Inline backend-guard error title in the version upgrade dialog",
  },
  "databases.versionUpgradeCancel": {
    message: "取消",
    description: "PostgreSQL major-version upgrade dialog cancel button",
  },
  "databases.versionUpgradeConfirm": {
    message: "升级到 PostgreSQL {version}",
    description: "PostgreSQL major-version upgrade dialog confirmation button",
  },
  "databases.versionUpgradeAccepted": {
    message: "已开始升级到 PostgreSQL {version}。",
    description: "Toast after a PostgreSQL major-version upgrade is accepted",
  },
  "databases.versionUpgradeError": {
    message: "无法开始 PostgreSQL 升级。请重试。",
    description: "Fallback error when the PostgreSQL upgrade mutation fails",
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
  "databases.diskAutoscalingLabel": {
    message: "磁盘自动扩容",
    description: "Label beside the database disk-usage chart toggle",
  },
  "databases.diskAutoscalingSize": {
    message: "当前 {current} GB · 上限 {cap} GB",
    description: "Current and maximum Postgres disk size beside autoscaling",
  },
  "databases.diskAutoscalingHint": {
    message:
      "使用率达到 90% 时，存储将增加 50% 并向上取整到 5 GB。扩容不可逆，且每 12 小时最多一次。",
    description: "Accessible explanation of Postgres disk autoscaling",
  },
  "databases.diskAutoscalingEnabled": {
    message: "已启用磁盘自动扩容。",
    description: "Toast after enabling Postgres disk autoscaling",
  },
  "databases.diskAutoscalingDisabled": {
    message: "已停用磁盘自动扩容。",
    description: "Toast after disabling Postgres disk autoscaling",
  },
  "databases.diskAutoscalingError": {
    message: "无法更新磁盘自动扩容设置。请重试。",
    description: "Toast when the disk-autoscaling mutation fails",
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
  "databases.metaRegion": {
    message: "区域",
    description: "Database detail metadata row label (platform placement)",
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
  "databases.recoveryDownloadExport": {
    message: "下载",
    description: "Download an available logical database export",
  },
  "databases.recoveryExportInProgress": {
    message: "已有导出正在进行。",
    description: "Reason the create-export button is disabled",
  },
  "databases.recoveryExportPreparing": {
    message: "导出仍在准备中。",
    description: "Reason an in-progress export cannot be downloaded",
  },
  "databases.recoveryExportExpired": {
    message: "此导出已过期，文件已删除。",
    description: "Reason an expired export cannot be downloaded",
  },
  "databases.recoveryExportUnavailable": {
    message: "此导出暂不可下载。",
    description: "Fallback reason an export cannot be downloaded",
  },
  "databases.recoveryExportFailure": {
    message: "导出失败：{reason}",
    description: "Logical export failure reason",
  },
  "databases.recoveryExportStatusCreated": {
    message: "已创建",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusRunning": {
    message: "进行中",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusAvailable": {
    message: "可用",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusFailed": {
    message: "失败",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusExpiring": {
    message: "正在过期",
    description: "Logical export status",
  },
  "databases.recoveryExportStatusExpired": {
    message: "已过期",
    description: "Logical export status",
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
  "databases.accessAllowListDescription": {
    message: "描述（可选）",
    description:
      "Placeholder for the optional per-entry allowlist description input",
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
  // --- Plan section (m16) ---
  "databases.planTitle": {
    message: "实例规格",
    description: "Database detail plan-picker card title",
  },
  "databases.planDescription": {
    message:
      "更改实例规格。操作员将在下次协调时调整资源——这是保留数据的滚动更新。",
    description: "Database detail plan-picker card description",
  },
  "databases.planPickerSave": {
    message: "保存",
    description: "Database plan-picker save button",
  },
  "databases.planPickerCancel": {
    message: "取消",
    description: "Database plan-picker cancel / reset button",
  },
  "databases.planPickerConfirmTitle": {
    message: "切换至 {name}？",
    description: "Database plan-picker confirm dialog title",
  },
  "databases.planPickerConfirmBody": {
    message:
      "操作员将在下次协调时调整数据库的计算资源，滚动重启期间连接会短暂中断。",
    description: "Database plan-picker confirm dialog body",
  },
  "databases.planPickerSuccess": {
    message: "正在更新规格至 {name}……",
    description: "Toast after a plan update is accepted",
  },
  "databases.planPickerError": {
    message: "无法更新规格，请重试。",
    description: "Toast when a plan update fails",
  },
  // --- Name section (w9/m3) ---
  "databases.nameTitle": {
    message: "数据库名称",
    description: "Database detail rename card title",
  },
  "databases.nameDescription": {
    message: "更改显示名称。数据库 ID 和所有连接信息保持不变。",
    description: "Database detail rename card description",
  },
  "databases.nameSave": {
    message: "保存名称",
    description: "Database rename save button",
  },
  "databases.nameSuccess": {
    message: "数据库已重命名为 {name}。",
    description: "Toast after a database rename succeeds",
  },
  "databases.nameError": {
    message: "无法重命名数据库，请重试。",
    description: "Toast when a database rename fails unexpectedly",
  },
  "databases.nameConflict": {
    message: "此工作区中已存在同名数据库。",
    description: "Toast when a database rename collides with another name",
  },
  "databases.nameInvalid": {
    message:
      "请使用小写字母、数字和连字符（最多 30 个字符），且不能以连字符开头或结尾。",
    description: "Database rename validation message",
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
  "databases.insightsNoSizes": {
    message: "暂无存储数据。",
    description: "Insights storage empty state",
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
  "databases.insightsParamsAdd": {
    message: "添加覆盖项",
    description: "Button that adds a Postgres parameter override row",
  },
  "databases.insightsParamsSave": {
    message: "保存覆盖项",
    description: "Button that replaces the Postgres parameter overrides",
  },
  "databases.insightsParamsSaving": {
    message: "正在保存…",
    description: "Postgres parameter-overrides save button while saving",
  },
  "databases.insightsParamsDiscard": {
    message: "放弃更改",
    description: "Button that restores the last saved parameter draft",
  },
  "databases.insightsParamsActions": {
    message: "操作",
    description: "Screen-reader-only header for parameter-row actions",
  },
  "databases.insightsParamNameLabel": {
    message: "参数 {index} 名称",
    description: "Accessible label for a parameter override name input",
  },
  "databases.insightsParamValueLabel": {
    message: "参数 {index} 值",
    description: "Accessible label for a parameter override value input",
  },
  "databases.insightsParamsRemove": {
    message: "移除 {name}",
    description: "Accessible label for a parameter override remove button",
  },
  "databases.insightsParamsUnnamed": {
    message: "参数 {index}",
    description: "Fallback name in the remove label for an empty draft row",
  },
  "databases.insightsParamsPending": {
    message: "待应用",
    description: "Source shown for a newly added, not-yet-applied override",
  },
  "databases.insightsParamsBlank": {
    message: "每个覆盖项都需要参数名称和值。",
    description: "Inline validation for an incomplete parameter row",
  },
  "databases.insightsParamsDuplicate": {
    message: "{name} 出现了多次。",
    description: "Inline validation for duplicate parameter names",
  },
  "databases.insightsParamsManaged": {
    message: "shared_preload_libraries 由 bex 管理，无法覆盖。",
    description:
      "Inline validation for the platform-managed Postgres parameter",
  },
  "databases.insightsParamsSaveError": {
    message: "无法保存参数覆盖项，请重试。",
    description: "Fallback error when the parameter mutation fails",
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
    description:
      "Insights table-scans column header (estimated live row count)",
  },
  "databases.insightsColParam": {
    message: "参数",
    description:
      "Insights parameter-overrides column header (pg_settings.name)",
  },
  "databases.insightsColSetting": {
    message: "值",
    description:
      "Insights parameter-overrides column header (pg_settings.setting)",
  },
  "databases.insightsColSource": {
    message: "来源",
    description:
      "Insights parameter-overrides column header (pg_settings.source)",
  },
  "databases.sqlTitle": {
    message: "SQL 控制台",
    description: "Database detail SQL console card title",
  },
  "databases.sqlDescription": {
    message:
      "对该数据库运行一条 SQL 语句。结果最多返回 500 行，查询将在 10 秒后超时。",
    description: "Database SQL console safety summary",
  },
  "databases.sqlEditorLabel": {
    message: "SQL 查询",
    description: "Accessible label for the SQL editor",
  },
  "databases.sqlShortcut": {
    message: "按 Ctrl+Enter 或 ⌘+Enter 运行",
    description: "SQL console keyboard shortcut hint",
  },
  "databases.sqlRun": {
    message: "运行查询",
    description: "SQL console run button",
  },
  "databases.sqlRunning": {
    message: "运行中…",
    description: "SQL console run button while a query is executing",
  },
  "databases.sqlUnknownError": {
    message: "查询失败。请检查语句后重试。",
    description: "SQL console fallback error",
  },
  "databases.sqlReturnedRows": {
    message: "返回 {count} 行",
    description: "SQL console SELECT result count",
  },
  "databases.sqlAffectedRows": {
    message: "影响 {count} 行",
    description: "SQL console write result count",
  },
  "databases.sqlTruncated": {
    message: "结果已限制为 {limit} 行",
    description: "SQL console backend row-cap notice",
  },
  "databases.sqlNull": {
    message: "NULL",
    description: "SQL console null cell value",
  },
  "databases.sqlNoRows": {
    message: "查询未返回任何行。",
    description: "SQL console empty result message",
  },
  "databases.sqlPrevious": {
    message: "上一页",
    description: "SQL result pagination previous button",
  },
  "databases.sqlNext": {
    message: "下一页",
    description: "SQL result pagination next button",
  },
  "databases.sqlPage": {
    message: "第 {page} 页，共 {pages} 页",
    description: "SQL result pagination position",
  },
  "databases.sqlHistory": {
    message: "查询历史",
    description: "SQL console session history heading",
  },
  "databases.sqlClearHistory": {
    message: "清除",
    description: "SQL console clear-history button",
  },
  "databases.sqlNoHistory": {
    message: "本次浏览器会话中运行的查询会显示在这里。",
    description: "SQL console empty history message",
  },
  "databases.sqlConfirmTitle": {
    message: "运行可能修改数据的语句？",
    description: "SQL console write confirmation title",
  },
  "databases.sqlConfirmDescription": {
    message:
      "该语句未被识别为只读语句。它可能更改或删除数据库数据，且 bex 无法撤销。",
    description: "SQL console write confirmation warning",
  },
  "databases.sqlConfirmCancel": {
    message: "取消",
    description: "SQL console write confirmation cancel button",
  },
  "databases.sqlConfirmRun": {
    message: "运行语句",
    description: "SQL console confirmed write action",
  },
  "databases.detailNavLabel": {
    message: "数据库详情",
    description: "Accessible label for the managed Postgres detail tabs",
  },
  "databases.overviewTab": {
    message: "概览",
    description: "Managed Postgres overview tab",
  },
  "databases.logsTab": {
    message: "日志",
    description: "Managed Postgres logs tab",
  },
  "databases.logsRangeLabel": {
    message: "时间范围",
    description: "Managed Postgres log range filter label",
  },
  "databases.logsRange1h": {
    message: "最近 1 小时",
    description: "Managed Postgres one-hour log range",
  },
  "databases.logsRange6h": {
    message: "最近 6 小时",
    description: "Managed Postgres six-hour log range",
  },
  "databases.logsRange24h": {
    message: "最近 24 小时",
    description: "Managed Postgres 24-hour log range",
  },
  "databases.logsInstanceLabel": {
    message: "数据库实例",
    description: "Managed Postgres log instance filter label",
  },
  "databases.logsAllInstances": {
    message: "所有实例",
    description: "Managed Postgres log instance filter all option",
  },
  "databases.logsSearchPlaceholder": {
    message: "搜索数据库日志",
    description: "Managed Postgres log search placeholder",
  },
  "databases.logsLoading": {
    message: "正在加载数据库日志…",
    description: "Managed Postgres logs loading state",
  },
  "databases.logsEmptyTitle": {
    message: "暂无数据库日志",
    description: "Managed Postgres logs empty-state title",
  },
  "databases.logsEmptyBody": {
    message: "此时间范围内的 Postgres 输出将显示在这里。",
    description: "Managed Postgres logs empty-state body",
  },
  "databases.logsEmptyFilteredBody": {
    message: "没有数据库日志符合当前筛选条件。",
    description: "Managed Postgres logs filtered empty-state body",
  },
  "databases.logsUnavailableTitle": {
    message: "尚未配置数据库日志",
    description: "Managed Postgres logs unavailable-state title",
  },
  "databases.logsUnavailableBody": {
    message: "此安装未配置持久日志历史或实时 CNPG Pod 日志源。",
    description: "Managed Postgres logs unavailable-state body",
  },
  "databases.logsUnauthorizedTitle": {
    message: "你无权查看这些日志",
    description: "Managed Postgres logs unauthorized-state title",
  },
  "databases.logsUnauthorizedBody": {
    message: "数据库日志需要此工作区的贡献者权限。",
    description: "Managed Postgres logs unauthorized-state body",
  },
  "databases.logsErrorTitle": {
    message: "无法加载数据库日志",
    description: "Managed Postgres logs generic error title",
  },
};

export default zhDatabases;
