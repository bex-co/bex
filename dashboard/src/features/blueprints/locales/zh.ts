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
    message: "新建蓝图实例",
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
  "blueprints.createNameTaken": {
    message: "此工作区中已有名为 {name} 的蓝图。请换一个名称。",
    description:
      "New Blueprint page — shown under the name field when Deploy was refused because an approved Blueprint already has the name (w8/250)",
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
  "blueprints.previewFileNotFoundTitle": {
    message: "未找到 Blueprint 文件",
    description:
      "Preview failure title: the repo and branch resolved, the file did not",
  },
  "blueprints.previewFileNotFoundBody": {
    message:
      "分支 {branch} 上不存在 {path}。请检查路径，或将该文件提交到该分支。",
    description: "Preview failure body for file_not_found",
  },
  "blueprints.previewBranchNotFoundTitle": {
    message: "未找到分支",
    description: "Preview failure title for branch_not_found",
  },
  "blueprints.previewBranchNotFoundBody": {
    message: "该仓库中不存在分支 {branch}。",
    description: "Preview failure body for branch_not_found",
  },
  "blueprints.previewRepoNotFoundTitle": {
    message: "未找到仓库",
    description: "Preview failure title for repo_not_found_or_no_access",
  },
  "blueprints.previewRepoNotFoundBody": {
    message:
      "该仓库不存在，或 bex 的 GitHub 应用无权访问。请检查仓库以及该应用被授予的仓库范围。",
    description: "Preview failure body for repo_not_found_or_no_access",
  },
  "blueprints.previewAccessDeniedTitle": {
    message: "访问被拒绝",
    description: "Preview failure title for access_denied",
  },
  "blueprints.previewAccessDeniedBody": {
    message: "bex 的 GitHub 应用无权读取该仓库。请重新检查安装及其授权的仓库。",
    description: "Preview failure body for access_denied",
  },
  "blueprints.previewPublicRepoNotFoundBody": {
    message:
      "该仓库在 {host} 上不存在，或不是公开仓库。bex 在该平台上只从公开仓库读取 Blueprint。",
    description:
      "Preview failure body for repo_not_found_or_no_access on gitlab.com or bitbucket.org, read anonymously; {host} is that host",
  },
  "blueprints.previewPublicAccessDeniedBody": {
    message:
      "{host} 拒绝了 bex 读取该仓库。bex 在该平台上只从公开仓库读取 Blueprint。",
    description:
      "Preview failure body for access_denied on gitlab.com or bitbucket.org, read anonymously; {host} is that host",
  },
  "blueprints.previewHostUnsupportedTitle": {
    message: "不支持的 Git 托管平台",
    description:
      "Preview failure title for a repository on a Git host bex reads no Blueprint from",
  },
  "blueprints.previewHostUnsupportedBody": {
    message:
      "与 Render 一样，bex 从 github.com、gitlab.com 或 bitbucket.org 上的公开仓库读取 Blueprint。{host} 不在其中。",
    description:
      "Preview failure body for a repository on a Git host bex reads no Blueprint from; {host} is that host",
  },
  "blueprints.previewRateLimitedTitle": {
    message: "{host} 正在限流 bex",
    description:
      "Preview failure title for rate_limited; {host} is GitHub or the repository's host",
  },
  "blueprints.previewRateLimitedBody": {
    message: "当前对 {host} 的请求过多，请几分钟后重试。",
    description:
      "Preview failure body for rate_limited; {host} is GitHub or the repository's host",
  },
  "blueprints.previewAmbiguousTitle": {
    message: "存在两个 Blueprint 文件",
    description: "Preview failure title for ambiguous_filename",
  },
  "blueprints.previewAmbiguousBody": {
    message:
      "该仓库在分支 {branch} 上同时存在 render.yaml 和 bex.yml。请删除其中一个，或在上方显式指定文件。",
    description: "Preview failure body for ambiguous_filename",
  },
  "blueprints.previewUnavailableTitle": {
    message: "无法连接 {host}",
    description:
      "Preview failure title for unavailable; {host} is GitHub or the repository's host",
  },
  "blueprints.previewUnavailableBody": {
    message: "bex 无法从 {host} 读取 Blueprint 文件，请几分钟后重试。",
    description:
      "Preview failure body for unavailable; {host} is GitHub or the repository's host",
  },
  "blueprints.previewInvalidPathExtension": {
    message: "Blueprint 路径必须是 .yaml 或 .yml 文件。",
    description: "Inline path-field error for invalid_path, wrong extension",
  },
  "blueprints.previewInvalidPathShape": {
    message:
      "Blueprint 路径必须是干净的仓库相对路径（不能以斜杠开头，不能包含 ..）。",
    description: "Inline path-field error for invalid_path, bad shape",
  },
  "blueprints.previewRetry": {
    message: "重试",
    description: "Review section retry button after a failed manifest fetch",
  },
  "blueprints.takeoverResourceTitle": {
    message: "接管 “{name}”？",
    description:
      "Takeover dialog title when a resource is managed by another Blueprint (BLUEPRINT_RESOURCE_CONFLICT)",
  },
  "blueprints.takeoverResourceBody": {
    message:
      "{kind} “{name}” 由 Blueprint {owner} 管理。继续将把它转移到此 Blueprint。请输入下方命令以继续。",
    description: "Takeover dialog body for a Blueprint resource conflict",
  },
  "blueprints.takeoverResourceBodyOtherOwner": {
    message:
      "{kind} “{name}” 由另一个 Blueprint 管理。继续将把它转移到此 Blueprint。请输入下方命令以继续。",
    description:
      "Takeover dialog body for a Blueprint resource conflict whose owning Blueprint bex-api could not name",
  },
  "blueprints.previewConflictBody": {
    message:
      "其中部分资源已由另一个 Blueprint 管理。部署会把它们转移到此 Blueprint；下一步会确认具体接管的内容。",
    description:
      "Preview alert body on /blueprint/new when the only problem is a takeover conflict (replaces the raw API message)",
  },
  "blueprints.previewConflict": {
    message: "已被占用 — 部署将接管",
    description:
      "Alert title on /blueprint/new when the only preview problem is a resource another Blueprint manages, which an explicit takeover confirmation resolves (w4/m125)",
  },
  "blueprints.previewMatchTitle": {
    message: "工作区中已有同名资源",
    description:
      "Notice title on /blueprint/new when the Blueprint file names resources that already exist in the workspace (w8/m153, Render parity)",
  },
  "blueprints.previewMatchBody": {
    message:
      "此蓝图中的部分资源与工作区中已有的资源同名。部署时会新建资源，并在每个名称后加上后缀“-{suffix}”，已有的资源不会改动。",
    description:
      "Notice body on /blueprint/new for matched resources; {suffix} is the four-character resource name suffix, shown after a hyphen as it appears in a name",
  },
  "blueprints.previewMatchListLabel": {
    message: "工作区中已有：",
    description:
      "Notice on /blueprint/new — label above the list of existing resources the Blueprint file matches",
  },
  "blueprints.previewInvalid": {
    message: "蓝图文件存在错误",
    description:
      "Review section error title when the fetched manifest fails validation",
  },
  "blueprints.previewValid_other": {
    message: "蓝图文件解析成功 — 将变更 {count} 个资源。",
    description:
      "Review section success line, with the plan's total resource count",
  },
  "blueprints.previewValidEnvironments_other": {
    message: "蓝图文件解析成功 — 将变更 {count} 个环境。",
    description:
      "Review section success line when only environments change (w9/095, plural)",
  },
  "blueprints.previewValidWithEnvironments_other": {
    message: "蓝图文件解析成功 — 将变更 {count} 个资源和{environments}。",
    description:
      "Review section success line: changing resources (plural) and blueprints.previewEnvironmentCount (w9/095)",
  },
  "blueprints.previewEnvironmentCount_other": {
    message: "{count} 个环境",
    description:
      "The changing environments inside blueprints.previewValidWithEnvironments (plural)",
  },
  "blueprints.errorAtLineColumn": {
    message: "第 {line} 行，第 {column} 列",
    description: "Source location beside a Blueprint validation error (w4/151)",
  },
  "blueprints.errorAtLine": {
    message: "第 {line} 行",
    description:
      "Source location beside a Blueprint validation error when only the line is known (w4/151)",
  },
  "blueprints.previewNoChanges": {
    message: "蓝图文件解析成功 — 没有变更，所有资源都已与此文件一致。",
    description:
      "Review section success line when every planned action is a no-op (w4/m138)",
  },
  "blueprints.previewOpCreate": {
    message: "创建",
    description:
      "Plan row badge: the resource does not exist yet and will be created",
  },
  "blueprints.previewOpUpdate": {
    message: "更新",
    description: "Plan row badge: an existing resource will be changed",
  },
  "blueprints.previewOpNoop": {
    message: "无变更",
    description: "Plan row badge: the resource already matches the file",
  },
  "blueprints.previewOpError": {
    message: "无法应用",
    description:
      "Plan row badge: the planner refused this resource (reason below)",
  },
  "blueprints.previewChangedFields": {
    message: "变更：{fields}",
    description:
      "Plan row detail for an update: the changed field paths (never values)",
  },
  "blueprints.previewKindService": {
    message: "服务",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindPostgres": {
    message: "Postgres",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindKeyValue": {
    message: "Key Value",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindEnvGroup": {
    message: "环境组",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindBuildSource": {
    message: "构建源",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindEnvironment": {
    message: "环境",
    description: "Plan row resource kind: a projects[].environments[] entry",
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
    message: "Key Value",
    description:
      "Review section plan group label — key-value instances in the manifest",
  },
  "blueprints.previewEnvGroups": {
    message: "环境组",
    description: "Review section plan group label — env groups in the manifest",
  },
  "blueprints.previewManualSyncNote": {
    message:
      "bex 收不到此仓库的推送，此后蓝图文件的更改不会自动同步。请在每次更改后同步蓝图，或通过 GitHub 应用连接此仓库。",
    description:
      "Review section note for a repo whose pushes can't reach bex (the preview's autoSync is false): the Blueprint is created without auto-sync",
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
  "blueprints.generateCopyFailed": {
    message: "无法复制 render.yaml。请允许访问剪贴板后重试，或直接下载。",
    description:
      "Toast when the browser refuses to copy the generated manifest to the clipboard",
  },
  "blueprints.generateDownload": {
    message: "下载 render.yaml",
    description: "Generate dialog — download the manifest file",
  },
  "blueprints.generateSecretsNote": {
    message:
      "已存储的值绝不导出。标记为 sync: false 的服务变量会在首次创建蓝图时提示填写。标记为 generateValue 的变量（包括导出的环境组键）会保留现有值，缺失时自动生成。",
    description: "Generate dialog note under the yaml preview",
  },
  "blueprints.generateError": {
    message: "无法生成蓝图",
    description: "Toast when the generate query fails",
  },
  // --- Empty state ---
  "blueprints.emptyTitle": {
    message: "你还没有创建任何蓝图实例。",
    description: "Blueprints list empty-state heading",
  },
  "blueprints.emptyBody": {
    message:
      "每次部署含仓库来源的 render.yaml 时，蓝图将自动注册。部署完成后，你的堆栈将显示在此处，你可以对其进行同步或验证。",
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
  "blueprints.resourceSuffixLabel": {
    message: "资源名称后缀",
    description:
      "Tooltip and screen-reader label of the suffix shown beside a Blueprint's name in the list and on its detail page (w8/m153)",
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
    message: "暂无资源——请同步蓝图以应用你的 render.yaml。",
    description: "Blueprint managed-resources empty state (never synced)",
  },
  "blueprints.resourcesEmptyAwaitingApproval": {
    message: "暂无资源——批准计划同步后即会创建 render.yaml 中的资源。",
    description:
      "Blueprint managed-resources empty state while the Blueprint awaits approval of its planned sync",
  },
  "blueprints.resourcesEmptyAfterError": {
    message:
      "没有归属此蓝图的资源。上次同步失败——请查看同步历史中的错误，修复蓝图后再重试。",
    description:
      "Blueprint managed-resources empty state when status is error after a sync",
  },
  // --- Syncs (w3/m678) ---
  "blueprints.syncHistoryEmpty": {
    message: "暂无同步记录。",
    description: "Syncs page: no syncs yet, Render's wording (w3/m678)",
  },
  "blueprints.detachPreviewTitle": {
    message: "资源将不再受此蓝图管理",
    description: "Blueprint resource detachment notice.",
  },
  "blueprints.detachEstimateNote": {
    message: "下方的费用估算不包含这些资源。",
    description:
      "Detached resources are outside the proposed manifest estimate.",
  },
  "blueprints.detachWarning": {
    message: "这些资源将继续运行，并可能继续产生费用。此蓝图将不再管理它们。",
    description: "Blueprint resource detachment notice.",
  },
  "blueprints.manifestTitle": {
    message: "render.yaml 清单",
    description: "Blueprint detail section heading for the stored manifest",
  },
  // --- A sync's page (w3/m678) ---
  "blueprints.plannedActions": {
    message: "计划的变更",
    description:
      "A sync's page: accessible label of its list of planned actions (w3/m678)",
  },
  "blueprints.plannedAction": {
    message: "{operation} {kind} {name}",
    description:
      "Planned-sync card: one plan action, e.g. 'create service api'",
  },
  "blueprints.plannedManualEnvVars": {
    message: "这些值不在 Blueprint 文件中（sync: false）。请输入它们以批准。",
    description:
      "A waiting sync's page: intro to the fields for the sync: false values approving it needs (w3/m678)",
  },
  "blueprints.plannedManualEnvVarLabel": {
    message: "{service}：{key}",
    description:
      "A waiting sync's page: label of one sync: false value's field (w3/m678)",
  },
  "blueprints.plannedApprove": {
    message: "批准",
    description:
      "A waiting sync's page: the button that applies the planned sync, Render's Approve (w3/m678)",
  },
  "blueprints.plannedChanged": {
    message: "有人重新规划了此 Blueprint。请查看最新的计划后再批准。",
    description:
      "A sync's page: inline alert when the plan on screen was replaced before it was approved (BLUEPRINT_PLAN_CHANGED)",
  },
  "blueprints.plannedReplanFailed": {
    message: "无法计划新的同步",
    description:
      "Sync Error banner: Manual sync could not plan a sync and the server gave no reason (w3/m678)",
  },
  // --- Sync action ---
  "blueprints.syncButton": {
    message: "手动同步",
    description:
      "The Blueprint header's one action, Render's Manual sync: plans a sync and opens its page (w3/m678)",
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
  "blueprints.disconnectBusy": {
    message: "同步正在应用中——待其完成后重试断开连接",
    description:
      "Toast shown when disconnectBlueprint refuses with BLUEPRINT_SYNC_BUSY",
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
  "blueprints.stateCreated": {
    message: "待批准",
    description:
      "Sync state (and Blueprint status) created: a planned sync waiting for approval (w2/m256); never a queued run.",
  },
  "blueprints.stateRunning": {
    message: "进行中",
    description: "Sync-run state: the run is in progress.",
  },
  "blueprints.stateSuccess": {
    message: "成功",
    description: "Sync-run state: the run completed successfully.",
  },
  "blueprints.createNameDescription": {
    message: "此蓝图实例的唯一名称。",
    description: "Create-Blueprint Name row description",
  },
  "blueprints.createBlockedSource": {
    message: "请选择仓库后继续。",
    description:
      "Create-Blueprint action bar: why Deploy is held while no repository is chosen",
  },
  "blueprints.createBlockedBranch": {
    message: "请输入分支后继续。",
    description:
      "Create-Blueprint action bar: why Deploy is held while the branch is empty",
  },
  "blueprints.createBlockedReviewing": {
    message: "正在检查蓝图文件…",
    description:
      "Create-Blueprint action bar: why Deploy is held while the Blueprint file is being reviewed",
  },
  "blueprints.createBlockedReview": {
    message: "请先解决检查发现的问题再继续。",
    description:
      "Create-Blueprint action bar: why Deploy is held while the review found problems in the Blueprint file",
  },
  "blueprints.navResources": {
    message: "资源",
    description:
      "Blueprint sidebar entry and page heading for the resources the Blueprint manages (Render's Resources, /blueprints/<id>/resources, w3/m677)",
  },
  "blueprints.navSyncs": {
    message: "同步记录",
    description:
      "Blueprint sidebar entry and page heading for the Blueprint's syncs (Render's Syncs, /blueprints/<id>/syncs, w3/m677)",
  },
  "blueprints.navSettings": {
    message: "设置",
    description:
      "Blueprint sidebar entry and page heading for the Blueprint's settings (Render's Settings, /blueprints/<id>/settings, w3/m677)",
  },
  "blueprints.colType": {
    message: "类型",
    description:
      "Blueprint Resources table column header for a managed resource's type",
  },
  "blueprints.resourcesViewSyncs": {
    message: "前往同步记录",
    description:
      "Link from a Blueprint's empty Resources page to its Syncs page, where the waiting plan or the failed sync is (w3/m677)",
  },
  "blueprints.syncViewDetails": {
    message: "查看详情",
    description:
      "Sync card: link to the sync's own page (Render's View details, w3/m678)",
  },
  "blueprints.syncsLoadMore": {
    message: "加载更多",
    description:
      "Syncs page: button reading the next page of older syncs (w3/m678)",
  },
  "blueprints.syncsLoadMoreError": {
    message: "无法加载更早的同步。",
    description:
      "Syncs page: toast when reading an older page of syncs fails (w3/m678)",
  },
  "blueprints.syncPageTitle": {
    message: "同步：{sha}",
    description:
      "A sync's page heading, Render's 'Sync: <short commit id>'; {sha} is the 7-character commit id (w3/m678)",
  },
  "blueprints.syncPageTitleNoCommit": {
    message: "同步",
    description:
      "A sync's page heading when the sync read no commit, Render's 'Sync' (w3/m678)",
  },
  "blueprints.syncDocumentTitle": {
    message: "同步",
    description:
      "Document title's page name on a sync's page: 'Sync ・ <name> ・ Blueprint' (w3/m678)",
  },
  "blueprints.syncUpToDate": {
    message: "资源已是最新",
    description:
      "A sync's page when its plan has no actions, Render's wording (w3/m678)",
  },
  "blueprints.syncNotFullySynced": {
    message: "我们未能完全同步这些资源。",
    description:
      "A failed sync's page with no actions and no errors, Render's critical line (w3/m678)",
  },
  "blueprints.syncNotFullySyncedBody": {
    message: "如果此错误持续出现，请联系支持团队。",
    description:
      "Under blueprints.syncNotFullySynced: what to do if it keeps happening (w3/m678)",
  },
  "blueprints.syncActionsNotRecorded": {
    message: "此同步记录于同步开始保存其操作之前，因此无法显示其操作。",
    description:
      "A sync's page for a sync recorded before every sync kept its actions (w3/m678)",
  },
  "blueprints.syncActionCreate": {
    message: "创建{kind} {name}",
    description:
      "A sync's planned action: create a resource; {kind} is its kind label, {name} its name, drawn bold (w3/m678)",
  },
  "blueprints.syncActionUpdate": {
    message: "更新{kind} {name}",
    description:
      "A sync's planned action: change a resource; {kind} is its kind label, {name} its name (w3/m678)",
  },
  "blueprints.syncActionDetach": {
    message: "停止管理{kind} {name}",
    description:
      "A sync's planned action: stop managing a resource the file no longer names; {kind}, {name} (w3/m678)",
  },
  "blueprints.syncActionError": {
    message: "无法应用{kind} {name}",
    description:
      "A sync's planned action the planner refused; {kind}, {name}; the reason follows (w3/m678)",
  },
  "blueprints.syncActionDetachNote": {
    message: "它会继续运行，并可能产生费用。",
    description:
      "Under a 'stop managing' action: the resource is not deleted (w3/m678)",
  },
  "blueprints.syncActionStateUnknown": {
    message: "状态未知",
    description:
      "Accessible name of a planned action's state icon when bex can't say which state it reached (a failed sync's actions, w3/m678)",
  },
  "blueprints.syncApproveFailed": {
    message: "无法批准此同步，请重试。",
    description:
      "A sync's page: inline alert when Approve fails without the server's reason, Render's wording (w3/m678)",
  },
  "blueprints.syncNotFound": {
    message: "此同步不存在。24 小时内未批准的计划同步会被删除。",
    description:
      "A sync's page for an id that is not one of this Blueprint's syncs, e.g. a plan deleted after 24 hours (w3/m678)",
  },
  "blueprints.syncErrorBannerTitle": {
    message: "同步错误",
    description:
      "Title of Render's dismissible banner when Manual sync could not plan a sync (w3/m678)",
  },
  "blueprints.syncErrorBannerDismiss": {
    message: "关闭",
    description:
      "Accessible name of the Sync Error banner's close button (w3/m678)",
  },
  "blueprints.syncErrorsIntro_other": {
    message: "与你的 Blueprint 同步时出现问题：",
    description:
      "A sync's page: the line above its errors, Render's 'There was a problem' / 'There were a few problems' (w3/m678)",
  },
};

export default zhBlueprints;
