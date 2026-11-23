import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ServiceRowActions } from "@/features/services/components/service-row-actions";
import { SuspendServiceCard } from "@/features/services/components/suspend-service-card";
import type { ServiceView } from "@/features/services/types";
import { toResourceSnapshot } from "@/features/capabilities/lib/resource-actions";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
import type { ProtectedActionResult } from "@/features/services/lib/protected-confirmation";
import i18n from "@/i18n/init";

vi.mock("@/features/projects/hooks/use-move-to-project", () => ({
  useMoveToProject: () => ({
    projects: [],
    currentProjectId: () => null,
    moveTo: vi.fn(),
    removeFromProject: vi.fn(),
    busyId: null,
  }),
}));

// w4/m238: the stranded-link guard is inert here — reads resolved, nothing
// stranded; the menu never renders with no projects.
vi.mock("@/features/env-groups/hooks/use-stranded-link-guard", () => ({
  useStrandedLinkGuard: () => ({
    groups: [],
    ready: true,
    loading: false,
    error: undefined,
    retry: () => undefined,
    strandedFor: () => [],
    scopeNameFor: () => undefined,
  }),
}));

const apolloQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useApolloClient: () => ({ query: apolloQuery }),
  useQuery: vi.fn(),
  useMutation: vi.fn(() => [vi.fn(), { loading: false }]),
}));

let workspaceId = "tea-test";
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: workspaceId }),
}));

const allowedServer = toResourceSnapshot("tea-test", "app", [
  { action: "suspend", outcome: "allowed", reason: null, precondition: null },
  { action: "resume", outcome: "allowed", reason: null, precondition: null },
  { action: "restart", outcome: "allowed", reason: null, precondition: null },
]);
const allowedDeploy = toResourceSnapshot("tea-test", "app", [
  { action: "deploy", outcome: "allowed", reason: null, precondition: null },
  {
    action: "cancel_deploy",
    outcome: "allowed",
    reason: null,
    precondition: null,
  },
  { action: "rollback", outcome: "allowed", reason: null, precondition: null },
]);
const deniedOperate = toResourceSnapshot("tea-test", "app", [
  {
    action: "suspend",
    outcome: "denied",
    reason: "insufficient_permission",
    precondition: null,
  },
  {
    action: "resume",
    outcome: "denied",
    reason: "insufficient_permission",
    precondition: null,
  },
  {
    action: "restart",
    outcome: "denied",
    reason: "insufficient_permission",
    precondition: null,
  },
]);
const deniedDeploy = toResourceSnapshot("tea-test", "app", [
  {
    action: "deploy",
    outcome: "denied",
    reason: "insufficient_permission",
    precondition: null,
  },
]);

let serverState = {
  status: "ready" as const,
  snapshot: allowedServer,
  refresh: vi.fn().mockResolvedValue(undefined),
};
let deployState = {
  status: "ready" as const,
  snapshot: allowedDeploy,
  refresh: vi.fn().mockResolvedValue(undefined),
};

vi.mock("@/features/capabilities/hooks/use-resource-actions", () => ({
  useServerActions: () => serverState,
  useDeployActions: () => deployState,
}));

const service = {
  id: "app",
  name: "app",
  type: "web_service",
  suspended: false,
  phase: "Running",
  url: null,
  createdAt: null,
  replicas: 1,
  revision: "r1",
} as ServiceView;

beforeEach(() => {
  const receivedAt = Date.now();
  for (const snapshot of [
    allowedServer,
    allowedDeploy,
    deniedOperate,
    deniedDeploy,
  ]) {
    snapshot.receivedAt = receivedAt;
  }
  workspaceId = "tea-test";
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  apolloQuery.mockReset();
  serverState = {
    status: "ready",
    snapshot: allowedServer,
    refresh: vi.fn().mockResolvedValue(undefined),
  };
  deployState = {
    status: "ready",
    snapshot: allowedDeploy,
    refresh: vi.fn().mockResolvedValue(undefined),
  };
  apolloQuery.mockResolvedValue({
    data: {
      serverActions: Object.values(allowedServer.decisions),
      deployActions: Object.values(allowedDeploy.decisions),
    },
  });
});

