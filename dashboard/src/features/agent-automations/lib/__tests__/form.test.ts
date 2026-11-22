import { describe, expect, it } from "vitest";
import { codedGraphQLError, uncodedGraphQLError } from "@/test/mocks/apollo";
import {
  emptyFormValues,
  hasProblems,
  mapAutomationError,
  toAutomationInput,
  validateForm,
  valuesFromAutomation,
  type AutomationFormValues,
} from "@/features/agent-automations/lib/form";
import type { AgentAutomationFieldsFragment } from "@/graphql/definitions";
import { automation } from "@/features/agent-automations/components/__tests__/fixtures";

const NOW = Date.parse("2026-11-07T12:00:00Z");

function valid(over: Partial<AutomationFormValues> = {}): AutomationFormValues {
  return {
    ...emptyFormValues("Europe/Berlin"),
    name: "nightly-deps",
    task: "  Update the dependencies  ",
    repo: "acme/widgets",
    spendLimit: "5",
    ...over,
  };
}

describe("validateForm", () => {
  it("passes a complete form", () => {
    expect(hasProblems(validateForm(valid(), NOW))).toBe(false);
  });

  it("requires a slug name, a task, a sane spend limit and valid skill arguments", () => {
    const problems = validateForm(
      valid({
        name: "Nightly Deps",
        task: "   ",
        spendLimit: "-3",
        skills: [{ name: "run-tests", arguments: "a\nb" }],
      }),
      NOW,
    );
    expect(problems.fields).toEqual({
      name: "agentAutomations.problem.nameInvalid",
      task: "agentAutomations.problem.taskRequired",
      spendLimit: "agentAutomations.problem.spendLimit",
      skills: "agentAutomations.problem.skills",
    });
    expect(validateForm(valid({ name: "" }), NOW).fields.name).toBe(
      "agentAutomations.problem.nameRequired",
    );
    expect(validateForm(valid({ name: "a".repeat(65) }), NOW).fields.name).toBe(
      "agentAutomations.problem.nameInvalid",
    );
  });

  it("accepts an empty spend limit (the server decides whether one is required)", () => {
    expect(validateForm(valid({ spendLimit: "" }), NOW).fields).toEqual({});
  });

  it("carries the schedule's own problems", () => {
    const values = valid();
    values.schedule = { ...values.schedule, frequency: "custom", cron: "" };
    const problems = validateForm(values, NOW);
    expect(problems.schedule).toEqual({
      cron: "agentAutomations.problem.cron",
    });
    expect(hasProblems(problems)).toBe(true);
  });
});

