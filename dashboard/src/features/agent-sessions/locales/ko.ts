import type { TranslationEntry } from "@/i18n";

const koAgentSessions: Record<string, TranslationEntry> = {
  "agentSessions.pageTitle": {
    message: "에이전트 세션",
    description: "Agent sessions list page heading and document title",
  },
  "agentSessions.listTitle": {
    message: "최근",
    description: "Agent sessions recents heading — matches the rail",
  },
  "agentSessions.colTask": {
    message: "작업",
    description: "Agent sessions table column header — the session's prompt",
  },
  "agentSessions.colRepo": {
    message: "저장소",
    description: "Agent sessions table column header — repo/branch",
  },
  "agentSessions.colAgent": {
    message: "에이전트",
    description: "Agent sessions table column header — the driver agent",
  },
  "agentSessions.colPhase": {
    message: "단계",
    description: "Agent sessions table column header — lifecycle phase chip",
  },
  "agentSessions.colPr": {
    message: "PR",
    description: "Agent sessions table column header — draft pull request",
  },
  "agentSessions.colCreated": {
    message: "생성",
    description: "Agent sessions table column header — createdAt relative age",
  },
  "agentSessions.prBadge": {
    message: "#{number}",
    description: "Draft PR badge label — the pull-request number",
  },
  "agentSessions.prState.open": {
    message: "열림",
    description:
      "PR state badge beside a session's PR link — the pull request is open (a draft counts as open)",
  },
  "agentSessions.prState.merged": {
    message: "병합됨",
    description:
      "PR state badge beside a session's PR link — the pull request was merged",
  },
  "agentSessions.prState.closed": {
    message: "닫힘",
    description:
      "PR state badge beside a session's PR link — the pull request was closed without merging",
  },
  "agentSessions.prAutoMerge": {
    message: "자동 병합",
    description:
      "Badge beside a session's PR state — GitHub auto-merge is armed on the pull request (w10/047)",
  },
  "agentSessions.prMergeQueued": {
    message: "병합 대기열",
    description:
      "Badge beside a session's PR state — the pull request is in its base branch's merge queue (w10/047)",
  },
  "agentSessions.prMergeQueuedHint": {
    message: "병합 대기열에 있음: 맨 앞에 오면 GitHub가 병합합니다.",
    description:
      "Tooltip of the merge-queue badge — what being in the merge queue means",
  },
  "agentSessions.prAutoMergeEnabledBy": {
    message: "{login}님이 자동 병합을 활성화함",
    description:
      "Tooltip of the auto-merge badge — the GitHub login that armed auto-merge",
  },
  "agentSessions.prAutoMergeEnabled": {
    message: "자동 병합 활성화됨",
    description:
      "Tooltip of the auto-merge badge when GitHub does not say who armed it",
  },
  "agentSessions.prAutoMergeHint": {
    message: "필수 리뷰와 검사를 통과하면 GitHub가 병합합니다.",
    description: "Tooltip of the auto-merge badge — what auto-merge does",
  },
  "agentSessions.prAutoMergeMethod.merge": {
    message: "병합 커밋 만들기",
    description:
      "Tooltip of the auto-merge badge — the merge method, GitHub's 'Create a merge commit'",
  },
  "agentSessions.prAutoMergeMethod.squash": {
    message: "스쿼시 후 병합",
    description:
      "Tooltip of the auto-merge badge — the merge method, GitHub's 'Squash and merge'",
  },
  "agentSessions.prAutoMergeMethod.rebase": {
    message: "리베이스 후 병합",
    description:
      "Tooltip of the auto-merge badge — the merge method, GitHub's 'Rebase and merge'",
  },
  "agentSessions.prStateFilter": {
    message: "풀 리퀘스트 상태로 필터링",
    description:
      "Session list — accessible name of the Any/Open/Merged/Closed PR-state filter",
  },
  "agentSessions.prStateFilterAny": {
    message: "전체",
    description:
      "Session list PR-state filter — the option that shows sessions whatever their PR state, including sessions with no PR",
  },
  "agentSessions.errorTitle": {
    message: "세션을 불러오지 못했습니다.",
    description: "Agent sessions list error — one sentence, Retry beside it",
  },
  "agentSessions.retry": {
    message: "다시 시도",
    description: "Retries a failed agent-sessions list request",
  },
  "agentSessions.emptyBody": {
    message: "시작한 세션이 여기에 표시됩니다.",
    description: "Default empty — one quiet line under the composer",
  },
  "agentSessions.emptyArchivedTitle": {
    message: "보관된 세션이 없습니다",
    description: "Archived-only list empty-state heading",
  },
  "agentSessions.emptyArchivedBody": {
    message: "보관한 세션이 여기에 표시됩니다.",
    description: "Archived-only list empty-state body",
  },
  "agentSessions.emptyFilteredTitle": {
    message: "일치하는 세션이 없습니다",
    description: "Phase-filtered list empty-state heading",
  },
  "agentSessions.emptyFilteredBody": {
    message: "다른 단계를 선택하거나 필터를 지우세요.",
    description: "Phase-filtered list empty-state body",
  },
  "agentSessions.clearFilters": {
    message: "필터 지우기",
    description: "Clears agent-session membership and phase filters",
  },
  "agentSessions.phase.creating": {
    message: "생성 중",
    description: "Agent session phase chip — sandbox is being created",
  },
  "agentSessions.phase.running": {
    message: "실행 중",
    description: "Agent session phase chip — a turn is in progress",
  },
  "agentSessions.phase.resuming": {
    message: "재개 중",
    description: "Agent session phase chip — session is resuming",
  },
  "agentSessions.phase.redispatching": {
    message: "재전달 중",
    description: "Agent session phase chip — session is being redispatched",
  },
  "agentSessions.phase.completed": {
    message: "완료됨",
    description: "Agent session phase chip — session finished successfully",
  },
  "agentSessions.phase.failed": {
    message: "실패",
    description: "Agent session phase chip — session failed",
  },
  "agentSessions.phase.canceling": {
    message: "취소 중",
    description: "Agent session phase chip — cancellation in progress",
  },
  "agentSessions.phase.canceled": {
    message: "취소됨",
    description: "Agent session phase chip — session was canceled",
  },
  "agentSessions.taskLabel": {
    message: "작업",
    description: "Composer — the prominent task prompt textarea label",
  },
  "agentSessions.taskPlaceholder": {
    message: "코딩 작업을 설명하세요…",
    description: "Composer — task editor placeholder",
  },
  "agentSessions.mentionButton": {
    message: "저장소 또는 세션 추가",
    description:
      "Composer toolbar — opens the repository/session mention picker",
  },
  "agentSessions.repoChip": {
    message: "저장소 {repo}",
    description: "Composer toolbar — accessible name of the selected-repo chip",
  },
  "agentSessions.keyboardHint": {
    message: "Enter로 시작 · Shift+Enter로 줄바꿈",
    description: "Muted hint under the composer",
  },
  "agentSessions.connectGitHubTitle": {
    message: "시작하려면 GitHub를 연결하세요",
    description: "Composer callout when the workspace has no App repos",
  },
  "agentSessions.connectGitHubBody": {
    message:
      "클라우드 에이전트를 사용하려면 연결된 GitHub 저장소가 필요합니다. 이 워크스페이스에 GitHub App을 연결한 후 돌아와서 세션을 시작하세요.",
    description: "Composer callout body when there are no installation repos",
  },
  "agentSessions.connectGitHubSettings": {
    message: "GitHub 설정",
    description:
      "Secondary link from the composer's GitHub callout to Account Settings → Integrations (/settings#integrations), where the GitHub card lives",
  },
  "agentSessions.exampleFixTests": {
    message: "실패하는 테스트를 수정하고 초안 PR 열기",
    description: "First-run example prompt chip",
  },
  "agentSessions.exampleAddReadme": {
    message: "프로젝트 실행 방법을 설명하는 README 추가",
    description: "First-run example prompt chip",
  },
  "agentSessions.chipRemove": {
    message: "{name} 제거",
    description: "Accessible label of a mention chip's remove button",
  },
  "agentSessions.mentionCategoryRepos": {
    message: "저장소",
    description: "Mention picker — the repositories category row",
  },
  "agentSessions.mentionCategoryReposDesc": {
    message: "세션을 연결된 GitHub 저장소로 한정합니다",
    description: "Mention picker — repositories category one-line description",
  },
  "agentSessions.mentionCategorySessions": {
    message: "세션",
    description: "Mention picker — the prior-sessions category row",
  },
  "agentSessions.mentionCategorySessionsDesc": {
    message: "이전 에이전트 세션을 컨텍스트로 참조합니다",
    description: "Mention picker — sessions category one-line description",
  },
  "agentSessions.mentionNoResults": {
    message: "일치하는 항목 없음",
    description:
      "Mention picker — empty state when the fuzzy filter matches nothing",
  },
  "agentSessions.mentionReposEmpty": {
    message:
      "아직 저장소가 없습니다. GitHub 계정을 연결하거나 bex GitHub App에 저장소 접근 권한을 부여하세요.",
    description:
      "Mention picker — empty state when no installation repos exist",
  },
  "agentSessions.mentionSessionsEmpty": {
    message: "아직 세션이 없습니다",
    description:
      "Mention picker — empty state when the workspace has no sessions",
  },
  "agentSessions.mentionConnected": {
    message: "GitHub App으로 연결됨",
    description: "Mention picker readiness footer — the repo is app-connected",
  },
  "agentSessions.mentionDefaultBranch": {
    message: "기본 브랜치: {branch}",
    description: "Mention picker readiness footer — the repo's default branch",
  },
  "agentSessions.agentLabel": {
    message: "에이전트",
    description: "Composer — agent select label",
  },
  "agentSessions.modelLabel": {
    message: "모델",
    description: "Composer — model select label (w10/m516)",
  },
  "agentSessions.modelDefaultOption": {
    message: "{model} (기본값)",
    description: "Model list — the model a session runs when it names none",
  },
  "agentSessions.modelFixed": {
    message: "이 에이전트는 항상 {model}을(를) 사용합니다",
    description: "Tooltip of the model of an agent with one fixed model",
  },
  "agentSessions.metaModel": {
    message: "모델: {model}",
    description: "Session header — tooltip of the model the session runs",
  },
  "agentSessions.sandboxLabel": {
    message: "샌드박스",
    description: "Composer — sandbox template select label (w10/m519)",
  },
  "agentSessions.sandboxSize": {
    message: "CPU {cpu}코어, 메모리 {memory}GB",
    description:
      "Sandbox template size: CPU cores and memory in GB, numbers already formatted",
  },
  "agentSessions.sandboxOption": {
    message: "{name} · {size}",
    description: "Sandbox template choice: its name, then its size",
  },
  "agentSessions.sandboxDefaultOption": {
    message: "{template} (기본값)",
    description:
      "Sandbox list — the template a session runs on when it names none (the workspace default)",
  },
  "agentSessions.sandboxWorkspaceDefault": {
    message: "워크스페이스 기본값 ({template})",
    description:
      "Automation sandbox choice — follow the workspace default at each run; {template} is the default now",
  },
  "agentSessions.metaSandbox": {
    message: "샌드박스: {template}",
    description:
      "Tooltip of the sandbox template a session runs on, with its size",
  },
  "agentSessions.githubCliInherit": {
    message: "상속",
    description:
      "GitHub CLI choice — inherit the level above, while what that resolves to is still loading (w10/m561)",
  },
  "agentSessions.githubCliInheritOn": {
    message: "상속(켜짐)",
    description:
      "GitHub CLI choice — inherit the level above, which has gh on (w10/m561)",
  },
  "agentSessions.githubCliInheritOff": {
    message: "상속(꺼짐)",
    description:
      "GitHub CLI choice — inherit the level above, which has gh off (w10/m561)",
  },
  "agentSessions.githubCliOn": {
    message: "켜짐",
    description:
      "GitHub CLI choice — gh on for this session or automation (w10/m561)",
  },
  "agentSessions.githubCliOff": {
    message: "꺼짐",
    description:
      "GitHub CLI choice — gh off for this session or automation (w10/m561)",
  },
  "agentSessions.githubCliOnMandatory": {
    message:
      "이 워크스페이스에서 GitHub CLI 끄기가 필수인 동안에는 사용할 수 없습니다.",
    description:
      "Why the GitHub CLI On choice is disabled: the workspace's off is mandatory (w10/m561)",
  },
  "agentSessions.githubCliAdminOnly": {
    message: "워크스페이스 관리자만 변경할 수 있습니다.",
    description:
      "Why a GitHub CLI choice is disabled: only a workspace admin chooses or changes a session's or an automation's gh (w10/044)",
  },
  "agentSessions.githubCliTitle": {
    message: "이 세션의 GitHub CLI: {choice}",
    description:
      "Tooltip of the composer's GitHub CLI control; {choice} is the chosen binding, e.g. Inherit (on) (w10/m561)",
  },
  "agentSessions.errors.AGENT_SESSION_GITHUB_CLI_LOCKED": {
    message:
      "이 워크스페이스에서는 GitHub CLI 끄기가 필수이므로 세션에서 gh를 켤 수 없습니다. 꺼짐 또는 상속을 선택하세요.",
    description:
      "Mapped message for AGENT_SESSION_GITHUB_CLI_LOCKED with reason mandatory (w10/m561)",
  },
  "agentSessions.errors.AGENT_SESSION_GITHUB_CLI_LOCKED_adminRequired": {
    message:
      "워크스페이스 관리자만 세션의 GitHub CLI를 선택할 수 있습니다. 상속을 선택하세요.",
    description:
      "Mapped message for AGENT_SESSION_GITHUB_CLI_LOCKED with reason admin_required: only a workspace admin names a session's gh binding (w10/044)",
  },
  "agentSessions.agent.claude": {
    message: "Claude",
    description: "Composer — agent option: claude",
  },
  "agentSessions.agent.gemini": {
    message: "Gemini",
    description: "Composer — agent option: gemini",
  },
  "agentSessions.agent.codex": {
    message: "Codex",
    description: "Composer — agent option: codex",
  },
  "agentSessions.agent.muse": {
    message: "Muse Code",
    description: "Composer — agent option: muse (Meta's Muse Code)",
  },
  "agentSessions.agent.deepseek": {
    message: "DeepSeek Harness",
    description:
      "Composer — agent option: deepseek (DeepSeek Harness, on DeepSeek Flash through OpenRouter)",
  },
  "agentSessions.submit": {
    message: "세션 시작",
    description: "Composer — submit button label",
  },
  "agentSessions.submitting": {
    message: "시작하는 중…",
    description: "Composer — submit button label while the create is in flight",
  },
  "agentSessions.unavailableTitle": {
    message: "에이전트 세션이 구성되지 않았습니다",
    description: "Composer — house callout title when the feature is 503",
  },
  "agentSessions.unavailableBody": {
    message:
      "이 플랫폼에서는 클라우드 에이전트 세션이 활성화되지 않았습니다. 운영자에게 에이전트 세션 게이트웨이 구성을 요청하세요.",
    description: "Composer — house callout body when the feature is 503",
  },
  "agentSessions.dependencyUnavailableTitle": {
    message: "에이전트 세션을 일시적으로 사용할 수 없습니다",
    description:
      "Composer — retryable dependency-outage title, distinct from configuration",
  },
  "agentSessions.dependencyUnavailableBody": {
    message:
      "플랫폼 의존 서비스에 현재 문제가 있습니다. 설정은 그대로 유지되니 잠시 후 다시 시도하세요.",
    description:
      "Composer — retryable dependency-outage body that does not send the user to an operator",
  },
  "agentSessions.modelKeyMissingTitle": {
    message: "먼저 모델 제공업체 키를 추가하세요",
    description:
      "Composer pre-flight title when the workspace has no model key",
  },
  "agentSessions.modelKeyMissingBody": {
    message:
      "이 워크스페이스에는 아직 모델 접근 권한이 없습니다. 워크스페이스 관리자가 설정 → 에이전트에서 워크스페이스 자체 모델 제공업체 키를 추가한 후 이곳으로 돌아오세요.",
    description:
      "Composer pre-flight body when the workspace has neither its own model key nor the bex-provided model (ADR091 D7)",
  },
  "agentSessions.modelKeyMissingAction": {
    message: "설정 → 에이전트 열기",
    description:
      "Composer pre-flight link to the Settings → Agents model key field (/workspace/settings#agents)",
  },
  "agentSessions.createErrorTitle": {
    message: "세션을 시작하지 못했습니다",
    description: "Composer — error alert title when the create fails",
  },
  "agentSessions.detailTitle": {
    message: "세션",
    description: "Agent-session detail page document title",
  },
  "agentSessions.untitled": {
    message: "제목 없는 세션",
    description:
      "Last-resort session name when a session carries neither a repository nor a task prompt",
  },
  "agentSessions.backToList": {
    message: "모든 세션",
    description: "Detail page — back link to the /agents list",
  },
  "agentSessions.detailErrorTitle": {
    message: "이 세션을 불러오지 못했습니다",
    description: "Detail page — heading when the session read fails",
  },
  "agentSessions.conversationTitle": {
    message: "대화",
    description: "Detail page — the live conversation column card title",
  },
  "agentSessions.failureTitle": {
    message: "세션 실패",
    description: "Detail page — heading over a failed session's reason",
  },
  "agentSessions.deliveryNoteTitle": {
    message: "변경 사항이 완전히 전달되지 않았습니다",
    description:
      "Detail page — heading over a completed session's note that the agent left work uncommitted, unpushed or without a pull request",
  },
  "agentSessions.deliveryNoteHint": {
    message:
      "후속 메시지를 보내 에이전트에게 커밋, 푸시, 풀 리퀘스트 업데이트를 마무리하도록 요청하세요.",
    description:
      "Detail page — the next step shown under an undelivered-work note",
  },
  "agentSessions.failureReasonFallback": {
    message: "세션을 시작하지 못했습니다.",
    description:
      "Detail page — generic failure text when the session carries no specific reason",
  },
  "agentSessions.failureRetry": {
    message: "다시 시도",
    description:
      "Detail page — button to re-run a failed session's original task",
  },
  "agentSessions.failureRetrying": {
    message: "다시 시도하는 중…",
    description:
      "Detail page — retry button label while the redispatch is in flight",
  },
  "agentSessions.capacityFailureTitle": {
    message: "샌드박스 용량 부족",
    description:
      "Detail page — heading when a session failed on the plan's sandbox limit",
  },
  "agentSessions.capacityFailureBody": {
    message:
      "이 워크스페이스가 요금제의 동시 샌드박스 한도에 도달했습니다. 요금제를 업그레이드하여 더 많이 동시에 실행하거나, 유휴 세션을 중지한 후 다시 시도하세요.",
    description:
      "Detail page — explains a plan-limit sandbox failure and the two remedies",
  },
  "agentSessions.upgradePlan": {
    message: "요금제 업그레이드",
    description:
      "Detail page — CTA on a capacity failure; opens Render's plan page on Pro (w3/m708)",
  },
  "agentSessions.failureCause.pool_full": {
    message: "여유 있는 샌드박스 머신이 없었습니다",
    description:
      "Failed-start cause line (session callout, automation runs) — no sandbox machine had room for the session's machine for the whole wait (w10/m497)",
  },
  "agentSessions.failureCause.image_pull": {
    message: "샌드박스 이미지를 가져올 수 없었습니다",
    description:
      "Failed-start cause line — the sandbox's container image did not pull (w10/m497)",
  },
  "agentSessions.failureCause.runtime": {
    message: "샌드박스 런타임이 시작하지 못했습니다",
    description:
      "Failed-start cause line — the sandbox runtime did not start the session's machine (w10/m497)",
  },
  "agentSessions.failureCause.quota": {
    message: "워크스페이스의 샌드박스 할당량이 거부했습니다",
    description:
      "Failed-start cause line — the workspace's sandbox quota refused the session's machine (w10/m497)",
  },
  "agentSessions.failureCause.unknown": {
    message: "플랫폼 문제",
    description:
      "Failed-start cause line — the sandbox did not start for a cause on bex's side that bex-api could not name (w10/m497)",
  },
  "agentSessions.provisioning": {
    message: "샌드박스를 시작하는 중…",
    description:
      "Detail page — shown while a new/steered session provisions its sandbox before the conversation stream is available",
  },
  "agentSessions.metaDuration": {
    message: "소요 시간 {duration}",
    description: "Detail header — elapsed session wall-clock",
  },
  "agentSessions.metaTurns_other": {
    message: "{count}턴",
    description: "Detail header — number of prompt turns taken (plural)",
  },
  "agentSessions.cancel": {
    message: "취소",
    description: "Detail header — cancel-session button label",
  },
  "agentSessions.cancelDisabledCanceling": {
    message: "이미 취소가 진행 중입니다.",
    description: "Detail header — tooltip on the disabled cancel button",
  },
  "agentSessions.canceling": {
    message: "취소하는 중…",
    description: "Detail header — confirm button label while the cancel runs",
  },
  "agentSessions.cancelSuccess": {
    message: "세션을 취소하는 중…",
    description: "Detail header — toast after the cancel is accepted",
  },
  "agentSessions.cancelConfirmTitle": {
    message: "이 세션을 취소하시겠습니까?",
    description: "Cancel confirm dialog title",
  },
  "agentSessions.cancelConfirmBody": {
    message:
      "에이전트가 중지됩니다. 브랜치에 이미 푸시된 커밋과 PR은 유지됩니다.",
    description: "Cancel confirm dialog body — states pushed work is preserved",
  },
  "agentSessions.cancelConfirmDismiss": {
    message: "계속 실행",
    description: "Cancel confirm dialog — dismiss button",
  },
  "agentSessions.cancelConfirmProceed": {
    message: "세션 취소",
    description: "Cancel confirm dialog — proceed button",
  },
  "agentSessions.stopTurn": {
    message: "턴 중지",
    description:
      "Detail header — Stop turn button label: ends only the running turn, unlike Cancel which ends the session",
  },
  "agentSessions.stoppingTurn": {
    message: "중지하는 중…",
    description: "Detail header — Stop turn button label while the stop runs",
  },
  "agentSessions.stopTurnHint": {
    message:
      "실행 중인 턴만 종료합니다. 세션은 계속 열려 있으며 대기 중인 메시지는 순서를 유지합니다. 단축키: Ctrl/Cmd+Shift+Backspace.",
    description: "Detail header — tooltip on the Stop turn button",
  },
  "agentSessions.stopTurnStarting": {
    message:
      "턴이 아직 시작되는 중입니다. 실행되기 시작하면 중지할 수 있습니다.",
    description:
      "Detail header — tooltip on the disabled Stop turn button while the sandbox starts",
  },
  "agentSessions.stopTurnSuccess": {
    message: "턴을 중지하는 중입니다. 몇 초 안에 종료됩니다.",
    description: "Detail header — toast after Stop turn is accepted",
  },
  "agentSessions.steerTitle": {
    message: "이 세션 조종하기",
    description: "Steering composer card title",
  },
  "agentSessions.steerErrorTitle": {
    message: "보내지 못했습니다",
    description: "Steering composer — inline error alert title",
  },
  "agentSessions.steerPlaceholderIdle": {
    message: "후속 지시를 보내 같은 브랜치에서 새 턴을 시작하세요.",
    description: "Steering composer — textarea placeholder for an idle session",
  },
  "agentSessions.steerPlaceholderQueue": {
    message: "메시지를 대기열에 추가하세요. 현재 턴이 끝나면 실행됩니다.",
    description:
      "Steering composer — textarea placeholder while a turn runs or messages wait (ADR047 D12)",
  },
  "agentSessions.steerHintIdle": {
    message:
      "이 세션은 유휴 상태입니다. 메시지를 보내면 같은 브랜치의 새 샌드박스에서 대화가 이어집니다.",
    description: "Steering composer — helper text for the redispatch route",
  },
  "agentSessions.steerHintQueue": {
    message:
      "에이전트가 작업 중입니다. 메시지는 대기열에 추가되며, 이 턴이 끝나면 별도의 턴으로 실행됩니다.",
    description:
      "Steering composer — helper text while a turn runs or messages wait (ADR047 D12)",
  },
  "agentSessions.steerDisabledCanceling": {
    message: "이 세션을 취소하는 중입니다. 이미 푸시된 작업은 유지됩니다.",
    description: "Steering composer — disabled reason while canceling",
  },
  "agentSessions.steerDisabledCanceled": {
    message: "이 세션은 취소되었습니다.",
    description: "Steering composer — disabled reason once canceled",
  },
  "agentSessions.steerDisabledStream": {
    message: "대화 스트림을 사용할 수 없어 실시간 조종이 일시 중지되었습니다.",
    description:
      "Steering composer — disabled reason when the m43 stream is down",
  },
  "agentSessions.steerSubmit": {
    message: "보내기",
    description: "Steering composer — submit button label",
  },
  "agentSessions.steerSending": {
    message: "보내는 중…",
    description: "Steering composer — submit button label while sending",
  },
  "agentSessions.steerSuccess": {
    message: "전송했습니다. 새 턴을 다시 전달하는 중입니다.",
    description: "Steering composer — toast after a redispatch is accepted",
  },
  "agentSessions.steerQueued": {
    message: "대기열에 추가했습니다. 현재 턴이 끝나면 실행됩니다.",
    description:
      "Steering composer — toast after a message was queued behind a running turn (ADR047 D12)",
  },
  "agentSessions.queueRegion": {
    message: "대기 중인 메시지",
    description: "Queued messages list — accessible name of the region",
  },
  "agentSessions.queueTitle_other": {
    message: "대기 중인 메시지 {count}개",
    description: "Queued messages list — heading with the count (plural)",
  },
  "agentSessions.queueHint": {
    message: "현재 턴이 끝난 후 이 순서대로 하나씩 실행됩니다.",
    description:
      "Queued messages list — one-line explanation next to the heading",
  },
  "agentSessions.queuePosition": {
    message: "대기열 {position}번째",
    description: "Queued messages list — tooltip on a row's place number",
  },
  "agentSessions.queueAttachments_other": {
    message: "첨부 파일 {count}개",
    description: "Queued message row — attachments that ride with it (plural)",
  },
  "agentSessions.queueBlocked": {
    message: "아직 시작하지 못했습니다: {reason}. 자동으로 다시 시도합니다.",
    description:
      "Queued message row — why the dispatcher could not start it (a spend ceiling or the live-sandbox limit)",
  },
  "agentSessions.queueModel": {
    message: "세션을 {model}(으)로 전환합니다",
    description:
      "Queued message — the model its turn switches the session to (w10/m516)",
  },
  "agentSessions.queueSendNow": {
    message: "지금 보내기",
    description:
      "Queued message row — stops the running turn and runs this message next",
  },
  "agentSessions.queueMoveUp": {
    message: "위로 이동",
    description: "Queued message row — move one place earlier",
  },
  "agentSessions.queueMoveDown": {
    message: "아래로 이동",
    description: "Queued message row — move one place later",
  },
  "agentSessions.queueEdit": {
    message: "편집",
    description: "Queued message row — edit the message in place",
  },
  "agentSessions.queueRemove": {
    message: "제거",
    description: "Queued message row — remove it so it never runs",
  },
  "agentSessions.queueEditLabel": {
    message: "대기 중인 메시지 편집",
    description: "Queued message row — accessible name of the edit box",
  },
  "agentSessions.queueEditSave": {
    message: "저장",
    description: "Queued message row — save the edit",
  },
  "agentSessions.queueEditSaveSendNow": {
    message: "저장 후 지금 보내기",
    description:
      "Queued message row — save the edit, stop the running turn and run this message next",
  },
  "agentSessions.queueEditDiscard": {
    message: "취소",
    description: "Queued message row — leave the edit without saving",
  },
  "agentSessions.queueEditHint": {
    message:
      "Enter는 저장, Shift+Enter는 줄바꿈, Ctrl/Cmd+Enter는 저장 후 지금 보내기, Esc는 취소입니다.",
    description: "Queued message row — keyboard hints while editing",
  },
  "agentSessions.queueSendNowStopping": {
    message: "현재 턴을 중지하는 중입니다. 이 메시지가 다음에 실행됩니다.",
    description: "Queued message row — toast after Send now stopped a turn",
  },
  "agentSessions.queueSendNowNext": {
    message: "이 메시지가 다음에 실행됩니다.",
    description:
      "Queued message row — toast after Send now when no turn was running",
  },
  "agentSessions.errors.AGENT_SESSION_INPUT_INVALID": {
    message:
      "입력이 올바르지 않습니다. 저장소, 브랜치, 작업을 확인한 후 다시 시도하세요.",
    description: "Mapped message for AGENT_SESSION_INPUT_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_ID_INVALID": {
    message: "세션 ID가 올바르지 않습니다.",
    description: "Mapped message for AGENT_SESSION_ID_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_FOUND": {
    message: "해당 세션이 더 이상 존재하지 않습니다.",
    description: "Mapped message for AGENT_SESSION_NOT_FOUND",
  },
  "agentSessions.errors.AGENT_SESSION_CONFLICT": {
    message: "이 세션이 다른 작업을 처리 중입니다. 잠시 후 다시 시도하세요.",
    description: "Mapped message for AGENT_SESSION_CONFLICT",
  },
  "agentSessions.errors.AGENT_SESSION_BRANCH_IN_USE": {
    message:
      "다른 세션이 이미 이 브랜치에서 작업 중입니다. 해당 세션에 메시지를 보내거나 다른 bex-agent/ 브랜치를 사용하세요.",
    description: "Mapped message for AGENT_SESSION_BRANCH_IN_USE (w5/m382)",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_STEERABLE": {
    message: "현재 단계({phase})에서는 이 세션을 조종할 수 없습니다.",
    description: "Mapped message for AGENT_SESSION_NOT_STEERABLE",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_RESUMABLE": {
    message: "현재 단계({phase})에서는 이 세션을 재개할 수 없습니다.",
    description: "Mapped message for AGENT_SESSION_NOT_RESUMABLE",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_ATTACHABLE": {
    message: "현재 단계({phase})에서는 이 세션에 연결할 수 없습니다.",
    description: "Mapped message for AGENT_SESSION_NOT_ATTACHABLE",
  },
  "agentSessions.errors.AGENT_SESSION_TURN_IN_FLIGHT": {
    message: "이미 턴이 실행 중입니다. 끝날 때까지 기다린 후 다시 보내세요.",
    description: "Mapped message for AGENT_SESSION_TURN_IN_FLIGHT",
  },
  "agentSessions.errors.AGENT_SESSION_QUEUE_FULL": {
    message:
      "이 세션에는 이미 메시지 {max}개가 대기 중입니다. 하나가 실행될 때까지 기다리거나 하나를 제거한 후 다시 보내세요.",
    description: "Mapped message for AGENT_SESSION_QUEUE_FULL (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_QUEUE_INLINE_SECRETS": {
    message:
      "인라인 시크릿이 포함된 메시지는 값이 저장되지 않으므로 대기열에서 기다릴 수 없습니다. 세션이 유휴 상태일 때 보내거나 워크스페이스 시크릿을 사용하세요.",
    description:
      "Mapped message for AGENT_SESSION_QUEUE_INLINE_SECRETS (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_QUEUED_MESSAGE_DISPATCHED": {
    message: "해당 메시지는 이미 실행이 시작되어 더 이상 변경할 수 없습니다.",
    description:
      "Mapped message for AGENT_QUEUED_MESSAGE_DISPATCHED (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_QUEUED_MESSAGE_NOT_FOUND": {
    message: "해당 메시지는 더 이상 대기열에 없습니다.",
    description:
      "Mapped message for AGENT_QUEUED_MESSAGE_NOT_FOUND (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_QUEUED_MESSAGE_ATTACHMENTS_OWNED": {
    message:
      "이 메시지에는 보낸 사람의 첨부 파일이 포함되어 있어 보낸 사람만 편집할 수 있습니다. 이동하거나 제거는 할 수 있습니다.",
    description:
      "Mapped message for AGENT_QUEUED_MESSAGE_ATTACHMENTS_OWNED (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_NO_TURN_RUNNING": {
    message: "실행 중인 턴이 없어 중지할 것이 없습니다.",
    description:
      "Mapped message for AGENT_SESSION_NO_TURN_RUNNING (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_TURN_STARTING": {
    message:
      "턴이 아직 시작되는 중이라 지금은 중지할 수 없습니다. 몇 초 후에 다시 시도하세요.",
    description: "Mapped message for AGENT_SESSION_TURN_STARTING (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_MODEL_ENDPOINT_INVALID": {
    message: "모델 엔드포인트는 올바른 HTTPS URL이어야 합니다.",
    description: "Mapped message for AGENT_SESSION_MODEL_ENDPOINT_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_MODEL_NOT_OFFERED": {
    message:
      "이 워크스페이스의 플랫폼 모델은 {model}을(를) 제공하지 않습니다. 모델 목록에서 선택하세요.",
    description:
      "Mapped message for AGENT_SESSION_MODEL_NOT_OFFERED (w10/m516)",
  },
  "agentSessions.errors.AGENT_SESSION_MODEL_PINNED": {
    message: "이 에이전트는 {pinnedModel}만 사용합니다.",
    description: "Mapped message for AGENT_SESSION_MODEL_PINNED",
  },
  "agentSessions.errors.AGENT_SESSION_EGRESS_ALLOWLIST_INVALID": {
    message: '이그레스 허용 목록 항목 "{entry}"이 올바르지 않습니다: {reason}',
    description: "Mapped message for AGENT_SESSION_EGRESS_ALLOWLIST_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_EGRESS_ALLOWLIST_IMMUTABLE": {
    message: "세션이 시작된 후에는 이그레스 허용 목록을 변경할 수 없습니다.",
    description: "Mapped message for AGENT_SESSION_EGRESS_ALLOWLIST_IMMUTABLE",
  },
  "agentSessions.errors.AGENT_SESSION_EGRESS_PHASE_INVALID": {
    message:
      "이 세션의 현재 단계에서는 이그레스 허용 목록을 설정할 수 없습니다.",
    description: "Mapped message for AGENT_SESSION_EGRESS_PHASE_INVALID",
  },
  "agentSessions.conversationLoading": {
    message: "대화를 불러오는 중…",
    description: "Placeholder while the client-only conversation column loads",
  },
  "agentSessions.conversationConnecting": {
    message: "세션 스트림에 연결하는 중…",
    description: "Shown while the conversation stream replay is in flight",
  },
  "agentSessions.conversationNotStarted": {
    message: "샌드박스가 준비되면 대화가 표시됩니다.",
    description:
      "Healthy pre-stream state while a newly created session is provisioning",
  },
  "agentSessions.conversationEmpty": {
    message: "아직 대화가 없습니다.",
    description: "Shown when a session has produced no transcript parts",
  },
  "agentSessions.turnInterrupted": {
    message: "{turn}번째 턴이 끝나기 전에 중단되었습니다.",
    description:
      "Transcript line at the end of a turn a person stopped with Stop turn (ADR047 D12); not a failure",
  },
  "agentSessions.turnRetentionSaving": {
    message: "다음 메시지에서 이어갈 수 있도록 변경 사항을 저장하는 중입니다.",
    description:
      "Transcript line under a stopped turn while its unsaved work is being saved so the next message can continue from it (ADR047 D12, w22/m413)",
  },
  "agentSessions.turnRetentionKept": {
    message: "변경 사항은 {time}까지 보관됩니다.",
    description:
      "Transcript line under a stopped turn: its unpushed changes are kept until {time}, a date and time in the viewer's own zone (ADR047 D12, w22/m413)",
  },
  "agentSessions.turnRetentionKeptNoTime": {
    message: "변경 사항은 다음 메시지를 위해 보관됩니다.",
    description:
      "Same as the kept line before the browser knows the viewer's time zone, so it names no time",
  },
  "agentSessions.turnRetentionContinued": {
    message: "다음 메시지가 이 변경 사항에서 이어졌습니다.",
    description:
      "Transcript line under a stopped turn: the next message started from its kept changes",
  },
  "agentSessions.turnRetentionLostExpired": {
    message:
      "보관된 샌드박스가 만료되어 푸시하지 않은 변경 사항이 사라졌습니다.",
    description:
      "Transcript line under a stopped turn: its kept sandbox expired and the unpushed changes in it are gone",
  },
  "agentSessions.turnRetentionLostStorageFull": {
    message:
      "워크스페이스의 샌드박스 저장 공간이 가득 차서 푸시하지 않은 변경 사항이 보관되지 않았습니다.",
    description:
      "Transcript line under a stopped turn: nothing was kept because the workspace's sandbox storage cap was full",
  },
  "agentSessions.turnRetentionLostSaveFailed": {
    message: "푸시하지 않은 변경 사항을 저장하지 못해 사라졌습니다.",
    description:
      "Transcript line under a stopped turn: saving its sandbox failed, so the unpushed changes are gone",
  },
  "agentSessions.turnRetentionLostEditorOpen": {
    message:
      "샌드박스에서 편집기가 열려 있어 푸시하지 않은 변경 사항이 보관되지 않았습니다.",
    description:
      "Transcript line under a stopped turn: nothing was kept because an editor (SSH) was open on the sandbox, which Stop does not end",
  },
  "agentSessions.turnRetentionLostNotRestored": {
    message:
      "푸시하지 않은 변경 사항을 복원하지 못해 다음 메시지는 새 체크아웃에서 시작되었습니다.",
    description:
      "Transcript line under a stopped turn: the next message could not start from the kept changes and started from a fresh checkout",
  },
  "agentSessions.turnRetentionLost": {
    message: "푸시하지 않은 변경 사항이 사라졌습니다.",
    description:
      "Transcript line under a stopped turn: its unpushed changes were lost, for a reason that is not worth naming (the session was canceled or archived)",
  },
  "agentSessions.turnRetentionLostSandboxLost": {
    message:
      "턴이 실패하고 샌드박스가 사라져 푸시하지 않은 변경 사항이 사라졌습니다.",
    description:
      "Transcript line under a failed turn: its sandbox was gone before its disk could be kept, so the unpushed work in it is lost (w10/m491)",
  },
  "agentSessions.turnStakeCommits_other": {
    message: "푸시하지 않은 커밋 {count}개",
    description:
      "Under a turn whose disk was kept or lost: how many commits its checkout held that were not pushed (w10/m491)",
  },
  "agentSessions.turnStakeChanges": {
    message: "커밋하지 않은 변경 사항",
    description:
      "Under a turn whose disk was kept or lost: its checkout held uncommitted changes (w10/m491)",
  },
  "agentSessions.turnError": {
    message: "{turn}번째 턴 실패",
    description:
      "Recorded failure of an earlier turn; subsequent turns remain readable",
  },
  "agentSessions.continuitySessionLoad": {
    message: "저장된 대화 상태로 에이전트가 재개되었습니다.",
    description:
      "System line when a fresh agent generation reloaded its own saved session (ADR047 D3 ladder rung 1)",
  },
  "agentSessions.continuityReprime": {
    message:
      "에이전트가 다시 시작되었습니다. 세션 기록에서 컨텍스트를 다시 구성했습니다.",
    description:
      "System line when a fresh agent generation was primed from the durable transcript (ladder rung 2)",
  },
  "agentSessions.continuityTaskRedelivery": {
    message: "에이전트가 다시 시작되었습니다. 원래 작업을 다시 전달했습니다.",
    description:
      "System line when no prior turn ever reached the agent, so the task was re-sent (ladder rung 3)",
  },
  "agentSessions.conversationEndedEmpty": {
    message: "대화가 기록되기 전에 이 세션이 종료되었습니다.",
    description: "Ended session with no durable transcript parts",
  },
  "agentSessions.conversationEnded": {
    message: "세션이 종료되었습니다.",
    description: "Footer note under a terminal session's replayed transcript",
  },
  "agentSessions.conversationIncomplete": {
    message: "일부 어시스턴트 출력을 보존하지 못했습니다",
    description:
      "Warning below a durable user turn whose assistant transcript is incomplete",
  },
  "agentSessions.conversationUnavailable": {
    message: "현재 대화 스트림을 사용할 수 없습니다.",
    description:
      "Degraded-state message when the m43 stream endpoint errors or is unconfigured",
  },
  "agentSessions.conversationUnavailableTerminal": {
    message: "이 세션의 대화 기록을 사용할 수 없습니다.",
    description:
      "Terminal session — transcript unavailable rather than live stream degraded",
  },
  "agentSessions.showEarlierMessages_other": {
    message: "이전 메시지 {count}개 표시",
    description:
      "Button revealing older transcript messages hidden by the render window (plural)",
  },
  "agentSessions.groupThought": {
    message: "생각",
    description: "Collapsible header for the agent's reasoning group",
  },
  "agentSessions.groupPlan": {
    message: "계획",
    description: "Collapsible header for the agent's plan/task checklist group",
  },
  "agentSessions.groupTerminal": {
    message: "터미널",
    description: "Collapsible header for a terminal-output group",
  },
  "agentSessions.groupCommand": {
    message: "명령",
    description: "Collapsible header for a command/tool-call group",
  },
  "agentSessions.groupDiff": {
    message: "Diff",
    description: "Label for a file-diff group when no path is given",
  },
  "agentSessions.terminalNoOutput": {
    message: "(출력 없음)",
    description: "Placeholder when a terminal group has no captured output",
  },
  "agentSessions.toolInput": {
    message: "입력",
    description: "Section label for a tool call's input",
  },
  "agentSessions.toolOutput": {
    message: "출력",
    description: "Section label for a tool call's output",
  },
  "agentSessions.toolError": {
    message: "오류",
    description: "Section label for a tool call's error text",
  },
  "agentSessions.toolStateRunning": {
    message: "실행 중",
    description: "Badge label for a tool call awaiting output",
  },
  "agentSessions.toolStateDone": {
    message: "완료",
    description: "Badge label for a completed tool call",
  },
  "agentSessions.toolStateDenied": {
    message: "거부됨",
    description: "Badge label for a tool call whose execution was denied",
  },
  "agentSessions.toolStateInterrupted": {
    message: "중단됨",
    description:
      "Badge label for a tool call the session ended before it finished",
  },
  "agentSessions.toolStateError": {
    message: "실패",
    description: "Badge label for a failed tool call",
  },
  "agentSessions.activityEdited_other": {
    message: "파일 {count}개 편집",
    description: "Activity-group summary when the turn edited files (diffs)",
  },
  "agentSessions.activityRan_other": {
    message: "명령 {count}개 실행",
    description: "Activity-group summary when the turn ran commands/terminals",
  },
  "agentSessions.activitySteps_other": {
    message: "{count}단계",
    description: "Activity-group summary fallback — the step count (plural)",
  },
  "agentSessions.activityRead_other": {
    message: "파일 {count}개 읽음",
    description: "Activity-group summary segment — file reads",
  },
  "agentSessions.activitySearched_other": {
    message: "{count}회 검색",
    description: "Activity-group summary segment — searches",
  },
  "agentSessions.activitySubagents_other": {
    message: "서브에이전트 {count}개 실행",
    description: "Activity-group summary segment — delegated subagent tasks",
  },
  "agentSessions.activityTools_other": {
    message: "도구 {count}개 사용",
    description: "Activity-group summary segment — tools of no known kind",
  },
  "agentSessions.activityThought_other": {
    message: "{count}회 생각",
    description:
      "Activity-group summary segment — the agent's thoughts between its calls",
  },
  "agentSessions.activityThinking": {
    message: "생각하는 중…",
    description: "Live status line while the agent is thinking between calls",
  },
  "agentSessions.activityFailed_other": {
    message: "{count}개 실패",
    description: "Activity-group summary segment — failed tool calls",
  },
  "agentSessions.outputClipped_other": {
    message: "{formatted}자 잘림",
    description: "Badge on a tool output the server stored head+tail-capped",
  },
  "agentSessions.showFullOutput": {
    message: "전체 출력 표시",
    description:
      "Button on a clipped tool output that loads the complete text the server kept",
  },
  "agentSessions.hideFullOutput": {
    message: "전체 출력 숨기기",
    description: "Button that collapses a loaded full tool output",
  },
  "agentSessions.fullOutputNotRetained": {
    message: "전체 출력이 보존되지 않았습니다",
    description:
      "Note on a clipped tool output whose complete text the server did not keep (clipped before full copies were stored)",
  },
  "agentSessions.fullOutputLoading": {
    message: "전체 출력을 불러오는 중…",
    description: "Status while a tool call's full output loads",
  },
  "agentSessions.fullOutputError": {
    message: "전체 출력을 불러오지 못했습니다.",
    description: "Inline error when loading a tool call's full output failed",
  },
  "agentSessions.fullOutputCopy": {
    message: "전체 출력 복사",
    description:
      "Accessible label of the copy button on a loaded full tool output",
  },
  "agentSessions.fullOutputCopied": {
    message: "전체 출력을 복사했습니다",
    description: "Toast after copying a full tool output",
  },
  "agentSessions.fullOutputCopyError": {
    message: "전체 출력을 복사하지 못했습니다",
    description: "Toast when copying a full tool output failed",
  },
  "agentSessions.conversationPartial": {
    message: "대화를 일부만 불러왔습니다.",
    description:
      "Inline row when the transcript stream failed after some of it rendered",
  },
  "agentSessions.liveStatus": {
    message: "에이전트 상태",
    description:
      "Accessible label for the single live status line under a streaming transcript",
  },
  "agentSessions.scrollToBottom": {
    message: "최신 항목으로 스크롤",
    description:
      "Accessible label for the floating jump-to-bottom button in the conversation column",
  },
  "agentSessions.recentSessions": {
    message: "최근",
    description: "Sessions sidebar — heading over the recent sessions list",
  },
  "agentSessions.sidebarEmpty": {
    message: "아직 세션이 없습니다",
    description: "Sessions sidebar — empty state when the workspace has none",
  },
  "agentSessions.sidebarLabel": {
    message: "에이전트 세션",
    description: "Sessions sidebar — accessible landmark label",
  },
  "agentSessions.sidebarSearch": {
    message: "세션 검색",
    description: "Sessions sidebar — search toggle + input accessible label",
  },
  "agentSessions.sidebarSearchPlaceholder": {
    message: "세션 검색…",
    description: "Sessions sidebar — search input placeholder",
  },
  "agentSessions.sidebarNoMatches": {
    message: "일치하는 세션이 없습니다",
    description:
      "Sessions sidebar — empty state when the search matches nothing",
  },
  "agentSessions.statusPhrase.prReady": {
    message: "PR 준비됨",
    description: "Sidebar status phrase — completed session with a draft PR",
  },
  "agentSessions.statusPhrase.working": {
    message: "작업 중…",
    description: "Sidebar status phrase — session still converging",
  },
  "agentSessions.statusPhrase.waitingForSandbox": {
    message: "샌드박스 머신을 기다리는 중",
    description:
      "Session status (header, list rows, sidebar, conversation, automation runs) — the turn waits because no sandbox machine has room for it yet (w10/m497); the time waited follows where there is room",
  },
  "agentSessions.menuMore": {
    message: "더 많은 작업",
    description: "Header — accessible label for the '…' overflow menu",
  },
  "agentSessions.openPr": {
    message: "풀 리퀘스트 열기",
    description: "Header overflow menu — open the draft PR in a new tab",
  },
  "agentSessions.connect": {
    message: "연결",
    description: "Header — trigger for the Open-in-Zed / SSH connect menu",
  },
  "agentSessions.openInZed": {
    message: "Zed에서 열기",
    description:
      "Connect menu — hotlink that opens the sandbox as a Zed remote project",
  },
  "agentSessions.openInZedHint": {
    message:
      "SSH로 샌드박스의 /workspace를 엽니다. Zed 편집기가 필요합니다. 닫힌 샌드박스는 연결하면 시작됩니다.",
    description:
      "Connect menu — helper text under the Open-in-Zed action; a closed sandbox starts when the editor or ssh connects (ADR054 D9)",
  },
  "agentSessions.connectSSH": {
    message: "SSH",
    description: "Connect menu — label above the copyable ssh command",
  },
  "agentSessions.sshCopy": {
    message: "SSH 명령 복사",
    description: "Connect menu — accessible label for the copy button",
  },
  "agentSessions.sshCopied": {
    message: "SSH 명령을 복사했습니다",
    description: "Connect menu — toast after copying the ssh command",
  },
  "agentSessions.sshCopyError": {
    message: "복사하지 못했습니다",
    description: "Connect menu — toast when copying the ssh command fails",
  },
  "agentSessions.groupThoughtFor": {
    message: "{duration} 동안 생각함",
    description:
      "Reasoning-group summary with elapsed duration from persisted part timestamps (or ~arrival timing)",
  },
  "agentSessions.terminalStatus.completed": {
    message: "에이전트 작업 완료",
    description: "Terminal transcript status line — completed session",
  },
  "agentSessions.terminalStatus.failed": {
    message: "세션이 오류로 종료되었습니다",
    description: "Terminal transcript status line — failed session",
  },
  "agentSessions.terminalStatus.canceled": {
    message: "세션이 취소되었습니다",
    description: "Terminal transcript status line — canceled session",
  },
  "agentSessions.tagsLabel": {
    message: "태그",
    description: "Accessible label for a session row's tag list (w21/002)",
  },
  "agentSessions.filterByTag": {
    message: "{tag} 태그가 붙은 세션 표시",
    description:
      "Accessible label of a tag chip that filters the session list to that tag",
  },
  "agentSessions.filterTagged": {
    message: "{tag} 태그",
    description: "Session list heading while narrowed to one tag",
  },
  "agentSessions.archive": {
    message: "보관",
    description: "Action putting a session out of the working set (ADR065)",
  },
  "agentSessions.unarchive": {
    message: "보관 해제",
    description: "Action returning an archived session to the working set",
  },
  "agentSessions.archiveSuccess": {
    message: "세션을 보관했습니다",
    description: "Toast after archiving a session",
  },
  "agentSessions.undoArchive": {
    message: "실행 취소",
    description: "Toast action that immediately unarchives a session",
  },
  "agentSessions.unarchiveSuccess": {
    message: "세션 보관을 해제했습니다",
    description: "Toast after unarchiving a session",
  },
  "agentSessions.archivedBadge": {
    message: "보관됨",
    description: "Badge marking a session as archived (out of the working set)",
  },
  "agentSessions.startedFromSnapshot": {
    message: "스냅샷에서 시작됨",
    description:
      "Session header chip: the latest turn booted from a repository environment build (ADR111 D4)",
  },
  "agentSessions.startedFromSnapshotTitle": {
    message:
      "최근 턴은 설정 단계가 이미 실행된 환경 빌드 {id}에서 시작되었습니다.",
    description:
      "Tooltip of the Started from snapshot chip (id is the build id, arb-…)",
  },
  "agentSessions.delete": {
    message: "삭제",
    description: "Destructive action permanently deleting a finished session",
  },
  "agentSessions.deleting": {
    message: "삭제하는 중…",
    description: "Delete confirm button while the delete is in flight",
  },
  "agentSessions.deleteSuccess": {
    message: "세션을 삭제했습니다",
    description: "Toast after permanently deleting a session",
  },
  "agentSessions.deleteConfirmTitle": {
    message: "이 세션을 삭제하시겠습니까?",
    description: "Delete confirmation dialog title",
  },
  "agentSessions.deleteConfirmBody": {
    message:
      "세션 기록과 대화 내용이 영구적으로 삭제됩니다. GitHub에 푸시된 브랜치와 풀 리퀘스트는 영향을 받지 않습니다. 이 작업은 되돌릴 수 없습니다.",
    description: "Delete confirmation dialog body",
  },
  "agentSessions.deleteConfirmDismiss": {
    message: "세션 유지",
    description: "Delete confirmation dialog dismiss button",
  },
  "agentSessions.deleteConfirmProceed": {
    message: "세션 삭제",
    description: "Delete confirmation dialog destructive proceed button",
  },
  "agentSessions.colActions": {
    message: "작업",
    description: "Screen-reader label of the list's trailing actions column",
  },
  "agentSessions.filterActive": {
    message: "최근",
    description: "List membership tab — the unarchived working set",
  },
  "agentSessions.filterArchived": {
    message: "보관됨",
    description: "List membership tab — archived sessions only",
  },
  "agentSessions.filterAll": {
    message: "전체",
    description: "List membership tab — archived and unarchived together",
  },
  "agentSessions.loadMore": {
    message: "더 보기",
    description: "Button fetching the next page of sessions",
  },
  "agentSessions.loadingMore": {
    message: "로딩 중…",
    description: "Load-more button while the next page is in flight",
  },
  "agentSessions.attachFiles": {
    message: "파일 첨부",
    description:
      "Paperclip button that opens the file picker in the agent composers",
  },
  "agentSessions.attachmentRemove": {
    message: "{name} 제거",
    description: "Accessible label of the remove button on an attachment chip",
  },
  "agentSessions.attachmentTooLarge": {
    message: "{max}보다 큼",
    description: "Attachment chip error when a file exceeds the per-file limit",
  },
  "agentSessions.attachmentTooMany": {
    message: "메시지당 최대 {max}개 파일",
    description: "Attachment chip error when a message has too many files",
  },
  "agentSessions.attachmentDropHint": {
    message: "파일을 놓아 첨부",
    description: "Overlay shown while dragging files over a composer",
  },
  "agentSessions.sharedFileLabel": {
    message: "파일 공유됨",
    description: "Label above a file the agent shared with share_file",
  },
  "agentSessions.filesButton": {
    message: "파일",
    description: "Session header button that opens the session files",
  },
  "agentSessions.filesTitle": {
    message: "세션 파일",
    description: "Title of the session files sheet",
  },
  "agentSessions.filesDescription": {
    message: "이 세션에서 첨부한 파일과 에이전트가 공유한 파일입니다.",
    description: "Description under the session files sheet title",
  },
  "agentSessions.filesEmpty": {
    message:
      "아직 파일이 없습니다. 메시지에 파일을 첨부하거나 에이전트에게 파일 공유를 요청하세요.",
    description: "Empty state of the session files sheet",
  },
  "agentSessions.filesError": {
    message: "세션 파일을 불러오지 못했습니다.",
    description: "Error state of the session files sheet",
  },
  "agentSessions.filesColumnName": {
    message: "이름",
    description: "Files table column: file name",
  },
  "agentSessions.filesColumnFrom": {
    message: "작성자",
    description: "Files table column: who added the file",
  },
  "agentSessions.filesColumnTurn": {
    message: "턴",
    description: "Files table column: turn number",
  },
  "agentSessions.filesColumnSize": {
    message: "크기",
    description: "Files table column: file size",
  },
  "agentSessions.filesFromInput": {
    message: "나",
    description: "Files table: a file the user attached",
  },
  "agentSessions.filesFromOutput": {
    message: "에이전트",
    description: "Files table: a file the agent shared",
  },
  "agentSessions.filesLoadMore": {
    message: "더 보기",
    description: "Files table: load the next page",
  },
  "agentSessions.attachmentsBlockSend": {
    message: "업로드에 실패한 파일을 제거해야 보낼 수 있습니다",
    description: "Hint when failed attachment chips block sending",
  },
  "agentSessions.attachmentOpen": {
    message: "{name} 열기",
    description:
      "Accessible label of an attachment thumbnail that opens the lightbox",
  },
  "agentSessions.attachmentDownload": {
    message: "{name} 다운로드",
    description: "Accessible label of an attachment download card",
  },
  "agentSessions.attachmentUnavailable": {
    message: "사용 불가",
    description:
      "Shown on an attachment whose download link could not be loaded",
  },
  "agentSessions.attachmentUploading": {
    message: "업로드 중, {percent}%",
    description: "Screen-reader status of an attachment chip while uploading",
  },
  "agentSessions.attachmentReady": {
    message: "업로드됨",
    description:
      "Screen-reader status of an attachment chip once its upload landed",
  },
  "agentSessions.attachmentHttpError": {
    message: "업로드 실패 (HTTP {status})",
    description:
      "Attachment chip error when the server answered without a message",
  },
  "agentSessions.attachmentNetworkError": {
    message: "업로드 실패: 네트워크 오류",
    description:
      "Attachment chip error when the upload could not reach the server",
  },
  "agentSessions.attachmentsUploading": {
    message: "파일 업로드가 끝날 때까지 기다리는 중…",
    description: "Composer hint while attachment uploads block sending",
  },
  "agentSessions.spendWarningTitle": {
    message: "에이전트 모델 지출이 이번 달 한도의 {percent}%에 도달했습니다",
    description: "Composer warning at 80% of the spend cap (ADR091 D6)",
  },
  "agentSessions.spendWarningBody": {
    message: "{cap} 중 {used} 사용. 한도에 도달하면 새 모델 요청이 중단됩니다.",
    description: "Composer warning body",
  },
  "agentSessions.spendWarningBodyCharge": {
    message:
      "결제 크레딧 초과 지출 {cap} 중 {used} 사용. 결제 크레딧이 먼저 사용되며, 그 초과 지출이 {cap}에 도달하면 등록된 카드로 청구되고 세션은 계속 실행됩니다. 청구에 실패한 경우에만 새 모델 요청이 중단됩니다.",
    description:
      "Composer warning body when the cap is a new workspace's charge point (ADR091 D10.2a); {used} is the spend billing credits leave",
  },
  "agentSessions.spendWarningBodyCeiling": {
    message:
      "결제 크레딧 초과 지출 {cap} 중 {used} 사용. 결제 크레딧이 먼저 사용되며, 그 초과 지출이 {cap}에 도달하면 새 모델 요청이 중단됩니다.",
    description:
      "Composer warning body when the trust tier's ceiling binds on a workspace bex cannot charge (payment hold, excluded or comped) and the spend it counts, what billing credits leave ({used}), differs from month-to-date",
  },
  "agentSessions.spendReachedTitle.cap": {
    message: "월 에이전트 모델 지출 한도에 도달했습니다",
    description: "Composer refusal title: the workspace's own cap",
  },
  "agentSessions.spendReachedTitle.newWorkspace": {
    message: "신규 워크스페이스 지출 한도에 도달했습니다",
    description: "Composer refusal title: the T0 trust-tier ceiling",
  },
  "agentSessions.spendReachedTitle.locked": {
    message: "에이전트 모델 지출이 보류되었습니다",
    description: "Composer refusal title: a payment hold",
  },
  "agentSessions.spendReachedBody.cap": {
    message:
      "사용량: {used} / 월 한도: {cap}. 새 세션을 시작할 수 없습니다. 워크스페이스 관리자가 한도를 올릴 수 있으며, 매월 1일(UTC)에 초기화됩니다.",
    description: "Composer refusal body: the workspace's own cap",
  },
  "agentSessions.spendReachedBody.capMax": {
    message:
      "사용량: {used} / 월 한도: {cap}(설정 가능한 최대값). 새 세션을 시작할 수 없습니다. 한도는 매월 1일(UTC)에 초기화되며, 더 높은 한도가 필요하면 지원팀에 문의하세요.",
    description:
      "Composer refusal body: the workspace's own cap binds at its maximum, so no admin can raise it",
  },
  "agentSessions.spendReachedBody.newWorkspace": {
    message:
      "신규 워크스페이스는 첫 결제가 성공할 때까지 bex 제공 모델에 {cap}까지 지출할 수 있습니다. 이후 한도가 올라가며, 매월 1일(UTC)에 초기화됩니다.",
    description: "Composer refusal body: the T0 trust-tier ceiling",
  },
  "agentSessions.spendReachedBody.locked": {
    message:
      "결제 실패 또는 분쟁으로 이 워크스페이스의 에이전트 모델 지출이 {cap} 이하로 제한됩니다. 결제에서 미결제 금액을 해결한 후 새 세션을 시작하세요.",
    description: "Composer refusal body: a payment hold",
  },
  "agentSessions.spendIncreaseCap": {
    message: "한도 늘리기",
    description: "Admin action linking to Settings → Agents",
  },
  "agentSessions.spendGoToBilling": {
    message: "결제로 이동",
    description: "Link to the Billing page",
  },
  "agentSessions.spendBalanceDueTitle": {
    message: "결제에 실패했습니다",
    description: "Composer refusal title: BILLING_BALANCE_DUE",
  },
  "agentSessions.spendBalanceDueBody": {
    message:
      "미결제 청구서를 결제할 때까지 bex 제공 모델을 사용하는 에이전트 세션이 일시 중지됩니다.",
    description: "Composer refusal body: BILLING_BALANCE_DUE",
  },
  "agentSessions.spendReachedHiddenTitle": {
    message: "에이전트 모델 지출 한도에 도달했습니다",
    description:
      "Composer refusal title at the spend cap, for a member whose role cannot view billing details (no figures, w9/144)",
  },
  "agentSessions.spendReachedHiddenBody": {
    message:
      "이 워크스페이스가 에이전트 모델 지출 한도에 도달하여 bex 제공 모델로 새 세션을 시작할 수 없습니다. 워크스페이스 관리자에게 문의하세요.",
    description:
      "Composer refusal body at the spend cap, for a member whose role cannot view billing details (no figures, w9/144)",
  },
  "agentSessions.errors.AGENT_SESSION_TRUST_LIMIT": {
    message:
      "신규 워크스페이스는 첫 결제가 성공할 때까지 bex 제공 모델로 에이전트 세션을 동시에 {limit}개까지 실행할 수 있습니다. 실행 중인 세션이 끝나기를 기다리거나 하나를 중지한 후 다시 시도하세요.",
    description: "Mapped message for AGENT_SESSION_TRUST_LIMIT (ADR091 D10.1)",
  },
  "agentSessions.errors.AGENT_SESSION_TRUST_LIMIT_locked": {
    message:
      "결제 실패 또는 분쟁 이후 이 워크스페이스는 bex 제공 모델로 에이전트 세션을 동시에 {limit}개까지 실행할 수 있습니다. 결제에서 미결제 금액을 해결하면 제한이 해제됩니다.",
    description:
      "Mapped message for AGENT_SESSION_TRUST_LIMIT while a payment hold applies",
  },
  "agentSessions.errors.AGENT_SESSION_SPEND_LIMIT_INVALID": {
    message: "지출 한도는 $0보다 크고 $100,000 이하여야 합니다.",
    description:
      "Mapped message for AGENT_SESSION_SPEND_LIMIT_INVALID (ADR094 D8)",
  },
  "agentSessions.errors.AGENT_SESSION_SPEND_LIMIT_UNAVAILABLE": {
    message:
      "지출 한도는 bex 제공 모델을 사용하는 세션에만 적용됩니다. 이 워크스페이스는 자체 모델 키를 사용하므로 한도를 지우거나 설정 → 에이전트에서 워크스페이스를 bex 모델로 전환하세요.",
    description:
      "Mapped message for AGENT_SESSION_SPEND_LIMIT_UNAVAILABLE (ADR094 D8)",
  },
  "agentSessions.spendLimitLabel": {
    message: "지출 한도 (USD)",
    description:
      "Composer — label/aria-label of the optional per-session spend ceiling input",
  },
  "agentSessions.spendLimitPlaceholder": {
    message: "지출 한도",
    description:
      "Composer — placeholder of the optional per-session spend ceiling input",
  },
  "agentSessions.spendLimitHint": {
    message:
      "선택 사항입니다. bex 모델 지출이 이 금액에 도달하면 세션이 중지됩니다($0 초과, 최대 $100,000).",
    description: "Composer — help text under the spend limit input",
  },
  "agentSessions.spendLimitInvalid": {
    message: "$0보다 크고 $100,000 이하인 금액을 입력하거나 비워 두세요.",
    description: "Composer — inline error for an out-of-range spend limit",
  },
  "agentSessions.failureSessionSpendLimitReached": {
    message:
      "모델 지출이 세션 한도 {limit}에 도달하여 이 세션이 중지되었습니다. 계속하려면 더 높은 한도로 새 세션을 시작하세요.",
    description:
      "Failure callout for AGENT_MODEL_SPEND_CAP_REACHED when the session's own maxSpendUSD ceiling was hit",
  },
  "agentSessions.spendNote": {
    message: "모델 지출 {spent}",
    description:
      "Session page — the session's model spend so far (outcome.cost)",
  },
  "agentSessions.spendNoteWithLimit": {
    message: "모델 지출 {spent} / 세션 한도 {limit}",
    description:
      "Session page — model spend so far beside the session's spend limit",
  },
  "agentSessions.costNote": {
    message: "비용 {total} · 모델 {model}, 컴퓨팅 {compute}",
    description:
      "Session page — the session's total cost so far (outcome.cost.totalUSD) with its model and sandbox compute parts",
  },
  "agentSessions.costNoteWithLimit": {
    message:
      "비용 {total} · 모델 {model} / 세션 한도 {limit}, 컴퓨팅 {compute}",
    description:
      "Session page — total cost with its parts; the session spend limit caps the model part",
  },
  "agentSessions.resultTitle": {
    message: "결과",
    description:
      "Session page — heading over the turn's final structured result",
  },
  "agentSessions.resultDraftTitle": {
    message: "결과 초안",
    description:
      "Session page — heading over the agent's not-yet-final structured result while the turn runs",
  },
  "agentSessions.resultDraftHint": {
    message: "아직 최종 결과가 아닙니다. 에이전트가 변경할 수 있습니다.",
    description: "Session page — note under a draft structured result",
  },
  "agentSessions.resultCopy": {
    message: "JSON 복사",
    description: "Session page — button copying the structured result",
  },
  "agentSessions.resultCopied": {
    message: "복사됨",
    description: "Session page — toast after copying the structured result",
  },
  "agentSessions.failureSpendCapReached": {
    message:
      "이 워크스페이스가 이번 달 에이전트 모델 지출 한도에 도달했습니다. 관리자가 설정 → 에이전트에서 한도를 올릴 수 있으며, 매월 1일(UTC)에 초기화됩니다.",
    description: "Failure callout for AGENT_MODEL_SPEND_CAP_REACHED",
  },
  "agentSessions.failureBalanceDue": {
    message:
      "이 워크스페이스의 결제에 실패했습니다. 결제에서 미결제 청구서를 결제한 후 새 세션을 시작하세요.",
    description: "Failure callout for BILLING_BALANCE_DUE",
  },
  "agentSessions.failureSetupTitle": {
    message: "저장소 설정 실패",
    description:
      "Failure callout heading when a repository setup step failed (AGENT_SESSION_SETUP_FAILED)",
  },
  "agentSessions.failureSetupHint": {
    message:
      "이 저장소 환경의 설정 단계가 실패하여 에이전트가 시작되지 않았고 모델 지출도 발생하지 않았습니다. 설정 → 에이전트 → 저장소 환경에서 해당 단계를 수정한 후 다시 시도하세요.",
    description:
      "Failure callout guidance under the recorded reason (which names the step and its exit code) for AGENT_SESSION_SETUP_FAILED",
  },
  "agentSessions.failureSetupAction": {
    message: "저장소 환경 편집",
    description:
      "Failure callout button opening Settings → Repository environments",
  },
  "agentSessions.costTotal": {
    message: "모델 비용 {amount}",
    description: "Session header: the session's model cost (ADR091 D6)",
  },
  "agentSessions.costTotalAll": {
    message: "총 비용 {amount}",
    description:
      "Session header: the session's total cost so far, model plus sandbox compute (outcome.cost.totalUSD, w9/m178)",
  },
  "agentSessions.costBreakdown": {
    message: "모델 {model} · 컴퓨팅 {compute}",
    description:
      "Session header cost popover: the model and sandbox compute parts of the total",
  },
  "agentSessions.costByo": {
    message: "제공업체에서 청구됨",
    description: "Session header: a BYO-key session has no bex model cost",
  },
  "agentSessions.costTitle": {
    message: "턴별 모델 비용",
    description: "Heading of the per-turn cost popover",
  },
  "agentSessions.costTurn": {
    message: "{turn}번째 턴",
    description: "One turn's row in the cost popover",
  },
  "agentSessions.costSetup": {
    message: "첫 번째 턴 이전",
    description: "Requests recorded before the first turn",
  },
  "agentSessions.costTurnTokens": {
    message: "입력 {input} · 출력 {output} · 캐시 {cache} 토큰",
    description: "A turn's token counts",
  },
  "agentSessions.costPricing": {
    message: "모델 제공업체 비용 + 30%로 청구됩니다.",
    description: "Pricing statement in the cost popover",
  },
  "agentSessions.secretsButton": {
    message: "시크릿",
    description: "Composer toolbar: the Secrets button label/tooltip (ADR098)",
  },
  "agentSessions.secretsButtonCount_other": {
    message: "시크릿 ({count}개 첨부됨)",
    description:
      "Composer Secrets button accessible label when secrets are attached",
  },
  "agentSessions.secretsTitle": {
    message: "이 세션의 시크릿",
    description: "Composer Secrets popover heading",
  },
  "agentSessions.secretsDescription": {
    message:
      "에이전트가 각 시크릿을 환경 변수로 읽습니다. 값은 다시 표시되지 않으며 대화 기록에서 가려집니다.",
    description: "Composer Secrets popover description",
  },
  "agentSessions.secretsWorkspace": {
    message: "워크스페이스 시크릿",
    description:
      "Composer Secrets popover: workspace agent secrets list heading",
  },
  "agentSessions.secretsWorkspaceLoadFailed": {
    message: "워크스페이스의 에이전트 시크릿을 불러오지 못했습니다.",
    description:
      "Composer secrets panel error when the workspace secret list fails to load",
  },
  "agentSessions.secretsWorkspaceEmpty": {
    message: "아직 워크스페이스 시크릿이 없습니다.",
    description: "Composer Secrets popover: no workspace agent secrets",
  },
  "agentSessions.secretsManage": {
    message: "설정에서 관리",
    description: "Composer Secrets popover: link to Settings → Agents",
  },
  "agentSessions.secretsInline": {
    message: "이 세션 전용",
    description: "Composer Secrets popover: inline secrets heading",
  },
  "agentSessions.secretsRemove": {
    message: "{name} 제거",
    description:
      "Composer Secrets popover: remove an inline secret (accessible label)",
  },
  "agentSessions.secretsNamePlaceholder": {
    message: "NAME",
    description: "Composer Secrets popover: inline secret name placeholder",
  },
  "agentSessions.secretsNameLabel": {
    message: "시크릿 이름",
    description: "Composer Secrets popover: inline secret name field label",
  },
  "agentSessions.secretsValuePlaceholder": {
    message: "값",
    description: "Composer Secrets popover: inline secret value placeholder",
  },
  "agentSessions.secretsValueLabel": {
    message: "시크릿 값",
    description: "Composer Secrets popover: inline secret value field label",
  },
  "agentSessions.secretsAdd": {
    message: "추가",
    description: "Composer Secrets popover: add an inline secret",
  },
  "agentSessions.secretsHint": {
    message: "세션과 함께 삭제됩니다.",
    description: "Composer Secrets popover: inline secret lifetime hint",
  },
  "agentSessions.secretsFull": {
    message: "세션에는 시크릿을 최대 20개까지 사용할 수 있습니다.",
    description: "Composer Secrets popover: session secret limit reached",
  },
  "agentSessions.secretsDuplicate": {
    message: "이 세션에 이미 같은 이름의 시크릿이 있습니다.",
    description: "Composer Secrets popover: duplicate inline secret name",
  },
  "agentSessions.secretsScopeRepo": {
    message: "이 저장소",
    description:
      "Composer Secrets popover: hint beside a secret scoped to the chosen repository, included",
  },
  "agentSessions.secretsScopeOtherRepo": {
    message: "{repo} 전용",
    description:
      "Composer Secrets popover: hint beside a secret scoped to another repository, not available here",
  },
  "agentSessions.appliedSecretsTitle": {
    message: "최근 턴의 시크릿",
    description:
      "Session header: heading of the applied-secrets popover (names and sources, never values)",
  },
  "agentSessions.appliedSecretSource.inline": {
    message: "이 세션",
    description:
      "Applied secret source: an inline secret given to this session only",
  },
  "agentSessions.appliedSecretSource.explicit": {
    message: "선택됨",
    description: "Applied secret source: a workspace secret the session picked",
  },
  "agentSessions.appliedSecretSource.repo": {
    message: "저장소",
    description:
      "Applied secret source: a workspace secret scoped to the session's repository",
  },
  "agentSessions.appliedSecretSource.workspace": {
    message: "워크스페이스",
    description:
      "Applied secret source: a workspace secret every session receives",
  },
  "agentSessions.metaSecrets_other": {
    message: "시크릿 {count}개",
    description: "Detail header — number of attached secrets (plural, ADR098)",
  },
  "agentSessions.skillsButton": {
    message: "스킬",
    description: "Composer toolbar: the Skills button label/tooltip (ADR099)",
  },
  "agentSessions.skillsButtonCount_other": {
    message: "스킬 ({count}개 선택됨)",
    description:
      "Composer Skills button accessible label when skills are chosen",
  },
  "agentSessions.skillsTitle": {
    message: "이 턴의 스킬",
    description: "Composer Skills popover heading",
  },
  "agentSessions.skillsDescription": {
    message:
      "에이전트는 선택한 각 스킬을 따릅니다. 활성화된 스킬은 에이전트가 스스로 적용할 수도 있습니다.",
    description: "Composer Skills popover description",
  },
  "agentSessions.skillsLoadFailed": {
    message: "워크스페이스 스킬을 불러오지 못했습니다.",
    description: "Composer Skills popover: list failed",
  },
  "agentSessions.skillsEmpty": {
    message: "이 워크스페이스에는 아직 스킬이 없습니다.",
    description: "Composer Skills popover: none exist",
  },
  "agentSessions.skillsManage": {
    message: "스킬 관리",
    description: "Composer Skills popover link to Settings → Agents",
  },
  "agentSessions.skillsArgumentsLabel": {
    message: "{name} 인수",
    description: "Composer Skills popover: argument field label",
  },
  "agentSessions.skillsArgumentsPlaceholder": {
    message: "인수 (선택 사항)",
    description: "Composer Skills popover: argument field placeholder",
  },
  "agentSessions.skillsArgumentsInvalid": {
    message: "인수는 최대 4 KiB의 한 줄이어야 합니다.",
    description: "Composer Skills popover: invalid arguments",
  },
  "agentSessions.skillsFull": {
    message: "한 턴에는 스킬을 최대 5개까지 사용할 수 있습니다.",
    description: "Composer Skills popover: limit reached",
  },
  "agentSessions.skillsHint": {
    message: "인수는 스킬의 $ARGUMENTS 자리에 채워집니다.",
    description: "Composer Skills popover hint",
  },
  "agentSessions.skillsRemove": {
    message: "{name} 제거",
    description: "Accessible label of a chosen skill chip's remove button",
  },
  "agentSessions.turnSkills": {
    message: "이 턴에서 사용한 스킬",
    description: "Accessible label of a transcript turn's skill pills",
  },
  "agentSessions.turnOrigins": {
    message: "이 턴을 조종한 원인",
    description:
      "Accessible label of the lines above a transcript turn that PR feedback started (w9/m344)",
  },
  "agentSessions.turnOriginReview": {
    message: "@{author}의 리뷰로 조종됨",
    description:
      "A turn started by a pull-request review; {author} is the reviewer's GitHub login. Links to the review",
  },
  "agentSessions.turnOriginReviewAnonymous": {
    message: "PR 리뷰로 조종됨",
    description:
      "A turn started by a pull-request review whose author is unknown. Links to the review",
  },
  "agentSessions.turnOriginComment": {
    message: "@{author}의 PR 댓글로 조종됨",
    description:
      "A turn started by a pull-request comment; {author} is the commenter's GitHub login. Links to the comment",
  },
  "agentSessions.turnOriginCommentAnonymous": {
    message: "PR 댓글로 조종됨",
    description:
      "A turn started by a pull-request comment whose author is unknown. Links to the comment",
  },
  "agentSessions.turnOriginCiFailure": {
    message: "CI 실행 실패로 조종됨",
    description:
      "A turn started by a failed CI run on the pull request. Links to the failed checks",
  },
  "agentSessions.turnOriginCiFailureOnBase": {
    message:
      "CI 실행 실패로 조종됨. 일부 검사는 이번 변경과 무관하게 {base}에서도 실패합니다",
    description:
      "A turn started by a failed CI run on the pull request where at least one failing check also fails on the base branch ({base}, a branch name), so the pull request did not cause it. Links to the failed checks",
  },
  "agentSessions.turnOriginsStart": {
    message: "이 세션을 시작한 원인",
    description:
      "Accessible label of the line above a session's first turn when a /bex comment on a pull request started it (w9/m348)",
  },
  "agentSessions.turnOriginStartComment": {
    message: "@{author}의 댓글로 시작됨",
    description:
      "The first turn of a session a /bex pull-request comment started; {author} is the commenter's GitHub login. Links to the comment",
  },
  "agentSessions.turnOriginStartCommentAnonymous": {
    message: "PR 댓글로 시작됨",
    description:
      "The first turn of a session a /bex pull-request comment started, when the commenter is unknown. Links to the comment",
  },
  "agentSessions.lineage.childSessions": {
    message: "하위 세션",
    description:
      "Heading of the block listing sessions the agent spawned, in the parent session's transcript",
  },
  "agentSessions.lineage.childCardTitle": {
    message: "세션을 생성함",
    description:
      "Small label above a child session's title on its card in the parent's transcript",
  },
  "agentSessions.lineage.agentsPanel": {
    message: "에이전트",
    description:
      "Heading of the session page's right-hand panel listing the sessions below this one",
  },
  "agentSessions.lineage.viewSession": {
    message: "세션 보기",
    description: "Link on an Agents panel row that opens that child session",
  },
  "agentSessions.lineage.parentSession": {
    message: "상위 세션",
    description:
      "Link in the Agents panel of a child session that opens its parent",
  },
  "agentSessions.lineage.collapsePanel": {
    message: "에이전트 패널 접기",
    description:
      "Accessible label of the button that collapses the Agents panel",
  },
  "agentSessions.lineage.expandPanel": {
    message: "에이전트 패널 펼치기",
    description: "Accessible label of the button that expands the Agents panel",
  },
  "agentSessions.lineage.collapseChildren": {
    message: "하위 세션 숨기기",
    description:
      "Accessible label of a sidebar row's button that collapses its nested child sessions",
  },
  "agentSessions.lineage.expandChildren": {
    message: "하위 세션 표시",
    description:
      "Accessible label of a sidebar row's button that expands its nested child sessions",
  },
  "agentSessions.lineage.status.working": {
    message: "작업 중",
    description:
      "Status group and badge of a child session that is still running",
  },
  "agentSessions.lineage.status.done": {
    message: "완료",
    description:
      "Status group and badge of a child session that finished successfully",
  },
  "agentSessions.lineage.status.failed": {
    message: "실패",
    description: "Status group and badge of a child session that failed",
  },
  "agentSessions.lineage.status.canceled": {
    message: "취소됨",
    description: "Status group and badge of a child session that was canceled",
  },
  "agentSessions.sidebarMine": {
    message: "내 세션",
    description:
      "Sessions sidebar — show only the sessions you started (Devin's default)",
  },
  "agentSessions.sidebarEveryone": {
    message: "모두",
    description: "Sessions sidebar — show every member's sessions",
  },
  "agentSessions.sidebarOwnerFilter": {
    message: "세션 소유자",
    description:
      "Sessions sidebar — accessible name of the Mine/Everyone toggle",
  },
  "agentSessions.sidebarEmptyMine": {
    message:
      '내가 시작한 세션이 없습니다. 이전 세션은 "모두"에서 볼 수 있습니다.',
    description:
      "Sessions sidebar — empty state under Mine. Sessions from before 2026-11-08 record no starter, so they appear only under Everyone",
  },
  "agentSessions.startedByPerson": {
    message: "{name} 님이 시작함",
    description: "Session header and list rows — who started the session",
  },
  "agentSessions.startedForPerson": {
    message: "{name} 님을 위해",
    description:
      "Session header — after 'Started by automation X': the member the automation runs as",
  },
  "agentSessions.editTags": {
    message: "태그 편집",
    description: "Session header menu — open the tag editor",
  },
  "agentSessions.editTagsTitle": {
    message: "태그 편집",
    description: "Tag editor dialog title",
  },
  "agentSessions.editTagsDescription": {
    message:
      "입력한 태그가 세션의 현재 태그를 대체합니다. 쉼표나 공백으로 구분하며, 비워 두면 모두 지워집니다.",
    description: "Tag editor dialog description",
  },
  "agentSessions.editTagsHint": {
    message:
      "최대 20개: 소문자, 숫자, 콜론, 밑줄, 하이픈만 사용할 수 있습니다.",
    description: "Tag editor dialog — the tag rules",
  },
  "agentSessions.editTagsSave": {
    message: "태그 저장",
    description: "Tag editor dialog — save button",
  },
  "agentSessions.editTagsSaved": {
    message: "태그를 저장했습니다",
    description: "Toast after the tags were replaced",
  },
  "agentSessions.settings": {
    message: "설정",
    description: "Session header menu — open the session's settings (w10/m599)",
  },
  "agentSessions.settingsTitle": {
    message: "세션 설정",
    description: "Session settings dialog title (w10/m599)",
  },
  "agentSessions.settingsDescription": {
    message:
      "실행 중인 이 세션에 적용되는 설정입니다. 워크스페이스 관리자만 변경할 수 있습니다.",
    description:
      "Session settings dialog description: the settings apply while the session runs and only a workspace admin changes them (w10/m599)",
  },
  "agentSessions.settingsGithubCliHint": {
    message:
      "끄기를 선택하면 이 세션과 이 세션이 시작한 세션에서 다음 gh 호출부터 gh가 꺼집니다. 켜기 또는 상속을 선택하면 세션의 다음 턴부터 gh가 돌아옵니다.",
    description:
      "Session settings dialog — when a GitHub CLI change applies: an off at the next gh call, an on or inherit at the next turn (w10/m599)",
  },
  "agentSessions.settingsSave": {
    message: "설정 저장",
    description: "Session settings dialog — save button (w10/m599)",
  },
  "agentSessions.settingsSaved": {
    message: "설정을 저장했습니다",
    description: "Toast after a session's settings were saved (w10/m599)",
  },
  "agentSessions.terminalButton": {
    message: "터미널",
    description:
      "Session header button that opens the session's Terminal (w10/m615)",
  },
  "agentSessions.terminalTitle": {
    message: "터미널",
    description: "Title of the sheet holding the session's Terminal",
  },
  "agentSessions.terminalDescription": {
    message:
      "이 세션 샌드박스의 /workspace에서 여는 셸입니다. 열면 닫힌 샌드박스가 시작됩니다.",
    description: "Description under the Terminal sheet's title",
  },
  "agentSessions.terminalConnecting": {
    message: "샌드박스에 연결하는 중…",
    description: "Terminal status while it connects to the sandbox",
  },
  "agentSessions.terminalStarting": {
    message: "샌드박스를 시작하는 중…",
    description: "Terminal status while the session's sandbox starts",
  },
  "agentSessions.terminalStartingDetail": {
    message: "샌드박스가 시작되면 셸이 열립니다. 보통 1분 안에 열립니다.",
    description: "Terminal detail while the session's sandbox starts",
  },
  "agentSessions.terminalStillStarting": {
    message:
      "샌드박스를 아직 시작하는 중입니다({elapsed}). 시작되면 셸이 열립니다.",
    description:
      "Terminal detail every 30 seconds while the sandbox still starts; {elapsed} is how long, e.g. 1m 30s",
  },
  "agentSessions.terminalConnected": {
    message: "연결됨",
    description: "Terminal status when the shell is live",
  },
  "agentSessions.terminalClosed": {
    message: "세션 닫힘",
    description: "Terminal status when the shell ended",
  },
  "agentSessions.terminalErrorStatus": {
    message: "연결 오류",
    description: "Terminal status when it failed to connect or errored",
  },
  "agentSessions.terminalEnded": {
    message: "세션 종료됨",
    description: "Terminal status when the server ended it for a stated reason",
  },
  "agentSessions.terminalReconnect": {
    message: "다시 연결",
    description: "Button that opens the Terminal again",
  },
  "agentSessions.terminalErrorGeneric": {
    message: "샌드박스에서 셸을 열 수 없습니다.",
    description: "Terminal generic connection error",
  },
  "agentSessions.terminalUnavailable": {
    message:
      "이 플랫폼에서는 브라우저 터미널이 활성화되어 있지 않습니다. 대신 연결에서 SSH로 샌드박스를 여세요.",
    description:
      "Terminal error when the browser terminal is not configured on the platform (503)",
  },
  "agentSessions.terminalCanceled": {
    message: "이 세션은 취소되어 열 샌드박스가 없습니다.",
    description: "Why a canceled session's Terminal has nothing to open",
  },
  "agentSessions.terminalArchived": {
    message:
      "이 세션은 보관되었고 샌드박스가 닫혔습니다. 터미널을 열려면 보관을 해제하세요.",
    description: "Why an archived session's Terminal has nothing to open",
  },
  "agentSessions.terminalCannotStart": {
    message:
      "이 세션은 실행 중이 아니며 샌드박스가 닫혔습니다. 다시 시작하려면 세션에 메시지를 보낸 다음 터미널을 다시 여세요.",
    description:
      "Terminal refusal for a member who may view the sandbox but not start it, as the SSH banner says",
  },
  "agentSessions.terminalStartRefused": {
    message: "샌드박스를 시작할 수 없습니다: {reason}",
    description:
      "Terminal refusal when the sandbox could not start; {reason} is the refusal (payment method, billing, the live-sandbox limit)",
  },
  "agentSessions.terminalEndRevoked": {
    message: "이 세션에 대한 접근 권한이 종료되었습니다.",
    description:
      "Terminal end reason: the caller lost access to the session (deliberately unspecific)",
  },
  "agentSessions.terminalEndSandboxGone": {
    message: "샌드박스가 중지되었습니다. 다시 연결하면 다시 시작됩니다.",
    description: "Terminal end reason: the sandbox stopped under the shell",
  },
  "agentSessions.terminalEndTimeout": {
    message:
      "터미널이 시간 제한에 도달했습니다. 계속 작업하려면 다시 연결하세요.",
    description: "Terminal end reason: it reached its time limit",
  },
  "agentSessions.terminalEndUnverified": {
    message:
      "접근 권한을 다시 확인할 수 없어 터미널을 닫았습니다. 다시 연결해 보세요.",
    description:
      "Terminal end reason: the periodic access check could not complete",
  },
  "agentSessions.terminalEndNoSandbox": {
    message: "샌드박스가 시작되지 않았습니다. 다시 연결해 보세요.",
    description:
      "Terminal end reason: the sandbox's start ended without a sandbox",
  },
  "agentSessions.terminalEndWakeTimeout": {
    message: "샌드박스가 제시간에 시작되지 않았습니다. 다시 연결해 보세요.",
    description:
      "Terminal end reason: the sandbox did not start within the start's ceiling",
  },
  "agentSessions.terminalSandboxClosed": {
    message:
      "이 세션의 샌드박스가 닫혔습니다. 셸을 열려면 샌드박스를 시작하세요.",
    description:
      "Terminal sheet the page reopened from an earlier visit, on a session whose sandbox has closed: nothing starts until the button below is pressed (w10/060)",
  },
  "agentSessions.terminalStartSandbox": {
    message: "샌드박스 시작",
    description:
      "Button in a reopened Terminal sheet that starts the session's closed sandbox and opens the shell (w10/060)",
  },
};

export default koAgentSessions;
