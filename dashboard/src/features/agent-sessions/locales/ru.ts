import type { TranslationEntry } from "@/i18n";

const ruAgentSessions: Record<string, TranslationEntry> = {
  "agentSessions.pageTitle": {
    message: "Сессии агентов",
    description: "Agent sessions list page heading and document title",
  },
  "agentSessions.listTitle": {
    message: "Недавние",
    description: "Agent sessions recents heading — matches the rail",
  },
  "agentSessions.colTask": {
    message: "Задача",
    description: "Agent sessions table column header — the session's prompt",
  },
  "agentSessions.colRepo": {
    message: "Репозиторий",
    description: "Agent sessions table column header — repo/branch",
  },
  "agentSessions.colAgent": {
    message: "Агент",
    description: "Agent sessions table column header — the driver agent",
  },
  "agentSessions.colPhase": {
    message: "Фаза",
    description: "Agent sessions table column header — lifecycle phase chip",
  },
  "agentSessions.colPr": {
    message: "PR",
    description: "Agent sessions table column header — draft pull request",
  },
  "agentSessions.colCreated": {
    message: "Создана",
    description: "Agent sessions table column header — createdAt relative age",
  },
  "agentSessions.prBadge": {
    message: "#{number}",
    description: "Draft PR badge label — the pull-request number",
  },
  "agentSessions.prState.open": {
    message: "Открыт",
    description:
      "PR state badge beside a session's PR link — the pull request is open (a draft counts as open)",
  },
  "agentSessions.prState.merged": {
    message: "Слит",
    description:
      "PR state badge beside a session's PR link — the pull request was merged",
  },
  "agentSessions.prState.closed": {
    message: "Закрыт",
    description:
      "PR state badge beside a session's PR link — the pull request was closed without merging",
  },
  "agentSessions.prAutoMerge": {
    message: "Автослияние",
    description:
      "Badge beside a session's PR state — GitHub auto-merge is armed on the pull request (w10/047)",
  },
  "agentSessions.prMergeQueued": {
    message: "В очереди слияния",
    description:
      "Badge beside a session's PR state — the pull request is in its base branch's merge queue (w10/047)",
  },
  "agentSessions.prMergeQueuedHint": {
    message: "В очереди слияния: GitHub сольёт его, когда подойдёт очередь.",
    description:
      "Tooltip of the merge-queue badge — what being in the merge queue means",
  },
  "agentSessions.prAutoMergeEnabledBy": {
    message: "Автослияние включил {login}",
    description:
      "Tooltip of the auto-merge badge — the GitHub login that armed auto-merge",
  },
  "agentSessions.prAutoMergeEnabled": {
    message: "Автослияние включено",
    description:
      "Tooltip of the auto-merge badge when GitHub does not say who armed it",
  },
  "agentSessions.prAutoMergeHint": {
    message: "GitHub сольёт его, когда пройдут обязательные ревью и проверки.",
    description:
      "Tooltip of the auto-merge badge — what auto-merge does",
  },
  "agentSessions.prAutoMergeMethod.merge": {
    message: "Создать коммит слияния",
    description:
      "Tooltip of the auto-merge badge — the merge method, GitHub's 'Create a merge commit'",
  },
  "agentSessions.prAutoMergeMethod.squash": {
    message: "Squash и слияние",
    description:
      "Tooltip of the auto-merge badge — the merge method, GitHub's 'Squash and merge'",
  },
  "agentSessions.prAutoMergeMethod.rebase": {
    message: "Rebase и слияние",
    description:
      "Tooltip of the auto-merge badge — the merge method, GitHub's 'Rebase and merge'",
  },
  "agentSessions.prStateFilter": {
    message: "Фильтр по состоянию pull request",
    description:
      "Session list — accessible name of the Any/Open/Merged/Closed PR-state filter",
  },
  "agentSessions.prStateFilterAny": {
    message: "Любое",
    description:
      "Session list PR-state filter — the option that shows sessions whatever their PR state, including sessions with no PR",
  },
  "agentSessions.errorTitle": {
    message: "Не удалось загрузить сессии.",
    description: "Agent sessions list error — one sentence, Retry beside it",
  },
  "agentSessions.retry": {
    message: "Повторить",
    description: "Retries a failed agent-sessions list request",
  },
  "agentSessions.emptyBody": {
    message: "Запущенные вами сессии появятся здесь.",
    description: "Default empty — one quiet line under the composer",
  },
  "agentSessions.emptyArchivedTitle": {
    message: "Нет архивных сессий",
    description: "Archived-only list empty-state heading",
  },
  "agentSessions.emptyArchivedBody": {
    message: "Сессии, которые вы архивируете, появятся здесь.",
    description: "Archived-only list empty-state body",
  },
  "agentSessions.emptyFilteredTitle": {
    message: "Нет подходящих сессий",
    description: "Phase-filtered list empty-state heading",
  },
  "agentSessions.emptyFilteredBody": {
    message: "Попробуйте другую фазу или сбросьте фильтры.",
    description: "Phase-filtered list empty-state body",
  },
  "agentSessions.clearFilters": {
    message: "Сбросить фильтры",
    description: "Clears agent-session membership and phase filters",
  },
  "agentSessions.phase.creating": {
    message: "Создаётся",
    description: "Agent session phase chip — sandbox is being created",
  },
  "agentSessions.phase.running": {
    message: "Выполняется",
    description: "Agent session phase chip — a turn is in progress",
  },
  "agentSessions.phase.resuming": {
    message: "Возобновляется",
    description: "Agent session phase chip — session is resuming",
  },
  "agentSessions.phase.redispatching": {
    message: "Повторная отправка",
    description: "Agent session phase chip — session is being redispatched",
  },
  "agentSessions.phase.completed": {
    message: "Завершена",
    description: "Agent session phase chip — session finished successfully",
  },
  "agentSessions.phase.failed": {
    message: "Сбой",
    description: "Agent session phase chip — session failed",
  },
  "agentSessions.phase.canceling": {
    message: "Отменяется",
    description: "Agent session phase chip — cancellation in progress",
  },
  "agentSessions.phase.canceled": {
    message: "Отменена",
    description: "Agent session phase chip — session was canceled",
  },
  "agentSessions.taskLabel": {
    message: "Задача",
    description: "Composer — the prominent task prompt textarea label",
  },
  "agentSessions.taskPlaceholder": {
    message: "Опишите задачу по программированию…",
    description: "Composer — task editor placeholder",
  },
  "agentSessions.mentionButton": {
    message: "Добавить репозиторий или сессию",
    description:
      "Composer toolbar — opens the repository/session mention picker",
  },
  "agentSessions.repoChip": {
    message: "Репозиторий {repo}",
    description: "Composer toolbar — accessible name of the selected-repo chip",
  },
  "agentSessions.keyboardHint": {
    message: "Enter — запустить · Shift+Enter — новая строка",
    description: "Muted hint under the composer",
  },
  "agentSessions.connectGitHubTitle": {
    message: "Подключите GitHub, чтобы начать",
    description: "Composer callout when the workspace has no App repos",
  },
  "agentSessions.connectGitHubBody": {
    message:
      "Облачным агентам нужен подключённый репозиторий GitHub. Подключите GitHub App для этого рабочего пространства, затем вернитесь, чтобы запустить сессию.",
    description: "Composer callout body when there are no installation repos",
  },
  "agentSessions.connectGitHubSettings": {
    message: "Настройки GitHub",
    description:
      "Secondary link from the composer's GitHub callout to Account Settings → Integrations (/settings#integrations), where the GitHub card lives",
  },
  "agentSessions.exampleFixTests": {
    message: "Исправь падающие тесты и открой черновой PR",
    description: "First-run example prompt chip",
  },
  "agentSessions.exampleAddReadme": {
    message: "Добавь README с описанием запуска проекта",
    description: "First-run example prompt chip",
  },
  "agentSessions.chipRemove": {
    message: "Удалить {name}",
    description: "Accessible label of a mention chip's remove button",
  },
  "agentSessions.mentionCategoryRepos": {
    message: "Репозитории",
    description: "Mention picker — the repositories category row",
  },
  "agentSessions.mentionCategoryReposDesc": {
    message: "Привязать сессию к подключённому репозиторию GitHub",
    description: "Mention picker — repositories category one-line description",
  },
  "agentSessions.mentionCategorySessions": {
    message: "Сессии",
    description: "Mention picker — the prior-sessions category row",
  },
  "agentSessions.mentionCategorySessionsDesc": {
    message: "Сослаться на предыдущую сессию агента как на контекст",
    description: "Mention picker — sessions category one-line description",
  },
  "agentSessions.mentionNoResults": {
    message: "Совпадений нет",
    description:
      "Mention picker — empty state when the fuzzy filter matches nothing",
  },
  "agentSessions.mentionReposEmpty": {
    message:
      "Репозиториев пока нет. Подключите аккаунт GitHub или откройте приложению bex GitHub App доступ к репозиторию.",
    description:
      "Mention picker — empty state when no installation repos exist",
  },
  "agentSessions.mentionSessionsEmpty": {
    message: "Сессий пока нет",
    description:
      "Mention picker — empty state when the workspace has no sessions",
  },
  "agentSessions.mentionConnected": {
    message: "Подключён через GitHub App",
    description: "Mention picker readiness footer — the repo is app-connected",
  },
  "agentSessions.mentionDefaultBranch": {
    message: "Ветка по умолчанию: {branch}",
    description: "Mention picker readiness footer — the repo's default branch",
  },
  "agentSessions.agentLabel": {
    message: "Агент",
    description: "Composer — agent select label",
  },
  "agentSessions.modelLabel": {
    message: "Модель",
    description: "Composer — model select label (w10/m516)",
  },
  "agentSessions.modelDefaultOption": {
    message: "{model} (по умолчанию)",
    description: "Model list — the model a session runs when it names none",
  },
  "agentSessions.modelFixed": {
    message: "Этот агент всегда использует {model}",
    description: "Tooltip of the model of an agent with one fixed model",
  },
  "agentSessions.metaModel": {
    message: "Модель: {model}",
    description: "Session header — tooltip of the model the session runs",
  },
  "agentSessions.sandboxLabel": {
    message: "Песочница",
    description: "Composer — sandbox template select label (w10/m519)",
  },
  "agentSessions.sandboxSize": {
    message: "{cpu} CPU, {memory} ГБ памяти",
    description:
      "Sandbox template size: CPU cores and memory in GB, numbers already formatted",
  },
  "agentSessions.sandboxOption": {
    message: "{name} · {size}",
    description: "Sandbox template choice: its name, then its size",
  },
  "agentSessions.sandboxDefaultOption": {
    message: "{template} (по умолчанию)",
    description:
      "Sandbox list — the template a session runs on when it names none (the workspace default)",
  },
  "agentSessions.sandboxWorkspaceDefault": {
    message: "По умолчанию для рабочего пространства ({template})",
    description:
      "Automation sandbox choice — follow the workspace default at each run; {template} is the default now",
  },
  "agentSessions.metaSandbox": {
    message: "Песочница: {template}",
    description:
      "Tooltip of the sandbox template a session runs on, with its size",
  },
  "agentSessions.githubCliInherit": {
    message: "Наследовать",
    description:
      "GitHub CLI choice — inherit the level above, while what that resolves to is still loading (w10/m561)",
  },
  "agentSessions.githubCliInheritOn": {
    message: "Наследовать (вкл.)",
    description:
      "GitHub CLI choice — inherit the level above, which has gh on (w10/m561)",
  },
  "agentSessions.githubCliInheritOff": {
    message: "Наследовать (выкл.)",
    description:
      "GitHub CLI choice — inherit the level above, which has gh off (w10/m561)",
  },
  "agentSessions.githubCliOn": {
    message: "Вкл.",
    description:
      "GitHub CLI choice — gh on for this session or automation (w10/m561)",
  },
  "agentSessions.githubCliOff": {
    message: "Выкл.",
    description:
      "GitHub CLI choice — gh off for this session or automation (w10/m561)",
  },
  "agentSessions.githubCliOnMandatory": {
    message:
      "Недоступно, пока выключение GitHub CLI обязательно в этом рабочем пространстве.",
    description:
      "Why the GitHub CLI On choice is disabled: the workspace's off is mandatory (w10/m561)",
  },
  "agentSessions.githubCliAdminOnly": {
    message: "Изменить это может только администратор рабочего пространства.",
    description:
      "Why a GitHub CLI choice is disabled: only a workspace admin chooses or changes a session's or an automation's gh (w10/044)",
  },
  "agentSessions.githubCliTitle": {
    message: "GitHub CLI для этой сессии: {choice}",
    description:
      "Tooltip of the composer's GitHub CLI control; {choice} is the chosen binding, e.g. Inherit (on) (w10/m561)",
  },
  "agentSessions.errors.AGENT_SESSION_GITHUB_CLI_LOCKED": {
    message:
      "В этом рабочем пространстве выключение GitHub CLI обязательно, поэтому сессия не может включить gh. Выберите «Выкл.» или «Наследовать».",
    description:
      "Mapped message for AGENT_SESSION_GITHUB_CLI_LOCKED with reason mandatory (w10/m561)",
  },
  "agentSessions.errors.AGENT_SESSION_GITHUB_CLI_LOCKED_adminRequired": {
    message:
      "Выбрать GitHub CLI для сессии может только администратор рабочего пространства. Выберите «Наследовать».",
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
    message: "Запустить сессию",
    description: "Composer — submit button label",
  },
  "agentSessions.submitting": {
    message: "Запуск…",
    description: "Composer — submit button label while the create is in flight",
  },
  "agentSessions.unavailableTitle": {
    message: "Сессии агентов не настроены",
    description: "Composer — house callout title when the feature is 503",
  },
  "agentSessions.unavailableBody": {
    message:
      "На этой платформе не включены облачные сессии агентов. Попросите оператора настроить шлюз сессий агентов.",
    description: "Composer — house callout body when the feature is 503",
  },
  "agentSessions.dependencyUnavailableTitle": {
    message: "Сессии агентов временно недоступны",
    description:
      "Composer — retryable dependency-outage title, distinct from configuration",
  },
  "agentSessions.dependencyUnavailableBody": {
    message:
      "У одной из зависимостей платформы сейчас неполадки. Ваши настройки не затронуты; подождите немного и повторите попытку.",
    description:
      "Composer — retryable dependency-outage body that does not send the user to an operator",
  },
  "agentSessions.modelKeyMissingTitle": {
    message: "Сначала добавьте ключ поставщика модели",
    description:
      "Composer pre-flight title when the workspace has no model key",
  },
  "agentSessions.modelKeyMissingBody": {
    message:
      "У этого рабочего пространства пока нет доступа к модели. Администратор рабочего пространства может добавить собственный ключ поставщика модели в разделе «Настройки → Агенты», после чего можно вернуться сюда.",
    description:
      "Composer pre-flight body when the workspace has neither its own model key nor the bex-provided model (ADR091 D7)",
  },
  "agentSessions.modelKeyMissingAction": {
    message: "Открыть «Настройки → Агенты»",
    description:
      "Composer pre-flight link to the Settings → Agents model key field (/workspace/settings#agents)",
  },
  "agentSessions.createErrorTitle": {
    message: "Не удалось запустить сессию",
    description: "Composer — error alert title when the create fails",
  },
  "agentSessions.detailTitle": {
    message: "Сессия",
    description: "Agent-session detail page document title",
  },
  "agentSessions.untitled": {
    message: "Сессия без названия",
    description:
      "Last-resort session name when a session carries neither a repository nor a task prompt",
  },
  "agentSessions.backToList": {
    message: "Все сессии",
    description: "Detail page — back link to the /agents list",
  },
  "agentSessions.detailErrorTitle": {
    message: "Не удалось загрузить эту сессию",
    description: "Detail page — heading when the session read fails",
  },
  "agentSessions.conversationTitle": {
    message: "Диалог",
    description: "Detail page — the live conversation column card title",
  },
  "agentSessions.failureTitle": {
    message: "Сбой сессии",
    description: "Detail page — heading over a failed session's reason",
  },
  "agentSessions.deliveryNoteTitle": {
    message: "Изменения доставлены не полностью",
    description:
      "Detail page — heading over a completed session's note that the agent left work uncommitted, unpushed or without a pull request",
  },
  "agentSessions.deliveryNoteHint": {
    message:
      "Отправьте дополнительное сообщение, чтобы агент завершил коммит, отправку изменений и обновление pull request.",
    description:
      "Detail page — the next step shown under an undelivered-work note",
  },
  "agentSessions.failureReasonFallback": {
    message: "Не удалось запустить сессию.",
    description:
      "Detail page — generic failure text when the session carries no specific reason",
  },
  "agentSessions.failureRetry": {
    message: "Повторить",
    description:
      "Detail page — button to re-run a failed session's original task",
  },
  "agentSessions.failureRetrying": {
    message: "Повторный запуск…",
    description:
      "Detail page — retry button label while the redispatch is in flight",
  },
  "agentSessions.capacityFailureTitle": {
    message: "Не хватает ёмкости песочниц",
    description:
      "Detail page — heading when a session failed on the plan's sandbox limit",
  },
  "agentSessions.capacityFailureBody": {
    message:
      "Это рабочее пространство достигло лимита своего тарифа на одновременные песочницы. Перейдите на другой тариф, чтобы запускать больше одновременно, или остановите простаивающую сессию и повторите попытку.",
    description:
      "Detail page — explains a plan-limit sandbox failure and the two remedies",
  },
  "agentSessions.upgradePlan": {
    message: "Повысить тариф",
    description:
      "Detail page — CTA on a capacity failure; opens the change-plan dialog",
  },
  "agentSessions.failureCause.pool_full": {
    message: "Ни на одной машине для песочниц не нашлось места",
    description:
      "Failed-start cause line (session callout, automation runs) — no sandbox machine had room for the session's machine for the whole wait (w10/m497)",
  },
  "agentSessions.failureCause.image_pull": {
    message: "Не удалось загрузить образ песочницы",
    description:
      "Failed-start cause line — the sandbox's container image did not pull (w10/m497)",
  },
  "agentSessions.failureCause.runtime": {
    message: "Среда выполнения песочницы не смогла запуститься",
    description:
      "Failed-start cause line — the sandbox runtime did not start the session's machine (w10/m497)",
  },
  "agentSessions.failureCause.quota": {
    message: "Квота песочниц рабочего пространства не позволила запуск",
    description:
      "Failed-start cause line — the workspace's sandbox quota refused the session's machine (w10/m497)",
  },
  "agentSessions.failureCause.unknown": {
    message: "Проблема платформы",
    description:
      "Failed-start cause line — the sandbox did not start for a cause on bex's side that bex-api could not name (w10/m497)",
  },
  "agentSessions.provisioning": {
    message: "Запуск песочницы…",
    description:
      "Detail page — shown while a new/steered session provisions its sandbox before the conversation stream is available",
  },
  "agentSessions.metaDuration": {
    message: "Длительность {duration}",
    description: "Detail header — elapsed session wall-clock",
  },
  "agentSessions.metaTurns_one": {
    message: "{count} ход",
    description: "Detail header — number of prompt turns taken (singular)",
  },
  "agentSessions.metaTurns_few": {
    message: "{count} хода",
    description: "Detail header — number of prompt turns taken (plural)",
  },
  "agentSessions.metaTurns_many": {
    message: "{count} ходов",
    description: "Detail header — number of prompt turns taken (plural)",
  },
  "agentSessions.metaTurns_other": {
    message: "{count} хода",
    description: "Detail header — number of prompt turns taken (plural)",
  },
  "agentSessions.cancel": {
    message: "Отменить",
    description: "Detail header — cancel-session button label",
  },
  "agentSessions.cancelDisabledCanceling": {
    message: "Отмена уже выполняется.",
    description: "Detail header — tooltip on the disabled cancel button",
  },
  "agentSessions.canceling": {
    message: "Отмена…",
    description: "Detail header — confirm button label while the cancel runs",
  },
  "agentSessions.cancelSuccess": {
    message: "Сессия отменяется…",
    description: "Detail header — toast after the cancel is accepted",
  },
  "agentSessions.cancelConfirmTitle": {
    message: "Отменить эту сессию?",
    description: "Cancel confirm dialog title",
  },
  "agentSessions.cancelConfirmBody": {
    message:
      "Агент будет остановлен. Коммиты, уже отправленные в ветку, и PR сохранятся.",
    description: "Cancel confirm dialog body — states pushed work is preserved",
  },
  "agentSessions.cancelConfirmDismiss": {
    message: "Продолжить работу",
    description: "Cancel confirm dialog — dismiss button",
  },
  "agentSessions.cancelConfirmProceed": {
    message: "Отменить сессию",
    description: "Cancel confirm dialog — proceed button",
  },
  "agentSessions.stopTurn": {
    message: "Остановить ход",
    description:
      "Detail header — Stop turn button label: ends only the running turn, unlike Cancel which ends the session",
  },
  "agentSessions.stoppingTurn": {
    message: "Остановка…",
    description: "Detail header — Stop turn button label while the stop runs",
  },
  "agentSessions.stopTurnHint": {
    message:
      "Завершает только выполняющийся ход. Сессия остаётся открытой, а сообщения в очереди сохраняют свои места. Сочетание клавиш: Ctrl/Cmd+Shift+Backspace.",
    description: "Detail header — tooltip on the Stop turn button",
  },
  "agentSessions.stopTurnStarting": {
    message:
      "Ход ещё запускается. Остановить его можно после начала выполнения.",
    description:
      "Detail header — tooltip on the disabled Stop turn button while the sandbox starts",
  },
  "agentSessions.stopTurnSuccess": {
    message: "Ход останавливается. Он завершится через несколько секунд.",
    description: "Detail header — toast after Stop turn is accepted",
  },
  "agentSessions.steerTitle": {
    message: "Скорректировать сессию",
    description: "Steering composer card title",
  },
  "agentSessions.steerErrorTitle": {
    message: "Не удалось отправить",
    description: "Steering composer — inline error alert title",
  },
  "agentSessions.steerPlaceholderIdle": {
    message:
      "Отправьте дополнительную инструкцию, чтобы начать новый ход в той же ветке.",
    description: "Steering composer — textarea placeholder for an idle session",
  },
  "agentSessions.steerPlaceholderQueue": {
    message:
      "Поставьте сообщение в очередь. Оно выполнится после завершения текущего хода.",
    description:
      "Steering composer — textarea placeholder while a turn runs or messages wait (ADR047 D12)",
  },
  "agentSessions.steerHintIdle": {
    message:
      "Сессия простаивает — отправка сообщения продолжит диалог в новой песочнице в той же ветке.",
    description: "Steering composer — helper text for the redispatch route",
  },
  "agentSessions.steerHintQueue": {
    message:
      "Агент работает. Ваше сообщение поставлено в очередь и выполнится отдельным ходом после завершения текущего.",
    description:
      "Steering composer — helper text while a turn runs or messages wait (ADR047 D12)",
  },
  "agentSessions.steerDisabledCanceling": {
    message: "Эта сессия отменяется. Уже отправленные изменения сохранятся.",
    description: "Steering composer — disabled reason while canceling",
  },
  "agentSessions.steerDisabledCanceled": {
    message: "Эта сессия отменена.",
    description: "Steering composer — disabled reason once canceled",
  },
  "agentSessions.steerDisabledStream": {
    message:
      "Поток диалога недоступен, поэтому корректировка в реальном времени приостановлена.",
    description:
      "Steering composer — disabled reason when the m43 stream is down",
  },
  "agentSessions.steerSubmit": {
    message: "Отправить",
    description: "Steering composer — submit button label",
  },
  "agentSessions.steerSending": {
    message: "Отправка…",
    description: "Steering composer — submit button label while sending",
  },
  "agentSessions.steerSuccess": {
    message: "Отправлено — запускается новый ход.",
    description: "Steering composer — toast after a redispatch is accepted",
  },
  "agentSessions.steerQueued": {
    message: "Поставлено в очередь. Выполнится после завершения текущего хода.",
    description:
      "Steering composer — toast after a message was queued behind a running turn (ADR047 D12)",
  },
  "agentSessions.queueRegion": {
    message: "Сообщения в очереди",
    description: "Queued messages list — accessible name of the region",
  },
  "agentSessions.queueTitle_one": {
    message: "{count} сообщение в очереди",
    description: "Queued messages list — heading with the count (singular)",
  },
  "agentSessions.queueTitle_few": {
    message: "{count} сообщения в очереди",
    description: "Queued messages list — heading with the count (plural)",
  },
  "agentSessions.queueTitle_many": {
    message: "{count} сообщений в очереди",
    description: "Queued messages list — heading with the count (plural)",
  },
  "agentSessions.queueTitle_other": {
    message: "{count} сообщения в очереди",
    description: "Queued messages list — heading with the count (plural)",
  },
  "agentSessions.queueHint": {
    message: "Они выполняются по одному, в этом порядке, после текущего хода.",
    description:
      "Queued messages list — one-line explanation next to the heading",
  },
  "agentSessions.queuePosition": {
    message: "Место {position} в очереди",
    description: "Queued messages list — tooltip on a row's place number",
  },
  "agentSessions.queueAttachments_one": {
    message: "{count} вложение",
    description:
      "Queued message row — attachments that ride with it (singular)",
  },
  "agentSessions.queueAttachments_few": {
    message: "{count} вложения",
    description: "Queued message row — attachments that ride with it (plural)",
  },
  "agentSessions.queueAttachments_many": {
    message: "{count} вложений",
    description: "Queued message row — attachments that ride with it (plural)",
  },
  "agentSessions.queueAttachments_other": {
    message: "{count} вложения",
    description: "Queued message row — attachments that ride with it (plural)",
  },
  "agentSessions.queueBlocked": {
    message:
      "Пока не удалось запустить: {reason}. Попытка будет повторена автоматически.",
    description:
      "Queued message row — why the dispatcher could not start it (a spend ceiling or the live-sandbox limit)",
  },
  "agentSessions.queueModel": {
    message: "Переключает сессию на {model}",
    description:
      "Queued message — the model its turn switches the session to (w10/m516)",
  },
  "agentSessions.queueSendNow": {
    message: "Отправить сейчас",
    description:
      "Queued message row — stops the running turn and runs this message next",
  },
  "agentSessions.queueMoveUp": {
    message: "Поднять выше",
    description: "Queued message row — move one place earlier",
  },
  "agentSessions.queueMoveDown": {
    message: "Опустить ниже",
    description: "Queued message row — move one place later",
  },
  "agentSessions.queueEdit": {
    message: "Изменить",
    description: "Queued message row — edit the message in place",
  },
  "agentSessions.queueRemove": {
    message: "Удалить",
    description: "Queued message row — remove it so it never runs",
  },
  "agentSessions.queueEditLabel": {
    message: "Изменить сообщение в очереди",
    description: "Queued message row — accessible name of the edit box",
  },
  "agentSessions.queueEditSave": {
    message: "Сохранить",
    description: "Queued message row — save the edit",
  },
  "agentSessions.queueEditSaveSendNow": {
    message: "Сохранить и отправить сейчас",
    description:
      "Queued message row — save the edit, stop the running turn and run this message next",
  },
  "agentSessions.queueEditDiscard": {
    message: "Отменить изменения",
    description: "Queued message row — leave the edit without saving",
  },
  "agentSessions.queueEditHint": {
    message:
      "Enter сохраняет, Shift+Enter добавляет строку, Ctrl/Cmd+Enter сохраняет и сразу отправляет, Esc отменяет изменения.",
    description: "Queued message row — keyboard hints while editing",
  },
  "agentSessions.queueSendNowStopping": {
    message: "Текущий ход останавливается. Это сообщение выполнится следующим.",
    description: "Queued message row — toast after Send now stopped a turn",
  },
  "agentSessions.queueSendNowNext": {
    message: "Это сообщение выполнится следующим.",
    description:
      "Queued message row — toast after Send now when no turn was running",
  },
  "agentSessions.errors.AGENT_SESSION_INPUT_INVALID": {
    message:
      "Эти данные недопустимы. Проверьте репозиторий, ветку и задачу и повторите попытку.",
    description: "Mapped message for AGENT_SESSION_INPUT_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_ID_INVALID": {
    message: "Этот идентификатор сессии недопустим.",
    description: "Mapped message for AGENT_SESSION_ID_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_FOUND": {
    message: "Этой сессии больше нет.",
    description: "Mapped message for AGENT_SESSION_NOT_FOUND",
  },
  "agentSessions.errors.AGENT_SESSION_CONFLICT": {
    message: "Сессия занята другой операцией. Повторите попытку чуть позже.",
    description: "Mapped message for AGENT_SESSION_CONFLICT",
  },
  "agentSessions.errors.AGENT_SESSION_BRANCH_IN_USE": {
    message:
      "Другая сессия уже работает в этой ветке. Отправьте сообщение той сессии или используйте другую ветку bex-agent/.",
    description: "Mapped message for AGENT_SESSION_BRANCH_IN_USE (w5/m382)",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_STEERABLE": {
    message: "Эту сессию нельзя скорректировать в её текущей фазе ({phase}).",
    description: "Mapped message for AGENT_SESSION_NOT_STEERABLE",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_RESUMABLE": {
    message: "Эту сессию нельзя возобновить в её текущей фазе ({phase}).",
    description: "Mapped message for AGENT_SESSION_NOT_RESUMABLE",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_ATTACHABLE": {
    message: "К этой сессии нельзя подключиться в её текущей фазе ({phase}).",
    description: "Mapped message for AGENT_SESSION_NOT_ATTACHABLE",
  },
  "agentSessions.errors.AGENT_SESSION_TURN_IN_FLIGHT": {
    message:
      "Ход уже выполняется. Дождитесь его завершения, прежде чем отправлять следующее сообщение.",
    description: "Mapped message for AGENT_SESSION_TURN_IN_FLIGHT",
  },
  "agentSessions.errors.AGENT_SESSION_QUEUE_FULL": {
    message:
      "В очереди этой сессии уже {max} сообщений. Дождитесь выполнения одного из них или удалите одно, затем отправьте снова.",
    description: "Mapped message for AGENT_SESSION_QUEUE_FULL (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_QUEUE_INLINE_SECRETS": {
    message:
      "Сообщение со встроенными секретами нельзя поставить в очередь, потому что их значения нигде не хранятся. Отправьте его, когда сессия простаивает, или используйте секрет рабочего пространства.",
    description:
      "Mapped message for AGENT_SESSION_QUEUE_INLINE_SECRETS (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_QUEUED_MESSAGE_DISPATCHED": {
    message:
      "Это сообщение уже начало выполняться, поэтому его больше нельзя изменить.",
    description:
      "Mapped message for AGENT_QUEUED_MESSAGE_DISPATCHED (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_QUEUED_MESSAGE_NOT_FOUND": {
    message: "Этого сообщения больше нет в очереди.",
    description:
      "Mapped message for AGENT_QUEUED_MESSAGE_NOT_FOUND (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_QUEUED_MESSAGE_ATTACHMENTS_OWNED": {
    message:
      "К этому сообщению приложены вложения отправителя, поэтому изменить его может только он. Вы по-прежнему можете переместить или удалить его.",
    description:
      "Mapped message for AGENT_QUEUED_MESSAGE_ATTACHMENTS_OWNED (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_NO_TURN_RUNNING": {
    message: "Ни один ход не выполняется, поэтому останавливать нечего.",
    description:
      "Mapped message for AGENT_SESSION_NO_TURN_RUNNING (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_TURN_STARTING": {
    message:
      "Ход ещё запускается, поэтому останавливать пока нечего. Повторите попытку через несколько секунд.",
    description: "Mapped message for AGENT_SESSION_TURN_STARTING (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_MODEL_ENDPOINT_INVALID": {
    message: "Конечная точка модели должна быть допустимым HTTPS URL.",
    description: "Mapped message for AGENT_SESSION_MODEL_ENDPOINT_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_MODEL_NOT_OFFERED": {
    message:
      "Модель платформы этого рабочего пространства не предлагает {model}. Выберите модель из списка.",
    description:
      "Mapped message for AGENT_SESSION_MODEL_NOT_OFFERED (w10/m516)",
  },
  "agentSessions.errors.AGENT_SESSION_MODEL_PINNED": {
    message: "Этот агент использует только {pinnedModel}.",
    description: "Mapped message for AGENT_SESSION_MODEL_PINNED",
  },
  "agentSessions.errors.AGENT_SESSION_EGRESS_ALLOWLIST_INVALID": {
    message:
      "Запись списка разрешённого исходящего трафика «{entry}» недопустима: {reason}",
    description: "Mapped message for AGENT_SESSION_EGRESS_ALLOWLIST_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_EGRESS_ALLOWLIST_IMMUTABLE": {
    message:
      "Список разрешённого исходящего трафика нельзя изменить после запуска сессии.",
    description: "Mapped message for AGENT_SESSION_EGRESS_ALLOWLIST_IMMUTABLE",
  },
  "agentSessions.errors.AGENT_SESSION_EGRESS_PHASE_INVALID": {
    message:
      "Список разрешённого исходящего трафика нельзя задать в текущей фазе этой сессии.",
    description: "Mapped message for AGENT_SESSION_EGRESS_PHASE_INVALID",
  },
  "agentSessions.conversationLoading": {
    message: "Загрузка диалога…",
    description: "Placeholder while the client-only conversation column loads",
  },
  "agentSessions.conversationConnecting": {
    message: "Подключение к потоку сессии…",
    description: "Shown while the conversation stream replay is in flight",
  },
  "agentSessions.conversationNotStarted": {
    message: "Диалог появится, когда песочница будет готова.",
    description:
      "Healthy pre-stream state while a newly created session is provisioning",
  },
  "agentSessions.conversationEmpty": {
    message: "Диалога пока нет.",
    description: "Shown when a session has produced no transcript parts",
  },
  "agentSessions.turnInterrupted": {
    message: "Ход {turn} был прерван до завершения.",
    description:
      "Transcript line at the end of a turn a person stopped with Stop turn (ADR047 D12); not a failure",
  },
  "agentSessions.turnRetentionSaving": {
    message:
      "Сохраняем ваши изменения, чтобы следующее сообщение могло продолжить с них.",
    description:
      "Transcript line under a stopped turn while its unsaved work is being saved so the next message can continue from it (ADR047 D12, w22/m413)",
  },
  "agentSessions.turnRetentionKept": {
    message: "Ваши изменения хранятся до {time}.",
    description:
      "Transcript line under a stopped turn: its unpushed changes are kept until {time}, a date and time in the viewer's own zone (ADR047 D12, w22/m413)",
  },
  "agentSessions.turnRetentionKeptNoTime": {
    message: "Ваши изменения сохранены для следующего сообщения.",
    description:
      "Same as the kept line before the browser knows the viewer's time zone, so it names no time",
  },
  "agentSessions.turnRetentionContinued": {
    message: "Следующее сообщение продолжило работу с этими изменениями.",
    description:
      "Transcript line under a stopped turn: the next message started from its kept changes",
  },
  "agentSessions.turnRetentionLostExpired": {
    message:
      "Ваши неотправленные изменения потеряны: срок хранения песочницы истёк.",
    description:
      "Transcript line under a stopped turn: its kept sandbox expired and the unpushed changes in it are gone",
  },
  "agentSessions.turnRetentionLostStorageFull": {
    message:
      "Ваши неотправленные изменения не сохранены: хранилище песочниц рабочего пространства было заполнено.",
    description:
      "Transcript line under a stopped turn: nothing was kept because the workspace's sandbox storage cap was full",
  },
  "agentSessions.turnRetentionLostSaveFailed": {
    message:
      "Не удалось сохранить ваши неотправленные изменения, и они потеряны.",
    description:
      "Transcript line under a stopped turn: saving its sandbox failed, so the unpushed changes are gone",
  },
  "agentSessions.turnRetentionLostEditorOpen": {
    message:
      "Ваши неотправленные изменения не сохранены: в песочнице был открыт редактор.",
    description:
      "Transcript line under a stopped turn: nothing was kept because an editor (SSH) was open on the sandbox, which Stop does not end",
  },
  "agentSessions.turnRetentionLostNotRestored": {
    message:
      "Не удалось восстановить ваши неотправленные изменения, поэтому следующее сообщение началось со свежего checkout.",
    description:
      "Transcript line under a stopped turn: the next message could not start from the kept changes and started from a fresh checkout",
  },
  "agentSessions.turnRetentionLost": {
    message: "Ваши неотправленные изменения потеряны.",
    description:
      "Transcript line under a stopped turn: its unpushed changes were lost, for a reason that is not worth naming (the session was canceled or archived)",
  },
  "agentSessions.turnRetentionLostSandboxLost": {
    message:
      "Ваши неотправленные изменения потеряны: ход завершился ошибкой, и его песочницы уже нет.",
    description:
      "Transcript line under a failed turn: its sandbox was gone before its disk could be kept, so the unpushed work in it is lost (w10/m491)",
  },
  "agentSessions.turnStakeCommits_one": {
    message: "{count} неотправленный коммит",
    description:
      "Under a turn whose disk was kept or lost: how many commits its checkout held that were not pushed (w10/m491)",
  },
  "agentSessions.turnStakeCommits_few": {
    message: "{count} неотправленных коммита",
    description:
      "Under a turn whose disk was kept or lost: how many commits its checkout held that were not pushed (w10/m491)",
  },
  "agentSessions.turnStakeCommits_many": {
    message: "{count} неотправленных коммитов",
    description:
      "Under a turn whose disk was kept or lost: how many commits its checkout held that were not pushed (w10/m491)",
  },
  "agentSessions.turnStakeCommits_other": {
    message: "{count} неотправленного коммита",
    description:
      "Under a turn whose disk was kept or lost: how many commits its checkout held that were not pushed (w10/m491)",
  },
  "agentSessions.turnStakeChanges": {
    message: "незакоммиченные изменения",
    description:
      "Under a turn whose disk was kept or lost: its checkout held uncommitted changes (w10/m491)",
  },
  "agentSessions.turnError": {
    message: "Сбой хода {turn}",
    description:
      "Recorded failure of an earlier turn; subsequent turns remain readable",
  },
  "agentSessions.continuitySessionLoad": {
    message: "Агент возобновил работу с восстановленным состоянием диалога.",
    description:
      "System line when a fresh agent generation reloaded its own saved session (ADR047 D3 ladder rung 1)",
  },
  "agentSessions.continuityReprime": {
    message: "Агент перезапущен — контекст восстановлен из истории сессии.",
    description:
      "System line when a fresh agent generation was primed from the durable transcript (ladder rung 2)",
  },
  "agentSessions.continuityTaskRedelivery": {
    message: "Агент перезапущен — исходная задача отправлена повторно.",
    description:
      "System line when no prior turn ever reached the agent, so the task was re-sent (ladder rung 3)",
  },
  "agentSessions.conversationEndedEmpty": {
    message: "Эта сессия завершилась, прежде чем был записан диалог.",
    description: "Ended session with no durable transcript parts",
  },
  "agentSessions.conversationEnded": {
    message: "Сессия завершена.",
    description: "Footer note under a terminal session's replayed transcript",
  },
  "agentSessions.conversationIncomplete": {
    message: "Часть ответа ассистента не удалось сохранить",
    description:
      "Warning below a durable user turn whose assistant transcript is incomplete",
  },
  "agentSessions.conversationUnavailable": {
    message: "Поток диалога сейчас недоступен.",
    description:
      "Degraded-state message when the m43 stream endpoint errors or is unconfigured",
  },
  "agentSessions.conversationUnavailableTerminal": {
    message: "Транскрипт диалога для этой сессии недоступен.",
    description:
      "Terminal session — transcript unavailable rather than live stream degraded",
  },
  "agentSessions.showEarlierMessages_one": {
    message: "Показать {count} предыдущее сообщение",
    description:
      "Button revealing older transcript messages hidden by the render window (singular)",
  },
  "agentSessions.showEarlierMessages_few": {
    message: "Показать {count} предыдущих сообщения",
    description:
      "Button revealing older transcript messages hidden by the render window (plural)",
  },
  "agentSessions.showEarlierMessages_many": {
    message: "Показать {count} предыдущих сообщений",
    description:
      "Button revealing older transcript messages hidden by the render window (plural)",
  },
  "agentSessions.showEarlierMessages_other": {
    message: "Показать {count} предыдущих сообщения",
    description:
      "Button revealing older transcript messages hidden by the render window (plural)",
  },
  "agentSessions.groupThought": {
    message: "Размышления",
    description: "Collapsible header for the agent's reasoning group",
  },
  "agentSessions.groupPlan": {
    message: "План",
    description: "Collapsible header for the agent's plan/task checklist group",
  },
  "agentSessions.groupTerminal": {
    message: "Терминал",
    description: "Collapsible header for a terminal-output group",
  },
  "agentSessions.groupCommand": {
    message: "Команда",
    description: "Collapsible header for a command/tool-call group",
  },
  "agentSessions.groupDiff": {
    message: "Diff",
    description: "Label for a file-diff group when no path is given",
  },
  "agentSessions.terminalNoOutput": {
    message: "(нет вывода)",
    description: "Placeholder when a terminal group has no captured output",
  },
  "agentSessions.toolInput": {
    message: "Ввод",
    description: "Section label for a tool call's input",
  },
  "agentSessions.toolOutput": {
    message: "Вывод",
    description: "Section label for a tool call's output",
  },
  "agentSessions.toolError": {
    message: "Ошибка",
    description: "Section label for a tool call's error text",
  },
  "agentSessions.toolStateRunning": {
    message: "Выполняется",
    description: "Badge label for a tool call awaiting output",
  },
  "agentSessions.toolStateDone": {
    message: "Готово",
    description: "Badge label for a completed tool call",
  },
  "agentSessions.toolStateDenied": {
    message: "Отклонено",
    description: "Badge label for a tool call whose execution was denied",
  },
  "agentSessions.toolStateInterrupted": {
    message: "Прервано",
    description:
      "Badge label for a tool call the session ended before it finished",
  },
  "agentSessions.toolStateError": {
    message: "Сбой",
    description: "Badge label for a failed tool call",
  },
  "agentSessions.activityEdited_one": {
    message: "Изменён {count} файл",
    description:
      "Activity-group summary when the turn edited files (diffs, singular)",
  },
  "agentSessions.activityEdited_few": {
    message: "Изменено {count} файла",
    description: "Activity-group summary when the turn edited files (diffs)",
  },
  "agentSessions.activityEdited_many": {
    message: "Изменено {count} файлов",
    description: "Activity-group summary when the turn edited files (diffs)",
  },
  "agentSessions.activityEdited_other": {
    message: "Изменено {count} файла",
    description: "Activity-group summary when the turn edited files (diffs)",
  },
  "agentSessions.activityRan_one": {
    message: "Выполнена {count} команда",
    description:
      "Activity-group summary when the turn ran commands/terminals (singular)",
  },
  "agentSessions.activityRan_few": {
    message: "Выполнено {count} команды",
    description: "Activity-group summary when the turn ran commands/terminals",
  },
  "agentSessions.activityRan_many": {
    message: "Выполнено {count} команд",
    description: "Activity-group summary when the turn ran commands/terminals",
  },
  "agentSessions.activityRan_other": {
    message: "Выполнено {count} команды",
    description: "Activity-group summary when the turn ran commands/terminals",
  },
  "agentSessions.activitySteps_one": {
    message: "{count} шаг",
    description: "Activity-group summary fallback — the step count (singular)",
  },
  "agentSessions.activitySteps_few": {
    message: "{count} шага",
    description: "Activity-group summary fallback — the step count (plural)",
  },
  "agentSessions.activitySteps_many": {
    message: "{count} шагов",
    description: "Activity-group summary fallback — the step count (plural)",
  },
  "agentSessions.activitySteps_other": {
    message: "{count} шага",
    description: "Activity-group summary fallback — the step count (plural)",
  },
  "agentSessions.activityRead_one": {
    message: "Прочитан {count} файл",
    description: "Activity-group summary segment — file reads (singular)",
  },
  "agentSessions.activityRead_few": {
    message: "Прочитано {count} файла",
    description: "Activity-group summary segment — file reads",
  },
  "agentSessions.activityRead_many": {
    message: "Прочитано {count} файлов",
    description: "Activity-group summary segment — file reads",
  },
  "agentSessions.activityRead_other": {
    message: "Прочитано {count} файла",
    description: "Activity-group summary segment — file reads",
  },
  "agentSessions.activitySearched_one": {
    message: "Выполнен {count} поиск",
    description: "Activity-group summary segment — searches (singular)",
  },
  "agentSessions.activitySearched_few": {
    message: "Выполнено {count} поиска",
    description: "Activity-group summary segment — searches",
  },
  "agentSessions.activitySearched_many": {
    message: "Выполнено {count} поисков",
    description: "Activity-group summary segment — searches",
  },
  "agentSessions.activitySearched_other": {
    message: "Выполнено {count} поиска",
    description: "Activity-group summary segment — searches",
  },
  "agentSessions.activitySubagents_one": {
    message: "Запущен {count} субагент",
    description:
      "Activity-group summary segment — delegated subagent tasks (singular)",
  },
  "agentSessions.activitySubagents_few": {
    message: "Запущено {count} субагента",
    description: "Activity-group summary segment — delegated subagent tasks",
  },
  "agentSessions.activitySubagents_many": {
    message: "Запущено {count} субагентов",
    description: "Activity-group summary segment — delegated subagent tasks",
  },
  "agentSessions.activitySubagents_other": {
    message: "Запущено {count} субагента",
    description: "Activity-group summary segment — delegated subagent tasks",
  },
  "agentSessions.activityTools_one": {
    message: "Использован {count} инструмент",
    description:
      "Activity-group summary segment — tools of no known kind (singular)",
  },
  "agentSessions.activityTools_few": {
    message: "Использовано {count} инструмента",
    description: "Activity-group summary segment — tools of no known kind",
  },
  "agentSessions.activityTools_many": {
    message: "Использовано {count} инструментов",
    description: "Activity-group summary segment — tools of no known kind",
  },
  "agentSessions.activityTools_other": {
    message: "Использовано {count} инструмента",
    description: "Activity-group summary segment — tools of no known kind",
  },
  "agentSessions.activityThought_one": {
    message: "Размышлял {count} раз",
    description:
      "Activity-group summary segment — the agent's thoughts between its calls (singular)",
  },
  "agentSessions.activityThought_few": {
    message: "Размышлял {count} раза",
    description:
      "Activity-group summary segment — the agent's thoughts between its calls",
  },
  "agentSessions.activityThought_many": {
    message: "Размышлял {count} раз",
    description:
      "Activity-group summary segment — the agent's thoughts between its calls",
  },
  "agentSessions.activityThought_other": {
    message: "Размышлял {count} раза",
    description:
      "Activity-group summary segment — the agent's thoughts between its calls",
  },
  "agentSessions.activityThinking": {
    message: "Размышляет…",
    description: "Live status line while the agent is thinking between calls",
  },
  "agentSessions.activityFailed_one": {
    message: "{count} с ошибкой",
    description:
      "Activity-group summary segment — failed tool calls (singular)",
  },
  "agentSessions.activityFailed_few": {
    message: "{count} с ошибкой",
    description: "Activity-group summary segment — failed tool calls",
  },
  "agentSessions.activityFailed_many": {
    message: "{count} с ошибкой",
    description: "Activity-group summary segment — failed tool calls",
  },
  "agentSessions.activityFailed_other": {
    message: "{count} с ошибкой",
    description: "Activity-group summary segment — failed tool calls",
  },
  "agentSessions.outputClipped_one": {
    message: "Обрезан {formatted} символ",
    description:
      "Badge on a tool output the server stored head+tail-capped (singular)",
  },
  "agentSessions.outputClipped_few": {
    message: "Обрезано {formatted} символа",
    description: "Badge on a tool output the server stored head+tail-capped",
  },
  "agentSessions.outputClipped_many": {
    message: "Обрезано {formatted} символов",
    description: "Badge on a tool output the server stored head+tail-capped",
  },
  "agentSessions.outputClipped_other": {
    message: "Обрезано {formatted} символа",
    description: "Badge on a tool output the server stored head+tail-capped",
  },
  "agentSessions.showFullOutput": {
    message: "Показать весь вывод",
    description:
      "Button on a clipped tool output that loads the complete text the server kept",
  },
  "agentSessions.hideFullOutput": {
    message: "Скрыть весь вывод",
    description: "Button that collapses a loaded full tool output",
  },
  "agentSessions.fullOutputNotRetained": {
    message: "Полный вывод не сохранён",
    description:
      "Note on a clipped tool output whose complete text the server did not keep (clipped before full copies were stored)",
  },
  "agentSessions.fullOutputLoading": {
    message: "Загрузка полного вывода…",
    description: "Status while a tool call's full output loads",
  },
  "agentSessions.fullOutputError": {
    message: "Не удалось загрузить полный вывод.",
    description: "Inline error when loading a tool call's full output failed",
  },
  "agentSessions.fullOutputCopy": {
    message: "Копировать весь вывод",
    description:
      "Accessible label of the copy button on a loaded full tool output",
  },
  "agentSessions.fullOutputCopied": {
    message: "Полный вывод скопирован",
    description: "Toast after copying a full tool output",
  },
  "agentSessions.fullOutputCopyError": {
    message: "Не удалось скопировать полный вывод",
    description: "Toast when copying a full tool output failed",
  },
  "agentSessions.conversationPartial": {
    message: "Диалог загружен лишь частично.",
    description:
      "Inline row when the transcript stream failed after some of it rendered",
  },
  "agentSessions.liveStatus": {
    message: "Статус агента",
    description:
      "Accessible label for the single live status line under a streaming transcript",
  },
  "agentSessions.scrollToBottom": {
    message: "К последнему сообщению",
    description:
      "Accessible label for the floating jump-to-bottom button in the conversation column",
  },
  "agentSessions.recentSessions": {
    message: "Недавние",
    description: "Sessions sidebar — heading over the recent sessions list",
  },
  "agentSessions.sidebarEmpty": {
    message: "Сессий пока нет",
    description: "Sessions sidebar — empty state when the workspace has none",
  },
  "agentSessions.sidebarLabel": {
    message: "Сессии агентов",
    description: "Sessions sidebar — accessible landmark label",
  },
  "agentSessions.sidebarSearch": {
    message: "Поиск сессий",
    description: "Sessions sidebar — search toggle + input accessible label",
  },
  "agentSessions.sidebarSearchPlaceholder": {
    message: "Поиск сессий…",
    description: "Sessions sidebar — search input placeholder",
  },
  "agentSessions.sidebarNoMatches": {
    message: "Нет подходящих сессий",
    description:
      "Sessions sidebar — empty state when the search matches nothing",
  },
  "agentSessions.statusPhrase.prReady": {
    message: "PR готов",
    description: "Sidebar status phrase — completed session with a draft PR",
  },
  "agentSessions.statusPhrase.working": {
    message: "Работает…",
    description: "Sidebar status phrase — session still converging",
  },
  "agentSessions.statusPhrase.waitingForSandbox": {
    message: "Ожидание машины для песочницы",
    description:
      "Session status (header, list rows, sidebar, conversation, automation runs) — the turn waits because no sandbox machine has room for it yet (w10/m497); the time waited follows where there is room",
  },
  "agentSessions.menuMore": {
    message: "Другие действия",
    description: "Header — accessible label for the '…' overflow menu",
  },
  "agentSessions.openPr": {
    message: "Открыть pull request",
    description: "Header overflow menu — open the draft PR in a new tab",
  },
  "agentSessions.connect": {
    message: "Подключиться",
    description: "Header — trigger for the Open-in-Zed / SSH connect menu",
  },
  "agentSessions.openInZed": {
    message: "Открыть в Zed",
    description:
      "Connect menu — hotlink that opens the sandbox as a Zed remote project",
  },
  "agentSessions.openInZedHint": {
    message: "Открывает /workspace песочницы по SSH. Требуется редактор Zed. Закрытая песочница запускается при подключении.",
    description:
      "Connect menu — helper text under the Open-in-Zed action; a closed sandbox starts when the editor or ssh connects (ADR054 D9)",
  },
  "agentSessions.connectSSH": {
    message: "SSH",
    description: "Connect menu — label above the copyable ssh command",
  },
  "agentSessions.sshCopy": {
    message: "Копировать команду SSH",
    description: "Connect menu — accessible label for the copy button",
  },
  "agentSessions.sshCopied": {
    message: "Команда SSH скопирована",
    description: "Connect menu — toast after copying the ssh command",
  },
  "agentSessions.sshCopyError": {
    message: "Не удалось скопировать",
    description: "Connect menu — toast when copying the ssh command fails",
  },
  "agentSessions.groupThoughtFor": {
    message: "Размышления: {duration}",
    description:
      "Reasoning-group summary with elapsed duration from persisted part timestamps (or ~arrival timing)",
  },
  "agentSessions.terminalStatus.completed": {
    message: "Агент завершил работу",
    description: "Terminal transcript status line — completed session",
  },
  "agentSessions.terminalStatus.failed": {
    message: "Сессия завершилась с ошибкой",
    description: "Terminal transcript status line — failed session",
  },
  "agentSessions.terminalStatus.canceled": {
    message: "Сессия была отменена",
    description: "Terminal transcript status line — canceled session",
  },
  "agentSessions.tagsLabel": {
    message: "Теги",
    description: "Accessible label for a session row's tag list (w21/002)",
  },
  "agentSessions.filterByTag": {
    message: "Показать сессии с тегом {tag}",
    description:
      "Accessible label of a tag chip that filters the session list to that tag",
  },
  "agentSessions.filterTagged": {
    message: "С тегом {tag}",
    description: "Session list heading while narrowed to one tag",
  },
  "agentSessions.archive": {
    message: "Архивировать",
    description: "Action putting a session out of the working set (ADR065)",
  },
  "agentSessions.unarchive": {
    message: "Вернуть из архива",
    description: "Action returning an archived session to the working set",
  },
  "agentSessions.archiveSuccess": {
    message: "Сессия архивирована",
    description: "Toast after archiving a session",
  },
  "agentSessions.undoArchive": {
    message: "Отменить",
    description: "Toast action that immediately unarchives a session",
  },
  "agentSessions.unarchiveSuccess": {
    message: "Сессия возвращена из архива",
    description: "Toast after unarchiving a session",
  },
  "agentSessions.archivedBadge": {
    message: "В архиве",
    description: "Badge marking a session as archived (out of the working set)",
  },
  "agentSessions.startedFromSnapshot": {
    message: "Запущено из снимка",
    description:
      "Session header chip: the latest turn booted from a repository environment build (ADR111 D4)",
  },
  "agentSessions.startedFromSnapshotTitle": {
    message:
      "Последний ход запущен из сборки окружения {id}, в которой шаги настройки уже выполнены.",
    description:
      "Tooltip of the Started from snapshot chip (id is the build id, arb-…)",
  },
  "agentSessions.delete": {
    message: "Удалить",
    description: "Destructive action permanently deleting a finished session",
  },
  "agentSessions.deleting": {
    message: "Удаление…",
    description: "Delete confirm button while the delete is in flight",
  },
  "agentSessions.deleteSuccess": {
    message: "Сессия удалена",
    description: "Toast after permanently deleting a session",
  },
  "agentSessions.deleteConfirmTitle": {
    message: "Удалить эту сессию?",
    description: "Delete confirmation dialog title",
  },
  "agentSessions.deleteConfirmBody": {
    message:
      "Запись сессии и транскрипт её диалога будут удалены без возможности восстановления. Отправленные ветки и pull request в GitHub не затрагиваются. Это действие нельзя отменить.",
    description: "Delete confirmation dialog body",
  },
  "agentSessions.deleteConfirmDismiss": {
    message: "Оставить сессию",
    description: "Delete confirmation dialog dismiss button",
  },
  "agentSessions.deleteConfirmProceed": {
    message: "Удалить сессию",
    description: "Delete confirmation dialog destructive proceed button",
  },
  "agentSessions.colActions": {
    message: "Действия",
    description: "Screen-reader label of the list's trailing actions column",
  },
  "agentSessions.filterActive": {
    message: "Недавние",
    description: "List membership tab — the unarchived working set",
  },
  "agentSessions.filterArchived": {
    message: "В архиве",
    description: "List membership tab — archived sessions only",
  },
  "agentSessions.filterAll": {
    message: "Все",
    description: "List membership tab — archived and unarchived together",
  },
  "agentSessions.loadMore": {
    message: "Показать ещё",
    description: "Button fetching the next page of sessions",
  },
  "agentSessions.loadingMore": {
    message: "Загрузка…",
    description: "Load-more button while the next page is in flight",
  },
  "agentSessions.attachFiles": {
    message: "Прикрепить файлы",
    description:
      "Paperclip button that opens the file picker in the agent composers",
  },
  "agentSessions.attachmentRemove": {
    message: "Удалить {name}",
    description: "Accessible label of the remove button on an attachment chip",
  },
  "agentSessions.attachmentTooLarge": {
    message: "Больше {max}",
    description: "Attachment chip error when a file exceeds the per-file limit",
  },
  "agentSessions.attachmentTooMany": {
    message: "Не более {max} файлов в сообщении",
    description: "Attachment chip error when a message has too many files",
  },
  "agentSessions.attachmentDropHint": {
    message: "Перетащите файлы, чтобы прикрепить",
    description: "Overlay shown while dragging files over a composer",
  },
  "agentSessions.sharedFileLabel": {
    message: "Поделился файлом",
    description: "Label above a file the agent shared with share_file",
  },
  "agentSessions.filesButton": {
    message: "Файлы",
    description: "Session header button that opens the session files",
  },
  "agentSessions.filesTitle": {
    message: "Файлы сессии",
    description: "Title of the session files sheet",
  },
  "agentSessions.filesDescription": {
    message:
      "Файлы, которые вы прикрепили, и файлы, которыми поделился агент в этой сессии.",
    description: "Description under the session files sheet title",
  },
  "agentSessions.filesEmpty": {
    message:
      "Файлов пока нет. Прикрепите файлы к сообщению или попросите агента поделиться файлом.",
    description: "Empty state of the session files sheet",
  },
  "agentSessions.filesError": {
    message: "Не удалось загрузить файлы сессии.",
    description: "Error state of the session files sheet",
  },
  "agentSessions.filesColumnName": {
    message: "Имя",
    description: "Files table column: file name",
  },
  "agentSessions.filesColumnFrom": {
    message: "От кого",
    description: "Files table column: who added the file",
  },
  "agentSessions.filesColumnTurn": {
    message: "Ход",
    description: "Files table column: turn number",
  },
  "agentSessions.filesColumnSize": {
    message: "Размер",
    description: "Files table column: file size",
  },
  "agentSessions.filesFromInput": {
    message: "Вы",
    description: "Files table: a file the user attached",
  },
  "agentSessions.filesFromOutput": {
    message: "Агент",
    description: "Files table: a file the agent shared",
  },
  "agentSessions.filesLoadMore": {
    message: "Показать ещё",
    description: "Files table: load the next page",
  },
  "agentSessions.attachmentsBlockSend": {
    message: "Удалите файлы, которые не удалось загрузить, чтобы отправить",
    description: "Hint when failed attachment chips block sending",
  },
  "agentSessions.attachmentOpen": {
    message: "Открыть {name}",
    description:
      "Accessible label of an attachment thumbnail that opens the lightbox",
  },
  "agentSessions.attachmentDownload": {
    message: "Скачать {name}",
    description: "Accessible label of an attachment download card",
  },
  "agentSessions.attachmentUnavailable": {
    message: "Недоступно",
    description:
      "Shown on an attachment whose download link could not be loaded",
  },
  "agentSessions.attachmentUploading": {
    message: "Загрузка, {percent}%",
    description: "Screen-reader status of an attachment chip while uploading",
  },
  "agentSessions.attachmentReady": {
    message: "Загружено",
    description:
      "Screen-reader status of an attachment chip once its upload landed",
  },
  "agentSessions.attachmentHttpError": {
    message: "Не удалось загрузить (HTTP {status})",
    description:
      "Attachment chip error when the server answered without a message",
  },
  "agentSessions.attachmentNetworkError": {
    message: "Не удалось загрузить: сетевая ошибка",
    description:
      "Attachment chip error when the upload could not reach the server",
  },
  "agentSessions.attachmentsUploading": {
    message: "Ожидание завершения загрузки файлов…",
    description: "Composer hint while attachment uploads block sending",
  },
  "agentSessions.spendWarningTitle": {
    message:
      "Расходы на модель агентов достигли {percent}% лимита этого месяца",
    description: "Composer warning at 80% of the spend cap (ADR091 D6)",
  },
  "agentSessions.spendWarningBody": {
    message:
      "Использовано {used} из {cap}. Новые запросы к модели прекратятся по достижении лимита.",
    description: "Composer warning body",
  },
  "agentSessions.spendWarningBodyCharge": {
    message:
      "Использовано {used} из {cap} сверх кредитов на оплату. Сначала расходуются кредиты на оплату; когда расходы сверх них достигнут {cap}, с карты на счёте будет списана сумма, а сессии продолжат работу. Новые запросы к модели прекратятся, только если списание не удастся.",
    description:
      "Composer warning body when the cap is a new workspace's charge point (ADR091 D10.2a); {used} is the spend billing credits leave",
  },
  "agentSessions.spendWarningBodyCeiling": {
    message:
      "Использовано {used} из {cap} сверх кредитов на оплату. Сначала расходуются кредиты на оплату; новые запросы к модели прекратятся, когда расходы сверх них достигнут {cap}.",
    description:
      "Composer warning body when the trust tier's ceiling binds on a workspace bex cannot charge (payment hold, excluded or comped) and the spend it counts, what billing credits leave ({used}), differs from month-to-date",
  },
  "agentSessions.spendReachedTitle.cap": {
    message: "Достигнут месячный лимит расходов на модель агентов",
    description: "Composer refusal title: the workspace's own cap",
  },
  "agentSessions.spendReachedTitle.newWorkspace": {
    message: "Достигнут лимит расходов для нового рабочего пространства",
    description: "Composer refusal title: the T0 trust-tier ceiling",
  },
  "agentSessions.spendReachedTitle.locked": {
    message: "Расходы на модель агентов приостановлены",
    description: "Composer refusal title: a payment hold",
  },
  "agentSessions.spendReachedBody.cap": {
    message:
      "Это рабочее пространство израсходовало {used} из месячного лимита {cap}, поэтому новые сессии запустить нельзя. Администратор рабочего пространства может повысить лимит, либо он сбросится 1-го числа (UTC).",
    description: "Composer refusal body: the workspace's own cap",
  },
  "agentSessions.spendReachedBody.capMax": {
    message:
      "Это рабочее пространство израсходовало {used} из месячного лимита {cap} — максимально возможного, поэтому новые сессии запустить нельзя. Лимит сбросится 1-го числа (UTC); если вам нужен более высокий лимит, обратитесь в поддержку.",
    description:
      "Composer refusal body: the workspace's own cap binds at its maximum, so no admin can raise it",
  },
  "agentSessions.spendReachedBody.newWorkspace": {
    message:
      "Новые рабочие пространства могут потратить до {cap} на модель от bex до первого успешного платежа. После этого лимит повысится, либо сбросится 1-го числа (UTC).",
    description: "Composer refusal body: the T0 trust-tier ceiling",
  },
  "agentSessions.spendReachedBody.locked": {
    message:
      "Неудачный или оспоренный платёж ограничивает расходы этого рабочего пространства на модель агентов суммой {cap}. Урегулируйте неоплаченный платёж в разделе «Оплата», затем запустите новую сессию.",
    description: "Composer refusal body: a payment hold",
  },
  "agentSessions.spendIncreaseCap": {
    message: "Повысить лимит",
    description: "Admin action linking to Settings → Agents",
  },
  "agentSessions.spendGoToBilling": {
    message: "Перейти к оплате",
    description: "Link to the Billing page",
  },
  "agentSessions.spendBalanceDueTitle": {
    message: "Платёж не прошёл",
    description: "Composer refusal title: BILLING_BALANCE_DUE",
  },
  "agentSessions.spendBalanceDueBody": {
    message:
      "Сессии агентов на модели от bex приостановлены, пока не будет оплачен неоплаченный счёт.",
    description: "Composer refusal body: BILLING_BALANCE_DUE",
  },
  "agentSessions.errors.AGENT_SESSION_TRUST_LIMIT": {
    message:
      "Новые рабочие пространства могут одновременно запускать {limit} сессий агентов на модели от bex до первого успешного платежа. Дождитесь завершения выполняющейся сессии или остановите одну из них и повторите попытку.",
    description: "Mapped message for AGENT_SESSION_TRUST_LIMIT (ADR091 D10.1)",
  },
  "agentSessions.errors.AGENT_SESSION_TRUST_LIMIT_locked": {
    message:
      "После неудачного или оспоренного платежа это рабочее пространство может одновременно запускать {limit} сессий агентов на модели от bex. Урегулируйте неоплаченный платёж в разделе «Оплата», чтобы снять ограничение.",
    description:
      "Mapped message for AGENT_SESSION_TRUST_LIMIT while a payment hold applies",
  },
  "agentSessions.errors.AGENT_SESSION_SPEND_LIMIT_INVALID": {
    message: "Лимит расходов должен быть больше $0 и не более $100 000.",
    description:
      "Mapped message for AGENT_SESSION_SPEND_LIMIT_INVALID (ADR094 D8)",
  },
  "agentSessions.errors.AGENT_SESSION_SPEND_LIMIT_UNAVAILABLE": {
    message:
      "Лимит расходов применяется только к сессиям на модели от bex. Это рабочее пространство использует собственный ключ модели, поэтому уберите лимит или переключите рабочее пространство на модель bex в разделе «Настройки → Агенты».",
    description:
      "Mapped message for AGENT_SESSION_SPEND_LIMIT_UNAVAILABLE (ADR094 D8)",
  },
  "agentSessions.spendLimitLabel": {
    message: "Лимит расходов (USD)",
    description:
      "Composer — label/aria-label of the optional per-session spend ceiling input",
  },
  "agentSessions.spendLimitPlaceholder": {
    message: "Лимит расходов",
    description:
      "Composer — placeholder of the optional per-session spend ceiling input",
  },
  "agentSessions.spendLimitHint": {
    message:
      "Необязательно. Сессия остановится, когда расходы на модель bex достигнут этой суммы (больше $0, не более $100 000).",
    description: "Composer — help text under the spend limit input",
  },
  "agentSessions.spendLimitInvalid": {
    message:
      "Введите сумму больше $0 и не более $100 000 или оставьте поле пустым.",
    description: "Composer — inline error for an out-of-range spend limit",
  },
  "agentSessions.failureSessionSpendLimitReached": {
    message:
      "Эта сессия остановлена, потому что расходы на модель достигли лимита сессии в {limit}. Запустите новую сессию с более высоким лимитом, чтобы продолжить.",
    description:
      "Failure callout for AGENT_MODEL_SPEND_CAP_REACHED when the session's own maxSpendUSD ceiling was hit",
  },
  "agentSessions.spendNote": {
    message: "Расходы на модель: {spent}",
    description:
      "Session page — the session's model spend so far (outcome.cost)",
  },
  "agentSessions.spendNoteWithLimit": {
    message: "Расходы на модель: {spent} из лимита сессии {limit}",
    description:
      "Session page — model spend so far beside the session's spend limit",
  },
  "agentSessions.costNote": {
    message: "Стоимость {total} · модель {model}, вычисления {compute}",
    description:
      "Session page — the session's total cost so far (outcome.cost.totalUSD) with its model and sandbox compute parts",
  },
  "agentSessions.costNoteWithLimit": {
    message:
      "Стоимость {total} · модель {model} из лимита сессии {limit}, вычисления {compute}",
    description:
      "Session page — total cost with its parts; the session spend limit caps the model part",
  },
  "agentSessions.resultTitle": {
    message: "Результат",
    description:
      "Session page — heading over the turn's final structured result",
  },
  "agentSessions.resultDraftTitle": {
    message: "Черновой результат",
    description:
      "Session page — heading over the agent's not-yet-final structured result while the turn runs",
  },
  "agentSessions.resultDraftHint": {
    message: "Пока не окончательный. Агент ещё может его изменить.",
    description: "Session page — note under a draft structured result",
  },
  "agentSessions.resultCopy": {
    message: "Копировать JSON",
    description: "Session page — button copying the structured result",
  },
  "agentSessions.resultCopied": {
    message: "Скопировано",
    description: "Session page — toast after copying the structured result",
  },
  "agentSessions.failureSpendCapReached": {
    message:
      "Это рабочее пространство достигло месячного лимита расходов на модель агентов. Администратор может повысить лимит в разделе «Настройки → Агенты», либо он сбросится 1-го числа (UTC).",
    description: "Failure callout for AGENT_MODEL_SPEND_CAP_REACHED",
  },
  "agentSessions.failureBalanceDue": {
    message:
      "Платёж для этого рабочего пространства не прошёл. Оплатите неоплаченный счёт в разделе «Оплата», затем запустите новую сессию.",
    description: "Failure callout for BILLING_BALANCE_DUE",
  },
  "agentSessions.failureSetupTitle": {
    message: "Не удалось настроить репозиторий",
    description:
      "Failure callout heading when a repository setup step failed (AGENT_SESSION_SETUP_FAILED)",
  },
  "agentSessions.failureSetupHint": {
    message:
      "Шаг настройки в окружении этого репозитория завершился ошибкой, поэтому агент не запустился и расходы на модель не начислялись. Исправьте шаг в разделе «Настройки → Агенты → Окружения репозиториев» и повторите попытку.",
    description:
      "Failure callout guidance under the recorded reason (which names the step and its exit code) for AGENT_SESSION_SETUP_FAILED",
  },
  "agentSessions.failureSetupAction": {
    message: "Редактировать окружение репозитория",
    description:
      "Failure callout button opening Settings → Repository environments",
  },
  "agentSessions.costTotal": {
    message: "Стоимость модели: {amount}",
    description: "Session header: the session's model cost (ADR091 D6)",
  },
  "agentSessions.costTotalAll": {
    message: "Общая стоимость: {amount}",
    description:
      "Session header: the session's total cost so far, model plus sandbox compute (outcome.cost.totalUSD, w9/m178)",
  },
  "agentSessions.costBreakdown": {
    message: "Модель {model} · вычисления {compute}",
    description:
      "Session header cost popover: the model and sandbox compute parts of the total",
  },
  "agentSessions.costByo": {
    message: "Оплачивается вашим поставщиком",
    description: "Session header: a BYO-key session has no bex model cost",
  },
  "agentSessions.costTitle": {
    message: "Стоимость модели по ходам",
    description: "Heading of the per-turn cost popover",
  },
  "agentSessions.costTurn": {
    message: "Ход {turn}",
    description: "One turn's row in the cost popover",
  },
  "agentSessions.costSetup": {
    message: "До первого хода",
    description: "Requests recorded before the first turn",
  },
  "agentSessions.costTurnTokens": {
    message: "{input} входных · {output} выходных · {cache} кэш-токенов",
    description: "A turn's token counts",
  },
  "agentSessions.costPricing": {
    message: "Оплата по стоимости поставщика модели + 30%.",
    description: "Pricing statement in the cost popover",
  },
  "agentSessions.secretsButton": {
    message: "Секреты",
    description: "Composer toolbar: the Secrets button label/tooltip (ADR098)",
  },
  "agentSessions.secretsButtonCount_one": {
    message: "Секреты (прикреплено: {count})",
    description:
      "Composer Secrets button accessible label when secrets are attached",
  },
  "agentSessions.secretsButtonCount_few": {
    message: "Секреты (прикреплено: {count})",
    description:
      "Composer Secrets button accessible label when secrets are attached",
  },
  "agentSessions.secretsButtonCount_many": {
    message: "Секреты (прикреплено: {count})",
    description:
      "Composer Secrets button accessible label when secrets are attached",
  },
  "agentSessions.secretsButtonCount_other": {
    message: "Секреты (прикреплено: {count})",
    description:
      "Composer Secrets button accessible label when secrets are attached",
  },
  "agentSessions.secretsTitle": {
    message: "Секреты для этой сессии",
    description: "Composer Secrets popover heading",
  },
  "agentSessions.secretsDescription": {
    message:
      "Агент читает каждый из них как переменную окружения. Значения больше не показываются и скрываются в транскрипте.",
    description: "Composer Secrets popover description",
  },
  "agentSessions.secretsWorkspace": {
    message: "Секреты рабочего пространства",
    description:
      "Composer Secrets popover: workspace agent secrets list heading",
  },
  "agentSessions.secretsWorkspaceLoadFailed": {
    message: "Не удалось загрузить секреты агентов рабочего пространства.",
    description:
      "Composer secrets panel error when the workspace secret list fails to load",
  },
  "agentSessions.secretsWorkspaceEmpty": {
    message: "Секретов рабочего пространства пока нет.",
    description: "Composer Secrets popover: no workspace agent secrets",
  },
  "agentSessions.secretsManage": {
    message: "Управлять в настройках",
    description: "Composer Secrets popover: link to Settings → Agents",
  },
  "agentSessions.secretsInline": {
    message: "Только для этой сессии",
    description: "Composer Secrets popover: inline secrets heading",
  },
  "agentSessions.secretsRemove": {
    message: "Удалить {name}",
    description:
      "Composer Secrets popover: remove an inline secret (accessible label)",
  },
  "agentSessions.secretsNamePlaceholder": {
    message: "NAME",
    description: "Composer Secrets popover: inline secret name placeholder",
  },
  "agentSessions.secretsNameLabel": {
    message: "Название секрета",
    description: "Composer Secrets popover: inline secret name field label",
  },
  "agentSessions.secretsValuePlaceholder": {
    message: "Значение",
    description: "Composer Secrets popover: inline secret value placeholder",
  },
  "agentSessions.secretsValueLabel": {
    message: "Значение секрета",
    description: "Composer Secrets popover: inline secret value field label",
  },
  "agentSessions.secretsAdd": {
    message: "Добавить",
    description: "Composer Secrets popover: add an inline secret",
  },
  "agentSessions.secretsHint": {
    message: "Удаляется вместе с сессией.",
    description: "Composer Secrets popover: inline secret lifetime hint",
  },
  "agentSessions.secretsFull": {
    message: "Сессия может содержать не более 20 секретов.",
    description: "Composer Secrets popover: session secret limit reached",
  },
  "agentSessions.secretsDuplicate": {
    message: "В этой сессии уже есть секрет с таким названием.",
    description: "Composer Secrets popover: duplicate inline secret name",
  },
  "agentSessions.secretsScopeRepo": {
    message: "Этот репозиторий",
    description:
      "Composer Secrets popover: hint beside a secret scoped to the chosen repository, included",
  },
  "agentSessions.secretsScopeOtherRepo": {
    message: "Только в {repo}",
    description:
      "Composer Secrets popover: hint beside a secret scoped to another repository, not available here",
  },
  "agentSessions.appliedSecretsTitle": {
    message: "Секреты в последнем ходе",
    description:
      "Session header: heading of the applied-secrets popover (names and sources, never values)",
  },
  "agentSessions.appliedSecretSource.inline": {
    message: "эта сессия",
    description:
      "Applied secret source: an inline secret given to this session only",
  },
  "agentSessions.appliedSecretSource.explicit": {
    message: "выбран",
    description: "Applied secret source: a workspace secret the session picked",
  },
  "agentSessions.appliedSecretSource.repo": {
    message: "репозиторий",
    description:
      "Applied secret source: a workspace secret scoped to the session's repository",
  },
  "agentSessions.appliedSecretSource.workspace": {
    message: "рабочее пространство",
    description:
      "Applied secret source: a workspace secret every session receives",
  },
  "agentSessions.metaSecrets_one": {
    message: "{count} секрет",
    description:
      "Detail header — number of attached secrets (singular, ADR098)",
  },
  "agentSessions.metaSecrets_few": {
    message: "{count} секрета",
    description: "Detail header — number of attached secrets (plural, ADR098)",
  },
  "agentSessions.metaSecrets_many": {
    message: "{count} секретов",
    description: "Detail header — number of attached secrets (plural, ADR098)",
  },
  "agentSessions.metaSecrets_other": {
    message: "{count} секрета",
    description: "Detail header — number of attached secrets (plural, ADR098)",
  },
  "agentSessions.skillsButton": {
    message: "Навыки",
    description: "Composer toolbar: the Skills button label/tooltip (ADR099)",
  },
  "agentSessions.skillsButtonCount_one": {
    message: "Навыки (выбрано: {count})",
    description:
      "Composer Skills button accessible label when skills are chosen",
  },
  "agentSessions.skillsButtonCount_few": {
    message: "Навыки (выбрано: {count})",
    description:
      "Composer Skills button accessible label when skills are chosen",
  },
  "agentSessions.skillsButtonCount_many": {
    message: "Навыки (выбрано: {count})",
    description:
      "Composer Skills button accessible label when skills are chosen",
  },
  "agentSessions.skillsButtonCount_other": {
    message: "Навыки (выбрано: {count})",
    description:
      "Composer Skills button accessible label when skills are chosen",
  },
  "agentSessions.skillsTitle": {
    message: "Навыки для этого хода",
    description: "Composer Skills popover heading",
  },
  "agentSessions.skillsDescription": {
    message:
      "Агент следует каждому выбранному навыку. Включённые навыки он может применять и сам.",
    description: "Composer Skills popover description",
  },
  "agentSessions.skillsLoadFailed": {
    message: "Не удалось загрузить навыки рабочего пространства.",
    description: "Composer Skills popover: list failed",
  },
  "agentSessions.skillsEmpty": {
    message: "В этом рабочем пространстве пока нет навыков.",
    description: "Composer Skills popover: none exist",
  },
  "agentSessions.skillsManage": {
    message: "Управление навыками",
    description: "Composer Skills popover link to Settings → Agents",
  },
  "agentSessions.skillsArgumentsLabel": {
    message: "Аргументы для {name}",
    description: "Composer Skills popover: argument field label",
  },
  "agentSessions.skillsArgumentsPlaceholder": {
    message: "Аргументы (необязательно)",
    description: "Composer Skills popover: argument field placeholder",
  },
  "agentSessions.skillsArgumentsInvalid": {
    message: "Аргументы — одна строка, до 4 КиБ.",
    description: "Composer Skills popover: invalid arguments",
  },
  "agentSessions.skillsFull": {
    message: "В одном ходе можно использовать не более 5 навыков.",
    description: "Composer Skills popover: limit reached",
  },
  "agentSessions.skillsHint": {
    message: "Аргументы подставляются вместо $ARGUMENTS в навыке.",
    description: "Composer Skills popover hint",
  },
  "agentSessions.skillsRemove": {
    message: "Удалить {name}",
    description: "Accessible label of a chosen skill chip's remove button",
  },
  "agentSessions.turnSkills": {
    message: "Навыки, использованные в этом ходе",
    description: "Accessible label of a transcript turn's skill pills",
  },
  "agentSessions.turnOrigins": {
    message: "Что скорректировало этот ход",
    description:
      "Accessible label of the lines above a transcript turn that PR feedback started (w9/m344)",
  },
  "agentSessions.turnOriginReview": {
    message: "Скорректировано по ревью от @{author}",
    description:
      "A turn started by a pull-request review; {author} is the reviewer's GitHub login. Links to the review",
  },
  "agentSessions.turnOriginReviewAnonymous": {
    message: "Скорректировано по ревью PR",
    description:
      "A turn started by a pull-request review whose author is unknown. Links to the review",
  },
  "agentSessions.turnOriginComment": {
    message: "Скорректировано по комментарию к PR от @{author}",
    description:
      "A turn started by a pull-request comment; {author} is the commenter's GitHub login. Links to the comment",
  },
  "agentSessions.turnOriginCommentAnonymous": {
    message: "Скорректировано по комментарию к PR",
    description:
      "A turn started by a pull-request comment whose author is unknown. Links to the comment",
  },
  "agentSessions.turnOriginCiFailure": {
    message: "Скорректировано из-за неудачного запуска CI",
    description:
      "A turn started by a failed CI run on the pull request. Links to the failed checks",
  },
  "agentSessions.turnOriginCiFailureOnBase": {
    message:
      "Скорректировано из-за неудачного запуска CI; некоторые проверки также не проходят в {base}, и дело не в этом изменении",
    description:
      "A turn started by a failed CI run on the pull request where at least one failing check also fails on the base branch ({base}, a branch name), so the pull request did not cause it. Links to the failed checks",
  },
  "agentSessions.turnOriginsStart": {
    message: "Что запустило эту сессию",
    description:
      "Accessible label of the line above a session's first turn when a /bex comment on a pull request started it (w9/m348)",
  },
  "agentSessions.turnOriginStartComment": {
    message: "Запущено по комментарию @{author}",
    description:
      "The first turn of a session a /bex pull-request comment started; {author} is the commenter's GitHub login. Links to the comment",
  },
  "agentSessions.turnOriginStartCommentAnonymous": {
    message: "Запущено по комментарию к PR",
    description:
      "The first turn of a session a /bex pull-request comment started, when the commenter is unknown. Links to the comment",
  },
  "agentSessions.lineage.childSessions": {
    message: "Дочерние сессии",
    description:
      "Heading of the block listing sessions the agent spawned, in the parent session's transcript",
  },
  "agentSessions.lineage.childCardTitle": {
    message: "Создана сессия",
    description:
      "Small label above a child session's title on its card in the parent's transcript",
  },
  "agentSessions.lineage.agentsPanel": {
    message: "Агенты",
    description:
      "Heading of the session page's right-hand panel listing the sessions below this one",
  },
  "agentSessions.lineage.viewSession": {
    message: "Открыть сессию",
    description: "Link on an Agents panel row that opens that child session",
  },
  "agentSessions.lineage.parentSession": {
    message: "Родительская сессия",
    description:
      "Link in the Agents panel of a child session that opens its parent",
  },
  "agentSessions.lineage.collapsePanel": {
    message: "Свернуть панель «Агенты»",
    description:
      "Accessible label of the button that collapses the Agents panel",
  },
  "agentSessions.lineage.expandPanel": {
    message: "Развернуть панель «Агенты»",
    description: "Accessible label of the button that expands the Agents panel",
  },
  "agentSessions.lineage.collapseChildren": {
    message: "Скрыть дочерние сессии",
    description:
      "Accessible label of a sidebar row's button that collapses its nested child sessions",
  },
  "agentSessions.lineage.expandChildren": {
    message: "Показать дочерние сессии",
    description:
      "Accessible label of a sidebar row's button that expands its nested child sessions",
  },
  "agentSessions.lineage.status.working": {
    message: "Работает",
    description:
      "Status group and badge of a child session that is still running",
  },
  "agentSessions.lineage.status.done": {
    message: "Готово",
    description:
      "Status group and badge of a child session that finished successfully",
  },
  "agentSessions.lineage.status.failed": {
    message: "Сбой",
    description: "Status group and badge of a child session that failed",
  },
  "agentSessions.lineage.status.canceled": {
    message: "Отменена",
    description: "Status group and badge of a child session that was canceled",
  },
  "agentSessions.sidebarMine": {
    message: "Мои",
    description:
      "Sessions sidebar — show only the sessions you started (Devin's default)",
  },
  "agentSessions.sidebarEveryone": {
    message: "Все",
    description: "Sessions sidebar — show every member's sessions",
  },
  "agentSessions.sidebarOwnerFilter": {
    message: "Чьи сессии",
    description:
      "Sessions sidebar — accessible name of the Mine/Everyone toggle",
  },
  "agentSessions.sidebarEmptyMine": {
    message: "Вы не запускали сессий. Более старые сессии — на вкладке «Все».",
    description:
      "Sessions sidebar — empty state under Mine. Sessions from before 2026-11-08 record no starter, so they appear only under Everyone",
  },
  "agentSessions.startedByPerson": {
    message: "Запустил(а) {name}",
    description: "Session header and list rows — who started the session",
  },
  "agentSessions.startedForPerson": {
    message: "для {name}",
    description:
      "Session header — after 'Started by automation X': the member the automation runs as",
  },
  "agentSessions.editTags": {
    message: "Редактировать теги",
    description: "Session header menu — open the tag editor",
  },
  "agentSessions.editTagsTitle": {
    message: "Редактирование тегов",
    description: "Tag editor dialog title",
  },
  "agentSessions.editTagsDescription": {
    message:
      "Теги заменяют текущие теги сессии. Разделяйте их запятыми или пробелами; оставьте поле пустым, чтобы очистить.",
    description: "Tag editor dialog description",
  },
  "agentSessions.editTagsHint": {
    message:
      "До 20 тегов: строчные буквы, цифры, двоеточие, подчёркивание или дефис.",
    description: "Tag editor dialog — the tag rules",
  },
  "agentSessions.editTagsSave": {
    message: "Сохранить теги",
    description: "Tag editor dialog — save button",
  },
  "agentSessions.editTagsSaved": {
    message: "Теги сохранены",
    description: "Toast after the tags were replaced",
  },
};

export default ruAgentSessions;
