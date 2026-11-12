import type { ReactNode } from "react";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { RouterContent, RouterPageSkeleton } from "../page";
import type { RouterOverview } from "../data";
import { requireRouterFeature } from "@/common/lib/growthbook/require-router-feature";
import { hydrateAcrossEnvironments } from "@/test/hydration";

vi.mock("@/common/components/dashboard-layout", () => ({
  DashboardLayout: ({ children }: { children: ReactNode }) => children,
}));
const overview: RouterOverview = {
  __typename: "RouterOverview",
  quota: {
    __typename: "RouterQuota",
    observedAt: new Date().toISOString(),
    windows: ["FIVE_HOUR", "WEEKLY", "MONTHLY"].map((kind, index) => ({
      __typename: "RouterWindow",
      kind,
      utilizationBps: [9, 11, 481][index],
      resetsAt: new Date(Date.now() + 3600000).toISOString(),
    })),
  },
  keys: [
    {
      __typename: "RouterKey",
      id: "test-key",
      name: "Example",
      accessKey: "test-only-key",
      createdAt: "2026-01-01T00:00:00Z",
      options: null,
    },
  ],
};

describe("Router", () => {
  it("guards direct routes for other and absent tenants", () => {
    for (const workspaceId of [undefined, null, "tea-other"])
      expect(() =>
        requireRouterFeature()({ context: { workspaceId } }),
      ).toThrow();
    expect(() =>
      requireRouterFeature()({
        context: { workspaceId: "tea-d98210cbbpdc73dcrkvg" },
      }),
    ).not.toThrow();
  });
  it("renders the three real usage windows and key table", () => {
    render(<RouterContent overview={overview} />);
    expect(screen.getByText("0.09%")).toBeInTheDocument();
    expect(screen.getByText("0.11%")).toBeInTheDocument();
    expect(screen.getByText("4.81%")).toBeInTheDocument();
    expect(screen.getAllByRole("progressbar")).toHaveLength(3);
    expect(screen.getByText("test-only-key")).toBeInTheDocument();
  });
  it("shows no time until hydration, so a browser in another zone agrees with the server (w21/m337)", async () => {
    const probe = await hydrateAcrossEnvironments(() => (
      <RouterContent overview={overview} />
    ));
    // The server's zone is UTC: a formatted key creation time baked into its
    // markup would read 12:00:00 AM and disagree with a viewer in Shanghai.
    expect(probe.html).not.toContain("12:00:00 AM");
    expect(probe.recovered).toHaveLength(0);
  });
  it("reserves the same major regions while pending and disables creation", () => {
    render(<RouterPageSkeleton />);
    expect(
      screen.getByRole("region", { name: "Multi-window usage" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("region", { name: "API Keys" }),
    ).toBeInTheDocument();
    expect(screen.getAllByRole("row")).toHaveLength(5);
    expect(screen.getByRole("button", { name: "Create Key" })).toBeDisabled();
  });
  it("creates through the supplied mutation and closes on success", async () => {
    const create = vi.fn().mockResolvedValue(undefined);
    render(
      <RouterContent
        overview={overview}
        create={create}
        update={vi.fn()}
        remove={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Create Key" }));
    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "New application" },
    });
    fireEvent.submit(screen.getByLabelText("Name").closest("form")!);
    await waitFor(() => expect(create).toHaveBeenCalledWith("New application"));
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
  });
  it("requires confirmation before deleting and leaves failures retryable", async () => {
    const remove = vi.fn().mockRejectedValue(new Error("failure"));
    render(
      <RouterContent
        overview={overview}
        create={vi.fn()}
        update={vi.fn()}
        remove={remove}
      />,
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Key settings: Example" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Delete key" }));
    expect(remove).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Delete key" }));
    await waitFor(() => expect(remove).toHaveBeenCalledWith("test-key"));
    expect(
      await screen.findByText("Could not save changes. Please try again."),
    ).toBeInTheDocument();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });
  it("shows an error and retry without presenting missing usage as zero", () => {
    const refetch = vi.fn();
    render(<RouterContent error={new Error("offline")} refetch={refetch} />);
    expect(screen.queryByText("0%")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(refetch).toHaveBeenCalled();
  });
});
