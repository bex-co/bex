import type { TranslationEntry } from "@/i18n";

const enAgentAutomations: Record<string, TranslationEntry> = {
  "agentAutomations.pageTitle": {
    message: "Automations",
    description:
      "Page title and sidebar label of the scheduled agent automations page",
  },
  "agentAutomations.detailTitle": {
    message: "Automation",
    description: "Browser tab title of one automation's page",
  },
  "agentAutomations.pageDescription": {
    message:
      "Start an agent session on a schedule. Each run uses the task and settings you save here.",
    description: "Automations list page description",
  },
  "agentAutomations.create": {
    message: "Create automation",
    description:
      "Create automation button on the list page and in the empty state",
  },
  "agentAutomations.createTitle": {
    message: "Create automation",
    description: "Heading of the create form",
  },
  "agentAutomations.editTitle": {
    message: "Edit automation",
    description: "Heading of the edit form",
  },
  "agentAutomations.createSubmit": {
    message: "Create automation",
    description: "Create form submit button",
  },
  "agentAutomations.editSubmit": {
    message: "Save changes",
    description: "Edit form submit button",
  },
  "agentAutomations.createDenied": {
    message:
      "Only members with the developer role or higher can create automations.",
    description: "Create page notice for a member who cannot create",
  },
  "agentAutomations.backToList": {
    message: "Back to automations",
    description: "Accessible label of the back link to the automations list",
  },
  "agentAutomations.backToAutomation": {
    message: "Back to the automation",
    description: "Button returning from the edit page to the automation's page",
  },
  "agentAutomations.tabMine": {
    message: "Mine",
    description: "List tab: automations the viewer created",
  },
  "agentAutomations.tabAll": {
    message: "All",
    description: "List tab: every automation in the workspace",
  },
  "agentAutomations.emptyMineTitle": {
    message: "You have no automations yet",
    description: "Empty state title on the Mine tab",
  },
  "agentAutomations.emptyAllTitle": {
    message: "This workspace has no automations yet",
    description: "Empty state title on the All tab",
  },
  "agentAutomations.emptyBody": {
    message:
      "An automation starts an agent session on a schedule, such as every weekday at 09:00, with the task, repository and limits you choose.",
    description: "Empty state explanation",
  },
  "agentAutomations.loadMore": {
    message: "Load more",
    description: "Load the next page of automations or runs",
  },
  "agentAutomations.loadFailed": {
    message: "Couldn't load automations.",
    description: "Load error",
  },
  "agentAutomations.retry": {
    message: "Retry",
    description: "Retry button on a load error",
  },
  "agentAutomations.forbidden": {
    message:
      "Only members with the developer role or higher can use automations.",
    description: "Load error for a member without access",
  },
  "agentAutomations.notFound": {
    message: "This automation doesn't exist or was deleted.",
    description: "Load error for a missing automation",
  },
  "agentAutomations.noTrigger": {
    message: "No schedule",
    description: "Row text when an automation has no schedule",
  },
  "agentAutomations.nextRun": {
    message: "Next run",
    description: "Label before the next run time",
  },
  "agentAutomations.lastRun": {
    message: "Last run",
    description: "Label before the last run time",
  },
  "agentAutomations.neverRun": {
    message: "never",
    description: "Last run value when it has not run",
  },
  "agentAutomations.lastSession": {
    message: "View session",
    description: "Link to the session the last run started",
  },
  "agentAutomations.pauseLabel": {
    message: "Pause {name}",
    description: "Accessible name of the switch that pauses an automation",
  },
  "agentAutomations.resumeLabel": {
    message: "Resume {name}",
    description: "Accessible name of the switch that resumes an automation",
  },
  "agentAutomations.pausedBadge": {
    message: "Paused",
    description: "Badge on an automation a person paused",
  },
  "agentAutomations.ended": {
    message: "Ended",
    description: "Badge on an automation with no future run",
  },
  "agentAutomations.active": {
    message: "Active",
    description: "Status of a running automation",
  },
  "agentAutomations.paused.repeated_failures": {
    message: "Paused: repeated failures",
    description: "Badge: paused by the platform after repeated failed runs",
  },
  "agentAutomations.paused.creator_lost_access": {
    message: "Paused: creator lost access",
    description:
      "Badge: paused because the creator can no longer start sessions",
  },
  "agentAutomations.paused.template_invalid": {
    message: "Paused: template invalid",
    description:
      "Badge: paused because the session template no longer validates",
  },
  "agentAutomations.paused.unknown": {
    message: "Paused by the platform",
    description: "Badge for a pause reason this version does not know",
  },
  "agentAutomations.pausedExplain.repeated_failures": {
    message:
      "This automation was paused after {failures} failed runs in a row. Check the run history, fix the cause, then resume it.",
    description: "Explanation on the detail page for a repeated-failures pause",
  },
  "agentAutomations.pausedExplain.creator_lost_access": {
    message:
      "The member who created this automation no longer has access to start sessions. A workspace admin can take it over, which resumes it.",
    description:
      "Explanation on the detail page for a creator-lost-access pause",
  },
  "agentAutomations.pausedExplain.template_invalid": {
    message:
      "The session template no longer passes validation, for example because a secret or model setting changed. Its creator can edit it, then resume it.",
    description: "Explanation on the detail page for a template-invalid pause",
  },
  "agentAutomations.pausedExplain.unknown": {
    message:
      "The platform paused this automation. Resume it once the cause is fixed.",
    description: "Explanation for a pause reason this version does not know",
  },
  "agentAutomations.pausedToast": {
    message: "Paused {name}.",
    description: "Toast after pausing an automation",
  },
  "agentAutomations.resumedToast": {
    message: "Resumed {name}.",
    description: "Toast after resuming an automation",
  },
  "agentAutomations.runStartedToast": {
    message: "{name} started a session.",
    description: "Toast after Run now started a session",
  },
  "agentAutomations.runSettledToast": {
    message: "Ran {name}: {outcome}.",
    description: "Toast after Run now ended with an outcome other than started",
  },
  "agentAutomations.tookOverToast": {
    message: "You now own {name}.",
    description: "Toast after an admin takes an automation over",
  },
  "agentAutomations.deletedToast": {
    message: "Deleted {name}.",
    description: "Toast after deleting an automation",
  },
  "agentAutomations.createdToast": {
    message: "Created {name}.",
    description: "Toast after creating an automation",
  },
  "agentAutomations.updatedToast": {
    message: "Saved {name}.",
    description: "Toast after editing an automation",
  },
  "agentAutomations.actionFailed": {
    message: "That didn't work. Try again.",
    description: "Generic failure toast for an automation action",
  },
  "agentAutomations.saveFailed": {
    message: "Couldn't save the automation. Try again.",
    description: "Generic failure message when saving the form",
  },
  "agentAutomations.actionsLabel": {
    message: "Automation actions",
    description: "Accessible label of the action button group",
  },
  "agentAutomations.runNow": {
    message: "Run now",
    description: "Button: fire the automation now",
  },
  "agentAutomations.runNowPausedHint": {
    message: "Resume this automation before running it.",
    description:
      "Tooltip on Run now while the automation is paused by the platform",
  },
  "agentAutomations.pause": {
    message: "Pause",
    description: "Button: pause the automation",
  },
  "agentAutomations.resume": {
    message: "Resume",
    description: "Button: resume the automation",
  },
  "agentAutomations.edit": {
    message: "Edit",
    description: "Button: edit the automation",
  },
  "agentAutomations.editCreatorOnly": {
    message: "Only the member who created this automation can edit it.",
    description: "Tooltip and notice: editing is for the creator only",
  },
  "agentAutomations.takeOver": {
    message: "Take over",
    description: "Button: an admin becomes the automation's creator",
  },
  "agentAutomations.takeOverAdminOnly": {
    message:
      "Only a workspace admin who didn't create this automation can take it over.",
    description: "Tooltip on a disabled Take over button",
  },
  "agentAutomations.takeOverTitle": {
    message: "Take over {name}?",
    description: "Take over confirmation title",
  },
  "agentAutomations.takeOverBody": {
    message:
      "Future runs will start sessions as you, with your access, and the template is checked again under your access. An automation paused because its creator lost access resumes.",
    description: "Take over confirmation body",
  },
  "agentAutomations.delete": {
    message: "Delete",
    description: "Button: delete the automation",
  },
  "agentAutomations.deleteTitle": {
    message: "Delete {name}?",
    description: "Delete confirmation title",
  },
  "agentAutomations.deleteBody": {
    message:
      "The automation and its run history are deleted. Sessions it already started are kept.",
    description: "Delete confirmation body",
  },
  "agentAutomations.settingsTitle": {
    message: "Settings",
    description: "Settings card title on the detail page",
  },
  "agentAutomations.row.status": {
    message: "Status",
    description: "Settings row label",
  },
  "agentAutomations.row.schedule": {
    message: "Schedule",
    description: "Settings row label",
  },
  "agentAutomations.row.nextRun": {
    message: "Next run",
    description: "Settings row label",
  },
  "agentAutomations.row.lastRun": {
    message: "Last run",
    description: "Settings row label",
  },
  "agentAutomations.row.createdBy": {
    message: "Created by",
    description: "Settings row label",
  },
  "agentAutomations.row.repo": {
    message: "Repository",
    description: "Settings row label",
  },
  "agentAutomations.row.agent": {
    message: "Agent",
    description: "Settings row label",
  },
  "agentAutomations.row.spendLimit": {
    message: "Spend limit per run",
    description: "Settings row label",
  },
  "agentAutomations.row.secrets": {
    message: "Secrets",
    description: "Settings row label",
  },
  "agentAutomations.row.skills": {
    message: "Skills",
    description: "Settings row label",
  },
  "agentAutomations.taskTitle": {
    message: "Task",
    description: "Task card title on the detail page",
  },
  "agentAutomations.noRepo": {
    message: "None (chat-only session)",
    description: "Repository value when the template has none",
  },
  "agentAutomations.noSpendLimit": {
    message: "No limit",
    description: "Spend limit value when the template has none",
  },
  "agentAutomations.none": {
    message: "None",
    description: "Empty value for secrets or skills",
  },
  "agentAutomations.runsTitle": {
    message: "Run history",
    description: "Run history card title",
  },
  "agentAutomations.runsDescription": {
    message: "Every scheduled and manual run, newest first.",
    description: "Run history card description",
  },
  "agentAutomations.runsEmpty": {
    message: "No runs yet.",
    description: "Run history empty state",
  },
  "agentAutomations.runs.fireAt": {
    message: "Fire time",
    description: "Run table column",
  },
  "agentAutomations.runs.outcome": {
    message: "Outcome",
    description: "Run table column",
  },
  "agentAutomations.runs.session": {
    message: "Session",
    description: "Run table column",
  },
  "agentAutomations.runs.missed": {
    message: "Missed",
    description: "Run table column: ticks skipped while the scheduler was down",
  },
  "agentAutomations.runs.error": {
    message: "Error",
    description: "Run table column",
  },
  "agentAutomations.runs.openSession": {
    message: "Open session",
    description: "Link to the session a run started",
  },
  "agentAutomations.outcome.started": {
    message: "Started",
    description: "Run outcome: a session was started",
  },
  "agentAutomations.outcome.claimed": {
    message: "In progress",
    description: "Run outcome: claimed, not yet settled",
  },
  "agentAutomations.outcome.skipped_overlap": {
    message: "Skipped",
    description:
      "Run outcome: skipped because the previous session was still running",
  },
  "agentAutomations.outcome.failed_create": {
    message: "Failed",
    description: "Run outcome: the session could not be created",
  },
  "agentAutomations.outcome.denied": {
    message: "Denied",
    description:
      "Run outcome: the creator was no longer allowed to start the session",
  },
  "agentAutomations.outcome.unknown": {
    message: "Unknown",
    description: "Run outcome this version does not know",
  },
  "agentAutomations.section.details": {
    message: "Details",
    description: "Form section title",
  },
  "agentAutomations.section.schedule": {
    message: "Schedule",
    description: "Form section title",
  },
  "agentAutomations.section.scheduleDescription": {
    message:
      "When the automation runs. The time zone decides what 09:00 means.",
    description: "Form section description",
  },
  "agentAutomations.section.session": {
    message: "Session",
    description: "Form section title",
  },
  "agentAutomations.section.sessionDescription": {
    message:
      "The agent session each run starts, with the same options as a new session on the Agents page.",
    description: "Form section description",
  },
  "agentAutomations.field.name": {
    message: "Name",
    description: "Form field label",
  },
  "agentAutomations.field.frequency": {
    message: "Repeat",
    description: "Form field label",
  },
  "agentAutomations.field.minute": {
    message: "Minute past the hour",
    description: "Form field label",
  },
  "agentAutomations.field.at": {
    message: "Time",
    description: "Form field label",
  },
  "agentAutomations.field.weekdays": {
    message: "Days",
    description: "Form field label",
  },
  "agentAutomations.field.runAt": {
    message: "Date and time",
    description: "Form field label",
  },
  "agentAutomations.field.cron": {
    message: "Cron expression",
    description: "Form field label",
  },
  "agentAutomations.field.timezone": {
    message: "Time zone",
    description: "Form field label",
  },
  "agentAutomations.field.endsOn": {
    message: "Ends on (optional)",
    description: "Form field label",
  },
  "agentAutomations.field.task": {
    message: "Task",
    description: "Form field label",
  },
  "agentAutomations.field.repo": {
    message: "Repository",
    description: "Form field label",
  },
  "agentAutomations.field.agent": {
    message: "Agent",
    description: "Form field label",
  },
  "agentAutomations.field.spendLimit": {
    message: "Spend limit per run (USD)",
    description: "Form field label",
  },
  "agentAutomations.field.secrets": {
    message: "Secrets",
    description: "Form field label",
  },
  "agentAutomations.field.skills": {
    message: "Skills",
    description: "Form field label",
  },
  "agentAutomations.frequency.hour": {
    message: "Every hour",
    description: "Frequency option",
  },
  "agentAutomations.frequency.day": {
    message: "Every day",
    description: "Frequency option",
  },
  "agentAutomations.frequency.week": {
    message: "Every week",
    description: "Frequency option",
  },
  "agentAutomations.frequency.once": {
    message: "Once",
    description: "Frequency option",
  },
  "agentAutomations.frequency.custom": {
    message: "Custom (cron)",
    description: "Frequency option",
  },
  "agentAutomations.weekday.mon": {
    message: "Mon",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.tue": {
    message: "Tue",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.wed": {
    message: "Wed",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.thu": {
    message: "Thu",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.fri": {
    message: "Fri",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.sat": {
    message: "Sat",
    description: "Weekday toggle",
  },
  "agentAutomations.weekday.sun": {
    message: "Sun",
    description: "Weekday toggle",
  },
  "agentAutomations.help.name": {
    message:
      "Lowercase letters, digits and single hyphens, up to 64 characters.",
    description: "Name field help",
  },
  "agentAutomations.help.minute": {
    message: "0 to 59.",
    description: "Minute field help",
  },
  "agentAutomations.help.runAt": {
    message: "Read in the time zone chosen below ({zone}).",
    description: "Run-at field help",
  },
  "agentAutomations.help.cron": {
    message:
      "Five fields: minute hour day-of-month month day-of-week. Runs can't be closer than 5 minutes apart.",
    description: "Cron field help",
  },
  "agentAutomations.help.endsOn": {
    message: "The last day it can run. Leave empty to run until you pause it.",
    description: "Ends-on field help",
  },
  "agentAutomations.help.task": {
    message:
      "What the agent should do each run. Mention the output you expect.",
    description: "Task field help",
  },
  "agentAutomations.help.repo": {
    message:
      "Optional. Without a repository each run is a chat-only session. Each run works on a fresh bex-agent branch.",
    description: "Repository field help",
  },
  "agentAutomations.help.spendLimit": {
    message:
      "A ceiling on model spend for each run. Required when runs use the platform model.",
    description: "Spend limit field help",
  },
  "agentAutomations.help.secrets": {
    message:
      "Workspace secrets the agent gets as environment variables. Add them in Workspace settings.",
    description: "Secrets field help",
  },
  "agentAutomations.noSecrets": {
    message: "No secrets",
    description: "Secrets row when none are chosen",
  },
  "agentAutomations.noSkills": {
    message: "No skills",
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
    message: "Runs: {schedule}",
    description: "Live summary under the schedule picker",
  },
  "agentAutomations.schedule.hour": {
    message: "Every hour at :{minute}",
    description: "Schedule in words, hourly",
  },
  "agentAutomations.schedule.day": {
    message: "Every day at {at}",
    description: "Schedule in words, daily",
  },
  "agentAutomations.schedule.week": {
    message: "Every {days} at {at}",
    description: "Schedule in words, weekly",
  },
  "agentAutomations.schedule.once": {
    message: "Once on {when}",
    description: "Schedule in words, one-off",
  },
  "agentAutomations.schedule.custom": {
    message: "Cron {cron}",
    description: "Schedule in words, cron",
  },
  "agentAutomations.schedule.until": {
    message: "until {date}",
    description: "Schedule end date suffix",
  },
  "agentAutomations.problem.nameRequired": {
    message: "Enter a name.",
    description: "Form validation",
  },
  "agentAutomations.problem.nameInvalid": {
    message:
      "Use lowercase letters, digits and single hyphens, up to 64 characters.",
    description: "Form validation",
  },
  "agentAutomations.problem.taskRequired": {
    message: "Describe the task.",
    description: "Form validation",
  },
  "agentAutomations.problem.spendLimit": {
    message: "Enter an amount above 0 and up to 100000, or leave it empty.",
    description: "Form validation",
  },
  "agentAutomations.problem.skills": {
    message: "A skill's arguments must be one line of at most 4 KB.",
    description: "Form validation",
  },
  "agentAutomations.problem.timezone": {
    message: "Choose a valid time zone.",
    description: "Form validation",
  },
  "agentAutomations.problem.minute": {
    message: "Enter a minute from 0 to 59.",
    description: "Form validation",
  },
  "agentAutomations.problem.at": {
    message: "Enter a time.",
    description: "Form validation",
  },
  "agentAutomations.problem.weekdays": {
    message: "Choose at least one day.",
    description: "Form validation",
  },
  "agentAutomations.problem.runAtRequired": {
    message: "Choose a date and time.",
    description: "Form validation",
  },
  "agentAutomations.problem.runAtPast": {
    message: "Choose a time in the future.",
    description: "Form validation",
  },
  "agentAutomations.problem.cron": {
    message: "Enter five fields: minute hour day-of-month month day-of-week.",
    description: "Form validation",
  },
  "agentAutomations.problem.endsOn": {
    message: "Enter a valid date.",
    description: "Form validation",
  },
  "agentAutomations.problem.endsOnPast": {
    message: "Choose a date in the future.",
    description: "Form validation",
  },
  "agentAutomations.startedBy": {
    message: "Started by automation",
    description: "Session page header note linking back to the automation",
  },
};

export default enAgentAutomations;
