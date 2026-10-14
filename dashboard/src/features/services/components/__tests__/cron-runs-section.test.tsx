import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CronRunsSection } from "@/features/services/components/cron-runs-section";
import type { CronRunView } from "@/features/services/types";
import { toResourceSnapshot } from "@/features/capabilities/lib/resource-actions";

const cancel = vi.fn();
const loadMore = vi.fn();
const trigger = vi.fn();
const clearTriggerError = vi.fn();
let runs: CronRunView[] = [];
let hasMore = false;
let hasActiveRun = false;
let triggering = false;
let triggerError: string | null = null;

vi.mock("@/features/services/hooks/use-cron-runs", () => ({
  useCronRuns: () => ({
    runs,
    loading: false,
    error: false,
    loadingMore: false,
    hasMore,
    cancelingId: null,
    cancel,
    loadMore,
    hasActiveRun,
    triggering,
    triggerError,
    clearTriggerError,
    trigger,
  }),
}));

// The service's action projection (w4/203). Default: both verbs allowed.
let preconditions: Record<string, string> = {};
let actionsStatus: "ready" | "checking" | "unavailable" = "ready";
const refreshActions = vi.fn(async () => {});
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
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

// The per-run detail (cronJobRun) read, exercised when a history row expands.
let detailRun: CronRunView | null = null;
let detailLoading = false;
let detailError = false;
vi.mock("@/features/services/hooks/use-cron-run", () => ({
  useCronRun: () => ({
    run: detailRun,
    loading: detailLoading,
    error: detailError,
  }),
}));

beforeEach(() => {
  preconditions = {};
  actionsStatus = "ready";
  refreshActions.mockReset();
  refreshActions.mockResolvedValue(undefined);
  runs = [];
  hasMore = false;
  hasActiveRun = false;
  triggering = false;
  triggerError = null;
  detailRun = null;
  detailLoading = false;
  detailError = false;
  cancel.mockReset();
  cancel.mockResolvedValue(true);
  loadMore.mockReset();
  loadMore.mockResolvedValue(undefined);
  trigger.mockReset();
  trigger.mockResolvedValue(true);
  clearTriggerError.mockReset();
});

