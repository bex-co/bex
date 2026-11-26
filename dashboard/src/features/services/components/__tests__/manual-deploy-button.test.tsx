import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ManualDeployButton } from "@/features/services/components/manual-deploy-button";
import type { ServiceView } from "@/features/services/types";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  toResourceSnapshot,
  type ResourceActionInput,
} from "@/features/capabilities/lib/resource-actions";
import { mockCapabilities } from "@/test/mocks/capabilities";
import { usePipelineMinutesExceeded } from "@/features/usage/hooks/use-pipeline-minutes-exceeded";

const mockNavigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => mockNavigate,
  Link: ({
    to,
    hash,
    children,
    className,
  }: {
    to: string;
    hash?: string;
    children: React.ReactNode;
    className?: string;
  }) => (
    <a href={hash ? `${to}#${hash}` : to} className={className}>
      {children}
    </a>
  ),
}));

const trigger = vi.fn();
const restart = vi.fn();
vi.mock("@/features/services/hooks/use-trigger-deploy", () => ({
  useTriggerDeploy: () => ({ deploying: false, trigger, restart }),
}));

const apolloQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useApolloClient: () => ({ query: apolloQuery }),
}));

const buildSourceBuildsState: {
  builds: Array<{
    id: string;
    buildSourceId: string;
    status: string;
    succeeded: boolean;
    commitId: string | null;
    commitUrl: string | null;
    imageRef: string | null;
    imageSha: string | null;
    createdAt: string | null;
  }>;
  loading: boolean;
  error: Error | undefined;
  refetch: ReturnType<typeof vi.fn>;
} = {
  builds: [],
  loading: false,
  error: undefined,
  refetch: vi.fn(),
};
vi.mock("@/features/services/hooks/use-build-sources", () => ({
  useBuildSourceBuilds: () => buildSourceBuildsState,
}));

const refresh = vi.fn();
const allowedDeploy: ResourceActionInput[] = [
  { action: "deploy", outcome: "allowed", reason: null, precondition: null },
];
const deniedDeploy: ResourceActionInput[] = [
  {
    action: "deploy",
    outcome: "denied",
    reason: "insufficient_permission",
    precondition: null,
  },
];
let deployRows = allowedDeploy;
vi.mock("@/features/capabilities/hooks/use-resource-actions", () => ({
  useDeployActions: () => ({
    status: "ready",
    snapshot: toResourceSnapshot(workspaceId, "web", deployRows),
    refresh,
  }),
}));

let workspaceId = "tea-test";
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: workspaceId }),
}));

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
    dockerContext: null,
    registryCredentialId: null,
    buildFilter: null,
    autoDeploy: null,
    autoDeployTrigger: null,
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
  workspaceId = "tea-test";
  deployRows = allowedDeploy;
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  refresh.mockReset().mockResolvedValue(undefined);
  apolloQuery.mockReset().mockResolvedValue({
    data: {
      deployActions: allowedDeploy,
      viewerCapabilities: {
        ...mockCapabilities(),
        fresh: true,
        grants: [{ action: "can_create", outcome: "allowed", reason: null }],
      },
    },
  });
  mockNavigate.mockReset();
  trigger.mockReset();
  restart.mockReset();
  buildSourceBuildsState.builds = [];
  buildSourceBuildsState.loading = false;
  buildSourceBuildsState.error = undefined;
  buildSourceBuildsState.refetch.mockReset();
});

