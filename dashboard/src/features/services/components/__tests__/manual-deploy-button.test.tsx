import { describe, it, expect, vi, beforeEach } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ManualDeployButton } from "@/features/services/components/manual-deploy-button";
import type { ServiceView } from "@/features/services/types";

const mockNavigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => mockNavigate,
}));

const trigger = vi.fn();
const restart = vi.fn();
vi.mock("@/features/services/hooks/use-trigger-deploy", () => ({
  useTriggerDeploy: () => ({ deploying: false, trigger, restart }),
}));

const setAutoDeploy = vi.fn();
vi.mock("@/features/services/hooks/use-auto-deploy", () => ({
  useAutoDeploy: () => ({ setAutoDeploy, busy: false }),
}));

vi.mock("@/features/capabilities/hooks/use-resource-actions", async () => {
  const { mockAllowedResourceActions } =
    await import("@/test/mocks/resource-actions");
  return mockAllowedResourceActions("web");
});

vi.mock("@/features/workspaces/context/hooks", async () => {
  const { mockWorkspaceContext } = await import("@/test/mocks/workspace");
  return mockWorkspaceContext();
});

function svc(overrides: Partial<ServiceView> = {}): ServiceView {
  return {
    id: "web",
    name: "web",
    slug: null,
    type: "web_service",
    suspended: false,
    phase: "Running",
    url: "https://web.onbex.co",
    internalAddress: null,
    createdAt: null,
    sshAddress: null,
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
    runtime: null,
    builder: null,
    buildCommand: null,
    startCommand: null,
    dockerfilePath: null,
    registryCredentialId: null,
    buildFilter: null,
    autoDeploy: null,
    notifyOnFail: null,
    notificationsToSend: null,
    healthCheckPath: null,
    maxShutdownDelaySeconds: null,
    preDeployCommand: null,
    renderSubdomainPolicy: null,
    publishPath: null,
    routes: [],
    headers: [],
    ipAllowList: null,
    ipAllowListEntries: null,
    maintenanceMode: null,
    outboundIps: null,
    ...overrides,
  };
}

beforeEach(() => {
  mockNavigate.mockReset();
  trigger.mockReset();
  restart.mockReset();
  setAutoDeploy.mockReset().mockResolvedValue(true);
});

