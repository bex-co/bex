import type { TranslationEntry } from "@/i18n";

const koWorkspaces: Record<string, TranslationEntry> = {
  "workspaces.idCopied": {
    message: "워크스페이스 ID를 복사했습니다",
    description: "Toast after copying the workspace ID",
  },
  "workspaces.idCopyFailed": {
    message: "워크스페이스 ID를 복사하지 못했습니다",
    description: "Toast when copying the workspace ID fails",
  },
  "workspaces.switcherEmpty": {
    message: "워크스페이스 선택",
    description: "Switcher trigger label before any workspace is selected",
  },
  "workspaces.switcherLabel": {
    message: "워크스페이스",
    description: "Switcher dropdown label above the workspace list",
  },
  "workspaces.switcherBilling": {
    message: "결제",
    description: "Switcher menu item linking to /billing",
  },
  "workspaces.switcherSettings": {
    message: "워크스페이스 설정",
    description: "Switcher menu item linking to /workspace/settings",
  },
  "workspaces.switcherNew": {
    message: "+ 새 워크스페이스",
    description: "Switcher menu item linking to /new/workspace",
  },
  "workspaces.newTitle": {
    message: "새 워크스페이스",
    description: "New-workspace page document title",
  },
  "workspaces.planPickerLabel": {
    message: "요금제",
    description: "Accessible label for the plan-card radiogroup",
  },
  "workspaces.planHobbyName": {
    message: "Hobby",
    description: "Workspace plan card name",
  },
  "workspaces.planHobbyBilling": {
    message: "$0/월",
    description:
      "Workspace plan card billing label (pricing.yaml hobby usdPerMonth)",
  },
  "workspaces.planHobbyDescription": {
    message: "시작하는 개인을 위한 요금제입니다.",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planHobbyBulletMembers": {
    message: "멤버 1명",
    description: "Hobby plan LimitsFor.MaxMembers bullet",
  },
  "workspaces.planHobbyBulletServices": {
    message: "서비스 최대 25개",
    description: "Hobby plan LimitsFor.MaxServices bullet",
  },
  "workspaces.planHobbyBulletWorkspaces": {
    message: "사용자당 Hobby 워크스페이스 5개",
    description: "Hobby plan LimitsFor.MaxWorkspacesPerUser bullet",
  },
  "workspaces.planProName": {
    message: "Pro",
    description: "Workspace plan card name",
  },
  "workspaces.planProBilling": {
    message: "$17.50/월",
    description:
      "Workspace plan card billing label (pricing.yaml pro usdPerMonth)",
  },
  "workspaces.planProDescription": {
    message: "함께 개발하고 배포하는 소규모 팀을 위한 요금제입니다.",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planProBulletMembers": {
    message: "멤버 무제한",
    description: "Pro plan unlimited MaxMembers bullet",
  },
  "workspaces.planProBulletServices": {
    message: "서비스 무제한",
    description: "Pro plan unlimited MaxServices bullet",
  },
  "workspaces.planScaleName": {
    message: "Scale",
    description: "Workspace plan card name",
  },
  "workspaces.planScaleBilling": {
    message: "$349.30/월",
    description:
      "Workspace plan card billing label (pricing.yaml scale usdPerMonth)",
  },
  "workspaces.planScaleDescription": {
    message: "추가 역할이 필요한 성장하는 팀을 위한 요금제입니다.",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planScaleBulletMembers": {
    message: "멤버 무제한",
    description: "Scale plan unlimited MaxMembers bullet",
  },
  "workspaces.planScaleBulletServices": {
    message: "서비스 무제한",
    description: "Scale plan unlimited MaxServices bullet",
  },
  "workspaces.planScaleBulletRoles": {
    message: "추가 역할(기여자, 뷰어, 결제 담당자)",
    description: "Scale plan AllowedRoles beyond Pro bullet",
  },
  "workspaces.planEnterpriseName": {
    message: "Enterprise",
    description: "Workspace plan card name",
  },
  "workspaces.planEnterpriseBilling": {
    message: "맞춤 조건",
    description: "Workspace plan card billing label",
  },
  "workspaces.planEnterpriseDescription": {
    message: "맞춤 한도와 지원을 제공합니다.",
    description: "Workspace plan card one-line pitch",
  },
  "workspaces.planEnterpriseBulletLimits": {
    message: "맞춤 한도",
    description: "Enterprise plan custom-limits bullet",
  },
  "workspaces.planEnterpriseBulletSupport": {
    message: "맞춤 지원",
    description: "Enterprise plan custom-support bullet",
  },
  "workspaces.planSelect": {
    message: "요금제 선택",
    description: "Unselected plan card action label",
  },
  "workspaces.planSelected": {
    message: "선택된 요금제",
    description: "Selected plan card action label",
  },
  "workspaces.planUsageBillingNote": {
    message: "서비스와 데이터스토어 사용량은 리소스 등급별로 별도 청구됩니다.",
    description: "Billing note below the workspace plan picker",
  },
  "workspaces.createTitle": {
    message: "워크스페이스 만들기",
    description: "/new/workspace page heading",
  },
  "workspaces.createDescription": {
    message: "세부 정보, 결제 담당자, 요금제, 결제 수단을 선택하세요.",
    description: "/new/workspace page subtitle",
  },
  "workspaces.detailsTitle": {
    message: "워크스페이스 세부 정보",
    description: "/new/workspace details section heading",
  },
  "workspaces.billingEmail": {
    message: "결제 이메일",
    description: "/new/workspace billing email label",
  },
  "workspaces.billingEmailHelp": {
    message: "이 워크스페이스의 영수증과 결제 안내가 이 주소로 전송됩니다.",
    description: "Editable paid-plan billing email help",
  },
  "workspaces.billingEmailHobbyHelp": {
    message: "Hobby 워크스페이스의 결제 이메일은 계정 이메일입니다.",
    description: "Read-only Hobby billing email help",
  },
  "workspaces.billingEmailError": {
    message: "올바른 결제 이메일을 입력하세요.",
    description: "Billing email validation error",
  },
  "workspaces.fieldSlug": {
    message: "워크스페이스 슬러그",
    description: "/new/workspace slug field label",
  },
  "workspaces.fieldSlugHelp": {
    message:
      "CLI와 API에서 사용하는 고유 키입니다. 소문자, 숫자, 하이픈으로 1~30자.",
    description: "/new/workspace slug helper text",
  },
  "workspaces.fieldSlugError": {
    message:
      "소문자, 숫자, 하이픈만 사용할 수 있으며 1~30자여야 하고 하이픈으로 시작하거나 끝날 수 없습니다.",
    description: "Workspace slug validation error",
  },
  "workspaces.paymentTitle": {
    message: "결제 수단",
    description: "/new/workspace payment panel heading",
  },
  "workspaces.paymentDescription": {
    message:
      "결제는 워크스페이스마다 별도입니다. 이 결제 수단은 새 워크스페이스에만 적용됩니다.",
    description: "/new/workspace workspace-specific payment copy",
  },
  "workspaces.paymentRequired": {
    message: "이 워크스페이스에는 결제 수단이 필요합니다.",
    description: "Required payment policy copy",
  },
  "workspaces.paymentOptional": {
    message: "지금 결제 수단을 추가하거나 추가하지 않고 계속하세요.",
    description: "Optional payment policy copy",
  },
  "workspaces.paymentSelfHosted": {
    message: "이 셀프 호스팅 설치에서는 결제 수금이 비활성화되어 있습니다.",
    description: "Billing-off create-flow copy",
  },
  "workspaces.paymentAdd": {
    message: "결제 수단 추가",
    description: "Open Payment Element action",
  },
  "workspaces.paymentSave": {
    message: "결제 수단 저장",
    description: "Confirm SetupIntent action",
  },
  "workspaces.paymentAdded": {
    message: "이 워크스페이스의 결제 수단을 확인했습니다.",
    description: "Successful payment setup state",
  },
  "workspaces.paymentError": {
    message: "해당 결제 수단을 확인하지 못했습니다. 다시 시도하세요.",
    description: "Payment Element fallback error",
  },
  "workspaces.fieldName": {
    message: "이름",
    description: "Workspace name field label (shared by create + settings)",
  },
  "workspaces.fieldDisplayNamePlaceholder": {
    message: "예: Acme Staging",
    description: "/new/workspace freeform name placeholder",
  },
  "workspaces.fieldNamePlaceholder": {
    message: "예: acme-staging",
    description: "Workspace name field placeholder",
  },
  "workspaces.fieldNameError": {
    message: "줄 바꿈 없이 1~100자의 이름을 입력하세요.",
    description: "Workspace name validation error",
  },
  "workspaces.fieldPlan": {
    message: "요금제",
    description: "Workspace plan field label (create picker + settings badge)",
  },
  "workspaces.createErrorTitle": {
    message: "워크스페이스를 만들지 못했습니다",
    description: "/new/workspace inline error alert title",
  },
  "workspaces.createCancel": {
    message: "취소",
    description: "/new/workspace cancel button",
  },
  "workspaces.createCancelError": {
    message: "워크스페이스 생성을 취소하지 못했습니다. 다시 시도하세요.",
    description:
      "/new/workspace toast when cancelling the pending creation fails",
  },
  "workspaces.createSubmit": {
    message: "워크스페이스 만들기",
    description: "/new/workspace submit button",
  },
  "workspaces.createSuccess": {
    message: "{name} 생성됨",
    description: "Toast on a successful workspace create",
  },
  "workspaces.createError": {
    message: "워크스페이스를 만들지 못했습니다",
    description: "Fallback toast/inline message on a failed create",
  },
  "workspaces.creationLoadError": {
    message:
      "워크스페이스 설정을 불러오지 못했습니다. 페이지를 새로 고쳐 다시 시도하세요.",
    description:
      "Inline message when the workspace-creation plan policy or a resumed setup attempt fails to load",
  },
  "workspaces.settingsTitle": {
    message: "워크스페이스 설정",
    description: "Workspace settings page and card title",
  },
  "workspaces.settingsDescription": {
    message:
      "이 워크스페이스의 이름을 바꾸거나 요금제와 메타데이터를 확인합니다.",
    description: "Workspace settings card description",
  },
  "workspaces.settingsEmpty": {
    message: "선택된 워크스페이스가 없습니다.",
    description: "Workspace settings page empty state",
  },
  "workspaces.settingsNavigation": {
    message: "설정 섹션",
    description:
      "Accessible label for the workspace settings section navigation",
  },
  "workspaces.renameSubmit": {
    message: "저장",
    description: "Workspace rename form submit button",
  },
  "workspaces.renameErrorTitle": {
    message: "워크스페이스 이름을 변경하지 못했습니다",
    description: "Workspace rename inline error alert title",
  },
  "workspaces.renameSuccess": {
    message: "이름을 {name}(으)로 변경했습니다",
    description: "Toast on a successful rename",
  },
  "workspaces.renameError": {
    message: "워크스페이스 이름을 변경하지 못했습니다",
    description: "Fallback toast/inline message on a failed rename",
  },
  "workspaces.fieldId": {
    message: "워크스페이스 ID",
    description: "Workspace settings metadata field label",
  },
  "workspaces.fieldCreatedAt": {
    message: "생성일",
    description: "Workspace settings metadata field label",
  },
  "workspaces.dangerZoneTitle": {
    message: "위험 영역",
    description: "Workspace settings delete section title",
  },
  "workspaces.dangerZoneDescription": {
    message:
      "워크스페이스의 모든 리소스와 데이터가 삭제됩니다. 모든 서비스, 데이터스토어, 환경 변수가 사라지며 이 작업은 되돌릴 수 없습니다.",
    description: "Workspace settings delete section description",
  },
  "workspaces.deleteConfirmLabel": {
    message: "확인하려면 아래에 다음을 입력하세요: {phrase}",
    description:
      "Body prompt naming the exact 'sudo delete workspace <name>' phrase (rendered bold by SudoCommandField)",
  },
  "workspaces.deleteErrorTitle": {
    message: "워크스페이스를 삭제하지 못했습니다",
    description: "Delete danger-zone inline error alert title",
  },
  "workspaces.deleteSubmit": {
    message: "워크스페이스 삭제",
    description: "Delete danger-zone submit button",
  },
  "workspaces.deleteSuccess": {
    message: "{name} 삭제됨",
    description: "Toast on a successful delete",
  },
  "workspaces.deleteError": {
    message: "워크스페이스를 삭제하지 못했습니다",
    description: "Fallback toast/inline message on a failed delete",
  },
  "workspaces.changePlanTrigger": {
    message: "요금제 변경",
    description:
      "Workspace settings plan-badge link opening the change-plan dialog",
  },
  "workspaces.changePlanTitle": {
    message: "요금제 변경",
    description: "Change-plan dialog title",
  },
  "workspaces.changePlanDescription": {
    message:
      "이 워크스페이스의 새 요금제를 선택하세요. 결제 단계 없이 요금제가 즉시 변경됩니다.",
    description: "Change-plan dialog description",
  },
  "workspaces.changePlanCancel": {
    message: "취소",
    description: "Change-plan dialog cancel button",
  },
  "workspaces.changePlanSubmit": {
    message: "요금제 변경",
    description: "Change-plan dialog submit button",
  },
  "workspaces.changePlanErrorTitle": {
    message: "요금제를 변경하지 못했습니다",
    description: "Change-plan dialog inline error alert title",
  },
  "workspaces.changePlanSuccess": {
    message: "요금제를 {plan}(으)로 변경했습니다",
    description: "Toast on a successful plan change",
  },
  "workspaces.changePlanError": {
    message: "워크스페이스 요금제를 변경하지 못했습니다",
    description: "Fallback toast/inline message on a failed plan change",
  },
  "workspaces.generalTitle": {
    message: "일반",
    description: "Title of the general card on the workspace settings page",
  },
  "workspaces.ipRulesTitle": {
    message: "인바운드 IP 규칙",
    description:
      "Workspace Settings → Inbound IP Rules card title and section link (w9/m388)",
  },
  "workspaces.ipRulesDescription": {
    message:
      "이 워크스페이스의 모든 Web Service, Static Site, Postgres 데이터베이스, Key Value 인스턴스에 접근할 수 있는 IP 주소를 선택합니다. 이 규칙은 환경 및 서비스 규칙 위에 추가로 적용되므로, 소스는 각 단계에서 모두 허용되어야 합니다. Private Service, Background Worker, Cron Job에는 영향을 주지 않습니다.",
    description:
      "Inbound IP Rules card description: scope and layering of workspace rules (Render's workspace-level inbound IP rules, w9/m388)",
  },
  "workspaces.ipRulesHint": {
    message:
      "CIDR 블록(예: 203.0.113.0/24)을 입력하고 소스 추가를 누르세요. 규칙이 없으면 모든 인바운드 트래픽이 차단되며, 0.0.0.0/0과 ::/0은 모든 IPv4 및 IPv6 소스를 허용합니다.",
    description: "Hint above the editable workspace inbound IP rules (w9/m388)",
  },
  "workspaces.ipRulesBlocked": {
    message:
      "규칙 없음: 이 워크스페이스의 서비스와 데이터스토어로 들어오는 모든 인바운드 트래픽이 차단됩니다. 모든 소스를 허용하려면 0.0.0.0/0과 ::/0을 추가하세요.",
    description:
      "Warning shown when the workspace rule list is empty (deny-all), saved or about to be saved (w9/m388)",
  },
  "workspaces.ipRulesOpen": {
    message: "모든 소스 IP에 열려 있음",
    description:
      "Shown in the read-only view when the workspace rules restrict nothing (Render's allow-all default, w9/m388)",
  },
  "workspaces.ipRulesScalePlanOnly": {
    message:
      "워크스페이스 인바운드 IP 규칙은 Scale 요금제 이상에서 사용할 수 있습니다. 이미 설정된 규칙은 계속 적용되며 모두 허용으로 초기화할 수 있습니다.",
    description:
      "Notice replacing the editable workspace rules below the Scale plan (w9/m388)",
  },
  "workspaces.ipRulesAdminOnly": {
    message: "워크스페이스 관리자만 이 규칙을 변경할 수 있습니다.",
    description:
      "Notice for a member who cannot edit the workspace inbound IP rules, and the forbidden-save explanation (w9/m388)",
  },
  "workspaces.ipRulesPlanRefused": {
    message:
      "이 워크스페이스의 요금제에는 인바운드 IP 규칙이 포함되어 있지 않습니다. 트래픽을 제한하려면 Scale 이상으로 업그레이드하세요. 규칙을 모두 허용으로 초기화하는 것은 언제든 가능합니다.",
    description:
      "Inline explanation when bex-api refuses a workspace rule change with PLAN_FEATURE_REQUIRED (w9/m388)",
  },
  "workspaces.ipRulesUnreadable": {
    message: "이 워크스페이스의 인바운드 IP 규칙을 불러오지 못했습니다.",
    description:
      "Shown in place of the rules when the workspace inbound IP rules read fails (w9/m388)",
  },
  "workspaces.ipRulesResetAllowAll": {
    message: "모두 허용으로 초기화",
    description:
      "Button replacing the workspace inbound IP rules with the allow-all pair below the Scale plan (w9/m388)",
  },
  "workspaces.ipRulesCIDR": {
    message: "새 CIDR 블록",
    description:
      "Accessible label for the add-row CIDR input of the workspace inbound IP rules",
  },
  "workspaces.ipRulesCIDRRule": {
    message: "규칙 {number}의 CIDR 블록",
    description:
      "Accessible label for an existing workspace inbound IP rule CIDR",
  },
  "workspaces.ipRulesDescriptionLabel": {
    message: "새 규칙 설명",
    description:
      "Accessible label for the add-row description input of the workspace inbound IP rules",
  },
  "workspaces.ipRulesDescriptionRule": {
    message: "규칙 {number}의 설명",
    description:
      "Accessible label for an existing workspace inbound IP rule description",
  },
  "workspaces.ipRulesDescriptionPlaceholder": {
    message: "설명(선택 사항)",
    description: "Placeholder for a workspace inbound IP rule description",
  },
  "workspaces.ipRulesAdd": {
    message: "소스 추가",
    description: "Button adding a CIDR to the draft workspace inbound IP rules",
  },
  "workspaces.ipRulesCancel": {
    message: "취소",
    description:
      "Button discarding unsaved changes to the workspace inbound IP rules",
  },
  "workspaces.ipRulesSave": {
    message: "규칙 저장",
    description:
      "Button saving the workspace inbound IP rules (setWorkspaceIPAllowList)",
  },
  "workspaces.ipRulesInvalid": {
    message: "중복되지 않는 올바른 IPv4 또는 IPv6 CIDR 블록을 입력하세요.",
    description:
      "Inline validation when a workspace rule block is not a unique, valid CIDR",
  },
  "workspaces.ipRulesRemove": {
    message: "{cidr} 제거",
    description:
      "Accessible label for a workspace inbound IP rule remove button",
  },
  "workspaces.ipRulesMoveUp": {
    message: "{cidr} 위로 이동",
    description: "Accessible label to move a workspace inbound IP rule earlier",
  },
  "workspaces.ipRulesMoveDown": {
    message: "{cidr} 아래로 이동",
    description: "Accessible label to move a workspace inbound IP rule later",
  },
  "workspaces.ipRulesSaved": {
    message: "인바운드 IP 규칙을 저장했습니다",
    description: "Toast after setWorkspaceIPAllowList succeeds (w9/m388)",
  },
  "workspaces.ipRulesSaveErrorTitle": {
    message: "인바운드 IP 규칙을 저장하지 못했습니다",
    description:
      "Title of the inline alert when a workspace rule change is refused (w9/m388)",
  },
  "workspaces.ipRulesSaveError": {
    message: "인바운드 IP 규칙을 저장하지 못했습니다. 다시 시도하세요.",
    description:
      "Fallback inline message when a workspace rule change fails without a server reason (w9/m388)",
  },
  "workspaces.twoFactorTitle": {
    message: "2단계 인증",
    description:
      "Title of the two-factor card on the workspace settings page, and its quick-nav label (w22/m402)",
  },
  "workspaces.twoFactorDescription": {
    message:
      "2단계 인증(2FA)을 사용하지 않는 멤버는 사용 설정하기 전에는 이 워크스페이스의 리소스나 설정에 접근할 수 없습니다. 그때까지 해당 멤버의 API 키와 SSH 키도 차단됩니다.",
    description:
      "Description under the two-factor card title; follows Render's login-settings wording",
  },
  "workspaces.twoFactorSwitchLabel": {
    message: "2단계 인증(2FA) 강제",
    description:
      "Label of the switch that requires 2FA of every member (Render's control name)",
  },
  "workspaces.twoFactorStatusRequired": {
    message: "필수",
    description:
      "Read-only status shown to a non-admin when the workspace requires 2FA",
  },
  "workspaces.twoFactorStatusNotRequired": {
    message: "필수 아님",
    description:
      "Read-only status shown to a non-admin when the workspace does not require 2FA",
  },
  "workspaces.twoFactorAdminOnly": {
    message: "워크스페이스 관리자만 이 설정을 변경할 수 있습니다.",
    description: "Hint shown to a non-admin in place of the switch",
  },
  "workspaces.twoFactorProPlanOnly": {
    message:
      "2단계 인증 강제는 Pro 요금제 이상에서 사용할 수 있습니다. 이미 강제하고 있는 워크스페이스는 관리자가 끌 때까지 계속 적용됩니다.",
    description:
      "Notice on the two-factor card below the Pro plan, and the refusal for code PLAN_FEATURE_REQUIRED (Render's plan table, w7/070)",
  },
  "workspaces.twoFactorEnabledToast": {
    message: "{name}에서 이제 2단계 인증이 필수입니다",
    description: "Toast after an admin turns the 2FA requirement on",
  },
  "workspaces.twoFactorDisabledToast": {
    message: "{name}에서 더 이상 2단계 인증이 필수가 아닙니다",
    description: "Toast after an admin turns the 2FA requirement off",
  },
  "workspaces.twoFactorErrorTitle": {
    message: "2단계 인증 설정을 변경하지 못했습니다",
    description: "Title of the inline error when the 2FA switch is refused",
  },
  "workspaces.twoFactorError": {
    message: "2단계 인증 설정을 변경하지 못했습니다. 다시 시도하세요.",
    description:
      "Generic message when the 2FA switch fails without a recognised code",
  },
  "workspaces.twoFactorErrorAdminNotEnrolled": {
    message:
      "이 워크스페이스에서 2단계 인증을 요구하기 전에 먼저 본인 계정의 2단계 인증을 켜세요. 그렇지 않으면 본인이 접근할 수 없게 됩니다.",
    description:
      "Refusal for code WORKSPACE_2FA_ADMIN_NOT_ENROLLED: the acting admin has no second factor",
  },
  "workspaces.twoFactorErrorUnavailable": {
    message: "이 설치에서는 2단계 인증 강제를 사용할 수 없어 켤 수 없습니다.",
    description:
      "Refusal for code WORKSPACE_2FA_UNAVAILABLE: the deployment has no identity lookup",
  },
  "workspaces.twoFactorEnrollLink": {
    message: "2단계 인증 설정",
    description:
      "Link to account security, in the admin-not-enrolled refusal and on the blocked page",
  },
  "workspaces.twoFactorBlockedTitle": {
    message: "{workspace}에서 2단계 인증을 요구합니다",
    description: "Heading of the page a blocked member sees (w22/m402)",
  },
  "workspaces.twoFactorBlockedBody": {
    message:
      "이 워크스페이스의 관리자가 모든 멤버에게 2단계 인증(2FA) 사용을 요구합니다. 2단계 인증을 켜기 전에는 이 워크스페이스의 리소스나 설정에 접근할 수 없으며, 본인의 API 키와 SSH 키도 이 워크스페이스에서 사용할 수 없습니다.",
    description: "Why the blocked page is shown",
  },
  "workspaces.twoFactorBlockedRecheck": {
    message: "켰습니다, 다시 확인",
    description:
      "Button on the blocked page that retries the workspace after enrolling",
  },
  "workspaces.twoFactorBlockedRecheckHint": {
    message: "2FA를 켠 후 접근이 복구되기까지 최대 1분이 걸릴 수 있습니다.",
    description: "Hint under the blocked page's check-again button",
  },
  "workspaces.twoFactorBlockedSwitchLabel": {
    message: "다른 워크스페이스로 전환",
    description: "Label of the workspace picker on the blocked page",
  },
  "workspaces.twoFactorBlockedSwitchPlaceholder": {
    message: "워크스페이스 선택",
    description: "Placeholder of the workspace picker on the blocked page",
  },
  "workspaces.twoFactorBlockedNoOtherWorkspaces": {
    message: "다른 워크스페이스 없음",
    description:
      "Placeholder of the blocked page's workspace picker when the member belongs to no other workspace",
  },
  "workspaces.overlappingDeployPolicyTitle": {
    message: "중복 배포 정책",
    description:
      "Workspace Settings section title, Render's name for the setting (w4/653)",
  },
  "workspaces.overlappingDeployPolicyDescription": {
    message:
      "서비스의 다른 배포가 진행 중일 때 새 배포가 트리거되면 어떻게 할지 선택하세요.",
    description:
      "Workspace Settings → Overlapping Deploy Policy description (w4/653)",
  },
  "workspaces.overlappingDeployPolicyWait": {
    message: "대기",
    description: "The Wait policy's label, Render's wording (w4/653)",
  },
  "workspaces.overlappingDeployPolicyWaitDescription": {
    message:
      "진행 중인 배포가 끝나도록 둔 다음, 가장 최근에 트리거된 배포로 바로 넘어갑니다.",
    description:
      "The Wait policy's sentence, Render's wording (render.com/docs/deploys, w4/653)",
  },
  "workspaces.overlappingDeployPolicyOverride": {
    message: "덮어쓰기",
    description: "The Override policy's label, Render's wording (w4/653)",
  },
  "workspaces.overlappingDeployPolicyOverrideDescription": {
    message: "진행 중인 배포를 즉시 취소하고 새 배포를 시작합니다.",
    description:
      "The Override policy's sentence, Render's wording (render.com/docs/deploys, w4/653)",
  },
  "workspaces.overlappingDeployPolicySave": {
    message: "저장",
    description: "Saves the selected Overlapping Deploy Policy (w4/653)",
  },
  "workspaces.overlappingDeployPolicySaving": {
    message: "저장하는 중…",
    description: "The Save button while the policy saves (w4/653)",
  },
  "workspaces.overlappingDeployPolicySaved": {
    message: "중복 배포 정책을 저장했습니다",
    description:
      "Toast after setWorkspaceOverlappingDeployPolicy succeeds (w4/653)",
  },
  "workspaces.overlappingDeployPolicyError": {
    message: "중복 배포 정책을 저장하지 못했습니다. 다시 시도하세요.",
    description:
      "Fallback toast when a policy save fails without a server reason (w4/653)",
  },
  "workspaces.overlappingDeployPolicyAdminOnly": {
    message: "워크스페이스 관리자만 이 정책을 변경할 수 있습니다.",
    description:
      "Shown in place of Save to a member who cannot manage the workspace (w4/653)",
  },
};

export default koWorkspaces;
