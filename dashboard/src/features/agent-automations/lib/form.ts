// The automation form's pure machinery (ADR101 D9): the draft values, the
// client checks that mirror the server's shape rules, the API input, and the
// mapping of the server's named errors onto the field they concern.

import { CombinedGraphQLErrors } from "@apollo/client/errors";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import type {
  AgentAutomationInput,
  AgentAutomationTemplateInput,
} from "@/graphql/definitions";
import { SKILL_NAME_PATTERN } from "@/features/agent-environment/lib/skills";
import {
  AGENT_OPTIONS,
  type AgentOption,
} from "@/features/agent-sessions/lib/composer";
import {
  composerSkillsInput,
  composerSkillsValid,
  type ComposerSkill,
} from "@/features/agent-sessions/lib/composer-skills";
import { parseSpendLimit } from "@/features/agent-sessions/lib/spend-limit";
import {
  DEFAULT_EMAIL_NOTIFICATION,
  isEmailNotification,
  type EmailNotification,
} from "@/features/agent-automations/lib/display";
import {
  defaultScheduleDraft,
  draftToScheduleInput,
  scheduleProblems,
  scheduleToDraft,
  type ScheduleDraft,
  type ScheduleProblems,
} from "@/features/agent-automations/lib/schedule";
import type { AgentAutomationFieldsFragment } from "@/graphql/definitions";

export const MAX_NAME_LENGTH = 64;

export interface AutomationFormValues {
  name: string;
  task: string;
  repo: string;
  agent: AgentOption;
  /** Typed as-is; blank means no ceiling (the server requires one on the platform model). */
  spendLimit: string;
  secretNames: string[];
  skills: ComposerSkill[];
  schedule: ScheduleDraft;
  /** Which run outcomes email the creator (w9/m343). */
  emailNotifications: EmailNotification;
  /**
   * Template fields the form does not edit but must not drop on an update
   * (the template is replaced whole): branch, model, title, tags, output
   * contract, egress.
   */
  carried: Omit<
    AgentAutomationTemplateInput,
    "task" | "repo" | "agentConfig" | "maxSpendUSD" | "secretNames" | "skills"
  > & { model?: string | null; agentTemplate?: string | null };
}

export function emptyFormValues(timezone?: string): AutomationFormValues {
  return {
    name: "",
    task: "",
    repo: "",
    agent: "claude",
    spendLimit: "",
    secretNames: [],
    skills: [],
    schedule: defaultScheduleDraft(timezone),
    emailNotifications: DEFAULT_EMAIL_NOTIFICATION,
    carried: {},
  };
}

function isAgentOption(value: string): value is AgentOption {
  return (AGENT_OPTIONS as readonly string[]).includes(value);
}

/** The edit form's starting values for a stored automation. */
export function valuesFromAutomation(
  automation: AgentAutomationFieldsFragment,
): AutomationFormValues {
  const tpl = automation.template;
  const schedule = automation.triggers[0]?.schedule;
  return {
    name: automation.name,
    task: tpl.task,
    repo: tpl.repo ?? "",
    agent: isAgentOption(tpl.agentConfig.agent)
      ? tpl.agentConfig.agent
      : "claude",
    spendLimit: tpl.maxSpendUSD != null ? String(tpl.maxSpendUSD) : "",
    secretNames: tpl.secretNames ?? [],
    skills: (tpl.skills ?? []).map((s) => ({
      name: s.name,
      ...(s.arguments ? { arguments: s.arguments } : {}),
    })),
    schedule: schedule ? scheduleToDraft(schedule) : defaultScheduleDraft(),
    emailNotifications: isEmailNotification(automation.notifications.email)
      ? automation.notifications.email
      : DEFAULT_EMAIL_NOTIFICATION,
    carried: {
      branch: tpl.branch ?? undefined,
      title: tpl.title ?? undefined,
      tags: tpl.tags ?? undefined,
      egressAllowlist: tpl.egressAllowlist ?? undefined,
      outputSchema: tpl.outputSchema ?? undefined,
      outputRequired: tpl.outputRequired ?? undefined,
      model: tpl.agentConfig.model ?? undefined,
      agentTemplate: tpl.agentConfig.template ?? undefined,
    },
  };
}

export type FormField =
  | "name"
  | "task"
  | "repo"
  | "spendLimit"
  | "emailNotifications"
  | "skills"
  | "schedule";

export interface FormProblems {
  fields: Partial<Record<FormField, string>>;
  schedule: ScheduleProblems;
}

/** The first problem's field in on-screen order, for focus; null when valid. */
export function hasProblems(p: FormProblems): boolean {
  return Object.keys(p.fields).length > 0 || Object.keys(p.schedule).length > 0;
}

