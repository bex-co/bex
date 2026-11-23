import type { TranslationEntry } from "@/i18n";

const koSandboxGroups: Record<string, TranslationEntry> = {
  "sandboxGroups.createTitle": {
    message: "새 샌드박스 그룹",
    description:
      "Title of the new-sandbox-group dialog and its entry in the + New menus",
  },
  "sandboxGroups.createDescription": {
    message:
      "샌드박스 그룹은 그룹에 속한 샌드박스를 동시에 몇 개까지 활성화할 수 있는지 제한합니다. 그룹 이름은 워크스페이스 내에서 고유해야 합니다.",
    description: "New-sandbox-group dialog description",
  },
  "sandboxGroups.fieldName": {
    message: "이름",
    description: "New-sandbox-group dialog name field label",
  },
  "sandboxGroups.fieldNamePlaceholder": {
    message: "my-sandbox-group",
    description: "Placeholder for the new sandbox group's name",
  },
  "sandboxGroups.fieldRegion": {
    message: "리전",
    description:
      "Label of the region row in the created sandbox group's confirmation",
  },
  "sandboxGroups.createCancel": {
    message: "취소",
    description: "Cancel button of the new-sandbox-group dialog",
  },
  "sandboxGroups.createSubmit": {
    message: "샌드박스 그룹 만들기",
    description: "Submit button of the new-sandbox-group dialog",
  },
  "sandboxGroups.createError": {
    message: "{name} 샌드박스 그룹을 만들지 못했습니다. 다시 시도하세요.",
    description: "Toast shown when creating a sandbox group fails",
  },
  "sandboxGroups.errorNameTaken": {
    message:
      '이 워크스페이스에 이름이 "{name}"인 샌드박스 그룹이 이미 있습니다. 다른 이름을 선택하세요.',
    description:
      "Inline error under the name field when the group name is already taken",
  },
  "sandboxGroups.errorNameInvalid": {
    message:
      "그룹 이름은 1자에서 {max}자 사이여야 하며, 앞뒤 공백과 제어 문자를 포함할 수 없습니다.",
    description: "Inline error under the name field when the name is refused",
  },
  "sandboxGroups.errorLimitReached_other": {
    message:
      "이 워크스페이스에는 이미 최대 {count}개의 샌드박스 그룹이 있습니다.",
    description:
      "Inline alert when the workspace is at its sandbox group limit",
  },
  "sandboxGroups.errorLimitReachedGeneric": {
    message: "이 워크스페이스가 샌드박스 그룹 한도에 도달했습니다.",
    description:
      "Inline alert for the sandbox group limit when the server names no number",
  },
  "sandboxGroups.errorLimitTitle": {
    message: "샌드박스 그룹 한도 도달",
    description: "Heading of the inline sandbox-group-limit alert",
  },
  "sandboxGroups.createdTitle": {
    message: "샌드박스 그룹을 만들었습니다",
    description: "Dialog title once a sandbox group has been created",
  },
  "sandboxGroups.createdDescription": {
    message:
      "{name} 그룹이 준비되었습니다. 이 그룹에 샌드박스를 만들 때 그룹의 ID를 sandboxGroupId로 전달하세요.",
    description: "Dialog description once a sandbox group has been created",
  },
  "sandboxGroups.createdDone": {
    message: "완료",
    description: "Button that closes the created-sandbox-group confirmation",
  },
  "sandboxGroups.fieldId": {
    message: "ID",
    description: "Label of the new sandbox group's ID in the confirmation",
  },
  "sandboxGroups.fieldDefault": {
    message: "기본 그룹",
    description:
      "Label of the confirmation row that says whether the group is the workspace's default",
  },
  "sandboxGroups.defaultYes": {
    message: "예",
    description: "The created group is the workspace's default sandbox group",
  },
  "sandboxGroups.defaultNo": {
    message: "아니요",
    description:
      "The created group is not the workspace's default sandbox group",
  },
  "sandboxGroups.fieldConcurrencyLimit": {
    message: "동시 실행 한도",
    description:
      "Label of the confirmation row with the most active sandboxes the group allows",
  },
  "sandboxGroups.copyId": {
    message: "샌드박스 그룹 ID 복사",
    description: "Accessible label of the copy button next to the group's ID",
  },
  "sandboxGroups.copied": {
    message: "샌드박스 그룹 ID를 복사했습니다",
    description: "Toast after copying the sandbox group ID",
  },
  "sandboxGroups.copyError": {
    message: "샌드박스 그룹 ID를 복사하지 못했습니다",
    description: "Toast when copying the sandbox group ID fails",
  },
  "sandboxGroups.environmentLoading": {
    message: "환경을 불러오는 중",
    description:
      "Accessible name of the loading placeholder shown where the new-sandbox-group dialog's Environment select will appear",
  },
  "sandboxGroups.environmentsError": {
    message: "환경을 불러오지 못했습니다. 환경 없이 그룹을 만들 수는 있습니다.",
    description:
      "Shown in the new-sandbox-group dialog when the workspace's environments could not be read; the group can still be created without one",
  },
  "sandboxGroups.errorEnvironmentGone": {
    message:
      "해당 환경이 더 이상 존재하지 않습니다. 다른 환경을 선택하거나 환경 없음을 선택하세요.",
    description:
      "Inline error under the Environment select when the chosen environment was deleted before the group was created",
  },
  "sandboxGroups.pageTitle": {
    message: "샌드박스 그룹",
    description: "Heading for the workspace sandbox groups page",
  },
  "sandboxGroups.pageDescription": {
    message: "각 그룹의 샌드박스 동시성 제한과 아웃바운드 브로커 자격 증명을 관리하세요.",
    description: "Subtitle for the workspace sandbox groups page",
  },
  "sandboxGroups.resourceType": {
    message: "샌드박스 그룹",
    description: "Singular resource type name for a sandbox group",
  },
  "sandboxGroups.backToList": {
    message: "샌드박스 그룹 목록으로 돌아가기",
    description: "Accessible label of the back link on a sandbox group detail page",
  },
  "sandboxGroups.defaultBadge": {
    message: "기본값",
    description: "Badge indicating the workspace default sandbox group",
  },
  "sandboxGroups.statusReady": {
    message: "준비됨",
    description:
      "Status of a sandbox group row in a project environment's resource table; Render shows every group as Ready",
  },
  "sandboxGroups.environmentUpdated": {
    message: "“{name}”의 샌드박스 그룹을 업데이트했습니다.",
    description:
      "Toast after sandbox groups were moved into or out of an environment; {name} is the environment",
  },
  "sandboxGroups.environmentUpdateError": {
    message: "{group} 샌드박스 그룹을 이동하지 못했습니다. 다시 시도하세요.",
    description:
      "Toast when moving a sandbox group into or out of an environment fails and the server gave no reason to show",
  },
  "sandboxGroups.listLoadError": {
    message: "샌드박스 그룹을 불러올 수 없습니다. 다시 시도해 주세요.",
    description: "Error state when loading sandbox groups fails",
  },
  "sandboxGroups.listEmptyTitle": {
    message: "아직 샌드박스 그룹이 없습니다",
    description: "Empty state heading when the workspace has no sandbox groups",
  },
  "sandboxGroups.listEmptyDescription": {
    message: "샌드박스 그룹을 만들어 동시성 제한과 아웃바운드 자격 증명을 구성하세요.",
    description: "Empty state body when the workspace has no sandbox groups",
  },
  "sandboxGroups.manageCredentialsButton": {
    message: "자격 증명",
    description: "Button on each sandbox group row linking to its detail and credentials",
  },
  "sandboxGroups.detailLoadError": {
    message: "이 샌드박스 그룹을 불러올 수 없습니다. 다시 시도해 주세요.",
    description: "Error state when loading a single sandbox group fails",
  },
  "sandboxGroups.credSectionTitle": {
    message: "아웃바운드 자격 증명",
    description: "Card title for the sandbox group credentials panel",
  },
  "sandboxGroups.credSectionDescription": {
    message:
      "이 그룹의 샌드박스에서 일치하는 아웃바운드 HTTPS 요청에 주입되는 브로커 비밀 값입니다. 저장된 비밀 값은 쓰기 전용이며 다시 표시되지 않습니다.",
    description: "Card subtitle for the sandbox group credentials panel",
  },
  "sandboxGroups.credAddButton": {
    message: "자격 증명 추가",
    description: "Button to open the create sandbox group credential dialog",
  },
  "sandboxGroups.credLoadError": {
    message: "샌드박스 그룹 자격 증명을 불러올 수 없습니다. 다시 시도해 주세요.",
    description: "Error state when fetching sandbox group credentials fails",
  },
  "sandboxGroups.credEmptyTitle": {
    message: "구성된 자격 증명이 없습니다",
    description: "Empty state heading when a sandbox group has no credentials",
  },
  "sandboxGroups.credEmptyDescriptionAdmin": {
    message:
      "자격 증명을 추가하면 샌드박스 코드에 비밀을 노출하지 않고 아웃바운드 요청에 인증을 주입할 수 있습니다.",
    description: "Empty state body for admins when no credentials exist",
  },
  "sandboxGroups.credEmptyDescriptionReadOnly": {
    message: "워크스페이스 관리자가 이 그룹의 아웃바운드 자격 증명을 추가할 수 있습니다.",
    description: "Empty state body for non-admins when no credentials exist",
  },
  "sandboxGroups.credColId": {
    message: "자격 증명 ID",
    description: "Table column header for credential ID",
  },
  "sandboxGroups.credColDomain": {
    message: "도메인",
    description: "Table column header for destination domain",
  },
  "sandboxGroups.credColInjection": {
    message: "주입 방식",
    description: "Table column header for injection method and target",
  },
  "sandboxGroups.credColRouting": {
    message: "라우팅",
    description: "Table column header for HTTP methods and path filters",
  },
  "sandboxGroups.credColEnabled": {
    message: "활성화",
    description: "Table column header for enabled toggle or status",
  },
  "sandboxGroups.credColActions": {
    message: "작업",
    description: "Table column header for row actions",
  },
  "sandboxGroups.credAllMethods": {
    message: "모든 메서드",
    description: "Badge shown when a credential matches all HTTP methods",
  },
  "sandboxGroups.credAllPaths": {
    message: "모든 경로",
    description: "Badge shown when a credential matches all request paths",
  },
  "sandboxGroups.credStatusEnabled": {
    message: "활성화됨",
    description: "Read-only badge when a credential is enabled",
  },
  "sandboxGroups.credStatusDisabled": {
    message: "비활성화됨",
    description: "Read-only badge when a credential is disabled",
  },
  "sandboxGroups.credToggleEnabledLabel": {
    message: "{id} 활성화 상태 전환",
    description: "Accessible label for the enabled switch on a credential row",
  },
  "sandboxGroups.credVersionsActionLabel": {
    message: "{id} 버전 기록 보기",
    description: "Accessible label for the version history icon button",
  },
  "sandboxGroups.credEditActionLabel": {
    message: "자격 증명 {id} 편집",
    description: "Accessible label for the edit credential icon button",
  },
  "sandboxGroups.credDeleteActionLabel": {
    message: "자격 증명 {id} 삭제",
    description: "Accessible label for the delete credential icon button",
  },
  "sandboxGroups.credVersionsButton": {
    message: "버전",
    description: "Mobile button label to open credential version history",
  },
  "sandboxGroups.credEditButton": {
    message: "편집",
    description: "Mobile button label to edit a credential",
  },
  "sandboxGroups.credDeleteButton": {
    message: "삭제",
    description: "Button label to delete a credential",
  },
  "sandboxGroups.credCreateTitle": {
    message: "자격 증명 추가",
    description: "Dialog title when creating a sandbox group credential",
  },
  "sandboxGroups.credCreateDescription": {
    message: "이 샌드박스 그룹에 쓰기 전용 아웃바운드 자격 증명을 구성하세요.",
    description: "Dialog description when creating a sandbox group credential",
  },
  "sandboxGroups.credEditTitle": {
    message: "{id} 편집",
    description: "Dialog title when editing a sandbox group credential",
  },
  "sandboxGroups.credEditDescriptionAdmin": {
    message:
      "라우팅, 주입 방식, 도메인을 수정하거나 비밀 값을 교체하세요. 비밀 값을 비워 두면 현재 값이 유지됩니다.",
    description: "Dialog description when an admin edits a credential",
  },
  "sandboxGroups.credEditDescriptionDeveloper": {
    message:
      "주입 방식, 메서드, 경로 또는 활성화 상태를 수정하세요. 도메인이나 비밀 값 변경은 워크스페이스 관리자만 가능합니다.",
    description: "Dialog description when a developer edits a credential",
  },
  "sandboxGroups.credFieldId": {
    message: "자격 증명 ID",
    description: "Form label for credential ID",
  },
  "sandboxGroups.credFieldIdHelp": {
    message: "대소문자를 구분하며 영문자, 숫자, ., _, ~, -만 사용할 수 있습니다. `rnd-`로 시작할 수 없습니다.",
    description: "Help text for credential ID format",
  },
  "sandboxGroups.credFieldDomain": {
    message: "대상 도메인",
    description: "Form label for destination hostname",
  },
  "sandboxGroups.credAdminOnlyDomainSecret": {
    message: "워크스페이스 관리자만 도메인이나 비밀 값을 변경할 수 있습니다.",
    description: "Helper text shown to developers on read-only domain field",
  },
  "sandboxGroups.credFieldValue": {
    message: "비밀 값",
    description: "Form label for secret value on create",
  },
  "sandboxGroups.credFieldValuePlaceholder": {
    message: "비밀 값을 붙여넣으세요",
    description: "Placeholder for secret value input on create",
  },
  "sandboxGroups.credFieldReplacementValue": {
    message: "새 비밀 값(선택 사항)",
    description: "Form label for rotating secret value on edit",
  },
  "sandboxGroups.credFieldReplacementValuePlaceholder": {
    message: "기존 비밀을 유지하려면 비워 두세요",
    description: "Placeholder for optional secret rotation input on edit",
  },
  "sandboxGroups.credFieldValueWriteOnlyHelp": {
    message: "쓰기 전용(최대 5 KiB)이며 저장 후 다시 표시되지 않습니다.",
    description: "Help text explaining write-only secret handling",
  },
  "sandboxGroups.credFieldInjectionType": {
    message: "주입 유형",
    description: "Form label for injection type select",
  },
  "sandboxGroups.credInjectionHeader": {
    message: "HTTP 헤더",
    description: "Injection type option for HTTP header",
  },
  "sandboxGroups.credInjectionBasic": {
    message: "Basic 인증",
    description: "Injection type option for HTTP Basic authentication",
  },
  "sandboxGroups.credInjectionCookie": {
    message: "쿠키",
    description: "Injection type option for HTTP cookie",
  },
  "sandboxGroups.credInjectionQuery": {
    message: "쿼리 매개변수",
    description: "Injection type option for URL query parameter",
  },
  "sandboxGroups.credFieldInjectionName": {
    message: "대상 이름",
    description: "Form label for header, cookie, or query parameter name",
  },
  "sandboxGroups.credBasicHelp": {
    message: "`Authorization` 헤더를 자동으로 사용합니다. `username:password` 형식으로 입력하세요.",
    description: "Helper text shown when basic auth injection is selected",
  },
  "sandboxGroups.credFieldTemplate": {
    message: "값 템플릿(선택 사항)",
    description: "Form label for optional value template",
  },
  "sandboxGroups.credFieldTemplateHelp": {
    message: "기본값은 {token}입니다. {token}을 1~5회 포함해야 합니다.",
    description: "Help text for injection template placeholder syntax",
  },
  "sandboxGroups.credFieldMethods": {
    message: "HTTP 메서드",
    description: "Form label for allowed HTTP methods multi-select",
  },
  "sandboxGroups.credFieldMethodsHelp": {
    message: "아무것도 선택하지 않으면 모든 HTTP 메서드와 일치합니다.",
    description: "Help text for HTTP methods selection",
  },
  "sandboxGroups.credFieldPaths": {
    message: "경로 패턴(줄당 하나)",
    description: "Form label for path patterns textarea",
  },
  "sandboxGroups.credFieldPathsHelp": {
    message: "/로 시작하는 절대 경로이며 전체 세그먼트 * 및 ** 와일드카드를 지원합니다. 비워 두면 모든 경로와 일치합니다.",
    description: "Help text for path pattern syntax",
  },
  "sandboxGroups.credFieldEnabled": {
    message: "활성화됨",
    description: "Form label for credential enabled switch",
  },
  "sandboxGroups.credFieldEnabledHelp": {
    message: "비활성화된 자격 증명은 그룹에 유지되지만 요청에 주입되지 않습니다.",
    description: "Help text for credential enabled switch",
  },
  "sandboxGroups.credCreateSubmit": {
    message: "자격 증명 생성",
    description: "Submit button label for creating a credential",
  },
  "sandboxGroups.credEditSubmit": {
    message: "변경 사항 저장",
    description: "Submit button label for editing a credential",
  },
  "sandboxGroups.credCreatedToast": {
    message: "자격 증명 {id}이(가) 생성되었습니다.",
    description: "Success toast after creating a credential",
  },
  "sandboxGroups.credUpdatedToast": {
    message: "자격 증명 {id}이(가) 업데이트되었습니다.",
    description: "Success toast after updating a credential",
  },
  "sandboxGroups.credDeletedToast": {
    message: "자격 증명 {id}이(가) 삭제되었습니다.",
    description: "Success toast after deleting a credential",
  },
  "sandboxGroups.credRestoredToast": {
    message: "{id}을(를) 버전 v{version}에서 복원했습니다.",
    description: "Success toast after restoring a credential version",
  },
  "sandboxGroups.credCreateError": {
    message: "자격 증명을 생성할 수 없습니다. 입력 내용을 확인해 주세요.",
    description: "Fallback error when creating a credential fails",
  },
  "sandboxGroups.credUpdateError": {
    message: "자격 증명을 업데이트할 수 없습니다. 다시 시도해 주세요.",
    description: "Fallback error when updating a credential fails",
  },
  "sandboxGroups.credDeleteError": {
    message: "자격 증명을 삭제할 수 없습니다. 다시 시도해 주세요.",
    description: "Fallback error when deleting a credential fails",
  },
  "sandboxGroups.credRestoreError": {
    message: "자격 증명 버전을 복원할 수 없습니다. 다시 시도해 주세요.",
    description: "Fallback error when restoring a credential version fails",
  },
  "sandboxGroups.credErrorExists": {
    message: '이 그룹에 ID "{id}"인 자격 증명이 이미 존재합니다.',
    description: "Inline error when creating a duplicate credential ID",
  },
  "sandboxGroups.credErrorBusy": {
    message: "이 그룹에서 다른 자격 증명 업데이트가 진행 중입니다. 잠시 후 다시 시도해 주세요.",
    description: "Error when concurrent credential writes contend on the group lock",
  },
  "sandboxGroups.credErrorUnavailable": {
    message: "자격 증명 저장소를 현재 사용할 수 없습니다.",
    description: "Error when OpenBao credential storage is unconfigured or unreachable",
  },
  "sandboxGroups.credErrorVersionNotFound": {
    message: "해당 자격 증명 버전은 더 이상 보관되지 않습니다.",
    description: "Error when restoring an evicted or unknown version",
  },
  "sandboxGroups.credVersionsTitle": {
    message: "버전 기록 · {id}",
    description: "Dialog title for credential version history",
  },
  "sandboxGroups.credVersionsDescription": {
    message: "현재 구성과 최근의 서로 다른 이전 구성 최대 3개를 보관합니다.",
    description: "Dialog subtitle explaining version retention policy",
  },
  "sandboxGroups.credVersionsError": {
    message: "버전 기록을 불러올 수 없습니다. 다시 시도해 주세요.",
    description: "Error state when loading credential versions fails",
  },
  "sandboxGroups.credVersionsEmpty": {
    message: "보관된 버전이 없습니다.",
    description: "Empty state for credential versions dialog",
  },
  "sandboxGroups.credVersionCurrent": {
    message: "현재",
    description: "Badge marking the active credential version",
  },
  "sandboxGroups.credVersionRestoredFrom": {
    message: "v{version}에서 복원됨",
    description: "Badge showing which prior version was restored",
  },
  "sandboxGroups.credFingerprintLabel": {
    message: "지문",
    description: "Label before the truncated SHA-256 fingerprint",
  },
  "sandboxGroups.credUpdatedBy": {
    message: "작성자: {actor}",
    description: "Actor attribution on a credential version row",
  },
  "sandboxGroups.credRestoreAction": {
    message: "복원",
    description: "Button label to restore a prior credential version",
  },
  "sandboxGroups.credRestoreConfirmTitle": {
    message: "{id}을(를) v{version}(으)로 복원할까요?",
    description: "Confirmation dialog title when restoring a credential version",
  },
  "sandboxGroups.credRestoreConfirmDescription": {
    message: "v{version}의 구성과 비밀 값을 사용하여 {id}의 새 현재 버전을 만듭니다.",
    description: "Confirmation dialog body when restoring a credential version",
  },
  "sandboxGroups.credDeleteConfirmTitle": {
    message: "자격 증명 {id}을(를) 삭제할까요?",
    description: "Confirmation dialog title when deleting a credential",
  },
  "sandboxGroups.credDeleteConfirmDescription": {
    message: "{id}을(를) 삭제하면 저장된 비밀 값과 모든 보관 버전이 영구적으로 삭제됩니다.",
    description: "Confirmation dialog body when deleting a credential",
  },
  "sandboxGroups.credValIdRequired": {
    message: "자격 증명 ID는 필수입니다.",
    description: "Validation error when credential ID is empty",
  },
  "sandboxGroups.credValIdInvalid": {
    message: "영문자, 숫자, 마침표, 밑줄, 물결표, 하이픈만 사용하세요.",
    description: "Validation error when credential ID has disallowed characters",
  },
  "sandboxGroups.credValIdReserved": {
    message: '".", ".." 및 `rnd-`로 시작하는 자격 증명 ID는 예약되어 있습니다.',
    description: "Validation error when credential ID is reserved",
  },
  "sandboxGroups.credValDomainRequired": {
    message: "대상 도메인은 필수입니다.",
    description: "Validation error when domain is empty",
  },
  "sandboxGroups.credValDomainInvalid": {
    message: "와일드카드, 포트, 경로가 없는 유효한 공개 DNS 호스트 이름을 입력하세요.",
    description: "Validation error when domain format is invalid",
  },
  "sandboxGroups.credValDomainPrivate": {
    message: "로컬, 사설 및 클러스터 호스트 이름은 허용되지 않습니다.",
    description: "Validation error when domain is private or cluster-local",
  },
  "sandboxGroups.credValValueRequired": {
    message: "비밀 값은 필수입니다.",
    description: "Validation error when secret value is empty on create",
  },
  "sandboxGroups.credValValueTooLong": {
    message: "비밀 값은 최대 {maxBytes}바이트여야 합니다.",
    description: "Validation error when secret value exceeds 5 KiB",
  },
  "sandboxGroups.credValValueNul": {
    message: "비밀 값에 NUL 바이트를 포함할 수 없습니다.",
    description: "Validation error when secret value contains a NUL byte",
  },
  "sandboxGroups.credValInjectionNameRequired": {
    message: "이 주입 유형에는 대상 이름이 필수입니다.",
    description: "Validation error when header/cookie/query name is missing",
  },
  "sandboxGroups.credValInjectionNameForbidden": {
    message: "Basic 인증 주입에는 대상 이름을 지정할 수 없습니다.",
    description: "Validation error when basic injection specifies a name",
  },
  "sandboxGroups.credValInjectionHeaderForbidden": {
    message: "라우팅 및 연결 제어 헤더는 주입할 수 없습니다.",
    description: "Validation error when a forbidden HTTP header is chosen",
  },
  "sandboxGroups.credValInjectionNameInvalid": {
    message: "유효한 대상 이름을 입력하세요.",
    description: "Validation error when target name has invalid characters",
  },
  "sandboxGroups.credValInjectionTemplateCount": {
    message: "템플릿에는 {token}이(가) 1~{max}회 포함되어야 합니다.",
    description: "Validation error when template placeholder count is out of range",
  },
  "sandboxGroups.credValInjectionBasicFormat": {
    message: "Basic 인증 렌더링 값은 `username:password` 형식이어야 합니다.",
    description: "Validation error when basic auth rendered value lacks username:password",
  },
  "sandboxGroups.credValInjectionHeaderControl": {
    message: "렌더링된 헤더 값에는 줄바꿈이나 NUL 바이트를 포함할 수 없습니다.",
    description: "Validation error when header value contains CR/LF/NUL",
  },
  "sandboxGroups.credValInjectionCookieInvalid": {
    message: "렌더링된 쿠키 값에는 세미콜론이나 줄바꿈을 포함할 수 없습니다.",
    description: "Validation error when cookie value contains semicolons or CR/LF",
  },
  "sandboxGroups.credValPathsInvalid": {
    message: "각 경로는 /로 시작하고 ?나 #을 포함하지 않으며 전체 세그먼트 * 또는 ** 와일드카드만 사용해야 합니다.",
    description: "Validation error when a path pattern is invalid",
  },
};

export default koSandboxGroups;
