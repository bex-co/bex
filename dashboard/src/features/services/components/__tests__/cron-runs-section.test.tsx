import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CronRunsSection } from "@/features/services/components/cron-runs-section";
import type { CronRunView, ServiceView } from "@/features/services/types";
import { toResourceSnapshot } from "@/features/capabilities/lib/resource-actions";
import { CRON_RUNS_PAGE_SIZE } from "@/features/services/lib/cron-run-list";
import { ServiceCronRunsSkeleton } from "@/common/components/route-skeletons";
import { useCronRuns } from "@/features/services/hooks/use-cron-runs";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  ACTIVITY_REASON,
  billingCapabilities,
  viewerCapabilities,
} from "@/test/mocks/capabilities";

// Links render as plain anchors carrying their target, every other prop passed
// through, so a test reads the window a row opens without a router.
vi.mock("@tanstack/react-router", () => ({
  Link: ({
    to,
    params,
    search,
    ...rest
  }: {
    to: string;
    params: { serviceId: string };
    search?: Record<string, string>;
  } & React.AnchorHTMLAttributes<HTMLAnchorElement>) => (
    <a
      {...rest}
      href={`${to.replace("$serviceId", params.serviceId)}?${new URLSearchParams(search)}`}
    />
  ),
}));

const cancel = vi.fn();
const loadMore = vi.fn();
let runs: CronRunView[] = [];
let hasMore = false;
let loading = false;
let error = false;

vi.mock("@/features/services/hooks/use-cron-runs", () => ({
  useCronRuns: vi.fn(() => ({
    runs,
    loading,
    error,
    loadingMore: false,
    hasMore,
    cancelingId: null,
    cancel,
    loadMore,
  })),
}));

const ME = "me@example.com";
vi.mock("@/common/hooks/use-root-context", () => ({
  useRootContext: () => ({ session: { identity: { traits: { email: ME } } } }),
}));

// The service the layout holds: its build picks the empty state, and its
// Trigger Run fills them.
let service: Partial<ServiceView> | undefined;
vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => ({ service }),
}));
const trigger = vi.fn();
vi.mock("@/features/services/hooks/use-trigger-cron-run", () => ({
  useTriggerCronRun: () => ({ triggering: false, trigger }),
}));

// The service's action projection (w4/203). Default: both verbs allowed.
let preconditions: Record<string, string> = {};
let actionsStatus: "ready" | "checking" | "unavailable" = "ready";
const refreshActions = vi.fn(async () => {});
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
}));
// A run's "by" shows the member's avatar (w1/m648); none uploaded here.
const memberAvatars = vi.hoisted(() => ({
  byEmail: {} as Record<string, string>,
}));
vi.mock("@/features/avatars/hooks/use-member-avatar", () => ({
  useMemberAvatarURL: (email: string | null) =>
    (email && memberAvatars.byEmail[email]) || null,
}));
vi.mock("@/features/capabilities/hooks/use-resource-actions", () => ({
  useServerActions: (serviceId: string) => ({
    status: actionsStatus,
    refresh: refreshActions,
    snapshot: toResourceSnapshot(
      "tea-1",
      serviceId,
      ["cron_run_now", "cron_cancel_run"].map((action) => ({
        action,
        outcome: "allowed",
        reason: null,
        precondition: preconditions[action] ?? "",
      })),
    ),
  }),
}));

beforeEach(() => {
  preconditions = {};
  actionsStatus = "ready";
  refreshActions.mockReset();
  refreshActions.mockResolvedValue(undefined);
  runs = [];
  hasMore = false;
  loading = false;
  error = false;
  service = {
    id: "nightly",
    repo: "https://github.com/bex-co/hello-go",
    revision: "3c2b1a0",
    suspended: false,
  };
  cancel.mockReset();
  cancel.mockResolvedValue(true);
  loadMore.mockReset();
  loadMore.mockResolvedValue(undefined);
  trigger.mockReset();
  vi.mocked(useCronRuns).mockClear();
});

const HOUR = 3_600_000;

