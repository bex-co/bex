import type { TranslationEntry } from "@/i18n";

const esAgentSessions: Record<string, TranslationEntry> = {
  "agentSessions.pageTitle": {
    message: "Sesiones de agente",
    description: "Agent sessions list page heading and document title",
  },
  "agentSessions.listTitle": {
    message: "Recientes",
    description: "Agent sessions recents heading — matches the rail",
  },
  "agentSessions.colTask": {
    message: "Tarea",
    description: "Agent sessions table column header — the session's prompt",
  },
  "agentSessions.colRepo": {
    message: "Repositorio",
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
    message: "Creada",
    description: "Agent sessions table column header — createdAt relative age",
  },
  "agentSessions.prBadge": {
    message: "#{number}",
    description: "Draft PR badge label — the pull-request number",
  },
  "agentSessions.prState.open": {
    message: "Abierta",
    description:
      "PR state badge beside a session's PR link — the pull request is open (a draft counts as open)",
  },
  "agentSessions.prState.merged": {
    message: "Fusionada",
    description:
      "PR state badge beside a session's PR link — the pull request was merged",
  },
  "agentSessions.prState.closed": {
    message: "Cerrada",
    description:
      "PR state badge beside a session's PR link — the pull request was closed without merging",
  },
  "agentSessions.prAutoMerge": {
    message: "Fusión automática",
    description:
      "Badge beside a session's PR state — GitHub auto-merge is armed on the pull request (w10/047)",
  },
  "agentSessions.prMergeQueued": {
    message: "En cola de fusión",
    description:
      "Badge beside a session's PR state — the pull request is in its base branch's merge queue (w10/047)",
  },
  "agentSessions.prMergeQueuedHint": {
    message: "En la cola de fusión: GitHub la fusiona cuando llegue al frente.",
    description:
      "Tooltip of the merge-queue badge — what being in the merge queue means",
  },
  "agentSessions.prAutoMergeEnabledBy": {
    message: "Fusión automática activada por {login}",
    description:
      "Tooltip of the auto-merge badge — the GitHub login that armed auto-merge",
  },
  "agentSessions.prAutoMergeEnabled": {
    message: "Fusión automática activada",
    description:
      "Tooltip of the auto-merge badge when GitHub does not say who armed it",
  },
  "agentSessions.prAutoMergeHint": {
    message: "GitHub la fusiona cuando pasen las revisiones y comprobaciones requeridas.",
    description:
      "Tooltip of the auto-merge badge — what auto-merge does",
  },
  "agentSessions.prAutoMergeMethod.merge": {
    message: "Crear un commit de fusión",
    description:
      "Tooltip of the auto-merge badge — the merge method, GitHub's 'Create a merge commit'",
  },
  "agentSessions.prAutoMergeMethod.squash": {
    message: "Squash y fusionar",
    description:
      "Tooltip of the auto-merge badge — the merge method, GitHub's 'Squash and merge'",
  },
  "agentSessions.prAutoMergeMethod.rebase": {
    message: "Rebase y fusionar",
    description:
      "Tooltip of the auto-merge badge — the merge method, GitHub's 'Rebase and merge'",
  },
  "agentSessions.prStateFilter": {
    message: "Filtrar por estado del pull request",
    description:
      "Session list — accessible name of the Any/Open/Merged/Closed PR-state filter",
  },
  "agentSessions.prStateFilterAny": {
    message: "Cualquiera",
    description:
      "Session list PR-state filter — the option that shows sessions whatever their PR state, including sessions with no PR",
  },
  "agentSessions.errorTitle": {
    message: "No se pudieron cargar las sesiones.",
    description: "Agent sessions list error — one sentence, Retry beside it",
  },
  "agentSessions.retry": {
    message: "Reintentar",
    description: "Retries a failed agent-sessions list request",
  },
  "agentSessions.emptyBody": {
    message: "Las sesiones que inicies aparecerán aquí.",
    description: "Default empty — one quiet line under the composer",
  },
  "agentSessions.emptyArchivedTitle": {
    message: "No hay sesiones archivadas",
    description: "Archived-only list empty-state heading",
  },
  "agentSessions.emptyArchivedBody": {
    message: "Las sesiones que archives aparecerán aquí.",
    description: "Archived-only list empty-state body",
  },
  "agentSessions.emptyFilteredTitle": {
    message: "No hay sesiones que coincidan",
    description: "Phase-filtered list empty-state heading",
  },
  "agentSessions.emptyFilteredBody": {
    message: "Prueba con otra fase o borra los filtros.",
    description: "Phase-filtered list empty-state body",
  },
  "agentSessions.clearFilters": {
    message: "Borrar filtros",
    description: "Clears agent-session membership and phase filters",
  },
  "agentSessions.phase.creating": {
    message: "Creando",
    description: "Agent session phase chip — sandbox is being created",
  },
  "agentSessions.phase.running": {
    message: "En ejecución",
    description: "Agent session phase chip — a turn is in progress",
  },
  "agentSessions.phase.resuming": {
    message: "Reanudando",
    description: "Agent session phase chip — session is resuming",
  },
  "agentSessions.phase.redispatching": {
    message: "Reenviando",
    description: "Agent session phase chip — session is being redispatched",
  },
  "agentSessions.phase.completed": {
    message: "Completada",
    description: "Agent session phase chip — session finished successfully",
  },
  "agentSessions.phase.failed": {
    message: "Fallida",
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
    message: "Tarea",
    description: "Composer — the prominent task prompt textarea label",
  },
  "agentSessions.taskPlaceholder": {
    message: "Describe una tarea de programación…",
    description: "Composer — task editor placeholder",
  },
  "agentSessions.mentionButton": {
    message: "Añadir repositorio o sesión",
    description:
      "Composer toolbar — opens the repository/session mention picker",
  },
  "agentSessions.repoChip": {
    message: "Repositorio {repo}",
    description: "Composer toolbar — accessible name of the selected-repo chip",
  },
  "agentSessions.keyboardHint": {
    message: "Intro para iniciar · Mayús+Intro para una nueva línea",
    description: "Muted hint under the composer",
  },
  "agentSessions.connectGitHubTitle": {
    message: "Conecta GitHub para empezar",
    description: "Composer callout when the workspace has no App repos",
  },
  "agentSessions.connectGitHubBody": {
    message:
      "Los agentes en la nube necesitan un repositorio de GitHub conectado. Conecta la GitHub App para este espacio de trabajo y vuelve para iniciar una sesión.",
    description: "Composer callout body when there are no installation repos",
  },
  "agentSessions.connectGitHubSettings": {
    message: "Configuración de GitHub",
    description:
      "Secondary link from the composer's GitHub callout to Account Settings → Integrations (/settings#integrations), where the GitHub card lives",
  },
  "agentSessions.exampleFixTests": {
    message: "Corrige las pruebas que fallan y abre un PR en borrador",
    description: "First-run example prompt chip",
  },
  "agentSessions.exampleAddReadme": {
    message: "Añade un README que explique cómo ejecutar el proyecto",
    description: "First-run example prompt chip",
  },
  "agentSessions.chipRemove": {
    message: "Quitar {name}",
    description: "Accessible label of a mention chip's remove button",
  },
  "agentSessions.mentionCategoryRepos": {
    message: "Repositorios",
    description: "Mention picker — the repositories category row",
  },
  "agentSessions.mentionCategoryReposDesc": {
    message: "Limita la sesión a un repositorio de GitHub conectado",
    description: "Mention picker — repositories category one-line description",
  },
  "agentSessions.mentionCategorySessions": {
    message: "Sesiones",
    description: "Mention picker — the prior-sessions category row",
  },
  "agentSessions.mentionCategorySessionsDesc": {
    message: "Usa una sesión de agente anterior como contexto",
    description: "Mention picker — sessions category one-line description",
  },
  "agentSessions.mentionNoResults": {
    message: "Sin coincidencias",
    description:
      "Mention picker — empty state when the fuzzy filter matches nothing",
  },
  "agentSessions.mentionReposEmpty": {
    message:
      "Aún no hay repositorios. Conecta una cuenta de GitHub o da a la GitHub App de bex acceso a un repositorio.",
    description:
      "Mention picker — empty state when no installation repos exist",
  },
  "agentSessions.mentionSessionsEmpty": {
    message: "Aún no hay sesiones",
    description:
      "Mention picker — empty state when the workspace has no sessions",
  },
  "agentSessions.mentionConnected": {
    message: "Conectado mediante la GitHub App",
    description: "Mention picker readiness footer — the repo is app-connected",
  },
  "agentSessions.mentionDefaultBranch": {
    message: "Rama predeterminada: {branch}",
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
    message: "{model} (predeterminado)",
    description: "Model list — the model a session runs when it names none",
  },
  "agentSessions.modelFixed": {
    message: "Este agente siempre usa {model}",
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
    message: "{cpu} CPU, {memory} GB de memoria",
    description:
      "Sandbox template size: CPU cores and memory in GB, numbers already formatted",
  },
  "agentSessions.sandboxOption": {
    message: "{name} · {size}",
    description: "Sandbox template choice: its name, then its size",
  },
  "agentSessions.sandboxDefaultOption": {
    message: "{template} (predeterminado)",
    description:
      "Sandbox list — the template a session runs on when it names none (the workspace default)",
  },
  "agentSessions.sandboxWorkspaceDefault": {
    message: "Predeterminado del espacio de trabajo ({template})",
    description:
      "Automation sandbox choice — follow the workspace default at each run; {template} is the default now",
  },
  "agentSessions.metaSandbox": {
    message: "Sandbox: {template}",
    description:
      "Tooltip of the sandbox template a session runs on, with its size",
  },
  "agentSessions.githubCliInherit": {
    message: "Heredar",
    description:
      "GitHub CLI choice — inherit the level above, while what that resolves to is still loading (w10/m561)",
  },
  "agentSessions.githubCliInheritOn": {
    message: "Heredar (activado)",
    description:
      "GitHub CLI choice — inherit the level above, which has gh on (w10/m561)",
  },
  "agentSessions.githubCliInheritOff": {
    message: "Heredar (desactivado)",
    description:
      "GitHub CLI choice — inherit the level above, which has gh off (w10/m561)",
  },
  "agentSessions.githubCliOn": {
    message: "Activado",
    description:
      "GitHub CLI choice — gh on for this session or automation (w10/m561)",
  },
  "agentSessions.githubCliOff": {
    message: "Desactivado",
    description:
      "GitHub CLI choice — gh off for this session or automation (w10/m561)",
  },
  "agentSessions.githubCliOnMandatory": {
    message:
      "No disponible mientras la desactivación de GitHub CLI sea obligatoria en este espacio de trabajo.",
    description:
      "Why the GitHub CLI On choice is disabled: the workspace's off is mandatory (w10/m561)",
  },
  "agentSessions.githubCliAdminOnly": {
    message: "Solo un administrador del espacio de trabajo puede cambiar esto.",
    description:
      "Why a GitHub CLI choice is disabled: only a workspace admin chooses or changes a session's or an automation's gh (w10/044)",
  },
  "agentSessions.githubCliTitle": {
    message: "GitHub CLI para esta sesión: {choice}",
    description:
      "Tooltip of the composer's GitHub CLI control; {choice} is the chosen binding, e.g. Inherit (on) (w10/m561)",
  },
  "agentSessions.errors.AGENT_SESSION_GITHUB_CLI_LOCKED": {
    message:
      "En este espacio de trabajo la desactivación de GitHub CLI es obligatoria, así que una sesión no puede activar gh. Elige “Desactivado” o “Heredar”.",
    description:
      "Mapped message for AGENT_SESSION_GITHUB_CLI_LOCKED with reason mandatory (w10/m561)",
  },
  "agentSessions.errors.AGENT_SESSION_GITHUB_CLI_LOCKED_adminRequired": {
    message:
      "Solo un administrador del espacio de trabajo puede elegir GitHub CLI para una sesión. Elige “Heredar”.",
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
    message: "Iniciar sesión de agente",
    description: "Composer — submit button label",
  },
  "agentSessions.submitting": {
    message: "Iniciando…",
    description: "Composer — submit button label while the create is in flight",
  },
  "agentSessions.unavailableTitle": {
    message: "Las sesiones de agente no están configuradas",
    description: "Composer — house callout title when the feature is 503",
  },
  "agentSessions.unavailableBody": {
    message:
      "Esta plataforma no ha habilitado las sesiones de agente en la nube. Pide a tu operador que configure el gateway de sesiones de agente.",
    description: "Composer — house callout body when the feature is 503",
  },
  "agentSessions.dependencyUnavailableTitle": {
    message: "Las sesiones de agente no están disponibles temporalmente",
    description:
      "Composer — retryable dependency-outage title, distinct from configuration",
  },
  "agentSessions.dependencyUnavailableBody": {
    message:
      "Una dependencia de la plataforma tiene problemas en este momento. Tu configuración sigue intacta; espera un momento e inténtalo de nuevo.",
    description:
      "Composer — retryable dependency-outage body that does not send the user to an operator",
  },
  "agentSessions.modelKeyMissingTitle": {
    message: "Primero añade una clave de proveedor del modelo",
    description:
      "Composer pre-flight title when the workspace has no model key",
  },
  "agentSessions.modelKeyMissingBody": {
    message:
      "Este espacio de trabajo aún no tiene acceso a ningún modelo. Un administrador del espacio de trabajo puede añadir la clave de proveedor del modelo propia del espacio de trabajo en Configuración → Agentes y luego volver aquí.",
    description:
      "Composer pre-flight body when the workspace has neither its own model key nor the bex-provided model (ADR091 D7)",
  },
  "agentSessions.modelKeyMissingAction": {
    message: "Abrir Configuración → Agentes",
    description:
      "Composer pre-flight link to the Settings → Agents model key field (/workspace/settings#agents)",
  },
  "agentSessions.createErrorTitle": {
    message: "No se pudo iniciar la sesión",
    description: "Composer — error alert title when the create fails",
  },
  "agentSessions.detailTitle": {
    message: "Sesión",
    description: "Agent-session detail page document title",
  },
  "agentSessions.untitled": {
    message: "Sesión sin título",
    description:
      "Last-resort session name when a session carries neither a repository nor a task prompt",
  },
  "agentSessions.backToList": {
    message: "Todas las sesiones",
    description: "Detail page — back link to the /agents list",
  },
  "agentSessions.detailErrorTitle": {
    message: "No se pudo cargar esta sesión",
    description: "Detail page — heading when the session read fails",
  },
  "agentSessions.conversationTitle": {
    message: "Conversación",
    description: "Detail page — the live conversation column card title",
  },
  "agentSessions.failureTitle": {
    message: "La sesión falló",
    description: "Detail page — heading over a failed session's reason",
  },
  "agentSessions.deliveryNoteTitle": {
    message: "Los cambios no se entregaron por completo",
    description:
      "Detail page — heading over a completed session's note that the agent left work uncommitted, unpushed or without a pull request",
  },
  "agentSessions.deliveryNoteHint": {
    message:
      "Envía un mensaje de seguimiento pidiendo al agente que termine de hacer commit, push y actualizar el pull request.",
    description:
      "Detail page — the next step shown under an undelivered-work note",
  },
  "agentSessions.failureReasonFallback": {
    message: "La sesión no pudo iniciarse.",
    description:
      "Detail page — generic failure text when the session carries no specific reason",
  },
  "agentSessions.failureRetry": {
    message: "Reintentar",
    description:
      "Detail page — button to re-run a failed session's original task",
  },
  "agentSessions.failureRetrying": {
    message: "Reintentando…",
    description:
      "Detail page — retry button label while the redispatch is in flight",
  },
  "agentSessions.capacityFailureTitle": {
    message: "Sin capacidad de sandbox",
    description:
      "Detail page — heading when a session failed on the plan's sandbox limit",
  },
  "agentSessions.capacityFailureBody": {
    message:
      "Este espacio de trabajo alcanzó el límite de sandboxes simultáneos de su plan. Mejora el plan para ejecutar más a la vez, o detén una sesión inactiva y vuelve a intentarlo.",
    description:
      "Detail page — explains a plan-limit sandbox failure and the two remedies",
  },
  "agentSessions.upgradePlan": {
    message: "Mejorar plan",
    description:
      "Detail page — CTA on a capacity failure; opens the change-plan dialog",
  },
  "agentSessions.failureCause.pool_full": {
    message: "Ninguna máquina de sandbox tenía espacio",
    description:
      "Failed-start cause line (session callout, automation runs) — no sandbox machine had room for the session's machine for the whole wait (w10/m497)",
  },
  "agentSessions.failureCause.image_pull": {
    message: "No se pudo descargar la imagen del sandbox",
    description:
      "Failed-start cause line — the sandbox's container image did not pull (w10/m497)",
  },
  "agentSessions.failureCause.runtime": {
    message: "El runtime del sandbox no pudo arrancar",
    description:
      "Failed-start cause line — the sandbox runtime did not start the session's machine (w10/m497)",
  },
  "agentSessions.failureCause.quota": {
    message: "La cuota de sandboxes del espacio de trabajo lo rechazó",
    description:
      "Failed-start cause line — the workspace's sandbox quota refused the session's machine (w10/m497)",
  },
  "agentSessions.failureCause.unknown": {
    message: "Problema de la plataforma",
    description:
      "Failed-start cause line — the sandbox did not start for a cause on bex's side that bex-api could not name (w10/m497)",
  },
  "agentSessions.provisioning": {
    message: "Iniciando el sandbox…",
    description:
      "Detail page — shown while a new/steered session provisions its sandbox before the conversation stream is available",
  },
  "agentSessions.metaDuration": {
    message: "Duración {duration}",
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
    message: "La cancelación ya está en curso.",
    description: "Detail header — tooltip on the disabled cancel button",
  },
  "agentSessions.canceling": {
    message: "Cancelando…",
    description: "Detail header — confirm button label while the cancel runs",
  },
  "agentSessions.cancelSuccess": {
    message: "Cancelando la sesión…",
    description: "Detail header — toast after the cancel is accepted",
  },
  "agentSessions.cancelConfirmTitle": {
    message: "¿Cancelar esta sesión?",
    description: "Cancel confirm dialog title",
  },
  "agentSessions.cancelConfirmBody": {
    message:
      "Esto detiene al agente. Los commits que ya se enviaron a la rama y el PR se conservan.",
    description: "Cancel confirm dialog body — states pushed work is preserved",
  },
  "agentSessions.cancelConfirmDismiss": {
    message: "Seguir ejecutando",
    description: "Cancel confirm dialog — dismiss button",
  },
  "agentSessions.cancelConfirmProceed": {
    message: "Cancelar sesión",
    description: "Cancel confirm dialog — proceed button",
  },
  "agentSessions.stopTurn": {
    message: "Detener turno",
    description:
      "Detail header — Stop turn button label: ends only the running turn, unlike Cancel which ends the session",
  },
  "agentSessions.stoppingTurn": {
    message: "Deteniendo…",
    description: "Detail header — Stop turn button label while the stop runs",
  },
  "agentSessions.stopTurnHint": {
    message:
      "Termina solo el turno en curso. La sesión sigue abierta y los mensajes en cola conservan su lugar. Atajo: Ctrl/Cmd+Shift+Backspace.",
    description: "Detail header — tooltip on the Stop turn button",
  },
  "agentSessions.stopTurnStarting": {
    message:
      "El turno aún está arrancando. Podrás detenerlo cuando esté en ejecución.",
    description:
      "Detail header — tooltip on the disabled Stop turn button while the sandbox starts",
  },
  "agentSessions.stopTurnSuccess": {
    message: "Deteniendo el turno. Terminará en unos segundos.",
    description: "Detail header — toast after Stop turn is accepted",
  },
  "agentSessions.steerTitle": {
    message: "Orientar esta sesión",
    description: "Steering composer card title",
  },
  "agentSessions.steerErrorTitle": {
    message: "No se pudo enviar",
    description: "Steering composer — inline error alert title",
  },
  "agentSessions.steerPlaceholderIdle": {
    message:
      "Envía una instrucción de seguimiento para iniciar un nuevo turno en la misma rama.",
    description: "Steering composer — textarea placeholder for an idle session",
  },
  "agentSessions.steerPlaceholderQueue": {
    message:
      "Pon un mensaje en cola. Se ejecuta cuando termine el turno actual.",
    description:
      "Steering composer — textarea placeholder while a turn runs or messages wait (ADR047 D12)",
  },
  "agentSessions.steerHintIdle": {
    message:
      "Esta sesión está inactiva: al enviar un mensaje, la conversación continúa en un sandbox nuevo en la misma rama.",
    description: "Steering composer — helper text for the redispatch route",
  },
  "agentSessions.steerHintQueue": {
    message:
      "El agente está trabajando. Tu mensaje queda en cola y se ejecuta como un turno propio cuando termine este.",
    description:
      "Steering composer — helper text while a turn runs or messages wait (ADR047 D12)",
  },
  "agentSessions.steerDisabledCanceling": {
    message:
      "Esta sesión se está cancelando. El trabajo ya enviado se conserva.",
    description: "Steering composer — disabled reason while canceling",
  },
  "agentSessions.steerDisabledCanceled": {
    message: "Esta sesión se canceló.",
    description: "Steering composer — disabled reason once canceled",
  },
  "agentSessions.steerDisabledStream": {
    message:
      "El flujo de la conversación no está disponible, por lo que la orientación en vivo está en pausa.",
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
    message: "Enviado: se está reenviando un nuevo turno.",
    description: "Steering composer — toast after a redispatch is accepted",
  },
  "agentSessions.steerQueued": {
    message: "En cola. Se ejecuta cuando termine el turno actual.",
    description:
      "Steering composer — toast after a message was queued behind a running turn (ADR047 D12)",
  },
  "agentSessions.queueRegion": {
    message: "Mensajes en cola",
    description: "Queued messages list — accessible name of the region",
  },
  "agentSessions.queueTitle_one": {
    message: "{count} mensaje en cola",
    description: "Queued messages list — heading with the count (singular)",
  },
  "agentSessions.queueTitle_many": {
    message: "{count} de mensajes en cola",
    description: "Queued messages list — heading with the count (plural)",
  },
  "agentSessions.queueTitle_other": {
    message: "{count} mensajes en cola",
    description: "Queued messages list — heading with the count (plural)",
  },
  "agentSessions.queueHint": {
    message: "Se ejecutan uno a uno, en este orden, después del turno actual.",
    description:
      "Queued messages list — one-line explanation next to the heading",
  },
  "agentSessions.queuePosition": {
    message: "Lugar {position} en la cola",
    description: "Queued messages list — tooltip on a row's place number",
  },
  "agentSessions.queueAttachments_one": {
    message: "{count} archivo adjunto",
    description:
      "Queued message row — attachments that ride with it (singular)",
  },
  "agentSessions.queueAttachments_many": {
    message: "{count} de archivos adjuntos",
    description: "Queued message row — attachments that ride with it (plural)",
  },
  "agentSessions.queueAttachments_other": {
    message: "{count} archivos adjuntos",
    description: "Queued message row — attachments that ride with it (plural)",
  },
  "agentSessions.queueBlocked": {
    message: "Aún no se pudo iniciar: {reason}. Se reintenta automáticamente.",
    description:
      "Queued message row — why the dispatcher could not start it (a spend ceiling or the live-sandbox limit)",
  },
  "agentSessions.queueModel": {
    message: "Cambia la sesión a {model}",
    description:
      "Queued message — the model its turn switches the session to (w10/m516)",
  },
  "agentSessions.queueSendNow": {
    message: "Enviar ahora",
    description:
      "Queued message row — stops the running turn and runs this message next",
  },
  "agentSessions.queueMoveUp": {
    message: "Subir",
    description: "Queued message row — move one place earlier",
  },
  "agentSessions.queueMoveDown": {
    message: "Bajar",
    description: "Queued message row — move one place later",
  },
  "agentSessions.queueEdit": {
    message: "Editar",
    description: "Queued message row — edit the message in place",
  },
  "agentSessions.queueRemove": {
    message: "Quitar",
    description: "Queued message row — remove it so it never runs",
  },
  "agentSessions.queueEditLabel": {
    message: "Editar mensaje en cola",
    description: "Queued message row — accessible name of the edit box",
  },
  "agentSessions.queueEditSave": {
    message: "Guardar",
    description: "Queued message row — save the edit",
  },
  "agentSessions.queueEditSaveSendNow": {
    message: "Guardar y enviar ahora",
    description:
      "Queued message row — save the edit, stop the running turn and run this message next",
  },
  "agentSessions.queueEditDiscard": {
    message: "Descartar",
    description: "Queued message row — leave the edit without saving",
  },
  "agentSessions.queueEditHint": {
    message:
      "Enter guarda, Shift+Enter añade una línea, Ctrl/Cmd+Enter guarda y lo envía ahora, Esc descarta.",
    description: "Queued message row — keyboard hints while editing",
  },
  "agentSessions.queueSendNowStopping": {
    message:
      "Deteniendo el turno actual. Este mensaje se ejecuta a continuación.",
    description: "Queued message row — toast after Send now stopped a turn",
  },
  "agentSessions.queueSendNowNext": {
    message: "Este mensaje se ejecuta a continuación.",
    description:
      "Queued message row — toast after Send now when no turn was running",
  },
  "agentSessions.errors.AGENT_SESSION_INPUT_INVALID": {
    message:
      "Esa entrada no es válida. Comprueba el repositorio, la rama y la tarea, e inténtalo de nuevo.",
    description: "Mapped message for AGENT_SESSION_INPUT_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_ID_INVALID": {
    message: "Ese ID de sesión no es válido.",
    description: "Mapped message for AGENT_SESSION_ID_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_FOUND": {
    message: "Esa sesión ya no existe.",
    description: "Mapped message for AGENT_SESSION_NOT_FOUND",
  },
  "agentSessions.errors.AGENT_SESSION_CONFLICT": {
    message:
      "Esta sesión está ocupada con otra operación. Inténtalo de nuevo en breve.",
    description: "Mapped message for AGENT_SESSION_CONFLICT",
  },
  "agentSessions.errors.AGENT_SESSION_BRANCH_IN_USE": {
    message:
      "Otra sesión ya está trabajando en esta rama. Envía un mensaje a esa sesión o usa otra rama bex-agent/.",
    description: "Mapped message for AGENT_SESSION_BRANCH_IN_USE (w5/m382)",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_STEERABLE": {
    message: "Esta sesión no se puede orientar en su fase actual ({phase}).",
    description: "Mapped message for AGENT_SESSION_NOT_STEERABLE",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_RESUMABLE": {
    message: "Esta sesión no se puede reanudar en su fase actual ({phase}).",
    description: "Mapped message for AGENT_SESSION_NOT_RESUMABLE",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_ATTACHABLE": {
    message: "No se puede conectar a esta sesión en su fase actual ({phase}).",
    description: "Mapped message for AGENT_SESSION_NOT_ATTACHABLE",
  },
  "agentSessions.errors.AGENT_SESSION_TURN_IN_FLIGHT": {
    message:
      "Ya hay un turno en ejecución. Espera a que termine antes de enviar otro.",
    description: "Mapped message for AGENT_SESSION_TURN_IN_FLIGHT",
  },
  "agentSessions.errors.AGENT_SESSION_QUEUE_FULL": {
    message:
      "Esta sesión ya tiene {max} mensajes en cola. Espera a que se ejecute uno, o quita uno, y vuelve a enviar.",
    description: "Mapped message for AGENT_SESSION_QUEUE_FULL (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_QUEUE_INLINE_SECRETS": {
    message:
      "Un mensaje con secretos en línea no puede esperar en la cola, porque sus valores nunca se almacenan. Envíalo cuando la sesión esté inactiva o usa un secreto del espacio de trabajo.",
    description:
      "Mapped message for AGENT_SESSION_QUEUE_INLINE_SECRETS (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_QUEUED_MESSAGE_DISPATCHED": {
    message:
      "Ese mensaje ya empezó a ejecutarse, así que ya no se puede cambiar.",
    description:
      "Mapped message for AGENT_QUEUED_MESSAGE_DISPATCHED (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_QUEUED_MESSAGE_NOT_FOUND": {
    message: "Ese mensaje ya no está en la cola.",
    description:
      "Mapped message for AGENT_QUEUED_MESSAGE_NOT_FOUND (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_QUEUED_MESSAGE_ATTACHMENTS_OWNED": {
    message:
      "Este mensaje lleva los archivos adjuntos de quien lo envió, así que solo esa persona puede editarlo. Aún puedes moverlo o quitarlo.",
    description:
      "Mapped message for AGENT_QUEUED_MESSAGE_ATTACHMENTS_OWNED (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_NO_TURN_RUNNING": {
    message:
      "No hay ningún turno en ejecución, así que no hay nada que detener.",
    description:
      "Mapped message for AGENT_SESSION_NO_TURN_RUNNING (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_TURN_STARTING": {
    message:
      "El turno aún está arrancando, así que todavía no hay nada que detener. Inténtalo de nuevo en unos segundos.",
    description: "Mapped message for AGENT_SESSION_TURN_STARTING (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_MODEL_ENDPOINT_INVALID": {
    message: "El endpoint del modelo debe ser una URL HTTPS válida.",
    description: "Mapped message for AGENT_SESSION_MODEL_ENDPOINT_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_MODEL_NOT_OFFERED": {
    message:
      "El modelo de la plataforma de este espacio de trabajo no ofrece {model}. Elige uno de la lista de modelos.",
    description:
      "Mapped message for AGENT_SESSION_MODEL_NOT_OFFERED (w10/m516)",
  },
  "agentSessions.errors.AGENT_SESSION_MODEL_PINNED": {
    message: "Este agente solo usa {pinnedModel}.",
    description: "Mapped message for AGENT_SESSION_MODEL_PINNED",
  },
  "agentSessions.errors.AGENT_SESSION_EGRESS_ALLOWLIST_INVALID": {
    message:
      "La entrada “{entry}” de la lista de permitidos de salida no es válida: {reason}",
    description: "Mapped message for AGENT_SESSION_EGRESS_ALLOWLIST_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_EGRESS_ALLOWLIST_IMMUTABLE": {
    message:
      "La lista de permitidos de salida no se puede cambiar después de iniciar la sesión.",
    description: "Mapped message for AGENT_SESSION_EGRESS_ALLOWLIST_IMMUTABLE",
  },
  "agentSessions.errors.AGENT_SESSION_EGRESS_PHASE_INVALID": {
    message:
      "La lista de permitidos de salida no se puede establecer en la fase actual de esta sesión.",
    description: "Mapped message for AGENT_SESSION_EGRESS_PHASE_INVALID",
  },
  "agentSessions.conversationLoading": {
    message: "Cargando conversación…",
    description: "Placeholder while the client-only conversation column loads",
  },
  "agentSessions.conversationConnecting": {
    message: "Conectando con el flujo de la sesión…",
    description: "Shown while the conversation stream replay is in flight",
  },
  "agentSessions.conversationNotStarted": {
    message: "La conversación aparecerá cuando el sandbox esté listo.",
    description:
      "Healthy pre-stream state while a newly created session is provisioning",
  },
  "agentSessions.conversationEmpty": {
    message: "Aún no hay conversación.",
    description: "Shown when a session has produced no transcript parts",
  },
  "agentSessions.turnInterrupted": {
    message: "El turno {turn} se interrumpió antes de terminar.",
    description:
      "Transcript line at the end of a turn a person stopped with Stop turn (ADR047 D12); not a failure",
  },
  "agentSessions.turnRetentionSaving": {
    message:
      "Guardando tus cambios para que el siguiente mensaje pueda continuar desde ellos.",
    description:
      "Transcript line under a stopped turn while its unsaved work is being saved so the next message can continue from it (ADR047 D12, w22/m413)",
  },
  "agentSessions.turnRetentionKept": {
    message: "Tus cambios se conservan hasta {time}.",
    description:
      "Transcript line under a stopped turn: its unpushed changes are kept until {time}, a date and time in the viewer's own zone (ADR047 D12, w22/m413)",
  },
  "agentSessions.turnRetentionKeptNoTime": {
    message: "Tus cambios se conservan para tu siguiente mensaje.",
    description:
      "Same as the kept line before the browser knows the viewer's time zone, so it names no time",
  },
  "agentSessions.turnRetentionContinued": {
    message: "El siguiente mensaje continuó desde estos cambios.",
    description:
      "Transcript line under a stopped turn: the next message started from its kept changes",
  },
  "agentSessions.turnRetentionLostExpired": {
    message:
      "Tus cambios sin hacer push se perdieron cuando caducó el sandbox conservado.",
    description:
      "Transcript line under a stopped turn: its kept sandbox expired and the unpushed changes in it are gone",
  },
  "agentSessions.turnRetentionLostStorageFull": {
    message:
      "Tus cambios sin hacer push no se conservaron porque el almacenamiento de sandbox del espacio de trabajo estaba lleno.",
    description:
      "Transcript line under a stopped turn: nothing was kept because the workspace's sandbox storage cap was full",
  },
  "agentSessions.turnRetentionLostSaveFailed": {
    message:
      "No se pudieron guardar tus cambios sin hacer push y se perdieron.",
    description:
      "Transcript line under a stopped turn: saving its sandbox failed, so the unpushed changes are gone",
  },
  "agentSessions.turnRetentionLostEditorOpen": {
    message:
      "Tus cambios sin hacer push no se conservaron porque había un editor abierto en el sandbox.",
    description:
      "Transcript line under a stopped turn: nothing was kept because an editor (SSH) was open on the sandbox, which Stop does not end",
  },
  "agentSessions.turnRetentionLostNotRestored": {
    message:
      "No se pudieron restaurar tus cambios sin hacer push, así que el siguiente mensaje empezó desde un checkout nuevo.",
    description:
      "Transcript line under a stopped turn: the next message could not start from the kept changes and started from a fresh checkout",
  },
  "agentSessions.turnRetentionLost": {
    message: "Tus cambios sin hacer push se perdieron.",
    description:
      "Transcript line under a stopped turn: its unpushed changes were lost, for a reason that is not worth naming (the session was canceled or archived)",
  },
  "agentSessions.turnRetentionLostSandboxLost": {
    message:
      "Tus cambios sin hacer push se perdieron cuando el turno falló y su sandbox ya no estaba.",
    description:
      "Transcript line under a failed turn: its sandbox was gone before its disk could be kept, so the unpushed work in it is lost (w10/m491)",
  },
  "agentSessions.turnStakeCommits_one": {
    message: "{count} commit sin hacer push",
    description:
      "Under a turn whose disk was kept or lost: how many commits its checkout held that were not pushed (w10/m491)",
  },
  "agentSessions.turnStakeCommits_many": {
    message: "{count} de commits sin hacer push",
    description:
      "Under a turn whose disk was kept or lost: how many commits its checkout held that were not pushed (w10/m491)",
  },
  "agentSessions.turnStakeCommits_other": {
    message: "{count} commits sin hacer push",
    description:
      "Under a turn whose disk was kept or lost: how many commits its checkout held that were not pushed (w10/m491)",
  },
  "agentSessions.turnStakeChanges": {
    message: "cambios sin confirmar",
    description:
      "Under a turn whose disk was kept or lost: its checkout held uncommitted changes (w10/m491)",
  },
  "agentSessions.turnError": {
    message: "El turno {turn} falló",
    description:
      "Recorded failure of an earlier turn; subsequent turns remain readable",
  },
  "agentSessions.continuitySessionLoad": {
    message: "El agente se reanudó con el estado de conversación restaurado.",
    description:
      "System line when a fresh agent generation reloaded its own saved session (ADR047 D3 ladder rung 1)",
  },
  "agentSessions.continuityReprime": {
    message:
      "El agente se reinició: el contexto se reconstruyó a partir del historial de la sesión.",
    description:
      "System line when a fresh agent generation was primed from the durable transcript (ladder rung 2)",
  },
  "agentSessions.continuityTaskRedelivery": {
    message: "El agente se reinició: se volvió a entregar la tarea original.",
    description:
      "System line when no prior turn ever reached the agent, so the task was re-sent (ladder rung 3)",
  },
  "agentSessions.conversationEndedEmpty": {
    message: "Esta sesión terminó antes de que se registrara una conversación.",
    description: "Ended session with no durable transcript parts",
  },
  "agentSessions.conversationEnded": {
    message: "Sesión finalizada.",
    description: "Footer note under a terminal session's replayed transcript",
  },
  "agentSessions.conversationIncomplete": {
    message: "No se pudo conservar parte de la salida del asistente",
    description:
      "Warning below a durable user turn whose assistant transcript is incomplete",
  },
  "agentSessions.conversationUnavailable": {
    message: "El flujo de la conversación no está disponible en este momento.",
    description:
      "Degraded-state message when the m43 stream endpoint errors or is unconfigured",
  },
  "agentSessions.conversationUnavailableTerminal": {
    message:
      "La transcripción de la conversación no está disponible para esta sesión.",
    description:
      "Terminal session — transcript unavailable rather than live stream degraded",
  },
  "agentSessions.showEarlierMessages_one": {
    message: "Mostrar {count} mensaje anterior",
    description:
      "Button revealing older transcript messages hidden by the render window (singular)",
  },
  "agentSessions.showEarlierMessages_many": {
    message: "Mostrar {count} de mensajes anteriores",
    description:
      "Button revealing older transcript messages hidden by the render window (plural)",
  },
  "agentSessions.showEarlierMessages_other": {
    message: "Mostrar {count} mensajes anteriores",
    description:
      "Button revealing older transcript messages hidden by the render window (plural)",
  },
  "agentSessions.groupThought": {
    message: "Razonamiento",
    description: "Collapsible header for the agent's reasoning group",
  },
  "agentSessions.groupPlan": {
    message: "Plan",
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
    message: "(sin salida)",
    description: "Placeholder when a terminal group has no captured output",
  },
  "agentSessions.toolInput": {
    message: "Entrada",
    description: "Section label for a tool call's input",
  },
  "agentSessions.toolOutput": {
    message: "Salida",
    description: "Section label for a tool call's output",
  },
  "agentSessions.toolError": {
    message: "Error",
    description: "Section label for a tool call's error text",
  },
  "agentSessions.toolStateRunning": {
    message: "En ejecución",
    description: "Badge label for a tool call awaiting output",
  },
  "agentSessions.toolStateDone": {
    message: "Hecho",
    description: "Badge label for a completed tool call",
  },
  "agentSessions.toolStateDenied": {
    message: "Denegada",
    description: "Badge label for a tool call whose execution was denied",
  },
  "agentSessions.toolStateInterrupted": {
    message: "Interrumpida",
    description:
      "Badge label for a tool call the session ended before it finished",
  },
  "agentSessions.toolStateError": {
    message: "Fallida",
    description: "Badge label for a failed tool call",
  },
  "agentSessions.activityEdited_one": {
    message: "Editó {count} archivo",
    description:
      "Activity-group summary when the turn edited files (diffs, singular)",
  },
  "agentSessions.activityEdited_many": {
    message: "Editó {count} de archivos",
    description: "Activity-group summary when the turn edited files (diffs)",
  },
  "agentSessions.activityEdited_other": {
    message: "Editó {count} archivos",
    description: "Activity-group summary when the turn edited files (diffs)",
  },
  "agentSessions.activityRan_one": {
    message: "Ejecutó {count} comando",
    description:
      "Activity-group summary when the turn ran commands/terminals (singular)",
  },
  "agentSessions.activityRan_many": {
    message: "Ejecutó {count} de comandos",
    description: "Activity-group summary when the turn ran commands/terminals",
  },
  "agentSessions.activityRan_other": {
    message: "Ejecutó {count} comandos",
    description: "Activity-group summary when the turn ran commands/terminals",
  },
  "agentSessions.activitySteps_one": {
    message: "{count} paso",
    description: "Activity-group summary fallback — the step count (singular)",
  },
  "agentSessions.activitySteps_many": {
    message: "{count} de pasos",
    description: "Activity-group summary fallback — the step count (plural)",
  },
  "agentSessions.activitySteps_other": {
    message: "{count} pasos",
    description: "Activity-group summary fallback — the step count (plural)",
  },
  "agentSessions.activityRead_one": {
    message: "Leyó {count} archivo",
    description: "Activity-group summary segment — file reads (singular)",
  },
  "agentSessions.activityRead_many": {
    message: "Leyó {count} de archivos",
    description: "Activity-group summary segment — file reads",
  },
  "agentSessions.activityRead_other": {
    message: "Leyó {count} archivos",
    description: "Activity-group summary segment — file reads",
  },
  "agentSessions.activitySearched_one": {
    message: "Buscó {count} vez",
    description: "Activity-group summary segment — searches (singular)",
  },
  "agentSessions.activitySearched_many": {
    message: "Buscó {count} de veces",
    description: "Activity-group summary segment — searches",
  },
  "agentSessions.activitySearched_other": {
    message: "Buscó {count} veces",
    description: "Activity-group summary segment — searches",
  },
  "agentSessions.activitySubagents_one": {
    message: "Ejecutó {count} subagente",
    description:
      "Activity-group summary segment — delegated subagent tasks (singular)",
  },
  "agentSessions.activitySubagents_many": {
    message: "Ejecutó {count} de subagentes",
    description: "Activity-group summary segment — delegated subagent tasks",
  },
  "agentSessions.activitySubagents_other": {
    message: "Ejecutó {count} subagentes",
    description: "Activity-group summary segment — delegated subagent tasks",
  },
  "agentSessions.activityTools_one": {
    message: "Usó {count} herramienta",
    description:
      "Activity-group summary segment — tools of no known kind (singular)",
  },
  "agentSessions.activityTools_many": {
    message: "Usó {count} de herramientas",
    description: "Activity-group summary segment — tools of no known kind",
  },
  "agentSessions.activityTools_other": {
    message: "Usó {count} herramientas",
    description: "Activity-group summary segment — tools of no known kind",
  },
  "agentSessions.activityThought_one": {
    message: "Razonó {count} vez",
    description:
      "Activity-group summary segment — the agent's thoughts between its calls (singular)",
  },
  "agentSessions.activityThought_many": {
    message: "Razonó {count} de veces",
    description:
      "Activity-group summary segment — the agent's thoughts between its calls",
  },
  "agentSessions.activityThought_other": {
    message: "Razonó {count} veces",
    description:
      "Activity-group summary segment — the agent's thoughts between its calls",
  },
  "agentSessions.activityThinking": {
    message: "Razonando…",
    description: "Live status line while the agent is thinking between calls",
  },
  "agentSessions.activityFailed_one": {
    message: "{count} con error",
    description:
      "Activity-group summary segment — failed tool calls (singular)",
  },
  "agentSessions.activityFailed_many": {
    message: "{count} de con error",
    description: "Activity-group summary segment — failed tool calls",
  },
  "agentSessions.activityFailed_other": {
    message: "{count} con error",
    description: "Activity-group summary segment — failed tool calls",
  },
  "agentSessions.outputClipped_one": {
    message: "{formatted} carácter recortado",
    description:
      "Badge on a tool output the server stored head+tail-capped (singular)",
  },
  "agentSessions.outputClipped_many": {
    message: "{formatted} de caracteres recortados",
    description: "Badge on a tool output the server stored head+tail-capped",
  },
  "agentSessions.outputClipped_other": {
    message: "{formatted} caracteres recortados",
    description: "Badge on a tool output the server stored head+tail-capped",
  },
  "agentSessions.showFullOutput": {
    message: "Mostrar salida completa",
    description:
      "Button on a clipped tool output that loads the complete text the server kept",
  },
  "agentSessions.hideFullOutput": {
    message: "Ocultar salida completa",
    description: "Button that collapses a loaded full tool output",
  },
  "agentSessions.fullOutputNotRetained": {
    message: "La salida completa no se conservó",
    description:
      "Note on a clipped tool output whose complete text the server did not keep (clipped before full copies were stored)",
  },
  "agentSessions.fullOutputLoading": {
    message: "Cargando salida completa…",
    description: "Status while a tool call's full output loads",
  },
  "agentSessions.fullOutputError": {
    message: "No se pudo cargar la salida completa.",
    description: "Inline error when loading a tool call's full output failed",
  },
  "agentSessions.fullOutputCopy": {
    message: "Copiar salida completa",
    description:
      "Accessible label of the copy button on a loaded full tool output",
  },
  "agentSessions.fullOutputCopied": {
    message: "Salida completa copiada",
    description: "Toast after copying a full tool output",
  },
  "agentSessions.fullOutputCopyError": {
    message: "No se pudo copiar la salida completa",
    description: "Toast when copying a full tool output failed",
  },
  "agentSessions.conversationPartial": {
    message: "La conversación solo se cargó parcialmente.",
    description:
      "Inline row when the transcript stream failed after some of it rendered",
  },
  "agentSessions.liveStatus": {
    message: "Estado del agente",
    description:
      "Accessible label for the single live status line under a streaming transcript",
  },
  "agentSessions.scrollToBottom": {
    message: "Ir a lo más reciente",
    description:
      "Accessible label for the floating jump-to-bottom button in the conversation column",
  },
  "agentSessions.recentSessions": {
    message: "Recientes",
    description: "Sessions sidebar — heading over the recent sessions list",
  },
  "agentSessions.sidebarEmpty": {
    message: "Aún no hay sesiones",
    description: "Sessions sidebar — empty state when the workspace has none",
  },
  "agentSessions.sidebarLabel": {
    message: "Sesiones de agente",
    description: "Sessions sidebar — accessible landmark label",
  },
  "agentSessions.sidebarSearch": {
    message: "Buscar sesiones",
    description: "Sessions sidebar — search toggle + input accessible label",
  },
  "agentSessions.sidebarSearchPlaceholder": {
    message: "Buscar sesiones…",
    description: "Sessions sidebar — search input placeholder",
  },
  "agentSessions.sidebarNoMatches": {
    message: "No hay sesiones que coincidan",
    description:
      "Sessions sidebar — empty state when the search matches nothing",
  },
  "agentSessions.statusPhrase.prReady": {
    message: "El PR está listo",
    description: "Sidebar status phrase — completed session with a draft PR",
  },
  "agentSessions.statusPhrase.working": {
    message: "Trabajando…",
    description: "Sidebar status phrase — session still converging",
  },
  "agentSessions.statusPhrase.waitingForSandbox": {
    message: "Esperando una máquina de sandbox",
    description:
      "Session status (header, list rows, sidebar, conversation, automation runs) — the turn waits because no sandbox machine has room for it yet (w10/m497); the time waited follows where there is room",
  },
  "agentSessions.menuMore": {
    message: "Más acciones",
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
    message: "Abrir en Zed",
    description:
      "Connect menu — hotlink that opens the sandbox as a Zed remote project",
  },
  "agentSessions.openInZedHint": {
    message: "Abre el /workspace del sandbox por SSH. Requiere el editor Zed.",
    description: "Connect menu — helper text under the Open-in-Zed action",
  },
  "agentSessions.openInZedStart": {
    message: "Iniciar el sandbox",
    description:
      "Connect menu — button that starts a closed session's sandbox so Zed can open it (ADR054 D8)",
  },
  "agentSessions.openInZedClosed": {
    message:
      "El sandbox se cerró porque la sesión estuvo inactiva. Inícialo para volver a abrir el trabajo de la sesión en Zed: tarda unos segundos y no ejecuta ningún turno del agente.",
    description:
      "Connect menu — why a closed session needs its sandbox started before Zed can open it",
  },
  "agentSessions.openInZedStarting": {
    message:
      "Iniciando el sandbox… El enlace de Zed aparecerá aquí cuando esté listo.",
    description:
      "Connect menu — shown while a closed session's sandbox starts for the editor",
  },
  "agentSessions.openInZedStartFailed": {
    message: "El sandbox no se inició. Inténtalo de nuevo.",
    description:
      "Connect menu — the sandbox started for the editor did not come up",
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
    message: "No se pudo copiar",
    description: "Connect menu — toast when copying the ssh command fails",
  },
  "agentSessions.groupThoughtFor": {
    message: "Razonó durante {duration}",
    description:
      "Reasoning-group summary with elapsed duration from persisted part timestamps (or ~arrival timing)",
  },
  "agentSessions.terminalStatus.completed": {
    message: "El agente terminó",
    description: "Terminal transcript status line — completed session",
  },
  "agentSessions.terminalStatus.failed": {
    message: "La sesión terminó con un error",
    description: "Terminal transcript status line — failed session",
  },
  "agentSessions.terminalStatus.canceled": {
    message: "La sesión se canceló",
    description: "Terminal transcript status line — canceled session",
  },
  "agentSessions.tagsLabel": {
    message: "Etiquetas",
    description: "Accessible label for a session row's tag list (w21/002)",
  },
  "agentSessions.filterByTag": {
    message: "Mostrar sesiones con la etiqueta {tag}",
    description:
      "Accessible label of a tag chip that filters the session list to that tag",
  },
  "agentSessions.filterTagged": {
    message: "Con la etiqueta {tag}",
    description: "Session list heading while narrowed to one tag",
  },
  "agentSessions.archive": {
    message: "Archivar",
    description: "Action putting a session out of the working set (ADR065)",
  },
  "agentSessions.unarchive": {
    message: "Desarchivar",
    description: "Action returning an archived session to the working set",
  },
  "agentSessions.archiveSuccess": {
    message: "Sesión archivada",
    description: "Toast after archiving a session",
  },
  "agentSessions.undoArchive": {
    message: "Deshacer",
    description: "Toast action that immediately unarchives a session",
  },
  "agentSessions.unarchiveSuccess": {
    message: "Sesión desarchivada",
    description: "Toast after unarchiving a session",
  },
  "agentSessions.archivedBadge": {
    message: "Archivada",
    description: "Badge marking a session as archived (out of the working set)",
  },
  "agentSessions.startedFromSnapshot": {
    message: "Iniciada desde instantánea",
    description:
      "Session header chip: the latest turn booted from a repository environment build (ADR111 D4)",
  },
  "agentSessions.startedFromSnapshotTitle": {
    message:
      "El último turno arrancó desde el build de entorno {id}, con los pasos de configuración ya ejecutados.",
    description:
      "Tooltip of the Started from snapshot chip (id is the build id, arb-…)",
  },
  "agentSessions.delete": {
    message: "Eliminar",
    description: "Destructive action permanently deleting a finished session",
  },
  "agentSessions.deleting": {
    message: "Eliminando…",
    description: "Delete confirm button while the delete is in flight",
  },
  "agentSessions.deleteSuccess": {
    message: "Sesión eliminada",
    description: "Toast after permanently deleting a session",
  },
  "agentSessions.deleteConfirmTitle": {
    message: "¿Eliminar esta sesión?",
    description: "Delete confirmation dialog title",
  },
  "agentSessions.deleteConfirmBody": {
    message:
      "El registro de la sesión y la transcripción de su conversación se eliminarán de forma permanente. Las ramas enviadas y los pull requests en GitHub no se ven afectados. Esta acción no se puede deshacer.",
    description: "Delete confirmation dialog body",
  },
  "agentSessions.deleteConfirmDismiss": {
    message: "Conservar sesión",
    description: "Delete confirmation dialog dismiss button",
  },
  "agentSessions.deleteConfirmProceed": {
    message: "Eliminar sesión",
    description: "Delete confirmation dialog destructive proceed button",
  },
  "agentSessions.colActions": {
    message: "Acciones",
    description: "Screen-reader label of the list's trailing actions column",
  },
  "agentSessions.filterActive": {
    message: "Recientes",
    description: "List membership tab — the unarchived working set",
  },
  "agentSessions.filterArchived": {
    message: "Archivadas",
    description: "List membership tab — archived sessions only",
  },
  "agentSessions.filterAll": {
    message: "Todas",
    description: "List membership tab — archived and unarchived together",
  },
  "agentSessions.loadMore": {
    message: "Cargar más",
    description: "Button fetching the next page of sessions",
  },
  "agentSessions.loadingMore": {
    message: "Cargando…",
    description: "Load-more button while the next page is in flight",
  },
  "agentSessions.attachFiles": {
    message: "Adjuntar archivos",
    description:
      "Paperclip button that opens the file picker in the agent composers",
  },
  "agentSessions.attachmentRemove": {
    message: "Quitar {name}",
    description: "Accessible label of the remove button on an attachment chip",
  },
  "agentSessions.attachmentTooLarge": {
    message: "Supera {max}",
    description: "Attachment chip error when a file exceeds the per-file limit",
  },
  "agentSessions.attachmentTooMany": {
    message: "Hasta {max} archivos por mensaje",
    description: "Attachment chip error when a message has too many files",
  },
  "agentSessions.attachmentDropHint": {
    message: "Suelta los archivos para adjuntarlos",
    description: "Overlay shown while dragging files over a composer",
  },
  "agentSessions.sharedFileLabel": {
    message: "Compartió un archivo",
    description: "Label above a file the agent shared with share_file",
  },
  "agentSessions.filesButton": {
    message: "Archivos",
    description: "Session header button that opens the session files",
  },
  "agentSessions.filesTitle": {
    message: "Archivos de la sesión",
    description: "Title of the session files sheet",
  },
  "agentSessions.filesDescription": {
    message:
      "Archivos que adjuntaste y archivos que el agente compartió en esta sesión.",
    description: "Description under the session files sheet title",
  },
  "agentSessions.filesEmpty": {
    message:
      "Aún no hay archivos. Adjunta archivos a un mensaje o pide al agente que comparta uno.",
    description: "Empty state of the session files sheet",
  },
  "agentSessions.filesError": {
    message: "No se pudieron cargar los archivos de la sesión.",
    description: "Error state of the session files sheet",
  },
  "agentSessions.filesColumnName": {
    message: "Nombre",
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
    message: "Tamaño",
    description: "Files table column: file size",
  },
  "agentSessions.filesFromInput": {
    message: "Tú",
    description: "Files table: a file the user attached",
  },
  "agentSessions.filesFromOutput": {
    message: "Agente",
    description: "Files table: a file the agent shared",
  },
  "agentSessions.filesLoadMore": {
    message: "Cargar más",
    description: "Files table: load the next page",
  },
  "agentSessions.attachmentsBlockSend": {
    message: "Quita los archivos que no se pudieron subir para enviar",
    description: "Hint when failed attachment chips block sending",
  },
  "agentSessions.attachmentOpen": {
    message: "Abrir {name}",
    description:
      "Accessible label of an attachment thumbnail that opens the lightbox",
  },
  "agentSessions.attachmentDownload": {
    message: "Descargar {name}",
    description: "Accessible label of an attachment download card",
  },
  "agentSessions.attachmentUnavailable": {
    message: "No disponible",
    description:
      "Shown on an attachment whose download link could not be loaded",
  },
  "agentSessions.attachmentUploading": {
    message: "Subiendo, {percent}%",
    description: "Screen-reader status of an attachment chip while uploading",
  },
  "agentSessions.attachmentReady": {
    message: "Subido",
    description:
      "Screen-reader status of an attachment chip once its upload landed",
  },
  "agentSessions.attachmentHttpError": {
    message: "La subida falló (HTTP {status})",
    description:
      "Attachment chip error when the server answered without a message",
  },
  "agentSessions.attachmentNetworkError": {
    message: "La subida falló: error de red",
    description:
      "Attachment chip error when the upload could not reach the server",
  },
  "agentSessions.attachmentsUploading": {
    message: "Esperando a que terminen de subirse los archivos…",
    description: "Composer hint while attachment uploads block sending",
  },
  "agentSessions.spendWarningTitle": {
    message:
      "El gasto del modelo del agente está al {percent}% del límite de este mes",
    description: "Composer warning at 80% of the spend cap (ADR091 D6)",
  },
  "agentSessions.spendWarningBody": {
    message:
      "{used} de {cap} usados. Las nuevas solicitudes al modelo se detienen al llegar al límite.",
    description: "Composer warning body",
  },
  "agentSessions.spendWarningBodyCharge": {
    message:
      "{used} de {cap} usados más allá de los créditos de facturación. Los créditos de facturación se usan primero; cuando el gasto que los supera alcance {cap}, se cobrará a la tarjeta registrada y las sesiones seguirán ejecutándose. Las nuevas solicitudes al modelo solo se detienen si el cobro falla.",
    description:
      "Composer warning body when the cap is a new workspace's charge point (ADR091 D10.2a); {used} is the spend billing credits leave",
  },
  "agentSessions.spendWarningBodyCeiling": {
    message:
      "{used} de {cap} usados más allá de los créditos de facturación. Los créditos de facturación se usan primero; las nuevas solicitudes al modelo se detienen cuando el gasto que los supera alcance {cap}.",
    description:
      "Composer warning body when the trust tier's ceiling binds on a workspace bex cannot charge (payment hold, excluded or comped) and the spend it counts, what billing credits leave ({used}), differs from month-to-date",
  },
  "agentSessions.spendReachedTitle.cap": {
    message: "Se alcanzó el límite mensual de gasto del modelo del agente",
    description: "Composer refusal title: the workspace's own cap",
  },
  "agentSessions.spendReachedTitle.newWorkspace": {
    message: "Se alcanzó el límite de gasto de los espacios de trabajo nuevos",
    description: "Composer refusal title: the T0 trust-tier ceiling",
  },
  "agentSessions.spendReachedTitle.locked": {
    message: "El gasto del modelo del agente está retenido",
    description: "Composer refusal title: a payment hold",
  },
  "agentSessions.spendReachedBody.cap": {
    message:
      "Este espacio de trabajo ha usado {used} de su límite mensual de {cap}, por lo que no se pueden iniciar sesiones nuevas. Un administrador del espacio de trabajo puede subir el límite, o se restablece el día 1 (UTC).",
    description: "Composer refusal body: the workspace's own cap",
  },
  "agentSessions.spendReachedBody.capMax": {
    message:
      "Este espacio de trabajo ha usado {used} de su límite mensual de {cap}, el máximo al que se puede establecer, por lo que no se pueden iniciar sesiones nuevas. El límite se restablece el día 1 (UTC); contacta con soporte si necesitas un límite mayor.",
    description:
      "Composer refusal body: the workspace's own cap binds at its maximum, so no admin can raise it",
  },
  "agentSessions.spendReachedBody.newWorkspace": {
    message:
      "Los espacios de trabajo nuevos pueden gastar hasta {cap} en el modelo que proporciona bex hasta su primer pago correcto. El límite sube después, o se restablece el día 1 (UTC).",
    description: "Composer refusal body: the T0 trust-tier ceiling",
  },
  "agentSessions.spendReachedBody.locked": {
    message:
      "Un pago fallido o disputado limita a {cap} el gasto del modelo del agente de este espacio de trabajo. Resuelve el pago pendiente en Facturación y luego inicia una sesión nueva.",
    description: "Composer refusal body: a payment hold",
  },
  "agentSessions.spendIncreaseCap": {
    message: "Aumentar límite",
    description: "Admin action linking to Settings → Agents",
  },
  "agentSessions.spendGoToBilling": {
    message: "Ir a Facturación",
    description: "Link to the Billing page",
  },
  "agentSessions.spendBalanceDueTitle": {
    message: "Un pago falló",
    description: "Composer refusal title: BILLING_BALANCE_DUE",
  },
  "agentSessions.spendBalanceDueBody": {
    message:
      "Las sesiones de agente con el modelo que proporciona bex están en pausa hasta que se pague la factura pendiente.",
    description: "Composer refusal body: BILLING_BALANCE_DUE",
  },
  "agentSessions.errors.AGENT_SESSION_TRUST_LIMIT": {
    message:
      "Los espacios de trabajo nuevos pueden ejecutar {limit} sesiones de agente a la vez con el modelo que proporciona bex hasta su primer pago correcto. Espera a que termine una sesión en ejecución o detén una, y vuelve a intentarlo.",
    description: "Mapped message for AGENT_SESSION_TRUST_LIMIT (ADR091 D10.1)",
  },
  "agentSessions.errors.AGENT_SESSION_TRUST_LIMIT_locked": {
    message:
      "Tras un pago fallido o disputado, este espacio de trabajo puede ejecutar {limit} sesiones de agente a la vez con el modelo que proporciona bex. Resuelve el pago pendiente en Facturación para quitar el límite.",
    description:
      "Mapped message for AGENT_SESSION_TRUST_LIMIT while a payment hold applies",
  },
  "agentSessions.errors.AGENT_SESSION_SPEND_LIMIT_INVALID": {
    message:
      "El límite de gasto debe ser mayor que $0 y de $100,000 como máximo.",
    description:
      "Mapped message for AGENT_SESSION_SPEND_LIMIT_INVALID (ADR094 D8)",
  },
  "agentSessions.errors.AGENT_SESSION_SPEND_LIMIT_UNAVAILABLE": {
    message:
      "Un límite de gasto solo se aplica a las sesiones con el modelo que proporciona bex. Este espacio de trabajo usa su propia clave de modelo, así que borra el límite o cambia el espacio de trabajo al modelo de bex en Configuración → Agentes.",
    description:
      "Mapped message for AGENT_SESSION_SPEND_LIMIT_UNAVAILABLE (ADR094 D8)",
  },
  "agentSessions.spendLimitLabel": {
    message: "Límite de gasto (USD)",
    description:
      "Composer — label/aria-label of the optional per-session spend ceiling input",
  },
  "agentSessions.spendLimitPlaceholder": {
    message: "Límite de gasto",
    description:
      "Composer — placeholder of the optional per-session spend ceiling input",
  },
  "agentSessions.spendLimitHint": {
    message:
      "Opcional. La sesión se detiene cuando su gasto en el modelo de bex alcanza esta cantidad (mayor que $0, máximo $100,000).",
    description: "Composer — help text under the spend limit input",
  },
  "agentSessions.spendLimitInvalid": {
    message:
      "Introduce una cantidad mayor que $0 y de $100,000 como máximo, o déjalo en blanco.",
    description: "Composer — inline error for an out-of-range spend limit",
  },
  "agentSessions.failureSessionSpendLimitReached": {
    message:
      "Esta sesión se detuvo porque su gasto en el modelo alcanzó el límite de sesión de {limit}. Inicia una sesión nueva con un límite mayor para continuar.",
    description:
      "Failure callout for AGENT_MODEL_SPEND_CAP_REACHED when the session's own maxSpendUSD ceiling was hit",
  },
  "agentSessions.spendNote": {
    message: "Gasto del modelo: {spent}",
    description:
      "Session page — the session's model spend so far (outcome.cost)",
  },
  "agentSessions.spendNoteWithLimit": {
    message: "Gasto del modelo: {spent} del límite de sesión de {limit}",
    description:
      "Session page — model spend so far beside the session's spend limit",
  },
  "agentSessions.costNote": {
    message: "Costo {total} · modelo {model}, cómputo {compute}",
    description:
      "Session page — the session's total cost so far (outcome.cost.totalUSD) with its model and sandbox compute parts",
  },
  "agentSessions.costNoteWithLimit": {
    message:
      "Costo {total} · modelo {model} del límite de sesión de {limit}, cómputo {compute}",
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
    message: "Aún no es definitivo. El agente todavía puede cambiarlo.",
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
      "Este espacio de trabajo alcanzó su límite de gasto del modelo del agente para el mes. Un administrador puede subir el límite en Configuración → Agentes, o se restablece el día 1 (UTC).",
    description: "Failure callout for AGENT_MODEL_SPEND_CAP_REACHED",
  },
  "agentSessions.failureBalanceDue": {
    message:
      "Un pago de este espacio de trabajo falló. Paga la factura pendiente en Facturación y luego inicia una sesión nueva.",
    description: "Failure callout for BILLING_BALANCE_DUE",
  },
  "agentSessions.failureSetupTitle": {
    message: "Falló la configuración del repositorio",
    description:
      "Failure callout heading when a repository setup step failed (AGENT_SESSION_SETUP_FAILED)",
  },
  "agentSessions.failureSetupHint": {
    message:
      "Un paso de configuración del entorno de este repositorio falló, así que el agente no arrancó y no se acumuló gasto del modelo. Corrige el paso en Configuración → Agentes → Entornos de repositorio y vuelve a intentarlo.",
    description:
      "Failure callout guidance under the recorded reason (which names the step and its exit code) for AGENT_SESSION_SETUP_FAILED",
  },
  "agentSessions.failureSetupAction": {
    message: "Editar entorno de repositorio",
    description:
      "Failure callout button opening Settings → Repository environments",
  },
  "agentSessions.costTotal": {
    message: "{amount} de costo del modelo",
    description: "Session header: the session's model cost (ADR091 D6)",
  },
  "agentSessions.costTotalAll": {
    message: "{amount} de costo total",
    description:
      "Session header: the session's total cost so far, model plus sandbox compute (outcome.cost.totalUSD, w9/m178)",
  },
  "agentSessions.costBreakdown": {
    message: "Modelo {model} · cómputo {compute}",
    description:
      "Session header cost popover: the model and sandbox compute parts of the total",
  },
  "agentSessions.costByo": {
    message: "Facturado por tu proveedor",
    description: "Session header: a BYO-key session has no bex model cost",
  },
  "agentSessions.costTitle": {
    message: "Costo del modelo por turno",
    description: "Heading of the per-turn cost popover",
  },
  "agentSessions.costTurn": {
    message: "Turno {turn}",
    description: "One turn's row in the cost popover",
  },
  "agentSessions.costSetup": {
    message: "Antes del primer turno",
    description: "Requests recorded before the first turn",
  },
  "agentSessions.costTurnTokens": {
    message:
      "{input} de entrada · {output} de salida · {cache} de caché (tokens)",
    description: "A turn's token counts",
  },
  "agentSessions.costPricing": {
    message: "Se factura al costo del proveedor del modelo + 30 %.",
    description: "Pricing statement in the cost popover",
  },
  "agentSessions.secretsButton": {
    message: "Secretos",
    description: "Composer toolbar: the Secrets button label/tooltip (ADR098)",
  },
  "agentSessions.secretsButtonCount_one": {
    message: "Secretos ({count} adjunto)",
    description:
      "Composer Secrets button accessible label when secrets are attached",
  },
  "agentSessions.secretsButtonCount_many": {
    message: "Secretos ({count} de adjuntos)",
    description:
      "Composer Secrets button accessible label when secrets are attached",
  },
  "agentSessions.secretsButtonCount_other": {
    message: "Secretos ({count} adjuntos)",
    description:
      "Composer Secrets button accessible label when secrets are attached",
  },
  "agentSessions.secretsTitle": {
    message: "Secretos de esta sesión",
    description: "Composer Secrets popover heading",
  },
  "agentSessions.secretsDescription": {
    message:
      "El agente lee cada uno como una variable de entorno. Los valores no se vuelven a mostrar y se ocultan en la transcripción.",
    description: "Composer Secrets popover description",
  },
  "agentSessions.secretsWorkspace": {
    message: "Secretos del espacio de trabajo",
    description:
      "Composer Secrets popover: workspace agent secrets list heading",
  },
  "agentSessions.secretsWorkspaceLoadFailed": {
    message:
      "No se pudieron cargar los secretos de agente del espacio de trabajo.",
    description:
      "Composer secrets panel error when the workspace secret list fails to load",
  },
  "agentSessions.secretsWorkspaceEmpty": {
    message: "Aún no hay secretos del espacio de trabajo.",
    description: "Composer Secrets popover: no workspace agent secrets",
  },
  "agentSessions.secretsManage": {
    message: "Administrar en Configuración",
    description: "Composer Secrets popover: link to Settings → Agents",
  },
  "agentSessions.secretsInline": {
    message: "Solo para esta sesión",
    description: "Composer Secrets popover: inline secrets heading",
  },
  "agentSessions.secretsRemove": {
    message: "Quitar {name}",
    description:
      "Composer Secrets popover: remove an inline secret (accessible label)",
  },
  "agentSessions.secretsNamePlaceholder": {
    message: "NOMBRE",
    description: "Composer Secrets popover: inline secret name placeholder",
  },
  "agentSessions.secretsNameLabel": {
    message: "Nombre del secreto",
    description: "Composer Secrets popover: inline secret name field label",
  },
  "agentSessions.secretsValuePlaceholder": {
    message: "Valor",
    description: "Composer Secrets popover: inline secret value placeholder",
  },
  "agentSessions.secretsValueLabel": {
    message: "Valor del secreto",
    description: "Composer Secrets popover: inline secret value field label",
  },
  "agentSessions.secretsAdd": {
    message: "Añadir",
    description: "Composer Secrets popover: add an inline secret",
  },
  "agentSessions.secretsHint": {
    message: "Se elimina con la sesión.",
    description: "Composer Secrets popover: inline secret lifetime hint",
  },
  "agentSessions.secretsFull": {
    message: "Una sesión admite como máximo 20 secretos.",
    description: "Composer Secrets popover: session secret limit reached",
  },
  "agentSessions.secretsDuplicate": {
    message: "Esta sesión ya tiene un secreto con ese nombre.",
    description: "Composer Secrets popover: duplicate inline secret name",
  },
  "agentSessions.secretsScopeRepo": {
    message: "Este repositorio",
    description:
      "Composer Secrets popover: hint beside a secret scoped to the chosen repository, included",
  },
  "agentSessions.secretsScopeOtherRepo": {
    message: "Solo en {repo}",
    description:
      "Composer Secrets popover: hint beside a secret scoped to another repository, not available here",
  },
  "agentSessions.appliedSecretsTitle": {
    message: "Secretos del último turno",
    description:
      "Session header: heading of the applied-secrets popover (names and sources, never values)",
  },
  "agentSessions.appliedSecretSource.inline": {
    message: "esta sesión",
    description:
      "Applied secret source: an inline secret given to this session only",
  },
  "agentSessions.appliedSecretSource.explicit": {
    message: "seleccionado",
    description: "Applied secret source: a workspace secret the session picked",
  },
  "agentSessions.appliedSecretSource.repo": {
    message: "repositorio",
    description:
      "Applied secret source: a workspace secret scoped to the session's repository",
  },
  "agentSessions.appliedSecretSource.workspace": {
    message: "espacio de trabajo",
    description:
      "Applied secret source: a workspace secret every session receives",
  },
  "agentSessions.metaSecrets_one": {
    message: "{count} secreto",
    description:
      "Detail header — number of attached secrets (singular, ADR098)",
  },
  "agentSessions.metaSecrets_many": {
    message: "{count} de secretos",
    description: "Detail header — number of attached secrets (plural, ADR098)",
  },
  "agentSessions.metaSecrets_other": {
    message: "{count} secretos",
    description: "Detail header — number of attached secrets (plural, ADR098)",
  },
  "agentSessions.skillsButton": {
    message: "Skills",
    description: "Composer toolbar: the Skills button label/tooltip (ADR099)",
  },
  "agentSessions.skillsButtonCount_one": {
    message: "Skills ({count} elegido)",
    description:
      "Composer Skills button accessible label when skills are chosen",
  },
  "agentSessions.skillsButtonCount_many": {
    message: "Skills ({count} de elegidos)",
    description:
      "Composer Skills button accessible label when skills are chosen",
  },
  "agentSessions.skillsButtonCount_other": {
    message: "Skills ({count} elegidos)",
    description:
      "Composer Skills button accessible label when skills are chosen",
  },
  "agentSessions.skillsTitle": {
    message: "Skills para este turno",
    description: "Composer Skills popover heading",
  },
  "agentSessions.skillsDescription": {
    message:
      "El agente sigue cada skill elegido. Los skills habilitados también puede aplicarlos por su cuenta.",
    description: "Composer Skills popover description",
  },
  "agentSessions.skillsLoadFailed": {
    message: "No se pudieron cargar los skills del espacio de trabajo.",
    description: "Composer Skills popover: list failed",
  },
  "agentSessions.skillsEmpty": {
    message: "Aún no hay skills en este espacio de trabajo.",
    description: "Composer Skills popover: none exist",
  },
  "agentSessions.skillsManage": {
    message: "Administrar skills",
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
    message: "Los argumentos son una sola línea, de hasta 4 KiB.",
    description: "Composer Skills popover: invalid arguments",
  },
  "agentSessions.skillsFull": {
    message: "Un turno puede usar como máximo 5 skills.",
    description: "Composer Skills popover: limit reached",
  },
  "agentSessions.skillsHint": {
    message: "Los argumentos rellenan $ARGUMENTS en el skill.",
    description: "Composer Skills popover hint",
  },
  "agentSessions.skillsRemove": {
    message: "Quitar {name}",
    description: "Accessible label of a chosen skill chip's remove button",
  },
  "agentSessions.turnSkills": {
    message: "Skills usados en este turno",
    description: "Accessible label of a transcript turn's skill pills",
  },
  "agentSessions.turnOrigins": {
    message: "Qué orientó este turno",
    description:
      "Accessible label of the lines above a transcript turn that PR feedback started (w9/m344)",
  },
  "agentSessions.turnOriginReview": {
    message: "Orientado por una revisión de @{author}",
    description:
      "A turn started by a pull-request review; {author} is the reviewer's GitHub login. Links to the review",
  },
  "agentSessions.turnOriginReviewAnonymous": {
    message: "Orientado por una revisión del PR",
    description:
      "A turn started by a pull-request review whose author is unknown. Links to the review",
  },
  "agentSessions.turnOriginComment": {
    message: "Orientado por un comentario de @{author} en el PR",
    description:
      "A turn started by a pull-request comment; {author} is the commenter's GitHub login. Links to the comment",
  },
  "agentSessions.turnOriginCommentAnonymous": {
    message: "Orientado por un comentario en el PR",
    description:
      "A turn started by a pull-request comment whose author is unknown. Links to the comment",
  },
  "agentSessions.turnOriginCiFailure": {
    message: "Orientado por una ejecución de CI fallida",
    description:
      "A turn started by a failed CI run on the pull request. Links to the failed checks",
  },
  "agentSessions.turnOriginCiFailureOnBase": {
    message:
      "Orientado por una ejecución de CI fallida; algunas comprobaciones también fallan en {base}, no por este cambio",
    description:
      "A turn started by a failed CI run on the pull request where at least one failing check also fails on the base branch ({base}, a branch name), so the pull request did not cause it. Links to the failed checks",
  },
  "agentSessions.turnOriginsStart": {
    message: "Qué inició esta sesión",
    description:
      "Accessible label of the line above a session's first turn when a /bex comment on a pull request started it (w9/m348)",
  },
  "agentSessions.turnOriginStartComment": {
    message: "Iniciada desde el comentario de @{author}",
    description:
      "The first turn of a session a /bex pull-request comment started; {author} is the commenter's GitHub login. Links to the comment",
  },
  "agentSessions.turnOriginStartCommentAnonymous": {
    message: "Iniciada desde un comentario en el PR",
    description:
      "The first turn of a session a /bex pull-request comment started, when the commenter is unknown. Links to the comment",
  },
  "agentSessions.lineage.childSessions": {
    message: "Sesiones hijas",
    description:
      "Heading of the block listing sessions the agent spawned, in the parent session's transcript",
  },
  "agentSessions.lineage.childCardTitle": {
    message: "Creó una sesión",
    description:
      "Small label above a child session's title on its card in the parent's transcript",
  },
  "agentSessions.lineage.agentsPanel": {
    message: "Agentes",
    description:
      "Heading of the session page's right-hand panel listing the sessions below this one",
  },
  "agentSessions.lineage.viewSession": {
    message: "Ver sesión",
    description: "Link on an Agents panel row that opens that child session",
  },
  "agentSessions.lineage.parentSession": {
    message: "Sesión padre",
    description:
      "Link in the Agents panel of a child session that opens its parent",
  },
  "agentSessions.lineage.collapsePanel": {
    message: "Contraer el panel Agentes",
    description:
      "Accessible label of the button that collapses the Agents panel",
  },
  "agentSessions.lineage.expandPanel": {
    message: "Expandir el panel Agentes",
    description: "Accessible label of the button that expands the Agents panel",
  },
  "agentSessions.lineage.collapseChildren": {
    message: "Ocultar sesiones hijas",
    description:
      "Accessible label of a sidebar row's button that collapses its nested child sessions",
  },
  "agentSessions.lineage.expandChildren": {
    message: "Mostrar sesiones hijas",
    description:
      "Accessible label of a sidebar row's button that expands its nested child sessions",
  },
  "agentSessions.lineage.status.working": {
    message: "Trabajando",
    description:
      "Status group and badge of a child session that is still running",
  },
  "agentSessions.lineage.status.done": {
    message: "Hecha",
    description:
      "Status group and badge of a child session that finished successfully",
  },
  "agentSessions.lineage.status.failed": {
    message: "Fallida",
    description: "Status group and badge of a child session that failed",
  },
  "agentSessions.lineage.status.canceled": {
    message: "Cancelada",
    description: "Status group and badge of a child session that was canceled",
  },
  "agentSessions.sidebarMine": {
    message: "Mías",
    description:
      "Sessions sidebar — show only the sessions you started (Devin's default)",
  },
  "agentSessions.sidebarEveryone": {
    message: "Todos",
    description: "Sessions sidebar — show every member's sessions",
  },
  "agentSessions.sidebarOwnerFilter": {
    message: "De quién son las sesiones",
    description:
      "Sessions sidebar — accessible name of the Mine/Everyone toggle",
  },
  "agentSessions.sidebarEmptyMine": {
    message:
      "No hay sesiones iniciadas por ti. Las sesiones más antiguas están en Todos.",
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
    message: "Editar etiquetas",
    description: "Session header menu — open the tag editor",
  },
  "agentSessions.editTagsTitle": {
    message: "Editar etiquetas",
    description: "Tag editor dialog title",
  },
  "agentSessions.editTagsDescription": {
    message:
      "Las etiquetas reemplazan las etiquetas actuales de la sesión. Sepáralas con comas o espacios; déjalo vacío para borrarlas.",
    description: "Tag editor dialog description",
  },
  "agentSessions.editTagsHint": {
    message:
      "Hasta 20 etiquetas: letras minúsculas, dígitos, dos puntos, guion bajo o guion.",
    description: "Tag editor dialog — the tag rules",
  },
  "agentSessions.editTagsSave": {
    message: "Guardar etiquetas",
    description: "Tag editor dialog — save button",
  },
  "agentSessions.editTagsSaved": {
    message: "Etiquetas guardadas",
    description: "Toast after the tags were replaced",
  },
};

export default esAgentSessions;
