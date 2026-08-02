import type { TranslationEntry } from "@/i18n";

const zhBlueprints: Record<string, TranslationEntry> = {
  "blueprints.resourceType": {
    message: "蓝图",
    description: "Blueprint resource type used in document titles",
  },
  // --- List page ---
  "blueprints.pageTitle": {
    message: "蓝图",
    description: "Blueprints list page heading and document title",
  },
  "blueprints.cardTitle": {
    message: "蓝图",
    description: "Blueprints table card title",
  },
  "blueprints.colName": {
    message: "名称",
    description:
      "Blueprints table column header — blueprint name (derived from repo)",
  },
  "blueprints.colRepo": {
    message: "仓库",
    description: "Blueprints table column header — source repo URL",
  },
  "blueprints.colBranch": {
    message: "分支",
    description: "Blueprints table column header — tracked branch",
  },
  "blueprints.colStatus": {
    message: "状态",
    description: "Blueprints table column header",
  },
  "blueprints.colUpdated": {
    message: "最近更新",
    description: "Blueprints table column header — updatedAt relative age",
  },
  // --- Create action ---
  "blueprints.createButton": {
    message: "新建蓝图",
    description: "Button label that opens the New Blueprint dialog",
  },
  "blueprints.createTitle": {
    message: "新建蓝图实例",
    description: "New Blueprint page title",
  },
  "blueprints.createDescription": {
    message: "连接一个包含 bex.yml 的仓库，创建并同步整套服务堆栈。",
    description: "New Blueprint page subtitle under the title",
  },
  "blueprints.createSourceTitle": {
    message: "连接仓库",
    description:
      "New Blueprint page — heading above the GitHub/Public Git source tabs (Render parity)",
  },
  "blueprints.createBranchLabel": {
    message: "分支",
    description: "New Blueprint page — branch combobox label",
  },
  "blueprints.createBranchPlaceholder": {
    message: "main",
    description: "New Blueprint page — branch combobox placeholder",
  },
  "blueprints.createBranchHint": {
    message: "包含 bex.yml 文件的仓库分支。",
    description: "New Blueprint page — branch field helper text",
  },
  "blueprints.createPathLabel": {
    message: "蓝图路径",
    description: "New Blueprint page — bex.yml path input label",
  },
  "blueprints.createPathPlaceholder": {
    message: "bex.yml",
    description: "New Blueprint page — manifest path placeholder",
  },
  "blueprints.createPathHint": {
    message:
      "蓝图文件在仓库中的路径（例如 infra/bex.yml）。默认为仓库根目录下的 bex.yml。",
    description: "New Blueprint page — manifest path helper text",
  },
  "blueprints.createNameLabel": {
    message: "蓝图名称",
    description: "New Blueprint page — name input label",
  },
  "blueprints.createAction": {
    message: "部署蓝图",
    description: "New Blueprint dialog submit button label",
  },
  "blueprints.createCancel": {
    message: "取消",
    description: "New Blueprint dialog cancel button label",
  },
  "blueprints.createSuccess": {
    message: "蓝图已创建",
    description: "Toast shown after a successful createBlueprint call",
  },
  "blueprints.createError": {
    message: "蓝图创建失败",
    description: "Toast shown when createBlueprint returns an error",
  },
  // --- Pre-create review (Render's "Review Blueprint configurations") ---
  "blueprints.previewTitle": {
    message: "预览蓝图配置",
    description:
      "New Blueprint page — heading of the pre-create fetch + validate section",
  },
  "blueprints.previewSelectSource": {
    message: "选择仓库与分支后，可在部署前预览蓝图文件。",
    description: "Review section placeholder before a source is chosen",
  },
  "blueprints.previewLoading": {
    message: "正在从仓库获取 {path}…",
    description: "Review section loading text while the manifest is fetched",
  },
  "blueprints.previewNotFoundTitle": {
    message: "未找到蓝图文件",
    description: "Review section error title when the manifest fetch fails",
  },
  "blueprints.previewNotFoundBody": {
    message: "在 {branch} 分支上未找到蓝图文件 {path}。",
    description:
      "Review section fallback error body when the backend returns no message",
  },
  "blueprints.previewRetry": {
    message: "重试",
    description: "Review section retry button after a failed manifest fetch",
  },
  "blueprints.previewInvalid": {
    message: "蓝图文件存在错误",
    description:
      "Review section error title when the fetched manifest fails validation",
  },
  "blueprints.previewValid": {
    message: "蓝图文件解析成功 — 将同步 {count} 个资源。",
    description:
      "Review section success line, with the plan's total resource count",
  },
  "blueprints.previewServices": {
    message: "服务",
    description: "Review section plan group label — services in the manifest",
  },
  "blueprints.previewDatabases": {
    message: "数据库",
    description: "Review section plan group label — databases in the manifest",
  },
  "blueprints.previewKeyValue": {
    message: "键值存储",
    description:
      "Review section plan group label — key-value instances in the manifest",
  },
  "blueprints.previewEnvGroups": {
    message: "环境变量组",
    description: "Review section plan group label — env groups in the manifest",
  },
  "blueprints.previewAutoSyncNote": {
    message:
      "此后该分支上蓝图文件的所有更新都会自动同步。创建后可暂停自动同步。",
    description:
      "Review section note about auto-sync being enabled by default (Render parity)",
  },
  "blueprints.previewError": {
    message: "无法加载蓝图预览。",
    description:
      "Review section error shown when the preview query itself fails (network)",
  },
  // --- Empty state ---
  "blueprints.emptyTitle": {
    message: "暂无蓝图",
    description: "Blueprints list empty-state heading",
  },
  "blueprints.emptyBody": {
    message:
      "每次部署含仓库来源的 bex.yml 时，蓝图将自动注册。部署完成后，您的堆栈将显示在此处，您可以对其进行同步或验证。",
    description:
      "Blueprints list empty-state body explaining auto-registration",
  },
  // --- Loading / error ---
  "blueprints.loadingBody": {
    message: "正在加载蓝图…",
    description: "Blueprints list loading state label",
  },
  "blueprints.errorTitle": {
    message: "无法加载蓝图",
    description: "Blueprints list error state heading",
  },
  // --- Status badges ---
  "blueprints.statusActive": {
    message: "活跃",
    description: "Blueprint status badge — row is active",
  },
  "blueprints.statusInSync": {
    message: "已同步",
    description: "Blueprint status badge — git-connected and in sync",
  },
  "blueprints.statusSyncing": {
    message: "同步中",
    description: "Blueprint status badge — sync in progress",
  },
  "blueprints.statusError": {
    message: "错误",
    description: "Blueprint status badge — last sync failed",
  },
  "blueprints.statusPaused": {
    message: "已暂停",
    description: "Blueprint status badge — auto-sync paused",
  },
  "blueprints.statusUnknown": {
    message: "{status}",
    description: "Blueprint status badge fallback — raw status value",
  },
  // --- Detail page ---
  "blueprints.detailTitle": {
    message: "{name} · 蓝图",
    description: "Blueprint detail page document title",
  },
  "blueprints.metaRepo": {
    message: "仓库",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaBranch": {
    message: "分支",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaPath": {
    message: "清单路径",
    description: "Blueprint detail metadata label — path to bex.yml in repo",
  },
  "blueprints.metaAutoSync": {
    message: "自动同步",
    description: "Blueprint detail metadata label — auto-sync on push toggle",
  },
  "blueprints.metaCreated": {
    message: "创建时间",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaUpdated": {
    message: "最近同步",
    description: "Blueprint detail metadata label",
  },
  "blueprints.autoSyncOn": {
    message: "开启",
    description: "Auto-sync toggle label — enabled",
  },
  "blueprints.autoSyncOff": {
    message: "关闭",
    description: "Auto-sync toggle label — disabled",
  },
  // --- Resources section ---
  "blueprints.resourcesTitle": {
    message: "托管资源",
    description:
      "Blueprint detail section heading listing services/databases managed by this blueprint",
  },
  "blueprints.resourcesEmpty": {
    message: "暂无资源——请同步蓝图以应用您的 bex.yml。",
    description: "Blueprint managed-resources empty state",
  },
  // --- Sync history ---
  "blueprints.syncHistoryTitle": {
    message: "同步历史",
    description: "Blueprint detail section heading for the sync run table",
  },
  "blueprints.syncHistoryEmpty": {
    message: "暂无同步记录。",
    description: "Blueprint sync history empty state",
  },
  "blueprints.syncColCommit": {
    message: "提交",
    description: "Sync history table column — commit SHA",
  },
  "blueprints.syncColState": {
    message: "状态",
    description: "Sync history table column — sync run state",
  },
  "blueprints.syncColStarted": {
    message: "开始时间",
    description: "Sync history table column — sync run start time",
  },
  "blueprints.syncColCompleted": {
    message: "完成时间",
    description: "Sync history table column — sync run completion time",
  },
  "blueprints.manifestTitle": {
    message: "bex.yml 清单",
    description: "Blueprint detail section heading for the stored manifest",
  },
  // --- Sync action ---
  "blueprints.syncButton": {
    message: "同步",
    description: "Button label that triggers an idempotent blueprint re-apply",
  },
  "blueprints.syncConfirmTitle": {
    message: "确认同步蓝图？",
    description: "Sync confirm dialog title",
  },
  "blueprints.syncConfirmBody": {
    message:
      "此操作将把存储的 bex.yml 重新应用到您的工作空间。应用是幂等的——已与清单匹配的资源不会被替换。",
    description: "Sync confirm dialog description",
  },
  "blueprints.syncConfirmAction": {
    message: "同步",
    description: "Sync confirm dialog primary action button label",
  },
  "blueprints.syncCancel": {
    message: "取消",
    description: "Sync confirm dialog cancel button label",
  },
  "blueprints.syncSuccess": {
    message: "蓝图同步成功",
    description: "Toast shown after a successful syncBlueprint call",
  },
  "blueprints.syncError": {
    message: "同步失败",
    description: "Toast shown when syncBlueprint returns an error",
  },
  // --- Update action ---
  "blueprints.updateSuccess": {
    message: "蓝图已更新",
    description: "Toast shown after a successful updateBlueprint call",
  },
  "blueprints.updateError": {
    message: "更新失败",
    description: "Toast shown when updateBlueprint returns an error",
  },
  // --- Disconnect action ---
  "blueprints.disconnectButton": {
    message: "断开连接",
    description:
      "Button that disconnects a blueprint from its Git repo (stops auto-sync, keeps resources)",
  },
  "blueprints.disconnectTitle": {
    message: "确认断开蓝图连接？",
    description: "Disconnect confirm dialog title",
  },
  "blueprints.disconnectBody": {
    message: "此操作将停止推送时自动同步，并从列表中移除蓝图。已部署的资源不受影响。",
    description: "Disconnect confirm dialog description",
  },
  "blueprints.disconnectAction": {
    message: "断开连接",
    description: "Disconnect confirm dialog primary action button label",
  },
  "blueprints.disconnectCancel": {
    message: "取消",
    description: "Disconnect confirm dialog cancel button label",
  },
  "blueprints.disconnectSuccess": {
    message: "蓝图已断开连接",
    description: "Toast shown after a successful disconnectBlueprint call",
  },
  "blueprints.disconnectError": {
    message: "断开连接失败",
    description: "Toast shown when disconnectBlueprint returns an error",
  },
  // --- Validate action ---
  "blueprints.validateTitle": {
    message: "验证",
    description: "Validate panel section heading",
  },
  "blueprints.validateRun": {
    message: "运行验证",
    description: "Validate panel button label",
  },
  "blueprints.validateValid": {
    message: "清单有效——未发现错误。",
    description: "Validate result: manifest parsed successfully",
  },
  "blueprints.validateInvalid": {
    message: "清单存在错误：",
    description:
      "Validate result: manifest has parse errors — followed by the error list",
  },
  "blueprints.validateNoResult": {
    message: "尚无结果。",
    description: "Validate panel placeholder before first run",
  },
};

export default zhBlueprints;
