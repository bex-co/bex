import type { TranslationEntry } from "@/i18n";

const jaAgentAutomations: Record<string, TranslationEntry> = {
  "agentAutomations.pageTitle": {
    message: "オートメーション",
    description:
      "Page title and sidebar label of the scheduled agent automations page",
  },
  "agentAutomations.detailTitle": {
    message: "オートメーション",
    description: "Browser tab title of one automation's page",
  },
  "agentAutomations.pageDescription": {
    message:
      "スケジュールに従ってエージェントセッションを開始します。実行のたびに、ここで保存したタスクと設定が使われます。",
    description: "Automations list page description",
  },
  "agentAutomations.create": {
    message: "オートメーションを作成",
    description:
      "Create automation button on the list page and in the empty state",
  },
  "agentAutomations.createTitle": {
    message: "オートメーションを作成",
    description: "Heading of the create form",
  },
  "agentAutomations.editTitle": {
    message: "オートメーションを編集",
    description: "Heading of the edit form",
  },
  "agentAutomations.createSubmit": {
    message: "オートメーションを作成",
    description: "Create form submit button",
  },
  "agentAutomations.editSubmit": {
    message: "変更を保存",
    description: "Edit form submit button",
  },
  "agentAutomations.createDenied": {
    message:
      "オートメーションを作成できるのは、開発者ロール以上のメンバーのみです。",
    description: "Create page notice for a member who cannot create",
  },
  "agentAutomations.backToList": {
    message: "オートメーションに戻る",
    description: "Accessible label of the back link to the automations list",
  },
  "agentAutomations.backToAutomation": {
    message: "オートメーションに戻る",
    description: "Button returning from the edit page to the automation's page",
  },
  "agentAutomations.tabMine": {
    message: "自分の分",
    description: "List tab: automations the viewer created",
  },
  "agentAutomations.tabAll": {
    message: "すべて",
    description: "List tab: every automation in the workspace",
  },
  "agentAutomations.emptyMineTitle": {
    message: "オートメーションはまだありません",
    description: "Empty state title on the Mine tab",
  },
  "agentAutomations.emptyAllTitle": {
    message: "このワークスペースにはまだオートメーションがありません",
    description: "Empty state title on the All tab",
  },
  "agentAutomations.emptyBody": {
    message:
      "オートメーションは、選択したタスク、リポジトリ、上限に基づいて、平日の 09:00 などのスケジュールでエージェントセッションを開始します。",
    description: "Empty state explanation",
  },
  "agentAutomations.loadMore": {
    message: "さらに読み込む",
    description: "Load the next page of automations or runs",
  },
  "agentAutomations.loadFailed": {
    message: "オートメーションを読み込めませんでした。",
    description: "Load error",
  },
  "agentAutomations.retry": {
    message: "再試行",
    description: "Retry button on a load error",
  },
  "agentAutomations.forbidden": {
    message:
      "オートメーションを使用できるのは、開発者ロール以上のメンバーのみです。",
    description: "Load error for a member without access",
  },
  "agentAutomations.notFound": {
    message: "このオートメーションは存在しないか、削除されました。",
    description: "Load error for a missing automation",
  },
  "agentAutomations.noTrigger": {
    message: "スケジュールなし",
    description: "Row text when an automation has no schedule",
  },
  "agentAutomations.nextRun": {
    message: "次回の実行",
    description: "Label before the next run time",
  },
  "agentAutomations.lastRun": {
    message: "前回の実行",
    description: "Label before the last run time",
  },
  "agentAutomations.neverRun": {
    message: "未実行",
    description: "Last run value when it has not run",
  },
  "agentAutomations.lastSession": {
    message: "セッションを表示",
    description: "Link to the session the last run started",
  },
  "agentAutomations.pauseLabel": {
    message: "{name} を一時停止",
    description: "Accessible name of the switch that pauses an automation",
  },
  "agentAutomations.resumeLabel": {
    message: "{name} を再開",
    description: "Accessible name of the switch that resumes an automation",
  },
  "agentAutomations.pausedBadge": {
    message: "一時停止中",
    description: "Badge on an automation a person paused",
  },
  "agentAutomations.ended": {
    message: "終了",
    description: "Badge on an automation with no future run",
  },
  "agentAutomations.active": {
    message: "有効",
    description: "Status of a running automation",
  },
  "agentAutomations.paused.repeated_failures": {
    message: "一時停止: 失敗の繰り返し",
    description: "Badge: paused by the platform after repeated failed runs",
  },
  "agentAutomations.paused.creator_lost_access": {
    message: "一時停止: 作成者のアクセス権の喪失",
    description:
      "Badge: paused because the creator can no longer start sessions",
  },
  "agentAutomations.paused.template_invalid": {
    message: "一時停止: テンプレートが無効",
    description:
      "Badge: paused because the session template no longer validates",
  },
  "agentAutomations.paused.unknown": {
    message: "プラットフォームにより一時停止",
    description: "Badge for a pause reason this version does not know",
  },
  "agentAutomations.pausedExplain.repeated_failures": {
    message:
      "このオートメーションは、実行が {failures} 回連続で失敗したため一時停止されました。実行履歴を確認し、原因を修正してから再開してください。",
    description: "Explanation on the detail page for a repeated-failures pause",
  },
  "agentAutomations.pausedExplain.creator_lost_access": {
    message:
      "このオートメーションを作成したメンバーは、セッションを開始するアクセス権を失いました。ワークスペースの管理者が引き継ぐと再開されます。",
    description:
      "Explanation on the detail page for a creator-lost-access pause",
  },
  "agentAutomations.pausedExplain.template_invalid": {
    message:
      "セッションテンプレートが検証を通過しなくなりました（シークレットやモデルの設定が変更されたなどの理由）。作成者が編集してから再開できます。",
    description: "Explanation on the detail page for a template-invalid pause",
  },
  "agentAutomations.pausedExplain.unknown": {
    message:
      "プラットフォームがこのオートメーションを一時停止しました。原因を修正したら再開してください。",
    description: "Explanation for a pause reason this version does not know",
  },
  "agentAutomations.pausedToast": {
    message: "{name} を一時停止しました。",
    description: "Toast after pausing an automation",
  },
  "agentAutomations.resumedToast": {
    message: "{name} を再開しました。",
    description: "Toast after resuming an automation",
  },
  "agentAutomations.runStartedToast": {
    message: "{name} がセッションを開始しました。",
    description: "Toast after Run now started a session",
  },
  "agentAutomations.runSettledToast": {
    message: "{name} を実行しました: {outcome}。",
    description: "Toast after Run now ended with an outcome other than started",
  },
  "agentAutomations.tookOverToast": {
    message: "{name} を引き継ぎました。",
    description: "Toast after an admin takes an automation over",
  },
  "agentAutomations.deletedToast": {
    message: "{name} を削除しました。",
    description: "Toast after deleting an automation",
  },
  "agentAutomations.createdToast": {
    message: "{name} を作成しました。",
    description: "Toast after creating an automation",
  },
  "agentAutomations.updatedToast": {
    message: "{name} を保存しました。",
    description: "Toast after editing an automation",
  },
  "agentAutomations.actionFailed": {
    message: "操作に失敗しました。もう一度お試しください。",
    description: "Generic failure toast for an automation action",
  },
  "agentAutomations.saveFailed": {
    message: "オートメーションを保存できませんでした。もう一度お試しください。",
    description: "Generic failure message when saving the form",
  },
  "agentAutomations.actionsLabel": {
    message: "オートメーションの操作",
    description: "Accessible label of the action button group",
  },
  "agentAutomations.runNow": {
    message: "今すぐ実行",
    description: "Button: fire the automation now",
  },
  "agentAutomations.runNowPausedHint": {
    message: "実行する前に、このオートメーションを再開してください。",
    description:
      "Tooltip on Run now while the automation is paused by the platform",
  },
  "agentAutomations.pause": {
    message: "一時停止",
    description: "Button: pause the automation",
  },
  "agentAutomations.resume": {
    message: "再開",
    description: "Button: resume the automation",
  },
  "agentAutomations.edit": {
    message: "編集",
    description: "Button: edit the automation",
  },
  "agentAutomations.editCreatorOnly": {
    message: "このオートメーションを編集できるのは、作成したメンバーのみです。",
    description: "Tooltip and notice: editing is for the creator only",
  },
  "agentAutomations.takeOver": {
    message: "引き継ぐ",
    description: "Button: an admin becomes the automation's creator",
  },
  "agentAutomations.takeOverAdminOnly": {
    message:
      "このオートメーションを引き継げるのは、作成者以外のワークスペース管理者のみです。",
    description: "Tooltip on a disabled Take over button",
  },
  "agentAutomations.takeOverTitle": {
    message: "{name} を引き継ぎますか？",
    description: "Take over confirmation title",
  },
  "agentAutomations.takeOverBody": {
    message:
      "今後の実行では、あなたのアクセス権であなたとしてセッションが開始され、テンプレートもあなたのアクセス権で再検証されます。作成者がアクセス権を失ったために一時停止されていたオートメーションは再開されます。",
    description: "Take over confirmation body",
  },
  "agentAutomations.delete": {
    message: "削除",
    description: "Button: delete the automation",
  },
  "agentAutomations.deleteTitle": {
    message: "{name} を削除しますか？",
    description: "Delete confirmation title",
  },
  "agentAutomations.deleteBody": {
    message:
      "オートメーションとその実行履歴が削除されます。すでに開始されたセッションは保持されます。",
    description: "Delete confirmation body",
  },
  "agentAutomations.settingsTitle": {
    message: "設定",
    description: "Settings card title on the detail page",
  },
  "agentAutomations.row.status": {
    message: "ステータス",
    description: "Settings row label",
  },
  "agentAutomations.row.schedule": {
    message: "スケジュール",
    description: "Settings row label",
  },
  "agentAutomations.row.nextRun": {
    message: "次回の実行",
    description: "Settings row label",
  },
  "agentAutomations.row.lastRun": {
    message: "前回の実行",
    description: "Settings row label",
  },
  "agentAutomations.row.createdBy": {
    message: "作成者",
    description: "Settings row label",
  },
  "agentAutomations.row.repo": {
    message: "リポジトリ",
    description: "Settings row label",
  },
  "agentAutomations.row.agent": {
    message: "エージェント",
    description: "Settings row label",
  },
  "agentAutomations.row.spendLimit": {
    message: "実行ごとの支出上限",
    description: "Settings row label",
  },
  "agentAutomations.row.emailNotifications": {
    message: "メール通知",
    description: "Settings row label",
  },
  "agentAutomations.row.secrets": {
    message: "シークレット",
    description: "Settings row label",
  },
  "agentAutomations.row.skills": {
    message: "スキル",
    description: "Settings row label",
  },
  "agentAutomations.taskTitle": {
    message: "タスク",
    description: "Task card title on the detail page",
  },
  "agentAutomations.noRepo": {
    message: "なし（チャットのみのセッション）",
    description: "Repository value when the template has none",
  },
  "agentAutomations.noSpendLimit": {
    message: "上限なし",
    description: "Spend limit value when the template has none",
  },
  "agentAutomations.none": {
    message: "なし",
    description: "Empty value for secrets or skills",
  },
  "agentAutomations.runsTitle": {
    message: "実行履歴",
    description: "Run history card title",
  },
  "agentAutomations.runsDescription": {
    message: "スケジュール実行と手動実行のすべてを、新しい順に表示します。",
    description: "Run history card description",
  },
  "agentAutomations.runsEmpty": {
    message: "まだ実行されていません。",
    description: "Run history empty state",
  },
  "agentAutomations.runs.fireAt": {
    message: "起動時刻",
    description: "Run table column",
  },
  "agentAutomations.runs.outcome": {
    message: "結果",
    description: "Run table column",
  },
  "agentAutomations.runs.session": {
    message: "セッション",
    description: "Run table column",
  },
  "agentAutomations.runs.missed": {
    message: "見逃し",
    description: "Run table column: ticks skipped while the scheduler was down",
  },
  "agentAutomations.runs.error": {
    message: "エラー",
    description: "Run table column",
  },
  "agentAutomations.runs.openSession": {
    message: "セッションを開く",
    description: "Link to the session a run started",
  },
  "agentAutomations.outcome.started": {
    message: "開始済み",
    description: "Run outcome: a session was started",
  },
  "agentAutomations.outcome.claimed": {
    message: "進行中",
    description: "Run outcome: claimed, not yet settled",
  },
  "agentAutomations.outcome.skipped_overlap": {
    message: "スキップ",
    description:
      "Run outcome: skipped because the previous session was still running",
  },
  "agentAutomations.outcome.failed_create": {
    message: "失敗",
    description: "Run outcome: the session could not be created",
  },
  "agentAutomations.outcome.denied": {
    message: "拒否",
    description:
      "Run outcome: the creator was no longer allowed to start the session",
  },
  "agentAutomations.outcome.unknown": {
    message: "不明",
    description: "Run outcome this version does not know",
  },
  "agentAutomations.section.details": {
    message: "詳細",
    description: "Form section title",
  },
  "agentAutomations.section.schedule": {
    message: "スケジュール",
    description: "Form section title",
  },
  "agentAutomations.section.scheduleDescription": {
    message:
      "オートメーションを実行するタイミングです。タイムゾーンによって 09:00 の意味が決まります。",
    description: "Form section description",
  },
  "agentAutomations.section.session": {
    message: "セッション",
    description: "Form section title",
  },
  "agentAutomations.section.sessionDescription": {
    message:
      "実行のたびに開始されるエージェントセッションです。オプションは、エージェントページの新規セッションと同じです。",
    description: "Form section description",
  },
  "agentAutomations.field.name": {
    message: "名前",
    description: "Form field label",
  },
  "agentAutomations.field.frequency": {
    message: "繰り返し",
    description: "Form field label",
  },
  "agentAutomations.field.minute": {
    message: "毎時の分",
    description: "Form field label",
  },
  "agentAutomations.field.at": {
    message: "時刻",
    description: "Form field label",
  },
  "agentAutomations.field.weekdays": {
    message: "曜日",
    description: "Form field label",
  },
  "agentAutomations.field.runAt": {
    message: "日時",
    description: "Form field label",
  },
  "agentAutomations.field.cron": {
    message: "Cron 式",
    description: "Form field label",
  },
  "agentAutomations.field.timezone": {
    message: "タイムゾーン",
    description: "Form field label",
  },
  "agentAutomations.field.endsOn": {
    message: "終了日（任意）",
    description: "Form field label",
  },
  "agentAutomations.field.task": {
    message: "タスク",
    description: "Form field label",
  },
  "agentAutomations.field.repo": {
    message: "リポジトリ",
    description: "Form field label",
  },
  "agentAutomations.field.agent": {
    message: "エージェント",
    description: "Form field label",
  },
  "agentAutomations.field.spendLimit": {
    message: "実行ごとの支出上限（USD）",
    description: "Form field label",
  },
  "agentAutomations.field.emailNotifications": {
    message: "メール通知",
    description: "Form field label",
  },
  "agentAutomations.field.secrets": {
    message: "シークレット",
    description: "Form field label",
  },
  "agentAutomations.field.skills": {
    message: "スキル",
    description: "Form field label",
  },
  "agentAutomations.frequency.hour": {
    message: "毎時",
    description: "Frequency option",
  },
  "agentAutomations.frequency.day": {
    message: "毎日",
    description: "Frequency option",
  },
  "agentAutomations.frequency.week": {
    message: "毎週",
    description: "Frequency option",
  },
  "agentAutomations.frequency.once": {
    message: "1 回のみ",
    description: "Frequency option",
  },
  "agentAutomations.frequency.custom": {
    message: "カスタム（cron）",
    description: "Frequency option",
  },
  "agentAutomations.emailNotifications.always": {
    message: "常に送信",
    description: "Email notification option: every run emails the creator",
  },
  "agentAutomations.emailNotifications.on_failure": {
    message: "失敗時",
    description:
      "Email notification option: only failed runs email the creator",
  },
  "agentAutomations.emailNotifications.on_success": {
    message: "成功時",
    description:
      "Email notification option: only succeeded runs email the creator",
  },
  "agentAutomations.emailNotifications.never": {
    message: "送信しない",
    description: "Email notification option: no run emails the creator",
  },
  "agentAutomations.weekday.mon": {
    message: "月",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.tue": {
    message: "火",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.wed": {
    message: "水",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.thu": {
    message: "木",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.fri": {
    message: "金",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.sat": {
    message: "土",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.sun": {
    message: "日",
    description: "Weekday toggle",
  },
  "agentAutomations.help.name": {
    message: "小文字の英字、数字、単一のハイフンのみ、64 文字以内。",
    description: "Name field help",
  },
  "agentAutomations.help.minute": {
    message: "0～59。",
    description: "Minute field help",
  },
  "agentAutomations.help.runAt": {
    message: "下で選択したタイムゾーン（{zone}）で解釈されます。",
    description: "Run-at field help",
  },
  "agentAutomations.help.cron": {
    message:
      "5 つのフィールド: 分 時 日 月 曜日。実行間隔は 5 分未満にできません。",
    description: "Cron field help",
  },
  "agentAutomations.help.endsOn": {
    message:
      "実行できる最終日です。空のままにすると、一時停止するまで実行されます。",
    description: "Ends-on field help",
  },
  "agentAutomations.help.task": {
    message:
      "実行のたびにエージェントが行う内容です。期待する成果物にも触れてください。",
    description: "Task field help",
  },
  "agentAutomations.help.repo": {
    message:
      "任意です。リポジトリを指定しない場合、各実行はチャットのみのセッションになります。実行のたびに、新しい bex-agent ブランチで作業します。",
    description: "Repository field help",
  },
  "agentAutomations.help.spendLimit": {
    message:
      "実行ごとのモデル支出の上限です。実行にプラットフォームのモデルを使う場合は必須です。",
    description: "Spend limit field help",
  },
  "agentAutomations.help.emailNotifications": {
    message:
      "メールを受け取る実行の条件です。プラットフォームによる一時停止は常に通知されます。",
    description: "Email notifications field help",
  },
  "agentAutomations.help.secrets": {
    message:
      "エージェントが環境変数として受け取るワークスペースのシークレットです。ワークスペース設定で追加してください。",
    description: "Secrets field help",
  },
  "agentAutomations.noSecrets": {
    message: "シークレットなし",
    description: "Secrets row when none are chosen",
  },
  "agentAutomations.noSkills": {
    message: "スキルなし",
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
  "agentAutomations.summary": {
    message: "実行: {schedule}",
    description: "Live summary under the schedule picker",
  },
  "agentAutomations.schedule.hour": {
    message: "毎時 {minute} 分",
    description: "Schedule in words, hourly",
  },
  "agentAutomations.schedule.day": {
    message: "毎日 {at}",
    description: "Schedule in words, daily",
  },
  "agentAutomations.schedule.week": {
    message: "毎週 {days} の {at}",
    description: "Schedule in words, weekly",
  },
  "agentAutomations.schedule.once": {
    message: "{when} に 1 回",
    description: "Schedule in words, one-off",
  },
  "agentAutomations.schedule.custom": {
    message: "Cron {cron}",
    description: "Schedule in words, cron",
  },
  "agentAutomations.schedule.until": {
    message: "{date} まで",
    description: "Schedule end date suffix",
  },
  "agentAutomations.problem.nameRequired": {
    message: "名前を入力してください。",
    description: "Form validation",
  },
  "agentAutomations.problem.nameInvalid": {
    message:
      "小文字の英字、数字、単一のハイフンのみを使用し、64 文字以内にしてください。",
    description: "Form validation",
  },
  "agentAutomations.problem.taskRequired": {
    message: "タスクを記述してください。",
    description: "Form validation",
  },
  "agentAutomations.problem.spendLimit": {
    message:
      "0 より大きく 100000 以下の金額を入力するか、空のままにしてください。",
    description: "Form validation",
  },
  "agentAutomations.problem.skills": {
    message: "スキルの引数は、4 KB 以内の 1 行にしてください。",
    description: "Form validation",
  },
  "agentAutomations.problem.timezone": {
    message: "有効なタイムゾーンを選択してください。",
    description: "Form validation",
  },
  "agentAutomations.problem.minute": {
    message: "0～59 の分を入力してください。",
    description: "Form validation",
  },
  "agentAutomations.problem.at": {
    message: "時刻を入力してください。",
    description: "Form validation",
  },
  "agentAutomations.problem.weekdays": {
    message: "曜日を 1 つ以上選択してください。",
    description: "Form validation",
  },
  "agentAutomations.problem.runAtRequired": {
    message: "日時を選択してください。",
    description: "Form validation",
  },
  "agentAutomations.problem.runAtPast": {
    message: "未来の時刻を選択してください。",
    description: "Form validation",
  },
  "agentAutomations.problem.cron": {
    message: "5 つのフィールドを入力してください: 分 時 日 月 曜日。",
    description: "Form validation",
  },
  "agentAutomations.problem.endsOn": {
    message: "有効な日付を入力してください。",
    description: "Form validation",
  },
  "agentAutomations.problem.endsOnPast": {
    message: "未来の日付を選択してください。",
    description: "Form validation",
  },
  "agentAutomations.startedBy": {
    message: "オートメーションにより開始",
    description: "Session page header note linking back to the automation",
  },
};

export default jaAgentAutomations;