describe("toAutomationInput", () => {
  it("builds the create input", () => {
    expect(
      toAutomationInput(
        valid({
          secretNames: ["TOKEN", "API_KEY"],
          skills: [
            { name: "run-tests", arguments: " fast " },
            { name: "lint" },
          ],
        }),
      ),
    ).toEqual({
      name: "nightly-deps",
      triggers: [
        {
          type: "schedule",
          schedule: {
            frequency: "day",
            at: "09:00",
            timezone: "Europe/Berlin",
          },
        },
      ],
      template: {
        task: "Update the dependencies",
        repo: "acme/widgets",
        agentConfig: { agent: "claude" },
        maxSpendUSD: 5,
        secretNames: ["API_KEY", "TOKEN"],
        skills: [{ name: "run-tests", arguments: "fast" }, { name: "lint" }],
      },
      notifications: { email: "on_failure" },
    });
  });

  it("sends the chosen email setting; a new form defaults to on_failure (w9/m343)", () => {
    expect(emptyFormValues().emailNotifications).toBe("on_failure");
    expect(
      toAutomationInput(valid({ emailNotifications: "never" })).notifications,
    ).toEqual({ email: "never" });
  });

  it("sends a blank repo as null and a blank spend limit as null", () => {
    const input = toAutomationInput(valid({ repo: " ", spendLimit: "" }));
    expect(input.template?.repo).toBeNull();
    expect(input.template?.maxSpendUSD).toBeNull();
  });

  // w10/m519: a named template is sent; "" names none, so each run takes the
  // workspace default in force when it fires.
  it("sends the chosen sandbox template and none for the workspace default", () => {
    expect(
      toAutomationInput(valid({ template: "agent-8c-16g" })).template
        ?.agentConfig,
    ).toEqual({ agent: "claude", template: "agent-8c-16g" });
    expect(
      toAutomationInput(valid({ template: "" })).template?.agentConfig,
    ).toEqual({ agent: "claude" });
  });

  it("sends ifVersion on an update", () => {
    expect(toAutomationInput(valid(), { ifVersion: 4 }).ifVersion).toBe(4);
    expect(toAutomationInput(valid()).ifVersion).toBeUndefined();
  });

  it("keeps the template fields the form does not edit when saving a stored automation", () => {
    const stored = {
      __typename: "AgentAutomation",
      id: "aut-1",
      ownerId: "tea-1",
      name: "nightly-deps",
      enabled: true,
      createdBy: "usr-1",
      triggers: [
        {
          __typename: "AgentAutomationTrigger",
          type: "schedule",
          schedule: {
            __typename: "AgentAutomationSchedule",
            frequency: "week",
            minute: null,
            at: "08:00",
            weekdays: ["mon", "wed"],
            runAt: null,
            cron: null,
            timezone: "Europe/Berlin",
            endsAt: null,
          },
        },
      ],
      template: {
        __typename: "AgentAutomationTemplate",
        task: "Update",
        repo: "acme/widgets",
        branch: "bex-agent/fixed",
        title: "Weekly",
        agentConfig: {
          __typename: "AgentAutomationAgentConfig",
          agent: "codex",
          model: "gpt-x",
          template: null,
        },
        maxSpendUSD: 2.5,
        secretNames: ["TOKEN"],
        skills: [],
        egressAllowlist: ["example.com"],
        tags: ["ci"],
        outputSchema: '{"type":"object"}',
        outputRequired: true,
      },
      notifications: {
        __typename: "AgentAutomationNotifications",
        email: "on_success",
      },
      nextFireAt: null,
      lastFireAt: null,
      lastSessionId: null,
      pausedReason: null,
      consecutiveFailures: 0,
      version: 3,
      createdAt: "2026-11-01T00:00:00Z",
      updatedAt: "2026-11-01T00:00:00Z",
    } as unknown as AgentAutomationFieldsFragment;
    const values = valuesFromAutomation(stored);
    expect(values.agent).toBe("codex");
    // A template that names no sandbox follows the workspace default.
    expect(values.template).toBe("");
    expect(values.spendLimit).toBe("2.5");
    expect(values.schedule.weekdays).toEqual(["mon", "wed"]);
    expect(values.emailNotifications).toBe("on_success");
    const input = toAutomationInput(values, { ifVersion: 3 });
    expect(input.notifications).toEqual({ email: "on_success" });
    expect(input.template).toMatchObject({
      branch: "bex-agent/fixed",
      title: "Weekly",
      tags: ["ci"],
      egressAllowlist: ["example.com"],
      outputSchema: '{"type":"object"}',
      outputRequired: true,
      agentConfig: { agent: "codex", model: "gpt-x" },
    });
  });
});

