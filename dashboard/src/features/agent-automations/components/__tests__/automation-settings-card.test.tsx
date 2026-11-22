import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import i18n from "@/i18n/init";
import { AutomationSettingsCard } from "@/features/agent-automations/components/automation-settings-card";
import { agentEnvironment } from "@/test/mocks/agent-environment";
import type { AgentEnvironmentView } from "@/features/agent-environment/lib/environment";
import { automation } from "./fixtures";

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, ...props }: React.ComponentProps<"a">) => (
    <a {...props}>{children}</a>
  ),
}));

// "Created by" resolves a member through Apollo; this card test does not need it.
vi.mock("@/common/hooks/use-workspace-subject-label", () => ({
  useWorkspaceSubjectLabel: (subject: string) => subject,
}));

// The workspace's gh chain (w10/m561), for what an inheriting automation's
// sessions get.
const env = vi.hoisted(() => ({
  current: null as AgentEnvironmentView | null,
}));
vi.mock("@/features/agent-environment/hooks/use-agent-environment", () => ({
  useAgentEnvironment: () => ({
    environment: env.current,
    loading: false,
    error: undefined,
    refetch: vi.fn(),
    update: vi.fn(),
    saving: false,
  }),
}));

function withLimit(maxSpendUSD: number | null) {
  const base = automation();
  return {
    ...base,
    template: { ...base.template!, maxSpendUSD },
  };
}

// w21/039: the per-run limit goes through the same currency helper the agent
// session pages use, so it is grouped and always has two decimals.
describe("AutomationSettingsCard spend limit", () => {
  it.each([
    [0.5, "$0.50"],
    [5, "$5.00"],
    [12345.5, "$12,345.50"],
    [100000, "$100,000.00"],
  ])("renders %s as %s", (limit, text) => {
    render(<AutomationSettingsCard automation={withLimit(limit)} />);
    expect(screen.getByText(text)).toBeInTheDocument();
  });

  it("keeps the no-limit copy when none is set", () => {
    render(<AutomationSettingsCard automation={withLimit(null)} />);
    expect(screen.queryByText(/^\$/)).toBeNull();
  });
});

function withEmail(email: string) {
  return automation({
    notifications: { __typename: "AgentAutomationNotifications", email },
  });
}

/** The value beside a settings row's label. */
function rowValue(label: string) {
  return screen.getByText(label, { selector: "dt" }).nextElementSibling;
}

// w9/m343: the stored email setting, in words, in each language.
describe("AutomationSettingsCard email notifications", () => {
  afterEach(async () => {
    await i18n.changeLanguage("en");
  });

  it.each([
    ["always", "Always"],
    ["on_failure", "On failure"],
    ["on_success", "On success"],
    ["never", "Never"],
  ])("shows %s as %s", (email, text) => {
    render(<AutomationSettingsCard automation={withEmail(email)} />);
    expect(rowValue("Email notifications")).toHaveTextContent(text);
  });

  it("shows the setting in Chinese", async () => {
    await i18n.changeLanguage("zh");
    render(<AutomationSettingsCard automation={withEmail("on_failure")} />);
    expect(rowValue("邮件通知")).toHaveTextContent("仅失败时");
  });

  it("shows a value this dashboard does not know as itself", () => {
    render(<AutomationSettingsCard automation={withEmail("weekly_digest")} />);
    expect(rowValue("Email notifications")).toHaveTextContent("weekly_digest");
  });
});

// w10/m561: the automation's own gh binding, beside its agent; inherit names
// what its sessions get from the workspace's automations default, else its
// GitHub CLI switch, with a mandatory off winning over both.
describe("AutomationSettingsCard GitHub CLI", () => {
  afterEach(() => {
    env.current = null;
  });

  function withGithubCli(githubCli: boolean | null) {
    const base = automation();
    return automation({
      template: {
        ...base.template,
        agentConfig: { ...base.template.agentConfig, githubCli },
      },
    });
  }

  it.each([
    ["inherits an on switch", null, agentEnvironment(), "Inherit (on)"],
    [
      "inherits the automations default",
      null,
      agentEnvironment({ automationsGithubCliEnabled: false }),
      "Inherit (off)",
    ],
    [
      "inherits a mandatory off over the automations default",
      null,
      agentEnvironment({
        githubCliEnabled: false,
        githubCliMandatory: true,
        automationsGithubCliEnabled: true,
      }),
      "Inherit (off)",
    ],
    ["inherits before the environment loads", null, null, "Inherit"],
    ["is bound on", true, agentEnvironment(), "On"],
    ["is bound off", false, agentEnvironment(), "Off"],
  ])("%s", (_, githubCli, environment, text) => {
    env.current = environment;
    render(<AutomationSettingsCard automation={withGithubCli(githubCli)} />);
    expect(rowValue("GitHub CLI (gh)")).toHaveTextContent(
      new RegExp(`^${text.replace(/[()]/g, "\\$&")}$`),
    );
  });

  it("holds the row in the pending card", () => {
    render(<AutomationSettingsCard automation={null} />);
    expect(rowValue("GitHub CLI (gh)")).toBeInTheDocument();
  });
});