describe("CronRunsSection", () => {
  it("shows an empty state when the cron has no runs", () => {
    render(<CronRunsSection serviceId="nightly" />);
    expect(screen.getByText("No runs yet.")).toBeInTheDocument();
  });

  it("shows Render statuses, started time, duration, and cancel only for pending", () => {
    runs = [
      {
        id: "crr-running",
        startedAt: "2026-07-09T10:00:00Z",
        finishedAt: null,
        status: "pending",
      },
      {
        id: "crr-success",
        startedAt: "2026-07-09T10:05:00Z",
        finishedAt: "2026-07-09T10:05:05Z",
        status: "successful",
      },
      {
        id: "crr-canceled",
        startedAt: "2026-07-09T10:10:00Z",
        finishedAt: "2026-07-09T10:10:01Z",
        status: "canceled",
      },
    ];
    render(<CronRunsSection serviceId="nightly" />);

    expect(screen.getByText("Running")).toBeInTheDocument();
    expect(screen.getByText("Succeeded")).toBeInTheDocument();
    expect(screen.getByText("Canceled")).toBeInTheDocument();
    expect(screen.getByText("5s")).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Cancel" })).toHaveLength(1);
  });

  it("confirms and cancels the selected pending run", async () => {
    runs = [
      {
        id: "crr-running",
        startedAt: "2026-07-09T10:00:00Z",
        finishedAt: null,
        status: "pending",
      },
    ];
    const user = userEvent.setup();
    render(<CronRunsSection serviceId="nightly" />);

    await user.click(screen.getByRole("button", { name: "Cancel" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(within(dialog).getByText("Cancel this run?")).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Proceed" }));

    expect(cancel).toHaveBeenCalledWith("crr-running");
  });

  it("loads the next cursor page", async () => {
    runs = [
      {
        id: "crr-first",
        startedAt: "2026-07-09T10:00:00Z",
        finishedAt: "2026-07-09T10:00:01Z",
        status: "successful",
      },
    ];
    hasMore = true;
    const user = userEvent.setup();
    render(<CronRunsSection serviceId="nightly" />);

    await user.click(screen.getByRole("button", { name: "Load more" }));
    expect(loadMore).toHaveBeenCalledOnce();
  });

  it("triggers a run after confirming (w5/m60)", async () => {
    const user = userEvent.setup();
    render(<CronRunsSection serviceId="nightly" />);

    await user.click(screen.getByRole("button", { name: "Trigger Run" }));
    // Confirm dialog → the second "Trigger Run" is the dialog's action.
    const actions = screen.getAllByRole("button", { name: "Trigger Run" });
    await user.click(actions[actions.length - 1]);

    expect(trigger).toHaveBeenCalledOnce();
  });

  // w4/m114 t003: the button used to be disabled here with "A run is already
  // in progress" while the server happily accepted runCronJob and preempted
  // the active run — one surface forbidding what the other performs, and the
  // only in-product way to recover a wedged run. It now offers the action and
  // names the consequence.
  it("offers Trigger Run during an active run and warns that it preempts", async () => {
    hasActiveRun = true;
    const user = userEvent.setup();
    render(<CronRunsSection serviceId="nightly" />);

    const button = screen.getByRole("button", { name: "Trigger Run" });
    expect(button).not.toBeDisabled();
    await user.click(button);

    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent(
      "A run is already in progress. Triggering now cancels it and starts a new run immediately, outside the schedule.",
    );
  });

  // w4/237: history can lag an accepted run (the refetch after a replacement
  // may hold only terminal rows), so "no active run loaded" is not proof that
  // triggering replaces nothing. The default copy states the replacement
  // semantics conditionally, without claiming a run exists.
  it("still explains replacement when no active run is loaded", async () => {
    hasActiveRun = false;
    const user = userEvent.setup();
    render(<CronRunsSection serviceId="nightly" />);

    await user.click(screen.getByRole("button", { name: "Trigger Run" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent(
      "This requests a run outside its schedule. It replaces any pending manual run request and cancels any active run before the replacement starts.",
    );
    expect(dialog.textContent ?? "").not.toContain("already in progress");

    await user.click(within(dialog).getByRole("button", { name: "Go Back" }));
    expect(trigger).not.toHaveBeenCalled();
  });

  it("disables Trigger Run on a suspended cron, with the reason (w4/203)", async () => {
    preconditions = { cron_run_now: "suspended" };
    render(<CronRunsSection serviceId="nightly" />);
    const button = screen.getByRole("button", { name: "Trigger Run" });
    expect(button).toBeDisabled();
    await userEvent.hover(button.parentElement ?? button);
    expect(
      (await screen.findAllByText(/Resume it first/)).length,
    ).toBeGreaterThan(0);
    expect(trigger).not.toHaveBeenCalled();
  });

  it("disables a run's Cancel when the server reports no active run", () => {
    preconditions = { cron_cancel_run: "no_active_run" };
    runs = [
      {
        id: "crr-1",
        startedAt: "2026-07-09T10:00:00Z",
        finishedAt: null,
        status: "pending",
      },
    ];
    render(<CronRunsSection serviceId="nightly" />);
    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Trigger Run" })).toBeEnabled();
  });

  it("shows the backend's trigger rejection inline, not a toast", () => {
    triggerError = "a run is already active";
    render(<CronRunsSection serviceId="nightly" />);
    expect(screen.getByText("a run is already active")).toBeInTheDocument();
  });

  it("expands a history row to show the run detail from cronJobRun (w5/m60)", async () => {
    runs = [
      {
        id: "crr-success",
        startedAt: "2026-07-09T10:05:00Z",
        finishedAt: "2026-07-09T10:05:05Z",
        status: "successful",
      },
    ];
    detailRun = {
      id: "crr-success",
      startedAt: "2026-07-09T10:05:00Z",
      finishedAt: "2026-07-09T10:05:05Z",
      status: "successful",
    };
    const user = userEvent.setup();
    render(<CronRunsSection serviceId="nightly" />);

    await user.click(
      screen.getByRole("button", { name: /Run details, started/ }),
    );
    // The detail exposes the run id + Finished label, which the row never shows.
    expect(screen.getByText("crr-success")).toBeInTheDocument();
    expect(screen.getByText("Finished")).toBeInTheDocument();
  });

  // w4/213: every row's toggle was named "Toggle run detail", so keyboard and
  // screen-reader users could not pick a run by name.
  it("names each run's detail toggle by its own start, with the id for ties and missing times", () => {
    const at = (iso: string) => new Date(iso).toLocaleString();
    runs = [
      {
        id: "crr-a",
        startedAt: "2026-07-09T10:00:00Z",
        finishedAt: null,
        status: "succeeded",
      },
      {
        id: "crr-b",
        startedAt: "2026-07-09T11:00:00Z",
        finishedAt: null,
        status: "succeeded",
      },
      {
        id: "crr-tie",
        startedAt: "2026-07-09T11:00:00Z",
        finishedAt: null,
        status: "succeeded",
      },
      {
        id: "crr-pending",
        startedAt: null,
        finishedAt: null,
        status: "pending",
      },
    ];
    render(<CronRunsSection serviceId="nightly" />);
    const names = screen
      .getAllByRole("button", { name: /Run details, started/ })
      .map((button) => button.textContent ?? "");
    expect(names).toHaveLength(4);
    expect(new Set(names).size).toBe(4);
    expect(names[0]).toContain(
      `Run details, started ${at("2026-07-09T10:00:00Z")}`,
    );
    expect(names[1]).toContain(`${at("2026-07-09T11:00:00Z")} (crr-b)`);
    expect(names[2]).toContain(`${at("2026-07-09T11:00:00Z")} (crr-tie)`);
    expect(names[3]).toContain("Run details, started crr-pending");
  });

  // w4/305: the first detail read showed one full-width h-16 block, so the
  // expanded row (and the table's columns) jumped to the five-field grid when
  // the read settled. The pending detail must be that same grid, sized by the
  // history row's own values but masked. jsdom has no layout, so this pins the
  // structural mapping; pixel bounds at 1440/390 px need live verification.
  it.each<[string, CronRunView]>([
    [
      "successful",
      {
        id: "crr-qsndkqj2819t9urkambm",
        startedAt: "2026-10-14T18:19:41Z",
        finishedAt: "2026-10-14T18:19:46Z",
        status: "successful",
      },
    ],
    [
      "running (no finish or duration)",
      {
        id: "crr-running",
        startedAt: "2026-10-14T18:19:41Z",
        finishedAt: null,
        status: "running",
      },
    ],
    [
      "failed",
      {
        id: "crr-failed",
        startedAt: "2026-10-14T18:19:41Z",
        finishedAt: "2026-10-14T18:21:03Z",
        status: "unsuccessful",
      },
    ],
    [
      "canceled",
      {
        id: "crr-canceled",
        startedAt: "2026-10-14T18:19:41Z",
        finishedAt: "2026-10-14T18:19:42Z",
        status: "canceled",
      },
    ],
  ])(
    "previews a %s run's detail with the ready five-field grid, values masked (w4/305)",
    async (_, row) => {
      runs = [row];
      detailLoading = true;
      const user = userEvent.setup();
      const { container, rerender } = render(
        <CronRunsSection serviceId="nightly" />,
      );
      await user.click(
        screen.getByRole("button", { name: /Run details, started/ }),
      );

      // A truthful loading affordance; no resolved value is exposed.
      expect(screen.getByRole("status")).toHaveTextContent(
        "Loading run detail…",
      );
      const pending = container.querySelector("dl");
      expect(pending).not.toBeNull();
      expect(pending).toHaveAttribute("aria-busy", "true");
      const pendingValues = Array.from(pending!.querySelectorAll("dd"));
      for (const dd of pendingValues) {
        const hidden = dd.querySelector(":scope > span.invisible");
        expect(hidden).toHaveAttribute("aria-hidden", "true");
        expect(
          dd.querySelector(':scope > [data-slot="skeleton"]'),
        ).toHaveAttribute("aria-hidden", "true");
      }

      const shape = (dl: HTMLElement) => ({
        className: dl.className,
        labels: Array.from(dl.querySelectorAll("dt")).map(
          (dt) => dt.textContent,
        ),
        fieldClasses: Array.from(dl.children).map((field) => field.className),
      });
      const pendingShape = shape(pending!);
      const pendingContent = pendingValues.map(
        (dd) => dd.querySelector(":scope > span.invisible")!.innerHTML,
      );

      // The same run settles: the ready grid matches the preview field for
      // field, and its values are exactly what sized the masked cells.
      detailLoading = false;
      detailRun = { ...row };
      rerender(<CronRunsSection serviceId="nightly" />);

      expect(screen.queryByRole("status")).toBeNull();
      const ready = container.querySelector("dl")!;
      expect(ready).not.toHaveAttribute("aria-busy");
      expect(shape(ready)).toEqual(pendingShape);
      expect(pendingShape.labels).toEqual([
        "Status",
        "Started",
        "Finished",
        "Duration",
        "Run ID",
      ]);
      expect(
        Array.from(ready.querySelectorAll("dd")).map((dd) => dd.innerHTML),
      ).toEqual(pendingContent);
      expect(within(ready).getByText(row.id)).toBeVisible();
    },
  );

  it("renders an explicit error when a run's detail read fails", async () => {
    runs = [
      {
        id: "crr-stale",
        startedAt: "2026-07-09T10:05:00Z",
        finishedAt: null,
        status: "pending",
      },
    ];
    detailError = true;
    const user = userEvent.setup();
    render(<CronRunsSection serviceId="nightly" />);

    await user.click(
      screen.getByRole("button", { name: /Run details, started/ }),
    );
    expect(
      screen.getByText("Couldn't load this run's detail."),
    ).toBeInTheDocument();
  });
});

// w4/249: the action snapshot is read once; a run that arrives later (after a
// trigger, or on a later history poll) must re-read it rather than keep the
// pre-run "no active run" Cancel decision.
describe("CronRunsSection active-run action refresh (w4/249)", () => {
  const finished: CronRunView = {
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
    runs = [finished];
    const { rerender } = render(<CronRunsSection serviceId="nightly" />);
    expect(refreshActions).not.toHaveBeenCalled();

    // A later history poll delivers the accepted run.
    const settle = deferRefresh(() => {
      preconditions = {};
      actionsStatus = "ready";
    });
    runs = [pending, finished];
    hasActiveRun = true;
    rerender(<CronRunsSection serviceId="nightly" />);

    expect(refreshActions).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();

    await settle();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeEnabled();

    // Further polls of the same active run do not re-read again.
    runs = [{ ...pending, status: "running" }, finished];
    rerender(<CronRunsSection serviceId="nightly" />);
    expect(refreshActions).toHaveBeenCalledTimes(1);

    // Go Back on the confirmation sends nothing.
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    await user.click(screen.getByRole("button", { name: "Go Back" }));
    expect(cancel).not.toHaveBeenCalled();

    // The run finishing is a transition too: the decision is re-read.
    runs = [{ ...pending, status: "successful" }, finished];
    hasActiveRun = false;
    rerender(<CronRunsSection serviceId="nightly" />);
    expect(refreshActions).toHaveBeenCalledTimes(2);
  });

  it("keeps Cancel disabled when the re-read fails", async () => {
    preconditions = { cron_cancel_run: "no_active_run" };
    runs = [finished];
    const { rerender } = render(<CronRunsSection serviceId="nightly" />);

    const settle = deferRefresh(() => {
      actionsStatus = "unavailable";
    });
    runs = [pending, finished];
    rerender(<CronRunsSection serviceId="nightly" />);
    await settle();

    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
    expect(refreshActions).toHaveBeenCalledTimes(1);
  });
});
