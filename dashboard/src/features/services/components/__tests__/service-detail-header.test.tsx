import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ServiceDetailHeader } from "@/features/services/components/service-detail-header";
import type { ServiceView } from "@/features/services/types";

// ServiceDetailHeader renders ServiceRowActions, which renders a "Move to
// project" submenu via useMoveToProject — needs an ApolloProvider + workspace
// context neither this render nor the component under test cares about; stub
// it to the empty-projects shape (the submenu renders nothing with no projects).
vi.mock("@/features/projects/hooks/use-move-to-project", () => ({
  useMoveToProject: () => ({
    projects: [],
    currentProjectId: () => null,
    moveTo: vi.fn(),
    removeFromProject: vi.fn(),
    busyId: null,
  }),
}));

function svc(overrides: Partial<ServiceView> = {}): ServiceView {
  return {
    id: "app",
    name: "app",
    suspended: false,
    phase: "Running",
    url: "https://app.onbex.co",
    createdAt: "2026-01-01T00:00:00Z",
    replicas: 1,
    revision: "r1",
    ...overrides,
  };
}

describe("ServiceDetailHeader", () => {
  it("renders the service identity, status badge, and live URL", () => {
    render(
      <ServiceDetailHeader service={svc()} pending={null} onRun={vi.fn()} />,
    );

    expect(screen.getByRole("heading", { name: "app" })).toBeInTheDocument();
    expect(screen.getByText("Running")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "https://app.onbex.co" }),
    ).toHaveAttribute("href", "https://app.onbex.co");
  });

  it("fires a lifecycle action through the reused row-actions control", async () => {
    const onRun = vi.fn();
    const suspended = svc({ suspended: true, phase: "Hibernated", url: null });
    const user = userEvent.setup();

    render(
      <ServiceDetailHeader service={suspended} pending={null} onRun={onRun} />,
    );

    // a suspended service exposes only Resume, which runs without a confirm
    await user.click(screen.getByRole("button", { name: "Open actions menu" }));
    await user.click(screen.getByRole("menuitem", { name: "Resume" }));

    expect(onRun).toHaveBeenCalledWith("resume", suspended);
  });

  it("disables the actions control while an action is in flight (poll-to-converge)", () => {
    render(
      <ServiceDetailHeader service={svc()} pending="restart" onRun={vi.fn()} />,
    );
    expect(
      screen.getByRole("button", { name: "Open actions menu" }),
    ).toBeDisabled();
  });
});