describe.each([
  ["resource list", ServiceRowActions],
  ["settings", SuspendServiceCard],
] as const)("%s protected confirmation freshness", (surface, Component) => {
  async function openProtectedConfirmation() {
    const onRun = vi.fn().mockResolvedValue({
      status: "confirmation_required",
      confirmation: "sudo suspend service app",
    });
    const user = userEvent.setup();
    const view = render(
      <Component service={service} pending={null} onRun={onRun} />,
    );
    if (surface === "resource list") {
      await user.click(
        screen.getByRole("button", { name: `Actions for ${service.name}` }),
      );
      await user.click(screen.getByRole("menuitem", { name: "Suspend" }));
    } else {
      await user.click(screen.getByRole("button", { name: "Suspend" }));
    }
    const confirm = await screen.findByRole("alertdialog");
    await user.click(within(confirm).getByRole("button", { name: "Suspend" }));
    const dialog = await screen.findByRole("dialog");
    expect(onRun).toHaveBeenCalledExactlyOnceWith("suspend", service);
    await user.type(
      within(dialog).getByLabelText("Sudo Command"),
      "sudo suspend service app",
    );
    return { user, onRun, dialog, ...view };
  }

  it.each(["denied", "unavailable"])(
    "does not retry when the fresh permission check is %s",
    async (outcome) => {
      const { user, onRun, dialog } = await openProtectedConfirmation();
      if (outcome === "unavailable") {
        apolloQuery.mockRejectedValue(new Error("permission request failed"));
      } else {
        apolloQuery.mockResolvedValue({
          data: { serverActions: Object.values(deniedOperate.decisions) },
        });
      }
      await user.click(within(dialog).getByRole("button", { name: "Suspend" }));
      expect(onRun).toHaveBeenCalledTimes(1);
      await waitFor(() =>
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
      );
    },
  );

  it.each(["workspace", "resource", "generation"])(
    "discards the server-issued phrase after the %s changes",
    async (change) => {
      const { onRun, rerender } = await openProtectedConfirmation();
      if (change === "workspace") workspaceId = "tea-second";
      if (change === "generation") {
        vi.mocked(useCapabilities).mockReturnValue(
          mockCapabilities({ generation: 2 }),
        );
      }
      rerender(
        <Component
          service={
            change === "resource"
              ? { ...service, id: "other", name: "other" }
              : service
          }
          pending={null}
          onRun={onRun}
        />,
      );
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
      expect(onRun).toHaveBeenCalledTimes(1);
    },
  );

  it("ignores a protected-confirmation response that arrives after access changes", async () => {
    let finish!: (result: ProtectedActionResult) => void;
    const onRun = vi.fn(
      () =>
        new Promise<ProtectedActionResult>((resolve) => {
          finish = resolve;
        }),
    );
    const user = userEvent.setup();
    const { rerender } = render(
      <Component service={service} pending={null} onRun={onRun} />,
    );
    if (surface === "resource list") {
      await user.click(screen.getByRole("button", { name: /^Actions for / }));
      await user.click(screen.getByRole("menuitem", { name: "Suspend" }));
    } else {
      await user.click(screen.getByRole("button", { name: "Suspend" }));
    }
    await user.click(
      within(await screen.findByRole("alertdialog")).getByRole("button", {
        name: "Suspend",
      }),
    );
    await waitFor(() => expect(onRun).toHaveBeenCalledTimes(1));
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ generation: 2 }),
    );
    rerender(<Component service={service} pending={null} onRun={onRun} />);
    await act(async () => {
      finish({
        status: "confirmation_required",
        confirmation: "sudo suspend service app",
      });
    });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(onRun).toHaveBeenCalledTimes(1);
  });

  it.each(["denied", "unavailable"])(
    "does not resume when the fresh permission check is %s",
    async (outcome) => {
      if (outcome === "unavailable") {
        apolloQuery.mockRejectedValue(new Error("permission request failed"));
      } else {
        apolloQuery.mockResolvedValue({
          data: { serverActions: Object.values(deniedOperate.decisions) },
        });
      }
      const onRun = vi.fn().mockResolvedValue({ status: "success" });
      const user = userEvent.setup();
      render(
        <Component
          service={{ ...service, suspended: true }}
          pending={null}
          onRun={onRun}
        />,
      );
      if (surface === "resource list") {
        await user.click(screen.getByRole("button", { name: /^Actions for / }));
        await user.click(screen.getByRole("menuitem", { name: "Resume" }));
      } else {
        await user.click(screen.getByRole("button", { name: "Resume" }));
      }
      expect(onRun).not.toHaveBeenCalled();
    },
  );
});

