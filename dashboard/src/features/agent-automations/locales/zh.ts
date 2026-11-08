import type { TranslationEntry } from "@/i18n";

const zhAgentAutomations: Record<string, TranslationEntry> = {
  "agentAutomations.pageTitle": {
    message: "自动化",
    description:
      "Page title and sidebar label of the scheduled agent automations page",
  },
  "agentAutomations.detailTitle": {
    message: "自动化详情",
    description: "Browser tab title of one automation's page",
  },
  "agentAutomations.pageDescription": {
    message: "按计划自动启动智能体会话。每次运行都使用你在此保存的任务和设置。",
    description: "Automations list page description",
  },
  "agentAutomations.create": {
    message: "创建自动化",
    description:
      "Create automation button on the list page and in the empty state",
  },
  "agentAutomations.createTitle": {
    message: "创建自动化",
    description: "Heading of the create form",
  },
  "agentAutomations.editTitle": {
    message: "编辑自动化",
    description: "Heading of the edit form",
  },
  "agentAutomations.createSubmit": {
    message: "创建自动化",
    description: "Create form submit button",
  },
  "agentAutomations.editSubmit": {
    message: "保存更改",
    description: "Edit form submit button",
  },
  "agentAutomations.createDenied": {
    message: "只有开发者及以上角色的成员才能创建自动化。",
    description: "Create page notice for a member who cannot create",
  },
  "agentAutomations.backToList": {
    message: "返回自动化列表",
    description: "Accessible label of the back link to the automations list",
  },
  "agentAutomations.backToAutomation": {
    message: "返回该自动化",
    description: "Button returning from the edit page to the automation's page",
  },
  "agentAutomations.tabMine": {
    message: "我的",
    description: "List tab: automations the viewer created",
  },
  "agentAutomations.tabAll": {
    message: "全部",
    description: "List tab: every automation in the workspace",
  },
  "agentAutomations.emptyMineTitle": {
    message: "你还没有自动化",
    description: "Empty state title on the Mine tab",
  },
  "agentAutomations.emptyAllTitle": {
    message: "此工作区还没有自动化",
    description: "Empty state title on the All tab",
  },
  "agentAutomations.emptyBody": {
    message:
      "自动化会按计划（例如每个工作日 09:00）启动智能体会话，并使用你选择的任务、仓库和限制。",
    description: "Empty state explanation",
  },
  "agentAutomations.loadMore": {
    message: "加载更多",
    description: "Load the next page of automations or runs",
  },
  "agentAutomations.loadFailed": {
    message: "无法加载自动化。",
    description: "Load error",
  },
  "agentAutomations.retry": {
    message: "重试",
    description: "Retry button on a load error",
  },
  "agentAutomations.forbidden": {
    message: "只有开发者及以上角色的成员才能使用自动化。",
    description: "Load error for a member without access",
  },
  "agentAutomations.notFound": {
    message: "此自动化不存在或已被删除。",
    description: "Load error for a missing automation",
  },
  "agentAutomations.noTrigger": {
    message: "无计划",
    description: "Row text when an automation has no schedule",
  },
  "agentAutomations.nextRun": {
    message: "下次运行",
    description: "Label before the next run time",
  },
  "agentAutomations.lastRun": {
    message: "上次运行",
    description: "Label before the last run time",
  },
  "agentAutomations.neverRun": {
    message: "从未",
    description: "Last run value when it has not run",
  },
  "agentAutomations.lastSession": {
    message: "查看会话",
    description: "Link to the session the last run started",
  },
  "agentAutomations.pauseLabel": {
    message: "暂停 {name}",
    description: "Accessible name of the switch that pauses an automation",
  },
  "agentAutomations.resumeLabel": {
    message: "恢复 {name}",
    description: "Accessible name of the switch that resumes an automation",
  },
  "agentAutomations.pausedBadge": {
    message: "已暂停",
    description: "Badge on an automation a person paused",
  },
  "agentAutomations.ended": {
    message: "已结束",
    description: "Badge on an automation with no future run",
  },
  "agentAutomations.active": {
    message: "运行中",
    description: "Status of a running automation",
  },
  "agentAutomations.paused.repeated_failures": {
    message: "已暂停：多次失败",
    description: "Badge: paused by the platform after repeated failed runs",
  },
  "agentAutomations.paused.creator_lost_access": {
    message: "已暂停：创建者失去权限",
    description:
      "Badge: paused because the creator can no longer start sessions",
  },
  "agentAutomations.paused.template_invalid": {
    message: "已暂停：模板无效",
    description:
      "Badge: paused because the session template no longer validates",
  },
  "agentAutomations.paused.unknown": {
    message: "已被平台暂停",
    description: "Badge for a pause reason this version does not know",
  },
  "agentAutomations.pausedExplain.repeated_failures": {
    message:
      "此自动化在连续 {failures} 次运行失败后被暂停。请查看运行历史，修复原因后再恢复。",
    description: "Explanation on the detail page for a repeated-failures pause",
  },
  "agentAutomations.pausedExplain.creator_lost_access": {
    message:
      "创建此自动化的成员已无权启动会话。工作区管理员可以接管它，接管后会自动恢复。",
    description:
      "Explanation on the detail page for a creator-lost-access pause",
  },
  "agentAutomations.pausedExplain.template_invalid": {
    message:
      "会话模板不再通过校验，例如密钥或模型设置已更改。创建者可以编辑后再恢复。",
    description: "Explanation on the detail page for a template-invalid pause",
  },
  "agentAutomations.pausedExplain.unknown": {
    message: "平台已暂停此自动化。原因修复后请恢复。",
    description: "Explanation for a pause reason this version does not know",
  },
  "agentAutomations.pausedToast": {
    message: "已暂停 {name}。",
    description: "Toast after pausing an automation",
  },
  "agentAutomations.resumedToast": {
    message: "已恢复 {name}。",
    description: "Toast after resuming an automation",
  },
  "agentAutomations.runStartedToast": {
    message: "{name} 已启动一个会话。",
    description: "Toast after Run now started a session",
  },
  "agentAutomations.runSettledToast": {
    message: "已运行 {name}：{outcome}。",
    description: "Toast after Run now ended with an outcome other than started",
  },
  "agentAutomations.tookOverToast": {
    message: "你现在是 {name} 的所有者。",
    description: "Toast after an admin takes an automation over",
  },
  "agentAutomations.deletedToast": {
    message: "已删除 {name}。",
    description: "Toast after deleting an automation",
  },
  "agentAutomations.createdToast": {
    message: "已创建 {name}。",
    description: "Toast after creating an automation",
  },
  "agentAutomations.updatedToast": {
    message: "已保存 {name}。",
    description: "Toast after editing an automation",
  },
  "agentAutomations.actionFailed": {
    message: "操作未成功，请重试。",
    description: "Generic failure toast for an automation action",
  },
  "agentAutomations.saveFailed": {
    message: "无法保存自动化，请重试。",
    description: "Generic failure message when saving the form",
  },
  "agentAutomations.actionsLabel": {
    message: "自动化操作",
    description: "Accessible label of the action button group",
  },
  "agentAutomations.runNow": {
    message: "立即运行",
    description: "Button: fire the automation now",
  },
  "agentAutomations.runNowPausedHint": {
    message: "请先恢复此自动化，再运行。",
    description:
      "Tooltip on Run now while the automation is paused by the platform",
  },
  "agentAutomations.pause": {
    message: "暂停",
    description: "Button: pause the automation",
  },
  "agentAutomations.resume": {
    message: "恢复",
    description: "Button: resume the automation",
  },
  "agentAutomations.edit": {
    message: "编辑",
    description: "Button: edit the automation",
  },
  "agentAutomations.editCreatorOnly": {
    message: "只有创建此自动化的成员才能编辑它。",
    description: "Tooltip and notice: editing is for the creator only",
  },
  "agentAutomations.takeOver": {
    message: "接管",
    description: "Button: an admin becomes the automation's creator",
  },
  "agentAutomations.takeOverAdminOnly": {
    message: "只有未创建此自动化的工作区管理员才能接管它。",
    description: "Tooltip on a disabled Take over button",
  },
  "agentAutomations.takeOverTitle": {
    message: "接管 {name}？",
    description: "Take over confirmation title",
  },
  "agentAutomations.takeOverBody": {
    message:
      "之后的运行将以你的身份和权限启动会话，并按你的权限重新校验模板。因创建者失去权限而暂停的自动化会恢复运行。",
    description: "Take over confirmation body",
  },
  "agentAutomations.delete": {
    message: "删除",
    description: "Button: delete the automation",
  },
  "agentAutomations.deleteTitle": {
    message: "删除 {name}？",
    description: "Delete confirmation title",
  },
  "agentAutomations.deleteBody": {
    message: "将删除该自动化及其运行历史。它已启动的会话会保留。",
    description: "Delete confirmation body",
  },
  "agentAutomations.settingsTitle": {
    message: "设置",
    description: "Settings card title on the detail page",
  },
  "agentAutomations.row.status": {
    message: "状态",
    description: "Settings row label",
  },
  "agentAutomations.row.schedule": {
    message: "计划",
    description: "Settings row label",
  },
  "agentAutomations.row.nextRun": {
    message: "下次运行",
    description: "Settings row label",
  },
  "agentAutomations.row.lastRun": {
    message: "上次运行",
    description: "Settings row label",
  },
  "agentAutomations.row.createdBy": {
    message: "创建者",
    description: "Settings row label",
  },
  "agentAutomations.row.repo": {
    message: "仓库",
    description: "Settings row label",
  },
  "agentAutomations.row.agent": {
    message: "智能体",
    description: "Settings row label",
  },
  "agentAutomations.row.spendLimit": {
    message: "每次运行的花费上限",
    description: "Settings row label",
  },
  "agentAutomations.row.secrets": {
    message: "密钥",
    description: "Settings row label",
  },
  "agentAutomations.row.skills": {
    message: "技能",
    description: "Settings row label",
  },
  "agentAutomations.taskTitle": {
    message: "任务",
    description: "Task card title on the detail page",
  },
  "agentAutomations.noRepo": {
    message: "无（仅对话会话）",
    description: "Repository value when the template has none",
  },
  "agentAutomations.noSpendLimit": {
    message: "无限制",
    description: "Spend limit value when the template has none",
  },
  "agentAutomations.none": {
    message: "无",
    description: "Empty value for secrets or skills",
  },
  "agentAutomations.runsTitle": {
    message: "运行历史",
    description: "Run history card title",
  },
  "agentAutomations.runsDescription": {
    message: "每一次计划运行和手动运行，最新的在前。",
    description: "Run history card description",
  },
  "agentAutomations.runsEmpty": {
    message: "暂无运行。",
    description: "Run history empty state",
  },
  "agentAutomations.runs.fireAt": {
    message: "触发时间",
    description: "Run table column",
  },
  "agentAutomations.runs.outcome": {
    message: "结果",
    description: "Run table column",
  },
  "agentAutomations.runs.session": {
    message: "会话",
    description: "Run table column",
  },
  "agentAutomations.runs.missed": {
    message: "错过",
    description: "Run table column: ticks skipped while the scheduler was down",
  },
  "agentAutomations.runs.error": {
    message: "错误",
    description: "Run table column",
  },
  "agentAutomations.runs.openSession": {
    message: "打开会话",
    description: "Link to the session a run started",
  },
  "agentAutomations.outcome.started": {
    message: "已启动",
    description: "Run outcome: a session was started",
  },
  "agentAutomations.outcome.claimed": {
    message: "进行中",
    description: "Run outcome: claimed, not yet settled",
  },
  "agentAutomations.outcome.skipped_overlap": {
    message: "已跳过",
    description:
      "Run outcome: skipped because the previous session was still running",
  },
  "agentAutomations.outcome.failed_create": {
    message: "失败",
    description: "Run outcome: the session could not be created",
  },
  "agentAutomations.outcome.denied": {
    message: "被拒绝",
    description:
      "Run outcome: the creator was no longer allowed to start the session",
  },
  "agentAutomations.outcome.unknown": {
    message: "未知",
    description: "Run outcome this version does not know",
  },
  "agentAutomations.section.details": {
    message: "基本信息",
    description: "Form section title",
  },
  "agentAutomations.section.schedule": {
    message: "计划",
    description: "Form section title",
  },
  "agentAutomations.section.scheduleDescription": {
    message: "自动化何时运行。时区决定 09:00 指哪个时刻。",
    description: "Form section description",
  },
  "agentAutomations.section.session": {
    message: "会话",
    description: "Form section title",
  },
  "agentAutomations.section.sessionDescription": {
    message: "每次运行启动的智能体会话，选项与智能体页面的新会话相同。",
    description: "Form section description",
  },
  "agentAutomations.field.name": {
    message: "名称",
    description: "Form field label",
  },
  "agentAutomations.field.frequency": {
    message: "重复",
    description: "Form field label",
  },
  "agentAutomations.field.minute": {
    message: "每小时的第几分钟",
    description: "Form field label",
  },
  "agentAutomations.field.at": {
    message: "时间",
    description: "Form field label",
  },
  "agentAutomations.field.weekdays": {
    message: "星期",
    description: "Form field label",
  },
  "agentAutomations.field.runAt": {
    message: "日期和时间",
    description: "Form field label",
  },
  "agentAutomations.field.cron": {
    message: "Cron 表达式",
    description: "Form field label",
  },
  "agentAutomations.field.timezone": {
    message: "时区",
    description: "Form field label",
  },
  "agentAutomations.field.endsOn": {
    message: "结束日期（可选）",
    description: "Form field label",
  },
  "agentAutomations.field.task": {
    message: "任务",
    description: "Form field label",
  },
  "agentAutomations.field.repo": {
    message: "仓库",
    description: "Form field label",
  },
  "agentAutomations.field.agent": {
    message: "智能体",
    description: "Form field label",
  },
  "agentAutomations.field.spendLimit": {
    message: "每次运行的花费上限（美元）",
    description: "Form field label",
  },
  "agentAutomations.field.secrets": {
    message: "密钥",
    description: "Form field label",
  },
  "agentAutomations.field.skills": {
    message: "技能",
    description: "Form field label",
  },
  "agentAutomations.frequency.hour": {
    message: "每小时",
    description: "Frequency option",
  },
  "agentAutomations.frequency.day": {
    message: "每天",
    description: "Frequency option",
  },
  "agentAutomations.frequency.week": {
    message: "每周",
    description: "Frequency option",
  },
  "agentAutomations.frequency.once": {
    message: "仅一次",
    description: "Frequency option",
  },
  "agentAutomations.frequency.custom": {
    message: "自定义（cron）",
    description: "Frequency option",
  },
  "agentAutomations.weekday.mon": {
    message: "周一",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.tue": {
    message: "周二",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.wed": {
    message: "周三",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.thu": {
    message: "周四",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.fri": {
    message: "周五",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.sat": {
    message: "周六",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.sun": {
    message: "周日",
    description: "Weekday toggle",
  },
  "agentAutomations.help.name": {
    message: "小写字母、数字和单个连字符，最多 64 个字符。",
    description: "Name field help",
  },
  "agentAutomations.help.minute": {
    message: "0 到 59。",
    description: "Minute field help",
  },
  "agentAutomations.help.runAt": {
    message: "按下方所选时区（{zone}）解读。",
    description: "Run-at field help",
  },
  "agentAutomations.help.cron": {
    message: "五个字段：分 时 日 月 星期。两次运行的间隔不能少于 5 分钟。",
    description: "Cron field help",
  },
  "agentAutomations.help.endsOn": {
    message: "最后一个可运行的日期。留空则一直运行，直到你暂停。",
    description: "Ends-on field help",
  },
  "agentAutomations.help.task": {
    message: "智能体每次运行要做的事。请说明期望的产出。",
    description: "Task field help",
  },
  "agentAutomations.help.repo": {
    message:
      "可选。不选仓库时每次运行都是仅对话会话。每次运行都会使用新的 bex-agent 分支。",
    description: "Repository field help",
  },
  "agentAutomations.help.spendLimit": {
    message: "每次运行的模型花费上限。使用平台模型时必填。",
    description: "Spend limit field help",
  },
  "agentAutomations.help.secrets": {
    message: "智能体以环境变量形式获得的工作区密钥。请在工作区设置中添加。",
    description: "Secrets field help",
  },
  "agentAutomations.noSecrets": {
    message: "无密钥",
    description: "Secrets row when none are chosen",
  },
  "agentAutomations.noSkills": {
    message: "无技能",
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
    message: "运行计划：{schedule}",
    description: "Live summary under the schedule picker",
  },
  "agentAutomations.schedule.hour": {
    message: "每小时第 {minute} 分",
    description: "Schedule in words, hourly",
  },
  "agentAutomations.schedule.day": {
    message: "每天 {at}",
    description: "Schedule in words, daily",
  },
  "agentAutomations.schedule.week": {
    message: "每周 {days} {at}",
    description: "Schedule in words, weekly",
  },
  "agentAutomations.schedule.once": {
    message: "仅一次，{when}",
    description: "Schedule in words, one-off",
  },
  "agentAutomations.schedule.custom": {
    message: "Cron {cron}",
    description: "Schedule in words, cron",
  },
  "agentAutomations.schedule.until": {
    message: "截至 {date}",
    description: "Schedule end date suffix",
  },
  "agentAutomations.problem.nameRequired": {
    message: "请输入名称。",
    description: "Form validation",
  },
  "agentAutomations.problem.nameInvalid": {
    message: "请使用小写字母、数字和单个连字符，最多 64 个字符。",
    description: "Form validation",
  },
  "agentAutomations.problem.taskRequired": {
    message: "请描述任务。",
    description: "Form validation",
  },
  "agentAutomations.problem.spendLimit": {
    message: "请输入大于 0 且不超过 100000 的金额，或留空。",
    description: "Form validation",
  },
  "agentAutomations.problem.skills": {
    message: "技能参数必须是单行，且不超过 4 KB。",
    description: "Form validation",
  },
  "agentAutomations.problem.timezone": {
    message: "请选择有效的时区。",
    description: "Form validation",
  },
  "agentAutomations.problem.minute": {
    message: "请输入 0 到 59 的分钟数。",
    description: "Form validation",
  },
  "agentAutomations.problem.at": {
    message: "请输入时间。",
    description: "Form validation",
  },
  "agentAutomations.problem.weekdays": {
    message: "请至少选择一天。",
    description: "Form validation",
  },
  "agentAutomations.problem.runAtRequired": {
    message: "请选择日期和时间。",
    description: "Form validation",
  },
  "agentAutomations.problem.runAtPast": {
    message: "请选择未来的时间。",
    description: "Form validation",
  },
  "agentAutomations.problem.cron": {
    message: "请输入五个字段：分 时 日 月 星期。",
    description: "Form validation",
  },
  "agentAutomations.problem.endsOn": {
    message: "请输入有效日期。",
    description: "Form validation",
  },
  "agentAutomations.problem.endsOnPast": {
    message: "请选择未来的日期。",
    description: "Form validation",
  },
  "agentAutomations.startedBy": {
    message: "由自动化启动",
    description: "Session page header note linking back to the automation",
  },
};

export default zhAgentAutomations;
