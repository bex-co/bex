import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { codedGraphQLError } from "@/test/mocks/apollo";
import type { AgentAutomationInput } from "@/graphql/definitions";
import { AutomationForm } from "@/features/agent-automations/components/automation-form";
import {
  emptyFormValues,
  valuesFromAutomation,
} from "@/features/agent-automations/lib/form";
import type { AgentTemplateOffer } from "@/features/agent-sessions/types";
import type { AgentEnvironmentView } from "@/features/agent-environment/lib/environment";
import { agentEnvironment } from "@/test/mocks/agent-environment";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
import { automation } from "./fixtures";

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({ children }: { children: ReactNode }) => <a href="#">{children}</a>,
  };
});
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
}));
const capabilitiesState: {
  capabilities: {
    agents: string[];
    models: Record<string, { model: string | null; models: string[] }>;
    sandbox?: AgentTemplateOffer;
  } | null;
} = {
  capabilities: null,
};
vi.mock(
  "@/features/agent-sessions/hooks/use-agent-session-capabilities",
  () => ({ useAgentSessionCapabilities: () => capabilitiesState }),
);
// The workspace's gh chain (w10/m561): null until a test loads one.
const envState = vi.hoisted(() => ({
  environment: null as AgentEnvironmentView | null,
}));
vi.mock("@/features/agent-environment/hooks/use-agent-environment", () => ({
  useAgentEnvironment: () => ({
    environment: envState.environment,
    loading: false,
    error: undefined,
    refetch: vi.fn(),
    update: vi.fn(),
    saving: false,
  }),
}));
vi.mock("@/features/services/hooks/use-repos", () => ({
  useRepos: () => ({
    repos: [{ fullName: "acme/widgets" }],
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("@/features/agent-sessions/components/composer-secrets", () => ({
  ComposerSecrets: ({
    workspaceOnly,
    onChange,
  }: {
    workspaceOnly?: boolean;
    onChange: (v: { secretNames: string[]; secrets: [] }) => void;
  }) => (
    <button
      type="button"
      data-workspace-only={String(workspaceOnly)}
      onClick={() => onChange({ secretNames: ["TOKEN"], secrets: [] })}
    >
      pick-secret
    </button>
  ),
}));
vi.mock("@/features/agent-sessions/components/composer-skills", () => ({
  ComposerSkills: () => null,
  ComposerSkillChips: () => null,
}));

function setup(onSave = vi.fn(async () => undefined)) {
  const user = userEvent.setup();
  render(
    <AutomationForm
      mode="create"
      initial={emptyFormValues("Europe/Berlin")}
      onSave={onSave}
      cancelTo={{ to: "/automations" }}
    />,
  );
  return { user, onSave };
}

async function fillValid(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText("Name"), "nightly-deps");
  await user.type(screen.getByLabelText("Task"), "Update the dependencies");
}

function renderWithZone(mode: "create" | "edit", zone: string) {
  render(
    <AutomationForm
      mode={mode}
      initial={emptyFormValues(zone)}
      version={mode === "edit" ? 1 : undefined}
      onSave={vi.fn(async () => undefined)}
      cancelTo={{ to: "/automations" }}
    />,
  );
}

describe("AutomationForm time zone", () => {
  it("an unchosen create-form zone is the browser's zone", async () => {
    const spy = vi
      .spyOn(Intl.DateTimeFormat.prototype, "resolvedOptions")
      .mockReturnValue({
        timeZone: "Asia/Tokyo",
      } as Intl.ResolvedDateTimeFormatOptions);
    renderWithZone("create", "");
    await waitFor(() =>
      expect(screen.getByText(/Runs:.*Asia\/Tokyo/)).toBeInTheDocument(),
    );
    spy.mockRestore();
  });

  it("an edit form keeps the saved zone even when it is UTC", async () => {
    const spy = vi
      .spyOn(Intl.DateTimeFormat.prototype, "resolvedOptions")
      .mockReturnValue({
        timeZone: "Asia/Tokyo",
      } as Intl.ResolvedDateTimeFormatOptions);
    renderWithZone("edit", "UTC");
    await Promise.resolve();
    expect(screen.queryByText(/Runs:.*Asia\/Tokyo/)).not.toBeInTheDocument();
    spy.mockRestore();
  });
});

describe("AutomationForm", () => {
  it("blocks an empty submit, announces each problem on its field and focuses the first", async () => {
    const { user, onSave } = setup();
    await user.click(screen.getByRole("button", { name: "Create automation" }));

    expect(onSave).not.toHaveBeenCalled();
    const name = screen.getByLabelText("Name");
    expect(name).toHaveAttribute("aria-invalid", "true");
    expect(name).toHaveAccessibleDescription(/Enter a name\./);
    expect(screen.getByLabelText("Task")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(screen.getAllByRole("alert").map((a) => a.textContent)).toEqual([
      "Enter a name.",
      "Describe the task.",
    ]);
    await waitFor(() => expect(name).toHaveFocus());
  });

  it("rejects a name that is not a slug as the server would", async () => {
    const { user, onSave } = setup();
    await fillValid(user);
    await user.clear(screen.getByLabelText("Name"));
    await user.type(screen.getByLabelText("Name"), "Nightly Deps");
    await user.click(screen.getByRole("button", { name: "Create automation" }));
    expect(onSave).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Use lowercase letters, digits and single hyphens",
    );
  });

  it("saves the API input for a valid form, with a workspace-only secrets picker", async () => {
    const { user, onSave } = setup();
    await fillValid(user);
    const secrets = screen.getByRole("button", { name: "pick-secret" });
    expect(secrets).toHaveAttribute("data-workspace-only", "true");
    await user.click(secrets);
    await user.type(screen.getByLabelText(/Spend limit/), "7.5");
    await user.click(screen.getByRole("button", { name: "Create automation" }));

    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1));
    expect(onSave).toHaveBeenCalledWith({
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
        repo: null,
        agentConfig: { agent: "claude" },
        maxSpendUSD: 7.5,
        secretNames: ["TOKEN"],
        skills: [],
      },
      notifications: { email: "on_failure" },
    });
  });

  it("emails on failure by default and saves the email setting the person picks (w9/m343)", async () => {
    const { user, onSave } = setup();
    await fillValid(user);
    const email = screen.getByRole("combobox", { name: "Email notifications" });
    expect(email).toHaveTextContent("On failure");
    expect(email).toHaveAccessibleDescription(
      "Which runs email you. A platform pause always does.",
    );
    await user.click(email);
    await user.click(await screen.findByRole("option", { name: "On success" }));
    await user.click(screen.getByRole("button", { name: "Create automation" }));

    await waitFor(() =>
      expect(onSave).toHaveBeenCalledWith(
        expect.objectContaining({ notifications: { email: "on_success" } }),
      ),
    );
  });

  it("an edit form opens on the stored email setting and saves it back", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn(async () => undefined);
    const stored = automation({
      notifications: {
        __typename: "AgentAutomationNotifications",
        email: "never",
      },
    });
    render(
      <AutomationForm
        mode="edit"
        initial={valuesFromAutomation(stored)}
        version={stored.version}
        onSave={onSave}
        cancelTo={{ to: "/automations/$automationId", id: stored.id }}
      />,
    );
    expect(
      screen.getByRole("combobox", { name: "Email notifications" }),
    ).toHaveTextContent("Never");
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    await waitFor(() =>
      expect(onSave).toHaveBeenCalledWith(
        expect.objectContaining({
          notifications: { email: "never" },
          ifVersion: 1,
        }),
      ),
    );
  });

  it("shows the server's refusal of the email setting under that field", async () => {
    const onSave = vi.fn(async () => {
      throw codedGraphQLError(
        "AGENT_AUTOMATION_INVALID",
        { field: "notifications.email" },
        "notifications.email must be always, on_failure, on_success or never",
      );
    });
    const { user } = setup(onSave);
    await fillValid(user);
    await user.click(screen.getByRole("button", { name: "Create automation" }));

    const email = screen.getByRole("combobox", { name: "Email notifications" });
    await waitFor(() => expect(email).toHaveAttribute("aria-invalid", "true"));
    expect(email).toHaveAccessibleDescription(
      /notifications\.email must be always, on_failure, on_success or never/i,
    );
    expect(screen.getAllByRole("alert")).toHaveLength(1);
  });

  it("shows the server's spend-limit refusal under the spend field and clears it on edit", async () => {
    const onSave = vi.fn(async () => {
      throw codedGraphQLError(
        "AGENT_AUTOMATION_SPEND_LIMIT_REQUIRED",
        { field: "template.maxSpendUSD" },
        "a template on the platform model needs maxSpendUSD",
      );
    });
    const { user } = setup(onSave);
    await fillValid(user);
    await user.click(screen.getByRole("button", { name: "Create automation" }));

    const spend = screen.getByLabelText(/Spend limit/);
    await waitFor(() => expect(spend).toHaveAttribute("aria-invalid", "true"));
    expect(spend).toHaveAccessibleDescription(
      /A template on the platform model needs maxSpendUSD/,
    );
    await user.type(spend, "3");
    expect(spend).not.toHaveAttribute("aria-invalid", "true");
  });

  it("shows a too-frequent cron refusal under the schedule", async () => {
    const onSave = vi.fn(async () => {
      throw codedGraphQLError(
        "AGENT_AUTOMATION_SCHEDULE_TOO_FREQUENT",
        { field: "cron" },
        "runs must be at least 5 minutes apart",
      );
    });
    const { user } = setup(onSave);
    await fillValid(user);
    await user.click(screen.getByRole("button", { name: "Create automation" }));
    expect(
      await screen.findByText("Runs must be at least 5 minutes apart"),
    ).toBeInTheDocument();
  });

  it("shows an unplaced refusal as a general alert and re-enables the form", async () => {
    const onSave = vi.fn(async () => {
      throw codedGraphQLError(
        "AGENT_AUTOMATION_LIMIT",
        {},
        "a workspace holds at most 50 automations",
      );
    });
    const { user } = setup(onSave);
    await fillValid(user);
    await user.click(screen.getByRole("button", { name: "Create automation" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "A workspace holds at most 50 automations",
    );
    expect(
      screen.getByRole("button", { name: "Create automation" }),
    ).toBeEnabled();
  });
});

// w10/m516: an automation picks its model from the agent's catalog, and a
// refused model is shown under the model field.
describe("AutomationForm model", () => {
  const catalog = [
    "deepseek/deepseek-v4.1-flash-1017",
    "anthropic/claude-opus-5.5",
  ];
  afterEach(() => {
    capabilitiesState.capabilities = null;
  });

  it("saves the picked catalog model in the template", async () => {
    capabilitiesState.capabilities = {
      agents: ["claude"],
      models: { claude: { model: catalog[0], models: catalog } },
    };
    const { user, onSave } = setup();
    await fillValid(user);
    await user.type(screen.getByLabelText(/Spend limit/), "5");
    const model = screen.getByRole("combobox", { name: "Model" });
    expect(model).toHaveTextContent(catalog[0]);
    await user.click(model);
    await user.click(screen.getByRole("option", { name: catalog[1] }));
    await user.click(screen.getByRole("button", { name: "Create automation" }));
    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1));
    expect(onSave).toHaveBeenCalledWith(
      expect.objectContaining({
        template: expect.objectContaining({
          agentConfig: { agent: "claude", model: catalog[1] },
        }),
      }),
    );
  });

  // w10/m527: a model an admin turned off leaves the capabilities' list; an
  // automation saved on it shows it, with a note that its runs use the
  // workspace default, until someone picks an offered one.
  it("shows a saved model the workspace no longer offers, with a note", async () => {
    const offered = [catalog[0], "anthropic/claude-haiku-5.5"];
    capabilitiesState.capabilities = {
      agents: ["claude"],
      models: { claude: { model: offered[1], models: offered } },
    };
    const base = automation();
    const saved = automation({
      template: {
        ...base.template,
        agentConfig: { ...base.template.agentConfig, model: catalog[1] },
      },
    });
    const user = userEvent.setup();
    const onSave = vi.fn(async () => undefined);
    render(
      <AutomationForm
        mode="edit"
        initial={valuesFromAutomation(saved)}
        version={saved.version}
        onSave={onSave}
        cancelTo={{ to: "/automations/$automationId", id: saved.id }}
      />,
    );
    const model = screen.getByRole("combobox", { name: "Model" });
    expect(model).toHaveTextContent(catalog[1]);
    expect(model).toHaveAccessibleDescription(
      "anthropic/claude-opus-5.5 is no longer offered in this workspace: an admin turned it off, or it left the platform catalog. Runs use the workspace default, anthropic/claude-haiku-5.5, instead. Pick an offered model to save changes.",
    );
    await user.click(model);
    // Only the offered models are choices.
    expect(
      screen.getAllByRole("option").map((option) => option.textContent),
    ).toEqual([catalog[0], "anthropic/claude-haiku-5.5 (default)"]);
    await user.click(screen.getByRole("option", { name: catalog[0] }));
    expect(model).toHaveAccessibleDescription(
      "The model each run uses. Leave it at the default to follow the workspace's.",
    );
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1));
    expect(
      (onSave.mock.calls[0] as unknown as [AgentAutomationInput])[0].template
        ?.agentConfig,
    ).toEqual({ agent: "claude", model: catalog[0] });
  });

  // With one model left on, a saved model that is off still offers it.
  it("lets a saved model that is off be replaced when one model is offered", async () => {
    capabilitiesState.capabilities = {
      agents: ["claude"],
      models: { claude: { model: catalog[0], models: [catalog[0]] } },
    };
    const base = automation();
    const saved = automation({
      template: {
        ...base.template,
        agentConfig: { ...base.template.agentConfig, model: catalog[1] },
      },
    });
    const user = userEvent.setup();
    const onSave = vi.fn(async () => undefined);
    render(
      <AutomationForm
        mode="edit"
        initial={valuesFromAutomation(saved)}
        version={saved.version}
        onSave={onSave}
        cancelTo={{ to: "/automations/$automationId", id: saved.id }}
      />,
    );
    expect(screen.queryByTestId("model-picker-fixed")).toBeNull();
    const model = screen.getByRole("combobox", { name: "Model" });
    expect(model).toHaveTextContent(catalog[1]);
    expect(model).toHaveAccessibleDescription(
      /Runs use the workspace default, deepseek\/deepseek-v4\.1-flash-1017, instead/,
    );
    await user.click(model);
    await user.click(
      screen.getByRole("option", { name: `${catalog[0]} (default)` }),
    );
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1));
    // The workspace default: no model named.
    expect(
      (onSave.mock.calls[0] as unknown as [AgentAutomationInput])[0].template
        ?.agentConfig,
    ).toEqual({ agent: "claude" });
  });

  it("shows the server's model refusal under the model field", async () => {
    const onSave = vi.fn(async () => {
      throw codedGraphQLError(
        "AGENT_SESSION_MODEL_NOT_OFFERED",
        { field: "agentConfig.model", model: "gpt-x" },
        "this workspace runs agents on the platform model route, which offers a, b; \"gpt-x\" is not one of them",
      );
    });
    const { user } = setup(onSave);
    await fillValid(user);
    const model = screen.getByLabelText("Model");
    await user.type(model, "gpt-x");
    await user.click(screen.getByRole("button", { name: "Create automation" }));
    await waitFor(() => expect(model).toHaveAttribute("aria-invalid", "true"));
    expect(model).toHaveAccessibleDescription(/"gpt-x" is not one of them/);
  });
});

