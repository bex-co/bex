import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toast } from "sonner";
import { AgentModelSource, AgentNetworkTier } from "@/graphql/definitions";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
import { agentEnvironment } from "@/test/mocks/agent-environment";
import { codedGraphQLError, uncodedGraphQLError } from "@/test/mocks/apollo";
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
  const view = render(<AgentEnvironmentCard ownerId="tea-1" />);
  // A newer server copy, as a refetch or another admin's change brings it.
  const rerender = (next: AgentEnvironmentView) => {
    hook.current = { ...hook.current, environment: next };
    view.rerender(<AgentEnvironmentCard ownerId="tea-1" />);
  };
  return { update, user: userEvent.setup(), rerender };
}

// A save's answer reaches the card through the Apollo cache, as in the app.
function setupWithCache(env: AgentEnvironmentView = ENV) {
  const harness = setup(env);
  harness.update.mockImplementation(async (input) => {
    const next = { ...hook.current.environment!, ...input };
    harness.rerender(next);
    return next;
  });
  return harness;
}

const radio = (name: RegExp) => screen.getByRole("radio", { name });
const gh = () => screen.getByRole("switch", { name: "GitHub CLI (gh)" });
const mandatory = () =>
  screen.getByRole("switch", { name: "Make off mandatory" });
const automations = () =>
  screen.getByRole("combobox", { name: "GitHub CLI for automations" });
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

  // w7/094: a member's card opens with the Admin-only note.
  it.each([
    ["a member", false, true],
    ["an admin", true, false],
  ])("holds the read-only note for %s", (_, canManage, shown) => {
    vi.mocked(useCapabilities).mockReturnValue(mockCapabilities({ canManage }));
    setup(null, true);
    const skeleton = screen.getByTestId("agent-environment-skeleton");
    expect(
      skeleton.textContent?.includes(
        "Only workspace admins can change agent settings.",
      ),
    ).toBe(shown);
  });

  it("holds wrapping data lines at their longest variant, hidden from assistive tech", () => {
    setup(null, true);
    // The default-model help is the platform catalog's variant on the
    // platform route (w10/m516), which is longer than the editable variant.
    expect(
      screen.getByText(
        /New sessions and automations that name no model run this one/,
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
    expect(spend).toHaveTextContent(
      /Trust tier T0: billing credits cover the first .* of agent model spend this billing period; your card is charged/,
    );
    // The longer of the T0 and payment-locked explanations.
    expect(spend).toHaveTextContent(
      /New workspaces are charged when agent model spend beyond any billing credits reaches/,
    );
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

  it("offers the platform catalog as the default model and saves a choice", async () => {
    const models = ["deepseek/deepseek-v4", "anthropic/claude-opus-5.5"];
    const { user, update } = setup({ ...ENV, models, enabledModels: models });
    const model = screen.getByRole("combobox", { name: "Default model" });
    expect(model).toHaveTextContent("Platform default (deepseek/deepseek-v4)");
    expect(
      screen.getByText(/Members can still pick another model/),
    ).toBeInTheDocument();
    await user.click(model);
    expect(
      screen.getAllByRole("option").map((option) => option.textContent),
    ).toEqual([
      "Platform default (deepseek/deepseek-v4)",
      "deepseek/deepseek-v4",
      "anthropic/claude-opus-5.5",
    ]);
    await user.click(
      screen.getByRole("option", { name: "anthropic/claude-opus-5.5" }),
    );
    await user.click(save());
    expect(update).toHaveBeenCalledWith({
      defaultModel: "anthropic/claude-opus-5.5",
    });
  });

  it("names a catalog refusal on the model field", async () => {
    const models = ["deepseek/deepseek-v4", "anthropic/claude-opus-5.5"];
    const { user, update } = setup({ ...ENV, models, enabledModels: models });
    update.mockRejectedValueOnce(
      codedGraphQLError("AGENT_ENVIRONMENT_MODEL_NOT_OFFERED", {
        field: "defaultModel",
        model: "anthropic/claude-opus-5.5",
      }),
    );
    await user.click(screen.getByRole("combobox", { name: "Default model" }));
    await user.click(
      screen.getByRole("option", { name: "anthropic/claude-opus-5.5" }),
    );
    await user.click(save());
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "The platform model doesn't offer anthropic/claude-opus-5.5",
    );
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

  // w10/m519: the workspace default sandbox template, Devin's default
  // platform. The list is the server's: a made-up third template proves no
  // name or size is built in.
  describe("default sandbox", () => {
    const tpl = (name: string, cpu: number, memoryGb: number) => ({
      __typename: "AgentSandboxTemplate" as const,
      name,
      cpu,
      memoryGb,
      default: name === "agent",
    });
    const SIZED = {
      ...ENV,
      defaultTemplate: "agent",
      templates: [
        tpl("agent", 2, 4),
        tpl("agent-4c-8g", 4, 8),
        tpl("agent-8c-16g", 8, 16),
      ],
    };
    const picker = () =>
      screen.getByRole("combobox", { name: "Default sandbox" });

    it("offers the templates with their sizes and saves the one picked", async () => {
      const { user, update } = setup(SIZED);
      expect(picker()).toHaveTextContent("agent · 2 CPU, 4 GB");
      expect(picker()).toHaveAccessibleDescription(
        /New sessions and automations that name no sandbox run on this one/,
      );
      await user.click(picker());
      expect(
        screen.getAllByRole("option").map((option) => option.textContent),
      ).toEqual([
        "agent · 2 CPU, 4 GB",
        "agent-4c-8g · 4 CPU, 8 GB",
        "agent-8c-16g · 8 CPU, 16 GB",
      ]);
      await user.click(
        screen.getByRole("option", { name: "agent-8c-16g · 8 CPU, 16 GB" }),
      );
      await user.click(save());
      expect(update).toHaveBeenCalledWith({ defaultTemplate: "agent-8c-16g" });
      expect(toast.success).toHaveBeenCalledWith("Agent settings saved");
    });

    it("shows the refusal on the field and clears it on the next pick", async () => {
      const { user, update } = setup(SIZED);
      update.mockRejectedValueOnce(
        codedGraphQLError("AGENT_ENVIRONMENT_TEMPLATE_INVALID", {
          field: "defaultTemplate",
          template: "agent-8c-16g",
          templates: ["agent", "agent-4c-8g"],
        }),
      );
      await user.click(picker());
      await user.click(
        screen.getByRole("option", { name: "agent-8c-16g · 8 CPU, 16 GB" }),
      );
      await user.click(save());
      expect(await screen.findByRole("alert")).toHaveTextContent(
        "agent-8c-16g isn't a sandbox template. Pick one of: agent, agent-4c-8g.",
      );
      expect(picker()).toHaveAttribute("aria-invalid", "true");
      expect(toast.success).not.toHaveBeenCalled();
      await user.click(picker());
      await user.click(
        screen.getByRole("option", { name: "agent-4c-8g · 4 CPU, 8 GB" }),
      );
      expect(screen.queryByRole("alert")).toBeNull();
    });

    it("is read-only for a member without can_manage", () => {
      vi.mocked(useCapabilities).mockReturnValue(
        mockCapabilities({ role: "DEVELOPER", canManage: false }),
      );
      setup(SIZED);
      expect(picker()).toBeDisabled();
      expect(picker()).toHaveTextContent("agent · 2 CPU, 4 GB");
    });

    it("keeps the skeleton's default-sandbox section in step with the ready card", () => {
      setup(SIZED);
      const ready = screen.getByTestId("default-template-section");
      const readyRows = ready.children.length;
      // A 36px (h-9) trigger, full width up to max-w-sm.
      expect(picker()).toHaveAttribute("data-size", "default");
      expect(picker()).toHaveClass("w-full", "max-w-sm");
      cleanup();

      setup(null, true);
      const skeleton = screen.getByTestId("default-template-section-skeleton");
      expect(skeleton.className).toBe(ready.className);
      // Label, control, help: the same three rows, the control the same box.
      expect(skeleton.children).toHaveLength(readyRows);
      expect(skeleton.children[1]).toHaveClass("h-9", "w-full", "max-w-sm");
      expect(skeleton).toHaveTextContent("Default sandbox");
      expect(skeleton).toHaveTextContent(
        /New sessions and automations that name no sandbox run on this one/,
      );
    });
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
        "Trust tier T0: billing credits pay agent model spend first; your card is charged when the spend they leave reaches $10.00.",
      );
      expect(screen.getByTestId("spend-tier-note")).toHaveTextContent(
        "New workspaces are charged when agent model spend beyond any billing credits reaches $10.00: the card is charged right then, sessions keep running once it succeeds, and the limit rises to your plan's cap.",
      );
      expect(capField()).toHaveValue(20);
      expect(
        screen.getByText("Plan default $20.00. Admins can set up to $200.00."),
      ).toBeInTheDocument();
    });

    it("says what billing credits cover before the card is charged (w6/m400)", () => {
      setup({ ...SPEND, agentModelCreditUsd: 25 });
      expect(screen.getByTestId("spend-tier")).toHaveTextContent(
        "Trust tier T0: billing credits cover the first $25.00 of agent model spend this billing period; your card is charged when spend beyond them reaches $10.00.",
      );
    });

    it("meters the limit the mint holds once credits move the spend (w6/131)", () => {
      // A T0 workspace bex cannot charge: the $10 ceiling counts the $8
      // credits leave, not the $15 spent this month.
      setup({
        ...SPEND,
        agentModelSpendMonthToDateUsd: 15,
        agentModelUnbilledUsd: 8,
        agentModelCreditUsd: 7,
        agentModelChargesAtThreshold: false,
      });
      expect(screen.getByTestId("spend-summary")).toHaveTextContent(
        "$8.00 of $10.00 beyond billing credits, $15.00 this month",
      );
      const meter = screen.getByRole("progressbar", {
        name: "Agent model spend",
      });
      expect(meter).toHaveAttribute("aria-valuenow", "80");
      expect(meter).toHaveAttribute("data-state", "warning");
    });

    it("meters the workspace's own cap when credits cover the ceiling (w6/131)", () => {
      setup({
        ...SPEND,
        agentModelSpendMonthToDateUsd: 9,
        agentModelUnbilledUsd: 0,
        agentModelCreditUsd: 1_000_000,
        agentModelChargesAtThreshold: false,
      });
      expect(screen.getByTestId("spend-summary")).toHaveTextContent(
        "$9.00 of $20.00 this month",
      );
      const meter = screen.getByRole("progressbar", {
        name: "Agent model spend",
      });
      expect(meter).toHaveAttribute("aria-valuenow", "45");
      expect(meter).toHaveAttribute("aria-valuetext", "$9.00 of $20.00");
      expect(meter).toHaveAttribute("data-state", "ok");
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

    // w9/144: the spend is a billing detail. A Contributor and a Viewer read
    // the role's reason in its place and no dollar figure, even from a copy
    // that still carries the figures, in the same lines as everyone else.
    it.each(["CONTRIBUTOR", "VIEWER"] as const)(
      "shows a %s the role's reason and no spend figure",
      (role) => {
        vi.mocked(useCapabilities).mockReturnValue(
          mockCapabilities({
            role,
            canManage: false,
            canCreate: false,
            canViewBilling: false,
            canManageBilling: false,
          }),
        );
        setup({ ...SPEND, agentModelSpendCapOverrideUsd: 50 });
        const section = screen.getByTestId("spend-cap-section");
        expect(screen.getByTestId("spend-summary")).toHaveTextContent(
          "Your role can’t view billing details. Ask a workspace admin.",
        );
        expect(section.textContent).not.toMatch(/\$|T0/);
        const meter = screen.getByRole("progressbar", {
          name: "Agent model spend",
        });
        expect(meter).not.toHaveAttribute("aria-valuenow");
        expect(meter.getAttribute("aria-valuetext")).not.toMatch(/\$/);
        expect(capField()).toBeDisabled();
        expect(capField()).toHaveValue(null);
        expect(saveCap()).toBeDisabled();
        expect(resetCap()).toBeDisabled();
        const readyLines = Array.from(
          section.querySelectorAll(":scope > div:nth-of-type(2) > p"),
          (line) => line.textContent,
        );
        cleanup();

        // The skeleton draws the same reason and the same blank lines.
        setup(null, true);
        const skeleton = screen.getByTestId("spend-cap-section-skeleton");
        expect(
          Array.from(
            skeleton.querySelectorAll(":scope > div:nth-of-type(2) > p"),
            (line) => line.textContent,
          ),
        ).toEqual(readyLines);
        expect(skeleton.textContent).not.toMatch(/\$|T0/);
      },
    );

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

    // ADR091 D8: the pricing statement links the public terms, and the
    // skeleton holds the same "Terms" text so the description wraps alike.
    it("links the agent model terms from the pricing statement", () => {
      setup(SPEND);
      const link = within(screen.getByTestId("spend-cap-section")).getByRole(
        "link",
        { name: "Terms" },
      );
      expect(link).toHaveAttribute(
        "href",
        "https://bex.co/terms-of-service#agent-model",
      );
      expect(link).toHaveAttribute("target", "_blank");
      cleanup();

      setup(null, true);
      const skeleton = within(screen.getByTestId("spend-cap-section-skeleton"));
      expect(skeleton.getByText("Terms")).toBeInTheDocument();
      expect(skeleton.queryByRole("link")).toBeNull();
    });
  });

  // w10/m524: the sessions' gh, a Devin security profile's gh API access.
  // It saves on toggle, as the available-agents switches do.
  describe("GitHub CLI", () => {
    it.each([true, false])("shows the stored value (%s)", (enabled) => {
      setup({ ...ENV, githubCliEnabled: enabled });
      expect(gh()).toHaveAttribute("aria-checked", String(enabled));
      expect(gh()).toBeEnabled();
      expect(gh()).toHaveAccessibleDescription(
        "Turning it off removes gh from every agent session in this workspace, running ones included; Git pushes to the session branch and bex's pull request tools keep working.",
      );
    });

    it("lets an admin turn gh off and back on", async () => {
      const { user, update } = setupWithCache();
      await user.click(gh());
      expect(update).toHaveBeenLastCalledWith({ githubCliEnabled: false });
      expect(gh()).not.toBeChecked();
      expect(toast.success).toHaveBeenLastCalledWith(
        "GitHub CLI turned off for agent sessions",
      );
      // It is not part of the draft the Save button sends.
      expect(save()).toBeDisabled();
      await user.click(gh());
      expect(update).toHaveBeenLastCalledWith({ githubCliEnabled: true });
      expect(gh()).toBeChecked();
      expect(toast.success).toHaveBeenLastCalledWith(
        "GitHub CLI turned on for agent sessions",
      );
      expect(update).toHaveBeenCalledTimes(2);
    });

    it("holds the switch until the server answers, then follows newer copies", async () => {
      const { user, update, rerender } = setupWithCache();
      let answer!: () => void;
      update.mockImplementationOnce(
        (input) =>
          new Promise<AgentEnvironmentView>((resolve) => {
            answer = () => {
              const next = { ...ENV, ...input };
              rerender(next);
              resolve(next);
            };
          }),
      );
      await user.click(gh());
      expect(gh()).not.toBeChecked();
      expect(gh()).toBeDisabled();
      await act(async () => answer());
      expect(gh()).not.toBeChecked();
      expect(gh()).toBeEnabled();
      // Another admin turned it back on: the next read wins.
      rerender({ ...ENV, githubCliEnabled: true });
      expect(gh()).toBeChecked();
    });

    it("shows the stored value again and says why when a save fails", async () => {
      const { user, update } = setupWithCache();
      update.mockRejectedValueOnce(uncodedGraphQLError("boom"));
      await user.click(gh());
      expect(update).toHaveBeenCalledWith({ githubCliEnabled: false });
      expect(await screen.findByRole("alert")).toHaveTextContent(
        "Couldn't change the GitHub CLI setting. Try again.",
      );
      expect(gh()).toBeChecked();
      expect(gh()).toBeEnabled();
      expect(toast.success).not.toHaveBeenCalled();
      // A refusal for a caller no longer an admin reads as the card's other
      // fields do.
      update.mockRejectedValueOnce(codedGraphQLError("FORBIDDEN"));
      await user.click(gh());
      expect(screen.getAllByRole("alert")).toHaveLength(1);
      expect(screen.getByRole("alert")).toHaveTextContent(
        "Only workspace admins can change agent settings.",
      );
      expect(gh()).toBeChecked();
    });

    it.each([true, false])(
      "is disabled for a member without can_manage (stored %s)",
      async (enabled) => {
        vi.mocked(useCapabilities).mockReturnValue(
          mockCapabilities({ role: "DEVELOPER", canManage: false }),
        );
        const { user, update } = setup({ ...ENV, githubCliEnabled: enabled });
        expect(gh()).toBeDisabled();
        expect(gh()).toHaveAttribute("aria-checked", String(enabled));
        await user.click(gh());
        expect(update).not.toHaveBeenCalled();
      },
    );

    it("keeps the skeleton's GitHub CLI rows in step with the ready card", () => {
      // Each row's frame and the boxes inside it, in order: the workspace
      // switch, the mandatory flag and the automations default (w10/m561).
      const shape = (section: HTMLElement) => ({
        // Right after the network tier, in the same card slot.
        slot: Array.from(section.parentElement!.children).indexOf(section),
        section: section.className,
        rows: Array.from(section.children).map((row) => ({
          row: row.className,
          children: Array.from(row.children).map((child) => ({
            tag: child.tagName,
            className: child.className,
            children: child.children.length,
          })),
        })),
      });
      setup();
      const section = screen.getByTestId("github-cli-section");
      const ready = shape(section);
      // The switches are h-5 w-9 and the select h-9; the skeleton holds them.
      expect(gh()).toHaveClass("h-5", "w-9", "shrink-0");
      expect(mandatory()).toHaveClass("h-5", "w-9", "shrink-0");
      expect(automations()).toHaveAttribute("data-size", "default");
      expect(automations()).toHaveClass("w-full", "max-w-sm");
      cleanup();

      setup(null, true);
      const skeleton = screen.getByTestId("github-cli-section-skeleton");
      const pending = shape(skeleton);
      expect(pending.slot).toBe(ready.slot);
      expect(pending.section).toBe(ready.section);
      expect(pending.rows.map((row) => row.row)).toEqual(
        ready.rows.map((row) => row.row),
      );
      // The text column and the help match; the control is a placeholder of
      // the same box, not the control.
      expect(pending.rows.map((row) => row.children.length)).toEqual(
        ready.rows.map((row) => row.children.length),
      );
      expect(pending.rows[0]!.children[0]).toEqual(ready.rows[0]!.children[0]);
      expect(pending.rows[1]!.children[0]).toEqual(ready.rows[1]!.children[0]);
      expect(pending.rows[2]!.children[2]).toEqual(ready.rows[2]!.children[2]);
      const [ghBox, mandatoryBox] = [0, 1].map(
        (i) => skeleton.children[i]!.lastElementChild,
      );
      for (const box of [ghBox, mandatoryBox]) {
        expect(box).toHaveClass("h-5", "w-9", "shrink-0", "rounded-full");
      }
      expect(skeleton.children[2]!.children[1]).toHaveClass(
        "h-9",
        "w-full",
        "max-w-sm",
      );
      for (const text of [
        "GitHub CLI (gh)",
        /removes gh from every agent session in this workspace/,
        "Make off mandatory",
        "Takes effect while GitHub CLI is off: no automation or session can then turn gh back on.",
        "GitHub CLI for automations",
        /unless an automation sets its own/,
      ]) {
        expect(within(skeleton).getByText(text)).toBeInTheDocument();
      }
      expect(within(skeleton).queryByRole("switch")).toBeNull();
      expect(within(skeleton).queryByRole("combobox")).toBeNull();
    });
  });

  // w10/m561: Devin's mandatory profile and automations default, beside the
  // workspace's switch. Each saves on change; members see them disabled.
  describe("GitHub CLI mandatory off and automations default", () => {
    it("shows what is stored", () => {
      setup({
        ...ENV,
        githubCliEnabled: false,
        githubCliMandatory: true,
        automationsGithubCliEnabled: false,
      });
      expect(mandatory()).toBeChecked();
      expect(mandatory()).toBeEnabled();
      expect(mandatory()).toHaveAccessibleDescription(
        "Takes effect while GitHub CLI is off: no automation or session can then turn gh back on.",
      );
      expect(automations()).toHaveTextContent(/^Off$/);
      expect(automations()).toBeEnabled();
      expect(automations()).toHaveAccessibleDescription(
        "The gh of the sessions automations start, unless an automation sets its own. Inherit follows the switch above.",
      );
    });

    it("lets an admin make the off mandatory while gh is on, and lift it", async () => {
      const { user, update } = setupWithCache({
        ...ENV,
        githubCliEnabled: true,
      });
      expect(mandatory()).not.toBeChecked();
      await user.click(mandatory());
      expect(update).toHaveBeenLastCalledWith({ githubCliMandatory: true });
      expect(mandatory()).toBeChecked();
      expect(toast.success).toHaveBeenLastCalledWith(
        "GitHub CLI off is now mandatory",
      );
      await user.click(mandatory());
      expect(update).toHaveBeenLastCalledWith({ githubCliMandatory: false });
      expect(toast.success).toHaveBeenLastCalledWith(
        "GitHub CLI off is no longer mandatory",
      );
      // Neither the workspace switch nor the Save button's draft moved.
      expect(gh()).toBeChecked();
      expect(save()).toBeDisabled();
    });

    it("saves the automations default: on and off bind, inherit resets", async () => {
      const { user, update, rerender } = setup();
      update.mockImplementation(async (input) => {
        const next = {
          ...hook.current.environment!,
          automationsGithubCliEnabled: input.resetAutomationsGithubCli
            ? null
            : (input.automationsGithubCliEnabled ?? null),
        };
        rerender(next);
        return next;
      });
      expect(automations()).toHaveTextContent("Inherit (on)");
      await user.click(automations());
      expect(
        screen.getAllByRole("option").map((option) => option.textContent),
      ).toEqual(["Inherit (on)", "On", "Off"]);
      await user.click(screen.getByRole("option", { name: "Off" }));
      expect(update).toHaveBeenLastCalledWith({
        automationsGithubCliEnabled: false,
      });
      expect(automations()).toHaveTextContent(/^Off$/);
      expect(toast.success).toHaveBeenLastCalledWith(
        "GitHub CLI turned off for automations",
      );

      await user.click(automations());
      await user.click(screen.getByRole("option", { name: "On" }));
      expect(update).toHaveBeenLastCalledWith({
        automationsGithubCliEnabled: true,
      });
      expect(toast.success).toHaveBeenLastCalledWith(
        "GitHub CLI turned on for automations",
      );

      // Inherit is the explicit null the GraphQL input cannot carry.
      await user.click(automations());
      await user.click(screen.getByRole("option", { name: "Inherit (on)" }));
      expect(update).toHaveBeenLastCalledWith({
        resetAutomationsGithubCli: true,
      });
      expect(automations()).toHaveTextContent("Inherit (on)");
      expect(toast.success).toHaveBeenLastCalledWith(
        "Automations now inherit the GitHub CLI setting",
      );
      expect(update).toHaveBeenCalledTimes(3);
      expect(save()).toBeDisabled();
    });

    it("locks On with the reason while the workspace's off is mandatory", async () => {
      const { user, update } = setup({
        ...ENV,
        githubCliEnabled: false,
        githubCliMandatory: true,
      });
      expect(automations()).toHaveTextContent("Inherit (off)");
      await user.click(automations());
      const on = screen.getByRole("option", { name: /^On/ });
      expect(on).toHaveAttribute("aria-disabled", "true");
      expect(on).toHaveTextContent(
        "Unavailable while this workspace's GitHub CLI off is mandatory.",
      );
      await user.click(screen.getByRole("option", { name: "Off" }));
      expect(update).toHaveBeenLastCalledWith({
        automationsGithubCliEnabled: false,
      });
    });

    it("shows the server's lock under the automations control and the stored value again", async () => {
      const { user, update } = setupWithCache();
      update.mockRejectedValueOnce(
        codedGraphQLError("AGENT_ENVIRONMENT_GITHUB_CLI_LOCKED", {
          field: "automationsGithubCliEnabled",
          reason: "mandatory",
        }),
      );
      await user.click(automations());
      await user.click(screen.getByRole("option", { name: "On" }));
      const alert = await screen.findByRole("alert");
      expect(alert).toHaveTextContent(
        "GitHub CLI off is mandatory, so automations can't turn gh on.",
      );
      expect(automations()).toHaveAttribute("aria-invalid", "true");
      expect(automations()).toHaveAccessibleDescription(
        /automations can't turn gh on.*unless an automation sets its own/,
      );
      expect(automations()).toHaveTextContent("Inherit (on)");
      expect(toast.success).not.toHaveBeenCalled();
    });

    it("says why a mandatory switch failed to save", async () => {
      const { user, update } = setupWithCache();
      update.mockRejectedValueOnce(uncodedGraphQLError("boom"));
      await user.click(mandatory());
      expect(await screen.findByRole("alert")).toHaveTextContent(
        "Couldn't change whether GitHub CLI off is mandatory. Try again.",
      );
      expect(mandatory()).not.toBeChecked();
    });

    it("disables every new control for a member without can_manage", async () => {
      vi.mocked(useCapabilities).mockReturnValue(
        mockCapabilities({ role: "DEVELOPER", canManage: false }),
      );
      const { user, update } = setup({
        ...ENV,
        githubCliMandatory: true,
        automationsGithubCliEnabled: true,
      });
      expect(gh()).toBeDisabled();
      expect(mandatory()).toBeDisabled();
      expect(mandatory()).toBeChecked();
      expect(automations()).toBeDisabled();
      expect(automations()).toHaveTextContent(/^On$/);
      await user.click(mandatory());
      await user.click(automations());
      expect(screen.queryByRole("option")).toBeNull();
      expect(update).not.toHaveBeenCalled();
    });
  });

  // w10/m527: the catalog models members can choose, Devin's allowlisted
  // models. Each switch saves the whole list, as the available agents do.
  describe("models members can choose", () => {
    const CATALOG = [
      "deepseek/deepseek-v4.1-flash-1017",
      "anthropic/claude-haiku-5.5",
      "anthropic/claude-sonnet-5.5",
      "anthropic/claude-opus-5.5",
    ];
    // Devin's "Allow all models" (w10/m592), every workspace's default.
    const ROUTED = { ...ENV, models: CATALOG, enabledModels: CATALOG };
    // An allowlist (w10/m568), here of every model: what "By model" stores.
    const LISTED = { ...ROUTED, allowAllModels: false };
    const modelSwitch = (name: string) => screen.getByRole("switch", { name });
    const switches = () =>
      within(screen.getByTestId("enabled-models-section")).getAllByRole(
        "switch",
      );
    const picker = () =>
      screen.getByRole("combobox", { name: "Default model" });

    const filter = (name: "All models" | "By model" | "By provider") =>
      screen.getByRole("radio", { name });
    const filterChoice = () =>
      screen.getByRole("radiogroup", {
        name: "Filter the models members can choose",
      });
    const ALL_HELP =
      "Members can pick every platform model, including models added to the platform later. To turn models off, filter by model or by provider.";
    const BY_MODEL_HELP =
      "Members can pick only the models turned on here. A model added to the platform later stays off until you turn it on, even when every model here is on.";
    // A save answers as bex-api does: "Allow all models" lists every model
    // and none new (w10/m592), false alone keeps the models on now as an
    // allowlist, a filter by provider replaces the list (w10/m573), and a
    // list keeps the mode.
    const setupAsServer = (env: AgentEnvironmentView) => {
      const harness = setup(env);
      harness.update.mockImplementation(async (input) => {
        const current = hook.current.environment!;
        let next: AgentEnvironmentView;
        if (input.allowAllModels) {
          next = {
            ...current,
            allowAllModels: true,
            enabledProviders: null,
            enabledModels: current.models,
            newModels: [],
          };
        } else if (input.enabledProviders) {
          const providers: string[] = input.enabledProviders;
          next = {
            ...current,
            allowAllModels: false,
            enabledProviders: providers,
            enabledModels: current.models.filter(
              (m, i) => i === 0 || providers.includes(m.split("/")[0]),
            ),
            newModels: [],
          };
        } else if (input.enabledModels) {
          const list: string[] = input.enabledModels;
          next = {
            ...current,
            allowAllModels:
              current.allowAllModels && list.length === current.models.length,
            enabledProviders: null,
            enabledModels: list,
            newModels: current.newModels.filter((m) => !list.includes(m)),
          };
        } else {
          next = { ...current, allowAllModels: false };
        }
        harness.rerender(next);
        return next;
      });
      return harness;
    };

    it("lists the catalog with a switch per model, on while members may choose it", () => {
      setup({ ...LISTED, enabledModels: [CATALOG[0], CATALOG[2]] });
      const section = screen.getByTestId("enabled-models-section");
      expect(section).toHaveTextContent("Models members can choose");
      expect(section).toHaveTextContent(
        /runs the default model from its next turn/,
      );
      expect(switches().map((s) => s.getAttribute("aria-checked"))).toEqual([
        "true",
        "false",
        "true",
        "false",
      ]);
      expect(
        within(section)
          .getAllByRole("listitem")
          .map((row) => row.querySelector("label")!.textContent),
      ).toEqual(CATALOG);
      for (const model of CATALOG.slice(1)) {
        expect(modelSwitch(model)).toBeEnabled();
      }
    });

    it("keeps the catalog default on and locked, saying why", async () => {
      const { user, update } = setup({ ...LISTED, enabledModels: [] });
      const locked = modelSwitch(CATALOG[0]);
      expect(locked).toBeChecked();
      expect(locked).toBeDisabled();
      expect(locked).toHaveAccessibleDescription(
        "The platform's default model always stays on, as sessions fall back to it.",
      );
      await user.click(locked);
      expect(update).not.toHaveBeenCalled();
    });

    // w10/m568: once a model is off the list is an allowlist, so a model
    // added to the catalog later is off and labelled new until a switch
    // turns it on, apart from the ones an admin turned off.
    it("labels only a model off because it is new in the catalog", () => {
      setup({
        ...LISTED,
        enabledModels: [CATALOG[0], CATALOG[1]],
        newModels: [CATALOG[3]],
      });
      const section = screen.getByTestId("enabled-models-section");
      expect(section).toHaveTextContent(BY_MODEL_HELP);
      expect(modelSwitch(CATALOG[3])).not.toBeChecked();
      expect(modelSwitch(CATALOG[3])).toBeEnabled();
      expect(modelSwitch(CATALOG[3])).toHaveAccessibleDescription(
        "New in the catalog",
      );
      for (const model of CATALOG.slice(1, 3)) {
        expect(modelSwitch(model)).not.toHaveAccessibleDescription(
          "New in the catalog",
        );
      }
      expect(within(section).getAllByText("New in the catalog")).toHaveLength(
        1,
      );
    });

    it("turns a model new in the catalog on into the list, dropping its label", async () => {
      const NEW = {
        ...LISTED,
        enabledModels: [CATALOG[0], CATALOG[1]],
        newModels: [CATALOG[3]],
      };
      const { user, update, rerender } = setupWithCache(NEW);
      // The server answers with the model on and no longer new.
      let answer!: () => void;
      update.mockImplementationOnce(
        () =>
          new Promise<AgentEnvironmentView>((resolve) => {
            answer = () => {
              const next = {
                ...NEW,
                enabledModels: [CATALOG[0], CATALOG[1], CATALOG[3]],
                newModels: [],
              };
              rerender(next);
              resolve(next);
            };
          }),
      );
      await user.click(modelSwitch(CATALOG[3]));
      expect(update).toHaveBeenLastCalledWith({
        enabledModels: [CATALOG[0], CATALOG[1], CATALOG[3]],
      });
      // On while it saves, so no longer labelled new.
      expect(modelSwitch(CATALOG[3])).toBeChecked();
      expect(screen.queryByText("New in the catalog")).toBeNull();
      await act(async () => answer());
      expect(modelSwitch(CATALOG[3])).toBeChecked();
      expect(screen.queryByText("New in the catalog")).toBeNull();
      expect(toast.success).toHaveBeenLastCalledWith(
        "anthropic/claude-opus-5.5 turned on for members",
      );
    });

    it("lets an admin turn a model off and back on, saving the whole list", async () => {
      const { user, update } = setupWithCache(LISTED);
      await user.click(modelSwitch(CATALOG[3]));
      expect(update).toHaveBeenLastCalledWith({
        enabledModels: CATALOG.slice(0, 3),
      });
      expect(modelSwitch(CATALOG[3])).not.toBeChecked();
      expect(toast.success).toHaveBeenLastCalledWith(
        "anthropic/claude-opus-5.5 turned off for members",
      );
      // It is not part of the draft the Save button sends.
      expect(save()).toBeDisabled();
      await user.click(modelSwitch(CATALOG[1]));
      expect(update).toHaveBeenLastCalledWith({
        enabledModels: [CATALOG[0], CATALOG[2]],
      });
      await user.click(modelSwitch(CATALOG[3]));
      // Back on, in its catalog slot.
      expect(update).toHaveBeenLastCalledWith({
        enabledModels: [CATALOG[0], CATALOG[2], CATALOG[3]],
      });
      expect(modelSwitch(CATALOG[3])).toBeChecked();
      expect(toast.success).toHaveBeenLastCalledWith(
        "anthropic/claude-opus-5.5 turned on for members",
      );
      expect(update).toHaveBeenCalledTimes(3);
    });

    it("holds the switches until the server answers, then shows its answer and newer copies", async () => {
      const { user, update, rerender } = setupWithCache(LISTED);
      let answer!: () => void;
      update.mockImplementationOnce(
        () =>
          new Promise<AgentEnvironmentView>((resolve) => {
            answer = () => {
              // Another admin turned sonnet off meanwhile: the answer says so.
              const next = { ...LISTED, enabledModels: [CATALOG[0]] };
              rerender(next);
              resolve(next);
            };
          }),
      );
      await user.click(modelSwitch(CATALOG[1]));
      expect(modelSwitch(CATALOG[1])).not.toBeChecked();
      expect(modelSwitch(CATALOG[2])).toBeChecked();
      for (const s of switches()) expect(s).toBeDisabled();
      await act(async () => answer());
      expect(switches().map((s) => s.getAttribute("aria-checked"))).toEqual([
        "true",
        "false",
        "false",
        "false",
      ]);
      expect(modelSwitch(CATALOG[1])).toBeEnabled();
      // The next read wins.
      rerender(LISTED);
      expect(modelSwitch(CATALOG[2])).toBeChecked();
    });

    it("returns the switches to the saved list and says why when a save fails", async () => {
      const { user, update } = setupWithCache(LISTED);
      update.mockRejectedValueOnce(uncodedGraphQLError("boom"));
      await user.click(modelSwitch(CATALOG[2]));
      expect(update).toHaveBeenCalledWith({
        enabledModels: [CATALOG[0], CATALOG[1], CATALOG[3]],
      });
      expect(await screen.findByRole("alert")).toHaveTextContent(
        "Couldn't update the models members can choose. Try again.",
      );
      expect(modelSwitch(CATALOG[2])).toBeChecked();
      expect(modelSwitch(CATALOG[2])).toBeEnabled();
      expect(toast.success).not.toHaveBeenCalled();
      // The server's list refusal (the catalog changed since the card loaded).
      update.mockRejectedValueOnce(
        codedGraphQLError("AGENT_ENVIRONMENT_MODELS_INVALID", {
          field: "enabledModels",
          model: CATALOG[2],
        }),
      );
      await user.click(modelSwitch(CATALOG[2]));
      expect(screen.getAllByRole("alert")).toHaveLength(1);
      expect(screen.getByRole("alert")).toHaveTextContent(
        /the platform's default model has to stay on/,
      );
      expect(modelSwitch(CATALOG[2])).toBeChecked();
      // A caller no longer an admin reads as the card's other fields do.
      update.mockRejectedValueOnce(codedGraphQLError("FORBIDDEN"));
      await user.click(modelSwitch(CATALOG[2]));
      expect(screen.getByRole("alert")).toHaveTextContent(
        "Only workspace admins can change agent settings.",
      );
      expect(modelSwitch(CATALOG[2])).toBeChecked();
    });

    it("shows a member the list read-only", async () => {
      vi.mocked(useCapabilities).mockReturnValue(
        mockCapabilities({ role: "DEVELOPER", canManage: false }),
      );
      const { user, update } = setup({
        ...LISTED,
        enabledModels: [CATALOG[0], CATALOG[3]],
      });
      expect(switches().map((s) => s.getAttribute("aria-checked"))).toEqual([
        "true",
        "false",
        "false",
        "true",
      ]);
      for (const s of switches()) expect(s).toBeDisabled();
      await user.click(modelSwitch(CATALOG[1]));
      expect(update).not.toHaveBeenCalled();
    });

    it("draws no list on the workspace's own key", () => {
      setup({
        ...ENV,
        modelKeySet: true,
        modelSource: AgentModelSource.Byo,
        models: [],
        enabledModels: [],
      });
      expect(screen.queryByTestId("enabled-models-section")).toBeNull();
      expect(screen.getByLabelText("Default model")).toHaveAttribute(
        "placeholder",
        "Provider default",
      );
    });

    it("offers only the models that are on as the default model", async () => {
      const { user } = setup({
        ...LISTED,
        enabledModels: [CATALOG[0], CATALOG[2]],
      });
      await user.click(picker());
      expect(
        screen.getAllByRole("option").map((option) => option.textContent),
      ).toEqual([
        "Platform default (deepseek/deepseek-v4.1-flash-1017)",
        CATALOG[0],
        CATALOG[2],
      ]);
      expect(screen.queryByRole("note")).toBeNull();
    });

    it("keeps a saved default that is off, saying the catalog default runs instead", async () => {
      const { user, update } = setup({
        ...LISTED,
        enabledModels: [CATALOG[0], CATALOG[1]],
        defaultModel: CATALOG[3],
      });
      expect(picker()).toHaveTextContent(
        "anthropic/claude-opus-5.5 (turned off)",
      );
      const note =
        "anthropic/claude-opus-5.5 is turned off, so sessions and automations that name no model run the platform default, deepseek/deepseek-v4.1-flash-1017, until an admin picks a model that is on or turns anthropic/claude-opus-5.5 back on.";
      expect(screen.getByRole("note")).toHaveTextContent(note);
      // The picker names the note beside its help.
      expect(picker().getAttribute("aria-describedby")?.split(" ")).toContain(
        screen.getByRole("note").id,
      );
      await user.click(picker());
      expect(
        screen.getAllByRole("option").map((option) => option.textContent),
      ).toEqual([
        "Platform default (deepseek/deepseek-v4.1-flash-1017)",
        CATALOG[0],
        CATALOG[1],
        "anthropic/claude-opus-5.5 (turned off)",
      ]);
      // Picking a model that is on clears the note; saving sends it.
      await user.click(screen.getByRole("option", { name: CATALOG[1] }));
      expect(screen.queryByRole("note")).toBeNull();
      await user.click(save());
      expect(update).toHaveBeenCalledWith({ defaultModel: CATALOG[1] });
    });

    it("says so when an admin turns off the model the default names", async () => {
      const { user } = setupWithCache({ ...LISTED, defaultModel: CATALOG[2] });
      expect(picker()).toHaveTextContent(CATALOG[2]);
      expect(screen.queryByRole("note")).toBeNull();
      await user.click(modelSwitch(CATALOG[2]));
      expect(picker()).toHaveTextContent(
        "anthropic/claude-sonnet-5.5 (turned off)",
      );
      expect(screen.getByRole("note")).toHaveTextContent(
        /run the platform default, deepseek\/deepseek-v4\.1-flash-1017, until an admin/,
      );
      // Nothing to save: the stored default stands, it just does not apply.
      expect(save()).toBeDisabled();
    });

    it("says a saved default the catalog dropped is not running either", () => {
      setup({ ...ROUTED, defaultModel: "anthropic/claude-opus-4.1" });
      expect(picker()).toHaveTextContent("anthropic/claude-opus-4.1");
      expect(screen.getByRole("note")).toHaveTextContent(
        "anthropic/claude-opus-4.1 is no longer in the platform catalog, so sessions and automations that name no model run the platform default, deepseek/deepseek-v4.1-flash-1017, until an admin picks another model.",
      );
    });

    it("keeps the skeleton's model list in step with the ready card", () => {
      const shape = (section: HTMLElement) => {
        const list = section.querySelector("ul")!;
        const rows = Array.from(list.children);
        const text = (row: Element) => row.firstElementChild!;
        return {
          // Right after the default model, in the same card slot.
          slot: Array.from(section.parentElement!.children).indexOf(section),
          section: section.className,
          children: section.children.length,
          heading: section.firstElementChild!.className,
          list: list.className,
          row: rows.map((row) => row.className),
          text: rows.map((row) => text(row).className),
          // The label, and under the catalog default its locked note.
          textRows: rows.map((row) => text(row).children.length),
          label: rows.map((row) => text(row).firstElementChild!.className),
          note: text(rows[0]).lastElementChild!.textContent,
          // The filter choice and the help under it (w10/m573).
          filter: section.children[1].className,
          filterOptions: Array.from(section.children[1].children).map(
            (option) => option.textContent,
          ),
          help: section.children[2].className,
          // The help of "All models", every workspace's default (w10/m592).
          helpText: section.children[2].textContent,
        };
      };
      setup(ROUTED);
      const readySection = screen.getByTestId("enabled-models-section");
      const ready = shape(readySection);
      // The switch is h-5 w-9; the skeleton holds that box.
      for (const s of switches())
        expect(s).toHaveClass("h-5", "w-9", "shrink-0");
      cleanup();

      setup(null, true);
      const skeleton = screen.getByTestId("enabled-models-section-skeleton");
      // A representative four rows, the platform catalog's size.
      expect(shape(skeleton)).toEqual(ready);
      expect(skeleton).toHaveTextContent("Models members can choose");
      expect(skeleton).toHaveTextContent(
        /Choose which platform models members can pick/,
      );
      for (const row of Array.from(skeleton.querySelectorAll("li"))) {
        expect(row.lastElementChild).toHaveClass(
          "h-5",
          "w-9",
          "shrink-0",
          "rounded-full",
        );
        const id = row.querySelector("[data-slot=skeleton-text]")!;
        expect(id).toHaveAttribute("aria-hidden", "true");
      }
      // Hidden ids as long as the catalog's in each slot, so a monospace
      // label wraps to as many lines at every width: the default's (two
      // lines at 390 px), then the others' (one).
      expect(
        Array.from(
          skeleton.querySelectorAll("li [data-slot=skeleton-text]"),
          (id) => id.textContent!.length,
        ),
      ).toEqual([
        CATALOG[0].length,
        ...CATALOG.slice(1).map(() =>
          Math.max(...CATALOG.slice(1).map((m) => m.length)),
        ),
      ]);
      expect(within(skeleton).queryByRole("switch")).toBeNull();
    });

    // w10/m592: Devin's "Allow all models", every workspace's default, is a
    // choice of its own, drawn first; "By model" is an allowlist even when
    // it lists every model.
    describe("all models", () => {
      const PROVIDERS = ["deepseek", "anthropic"];
      const ALL = { ...ROUTED, providers: PROVIDERS };
      const allOn = () =>
        expect(switches().map((s) => s.getAttribute("aria-checked"))).toEqual(
          CATALOG.map(() => "true"),
        );

      it("offers every model on and locked, as the first choice", async () => {
        const { user, update } = setup(ALL);
        expect(
          within(filterChoice())
            .getAllByRole("radio")
            .map((radio) => radio.parentElement!.textContent),
        ).toEqual(["All models", "By model", "By provider"]);
        expect(filter("All models")).toBeChecked();
        expect(filterChoice()).toHaveAccessibleDescription(ALL_HELP);
        allOn();
        for (const s of switches()) expect(s).toBeDisabled();
        expect(modelSwitch(CATALOG[0])).toHaveAccessibleDescription(
          "The platform's default model always stays on, as sessions fall back to it.",
        );
        // The others are locked by the choice, whose help says why.
        expect(modelSwitch(CATALOG[2])).toHaveAccessibleDescription(ALL_HELP);
        expect(screen.queryByText("New in the catalog")).toBeNull();
        await user.click(modelSwitch(CATALOG[2]));
        expect(update).not.toHaveBeenCalled();
      });

      it("switches to By model with only the mode, every model on as the allowlist", async () => {
        const { user, update } = setupAsServer(ALL);
        await user.click(filter("By model"));
        expect(update).toHaveBeenLastCalledWith({ allowAllModels: false });
        expect(filter("By model")).toBeChecked();
        expect(filterChoice()).toHaveAccessibleDescription(BY_MODEL_HELP);
        allOn();
        for (const model of CATALOG.slice(1)) {
          expect(modelSwitch(model)).toBeEnabled();
        }
        expect(toast.success).toHaveBeenLastCalledWith(
          "Members now choose from the models you allow",
        );
      });

      it("switches to By provider with every provider", async () => {
        const { user, update } = setupAsServer(ALL);
        await user.click(filter("By provider"));
        expect(update).toHaveBeenLastCalledWith({
          enabledProviders: PROVIDERS,
        });
        expect(filter("By provider")).toBeChecked();
        expect(toast.success).toHaveBeenLastCalledWith(
          "Members now choose from the models of the providers you allow",
        );
      });

      it.each([
        ["By model", { ...LISTED, enabledModels: [CATALOG[0], CATALOG[2]] }],
        ["By provider", { ...LISTED, enabledProviders: ["anthropic"] }],
      ])(
        "switches back from %s with only the mode, never a list",
        async (_, env) => {
          const { user, update } = setupAsServer({
            ...env,
            providers: PROVIDERS,
          });
          await user.click(filter("All models"));
          expect(update).toHaveBeenLastCalledWith({ allowAllModels: true });
          expect(filter("All models")).toBeChecked();
          allOn();
          for (const s of switches()) expect(s).toBeDisabled();
          expect(toast.success).toHaveBeenLastCalledWith(
            "Members now choose from every model, including models added to the platform later",
          );
        },
      );

      it("keeps an allowlist of every model an allowlist, a later model off and new", async () => {
        const KIMI = "moonshotai/kimi-k3";
        const { user, update } = setupAsServer({
          ...LISTED,
          models: [...CATALOG, KIMI],
          newModels: [KIMI],
        });
        expect(filter("By model")).toBeChecked();
        expect(modelSwitch(KIMI)).not.toBeChecked();
        expect(modelSwitch(KIMI)).toHaveAccessibleDescription(
          "New in the catalog",
        );
        await user.click(modelSwitch(CATALOG[1]));
        expect(update).toHaveBeenLastCalledWith({
          enabledModels: [CATALOG[0], ...CATALOG.slice(2)],
        });
        // Every earlier model on again sends the list alone: it stays an
        // allowlist, and the later model stays off.
        await user.click(modelSwitch(CATALOG[1]));
        expect(update).toHaveBeenLastCalledWith({ enabledModels: CATALOG });
        expect(filter("By model")).toBeChecked();
        expect(modelSwitch(KIMI)).not.toBeChecked();
        expect(modelSwitch(KIMI)).toHaveAccessibleDescription(
          "New in the catalog",
        );
      });

      it("returns to the saved mode and says why when the server refuses it", async () => {
        const { user, update } = setupAsServer(ALL);
        update.mockRejectedValueOnce(
          codedGraphQLError("AGENT_ENVIRONMENT_MODELS_INVALID", {
            field: "allowAllModels",
          }),
        );
        await user.click(filter("By model"));
        expect(update).toHaveBeenLastCalledWith({ allowAllModels: false });
        expect(await screen.findByRole("alert")).toHaveTextContent(
          "Couldn't change the models members can choose: this workspace no longer uses the platform model catalog. Reload to see the current settings.",
        );
        expect(filter("All models")).toBeChecked();
        for (const s of switches()) expect(s).toBeDisabled();
        expect(toast.success).not.toHaveBeenCalled();
      });
    });

    // w10/m573: the list filtered by provider, Devin's other filter type,
    // one at a time.
    describe("by provider", () => {
      const KIMI = "moonshotai/kimi-k3";
      const BY_PROVIDER = {
        ...LISTED,
        models: [...CATALOG, KIMI],
        enabledModels: CATALOG,
        providers: ["deepseek", "anthropic", "moonshotai"],
        enabledProviders: ["anthropic"],
      };
      const providerSwitch = (name: string) =>
        screen.getByRole("switch", { name });

      it("switches to the providers of the models on now, saying what it does", async () => {
        const { user, update } = setupAsServer({
          ...LISTED,
          enabledModels: [CATALOG[0], CATALOG[2]],
          providers: ["deepseek", "anthropic"],
        });
        expect(filter("By model")).toBeChecked();
        expect(filterChoice()).toHaveAccessibleDescription(BY_MODEL_HELP);
        await user.click(filter("By provider"));
        expect(update).toHaveBeenLastCalledWith({
          enabledProviders: ["deepseek", "anthropic"],
        });
        expect(toast.success).toHaveBeenLastCalledWith(
          "Members now choose from the models of the providers you allow",
        );
      });

      it("lists a switch per provider with its models, the default always on", async () => {
        const { user, update } = setupAsServer(BY_PROVIDER);
        const section = screen.getByTestId("enabled-models-section");
        expect(filter("By provider")).toBeChecked();
        expect(filterChoice()).toHaveAccessibleDescription(
          /every model of the providers you allow, including models added to the platform later/,
        );
        expect(
          within(section)
            .getAllByRole("listitem")
            .map((row) => row.querySelector("label")!.textContent),
        ).toEqual(["deepseek", "anthropic", "moonshotai"]);
        expect(switches().map((s) => s.getAttribute("aria-checked"))).toEqual([
          "false",
          "true",
          "false",
        ]);
        expect(providerSwitch("deepseek")).toHaveAccessibleDescription(
          "deepseek/deepseek-v4.1-flash-1017 (the default, always on)",
        );
        expect(providerSwitch("moonshotai")).toHaveAccessibleDescription(KIMI);
        // The one provider allowed is locked on: an empty filter is refused.
        expect(providerSwitch("anthropic")).toBeDisabled();
        expect(providerSwitch("anthropic")).toHaveAccessibleDescription(
          `${CATALOG.slice(1).join(", ")}At least one provider stays allowed. To offer only the default model, filter by model.`,
        );
        await user.click(providerSwitch("moonshotai"));
        expect(update).toHaveBeenLastCalledWith({
          enabledProviders: ["anthropic", "moonshotai"],
        });
        expect(providerSwitch("moonshotai")).toBeChecked();
        expect(providerSwitch("anthropic")).toBeEnabled();
        expect(toast.success).toHaveBeenLastCalledWith(
          "moonshotai models turned on for members",
        );
        await user.click(providerSwitch("anthropic"));
        expect(update).toHaveBeenLastCalledWith({
          enabledProviders: ["moonshotai"],
        });
      });

      it("switches back to the list by model with the models on now", async () => {
        const { user, update } = setupAsServer(BY_PROVIDER);
        await user.click(filter("By model"));
        expect(update).toHaveBeenLastCalledWith({ enabledModels: CATALOG });
        expect(filter("By model")).toBeChecked();
        expect(
          within(screen.getByTestId("enabled-models-section"))
            .getAllByRole("listitem")
            .map((row) => row.querySelector("label")!.textContent),
        ).toEqual([...CATALOG, KIMI]);
        expect(modelSwitch(KIMI)).not.toBeChecked();
        expect(toast.success).toHaveBeenLastCalledWith(
          "Members now choose from the models you allow",
        );
      });

      it("returns to the saved filter and names the providers when the server refuses", async () => {
        const { user, update } = setupAsServer(BY_PROVIDER);
        update.mockRejectedValueOnce(
          codedGraphQLError("AGENT_ENVIRONMENT_MODELS_INVALID", {
            field: "enabledProviders",
            provider: "moonshotai",
          }),
        );
        await user.click(providerSwitch("moonshotai"));
        expect(await screen.findByRole("alert")).toHaveTextContent(
          "Couldn't update the providers: the list has to name at least one provider of the platform catalog.",
        );
        expect(providerSwitch("moonshotai")).not.toBeChecked();
        expect(filter("By provider")).toBeChecked();
      });

      it("shows a member the filter read-only", async () => {
        vi.mocked(useCapabilities).mockReturnValue(
          mockCapabilities({ role: "DEVELOPER", canManage: false }),
        );
        const { user, update } = setup(BY_PROVIDER);
        for (const radio of screen.getAllByRole("radio")) {
          expect(radio).toBeDisabled();
        }
        for (const s of switches()) expect(s).toBeDisabled();
        await user.click(filter("By model"));
        expect(update).not.toHaveBeenCalled();
      });
    });
  });

  // w7/m392: admins choose the available agents; Muse Code (whose provider
  // may train on session data, ADR108) turns on only through an
  // acknowledgement.
  describe("available agents", () => {
    const toggle = (name: string) => screen.getByRole("switch", { name });

    it("turns an agent off by replacing the enabled list", async () => {
      const { update, user } = setup();
      expect(toggle("Codex")).toBeChecked();
      expect(toggle("Muse Code")).not.toBeChecked();
      await user.click(toggle("Codex"));
      expect(update).toHaveBeenCalledWith({
        enabledAgents: ["claude", "gemini"],
      });
    });

    it("asks for the data-use acknowledgement before turning Muse Code on", async () => {
      const { update, user } = setup();
      await user.click(toggle("Muse Code"));
      expect(update).not.toHaveBeenCalled();
      const dialog = await screen.findByRole("alertdialog");
      expect(
        within(dialog).getByText(/Meta may use the prompts, code, files/),
      ).toBeInTheDocument();
      await user.click(
        within(dialog).getByRole("button", { name: "Acknowledge and turn on" }),
      );
      expect(update).toHaveBeenCalledWith({
        enabledAgents: ["claude", "codex", "gemini", "muse"],
        acknowledgeAgentDataUse: ["muse"],
      });
    });

    it("turns an acknowledged agent back on without asking again", async () => {
      const { update, user } = setup(
        agentEnvironment({
          enabledAgents: ["claude"],
          agentDataUse: [
            {
              __typename: "AgentDataUse",
              agent: "muse",
              acknowledgedAt: "2026-11-14T09:00:00Z",
              acknowledgedBy: "usr-admin",
            },
          ],
        }),
      );
      await user.click(toggle("Muse Code"));
      expect(screen.queryByRole("alertdialog")).toBeNull();
      expect(update).toHaveBeenCalledWith({
        enabledAgents: ["claude", "muse"],
      });
    });

    it("shows non-admins the agents with every switch disabled", () => {
      vi.mocked(useCapabilities).mockReturnValue(
        mockCapabilities({ role: "DEVELOPER", canManage: false }),
      );
      setup();
      for (const name of ["Claude", "Codex", "Gemini", "Muse Code"]) {
        expect(toggle(name)).toBeDisabled();
      }
    });
  });
});
