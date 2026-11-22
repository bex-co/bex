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
  "environments.renameAction": {
    message: "이름 변경",
    description: "Environment overflow-menu item that opens the rename dialog",
  },
  "environments.deleteAction": {
    message: "삭제",
    description:
      "Environment overflow-menu item that opens the delete confirmation, and the confirm button",
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
  "environments.renameFieldLabel": {
    message: "환경 이름",
    description:
      'Accessible/visible label for the environment rename field. Deliberately more specific than environments.fieldName\'s bare "Name" (w4/134)',
  },
  "environments.renameTitle": {
    message: "환경 이름 변경",
    description: "Rename-environment dialog title",
  },
  "environments.renameSubmit": {
    message: "저장",
    description: "Rename-environment dialog submit button",
  },
  "environments.renameSuccess": {
    message: "환경 이름을 “{name}”(으)로 변경했습니다.",
    description: "Toast shown after an environment is renamed",
  },
  "environments.renameError": {
    message: "환경 이름을 “{name}”(으)로 변경하지 못했습니다.",
    description: "Toast shown when renaming an environment fails",
  },
  "environments.deleteConfirmTitle": {
    message: "환경 “{name}”을(를) 삭제할까요?",
    description: "Delete-environment confirmation dialog title",
  },
  "environments.deleteConfirmBody": {
    message:
      "이 환경의 환경 그룹은 삭제되지 않고 미할당 상태가 됩니다. 더 이상 이 환경이나 프로젝트에 속하지 않으며 환경 그룹 아래에 그대로 남습니다. 이 작업은 되돌릴 수 없습니다.",
    description: "Delete-environment confirmation dialog body",
  },
  "environments.deleteConfirmBodyResources": {
    message:
      "이 환경을 삭제하면 모든 서비스, 데이터베이스, Key Value 인스턴스가 삭제됩니다: {resources}. 이 환경의 환경 그룹은 삭제되지 않고 미할당 상태가 됩니다. 더 이상 이 환경이나 프로젝트에 속하지 않으며 환경 그룹 아래에 그대로 남습니다. 이 작업은 되돌릴 수 없습니다.",
    description:
      "Delete-environment confirmation body when services, databases or key value stores would be deleted (w4/m264, w6/m290, Render parity)",
  },
  "environments.deleteSuccess": {
    message: "환경 “{name}”을(를) 삭제했습니다.",
    description: "Toast shown after an environment is deleted",
  },
  "environments.deleteRefreshFailed": {
    message:
      "환경 “{name}”을(를) 삭제했지만 목록을 새로 고치지 못했습니다. 다시 시도하거나 페이지를 새로 고쳐 업데이트하세요.",
    description:
      "Warning after the environment was deleted but re-reading the list failed; deletion must not be retried or reported as failed (w4/290)",
  },
  "environments.deleteError": {
    message: "환경 “{name}”을(를) 삭제하지 못했습니다.",
    description: "Toast shown when deleting an environment fails",
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
  "environments.settingsTitle": {
    message: "“{name}” 설정",
    description: "Environment ACL settings dialog title",
  },
  "environments.settingsDescription": {
    message: "권한, 프라이빗 네트워크 격리, 인바운드 IP 규칙을 관리합니다.",
    description: "Environment ACL settings dialog description",
  },
  "environments.protectedBadge": {
    message: "보호됨",
    description: "Badge shown beside a protected environment name",
  },
  "environments.protectedLabel": {
    message: "보호된 환경",
    description: "Protected-status setting label",
  },
  "environments.protectedHint": {
    message:
      "워크스페이스 관리자만 이 환경의 리소스를 삭제, 일시 중지, 재개하거나 유지 보수 모드를 켤 수 있습니다.",
    description: "Protected-status setting explanation",
  },
  "environments.isolationLabel": {
    message: "환경 간 연결 차단",
    description: "Private-network isolation setting label",
  },
  "environments.isolationProOnly": {
    message:
      "네트워크 격리는 Pro 요금제 이상에서 사용할 수 있습니다. 이전에 활성화했다면 여기서 끌 수 있습니다.",
    description: "Isolation setting explanation on a Hobby workspace (w8/m138)",
  },
  "environments.isolationHint": {
    message:
      "Postgres 및 Key Value 인스턴스를 포함해 프라이빗 네트워크 트래픽이 이 환경의 경계를 넘지 못하게 합니다. 외부 URL은 계속 해당 IP 규칙을 따릅니다.",
    description: "Private-network isolation setting explanation",
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
  "environments.settingsSave": {
    message: "저장",
    description: "Environment ACL settings save button",
  },
  "environments.aclSaveSuccess": {
    message: "“{name}” 설정을 저장했습니다.",
    description: "Toast shown after an Environment ACL is saved",
  },
  "environments.aclSaveError": {
    message: "“{name}” 설정을 저장하지 못했습니다.",
    description: "Toast shown when an Environment ACL save fails",
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
};

export default koEnvironments;
