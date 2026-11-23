import type { TranslationEntry } from "@/i18n";

const koServices: Record<string, TranslationEntry> = {
  "services.protectedConfirmationTitle": {
    message: "보호된 환경 확인",
    description: "Title of a protected-service destructive-action retry dialog",
  },
  "services.protectedConfirmationBody": {
    message:
      '보호된 환경에 속한 서비스입니다: "{name}". 계속하려면 bex-api가 반환한 sudo 명령을 정확히 입력하세요.',
    description: "Body of a protected-service destructive-action retry dialog",
  },
  "services.protectedConfirmationPrompt": {
    message: "확인하려면 아래에 다음을 입력하세요: {phrase}",
    description:
      "Body prompt naming the exact protected-action confirmation phrase (rendered bold by SudoCommandField)",
  },
  "services.connect": {
    message: "연결",
    description: "Open the service connection menu",
  },
  "services.connectSSH": {
    message: "SSH",
    description: "SSH section label in the service connection menu",
  },
  "services.openBrowserTerminal": {
    message: "브라우저 터미널 열기",
    description:
      "RequiresSshKey gate — zero-setup second door to the Web Shell (no SSH key)",
  },
  "services.connectInternal": {
    message: "내부",
    description:
      "Internal-address section label in the service connection menu (Render's Connect → Internal tab)",
  },
  "services.connectInternalFreeNote": {
    message:
      "무료 Web Service는 프라이빗 네트워크 트래픽을 받을 수 없습니다. 내부 주소를 사용하려면 업그레이드하세요.",
    description:
      "Connect → Internal note for a free public web service: Render's Free plan does not support receiving private network traffic, so a sibling's internal call never reaches it (w8/m130)",
  },
  "services.headerServiceAddress": {
    message: "서비스 주소",
    description:
      "Label before a private service's internal address in the detail header (Render's private-service wording)",
  },
  "services.openPorts": {
    message: "열린 포트",
    description:
      "Label before the TCP ports a private service's hostname serves, beside its Service Address and in Connect → Internal (Render's serviceDetails.openPorts, w6/m148)",
  },
  "services.openPortsPrimary": {
    message: "기본",
    description:
      "Marker on the open port that is the service's primary port — the <slug>:<port> suffix of its Service Address (w6/m148)",
  },
  "services.internalCopy": {
    message: "내부 주소 복사",
    description: "Copy service internal address button",
  },
  "services.internalCopied": {
    message: "내부 주소를 복사했습니다",
    description: "Successful internal address copy",
  },
  "services.internalCopyError": {
    message: "내부 주소를 복사하지 못했습니다",
    description: "Failed internal address copy",
  },
  "services.sshCopy": {
    message: "SSH 명령 복사",
    description: "Copy service SSH command button",
  },
  "services.sshCopied": {
    message: "SSH 명령을 복사했습니다",
    description: "Successful SSH command copy",
  },
  "services.sshCopyError": {
    message: "SSH 명령을 복사하지 못했습니다",
    description: "Failed SSH command copy",
  },
  "services.sshUnavailable": {
    message: "SSH를 사용할 수 없습니다",
    description: "Service header state when no SSH address is advertised",
  },
  "services.sshUnavailableHint": {
    message:
      "SSH를 사용하려면 실행 중인 유료 Web Service, Private Service 또는 Background Worker와 활성 게이트웨이가 필요합니다.",
    description: "Explanation for a service without an SSH address",
  },
  "services.sshUnavailableSuspended": {
    message:
      "이 서비스는 일시 중지되었습니다. SSH로 연결하려면 서비스를 재개하세요.",
    description: "Why SSH is unavailable on a suspended service (w4/143)",
  },
  "services.sshUnavailableSuspendedFree": {
    message:
      "이 서비스는 일시 중지되었고, SSH에는 유료 컴퓨팅 플랜이 필요합니다. 재개하면 트래픽은 복구되지만 무료 서비스에는 여전히 셸 액세스가 포함되지 않습니다.",
    description:
      "Why SSH is unavailable on a suspended Free service: both blockers, without implying a resume grants SSH (w4/m183)",
  },
  "services.sshUnavailableFree": {
    message:
      "SSH에는 유료 컴퓨팅 플랜이 필요합니다. 무료 서비스에는 셸 액세스가 포함되지 않습니다.",
    description: "Why SSH is unavailable on a Free-plan service (w4/143)",
  },
  "services.sshRemedyResume": {
    message: "서비스 재개",
    description:
      "Link from an SSH-unavailable state to the service's suspend/resume setting (w4/143)",
  },
  "services.sshRemedyChangePlan": {
    message: "컴퓨팅 플랜 변경",
    description:
      "Link from an SSH-unavailable state on a Free service to its compute-plan page (w4/143)",
  },
  "services.shellDescription": {
    message: "터미널에서 실행 중인 서비스 인스턴스의 셸을 엽니다.",
    description: "Running-instance SSH connection page description",
  },
  "services.shellConnectionTitle": {
    message: "SSH로 연결",
    description: "SSH connection card title",
  },
  "services.shellConnectionDescription": {
    message:
      "bex는 이미 준비된 기존 인스턴스에 연결하며, 별도의 셸 인스턴스를 만들지 않습니다.",
    description: "SSH running-instance behavior explanation",
  },
  "services.shellCommand": {
    message: "SSH 명령",
    description: "Label for a copy-ready service SSH command",
  },
  "services.shellManageKeys": {
    message: "SSH 공개 키 관리",
    description: "Link from a service shell page to account SSH key settings",
  },
  "services.shellSessionLifecycle": {
    message:
      "이 명령은 준비된 인스턴스 하나를 선택합니다. 서비스를 다시 시작하거나 재배포하거나 일시 중지하면 세션이 종료됩니다.",
    description: "SSH session selection and lifecycle guidance",
  },
  "services.shellUnavailableTitle": {
    message: "셸 액세스를 사용할 수 없습니다",
    description: "Unavailable SSH connection card title",
  },
  "services.shellUnavailableBody": {
    message:
      "셸 액세스에는 실행 중인 유료 Web Service, Private Service 또는 Background Worker와 활성 SSH 게이트웨이가 필요합니다.",
    description: "Unavailable SSH connection card explanation",
  },
  "services.shellWebTitle": {
    message: "웹 셸",
    description: "In-browser terminal card title",
  },
  "services.shellWebDescription": {
    message: "브라우저에서 실행 중인 인스턴스 안의 명령을 실행합니다.",
    description: "In-browser terminal card description",
  },
  "services.shellConnecting": {
    message: "실행 중인 인스턴스에 연결하는 중…",
    description: "Web shell status while the terminal is connecting",
  },
  "services.shellConnected": {
    message: "연결됨",
    description: "Web shell status when the terminal stream is live",
  },
  "services.shellClosed": {
    message: "세션 종료됨",
    description: "Web shell status when the terminal stream has ended",
  },
  "services.shellErrorStatus": {
    message: "연결 오류",
    description:
      "Web shell status when the session failed to connect or errored",
  },
  "services.shellEnded": {
    message: "세션 종료됨",
    description:
      "Web shell status when the server ended a live session for a stated reason (access, suspension, instance stopped, time limit)",
  },
  "services.shellEndRevoked": {
    message: "이 서비스에 대한 접근 권한이 종료되었습니다.",
    description:
      "Web shell end reason: the caller lost access to the service mid-session (deliberately unspecific)",
  },
  "services.shellEndSuspended": {
    message:
      "서비스가 일시 중지되어 이 세션이 종료되었습니다. 재개하면 새 세션을 열 수 있습니다.",
    description: "Web shell end reason: the service was suspended",
  },
  "services.shellEndIneligible": {
    message:
      "이 서비스는 더 이상 셸 접근을 지원하지 않아(요금제 또는 유형이 변경됨) 이 세션이 종료되었습니다.",
    description:
      "Web shell end reason: the service moved to the Free plan or a type without instances",
  },
  "services.shellEndInstanceGone": {
    message:
      "이 세션이 실행되던 인스턴스가 중지되었습니다. 다시 연결하면 실행 중인 인스턴스에서 셸을 열 수 있습니다.",
    description:
      "Web shell end reason: the attached instance stopped (restart, or the old instance leaving after a deploy went live)",
  },
  "services.shellEndTimeout": {
    message: "세션이 시간 제한에 도달했습니다. 다시 연결하여 계속 작업하세요.",
    description: "Web shell end reason: the session hit its maximum duration",
  },
  "services.shellEndUnverified": {
    message:
      "접근 권한을 다시 확인할 수 없어 세션이 종료되었습니다. 다시 연결하여 시도하세요.",
    description:
      "Web shell end reason: the periodic access check could not complete",
  },
  "services.shellReconnect": {
    message: "다시 연결",
    description: "Button that re-opens a closed web shell session",
  },
  "services.shellConnect": {
    message: "셸 시작",
    description: "Button that opens a web shell session",
  },
  "services.shellErrorGeneric": {
    message: "셸 세션을 시작하지 못했습니다.",
    description: "Web shell generic connection error",
  },
  "services.shellErrorUnavailable": {
    message:
      "이 플랫폼에서는 브라우저 셸이 활성화되어 있지 않습니다. 아래 SSH 명령을 대신 사용하세요.",
    description:
      "Web shell error when the browser transport is unconfigured (503)",
  },
  "services.shellInstanceLabel": {
    message: "인스턴스",
    description: "Label for the web shell instance picker",
  },
  "services.shellInstanceAny": {
    message: "준비된 아무 인스턴스",
    description:
      "Web shell instance picker option that selects a random ready replica",
  },
  "services.shellInstanceSelect": {
    message: "인스턴스 선택",
    description: "Placeholder for the web shell instance picker",
  },
  "services.shellSshFallback": {
    message: "사용 중인 터미널을 선호하시나요? 아래 SSH 명령을 사용하세요.",
    description: "Hint pointing web shell users to the copy-ready SSH command",
  },
  "services.actions": {
    message: "작업",
    description: "Accessible heading for service configuration row actions",
  },
  "services.statTotal": {
    message: "전체 서비스",
    description: "Services page stat card label",
  },
  "services.statRunning": {
    message: "실행 중",
    description: "Services page stat card label",
  },
  "services.statSuspended": {
    message: "일시 중지됨",
    description: "Services page stat card label",
  },
  "services.cardTitle": {
    message: "서비스",
    description:
      "Services table card title, also used as the metrics page back-link",
  },
  "services.colName": {
    message: "이름",
    description: "Services table column header",
  },
  "services.colStatus": {
    message: "상태",
    description: "Services table column header",
  },
  "services.colUrl": {
    message: "URL",
    description: "Services table column header",
  },
  "services.colBlueprintName": {
    message: "Blueprint 이름",
    description: "Immutable service name used to match Blueprint resources.",
  },
  "services.blueprintNameHelp": {
    message:
      "Blueprint 매니페스트의 services[].name에 이 변경 불가능한 이름을 사용하세요. 표시 이름과 공개 호스트 이름은 다를 수 있습니다.",
    description: "Immutable service name used to match Blueprint resources.",
  },
  "services.colSlug": {
    message: "슬러그",
    description:
      "Service detail header fact (globally-unique platform-host segment, Render's slug field)",
  },
  "services.instancesSuspended": {
    message: "0 (재개 시 {n})",
    description:
      "The header's Instances fact for a suspended service, which runs none; n is its configured count (w4/547)",
  },
  "services.instancesSleeping": {
    message: "0 (깨어나면 {n})",
    description:
      "The header's Instances fact for a sleeping Free service, which runs none; n is its configured count (w4/547)",
  },
  "services.colInstances": {
    message: "인스턴스",
    description: "Services table column header (replica count — bex-native)",
  },
  "services.colRevision": {
    message: "리비전",
    description: "Services table column header (active revision — bex-native)",
  },
  "services.colCreated": {
    message: "생성일",
    description: "Services table column header (relative age from createdAt)",
  },
  "services.colActions": {
    message: "작업",
    description: "Services table actions column header (screen-reader only)",
  },
  "services.statusRunning": {
    message: "실행 중",
    description: "Services table status badge",
  },
  "services.statusSuspended": {
    message: "일시 중지됨",
    description: "Services table status badge",
  },
  "services.statusSleeping": {
    message: "절전 중",
    description:
      "Services status badge: a free-tier App auto-hibernated after idle (bex extension)",
  },
  "services.undeployedChangesSuspendedHint": {
    message:
      "저장된 변경 사항이 아직 적용되지 않았습니다. 서비스를 재개한 다음 배포하여 적용하세요.",
    description:
      "Notice when a suspended service has saved settings that differ from the release it serves: no deploy can run until it resumes",
  },
  "services.undeployedChangesHint": {
    message:
      "저장된 변경 사항이 아직 적용되지 않았습니다. 표준 배포로 적용하세요.",
    description:
      "Notice when saved settings differ from the running release, including Save only, cancellation, rollback or restart",
  },
  "services.undeployedChangesApplyingHint": {
    message: "저장된 변경 사항은 현재 배포가 끝나면 적용됩니다.",
    description:
      "Notice while an in-flight deploy already carries the saved settings, so no further deploy is needed (w4/m184)",
  },
  "services.undeployedChangesOverrideHint": {
    message:
      "이 배포는 이번 배포를 위해 선택한 이미지로 실행됩니다: {running}. 저장된 이미지: {saved}. 표준 배포를 하면 저장된 이미지로 돌아갑니다.",
    description:
      "Notice when a per-deploy image override (imageUrl/imgURL) runs a different image than the saved one: nothing was saved, and a standard deploy reverts to the saved image (w4/414)",
  },
  "services.statusSleepingHint": {
    message:
      "리소스를 절약하기 위해 절전 중입니다. 공개 URL로 다음 요청이 오면 깨어납니다.",
    description:
      "Hint next to the Sleeping badge explaining free-tier auto-sleep + wake-on-request",
  },
  "services.statusPending": {
    message: "대기 중",
    description: "Services table status badge",
  },
  "services.statusBuilding": {
    message: "빌드 중",
    description: "Services table status badge",
  },
  "services.statusDeploying": {
    message: "배포 중",
    description: "Services table status badge",
  },
  "services.statusCanceled": {
    message: "취소됨",
    description:
      "Services table status badge for a service whose first-ever deploy the user canceled",
  },
  "services.statusDeleting": {
    message: "삭제 중",
    description:
      "Services status badge for a service whose deletion is finalizing",
  },
  "services.statusFailed": {
    message: "실패",
    description: "Services table status badge",
  },
  "services.statusUnknown": {
    message: "알 수 없음",
    description: "Services table status badge for an unrecognized phase",
  },
  "services.actionsMenu": {
    message: "{name} 작업",
    description: "Accessible label for the per-row actions trigger",
  },
  "services.actionSuspend": {
    message: "일시 중지",
    description: "Row action: park the service",
  },
  "services.actionResume": {
    message: "재개",
    description: "Row action: bring a suspended service back",
  },
  "services.actionRestart": {
    message: "다시 시작",
    description:
      "Row action: restart the service on the commit or image it is running",
  },
  "services.confirmSuspendTitle": {
    message: "{name} 서비스를 일시 중지하시겠습니까?",
    description: "Suspend confirmation dialog title",
  },
  "services.confirmSuspendBody": {
    message:
      "서비스가 0개로 축소되어 트래픽 처리를 중단합니다. URL과 인증서는 유지되며 언제든지 재개할 수 있습니다.",
    description:
      "Suspend confirmation dialog body, for the types served at a public host",
  },
  "services.confirmSuspendBodyNoUrl": {
    message:
      "서비스가 0개로 축소되어 실행을 중단합니다. 언제든지 재개할 수 있습니다.",
    description:
      "Suspend confirmation dialog body, for private services and background workers — neither has a public URL or certificates to keep",
  },
  "services.confirmSuspendBodyStatic": {
    message:
      "일시 중지된 동안 사이트는 서비스를 중단하고 503으로 응답합니다. URL과 인증서는 유지되며 언제든지 재개할 수 있습니다.",
    description:
      "Suspend confirmation dialog body for a static site, which has no instances to scale to zero (w4/380)",
  },
  "services.suspendCronDescription": {
    message:
      "이 Cron Job을 일시 중지하면 이후 예약된 실행이 중단됩니다. 진행 중인 실행은 계속됩니다. 중지하려면 실행 페이지에서 취소를 선택하세요.",
    description:
      "Cron suspension explanation in Settings and suspend confirmations; suspension does not cancel an active run",
  },
  "services.confirmRestartTitle": {
    message: "{name} 서비스를 다시 시작하시겠습니까?",
    description: "Restart confirmation dialog title",
  },
  "services.confirmRestartBody": {
    message:
      "{name} 서비스는 현재 실행 중인 커밋 또는 이미지로 다시 시작됩니다. 이후에 푸시된 커밋은 배포되지 않습니다. 새 인스턴스가 정상 상태가 되면 이전 인스턴스를 대체합니다.",
    description:
      "Restart confirmation dialog body (service header and services list). A restart keeps the running release, as on Render; it never picks up newer commits",
  },
  "services.confirmRestartBodyStatic": {
    message:
      "{name} 사이트는 현재 제공 중인 커밋으로 다시 게시됩니다. 이후에 푸시된 커밋은 배포되지 않습니다.",
    description:
      "Restart confirmation body for a static site, which has no instances to replace (w4/m141)",
  },
  "services.confirmCancel": {
    message: "취소",
    description: "Confirmation dialog cancel button",
  },
  "services.toastSuspendSuccess": {
    message: "{name} 일시 중지 중…",
    description: "Toast shown after a suspend request is accepted",
  },
  "services.toastResumeSuccess": {
    message: "{name} 재개 중…",
    description: "Toast shown after a resume request is accepted",
  },
  "services.toastRestartSuccess": {
    message: "{name} 다시 시작 중…",
    description: "Toast shown after a restart request is accepted",
  },
  "services.toastError": {
    message: "{name} 서비스를 업데이트하지 못했습니다. 다시 시도하세요.",
    description: "Toast shown when a lifecycle action fails",
  },
  "services.errorTitle": {
    message: "서비스를 불러오지 못했습니다",
    description: "Services list error card title",
  },
  "services.errorBody": {
    message: "bex-api 요청이 실패했습니다. 연결을 확인한 후 다시 시도하세요.",
    description: "Services list error card body",
  },
  "services.detailErrorTitle": {
    message: "서비스를 불러오지 못했습니다",
    description: "Service detail query error card title",
  },
  "services.detailErrorBody": {
    message: "bex-api 요청이 실패했습니다. 서비스는 아직 존재할 수 있습니다.",
    description: "Service detail query error card body",
  },
  "services.emptyTitle": {
    message: "아직 서비스 없음",
    description: "Services list empty state title",
  },
  "services.emptyBody": {
    message: "첫 번째 App을 배포하면 여기에 표시됩니다.",
    description: "Services list empty state body",
  },
  "services.headerServiceId": {
    message: "서비스 ID:",
    description: "Service-detail header metadata label for the service id",
  },
  "services.headerServicePhase": {
    message: "서비스",
    description:
      "Service-detail header label distinguishing operator/App phase from deploy status",
  },
  "services.headerLatestDeploy": {
    message: "최근 배포",
    description: "Service-detail header label for the newest deploy status",
  },
  "services.headerRuntime": {
    message: "런타임",
    description: "Service-detail header label for the selected runtime",
  },
  "services.headerMoreFacts": {
    message: "상세 정보",
    description:
      "Service-detail header toggle, phones only, that shows or hides the slug, instances, revision and age line",
  },
  "services.headerLastRun": {
    message: "마지막 성공 실행",
    description:
      "Service-detail header: a cron job's second fact line, before the last successful run's date (Render's \"Last successful run <date>\", w3/m555)",
  },
  "services.headerRuns": {
    message: "실행",
    description:
      "Service-detail header: the word before a cron job's command on its first fact line (Render's \"Runs <command> <schedule>\", w3/m555)",
  },
  "services.headerRunsDockerCmd": {
    message: "{dockerfile}의 {cmd}",
    description:
      "Service-detail header: what a Docker or image cron job with no command override runs; {cmd} is the code word CMD and {dockerfile} the Dockerfile's path, both shown as code (Render's \"CMD in Dockerfile\", w3/m555)",
  },
  "services.headerNoSuccessfulRuns": {
    message: "아직 성공한 실행이 없습니다.",
    description:
      "Service-detail header: a cron job's second fact line before its first successful run (Render's text, w3/m555)",
  },
  "services.suspendedNoticeManual": {
    message: "수동으로 일시 중지됨",
    description:
      "Title of the notice under a suspended cron job's header when its owner suspended it (Render's \"Manually suspended\", w3/m555)",
  },
  "services.suspendedNoticeTitle": {
    message: "일시 중지됨",
    description:
      "Title of the notice under a suspended cron job's header when the platform suspended it (Render's fallback \"Suspended\", w3/m555)",
  },
  "services.suspendedNoticeBody": {
    message: "일시 중지된 서비스에는 요금이 청구되지 않습니다.",
    description:
      "Body of the notice under a suspended cron job's header (Render's text, w3/m555)",
  },
  "services.suspendedNoticeResume": {
    message: "서비스 재개",
    description:
      "The notice's inline action that resumes a suspended cron job (Render's \"Resume service\", w3/m555)",
  },
  "services.headerCopyServiceId": {
    message: "서비스 ID 복사",
    description: "Accessible label for the header's service-id copy button",
  },
  "services.headerCopyUrl": {
    message: "서비스 URL 복사",
    description: "Accessible label for the header's live-URL copy button",
  },
  "services.headerCopied": {
    message: "클립보드에 복사했습니다",
    description: "Toast after copying a value from the service-detail header",
  },
  "services.headerCopyError": {
    message: "클립보드에 복사하지 못했습니다",
    description: "Toast when a service-detail header copy fails",
  },
  "services.navLogs": {
    message: "로그",
    description: "Service-detail nav item (logs tab)",
  },
  "services.navMetrics": {
    message: "메트릭",
    description: "Service-detail nav item (metrics tab)",
  },
  "services.navRedirects": {
    message: "리디렉션/리라이트",
    description:
      "Service-detail nav item (static-site route rules page, Render's label)",
  },
  "services.navHeaders": {
    message: "헤더",
    description:
      "Service-detail nav item (static-site response headers page, Render's label)",
  },
  "services.scalingTitle": {
    message: "자동 확장",
    description: "Scaling tab section heading",
  },
  "services.autoscaleMinInstances": {
    message: "최소 인스턴스 수",
    description:
      "Accessible name of the minimum-instances number input beside the autoscaling range slider",
  },
  "services.autoscaleMaxInstances": {
    message: "최대 인스턴스 수",
    description:
      "Accessible name of the maximum-instances number input beside the autoscaling range slider",
  },
  "services.scalingEnabled": {
    message: "자동 확장 사용",
    description: "Autoscaling enable/disable toggle label",
  },
  "services.scalingMinInstances": {
    message: "최소 인스턴스",
    description: "Autoscaling min instances input label",
  },
  "services.scalingMaxInstances": {
    message: "최대 인스턴스",
    description: "Autoscaling max instances input label",
  },
  "services.scalingTargetCPU": {
    message: "목표 CPU %",
    description: "Autoscaling target CPU utilization input label",
  },
  "services.scalingTargetMemory": {
    message: "목표 메모리 %",
    description: "Autoscaling target memory utilization input label",
  },
  "services.scalingValidationMinMax": {
    message: "최소 인스턴스는 최대 인스턴스 이하여야 합니다.",
    description: "Autoscaling form validation when min exceeds max",
  },
  "services.scalingValidationTargets": {
    message: "사용률 목표(CPU 또는 메모리)를 하나 이상 활성화해야 합니다.",
    description:
      "Autoscaling form validation when both CPU and memory targets are off",
  },
  "services.scalingSave": {
    message: "저장",
    description: "Autoscaling form save button",
  },
  "services.scalingSaved": {
    message: "자동 확장 설정이 저장되었습니다.",
    description: "Autoscaling save success toast",
  },
  "services.scalingDisabled": {
    message: "자동 확장이 비활성화되었습니다.",
    description: "Autoscaling disable success toast",
  },
  "services.scalingError": {
    message: "자동 확장 설정을 업데이트하지 못했습니다.",
    description: "Autoscaling save error toast",
  },
  "services.scalingDescription": {
    message:
      "CPU와 메모리 사용률에 따라 이 서비스를 자동으로 확장하거나 축소합니다.",
    description: "Autoscaling card description",
  },
  "services.scalingOn": {
    message: "자동 확장 켜짐",
    description: "Autoscaling main toggle label when enabled",
  },
  "services.scalingPaidOnly": {
    message:
      "자동 확장은 유료 요금제에서 사용할 수 있습니다. 활성화하려면 업그레이드하세요.",
    description: "Autoscaling card explanation for a free service",
  },
  "services.scalingPaidOnlyInactive": {
    message:
      "자동 확장 설정은 저장되어 있지만 Free 요금제에서는 적용되지 않습니다. 이 서비스가 유료 요금제로 이동하면 적용됩니다. 저장된 설정을 제거하려면 자동 확장을 끄세요.",
    description:
      "Autoscaling card explanation for a free service carrying a stored autoscaling config (w4/365)",
  },
  "services.scalingProPlanOnly": {
    message:
      "자동 확장은 Pro 이상의 워크스페이스 요금제에서 사용할 수 있습니다. 이 워크스페이스는 Hobby 요금제입니다.",
    description:
      "Autoscaling card notice on a Hobby workspace, followed by an Open Billing link (w7/m429)",
  },
  "services.scalingProPlanOnlyRunning": {
    message:
      "자동 확장은 Pro 이상의 워크스페이스 요금제에서 사용할 수 있습니다. 이 워크스페이스는 Hobby 요금제이므로 아래 정책은 계속 실행되며 끌 수 있지만, 변경하거나 다시 켜려면 Pro가 필요합니다.",
    description:
      "Autoscaling card notice on a Hobby workspace whose service already autoscales, followed by an Open Billing link (w7/m429)",
  },
  "services.scalingMetricsUnavailableTitle": {
    message: "자동 확장이 일시 중지됨: 사용률 메트릭을 사용할 수 없습니다",
    description:
      "Autoscaling card notice title while the operator cannot read the service's CPU and memory utilization (w4/m364)",
  },
  "services.scalingMetricsUnavailableBody": {
    message:
      "CPU 및 메모리 사용률을 다시 읽을 수 있을 때까지 서비스는 현재 인스턴스 수를 유지합니다. 자동 확장 설정은 저장되어 있습니다.",
    description:
      "Autoscaling card notice body while utilization metrics are unavailable (w4/m364)",
  },
  "services.scalingOff": {
    message: "자동 확장 꺼짐",
    description: "Autoscaling main toggle label when disabled",
  },
  "services.scalingInstancesTitle": {
    message: "인스턴스 수",
    description: "Autoscaling instances range-slider section heading",
  },
  "services.scalingInstancesHint": {
    message: "bex는 지정한 범위 안에서 이 서비스의 인스턴스 수를 조정합니다.",
    description: "Autoscaling instances range-slider section description",
  },
  "services.scalingCPUTitle": {
    message: "목표 CPU 사용률",
    description: "Autoscaling CPU metric section heading",
  },
  "services.scalingCPUHint": {
    message:
      "평균 CPU 사용률이 이 값보다 크게 높거나 낮으면 bex가 그에 맞춰 인스턴스를 추가하거나 제거합니다.",
    description: "Autoscaling CPU metric section description",
  },
  "services.scalingMemoryTitle": {
    message: "목표 메모리 사용률",
    description: "Autoscaling memory metric section heading",
  },
  "services.scalingMemoryHint": {
    message:
      "평균 메모리 사용률이 이 값보다 크게 높거나 낮으면 bex가 그에 맞춰 인스턴스를 추가하거나 제거합니다.",
    description: "Autoscaling memory metric section description",
  },
  "services.scalingCancel": {
    message: "취소",
    description: "Autoscaling form cancel button",
  },
  "services.scalingSaveChanges": {
    message: "변경 사항 저장",
    description: "Autoscaling form save-changes button",
  },
  "services.scalingDisableConfirmTitle": {
    message: "자동 확장을 비활성화하시겠습니까?",
    description: "Autoscaling disable confirmation dialog title (w7/m43)",
  },
  "services.scalingDisableConfirmBody": {
    message: "서비스가 수동 확장에 지정된 고정 개수의 인스턴스로 실행됩니다.",
    description: "Autoscaling disable confirmation dialog body",
  },
  "services.scalingDisableConfirmAction": {
    message: "비활성화",
    description: "Autoscaling disable confirmation dialog confirm button",
  },
  "services.scalingManualTitle": {
    message: "수동 확장",
    description: "Scaling page: manual instance-count card title (w7/m43)",
  },
  "services.scalingManualDescription": {
    message:
      "자동으로 로드 밸런싱되는 여러 인스턴스를 실행합니다. 모든 인스턴스는 같은 컴퓨팅 플랜을 사용하며 그에 따라 요금이 청구됩니다.",
    description: "Scaling page: manual instance-count card description",
  },
  "services.scalingManualDescriptionFree": {
    message:
      "무료 서비스는 인스턴스 하나만 실행합니다. 로드 밸런싱되는 여러 인스턴스를 실행하려면 유료 요금제로 업그레이드하세요.",
    description:
      "Scaling page: manual instance-count card description on the free plan",
  },
  "services.scalingManualInstances": {
    message: "인스턴스",
    description: "Scaling page: manual instance-count slider label",
  },
  "services.scalingMetricsTitle": {
    message: "최근 메트릭",
    description: "Scaling page: recent-metrics section title (w7/m43)",
  },
  "services.scalingMetricsNote": {
    message: "최근 48시간의 메트릭을 표시합니다.",
    description: "Scaling page: recent-metrics window note",
  },
  "services.scalingMetricsViewAll": {
    message: "모든 메트릭 보기",
    description: "Scaling page: link to the full Metrics tab",
  },
  "services.scalingMetricsMemory": {
    message: "평균 메모리 사용률",
    description: "Scaling page: averaged memory utilization chart title",
  },
  "services.scalingMetricsCPU": {
    message: "평균 CPU 사용률",
    description: "Scaling page: averaged CPU utilization chart title",
  },
  "services.scalingMetricsAcross": {
    message: "모든 인스턴스 기준",
    description: "Scaling page: recent-metrics chart subtitle",
  },
  "services.scalingMetricsEmpty": {
    message: "최근 48시간 동안 수집된 데이터 없음",
    description: "Scaling page: recent-metrics empty state",
  },
  "services.navEnvironment": {
    message: "환경",
    description: "Service-detail nav item (environment variables tab)",
  },
  "services.envTitle": {
    message: "환경 변수",
    description: "Environment tab card title",
  },
  "services.envDescription": {
    message:
      "환경별 설정과 시크릿을 지정한 다음, 코드에서 해당 값을 읽어 사용하세요.",
    description: "Environment tab card description",
  },
  "services.envColKey": {
    message: "키",
    description: "Environment table column header (variable name)",
  },
  "services.envColValue": {
    message: "값",
    description: "Environment table column header (variable value)",
  },
  "services.envManifestManaged": {
    message: "Blueprint에서 관리됨",
    description:
      "Badge on an environment row whose value the service's render.yaml manifest owns",
  },
  "services.envManifestManagedBody": {
    message:
      "이 변수는 서비스의 render.yaml 매니페스트에 선언되어 있으며 값은 매니페스트가 관리합니다. 값을 바꾸려면 매니페스트를 수정하고 Blueprint를 동기화하세요. 또는 `sync: false`로 선언하여 대시보드가 관리하도록 할 수 있습니다.",
    description:
      "Explanation under a manifest-managed environment row: why it cannot be edited here and what to do instead",
  },
  "services.envShowSecret": {
    message: "값 표시",
    description: "Environment row button to reveal a masked value",
  },
  "services.envHideSecret": {
    message: "값 숨기기",
    description: "Environment row button to re-mask a revealed value",
  },
  "services.envRevealError": {
    message: "값을 불러오지 못했습니다.",
    description: "Environment row inline error when a value reveal fails",
  },
  "services.envEmptyTitle": {
    message: "환경 변수 없음",
    description: "Environment tab empty-state title",
  },
  "services.envEmptyBody": {
    message: "이 서비스를 구성하려면 변수를 추가하세요.",
    description: "Environment tab empty-state body",
  },
  "services.envUnavailableTitle": {
    message: "환경 변수를 사용할 수 없습니다",
    description:
      "Environment tab state when the secret store is unconfigured (503)",
  },
  "services.envUnavailableBody": {
    message: "이 배포에는 시크릿 저장소가 구성되어 있지 않습니다.",
    description: "Environment tab unavailable-state body",
  },
  "services.envForbiddenTitle": {
    message: "권한 없음",
    description: "Environment tab state when the caller lacks permission (403)",
  },
  "services.envForbiddenBody": {
    message: "이 서비스의 환경 변수를 볼 권한이 없습니다.",
    description: "Environment tab forbidden-state body",
  },
  "services.envErrorTitle": {
    message: "환경 변수를 불러오지 못했습니다",
    description: "Environment tab generic error title",
  },
  "services.envErrorBody": {
    message: "문제가 발생했습니다. 다시 시도하세요.",
    description: "Environment tab generic error body",
  },
  "services.envAdd": {
    message: "변수 추가",
    description: "Environment tab button to open the add-variable form",
  },
  "services.envExport": {
    message: "내보내기",
    description:
      "Environment tab button that downloads all service variables as dotenv",
  },
  "services.envExportSuccess": {
    message: "환경을 내보냈습니다",
    description:
      "Toast after every current environment value was safely exported",
  },
  "services.envExportError": {
    message: "전체 환경을 내보내지 못했습니다. 다운로드된 파일이 없습니다.",
    description:
      "Fail-closed toast when any environment value cannot be revealed",
  },
  "services.envEdit": {
    message: "편집",
    description: "Environment row button to edit a variable's value",
  },
  "services.envDelete": {
    message: "삭제",
    description: "Environment row button to remove a variable",
  },
  "services.envValueFor": {
    message: "{name}의 값",
    description:
      "Accessible name for a Value textbox once its Key is set (m101/t013)",
  },
  "services.envDeleteOne": {
    message: "{name} 삭제",
    description:
      "Accessible name for a Delete button once its Key is set (m101/t013)",
  },
  "services.envSave": {
    message: "저장",
    description: "Environment add/edit form save button",
  },
  "services.envCancel": {
    message: "취소",
    description: "Environment add/edit form cancel button",
  },
  "services.envKeyPlaceholder": {
    message: "NAME_OF_VARIABLE",
    description: "Environment add-variable key input placeholder",
  },
  "services.envValuePlaceholder": {
    message: "값",
    description: "Environment value input placeholder",
  },
  "services.envGenerate": {
    message: "생성",
    description: "Generate a cryptographically random environment value",
  },
  "services.envGeneratePlaceholder": {
    message: "저장 시 안전하게 생성됩니다",
    description:
      "Environment value placeholder while server generation is selected",
  },
  "services.envInvalidKey": {
    message:
      "영문자, 숫자, 점, 대시, 밑줄을 사용해 최대 253자까지 입력하세요. '.' 단독이거나 '..'으로 시작하는 이름은 사용할 수 없습니다.",
    description:
      "Environment add-variable validation message for an invalid key",
  },
  "services.envDeleteConfirmTitle": {
    message: "{key} 제거",
    description: "Environment delete-confirmation dialog title",
  },
  "services.envDeleteConfirmBody": {
    message: "이 변수 없이 서비스가 다시 배포됩니다.",
    description: "Environment delete-confirmation dialog body",
  },
  "services.envRolloutNote": {
    message: "변경 사항을 적용하기 위해 서비스를 다시 배포하는 중입니다.",
    description:
      "Toast description after an env-var write (bex rolls the pods)",
  },
  "services.envSaveSuccess": {
    message: "{key} 저장됨",
    description: "Toast on a successful env-var add/update",
  },
  "services.envSaveError": {
    message: "{key} 저장 실패",
    description: "Toast on a failed env-var add/update",
  },
  "services.envDeleteSuccess": {
    message: "{key} 제거됨",
    description: "Toast on a successful env-var delete",
  },
  "services.envDeleteError": {
    message: "{key} 제거 실패",
    description: "Toast on a failed env-var delete",
  },
  "services.secretFilesTitle": {
    message: "시크릿 파일",
    description: "Environment tab secret-files section title",
  },
  "services.secretFilesDescription": {
    message:
      "개인 키나 .env 파일처럼 비밀 내용을 담은 파일을 저장합니다. 다음 배포 후 서비스는 각 파일을 /etc/secrets/<파일 이름>에서 읽습니다.",
    description: "Environment tab secret-files section description",
  },
  "services.secretFileColName": {
    message: "파일 이름",
    description: "Secret-files table column header (file name)",
  },
  "services.secretFileColContent": {
    message: "내용",
    description: "Secret-files table column header (file body)",
  },
  "services.secretFileRevealError": {
    message:
      "이 시크릿 파일의 내용을 불러오지 못했습니다. 대화 상자를 닫고 다시 시도하세요.",
    description:
      "Shown in the secret-file dialog when reading the stored contents fails.",
  },
  "services.secretFilesEmptyTitle": {
    message: "시크릿 파일 없음",
    description: "Secret-files empty-state title",
  },
  "services.secretFilesEmptyBody": {
    message: "비밀 내용을 이 서비스에 마운트하려면 파일을 추가하세요.",
    description: "Secret-files empty-state body",
  },
  "services.secretFilesUnavailableTitle": {
    message: "시크릿 파일을 사용할 수 없습니다",
    description:
      "Secret-files state when the secret store is unconfigured (503)",
  },
  "services.secretFilesUnavailableBody": {
    message: "이 배포에는 시크릿 저장소가 구성되어 있지 않습니다.",
    description: "Secret-files unavailable-state body",
  },
  "services.secretFilesForbiddenTitle": {
    message: "권한 없음",
    description: "Secret-files state when the caller lacks permission (403)",
  },
  "services.secretFilesForbiddenBody": {
    message: "이 서비스의 시크릿 파일을 볼 권한이 없습니다.",
    description: "Secret-files forbidden-state body",
  },
  "services.secretFilesErrorTitle": {
    message: "시크릿 파일을 불러오지 못했습니다",
    description: "Secret-files generic error title",
  },
  "services.secretFilesErrorBody": {
    message: "문제가 발생했습니다. 다시 시도하세요.",
    description: "Secret-files generic error body",
  },
  "services.secretFileAdd": {
    message: "시크릿 파일 추가",
    description: "Secret-files button to open the add-file form",
  },
  "services.secretFileNamePlaceholder": {
    message: "filename.ext",
    description: "Secret-files add-file name input placeholder",
  },
  "services.secretFileContentPlaceholder": {
    message: "파일 내용",
    description: "Secret-files content input placeholder",
  },
  "services.secretFileInvalidName": {
    message:
      "영문자, 숫자, 점, 대시, 밑줄을 사용해 최대 253자까지 입력하세요. '.' 또는 '..'은 사용할 수 없습니다.",
    description: "Secret-files add-file validation message for an invalid name",
  },
  "services.secretFileNameHint": {
    message:
      "영문자, 숫자, 점, 대시, 밑줄을 사용해 최대 253자까지 입력하세요. '.' 또는 '..'은 사용할 수 없습니다.",
    description:
      "Neutral file-name rule under a new secret-file row before the user types or blurs — same rule as secretFileInvalidName but helper styling, not an error",
  },
  "services.secretFileDeleteConfirmTitle": {
    message: "{name} 제거",
    description: "Secret-file delete-confirmation dialog title",
  },
  "services.secretFileDeleteConfirmBody": {
    message: "이 파일 없이 서비스가 다시 배포됩니다.",
    description: "Secret-file delete-confirmation dialog body",
  },
  "services.secretFileSaveSuccess": {
    message: "{name} 저장됨",
    description: "Toast on a successful secret-file add/update",
  },
  "services.secretFileSaveError": {
    message: "{name} 저장 실패",
    description: "Toast on a failed secret-file add/update",
  },
  "services.secretFileDeleteSuccess": {
    message: "{name} 제거됨",
    description: "Toast on a successful secret-file delete",
  },
  "services.secretFileDeleteError": {
    message: "{name} 제거 실패",
    description: "Toast on a failed secret-file delete",
  },
  "services.envGroupsTitle": {
    message: "환경 그룹",
    description: "Environment tab env-groups section title",
  },
  "services.envGroupsDescription": {
    message:
      "이 서비스와 다른 서비스에 연결할 수 있는, 재사용 가능한 환경 변수 및 시크릿 파일 묶음입니다.",
    description: "Environment tab env-groups section description",
  },
  "services.envGroupsEmptyTitle": {
    message: "환경 그룹 없음",
    description: "Env-groups empty-state title",
  },
  "services.envGroupsEmptyBody": {
    message: "서비스 간에 설정을 공유하려면 그룹을 만드세요.",
    description: "Env-groups empty-state body",
  },
  "services.envGroupsUnavailableTitle": {
    message: "환경 그룹을 사용할 수 없습니다",
    description: "Env-groups state when the secret store is unconfigured (503)",
  },
  "services.envGroupsUnavailableBody": {
    message: "이 배포에는 시크릿 저장소가 구성되어 있지 않습니다.",
    description: "Env-groups unavailable-state body",
  },
  "services.envGroupsForbiddenTitle": {
    message: "권한 없음",
    description: "Env-groups state when the caller lacks permission (403)",
  },
  "services.envGroupsForbiddenBody": {
    message: "환경 그룹을 볼 권한이 없습니다.",
    description: "Env-groups forbidden-state body",
  },
  "services.envGroupsErrorTitle": {
    message: "환경 그룹을 불러오지 못했습니다",
    description: "Env-groups generic error title",
  },
  "services.envGroupsErrorBody": {
    message: "문제가 발생했습니다. 다시 시도하세요.",
    description: "Env-groups generic error body",
  },
  "services.envGroupCreate": {
    message: "그룹 만들기",
    description: "Env-groups button to open the create-group form",
  },
  "services.envGroupCreateSubmit": {
    message: "만들기",
    description: "Env-groups create-group form submit button",
  },
  "services.envGroupNamePlaceholder": {
    message: "group-name",
    description: "Env-groups create-group name input placeholder",
  },
  "services.envGroupNameLabel": {
    message: "그룹 이름",
    description: "Env-groups create-group name input accessible label",
  },
  "services.envGroupInvalidName": {
    message: "그룹 이름을 입력하세요.",
    description:
      "Env-groups create-group validation message for an invalid name",
  },
  "services.envGroupLinked": {
    message: "연결됨",
    description:
      "Env-groups badge: this group is linked to the current service",
  },
  "services.envGroupWorkspaceScope": {
    message: "워크스페이스",
    description: "Scope label for a workspace-scoped environment group",
  },
  "services.envGroupOutOfScope": {
    message:
      "{groupScope} 범위로 지정되어 있지만 이 서비스는 {serviceScope}에 속해 있습니다. 변수와 파일은 계속 적용됩니다.",
    description:
      "Notice on a linked group whose scope no longer matches the service (w4/m238)",
  },
  "services.envGroupOutOfScopeNoEnvironment": {
    message:
      "{groupScope} 범위로 지정되어 있지만 이 서비스에는 환경이 없습니다. 변수와 파일은 계속 적용됩니다.",
    description:
      "Notice on a linked group scoped to an environment while the service has none (w4/m238)",
  },
  "services.envGroupEmptyContents": {
    message: "아직 변수나 파일이 없습니다.",
    description: "Env-groups: shown when a group has no vars or secret files",
  },
  "services.envGroupKeyOverridden": {
    message: "이 서비스 자체의 {key} 환경 변수에 의해 재정의됨",
    description:
      "Tooltip on a linked group's key badge when a service variable of the same name shadows it at runtime (service wins)",
  },
  "services.envGroupFileOverridden": {
    message: "이 서비스 자체의 {name} 시크릿 파일에 의해 재정의됨",
    description:
      "Tooltip on a linked group's secret-file badge when a service secret file of the same name shadows it at runtime (service wins)",
  },
  "services.envGroupKeyShadowedByGroup": {
    message: "나중에 생성된 {group}에 의해 재정의됨",
    description:
      "Tooltip on a linked group's key badge when a MORE recently created linked group defines the same key and wins it",
  },
  "services.envGroupFileShadowedByGroup": {
    message: "나중에 생성된 {group}에 의해 재정의됨",
    description:
      "Tooltip on a linked group's secret-file badge when a MORE recently created linked group defines the same file name and wins it",
  },
  "services.envGroupOverriddenNote": {
    message:
      "취소선이 있는 키는 이 서비스 자체의 환경 변수에 의해 재정의됩니다.",
    description:
      "Legend under a linked group's key list when at least one key is shadowed by a service variable",
  },
  "services.envGroupLink": {
    message: "연결",
    description: "Env-groups button: attach this group to the current service",
  },
  "services.envGroupUnlink": {
    message: "연결 해제",
    description:
      "Env-groups button: detach this group from the current service",
  },
  "services.envGroupLinkAria": {
    message: "{name} 연결",
    description:
      "Accessible name of an env-group row's Link button on the service Environment tab, naming the group (w4/537)",
  },
  "services.envGroupUnlinkAria": {
    message: "{name} 연결 해제",
    description:
      "Accessible name of an env-group row's Unlink button on the service Environment tab, naming the group (w4/537)",
  },
  "services.envGroupDelete": {
    message: "삭제",
    description: "Env-groups action: delete the group",
  },
  "services.envGroupDeleteConfirmTitle": {
    message: "{name} 삭제",
    description: "Env-group delete-confirmation dialog title",
  },
  "services.envGroupDeleteConfirmBody": {
    message:
      "이 그룹은 연결된 모든 서비스에서 제거됩니다. 이 작업은 되돌릴 수 없습니다.",
    description: "Env-group delete-confirmation dialog body",
  },
  "services.envGroupCreateSuccess": {
    message: "{name} 생성됨",
    description: "Toast on a successful env-group create",
  },
  "services.envGroupCreateError": {
    message: "{name} 생성 실패",
    description: "Toast on a failed env-group create",
  },
  "services.envGroupDeleteSuccess": {
    message: "그룹을 삭제했습니다",
    description: "Toast on a successful env-group delete",
  },
  "services.envGroupDeleteError": {
    message: "그룹을 삭제하지 못했습니다",
    description: "Toast on a failed env-group delete",
  },
  "services.envGroupLinkSuccess": {
    message: "그룹을 연결했습니다",
    description: "Toast on a successful env-group link",
  },
  "services.envGroupLinkError": {
    message: "그룹을 연결하지 못했습니다",
    description: "Toast on a failed env-group link",
  },
  "services.envGroupUnlinkSuccess": {
    message: "그룹 연결을 해제했습니다",
    description: "Toast on a successful env-group unlink",
  },
  "services.envGroupUnlinkError": {
    message: "그룹 연결을 해제하지 못했습니다",
    description: "Toast on a failed env-group unlink",
  },
  "services.navSettings": {
    message: "설정",
    description: "Service-detail nav item (settings tab)",
  },
  "services.settingsNavigation": {
    message: "설정 섹션",
    description: "Accessible label for the service settings section navigation",
  },
  "services.settingsDescription": {
    message: "이 서비스의 이름, 인스턴스 크기 및 기타 설정을 구성합니다.",
    description: "Settings tab card description",
  },
  "services.generalTitle": {
    message: "일반",
    description:
      "Settings tab: first card heading, Render's 'General' section (w5/m52)",
  },
  "services.regionLabel": {
    message: "리전",
    description:
      "Settings General card: read-only platform placement row label",
  },
  "services.regionHint": {
    message:
      "서비스가 실행되는 리전입니다. 같은 리전의 서비스끼리는 프라이빗 네트워크로 통신할 수 있습니다.",
    description: "Settings General card: Region row explanatory copy",
  },
  "services.editRowSave": {
    message: "변경 사항 저장",
    description:
      "Shared editable-field row: save button shown while editing (Render parity)",
  },
  "services.editRowCancel": {
    message: "취소",
    description: "Shared editable-field row: cancel button shown while editing",
  },
  "services.editRowOptional": {
    message: "선택 사항",
    description:
      "Shared editable-field row: badge next to a label for an optional field",
  },
  "services.displayNameLabel": {
    message: "서비스 이름",
    description:
      "Settings tab row label for the mutable human-facing service name",
  },
  "services.displayNameHint": {
    message:
      "서비스 ID는 그대로 유지됩니다({id}). URL과 인프라는 바뀌지 않습니다.",
    description:
      "Settings tab explanation that a display-name change preserves identity",
  },
  "services.displayNameEdit": {
    message: "서비스 이름 편집",
    description: "Accessible label for the service-name edit button",
  },
  "services.displayNameSuccess": {
    message: '서비스 이름을 변경했습니다: "{name}"',
    description: "Toast after setDisplayName succeeds",
  },
  "services.displayNameCleared": {
    message: "사용자 지정 서비스 이름을 지웠습니다.",
    description: "Toast after clearing displayName",
  },
  "services.displayNameRefreshFailed": {
    message:
      '이름을 "{name}" 이름으로 변경했지만 이 페이지를 새로 고치지 못했습니다. 다시 시도하거나 페이지를 새로고침하여 저장된 이름을 확인하세요.',
    description:
      "Warning after a service rename committed but the page could not re-read the saved name; offers Try again",
  },
  "services.displayNameUnpublished": {
    message:
      "새 이름은 저장되었지만 이 페이지에 아직 표시하지 못했습니다. 다시 편집하기 전에 알림의 다시 시도를 사용하거나 페이지를 새로고침하세요.",
    description:
      "Why the service name row is locked after a rename saved but its re-read failed",
  },
  "services.displayNameError": {
    message: "서비스 이름을 변경하지 못했습니다. 다시 시도하세요.",
    description: "Toast after setDisplayName fails",
  },
  "services.settingsInstanceType": {
    message: "컴퓨팅 플랜",
    description: "Settings tab row label for the App's current plan/tier",
  },
  "services.settingsNoInstanceType": {
    message: "컴퓨팅 플랜이 설정되지 않음",
    description: "Settings tab state for an untiered (bare-CR) App",
  },
  "services.settingsUpdate": {
    message: "업데이트",
    description: "Settings tab link to the compute-plan picker",
  },
  "services.settingsIdleTimeout": {
    message: "유휴 시간 제한",
    description:
      "Settings tab: label for the free-tier auto-sleep window control",
  },
  "services.settingsIdleTimeoutHint": {
    message:
      "무료 서비스는 공개 HTTP 요청이나 WebSocket 메시지가 없는 상태로 이 시간이 지나면 절전 상태가 되며, 다음 공개 요청이 오면 깨어납니다. 다른 서비스의 내부 요청은 절전을 막거나 서비스를 깨우지 않습니다.",
    description: "Settings tab: idle-timeout control help text (bex extension)",
  },
  "services.settingsIdleTimeoutPaid": {
    message: "유료 서비스는 항상 켜져 있으며 절전 상태가 되지 않습니다.",
    description: "Settings tab: shown instead of the control on a paid plan",
  },
  "services.idleTimeoutDefault": {
    message: "플랫폼 기본값",
    description: "Idle-timeout option: 0 seconds = the operator's own window",
  },
  "services.idleTimeoutMinutes": {
    message: "{minutes}분",
    description: "Idle-timeout option label in minutes",
  },
  "services.idleTimeoutHours": {
    message: "{hours}시간",
    description: "Idle-timeout option label in hours",
  },
  "services.idleTimeoutSeconds": {
    message: "{seconds}초",
    description: "Idle-timeout option label in seconds (non-round values)",
  },
  "services.idleTimeoutSuccess": {
    message: "유휴 시간 제한을 업데이트했습니다.",
    description: "Toast after setIdleTimeout succeeds",
  },
  "services.idleTimeoutError": {
    message: "유휴 시간 제한을 업데이트하지 못했습니다.",
    description: "Toast after setIdleTimeout fails",
  },
  "services.settingsMaxShutdownDelay": {
    message: "최대 종료 지연 시간",
    description: "Settings row label for the graceful SIGTERM window",
  },
  "services.settingsMaxShutdownDelayHint": {
    message:
      "SIGTERM 이후 프로세스를 강제 종료하기 전에 1~300초 동안 기다립니다(기본값 30).",
    description: "Settings row help for the shutdown-delay range",
  },
  "services.maxShutdownDelayDiskReason": {
    message: "영구 디스크가 있는 서비스는 종료 지연 시간을 변경할 수 없습니다.",
    description:
      "Settings hint replacing the range on a service with a disk (Render's rule, render.com/docs/disks); a link to the Disk tab follows",
  },
  "services.maxShutdownDelayDiskLink": {
    message: "디스크 보기",
    description:
      "Link after maxShutdownDelayDiskReason to the service's Disk tab",
  },
  "services.maxShutdownDelayEdit": {
    message: "최대 종료 지연 시간 편집",
    description: "Accessible label for the shutdown-delay edit button",
  },
  "services.maxShutdownDelaySuccess": {
    message:
      "최대 종료 지연 시간을 업데이트했습니다. 새 배포가 시작되었습니다.",
    description: "Toast after setMaxShutdownDelay succeeds",
  },
  "services.maxShutdownDelayRangeError": {
    message: "{min}에서 {max} 사이의 정수(초)를 입력하세요.",
    description:
      "Inline validation message for a max shutdown delay outside 1–300 or not a whole number (w4/494)",
  },
  "services.maxShutdownDelayError": {
    message: "최대 종료 지연 시간을 업데이트하지 못했습니다.",
    description: "Toast after setMaxShutdownDelay fails",
  },
  "services.settingsHealthChecksTitle": {
    message: "헬스 체크",
    description: "Settings tab: Health Checks section card title",
  },
  "services.settingsHealthChecksDescription": {
    message:
      "서비스를 모니터링하기 위해 bex가 주기적으로 확인하는 HTTP 경로를 구성합니다.",
    description: "Settings tab: Health Checks section card description",
  },
  "services.settingsHealthCheckPath": {
    message: "헬스 체크 경로",
    description: "Settings tab: health-check path row label",
  },
  "services.settingsHealthCheckPathHint": {
    message:
      "서비스가 정상인지 판단하기 위해 bex가 확인하는 HTTP 경로입니다. 5초 이내에 2xx 또는 3xx로 응답해야 하며, 60초 동안 연속으로 실패하면 인스턴스가 다시 시작되므로 가벼운 경로를 지정하세요. 비워 두면 프로세스가 포트에서 수신 대기 중인지만 확인하며, 2xx를 반환하는 가벼운 경로가 없는 서비스에 적합합니다.",
    description: "Settings tab: health-check path row hint text",
  },
  "services.settingsHealthCheckPathPlaceholder": {
    message: "비어 있음 — 포트만 확인",
    description: "Settings tab: health-check path input placeholder",
  },
  "services.settingsHealthCheckPathEdit": {
    message: "헬스 체크 경로 편집",
    description:
      "Settings tab: accessible label for the health-check path edit-pencil button",
  },
  "services.healthCheckPathSuccess": {
    message: "헬스 체크 경로를 업데이트했습니다. 새 배포가 시작되었습니다.",
    description: "Toast after setHealthCheckPath succeeds",
  },
  "services.healthCheckPathError": {
    message: "헬스 체크 경로를 업데이트하지 못했습니다.",
    description: "Toast after setHealthCheckPath fails",
  },
  "services.settingsPortTitle": {
    message: "포트",
    description: "Settings tab: Port section card title",
  },
  "services.settingsPortDescription": {
    message:
      "컨테이너가 수신 대기하는 포트입니다. bex가 이 포트로 트래픽을 라우팅하고 $PORT로 주입합니다.",
    description: "Settings tab: Port section card description",
  },
  "services.settingsPort": {
    message: "포트",
    description: "Settings tab: service port row label",
  },
  "services.settingsPortHint": {
    message:
      "컨테이너가 수신 대기하는 포트입니다(기본값 {port}). bex가 이 포트로 트래픽을 라우팅하고 $PORT로 주입합니다. Render와 마찬가지로 서비스의 PORT 환경 변수가 이 값을 재정의합니다. 저장하면 새 배포가 시작되고 인스턴스가 다시 시작됩니다.",
    description: "Settings tab: service port row hint text",
  },
  "services.settingsPortDetected": {
    message: "감지됨: {port}",
    description:
      "Settings tab: port placeholder for a service whose serving port bex detected from the listener its process binds, while it names no port of its own (w8/338)",
  },
  "services.settingsPortDetectedHint": {
    message:
      "bex가 서비스가 포트 {port}에서 수신 대기하는 것을 감지하여 이 포트로 트래픽을 라우팅합니다. Render와 마찬가지로 $PORT는 {defaultPort}(으)로 유지됩니다. 포트를 입력하면 그 포트를 설정할 수 있으며, bex가 그 포트로 트래픽을 라우팅하고 $PORT로 주입합니다. 저장하면 새 배포가 시작되고 인스턴스가 다시 시작됩니다.",
    description:
      "Settings tab: service port row hint for a service serving on a detected port; PORT stays the default (w8/177, w8/267, w8/338)",
  },
  "services.settingsPortImageHint": {
    message:
      "이 서비스는 이미지의 EXPOSE 명령에서 포트를 가져옵니다. 여기서 포트를 지정하면 기본 리스너(bex가 트래픽을 라우팅하고, 헬스 체크를 수행하고, $PORT로 주입하는 포트)가 선택되며, 이미지에 선언된 나머지 TCP 포트는 계속 게시됩니다. 비우면 다시 이미지의 포트를 사용합니다. 저장하면 새 배포가 시작되고 인스턴스가 다시 시작됩니다.",
    description:
      "Settings tab: service port row hint for an image-port service (a Docker-built or prebuilt-image web or private service) whose ports come from its image's EXPOSE instructions; a blank port returns it to the image's (w6/m148, w4/m243, w4/m323)",
  },
  "services.settingsPortEdit": {
    message: "포트 편집",
    description:
      "Settings tab: accessible label for the service port edit-pencil button",
  },
  "services.portRangeError": {
    message: "{min}에서 {max} 사이의 포트를 입력하세요.",
    description:
      "Validation message for a port outside the range the container can bind",
  },
  "services.portSuccess": {
    message: "포트를 업데이트했습니다. 새 배포가 진행 중입니다.",
    description: "Toast after setPort succeeds",
  },
  "services.portError": {
    message: "포트를 업데이트하지 못했습니다.",
    description: "Toast after setPort fails",
  },
  "services.settingsNotificationsTitle": {
    message: "알림",
    description: "Settings tab: Notifications section card title",
  },
  "services.settingsNotificationsDescription": {
    message: "이 서비스에 대해 받을 배포 알림을 선택합니다.",
    description: "Settings tab: Notifications section card description",
  },
  "services.notificationsLabel": {
    message: "서비스 알림",
    description: "Settings tab: service notification policy label",
  },
  "services.notificationsHint": {
    message:
      "이 서비스에 한해 워크스페이스 기본값을 재정의합니다. 워크스페이스 기본값은 워크스페이스 설정 → 알림에서 지정합니다.",
    description: "Settings tab: service notification policy explanation",
  },
  "services.notificationsEdit": {
    message: "서비스 알림 편집",
    description: "Accessible label for the service-notifications edit button",
  },
  "services.notificationsOptionDefault": {
    message: "워크스페이스 기본값 사용 ({level})",
    description:
      "Service notification option: inherit workspace default; {level} is the workspace's saved level (e.g. Only failure notifications)",
  },
  "services.notificationsOptionDefaultUnknown": {
    message: "워크스페이스 기본값 사용",
    description:
      "Service notification option: inherit workspace default, while the workspace level is loading or unreadable",
  },
  "services.notificationsOptionAll": {
    message: "모든 알림",
    description: "Service notification option: all lifecycle mail",
  },
  "services.notificationsOptionFailure": {
    message: "실패 알림만",
    description: "Service notification option: failures only",
  },
  "services.notificationsOptionNone": {
    message: "없음",
    description: "Service notification option: no mail",
  },
  "services.notificationsSuccess": {
    message: "서비스 알림 설정을 업데이트했습니다.",
    description: "Toast after notification policy update succeeds",
  },
  "services.notificationsError": {
    message: "서비스 알림 설정을 업데이트하지 못했습니다.",
    description: "Toast after notification policy update fails",
  },
  "services.notifyOnFailLabel": {
    message: "배포 실패 알림",
    description: "Settings tab: notifyOnFail row label",
  },
  "services.notifyOnFailHint": {
    message:
      "기본값은 각 멤버의 개별 알림 설정을 따르며, 이 서비스에 한해 강제로 켜거나 끌 수 있습니다.",
    description: "Settings tab: notifyOnFail row hint text",
  },
  "services.notifyOnFailOptionDefault": {
    message: "멤버 설정 사용",
    description: "notifyOnFail select option: default",
  },
  "services.notifyOnFailOptionNotify": {
    message: "항상 알림",
    description: "notifyOnFail select option: notify",
  },
  "services.notifyOnFailOptionIgnore": {
    message: "알리지 않음",
    description: "notifyOnFail select option: ignore",
  },
  "services.notifyOnFailSuccess": {
    message: "알림 설정을 업데이트했습니다.",
    description: "Toast after setNotifyOnFail succeeds",
  },
  "services.notifyOnFailError": {
    message: "알림 설정을 업데이트하지 못했습니다.",
    description: "Toast after setNotifyOnFail fails",
  },
  "services.planPickerTitle": {
    message: "컴퓨팅 플랜",
    description:
      "Accessible name of the Compute section's plan cards, shown after Edit (w6/m483)",
  },
  "services.computeTitle": {
    message: "컴퓨팅",
    description:
      "Compute page: title of the section holding the service's compute plan (Render: 'Under Compute, click Edit', w6/m483)",
  },
  "services.computePlanEdit": {
    message: "편집",
    description:
      "Compute section button that opens the compute plan cards (w6/m483)",
  },
  "services.planPickerFreeGroup": {
    message: "무료",
    description:
      "Plan-picker section label separating the Free tier from paid tiers",
  },
  "services.planPickerFreeSmtp": {
    message:
      "무료 서비스는 SMTP 포트 25, 465, 587로 이메일을 보낼 수 없습니다.",
    description:
      "Plan picker note under the Free group: free instances are refused outbound SMTP, as on Render (w8/m127)",
  },
  "services.planPickerPaidSmtp": {
    message:
      "유료 서비스는 STARTTLS를 사용하여 587 또는 2525 포트로 이메일을 보낼 수 있습니다. 25번과 465번 포트는 모든 요금제에서 사용할 수 없습니다.",
    description:
      "Plan picker note under the Paid group: outbound SMTP ports paid services may use (w8/m127)",
  },
  "services.planPickerLegacyFreeWorker": {
    message:
      "이 Background Worker는 Free 요금제를 사용 중이지만 Background Worker는 더 이상 Free를 사용할 수 없습니다. 계속 실행되며, 유료 컴퓨팅 플랜을 선택하면 Free에서 벗어나고 다시 돌아올 수 없습니다.",
    description:
      "Plan picker notice: this legacy service type is still on Free, which is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerUntieredWorker": {
    message:
      "이 Background Worker에는 컴퓨팅 플랜이 없고, Background Worker는 더 이상 Free를 사용할 수 없습니다. 계속 실행되며, 유료 컴퓨팅 플랜을 선택하여 지정하세요.",
    description:
      "Plan picker notice: this service type has no compute plan, and Free is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerLegacyFreePrivateService": {
    message:
      "이 Private Service는 Free 요금제를 사용 중이지만 Private Service는 더 이상 Free를 사용할 수 없습니다. 계속 실행되며, 유료 컴퓨팅 플랜을 선택하면 Free에서 벗어나고 다시 돌아올 수 없습니다.",
    description:
      "Plan picker notice: this legacy service type is still on Free, which is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerUntieredPrivateService": {
    message:
      "이 Private Service에는 컴퓨팅 플랜이 없고, Private Service는 더 이상 Free를 사용할 수 없습니다. 계속 실행되며, 유료 컴퓨팅 플랜을 선택하여 지정하세요.",
    description:
      "Plan picker notice: this service type has no compute plan, and Free is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerLegacyFreeCronJob": {
    message:
      "이 Cron Job은 Free 요금제를 사용 중이지만 Cron Job은 더 이상 Free를 사용할 수 없습니다. 계속 실행되며, 유료 컴퓨팅 플랜을 선택하면 Free에서 벗어나고 다시 돌아올 수 없습니다.",
    description:
      "Plan picker notice: this legacy service type is still on Free, which is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerUntieredCronJob": {
    message:
      "이 Cron Job에는 컴퓨팅 플랜이 없고, Cron Job은 더 이상 Free를 사용할 수 없습니다. 계속 실행되며, 유료 컴퓨팅 플랜을 선택하여 지정하세요.",
    description:
      "Plan picker notice: this service type has no compute plan, and Free is no longer offered to it (ADR030 §7)",
  },
  "services.planPickerPaidGroup": {
    message: "유료",
    description: "Plan-picker section label for the paid tier ladder",
  },
  "services.planCurrent": {
    message: "현재",
    description:
      "Plan picker: badge on the instance type the service is on now",
  },
  "services.planPickerCancel": {
    message: "취소",
    description: "Plan-picker footer button: discard the selection",
  },
  "services.planPickerSave": {
    message: "저장",
    description:
      'Compute section footer button: confirm the plan change ("Select a new compute plan and click Save", render.com/docs/compute-plans)',
  },
  "services.planPickerConfirmTitle": {
    message: "컴퓨팅 플랜을 변경하시겠습니까? 새 플랜: {name}",
    description: "Plan-change confirm dialog title",
  },
  "services.planPickerConfirmBody": {
    message:
      "서비스 크기가 조정되고 다운타임 없이 롤아웃됩니다. 진행 중인 요청은 이전 인스턴스가 교체되기 전에 완료됩니다.",
    description: "Plan-change confirm dialog body",
  },
  "services.instanceTypePausedReason": {
    message:
      '"{name}" 플랜은 일시적으로 사용할 수 없습니다. {largest} 또는 더 작은 컴퓨팅 플랜을 선택하세요.',
    description:
      "A paused compute plan's tooltip and screen-reader reason, composed in the dashboard's language from the tier and the largest still offered; matches bex-api's English unavailableReason (w4/552)",
  },
  "services.planPickerConfirmBodyCron": {
    message: "변경 이후에 시작되는 실행부터 새 컴퓨팅 플랜이 사용됩니다.",
    description:
      "Plan-change confirm body for a cron job, which serves no requests: its next run takes the new size (w4/548)",
  },
  "services.planPickerConfirmBodyNeverLive": {
    message:
      "서비스가 새 컴퓨팅 플랜으로 실행됩니다. 아직 서비스 중인 것이 없으며, 배포가 성공하면 라이브가 됩니다.",
    description:
      "Plan-change confirm body for a service that has never gone live (w4/548)",
  },
  "services.planPickerConfirmRunPrice": {
    message:
      "이 컴퓨팅 플랜은 실행 시간 1분당 ${price}이며, 실행되는 동안 초 단위로 청구됩니다. Cron Job당 최소 요금은 월 ${minimum}입니다.",
    description:
      "Plan-change confirm for a cron job: the new compute plan's run-time rate (w4/546); minimum is the cron job minimum in USD without the currency symbol (w9/m448)",
  },
  "services.createFieldPlanCronHint": {
    message:
      "Cron Job은 실행되는 동안 초 단위로 청구됩니다. Cron Job당 최소 요금은 월 ${minimum}입니다.",
    description:
      "Under the create form's compute plans for a cron job, which are priced per minute of run time (w4/546); minimum is the cron job minimum in USD without the currency symbol (w9/m448)",
  },
  "services.planPickerConfirmPrice": {
    message: "이 컴퓨팅 플랜은 월 ${price}입니다.",
    description:
      "Plan-change confirm: the new compute plan's monthly price, from the API's price sheet (w4/400)",
  },
  "services.planPickerSuccess": {
    message: "컴퓨팅 플랜을 업데이트했습니다: {name}",
    description: "Toast on a successful plan change",
  },
  "services.planPickerError": {
    message: "컴퓨팅 플랜을 업데이트하지 못했습니다. 다시 시도하세요.",
    description: "Toast on a failed plan change",
  },
  "services.planPickerErrorTitle": {
    message: "컴퓨팅 플랜을 불러오지 못했습니다",
    description: "Plan-picker error state title (instanceTypes query failed)",
  },
  "services.planPickerErrorBody": {
    message: "bex-api 요청이 실패했습니다. 연결을 확인한 후 다시 시도하세요.",
    description: "Plan-picker error state body",
  },
  "services.domainsTitle": {
    message: "사용자 지정 도메인",
    description: "Settings tab custom-domains section title",
  },
  "services.domainsDescription": {
    message: "소유한 사용자 지정 도메인을 이 서비스로 연결합니다.",
    description: "Settings tab custom-domains section description",
  },
  "services.domainColName": {
    message: "이름",
    description: "Custom-domains table column header (the FQDN)",
  },
  "services.domainColVerified": {
    message: "확인 상태",
    description: "Custom-domains table column header (DNS verification)",
  },
  "services.domainColCertificate": {
    message: "인증서 상태",
    description:
      "Custom-domains table column header (TLS certificate serving state)",
  },
  "services.domainColActions": {
    message: "작업",
    description:
      "Custom-domains table actions column header (screen-reader only)",
  },
  "services.domainVerified": {
    message: "확인됨",
    description:
      "Custom-domains status badge: DNS or TLS verification is complete",
  },
  "services.domainCertActive": {
    message: "활성",
    description:
      "Custom-domains status badge: certificate issued and serving traffic",
  },
  "services.domainCertBlockedLabel": {
    message: "인증서 차단됨:",
    description:
      "Label before cert-manager's verbatim reason a custom domain's TLS certificate has not been issued",
  },
  "services.domainCertBlockedHint": {
    message:
      "DNS 레코드는 DNS 전용(프록시 아님)이어야 하며 서비스의 bex 호스트 이름을 가리켜야 합니다. 그렇지 않으면 인증서 확인이 도달하지 못합니다.",
    description:
      "Fix-it guidance shown with a blocked-certificate reason: proxied records (e.g. Cloudflare's orange cloud) break the ACME HTTP-01 challenge",
  },
  "services.domainPending": {
    message: "대기 중",
    description:
      "Custom-domains status badge: DNS or certificate is not complete",
  },
  "services.domainActionsMenu": {
    message: "{domain} 작업",
    description: "Accessible label for the per-domain actions trigger",
  },
  "services.domainDelete": {
    message: "삭제",
    description: "Custom-domains row action: remove the domain",
  },
  "services.domainCancel": {
    message: "취소",
    description: "Custom-domains dialog cancel button",
  },
  "services.domainDeleteConfirmTitle": {
    message: "{name} 삭제",
    description: "Custom-domain delete-confirmation dialog title",
  },
  "services.domainDeleteConfirmBody": {
    message:
      "서비스가 이 도메인의 서비스를 중단합니다. Ingress 규칙이 제거되고 TLS 인증서는 만료되도록 둡니다. 이 작업은 되돌릴 수 없습니다.",
    description: "Custom-domain delete-confirmation dialog body",
  },
  "services.domainDeletePairConfirmBody": {
    message:
      "{name}과 자동으로 추가된 리디렉션 {sibling}이 함께 제거됩니다. 두 Ingress 규칙이 모두 제거되고 TLS 인증서는 만료되도록 둡니다. 이 작업은 되돌릴 수 없습니다.",
    description:
      "Custom-domain delete confirmation when deleting a canonical host also removes its generated redirecting sibling",
  },
  "services.domainDeleteGeneratedConfirmBody": {
    message:
      "{name} 도메인만 제거됩니다. {canonical} 도메인이 계속 정식 도메인으로 유지되며 리디렉션하지 않습니다. 제거된 도메인의 Ingress 규칙은 삭제되고 TLS 인증서는 만료되도록 둡니다. 이 작업은 되돌릴 수 없습니다.",
    description:
      "Custom-domain delete confirmation when deleting only the generated redirecting sibling",
  },
  "services.domainDeletePendingConfirmBody": {
    message:
      "{name} 도메인의 대기 중인 소유권 요청이 제거됩니다. 확인된 적이 없으므로 서비스가 제공된 적이 없고 인증서도 발급되지 않았습니다.",
    description:
      "Custom-domain delete confirmation for a domain still unverified: it was never routed (w4/142)",
  },
  "services.domainDeletePendingPairConfirmBody": {
    message:
      "{name} 및 {sibling} 도메인의 대기 중인 소유권 요청이 제거됩니다. 둘 다 확인된 적이 없으므로 서비스가 제공된 적이 없고 인증서도 발급되지 않았습니다.",
    description:
      "Custom-domain delete confirmation for a still-pending domain and its auto-paired sibling (w4/142)",
  },
  "services.domainDeletePendingGeneratedConfirmBody": {
    message:
      "{name} 도메인의 대기 중인 소유권 요청만 제거됩니다. {canonical} 도메인은 정식 도메인으로 유지됩니다. 확인된 적이 없으므로 서비스가 제공된 적이 없고 인증서도 발급되지 않았습니다.",
    description:
      "Custom-domain delete confirmation for a still-pending generated sibling: only it is removed, its canonical claim stays (w4/539)",
  },
  "services.domainAdd": {
    message: "사용자 지정 도메인 추가",
    description: "Custom-domains button to open the add-domain dialog",
  },
  "services.domainAddTitle": {
    message: "사용자 지정 도메인 추가",
    description: "Add-domain dialog title",
  },
  "services.domainAddDescription": {
    message:
      "소유한 도메인을 입력하세요. DNS를 이 서비스로 지정하면 bex가 TLS 인증서를 자동으로 발급합니다.",
    description: "Add-domain dialog description",
  },
  "services.domainPlaceholder": {
    message: "www.example.com",
    description: "Add-domain FQDN input placeholder",
  },
  "services.domainInvalid": {
    message: "www.example.com과 같은 올바른 도메인을 입력하세요.",
    description: "Add-domain validation message for a malformed hostname",
  },
  "services.domainAddButton": {
    message: "도메인 추가",
    description: "Add-domain dialog submit button",
  },
  "services.domainAddPairSuccess": {
    message: "{name} 및 {sibling} 추가됨",
    description:
      "Toast on a custom-domain add that auto-paired its www/apex sibling (w4/618)",
  },
  "services.domainAddSuccess": {
    message: "{name} 추가됨",
    description: "Toast on a successful custom-domain add",
  },
  "services.domainAddError": {
    message: "{name} 추가 실패",
    description: "Toast on a failed custom-domain add",
  },
  "services.domainAddConflict": {
    message: "다른 서비스에서 이미 사용 중인 도메인입니다: {name}",
    description:
      "Toast when a custom-domain add is rejected because the host is registered on another service (409)",
  },
  "services.domainAddReserved": {
    message: "플랫폼 예약 호스트 이름입니다: {name}",
    description:
      "Toast when a custom-domain add is rejected because the host is a platform-owned name (400)",
  },
  "services.domainAddOwnPlatformHost": {
    message:
      "{name} 도메인은 이 서비스의 플랫폼 서브도메인이므로 사용자 지정 도메인으로 추가할 수 없습니다",
    description:
      "Toast when a custom-domain add names the service's own platform subdomain (400 CUSTOM_DOMAIN_OWN_PLATFORM_HOST, w4/558)",
  },
  "services.domainDeletePairSuccess": {
    message: "{name} 및 {sibling} 제거됨",
    description:
      "Toast on a custom-domain delete that also removed its auto-paired sibling (w4/618)",
  },
  "services.domainDeleteSuccess": {
    message: "{name} 제거됨",
    description: "Toast on a successful custom-domain delete",
  },
  "services.domainDeleteError": {
    message: "{name} 제거 실패",
    description: "Toast on a failed custom-domain delete",
  },
  "services.domainPropagateNote": {
    message: "DNS와 TLS 인증서는 백그라운드에서 전파됩니다.",
    description:
      "Toast description after a custom-domain add (async convergence)",
  },
  "services.domainDnsToggle": {
    message: "{domain} DNS 설정 표시",
    description:
      "aria-label for the per-domain DNS-instructions disclosure toggle",
  },
  "services.domainDnsTitle": {
    message: "DNS 설정",
    description: "Heading of the per-domain DNS-instructions panel",
  },
  "services.domainDnsSubdomainGuidance": {
    message:
      "DNS 제공업체의 {zone} 영역에 이 레코드를 만드세요. 호스트는 해당 영역을 기준으로 한 상대 값입니다. 레코드가 확인되면 도메인 확인과 TLS 인증서 발급이 자동으로 진행됩니다.",
    description:
      "Guidance line above the DNS record for a subdomain — names the zone Host is relative to (w4/092)",
  },
  "services.domainDnsWildcardChallenge": {
    message:
      "와일드카드 도메인에는 이 레코드도 필요합니다. bex가 도메인의 TLS 인증서를 발급하고 갱신할 수 있게 합니다.",
    description:
      "Above a wildcard custom domain's second DNS record, its _acme-challenge CNAME (w4/m326; Render's docs say it enables certificate issuance and renewal)",
  },
  "services.domainDnsApexFallback": {
    message:
      "ALIAS/ANAME 또는 CNAME 플래트닝을 사용할 수 없나요? 대신 아래의 현재 엣지 주소 중 하나로 @에 대한 A 레코드를 만드세요. 이 주소는 바뀔 수 있지만 ALIAS/ANAME 레코드는 바뀌어도 계속 작동합니다.",
    description:
      "A-record fallback under an apex domain's DNS instructions, shown with the resolved edge addresses (w4/378)",
  },
  "services.domainDnsApexGuidance": {
    message:
      "루트(apex) 도메인에는 일반 CNAME을 사용할 수 없습니다. {zone} 영역에 이 ALIAS/ANAME(또는 CNAME 플래트닝) 레코드를 만드세요(@는 영역의 루트를 의미합니다). 제공업체가 둘 다 지원하지 않으면 대상이 가리키는 IP 주소로 @에 대한 A 레코드를 만드세요. 레코드가 확인되면 도메인 확인과 TLS가 자동으로 진행되며, bex가 www ↔ 루트 리디렉션도 설정해 줍니다.",
    description:
      "Guidance line above the DNS record for an apex domain — names the zone Host is relative to (w4/092)",
  },
  "services.domainRecordType": {
    message: "유형",
    description: "Label for the DNS record type field (CNAME/ALIAS)",
  },
  "services.domainRecordHost": {
    message: "호스트",
    description: "Label for the DNS record host/name field",
  },
  "services.domainRecordTarget": {
    message: "대상",
    description: "Label for the DNS record target/value field",
  },
  "services.domainCopyHost": {
    message: "{domain}의 {type} 호스트 복사",
    description: "Accessible label for copying a DNS host field",
  },
  "services.domainCopyTarget": {
    message: "{domain}의 {type} 대상 복사",
    description: "Accessible label for copying a DNS target/value field",
  },
  "services.domainDnsUnavailable": {
    message: "DNS 대상이 아직 없습니다. 서비스가 실행되면 다시 확인하세요.",
    description: "Shown when the backend couldn't derive the DNS record target",
  },
  "services.domainRecheck": {
    message: "다시 확인",
    description: "Button that re-checks a domain's DNS/certificate status",
  },
  "services.domainRecheckAria": {
    message: "{domain} 다시 확인",
    description:
      "Accessible name of a custom domain's DNS-panel Re-check button, naming the domain (w4/540)",
  },
  "services.domainCopied": {
    message: "클립보드에 복사했습니다",
    description: "Toast when a DNS record value is copied",
  },
  "services.domainCopyError": {
    message: "클립보드에 복사하지 못했습니다",
    description: "Toast when copying a DNS record value fails",
  },
  "services.domainAddedTitle": {
    message: "도메인 추가됨 — DNS 설정",
    description: "Title of the post-add DNS-record step in the add dialog",
  },
  "services.domainAddedDescription": {
    message:
      "DNS 제공업체에서 DNS 레코드를 만드세요. 레코드가 확인되면 도메인 확인이 자동으로 완료됩니다.",
    description: "Subtitle of the post-add DNS-record step in the add dialog",
  },
  "services.domainRedirectsTo": {
    message: "리디렉션 대상: {canonical}",
    description:
      "Note under an auto-paired sibling domain showing its canonical redirect target",
  },
  "services.domainRedirectsHere": {
    message: "{sibling} 도메인이 여기로 리디렉션됨",
    description:
      "Note under a canonical domain showing which auto-paired sibling redirects to it",
  },
  "services.domainPairedDnsTitle": {
    message:
      "{sibling} 도메인이 자동으로 추가되어 {canonical} 도메인으로 리디렉션됩니다. 이 도메인의 DNS도 설정하세요",
    description:
      "Heading of the second DNS-record block in the add dialog when the add auto-paired a www<->apex sibling (w6/m23)",
  },
  "services.domainDone": {
    message: "완료",
    description: "Button closing the post-add DNS-record step",
  },
  "services.domainVerifySuccess": {
    message: "{name} 확인됨",
    description: "Toast when a re-check finds the domain verified",
  },
  "services.domainVerifyPending": {
    message:
      "{name} 도메인이 아직 확인되지 않았습니다. DNS가 아직 전파 중일 수 있습니다.",
    description: "Toast when a re-check finds the domain still pending",
  },
  "services.domainVerifyError": {
    message: "{name} 도메인을 다시 확인하지 못했습니다.",
    description: "Toast when the re-check request fails",
  },
  "services.domainsEmptyTitle": {
    message: "사용자 지정 도메인 없음",
    description: "Custom-domains empty-state title",
  },
  "services.domainsEmptyBody": {
    message: "소유한 도메인을 추가하여 이 서비스를 해당 도메인으로 제공하세요.",
    description: "Custom-domains empty-state body",
  },
  "services.domainsErrorTitle": {
    message: "사용자 지정 도메인을 불러오지 못했습니다",
    description: "Custom-domains generic error title",
  },
  "services.domainsErrorBody": {
    message: "bex-api 요청이 실패했습니다. 연결을 확인한 후 다시 시도하세요.",
    description: "Custom-domains generic error body",
  },
  "services.platformSubdomainTitle": {
    message: "플랫폼 서브도메인",
    description: "Settings tab platform-subdomain section title",
  },
  "services.platformSubdomainDescription": {
    message:
      "설정한 사용자 지정 도메인과 함께 서비스가 bex 플랫폼 서브도메인에서도 응답할지 지정합니다.",
    description: "Settings tab platform-subdomain section description",
  },
  "services.platformSubdomainEnabled": {
    message: "사용",
    description: "Platform-subdomain toggle label when the subdomain is active",
  },
  "services.platformSubdomainDisabled": {
    message: "사용 안 함",
    description:
      "Platform-subdomain toggle label when the subdomain is disabled",
  },
  "services.platformSubdomainPending": {
    message: "서비스가 실행되면 플랫폼 URL이 할당됩니다.",
    description: "Platform-subdomain state when the service has no URL yet",
  },
  "services.platformSubdomainDisabledNote": {
    message:
      "플랫폼 서브도메인이 비활성화되어 있습니다. 서비스는 사용자 지정 도메인으로만 접근할 수 있습니다.",
    description:
      "Platform-subdomain note shown when the policy is set to disabled",
  },
  "services.platformSubdomainToggleLabel": {
    message: "플랫폼 서브도메인 전환",
    description: "Accessible label for the platform-subdomain Switch",
  },
  "services.platformSubdomainNeedsVerifiedDomain": {
    message:
      "서비스가 주소를 유지할 수 있도록 플랫폼 서브도메인을 끄기 전에 사용자 지정 도메인을 확인하세요.",
    description:
      "Hint beside the platform-subdomain switch when no custom domain is verified yet (w4/142)",
  },
  "services.platformSubdomainNeedsVerifiedDomainPending": {
    message:
      "플랫폼 서브도메인을 끄려면 먼저 {names} 확인이 완료되어야 합니다.",
    description:
      "Hint beside the platform-subdomain switch when custom domains exist but are all still pending verification (w4/142)",
  },
  "services.subdomainPolicySuccess": {
    message: "플랫폼 서브도메인 설정을 업데이트했습니다.",
    description: "Toast on successful setSubdomainPolicy mutation",
  },
  "services.subdomainPolicyError": {
    message:
      "플랫폼 서브도메인 설정을 업데이트하지 못했습니다. 다시 시도하세요.",
    description: "Toast on failed setSubdomainPolicy mutation",
  },
  "services.maintenanceModeTitle": {
    message: "유지 관리 모드",
    description: "Settings tab maintenance-mode section title",
  },
  "services.maintenanceModeDescription": {
    message:
      "서비스를 일시 중지하지 않고 유지 관리 페이지 뒤로 오프라인 상태로 전환합니다. 파드는 계속 실행되며 공개 트래픽만 리디렉션됩니다.",
    description: "Settings tab maintenance-mode section description",
  },
  "services.maintenanceModeEnabled": {
    message: "사용",
    description: "Maintenance-mode toggle label when active",
  },
  "services.maintenanceModeDisabled": {
    message: "사용 안 함",
    description: "Maintenance-mode toggle label when inactive",
  },
  "services.maintenanceModeSwitchHint": {
    message:
      "이 서비스가 제공하는 모든 호스트가 유지 관리 페이지로 응답합니다.",
    description: "Maintenance-mode section hint text next to the switch",
  },
  "services.maintenanceModePaidOnly": {
    message: "유지 관리 모드는 유료 Web Service 요금제에서 사용할 수 있습니다.",
    description: "Maintenance mode free-plan eligibility note",
  },
  "services.maintenanceModeToggleLabel": {
    message: "유지 관리 모드 전환",
    description: "Accessible label for the maintenance-mode Switch",
  },
  "services.maintenanceModeUriLabel": {
    message: "사용자 지정 유지 관리 페이지 URL",
    description: "Label for the maintenance-mode custom-page URL field",
  },
  "services.maintenanceModeUriPlaceholder": {
    message: "https://status.example.com/maintenance (선택 사항)",
    description: "Placeholder for the maintenance-mode custom-page URL field",
  },
  "services.maintenanceModeUriHint": {
    message:
      "기본 페이지 대신 이 페이지를 가져와 제공합니다. 비워 두면 bex의 기본 유지 관리 페이지를 사용합니다. 이 서비스 자체의 URL을 가리키면 안 됩니다.",
    description: "Hint text under the maintenance-mode custom-page URL field",
  },
  "services.maintenanceModeUriEdit": {
    message: "유지 관리 페이지 URL 편집",
    description:
      "Accessible label for the maintenance-page URL edit-pencil button",
  },
  "services.maintenanceModeEnableAction": {
    message: "유지 관리 모드 사용",
    description: "Confirm-dialog action button for enabling maintenance mode",
  },
  "services.confirmMaintenanceModeTitle": {
    message: "유지 관리 모드를 사용하시겠습니까?",
    description: "Confirm-dialog title for enabling maintenance mode",
  },
  "services.confirmMaintenanceModeBody": {
    message:
      "비활성화할 때까지 {name} 서비스의 모든 방문자에게 유지 관리 페이지가 표시됩니다. 서비스의 파드는 계속 실행됩니다.",
    description: "Confirm-dialog body for enabling maintenance mode",
  },
  "services.maintenanceModeEnabledSuccess": {
    message: "유지 관리 모드를 사용 설정했습니다.",
    description: "Toast on successfully enabling maintenance mode",
  },
  "services.maintenanceModeDisabledSuccess": {
    message: "유지 관리 모드를 해제했습니다.",
    description: "Toast on successfully disabling maintenance mode",
  },
  "services.maintenancePageSavedSuccess": {
    message: "유지 관리 페이지를 저장했습니다.",
    description: "Toast on saving only the custom maintenance page URL",
  },
  "services.maintenancePageClearedSuccess": {
    message: "기본 유지 관리 페이지로 복원했습니다.",
    description: "Toast on clearing the custom maintenance page URL",
  },
  "services.maintenanceModeError": {
    message: "유지 관리 모드를 업데이트하지 못했습니다. 다시 시도하세요.",
    description: "Toast on failed setMaintenanceMode mutation",
  },
  "services.edgeCachingTitle": {
    message: "엣지 캐싱",
    description: "Settings tab edge-caching section title (Render parity)",
  },
  "services.edgeCachingDescription": {
    message:
      "엣지에서 응답을 캐시하여 반복 요청이 이 서비스에 도달하지 않고 처리되게 합니다.",
    description: "Settings tab edge-caching section description",
  },
  "services.edgeCachingFileTypesLabel": {
    message: "캐시 가능한 파일 유형",
    description: "Edge-caching radio group label",
  },
  "services.edgeCachingOptionNone": {
    message: "없음",
    description: "Edge-caching option: no-cache profile",
  },
  "services.edgeCachingOptionNoneDesc": {
    message: "모든 응답을 서비스에서 제공합니다.",
    description: "Edge-caching no-cache option description",
  },
  "services.edgeCachingOptionStatic": {
    message: "일반 정적 파일",
    description: "Edge-caching option: origin-controlled profile",
  },
  "services.edgeCachingOptionStaticDesc": {
    message:
      "응답의 Cache-Control 헤더에서 달리 지정하지 않는 한 CSS, JavaScript, 이미지, 글꼴 같은 일반 정적 파일 유형을 캐시합니다.",
    description: "Edge-caching origin-controlled option description",
  },
  "services.edgeCachingOptionAll": {
    message: "모든 파일",
    description: "Edge-caching option: origin-controlled-all profile",
  },
  "services.edgeCachingOptionAllDesc": {
    message:
      "Cache-Control 헤더에서 달리 지정하지 않는 한 캐시할 수 있는 모든 응답을 캐시합니다. 동적 콘텐츠가 캐시되지 않도록 Cache-Control을 설정해야 합니다.",
    description:
      "Edge-caching origin-controlled-all option description, with the dynamic-content warning",
  },
  "services.edgeCachingPurgeOnDeploy": {
    message: "캐시는 배포가 성공할 때마다 비워집니다.",
    description: "Note under the edge-caching options",
  },
  "services.edgeCachingPaidOnly": {
    message: "엣지 캐싱은 무료 Web Service에서 사용할 수 없습니다.",
    description: "Edge-caching free-plan eligibility note",
  },
  "services.edgeCachingSave": {
    message: "엣지 캐싱 저장",
    description: "Edge-caching save button",
  },
  "services.edgeCachingSaving": {
    message: "저장하는 중…",
    description: "Edge-caching save button while the mutation is in flight",
  },
  "services.edgeCachingSaved": {
    message: "엣지 캐싱을 업데이트했습니다.",
    description: "Toast on successful setCacheProfile mutation",
  },
  "services.edgeCachingError": {
    message: "엣지 캐싱을 업데이트하지 못했습니다. 다시 시도하세요.",
    description: "Toast on failed setCacheProfile mutation",
  },
  "services.edgeCachingPurgeAction": {
    message: "캐시 비우기",
    description: "Edge-caching purge button and confirm action",
  },
  "services.edgeCachingPurgeConfirmTitle": {
    message: "엣지 캐시를 비우시겠습니까?",
    description: "Confirm-dialog title for purging the edge cache",
  },
  "services.edgeCachingPurgeConfirmBody": {
    message:
      "캐시된 모든 응답이 삭제됩니다. 캐시가 다시 채워질 때까지 요청은 서비스로 전달됩니다.",
    description: "Confirm-dialog body for purging the edge cache",
  },
  "services.edgeCachingPurged": {
    message: "캐시 비우기를 시작했습니다.",
    description: "Toast on successful purgeServiceCache mutation",
  },
  "services.edgeCachingPurgeError": {
    message: "캐시를 비우지 못했습니다. 다시 시도하세요.",
    description: "Toast on failed purgeServiceCache mutation",
  },
  "services.runtimeTitle": {
    message: "런타임",
    description: "Settings tab runtime section title (Render parity)",
  },
  "services.runtimeDescription": {
    message: "bex가 이 서비스를 빌드하고 실행하는 환경입니다.",
    description: "Settings tab runtime section description",
  },
  "services.runtimeLabel": {
    message: "런타임",
    description: "Settings runtime select label",
  },
  "services.runtimeNote": {
    message:
      "런타임을 변경하면 서비스가 다시 빌드되고 재배포됩니다. 네이티브 런타임은 아래의 빌드 명령과 시작 명령을 사용합니다.",
    description: "Note beside the Settings runtime select",
  },
  "services.runtimePlaceholder": {
    message: "런타임 선택",
    description:
      "Settings runtime select placeholder for a legacy service with no runtime recorded",
  },
  "services.runtimeSave": {
    message: "런타임 저장",
    description: "Settings runtime save button",
  },
  "services.runtimeSaving": {
    message: "저장하는 중…",
    description: "Settings runtime save button while the mutation is in flight",
  },
  "services.runtimeConfirmTitle": {
    message: "런타임을 변경하시겠습니까? 새 런타임: {runtime}",
    description: "Confirm-dialog title for changing a service's runtime",
  },
  "services.runtimeConfirmBody": {
    message:
      "서비스가 {runtime} 런타임으로 다시 빌드되고 재배포됩니다. 새 배포가 시작됩니다.",
    description: "Confirm-dialog body for changing a service's runtime",
  },
  "services.runtimeConfirmAction": {
    message: "런타임 변경",
    description: "Confirm-dialog action for changing a service's runtime",
  },
  "services.runtimeSaved": {
    message: "런타임을 업데이트했습니다. 새 배포가 시작되었습니다.",
    description: "Toast on successful setRuntime mutation",
  },
  "services.runtimeError": {
    message: "런타임을 변경하지 못했습니다. 다시 시도하세요.",
    description: "Toast on failed setRuntime mutation without a server message",
  },
  "services.maintenanceModeBannerTitle": {
    message: "유지 관리 모드가 켜져 있습니다",
    description: "Service-detail header banner title while in maintenance",
  },
  "services.maintenanceModeBannerBody": {
    message:
      "방문자에게 이 서비스 대신 유지 관리 페이지가 표시됩니다. 파드는 계속 실행 중이며, 정상 서비스를 재개하려면 설정에서 유지 관리 모드를 해제하세요.",
    description: "Service-detail header banner body while in maintenance",
  },
  "services.deployTitle": {
    message: "배포",
    description: "Cron job Settings tab: Deploy section title (Render parity)",
  },
  "services.deployDescription": {
    message: "이 Cron Job이 실행되는 방식입니다.",
    description: "Cron job Settings tab: Deploy section description",
  },
  "services.deploySuccess": {
    message: "Cron Job 설정을 저장했습니다.",
    description: "Toast after updateCronJob succeeds",
  },
  "services.cronScheduleSaved": {
    message: "다음 실행부터 새 스케줄이 적용됩니다.",
    description:
      "Toast description after saving a cron job's schedule alone: the next run follows it, nothing deploys (w4/593)",
  },
  "services.cronCommandSaved": {
    message:
      "새 명령을 적용하는 배포가 진행 중이며, 이후 실행부터 새 명령이 사용됩니다.",
    description:
      "Toast description after saving a cron job's command: the save rolls a deploy that applies it (w4/593)",
  },
  "services.deployError": {
    message: "Cron Job 설정을 저장하지 못했습니다. 다시 시도하세요.",
    description: "Toast after updateCronJob fails",
  },
  "services.deployRefreshFailed": {
    message:
      "Cron Job 설정이 저장되었지만 이 페이지를 새로 고치지 못했습니다. 다시 시도하거나 페이지를 새로고침하여 저장된 설정을 확인하세요.",
    description:
      "Warning after a cron save committed but its authoritative re-read failed; the save must not be retried or reported as failed (w4/276)",
  },
  "services.deployRefreshPending": {
    message:
      "설정이 저장되었습니다. 추가로 편집하기 전에 최신 정보를 불러오는 중입니다.",
    description:
      "Reason shown while cron rows stay unwritable after a committed save whose refresh failed (w4/276)",
  },
  "services.deployScheduleLabel": {
    message: "스케줄",
    description: "Cron job Settings tab: Deploy section schedule field label",
  },
  "services.deployScheduleHint": {
    message:
      "이 스케줄에 따라 이 명령을 실행합니다(5개 필드 crontab, UTC 기준).",
    description: "Cron job Settings tab: Deploy section schedule help text",
  },
  "services.deploySchedulePreview": {
    message: "{description} · UTC 기준 실행",
    description:
      "Cron job Settings tab: live human-readable description of the schedule",
  },
  "services.deploySchedulePlaceholder": {
    message: "0 * * * *",
    description: "Cron job Deploy section: schedule input placeholder",
  },
  "services.deployScheduleEdit": {
    message: "스케줄 편집",
    description: "Accessible label for the cron schedule edit button",
  },
  "services.deployCommandEdit": {
    message: "명령 편집",
    description: "Accessible label for the cron command edit button",
  },
  "services.cronScheduleNeverFires": {
    message:
      "이 스케줄은 실행되지 않습니다. 일과 월이 일치하는 날짜가 없습니다(예: 31 2는 2월 31일).",
    description:
      "Cron schedule validation error when the schedule parses but no date ever matches (w4/197)",
  },
  "services.deployScheduleError": {
    message: "0 * * * *와 같은 올바른 5개 필드 cron 표현식을 입력하세요.",
    description: "Cron job Deploy section: schedule validation error",
  },
  "services.deployScheduleRequired": {
    message: "스케줄은 필수입니다.",
    description: "Cron job Deploy section: schedule required validation error",
  },
  "services.deployCommandLabel": {
    message: "명령",
    description: "Cron job Settings tab: Deploy section command field label",
  },
  "services.deployCommandPlaceholder": {
    message: "예: python script.py",
    description: "Cron job Deploy section: command input placeholder",
  },
  "services.deployCommandHint": {
    message:
      "이미지의 기본 진입점을 재정의합니다. 비워 두면 이미지 자체의 명령을 실행합니다.",
    description: "Cron job Deploy section: command field help text",
  },
  "services.deployCommandNativeHint": {
    message:
      "각 스케줄마다 작업이 실행하는 명령이며, 해당 런타임의 시작 명령으로 사용됩니다.",
    description:
      "Cron job Deploy section: command field help text for a native-runtime cron, whose command is its start command (w8/m69)",
  },
  "services.deployCommandRequired": {
    message: "명령은 필수입니다.",
    description:
      "Cron job Deploy section: command required validation error for a native-runtime cron (w8/m69)",
  },
  "services.buildTitle": {
    message: "빌드",
    description:
      "Settings tab: Build card title, Render's split Build section (w5/m52)",
  },
  "services.buildDeployNav": {
    message: "빌드 및 배포",
    description:
      "Settings section index: the one entry for the Build and Deploy cards, which the page renders as one section (Render names it Build & Deploy)",
  },
  "services.buildDescription": {
    message: "bex가 저장소에서 서비스를 빌드하는 방식을 구성합니다.",
    description: "Settings tab: Build card description",
  },
  "services.deploySectionTitle": {
    message: "배포",
    description:
      "Settings tab: Deploy card title, Render's split Deploy section (w5/m52)",
  },
  "services.deploySectionDescription": {
    message: "bex가 서비스를 배포하고 실행하는 방식을 구성합니다.",
    description: "Settings tab: Deploy card description",
  },
  "services.buildDeploySourceLabel": {
    message: "소스",
    description: "Build & Deploy: repo field label",
  },
  "services.buildDeploySourceHint": {
    message:
      "이 서비스를 빌드하는 Git 저장소입니다. 연결된 저장소 중 하나를 선택하거나 저장소 URL을 붙여 넣으세요.",
    description: "Build & Deploy: Source (repo) field help text (w5/m54)",
  },
  "services.buildDeploySourcePlaceholder": {
    message: "https://github.com/owner/repo",
    description: "Build & Deploy: Source combobox input placeholder",
  },
  "services.buildDeploySourceEdit": {
    message: "소스 편집",
    description: "Accessible label for the Source edit-pencil button",
  },
  "services.buildDeploySourceConfirmTitle": {
    message: "소스를 전환하시겠습니까? 새 소스: {value}",
    description: "Build & Deploy: Source-change confirm dialog title",
  },
  "services.buildDeployBranchLabel": {
    message: "브랜치",
    description: "Build & Deploy: branch field label",
  },
  "services.buildDeployBranchHint": {
    message: "빌드하고 배포할 Git 브랜치입니다.",
    description: "Build & Deploy: branch field hint (Render's copy)",
  },
  "services.buildDeployBranchEmpty": {
    message: "main",
    description:
      "Build & Deploy: branch value shown when unset (the backend default)",
  },
  "services.buildDeployBranchPlaceholder": {
    message: "main",
    description: "Build & Deploy: branch input placeholder",
  },
  "services.buildDeployBranchEdit": {
    message: "브랜치 편집",
    description: "Build & Deploy: branch edit-button aria label",
  },
  "services.buildDeployBranchConfirmTitle": {
    message: '브랜치를 변경하시겠습니까? 새 브랜치: "{value}"',
    description: "Build & Deploy: branch confirm-dialog title",
  },
  "services.buildDeployRootDirLabel": {
    message: "루트 디렉터리",
    description: "Build & Deploy: root-directory field label",
  },
  "services.buildDeployRootDirOptional": {
    message: "선택 사항",
    description:
      "Build & Deploy: badge next to the Root Directory label (Render parity)",
  },
  "services.buildDeployRootDirHint": {
    message:
      "설정하면 저장소 루트 대신 이 하위 디렉터리에서 빌드가 실행됩니다. 이 디렉터리 밖의 코드 변경은 빌드 필터의 포함 경로와 일치하지 않는 한 자동 배포를 트리거하지 않습니다. 모노레포에서 가장 많이 사용합니다.",
    description: "Build & Deploy: root-directory field help text",
  },
  "services.buildDeployConfirmRoot": {
    message: "저장소 루트",
    description:
      "Build & Deploy: mid-sentence phrase for the confirm dialog title when clearing rootDir to empty (a dedicated key, not a lowercased buildDeployRootDirEmpty, since that transform doesn't hold in every language)",
  },
  "services.buildDeployRootDirPlaceholder": {
    message: "예: backend",
    description: "Build & Deploy: root-directory input placeholder",
  },
  "services.buildDeployEdit": {
    message: "루트 디렉터리 편집",
    description: "Build & Deploy: accessible label for the edit-pencil button",
  },
  "services.buildDeployCancel": {
    message: "취소",
    description: "Build & Deploy: root-directory inline-edit cancel button",
  },
  "services.buildDeployConfirmTitle": {
    message: "루트 디렉터리를 변경하시겠습니까? 새 디렉터리: {value}",
    description: "Build & Deploy: root-directory change confirm dialog title",
  },
  "services.buildDeployConfirmBody": {
    message:
      "서비스가 새 디렉터리 기준으로 다시 빌드되고 재배포됩니다. 진행 중인 요청은 이전 인스턴스가 교체되기 전에 완료됩니다.",
    description: "Build & Deploy: root-directory change confirm dialog body",
  },
  "services.buildDeployConfirmBodyStatic": {
    message:
      "새 디렉터리에서 다시 빌드가 시작됩니다. 새 빌드가 성공적으로 게시될 때까지 사이트는 현재 게시된 버전을 계속 제공합니다.",
    description:
      "Build & Deploy: a live static site's root-directory change confirm body (atomic publish, no instances; w4/548)",
  },
  "services.buildDeployConfirmBodyNeverLive": {
    message:
      "서비스가 새 디렉터리에서 빌드되고 배포됩니다. 아직 서비스 중인 것이 없으며, 배포가 성공하면 라이브가 됩니다.",
    description:
      "Build & Deploy: root-directory change confirm body for a service that has never gone live, so no current version keeps serving (w4/548)",
  },
  "services.buildDeploySuccess": {
    message: "루트 디렉터리를 업데이트했습니다.",
    description: "Toast after setRootDir succeeds",
  },
  "services.buildDeployError": {
    message: "루트 디렉터리를 업데이트하지 못했습니다. 다시 시도하세요.",
    description: "Toast after setRootDir fails",
  },
  "services.sourceUpdateSuccess": {
    message: "소스를 업데이트했습니다.",
    description:
      "Toast after a repository, branch, or image source update succeeds",
  },
  "services.sourceUpdateError": {
    message: "소스를 업데이트하지 못했습니다. 다시 시도하세요.",
    description:
      "Fallback toast when a source update fails without a server reason",
  },
  "services.sourceTitle": {
    message: "소스",
    description: "Source card title",
  },
  "services.sourceRepoDescription": {
    message: "이 서비스를 빌드하는 Git 저장소와 브랜치입니다.",
    description: "Source card description for a repository-backed service",
  },
  "services.sourceImageDescription": {
    message: "이 서비스가 배포하는 사전 빌드된 컨테이너 이미지입니다.",
    description: "Source card description for an image-backed service",
  },
  "services.sourceImageLabel": {
    message: "이미지",
    description: "Source card: container image path field label",
  },
  "services.sourceNoAutoDeploy": {
    message:
      "변경 사항은 자동으로 배포되지 않으며, 다음 배포부터 새 소스가 사용됩니다.",
    description:
      "Shared note (Render parity): a source change doesn't trigger a deploy",
  },
  "services.sourceMissing": {
    message: "구성되지 않음",
    description: "Source card fallback when no source value is available",
  },
  "services.sourceRepoAccessLostTitle": {
    message: "bex의 GitHub 앱이 이 저장소를 읽을 수 없습니다",
    description:
      "Settings → Source warning title: the GitHub App connected this service's repository and can no longer read it (repoAccessLost, w10/m490)",
  },
  "services.sourceRepoAccessLostBody": {
    message:
      "푸시가 더 이상 이 서비스를 배포하지 않습니다. 저장소가 다른 GitHub 계정으로 옮겨졌다면 그 계정에 GitHub 앱을 설치하세요. 그렇지 않다면 앱에 저장소 접근 권한을 다시 부여하세요.",
    description:
      "Settings → Source warning body: pushes stop reaching bex once the GitHub App loses the repository (moved to an account without the app, or the grant was revoked); what to do (w10/m490)",
  },
  "services.sourceEdit": {
    message: "편집",
    description: "Button opening the Update Source dialog",
  },
  "services.sourceDialogTitle": {
    message: "소스 업데이트",
    description: "Update Source dialog title",
  },
  "services.sourcePickerLabel": {
    message: "소스 선택",
    description: "Label above the source kind picker in Update Source",
  },
  "services.sourceBranchHint": {
    message: "브랜치를 선택하거나 목록에 없는 브랜치를 직접 입력하세요.",
    description: "Branch picker free-text fallback hint",
  },
  "services.sourceBranchEmpty": {
    message: "일치하는 브랜치가 없습니다. 브랜치 이름을 입력하세요.",
    description: "Empty state for the Update Source branch picker",
  },
  "services.sourceTabBuildSource": {
    message: "빌드 소스",
    description: "Update Source tab: build the service from a build source",
  },
  "services.sourceBuildSourceDescription": {
    message:
      "이 서비스가 배포하는 빌드 소스입니다. 이 빌드 소스에 연결된 모든 서비스가 빌드를 공유합니다.",
    description:
      "Source card description for a service linked to a build source",
  },
  "services.sourceBuildSourceLabel": {
    message: "빌드 소스",
    description: "Source card and Update Source: build source field label",
  },
  "services.sourceBuildSourceBuildsFrom": {
    message: "빌드 원본",
    description:
      "Source card: the repository or image the linked build source builds from",
  },
  "services.sourceBuildSourceEmpty": {
    message:
      "이 워크스페이스에는 아직 빌드 소스가 없습니다. API 또는 MCP로 빌드 소스를 만든 다음 여기서 서비스를 연결하세요.",
    description: "Update Source build source tab: empty state",
  },
  "services.sourceBuildSourceLoadError": {
    message:
      "이 워크스페이스의 빌드 소스를 불러오지 못했습니다. 대화 상자를 닫고 다시 시도하세요.",
    description: "Update Source build source tab: the list failed to load",
  },
  "services.sourceBuildSourcePlaceholder": {
    message: "빌드 소스 선택",
    description: "Update Source build source picker placeholder",
  },
  "services.sourceBuildSourceStartHint": {
    message:
      "이 빌드 소스는 네이티브 런타임을 사용하므로 서비스에 자체 시작 명령이 필요합니다.",
    description:
      "Update Source: why a native-runtime build source needs a start command",
  },
  "services.sourceNativeCommandsHint": {
    message:
      "이 서비스는 네이티브 런타임으로 빌드하므로 Git으로 되돌리려면 빌드 명령과 시작 명령이 필요합니다.",
    description:
      "Update Source: why switching an image service back to a native Git build needs its build and start commands",
  },
  "services.sourceUpdateSave": {
    message: "소스 업데이트",
    description: "Save button in the Update Source dialog",
  },
  "services.startCommandLabel": {
    message: "시작 명령",
    description: "Build & Deploy: native-runtime start-command label",
  },
  "services.dockerCommandLabel": {
    message: "Docker 명령",
    description: "Build & Deploy: Docker CMD override label (Render wording)",
  },
  "services.startCommandHint": {
    message: "빌드가 성공한 후 이 서비스를 시작하는 명령입니다.",
    description: "Build & Deploy: native start-command help text",
  },
  "services.dockerCommandHint": {
    message:
      "Dockerfile의 CMD를 재정의합니다. 비워 두면 이미지의 기본 명령을 사용합니다.",
    description: "Build & Deploy: Docker Command help text",
  },
  "services.startCommandConfirmEmpty": {
    message: "기본 명령",
    description: "Build & Deploy: empty command phrase in confirmation title",
  },
  "services.startCommandPlaceholder": {
    message: "예: npm start",
    description: "Build & Deploy: start-command input placeholder",
  },
  "services.startCommandEdit": {
    message: "시작 명령 편집",
    description: "Build & Deploy: accessible command edit button label",
  },
  "services.dockerCommandEdit": {
    message: "Docker 명령 편집",
    description: "Build & Deploy: accessible Docker Command edit button label",
  },
  "services.dockerCommandPlaceholder": {
    message: "예: bin/server",
    description: "Build & Deploy: Docker Command input placeholder",
  },
  "services.startCommandConfirmTitle": {
    message: "시작 명령을 변경하시겠습니까? 새 명령: {value}",
    description: "Build & Deploy: command-change confirmation title",
  },
  "services.dockerCommandConfirmTitle": {
    message: "Docker 명령을 변경하시겠습니까? 새 명령: {value}",
    description: "Build & Deploy: Docker Command confirmation title",
  },
  "services.startCommandConfirmBody": {
    message:
      "서비스가 새 명령으로 재배포됩니다. 진행 중인 요청은 이전 인스턴스가 교체되기 전에 완료됩니다.",
    description: "Build & Deploy: command-change confirmation body",
  },
  "services.startCommandConfirmBodyNeverLive": {
    message:
      "서비스가 새 명령으로 배포됩니다. 아직 서비스 중인 것이 없으며, 배포가 성공하면 라이브가 됩니다.",
    description:
      "Deploy: command change confirm body for a service that has never gone live (w4/548)",
  },
  "services.startCommandSuccess": {
    message: "시작 명령을 업데이트했습니다.",
    description: "Toast after setStartCommand succeeds",
  },
  "services.startCommandError": {
    message: "시작 명령을 업데이트하지 못했습니다. 다시 시도하세요.",
    description: "Toast after setStartCommand fails",
  },
  "services.dockerCommandSuccess": {
    message: "Docker 명령을 업데이트했습니다.",
    description:
      "Toast after a Dockerfile or Existing Image service's Docker Command is saved",
  },
  "services.dockerCommandError": {
    message: "Docker 명령을 업데이트하지 못했습니다. 다시 시도하세요.",
    description:
      "Toast when saving a Dockerfile or Existing Image service's Docker Command fails",
  },
  "services.buildCommandLabel": {
    message: "빌드 명령",
    description:
      "Build & Deploy: build-command field label (all service types' settings + the New Service form)",
  },
  "services.buildCommandHint": {
    message:
      "서비스를 빌드하는 명령입니다(예: npm run build). 비워 두면 런타임 기본값을 사용합니다.",
    description:
      "Build & Deploy: build-command help text — shown for every service type, so it must not assume static-site output",
  },
  "services.buildCommandConfirmEmpty": {
    message: "런타임 기본값",
    description:
      "Build & Deploy: empty build-command phrase in confirmation title",
  },
  "services.buildCommandPlaceholder": {
    message: "npm run build",
    description: "Build & Deploy: build-command input placeholder",
  },
  "services.buildCommandEdit": {
    message: "빌드 명령 편집",
    description: "Build & Deploy: accessible build-command edit button label",
  },
  "services.buildCommandConfirmTitle": {
    message: "빌드 명령을 변경하시겠습니까? 새 명령: {value}",
    description: "Build & Deploy: build-command change confirmation title",
  },
  "services.buildCommandConfirmBody": {
    message:
      "서비스가 새 빌드 명령으로 재배포됩니다. 기존 요청은 이전 인스턴스가 교체되기 전에 모두 처리됩니다.",
    description: "Build & Deploy: build-command change confirmation body",
  },
  "services.buildCommandConfirmBodyNeverLive": {
    message:
      "서비스가 새 빌드 명령으로 빌드되고 배포됩니다. 아직 서비스 중인 것이 없으며, 배포가 성공하면 라이브가 됩니다.",
    description:
      "Build & Deploy: build-command change confirm body for a service that has never gone live (w4/548)",
  },
  "services.buildCommandConfirmBodyStatic": {
    message:
      "새 빌드 명령으로 다시 빌드가 시작됩니다. 새 빌드가 성공적으로 게시될 때까지 사이트는 현재 게시된 버전을 계속 제공합니다.",
    description:
      "Build & Deploy: static-site build-command change confirmation body (atomic publish, no instances)",
  },
  "services.staticBuildConfirmBodyNeverLive": {
    message:
      "이 변경 사항으로 빌드가 시작됩니다. 사이트는 아직 게시되지 않았으며, 이 빌드가 성공하면 라이브가 됩니다.",
    description:
      "Build & Deploy: a static site's build-command or root-directory change confirm body when the site has never been published (w4/548)",
  },
  "services.buildCommandSuccess": {
    message: "빌드 명령을 업데이트했습니다.",
    description: "Toast after setBuildCommand succeeds",
  },
  "services.buildCommandError": {
    message: "빌드 명령을 업데이트하지 못했습니다. 다시 시도하세요.",
    description: "Toast after setBuildCommand fails",
  },
  "services.dockerfilePathLabel": {
    message: "Dockerfile 경로",
    description: "Build & Deploy: Dockerfile-path field label",
  },
  "services.dockerfilePathHint": {
    message:
      "루트 디렉터리를 기준으로 한 Dockerfile 경로입니다. 비워 두면 Dockerfile을 사용합니다.",
    description: "Build & Deploy: Dockerfile-path help text",
  },
  "services.dockerfilePathConfirmEmpty": {
    message: "기본 Dockerfile",
    description: "Build & Deploy: empty Dockerfile-path confirmation phrase",
  },
  "services.dockerfilePathPlaceholder": {
    message: "예: docker/Dockerfile.prod",
    description: "Build & Deploy: Dockerfile-path input placeholder",
  },
  "services.dockerfilePathEdit": {
    message: "Dockerfile 경로 편집",
    description: "Build & Deploy: accessible Dockerfile-path edit label",
  },
  "services.dockerfilePathConfirmTitle": {
    message: "Dockerfile 경로를 변경하시겠습니까? 새 경로: {value}",
    description: "Build & Deploy: Dockerfile-path confirmation title",
  },
  "services.dockerfilePathConfirmBody": {
    message:
      "서비스가 선택한 Dockerfile로 다시 빌드되고, 생성된 이미지가 배포됩니다.",
    description: "Build & Deploy: Dockerfile-path confirmation body",
  },
  "services.dockerfilePathSuccess": {
    message: "Dockerfile 경로를 업데이트했습니다.",
    description: "Toast after setDockerfilePath succeeds",
  },
  "services.dockerfilePathError": {
    message: "Dockerfile 경로를 업데이트하지 못했습니다. 다시 시도하세요.",
    description: "Toast after setDockerfilePath fails",
  },
  "services.dockerContextLabel": {
    message: "Docker 빌드 컨텍스트 디렉터리",
    description: "Build & Deploy: Docker build context field label (w8/m128)",
  },
  "services.dockerContextHint": {
    message:
      "루트 디렉터리를 기준으로 한 빌드 컨텍스트입니다. 비워 두면 루트 디렉터리를 사용합니다.",
    description: "Build & Deploy: Docker build context help text",
  },
  "services.dockerContextConfirmEmpty": {
    message: "루트 디렉터리",
    description:
      "Build & Deploy: empty Docker build context confirmation phrase",
  },
  "services.dockerContextPlaceholder": {
    message: "예: .",
    description: "Build & Deploy: Docker build context input placeholder",
  },
  "services.dockerContextEdit": {
    message: "Docker 빌드 컨텍스트 디렉터리 편집",
    description: "Build & Deploy: accessible Docker build context edit label",
  },
  "services.dockerContextConfirmTitle": {
    message:
      "Docker 빌드 컨텍스트 디렉터리를 변경하시겠습니까? 새 디렉터리: {value}",
    description: "Build & Deploy: Docker build context confirmation title",
  },
  "services.dockerContextConfirmBody": {
    message:
      "서비스가 새 빌드 컨텍스트로 다시 빌드되고, 생성된 이미지가 배포됩니다.",
    description: "Build & Deploy: Docker build context confirmation body",
  },
  "services.dockerContextSuccess": {
    message: "Docker 빌드 컨텍스트 디렉터리를 업데이트했습니다.",
    description: "Toast after setDockerContext succeeds",
  },
  "services.dockerContextError": {
    message:
      "Docker 빌드 컨텍스트 디렉터리를 업데이트하지 못했습니다. 다시 시도하세요.",
    description: "Toast after setDockerContext fails",
  },
  "services.buildFilterLabel": {
    message: "빌드 필터",
    description: "Build & Deploy: label for the build-filters editor",
  },
  "services.buildFilterHint": {
    message:
      "git push로 일치하는 파일이 변경될 때만 배포를 트리거합니다. 경로는 루트 디렉터리를 설정해도 저장소 루트를 기준으로 한 glob 패턴입니다(예: src/**, **/*.md).",
    description: "Build & Deploy: help text for the build-filters editor",
  },
  "services.buildFilterIncludedTitle": {
    message: "포함 경로",
    description: "Build & Deploy: title for the included-paths list",
  },
  "services.buildFilterIncludedHint": {
    message:
      "변경된 파일이 이 중 하나와 일치할 때만 push로 배포되며, 루트 디렉터리 밖의 파일도 마찬가지입니다. 비워 두면 루트 디렉터리 안의 모든 경로가 포함됩니다.",
    description: "Build & Deploy: help text for the included-paths list",
  },
  "services.buildFilterIncludedPlaceholder": {
    message: "예: src/**",
    description: "Build & Deploy: placeholder for an included-path input",
  },
  "services.buildFilterAddIncluded": {
    message: "포함 경로 추가",
    description: "Build & Deploy: add-row button for the included-paths list",
  },
  "services.buildFilterRemoveIncluded": {
    message: "포함 경로 제거",
    description: "Build & Deploy: remove-row label for an included path",
  },
  "services.buildFilterIgnoredTitle": {
    message: "무시 경로",
    description: "Build & Deploy: title for the ignored-paths list",
  },
  "services.buildFilterIgnoredHint": {
    message:
      "변경된 파일이 이 중 하나와 일치하면 포함 경로와도 일치하더라도 배포가 트리거되지 않습니다.",
    description: "Build & Deploy: help text for the ignored-paths list",
  },
  "services.buildFilterIgnoredPlaceholder": {
    message: "예: docs/**",
    description: "Build & Deploy: placeholder for an ignored-path input",
  },
  "services.buildFilterAddIgnored": {
    message: "무시 경로 추가",
    description: "Build & Deploy: add-row button for the ignored-paths list",
  },
  "services.buildFilterRemoveIgnored": {
    message: "무시 경로 제거",
    description: "Build & Deploy: remove-row label for an ignored path",
  },
  "services.buildPathTooLong": {
    message: "최대 512바이트",
    description:
      "Inline error under a Root Directory, Dockerfile Path, Docker Build Context or Build Filter pattern over the server's 512-byte limit; a Chinese character is 3 bytes (w21/045)",
  },
  "services.buildPathControlChar": {
    message: "제어 문자나 탭을 포함할 수 없습니다",
    description:
      "Inline error under a build path or Build Filter pattern with a control character (w21/045)",
  },
  "services.buildPathBackslash": {
    message: "경로 구분자로 \\ 대신 /를 사용하세요",
    description:
      "Inline error under a build path or Build Filter pattern with a backslash (w21/045)",
  },
  "services.buildPathAbsolute": {
    message: "저장소 기준 상대 경로여야 하며 /로 시작할 수 없습니다",
    description:
      "Inline error under a build path or Build Filter pattern that starts with / (w21/045)",
  },
  "services.buildPathDotDot": {
    message: "저장소 내부에 있어야 합니다. .. 세그먼트를 사용할 수 없습니다",
    description:
      "Inline error under a build path or Build Filter pattern with a .. segment (w21/045)",
  },
  "services.buildPathEmptySegment": {
    message:
      "빈 세그먼트를 포함할 수 없습니다. //와 끝의 /를 사용할 수 없습니다",
    description:
      "Inline error under a Dockerfile Path or Docker Build Context with an empty path segment; the Root Directory normalizes these away (w21/045)",
  },
  "services.buildFilterUnclosedClass": {
    message: "[가 ]로 닫히지 않았습니다",
    description:
      "Inline error under a Build Filter glob with an unclosed character class (w21/045)",
  },
  "services.healthCheckPathSlash": {
    message: "/로 시작해야 하며, 포트만 확인하려면 비워 두세요",
    description:
      "Inline error under a Health Check Path that does not start with / (w21/045)",
  },
  "services.buildFilterSave": {
    message: "빌드 필터 저장",
    description: "Build & Deploy: save button for the build-filters editor",
  },
  "services.buildFilterSuccess": {
    message: "빌드 필터를 업데이트했습니다.",
    description: "Toast after setBuildFilter succeeds",
  },
  "services.buildFilterError": {
    message: "빌드 필터를 업데이트하지 못했습니다. 다시 시도하세요.",
    description: "Toast after setBuildFilter fails",
  },
  "services.preDeployLabel": {
    message: "사전 배포 명령",
    description: "Build & Deploy: label for the pre-deploy command field",
  },
  "services.preDeployHint": {
    message:
      "새 이미지가 트래픽을 처리하기 전에 한 번 실행됩니다(예: 데이터베이스 마이그레이션). 0이 아닌 코드로 종료되면 배포가 실패하고 이전 버전이 계속 라이브 상태로 유지됩니다.",
    description: "Build & Deploy: help text for the pre-deploy command field",
  },
  "services.preDeployPaidOnly": {
    message: "사전 배포 명령은 유료 요금제에서 사용할 수 있습니다.",
    description:
      "Build & Deploy: why the pre-deploy command field is disabled on a Free service (Render: paid web services, private services and background workers; w8/m131)",
  },
  "services.preDeployPlaceholder": {
    message: "예: npm run migrate",
    description: "Build & Deploy: placeholder for the pre-deploy command input",
  },
  "services.preDeployEdit": {
    message: "사전 배포 명령 편집",
    description:
      "Build & Deploy: accessible label for the pre-deploy edit-pencil button",
  },
  "services.healthCheckPathConfirmTitle": {
    message: "헬스 체크 경로를 변경하시겠습니까? 새 값: {value}",
    description:
      "Settings: Health Check Path change confirmation title (w4/616)",
  },
  "services.healthCheckPathConfirmEmpty": {
    message: "TCP 확인",
    description:
      "Settings: empty Health Check Path phrase in its confirmation title",
  },
  "services.maxShutdownDelayConfirmTitle": {
    message: "최대 종료 지연 시간을 {value}초로 변경하시겠습니까?",
    description:
      "Settings: Max shutdown delay change confirmation title (w4/616)",
  },
  "services.preDeployConfirmTitle": {
    message: "사전 배포 명령을 변경하시겠습니까? 새 명령: {value}",
    description:
      "Settings: Pre-Deploy Command change confirmation title (w4/616)",
  },
  "services.preDeployConfirmEmpty": {
    message: "사전 배포 명령 없음",
    description:
      "Settings: empty Pre-Deploy Command phrase in its confirmation title",
  },
  "services.settingRedeployConfirmBody": {
    message:
      "서비스가 새 설정으로 재배포됩니다. 진행 중인 요청은 이전 인스턴스가 교체되기 전에 완료됩니다.",
    description:
      "Settings: confirmation body for a setting whose save starts a deploy (w4/616)",
  },
  "services.settingRedeployConfirmBodyNeverLive": {
    message:
      "서비스가 새 설정으로 배포됩니다. 아직 서비스 중인 것이 없으며, 배포가 성공하면 라이브가 됩니다.",
    description:
      "Settings: deploying-setting confirmation body for a service that has never gone live (w4/616)",
  },
  "services.preDeployConfirmBody": {
    message:
      "서비스가 재배포되며, 새 릴리스가 라이브가 되기 전에 새 명령이 실행됩니다. 진행 중인 요청은 이전 인스턴스가 교체되기 전에 완료됩니다.",
    description:
      "Settings: Pre-Deploy Command change confirmation body (w4/616)",
  },
  "services.preDeployConfirmBodyNeverLive": {
    message:
      "서비스가 배포되며, 릴리스가 라이브가 되기 전에 새 명령이 실행됩니다. 아직 서비스 중인 것이 없으며, 배포가 성공하면 라이브가 됩니다.",
    description:
      "Settings: Pre-Deploy Command confirmation body for a service that has never gone live (w4/616)",
  },
  "services.preDeploySuccess": {
    message: "사전 배포 명령을 업데이트했습니다. 새 배포가 시작되었습니다.",
    description: "Toast after setPreDeployCommand succeeds",
  },
  "services.preDeployError": {
    message: "사전 배포 명령을 업데이트하지 못했습니다. 다시 시도하세요.",
    description: "Toast after setPreDeployCommand fails",
  },
  "services.autoDeployLabel": {
    message: "자동 배포",
    description: "Build & Deploy: label for the auto-deploy toggle",
  },
  "services.autoDeployOnCommit": {
    message: "커밋 시",
    description:
      "Auto-Deploy select option: redeploy on every matching git push (Render's autoDeployTrigger 'commit', w5/m53)",
  },
  "services.autoDeployAfterChecks": {
    message: "CI 검사 통과 후",
    description:
      "Auto-Deploy select option: redeploy a pushed commit once all its GitHub checks pass (Render's autoDeployTrigger 'checksPass', w1/m181)",
  },
  "services.autoDeployOff": {
    message: "꺼짐",
    description:
      "Auto-Deploy select option: never redeploy on push (Render's autoDeployTrigger 'off')",
  },
  "services.autoDeployEdit": {
    message: "자동 배포 편집",
    description:
      "Accessible label for the Auto-Deploy select's edit-pencil button",
  },
  "services.autoDeployViaGitHub": {
    message:
      "추적 중인 브랜치에 push하면 GitHub 앱을 통해 자동으로 재배포됩니다.",
    description:
      "Build & Deploy: source indicator when the connected GitHub App installation grants THIS repo (pushDeliveryMethod github_app)",
  },
  "services.autoDeployViaWebhook": {
    message:
      "이 저장소의 수동 git 웹훅이 이 설치의 BEX_WEBHOOK_SECRET으로 구성된 경우에만 push 시 재배포됩니다.",
    description:
      "Build & Deploy: source indicator when the GitHub App cannot deliver a push for this repo — no connection, another git host, or a repo the installation does not grant (pushDeliveryMethod manual_webhook)",
  },
  "services.autoDeployUnavailable": {
    message:
      "이 저장소에서는 자동 배포를 실행할 수 없습니다. GitHub가 bex로 push 이벤트를 보내지 않습니다. GitHub 앱을 통해 저장소를 연결하거나 Deploy Hook으로 배포를 트리거하세요.",
    description:
      "Build & Deploy: source indicator when no push can reach bex for this repo — the GitHub App does not cover it and this install rejects the shared manual webhook key (pushDeliveryMethod unavailable, w4/483)",
  },
  "services.autoDeployDeliveryUnknown": {
    message:
      "추적 중인 브랜치에 대한 push가 bex에 전달되는 방식을 확인하는 중… 이 상태가 계속되면 GitHub에 연결할 수 없어 전달 방식을 확인할 수 없는 것입니다.",
    description:
      "Build & Deploy: source indicator while the per-repo push-delivery check is loading or returned unknown (GitHub unreachable) — states the uncertainty instead of naming a mechanism (w6/m99)",
  },
  "services.autoDeployOnSuccess": {
    message: "자동 배포를 켰습니다.",
    description: "Toast after enabling auto-deploy",
  },
  "services.autoDeployChecksSuccess": {
    message: "이제 자동 배포는 CI 검사가 통과할 때까지 기다립니다.",
    description: "Toast after choosing After CI Checks Pass",
  },
  "services.autoDeployOffSuccess": {
    message: "자동 배포를 껐습니다.",
    description: "Toast after disabling auto-deploy",
  },
  "services.autoDeployError": {
    message: "자동 배포를 변경하지 못했습니다. 다시 시도하세요.",
    description: "Toast after setAutoDeploy fails",
  },
  "services.deployHookTitle": {
    message: "Deploy Hook",
    description: "Settings tab: secret Deploy Hook section title",
  },
  "services.deployHookDescription": {
    message: "비밀 URL 하나로 CI에서 배포를 트리거합니다.",
    description: "Settings tab: Deploy Hook section description",
  },
  "services.deployHookURLLabel": {
    message: "Deploy Hook URL",
    description: "Accessible label for the secret Deploy Hook URL field",
  },
  "services.deployHookReveal": {
    message: "Deploy Hook URL 표시",
    description: "Accessible label for the reveal-secret button",
  },
  "services.deployHookHide": {
    message: "Deploy Hook URL 숨기기",
    description: "Accessible label for the hide-secret button",
  },
  "services.deployHookCopy": {
    message: "Deploy Hook URL 복사",
    description: "Accessible label for the Deploy Hook copy button",
  },
  "services.deployHookCopied": {
    message: "Deploy Hook URL을 복사했습니다.",
    description: "Toast after copying the Deploy Hook URL",
  },
  "services.deployHookCopyError": {
    message: "Deploy Hook URL을 복사하지 못했습니다.",
    description: "Toast after Deploy Hook URL clipboard failure",
  },
  "services.deployHookSecretHint": {
    message:
      "이 URL을 비밀로 유지하세요. URL을 가진 사람은 누구나 API 키 없이 이 서비스를 배포할 수 있습니다.",
    description: "Security warning below the Deploy Hook URL",
  },
  "services.deployHookRegenerate": {
    message: "Hook 재생성",
    description: "Deploy Hook rotation button",
  },
  "services.deployHookRegenerateTitle": {
    message: "Deploy Hook을 재생성하시겠습니까?",
    description: "Deploy Hook rotation confirmation title",
  },
  "services.deployHookRegenerateWarning": {
    message:
      "현재 URL은 즉시 작동하지 않게 됩니다. 이 URL을 사용하는 모든 CI 시스템, Cron Job, 연동을 업데이트하세요.",
    description: "Deploy Hook rotation confirmation warning",
  },
  "services.deployHookCancel": {
    message: "취소",
    description: "Deploy Hook rotation confirmation cancel button",
  },
  "services.deployHookRegenerateConfirm": {
    message: "재생성",
    description: "Deploy Hook rotation confirmation action",
  },
  "services.deployHookRegenerated": {
    message:
      "Deploy Hook을 재생성했습니다. 이전 URL은 더 이상 작동하지 않습니다.",
    description: "Toast after successful Deploy Hook rotation",
  },
  "services.deployHookRegenerateError": {
    message: "Deploy Hook을 재생성하지 못했습니다. 다시 시도하세요.",
    description: "Toast after Deploy Hook rotation fails",
  },
  "services.deployHookLoadError": {
    message: "Deploy Hook URL을 불러오지 못했습니다.",
    description: "Deploy Hook section query error",
  },
  "services.colType": {
    message: "유형",
    description: "Services table column header (service type)",
  },
  "services.typeWeb": {
    message: "Web Service",
    description: "Service-type badge: an HTTP service exposed at a URL",
  },
  "services.typePrivate": {
    message: "Private Service",
    description:
      "Service-type badge: an HTTP service reachable only in-cluster",
  },
  "services.typeWorker": {
    message: "Background Worker",
    description: "Service-type badge: runs with no HTTP port/URL",
  },
  "services.typeCron": {
    message: "Cron Job",
    description: "Service-type badge: runs a command on a schedule",
  },
  "services.typeStatic": {
    message: "Static Site",
    description:
      "Service-type badge: built output served from the object-store origin",
  },
  "services.staticTitle": {
    message: "Static Site",
    description:
      "Settings section title for a static site's publish dir + edge rules",
  },
  "services.staticDescription": {
    message:
      "게시되는 출력 디렉터리와 이를 제공할 때 적용되는 엣지 규칙입니다.",
    description: "Static Site settings section description",
  },
  "services.staticEdgeRulesHint": {
    message: "리디렉션/리라이트와 응답 헤더에는 별도의 페이지가 있습니다:",
    description:
      "Settings hint pointing at the dedicated Redirects/Rewrites + Headers pages",
  },
  "services.staticCancel": {
    message: "취소",
    description: "Cancel an inline edit",
  },
  "services.publishPathEscapes": {
    message:
      "이 경로는 저장소 밖으로 벗어납니다. 게시 디렉터리는 루트 디렉터리를 기준으로 해석되므로 .. 세그먼트를 제거하세요.",
    description:
      "Inline error under a repo-backed static site's publish directory whose .. segments leave the checkout (w21/050)",
  },
  "services.publishPathControlChar": {
    message: "제어 문자나 줄 바꿈을 포함할 수 없습니다",
    description:
      "Inline error under a static site's publish directory with a control character (w21/050)",
  },
  "services.publishPathLabel": {
    message: "게시 디렉터리",
    description: "Label for a static site's publishPath",
  },
  "services.publishPathPlaceholder": {
    message: "dist",
    description: "Placeholder for the publish directory input",
  },
  "services.publishPathEdit": {
    message: "게시 디렉터리 편집",
    description:
      "Accessible label for the publish-directory edit-pencil button",
  },
  "services.publishPathHint": {
    message:
      "사이트 루트로 제공되는 빌드 출력 디렉터리입니다(예: dist, build, public). 변경하면 사이트가 다시 게시됩니다.",
    description: "Help text under the publish directory field",
  },
  "services.publishPathSaved": {
    message: "게시 디렉터리를 업데이트했습니다",
    description: "Toast after saving a static site's publishPath",
  },
  "services.publishPathRepublishNote": {
    message: "사이트가 곧 다시 게시됩니다.",
    description: "Toast description after changing publishPath",
  },
  "services.publishPathError": {
    message: "게시 디렉터리를 업데이트하지 못했습니다",
    description: "Error toast for a failed publishPath change",
  },
  "services.routesTitle": {
    message: "리디렉션 및 리라이트",
    description: "Title for the static-site routes editor",
  },
  "services.routesHint": {
    message:
      "순서대로 일치 여부를 확인하며 처음 일치하는 규칙이 적용됩니다. 리디렉션은 301을 반환하고, 리라이트는 다른 경로를 제공합니다(SPA 폴백은 /*를 /index.html로 리라이트). :name 세그먼트는 경로 세그먼트 하나를 캡처합니다(예: /blog/posts/:postid → /blog/:postid).",
    description: "Help text for the routes editor",
  },
  "services.routeAdd": {
    message: "규칙 추가",
    description: "Add a route rule",
  },
  "services.routeType": {
    message: "유형",
    description: "Route type column",
  },
  "services.routeSource": {
    message: "소스",
    description: "Route source-path column",
  },
  "services.routeDestination": {
    message: "대상",
    description: "Route destination-path column",
  },
  "services.routeRewrite": {
    message: "리라이트",
    description: "Route type: rewrite",
  },
  "services.routeRedirect": {
    message: "리디렉션",
    description: "Route type: redirect",
  },
  "services.routeTypeAria": {
    message: "규칙 {n} 유형",
    description:
      "A route rule's type select (aria-label), named by its row as Remove rule {n} is (w4/541)",
  },
  "services.routeSourceAria": {
    message: "규칙 {n} 소스",
    description:
      "A route rule's source field (aria-label), named by its row (w4/541)",
  },
  "services.routeDestinationAria": {
    message: "규칙 {n} 대상",
    description:
      "A route rule's destination field (aria-label), named by its row (w4/541)",
  },
  "services.routeRemove": {
    message: "규칙 {n} 제거",
    description: "Remove a route rule (aria-label)",
  },
  "services.routesSave": {
    message: "경로 저장",
    description: "Save the routes list",
  },
  "services.staticRuleRequired": {
    message: "필수 항목",
    description: "Inline error under an empty static-site rule field (w4/145)",
  },
  "services.staticRulePathSlash": {
    message: "/로 시작하는 경로여야 합니다",
    description:
      "Inline error under a static-site rule path that doesn't start with / (w4/145)",
  },
  "services.staticRuleExternalHost": {
    message:
      "고정된 호스트가 있는 일반 http(s) URL을 사용하세요(예: https://example.org/new)",
    description:
      "Inline error under a redirect destination that is an absolute URL without a literal host (w8/162)",
  },
  "services.staticRuleLocalPath": {
    message: "//host가 아닌 이 사이트 내의 경로여야 합니다",
    description:
      "Inline error under a redirect/rewrite destination that is a network-path reference (w4/145)",
  },
  "services.staticRuleDestinationPathOrUrl": {
    message: "경로 또는 http(s) URL이어야 합니다",
    description:
      "Inline error under a redirect/rewrite destination whose first segment reads as a scheme other than http(s), such as javascript: (w33/007)",
  },
  "services.staticRuleFixRows": {
    message: "저장하려면 표시된 행을 수정하세요.",
    description:
      "Note beside a disabled Save while a static-site rule row is invalid (w4/145)",
  },
  "services.staticRuleEmptyRow": {
    message: "저장하려면 빈 행을 채우거나 제거하세요.",
    description:
      "Note beside a disabled Save while a static-site rule row is still completely empty (w4/145)",
  },
  "services.staticRuleRow": {
    message: "{row}행:",
    description:
      "Prefix naming the 1-based rule row in a server refusal, replacing its zero-based wire index (w4/145)",
  },
  "services.staticRulePathTooLong": {
    message: "최대 2048바이트",
    description:
      "Inline error under an over-long static-site rule path; the server counts UTF-8 bytes, so a Chinese character is 3 (w4/402, w21/043)",
  },
  "services.staticRuleSourceRepeatsPlaceholder": {
    message: "각 :name은 소스에 한 번만 나올 수 있습니다",
    description:
      "Inline error under a redirect/rewrite source that declares the same :name placeholder twice (w21/043)",
  },
  "services.staticRuleUndeclaredPlaceholder": {
    message: "소스에 선언되지 않은 :name을 사용했습니다",
    description:
      "Inline error under a redirect/rewrite destination whose :name placeholder (other than :splat) the source never declares (w21/043)",
  },
  "services.staticRuleHeaderNameTooLong": {
    message: "최대 256자",
    description: "Inline error under an over-long header name (w4/402)",
  },
  "services.staticRuleHeaderNameInvalid": {
    message: "올바른 HTTP 헤더 이름이 아닙니다",
    description:
      "Inline error under a header name with characters a header name can't hold (w4/402)",
  },
  "services.staticRuleHeaderNameReserved": {
    message: "이 헤더는 서버에서 설정하며 재정의할 수 없습니다",
    description:
      "Inline error under a reserved header name such as Content-Length (w4/402)",
  },
  "services.staticRuleHeaderValueTooLong": {
    message: "최대 4096바이트",
    description: "Inline error under an over-long header value (w4/402)",
  },
  "services.staticRuleHeaderValueInvalid": {
    message: "헤더 값에 사용할 수 없는 문자가 포함되어 있습니다",
    description:
      "Inline error under a header value with a control character (w4/402)",
  },
  "services.staticRuleCookieDomain": {
    message: "공유 호스팅에서는 Set-Cookie로 Domain을 설정할 수 없습니다",
    description:
      "Inline error under a Set-Cookie value carrying a Domain attribute (w4/402)",
  },
  "services.staticRoutesSaved": {
    message: "경로를 업데이트했습니다",
    description: "Toast after saving routes",
  },
  "services.staticRoutesError": {
    message: "경로를 업데이트하지 못했습니다",
    description: "Error toast for a failed routes save",
  },
  "services.headersTitle": {
    message: "응답 헤더",
    description: "Title for the static-site custom-headers editor",
  },
  "services.headersHint": {
    message:
      "경로가 일치하는 응답에 추가되는 사용자 지정 응답 헤더입니다. Content-Length 같은 본문 프레이밍 및 연결 헤더는 서버에서 설정하며 재정의할 수 없습니다. Content-Encoding은 실제로 압축되어 저장된 파일에만 설정하세요.",
    description: "Help text for the headers editor",
  },
  "services.headerAdd": {
    message: "헤더 추가",
    description: "Add a custom header",
  },
  "services.headerPath": {
    message: "경로",
    description: "Header path column",
  },
  "services.headerName": {
    message: "이름",
    description: "Header name column",
  },
  "services.headerValue": {
    message: "값",
    description: "Header value column",
  },
  "services.headerPathAria": {
    message: "헤더 {n} 경로",
    description:
      "A header rule's path field (aria-label), named by its row as Remove header {n} is (w4/541)",
  },
  "services.headerNameAria": {
    message: "헤더 {n} 이름",
    description:
      "A header rule's name field (aria-label), named by its row (w4/541)",
  },
  "services.headerValueAria": {
    message: "헤더 {n} 값",
    description:
      "A header rule's value field (aria-label), named by its row (w4/541)",
  },
  "services.headerRemove": {
    message: "헤더 {n} 제거",
    description: "Remove a header (aria-label)",
  },
  "services.headersSave": {
    message: "헤더 저장",
    description: "Save the headers list",
  },
  "services.staticHeadersSaved": {
    message: "헤더를 업데이트했습니다",
    description: "Toast after saving headers",
  },
  "services.staticHeadersError": {
    message: "헤더를 업데이트하지 못했습니다",
    description: "Error toast for a failed headers save",
  },
  "services.typeUnknown": {
    message: "서비스",
    description: "Service-type badge fallback for an unrecognized type",
  },
  "services.cronRunColDuration": {
    message: "소요 시간",
    description:
      "Cron Runs page: the Duration column's header, and a row's duration for assistive tech (Render's DURATION, w3/m556)",
  },
  "services.cronRunStatusSucceeded": {
    message: "실행 성공",
    description:
      "Cron Runs page: a successful run's status, in its icon's tooltip (Render's text, w3/m556)",
  },
  "services.cronRunStatusFailed": {
    message: "실행 실패",
    description:
      "Cron Runs page: a failed run's status, in its icon's tooltip (Render's text, w3/m556)",
  },
  "services.cronRunStatusCanceled": {
    message: "취소됨",
    description:
      "Cron Runs page: a canceled run's status in its icon's tooltip, and its line when no one is named as canceling it (Render's text, w3/m556)",
  },
  "services.cronRunStatusInProgress": {
    message: "진행 중",
    description:
      "Cron Runs page: the status of a run that has not ended, started or not, in its icon's tooltip (Render's text, w3/m556)",
  },
  "services.cronRunStatusLastSuccessful": {
    message: "마지막 성공 실행",
    description:
      "Cron Runs page: the newest successful run's status, in its icon's tooltip (Render's text, w3/m556)",
  },
  "services.cronRunCancel": {
    message: "취소",
    description:
      "Cron Runs page: a run in progress's Cancel button, which cancels it at once (Render's text, w3/m556)",
  },
  "services.cronRunCancelError": {
    message: "Cron 실행을 취소하지 못했습니다.",
    description: "Toast after cron run cancellation fails",
  },
  "services.cronTriggerRun": {
    message: "실행 트리거",
    description: "Button that fires an immediate cron run (Render parity)",
  },
  "services.cronTriggerError": {
    message: "실행을 트리거하지 못했습니다. 잠시 후 다시 시도하세요.",
    description:
      "Toast, kept until dismissed, when Trigger Run fails and the server gave no message (Render's text, w3/m555)",
  },
  "services.cronTriggerForbidden": {
    message: "이 Cron Job을 실행할 권한이 없습니다.",
    description:
      "Toast, kept until dismissed, when Trigger Run is refused for permission (Render's text, w3/m555)",
  },
  "services.cronTriggerNoBuild": {
    message: "성공한 빌드가 없습니다. {deploys}에서 확인하세요.",
    description:
      "Tooltip of a disabled Trigger Run on a repo-backed cron job whose build never succeeded; {deploys} is a link to its Deploys page, named by services.navDeploys (Render's text, w3/m555)",
  },
  "services.cronTriggerSuspended": {
    message: "실행을 트리거하려면 이 Cron Job을 재개하세요.",
    description:
      "Tooltip of a disabled Trigger Run while the cron job is suspended, shown in the Runs page's empty states (Render's text, w3/m556)",
  },
  "services.cronRunsLoadMore": {
    message: "더 보기",
    description: "Cron run history pagination button",
  },
  "services.cronRunsLoadingMore": {
    message: "불러오는 중…",
    description: "Cron run history pagination busy label",
  },
  "services.cronRunsLoadError": {
    message: "Cron 실행 기록을 불러오지 못했습니다.",
    description: "Cron run history read error",
  },
  "services.cronRunsLoadFailed": {
    message:
      "죄송합니다. 이 Cron Job의 실행 기록을 불러오지 못했습니다. 새로고침 후 다시 시도해 보세요.",
    description:
      "Cron Runs page: the warning shown in place of the list when the runs could not be read (Render's text, w3/m556)",
  },
  "services.cronRunsSearchPlaceholder": {
    message: "Cron Job 실행 검색",
    description:
      "Cron Runs page: the search field's placeholder and name (Render's text, w3/m556)",
  },
  "services.cronRunsSearchClear": {
    message: "검색 지우기",
    description:
      "Cron Runs page: the search field's clear button, for assistive tech (w3/m556)",
  },
  "services.cronRunsColRun": {
    message: "실행",
    description:
      "Cron Runs page: the list's first column header, before the count (Render's RUN, w3/m556)",
  },
  "services.cronRunsColTrigger": {
    message: "트리거",
    description:
      "Cron Runs page: the Trigger column's header, and a row's trigger for assistive tech (Render's TRIGGER, w3/m556)",
  },
  "services.cronRunTitle": {
    message: "Cron Job 실행",
    description: "Cron Runs page: a run row's title (Render's text, w3/m556)",
  },
  "services.cronRunStartedAgo": {
    message: "{timestamp} 실행됨",
    description:
      'Cron Runs page: when a run started, under its title; {timestamp} is elapsed text like "2 hours ago" (Render\'s "Run 21h ago", w3/m556)',
  },
  "services.cronRunBy": {
    message: "{user}님이 실행",
    description:
      'Cron Runs page: after when a run started, the email of the user who triggered it (Render\'s "by <user>", w3/m556)',
  },
  "services.cronRunByYou": {
    message: "내가 실행",
    description:
      'Cron Runs page: after when a run started, when the viewer triggered it (Render\'s "by you", w3/m556)',
  },
  "services.cronRunCanceledBy": {
    message: "{user}님이 취소함",
    description:
      "Cron Runs page: a canceled run's line naming the email of the user who canceled it (Render's text, w3/m556)",
  },
  "services.cronRunCanceledByYou": {
    message: "내가 취소함",
    description:
      "Cron Runs page: a canceled run's line when the viewer canceled it (Render's \"Canceled by you\", w3/m556)",
  },
  "services.cronRunTriggerScheduled": {
    message: "예약됨",
    description:
      "Cron Runs page: the trigger of a run the schedule started (Render's text, w3/m556)",
  },
  "services.cronRunTriggerManual": {
    message: "수동",
    description:
      "Cron Runs page: the trigger of a run a user started (Render's text, w3/m556)",
  },
  "services.cronRunsNoMatchTitle": {
    message: "일치하는 결과가 없습니다",
    description:
      "Cron Runs page: title when the search matches none of the loaded runs (Render's text, w3/m556)",
  },
  "services.cronRunsNoMatchBody": {
    message: "찾는 항목이 없나요? 검색어를 바꾸거나 오타가 없는지 확인하세요.",
    description:
      "Cron Runs page: body when the search matches none of the loaded runs (Render's text, w3/m556)",
  },
  "services.cronRunsResetSearch": {
    message: "검색 초기화",
    description:
      "Cron Runs page: the button that clears a search that matched nothing (Render's text, w3/m556)",
  },
  "services.cronRunsEmptyRecentTitle": {
    message: "이 Cron Job은 지난 90일 동안 실행되지 않았습니다.",
    description:
      "Cron Runs page: title when a cron job whose build succeeded lists no runs (Render's text, w3/m556)",
  },
  "services.cronRunsEmptyRecentBody": {
    message: "수동으로 실행을 트리거하거나 다음 예약 실행을 기다리세요.",
    description:
      "Cron Runs page: body when a cron job whose build succeeded lists no runs, above Trigger Run (Render's text, w3/m556)",
  },
  "services.cronRunsEmptyNeverTitle": {
    message: "이 Cron Job은 아직 실행되지 않았습니다.",
    description:
      "Cron Runs page: title when a cron job whose build never succeeded has no runs (Render's text, w3/m556)",
  },
  "services.cronRunsEmptyNeverBody": {
    message:
      "첫 빌드가 완료되면 실행됩니다. 수동으로 실행을 트리거할 수도 있습니다.",
    description:
      "Cron Runs page: body when a cron job whose build never succeeded has no runs, above Trigger Run (Render's text, w3/m556)",
  },
  "services.cronRunsCount_other": {
    message: "실행 {count}건",
    description:
      "Cron Runs page: the list's count, for assistive tech, beside RUN (Render's \"30 runs\", w3/m556)",
  },
  "services.suspendCardTitle": {
    message: "서비스 일시 중지",
    description: "Settings tab suspend section title",
  },
  "services.suspendCardDescription": {
    message:
      "서비스를 일시 중지하면 서비스가 종료되고 트래픽 처리가 중단됩니다. URL과 인증서는 유지되며 언제든지 재개할 수 있습니다.",
    description:
      "Settings tab suspend section description, for the types served at a public host",
  },
  "services.suspendCardDescriptionNoUrl": {
    message:
      "서비스를 일시 중지하면 서비스가 종료되고 실행이 중단됩니다. 언제든지 재개할 수 있습니다.",
    description:
      "Settings tab suspend section description, for private services and background workers — neither has a public URL or certificates to keep",
  },
  "services.resumeCardTitle": {
    message: "서비스 재개",
    description:
      "Settings tab resume section title (shown when service is suspended)",
  },
  "services.resumeCardDescription": {
    message:
      "서비스를 재개하면 다시 온라인 상태가 되어 트래픽 처리를 재개합니다.",
    description: "Settings tab resume section description",
  },
  "services.resumeCardDescriptionCrashLoop": {
    message:
      "이 서비스는 24시간 넘게 시작과 정상 상태 전환에 반복적으로 실패하여 일시 중지되었습니다. 설정 오류, 애플리케이션 코드의 버그, 또는 컴퓨팅 플랜이 작은 것이 원인인 경우가 많습니다. 원인을 해결한 다음 서비스를 재개하여 다시 온라인 상태로 만드세요.",
    description:
      "Settings tab resume section description for a service the platform suspended for a crash loop (stuck_crashlooping)",
  },
  "services.resumeCardDescriptionCron": {
    message: "이 Cron Job을 재개하면 예약된 실행이 다시 시작됩니다.",
    description: "Settings tab resume section description for a cron job",
  },
  "services.dangerZoneTitle": {
    message: "위험 구역",
    description: "Settings tab delete section title (destructive)",
  },
  "services.dangerZoneDescription": {
    message:
      "서비스를 삭제하면 서비스와 배포, URL이 영구적으로 제거됩니다. 이 작업은 되돌릴 수 없습니다.",
    description:
      "Settings tab delete section description, for the types served at a public host",
  },
  "services.dangerZoneDescriptionNoUrl": {
    message:
      "서비스를 삭제하면 서비스와 배포가 영구적으로 제거됩니다. 이 작업은 되돌릴 수 없습니다.",
    description:
      "Settings tab delete section description, for private services, background workers and cron jobs — none of which have a URL to remove",
  },
  "services.deleteButton": {
    message: "서비스 삭제",
    description: "Danger-zone button that opens the delete-confirm dialog",
  },
  "services.deleteConfirmTitle": {
    message: "{name} 삭제",
    description: "Delete-confirm dialog title",
  },
  "services.deleteConfirmBody": {
    message:
      "이 작업은 되돌릴 수 없습니다. 정말 이 서비스({type})를 삭제하시겠습니까?",
    description:
      "Delete-confirm dialog body ({type} = lowercase Render type words, e.g. 'web service')",
  },
  "services.deleteConfirmPrompt": {
    message: "확인하려면 아래에 다음을 입력하세요: {phrase}",
    description:
      "Body prompt naming Render's exact 'sudo delete <type> <name>' phrase (rendered bold by SudoCommandField)",
  },
  "services.deleteCancel": {
    message: "취소",
    description: "Delete-confirm dialog cancel button",
  },
  "services.deleteConfirm": {
    message: "서비스 삭제",
    description:
      "Delete-confirm dialog submit button (armed once the name matches)",
  },
  "services.deleteSuccess": {
    message: "{name} 삭제 완료",
    description: "Toast on a successful service delete",
  },
  "services.deleteError": {
    message: "{name} 삭제 실패",
    description: "Toast on a failed service delete",
  },
  "services.newServiceButton": {
    message: "새 서비스",
    description:
      "Button on the services list page that opens the create wizard",
  },
  "services.createTitle": {
    message: "새 서비스",
    description: "Create-wizard page title",
  },
  "services.createWebTitle": {
    message: "새 Web Service",
    description:
      "Create-wizard page title when the web_service type is selected",
  },
  "services.createWebDescription": {
    message: "Git 저장소 또는 Docker 이미지에서 Web Service를 배포합니다.",
    description:
      "Create-wizard page subtitle when the web_service type is selected",
  },
  "services.createPrivateTitle": {
    message: "새 Private Service",
    description:
      "Create-wizard page title when the private_service type is selected",
  },
  "services.createPrivateDescription": {
    message: "Git 저장소 또는 Docker 이미지에서 Private Service를 배포합니다.",
    description:
      "Create-wizard page subtitle when the private_service type is selected",
  },
  "services.createWorkerTitle": {
    message: "새 Background Worker",
    description:
      "Create-wizard page title when the background_worker type is selected",
  },
  "services.createWorkerDescription": {
    message:
      "Git 저장소 또는 Docker 이미지에서 Background Worker를 배포합니다.",
    description:
      "Create-wizard page subtitle when the background_worker type is selected",
  },
  "services.createStaticTitle": {
    message: "새 Static Site",
    description:
      "Create-wizard page title when the static_site type is selected",
  },
  "services.createStaticDescription": {
    message: "Git 저장소에서 Static Site를 빌드하고 배포합니다.",
    description:
      "Create-wizard page subtitle when the static_site type is selected",
  },
  "services.createCronTitle": {
    message: "새 Cron Job",
    description: "Create-wizard page title when the cron_job type is selected",
  },
  "services.createCronDescription": {
    message: "반복 스케줄에 따라 명령을 실행합니다.",
    description:
      "Create-wizard page subtitle when the cron_job type is selected",
  },
  "services.createSourceTitle": {
    message: "소스 코드",
    description: "Create-wizard source-picker section label",
  },
  "services.createTabGitHub": {
    message: "GitHub",
    description: "Create-wizard source-tab label for connected GitHub repos",
  },
  "services.createTabPublicGit": {
    message: "공개 Git URL",
    description: "Create-wizard source-tab label for a public git URL",
  },
  "services.createTabImage": {
    message: "기존 이미지",
    description: "Create-wizard source-tab label for a pre-built Docker image",
  },
  "services.createRepoSearchShort": {
    message: "검색…",
    description:
      "Repository search placeholder where the field is narrow (a phone); the field keeps its full accessible name",
  },
  "services.createRepoCurrent": {
    message: "현재",
    description:
      "Update Source repository list: tag on the repository the service uses now, shown once another one is chosen",
  },
  "services.createRepoSearchPlaceholder": {
    message: "저장소 검색…",
    description: "Create-wizard GitHub tab repo-search input placeholder",
  },
  "services.createRepoPrivateBadge": {
    message: "비공개",
    description: "Badge on a private GitHub repo row in the repo picker",
  },
  "services.createRepoEmpty": {
    message: "저장소를 찾을 수 없습니다.",
    description:
      "Create-wizard GitHub tab empty state (no repos in the installation)",
  },
  "services.createRepoNoMatch": {
    message: "검색과 일치하는 저장소가 없습니다.",
    description:
      "Create-wizard GitHub tab empty state when the search filter returns nothing",
  },
  "services.createRepoEmptyHint": {
    message:
      "GitHub에서 bex가 저장소에 접근하도록 허용하거나 다른 계정을 연결하세요.",
    description:
      "Source picker GitHub tab, under the empty state: what to do when the connected accounts grant no repository. Credentials is the menu beside the search field",
  },
  "services.createRepoLoadError": {
    message: "저장소를 불러오지 못했습니다.",
    description:
      "Source picker GitHub tab: the repository list could not be read, shown with a Try again button. Not the same as having no repositories",
  },
  "services.createRepoListLabel": {
    message: "저장소",
    description:
      "Accessible name of the repository list in the source picker's GitHub tab",
  },
  "services.createRepoClearSearch": {
    message: "검색 지우기",
    description:
      "Source picker GitHub tab: button under the no-match message that empties the repository search field",
  },
  "services.sourceNoChanges": {
    message: "저장할 변경 사항이 없습니다.",
    description:
      "Update Source dialog footer: why Update source is disabled when the dialog still shows the service's current source",
  },
  "services.sourceSelectRepo": {
    message: "저장소를 선택하세요.",
    description:
      "Update Source dialog footer: why Update source is disabled on the GitHub tab before a repository is picked",
  },
  "services.sourceSelectBuildSource": {
    message: "빌드 소스를 선택하세요.",
    description:
      "Update Source dialog footer: why Update source is disabled on the Build source tab before one is picked",
  },
  "services.sourceEnterRepoUrl": {
    message: "저장소 URL을 입력하세요.",
    description:
      "Update Source dialog footer: why Update source is disabled on the Public Git URL tab while the URL field is empty",
  },
  "services.sourceEnterImage": {
    message: "이미지를 입력하세요.",
    description:
      "Update Source dialog footer: why Update source is disabled on the Existing Image tab while the image field is empty",
  },
  "services.createRepoOpenCredentials": {
    message: "자격 증명 열기",
    description:
      "Source picker GitHub tab: button under the empty repository list that opens the Credentials menu beside the search field",
  },
  "services.sourcePending": {
    message: "새 소스: {source}",
    description:
      "Update Source dialog footer: names what Update source will save. {source} is a repository and branch, an image, or a build source's name",
  },
  "services.sourceBranchLocked": {
    message: "저장소를 선택하면 사용할 수 있습니다.",
    description:
      "Update Source dialog: helper under the disabled Branch field before a repository is picked or a valid URL entered",
  },
  "services.sourceNoBuildSources": {
    message: "선택할 빌드 소스가 없습니다.",
    description:
      "Update Source dialog footer: why Update source is disabled on the Build source tab when the workspace has no build sources",
  },
  "services.sourceFixField": {
    message: "계속하려면 위 필드를 완성하세요.",
    description:
      "Update Source dialog footer: why Update source is disabled while a field is malformed (its own message is under it) or a required one is empty",
  },
  "services.createRepoNoMatchHint": {
    message:
      "목록에 없나요? GitHub에서 bex에 접근 권한을 주거나 다른 계정을 연결하세요.",
    description:
      "Source picker GitHub tab, under the no-match message: the usual cause is a repository the connected accounts do not grant. Shown with Clear search and Open Credentials buttons",
  },
  "services.sourceBranchLockedUrl": {
    message: "저장소 URL을 입력하면 사용할 수 있습니다.",
    description:
      "Update Source dialog, Public Git URL tab: helper under the disabled Branch field before a valid URL is entered",
  },
  "services.createGitConnectPromptTitle": {
    message: "GitHub 연결",
    description:
      "Create-wizard GitHub tab connect-prompt heading (no GitHub connection yet)",
  },
  "services.createGitConnectPromptBody": {
    message:
      "GitHub 계정을 연결하면 비공개 또는 공개 저장소에서 배포할 수 있습니다.",
    description:
      "Create-wizard GitHub tab connect-prompt body (no GitHub connection yet)",
  },
  "services.createPublicUrlLabel": {
    message: "저장소 URL",
    description: "Create-wizard Public Git URL tab input label",
  },
  "services.createPublicUrlPlaceholder": {
    message: "https://github.com/you/your-repo",
    description: "Create-wizard Public Git URL tab input placeholder",
  },
  "services.createPublicUrlError": {
    message:
      "https:// URL 또는 GitHub SSH URL(git@github.com:owner/repo.git)을 입력하세요.",
    description: "Create-wizard Public Git URL tab validation message",
  },
  "services.createPublicUrlFileError": {
    message: "파일 페이지가 아닌 저장소 URL을 사용하세요.",
    description:
      "Create-wizard Public Git URL: a GitHub file (blob) page was pasted, which names no repository to build (w4/576)",
  },
  "services.createImageLabel": {
    message: "Docker 이미지",
    description: "Create-wizard Existing Image tab input label",
  },
  "services.createImagePlaceholder": {
    message: "docker.io/library/nginx:latest",
    description: "Create-wizard Existing Image tab input placeholder",
  },
  "services.createImagePortHint": {
    message:
      "bex는 이미지가 선언한 포트(EXPOSE)를 사용합니다. 이미지가 선언하지 않은 포트에서 수신 대기하는 경우에만 아래에 포트를 설정하세요.",
    description:
      "Create-wizard Existing Image tab hint: set the port below to the one the image listens on (w9/011, w6/m149)",
  },
  "services.createImageMalformed": {
    message:
      "Docker 이미지 참조를 사용하세요. 예: docker.io/library/nginx:latest — 소문자 경로에 선택적으로 태그 또는 다이제스트를 붙일 수 있습니다.",
    description: "Inline error under a malformed Docker Image field (w4/406)",
  },
  "services.createImageTooLong": {
    message: "최대 512바이트",
    description: "Inline error under an over-long Docker Image field (w4/406)",
  },
  "services.createImagePrivateHost": {
    message:
      "레지스트리 호스트는 공개되어 있어야 합니다. localhost와 비공개 또는 링크 로컬 주소는 사용할 수 없습니다.",
    description:
      "Inline error under a Docker Image field naming a private registry host (w4/406)",
  },
  "services.createFieldPort": {
    message: "포트",
    description: "Create wizard: service port field label",
  },
  "services.createFieldPortFromImage": {
    message: "이미지에서 가져오기",
    description:
      "Create wizard: port placeholder for an image-port service (a Docker-built or prebuilt-image web or private service), whose ports come from its image (w6/m148, w4/m243, w4/m323)",
  },
  "services.createFieldPortImageHint": {
    message: "비워 두면 이미지가 선언한 포트가 $PORT가 됩니다.",
    description:
      "Create wizard: port hint for an image-port private service (Docker-built or prebuilt image), whose primary is also its $PORT — blank takes the image's declared ports (w6/m148, w4/m323, w4/615)",
  },
  "services.createFieldPortImageHintWeb": {
    message:
      "비워 두면 이미지가 선언한 포트로 트래픽을 보냅니다. 포트를 설정하지 않으면 $PORT 변수는 10000으로 유지됩니다.",
    description:
      "Create wizard: port hint for an image-port web service (Docker-built or prebuilt image) — the selected port gets the traffic, while $PORT stays Render's 10000 unless set (w4/m243, w4/615)",
  },
  "services.createFieldPortHint": {
    message:
      "컨테이너는 포트 {port}에서 수신해야 합니다. bex가 이 포트로 트래픽을 보내고 $PORT로 설정합니다.",
    description: "Create wizard: service port field hint",
  },
  "services.createRegistryCredentialLabel": {
    message: "레지스트리 자격 증명",
    description: "Create-wizard private-image registry credential label",
  },
  "services.createRegistryCredentialPlaceholder": {
    message: "레지스트리 자격 증명 선택",
    description: "Create-wizard registry credential picker placeholder",
  },
  "services.createRegistryCredentialNone": {
    message: "자격 증명 없음 — 공개 이미지",
    description: "Create-wizard registry credential picker empty option",
  },
  "services.createRegistryCredentialDescription": {
    message:
      "이미지 또는 Dockerfile 베이스 이미지가 비공개인 경우 저장된 워크스페이스 자격 증명을 선택하세요.",
    description: "Create-wizard registry credential picker help text",
  },
  "services.registryCredentialEmpty": {
    message: "사용할 수 있는 저장된 레지스트리 자격 증명이 없습니다.",
    description: "Registry credential selector empty state",
  },
  "services.registryCredentialListError": {
    message: "레지스트리 자격 증명을 불러오지 못했습니다.",
    description: "Registry credential selector load error",
  },
  "services.registryCredentialManage": {
    message: "레지스트리 자격 증명 관리",
    description: "Link from a service credential selector to account settings",
  },
  "services.registryCredentialSettingsTitle": {
    message: "이미지 레지스트리 자격 증명",
    description: "Service Settings private-image credential card title",
  },
  "services.registryCredentialSettingsDescription": {
    message:
      "이 비공개 이미지 또는 Dockerfile 베이스 이미지를 가져올 때 사용할 저장된 자격 증명을 선택하세요.",
    description: "Service Settings private-image credential card description",
  },
  "services.registryCredentialSave": {
    message: "자격 증명 저장",
    description: "Service Settings registry credential save action",
  },
  "services.registryCredentialSaved": {
    message: "레지스트리 자격 증명을 업데이트했습니다.",
    description: "Toast after attaching or changing a registry credential",
  },
  "services.registryCredentialCleared": {
    message: "레지스트리 자격 증명을 해제했습니다.",
    description: "Toast after clearing a registry credential",
  },
  "services.registryCredentialError": {
    message: "레지스트리 자격 증명을 업데이트하지 못했습니다.",
    description: "Toast after a registry credential update fails",
  },
  "services.headerRegistryCredential": {
    message: "레지스트리 자격 증명:",
    description: "Service detail header label for a bound registry credential",
  },
  "services.createSettingsTitle": {
    message: "설정",
    description: "Create-wizard settings section heading",
  },
  "services.createFieldName": {
    message: "이름",
    description: "Create-wizard name input label",
  },
  "services.createFieldNamePlaceholder": {
    message: "my-service",
    description: "Create-wizard name input placeholder",
  },
  "services.createFieldNameError": {
    message: "줄 바꿈이나 제어 문자 없이 100자 이내의 이름을 입력하세요.",
    description: "Create-wizard name validation message",
  },
  "services.createFieldNameResourceName": {
    message: "리소스 이름: {name}",
    description:
      "Create-wizard preview of the resource name (slug) bex derives from a free-form service name (w5/m153)",
  },
  "services.createFieldNameTaken": {
    message: "이미 사용 중인 이름입니다",
    description:
      "Create-wizard inline error when the service name is already taken in the current workspace (w4/m19)",
  },
  "services.createFieldNameUseSuggestion": {
    message: "{name} 사용",
    description:
      "Create-wizard button offering the suggested free name in place of a taken one (w4/m19)",
  },
  "services.createFieldNameChecking": {
    message: "사용 가능 여부 확인 중…",
    description:
      "Create-wizard transient message while the debounced name-availability check is in flight (w4/m19)",
  },
  "services.createFieldBranch": {
    message: "브랜치",
    description: "Create-wizard branch input label (git sources)",
  },
  "services.createFieldBranchPlaceholder": {
    message: "저장소 기본 브랜치",
    description:
      "Create-wizard branch input placeholder when the repo's default is not known client-side: a blank branch builds the repository's default branch (w8/m76)",
  },
  "services.createFieldRootDir": {
    message: "루트 디렉터리",
    description: "Create-wizard root-directory input label",
  },
  "services.createFieldRootDirPlaceholder": {
    message: "예: backend",
    description: "Create-wizard root-directory input placeholder",
  },
  "services.createFieldRootDirHint": {
    message: "빌드할 하위 디렉터리입니다. 비워 두면 저장소 루트를 사용합니다.",
    description: "Create-wizard root-directory input hint text",
  },
  "services.createFieldRuntime": {
    message: "런타임",
    description: "Create-wizard Render-compatible runtime selector label",
  },
  "services.createFieldBuildCommand": {
    message: "빌드 명령",
    description: "Create-wizard Render-compatible build command label",
  },
  "services.createFieldStartCommand": {
    message: "시작 명령",
    description: "Create-wizard Render-compatible start command label",
  },
  "services.createFieldDockerfilePath": {
    message: "Dockerfile 경로",
    description: "Create-wizard Dockerfile-path label for the Docker runtime",
  },
  "services.createFieldDockerfilePathPlaceholder": {
    message: "Dockerfile",
    description: "Create-wizard Dockerfile-path placeholder",
  },
  "services.createFieldDockerfilePathHint": {
    message:
      "루트 디렉터리 기준 경로입니다. 비워 두면 Dockerfile을 사용합니다.",
    description: "Create-wizard Dockerfile-path help text",
  },
  "services.createFieldDockerCommand": {
    message: "Docker 명령",
    description: "Create-wizard Docker CMD override label (Render wording)",
  },
  "services.createFieldDockerCommandPlaceholder": {
    message: "Dockerfile의 CMD 사용",
    description: "Create-wizard optional Docker Command placeholder",
  },
  "services.imageCommandPlaceholder": {
    message: "이미지의 기본 명령 사용",
    description:
      "Create-wizard and settings placeholder for an Existing Image service's optional Docker Command",
  },
  "services.imageCommandHint": {
    message:
      "이미지의 기본 명령(CMD)을 재정의합니다. 비워 두면 기본값을 사용합니다.",
    description: "Existing Image service Settings: Docker Command help text",
  },
  "services.createRuntimeNode": {
    message: "Node",
    description: "Create-wizard Node runtime option",
  },
  "services.createRuntimePython": {
    message: "Python 3",
    description: "Create-wizard Python runtime option",
  },
  "services.createRuntimeGo": {
    message: "Go",
    description: "Create-wizard Go runtime option",
  },
  "services.createRuntimeRuby": {
    message: "Ruby",
    description: "Create-wizard Ruby runtime option",
  },
  "services.createRuntimeRust": {
    message: "Rust",
    description: "Create-wizard Rust runtime option",
  },
  "services.createRuntimeElixir": {
    message: "Elixir",
    description: "Create-wizard Elixir runtime option",
  },
  "services.createRuntimeDocker": {
    message: "Docker",
    description: "Create-wizard Docker runtime option",
  },
  "services.createFieldPlan": {
    message: "컴퓨팅 플랜",
    description: "Create-wizard plan-picker section label",
  },
  "services.createFieldAutoDeploy": {
    message: "푸시 시 자동 배포",
    description: "Create-wizard auto-deploy toggle label",
  },
  "services.createFieldAutoDeployHint": {
    message: "이 브랜치에 푸시하면 자동으로 다시 배포합니다.",
    description: "Create-wizard auto-deploy toggle hint text",
  },
  "services.createCancel": {
    message: "취소",
    description: "Create-wizard cancel button",
  },
  "services.createSubmit": {
    message: "서비스 배포",
    description: "Create-wizard submit button",
  },
  "services.createSuccess": {
    message: "{name} 배포 중…",
    description: "Toast shown after createService succeeds",
  },
  "services.createError": {
    message: "{name} 서비스를 생성하지 못했습니다. 다시 시도하세요.",
    description: "Toast shown after createService fails",
  },
  "services.scaleSuccess_other": {
    message: "인스턴스 {count}개로 확장하는 중…",
    description:
      "Toast acknowledging that scaleService accepted the desired count; convergence is still asynchronous (plural)",
  },
  "services.scaleError": {
    message: "인스턴스 수를 업데이트하지 못했습니다.",
    description: "Toast shown after scaleService fails",
  },
  "services.createTypePickerTitle": {
    message: "서비스 유형",
    description: "Label above the service type picker in the create wizard",
  },
  "services.createTypeWebDesc": {
    message: "서비스를 공개 URL로 노출합니다",
    description:
      "Description shown under the Web Service type card in the create wizard",
  },
  "services.createTypePrivateDesc": {
    message: "플랫폼 네트워크 내부에서만 접근할 수 있습니다",
    description: "Description shown under the Private Service type card",
  },
  "services.createTypeWorkerDesc": {
    message: "포트나 URL 없이 백그라운드 작업을 처리합니다",
    description: "Description shown under the Background Worker type card",
  },
  "services.createTypeCronDesc": {
    message: "반복 일정에 따라 명령을 실행합니다",
    description: "Description shown under the Cron Job type card",
  },
  "services.createTypeStaticDesc": {
    message: "정적 사이트를 빌드하여 오브젝트 스토리지에서 제공합니다",
    description: "Description shown under the Static Site type card",
  },
  "services.createFieldSchedule": {
    message: "일정",
    description: "Label for the cron schedule field in the create wizard",
  },
  "services.createFieldSchedulePlaceholder": {
    message: "0 0 * * *",
    description: "Placeholder for the cron schedule field",
  },
  "services.createFieldScheduleHint": {
    message:
      "5개 필드로 이루어진 crontab 표현식(분 시 일 월 요일)입니다. UTC 기준으로 실행됩니다.",
    description: "Hint text under the schedule field",
  },
  "services.createFieldSchedulePreview": {
    message: "{description} · UTC 기준 실행",
    description:
      "Live human-readable description of a valid schedule, shown under the field",
  },
  "services.createFieldScheduleError": {
    message: "올바른 5개 필드 cron 표현식을 입력하세요. 예: 0 0 * * *",
    description: "Validation error for an invalid cron expression",
  },
  "services.createFieldCommand": {
    message: "명령",
    description: "Label for the command field in the create wizard",
  },
  "services.createFieldCommandPlaceholder": {
    message: "python script.py",
    description: "Placeholder for the command field",
  },
  "services.createFieldCommandHint": {
    message: "예약된 실행마다 수행할 명령입니다.",
    description: "Hint text under the command field",
  },
  "services.createFieldPublishPath": {
    message: "게시 디렉터리",
    description: "Label for the publish directory field in the create wizard",
  },
  "services.createFieldPublishPathPlaceholder": {
    message: "예: dist",
    description: "Placeholder for the publish directory field",
  },
  "services.createFieldPublishPathHint": {
    message:
      "빌드된 사이트가 있는 폴더로, 루트 디렉터리 기준 경로입니다(예: dist, 루트 디렉터리 자체는 .).",
    description: "Hint text under the publish directory field",
  },
  "services.createFieldPublishPathRequired": {
    message:
      "필수 항목입니다. 사이트 루트로 제공할 빌드 결과물 디렉터리를 루트 디렉터리 기준 경로로 입력하세요(예: dist, build, public, 또는 루트 디렉터리 자체를 뜻하는 . — 루트 디렉터리가 비어 있으면 저장소 루트).",
    description:
      "Hint under the publish directory field while it is blank: the field is required, the placeholder is only an example",
  },
  "services.createNoPublicUrlNote": {
    message: "이 서비스 유형에는 공개 URL이 없습니다.",
    description:
      "Note shown for private/worker types that don't produce a public URL",
  },
  "services.createFieldEnvVarsTitle": {
    message: "환경 변수",
    description: "Section heading for env vars in the create wizard",
  },
  "services.createFieldEnvVarsAdd": {
    message: "변수 추가",
    description: "Button to add an env var row in the create wizard",
  },
  "services.envRowKey": {
    message: "변수 {n} 키",
    description:
      "A create-flow variable row's key field (aria-label), named by its row as its Remove button is (w4/544)",
  },
  "services.envRowValue": {
    message: "변수 {n} 값",
    description:
      "A create-flow variable row's value field (aria-label), named by its row (w4/544)",
  },
  "services.envRowGenerate": {
    message: "변수 {n}의 값 생성",
    description:
      "A create-flow variable row's Generate button (aria-label), named by its row (w4/544)",
  },
  "services.envRowRemove": {
    message: "변수 {n} 제거",
    description:
      "A create-flow variable row's Remove button (aria-label), named by its row (w4/544)",
  },
  "services.fileRowName": {
    message: "시크릿 파일 {n}의 파일 이름",
    description:
      "A create-flow secret file row's name field (aria-label), named by its row and starting with the dialog's visible \"File name\" label (w4/544)",
  },
  "services.fileRowContent": {
    message: "시크릿 파일 {n} 내용",
    description:
      "A create-flow secret file row's contents field (aria-label), named by its row (w4/544)",
  },
  "services.fileRowRemove": {
    message: "시크릿 파일 {n} 제거",
    description:
      "A create-flow secret file row's Remove button (aria-label), named by its row (w4/544)",
  },
  "services.createFieldEnvVarsKey": {
    message: "키",
    description: "Label for the env var key column in the create wizard",
  },
  "services.createFieldEnvVarsValue": {
    message: "값",
    description: "Label for the env var value column in the create wizard",
  },
  "services.createFieldEnvVarsKeyPlaceholder": {
    message: "KEY_NAME",
    description: "Placeholder for the env var key input in the create wizard",
  },
  "services.createFieldEnvVarsValuePlaceholder": {
    message: "value",
    description: "Placeholder for the env var value input in the create wizard",
  },
  "services.createFieldEnvVarsKeyError": {
    message:
      "영문자, 숫자, 점, 대시, 밑줄을 최대 253자까지 사용할 수 있습니다. '.'이거나 '..'으로 시작하면 안 됩니다.",
    description:
      "Error shown when an env var key is invalid in the create wizard",
  },
  "services.createFieldSecretFilesTitle": {
    message: "시크릿 파일",
    description: "Section heading for secret files in the create wizard",
  },
  "services.createFieldSecretFilesHint": {
    message: "첫 배포부터 /etc/secrets 아래에 읽기 전용으로 마운트됩니다.",
    description: "Hint for create-time secret files",
  },
  "services.createFieldSecretFilesAdd": {
    message: "시크릿 파일 추가",
    description: "Button to add a create-time secret file",
  },
  "services.createFieldSecretFilesNamePlaceholder": {
    message: "credentials.json",
    description: "Placeholder for a secret file name",
  },
  "services.createFieldSecretFilesContentPlaceholder": {
    message: "시크릿 내용을 붙여넣으세요",
    description: "Placeholder for secret file contents",
  },
  "services.createFieldSecretFilesNameError": {
    message:
      "영문자, 숫자, 점, 대시, 밑줄만 사용할 수 있으며 . 와 .. 는 허용되지 않습니다.",
    description: "Invalid secret file name error",
  },
  "services.createFieldEnvironmentTitle": {
    message: "프로젝트 및 환경",
    description: "Create wizard grouping section title",
  },
  "services.createFieldProject": {
    message: "프로젝트",
    description: "Accessible label for the create project picker",
  },
  "services.createFieldProjectNone": {
    message: "프로젝트 없음",
    description: "Unassigned option in the project picker",
  },
  "services.createFieldEnvironment": {
    message: "환경",
    description: "Accessible label for the create environment picker",
  },
  "services.createFieldEnvironmentNone": {
    message: "환경 없음",
    description: "Unassigned option in the environment picker",
  },
  "services.createFieldEnvironmentHint": {
    message:
      "환경을 선택하면 해당 환경의 상위 프로젝트에도 서비스가 추가됩니다.",
    description: "Hint for create-time environment assignment",
  },
  "services.navEvents": {
    message: "이벤트",
    description: "Service-detail nav item (events tab)",
  },
  "services.navDeploys": {
    message: "배포",
    description:
      "Service-detail nav item (dedicated deploy-history tab, w9/002)",
  },
  "services.navRuns": {
    message: "실행",
    description:
      "Service-detail nav item for a cron job: its run history, Render's Runs page (w2/m360)",
  },
  "services.navDisk": {
    message: "디스크",
    description:
      "Service-detail nav item (persistent disk page); Render's label is singular",
  },
  "services.diskDescription": {
    message:
      "SSD를 연결하면 배포 간에도 서비스의 파일 시스템 데이터가 유지됩니다. 디스크는 월 $0.175/GB가 청구됩니다.",
    description:
      "Disk tab card description; bex's rate is 30% below Render's $0.25",
  },
  "services.diskEmptyTitle": {
    message: "연결된 디스크 없음",
    description: "Disk tab empty state title",
  },
  "services.diskEmptyBody": {
    message:
      "디스크 마운트 경로 아래의 파일만 배포와 재시작 후에도 유지됩니다.",
    description: "Disk tab empty state body",
  },
  "services.diskPaidOnly": {
    message:
      "디스크를 사용하려면 유료 컴퓨팅 플랜이 필요합니다. 디스크를 연결하려면 이 서비스의 요금제를 업그레이드하세요.",
    description: "Shown instead of the add button on a free service",
  },
  "services.diskAddAction": {
    message: "디스크 추가",
    description: "Button that opens and submits the add-disk form",
  },
  "services.diskLoadErrorTitle": {
    message: "이 서비스의 디스크를 불러올 수 없습니다",
    description: "Disk tab error state title",
  },
  "services.diskWarningsTitle": {
    message: "다음 사항을 확인하세요.",
    description: "Heading above the five add-disk warnings (Render's wording)",
  },
  "services.diskWarningZeroDowntime": {
    message: "디스크를 연결하면 이 서비스의 무중단 배포가 비활성화됩니다.",
    description: "Add-disk warning 1",
  },
  "services.diskWarningSingleInstance": {
    message: "디스크가 연결된 서비스는 여러 인스턴스로 확장할 수 없습니다.",
    description: "Add-disk warning 2",
  },
  "services.diskWarningOnePerService": {
    message: "서비스당 최대 1개의 디스크만 연결할 수 있습니다.",
    description: "Add-disk warning 3",
  },
  "services.diskWarningMountPathOnly": {
    message: "디스크 마운트 경로 아래의 파일만 유지됩니다.",
    description: "Add-disk warning 4",
  },
  "services.diskWarningNoSharing": {
    message: "다른 서비스는 이 서비스의 디스크에 접근할 수 없습니다.",
    description: "Add-disk warning 5",
  },
  "services.diskMountPathLabel": {
    message: "마운트 경로",
    description: "Add-disk form field label",
  },
  "services.diskMountPathHint": {
    message:
      "디스크의 절대 마운트 경로입니다. 이 경로 아래의 파일만 배포 간에 유지됩니다. 루트 디렉터리(/)는 사용할 수 없습니다.",
    description: "Add-disk mount path helper text",
  },
  "services.diskMountPathRequired": {
    message: "마운트 경로를 입력하세요.",
    description: "Client-side validation: empty mount path",
  },
  "services.diskMountPathAbsolute": {
    message: "마운트 경로는 절대 경로(/로 시작)여야 합니다.",
    description: "Client-side validation: relative mount path",
  },
  "services.diskMountPathNotRoot": {
    message: "마운트 경로는 루트 디렉터리이거나 슬래시로 끝날 수 없습니다.",
    description: "Client-side validation: root or trailing slash",
  },
  "services.diskMountPathReserved": {
    message:
      "해당 경로는 플랫폼에서 예약되어 있습니다. 대신 그 하위 디렉터리를 마운트하세요.",
    description: "Client-side validation: reserved mount path",
  },
  "services.diskSizeLabel": {
    message: "크기",
    description: "Add-disk size field label",
  },
  "services.diskSizeHint": {
    message:
      "나중에 크기를 늘릴 수는 있지만 줄일 수는 없습니다. 사용 사례에 맞는 가장 작은 값으로 시작하는 것을 권장합니다.",
    description: "Add-disk size helper text (Render's wording)",
  },
  "services.diskSizeChip": {
    message: "{size} GB",
    description: "Size quick-select chip and size display",
  },
  "services.diskAddSuccess": {
    message: "디스크를 연결했습니다. 서비스를 다시 배포하는 중입니다.",
    description: "Toast after a disk is attached",
  },
  "services.diskGrowLabel": {
    message: "크기 늘리기(GB)",
    description: "Grow control label on an attached disk",
  },
  "services.diskGrowAction": {
    message: "크기 늘리기",
    description: "Grow button",
  },
  "services.diskGrowHint": {
    message:
      "크기를 늘려도 다운타임이 발생하지 않으며, 추가된 스토리지는 몇 초 안에 사용할 수 있습니다. 디스크는 절대 줄일 수 없습니다.",
    description: "Grow helper text in Render's wording",
  },
  "services.diskResizeSuccess": {
    message: "디스크 크기를 늘렸습니다.",
    description: "Toast after a successful grow",
  },
  "services.diskDeleteAction": {
    message: "디스크 삭제",
    description: "Destructive action on an attached disk",
  },
  "services.diskDeleteTitle": {
    message: "이 디스크를 삭제할까요?",
    description: "Delete confirmation title",
  },
  "services.diskDeleteWarning": {
    message:
      "디스크의 모든 데이터가 사라지고 서비스는 즉시 디스크에 접근할 수 없게 됩니다. 스냅샷도 함께 삭제됩니다. 이 작업은 되돌릴 수 없습니다.",
    description: "Delete confirmation body",
  },
  "services.diskDeleteConfirm": {
    message: "디스크 삭제",
    description: "Delete confirmation action",
  },
  "services.diskDeleteSuccess": {
    message: "디스크를 삭제했습니다.",
    description: "Toast after a disk is deleted",
  },
  "services.diskSnapshotsTitle": {
    message: "스냅샷",
    description: "Snapshots card title",
  },
  "services.diskSnapshotsDescription": {
    message: "이 디스크의 스냅샷은 매일 생성되며 최소 7일간 보관됩니다.",
    description: "Snapshots card description",
  },
  "services.diskSnapshotsNotConfiguredTitle": {
    message: "스냅샷이 설정되지 않았습니다",
    description:
      "Snapshots card heading when the deployment has no snapshot object store configured (not a failure).",
  },
  "services.diskSnapshotsNotConfiguredBody": {
    message:
      "이 bex 배포에는 디스크 스냅샷용 오브젝트 스토어가 구성되어 있지 않아 스냅샷이 생성되지 않습니다. 데이터에는 영향이 없습니다. 운영자에게 스냅샷 활성화를 요청하세요.",
    description:
      "Explains that snapshots are unconfigured rather than broken, and that an operator enables them.",
  },
  "services.diskSnapshotsEmptyTitle": {
    message: "아직 스냅샷이 없습니다",
    description: "Snapshots empty state title",
  },
  "services.diskSnapshotsEmptyBody": {
    message: "첫 스냅샷은 다음 야간 실행 때 생성됩니다.",
    description: "Snapshots empty state body",
  },
  "services.diskSnapshotsFailedTitle": {
    message: "마지막 스냅샷 실패",
    description: "A disk's most recent nightly snapshot attempt failed",
  },
  "services.diskSnapshotsFailedReadBody": {
    message:
      "디스크의 일부를 읽을 수 없어 스냅샷이 생성되지 않았으며 최근 변경 사항이 백업되지 않았습니다. 기존 스냅샷에는 영향이 없습니다. 다시 발생하면 지원팀에 문의하세요.",
    description: "Failed snapshot: the volume could not be read",
  },
  "services.diskSnapshotsFailedUploadBody": {
    message:
      "스냅샷을 스토리지에 저장하지 못해 최근 변경 사항이 백업되지 않았습니다. 기존 스냅샷에는 영향이 없으며 다음 야간 실행 때 다시 시도합니다.",
    description: "Failed snapshot: the object store did not accept it",
  },
  "services.diskSnapshotsFailedConfigBody": {
    message:
      "스냅샷 작업이 올바르게 시작되지 않아 최근 변경 사항이 백업되지 않았습니다. 기존 스냅샷에는 영향이 없습니다. 지원팀에 문의하세요.",
    description: "Failed snapshot: the job was misconfigured",
  },
  "services.diskSnapshotsFailedOtherBody": {
    message:
      "마지막 스냅샷 시도가 실패하여 최근 변경 사항이 백업되지 않았습니다. 기존 스냅샷에는 영향이 없습니다. 다시 발생하면 지원팀에 문의하세요.",
    description: "Failed snapshot: an unrecognized stage",
  },
  "services.diskSnapshotsOverdueTitle": {
    message: "최근 스냅샷 없음",
    description: "A disk's nightly snapshot did not arrive when expected",
  },
  "services.diskSnapshotsOverdueBody": {
    message:
      "{dueBy}까지 생성되어야 할 야간 스냅샷이 아직 생성되지 않아 이 디스크의 최근 변경 사항이 백업되지 않았습니다. 기존 스냅샷에는 영향이 없습니다. 계속되면 지원팀에 문의하세요.",
    description:
      "Explains an overdue nightly snapshot; {dueBy} is a local date/time",
  },
  "services.diskSnapshotsNotRunningTitle": {
    message: "스냅샷이 실행되지 않고 있습니다",
    description:
      "The platform could not schedule this disk's nightly snapshots",
  },
  "services.diskSnapshotsNotRunningBody": {
    message:
      "bex가 이 디스크의 야간 스냅샷을 예약하지 못해 새 변경 사항이 백업되지 않고 있습니다. 기존 스냅샷에는 영향이 없습니다. 지원팀에 문의하세요.",
    description:
      "Explains that nightly snapshots cannot be scheduled and whom to contact",
  },
  "services.diskSnapshotsUnavailableTitle": {
    message: "스냅샷을 사용할 수 없습니다",
    description: "Snapshots error state title",
  },
  "services.diskRestoreDatabaseWarning": {
    message:
      "디스크에서 실행 중인 데이터베이스를 복구하려고 디스크를 복원하지 마세요. 실행 중인 데이터베이스의 파일을 덮어쓰면 데이터베이스가 손상될 수 있습니다. 대신 해당 데이터베이스의 자체 백업 및 복원 기능을 사용하세요.",
    description:
      "Render's database-recovery warning, carried verbatim in meaning",
  },
  "services.diskRestoreAction": {
    message: "복원",
    description: "Restore button on a snapshot row",
  },
  "services.diskRestoreTitle": {
    message: "이 스냅샷을 복원할까요?",
    description: "Restore confirmation title",
  },
  "services.diskRestoreWarning": {
    message:
      "서비스가 중지되고, 디스크가 이 스냅샷으로 교체된 뒤 서비스가 다시 시작됩니다. 스냅샷 이후에 기록된 모든 데이터는 사라집니다. 이 작업은 되돌릴 수 없습니다.",
    description: "Restore confirmation body",
  },
  "services.diskRestoreConfirm": {
    message: "디스크 복원",
    description: "Restore confirmation action",
  },
  "services.diskUnsupportedType": {
    message:
      "영구 디스크는 마운트할 장기 실행 인스턴스가 필요하므로 Web Service, Private Service, Background Worker에서 사용할 수 있습니다.",
    description:
      "Empty-state body on the Disk tab for a service type that cannot have a disk.",
  },
  "services.diskSudoPrompt": {
    message: "계속하려면 다음을 입력하세요: {phrase}",
    description:
      "Label above the typed-confirmation input for an irreversible disk action.",
  },
  "services.diskConfigTitle": {
    message: "디스크 구성",
    description:
      "Heading of the card holding the disk's mount path and size settings.",
  },
  "services.diskConfigDescription": {
    message: "이 디스크의 마운트 경로와 크기를 구성합니다.",
    description: "Subtitle of the disk configuration card.",
  },
  "services.diskSizeUnit": {
    message: "GB",
    description: "Unit suffix shown beside the disk size input.",
  },
  "services.diskEditAction": {
    message: "편집",
    description: "Button that unlocks the disk size field for editing.",
  },
  "services.diskProvisionedLabel": {
    message: "프로비저닝됨 {size} GB",
    description:
      "Provisioned disk size shown beside the usage chart heading; this is the size billing meters.",
  },
  "services.diskDeleteCardTitle": {
    message: "디스크 삭제",
    description:
      "Heading of the delete-disk card. Distinct from diskDeleteTitle, which is the confirmation dialog's question.",
  },
  "services.diskDeleteCardDescription": {
    message:
      "디스크가 이 서비스에서 분리되고 디스크의 모든 데이터가 영구적으로 삭제됩니다.",
    description: "Subtitle of the delete-disk card, stating what is lost.",
  },
  "services.diskUsageTitle": {
    message: "디스크 사용량",
    description: "Heading of the disk usage chart on a service's Disk tab.",
  },
  "services.diskUsageDescription": {
    message:
      "마운트된 볼륨에 기록된 바이트 수를 프로비저닝된 용량과 비교해 보여 줍니다. 요금은 사용한 바이트가 아니라 프로비저닝된 크기를 기준으로 측정됩니다.",
    description:
      "Subtitle of the disk usage chart, stating that billing uses provisioned size.",
  },
  "services.diskRestoreStarted": {
    message: "디스크를 복원하는 중입니다. 완료될 때까지 서비스가 중지됩니다.",
    description: "Toast after a restore is requested",
  },
  "services.navShell": {
    message: "셸",
    description: "Service-detail nav item (running-instance SSH page)",
  },
  "services.navCompute": {
    message: "컴퓨팅",
    description:
      "Service sidebar link to the Compute page: the compute plan, manual scaling and autoscaling, as on Render (w6/m483)",
  },
  "services.eventsCount_other": {
    message: "이벤트 {count}개",
    description:
      "Accessible label for the number of visible service events (plural)",
  },
  "services.eventsCountAtLeast_other": {
    message: "이벤트 {count}개 이상",
    description:
      "Accessible label for the event count badge while older events remain unread (plural)",
  },
  "services.eventsEmptyTitle": {
    message: "아직 이벤트가 없습니다",
    description: "Events tab empty-state title",
  },
  "services.eventsEmpty": {
    message: "배포 및 서비스 변경 내역이 여기에 표시됩니다.",
    description: "Events tab empty-state description",
  },
  "services.eventsErrorTitle": {
    message: "이벤트를 표시할 수 없습니다",
    description: "Events tab query-error title",
  },
  "services.eventsErrorDescription": {
    message: "이 서비스의 이벤트를 불러오지 못했습니다.",
    description: "Events tab query-error description",
  },
  "services.eventsRetry": {
    message: "다시 시도",
    description: "Events tab query-error retry button",
  },
  "services.eventsLoadOlder": {
    message: "이전 이벤트 불러오기",
    description: "Button that loads the next page or earlier event window",
  },
  "services.eventsLoadingOlder": {
    message: "이전 이벤트를 불러오는 중…",
    description: "Busy label while an older service-event page is loading",
  },
  "services.eventsHistoryEnd": {
    message: "이 서비스의 기록 처음까지 모두 확인했습니다.",
    description: "Terminal label after all retained service events are loaded",
  },
  "services.eventsFilter": {
    message: "이벤트 필터",
    description: "Button opening the service event-type filter",
  },
  "services.eventsFilterSelected": {
    message: "이벤트 유형: {count}",
    description: "Event filter button with selected-type count",
  },
  "services.eventsFilterSearch": {
    message: "이벤트 유형 검색",
    description: "Search input for service event types",
  },
  "services.eventsFilterShowAll": {
    message: "모든 이벤트 표시",
    description:
      "Events tab: button in the filtered-to-nothing state that shows every event type again",
  },
  "services.eventsShownOf": {
    message: "{total}개 중 {shown}개 표시",
    description:
      "Events tab: count over the feed while a filter hides some rows; {total} is the loaded count, '20+' while older events remain",
  },
  "services.eventsFilterAll": {
    message: "모든 이벤트",
    description: "Master service event filter option",
  },
  "services.eventsFilterNoMatches": {
    message: "검색과 일치하는 이벤트 유형이 없습니다.",
    description: "Empty search result in the event filter",
  },
  "services.eventsFilterEmptyTitle": {
    message: "일치하는 이벤트 없음",
    description: "Title when loaded history has no selected event types",
  },
  "services.eventsFilterEmpty": {
    message: "이벤트 유형을 더 선택하거나 이전 기록을 불러오세요.",
    description: "Body when loaded history has no selected event types",
  },
  "services.eventsFilterNoneTitle": {
    message: "선택된 이벤트 유형이 없습니다",
    description:
      "Events tab: title when the filter has every event type deselected",
  },
  "services.eventsFilterNone": {
    message: "이벤트를 보려면 유형을 하나 이상 선택하세요.",
    description:
      "Events tab: body when the filter has every event type deselected",
  },
  "services.eventsFilterGroup.deploy": {
    message: "배포",
    description: "Deploy event filter group",
  },
  "services.eventsFilterGroup.serviceStatus": {
    message: "서비스 상태",
    description: "Service-status event filter group",
  },
  "services.eventsFilterGroup.scaling": {
    message: "확장",
    description: "Scaling event filter group",
  },
  "services.eventsFilterGroup.maintenanceMode": {
    message: "유지 보수 모드",
    description: "Maintenance-mode event filter group",
  },
  "services.eventsFilterGroup.edgeCache": {
    message: "엣지 캐시",
    description: "Edge-cache event filter group",
  },
  "services.eventsFilterGroup.configuration": {
    message: "구성",
    description: "bex configuration event filter group",
  },
  "services.eventsFilterGroup.other": {
    message: "기타",
    description:
      "Event filter group holding types the API returned that the dashboard catalog does not know yet",
  },
  "services.eventsActor": {
    message: "{actor}님",
    description: "Actor attribution shown on a service event",
  },
  "services.eventsDeployReference": {
    message: "배포 {id}",
    description: "Deploy identifier shown on a deploy activity row",
  },
  "services.eventsTriggerRollback": {
    message: "롤백",
    description: "Deploy event trigger: rollback",
  },
  "services.eventsTriggerFirstBuild": {
    message: "첫 배포",
    description: "Deploy event trigger: the service's first deploy",
  },
  "services.eventsTriggerManual": {
    message: "수동 배포",
    description: "Deploy event trigger: manual",
  },
  "services.eventsTriggerEnvUpdated": {
    message: "구성 변경",
    description:
      "Deploy event trigger: an environment change (env var, secret file, or env-group edit); a Settings save no longer sets envUpdated (w8/m135)",
  },
  "services.eventsTriggerDeployHook": {
    message: "Deploy Hook",
    description:
      "Deploy event trigger: a POST to the service's secret deploy-hook URL",
  },
  "services.eventsTriggerClearCache": {
    message: "캐시 삭제됨",
    description: "Deploy event trigger: build-cache clear",
  },
  "services.eventsTriggerDeployedByRender": {
    message: "플랫폼 배포",
    description: "Deploy event trigger: platform initiated",
  },
  "services.eventsTypeDeployStarted": {
    message: "배포 시작됨",
    description: "Service activity type: deploy started",
  },
  "services.eventsTypeDeployFinished": {
    message: "배포 종료됨",
    description: "Service activity type: deploy finished",
  },
  "services.eventsTypeZeroDowntimeRedeployStarted": {
    message: "무중단 재배포 시작됨",
    description:
      "Service activity type: a Bex-initiated zero-downtime deploy started for a service",
  },
  "services.eventsTypeZeroDowntimeRedeployEnded": {
    message: "무중단 재배포 종료됨",
    description:
      "Service activity type: a Bex-initiated zero-downtime deploy completed for a service",
  },
  "services.eventsRedeployTrigger.node_replacement": {
    message: "노드 교체됨",
    description:
      "zero_downtime_redeploy_started trigger: drain-guard rolled the service off a node being replaced",
  },
  "services.eventsRedeployTrigger.node_consolidation": {
    message: "오토스케일링으로 노드 제거됨",
    description:
      "zero_downtime_redeploy_started trigger: drain-guard rolled the service off a node marked for Cluster Autoscaler scale-down",
  },
  "services.eventsTypePipelineMinutesExhausted": {
    message: "파이프라인 시간 소진",
    description:
      "Service activity type: a build refused because the workspace used its pipeline minutes",
  },
  "services.eventsTypeEdgeCacheEnabled": {
    message: "엣지 캐시 활성화됨",
    description: "Service activity type: edge caching turned on",
  },
  "services.eventsTypeEdgeCacheDisabled": {
    message: "엣지 캐시 비활성화됨",
    description: "Service activity type: edge caching turned off",
  },
  "services.eventsTypeEdgeCachePurged": {
    message: "엣지 캐시 삭제됨",
    description: "Service activity type: edge cache purged",
  },
  "services.eventsTypeImagePullFailed": {
    message: "이미지 가져오기 실패",
    description: "Service activity type: image pull failed",
  },
  "services.eventsTypeArtifactFetchFailed": {
    message: "아티팩트 가져오기 실패",
    description:
      "Service activity type: a build published by a build source could not be fetched",
  },
  "services.eventsTypeArtifactSourceChanged": {
    message: "빌드 소스 변경됨",
    description:
      "Service activity type: the service's linked build source changed",
  },
  "services.eventsArtifactSourceChanged": {
    message: "빌드 소스 {from} → {to}",
    description:
      "Detail line on artifact_source_changed when switching between two build sources",
  },
  "services.eventsArtifactSourceLinked": {
    message: "빌드 소스 {to}에 연결됨",
    description:
      "Detail line on artifact_source_changed when newly linked to a build source",
  },
  "services.eventsArtifactSourceUnlinked": {
    message: "빌드 소스 {from} 연결 해제됨",
    description:
      "Detail line on artifact_source_changed when detached from a build source",
  },
  "services.eventsBuildReference": {
    message: "빌드 {id}",
    description:
      "Build identifier shown on an artifact_fetch_failed activity row",
  },
  "services.eventsTypeSuspended": {
    message: "일시 중지됨",
    description: "Service activity type: service suspended",
  },
  "services.eventsTypeResumed": {
    message: "재개됨",
    description: "Service activity type: service resumed",
  },
  "services.eventsTypeRestarted": {
    message: "서버 다시 시작됨",
    description: "Service activity type: service restarted",
  },
  "services.eventsTypeInstanceFailed": {
    message: "인스턴스 실패",
    description: "Service activity type: running instance became unhealthy",
  },
  "services.eventsTypeDiskUsageHigh": {
    message: "디스크 사용량 높음",
    description:
      "Service activity type: a service disk is over 80% full (service_disk_usage_high)",
  },
  "services.eventsTypeDiskUsageRecovered": {
    message: "디스크 사용량 정상화",
    description:
      "Service activity type: a service disk dropped back below 80% (service_disk_usage_recovered)",
  },
  "services.eventsDiskUsage": {
    message:
      "디스크 {disk}, {mountPath}: {usage}% 사용 중(임계값 {threshold}%)",
    description: "Disk usage event detail line",
  },
  "services.eventsTypeHardwareFailure": {
    message: "하드웨어 장애",
    description:
      "Service activity type: an instance was lost with the machine it ran on",
  },
  "services.eventsTypeServiceSuspended": {
    message: "서비스 일시 중지됨",
    description: "Service activity type: suspension converged",
  },
  "services.eventsTypeServiceResumed": {
    message: "서비스 재개됨",
    description: "Service activity type: resume converged",
  },
  "services.eventsTypeServiceHibernated": {
    message: "서비스가 절전 상태가 됨",
    description:
      "Service activity type: free-tier idle auto-hibernation — deliberately worded apart from a user-driven suspension",
  },
  "services.eventsTypeServiceWoken": {
    message: "서비스가 절전에서 깨어남",
    description:
      "Service activity type: a sleeping free-tier service woke on an incoming request",
  },
  "services.eventsTypeServiceRecovered": {
    message: "서비스 복구됨",
    description: "Service activity type: unhealthy service recovered",
  },
  "services.eventsTypePlanChanged": {
    message: "인스턴스 유형 변경됨",
    description: "Service activity type: plan changed",
  },
  "services.eventsTypeInstanceCountChanged": {
    message: "인스턴스 수 변경됨",
    description: "Service activity type: manual scale",
  },
  "services.eventsTypeAutoscalingChanged": {
    message: "자동 확장 구성 변경됨",
    description: "Service activity type: autoscaling configuration changed",
  },
  "services.eventsTypeAutoscalingStarted": {
    message: "자동 확장 시작됨",
    description: "Service activity type: autoscaler began a replica change",
  },
  "services.eventsTypeAutoscalingEnded": {
    message: "자동 확장 종료됨",
    description: "Service activity type: autoscaler replica change converged",
  },
  "services.eventsTypeBranchChanged": {
    message: "브랜치 변경됨",
    description: "Service activity type: tracked Git branch changed",
  },
  "services.eventsTypeCommitIgnored": {
    message: "커밋 무시됨",
    description: "Service activity type: matching Git push did not deploy",
  },
  "services.eventsTypeJobStarted": {
    message: "작업 시작됨",
    description: "Service activity type: one-off job started",
  },
  "services.eventsTypeJobCanceled": {
    message: "작업 취소됨",
    description: "Service activity type: one-off job canceled",
  },
  "services.eventsTypeJobRunEnded": {
    message: "작업 종료됨",
    description: "Service activity type: one-off job finished",
  },
  "services.eventsTypeBuildStarted": {
    message: "빌드 시작됨",
    description: "Service activity type: image build started",
  },
  "services.eventsTypeBuildEnded": {
    message: "빌드 종료됨",
    description: "Service activity type: image build finished",
  },
  "services.eventsTypePreDeployStarted": {
    message: "사전 배포 시작됨",
    description: "Service activity type: pre-deploy command started",
  },
  "services.eventsTypePreDeployEnded": {
    message: "사전 배포 종료됨",
    description: "Service activity type: pre-deploy command finished",
  },
  "services.eventsTypeBranchDeleted": {
    message: "브랜치 삭제됨",
    description: "Service activity type: tracked Git branch deleted",
  },
  "services.eventsTypeMaintenanceModeChanged": {
    message: "유지 보수 모드 구성 변경됨",
    description: "Service activity type: maintenance mode setting changed",
  },
  "services.eventsTypeMaintenanceModeUriUpdated": {
    message: "유지 보수 모드 URI 업데이트됨",
    description: "Service activity type: maintenance mode URI changed",
  },
  "services.eventsInstanceReference": {
    message: "인스턴스 {id}",
    description: "Instance identifier shown on a service event",
  },
  "services.eventsReplicaChange": {
    message: "인스턴스 {from} → {to}",
    description:
      "Instance count from/to summary: an autoscaling start/end's replicas or an instance_count_changed event's counts",
  },
  "services.eventsAutoscalingChange": {
    message: "자동 확장 {from} → {to}",
    description:
      "autoscaling_config_changed detail line; from/to are services.eventsAutoscalingOff or services.eventsAutoscalingRange (w4/568)",
  },
  "services.eventsAutoscalingOff": {
    message: "끔",
    description: "One side of an autoscaling change: autoscaling off (w4/568)",
  },
  "services.eventsAutoscalingRange": {
    message: "인스턴스 {min}–{max}개",
    description:
      "One side of an autoscaling change: its minimum–maximum instances (w4/568)",
  },
  "services.eventsAutoscalingRangeCPU": {
    message: "인스턴스 {min}–{max}개, CPU {cpu}%",
    description:
      "One side of an autoscaling change: its instance range and CPU target (w4/m335)",
  },
  "services.eventsAutoscalingRangeMemory": {
    message: "인스턴스 {min}–{max}개, 메모리 {memory}%",
    description:
      "One side of an autoscaling change: its instance range and memory target (w4/m335)",
  },
  "services.eventsAutoscalingRangeCPUMemory": {
    message: "인스턴스 {min}–{max}개, CPU {cpu}%, 메모리 {memory}%",
    description:
      "One side of an autoscaling change: its instance range and both targets (w4/m335)",
  },
  "services.eventsPlanChange": {
    message: "컴퓨팅 플랜 {from} → {to}",
    description:
      "plan_changed detail line, by the compute plans' names (w4/568, w6/m483)",
  },
  "services.eventsBranchChange": {
    message: "브랜치 {from} → {to}",
    description: "Tracked branch from/to summary",
  },
  "services.eventsAutoDeployDisabledReason.setting_change": {
    message: "설정에서 꺼짐",
    description:
      "auto_deploy_disabled reason: the auto-deploy setting was turned off (w4/m431)",
  },
  "services.eventsAutoDeployDisabledReason.rollback": {
    message: "롤백으로 꺼졌습니다. 이후 푸시가 이를 대체하지 않습니다",
    description:
      "auto_deploy_disabled reason: a dashboard rollback turned auto-deploy off (w4/m431)",
  },
  "services.eventsAutoDeployDisabledReason.manual_deploy": {
    message: "특정 커밋 배포로 꺼졌습니다. 이후 푸시가 이를 대체하지 않습니다",
    description:
      "auto_deploy_disabled reason: a deploy of a specific commit from the dashboard or a deploy hook turned auto-deploy off (w4/m431)",
  },
  "services.eventsBranchDeleted": {
    message: "브랜치 {branch} 삭제됨 — 자동 배포 비활성화",
    description: "Deleted tracked branch summary",
  },
  "services.eventsStatus.succeeded": {
    message: "성공",
    description: "Lifecycle-step outcome badge: succeeded",
  },
  "services.eventsStatus.failed": {
    message: "실패",
    description: "Lifecycle-step outcome badge: failed",
  },
  "services.eventsStatus.canceled": {
    message: "취소됨",
    description: "Lifecycle-step outcome badge: canceled",
  },
  "services.eventsReason.superseded": {
    message: "더 최신 배포가 이 배포를 대체했습니다.",
    description:
      "Deploy/build ended reason: a newer deploy superseded this one",
  },
  "services.eventsReason.image_pull_backoff": {
    message: "런타임이 구성된 이미지를 가져오지 못했습니다.",
    description: "Bounded public image-pull failure reason",
  },
  "services.eventsReason.readiness_failed": {
    message: "실행 중인 인스턴스가 헬스 체크에 실패했습니다.",
    description:
      "Bounded public readiness failure reason; names the health check, as Settings and REST do (w4/m327)",
  },
  "services.eventsReason.root_directory": {
    message: "서비스의 루트 디렉터리와 일치하는 변경 파일이 없습니다.",
    description: "Commit ignored because of root-directory scoping",
  },
  "services.eventsReason.build_filter": {
    message: "변경된 파일이 서비스의 빌드 필터를 통과하지 못했습니다.",
    description: "Commit ignored because of build filters",
  },
  "services.eventsReason.skip_phrase": {
    message: "커밋 메시지에서 이 배포를 건너뛰도록 요청했습니다.",
    description: "Commit ignored because of a Render skip phrase",
  },
  "services.eventsReason.non_zero_exit": {
    message: "실행이 상태 코드 {exitCode}(으)로 종료되었습니다.",
    description:
      "Cron run failure reason: the container exited non-zero (w4/m114)",
  },
  "services.eventsReason.oom_killed": {
    message: "실행 중 메모리가 부족하여 중지되었습니다.",
    description:
      "Cron run failure reason: the container was OOM-killed (w4/m114)",
  },
  "services.eventsReason.instance.non_zero_exit": {
    message:
      "인스턴스의 프로세스가 상태 코드 {exitCode}(으)로 종료되어 다시 시작되었습니다.",
    description:
      "Instance failed reason for a running service: its process crashed (w8/m158)",
  },
  "services.eventsReason.instance.oom_killed": {
    message: "인스턴스의 메모리({memory} 제한)가 부족하여 다시 시작되었습니다.",
    description:
      "Instance failed reason for a running service: killed at its plan's memory limit (w8/m158)",
  },
  "services.eventsReason.instance.oom_killed_no_limit": {
    message: "인스턴스의 메모리가 부족하여 다시 시작되었습니다.",
    description:
      "Instance failed reason for a running service: killed for memory, limit unknown (w8/m158)",
  },
  "services.eventsReason.early_exit": {
    message:
      "인스턴스의 프로세스가 종료되어 다시 시작되었습니다. 서비스는 계속 실행 중이어야 합니다.",
    description:
      "Instance failed reason for a running service: its process ended with status 0 (w8/m158)",
  },
  "services.eventsReason.evicted": {
    message: "실행이 완료되기 전에 노드에서 축출되었습니다.",
    description: "Cron run failure reason: the run's pod was evicted (w4/m114)",
  },
  "services.eventsReason.timed_out": {
    message: "실행이 12시간 제한에 도달하여 중지되었습니다.",
    description:
      "Cron run failure reason: the run exceeded its deadline (w4/m114)",
  },
  "services.eventsTypeCronRunStarted": {
    message: "Cron 실행 시작됨",
    description: "Service activity type: cron run started",
  },
  "services.eventsTypeCronRunFinished": {
    message: "Cron 실행 완료됨",
    description: "Service activity type: cron run finished",
  },
  "services.eventsTypeEnvVarsChanged": {
    message: "환경 변수 변경됨",
    description: "Service activity type: environment variables changed",
  },
  "services.eventsTypeEnvironmentChanged": {
    message: "환경 구성 변경됨",
    description:
      "Service activity type: environment variables and secret files changed",
  },
  "services.eventsTypeServiceMoved": {
    message: "서비스 이동됨",
    description:
      "Service activity type: the service was reassigned between projects or environments (service_moved) — distinct from the env-var fact above",
  },
  "services.eventsProjectChange": {
    message: "프로젝트 {from} → {to}",
    description:
      "service_moved detail line: previous and new project name (its id once it no longer exists), or the placement-none placeholder (w4/561)",
  },
  "services.eventsEnvironmentChange": {
    message: "환경 {from} → {to}",
    description:
      "service_moved detail line: previous and new environment name (its id once it no longer exists), or the placement-none placeholder (w4/561)",
  },
  "services.eventsPlacementNone": {
    message: "없음",
    description:
      "Placeholder for an absent side of a service_moved event (assigned from / removed to nothing) or of a plan_changed event (no plan before)",
  },
  "services.eventsTypeEnvGroupLinked": {
    message: "환경 그룹 연결됨",
    description: "Service activity type: environment group linked",
  },
  "services.eventsTypeEnvGroupUnlinked": {
    message: "환경 그룹 연결 해제됨",
    description: "Service activity type: environment group unlinked",
  },
  "services.eventsEnvGroupLinkedNamed": {
    message: "환경 그룹 {name} 연결됨",
    description:
      "Service activity title for env_group_linked naming the group, by its name at the time (w4/563)",
  },
  "services.eventsEnvGroupUnlinkedNamed": {
    message: "환경 그룹 {name} 연결 해제됨",
    description:
      "Service activity title for env_group_unlinked naming the group, by its name at the time (w4/563)",
  },
  "services.eventsCustomDomainAddedNamed": {
    message: "사용자 지정 도메인 {name} 추가됨",
    description:
      "Service activity title for custom_domain_added naming the domain's hostname (w4/m337)",
  },
  "services.eventsCustomDomainRemovedNamed": {
    message: "사용자 지정 도메인 {name} 제거됨",
    description:
      "Service activity title for custom_domain_removed naming the domain's hostname (w4/m337)",
  },
  "services.eventsCustomDomainVerifiedNamed": {
    message: "사용자 지정 도메인 {name} 확인됨",
    description:
      "Service activity title for custom_domain_verified naming the domain's hostname (w4/m337)",
  },
  "services.eventsDisplayNameChange": {
    message: "이름 {from} → {to}",
    description:
      "display_name_changed detail line: the service's name before and after the rename (w4/m337)",
  },
  "services.eventsTypeAutoDeployChanged": {
    message: "자동 배포 업데이트됨",
    description: "Service activity type: auto-deploy setting changed",
  },
  "services.eventsTypeIdleTimeoutChanged": {
    message: "유휴 시간 제한 업데이트됨",
    description: "Service activity type: idle timeout changed",
  },
  "services.eventsTypeDisplayNameChanged": {
    message: "표시 이름 변경됨",
    description: "Service activity type: display name changed",
  },
  "services.eventsTypeCustomDomainAdded": {
    message: "사용자 지정 도메인 추가됨",
    description: "Service activity type: custom domain added",
  },
  "services.eventsTypeCustomDomainRemoved": {
    message: "사용자 지정 도메인 제거됨",
    description: "Service activity type: custom domain removed",
  },
  "services.eventsTypeCustomDomainVerified": {
    message: "사용자 지정 도메인 확인됨",
    description: "Service activity type: custom domain verified",
  },
  "services.eventsTypeDiskCreated": {
    message: "디스크 생성됨",
    description: "Service activity type: persistent disk created",
  },
  "services.eventsTypeDiskUpdated": {
    message: "디스크 업데이트됨",
    description: "Service activity type: persistent disk updated",
  },
  "services.eventsTypeDiskDeleted": {
    message: "디스크 삭제됨",
    description: "Service activity type: persistent disk deleted",
  },
  "services.eventsTypeDiskSnapshotRestored": {
    message: "디스크 스냅샷 복원됨",
    description:
      "Service activity type: persistent disk restored from a snapshot",
  },
  "services.eventsTypeNotificationsChanged": {
    message: "실패 알림 업데이트됨",
    description: "Service activity type: failure notification setting changed",
  },
  "services.eventsTypeSubdomainPolicyChanged": {
    message: "플랫폼 서브도메인 업데이트됨",
    description: "Service activity type: platform subdomain policy changed",
  },
  "services.eventsTypeStaticSiteChanged": {
    message: "Static Site 설정 변경됨",
    description: "Service activity type: static-site configuration changed",
  },
  "services.eventsFilterType.autoDeployEnabled": {
    message: "자동 배포 활성화됨",
    description: "Events filter option for the auto deploy enabled event type",
  },
  "services.eventsFilterType.autoDeployDisabled": {
    message: "자동 배포 비활성화됨",
    description: "Events filter option for the auto deploy disabled event type",
  },
  "services.eventsFilterType.autoDeployChanged": {
    message: "자동 배포 업데이트됨(레거시)",
    description: "Events filter option for the auto deploy changed event type",
  },
  "services.eventsFilterType.publishPathChanged": {
    message: "게시 디렉터리 변경됨",
    description: "Events filter option for the publish path changed event type",
  },
  "services.eventsFilterType.routesChanged": {
    message: "리디렉션 및 리라이트 변경됨",
    description: "Events filter option for the routes changed event type",
  },
  "services.eventsFilterType.headersChanged": {
    message: "헤더 변경됨",
    description: "Events filter option for the headers changed event type",
  },
  "services.eventsFilterType.rootDirectoryChanged": {
    message: "루트 디렉터리 변경됨",
    description:
      "Events filter option for the root directory changed event type",
  },
  "services.eventsFilterType.dockerfilePathChanged": {
    message: "Dockerfile 경로 변경됨",
    description:
      "Events filter option for the dockerfile path changed event type",
  },
  "services.eventsFilterType.dockerContextChanged": {
    message: "Docker 빌드 컨텍스트 변경됨",
    description:
      "Events filter option for the docker context changed event type (w8/m128)",
  },
  "services.eventsFilterType.buildFilterChanged": {
    message: "빌드 필터 변경됨",
    description: "Events filter option for the build filter changed event type",
  },
  "services.eventsFilterType.commandsChanged": {
    message: "명령 변경됨",
    description: "Events filter option for the commands changed event type",
  },
  "services.eventsFilterType.sourceChanged": {
    message: "소스 변경됨",
    description: "Events filter option for the source changed event type",
  },
  "services.eventsFilterType.preDeployCommandChanged": {
    message: "사전 배포 명령 변경됨",
    description:
      "Events filter option for the pre deploy command changed event type",
  },
  "services.eventsFilterType.maxShutdownDelayChanged": {
    message: "최대 종료 지연 시간 변경됨",
    description:
      "Events filter option for the max shutdown delay changed event type",
  },
  "services.eventsFilterType.deployHookRegenerated": {
    message: "Deploy Hook 재생성됨",
    description:
      "Events filter option for the deploy hook regenerated event type",
  },
  "services.eventsTypeBuildSettingsChanged": {
    message: "빌드 및 배포 설정 변경됨",
    description: "Service activity type: build or deploy configuration changed",
  },
  "services.eventsTypePortChanged": {
    message: "포트 변경됨",
    description: "Service activity type: the service's listening port changed",
  },
  "services.eventsTypeServiceChanged": {
    message: "서비스 설정 변경됨",
    description: "Fallback service activity type",
  },
  "services.eventsManualDeploy": {
    message: "수동 배포",
    description: "Button to trigger a new deploy",
  },
  "services.deployMenuLatestCommit": {
    message: "최신 커밋 배포",
    description:
      "Manual Deploy dropdown item, repo-backed service: rebuild and redeploy from the branch's HEAD",
  },
  "services.deployMenuLatestImage": {
    message: "최신 이미지 배포",
    description:
      "Manual Deploy dropdown item, image-backed service (no repo to rebuild from)",
  },
  "services.deployMenuClearCache": {
    message: "빌드 캐시 삭제 후 배포",
    description:
      "Manual Deploy dropdown item (Render parity): rebuild without importing prior build-cache layers when registry caching is enabled; still exports a fresh cache for later deploys",
  },
  "services.deployMenuRestart": {
    message: "서비스 다시 시작",
    description:
      "Manual Deploy dropdown item: restart on the commit or image that is running; never deploys newer commits",
  },
  "services.deployMenuSpecificCommit": {
    message: "특정 커밋 배포",
    description:
      "Manual Deploy dropdown item, repo-backed service (Render parity): deploy a commit by SHA and turn auto-deploy off",
  },
  "services.deployCommitTitle": {
    message: "특정 커밋 배포",
    description: "Title of the dialog that deploys a commit by SHA",
  },
  "services.deployCommitBody": {
    message:
      "입력한 커밋을 배포하고 자동 배포를 끕니다. 이후 푸시가 이 배포를 대체하지 않습니다. 설정에서 자동 배포를 다시 켤 수 있습니다.",
    description:
      "Body of the deploy-a-specific-commit dialog. Render's dashboard disables automatic deploys when a specific commit is deployed",
  },
  "services.deployCommitLabel": {
    message: "커밋 SHA",
    description:
      "Label of the commit SHA input in the deploy-a-specific-commit dialog",
  },
  "services.deployCommitInvalid": {
    message: "커밋 SHA를 입력하세요. 7~40자의 16진수 문자여야 합니다.",
    description: "Inline error when the entered commit SHA is malformed",
  },
  "services.deployCommitConfirm": {
    message: "커밋 배포",
    description: "Confirm button of the deploy-a-specific-commit dialog",
  },
  "services.restartServiceSuccess": {
    message: "실행 중인 릴리스에서 다시 시작을 시작했습니다.",
    description: "Toast after the header's Restart service is accepted",
  },
  "services.restartServiceError": {
    message: "서비스를 다시 시작하지 못했습니다.",
    description:
      "Toast when the header's Restart service fails and the server gave no reason",
  },
  "services.eventsCancelDeploy": {
    message: "취소",
    description: "Button to cancel an in-progress deploy",
  },
  "services.eventsCancelConfirmTitle": {
    message: "이 배포를 취소할까요?",
    description: "Cancel deploy confirm dialog title",
  },
  "services.eventsCancelConfirmBody": {
    message:
      "진행 중인 배포가 중지됩니다. 이전 배포가 성공했다면 그 배포가 계속 라이브 상태로 유지되고, 그렇지 않다면 제공되는 서비스가 없게 됩니다. 저장한 변경 사항은 그대로 유지되며 다음 배포에 반영됩니다.",
    description:
      "Cancel deploy confirm dialog body when it is unknown whether an earlier deploy is live, so both outcomes are stated (w4/334)",
  },
  "services.eventsCancelConfirmBodyFirstDeploy": {
    message:
      "진행 중인 배포가 중지됩니다. 이 서비스의 첫 배포이므로 아직 라이브 버전이 없습니다. 취소하면 제공되는 서비스가 없게 됩니다.",
    description:
      "Cancel deploy confirm dialog body — the service's first deploy, nothing is serving yet (w4/103)",
  },
  "services.eventsRollback": {
    message: "롤백",
    description: "Button to roll back to a specific deploy",
  },
  "services.eventsRollbackConfirmTitle": {
    message: "이 배포로 롤백할까요?",
    description: "Rollback confirm dialog title",
  },
  "services.eventsRollbackConfirmBody": {
    message:
      "서비스가 이 배포의 이미지와 보존된 구성으로 실행됩니다. 저장된 설정은 변경되지 않으며 다음 표준 배포 때 적용됩니다. 자동 배포가 꺼집니다. 보존된 구성이 없는 이전 배포는 이미지만 복원합니다.",
    description: "Rollback confirm dialog body",
  },
  "services.eventsRollbackConfirmBodyCommit": {
    message:
      "서비스가 이 배포의 이미지({commit}에서 빌드됨)와 보존된 구성으로 실행됩니다. 저장된 설정은 변경되지 않으며 다음 표준 배포 때 적용됩니다. 자동 배포가 꺼집니다. 보존된 구성이 없는 이전 배포는 이미지만 복원합니다.",
    description:
      "Rollback confirm dialog body naming the commit being restored",
  },
  "services.eventsRollbackConfirmBodyNoAutoDeploy": {
    message:
      "서비스가 이 배포의 이미지와 보존된 구성으로 실행됩니다. 저장된 설정은 변경되지 않으며 다음 표준 배포 때 적용됩니다. 보존된 구성이 없는 이전 배포는 이미지만 복원합니다.",
    description:
      "Rollback confirm dialog body when the service's auto-deploy is already off (always so for an image-backed service), so no auto-deploy sentence (w4/554)",
  },
  "services.eventsRollbackConfirmBodyCommitNoAutoDeploy": {
    message:
      "서비스가 이 배포의 이미지({commit}에서 빌드됨)와 보존된 구성으로 실행됩니다. 저장된 설정은 변경되지 않으며 다음 표준 배포 때 적용됩니다. 보존된 구성이 없는 이전 배포는 이미지만 복원합니다.",
    description:
      "Rollback confirm dialog body naming the commit, when the service's auto-deploy is already off (w4/554)",
  },
  "services.eventsRollbackConfirmBodyStatic": {
    message:
      "이 배포를 기준으로 사이트가 다시 게시됩니다. 저장된 설정은 변경되지 않으며 자동 배포가 꺼집니다.",
    description:
      "Rollback confirm dialog body for a static site: it re-publishes the deploy's files, it has no image or instances (w4/m141)",
  },
  "services.eventsRollbackConfirmBodyStaticCommit": {
    message:
      "이 배포({commit})를 기준으로 사이트가 다시 게시됩니다. 저장된 설정은 변경되지 않으며 자동 배포가 꺼집니다.",
    description:
      "Static-site rollback confirm dialog body naming the commit being re-published (w4/m141)",
  },
  "services.eventsRollbackConfirmTagPull": {
    message:
      "{image}은(는) 태그입니다. 롤백은 이 태그가 지금 가리키는 이미지를 가져오며, 이 배포가 실행한 이미지와 다를 수 있습니다. 태그가 더 이상 없으면 롤백이 실패합니다.",
    description:
      "Rollback confirm sentence for a deploy whose image is a registry tag, not a digest: Render pulls the tag again (render.com/docs/rollbacks, w4/m432)",
  },
  "services.eventsRollbackAria": {
    message: "{target}(으)로 롤백",
    description:
      "Accessible name of a deploy row's Rollback control, naming the deploy it restores (w4/m141)",
  },
  "services.eventsCancelDeployAria": {
    message: "배포 {target} 취소",
    description:
      "Accessible name of a deploy row's Cancel control, naming the deploy (w4/m141)",
  },
  "services.eventsConfirmProceed": {
    message: "계속",
    description: "Confirm dialog proceed button",
  },
  "services.eventsConfirmCancel": {
    message: "돌아가기",
    description: "Confirm dialog cancel button",
  },
  "services.triggerDeploySuccess": {
    message: "배포를 시작했습니다.",
    description: "Toast after triggerDeploy succeeds",
  },
  "services.triggerDeployError": {
    message: "배포를 시작하지 못했습니다.",
    description: "Toast after triggerDeploy fails",
  },
  "services.cancelDeploySuccess": {
    message: "배포를 취소했습니다.",
    description: "Toast after cancelDeploy succeeds",
  },
  "services.cancelDeployError": {
    message: "배포를 취소하지 못했습니다.",
    description: "Toast after cancelDeploy fails",
  },
  "services.rollbackSuccess": {
    message: "롤백을 시작했습니다.",
    description: "Toast after rollbackService succeeds",
  },
  "services.rollbackError": {
    message: "롤백하지 못했습니다.",
    description: "Toast after rollbackService fails",
  },
  "services.eventsStatusLive": {
    message: "라이브",
    description: "Deploy status: live",
  },
  "services.eventsStatusInProgress": {
    message: "진행 중",
    description: "Deploy status: update_in_progress",
  },
  "services.eventsStatusFailed": {
    message: "실패",
    description: "Deploy status: update_failed",
  },
  "services.eventsStatusCanceled": {
    message: "취소됨",
    description: "Deploy status: canceled",
  },
  "services.eventsPreDeployRunning": {
    message: "사전 배포 명령 실행 중",
    description: "Deploy row: the pre-deploy step is in progress",
  },
  "services.eventsPreDeploySucceeded": {
    message: "사전 배포 명령 성공",
    description: "Deploy row: the pre-deploy step passed",
  },
  "services.eventsPreDeployCanceled": {
    message: "사전 배포 명령 취소됨",
    description:
      "Deploy row: the deploy ended (superseded or canceled) while its pre-deploy step was still running",
  },
  "services.eventsPreDeployFailed": {
    message: "사전 배포 명령 실패",
    description:
      "Deploy row: the pre-deploy step failed (distinct from a health-check failure)",
  },
  "services.eventsRolledBackFrom": {
    message: "{target}에서 롤백됨",
    description: "Deploy row: provenance note when trigger=rollback",
  },
  "services.capLimitTitle": {
    message: "서비스 한도에 도달했습니다",
    description:
      "Alert title when the workspace's service creation cap is hit (w7/m9)",
  },
  "services.capLimitUpgrade": {
    message: "요금제 업그레이드",
    description: "Upgrade CTA button inside the cap-limit Alert (w7/m9)",
  },
  "services.networkingTitle": {
    message: "네트워킹",
    description: "Settings Networking card title (w7/m32)",
  },
  "services.networkingDescription": {
    message: "인바운드 HTTP 트래픽을 다음 소스 CIDR로 제한합니다.",
    description: "Settings Networking card description (w7/m32)",
  },
  "services.networkingScalePlanOnly": {
    message:
      "Web Service와 Static Site의 인바운드 IP 규칙은 Scale 요금제 이상에서 사용할 수 있습니다. 이미 설정된 규칙은 계속 적용되며 모두 허용 규칙으로 바꿀 수 있습니다.",
    description:
      "Hint replacing the editable IP rules below the Scale plan (w8/m137)",
  },
  "services.networkingClearAll": {
    message: "모든 트래픽 허용",
    description:
      "Button replacing every inbound IP rule with the allow-all rule below the Scale plan (w8/m137, w8/m157)",
  },
  "services.networkingHint": {
    message:
      "CIDR 블록(예: 203.0.113.0/24)을 입력하고 추가를 누르세요. 규칙이 없으면 모든 인바운드 트래픽이 차단되며, 규칙 0.0.0.0/0은 모든 소스 IP를 허용합니다.",
    description: "Hint below the CIDR list in the Networking card (w7/m32)",
  },
  "services.networkingOpen": {
    message: "모든 소스 IP에 열려 있음",
    description:
      "Shown below the Scale plan when the service has only the allow-all rule (Render default, w7/m32, w8/m157)",
  },
  "services.networkingBlocked": {
    message:
      "규칙 없음: 이 서비스로 들어오는 모든 인바운드 트래픽이 차단됩니다. 모든 소스 IP를 허용하려면 0.0.0.0/0을 추가하세요.",
    description:
      "Warning shown when a service's inbound IP rule list is empty, saved or about to be saved (w8/m157)",
  },
  "services.networkingAdd": {
    message: "허용 목록 항목 추가",
    description: "Button to add a CIDR to the draft list (w7/m32)",
  },
  "services.networkingEntryDescription": {
    message: "설명(선택 사항)",
    description: "Placeholder for a service allowlist entry description",
  },
  "services.networkingCIDR": {
    message: "새 CIDR 블록",
    description: "Accessible label for the add-row CIDR input",
  },
  "services.networkingCIDRRule": {
    message: "규칙 {number}의 CIDR 블록",
    description: "Accessible label for an existing allowlist CIDR row",
  },
  "services.networkingDescriptionLabel": {
    message: "새 규칙 설명",
    description: "Accessible label for the add-row description input",
  },
  "services.networkingDescriptionRule": {
    message: "규칙 {number}의 설명",
    description: "Accessible label for an existing allowlist description row",
  },
  "services.networkingInvalid": {
    message: "올바른 IPv4 또는 IPv6 CIDR 블록을 입력하세요.",
    description: "Validation error for an invalid service allowlist CIDR",
  },
  "services.networkingSave": {
    message: "허용 목록 저장",
    description: "Button to persist the CIDR list (w7/m32)",
  },
  "services.networkingRemove": {
    message: "{cidr} 제거",
    description:
      "Accessible label on the trash icon next to a CIDR tag (w7/m32)",
  },
  "services.networkingMoveUp": {
    message: "{cidr} 위로 이동",
    description: "Accessible label to move an allowlist entry earlier",
  },
  "services.networkingMoveDown": {
    message: "{cidr} 아래로 이동",
    description: "Accessible label to move an allowlist entry later",
  },
  "services.networkingSaved": {
    message: "IP 허용 목록을 업데이트했습니다",
    description: "Toast on successful setServiceIpAllowList mutation (w7/m32)",
  },
  "services.networkingError": {
    message: "IP 허용 목록을 업데이트하지 못했습니다. 다시 시도하세요.",
    description: "Toast on failed setServiceIpAllowList mutation (w7/m32)",
  },
  "services.outboundIpsTitle": {
    message: "아웃바운드 IP",
    description: "Settings card title for service egress IPs (w8/010)",
  },
  "services.outboundIpsDescription": {
    message:
      "이 서비스의 공유 송신 주소입니다. 테넌트 풀 노드의 현재 공개 IP이며, 오토스케일러가 노드를 추가하거나 제거하면 바뀔 수 있습니다.",
    description:
      "Outbound IPs card helper — truthful shared-pool semantics (w8/010)",
  },
  "services.outboundIpsEmpty": {
    message:
      "아직 보고된 아웃바운드 IP가 없습니다(노드 ExternalIP가 없는 로컬 클러스터에서 흔히 나타납니다).",
    description: "Honest empty state when outboundIps.ips is empty (w8/010)",
  },
  "services.outboundIpsCopy": {
    message: "{ip} 복사",
    description:
      "Accessible label on the copy button for one outbound IP (w8/010)",
  },
  "services.outboundIpsCopied": {
    message: "복사됨",
    description: "Toast after copying an outbound IP (w8/010)",
  },
  "services.outboundIpsCopyError": {
    message: "복사할 수 없습니다",
    description: "Toast when clipboard write fails for an outbound IP (w8/010)",
  },
  "services.environmentPageDescription": {
    message:
      "이 서비스가 실행 중에 읽는 모든 값입니다. 값은 직접 표시하기 전까지 가려집니다.",
    description: "Service Environment page introduction",
  },
  "services.environmentEdit": {
    message: "편집",
    description: "Enter the coherent service environment draft",
  },
  "services.environmentMaskedValue": {
    message: "가려진 시크릿 값",
    description: "Accessible label for an unrevealed secret",
  },
  "services.environmentUnchangedMasked": {
    message: "변경 없음(가려짐)",
    description: "Placeholder for an opaque unchanged draft value",
  },
  "services.environmentDuplicateKey": {
    message: "각 변수 키는 고유해야 합니다.",
    description: "Draft validation for a duplicate environment key",
  },
  "services.environmentValueRequired": {
    message: "이 새 변수의 값을 입력하세요.",
    description: "Draft validation for a new variable without a value",
  },
  "services.environmentValueNul": {
    message:
      "값에는 NUL 바이트를 포함할 수 없습니다. 프로세스 환경은 NUL 바이트를 담을 수 없습니다. 바이너리 내용에는 시크릿 파일을 사용하세요.",
    description:
      "Draft validation under an environment variable value with a NUL byte (w21/047)",
  },
  "services.environmentValueTooLarge": {
    message:
      "KEY=value 항목은 최대 131,071바이트까지 가능합니다. 그보다 크면 프로세스를 시작할 수 없습니다. 큰 페이로드에는 시크릿 파일을 사용하세요.",
    description:
      "Draft validation under an environment variable whose KEY=value entry is over the process limit; the server counts UTF-8 bytes, so a Chinese character is 3 (w21/047)",
  },
  "services.environmentPortInvalid": {
    message:
      "PORT는 1에서 65535 사이의 정수여야 하며, 플랫폼이 예약한 18012, 18013, 19099는 사용할 수 없습니다.",
    description:
      "Draft validation under a PORT environment variable the operator would ignore (w21/047)",
  },
  "services.environmentLimit": {
    message:
      "환경 변수는 서비스 또는 그룹당 최대 500개, 총 512 KiB로 제한됩니다.",
    description:
      "Draft validation when the environment variables would exceed bex-api's per-service quota",
  },
  "services.environmentStagedDelete": {
    message: "제거 예정",
    description: "Badge on a staged environment deletion",
  },
  "services.environmentUndo": {
    message: "실행 취소",
    description: "Undo a staged environment deletion",
  },
  "services.environmentUnsavedTitle": {
    message: "저장되지 않은 변경 사항",
    description: "Combined draft save bar heading",
  },
  "services.environmentRestoredTitle": {
    message: "저장되지 않은 변경 사항을 복원했습니다",
    description:
      "Heading of the banner shown when an unsaved env draft is recovered in this tab, after a reload or a session-expiry sign-in (w3/m80)",
  },
  "services.environmentRestoredBody": {
    message:
      "저장하지 않은 환경 편집 내용을 복원했습니다. 내용을 검토한 뒤 저장하거나 취소하세요.",
    description:
      "Body of the restored-draft banner. Names no cause: an ordinary reload restores the draft as a sign-in does (w4/337)",
  },
  "services.environmentRestoredDiscard": {
    message: "복원된 변경 사항 취소",
    description: "Button that throws away a recovered env draft",
  },
  "services.environmentUnsavedVariables_other": {
    message: "변수 {count}개 변경됨",
    description:
      "Draft variable-operation count, composed with the file count at the call site (plural)",
  },
  "services.environmentUnsavedFiles_other": {
    message: "파일 {count}개 변경됨",
    description:
      "Draft file-operation count, composed with the variable count at the call site (plural)",
  },
  "services.environmentSaveOptions": {
    message: "환경 저장 옵션",
    description: "Accessible label for the split save menu",
  },
  "services.environmentSaveOnly": {
    message: "저장만 하기",
    description: "Persist and project without rolling the service",
  },
  "services.environmentSaveDeploy": {
    message: "저장 후 배포",
    description: "Persist and roll the current image once",
  },
  "services.environmentSaveRebuild": {
    message: "저장, 다시 빌드 후 배포",
    description: "Persist then start one source build and deploy",
  },
  "services.environmentSaveOnlySuccess": {
    message:
      "배포 없이 환경을 저장했습니다. 대기 중인 변경 사항을 적용하려면 표준 배포를 사용하세요.",
    description:
      "Save-only success feedback while the operator checks saved/runtime differences; does not assert that a no-op or revert has pending changes",
  },
  "services.environmentSaveDeploySuccess": {
    message: "환경을 저장하고 배포를 시작했습니다",
    description: "Toast after save-and-deploy succeeds",
  },
  "services.environmentSavedRefreshFailed": {
    message:
      "환경을 저장했지만 이 페이지를 새로 고치지 못했습니다. 페이지를 다시 불러와 최신 값과 배포 상태를 확인하세요.",
    description:
      "A committed environment patch must not be retried because a follow-up read failed",
  },
  "services.environmentSaveError": {
    message: "환경을 저장하지 못했습니다. 초안은 그대로 남아 있습니다.",
    description: "Batch environment save failure",
  },
  "services.environmentDraftStale": {
    message:
      "초안을 작성한 이후 이 환경 그룹이 변경되었을 수 있어 초안을 저장하지 않았습니다. 필요한 내용을 복사하고 초안을 취소한 뒤, 다시 편집하여 최신 버전에서 시작하세요.",
    description:
      "Refusal when a group draft's base revision is no longer current (ENV_GROUP_REVISION_CONFLICT) or predates revision tracking (w4/m161)",
  },
  "services.environmentSavedDeployFailedTitle": {
    message: "구성을 저장했지만 롤아웃이 완료되지 않았습니다",
    description: "Partial rebuild failure heading",
  },
  "services.environmentSavedDeployFailedBody": {
    message:
      "환경 변경 사항은 저장되었습니다. 준비가 되면 완료되지 않은 롤아웃만 다시 시도하세요.",
    description: "Partial rebuild failure recovery explanation",
  },
  "services.environmentRetryDeploy": {
    message: "롤아웃 다시 시도",
    description: "Retry only the failed deploy phase",
  },
  "services.environmentDiscardTitle": {
    message: "환경 변경 사항을 취소할까요?",
    description: "Dirty navigation guard title",
  },
  "services.environmentDiscardBody": {
    message: "이 변경 사항은 저장되지 않았으며 페이지를 떠나면 사라집니다.",
    description: "Dirty navigation guard explanation",
  },
  "services.environmentKeepEditing": {
    message: "계속 편집",
    description: "Cancel dirty navigation",
  },
  "services.environmentDiscard": {
    message: "취소하고 나가기",
    description: "Confirm dirty navigation",
  },
  "services.envReveal": {
    message: "표시",
    description: "Reveal one masked environment value",
  },
  "services.envRevealOne": {
    message: "{name} 표시",
    description:
      'aria-label of the per-row Reveal button for one env var or secret file; starts with the visible text "Reveal" (w4/499)',
  },
  "services.envHideOne": {
    message: "{name} 숨기기",
    description:
      'aria-label of the per-row Hide button for one env var or secret file; starts with the visible text "Hide" (w4/499)',
  },
  "services.envHide": {
    message: "숨기기",
    description: "Mask one revealed environment value",
  },
  "services.envCopy": {
    message: "환경 변수 복사",
    description: "Copy the complete dotenv export",
  },
  "services.envDownload": {
    message: ".env 다운로드",
    description: "Download the complete dotenv export",
  },
  "services.envCopySuccess": {
    message: "환경을 복사했습니다",
    description: "Toast after the complete dotenv export is copied",
  },
  "services.envCopyOne": {
    message: "{name} 복사",
    description:
      "aria-label of the per-row copy button for one env var or secret file — distinct from the bulk envCopy dotenv export",
  },
  "services.envCopyOneSuccess": {
    message: "{name} 복사됨",
    description:
      "Toast after one env var value or secret file's contents is copied — distinct from the bulk envCopySuccess",
  },
  "services.envAddVariable": {
    message: "변수 추가",
    description: "Add one blank draft variable",
  },
  "services.envAddGenerated": {
    message: "생성된 시크릿",
    description: "Add one previewable generated secret",
  },
  "services.envImport": {
    message: ".env에서 가져오기",
    description: "Open the dotenv import dialog",
  },
  "services.envImportTitle": {
    message: "환경 변수 가져오기",
    description: "Dotenv import dialog title",
  },
  "services.envImportDescription": {
    message:
      "dotenv 할당문을 붙여넣거나 텍스트 파일을 선택하세요. 가져온 값은 저장하기 전까지 이 초안에만 유지됩니다.",
    description: "Dotenv import dialog explanation",
  },
  "services.envImportTextLabel": {
    message: "dotenv 내용",
    description: "Accessible label for the dotenv import textarea",
  },
  "services.envImportPlaceholder": {
    message: "API_URL=https://example.com\nSECRET=replace-me",
    description: "Dotenv import textarea placeholder",
  },
  "services.envImportChooseFile": {
    message: "파일 선택",
    description: "Dotenv file picker button",
  },
  "services.envImportAdd": {
    message: "변수 추가",
    description: "Stage parsed dotenv variables",
  },
  "services.envImportLineError": {
    message: "{line}번째 줄이 KEY=value 형식의 할당문이 아닙니다.",
    description: "Line-numbered dotenv parse error",
  },
  "services.envImportKeyError": {
    message:
      "{line}번째 줄: 이름은 영문자, 숫자, 점, 대시, 밑줄로 최대 253자까지 사용할 수 있습니다.",
    description:
      "Dotenv import error: the variable name on a line is invalid (w4/159)",
  },
  "services.envImportQuoteError": {
    message: "{line}번째 줄에서 연 따옴표가 닫히지 않았습니다.",
    description: "Dotenv import error: a quoted value never closes (w4/159)",
  },
  "services.envImportTrailingError": {
    message: "{line}번째 줄의 닫는 따옴표 뒤에 텍스트가 있습니다.",
    description:
      "Dotenv import error: text after a quoted value's closing quote (w4/159)",
  },
  "services.envImportReading": {
    message: "파일을 읽는 중…",
    description:
      "Status while a chosen dotenv file is being read; Add waits for it",
  },
  "services.envImportFileError": {
    message: "1 MiB 이하의 읽을 수 있는 텍스트 파일을 선택하세요.",
    description: "Dotenv file read or size error",
  },
  "services.secretFileUpload": {
    message: "파일 업로드",
    description: "Stage multiple local text files",
  },
  "services.secretFileSkippedTitle_other": {
    message: "파일 {count}개를 건너뛰었습니다.",
    description: "Secret-file upload: heading over the skipped files (w4/628)",
  },
  "services.secretFileSkippedName": {
    message:
      "이름에는 영문자, 숫자, '.', '-', '_'만 사용할 수 있습니다(최대 253자).",
    description: "Secret-file upload: why a file was skipped — its name",
  },
  "services.secretFileSkippedSize": {
    message: "1 MiB보다 큽니다.",
    description: "Secret-file upload: why a file was skipped — its size",
  },
  "services.secretFileSkippedEncoding": {
    message:
      "UTF-8 텍스트가 아닙니다. 바이너리 파일(keystore, .p12, DER 인증서)은 base64로 인코딩하여 업로드하고 시작할 때 디코딩하세요. 예: base64 -d /etc/secrets/keystore.b64 > /tmp/keystore.p12.",
    description: "Secret-file upload: why a file was skipped — not UTF-8 text",
  },
  "services.secretFileSkippedDuplicate": {
    message: "같은 이름의 파일이 이미 목록에 있습니다.",
    description: "Secret-file upload: why a file was skipped — duplicate name",
  },
  "services.secretFileDuplicateName": {
    message: "각 시크릿 파일 이름은 고유해야 합니다.",
    description: "Draft validation for a duplicate file name",
  },
  "services.secretFileContentRequired": {
    message: "이 새 파일의 내용을 추가하세요.",
    description: "Draft validation for missing new-file content",
  },
  "services.secretFileLimit": {
    message:
      "시크릿 파일은 서비스 또는 그룹당 500개까지, 전체 크기는 1 MiB까지 가능합니다.",
    description:
      "Draft validation when the secret files would exceed bex-api's per-service quota: 500 files, 1 MiB of contents (names not counted), Render's 1 MB (w4/m363)",
  },
  "services.secretFileViewContent": {
    message: "내용 보기",
    description: "Reveal or inspect secret-file contents in a dialog",
  },
  "services.secretFileViewContentOne": {
    message: "{name} 내용 보기",
    description:
      "Accessible name for View contents once the file has a name (m101/t013)",
  },
  "services.secretFileEditContent": {
    message: "내용 편집",
    description: "Edit staged secret-file contents in a dialog",
  },
  "services.secretFileEditContentOne": {
    message: "{name} 내용 편집",
    description:
      "Accessible name for Edit contents once the file has a name (m101/t013)",
  },
  "services.secretFileContentDialogTitle": {
    message: "{name} 내용",
    description: "Secret-file content dialog title",
  },
  "services.secretFileContentDialogDescription": {
    message:
      "이 파일은 다음 배포 이후 /etc/secrets에 읽기 전용으로 마운트됩니다.",
    description: "Secret-file content dialog explanation",
  },
  "services.secretFileContentDialogDescriptionDocker": {
    message:
      "이 파일은 다음 배포 이후 /etc/secrets에 읽기 전용으로 마운트됩니다. Docker 빌드에서도 빌드 컨텍스트의 루트에서 이 파일을 찾을 수 있으므로, 이미지에 포함되지 않도록 .dockerignore에 나열하세요.",
    description:
      "Secret-file content dialog explanation for a Docker-runtime service: the build context also receives the file, as on Render (w8/253)",
  },
  "services.secretFileLoadingContent": {
    message: "내용을 불러오는 중…",
    description: "Secret-file content reveal progress",
  },
  "services.secretFileContentDone": {
    message: "완료",
    description: "Close and stage secret-file content edits",
  },
  "services.secretFileUntitled": {
    message: "이름 없는 파일",
    description: "Fallback title for a new unnamed secret file",
  },
  "services.secretFileDelete": {
    message: "시크릿 파일 삭제",
    description: "Accessible label for a staged secret-file deletion",
  },
  "services.secretFileDeleteOne": {
    message: "시크릿 파일 {name} 삭제",
    description:
      "Accessible name for Delete secret file once the file has a name (m101/t013)",
  },
  "services.envGroupsLinkedTitle": {
    message: "연결된 환경 그룹",
    description: "Service-side environment group card title",
  },
  "services.envGroupsLinkedCount": {
    message: "연결됨({count})",
    description: "Count of groups linked to this service",
  },
  "services.envGroupsAvailableCount": {
    message: "연결 가능({count})",
    description: "Count of groups available to link",
  },
  "services.envGroupsNoneLinked": {
    message: "이 서비스에 연결된 환경 그룹이 없습니다.",
    description: "Service group empty linked state",
  },
  "services.envGroupsAvailableTitle": {
    message: "연결 가능",
    description:
      "Available-groups heading while the service's environment is still resolving (no count yet)",
  },
  "services.envGroupsScopeLoading": {
    message: "이 서비스의 환경을 공유하는 그룹을 확인하는 중…",
    description:
      "Available groups wait for the service's environment scope (w4/217)",
  },
  "services.envGroupsOrderReading": {
    message: "어느 연결 그룹이 우선하는지 확인하는 중…",
    description:
      "Link order is being re-read after a link change; no winner is claimed until it lands (w4/322)",
  },
  "services.envGroupsOrderFailed": {
    message:
      "연결했지만 어느 그룹이 우선하는지 확인하지 못했습니다. 순서를 다시 불러와 확인하세요.",
    description:
      "The post-write Server read failed; the write stands, precedence stays unclaimed until a retry succeeds (w4/322)",
  },
  "services.envGroupsScopeError": {
    message: "이 서비스에 연결할 수 있는 그룹을 확인하지 못했습니다.",
    description:
      "Scope index failed; nothing is offered until a retry succeeds (w4/217)",
  },
  "services.envGroupsScopeEnvironmentMissing": {
    message:
      "이 서비스의 환경이 불러온 환경 목록에 없어 연결할 그룹을 표시하지 않습니다. 다시 시도하여 새로 고치세요.",
    description:
      "The service reports an Environment the authorized scope index lacks even after a refresh; never falls back to Workspace (w4/220)",
  },
  "services.envGroupsNoneInScope": {
    message:
      "이 서비스의 환경을 공유하는 연결되지 않은 그룹이 없습니다. 여기에서 그룹을 만들거나 기존 그룹을 이 환경으로 옮기세요.",
    description:
      "Unlinked groups exist, but none in this service's environment scope (w4/217)",
  },
  "services.envGroupsNoneAvailable": {
    message: "워크스페이스의 모든 환경 그룹이 이미 연결되어 있습니다.",
    description: "Service group empty available state",
  },
  "services.envGroupsNoneAvailableCreate": {
    message:
      "워크스페이스에 환경 그룹이 아직 없습니다. 변수, 파일, 그리고 이 서비스가 미리 선택된 상태로 그룹을 만드세요.",
    description: "Service group create-first empty state",
  },
  "services.cronEveryMinute": {
    message: "매분",
    description: "Cron schedule preview phrase (w4/m188): fires every minute",
  },
  "services.cronEveryNMinutes": {
    message: "{n}분마다",
    description:
      "Cron schedule preview phrase (w4/m188): uniform minute step; n is the step",
  },
  "services.cronEveryHour": {
    message: "매시간",
    description:
      "Cron schedule preview phrase (w4/m188): fires at minute 0 of every hour",
  },
  "services.cronEveryNHours": {
    message: "{n}시간마다",
    description:
      "Cron schedule preview phrase (w4/m188): uniform hour step; n is the step",
  },
  "services.cronHourlyAtMinute": {
    message: "매시간 {minute}분",
    description:
      "Cron schedule preview phrase (w4/m188): fires once an hour at the given minute",
  },
  "services.cronDailyAt": {
    message: "매일 {time}",
    description: "Cron schedule preview phrase (w4/m188): daily at HH:MM (UTC)",
  },
  "services.cronWeeklyAt": {
    message: "매주 {day} {time}",
    description:
      "Cron schedule preview phrase (w4/m188): weekly; day is a services.cronDay* weekday name, time is HH:MM (UTC)",
  },
  "services.cronMonthlyAt": {
    message: "매월 {day}일 {time}",
    description:
      "Cron schedule preview phrase (w4/m188): monthly on a day of the month at HH:MM (UTC)",
  },
  "services.cronRunsEveryMinute": {
    message: "매분",
    description:
      'Cron job header, after "Runs <command>": the schedule every minute (Render\'s cronstrue phrase, w3/m555)',
  },
  "services.cronRunsEveryNMinutes": {
    message: "{n}분마다",
    description:
      'Cron job header, after "Runs <command>": every {n} minutes (Render\'s cronstrue phrase, w3/m555)',
  },
  "services.cronRunsEveryHour": {
    message: "매시간",
    description:
      'Cron job header, after "Runs <command>": every hour on the hour (Render\'s cronstrue phrase, w3/m555)',
  },
  "services.cronRunsHourlyAtMinute": {
    message: "매시 {minute}분",
    description:
      'Cron job header, after "Runs <command>": hourly at minute {minute} (Render\'s cronstrue phrase, w3/m555)',
  },
  "services.cronRunsEveryNHours": {
    message: "{n}시간마다 {minute}분",
    description:
      'Cron job header, after "Runs <command>": at minute {minute}, every {n} hours (Render\'s cronstrue phrase, w3/m555)',
  },
  "services.cronRunsDailyAt": {
    message: "매일 UTC {time}",
    description:
      'Cron job header, after "Runs <command>": daily at {time} ("09:00 AM" in English, the 24-hour clock elsewhere), UTC (Render\'s cronstrue phrase, w3/m555)',
  },
  "services.cronRunsWeeklyAt": {
    message: "매주 {day} UTC {time}",
    description:
      'Cron job header, after "Runs <command>": weekly on {day} (a services.cronDay* name) at {time}, UTC (Render\'s cronstrue phrase, w3/m555)',
  },
  "services.cronRunsMonthlyAt": {
    message: "매월 {day}일 UTC {time}",
    description:
      'Cron job header, after "Runs <command>": monthly on day {day} at {time}, UTC (Render\'s cronstrue phrase, w3/m555)',
  },
  "services.cronDaySunday": {
    message: "일요일",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDayMonday": {
    message: "월요일",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDayTuesday": {
    message: "화요일",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDayWednesday": {
    message: "수요일",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDayThursday": {
    message: "목요일",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDayFriday": {
    message: "금요일",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.cronDaySaturday": {
    message: "토요일",
    description:
      "Cron schedule preview phrase (w4/m188): weekday name inserted into services.cronWeeklyAt",
  },
  "services.environmentEditingTitle": {
    message: "환경 편집 중",
    description:
      "Edit bar title while the environment draft is open and holds no change yet (the bar says Unsaved changes once it does)",
  },
  "services.environmentNoChanges": {
    message: "아직 변경 사항이 없습니다.",
    description:
      "Edit bar line while the open environment draft holds no change: what to do next",
  },
  "services.envAddOptions": {
    message: "변수를 추가하는 다른 방법",
    description:
      "Accessible name of the chevron beside Add variable that opens the other ways to add (generated secret, .env import)",
  },
  "services.secretFilesReloadWithVariables": {
    message: "위에서 다시 시도하면 변수와 함께 다시 불러옵니다.",
    description:
      "Secret Files card line when both lists failed to load: the variables card's Try again reloads the files too, so this card has no button of its own",
  },
  "services.createSubmitWeb": {
    message: "Web Service 배포",
    description:
      "Create-wizard submit button for a web service (Render's 'Deploy web service')",
  },
  "services.createSubmitPrivate": {
    message: "Private Service 배포",
    description: "Create-wizard submit button for a private service",
  },
  "services.createSubmitWorker": {
    message: "Background Worker 배포",
    description: "Create-wizard submit button for a background worker",
  },
  "services.createSubmitCron": {
    message: "Cron Job 배포",
    description:
      "Create-wizard submit button for a cron job (Render's 'Deploy cron job')",
  },
  "services.createSubmitStatic": {
    message: "Static Site 배포",
    description: "Create-wizard submit button for a static site",
  },
  "services.createSourceEdit": {
    message: "편집",
    description:
      "Button on the chosen repository's summary row that reopens the source picker",
  },
  "services.createSourceEditLabel": {
    message: "소스 변경",
    description:
      "Accessible name of the Edit button on the chosen repository's summary row ({repo} = owner/name)",
  },
  "services.createBlockedSource": {
    message: "계속하려면 소스를 선택하세요.",
    description:
      "Create-wizard action bar: why Deploy is held while no source is chosen",
  },
  "services.createFieldNameDescription": {
    message: "이 서비스의 고유한 이름입니다.",
    description: "Create-wizard Name row description",
  },
  "services.createFieldRuntimeDescription": {
    message: "이 서비스의 런타임 환경을 선택하세요.",
    description: "Create-wizard Language/runtime row description",
  },
  "services.createFieldBranchDescription": {
    message: "빌드하고 배포할 Git 브랜치입니다.",
    description: "Create-wizard Branch row description",
  },
  "services.createFieldBuildCommandDescription": {
    message: "배포할 때마다 먼저 이 명령으로 앱을 빌드합니다.",
    description:
      "Create-wizard Build Command row description (native runtimes)",
  },
  "services.createFieldStartCommandDescription": {
    message: "배포할 때마다 이 명령으로 앱을 시작합니다.",
    description: "Create-wizard Start Command row description",
  },
  "services.createFieldDockerCommandDescription": {
    message:
      "선택 사항: 이미지의 CMD와 ENTRYPOINT를 다른 명령으로 덮어써 서비스를 시작합니다.",
    description: "Create-wizard Docker Command row description (Advanced)",
  },
  "services.createFieldPlanDescription": {
    message: "서비스가 실행되는 인스턴스입니다. 언제든지 변경할 수 있습니다.",
    description: "Create-wizard compute plan row description",
  },
  "services.createEnvVarsDescription": {
    message:
      "환경별 설정과 시크릿(API 키 등)을 지정하고 코드에서 그 값을 읽으세요.",
    description: "Create-wizard Environment Variables card description",
  },
  "services.createBlockedPublishPath": {
    message: "계속하려면 게시 디렉터리를 입력하세요.",
    description:
      "Create-wizard action bar: Deploy is held until a static site has a publish directory",
  },
  "services.createBlockedSchedule": {
    message: "계속하려면 올바른 일정을 입력하세요.",
    description:
      "Create-wizard action bar: Deploy is held until a cron job has a valid schedule",
  },
  "services.createBlockedCommands": {
    message: "계속하려면 빌드 명령과 시작 명령을 입력하세요.",
    description:
      "Create-wizard action bar: Deploy is held until the native runtime's build and start commands are filled",
  },
  "services.createBlockedPort": {
    message: "계속하려면 올바른 포트를 입력하세요.",
    description:
      "Create-wizard action bar: Deploy is held while the port is out of range",
  },
};

export default koServices;
