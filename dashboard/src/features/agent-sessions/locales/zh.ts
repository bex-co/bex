import type { TranslationEntry } from "@/i18n";

const zhAgentSessions: Record<string, TranslationEntry> = {
  // --- Page / list ---
  "agentSessions.pageTitle": {
    message: "智能体会话",
    description: "Agent sessions list page heading and document title",
  },
  "agentSessions.listTitle": {
    message: "最近",
    description: "Agent sessions recents heading — matches the rail",
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
    message: "无法加载会话。",
    description: "Agent sessions list error — one sentence, Retry beside it",
  },
  "agentSessions.retry": {
    message: "重试",
    description: "Retries a failed agent-sessions list request",
  },
  // --- Empty state ---
  "agentSessions.emptyBody": {
    message: "你开始的会话会显示在这里。",
    description: "Default empty — one quiet line under the composer",
  },
  "agentSessions.emptyArchivedTitle": {
    message: "没有已归档的会话",
    description: "Archived-only list empty-state heading",
  },
  "agentSessions.emptyArchivedBody": {
    message: "归档后的会话会显示在这里。",
    description: "Archived-only list empty-state body",
  },
  "agentSessions.emptyFilteredTitle": {
    message: "没有匹配的会话",
    description: "Phase-filtered list empty-state heading",
  },
  "agentSessions.emptyFilteredBody": {
    message: "请选择其他阶段或清除筛选条件。",
    description: "Phase-filtered list empty-state body",
  },
  "agentSessions.clearFilters": {
    message: "清除筛选",
    description: "Clears agent-session membership and phase filters",
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
  "agentSessions.phase.hibernating": {
    message: "休眠中",
    description: "Agent session phase chip — snapshot upload in progress",
  },
  "agentSessions.phase.hibernated": {
    message: "已休眠",
    description: "Agent session phase chip — reclaimed to a durable snapshot",
  },
  // --- Hibernation (ADR059 D5/D6) ---
  "agentSessions.pin": {
    message: "固定",
    description: "Button — pin a hibernated session so it never expires",
  },
  "agentSessions.unpin": {
    message: "取消固定",
    description: "Button — remove a session's never-expire pin",
  },
  "agentSessions.pinned": {
    message: "已固定",
    description: "Badge — this session is pinned (never expires)",
  },
  "agentSessions.pinSuccess": {
    message: "已固定 —— 此工作区不会过期。",
    description: "Toast — pin succeeded",
  },
  "agentSessions.unpinSuccess": {
    message: "已取消固定 —— 此工作区重新进入保留倒计时。",
    description: "Toast — unpin succeeded",
  },
  "agentSessions.hibernatedStorage": {
    message: "已休眠 · {size}",
    description: "Meta — hibernated state with snapshot storage size",
  },
  // --- Composer (prompt box) ---
  "agentSessions.taskLabel": {
    message: "任务",
    description: "Composer — the prominent task prompt textarea label",
  },
  "agentSessions.taskPlaceholder": {
    message: "描述一个编码任务…",
    description: "Composer — task editor placeholder",
  },
  "agentSessions.mentionButton": {
    message: "添加仓库或会话",
    description:
      "Composer toolbar — opens the repository/session mention picker",
  },
  "agentSessions.configButton": {
    message: "高级",
    description: "Composer toolbar — the Advanced popover trigger",
  },
  "agentSessions.repoChip": {
    message: "仓库 {repo}",
    description: "Composer toolbar — accessible name of the selected-repo chip",
  },
  "agentSessions.keyboardHint": {
    message: "Enter 开始 · Shift+Enter 换行",
    description: "Muted hint under the composer",
  },
  "agentSessions.connectGitHubTitle": {
    message: "连接 GitHub 后即可开始",
    description: "Composer callout when the workspace has no App repos",
  },
  "agentSessions.connectGitHubBody": {
    message:
      "云端智能体需要已连接的 GitHub 仓库。先为本工作区连接 GitHub App，再回来开始会话。",
    description: "Composer callout body when there are no installation repos",
  },
  "agentSessions.connectGitHub": {
    message: "连接 GitHub",
    description: "Composer CTA that starts the GitHub App install",
  },
  "agentSessions.connectGitHubSettings": {
    message: "工作区设置",
    description: "Secondary link to /workspace/settings from the GitHub CTA",
  },
  "agentSessions.exampleFixTests": {
    message: "修复失败的测试并提交草稿 PR",
    description: "First-run example prompt chip",
  },
  "agentSessions.exampleAddReadme": {
    message: "添加一份说明如何运行项目的 README",
    description: "First-run example prompt chip",
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
  "agentSessions.dependencyUnavailableTitle": {
    message: "智能体会话暂时不可用",
    description:
      "Composer — retryable dependency-outage title, distinct from configuration",
  },
  "agentSessions.dependencyUnavailableBody": {
    message: "平台依赖项当前出现问题。你的配置仍然完好，请稍后重试。",
    description:
      "Composer — retryable dependency-outage body that does not send the user to an operator",
  },
  "agentSessions.snapshotUnavailableTitle": {
    message: "会话存储暂时不可用",
    description: "Agent-session snapshot restore/delete outage title",
  },
  "agentSessions.snapshotUnavailableBody": {
    message: "无法访问会话快照存储。请稍后重试。",
    description: "Agent-session snapshot restore/delete outage body",
  },
  "agentSessions.modelKeyMissingTitle": {
    message: "请先添加模型提供方密钥",
    description:
      "Composer pre-flight title when the workspace has no model key",
  },
  "agentSessions.modelKeyMissingBody": {
    message:
      "此工作区需要模型提供方密钥才能启动智能体会话。请在工作区设置中添加后返回此处。",
    description: "Composer pre-flight body when the workspace has no model key",
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
  "agentSessions.untitled": {
    message: "未命名会话",
    description:
      "Last-resort session name when a session carries neither a repository nor a task prompt",
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
  "agentSessions.failureReasonFallback": {
    message: "会话启动失败。",
    description:
      "Detail page — generic failure text when the session carries no specific reason",
  },
  "agentSessions.failureRetry": {
    message: "重试",
    description:
      "Detail page — button to re-run a failed session's original task",
  },
  "agentSessions.failureRetrying": {
    message: "重试中…",
    description:
      "Detail page — retry button label while the redispatch is in flight",
  },
  "agentSessions.capacityFailureTitle": {
    message: "沙箱容量已用尽",
    description:
      "Detail page — heading when a session failed on the plan's sandbox limit",
  },
  "agentSessions.capacityFailureBody": {
    message:
      "此工作区已达到当前套餐的并发沙箱上限。升级套餐可同时运行更多沙箱，或停止一个空闲会话后重试。",
    description:
      "Detail page — explains a plan-limit sandbox failure and the two remedies",
  },
  "agentSessions.upgradePlan": {
    message: "升级套餐",
    description:
      "Detail page — CTA on a capacity failure; opens the change-plan dialog",
  },
  "agentSessions.provisioning": {
    message: "正在启动沙箱…",
    description:
      "Detail page — shown while a new/steered session provisions its sandbox before the conversation stream is available",
  },
  // Header meta + cancel
  "agentSessions.metaDuration": {
    message: "时长 {duration}",
    description: "Detail header — elapsed session wall-clock",
  },
  "agentSessions.metaTurns_other": {
    message: "{count} 轮",
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
    message: "该会话处于空闲状态——发送消息将在同一分支上重新调度新一轮。",
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
  "agentSessions.steerDisabledWaitForTurn": {
    message: "请等待当前轮次结束后再发送后续消息。",
    description:
      "Composer disabled reason while the current durable agent turn is running",
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
  "agentSessions.conversationNotStarted": {
    message: "沙箱准备就绪后，对话将显示在此处。",
    description:
      "Healthy pre-stream state while a newly created session is provisioning",
  },
  "agentSessions.conversationEmpty": {
    message: "暂无对话。",
    description: "Shown when a session has produced no transcript parts",
  },
  "agentSessions.turnError": {
    message: "第 {turn} 轮失败",
    description:
      "Recorded failure of an earlier turn; subsequent turns remain readable",
  },
  "agentSessions.continuitySessionLoad": {
    message: "智能体已恢复其保存的会话状态。",
    description:
      "System line when a fresh agent generation reloaded its own saved session (ADR047 D3 ladder rung 1)",
  },
  "agentSessions.continuityReprime": {
    message: "智能体已重启——上下文已从会话历史重建。",
    description:
      "System line when a fresh agent generation was primed from the durable transcript (ladder rung 2)",
  },
  "agentSessions.continuityTaskRedelivery": {
    message: "智能体已重启——原始任务已重新下发。",
    description:
      "System line when no prior turn ever reached the agent, so the task was re-sent (ladder rung 3)",
  },
  "agentSessions.conversationEndedEmpty": {
    message: "该会话在记录对话之前已结束。",
    description: "Ended session with no durable transcript parts",
  },
  "agentSessions.conversationEnded": {
    message: "会话已结束。",
    description: "Footer note under a terminal session's replayed transcript",
  },
  "agentSessions.conversationIncomplete": {
    message: "部分智能体输出未能完整保存",
    description:
      "Warning below a durable user turn whose assistant transcript is incomplete",
  },
  "agentSessions.conversationUnavailable": {
    message: "对话流当前不可用。",
    description:
      "Degraded-state message when the m43 stream endpoint errors or is unconfigured",
  },
  "agentSessions.conversationUnavailableTerminal": {
    message: "该会话的对话记录不可用。",
    description:
      "Terminal session — transcript unavailable rather than live stream degraded",
  },
  "agentSessions.showEarlierMessages": {
    message: "显示更早的 {count} 条消息",
    description:
      "Button revealing older transcript messages hidden by the render window",
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
  "agentSessions.activityEdited_other": {
    message: "已编辑 {count} 个文件",
    description: "Activity-group summary when the turn edited files (diffs)",
  },
  "agentSessions.activityRan_other": {
    message: "已运行 {count} 条命令",
    description: "Activity-group summary when the turn ran commands/terminals",
  },
  "agentSessions.activitySteps_other": {
    message: "{count} 个步骤",
    description: "Activity-group summary fallback — the step count",
  },
  "agentSessions.scrollToBottom": {
    message: "滚动到最新",
    description:
      "Accessible label for the floating jump-to-bottom button in the conversation column",
  },
  // --- Full-page chat restructure (w3/m44) ---
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
  "agentSessions.menuMore": {
    message: "更多操作",
    description: "Header — accessible label for the '…' overflow menu",
  },
  "agentSessions.openPr": {
    message: "打开拉取请求",
    description: "Header overflow menu — open the draft PR in a new tab",
  },
  "agentSessions.connect": {
    message: "连接",
    description: "Header — trigger for the Open-in-Zed / SSH connect menu",
  },
  "agentSessions.openInZed": {
    message: "在 Zed 中打开",
    description:
      "Connect menu — hotlink that opens the sandbox as a Zed remote project",
  },
  "agentSessions.openInZedHint": {
    message: "通过 SSH 打开沙箱的 /workspace，需要安装 Zed 编辑器。",
    description: "Connect menu — helper text under the Open-in-Zed action",
  },
  "agentSessions.connectSSH": {
    message: "SSH",
    description: "Connect menu — label above the copyable ssh command",
  },
  "agentSessions.sshCopy": {
    message: "复制 SSH 命令",
    description: "Connect menu — accessible label for the copy button",
  },
  "agentSessions.sshCopied": {
    message: "已复制 SSH 命令",
    description: "Connect menu — toast after copying the ssh command",
  },
  "agentSessions.sshCopyError": {
    message: "复制失败",
    description: "Connect menu — toast when copying the ssh command fails",
  },
  "agentSessions.groupWorkedFor": {
    message: "工作了 {duration}",
    description:
      "Activity-group summary with elapsed duration from persisted part timestamps (or ~arrival timing)",
  },
  "agentSessions.groupWorked": {
    message: "已工作",
    description:
      "Activity-group summary when no duration could be derived (history replay)",
  },
  "agentSessions.groupThoughtFor": {
    message: "思考了 {duration}",
    description:
      "Reasoning-group summary with elapsed duration from persisted part timestamps (or ~arrival timing)",
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
  "agentSessions.archive": {
    message: "归档",
    description: "Action putting a session out of the working set (ADR065)",
  },
  "agentSessions.unarchive": {
    message: "取消归档",
    description: "Action returning an archived session to the working set",
  },
  "agentSessions.archiveSuccess": {
    message: "会话已归档",
    description: "Toast after archiving a session",
  },
  "agentSessions.undoArchive": {
    message: "撤销",
    description: "Toast action that immediately unarchives a session",
  },
  "agentSessions.unarchiveSuccess": {
    message: "会话已取消归档",
    description: "Toast after unarchiving a session",
  },
  "agentSessions.archivedBadge": {
    message: "已归档",
    description: "Badge marking a session as archived (out of the working set)",
  },
  "agentSessions.delete": {
    message: "删除",
    description: "Destructive action permanently deleting a finished session",
  },
  "agentSessions.deleting": {
    message: "正在删除…",
    description: "Delete confirm button while the delete is in flight",
  },
  "agentSessions.deleteSuccess": {
    message: "会话已删除",
    description: "Toast after permanently deleting a session",
  },
  "agentSessions.deleteConfirmTitle": {
    message: "删除此会话？",
    description: "Delete confirmation dialog title",
  },
  "agentSessions.deleteConfirmBody": {
    message:
      "会话记录、对话转录以及任何休眠快照都将被永久删除。GitHub 上已推送的分支和拉取请求不受影响。此操作无法撤销。",
    description: "Delete confirmation dialog body",
  },
  "agentSessions.deleteConfirmDismiss": {
    message: "保留会话",
    description: "Delete confirmation dialog dismiss button",
  },
  "agentSessions.deleteConfirmProceed": {
    message: "删除会话",
    description: "Delete confirmation dialog destructive proceed button",
  },
  "agentSessions.colActions": {
    message: "操作",
    description: "Screen-reader label of the list's trailing actions column",
  },
  "agentSessions.filterActive": {
    message: "最近",
    description: "List membership tab — the unarchived working set",
  },
  "agentSessions.filterArchived": {
    message: "已归档",
    description: "List membership tab — archived sessions only",
  },
  "agentSessions.filterAll": {
    message: "全部",
    description: "List membership tab — archived and unarchived together",
  },
  "agentSessions.loadMore": {
    message: "加载更多",
    description: "Button fetching the next page of sessions",
  },
  "agentSessions.loadingMore": {
    message: "加载中…",
    description: "Load-more button while the next page is in flight",
  },
};

export default zhAgentSessions;
