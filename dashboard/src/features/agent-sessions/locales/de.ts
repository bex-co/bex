import type { TranslationEntry } from "@/i18n";

const deAgentSessions: Record<string, TranslationEntry> = {
  "agentSessions.pageTitle": {
    message: "Agent-Sitzungen",
    description: "Agent sessions list page heading and document title",
  },
  "agentSessions.listTitle": {
    message: "Zuletzt",
    description: "Agent sessions recents heading — matches the rail",
  },
  "agentSessions.colTask": {
    message: "Aufgabe",
    description: "Agent sessions table column header — the session's prompt",
  },
  "agentSessions.colRepo": {
    message: "Repository",
    description: "Agent sessions table column header — repo/branch",
  },
  "agentSessions.colAgent": {
    message: "Agent",
    description: "Agent sessions table column header — the driver agent",
  },
  "agentSessions.colPhase": {
    message: "Phase",
    description: "Agent sessions table column header — lifecycle phase chip",
  },
  "agentSessions.colPr": {
    message: "PR",
    description: "Agent sessions table column header — draft pull request",
  },
  "agentSessions.colCreated": {
    message: "Erstellt",
    description: "Agent sessions table column header — createdAt relative age",
  },
  "agentSessions.prBadge": {
    message: "#{number}",
    description: "Draft PR badge label — the pull-request number",
  },
  "agentSessions.prState.open": {
    message: "Offen",
    description:
      "PR state badge beside a session's PR link — the pull request is open (a draft counts as open)",
  },
  "agentSessions.prState.merged": {
    message: "Gemergt",
    description:
      "PR state badge beside a session's PR link — the pull request was merged",
  },
  "agentSessions.prState.closed": {
    message: "Geschlossen",
    description:
      "PR state badge beside a session's PR link — the pull request was closed without merging",
  },
  "agentSessions.prStateFilter": {
    message: "Nach Pull-Request-Status filtern",
    description:
      "Session list — accessible name of the Any/Open/Merged/Closed PR-state filter",
  },
  "agentSessions.prStateFilterAny": {
    message: "Beliebig",
    description:
      "Session list PR-state filter — the option that shows sessions whatever their PR state, including sessions with no PR",
  },
  "agentSessions.errorTitle": {
    message: "Sitzungen konnten nicht geladen werden.",
    description: "Agent sessions list error — one sentence, Retry beside it",
  },
  "agentSessions.retry": {
    message: "Erneut versuchen",
    description: "Retries a failed agent-sessions list request",
  },
  "agentSessions.emptyBody": {
    message: "Sitzungen, die du startest, erscheinen hier.",
    description: "Default empty — one quiet line under the composer",
  },
  "agentSessions.emptyArchivedTitle": {
    message: "Keine archivierten Sitzungen",
    description: "Archived-only list empty-state heading",
  },
  "agentSessions.emptyArchivedBody": {
    message: "Sitzungen, die du archivierst, erscheinen hier.",
    description: "Archived-only list empty-state body",
  },
  "agentSessions.emptyFilteredTitle": {
    message: "Keine passenden Sitzungen",
    description: "Phase-filtered list empty-state heading",
  },
  "agentSessions.emptyFilteredBody": {
    message: "Probiere eine andere Phase oder entferne die Filter.",
    description: "Phase-filtered list empty-state body",
  },
  "agentSessions.clearFilters": {
    message: "Filter zurücksetzen",
    description: "Clears agent-session membership and phase filters",
  },
  "agentSessions.phase.creating": {
    message: "Wird erstellt",
    description: "Agent session phase chip — sandbox is being created",
  },
  "agentSessions.phase.running": {
    message: "Läuft",
    description: "Agent session phase chip — a turn is in progress",
  },
  "agentSessions.phase.resuming": {
    message: "Wird fortgesetzt",
    description: "Agent session phase chip — session is resuming",
  },
  "agentSessions.phase.redispatching": {
    message: "Wird neu zugewiesen",
    description: "Agent session phase chip — session is being redispatched",
  },
  "agentSessions.phase.completed": {
    message: "Abgeschlossen",
    description: "Agent session phase chip — session finished successfully",
  },
  "agentSessions.phase.failed": {
    message: "Fehlgeschlagen",
    description: "Agent session phase chip — session failed",
  },
  "agentSessions.phase.canceling": {
    message: "Wird abgebrochen",
    description: "Agent session phase chip — cancellation in progress",
  },
  "agentSessions.phase.canceled": {
    message: "Abgebrochen",
    description: "Agent session phase chip — session was canceled",
  },
  "agentSessions.taskLabel": {
    message: "Aufgabe",
    description: "Composer — the prominent task prompt textarea label",
  },
  "agentSessions.taskPlaceholder": {
    message: "Beschreibe eine Coding-Aufgabe…",
    description: "Composer — task editor placeholder",
  },
  "agentSessions.mentionButton": {
    message: "Repository oder Sitzung hinzufügen",
    description:
      "Composer toolbar — opens the repository/session mention picker",
  },
  "agentSessions.repoChip": {
    message: "Repository {repo}",
    description: "Composer toolbar — accessible name of the selected-repo chip",
  },
  "agentSessions.keyboardHint": {
    message:
      "Eingabetaste zum Starten · Umschalt+Eingabetaste für eine neue Zeile",
    description: "Muted hint under the composer",
  },
  "agentSessions.connectGitHubTitle": {
    message: "Verbinde GitHub, um zu starten",
    description: "Composer callout when the workspace has no App repos",
  },
  "agentSessions.connectGitHubBody": {
    message:
      "Cloud-Agents brauchen ein verbundenes GitHub-Repository. Verbinde die GitHub App für diesen Workspace und komm dann zurück, um eine Sitzung zu starten.",
    description: "Composer callout body when there are no installation repos",
  },
  "agentSessions.connectGitHub": {
    message: "GitHub verbinden",
    description: "Composer CTA that starts the GitHub App install",
  },
  "agentSessions.connectGitHubSettings": {
    message: "Workspace-Einstellungen",
    description: "Secondary link to /workspace/settings from the GitHub CTA",
  },
  "agentSessions.exampleFixTests": {
    message: "Behebe die fehlschlagenden Tests und öffne einen Draft-PR",
    description: "First-run example prompt chip",
  },
  "agentSessions.exampleAddReadme": {
    message:
      "Füge eine README hinzu, die erklärt, wie das Projekt ausgeführt wird",
    description: "First-run example prompt chip",
  },
  "agentSessions.chipRemove": {
    message: "{name} entfernen",
    description: "Accessible label of a mention chip's remove button",
  },
  "agentSessions.mentionCategoryRepos": {
    message: "Repositories",
    description: "Mention picker — the repositories category row",
  },
  "agentSessions.mentionCategoryReposDesc": {
    message: "Beschränke die Sitzung auf ein verbundenes GitHub-Repository",
    description: "Mention picker — repositories category one-line description",
  },
  "agentSessions.mentionCategorySessions": {
    message: "Sitzungen",
    description: "Mention picker — the prior-sessions category row",
  },
  "agentSessions.mentionCategorySessionsDesc": {
    message: "Verweise auf eine frühere Agent-Sitzung als Kontext",
    description: "Mention picker — sessions category one-line description",
  },
  "agentSessions.mentionNoResults": {
    message: "Keine Treffer",
    description:
      "Mention picker — empty state when the fuzzy filter matches nothing",
  },
  "agentSessions.mentionReposEmpty": {
    message:
      "Keine Repositories — verbinde zuerst GitHub in den Workspace-Einstellungen.",
    description:
      "Mention picker — empty state when no installation repos exist",
  },
  "agentSessions.mentionSessionsEmpty": {
    message: "Noch keine Sitzungen",
    description:
      "Mention picker — empty state when the workspace has no sessions",
  },
  "agentSessions.mentionConnected": {
    message: "Über GitHub App verbunden",
    description: "Mention picker readiness footer — the repo is app-connected",
  },
  "agentSessions.mentionDefaultBranch": {
    message: "Standard-Branch: {branch}",
    description: "Mention picker readiness footer — the repo's default branch",
  },
  "agentSessions.agentLabel": {
    message: "Agent",
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
  "agentSessions.agent.deepseek": {
    message: "DeepSeek Harness",
    description:
      "Composer — agent option: deepseek (DeepSeek Harness, on DeepSeek Flash through OpenRouter)",
  },
  "agentSessions.submit": {
    message: "Sitzung starten",
    description: "Composer — submit button label",
  },
  "agentSessions.submitting": {
    message: "Wird gestartet…",
    description: "Composer — submit button label while the create is in flight",
  },
  "agentSessions.unavailableTitle": {
    message: "Agent-Sitzungen sind nicht konfiguriert",
    description: "Composer — house callout title when the feature is 503",
  },
  "agentSessions.unavailableBody": {
    message:
      "Diese Plattform hat Cloud-Agent-Sitzungen nicht aktiviert. Bitte deinen Betreiber, das Agent-Session-Gateway zu konfigurieren.",
    description: "Composer — house callout body when the feature is 503",
  },
  "agentSessions.dependencyUnavailableTitle": {
    message: "Agent-Sitzungen sind vorübergehend nicht verfügbar",
    description:
      "Composer — retryable dependency-outage title, distinct from configuration",
  },
  "agentSessions.dependencyUnavailableBody": {
    message:
      "Eine Abhängigkeit der Plattform hat gerade Probleme. Deine Einrichtung ist unverändert; warte einen Moment und versuche es erneut.",
    description:
      "Composer — retryable dependency-outage body that does not send the user to an operator",
  },
  "agentSessions.modelKeyMissingTitle": {
    message: "Füge zuerst einen Schlüssel für den Modellanbieter hinzu",
    description:
      "Composer pre-flight title when the workspace has no model key",
  },
  "agentSessions.modelKeyMissingBody": {
    message:
      "Dieser Workspace hat noch keinen Modellzugriff. Ein Workspace-Admin kann unter Einstellungen → Agents den eigenen Schlüssel des Workspace für den Modellanbieter hinzufügen; komm danach hierher zurück.",
    description:
      "Composer pre-flight body when the workspace has neither its own model key nor the bex-provided model (ADR091 D7)",
  },
  "agentSessions.modelKeyMissingAction": {
    message: "Einstellungen → Agents öffnen",
    description:
      "Composer pre-flight link to the Settings → Agents model key field (/workspace/settings#agents)",
  },
  "agentSessions.createErrorTitle": {
    message: "Die Sitzung konnte nicht gestartet werden",
    description: "Composer — error alert title when the create fails",
  },
  "agentSessions.detailTitle": {
    message: "Sitzung",
    description: "Agent-session detail page document title",
  },
  "agentSessions.untitled": {
    message: "Unbenannte Sitzung",
    description:
      "Last-resort session name when a session carries neither a repository nor a task prompt",
  },
  "agentSessions.backToList": {
    message: "Alle Sitzungen",
    description: "Detail page — back link to the /agents list",
  },
  "agentSessions.detailErrorTitle": {
    message: "Diese Sitzung konnte nicht geladen werden",
    description: "Detail page — heading when the session read fails",
  },
  "agentSessions.conversationTitle": {
    message: "Unterhaltung",
    description: "Detail page — the live conversation column card title",
  },
  "agentSessions.failureTitle": {
    message: "Sitzung fehlgeschlagen",
    description: "Detail page — heading over a failed session's reason",
  },
  "agentSessions.deliveryNoteTitle": {
    message: "Änderungen nicht vollständig übergeben",
    description:
      "Detail page — heading over a completed session's note that the agent left work uncommitted, unpushed or without a pull request",
  },
  "agentSessions.deliveryNoteHint": {
    message:
      "Sende eine Folgenachricht und bitte den Agent, das Committen, Pushen und Aktualisieren des Pull Requests abzuschließen.",
    description:
      "Detail page — the next step shown under an undelivered-work note",
  },
  "agentSessions.failureReasonFallback": {
    message: "Die Sitzung konnte nicht gestartet werden.",
    description:
      "Detail page — generic failure text when the session carries no specific reason",
  },
  "agentSessions.failureRetry": {
    message: "Erneut versuchen",
    description:
      "Detail page — button to re-run a failed session's original task",
  },
  "agentSessions.failureRetrying": {
    message: "Neuer Versuch…",
    description:
      "Detail page — retry button label while the redispatch is in flight",
  },
  "agentSessions.capacityFailureTitle": {
    message: "Sandbox-Kapazität erschöpft",
    description:
      "Detail page — heading when a session failed on the plan's sandbox limit",
  },
  "agentSessions.capacityFailureBody": {
    message:
      "Dieser Workspace hat das Limit seines Tarifs für gleichzeitige Sandboxes erreicht. Wechsle auf einen höheren Tarif, um mehr gleichzeitig auszuführen, oder beende eine inaktive Sitzung und versuche es erneut.",
    description:
      "Detail page — explains a plan-limit sandbox failure and the two remedies",
  },
  "agentSessions.upgradePlan": {
    message: "Tarif upgraden",
    description:
      "Detail page — CTA on a capacity failure; opens the change-plan dialog",
  },
  "agentSessions.provisioning": {
    message: "Sandbox wird gestartet…",
    description:
      "Detail page — shown while a new/steered session provisions its sandbox before the conversation stream is available",
  },
  "agentSessions.metaDuration": {
    message: "Dauer {duration}",
    description: "Detail header — elapsed session wall-clock",
  },
  "agentSessions.metaTurns_one": {
    message: "{count} Durchgang",
    description: "Detail header — number of prompt turns taken (singular)",
  },
  "agentSessions.metaTurns_other": {
    message: "{count} Durchgänge",
    description: "Detail header — number of prompt turns taken (plural)",
  },
  "agentSessions.cancel": {
    message: "Abbrechen",
    description: "Detail header — cancel-session button label",
  },
  "agentSessions.cancelDisabledCanceling": {
    message: "Der Abbruch läuft bereits.",
    description: "Detail header — tooltip on the disabled cancel button",
  },
  "agentSessions.canceling": {
    message: "Wird abgebrochen…",
    description: "Detail header — confirm button label while the cancel runs",
  },
  "agentSessions.cancelSuccess": {
    message: "Sitzung wird abgebrochen…",
    description: "Detail header — toast after the cancel is accepted",
  },
  "agentSessions.cancelConfirmTitle": {
    message: "Diese Sitzung abbrechen?",
    description: "Cancel confirm dialog title",
  },
  "agentSessions.cancelConfirmBody": {
    message:
      "Dadurch wird der Agent gestoppt. Bereits in den Branch gepushte Commits und der Draft-PR bleiben erhalten.",
    description: "Cancel confirm dialog body — states pushed work is preserved",
  },
  "agentSessions.cancelConfirmDismiss": {
    message: "Weiterlaufen lassen",
    description: "Cancel confirm dialog — dismiss button",
  },
  "agentSessions.cancelConfirmProceed": {
    message: "Sitzung abbrechen",
    description: "Cancel confirm dialog — proceed button",
  },
  "agentSessions.stopTurn": {
    message: "Durchgang stoppen",
    description:
      "Detail header — Stop turn button label: ends only the running turn, unlike Cancel which ends the session",
  },
  "agentSessions.stoppingTurn": {
    message: "Wird gestoppt…",
    description: "Detail header — Stop turn button label while the stop runs",
  },
  "agentSessions.stopTurnHint": {
    message:
      "Beendet nur den laufenden Durchgang. Die Sitzung bleibt offen, und Nachrichten in der Warteschlange behalten ihren Platz. Tastenkürzel: Ctrl/Cmd+Shift+Backspace.",
    description: "Detail header — tooltip on the Stop turn button",
  },
  "agentSessions.stopTurnStarting": {
    message:
      "Der Durchgang startet noch. Du kannst ihn stoppen, sobald er läuft.",
    description:
      "Detail header — tooltip on the disabled Stop turn button while the sandbox starts",
  },
  "agentSessions.stopTurnSuccess": {
    message:
      "Der Durchgang wird gestoppt. Er endet innerhalb weniger Sekunden.",
    description: "Detail header — toast after Stop turn is accepted",
  },
  "agentSessions.steerTitle": {
    message: "Diese Sitzung steuern",
    description: "Steering composer card title",
  },
  "agentSessions.steerErrorTitle": {
    message: "Senden fehlgeschlagen",
    description: "Steering composer — inline error alert title",
  },
  "agentSessions.steerPlaceholderIdle": {
    message:
      "Sende eine Folgeanweisung, um einen neuen Durchgang auf demselben Branch zu starten.",
    description: "Steering composer — textarea placeholder for an idle session",
  },
  "agentSessions.steerPlaceholderQueue": {
    message:
      "Stelle eine Nachricht in die Warteschlange. Sie läuft, wenn der aktuelle Durchgang endet.",
    description:
      "Steering composer — textarea placeholder while a turn runs or messages wait (ADR047 D12)",
  },
  "agentSessions.steerHintIdle": {
    message:
      "Diese Sitzung ist inaktiv — beim Senden einer Nachricht wird die Unterhaltung in einer frischen Sandbox auf demselben Branch fortgesetzt.",
    description: "Steering composer — helper text for the redispatch route",
  },
  "agentSessions.steerHintQueue": {
    message:
      "Der Agent arbeitet. Deine Nachricht steht in der Warteschlange und läuft als eigener Durchgang, wenn dieser endet.",
    description:
      "Steering composer — helper text while a turn runs or messages wait (ADR047 D12)",
  },
  "agentSessions.steerDisabledCanceling": {
    message:
      "Diese Sitzung wird abgebrochen. Bereits gepushte Arbeit bleibt erhalten.",
    description: "Steering composer — disabled reason while canceling",
  },
  "agentSessions.steerDisabledCanceled": {
    message: "Diese Sitzung wurde abgebrochen.",
    description: "Steering composer — disabled reason once canceled",
  },
  "agentSessions.steerDisabledStream": {
    message:
      "Der Unterhaltungs-Stream ist nicht verfügbar, daher ist die Live-Steuerung pausiert.",
    description:
      "Steering composer — disabled reason when the m43 stream is down",
  },
  "agentSessions.steerSubmit": {
    message: "Senden",
    description: "Steering composer — submit button label",
  },
  "agentSessions.steerSending": {
    message: "Wird gesendet…",
    description: "Steering composer — submit button label while sending",
  },
  "agentSessions.steerSuccess": {
    message: "Gesendet — ein neuer Durchgang wird zugewiesen.",
    description: "Steering composer — toast after a redispatch is accepted",
  },
  "agentSessions.steerQueued": {
    message:
      "In die Warteschlange gestellt. Sie läuft, wenn der aktuelle Durchgang endet.",
    description:
      "Steering composer — toast after a message was queued behind a running turn (ADR047 D12)",
  },
  "agentSessions.queueRegion": {
    message: "Nachrichten in der Warteschlange",
    description: "Queued messages list — accessible name of the region",
  },
  "agentSessions.queueTitle_one": {
    message: "{count} Nachricht in der Warteschlange",
    description: "Queued messages list — heading with the count (singular)",
  },
  "agentSessions.queueTitle_other": {
    message: "{count} Nachrichten in der Warteschlange",
    description: "Queued messages list — heading with the count (plural)",
  },
  "agentSessions.queueHint": {
    message:
      "Sie laufen nacheinander in dieser Reihenfolge, nach dem aktuellen Durchgang.",
    description:
      "Queued messages list — one-line explanation next to the heading",
  },
  "agentSessions.queuePosition": {
    message: "Platz {position} in der Warteschlange",
    description: "Queued messages list — tooltip on a row's place number",
  },
  "agentSessions.queueAttachments_one": {
    message: "{count} Anhang",
    description:
      "Queued message row — attachments that ride with it (singular)",
  },
  "agentSessions.queueAttachments_other": {
    message: "{count} Anhänge",
    description: "Queued message row — attachments that ride with it (plural)",
  },
  "agentSessions.queueBlocked": {
    message:
      "Konnte noch nicht starten: {reason}. Es wird automatisch erneut versucht.",
    description:
      "Queued message row — why the dispatcher could not start it (a spend ceiling or the live-sandbox limit)",
  },
  "agentSessions.queueSendNow": {
    message: "Jetzt senden",
    description:
      "Queued message row — stops the running turn and runs this message next",
  },
  "agentSessions.queueMoveUp": {
    message: "Nach oben",
    description: "Queued message row — move one place earlier",
  },
  "agentSessions.queueMoveDown": {
    message: "Nach unten",
    description: "Queued message row — move one place later",
  },
  "agentSessions.queueEdit": {
    message: "Bearbeiten",
    description: "Queued message row — edit the message in place",
  },
  "agentSessions.queueRemove": {
    message: "Entfernen",
    description: "Queued message row — remove it so it never runs",
  },
  "agentSessions.queueEditLabel": {
    message: "Nachricht in der Warteschlange bearbeiten",
    description: "Queued message row — accessible name of the edit box",
  },
  "agentSessions.queueEditSave": {
    message: "Speichern",
    description: "Queued message row — save the edit",
  },
  "agentSessions.queueEditSaveSendNow": {
    message: "Speichern und jetzt senden",
    description:
      "Queued message row — save the edit, stop the running turn and run this message next",
  },
  "agentSessions.queueEditDiscard": {
    message: "Verwerfen",
    description: "Queued message row — leave the edit without saving",
  },
  "agentSessions.queueEditHint": {
    message:
      "Enter speichert, Shift+Enter fügt eine Zeile hinzu, Ctrl/Cmd+Enter speichert und sendet sofort, Esc verwirft.",
    description: "Queued message row — keyboard hints while editing",
  },
  "agentSessions.queueSendNowStopping": {
    message:
      "Der aktuelle Durchgang wird gestoppt. Diese Nachricht läuft als Nächstes.",
    description: "Queued message row — toast after Send now stopped a turn",
  },
  "agentSessions.queueSendNowNext": {
    message: "Diese Nachricht läuft als Nächstes.",
    description:
      "Queued message row — toast after Send now when no turn was running",
  },
  "agentSessions.errors.AGENT_SESSION_INPUT_INVALID": {
    message:
      "Diese Eingabe ist ungültig. Prüfe Repository, Branch und Aufgabe und versuche es dann erneut.",
    description: "Mapped message for AGENT_SESSION_INPUT_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_ID_INVALID": {
    message: "Diese Sitzungs-ID ist ungültig.",
    description: "Mapped message for AGENT_SESSION_ID_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_FOUND": {
    message: "Diese Sitzung existiert nicht mehr.",
    description: "Mapped message for AGENT_SESSION_NOT_FOUND",
  },
  "agentSessions.errors.AGENT_SESSION_CONFLICT": {
    message:
      "Diese Sitzung ist mit einem anderen Vorgang beschäftigt. Versuche es gleich noch einmal.",
    description: "Mapped message for AGENT_SESSION_CONFLICT",
  },
  "agentSessions.errors.AGENT_SESSION_BRANCH_IN_USE": {
    message:
      "Eine andere Sitzung arbeitet bereits auf diesem Branch. Sende dieser Sitzung eine Nachricht oder verwende einen anderen bex-agent/-Branch.",
    description: "Mapped message for AGENT_SESSION_BRANCH_IN_USE (w5/m382)",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_STEERABLE": {
    message:
      "Diese Sitzung kann in ihrer aktuellen Phase ({phase}) nicht gesteuert werden.",
    description: "Mapped message for AGENT_SESSION_NOT_STEERABLE",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_RESUMABLE": {
    message:
      "Diese Sitzung kann in ihrer aktuellen Phase ({phase}) nicht fortgesetzt werden.",
    description: "Mapped message for AGENT_SESSION_NOT_RESUMABLE",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_ATTACHABLE": {
    message:
      "Mit dieser Sitzung kann man sich in ihrer aktuellen Phase ({phase}) nicht verbinden.",
    description: "Mapped message for AGENT_SESSION_NOT_ATTACHABLE",
  },
  "agentSessions.errors.AGENT_SESSION_TURN_IN_FLIGHT": {
    message:
      "Ein Durchgang läuft bereits. Warte, bis er beendet ist, bevor du einen weiteren sendest.",
    description: "Mapped message for AGENT_SESSION_TURN_IN_FLIGHT",
  },
  "agentSessions.errors.AGENT_SESSION_QUEUE_FULL": {
    message:
      "In dieser Sitzung warten bereits {max} Nachrichten. Warte, bis eine läuft, oder entferne eine, und sende dann erneut.",
    description: "Mapped message for AGENT_SESSION_QUEUE_FULL (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_QUEUE_INLINE_SECRETS": {
    message:
      "Eine Nachricht mit Inline-Secrets kann nicht in der Warteschlange warten, weil ihre Werte nie gespeichert werden. Sende sie, wenn die Sitzung inaktiv ist, oder nutze ein Workspace-Secret.",
    description:
      "Mapped message for AGENT_SESSION_QUEUE_INLINE_SECRETS (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_QUEUED_MESSAGE_DISPATCHED": {
    message:
      "Diese Nachricht läuft bereits und kann nicht mehr geändert werden.",
    description:
      "Mapped message for AGENT_QUEUED_MESSAGE_DISPATCHED (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_QUEUED_MESSAGE_NOT_FOUND": {
    message: "Diese Nachricht ist nicht mehr in der Warteschlange.",
    description:
      "Mapped message for AGENT_QUEUED_MESSAGE_NOT_FOUND (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_QUEUED_MESSAGE_ATTACHMENTS_OWNED": {
    message:
      "Diese Nachricht enthält Anhänge ihres Absenders, daher kann nur der Absender sie bearbeiten. Du kannst sie weiterhin verschieben oder entfernen.",
    description:
      "Mapped message for AGENT_QUEUED_MESSAGE_ATTACHMENTS_OWNED (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_NO_TURN_RUNNING": {
    message: "Es läuft kein Durchgang, daher gibt es nichts zu stoppen.",
    description:
      "Mapped message for AGENT_SESSION_NO_TURN_RUNNING (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_TURN_STARTING": {
    message:
      "Der Durchgang startet noch, daher gibt es noch nichts zu stoppen. Versuche es in ein paar Sekunden erneut.",
    description: "Mapped message for AGENT_SESSION_TURN_STARTING (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_MODEL_ENDPOINT_INVALID": {
    message: "Der Modell-Endpunkt muss eine gültige HTTPS-URL sein.",
    description: "Mapped message for AGENT_SESSION_MODEL_ENDPOINT_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_EGRESS_ALLOWLIST_INVALID": {
    message: "Der Egress-Allowlist-Eintrag „{entry}“ ist ungültig: {reason}",
    description: "Mapped message for AGENT_SESSION_EGRESS_ALLOWLIST_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_EGRESS_ALLOWLIST_IMMUTABLE": {
    message:
      "Die Egress-Allowlist kann nach dem Start der Sitzung nicht mehr geändert werden.",
    description: "Mapped message for AGENT_SESSION_EGRESS_ALLOWLIST_IMMUTABLE",
  },
  "agentSessions.errors.AGENT_SESSION_EGRESS_PHASE_INVALID": {
    message:
      "Die Egress-Allowlist kann in der aktuellen Phase dieser Sitzung nicht gesetzt werden.",
    description: "Mapped message for AGENT_SESSION_EGRESS_PHASE_INVALID",
  },
  "agentSessions.conversationLoading": {
    message: "Unterhaltung wird geladen…",
    description: "Placeholder while the client-only conversation column loads",
  },
  "agentSessions.conversationConnecting": {
    message: "Verbindung zum Sitzungs-Stream wird hergestellt…",
    description: "Shown while the conversation stream replay is in flight",
  },
  "agentSessions.conversationNotStarted": {
    message: "Die Unterhaltung erscheint, sobald die Sandbox bereit ist.",
    description:
      "Healthy pre-stream state while a newly created session is provisioning",
  },
  "agentSessions.conversationEmpty": {
    message: "Noch keine Unterhaltung.",
    description: "Shown when a session has produced no transcript parts",
  },
  "agentSessions.turnInterrupted": {
    message: "Durchgang {turn} wurde unterbrochen, bevor er beendet war.",
    description:
      "Transcript line at the end of a turn a person stopped with Stop turn (ADR047 D12); not a failure",
  },
  "agentSessions.turnRetentionSaving": {
    message:
      "Deine Änderungen werden gespeichert, damit die nächste Nachricht daran anknüpfen kann.",
    description:
      "Transcript line under a stopped turn while its unsaved work is being saved so the next message can continue from it (ADR047 D12, w22/m413)",
  },
  "agentSessions.turnRetentionKept": {
    message: "Deine Änderungen werden bis {time} aufbewahrt.",
    description:
      "Transcript line under a stopped turn: its unpushed changes are kept until {time}, a date and time in the viewer's own zone (ADR047 D12, w22/m413)",
  },
  "agentSessions.turnRetentionKeptNoTime": {
    message: "Deine Änderungen werden für deine nächste Nachricht aufbewahrt.",
    description:
      "Same as the kept line before the browser knows the viewer's time zone, so it names no time",
  },
  "agentSessions.turnRetentionContinued": {
    message: "Die nächste Nachricht hat mit diesen Änderungen weitergemacht.",
    description:
      "Transcript line under a stopped turn: the next message started from its kept changes",
  },
  "agentSessions.turnRetentionLostExpired": {
    message:
      "Deine nicht gepushten Änderungen gingen verloren, als die aufbewahrte Sandbox ablief.",
    description:
      "Transcript line under a stopped turn: its kept sandbox expired and the unpushed changes in it are gone",
  },
  "agentSessions.turnRetentionLostStorageFull": {
    message:
      "Deine nicht gepushten Änderungen wurden nicht aufbewahrt, weil der Sandbox-Speicher des Workspace voll war.",
    description:
      "Transcript line under a stopped turn: nothing was kept because the workspace's sandbox storage cap was full",
  },
  "agentSessions.turnRetentionLostSaveFailed": {
    message:
      "Deine nicht gepushten Änderungen konnten nicht gespeichert werden und gingen verloren.",
    description:
      "Transcript line under a stopped turn: saving its sandbox failed, so the unpushed changes are gone",
  },
  "agentSessions.turnRetentionLostEditorOpen": {
    message:
      "Deine nicht gepushten Änderungen wurden nicht aufbewahrt, weil in der Sandbox ein Editor geöffnet war.",
    description:
      "Transcript line under a stopped turn: nothing was kept because an editor (SSH) was open on the sandbox, which Stop does not end",
  },
  "agentSessions.turnRetentionLostNotRestored": {
    message:
      "Deine nicht gepushten Änderungen konnten nicht wiederhergestellt werden, daher begann die nächste Nachricht mit einem frischen Checkout.",
    description:
      "Transcript line under a stopped turn: the next message could not start from the kept changes and started from a fresh checkout",
  },
  "agentSessions.turnRetentionLost": {
    message: "Deine nicht gepushten Änderungen gingen verloren.",
    description:
      "Transcript line under a stopped turn: its unpushed changes were lost, for a reason that is not worth naming (the session was canceled or archived)",
  },
  "agentSessions.turnError": {
    message: "Durchgang {turn} fehlgeschlagen",
    description:
      "Recorded failure of an earlier turn; subsequent turns remain readable",
  },
  "agentSessions.continuitySessionLoad": {
    message:
      "Der Agent wurde mit seinem wiederhergestellten Unterhaltungsstatus fortgesetzt.",
    description:
      "System line when a fresh agent generation reloaded its own saved session (ADR047 D3 ladder rung 1)",
  },
  "agentSessions.continuityReprime": {
    message:
      "Agent neu gestartet — Kontext aus dem Sitzungsverlauf neu aufgebaut.",
    description:
      "System line when a fresh agent generation was primed from the durable transcript (ladder rung 2)",
  },
  "agentSessions.continuityTaskRedelivery": {
    message:
      "Agent neu gestartet — die ursprüngliche Aufgabe wurde erneut zugestellt.",
    description:
      "System line when no prior turn ever reached the agent, so the task was re-sent (ladder rung 3)",
  },
  "agentSessions.conversationEndedEmpty": {
    message:
      "Diese Sitzung endete, bevor eine Unterhaltung aufgezeichnet wurde.",
    description: "Ended session with no durable transcript parts",
  },
  "agentSessions.conversationEnded": {
    message: "Sitzung beendet.",
    description: "Footer note under a terminal session's replayed transcript",
  },
  "agentSessions.conversationIncomplete": {
    message: "Ein Teil der Assistant-Ausgabe konnte nicht gesichert werden",
    description:
      "Warning below a durable user turn whose assistant transcript is incomplete",
  },
  "agentSessions.conversationUnavailable": {
    message: "Der Unterhaltungs-Stream ist gerade nicht verfügbar.",
    description:
      "Degraded-state message when the m43 stream endpoint errors or is unconfigured",
  },
  "agentSessions.conversationUnavailableTerminal": {
    message:
      "Das Unterhaltungsprotokoll ist für diese Sitzung nicht verfügbar.",
    description:
      "Terminal session — transcript unavailable rather than live stream degraded",
  },
  "agentSessions.showEarlierMessages_one": {
    message: "{count} frühere Nachricht anzeigen",
    description:
      "Button revealing older transcript messages hidden by the render window (singular)",
  },
  "agentSessions.showEarlierMessages_other": {
    message: "{count} frühere Nachrichten anzeigen",
    description:
      "Button revealing older transcript messages hidden by the render window (plural)",
  },
  "agentSessions.groupThought": {
    message: "Überlegt",
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
    message: "Befehl",
    description: "Collapsible header for a command/tool-call group",
  },
  "agentSessions.groupDiff": {
    message: "Diff",
    description: "Label for a file-diff group when no path is given",
  },
  "agentSessions.terminalNoOutput": {
    message: "(keine Ausgabe)",
    description: "Placeholder when a terminal group has no captured output",
  },
  "agentSessions.toolInput": {
    message: "Eingabe",
    description: "Section label for a tool call's input",
  },
  "agentSessions.toolOutput": {
    message: "Ausgabe",
    description: "Section label for a tool call's output",
  },
  "agentSessions.toolError": {
    message: "Fehler",
    description: "Section label for a tool call's error text",
  },
  "agentSessions.toolStateRunning": {
    message: "Läuft",
    description: "Badge label for a tool call awaiting output",
  },
  "agentSessions.toolStateDone": {
    message: "Fertig",
    description: "Badge label for a completed tool call",
  },
  "agentSessions.toolStateDenied": {
    message: "Abgelehnt",
    description: "Badge label for a tool call whose execution was denied",
  },
  "agentSessions.toolStateInterrupted": {
    message: "Unterbrochen",
    description:
      "Badge label for a tool call the session ended before it finished",
  },
  "agentSessions.toolStateError": {
    message: "Fehlgeschlagen",
    description: "Badge label for a failed tool call",
  },
  "agentSessions.activityEdited_one": {
    message: "{count} Datei bearbeitet",
    description:
      "Activity-group summary when the turn edited files (diffs, singular)",
  },
  "agentSessions.activityEdited_other": {
    message: "{count} Dateien bearbeitet",
    description: "Activity-group summary when the turn edited files (diffs)",
  },
  "agentSessions.activityRan_one": {
    message: "{count} Befehl ausgeführt",
    description:
      "Activity-group summary when the turn ran commands/terminals (singular)",
  },
  "agentSessions.activityRan_other": {
    message: "{count} Befehle ausgeführt",
    description: "Activity-group summary when the turn ran commands/terminals",
  },
  "agentSessions.activitySteps_one": {
    message: "{count} Schritt",
    description: "Activity-group summary fallback — the step count (singular)",
  },
  "agentSessions.activitySteps_other": {
    message: "{count} Schritte",
    description: "Activity-group summary fallback — the step count (plural)",
  },
  "agentSessions.activityRead_one": {
    message: "{count} Datei gelesen",
    description: "Activity-group summary segment — file reads (singular)",
  },
  "agentSessions.activityRead_other": {
    message: "{count} Dateien gelesen",
    description: "Activity-group summary segment — file reads",
  },
  "agentSessions.activitySearched_one": {
    message: "{count} Mal gesucht",
    description: "Activity-group summary segment — searches (singular)",
  },
  "agentSessions.activitySearched_other": {
    message: "{count} Mal gesucht",
    description: "Activity-group summary segment — searches",
  },
  "agentSessions.activitySubagents_one": {
    message: "{count} Subagent ausgeführt",
    description:
      "Activity-group summary segment — delegated subagent tasks (singular)",
  },
  "agentSessions.activitySubagents_other": {
    message: "{count} Subagents ausgeführt",
    description: "Activity-group summary segment — delegated subagent tasks",
  },
  "agentSessions.activityTools_one": {
    message: "{count} Tool verwendet",
    description:
      "Activity-group summary segment — tools of no known kind (singular)",
  },
  "agentSessions.activityTools_other": {
    message: "{count} Tools verwendet",
    description: "Activity-group summary segment — tools of no known kind",
  },
  "agentSessions.activityThought_one": {
    message: "{count} Mal überlegt",
    description:
      "Activity-group summary segment — the agent's thoughts between its calls (singular)",
  },
  "agentSessions.activityThought_other": {
    message: "{count} Mal überlegt",
    description:
      "Activity-group summary segment — the agent's thoughts between its calls",
  },
  "agentSessions.activityThinking": {
    message: "Überlegt…",
    description: "Live status line while the agent is thinking between calls",
  },
  "agentSessions.activityFailed_one": {
    message: "{count} fehlgeschlagen",
    description:
      "Activity-group summary segment — failed tool calls (singular)",
  },
  "agentSessions.activityFailed_other": {
    message: "{count} fehlgeschlagen",
    description: "Activity-group summary segment — failed tool calls",
  },
  "agentSessions.outputClipped_one": {
    message: "{formatted} Zeichen gekürzt",
    description:
      "Badge on a tool output the server stored head+tail-capped (singular)",
  },
  "agentSessions.outputClipped_other": {
    message: "{formatted} Zeichen gekürzt",
    description: "Badge on a tool output the server stored head+tail-capped",
  },
  "agentSessions.showFullOutput": {
    message: "Gesamte Ausgabe anzeigen",
    description:
      "Button on a clipped tool output that loads the complete text the server kept",
  },
  "agentSessions.hideFullOutput": {
    message: "Gesamte Ausgabe ausblenden",
    description: "Button that collapses a loaded full tool output",
  },
  "agentSessions.fullOutputNotRetained": {
    message: "Gesamte Ausgabe nicht aufbewahrt",
    description:
      "Note on a clipped tool output whose complete text the server did not keep (clipped before full copies were stored)",
  },
  "agentSessions.fullOutputLoading": {
    message: "Gesamte Ausgabe wird geladen…",
    description: "Status while a tool call's full output loads",
  },
  "agentSessions.fullOutputError": {
    message: "Die gesamte Ausgabe konnte nicht geladen werden.",
    description: "Inline error when loading a tool call's full output failed",
  },
  "agentSessions.fullOutputCopy": {
    message: "Gesamte Ausgabe kopieren",
    description:
      "Accessible label of the copy button on a loaded full tool output",
  },
  "agentSessions.fullOutputCopied": {
    message: "Gesamte Ausgabe kopiert",
    description: "Toast after copying a full tool output",
  },
  "agentSessions.fullOutputCopyError": {
    message: "Die gesamte Ausgabe konnte nicht kopiert werden",
    description: "Toast when copying a full tool output failed",
  },
  "agentSessions.conversationPartial": {
    message: "Die Unterhaltung wurde nur teilweise geladen.",
    description:
      "Inline row when the transcript stream failed after some of it rendered",
  },
  "agentSessions.liveStatus": {
    message: "Agent-Status",
    description:
      "Accessible label for the single live status line under a streaming transcript",
  },
  "agentSessions.scrollToBottom": {
    message: "Zum Neuesten scrollen",
    description:
      "Accessible label for the floating jump-to-bottom button in the conversation column",
  },
  "agentSessions.recentSessions": {
    message: "Zuletzt",
    description: "Sessions sidebar — heading over the recent sessions list",
  },
  "agentSessions.sidebarEmpty": {
    message: "Noch keine Sitzungen",
    description: "Sessions sidebar — empty state when the workspace has none",
  },
  "agentSessions.sidebarLabel": {
    message: "Agent-Sitzungen",
    description: "Sessions sidebar — accessible landmark label",
  },
  "agentSessions.sidebarSearch": {
    message: "Sitzungen durchsuchen",
    description: "Sessions sidebar — search toggle + input accessible label",
  },
  "agentSessions.sidebarSearchPlaceholder": {
    message: "Sitzungen durchsuchen…",
    description: "Sessions sidebar — search input placeholder",
  },
  "agentSessions.sidebarNoMatches": {
    message: "Keine passenden Sitzungen",
    description:
      "Sessions sidebar — empty state when the search matches nothing",
  },
  "agentSessions.statusPhrase.prReady": {
    message: "PR ist bereit",
    description: "Sidebar status phrase — completed session with a draft PR",
  },
  "agentSessions.statusPhrase.working": {
    message: "Arbeitet…",
    description: "Sidebar status phrase — session still converging",
  },
  "agentSessions.menuMore": {
    message: "Weitere Aktionen",
    description: "Header — accessible label for the '…' overflow menu",
  },
  "agentSessions.openPr": {
    message: "Pull Request öffnen",
    description: "Header overflow menu — open the draft PR in a new tab",
  },
  "agentSessions.connect": {
    message: "Verbinden",
    description: "Header — trigger for the Open-in-Zed / SSH connect menu",
  },
  "agentSessions.openInZed": {
    message: "In Zed öffnen",
    description:
      "Connect menu — hotlink that opens the sandbox as a Zed remote project",
  },
  "agentSessions.openInZedHint": {
    message:
      "Öffnet das /workspace der Sandbox über SSH. Erfordert den Zed-Editor.",
    description: "Connect menu — helper text under the Open-in-Zed action",
  },
  "agentSessions.connectSSH": {
    message: "SSH",
    description: "Connect menu — label above the copyable ssh command",
  },
  "agentSessions.sshCopy": {
    message: "SSH-Befehl kopieren",
    description: "Connect menu — accessible label for the copy button",
  },
  "agentSessions.sshCopied": {
    message: "SSH-Befehl kopiert",
    description: "Connect menu — toast after copying the ssh command",
  },
  "agentSessions.sshCopyError": {
    message: "Kopieren fehlgeschlagen",
    description: "Connect menu — toast when copying the ssh command fails",
  },
  "agentSessions.groupThoughtFor": {
    message: "{duration} überlegt",
    description:
      "Reasoning-group summary with elapsed duration from persisted part timestamps (or ~arrival timing)",
  },
  "agentSessions.terminalStatus.completed": {
    message: "Agent fertig",
    description: "Terminal transcript status line — completed session",
  },
  "agentSessions.terminalStatus.failed": {
    message: "Sitzung mit einem Fehler beendet",
    description: "Terminal transcript status line — failed session",
  },
  "agentSessions.terminalStatus.canceled": {
    message: "Sitzung wurde abgebrochen",
    description: "Terminal transcript status line — canceled session",
  },
  "agentSessions.tagsLabel": {
    message: "Tags",
    description: "Accessible label for a session row's tag list (w21/002)",
  },
  "agentSessions.filterByTag": {
    message: "Sitzungen mit dem Tag {tag} anzeigen",
    description:
      "Accessible label of a tag chip that filters the session list to that tag",
  },
  "agentSessions.filterTagged": {
    message: "Getaggt mit {tag}",
    description: "Session list heading while narrowed to one tag",
  },
  "agentSessions.archive": {
    message: "Archivieren",
    description: "Action putting a session out of the working set (ADR065)",
  },
  "agentSessions.unarchive": {
    message: "Aus Archiv holen",
    description: "Action returning an archived session to the working set",
  },
  "agentSessions.archiveSuccess": {
    message: "Sitzung archiviert",
    description: "Toast after archiving a session",
  },
  "agentSessions.undoArchive": {
    message: "Rückgängig",
    description: "Toast action that immediately unarchives a session",
  },
  "agentSessions.unarchiveSuccess": {
    message: "Sitzung aus dem Archiv geholt",
    description: "Toast after unarchiving a session",
  },
  "agentSessions.archivedBadge": {
    message: "Archiviert",
    description: "Badge marking a session as archived (out of the working set)",
  },
  "agentSessions.startedFromSnapshot": {
    message: "Aus Snapshot gestartet",
    description:
      "Session header chip: the latest turn booted from a repository environment build (ADR111 D4)",
  },
  "agentSessions.startedFromSnapshotTitle": {
    message:
      "Der letzte Durchgang wurde aus dem Umgebungs-Build {id} gestartet, wobei die Setup-Schritte bereits ausgeführt waren.",
    description:
      "Tooltip of the Started from snapshot chip (id is the build id, arb-…)",
  },
  "agentSessions.delete": {
    message: "Löschen",
    description: "Destructive action permanently deleting a finished session",
  },
  "agentSessions.deleting": {
    message: "Wird gelöscht…",
    description: "Delete confirm button while the delete is in flight",
  },
  "agentSessions.deleteSuccess": {
    message: "Sitzung gelöscht",
    description: "Toast after permanently deleting a session",
  },
  "agentSessions.deleteConfirmTitle": {
    message: "Diese Sitzung löschen?",
    description: "Delete confirmation dialog title",
  },
  "agentSessions.deleteConfirmBody": {
    message:
      "Der Sitzungseintrag und sein Unterhaltungsprotokoll werden dauerhaft gelöscht. Gepushte Branches und Pull Requests auf GitHub bleiben unberührt. Das lässt sich nicht rückgängig machen.",
    description: "Delete confirmation dialog body",
  },
  "agentSessions.deleteConfirmDismiss": {
    message: "Sitzung behalten",
    description: "Delete confirmation dialog dismiss button",
  },
  "agentSessions.deleteConfirmProceed": {
    message: "Sitzung löschen",
    description: "Delete confirmation dialog destructive proceed button",
  },
  "agentSessions.colActions": {
    message: "Aktionen",
    description: "Screen-reader label of the list's trailing actions column",
  },
  "agentSessions.filterActive": {
    message: "Zuletzt",
    description: "List membership tab — the unarchived working set",
  },
  "agentSessions.filterArchived": {
    message: "Archiviert",
    description: "List membership tab — archived sessions only",
  },
  "agentSessions.filterAll": {
    message: "Alle",
    description: "List membership tab — archived and unarchived together",
  },
  "agentSessions.loadMore": {
    message: "Mehr laden",
    description: "Button fetching the next page of sessions",
  },
  "agentSessions.loadingMore": {
    message: "Wird geladen…",
    description: "Load-more button while the next page is in flight",
  },
  "agentSessions.attachFiles": {
    message: "Dateien anhängen",
    description:
      "Paperclip button that opens the file picker in the agent composers",
  },
  "agentSessions.attachmentRemove": {
    message: "{name} entfernen",
    description: "Accessible label of the remove button on an attachment chip",
  },
  "agentSessions.attachmentTooLarge": {
    message: "Größer als {max}",
    description: "Attachment chip error when a file exceeds the per-file limit",
  },
  "agentSessions.attachmentTooMany": {
    message: "Bis zu {max} Dateien pro Nachricht",
    description: "Attachment chip error when a message has too many files",
  },
  "agentSessions.attachmentDropHint": {
    message: "Dateien zum Anhängen hierher ziehen",
    description: "Overlay shown while dragging files over a composer",
  },
  "agentSessions.sharedFileLabel": {
    message: "Hat eine Datei geteilt",
    description: "Label above a file the agent shared with share_file",
  },
  "agentSessions.filesButton": {
    message: "Dateien",
    description: "Session header button that opens the session files",
  },
  "agentSessions.filesTitle": {
    message: "Sitzungsdateien",
    description: "Title of the session files sheet",
  },
  "agentSessions.filesDescription": {
    message:
      "Dateien, die du angehängt hast, und Dateien, die der Agent in dieser Sitzung geteilt hat.",
    description: "Description under the session files sheet title",
  },
  "agentSessions.filesEmpty": {
    message:
      "Noch keine Dateien. Hänge Dateien an eine Nachricht an oder bitte den Agent, eine zu teilen.",
    description: "Empty state of the session files sheet",
  },
  "agentSessions.filesError": {
    message: "Die Dateien der Sitzung konnten nicht geladen werden.",
    description: "Error state of the session files sheet",
  },
  "agentSessions.filesColumnName": {
    message: "Name",
    description: "Files table column: file name",
  },
  "agentSessions.filesColumnFrom": {
    message: "Von",
    description: "Files table column: who added the file",
  },
  "agentSessions.filesColumnTurn": {
    message: "Durchgang",
    description: "Files table column: turn number",
  },
  "agentSessions.filesColumnSize": {
    message: "Größe",
    description: "Files table column: file size",
  },
  "agentSessions.filesFromInput": {
    message: "Du",
    description: "Files table: a file the user attached",
  },
  "agentSessions.filesFromOutput": {
    message: "Agent",
    description: "Files table: a file the agent shared",
  },
  "agentSessions.filesLoadMore": {
    message: "Mehr laden",
    description: "Files table: load the next page",
  },
  "agentSessions.attachmentsBlockSend": {
    message:
      "Entferne die Dateien, deren Upload fehlgeschlagen ist, um zu senden",
    description: "Hint when failed attachment chips block sending",
  },
  "agentSessions.attachmentOpen": {
    message: "{name} öffnen",
    description:
      "Accessible label of an attachment thumbnail that opens the lightbox",
  },
  "agentSessions.attachmentDownload": {
    message: "{name} herunterladen",
    description: "Accessible label of an attachment download card",
  },
  "agentSessions.attachmentUnavailable": {
    message: "Nicht verfügbar",
    description:
      "Shown on an attachment whose download link could not be loaded",
  },
  "agentSessions.attachmentUploading": {
    message: "Wird hochgeladen, {percent} %",
    description: "Screen-reader status of an attachment chip while uploading",
  },
  "agentSessions.attachmentReady": {
    message: "Hochgeladen",
    description:
      "Screen-reader status of an attachment chip once its upload landed",
  },
  "agentSessions.attachmentHttpError": {
    message: "Upload fehlgeschlagen (HTTP {status})",
    description:
      "Attachment chip error when the server answered without a message",
  },
  "agentSessions.attachmentNetworkError": {
    message: "Upload fehlgeschlagen: Netzwerkfehler",
    description:
      "Attachment chip error when the upload could not reach the server",
  },
  "agentSessions.attachmentsUploading": {
    message: "Warte, bis die Dateien fertig hochgeladen sind…",
    description: "Composer hint while attachment uploads block sending",
  },
  "agentSessions.spendWarningTitle": {
    message:
      "Die Ausgaben für Agent-Modelle liegen bei {percent} % des Limits dieses Monats",
    description: "Composer warning at 80% of the spend cap (ADR091 D6)",
  },
  "agentSessions.spendWarningBody": {
    message:
      "{used} von {cap} verbraucht. Neue Modellanfragen stoppen am Limit.",
    description: "Composer warning body",
  },
  "agentSessions.spendWarningBodyCharge": {
    message:
      "{used} von {cap} über das Abrechnungsguthaben hinaus verbraucht. Abrechnungsguthaben wird zuerst verwendet; sobald die darüber hinausgehenden Ausgaben {cap} erreichen, wird die hinterlegte Karte belastet und Sitzungen laufen weiter. Neue Modellanfragen stoppen nur, wenn die Belastung fehlschlägt.",
    description:
      "Composer warning body when the cap is a new workspace's charge point (ADR091 D10.2a); {used} is the spend billing credits leave",
  },
  "agentSessions.spendWarningBodyCeiling": {
    message:
      "{used} von {cap} über das Abrechnungsguthaben hinaus verbraucht. Abrechnungsguthaben wird zuerst verwendet; neue Modellanfragen stoppen, sobald die darüber hinausgehenden Ausgaben {cap} erreichen.",
    description:
      "Composer warning body when the trust tier's ceiling binds on a workspace bex cannot charge (payment hold, excluded or comped) and the spend it counts, what billing credits leave ({used}), differs from month-to-date",
  },
  "agentSessions.spendReachedTitle.cap": {
    message: "Monatliches Ausgabenlimit für Agent-Modelle erreicht",
    description: "Composer refusal title: the workspace's own cap",
  },
  "agentSessions.spendReachedTitle.newWorkspace": {
    message: "Ausgabenlimit für neue Workspaces erreicht",
    description: "Composer refusal title: the T0 trust-tier ceiling",
  },
  "agentSessions.spendReachedTitle.locked": {
    message: "Ausgaben für Agent-Modelle sind angehalten",
    description: "Composer refusal title: a payment hold",
  },
  "agentSessions.spendReachedBody.cap": {
    message:
      "Dieser Workspace hat {used} seines monatlichen Limits von {cap} verbraucht, daher können keine neuen Sitzungen starten. Ein Workspace-Admin kann das Limit erhöhen, oder es wird am 1. (UTC) zurückgesetzt.",
    description: "Composer refusal body: the workspace's own cap",
  },
  "agentSessions.spendReachedBody.capMax": {
    message:
      "Dieser Workspace hat {used} seines monatlichen Limits von {cap} verbraucht, dem Höchstwert, auf den es sich setzen lässt, daher können keine neuen Sitzungen starten. Das Limit wird am 1. (UTC) zurückgesetzt; wende dich an den Support, wenn du ein höheres Limit brauchst.",
    description:
      "Composer refusal body: the workspace's own cap binds at its maximum, so no admin can raise it",
  },
  "agentSessions.spendReachedBody.newWorkspace": {
    message:
      "Neue Workspaces können bis zu {cap} für das von bex bereitgestellte Modell ausgeben, bis ihre erste Zahlung erfolgreich war. Danach steigt das Limit, oder es wird am 1. (UTC) zurückgesetzt.",
    description: "Composer refusal body: the T0 trust-tier ceiling",
  },
  "agentSessions.spendReachedBody.locked": {
    message:
      "Eine fehlgeschlagene oder angefochtene Zahlung begrenzt die Ausgaben dieses Workspaces für Agent-Modelle auf {cap}. Begleiche die offene Zahlung unter Abrechnung und starte dann eine neue Sitzung.",
    description: "Composer refusal body: a payment hold",
  },
  "agentSessions.spendIncreaseCap": {
    message: "Limit erhöhen",
    description: "Admin action linking to Settings → Agents",
  },
  "agentSessions.spendGoToBilling": {
    message: "Zur Abrechnung",
    description: "Link to the Billing page",
  },
  "agentSessions.spendBalanceDueTitle": {
    message: "Eine Zahlung ist fehlgeschlagen",
    description: "Composer refusal title: BILLING_BALANCE_DUE",
  },
  "agentSessions.spendBalanceDueBody": {
    message:
      "Agent-Sitzungen mit dem von bex bereitgestellten Modell sind pausiert, bis die offene Rechnung bezahlt ist.",
    description: "Composer refusal body: BILLING_BALANCE_DUE",
  },
  "agentSessions.errors.AGENT_SESSION_TRUST_LIMIT": {
    message:
      "Neue Workspaces können bis zu ihrer ersten erfolgreichen Zahlung höchstens {limit} Agent-Sitzungen gleichzeitig mit dem von bex bereitgestellten Modell ausführen. Warte, bis eine laufende Sitzung beendet ist, oder beende eine, und versuche es dann erneut.",
    description: "Mapped message for AGENT_SESSION_TRUST_LIMIT (ADR091 D10.1)",
  },
  "agentSessions.errors.AGENT_SESSION_TRUST_LIMIT_locked": {
    message:
      "Nach einer fehlgeschlagenen oder angefochtenen Zahlung kann dieser Workspace höchstens {limit} Agent-Sitzungen gleichzeitig mit dem von bex bereitgestellten Modell ausführen. Begleiche die offene Zahlung unter Abrechnung, um die Einschränkung aufzuheben.",
    description:
      "Mapped message for AGENT_SESSION_TRUST_LIMIT while a payment hold applies",
  },
  "agentSessions.errors.AGENT_SESSION_SPEND_LIMIT_INVALID": {
    message: "Das Ausgabenlimit muss über 0 $ und höchstens 100.000 $ liegen.",
    description:
      "Mapped message for AGENT_SESSION_SPEND_LIMIT_INVALID (ADR094 D8)",
  },
  "agentSessions.errors.AGENT_SESSION_SPEND_LIMIT_UNAVAILABLE": {
    message:
      "Ein Ausgabenlimit gilt nur für Sitzungen mit dem von bex bereitgestellten Modell. Dieser Workspace verwendet seinen eigenen Modellschlüssel; entferne daher das Limit oder stelle den Workspace unter Einstellungen → Agents auf das bex-Modell um.",
    description:
      "Mapped message for AGENT_SESSION_SPEND_LIMIT_UNAVAILABLE (ADR094 D8)",
  },
  "agentSessions.spendLimitLabel": {
    message: "Ausgabenlimit (USD)",
    description:
      "Composer — label/aria-label of the optional per-session spend ceiling input",
  },
  "agentSessions.spendLimitPlaceholder": {
    message: "Ausgabenlimit",
    description:
      "Composer — placeholder of the optional per-session spend ceiling input",
  },
  "agentSessions.spendLimitHint": {
    message:
      "Optional. Die Sitzung stoppt, sobald ihre Ausgaben für das bex-Modell diesen Betrag erreichen (über 0 $, höchstens 100.000 $).",
    description: "Composer — help text under the spend limit input",
  },
  "agentSessions.spendLimitInvalid": {
    message:
      "Gib einen Betrag über 0 $ und höchstens 100.000 $ ein oder lass das Feld leer.",
    description: "Composer — inline error for an out-of-range spend limit",
  },
  "agentSessions.failureSessionSpendLimitReached": {
    message:
      "Diese Sitzung wurde gestoppt, weil ihre Modellausgaben das Sitzungslimit von {limit} erreicht haben. Starte eine neue Sitzung mit einem höheren Limit, um fortzufahren.",
    description:
      "Failure callout for AGENT_MODEL_SPEND_CAP_REACHED when the session's own maxSpendUSD ceiling was hit",
  },
  "agentSessions.spendNote": {
    message: "Modellausgaben {spent}",
    description:
      "Session page — the session's model spend so far (outcome.cost)",
  },
  "agentSessions.spendNoteWithLimit": {
    message: "Modellausgaben {spent} vom Sitzungslimit von {limit}",
    description:
      "Session page — model spend so far beside the session's spend limit",
  },
  "agentSessions.costNote": {
    message: "Kosten {total} · Modell {model}, Compute {compute}",
    description:
      "Session page — the session's total cost so far (outcome.cost.totalUSD) with its model and sandbox compute parts",
  },
  "agentSessions.costNoteWithLimit": {
    message:
      "Kosten {total} · Modell {model} vom Sitzungslimit von {limit}, Compute {compute}",
    description:
      "Session page — total cost with its parts; the session spend limit caps the model part",
  },
  "agentSessions.resultTitle": {
    message: "Ergebnis",
    description:
      "Session page — heading over the turn's final structured result",
  },
  "agentSessions.resultDraftTitle": {
    message: "Ergebnisentwurf",
    description:
      "Session page — heading over the agent's not-yet-final structured result while the turn runs",
  },
  "agentSessions.resultDraftHint": {
    message: "Noch nicht final. Der Agent kann es noch ändern.",
    description: "Session page — note under a draft structured result",
  },
  "agentSessions.resultCopy": {
    message: "JSON kopieren",
    description: "Session page — button copying the structured result",
  },
  "agentSessions.resultCopied": {
    message: "Kopiert",
    description: "Session page — toast after copying the structured result",
  },
  "agentSessions.failureSpendCapReached": {
    message:
      "Dieser Workspace hat sein Ausgabenlimit für Agent-Modelle für den Monat erreicht. Ein Admin kann das Limit unter Einstellungen → Agents erhöhen, oder es wird am 1. (UTC) zurückgesetzt.",
    description: "Failure callout for AGENT_MODEL_SPEND_CAP_REACHED",
  },
  "agentSessions.failureBalanceDue": {
    message:
      "Eine Zahlung für diesen Workspace ist fehlgeschlagen. Bezahle die offene Rechnung unter Abrechnung und starte dann eine neue Sitzung.",
    description: "Failure callout for BILLING_BALANCE_DUE",
  },
  "agentSessions.failureSetupTitle": {
    message: "Repository-Setup fehlgeschlagen",
    description:
      "Failure callout heading when a repository setup step failed (AGENT_SESSION_SETUP_FAILED)",
  },
  "agentSessions.failureSetupHint": {
    message:
      "Ein Setup-Schritt in der Umgebung dieses Repositorys ist fehlgeschlagen, daher ist der Agent nicht gestartet und es sind keine Modellausgaben angefallen. Behebe den Schritt unter Einstellungen → Agents → Repository-Umgebungen und versuche es dann erneut.",
    description:
      "Failure callout guidance under the recorded reason (which names the step and its exit code) for AGENT_SESSION_SETUP_FAILED",
  },
  "agentSessions.failureSetupAction": {
    message: "Repository-Umgebung bearbeiten",
    description:
      "Failure callout button opening Settings → Repository environments",
  },
  "agentSessions.costTotal": {
    message: "{amount} Modellkosten",
    description: "Session header: the session's model cost (ADR091 D6)",
  },
  "agentSessions.costTotalAll": {
    message: "{amount} Gesamtkosten",
    description:
      "Session header: the session's total cost so far, model plus sandbox compute (outcome.cost.totalUSD, w9/m178)",
  },
  "agentSessions.costBreakdown": {
    message: "Modell {model} · Compute {compute}",
    description:
      "Session header cost popover: the model and sandbox compute parts of the total",
  },
  "agentSessions.costByo": {
    message: "Von deinem Anbieter abgerechnet",
    description: "Session header: a BYO-key session has no bex model cost",
  },
  "agentSessions.costTitle": {
    message: "Modellkosten pro Durchgang",
    description: "Heading of the per-turn cost popover",
  },
  "agentSessions.costTurn": {
    message: "Durchgang {turn}",
    description: "One turn's row in the cost popover",
  },
  "agentSessions.costSetup": {
    message: "Vor dem ersten Durchgang",
    description: "Requests recorded before the first turn",
  },
  "agentSessions.costTurnTokens": {
    message: "{input} Input · {output} Output · {cache} Cache-Tokens",
    description: "A turn's token counts",
  },
  "agentSessions.costPricing": {
    message: "Abgerechnet zu den Kosten des Modellanbieters + 30 %.",
    description: "Pricing statement in the cost popover",
  },
  "agentSessions.secretsButton": {
    message: "Secrets",
    description: "Composer toolbar: the Secrets button label/tooltip (ADR098)",
  },
  "agentSessions.secretsButtonCount_one": {
    message: "Secrets ({count} angehängt)",
    description:
      "Composer Secrets button accessible label when secrets are attached",
  },
  "agentSessions.secretsButtonCount_other": {
    message: "Secrets ({count} angehängt)",
    description:
      "Composer Secrets button accessible label when secrets are attached",
  },
  "agentSessions.secretsTitle": {
    message: "Secrets für diese Sitzung",
    description: "Composer Secrets popover heading",
  },
  "agentSessions.secretsDescription": {
    message:
      "Der Agent liest jedes als Umgebungsvariable. Werte werden nie wieder angezeigt und im Protokoll geschwärzt.",
    description: "Composer Secrets popover description",
  },
  "agentSessions.secretsWorkspace": {
    message: "Workspace-Secrets",
    description:
      "Composer Secrets popover: workspace agent secrets list heading",
  },
  "agentSessions.secretsWorkspaceLoadFailed": {
    message: "Die Agent-Secrets des Workspace konnten nicht geladen werden.",
    description:
      "Composer secrets panel error when the workspace secret list fails to load",
  },
  "agentSessions.secretsWorkspaceEmpty": {
    message: "Noch keine Workspace-Secrets.",
    description: "Composer Secrets popover: no workspace agent secrets",
  },
  "agentSessions.secretsManage": {
    message: "In den Einstellungen verwalten",
    description: "Composer Secrets popover: link to Settings → Agents",
  },
  "agentSessions.secretsInline": {
    message: "Nur für diese Sitzung",
    description: "Composer Secrets popover: inline secrets heading",
  },
  "agentSessions.secretsRemove": {
    message: "{name} entfernen",
    description:
      "Composer Secrets popover: remove an inline secret (accessible label)",
  },
  "agentSessions.secretsNamePlaceholder": {
    message: "NAME",
    description: "Composer Secrets popover: inline secret name placeholder",
  },
  "agentSessions.secretsNameLabel": {
    message: "Secret-Name",
    description: "Composer Secrets popover: inline secret name field label",
  },
  "agentSessions.secretsValuePlaceholder": {
    message: "Wert",
    description: "Composer Secrets popover: inline secret value placeholder",
  },
  "agentSessions.secretsValueLabel": {
    message: "Secret-Wert",
    description: "Composer Secrets popover: inline secret value field label",
  },
  "agentSessions.secretsAdd": {
    message: "Hinzufügen",
    description: "Composer Secrets popover: add an inline secret",
  },
  "agentSessions.secretsHint": {
    message: "Wird mit der Sitzung gelöscht.",
    description: "Composer Secrets popover: inline secret lifetime hint",
  },
  "agentSessions.secretsFull": {
    message: "Eine Sitzung nimmt höchstens 20 Secrets auf.",
    description: "Composer Secrets popover: session secret limit reached",
  },
  "agentSessions.secretsDuplicate": {
    message: "Diese Sitzung hat bereits ein Secret mit diesem Namen.",
    description: "Composer Secrets popover: duplicate inline secret name",
  },
  "agentSessions.secretsScopeRepo": {
    message: "Dieses Repository",
    description:
      "Composer Secrets popover: hint beside a secret scoped to the chosen repository, included",
  },
  "agentSessions.secretsScopeOtherRepo": {
    message: "Nur für {repo}",
    description:
      "Composer Secrets popover: hint beside a secret scoped to another repository, not available here",
  },
  "agentSessions.appliedSecretsTitle": {
    message: "Secrets im letzten Durchgang",
    description:
      "Session header: heading of the applied-secrets popover (names and sources, never values)",
  },
  "agentSessions.appliedSecretSource.inline": {
    message: "diese Sitzung",
    description:
      "Applied secret source: an inline secret given to this session only",
  },
  "agentSessions.appliedSecretSource.explicit": {
    message: "ausgewählt",
    description: "Applied secret source: a workspace secret the session picked",
  },
  "agentSessions.appliedSecretSource.repo": {
    message: "Repository",
    description:
      "Applied secret source: a workspace secret scoped to the session's repository",
  },
  "agentSessions.appliedSecretSource.workspace": {
    message: "Workspace",
    description:
      "Applied secret source: a workspace secret every session receives",
  },
  "agentSessions.metaSecrets_one": {
    message: "{count} Secret",
    description:
      "Detail header — number of attached secrets (singular, ADR098)",
  },
  "agentSessions.metaSecrets_other": {
    message: "{count} Secrets",
    description: "Detail header — number of attached secrets (plural, ADR098)",
  },
  "agentSessions.skillsButton": {
    message: "Skills",
    description: "Composer toolbar: the Skills button label/tooltip (ADR099)",
  },
  "agentSessions.skillsButtonCount_one": {
    message: "Skills ({count} ausgewählt)",
    description:
      "Composer Skills button accessible label when skills are chosen",
  },
  "agentSessions.skillsButtonCount_other": {
    message: "Skills ({count} ausgewählt)",
    description:
      "Composer Skills button accessible label when skills are chosen",
  },
  "agentSessions.skillsTitle": {
    message: "Skills für diesen Durchgang",
    description: "Composer Skills popover heading",
  },
  "agentSessions.skillsDescription": {
    message:
      "Der Agent befolgt jeden ausgewählten Skill. Aktivierte Skills darf er auch von sich aus anwenden.",
    description: "Composer Skills popover description",
  },
  "agentSessions.skillsLoadFailed": {
    message: "Die Skills des Workspace konnten nicht geladen werden.",
    description: "Composer Skills popover: list failed",
  },
  "agentSessions.skillsEmpty": {
    message: "Noch keine Skills in diesem Workspace.",
    description: "Composer Skills popover: none exist",
  },
  "agentSessions.skillsManage": {
    message: "Skills verwalten",
    description: "Composer Skills popover link to Settings → Agents",
  },
  "agentSessions.skillsArgumentsLabel": {
    message: "Argumente für {name}",
    description: "Composer Skills popover: argument field label",
  },
  "agentSessions.skillsArgumentsPlaceholder": {
    message: "Argumente (optional)",
    description: "Composer Skills popover: argument field placeholder",
  },
  "agentSessions.skillsArgumentsInvalid": {
    message: "Argumente bestehen aus einer Zeile, bis zu 4 KiB.",
    description: "Composer Skills popover: invalid arguments",
  },
  "agentSessions.skillsFull": {
    message: "Ein Durchgang kann höchstens 5 Skills verwenden.",
    description: "Composer Skills popover: limit reached",
  },
  "agentSessions.skillsHint": {
    message: "Argumente füllen $ARGUMENTS im Skill.",
    description: "Composer Skills popover hint",
  },
  "agentSessions.skillsRemove": {
    message: "{name} entfernen",
    description: "Accessible label of a chosen skill chip's remove button",
  },
  "agentSessions.turnSkills": {
    message: "In diesem Durchgang verwendete Skills",
    description: "Accessible label of a transcript turn's skill pills",
  },
  "agentSessions.turnOrigins": {
    message: "Was diesen Durchgang gesteuert hat",
    description:
      "Accessible label of the lines above a transcript turn that PR feedback started (w9/m344)",
  },
  "agentSessions.turnOriginReview": {
    message: "Gesteuert durch ein Review von @{author}",
    description:
      "A turn started by a pull-request review; {author} is the reviewer's GitHub login. Links to the review",
  },
  "agentSessions.turnOriginReviewAnonymous": {
    message: "Gesteuert durch ein PR-Review",
    description:
      "A turn started by a pull-request review whose author is unknown. Links to the review",
  },
  "agentSessions.turnOriginComment": {
    message: "Gesteuert durch einen PR-Kommentar von @{author}",
    description:
      "A turn started by a pull-request comment; {author} is the commenter's GitHub login. Links to the comment",
  },
  "agentSessions.turnOriginCommentAnonymous": {
    message: "Gesteuert durch einen PR-Kommentar",
    description:
      "A turn started by a pull-request comment whose author is unknown. Links to the comment",
  },
  "agentSessions.turnOriginCiFailure": {
    message: "Gesteuert durch einen fehlgeschlagenen CI-Lauf",
    description:
      "A turn started by a failed CI run on the pull request. Links to the failed checks",
  },
  "agentSessions.turnOriginCiFailureOnBase": {
    message:
      "Gesteuert durch einen fehlgeschlagenen CI-Lauf; einige Checks schlagen auch auf {base} fehl, nicht wegen dieser Änderung",
    description:
      "A turn started by a failed CI run on the pull request where at least one failing check also fails on the base branch ({base}, a branch name), so the pull request did not cause it. Links to the failed checks",
  },
  "agentSessions.turnOriginsStart": {
    message: "Was diese Sitzung gestartet hat",
    description:
      "Accessible label of the line above a session's first turn when a /bex comment on a pull request started it (w9/m348)",
  },
  "agentSessions.turnOriginStartComment": {
    message: "Gestartet durch den Kommentar von @{author}",
    description:
      "The first turn of a session a /bex pull-request comment started; {author} is the commenter's GitHub login. Links to the comment",
  },
  "agentSessions.turnOriginStartCommentAnonymous": {
    message: "Gestartet durch einen PR-Kommentar",
    description:
      "The first turn of a session a /bex pull-request comment started, when the commenter is unknown. Links to the comment",
  },
  "agentSessions.lineage.childSessions": {
    message: "Untergeordnete Sitzungen",
    description:
      "Heading of the block listing sessions the agent spawned, in the parent session's transcript",
  },
  "agentSessions.lineage.childCardTitle": {
    message: "Hat eine Sitzung erstellt",
    description:
      "Small label above a child session's title on its card in the parent's transcript",
  },
  "agentSessions.lineage.agentsPanel": {
    message: "Agents",
    description:
      "Heading of the session page's right-hand panel listing the sessions below this one",
  },
  "agentSessions.lineage.viewSession": {
    message: "Sitzung ansehen",
    description: "Link on an Agents panel row that opens that child session",
  },
  "agentSessions.lineage.parentSession": {
    message: "Übergeordnete Sitzung",
    description:
      "Link in the Agents panel of a child session that opens its parent",
  },
  "agentSessions.lineage.collapsePanel": {
    message: "Agents-Bereich einklappen",
    description:
      "Accessible label of the button that collapses the Agents panel",
  },
  "agentSessions.lineage.expandPanel": {
    message: "Agents-Bereich ausklappen",
    description: "Accessible label of the button that expands the Agents panel",
  },
  "agentSessions.lineage.collapseChildren": {
    message: "Untergeordnete Sitzungen ausblenden",
    description:
      "Accessible label of a sidebar row's button that collapses its nested child sessions",
  },
  "agentSessions.lineage.expandChildren": {
    message: "Untergeordnete Sitzungen anzeigen",
    description:
      "Accessible label of a sidebar row's button that expands its nested child sessions",
  },
  "agentSessions.lineage.status.working": {
    message: "Arbeitet",
    description:
      "Status group and badge of a child session that is still running",
  },
  "agentSessions.lineage.status.done": {
    message: "Fertig",
    description:
      "Status group and badge of a child session that finished successfully",
  },
  "agentSessions.lineage.status.failed": {
    message: "Fehlgeschlagen",
    description: "Status group and badge of a child session that failed",
  },
  "agentSessions.lineage.status.canceled": {
    message: "Abgebrochen",
    description: "Status group and badge of a child session that was canceled",
  },
  "agentSessions.sidebarMine": {
    message: "Meine",
    description:
      "Sessions sidebar — show only the sessions you started (Devin's default)",
  },
  "agentSessions.sidebarEveryone": {
    message: "Alle",
    description: "Sessions sidebar — show every member's sessions",
  },
  "agentSessions.sidebarOwnerFilter": {
    message: "Wessen Sitzungen",
    description:
      "Sessions sidebar — accessible name of the Mine/Everyone toggle",
  },
  "agentSessions.sidebarEmptyMine": {
    message:
      "Keine von dir gestarteten Sitzungen. Ältere Sitzungen findest du unter Alle.",
    description:
      "Sessions sidebar — empty state under Mine. Sessions from before 2026-11-08 record no starter, so they appear only under Everyone",
  },
  "agentSessions.startedByPerson": {
    message: "Gestartet von {name}",
    description: "Session header and list rows — who started the session",
  },
  "agentSessions.startedForPerson": {
    message: "für {name}",
    description:
      "Session header — after 'Started by automation X': the member the automation runs as",
  },
  "agentSessions.editTags": {
    message: "Tags bearbeiten",
    description: "Session header menu — open the tag editor",
  },
  "agentSessions.editTagsTitle": {
    message: "Tags bearbeiten",
    description: "Tag editor dialog title",
  },
  "agentSessions.editTagsDescription": {
    message:
      "Tags ersetzen die aktuellen Tags der Sitzung. Trenne sie mit Kommas oder Leerzeichen; leer lassen, um alle zu entfernen.",
    description: "Tag editor dialog description",
  },
  "agentSessions.editTagsHint": {
    message:
      "Bis zu 20 Tags: Kleinbuchstaben, Ziffern, Doppelpunkt, Unterstrich oder Bindestrich.",
    description: "Tag editor dialog — the tag rules",
  },
  "agentSessions.editTagsSave": {
    message: "Tags speichern",
    description: "Tag editor dialog — save button",
  },
  "agentSessions.editTagsSaved": {
    message: "Tags gespeichert",
    description: "Toast after the tags were replaced",
  },
};

export default deAgentSessions;
