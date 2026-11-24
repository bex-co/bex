import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { BuildPipelineCard } from "@/features/usage/components/build-pipeline-card";
import { usePipelineSettings } from "@/features/usage/hooks/use-pipeline-settings";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";

vi.mock("@/features/usage/hooks/use-pipeline-settings", () => ({
  usePipelineSettings: vi.fn(),
}));

const mocked = vi.mocked(usePipelineSettings);

function settings(spendLimitUsd: string | null) {
  const save = vi.fn().mockResolvedValue(true);
  mocked.mockReturnValue({
    spendLimitUsd,
    loading: false,
    error: undefined,
    saving: false,
    save,
  });
  return save;
}

// w6/m309: Render's Workspace Settings → Build Pipeline → Set spend limit.
describe("BuildPipelineCard", () => {
  beforeEach(() => mocked.mockReset());

  it("says there is no limit and sets one in dollars", async () => {
    const save = settings(null);
    render(<BuildPipelineCard workspaceId="tea-a" />);
    expect(screen.getByText("No spend limit")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Set spend limit" }));
    const input = screen.getByLabelText("Monthly limit (USD)");
    fireEvent.change(input, { target: { value: "25.505" } });
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    fireEvent.change(input, { target: { value: "25.50" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(save).toHaveBeenCalledWith("25.50");
  });

  it("shows the limit and removes it", () => {
    const save = settings("40.00");
    render(<BuildPipelineCard workspaceId="tea-a" />);
    expect(screen.getByText("Spend limit: $40.00 a month")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Remove limit" }));
    expect(save).toHaveBeenCalledWith(null);
  });
});

// w9/m620: a Developer views billing details but does not edit them (Render:
// "Edit payment method" is Admin and Billing only), so the limit reads and
// Set, Edit and Remove are held, saying why.
describe("BuildPipelineCard for a member who views but does not manage billing", () => {
  beforeEach(() => {
    mocked.mockReset();
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "DEVELOPER", canManageBilling: false }),
    );
  });
  afterEach(() => {
    vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  });

  it("shows the limit with every change held", () => {
    const save = settings("40.00");
    render(<BuildPipelineCard workspaceId="tea-a" />);
    expect(screen.getByText("Spend limit: $40.00 a month")).toBeInTheDocument();
    for (const name of ["Remove limit", "Edit"]) {
      const button = screen.getByRole("button", { name });
      expect(button).toBeDisabled();
      fireEvent.click(button);
    }
    expect(save).not.toHaveBeenCalled();
    expect(
      screen.queryByLabelText("Monthly limit (USD)"),
    ).not.toBeInTheDocument();
  });

  it("holds Set when there is no limit", () => {
    settings(null);
    render(<BuildPipelineCard workspaceId="tea-a" />);
    expect(screen.getByText("No spend limit")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Set spend limit" }),
    ).toBeDisabled();
  });
});

describe("BuildPipelineCard read failures", () => {
  it("does not pass a failed read off as no limit", () => {
    mocked.mockReturnValue({
      spendLimitUsd: null,
      loading: false,
      error: new Error("network down"),
      saving: false,
      save: vi.fn(),
    });
    render(<BuildPipelineCard workspaceId="tea-a" />);
    expect(screen.queryByText("No spend limit")).not.toBeInTheDocument();
    expect(
      screen.getByText(/Couldn't load the spend limit/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Set spend limit" }),
    ).toBeDisabled();
  });
});

// The pending card renders the same static header copy as the ready one, so
// both wrap alike at every width and the cards below it do not jump when it
// loads (w9/097).
describe("BuildPipelineCard skeleton", () => {
  it("keeps the ready card's header text and its row of limit and action", () => {
    mocked.mockReturnValue({
      spendLimitUsd: null,
      loading: true,
      error: undefined,
      saving: false,
      save: vi.fn(),
    });
    const { unmount } = render(<BuildPipelineCard workspaceId="tea-a" />);
    const skeleton = screen.getByTestId("build-pipeline-skeleton");
    const pendingHeader = skeleton.querySelector(
      '[data-slot="card-header"]',
    )?.textContent;
    unmount();

    settings(null);
    const { container } = render(<BuildPipelineCard workspaceId="tea-a" />);
    const readyHeader = container.querySelector(
      '[data-slot="card-header"]',
    )?.textContent;
    expect(pendingHeader).toBeTruthy();
    expect(pendingHeader).toBe(readyHeader);
    expect(
      skeleton.querySelector('[data-slot="card-content"]')?.className,
    ).toBe(container.querySelector('[data-slot="card-content"]')?.className);
  });
});
