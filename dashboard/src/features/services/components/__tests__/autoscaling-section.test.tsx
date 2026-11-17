import type { ReactNode } from "react";
import { describe, it, expect, vi } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { AutoscalingSection } from "../autoscaling-section";
import type { UseAutoscalingResult } from "@/features/services/hooks/use-autoscaling";

// The plan notice links to Billing; render the router Link as its anchor.
vi.mock("@tanstack/react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  Link: ({
    to,
    children,
    className,
  }: {
    to: string;
    children: ReactNode;
    className?: string;
  }) => (
    <a href={to} className={className}>
      {children}
    </a>
  ),
}));

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
    metricsUnavailable: false,
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

// w4/597: a disable keeps the settings turned off, so turning autoscaling back
// on starts from them and saves them unchanged.
describe("AutoscalingSection after a disable (w4/597)", () => {
  it("turns back on with the kept range and targets", async () => {
    const as = storedFreeConfig({
      enabled: false,
      minInstances: 2,
      maxInstances: 4,
      targetCPUPercent: 70,
    });
    render(<AutoscalingSection autoscaling={as} plan="starter" />);

    expect(screen.getByText("Autoscaling off")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("switch"));
    fireEvent.click(
      await screen.findByRole("button", { name: "Save Changes" }),
    );
    expect(as.save).toHaveBeenCalledWith({
      minInstances: 2,
      maxInstances: 4,
      targetCPUPercent: 70,
      targetMemoryPercent: null,
    });
  });
});

// w4/m364: the operator could not read utilization on production, held every
// autoscaled service at its count, and the card still read as active
// autoscaling. A paused autoscaler now says so.
describe("AutoscalingSection while utilization metrics are unavailable (w4/m364)", () => {
  it("says autoscaling is paused and keeps the settings editable", () => {
    const as = storedFreeConfig({ maxInstances: 3, metricsUnavailable: true });
    render(<AutoscalingSection autoscaling={as} plan="starter" />);

    expect(
      screen.getByText(
        "Autoscaling is paused: utilization metrics are unavailable",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Autoscaling on")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Save Changes" }),
    ).toBeInTheDocument();
  });

  it("shows no notice while metrics are read", () => {
    render(
      <AutoscalingSection
        autoscaling={storedFreeConfig({ maxInstances: 3 })}
        plan="starter"
      />,
    );
    expect(screen.queryByText(/Autoscaling is paused/)).not.toBeInTheDocument();
  });

  it("shows no notice when autoscaling is off", () => {
    render(
      <AutoscalingSection
        autoscaling={storedFreeConfig({
          enabled: false,
          metricsUnavailable: true,
        })}
        plan="starter"
      />,
    );
    expect(screen.queryByText(/Autoscaling is paused/)).not.toBeInTheDocument();
  });
});

// w7/m429: Render offers autoscaling on Pro workspaces and higher. On Hobby
// the card says so with a Billing link instead of letting the user fill the
// form and receive a 400; a policy already on keeps running and can only be
// turned off.
describe("AutoscalingSection on a Hobby workspace (w7/m429)", () => {
  const notice =
    "Autoscaling is available on the Pro workspace plan and higher. This workspace is on Hobby.";

  it("shows the plan notice and holds the switch off", () => {
    const as = storedFreeConfig({ enabled: false });
    render(
      <AutoscalingSection
        autoscaling={as}
        plan="starter"
        workspacePlan="hobby"
      />,
    );

    expect(screen.getByText("Autoscaling off")).toBeInTheDocument();
    expect(screen.getByText(notice, { exact: false })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open Billing" })).toHaveAttribute(
      "href",
      "/billing",
    );
    expect(screen.getByRole("switch")).toBeDisabled();
    expect(
      screen.queryByRole("button", { name: "Save Changes" }),
    ).not.toBeInTheDocument();
  });

  it("shows a running policy read-only, and turning it off still works", async () => {
    const as = storedFreeConfig({ maxInstances: 3, targetCPUPercent: 60 });
    render(
      <AutoscalingSection
        autoscaling={as}
        plan="starter"
        workspacePlan="hobby"
      />,
    );

    expect(screen.getByText("Autoscaling on")).toBeInTheDocument();
    expect(
      screen.getByText(/the policy below keeps running and can be turned off/),
    ).toBeInTheDocument();
    expect(screen.getByText(/Number of Instances: 1–3/)).toBeInTheDocument();
    // No editor: a change would be refused by the plan.
    expect(
      screen.queryByRole("button", { name: "Save Changes" }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("switch"));
    fireEvent.click(await screen.findByRole("button", { name: "Disable" }));
    expect(as.disable).toHaveBeenCalledOnce();
    expect(as.save).not.toHaveBeenCalled();
  });

  it("draws the notice in the loading frame too, so the ready card does not shift", () => {
    const { unmount } = render(
      <AutoscalingSection
        autoscaling={storedFreeConfig({ loading: true, enabled: false })}
        plan="starter"
        workspacePlan="hobby"
      />,
    );
    const loading = screen.getByTestId("autoscaling-loading");
    expect(
      within(loading).getByText(notice, { exact: false }),
    ).toBeInTheDocument();
    unmount();
  });

  it("leaves Pro and higher unchanged", () => {
    for (const workspacePlan of ["pro", "scale", "enterprise", undefined]) {
      const { unmount } = render(
        <AutoscalingSection
          autoscaling={storedFreeConfig({ enabled: false })}
          plan="starter"
          workspacePlan={workspacePlan}
        />,
      );
      expect(
        screen.queryByText(notice, { exact: false }),
      ).not.toBeInTheDocument();
      const toggle = screen.getByRole("switch");
      expect(toggle).toBeEnabled();
      fireEvent.click(toggle);
      expect(
        screen.getByRole("button", { name: "Save Changes" }),
      ).toBeInTheDocument();
      unmount();
    }
  });

  it("keeps the Free-instance rule first on a Hobby workspace", () => {
    render(
      <AutoscalingSection
        autoscaling={storedFreeConfig({ enabled: false })}
        plan="free"
        workspacePlan="hobby"
      />,
    );
    expect(
      screen.getByText(
        "Autoscaling is available on paid plans. Upgrade to enable it.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(notice, { exact: false }),
    ).not.toBeInTheDocument();
  });
});
