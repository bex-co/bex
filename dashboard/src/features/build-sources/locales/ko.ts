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
  "buildSources.viewBuildSource": {
    message: "빌드 소스 열기",
    description: "Button navigating to a newly created Build Source detail page",
  },
  "buildSources.detailTabsLabel": {
    message: "빌드 소스 섹션",
    description: "Accessible label for the Build Source detail tab navigation",
  },
  "buildSources.tabBuilds": {
    message: "빌드",
    description: "Tab label for Build Source build history",
  },
  "buildSources.tabServices": {
    message: "서비스",
    description: "Tab label for Build Source linked services",
  },
  "buildSources.tabEnvironment": {
    message: "환경",
    description: "Tab label for Build Source build-time environment",
  },
  "buildSources.tabSettings": {
    message: "설정",
    description: "Tab label for Build Source settings and danger zone",
  },
  "buildSources.triggerBuildButton": {
    message: "빌드 트리거",
    description: "Primary CTA to trigger a new build on a Git build source",
  },
  "buildSources.triggerSubmitting": {
    message: "트리거 중…",
    description: "Submit button label while triggerBuild mutation is in flight",
  },
  "buildSources.triggerDialogTitle": {
    message: "빌드 트리거",
    description: "Title of the Trigger Build modal dialog",
  },
  "buildSources.triggerDialogDescription": {
    message: '"{name}"의 새 빌드를 시작하고 빌드 성공 시 배포할 연결된 서비스를 선택하세요.',
    description: "Description of the Trigger Build modal dialog",
  },
  "buildSources.triggerScopeLegend": {
    message: "빌드 성공 후 배포 대상",
    description: "Legend for the deploy.serviceScope radio group",
  },
  "buildSources.triggerScopeAutoDeploy": {
    message: "자동 배포가 켜진 연결 서비스",
    description: "Radio option for deploy.serviceScope=autodeploy_only",
  },
  "buildSources.triggerScopeAutoDeployHint": {
    message: "자동 배포가 활성화된 모든 연결 서비스를 배포합니다.",
    description: "Hint text for deploy.serviceScope=autodeploy_only",
  },
  "buildSources.triggerScopeNone": {
    message: "빌드만 실행 (배포 안 함)",
    description: "Radio option for deploy.serviceScope=none",
  },
  "buildSources.triggerScopeNoneHint": {
    message: "연결된 서비스를 배포하지 않고 빌드 아티팩트만 생성합니다.",
    description: "Hint text for deploy.serviceScope=none",
  },
  "buildSources.triggerScopeCustom": {
    message: "선택한 서비스",
    description: "Radio option for deploy.serviceScope=custom",
  },
  "buildSources.triggerScopeCustomHint": {
    message: "빌드가 성공하면 배포할 특정 연결 서비스를 선택하세요.",
    description: "Hint text for deploy.serviceScope=custom",
  },
  "buildSources.triggerScopeCustomNoServices": {
    message: "이 빌드 소스에 연결된 서비스가 아직 없습니다.",
    description: "Message inside custom deploy scope when serviceLinks is empty",
  },
  "buildSources.triggerScopeCustomSelectOne": {
    message: "배포할 서비스를 하나 이상 선택하세요.",
    description: "Validation hint when custom scope has zero checked services",
  },
  "buildSources.triggerClearCache": {
    message: "빌드 캐시 지우기",
    description: "Checkbox label for clearCache=clear",
  },
  "buildSources.triggerClearCacheHint": {
    message: "캐시된 레이어를 재사용하지 않고 처음부터 다시 빌드합니다.",
    description: "Hint text for clearCache checkbox",
  },
  "buildSources.triggerResultTitle": {
    message: "빌드가 시작되었습니다",
    description: "Confirmation heading in Trigger Build dialog after success",
  },
  "buildSources.triggerResultDescription": {
    message: "빌드 {id}이(가) 대기열에 추가되었습니다.",
    description: "Confirmation summary naming the new bld- ID",
  },
  "buildSources.triggerResultDeploysTitle_other": {
    message: "{count}개 서비스 배포가 대기열에 추가되었습니다",
    description: "Plural heading listing fan-out deploys queued by triggerBuild",
  },
  "buildSources.triggerResultNoDeploys": {
    message: "이 빌드에 대해 대기열에 추가된 서비스 배포가 없습니다.",
    description: "Message when triggerBuild returned an empty deploys array",
  },
  "buildSources.triggerResultSkippedTitle_other": {
    message: "연결된 서비스 {count}개를 건너뛰었습니다",
    description: "Plural alert heading when triggerBuild returns skippedServices",
  },
  "buildSources.triggerResultViewLogs": {
    message: "빌드 로그 보기",
    description: "Primary action navigating from Trigger Build confirmation to Build Detail",
  },
  "buildSources.skippedReasonSuspended": {
    message: "일시 중지됨",
    description: "Badge label when a service was skipped because it is suspended",
  },
  "buildSources.buildStatusSucceeded": {
    message: "성공",
    description: "Build status badge for succeeded builds",
  },
  "buildSources.buildStatusFailed": {
    message: "실패",
    description: "Build status badge for failed builds",
  },
  "buildSources.buildStatusCanceled": {
    message: "취소됨",
    description: "Build status badge for canceled builds",
  },
  "buildSources.buildStatusInProgress": {
    message: "빌드 중",
    description: "Build status badge for in-progress builds",
  },
  "buildSources.buildStatusCreated": {
    message: "대기 중",
    description: "Build status badge for newly queued builds",
  },
  "buildSources.buildStatusUnknown": {
    message: "알 수 없음",
    description: "Fallback build status badge",
  },
  "buildSources.buildsEmptyTitle": {
    message: "아직 빌드가 없습니다",
    description: "Empty state heading on the Builds tab",
  },
  "buildSources.buildsEmptyGitDescription": {
    message: "수동으로 빌드를 트리거하거나 추적 브랜치에 푸시하여 첫 빌드 아티팩트를 만드세요.",
    description: "Empty state description on Builds tab for Git build sources",
  },
  "buildSources.buildsEmptyImageDescription": {
    message: "Docker 이미지 빌드 소스는 연결된 서비스가 배포될 때 이미지 리비전을 기록합니다.",
    description: "Empty state description on Builds tab for image build sources",
  },
  "buildSources.buildsRefreshError": {
    message: "빌드 목록을 새로고침하지 못했습니다. 캐시된 결과를 표시합니다.",
    description: "Inline error banner when polling builds fails over cached rows",
  },
  "buildSources.buildsLoadMore": {
    message: "빌드 더 불러오기",
    description: "Pagination button loading older builds",
  },
  "buildSources.colBuildId": {
    message: "빌드 ID",
    description: "Table header for build ID column",
  },
  "buildSources.colStatus": {
    message: "상태",
    description: "Table header for build status column",
  },
  "buildSources.colArtifact": {
    message: "커밋 / 이미지",
    description: "Table header for commit SHA or image digest column",
  },
  "buildSources.colDuration": {
    message: "소요 시간",
    description: "Table header for build duration column",
  },
  "buildSources.colDeploys": {
    message: "배포",
    description: "Table header for fan-out deploys count column",
  },
  "buildSources.colStarted": {
    message: "시작 시간",
    description: "Table header for build start timestamp column",
  },
  "buildSources.buildDeploysCount_other": {
    message: "배포 {count}개",
    description: "Plural badge count of fan-out deploys on a build",
  },
  "buildSources.buildSkippedCount_other": {
    message: "건너뜀 {count}개",
    description: "Plural badge count of skipped services on a build",
  },
  "buildSources.backToBuilds": {
    message: "빌드 목록으로 돌아가기",
    description: "Back link from Build Detail to the Builds tab",
  },
  "buildSources.copyBuildId": {
    message: "빌드 ID 복사",
    description: "Accessible label for copying a bld- ID",
  },
  "buildSources.buildDetailSubtitle": {
    message: "{name}의 빌드",
    description: "Subtitle under the bld- ID on Build Detail",
  },
  "buildSources.buildDetailRefreshError": {
    message: "빌드 세부 정보를 새로고침하지 못했습니다. 마지막으로 불러온 상태를 표시합니다.",
    description: "Inline error banner when build detail polling fails",
  },
  "buildSources.factStarted": {
    message: "시작 시간",
    description: "Fact label for build start timestamp",
  },
  "buildSources.factCommit": {
    message: "커밋",
    description: "Fact label for build commit SHA",
  },
  "buildSources.factArtifact": {
    message: "아티팩트",
    description: "Fact label for built image reference and digest",
  },
  "buildSources.fanoutTitle": {
    message: "연결된 서비스 배포",
    description: "Card heading listing fan-out deploys on Build Detail",
  },
  "buildSources.fanoutEmpty": {
    message: "이 빌드에서 배포된 서비스가 아직 없습니다.",
    description: "Empty message inside fan-out deploys card",
  },
  "buildSources.buildLogsSearchLabel": {
    message: "빌드 로그 필터링",
    description: "Accessible label for search input in BuildLogPanel",
  },
  "buildSources.buildLogsEmptyTitle": {
    message: "아직 빌드 로그가 없습니다",
    description: "Empty state heading in BuildLogPanel",
  },
  "buildSources.buildLogsEmptyBody": {
    message: "빌드 출력이 시작되면 여기에 로그가 표시됩니다.",
    description: "Empty state body in BuildLogPanel",
  },
  "buildSources.imageBuildLogsNotice": {
    message: "Docker 이미지 빌드 소스는 기존 이미지를 재사용하므로 빌드 로그를 생성하지 않습니다.",
    description: "Notice shown on Build Detail for image-backed build sources",
  },
  "buildSources.servicesTabTitle": {
    message: "연결된 서비스",
    description: "Heading of the Linked Services tab",
  },
  "buildSources.servicesTabDescription": {
    message: "이 빌드 소스에 연결된 서비스는 해당 빌드에서 생성된 아티팩트를 배포합니다.",
    description: "Description of the Linked Services tab",
  },
  "buildSources.linkExistingService": {
    message: "기존 서비스 연결",
    description: "Button opening the Link Existing Service dialog",
  },
  "buildSources.servicesEmptyTitle": {
    message: "연결된 서비스 없음",
    description: "Empty state heading on Linked Services tab",
  },
  "buildSources.servicesEmptyDescription": {
    message: "기존 Web Service, Private Service, Background Worker 또는 Cron Job을 연결해 이 소스의 빌드를 공유하세요.",
    description: "Empty state description on Linked Services tab",
  },
  "buildSources.autoDeployOn": {
    message: "자동 배포 켜짐",
    description: "Badge indicating a linked service auto-deploys new builds",
  },
  "buildSources.autoDeployOff": {
    message: "자동 배포 꺼짐",
    description: "Badge indicating a linked service requires manual deploys",
  },
  "buildSources.changeOrUnlinkSource": {
    message: "소스 변경",
    description: "Link to a service's settings page to switch or unlink its source",
  },
  "buildSources.linkDialogTitle": {
    message: "기존 서비스 연결",
    description: "Heading of the Link Existing Service dialog",
  },
  "buildSources.linkDialogDescription": {
    message: '워크스페이스의 기존 서비스를 "{name}"에 연결합니다. 연결은 배포를 트리거하지 않고 저장됩니다.',
    description: "Description of the Link Existing Service dialog",
  },
  "buildSources.linkDialogNoEligible": {
    message: "이 워크스페이스에서 연결 가능한 서비스가 모두 이미 연결되었거나 아직 없습니다.",
    description: "Message when no linkable unlinked services exist",
  },
  "buildSources.linkDialogServiceLabel": {
    message: "서비스",
    description: "Select label in Link Existing Service dialog",
  },
  "buildSources.linkDialogServicePlaceholder": {
    message: "서비스 선택…",
    description: "Select placeholder in Link Existing Service dialog",
  },
  "buildSources.linkDialogRegionMismatch": {
    message: "리전 불일치: 이 Git 빌드 소스는 {sourceRegion}에 있지만 선택한 서비스는 {serviceRegion}에 있습니다.",
    description: "Inline error when linking a Git build source to a service in another region",
  },
  "buildSources.linkDialogStartCommandHint": {
    message: "네이티브 런타임 빌드 소스에서는 서비스 시작 명령어가 필수입니다.",
    description: "Help text for startCommand input in Link Existing Service dialog",
  },
  "buildSources.linkDialogSubmit": {
    message: "서비스 연결",
    description: "Submit button label in Link Existing Service dialog",
  },
  "buildSources.envImageUnsupportedTitle": {
    message: "Docker 이미지 소스에서는 빌드 시점 환경을 사용할 수 없습니다",
    description: "Fallback heading if an image build source visits /environment",
  },
  "buildSources.envImageUnsupportedBody": {
    message: "빌드 시점 환경 변수와 시크릿 파일은 Git 기반 빌드 소스에서만 지원됩니다.",
    description: "Fallback body if an image build source visits /environment",
  },
  "buildSources.envSaveOnlyNote": {
    message: "변경 사항은 빌드를 시작하지 않고 저장되며 다음 빌드부터 적용됩니다.",
    description: "Top note on Build-time Environment tab",
  },
  "buildSources.envVarsDescription": {
    message: "이 빌드 소스의 빌드 중에 사용할 수 있습니다. 변경 사항은 다음 빌드에 적용됩니다.",
    description: "Environment Variables card description on Build-time Environment tab",
  },
  "buildSources.secretFilesDescription": {
    message: "이 빌드 소스의 빌드 중에 마운트됩니다. 변경 사항은 다음 빌드에 적용됩니다.",
    description: "Secret Files card description on Build-time Environment tab",
  },
  "buildSources.envGroupsPanelDescription": {
    message: "빌드 시점에 연결되는 워크스페이스 수준 환경 그룹입니다. 변경 사항은 배포 없이 다음 빌드에 적용됩니다.",
    description: "Environment Groups card description on Build-time Environment tab",
  },
  "buildSources.envGroupsNoneLinked": {
    message: "이 빌드 소스에 연결된 환경 그룹이 없습니다.",
    description: "Empty state when no env groups are linked to a build source",
  },
  "buildSources.settingsIdentityTitle": {
    message: "식별자 및 범위",
    description: "Heading of read-only identity card on Settings tab",
  },
  "buildSources.settingsIdentityDescription": {
    message: "이 빌드 소스의 변경할 수 없는 식별자와 범위입니다.",
    description: "Description of read-only identity card on Settings tab",
  },
  "buildSources.settingsFieldId": {
    message: "빌드 소스 ID",
    description: "Label for read-only bds- ID field on Settings tab",
  },
  "buildSources.settingsConfigTitle": {
    message: "빌드 구성",
    description: "Heading of editable settings card on Settings tab",
  },
  "buildSources.settingsNoBuildNote": {
    message: "설정을 저장해도 자동으로 빌드가 시작되지 않습니다.",
    description: "Inline note explaining Settings edits do not rebuild",
  },
  "buildSources.settingsFieldRepoUrl": {
    message: "리포지토리 URL",
    description: "Input label for Git repository URL on Settings tab",
  },
  "buildSources.settingsInvalidRepoUrl": {
    message: "유효한 Git 리포지토리 URL을 입력하세요.",
    description: "Validation error for malformed repo URL on Settings tab",
  },
  "buildSources.settingsInvalidImageUrl": {
    message: "유효한 Docker 이미지 참조를 입력하세요.",
    description: "Validation error for malformed image URL on Settings tab",
  },
  "buildSources.settingsSaveButton": {
    message: "변경 사항 저장",
    description: "Submit button label on Settings tab",
  },
  "buildSources.settingsSaving": {
    message: "저장 중…",
    description: "Submit button label while updateBuildSource is in flight",
  },
  "buildSources.dangerZoneTitle": {
    message: "위험 구역",
    description: "Heading of Delete Build Source card on Settings tab",
  },
  "buildSources.dangerZoneDescription": {
    message: "이 빌드 소스와 빌드 시점 구성을 영구적으로 삭제합니다. 이 작업은 되돌릴 수 없습니다.",
    description: "Description of Delete Build Source card",
  },
  "buildSources.deleteBlockedTitle_other": {
    message: "삭제하기 전에 {count}개 서비스의 연결을 해제하세요",
    description: "Plural alert heading blocking delete when services remain linked",
  },
  "buildSources.deleteBlockedDescription": {
    message: "서비스가 참조하고 있는 빌드 소스는 삭제할 수 없습니다. 먼저 연결된 각 서비스를 다른 소스로 전환하세요:",
    description: "Explanation under delete-blocked alert",
  },
  "buildSources.deleteButton": {
    message: "빌드 소스 삭제",
    description: "Destructive button opening the delete confirmation dialog",
  },
  "buildSources.deleteConfirmTitle": {
    message: '"{name}"을(를) 삭제하시겠습니까?',
    description: "Title of Delete Build Source confirmation dialog",
  },
  "buildSources.deleteConfirmBody": {
    message: "이 빌드 소스와 빌드 시점 환경이 영구적으로 삭제됩니다. 빌드 기록과 구성은 복구할 수 없습니다.",
    description: "Body of Delete Build Source confirmation dialog",
  },
  "buildSources.deleteConfirmPrompt": {
    message: "확인하려면 아래에 {phrase}을(를) 입력하세요.",
    description: "Sudo command prompt inside Delete Build Source confirmation dialog",
  },
  "buildSources.deleteConfirmButton": {
    message: "빌드 소스 삭제",
    description: "Confirm action label inside Delete Build Source confirmation dialog",
  },
};

export default koBuildSources;