describe("ServiceRowActions", () => {
  it("retries suspend only after the exact protected-environment phrase", async () => {
    const onRun = vi
      .fn()
      .mockResolvedValueOnce({
        status: "confirmation_required",
        confirmation: "sudo suspend service app",
      })
      .mockResolvedValueOnce({ status: "success" });
    const user = userEvent.setup();
    render(
      <ServiceRowActions service={service} pending={null} onRun={onRun} />,
    );

    await user.click(screen.getByRole("button", { name: /^Actions for / }));
    await user.click(screen.getByRole("menuitem", { name: "Suspend" }));
    const ordinaryConfirm = await screen.findByRole("alertdialog");
    await user.click(
      within(ordinaryConfirm).getByRole("button", { name: "Suspend" }),
    );

    expect(onRun).toHaveBeenNthCalledWith(1, "suspend", service);
    const protectedDialog = await screen.findByRole("dialog");
    expect(
      within(protectedDialog).getByText("sudo suspend service app"),
    ).toBeInTheDocument();
    const input = within(protectedDialog).getByLabelText("Sudo Command");
    const retry = within(protectedDialog).getByRole("button", {
      name: "Suspend",
    });
    await user.type(input, "sudo suspend service ap");
    expect(retry).toBeDisabled();
    await user.type(input, "p");
    expect(retry).toBeEnabled();
    await user.click(retry);

    expect(onRun).toHaveBeenNthCalledWith(
      2,
      "suspend",
      service,
      "sudo suspend service app",
    );
  });

  it("dispatches zero forbidden lifecycle mutations for a viewer", async () => {
    serverState = {
      status: "ready",
      snapshot: deniedOperate,
      refresh: vi.fn().mockResolvedValue(undefined),
    };
    deployState = {
      status: "ready",
      snapshot: deniedDeploy,
      refresh: vi.fn().mockResolvedValue(undefined),
    };
    const onRun = vi.fn();
    const user = userEvent.setup();
    render(
      <ServiceRowActions service={service} pending={null} onRun={onRun} />,
    );

    await user.click(screen.getByRole("button", { name: /^Actions for / }));
    const suspend = screen.getByRole("menuitem", { name: "Suspend" });
    expect(suspend).toHaveAttribute("aria-disabled", "true");
    await user.click(suspend);
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(onRun).not.toHaveBeenCalled();
  });

  // w4/m578: in a protected environment only an Admin may suspend or resume.
  // A Developer reads Render's reason before the click, never a 403 after it,
  // while Restart (a deploy, which Render leaves open there) stays enabled.
  it("shows Render's admin-only reason on Suspend in a protected environment and sends nothing", async () => {
    serverState = {
      status: "ready",
      snapshot: toResourceSnapshot("tea-test", "app", [
        {
          action: "suspend",
          outcome: "denied",
          reason: "protected_environment_admin_required",
          precondition: null,
        },
      ]),
      refresh: vi.fn().mockResolvedValue(undefined),
    };
    const onRun = vi.fn();
    const user = userEvent.setup();
    render(
      <ServiceRowActions service={service} pending={null} onRun={onRun} />,
    );

    await user.click(screen.getByRole("button", { name: /^Actions for / }));
    const suspend = screen.getByRole("menuitem", { name: "Suspend" });
    expect(suspend).toHaveAttribute("aria-disabled", "true");
    await user.hover(suspend);
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      "This action is limited to admins.",
    );
    expect(
      screen.getByRole("menuitem", { name: "Restart" }),
    ).not.toHaveAttribute("aria-disabled");
    await user.click(suspend);
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(onRun).not.toHaveBeenCalled();
    expect(apolloQuery).not.toHaveBeenCalled();
  });

  // The live replay of w4/m578: a successful Suspend flips the row's state,
  // which re-asks the projection while the lifecycle hook still waits for the
  // service to converge. The open dialog must not then read the new answer
  // ("suspended") as a reason the call in flight cannot run.
  it.each([
    ["resource list", ServiceRowActions],
    ["settings", SuspendServiceCard],
  ] as const)(
    "%s: a dispatched Suspend's dialog reads no reason from the state it caused",
    async (surface, Component) => {
      const onRun = vi.fn(() => new Promise<ProtectedActionResult>(() => {}));
      const user = userEvent.setup();
      const { rerender } = render(
        <Component service={service} pending={null} onRun={onRun} />,
      );
      if (surface === "resource list") {
        await user.click(screen.getByRole("button", { name: /^Actions for / }));
        await user.click(screen.getByRole("menuitem", { name: "Suspend" }));
      } else {
        await user.click(screen.getByRole("button", { name: "Suspend" }));
      }
      const dialog = await screen.findByRole("alertdialog");
      await user.click(within(dialog).getByRole("button", { name: "Suspend" }));
      await waitFor(() => expect(onRun).toHaveBeenCalledTimes(1));

      // First the re-ask is in flight, then it answers for the new state.
      serverState = {
        status: "checking",
        refresh: vi.fn().mockResolvedValue(undefined),
      } as never;
      rerender(
        <Component
          service={{ ...service, suspended: true }}
          pending="suspend"
          onRun={onRun}
        />,
      );
      expect(
        screen.queryByText("Checking whether you can perform this action…"),
      ).not.toBeInTheDocument();
      serverState = {
        status: "ready",
        snapshot: toResourceSnapshot("tea-test", "app", [
          {
            action: "suspend",
            outcome: "allowed",
            reason: null,
            precondition: "suspended",
          },
          {
            action: "resume",
            outcome: "allowed",
            reason: null,
            precondition: null,
          },
        ]),
        refresh: vi.fn().mockResolvedValue(undefined),
      };
      rerender(
        <Component
          service={{ ...service, suspended: true }}
          pending="suspend"
          onRun={onRun}
        />,
      );
      expect(
        screen.queryByText(
          "Unavailable while the service is suspended. Resume it first.",
        ),
      ).not.toBeInTheDocument();
    },
  );

  it.each([
    ["Suspend", false],
    ["Resume", true],
  ])(
    "shows Render's admin-only reason on the Settings card's %s in a protected environment",
    async (name, suspended) => {
      serverState = {
        status: "ready",
        snapshot: toResourceSnapshot(
          "tea-test",
          "app",
          ["suspend", "resume"].map((action) => ({
            action,
            outcome: "denied",
            reason: "protected_environment_admin_required",
            precondition: null,
          })),
        ),
        refresh: vi.fn().mockResolvedValue(undefined),
      };
      const onRun = vi.fn();
      const user = userEvent.setup();
      render(
        <SuspendServiceCard
          service={{ ...service, suspended }}
          pending={null}
          onRun={onRun}
        />,
      );

      const control = screen.getByRole("button", { name });
      expect(control).toBeDisabled();
      await user.hover(control.parentElement!);
      expect(await screen.findByRole("tooltip")).toHaveTextContent(
        "This action is limited to admins.",
      );
      await user.click(control);
      expect(onRun).not.toHaveBeenCalled();
    },
  );

  it("gates restart on the deploy verb, not server restart", async () => {
    serverState = {
      status: "ready",
      snapshot: allowedServer,
      refresh: vi.fn().mockResolvedValue(undefined),
    };
    deployState = {
      status: "ready",
      snapshot: deniedDeploy,
      refresh: vi.fn().mockResolvedValue(undefined),
    };
    const onRun = vi.fn();
    const user = userEvent.setup();
    render(
      <ServiceRowActions service={service} pending={null} onRun={onRun} />,
    );

    await user.click(screen.getByRole("button", { name: /^Actions for / }));
    const restart = screen.getByRole("menuitem", { name: "Restart" });
    expect(restart).toHaveAttribute("aria-disabled", "true");
    await user.click(restart);
    expect(onRun).not.toHaveBeenCalled();
  });

  it("offers no Restart for a cron job (Render: not supported, w8/040)", async () => {
    const user = userEvent.setup();
    render(
      <ServiceRowActions
        service={{ ...service, type: "cron_job" }}
        pending={null}
        onRun={vi.fn()}
      />,
    );

    await user.click(screen.getByRole("button", { name: /^Actions for / }));
    expect(
      screen.getByRole("menuitem", { name: "Suspend" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: "Restart" }),
    ).not.toBeInTheDocument();
  });
});

