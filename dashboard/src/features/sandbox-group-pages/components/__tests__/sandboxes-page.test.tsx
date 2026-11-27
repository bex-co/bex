import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
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
  Link: ({ to, children, ...rest }: { to: string; children: ReactNode }) => (
    <a href={to} {...rest}>
      {children}
    </a>
  ),
}));

const state = vi.hoisted(() => ({
  list: {} as Record<string, unknown>,
  calls: [] as Record<string, unknown>[],
  terminate: vi.fn(),
}));

vi.mock(
  "@/features/sandbox-group-pages/hooks/use-group-pages",
  async (importOriginal) => ({
    ...(await importOriginal<
      typeof import("@/features/sandbox-group-pages/hooks/use-group-pages")
    >()),
    useGroupSandboxes: (q: Record<string, unknown>) => {
      state.calls.push(q);
      return state.list;
    },
    useTerminateSandboxes: () => ({
      terminate: state.terminate,
      pending: false,
    }),
  }),
);

vi.mock(
  "@/features/sandbox-group-pages/components/sandbox-drawer",
  async (importOriginal) => ({
    ...(await importOriginal<
      typeof import("@/features/sandbox-group-pages/components/sandbox-drawer")
    >()),
    SandboxDrawer: ({ sandboxId }: { sandboxId: string }) => (
      <div>drawer {sandboxId}</div>
    ),
  }),
);

import { SandboxesPage } from "@/features/sandbox-group-pages/components/sandboxes-page";
import { SandboxesPageSkeleton } from "@/features/sandbox-group-pages/components/group-page-skeletons";

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

function row(id: string, status: string, name: string | null = null) {
  return {
    __typename: "Sandbox" as const,
    id,
    name,
    status,
    createdAt: "2026-11-26T10:00:00Z",
    startedAt: "2026-11-26T10:00:05Z",
    endedAt: status === "terminated" ? "2026-11-26T10:05:00Z" : null,
    errorReason: null,
    timeoutSeconds: 3600,
    timeoutAt: null,
    idleTimeoutSeconds: 900,
    region: "oregon",
    image: null,
    plan: "1c-2g",
    planSpec: null,
    networkPolicy: null,
    sourceSnapshotId: null,
    sourceBuildId: null,
    sourceBuild: null,
    source: null,
    trigger: "DASHBOARD",
    sandboxGroupId: GROUP.id,
  };
}

function listState(over: Record<string, unknown> = {}) {
  return {
    sandboxes: [
      row("sbx-0000000000000000000a", "running", "alpha"),
      row("sbx-0000000000000000000b", "running"),
      row("sbx-0000000000000000000c", "terminated"),
    ],
    cursor: "sbx-0000000000000000000c",
    hasMore: true,
    loading: false,
    error: undefined,
    refetch: vi.fn(),
    runs: {
      total: 4,
      errored: 1,
      succeeded: 3,
      buckets: [{ time: "2026-11-26T10:00:00Z", succeeded: 3, errored: 1 }],
    },
    running: 2,
    ...over,
  };
}

function renderPage(props: Partial<Parameters<typeof SandboxesPage>[0]> = {}) {
  const onOpen = vi.fn();
  render(
    <SandboxGroupDetailProvider value={{ group: GROUP, refetch: vi.fn() }}>
      <SandboxesPage
        openSandboxId={undefined}
        openTab="events"
        onOpen={onOpen}
        onClose={vi.fn()}
        {...props}
      />
    </SandboxGroupDetailProvider>,
  );
  return { onOpen };
}

const lastCall = () => state.calls.at(-1) ?? {};

