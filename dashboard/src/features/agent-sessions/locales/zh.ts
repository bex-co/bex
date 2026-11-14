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
  "agentSessions.prState.open": {
    message: "已打开",
    description:
      "PR state badge beside a session's PR link — the pull request is open (a draft counts as open)",
  },
  "agentSessions.prState.merged": {
    message: "已合并",
    description:
      "PR state badge beside a session's PR link — the pull request was merged",
  },
  "agentSessions.prState.closed": {
    message: "已关闭",
    description:
      "PR state badge beside a session's PR link — the pull request was closed without merging",
  },
  "agentSessions.prStateFilter": {
    message: "按拉取请求状态筛选",
    description:
      "Session list — accessible name of the Any/Open/Merged/Closed PR-state filter",
  },
  "agentSessions.prStateFilterAny": {
    message: "全部",
    description:
      "Session list PR-state filter — the option that shows sessions whatever their PR state, including sessions with no PR",
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
  "agentSessions.agent.muse": {
    message: "Muse Code",
    description: "Composer — agent option: muse (Meta's Muse Code)",
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
  "agentSessions.modelKeyMissingTitle": {
    message: "请先添加模型提供方密钥",
    description:
      "Composer pre-flight title when the workspace has no model key",
  },
  "agentSessions.modelKeyMissingBody": {
    message:
      "此工作区还没有可用的模型。工作区管理员可以在“设置 → 智能体”中添加工作区自己的模型提供方密钥，然后返回此处。",
    description:
      "Composer pre-flight body when the workspace has neither its own model key nor the bex-provided model (ADR091 D7)",
  },
  "agentSessions.modelKeyMissingAction": {
    message: "打开“设置 → 智能体”",
    description:
      "Composer pre-flight link to the Settings → Agents model key field (/workspace/settings#agents)",
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
  "agentSessions.deliveryNoteTitle": {
    message: "改动尚未完全交付",
    description:
      "Detail page — heading over a completed session's note that the agent left work uncommitted, unpushed or without a pull request",
  },
  "agentSessions.deliveryNoteHint": {
    message: "发送一条跟进消息，让 agent 完成提交、推送并更新 pull request。",
    description:
      "Detail page — the next step shown under an undelivered-work note",
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
  // Stop turn (ADR047 D12): ends only the running turn, never the session.
  "agentSessions.stopTurn": {
    message: "停止本轮",
    description:
      "Detail header — Stop turn button label: ends only the running turn, unlike Cancel which ends the session",
  },
  "agentSessions.stoppingTurn": {
    message: "正在停止…",
    description: "Detail header — Stop turn button label while the stop runs",
  },
  "agentSessions.stopTurnHint": {
    message:
      "只结束正在运行的这一轮。会话保持打开，排队的消息保持原有顺序。快捷键：Ctrl/Cmd+Shift+Backspace。",
    description: "Detail header — tooltip on the Stop turn button",
  },
  "agentSessions.stopTurnStarting": {
    message: "本轮仍在启动。运行起来后即可停止。",
    description:
      "Detail header — tooltip on the disabled Stop turn button while the sandbox starts",
  },
  "agentSessions.stopTurnSuccess": {
    message: "正在停止本轮，约 15 秒内结束。",
    description: "Detail header — toast after Stop turn is accepted",
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
  "agentSessions.steerPlaceholderQueue": {
    message: "排入一条消息。当前轮次结束后运行。",
    description:
      "Steering composer — textarea placeholder while a turn runs or messages wait (ADR047 D12)",
  },
  "agentSessions.steerHintIdle": {
    message:
      "该会话处于空闲状态——发送消息将在同一分支上的全新沙箱中继续这段对话。",
    description: "Steering composer — helper text for the redispatch route",
  },
  "agentSessions.steerHintQueue": {
    message: "智能体正在工作。你的消息已排队，将在本轮结束后作为单独一轮运行。",
    description:
      "Steering composer — helper text while a turn runs or messages wait (ADR047 D12)",
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
  "agentSessions.steerQueued": {
    message: "已排队，当前轮次结束后运行。",
    description:
      "Steering composer — toast after a message was queued behind a running turn (ADR047 D12)",
  },
  // Queued messages (ADR047 D12): the list above the composer.
  "agentSessions.queueRegion": {
    message: "排队的消息",
    description: "Queued messages list — accessible name of the region",
  },
  "agentSessions.queueTitle_other": {
    message: "{count} 条排队的消息",
    description: "Queued messages list — heading with the count (plural)",
  },
  "agentSessions.queueHint": {
    message: "它们会在当前轮次之后，按此顺序逐条运行。",
    description:
      "Queued messages list — one-line explanation next to the heading",
  },
  "agentSessions.queuePosition": {
    message: "队列第 {position} 位",
    description: "Queued messages list — tooltip on a row's place number",
  },
  "agentSessions.queueAttachments_other": {
    message: "{count} 个附件",
    description: "Queued message row — attachments that ride with it (plural)",
  },
  "agentSessions.queueBlocked": {
    message: "暂时无法启动：{reason}。系统会自动重试。",
    description:
      "Queued message row — why the dispatcher could not start it (a spend ceiling or the live-sandbox limit)",
  },
  "agentSessions.queueSendNow": {
    message: "立即发送",
    description:
      "Queued message row — stops the running turn and runs this message next",
  },
  "agentSessions.queueMoveUp": {
    message: "上移",
    description: "Queued message row — move one place earlier",
  },
  "agentSessions.queueMoveDown": {
    message: "下移",
    description: "Queued message row — move one place later",
  },
  "agentSessions.queueEdit": {
    message: "编辑",
    description: "Queued message row — edit the message in place",
  },
  "agentSessions.queueRemove": {
    message: "移除",
    description: "Queued message row — remove it so it never runs",
  },
  "agentSessions.queueEditLabel": {
    message: "编辑排队的消息",
    description: "Queued message row — accessible name of the edit box",
  },
  "agentSessions.queueEditSave": {
    message: "保存",
    description: "Queued message row — save the edit",
  },
  "agentSessions.queueEditSaveSendNow": {
    message: "保存并立即发送",
    description:
      "Queued message row — save the edit, stop the running turn and run this message next",
  },
  "agentSessions.queueEditDiscard": {
    message: "放弃",
    description: "Queued message row — leave the edit without saving",
  },
  "agentSessions.queueEditHint": {
    message:
      "Enter 保存，Shift+Enter 换行，Ctrl/Cmd+Enter 保存并立即发送，Esc 放弃。",
    description: "Queued message row — keyboard hints while editing",
  },
  "agentSessions.queueSendNowStopping": {
    message: "正在停止当前轮次，这条消息接着运行。",
    description: "Queued message row — toast after Send now stopped a turn",
  },
  "agentSessions.queueSendNowNext": {
    message: "这条消息接着运行。",
    description:
      "Queued message row — toast after Send now when no turn was running",
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
  "agentSessions.errors.AGENT_SESSION_BRANCH_IN_USE": {
    message:
      "已有另一个会话在此分支上工作。请给那个会话发消息，或改用另一个 bex-agent/ 分支。",
    description: "Mapped message for AGENT_SESSION_BRANCH_IN_USE (w5/m382)",
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
  "agentSessions.errors.AGENT_SESSION_QUEUE_FULL": {
    message:
      "该会话已排了 {max} 条消息。请等待其中一条运行，或移除一条后再发送。",
    description: "Mapped message for AGENT_SESSION_QUEUE_FULL (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_QUEUE_INLINE_SECRETS": {
    message:
      "带有内联密钥的消息不能排队，因为密钥的值从不存储。请在会话空闲时发送，或改用工作区密钥。",
    description:
      "Mapped message for AGENT_SESSION_QUEUE_INLINE_SECRETS (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_QUEUED_MESSAGE_DISPATCHED": {
    message: "这条消息已经开始运行，无法再修改。",
    description:
      "Mapped message for AGENT_QUEUED_MESSAGE_DISPATCHED (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_QUEUED_MESSAGE_NOT_FOUND": {
    message: "这条消息已不在队列中。",
    description:
      "Mapped message for AGENT_QUEUED_MESSAGE_NOT_FOUND (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_QUEUED_MESSAGE_ATTACHMENTS_OWNED": {
    message:
      "这条消息带有发送者的附件，只有发送者可以编辑。你仍然可以移动或移除它。",
    description:
      "Mapped message for AGENT_QUEUED_MESSAGE_ATTACHMENTS_OWNED (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_NO_TURN_RUNNING": {
    message: "当前没有正在运行的轮次，无需停止。",
    description:
      "Mapped message for AGENT_SESSION_NO_TURN_RUNNING (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_TURN_STARTING": {
    message: "本轮仍在启动，暂时无法停止。请几秒后重试。",
    description: "Mapped message for AGENT_SESSION_TURN_STARTING (ADR047 D12)",
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
  "agentSessions.turnInterrupted": {
    message: "第 {turn} 轮在完成前被中断。",
    description:
      "Transcript line at the end of a turn a person stopped with Stop turn (ADR047 D12); not a failure",
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
  "agentSessions.showEarlierMessages_other": {
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
  "agentSessions.toolStateDenied": {
    message: "已拒绝",
    description: "Badge label for a tool call whose execution was denied",
  },
  "agentSessions.toolStateInterrupted": {
    message: "已中断",
    description:
      "Badge label for a tool call the session ended before it finished",
  },
  "agentSessions.toolStateError": {
    message: "失败",
    description: "Badge label for a failed tool call",
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
  "agentSessions.activityRead_other": {
    message: "已读取 {count} 个文件",
    description: "Activity-group summary segment — file reads",
  },
  "agentSessions.activitySearched_other": {
    message: "已搜索 {count} 次",
    description: "Activity-group summary segment — searches",
  },
  "agentSessions.activitySubagents_other": {
    message: "已运行 {count} 个子智能体",
    description: "Activity-group summary segment — delegated subagent tasks",
  },
  "agentSessions.activityTools_other": {
    message: "已使用 {count} 个工具",
    description: "Activity-group summary segment — tools of no known kind",
  },
  "agentSessions.activityFailed_other": {
    message: "{count} 个失败",
    description: "Activity-group summary segment — failed tool calls",
  },
  "agentSessions.outputClipped_other": {
    message: "已截断 {formatted} 个字符",
    description: "Badge on a tool output the server stored head+tail-capped",
  },
  "agentSessions.showFullOutput": {
    message: "显示完整输出",
    description:
      "Button on a clipped tool output that loads the complete text the server kept",
  },
  "agentSessions.hideFullOutput": {
    message: "隐藏完整输出",
    description: "Button that collapses a loaded full tool output",
  },
  "agentSessions.fullOutputNotRetained": {
    message: "未保留完整输出",
    description:
      "Note on a clipped tool output whose complete text the server did not keep (clipped before full copies were stored)",
  },
  "agentSessions.fullOutputLoading": {
    message: "正在加载完整输出…",
    description: "Status while a tool call's full output loads",
  },
  "agentSessions.fullOutputError": {
    message: "无法加载完整输出。",
    description: "Inline error when loading a tool call's full output failed",
  },
  "agentSessions.fullOutputCopy": {
    message: "复制完整输出",
    description:
      "Accessible label of the copy button on a loaded full tool output",
  },
  "agentSessions.fullOutputCopied": {
    message: "已复制完整输出",
    description: "Toast after copying a full tool output",
  },
  "agentSessions.fullOutputCopyError": {
    message: "无法复制完整输出",
    description: "Toast when copying a full tool output failed",
  },
  "agentSessions.conversationPartial": {
    message: "对话仅部分加载。",
    description:
      "Inline row when the transcript stream failed after some of it rendered",
  },
  "agentSessions.liveStatus": {
    message: "智能体状态",
    description:
      "Accessible label for the single live status line under a streaming transcript",
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
  "agentSessions.groupThoughtFor": {
    message: "思考了 {duration}",
    description:
      "Reasoning-group summary with elapsed duration from persisted part timestamps (or ~arrival timing)",
  },
  "agentSessions.terminalStatus.completed": {
    message: "Agent 已完成",
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
  "agentSessions.tagsLabel": {
    message: "标签",
    description: "Accessible label for a session row's tag list (w21/002)",
  },
  "agentSessions.filterByTag": {
    message: "显示带有标签 {tag} 的会话",
    description:
      "Accessible label of a tag chip that filters the session list to that tag",
  },
  "agentSessions.filterTagged": {
    message: "标签：{tag}",
    description: "Session list heading while narrowed to one tag",
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
      "会话记录和对话转录都将被永久删除。GitHub 上已推送的分支和拉取请求不受影响。此操作无法撤销。",
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
  // --- Attachments (ADR095) ---
  "agentSessions.attachFiles": {
    message: "附加文件",
    description:
      "Paperclip button that opens the file picker in the agent composers",
  },
  "agentSessions.attachmentRemove": {
    message: "移除 {name}",
    description: "Accessible label of the remove button on an attachment chip",
  },
  "agentSessions.attachmentTooLarge": {
    message: "超过 {max}",
    description: "Attachment chip error when a file exceeds the per-file limit",
  },
  "agentSessions.attachmentTooMany": {
    message: "每条消息最多 {max} 个文件",
    description: "Attachment chip error when a message has too many files",
  },
  "agentSessions.attachmentDropHint": {
    message: "拖放文件以附加",
    description: "Overlay shown while dragging files over a composer",
  },
  "agentSessions.sharedFileLabel": {
    message: "分享了一个文件",
    description: "Label above a file the agent shared with share_file",
  },
  "agentSessions.filesButton": {
    message: "文件",
    description: "Session header button that opens the session files",
  },
  "agentSessions.filesTitle": {
    message: "会话文件",
    description: "Title of the session files sheet",
  },
  "agentSessions.filesDescription": {
    message: "你在本会话中附加的文件，以及代理分享的文件。",
    description: "Description under the session files sheet title",
  },
  "agentSessions.filesEmpty": {
    message: "还没有文件。可以在消息中附加文件，或让代理分享文件。",
    description: "Empty state of the session files sheet",
  },
  "agentSessions.filesError": {
    message: "无法加载会话文件。",
    description: "Error state of the session files sheet",
  },
  "agentSessions.filesColumnName": {
    message: "名称",
    description: "Files table column: file name",
  },
  "agentSessions.filesColumnFrom": {
    message: "来源",
    description: "Files table column: who added the file",
  },
  "agentSessions.filesColumnTurn": {
    message: "轮次",
    description: "Files table column: turn number",
  },
  "agentSessions.filesColumnSize": {
    message: "大小",
    description: "Files table column: file size",
  },
  "agentSessions.filesFromInput": {
    message: "你",
    description: "Files table: a file the user attached",
  },
  "agentSessions.filesFromOutput": {
    message: "代理",
    description: "Files table: a file the agent shared",
  },
  "agentSessions.filesLoadMore": {
    message: "加载更多",
    description: "Files table: load the next page",
  },
  "agentSessions.attachmentsBlockSend": {
    message: "请先移除上传失败的文件再发送",
    description: "Hint when failed attachment chips block sending",
  },
  "agentSessions.attachmentOpen": {
    message: "打开 {name}",
    description:
      "Accessible label of an attachment thumbnail that opens the lightbox",
  },
  "agentSessions.attachmentDownload": {
    message: "下载 {name}",
    description: "Accessible label of an attachment download card",
  },
  "agentSessions.attachmentUnavailable": {
    message: "不可用",
    description:
      "Shown on an attachment whose download link could not be loaded",
  },
  "agentSessions.attachmentUploading": {
    message: "正在上传，{percent}%",
    description: "Screen-reader status of an attachment chip while uploading",
  },
  "agentSessions.attachmentReady": {
    message: "已上传",
    description:
      "Screen-reader status of an attachment chip once its upload landed",
  },
  "agentSessions.attachmentHttpError": {
    message: "上传失败（HTTP {status}）",
    description:
      "Attachment chip error when the server answered without a message",
  },
  "agentSessions.attachmentNetworkError": {
    message: "上传失败：网络错误",
    description:
      "Attachment chip error when the upload could not reach the server",
  },
  "agentSessions.attachmentsUploading": {
    message: "正在等待文件上传完成…",
    description: "Composer hint while attachment uploads block sending",
  },
  "agentSessions.spendWarningTitle": {
    message: "本月智能体模型支出已达上限的 {percent}%",
    description: "Composer warning at 80% of the spend cap (ADR091 D6)",
  },
  "agentSessions.spendWarningBody": {
    message: "已使用 {used} / {cap}。达到上限后将停止新的模型请求。",
    description: "Composer warning body",
  },
  "agentSessions.spendWarningBodyCharge": {
    message:
      "已使用 {used} / {cap}。先用 billing credits 抵扣；超出部分达到 {cap} 时会从绑定的卡扣款，会话继续运行。只有扣款失败才会停止新的模型请求。",
    description:
      "Composer warning body when the cap is a new workspace's charge point (ADR091 D10.2a)",
  },
  "agentSessions.spendReachedTitle.cap": {
    message: "已达到每月智能体模型支出上限",
    description: "Composer refusal title: the workspace's own cap",
  },
  "agentSessions.spendReachedTitle.newWorkspace": {
    message: "已达到新工作区支出限额",
    description: "Composer refusal title: the T0 trust-tier ceiling",
  },
  "agentSessions.spendReachedTitle.locked": {
    message: "智能体模型支出已暂停",
    description: "Composer refusal title: a payment hold",
  },
  "agentSessions.spendReachedBody.cap": {
    message:
      "此工作区本月已使用 {used}，达到 {cap} 的上限，无法启动新会话。工作区管理员可以提高上限，或等待每月 1 日（UTC）重置。",
    description: "Composer refusal body: the workspace's own cap",
  },
  "agentSessions.spendReachedBody.capMax": {
    message:
      "此工作区本月已使用 {used}，达到 {cap} 的上限，且上限已是可设置的最大值，无法启动新会话。上限将在每月 1 日（UTC）重置；如需更高限额，请联系支持团队。",
    description:
      "Composer refusal body: the workspace's own cap binds at its maximum, so no admin can raise it",
  },
  "agentSessions.spendReachedBody.newWorkspace": {
    message:
      "新工作区在首次成功付款前，最多可在 bex 提供的模型上支出 {cap}。付款后限额会提高，或在每月 1 日（UTC）重置。",
    description: "Composer refusal body: the T0 trust-tier ceiling",
  },
  "agentSessions.spendReachedBody.locked": {
    message:
      "由于付款失败或存在争议，此工作区的智能体模型支出被限制为 {cap}。请在账单页面结清欠款后再启动新会话。",
    description: "Composer refusal body: a payment hold",
  },
  "agentSessions.spendIncreaseCap": {
    message: "提高上限",
    description: "Admin action linking to Settings \u2192 Agents",
  },
  "agentSessions.spendGoToBilling": {
    message: "前往账单",
    description: "Link to the Billing page",
  },
  "agentSessions.spendBalanceDueTitle": {
    message: "付款失败",
    description: "Composer refusal title: BILLING_BALANCE_DUE",
  },
  "agentSessions.spendBalanceDueBody": {
    message: "在结清未付发票之前，使用 bex 提供的模型的智能体会话已暂停。",
    description: "Composer refusal body: BILLING_BALANCE_DUE",
  },
  "agentSessions.errors.AGENT_SESSION_TRUST_LIMIT": {
    message:
      "新工作区在首次成功付款前，最多可同时运行 {limit} 个使用 bex 提供的模型的智能体会话。请等待正在运行的会话结束或停止一个会话，然后重试。",
    description: "Mapped message for AGENT_SESSION_TRUST_LIMIT (ADR091 D10.1)",
  },
  "agentSessions.errors.AGENT_SESSION_TRUST_LIMIT_locked": {
    message:
      "由于付款失败或存在争议，此工作区最多可同时运行 {limit} 个使用 bex 提供的模型的智能体会话。请在账单页面结清欠款以解除限制。",
    description:
      "Mapped message for AGENT_SESSION_TRUST_LIMIT while a payment hold applies",
  },
  "agentSessions.errors.AGENT_SESSION_SPEND_LIMIT_INVALID": {
    message: "支出上限必须大于 $0 且不超过 $100,000。",
    description:
      "Mapped message for AGENT_SESSION_SPEND_LIMIT_INVALID (ADR094 D8)",
  },
  "agentSessions.errors.AGENT_SESSION_SPEND_LIMIT_UNAVAILABLE": {
    message:
      "支出上限仅适用于使用 bex 提供模型的会话。此工作区使用自己的模型密钥，请清除上限，或在“设置 → 智能体”中切换为 bex 模型。",
    description:
      "Mapped message for AGENT_SESSION_SPEND_LIMIT_UNAVAILABLE (ADR094 D8)",
  },
  "agentSessions.spendLimitLabel": {
    message: "支出上限（USD）",
    description:
      "Composer — label/aria-label of the optional per-session spend ceiling input",
  },
  "agentSessions.spendLimitPlaceholder": {
    message: "支出上限",
    description:
      "Composer — placeholder of the optional per-session spend ceiling input",
  },
  "agentSessions.spendLimitHint": {
    message:
      "可选。会话的 bex 模型支出达到该金额后即停止（大于 $0，不超过 $100,000）。",
    description: "Composer — help text under the spend limit input",
  },
  "agentSessions.spendLimitInvalid": {
    message: "请输入大于 $0 且不超过 $100,000 的金额，或留空。",
    description: "Composer — inline error for an out-of-range spend limit",
  },
  "agentSessions.failureSessionSpendLimitReached": {
    message:
      "此会话因模型支出达到会话上限 {limit} 而停止。请新建会话并设置更高的上限后继续。",
    description:
      "Failure callout for AGENT_MODEL_SPEND_CAP_REACHED when the session's own maxSpendUSD ceiling was hit",
  },
  "agentSessions.spendNote": {
    message: "模型支出 {spent}",
    description:
      "Session page — the session's model spend so far (outcome.cost)",
  },
  "agentSessions.spendNoteWithLimit": {
    message: "模型支出 {spent}，会话上限 {limit}",
    description:
      "Session page — model spend so far beside the session's spend limit",
  },
  "agentSessions.costNote": {
    message: "费用 {total} · 模型 {model}，计算 {compute}",
    description:
      "Session page — the session's total cost so far (outcome.cost.totalUSD) with its model and sandbox compute parts",
  },
  "agentSessions.costNoteWithLimit": {
    message: "费用 {total} · 模型 {model}（会话上限 {limit}），计算 {compute}",
    description:
      "Session page — total cost with its parts; the session spend limit caps the model part",
  },
  "agentSessions.resultTitle": {
    message: "结果",
    description:
      "Session page — heading over the turn's final structured result",
  },
  "agentSessions.resultDraftTitle": {
    message: "草稿结果",
    description:
      "Session page — heading over the agent's not-yet-final structured result while the turn runs",
  },
  "agentSessions.resultDraftHint": {
    message: "尚未定稿，智能体仍可能修改。",
    description: "Session page — note under a draft structured result",
  },
  "agentSessions.resultCopy": {
    message: "复制 JSON",
    description: "Session page — button copying the structured result",
  },
  "agentSessions.resultCopied": {
    message: "已复制",
    description: "Session page — toast after copying the structured result",
  },
  "agentSessions.failureSpendCapReached": {
    message:
      "此工作区已达到本月智能体模型支出上限。管理员可以在“设置 → 智能体”中提高上限，或等待每月 1 日（UTC）重置。",
    description: "Failure callout for AGENT_MODEL_SPEND_CAP_REACHED",
  },
  "agentSessions.failureBalanceDue": {
    message:
      "此工作区的一笔付款失败。请在账单页面支付未结发票，然后启动新会话。",
    description: "Failure callout for BILLING_BALANCE_DUE",
  },
  "agentSessions.failureSetupTitle": {
    message: "仓库设置失败",
    description:
      "Failure callout heading when a repository setup step failed (AGENT_SESSION_SETUP_FAILED)",
  },
  "agentSessions.failureSetupHint": {
    message:
      "此仓库环境中的一个设置步骤失败，因此智能体没有启动，也没有产生模型费用。请在“设置 → 智能体 → 仓库环境”中修复该步骤，然后重试。",
    description:
      "Failure callout guidance under the recorded reason (which names the step and its exit code) for AGENT_SESSION_SETUP_FAILED",
  },
  "agentSessions.failureSetupAction": {
    message: "编辑仓库环境",
    description:
      "Failure callout button opening Settings → Repository environments",
  },
  "agentSessions.costTotal": {
    message: "模型费用 {amount}",
    description: "Session header: the session's model cost (ADR091 D6)",
  },
  "agentSessions.costTotalAll": {
    message: "总费用 {amount}",
    description:
      "Session header: the session's total cost so far, model plus sandbox compute (outcome.cost.totalUSD, w9/m178)",
  },
  "agentSessions.costBreakdown": {
    message: "模型 {model} · 计算 {compute}",
    description:
      "Session header cost popover: the model and sandbox compute parts of the total",
  },
  "agentSessions.costByo": {
    message: "由你的模型提供方计费",
    description: "Session header: a BYO-key session has no bex model cost",
  },
  "agentSessions.costTitle": {
    message: "各轮次的模型费用",
    description: "Heading of the per-turn cost popover",
  },
  "agentSessions.costTurn": {
    message: "第 {turn} 轮",
    description: "One turn's row in the cost popover",
  },
  "agentSessions.costSetup": {
    message: "第一轮之前",
    description: "Requests recorded before the first turn",
  },
  "agentSessions.costTurnTokens": {
    message: "输入 {input} · 输出 {output} · 缓存 {cache} token",
    description: "A turn's token counts",
  },
  "agentSessions.costPricing": {
    message: "按模型提供方成本 + 30% 计费。",
    description: "Pricing statement in the cost popover",
  },
  "agentSessions.secretsButton": {
    message: "密钥",
    description: "Composer toolbar: the Secrets button label/tooltip (ADR098)",
  },
  "agentSessions.secretsButtonCount_other": {
    message: "密钥（已附加 {count} 个）",
    description:
      "Composer Secrets button accessible label when secrets are attached",
  },
  "agentSessions.secretsTitle": {
    message: "此会话的密钥",
    description: "Composer Secrets popover heading",
  },
  "agentSessions.secretsDescription": {
    message:
      "智能体以环境变量的形式读取每个密钥。值不会再次显示，并会从对话记录中隐去。",
    description: "Composer Secrets popover description",
  },
  "agentSessions.secretsWorkspace": {
    message: "工作区密钥",
    description:
      "Composer Secrets popover: workspace agent secrets list heading",
  },
  "agentSessions.secretsWorkspaceLoadFailed": {
    message: "无法加载工作区的智能体密钥。",
    description:
      "Composer secrets panel error when the workspace secret list fails to load",
  },
  "agentSessions.secretsWorkspaceEmpty": {
    message: "还没有工作区密钥。",
    description: "Composer Secrets popover: no workspace agent secrets",
  },
  "agentSessions.secretsManage": {
    message: "在设置中管理",
    description: "Composer Secrets popover: link to Settings \u2192 Agents",
  },
  "agentSessions.secretsInline": {
    message: "仅用于此会话",
    description: "Composer Secrets popover: inline secrets heading",
  },
  "agentSessions.secretsRemove": {
    message: "移除 {name}",
    description:
      "Composer Secrets popover: remove an inline secret (accessible label)",
  },
  "agentSessions.secretsNamePlaceholder": {
    message: "名称",
    description: "Composer Secrets popover: inline secret name placeholder",
  },
  "agentSessions.secretsNameLabel": {
    message: "密钥名称",
    description: "Composer Secrets popover: inline secret name field label",
  },
  "agentSessions.secretsValuePlaceholder": {
    message: "值",
    description: "Composer Secrets popover: inline secret value placeholder",
  },
  "agentSessions.secretsValueLabel": {
    message: "密钥值",
    description: "Composer Secrets popover: inline secret value field label",
  },
  "agentSessions.secretsAdd": {
    message: "添加",
    description: "Composer Secrets popover: add an inline secret",
  },
  "agentSessions.secretsHint": {
    message: "随会话一起删除。",
    description: "Composer Secrets popover: inline secret lifetime hint",
  },
  "agentSessions.secretsFull": {
    message: "一个会话最多可附加 20 个密钥。",
    description: "Composer Secrets popover: session secret limit reached",
  },
  "agentSessions.secretsDuplicate": {
    message: "此会话已有同名密钥。",
    description: "Composer Secrets popover: duplicate inline secret name",
  },
  "agentSessions.secretsScopeRepo": {
    message: "此仓库",
    description:
      "Composer Secrets popover: hint beside a secret scoped to the chosen repository, included",
  },
  "agentSessions.secretsScopeOtherRepo": {
    message: "仅限 {repo}",
    description:
      "Composer Secrets popover: hint beside a secret scoped to another repository, not available here",
  },
  "agentSessions.appliedSecretsTitle": {
    message: "最近一轮的密钥",
    description:
      "Session header: heading of the applied-secrets popover (names and sources, never values)",
  },
  "agentSessions.appliedSecretSource.inline": {
    message: "此会话",
    description:
      "Applied secret source: an inline secret given to this session only",
  },
  "agentSessions.appliedSecretSource.explicit": {
    message: "已选择",
    description: "Applied secret source: a workspace secret the session picked",
  },
  "agentSessions.appliedSecretSource.repo": {
    message: "仓库",
    description:
      "Applied secret source: a workspace secret scoped to the session's repository",
  },
  "agentSessions.appliedSecretSource.workspace": {
    message: "工作区",
    description:
      "Applied secret source: a workspace secret every session receives",
  },
  "agentSessions.metaSecrets_other": {
    message: "{count} 个密钥",
    description:
      "Detail header \u2014 number of attached secrets (plural, ADR098)",
  },
  "agentSessions.skillsButton": {
    message: "技能",
    description: "Composer toolbar: the Skills button label/tooltip (ADR099)",
  },
  "agentSessions.skillsButtonCount_other": {
    message: "技能（已选 {count} 个）",
    description:
      "Composer Skills button accessible label when skills are chosen",
  },
  "agentSessions.skillsTitle": {
    message: "本轮使用的技能",
    description: "Composer Skills popover heading",
  },
  "agentSessions.skillsDescription": {
    message: "智能体会执行所选的每个技能；已启用的技能它也可能自行使用。",
    description: "Composer Skills popover description",
  },
  "agentSessions.skillsLoadFailed": {
    message: "无法加载工作区技能。",
    description: "Composer Skills popover: list failed",
  },
  "agentSessions.skillsEmpty": {
    message: "此工作区还没有技能。",
    description: "Composer Skills popover: none exist",
  },
  "agentSessions.skillsManage": {
    message: "管理技能",
    description: "Composer Skills popover link to Settings → Agents",
  },
  "agentSessions.skillsArgumentsLabel": {
    message: "{name} 的参数",
    description: "Composer Skills popover: argument field label",
  },
  "agentSessions.skillsArgumentsPlaceholder": {
    message: "参数（可选）",
    description: "Composer Skills popover: argument field placeholder",
  },
  "agentSessions.skillsArgumentsInvalid": {
    message: "参数须为一行，最多 4 KiB。",
    description: "Composer Skills popover: invalid arguments",
  },
  "agentSessions.skillsFull": {
    message: "每轮最多使用 5 个技能。",
    description: "Composer Skills popover: limit reached",
  },
  "agentSessions.skillsHint": {
    message: "参数会填入技能中的 $ARGUMENTS。",
    description: "Composer Skills popover hint",
  },
  "agentSessions.skillsRemove": {
    message: "移除 {name}",
    description: "Accessible label of a chosen skill chip's remove button",
  },
  "agentSessions.turnSkills": {
    message: "本轮使用的技能",
    description: "Accessible label of a transcript turn's skill pills",
  },
  "agentSessions.turnOrigins": {
    message: "引导本轮的来源",
    description:
      "Accessible label of the lines above a transcript turn that PR feedback started (w9/m344)",
  },
  "agentSessions.turnOriginReview": {
    message: "由 @{author} 的评审引导",
    description:
      "A turn started by a pull-request review; {author} is the reviewer's GitHub login. Links to the review",
  },
  "agentSessions.turnOriginReviewAnonymous": {
    message: "由一次 PR 评审引导",
    description:
      "A turn started by a pull-request review whose author is unknown. Links to the review",
  },
  "agentSessions.turnOriginComment": {
    message: "由 @{author} 的 PR 评论引导",
    description:
      "A turn started by a pull-request comment; {author} is the commenter's GitHub login. Links to the comment",
  },
  "agentSessions.turnOriginCommentAnonymous": {
    message: "由一条 PR 评论引导",
    description:
      "A turn started by a pull-request comment whose author is unknown. Links to the comment",
  },
  "agentSessions.turnOriginCiFailure": {
    message: "由一次失败的 CI 运行引导",
    description:
      "A turn started by a failed CI run on the pull request. Links to the failed checks",
  },
  "agentSessions.turnOriginCiFailureOnBase": {
    message:
      "由一次失败的 CI 运行引导；其中有检查在 {base} 上同样失败，并非本次改动导致",
    description:
      "A turn started by a failed CI run on the pull request where at least one failing check also fails on the base branch ({base}, a branch name), so the pull request did not cause it. Links to the failed checks",
  },
  "agentSessions.turnOriginsStart": {
    message: "发起本会话的来源",
    description:
      "Accessible label of the line above a session's first turn when a /bex comment on a pull request started it (w9/m348)",
  },
  "agentSessions.turnOriginStartComment": {
    message: "由 @{author} 的评论发起",
    description:
      "The first turn of a session a /bex pull-request comment started; {author} is the commenter's GitHub login. Links to the comment",
  },
  "agentSessions.turnOriginStartCommentAnonymous": {
    message: "由一条 PR 评论发起",
    description:
      "The first turn of a session a /bex pull-request comment started, when the commenter is unknown. Links to the comment",
  },
  "agentSessions.lineage.childSessions": {
    message: "子会话",
    description:
      "Heading of the block listing sessions the agent spawned, in the parent session's transcript",
  },
  "agentSessions.lineage.childCardTitle": {
    message: "已创建会话",
    description:
      "Small label above a child session's title on its card in the parent's transcript",
  },
  "agentSessions.lineage.agentsPanel": {
    message: "智能体",
    description:
      "Heading of the session page's right-hand panel listing the sessions below this one",
  },
  "agentSessions.lineage.viewSession": {
    message: "查看会话",
    description: "Link on an Agents panel row that opens that child session",
  },
  "agentSessions.lineage.parentSession": {
    message: "父会话",
    description:
      "Link in the Agents panel of a child session that opens its parent",
  },
  "agentSessions.lineage.collapsePanel": {
    message: "收起智能体面板",
    description:
      "Accessible label of the button that collapses the Agents panel",
  },
  "agentSessions.lineage.expandPanel": {
    message: "展开智能体面板",
    description: "Accessible label of the button that expands the Agents panel",
  },
  "agentSessions.lineage.collapseChildren": {
    message: "收起子会话",
    description:
      "Accessible label of a sidebar row's button that collapses its nested child sessions",
  },
  "agentSessions.lineage.expandChildren": {
    message: "展开子会话",
    description:
      "Accessible label of a sidebar row's button that expands its nested child sessions",
  },
  "agentSessions.lineage.status.working": {
    message: "进行中",
    description:
      "Status group and badge of a child session that is still running",
  },
  "agentSessions.lineage.status.done": {
    message: "已完成",
    description:
      "Status group and badge of a child session that finished successfully",
  },
  "agentSessions.lineage.status.failed": {
    message: "失败",
    description: "Status group and badge of a child session that failed",
  },
  "agentSessions.lineage.status.canceled": {
    message: "已取消",
    description: "Status group and badge of a child session that was canceled",
  },
  "agentSessions.sidebarMine": {
    message: "我的",
    description:
      "Sessions sidebar — show only the sessions you started (Devin's default)",
  },
  "agentSessions.sidebarEveryone": {
    message: "所有人",
    description: "Sessions sidebar — show every member's sessions",
  },
  "agentSessions.sidebarOwnerFilter": {
    message: "会话范围",
    description:
      "Sessions sidebar — accessible name of the Mine/Everyone toggle",
  },
  "agentSessions.sidebarEmptyMine": {
    message: "没有你启动的会话。较早的会话在“所有人”中。",
    description:
      "Sessions sidebar — empty state under Mine. Sessions from before 2026-11-08 record no starter, so they appear only under Everyone",
  },
  "agentSessions.startedByPerson": {
    message: "由 {name} 启动",
    description: "Session header and list rows — who started the session",
  },
  "agentSessions.startedForPerson": {
    message: "代表 {name}",
    description:
      "Session header — after 'Started by automation X': the member the automation runs as",
  },
  "agentSessions.editTags": {
    message: "编辑标签",
    description: "Session header menu — open the tag editor",
  },
  "agentSessions.editTagsTitle": {
    message: "编辑标签",
    description: "Tag editor dialog title",
  },
  "agentSessions.editTagsDescription": {
    message: "新标签会替换会话当前的全部标签。用逗号或空格分隔；留空则清除。",
    description: "Tag editor dialog description",
  },
  "agentSessions.editTagsHint": {
    message: "最多 20 个标签：小写字母、数字、冒号、下划线或连字符。",
    description: "Tag editor dialog — the tag rules",
  },
  "agentSessions.editTagsSave": {
    message: "保存标签",
    description: "Tag editor dialog — save button",
  },
  "agentSessions.editTagsSaved": {
    message: "标签已保存",
    description: "Toast after the tags were replaced",
  },
};

export default zhAgentSessions;