/** A finished run that started `hours` ago and took `seconds`. */
function finished(
  id: string,
  status: string,
  hours: number,
  seconds: number,
  patch: Partial<CronRunView> = {},
): CronRunView {
  const start = Date.now() - hours * HOUR;
  const iso = (ms: number) =>
    new Date(Math.floor(ms / 1000) * 1000).toISOString();
  return {
    id,
    status,
    startedAt: iso(start),
    finishedAt: iso(start + seconds * 1000),
    ...patch,
  };
}

/** The rows, by the run each one is. */
const rows = () => screen.queryAllByRole("listitem");
const rowLinks = () =>
  rows().map((row) => within(row).getByRole("link", { name: /Cron job run/ }));
const search = () =>
  screen.getByRole("searchbox", { name: "Search cron job runs" });

// w9/m621: Render shows a cron job's runs as service events, which its
// Billing role does not view; bex-api refuses the run history to it
// (can_view_activity). A Viewer reads the runs as an Admin does.
describe("CronRunsSection per role (w9/m621)", () => {
  beforeEach(() => {
    runs = [finished("crr-1", "successful", 21, 14)];
  });

  it("a Billing member sees why, and the runs are never read", () => {
    vi.mocked(useCapabilities).mockReturnValue(billingCapabilities());
    render(<CronRunsSection serviceId="nightly" />);

    expect(screen.getByText(ACTIVITY_REASON)).toBeInTheDocument();
    expect(rows()).toHaveLength(0);
    expect(
      screen.queryByRole("searchbox", { name: "Search cron job runs" }),
    ).not.toBeInTheDocument();
    expect(useCronRuns).not.toHaveBeenCalled();
  });

  it("a Viewer reads the runs", () => {
    vi.mocked(useCapabilities).mockReturnValue(viewerCapabilities());
    render(<CronRunsSection serviceId="nightly" />);

    expect(rows()).toHaveLength(1);
    expect(screen.queryByText(ACTIVITY_REASON)).not.toBeInTheDocument();
    expect(useCronRuns).toHaveBeenCalledWith("nightly");
  });
});