/** Client-side checks, as locale keys. The server stays the authority. */
export function validateForm(
  values: AutomationFormValues,
  now: number = Date.now(),
): FormProblems {
  const fields: FormProblems["fields"] = {};
  const name = values.name.trim();
  if (name === "") fields.name = "agentAutomations.problem.nameRequired";
  else if (name.length > MAX_NAME_LENGTH || !SKILL_NAME_PATTERN.test(name)) {
    fields.name = "agentAutomations.problem.nameInvalid";
  }
  if (values.task.trim() === "") {
    fields.task = "agentAutomations.problem.taskRequired";
  }
  if (!parseSpendLimit(values.spendLimit).ok) {
    fields.spendLimit = "agentAutomations.problem.spendLimit";
  }
  if (!composerSkillsValid(values.skills)) {
    fields.skills = "agentAutomations.problem.skills";
  }
  return { fields, schedule: scheduleProblems(values.schedule, now) };
}

/** The create/update input for valid values. */
export function toAutomationInput(
  values: AutomationFormValues,
  options: { ifVersion?: number } = {},
): AgentAutomationInput {
  const spend = parseSpendLimit(values.spendLimit);
  const { model, agentTemplate, ...carried } = values.carried;
  const template: AgentAutomationTemplateInput = {
    ...carried,
    task: values.task.trim(),
    repo: values.repo.trim() || null,
    agentConfig: {
      agent: values.agent,
      ...(model ? { model } : {}),
      ...(agentTemplate ? { template: agentTemplate } : {}),
    },
    maxSpendUSD: spend.ok ? spend.value : null,
    secretNames: [...values.secretNames].sort(),
    skills: composerSkillsInput(values.skills),
  };
  // outputSchema travels as JSON text; the stored value is already text.
  return {
    name: values.name.trim(),
    triggers: [
      { type: "schedule", schedule: draftToScheduleInput(values.schedule) },
    ],
    template,
    notifications: { email: values.emailNotifications },
    ...(options.ifVersion != null ? { ifVersion: options.ifVersion } : {}),
  };
}

/** Where a server refusal is shown. */
export type ErrorTarget = FormField | "general";

export interface MappedError {
  target: ErrorTarget;
  /** A schedule sub-field the refusal named, when it did. */
  scheduleField?: keyof ScheduleProblems;
  /** The server's own message, shown next to the field. */
  message: string;
  code: string | null;
}

const SCHEDULE_FIELDS: Record<string, keyof ScheduleProblems> = {
  frequency: "frequency",
  minute: "minute",
  at: "at",
  weekdays: "weekdays",
  runAt: "runAt",
  cron: "cron",
  timezone: "timezone",
  endsAt: "endsOn",
};

/** An offending input field name from the server mapped to the form's field. */
function targetForField(
  field: string,
): Pick<MappedError, "target" | "scheduleField"> {
  const bare = field.replace(/^triggers(\[\d+\])?\.?(schedule\.)?/, "");
  if (field.startsWith("triggers") && bare === "")
    return { target: "schedule" };
  const sub = SCHEDULE_FIELDS[bare];
  if (sub) return { target: "schedule", scheduleField: sub };
  // notifications.email: the email setting is the form's only notification
  // setting (w9/m343).
  if (field === "notifications" || field.startsWith("notifications.")) {
    return { target: "emailNotifications" };
  }
  switch (field) {
    case "name":
      return { target: "name" };
    case "template.maxSpendUSD":
    case "maxSpendUSD":
      return { target: "spendLimit" };
    case "template.repo":
    case "repo":
      return { target: "repo" };
    case "template.task":
    case "task":
    case "agentConfig.task":
      return { target: "task" };
    case "template.skills":
    case "skills":
      return { target: "skills" };
    default:
      return { target: "general" };
  }
}

const CODE_TARGETS: Record<string, ErrorTarget> = {
  AGENT_AUTOMATION_SCHEDULE_INVALID: "schedule",
  AGENT_AUTOMATION_SCHEDULE_PAST: "schedule",
  AGENT_AUTOMATION_SCHEDULE_TOO_FREQUENT: "schedule",
  AGENT_AUTOMATION_SPEND_LIMIT_REQUIRED: "spendLimit",
  AGENT_SESSION_SPEND_LIMIT_INVALID: "spendLimit",
  AGENT_AUTOMATION_CONFLICT: "name",
};

/**
 * Maps a refused create/update onto the field it concerns: the `field`
 * extension the server names wins, the error code decides otherwise, and
 * anything else is a general message above the form. The message is the
 * server's own wording; a transport failure falls back to `fallback`.
 */
export function mapAutomationError(
  err: unknown,
  fallback: string,
): MappedError {
  const message = mutationErrorMessage(err, fallback);
  if (CombinedGraphQLErrors.is(err)) {
    const first = err.errors[0];
    const code =
      typeof first?.extensions?.["code"] === "string"
        ? (first.extensions["code"] as string)
        : null;
    const field = first?.extensions?.["field"];
    if (typeof field === "string") {
      const mapped = targetForField(field);
      if (mapped.target !== "general") return { ...mapped, message, code };
    }
    if (code && CODE_TARGETS[code]) {
      // A name collision is the only conflict that is the name's fault; a
      // stale-version conflict is a general message.
      if (
        code === "AGENT_AUTOMATION_CONFLICT" &&
        /version|changed/i.test(message)
      ) {
        return { target: "general", message, code };
      }
      return { target: CODE_TARGETS[code], message, code };
    }
    return { target: "general", message, code };
  }
  return { target: "general", message, code: null };
}
