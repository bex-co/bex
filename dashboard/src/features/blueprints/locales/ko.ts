import type { TranslationEntry } from "@/i18n";

const koBlueprints: Record<string, TranslationEntry> = {
  "blueprints.resourceType": {
    message: "Blueprint",
    description: "Blueprint resource type used in document titles",
  },
  "blueprints.pageTitle": {
    message: "Blueprints",
    description: "Blueprints list page heading and document title",
  },
  "blueprints.cardTitle": {
    message: "Blueprints",
    description: "Blueprints table card title",
  },
  "blueprints.colName": {
    message: "이름",
    description:
      "Blueprints table column header — blueprint name (derived from repo)",
  },
  "blueprints.colRepo": {
    message: "저장소",
    description: "Blueprints table column header — source repo URL",
  },
  "blueprints.colBranch": {
    message: "브랜치",
    description: "Blueprints table column header — tracked branch",
  },
  "blueprints.colStatus": {
    message: "상태",
    description: "Blueprints table column header",
  },
  "blueprints.colUpdated": {
    message: "마지막 업데이트",
    description: "Blueprints table column header — updatedAt relative age",
  },
  "blueprints.createButton": {
    message: "새 Blueprint 인스턴스",
    description: "Button label that opens the New Blueprint dialog",
  },
  "blueprints.createTitle": {
    message: "새 Blueprint 인스턴스",
    description: "New Blueprint page title",
  },
  "blueprints.createDescription": {
    message:
      "render.yaml Blueprint가 포함된 저장소를 연결하여 서비스 스택 전체를 만들고 동기화하세요.",
    description: "New Blueprint page subtitle under the title",
  },
  "blueprints.createSourceTitle": {
    message: "저장소 연결",
    description:
      "New Blueprint page — heading above the GitHub/Public Git source tabs (Render parity)",
  },
  "blueprints.createBranchLabel": {
    message: "브랜치",
    description: "New Blueprint page — branch combobox label",
  },
  "blueprints.createBranchPlaceholder": {
    message: "main",
    description: "New Blueprint page — branch combobox placeholder",
  },
  "blueprints.createBranchHint": {
    message: "render.yaml 파일이 있는 저장소 브랜치입니다.",
    description: "New Blueprint page — branch field helper text",
  },
  "blueprints.createPathLabel": {
    message: "Blueprint 경로",
    description: "New Blueprint page — render.yaml path input label",
  },
  "blueprints.createNamePlaceholder": {
    message: "my-stack",
    description: "New Blueprint page — blueprint name placeholder",
  },
  "blueprints.createPathPlaceholder": {
    message: "render.yaml",
    description: "New Blueprint page — manifest path placeholder",
  },
  "blueprints.createPathHint": {
    message:
      "저장소 내 Blueprint 파일의 경로입니다. .yaml/.yml 파일이면 모두 사용할 수 있습니다(예: infra/bex/stack.yaml). 기본값은 저장소 루트의 render.yaml입니다.",
    description: "New Blueprint page — manifest path helper text",
  },
  "blueprints.createNameLabel": {
    message: "Blueprint 이름",
    description: "New Blueprint page — name input label",
  },
  "blueprints.createNameTaken": {
    message:
      "이 워크스페이스에 {name} 이름의 Blueprint가 이미 있습니다. 다른 이름을 선택하세요.",
    description:
      "New Blueprint page — shown under the name field when Deploy was refused because an approved Blueprint already has the name (w8/250)",
  },
  "blueprints.createAction": {
    message: "Blueprint 배포",
    description: "New Blueprint dialog submit button label",
  },
  "blueprints.createCancel": {
    message: "취소",
    description: "New Blueprint dialog cancel button label",
  },
  "blueprints.createSuccess": {
    message: "Blueprint를 만들었습니다",
    description: "Toast shown after a successful createBlueprint call",
  },
  "blueprints.createError": {
    message: "Blueprint 생성에 실패했습니다",
    description: "Toast shown when createBlueprint returns an error",
  },
  "blueprints.previewTitle": {
    message: "Blueprint 구성 검토",
    description:
      "New Blueprint page — heading of the pre-create fetch + validate section",
  },
  "blueprints.previewSelectSource": {
    message:
      "배포하기 전에 Blueprint 파일을 검토하려면 저장소와 브랜치를 선택하세요.",
    description: "Review section placeholder before a source is chosen",
  },
  "blueprints.previewLoading": {
    message: "저장소에서 {path} 파일을 가져오는 중…",
    description: "Review section loading text while the manifest is fetched",
  },
  "blueprints.previewNotFoundTitle": {
    message: "Blueprint 파일을 찾을 수 없습니다",
    description: "Review section error title when the manifest fetch fails",
  },
  "blueprints.previewNotFoundBody": {
    message:
      "{branch} 브랜치에서 Blueprint 파일 {path} 파일을 찾을 수 없습니다.",
    description:
      "Review section fallback error body when the backend returns no message",
  },
  "blueprints.previewFileNotFoundTitle": {
    message: "Blueprint 파일을 찾을 수 없습니다",
    description:
      "Preview failure title: the repo and branch resolved, the file did not",
  },
  "blueprints.previewFileNotFoundBody": {
    message:
      "{branch} 브랜치에 {path} 파일이 없습니다. 경로를 확인하거나 해당 브랜치에 파일을 커밋하세요.",
    description: "Preview failure body for file_not_found",
  },
  "blueprints.previewBranchNotFoundTitle": {
    message: "브랜치를 찾을 수 없습니다",
    description: "Preview failure title for branch_not_found",
  },
  "blueprints.previewBranchNotFoundBody": {
    message: "이 저장소에 {branch} 브랜치가 없습니다.",
    description: "Preview failure body for branch_not_found",
  },
  "blueprints.previewRepoNotFoundTitle": {
    message: "저장소를 찾을 수 없습니다",
    description: "Preview failure title for repo_not_found_or_no_access",
  },
  "blueprints.previewRepoNotFoundBody": {
    message:
      "이 저장소가 존재하지 않거나 bex의 GitHub 앱이 접근할 수 없습니다. 저장소와 앱의 저장소 접근 권한을 확인하세요.",
    description: "Preview failure body for repo_not_found_or_no_access",
  },
  "blueprints.previewAccessDeniedTitle": {
    message: "접근이 거부되었습니다",
    description: "Preview failure title for access_denied",
  },
  "blueprints.previewAccessDeniedBody": {
    message:
      "bex의 GitHub 앱에 이 저장소를 읽을 권한이 없습니다. 설치 상태와 앱에 부여된 저장소를 다시 확인하세요.",
    description: "Preview failure body for access_denied",
  },
  "blueprints.previewPublicRepoNotFoundBody": {
    message:
      "이 저장소가 {host}에 존재하지 않거나 공개되어 있지 않습니다. bex는 그곳에서 공개 저장소의 Blueprint만 읽습니다.",
    description:
      "Preview failure body for repo_not_found_or_no_access on gitlab.com or bitbucket.org, read anonymously; {host} is that host",
  },
  "blueprints.previewPublicAccessDeniedBody": {
    message:
      "{host}에서 bex의 이 저장소 읽기를 허용하지 않았습니다. bex는 그곳에서 공개 저장소의 Blueprint만 읽습니다.",
    description:
      "Preview failure body for access_denied on gitlab.com or bitbucket.org, read anonymously; {host} is that host",
  },
  "blueprints.previewHostUnsupportedTitle": {
    message: "지원되지 않는 Git 호스트",
    description:
      "Preview failure title for a repository on a Git host bex reads no Blueprint from",
  },
  "blueprints.previewHostUnsupportedBody": {
    message:
      "bex는 Render와 마찬가지로 github.com, gitlab.com 또는 bitbucket.org의 공개 저장소에서 Blueprint를 읽습니다. {host}은(는) 여기에 해당하지 않습니다.",
    description:
      "Preview failure body for a repository on a Git host bex reads no Blueprint from; {host} is that host",
  },
  "blueprints.previewRateLimitedTitle": {
    message: "{host}에서 bex의 요청을 제한하고 있습니다",
    description:
      "Preview failure title for rate_limited; {host} is GitHub or the repository's host",
  },
  "blueprints.previewRateLimitedBody": {
    message: "현재 {host}에 요청이 너무 많습니다. 몇 분 후에 다시 시도하세요.",
    description:
      "Preview failure body for rate_limited; {host} is GitHub or the repository's host",
  },
  "blueprints.previewAmbiguousTitle": {
    message: "Blueprint 파일이 두 개 있습니다",
    description: "Preview failure title for ambiguous_filename",
  },
  "blueprints.previewAmbiguousBody": {
    message:
      "이 저장소의 {branch} 브랜치에 render.yaml과 bex.yml이 모두 있습니다. 하나를 제거하거나 위에서 파일을 직접 지정하세요.",
    description: "Preview failure body for ambiguous_filename",
  },
  "blueprints.previewUnavailableTitle": {
    message: "{host}에 연결할 수 없습니다",
    description:
      "Preview failure title for unavailable; {host} is GitHub or the repository's host",
  },
  "blueprints.previewUnavailableBody": {
    message:
      "bex가 {host}에서 Blueprint 파일을 읽지 못했습니다. 몇 분 후에 다시 시도하세요.",
    description:
      "Preview failure body for unavailable; {host} is GitHub or the repository's host",
  },
  "blueprints.previewInvalidPathExtension": {
    message: "Blueprint 경로는 .yaml 또는 .yml 파일이어야 합니다.",
    description: "Inline path-field error for invalid_path, wrong extension",
  },
  "blueprints.previewInvalidPathShape": {
    message:
      "Blueprint 경로는 저장소 기준의 깔끔한 상대 경로여야 합니다(앞의 슬래시 및 .. 불가).",
    description: "Inline path-field error for invalid_path, bad shape",
  },
  "blueprints.previewRetry": {
    message: "다시 시도",
    description: "Review section retry button after a failed manifest fetch",
  },
  "blueprints.takeoverResourceTitle": {
    message: "“{name}” 리소스를 인수하시겠습니까?",
    description:
      "Takeover dialog title when a resource is managed by another Blueprint (BLUEPRINT_RESOURCE_CONFLICT)",
  },
  "blueprints.takeoverResourceBody": {
    message:
      "“{name}” {kind} 항목은 Blueprint {owner}에서 관리 중입니다. 계속하면 이 Blueprint로 이전됩니다. 계속하려면 아래 명령을 입력하세요.",
    description: "Takeover dialog body for a Blueprint resource conflict",
  },
  "blueprints.takeoverResourceBodyOtherOwner": {
    message:
      "“{name}” {kind} 항목은 다른 Blueprint에서 관리 중입니다. 계속하면 이 Blueprint로 이전됩니다. 계속하려면 아래 명령을 입력하세요.",
    description:
      "Takeover dialog body for a Blueprint resource conflict whose owning Blueprint bex-api could not name",
  },
  "blueprints.previewConflictBody": {
    message:
      "다른 Blueprint가 이 리소스 중 일부를 이미 관리하고 있습니다. 배포하면 이 Blueprint로 이전되며, 인수되는 항목은 다음 단계에서 정확히 확인할 수 있습니다.",
    description:
      "Preview alert body on /blueprint/new when the only problem is a takeover conflict (replaces the raw API message)",
  },
  "blueprints.previewConflict": {
    message: "이미 관리 중 — 배포하면 인수됩니다",
    description:
      "Alert title on /blueprint/new when the only preview problem is a resource another Blueprint manages, which an explicit takeover confirmation resolves (w4/m125)",
  },
  "blueprints.previewMatchTitle": {
    message: "기존 리소스와 일치합니다",
    description:
      "Notice title on /blueprint/new when the Blueprint file names resources that already exist in the workspace (w8/m153, Render parity)",
  },
  "blueprints.previewMatchBody": {
    message:
      "이 Blueprint는 워크스페이스에 이미 있는 리소스와 일치합니다. 새 리소스는 각 이름에 “-{suffix}” 접미사가 붙어 생성되며, 기존 리소스는 변경되지 않습니다.",
    description:
      "Notice body on /blueprint/new for matched resources; {suffix} is the four-character resource name suffix, shown after a hyphen as it appears in a name",
  },
  "blueprints.previewMatchListLabel": {
    message: "이미 워크스페이스에 있음:",
    description:
      "Notice on /blueprint/new — label above the list of existing resources the Blueprint file matches",
  },
  "blueprints.previewInvalid": {
    message: "Blueprint 파일에 오류가 있습니다",
    description:
      "Review section error title when the fetched manifest fails validation",
  },
  "blueprints.previewValid_other": {
    message:
      "Blueprint 파일을 성공적으로 파싱했습니다. 리소스 {count}개가 변경됩니다.",
    description:
      "Review section success line, with the plan's total resource count (plural)",
  },
  "blueprints.previewValidEnvironments_other": {
    message:
      "Blueprint 파일을 성공적으로 파싱했습니다. 환경 {count}개가 변경됩니다.",
    description:
      "Review section success line when only environments change (w9/095, plural)",
  },
  "blueprints.previewValidWithEnvironments_other": {
    message:
      "Blueprint 파일을 성공적으로 파싱했습니다. 리소스 {count}개와 {environments}가 변경됩니다.",
    description:
      "Review section success line: changing resources (plural) and blueprints.previewEnvironmentCount (w9/095)",
  },
  "blueprints.previewEnvironmentCount_other": {
    message: "환경 {count}개",
    description:
      "The changing environments inside blueprints.previewValidWithEnvironments (plural)",
  },
  "blueprints.errorAtLineColumn": {
    message: "{line}행, {column}열",
    description: "Source location beside a Blueprint validation error (w4/151)",
  },
  "blueprints.errorAtLine": {
    message: "{line}행",
    description:
      "Source location beside a Blueprint validation error when only the line is known (w4/151)",
  },
  "blueprints.previewNoChanges": {
    message:
      "Blueprint 파일을 성공적으로 파싱했습니다. 변경 사항이 없습니다. 모든 리소스가 이미 이 파일과 일치합니다.",
    description:
      "Review section success line when every planned action is a no-op (w4/m138)",
  },
  "blueprints.previewOpCreate": {
    message: "생성",
    description:
      "Plan row badge: the resource does not exist yet and will be created",
  },
  "blueprints.previewOpUpdate": {
    message: "업데이트",
    description: "Plan row badge: an existing resource will be changed",
  },
  "blueprints.previewOpNoop": {
    message: "변경 없음",
    description: "Plan row badge: the resource already matches the file",
  },
  "blueprints.previewOpError": {
    message: "적용 불가",
    description:
      "Plan row badge: the planner refused this resource (reason below)",
  },
  "blueprints.previewChangedFields": {
    message: "변경: {fields}",
    description:
      "Plan row detail for an update: the changed field paths (never values)",
  },
  "blueprints.previewKindService": {
    message: "서비스",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindPostgres": {
    message: "Postgres",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindKeyValue": {
    message: "Key Value",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindEnvGroup": {
    message: "환경 그룹",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindBuildSource": {
    message: "빌드 소스",
    description: "Plan row resource kind",
  },
  "blueprints.previewKindEnvironment": {
    message: "환경",
    description: "Plan row resource kind: a projects[].environments[] entry",
  },
  "blueprints.previewServices": {
    message: "서비스",
    description: "Review section plan group label — services in the manifest",
  },
  "blueprints.previewDatabases": {
    message: "데이터베이스",
    description: "Review section plan group label — databases in the manifest",
  },
  "blueprints.previewKeyValue": {
    message: "Key Value",
    description:
      "Review section plan group label — key-value instances in the manifest",
  },
  "blueprints.previewEnvGroups": {
    message: "환경 그룹",
    description: "Review section plan group label — env groups in the manifest",
  },
  "blueprints.previewManualSyncNote": {
    message:
      "bex는 이 저장소의 푸시를 받지 못하므로 이후 Blueprint 파일을 변경해도 자동으로 동기화되지 않습니다. 변경할 때마다 Blueprint를 동기화하거나 GitHub 앱으로 저장소를 연결하세요.",
    description:
      "Review section note for a repo whose pushes can't reach bex (the preview's autoSync is false): the Blueprint is created without auto-sync",
  },
  "blueprints.previewAutoSyncNote": {
    message:
      "이 브랜치에서 Blueprint 파일에 대한 이후의 모든 업데이트가 자동으로 동기화됩니다. 생성 후 자동 동기화를 일시 중지할 수 있습니다.",
    description:
      "Review section note about auto-sync being enabled by default (Render parity)",
  },
  "blueprints.previewError": {
    message: "Blueprint 미리보기를 불러오지 못했습니다.",
    description:
      "Review section error shown when the preview query itself fails (network)",
  },
  "blueprints.promptTitle": {
    message: "시크릿 값",
    description:
      "Review section — heading for the sync:false env var prompt inputs",
  },
  "blueprints.promptHint": {
    message:
      "이 Blueprint는 sync: false로 환경 변수를 선언합니다. 해당 값은 저장소가 아닌 bex에만 저장됩니다. 지금 설정하세요. 값은 한 번만 입력되며 이후 동기화에서 덮어쓰지 않습니다.",
    description: "sync:false prompt section helper text",
  },
  "blueprints.promptEmptyWarning": {
    message:
      "비워 둔 값은 미설정 상태로 배포되므로, 서비스의 환경 탭에서 설정할 때까지 서비스가 실패할 수 있습니다.",
    description:
      "Warning below the sync:false inputs when at least one is blank",
  },
  "blueprints.pricingTitle": {
    message: "예상 요금",
    description: "Review section — estimated-pricing panel heading",
  },
  "blueprints.pricingSubtitle": {
    message:
      "컴퓨팅 비용은 초 단위로 일할 계산됩니다. 결제는 월초에 이루어집니다.",
    description:
      "Estimated-pricing panel subtitle explaining billing semantics (Render parity)",
  },
  "blueprints.pricingLineAmount": {
    message: "({tier}) ${amount} / 월",
    description:
      "Estimated-pricing row amount — plan label + monthly dollar cost",
  },
  "blueprints.pricingLineBreakdown": {
    message: "인스턴스 ${instance} + 디스크 ({gb} GB) ${storage}",
    description:
      "Estimated-pricing datastore row sub-line — instance vs provisioned-storage cost breakdown",
  },
  "blueprints.pricingVariable": {
    message: "변동",
    description:
      "Estimated-pricing row amount for resources whose cost depends on runtime behavior",
  },
  "blueprints.pricingReasonAutoscaling": {
    message: "자동 확장",
    description:
      "Estimated-pricing variable-row badge — service has autoscaling enabled",
  },
  "blueprints.pricingReasonMultiInstance": {
    message: "다중 인스턴스",
    description:
      "Estimated-pricing variable-row badge — service declares numInstances > 1",
  },
  "blueprints.pricingReasonCron": {
    message: "Cron Job",
    description:
      "Estimated-pricing variable-row badge — cron jobs bill only while runs execute",
  },
  "blueprints.pricingTotalLabel": {
    message: "합계",
    description: "Estimated-pricing panel total row label",
  },
  "blueprints.pricingTotalAmount": {
    message: "월 ${amount}{marker}",
    description:
      "Estimated-pricing panel total amount; marker is an asterisk when variable costs are excluded",
  },
  "blueprints.pricingExclusions": {
    message: "* 제외 항목: {items}.",
    description:
      "Estimated-pricing footnote listing the variable costs excluded from the total",
  },
  "blueprints.pricingExcludeAutoscaling": {
    message: "자동 확장",
    description: "Excluded-cost phrase in the estimated-pricing footnote",
  },
  "blueprints.pricingExcludeMultiInstance": {
    message: "추가 고정 인스턴스",
    description: "Excluded-cost phrase in the estimated-pricing footnote",
  },
  "blueprints.pricingExcludeCron": {
    message: "Cron Job",
    description: "Excluded-cost phrase in the estimated-pricing footnote",
  },
  "blueprints.pricingEstimateNote": {
    message:
      "추정치일 뿐입니다. 스토리지는 프로비저닝된 크기를 기준으로 산정되며, 청구서에는 실제 사용량이 반영됩니다.",
    description:
      "Estimated-pricing disclaimer: forward estimate is an upper bound of metered billing",
  },
  "blueprints.generateButton": {
    message: "Blueprint 생성",
    description: "Blueprints list header — opens the export dialog (w8/m22)",
  },
  "blueprints.generateTitle": {
    message: "Blueprint 생성",
    description: "Generate dialog title",
  },
  "blueprints.generateDescription": {
    message:
      "기존 리소스를 선택하여 저장소에 커밋하고 Blueprint로 연결할 수 있는 render.yaml로 내보냅니다.",
    description: "Generate dialog description",
  },
  "blueprints.generateEmptyHint": {
    message: "생성할 리소스를 하나 이상 선택하세요.",
    description: "Generate dialog hint when nothing is selected",
  },
  "blueprints.generateAction": {
    message: "생성",
    description: "Generate dialog primary action",
  },
  "blueprints.generateBack": {
    message: "뒤로",
    description: "Generate dialog — return from preview to selection",
  },
  "blueprints.generateCopy": {
    message: "복사",
    description: "Generate dialog — copy the yaml to the clipboard",
  },
  "blueprints.generateCopied": {
    message: "render.yaml을 복사했습니다",
    description: "Toast after copying the generated manifest",
  },
  "blueprints.generateCopyFailed": {
    message:
      "render.yaml을 복사하지 못했습니다. 클립보드 접근을 허용한 후 다시 시도하거나 다운로드하세요.",
    description:
      "Toast when the browser refuses to copy the generated manifest to the clipboard",
  },
  "blueprints.generateDownload": {
    message: "render.yaml 다운로드",
    description: "Generate dialog — download the manifest file",
  },
  "blueprints.generateSecretsNote": {
    message:
      "저장된 값은 내보내지 않습니다. sync: false로 표시된 서비스 변수는 Blueprint를 처음 만들 때 입력을 요청합니다. generateValue로 표시된 변수(내보낸 환경 그룹 키 포함)는 기존 값을 유지하며, 없으면 새 값이 생성됩니다.",
    description: "Generate dialog note under the yaml preview",
  },
  "blueprints.generateError": {
    message: "Blueprint를 생성하지 못했습니다",
    description: "Toast when the generate query fails",
  },
  "blueprints.emptyTitle": {
    message: "아직 만든 Blueprint 인스턴스가 없습니다.",
    description: "Blueprints list empty-state heading",
  },
  "blueprints.emptyBody": {
    message:
      "저장소 기반 render.yaml을 배포하면 Blueprint가 자동으로 등록됩니다. 배포하면 스택이 여기에 표시되며 동기화하거나 검증할 수 있습니다.",
    description:
      "Blueprints list empty-state body explaining auto-registration",
  },
  "blueprints.loadingBody": {
    message: "Blueprint를 불러오는 중…",
    description: "Blueprints list loading state label",
  },
  "blueprints.errorTitle": {
    message: "Blueprint를 불러오지 못했습니다",
    description: "Blueprints list error state heading",
  },
  "blueprints.statusActive": {
    message: "활성",
    description: "Blueprint status badge — row is active",
  },
  "blueprints.statusInSync": {
    message: "동기화됨",
    description: "Blueprint status badge — git-connected and in sync",
  },
  "blueprints.statusSyncing": {
    message: "동기화 중",
    description: "Blueprint status badge — sync in progress",
  },
  "blueprints.statusError": {
    message: "오류",
    description: "Blueprint status badge — last sync failed",
  },
  "blueprints.statusPaused": {
    message: "일시 중지됨",
    description: "Blueprint status badge — auto-sync paused",
  },
  "blueprints.statusUnknown": {
    message: "{status}",
    description: "Blueprint status badge fallback — raw status value",
  },
  "blueprints.resourceSuffixLabel": {
    message: "리소스 이름 접미사",
    description:
      "Tooltip and screen-reader label of the suffix shown beside a Blueprint's name in the list and on its detail page (w8/m153)",
  },
  "blueprints.detailTitle": {
    message: "{name} · Blueprints",
    description: "Blueprint detail page document title",
  },
  "blueprints.metaRepo": {
    message: "저장소",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaCreated": {
    message: "생성일",
    description: "Blueprint detail metadata label",
  },
  "blueprints.metaUpdated": {
    message: "마지막 동기화",
    description: "Blueprint detail metadata label",
  },
  "blueprints.detailsTitle": {
    message: "세부 정보",
    description:
      "Blueprint Settings page: title of bex's card of read-only facts (repository, created, last synced) under Render's Settings card (w3/m679)",
  },
  "blueprints.settingsNameHint": {
    message: "Blueprint의 고유한 이름입니다.",
    description:
      "Blueprint Settings card: help under Blueprint Name (Render's copy, w3/m679)",
  },
  "blueprints.settingsRequired": {
    message: "필수",
    description:
      "Blueprint Settings card: shown under a row left empty (Render's copy, w3/m679)",
  },
  "blueprints.settingsNameTaken": {
    message: "이미 사용 중인 이름입니다",
    description:
      "Blueprint Settings card: shown under Blueprint Name when another Blueprint of the workspace has the name (Render's copy, w3/m679)",
  },
  "blueprints.settingsAutoSync": {
    message: "자동 동기화",
    description:
      "Blueprint Settings card: the Auto Sync row's label (Render's copy, w3/m679)",
  },
  "blueprints.settingsAutoSyncHint": {
    message:
      'Blueprint 파일의 변경 사항을 자동으로 동기화할까요? 동기화를 수동으로 처리하려면 "아니요"를 선택하세요.',
    description:
      "Blueprint Settings card: help under Auto Sync; quotes the No option (Render's copy, w3/m679)",
  },
  "blueprints.settingsYes": {
    message: "예",
    description:
      "Blueprint Settings card: Auto Sync option, on (Render's copy, w3/m679)",
  },
  "blueprints.settingsNo": {
    message: "아니요",
    description:
      "Blueprint Settings card: Auto Sync option, off (Render's copy, w3/m679)",
  },
  "blueprints.settingsBranch": {
    message: "브랜치",
    description: "Blueprint Settings card: the Branch row's label (w3/m679)",
  },
  "blueprints.settingsBranchHint": {
    message: "빌드하고 배포할 Git 브랜치입니다.",
    description:
      "Blueprint Settings card: help under Branch (Render's copy, w3/m679)",
  },
  "blueprints.settingsBranchNotFound": {
    message:
      "이 저장소에서 브랜치를 찾을 수 없습니다. 다른 브랜치를 선택하세요.",
    description:
      "Blueprint Settings card: shown under Branch when the repository has no such branch (Render's copy, w3/m679)",
  },
  "blueprints.settingsPath": {
    message: "Blueprint 경로",
    description:
      "Blueprint Settings card: the Blueprint Path row's label (Render's copy, w3/m679)",
  },
  "blueprints.settingsPathHint": {
    message:
      "저장소 내 Blueprint 파일의 경로입니다(예: infra/render.yaml). 기본값은 저장소 루트의 render.yaml입니다.",
    description:
      "Blueprint Settings card: help under Blueprint Path (Render's copy, w3/m679)",
  },
  "blueprints.settingsFileNotFound": {
    message:
      '"{branch}" 브랜치에서 Blueprint 파일 "{path}"을(를) 찾을 수 없습니다. 이 브랜치에 파일이 있는지 확인하세요.',
    description:
      "Blueprint Settings card: shown under Branch or Blueprint Path when the Blueprint file is not at that path on that branch (Render's copy, w3/m679)",
  },
  "blueprints.settingsUpdateFailed": {
    message: "Blueprint를 업데이트하지 못했습니다. 다시 시도하세요.",
    description:
      "Blueprint Settings card: shown under a row whose save failed with no reason from the server (Render's copy, w3/m679)",
  },
  "blueprints.disconnectNoSync": {
    message:
      "리소스가 더 이상 Blueprint 파일에서 자동으로 동기화되지 않습니다.",
    description:
      "Disconnect Blueprint modal: first sentence (Render's copy, w3/m679)",
  },
  "blueprints.disconnectKeeps": {
    message:
      "파일 자체는 삭제되지 않습니다. 관리 중인 리소스도 삭제되지 않습니다. 나중에 언제든지 Blueprints 페이지에서 파일을 다시 연결할 수 있습니다.",
    description:
      "Disconnect Blueprint modal: second sentence (Render's copy, w3/m679)",
  },
  "blueprints.disconnectConfirm": {
    message: "Blueprint 사양 {name}의 연결을 해제하시겠습니까?",
    description:
      "Disconnect Blueprint modal: the question, with the Blueprint's name in bold (Render's copy, w3/m679)",
  },
  "blueprints.resourcesTitle": {
    message: "관리 중인 리소스",
    description:
      "Blueprint detail section heading listing services/databases managed by this blueprint",
  },
  "blueprints.resourcesEmpty": {
    message:
      "아직 리소스가 없습니다. Blueprint를 동기화하여 render.yaml을 적용하세요.",
    description: "Blueprint managed-resources empty state (never synced)",
  },
  "blueprints.resourcesEmptyAwaitingApproval": {
    message:
      "아직 리소스가 없습니다. 계획된 동기화를 승인하면 render.yaml의 리소스가 생성됩니다.",
    description:
      "Blueprint managed-resources empty state while the Blueprint awaits approval of its planned sync",
  },
  "blueprints.resourcesEmptyAfterError": {
    message:
      "확보된 리소스가 없습니다. 마지막 동기화가 실패했습니다. 동기화 기록에서 오류를 확인하고 Blueprint를 수정한 후 다시 시도하세요.",
    description:
      "Blueprint managed-resources empty state when status is error after a sync",
  },
  "blueprints.syncHistoryEmpty": {
    message: "아직 동기화 기록이 없습니다.",
    description: "Syncs page: no syncs yet, Render's wording (w3/m678)",
  },
  "blueprints.detachPreviewTitle": {
    message: "리소스가 더 이상 관리되지 않습니다",
    description: "Blueprint resource detachment notice.",
  },
  "blueprints.detachEstimateNote": {
    message: "이 리소스의 비용은 아래 추정치에 포함되지 않습니다.",
    description:
      "Detached resources are outside the proposed manifest estimate.",
  },
  "blueprints.detachWarning": {
    message:
      "이 리소스는 계속 실행되며 요금이 계속 발생할 수 있습니다. 이 Blueprint는 더 이상 이 리소스를 관리하지 않습니다.",
    description: "Blueprint resource detachment notice.",
  },
  "blueprints.manifestTitle": {
    message: "render.yaml 매니페스트",
    description: "Blueprint detail section heading for the stored manifest",
  },
  "blueprints.plannedActions": {
    message: "계획된 변경 사항",
    description:
      "A sync's page: accessible label of its list of planned actions (w3/m678)",
  },
  "blueprints.plannedAction": {
    message: "{operation} {kind} {name}",
    description:
      "Planned-sync card: one plan action, e.g. 'create service api'",
  },
  "blueprints.plannedManualEnvVars": {
    message:
      "이 값은 Blueprint 파일에 없습니다(sync: false). 승인하려면 입력하세요.",
    description:
      "A waiting sync's page: intro to the fields for the sync: false values approving it needs (w3/m678)",
  },
  "blueprints.plannedManualEnvVarLabel": {
    message: "{service}: {key}",
    description:
      "A waiting sync's page: label of one sync: false value's field (w3/m678)",
  },
  "blueprints.plannedApprove": {
    message: "승인",
    description:
      "A waiting sync's page: the button that applies the planned sync, Render's Approve (w3/m678)",
  },
  "blueprints.plannedChanged": {
    message:
      "다른 사용자가 이 Blueprint를 다시 계획했습니다. 최신 계획을 검토한 후 승인하세요.",
    description:
      "A sync's page: inline alert when the plan on screen was replaced before it was approved (BLUEPRINT_PLAN_CHANGED)",
  },
  "blueprints.plannedReplanFailed": {
    message: "새 동기화를 계획하지 못했습니다",
    description:
      "Sync Error banner: Manual sync could not plan a sync and the server gave no reason (w3/m678)",
  },
  "blueprints.syncButton": {
    message: "수동 동기화",
    description:
      "The Blueprint header's one action, Render's Manual sync: plans a sync and opens its page (w3/m678)",
  },
  "blueprints.editField": {
    message: "{field} 편집",
    description: "Aria label for the inline-edit pencil button",
  },
  "blueprints.pathInvalid": {
    message:
      "저장소 기준의 깔끔한 .yaml 또는 .yml 상대 경로여야 합니다(앞의 슬래시 및 '..' 불가).",
    description: "Inline path editor client-side validation message",
  },
  "blueprints.updateSuccess": {
    message: "Blueprint를 업데이트했습니다",
    description: "Toast shown after a successful updateBlueprint call",
  },
  "blueprints.disconnectButton": {
    message: "Blueprint 연결 해제",
    description:
      "Blueprint Settings page: the critical button that opens the Disconnect Blueprint modal (Render's copy, w3/m679)",
  },
  "blueprints.disconnectTitle": {
    message: "Blueprint 연결 해제",
    description: "Disconnect Blueprint modal title (Render's copy, w3/m679)",
  },
  "blueprints.disconnectAction": {
    message: "Blueprint 연결 해제",
    description:
      "Disconnect Blueprint modal: the critical confirm button (Render's copy, w3/m679)",
  },
  "blueprints.disconnectCancel": {
    message: "취소",
    description: "Disconnect confirm dialog cancel button label",
  },
  "blueprints.disconnectSuccess": {
    message: "Blueprint 연결을 해제했습니다",
    description: "Toast shown after a successful disconnectBlueprint call",
  },
  "blueprints.disconnectError": {
    message: "연결 해제에 실패했습니다",
    description: "Toast shown when disconnectBlueprint returns an error",
  },
  "blueprints.disconnectBusy": {
    message:
      "동기화를 적용하는 중입니다. 완료된 후 연결 해제를 다시 시도하세요",
    description:
      "Toast shown when disconnectBlueprint refuses with BLUEPRINT_SYNC_BUSY",
  },
  "blueprints.validateTitle": {
    message: "검증",
    description: "Validate panel section heading",
  },
  "blueprints.validateRun": {
    message: "검증 실행",
    description: "Validate panel button label",
  },
  "blueprints.validateValid": {
    message: "매니페스트가 올바릅니다. 오류가 없습니다.",
    description: "Validate result: manifest parsed successfully",
  },
  "blueprints.validateInvalid": {
    message: "매니페스트에 오류가 있습니다:",
    description:
      "Validate result: manifest has parse errors — followed by the error list",
  },
  "blueprints.validateNoResult": {
    message: "아직 결과가 없습니다.",
    description: "Validate panel placeholder before first run",
  },
  "blueprints.stateCreated": {
    message: "승인 대기 중",
    description:
      "Sync state (and Blueprint status) created: a planned sync waiting for approval (w2/m256); never a queued run.",
  },
  "blueprints.stateRunning": {
    message: "실행 중",
    description: "Sync-run state: the run is in progress.",
  },
  "blueprints.stateSuccess": {
    message: "성공",
    description: "Sync-run state: the run completed successfully.",
  },
  "blueprints.createNameDescription": {
    message: "이 Blueprint 인스턴스의 고유한 이름입니다.",
    description: "Create-Blueprint Name row description",
  },
  "blueprints.createBlockedSource": {
    message: "계속하려면 저장소를 선택하세요.",
    description:
      "Create-Blueprint action bar: why Deploy is held while no repository is chosen",
  },
  "blueprints.createBlockedBranch": {
    message: "계속하려면 브랜치를 입력하세요.",
    description:
      "Create-Blueprint action bar: why Deploy is held while the branch is empty",
  },
  "blueprints.createBlockedReviewing": {
    message: "Blueprint 파일을 검토하는 중…",
    description:
      "Create-Blueprint action bar: why Deploy is held while the Blueprint file is being reviewed",
  },
  "blueprints.createBlockedReview": {
    message: "계속하려면 검토에서 발견된 문제를 해결하세요.",
    description:
      "Create-Blueprint action bar: why Deploy is held while the review found problems in the Blueprint file",
  },
  "blueprints.navResources": {
    message: "리소스",
    description:
      "Blueprint sidebar entry and page heading for the resources the Blueprint manages (Render's Resources, /blueprints/<id>/resources, w3/m677)",
  },
  "blueprints.navSyncs": {
    message: "동기화 기록",
    description:
      "Blueprint sidebar entry and page heading for the Blueprint's syncs (Render's Syncs, /blueprints/<id>/syncs, w3/m677)",
  },
  "blueprints.navSettings": {
    message: "설정",
    description:
      "Blueprint sidebar entry and page heading for the Blueprint's settings (Render's Settings, /blueprints/<id>/settings, w3/m677)",
  },
  "blueprints.colType": {
    message: "유형",
    description:
      "Blueprint Resources table column header for a managed resource's type",
  },
  "blueprints.resourcesViewSyncs": {
    message: "동기화 기록으로 이동",
    description:
      "Link from a Blueprint's empty Resources page to its Syncs page, where the waiting plan or the failed sync is (w3/m677)",
  },
  "blueprints.syncViewDetails": {
    message: "세부 정보 보기",
    description:
      "Sync card: link to the sync's own page (Render's View details, w3/m678)",
  },
  "blueprints.syncsLoadMore": {
    message: "더 보기",
    description:
      "Syncs page: button reading the next page of older syncs (w3/m678)",
  },
  "blueprints.syncsLoadMoreError": {
    message: "이전 동기화를 불러오지 못했습니다.",
    description:
      "Syncs page: toast when reading an older page of syncs fails (w3/m678)",
  },
  "blueprints.syncPageTitle": {
    message: "동기화: {sha}",
    description:
      "A sync's page heading, Render's 'Sync: <short commit id>'; {sha} is the 7-character commit id (w3/m678)",
  },
  "blueprints.syncPageTitleNoCommit": {
    message: "동기화",
    description:
      "A sync's page heading when the sync read no commit, Render's 'Sync' (w3/m678)",
  },
  "blueprints.syncDocumentTitle": {
    message: "동기화",
    description:
      "Document title's page name on a sync's page: 'Sync ・ <name> ・ Blueprint' (w3/m678)",
  },
  "blueprints.syncUpToDate": {
    message: "리소스가 이미 최신 상태입니다",
    description:
      "A sync's page when its plan has no actions, Render's wording (w3/m678)",
  },
  "blueprints.syncNotFullySynced": {
    message: "이 리소스를 완전히 동기화하지 못했습니다.",
    description:
      "A failed sync's page with no actions and no errors, Render's critical line (w3/m678)",
  },
  "blueprints.syncNotFullySyncedBody": {
    message: "이 오류가 계속 표시되면 지원팀에 문의하세요.",
    description:
      "Under blueprints.syncNotFullySynced: what to do if it keeps happening (w3/m678)",
  },
  "blueprints.syncActionsNotRecorded": {
    message:
      "이 동기화는 동기화가 작업을 저장하기 전에 기록되어 작업을 표시할 수 없습니다.",
    description:
      "A sync's page for a sync recorded before every sync kept its actions (w3/m678)",
  },
  "blueprints.syncActionCreate": {
    message: "{kind} {name} 생성",
    description:
      "A sync's planned action: create a resource; {kind} is its kind label, {name} its name, drawn bold (w3/m678)",
  },
  "blueprints.syncActionUpdate": {
    message: "{kind} {name} 업데이트",
    description:
      "A sync's planned action: change a resource; {kind} is its kind label, {name} its name (w3/m678)",
  },
  "blueprints.syncActionDetach": {
    message: "{kind} {name} 관리 중지",
    description:
      "A sync's planned action: stop managing a resource the file no longer names; {kind}, {name} (w3/m678)",
  },
  "blueprints.syncActionError": {
    message: "{kind} {name} 적용 불가",
    description:
      "A sync's planned action the planner refused; {kind}, {name}; the reason follows (w3/m678)",
  },
  "blueprints.syncActionDetachNote": {
    message: "계속 실행되며 요금이 발생할 수 있습니다.",
    description:
      "Under a 'stop managing' action: the resource is not deleted (w3/m678)",
  },
  "blueprints.syncActionStateUnknown": {
    message: "상태 알 수 없음",
    description:
      "Accessible name of a planned action's state icon when bex can't say which state it reached (a failed sync's actions, w3/m678)",
  },
  "blueprints.syncApproveFailed": {
    message: "이 동기화를 승인하지 못했습니다. 다시 시도하세요.",
    description:
      "A sync's page: inline alert when Approve fails without the server's reason, Render's wording (w3/m678)",
  },
  "blueprints.syncNotFound": {
    message:
      "이 동기화가 없습니다. 24시간 이내에 승인되지 않은 계획된 동기화는 삭제됩니다.",
    description:
      "A sync's page for an id that is not one of this Blueprint's syncs, e.g. a plan deleted after 24 hours (w3/m678)",
  },
  "blueprints.syncErrorBannerTitle": {
    message: "동기화 오류",
    description:
      "Title of Render's dismissible banner when Manual sync could not plan a sync (w3/m678)",
  },
  "blueprints.syncErrorBannerDismiss": {
    message: "닫기",
    description:
      "Accessible name of the Sync Error banner's close button (w3/m678)",
  },
  "blueprints.syncErrorsIntro_other": {
    message: "Blueprint와 동기화하는 중에 문제가 발생했습니다:",
    description:
      "A sync's page: the line above its errors, Render's 'There was a problem' / 'There were a few problems' (w3/m678)",
  },
};

export default koBlueprints;