describe("mapAutomationError (server refusals placed on their field)", () => {
  const fallback = "Couldn't save";

  it("places a schedule refusal on the field the server names", () => {
    const err = codedGraphQLError(
      "AGENT_AUTOMATION_SCHEDULE_INVALID",
      { field: "cron" },
      "cron is not a valid five-field expression",
    );
    expect(mapAutomationError(err, fallback)).toEqual({
      target: "schedule",
      scheduleField: "cron",
      message: "Cron is not a valid five-field expression",
      code: "AGENT_AUTOMATION_SCHEDULE_INVALID",
    });
    expect(
      mapAutomationError(
        codedGraphQLError("AGENT_AUTOMATION_SCHEDULE_PAST", {
          field: "endsAt",
        }),
        fallback,
      ),
    ).toMatchObject({ target: "schedule", scheduleField: "endsOn" });
  });

  it("places a too-frequent schedule on the schedule when the server names no field", () => {
    expect(
      mapAutomationError(
        codedGraphQLError(
          "AGENT_AUTOMATION_SCHEDULE_TOO_FREQUENT",
          {},
          "runs must be 5 minutes apart",
        ),
        fallback,
      ),
    ).toEqual({
      target: "schedule",
      message: "Runs must be 5 minutes apart",
      code: "AGENT_AUTOMATION_SCHEDULE_TOO_FREQUENT",
    });
  });

  it("places the missing spend ceiling on the spend field", () => {
    expect(
      mapAutomationError(
        codedGraphQLError("AGENT_AUTOMATION_SPEND_LIMIT_REQUIRED", {
          field: "template.maxSpendUSD",
        }),
        fallback,
      ).target,
    ).toBe("spendLimit");
  });

  it("places name and template-field refusals", () => {
    expect(
      mapAutomationError(
        codedGraphQLError("AGENT_AUTOMATION_INVALID", { field: "name" }),
        fallback,
      ).target,
    ).toBe("name");
    expect(
      mapAutomationError(
        codedGraphQLError("AGENT_SESSION_INPUT_INVALID", { field: "repo" }),
        fallback,
      ).target,
    ).toBe("repo");
    // An unknown sandbox template (w10/m519), however the server names it.
    for (const field of [
      "agentConfig.template",
      "template.agentConfig.template",
    ]) {
      expect(
        mapAutomationError(
          codedGraphQLError("AGENT_SESSION_INPUT_INVALID", { field }),
          fallback,
        ).target,
      ).toBe("template");
    }
  });

  it("places a refused email setting on the email notifications field", () => {
    expect(
      mapAutomationError(
        codedGraphQLError(
          "AGENT_AUTOMATION_INVALID",
          { field: "notifications.email" },
          "notifications.email must be always, on_failure, on_success or never",
        ),
        fallback,
      ),
    ).toMatchObject({
      target: "emailNotifications",
      code: "AGENT_AUTOMATION_INVALID",
    });
  });

  it("puts a taken name on the name but a stale version in the general slot", () => {
    expect(
      mapAutomationError(
        codedGraphQLError(
          "AGENT_AUTOMATION_CONFLICT",
          {},
          "the workspace already has an automation with this name",
        ),
        fallback,
      ).target,
    ).toBe("name");
    expect(
      mapAutomationError(
        codedGraphQLError(
          "AGENT_AUTOMATION_CONFLICT",
          {},
          "the automation changed since that version; read it again",
        ),
        fallback,
      ).target,
    ).toBe("general");
  });

  it("shows an unplaced refusal in general and a transport failure as the fallback", () => {
    expect(
      mapAutomationError(
        codedGraphQLError(
          "AGENT_AUTOMATION_LIMIT",
          {},
          "a workspace holds at most 50 automations",
        ),
        fallback,
      ),
    ).toMatchObject({
      target: "general",
      message: "A workspace holds at most 50 automations",
    });
    expect(mapAutomationError(new Error("Failed to fetch"), fallback)).toEqual({
      target: "general",
      message: fallback,
      code: null,
    });
    expect(
      mapAutomationError(uncodedGraphQLError("nope"), fallback).target,
    ).toBe("general");
  });
});

// w10/m561: the automation's own gh binding rides in its template's
// agentConfig, and its lock refusal is placed on the field in the form's own
// words.
describe("the gh binding", () => {
  const agentConfig = (values: AutomationFormValues) =>
    toAutomationInput(values).template?.agentConfig;

  it("sends on or off and nothing to inherit", () => {
    expect(emptyFormValues().githubCli).toBe("inherit");
    expect(agentConfig(valid())).toEqual({ agent: "claude" });
    expect(agentConfig(valid({ githubCli: "off" }))).toEqual({
      agent: "claude",
      githubCli: false,
    });
    expect(agentConfig(valid({ githubCli: "on" }))).toEqual({
      agent: "claude",
      githubCli: true,
    });
  });

  it.each([
    [null, "inherit"],
    [false, "off"],
    [true, "on"],
  ] as const)("reads a stored %s back as %s", (githubCli, choice) => {
    const base = automation();
    const stored = automation({
      template: {
        ...base.template,
        agentConfig: { ...base.template.agentConfig, githubCli },
      },
    });
    const values = valuesFromAutomation(stored);
    expect(values.githubCli).toBe(choice);
    expect(agentConfig(values)?.githubCli).toBe(githubCli ?? undefined);
  });

  it.each([
    ["mandatory", "agentAutomations.problem.githubCliMandatory"],
    ["admin_required", "agentAutomations.problem.githubCliAdminRequired"],
  ])("places a %s lock on the field with its own copy", (reason, key) => {
    const err = codedGraphQLError(
      "AGENT_AUTOMATION_GITHUB_CLI_LOCKED",
      { field: "template.agentConfig.githubCli", reason },
      "server wording",
    );
    expect(mapAutomationError(err, "Couldn't save")).toEqual({
      target: "githubCli",
      message: "Server wording",
      messageKey: key,
      code: "AGENT_AUTOMATION_GITHUB_CLI_LOCKED",
    });
  });
});