describe("CronRunsSection rows (w3/m556)", () => {
  it("lists runs as Render's list: RUN with the count, TRIGGER, DURATION, then each run", () => {
    runs = [
      finished("crr-2", "successful", 21, 14),
      finished("crr-1", "unsuccessful", 45, 3, {
        triggeredBy: "ops@example.com",
      }),
    ];
    render(<CronRunsSection serviceId="nightly" />);

    expect(screen.getByText("Run")).toBeInTheDocument();
    expect(screen.getByLabelText("2 runs")).toHaveTextContent("2");
    expect(screen.getByText("Trigger")).toBeInTheDocument();
    expect(screen.getByText("Duration")).toBeInTheDocument();

    const [first, second] = rows();
    expect(within(first).getByText("Cron job run")).toBeInTheDocument();
    expect(within(first).getByText("Run 21 hours ago")).toBeInTheDocument();
    // The fold line and the wide column both carry trigger and duration;
    // which one shows is the list's width.
    expect(within(first).getAllByText("Scheduled")).toHaveLength(2);
    expect(within(first).getAllByText(/14s$/)).toHaveLength(2);
    expect(within(first).queryByText(/^by /)).toBeNull();

    expect(within(second).getByText("Run 2 days ago")).toBeInTheDocument();
    expect(within(second).getByText("by ops@example.com")).toBeInTheDocument();
    expect(within(second).getAllByText("Manual")).toHaveLength(2);
    expect(within(second).getAllByText(/3s$/)).toHaveLength(2);
  });

  it("opens the Logs over each run's window, a minute either side, from the row and its icon", () => {
    runs = [
      {
        id: "crr-1",
        status: "successful",
        startedAt: "2026-11-21T09:00:00Z",
        finishedAt: "2026-11-21T09:00:14Z",
      },
    ];
    const { container } = render(<CronRunsSection serviceId="nightly" />);

    const want =
      "/services/nightly/logs?range=custom&rangeStart=2026-11-21T08%3A59%3A00.000Z&rangeEnd=2026-11-21T09%3A01%3A14.000Z";
    expect(rowLinks()[0]).toHaveAttribute("href", want);
    // The icon opens the same window; the row's link names it for the
    // keyboard, so the icon is left out of the Tab order.
    const icon = container.querySelector('li a[aria-hidden="true"]');
    expect(icon).toHaveAttribute("href", want);
    expect(icon).toHaveAttribute("tabindex", "-1");
  });

  it("names each status in Render's words, the newest success as Last successful run", async () => {
    runs = [
      finished("crr-4", "pending", 0.01, 0, { finishedAt: null }),
      finished("crr-3", "unsuccessful", 21, 3),
      finished("crr-2", "successful", 45, 9),
      finished("crr-1", "successful", 69, 9),
      finished("crr-0", "canceled", 93, 9),
    ];
    const { container } = render(<CronRunsSection serviceId="nightly" />);

    expect(rowLinks().map((link) => link.textContent)).toEqual([
      expect.stringContaining("In progress"),
      expect.stringContaining("Run failed"),
      expect.stringContaining("Last successful run"),
      expect.stringContaining("Run succeeded"),
      expect.stringContaining("Canceled"),
    ]);

    // Hovering the icon shows the same words, as Render's tooltip does.
    const user = userEvent.setup();
    const icons = container.querySelectorAll('li a[aria-hidden="true"]');
    await user.hover(icons[2]);
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      "Last successful run",
    );
  });

  it("counts a run in progress up to now, and its window ends a minute after now", () => {
    const started = Date.now() - 65_000;
    runs = [
      {
        id: "crr-1",
        status: "pending",
        startedAt: new Date(started).toISOString(),
        finishedAt: null,
        triggeredBy: ME,
      },
    ];
    render(<CronRunsSection serviceId="nightly" />);

    const [row] = rows();
    expect(within(row).getAllByText(/^1m [56]s$/)).toHaveLength(2);
    expect(within(row).getByText("by you")).toBeInTheDocument();
    const href = new URL(rowLinks()[0].getAttribute("href")!, "http://x");
    expect(Date.parse(href.searchParams.get("rangeStart")!)).toBe(
      started - 60_000,
    );
    const end = Date.parse(href.searchParams.get("rangeEnd")!);
    expect(Math.abs(end - (Date.now() + 60_000))).toBeLessThan(5_000);
  });

  it("shows a dash for a run that has not started, and no age", () => {
    runs = [
      { id: "crr-1", status: "pending", startedAt: null, finishedAt: null },
    ];
    render(<CronRunsSection serviceId="nightly" />);
    const [row] = rows();
    expect(within(row).getAllByText(/—$/)).toHaveLength(2);
    expect(within(row).queryByText(/^Run /)).toBeNull();
  });

  it("cancels a run in progress at once from its row, with no confirm", async () => {
    runs = [
      finished("crr-2", "pending", 0.01, 0, { finishedAt: null }),
      finished("crr-1", "successful", 21, 14),
    ];
    const user = userEvent.setup();
    render(<CronRunsSection serviceId="nightly" />);

    // Only the run in progress has Cancel.
    expect(screen.getAllByRole("button", { name: "Cancel" })).toHaveLength(1);
    expect(within(rows()[1]).queryByRole("button")).toBeNull();

    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(cancel).toHaveBeenCalledWith("crr-2");
    expect(screen.queryByRole("alertdialog")).toBeNull();
  });

  it("disables a run's Cancel when the server reports no active run", () => {
    preconditions = { cron_cancel_run: "no_active_run" };
    runs = [finished("crr-1", "pending", 0.01, 0, { finishedAt: null })];
    render(<CronRunsSection serviceId="nightly" />);
    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
  });

  it("says who canceled a run, the viewer as you, and Canceled when no one is named", () => {
    runs = [
      finished("crr-3", "canceled", 21, 5, { canceledBy: ME }),
      finished("crr-2", "canceled", 45, 5, {
        triggeredBy: ME,
        canceledBy: "ops@example.com",
      }),
      finished("crr-1", "canceled", 69, 5),
    ];
    render(<CronRunsSection serviceId="nightly" />);
    const [mine, theirs, nobody] = rows();
    expect(within(mine).getByText("Canceled by you")).toBeInTheDocument();
    expect(within(theirs).getByText("by you")).toBeInTheDocument();
    expect(
      within(theirs).getByText("Canceled by ops@example.com"),
    ).toBeInTheDocument();
    expect(within(nobody).getByText("Canceled")).toBeInTheDocument();
  });

  // w1/m648: who ran it, with their avatar: the member's upload, else the
  // monogram of their email.
  it("shows the avatar of who ran it", () => {
    memberAvatars.byEmail = {
      "ops@example.com": "https://api.bex.test/v1/avatars/k1",
    };
    runs = [
      finished("crr-2", "successful", 45, 5, {
        triggeredBy: "ops@example.com",
      }),
      finished("crr-1", "successful", 69, 5, {
        triggeredBy: "dev@example.com",
      }),
    ];
    render(<CronRunsSection serviceId="nightly" />);
    const [ops, dev] = rows();
    expect(
      within(ops).getByRole("img", {
        name: "User-uploaded Avatar for ops@example.com",
      }),
    ).toHaveAttribute("src", "https://api.bex.test/v1/avatars/k1");
    expect(within(dev).queryByRole("img")).toBeNull();
    expect(dev.querySelector('[data-slot="monogram"]')).toHaveTextContent("D");
    memberAvatars.byEmail = {};
  });

  it("has no Trigger Run of its own while it lists runs (the header has it, w3/m555)", () => {
    runs = [finished("crr-1", "pending", 0.01, 0, { finishedAt: null })];
    render(<CronRunsSection serviceId="nightly" />);
    expect(screen.queryByRole("button", { name: "Trigger Run" })).toBeNull();
  });
});

