import { describe, it, expect, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { AutoscalingSection } from "../autoscaling-section";
import type { UseAutoscalingResult } from "@/features/services/hooks/use-autoscaling";

function storedFreeConfig(
  overrides: Partial<UseAutoscalingResult> = {},
): UseAutoscalingResult {
  return {
    enabled: true,
    loading: false,
    saving: false,
    minInstances: 1,
    maxInstances: 1,
    targetCPUPercent: 70,
    targetMemoryPercent: null,
    save: vi.fn(async () => true),
    disable: vi.fn(async () => true),
    ...overrides,
  };
}

describe("AutoscalingSection on the free plan (w4/365)", () => {
  it("shows a stored config as saved-but-inactive with a working turn-off", async () => {
    const as = storedFreeConfig();
    render(<AutoscalingSection autoscaling={as} plan="free" />);

    // Stored and visible — not a plain "Autoscaling off" hiding settings
    // nobody can remove.
    expect(screen.getByText("Autoscaling on")).toBeInTheDocument();
    expect(
      screen.getByText(/saved but inactive on the Free plan/),
    ).toBeInTheDocument();
    expect(screen.getByText(/Number of Instances: 1–1/)).toBeInTheDocument();

    // Turning it off is reachable and runs the existing disable flow.
    fireEvent.click(screen.getByRole("switch"));
    fireEvent.click(await screen.findByRole("button", { name: "Disable" }));
    expect(as.disable).toHaveBeenCalledOnce();
  });

  it("without a stored config keeps the paid-only gate with the switch off and disabled", () => {
    const as = storedFreeConfig({ enabled: false });
    render(<AutoscalingSection autoscaling={as} plan="free" />);

    expect(screen.getByText("Autoscaling off")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Autoscaling is available on paid plans. Upgrade to enable it.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("switch")).toBeDisabled();
  });
});
