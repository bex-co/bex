import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CronRunsSection } from "@/features/services/components/cron-runs-section";
import type { CronRunView } from "@/features/services/types";

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

  it("disables Trigger Run while a run is active (ForbidConcurrent)", () => {
    hasActiveRun = true;
    render(<CronRunsSection serviceId="nightly" />);
    expect(screen.getByRole("button", { name: "Trigger Run" })).toBeDisabled();
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
