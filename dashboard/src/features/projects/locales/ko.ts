import type { TranslationEntry } from "@/i18n";

const koProjects: Record<string, TranslationEntry> = {
  "projects.overviewTitle": {
    message: "개요",
    description: "Page title of the Overview page (workspace homepage)",
  },
  "projects.projectsHeading": {
    message: "프로젝트",
    description: "Section heading above the Overview page's project grid",
  },
  "projects.ungroupedHeading": {
    message: "그룹에 속하지 않은 리소스",
    description:
      "Section heading above the Overview page's ungrouped resource table",
  },
  "projects.allResourcesHeading": {
    message: "모든 리소스",
    description:
      "Heading above a project page's full resource table, below the environments section",
  },
  "projects.filterTypeLabel": {
    message: "리소스 유형",
    description: "Accessible label for project environment type filters",
  },
  "projects.filterAll": {
    message: "전체",
    description: "All project environment resource types",
  },
  "projects.filterServices": {
    message: "서비스",
    description: "Service resource filter",
  },
  "projects.filterDatabases": {
    message: "데이터베이스",
    description: "Database resource filter",
  },
  "projects.filterKeyValues": {
    message: "Key Value",
    description: "Key-value resource filter",
  },
  "projects.filterEnvGroups": {
    message: "환경 그룹",
    description: "Environment-group resource filter",
  },
  "projects.searchLabel": {
    message: "{environment}에서 리소스 검색",
    description: "Accessible project environment resource-search label",
  },
  "projects.searchPlaceholder": {
    message: "{environment}에서 리소스 검색",
    description: "Project environment resource-search placeholder",
  },
  "projects.noMatchesTitle": {
    message: "일치하는 리소스 없음",
    description: "Project environment filtered no-results title",
  },
  "projects.noMatchesBody": {
    message: "다른 검색어나 리소스 유형을 시도해 보세요.",
    description: "Project environment filtered no-results body",
  },
  "projects.eyebrow": {
    message: "프로젝트",
    description: "Small uppercase label above a project page's name heading",
  },
  "projects.cardEmpty": {
    message: "아직 리소스 없음",
    description: "Project card status line when the project has no resources",
  },
  "projects.cardHealthy": {
    message: "모든 리소스 실행 중",
    description: "Project card status line when every resource is healthy",
  },
  "projects.cardInProgress_other": {
    message: "리소스 {count}개 진행 중",
    description:
      "Project card status line when resources are provisioning or deploying without a failure (plural)",
  },
  "projects.cardUnhealthy_other": {
    message: "리소스 {count}개 확인 필요",
    description:
      "Project card status line when more than one resource is unhealthy",
  },
  "projects.newProjectCard": {
    message: "새 프로젝트 만들기",
    description:
      'The dashed "+ create project" tile at the end of the project grid',
  },
  "projects.errorTitle": {
    message: "리소스를 불러오지 못했습니다",
    description: "Error state heading on the unified Projects page",
  },
  "projects.errorBody": {
    message:
      "리소스를 불러오는 중 문제가 발생했습니다. 잠시 후 다시 시도하세요.",
    description: "Error state body on the unified Projects page",
  },
  "projects.emptyTitle": {
    message: "아직 리소스 없음",
    description: "Empty state heading on the unified Projects page",
  },
  "projects.emptyBody": {
    message: "서비스, 데이터베이스 또는 Key Value 저장소를 만들어 시작하세요.",
    description: "Empty state body on the unified Projects page",
  },
  "projects.emptyCreateService": {
    message: "서비스 만들기",
    description: "Primary action of the empty Projects page (ADR075 D1)",
  },
  "projects.newButton": {
    message: "새로 만들기",
    description: "Button that opens the new-resource menu",
  },
  "projects.newService": {
    message: "새 서비스",
    description: "New-resource menu item that starts the service create wizard",
  },
  "projects.newDatabase": {
    message: "새 데이터베이스",
    description: "New-resource menu item that opens the database create dialog",
  },
  "projects.newKeyValue": {
    message: "새 Key Value",
    description: "New-resource menu item that starts the key value create form",
  },
  "projects.colName": {
    message: "이름",
    description: "Merged resource table column header",
  },
  "projects.colType": {
    message: "유형",
    description: "Merged resource table column header",
  },
  "projects.colStatus": {
    message: "상태",
    description: "Merged resource table column header",
  },
  "projects.colCreated": {
    message: "생성일",
    description: "Merged resource table column header",
  },
  "projects.colRuntime": {
    message: "런타임",
    description:
      "Authoritative runtime/product column in a Project resource table",
  },
  "projects.colRegion": {
    message: "리전",
    description:
      "Explicit installation placement column in a Project resource table",
  },
  "projects.colUpdated": {
    message: "업데이트됨",
    description:
      "Authoritative resource update-time column in a Project resource table",
  },
  "projects.colActions": {
    message: "작업",
    description: "Merged resource table column header (screen-reader only)",
  },
  "projects.typeService": {
    message: "서비스",
    description: "Type badge for a service row in the merged resource table",
  },
  "projects.typeDatabase": {
    message: "데이터베이스",
    description: "Type badge for a database row in the merged resource table",
  },
  "projects.typeKeyValue": {
    message: "Key Value",
    description: "Type badge for a key value row in the merged resource table",
  },
  "projects.typeEnvGroup": {
    message: "환경 그룹",
    description: "Type badge for an environment-group row",
  },
  "projects.metadataNotApplicable": {
    message: "해당 없음",
    description:
      "Accessible label for a status, runtime or region cell on an environment group row, which has none of them (w4/549)",
  },
  "projects.metadataUnavailable": {
    message: "사용할 수 없음",
    description:
      "Accessible label for an unavailable runtime, region, status, or timestamp",
  },
  "projects.selectAllVisible": {
    message: "표시된 모든 리소스 선택",
    description: "Accessible label for the Project table header checkbox",
  },
  "projects.selectResource": {
    message: "{name} 선택",
    description: "Accessible label for one Project resource row checkbox",
  },
  "projects.selectedCount": {
    message: "선택됨: {count}",
    description: "Bulk action toolbar selection count",
  },
  "projects.moveTargetLabel": {
    message: "선택한 리소스를 이동할 환경",
    description: "Accessible label for the bulk Move target selector",
  },
  "projects.moveTargetPlaceholder": {
    message: "환경 선택",
    description: "Placeholder for the bulk Move target selector",
  },
  "projects.moveSelected": {
    message: "이동",
    description:
      "Bulk action that moves selected resources to another Environment",
  },
  "projects.bulkResume": {
    message: "재개",
    description:
      "Selection toolbar action that resumes every selected suspended resource (Render's bulk Resume, w4/m550)",
  },
  "projects.bulkSuspend": {
    message: "일시 중지",
    description:
      "Selection toolbar action that suspends every selected running resource (Render's bulk Suspend, w4/m550)",
  },
  "projects.bulkSuspendTitle": {
    message: "서비스 일시 중지",
    description: "Title of the bulk Suspend dialog",
  },
  "projects.bulkSuspendBody": {
    message:
      "다음 서비스가 일시 중지되며 즉시 작동을 멈춥니다. 서비스를 재개하면 되돌릴 수 있습니다.",
    description:
      "Body of the bulk Suspend dialog, above the list of resources it suspends",
  },
  "projects.bulkSuspendConfirm": {
    message: "서비스 일시 중지",
    description: "Confirm button of the bulk Suspend dialog",
  },
  "projects.bulkResumeTitle": {
    message: "서비스 재개",
    description: "Title of the bulk Resume dialog",
  },
  "projects.bulkResumeBody": {
    message: "다음 서비스가 재개됩니다:",
    description:
      "Body of the bulk Resume dialog, above the list of resources it resumes",
  },
  "projects.bulkResumeConfirm": {
    message: "서비스 재개",
    description: "Confirm button of the bulk Resume dialog",
  },
  "projects.bulkRetry": {
    message: "다시 시도",
    description:
      "Button on a bulk Suspend/Resume row whose call failed; runs that row again",
  },
  "projects.bulkRetryRow": {
    message: "{name} 다시 시도",
    description:
      "Accessible label of a row's Retry button ({name} = the resource)",
  },
  "projects.bulkRowSuspending": {
    message: "일시 중지 중",
    description:
      "Screen-reader state of a bulk Suspend row whose call is in flight",
  },
  "projects.bulkRowSuspended": {
    message: "일시 중지됨",
    description: "Screen-reader state of a bulk Suspend row that succeeded",
  },
  "projects.bulkRowResuming": {
    message: "재개 중",
    description:
      "Screen-reader state of a bulk Resume row whose call is in flight",
  },
  "projects.bulkRowResumed": {
    message: "재개됨",
    description: "Screen-reader state of a bulk Resume row that succeeded",
  },
  "projects.bulkRowError": {
    message: "요청이 실패했습니다. 다시 시도하세요.",
    description:
      "Fallback reason on a bulk Suspend/Resume row whose call failed without a server reason",
  },
  "projects.newProjectButton": {
    message: "새 프로젝트",
    description: "Button that opens the new-project dialog",
  },
  "projects.createTitle": {
    message: "새 프로젝트",
    description: "New-project dialog title",
  },
  "projects.createDescription": {
    message:
      "서비스, 데이터베이스, Key Value 저장소를 하나로 묶습니다. 모든 프로젝트는 환경 하나와 함께 시작됩니다.",
    description: "New-project dialog description",
  },
  "projects.fieldName": {
    message: "이름",
    description: "New-project dialog name field label",
  },
  "projects.fieldFirstEnvironment": {
    message: "첫 번째 환경 이름",
    description:
      "New-project dialog: name of the environment created with the project (w4/m264, Render parity)",
  },
  "projects.fieldNamePlaceholder": {
    message: "my-project",
    description: "New-project dialog name field placeholder",
  },
  "projects.createCancel": {
    message: "취소",
    description: "New-project dialog cancel button",
  },
  "projects.createSubmit": {
    message: "프로젝트 만들기",
    description: "New-project dialog submit button",
  },
  "projects.createSuccess": {
    message: '프로젝트 "{name}" 생성 완료.',
    description: "Toast shown after a project is created",
  },
  "projects.createError": {
    message: '프로젝트 "{name}" 생성에 실패했습니다.',
    description: "Toast shown when creating a project fails",
  },
  "projects.editNameButton": {
    message: "편집",
    description:
      "Accessible label for the project page's inline pencil-edit button, and the Settings page's Project Name \"Edit\" button",
  },
  "projects.renameFieldLabel": {
    message: "프로젝트 이름",
    description:
      'Accessible/visible label for the project rename field, on both the overview dialog and the Settings card. Deliberately more specific than projects.fieldName\'s bare "Name": these controls are identified by value alone otherwise (w4/134)',
  },
  "projects.renameTitle": {
    message: "프로젝트 이름 변경",
    description: "Rename-project dialog title",
  },
  "projects.renameSubmit": {
    message: "저장",
    description: "Rename-project dialog submit button",
  },
  "projects.renameSuccess": {
    message: '프로젝트 이름을 변경했습니다: "{name}"',
    description: "Toast shown after a project is renamed",
  },
  "projects.renameRefreshFailed": {
    message:
      '프로젝트 이름을 "{name}" 이름으로 변경했지만 이 페이지를 새로 고치지 못했습니다. 다시 시도하거나 페이지를 새로고침하여 저장된 이름을 확인하세요.',
    description:
      "Warning after the rename saved but re-reading the project failed; the rename must not be retried or reported as failed (w4/m186)",
  },
  "projects.renameError": {
    message: '프로젝트 이름을 변경하지 못했습니다: "{name}"',
    description: "Toast shown when renaming a project fails",
  },
  "projects.settingsTitle": {
    message: "프로젝트 설정",
    description: "Project settings page heading",
  },
  "projects.nameCardTitle": {
    message: "프로젝트 이름",
    description: "Project settings page's name card title",
  },
  "projects.nameCardDescription": {
    message: "프로젝트의 고유한 이름",
    description: "Project settings page's name card description",
  },
  "projects.deleteCardTitle": {
    message: "프로젝트 삭제",
    description: "Project settings page's danger-zone card title",
  },
  "projects.deleteCardDescription": {
    message:
      "프로젝트의 환경과 그 안의 모든 서비스, 데이터베이스, Key Value 저장소가 함께 삭제됩니다. 프로젝트의 환경 그룹은 할당 해제됩니다. 이 작업은 되돌릴 수 없습니다.",
    description: "Project settings page's danger-zone card description",
  },
  "projects.deleteCardButton": {
    message: "프로젝트 삭제",
    description:
      "Project settings page's danger-zone card button, and the delete-confirmation dialog's confirm button",
  },
  "projects.deleteConfirmTitle": {
    message: '프로젝트 "{name}" 삭제하시겠습니까?',
    description: "Delete-project confirmation dialog title",
  },
  "projects.deleteConfirmBody": {
    message:
      "프로젝트의 환경과 해당 구성이 삭제됩니다. 프로젝트의 환경 그룹은 삭제되지 않고 할당 해제됩니다.",
    description: "Delete-project confirmation dialog body",
  },
  "projects.deleteConfirmBodyResources": {
    message:
      "이 프로젝트를 삭제하면 모든 환경과 그 안의 서비스, 데이터베이스, Key Value 저장소가 삭제됩니다: {resources}. 환경 그룹은 삭제되지 않고 할당 해제됩니다. 이 작업은 되돌릴 수 없습니다.",
    description:
      "Delete-project confirmation body naming every service, database and key value store it deletes, as the environment delete does (w4/m264, w6/m290, w4/542)",
  },
  "projects.deleteSuccess": {
    message: '프로젝트 "{name}" 삭제 완료.',
    description: "Toast shown after a project is deleted",
  },
  "projects.deleteError": {
    message: '프로젝트 "{name}" 삭제에 실패했습니다.',
    description: "Toast shown when deleting a project fails",
  },
  "projects.moveToProject": {
    message: "프로젝트로 이동",
    description:
      "Row-actions submenu label for assigning a resource to a project",
  },
  "projects.removeFromProject": {
    message: "프로젝트에서 제거",
    description:
      "Row-actions submenu item that unassigns a resource from its project",
  },
  "projects.moveSuccess": {
    message: '"{name}" 항목을 "{project}" 프로젝트로 이동했습니다.',
    description: "Toast shown after moving a resource to a project",
  },
  "projects.moveError": {
    message: '"{name}" 항목을 이동하지 못했습니다.',
    description: "Toast shown when moving a resource to a project fails",
  },
  "projects.removeSuccess": {
    message: '"{name}" 항목을 프로젝트에서 제거했습니다.',
    description: "Toast shown after unassigning a resource from its project",
  },
  "projects.removeError": {
    message: '"{name}" 항목을 프로젝트에서 제거하지 못했습니다.',
    description: "Toast shown when unassigning a resource fails",
  },
};

export default koProjects;
