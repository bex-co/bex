import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toast } from "sonner";
import { AgentNetworkTier } from "@/graphql/definitions";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
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

const ENV: AgentEnvironmentView = {
  __typename: "AgentEnvironment",
  ownerId: "tea-1",
  networkTier: AgentNetworkTier.Registries,
  customHosts: [],
  defaultModel: null,
  modelPinned: false,
  pinnedModel: null,
  registryHosts: ["registry.npmjs.org", "pypi.org", "proxy.golang.org"],
  updatedAt: null,
};

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

  it("renders the three tiers, Full disabled, and the registry hosts", async () => {
    const { user } = setup();
    expect(radio(/^Package registries/)).toHaveAttribute(
      "aria-checked",
      "true",
    );
    expect(radio(/^Custom/)).toBeEnabled();
    expect(radio(/Full internet/)).toBeDisabled();
    expect(screen.getByText("Coming soon")).toBeInTheDocument();
    expect(screen.queryByLabelText("Additional hostnames")).toBeNull();
    await user.click(screen.getByText("Included package registries (3 hosts)"));
    expect(screen.getByText("pypi.org")).toBeVisible();
    expect(save()).toBeDisabled();
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
});