describe("CronRunsSection search (w3/m556)", () => {
  beforeEach(() => {
    runs = [
      finished("crr-d9aaaaaaaaaaaaaaaaa4", "unsuccessful", 21, 3),
      finished("crr-d9aaaaaaaaaaaaaaaaa3", "successful", 45, 9, {
        triggeredBy: "ops@example.com",
      }),
      finished("crr-d9aaaaaaaaaaaaaaaaa2", "successful", 69, 9),
      finished("crr-d9aaaaaaaaaaaaaaaaa1", "canceled", 93, 9),
    ];
  });

  it("narrows the loaded runs to the words typed, and the count follows", async () => {
    const user = userEvent.setup();
    render(<CronRunsSection serviceId="nightly" />);
    expect(search()).toHaveAttribute("placeholder", "Search cron job runs");

    await user.type(search(), "failed");
    expect(rows()).toHaveLength(1);
    expect(rowLinks()[0]).toHaveTextContent("Run failed");
    expect(screen.getByLabelText("1 run")).toHaveTextContent("1");

    await user.clear(search());
    await user.type(search(), "manual");
    expect(rows()).toHaveLength(1);
    expect(within(rows()[0]).getByText("by ops@example.com")).toBeVisible();

    await user.clear(search());
    await user.type(search(), "last successful");
    expect(rowLinks().map((link) => link.textContent)).toEqual([
      expect.stringContaining("Last successful run"),
    ]);

    await user.clear(search());
    await user.type(search(), "ops@example");
    expect(rows()).toHaveLength(1);

    await user.clear(search());
    await user.type(search(), "successful");
    expect(rows()).toHaveLength(2);
  });

  it("finds a run by its whole id", async () => {
    const user = userEvent.setup();
    render(<CronRunsSection serviceId="nightly" />);
    await user.type(search(), "crr-d9aaaaaaaaaaaaaaaaa2");
    expect(rows()).toHaveLength(1);
    expect(rowLinks()[0]).toHaveTextContent("Run succeeded");
  });

  it("says No matching results, and Reset search brings every run back", async () => {
    const user = userEvent.setup();
    render(<CronRunsSection serviceId="nightly" />);
    await user.type(search(), "nothing like this");

    expect(rows()).toHaveLength(0);
    expect(screen.getByText("No matching results")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Not finding what you're looking for? Try adjusting your search and checking for typos.",
      ),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Reset search" }));
    expect(rows()).toHaveLength(4);
    expect(search()).toHaveValue("");
  });

  it("clears the search from its own button", async () => {
    const user = userEvent.setup();
    render(<CronRunsSection serviceId="nightly" />);
    expect(screen.queryByRole("button", { name: "Clear search" })).toBeNull();
    await user.type(search(), "failed");
    await user.click(screen.getByRole("button", { name: "Clear search" }));
    expect(search()).toHaveValue("");
    expect(rows()).toHaveLength(4);
  });

  it("keeps Load more while a search matches nothing loaded", async () => {
    hasMore = true;
    const user = userEvent.setup();
    render(<CronRunsSection serviceId="nightly" />);
    await user.type(search(), "nothing like this");
    await user.click(screen.getByRole("button", { name: "Load more" }));
    expect(loadMore).toHaveBeenCalledOnce();
  });
});

