import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { WebShellPanel } from "@/features/services/components/web-shell-panel";

const instancesState = {
  instances: [
    { id: "srv-x-pod01", createdAt: "" },
    { id: "srv-x-pod02", createdAt: "" },
  ],
  loading: false,
};
vi.mock("@/features/services/hooks/use-service-instances", () => ({
  useServiceInstances: () => ({ ...instancesState, refetch: vi.fn() }),
}));

// Capture the instanceId the panel hands the terminal without opening a socket.
vi.mock("@/features/services/components/web-shell-terminal", () => ({
  WebShellTerminal: ({ instanceId }: { instanceId?: string }) => (
    <div data-testid="terminal-target">{instanceId ?? "any"}</div>
  ),
}));

describe("WebShellPanel", () => {
  it("defaults to any ready instance and offers the picker", () => {
    render(<WebShellPanel serviceId="srv-x" />);
    // The terminal targets no specific instance by default (random Ready replica).
    expect(screen.getByTestId("terminal-target")).toHaveTextContent("any");
    // The picker shows the default "Any ready instance" selection.
    expect(screen.getByText("Any ready instance")).toBeInTheDocument();
  });

  // w4/501: the combobox is named by the visible "Instance" text.
  it("names the instance picker", () => {
    render(<WebShellPanel serviceId="srv-x" />);
    expect(
      screen.getByRole("combobox", { name: "Instance" }),
    ).toBeInTheDocument();
  });
});

// A log line's SSH into instance opens the Shell on its instance (w3/m528,
// Render's `/shell/{instance}`).
describe("WebShellPanel opened on an instance", () => {
  beforeEach(() => {
    instancesState.loading = false;
    instancesState.instances = [
      { id: "srv-x-pod01", createdAt: "" },
      { id: "srv-x-pod02", createdAt: "" },
    ];
  });

  it("starts on the instance the link names when it is running", () => {
    render(<WebShellPanel serviceId="srv-x" initialInstance="srv-x-pod02" />);
    expect(screen.getByTestId("terminal-target")).toHaveTextContent(
      "srv-x-pod02",
    );
    expect(screen.getByText("Instance pod02")).toBeInTheDocument();
  });

  it("falls back to any ready instance for one that is gone", () => {
    render(<WebShellPanel serviceId="srv-x" initialInstance="srv-x-gone" />);
    expect(screen.getByTestId("terminal-target")).toHaveTextContent("any");
  });

  it("opens no terminal until the instance list has loaded", () => {
    instancesState.loading = true;
    instancesState.instances = [];
    const { rerender } = render(
      <WebShellPanel serviceId="srv-x" initialInstance="srv-x-pod02" />,
    );
    expect(screen.queryByTestId("terminal-target")).toBeNull();
    expect(
      screen.getByTestId("web-shell-awaiting-instances"),
    ).toBeInTheDocument();
    instancesState.loading = false;
    instancesState.instances = [{ id: "srv-x-pod02", createdAt: "" }];
    rerender(<WebShellPanel serviceId="srv-x" initialInstance="srv-x-pod02" />);
    expect(screen.getByTestId("terminal-target")).toHaveTextContent(
      "srv-x-pod02",
    );
  });
});
