import type { TranslationEntry } from "@/i18n";

const zhWorkspaces: Record<string, TranslationEntry> = {
  "workspaces.idCopied": {
    message: "已复制工作区 ID",
    description: "Toast after copying the workspace ID",
  },
  "workspaces.idCopyFailed": {
    message: "无法复制工作区 ID",
    description: "Toast when copying the workspace ID fails",
  },
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
    message: "每个组织 $349.30/月",
    description:
      "Workspace plan card billing label (pricing.yaml scale usdPerMonth, per: organization; w7/m464)",
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
    message: "每个组织定制条款",
    description:
      "Workspace plan card billing label: Enterprise is charged once per organization like Scale (w7/m464)",
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
    message:
      "CLI 和 API 使用的唯一标识。仅限小写字母、数字和连字符，1–30 个字符。",
    description: "/new/workspace slug helper text",
  },
  "workspaces.fieldSlugError": {
    message: "仅限小写字母、数字和连字符，1-30 个字符，首尾不能是连字符。",
    description: "Workspace slug validation error",
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
  "workspaces.fieldDisplayNamePlaceholder": {
    message: "例如：Acme 预发环境",
    description: "/new/workspace freeform name placeholder",
  },
  "workspaces.fieldNamePlaceholder": {
    message: "例如 acme-staging",
    description: "Workspace name field placeholder",
  },
  "workspaces.fieldNameError": {
    message: "请输入 1–100 个字符的名称，不能包含换行。",
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
  "workspaces.createCancelError": {
    message: "取消创建工作区失败，请重试。",
    description:
      "/new/workspace toast when cancelling the pending creation fails",
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
  "workspaces.creationLoadError": {
    message: "无法加载工作区创建信息。请刷新页面重试。",
    description:
      "Inline message when the workspace-creation plan policy or a resumed setup attempt fails to load",
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
  "workspaces.ipRulesTitle": {
    message: "入站 IP 规则",
    description:
      "Workspace Settings → Inbound IP Rules card title and section link (w9/m388)",
  },
  "workspaces.ipRulesDescription": {
    message:
      "选择哪些 IP 地址可以访问此工作区中的所有 Web 服务、静态站点、Postgres 数据库和 Key Value 实例。这些规则叠加在环境和服务规则之上，来源必须在每一层都被允许。私有服务、后台 Worker 和 Cron 任务不受影响。",
    description:
      "Inbound IP Rules card description: scope and layering of workspace rules (Render's workspace-level inbound IP rules, w9/m388)",
  },
  "workspaces.ipRulesHint": {
    message:
      "输入 CIDR（例如 203.0.113.0/24）并点击添加来源。没有任何规则时，所有入站流量都会被拦截；0.0.0.0/0 和 ::/0 允许所有 IPv4 和 IPv6 来源。",
    description: "Hint above the editable workspace inbound IP rules (w9/m388)",
  },
  "workspaces.ipRulesBlocked": {
    message:
      "没有规则：此工作区服务和数据存储的所有入站流量都会被拦截。添加 0.0.0.0/0 和 ::/0 可允许所有来源。",
    description:
      "Warning shown when the workspace rule list is empty (deny-all), saved or about to be saved (w9/m388)",
  },
  "workspaces.ipRulesOpen": {
    message: "向所有源 IP 开放",
    description:
      "Shown in the read-only view when the workspace rules restrict nothing (Render's allow-all default, w9/m388)",
  },
  "workspaces.ipRulesScalePlanOnly": {
    message:
      "工作区的入站 IP 规则需要 Scale 或更高套餐。已设置的规则继续生效，可以重置为允许全部。",
    description:
      "Notice replacing the editable workspace rules below the Scale plan (w9/m388)",
  },
  "workspaces.ipRulesAdminOnly": {
    message: "只有工作区管理员可以更改这些规则。",
    description:
      "Notice for a member who cannot edit the workspace inbound IP rules, and the forbidden-save explanation (w9/m388)",
  },
  "workspaces.ipRulesPlanRefused": {
    message:
      "此工作区的套餐不包含入站 IP 规则。升级到 Scale 或更高套餐即可限制流量；你随时可以将规则重置为允许全部。",
    description:
      "Inline explanation when bex-api refuses a workspace rule change with PLAN_FEATURE_REQUIRED (w9/m388)",
  },
  "workspaces.ipRulesUnreadable": {
    message: "无法加载此工作区的入站 IP 规则。",
    description:
      "Shown in place of the rules when the workspace inbound IP rules read fails (w9/m388)",
  },
  "workspaces.ipRulesResetAllowAll": {
    message: "重置为允许全部",
    description:
      "Button replacing the workspace inbound IP rules with the allow-all pair below the Scale plan (w9/m388)",
  },
  "workspaces.ipRulesCIDR": {
    message: "新 CIDR 网段",
    description:
      "Accessible label for the add-row CIDR input of the workspace inbound IP rules",
  },
  "workspaces.ipRulesCIDRRule": {
    message: "规则 {number} 的 CIDR 网段",
    description:
      "Accessible label for an existing workspace inbound IP rule CIDR",
  },
  "workspaces.ipRulesDescriptionLabel": {
    message: "新规则描述",
    description:
      "Accessible label for the add-row description input of the workspace inbound IP rules",
  },
  "workspaces.ipRulesDescriptionRule": {
    message: "规则 {number} 的描述",
    description:
      "Accessible label for an existing workspace inbound IP rule description",
  },
  "workspaces.ipRulesDescriptionPlaceholder": {
    message: "描述（可选）",
    description: "Placeholder for a workspace inbound IP rule description",
  },
  "workspaces.ipRulesAdd": {
    message: "添加来源",
    description: "Button adding a CIDR to the draft workspace inbound IP rules",
  },
  "workspaces.ipRulesCancel": {
    message: "取消",
    description:
      "Button discarding unsaved changes to the workspace inbound IP rules",
  },
  "workspaces.ipRulesSave": {
    message: "保存规则",
    description:
      "Button saving the workspace inbound IP rules (setWorkspaceIPAllowList)",
  },
  "workspaces.ipRulesInvalid": {
    message: "请输入唯一且有效的 IPv4 或 IPv6 CIDR。",
    description:
      "Inline validation when a workspace rule block is not a unique, valid CIDR",
  },
  "workspaces.ipRulesRemove": {
    message: "移除 {cidr}",
    description:
      "Accessible label for a workspace inbound IP rule remove button",
  },
  "workspaces.ipRulesMoveUp": {
    message: "上移 {cidr}",
    description: "Accessible label to move a workspace inbound IP rule earlier",
  },
  "workspaces.ipRulesMoveDown": {
    message: "下移 {cidr}",
    description: "Accessible label to move a workspace inbound IP rule later",
  },
  "workspaces.ipRulesSaved": {
    message: "入站 IP 规则已保存",
    description: "Toast after setWorkspaceIPAllowList succeeds (w9/m388)",
  },
  "workspaces.ipRulesSaveErrorTitle": {
    message: "无法保存入站 IP 规则",
    description:
      "Title of the inline alert when a workspace rule change is refused (w9/m388)",
  },
  "workspaces.ipRulesSaveError": {
    message: "无法保存入站 IP 规则，请重试。",
    description:
      "Fallback inline message when a workspace rule change fails without a server reason (w9/m388)",
  },
  "workspaces.twoFactorTitle": {
    message: "两步验证",
    description:
      "Title of the two-factor card on the workspace settings page, and its quick-nav label (w22/m402)",
  },
  "workspaces.twoFactorDescription": {
    message:
      "未启用两步验证（2FA）的成员在启用之前无法访问此工作区的资源或设置，其 API 密钥和 SSH 密钥在此之前也会被阻止。",
    description:
      "Description under the two-factor card title; follows Render's login-settings wording",
  },
  "workspaces.twoFactorSwitchLabel": {
    message: "强制启用两步验证（2FA）",
    description:
      "Label of the switch that requires 2FA of every member (Render's control name)",
  },
  "workspaces.loginRequirementStatusRequired": {
    message: "已要求",
    description:
      "Read-only status shown to a non-admin when the workspace requires 2FA",
  },
  "workspaces.loginRequirementStatusNotRequired": {
    message: "未要求",
    description:
      "Read-only status shown to a non-admin when the workspace does not require 2FA",
  },
  "workspaces.loginRequirementAdminOnly": {
    message: "只有工作区管理员可以更改此设置。",
    description: "Hint shown to a non-admin in place of the switch",
  },
  "workspaces.twoFactorProPlanOnly": {
    message:
      "强制启用两步验证需要 Pro 或更高套餐。已经强制启用的工作区会保留该设置，直到管理员将其关闭。",
    description:
      "Notice on the two-factor card below the Pro plan, and the refusal for code PLAN_FEATURE_REQUIRED (Render's plan table, w7/070)",
  },
  "workspaces.twoFactorEnabledToast": {
    message: "{name} 现已要求使用两步验证",
    description: "Toast after an admin turns the 2FA requirement on",
  },
  "workspaces.twoFactorDisabledToast": {
    message: "{name} 不再要求使用两步验证",
    description: "Toast after an admin turns the 2FA requirement off",
  },
  "workspaces.twoFactorErrorTitle": {
    message: "无法更改两步验证设置",
    description: "Title of the inline error when the 2FA switch is refused",
  },
  "workspaces.twoFactorError": {
    message: "无法更改两步验证设置，请重试。",
    description:
      "Generic message when the 2FA switch fails without a recognised code",
  },
  "workspaces.twoFactorErrorAdminNotEnrolled": {
    message:
      "请先为你自己的账号启用两步验证，再在此工作区中要求使用，否则你会把自己锁在门外。",
    description:
      "Refusal for code WORKSPACE_2FA_ADMIN_NOT_ENROLLED: the acting admin has no second factor",
  },
  "workspaces.twoFactorErrorUnavailable": {
    message: "此部署不支持强制两步验证，因此无法开启。",
    description:
      "Refusal for code WORKSPACE_2FA_UNAVAILABLE: the deployment has no identity lookup",
  },
  "workspaces.twoFactorEnrollLink": {
    message: "设置两步验证",
    description:
      "Link to account security, in the admin-not-enrolled refusal and on the blocked page",
  },
  "workspaces.twoFactorBlockedTitle": {
    message: "{workspace} 要求使用两步验证",
    description: "Heading of the page a blocked member sees (w22/m402)",
  },
  "workspaces.twoFactorBlockedBody": {
    message:
      "此工作区的管理员要求每位成员使用两步验证（2FA）。在你启用之前，你无法访问此工作区的资源或设置，你的 API 密钥和 SSH 密钥也无法在其中使用。",
    description: "Why the blocked page is shown",
  },
  "workspaces.twoFactorBlockedRecheck": {
    message: "我已启用，重新检查",
    description:
      "Button on the blocked page that retries the workspace after enrolling",
  },
  "workspaces.twoFactorBlockedRecheckHint": {
    message: "启用两步验证后，最多需要一分钟才能恢复访问。",
    description: "Hint under the blocked page's check-again button",
  },
  "workspaces.loginBlockedSwitchLabel": {
    message: "切换到其他工作区",
    description: "Label of the workspace picker on the blocked page",
  },
  "workspaces.loginBlockedSwitchPlaceholder": {
    message: "选择工作区",
    description: "Placeholder of the workspace picker on the blocked page",
  },
  "workspaces.loginBlockedNoOtherWorkspaces": {
    message: "没有其他工作区",
    description:
      "Placeholder of the blocked page's workspace picker when the member belongs to no other workspace",
  },
  "workspaces.googleLoginTitle": {
    message: "Google 登录",
    description:
      "Title of the Google login card on the workspace settings page, and its quick-nav label (w7/m465)",
  },
  "workspaces.googleLoginDescription": {
    message:
      "未使用其 bex 邮箱对应的 Google 账号登录的成员无法访问此工作区的资源或设置。以其他方式登录时创建的 API 密钥和 SSH 密钥也会被阻止。",
    description:
      "Description under the Google login card title; follows Render's login-settings wording",
  },
  "workspaces.googleLoginSwitchLabel": {
    message: "强制使用 Google 登录",
    description:
      "Label of the switch that requires every member to sign in with Google (Render's control name)",
  },
  "workspaces.googleLoginProPlanOnly": {
    message:
      "强制使用 Google 登录需要 Pro 或更高套餐。已经强制启用的工作区会保留该设置，直到管理员将其关闭。",
    description:
      "Notice on the Google login card below the Pro plan, and the refusal for code PLAN_FEATURE_REQUIRED (Render's plan table)",
  },
  "workspaces.googleLoginEnabledToast": {
    message: "{name} 现已要求使用 Google 登录",
    description: "Toast after an admin turns the Google login requirement on",
  },
  "workspaces.googleLoginDisabledToast": {
    message: "{name} 不再要求使用 Google 登录",
    description: "Toast after an admin turns the Google login requirement off",
  },
  "workspaces.googleLoginErrorTitle": {
    message: "无法更改 Google 登录设置",
    description:
      "Title of the inline error when the Google login switch is refused",
  },
  "workspaces.googleLoginError": {
    message: "无法更改 Google 登录设置，请重试。",
    description:
      "Generic message when the Google login switch fails without a recognised code",
  },
  "workspaces.googleLoginErrorAdminNotSignedIn": {
    message:
      "请先使用你的 bex 邮箱对应的 Google 账号登录，再在此工作区中要求使用 Google 登录，否则你会把自己锁在门外。",
    description:
      "Refusal for code WORKSPACE_GOOGLE_LOGIN_ADMIN_NOT_SIGNED_IN: the acting admin is not signed in with Google",
  },
  "workspaces.googleLoginErrorUnavailable": {
    message: "此部署不支持强制 Google 登录，因此无法开启。",
    description:
      "Refusal for code WORKSPACE_GOOGLE_LOGIN_UNAVAILABLE: the deployment cannot look up Google accounts",
  },
  "workspaces.googleLoginSignIn": {
    message: "使用 Google 登录",
    description:
      "Button that signs the session in again with Google and returns to the page: on the blocked page and in the admin's lock-out refusal",
  },
  "workspaces.googleLoginSignInError": {
    message: "无法开始使用 Google 登录，请重试。",
    description: "Shown when Kratos did not start the Google sign-in",
  },
  "workspaces.googleLoginBlockedTitle": {
    message: "{workspace} 要求使用 Google 登录",
    description:
      "Heading of the page a member who did not sign in with Google sees (w7/m465)",
  },
  "workspaces.googleLoginBlockedBody": {
    message:
      "此工作区的管理员要求每位成员使用 Google 登录。在你使用 bex 邮箱对应的 Google 账号登录之前，你无法访问此工作区的资源或设置，你以其他方式登录时创建的 API 密钥和 SSH 密钥也无法在其中使用。",
    description: "Why the Google login blocked page is shown",
  },
  "workspaces.googleLoginBlockedHint": {
    message: "登录后你会回到此页面。",
    description: "Hint under the blocked page's Sign in with Google button",
  },
  "workspaces.requireTwoFactorConfirmTitle": {
    message: "要在 {workspace} 中要求两步验证吗？",
    description:
      "Title of the confirmation before an admin requires 2FA in the workspace (w7/m471)",
  },
  "workspaces.requireTwoFactorConfirm": {
    message:
      "未启用两步验证的成员将无法访问此工作区，直到他们启用为止，其 API 密钥和 SSH 密钥也将无法在其中使用。",
    description:
      "Body of the confirmation before an admin requires 2FA in the workspace (w7/m471)",
  },
  "workspaces.requireGoogleLoginConfirmTitle": {
    message: "要在 {workspace} 中要求使用 Google 登录吗？",
    description:
      "Title of the confirmation before an admin requires Google login in the workspace (w7/m471)",
  },
  "workspaces.requireGoogleLoginConfirm": {
    message:
      "未使用其 bex 邮箱对应的 Google 账号登录的成员将无法访问此工作区，直到他们以这种方式登录为止；以其他方式登录时创建的 API 密钥和 SSH 密钥也将无法在其中使用。",
    description:
      "Body of the confirmation before an admin requires Google login in the workspace (w7/m471)",
  },
  "workspaces.requireConfirmAction": {
    message: "要求",
    description:
      "Confirm button of the workspace's require-2FA and require-Google-login confirmations (w7/m471)",
  },
  "workspaces.loginImpactRegionLabel": {
    message: "谁将失去访问权限",
    description:
      "Accessible name of the confirmation's list of members and keys a login requirement would block (w7/m471)",
  },
  "workspaces.loginImpactChecking": {
    message: "正在检查谁将失去访问权限…",
    description:
      "Screen-reader text while the confirmation checks who a login requirement would block",
  },
  "workspaces.loginImpactError": {
    message: "无法检查谁将失去访问权限。你仍然可以开启此设置。",
    description:
      "Shown in the confirmation when bex could not check who a login requirement would block; confirming stays possible",
  },
  "workspaces.loginImpactNobody": {
    message:
      "没有人会失去访问权限：每位成员都满足此要求，也没有 API 密钥或 SSH 密钥会停止工作。",
    description:
      "Shown in the confirmation when a login requirement would block nobody",
  },
  "workspaces.loginImpactMembersTwoFactor": {
    message:
      "列出的每位成员将失去访问权限，直到他们启用两步验证。列出的每个 API 密钥和 SSH 密钥将停止工作，直到其所有者启用为止。",
    description:
      "Sentence above the members and keys requiring 2FA would block, after Render's SSO enforcement list",
  },
  "workspaces.loginImpactMembersGoogleLogin": {
    message:
      "列出的每位成员将失去访问权限，直到他们使用其 bex 邮箱对应的 Google 账号登录。列出的每个 API 密钥和 SSH 密钥将停止工作；其所有者可以在使用 Google 登录后创建新的密钥。",
    description:
      "Sentence above the members and keys requiring Google login would block, after Render's SSO enforcement list",
  },
  "workspaces.loginImpactLosesAccess": {
    message: "失去访问权限",
    description: "Badge on a listed member who would lose access",
  },
  "workspaces.loginImpactKeepsAccess": {
    message: "保留访问权限",
    description:
      "Badge on a listed member who keeps access while their listed keys stop working",
  },
  "workspaces.loginImpactApiKey": {
    message: "API 密钥 {name}",
    description:
      "A listed API key that would stop working; {name} is the key's name",
  },
  "workspaces.loginImpactApiKeyInWorkspace": {
    message: "{workspace} 中的 API 密钥 {name}",
    description:
      "A listed API key on an org's confirmation; {workspace} is the workspace it belongs to",
  },
  "workspaces.loginImpactSshKey": {
    message: "SSH 密钥 {name}",
    description:
      "A listed SSH key that would stop working; {name} is the key's name",
  },
  "workspaces.loginImpactUnattributed": {
    message: "没有记录创建者、或创建者已不再是成员的 API 密钥将停止工作：",
    description:
      "Heading of the API keys no current member created, which would stop working",
  },
  "workspaces.loginImpactGoogleSessions": {
    message: "满足要求但当前以其他方式登录的成员将被要求使用 Google 登录。",
    description:
      "One line under the Google login confirmation's list: who the list cannot show",
  },
  "workspaces.overlappingDeployPolicyTitle": {
    message: "重叠部署策略",
    description:
      "Workspace Settings section title, Render's name for the setting (w4/653)",
  },
  "workspaces.overlappingDeployPolicyDescription": {
    message: "选择当某个服务已有部署正在进行时，再次触发部署会发生什么。",
    description:
      "Workspace Settings → Overlapping Deploy Policy description (w4/653)",
  },
  "workspaces.overlappingDeployPolicyWait": {
    message: "等待",
    description: "The Wait policy's label, Render's wording (w4/653)",
  },
  "workspaces.overlappingDeployPolicyWaitDescription": {
    message: "让正在进行的部署完成，然后直接进行最近一次触发的部署。",
    description:
      "The Wait policy's sentence, Render's wording (render.com/docs/deploys, w4/653)",
  },
  "workspaces.overlappingDeployPolicyOverride": {
    message: "覆盖",
    description: "The Override policy's label, Render's wording (w4/653)",
  },
  "workspaces.overlappingDeployPolicyOverrideDescription": {
    message: "立即取消正在进行的部署，并开始新的部署。",
    description:
      "The Override policy's sentence, Render's wording (render.com/docs/deploys, w4/653)",
  },
  "workspaces.overlappingDeployPolicySave": {
    message: "保存",
    description: "Saves the selected Overlapping Deploy Policy (w4/653)",
  },
  "workspaces.overlappingDeployPolicySaving": {
    message: "正在保存…",
    description: "The Save button while the policy saves (w4/653)",
  },
  "workspaces.overlappingDeployPolicySaved": {
    message: "重叠部署策略已保存",
    description:
      "Toast after setWorkspaceOverlappingDeployPolicy succeeds (w4/653)",
  },
  "workspaces.overlappingDeployPolicyError": {
    message: "无法保存重叠部署策略，请重试。",
    description:
      "Fallback toast when a policy save fails without a server reason (w4/653)",
  },
  "workspaces.overlappingDeployPolicyAdminOnly": {
    message: "只有工作区管理员可以更改此策略。",
    description:
      "Shown in place of Save to a member who cannot manage the workspace (w4/653)",
  },
  "workspaces.orgDowngradeWarningTitle": {
    message: "降级到 Scale 以下会删除组织",
    description:
      "Change-plan dialog on an org's primary: title of the warning shown when a plan below Scale is selected (w7/m464)",
  },
  "workspaces.orgDowngradeWarning": {
    message:
      "这是你的组织的主工作区，它的套餐覆盖组织中的每个工作区。降级会移除组织以及组织级功能，此工作区将作为独立工作区继续使用。请先删除组织的其他工作区，将角色为账单、查看者或贡献者的成员和待处理邀请改为开发者或管理员，并删除服务、环境和此工作区的入站 IP 规则。",
    description:
      "Change-plan dialog on an org's primary: what a downgrade below Scale does and Render's preconditions (render.com/docs/organizations, w7/m464)",
  },
  "workspaces.orgDowngradeWarningHobby": {
    message:
      "Hobby 只有一名成员，因此还需移除此工作区的其他成员并撤销其待处理邀请。",
    description:
      "Change-plan dialog on an org's primary with Hobby selected: the extra step a one-member plan needs, below the downgrade warning (w7/087)",
  },
  "workspaces.orgDowngradeBlocked": {
    message: "组织暂时无法删除。降级到 Scale 以下之前，请先完成以下事项：",
    description:
      "Change-plan dialog: lead-in above the unmet preconditions of ORGANIZATION_DOWNGRADE_BLOCKED (w7/m464)",
  },
  "workspaces.orgDowngradeSecondaryWorkspaces": {
    message: "删除组织的其他工作区",
    description:
      "Unmet downgrade precondition secondary_workspaces; the workspace names follow (w7/m464)",
  },
  "workspaces.orgDowngradeMemberRoles": {
    message: "将这些成员的角色改为开发者或管理员",
    description:
      "Unmet downgrade precondition member_roles; the members and roles follow (w7/m464)",
  },
  "workspaces.orgDowngradeInviteRoles": {
    message: "将这些待处理邀请的角色改为开发者或管理员",
    description:
      "Unmet downgrade precondition invite_roles; the invited emails and roles follow (w7/m464)",
  },
  "workspaces.orgDowngradeMemberLimit": {
    message: "移除这些成员和待处理邀请（Hobby 只有一名成员）",
    description:
      "Unmet downgrade precondition member_limit on a Hobby target: the members and pending invites beyond its one seat follow (w7/087)",
  },
  "workspaces.orgDowngradeInboundIpRules": {
    message: "删除入站 IP 规则",
    description:
      "Unmet downgrade precondition inbound_ip_rules; the workspace, environments and services that hold them follow (w7/m464)",
  },
  "workspaces.orgDowngradeItems": {
    message: "{label}：{items}",
    description:
      "One unmet downgrade precondition: {label} is what to do, {items} the list of what it applies to (w7/m464)",
  },
  "workspaces.orgDowngradePersonRole": {
    message: "{person}（{role}）",
    description:
      "A member or invitee in a downgrade precondition: {person} is their id or email, {role} the role to change (w7/m464)",
  },
  "workspaces.orgDowngradeIpRuleWorkspace": {
    message: "此工作区",
    description:
      "The workspace's own inbound IP rules, in the inbound_ip_rules precondition (w7/m464)",
  },
  "workspaces.orgDowngradeIpRuleEnvironment": {
    message: "环境 {name}",
    description:
      "An environment with inbound IP rules, in the inbound_ip_rules precondition (w7/m464)",
  },
  "workspaces.orgDowngradeIpRuleService": {
    message: "服务 {name}",
    description:
      "A service with inbound IP rules, in the inbound_ip_rules precondition (w7/m464)",
  },
  "workspaces.orgPlanOnPrimaryError": {
    message: "此工作区的套餐是其组织的套餐，请在组织的主工作区中更改。",
    description:
      "Refusal for code ORGANIZATION_PLAN_ON_PRIMARY: a secondary workspace's plan changes only on the org's primary (w7/m464)",
  },
  "workspaces.orgPlanSecondaryNote": {
    message: "已包含在你的组织套餐中。组织的主工作区管理套餐和账单。",
    description:
      "Workspace Settings and Billing on an org's secondary workspace: its plan is the org's, managed on the primary (ADR115, w7/m464)",
  },
  "workspaces.orgPlanPrimarySettingsLink": {
    message: "前往主工作区",
    description:
      "Link from a secondary workspace's settings to the org's primary workspace settings (w7/m464)",
  },
  "workspaces.orgPlanPrimaryBillingLink": {
    message: "前往主工作区的账单",
    description:
      "Button on a secondary workspace's Billing plan card linking to the org's primary workspace's Billing (w7/m464)",
  },
  "workspaces.privacyTitle": {
    message: "工作区隐私",
    description:
      "Workspace Settings → Workspace privacy card title, radio group name and section link (w7/m464)",
  },
  "workspaces.privacyDescription": {
    message: "选择组织中哪些人可以加入此工作区。",
    description: "Description under the Workspace privacy card title (w7/m464)",
  },
  "workspaces.privacyPublic": {
    message: "公开",
    description: "The Public privacy setting's label, Render's name (w7/m464)",
  },
  "workspaces.privacyPublicDescription": {
    message: "组织的任何成员都可以自行加入此工作区。",
    description:
      "What the Public privacy setting allows, Render's wording (render.com/docs/organizations, w7/m464)",
  },
  "workspaces.privacyInviteOnly": {
    message: "仅限邀请",
    description:
      "The Invite-only privacy setting's label, Render's name (w7/m464)",
  },
  "workspaces.privacyInviteOnlyDescription": {
    message: "只有工作区管理员可以邀请他人加入此工作区。",
    description:
      "What the Invite-only privacy setting allows, Render's wording (render.com/docs/organizations, w7/m464)",
  },
  "workspaces.privacySave": {
    message: "保存",
    description: "Saves the selected workspace privacy (w7/m464)",
  },
  "workspaces.privacySaving": {
    message: "正在保存…",
    description: "The Save button while the workspace privacy saves (w7/m464)",
  },
  "workspaces.privacySaved": {
    message: "工作区隐私已保存",
    description: "Toast after setWorkspacePrivacy succeeds (w7/m464)",
  },
  "workspaces.privacyError": {
    message: "无法保存工作区隐私，请重试。",
    description:
      "Fallback toast when a privacy save fails without a server reason (w7/m464)",
  },
  "workspaces.loginRequirementRequiredByOrganization": {
    message: "{organization} 要求其所有工作区启用此项。",
    description:
      "Workspace Settings, under the 2FA or Google login switch: why it is on and locked, the workspace's organization requires it in all of its workspaces (w7/m466)",
  },
  "workspaces.loginRequirementOrganizationSettingsLink": {
    message: "在组织设置中更改",
    description:
      "Link for an org Owner after the required-by-organization note, to the organization's Settings page (w7/m466)",
  },
  "workspaces.twoFactorBlockedOrganizationTitle": {
    message: "{organization} 要求在其所有工作区中使用两步验证",
    description:
      "Heading of the 2FA blocked page when the organization's setting refuses the member, on a workspace or on the org's own pages (w7/m466)",
  },
  "workspaces.twoFactorBlockedOrganizationBody": {
    message:
      "此组织的所有者要求每位成员在组织及其所有工作区中使用两步验证（2FA）。在你启用之前，你无法访问它们，你的 API 密钥和 SSH 密钥也无法在其中使用。",
    description:
      "Why the 2FA blocked page is shown when the organization's setting refuses the member (w7/m466)",
  },
  "workspaces.googleLoginBlockedOrganizationTitle": {
    message: "{organization} 要求在其所有工作区中使用 Google 登录",
    description:
      "Heading of the Google login blocked page when the organization's setting refuses the member (w7/m466)",
  },
  "workspaces.googleLoginBlockedOrganizationBody": {
    message:
      "此组织的所有者要求每位成员在组织及其所有工作区中使用 Google 登录。在你使用 bex 邮箱对应的 Google 账号登录之前，你无法访问它们，你以其他方式登录时创建的 API 密钥和 SSH 密钥也无法在其中使用。",
    description:
      "Why the Google login blocked page is shown when the organization's setting refuses the member (w7/m466)",
  },
};

export default zhWorkspaces;
