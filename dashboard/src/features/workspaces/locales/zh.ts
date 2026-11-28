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
      "Plan link on Workspace Settings, the plan gates and the webhooks gate, opening Render's plan page (w3/m708)",
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
  "workspaces.loginNoticeMembers_other": {
    message: "{count} 名成员",
    description:
      "The bold count in the notice under an enforced login requirement: how many members do not meet it (w1/m647, Render's wording)",
  },
  "workspaces.loginNoticeTwoFactor_other": {
    message: "团队中有 {members}尚未启用两步验证。",
    description:
      "Notice under enforced 2FA when members have not enabled it; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeGoogleLogin_other": {
    message: "团队中有 {members}尚未使用 Google 登录。",
    description:
      "Notice under enforced Google login when members have not signed in with Google; {members} is the bold count (Render's wording)",
  },
  "workspaces.loginNoticeViewMembers_other": {
    message: "查看成员",
    description:
      "Button on the notice that opens the list of members who do not meet the requirement (Render's wording)",
  },
  "workspaces.loginNoticeAllMembers": {
    message: "所有成员",
    description:
      "The bold words of the all-compliant notice (Render's wording)",
  },
  "workspaces.loginNoticeAllTwoFactor": {
    message: "团队{members}均已启用两步验证！",
    description:
      "Notice under enforced 2FA when every member has enabled it; {members} is the bold 'All members' (Render's wording)",
  },
  "workspaces.loginNoticeAllGoogleLogin": {
    message: "团队{members}均已使用 Google 登录！",
    description:
      "Notice under enforced Google login when every member has signed in with Google; {members} is the bold 'All members' (Render's wording)",
  },
  "workspaces.loginMembersTitleTwoFactor": {
    message: "未启用两步验证的成员",
    description:
      "Title of the dialog listing members without 2FA (Render's wording)",
  },
  "workspaces.loginMembersTitleGoogleLogin": {
    message: "未使用 Google 登录的成员",
    description:
      "Title of the dialog listing members who have not signed in with Google (Render's wording)",
  },
  "workspaces.loginMembersDescription_other": {
    message: "列出的成员目前被禁止访问此团队工作区及其中的资源。",
    description:
      "Description of the dialog listing members who do not meet the enforced requirement (Render's wording)",
  },
  "workspaces.twoFactorRemindTitle_other": {
    message: "发送启用 2FA 的提醒？",
    description:
      "Title of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindBody_other": {
    message: "通过电子邮件发送提醒。",
    description:
      "Body of the reminder callout in the members-without-2FA dialog (Render's wording)",
  },
  "workspaces.twoFactorRemindAction_other": {
    message: "发送提醒",
    description: "Button that emails the 2FA reminders (Render's wording)",
  },
  "workspaces.twoFactorRemindError": {
    message: "抱歉，我们无法向这些团队成员发送提醒。要重试吗？",
    description:
      "Shown in the members dialog when the reminders could not be sent (Render's wording)",
  },
  "workspaces.twoFactorRemindSentToast": {
    message: "已向未启用两步验证的成员发送电子邮件提醒！",
    description: "Toast after the 2FA reminders are sent (Render's wording)",
  },
  "workspaces.twoFactorRemindLimitTitle": {
    message: "已达到提醒次数上限！",
    description:
      "Title of the dialog shown when reminders were already sent today (Render's wording)",
  },
  "workspaces.twoFactorRemindLimitBody": {
    message: "今天似乎已经发送过提醒了。你明天可以再次发送提醒。",
    description:
      "Body of the dialog shown when reminders were already sent today; the day is UTC (Render's wording)",
  },
  "workspaces.twoFactorConfirmReminder_other": {
    message: "将向列出的成员发送启用 2FA 的提醒。",
    description:
      "Line in the require-2FA confirmation: turning it on emails a reminder to each member listed (Render's wording)",
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
  "workspaces.updatePlanPageTitle": {
    message: "更改套餐",
    description: "Document title of the plan pages (w3/m708)",
  },
  "workspaces.updatePlanTitle": {
    message: "更改 {name} 的套餐",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): page title with the workspace's name",
  },
  "workspaces.updatePlanConfirmTitle": {
    message: "更改 {name} 的套餐",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): page title with the workspace's name (Render capitalizes Plan here)",
  },
  "workspaces.updatePlanBack": {
    message: "返回",
    description:
      "Accessible label of the plan pages' Back button, and the confirmation's Back action (w3/m708)",
  },
  "workspaces.updatePlanSelectTitle": {
    message: "选择套餐",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): heading of the plan cards",
  },
  "workspaces.updatePlanCompare": {
    message: "访问我们的{link}，了解或比较各套餐。",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): line under Select a plan; {link} is the updatePlanPricingLink link to bex's pricing page",
  },
  "workspaces.updatePlanPricingLink": {
    message: "价格页面",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): the link in updatePlanCompare",
  },
  "workspaces.updatePlanPaymentTitle": {
    message: "付款方式",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): heading of the payment method section",
  },
  "workspaces.updatePlanCardRequired": {
    message: "此工作区套餐需要绑定银行卡，用于付费服务和计算资源。",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): notice while a plan other than Hobby is selected",
  },
  "workspaces.updatePlanHowBillingWorks": {
    message: "计费方式说明",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): link beside updatePlanCardRequired to bex's billing docs",
  },
  "workspaces.updatePlanWorkspaceBillingTitle": {
    message: "工作区计费",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): heading beside the card on file",
  },
  "workspaces.updatePlanWorkspaceBillingBody": {
    message:
      "每个工作区单独计费。此工作区中付费服务或计算资源产生的费用将从你的银行卡扣除。",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): line under updatePlanWorkspaceBillingTitle",
  },
  "workspaces.updatePlanErrorsSummary": {
    message: "你的选择存在一些问题，请检查并修正后继续。",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): summary under the plan rules a selection breaks",
  },
  "workspaces.updatePlanErrorPrefix": {
    message: "发生错误。{message}",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): a refused change; {message} is the reason",
  },
  "workspaces.updatePlanPaymentRequired": {
    message: "请添加有效的银行卡以升级到付费套餐。",
    description:
      "The plan pages' message when bex-api refuses a paid plan with PAYMENT_REQUIRED (w3/m708)",
  },
  "workspaces.updatePlanServerError": {
    message: "提交时服务器出错，请重试",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): fallback reason when an upgrade fails without one",
  },
  "workspaces.updatePlanSubmit": {
    message: "选择套餐",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): submit button",
  },
  "workspaces.updatePlanCancel": {
    message: "取消",
    description:
      "Render's plan page, /billing/update-plan (w3/m708): cancel button, back to where the page was opened from",
  },
  "workspaces.updatePlanNoPermission": {
    message: "你的角色无法更改此工作区的套餐。请联系工作区管理员。",
    description:
      "The plan pages' notice for a member who cannot change the plan (can_manage) (w3/m708)",
  },
  "workspaces.planCurrentBadge": {
    message: "当前套餐",
    description:
      "Badge on the workspace's own plan card on the plan page (Render's TeamPlanPicker, w3/m708)",
  },
  "workspaces.planNewBadge": {
    message: "新套餐",
    description:
      "Badge on the chosen plan card on the plan page (Render's TeamPlanPicker, w3/m708)",
  },
  "workspaces.planContact": {
    message: "联系我们",
    description:
      "Enterprise card's contact action on the plan page, Render's Get in touch (w3/m708)",
  },
  "workspaces.updatePlanSummaryTitle": {
    message: "新套餐摘要",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): heading of the plan comparison and the card on file",
  },
  "workspaces.updatePlanConfirmDowngradeTitle": {
    message: "确认降级",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): heading of the typed confirmation a downgrade asks for",
  },
  "workspaces.updatePlanConfirmChangesIncluding": {
    message: "要开始更改你的套餐（包括以下内容）：",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): lead-in above the list of what a downgrade changes",
  },
  "workspaces.updatePlanConfirmTypeAfterList": {
    message: "在文本框中输入 {phrase}，然后点击确认按钮。",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): the typed phrase after the list of changes; {phrase} is 'sudo downgrade workspace <name>' (rendered bold)",
  },
  "workspaces.updatePlanConfirmType": {
    message:
      "要开始更改你的套餐，请在文本框中输入 {phrase}，然后点击确认按钮。",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): the typed phrase when the downgrade lists no changes; {phrase} is 'sudo downgrade workspace <name>' (rendered bold)",
  },
  "workspaces.updatePlanConfirmSubmit": {
    message: "确认",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): confirm button",
  },
  "workspaces.updatePlanErrorTitle": {
    message: "发生错误。",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): title of a refused change",
  },
  "workspaces.updatePlanError": {
    message: "更新套餐时出错。",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): fallback reason when the change fails without one",
  },
  "workspaces.updatePlanRemovesOrganization": {
    message: "该组织将被移除。",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): bold start of the change an org primary's downgrade below Scale makes (ADR115 D6)",
  },
  "workspaces.updatePlanRemovesOrganizationDetail": {
    message: "组织级功能将终止，此工作区将作为独立工作区继续存在。",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): rest of the org removal line after updatePlanRemovesOrganization",
  },
  "workspaces.planUpdated": {
    message: "你的工作区现已切换到 {plan} 套餐。",
    description:
      "Toast after a plan change, Render's plan-updated toast; {plan} is the plan's name (w3/m708)",
  },
  "workspaces.planUpdatedWelcome": {
    message: "欢迎使用 {plan} 套餐。",
    description:
      "Toast after a change to Scale, Render's welcome; {plan} is the plan's name (w3/m708)",
  },
  "workspaces.planComparisonCurrent": {
    message: "当前套餐",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): badge on the current plan's side of the comparison",
  },
  "workspaces.planComparisonNew": {
    message: "新套餐",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): badge on the new plan's side of the comparison",
  },
  "workspaces.planComparisonHighlights": {
    message: "亮点",
    description:
      "Render's plan confirmation, /billing/update-plan-confirm (w3/m708): label above each side's highlights (shown in capitals)",
  },
  "workspaces.planHighlightJustForMe": {
    message: "仅限本人",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): Hobby's one seat",
  },
  "workspaces.planHighlightUnlimitedMembers": {
    message: "团队成员不限",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): unlimited seats",
  },
  "workspaces.planHighlightIncludedDomains": {
    message: "含 {included} 个自定义域名，超出后每个 {price}",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): custom domains included before they are metered; {included} is the count, {price} bex's price per domain (ADR030 §16)",
  },
  "workspaces.planHighlightUnlimitedDomains": {
    message: "自定义域名不限",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): every custom domain included (Enterprise)",
  },
  "workspaces.planHighlightTwoEnvironments": {
    message: "每个项目 2 个环境",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): Hobby's environments per project",
  },
  "workspaces.planHighlightUnlimitedEnvironments": {
    message: "项目和环境不限",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): unlimited projects and environments",
  },
  "workspaces.planHighlightRetention": {
    message: "日志和指标保留 {days} 天",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): how far back logs and metrics reach; {days} is the number of days",
  },
  "workspaces.planHighlightBasicLogs": {
    message: "基础日志和指标",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): Hobby's logs and metrics, without request logs",
  },
  "workspaces.planHighlightRequestLogs": {
    message: "HTTP 请求日志和响应时间指标",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): Pro's HTTP request logs and response time metrics",
  },
  "workspaces.planHighlightKeptBuilds": {
    message: "保留 {builds} 个构建用于即时回滚",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): builds kept for rollback; {builds} is the count (core.KeptBuilds)",
  },
  "workspaces.planHighlightAuditLogs": {
    message: "工作区审计日志",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): workspace audit logs (Pro and higher)",
  },
  "workspaces.planHighlightOrganization": {
    message: "组织：更多工作区、额外角色和入站 IP 规则",
    description:
      "A highlight in the confirmation's plan comparison (w3/m708): what an organization adds (Scale and higher, ADR115)",
  },
  "workspaces.planBlockerMemberLimitTitle": {
    message: "团队用户过多。",
    description:
      "Render's plan rule on its plan page (w3/m708): title when the workspace has more people than the plan seats",
  },
  "workspaces.planBlockerMemberLimitOne": {
    message: "{plan} 工作区只能有 1 名用户。",
    description:
      "Render's plan rule on its plan page (w3/m708): a plan with one seat; {plan} is the plan's name",
  },
  "workspaces.planBlockerMemberLimitMany": {
    message: "{plan} 工作区最多只能有 {limit} 名用户。",
    description:
      "Render's plan rule on its plan page (w3/m708): a plan with several seats; {plan} is the plan's name, {limit} its seats",
  },
  "workspaces.planBlockerUpdateTeamMembers": {
    message: "更新团队成员",
    description:
      "Render's plan rule on its plan page (w3/m708): link to Workspace Settings' Team section",
  },
  "workspaces.planBlockerServiceLimitTitle": {
    message: "工作区服务过多。",
    description:
      "Render's plan rule on its plan page (w3/m708): title when the workspace has more services than the plan allows",
  },
  "workspaces.planBlockerServiceLimit": {
    message: "{plan} 工作区最多只能有 {limit} 个服务。",
    description:
      "Render's plan rule on its plan page (w3/m708): the plan's service cap; {plan} is the plan's name, {limit} its cap",
  },
  "workspaces.planBlockerUpdateServices": {
    message: "更新服务",
    description:
      "Render's plan rule on its plan page (w3/m708): link to the workspace's services",
  },
  "workspaces.planBlockerWorkspaceLimitTitle": {
    message: "Hobby 工作区过多。",
    description:
      "Render's plan rule on its plan page (w3/m708): title when the caller already has the most Hobby workspaces allowed",
  },
  "workspaces.planBlockerWorkspaceLimit": {
    message:
      "Hobby 工作区不能超过 {limit} 个。请升级套餐或删除另一个 Hobby 工作区。",
    description:
      "Render's plan rule on its plan page (w3/m708): the cap on a person's Hobby workspaces; {limit} is the cap",
  },
  "workspaces.planBlockerWorkspaceLimitsDocs": {
    message: "工作区限制",
    description:
      "Render's plan rule on its plan page (w3/m708): link to bex's docs on workspace limits",
  },
  "workspaces.planBlockerWorkspacesTitle": {
    message: "工作区过多。",
    description:
      "Render's plan rule on its plan page (w3/m708): title when an org primary's downgrade leaves secondary workspaces",
  },
  "workspaces.planBlockerWorkspaces": {
    message: "{plan} 不支持多个工作区。",
    description:
      "Render's plan rule on its plan page (w3/m708): a plan without an organization; {plan} is the plan's name",
  },
  "workspaces.planBlockerUpdateWorkspaces": {
    message: "更新工作区",
    description:
      "Render's plan rule on its plan page (w3/m708): link to the organization's workspaces",
  },
  "workspaces.planBlockerRolesTitle": {
    message: "使用不可用角色时无法降级",
    description:
      "Render's plan rule on its plan page (w3/m708): title when members or invites hold roles the plan does not offer",
  },
  "workspaces.planBlockerRolesOne": {
    message:
      "{plan} 不支持 {roles} 角色。要降级工作区，请先将受影响的团队成员和待处理邀请改为 Developer 或 Admin。",
    description:
      "Render's plan rule on its plan page (w3/m708): one role the plan lacks; {plan} is the plan's name, {roles} the role",
  },
  "workspaces.planBlockerRolesMany": {
    message:
      "{plan} 不支持 {roles} 角色。要降级工作区，请先将受影响的团队成员和待处理邀请改为 Developer 或 Admin。",
    description:
      "Render's plan rule on its plan page (w3/m708): several roles the plan lacks; {plan} is the plan's name, {roles} the roles",
  },
  "workspaces.planBlockerIpRulesTitle": {
    message: "降级前必须移除 IP 允许列表。",
    description:
      "Render's plan rule on its plan page (w3/m708): title when inbound IP rules stand in the way (Render's words for them on its confirmation)",
  },
  "workspaces.planBlockerIpRules": {
    message: "更改套餐前，请从工作区、环境和服务中移除所有 IP 允许列表配置。",
    description:
      "Render's plan rule on its plan page (w3/m708): what to remove for planBlockerIpRulesTitle",
  },
  "workspaces.planBlockerUpdateIpRules": {
    message: "更新入站 IP 规则",
    description:
      "Render's plan rule on its plan page (w3/m708): link to Workspace Settings' inbound IP rules",
  },
  "workspaces.planBlockerOtherTitle": {
    message: "暂时无法选择此套餐。",
    description:
      "Render's plan rule on its plan page (w3/m708): title of a refusal kind this dashboard does not know yet",
  },
  "workspaces.planBlockerOther": {
    message: "此工作区尚未满足 {plan} 套餐的限制。",
    description:
      "Render's plan rule on its plan page (w3/m708): line under planBlockerOtherTitle; {plan} is the plan's name",
  },
};

export default zhWorkspaces;
