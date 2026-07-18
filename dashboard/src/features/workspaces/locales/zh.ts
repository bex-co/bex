import type { TranslationEntry } from "@/i18n";

const zhWorkspaces: Record<string, TranslationEntry> = {
  "workspaces.switcherEmpty": {
    message: "选择工作区",
    description: "Switcher trigger label before any workspace is selected",
  },
  "workspaces.switcherLabel": {
    message: "工作区",
    description: "Switcher dropdown label above the workspace list",
  },
  "workspaces.switcherSettings": {
    message: "工作区设置",
    description: "Switcher menu item linking to /workspace/settings",
  },
  "workspaces.switcherNew": {
    message: "+ 新建工作区",
    description: "Switcher menu item linking to /new/workspace",
  },
  "workspaces.newTitle": {
    message: "新建工作区",
    description: "New-workspace page document title",
  },
  "workspaces.planPickerLabel": {
    message: "套餐",
    description: "Accessible label for the plan-card radiogroup",
  },
  "workspaces.planHobbyName": {
    message: "Hobby",
    description: "Workspace plan card name",
  },
  "workspaces.planHobbyPrice": {
    message: "免费",
    description: "Workspace plan card price",
  },
  "workspaces.planHobbyDescription": {
    message: "1 名成员，最多 25 个服务，每位用户最多 5 个 Hobby 工作区。",
    description: "Workspace plan card description",
  },
  "workspaces.planProName": {
    message: "Pro",
    description: "Workspace plan card name",
  },
  "workspaces.planProPrice": {
    message: "$25/月",
    description: "Workspace plan card price",
  },
  "workspaces.planProDescription": {
    message: "成员和服务数量不限。",
    description: "Workspace plan card description",
  },
  "workspaces.planScaleName": {
    message: "Scale",
    description: "Workspace plan card name",
  },
  "workspaces.planScalePrice": {
    message: "$499/月",
    description: "Workspace plan card price",
  },
  "workspaces.planScaleDescription": {
    message: "成员和服务数量不限，支持更多角色。",
    description: "Workspace plan card description",
  },
  "workspaces.planEnterpriseName": {
    message: "Enterprise",
    description: "Workspace plan card name",
  },
  "workspaces.planEnterprisePrice": {
    message: "定制",
    description: "Workspace plan card price",
  },
  "workspaces.planEnterpriseDescription": {
    message: "定制额度与支持。",
    description: "Workspace plan card description",
  },
  "workspaces.createTitle": {
    message: "创建工作区",
    description: "/new/workspace card title",
  },
  "workspaces.createDescription": {
    message: "为它取个名字并选择一个套餐。",
    description: "/new/workspace card description",
  },
  "workspaces.fieldName": {
    message: "名称",
    description: "Workspace name field label (shared by create + settings)",
  },
  "workspaces.fieldNamePlaceholder": {
    message: "例如 acme-staging",
    description: "Workspace name field placeholder",
  },
  "workspaces.fieldNameError": {
    message: "仅限小写字母、数字和连字符，1-30 个字符，首尾不能是连字符。",
    description: "Workspace name validation error",
  },
  "workspaces.fieldPlan": {
    message: "套餐",
    description: "Workspace plan field label (create picker + settings badge)",
  },
  "workspaces.createErrorTitle": {
    message: "创建工作区失败",
    description: "/new/workspace inline error alert title",
  },
  "workspaces.createCancel": {
    message: "取消",
    description: "/new/workspace cancel button",
  },
  "workspaces.createSubmit": {
    message: "创建工作区",
    description: "/new/workspace submit button",
  },
  "workspaces.createSuccess": {
    message: "已创建 {name}",
    description: "Toast on a successful workspace create",
  },
  "workspaces.createError": {
    message: "无法创建工作区",
    description: "Fallback toast/inline message on a failed create",
  },
  "workspaces.settingsTitle": {
    message: "工作区设置",
    description: "Workspace settings page and card title",
  },
  "workspaces.settingsDescription": {
    message: "重命名此工作区，或查看其套餐与元数据。",
    description: "Workspace settings card description",
  },
  "workspaces.settingsEmpty": {
    message: "未选择工作区。",
    description: "Workspace settings page empty state",
  },
  "workspaces.renameSubmit": {
    message: "保存",
    description: "Workspace rename form submit button",
  },
  "workspaces.renameErrorTitle": {
    message: "重命名工作区失败",
    description: "Workspace rename inline error alert title",
  },
  "workspaces.renameSuccess": {
    message: "已重命名为 {name}",
    description: "Toast on a successful rename",
  },
  "workspaces.renameError": {
    message: "无法重命名工作区",
    description: "Fallback toast/inline message on a failed rename",
  },
  "workspaces.fieldId": {
    message: "工作区 ID",
    description: "Workspace settings metadata field label",
  },
  "workspaces.fieldCreatedAt": {
    message: "创建时间",
    description: "Workspace settings metadata field label",
  },
  "workspaces.dangerZoneTitle": {
    message: "危险区域",
    description: "Workspace settings delete section title",
  },
  "workspaces.dangerZoneDescription": {
    message:
      "此操作将删除该工作区的所有资源和数据。所有服务、数据存储和环境变量都将丢失，且无法撤销。",
    description: "Workspace settings delete section description",
  },
  "workspaces.deleteConfirmLabel": {
    message: "在下方输入 {phrase} 以确认。",
    description:
      "Body prompt naming the exact 'sudo delete workspace <name>' phrase (rendered bold by SudoCommandField)",
  },
  "workspaces.deleteErrorTitle": {
    message: "删除工作区失败",
    description: "Delete danger-zone inline error alert title",
  },
  "workspaces.deleteSubmit": {
    message: "删除工作区",
    description: "Delete danger-zone submit button",
  },
  "workspaces.deleteSuccess": {
    message: "已删除 {name}",
    description: "Toast on a successful delete",
  },
  "workspaces.deleteError": {
    message: "无法删除工作区",
    description: "Fallback toast/inline message on a failed delete",
  },
  "workspaces.changePlanTrigger": {
    message: "更改套餐",
    description:
      "Workspace settings plan-badge link opening the change-plan dialog",
  },
  "workspaces.changePlanTitle": {
    message: "更改套餐",
    description: "Change-plan dialog title",
  },
  "workspaces.changePlanDescription": {
    message: "为此工作区选择新套餐。无需支付步骤 —— 套餐立即生效。",
    description: "Change-plan dialog description",
  },
  "workspaces.changePlanCancel": {
    message: "取消",
    description: "Change-plan dialog cancel button",
  },
  "workspaces.changePlanSubmit": {
    message: "更改套餐",
    description: "Change-plan dialog submit button",
  },
  "workspaces.changePlanErrorTitle": {
    message: "更改套餐失败",
    description: "Change-plan dialog inline error alert title",
  },
  "workspaces.changePlanSuccess": {
    message: "已更改为 {plan} 套餐",
    description: "Toast on a successful plan change",
  },
  "workspaces.changePlanError": {
    message: "无法更改工作区套餐",
    description: "Fallback toast/inline message on a failed plan change",
  },
};

export default zhWorkspaces;
