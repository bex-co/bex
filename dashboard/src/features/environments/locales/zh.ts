import type { TranslationEntry } from "@/i18n";

const zhEnvironments: Record<string, TranslationEntry> = {
  "environments.heading": {
    message: "环境",
    description: "Section heading above a project page's environments list",
  },
  "environments.newButton": {
    message: "新建环境",
    description: "Button that opens the new-environment dialog",
  },
  "environments.emptyBody": {
    message: "暂无环境。创建一个环境（如预发布或生产）以分组此项目的资源。",
    description: "Empty state shown when a project has no environments",
  },
  "environments.errorBody": {
    message: "加载环境时出错，请稍后重试。",
    description: "Error state shown when the environments query fails",
  },
  "environments.resourceCount": {
    message: "{count} 项资源",
    description:
      "Count of visible services+databases+key-value instances assigned to an environment, next to its name",
  },
  "environments.manageButton": {
    message: "管理资源",
    description:
      "Button on an environment card that opens the manage-resources dialog",
  },
  "environments.moreActions": {
    message: "更多操作",
    description:
      "Accessible label for an environment card's overflow (•••) menu",
  },
  "environments.renameAction": {
    message: "重命名",
    description: "Environment overflow-menu item that opens the rename dialog",
  },
  "environments.deleteAction": {
    message: "删除",
    description:
      "Environment overflow-menu item that opens the delete confirmation, and the confirm button",
  },
  "environments.cardEmpty": {
    message: "此环境暂无资源。",
    description:
      "Shown inside an environment card when it has no assigned resources",
  },
  "environments.createTitle": {
    message: "新建环境",
    description: "New-environment dialog title",
  },
  "environments.createDescription": {
    message: "以名称（如预发布或生产）分组此项目的部分资源。",
    description: "New-environment dialog description",
  },
  "environments.fieldName": {
    message: "名称",
    description: "New-environment dialog name field label",
  },
  "environments.fieldNamePlaceholder": {
    message: "staging",
    description: "New-environment dialog name field placeholder",
  },
  "environments.cancel": {
    message: "取消",
    description: "Cancel button shared by the environment dialogs",
  },
  "environments.createSubmit": {
    message: "创建环境",
    description: "New-environment dialog submit button",
  },
  "environments.createSuccess": {
    message: "已创建环境“{name}”。",
    description: "Toast shown after an environment is created",
  },
  "environments.createError": {
    message: "创建环境“{name}”失败。",
    description: "Toast shown when creating an environment fails",
  },
  "environments.renameTitle": {
    message: "重命名环境",
    description: "Rename-environment dialog title",
  },
  "environments.renameSubmit": {
    message: "保存",
    description: "Rename-environment dialog submit button",
  },
  "environments.renameSuccess": {
    message: "环境已重命名为“{name}”。",
    description: "Toast shown after an environment is renamed",
  },
  "environments.renameError": {
    message: "将环境重命名为“{name}”失败。",
    description: "Toast shown when renaming an environment fails",
  },
  "environments.deleteConfirmTitle": {
    message: "删除环境“{name}”？",
    description: "Delete-environment confirmation dialog title",
  },
  "environments.deleteConfirmBody": {
    message:
      "其服务、数据库和键值存储仍保留在项目中并继续运行，仅会移除此环境标签。此操作无法撤销。",
    description: "Delete-environment confirmation dialog body",
  },
  "environments.deleteSuccess": {
    message: "已删除环境“{name}”。",
    description: "Toast shown after an environment is deleted",
  },
  "environments.deleteError": {
    message: "删除环境“{name}”失败。",
    description: "Toast shown when deleting an environment fails",
  },
  "environments.manageTitle": {
    message: "管理“{name}”中的资源",
    description: "Manage-resources dialog title",
  },
  "environments.manageDescription": {
    message: "勾选属于此环境的资源。分配资源时也会将其加入此项目。",
    description: "Manage-resources dialog description",
  },
  "environments.tabServices": {
    message: "服务",
    description: "Manage-resources dialog tab label for the services checklist",
  },
  "environments.tabDatabases": {
    message: "数据库",
    description:
      "Manage-resources dialog tab label for the databases checklist",
  },
  "environments.tabKeyValues": {
    message: "键值存储",
    description:
      "Manage-resources dialog tab label for the key-value checklist",
  },
  "environments.manageNoServices": {
    message: "此工作区暂无可分配的服务。",
    description:
      "Manage-resources dialog empty state when the workspace has no services",
  },
  "environments.manageNoDatabases": {
    message: "此工作区暂无可分配的数据库。",
    description:
      "Manage-resources dialog empty state when the workspace has no databases",
  },
  "environments.manageNoKeyValues": {
    message: "此工作区暂无可分配的键值存储实例。",
    description:
      "Manage-resources dialog empty state when the workspace has no key-value instances",
  },
  "environments.manageSubmit": {
    message: "保存",
    description: "Manage-resources dialog submit button",
  },
  "environments.assignSuccess": {
    message: "已更新“{name}”的服务。",
    description: "Toast shown after an environment's services are updated",
  },
  "environments.assignError": {
    message: "更新“{name}”的服务失败。",
    description: "Toast shown when updating an environment's services fails",
  },
  "environments.assignDatabasesSuccess": {
    message: "已更新“{name}”的数据库。",
    description: "Toast shown after an environment's databases are updated",
  },
  "environments.assignDatabasesError": {
    message: "更新“{name}”的数据库失败。",
    description: "Toast shown when updating an environment's databases fails",
  },
  "environments.assignKeyValuesSuccess": {
    message: "已更新“{name}”的键值存储实例。",
    description:
      "Toast shown after an environment's key-value instances are updated",
  },
  "environments.assignKeyValuesError": {
    message: "更新“{name}”的键值存储实例失败。",
    description:
      "Toast shown when updating an environment's key-value instances fails",
  },
  "environments.tabEnvGroups": {
    message: "环境组",
    description: "Manage-resources dialog tab label for environment groups",
  },
  "environments.manageNoEnvGroups": {
    message: "此工作区暂无可分配的环境组。",
    description: "Environment-groups checklist empty state",
  },
  "environments.assignEnvGroupsSuccess": {
    message: "已更新“{name}”的环境组。",
    description: "Toast shown after environment-group membership is updated",
  },
  "environments.assignEnvGroupsError": {
    message: "更新“{name}”的环境组失败。",
    description:
      "Toast shown when environment-group membership fails to update",
  },
  "environments.settingsAction": {
    message: "所有设置",
    description: "Environment overflow-menu action opening ACL settings",
  },
  "environments.settingsTitle": {
    message: "“{name}”的设置",
    description: "Environment ACL settings dialog title",
  },
  "environments.settingsDescription": {
    message: "管理权限、私有网络隔离和入站 IP 规则。",
    description: "Environment ACL settings dialog description",
  },
  "environments.protectedBadge": {
    message: "受保护",
    description: "Badge shown beside a protected environment name",
  },
  "environments.protectedLabel": {
    message: "受保护环境",
    description: "Protected-status setting label",
  },
  "environments.protectedHint": {
    message: "对破坏性服务操作要求明确输入 sudo 确认短语。",
    description: "Protected-status setting explanation",
  },
  "environments.isolationLabel": {
    message: "阻止跨环境连接",
    description: "Private-network isolation setting label",
  },
  "environments.isolationHint": {
    message: "阻止私有网络流量跨越此环境边界。",
    description: "Private-network isolation setting explanation",
  },
  "environments.ipAllowListLabel": {
    message: "入站 IP 限制",
    description: "Environment inbound-IP allowlist heading",
  },
  "environments.ipAllowListHint": {
    message: "只有列出的 CIDR 网段才能访问该环境的公开服务、静态站点和数据存储。来源还必须通过每个资源自身的 IP 规则。",
    description: "Environment inbound-IP allowlist explanation",
  },
  "environments.ipAllowListOpen": {
    message: "没有规则 — 该环境的公开成员将拒绝所有流量。添加 0.0.0.0/0 和 ::/0 可允许全部来源。",
    description: "Empty environment inbound-IP allowlist state",
  },
  "environments.ipAllowListRemove": {
    message: "移除 {cidr}",
    description: "Accessible label for an inbound-IP rule remove button",
  },
  "environments.ipAllowListAdd": {
    message: "添加来源",
    description: "Button adding an inbound-IP CIDR rule",
  },
  "environments.settingsSave": {
    message: "保存",
    description: "Environment ACL settings save button",
  },
  "environments.aclSaveSuccess": {
    message: "已保存“{name}”的设置。",
    description: "Toast shown after an Environment ACL is saved",
  },
  "environments.aclSaveError": {
    message: "保存“{name}”的设置失败。",
    description: "Toast shown when an Environment ACL save fails",
  },
  "environments.assignmentTitle": {
    message: "项目和环境",
    description: "Shared create-form Project/Environment selector heading",
  },
  "environments.assignmentProject": {
    message: "项目",
    description: "Accessible label for a create-form Project selector",
  },
  "environments.assignmentProjectNone": {
    message: "无项目",
    description: "Unassigned Project selector option",
  },
  "environments.assignmentEnvironment": {
    message: "环境",
    description: "Accessible label for a create-form Environment selector",
  },
  "environments.assignmentEnvironmentNone": {
    message: "无环境",
    description: "Unassigned Environment selector option",
  },
  "environments.assignmentHint": {
    message: "可选。选择环境时也会将资源加入其所属项目。",
    description: "Shared create-form Environment-assignment hint",
  },
};

export default zhEnvironments;
