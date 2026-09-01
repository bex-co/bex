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
  "workspaces.switcherBilling": {
    message: "账单",
    description: "Switcher menu item linking to /billing",
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
  "workspaces.planHobbyBilling": {
    message: "$0/月",
    description:
      "Workspace plan card billing label (pricing.yaml hobby usdPerMonth)",
  },
  "workspaces.planHobbyDescription": {
    message: "适合刚起步的个人。",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planHobbyBulletMembers": {
    message: "1 名成员",
    description: "Hobby plan LimitsFor.MaxMembers bullet",
  },
  "workspaces.planHobbyBulletServices": {
    message: "最多 25 个服务",
    description: "Hobby plan LimitsFor.MaxServices bullet",
  },
  "workspaces.planHobbyBulletWorkspaces": {
    message: "每位用户最多 5 个 Hobby 工作区",
    description: "Hobby plan LimitsFor.MaxWorkspacesPerUser bullet",
  },
  "workspaces.planProName": {
    message: "Pro",
    description: "Workspace plan card name",
  },
  "workspaces.planProBilling": {
    message: "$17.50/月",
    description:
      "Workspace plan card billing label (pricing.yaml pro usdPerMonth)",
  },
  "workspaces.planProDescription": {
    message: "适合一起交付的小团队。",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planProBulletMembers": {
    message: "成员数量不限",
    description: "Pro plan unlimited MaxMembers bullet",
  },
  "workspaces.planProBulletServices": {
    message: "服务数量不限",
    description: "Pro plan unlimited MaxServices bullet",
  },
  "workspaces.planScaleName": {
    message: "Scale",
    description: "Workspace plan card name",
  },
  "workspaces.planScaleBilling": {
    message: "$349.30/月",
    description:
      "Workspace plan card billing label (pricing.yaml scale usdPerMonth)",
  },
  "workspaces.planScaleDescription": {
    message: "适合需要更多角色的成长团队。",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planScaleBulletMembers": {
    message: "成员数量不限",
    description: "Scale plan unlimited MaxMembers bullet",
  },
  "workspaces.planScaleBulletServices": {
    message: "服务数量不限",
    description: "Scale plan unlimited MaxServices bullet",
  },
  "workspaces.planScaleBulletRoles": {
    message: "额外角色（Contributor、Viewer、Billing）",
    description: "Scale plan AllowedRoles beyond Pro bullet",
  },
  "workspaces.planEnterpriseName": {
    message: "Enterprise",
    description: "Workspace plan card name",
  },
  "workspaces.planEnterpriseBilling": {
    message: "定制条款",
    description: "Workspace plan card billing label",
  },
  "workspaces.planEnterpriseDescription": {
    message: "定制额度与支持。",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planEnterpriseBulletLimits": {
    message: "定制额度",
    description: "Enterprise plan custom-limits bullet",
  },
  "workspaces.planEnterpriseBulletSupport": {
    message: "定制支持",
    description: "Enterprise plan custom-support bullet",
  },
  "workspaces.planSelect": {
    message: "选择套餐",
    description: "Unselected plan card action label",
  },
  "workspaces.planSelected": {
    message: "已选择此套餐",
    description: "Selected plan card action label",
  },
  "workspaces.planUsageBillingNote": {
    message: "服务和数据存储用量按资源层级另行计费。",
    description: "Billing note below the workspace plan picker",
  },
  "workspaces.createTitle": {
    message: "创建工作区",
    description: "/new/workspace page heading",
  },
  "workspaces.createDescription": {
    message: "选择工作区详情、账单联系人、套餐和付款方式。",
    description: "/new/workspace page subtitle",
  },
  "workspaces.detailsTitle": {
    message: "工作区详情",
    description: "/new/workspace details section heading",
  },
  "workspaces.billingEmail": {
    message: "账单邮箱",
    description: "/new/workspace billing email label",
  },
  "workspaces.billingEmailHelp": {
    message: "此工作区的收据和账单通知将发送到这里。",
    description: "Editable paid-plan billing email help",
  },
  "workspaces.billingEmailHobbyHelp": {
    message: "Hobby 工作区的账单邮箱必须是你的账户邮箱。",
    description: "Read-only Hobby billing email help",
  },
  "workspaces.billingEmailError": {
    message: "请输入有效的账单邮箱。",
    description: "Billing email validation error",
  },
  "workspaces.fieldSlug": {
    message: "工作区 slug",
    description: "/new/workspace slug field label",
  },
  "workspaces.fieldSlugHelp": {
    message: "用于 URL 和资源名称。仅限小写字母、数字和连字符，1–30 个字符。",
    description: "/new/workspace slug helper text",
  },
  "workspaces.paymentTitle": {
    message: "付款方式",
    description: "/new/workspace payment panel heading",
  },
  "workspaces.paymentDescription": {
    message: "每个工作区独立计费。此付款方式仅属于即将创建的工作区。",
    description: "/new/workspace workspace-specific payment copy",
  },
  "workspaces.paymentRequired": {
    message: "此工作区必须添加付款方式。",
    description: "Required payment policy copy",
  },
  "workspaces.paymentOptional": {
    message: "你可以现在添加付款方式，也可以暂时跳过。",
    description: "Optional payment policy copy",
  },
  "workspaces.paymentSelfHosted": {
    message: "此自托管安装未启用付款收集。",
    description: "Billing-off create-flow copy",
  },
  "workspaces.paymentAdd": {
    message: "添加付款方式",
    description: "Open Payment Element action",
  },
  "workspaces.paymentSave": {
    message: "保存付款方式",
    description: "Confirm SetupIntent action",
  },
  "workspaces.paymentAdded": {
    message: "已验证此工作区的付款方式。",
    description: "Successful payment setup state",
  },
  "workspaces.paymentError": {
    message: "无法验证该付款方式，请重试。",
    description: "Payment Element fallback error",
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
  "workspaces.settingsNavigation": {
    message: "设置区块",
    description:
      "Accessible label for the workspace settings section navigation",
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
  "workspaces.generalTitle": {
    message: "常规",
    description: "Title of the general card on the workspace settings page",
  },
};

export default zhWorkspaces;
