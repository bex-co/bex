import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SandboxGroupDetailProvider } from "@/features/sandbox-group-pages/lib/group-detail";

const toasts = vi.hoisted(() => ({
  success: vi.fn(),
  error: vi.fn(),
  info: vi.fn(),
}));
vi.mock("sonner", () => ({ toast: toasts }));

vi.mock("@tanstack/react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  Link: ({
    to,
    search,
    children,
    ...rest
  }: {
    to: string;
    search?: Record<string, string>;
    children: ReactNode;
  }) => (
    <a href={`${to}?${new URLSearchParams(search ?? {}).toString()}`} {...rest}>
      {children}
    </a>
  ),
}));

const state = vi.hoisted(() => ({
  details: {} as Record<string, unknown>,
  events: {} as Record<string, unknown>,
  snapshots: {} as Record<string, unknown>,
  terminate: vi.fn(),
}));

vi.mock(
  "@/features/sandbox-group-pages/hooks/use-group-pages",
  async (importOriginal) => ({
    ...(await importOriginal<
      typeof import("@/features/sandbox-group-pages/hooks/use-group-pages")
    >()),
    useSandboxDetails: () => state.details,
    useSandboxEvents: () => state.events,
    useGroupSnapshots: () => state.snapshots,
    useTerminateSandboxes: () => ({
      terminate: state.terminate,
      pending: false,
    }),
  }),
);

import { SandboxDrawer } from "@/features/sandbox-group-pages/components/sandbox-drawer";
import type { SandboxDrawerTab } from "@/features/sandbox-group-pages/lib/drawer-tabs";

const GROUP = {
  __typename: "SandboxGroup" as const,
  id: "sbg-dc44tha9086ovmjtv6c0",
  name: "agents",
  region: "oregon",
  isDefault: false,
  environmentId: null,
  concurrencyLimit: 20,
  ownerId: "tea-a",
  createdAt: null,
  updatedAt: null,
};

const SANDBOX = {
  __typename: "Sandbox" as const,
  id: "sbx-0000000000000000000a",
  name: "alpha",
  status: "running",
  createdAt: "2026-11-26T10:00:00Z",
  startedAt: "2026-11-26T10:00:05Z",
  endedAt: null,
  errorReason: null,
  timeoutSeconds: 3600,
  timeoutAt: "2026-11-26T11:00:05Z",
  idleTimeoutSeconds: 0,
  region: "oregon",
  image: "snp-0000000000000000000s",
  plan: "2c-4g",
  planSpec: {
    __typename: "SandboxPlanSpec" as const,
    name: "2c-4g",
    cpus: 2,
    memoryGb: 4,
  },
  networkPolicy: {
    __typename: "SandboxNetworkPolicy" as const,
    type: "allow-list",
    rules: [
      {
        __typename: "SandboxNetworkPolicyRule" as const,
        domain: "api.example.com",
      },
    ],
  },
  sourceSnapshotId: "snp-0000000000000000000s",
  sourceBuildId: null,
  sourceBuild: null,
  source: {
    __typename: "SandboxEmailSource" as const,
    email: "ada@example.com",
  },
  trigger: "CLI",
  sandboxGroupId: GROUP.id,
};

const EVENTS = [
  {
    __typename: "SandboxExecutionEvent",
    id: "exe-2",
    timestamp: "2026-11-26T10:02:00Z",
    outcome: "ERROR",
    label: "false",
    summary: "false",
    executionId: "exe-2",
    executionType: "runs",
    operation: "sync",
    fullCommand: "false",
    exitCode: 1,
    startedAt: null,
    completedAt: null,
  },
  {
    __typename: "SandboxSnapshotEvent",
    id: "snp-0000000000000000000t",
    timestamp: "2026-11-26T10:01:00Z",
    outcome: "ERROR",
    label: "Snapshot",
    snapshot: {
      __typename: "SandboxSnapshot",
      id: "snp-0000000000000000000t",
      kind: "filesystem",
      status: "failed",
      failureCode: "DEADLINE_EXCEEDED",
      requestedAt: null,
      capturedAt: null,
      sizeBytes: null,
    },
  },
  {
    __typename: "SandboxLifecycleEvent",
    id: "evt-1",
    timestamp: "2026-11-26T10:00:00Z",
    outcome: "SUCCESS",
    label: "Created",
    lifecycleType: "CREATED",
    trigger: "CLI",
    userEmail: "ada@example.com",
  },
];

