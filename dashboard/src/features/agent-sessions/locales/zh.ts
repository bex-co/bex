import type { TranslationEntry } from "@/i18n";

const zhAgentSessions: Record<string, TranslationEntry> = {
  // --- Page / list ---
  "agentSessions.pageTitle": {
    message: "智能体会话",
    description: "Agent sessions list page heading and document title",
  },
  "agentSessions.pageSubtitle": {
    message:
      "将编码任务分配给云端智能体。它会在沙箱内的 bex-agent/* 分支上工作，并提交草稿 PR。",
    description: "Agent sessions page subtitle explaining the feature",
  },
  "agentSessions.listTitle": {
    message: "会话",
    description: "Agent sessions table card title",
  },
  "agentSessions.colTask": {
    message: "任务",
    description: "Agent sessions table column header — the session's prompt",
  },
  "agentSessions.colRepo": {
    message: "仓库",
    description: "Agent sessions table column header — repo/branch",
  },
  "agentSessions.colAgent": {
    message: "智能体",
    description: "Agent sessions table column header — the driver agent",
  },
  "agentSessions.colPhase": {
    message: "阶段",
    description: "Agent sessions table column header — lifecycle phase chip",
  },
  "agentSessions.colPr": {
    message: "PR",
    description: "Agent sessions table column header — draft pull request",
  },
  "agentSessions.colCreated": {
    message: "创建时间",
    description: "Agent sessions table column header — createdAt relative age",
  },
  "agentSessions.prBadge": {
    message: "#{number}",
    description: "Draft PR badge label — the pull-request number",
  },
  "agentSessions.errorTitle": {
    message: "无法加载智能体会话",
    description: "Agent sessions list error state heading",
  },
  // --- Empty state ---
  "agentSessions.emptyTitle": {
    message: "还没有智能体会话",
    description: "Agent sessions list empty-state heading",
  },
  "agentSessions.emptyBody": {
    message:
      "从「新建会话」开始——描述任务并用 @ 提及一个仓库，智能体就会在云端沙箱中完成任务，并提交草稿 PR。",
    description: "Agent sessions list empty-state body",
  },
  // --- Phase chips ---
  "agentSessions.phase.creating": {
    message: "创建中",
    description: "Agent session phase chip — sandbox is being created",
  },
  "agentSessions.phase.running": {
    message: "运行中",
    description: "Agent session phase chip — a turn is in progress",
  },
  "agentSessions.phase.resuming": {
    message: "恢复中",
    description: "Agent session phase chip — session is resuming",
  },
  "agentSessions.phase.redispatching": {
    message: "重新调度中",
    description: "Agent session phase chip — session is being redispatched",
  },
  "agentSessions.phase.completed": {
    message: "已完成",
    description: "Agent session phase chip — session finished successfully",
  },
  "agentSessions.phase.failed": {
    message: "已失败",
    description: "Agent session phase chip — session failed",
  },
  "agentSessions.phase.canceling": {
    message: "取消中",
    description: "Agent session phase chip — cancellation in progress",
  },
  "agentSessions.phase.canceled": {
    message: "已取消",
    description: "Agent session phase chip — session was canceled",
  },
  // --- Composer (prompt box) ---
  "agentSessions.promptHeading": {
    message: "让智能体做点什么？",
    description: "Centered heading over the /agents prompt-box composer",
  },
  "agentSessions.taskLabel": {
    message: "任务",
    description: "Composer — the prominent task prompt textarea label",
  },
  "agentSessions.taskPlaceholder": {
    message:
      "描述一个任务，并用 @ 提及一个仓库来限定范围。请具体说明——智能体会自主工作并提交草稿 PR。",
    description: "Composer — task textarea placeholder",
  },
  "agentSessions.mentionButton": {
    message: "提及仓库或会话",
    description: "Composer toolbar — accessible label of the @ mention button",
  },
  "agentSessions.configButton": {
    message: "配置",
    description: "Composer toolbar — the Configuration popover trigger",
  },
  "agentSessions.repoNudge": {
    message: "请先用 @ 选择一个仓库。",
    description:
      "Inline nudge anchored at the @ button when submitting without a repo chip",
  },
  "agentSessions.chipRemove": {
    message: "移除 {name}",
    description: "Accessible label of a mention chip's remove button",
  },
  // --- @ mention picker ---
  "agentSessions.mentionCategoryRepos": {
    message: "仓库",
    description: "Mention picker — the repositories category row",
  },
  "agentSessions.mentionCategoryReposDesc": {
    message: "将会话限定到已连接的 GitHub 仓库",
    description: "Mention picker — repositories category one-line description",
  },
  "agentSessions.mentionCategorySessions": {
    message: "会话",
    description: "Mention picker — the prior-sessions category row",
  },
  "agentSessions.mentionCategorySessionsDesc": {
    message: "引用之前的智能体会话作为上下文",
    description: "Mention picker — sessions category one-line description",
  },
  "agentSessions.mentionNoResults": {
    message: "没有匹配项",
    description:
      "Mention picker — empty state when the fuzzy filter matches nothing",
  },
  "agentSessions.mentionReposEmpty": {
    message: "没有仓库——请先在工作区设置中连接 GitHub。",
    description:
      "Mention picker — empty state when no installation repos exist",
  },
  "agentSessions.mentionSessionsEmpty": {
    message: "还没有会话",
    description:
      "Mention picker — empty state when the workspace has no sessions",
  },
  "agentSessions.mentionConnected": {
    message: "已通过 GitHub App 连接",
    description: "Mention picker readiness footer — the repo is app-connected",
  },
  "agentSessions.mentionDefaultBranch": {
    message: "默认分支：{branch}",
    description: "Mention picker readiness footer — the repo's default branch",
  },
  "agentSessions.branchLabel": {
    message: "分支",
    description: "Composer — working branch input label",
  },
  "agentSessions.branchInvalid": {
    message: "分支必须位于 bex-agent/ 之下。",
    description:
      "Composer — validation error when the branch leaves the bex-agent/* namespace",
  },
  "agentSessions.branchHint": {
    message: "智能体会提交到 bex-agent/* 分支，并从该分支提交草稿 PR。",
    description: "Composer — branch field helper text (bex-agent/* guidance)",
  },
  "agentSessions.agentLabel": {
    message: "智能体",
    description: "Composer — agent select label",
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
  "agentSessions.modelLabel": {
    message: "模型",
    description: "Composer (advanced) — model override input label",
  },
  "agentSessions.modelPlaceholder": {
    message: "提供方默认",
    description: "Composer (advanced) — model input placeholder",
  },
  "agentSessions.modelHint": {
    message: "所选智能体的可选模型覆盖。",
    description: "Composer (advanced) — model field helper text",
  },
  "agentSessions.modelEndpointLabel": {
    message: "模型端点",
    description: "Composer (advanced) — model endpoint input label",
  },
  "agentSessions.modelEndpointPlaceholder": {
    message: "https://api.example.com",
    description: "Composer (advanced) — model endpoint input placeholder",
  },
  "agentSessions.modelEndpointHint": {
    message: "模型提供方的可选自定义 HTTPS 端点。",
    description: "Composer (advanced) — model endpoint field helper text",
  },
  "agentSessions.egressLabel": {
    message: "出站允许列表",
    description: "Composer (advanced) — extra egress hostnames textarea label",
  },
  "agentSessions.egressPlaceholder": {
    message: "每行一个主机名",
    description: "Composer (advanced) — egress allowlist textarea placeholder",
  },
  "agentSessions.egressHint": {
    message:
      "除模型端点和内置软件包镜像源之外，沙箱可访问的额外公共主机名（最多 32 个）。",
    description: "Composer (advanced) — egress allowlist field helper text",
  },
  "agentSessions.egressTooMany": {
    message: "主机名过多——允许列表最多支持 32 个条目。",
    description: "Composer — validation error when more than 32 egress entries",
  },
  "agentSessions.submit": {
    message: "开始会话",
    description: "Composer — submit button label",
  },
  "agentSessions.submitting": {
    message: "正在开始…",
    description: "Composer — submit button label while the create is in flight",
  },
  // --- Unavailable (503) + create error ---
  "agentSessions.unavailableTitle": {
    message: "智能体会话尚未配置",
    description: "Composer — house callout title when the feature is 503",
  },
  "agentSessions.unavailableBody": {
    message: "该平台尚未启用云端智能体会话。请联系运维配置智能体会话网关。",
    description: "Composer — house callout body when the feature is 503",
  },
  "agentSessions.createErrorTitle": {
    message: "无法开始会话",
    description: "Composer — error alert title when the create fails",
  },
  // --- Detail page (t004) ---
  "agentSessions.detailTitle": {
    message: "会话",
    description: "Agent-session detail page document title",
  },
  "agentSessions.backToList": {
    message: "所有会话",
    description: "Detail page — back link to the /agents list",
  },
  "agentSessions.detailErrorTitle": {
    message: "无法加载该会话",
    description: "Detail page — heading when the session read fails",
  },
  "agentSessions.conversationTitle": {
    message: "对话",
    description: "Detail page — the live conversation column card title",
  },
  "agentSessions.failureTitle": {
    message: "会话失败",
    description: "Detail page — heading over a failed session's reason",
  },
  // Header meta + cancel
  "agentSessions.metaDuration": {
    message: "时长 {duration}",
    description: "Detail header — elapsed session wall-clock",
  },
  "agentSessions.metaTurns": {
    message: "{turns} 轮",
    description: "Detail header — number of prompt turns taken",
  },
  "agentSessions.metaDelivery": {
    message: "交付 {mode}",
    description: "Detail header — how the last turn's sandbox was obtained",
  },
  "agentSessions.delivery.resume": {
    message: "恢复",
    description: "Delivery mode — the sandbox was resumed",
  },
  "agentSessions.delivery.redispatch": {
    message: "重新调度",
    description: "Delivery mode — a new sandbox was dispatched",
  },
  "agentSessions.cancel": {
    message: "取消",
    description: "Detail header — cancel-session button label",
  },
  "agentSessions.cancelDisabledCanceling": {
    message: "取消已在进行中。",
    description: "Detail header — tooltip on the disabled cancel button",
  },
  "agentSessions.canceling": {
    message: "正在取消…",
    description: "Detail header — confirm button label while the cancel runs",
  },
  "agentSessions.cancelSuccess": {
    message: "正在取消会话…",
    description: "Detail header — toast after the cancel is accepted",
  },
  "agentSessions.cancelConfirmTitle": {
    message: "取消该会话？",
    description: "Cancel confirm dialog title",
  },
  "agentSessions.cancelConfirmBody": {
    message: "这将停止智能体。已推送到分支的提交和草稿 PR 都会保留。",
    description: "Cancel confirm dialog body — states pushed work is preserved",
  },
  "agentSessions.cancelConfirmDismiss": {
    message: "继续运行",
    description: "Cancel confirm dialog — dismiss button",
  },
  "agentSessions.cancelConfirmProceed": {
    message: "取消会话",
    description: "Cancel confirm dialog — proceed button",
  },
  // PR card
  "agentSessions.prCardTitle": {
    message: "拉取请求",
    description: "Detail page — draft-PR card title",
  },
  "agentSessions.prCardNone": {
    message: "尚无拉取请求。智能体推送工作后会打开一个草稿 PR。",
    description: "PR card — shown before the session has opened a PR",
  },
  "agentSessions.prCardHeadSha": {
    message: "头部提交",
    description: "PR card — label for the head SHA",
  },
  // Evidence panel
  "agentSessions.evidenceTitle": {
    message: "证据",
    description: "Detail page — bounded evidence card title",
  },
  "agentSessions.evidenceEmpty": {
    message: "尚未捕获证据。",
    description: "Evidence panel — empty state",
  },
  "agentSessions.evidenceCommits": {
    message: "{count} 次提交",
    description: "Evidence panel — commit count",
  },
  "agentSessions.evidenceCommandLog": {
    message: "命令日志",
    description: "Evidence panel — command log section label",
  },
  "agentSessions.evidenceTestOutput": {
    message: "测试输出",
    description: "Evidence panel — test output section label",
  },
  "agentSessions.evidenceOutputTail": {
    message: "输出末尾",
    description: "Evidence panel — output tail section label",
  },
  "agentSessions.evidenceChangedFiles": {
    message: "变更文件",
    description: "Evidence panel — changed-files section label",
  },
  "agentSessions.evidenceTruncated": {
    message: "部分捕获的输出已被截断。",
    description: "Evidence panel — honest truncation note",
  },
  // Steering composer
  "agentSessions.steerTitle": {
    message: "引导该会话",
    description: "Steering composer card title",
  },
  "agentSessions.steerErrorTitle": {
    message: "发送失败",
    description: "Steering composer — inline error alert title",
  },
  "agentSessions.steerPlaceholderIdle": {
    message: "发送后续指令，在同一分支上开始新一轮。",
    description: "Steering composer — textarea placeholder for an idle session",
  },
  "agentSessions.steerPlaceholderLive": {
    message: "发送消息以引导正在运行的智能体。",
    description: "Steering composer — textarea placeholder for a live session",
  },
  "agentSessions.steerHintIdle": {
    message: "该会话处于空闲状态——发送将在同一分支上重新调度新一轮。",
    description: "Steering composer — helper text for the redispatch route",
  },
  "agentSessions.steerHintLive": {
    message: "你的消息将作为实时轮次发送到对话中。",
    description: "Steering composer — helper text for the live chat route",
  },
  "agentSessions.steerDisabledCanceling": {
    message: "该会话正在取消。已推送的工作会保留。",
    description: "Steering composer — disabled reason while canceling",
  },
  "agentSessions.steerDisabledCanceled": {
    message: "该会话已取消。",
    description: "Steering composer — disabled reason once canceled",
  },
  "agentSessions.steerDisabledStream": {
    message: "对话流不可用，实时引导已暂停。",
    description:
      "Steering composer — disabled reason when the m43 stream is down",
  },
  "agentSessions.steerDisabledInFlight": {
    message: "有一轮正在进行。请等待其完成后再发送。",
    description: "Steering composer — disabled reason while a turn streams",
  },
  "agentSessions.steerSubmit": {
    message: "发送",
    description: "Steering composer — submit button label",
  },
  "agentSessions.steerSending": {
    message: "正在发送…",
    description: "Steering composer — submit button label while sending",
  },
  "agentSessions.steerSuccess": {
    message: "已发送——正在重新调度新一轮。",
    description: "Steering composer — toast after a redispatch is accepted",
  },
  // --- Typed AGENT_SESSION_* error messages (keyed by errors.ts messageKey) ---
  "agentSessions.errors.AGENT_SESSION_INPUT_INVALID": {
    message: "输入无效。请检查仓库、分支和任务后重试。",
    description: "Mapped message for AGENT_SESSION_INPUT_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_ID_INVALID": {
    message: "该会话 id 无效。",
    description: "Mapped message for AGENT_SESSION_ID_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_FOUND": {
    message: "该会话已不存在。",
    description: "Mapped message for AGENT_SESSION_NOT_FOUND",
  },
  "agentSessions.errors.AGENT_SESSION_CONFLICT": {
    message: "该会话正在执行其他操作，请稍后重试。",
    description: "Mapped message for AGENT_SESSION_CONFLICT",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_STEERABLE": {
    message: "该会话在当前阶段（{phase}）无法引导。",
    description: "Mapped message for AGENT_SESSION_NOT_STEERABLE",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_RESUMABLE": {
    message: "该会话在当前阶段（{phase}）无法恢复。",
    description: "Mapped message for AGENT_SESSION_NOT_RESUMABLE",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_ATTACHABLE": {
    message: "该会话在当前阶段（{phase}）无法接入。",
    description: "Mapped message for AGENT_SESSION_NOT_ATTACHABLE",
  },
  "agentSessions.errors.AGENT_SESSION_TURN_IN_FLIGHT": {
    message: "已有一个轮次正在运行。请等待其完成后再发送。",
    description: "Mapped message for AGENT_SESSION_TURN_IN_FLIGHT",
  },
  "agentSessions.errors.AGENT_SESSION_MODEL_ENDPOINT_INVALID": {
    message: "模型端点必须是有效的 HTTPS URL。",
    description: "Mapped message for AGENT_SESSION_MODEL_ENDPOINT_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_EGRESS_ALLOWLIST_INVALID": {
    message: "出站允许列表条目“{entry}”无效：{reason}",
    description: "Mapped message for AGENT_SESSION_EGRESS_ALLOWLIST_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_EGRESS_ALLOWLIST_IMMUTABLE": {
    message: "会话开始后无法更改出站允许列表。",
    description: "Mapped message for AGENT_SESSION_EGRESS_ALLOWLIST_IMMUTABLE",
  },
  "agentSessions.errors.AGENT_SESSION_EGRESS_PHASE_INVALID": {
    message: "该会话在当前阶段无法设置出站允许列表。",
    description: "Mapped message for AGENT_SESSION_EGRESS_PHASE_INVALID",
  },

  // --- Conversation column (t002) ---
  "agentSessions.conversationLoading": {
    message: "正在加载对话…",
    description: "Placeholder while the client-only conversation column loads",
  },
  "agentSessions.conversationConnecting": {
    message: "正在连接会话流…",
    description: "Shown while the conversation stream replay is in flight",
  },
  "agentSessions.conversationEmpty": {
    message: "暂无对话。",
    description: "Shown when a session has produced no transcript parts",
  },
  "agentSessions.conversationEnded": {
    message: "会话已结束。",
    description: "Footer note under a terminal session's replayed transcript",
  },
  "agentSessions.conversationUnavailable": {
    message: "对话流当前不可用。",
    description:
      "Degraded-state message when the m43 stream endpoint errors or is unconfigured",
  },
  "agentSessions.groupThought": {
    message: "思考",
    description: "Collapsible header for the agent's reasoning group",
  },
  "agentSessions.groupPlan": {
    message: "计划",
    description: "Collapsible header for the agent's plan/task checklist group",
  },
  "agentSessions.groupTerminal": {
    message: "终端",
    description: "Collapsible header for a terminal-output group",
  },
  "agentSessions.groupCommand": {
    message: "命令",
    description: "Collapsible header for a command/tool-call group",
  },
  "agentSessions.groupDiff": {
    message: "差异",
    description: "Label for a file-diff group when no path is given",
  },
  "agentSessions.terminalNoOutput": {
    message: "（无输出）",
    description: "Placeholder when a terminal group has no captured output",
  },
  "agentSessions.toolInput": {
    message: "输入",
    description: "Section label for a tool call's input",
  },
  "agentSessions.toolOutput": {
    message: "输出",
    description: "Section label for a tool call's output",
  },
  "agentSessions.toolError": {
    message: "错误",
    description: "Section label for a tool call's error text",
  },
  "agentSessions.toolStateRunning": {
    message: "运行中",
    description: "Badge label for a tool call awaiting output",
  },
  "agentSessions.toolStateDone": {
    message: "完成",
    description: "Badge label for a completed tool call",
  },
  "agentSessions.toolStateError": {
    message: "失败",
    description: "Badge label for a failed tool call",
  },
  "agentSessions.activityWorking": {
    message: "处理中…",
    description: "Activity-group summary while any tool step is still pending",
  },
  "agentSessions.activityEdited": {
    message: "已编辑 {count} 个文件",
    description: "Activity-group summary when the turn edited files (diffs)",
  },
  "agentSessions.activityRan": {
    message: "已运行 {count} 条命令",
    description: "Activity-group summary when the turn ran commands/terminals",
  },
  "agentSessions.activitySteps": {
    message: "{count} 个步骤",
    description: "Activity-group summary fallback — the step count",
  },
  "agentSessions.scrollToBottom": {
    message: "滚动到最新",
    description:
      "Accessible label for the floating jump-to-bottom button in the conversation column",
  },
  // --- Full-page chat restructure (w3/m44) ---
  "agentSessions.newSession": {
    message: "新建会话",
    description: "Sessions sidebar — start-a-new-session affordance",
  },
  "agentSessions.recentSessions": {
    message: "最近",
    description: "Sessions sidebar — heading over the recent sessions list",
  },
  "agentSessions.sidebarEmpty": {
    message: "暂无会话",
    description: "Sessions sidebar — empty state when the workspace has none",
  },
  "agentSessions.sidebarLabel": {
    message: "智能体会话",
    description: "Sessions sidebar — accessible landmark label",
  },
  // --- Sidebar ---
  "agentSessions.sidebarSearch": {
    message: "搜索会话",
    description: "Sessions sidebar — search toggle + input accessible label",
  },
  "agentSessions.sidebarSearchPlaceholder": {
    message: "搜索会话…",
    description: "Sessions sidebar — search input placeholder",
  },
  "agentSessions.sidebarMore": {
    message: "查看全部会话",
    description: "Sessions sidebar — More action reaching the standalone list",
  },
  "agentSessions.sidebarNoMatches": {
    message: "没有匹配的会话",
    description:
      "Sessions sidebar — empty state when the search matches nothing",
  },
  "agentSessions.statusPhrase.prReady": {
    message: "PR 已就绪",
    description: "Sidebar status phrase — completed session with a draft PR",
  },
  "agentSessions.statusPhrase.working": {
    message: "工作中…",
    description: "Sidebar status phrase — session still converging",
  },
  "agentSessions.evidenceToggle": {
    message: "证据",
    description: "Header — toggle that opens the evidence side panel",
  },
  "agentSessions.menuMore": {
    message: "更多操作",
    description: "Header — accessible label for the '…' overflow menu",
  },
  "agentSessions.openPr": {
    message: "打开拉取请求",
    description: "Header overflow menu — open the draft PR in a new tab",
  },
  "agentSessions.groupWorkedFor": {
    message: "工作了 {duration}",
    description: "Activity-group summary with a derived elapsed duration",
  },
  "agentSessions.groupWorked": {
    message: "已工作",
    description:
      "Activity-group summary when no duration could be derived (history replay)",
  },
  "agentSessions.groupThoughtFor": {
    message: "思考了 {duration}",
    description: "Reasoning-group summary with a derived elapsed duration",
  },
  "agentSessions.terminalStatus.completed": {
    message: "会话已休眠",
    description: "Terminal transcript status line — completed session",
  },
  "agentSessions.terminalStatus.failed": {
    message: "会话因错误结束",
    description: "Terminal transcript status line — failed session",
  },
  "agentSessions.terminalStatus.canceled": {
    message: "会话已取消",
    description: "Terminal transcript status line — canceled session",
  },
  "agentSessions.prInlineTitle": {
    message: "草稿拉取请求",
    description: "Inline PR card — heading",
  },
  "agentSessions.prBot": {
    message: "bot",
    description: "Inline PR card — the agent authored the PR (bot author tag)",
  },
  "agentSessions.prReview": {
    message: "查看",
    description: "Inline PR card — review/open action label",
  },
  "agentSessions.prDiffStat": {
    message: "+{added} −{deleted}",
    description: "Inline PR card — added/deleted line diff stat",
  },
};

export default zhAgentSessions;
