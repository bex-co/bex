import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { codedGraphQLError } from "@/test/mocks/apollo";
import { AutomationForm } from "@/features/agent-automations/components/automation-form";
import {
  emptyFormValues,
  valuesFromAutomation,
} from "@/features/agent-automations/lib/form";
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
