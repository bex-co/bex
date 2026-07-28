import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CronRunsSection } from "@/features/services/components/cron-runs-section";
import type { CronRunView } from "@/features/services/types";

const cancel = vi.fn();
const loadMore = vi.fn();
let runs: CronRunView[] = [];
let hasMore = false;

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
  }),
}));

beforeEach(() => {
  runs = [];
  hasMore = false;
  cancel.mockReset();
  cancel.mockResolvedValue(true);
  loadMore.mockReset();
  loadMore.mockResolvedValue(undefined);
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
        name: "crr-first",
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
});