describe("SandboxesPage (w1/m658)", () => {
  beforeEach(() => {
    state.calls = [];
    state.list = listState();
    state.terminate = vi.fn();
    vi.clearAllMocks();
  });

  it("shows Total, Errored and Running over the range, on the last 7 days", () => {
    renderPage();
    expect(lastCall()).toMatchObject({
      groupId: GROUP.id,
      ownerId: "tea-a",
      range: "7d",
      live: true,
      status: null,
    });
    expect(screen.getByText("25%")).toBeInTheDocument();
    expect(screen.getByText("4")).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
    expect(screen.getAllByText("alpha").length).toBeGreaterThan(0);
  });

  it("searches by name or id, and turns a status word into a status filter", async () => {
    renderPage();
    const box = screen.getByRole("searchbox", {
      name: "Search by name, sandbox ID, or status",
    });
    await userEvent.type(box, "alp");
    // The search is debounced before it is asked.
    await waitFor(() =>
      expect(lastCall()).toMatchObject({ search: "alp", status: null }),
    );
    await userEvent.clear(box);
    await userEvent.type(box, "Running");
    await waitFor(() =>
      expect(lastCall()).toMatchObject({ status: "running" }),
    );
  });

  it("pages forward with the answer's cursor and back again", async () => {
    renderPage();
    await userEvent.click(screen.getByRole("button", { name: /Next/ }));
    expect(lastCall()).toMatchObject({ cursor: "sbx-0000000000000000000c" });
    await userEvent.click(screen.getByRole("button", { name: /Previous/ }));
    expect(lastCall()).toMatchObject({ cursor: undefined });
  });

  it("holds the list still while live updates are paused", async () => {
    renderPage();
    await userEvent.click(
      screen.getByRole("button", { name: /Pause live updates/ }),
    );
    expect(lastCall()).toMatchObject({ live: false });
    await userEvent.click(
      screen.getByRole("button", { name: /Resume live updates/ }),
    );
    expect(lastCall()).toMatchObject({ live: true });
  });

  // t009's second production check: Russian's "Pause live updates" is 383
  // px wide and pushed the columns button 49 px past 390 px's column, so the
  // page scrolled sideways. The button now shrinks and its label truncates.
  it("lets the live-updates label shrink instead of overflowing the toolbar", () => {
    renderPage();
    const button = screen.getByRole("button", { name: /Pause live updates/ });
    expect(button).toHaveClass("min-w-0", "shrink");
    expect(button).not.toHaveClass("shrink-0");
    expect(button.parentElement).toHaveClass("min-w-0");
    expect(within(button).getByText("Pause live updates")).toHaveClass(
      "truncate",
    );
  });

  it("terminates the selected sandboxes and names the ones that failed", async () => {
    state.terminate.mockResolvedValue(["sbx-0000000000000000000b"]);
    renderPage();
    const table = screen.getAllByRole("table")[0];
    await userEvent.click(
      within(table).getByRole("checkbox", { name: "Select alpha" }),
    );
    await userEvent.click(
      within(table).getByRole("checkbox", {
        name: "Select sbx-0000000000000000000b",
      }),
    );
    expect(
      within(table).getByRole("checkbox", {
        name: "Select sbx-0000000000000000000c",
      }),
    ).toBeDisabled();
    await userEvent.click(screen.getByRole("button", { name: "Terminate" }));
    const dialog = await screen.findByRole("alertdialog");
    await userEvent.click(
      within(dialog).getByRole("button", { name: "Terminate" }),
    );
    expect(state.terminate).toHaveBeenCalledWith([
      "sbx-0000000000000000000a",
      "sbx-0000000000000000000b",
    ]);
    expect(toasts.error).toHaveBeenCalledWith(
      "Failed to terminate 1 sandbox. Please try again.",
    );
  });

  it("opens a sandbox's drawer from its row", async () => {
    const { onOpen } = renderPage();
    await userEvent.click(screen.getAllByRole("button", { name: "alpha" })[0]);
    expect(onOpen).toHaveBeenCalledWith("sbx-0000000000000000000a");
  });

  it("starts an empty group with Render's CLI and SDK starter code", async () => {
    state.list = listState({ sandboxes: [], hasMore: false, cursor: null });
    renderPage();
    expect(screen.getByText("Create your first sandbox")).toBeInTheDocument();
    expect(
      screen.getByText(`bex sandboxes create --group ${GROUP.id} --wait`),
    ).toBeInTheDocument();
    await userEvent.click(screen.getByRole("tab", { name: "SDK" }));
    expect(
      screen.getByText(/SANDBOX_GROUP_ID = "sbg-dc44tha9086ovmjtv6c0"/),
    ).toBeInTheDocument();
  });

  it("says no sandbox matches a search, and clears it", async () => {
    state.list = listState({ sandboxes: [], hasMore: false, cursor: null });
    renderPage();
    await userEvent.type(screen.getByRole("searchbox"), "zzz");
    expect(screen.getByText("No matching sandboxes")).toBeInTheDocument();
    await userEvent.click(
      screen.getByRole("button", { name: "Clear filters" }),
    );
    expect(lastCall()).toMatchObject({ search: "", range: "7d" });
  });

  it("offers Try again when the list cannot load", async () => {
    const refetch = vi.fn();
    state.list = listState({
      sandboxes: [],
      error: new Error("down"),
      refetch,
    });
    renderPage();
    expect(screen.getByText("Couldn't load sandboxes")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /Try again/ }));
    expect(refetch).toHaveBeenCalled();
  });

  it("renders the open sandbox's drawer from the URL", () => {
    renderPage({ openSandboxId: "sbx-0000000000000000000b" });
    expect(
      screen.getByText("drawer sbx-0000000000000000000b"),
    ).toBeInTheDocument();
  });
});