describe("ManualDeployButton permission freshness", () => {
  const repo = { repo: "https://github.com/bex-co/bex", autoDeploy: true };

  describe.each([
    ["Deploy latest commit", null],
    ["Clear build cache & deploy", null],
    ["Restart service", "Restart"],
    ["Deploy a specific commit", "Deploy commit"],
  ] as const)("%s", (menuItem, confirm) => {
    it.each(["denied", "unavailable"])(
      "dispatches nothing when the fresh decision is %s",
      async (outcome) => {
        if (outcome === "unavailable") {
          apolloQuery.mockRejectedValue(new Error("permission request failed"));
        } else {
          apolloQuery.mockResolvedValue({
            data: {
              deployActions: [
                {
                  action: "deploy",
                  outcome: "denied",
                  reason: "insufficient_permission",
                  precondition: null,
                },
              ],
            },
          });
        }
        const user = userEvent.setup();
        render(<ManualDeployButton service={svc(repo)} pending={false} />);
        await user.click(screen.getByRole("button", { name: "Manual Deploy" }));
        await user.click(screen.getByRole("menuitem", { name: menuItem }));
        if (confirm === "Deploy commit") {
          await user.type(screen.getByLabelText("Commit SHA"), "abcdef1");
        }
        if (confirm) {
          await user.click(screen.getByRole("button", { name: confirm }));
        }
        expect(trigger, menuItem).not.toHaveBeenCalled();
        expect(restart, menuItem).not.toHaveBeenCalled();
        expect(mockNavigate, menuItem).not.toHaveBeenCalled();
      },
    );
  });

  it.each(["workspace", "resource", "generation"])(
    "discards a restart confirmation after the %s changes",
    async (change) => {
      const user = userEvent.setup();
      const { rerender } = render(
        <ManualDeployButton service={svc()} pending={false} />,
      );
      await user.click(screen.getByRole("button", { name: "Manual Deploy" }));
      await user.click(
        screen.getByRole("menuitem", { name: "Restart service" }),
      );
      expect(screen.getByRole("alertdialog")).toBeInTheDocument();
      if (change === "workspace") workspaceId = "tea-second";
      if (change === "generation") {
        vi.mocked(useCapabilities).mockReturnValue(
          mockCapabilities({ generation: 2 }),
        );
      }
      rerender(
        <ManualDeployButton
          service={svc(
            change === "resource" ? { id: "other", name: "other" } : {},
          )}
          pending={false}
        />,
      );
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
      expect(restart).not.toHaveBeenCalled();
    },
  );

  it("discards the commit draft after access changes", async () => {
    const user = userEvent.setup();
    const { rerender } = render(
      <ManualDeployButton service={svc(repo)} pending={false} />,
    );
    await user.click(screen.getByRole("button", { name: "Manual Deploy" }));
    await user.click(
      screen.getByRole("menuitem", { name: "Deploy a specific commit" }),
    );
    await user.type(screen.getByLabelText("Commit SHA"), "abcdef1");
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ generation: 2 }),
    );
    rerender(<ManualDeployButton service={svc(repo)} pending={false} />);
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Manual Deploy" }));
    await user.click(
      screen.getByRole("menuitem", { name: "Deploy a specific commit" }),
    );
    expect(screen.getByLabelText("Commit SHA")).toHaveValue("");
    expect(trigger).not.toHaveBeenCalled();
  });

  // Render's role table gives a Contributor "Trigger service deploys", and
  // its Manual Deploy menu offers a specific commit as a deploy (w9/m622).
  it("offers a Contributor every Manual Deploy entry and deploys a specific commit", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    // The fresh recheck reads the deploy decision alone: a Contributor holds
    // no can_create, and the deploy still goes through.
    apolloQuery.mockResolvedValue({
      data: {
        deployActions: allowedDeploy,
        viewerCapabilities: {
          ...mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
          fresh: true,
          grants: [
            {
              action: "can_create",
              outcome: "denied",
              reason: "insufficient_permission",
            },
          ],
        },
      },
    });
    trigger.mockResolvedValue("dep-contrib-pin");
    const user = userEvent.setup();
    render(<ManualDeployButton service={svc(repo)} pending={false} />);
    await user.click(screen.getByRole("button", { name: "Manual Deploy" }));
    for (const name of [
      "Deploy latest commit",
      "Deploy a specific commit",
      "Clear build cache & deploy",
      "Restart service",
    ]) {
      expect(screen.getByRole("menuitem", { name })).not.toHaveAttribute(
        "aria-disabled",
        "true",
      );
    }
    await user.click(
      screen.getByRole("menuitem", { name: "Deploy a specific commit" }),
    );
    await user.type(screen.getByLabelText("Commit SHA"), "abcdef1");
    await user.click(screen.getByRole("button", { name: "Deploy commit" }));
    await waitFor(() =>
      expect(trigger).toHaveBeenCalledExactlyOnceWith("web", {
        commitId: "abcdef1",
        disableAutoDeploy: true,
      }),
    );
  });

  // A Viewer and a Billing member get no deploy: every entry is held with
  // the role's reason, and choosing one opens nothing.
  it("holds every entry with the role's reason when the deploy decision is denied", async () => {
    deployRows = deniedDeploy;
    const user = userEvent.setup();
    render(<ManualDeployButton service={svc(repo)} pending={false} />);
    await user.click(screen.getByRole("button", { name: "Manual Deploy" }));
    for (const name of [
      "Deploy latest commit",
      "Deploy a specific commit",
      "Clear build cache & deploy",
      "Restart service",
    ]) {
      const item = screen.getByRole("menuitem", { name });
      expect(item).toHaveAttribute("aria-disabled", "true");
    }
    await user.hover(
      screen.getByRole("menuitem", { name: "Deploy a specific commit" }),
    );
    expect(
      await screen.findAllByText(
        "Your role can’t perform this action. Ask a workspace admin.",
      ),
    ).not.toHaveLength(0);
    await user.click(
      screen.getByRole("menuitem", { name: "Deploy a specific commit" }),
    );
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(trigger).not.toHaveBeenCalled();
  });
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
    render(
      <ManualDeployButton
        service={svc({ repo: "https://github.com/bex-co/bex" })}
        pending={false}
      />,
    );

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

  it("offers no build cache to clear on an image-backed service (w4/m141)", async () => {
    const user = userEvent.setup();
    render(<ManualDeployButton service={svc()} pending={false} />);

    await user.click(screen.getByRole("button", { name: /Manual Deploy/i }));
    expect(
      screen.queryByText("Clear build cache & deploy"),
    ).not.toBeInTheDocument();
  });

  it("tells a static site's restart that it re-publishes, with no instances to replace (w4/m141)", async () => {
    const user = userEvent.setup();
    render(
      <ManualDeployButton
        service={svc({
          type: "static_site",
          repo: "https://github.com/bex-co/bex",
        })}
        pending={false}
      />,
    );

    await user.click(screen.getByRole("button", { name: /Manual Deploy/i }));
    await user.click(screen.getByText("Restart service"));

    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent(
      "web is published again from the commit it is serving now.",
    );
    expect(dialog).not.toHaveTextContent(/instances/);
  });

  it("offers no Restart service for a cron job (Render: not supported, w8/040)", async () => {
    const user = userEvent.setup();
    render(
      <ManualDeployButton
        service={svc({ type: "cron_job" })}
        pending={false}
      />,
    );

    await user.click(screen.getByRole("button", { name: /Manual Deploy/i }));
    expect(screen.queryByText("Restart service")).not.toBeInTheDocument();
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

  it("deploys the entered SHA and turns auto-deploy off in one mutation, then opens the deploy", async () => {
    trigger.mockResolvedValue("dep-pin-1");
    const user = await openCommitDialog(svc(repo));

    expect(screen.getByRole("alertdialog")).toHaveTextContent(
      "turns off auto-deploy, so later pushes don't replace it",
    );
    await user.type(screen.getByLabelText("Commit SHA"), ` ${SHA} `);
    await user.click(screen.getByRole("button", { name: "Deploy commit" }));

    // One write pins the commit and turns auto-deploy off (w4/m431): no
    // follow-up setAutoDeploy a push could land in front of.
    expect(trigger).toHaveBeenCalledExactlyOnceWith("web", {
      commitId: SHA,
      disableAutoDeploy: true,
    });
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

  it("does not open a deploy the server refused", async () => {
    trigger.mockResolvedValue(null);
    const user = await openCommitDialog(svc(repo));
    await user.type(screen.getByLabelText("Commit SHA"), SHA);
    await user.click(screen.getByRole("button", { name: "Deploy commit" }));
    await waitFor(() =>
      expect(trigger).toHaveBeenCalledExactlyOnceWith("web", {
        commitId: SHA,
        disableAutoDeploy: true,
      }),
    );
    expect(mockNavigate).not.toHaveBeenCalled();
  });
});

describe("ManualDeployButton — Build-Source-linked service (w2/m612 t004)", () => {
  const linked = { buildSourceId: "bds-mono", repo: null };

  it("shows Deploy latest build and Deploy a specific build… (never Deploy latest image or git-only items)", async () => {
    trigger.mockResolvedValue("dep-bds-latest");
    const user = userEvent.setup();
    render(<ManualDeployButton service={svc(linked)} pending={false} />);

    await user.click(screen.getByRole("button", { name: /Manual Deploy/i }));
    expect(
      screen.getByRole("menuitem", { name: "Deploy latest build" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: "Deploy a specific build…" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Deploy latest image")).not.toBeInTheDocument();
    expect(screen.queryByText("Deploy latest commit")).not.toBeInTheDocument();
    expect(
      screen.queryByText("Deploy a specific commit"),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("Clear build cache & deploy"),
    ).not.toBeInTheDocument();

    await user.click(
      screen.getByRole("menuitem", { name: "Deploy latest build" }),
    );
    expect(trigger).toHaveBeenCalledExactlyOnceWith("web");
    await waitFor(() =>
      expect(mockNavigate).toHaveBeenCalledWith({
        to: "/services/$serviceId/deploys/$deployId",
        params: { serviceId: "web", deployId: "dep-bds-latest" },
      }),
    );
  });

  it("lets the user pick a succeeded build, disables non-succeeded builds, triggers with buildId, and navigates", async () => {
    trigger.mockResolvedValue("dep-bds-picked");
    buildSourceBuildsState.builds = [
      {
        id: "bld-failed01",
        buildSourceId: "bds-mono",
        status: "failed",
        succeeded: false,
        commitId: "1111111aaaa",
        commitUrl: null,
        imageRef: null,
        imageSha: null,
        createdAt: "2026-11-23T12:00:00Z",
      },
      {
        id: "bld-ok00002",
        buildSourceId: "bds-mono",
        status: "succeeded",
        succeeded: true,
        commitId: "abcdef123456",
        commitUrl: null,
        imageRef: null,
        imageSha: null,
        createdAt: "2026-11-23T11:00:00Z",
      },
    ];

    const user = userEvent.setup();
    render(<ManualDeployButton service={svc(linked)} pending={false} />);
    await user.click(screen.getByRole("button", { name: /Manual Deploy/i }));
    await user.click(
      screen.getByRole("menuitem", { name: "Deploy a specific build…" }),
    );

    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent("Deploy a specific build");
    const confirm = screen.getByRole("button", { name: "Deploy build" });
    expect(confirm).toBeDisabled();

    const failedRow = screen.getByRole("button", { name: /bld-failed01/ });
    expect(failedRow).toBeDisabled();
    expect(failedRow).toHaveTextContent("Failed");

    const okRow = screen.getByRole("button", { name: /bld-ok00002/ });
    expect(okRow).toBeEnabled();
    expect(okRow).toHaveTextContent("Succeeded");
    expect(okRow).toHaveTextContent("abcdef1");

    await user.click(okRow);
    expect(okRow).toHaveAttribute("aria-pressed", "true");
    expect(confirm).toBeEnabled();

    await user.click(confirm);
    expect(trigger).toHaveBeenCalledExactlyOnceWith("web", {
      buildId: "bld-ok00002",
    });
    await waitFor(() =>
      expect(mockNavigate).toHaveBeenCalledWith({
        to: "/services/$serviceId/deploys/$deployId",
        params: { serviceId: "web", deployId: "dep-bds-picked" },
      }),
    );
  });

  it("offers Deploy a specific build… to a Contributor, as a deploy (w9/m622)", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "CONTRIBUTOR", canCreate: false }),
    );
    const user = userEvent.setup();
    render(<ManualDeployButton service={svc(linked)} pending={false} />);
    await user.click(screen.getByRole("button", { name: /Manual Deploy/i }));
    expect(
      screen.getByRole("menuitem", { name: "Deploy latest build" }),
    ).not.toHaveAttribute("aria-disabled", "true");
    expect(
      screen.getByRole("menuitem", { name: "Deploy a specific build…" }),
    ).not.toHaveAttribute("aria-disabled", "true");
  });

  it("shows empty and error+retry states in the build picker", async () => {
    buildSourceBuildsState.error = new Error("upstream down");
    const user = userEvent.setup();
    const { rerender } = render(
      <ManualDeployButton service={svc(linked)} pending={false} />,
    );
    await user.click(screen.getByRole("button", { name: /Manual Deploy/i }));
    await user.click(
      screen.getByRole("menuitem", { name: "Deploy a specific build…" }),
    );
    expect(
      screen.getByText("Couldn't load builds for this Build Source."),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Try again" }));
    expect(buildSourceBuildsState.refetch).toHaveBeenCalledOnce();

    buildSourceBuildsState.error = undefined;
    buildSourceBuildsState.builds = [];
    rerender(<ManualDeployButton service={svc(linked)} pending={false} />);
    expect(
      screen.getByText(/This Build Source has no builds yet/),
    ).toBeInTheDocument();
  });
});

// Render's Manual Deploy (`sFe`, w3/108): while the workspace's pipeline minutes
// are spent the whole menu is disabled, Restart included, and hovering it shows
// "You’ve used up the maximum number of pipeline minutes specified by your
// spend limit. Learn more." with both links.
describe("ManualDeployButton — pipeline minutes spent (w3/108)", () => {
  const repo = { repo: "https://github.com/bex-co/bex", autoDeploy: true };

  it("disables the whole menu and explains why, with Render's links", async () => {
    vi.mocked(usePipelineMinutesExceeded).mockReturnValue(true);
    const user = userEvent.setup();
    render(<ManualDeployButton service={svc(repo)} pending={false} />);
    const button = screen.getByRole("button", { name: "Manual Deploy" });
    expect(button).toBeDisabled();
    await user.click(button);
    expect(screen.queryByRole("menuitem")).not.toBeInTheDocument();

    await user.hover(button.parentElement!);
    const notice = await screen.findByRole("tooltip");
    expect(notice).toHaveTextContent(
      "You’ve used up the maximum number of pipeline minutes specified by your spend limit.Learn more.",
    );
    expect(
      within(notice).getByRole("link", { name: "pipeline minutes" }),
    ).toHaveAttribute("href", "/workspace/settings#build-pipeline");
    expect(
      within(notice).getByRole("link", { name: "Learn more" }),
    ).toHaveAttribute(
      "href",
      "https://render.com/docs/build-pipeline#pipeline-minutes",
    );
    expect(trigger).not.toHaveBeenCalled();
    expect(restart).not.toHaveBeenCalled();
  });

  it("opens the menu as before while minutes remain", async () => {
    const user = userEvent.setup();
    render(<ManualDeployButton service={svc(repo)} pending={false} />);
    const button = screen.getByRole("button", { name: "Manual Deploy" });
    expect(button).toBeEnabled();
    await user.click(button);
    expect(
      screen.getByRole("menuitem", { name: "Deploy latest commit" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: "Restart service" }),
    ).toBeInTheDocument();
  });
});
