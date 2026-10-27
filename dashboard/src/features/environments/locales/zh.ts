import type { TranslationEntry } from "@/i18n";

const zhEnvironments: Record<string, TranslationEntry> = {
  "environments.heading": {
    message: "环境",
    description: "Section heading above a project page's environments list",
  },
  "environments.selectorLabel": {
    message: "所选环境",
    description:
      "Accessible label for the Project overview Environment selector",
  },
  "environments.unassignedOption": {
    message: "未分配",
    description: "Project resources that belong to no Environment",
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
  "environments.refreshErrorBody": {
    message: "无法刷新环境，正在显示上次加载的数据。",
    description:
      "Inline notice when a background refresh of environments fails while the previously loaded environments stay on screen",
  },
  "environments.resourceCount_other": {
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
  "environments.createRefreshFailed": {
    message: "环境“{name}”已创建，但列表刷新失败。请重试或重新加载以查看。",
    description:
      "Warning after the environment was created but re-reading the list failed; creation must not be retried or reported as failed (w4/231)",
  },
  "environments.createError": {
    message: "创建环境“{name}”失败。",
    description: "Toast shown when creating an environment fails",
  },
  "environments.renameFieldLabel": {
    message: "环境名称",
    description:
      'Accessible/visible label for the environment rename field. Deliberately more specific than environments.fieldName\'s bare "Name" (w4/134)',
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
  "environments.deleteRefreshFailed": {
    message: "环境“{name}”已删除，但列表刷新失败。请重试或重新加载以更新。",
    description:
      "Warning after the environment was deleted but re-reading the list failed; deletion must not be retried or reported as failed (w4/290)",
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
  "environments.manageResourceIn": {
    message: "位于 {environment}",
    description:
      "Under a Manage resources row: the other environment the resource is in now ({environment} = its name)",
  },
  "environments.markerProtected": {
    message: "受保护",
    description:
      "Marker: the environment the resource would leave is protected",
  },
  "environments.markerIsolated": {
    message: "已隔离",
    description:
      "Marker: the environment the resource would leave has network isolation",
  },
  "environments.markerAllowList": {
    message: "IP 允许列表",
    description:
      "Marker: the environment the resource would leave restricts inbound IPs",
  },
  "environments.manageMovesOut": {
    message: "将移出 {environment}：{resources}",
    description:
      "Before Save: the selected resources this save moves out of another environment ({resources} = comma-separated names)",
  },
  "environments.manageNeedsAdminToMove": {
    message: "只有管理员可以将其移出 {environment}。",
    description:
      "Why a Manage resources row is disabled: its current environment is admin-managed",
  },
  "environments.manageUnsaved": {
    message: "未保存的更改",
    description:
      "Accessible description of a Manage resources tab whose selection differs from the saved membership",
  },
  "environments.manageSubmit": {
    message: "保存",
    description: "Manage-resources dialog submit button",
  },
  "environments.assignSuccess": {
    message: "已更新“{name}”的服务。",
    description: "Toast shown after an environment's services are updated",
  },
  "environments.strandedMoveTitle": {
    message: "此次移动会留下已关联的环境组",
    description:
      "Confirmation title shown before a move that strands an environment-scoped group link (w4/m238)",
  },
  "environments.strandedMoveBody": {
    message:
      "以下环境组仍会关联到移动中的服务，其变量和文件会持续作用于该服务，直到取消关联或移动该组：",
    description:
      "Confirmation body: the stranded groups keep feeding the service after the move",
  },
  "environments.strandedMoveConfirm": {
    message: "仍要移动",
    description: "Confirm button on the stranded-groups move confirmation",
  },
  "environments.strandedScopeWorkspace": {
    message: "工作区",
    description:
      "Scope label for a stranded group that is scoped to the whole workspace",
  },
  "environments.strandedScopeUnknown": {
    message: "尚无法检查已关联的环境组——请等列表加载完成后重试。",
    description:
      "Toast shown when a move is attempted before the group/scope reads resolve",
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
  "environments.assignDatabasesRefreshFailed": {
    message: "已更新“{name}”的数据库，但列表刷新失败。请重试或重新加载以查看。",
    description:
      "Warning after the database membership was saved but re-reading the environments list failed; the write must not be retried or reported as failed (w4/325)",
  },
  "environments.assignKeyValuesSuccess": {
    message: "已更新“{name}”的键值存储实例。",
    description:
      "Toast shown after an environment's key-value instances are updated",
  },
  "environments.assignKeyValuesRefreshFailed": {
    message:
      "已更新“{name}”的键值存储实例，但列表刷新失败。请重试或重新加载以查看。",
    description:
      "Warning after the key-value membership was saved but re-reading the environments list failed; the write must not be retried or reported as failed (w4/263)",
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
  "environments.assignEnvGroupsPlacementUnread": {
    message: "无法确认“{name}”的环境组当前位置。请重试或重新加载后再移动。",
    description:
      "Warning after an environment-group move failed and re-reading the groups' placement failed too; its action re-reads, it never moves again (w4/m206)",
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
    message:
      "只有工作区管理员可以删除、暂停、恢复本环境的资源或为其开启维护模式。",
    description: "Protected-status setting explanation",
  },
  "environments.isolationLabel": {
    message: "阻止跨环境连接",
    description: "Private-network isolation setting label",
  },
  "environments.isolationProOnly": {
    message: "网络隔离需要 Pro 或更高套餐。如果之前已启用，可以在此关闭。",
    description: "Isolation setting explanation on a Hobby workspace (w8/m138)",
  },
  "environments.isolationHint": {
    message:
      "阻止私有网络流量跨越此环境边界，包括流向其中 Postgres 和 Key Value 实例的流量。它们的外部 URL 仍遵循各自的 IP 规则。",
    description: "Private-network isolation setting explanation",
  },
  "environments.ipAllowListLabel": {
    message: "入站 IP 限制",
    description: "Environment inbound-IP allowlist heading",
  },
  "environments.ipAllowListHint": {
    message:
      "只有列出的 CIDR 网段才能访问该环境的公开服务、静态站点和数据存储。来源还必须通过每个资源自身的 IP 规则。",
    description: "Environment inbound-IP allowlist explanation",
  },
  "environments.ipAllowListOpen": {
    message:
      "没有规则 — 该环境的公开成员将拒绝所有流量。添加 0.0.0.0/0 和 ::/0 可允许全部来源。",
    description: "Empty environment inbound-IP allowlist state",
  },
  "environments.ipAllowListRemove": {
    message: "移除 {cidr}",
    description: "Accessible label for an inbound-IP rule remove button",
  },
  "environments.ipAllowListRuleCIDR": {
    message: "规则 {number} 的 CIDR 网段",
    description: "Accessible label for an existing inbound-IP rule CIDR",
  },
  "environments.ipAllowListRuleDescription": {
    message: "规则 {number} 的描述",
    description: "Accessible label for an existing inbound-IP rule description",
  },
  "environments.ipAllowListNewCIDR": {
    message: "新 CIDR 网段",
    description: "Accessible label for a new inbound-IP rule CIDR",
  },
  "environments.ipAllowListNewDescription": {
    message: "新规则描述",
    description: "Accessible label for a new inbound-IP rule description",
  },
  "environments.ipAllowListDescriptionPlaceholder": {
    message: "描述（可选）",
    description: "Placeholder for an inbound-IP rule description",
  },
  "environments.ipAllowListScalePlanOnly": {
    message:
      "环境的入站 IP 规则需要 Scale 或更高套餐。已设置的规则继续生效，可以重置。",
    description:
      "Hint replacing the editable IP rules below the Scale plan (w8/m137)",
  },
  "environments.ipAllowListResetAllowAll": {
    message: "重置为允许全部",
    description:
      "Button resetting an environment's inbound IP rules to allow-all below the Scale plan (w8/m137)",
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
  "environments.resourceCountLoading": {
    message: "正在统计…",
    description:
      "Environment card header while its resource lists are still loading, instead of a count",
  },
  "environments.resourceCountUnknown": {
    message: "数量不可用",
    description:
      "Environment card header when a resource list has not answered, instead of a count",
  },
  "environments.resourcesUnavailable": {
    message: "部分资源加载失败，此环境的内容可能不完整或已过期。",
    description:
      "Inline alert on a Project environment card when a resource list failed to load; offers Try again",
  },
  "environments.resourcesLoading": {
    message: "正在加载资源…",
    description:
      "Environment card line while its resource lists are still loading, instead of the empty message",
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
  "environments.assignmentSelectedLoading": {
    message: "加载中…",
    description:
      "Label for a kept Project/Environment selection while its list is still loading",
  },
  "environments.assignmentSelectedUnavailable": {
    message: "已选择（名称不可用）",
    description:
      "Label for a kept Project/Environment selection whose list failed to load",
  },
  "environments.assignmentReadError": {
    message: "项目或环境加载失败。请重试；你已做的选择会保留。",
    description:
      "Inline alert when the create form's Project or Environment list fails to load; the selection is kept",
  },
  "environments.assignmentHint": {
    message:
      "创建资源时，选择项目只会筛选其环境列表。选择一个环境，资源才会加入该环境所属的项目。保持“无环境”会在所有项目之外创建资源。如果项目还没有环境，请先在项目页面创建一个。",
    description: "Shared create-form Environment-assignment hint",
  },
};

export default zhEnvironments;
