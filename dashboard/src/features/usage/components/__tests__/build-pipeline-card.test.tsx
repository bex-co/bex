import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { BuildPipelineCard } from "@/features/usage/components/build-pipeline-card";
import { usePipelineSettings } from "@/features/usage/hooks/use-pipeline-settings";

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
