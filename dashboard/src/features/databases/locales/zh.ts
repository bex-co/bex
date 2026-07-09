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
};

export default zhDatabases;
