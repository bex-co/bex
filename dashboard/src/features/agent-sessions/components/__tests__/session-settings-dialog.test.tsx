import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SessionSettingsDialog } from "@/features/agent-sessions/components/session-settings-dialog";
import { AgentSessionError } from "@/features/agent-sessions/lib/errors";
import type { AgentSessionView } from "@/features/agent-sessions/types";
import type { AgentEnvironmentView } from "@/features/agent-environment/lib/environment";
import { agentSessionView } from "@/test/mocks/agent-session";
import { agentEnvironment } from "@/test/mocks/agent-environment";
import { mockCapabilities } from "@/test/mocks/capabilities";

// w10/m599: a session's settings change its gh binding after it started, as
// Devin's session profile is "changed later from the session's settings".

const setGithubCli = vi.fn();
vi.mock("@/features/agent-sessions/hooks/use-agent-session-mutations", () => ({
  useAgentSessionMutations: () => ({ setGithubCli }),
}));
const state = vi.hoisted(() => ({
  environment: null as AgentEnvironmentView | null,
  canManage: true,
  automation: null as { githubCli: boolean | null } | null,
}));
vi.mock("@/features/agent-environment/hooks/use-agent-environment", () => ({
  useAgentEnvironment: () => ({
    environment: state.environment,
    loading: false,
    error: undefined,
    refetch: vi.fn(),
    update: vi.fn(),
    saving: false,
  }),
}));
vi.mock("@/features/capabilities/hooks/use-capabilities", () => ({
  useCapabilities: () => mockCapabilities({ canManage: state.canManage }),
}));
vi.mock("@/features/agent-automations/hooks/use-agent-automation", () => ({
  useAgentAutomation: () => ({
    automation: state.automation
      ? { template: { agentConfig: { githubCli: state.automation.githubCli } } }
      : null,
    loading: false,
    error: undefined,
    refetch: vi.fn(),
  }),
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
// A child's inherit resolves through its tree, which the page already read.
const tree = vi.hoisted(() => ({
  sessions: [] as AgentSessionView[],
  loading: false,
}));
vi.mock("@/features/agent-sessions/hooks/use-agent-session-tree", () => ({
  useAgentSessionTree: (id: string) =>
    id ? tree : { sessions: [], loading: false },
}));

function session(
  githubCli: boolean | null,
  over: Partial<AgentSessionView> = {},
): AgentSessionView {
  const base = agentSessionView({ phase: "running" });
  return {
    ...base,
    ...over,
    agentConfig: { ...base.agentConfig, githubCli },
  };
}

const picker = () => screen.getByRole("combobox", { name: "GitHub CLI (gh)" });
const option = (name: RegExp) => screen.getByRole("option", { name });
const save = () => screen.getByRole("button", { name: "Save settings" });

describe("SessionSettingsDialog", () => {
  beforeEach(() => {
    setGithubCli.mockReset();
    setGithubCli.mockResolvedValue(undefined);
    state.environment = agentEnvironment();
    state.canManage = true;
    state.automation = null;
    tree.sessions = [];
    tree.loading = false;
  });

  it("shows what the session inherits and saves an admin's off", async () => {
    const onOpenChange = vi.fn();
    const user = userEvent.setup();
    const view = session(null);
    render(
      <SessionSettingsDialog session={view} onOpenChange={onOpenChange} />,
    );
    expect(screen.getByRole("dialog")).toHaveAccessibleName("Session settings");
    expect(picker()).toHaveTextContent("Inherit (on)");
    expect(picker()).toHaveAccessibleDescription(
      expect.stringContaining("from the session's next turn"),
    );
    // Nothing changed yet, so there is nothing to save.
    expect(save()).toBeDisabled();
    await user.click(picker());
    await user.click(option(/^Off$/));
    await user.click(save());
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false));
    expect(setGithubCli).toHaveBeenCalledWith(view.id, "off");
  });

  it("returns an admin's stored off to inherit", async () => {
    const user = userEvent.setup();
    const view = session(false);
    render(<SessionSettingsDialog session={view} onOpenChange={vi.fn()} />);
    expect(picker()).toHaveTextContent(/^Off$/);
    await user.click(picker());
    await user.click(option(/^Inherit \(on\)$/));
    await user.click(save());
    await waitFor(() =>
      expect(setGithubCli).toHaveBeenCalledWith(view.id, "inherit"),
    );
  });

  // Devin: "Members without the permission can't enumerate, select, or
  // change profiles", but "anyone can see whether their session is governed
  // by a profile".
  it("shows a member what applies with every other choice locked", async () => {
    state.canManage = false;
    const user = userEvent.setup();
    render(
      <SessionSettingsDialog session={session(false)} onOpenChange={vi.fn()} />,
    );
    expect(picker()).toHaveTextContent(/^Off$/);
    expect(picker()).toBeEnabled();
    await user.click(picker());
    expect(option(/^Off$/)).not.toHaveAttribute("aria-disabled");
    for (const choice of [/^Inherit/, /^On/]) {
      expect(option(choice)).toHaveAttribute("aria-disabled", "true");
      expect(option(choice)).toHaveTextContent(
        "Only a workspace admin can change this.",
      );
    }
    await user.keyboard("{Escape}");
    expect(save()).toBeDisabled();
    expect(setGithubCli).not.toHaveBeenCalled();
  });

  it("disables On with the reason under the workspace's mandatory off", async () => {
    state.environment = agentEnvironment({
      githubCliEnabled: false,
      githubCliMandatory: true,
    });
    const user = userEvent.setup();
    render(
      <SessionSettingsDialog session={session(null)} onOpenChange={vi.fn()} />,
    );
    expect(picker()).toHaveTextContent("Inherit (off)");
    await user.click(picker());
    expect(option(/^On/)).toHaveAttribute("aria-disabled", "true");
    expect(option(/^On/)).toHaveTextContent(
      "Unavailable while this workspace's GitHub CLI off is mandatory.",
    );
    expect(option(/^Off$/)).not.toHaveAttribute("aria-disabled");
  });

  it.each([
    [
      "mandatory",
      "This workspace's GitHub CLI off is mandatory, so a session can't turn gh on. Choose Off or Inherit.",
    ],
    ["admin_required", "Only a workspace admin can change this."],
  ])("shows a %s refusal in the dialog", async (reason, text) => {
    setGithubCli.mockRejectedValueOnce(
      new AgentSessionError("AGENT_SESSION_GITHUB_CLI_LOCKED", "server copy", {
        field: "agentConfig.githubCli",
        reason,
      }),
    );
    const onOpenChange = vi.fn();
    const user = userEvent.setup();
    render(
      <SessionSettingsDialog
        session={session(null)}
        onOpenChange={onOpenChange}
      />,
    );
    await user.click(picker());
    await user.click(option(/^On$/));
    await user.click(save());
    expect(await screen.findByRole("alert")).toHaveTextContent(text);
    expect(picker()).toHaveAttribute("aria-invalid", "true");
    expect(onOpenChange).not.toHaveBeenCalled();
    await user.click(picker());
    await user.click(option(/^Off$/));
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("names what an automation's session inherits through its automation", () => {
    state.automation = { githubCli: false };
    const { unmount } = render(
      <SessionSettingsDialog
        session={session(null, { automationId: "aut-1" })}
        onOpenChange={vi.fn()}
      />,
    );
    expect(picker()).toHaveTextContent("Inherit (off)");
    unmount();
    // An automation that binds nothing inherits the automations default.
    state.automation = { githubCli: null };
    state.environment = agentEnvironment({
      automationsGithubCliEnabled: false,
    });
    render(
      <SessionSettingsDialog
        session={session(null, { automationId: "aut-1" })}
        onOpenChange={vi.fn()}
      />,
    );
    expect(picker()).toHaveTextContent("Inherit (off)");
  });

  it("names what a child inherits through the sessions above it", () => {
    // The root turned gh off, so its grandchild inherits the off.
    tree.sessions = [
      session(false, { id: "ags-root" }),
      session(null, { id: "ags-mid", parentSessionId: "ags-root" }),
    ];
    const { unmount } = render(
      <SessionSettingsDialog
        session={session(null, { id: "ags-leaf", parentSessionId: "ags-mid" })}
        onOpenChange={vi.fn()}
      />,
    );
    expect(picker()).toHaveTextContent("Inherit (off)");
    unmount();
    // A root an automation started passes the automation's off down.
    state.automation = { githubCli: false };
    tree.sessions = [session(null, { id: "ags-root", automationId: "aut-1" })];
    render(
      <SessionSettingsDialog
        session={session(null, { id: "ags-leaf", parentSessionId: "ags-root" })}
        onOpenChange={vi.fn()}
      />,
    );
    expect(picker()).toHaveTextContent("Inherit (off)");
  });

  it("names no inherited value while a child's tree loads", () => {
    tree.loading = true;
    render(
      <SessionSettingsDialog
        session={session(null, { parentSessionId: "ags-parent" })}
        onOpenChange={vi.fn()}
      />,
    );
    expect(picker()).toHaveTextContent(/^Inherit$/);
  });
});
