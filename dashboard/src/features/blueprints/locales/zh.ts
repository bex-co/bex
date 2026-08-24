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
    message: "连接一个包含 render.yaml 的仓库，创建并同步整套服务堆栈。",
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
    message: "包含 render.yaml 文件的仓库分支。",
    description: "New Blueprint page — branch field helper text",
  },
  "blueprints.createPathLabel": {
    message: "蓝图路径",
    description: "New Blueprint page — render.yaml path input label",
  },
  "blueprints.createNamePlaceholder": {
    message: "my-stack",
    description: "New Blueprint page — blueprint name placeholder",
  },
  "blueprints.createPathPlaceholder": {
    message: "render.yaml",
    description: "New Blueprint page — manifest path placeholder",
  },
  "blueprints.createPathHint": {
    message:
      "蓝图文件在仓库中的路径——任何 .yaml/.yml 文件均可（例如 infra/bex/stack.yaml）。默认为仓库根目录下的 render.yaml。",
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
  // --- sync:false secret prompts (w8/m21, Render's create-time prompt) ---
  "blueprints.promptTitle": {
    message: "机密值",
    description:
      "Review section — heading for the sync:false env var prompt inputs",
  },
  "blueprints.promptHint": {
    message:
      "此蓝图声明了 sync: false 的环境变量——其值只存于 bex，不入仓库。现在填写；仅在首次创建时写入，后续同步不会覆盖。",
    description: "sync:false prompt section helper text",
  },
  "blueprints.promptEmptyWarning": {
    message:
      "留空的值将不会设置——服务可能启动失败，可稍后在其 Environment 页补填。",
    description:
      "Warning below the sync:false inputs when at least one is blank",
  },
  // --- Estimated pricing (Render's Blueprint pricing panel, w8/m18) ---
  "blueprints.pricingTitle": {
    message: "预估价格",
    description: "Review section — estimated-pricing panel heading",
  },
  "blueprints.pricingSubtitle": {
    message: "计算费用按秒计费。账单在每月初生成。",
    description:
      "Estimated-pricing panel subtitle explaining billing semantics (Render parity)",
  },
  "blueprints.pricingLineAmount": {
    message: "({tier}) ${amount} / 月",
    description:
      "Estimated-pricing row amount — plan label + monthly dollar cost",
  },
  "blueprints.pricingLineBreakdown": {
    message: "实例 ${instance} + 磁盘（{gb} GB）${storage}",
    description:
      "Estimated-pricing datastore row sub-line — instance vs provisioned-storage cost breakdown",
  },
  "blueprints.pricingVariable": {
    message: "浮动",
    description:
      "Estimated-pricing row amount for resources whose cost depends on runtime behavior",
  },
  "blueprints.pricingReasonAutoscaling": {
    message: "自动扩缩容",
    description:
      "Estimated-pricing variable-row badge — service has autoscaling enabled",
  },
  "blueprints.pricingReasonMultiInstance": {
    message: "多实例",
    description:
      "Estimated-pricing variable-row badge — service declares numInstances > 1",
  },
  "blueprints.pricingReasonCron": {
    message: "定时任务",
    description:
      "Estimated-pricing variable-row badge — cron jobs bill only while runs execute",
  },
  "blueprints.pricingTotalLabel": {
    message: "合计",
    description: "Estimated-pricing panel total row label",
  },
  "blueprints.pricingTotalAmount": {
    message: "${amount}{marker} / 月",
    description:
      "Estimated-pricing panel total amount; marker is an asterisk when variable costs are excluded",
  },
  "blueprints.pricingExclusions": {
    message: "* 不含{items}。",
    description:
      "Estimated-pricing footnote listing the variable costs excluded from the total",
  },
  "blueprints.pricingExcludeAutoscaling": {
    message: "自动扩缩容",
    description: "Excluded-cost phrase in the estimated-pricing footnote",
  },
  "blueprints.pricingExcludeMultiInstance": {
    message: "额外固定实例",
    description: "Excluded-cost phrase in the estimated-pricing footnote",
  },
  "blueprints.pricingExcludeCron": {
    message: "定时任务",
    description: "Excluded-cost phrase in the estimated-pricing footnote",
  },
  "blueprints.pricingEstimateNote": {
    message: "仅为预估——存储按预配容量估算；实际账单按用量计费。",
    description:
      "Estimated-pricing disclaimer: forward estimate is an upper bound of metered billing",
  },
  "blueprints.generateButton": {
    message: "生成蓝图",
    description: "Blueprints list header — opens the export dialog (w8/m22)",
  },
  "blueprints.generateTitle": {
    message: "生成蓝图",
    description: "Generate dialog title",
  },
  "blueprints.generateDescription": {
    message: "选择现有资源导出为 render.yaml，可提交到仓库并连接为蓝图。",
    description: "Generate dialog description",
  },
  "blueprints.generateEmptyHint": {
    message: "请至少选择一个资源。",
    description: "Generate dialog hint when nothing is selected",
  },
  "blueprints.generateAction": {
    message: "生成",
    description: "Generate dialog primary action",
  },
  "blueprints.generateBack": {
    message: "返回",
    description: "Generate dialog — return from preview to selection",
  },
  "blueprints.generateCopy": {
    message: "复制",
    description: "Generate dialog — copy the yaml to the clipboard",
  },
  "blueprints.generateCopied": {
    message: "render.yaml 已复制",
    description: "Toast after copying the generated manifest",
  },
  "blueprints.generateDownload": {
    message: "下载 render.yaml",
    description: "Generate dialog — download the manifest file",
  },
  "blueprints.generateSecretsNote": {
    message:
      "机密值绝不导出——机密变量以 sync: false 形式出现，首次创建蓝图时会提示填写。",
    description: "Generate dialog note under the yaml preview",
  },
  "blueprints.generateError": {
    message: "无法生成蓝图",
    description: "Toast when the generate query fails",
  },
  // --- Empty state ---
  "blueprints.emptyTitle": {
    message: "暂无蓝图",
    description: "Blueprints list empty-state heading",
  },
  "blueprints.emptyBody": {
    message:
      "每次部署含仓库来源的 render.yaml 时，蓝图将自动注册。部署完成后，您的堆栈将显示在此处，您可以对其进行同步或验证。",
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
    description:
      "Blueprint detail metadata label — path to render.yaml in repo",
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
    message: "暂无资源——请同步蓝图以应用您的 render.yaml。",
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
  "blueprints.syncColError": {
    message: "错误",
    description:
      "Sync history table column — failure reason for an error-state run",
  },
  "blueprints.manifestTitle": {
    message: "render.yaml 清单",
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
      "此操作将把存储的 render.yaml 重新应用到您的工作空间。应用是幂等的——已与清单匹配的资源不会被替换。",
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
  "blueprints.syncPreviewLoading": {
    message: "正在从仓库计算同步计划…",
    description: "Pre-sync dialog — loading line while blueprintPreview runs",
  },
  "blueprints.syncPreviewInvalid": {
    message:
      "分支上当前的蓝图文件存在问题——现在同步可能失败或回退到已存储的清单：",
    description:
      "Pre-sync dialog — heading above validation errors from the fetched manifest",
  },
  "blueprints.syncPreviewUnavailable": {
    message:
      "无法计算同步计划（预览不可用）。仍可同步——后端会在应用前重新校验。",
    description:
      "Pre-sync dialog — graceful-degrade warning when the preview query fails",
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
  "blueprints.editField": {
    message: "编辑{field}",
    description: "Aria label for the inline-edit pencil button",
  },
  "blueprints.saveField": {
    message: "保存{field}",
    description: "Aria label for the inline-edit save button",
  },
  "blueprints.cancelEdit": {
    message: "取消编辑",
    description: "Aria label for the inline-edit cancel button",
  },
  "blueprints.pathInvalid": {
    message:
      "必须是干净的仓库相对 .yaml/.yml 路径（不能以 / 开头，不能包含 ..）。",
    description: "Inline path editor client-side validation message",
  },
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
    message:
      "此操作将停止推送时自动同步，并从列表中移除蓝图。已部署的资源不受影响。",
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
