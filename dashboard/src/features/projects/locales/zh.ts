import type { TranslationEntry } from "@/i18n";

const zhProjects: Record<string, TranslationEntry> = {
  "projects.groupLabel": {
    message: "项目",
    description: "Row group label shown above a project's resources",
  },
  "projects.ungroupedLabel": {
    message: "无项目",
    description: "Group label for resources not assigned to any project",
  },
  "projects.cardTitle": {
    message: "全部资源",
    description: "Title of the unified Projects page's resources card",
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
  "projects.projectActionsMenu": {
    message: "项目操作",
    description: "Accessible label for a project section's \"•••\" menu button",
  },
  "projects.actionRename": {
    message: "重命名",
    description: "Project actions menu item",
  },
  "projects.actionDelete": {
    message: "删除",
    description: "Project actions menu item",
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
  "projects.deleteConfirmTitle": {
    message: "删除项目「{name}」？",
    description: "Delete-project confirmation dialog title",
  },
  "projects.deleteConfirmBody": {
    message: "其服务、数据库和键值存储将变为未分组状态——不会被删除。",
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
    description: "Row-actions submenu label for assigning a resource to a project",
  },
  "projects.removeFromProject": {
    message: "从项目中移除",
    description: "Row-actions submenu item that unassigns a resource from its project",
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
