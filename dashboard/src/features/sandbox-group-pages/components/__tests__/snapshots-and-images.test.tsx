import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
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
  snapshots: {} as Record<string, unknown>,
  snapshotCalls: [] as Record<string, unknown>[],
  started: {} as Record<string, unknown>,
  remove: vi.fn(),
  sources: {} as Record<string, unknown>,
  versions: {} as Record<string, unknown>,
}));

vi.mock(
  "@/features/sandbox-group-pages/hooks/use-group-pages",
  async (importOriginal) => ({
    ...(await importOriginal<
      typeof import("@/features/sandbox-group-pages/hooks/use-group-pages")
    >()),
    useGroupSnapshots: (q: Record<string, unknown>) => {
      state.snapshotCalls.push(q);
      return state.snapshots;
    },
    useSandboxesStartedFrom: () => state.started,
    useDeleteSnapshot: () => ({ remove: state.remove, pending: false }),
    useSnapshotDetails: () => ({
      snapshot: null,
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    }),
    useGroupImageSources: () => state.sources,
    useImageVersions: () => state.versions,
  }),
);

import { SnapshotsPage } from "@/features/sandbox-group-pages/components/snapshots-page";
import { ImagesPage } from "@/features/sandbox-group-pages/components/images-page";
import type { ImageDrawerTab } from "@/features/sandbox-group-pages/lib/drawer-tabs";

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

function snapshot(
  id: string,
  status: string,
  over: Record<string, unknown> = {},
) {
  return {
    __typename: "SandboxSnapshot" as const,
    id,
    sandboxGroupId: GROUP.id,
    name: `snap-${id.slice(-1)}`,
    kind: "filesystem",
    status,
    sourceSandboxId: "sbx-0000000000000000000a",
    planSpec: {
      __typename: "SandboxPlanSpec" as const,
      name: "1c-2g",
      cpus: 1,
      memoryGb: 2,
    },
    sizeBytes: 1024,
    failureCode: null,
    requestedAt: "2026-11-26T10:00:00Z",
    capturedAt: status === "available" ? "2026-11-26T10:01:00Z" : null,
    expiresAt: "2026-12-03T10:00:00Z",
    ttlSeconds: 604800,
    ...over,
  };
}

function wrap(children: ReactNode) {
  return (
    <SandboxGroupDetailProvider value={{ group: GROUP, refetch: vi.fn() }}>
      {children}
    </SandboxGroupDetailProvider>
  );
}

describe("SnapshotsPage (w1/m658)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    state.snapshotCalls = [];
    state.snapshots = {
      snapshots: [
        snapshot("snp-0000000000000000000a", "available", {
          ttlSeconds: 0,
          expiresAt: "9999-12-31T23:59:59Z",
        }),
        snapshot("snp-0000000000000000000b", "failed", {
          failureCode: "DEADLINE_EXCEEDED",
        }),
        snapshot("snp-0000000000000000000c", "creating"),
      ],
      cursor: null,
      hasMore: false,
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    };
    state.started = { sandboxes: [], loading: false, error: undefined };
    state.remove = vi.fn().mockResolvedValue(undefined);
  });

  it("lists Render's statuses, failure texts and Never", () => {
    render(
      wrap(
        <SnapshotsPage
          openSnapshotId={undefined}
          onOpen={vi.fn()}
          onClose={vi.fn()}
        />,
      ),
    );
    const table = screen.getAllByRole("table")[0];
    expect(within(table).getByText("Available")).toBeInTheDocument();
    expect(within(table).getByText("Capturing")).toBeInTheDocument();
    expect(within(table).getByText("Failed")).toBeInTheDocument();
    expect(
      within(table).getByText("This capture timed out."),
    ).toBeInTheDocument();
    expect(within(table).getByText("Never")).toBeInTheDocument();
  });

  it("searches the group's snapshots", async () => {
    render(
      wrap(
        <SnapshotsPage
          openSnapshotId={undefined}
          onOpen={vi.fn()}
          onClose={vi.fn()}
        />,
      ),
    );
    await userEvent.type(
      screen.getByRole("searchbox", { name: "Search snapshots" }),
      "nightly",
    );
    await waitFor(() =>
      expect(state.snapshotCalls.at(-1)).toMatchObject({
        search: "nightly",
        groupId: GROUP.id,
      }),
    );
  });

  it("deletes a snapshot after its sudo phrase", async () => {
    render(
      wrap(
        <SnapshotsPage
          openSnapshotId="snp-0000000000000000000a"
          onOpen={vi.fn()}
          onClose={vi.fn()}
        />,
      ),
    );
    const drawer = screen.getByRole("dialog");
    expect(within(drawer).getByText("Kept until deleted")).toBeInTheDocument();
    await userEvent.click(
      within(drawer).getByRole("button", { name: "Delete snapshot" }),
    );
    const confirm = await screen.findByRole("alertdialog");
    const button = within(confirm).getByRole("button", {
      name: "Delete snapshot",
    });
    expect(button).toBeDisabled();
    await userEvent.type(
      within(confirm).getByRole("textbox"),
      "sudo delete snapshot snap-a",
    );
    await userEvent.click(button);
    expect(state.remove).toHaveBeenCalledWith("snp-0000000000000000000a");
    expect(toasts.success).toHaveBeenCalledWith("Deleted snapshot snap-a");
  });

  it("shows bex-api's refusal of a capture still running", async () => {
    state.remove = vi.fn().mockRejectedValue(
      new CombinedGraphQLErrors({
        errors: [
          {
            message:
              "snapshot snp-0000000000000000000c is still being captured; delete it once it is available or failed",
            extensions: { code: "snapshot_creating" },
          },
        ],
      }),
    );
    render(
      wrap(
        <SnapshotsPage
          openSnapshotId="snp-0000000000000000000c"
          onOpen={vi.fn()}
          onClose={vi.fn()}
        />,
      ),
    );
    await userEvent.click(
      within(screen.getByRole("dialog")).getByRole("button", {
        name: "Delete snapshot",
      }),
    );
    const confirm = await screen.findByRole("alertdialog");
    await userEvent.type(
      within(confirm).getByRole("textbox"),
      "sudo delete snapshot snap-c",
    );
    await userEvent.click(
      within(confirm).getByRole("button", { name: "Delete snapshot" }),
    );
    expect(toasts.error).toHaveBeenCalledWith(
      expect.stringContaining("still being captured"),
    );
  });

  it("starts an empty group with Render's snapshot starter code", () => {
    state.snapshots = { ...state.snapshots, snapshots: [] };
    render(
      wrap(
        <SnapshotsPage
          openSnapshotId={undefined}
          onOpen={vi.fn()}
          onClose={vi.fn()}
        />,
      ),
    );
    expect(screen.getByText("Create your first snapshot")).toBeInTheDocument();
    expect(
      screen.getByText(
        `bex sandboxes create --from my-first-snapshot --group ${GROUP.id}`,
      ),
    ).toBeInTheDocument();
  });
});