describe.each([
  {
    language: "en",
    menu: /^Actions for /,
    suspend: "Suspend",
    resume: "Resume",
    cron: /pauses future scheduled runs\. An active run continues\. To stop it, select Cancel on the Runs page\./,
    cronResume: "Resuming this cron job resumes scheduled runs.",
    publicConfirm: /stops serving traffic\. Its URL and certificates are kept/,
    staticConfirm:
      /stops serving and answers 503 while suspended\. Its URL and certificates are kept/,
    privateConfirm: /scales to zero and stops running/,
    publicCard: /shut it down and stop it from serving traffic/,
    privateCard: /shut it down and stop it from running/,
  },
  {
    language: "zh",
    menu: /的操作$/,
    suspend: "暂停",
    resume: "恢复",
    cron: /暂停后续的计划运行。正在运行的任务会继续。如需停止它，请在“运行”页面中选择“取消”/,
    cronResume: "恢复此定时任务会恢复计划运行。",
    publicConfirm: /停止处理流量。其 URL 与证书会保留/,
    staticConfirm: /停止服务，并在暂停期间返回 503。其 URL 与证书会保留/,
    privateConfirm: /缩容至零并停止运行/,
    publicCard: /将其关闭并停止处理流量/,
    privateCard: /将其关闭并停止运行/,
  },
])("suspension descriptions ($language)", (copy) => {
  beforeEach(async () => {
    await i18n.changeLanguage(copy.language);
  });

  afterEach(async () => {
    await i18n.changeLanguage("en");
  });

  describe.each(["settings", "resource list"] as const)("%s", (surface) => {
    async function openSuspend(target: ServiceView) {
      const onRun = vi.fn().mockResolvedValue({ status: "success" });
      const user = userEvent.setup();
      render(
        surface === "settings" ? (
          <SuspendServiceCard service={target} pending={null} onRun={onRun} />
        ) : (
          <ServiceRowActions service={target} pending={null} onRun={onRun} />
        ),
      );
      if (surface === "settings") {
        await user.click(screen.getByRole("button", { name: copy.suspend }));
      } else {
        await user.click(screen.getByRole("button", { name: copy.menu }));
        await user.click(screen.getByRole("menuitem", { name: copy.suspend }));
      }
      return { user, onRun, dialog: await screen.findByRole("alertdialog") };
    }

    it("explains schedule-only suspension and how to stop an active cron run", async () => {
      const cron = { ...service, type: "cron_job" };
      const { user, onRun, dialog } = await openSuspend(cron);

      expect(within(dialog).getByText(copy.cron)).toBeInTheDocument();
      if (surface === "settings") {
        expect(screen.getAllByText(copy.cron)).toHaveLength(2);
      }
      await user.click(
        within(dialog).getByRole("button", { name: copy.suspend }),
      );
      await waitFor(() => expect(onRun).toHaveBeenCalledWith("suspend", cron));
      expect(onRun).toHaveBeenCalledTimes(1);
    });

    it.each([
      ["web_service", true, "publicConfirm"],
      ["static_site", true, "staticConfirm"],
      ["private_service", false, "privateConfirm"],
      ["background_worker", false, "privateConfirm"],
    ] as const)(
      "keeps %s suspension semantics",
      async (type, publicUrl, confirmCopy) => {
        const { dialog } = await openSuspend({ ...service, type });
        expect(within(dialog).getByText(copy[confirmCopy])).toBeInTheDocument();
        if (surface === "settings") {
          expect(
            screen.getByText(publicUrl ? copy.publicCard : copy.privateCard),
          ).toBeInTheDocument();
        }
        expect(screen.queryByText(copy.cron)).not.toBeInTheDocument();
      },
    );
  });

  it("describes resuming the cron schedule and dispatches resume", async () => {
    const cron = { ...service, type: "cron_job", suspended: true };
    const onRun = vi.fn().mockResolvedValue({ status: "success" });
    const user = userEvent.setup();
    render(<SuspendServiceCard service={cron} pending={null} onRun={onRun} />);

    expect(screen.getByText(copy.cronResume)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: copy.resume }));
    expect(onRun).toHaveBeenCalledWith("resume", cron);
  });
});
