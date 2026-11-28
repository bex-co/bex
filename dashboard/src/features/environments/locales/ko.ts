import type { TranslationEntry } from "@/i18n";

const koEnvironments: Record<string, TranslationEntry> = {
  "environments.heading": {
    message: "환경",
    description: "Section heading above a project page's environments list",
  },
  "environments.selectorLabel": {
    message: "선택한 환경",
    description:
      "Accessible label for the Project overview Environment selector",
  },
  "environments.unassignedOption": {
    message: "미할당",
    description: "Project resources that belong to no Environment",
  },
  "environments.newButton": {
    message: "새 환경",
    description: "Button that opens the new-environment dialog",
  },
  "environments.emptyBody": {
    message:
      "아직 환경이 없습니다. staging, production 같은 환경을 만들어 이 프로젝트의 리소스를 그룹으로 묶으세요.",
    description: "Empty state shown when a project has no environments",
  },
  "environments.errorBody": {
    message: "환경을 불러오는 중 문제가 발생했습니다. 잠시 후 다시 시도하세요.",
    description: "Error state shown when the environments query fails",
  },
  "environments.refreshErrorBody": {
    message:
      "환경을 새로 고치지 못했습니다. 마지막으로 불러온 데이터를 표시합니다.",
    description:
      "Inline notice when a background refresh of environments fails while the previously loaded environments stay on screen",
  },
  "environments.resourceCount_other": {
    message: "리소스 {count}개",
    description:
      "Count of visible services+databases+key-value instances assigned to an environment, next to its name (plural)",
  },
  "environments.manageButton": {
    message: "리소스 관리",
    description:
      "Button on an environment card that opens the manage-resources dialog",
  },
  "environments.moreActions": {
    message: "추가 작업",
    description:
      "Accessible label for an environment card's overflow (•••) menu",
  },
  "environments.cardEmpty": {
    message: "이 환경에는 아직 리소스가 없습니다.",
    description:
      "Shown inside an environment card when it has no assigned resources",
  },
  "environments.createTitle": {
    message: "새 환경",
    description: "New-environment dialog title",
  },
  "environments.createDescription": {
    message:
      "이 프로젝트 리소스의 일부를 staging, production 같은 이름으로 그룹화합니다.",
    description: "New-environment dialog description",
  },
  "environments.fieldName": {
    message: "이름",
    description: "New-environment dialog name field label",
  },
  "environments.fieldNamePlaceholder": {
    message: "staging",
    description: "New-environment dialog name field placeholder",
  },
  "environments.cancel": {
    message: "취소",
    description: "Cancel button shared by the environment dialogs",
  },
  "environments.createSubmit": {
    message: "환경 생성",
    description: "New-environment dialog submit button",
  },
  "environments.createSuccess": {
    message: "환경 “{name}”을(를) 생성했습니다.",
    description: "Toast shown after an environment is created",
  },
  "environments.createRefreshFailed": {
    message:
      "환경 “{name}”을(를) 생성했지만 목록을 새로 고치지 못했습니다. 다시 시도하거나 페이지를 새로 고쳐 확인하세요.",
    description:
      "Warning after the environment was created but re-reading the list failed; creation must not be retried or reported as failed (w4/231)",
  },
  "environments.createError": {
    message: "환경 “{name}”을(를) 생성하지 못했습니다.",
    description: "Toast shown when creating an environment fails",
  },
  "environments.deleteSuccess": {
    message: "환경 {name}을(를) 삭제했습니다.",
    description:
      "Toast shown after an environment is deleted (Render's words, w1/m683)",
  },
  "environments.deleteRefreshFailed": {
    message:
      "환경 “{name}”을(를) 삭제했지만 목록을 새로 고치지 못했습니다. 다시 시도하거나 페이지를 새로 고쳐 업데이트하세요.",
    description:
      "Warning after the environment was deleted but re-reading the list failed; deletion must not be retried or reported as failed (w4/290)",
  },
  "environments.manageTitle": {
    message: "“{name}”의 리소스 관리",
    description: "Manage-resources dialog title",
  },
  "environments.manageDescription": {
    message:
      "이 환경에 속할 리소스를 선택하세요. 리소스를 할당하면 이 프로젝트에도 추가됩니다.",
    description: "Manage-resources dialog description",
  },
  "environments.tabServices": {
    message: "서비스",
    description: "Manage-resources dialog tab label for the services checklist",
  },
  "environments.tabDatabases": {
    message: "데이터베이스",
    description:
      "Manage-resources dialog tab label for the databases checklist",
  },
  "environments.tabKeyValues": {
    message: "Key Value",
    description:
      "Manage-resources dialog tab label for the key-value checklist",
  },
  "environments.manageNoServices": {
    message: "이 워크스페이스에는 아직 할당할 서비스가 없습니다.",
    description:
      "Manage-resources dialog empty state when the workspace has no services",
  },
  "environments.manageNoDatabases": {
    message: "이 워크스페이스에는 아직 할당할 데이터베이스가 없습니다.",
    description:
      "Manage-resources dialog empty state when the workspace has no databases",
  },
  "environments.manageNoKeyValues": {
    message: "이 워크스페이스에는 아직 할당할 Key Value 인스턴스가 없습니다.",
    description:
      "Manage-resources dialog empty state when the workspace has no key-value instances",
  },
  "environments.manageResourceIn": {
    message: "{environment}에 있음",
    description:
      "Under a Manage resources row: the other environment the resource is in now ({environment} = its name)",
  },
  "environments.markerProtected": {
    message: "보호됨",
    description:
      "Marker: the environment the resource would leave is protected",
  },
  "environments.markerIsolated": {
    message: "격리됨",
    description:
      "Marker: the environment the resource would leave has network isolation",
  },
  "environments.markerAllowList": {
    message: "IP 허용 목록",
    description:
      "Marker: the environment the resource would leave restricts inbound IPs",
  },
  "environments.manageMovesOut": {
    message: "{environment}에서 이동: {resources}",
    description:
      "Before Save: the selected resources this save moves out of another environment ({resources} = comma-separated names)",
  },
  "environments.manageNeedsAdminToMove": {
    message: "관리자만 {environment}에서 이동할 수 있습니다.",
    description:
      "Why a Manage resources row is disabled: its current environment is admin-managed",
  },
  "environments.manageUnsaved": {
    message: "저장되지 않은 변경 사항",
    description:
      "Accessible description of a Manage resources tab whose selection differs from the saved membership",
  },
  "environments.manageSubmit": {
    message: "저장",
    description: "Manage-resources dialog submit button",
  },
  "environments.assignSuccess": {
    message: "“{name}”의 서비스를 업데이트했습니다.",
    description: "Toast shown after an environment's services are updated",
  },
  "environments.strandedMoveTitle": {
    message: "이 이동으로 연결된 그룹이 남게 됩니다",
    description:
      "Confirmation title shown before a move that strands an environment-scoped group link (w4/m238)",
  },
  "environments.strandedMoveBody": {
    message:
      "다음 환경 그룹은 이동하는 서비스에 계속 연결되어 있으며, 그룹의 연결을 해제하거나 이동하기 전까지 변수와 파일이 계속 적용됩니다:",
    description:
      "Confirmation body: the stranded groups keep feeding the service after the move",
  },
  "environments.strandedMoveConfirm": {
    message: "그래도 이동",
    description: "Confirm button on the stranded-groups move confirmation",
  },
  "environments.strandedScopeWorkspace": {
    message: "워크스페이스",
    description:
      "Scope label for a stranded group that is scoped to the whole workspace",
  },
  "environments.strandedScopeUnknown": {
    message:
      "연결된 환경 그룹을 아직 확인하지 못했습니다. 목록이 로드될 때까지 기다린 후 다시 시도하세요.",
    description:
      "Toast shown when a move is attempted before the group/scope reads resolve",
  },
  "environments.assignError": {
    message: "“{name}”의 서비스를 업데이트하지 못했습니다.",
    description: "Toast shown when updating an environment's services fails",
  },
  "environments.assignDatabasesSuccess": {
    message: "“{name}”의 데이터베이스를 업데이트했습니다.",
    description: "Toast shown after an environment's databases are updated",
  },
  "environments.assignDatabasesError": {
    message: "“{name}”의 데이터베이스를 업데이트하지 못했습니다.",
    description: "Toast shown when updating an environment's databases fails",
  },
  "environments.assignDatabasesRefreshFailed": {
    message:
      "“{name}”의 데이터베이스를 업데이트했지만 목록을 새로 고치지 못했습니다. 다시 시도하거나 페이지를 새로 고쳐 확인하세요.",
    description:
      "Warning after the database membership was saved but re-reading the environments list failed; the write must not be retried or reported as failed (w4/325)",
  },
  "environments.assignKeyValuesSuccess": {
    message: "“{name}”의 Key Value 인스턴스를 업데이트했습니다.",
    description:
      "Toast shown after an environment's key-value instances are updated",
  },
  "environments.assignKeyValuesRefreshFailed": {
    message:
      "“{name}”의 Key Value 인스턴스를 업데이트했지만 목록을 새로 고치지 못했습니다. 다시 시도하거나 페이지를 새로 고쳐 확인하세요.",
    description:
      "Warning after the key-value membership was saved but re-reading the environments list failed; the write must not be retried or reported as failed (w4/263)",
  },
  "environments.assignKeyValuesError": {
    message: "“{name}”의 Key Value 인스턴스를 업데이트하지 못했습니다.",
    description:
      "Toast shown when updating an environment's key-value instances fails",
  },
  "environments.tabEnvGroups": {
    message: "환경 그룹",
    description: "Manage-resources dialog tab label for environment groups",
  },
  "environments.manageNoEnvGroups": {
    message: "이 워크스페이스에는 아직 할당할 환경 그룹이 없습니다.",
    description: "Environment-groups checklist empty state",
  },
  "environments.tabSandboxGroups": {
    message: "샌드박스 그룹",
    description:
      "Manage-resources dialog tab label for the workspace's sandbox groups",
  },
  "environments.manageNoSandboxGroups": {
    message: "이 워크스페이스에는 아직 할당할 샌드박스 그룹이 없습니다.",
    description: "Sandbox-groups checklist empty state",
  },
  "environments.assignEnvGroupsSuccess": {
    message: "“{name}”의 환경 그룹을 업데이트했습니다.",
    description: "Toast shown after environment-group membership is updated",
  },
  "environments.assignEnvGroupsPlacementUnread": {
    message:
      "“{name}”의 그룹이 현재 어디에 있는지 확인하지 못했습니다. 그룹을 다시 이동하기 전에 다시 시도하거나 페이지를 새로 고치세요.",
    description:
      "Warning after an environment-group move failed and re-reading the groups' placement failed too; its action re-reads, it never moves again (w4/m206)",
  },
  "environments.assignEnvGroupsError": {
    message: "“{name}”의 환경 그룹을 업데이트하지 못했습니다.",
    description:
      "Toast shown when environment-group membership fails to update",
  },
  "environments.settingsAction": {
    message: "모든 설정",
    description: "Environment overflow-menu action opening ACL settings",
  },
  "environments.protectedBadge": {
    message: "보호됨",
    description: "Badge shown beside a protected environment name",
  },
  "environments.ipAllowListLabel": {
    message: "인바운드 IP 제한",
    description: "Environment inbound-IP allowlist heading",
  },
  "environments.ipAllowListHint": {
    message:
      "나열된 CIDR 범위만 이 환경의 공개 서비스, 정적 사이트, 데이터 저장소에 접근할 수 있습니다. 소스는 각 리소스의 자체 IP 규칙도 통과해야 합니다.",
    description: "Environment inbound-IP allowlist explanation",
  },
  "environments.ipAllowListOpen": {
    message:
      "규칙이 없습니다. 이 환경의 공개 멤버는 모든 트래픽을 거부합니다. 모두 허용하려면 0.0.0.0/0과 ::/0을 추가하세요.",
    description: "Empty environment inbound-IP allowlist state (deny-all)",
  },
  "environments.ipAllowListRemove": {
    message: "{cidr} 제거",
    description: "Accessible label for an inbound-IP rule remove button",
  },
  "environments.ipAllowListRuleCIDR": {
    message: "규칙 {number}의 CIDR 블록",
    description: "Accessible label for an existing inbound-IP rule CIDR",
  },
  "environments.ipAllowListRuleDescription": {
    message: "규칙 {number}의 설명",
    description: "Accessible label for an existing inbound-IP rule description",
  },
  "environments.ipAllowListNewCIDR": {
    message: "새 CIDR 블록",
    description: "Accessible label for a new inbound-IP rule CIDR",
  },
  "environments.ipAllowListNewDescription": {
    message: "새 규칙 설명",
    description: "Accessible label for a new inbound-IP rule description",
  },
  "environments.ipAllowListDescriptionPlaceholder": {
    message: "설명 (선택 사항)",
    description: "Placeholder for an inbound-IP rule description",
  },
  "environments.ipAllowListScalePlanOnly": {
    message:
      "환경의 인바운드 IP 규칙은 Scale 요금제 이상에서 사용할 수 있습니다. 이미 설정된 규칙은 계속 적용되며 초기화할 수 있습니다.",
    description:
      "Hint replacing the editable IP rules below the Scale plan (w8/m137)",
  },
  "environments.ipAllowListResetAllowAll": {
    message: "모두 허용으로 초기화",
    description:
      "Button resetting an environment's inbound IP rules to allow-all below the Scale plan (w8/m137)",
  },
  "environments.ipAllowListInvalid": {
    message: "중복되지 않는 올바른 IPv4 또는 IPv6 CIDR 블록을 입력하세요.",
    description:
      "Inline validation when an environment allow list block is not a unique, valid CIDR",
  },
  "environments.ipAllowListAdd": {
    message: "소스 추가",
    description: "Button adding an inbound-IP CIDR rule",
  },
  "environments.resourceCountLoading": {
    message: "집계 중…",
    description:
      "Environment card header while its resource lists are still loading, instead of a count",
  },
  "environments.resourceCountUnknown": {
    message: "개수를 확인할 수 없음",
    description:
      "Environment card header when a resource list has not answered, instead of a count",
  },
  "environments.resourcesUnavailable": {
    message:
      "일부 리소스를 불러오지 못해 이 환경이 불완전하거나 최신 상태가 아닐 수 있습니다.",
    description:
      "Inline alert on a Project environment card when a resource list failed to load; offers Try again",
  },
  "environments.resourcesLoading": {
    message: "리소스를 불러오는 중…",
    description:
      "Environment card line while its resource lists are still loading, instead of the empty message",
  },
  "environments.assignmentTitle": {
    message: "프로젝트",
    description: "Shared create-form Project/Environment selector heading",
  },
  "environments.assignmentProject": {
    message: "프로젝트",
    description: "Accessible label for a create-form Project selector",
  },
  "environments.assignmentProjectNone": {
    message: "프로젝트 없음",
    description: "Unassigned Project selector option",
  },
  "environments.assignmentEnvironment": {
    message: "환경",
    description: "Accessible label for a create-form Environment selector",
  },
  "environments.assignmentEnvironmentNone": {
    message: "환경 없음",
    description: "Unassigned Environment selector option",
  },
  "environments.assignmentSelectedLoading": {
    message: "불러오는 중…",
    description:
      "Label for a kept Project/Environment selection while its list is still loading",
  },
  "environments.assignmentSelectedUnavailable": {
    message: "선택됨 (이름을 사용할 수 없음)",
    description:
      "Label for a kept Project/Environment selection whose list failed to load",
  },
  "environments.assignmentReadError": {
    message:
      "프로젝트 또는 환경을 불러오지 못했습니다. 다시 시도하세요. 선택한 항목은 유지됩니다.",
    description:
      "Inline alert when the create form's Project or Environment list fails to load; the selection is kept",
  },
  "environments.assignmentHint": {
    message:
      "리소스를 생성할 때 프로젝트를 선택하면 해당 프로젝트의 환경만 표시됩니다. 환경을 선택하면 리소스가 그 프로젝트에 추가됩니다. 환경 없음을 선택하면 리소스가 어떤 프로젝트에도 속하지 않게 생성됩니다. 프로젝트에 아직 환경이 없다면 먼저 프로젝트 페이지에서 환경을 만드세요.",
    description: "Shared create-form Environment-assignment hint",
  },
  "environments.protectedPlacementAdminOnly": {
    message:
      "보호된 환경에 리소스를 생성하거나 그 환경으로 또는 그 환경에서 리소스를 이동하는 작업은 관리자만 할 수 있습니다.",
    description:
      "Why a create or move control that targets (or leaves) a protected environment is disabled for a non-admin, and the message for the same server refusal (w3/m281)",
  },
  "environments.protectedOptionAdminOnly": {
    message: "{environment} (관리자 전용)",
    description:
      "A protected environment in a create or move picker that a non-admin cannot choose ({environment} = its name)",
  },
  "environments.assignmentDescription": {
    message: "생성 후 프로젝트와 환경에 추가합니다.",
    description: "Shared create-form Project row description",
  },
  "environments.assignmentNoEnvironments": {
    message:
      "이 프로젝트에는 아직 환경이 없습니다. 먼저 프로젝트 페이지에서 환경을 만드세요.",
    description:
      "Shared create-form note when the chosen Project has no Environment to add the resource to",
  },
  "environments.assignmentPickEnvironment": {
    message:
      "이 프로젝트에 추가하려면 환경을 선택하세요. '환경 없음'을 선택하면 어떤 프로젝트에도 속하지 않은 채 생성됩니다.",
    description:
      "Shared create-form note when a Project is chosen but No environment is: only an Environment adds the resource to the Project",
  },
  "environments.assignmentEnvironmentPickProject": {
    message: "먼저 프로젝트를 선택하세요",
    description:
      "Shared create-form Environment select while no Project is chosen (it is disabled until one is)",
  },
  "environments.protectedDescription": {
    message: "보호된 환경",
    description:
      "Line under a protected environment's section heading on the project page (Render's words, w1/m682)",
  },
  "environments.emptyTitle": {
    message: "{name} 환경이 비어 있습니다",
    description:
      "Heading of an environment section with no services, Postgres or Key Value; {name} is the environment (Render's words, w1/m682)",
  },
  "environments.emptyBodyCreate": {
    message: "새 서비스를 만들어 환경을 시작하세요.",
    description:
      "An empty environment's sentence when the workspace has nothing to move into it (Render's words, w1/m682)",
  },
  "environments.emptyBodyCreateOrMove": {
    message: "새 서비스를 만들거나 기존 서비스를 옮겨 환경을 시작하세요.",
    description:
      "An empty environment's sentence when the workspace has resources that could move into it (Render's words, w1/m682)",
  },
  "environments.createNewService": {
    message: "새 서비스 만들기",
    description:
      "An empty environment's button to the create flow with the project and environment chosen (Render's words, w1/m682)",
  },
  "environments.moveExistingServices": {
    message: "기존 서비스 옮기기",
    description:
      "An empty environment's button that moves existing resources into it (Render's words, w1/m682)",
  },
  "environments.noEnvGroups": {
    message: "이 환경에는 환경 그룹이 없습니다.",
    description:
      "An environment section's Env Groups view when the environment has none (Render's words, w1/m682)",
  },
  "environments.unassignedDescription": {
    message: "이 프로젝트에서 어떤 환경에도 속하지 않은 리소스입니다.",
    description:
      "Line under the project page's Unassigned section, which lists the project's resources in no environment (a bex section; Render requires an environment)",
  },
  "environments.settingsPageTitle": {
    message: "환경 설정",
    description:
      "Heading of an environment's settings page, /project/<id>/environment/<envId>/settings (Render's words, w1/m683)",
  },
  "environments.settingsNavigation": {
    message: "환경 설정 섹션",
    description:
      "Accessible name of the environment settings page's section navigation (w1/m683)",
  },
  "environments.settingsEdit": {
    message: "편집",
    description:
      "Button on an environment settings card that makes its field editable (Render's words, w1/m683)",
  },
  "environments.settingsCardSave": {
    message: "저장",
    description:
      "Button that saves an environment settings card's change (Render's words, w1/m683)",
  },
  "environments.settingsUpdateError": {
    message: "환경을 업데이트하는 중 오류가 발생했습니다. 다시 시도하세요.",
    description:
      "Inline error on an environment settings card when the save failed for a reason the server did not name (Render's words, w1/m683)",
  },
  "environments.nameCardTitle": {
    message: "이름",
    description:
      "Title of the environment settings page's Name card, and its navigation entry (Render's words, w1/m683)",
  },
  "environments.nameCardDescription": {
    message: "환경의 고유한 이름",
    description:
      "Description of the environment settings page's Name card (Render's words, w1/m683)",
  },
  "environments.nameFieldLabel": {
    message: "환경 이름",
    description:
      "Accessible label of the Name card's field (Render's words, w1/m683)",
  },
  "environments.nameRequired": {
    message: "환경 이름은 필수입니다",
    description:
      "Error under the Name card's field when it is saved empty (Render's words, w1/m683)",
  },
  "environments.nameInUse": {
    message: "이 프로젝트의 다른 환경에서 이미 사용 중인 이름입니다",
    description:
      "Error under the Name card's field when another environment of the project has the name (Render's words, w1/m683)",
  },
  "environments.permissionsCardTitle": {
    message: "권한",
    description:
      "Title of the environment settings page's Permissions card, and its navigation entry (Render's words, w1/m683)",
  },
  "environments.permissionsDefaultBody": {
    message:
      "기본적으로 모든 팀 구성원이 리소스를 삭제 및 일시 중지하고, 서비스를 이동하고, 데이터베이스 네트워킹을 관리할 수 있습니다.",
    description:
      "First paragraph of the Permissions card (Render's words, w1/m683)",
  },
  "environments.permissionsProtectedBody": {
    message:
      "{protectedEnvironment}으로 지정하면 관리자만 리소스를 삭제 및 일시 중지하고, 서비스를 이동하고, 데이터베이스 네트워킹을 관리할 수 있습니다.",
    description:
      "Second paragraph of the Permissions card; {protectedEnvironment} is a link whose text is environments.permissionsProtectedLink (Render's words, w1/m683)",
  },
  "environments.permissionsProtectedLink": {
    message: "보호된 환경",
    description:
      "Link text inside environments.permissionsProtectedBody, to the projects and environments docs (Render's words, w1/m683)",
  },
  "environments.permissionsLegend": {
    message: "환경 권한",
    description:
      "Accessible name of the Permissions card's Default / Protected choice (Render's words, w1/m683)",
  },
  "environments.permissionsDefault": {
    message: "기본",
    description:
      "The Permissions card's option for an unprotected environment (Render's words, w1/m683)",
  },
  "environments.permissionsDefaultHint": {
    message: "스테이징 및 개발용",
    description:
      "Line under the Permissions card's Default option (Render's words, w1/m683)",
  },
  "environments.permissionsProtected": {
    message: "보호됨",
    description:
      "The Permissions card's option for a protected environment (Render's words, w1/m683)",
  },
  "environments.permissionsProtectedHint": {
    message: "프로덕션에 적합",
    description:
      "Line under the Permissions card's Protected option (Render's words, w1/m683)",
  },
  "environments.removeRestrictionsTitle": {
    message: "제한 해제",
    description:
      "Title of the dialog that confirms unprotecting an environment, or allowing its cross-environment connections again (Render's words, w1/m683)",
  },
  "environments.removeProtectionBody": {
    message:
      "{name} 환경의 제한을 해제하면 모든 팀 구성원이 파괴적인 작업을 수행할 수 있습니다. 정말 진행하시겠습니까?",
    description:
      "Body of the dialog that confirms unprotecting an environment; {name} is the environment (Render's words, w1/m683)",
  },
  "environments.removeRestrictionsConfirm": {
    message: "예, 제한을 해제합니다",
    description:
      "Confirm button that unprotects an environment, or allows its cross-environment connections again (Render's words, w1/m683)",
  },
  "environments.connectionsNavLabel": {
    message: "연결",
    description:
      "Navigation entry of the environment settings page's Cross-Environment Connections card (Render's words, w1/m683)",
  },
  "environments.connectionsCardTitle": {
    message: "환경 간 연결",
    description:
      "Title of the environment settings page's connections card (Render's words, w1/m683)",
  },
  "environments.connectionsBody": {
    message:
      "이 환경의 서비스 간 트래픽은 허용하면서 프라이빗 네트워크 트래픽이 환경 경계를 넘지 못하게 합니다.",
    description:
      "First paragraph of the Cross-Environment Connections card (Render's words, w1/m683)",
  },
  "environments.connectionsBodyOutside": {
    message: "이 환경의 서비스는 환경 외부의 bex 서비스와 통신할 수 없습니다.",
    description:
      "Second paragraph of the Cross-Environment Connections card (Render's words, with bex for Render, w1/m683)",
  },
  "environments.connectionsLegend": {
    message: "환경 간 연결",
    description:
      "Accessible name of the Cross-Environment Connections card's Allowed / Blocked choice (w1/m683)",
  },
  "environments.connectionsAllowed": {
    message: "허용됨",
    description:
      "The Cross-Environment Connections card's option for an environment whose private traffic may cross its boundary (Render's words, w1/m683)",
  },
  "environments.connectionsAllowedHint": {
    message: "새 환경의 기본값",
    description:
      "Line under the Cross-Environment Connections card's Allowed option (Render's words, w1/m683)",
  },
  "environments.connectionsBlocked": {
    message: "차단됨",
    description:
      "The Cross-Environment Connections card's option for an isolated environment (Render's words, w1/m683)",
  },
  "environments.connectionsBlockedHint": {
    message: "프로덕션 환경에 권장",
    description:
      "Line under the Cross-Environment Connections card's Blocked option (Render's words, w1/m683)",
  },
  "environments.connectionsPaywall": {
    message:
      "{plan} 요금제로 업그레이드하면 환경 간 연결을 차단할 수 있습니다.",
    description:
      "Notice on the Cross-Environment Connections card below Pro; {plan} is the Pro plan's name (Render's words, w1/m683)",
  },
  "environments.upgrade": {
    message: "업그레이드",
    description:
      "Button or link that opens Render's plan page from an environment's plan gate (Render's words, w1/m683, w3/m708)",
  },
  "environments.isolateTitle": {
    message: "연결 제한",
    description:
      "Title of the dialog that confirms blocking an environment's cross-environment connections, and its confirm button (Render's words, w1/m683)",
  },
  "environments.isolateDescription": {
    message: "{environment}의 연결을 격리할지 확인하세요",
    description:
      "Description of the dialog that confirms blocking or allowing an environment's cross-environment connections; {environment} is <project> / <environment> (Render's words, w1/m683)",
  },
  "environments.isolateRestrictBody": {
    message:
      "이 환경 외부의 bex 서비스와 주고받는 프라이빗 네트워크 트래픽을 차단합니다. 경계를 넘는 트래픽에 의존하는 서비스가 중단될 수 있습니다.",
    description:
      "First paragraph of the dialog that confirms blocking cross-environment connections (Render's words, with bex for Render, w1/m683)",
  },
  "environments.isolateRestrictRestart": {
    message:
      "이미 열려 있는 네트워크 연결은 영향을 받지 않습니다. 기존 연결을 모두 닫으려면 이 환경의 서비스를 다시 시작하세요.",
    description:
      "Second paragraph of the dialog that confirms blocking cross-environment connections (Render's words, w1/m683)",
  },
  "environments.isolateRemoveBody": {
    message:
      "이 환경 외부의 bex 서비스와 주고받는 프라이빗 네트워크 트래픽이 더 이상 차단되지 않습니다.",
    description:
      "Body of the dialog that confirms allowing cross-environment connections again (Render's words, with bex for Render, w1/m683)",
  },
  "environments.isolateQuestion": {
    message: "정말 진행하시겠습니까?",
    description:
      "Last line of the dialog that confirms blocking or allowing cross-environment connections (Render's words, w1/m683)",
  },
  "environments.isolationBlockedToast": {
    message: "{name}의 환경 간 연결을 차단했습니다.",
    description:
      "Toast after the section menu's toggle blocks an environment's cross-environment connections (Render's words, w1/m683)",
  },
  "environments.isolationAllowedToast": {
    message: "{name}의 환경 간 연결을 허용했습니다.",
    description:
      "Toast after the section menu's toggle allows an environment's cross-environment connections again (Render's words, w1/m683)",
  },
  "environments.menuBlockConnections": {
    message: "환경 간 연결 차단",
    description:
      "The environment section's ••• menu toggle that isolates the environment (Render's words, w1/m683)",
  },
  "environments.menuBlockConnectionsHint": {
    message: "프라이빗 네트워크 트래픽이 환경 경계를 넘지 못하게 합니다.",
    description:
      "Line under the ••• menu's Block cross-environment connections toggle (Render's words, w1/m683)",
  },
  "environments.menuUpgradeToUnlock": {
    message: "{upgrade}하여 이 기능을 사용하세요",
    description:
      "The ••• menu's plan gate on Block cross-environment connections below Pro; {upgrade} is a link whose text is environments.upgrade (Render's words, w1/m683)",
  },
  "environments.envGroupsCardTitle": {
    message: "환경 그룹",
    description:
      "Title of the environment settings page's Environment Groups card, and its navigation entry (Render's words, w1/m683)",
  },
  "environments.envGroupsNew": {
    message: "새 환경 그룹",
    description:
      "Button on the Environment Groups card that creates an env group in the environment (Render's words, w1/m683)",
  },
  "environments.envGroupsSearch": {
    message: "환경 그룹 검색",
    description:
      "Placeholder and accessible name of the Environment Groups card's search (Render's words, w1/m683)",
  },
  "environments.envGroupsNone": {
    message: "환경 그룹을 찾을 수 없습니다",
    description:
      "The Environment Groups card when the environment has no env groups (Render's words, w1/m683)",
  },
  "environments.networkingCardTitle": {
    message: "네트워킹",
    description:
      "Title of the environment settings page's Networking card (inbound IP rules), and its navigation entry (Render's words, w1/m683)",
  },
  "environments.ipAllowListMoveUp": {
    message: "{cidr} 위로 이동",
    description:
      "Accessible name of an environment IP rule's move-up button; {cidr} is the rule's CIDR block",
  },
  "environments.ipAllowListMoveDown": {
    message: "{cidr} 아래로 이동",
    description:
      "Accessible name of an environment IP rule's move-down button; {cidr} is the rule's CIDR block",
  },
  "environments.ipAllowListAllowsAll": {
    message: "모든 소스 IP에 개방",
    description:
      "The Networking card below Scale when the environment's rules are the allow-all pair",
  },
  "environments.ipAllowListSaved": {
    message: "{name}의 인바운드 IP 규칙을 저장했습니다.",
    description:
      "Toast after the Networking card saves an environment's inbound IP rules",
  },
  "environments.deleteCardTitle": {
    message: "환경 삭제",
    description:
      "Title of the environment settings page's delete card, and its navigation entry (Render's words, w1/m683)",
  },
  "environments.deleteCardDescription": {
    message:
      "이 환경과 그 안의 서비스가 영구적으로 삭제됩니다. 이 작업은 되돌릴 수 없습니다.",
    description:
      "Description of the Delete Environment card (Render's words, w1/m683)",
  },
  "environments.deleteLastEnvironment": {
    message:
      "프로젝트에는 환경이 하나 이상 있어야 하므로 이 환경은 삭제할 수 없습니다. 대신 {project}를 삭제하는 것을 고려하세요.",
    description:
      "Notice on the Delete Environment card for a project's only environment; {project} is a link whose text is environments.deleteLastEnvironmentProject (Render's words, w1/m683)",
  },
  "environments.deleteLastEnvironmentProject": {
    message: "프로젝트",
    description:
      "Link text inside environments.deleteLastEnvironment, to the project's settings (Render's words, w1/m683)",
  },
  "environments.deleteButton": {
    message: "환경 삭제",
    description:
      "The Delete Environment card's button, the confirmation dialog's title and its confirm button (Render's words, w1/m683)",
  },
  "environments.deleteDialogBody": {
    message:
      "이 환경은 모든 서비스와 함께 영구적으로 삭제됩니다. 유지하려는 서비스가 있다면 먼저 환경 밖으로 옮기세요.",
    description:
      "Body of the dialog that confirms deleting an environment (Render's words, w1/m683)",
  },
  "environments.deleteDialogEnvGroupsKept": {
    message:
      "이 환경의 환경 그룹은 삭제되지 않고 미할당 상태가 됩니다. 더 이상 이 환경이나 프로젝트에 속하지 않으며 환경 그룹 아래에 그대로 남습니다.",
    description:
      "Added to the delete dialog's body when the environment holds env groups, which bex keeps (w1/m683)",
  },
  "environments.deleteFailed": {
    message: "환경을 삭제하지 못했습니다. 잠시 후 다시 시도하세요. 죄송합니다!",
    description:
      "Error in the delete dialog when the delete failed for a reason the server did not name (Render's words, w1/m683)",
  },
  "environments.deleteCardDenied": {
    message: "환경 삭제",
    description:
      "Title of the card that replaces Delete Environment for a member who may not delete the environment (Render's words, w1/m683)",
  },
  "environments.protectedDeleteAdminOnly": {
    message:
      "보호된 환경 또는 네트워크 격리나 인바운드 IP 규칙이 있는 환경은 관리자만 삭제할 수 있습니다.",
    description:
      "Why a non-Admin cannot delete an environment that is protected, isolated or has inbound IP rules (w1/m683)",
  },
};

export default koEnvironments;
