import type { TranslationEntry } from "@/i18n";

const koEnvGroups: Record<string, TranslationEntry> = {
  "envGroups.resourceType": {
    message: "환경 그룹",
    description: "Environment Group resource type used in document titles",
  },
  "envGroups.pageTitle": {
    message: "환경 그룹",
    description: "Workspace env-groups page title",
  },
  "envGroups.pageDescription": {
    message: "서비스 간에 환경 변수와 시크릿 파일을 공유합니다.",
    description: "Workspace env-groups page subtitle",
  },
  "envGroups.newButton": {
    message: "새 환경 그룹",
    description: "Open the create env-group dialog",
  },
  "envGroups.createTitle": {
    message: "환경 그룹 생성",
    description: "Create env-group dialog title",
  },
  "envGroups.createDescription": {
    message:
      "변수, 시크릿 파일, 서비스 연결을 지금 추가하거나 비워 둘 수 있습니다.",
    description: "Create env-group dialog description",
  },
  "envGroups.createVarsTitle": {
    message: "환경 변수",
    description: "Initial environment variables section title",
  },
  "envGroups.createVarsDescription": {
    message: "고정 값을 설정하거나 생성 시 강력한 시크릿을 생성합니다.",
    description: "Initial environment variables section description",
  },
  "envGroups.varKeyLabel": {
    message: "키",
    description: "Initial environment variable key label",
  },
  "envGroups.varKeyPlaceholder": {
    message: "API_TOKEN",
    description: "Initial environment variable key placeholder",
  },
  "envGroups.varValueLabel": {
    message: "값",
    description: "Initial environment variable value label",
  },
  "envGroups.varValuePlaceholder": {
    message: "시크릿 값",
    description: "Initial environment variable value placeholder",
  },
  "envGroups.generateValue": {
    message: "생성",
    description: "Generate an initial environment variable value",
  },
  "envGroups.addInitialVar": {
    message: "환경 변수 추가",
    description: "Add an initial environment variable row",
  },
  "envGroups.importEnv": {
    message: ".env에서 가져오기",
    description:
      "Import initial environment variables from dotenv text or file",
  },
  "envGroups.createFilesTitle": {
    message: "시크릿 파일",
    description: "Initial secret files section title",
  },
  "envGroups.createFilesDescription": {
    message: "파일은 암호화되어 /etc/secrets 아래에 마운트됩니다.",
    description: "Initial secret files section description",
  },
  "envGroups.fileNameLabel": {
    message: "파일 이름",
    description: "Initial secret file name label",
  },
  "envGroups.fileNamePlaceholder": {
    message: "credentials.json",
    description: "Initial secret file name placeholder",
  },
  "envGroups.fileContentLabel": {
    message: "내용",
    description: "Initial secret file contents label",
  },
  "envGroups.fileContentPlaceholder": {
    message: "파일 내용",
    description: "Initial secret file contents placeholder",
  },
  "envGroups.addInitialFile": {
    message: "시크릿 파일 추가",
    description: "Add an initial secret file row",
  },
  "envGroups.createServicesTitle": {
    message: "연결된 서비스",
    description: "Initial service links section title",
  },
  "envGroups.createServicesDescription": {
    message: "선택한 서비스는 그룹이 생성되는 즉시 이 그룹을 받습니다.",
    description: "Initial service links section description",
  },
  "envGroups.servicesLoading": {
    message: "워크스페이스 서비스를 불러오는 중…",
    description: "Initial service links loading state",
  },
  "envGroups.createScopeLoading": {
    message: "환경과 서비스를 불러오는 중…",
    description:
      "Create dialog waits for a confirmed scope and link candidates",
  },
  "envGroups.createScopeError": {
    message:
      "환경 또는 서비스를 불러오지 못했습니다. 그룹을 생성하기 전에 다시 시도하세요.",
    description:
      "Create dialog blocks submission when scope or service lookup fails",
  },
  "envGroups.noServicesToLink": {
    message: "이 워크스페이스에는 아직 연결할 서비스가 없습니다.",
    description: "Initial service links empty state",
  },
  "envGroups.noServicesInScope": {
    message:
      "이 환경에 서비스가 없습니다. 서비스는 자신의 환경에 있는 그룹에만 연결할 수 있습니다.",
    description: "Initial service links empty state for the picked scope",
  },
  "envGroups.invalidInitialContents": {
    message:
      "그룹을 생성하기 전에 올바르지 않은 변수 키 또는 파일 이름을 수정하세요.",
    description: "Invalid initial env-group contents message",
  },
  "envGroups.nameLabel": {
    message: "그룹 이름",
    description: "Env-group name field label",
  },
  "envGroups.namePlaceholder": {
    message: "shared-production",
    description: "Env-group name placeholder",
  },
  "envGroups.invalidName": {
    message: "그룹 이름을 입력하세요.",
    description: "Invalid env-group name message",
  },
  "envGroups.cancel": {
    message: "취소",
    description: "Cancel an env-group dialog",
  },
  "envGroups.createSubmit": {
    message: "환경 그룹 생성",
    description: "Create env-group submit button",
  },
  "envGroups.emptyTitle": {
    message: "환경 그룹이 없습니다",
    description: "Workspace env-groups empty-state title",
  },
  "envGroups.emptyBody": {
    message: "서비스에 연결하기 전에 공유 구성을 관리할 그룹을 만드세요.",
    description: "Workspace env-groups empty-state body",
  },
  "envGroups.serviceCount_other": {
    message: "연결된 서비스 {count}개",
    description: "Env-group linked-service count (plural)",
  },
  "envGroups.metadataTitle": {
    message: "그룹 메타데이터",
    description: "Env-group metadata card title",
  },
  "envGroups.ownerLabel": {
    message: "소유자",
    description: "Env-group owner id label",
  },
  "envGroups.environmentLabel": {
    message: "환경",
    description: "Environment Group scope label",
  },
  "envGroups.workspaceLabel": {
    message: "워크스페이스",
    description: "Destination workspace label",
  },
  "envGroups.workspaceScope": {
    message: "워크스페이스 (환경 없음)",
    description: "Standalone Environment Group scope",
  },
  "envGroups.environmentLookupLoading": {
    message: "환경을 불러오는 중…",
    description:
      "Group metadata Environment value while the scope lookup is still loading",
  },
  "envGroups.environmentLookupFailed": {
    message: "환경을 불러오지 못했습니다",
    description:
      "Group metadata Environment value when the scope lookup request failed",
  },
  "envGroups.unknownEnvironment": {
    message: "알 수 없는 환경 ({id})",
    description: "Deleted or inaccessible Environment fallback",
  },
  "envGroups.createdAtLabel": {
    message: "생성",
    description: "Env-group created timestamp label",
  },
  "envGroups.updatedAtLabel": {
    message: "업데이트",
    description: "Env-group updated timestamp label",
  },
  "envGroups.availabilityLabel": {
    message: "상태",
    description: "Env-group availability label when busy or repair-required",
  },
  "envGroups.availabilityBusy": {
    message: "저장 진행 중 — 잠시 후 다시 시도하세요",
    description: "Busy environment-group availability message",
  },
  "envGroups.availabilityRepairRequired": {
    message: "복구가 필요합니다 — 계속되면 지원팀에 문의하세요",
    description: "Repair-required environment-group availability message",
  },
  "envGroups.unavailableTitle": {
    message: "환경 그룹을 사용할 수 없습니다",
    description: "Secret store unavailable state title",
  },
  "envGroups.unavailableBody": {
    message: "이 배포에는 시크릿 저장소가 구성되어 있지 않습니다.",
    description: "Secret store unavailable state body",
  },
  "envGroups.forbiddenTitle": {
    message: "권한이 없습니다",
    description: "Env-group forbidden state title",
  },
  "envGroups.forbiddenBody": {
    message: "환경 그룹을 보거나 관리할 권한이 없습니다.",
    description: "Env-group forbidden state body",
  },
  "envGroups.genericTitle": {
    message: "환경 그룹을 불러오지 못했습니다",
    description: "Env-group generic query error title",
  },
  "envGroups.genericBody": {
    message: "문제가 발생했습니다. 다시 시도하세요.",
    description: "Env-group generic query error body",
  },
  "envGroups.errorTitle": {
    message: "그룹 내용을 불러오지 못했습니다",
    description: "Env-group editor error title",
  },
  "envGroups.errorBody": {
    message: "문제가 발생했습니다. 다시 시도하세요.",
    description: "Env-group editor error body",
  },
  "envGroups.backToList": {
    message: "환경 그룹으로 돌아가기",
    description: "Detail-page back button label",
  },
  "envGroups.renameButton": {
    message: "이름 변경",
    description: "Open rename env-group dialog",
  },
  "envGroups.manageButton": {
    message: "관리",
    description: "Open Environment Group management menu",
  },
  "envGroups.moveButton": {
    message: "그룹 이동",
    description: "Open Environment Group move dialog",
  },
  "envGroups.moveTitle": {
    message: "환경 그룹 이동",
    description: "Environment Group move dialog title",
  },
  "envGroups.moveDescription": {
    message:
      "호환되는 환경을 선택하거나 이 그룹을 워크스페이스 범위로 되돌립니다.",
    description: "Environment Group move dialog description",
  },
  "envGroups.moveSubmit": {
    message: "그룹 이동",
    description: "Submit Environment Group move",
  },
  "envGroups.moveSuccess": {
    message: "환경 그룹을 이동했습니다",
    description: "Environment Group move success toast",
  },
  "envGroups.moveError": {
    message: "그룹을 이동하지 못했습니다",
    description: "Environment Group move error toast",
  },
  "envGroups.cloneButton": {
    message: "그룹 복제",
    description: "Open Environment Group clone dialog",
  },
  "envGroups.cloneTitle": {
    message: "환경 그룹 복제",
    description: "Environment Group clone dialog title",
  },
  "envGroups.cloneDescription": {
    message:
      "변수와 파일을 새 그룹에 복사합니다. 서비스 연결은 복사되지 않습니다.",
    description: "Environment Group clone dialog description",
  },
  "envGroups.cloneSubmit": {
    message: "그룹 복제",
    description: "Submit Environment Group clone",
  },
  "envGroups.cloneSuccess": {
    message: "{name} 복제 완료",
    description: "Environment Group clone success toast",
  },
  "envGroups.cloneError": {
    message: "그룹을 복제하지 못했습니다",
    description: "Environment Group clone error toast",
  },
  "envGroups.environmentPlaceholder": {
    message: "환경 선택",
    description: "Environment scope selector placeholder",
  },
  "envGroups.renameTitle": {
    message: "환경 그룹 이름 변경",
    description: "Rename env-group dialog title",
  },
  "envGroups.renameDescription": {
    message: "이름을 바꿔도 그룹의 변수, 파일, 서비스 연결은 유지됩니다.",
    description: "Rename env-group dialog description",
  },
  "envGroups.renameSubmit": {
    message: "이름 저장",
    description: "Rename env-group submit button",
  },
  "envGroups.deleteButton": {
    message: "삭제",
    description: "Open delete env-group dialog",
  },
  "envGroups.deleteTitle": {
    message: "{name} 환경 그룹을 삭제할까요?",
    description: "Delete env-group dialog title",
  },
  "envGroups.deleteDescription": {
    message:
      "이 작업은 되돌릴 수 없습니다. 이 환경 그룹을 삭제하시겠습니까? 모든 서비스에서 연결이 해제됩니다.",
    description: "Delete env-group dialog warning (Render-style copy)",
  },
  "envGroups.deletePrompt": {
    message: "확인하려면 아래에 {phrase}을(를) 입력하세요.",
    description:
      "Body prompt naming the exact 'sudo delete env group <name>' phrase (rendered bold by SudoCommandField)",
  },
  "envGroups.deleteConfirm": {
    message: "환경 그룹 삭제",
    description: "Delete env-group confirm button",
  },
  "envGroups.varsTitle": {
    message: "환경 변수",
    description: "Env-group variable editor title",
  },
  "envGroups.varsDescription": {
    message: "값은 암호화되어 연결된 모든 서비스에 적용됩니다.",
    description: "Env-group variable editor description",
  },
  "envGroups.varsEmptyTitle": {
    message: "환경 변수가 없습니다",
    description: "Env-group variable editor empty title",
  },
  "envGroups.varsEmptyBody": {
    message: "지금 변수를 추가하세요. 그룹을 먼저 연결할 필요는 없습니다.",
    description: "Env-group variable editor empty body",
  },
  "envGroups.varDeleteConfirmBody": {
    message:
      "연결된 서비스는 이 변수 없이 다시 배포됩니다. 단, 자동 배포가 꺼진 저장소 기반 서비스는 현재 릴리스를 계속 제공하다가 다음 배포 때 변경 사항을 적용합니다.",
    description: "Delete env-group variable warning",
  },
  "envGroups.filesTitle": {
    message: "시크릿 파일",
    description: "Env-group secret-file editor title",
  },
  "envGroups.filesDescription": {
    message:
      "암호화된 파일이 연결된 모든 서비스의 /etc/secrets에 마운트됩니다.",
    description: "Env-group secret-file editor description",
  },
  "envGroups.filesEmptyTitle": {
    message: "시크릿 파일이 없습니다",
    description: "Env-group secret-file editor empty title",
  },
  "envGroups.filesEmptyBody": {
    message: "지금 파일을 추가하세요. 그룹을 먼저 연결할 필요는 없습니다.",
    description: "Env-group secret-file editor empty body",
  },
  "envGroups.fileDeleteConfirmBody": {
    message:
      "연결된 서비스는 이 파일 없이 다시 배포됩니다. 단, 자동 배포가 꺼진 저장소 기반 서비스는 현재 릴리스를 계속 제공하다가 다음 배포 때 변경 사항을 적용합니다.",
    description: "Delete env-group secret-file warning",
  },
  "envGroups.servicesTitle": {
    message: "연결된 서비스",
    description: "Env-group linked-services card title",
  },
  "envGroups.servicesDescription": {
    message:
      "연결하면 자동 배포 설정과 관계없이 서비스가 그룹의 값으로 지금 배포됩니다. 연결을 해제하면 서비스 구성이 변경됩니다. 자동 배포가 꺼진 저장소 기반 서비스는 다음 배포 때 변경 사항을 적용하고, 그 외 모든 서비스는 지금 배포됩니다.",
    description: "Env-group linked-services card description",
  },
  "envGroups.selectService": {
    message: "서비스 선택",
    description: "Link-service selector placeholder",
  },
  "envGroups.linkButton": {
    message: "서비스 연결",
    description: "Link selected service to env group",
  },
  "envGroups.unlinkButton": {
    message: "연결 해제",
    description: "Unlink service from env group",
  },
  "envGroups.unlinkServiceAria": {
    message: "{name} 연결 해제",
    description:
      "Accessible name of a linked-service row's Unlink control, naming the service (w4/150)",
  },
  "envGroups.noLinkedServices": {
    message:
      "이 그룹은 아직 어떤 서비스에도 연결되어 있지 않습니다. 그래도 자유롭게 편집할 수 있습니다.",
    description: "No linked services state",
  },
  "envGroups.servicesLoadError": {
    message:
      "워크스페이스 서비스를 불러오지 못했습니다. 기존 연결은 아래에 그대로 표시됩니다.",
    description: "Linked-services inventory error",
  },
  "envGroups.incompatibleLink": {
    message: "이 그룹의 환경 밖에서 연결됨",
    description: "Warning on an existing incompatible Environment Group link",
  },
  "envGroups.searchPlaceholder": {
    message: "환경 그룹 검색",
    description: "Environment Group list search placeholder",
  },
  "envGroups.searchLabel": {
    message: "이름으로 환경 그룹 검색",
    description: "Accessible Environment Group search label",
  },
  "envGroups.resetSearch": {
    message: "검색 초기화",
    description: "Clear Environment Group search",
  },
  "envGroups.noMatchesTitle": {
    message: "일치하는 환경 그룹이 없습니다",
    description: "Environment Group search empty-state title",
  },
  "envGroups.noMatchesBody": {
    message: "다른 이름을 입력하거나 검색을 초기화하세요.",
    description: "Environment Group search empty-state body",
  },
  "envGroups.colName": {
    message: "환경 그룹 이름",
    description: "Environment Group table name column",
  },
  "envGroups.colEnvironment": {
    message: "환경",
    description: "Environment Group table Environment column",
  },
  "envGroups.colEnvVars": {
    message: "환경 변수",
    description: "Environment Group table variable-count column",
  },
  "envGroups.colSecretFiles": {
    message: "시크릿 파일",
    description: "Environment Group table secret-file-count column",
  },
  "envGroups.colUpdated": {
    message: "업데이트",
    description: "Environment Group table updated-time column",
  },
  "envGroups.rolloutNote": {
    message:
      "변경 사항을 적용하기 위해 연결된 서비스를 다시 배포하는 중입니다. 단, 자동 배포가 꺼진 저장소 기반 서비스는 다음 배포 때 적용합니다.",
    description: "Env-group write rollout toast detail",
  },
  "envGroups.linkChangeNote": {
    message:
      "자동 배포 설정과 관계없이 서비스가 그룹의 값으로 지금 배포됩니다.",
    description: "Env-group link success toast detail",
  },
  "envGroups.unlinkChangeNote": {
    message:
      "서비스 구성이 변경되었습니다. 자동 배포가 꺼진 저장소 기반 서비스는 다음 배포 때 적용하고, 그 외 서비스는 지금 배포됩니다.",
    description: "Env-group unlink success toast detail",
  },
  "envGroups.createSuccess": {
    message: "{name} 생성 완료",
    description: "Env-group create success toast",
  },
  "envGroups.createError": {
    message: "{name}을(를) 생성하지 못했습니다",
    description: "Env-group create error toast",
  },
  "envGroups.renameSuccess": {
    message: "{name}(으)로 이름을 변경했습니다",
    description: "Env-group rename success toast",
  },
  "envGroups.renameError": {
    message: "그룹 이름을 변경하지 못했습니다",
    description: "Env-group rename error toast",
  },
  "envGroups.deleteSuccess": {
    message: "환경 그룹을 삭제했습니다",
    description: "Env-group delete success toast",
  },
  "envGroups.deleteError": {
    message: "그룹을 삭제하지 못했습니다",
    description: "Env-group delete error toast",
  },
  "envGroups.linkSuccess": {
    message: "서비스를 연결했습니다",
    description: "Env-group link success toast",
  },
  "envGroups.linkError": {
    message: "서비스를 연결하지 못했습니다",
    description: "Env-group link error toast",
  },
  "envGroups.unlinkSuccess": {
    message: "서비스 연결을 해제했습니다",
    description: "Env-group unlink success toast",
  },
  "envGroups.unlinkError": {
    message: "서비스 연결을 해제하지 못했습니다",
    description: "Env-group unlink error toast",
  },
  "envGroups.varSaveSuccess": {
    message: "{key} 저장 완료",
    description: "Env-group variable save success toast",
  },
  "envGroups.varsSaveSuccess": {
    message: "환경 변수를 저장했습니다",
    description: "Env-group replace-all variables success toast",
  },
  "envGroups.varsSaveError": {
    message: "환경 변수를 저장하지 못했습니다",
    description: "Env-group replace-all variables error toast",
  },
  "envGroups.varSaveError": {
    message: "{key}을(를) 저장하지 못했습니다",
    description: "Env-group variable save error toast",
  },
  "envGroups.varDeleteSuccess": {
    message: "{key} 제거 완료",
    description: "Env-group variable delete success toast",
  },
  "envGroups.varDeleteError": {
    message: "{key}을(를) 제거하지 못했습니다",
    description: "Env-group variable delete error toast",
  },
};

export default koEnvGroups;