/** The summary's boxes as their classes size them: each card's header, label
 *  and value, and the chart's legend line. */
function summaryShape(root: Element) {
  const summary = root.querySelector("[data-skeleton-region=run-summary]")!;
  return {
    cards: [...summary.querySelectorAll("[data-slot=card]")].map((card) =>
      [
        card,
        card.querySelector("[data-slot=card-header]"),
        card.querySelector("[data-slot=card-description]"),
        card.querySelector("[data-slot=card-title]"),
      ].map((el) => el?.className),
    ),
    chart: summary.querySelector(".h-\\[72px\\]") !== null,
    legend: summary.querySelector(".mt-2")?.className,
  };
}

/** The first table row and the first stacked row, as their lines size them. */
function rowShape(root: Element) {
  const cells = [...root.querySelector("tbody tr")!.children];
  const stacked = root.querySelector(".md\\:hidden > *")!;
  return {
    cells: cells.length,
    nameLines: cells[1].children.length,
    stackedLines: stacked.querySelector(".space-y-1")!.children.length,
  };
}

// t009 measured the first frame 14 px short in the summary on production (a
// 14 px label bar and a 20 px value bar under 20 and 24 px lines) and each
// table row 12 px short (one bar where the row shows a name over its id).
describe("SandboxesPageSkeleton matches the ready page (w1/m658 t009)", () => {
  beforeEach(() => {
    state.calls = [];
    state.list = listState();
  });

  it("draws the summary's cards and legend and the rows' lines as the page does", () => {
    const pending = render(<SandboxesPageSkeleton />);
    const frame = {
      summary: summaryShape(pending.container),
      rows: rowShape(pending.container),
    };
    pending.unmount();

    const ready = render(
      <SandboxGroupDetailProvider value={{ group: GROUP, refetch: vi.fn() }}>
        <SandboxesPage
          openSandboxId={undefined}
          openTab="events"
          onOpen={vi.fn()}
          onClose={vi.fn()}
        />
      </SandboxGroupDetailProvider>,
    );
    expect(summaryShape(ready.container)).toEqual(frame.summary);
    expect(rowShape(ready.container)).toEqual(frame.rows);
    expect(frame.rows).toEqual({ cells: 7, nameLines: 2, stackedLines: 3 });
  });
});
