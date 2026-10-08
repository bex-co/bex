import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
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
vi.mock("@/features/workspaces/context/hooks", () => ({
  useWorkspace: () => ({ currentWorkspaceId: "tea-1" }),
}));
vi.mock("@/features/capabilities/hooks/use-resource-actions", () => ({
  useServerActions: (serviceId: string) => ({
    status: "ready",
    refresh: vi.fn(),
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

  it("keeps the plain confirm copy when no run is active", async () => {
    hasActiveRun = false;
    const user = userEvent.setup();
    render(<CronRunsSection serviceId="nightly" />);

    await user.click(screen.getByRole("button", { name: "Trigger Run" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent(
      "This runs the job's command immediately, outside its schedule.",
    );
    expect(dialog.textContent ?? "").not.toContain("cancels it");
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

    await user.click(screen.getByRole("button", { name: "Toggle run detail" }));
    // The detail exposes the run id + Finished label, which the row never shows.
    expect(screen.getByText("crr-success")).toBeInTheDocument();
    expect(screen.getByText("Finished")).toBeInTheDocument();
  });

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

    await user.click(screen.getByRole("button", { name: "Toggle run detail" }));
    expect(
      screen.getByText("Couldn't load this run's detail."),
    ).toBeInTheDocument();
  });
});
