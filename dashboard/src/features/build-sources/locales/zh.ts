import type { TranslationEntry } from "@/i18n";

const zhBuildSources: Record<string, TranslationEntry> = {
  "buildSources.pageTitle": {
    message: "构建源",
    description: "Title of the Build Sources list page",
  },
  "buildSources.pageDescription": {
    message: "从 Git 仓库或 Docker 镜像构建一次，并在多个服务间共享部署产物。",
    description: "Subtitle of the Build Sources list page",
  },
  "buildSources.resourceType": {
    message: "构建源",
    description: "Singular resource type label shown in global search results",
  },
  "buildSources.newButton": {
    message: "新建构建源",
    description: "Primary CTA button to create a new Build Source",
  },
  "buildSources.backToList": {
    message: "返回构建源列表",
    description: "Back button label returning to /build-sources",
  },
  "buildSources.viewAll": {
    message: "查看全部",
    description: "Link from Project Overview Build Sources section to /build-sources",
  },
  "buildSources.viewBuildSources": {
    message: "查看构建源",
    description: "Button in partial-link alert navigating to /build-sources",
  },
  "buildSources.emptyTitle": {
    message: "暂无构建源",
    description: "Empty state heading when the workspace has no build sources",
  },
  "buildSources.emptyDescription": {
    message:
      "创建构建源以在 Web 服务、私有服务、后台工作进程和定时任务之间共享同一次 Git 构建或 Docker 镜像。",
    description: "Empty state explanation on /build-sources",
  },
  "buildSources.refreshError": {
    message: "无法刷新构建源列表，当前显示缓存结果。",
    description: "Inline error banner when a background refresh fails over cached rows",
  },
  "buildSources.searchPlaceholder": {
    message: "按名称、仓库、镜像或项目搜索…",
    description: "Placeholder for the /build-sources search input",
  },
  "buildSources.searchLabel": {
    message: "搜索构建源",
    description: "Accessible label for the /build-sources search input",
  },
  "buildSources.resetFilters": {
    message: "重置筛选",
    description: "Button to clear active search and kind filters on /build-sources",
  },
  "buildSources.filterKindLabel": {
    message: "按来源类型筛选",
    description: "Accessible label for the source-kind filter button group",
  },
  "buildSources.filterAll": {
    message: "全部",
    description: "Filter button showing both Git and Docker image build sources",
  },
  "buildSources.kindGit": {
    message: "Git",
    description: "Source kind badge and filter label for Git-backed build sources",
  },
  "buildSources.kindImage": {
    message: "Docker 镜像",
    description: "Source kind badge and filter label for Docker-image-backed build sources",
  },
  "buildSources.noMatchesTitle": {
    message: "没有匹配的构建源",
    description: "Heading when search or kind filter matches zero build sources",
  },
  "buildSources.noMatchesDescription": {
    message: "请尝试调整搜索词或清除来源类型筛选。",
    description: "Body copy when search or kind filter matches zero build sources",
  },
  "buildSources.colName": {
    message: "名称",
    description: "Table column header for Build Source name",
  },
  "buildSources.colSource": {
    message: "来源",
    description: "Table column header for Git repo/branch or Docker image",
  },
  "buildSources.colRegion": {
    message: "区域",
    description: "Table column header for Build Source region",
  },
  "buildSources.colLinkedServices": {
    message: "已关联服务",
    description: "Table column header for linked service count",
  },
  "buildSources.colProject": {
    message: "项目",
    description: "Table column header for optional project scope",
  },
  "buildSources.colUpdated": {
    message: "更新时间",
    description: "Table column header for last updated timestamp",
  },
  "buildSources.copyId": {
    message: "复制构建源 ID",
    description: "Accessible label for copying a bds- ID",
  },
  "buildSources.copied": {
    message: "已复制",
    description: "Copy button confirmation tooltip",
  },
  "buildSources.copyError": {
    message: "复制失败",
    description: "Copy button failure tooltip",
  },
  "buildSources.linkedServicesCount_other": {
    message: "{count} 个服务",
    description: "Count of services linked to a build source",
  },
  "buildSources.workspaceScope": {
    message: "工作区",
    description: "Label when a build source is workspace-wide (not scoped to a project)",
  },
  "buildSources.createTitle": {
    message: "新建构建源",
    description: "Page heading of /build-sources/new",
  },
  "buildSources.createDescription": {
    message: "配置可供多个服务共享部署的 Git 仓库构建或 Docker 镜像。",
    description: "Subtitle of /build-sources/new",
  },
  "buildSources.createSubmit": {
    message: "创建构建源",
    description: "Submit button label on /build-sources/new",
  },
  "buildSources.createSuccess": {
    message: "已创建构建源“{name}”",
    description: "Toast shown after creating a Build Source",
  },
  "buildSources.createError": {
    message: "无法创建构建源，请重试。",
    description: "Fallback error toast when createBuildSource fails",
  },
  "buildSources.updateSuccess": {
    message: "已更新构建源“{name}”",
    description: "Toast shown after updating a Build Source",
  },
  "buildSources.updateError": {
    message: "无法更新构建源，请重试。",
    description: "Fallback error toast when updateBuildSource fails",
  },
  "buildSources.deleteSuccess": {
    message: "已删除构建源“{name}”",
    description: "Toast shown after deleting a Build Source",
  },
  "buildSources.deleteError": {
    message: "无法删除构建源，请重试。",
    description: "Fallback error toast when deleteBuildSource fails",
  },
  "buildSources.triggerBuildSuccess": {
    message: "已触发构建",
    description: "Toast shown after triggering a Build Source build",
  },
  "buildSources.triggerBuildError": {
    message: "无法触发构建，请重试。",
    description: "Fallback error toast when triggerBuild fails",
  },
  "buildSources.createPermissionDenied": {
    message: "你需要此工作区的开发者或管理员权限才能创建构建源。",
    description: "Read-only notice shown on /build-sources/new when can_create is false",
  },
  "buildSources.createBlockedSource": {
    message: "请先选择一个 Git 仓库或 Docker 镜像以继续。",
    description: "Action bar hint while no source is selected",
  },
  "buildSources.partiallyLinkedTitle": {
    message: "构建源已创建，但部分服务关联失败",
    description: "Alert heading when createBuildSource returns 409 BUILD_SOURCE_PARTIALLY_LINKED",
  },
  "buildSources.partiallyLinkedDescription": {
    message: "构建源 {id} 已创建，但未能关联所有选中的服务。",
    description: "Alert summary naming the newly created bds- ID on partial link conflict",
  },
  "buildSources.partiallyLinkedSucceeded": {
    message: "已关联：{services}",
    description: "List of services that were successfully linked before the conflict",
  },
  "buildSources.partiallyLinkedFailed": {
    message: "关联失败：{service}",
    description: "The service whose link failed during create",
  },
  "buildSources.fieldSource": {
    message: "来源",
    description: "Create form label for Git repository vs Docker image picker",
  },
  "buildSources.fieldSourceDescription": {
    message: "选择已连接的 GitHub 仓库、公开 Git 地址或已有 Docker 镜像。",
    description: "Help text for the source picker row on /build-sources/new",
  },
  "buildSources.fieldName": {
    message: "名称",
    description: "Create form label for Build Source name",
  },
  "buildSources.fieldNameDescription": {
    message: "在当前工作区内唯一的名称。",
    description: "Help text for Build Source name",
  },
  "buildSources.fieldNamePlaceholder": {
    message: "my-shared-api",
    description: "Placeholder for Build Source name input",
  },
  "buildSources.errorNameTaken": {
    message: "当前工作区已存在名为“{name}”的构建源。",
    description: "Inline error when BUILD_SOURCE_NAME_TAKEN is returned",
  },
  "buildSources.fieldProject": {
    message: "项目",
    description: "Create form label for optional Project scope",
  },
  "buildSources.fieldProjectDescription": {
    message: "可选：将此构建源限定到单个项目。",
    description: "Help text for optional Project selector",
  },
  "buildSources.fieldProjectNone": {
    message: "无（工作区级）",
    description: "Option leaving the build source unscoped at workspace level",
  },
  "buildSources.fieldRuntime": {
    message: "运行时",
    description: "Create form label for Git build source runtime",
  },
  "buildSources.fieldRuntimeDescription": {
    message: "选择原生运行时或 Docker。",
    description: "Help text for Git build source runtime selector",
  },
  "buildSources.fieldBranch": {
    message: "分支",
    description: "Create form label for Git branch",
  },
  "buildSources.fieldBranchDescription": {
    message: "用于构建的 Git 分支，默认使用仓库的默认分支。",
    description: "Help text for Git branch input",
  },
  "buildSources.fieldRegion": {
    message: "区域",
    description: "Create form label for Git build source region",
  },
  "buildSources.fieldRegionDescription": {
    message: "Git 构建源的区域必须与其关联的服务一致。",
    description: "Help text for Git build source region selector",
  },
  "buildSources.fieldRegionDefault": {
    message: "默认工作区区域",
    description: "Option leaving the build source region unset to use the platform default",
  },
  "buildSources.fieldGitRegistryDescription": {
    message: "可选：用于在 Docker 构建期间拉取私有基础镜像的凭据。",
    description: "Help text for registry credential on Docker Git build sources",
  },
  "buildSources.fieldEnvGroups": {
    message: "构建时环境组",
    description: "Label for selecting workspace env groups in build-time environment",
  },
  "buildSources.fieldEnvGroupsDescription": {
    message: "将工作区级环境组关联到构建环境中；限定于特定环境的环境组不可用。",
    description: "Help text for build-time env group checkboxes",
  },
  "buildSources.fieldEnvGroupsEmpty": {
    message: "暂无可用的工作区级环境组。",
    description: "Empty message when no workspace-level env groups exist",
  },
  "buildSources.linkServicesTitle": {
    message: "关联现有服务",
    description: "Section heading for optional initial serviceIds linking",
  },
  "buildSources.linkServicesDescription": {
    message:
      "可选：立即关联现有的 Web 服务、私有服务、后台工作进程或定时任务。静态站点不支持使用构建源。",
    description: "Section description for optional initial serviceIds linking",
  },
  "buildSources.linkServicesEmpty": {
    message: "此工作区暂无可关联的服务。",
    description: "Empty message when workspace has no linkable services",
  },
  "buildSources.projectSectionTitle": {
    message: "构建源",
    description: "Heading of the Build Sources section on Project Overview",
  },
  "buildSources.projectLoadError": {
    message: "无法加载此项目的构建源。",
    description: "Error text inside Project Overview Build Sources section",
  },
  "buildSources.projectEmptyDescription": {
    message: "此项目下暂无构建源。",
    description: "Empty state text inside Project Overview Build Sources section",
  },
  "buildSources.viewBuildSource": {
    message: "打开构建源",
    description: "Button navigating to a newly created Build Source detail page",
  },
  "buildSources.detailTabsLabel": {
    message: "构建源分区",
    description: "Accessible label for the Build Source detail tab navigation",
  },
  "buildSources.tabBuilds": {
    message: "构建",
    description: "Tab label for Build Source build history",
  },
  "buildSources.tabServices": {
    message: "服务",
    description: "Tab label for Build Source linked services",
  },
  "buildSources.tabEnvironment": {
    message: "环境变量",
    description: "Tab label for Build Source build-time environment",
  },
  "buildSources.tabSettings": {
    message: "设置",
    description: "Tab label for Build Source settings and danger zone",
  },
  "buildSources.triggerBuildButton": {
    message: "触发构建",
    description: "Primary CTA to trigger a new build on a Git build source",
  },
  "buildSources.triggerSubmitting": {
    message: "正在触发…",
    description: "Submit button label while triggerBuild mutation is in flight",
  },
  "buildSources.triggerDialogTitle": {
    message: "触发构建",
    description: "Title of the Trigger Build modal dialog",
  },
  "buildSources.triggerDialogDescription": {
    message: "为“{name}”启动新构建，并选择构建成功后要部署的关联服务。",
    description: "Description of the Trigger Build modal dialog",
  },
  "buildSources.triggerScopeLegend": {
    message: "构建成功后的部署范围",
    description: "Legend for the deploy.serviceScope radio group",
  },
  "buildSources.triggerScopeAutoDeploy": {
    message: "自动部署关联服务",
    description: "Radio option for deploy.serviceScope=autodeploy_only",
  },
  "buildSources.triggerScopeAutoDeployHint": {
    message: "部署所有已开启自动部署的关联服务。",
    description: "Hint text for deploy.serviceScope=autodeploy_only",
  },
  "buildSources.triggerScopeNone": {
    message: "仅构建（不部署）",
    description: "Radio option for deploy.serviceScope=none",
  },
  "buildSources.triggerScopeNoneHint": {
    message: "仅生成构建产物，不部署任何关联服务。",
    description: "Hint text for deploy.serviceScope=none",
  },
  "buildSources.triggerScopeCustom": {
    message: "选定服务",
    description: "Radio option for deploy.serviceScope=custom",
  },
  "buildSources.triggerScopeCustomHint": {
    message: "选择构建成功后要部署的特定关联服务。",
    description: "Hint text for deploy.serviceScope=custom",
  },
  "buildSources.triggerScopeCustomNoServices": {
    message: "此构建源尚未关联任何服务。",
    description: "Message inside custom deploy scope when serviceLinks is empty",
  },
  "buildSources.triggerScopeCustomSelectOne": {
    message: "请至少选择一个要部署的服务。",
    description: "Validation hint when custom scope has zero checked services",
  },
  "buildSources.triggerClearCache": {
    message: "清除构建缓存",
    description: "Checkbox label for clearCache=clear",
  },
  "buildSources.triggerClearCacheHint": {
    message: "不复用缓存层，从头重新构建。",
    description: "Hint text for clearCache checkbox",
  },
  "buildSources.triggerResultTitle": {
    message: "构建已开始",
    description: "Confirmation heading in Trigger Build dialog after success",
  },
  "buildSources.triggerResultDescription": {
    message: "构建 {id} 已进入队列。",
    description: "Confirmation summary naming the new bld- ID",
  },
  "buildSources.triggerResultDeploysTitle_other": {
    message: "已排队 {count} 个服务部署",
    description: "Plural heading listing fan-out deploys queued by triggerBuild",
  },
  "buildSources.triggerResultNoDeploys": {
    message: "本次构建未排队任何服务部署。",
    description: "Message when triggerBuild returned an empty deploys array",
  },
  "buildSources.triggerResultSkippedTitle_other": {
    message: "已跳过 {count} 个关联服务",
    description: "Plural alert heading when triggerBuild returns skippedServices",
  },
  "buildSources.triggerResultViewLogs": {
    message: "查看构建日志",
    description: "Primary action navigating from Trigger Build confirmation to Build Detail",
  },
  "buildSources.skippedReasonSuspended": {
    message: "已暂停",
    description: "Badge label when a service was skipped because it is suspended",
  },
  "buildSources.buildStatusSucceeded": {
    message: "成功",
    description: "Build status badge for succeeded builds",
  },
  "buildSources.buildStatusFailed": {
    message: "失败",
    description: "Build status badge for failed builds",
  },
  "buildSources.buildStatusCanceled": {
    message: "已取消",
    description: "Build status badge for canceled builds",
  },
  "buildSources.buildStatusInProgress": {
    message: "构建中",
    description: "Build status badge for in-progress builds",
  },
  "buildSources.buildStatusCreated": {
    message: "排队中",
    description: "Build status badge for newly queued builds",
  },
  "buildSources.buildStatusUnknown": {
    message: "未知",
    description: "Fallback build status badge",
  },
  "buildSources.buildsEmptyTitle": {
    message: "暂无构建记录",
    description: "Empty state heading on the Builds tab",
  },
  "buildSources.buildsEmptyGitDescription": {
    message: "手动触发构建或推送到跟踪分支，即可生成首个构建产物。",
    description: "Empty state description on Builds tab for Git build sources",
  },
  "buildSources.buildsEmptyImageDescription": {
    message: "Docker 镜像构建源会在关联服务部署时记录镜像版本。",
    description: "Empty state description on Builds tab for image build sources",
  },
  "buildSources.buildsRefreshError": {
    message: "无法刷新构建列表，当前显示缓存结果。",
    description: "Inline error banner when polling builds fails over cached rows",
  },
  "buildSources.buildsLoadMore": {
    message: "加载更多构建",
    description: "Pagination button loading older builds",
  },
  "buildSources.colBuildId": {
    message: "构建 ID",
    description: "Table header for build ID column",
  },
  "buildSources.colStatus": {
    message: "状态",
    description: "Table header for build status column",
  },
  "buildSources.colArtifact": {
    message: "提交 / 镜像",
    description: "Table header for commit SHA or image digest column",
  },
  "buildSources.colDuration": {
    message: "耗时",
    description: "Table header for build duration column",
  },
  "buildSources.colDeploys": {
    message: "部署",
    description: "Table header for fan-out deploys count column",
  },
  "buildSources.colStarted": {
    message: "开始时间",
    description: "Table header for build start timestamp column",
  },
  "buildSources.buildDeploysCount_other": {
    message: "{count} 次部署",
    description: "Plural badge count of fan-out deploys on a build",
  },
  "buildSources.buildSkippedCount_other": {
    message: "跳过 {count} 个",
    description: "Plural badge count of skipped services on a build",
  },
  "buildSources.backToBuilds": {
    message: "返回构建列表",
    description: "Back link from Build Detail to the Builds tab",
  },
  "buildSources.copyBuildId": {
    message: "复制构建 ID",
    description: "Accessible label for copying a bld- ID",
  },
  "buildSources.buildDetailSubtitle": {
    message: "{name} 的构建",
    description: "Subtitle under the bld- ID on Build Detail",
  },
  "buildSources.buildDetailRefreshError": {
    message: "无法刷新构建详情，当前显示上次加载的状态。",
    description: "Inline error banner when build detail polling fails",
  },
  "buildSources.factStarted": {
    message: "开始时间",
    description: "Fact label for build start timestamp",
  },
  "buildSources.factCommit": {
    message: "提交",
    description: "Fact label for build commit SHA",
  },
  "buildSources.factArtifact": {
    message: "产物",
    description: "Fact label for built image reference and digest",
  },
  "buildSources.fanoutTitle": {
    message: "关联服务部署",
    description: "Card heading listing fan-out deploys on Build Detail",
  },
  "buildSources.fanoutEmpty": {
    message: "此构建尚未触发任何服务部署。",
    description: "Empty message inside fan-out deploys card",
  },
  "buildSources.buildLogsSearchLabel": {
    message: "筛选构建日志",
    description: "Accessible label for search input in BuildLogPanel",
  },
  "buildSources.buildLogsEmptyTitle": {
    message: "暂无构建日志",
    description: "Empty state heading in BuildLogPanel",
  },
  "buildSources.buildLogsEmptyBody": {
    message: "构建开始输出后，日志将显示在这里。",
    description: "Empty state body in BuildLogPanel",
  },
  "buildSources.imageBuildLogsNotice": {
    message: "Docker 镜像构建源直接复用已有镜像，不会产生构建日志。",
    description: "Notice shown on Build Detail for image-backed build sources",
  },
  "buildSources.servicesTabTitle": {
    message: "关联服务",
    description: "Heading of the Linked Services tab",
  },
  "buildSources.servicesTabDescription": {
    message: "关联到此构建源的服务将部署其构建生成的产物。",
    description: "Description of the Linked Services tab",
  },
  "buildSources.linkExistingService": {
    message: "关联现有服务",
    description: "Button opening the Link Existing Service dialog",
  },
  "buildSources.servicesEmptyTitle": {
    message: "暂无关联服务",
    description: "Empty state heading on Linked Services tab",
  },
  "buildSources.servicesEmptyDescription": {
    message: "将现有的 Web 服务、私有服务、后台工作进程或定时任务关联到此构建源以共享构建。",
    description: "Empty state description on Linked Services tab",
  },
  "buildSources.autoDeployOn": {
    message: "自动部署已开启",
    description: "Badge indicating a linked service auto-deploys new builds",
  },
  "buildSources.autoDeployOff": {
    message: "自动部署已关闭",
    description: "Badge indicating a linked service requires manual deploys",
  },
  "buildSources.changeOrUnlinkSource": {
    message: "更改来源",
    description: "Link to a service's settings page to switch or unlink its source",
  },
  "buildSources.linkDialogTitle": {
    message: "关联现有服务",
    description: "Heading of the Link Existing Service dialog",
  },
  "buildSources.linkDialogDescription": {
    message: "将工作区中的现有服务关联到“{name}”。关联操作仅保存配置，不会立即触发部署。",
    description: "Description of the Link Existing Service dialog",
  },
  "buildSources.linkDialogNoEligible": {
    message: "此工作区中所有符合条件的服务均已关联，或尚无可用服务。",
    description: "Message when no linkable unlinked services exist",
  },
  "buildSources.linkDialogServiceLabel": {
    message: "服务",
    description: "Select label in Link Existing Service dialog",
  },
  "buildSources.linkDialogServicePlaceholder": {
    message: "选择一个服务…",
    description: "Select placeholder in Link Existing Service dialog",
  },
  "buildSources.linkDialogRegionMismatch": {
    message: "区域不匹配：此 Git 构建源位于 {sourceRegion}，而所选服务位于 {serviceRegion}。",
    description: "Inline error when linking a Git build source to a service in another region",
  },
  "buildSources.linkDialogStartCommandHint": {
    message: "原生运行时构建源必填，用于指定服务的启动命令。",
    description: "Help text for startCommand input in Link Existing Service dialog",
  },
  "buildSources.linkDialogSubmit": {
    message: "关联服务",
    description: "Submit button label in Link Existing Service dialog",
  },
  "buildSources.envImageUnsupportedTitle": {
    message: "Docker 镜像构建源不支持构建时环境配置",
    description: "Fallback heading if an image build source visits /environment",
  },
  "buildSources.envImageUnsupportedBody": {
    message: "只有基于 Git 的构建源会在构建时使用环境变量和机密文件。",
    description: "Fallback body if an image build source visits /environment",
  },
  "buildSources.envSaveOnlyNote": {
    message: "更改会直接保存而不会触发构建，并在下次构建时生效。",
    description: "Top note on Build-time Environment tab",
  },
  "buildSources.envVarsDescription": {
    message: "在此构建源执行构建时可用，更改将在下次构建时生效。",
    description: "Environment Variables card description on Build-time Environment tab",
  },
  "buildSources.secretFilesDescription": {
    message: "在此构建源执行构建时挂载，更改将在下次构建时生效。",
    description: "Secret Files card description on Build-time Environment tab",
  },
  "buildSources.envGroupsPanelDescription": {
    message: "关联到构建时的工作区级环境组。更改将在下次构建时生效，不会触发部署。",
    description: "Environment Groups card description on Build-time Environment tab",
  },
  "buildSources.envGroupsNoneLinked": {
    message: "此构建源尚未关联任何环境组。",
    description: "Empty state when no env groups are linked to a build source",
  },
  "buildSources.settingsIdentityTitle": {
    message: "标识与范围",
    description: "Heading of read-only identity card on Settings tab",
  },
  "buildSources.settingsIdentityDescription": {
    message: "此构建源的不可变标识与所属范围。",
    description: "Description of read-only identity card on Settings tab",
  },
  "buildSources.settingsFieldId": {
    message: "构建源 ID",
    description: "Label for read-only bds- ID field on Settings tab",
  },
  "buildSources.settingsConfigTitle": {
    message: "构建配置",
    description: "Heading of editable settings card on Settings tab",
  },
  "buildSources.settingsNoBuildNote": {
    message: "保存设置只会更新此构建源，不会自动触发构建。",
    description: "Inline note explaining Settings edits do not rebuild",
  },
  "buildSources.settingsFieldRepoUrl": {
    message: "仓库 URL",
    description: "Input label for Git repository URL on Settings tab",
  },
  "buildSources.settingsInvalidRepoUrl": {
    message: "请输入有效的 Git 仓库 URL。",
    description: "Validation error for malformed repo URL on Settings tab",
  },
  "buildSources.settingsInvalidImageUrl": {
    message: "请输入有效的 Docker 镜像地址。",
    description: "Validation error for malformed image URL on Settings tab",
  },
  "buildSources.settingsSaveButton": {
    message: "保存更改",
    description: "Submit button label on Settings tab",
  },
  "buildSources.settingsSaving": {
    message: "正在保存…",
    description: "Submit button label while updateBuildSource is in flight",
  },
  "buildSources.dangerZoneTitle": {
    message: "危险区域",
    description: "Heading of Delete Build Source card on Settings tab",
  },
  "buildSources.dangerZoneDescription": {
    message: "永久删除此构建源及其构建时配置。此操作无法撤销。",
    description: "Description of Delete Build Source card",
  },
  "buildSources.deleteBlockedTitle_other": {
    message: "请先取消关联 {count} 个服务再删除",
    description: "Plural alert heading blocking delete when services remain linked",
  },
  "buildSources.deleteBlockedDescription": {
    message: "当仍有服务引用此构建源时无法删除。请先将每个关联服务切换到其他来源：",
    description: "Explanation under delete-blocked alert",
  },
  "buildSources.deleteButton": {
    message: "删除构建源",
    description: "Destructive button opening the delete confirmation dialog",
  },
  "buildSources.deleteConfirmTitle": {
    message: "删除“{name}”？",
    description: "Title of Delete Build Source confirmation dialog",
  },
  "buildSources.deleteConfirmBody": {
    message: "此操作将永久删除该构建源及其构建时环境，构建记录和相关配置均无法恢复。",
    description: "Body of Delete Build Source confirmation dialog",
  },
  "buildSources.deleteConfirmPrompt": {
    message: "请在下方输入 {phrase} 以确认。",
    description: "Sudo command prompt inside Delete Build Source confirmation dialog",
  },
  "buildSources.deleteConfirmButton": {
    message: "删除构建源",
    description: "Confirm action label inside Delete Build Source confirmation dialog",
  },
};

export default zhBuildSources;