describe("ManualDeployButton — navigate to the new deploy's page (w9/m1/t004)", () => {
  it("navigates to the new deploy's page once the trigger resolves an id", async () => {
    trigger.mockResolvedValue("dep-new-1");
    const user = userEvent.setup();
    render(<ManualDeployButton service={svc()} pending={false} />);

    await user.click(screen.getByRole("button", { name: /Manual Deploy/i }));
    await user.click(screen.getByText("Deploy latest image"));

    expect(trigger).toHaveBeenCalledWith("web");
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    await waitFor(() =>
      expect(mockNavigate).toHaveBeenCalledWith({
        to: "/services/$serviceId/deploys/$deployId",
        params: { serviceId: "web", deployId: "dep-new-1" },
      }),
    );
  });

  it("does not navigate when the trigger fails (already toasted, no deploy id)", async () => {
    trigger.mockResolvedValue(null);
    const user = userEvent.setup();
    render(<ManualDeployButton service={svc()} pending={false} />);

    await user.click(screen.getByRole("button", { name: /Manual Deploy/i }));
    await user.click(screen.getByText("Deploy latest image"));

    expect(trigger).toHaveBeenCalledWith("web");
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("Clear build cache & deploy triggers with clearCache=clear (w3/m46 Render parity)", async () => {
    trigger.mockResolvedValue("dep-clear-1");
    const user = userEvent.setup();
    render(<ManualDeployButton service={svc()} pending={false} />);

    await user.click(screen.getByRole("button", { name: /Manual Deploy/i }));
    await user.click(screen.getByText("Clear build cache & deploy"));

    // The plain deploy sends no options; only clear-cache passes clearCache.
    expect(trigger).toHaveBeenCalledWith("web", { clearCache: "clear" });
    await waitFor(() =>
      expect(mockNavigate).toHaveBeenCalledWith({
        to: "/services/$serviceId/deploys/$deployId",
        params: { serviceId: "web", deployId: "dep-clear-1" },
      }),
    );
  });

  it("Restart service confirms, then restarts on the running release and opens its deploy (w1/m148)", async () => {
    restart.mockResolvedValue("dep-restart-1");
    const user = userEvent.setup();
    render(<ManualDeployButton service={svc()} pending={false} />);

    await user.click(screen.getByRole("button", { name: /Manual Deploy/i }));
    await user.click(screen.getByText("Restart service"));

    // The header used to restart with no confirmation, through the same
    // parameter-free trigger as "Deploy latest", which built the branch head.
    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent(
      "web restarts on the commit or image it is running now. Commits pushed since are not deployed.",
    );
    expect(restart).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Restart" }));

    expect(restart).toHaveBeenCalledWith("web");
    expect(trigger).not.toHaveBeenCalled();
    await waitFor(() =>
      expect(mockNavigate).toHaveBeenCalledWith({
        to: "/services/$serviceId/deploys/$deployId",
        params: { serviceId: "web", deployId: "dep-restart-1" },
      }),
    );
  });

  it("cancelling the restart confirmation restarts nothing", async () => {
    const user = userEvent.setup();
    render(<ManualDeployButton service={svc()} pending={false} />);

    await user.click(screen.getByRole("button", { name: /Manual Deploy/i }));
    await user.click(screen.getByText("Restart service"));
    await user.click(await screen.findByRole("button", { name: "Cancel" }));

    expect(restart).not.toHaveBeenCalled();
    expect(trigger).not.toHaveBeenCalled();
  });
});

describe("ManualDeployButton — Deploy a specific commit (w1/m148, Render parity)", () => {
  const repo = {
    repo: "https://github.com/bex-co/bex",
    branch: "main",
    autoDeploy: true,
  };
  const SHA = "f3284af44e2f00fbfe2b2f10f04ae5243e1e2bdf";

  async function openCommitDialog(service: ServiceView) {
    const user = userEvent.setup();
    render(<ManualDeployButton service={service} pending={false} />);
    await user.click(screen.getByRole("button", { name: /Manual Deploy/i }));
    await user.click(screen.getByText("Deploy a specific commit"));
    await screen.findByRole("alertdialog");
    return user;
  }

  it("is offered only for a repo-backed service that is not a cron job", async () => {
    const user = userEvent.setup();
    const { unmount } = render(
      <ManualDeployButton service={svc()} pending={false} />,
    );
    await user.click(screen.getByRole("button", { name: /Manual Deploy/i }));
    expect(
      screen.queryByText("Deploy a specific commit"),
    ).not.toBeInTheDocument();
    unmount();

    render(
      <ManualDeployButton
        service={svc({ ...repo, type: "cron_job" })}
        pending={false}
      />,
    );
    await user.click(screen.getByRole("button", { name: /Manual Deploy/i }));
    expect(
      screen.queryByText("Deploy a specific commit"),
    ).not.toBeInTheDocument();
  });

  it("deploys the entered SHA, turns auto-deploy off, then opens the deploy", async () => {
    trigger.mockResolvedValue("dep-pin-1");
    const user = await openCommitDialog(svc(repo));

    expect(screen.getByRole("alertdialog")).toHaveTextContent(
      "turns off auto-deploy, so later pushes don't replace it",
    );
    await user.type(screen.getByLabelText("Commit SHA"), ` ${SHA} `);
    await user.click(screen.getByRole("button", { name: "Deploy commit" }));

    expect(trigger).toHaveBeenCalledWith("web", { commitId: SHA });
    await waitFor(() =>
      expect(setAutoDeploy).toHaveBeenCalledWith("web", false),
    );
    await waitFor(() =>
      expect(mockNavigate).toHaveBeenCalledWith({
        to: "/services/$serviceId/deploys/$deployId",
        params: { serviceId: "web", deployId: "dep-pin-1" },
      }),
    );
  });

  it("forgets the typed SHA when the dialog is cancelled", async () => {
    const user = await openCommitDialog(svc(repo));
    await user.type(screen.getByLabelText("Commit SHA"), SHA);
    await user.click(screen.getByRole("button", { name: "Cancel" }));

    await user.click(screen.getByRole("button", { name: /Manual Deploy/i }));
    await user.click(screen.getByText("Deploy a specific commit"));
    expect(await screen.findByLabelText("Commit SHA")).toHaveValue("");
    expect(trigger).not.toHaveBeenCalled();
  });

  it("keeps confirm disabled and explains a malformed SHA", async () => {
    const user = await openCommitDialog(svc(repo));
    const confirm = screen.getByRole("button", { name: "Deploy commit" });
    expect(confirm).toBeDisabled();

    await user.type(screen.getByLabelText("Commit SHA"), "main");
    expect(
      screen.getByText("Enter a commit SHA: 7 to 40 hexadecimal characters."),
    ).toBeInTheDocument();
    expect(confirm).toBeDisabled();
    expect(trigger).not.toHaveBeenCalled();
  });

  it("leaves auto-deploy alone when the deploy is refused or it is already off", async () => {
    trigger.mockResolvedValue(null);
    let user = await openCommitDialog(svc(repo));
    await user.type(screen.getByLabelText("Commit SHA"), SHA);
    await user.click(screen.getByRole("button", { name: "Deploy commit" }));
    await waitFor(() => expect(trigger).toHaveBeenCalled());
    expect(setAutoDeploy).not.toHaveBeenCalled();
    expect(mockNavigate).not.toHaveBeenCalled();
    cleanup();

    trigger.mockResolvedValue("dep-pin-2");
    user = await openCommitDialog(svc({ ...repo, autoDeploy: false }));
    await user.type(screen.getByLabelText("Commit SHA"), SHA);
    await user.click(screen.getByRole("button", { name: "Deploy commit" }));
    await waitFor(() => expect(mockNavigate).toHaveBeenCalled());
    expect(setAutoDeploy).not.toHaveBeenCalled();
  });
});
