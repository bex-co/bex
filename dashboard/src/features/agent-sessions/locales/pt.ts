import type { TranslationEntry } from "@/i18n";

const ptAgentSessions: Record<string, TranslationEntry> = {
  "agentSessions.pageTitle": {
    message: "Sessões de agente",
    description: "Agent sessions list page heading and document title",
  },
  "agentSessions.listTitle": {
    message: "Recentes",
    description: "Agent sessions recents heading — matches the rail",
  },
  "agentSessions.colTask": {
    message: "Tarefa",
    description: "Agent sessions table column header — the session's prompt",
  },
  "agentSessions.colRepo": {
    message: "Repositório",
    description: "Agent sessions table column header — repo/branch",
  },
  "agentSessions.colAgent": {
    message: "Agente",
    description: "Agent sessions table column header — the driver agent",
  },
  "agentSessions.colPhase": {
    message: "Fase",
    description: "Agent sessions table column header — lifecycle phase chip",
  },
  "agentSessions.colPr": {
    message: "PR",
    description: "Agent sessions table column header — draft pull request",
  },
  "agentSessions.colCreated": {
    message: "Criada",
    description: "Agent sessions table column header — createdAt relative age",
  },
  "agentSessions.prBadge": {
    message: "#{number}",
    description: "Draft PR badge label — the pull-request number",
  },
  "agentSessions.prState.open": {
    message: "Aberto",
    description:
      "PR state badge beside a session's PR link — the pull request is open (a draft counts as open)",
  },
  "agentSessions.prState.merged": {
    message: "Mesclado",
    description:
      "PR state badge beside a session's PR link — the pull request was merged",
  },
  "agentSessions.prState.closed": {
    message: "Fechado",
    description:
      "PR state badge beside a session's PR link — the pull request was closed without merging",
  },
  "agentSessions.prAutoMerge": {
    message: "Mesclagem automática",
    description:
      "Badge beside a session's PR state — GitHub auto-merge is armed on the pull request (w10/047)",
  },
  "agentSessions.prMergeQueued": {
    message: "Na fila de mesclagem",
    description:
      "Badge beside a session's PR state — the pull request is in its base branch's merge queue (w10/047)",
  },
  "agentSessions.prMergeQueuedHint": {
    message: "Na fila de mesclagem: o GitHub a mescla quando chegar à frente.",
    description:
      "Tooltip of the merge-queue badge — what being in the merge queue means",
  },
  "agentSessions.prAutoMergeEnabledBy": {
    message: "Mesclagem automática ativada por {login}",
    description:
      "Tooltip of the auto-merge badge — the GitHub login that armed auto-merge",
  },
  "agentSessions.prAutoMergeEnabled": {
    message: "Mesclagem automática ativada",
    description:
      "Tooltip of the auto-merge badge when GitHub does not say who armed it",
  },
  "agentSessions.prAutoMergeHint": {
    message:
      "O GitHub a mescla quando as revisões e verificações obrigatórias passarem.",
    description: "Tooltip of the auto-merge badge — what auto-merge does",
  },
  "agentSessions.prAutoMergeMethod.merge": {
    message: "Criar um commit de mesclagem",
    description:
      "Tooltip of the auto-merge badge — the merge method, GitHub's 'Create a merge commit'",
  },
  "agentSessions.prAutoMergeMethod.squash": {
    message: "Squash e mesclar",
    description:
      "Tooltip of the auto-merge badge — the merge method, GitHub's 'Squash and merge'",
  },
  "agentSessions.prAutoMergeMethod.rebase": {
    message: "Rebase e mesclar",
    description:
      "Tooltip of the auto-merge badge — the merge method, GitHub's 'Rebase and merge'",
  },
  "agentSessions.prStateFilter": {
    message: "Filtrar por estado do pull request",
    description:
      "Session list — accessible name of the Any/Open/Merged/Closed PR-state filter",
  },
  "agentSessions.prStateFilterAny": {
    message: "Qualquer",
    description:
      "Session list PR-state filter — the option that shows sessions whatever their PR state, including sessions with no PR",
  },
  "agentSessions.errorTitle": {
    message: "Não foi possível carregar as sessões.",
    description: "Agent sessions list error — one sentence, Retry beside it",
  },
  "agentSessions.retry": {
    message: "Tentar novamente",
    description: "Retries a failed agent-sessions list request",
  },
  "agentSessions.emptyBody": {
    message: "As sessões que você iniciar aparecerão aqui.",
    description: "Default empty — one quiet line under the composer",
  },
  "agentSessions.emptyArchivedTitle": {
    message: "Nenhuma sessão arquivada",
    description: "Archived-only list empty-state heading",
  },
  "agentSessions.emptyArchivedBody": {
    message: "As sessões que você arquivar aparecerão aqui.",
    description: "Archived-only list empty-state body",
  },
  "agentSessions.emptyFilteredTitle": {
    message: "Nenhuma sessão correspondente",
    description: "Phase-filtered list empty-state heading",
  },
  "agentSessions.emptyFilteredBody": {
    message: "Tente outra fase ou limpe os filtros.",
    description: "Phase-filtered list empty-state body",
  },
  "agentSessions.clearFilters": {
    message: "Limpar filtros",
    description: "Clears agent-session membership and phase filters",
  },
  "agentSessions.phase.creating": {
    message: "Criando",
    description: "Agent session phase chip — sandbox is being created",
  },
  "agentSessions.phase.running": {
    message: "Em execução",
    description: "Agent session phase chip — a turn is in progress",
  },
  "agentSessions.phase.resuming": {
    message: "Retomando",
    description: "Agent session phase chip — session is resuming",
  },
  "agentSessions.phase.redispatching": {
    message: "Reenviando",
    description: "Agent session phase chip — session is being redispatched",
  },
  "agentSessions.phase.completed": {
    message: "Concluída",
    description: "Agent session phase chip — session finished successfully",
  },
  "agentSessions.phase.failed": {
    message: "Com falha",
    description: "Agent session phase chip — session failed",
  },
  "agentSessions.phase.canceling": {
    message: "Cancelando",
    description: "Agent session phase chip — cancellation in progress",
  },
  "agentSessions.phase.canceled": {
    message: "Cancelada",
    description: "Agent session phase chip — session was canceled",
  },
  "agentSessions.taskLabel": {
    message: "Tarefa",
    description: "Composer — the prominent task prompt textarea label",
  },
  "agentSessions.taskPlaceholder": {
    message: "Descreva uma tarefa de programação…",
    description: "Composer — task editor placeholder",
  },
  "agentSessions.mentionButton": {
    message: "Adicionar repositório ou sessão",
    description:
      "Composer toolbar — opens the repository/session mention picker",
  },
  "agentSessions.repoChip": {
    message: "Repositório {repo}",
    description: "Composer toolbar — accessible name of the selected-repo chip",
  },
  "agentSessions.keyboardHint": {
    message: "Enter para iniciar · Shift+Enter para uma nova linha",
    description: "Muted hint under the composer",
  },
  "agentSessions.connectGitHubTitle": {
    message: "Conecte o GitHub para começar",
    description: "Composer callout when the workspace has no App repos",
  },
  "agentSessions.connectGitHubBody": {
    message:
      "Os agentes na nuvem precisam de um repositório do GitHub conectado. Conecte o GitHub App deste workspace e depois volte para iniciar uma sessão.",
    description: "Composer callout body when there are no installation repos",
  },
  "agentSessions.connectGitHubSettings": {
    message: "Configurações do GitHub",
    description:
      "Secondary link from the composer's GitHub callout to Account Settings → Integrations (/settings#integrations), where the GitHub card lives",
  },
  "agentSessions.exampleFixTests": {
    message: "Corrija os testes que estão falhando e abra um PR de rascunho",
    description: "First-run example prompt chip",
  },
  "agentSessions.exampleAddReadme": {
    message: "Adicione um README que explique como executar o projeto",
    description: "First-run example prompt chip",
  },
  "agentSessions.chipRemove": {
    message: "Remover {name}",
    description: "Accessible label of a mention chip's remove button",
  },
  "agentSessions.mentionCategoryRepos": {
    message: "Repositórios",
    description: "Mention picker — the repositories category row",
  },
  "agentSessions.mentionCategoryReposDesc": {
    message: "Limite a sessão a um repositório do GitHub conectado",
    description: "Mention picker — repositories category one-line description",
  },
  "agentSessions.mentionCategorySessions": {
    message: "Sessões",
    description: "Mention picker — the prior-sessions category row",
  },
  "agentSessions.mentionCategorySessionsDesc": {
    message: "Use uma sessão de agente anterior como contexto",
    description: "Mention picker — sessions category one-line description",
  },
  "agentSessions.mentionNoResults": {
    message: "Nenhuma correspondência",
    description:
      "Mention picker — empty state when the fuzzy filter matches nothing",
  },
  "agentSessions.mentionReposEmpty": {
    message:
      "Ainda não há repositórios. Conecte uma conta do GitHub ou dê ao GitHub App do bex acesso a um repositório.",
    description:
      "Mention picker — empty state when no installation repos exist",
  },
  "agentSessions.mentionSessionsEmpty": {
    message: "Nenhuma sessão ainda",
    description:
      "Mention picker — empty state when the workspace has no sessions",
  },
  "agentSessions.mentionConnected": {
    message: "Conectado pelo GitHub App",
    description: "Mention picker readiness footer — the repo is app-connected",
  },
  "agentSessions.mentionDefaultBranch": {
    message: "Branch padrão: {branch}",
    description: "Mention picker readiness footer — the repo's default branch",
  },
  "agentSessions.agentLabel": {
    message: "Agente",
    description: "Composer — agent select label",
  },
  "agentSessions.modelLabel": {
    message: "Modelo",
    description: "Composer — model select label (w10/m516)",
  },
  "agentSessions.modelDefaultOption": {
    message: "{model} (padrão)",
    description: "Model list — the model a session runs when it names none",
  },
  "agentSessions.modelFixed": {
    message: "Este agente sempre usa {model}",
    description: "Tooltip of the model of an agent with one fixed model",
  },
  "agentSessions.metaModel": {
    message: "Modelo: {model}",
    description: "Session header — tooltip of the model the session runs",
  },
  "agentSessions.sandboxLabel": {
    message: "Sandbox",
    description: "Composer — sandbox template select label (w10/m519)",
  },
  "agentSessions.sandboxSize": {
    message: "{cpu} CPU, {memory} GB de memória",
    description:
      "Sandbox template size: CPU cores and memory in GB, numbers already formatted",
  },
  "agentSessions.sandboxOption": {
    message: "{name} · {size}",
    description: "Sandbox template choice: its name, then its size",
  },
  "agentSessions.sandboxDefaultOption": {
    message: "{template} (padrão)",
    description:
      "Sandbox list — the template a session runs on when it names none (the workspace default)",
  },
  "agentSessions.sandboxWorkspaceDefault": {
    message: "Padrão do workspace ({template})",
    description:
      "Automation sandbox choice — follow the workspace default at each run; {template} is the default now",
  },
  "agentSessions.metaSandbox": {
    message: "Sandbox: {template}",
    description:
      "Tooltip of the sandbox template a session runs on, with its size",
  },
  "agentSessions.githubCliInherit": {
    message: "Herdar",
    description:
      "GitHub CLI choice — inherit the level above, while what that resolves to is still loading (w10/m561)",
  },
  "agentSessions.githubCliInheritOn": {
    message: "Herdar (ativado)",
    description:
      "GitHub CLI choice — inherit the level above, which has gh on (w10/m561)",
  },
  "agentSessions.githubCliInheritOff": {
    message: "Herdar (desativado)",
    description:
      "GitHub CLI choice — inherit the level above, which has gh off (w10/m561)",
  },
  "agentSessions.githubCliOn": {
    message: "Ativado",
    description:
      "GitHub CLI choice — gh on for this session or automation (w10/m561)",
  },
  "agentSessions.githubCliOff": {
    message: "Desativado",
    description:
      "GitHub CLI choice — gh off for this session or automation (w10/m561)",
  },
  "agentSessions.githubCliOnMandatory": {
    message:
      "Indisponível enquanto a desativação do GitHub CLI for obrigatória neste workspace.",
    description:
      "Why the GitHub CLI On choice is disabled: the workspace's off is mandatory (w10/m561)",
  },
  "agentSessions.githubCliAdminOnly": {
    message: "Somente um administrador do workspace pode alterar isso.",
    description:
      "Why a GitHub CLI choice is disabled: only a workspace admin chooses or changes a session's or an automation's gh (w10/044)",
  },
  "agentSessions.githubCliTitle": {
    message: "GitHub CLI para esta sessão: {choice}",
    description:
      "Tooltip of the composer's GitHub CLI control; {choice} is the chosen binding, e.g. Inherit (on) (w10/m561)",
  },
  "agentSessions.errors.AGENT_SESSION_GITHUB_CLI_LOCKED": {
    message:
      "Neste workspace a desativação do GitHub CLI é obrigatória, então uma sessão não pode ativar o gh. Escolha “Desativado” ou “Herdar”.",
    description:
      "Mapped message for AGENT_SESSION_GITHUB_CLI_LOCKED with reason mandatory (w10/m561)",
  },
  "agentSessions.errors.AGENT_SESSION_GITHUB_CLI_LOCKED_adminRequired": {
    message:
      "Somente um administrador do workspace pode escolher o GitHub CLI de uma sessão. Escolha “Herdar”.",
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
    message: "Iniciar sessão",
    description: "Composer — submit button label",
  },
  "agentSessions.submitting": {
    message: "Iniciando…",
    description: "Composer — submit button label while the create is in flight",
  },
  "agentSessions.unavailableTitle": {
    message: "As sessões de agente não estão configuradas",
    description: "Composer — house callout title when the feature is 503",
  },
  "agentSessions.unavailableBody": {
    message:
      "Esta plataforma não ativou as sessões de agente na nuvem. Peça ao seu operador para configurar o gateway de sessões de agente.",
    description: "Composer — house callout body when the feature is 503",
  },
  "agentSessions.dependencyUnavailableTitle": {
    message: "As sessões de agente estão temporariamente indisponíveis",
    description:
      "Composer — retryable dependency-outage title, distinct from configuration",
  },
  "agentSessions.dependencyUnavailableBody": {
    message:
      "Uma dependência da plataforma está com problemas no momento. Sua configuração continua intacta; aguarde um instante e tente novamente.",
    description:
      "Composer — retryable dependency-outage body that does not send the user to an operator",
  },
  "agentSessions.modelKeyMissingTitle": {
    message: "Adicione primeiro uma chave de provedor de modelo",
    description:
      "Composer pre-flight title when the workspace has no model key",
  },
  "agentSessions.modelKeyMissingBody": {
    message:
      "Este workspace ainda não tem acesso a um modelo. Um administrador do workspace pode adicionar a chave de provedor de modelo própria do workspace em Configurações → Agentes e, depois, voltar aqui.",
    description:
      "Composer pre-flight body when the workspace has neither its own model key nor the bex-provided model (ADR091 D7)",
  },
  "agentSessions.modelKeyMissingAction": {
    message: "Abrir Configurações → Agentes",
    description:
      "Composer pre-flight link to the Settings → Agents model key field (/workspace/settings#agents)",
  },
  "agentSessions.createErrorTitle": {
    message: "Não foi possível iniciar a sessão",
    description: "Composer — error alert title when the create fails",
  },
  "agentSessions.detailTitle": {
    message: "Sessão",
    description: "Agent-session detail page document title",
  },
  "agentSessions.untitled": {
    message: "Sessão sem título",
    description:
      "Last-resort session name when a session carries neither a repository nor a task prompt",
  },
  "agentSessions.backToList": {
    message: "Todas as sessões",
    description: "Detail page — back link to the /agents list",
  },
  "agentSessions.detailErrorTitle": {
    message: "Não foi possível carregar esta sessão",
    description: "Detail page — heading when the session read fails",
  },
  "agentSessions.conversationTitle": {
    message: "Conversa",
    description: "Detail page — the live conversation column card title",
  },
  "agentSessions.failureTitle": {
    message: "A sessão falhou",
    description: "Detail page — heading over a failed session's reason",
  },
  "agentSessions.deliveryNoteTitle": {
    message: "Alterações não totalmente entregues",
    description:
      "Detail page — heading over a completed session's note that the agent left work uncommitted, unpushed or without a pull request",
  },
  "agentSessions.deliveryNoteHint": {
    message:
      "Envie uma mensagem de acompanhamento pedindo ao agente que termine de fazer commit, push e atualizar o pull request.",
    description:
      "Detail page — the next step shown under an undelivered-work note",
  },
  "agentSessions.failureReasonFallback": {
    message: "A sessão falhou ao iniciar.",
    description:
      "Detail page — generic failure text when the session carries no specific reason",
  },
  "agentSessions.failureRetry": {
    message: "Tentar novamente",
    description:
      "Detail page — button to re-run a failed session's original task",
  },
  "agentSessions.failureRetrying": {
    message: "Tentando novamente…",
    description:
      "Detail page — retry button label while the redispatch is in flight",
  },
  "agentSessions.capacityFailureTitle": {
    message: "Capacidade de sandbox esgotada",
    description:
      "Detail page — heading when a session failed on the plan's sandbox limit",
  },
  "agentSessions.capacityFailureBody": {
    message:
      "Este workspace atingiu o limite de sandboxes simultâneos do plano. Faça upgrade do plano para executar mais ao mesmo tempo, ou pare uma sessão ociosa e tente novamente.",
    description:
      "Detail page — explains a plan-limit sandbox failure and the two remedies",
  },
  "agentSessions.upgradePlan": {
    message: "Fazer upgrade do plano",
    description:
      "Detail page — CTA on a capacity failure; opens the change-plan dialog",
  },
  "agentSessions.failureCause.pool_full": {
    message: "Nenhuma máquina de sandbox tinha espaço",
    description:
      "Failed-start cause line (session callout, automation runs) — no sandbox machine had room for the session's machine for the whole wait (w10/m497)",
  },
  "agentSessions.failureCause.image_pull": {
    message: "Não foi possível baixar a imagem do sandbox",
    description:
      "Failed-start cause line — the sandbox's container image did not pull (w10/m497)",
  },
  "agentSessions.failureCause.runtime": {
    message: "O runtime do sandbox não conseguiu iniciar",
    description:
      "Failed-start cause line — the sandbox runtime did not start the session's machine (w10/m497)",
  },
  "agentSessions.failureCause.quota": {
    message: "A cota de sandboxes do workspace recusou a máquina",
    description:
      "Failed-start cause line — the workspace's sandbox quota refused the session's machine (w10/m497)",
  },
  "agentSessions.failureCause.unknown": {
    message: "Problema na plataforma",
    description:
      "Failed-start cause line — the sandbox did not start for a cause on bex's side that bex-api could not name (w10/m497)",
  },
  "agentSessions.provisioning": {
    message: "Iniciando o sandbox…",
    description:
      "Detail page — shown while a new/steered session provisions its sandbox before the conversation stream is available",
  },
  "agentSessions.metaDuration": {
    message: "Duração {duration}",
    description: "Detail header — elapsed session wall-clock",
  },
  "agentSessions.metaTurns_one": {
    message: "{count} turno",
    description: "Detail header — number of prompt turns taken (singular)",
  },
  "agentSessions.metaTurns_many": {
    message: "{count} de turnos",
    description: "Detail header — number of prompt turns taken (plural)",
  },
  "agentSessions.metaTurns_other": {
    message: "{count} turnos",
    description: "Detail header — number of prompt turns taken (plural)",
  },
  "agentSessions.cancel": {
    message: "Cancelar",
    description: "Detail header — cancel-session button label",
  },
  "agentSessions.cancelDisabledCanceling": {
    message: "O cancelamento já está em andamento.",
    description: "Detail header — tooltip on the disabled cancel button",
  },
  "agentSessions.canceling": {
    message: "Cancelando…",
    description: "Detail header — confirm button label while the cancel runs",
  },
  "agentSessions.cancelSuccess": {
    message: "Cancelando a sessão…",
    description: "Detail header — toast after the cancel is accepted",
  },
  "agentSessions.cancelConfirmTitle": {
    message: "Cancelar esta sessão?",
    description: "Cancel confirm dialog title",
  },
  "agentSessions.cancelConfirmBody": {
    message:
      "Isso interrompe o agente. Os commits já enviados à branch e o PR são preservados.",
    description: "Cancel confirm dialog body — states pushed work is preserved",
  },
  "agentSessions.cancelConfirmDismiss": {
    message: "Continuar executando",
    description: "Cancel confirm dialog — dismiss button",
  },
  "agentSessions.cancelConfirmProceed": {
    message: "Cancelar sessão",
    description: "Cancel confirm dialog — proceed button",
  },
  "agentSessions.stopTurn": {
    message: "Parar turno",
    description:
      "Detail header — Stop turn button label: ends only the running turn, unlike Cancel which ends the session",
  },
  "agentSessions.stoppingTurn": {
    message: "Parando…",
    description: "Detail header — Stop turn button label while the stop runs",
  },
  "agentSessions.stopTurnHint": {
    message:
      "Encerra apenas o turno em execução. A sessão continua aberta e as mensagens na fila mantêm o lugar delas. Atalho: Ctrl/Cmd+Shift+Backspace.",
    description: "Detail header — tooltip on the Stop turn button",
  },
  "agentSessions.stopTurnStarting": {
    message:
      "O turno ainda está iniciando. Você poderá pará-lo quando ele estiver em execução.",
    description:
      "Detail header — tooltip on the disabled Stop turn button while the sandbox starts",
  },
  "agentSessions.stopTurnSuccess": {
    message: "Parando o turno. Ele termina em alguns segundos.",
    description: "Detail header — toast after Stop turn is accepted",
  },
  "agentSessions.steerTitle": {
    message: "Direcionar esta sessão",
    description: "Steering composer card title",
  },
  "agentSessions.steerErrorTitle": {
    message: "Não foi possível enviar",
    description: "Steering composer — inline error alert title",
  },
  "agentSessions.steerPlaceholderIdle": {
    message:
      "Envie uma instrução de acompanhamento para iniciar um novo turno na mesma branch.",
    description: "Steering composer — textarea placeholder for an idle session",
  },
  "agentSessions.steerPlaceholderQueue": {
    message:
      "Coloque uma mensagem na fila. Ela é executada quando o turno atual terminar.",
    description:
      "Steering composer — textarea placeholder while a turn runs or messages wait (ADR047 D12)",
  },
  "agentSessions.steerHintIdle": {
    message:
      "Esta sessão está ociosa — enviar uma mensagem continua a conversa em um sandbox novo, na mesma branch.",
    description: "Steering composer — helper text for the redispatch route",
  },
  "agentSessions.steerHintQueue": {
    message:
      "O agente está trabalhando. Sua mensagem fica na fila e é executada como um turno próprio quando este terminar.",
    description:
      "Steering composer — helper text while a turn runs or messages wait (ADR047 D12)",
  },
  "agentSessions.steerDisabledCanceling": {
    message:
      "Esta sessão está sendo cancelada. O trabalho já enviado é preservado.",
    description: "Steering composer — disabled reason while canceling",
  },
  "agentSessions.steerDisabledCanceled": {
    message: "Esta sessão foi cancelada.",
    description: "Steering composer — disabled reason once canceled",
  },
  "agentSessions.steerDisabledStream": {
    message:
      "O fluxo da conversa está indisponível, então o direcionamento ao vivo está pausado.",
    description:
      "Steering composer — disabled reason when the m43 stream is down",
  },
  "agentSessions.steerSubmit": {
    message: "Enviar",
    description: "Steering composer — submit button label",
  },
  "agentSessions.steerSending": {
    message: "Enviando…",
    description: "Steering composer — submit button label while sending",
  },
  "agentSessions.steerSuccess": {
    message: "Enviado — reenviando um novo turno.",
    description: "Steering composer — toast after a redispatch is accepted",
  },
  "agentSessions.steerQueued": {
    message: "Na fila. Será executada quando o turno atual terminar.",
    description:
      "Steering composer — toast after a message was queued behind a running turn (ADR047 D12)",
  },
  "agentSessions.queueRegion": {
    message: "Mensagens na fila",
    description: "Queued messages list — accessible name of the region",
  },
  "agentSessions.queueTitle_one": {
    message: "{count} mensagem na fila",
    description: "Queued messages list — heading with the count (singular)",
  },
  "agentSessions.queueTitle_many": {
    message: "{count} de mensagens na fila",
    description: "Queued messages list — heading with the count (plural)",
  },
  "agentSessions.queueTitle_other": {
    message: "{count} mensagens na fila",
    description: "Queued messages list — heading with the count (plural)",
  },
  "agentSessions.queueHint": {
    message:
      "Elas são executadas uma de cada vez, nesta ordem, depois do turno atual.",
    description:
      "Queued messages list — one-line explanation next to the heading",
  },
  "agentSessions.queuePosition": {
    message: "Posição {position} na fila",
    description: "Queued messages list — tooltip on a row's place number",
  },
  "agentSessions.queueAttachments_one": {
    message: "{count} anexo",
    description:
      "Queued message row — attachments that ride with it (singular)",
  },
  "agentSessions.queueAttachments_many": {
    message: "{count} de anexos",
    description: "Queued message row — attachments that ride with it (plural)",
  },
  "agentSessions.queueAttachments_other": {
    message: "{count} anexos",
    description: "Queued message row — attachments that ride with it (plural)",
  },
  "agentSessions.queueBlocked": {
    message:
      "Não foi possível iniciar ainda: {reason}. Uma nova tentativa é feita automaticamente.",
    description:
      "Queued message row — why the dispatcher could not start it (a spend ceiling or the live-sandbox limit)",
  },
  "agentSessions.queueModel": {
    message: "Muda a sessão para {model}",
    description:
      "Queued message — the model its turn switches the session to (w10/m516)",
  },
  "agentSessions.queueSendNow": {
    message: "Enviar agora",
    description:
      "Queued message row — stops the running turn and runs this message next",
  },
  "agentSessions.queueMoveUp": {
    message: "Mover para cima",
    description: "Queued message row — move one place earlier",
  },
  "agentSessions.queueMoveDown": {
    message: "Mover para baixo",
    description: "Queued message row — move one place later",
  },
  "agentSessions.queueEdit": {
    message: "Editar",
    description: "Queued message row — edit the message in place",
  },
  "agentSessions.queueRemove": {
    message: "Remover",
    description: "Queued message row — remove it so it never runs",
  },
  "agentSessions.queueEditLabel": {
    message: "Editar mensagem na fila",
    description: "Queued message row — accessible name of the edit box",
  },
  "agentSessions.queueEditSave": {
    message: "Salvar",
    description: "Queued message row — save the edit",
  },
  "agentSessions.queueEditSaveSendNow": {
    message: "Salvar e enviar agora",
    description:
      "Queued message row — save the edit, stop the running turn and run this message next",
  },
  "agentSessions.queueEditDiscard": {
    message: "Descartar",
    description: "Queued message row — leave the edit without saving",
  },
  "agentSessions.queueEditHint": {
    message:
      "Enter salva, Shift+Enter adiciona uma linha, Ctrl/Cmd+Enter salva e envia agora, Esc descarta.",
    description: "Queued message row — keyboard hints while editing",
  },
  "agentSessions.queueSendNowStopping": {
    message: "Parando o turno atual. Esta mensagem será executada em seguida.",
    description: "Queued message row — toast after Send now stopped a turn",
  },
  "agentSessions.queueSendNowNext": {
    message: "Esta mensagem será executada em seguida.",
    description:
      "Queued message row — toast after Send now when no turn was running",
  },
  "agentSessions.errors.AGENT_SESSION_INPUT_INVALID": {
    message:
      "Essa entrada não é válida. Verifique o repositório, a branch e a tarefa e tente novamente.",
    description: "Mapped message for AGENT_SESSION_INPUT_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_ID_INVALID": {
    message: "Esse ID de sessão não é válido.",
    description: "Mapped message for AGENT_SESSION_ID_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_FOUND": {
    message: "Essa sessão não existe mais.",
    description: "Mapped message for AGENT_SESSION_NOT_FOUND",
  },
  "agentSessions.errors.AGENT_SESSION_CONFLICT": {
    message:
      "Esta sessão está ocupada com outra operação. Tente novamente em instantes.",
    description: "Mapped message for AGENT_SESSION_CONFLICT",
  },
  "agentSessions.errors.AGENT_SESSION_BRANCH_IN_USE": {
    message:
      "Outra sessão já está trabalhando nesta branch. Envie uma mensagem a essa sessão ou use outra branch bex-agent/.",
    description: "Mapped message for AGENT_SESSION_BRANCH_IN_USE (w5/m382)",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_STEERABLE": {
    message: "Esta sessão não pode ser direcionada na fase atual ({phase}).",
    description: "Mapped message for AGENT_SESSION_NOT_STEERABLE",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_RESUMABLE": {
    message: "Esta sessão não pode ser retomada na fase atual ({phase}).",
    description: "Mapped message for AGENT_SESSION_NOT_RESUMABLE",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_ATTACHABLE": {
    message:
      "Não é possível se conectar a esta sessão na fase atual ({phase}).",
    description: "Mapped message for AGENT_SESSION_NOT_ATTACHABLE",
  },
  "agentSessions.errors.AGENT_SESSION_TURN_IN_FLIGHT": {
    message:
      "Já há um turno em execução. Aguarde ele terminar antes de enviar outro.",
    description: "Mapped message for AGENT_SESSION_TURN_IN_FLIGHT",
  },
  "agentSessions.errors.AGENT_SESSION_QUEUE_FULL": {
    message:
      "Esta sessão já tem {max} mensagens na fila. Aguarde uma delas ser executada ou remova uma e envie novamente.",
    description: "Mapped message for AGENT_SESSION_QUEUE_FULL (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_QUEUE_INLINE_SECRETS": {
    message:
      "Uma mensagem com segredos inline não pode esperar na fila, porque os valores deles nunca são armazenados. Envie-a quando a sessão estiver ociosa ou use um segredo do workspace.",
    description:
      "Mapped message for AGENT_SESSION_QUEUE_INLINE_SECRETS (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_QUEUED_MESSAGE_DISPATCHED": {
    message:
      "Essa mensagem já começou a ser executada e não pode mais ser alterada.",
    description:
      "Mapped message for AGENT_QUEUED_MESSAGE_DISPATCHED (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_QUEUED_MESSAGE_NOT_FOUND": {
    message: "Essa mensagem não está mais na fila.",
    description:
      "Mapped message for AGENT_QUEUED_MESSAGE_NOT_FOUND (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_QUEUED_MESSAGE_ATTACHMENTS_OWNED": {
    message:
      "Esta mensagem carrega os anexos de quem a enviou, então somente essa pessoa pode editá-la. Você ainda pode movê-la ou removê-la.",
    description:
      "Mapped message for AGENT_QUEUED_MESSAGE_ATTACHMENTS_OWNED (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_NO_TURN_RUNNING": {
    message: "Nenhum turno está em execução, então não há nada para parar.",
    description:
      "Mapped message for AGENT_SESSION_NO_TURN_RUNNING (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_TURN_STARTING": {
    message:
      "O turno ainda está iniciando, então ainda não há nada para parar. Tente novamente em alguns segundos.",
    description: "Mapped message for AGENT_SESSION_TURN_STARTING (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_MODEL_ENDPOINT_INVALID": {
    message: "O endpoint do modelo deve ser uma URL HTTPS válida.",
    description: "Mapped message for AGENT_SESSION_MODEL_ENDPOINT_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_MODEL_NOT_OFFERED": {
    message:
      "O modelo da plataforma deste workspace não oferece {model}. Escolha um na lista de modelos.",
    description:
      "Mapped message for AGENT_SESSION_MODEL_NOT_OFFERED (w10/m516)",
  },
  "agentSessions.errors.AGENT_SESSION_MODEL_PINNED": {
    message: "Este agente usa apenas {pinnedModel}.",
    description: "Mapped message for AGENT_SESSION_MODEL_PINNED",
  },
  "agentSessions.errors.AGENT_SESSION_EGRESS_ALLOWLIST_INVALID": {
    message:
      'A entrada "{entry}" da lista de permissões de saída é inválida: {reason}',
    description: "Mapped message for AGENT_SESSION_EGRESS_ALLOWLIST_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_EGRESS_ALLOWLIST_IMMUTABLE": {
    message:
      "A lista de permissões de saída não pode ser alterada depois que a sessão inicia.",
    description: "Mapped message for AGENT_SESSION_EGRESS_ALLOWLIST_IMMUTABLE",
  },
  "agentSessions.errors.AGENT_SESSION_EGRESS_PHASE_INVALID": {
    message:
      "A lista de permissões de saída não pode ser definida na fase atual desta sessão.",
    description: "Mapped message for AGENT_SESSION_EGRESS_PHASE_INVALID",
  },
  "agentSessions.conversationLoading": {
    message: "Carregando conversa…",
    description: "Placeholder while the client-only conversation column loads",
  },
  "agentSessions.conversationConnecting": {
    message: "Conectando ao fluxo da sessão…",
    description: "Shown while the conversation stream replay is in flight",
  },
  "agentSessions.conversationNotStarted": {
    message: "A conversa aparecerá quando o sandbox estiver pronto.",
    description:
      "Healthy pre-stream state while a newly created session is provisioning",
  },
  "agentSessions.conversationEmpty": {
    message: "Nenhuma conversa ainda.",
    description: "Shown when a session has produced no transcript parts",
  },
  "agentSessions.turnInterrupted": {
    message: "O turno {turn} foi interrompido antes de terminar.",
    description:
      "Transcript line at the end of a turn a person stopped with Stop turn (ADR047 D12); not a failure",
  },
  "agentSessions.turnRetentionSaving": {
    message:
      "Salvando suas alterações para que a próxima mensagem possa continuar a partir delas.",
    description:
      "Transcript line under a stopped turn while its unsaved work is being saved so the next message can continue from it (ADR047 D12, w22/m413)",
  },
  "agentSessions.turnRetentionKept": {
    message: "Suas alterações são mantidas até {time}.",
    description:
      "Transcript line under a stopped turn: its unpushed changes are kept until {time}, a date and time in the viewer's own zone (ADR047 D12, w22/m413)",
  },
  "agentSessions.turnRetentionKeptNoTime": {
    message: "Suas alterações são mantidas para a sua próxima mensagem.",
    description:
      "Same as the kept line before the browser knows the viewer's time zone, so it names no time",
  },
  "agentSessions.turnRetentionContinued": {
    message: "A próxima mensagem continuou a partir destas alterações.",
    description:
      "Transcript line under a stopped turn: the next message started from its kept changes",
  },
  "agentSessions.turnRetentionLostExpired": {
    message:
      "Suas alterações sem push foram perdidas quando o sandbox mantido expirou.",
    description:
      "Transcript line under a stopped turn: its kept sandbox expired and the unpushed changes in it are gone",
  },
  "agentSessions.turnRetentionLostStorageFull": {
    message:
      "Suas alterações sem push não foram mantidas porque o armazenamento de sandbox do workspace estava cheio.",
    description:
      "Transcript line under a stopped turn: nothing was kept because the workspace's sandbox storage cap was full",
  },
  "agentSessions.turnRetentionLostSaveFailed": {
    message:
      "Não foi possível salvar suas alterações sem push, e elas foram perdidas.",
    description:
      "Transcript line under a stopped turn: saving its sandbox failed, so the unpushed changes are gone",
  },
  "agentSessions.turnRetentionLostEditorOpen": {
    message:
      "Suas alterações sem push não foram mantidas porque havia um editor aberto no sandbox.",
    description:
      "Transcript line under a stopped turn: nothing was kept because an editor (SSH) was open on the sandbox, which Stop does not end",
  },
  "agentSessions.turnRetentionLostNotRestored": {
    message:
      "Não foi possível restaurar suas alterações sem push, então a próxima mensagem começou de um checkout novo.",
    description:
      "Transcript line under a stopped turn: the next message could not start from the kept changes and started from a fresh checkout",
  },
  "agentSessions.turnRetentionLost": {
    message: "Suas alterações sem push foram perdidas.",
    description:
      "Transcript line under a stopped turn: its unpushed changes were lost, for a reason that is not worth naming (the session was canceled or archived)",
  },
  "agentSessions.turnRetentionLostSandboxLost": {
    message:
      "Suas alterações sem push foram perdidas quando o turno falhou e seu sandbox já não existia.",
    description:
      "Transcript line under a failed turn: its sandbox was gone before its disk could be kept, so the unpushed work in it is lost (w10/m491)",
  },
  "agentSessions.turnStakeCommits_one": {
    message: "{count} commit sem push",
    description:
      "Under a turn whose disk was kept or lost: how many commits its checkout held that were not pushed (w10/m491)",
  },
  "agentSessions.turnStakeCommits_many": {
    message: "{count} de commits sem push",
    description:
      "Under a turn whose disk was kept or lost: how many commits its checkout held that were not pushed (w10/m491)",
  },
  "agentSessions.turnStakeCommits_other": {
    message: "{count} commits sem push",
    description:
      "Under a turn whose disk was kept or lost: how many commits its checkout held that were not pushed (w10/m491)",
  },
  "agentSessions.turnStakeChanges": {
    message: "alterações não confirmadas",
    description:
      "Under a turn whose disk was kept or lost: its checkout held uncommitted changes (w10/m491)",
  },
  "agentSessions.turnError": {
    message: "O turno {turn} falhou",
    description:
      "Recorded failure of an earlier turn; subsequent turns remain readable",
  },
  "agentSessions.continuitySessionLoad": {
    message: "O agente foi retomado com o estado da conversa restaurado.",
    description:
      "System line when a fresh agent generation reloaded its own saved session (ADR047 D3 ladder rung 1)",
  },
  "agentSessions.continuityReprime": {
    message:
      "O agente foi reiniciado — o contexto foi reconstruído a partir do histórico da sessão.",
    description:
      "System line when a fresh agent generation was primed from the durable transcript (ladder rung 2)",
  },
  "agentSessions.continuityTaskRedelivery": {
    message: "O agente foi reiniciado — a tarefa original foi reenviada.",
    description:
      "System line when no prior turn ever reached the agent, so the task was re-sent (ladder rung 3)",
  },
  "agentSessions.conversationEndedEmpty": {
    message: "Esta sessão terminou antes de uma conversa ser registrada.",
    description: "Ended session with no durable transcript parts",
  },
  "agentSessions.conversationEnded": {
    message: "Sessão encerrada.",
    description: "Footer note under a terminal session's replayed transcript",
  },
  "agentSessions.conversationIncomplete": {
    message: "Parte da saída do assistente não pôde ser preservada",
    description:
      "Warning below a durable user turn whose assistant transcript is incomplete",
  },
  "agentSessions.conversationUnavailable": {
    message: "O fluxo da conversa está indisponível no momento.",
    description:
      "Degraded-state message when the m43 stream endpoint errors or is unconfigured",
  },
  "agentSessions.conversationUnavailableTerminal": {
    message: "A transcrição da conversa não está disponível para esta sessão.",
    description:
      "Terminal session — transcript unavailable rather than live stream degraded",
  },
  "agentSessions.showEarlierMessages_one": {
    message: "Mostrar {count} mensagem anterior",
    description:
      "Button revealing older transcript messages hidden by the render window (singular)",
  },
  "agentSessions.showEarlierMessages_many": {
    message: "Mostrar {count} de mensagens anteriores",
    description:
      "Button revealing older transcript messages hidden by the render window (plural)",
  },
  "agentSessions.showEarlierMessages_other": {
    message: "Mostrar {count} mensagens anteriores",
    description:
      "Button revealing older transcript messages hidden by the render window (plural)",
  },
  "agentSessions.groupThought": {
    message: "Pensamento",
    description: "Collapsible header for the agent's reasoning group",
  },
  "agentSessions.groupPlan": {
    message: "Plano",
    description: "Collapsible header for the agent's plan/task checklist group",
  },
  "agentSessions.groupTerminal": {
    message: "Terminal",
    description: "Collapsible header for a terminal-output group",
  },
  "agentSessions.groupCommand": {
    message: "Comando",
    description: "Collapsible header for a command/tool-call group",
  },
  "agentSessions.groupDiff": {
    message: "Diff",
    description: "Label for a file-diff group when no path is given",
  },
  "agentSessions.terminalNoOutput": {
    message: "(sem saída)",
    description: "Placeholder when a terminal group has no captured output",
  },
  "agentSessions.toolInput": {
    message: "Entrada",
    description: "Section label for a tool call's input",
  },
  "agentSessions.toolOutput": {
    message: "Saída",
    description: "Section label for a tool call's output",
  },
  "agentSessions.toolError": {
    message: "Erro",
    description: "Section label for a tool call's error text",
  },
  "agentSessions.toolStateRunning": {
    message: "Em execução",
    description: "Badge label for a tool call awaiting output",
  },
  "agentSessions.toolStateDone": {
    message: "Concluído",
    description: "Badge label for a completed tool call",
  },
  "agentSessions.toolStateDenied": {
    message: "Negado",
    description: "Badge label for a tool call whose execution was denied",
  },
  "agentSessions.toolStateInterrupted": {
    message: "Interrompido",
    description:
      "Badge label for a tool call the session ended before it finished",
  },
  "agentSessions.toolStateError": {
    message: "Com falha",
    description: "Badge label for a failed tool call",
  },
  "agentSessions.activityEdited_one": {
    message: "Editou {count} arquivo",
    description:
      "Activity-group summary when the turn edited files (diffs, singular)",
  },
  "agentSessions.activityEdited_many": {
    message: "Editou {count} de arquivos",
    description: "Activity-group summary when the turn edited files (diffs)",
  },
  "agentSessions.activityEdited_other": {
    message: "Editou {count} arquivos",
    description: "Activity-group summary when the turn edited files (diffs)",
  },
  "agentSessions.activityRan_one": {
    message: "Executou {count} comando",
    description:
      "Activity-group summary when the turn ran commands/terminals (singular)",
  },
  "agentSessions.activityRan_many": {
    message: "Executou {count} de comandos",
    description: "Activity-group summary when the turn ran commands/terminals",
  },
  "agentSessions.activityRan_other": {
    message: "Executou {count} comandos",
    description: "Activity-group summary when the turn ran commands/terminals",
  },
  "agentSessions.activitySteps_one": {
    message: "{count} etapa",
    description: "Activity-group summary fallback — the step count (singular)",
  },
  "agentSessions.activitySteps_many": {
    message: "{count} de etapas",
    description: "Activity-group summary fallback — the step count (plural)",
  },
  "agentSessions.activitySteps_other": {
    message: "{count} etapas",
    description: "Activity-group summary fallback — the step count (plural)",
  },
  "agentSessions.activityRead_one": {
    message: "Leu {count} arquivo",
    description: "Activity-group summary segment — file reads (singular)",
  },
  "agentSessions.activityRead_many": {
    message: "Leu {count} de arquivos",
    description: "Activity-group summary segment — file reads",
  },
  "agentSessions.activityRead_other": {
    message: "Leu {count} arquivos",
    description: "Activity-group summary segment — file reads",
  },
  "agentSessions.activitySearched_one": {
    message: "Pesquisou {count} vez",
    description: "Activity-group summary segment — searches (singular)",
  },
  "agentSessions.activitySearched_many": {
    message: "Pesquisou {count} de vezes",
    description: "Activity-group summary segment — searches",
  },
  "agentSessions.activitySearched_other": {
    message: "Pesquisou {count} vezes",
    description: "Activity-group summary segment — searches",
  },
  "agentSessions.activitySubagents_one": {
    message: "Executou {count} subagente",
    description:
      "Activity-group summary segment — delegated subagent tasks (singular)",
  },
  "agentSessions.activitySubagents_many": {
    message: "Executou {count} de subagentes",
    description: "Activity-group summary segment — delegated subagent tasks",
  },
  "agentSessions.activitySubagents_other": {
    message: "Executou {count} subagentes",
    description: "Activity-group summary segment — delegated subagent tasks",
  },
  "agentSessions.activityTools_one": {
    message: "Usou {count} ferramenta",
    description:
      "Activity-group summary segment — tools of no known kind (singular)",
  },
  "agentSessions.activityTools_many": {
    message: "Usou {count} de ferramentas",
    description: "Activity-group summary segment — tools of no known kind",
  },
  "agentSessions.activityTools_other": {
    message: "Usou {count} ferramentas",
    description: "Activity-group summary segment — tools of no known kind",
  },
  "agentSessions.activityThought_one": {
    message: "Pensou {count} vez",
    description:
      "Activity-group summary segment — the agent's thoughts between its calls (singular)",
  },
  "agentSessions.activityThought_many": {
    message: "Pensou {count} de vezes",
    description:
      "Activity-group summary segment — the agent's thoughts between its calls",
  },
  "agentSessions.activityThought_other": {
    message: "Pensou {count} vezes",
    description:
      "Activity-group summary segment — the agent's thoughts between its calls",
  },
  "agentSessions.activityThinking": {
    message: "Pensando…",
    description: "Live status line while the agent is thinking between calls",
  },
  "agentSessions.activityFailed_one": {
    message: "{count} com falha",
    description:
      "Activity-group summary segment — failed tool calls (singular)",
  },
  "agentSessions.activityFailed_many": {
    message: "{count} com falha",
    description: "Activity-group summary segment — failed tool calls",
  },
  "agentSessions.activityFailed_other": {
    message: "{count} com falha",
    description: "Activity-group summary segment — failed tool calls",
  },
  "agentSessions.outputClipped_one": {
    message: "{formatted} caractere cortado",
    description:
      "Badge on a tool output the server stored head+tail-capped (singular)",
  },
  "agentSessions.outputClipped_many": {
    message: "{formatted} de caracteres cortados",
    description: "Badge on a tool output the server stored head+tail-capped",
  },
  "agentSessions.outputClipped_other": {
    message: "{formatted} caracteres cortados",
    description: "Badge on a tool output the server stored head+tail-capped",
  },
  "agentSessions.showFullOutput": {
    message: "Mostrar saída completa",
    description:
      "Button on a clipped tool output that loads the complete text the server kept",
  },
  "agentSessions.hideFullOutput": {
    message: "Ocultar saída completa",
    description: "Button that collapses a loaded full tool output",
  },
  "agentSessions.fullOutputNotRetained": {
    message: "Saída completa não retida",
    description:
      "Note on a clipped tool output whose complete text the server did not keep (clipped before full copies were stored)",
  },
  "agentSessions.fullOutputLoading": {
    message: "Carregando saída completa…",
    description: "Status while a tool call's full output loads",
  },
  "agentSessions.fullOutputError": {
    message: "Não foi possível carregar a saída completa.",
    description: "Inline error when loading a tool call's full output failed",
  },
  "agentSessions.fullOutputCopy": {
    message: "Copiar saída completa",
    description:
      "Accessible label of the copy button on a loaded full tool output",
  },
  "agentSessions.fullOutputCopied": {
    message: "Saída completa copiada",
    description: "Toast after copying a full tool output",
  },
  "agentSessions.fullOutputCopyError": {
    message: "Não foi possível copiar a saída completa",
    description: "Toast when copying a full tool output failed",
  },
  "agentSessions.conversationPartial": {
    message: "Conversa carregada apenas parcialmente.",
    description:
      "Inline row when the transcript stream failed after some of it rendered",
  },
  "agentSessions.liveStatus": {
    message: "Status do agente",
    description:
      "Accessible label for the single live status line under a streaming transcript",
  },
  "agentSessions.scrollToBottom": {
    message: "Rolar até a mais recente",
    description:
      "Accessible label for the floating jump-to-bottom button in the conversation column",
  },
  "agentSessions.recentSessions": {
    message: "Recentes",
    description: "Sessions sidebar — heading over the recent sessions list",
  },
  "agentSessions.sidebarEmpty": {
    message: "Nenhuma sessão ainda",
    description: "Sessions sidebar — empty state when the workspace has none",
  },
  "agentSessions.sidebarLabel": {
    message: "Sessões de agente",
    description: "Sessions sidebar — accessible landmark label",
  },
  "agentSessions.sidebarSearch": {
    message: "Pesquisar sessões",
    description: "Sessions sidebar — search toggle + input accessible label",
  },
  "agentSessions.sidebarSearchPlaceholder": {
    message: "Pesquisar sessões…",
    description: "Sessions sidebar — search input placeholder",
  },
  "agentSessions.sidebarNoMatches": {
    message: "Nenhuma sessão correspondente",
    description:
      "Sessions sidebar — empty state when the search matches nothing",
  },
  "agentSessions.statusPhrase.prReady": {
    message: "PR pronto",
    description: "Sidebar status phrase — completed session with a draft PR",
  },
  "agentSessions.statusPhrase.working": {
    message: "Trabalhando…",
    description: "Sidebar status phrase — session still converging",
  },
  "agentSessions.statusPhrase.waitingForSandbox": {
    message: "Aguardando uma máquina de sandbox",
    description:
      "Session status (header, list rows, sidebar, conversation, automation runs) — the turn waits because no sandbox machine has room for it yet (w10/m497); the time waited follows where there is room",
  },
  "agentSessions.menuMore": {
    message: "Mais ações",
    description: "Header — accessible label for the '…' overflow menu",
  },
  "agentSessions.openPr": {
    message: "Abrir pull request",
    description: "Header overflow menu — open the draft PR in a new tab",
  },
  "agentSessions.connect": {
    message: "Conectar",
    description: "Header — trigger for the Open-in-Zed / SSH connect menu",
  },
  "agentSessions.openInZed": {
    message: "Abrir no Zed",
    description:
      "Connect menu — hotlink that opens the sandbox as a Zed remote project",
  },
  "agentSessions.openInZedHint": {
    message:
      "Abre o /workspace do sandbox por SSH. Requer o editor Zed. Um sandbox fechado é iniciado quando você se conecta.",
    description:
      "Connect menu — helper text under the Open-in-Zed action; a closed sandbox starts when the editor or ssh connects (ADR054 D9)",
  },
  "agentSessions.connectSSH": {
    message: "SSH",
    description: "Connect menu — label above the copyable ssh command",
  },
  "agentSessions.sshCopy": {
    message: "Copiar comando SSH",
    description: "Connect menu — accessible label for the copy button",
  },
  "agentSessions.sshCopied": {
    message: "Comando SSH copiado",
    description: "Connect menu — toast after copying the ssh command",
  },
  "agentSessions.sshCopyError": {
    message: "Não foi possível copiar",
    description: "Connect menu — toast when copying the ssh command fails",
  },
  "agentSessions.groupThoughtFor": {
    message: "Pensou por {duration}",
    description:
      "Reasoning-group summary with elapsed duration from persisted part timestamps (or ~arrival timing)",
  },
  "agentSessions.terminalStatus.completed": {
    message: "O agente terminou",
    description: "Terminal transcript status line — completed session",
  },
  "agentSessions.terminalStatus.failed": {
    message: "A sessão terminou com um erro",
    description: "Terminal transcript status line — failed session",
  },
  "agentSessions.terminalStatus.canceled": {
    message: "A sessão foi cancelada",
    description: "Terminal transcript status line — canceled session",
  },
  "agentSessions.tagsLabel": {
    message: "Tags",
    description: "Accessible label for a session row's tag list (w21/002)",
  },
  "agentSessions.filterByTag": {
    message: "Mostrar sessões com a tag {tag}",
    description:
      "Accessible label of a tag chip that filters the session list to that tag",
  },
  "agentSessions.filterTagged": {
    message: "Com a tag {tag}",
    description: "Session list heading while narrowed to one tag",
  },
  "agentSessions.archive": {
    message: "Arquivar",
    description: "Action putting a session out of the working set (ADR065)",
  },
  "agentSessions.unarchive": {
    message: "Desarquivar",
    description: "Action returning an archived session to the working set",
  },
  "agentSessions.archiveSuccess": {
    message: "Sessão arquivada",
    description: "Toast after archiving a session",
  },
  "agentSessions.undoArchive": {
    message: "Desfazer",
    description: "Toast action that immediately unarchives a session",
  },
  "agentSessions.unarchiveSuccess": {
    message: "Sessão desarquivada",
    description: "Toast after unarchiving a session",
  },
  "agentSessions.archivedBadge": {
    message: "Arquivada",
    description: "Badge marking a session as archived (out of the working set)",
  },
  "agentSessions.startedFromSnapshot": {
    message: "Iniciada a partir de snapshot",
    description:
      "Session header chip: the latest turn booted from a repository environment build (ADR111 D4)",
  },
  "agentSessions.startedFromSnapshotTitle": {
    message:
      "O último turno começou a partir do build de ambiente {id}, com as etapas de configuração já executadas.",
    description:
      "Tooltip of the Started from snapshot chip (id is the build id, arb-…)",
  },
  "agentSessions.delete": {
    message: "Excluir",
    description: "Destructive action permanently deleting a finished session",
  },
  "agentSessions.deleting": {
    message: "Excluindo…",
    description: "Delete confirm button while the delete is in flight",
  },
  "agentSessions.deleteSuccess": {
    message: "Sessão excluída",
    description: "Toast after permanently deleting a session",
  },
  "agentSessions.deleteConfirmTitle": {
    message: "Excluir esta sessão?",
    description: "Delete confirmation dialog title",
  },
  "agentSessions.deleteConfirmBody": {
    message:
      "O registro da sessão e a transcrição da conversa serão excluídos permanentemente. As branches enviadas e os pull requests no GitHub não são afetados. Esta ação não pode ser desfeita.",
    description: "Delete confirmation dialog body",
  },
  "agentSessions.deleteConfirmDismiss": {
    message: "Manter sessão",
    description: "Delete confirmation dialog dismiss button",
  },
  "agentSessions.deleteConfirmProceed": {
    message: "Excluir sessão",
    description: "Delete confirmation dialog destructive proceed button",
  },
  "agentSessions.colActions": {
    message: "Ações",
    description: "Screen-reader label of the list's trailing actions column",
  },
  "agentSessions.filterActive": {
    message: "Recentes",
    description: "List membership tab — the unarchived working set",
  },
  "agentSessions.filterArchived": {
    message: "Arquivadas",
    description: "List membership tab — archived sessions only",
  },
  "agentSessions.filterAll": {
    message: "Todas",
    description: "List membership tab — archived and unarchived together",
  },
  "agentSessions.loadMore": {
    message: "Carregar mais",
    description: "Button fetching the next page of sessions",
  },
  "agentSessions.loadingMore": {
    message: "Carregando…",
    description: "Load-more button while the next page is in flight",
  },
  "agentSessions.attachFiles": {
    message: "Anexar arquivos",
    description:
      "Paperclip button that opens the file picker in the agent composers",
  },
  "agentSessions.attachmentRemove": {
    message: "Remover {name}",
    description: "Accessible label of the remove button on an attachment chip",
  },
  "agentSessions.attachmentTooLarge": {
    message: "Maior que {max}",
    description: "Attachment chip error when a file exceeds the per-file limit",
  },
  "agentSessions.attachmentTooMany": {
    message: "Até {max} arquivos por mensagem",
    description: "Attachment chip error when a message has too many files",
  },
  "agentSessions.attachmentDropHint": {
    message: "Solte os arquivos para anexar",
    description: "Overlay shown while dragging files over a composer",
  },
  "agentSessions.sharedFileLabel": {
    message: "Compartilhou um arquivo",
    description: "Label above a file the agent shared with share_file",
  },
  "agentSessions.filesButton": {
    message: "Arquivos",
    description: "Session header button that opens the session files",
  },
  "agentSessions.filesTitle": {
    message: "Arquivos da sessão",
    description: "Title of the session files sheet",
  },
  "agentSessions.filesDescription": {
    message:
      "Arquivos que você anexou e arquivos que o agente compartilhou nesta sessão.",
    description: "Description under the session files sheet title",
  },
  "agentSessions.filesEmpty": {
    message:
      "Nenhum arquivo ainda. Anexe arquivos a uma mensagem ou peça ao agente que compartilhe um.",
    description: "Empty state of the session files sheet",
  },
  "agentSessions.filesError": {
    message: "Não foi possível carregar os arquivos da sessão.",
    description: "Error state of the session files sheet",
  },
  "agentSessions.filesColumnName": {
    message: "Nome",
    description: "Files table column: file name",
  },
  "agentSessions.filesColumnFrom": {
    message: "De",
    description: "Files table column: who added the file",
  },
  "agentSessions.filesColumnTurn": {
    message: "Turno",
    description: "Files table column: turn number",
  },
  "agentSessions.filesColumnSize": {
    message: "Tamanho",
    description: "Files table column: file size",
  },
  "agentSessions.filesFromInput": {
    message: "Você",
    description: "Files table: a file the user attached",
  },
  "agentSessions.filesFromOutput": {
    message: "Agente",
    description: "Files table: a file the agent shared",
  },
  "agentSessions.filesLoadMore": {
    message: "Carregar mais",
    description: "Files table: load the next page",
  },
  "agentSessions.attachmentsBlockSend": {
    message: "Remova os arquivos que falharam no upload para enviar",
    description: "Hint when failed attachment chips block sending",
  },
  "agentSessions.attachmentOpen": {
    message: "Abrir {name}",
    description:
      "Accessible label of an attachment thumbnail that opens the lightbox",
  },
  "agentSessions.attachmentDownload": {
    message: "Baixar {name}",
    description: "Accessible label of an attachment download card",
  },
  "agentSessions.attachmentUnavailable": {
    message: "Indisponível",
    description:
      "Shown on an attachment whose download link could not be loaded",
  },
  "agentSessions.attachmentUploading": {
    message: "Enviando, {percent}%",
    description: "Screen-reader status of an attachment chip while uploading",
  },
  "agentSessions.attachmentReady": {
    message: "Enviado",
    description:
      "Screen-reader status of an attachment chip once its upload landed",
  },
  "agentSessions.attachmentHttpError": {
    message: "Falha no upload (HTTP {status})",
    description:
      "Attachment chip error when the server answered without a message",
  },
  "agentSessions.attachmentNetworkError": {
    message: "Falha no upload: erro de rede",
    description:
      "Attachment chip error when the upload could not reach the server",
  },
  "agentSessions.attachmentsUploading": {
    message: "Aguardando o término do upload dos arquivos…",
    description: "Composer hint while attachment uploads block sending",
  },
  "agentSessions.spendWarningTitle": {
    message:
      "Os gastos com o modelo do agente estão em {percent}% do limite deste mês",
    description: "Composer warning at 80% of the spend cap (ADR091 D6)",
  },
  "agentSessions.spendWarningBody": {
    message:
      "{used} de {cap} usados. As novas requisições ao modelo param no limite.",
    description: "Composer warning body",
  },
  "agentSessions.spendWarningBodyCharge": {
    message:
      "{used} de {cap} usados além dos créditos de faturamento. Os créditos de faturamento pagam primeiro; quando o gasto além deles chegar a {cap}, o cartão cadastrado é cobrado e as sessões continuam em execução. As novas requisições ao modelo só param se a cobrança falhar.",
    description:
      "Composer warning body when the cap is a new workspace's charge point (ADR091 D10.2a); {used} is the spend billing credits leave",
  },
  "agentSessions.spendWarningBodyCeiling": {
    message:
      "{used} de {cap} usados além dos créditos de faturamento. Os créditos de faturamento pagam primeiro; as novas requisições ao modelo param quando o gasto além deles chegar a {cap}.",
    description:
      "Composer warning body when the trust tier's ceiling binds on a workspace bex cannot charge (payment hold, excluded or comped) and the spend it counts, what billing credits leave ({used}), differs from month-to-date",
  },
  "agentSessions.spendReachedTitle.cap": {
    message: "Limite mensal de gastos com o modelo do agente atingido",
    description: "Composer refusal title: the workspace's own cap",
  },
  "agentSessions.spendReachedTitle.newWorkspace": {
    message: "Limite de gastos de workspace novo atingido",
    description: "Composer refusal title: the T0 trust-tier ceiling",
  },
  "agentSessions.spendReachedTitle.locked": {
    message: "Os gastos com o modelo do agente estão suspensos",
    description: "Composer refusal title: a payment hold",
  },
  "agentSessions.spendReachedBody.cap": {
    message:
      "Este workspace usou {used} do limite mensal de {cap}, então não é possível iniciar novas sessões. Um administrador do workspace pode aumentar o limite, ou ele é redefinido no dia 1º (UTC).",
    description: "Composer refusal body: the workspace's own cap",
  },
  "agentSessions.spendReachedBody.capMax": {
    message:
      "Este workspace usou {used} do limite mensal de {cap}, o máximo que ele pode ter, então não é possível iniciar novas sessões. O limite é redefinido no dia 1º (UTC); entre em contato com o suporte se precisar de um limite maior.",
    description:
      "Composer refusal body: the workspace's own cap binds at its maximum, so no admin can raise it",
  },
  "agentSessions.spendReachedBody.newWorkspace": {
    message:
      "Workspaces novos podem gastar até {cap} no modelo fornecido pelo bex até o primeiro pagamento bem-sucedido. O limite aumenta depois disso, ou é redefinido no dia 1º (UTC).",
    description: "Composer refusal body: the T0 trust-tier ceiling",
  },
  "agentSessions.spendReachedBody.locked": {
    message:
      "Um pagamento falho ou contestado limita os gastos com o modelo do agente deste workspace a {cap}. Regularize o pagamento pendente em Faturamento e depois inicie uma nova sessão.",
    description: "Composer refusal body: a payment hold",
  },
  "agentSessions.spendIncreaseCap": {
    message: "Aumentar limite",
    description: "Admin action linking to Settings → Agents",
  },
  "agentSessions.spendGoToBilling": {
    message: "Ir para Faturamento",
    description: "Link to the Billing page",
  },
  "agentSessions.spendBalanceDueTitle": {
    message: "Um pagamento falhou",
    description: "Composer refusal title: BILLING_BALANCE_DUE",
  },
  "agentSessions.spendBalanceDueBody": {
    message:
      "As sessões de agente no modelo fornecido pelo bex ficam pausadas até que a fatura pendente seja paga.",
    description: "Composer refusal body: BILLING_BALANCE_DUE",
  },
  "agentSessions.spendReachedHiddenTitle": {
    message: "Limite de gastos com o modelo do agente atingido",
    description:
      "Composer refusal title at the spend cap, for a member whose role cannot view billing details (no figures, w9/144)",
  },
  "agentSessions.spendReachedHiddenBody": {
    message:
      "Este workspace atingiu o limite de gastos com o modelo do agente, então novas sessões com o modelo fornecido pela bex não podem começar. Peça a um administrador do workspace.",
    description:
      "Composer refusal body at the spend cap, for a member whose role cannot view billing details (no figures, w9/144)",
  },
  "agentSessions.errors.AGENT_SESSION_TRUST_LIMIT": {
    message:
      "Workspaces novos podem executar {limit} sessões de agente ao mesmo tempo no modelo fornecido pelo bex até o primeiro pagamento bem-sucedido. Aguarde uma sessão em execução terminar ou pare uma e tente novamente.",
    description: "Mapped message for AGENT_SESSION_TRUST_LIMIT (ADR091 D10.1)",
  },
  "agentSessions.errors.AGENT_SESSION_TRUST_LIMIT_locked": {
    message:
      "Após um pagamento falho ou contestado, este workspace pode executar {limit} sessões de agente ao mesmo tempo no modelo fornecido pelo bex. Regularize o pagamento pendente em Faturamento para remover o limite.",
    description:
      "Mapped message for AGENT_SESSION_TRUST_LIMIT while a payment hold applies",
  },
  "agentSessions.errors.AGENT_SESSION_SPEND_LIMIT_INVALID": {
    message:
      "O limite de gastos deve ser maior que $0 e de no máximo $100.000.",
    description:
      "Mapped message for AGENT_SESSION_SPEND_LIMIT_INVALID (ADR094 D8)",
  },
  "agentSessions.errors.AGENT_SESSION_SPEND_LIMIT_UNAVAILABLE": {
    message:
      "Um limite de gastos só se aplica a sessões no modelo fornecido pelo bex. Este workspace usa a própria chave de modelo, então remova o limite ou mude o workspace para o modelo do bex em Configurações → Agentes.",
    description:
      "Mapped message for AGENT_SESSION_SPEND_LIMIT_UNAVAILABLE (ADR094 D8)",
  },
  "agentSessions.spendLimitLabel": {
    message: "Limite de gastos (USD)",
    description:
      "Composer — label/aria-label of the optional per-session spend ceiling input",
  },
  "agentSessions.spendLimitPlaceholder": {
    message: "Limite de gastos",
    description:
      "Composer — placeholder of the optional per-session spend ceiling input",
  },
  "agentSessions.spendLimitHint": {
    message:
      "Opcional. A sessão para quando os gastos com o modelo do bex chegam a este valor (mais de $0, no máximo $100.000).",
    description: "Composer — help text under the spend limit input",
  },
  "agentSessions.spendLimitInvalid": {
    message:
      "Insira um valor maior que $0 e de no máximo $100.000, ou deixe em branco.",
    description: "Composer — inline error for an out-of-range spend limit",
  },
  "agentSessions.failureSessionSpendLimitReached": {
    message:
      "Esta sessão parou porque os gastos com o modelo atingiram o limite da sessão, de {limit}. Inicie uma nova sessão com um limite maior para continuar.",
    description:
      "Failure callout for AGENT_MODEL_SPEND_CAP_REACHED when the session's own maxSpendUSD ceiling was hit",
  },
  "agentSessions.spendNote": {
    message: "Gastos com o modelo: {spent}",
    description:
      "Session page — the session's model spend so far (outcome.cost)",
  },
  "agentSessions.spendNoteWithLimit": {
    message: "Gastos com o modelo: {spent} do limite da sessão, de {limit}",
    description:
      "Session page — model spend so far beside the session's spend limit",
  },
  "agentSessions.costNote": {
    message: "Custo {total} · modelo {model}, computação {compute}",
    description:
      "Session page — the session's total cost so far (outcome.cost.totalUSD) with its model and sandbox compute parts",
  },
  "agentSessions.costNoteWithLimit": {
    message:
      "Custo {total} · modelo {model} do limite da sessão, de {limit}, computação {compute}",
    description:
      "Session page — total cost with its parts; the session spend limit caps the model part",
  },
  "agentSessions.resultTitle": {
    message: "Resultado",
    description:
      "Session page — heading over the turn's final structured result",
  },
  "agentSessions.resultDraftTitle": {
    message: "Resultado preliminar",
    description:
      "Session page — heading over the agent's not-yet-final structured result while the turn runs",
  },
  "agentSessions.resultDraftHint": {
    message: "Ainda não é final. O agente ainda pode alterá-lo.",
    description: "Session page — note under a draft structured result",
  },
  "agentSessions.resultCopy": {
    message: "Copiar JSON",
    description: "Session page — button copying the structured result",
  },
  "agentSessions.resultCopied": {
    message: "Copiado",
    description: "Session page — toast after copying the structured result",
  },
  "agentSessions.failureSpendCapReached": {
    message:
      "Este workspace atingiu o limite de gastos com o modelo do agente no mês. Um administrador pode aumentar o limite em Configurações → Agentes, ou ele é redefinido no dia 1º (UTC).",
    description: "Failure callout for AGENT_MODEL_SPEND_CAP_REACHED",
  },
  "agentSessions.failureBalanceDue": {
    message:
      "Um pagamento deste workspace falhou. Pague a fatura pendente em Faturamento e depois inicie uma nova sessão.",
    description: "Failure callout for BILLING_BALANCE_DUE",
  },
  "agentSessions.failureSetupTitle": {
    message: "Falha na configuração do repositório",
    description:
      "Failure callout heading when a repository setup step failed (AGENT_SESSION_SETUP_FAILED)",
  },
  "agentSessions.failureSetupHint": {
    message:
      "Uma etapa de configuração do ambiente deste repositório falhou, então o agente não iniciou e nenhum gasto com o modelo foi acumulado. Corrija a etapa em Configurações → Agentes → Ambientes de repositório e tente novamente.",
    description:
      "Failure callout guidance under the recorded reason (which names the step and its exit code) for AGENT_SESSION_SETUP_FAILED",
  },
  "agentSessions.failureSetupAction": {
    message: "Editar ambiente do repositório",
    description:
      "Failure callout button opening Settings → Repository environments",
  },
  "agentSessions.costTotal": {
    message: "{amount} de custo do modelo",
    description: "Session header: the session's model cost (ADR091 D6)",
  },
  "agentSessions.costTotalAll": {
    message: "{amount} de custo total",
    description:
      "Session header: the session's total cost so far, model plus sandbox compute (outcome.cost.totalUSD, w9/m178)",
  },
  "agentSessions.costBreakdown": {
    message: "Modelo {model} · computação {compute}",
    description:
      "Session header cost popover: the model and sandbox compute parts of the total",
  },
  "agentSessions.costByo": {
    message: "Cobrado pelo seu provedor",
    description: "Session header: a BYO-key session has no bex model cost",
  },
  "agentSessions.costTitle": {
    message: "Custo do modelo por turno",
    description: "Heading of the per-turn cost popover",
  },
  "agentSessions.costTurn": {
    message: "Turno {turn}",
    description: "One turn's row in the cost popover",
  },
  "agentSessions.costSetup": {
    message: "Antes do primeiro turno",
    description: "Requests recorded before the first turn",
  },
  "agentSessions.costTurnTokens": {
    message:
      "{input} de entrada · {output} de saída · {cache} de cache (tokens)",
    description: "A turn's token counts",
  },
  "agentSessions.costPricing": {
    message: "Cobrado ao custo do provedor do modelo + 30%.",
    description: "Pricing statement in the cost popover",
  },
  "agentSessions.secretsButton": {
    message: "Segredos",
    description: "Composer toolbar: the Secrets button label/tooltip (ADR098)",
  },
  "agentSessions.secretsButtonCount_one": {
    message: "Segredos ({count} anexado)",
    description:
      "Composer Secrets button accessible label when secrets are attached",
  },
  "agentSessions.secretsButtonCount_many": {
    message: "Segredos ({count} de anexados)",
    description:
      "Composer Secrets button accessible label when secrets are attached",
  },
  "agentSessions.secretsButtonCount_other": {
    message: "Segredos ({count} anexados)",
    description:
      "Composer Secrets button accessible label when secrets are attached",
  },
  "agentSessions.secretsTitle": {
    message: "Segredos desta sessão",
    description: "Composer Secrets popover heading",
  },
  "agentSessions.secretsDescription": {
    message:
      "O agente lê cada um como uma variável de ambiente. Os valores nunca são exibidos novamente e são ocultados na transcrição.",
    description: "Composer Secrets popover description",
  },
  "agentSessions.secretsWorkspace": {
    message: "Segredos do workspace",
    description:
      "Composer Secrets popover: workspace agent secrets list heading",
  },
  "agentSessions.secretsWorkspaceLoadFailed": {
    message: "Não foi possível carregar os segredos do agente do workspace.",
    description:
      "Composer secrets panel error when the workspace secret list fails to load",
  },
  "agentSessions.secretsWorkspaceEmpty": {
    message: "Nenhum segredo do workspace ainda.",
    description: "Composer Secrets popover: no workspace agent secrets",
  },
  "agentSessions.secretsManage": {
    message: "Gerenciar nas configurações",
    description: "Composer Secrets popover: link to Settings → Agents",
  },
  "agentSessions.secretsInline": {
    message: "Somente para esta sessão",
    description: "Composer Secrets popover: inline secrets heading",
  },
  "agentSessions.secretsRemove": {
    message: "Remover {name}",
    description:
      "Composer Secrets popover: remove an inline secret (accessible label)",
  },
  "agentSessions.secretsNamePlaceholder": {
    message: "NOME",
    description: "Composer Secrets popover: inline secret name placeholder",
  },
  "agentSessions.secretsNameLabel": {
    message: "Nome do segredo",
    description: "Composer Secrets popover: inline secret name field label",
  },
  "agentSessions.secretsValuePlaceholder": {
    message: "Valor",
    description: "Composer Secrets popover: inline secret value placeholder",
  },
  "agentSessions.secretsValueLabel": {
    message: "Valor do segredo",
    description: "Composer Secrets popover: inline secret value field label",
  },
  "agentSessions.secretsAdd": {
    message: "Adicionar",
    description: "Composer Secrets popover: add an inline secret",
  },
  "agentSessions.secretsHint": {
    message: "Excluído junto com a sessão.",
    description: "Composer Secrets popover: inline secret lifetime hint",
  },
  "agentSessions.secretsFull": {
    message: "Uma sessão aceita no máximo 20 segredos.",
    description: "Composer Secrets popover: session secret limit reached",
  },
  "agentSessions.secretsDuplicate": {
    message: "Esta sessão já tem um segredo com esse nome.",
    description: "Composer Secrets popover: duplicate inline secret name",
  },
  "agentSessions.secretsScopeRepo": {
    message: "Este repositório",
    description:
      "Composer Secrets popover: hint beside a secret scoped to the chosen repository, included",
  },
  "agentSessions.secretsScopeOtherRepo": {
    message: "Somente em {repo}",
    description:
      "Composer Secrets popover: hint beside a secret scoped to another repository, not available here",
  },
  "agentSessions.appliedSecretsTitle": {
    message: "Segredos no último turno",
    description:
      "Session header: heading of the applied-secrets popover (names and sources, never values)",
  },
  "agentSessions.appliedSecretSource.inline": {
    message: "esta sessão",
    description:
      "Applied secret source: an inline secret given to this session only",
  },
  "agentSessions.appliedSecretSource.explicit": {
    message: "selecionado",
    description: "Applied secret source: a workspace secret the session picked",
  },
  "agentSessions.appliedSecretSource.repo": {
    message: "repositório",
    description:
      "Applied secret source: a workspace secret scoped to the session's repository",
  },
  "agentSessions.appliedSecretSource.workspace": {
    message: "workspace",
    description:
      "Applied secret source: a workspace secret every session receives",
  },
  "agentSessions.metaSecrets_one": {
    message: "{count} segredo",
    description:
      "Detail header — number of attached secrets (singular, ADR098)",
  },
  "agentSessions.metaSecrets_many": {
    message: "{count} de segredos",
    description: "Detail header — number of attached secrets (plural, ADR098)",
  },
  "agentSessions.metaSecrets_other": {
    message: "{count} segredos",
    description: "Detail header — number of attached secrets (plural, ADR098)",
  },
  "agentSessions.skillsButton": {
    message: "Skills",
    description: "Composer toolbar: the Skills button label/tooltip (ADR099)",
  },
  "agentSessions.skillsButtonCount_one": {
    message: "Skills ({count} escolhida)",
    description:
      "Composer Skills button accessible label when skills are chosen",
  },
  "agentSessions.skillsButtonCount_many": {
    message: "Skills ({count} de escolhidas)",
    description:
      "Composer Skills button accessible label when skills are chosen",
  },
  "agentSessions.skillsButtonCount_other": {
    message: "Skills ({count} escolhidas)",
    description:
      "Composer Skills button accessible label when skills are chosen",
  },
  "agentSessions.skillsTitle": {
    message: "Skills para este turno",
    description: "Composer Skills popover heading",
  },
  "agentSessions.skillsDescription": {
    message:
      "O agente segue cada skill escolhida. As skills ativadas ele também pode aplicar por conta própria.",
    description: "Composer Skills popover description",
  },
  "agentSessions.skillsLoadFailed": {
    message: "Não foi possível carregar as skills do workspace.",
    description: "Composer Skills popover: list failed",
  },
  "agentSessions.skillsEmpty": {
    message: "Nenhuma skill neste workspace ainda.",
    description: "Composer Skills popover: none exist",
  },
  "agentSessions.skillsManage": {
    message: "Gerenciar skills",
    description: "Composer Skills popover link to Settings → Agents",
  },
  "agentSessions.skillsArgumentsLabel": {
    message: "Argumentos de {name}",
    description: "Composer Skills popover: argument field label",
  },
  "agentSessions.skillsArgumentsPlaceholder": {
    message: "Argumentos (opcional)",
    description: "Composer Skills popover: argument field placeholder",
  },
  "agentSessions.skillsArgumentsInvalid": {
    message: "Os argumentos ocupam uma linha, de até 4 KiB.",
    description: "Composer Skills popover: invalid arguments",
  },
  "agentSessions.skillsFull": {
    message: "Um turno pode usar no máximo 5 skills.",
    description: "Composer Skills popover: limit reached",
  },
  "agentSessions.skillsHint": {
    message: "Os argumentos preenchem $ARGUMENTS na skill.",
    description: "Composer Skills popover hint",
  },
  "agentSessions.skillsRemove": {
    message: "Remover {name}",
    description: "Accessible label of a chosen skill chip's remove button",
  },
  "agentSessions.turnSkills": {
    message: "Skills usadas neste turno",
    description: "Accessible label of a transcript turn's skill pills",
  },
  "agentSessions.turnOrigins": {
    message: "O que direcionou este turno",
    description:
      "Accessible label of the lines above a transcript turn that PR feedback started (w9/m344)",
  },
  "agentSessions.turnOriginReview": {
    message: "Direcionado por uma revisão de @{author}",
    description:
      "A turn started by a pull-request review; {author} is the reviewer's GitHub login. Links to the review",
  },
  "agentSessions.turnOriginReviewAnonymous": {
    message: "Direcionado por uma revisão do PR",
    description:
      "A turn started by a pull-request review whose author is unknown. Links to the review",
  },
  "agentSessions.turnOriginComment": {
    message: "Direcionado por um comentário de @{author} no PR",
    description:
      "A turn started by a pull-request comment; {author} is the commenter's GitHub login. Links to the comment",
  },
  "agentSessions.turnOriginCommentAnonymous": {
    message: "Direcionado por um comentário no PR",
    description:
      "A turn started by a pull-request comment whose author is unknown. Links to the comment",
  },
  "agentSessions.turnOriginCiFailure": {
    message: "Direcionado por uma execução de CI com falha",
    description:
      "A turn started by a failed CI run on the pull request. Links to the failed checks",
  },
  "agentSessions.turnOriginCiFailureOnBase": {
    message:
      "Direcionado por uma execução de CI com falha; algumas verificações também falham em {base}, não por causa desta alteração",
    description:
      "A turn started by a failed CI run on the pull request where at least one failing check also fails on the base branch ({base}, a branch name), so the pull request did not cause it. Links to the failed checks",
  },
  "agentSessions.turnOriginsStart": {
    message: "O que iniciou esta sessão",
    description:
      "Accessible label of the line above a session's first turn when a /bex comment on a pull request started it (w9/m348)",
  },
  "agentSessions.turnOriginStartComment": {
    message: "Iniciada pelo comentário de @{author}",
    description:
      "The first turn of a session a /bex pull-request comment started; {author} is the commenter's GitHub login. Links to the comment",
  },
  "agentSessions.turnOriginStartCommentAnonymous": {
    message: "Iniciada por um comentário no PR",
    description:
      "The first turn of a session a /bex pull-request comment started, when the commenter is unknown. Links to the comment",
  },
  "agentSessions.lineage.childSessions": {
    message: "Sessões filhas",
    description:
      "Heading of the block listing sessions the agent spawned, in the parent session's transcript",
  },
  "agentSessions.lineage.childCardTitle": {
    message: "Criou uma sessão",
    description:
      "Small label above a child session's title on its card in the parent's transcript",
  },
  "agentSessions.lineage.agentsPanel": {
    message: "Agentes",
    description:
      "Heading of the session page's right-hand panel listing the sessions below this one",
  },
  "agentSessions.lineage.viewSession": {
    message: "Ver sessão",
    description: "Link on an Agents panel row that opens that child session",
  },
  "agentSessions.lineage.parentSession": {
    message: "Sessão pai",
    description:
      "Link in the Agents panel of a child session that opens its parent",
  },
  "agentSessions.lineage.collapsePanel": {
    message: "Recolher o painel Agentes",
    description:
      "Accessible label of the button that collapses the Agents panel",
  },
  "agentSessions.lineage.expandPanel": {
    message: "Expandir o painel Agentes",
    description: "Accessible label of the button that expands the Agents panel",
  },
  "agentSessions.lineage.collapseChildren": {
    message: "Ocultar sessões filhas",
    description:
      "Accessible label of a sidebar row's button that collapses its nested child sessions",
  },
  "agentSessions.lineage.expandChildren": {
    message: "Mostrar sessões filhas",
    description:
      "Accessible label of a sidebar row's button that expands its nested child sessions",
  },
  "agentSessions.lineage.status.working": {
    message: "Trabalhando",
    description:
      "Status group and badge of a child session that is still running",
  },
  "agentSessions.lineage.status.done": {
    message: "Concluída",
    description:
      "Status group and badge of a child session that finished successfully",
  },
  "agentSessions.lineage.status.failed": {
    message: "Com falha",
    description: "Status group and badge of a child session that failed",
  },
  "agentSessions.lineage.status.canceled": {
    message: "Cancelada",
    description: "Status group and badge of a child session that was canceled",
  },
  "agentSessions.sidebarMine": {
    message: "Minhas",
    description:
      "Sessions sidebar — show only the sessions you started (Devin's default)",
  },
  "agentSessions.sidebarEveryone": {
    message: "Todos",
    description: "Sessions sidebar — show every member's sessions",
  },
  "agentSessions.sidebarOwnerFilter": {
    message: "Sessões de quem",
    description:
      "Sessions sidebar — accessible name of the Mine/Everyone toggle",
  },
  "agentSessions.sidebarEmptyMine": {
    message:
      "Nenhuma sessão iniciada por você. As sessões mais antigas estão em Todos.",
    description:
      "Sessions sidebar — empty state under Mine. Sessions from before 2026-11-08 record no starter, so they appear only under Everyone",
  },
  "agentSessions.startedByPerson": {
    message: "Iniciada por {name}",
    description: "Session header and list rows — who started the session",
  },
  "agentSessions.startedForPerson": {
    message: "para {name}",
    description:
      "Session header — after 'Started by automation X': the member the automation runs as",
  },
  "agentSessions.editTags": {
    message: "Editar tags",
    description: "Session header menu — open the tag editor",
  },
  "agentSessions.editTagsTitle": {
    message: "Editar tags",
    description: "Tag editor dialog title",
  },
  "agentSessions.editTagsDescription": {
    message:
      "As tags substituem as tags atuais da sessão. Separe-as com vírgulas ou espaços; deixe vazio para limpar.",
    description: "Tag editor dialog description",
  },
  "agentSessions.editTagsHint": {
    message:
      "Até 20 tags: letras minúsculas, dígitos, dois-pontos, sublinhado ou hífen.",
    description: "Tag editor dialog — the tag rules",
  },
  "agentSessions.editTagsSave": {
    message: "Salvar tags",
    description: "Tag editor dialog — save button",
  },
  "agentSessions.editTagsSaved": {
    message: "Tags salvas",
    description: "Toast after the tags were replaced",
  },
  "agentSessions.settings": {
    message: "Configurações",
    description: "Session header menu — open the session's settings (w10/m599)",
  },
  "agentSessions.settingsTitle": {
    message: "Configurações da sessão",
    description: "Session settings dialog title (w10/m599)",
  },
  "agentSessions.settingsDescription": {
    message:
      "O que se aplica a esta sessão enquanto ela é executada. Somente um administrador do workspace pode alterar isso.",
    description:
      "Session settings dialog description: the settings apply while the session runs and only a workspace admin changes them (w10/m599)",
  },
  "agentSessions.settingsGithubCliHint": {
    message:
      "Desativado desliga o gh para esta sessão e as sessões que ela inicia a partir da próxima chamada ao gh. Ativado ou Herdar devolve o gh a partir do próximo turno da sessão.",
    description:
      "Session settings dialog — when a GitHub CLI change applies: an off at the next gh call, an on or inherit at the next turn (w10/m599)",
  },
  "agentSessions.settingsSave": {
    message: "Salvar configurações",
    description: "Session settings dialog — save button (w10/m599)",
  },
  "agentSessions.settingsSaved": {
    message: "Configurações salvas",
    description: "Toast after a session's settings were saved (w10/m599)",
  },
  "agentSessions.terminalButton": {
    message: "Terminal",
    description:
      "Session header button that opens the session's Terminal (w10/m615)",
  },
  "agentSessions.terminalTitle": {
    message: "Terminal",
    description: "Title of the sheet holding the session's Terminal",
  },
  "agentSessions.terminalDescription": {
    message:
      "Um shell no sandbox desta sessão, em /workspace. Abri-lo inicia um sandbox que foi fechado.",
    description: "Description under the Terminal sheet's title",
  },
  "agentSessions.terminalConnecting": {
    message: "Conectando ao sandbox…",
    description: "Terminal status while it connects to the sandbox",
  },
  "agentSessions.terminalStarting": {
    message: "Iniciando o sandbox…",
    description: "Terminal status while the session's sandbox starts",
  },
  "agentSessions.terminalStartingDetail": {
    message:
      "O shell abre assim que o sandbox estiver ativo, normalmente em menos de um minuto.",
    description: "Terminal detail while the session's sandbox starts",
  },
  "agentSessions.terminalStillStarting": {
    message:
      "Ainda iniciando o sandbox ({elapsed}). O shell abre assim que ele estiver ativo.",
    description:
      "Terminal detail every 30 seconds while the sandbox still starts; {elapsed} is how long, e.g. 1m 30s",
  },
  "agentSessions.terminalConnected": {
    message: "Conectado",
    description: "Terminal status when the shell is live",
  },
  "agentSessions.terminalClosed": {
    message: "Sessão encerrada",
    description: "Terminal status when the shell ended",
  },
  "agentSessions.terminalErrorStatus": {
    message: "Erro de conexão",
    description: "Terminal status when it failed to connect or errored",
  },
  "agentSessions.terminalEnded": {
    message: "Sessão finalizada",
    description: "Terminal status when the server ended it for a stated reason",
  },
  "agentSessions.terminalReconnect": {
    message: "Reconectar",
    description: "Button that opens the Terminal again",
  },
  "agentSessions.terminalErrorGeneric": {
    message: "Não foi possível abrir um shell no sandbox.",
    description: "Terminal generic connection error",
  },
  "agentSessions.terminalUnavailable": {
    message:
      "O terminal no navegador não está habilitado nesta plataforma. Use Conectar para abrir o sandbox por SSH.",
    description:
      "Terminal error when the browser terminal is not configured on the platform (503)",
  },
  "agentSessions.terminalCanceled": {
    message: "Esta sessão foi cancelada, então não há sandbox para abrir.",
    description: "Why a canceled session's Terminal has nothing to open",
  },
  "agentSessions.terminalArchived": {
    message:
      "Esta sessão está arquivada e seu sandbox foi fechado. Desarquive-a para abrir o Terminal.",
    description: "Why an archived session's Terminal has nothing to open",
  },
  "agentSessions.terminalCannotStart": {
    message:
      "Esta sessão não está em execução e seu sandbox foi fechado. Para iniciá-la de novo, envie uma mensagem à sessão e depois abra o Terminal novamente.",
    description:
      "Terminal refusal for a member who may view the sandbox but not start it, as the SSH banner says",
  },
  "agentSessions.terminalStartRefused": {
    message: "O sandbox não pôde iniciar: {reason}",
    description:
      "Terminal refusal when the sandbox could not start; {reason} is the refusal (payment method, billing, the live-sandbox limit)",
  },
  "agentSessions.terminalEndRevoked": {
    message: "Seu acesso a esta sessão terminou.",
    description:
      "Terminal end reason: the caller lost access to the session (deliberately unspecific)",
  },
  "agentSessions.terminalEndSandboxGone": {
    message: "O sandbox parou. Reconecte para iniciá-lo de novo.",
    description: "Terminal end reason: the sandbox stopped under the shell",
  },
  "agentSessions.terminalEndTimeout": {
    message:
      "O Terminal atingiu o limite de tempo. Reconecte para continuar trabalhando.",
    description: "Terminal end reason: it reached its time limit",
  },
  "agentSessions.terminalEndUnverified": {
    message:
      "Não foi possível verificar seu acesso novamente, então o Terminal foi fechado. Reconecte para tentar de novo.",
    description:
      "Terminal end reason: the periodic access check could not complete",
  },
  "agentSessions.terminalEndNoSandbox": {
    message: "O sandbox não iniciou. Reconecte para tentar de novo.",
    description:
      "Terminal end reason: the sandbox's start ended without a sandbox",
  },
  "agentSessions.terminalEndWakeTimeout": {
    message: "O sandbox não iniciou a tempo. Reconecte para tentar de novo.",
    description:
      "Terminal end reason: the sandbox did not start within the start's ceiling",
  },
  "agentSessions.terminalSandboxClosed": {
    message:
      "O sandbox desta sessão foi fechado. Inicie-o para abrir um shell.",
    description:
      "Terminal sheet the page reopened from an earlier visit, on a session whose sandbox has closed: nothing starts until the button below is pressed (w10/060)",
  },
  "agentSessions.terminalStartSandbox": {
    message: "Iniciar o sandbox",
    description:
      "Button in a reopened Terminal sheet that starts the session's closed sandbox and opens the shell (w10/060)",
  },
};

export default ptAgentSessions;
