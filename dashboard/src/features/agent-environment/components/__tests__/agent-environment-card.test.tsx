import { beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toast } from "sonner";
import { AgentModelSource, AgentNetworkTier } from "@/graphql/definitions";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
import { agentEnvironment } from "@/test/mocks/agent-environment";
import { codedGraphQLError } from "@/test/mocks/apollo";
import { AgentEnvironmentCard } from "@/features/agent-environment/components/agent-environment-card";
import type { UseAgentEnvironmentResult } from "@/features/agent-environment/hooks/use-agent-environment";
import type { AgentEnvironmentView } from "@/features/agent-environment/lib/environment";

const hook = vi.hoisted(() => ({
  current: null as unknown as UseAgentEnvironmentResult,
}));

vi.mock("@/features/agent-environment/hooks/use-agent-environment", () => ({
  useAgentEnvironment: () => hook.current,
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const ENV: AgentEnvironmentView = agentEnvironment({
  registryHosts: ["registry.npmjs.org", "pypi.org", "proxy.golang.org"],
});

function setup(env: AgentEnvironmentView | null = ENV, loading = false) {
  const update = vi.fn(async (input) => ({ ...ENV, ...input }));
  hook.current = {
    environment: env,
    loading,
    error: undefined,
    refetch: vi.fn(async () => undefined),
    update,
    saving: false,
  };
  render(<AgentEnvironmentCard ownerId="tea-1" />);
  return { update, user: userEvent.setup() };
}

const radio = (name: RegExp) => screen.getByRole("radio", { name });
const save = () => screen.getByRole("button", { name: "Save" });

describe("AgentEnvironmentCard", () => {
  beforeEach(() => {
    vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
    vi.mocked(toast.success).mockClear();
  });

  it("renders the skeleton while loading", () => {
    setup(null, true);
    expect(
      screen.getByTestId("agent-environment-skeleton"),
    ).toBeInTheDocument();
  });

  // The static copy wraps with the viewport, so the skeleton renders it as
  // real text (the ready card's own pieces) and skeletonizes only data, so
  // pending and ready keep one geometry at every width.
  it("renders the ready card's static copy in the skeleton", () => {
    setup(null, true);
    const skeleton = within(screen.getByTestId("agent-environment-skeleton"));
    for (const text of [
      "Agents",
      /How agent sessions in this workspace reach the network/,
      "Network access",
      /GitHub and the model provider are always reachable/,
      "Package registries (recommended)",
      /install dependencies from npm, PyPI/,
      "Custom",
      "Package registries plus the hostnames you list below.",
      "Full internet",
      /Any public website over HTTP\(S\)/,
      "Default model",
      "Model provider",
      /Agent sessions use the bex-provided model unless/,
      "Model source:",
      "Write-only: the key is stored securely and never shown again.",
      "Agent model spend",
      /The cap stops new model requests/,
      "Monthly spend cap (USD)",
    ]) {
      expect(skeleton.getByText(text)).toBeInTheDocument();
    }
    for (const name of [
      "Save key",
      "Save cap",
      "Reset to plan default",
      "Save",
    ]) {
      expect(skeleton.getByRole("button", { name })).toBeDisabled();
    }
    // No radio, field or other control is live before the data arrives.
    expect(skeleton.queryAllByRole("radio")).toHaveLength(0);
    expect(skeleton.queryAllByRole("textbox")).toHaveLength(0);
  });

  it("holds wrapping data lines at their longest variant, hidden from assistive tech", () => {
    setup(null, true);
    // The default-model help is the platform-pinned variant when a route pins
    // the model, which is longer than the editable variant (measured +16px).
    expect(
      screen.getByText(
        /This workspace runs agents on the platform model, so every session uses deepseek\/deepseek-v4\.1-flash-1017/,
      ),
    ).toHaveAttribute("aria-hidden", "true");
    const source = screen
      .getByTestId("model-key-section-skeleton")
      .querySelector("[data-slot=skeleton-text]");
    expect(source).toHaveTextContent(
      "bex-provided (billed at the model provider's cost + 30%)",
    );
    expect(source).toHaveAttribute("aria-hidden", "true");
    expect(source).toHaveClass("text-transparent");
    const spend = screen.getByTestId("spend-cap-section-skeleton");
    expect(spend).toHaveTextContent(/Trust tier T0: your card is charged/);
    expect(spend).toHaveTextContent(/A failed or disputed payment/);
  });

  it("renders the three selectable tiers and the registry hosts", async () => {
    const { user } = setup();
    expect(radio(/^Package registries/)).toHaveAttribute(
      "aria-checked",
      "true",
    );
    expect(radio(/^Custom/)).toBeEnabled();
    expect(radio(/Full internet/)).toBeEnabled();
    expect(screen.queryByText("Coming soon")).toBeNull();
    expect(screen.queryByLabelText("Additional hostnames")).toBeNull();
    await user.click(screen.getByText("Included package registries (3 hosts)"));
    expect(screen.getByText("pypi.org")).toBeVisible();
    expect(save()).toBeDisabled();
  });

  it("saves the Full internet tier without a hostname list", async () => {
    const { user, update } = setup();
    await user.click(radio(/Full internet/));
    expect(radio(/Full internet/)).toHaveAttribute("aria-checked", "true");
    expect(screen.getByText(/through the bex egress proxy/)).toBeVisible();
    expect(screen.queryByLabelText("Additional hostnames")).toBeNull();
    await user.click(save());
    expect(update).toHaveBeenCalledWith({
      networkTier: AgentNetworkTier.Full,
    });
  });

  it("validates custom hosts client-side before saving", async () => {
    const { user, update } = setup();
    await user.click(radio(/^Custom/));
    const hosts = screen.getByLabelText("Additional hostnames");
    await user.type(hosts, "https://api.example.com");
    expect(screen.getByRole("alert")).toHaveTextContent(
      '"https://api.example.com" isn\'t an exact hostname',
    );
    expect(save()).toBeDisabled();
    await user.clear(hosts);
    await user.type(hosts, "api.example.com{enter}api.example.com");
    expect(screen.getByRole("alert")).toHaveTextContent(
      "listed more than once",
    );
    expect(update).not.toHaveBeenCalled();
  });

  it("maps a server host refusal onto the hostname field", async () => {
    const { user, update } = setup();
    update.mockRejectedValueOnce(
      codedGraphQLError("AGENT_ENVIRONMENT_HOSTS_INVALID", {
        field: "customHosts",
        reason: "private_name",
        entry: "db.corp.example",
      }),
    );
    await user.click(radio(/^Custom/));
    await user.type(
      screen.getByLabelText("Additional hostnames"),
      "db.corp.example",
    );
    await user.click(save());
    expect(update).toHaveBeenCalledWith({
      networkTier: AgentNetworkTier.Custom,
      customHosts: ["db.corp.example"],
    });
    expect(await screen.findByRole("alert")).toHaveTextContent(
      '"db.corp.example" is a private or cluster-local name',
    );
    expect(toast.success).not.toHaveBeenCalled();
  });

  it("maps a server model refusal onto the model field", async () => {
    const { user, update } = setup();
    update.mockRejectedValueOnce(
      codedGraphQLError("AGENT_ENVIRONMENT_MODEL_INVALID", {
        field: "defaultModel",
      }),
    );
    await user.type(screen.getByLabelText("Default model"), "x");
    await user.click(save());
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Use a model id without spaces",
    );
  });

  it("saves only the changed fields and toasts", async () => {
    const { user, update } = setup({
      ...ENV,
      networkTier: AgentNetworkTier.Custom,
      customHosts: ["api.example.com"],
    });
    const model = screen.getByLabelText("Default model");
    expect(model).toHaveAttribute("placeholder", "Provider default");
    await user.type(model, "claude-sonnet-4-5");
    await user.click(save());
    expect(update).toHaveBeenCalledWith({ defaultModel: "claude-sonnet-4-5" });
    expect(toast.success).toHaveBeenCalledWith("Agent settings saved");
  });

  it("shows a pinned model read-only and never sends it", async () => {
    setup({ ...ENV, modelPinned: true, pinnedModel: "deepseek/deepseek-v4" });
    const model = screen.getByLabelText("Default model");
    expect(model).toHaveValue("deepseek/deepseek-v4");
    expect(model).toHaveAttribute("readonly");
    expect(
      screen.getByText(/runs agents on the platform model/),
    ).toBeInTheDocument();
  });

  describe("approvals (ADR100)", () => {
    it("saves a stricter approval mode and the bypass switch with the card", async () => {
      const { user, update } = setup();
      expect(radio(/^Autonomous/)).toHaveAttribute("aria-checked", "true");
      const bypass = screen.getByRole("switch", {
        name: "Members may start sessions that ask less",
      });
      expect(bypass).toBeChecked();
      expect(save()).toBeDisabled();

      await user.click(radio(/^Ask before risky actions/));
      await user.click(bypass);
      expect(save()).toBeEnabled();
      await user.click(save());
      expect(update).toHaveBeenCalledWith({
        approvalMode: "risky",
        approvalBypassAllowed: false,
      });
    });

    it("sends nothing when a change is undone", async () => {
      const { user } = setup({ ...ENV, approvalMode: "all" });
      expect(radio(/^Ask before every action/)).toHaveAttribute(
        "aria-checked",
        "true",
      );
      await user.click(radio(/^Autonomous/));
      expect(save()).toBeEnabled();
      await user.click(radio(/^Ask before every action/));
      expect(save()).toBeDisabled();
    });

    it("is read-only for a member", () => {
      vi.mocked(useCapabilities).mockReturnValue(
        mockCapabilities({ role: "DEVELOPER", canManage: false }),
      );
      setup({ ...ENV, approvalMode: "risky", approvalBypassAllowed: false });
      expect(radio(/^Ask before risky actions/)).toHaveAttribute(
        "aria-checked",
        "true",
      );
      expect(radio(/^Autonomous/)).toBeDisabled();
      expect(
        screen.getByRole("switch", {
          name: "Members may start sessions that ask less",
        }),
      ).toBeDisabled();
    });

    it("renders the approval section's static copy in the skeleton", () => {
      setup(null, true);
      const section = within(
        screen.getByTestId("approval-mode-section-skeleton"),
      );
      for (const text of [
        "Approvals",
        /When agents stop to ask you before acting/,
        "Autonomous",
        "Agents never pause for approval.",
        "Ask before risky actions",
        /Agents ask before force-pushing/,
        "Ask before every action",
        /Agents ask before every command and file change/,
        "Members may start sessions that ask less",
        /can only be as cautious as this setting or more/,
      ]) {
        expect(section.getByText(text)).toBeInTheDocument();
      }
    });
  });

  it("is read-only for a member without can_manage", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "DEVELOPER", canManage: false }),
    );
    setup({
      ...ENV,
      networkTier: AgentNetworkTier.Custom,
      customHosts: ["api.example.com"],
    });
    expect(
      screen.getByText("Only workspace admins can change agent settings."),
    ).toBeInTheDocument();
    expect(radio(/^Package registries/)).toBeDisabled();
    expect(screen.getByLabelText("Additional hostnames")).toHaveAttribute(
      "readonly",
    );
    expect(screen.getByLabelText("Default model")).toHaveAttribute("readonly");
    expect(save()).toBeDisabled();
  });

  describe("model provider key", () => {
    const keyField = () => screen.getByLabelText(/provider API key/i);
    const saveKey = () => screen.getByRole("button", { name: "Save key" });

    it("shows the bex-provided source and a write-only key field", async () => {
      const { user, update } = setup();
      expect(screen.getByTestId("model-source")).toHaveTextContent(
        "bex-provided (billed at the model provider's cost + 30%)",
      );
      expect(keyField()).toHaveAttribute("type", "password");
      // Not a login password: browsers and password managers keep out.
      expect(keyField()).toHaveAttribute("autocomplete", "new-password");
      expect(keyField()).toHaveAttribute("data-1p-ignore");
      expect(keyField()).toHaveAttribute("data-lpignore", "true");
      expect(keyField()).toHaveAttribute("data-bwignore");
      expect(keyField()).toHaveValue("");
      expect(saveKey()).toBeDisabled();
      expect(screen.queryByRole("button", { name: "Remove key" })).toBeNull();

      await user.type(keyField(), "  sk-ant-api03-secret  ");
      await user.click(saveKey());
      expect(update).toHaveBeenCalledWith({
        modelApiKey: "sk-ant-api03-secret",
      });
      // Write-only: the field is emptied once saved, never refilled.
      expect(keyField()).toHaveValue("");
      expect(toast.success).toHaveBeenCalledWith("Model key saved");
    });

    it("never prefills a saved key and offers replace and remove", async () => {
      const { user, update } = setup({
        ...ENV,
        modelKeySet: true,
        modelSource: AgentModelSource.Byo,
      });
      expect(screen.getByTestId("model-source")).toHaveTextContent(
        "Your own key",
      );
      expect(screen.getByLabelText("Replace provider API key")).toHaveValue("");
      await user.click(screen.getByRole("button", { name: "Remove key" }));
      expect(
        screen.getByText("Remove the model provider key?"),
      ).toBeInTheDocument();
      const confirm = screen
        .getAllByRole("button", { name: "Remove key" })
        .at(-1)!;
      await user.click(confirm);
      expect(update).toHaveBeenCalledWith({ clearModelApiKey: true });
    });

    it("refuses a key with inner spaces before sending it", async () => {
      const { user, update } = setup();
      await user.type(keyField(), "sk-ant two");
      expect(screen.getByRole("alert")).toHaveTextContent(
        "Enter the key without spaces",
      );
      expect(saveKey()).toBeDisabled();
      expect(update).not.toHaveBeenCalled();
    });

    it("maps a server key refusal without echoing the key", async () => {
      const { user, update } = setup();
      update.mockRejectedValueOnce(
        codedGraphQLError("AGENT_ENVIRONMENT_MODEL_KEY_INVALID", {
          field: "modelApiKey",
        }),
      );
      await user.type(keyField(), "sk-ant-rejected");
      await user.click(saveKey());
      expect(screen.getByRole("alert")).toHaveTextContent(
        "Enter the key without spaces",
      );
      expect(screen.getByRole("alert")).not.toHaveTextContent(
        "sk-ant-rejected",
      );
    });

    // Non-admins see the same layout disabled, so the ready card keeps the
    // skeleton's geometry (dashboard AGENTS.md skeleton rule).
    it("shows non-admins the status with every key control disabled", () => {
      vi.mocked(useCapabilities).mockReturnValue(
        mockCapabilities({ role: "DEVELOPER", canManage: false }),
      );
      setup({ ...ENV, modelKeySet: true, modelSource: AgentModelSource.Byo });
      expect(screen.getByTestId("model-source")).toHaveTextContent(
        "Your own key",
      );
      expect(screen.getByLabelText(/provider API key/i)).toBeDisabled();
      expect(screen.getByRole("button", { name: "Save key" })).toBeDisabled();
      expect(screen.getByRole("button", { name: "Remove key" })).toBeDisabled();
    });

    it("keeps the skeleton's model-key section in step with the ready card", () => {
      setup(null, true);
      expect(
        screen.getByTestId("model-key-section-skeleton"),
      ).toBeInTheDocument();
    });
  });

  // ADR091 D5, D6, D10.1: month-to-date spend against the cap, the pricing
  // statement, the trust tier and the admin-editable cap.
  describe("agent model spend", () => {
    const SPEND = {
      ...ENV,
      agentModelSpendMonthToDateUsd: 8.5,
      agentModelMonthlySpendCapUsd: 10,
      agentModelPlanSpendCapUsd: 20,
      agentModelMaxSpendCapUsd: 200,
      agentModelSpendCapOverrideUsd: null,
      agentModelSpendCapTierLimited: true,
      trustTier: "T0",
      trustTierLocked: false,
      nextChargeThresholdUsd: 10,
      billingBalanceDue: false,
    };
    const capField = () => screen.getByLabelText("Monthly spend cap (USD)");
    const saveCap = () => screen.getByRole("button", { name: "Save cap" });
    const resetCap = () =>
      screen.getByRole("button", { name: "Reset to plan default" });

    it("shows spend against the effective cap, the pricing and the new-workspace tier", () => {
      setup(SPEND);
      expect(screen.getByTestId("spend-summary")).toHaveTextContent(
        "$8.50 of $10.00 this month",
      );
      const meter = screen.getByRole("progressbar", {
        name: "Agent model spend",
      });
      expect(meter).toHaveAttribute("aria-valuenow", "85");
      expect(meter).toHaveAttribute("aria-valuetext", "$8.50 of $10.00");
      expect(
        screen.getByText(/Billed at the model provider's cost \+ 30%/),
      ).toBeInTheDocument();
      expect(screen.getByTestId("spend-tier")).toHaveTextContent(
        "Trust tier T0: your card is charged when agent model spend reaches $10.00.",
      );
      expect(screen.getByTestId("spend-tier-note")).toHaveTextContent(
        "New workspaces start with a $10 limit that rises after the first successful payment.",
      );
      expect(capField()).toHaveValue(20);
      expect(
        screen.getByText("Plan default $20.00. Admins can set up to $200.00."),
      ).toBeInTheDocument();
    });

    it("names a top-tier month-end charge and a payment hold", () => {
      setup({
        ...SPEND,
        trustTier: "T4",
        nextChargeThresholdUsd: null,
        trustTierLocked: false,
      });
      expect(screen.getByTestId("spend-tier")).toHaveTextContent(
        "Trust tier T4: agent model spend is charged at month end.",
      );
      // The note line stays reserved (empty) so the layout never shifts.
      expect(screen.getByTestId("spend-tier-note").textContent?.trim()).toBe(
        "",
      );
    });

    it("announces unavailable spend instead of an empty 0% bar", () => {
      setup({
        ...SPEND,
        agentModelSpendMonthToDateUsd: null,
        agentModelMonthlySpendCapUsd: null,
      });
      const meter = screen.getByRole("progressbar", {
        name: "Agent model spend",
      });
      expect(meter).not.toHaveAttribute("aria-valuenow");
      expect(meter).toHaveAttribute(
        "aria-valuetext",
        "Spend isn't available right now.",
      );
      expect(meter).toHaveAttribute("data-state", "unavailable");
    });

    it("lets an admin set the cap within 10x the plan default", async () => {
      const { user, update } = setup(SPEND);
      expect(saveCap()).toBeDisabled();
      await user.clear(capField());
      await user.type(capField(), "250");
      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent(
        "Enter an amount more than $0 and at most $200.00.",
      );
      // The field names both its help and its error.
      expect(capField().getAttribute("aria-describedby")?.split(" ")).toEqual(
        expect.arrayContaining([alert.id]),
      );
      expect(capField()).toHaveAccessibleDescription(
        /Admins can set up to \$200\.00\. Enter an amount/,
      );
      expect(saveCap()).toBeDisabled();
      await user.clear(capField());
      await user.type(capField(), "50");
      await user.click(saveCap());
      expect(update).toHaveBeenCalledWith({ agentModelMonthlySpendCapUsd: 50 });
      expect(toast.success).toHaveBeenCalledWith("Spend cap saved");
    });

    it("resets an override to the plan default", async () => {
      const { user, update } = setup({
        ...SPEND,
        agentModelSpendCapOverrideUsd: 50,
      });
      expect(capField()).toHaveValue(50);
      await user.click(resetCap());
      expect(update).toHaveBeenCalledWith({
        resetAgentModelMonthlySpendCap: true,
      });
    });

    it("maps a server cap refusal", async () => {
      const { user, update } = setup(SPEND);
      update.mockRejectedValueOnce(
        codedGraphQLError("AGENT_ENVIRONMENT_SPEND_CAP_INVALID", {
          field: "agentModelMonthlySpendCapUsd",
        }),
      );
      await user.clear(capField());
      await user.type(capField(), "30");
      await user.click(saveCap());
      expect(screen.getByRole("alert")).toHaveTextContent("at most $200.00");
    });

    it("shows non-admins the spend with every cap control disabled", () => {
      vi.mocked(useCapabilities).mockReturnValue(
        mockCapabilities({ role: "DEVELOPER", canManage: false }),
      );
      setup({ ...SPEND, agentModelSpendCapOverrideUsd: 50 });
      expect(screen.getByTestId("spend-summary")).toHaveTextContent(
        "$8.50 of $10.00",
      );
      expect(capField()).toBeDisabled();
      expect(saveCap()).toBeDisabled();
      expect(resetCap()).toBeDisabled();
    });

    it("keeps the skeleton's spend section in step with the ready card", () => {
      setup(SPEND);
      const readySection = screen.getByTestId("spend-cap-section");
      // Ready: summary, meter, tier line and its own note line, then the cap
      // field with Save (min-w-24) and Reset (min-w-48).
      expect(saveCap()).toHaveClass("min-w-24");
      expect(resetCap()).toHaveClass("min-w-48");
      const readyLines = readySection.querySelectorAll(
        ":scope > div:nth-of-type(2) > *",
      ).length;
      cleanup();

      setup(null, true);
      const skeleton = screen.getByTestId("spend-cap-section-skeleton");
      expect(
        skeleton.querySelectorAll(":scope > div:nth-of-type(2) > *"),
      ).toHaveLength(readyLines);
      const buttons = within(skeleton);
      expect(buttons.getByRole("button", { name: "Save cap" })).toHaveClass(
        "min-w-24",
      );
      expect(
        buttons.getByRole("button", { name: "Reset to plan default" }),
      ).toHaveClass("min-w-48");
    });
  });
});
