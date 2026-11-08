import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RunHistoryCard } from "@/features/agent-automations/components/run-history-card";
import type {
  AutomationRunRow,
  UseAutomationRunsResult,
} from "@/features/agent-automations/hooks/use-agent-automation-runs";

vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({
      children,
      params,
    }: {
      children: ReactNode;
      params?: { agentSessionId?: string };
    }) => <a href={`/agents/${params?.agentSessionId}`}>{children}</a>,
  };
});

function run(over: Partial<AutomationRunRow>): AutomationRunRow {
  return {
    __typename: "AgentAutomationRun",
    automationId: "aut-1",
    fireAt: "2026-11-07T09:00:00Z",
    outcome: "started",
    sessionId: null,
    missedTicks: 0,
    error: null,
    createdAt: "2026-11-07T09:00:00Z",
    settledAt: null,
    ...over,
  } as AutomationRunRow;
}

function runs(
  rows: AutomationRunRow[],
  over: Partial<UseAutomationRunsResult> = {},
): UseAutomationRunsResult {
  return {
    runs: rows,
    loading: false,
    error: undefined,
    refetch: vi.fn(),
    loadMore: vi.fn(async () => undefined),
    loadingMore: false,
    hasMore: false,
    ...over,
  };
}

describe("RunHistoryCard", () => {
  it("renders each outcome with its badge, session link, missed ticks and error", () => {
    render(
      <RunHistoryCard
        runs={runs([
          run({
            fireAt: "2026-11-07T09:05:00Z",
            outcome: "started",
            sessionId: "ags-1",
            missedTicks: 3,
          }),
          run({ fireAt: "2026-11-07T09:04:00Z", outcome: "claimed" }),
          run({ fireAt: "2026-11-07T09:03:00Z", outcome: "skipped_overlap" }),
          run({
            fireAt: "2026-11-07T09:02:00Z",
            outcome: "failed_create",
            error: "model key missing",
          }),
          run({
            fireAt: "2026-11-07T09:01:00Z",
            outcome: "denied",
            error: "creator lost access",
          }),
          run({ fireAt: "2026-11-07T09:00:00Z", outcome: "reticulated" }),
        ])}
      />,
    );
    const rows = screen.getAllByRole("row").slice(1);
    expect(rows.map((r) => r.getAttribute("data-outcome"))).toEqual([
      "started",
      "claimed",
      "skipped_overlap",
      "failed_create",
      "denied",
      "reticulated",
    ]);
    const badge = (i: number) =>
      within(rows[i]).getByText(/./, { selector: "[data-outcome]" });
    expect(badge(0)).toHaveTextContent("Started");
    expect(badge(1)).toHaveTextContent("In progress");
    expect(badge(2)).toHaveTextContent("Skipped");
    expect(badge(3)).toHaveTextContent("Failed");
    expect(badge(4)).toHaveTextContent("Denied");
    // An outcome this build doesn't know shows the server's word, not a blank.
    expect(badge(5)).toHaveTextContent("reticulated");

    expect(
      within(rows[0]).getByRole("link", { name: "Open session" }),
    ).toHaveAttribute("href", "/agents/ags-1");
    expect(within(rows[0]).getByText("3")).toBeInTheDocument();
    expect(within(rows[3]).getByText("model key missing")).toBeInTheDocument();
    expect(
      within(rows[4]).getByText("creator lost access"),
    ).toBeInTheDocument();
    expect(within(rows[1]).queryByRole("link")).not.toBeInTheDocument();
  });

  it("pages with a cursor button only while more runs exist", async () => {
    const user = userEvent.setup();
    const loadMore = vi.fn(async () => undefined);
    const { rerender } = render(
      <RunHistoryCard runs={runs([run({})], { hasMore: true, loadMore })} />,
    );
    await user.click(screen.getByRole("button", { name: "Load more" }));
    expect(loadMore).toHaveBeenCalledTimes(1);
    rerender(<RunHistoryCard runs={runs([run({})], { hasMore: false })} />);
    expect(
      screen.queryByRole("button", { name: "Load more" }),
    ).not.toBeInTheDocument();
  });

  it("says so when there are no runs yet", () => {
    render(<RunHistoryCard runs={runs([])} />);
    expect(screen.getByText("No runs yet.")).toBeInTheDocument();
  });

  it("keeps the real header and shows bars while pending", () => {
    const { container } = render(<RunHistoryCard runs={null} />);
    expect(screen.getByText("Run history")).toBeInTheDocument();
    expect(
      screen.getAllByRole("columnheader").map((h) => h.textContent),
    ).toEqual(["Fire time", "Outcome", "Session", "Missed", "Error"]);
    expect(
      container.querySelectorAll('[data-skeleton-region="run-row"]'),
    ).toHaveLength(3);
  });

  it("offers Retry when the history fails to load", async () => {
    const user = userEvent.setup();
    const refetch = vi.fn();
    render(
      <RunHistoryCard runs={runs([], { error: new Error("boom"), refetch })} />,
    );
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(refetch).toHaveBeenCalled();
  });
});
