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
};

export default zhBuildSources;
