import type { TranslationEntry } from "@/i18n";

const ko: Record<string, TranslationEntry> = {
  "sandboxGroupPages.navSandboxes": {
    message: "샌드박스",
    description: "Sidebar entry and heading: a sandbox group's sandboxes",
  },
  "sandboxGroupPages.navSnapshots": {
    message: "스냅샷",
    description: "Sidebar entry and heading: a sandbox group's snapshots",
  },
  "sandboxGroupPages.navImages": {
    message: "사용자 지정 이미지",
    description:
      "Sidebar entry and heading: the images imported into a sandbox group",
  },
  "sandboxGroupPages.navSettings": {
    message: "설정",
    description: "Sidebar entry and heading: a sandbox group's settings",
  },
  "sandboxGroupPages.statusCreating": {
    message: "생성 중",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusRunning": {
    message: "실행 중",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusResuming": {
    message: "재개 중",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusSuspending": {
    message: "일시 중지 중",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusSuspended": {
    message: "일시 중지됨",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusErrored": {
    message: "오류",
    description: "Sandbox status",
  },
  "sandboxGroupPages.statusTerminated": {
    message: "종료됨",
    description: "Sandbox status",
  },
  "sandboxGroupPages.rangeLabel": {
    message: "시간 범위",
    description: "Accessible name of the time range picker",
  },
  "sandboxGroupPages.rangeHour": {
    message: "최근 1시간",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeDay": {
    message: "최근 24시간",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeWeek": {
    message: "최근 7일",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeMonth": {
    message: "최근 30일",
    description: "Time range option",
  },
  "sandboxGroupPages.rangeAll": {
    message: "전체 기간",
    description: "Time range option",
  },
  "sandboxGroupPages.searchSandboxes": {
    message: "이름, 샌드박스 ID 또는 상태로 검색",
    description: "Placeholder of the Sandboxes search",
  },
  "sandboxGroupPages.searchSnapshots": {
    message: "스냅샷 검색",
    description: "Placeholder of the Snapshots search",
  },
  "sandboxGroupPages.searchImages": {
    message: "이미지 검색",
    description: "Placeholder of the Custom images search",
  },
  "sandboxGroupPages.pauseLive": {
    message: "실시간 업데이트 일시 중지",
    description: "Button that stops the list refreshing",
  },
  "sandboxGroupPages.resumeLive": {
    message: "실시간 업데이트 재개",
    description: "Button that restarts the list refreshing",
  },
  "sandboxGroupPages.showHideColumns": {
    message: "열 표시 또는 숨기기",
    description: "Accessible name of the columns menu",
  },
  "sandboxGroupPages.summaryTotal": {
    message: "합계",
    description: "Summary: runs that ended in the time range",
  },
  "sandboxGroupPages.summaryErrored": {
    message: "오류",
    description: "Summary: share of runs that ended in an error",
  },
  "sandboxGroupPages.summaryErroredTip": {
    message: "시간 범위 내에서 오류로 끝난 샌드박스의 비율입니다.",
    description: "Tooltip of the Errored summary",
  },
  "sandboxGroupPages.summaryRunning": {
    message: "실행 중",
    description: "Summary: sandboxes active now",
  },
  "sandboxGroupPages.summaryRunningTip": {
    message:
      "시간 범위와 관계없이 지금 생성, 실행, 재개 또는 일시 중지 중인 샌드박스입니다. 이 샌드박스는 동시 실행 한도에 포함됩니다.",
    description: "Tooltip of the Running summary",
  },
  "sandboxGroupPages.loadRunsError": {
    message: "샌드박스 실행 기록을 불러오지 못했습니다",
    description: "The run chart failed to load",
  },
  "sandboxGroupPages.runsChartLabel": {
    message: "종료된 샌드박스 실행: 성공 {succeeded}건, 오류 {errored}건",
    description: "Accessible name of the run chart",
  },
  "sandboxGroupPages.runsSucceeded": {
    message: "성공",
    description: "Chart legend: runs that ended without an error",
  },
  "sandboxGroupPages.runsErrored": {
    message: "오류",
    description: "Chart legend: runs that ended in an error",
  },
  "sandboxGroupPages.selectedCount_other": {
    message: "샌드박스 {count}개 선택됨",
    description: "How many rows are selected",
  },
  "sandboxGroupPages.terminateSelected": {
    message: "종료",
    description: "Button: terminate the selected sandboxes",
  },
  "sandboxGroupPages.terminateTitle_other": {
    message: "샌드박스 {count}개를 종료하시겠습니까?",
    description: "Terminate confirmation title",
  },
  "sandboxGroupPages.terminateDescription_other": {
    message:
      "샌드박스 {count}개가 중지되며, 스냅샷에 저장되지 않은 내용은 모두 사라집니다.",
    description: "Terminate confirmation body",
  },
  "sandboxGroupPages.terminateConfirm": {
    message: "종료",
    description: "Terminate confirmation button",
  },
  "sandboxGroupPages.terminateFailed_other": {
    message: "샌드박스 {count}개를 종료하지 못했습니다. 다시 시도해 주세요.",
    description: "Toast: terminates failed",
  },
  "sandboxGroupPages.terminateStarted_other": {
    message: "샌드박스 {count}개를 종료하는 중",
    description: "Toast: terminates were accepted",
  },
  "sandboxGroupPages.terminateSandbox": {
    message: "샌드박스 종료",
    description: "Drawer menu item",
  },
  "sandboxGroupPages.sandboxActions": {
    message: "샌드박스 작업",
    description: "Accessible name of the drawer's menu",
  },
  "sandboxGroupPages.loadSandboxError": {
    message: "샌드박스를 불러올 수 없습니다",
    description: "A sandbox's drawer failed to load",
  },
  "sandboxGroupPages.loadSandboxesError": {
    message: "샌드박스를 불러오지 못했습니다",
    description: "The list failed to load",
  },
  "sandboxGroupPages.noMatchingSandboxes": {
    message: "일치하는 샌드박스가 없습니다",
    description: "Empty state with filters",
  },
  "sandboxGroupPages.noMatchingSandboxesHelp": {
    message: "다른 검색어를 사용하거나 시간 범위를 넓혀 보세요.",
    description: "Empty state with filters",
  },
  "sandboxGroupPages.clearFilters": {
    message: "필터 지우기",
    description: "Button: reset the search and time range",
  },
  "sandboxGroupPages.firstSandboxTitle": {
    message: "첫 샌드박스 만들기",
    description: "Empty state title of the Sandboxes page",
  },
  "sandboxGroupPages.firstSandboxDescription": {
    message:
      "CLI로 샌드박스를 실행하거나, TypeScript 및 Python SDK로 샌드박스를 프로그래밍 방식으로 만들도록 코드를 설정하세요.",
    description: "Empty state body of the Sandboxes page",
  },
  "sandboxGroupPages.starterCli": {
    message: "CLI",
    description: "Starter tab: the command line",
  },
  "sandboxGroupPages.starterSdk": {
    message: "SDK",
    description: "Starter tab: the SDKs",
  },
  "sandboxGroupPages.starterInstallCli": {
    message: "최신 CLI 설치 또는 업데이트",
    description: "Starter step",
  },
  "sandboxGroupPages.starterHomebrew": {
    message: "Homebrew",
    description: "Install method",
  },
  "sandboxGroupPages.starterDirect": {
    message: "직접 다운로드",
    description: "Install method",
  },
  "sandboxGroupPages.starterLogin": {
    message: "로그인하고 이 워크스페이스 사용",
    description: "Starter step",
  },
  "sandboxGroupPages.starterCreateSandbox": {
    message: "기본값으로 샌드박스 만들기",
    description: "Starter step",
  },
  "sandboxGroupPages.starterApiKey": {
    message: "API 키 설정",
    description: "Starter step",
  },
  "sandboxGroupPages.starterApiKeyHelp": {
    message:
      "코드를 실행할 터미널에서 이 명령을 실행하세요. SDK는 RENDER_API_KEY 환경 변수에서 키를 읽습니다. 자리 표시자를 bex API 키로 바꾸세요.",
    description: "Starter step help",
  },
  "sandboxGroupPages.starterInstallSdk": {
    message: "프로젝트 설정 및 SDK 설치",
    description: "Starter step",
  },
  "sandboxGroupPages.starterPythonNeeds": {
    message:
      "Python 3.10 이상이 필요합니다. -U는 이전 SDK를 업그레이드합니다. 1.4.0 이하 버전은 샌드박스를 지원하지 않습니다.",
    description: "Starter step help",
  },
  "sandboxGroupPages.starterNodeNeeds": {
    message:
      "Node.js 18 이상이 필요합니다. @latest는 이미 설치된 이전 SDK도 업그레이드합니다.",
    description: "Starter step help",
  },
  "sandboxGroupPages.starterFirstSandboxFile": {
    message: "첫 샌드박스 초기화",
    description: "Starter step",
  },
  "sandboxGroupPages.starterRun": {
    message: "로컬에서 코드 실행",
    description: "Starter step",
  },
  "sandboxGroupPages.starterTypeScript": {
    message: "TypeScript",
    description: "SDK language",
  },
  "sandboxGroupPages.starterPython": {
    message: "Python",
    description: "SDK language",
  },
  "sandboxGroupPages.colSandbox": {
    message: "샌드박스",
    description: "Column",
  },
  "sandboxGroupPages.colStatus": { message: "상태", description: "Column" },
  "sandboxGroupPages.colCreated": { message: "생성일", description: "Column" },
  "sandboxGroupPages.colDuration": {
    message: "소요 시간",
    description: "Column",
  },
  "sandboxGroupPages.colPlan": { message: "요금제", description: "Column" },
  "sandboxGroupPages.colSource": {
    message: "시작 원본",
    description: "Column: the image or snapshot a sandbox started from",
  },
  "sandboxGroupPages.selectAll": {
    message: "이 페이지의 모든 샌드박스 선택",
    description: "Accessible name of the select-all checkbox",
  },
  "sandboxGroupPages.selectSandbox": {
    message: "{name} 선택",
    description: "Accessible name of a row's checkbox",
  },
  "sandboxGroupPages.baseImage": {
    message: "기본 이미지",
    description: "A sandbox started from the default image",
  },
  "sandboxGroupPages.importedImage": {
    message: "가져온 이미지",
    description: "A sandbox started from an imported image",
  },
  "sandboxGroupPages.previousPage": {
    message: "이전",
    description: "Paging button",
  },
  "sandboxGroupPages.nextPage": {
    message: "다음",
    description: "Paging button",
  },
  "sandboxGroupPages.switchedToRunning": {
    message: "선택한 샌드박스가 중지되어 실행 중인 샌드박스로 전환했습니다.",
    description: "Toast when the open sandbox stops",
  },
  "sandboxGroupPages.copied": { message: "복사됨", description: "Copy toast" },
  "sandboxGroupPages.copyError": {
    message: "복사하지 못했습니다",
    description: "Copy failed toast",
  },
  "sandboxGroupPages.copySandboxId": {
    message: "샌드박스 ID 복사",
    description: "Copy button",
  },
  "sandboxGroupPages.copySnapshotId": {
    message: "스냅샷 ID 복사",
    description: "Copy button",
  },
  "sandboxGroupPages.tabEvents": {
    message: "이벤트",
    description: "Drawer tab",
  },
  "sandboxGroupPages.tabSnapshots": {
    message: "스냅샷",
    description: "Drawer tab",
  },
  "sandboxGroupPages.tabDetails": {
    message: "세부 정보",
    description: "Drawer tab and section",
  },
  "sandboxGroupPages.tabVersions": {
    message: "버전",
    description: "Image drawer tab",
  },
  "sandboxGroupPages.sandboxGoneTitle": {
    message: "샌드박스를 찾을 수 없음",
    description: "A sandbox that does not exist here",
  },
  "sandboxGroupPages.sandboxGoneDescription": {
    message: "{id}은(는) 이 그룹에서 볼 수 있는 샌드박스가 아닙니다.",
    description: "A sandbox that does not exist here",
  },
  "sandboxGroupPages.loadingEvents": {
    message: "이벤트를 불러오는 중",
    description: "Events tab loading",
  },
  "sandboxGroupPages.loadEventsError": {
    message: "이벤트를 불러오지 못했습니다. 잠시 후 다시 시도하세요.",
    description: "Events tab error",
  },
  "sandboxGroupPages.noEvents": {
    message: "아직 이벤트가 없습니다.",
    description: "Events tab empty",
  },
  "sandboxGroupPages.loadEarlierEvents": {
    message: "이전 이벤트 불러오기",
    description: "Button: the next page of events",
  },
  "sandboxGroupPages.loadingEarlierEvents": {
    message: "이전 이벤트를 불러오는 중",
    description: "Loading the next page of events",
  },
  "sandboxGroupPages.eventCreated": {
    message: "생성됨",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventReady": {
    message: "준비됨",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventSuspending": {
    message: "일시 중지 중",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventSuspended": {
    message: "일시 중지됨",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventResuming": {
    message: "재개 중",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventTerminated": {
    message: "종료됨",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventErrored": {
    message: "오류 발생",
    description: "Lifecycle event",
  },
  "sandboxGroupPages.eventCommand": {
    message: "명령 ({operation})",
    description: "An execution event that ran a command",
  },
  "sandboxGroupPages.eventFileTransfer": {
    message: "파일 전송 ({operation})",
    description: "An execution event that moved a file",
  },
  "sandboxGroupPages.exitCode": {
    message: "종료 코드 {code}",
    description: "A command's exit code",
  },
  "sandboxGroupPages.eventSnapshot": {
    message: "스냅샷",
    description: "A snapshot event",
  },
  "sandboxGroupPages.viewSnapshot": {
    message: "스냅샷 보기",
    description: "Link to a snapshot's drawer",
  },
  "sandboxGroupPages.triggerSdk": {
    message: "SDK",
    description: "Trigger: the SDK",
  },
  "sandboxGroupPages.triggerCli": {
    message: "CLI",
    description: "Trigger: the CLI",
  },
  "sandboxGroupPages.triggerApi": {
    message: "API",
    description: "Trigger: the API",
  },
  "sandboxGroupPages.triggerDashboard": {
    message: "대시보드",
    description: "Trigger: the dashboard",
  },
  "sandboxGroupPages.triggerIdleTimeout": {
    message: "유휴 시간 제한",
    description: "Trigger: the idle timeout",
  },
  "sandboxGroupPages.triggerSystem": {
    message: "bex",
    description: "Trigger: the platform itself",
  },
  "sandboxGroupPages.triggerConnection": {
    message: "새 명령 또는 파일 전송",
    description: "Trigger: a resume a command started",
  },
  "sandboxGroupPages.loading": {
    message: "불러오는 중",
    description: "Loading text",
  },
  "sandboxGroupPages.loadSnapshotsError": {
    message: "스냅샷을 불러오지 못했습니다",
    description: "Snapshots failed to load",
  },
  "sandboxGroupPages.noSnapshotsRunning": {
    message:
      "이 샌드박스의 스냅샷이 아직 없습니다. CLI 또는 SDK로 스냅샷을 캡처하세요.",
    description: "Snapshots tab of a running sandbox",
  },
  "sandboxGroupPages.noSnapshotsEnded": {
    message: "이 샌드박스에서 생성된 스냅샷이 없습니다.",
    description: "Snapshots tab of an ended sandbox",
  },
  "sandboxGroupPages.detailSandboxId": {
    message: "샌드박스 ID",
    description: "Detail label",
  },
  "sandboxGroupPages.detailGroup": {
    message: "그룹",
    description: "Detail label",
  },
  "sandboxGroupPages.detailPlan": {
    message: "요금제",
    description: "Detail label",
  },
  "sandboxGroupPages.planSize": {
    message: "{plan} (CPU {cpus}코어, 메모리 {memory}GB)",
    description: "A plan with its size",
  },
  "sandboxGroupPages.detailCreated": {
    message: "생성일",
    description: "Detail label",
  },
  "sandboxGroupPages.detailEnded": {
    message: "종료일",
    description: "Detail label",
  },
  "sandboxGroupPages.detailDuration": {
    message: "소요 시간",
    description: "Detail label",
  },
  "sandboxGroupPages.detailTimeout": {
    message: "시간 제한",
    description: "Detail label",
  },
  "sandboxGroupPages.detailIdleTimeout": {
    message: "유휴 시간 제한",
    description: "Detail label",
  },
  "sandboxGroupPages.off": {
    message: "꺼짐",
    description: "A timeout that is off",
  },
  "sandboxGroupPages.detailNetwork": {
    message: "네트워크",
    description: "Detail label",
  },
  "sandboxGroupPages.networkAllowAll": {
    message: "모두 허용",
    description: "Network policy",
  },
  "sandboxGroupPages.networkDenyAll": {
    message: "모두 거부",
    description: "Network policy",
  },
  "sandboxGroupPages.networkAllowList": {
    message: "허용 목록",
    description: "Network policy",
  },
  "sandboxGroupPages.detailRegion": {
    message: "리전",
    description: "Detail label",
  },
  "sandboxGroupPages.detailSource": {
    message: "시작 원본",
    description: "Detail label",
  },
  "sandboxGroupPages.goToSnapshot": {
    message: "스냅샷으로 이동",
    description: "Link to the snapshot a sandbox started from",
  },
  "sandboxGroupPages.goToImage": {
    message: "이미지로 이동",
    description: "Link to the image a sandbox started from",
  },
  "sandboxGroupPages.detailTrigger": {
    message: "트리거",
    description: "Detail label: what started the sandbox",
  },
  "sandboxGroupPages.detailUser": {
    message: "사용자",
    description: "Detail label: who started the sandbox",
  },
  "sandboxGroupPages.detailError": {
    message: "오류",
    description: "Detail label",
  },
  "sandboxGroupPages.errorUnknown": {
    message:
      "bex가 이 샌드박스에서 오류가 발생한 원인을 확인하지 못했습니다. 다시 시도하거나 샌드박스 ID와 함께 지원팀에 문의하세요.",
    description: "An errored sandbox with no known reason",
  },
  "sandboxGroupPages.snapshotCapturing": {
    message: "캡처 중",
    description: "Snapshot status",
  },
  "sandboxGroupPages.snapshotAvailable": {
    message: "사용 가능",
    description: "Snapshot status",
  },
  "sandboxGroupPages.snapshotFailed": {
    message: "실패",
    description: "Snapshot status",
  },
  "sandboxGroupPages.snapshotFailureInternal": {
    message: "내부 오류로 캡처가 중단되었습니다.",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailureTimeout": {
    message: "이 캡처는 시간이 초과되었습니다.",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailureSourceTerminated": {
    message: "캡처가 끝나기 전에 원본 샌드박스가 종료되었습니다.",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailureSourceErrored": {
    message: "캡처가 끝나기 전에 원본 샌드박스에서 오류가 발생했습니다.",
    description: "Snapshot failure",
  },
  "sandboxGroupPages.snapshotFailedHelp": {
    message:
      "다시 시도하려면 새 스냅샷을 만드세요. 이 스냅샷에서는 새 샌드박스를 시작할 수 없습니다.",
    description: "After a snapshot failure",
  },
  "sandboxGroupPages.noMatchingSnapshots": {
    message: "일치하는 스냅샷이 없습니다",
    description: "Empty state with filters",
  },
  "sandboxGroupPages.firstSnapshotTitle": {
    message: "첫 스냅샷 만들기",
    description: "Empty state title of the Snapshots page",
  },
  "sandboxGroupPages.firstSnapshotDescription": {
    message:
      "스냅샷을 캡처해 샌드박스의 파일을 저장하세요. 모든 설정을 다시 하지 않고도 같은 지점에서 새 샌드박스를 시작할 수 있습니다.",
    description: "Empty state body of the Snapshots page",
  },
  "sandboxGroupPages.firstSnapshotNeedsRunning": {
    message: "스냅샷을 만들려면 실행 중인 샌드박스가 필요합니다.",
    description: "Empty state note",
  },
  "sandboxGroupPages.starterSnapshotSandbox": {
    message: "실행 중인 샌드박스의 스냅샷 만들기",
    description: "Starter step",
  },
  "sandboxGroupPages.starterStartFromSnapshot": {
    message: "스냅샷에서 샌드박스 만들기",
    description: "Starter step",
  },
  "sandboxGroupPages.starterSnapshotCode": {
    message: "코드 설정",
    description: "Starter step",
  },
  "sandboxGroupPages.colSnapshot": { message: "스냅샷", description: "Column" },
  "sandboxGroupPages.colSourceSandbox": {
    message: "원본 샌드박스",
    description: "Column and detail label",
  },
  "sandboxGroupPages.colSize": {
    message: "크기",
    description: "Column and detail label",
  },
  "sandboxGroupPages.colCaptured": {
    message: "캡처 시각",
    description: "Column and detail label",
  },
  "sandboxGroupPages.colExpires": {
    message: "만료",
    description: "Column and detail label",
  },
  "sandboxGroupPages.never": {
    message: "만료 안 됨",
    description: "A snapshot that never expires",
  },
  "sandboxGroupPages.snapshotGoneTitle": {
    message: "스냅샷을 찾을 수 없음",
    description: "A snapshot that does not exist here",
  },
  "sandboxGroupPages.snapshotGoneDescription": {
    message:
      "{id}은(는) 이 워크스페이스에서 볼 수 있는 스냅샷이 아닙니다. 삭제되었거나 만료되었을 수 있습니다.",
    description: "A snapshot that does not exist here",
  },
  "sandboxGroupPages.detailType": {
    message: "유형",
    description: "Detail label: snapshot kind",
  },
  "sandboxGroupPages.kindFilesystem": {
    message: "파일 시스템",
    description: "Snapshot kind",
  },
  "sandboxGroupPages.kindRuntime": {
    message: "런타임",
    description: "Snapshot kind",
  },
  "sandboxGroupPages.detailResourceConstraint": {
    message: "리소스 제약",
    description: "Detail label: the plan a snapshot needs",
  },
  "sandboxGroupPages.detailLifecycle": {
    message: "수명 주기",
    description: "Detail label",
  },
  "sandboxGroupPages.lifecycleKept": {
    message: "삭제할 때까지 보관",
    description: "A snapshot with no time to live",
  },
  "sandboxGroupPages.lifecycleAfterUse": {
    message: "마지막 사용 후 {days}일이 지나면 만료",
    description: "A snapshot's time to live",
  },
  "sandboxGroupPages.noSandboxesFromSnapshot": {
    message: "이 스냅샷에서 시작된 샌드박스가 없습니다.",
    description: "Snapshot drawer's sandboxes",
  },
  "sandboxGroupPages.restoreTitle": {
    message: "이 스냅샷에서 샌드박스 시작",
    description: "Snapshot drawer starter code",
  },
  "sandboxGroupPages.deleteSnapshot": {
    message: "스냅샷 삭제",
    description: "Button",
  },
  "sandboxGroupPages.deleteSnapshotTitle": {
    message: "스냅샷을 삭제하시겠습니까?",
    description: "Confirmation title",
  },
  "sandboxGroupPages.deleteSnapshotDescription": {
    message:
      "{name}이(가) 영구적으로 삭제됩니다. 새 샌드박스는 이 스냅샷에서 시작할 수 없습니다. 이미 이 스냅샷에서 시작된 샌드박스는 계속 실행됩니다.",
    description: "Confirmation body",
  },
  "sandboxGroupPages.deleteSnapshotError": {
    message: "스냅샷을 삭제하지 못했습니다. 다시 시도하세요.",
    description: "Toast",
  },
  "sandboxGroupPages.snapshotDeleted": {
    message: "스냅샷 {name}을(를) 삭제했습니다",
    description: "Toast",
  },
  "sandboxGroupPages.imageQueued": {
    message: "대기 중",
    description: "Image status",
  },
  "sandboxGroupPages.imageImporting": {
    message: "가져오는 중",
    description: "Image status",
  },
  "sandboxGroupPages.imageAvailable": {
    message: "사용 가능",
    description: "Image status",
  },
  "sandboxGroupPages.imageUnavailable": {
    message: "사용 불가",
    description: "Image status: no version can start a sandbox",
  },
  "sandboxGroupPages.imageFailed": {
    message: "실패",
    description: "Image version status",
  },
  "sandboxGroupPages.imageLatest": {
    message: "최신",
    description: "The newest available version of an image",
  },
  "sandboxGroupPages.imageImportingHelp": {
    message:
      "이 버전이 완료될 때까지 샌드박스는 사용 가능한 최신 버전에서 시작됩니다.",
    description: "A version still importing",
  },
  "sandboxGroupPages.imageFailureTimeout": {
    message: "가져오기가 너무 오래 걸렸습니다.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureRetries": {
    message: "여러 번 시도했지만 가져오기에 실패했습니다.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureCredentials": {
    message:
      "bex가 이미지를 내려받지 못했습니다. 레지스트리 자격 증명을 확인하세요.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureNotFound": {
    message: "bex가 이미지를 찾지 못했습니다. 이미지 이름과 태그를 확인하세요.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailurePull": {
    message: "bex가 이미지를 내려받지 못했습니다.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailurePlatform": {
    message:
      "이 이미지는 {platform}용입니다. 샌드박스는 linux/amd64만 지원합니다.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureMissing": {
    message:
      "이미지에 샌드박스 시작에 필요한 {what}이(가) 없습니다. slim 또는 Alpine 변형처럼 셸이 포함된 이미지를 사용하세요.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureConvert": {
    message: "bex가 이미지를 변환하지 못했습니다. 다시 가져와 보세요.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureLayers": {
    message: "bex가 이미지 레이어를 읽지 못했습니다.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureStart": {
    message: "가져오기를 시작하지 못했습니다.",
    description: "Image failure",
  },
  "sandboxGroupPages.imageFailureStopped": {
    message: "가져오기가 완료되기 전에 중지되었습니다.",
    description: "Image failure",
  },
  "sandboxGroupPages.loadImagesError": {
    message: "이미지를 불러오지 못했습니다",
    description: "Images failed to load",
  },
  "sandboxGroupPages.noMatchingImages": {
    message: "일치하는 이미지가 없습니다",
    description: "Empty state with a search",
  },
  "sandboxGroupPages.firstImageTitle": {
    message: "첫 이미지 가져오기",
    description: "Empty state title of the Custom images page",
  },
  "sandboxGroupPages.firstImageDescription": {
    message:
      "레지스트리에서 이미지를 가져오면 샌드박스를 그 이미지에서 시작할 수 있습니다. CLI 또는 SDK로 이미지를 가져온 다음 그 이미지에서 샌드박스를 시작하세요.",
    description: "Empty state body of the Custom images page",
  },
  "sandboxGroupPages.starterImportImage": {
    message: "이미지 가져오기 (linux/amd64)",
    description: "Starter step",
  },
  "sandboxGroupPages.starterStartFromImage": {
    message: "이미지에서 샌드박스 만들기",
    description: "Starter step",
  },
  "sandboxGroupPages.starterImportCode": {
    message: "이미지 가져오기",
    description: "Starter step",
  },
  "sandboxGroupPages.colImage": { message: "이미지", description: "Column" },
  "sandboxGroupPages.colSourceImage": {
    message: "소스",
    description: "Column: the registry image",
  },
  "sandboxGroupPages.colLatestAvailable": {
    message: "사용 가능한 최신 버전",
    description: "Column: the newest available version's digest",
  },
  "sandboxGroupPages.colUpdated": {
    message: "업데이트됨",
    description: "Column: the newest import",
  },
  "sandboxGroupPages.imageGoneTitle": {
    message: "이미지를 찾을 수 없음",
    description: "An image name with no versions",
  },
  "sandboxGroupPages.imageGoneDescription": {
    message: "이 그룹에는 {name} 이미지가 없습니다.",
    description: "An image name with no versions",
  },
  "sandboxGroupPages.detailLatestAvailable": {
    message: "사용 가능한 최신 버전",
    description: "Detail label",
  },
  "sandboxGroupPages.detailImageSource": {
    message: "소스",
    description: "Detail label: the registry",
  },
  "sandboxGroupPages.detailImage": {
    message: "이미지",
    description: "Detail label",
  },
  "sandboxGroupPages.detailDigest": {
    message: "다이제스트",
    description: "Detail label",
  },
  "sandboxGroupPages.detailVersionId": {
    message: "버전 ID",
    description: "Detail label",
  },
  "sandboxGroupPages.noSandboxesFromImage": {
    message: "이 이미지에서 시작된 샌드박스가 없습니다.",
    description: "Image drawer's sandboxes",
  },
  "sandboxGroupPages.general": {
    message: "일반",
    description: "Settings section",
  },
  "sandboxGroupPages.makeDefault": {
    message: "기본값으로 설정",
    description: "Button: make this the workspace's default group",
  },
  "sandboxGroupPages.madeDefault": {
    message: "이제 {name}이(가) 기본 샌드박스 그룹입니다",
    description: "Toast",
  },
  "sandboxGroupPages.save": { message: "저장", description: "Button" },
  "sandboxGroupPages.limitSaved": {
    message: "동시 실행 한도를 저장했습니다",
    description: "Toast",
  },
  "sandboxGroupPages.brokeredCredentials": {
    message: "브로커 자격 증명",
    description: "What a group delete removes",
  },
  "sandboxGroupPages.deleteGroup": {
    message: "샌드박스 그룹 삭제",
    description: "Button and section",
  },
  "sandboxGroupPages.deleteGroupHelp": {
    message: "이 그룹을 샌드박스, 스냅샷, 이미지, 자격 증명과 함께 삭제합니다.",
    description: "Delete section help",
  },
  "sandboxGroupPages.deleteGroupDefault": {
    message:
      "워크스페이스의 기본 그룹은 삭제할 수 없습니다. 먼저 다른 그룹을 기본값으로 설정하세요.",
    description: "Delete section help for the default group",
  },
  "sandboxGroupPages.deleteGroupTitle": {
    message: "샌드박스 그룹을 삭제하시겠습니까?",
    description: "Confirmation title",
  },
  "sandboxGroupPages.deleteGroupDescription": {
    message:
      "샌드박스 그룹 {name}이(가) 실행 기록, 로그, 설정과 함께 영구적으로 삭제됩니다:",
    description: "Confirmation body",
  },
  "sandboxGroupPages.deleteGroupStops": {
    message:
      "SDK, CLI 또는 API로 이 그룹을 사용하는 모든 항목이 작동을 멈춥니다.",
    description: "Confirmation body",
  },
  "sandboxGroupPages.groupDeleted": {
    message: "샌드박스 그룹 {name}을(를) 삭제했습니다",
    description: "Toast",
  },
  "sandboxGroupPages.linkLoadError": {
    message: "링크를 열지 못했습니다. 잠시 후 다시 시도하세요.",
    description: "A link by id failed to resolve",
  },
  "sandboxGroupPages.backToGroups": {
    message: "샌드박스 그룹으로 이동",
    description: "Button",
  },
};

export default ko;
