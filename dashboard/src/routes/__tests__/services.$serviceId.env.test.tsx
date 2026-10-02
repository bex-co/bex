import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/features/services/components/service-environment-editor", () => ({
  ServiceEnvironmentEditor: () => <div>environment editor</div>,
}));
vi.mock("@/features/services/components/env-groups-panel", () => ({
  EnvGroupsPanel: () => null,
}));
const useServer = vi.fn();
vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: (...args: unknown[]) => useServer(...args),
}));

import { ServiceEnvPage } from "@/routes/services.$serviceId.env";

const NOTICE =
  "Saved changes aren't live yet. Use a standard deploy to apply them.";

// This page lists saved values. Cancellation, rollback and a restart of that
// release can leave those values different from the running configuration.
describe("ServiceEnvPage undeployed-changes notice", () => {
  beforeEach(() => useServer.mockReset());

  it("explains how to apply saved values that are not running", () => {
    useServer.mockReturnValue({ service: { undeployedChanges: true } });
    render(<ServiceEnvPage serviceId="srv-1" />);
    expect(screen.getByText(NOTICE)).toBeInTheDocument();
    expect(screen.getByText("environment editor")).toBeInTheDocument();
  });

  it("says nothing on an ordinary service, or before the service loads", () => {
    useServer.mockReturnValue({ service: { undeployedChanges: false } });
    const { unmount } = render(<ServiceEnvPage serviceId="srv-1" />);
    expect(screen.queryByText(NOTICE)).not.toBeInTheDocument();
    unmount();

    useServer.mockReturnValue({ service: null });
    render(<ServiceEnvPage serviceId="srv-1" />);
    expect(screen.queryByText(NOTICE)).not.toBeInTheDocument();
  });

  it("reads the layout's cached document without starting its own poll", () => {
    useServer.mockReturnValue({ service: null });
    render(<ServiceEnvPage serviceId="srv-1" />);
    // A second polling consumer drifts into separate round trips (use-server.ts);
    // the detail layout owns the cadence.
    expect(useServer).toHaveBeenCalledWith("srv-1", { poll: false });
  });
});
