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
  "environments.deleteSuccess": {
    message: "已成功删除环境 {name}。",
    description:
      "Toast shown after an environment is deleted (Render's words, w1/m683)",
  },
  "environments.deleteRefreshFailed": {
    message: "环境“{name}”已删除，但列表刷新失败。请重试或重新加载以更新。",
    description:
      "Warning after the environment was deleted but re-reading the list failed; deletion must not be retried or reported as failed (w4/290)",
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
    message: "Key Value",
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
    message: "此工作区暂无可分配的 Key Value 实例。",
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
    message: "已更新“{name}”的 Key Value 实例。",
    description:
      "Toast shown after an environment's key-value instances are updated",
  },
  "environments.assignKeyValuesRefreshFailed": {
    message:
      "已更新“{name}”的 Key Value 实例，但列表刷新失败。请重试或重新加载以查看。",
    description:
      "Warning after the key-value membership was saved but re-reading the environments list failed; the write must not be retried or reported as failed (w4/263)",
  },
  "environments.assignKeyValuesError": {
    message: "更新“{name}”的 Key Value 实例失败。",
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
  "environments.tabSandboxGroups": {
    message: "沙箱组",
    description:
      "Manage-resources dialog tab label for the workspace's sandbox groups",
  },
  "environments.manageNoSandboxGroups": {
    message: "此工作区暂无可分配的沙箱组。",
    description: "Sandbox-groups checklist empty state",
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
  "environments.protectedBadge": {
    message: "受保护",
    description: "Badge shown beside a protected environment name",
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
  "environments.ipAllowListInvalid": {
    message: "请输入唯一且有效的 IPv4 或 IPv6 CIDR。",
    description:
      "Inline validation when an environment allow list block is not a unique, valid CIDR",
  },
  "environments.ipAllowListAdd": {
    message: "添加来源",
    description: "Button adding an inbound-IP CIDR rule",
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
    message: "项目",
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
  "environments.protectedPlacementAdminOnly": {
    message:
      "只有管理员可以在受保护的环境中创建资源，或将资源移入或移出该环境。",
    description:
      "Why a create or move control that targets (or leaves) a protected environment is disabled for a non-admin, and the message for the same server refusal (w3/m281)",
  },
  "environments.protectedOptionAdminOnly": {
    message: "{environment}（仅限管理员）",
    description:
      "A protected environment in a create or move picker that a non-admin cannot choose ({environment} = its name)",
  },
  "environments.assignmentDescription": {
    message: "创建后将其加入一个项目和环境。",
    description: "Shared create-form Project row description",
  },
  "environments.assignmentNoEnvironments": {
    message: "此项目还没有环境。请先在项目页面创建一个环境。",
    description:
      "Shared create-form note when the chosen Project has no Environment to add the resource to",
  },
  "environments.assignmentPickEnvironment": {
    message:
      "选择一个环境才能将资源加入此项目；若选择“无环境”，资源将创建在所有项目之外。",
    description:
      "Shared create-form note when a Project is chosen but No environment is: only an Environment adds the resource to the Project",
  },
  "environments.assignmentEnvironmentPickProject": {
    message: "请先选择项目",
    description:
      "Shared create-form Environment select while no Project is chosen (it is disabled until one is)",
  },
  "environments.protectedDescription": {
    message: "受保护的环境",
    description:
      "Line under a protected environment's section heading on the project page (Render's words, w1/m682)",
  },
  "environments.emptyTitle": {
    message: "{name} 为空",
    description:
      "Heading of an environment section with no services, Postgres or Key Value; {name} is the environment (Render's words, w1/m682)",
  },
  "environments.emptyBodyCreate": {
    message: "新建一个服务，开始使用你的环境。",
    description:
      "An empty environment's sentence when the workspace has nothing to move into it (Render's words, w1/m682)",
  },
  "environments.emptyBodyCreateOrMove": {
    message: "新建一个服务，或移入一个已有服务，开始使用你的环境。",
    description:
      "An empty environment's sentence when the workspace has resources that could move into it (Render's words, w1/m682)",
  },
  "environments.createNewService": {
    message: "新建服务",
    description:
      "An empty environment's button to the create flow with the project and environment chosen (Render's words, w1/m682)",
  },
  "environments.moveExistingServices": {
    message: "移入已有服务",
    description:
      "An empty environment's button that moves existing resources into it (Render's words, w1/m682)",
  },
  "environments.noEnvGroups": {
    message: "此环境没有环境组。",
    description:
      "An environment section's Env Groups view when the environment has none (Render's words, w1/m682)",
  },
  "environments.unassignedDescription": {
    message: "此项目中不属于任何环境的资源。",
    description:
      "Line under the project page's Unassigned section, which lists the project's resources in no environment (a bex section; Render requires an environment)",
  },
  "environments.settingsPageTitle": {
    message: "环境设置",
    description:
      "Heading of an environment's settings page, /project/<id>/environment/<envId>/settings (Render's words, w1/m683)",
  },
  "environments.settingsNavigation": {
    message: "环境设置区块",
    description:
      "Accessible name of the environment settings page's section navigation (w1/m683)",
  },
  "environments.settingsEdit": {
    message: "编辑",
    description:
      "Button on an environment settings card that makes its field editable (Render's words, w1/m683)",
  },
  "environments.settingsCardSave": {
    message: "保存",
    description:
      "Button that saves an environment settings card's change (Render's words, w1/m683)",
  },
  "environments.settingsUpdateError": {
    message: "更新环境时出错，请重试。",
    description:
      "Inline error on an environment settings card when the save failed for a reason the server did not name (Render's words, w1/m683)",
  },
  "environments.nameCardTitle": {
    message: "名称",
    description:
      "Title of the environment settings page's Name card, and its navigation entry (Render's words, w1/m683)",
  },
  "environments.nameCardDescription": {
    message: "环境的唯一名称",
    description:
      "Description of the environment settings page's Name card (Render's words, w1/m683)",
  },
  "environments.nameFieldLabel": {
    message: "环境名称",
    description:
      "Accessible label of the Name card's field (Render's words, w1/m683)",
  },
  "environments.nameRequired": {
    message: "环境名称为必填项",
    description:
      "Error under the Name card's field when it is saved empty (Render's words, w1/m683)",
  },
  "environments.nameInUse": {
    message: "此项目中的另一个环境已使用该名称",
    description:
      "Error under the Name card's field when another environment of the project has the name (Render's words, w1/m683)",
  },
  "environments.permissionsCardTitle": {
    message: "权限",
    description:
      "Title of the environment settings page's Permissions card, and its navigation entry (Render's words, w1/m683)",
  },
  "environments.permissionsDefaultBody": {
    message:
      "默认情况下，所有团队成员都可以删除和暂停资源、移动服务以及管理数据库网络。",
    description:
      "First paragraph of the Permissions card (Render's words, w1/m683)",
  },
  "environments.permissionsProtectedBody": {
    message:
      "如果标记为{protectedEnvironment}，则只有管理员可以删除和暂停资源、移动服务以及管理数据库网络。",
    description:
      "Second paragraph of the Permissions card; {protectedEnvironment} is a link whose text is environments.permissionsProtectedLink (Render's words, w1/m683)",
  },
  "environments.permissionsProtectedLink": {
    message: "受保护环境",
    description:
      "Link text inside environments.permissionsProtectedBody, to the projects and environments docs (Render's words, w1/m683)",
  },
  "environments.permissionsLegend": {
    message: "环境权限",
    description:
      "Accessible name of the Permissions card's Default / Protected choice (Render's words, w1/m683)",
  },
  "environments.permissionsDefault": {
    message: "默认",
    description:
      "The Permissions card's option for an unprotected environment (Render's words, w1/m683)",
  },
  "environments.permissionsDefaultHint": {
    message: "适用于预发布和开发",
    description:
      "Line under the Permissions card's Default option (Render's words, w1/m683)",
  },
  "environments.permissionsProtected": {
    message: "受保护",
    description:
      "The Permissions card's option for a protected environment (Render's words, w1/m683)",
  },
  "environments.permissionsProtectedHint": {
    message: "最适合生产环境",
    description:
      "Line under the Permissions card's Protected option (Render's words, w1/m683)",
  },
  "environments.removeRestrictionsTitle": {
    message: "解除限制",
    description:
      "Title of the dialog that confirms unprotecting an environment, or allowing its cross-environment connections again (Render's words, w1/m683)",
  },
  "environments.removeProtectionBody": {
    message:
      "解除 {name} 环境的限制后，所有团队成员都将可以执行破坏性操作。确定要这样做吗？",
    description:
      "Body of the dialog that confirms unprotecting an environment; {name} is the environment (Render's words, w1/m683)",
  },
  "environments.removeRestrictionsConfirm": {
    message: "是，解除限制",
    description:
      "Confirm button that unprotects an environment, or allows its cross-environment connections again (Render's words, w1/m683)",
  },
  "environments.connectionsNavLabel": {
    message: "连接",
    description:
      "Navigation entry of the environment settings page's Cross-Environment Connections card (Render's words, w1/m683)",
  },
  "environments.connectionsCardTitle": {
    message: "跨环境连接",
    description:
      "Title of the environment settings page's connections card (Render's words, w1/m683)",
  },
  "environments.connectionsBody": {
    message: "阻止私有网络流量跨越环境边界，同时允许此环境内服务之间的流量。",
    description:
      "First paragraph of the Cross-Environment Connections card (Render's words, w1/m683)",
  },
  "environments.connectionsBodyOutside": {
    message: "此环境中的服务将无法与环境之外的 bex 服务通信。",
    description:
      "Second paragraph of the Cross-Environment Connections card (Render's words, with bex for Render, w1/m683)",
  },
  "environments.connectionsLegend": {
    message: "跨环境连接",
    description:
      "Accessible name of the Cross-Environment Connections card's Allowed / Blocked choice (w1/m683)",
  },
  "environments.connectionsAllowed": {
    message: "允许",
    description:
      "The Cross-Environment Connections card's option for an environment whose private traffic may cross its boundary (Render's words, w1/m683)",
  },
  "environments.connectionsAllowedHint": {
    message: "新环境的默认设置",
    description:
      "Line under the Cross-Environment Connections card's Allowed option (Render's words, w1/m683)",
  },
  "environments.connectionsBlocked": {
    message: "阻止",
    description:
      "The Cross-Environment Connections card's option for an isolated environment (Render's words, w1/m683)",
  },
  "environments.connectionsBlockedHint": {
    message: "推荐用于生产环境",
    description:
      "Line under the Cross-Environment Connections card's Blocked option (Render's words, w1/m683)",
  },
  "environments.connectionsPaywall": {
    message: "升级到 {plan} 套餐即可阻止跨环境连接。",
    description:
      "Notice on the Cross-Environment Connections card below Pro; {plan} is the Pro plan's name (Render's words, w1/m683)",
  },
  "environments.upgrade": {
    message: "升级",
    description:
      "Button or link that opens Render's plan page from an environment's plan gate (Render's words, w1/m683, w3/m708)",
  },
  "environments.isolateTitle": {
    message: "限制连接",
    description:
      "Title of the dialog that confirms blocking an environment's cross-environment connections, and its confirm button (Render's words, w1/m683)",
  },
  "environments.isolateDescription": {
    message: "确认要隔离 {environment} 的连接",
    description:
      "Description of the dialog that confirms blocking or allowing an environment's cross-environment connections; {environment} is <project> / <environment> (Render's words, w1/m683)",
  },
  "environments.isolateRestrictBody": {
    message:
      "阻止此环境与其外部的 bex 服务之间的私有网络流量。这可能会中断依赖跨边界流量的服务。",
    description:
      "First paragraph of the dialog that confirms blocking cross-environment connections (Render's words, with bex for Render, w1/m683)",
  },
  "environments.isolateRestrictRestart": {
    message:
      "已打开的现有网络连接不受影响。重启此环境中的服务，以确保关闭所有现有连接。",
    description:
      "Second paragraph of the dialog that confirms blocking cross-environment connections (Render's words, w1/m683)",
  },
  "environments.isolateRemoveBody": {
    message: "此环境与其外部的 bex 服务之间的私有网络流量将不再被阻止。",
    description:
      "Body of the dialog that confirms allowing cross-environment connections again (Render's words, with bex for Render, w1/m683)",
  },
  "environments.isolateQuestion": {
    message: "确定要这样做吗？",
    description:
      "Last line of the dialog that confirms blocking or allowing cross-environment connections (Render's words, w1/m683)",
  },
  "environments.isolationBlockedToast": {
    message: "已阻止 {name} 的跨环境连接。",
    description:
      "Toast after the section menu's toggle blocks an environment's cross-environment connections (Render's words, w1/m683)",
  },
  "environments.isolationAllowedToast": {
    message: "已允许 {name} 的跨环境连接。",
    description:
      "Toast after the section menu's toggle allows an environment's cross-environment connections again (Render's words, w1/m683)",
  },
  "environments.menuBlockConnections": {
    message: "阻止跨环境连接",
    description:
      "The environment section's ••• menu toggle that isolates the environment (Render's words, w1/m683)",
  },
  "environments.menuBlockConnectionsHint": {
    message: "阻止私有网络流量跨越环境边界。",
    description:
      "Line under the ••• menu's Block cross-environment connections toggle (Render's words, w1/m683)",
  },
  "environments.menuUpgradeToUnlock": {
    message: "{upgrade}即可解锁此功能",
    description:
      "The ••• menu's plan gate on Block cross-environment connections below Pro; {upgrade} is a link whose text is environments.upgrade (Render's words, w1/m683)",
  },
  "environments.envGroupsCardTitle": {
    message: "环境组",
    description:
      "Title of the environment settings page's Environment Groups card, and its navigation entry (Render's words, w1/m683)",
  },
  "environments.envGroupsNew": {
    message: "新建环境组",
    description:
      "Button on the Environment Groups card that creates an env group in the environment (Render's words, w1/m683)",
  },
  "environments.envGroupsSearch": {
    message: "搜索环境组",
    description:
      "Placeholder and accessible name of the Environment Groups card's search (Render's words, w1/m683)",
  },
  "environments.envGroupsNone": {
    message: "未找到环境组",
    description:
      "The Environment Groups card when the environment has no env groups (Render's words, w1/m683)",
  },
  "environments.networkingCardTitle": {
    message: "网络",
    description:
      "Title of the environment settings page's Networking card (inbound IP rules), and its navigation entry (Render's words, w1/m683)",
  },
  "environments.ipAllowListMoveUp": {
    message: "上移 {cidr}",
    description:
      "Accessible name of an environment IP rule's move-up button; {cidr} is the rule's CIDR block",
  },
  "environments.ipAllowListMoveDown": {
    message: "下移 {cidr}",
    description:
      "Accessible name of an environment IP rule's move-down button; {cidr} is the rule's CIDR block",
  },
  "environments.ipAllowListAllowsAll": {
    message: "对所有源 IP 开放",
    description:
      "The Networking card below Scale when the environment's rules are the allow-all pair",
  },
  "environments.ipAllowListSaved": {
    message: "已保存 {name} 的入站 IP 规则。",
    description:
      "Toast after the Networking card saves an environment's inbound IP rules",
  },
  "environments.deleteCardTitle": {
    message: "删除环境",
    description:
      "Title of the environment settings page's delete card, and its navigation entry (Render's words, w1/m683)",
  },
  "environments.deleteCardDescription": {
    message: "此环境（包括其中的服务）将被永久删除。此操作无法撤销。",
    description:
      "Description of the Delete Environment card (Render's words, w1/m683)",
  },
  "environments.deleteLastEnvironment": {
    message:
      "项目至少需要一个环境，因此无法删除此环境。可以考虑改为删除{project}。",
    description:
      "Notice on the Delete Environment card for a project's only environment; {project} is a link whose text is environments.deleteLastEnvironmentProject (Render's words, w1/m683)",
  },
  "environments.deleteLastEnvironmentProject": {
    message: "项目",
    description:
      "Link text inside environments.deleteLastEnvironment, to the project's settings (Render's words, w1/m683)",
  },
  "environments.deleteButton": {
    message: "删除环境",
    description:
      "The Delete Environment card's button, the confirmation dialog's title and its confirm button (Render's words, w1/m683)",
  },
  "environments.deleteDialogBody": {
    message:
      "此环境及其所有服务将被永久删除。如需保留某些服务，请先将它们移出此环境。",
    description:
      "Body of the dialog that confirms deleting an environment (Render's words, w1/m683)",
  },
  "environments.deleteDialogEnvGroupsKept": {
    message:
      "其环境组会保留但变为未分配：不再属于此环境或项目，仍可在“环境组”中找到。",
    description:
      "Added to the delete dialog's body when the environment holds env groups, which bex keeps (w1/m683)",
  },
  "environments.deleteFailed": {
    message: "无法删除环境，请稍后重试。抱歉！",
    description:
      "Error in the delete dialog when the delete failed for a reason the server did not name (Render's words, w1/m683)",
  },
  "environments.deleteCardDenied": {
    message: "环境删除",
    description:
      "Title of the card that replaces Delete Environment for a member who may not delete the environment (Render's words, w1/m683)",
  },
  "environments.protectedDeleteAdminOnly": {
    message:
      "只有管理员可以删除受保护的环境，或启用了网络隔离或入站 IP 规则的环境。",
    description:
      "Why a non-Admin cannot delete an environment that is protected, isolated or has inbound IP rules (w1/m683)",
  },
};

export default zhEnvironments;