describe("CronRunsSection empty, error and loading states (w3/m556)", () => {
  it("a cron job whose build succeeded: hasn't run in the last 90 days, with Trigger Run", async () => {
    const user = userEvent.setup();
    render(<CronRunsSection serviceId="nightly" />);
    expect(
      screen.getByText("This cron job hasn't run in the last 90 days."),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "You can trigger a run manually, or wait for the next scheduled run.",
      ),
    ).toBeInTheDocument();
    // The search stays, as on Render.
    expect(search()).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Trigger Run" }));
    expect(trigger).toHaveBeenCalledOnce();
  });

  it("a cron job whose build never succeeded: hasn't run yet, with Trigger Run held for the build", () => {
    service = { ...service, revision: "" };
    render(<CronRunsSection serviceId="nightly" />);
    expect(
      screen.getByText("This cron job hasn't run yet."),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "It will run once its first build finishes, or you can trigger a run manually.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Trigger Run" })).toBeDisabled();
  });

  it("a suspended cron job's Trigger Run is disabled with Render's reason", async () => {
    service = { ...service, suspended: true };
    preconditions = { cron_run_now: "suspended" };
    const user = userEvent.setup();
    render(<CronRunsSection serviceId="nightly" />);
    const button = screen.getByRole("button", { name: "Trigger Run" });
    expect(button).toBeDisabled();
    await user.hover(button.parentElement!);
    expect(
      (await screen.findAllByText("Resume this cron job to trigger a run."))
        .length,
    ).toBeGreaterThan(0);
  });

  it("a failed read shows Render's warning, with no search to type into", () => {
    error = true;
    render(<CronRunsSection serviceId="nightly" />);
    expect(screen.getByRole("alert")).toHaveTextContent(
      "We're sorry. We weren't able to load runs for this cron job. Refresh and try again?",
    );
    expect(screen.queryByRole("searchbox")).toBeNull();
  });

  it("keeps the rows it has when a refresh fails", () => {
    error = true;
    runs = [finished("crr-1", "successful", 21, 14)];
    render(<CronRunsSection serviceId="nightly" />);
    expect(rows()).toHaveLength(1);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // The page's loading frame and the route's skeleton draw the ready page's
  // search box, list header, row grid and cells, and the Load more slot, so
  // neither swap moves the page (w3/m556 t005).
  it("frames a full page while it loads, as the ready list and the route skeleton draw it", () => {
    runs = Array.from({ length: CRON_RUNS_PAGE_SIZE }, (_, i) =>
      finished(`crr-${i}`, "successful", i + 1, 9),
    );
    hasMore = true;
    const ready = render(<CronRunsSection serviceId="nightly" />);
    const readyHeader = screen.getByText("Trigger").parentElement!.className;
    const readyRow = ready.container.querySelector("li")!;
    const grid = [...readyRow.classList].filter((name) =>
      name.includes("grid-cols-"),
    );
    expect(grid.length).toBeGreaterThan(1);
    const readyCells = [...readyRow.children]
      .slice(0, 2)
      .map((cell) => cell.className);
    const searchHeight = search()
      .className.split(" ")
      .filter((name) => /^(max-lg:|pointer-coarse:)?h-/.test(name));
    expect(searchHeight).toEqual(["h-9", "max-lg:h-11", "pointer-coarse:h-11"]);
    const readyLoadMore = screen.getByRole("button", { name: "Load more" });
    expect(readyLoadMore.parentElement).toHaveClass("mt-4");
    ready.unmount();

    runs = [];
    hasMore = false;
    loading = true;
    const pending = render(<CronRunsSection serviceId="nightly" />);
    const skeleton = render(<ServiceCronRunsSkeleton />);
    for (const frame of [pending.container, skeleton.container]) {
      expect(
        within(frame as HTMLElement).getByText("Trigger").parentElement!
          .className,
      ).toBe(readyHeader);
      const placeholders = frame.querySelectorAll(
        "[data-testid='cron-run-row-placeholder']",
      );
      expect(placeholders).toHaveLength(CRON_RUNS_PAGE_SIZE);
      for (const placeholder of placeholders) {
        expect(placeholder).toHaveClass(...grid);
        expect(
          [...placeholder.children].slice(0, 2).map((cell) => cell.className),
        ).toEqual(readyCells);
      }
      const rowsFrame = frame.querySelector(
        "[data-testid='cron-runs-rows-frame']",
      )!;
      expect(rowsFrame.lastElementChild).toHaveClass("mt-4");
      expect(rowsFrame.lastElementChild).toHaveTextContent("Load more");
    }
    // The search box keeps its height: the disabled field while the runs
    // load, and a bar of the same height in the route skeleton.
    expect(search()).toBeDisabled();
    const bar = skeleton.container.querySelector(
      "section > div:first-child > [data-slot='skeleton']",
    )!;
    expect(bar).toHaveClass(...searchHeight);
  });
});

