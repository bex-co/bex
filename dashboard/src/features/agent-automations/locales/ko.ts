import type { TranslationEntry } from "@/i18n";

const koAgentAutomations: Record<string, TranslationEntry> = {
  "agentAutomations.pageTitle": {
    message: "자동화",
    description:
      "Page title and sidebar label of the scheduled agent automations page",
  },
  "agentAutomations.detailTitle": {
    message: "자동화",
    description: "Browser tab title of one automation's page",
  },
  "agentAutomations.pageDescription": {
    message:
      "일정에 따라 에이전트 세션을 시작합니다. 실행할 때마다 여기에 저장한 작업과 설정이 사용됩니다.",
    description: "Automations list page description",
  },
  "agentAutomations.create": {
    message: "자동화 만들기",
    description:
      "Create automation button on the list page and in the empty state",
  },
  "agentAutomations.createTitle": {
    message: "자동화 만들기",
    description: "Heading of the create form",
  },
  "agentAutomations.editTitle": {
    message: "자동화 편집",
    description: "Heading of the edit form",
  },
  "agentAutomations.createSubmit": {
    message: "자동화 만들기",
    description: "Create form submit button",
  },
  "agentAutomations.editSubmit": {
    message: "변경 사항 저장",
    description: "Edit form submit button",
  },
  "agentAutomations.createDenied": {
    message: "개발자 역할 이상의 멤버만 자동화를 만들 수 있습니다.",
    description: "Create page notice for a member who cannot create",
  },
  "agentAutomations.backToList": {
    message: "자동화 목록으로 돌아가기",
    description: "Accessible label of the back link to the automations list",
  },
  "agentAutomations.backToAutomation": {
    message: "자동화로 돌아가기",
    description: "Button returning from the edit page to the automation's page",
  },
  "agentAutomations.tabMine": {
    message: "내 자동화",
    description: "List tab: automations the viewer created",
  },
  "agentAutomations.tabAll": {
    message: "전체",
    description: "List tab: every automation in the workspace",
  },
  "agentAutomations.emptyMineTitle": {
    message: "아직 자동화가 없습니다",
    description: "Empty state title on the Mine tab",
  },
  "agentAutomations.emptyAllTitle": {
    message: "이 워크스페이스에는 아직 자동화가 없습니다",
    description: "Empty state title on the All tab",
  },
  "agentAutomations.emptyBody": {
    message:
      "자동화는 선택한 작업, 저장소, 한도로 평일 매일 09:00 같은 일정에 따라 에이전트 세션을 시작합니다.",
    description: "Empty state explanation",
  },
  "agentAutomations.loadMore": {
    message: "더 보기",
    description: "Load the next page of automations or runs",
  },
  "agentAutomations.loadFailed": {
    message: "자동화를 불러오지 못했습니다.",
    description: "Load error",
  },
  "agentAutomations.retry": {
    message: "다시 시도",
    description: "Retry button on a load error",
  },
  "agentAutomations.forbidden": {
    message: "개발자 역할 이상의 멤버만 자동화를 사용할 수 있습니다.",
    description: "Load error for a member without access",
  },
  "agentAutomations.notFound": {
    message: "이 자동화가 존재하지 않거나 삭제되었습니다.",
    description: "Load error for a missing automation",
  },
  "agentAutomations.noTrigger": {
    message: "일정 없음",
    description: "Row text when an automation has no schedule",
  },
  "agentAutomations.nextRun": {
    message: "다음 실행",
    description: "Label before the next run time",
  },
  "agentAutomations.lastRun": {
    message: "마지막 실행",
    description: "Label before the last run time",
  },
  "agentAutomations.neverRun": {
    message: "없음",
    description: "Last run value when it has not run",
  },
  "agentAutomations.lastSession": {
    message: "세션 보기",
    description: "Link to the session the last run started",
  },
  "agentAutomations.pauseLabel": {
    message: "{name} 일시 중지",
    description: "Accessible name of the switch that pauses an automation",
  },
  "agentAutomations.resumeLabel": {
    message: "{name} 재개",
    description: "Accessible name of the switch that resumes an automation",
  },
  "agentAutomations.pausedBadge": {
    message: "일시 중지됨",
    description: "Badge on an automation a person paused",
  },
  "agentAutomations.ended": {
    message: "종료됨",
    description: "Badge on an automation with no future run",
  },
  "agentAutomations.active": {
    message: "활성",
    description: "Status of a running automation",
  },
  "agentAutomations.paused.repeated_failures": {
    message: "일시 중지됨: 반복 실패",
    description: "Badge: paused by the platform after repeated failed runs",
  },
  "agentAutomations.paused.creator_lost_access": {
    message: "일시 중지됨: 작성자 권한 상실",
    description:
      "Badge: paused because the creator can no longer start sessions",
  },
  "agentAutomations.paused.template_invalid": {
    message: "일시 중지됨: 템플릿 유효하지 않음",
    description:
      "Badge: paused because the session template no longer validates",
  },
  "agentAutomations.paused.unknown": {
    message: "플랫폼에서 일시 중지함",
    description: "Badge for a pause reason this version does not know",
  },
  "agentAutomations.pausedExplain.repeated_failures": {
    message:
      "이 자동화는 {failures}회 연속 실행에 실패하여 일시 중지되었습니다. 실행 기록을 확인하고 원인을 해결한 후 재개하세요.",
    description: "Explanation on the detail page for a repeated-failures pause",
  },
  "agentAutomations.pausedExplain.creator_lost_access": {
    message:
      "이 자동화를 만든 멤버가 더 이상 세션을 시작할 권한이 없습니다. 워크스페이스 관리자가 인수하면 재개됩니다.",
    description:
      "Explanation on the detail page for a creator-lost-access pause",
  },
  "agentAutomations.pausedExplain.template_invalid": {
    message:
      "시크릿이나 모델 설정이 변경되는 등의 이유로 세션 템플릿이 더 이상 검증을 통과하지 못합니다. 작성자가 편집한 후 재개할 수 있습니다.",
    description: "Explanation on the detail page for a template-invalid pause",
  },
  "agentAutomations.pausedExplain.unknown": {
    message:
      "플랫폼에서 이 자동화를 일시 중지했습니다. 원인을 해결한 후 재개하세요.",
    description: "Explanation for a pause reason this version does not know",
  },
  "agentAutomations.pausedToast": {
    message: "{name} 자동화를 일시 중지했습니다.",
    description: "Toast after pausing an automation",
  },
  "agentAutomations.resumedToast": {
    message: "{name} 자동화를 재개했습니다.",
    description: "Toast after resuming an automation",
  },
  "agentAutomations.runStartedToast": {
    message: "{name} 자동화가 세션을 시작했습니다.",
    description: "Toast after Run now started a session",
  },
  "agentAutomations.runSettledToast": {
    message: "{name} 실행 결과: {outcome}.",
    description: "Toast after Run now ended with an outcome other than started",
  },
  "agentAutomations.tookOverToast": {
    message: "이제 {name} 자동화의 소유자는 회원님입니다.",
    description: "Toast after an admin takes an automation over",
  },
  "agentAutomations.deletedToast": {
    message: "{name} 자동화를 삭제했습니다.",
    description: "Toast after deleting an automation",
  },
  "agentAutomations.createdToast": {
    message: "{name} 자동화를 만들었습니다.",
    description: "Toast after creating an automation",
  },
  "agentAutomations.updatedToast": {
    message: "{name} 자동화를 저장했습니다.",
    description: "Toast after editing an automation",
  },
  "agentAutomations.actionFailed": {
    message: "작업을 완료하지 못했습니다. 다시 시도하세요.",
    description: "Generic failure toast for an automation action",
  },
  "agentAutomations.saveFailed": {
    message: "자동화를 저장하지 못했습니다. 다시 시도하세요.",
    description: "Generic failure message when saving the form",
  },
  "agentAutomations.actionsLabel": {
    message: "자동화 작업",
    description: "Accessible label of the action button group",
  },
  "agentAutomations.runNow": {
    message: "지금 실행",
    description: "Button: fire the automation now",
  },
  "agentAutomations.runNowPausedHint": {
    message: "실행하기 전에 이 자동화를 재개하세요.",
    description:
      "Tooltip on Run now while the automation is paused by the platform",
  },
  "agentAutomations.pause": {
    message: "일시 중지",
    description: "Button: pause the automation",
  },
  "agentAutomations.resume": {
    message: "재개",
    description: "Button: resume the automation",
  },
  "agentAutomations.edit": {
    message: "편집",
    description: "Button: edit the automation",
  },
  "agentAutomations.editCreatorOnly": {
    message: "이 자동화를 만든 멤버만 편집할 수 있습니다.",
    description: "Tooltip and notice: editing is for the creator only",
  },
  "agentAutomations.takeOver": {
    message: "인수",
    description: "Button: an admin becomes the automation's creator",
  },
  "agentAutomations.takeOverAdminOnly": {
    message:
      "이 자동화를 만들지 않은 워크스페이스 관리자만 인수할 수 있습니다.",
    description: "Tooltip on a disabled Take over button",
  },
  "agentAutomations.takeOverTitle": {
    message: "{name} 자동화를 인수하시겠습니까?",
    description: "Take over confirmation title",
  },
  "agentAutomations.takeOverBody": {
    message:
      "앞으로의 실행은 회원님의 권한으로 회원님 명의의 세션을 시작하며, 템플릿도 회원님의 권한으로 다시 검증됩니다. 작성자의 권한 상실로 일시 중지된 자동화는 재개됩니다.",
    description: "Take over confirmation body",
  },
  "agentAutomations.delete": {
    message: "삭제",
    description: "Button: delete the automation",
  },
  "agentAutomations.deleteTitle": {
    message: "{name} 자동화를 삭제하시겠습니까?",
    description: "Delete confirmation title",
  },
  "agentAutomations.deleteBody": {
    message: "자동화와 실행 기록이 삭제됩니다. 이미 시작된 세션은 유지됩니다.",
    description: "Delete confirmation body",
  },
  "agentAutomations.settingsTitle": {
    message: "설정",
    description: "Settings card title on the detail page",
  },
  "agentAutomations.row.status": {
    message: "상태",
    description: "Settings row label",
  },
  "agentAutomations.row.schedule": {
    message: "일정",
    description: "Settings row label",
  },
  "agentAutomations.row.nextRun": {
    message: "다음 실행",
    description: "Settings row label",
  },
  "agentAutomations.row.lastRun": {
    message: "마지막 실행",
    description: "Settings row label",
  },
  "agentAutomations.row.createdBy": {
    message: "만든 사람",
    description: "Settings row label",
  },
  "agentAutomations.row.repo": {
    message: "저장소",
    description: "Settings row label",
  },
  "agentAutomations.row.agent": {
    message: "에이전트",
    description: "Settings row label",
  },
  "agentAutomations.row.spendLimit": {
    message: "실행당 지출 한도",
    description: "Settings row label",
  },
  "agentAutomations.row.emailNotifications": {
    message: "이메일 알림",
    description: "Settings row label",
  },
  "agentAutomations.row.secrets": {
    message: "시크릿",
    description: "Settings row label",
  },
  "agentAutomations.row.skills": {
    message: "스킬",
    description: "Settings row label",
  },
  "agentAutomations.taskTitle": {
    message: "작업",
    description: "Task card title on the detail page",
  },
  "agentAutomations.noRepo": {
    message: "없음 (채팅 전용 세션)",
    description: "Repository value when the template has none",
  },
  "agentAutomations.noSpendLimit": {
    message: "한도 없음",
    description: "Spend limit value when the template has none",
  },
  "agentAutomations.none": {
    message: "없음",
    description: "Empty value for secrets or skills",
  },
  "agentAutomations.runsTitle": {
    message: "실행 기록",
    description: "Run history card title",
  },
  "agentAutomations.runsDescription": {
    message: "예약 실행과 수동 실행을 모두 최신순으로 표시합니다.",
    description: "Run history card description",
  },
  "agentAutomations.runsEmpty": {
    message: "아직 실행 기록이 없습니다.",
    description: "Run history empty state",
  },
  "agentAutomations.runs.fireAt": {
    message: "실행 시각",
    description: "Run table column",
  },
  "agentAutomations.runs.outcome": {
    message: "결과",
    description: "Run table column",
  },
  "agentAutomations.runs.session": {
    message: "세션",
    description: "Run table column",
  },
  "agentAutomations.runs.missed": {
    message: "누락",
    description: "Run table column: ticks skipped while the scheduler was down",
  },
  "agentAutomations.runs.error": {
    message: "오류",
    description: "Run table column",
  },
  "agentAutomations.runs.openSession": {
    message: "세션 열기",
    description: "Link to the session a run started",
  },
  "agentAutomations.outcome.started": {
    message: "시작됨",
    description: "Run outcome: a session was started",
  },
  "agentAutomations.outcome.claimed": {
    message: "진행 중",
    description: "Run outcome: claimed, not yet settled",
  },
  "agentAutomations.outcome.skipped_overlap": {
    message: "건너뜀",
    description:
      "Run outcome: skipped because the previous session was still running",
  },
  "agentAutomations.outcome.failed_create": {
    message: "실패",
    description: "Run outcome: the session could not be created",
  },
  "agentAutomations.outcome.denied": {
    message: "거부됨",
    description:
      "Run outcome: the creator was no longer allowed to start the session",
  },
  "agentAutomations.outcome.didNotStart": {
    message: "시작되지 않음",
    description:
      "Run outcome: the run started a session whose sandbox did not start for a cause on bex's side (w10/m497); the cause shows in the Error column. Not a failed create",
  },
  "agentAutomations.outcome.unknown": {
    message: "알 수 없음",
    description: "Run outcome this version does not know",
  },
  "agentAutomations.section.details": {
    message: "세부 정보",
    description: "Form section title",
  },
  "agentAutomations.section.schedule": {
    message: "일정",
    description: "Form section title",
  },
  "agentAutomations.section.scheduleDescription": {
    message:
      "자동화가 실행되는 시점입니다. 시간대에 따라 09:00의 의미가 달라집니다.",
    description: "Form section description",
  },
  "agentAutomations.section.session": {
    message: "세션",
    description: "Form section title",
  },
  "agentAutomations.section.sessionDescription": {
    message:
      "실행할 때마다 시작되는 에이전트 세션입니다. 에이전트 페이지의 새 세션과 같은 옵션을 사용합니다.",
    description: "Form section description",
  },
  "agentAutomations.field.name": {
    message: "이름",
    description: "Form field label",
  },
  "agentAutomations.field.frequency": {
    message: "반복",
    description: "Form field label",
  },
  "agentAutomations.field.minute": {
    message: "정시 이후 분",
    description: "Form field label",
  },
  "agentAutomations.field.at": {
    message: "시간",
    description: "Form field label",
  },
  "agentAutomations.field.weekdays": {
    message: "요일",
    description: "Form field label",
  },
  "agentAutomations.field.runAt": {
    message: "날짜 및 시간",
    description: "Form field label",
  },
  "agentAutomations.field.cron": {
    message: "Cron 표현식",
    description: "Form field label",
  },
  "agentAutomations.field.timezone": {
    message: "시간대",
    description: "Form field label",
  },
  "agentAutomations.field.endsOn": {
    message: "종료일 (선택 사항)",
    description: "Form field label",
  },
  "agentAutomations.field.task": {
    message: "작업",
    description: "Form field label",
  },
  "agentAutomations.field.repo": {
    message: "저장소",
    description: "Form field label",
  },
  "agentAutomations.field.agent": {
    message: "에이전트",
    description: "Form field label",
  },
  "agentAutomations.field.model": {
    message: "모델",
    description: "Automation form — model field label (w10/m516)",
  },
  "agentAutomations.field.template": {
    message: "샌드박스",
    description: "Automation form — sandbox template field label (w10/m519)",
  },
  "agentAutomations.field.spendLimit": {
    message: "실행당 지출 한도 (USD)",
    description: "Form field label",
  },
  "agentAutomations.field.emailNotifications": {
    message: "이메일 알림",
    description: "Form field label",
  },
  "agentAutomations.field.secrets": {
    message: "시크릿",
    description: "Form field label",
  },
  "agentAutomations.field.skills": {
    message: "스킬",
    description: "Form field label",
  },
  "agentAutomations.frequency.hour": {
    message: "매시간",
    description: "Frequency option",
  },
  "agentAutomations.frequency.day": {
    message: "매일",
    description: "Frequency option",
  },
  "agentAutomations.frequency.week": {
    message: "매주",
    description: "Frequency option",
  },
  "agentAutomations.frequency.once": {
    message: "한 번",
    description: "Frequency option",
  },
  "agentAutomations.frequency.custom": {
    message: "사용자 지정 (cron)",
    description: "Frequency option",
  },
  "agentAutomations.emailNotifications.always": {
    message: "항상",
    description: "Email notification option: every run emails the creator",
  },
  "agentAutomations.emailNotifications.on_failure": {
    message: "실패 시",
    description:
      "Email notification option: only failed runs email the creator",
  },
  "agentAutomations.emailNotifications.on_success": {
    message: "성공 시",
    description:
      "Email notification option: only succeeded runs email the creator",
  },
  "agentAutomations.emailNotifications.never": {
    message: "안 함",
    description: "Email notification option: no run emails the creator",
  },
  "agentAutomations.weekday.mon": {
    message: "월",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.tue": {
    message: "화",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.wed": {
    message: "수",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.thu": {
    message: "목",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.fri": {
    message: "금",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.sat": {
    message: "토",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.sun": {
    message: "일",
    description: "Weekday toggle",
  },
  "agentAutomations.help.name": {
    message: "소문자, 숫자, 단일 하이픈만 사용할 수 있으며 최대 64자입니다.",
    description: "Name field help",
  },
  "agentAutomations.help.minute": {
    message: "0~59.",
    description: "Minute field help",
  },
  "agentAutomations.help.runAt": {
    message: "아래에서 선택한 시간대({zone}) 기준입니다.",
    description: "Run-at field help",
  },
  "agentAutomations.help.cron": {
    message:
      "다섯 개 필드: 분 시 일 월 요일. 실행 간격은 5분 이상이어야 합니다.",
    description: "Cron field help",
  },
  "agentAutomations.help.endsOn": {
    message:
      "실행할 수 있는 마지막 날입니다. 비워 두면 일시 중지할 때까지 실행됩니다.",
    description: "Ends-on field help",
  },
  "agentAutomations.help.task": {
    message:
      "실행할 때마다 에이전트가 해야 할 일입니다. 기대하는 결과물을 함께 적으세요.",
    description: "Task field help",
  },
  "agentAutomations.help.repo": {
    message:
      "선택 사항입니다. 저장소가 없으면 실행할 때마다 채팅 전용 세션이 됩니다. 실행할 때마다 새 bex-agent 브랜치에서 작업합니다.",
    description: "Repository field help",
  },
  "agentAutomations.help.spendLimit": {
    message:
      "실행당 모델 지출 상한입니다. 플랫폼 모델을 사용하는 실행에는 필수입니다.",
    description: "Spend limit field help",
  },
  "agentAutomations.help.model": {
    message:
      "각 실행에서 사용할 모델입니다. 기본값으로 두면 워크스페이스 설정을 따릅니다.",
    description: "Automation form — model field help",
  },
  "agentAutomations.help.modelUnoffered": {
    message:
      "{model} 모델은 이 워크스페이스에서 더 이상 제공되지 않습니다. 관리자가 껐거나 플랫폼 카탈로그에서 빠졌습니다. 실행에는 대신 워크스페이스 기본 모델인 {fallback} 모델이 사용됩니다. 변경 사항을 저장하려면 제공되는 모델을 선택하세요.",
    description:
      "Automation form — model field note when the saved model is no longer offered (turned off by an admin or gone from the catalog); {fallback} is the workspace default its runs use (w10/m527)",
  },
  "agentAutomations.help.template": {
    message:
      "각 실행의 샌드박스가 받는 머신입니다. 워크스페이스 기본값은 실행할 때마다 설정 → 에이전트를 따릅니다.",
    description: "Automation form — sandbox template field help (w10/m519)",
  },
  "agentAutomations.help.githubCli": {
    message:
      "이 자동화의 세션이 gh를 받을지 여부입니다. 상속은 설정 → 에이전트의 “자동화의 GitHub CLI”를 따릅니다.",
    description: "Automation form — GitHub CLI field help (w10/m561)",
  },
  "agentAutomations.help.emailNotifications": {
    message:
      "어떤 실행에 대해 이메일을 받을지 선택합니다. 플랫폼에 의한 일시 중지는 항상 이메일로 알려드립니다.",
    description: "Email notifications field help",
  },
  "agentAutomations.help.secrets": {
    message:
      "에이전트가 환경 변수로 받는 워크스페이스 시크릿입니다. 워크스페이스 설정에서 추가하세요.",
    description: "Secrets field help",
  },
  "agentAutomations.noSecrets": {
    message: "시크릿 없음",
    description: "Secrets row when none are chosen",
  },
  "agentAutomations.noSkills": {
    message: "스킬 없음",
    description: "Skills row when none are chosen",
  },
  "agentAutomations.placeholder.name": {
    message: "nightly-dependency-check",
    description: "Name field placeholder (an example slug)",
  },
  "agentAutomations.placeholder.repo": {
    message: "owner/repository",
    description: "Repository field placeholder",
  },
  "agentAutomations.placeholder.model": {
    message: "워크스페이스 기본값",
    description: "Automation form — empty free-form model field",
  },
  "agentAutomations.placeholder.template": {
    message: "워크스페이스 기본값",
    description:
      "Automation form — the sandbox field while no template list is available: the workspace default",
  },
  "agentAutomations.summary": {
    message: "실행: {schedule}",
    description: "Live summary under the schedule picker",
  },
  "agentAutomations.schedule.hour": {
    message: "매시간 {minute}분",
    description: "Schedule in words, hourly",
  },
  "agentAutomations.schedule.day": {
    message: "매일 {at}",
    description: "Schedule in words, daily",
  },
  "agentAutomations.schedule.week": {
    message: "매주 {days} {at}",
    description: "Schedule in words, weekly",
  },
  "agentAutomations.schedule.once": {
    message: "{when}에 한 번",
    description: "Schedule in words, one-off",
  },
  "agentAutomations.schedule.custom": {
    message: "Cron {cron}",
    description: "Schedule in words, cron",
  },
  "agentAutomations.schedule.until": {
    message: "{date}까지",
    description: "Schedule end date suffix",
  },
  "agentAutomations.problem.nameRequired": {
    message: "이름을 입력하세요.",
    description: "Form validation",
  },
  "agentAutomations.problem.nameInvalid": {
    message: "소문자, 숫자, 단일 하이픈만 사용하세요. 최대 64자입니다.",
    description: "Form validation",
  },
  "agentAutomations.problem.taskRequired": {
    message: "작업을 설명하세요.",
    description: "Form validation",
  },
  "agentAutomations.problem.spendLimit": {
    message: "0보다 크고 100000 이하인 금액을 입력하거나 비워 두세요.",
    description: "Form validation",
  },
  "agentAutomations.problem.skills": {
    message: "스킬의 인수는 최대 4 KB의 한 줄이어야 합니다.",
    description: "Form validation",
  },
  "agentAutomations.problem.githubCliMandatory": {
    message:
      "이 워크스페이스에서는 GitHub CLI 끄기가 필수이므로 자동화에서 gh를 켤 수 없습니다. 꺼짐 또는 상속을 선택하세요.",
    description:
      "Automation form — AGENT_AUTOMATION_GITHUB_CLI_LOCKED with reason mandatory (w10/m561)",
  },
  "agentAutomations.problem.githubCliAdminRequired": {
    message:
      "워크스페이스 관리자만 자동화의 GitHub CLI를 선택하거나 변경할 수 있습니다. 원래 선택을 유지하세요.",
    description:
      "Automation form — AGENT_AUTOMATION_GITHUB_CLI_LOCKED with reason admin_required: only a workspace admin sets or changes an automation's gh binding (w10/044)",
  },
  "agentAutomations.problem.timezone": {
    message: "올바른 시간대를 선택하세요.",
    description: "Form validation",
  },
  "agentAutomations.problem.minute": {
    message: "0~59 사이의 분을 입력하세요.",
    description: "Form validation",
  },
  "agentAutomations.problem.at": {
    message: "시간을 입력하세요.",
    description: "Form validation",
  },
  "agentAutomations.problem.weekdays": {
    message: "요일을 하나 이상 선택하세요.",
    description: "Form validation",
  },
  "agentAutomations.problem.runAtRequired": {
    message: "날짜와 시간을 선택하세요.",
    description: "Form validation",
  },
  "agentAutomations.problem.runAtPast": {
    message: "미래의 시간을 선택하세요.",
    description: "Form validation",
  },
  "agentAutomations.problem.cron": {
    message: "다섯 개 필드를 입력하세요: 분 시 일 월 요일.",
    description: "Form validation",
  },
  "agentAutomations.problem.endsOn": {
    message: "올바른 날짜를 입력하세요.",
    description: "Form validation",
  },
  "agentAutomations.problem.endsOnPast": {
    message: "미래의 날짜를 선택하세요.",
    description: "Form validation",
  },
  "agentAutomations.startedBy": {
    message: "자동화로 시작됨",
    description: "Session page header note linking back to the automation",
  },
};

export default koAgentAutomations;
