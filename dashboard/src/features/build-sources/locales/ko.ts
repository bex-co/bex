import type { TranslationEntry } from "@/i18n";

const koBuildSources: Record<string, TranslationEntry> = {
  "buildSources.pageTitle": {
    message: "빌드 소스",
    description: "Title of the Build Sources list page",
  },
  "buildSources.pageDescription": {
    message:
      "Git 저장소 또는 Docker 이미지에서 한 번 빌드하고 여러 서비스에 아티팩트를 배포하세요.",
    description: "Subtitle of the Build Sources list page",
  },
  "buildSources.resourceType": {
    message: "빌드 소스",
    description: "Singular resource type label shown in global search results",
  },
  "buildSources.newButton": {
    message: "새 빌드 소스",
    description: "Primary CTA button to create a new Build Source",
  },
  "buildSources.backToList": {
    message: "빌드 소스 목록으로 돌아가기",
    description: "Back button label returning to /build-sources",
  },
  "buildSources.viewAll": {
    message: "모두 보기",
    description: "Link from Project Overview Build Sources section to /build-sources",
  },
  "buildSources.viewBuildSources": {
    message: "빌드 소스 보기",
    description: "Button in partial-link alert navigating to /build-sources",
  },
  "buildSources.emptyTitle": {
    message: "아직 빌드 소스가 없습니다",
    description: "Empty state heading when the workspace has no build sources",
  },
  "buildSources.emptyDescription": {
    message:
      "빌드 소스를 만들어 Web Service, Private Service, Background Worker, Cron Job에서 단일 Git 빌드나 Docker 이미지를 공유하세요.",
    description: "Empty state explanation on /build-sources",
  },
  "buildSources.refreshError": {
    message: "빌드 소스를 새로고침하지 못했습니다. 캐시된 결과를 표시합니다.",
    description: "Inline error banner when a background refresh fails over cached rows",
  },
  "buildSources.searchPlaceholder": {
    message: "이름, 저장소, 이미지 또는 프로젝트로 검색…",
    description: "Placeholder for the /build-sources search input",
  },
  "buildSources.searchLabel": {
    message: "빌드 소스 검색",
    description: "Accessible label for the /build-sources search input",
  },
  "buildSources.resetFilters": {
    message: "필터 초기화",
    description: "Button to clear active search and kind filters on /build-sources",
  },
  "buildSources.filterKindLabel": {
    message: "소스 유형별 필터",
    description: "Accessible label for the source-kind filter button group",
  },
  "buildSources.filterAll": {
    message: "전체",
    description: "Filter button showing both Git and Docker image build sources",
  },
  "buildSources.kindGit": {
    message: "Git",
    description: "Source kind badge and filter label for Git-backed build sources",
  },
  "buildSources.kindImage": {
    message: "Docker 이미지",
    description: "Source kind badge and filter label for Docker-image-backed build sources",
  },
  "buildSources.noMatchesTitle": {
    message: "일치하는 빌드 소스가 없습니다",
    description: "Heading when search or kind filter matches zero build sources",
  },
  "buildSources.noMatchesDescription": {
    message: "검색어를 바꾸거나 소스 유형 필터를 해제해 보세요.",
    description: "Body copy when search or kind filter matches zero build sources",
  },
  "buildSources.colName": {
    message: "이름",
    description: "Table column header for Build Source name",
  },
  "buildSources.colSource": {
    message: "소스",
    description: "Table column header for Git repo/branch or Docker image",
  },
  "buildSources.colRegion": {
    message: "리전",
    description: "Table column header for Build Source region",
  },
  "buildSources.colLinkedServices": {
    message: "연결된 서비스",
    description: "Table column header for linked service count",
  },
  "buildSources.colProject": {
    message: "프로젝트",
    description: "Table column header for optional project scope",
  },
  "buildSources.colUpdated": {
    message: "업데이트됨",
    description: "Table column header for last updated timestamp",
  },
  "buildSources.copyId": {
    message: "빌드 소스 ID 복사",
    description: "Accessible label for copying a bds- ID",
  },
  "buildSources.copied": {
    message: "복사됨",
    description: "Copy button confirmation tooltip",
  },
  "buildSources.copyError": {
    message: "복사하지 못했습니다",
    description: "Copy button failure tooltip",
  },
  "buildSources.linkedServicesCount_other": {
    message: "서비스 {count}개",
    description: "Count of services linked to a build source",
  },
  "buildSources.workspaceScope": {
    message: "워크스페이스",
    description: "Label when a build source is workspace-wide (not scoped to a project)",
  },
  "buildSources.createTitle": {
    message: "새 빌드 소스",
    description: "Page heading of /build-sources/new",
  },
  "buildSources.createDescription": {
    message:
      "여러 서비스가 함께 배포할 수 있는 공유 Git 저장소 빌드 또는 Docker 이미지를 구성합니다.",
    description: "Subtitle of /build-sources/new",
  },
  "buildSources.createSubmit": {
    message: "빌드 소스 만들기",
    description: "Submit button label on /build-sources/new",
  },
  "buildSources.createSuccess": {
    message: '"{name}" 빌드 소스를 만들었습니다',
    description: "Toast shown after creating a Build Source",
  },
  "buildSources.createError": {
    message: "빌드 소스를 만들지 못했습니다. 다시 시도해 주세요.",
    description: "Fallback error toast when createBuildSource fails",
  },
  "buildSources.updateSuccess": {
    message: '"{name}" 빌드 소스를 업데이트했습니다',
    description: "Toast shown after updating a Build Source",
  },
  "buildSources.updateError": {
    message: "빌드 소스를 업데이트하지 못했습니다. 다시 시도해 주세요.",
    description: "Fallback error toast when updateBuildSource fails",
  },
  "buildSources.deleteSuccess": {
    message: '"{name}" 빌드 소스를 삭제했습니다',
    description: "Toast shown after deleting a Build Source",
  },
  "buildSources.deleteError": {
    message: "빌드 소스를 삭제하지 못했습니다. 다시 시도해 주세요.",
    description: "Fallback error toast when deleteBuildSource fails",
  },
  "buildSources.triggerBuildSuccess": {
    message: "빌드를 시작했습니다",
    description: "Toast shown after triggering a Build Source build",
  },
  "buildSources.triggerBuildError": {
    message: "빌드를 시작하지 못했습니다. 다시 시도해 주세요.",
    description: "Fallback error toast when triggerBuild fails",
  },
  "buildSources.createPermissionDenied": {
    message:
      "빌드 소스를 만들려면 이 워크스페이스에서 Developer 또는 Admin 권한이 필요합니다.",
    description: "Read-only notice shown on /build-sources/new when can_create is false",
  },
  "buildSources.createBlockedSource": {
    message: "계속하려면 Git 저장소 또는 Docker 이미지를 선택하세요.",
    description: "Action bar hint while no source is selected",
  },
  "buildSources.partiallyLinkedTitle": {
    message: "빌드 소스는 생성되었지만 일부 서비스 연결에 실패했습니다",
    description: "Alert heading when createBuildSource returns 409 BUILD_SOURCE_PARTIALLY_LINKED",
  },
  "buildSources.partiallyLinkedDescription": {
    message:
      "빌드 소스 {id}가 생성되었지만 선택한 모든 서비스를 연결하지 못했습니다.",
    description: "Alert summary naming the newly created bds- ID on partial link conflict",
  },
  "buildSources.partiallyLinkedSucceeded": {
    message: "연결됨: {services}",
    description: "List of services that were successfully linked before the conflict",
  },
  "buildSources.partiallyLinkedFailed": {
    message: "연결 실패: {service}",
    description: "The service whose link failed during create",
  },
  "buildSources.fieldSource": {
    message: "소스",
    description: "Create form label for Git repository vs Docker image picker",
  },
  "buildSources.fieldSourceDescription": {
    message:
      "연결된 GitHub 저장소, 공개 Git URL 또는 기존 Docker 이미지를 선택하세요.",
    description: "Help text for the source picker row on /build-sources/new",
  },
  "buildSources.fieldName": {
    message: "이름",
    description: "Create form label for Build Source name",
  },
  "buildSources.fieldNameDescription": {
    message: "이 워크스페이스 내에서 고유한 이름입니다.",
    description: "Help text for Build Source name",
  },
  "buildSources.fieldNamePlaceholder": {
    message: "my-shared-api",
    description: "Placeholder for Build Source name input",
  },
  "buildSources.errorNameTaken": {
    message: '"{name}" 이름의 빌드 소스가 이 워크스페이스에 이미 있습니다.',
    description: "Inline error when BUILD_SOURCE_NAME_TAKEN is returned",
  },
  "buildSources.fieldProject": {
    message: "프로젝트",
    description: "Create form label for optional Project scope",
  },
  "buildSources.fieldProjectDescription": {
    message: "선택적으로 이 빌드 소스를 단일 프로젝트로 제한합니다.",
    description: "Help text for optional Project selector",
  },
  "buildSources.fieldProjectNone": {
    message: "없음 (워크스페이스 전체)",
    description: "Option leaving the build source unscoped at workspace level",
  },
  "buildSources.fieldRuntime": {
    message: "런타임",
    description: "Create form label for Git build source runtime",
  },
  "buildSources.fieldRuntimeDescription": {
    message: "네이티브 런타임 또는 Docker를 선택하세요.",
    description: "Help text for Git build source runtime selector",
  },
  "buildSources.fieldBranch": {
    message: "브랜치",
    description: "Create form label for Git branch",
  },
  "buildSources.fieldBranchDescription": {
    message: "빌드할 Git 브랜치입니다. 비워 두면 저장소 기본 브랜치를 사용합니다.",
    description: "Help text for Git branch input",
  },
  "buildSources.fieldRegion": {
    message: "리전",
    description: "Create form label for Git build source region",
  },
  "buildSources.fieldRegionDescription": {
    message: "Git 빌드 소스는 연결된 서비스와 동일한 리전이어야 합니다.",
    description: "Help text for Git build source region selector",
  },
  "buildSources.fieldRegionDefault": {
    message: "기본 워크스페이스 리전",
    description: "Option leaving the build source region unset to use the platform default",
  },
  "buildSources.fieldGitRegistryDescription": {
    message: "Docker 빌드 중 비공개 베이스 이미지를 가져올 때 사용할 선택 자격 증명입니다.",
    description: "Help text for registry credential on Docker Git build sources",
  },
  "buildSources.fieldEnvGroups": {
    message: "빌드 시 환경 그룹",
    description: "Label for selecting workspace env groups in build-time environment",
  },
  "buildSources.fieldEnvGroupsDescription": {
    message:
      "워크스페이스 수준 환경 그룹을 빌드 시점에 연결합니다. 특정 환경에 속한 그룹은 선택할 수 없습니다.",
    description: "Help text for build-time env group checkboxes",
  },
  "buildSources.fieldEnvGroupsEmpty": {
    message: "사용 가능한 워크스페이스 수준 환경 그룹이 없습니다.",
    description: "Empty message when no workspace-level env groups exist",
  },
  "buildSources.linkServicesTitle": {
    message: "기존 서비스 연결",
    description: "Section heading for optional initial serviceIds linking",
  },
  "buildSources.linkServicesDescription": {
    message:
      "기존 Web Service, Private Service, Background Worker 또는 Cron Job을 선택적으로 연결합니다. Static Site는 빌드 소스를 사용할 수 없습니다.",
    description: "Section description for optional initial serviceIds linking",
  },
  "buildSources.linkServicesEmpty": {
    message: "이 워크스페이스에 아직 연결할 수 있는 서비스가 없습니다.",
    description: "Empty message when workspace has no linkable services",
  },
  "buildSources.projectSectionTitle": {
    message: "빌드 소스",
    description: "Heading of the Build Sources section on Project Overview",
  },
  "buildSources.projectLoadError": {
    message: "이 프로젝트의 빌드 소스를 불러오지 못했습니다.",
    description: "Error text inside Project Overview Build Sources section",
  },
  "buildSources.projectEmptyDescription": {
    message: "이 프로젝트에 연결된 빌드 소스가 아직 없습니다.",
    description: "Empty state text inside Project Overview Build Sources section",
  },
};

export default koBuildSources;
