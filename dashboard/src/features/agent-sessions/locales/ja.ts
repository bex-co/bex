import type { TranslationEntry } from "@/i18n";

const jaAgentSessions: Record<string, TranslationEntry> = {
  "agentSessions.pageTitle": {
    message: "エージェントセッション",
    description: "Agent sessions list page heading and document title",
  },
  "agentSessions.listTitle": {
    message: "最近",
    description: "Agent sessions recents heading — matches the rail",
  },
  "agentSessions.colTask": {
    message: "タスク",
    description: "Agent sessions table column header — the session's prompt",
  },
  "agentSessions.colRepo": {
    message: "リポジトリ",
    description: "Agent sessions table column header — repo/branch",
  },
  "agentSessions.colAgent": {
    message: "エージェント",
    description: "Agent sessions table column header — the driver agent",
  },
  "agentSessions.colPhase": {
    message: "フェーズ",
    description: "Agent sessions table column header — lifecycle phase chip",
  },
  "agentSessions.colPr": {
    message: "PR",
    description: "Agent sessions table column header — draft pull request",
  },
  "agentSessions.colCreated": {
    message: "作成",
    description: "Agent sessions table column header — createdAt relative age",
  },
  "agentSessions.prBadge": {
    message: "#{number}",
    description: "Draft PR badge label — the pull-request number",
  },
  "agentSessions.prState.open": {
    message: "オープン",
    description:
      "PR state badge beside a session's PR link — the pull request is open (a draft counts as open)",
  },
  "agentSessions.prState.merged": {
    message: "マージ済み",
    description:
      "PR state badge beside a session's PR link — the pull request was merged",
  },
  "agentSessions.prState.closed": {
    message: "クローズ",
    description:
      "PR state badge beside a session's PR link — the pull request was closed without merging",
  },
  "agentSessions.prStateFilter": {
    message: "プルリクエストの状態で絞り込み",
    description:
      "Session list — accessible name of the Any/Open/Merged/Closed PR-state filter",
  },
  "agentSessions.prStateFilterAny": {
    message: "すべて",
    description:
      "Session list PR-state filter — the option that shows sessions whatever their PR state, including sessions with no PR",
  },
  "agentSessions.errorTitle": {
    message: "セッションを読み込めませんでした。",
    description: "Agent sessions list error — one sentence, Retry beside it",
  },
  "agentSessions.retry": {
    message: "再試行",
    description: "Retries a failed agent-sessions list request",
  },
  "agentSessions.emptyBody": {
    message: "開始したセッションがここに表示されます。",
    description: "Default empty — one quiet line under the composer",
  },
  "agentSessions.emptyArchivedTitle": {
    message: "アーカイブ済みのセッションはありません",
    description: "Archived-only list empty-state heading",
  },
  "agentSessions.emptyArchivedBody": {
    message: "アーカイブしたセッションがここに表示されます。",
    description: "Archived-only list empty-state body",
  },
  "agentSessions.emptyFilteredTitle": {
    message: "一致するセッションはありません",
    description: "Phase-filtered list empty-state heading",
  },
  "agentSessions.emptyFilteredBody": {
    message: "別のフェーズを試すか、フィルターをクリアしてください。",
    description: "Phase-filtered list empty-state body",
  },
  "agentSessions.clearFilters": {
    message: "フィルターをクリア",
    description: "Clears agent-session membership and phase filters",
  },
  "agentSessions.phase.creating": {
    message: "作成中",
    description: "Agent session phase chip — sandbox is being created",
  },
  "agentSessions.phase.running": {
    message: "実行中",
    description: "Agent session phase chip — a turn is in progress",
  },
  "agentSessions.phase.resuming": {
    message: "再開中",
    description: "Agent session phase chip — session is resuming",
  },
  "agentSessions.phase.redispatching": {
    message: "再ディスパッチ中",
    description: "Agent session phase chip — session is being redispatched",
  },
  "agentSessions.phase.completed": {
    message: "完了",
    description: "Agent session phase chip — session finished successfully",
  },
  "agentSessions.phase.failed": {
    message: "失敗",
    description: "Agent session phase chip — session failed",
  },
  "agentSessions.phase.canceling": {
    message: "キャンセル中",
    description: "Agent session phase chip — cancellation in progress",
  },
  "agentSessions.phase.canceled": {
    message: "キャンセル済み",
    description: "Agent session phase chip — session was canceled",
  },
  "agentSessions.taskLabel": {
    message: "タスク",
    description: "Composer — the prominent task prompt textarea label",
  },
  "agentSessions.taskPlaceholder": {
    message: "コーディングタスクを入力…",
    description: "Composer — task editor placeholder",
  },
  "agentSessions.mentionButton": {
    message: "リポジトリまたはセッションを追加",
    description:
      "Composer toolbar — opens the repository/session mention picker",
  },
  "agentSessions.repoChip": {
    message: "リポジトリ {repo}",
    description: "Composer toolbar — accessible name of the selected-repo chip",
  },
  "agentSessions.keyboardHint": {
    message: "Enter で開始 · Shift+Enter で改行",
    description: "Muted hint under the composer",
  },
  "agentSessions.connectGitHubTitle": {
    message: "開始するには GitHub を接続してください",
    description: "Composer callout when the workspace has no App repos",
  },
  "agentSessions.connectGitHubBody": {
    message:
      "クラウドエージェントには、接続済みの GitHub リポジトリが必要です。このワークスペースに GitHub App を接続してから、ここに戻ってセッションを開始してください。",
    description: "Composer callout body when there are no installation repos",
  },
  "agentSessions.connectGitHubSettings": {
    message: "GitHub の設定",
    description:
      "Secondary link from the composer's GitHub callout to Account Settings → Integrations (/settings#integrations), where the GitHub card lives",
  },
  "agentSessions.exampleFixTests": {
    message: "失敗しているテストを修正して、ドラフトの PR を作成する",
    description: "First-run example prompt chip",
  },
  "agentSessions.exampleAddReadme": {
    message: "プロジェクトの実行方法を説明する README を追加する",
    description: "First-run example prompt chip",
  },
  "agentSessions.chipRemove": {
    message: "{name} を削除",
    description: "Accessible label of a mention chip's remove button",
  },
  "agentSessions.mentionCategoryRepos": {
    message: "リポジトリ",
    description: "Mention picker — the repositories category row",
  },
  "agentSessions.mentionCategoryReposDesc": {
    message: "接続済みの GitHub リポジトリにセッションの対象を限定します",
    description: "Mention picker — repositories category one-line description",
  },
  "agentSessions.mentionCategorySessions": {
    message: "セッション",
    description: "Mention picker — the prior-sessions category row",
  },
  "agentSessions.mentionCategorySessionsDesc": {
    message: "以前のエージェントセッションをコンテキストとして参照します",
    description: "Mention picker — sessions category one-line description",
  },
  "agentSessions.mentionNoResults": {
    message: "一致するものはありません",
    description:
      "Mention picker — empty state when the fuzzy filter matches nothing",
  },
  "agentSessions.mentionReposEmpty": {
    message:
      "リポジトリはまだありません。GitHub アカウントを接続するか、bex GitHub App にリポジトリへのアクセスを許可してください。",
    description:
      "Mention picker — empty state when no installation repos exist",
  },
  "agentSessions.mentionSessionsEmpty": {
    message: "セッションはまだありません",
    description:
      "Mention picker — empty state when the workspace has no sessions",
  },
  "agentSessions.mentionConnected": {
    message: "GitHub App 経由で接続済み",
    description: "Mention picker readiness footer — the repo is app-connected",
  },
  "agentSessions.mentionDefaultBranch": {
    message: "デフォルトブランチ: {branch}",
    description: "Mention picker readiness footer — the repo's default branch",
  },
  "agentSessions.agentLabel": {
    message: "エージェント",
    description: "Composer — agent select label",
  },
  "agentSessions.modelLabel": {
    message: "モデル",
    description: "Composer — model select label (w10/m516)",
  },
  "agentSessions.modelDefaultOption": {
    message: "{model}（デフォルト）",
    description: "Model list — the model a session runs when it names none",
  },
  "agentSessions.modelFixed": {
    message: "このエージェントは常に {model} を使用します",
    description: "Tooltip of the model of an agent with one fixed model",
  },
  "agentSessions.metaModel": {
    message: "モデル: {model}",
    description: "Session header — tooltip of the model the session runs",
  },
  "agentSessions.sandboxLabel": {
    message: "サンドボックス",
    description: "Composer — sandbox template select label (w10/m519)",
  },
  "agentSessions.sandboxSize": {
    message: "CPU {cpu} コア、メモリ {memory} GB",
    description:
      "Sandbox template size: CPU cores and memory in GB, numbers already formatted",
  },
  "agentSessions.sandboxOption": {
    message: "{name} · {size}",
    description: "Sandbox template choice: its name, then its size",
  },
  "agentSessions.sandboxDefaultOption": {
    message: "{template}（デフォルト）",
    description:
      "Sandbox list — the template a session runs on when it names none (the workspace default)",
  },
  "agentSessions.sandboxWorkspaceDefault": {
    message: "ワークスペースのデフォルト（{template}）",
    description:
      "Automation sandbox choice — follow the workspace default at each run; {template} is the default now",
  },
  "agentSessions.metaSandbox": {
    message: "サンドボックス: {template}",
    description:
      "Tooltip of the sandbox template a session runs on, with its size",
  },
  "agentSessions.githubCliInherit": {
    message: "継承",
    description:
      "GitHub CLI choice — inherit the level above, while what that resolves to is still loading (w10/m561)",
  },
  "agentSessions.githubCliInheritOn": {
    message: "継承（オン）",
    description:
      "GitHub CLI choice — inherit the level above, which has gh on (w10/m561)",
  },
  "agentSessions.githubCliInheritOff": {
    message: "継承（オフ）",
    description:
      "GitHub CLI choice — inherit the level above, which has gh off (w10/m561)",
  },
  "agentSessions.githubCliOn": {
    message: "オン",
    description:
      "GitHub CLI choice — gh on for this session or automation (w10/m561)",
  },
  "agentSessions.githubCliOff": {
    message: "オフ",
    description:
      "GitHub CLI choice — gh off for this session or automation (w10/m561)",
  },
  "agentSessions.githubCliOnMandatory": {
    message:
      "このワークスペースでは GitHub CLI のオフが必須のため、選択できません。",
    description:
      "Why the GitHub CLI On choice is disabled: the workspace's off is mandatory (w10/m561)",
  },
  "agentSessions.githubCliOnAdminOnly": {
    message: "gh をオンにできるのはワークスペースの管理者のみです。",
    description:
      "Why the GitHub CLI On choice is disabled: the member is not a workspace admin (w10/m561)",
  },
  "agentSessions.githubCliTitle": {
    message: "このセッションの GitHub CLI: {choice}",
    description:
      "Tooltip of the composer's GitHub CLI control; {choice} is the chosen binding, e.g. Inherit (on) (w10/m561)",
  },
  "agentSessions.errors.AGENT_SESSION_GITHUB_CLI_LOCKED": {
    message:
      "このワークスペースでは GitHub CLI のオフが必須のため、セッションで gh をオンにすることはできません。「オフ」または「継承」を選んでください。",
    description:
      "Mapped message for AGENT_SESSION_GITHUB_CLI_LOCKED with reason mandatory (w10/m561)",
  },
  "agentSessions.errors.AGENT_SESSION_GITHUB_CLI_LOCKED_adminRequired": {
    message:
      "セッションで gh をオンにできるのはワークスペースの管理者のみです。「オフ」または「継承」を選んでください。",
    description:
      "Mapped message for AGENT_SESSION_GITHUB_CLI_LOCKED with reason admin_required (w10/m561)",
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
    message: "セッションを開始",
    description: "Composer — submit button label",
  },
  "agentSessions.submitting": {
    message: "開始中…",
    description: "Composer — submit button label while the create is in flight",
  },
  "agentSessions.unavailableTitle": {
    message: "エージェントセッションが設定されていません",
    description: "Composer — house callout title when the feature is 503",
  },
  "agentSessions.unavailableBody": {
    message:
      "このプラットフォームでは、クラウドエージェントセッションが有効になっていません。運用担当者にエージェントセッションゲートウェイの設定を依頼してください。",
    description: "Composer — house callout body when the feature is 503",
  },
  "agentSessions.dependencyUnavailableTitle": {
    message: "エージェントセッションは一時的に利用できません",
    description:
      "Composer — retryable dependency-outage title, distinct from configuration",
  },
  "agentSessions.dependencyUnavailableBody": {
    message:
      "プラットフォームの依存先で問題が発生しています。設定は変更されていません。しばらく待ってから、もう一度お試しください。",
    description:
      "Composer — retryable dependency-outage body that does not send the user to an operator",
  },
  "agentSessions.modelKeyMissingTitle": {
    message: "先にモデルプロバイダーのキーを追加してください",
    description:
      "Composer pre-flight title when the workspace has no model key",
  },
  "agentSessions.modelKeyMissingBody": {
    message:
      "このワークスペースにはまだモデルへのアクセスがありません。ワークスペースの管理者が、設定 → エージェントでワークスペース独自のモデルプロバイダーのキーを追加してから、ここに戻ってください。",
    description:
      "Composer pre-flight body when the workspace has neither its own model key nor the bex-provided model (ADR091 D7)",
  },
  "agentSessions.modelKeyMissingAction": {
    message: "設定 → エージェントを開く",
    description:
      "Composer pre-flight link to the Settings → Agents model key field (/workspace/settings#agents)",
  },
  "agentSessions.createErrorTitle": {
    message: "セッションを開始できませんでした",
    description: "Composer — error alert title when the create fails",
  },
  "agentSessions.detailTitle": {
    message: "セッション",
    description: "Agent-session detail page document title",
  },
  "agentSessions.untitled": {
    message: "無題のセッション",
    description:
      "Last-resort session name when a session carries neither a repository nor a task prompt",
  },
  "agentSessions.backToList": {
    message: "すべてのセッション",
    description: "Detail page — back link to the /agents list",
  },
  "agentSessions.detailErrorTitle": {
    message: "このセッションを読み込めませんでした",
    description: "Detail page — heading when the session read fails",
  },
  "agentSessions.conversationTitle": {
    message: "会話",
    description: "Detail page — the live conversation column card title",
  },
  "agentSessions.failureTitle": {
    message: "セッションが失敗しました",
    description: "Detail page — heading over a failed session's reason",
  },
  "agentSessions.deliveryNoteTitle": {
    message: "変更が完全には反映されていません",
    description:
      "Detail page — heading over a completed session's note that the agent left work uncommitted, unpushed or without a pull request",
  },
  "agentSessions.deliveryNoteHint": {
    message:
      "フォローアップのメッセージを送り、コミット、プッシュ、プルリクエストの更新を最後まで行うようエージェントに依頼してください。",
    description:
      "Detail page — the next step shown under an undelivered-work note",
  },
  "agentSessions.failureReasonFallback": {
    message: "セッションを開始できませんでした。",
    description:
      "Detail page — generic failure text when the session carries no specific reason",
  },
  "agentSessions.failureRetry": {
    message: "再試行",
    description:
      "Detail page — button to re-run a failed session's original task",
  },
  "agentSessions.failureRetrying": {
    message: "再試行中…",
    description:
      "Detail page — retry button label while the redispatch is in flight",
  },
  "agentSessions.capacityFailureTitle": {
    message: "サンドボックスの容量が不足しています",
    description:
      "Detail page — heading when a session failed on the plan's sandbox limit",
  },
  "agentSessions.capacityFailureBody": {
    message:
      "このワークスペースは、プランの同時サンドボックス数の上限に達しました。同時に多く実行するにはプランをアップグレードするか、アイドル状態のセッションを停止して再試行してください。",
    description:
      "Detail page — explains a plan-limit sandbox failure and the two remedies",
  },
  "agentSessions.upgradePlan": {
    message: "プランをアップグレード",
    description:
      "Detail page — CTA on a capacity failure; opens the change-plan dialog",
  },
  "agentSessions.failureCause.pool_full": {
    message: "空きのあるサンドボックスマシンがありませんでした",
    description:
      "Failed-start cause line (session callout, automation runs) — no sandbox machine had room for the session's machine for the whole wait (w10/m497)",
  },
  "agentSessions.failureCause.image_pull": {
    message: "サンドボックスのイメージを取得できませんでした",
    description:
      "Failed-start cause line — the sandbox's container image did not pull (w10/m497)",
  },
  "agentSessions.failureCause.runtime": {
    message: "サンドボックスのランタイムが起動できませんでした",
    description:
      "Failed-start cause line — the sandbox runtime did not start the session's machine (w10/m497)",
  },
  "agentSessions.failureCause.quota": {
    message: "ワークスペースのサンドボックスのクォータで拒否されました",
    description:
      "Failed-start cause line — the workspace's sandbox quota refused the session's machine (w10/m497)",
  },
  "agentSessions.failureCause.unknown": {
    message: "プラットフォームの問題",
    description:
      "Failed-start cause line — the sandbox did not start for a cause on bex's side that bex-api could not name (w10/m497)",
  },
  "agentSessions.provisioning": {
    message: "サンドボックスを起動しています…",
    description:
      "Detail page — shown while a new/steered session provisions its sandbox before the conversation stream is available",
  },
  "agentSessions.metaDuration": {
    message: "所要時間 {duration}",
    description: "Detail header — elapsed session wall-clock",
  },
  "agentSessions.metaTurns_other": {
    message: "{count} ターン",
    description: "Detail header — number of prompt turns taken (plural)",
  },
  "agentSessions.cancel": {
    message: "キャンセル",
    description: "Detail header — cancel-session button label",
  },
  "agentSessions.cancelDisabledCanceling": {
    message: "すでにキャンセル中です。",
    description: "Detail header — tooltip on the disabled cancel button",
  },
  "agentSessions.canceling": {
    message: "キャンセル中…",
    description: "Detail header — confirm button label while the cancel runs",
  },
  "agentSessions.cancelSuccess": {
    message: "セッションをキャンセルしています…",
    description: "Detail header — toast after the cancel is accepted",
  },
  "agentSessions.cancelConfirmTitle": {
    message: "このセッションをキャンセルしますか？",
    description: "Cancel confirm dialog title",
  },
  "agentSessions.cancelConfirmBody": {
    message:
      "エージェントが停止します。ブランチにすでにプッシュされたコミットとドラフトの PR は保持されます。",
    description: "Cancel confirm dialog body — states pushed work is preserved",
  },
  "agentSessions.cancelConfirmDismiss": {
    message: "実行を続ける",
    description: "Cancel confirm dialog — dismiss button",
  },
  "agentSessions.cancelConfirmProceed": {
    message: "セッションをキャンセル",
    description: "Cancel confirm dialog — proceed button",
  },
  "agentSessions.stopTurn": {
    message: "ターンを停止",
    description:
      "Detail header — Stop turn button label: ends only the running turn, unlike Cancel which ends the session",
  },
  "agentSessions.stoppingTurn": {
    message: "停止中…",
    description: "Detail header — Stop turn button label while the stop runs",
  },
  "agentSessions.stopTurnHint": {
    message:
      "実行中のターンだけを終了します。セッションは開いたままで、キュー内のメッセージの順序も保たれます。ショートカット: Ctrl/Cmd+Shift+Backspace。",
    description: "Detail header — tooltip on the Stop turn button",
  },
  "agentSessions.stopTurnStarting": {
    message: "ターンはまだ開始中です。実行が始まってから停止できます。",
    description:
      "Detail header — tooltip on the disabled Stop turn button while the sandbox starts",
  },
  "agentSessions.stopTurnSuccess": {
    message: "ターンを停止しています。数秒以内に終了します。",
    description: "Detail header — toast after Stop turn is accepted",
  },
  "agentSessions.steerTitle": {
    message: "このセッションを誘導",
    description: "Steering composer card title",
  },
  "agentSessions.steerErrorTitle": {
    message: "送信できませんでした",
    description: "Steering composer — inline error alert title",
  },
  "agentSessions.steerPlaceholderIdle": {
    message:
      "フォローアップの指示を送ると、同じブランチで新しいターンが始まります。",
    description: "Steering composer — textarea placeholder for an idle session",
  },
  "agentSessions.steerPlaceholderQueue": {
    message:
      "メッセージをキューに追加します。現在のターンが終わると実行されます。",
    description:
      "Steering composer — textarea placeholder while a turn runs or messages wait (ADR047 D12)",
  },
  "agentSessions.steerHintIdle": {
    message:
      "このセッションはアイドル状態です。メッセージを送ると、同じブランチの新しいサンドボックスで会話が続きます。",
    description: "Steering composer — helper text for the redispatch route",
  },
  "agentSessions.steerHintQueue": {
    message:
      "エージェントが作業中です。メッセージはキューに追加され、このターンが終わると独立したターンとして実行されます。",
    description:
      "Steering composer — helper text while a turn runs or messages wait (ADR047 D12)",
  },
  "agentSessions.steerDisabledCanceling": {
    message:
      "このセッションはキャンセル中です。すでにプッシュされた作業は保持されます。",
    description: "Steering composer — disabled reason while canceling",
  },
  "agentSessions.steerDisabledCanceled": {
    message: "このセッションはキャンセルされました。",
    description: "Steering composer — disabled reason once canceled",
  },
  "agentSessions.steerDisabledStream": {
    message:
      "会話ストリームを利用できないため、ライブでの誘導は一時停止中です。",
    description:
      "Steering composer — disabled reason when the m43 stream is down",
  },
  "agentSessions.steerSubmit": {
    message: "送信",
    description: "Steering composer — submit button label",
  },
  "agentSessions.steerSending": {
    message: "送信中…",
    description: "Steering composer — submit button label while sending",
  },
  "agentSessions.steerSuccess": {
    message: "送信しました。新しいターンを再ディスパッチしています。",
    description: "Steering composer — toast after a redispatch is accepted",
  },
  "agentSessions.steerQueued": {
    message: "キューに追加しました。現在のターンが終わると実行されます。",
    description:
      "Steering composer — toast after a message was queued behind a running turn (ADR047 D12)",
  },
  "agentSessions.queueRegion": {
    message: "キュー内のメッセージ",
    description: "Queued messages list — accessible name of the region",
  },
  "agentSessions.queueTitle_other": {
    message: "キュー内のメッセージ {count} 件",
    description: "Queued messages list — heading with the count (plural)",
  },
  "agentSessions.queueHint": {
    message: "現在のターンの後、この順序で 1 件ずつ実行されます。",
    description:
      "Queued messages list — one-line explanation next to the heading",
  },
  "agentSessions.queuePosition": {
    message: "キューの {position} 番目",
    description: "Queued messages list — tooltip on a row's place number",
  },
  "agentSessions.queueAttachments_other": {
    message: "添付ファイル {count} 件",
    description: "Queued message row — attachments that ride with it (plural)",
  },
  "agentSessions.queueBlocked": {
    message: "まだ開始できません: {reason}。自動的に再試行されます。",
    description:
      "Queued message row — why the dispatcher could not start it (a spend ceiling or the live-sandbox limit)",
  },
  "agentSessions.queueModel": {
    message: "セッションを {model} に切り替えます",
    description:
      "Queued message — the model its turn switches the session to (w10/m516)",
  },
  "agentSessions.queueSendNow": {
    message: "今すぐ送信",
    description:
      "Queued message row — stops the running turn and runs this message next",
  },
  "agentSessions.queueMoveUp": {
    message: "上へ移動",
    description: "Queued message row — move one place earlier",
  },
  "agentSessions.queueMoveDown": {
    message: "下へ移動",
    description: "Queued message row — move one place later",
  },
  "agentSessions.queueEdit": {
    message: "編集",
    description: "Queued message row — edit the message in place",
  },
  "agentSessions.queueRemove": {
    message: "削除",
    description: "Queued message row — remove it so it never runs",
  },
  "agentSessions.queueEditLabel": {
    message: "キュー内のメッセージを編集",
    description: "Queued message row — accessible name of the edit box",
  },
  "agentSessions.queueEditSave": {
    message: "保存",
    description: "Queued message row — save the edit",
  },
  "agentSessions.queueEditSaveSendNow": {
    message: "保存して今すぐ送信",
    description:
      "Queued message row — save the edit, stop the running turn and run this message next",
  },
  "agentSessions.queueEditDiscard": {
    message: "破棄",
    description: "Queued message row — leave the edit without saving",
  },
  "agentSessions.queueEditHint": {
    message:
      "Enter で保存、Shift+Enter で改行、Ctrl/Cmd+Enter で保存して今すぐ送信、Esc で破棄します。",
    description: "Queued message row — keyboard hints while editing",
  },
  "agentSessions.queueSendNowStopping": {
    message: "現在のターンを停止しています。このメッセージが次に実行されます。",
    description: "Queued message row — toast after Send now stopped a turn",
  },
  "agentSessions.queueSendNowNext": {
    message: "このメッセージが次に実行されます。",
    description:
      "Queued message row — toast after Send now when no turn was running",
  },
  "agentSessions.errors.AGENT_SESSION_INPUT_INVALID": {
    message:
      "入力が無効です。リポジトリ、ブランチ、タスクを確認して、もう一度お試しください。",
    description: "Mapped message for AGENT_SESSION_INPUT_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_ID_INVALID": {
    message: "そのセッション ID は無効です。",
    description: "Mapped message for AGENT_SESSION_ID_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_FOUND": {
    message: "そのセッションはすでに存在しません。",
    description: "Mapped message for AGENT_SESSION_NOT_FOUND",
  },
  "agentSessions.errors.AGENT_SESSION_CONFLICT": {
    message:
      "このセッションは別の操作を処理中です。しばらくしてからもう一度お試しください。",
    description: "Mapped message for AGENT_SESSION_CONFLICT",
  },
  "agentSessions.errors.AGENT_SESSION_BRANCH_IN_USE": {
    message:
      "別のセッションがすでにこのブランチで作業しています。そのセッションにメッセージを送るか、別の bex-agent/ ブランチを使用してください。",
    description: "Mapped message for AGENT_SESSION_BRANCH_IN_USE (w5/m382)",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_STEERABLE": {
    message: "このセッションは現在のフェーズ（{phase}）では誘導できません。",
    description: "Mapped message for AGENT_SESSION_NOT_STEERABLE",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_RESUMABLE": {
    message: "このセッションは現在のフェーズ（{phase}）では再開できません。",
    description: "Mapped message for AGENT_SESSION_NOT_RESUMABLE",
  },
  "agentSessions.errors.AGENT_SESSION_NOT_ATTACHABLE": {
    message:
      "このセッションは現在のフェーズ（{phase}）ではアタッチできません。",
    description: "Mapped message for AGENT_SESSION_NOT_ATTACHABLE",
  },
  "agentSessions.errors.AGENT_SESSION_TURN_IN_FLIGHT": {
    message:
      "すでにターンが実行中です。別のメッセージを送る前に、終了するまでお待ちください。",
    description: "Mapped message for AGENT_SESSION_TURN_IN_FLIGHT",
  },
  "agentSessions.errors.AGENT_SESSION_QUEUE_FULL": {
    message:
      "このセッションにはすでに {max} 件のメッセージがキューに入っています。1 件が実行されるのを待つか、1 件削除してから、もう一度送信してください。",
    description: "Mapped message for AGENT_SESSION_QUEUE_FULL (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_QUEUE_INLINE_SECRETS": {
    message:
      "インラインのシークレットを含むメッセージは、その値が保存されないため、キューで待機できません。セッションがアイドル状態のときに送信するか、ワークスペースのシークレットを使用してください。",
    description:
      "Mapped message for AGENT_SESSION_QUEUE_INLINE_SECRETS (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_QUEUED_MESSAGE_DISPATCHED": {
    message: "そのメッセージはすでに実行が始まっているため、変更できません。",
    description:
      "Mapped message for AGENT_QUEUED_MESSAGE_DISPATCHED (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_QUEUED_MESSAGE_NOT_FOUND": {
    message: "そのメッセージはすでにキューにありません。",
    description:
      "Mapped message for AGENT_QUEUED_MESSAGE_NOT_FOUND (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_QUEUED_MESSAGE_ATTACHMENTS_OWNED": {
    message:
      "このメッセージには送信者の添付ファイルが含まれているため、編集できるのは送信者のみです。移動と削除は引き続き可能です。",
    description:
      "Mapped message for AGENT_QUEUED_MESSAGE_ATTACHMENTS_OWNED (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_NO_TURN_RUNNING": {
    message: "実行中のターンがないため、停止するものがありません。",
    description:
      "Mapped message for AGENT_SESSION_NO_TURN_RUNNING (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_TURN_STARTING": {
    message:
      "ターンはまだ開始中のため、まだ停止できません。数秒後にもう一度お試しください。",
    description: "Mapped message for AGENT_SESSION_TURN_STARTING (ADR047 D12)",
  },
  "agentSessions.errors.AGENT_SESSION_MODEL_ENDPOINT_INVALID": {
    message: "モデルのエンドポイントは有効な HTTPS URL にしてください。",
    description: "Mapped message for AGENT_SESSION_MODEL_ENDPOINT_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_MODEL_NOT_OFFERED": {
    message:
      "このワークスペースのプラットフォームモデルでは {model} を利用できません。モデル一覧から選んでください。",
    description:
      "Mapped message for AGENT_SESSION_MODEL_NOT_OFFERED (w10/m516)",
  },
  "agentSessions.errors.AGENT_SESSION_MODEL_PINNED": {
    message: "このエージェントは {pinnedModel} のみを使用します。",
    description: "Mapped message for AGENT_SESSION_MODEL_PINNED",
  },
  "agentSessions.errors.AGENT_SESSION_EGRESS_ALLOWLIST_INVALID": {
    message: "下り通信の許可リストのエントリ「{entry}」は無効です: {reason}",
    description: "Mapped message for AGENT_SESSION_EGRESS_ALLOWLIST_INVALID",
  },
  "agentSessions.errors.AGENT_SESSION_EGRESS_ALLOWLIST_IMMUTABLE": {
    message: "下り通信の許可リストは、セッションの開始後に変更できません。",
    description: "Mapped message for AGENT_SESSION_EGRESS_ALLOWLIST_IMMUTABLE",
  },
  "agentSessions.errors.AGENT_SESSION_EGRESS_PHASE_INVALID": {
    message:
      "このセッションの現在のフェーズでは、下り通信の許可リストを設定できません。",
    description: "Mapped message for AGENT_SESSION_EGRESS_PHASE_INVALID",
  },
  "agentSessions.conversationLoading": {
    message: "会話を読み込み中…",
    description: "Placeholder while the client-only conversation column loads",
  },
  "agentSessions.conversationConnecting": {
    message: "セッションストリームに接続中…",
    description: "Shown while the conversation stream replay is in flight",
  },
  "agentSessions.conversationNotStarted": {
    message: "サンドボックスの準備ができると、会話が表示されます。",
    description:
      "Healthy pre-stream state while a newly created session is provisioning",
  },
  "agentSessions.conversationEmpty": {
    message: "会話はまだありません。",
    description: "Shown when a session has produced no transcript parts",
  },
  "agentSessions.turnInterrupted": {
    message: "ターン {turn} は完了前に中断されました。",
    description:
      "Transcript line at the end of a turn a person stopped with Stop turn (ADR047 D12); not a failure",
  },
  "agentSessions.turnRetentionSaving": {
    message: "次のメッセージで続きから再開できるよう、変更を保存しています。",
    description:
      "Transcript line under a stopped turn while its unsaved work is being saved so the next message can continue from it (ADR047 D12, w22/m413)",
  },
  "agentSessions.turnRetentionKept": {
    message: "変更は {time} まで保持されます。",
    description:
      "Transcript line under a stopped turn: its unpushed changes are kept until {time}, a date and time in the viewer's own zone (ADR047 D12, w22/m413)",
  },
  "agentSessions.turnRetentionKeptNoTime": {
    message: "変更は次のメッセージのために保持されています。",
    description:
      "Same as the kept line before the browser knows the viewer's time zone, so it names no time",
  },
  "agentSessions.turnRetentionContinued": {
    message: "次のメッセージは、これらの変更から再開されました。",
    description:
      "Transcript line under a stopped turn: the next message started from its kept changes",
  },
  "agentSessions.turnRetentionLostExpired": {
    message:
      "保持されていたサンドボックスの期限が切れ、プッシュされていない変更は失われました。",
    description:
      "Transcript line under a stopped turn: its kept sandbox expired and the unpushed changes in it are gone",
  },
  "agentSessions.turnRetentionLostStorageFull": {
    message:
      "ワークスペースのサンドボックスのストレージが満杯だったため、プッシュされていない変更は保持されませんでした。",
    description:
      "Transcript line under a stopped turn: nothing was kept because the workspace's sandbox storage cap was full",
  },
  "agentSessions.turnRetentionLostSaveFailed": {
    message: "プッシュされていない変更を保存できず、失われました。",
    description:
      "Transcript line under a stopped turn: saving its sandbox failed, so the unpushed changes are gone",
  },
  "agentSessions.turnRetentionLostEditorOpen": {
    message:
      "サンドボックスでエディターが開かれていたため、プッシュされていない変更は保持されませんでした。",
    description:
      "Transcript line under a stopped turn: nothing was kept because an editor (SSH) was open on the sandbox, which Stop does not end",
  },
  "agentSessions.turnRetentionLostNotRestored": {
    message:
      "プッシュされていない変更を復元できなかったため、次のメッセージは新しいチェックアウトから始まりました。",
    description:
      "Transcript line under a stopped turn: the next message could not start from the kept changes and started from a fresh checkout",
  },
  "agentSessions.turnRetentionLost": {
    message: "プッシュされていない変更は失われました。",
    description:
      "Transcript line under a stopped turn: its unpushed changes were lost, for a reason that is not worth naming (the session was canceled or archived)",
  },
  "agentSessions.turnRetentionLostSandboxLost": {
    message:
      "ターンが失敗し、サンドボックスがなくなったため、プッシュされていない変更は失われました。",
    description:
      "Transcript line under a failed turn: its sandbox was gone before its disk could be kept, so the unpushed work in it is lost (w10/m491)",
  },
  "agentSessions.turnStakeCommits_other": {
    message: "プッシュされていないコミット {count} 件",
    description:
      "Under a turn whose disk was kept or lost: how many commits its checkout held that were not pushed (w10/m491)",
  },
  "agentSessions.turnStakeChanges": {
    message: "コミットされていない変更",
    description:
      "Under a turn whose disk was kept or lost: its checkout held uncommitted changes (w10/m491)",
  },
  "agentSessions.turnError": {
    message: "ターン {turn} が失敗しました",
    description:
      "Recorded failure of an earlier turn; subsequent turns remain readable",
  },
  "agentSessions.continuitySessionLoad": {
    message: "エージェントは、復元された会話の状態で再開しました。",
    description:
      "System line when a fresh agent generation reloaded its own saved session (ADR047 D3 ladder rung 1)",
  },
  "agentSessions.continuityReprime": {
    message:
      "エージェントが再起動しました。セッション履歴からコンテキストを再構築しました。",
    description:
      "System line when a fresh agent generation was primed from the durable transcript (ladder rung 2)",
  },
  "agentSessions.continuityTaskRedelivery": {
    message: "エージェントが再起動しました。元のタスクが再送されました。",
    description:
      "System line when no prior turn ever reached the agent, so the task was re-sent (ladder rung 3)",
  },
  "agentSessions.conversationEndedEmpty": {
    message: "このセッションは、会話が記録される前に終了しました。",
    description: "Ended session with no durable transcript parts",
  },
  "agentSessions.conversationEnded": {
    message: "セッションが終了しました。",
    description: "Footer note under a terminal session's replayed transcript",
  },
  "agentSessions.conversationIncomplete": {
    message: "アシスタントの出力の一部を保存できませんでした",
    description:
      "Warning below a durable user turn whose assistant transcript is incomplete",
  },
  "agentSessions.conversationUnavailable": {
    message: "現在、会話ストリームを利用できません。",
    description:
      "Degraded-state message when the m43 stream endpoint errors or is unconfigured",
  },
  "agentSessions.conversationUnavailableTerminal": {
    message: "このセッションの会話トランスクリプトは利用できません。",
    description:
      "Terminal session — transcript unavailable rather than live stream degraded",
  },
  "agentSessions.showEarlierMessages_other": {
    message: "以前のメッセージ {count} 件を表示",
    description:
      "Button revealing older transcript messages hidden by the render window (plural)",
  },
  "agentSessions.groupThought": {
    message: "思考",
    description: "Collapsible header for the agent's reasoning group",
  },
  "agentSessions.groupPlan": {
    message: "プラン",
    description: "Collapsible header for the agent's plan/task checklist group",
  },
  "agentSessions.groupTerminal": {
    message: "ターミナル",
    description: "Collapsible header for a terminal-output group",
  },
  "agentSessions.groupCommand": {
    message: "コマンド",
    description: "Collapsible header for a command/tool-call group",
  },
  "agentSessions.groupDiff": {
    message: "差分",
    description: "Label for a file-diff group when no path is given",
  },
  "agentSessions.terminalNoOutput": {
    message: "（出力なし）",
    description: "Placeholder when a terminal group has no captured output",
  },
  "agentSessions.toolInput": {
    message: "入力",
    description: "Section label for a tool call's input",
  },
  "agentSessions.toolOutput": {
    message: "出力",
    description: "Section label for a tool call's output",
  },
  "agentSessions.toolError": {
    message: "エラー",
    description: "Section label for a tool call's error text",
  },
  "agentSessions.toolStateRunning": {
    message: "実行中",
    description: "Badge label for a tool call awaiting output",
  },
  "agentSessions.toolStateDone": {
    message: "完了",
    description: "Badge label for a completed tool call",
  },
  "agentSessions.toolStateDenied": {
    message: "拒否",
    description: "Badge label for a tool call whose execution was denied",
  },
  "agentSessions.toolStateInterrupted": {
    message: "中断",
    description:
      "Badge label for a tool call the session ended before it finished",
  },
  "agentSessions.toolStateError": {
    message: "失敗",
    description: "Badge label for a failed tool call",
  },
  "agentSessions.activityEdited_other": {
    message: "{count} 個のファイルを編集",
    description: "Activity-group summary when the turn edited files (diffs)",
  },
  "agentSessions.activityRan_other": {
    message: "{count} 個のコマンドを実行",
    description: "Activity-group summary when the turn ran commands/terminals",
  },
  "agentSessions.activitySteps_other": {
    message: "{count} ステップ",
    description: "Activity-group summary fallback — the step count (plural)",
  },
  "agentSessions.activityRead_other": {
    message: "{count} 個のファイルを読み取り",
    description: "Activity-group summary segment — file reads",
  },
  "agentSessions.activitySearched_other": {
    message: "{count} 回検索",
    description: "Activity-group summary segment — searches",
  },
  "agentSessions.activitySubagents_other": {
    message: "{count} 個のサブエージェントを実行",
    description: "Activity-group summary segment — delegated subagent tasks",
  },
  "agentSessions.activityTools_other": {
    message: "{count} 個のツールを使用",
    description: "Activity-group summary segment — tools of no known kind",
  },
  "agentSessions.activityThought_other": {
    message: "{count} 回思考",
    description:
      "Activity-group summary segment — the agent's thoughts between its calls",
  },
  "agentSessions.activityThinking": {
    message: "思考中…",
    description: "Live status line while the agent is thinking between calls",
  },
  "agentSessions.activityFailed_other": {
    message: "{count} 件失敗",
    description: "Activity-group summary segment — failed tool calls",
  },
  "agentSessions.outputClipped_other": {
    message: "{formatted} 文字を省略",
    description: "Badge on a tool output the server stored head+tail-capped",
  },
  "agentSessions.showFullOutput": {
    message: "出力全体を表示",
    description:
      "Button on a clipped tool output that loads the complete text the server kept",
  },
  "agentSessions.hideFullOutput": {
    message: "出力全体を非表示",
    description: "Button that collapses a loaded full tool output",
  },
  "agentSessions.fullOutputNotRetained": {
    message: "出力全体は保持されていません",
    description:
      "Note on a clipped tool output whose complete text the server did not keep (clipped before full copies were stored)",
  },
  "agentSessions.fullOutputLoading": {
    message: "出力全体を読み込み中…",
    description: "Status while a tool call's full output loads",
  },
  "agentSessions.fullOutputError": {
    message: "出力全体を読み込めませんでした。",
    description: "Inline error when loading a tool call's full output failed",
  },
  "agentSessions.fullOutputCopy": {
    message: "出力全体をコピー",
    description:
      "Accessible label of the copy button on a loaded full tool output",
  },
  "agentSessions.fullOutputCopied": {
    message: "出力全体をコピーしました",
    description: "Toast after copying a full tool output",
  },
  "agentSessions.fullOutputCopyError": {
    message: "出力全体をコピーできませんでした",
    description: "Toast when copying a full tool output failed",
  },
  "agentSessions.conversationPartial": {
    message: "会話は一部のみ読み込まれました。",
    description:
      "Inline row when the transcript stream failed after some of it rendered",
  },
  "agentSessions.liveStatus": {
    message: "エージェントのステータス",
    description:
      "Accessible label for the single live status line under a streaming transcript",
  },
  "agentSessions.scrollToBottom": {
    message: "最新へスクロール",
    description:
      "Accessible label for the floating jump-to-bottom button in the conversation column",
  },
  "agentSessions.recentSessions": {
    message: "最近",
    description: "Sessions sidebar — heading over the recent sessions list",
  },
  "agentSessions.sidebarEmpty": {
    message: "セッションはまだありません",
    description: "Sessions sidebar — empty state when the workspace has none",
  },
  "agentSessions.sidebarLabel": {
    message: "エージェントセッション",
    description: "Sessions sidebar — accessible landmark label",
  },
  "agentSessions.sidebarSearch": {
    message: "セッションを検索",
    description: "Sessions sidebar — search toggle + input accessible label",
  },
  "agentSessions.sidebarSearchPlaceholder": {
    message: "セッションを検索…",
    description: "Sessions sidebar — search input placeholder",
  },
  "agentSessions.sidebarNoMatches": {
    message: "一致するセッションはありません",
    description:
      "Sessions sidebar — empty state when the search matches nothing",
  },
  "agentSessions.statusPhrase.prReady": {
    message: "PR の準備ができました",
    description: "Sidebar status phrase — completed session with a draft PR",
  },
  "agentSessions.statusPhrase.working": {
    message: "作業中…",
    description: "Sidebar status phrase — session still converging",
  },
  "agentSessions.statusPhrase.waitingForSandbox": {
    message: "サンドボックスマシンを待っています",
    description:
      "Session status (header, list rows, sidebar, conversation, automation runs) — the turn waits because no sandbox machine has room for it yet (w10/m497); the time waited follows where there is room",
  },
  "agentSessions.menuMore": {
    message: "その他の操作",
    description: "Header — accessible label for the '…' overflow menu",
  },
  "agentSessions.openPr": {
    message: "プルリクエストを開く",
    description: "Header overflow menu — open the draft PR in a new tab",
  },
  "agentSessions.connect": {
    message: "接続",
    description: "Header — trigger for the Open-in-Zed / SSH connect menu",
  },
  "agentSessions.openInZed": {
    message: "Zed で開く",
    description:
      "Connect menu — hotlink that opens the sandbox as a Zed remote project",
  },
  "agentSessions.openInZedHint": {
    message:
      "SSH 経由でサンドボックスの /workspace を開きます。Zed エディターが必要です。",
    description: "Connect menu — helper text under the Open-in-Zed action",
  },
  "agentSessions.connectSSH": {
    message: "SSH",
    description: "Connect menu — label above the copyable ssh command",
  },
  "agentSessions.sshCopy": {
    message: "SSH コマンドをコピー",
    description: "Connect menu — accessible label for the copy button",
  },
  "agentSessions.sshCopied": {
    message: "SSH コマンドをコピーしました",
    description: "Connect menu — toast after copying the ssh command",
  },
  "agentSessions.sshCopyError": {
    message: "コピーできませんでした",
    description: "Connect menu — toast when copying the ssh command fails",
  },
  "agentSessions.groupThoughtFor": {
    message: "{duration} 思考",
    description:
      "Reasoning-group summary with elapsed duration from persisted part timestamps (or ~arrival timing)",
  },
  "agentSessions.terminalStatus.completed": {
    message: "エージェントが完了しました",
    description: "Terminal transcript status line — completed session",
  },
  "agentSessions.terminalStatus.failed": {
    message: "セッションはエラーで終了しました",
    description: "Terminal transcript status line — failed session",
  },
  "agentSessions.terminalStatus.canceled": {
    message: "セッションはキャンセルされました",
    description: "Terminal transcript status line — canceled session",
  },
  "agentSessions.tagsLabel": {
    message: "タグ",
    description: "Accessible label for a session row's tag list (w21/002)",
  },
  "agentSessions.filterByTag": {
    message: "タグ {tag} のセッションを表示",
    description:
      "Accessible label of a tag chip that filters the session list to that tag",
  },
  "agentSessions.filterTagged": {
    message: "タグ: {tag}",
    description: "Session list heading while narrowed to one tag",
  },
  "agentSessions.archive": {
    message: "アーカイブ",
    description: "Action putting a session out of the working set (ADR065)",
  },
  "agentSessions.unarchive": {
    message: "アーカイブ解除",
    description: "Action returning an archived session to the working set",
  },
  "agentSessions.archiveSuccess": {
    message: "セッションをアーカイブしました",
    description: "Toast after archiving a session",
  },
  "agentSessions.undoArchive": {
    message: "元に戻す",
    description: "Toast action that immediately unarchives a session",
  },
  "agentSessions.unarchiveSuccess": {
    message: "セッションのアーカイブを解除しました",
    description: "Toast after unarchiving a session",
  },
  "agentSessions.archivedBadge": {
    message: "アーカイブ済み",
    description: "Badge marking a session as archived (out of the working set)",
  },
  "agentSessions.startedFromSnapshot": {
    message: "スナップショットから開始",
    description:
      "Session header chip: the latest turn booted from a repository environment build (ADR111 D4)",
  },
  "agentSessions.startedFromSnapshotTitle": {
    message:
      "最新のターンは、セットアップ手順を実行済みの環境ビルド {id} から開始されました。",
    description:
      "Tooltip of the Started from snapshot chip (id is the build id, arb-…)",
  },
  "agentSessions.delete": {
    message: "削除",
    description: "Destructive action permanently deleting a finished session",
  },
  "agentSessions.deleting": {
    message: "削除中…",
    description: "Delete confirm button while the delete is in flight",
  },
  "agentSessions.deleteSuccess": {
    message: "セッションを削除しました",
    description: "Toast after permanently deleting a session",
  },
  "agentSessions.deleteConfirmTitle": {
    message: "このセッションを削除しますか？",
    description: "Delete confirmation dialog title",
  },
  "agentSessions.deleteConfirmBody": {
    message:
      "セッションの記録と会話のトランスクリプトが完全に削除されます。GitHub 上のプッシュ済みブランチとプルリクエストには影響しません。この操作は元に戻せません。",
    description: "Delete confirmation dialog body",
  },
  "agentSessions.deleteConfirmDismiss": {
    message: "セッションを残す",
    description: "Delete confirmation dialog dismiss button",
  },
  "agentSessions.deleteConfirmProceed": {
    message: "セッションを削除",
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
    message: "アーカイブ済み",
    description: "List membership tab — archived sessions only",
  },
  "agentSessions.filterAll": {
    message: "すべて",
    description: "List membership tab — archived and unarchived together",
  },
  "agentSessions.loadMore": {
    message: "さらに読み込む",
    description: "Button fetching the next page of sessions",
  },
  "agentSessions.loadingMore": {
    message: "読み込み中…",
    description: "Load-more button while the next page is in flight",
  },
  "agentSessions.attachFiles": {
    message: "ファイルを添付",
    description:
      "Paperclip button that opens the file picker in the agent composers",
  },
  "agentSessions.attachmentRemove": {
    message: "{name} を削除",
    description: "Accessible label of the remove button on an attachment chip",
  },
  "agentSessions.attachmentTooLarge": {
    message: "{max} を超えています",
    description: "Attachment chip error when a file exceeds the per-file limit",
  },
  "agentSessions.attachmentTooMany": {
    message: "1 つのメッセージにつき最大 {max} 個のファイル",
    description: "Attachment chip error when a message has too many files",
  },
  "agentSessions.attachmentDropHint": {
    message: "ファイルをドロップして添付",
    description: "Overlay shown while dragging files over a composer",
  },
  "agentSessions.sharedFileLabel": {
    message: "ファイルを共有しました",
    description: "Label above a file the agent shared with share_file",
  },
  "agentSessions.filesButton": {
    message: "ファイル",
    description: "Session header button that opens the session files",
  },
  "agentSessions.filesTitle": {
    message: "セッションのファイル",
    description: "Title of the session files sheet",
  },
  "agentSessions.filesDescription": {
    message:
      "このセッションで添付したファイルと、エージェントが共有したファイルです。",
    description: "Description under the session files sheet title",
  },
  "agentSessions.filesEmpty": {
    message:
      "ファイルはまだありません。メッセージにファイルを添付するか、エージェントに共有を依頼してください。",
    description: "Empty state of the session files sheet",
  },
  "agentSessions.filesError": {
    message: "セッションのファイルを読み込めませんでした。",
    description: "Error state of the session files sheet",
  },
  "agentSessions.filesColumnName": {
    message: "名前",
    description: "Files table column: file name",
  },
  "agentSessions.filesColumnFrom": {
    message: "追加者",
    description: "Files table column: who added the file",
  },
  "agentSessions.filesColumnTurn": {
    message: "ターン",
    description: "Files table column: turn number",
  },
  "agentSessions.filesColumnSize": {
    message: "サイズ",
    description: "Files table column: file size",
  },
  "agentSessions.filesFromInput": {
    message: "あなた",
    description: "Files table: a file the user attached",
  },
  "agentSessions.filesFromOutput": {
    message: "エージェント",
    description: "Files table: a file the agent shared",
  },
  "agentSessions.filesLoadMore": {
    message: "さらに読み込む",
    description: "Files table: load the next page",
  },
  "agentSessions.attachmentsBlockSend": {
    message: "送信するには、アップロードに失敗したファイルを削除してください",
    description: "Hint when failed attachment chips block sending",
  },
  "agentSessions.attachmentOpen": {
    message: "{name} を開く",
    description:
      "Accessible label of an attachment thumbnail that opens the lightbox",
  },
  "agentSessions.attachmentDownload": {
    message: "{name} をダウンロード",
    description: "Accessible label of an attachment download card",
  },
  "agentSessions.attachmentUnavailable": {
    message: "利用不可",
    description:
      "Shown on an attachment whose download link could not be loaded",
  },
  "agentSessions.attachmentUploading": {
    message: "アップロード中、{percent}%",
    description: "Screen-reader status of an attachment chip while uploading",
  },
  "agentSessions.attachmentReady": {
    message: "アップロード済み",
    description:
      "Screen-reader status of an attachment chip once its upload landed",
  },
  "agentSessions.attachmentHttpError": {
    message: "アップロードに失敗しました（HTTP {status}）",
    description:
      "Attachment chip error when the server answered without a message",
  },
  "agentSessions.attachmentNetworkError": {
    message: "アップロードに失敗しました: ネットワークエラー",
    description:
      "Attachment chip error when the upload could not reach the server",
  },
  "agentSessions.attachmentsUploading": {
    message: "ファイルのアップロードが完了するまでお待ちください…",
    description: "Composer hint while attachment uploads block sending",
  },
  "agentSessions.spendWarningTitle": {
    message: "エージェントのモデル支出が今月の上限の {percent}% に達しました",
    description: "Composer warning at 80% of the spend cap (ADR091 D6)",
  },
  "agentSessions.spendWarningBody": {
    message:
      "{cap} のうち {used} を使用しました。上限に達すると、新しいモデルリクエストは停止されます。",
    description: "Composer warning body",
  },
  "agentSessions.spendWarningBodyCharge": {
    message:
      "請求クレジットを超える支出として {cap} のうち {used} を使用しました。まず請求クレジットが充当され、それを超える支出が {cap} に達すると、登録済みのカードに請求され、セッションは継続して実行されます。請求に失敗した場合のみ、新しいモデルリクエストが停止されます。",
    description:
      "Composer warning body when the cap is a new workspace's charge point (ADR091 D10.2a); {used} is the spend billing credits leave",
  },
  "agentSessions.spendWarningBodyCeiling": {
    message:
      "請求クレジットを超える支出として {cap} のうち {used} を使用しました。まず請求クレジットが充当され、それを超える支出が {cap} に達すると、新しいモデルリクエストは停止されます。",
    description:
      "Composer warning body when the trust tier's ceiling binds on a workspace bex cannot charge (payment hold, excluded or comped) and the spend it counts, what billing credits leave ({used}), differs from month-to-date",
  },
  "agentSessions.spendReachedTitle.cap": {
    message: "エージェントのモデル支出の月間上限に達しました",
    description: "Composer refusal title: the workspace's own cap",
  },
  "agentSessions.spendReachedTitle.newWorkspace": {
    message: "新規ワークスペースの支出上限に達しました",
    description: "Composer refusal title: the T0 trust-tier ceiling",
  },
  "agentSessions.spendReachedTitle.locked": {
    message: "エージェントのモデル支出は保留中です",
    description: "Composer refusal title: a payment hold",
  },
  "agentSessions.spendReachedBody.cap": {
    message:
      "このワークスペースは、月間上限 {cap} のうち {used} を使用したため、新しいセッションを開始できません。ワークスペースの管理者が上限を引き上げるか、毎月 1 日（UTC）にリセットされます。",
    description: "Composer refusal body: the workspace's own cap",
  },
  "agentSessions.spendReachedBody.capMax": {
    message:
      "このワークスペースは、設定できる最大値である月間上限 {cap} のうち {used} を使用したため、新しいセッションを開始できません。上限は毎月 1 日（UTC）にリセットされます。より高い上限が必要な場合は、サポートにお問い合わせください。",
    description:
      "Composer refusal body: the workspace's own cap binds at its maximum, so no admin can raise it",
  },
  "agentSessions.spendReachedBody.newWorkspace": {
    message:
      "新規ワークスペースは、最初の支払いが成功するまで、bex 提供のモデルに最大 {cap} まで支出できます。その後は上限が引き上げられるか、毎月 1 日（UTC）にリセットされます。",
    description: "Composer refusal body: the T0 trust-tier ceiling",
  },
  "agentSessions.spendReachedBody.locked": {
    message:
      "支払いの失敗または異議申し立てにより、このワークスペースのエージェントのモデル支出は {cap} に制限されています。請求ページで未払いの支払いを解決してから、新しいセッションを開始してください。",
    description: "Composer refusal body: a payment hold",
  },
  "agentSessions.spendIncreaseCap": {
    message: "上限を引き上げる",
    description: "Admin action linking to Settings → Agents",
  },
  "agentSessions.spendGoToBilling": {
    message: "請求ページへ",
    description: "Link to the Billing page",
  },
  "agentSessions.spendBalanceDueTitle": {
    message: "支払いに失敗しました",
    description: "Composer refusal title: BILLING_BALANCE_DUE",
  },
  "agentSessions.spendBalanceDueBody": {
    message:
      "未払いの請求書が支払われるまで、bex 提供のモデルを使用するエージェントセッションは一時停止されます。",
    description: "Composer refusal body: BILLING_BALANCE_DUE",
  },
  "agentSessions.errors.AGENT_SESSION_TRUST_LIMIT": {
    message:
      "新規ワークスペースは、最初の支払いが成功するまで、bex 提供のモデルで同時に {limit} 件のエージェントセッションを実行できます。実行中のセッションが終了するまで待つか、いずれかを停止してから、もう一度お試しください。",
    description: "Mapped message for AGENT_SESSION_TRUST_LIMIT (ADR091 D10.1)",
  },
  "agentSessions.errors.AGENT_SESSION_TRUST_LIMIT_locked": {
    message:
      "支払いの失敗または異議申し立てにより、このワークスペースは bex 提供のモデルで同時に {limit} 件のエージェントセッションまで実行できます。制限を解除するには、請求ページで未払いの支払いを解決してください。",
    description:
      "Mapped message for AGENT_SESSION_TRUST_LIMIT while a payment hold applies",
  },
  "agentSessions.errors.AGENT_SESSION_SPEND_LIMIT_INVALID": {
    message: "支出上限は、$0 より大きく $100,000 以下にしてください。",
    description:
      "Mapped message for AGENT_SESSION_SPEND_LIMIT_INVALID (ADR094 D8)",
  },
  "agentSessions.errors.AGENT_SESSION_SPEND_LIMIT_UNAVAILABLE": {
    message:
      "支出上限は、bex 提供のモデルを使用するセッションにのみ適用されます。このワークスペースは独自のモデルキーを使用しているため、上限をクリアするか、設定 → エージェントでワークスペースを bex のモデルに切り替えてください。",
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
      "任意です。bex モデルの支出がこの金額に達すると、セッションは停止します（$0 より大きく $100,000 以下）。",
    description: "Composer — help text under the spend limit input",
  },
  "agentSessions.spendLimitInvalid": {
    message:
      "$0 より大きく $100,000 以下の金額を入力するか、空欄のままにしてください。",
    description: "Composer — inline error for an out-of-range spend limit",
  },
  "agentSessions.failureSessionSpendLimitReached": {
    message:
      "モデルの支出がセッションの上限 {limit} に達したため、このセッションは停止しました。続けるには、より高い上限で新しいセッションを開始してください。",
    description:
      "Failure callout for AGENT_MODEL_SPEND_CAP_REACHED when the session's own maxSpendUSD ceiling was hit",
  },
  "agentSessions.spendNote": {
    message: "モデル支出 {spent}",
    description:
      "Session page — the session's model spend so far (outcome.cost)",
  },
  "agentSessions.spendNoteWithLimit": {
    message: "モデル支出 {spent}（セッションの上限 {limit}）",
    description:
      "Session page — model spend so far beside the session's spend limit",
  },
  "agentSessions.costNote": {
    message: "コスト {total} · モデル {model}、コンピュート {compute}",
    description:
      "Session page — the session's total cost so far (outcome.cost.totalUSD) with its model and sandbox compute parts",
  },
  "agentSessions.costNoteWithLimit": {
    message:
      "コスト {total} · モデル {model}（セッションの上限 {limit}）、コンピュート {compute}",
    description:
      "Session page — total cost with its parts; the session spend limit caps the model part",
  },
  "agentSessions.resultTitle": {
    message: "結果",
    description:
      "Session page — heading over the turn's final structured result",
  },
  "agentSessions.resultDraftTitle": {
    message: "結果のドラフト",
    description:
      "Session page — heading over the agent's not-yet-final structured result while the turn runs",
  },
  "agentSessions.resultDraftHint": {
    message: "まだ確定していません。エージェントが変更する可能性があります。",
    description: "Session page — note under a draft structured result",
  },
  "agentSessions.resultCopy": {
    message: "JSON をコピー",
    description: "Session page — button copying the structured result",
  },
  "agentSessions.resultCopied": {
    message: "コピーしました",
    description: "Session page — toast after copying the structured result",
  },
  "agentSessions.failureSpendCapReached": {
    message:
      "このワークスペースは、今月のエージェントのモデル支出の上限に達しました。管理者が設定 → エージェントで上限を引き上げるか、毎月 1 日（UTC）にリセットされます。",
    description: "Failure callout for AGENT_MODEL_SPEND_CAP_REACHED",
  },
  "agentSessions.failureBalanceDue": {
    message:
      "このワークスペースの支払いに失敗しました。請求ページで未払いの請求書を支払ってから、新しいセッションを開始してください。",
    description: "Failure callout for BILLING_BALANCE_DUE",
  },
  "agentSessions.failureSetupTitle": {
    message: "リポジトリのセットアップに失敗しました",
    description:
      "Failure callout heading when a repository setup step failed (AGENT_SESSION_SETUP_FAILED)",
  },
  "agentSessions.failureSetupHint": {
    message:
      "このリポジトリの環境のセットアップステップが失敗したため、エージェントは起動せず、モデルの支出も発生していません。設定 → エージェント → リポジトリ環境でステップを修正してから、再試行してください。",
    description:
      "Failure callout guidance under the recorded reason (which names the step and its exit code) for AGENT_SESSION_SETUP_FAILED",
  },
  "agentSessions.failureSetupAction": {
    message: "リポジトリ環境を編集",
    description:
      "Failure callout button opening Settings → Repository environments",
  },
  "agentSessions.costTotal": {
    message: "モデルコスト {amount}",
    description: "Session header: the session's model cost (ADR091 D6)",
  },
  "agentSessions.costTotalAll": {
    message: "合計コスト {amount}",
    description:
      "Session header: the session's total cost so far, model plus sandbox compute (outcome.cost.totalUSD, w9/m178)",
  },
  "agentSessions.costBreakdown": {
    message: "モデル {model} · コンピュート {compute}",
    description:
      "Session header cost popover: the model and sandbox compute parts of the total",
  },
  "agentSessions.costByo": {
    message: "プロバイダーから請求されます",
    description: "Session header: a BYO-key session has no bex model cost",
  },
  "agentSessions.costTitle": {
    message: "ターンごとのモデルコスト",
    description: "Heading of the per-turn cost popover",
  },
  "agentSessions.costTurn": {
    message: "ターン {turn}",
    description: "One turn's row in the cost popover",
  },
  "agentSessions.costSetup": {
    message: "最初のターンより前",
    description: "Requests recorded before the first turn",
  },
  "agentSessions.costTurnTokens": {
    message: "入力 {input} · 出力 {output} · キャッシュ {cache} トークン",
    description: "A turn's token counts",
  },
  "agentSessions.costPricing": {
    message: "モデルプロバイダーの原価 + 30% で請求されます。",
    description: "Pricing statement in the cost popover",
  },
  "agentSessions.secretsButton": {
    message: "シークレット",
    description: "Composer toolbar: the Secrets button label/tooltip (ADR098)",
  },
  "agentSessions.secretsButtonCount_other": {
    message: "シークレット（{count} 件添付）",
    description:
      "Composer Secrets button accessible label when secrets are attached",
  },
  "agentSessions.secretsTitle": {
    message: "このセッションのシークレット",
    description: "Composer Secrets popover heading",
  },
  "agentSessions.secretsDescription": {
    message:
      "エージェントは、それぞれを環境変数として読み取ります。値は再表示されず、トランスクリプトでは伏せ字になります。",
    description: "Composer Secrets popover description",
  },
  "agentSessions.secretsWorkspace": {
    message: "ワークスペースのシークレット",
    description:
      "Composer Secrets popover: workspace agent secrets list heading",
  },
  "agentSessions.secretsWorkspaceLoadFailed": {
    message:
      "ワークスペースのエージェントのシークレットを読み込めませんでした。",
    description:
      "Composer secrets panel error when the workspace secret list fails to load",
  },
  "agentSessions.secretsWorkspaceEmpty": {
    message: "ワークスペースのシークレットはまだありません。",
    description: "Composer Secrets popover: no workspace agent secrets",
  },
  "agentSessions.secretsManage": {
    message: "設定で管理",
    description: "Composer Secrets popover: link to Settings → Agents",
  },
  "agentSessions.secretsInline": {
    message: "このセッションのみ",
    description: "Composer Secrets popover: inline secrets heading",
  },
  "agentSessions.secretsRemove": {
    message: "{name} を削除",
    description:
      "Composer Secrets popover: remove an inline secret (accessible label)",
  },
  "agentSessions.secretsNamePlaceholder": {
    message: "NAME",
    description: "Composer Secrets popover: inline secret name placeholder",
  },
  "agentSessions.secretsNameLabel": {
    message: "シークレット名",
    description: "Composer Secrets popover: inline secret name field label",
  },
  "agentSessions.secretsValuePlaceholder": {
    message: "値",
    description: "Composer Secrets popover: inline secret value placeholder",
  },
  "agentSessions.secretsValueLabel": {
    message: "シークレットの値",
    description: "Composer Secrets popover: inline secret value field label",
  },
  "agentSessions.secretsAdd": {
    message: "追加",
    description: "Composer Secrets popover: add an inline secret",
  },
  "agentSessions.secretsHint": {
    message: "セッションとともに削除されます。",
    description: "Composer Secrets popover: inline secret lifetime hint",
  },
  "agentSessions.secretsFull": {
    message: "1 つのセッションに設定できるシークレットは最大 20 個です。",
    description: "Composer Secrets popover: session secret limit reached",
  },
  "agentSessions.secretsDuplicate": {
    message: "このセッションには、その名前のシークレットがすでにあります。",
    description: "Composer Secrets popover: duplicate inline secret name",
  },
  "agentSessions.secretsScopeRepo": {
    message: "このリポジトリ",
    description:
      "Composer Secrets popover: hint beside a secret scoped to the chosen repository, included",
  },
  "agentSessions.secretsScopeOtherRepo": {
    message: "{repo} のみ",
    description:
      "Composer Secrets popover: hint beside a secret scoped to another repository, not available here",
  },
  "agentSessions.appliedSecretsTitle": {
    message: "最新のターンのシークレット",
    description:
      "Session header: heading of the applied-secrets popover (names and sources, never values)",
  },
  "agentSessions.appliedSecretSource.inline": {
    message: "このセッション",
    description:
      "Applied secret source: an inline secret given to this session only",
  },
  "agentSessions.appliedSecretSource.explicit": {
    message: "選択済み",
    description: "Applied secret source: a workspace secret the session picked",
  },
  "agentSessions.appliedSecretSource.repo": {
    message: "リポジトリ",
    description:
      "Applied secret source: a workspace secret scoped to the session's repository",
  },
  "agentSessions.appliedSecretSource.workspace": {
    message: "ワークスペース",
    description:
      "Applied secret source: a workspace secret every session receives",
  },
  "agentSessions.metaSecrets_other": {
    message: "{count} 個のシークレット",
    description: "Detail header — number of attached secrets (plural, ADR098)",
  },
  "agentSessions.skillsButton": {
    message: "スキル",
    description: "Composer toolbar: the Skills button label/tooltip (ADR099)",
  },
  "agentSessions.skillsButtonCount_other": {
    message: "スキル（{count} 件選択）",
    description:
      "Composer Skills button accessible label when skills are chosen",
  },
  "agentSessions.skillsTitle": {
    message: "このターンのスキル",
    description: "Composer Skills popover heading",
  },
  "agentSessions.skillsDescription": {
    message:
      "エージェントは、選択した各スキルに従います。有効なスキルは、エージェントが自ら適用することもあります。",
    description: "Composer Skills popover description",
  },
  "agentSessions.skillsLoadFailed": {
    message: "ワークスペースのスキルを読み込めませんでした。",
    description: "Composer Skills popover: list failed",
  },
  "agentSessions.skillsEmpty": {
    message: "このワークスペースには、まだスキルがありません。",
    description: "Composer Skills popover: none exist",
  },
  "agentSessions.skillsManage": {
    message: "スキルを管理",
    description: "Composer Skills popover link to Settings → Agents",
  },
  "agentSessions.skillsArgumentsLabel": {
    message: "{name} の引数",
    description: "Composer Skills popover: argument field label",
  },
  "agentSessions.skillsArgumentsPlaceholder": {
    message: "引数（任意）",
    description: "Composer Skills popover: argument field placeholder",
  },
  "agentSessions.skillsArgumentsInvalid": {
    message: "引数は 1 行で、4 KiB 以内にしてください。",
    description: "Composer Skills popover: invalid arguments",
  },
  "agentSessions.skillsFull": {
    message: "1 つのターンで使用できるスキルは最大 5 個です。",
    description: "Composer Skills popover: limit reached",
  },
  "agentSessions.skillsHint": {
    message: "引数は、スキル内の $ARGUMENTS に入ります。",
    description: "Composer Skills popover hint",
  },
  "agentSessions.skillsRemove": {
    message: "{name} を削除",
    description: "Accessible label of a chosen skill chip's remove button",
  },
  "agentSessions.turnSkills": {
    message: "このターンで使用したスキル",
    description: "Accessible label of a transcript turn's skill pills",
  },
  "agentSessions.turnOrigins": {
    message: "このターンのきっかけ",
    description:
      "Accessible label of the lines above a transcript turn that PR feedback started (w9/m344)",
  },
  "agentSessions.turnOriginReview": {
    message: "@{author} のレビューにより誘導",
    description:
      "A turn started by a pull-request review; {author} is the reviewer's GitHub login. Links to the review",
  },
  "agentSessions.turnOriginReviewAnonymous": {
    message: "PR のレビューにより誘導",
    description:
      "A turn started by a pull-request review whose author is unknown. Links to the review",
  },
  "agentSessions.turnOriginComment": {
    message: "@{author} の PR コメントにより誘導",
    description:
      "A turn started by a pull-request comment; {author} is the commenter's GitHub login. Links to the comment",
  },
  "agentSessions.turnOriginCommentAnonymous": {
    message: "PR のコメントにより誘導",
    description:
      "A turn started by a pull-request comment whose author is unknown. Links to the comment",
  },
  "agentSessions.turnOriginCiFailure": {
    message: "CI の実行失敗により誘導",
    description:
      "A turn started by a failed CI run on the pull request. Links to the failed checks",
  },
  "agentSessions.turnOriginCiFailureOnBase": {
    message:
      "CI の実行失敗により誘導。一部のチェックは {base} でも失敗しており、この変更が原因ではありません",
    description:
      "A turn started by a failed CI run on the pull request where at least one failing check also fails on the base branch ({base}, a branch name), so the pull request did not cause it. Links to the failed checks",
  },
  "agentSessions.turnOriginsStart": {
    message: "このセッションのきっかけ",
    description:
      "Accessible label of the line above a session's first turn when a /bex comment on a pull request started it (w9/m348)",
  },
  "agentSessions.turnOriginStartComment": {
    message: "@{author} のコメントから開始",
    description:
      "The first turn of a session a /bex pull-request comment started; {author} is the commenter's GitHub login. Links to the comment",
  },
  "agentSessions.turnOriginStartCommentAnonymous": {
    message: "PR のコメントから開始",
    description:
      "The first turn of a session a /bex pull-request comment started, when the commenter is unknown. Links to the comment",
  },
  "agentSessions.lineage.childSessions": {
    message: "子セッション",
    description:
      "Heading of the block listing sessions the agent spawned, in the parent session's transcript",
  },
  "agentSessions.lineage.childCardTitle": {
    message: "セッションを作成しました",
    description:
      "Small label above a child session's title on its card in the parent's transcript",
  },
  "agentSessions.lineage.agentsPanel": {
    message: "エージェント",
    description:
      "Heading of the session page's right-hand panel listing the sessions below this one",
  },
  "agentSessions.lineage.viewSession": {
    message: "セッションを表示",
    description: "Link on an Agents panel row that opens that child session",
  },
  "agentSessions.lineage.parentSession": {
    message: "親セッション",
    description:
      "Link in the Agents panel of a child session that opens its parent",
  },
  "agentSessions.lineage.collapsePanel": {
    message: "エージェントパネルを折りたたむ",
    description:
      "Accessible label of the button that collapses the Agents panel",
  },
  "agentSessions.lineage.expandPanel": {
    message: "エージェントパネルを展開",
    description: "Accessible label of the button that expands the Agents panel",
  },
  "agentSessions.lineage.collapseChildren": {
    message: "子セッションを非表示",
    description:
      "Accessible label of a sidebar row's button that collapses its nested child sessions",
  },
  "agentSessions.lineage.expandChildren": {
    message: "子セッションを表示",
    description:
      "Accessible label of a sidebar row's button that expands its nested child sessions",
  },
  "agentSessions.lineage.status.working": {
    message: "作業中",
    description:
      "Status group and badge of a child session that is still running",
  },
  "agentSessions.lineage.status.done": {
    message: "完了",
    description:
      "Status group and badge of a child session that finished successfully",
  },
  "agentSessions.lineage.status.failed": {
    message: "失敗",
    description: "Status group and badge of a child session that failed",
  },
  "agentSessions.lineage.status.canceled": {
    message: "キャンセル済み",
    description: "Status group and badge of a child session that was canceled",
  },
  "agentSessions.sidebarMine": {
    message: "自分",
    description:
      "Sessions sidebar — show only the sessions you started (Devin's default)",
  },
  "agentSessions.sidebarEveryone": {
    message: "全員",
    description: "Sessions sidebar — show every member's sessions",
  },
  "agentSessions.sidebarOwnerFilter": {
    message: "対象のセッション",
    description:
      "Sessions sidebar — accessible name of the Mine/Everyone toggle",
  },
  "agentSessions.sidebarEmptyMine": {
    message:
      "あなたが開始したセッションはありません。以前のセッションは「全員」に表示されます。",
    description:
      "Sessions sidebar — empty state under Mine. Sessions from before 2026-11-08 record no starter, so they appear only under Everyone",
  },
  "agentSessions.startedByPerson": {
    message: "{name} が開始",
    description: "Session header and list rows — who started the session",
  },
  "agentSessions.startedForPerson": {
    message: "{name} として",
    description:
      "Session header — after 'Started by automation X': the member the automation runs as",
  },
  "agentSessions.editTags": {
    message: "タグを編集",
    description: "Session header menu — open the tag editor",
  },
  "agentSessions.editTagsTitle": {
    message: "タグを編集",
    description: "Tag editor dialog title",
  },
  "agentSessions.editTagsDescription": {
    message:
      "タグは、セッションの現在のタグと置き換えられます。カンマまたはスペースで区切ってください。空にするとクリアされます。",
    description: "Tag editor dialog description",
  },
  "agentSessions.editTagsHint": {
    message:
      "最大 20 個のタグ。小文字の英字、数字、コロン、アンダースコア、ハイフンが使用できます。",
    description: "Tag editor dialog — the tag rules",
  },
  "agentSessions.editTagsSave": {
    message: "タグを保存",
    description: "Tag editor dialog — save button",
  },
  "agentSessions.editTagsSaved": {
    message: "タグを保存しました",
    description: "Toast after the tags were replaced",
  },
};

export default jaAgentSessions;