function renderDrawer(tab: SandboxDrawerTab = "events") {
  const onTabChange = vi.fn();
  const onTerminated = vi.fn();
  render(
    <SandboxGroupDetailProvider value={{ group: GROUP, refetch: vi.fn() }}>
      <SandboxDrawer
        sandboxId={SANDBOX.id}
        tab={tab}
        onTabChange={onTabChange}
        onClose={vi.fn()}
        onTerminated={onTerminated}
      />
    </SandboxGroupDetailProvider>,
  );
  return { onTabChange, onTerminated };
}

describe("SandboxDrawer (w1/m658)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    state.details = {
      sandbox: SANDBOX,
      loading: false,
      notFound: false,
      error: undefined,
      refetch: vi.fn(),
    };
    state.events = {
      events: EVENTS,
      hasMore: true,
      loading: false,
      error: undefined,
      loadEarlier: vi.fn(),
      loadingEarlier: false,
      earlierError: false,
    };
    state.snapshots = { snapshots: [], loading: false, error: undefined };
    state.terminate = vi.fn().mockResolvedValue([]);
  });

  it("lists the events newest first with each kind's words, and loads earlier ones", async () => {
    renderDrawer();
    const items = within(screen.getByRole("dialog")).getAllByRole("listitem");
    expect(items.map((li) => li.textContent)).toEqual([
      expect.stringContaining("Exit code 1"),
      expect.stringContaining("This capture timed out."),
      expect.stringContaining("CLI · ada@example.com"),
    ]);
    expect(screen.getByRole("link", { name: "View snapshot" })).toHaveAttribute(
      "href",
      expect.stringContaining("snapshot=snp-0000000000000000000t"),
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Load earlier events" }),
    );
    expect(state.events.loadEarlier).toHaveBeenCalled();
  });

  it("puts the tab in the URL", async () => {
    const { onTabChange } = renderDrawer();
    await userEvent.click(screen.getByRole("tab", { name: "Details" }));
    expect(onTabChange).toHaveBeenCalledWith("details");
  });

  it("shows Render's details: plan, timeouts, network, where it started from and who started it", () => {
    renderDrawer("details");
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText("2c-4g (2 CPU, 4 GB)")).toBeInTheDocument();
    expect(within(dialog).getByText("1h")).toBeInTheDocument();
    expect(within(dialog).getByText("Off")).toBeInTheDocument();
    expect(within(dialog).getByText("Allow list")).toBeInTheDocument();
    expect(within(dialog).getByText("api.example.com")).toBeInTheDocument();
    expect(
      within(dialog).getByRole("link", { name: "Go to snapshot" }),
    ).toHaveAttribute(
      "href",
      expect.stringContaining("snapshot=snp-0000000000000000000s"),
    );
    expect(within(dialog).getByText("ada@example.com")).toBeInTheDocument();
  });

  it("says a running sandbox has no snapshots yet", () => {
    renderDrawer("snapshots");
    expect(
      screen.getByText(/No snapshots of this sandbox yet/),
    ).toBeInTheDocument();
  });

  it("terminates the sandbox from its menu", async () => {
    const { onTerminated } = renderDrawer();
    await userEvent.click(
      screen.getByRole("button", { name: "Sandbox actions" }),
    );
    await userEvent.click(
      await screen.findByRole("menuitem", { name: "Terminate sandbox" }),
    );
    const dialog = await screen.findByRole("alertdialog");
    await userEvent.click(
      within(dialog).getByRole("button", { name: "Terminate" }),
    );
    expect(state.terminate).toHaveBeenCalledWith([SANDBOX.id]);
    expect(onTerminated).toHaveBeenCalled();
  });

  it("says a sandbox that is gone, or of another group, is not found", () => {
    state.details = {
      sandbox: null,
      loading: false,
      notFound: true,
      error: undefined,
      refetch: vi.fn(),
    };
    renderDrawer();
    expect(screen.getByText("Sandbox not found")).toBeInTheDocument();
  });
});
