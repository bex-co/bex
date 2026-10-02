import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ServiceRowActions } from "@/features/services/components/service-row-actions";
import { SuspendServiceCard } from "@/features/services/components/suspend-service-card";
import type { ServiceView } from "@/features/services/types";
import { toResourceSnapshot } from "@/features/capabilities/lib/resource-actions";
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

const apolloQuery = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useApolloClient: () => ({ query: apolloQuery }),
  useQuery: vi.fn(),
  useMutation: vi.fn(() => [vi.fn(), { loading: false }]),
}));

vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-test" }),
}));

const allowedServer = toResourceSnapshot("tea-test", "app", [
  { action: "suspend", outcome: "allowed", reason: null, precondition: null },
  { action: "resume", outcome: "allowed", reason: null, precondition: null },
  { action: "restart", outcome: "allowed", reason: null, precondition: null },
]);
const allowedDeploy = toResourceSnapshot("tea-test", "app", [
  { action: "deploy", outcome: "allowed", reason: null, precondition: null },
  { action: "cancel_deploy", outcome: "allowed", reason: null, precondition: null },
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
      serverActions: [
        { action: "suspend", outcome: "allowed", reason: null, precondition: null },
      ],
    },
  });
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

    await user.click(screen.getByRole("button", { name: "Open actions menu" }));
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

    await user.click(screen.getByRole("button", { name: "Open actions menu" }));
    const suspend = screen.getByRole("menuitem", { name: "Suspend" });
    expect(suspend).toHaveAttribute("aria-disabled", "true");
    await user.click(suspend);
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(onRun).not.toHaveBeenCalled();
  });

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

    await user.click(screen.getByRole("button", { name: "Open actions menu" }));
    const restart = screen.getByRole("menuitem", { name: "Restart" });
    expect(restart).toHaveAttribute("aria-disabled", "true");
    await user.click(restart);
    expect(onRun).not.toHaveBeenCalled();
  });
});

describe.each([
  {
    language: "en",
    menu: "Open actions menu",
    suspend: "Suspend",
    resume: "Resume",
    cron: /pauses future scheduled runs\. An active run continues\. To stop it, select Cancel in Recent Runs\./,
    cronResume: "Resuming this cron job resumes scheduled runs.",
    publicConfirm: /stops serving traffic\. Its URL and certificates are kept/,
    privateConfirm: /scales to zero and stops running/,
    publicCard: /shut it down and stop it from serving traffic/,
    privateCard: /shut it down and stop it from running/,
  },
  {
    language: "zh",
    menu: "打开操作菜单",
    suspend: "暂停",
    resume: "恢复",
    cron: /暂停后续的计划运行。正在运行的任务会继续。如需停止它，请在“最近运行”中选择“取消”/,
    cronResume: "恢复此定时任务会恢复计划运行。",
    publicConfirm: /停止处理流量。其 URL 与证书会保留/,
    privateConfirm: /缩容至零并停止运行/,
    publicCard: /关闭它并停止流量服务/,
    privateCard: /关闭它并停止运行/,
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
      await user.click(within(dialog).getByRole("button", { name: copy.suspend }));
      await waitFor(() => expect(onRun).toHaveBeenCalledWith("suspend", cron));
      expect(onRun).toHaveBeenCalledTimes(1);
    });

    it.each([
      ["web_service", true],
      ["static_site", true],
      ["private_service", false],
      ["background_worker", false],
    ] as const)("keeps %s suspension semantics", async (type, publicUrl) => {
      const { dialog } = await openSuspend({ ...service, type });
      expect(
        within(dialog).getByText(publicUrl ? copy.publicConfirm : copy.privateConfirm),
      ).toBeInTheDocument();
      if (surface === "settings") {
        expect(
          screen.getByText(publicUrl ? copy.publicCard : copy.privateCard),
        ).toBeInTheDocument();
      }
      expect(screen.queryByText(copy.cron)).not.toBeInTheDocument();
    });
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