// w4/249: the action snapshot is read once and never refreshed by history
// polling, so the arrival of a pending run (from the header's Trigger Run, or
// on a later history poll) must re-read it rather than keep the pre-run
// "no active run" Cancel decision.
describe("CronRunsSection active-run action refresh (w4/249)", () => {
  const done: CronRunView = {
    id: "crr-old",
    startedAt: "2026-10-11T03:40:00Z",
    finishedAt: "2026-10-11T03:41:15Z",
    status: "successful",
  };
  const pending: CronRunView = {
    id: "crr-new",
    startedAt: "2026-10-11T03:47:31Z",
    finishedAt: null,
    status: "pending",
  };

  function deferRefresh(settle: () => void) {
    let resolve!: () => void;
    refreshActions.mockImplementationOnce(() => {
      actionsStatus = "checking";
      return new Promise<void>((r) => {
        resolve = () => {
          settle();
          r();
        };
      });
    });
    return () => act(async () => resolve());
  }

  it("re-reads the action decision when a new run arrives and enables Cancel once it settles", async () => {
    preconditions = { cron_cancel_run: "no_active_run" };
    runs = [done];
    const { rerender } = render(<CronRunsSection serviceId="nightly" />);
    expect(refreshActions).not.toHaveBeenCalled();

    // A later history poll delivers the accepted run.
    const settle = deferRefresh(() => {
      preconditions = {};
      actionsStatus = "ready";
    });
    runs = [pending, done];
    rerender(<CronRunsSection serviceId="nightly" />);

    expect(refreshActions).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();

    await settle();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeEnabled();

    // Further polls of the same active run do not re-read again.
    runs = [{ ...pending, status: "running" }, done];
    rerender(<CronRunsSection serviceId="nightly" />);
    expect(refreshActions).toHaveBeenCalledTimes(1);

    // The run finishing is a transition too: the decision is re-read.
    runs = [{ ...pending, status: "successful" }, done];
    rerender(<CronRunsSection serviceId="nightly" />);
    expect(refreshActions).toHaveBeenCalledTimes(2);
  });

  it("keeps Cancel disabled when the re-read fails", async () => {
    preconditions = { cron_cancel_run: "no_active_run" };
    runs = [done];
    const { rerender } = render(<CronRunsSection serviceId="nightly" />);

    const settle = deferRefresh(() => {
      actionsStatus = "unavailable";
    });
    runs = [pending, done];
    rerender(<CronRunsSection serviceId="nightly" />);
    await settle();

    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
    expect(refreshActions).toHaveBeenCalledTimes(1);
  });
});
