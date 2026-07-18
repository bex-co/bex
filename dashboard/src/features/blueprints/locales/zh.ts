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
  "blueprints.metaCreated": {
    message: "创建时间",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaUpdated": {
    message: "最近同步",
    description: "Blueprint detail metadata label",
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