function build(id: string, status: string, over: Record<string, unknown> = {}) {
  return {
    __typename: "SandboxImage" as const,
    id,
    sandboxGroupId: GROUP.id,
    name: "runtime",
    source: {
      __typename: "SandboxImageSource" as const,
      repository: "ghcr.io",
      image: "acme/runtime",
      reference: "v2",
    },
    sourceDigest: "sha256:" + "a".repeat(64),
    status,
    failureReason: null,
    createdAt: "2026-11-26T10:00:00Z",
    finishedAt: null,
    ...over,
  };
}

describe("ImagesPage (w1/m658)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    const failed = build("sbb-0000000000000000000b", "failed", {
      failureReason: "fetch source image: manifest unknown",
    });
    const ok = build("sbb-0000000000000000000a", "succeeded", {
      source: {
        __typename: "SandboxImageSource",
        repository: "ghcr.io",
        image: "acme/runtime",
        reference: "v1",
      },
    });
    state.sources = {
      sources: [
        {
          __typename: "SandboxBuildSource",
          name: "runtime",
          source: failed.source,
          latestBuild: failed,
          latestSucceededBuild: ok,
        },
        {
          __typename: "SandboxBuildSource",
          name: "tools",
          source: failed.source,
          latestBuild: build("sbb-0000000000000000000c", "running", {
            name: "tools",
          }),
          latestSucceededBuild: null,
        },
        {
          __typename: "SandboxBuildSource",
          name: "broken",
          source: failed.source,
          latestBuild: build("sbb-0000000000000000000d", "failed", {
            name: "broken",
          }),
          latestSucceededBuild: null,
        },
      ],
      cursor: null,
      hasMore: false,
      loading: false,
      error: undefined,
      refetch: vi.fn(),
    };
    state.versions = {
      versions: [failed, ok],
      loading: false,
      error: undefined,
    };
    state.started = { sandboxes: [], loading: false, error: undefined };
  });

  it("reads each name's status from its builds, with Render's failure words", () => {
    render(
      wrap(
        <ImagesPage
          openImage={undefined}
          openTab="versions"
          onOpen={vi.fn()}
          onClose={vi.fn()}
        />,
      ),
    );
    const table = screen.getAllByRole("table")[0];
    const rows = within(table).getAllByRole("row").slice(1);
    expect(within(rows[0]).getByText("Available")).toBeInTheDocument();
    expect(
      within(rows[0]).getByText(
        "bex couldn't find the image. Check the image name and tag.",
      ),
    ).toBeInTheDocument();
    expect(within(rows[1]).getByText("Importing")).toBeInTheDocument();
    expect(within(rows[2]).getByText("Unavailable")).toBeInTheDocument();
  });

  it.each<[ImageDrawerTab, string]>([
    ["versions", "Latest"],
    ["details", "Latest available version"],
    ["sandboxes", "No sandboxes have started from this image."],
  ])("opens the drawer's %s tab", (tab, text) => {
    render(
      wrap(
        <ImagesPage
          openImage="runtime"
          openTab={tab}
          onOpen={vi.fn()}
          onClose={vi.fn()}
        />,
      ),
    );
    expect(
      within(screen.getByRole("dialog")).getByText(text),
    ).toBeInTheDocument();
  });

  it("starts an empty group with Render's import starter code", () => {
    state.sources = { ...state.sources, sources: [] };
    render(
      wrap(
        <ImagesPage
          openImage={undefined}
          openTab="versions"
          onOpen={vi.fn()}
          onClose={vi.fn()}
        />,
      ),
    );
    expect(screen.getByText("Import your first image")).toBeInTheDocument();
    expect(
      screen.getByText(
        /bex sandbox-groups images import ghcr.io\/acme\/agent-runtime:v1/,
      ),
    ).toBeInTheDocument();
  });
});