// w10/m519: an automation runs on a sandbox template it names, or on the
// workspace default in force at each run when it names none. The list is the
// server's: a made-up third template proves no name or size is built in.
describe("AutomationForm sandbox", () => {
  const sandbox: AgentTemplateOffer = {
    defaultTemplate: "agent-4c-8g",
    templates: [
      { name: "agent", cpu: 2, memoryGb: 4 },
      { name: "agent-4c-8g", cpu: 4, memoryGb: 8 },
      { name: "agent-8c-16g", cpu: 8, memoryGb: 16 },
    ],
  };
  const picker = () => screen.getByRole("combobox", { name: "Sandbox" });
  const stored = (template: string | null) => {
    const base = automation();
    return automation({
      template: {
        ...base.template,
        agentConfig: { ...base.template.agentConfig, template },
      },
    });
  };
  function renderEdit(template: string | null) {
    const user = userEvent.setup();
    const onSave = vi.fn(async () => undefined);
    const saved = stored(template);
    render(
      <AutomationForm
        mode="edit"
        initial={valuesFromAutomation(saved)}
        version={saved.version}
        onSave={onSave}
        cancelTo={{ to: "/automations/$automationId", id: saved.id }}
      />,
    );
    return { user, onSave };
  }
  const savedAgentConfig = (onSave: ReturnType<typeof vi.fn>) =>
    (onSave.mock.calls[0][0] as AgentAutomationInput).template?.agentConfig;

  beforeEach(() => {
    capabilitiesState.capabilities = {
      agents: ["claude"],
      models: {},
      sandbox,
    };
  });
  afterEach(() => {
    capabilitiesState.capabilities = null;
  });

  it("follows the workspace default by default and sends no template", async () => {
    const { user, onSave } = setup();
    await fillValid(user);
    expect(picker()).toHaveTextContent(
      "Workspace default (agent-4c-8g · 4 CPU, 8 GB)",
    );
    await user.click(picker());
    expect(
      screen.getAllByRole("option").map((option) => option.textContent),
    ).toEqual([
      "Workspace default (agent-4c-8g · 4 CPU, 8 GB)",
      "agent · 2 CPU, 4 GB",
      "agent-4c-8g · 4 CPU, 8 GB",
      "agent-8c-16g · 8 CPU, 16 GB",
    ]);
    await user.keyboard("{Escape}");
    await user.click(screen.getByRole("button", { name: "Create automation" }));
    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1));
    expect(savedAgentConfig(onSave)).toEqual({ agent: "claude" });
  });

  it("saves the picked template", async () => {
    const { user, onSave } = setup();
    await fillValid(user);
    await user.click(picker());
    await user.click(
      screen.getByRole("option", { name: "agent-8c-16g · 8 CPU, 16 GB" }),
    );
    expect(picker()).toHaveTextContent("agent-8c-16g · 8 CPU, 16 GB");
    await user.click(screen.getByRole("button", { name: "Create automation" }));
    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1));
    expect(savedAgentConfig(onSave)).toEqual({
      agent: "claude",
      template: "agent-8c-16g",
    });
  });

  it("pins the default's template by name when it is picked itself", async () => {
    const { user, onSave } = setup();
    await fillValid(user);
    await user.click(picker());
    await user.click(
      screen.getByRole("option", { name: "agent-4c-8g · 4 CPU, 8 GB" }),
    );
    await user.click(screen.getByRole("button", { name: "Create automation" }));
    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1));
    expect(savedAgentConfig(onSave)).toEqual({
      agent: "claude",
      template: "agent-4c-8g",
    });
  });

  it("an edit form shows the stored template and can return to the workspace default", async () => {
    const { user, onSave } = renderEdit("agent-8c-16g");
    expect(picker()).toHaveTextContent("agent-8c-16g · 8 CPU, 16 GB");
    await user.click(picker());
    await user.click(
      screen.getByRole("option", { name: /^Workspace default/ }),
    );
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1));
    expect(savedAgentConfig(onSave)).toEqual({ agent: "claude" });
  });

  it("keeps a stored template the list no longer carries", async () => {
    const { user, onSave } = renderEdit("retired-1c-1g");
    expect(picker()).toHaveTextContent("retired-1c-1g");
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1));
    expect(savedAgentConfig(onSave)).toEqual({
      agent: "claude",
      template: "retired-1c-1g",
    });
  });

  it("keeps the stored template, read-only, while no list is available", async () => {
    capabilitiesState.capabilities = null;
    const { user, onSave } = renderEdit("agent-8c-16g");
    const field = screen.getByLabelText("Sandbox");
    expect(field).toHaveValue("agent-8c-16g");
    expect(field).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1));
    expect(savedAgentConfig(onSave)).toEqual({
      agent: "claude",
      template: "agent-8c-16g",
    });
  });

  it("shows the server's template refusal under the sandbox field", async () => {
    const onSave = vi.fn(async () => {
      throw codedGraphQLError(
        "AGENT_SESSION_INPUT_INVALID",
        {
          field: "agentConfig.template",
          template: "agent-8c-16g",
          templates: ["agent", "agent-4c-8g"],
        },
        'agentConfig.template "agent-8c-16g" is not an agent template; use one of agent (2 CPU, 4 GB), agent-4c-8g (4 CPU, 8 GB)',
      );
    });
    const { user } = setup(onSave);
    await fillValid(user);
    await user.click(picker());
    await user.click(
      screen.getByRole("option", { name: "agent-8c-16g · 8 CPU, 16 GB" }),
    );
    await user.click(screen.getByRole("button", { name: "Create automation" }));
    await waitFor(() =>
      expect(picker()).toHaveAttribute("aria-invalid", "true"),
    );
    expect(picker()).toHaveAccessibleDescription(
      /is not an agent template; use one of agent/,
    );
  });
});

