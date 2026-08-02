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
    description: "New Blueprint dialog/page title",
  },
  "blueprints.createRepoLabel": {
    message: "仓库 URL",
    description: "New Blueprint dialog — repo URL input label",
  },
  "blueprints.createRepoPlaceholder": {
    message: "https://github.com/org/repo",
    description: "New Blueprint dialog — repo URL input placeholder",
  },
  "blueprints.createBranchLabel": {
    message: "分支",
    description: "New Blueprint dialog — branch input label",
  },
  "blueprints.createBranchPlaceholder": {
    message: "main",
    description: "New Blueprint dialog — branch input placeholder",
  },
  "blueprints.createPathLabel": {
    message: "清单路径",
    description: "New Blueprint dialog — bex.yml path input label",
  },
  "blueprints.createPathPlaceholder": {
    message: "bex.yml",
    description: "New Blueprint dialog — manifest path placeholder",
  },
  "blueprints.createNameLabel": {
    message: "名称（可选）",
    description: "New Blueprint dialog — optional name input label",
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
