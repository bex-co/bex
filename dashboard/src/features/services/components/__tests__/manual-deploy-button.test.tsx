import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ManualDeployButton } from "@/features/services/components/manual-deploy-button";
import type { ServiceView } from "@/features/services/types";

const mockNavigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => mockNavigate,
}));

const trigger = vi.fn();
vi.mock("@/features/services/hooks/use-trigger-deploy", () => ({
  useTriggerDeploy: () => ({ deploying: false, trigger }),
}));

function svc(overrides: Partial<ServiceView> = {}): ServiceView {
  return {
    id: "web",
    name: "web",
    type: "web_service",
    suspended: false,
    phase: "Running",
    url: "https://web.onbex.co",
    createdAt: null,
    replicas: 1,
    revision: "r1",
    plan: "starter",
    idleTTLSeconds: 0,
    schedule: null,
    command: null,
    runs: [],
    repo: null,
    branch: null,
    rootDir: null,
    ...overrides,
  };
}

beforeEach(() => {
  mockNavigate.mockReset();
  trigger.mockReset();
});

describe("ManualDeployButton — navigate to the new deploy's page (w9/m1/t004)", () => {
  it("navigates to the new deploy's page once the trigger resolves an id", async () => {
    trigger.mockResolvedValue("dep-new-1");
    const user = userEvent.setup();
    render(<ManualDeployButton service={svc()} pending={false} />);

    await user.click(
      screen.getByRole("button", { name: /Manual Deploy/i }),
    );
    await user.click(screen.getByText("Deploy latest image"));
    await user.click(
      screen.getByRole("button", { name: "Proceed" }),
    );

    expect(trigger).toHaveBeenCalledWith("web");
    expect(mockNavigate).toHaveBeenCalledWith({
      to: "/services/$serviceId/deploys/$deployId",
      params: { serviceId: "web", deployId: "dep-new-1" },
    });
  });

  it("does not navigate when the trigger fails (already toasted, no deploy id)", async () => {
    trigger.mockResolvedValue(null);
    const user = userEvent.setup();
    render(<ManualDeployButton service={svc()} pending={false} />);

    await user.click(
      screen.getByRole("button", { name: /Manual Deploy/i }),
    );
    await user.click(screen.getByText("Deploy latest image"));
    await user.click(
      screen.getByRole("button", { name: "Proceed" }),
    );

    expect(trigger).toHaveBeenCalledWith("web");
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("navigates to the restart-triggered deploy's page too (w2/m30: restart opens a deploy row)", async () => {
    trigger.mockResolvedValue("dep-restart-1");
    const user = userEvent.setup();
    render(<ManualDeployButton service={svc()} pending={false} />);

    await user.click(
      screen.getByRole("button", { name: /Manual Deploy/i }),
    );
    await user.click(screen.getByText("Restart service"));
    await user.click(
      screen.getByRole("button", { name: "Proceed" }),
    );

    expect(mockNavigate).toHaveBeenCalledWith({
      to: "/services/$serviceId/deploys/$deployId",
      params: { serviceId: "web", deployId: "dep-restart-1" },
    });
  });
});
