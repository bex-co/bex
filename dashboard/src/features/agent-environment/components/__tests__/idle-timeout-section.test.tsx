import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toast } from "sonner";
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

// w6/m563: an Enterprise workspace sets how long the sandbox of a session a
// person started stays up after its turn, 5 to 120 minutes, as Devin's
// Enterprise customers can; every other plan keeps 5 and sees no field.
const ENTERPRISE = agentEnvironment({ idleTimeoutConfigurable: true });

function setup(env: AgentEnvironmentView) {
  const update = vi.fn(async (input) => ({ ...env, ...input }));
  hook.current = {
    environment: env,
    loading: false,
    error: undefined,
    refetch: vi.fn(async () => undefined),
    update,
    saving: false,
  };
  render(<AgentEnvironmentCard ownerId="tea-1" />);
  return { update, user: userEvent.setup() };
}

const field = () => screen.getByRole("spinbutton", { name: "Minutes" });
const saveTimeout = () => screen.getByRole("button", { name: "Save timeout" });

describe("Settings → Agents idle timeout", () => {
  beforeEach(() => {
    vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
    vi.mocked(toast.success).mockClear();
  });

  it("is not drawn on a workspace whose plan cannot set it", () => {
    setup(agentEnvironment());
    expect(screen.queryByTestId("idle-timeout-section")).toBeNull();
    expect(screen.queryByRole("spinbutton", { name: "Minutes" })).toBeNull();
  });

  it("shows an Enterprise workspace its timeout and saves a new one", async () => {
    const { user, update } = setup(ENTERPRISE);
    const section = within(screen.getByTestId("idle-timeout-section"));
    expect(section.getByText("Idle timeout")).toBeInTheDocument();
    expect(field()).toHaveValue(5);
    expect(field()).toHaveAccessibleDescription(
      "From 5 to 120 minutes. Workspaces that set none use 5.",
    );
    expect(saveTimeout()).toBeDisabled(); // unchanged
    await user.clear(field());
    await user.type(field(), "30");
    await user.click(saveTimeout());
    expect(update).toHaveBeenCalledWith({ idleTimeoutMinutes: 30 });
    expect(toast.success).toHaveBeenCalledWith(
      "Idle timeout set to 30 minutes",
    );
  });

  it.each(["4", "121", "7.5"])(
    "refuses %s minutes before saving",
    async (typed) => {
      const { user, update } = setup(ENTERPRISE);
      await user.clear(field());
      await user.type(field(), typed);
      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent(
        "Enter a whole number of minutes from 5 to 120.",
      );
      expect(field().getAttribute("aria-describedby")?.split(" ")).toEqual(
        expect.arrayContaining([alert.id]),
      );
      expect(saveTimeout()).toBeDisabled();
      expect(update).not.toHaveBeenCalled();
    },
  );

  it.each([
    [
      codedGraphQLError("PLAN_FEATURE_REQUIRED", {
        feature: "agentIdleTimeout",
      }),
      "Only Enterprise workspaces can change the idle timeout.",
    ],
    [
      codedGraphQLError("AGENT_ENVIRONMENT_IDLE_TIMEOUT_INVALID", {
        field: "idleTimeoutMinutes",
      }),
      "Enter a whole number of minutes from 5 to 120.",
    ],
    [codedGraphQLError("FORBIDDEN"), "Only workspace admins"],
    [uncodedGraphQLError("boom"), "Couldn't save the idle timeout."],
  ])("maps a refused save", async (err, text) => {
    const { user, update } = setup(ENTERPRISE);
    update.mockRejectedValueOnce(err);
    await user.clear(field());
    await user.type(field(), "60");
    await user.click(saveTimeout());
    expect(screen.getByRole("alert")).toHaveTextContent(text);
    expect(toast.success).not.toHaveBeenCalled();
  });

  it("shows a member the timeout with the field disabled", () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "DEVELOPER", canManage: false }),
    );
    setup({ ...ENTERPRISE, idleTimeoutMinutes: 45 });
    expect(field()).toHaveValue(45);
    expect(field()).toBeDisabled();
    expect(saveTimeout()).toBeDisabled();
  });
});
