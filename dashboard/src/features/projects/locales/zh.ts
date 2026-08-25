import type { TranslationEntry } from "@/i18n";

const zhProjects: Record<string, TranslationEntry> = {
  "projects.overviewTitle": {
    message: "概览",
    description: "Page title of the Overview page (workspace homepage)",
  },
  "projects.projectsHeading": {
    message: "项目",
    description: "Section heading above the Overview page's project grid",
  },
  "projects.ungroupedHeading": {
    message: "未分组资源",
    description:
      "Section heading above the Overview page's ungrouped resource table",
  },
  "projects.allResourcesHeading": {
    message: "所有资源",
    description:
      "Heading above a project page's full resource table, below the environments section",
  },
  "projects.filterTypeLabel": {
    message: "资源类型",
    description: "Accessible label for project environment type filters",
  },
  "projects.filterAll": {
    message: "全部",
    description: "All project environment resource types",
  },
  "projects.filterServices": {
    message: "服务",
    description: "Service resource filter",
  },
  "projects.filterDatabases": {
    message: "数据库",
    description: "Database resource filter",
  },
  "projects.filterKeyValues": {
    message: "键值存储",
    description: "Key-value resource filter",
  },
  "projects.filterEnvGroups": {
    message: "环境变量组",
    description: "Environment-group resource filter",
  },
  "projects.searchLabel": {
    message: "搜索 {environment} 中的资源",
    description: "Accessible project environment resource-search label",
  },
  "projects.searchPlaceholder": {
    message: "搜索 {environment} 中的资源",
    description: "Project environment resource-search placeholder",
  },
  "projects.noMatchesTitle": {
    message: "没有匹配的资源",
    description: "Project environment filtered no-results title",
  },
  "projects.noMatchesBody": {
    message: "请尝试其他搜索词或资源类型。",
    description: "Project environment filtered no-results body",
  },
  "projects.eyebrow": {
    message: "项目",
    description: "Small uppercase label above a project page's name heading",
  },
  "projects.cardEmpty": {
    message: "暂无资源",
    description: "Project card status line when the project has no resources",
  },
  "projects.cardHealthy": {
    message: "所有资源运行正常",
    description: "Project card status line when every resource is healthy",
  },
  "projects.cardInProgress": {
    message: "{count} 项资源正在进行中",
    description:
      "Project card status line when resources are provisioning or deploying without a failure",
  },
  "projects.cardUnhealthy": {
    message: "{count} 项资源需要处理",
    description:
      "Project card status line when one or more resources are unhealthy",
  },
  "projects.newProjectCard": {
    message: "创建新项目",
    description:
      'The dashed "+ create project" tile at the end of the project grid',
  },
  "projects.errorTitle": {
    message: "资源加载失败",
    description: "Error state heading on the unified Projects page",
  },
  "projects.errorBody": {
    message: "加载资源时出错，请稍后重试。",
    description: "Error state body on the unified Projects page",
  },
  "projects.emptyTitle": {
    message: "暂无资源",
    description: "Empty state heading on the unified Projects page",
  },
  "projects.emptyBody": {
    message: "创建一个服务、数据库或键值存储以开始使用。",
    description: "Empty state body on the unified Projects page",
  },
  "projects.newButton": {
    message: "新建",
    description: "Button that opens the new-resource menu",
  },
  "projects.newService": {
    message: "新建服务",
    description: "New-resource menu item that starts the service create wizard",
  },
  "projects.newDatabase": {
    message: "新建数据库",
    description: "New-resource menu item that opens the database create dialog",
  },
  "projects.newKeyValue": {
    message: "新建键值存储",
    description: "New-resource menu item that starts the key value create form",
  },
  "projects.colName": {
    message: "名称",
    description: "Merged resource table column header",
  },
  "projects.colType": {
    message: "类型",
    description: "Merged resource table column header",
  },
  "projects.colStatus": {
    message: "状态",
    description: "Merged resource table column header",
  },
  "projects.colCreated": {
    message: "创建时间",
    description: "Merged resource table column header",
  },
  "projects.colRuntime": {
    message: "运行时",
    description:
      "Authoritative runtime/product column in a Project resource table",
  },
  "projects.colRegion": {
    message: "区域",
    description:
      "Explicit installation placement column in a Project resource table",
  },
  "projects.colUpdated": {
    message: "更新时间",
    description:
      "Authoritative resource update-time column in a Project resource table",
  },
  "projects.colActions": {
    message: "操作",
    description: "Merged resource table column header (screen-reader only)",
  },
  "projects.typeService": {
    message: "服务",
    description: "Type badge for a service row in the merged resource table",
  },
  "projects.typeDatabase": {
    message: "数据库",
    description: "Type badge for a database row in the merged resource table",
  },
  "projects.typeKeyValue": {
    message: "键值存储",
    description: "Type badge for a key value row in the merged resource table",
  },
  "projects.typeEnvGroup": {
    message: "环境变量组",
    description: "Type badge for an environment-group row",
  },
  "projects.metadataUnavailable": {
    message: "不可用",
    description:
      "Accessible label for an unavailable runtime, region, status, or timestamp",
  },
  "projects.selectAllVisible": {
    message: "选择所有可见资源",
    description: "Accessible label for the Project table header checkbox",
  },
  "projects.selectResource": {
    message: "选择 {name}",
    description: "Accessible label for one Project resource row checkbox",
  },
  "projects.selectedCount": {
    message: "已选择：{count}",
    description: "Bulk action toolbar selection count",
  },
  "projects.moveTargetLabel": {
    message: "将所选资源移动到环境",
    description: "Accessible label for the bulk Move target selector",
  },
  "projects.moveTargetPlaceholder": {
    message: "选择环境",
    description: "Placeholder for the bulk Move target selector",
  },
  "projects.moveSelected": {
    message: "移动",
    description:
      "Bulk action that moves selected resources to another Environment",
  },
  "projects.newProjectButton": {
    message: "新建项目",
    description: "Button that opens the new-project dialog",
  },
  "projects.createTitle": {
    message: "新建项目",
    description: "New-project dialog title",
  },
  "projects.createDescription": {
    message: "将现有的服务、数据库和键值存储归类到一起。",
    description: "New-project dialog description",
  },
  "projects.fieldName": {
    message: "名称",
    description: "New-project dialog name field label",
  },
  "projects.fieldNamePlaceholder": {
    message: "my-project",
    description: "New-project dialog name field placeholder",
  },
  "projects.createCancel": {
    message: "取消",
    description: "New-project dialog cancel button",
  },
  "projects.createSubmit": {
    message: "创建项目",
    description: "New-project dialog submit button",
  },
  "projects.createSuccess": {
    message: "项目「{name}」已创建。",
    description: "Toast shown after a project is created",
  },
  "projects.createError": {
    message: "创建项目「{name}」失败。",
    description: "Toast shown when creating a project fails",
  },
  "projects.editNameButton": {
    message: "编辑",
    description:
      "Accessible label for the project page's inline pencil-edit button, and the Settings page's Project Name \"Edit\" button",
  },
  "projects.renameTitle": {
    message: "重命名项目",
    description: "Rename-project dialog title",
  },
  "projects.renameSubmit": {
    message: "保存",
    description: "Rename-project dialog submit button",
  },
  "projects.renameSuccess": {
    message: "项目已重命名为「{name}」。",
    description: "Toast shown after a project is renamed",
  },
  "projects.renameError": {
    message: "重命名项目为「{name}」失败。",
    description: "Toast shown when renaming a project fails",
  },
  "projects.settingsTitle": {
    message: "项目设置",
    description: "Project settings page heading",
  },
  "projects.nameCardTitle": {
    message: "项目名称",
    description: "Project settings page's name card title",
  },
  "projects.nameCardDescription": {
    message: "项目的唯一名称",
    description: "Project settings page's name card description",
  },
  "projects.deleteCardTitle": {
    message: "删除项目",
    description: "Project settings page's danger-zone card title",
  },
  "projects.deleteCardDescription": {
    message:
      "项目下的环境及其配置将一并删除。其服务、数据库和键值存储不会被删除，只会变为未分组状态。此操作无法撤销。",
    description: "Project settings page's danger-zone card description",
  },
  "projects.deleteCardButton": {
    message: "删除项目",
    description:
      "Project settings page's danger-zone card button, and the delete-confirmation dialog's confirm button",
  },
  "projects.deleteConfirmTitle": {
    message: "删除项目「{name}」？",
    description: "Delete-project confirmation dialog title",
  },
  "projects.deleteConfirmBody": {
    message: "项目下的环境及其配置将被删除。其服务、数据库和键值存储将变为未分组状态，不会被删除。",
    description: "Delete-project confirmation dialog body",
  },
  "projects.deleteSuccess": {
    message: "项目「{name}」已删除。",
    description: "Toast shown after a project is deleted",
  },
  "projects.deleteError": {
    message: "删除项目「{name}」失败。",
    description: "Toast shown when deleting a project fails",
  },
  "projects.moveToProject": {
    message: "移动到项目",
    description:
      "Row-actions submenu label for assigning a resource to a project",
  },
  "projects.removeFromProject": {
    message: "从项目中移除",
    description:
      "Row-actions submenu item that unassigns a resource from its project",
  },
  "projects.moveSuccess": {
    message: "「{name}」已移动到「{project}」。",
    description: "Toast shown after moving a resource to a project",
  },
  "projects.moveError": {
    message: "移动「{name}」失败。",
    description: "Toast shown when moving a resource to a project fails",
  },
  "projects.removeSuccess": {
    message: "「{name}」已从项目中移除。",
    description: "Toast shown after unassigning a resource from its project",
  },
  "projects.removeError": {
    message: "从项目中移除「{name}」失败。",
    description: "Toast shown when unassigning a resource fails",
  },
};

export default zhProjects;