// w10/m561: an automation binds its sessions' gh in its editor, as Devin pins
// a security profile on an automation. Inherit follows the workspace's
// automations default, else its GitHub CLI switch; "on" is refused under the
// workspace's mandatory off, and a creator who is not an admin changes
// nothing (w10/044), unless the automation already stores the choice.
describe("AutomationForm GitHub CLI", () => {
  const picker = () =>
    screen.getByRole("combobox", { name: "GitHub CLI (gh)" });
  const option = (name: RegExp) => screen.getByRole("option", { name });
  const savedAgentConfig = (onSave: ReturnType<typeof vi.fn>) =>
    (onSave.mock.calls[0][0] as AgentAutomationInput).template?.agentConfig;
  const stored = (githubCli: boolean | null) => {
    const base = automation();
    return automation({
      template: {
        ...base.template,
        agentConfig: { ...base.template.agentConfig, githubCli },
      },
    });
  };
  function renderEdit(githubCli: boolean | null) {
    const user = userEvent.setup();
    const onSave = vi.fn(async () => undefined);
    const saved = stored(githubCli);
    render(
      <AutomationForm
        mode="edit"
        initial={valuesFromAutomation(saved)}
        version={saved.version}
        onSave={onSave}
        cancelTo={{ to: "/automations/$automationId", id: saved.id }}
      />,
    );
    return { user, onSave };
  }
  const asMember = () =>
    vi
      .mocked(useCapabilities)
      .mockReturnValue(
        mockCapabilities({ role: "DEVELOPER", canManage: false }),
      );

  beforeEach(() => {
    vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
    envState.environment = agentEnvironment();
  });
  afterEach(() => {
    vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
    envState.environment = null;
  });

  it("inherits the automations default by default and sends no binding", async () => {
    // The workspace's switch is on, its automations default off.
    envState.environment = agentEnvironment({
      githubCliEnabled: true,
      automationsGithubCliEnabled: false,
    });
    const { user, onSave } = setup();
    await fillValid(user);
    expect(picker()).toHaveTextContent("Inherit (off)");
    expect(picker()).toHaveAccessibleDescription(
      "Whether its sessions get gh. Inherit follows GitHub CLI for automations in Settings → Agents.",
    );
    await user.click(picker());
    expect(screen.getAllByRole("option").map((o) => o.textContent)).toEqual([
      "Inherit (off)",
      "On",
      "Off",
    ]);
    await user.keyboard("{Escape}");
    await user.click(screen.getByRole("button", { name: "Create automation" }));
    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1));
    expect(savedAgentConfig(onSave)).toEqual({ agent: "claude" });
  });

  it("inherits the workspace's switch while the automations default inherits", () => {
    envState.environment = agentEnvironment({ githubCliEnabled: true });
    setup();
    expect(picker()).toHaveTextContent("Inherit (on)");
  });

  it("saves off, and an edit form reloads it and saves it back", async () => {
    const { user, onSave } = setup();
    await fillValid(user);
    await user.click(picker());
    await user.click(option(/^Off$/));
    await user.click(screen.getByRole("button", { name: "Create automation" }));
    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1));
    expect(savedAgentConfig(onSave)).toEqual({
      agent: "claude",
      githubCli: false,
    });

    // The read carries it back as the stored false.
    cleanup();
    const edit = renderEdit(false);
    expect(picker()).toHaveTextContent("Off");
    await edit.user.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(edit.onSave).toHaveBeenCalledTimes(1));
    expect(savedAgentConfig(edit.onSave)).toEqual({
      agent: "claude",
      githubCli: false,
    });
  });

  it("an edit form can return a stored off to inherit", async () => {
    const { user, onSave } = renderEdit(false);
    await user.click(picker());
    await user.click(option(/^Inherit/));
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1));
    expect(savedAgentConfig(onSave)).toEqual({ agent: "claude" });
  });

  it("disables On with the reason under the workspace's mandatory off", async () => {
    envState.environment = agentEnvironment({
      githubCliEnabled: false,
      githubCliMandatory: true,
      // Ignored under the floor.
      automationsGithubCliEnabled: true,
    });
    const { user } = setup();
    expect(picker()).toHaveTextContent("Inherit (off)");
    await user.click(picker());
    const on = option(/^On/);
    expect(on).toHaveAttribute("aria-disabled", "true");
    expect(on).toHaveTextContent(
      "Unavailable while this workspace's GitHub CLI off is mandatory.",
    );
    expect(option(/^Off$/)).not.toHaveAttribute("aria-disabled");
  });

  // Devin: "Members without the permission can't enumerate, select, or
  // change profiles".
  it("lets a member who is not an admin create one that inherits, choosing neither on nor off", async () => {
    asMember();
    const { user, onSave } = setup();
    await fillValid(user);
    expect(picker()).toBeEnabled();
    await user.click(picker());
    expect(option(/^Inherit/)).not.toHaveAttribute("aria-disabled");
    for (const choice of [/^On/, /^Off/]) {
      expect(option(choice)).toHaveAttribute("aria-disabled", "true");
      expect(option(choice)).toHaveTextContent(
        "Only a workspace admin can change this.",
      );
    }
    await user.keyboard("{Escape}");
    await user.click(screen.getByRole("button", { name: "Create automation" }));
    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1));
    expect(savedAgentConfig(onSave)).toEqual({ agent: "claude" });
  });

  it("keeps a member's stored off and lets them change nothing else", async () => {
    asMember();
    const { user, onSave } = renderEdit(false);
    expect(picker()).toHaveTextContent("Off");
    await user.click(picker());
    expect(option(/^Off/)).not.toHaveAttribute("aria-disabled");
    for (const choice of [/^Inherit/, /^On/]) {
      expect(option(choice)).toHaveAttribute("aria-disabled", "true");
    }
    await user.keyboard("{Escape}");
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1));
    expect(savedAgentConfig(onSave)).toEqual({
      agent: "claude",
      githubCli: false,
    });
  });

  it("keeps an automation that already stores on, even for a member under a mandatory off", async () => {
    asMember();
    envState.environment = agentEnvironment({
      githubCliEnabled: false,
      githubCliMandatory: true,
    });
    const { user, onSave } = renderEdit(true);
    expect(picker()).toHaveTextContent("On");
    await user.click(picker());
    expect(option(/^On/)).not.toHaveAttribute("aria-disabled");
    expect(option(/^Off/)).toHaveAttribute("aria-disabled", "true");
    await user.keyboard("{Escape}");
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1));
    expect(savedAgentConfig(onSave)).toEqual({
      agent: "claude",
      githubCli: true,
    });
  });

  it.each([
    [
      "admin_required",
      "Only a workspace admin can choose or change GitHub CLI for an automation. Leave it as it was.",
    ],
    [
      "mandatory",
      "This workspace's GitHub CLI off is mandatory, so an automation can't turn gh on. Choose Off or Inherit.",
    ],
  ])(
    "shows the server's %s refusal under the GitHub CLI field and clears it on change",
    async (reason, text) => {
      const onSave = vi.fn(async () => {
        throw codedGraphQLError(
          "AGENT_AUTOMATION_GITHUB_CLI_LOCKED",
          { field: "template.agentConfig.githubCli", reason },
          "server wording",
        );
      });
      const { user } = setup(onSave);
      await fillValid(user);
      await user.click(picker());
      await user.click(option(/^On$/));
      await user.click(
        screen.getByRole("button", { name: "Create automation" }),
      );
      await waitFor(() =>
        expect(picker()).toHaveAttribute("aria-invalid", "true"),
      );
      expect(picker()).toHaveAccessibleDescription(new RegExp(`^${text}`));
      expect(screen.getAllByRole("alert")).toHaveLength(1);
      await user.click(picker());
      await user.click(option(/^Off$/));
      expect(picker()).not.toHaveAttribute("aria-invalid");
      expect(screen.queryByRole("alert")).toBeNull();
    },
  );
});
