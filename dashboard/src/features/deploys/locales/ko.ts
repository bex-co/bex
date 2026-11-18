import type { TranslationEntry } from "@/i18n";

const koDeploys: Record<string, TranslationEntry> = {
  "deploys.statusCreated": {
    message: "생성됨",
    description: "Deploy status: created",
  },
  "deploys.statusQueued": {
    message: "대기 중",
    description: "Deploy status: queued",
  },
  "deploys.statusBuildInProgress": {
    message: "빌드 중",
    description: "Deploy status: build_in_progress",
  },
  "deploys.statusBuildFailed": {
    message: "빌드 실패",
    description: "Deploy status: build_failed",
  },
  "deploys.statusPreDeployInProgress": {
    message: "사전 배포 진행 중",
    description: "Deploy status: pre_deploy_in_progress",
  },
  "deploys.statusPreDeployFailed": {
    message: "사전 배포 실패",
    description: "Deploy status: pre_deploy_failed",
  },
  "deploys.statusUpdateInProgress": {
    message: "진행 중",
    description: "Deploy status: update_in_progress",
  },
  "deploys.statusUpdateFailed": {
    message: "실패",
    description: "Deploy status: update_failed",
  },
  "deploys.statusLive": {
    message: "라이브",
    description: "Deploy status: live",
  },
  "deploys.statusCanceled": {
    message: "취소됨",
    description: "Deploy status: canceled",
  },
  "deploys.statusDeactivated": {
    message: "비활성화됨",
    description: "Deploy status: deactivated",
  },
  "deploys.statusUnknown": {
    message: "알 수 없음",
    description: "Deploy status: unrecognized backend value",
  },
  "deploys.statusLabel": {
    message: "배포 상태",
    description:
      "Deploy detail header: explicit label distinguishing deploy status from service phase",
  },
  "deploys.imageDigestCopy": {
    message: "이미지 다이제스트 복사",
    description:
      "Deploy header: accessible label of the button copying the image digest the deploy ran",
  },
  "deploys.imageDigestCopied": {
    message: "이미지 다이제스트를 복사했습니다",
    description: "Deploy header: toast after copying the image digest",
  },
  "deploys.imageDigestCopyError": {
    message: "클립보드에 복사하지 못했습니다",
    description: "Deploy header: toast when copying the image digest fails",
  },
  "deploys.created": {
    message: "생성",
    description: "Deploy header: created-at label",
  },
  "deploys.updated": {
    message: "업데이트",
    description: "Label for the deploy's latest stored transition time",
  },
  "deploys.started": {
    message: "시작",
    description: "Deploy header: started-at label",
  },
  "deploys.finished": {
    message: "완료",
    description: "Deploy header: finished-at label",
  },
  "deploys.duration": {
    message: "소요 시간",
    description: "Deploy header: elapsed build/deploy time label",
  },
  "deploys.durationValue": {
    message: "소요 시간 {duration}",
    description: "Deploy row: elapsed build/deploy time",
  },
  "deploys.durationInProgress": {
    message: "소요 시간 측정 중",
    description: "Deploy row: elapsed time is still running",
  },
  "deploys.durationActive": {
    message: "진행 중",
    description:
      "Deploys tab Duration column: bare running-elapsed marker (the column header supplies the label)",
  },
  "deploys.deployedAt": {
    message: "{timestamp} 배포됨",
    description:
      'Deploy row: how long ago a live/deactivated deploy went live — {timestamp} is elapsed text like "2 hours ago" (its finish time; createdAt fallback)',
  },
  "deploys.createdAt": {
    message: "{timestamp} 생성됨",
    description:
      'Deploy row: how long ago a deploy that hasn\'t finished (created/queued/in-progress) was created — {timestamp} is elapsed text like "2 minutes ago"',
  },
  "deploys.canceledAt": {
    message: "{timestamp} 취소됨",
    description:
      'Deploy row: how long ago a canceled deploy was canceled — {timestamp} is elapsed text like "3 days ago" (its finish time; createdAt fallback)',
  },
  "deploys.failedAt": {
    message: "{timestamp} 실패함",
    description:
      'Deploy row: how long ago a build/pre-deploy/update failure landed — {timestamp} is elapsed text like "3 days ago" (its finish time; createdAt fallback)',
  },
  "deploys.notYet": {
    message: "—",
    description:
      "Deploy header: placeholder for a timestamp that hasn't happened yet",
  },
  "deploys.triggerCreate": {
    message: "최초 배포",
    description: "Deploy header: trigger=create label",
  },
  "deploys.triggerApi": {
    message: "API 배포",
    description: "Deploy header: trigger=api label (an API-key deploy, w4/413)",
  },
  "deploys.triggerManual": {
    message: "수동 배포",
    description:
      "Deploy header: trigger=manual label (a dashboard click, w4/413)",
  },
  "deploys.triggerRollback": {
    message: "{deployId}(으)로 롤백",
    description:
      "Deploy header: trigger label for a rollback deploy, naming the restored deploy (id must stay literal — never wrap in CSS capitalize)",
  },
  "deploys.triggerDeployHook": {
    message: "Deploy Hook",
    description: "Deploy row: trigger=deploy_hook label (secret-URL trigger)",
  },
  "deploys.triggerBlueprint": {
    message: "Blueprint 동기화",
    description: "Deploy row: trigger=blueprint label (Blueprint sync deploy)",
  },
  "deploys.triggerNewCommit": {
    message: "새 커밋",
    description: "Deploy row: trigger=new_commit label (git push redeploy)",
  },
  "deploys.triggerConfigChange": {
    message: "구성 변경",
    description:
      "Deploy row: trigger=config_change label (an env var, secret file, or env-group edit that deployed)",
  },
  "deploys.triggerSettingsChange": {
    message: "설정 변경",
    description:
      "Deploy row: trigger=settings_change label (a Settings save that deployed, or a plan change; w8/m135)",
  },
  "deploys.triggerRestart": {
    message: "다시 시작",
    description:
      "Deploy row: trigger=restart label (a Restart roll of the live release, w4/379)",
  },
  "deploys.triggerDeployedByRender": {
    message: "플랫폼 재빌드",
    description:
      "Deploy row: trigger=deployed_by_render label (the platform rebuilt the live commit, e.g. to ship a runtime security patch)",
  },
  "deploys.logSearchPlaceholder": {
    message: "로그 검색…",
    description: "Deploy detail page: log search input placeholder",
  },
  "deploys.logTypeFilter": {
    message: "로그 유형",
    description:
      "Deploy log viewer: aria-label of the log-type filter dropdown (Render's All/Application/Build selector)",
  },
  "deploys.logTypeAll": {
    message: "모든 로그",
    description: "Deploy log viewer type filter: show every log type",
  },
  "deploys.logTypeApp": {
    message: "애플리케이션 로그",
    description:
      "Deploy log viewer type filter: app + pre-deploy lines, matching Render's Application bucket",
  },
  "deploys.logTypeBuild": {
    message: "빌드 로그",
    description: "Deploy log viewer type filter: build lines only",
  },
  "deploys.buildLogsStoreUnavailable": {
    message: "이전 빌드 로그를 사용할 수 없습니다",
    description:
      "Deploy detail page: title shown when the durable log store isn't wired",
  },
  "deploys.buildLogsStoreUnavailableBody": {
    message:
      "이 대시보드는 영구 로그 스토리지에 연결되어 있지 않습니다. 이 배포의 애플리케이션 로그는 계속 표시될 수 있습니다.",
    description:
      "Deploy detail page: accurate explanation of a storeless build-log response",
  },
  "deploys.detailRefreshError": {
    message:
      "이 배포를 새로 고치지 못했습니다. 마지막으로 불러온 세부 정보를 표시합니다.",
    description:
      "Deploy detail page: a background refresh of an already loaded deploy failed; the page keeps the last good details and offers a retry",
  },
  "deploys.logPartiallyLoaded": {
    message:
      "이 시간 범위의 일부만 불러왔습니다. 이전 줄을 검색하려면 이전 로그를 더 불러오세요.",
    description:
      "Deploy log viewer banner while older history remains unloaded. The panel's search and type/instance filters run over loaded lines only, so it must not claim a number of matches",
  },
  "deploys.logStepEndMissing": {
    message:
      "이 배포는 사전 배포 단계가 끝나기 전에 취소되었고 해당 단계의 완료 시간이 기록되지 않아, 이후 출력이 이 범위 밖에 있을 수 있습니다. 로그 옵션에서 더 넓은 범위를 선택하세요.",
    description:
      "Deploy log viewer: a canceled deploy's continued pre-deploy step has no recorded end, so the deploy window may omit its later output",
  },
  "deploys.logStepEndUnavailable": {
    message:
      "이 배포의 사전 배포 단계가 끝난 시점을 확인하지 못해, 이후 단계의 출력이 이 범위 밖에 있을 수 있습니다. 다시 시도하거나 로그 옵션에서 더 넓은 범위를 선택하세요.",
    description:
      "Deploy log viewer: reading a canceled deploy's continued pre-deploy step end failed; offers a retry",
  },
  "deploys.timelineTitle": {
    message: "상태 타임라인",
    description: "Deploy detail page: status-timeline card title",
  },
  "deploys.timelineCreated": {
    message: "배포 생성됨",
    description: "Deploy timeline: deploy row was created",
  },
  "deploys.timelineStarted": {
    message: "배포 시작됨",
    description: "Deploy timeline: backend-provided startedAt timestamp",
  },
  "deploys.timelineInProgress": {
    message: "배포 진행 중",
    description: "Deploy timeline: current non-terminal deploy status",
  },
  "deploys.timelineLive": {
    message: "배포 라이브",
    description: "Deploy timeline: successful terminal state",
  },
  "deploys.timelineFailed": {
    message: "배포 실패",
    description: "Deploy timeline: failed terminal state",
  },
  "deploys.timelineCanceled": {
    message: "배포 취소됨",
    description: "Deploy timeline: canceled terminal state",
  },
  "deploys.timelineDeactivated": {
    message: "배포 비활성화됨",
    description: "Deploy timeline: deactivated terminal state",
  },
  "deploys.timelineEventsUnavailable": {
    message: "서비스 이벤트를 사용할 수 없어 배포 시간만 표시합니다.",
    description:
      "Deploy timeline: graceful fallback when service-events query fails",
  },
  "deploys.autoDeployOffTitle": {
    message: "자동 배포가 꺼져 있습니다",
    description:
      "Deploys page banner title when the service's auto-deploy is off (Render, w3/064)",
  },
  "deploys.autoDeployOffBody": {
    message:
      "{branch} 브랜치에 푸시해도 이 서비스가 배포되지 않습니다. 수동으로 배포하거나 자동 배포를 다시 켜세요.",
    description: "Deploys page banner body; {branch} is the tracked git branch",
  },
  "deploys.autoDeployOffBodyNoBranch": {
    message:
      "푸시해도 이 서비스가 배포되지 않습니다. 수동으로 배포하거나 자동 배포를 다시 켜세요.",
    description: "Deploys page banner body when the branch is unknown",
  },
  "deploys.autoDeployOffSettings": {
    message: "자동 배포 설정",
    description: "Deploys page banner link to the service Settings page",
  },
  "deploys.deploysManualTitle": {
    message: "배포는 수동입니다",
    description:
      "Deploys page banner title for a repo whose pushes can't reach bex (pushDeliveryMethod unavailable)",
  },
  "deploys.deploysManualBody": {
    message:
      "bex가 이 저장소의 푸시를 받지 못하므로 푸시해도 이 서비스가 배포되지 않습니다. 수동으로 또는 Deploy Hook으로 배포하거나, GitHub 앱을 통해 저장소를 연결하세요.",
    description:
      "Deploys page banner body for a repo whose pushes can't reach bex: no Auto-Deploy setting can turn deploys on push on (Render: a public Git URL must be deployed manually)",
  },
  "deploys.listTitle": {
    message: "배포",
    description: "Deploys tab: card title over the deploy-history list",
  },
  "deploys.buildsTitle": {
    message: "빌드",
    description:
      "A cron job's Builds tab: card title over its build history, Render's Builds page (w2/m360)",
  },
  "deploys.columnDeploy": {
    message: "배포",
    description:
      "Deploys tab table: first column header (status, id, commit, timestamp)",
  },
  "deploys.columnTrigger": {
    message: "트리거",
    description: "Deploys tab table: column header for what triggered a deploy",
  },
  "deploys.columnDuration": {
    message: "소요 시간",
    description:
      "Deploys tab table: column header for elapsed build/deploy time",
  },
  "deploys.columnActions": {
    message: "작업",
    description:
      "Deploys tab table: accessible-only header over the Cancel/Rollback action column",
  },
  "deploys.listEmpty": {
    message: "아직 배포가 없습니다.",
    description: "Deploys tab: empty state with no status filter",
  },
  "deploys.listEmptyFiltered": {
    message: "선택한 상태와 일치하는 배포가 없습니다.",
    description: "Deploys tab: empty state while a status filter is active",
  },
  "deploys.listStatusFilterLabel": {
    message: "상태별 필터",
    description: "Deploys tab: aria-label of the status filter dropdown",
  },
  "deploys.listStatusAll": {
    message: "모든 상태",
    description: "Deploys tab: status filter option matching every deploy",
  },
  "deploys.listSearchPlaceholder": {
    message: "검색…",
    description:
      "Deploys tab: placeholder for client-side deploy-history search",
  },
  "deploys.listSearchLabel": {
    message: "불러온 배포와 커밋 검색",
    description: "Deploys tab: accessible label for deploy-history search",
  },
  "deploys.listClearSearch": {
    message: "검색 지우기",
    description:
      "Deploys tab: button under the no-match message that empties the search box",
  },
  "deploys.listMatchCount": {
    message: "불러온 {total}개 중 {matched}개 일치",
    description:
      "Deploys tab: count shown while a search is typed; matched = loaded deploys that match, total = loaded deploys",
  },
  "deploys.listLoadErrorTitle": {
    message: "배포를 불러오지 못했습니다",
    description:
      "Deploys tab: heading of the error state when the deploy history could not be read; a Try again button follows",
  },
  "deploys.listLoadErrorBody": {
    message: "배포에는 영향이 없습니다. 연결을 확인한 뒤 다시 시도하세요.",
    description:
      "Deploys tab: body of the error state when the deploy history could not be read and the server gave no message; a Try again button follows",
  },
  "deploys.listEmptySearch": {
    message: "검색과 일치하는 불러온 배포가 없습니다.",
    description:
      "Deploys tab: empty state when client-side search has no match",
  },
  "deploys.listCount_other": {
    message: "배포 {count}개",
    description: "Deploys tab: complete deploy count (plural)",
  },
  "deploys.listCountLoaded_other": {
    message: "배포 {count}개 불러옴",
    description:
      "Deploys tab: loaded count when more server pages remain (plural)",
  },
  "deploys.listLoadMore": {
    message: "더 보기",
    description: "Deploys tab: button fetching the next page of deploy history",
  },
  "deploys.listLoadMoreError": {
    message: "이전 배포를 불러오지 못했습니다.",
    description:
      "Deploys tab: toast when reading an older page of deploy history fails",
  },
  "deploys.logOptions": {
    message: "로그 옵션",
    description:
      "Deploy log viewer: aria-label/tooltip of the options menu button",
  },
  "deploys.logRangeLabel": {
    message: "시간 범위",
    description:
      "Deploy log viewer options menu: heading over the time-range choices",
  },
  "deploys.logRangeDeploy": {
    message: "배포 기간",
    description:
      "Deploy log viewer: the default time range — the deploy's own createdAt..finishedAt window",
  },
  "deploys.logRangeLast15m": {
    message: "최근 15분",
    description: "Deploy log viewer: relative time-range option (?r=15m)",
  },
  "deploys.logRangeLast1h": {
    message: "최근 1시간",
    description: "Deploy log viewer: relative time-range option (?r=1h)",
  },
  "deploys.logRangeLast6h": {
    message: "최근 6시간",
    description: "Deploy log viewer: relative time-range option (?r=6h)",
  },
  "deploys.logRangeLast24h": {
    message: "최근 24시간",
    description: "Deploy log viewer: relative time-range option (?r=24h)",
  },
  "deploys.logRangeLast7d": {
    message: "최근 7일",
    description: "Deploy log viewer: relative time-range option (?r=7d)",
  },
  "deploys.logWrap": {
    message: "줄 바꿈",
    description:
      "Deploy log viewer options menu: toggle wrapping long log lines vs horizontal scroll",
  },
  "deploys.logTimestamps": {
    message: "타임스탬프 표시",
    description:
      "Deploy log viewer options menu: toggle the per-line timestamp column",
  },
  "deploys.logMaximize": {
    message: "최대화",
    description:
      "Deploy log viewer: button expanding the viewer to fill the screen",
  },
  "deploys.logMinimize": {
    message: "전체 화면 종료",
    description:
      "Deploy log viewer: button restoring the maximized viewer to its inline size",
  },
  "deploys.reason.http_port_waiting": {
    message: "컨테이너가 시작되기를 기다린 후 HTTP 포트를 감지합니다",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_waiting, w4/566)",
  },
  "deploys.reason.http_port_detecting": {
    message: "컨테이너의 HTTP 포트를 감지하는 중입니다",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_detecting, w4/566)",
  },
  "deploys.reason.http_port_limit": {
    message:
      "감지된 HTTP 포트가 서비스의 75개 포트 한도를 초과합니다. PORT를 명시적으로 설정하세요",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_limit, w4/566)",
  },
  "deploys.reason.http_port_detected": {
    message: "{port} 포트에서 HTTP를 감지하여 해당 포트로 서비스합니다",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_detected, w4/566); params are the sentence's values",
  },
  "deploys.reason.http_port_not_answering": {
    message:
      "{port} 포트가 연결은 수락했지만 아직 HTTP에 응답하지 않습니다. 컨테이너가 시작되는 동안 계속 검색합니다",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_not_answering, w4/566); params are the sentence's values",
  },
  "deploys.reason.http_port_none_yet": {
    message:
      "아직 HTTP에 응답하는 포트가 없습니다. 컨테이너가 시작되는 동안 계속 검색합니다",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_none_yet, w4/566)",
  },
  "deploys.reason.http_port_fallback": {
    message:
      "HTTP에 응답하는 포트가 없어 대체 포트 {port}을(를) 사용합니다. HTTP 리스너가 다른 포트에 바인딩되어 있다면 PORT를 해당 포트로 설정하세요",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason http_port_fallback, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_not_found": {
    message:
      "레지스트리에서 {image} 이미지를 찾을 수 없습니다. 저장소 또는 태그가 존재하지 않습니다. 이미지 URL과 태그를 확인하거나, 존재하는 다이제스트로 배포하세요",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_not_found, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_credential_refused": {
    message:
      "{host}이(가) {image} 이미지에 대한 이 서비스의 레지스트리 자격 증명을 거부했습니다(HTTP {status}). 자격 증명의 사용자 이름과 토큰을 확인하거나, 공개 이미지를 익명으로 가져오려면 이 서비스에서 자격 증명을 제거하세요",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_credential_refused, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_access_refused": {
    message:
      "레지스트리가 {image} 이미지에 대한 액세스를 거부했습니다. 비공개 이미지라면 이 서비스에 {host}의 레지스트리 자격 증명을 추가하세요",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_access_refused, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_read_refused": {
    message:
      "레지스트리가 {image} 이미지 읽기를 거부했습니다(HTTP {status}). 이미지 URL과 태그를 확인하세요",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_read_refused, w4/566); params are the sentence's values",
  },
  "deploys.reason.image_registry_unreachable": {
    message:
      "레지스트리 {host}가 {minutes}분 동안 {image} 이미지 읽기에 응답하지 않았습니다. 레지스트리에 연결할 수 있는지 확인한 후 다시 배포하세요",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason image_registry_unreachable, w4/692); params are the sentence's values",
  },
  "deploys.reason.build_timed_out": {
    message: "빌드가 허용 시간 내에 끝나지 않았습니다. 빌드 로그를 확인하세요",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason build_timed_out, w4/566)",
  },
  "deploys.reason.pre_deploy_timed_out": {
    message:
      "사전 배포 명령이 허용 시간 내에 끝나지 않았습니다. 사전 배포 로그를 확인하세요",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_timed_out, w4/566)",
  },
  "deploys.reason.health_timed_out": {
    message:
      "배포가 헬스 체크 허용 시간 내에 정상 상태가 되지 않았습니다. 서비스 로그를 확인하세요",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason health_timed_out, w4/566)",
  },
  "deploys.reason.startup_out_of_memory": {
    message:
      "시작하는 동안 인스턴스의 메모리가 부족했습니다(인스턴스 유형의 {memory} 한도, 종료 코드 {code}) — 메모리 사용량을 줄이거나 더 큰 인스턴스 유형을 선택하세요",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason startup_out_of_memory, w4/630); params are the sentence's values: memory is the instance type's memory limit (e.g. 512Mi), code the exit code",
  },
  "deploys.reason.probe_http_waiting": {
    message:
      "컨테이너는 실행 중이지만 {kind} 헬스 체크가 아직 성공하지 않아 롤아웃이 대기 중입니다. {port} 포트의 GET {path} 요청입니다. 해당 경로가 서비스 자체 포트에서 2xx 또는 3xx 상태를 반환하는지 확인하세요.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_waiting, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_http_waiting_restarted": {
    message:
      "컨테이너는 실행 중이지만 {kind} 헬스 체크가 아직 성공하지 않아 롤아웃이 대기 중입니다. {port} 포트의 GET {path} 요청입니다. 라이브니스 체크가 이미 컨테이너를 {restarts}회 다시 시작했으므로, 단순히 느린 것이 아니라 체크가 실패하고 있습니다. 해당 경로가 서비스 자체 포트에서 2xx 또는 3xx 상태를 반환하는지 확인하세요.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_waiting_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_http_failed": {
    message:
      "컨테이너는 실행 중이지만 {kind} 헬스 체크가 롤아웃 허용 시간 내에 성공하지 않아 배포가 실패했습니다. {port} 포트의 GET {path} 요청입니다. 해당 경로가 서비스 자체 포트에서 2xx 또는 3xx 상태를 반환하는지 확인하세요.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_failed, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_http_failed_restarted": {
    message:
      "컨테이너는 실행 중이지만 {kind} 헬스 체크가 롤아웃 허용 시간 내에 성공하지 않아 배포가 실패했습니다. {port} 포트의 GET {path} 요청입니다. 라이브니스 체크가 이미 컨테이너를 {restarts}회 다시 시작했으므로, 단순히 느린 것이 아니라 체크가 실패하고 있습니다. 해당 경로가 서비스 자체 포트에서 2xx 또는 3xx 상태를 반환하는지 확인하세요.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_http_failed_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_waiting": {
    message:
      "컨테이너는 실행 중이지만 {kind} 헬스 체크가 아직 성공하지 않아 롤아웃이 대기 중입니다. {port} 포트로의 TCP 연결입니다. 헬스 체크 경로가 설정되지 않아 TCP 연결로 확인하며, 이는 프로세스가 해당 포트에서 수신 대기 중인지만 확인합니다.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_waiting, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_waiting_restarted": {
    message:
      "컨테이너는 실행 중이지만 {kind} 헬스 체크가 아직 성공하지 않아 롤아웃이 대기 중입니다. {port} 포트로의 TCP 연결입니다. 라이브니스 체크가 이미 컨테이너를 {restarts}회 다시 시작했으므로, 단순히 느린 것이 아니라 체크가 실패하고 있습니다. 헬스 체크 경로가 설정되지 않아 TCP 연결로 확인하며, 이는 프로세스가 해당 포트에서 수신 대기 중인지만 확인합니다.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_waiting_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_failed": {
    message:
      "컨테이너는 실행 중이지만 {kind} 헬스 체크가 롤아웃 허용 시간 내에 성공하지 않아 배포가 실패했습니다. {port} 포트로의 TCP 연결입니다. 헬스 체크 경로가 설정되지 않아 TCP 연결로 확인하며, 이는 프로세스가 해당 포트에서 수신 대기 중인지만 확인합니다.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_failed, w4/591); params are the sentence's values",
  },
  "deploys.reason.probe_tcp_failed_restarted": {
    message:
      "컨테이너는 실행 중이지만 {kind} 헬스 체크가 롤아웃 허용 시간 내에 성공하지 않아 배포가 실패했습니다. {port} 포트로의 TCP 연결입니다. 라이브니스 체크가 이미 컨테이너를 {restarts}회 다시 시작했으므로, 단순히 느린 것이 아니라 체크가 실패하고 있습니다. 헬스 체크 경로가 설정되지 않아 TCP 연결로 확인하며, 이는 프로세스가 해당 포트에서 수신 대기 중인지만 확인합니다.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_tcp_failed_restarted, w4/591); params are the sentence's values",
  },
  "deploys.reason.rollout_deadline_exceeded": {
    message: "롤아웃이 진행 제한 시간 내에 정상 상태가 되지 않았습니다",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason rollout_deadline_exceeded, w4/591)",
  },
  "deploys.reason.pre_deploy_window_exceeded": {
    message:
      "사전 배포 명령이 {minutes}분 제한 시간 내에 끝나지 않았습니다. 사전 배포 로그를 확인하세요",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_window_exceeded, w4/591); params are the sentence's values",
  },
  "deploys.reason.pre_deploy_out_of_memory": {
    message:
      "사전 배포 명령의 메모리가 부족했습니다(종료 코드 {code}). 사전 배포 로그를 확인하세요",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_out_of_memory, w4/591); params are the sentence's values",
  },
  "deploys.reason.pre_deploy_exited": {
    message:
      "사전 배포 명령이 종료 코드 {code}(으)로 종료되었습니다. 사전 배포 로그를 확인하세요",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_exited, w4/591); params are the sentence's values",
  },
  "deploys.reason.pre_deploy_failed": {
    message: "사전 배포 명령이 실패했습니다. 사전 배포 로그를 확인하세요",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_failed, w4/591)",
  },
  "deploys.reason.pre_deploy_could_not_launch": {
    message:
      "사전 배포 명령을 시작할 수 없습니다. 컨테이너 런타임이 프로그램을 실행하지 못했으며, 프로그램이 실행되지 않았으므로 사전 배포 로그가 없습니다",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason pre_deploy_could_not_launch, w4/591)",
  },
  "deploys.reason.build_never_started": {
    message: "빌드가 시작되지 않았습니다: {cause}",
    description:
      "A deploy failure reason by its code (bex-api DeployReason build_never_started, w4/591); {cause} is the operator's wait, whose sentences are worded as parts (w5/m356)",
  },
  "deploys.reason.build_abandoned": {
    message:
      "빌드가 허용 시간 내에 끝나지 않았으며, 이 서비스의 App 레코드를 읽을 수 없어 원인을 설명할 수 없습니다",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason build_abandoned, w4/591)",
  },
  "deploys.reason.deploy_abandoned": {
    message:
      "배포가 허용 시간 내에 끝나지 않았으며, 이 서비스의 App 레코드를 읽을 수 없어 원인을 설명할 수 없습니다",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason deploy_abandoned, w4/591)",
  },
  "deploys.reason.platform_error": {
    message:
      "코드나 설정의 문제가 아니라 플랫폼 오류 때문에 bex가 이 릴리스를 적용하지 못했습니다. 참조 번호 {ref}와 함께 지원팀에 문의하세요",
    description:
      "A deploy failure reason by its code (bex-api DeployReason platform_error, w9/090): the platform refused one of bex's own changes; {ref} is the support reference, shown as-is",
  },
  "deploys.reason.canceled_suspended": {
    message: "취소됨: 서비스가 일시 중지되었습니다",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_suspended, w4/591)",
  },
  "deploys.reason.superseded_by_newer_release": {
    message: "더 최신 릴리스로 대체되었습니다",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason superseded_by_newer_release, w4/591)",
  },
  "deploys.reason.superseded_by": {
    message: "{deploy}(으)로 대체되었습니다",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason superseded_by, w4/591); params are the sentence's values",
  },
  "deploys.reason.canceled_by_restart": {
    message:
      "실행 중인 릴리스를 유지한 다시 시작 {deploy}에 의해 취소되었습니다. 이 배포가 적용하려던 내용은 배포되지 않았습니다",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_by_restart, w4/591); params are the sentence's values",
  },
  "deploys.reason.canceled_along_with": {
    message: "{deploy}와(과) 함께 취소되었습니다",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_along_with, w4/591); params are the sentence's values",
  },
  "deploys.reason.canceled_along_with_newer": {
    message: "더 최신 배포와 함께 취소되었습니다",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason canceled_along_with_newer, w4/591)",
  },
  "deploys.reason.restart_service_gone": {
    message: "서비스가 더 이상 존재하지 않습니다",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason restart_service_gone, w4/591)",
  },
  "deploys.reason.restart_wait_timed_out": {
    message: "이 다시 시작이 기다리던 배포가 {wait} 내에 끝나지 않았습니다",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason restart_wait_timed_out, w4/591); params are the sentence's values",
  },
  "deploys.reason.deploy_wait_timed_out": {
    message: "이 배포가 기다리던 배포가 {wait} 안에 끝나지 않았습니다",
    description:
      "A canceled deploy's cancel reason by its code (bex-api DeployReason deploy_wait_timed_out, w4/652): a Rollback or another deploy that waited for the deploy in flight; params are the sentence's values",
  },
  "deploys.reason.probe_other_waiting": {
    message:
      "컨테이너는 실행 중이지만 {kind} 헬스 체크가 아직 성공하지 않아 롤아웃이 대기 중입니다. 구성된 헬스 체크입니다. 해당 경로가 서비스 자체 포트에서 2xx 또는 3xx 상태를 반환하는지 확인하세요.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_waiting, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.probe_other_waiting_restarted": {
    message:
      "컨테이너는 실행 중이지만 {kind} 헬스 체크가 아직 성공하지 않아 롤아웃이 대기 중입니다. 구성된 헬스 체크입니다. 라이브니스 체크가 이미 컨테이너를 {restarts}회 다시 시작했으므로, 단순히 느린 것이 아니라 체크가 실패하고 있습니다. 해당 경로가 서비스 자체 포트에서 2xx 또는 3xx 상태를 반환하는지 확인하세요.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_waiting_restarted, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.probe_other_failed": {
    message:
      "컨테이너는 실행 중이지만 {kind} 헬스 체크가 롤아웃 허용 시간 내에 성공하지 않아 배포가 실패했습니다. 구성된 헬스 체크입니다. 해당 경로가 서비스 자체 포트에서 2xx 또는 3xx 상태를 반환하는지 확인하세요.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_failed, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.probe_other_failed_restarted": {
    message:
      "컨테이너는 실행 중이지만 {kind} 헬스 체크가 롤아웃 허용 시간 내에 성공하지 않아 배포가 실패했습니다. 구성된 헬스 체크입니다. 라이브니스 체크가 이미 컨테이너를 {restarts}회 다시 시작했으므로, 단순히 느린 것이 아니라 체크가 실패하고 있습니다. 해당 경로가 서비스 자체 포트에서 2xx 또는 3xx 상태를 반환하는지 확인하세요.{hint}",
    description:
      "A deploy failure or in-progress reason by its code (bex-api DeployReason probe_other_failed_restarted, w5/m356): the stall of a health check that is neither a GET nor a TCP connect; params are the sentence's values",
  },
  "deploys.reason.hint_image_default_port": {
    message:
      " 이미지가 TCP 포트를 선언하지 않아 기본값으로 사용한 {port} 포트에서 TCP 연결을 수락하는 곳이 없습니다. 서버는 PORT({port})에 지정된 포트의 호스트 0.0.0.0에 바인딩해야 합니다.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_image_default_port, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_image_declared_ports": {
    message:
      " 이미지에 선언된 TCP 포트({ports}) 중 주 포트로 선택된 {port} 포트에서 TCP 연결을 수락하는 곳이 없습니다. 서버를 해당 포트 중 하나의 0.0.0.0에 바인딩하거나, 서비스 포트를 서버가 수신 대기하는 포트로 설정하세요.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_image_declared_ports, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_env_port_only": {
    message:
      " 서비스의 PORT 환경 변수로 설정된 {port} 포트에서 TCP 연결을 수락하는 곳이 없으며, 이미지는 다른 TCP 포트를 선언하지 않습니다. 서버를 {port} 포트의 0.0.0.0에 바인딩하거나, 서비스 포트를 서버가 수신 대기하는 포트로 변경하세요.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_env_port_only, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_configured_port_only": {
    message:
      " 서비스에 구성된 포트로 설정된 {port} 포트에서 TCP 연결을 수락하는 곳이 없으며, 이미지는 다른 TCP 포트를 선언하지 않습니다. 서버를 {port} 포트의 0.0.0.0에 바인딩하거나, 서비스 포트를 서버가 수신 대기하는 포트로 변경하세요.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_configured_port_only, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_env_port_declared": {
    message:
      " 서비스의 PORT 환경 변수로 설정된 {port} 포트에서 TCP 연결을 수락하는 곳이 없으며, 이미지는 TCP 포트({ports})를 선언하고 있습니다. PORT 환경 변수를 서버가 수신 대기하는 포트로 변경하거나, 제거하여 이미지의 포트를 사용하세요.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_env_port_declared, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_configured_port_declared": {
    message:
      " 서비스에 구성된 포트로 설정된 {port} 포트에서 TCP 연결을 수락하는 곳이 없으며, 이미지는 TCP 포트({ports})를 선언하고 있습니다. 설정에서 서비스 포트를 서버가 수신 대기하는 포트로 변경하거나, 제거하여 이미지의 포트를 사용하세요.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_configured_port_declared, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_background_worker": {
    message:
      " 서비스가 어떤 포트로도 트래픽을 받을 필요가 없다면 Background Worker를 대신 만드세요.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_background_worker, w5/m356); en starts with the space that joins it to the sentence before",
  },
  "deploys.reason.hint_no_http_discovered": {
    message: " 시작 시 포트 검색에서 HTTP에 응답하는 포트가 없었습니다.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_http_discovered, w5/m356); en starts with the space that joins it to the sentence before",
  },
  "deploys.reason.hint_listening": {
    message: " 프로세스가 {port} 포트에서 수신 대기 중입니다.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_listening_adopted": {
    message:
      " 프로세스가 {port} 포트에서 수신 대기 중이며, 서비스가 이제 이 포트를 사용합니다.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening_adopted, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_listening_ports": {
    message:
      " 프로세스가 {port} 포트가 아니라 {ports} 포트에서 수신 대기 중입니다. 서비스 포트(또는 PORT 변수)를 HTTP를 처리하는 포트로 설정하세요.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening_ports, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_listening_primary": {
    message:
      " 프로세스가 {port} 포트에서 수신 대기 중이며, 서비스가 이제 이 포트를 주 포트로 사용합니다.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_listening_primary, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_no_open_port": {
    message:
      " 인스턴스에서 열린 포트가 감지되지 않았습니다. 서버는 PORT({port})에 지정된 포트의 호스트 0.0.0.0에 바인딩해야 합니다.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_port, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_no_open_port_before": {
    message:
      " 인스턴스에서 열린 포트가 감지되지 않았습니다. 서버는 PORT({port})에 지정된 포트의 호스트 0.0.0.0에 바인딩해야 합니다. 이전에는 {previous} 포트에서 수신 대기했습니다.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_port_before, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.hint_no_open_service_port": {
    message:
      " 인스턴스에서 열린 포트가 감지되지 않았습니다. 서버는 서비스 포트({port})의 호스트 0.0.0.0에 바인딩해야 합니다.",
    description:
      "A port hint the operator appends to a health-check stall, by its code (bex-api DeployReason part hint_no_open_service_port, w5/m356); en starts with the space that joins it to the sentence before; params are the sentence's values",
  },
  "deploys.reason.queued_build_limit": {
    message:
      "{noun}에서 동시 빌드 {active}/{limit}개가 활성 상태입니다. 슬롯을 기다리는 중입니다",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_build_limit, w5/m356): a full concurrent-build cap; {noun} is the cap's scope, worded by deploys.reasonParam.noun.*",
  },
  "deploys.reason.queued_cluster_full": {
    message: "클러스터의 빌드 용량이 가득 찼습니다. 슬롯을 기다리는 중입니다",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_cluster_full, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_slot_taken": {
    message:
      "동시에 요청된 다른 디스패치가 마지막 빌드 슬롯을 차지했습니다. 슬롯을 기다리는 중입니다",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_slot_taken, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_database": {
    message:
      "빌드하기 전에 연결된 데이터베이스 {database}을(를) 사용할 수 있을 때까지 기다리는 중입니다",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_database, w5/m356): what the build waited on; params are the sentence's values",
  },
  "deploys.reason.queued_datastore": {
    message:
      "빌드하기 전에 연결된 데이터 저장소의 연결 정보를 기다리는 중입니다",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_datastore, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_build_credential": {
    message:
      "레지스트리가 이 앱의 빌드 자격 증명을 수락하기를 기다리는 중입니다",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_build_credential, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_pull_credential": {
    message: "레지스트리가 이 앱의 풀 자격 증명을 수락하기를 기다리는 중입니다",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_pull_credential, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_capacity": {
    message: "빌드 용량을 기다리는 중: {detail}",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity, w5/m356): waiting for build capacity; {detail} is the build plane's own wait, shown as-is",
  },
  "deploys.reason.queued_capacity_pod": {
    message:
      "빌드 용량을 기다리는 중: 빌드 작업이 디스패치되었으며 빌드 파드가 생성되기를 기다립니다",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity_pod, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_capacity_no_node": {
    message: "빌드 용량을 기다리는 중: 아직 여유가 있는 빌드 노드가 없습니다",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity_no_node, w5/m356): what the build waited on",
  },
  "deploys.reason.queued_capacity_short": {
    message:
      "빌드 용량을 기다리는 중: 아직 여유가 있는 빌드 노드가 없습니다({resources} 여유 부족)",
    description:
      "A never-started build's cause by its code (bex-api DeployReason part queued_capacity_short, w5/m356): what the build waited on; params are the sentence's values",
  },
  "deploys.reasonParam.kind.startup": {
    message: "시작(startup)",
    description:
      "A deploy reason's {kind} value startup (w4/591): which health check a stalled rollout is waiting on",
  },
  "deploys.reasonParam.kind.readiness": {
    message: "준비 상태(readiness)",
    description:
      "A deploy reason's {kind} value readiness (w4/591): which health check a stalled rollout is waiting on",
  },
  "deploys.reasonParam.noun.workspace": {
    message: "워크스페이스",
    description:
      "A deploy reason's {noun} value workspace (w5/m356): the scope of a full concurrent-build cap",
  },
  "deploys.reasonWait.duration": {
    message: "{units}",
    description:
      "A deferred restart's wait, restart_wait_timed_out's {wait} (w5/m356): {wait} is bex-api's Go duration (3h0m0s), which en keeps so the sentence reads as the API's; {units} is the duration in words (deploys.reasonWait.hours and the rest), which other languages read",
  },
  "deploys.reasonWait.hours": {
    message: "{hours}시간",
    description: "A deferred restart's wait in words (w5/m356): its hours",
  },
  "deploys.reasonWait.minutes": {
    message: "{minutes}분",
    description: "A deferred restart's wait in words (w5/m356): its minutes",
  },
  "deploys.reasonWait.seconds": {
    message: "{seconds}초",
    description: "A deferred restart's wait in words (w5/m356): its seconds",
  },
};

export default koDeploys;
