import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { WebShellPanel } from "@/features/services/components/web-shell-panel";
import type { ShellGate } from "@/features/services/hooks/use-shell-gate";

const allowed: ShellGate = { gate: { kind: "ready" }, reason: undefined };
const checking: ShellGate = {
  gate: { kind: "checking" },
  reason: "Checking whether you can perform this action…",
};
const adminOnly: ShellGate = {
  gate: { kind: "denied", reasonKey: "capabilities.actionAdminOnly" },
  reason: "This action is limited to admins.",
};
const unconfirmed: ShellGate = {
  gate: { kind: "unavailable", reasonKey: "capabilities.actionUnavailable" },
  reason: "The server could not confirm this action right now. Try again.",
};

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
    render(<WebShellPanel serviceId="srv-x" shell={allowed} />);
    // The terminal targets no specific instance by default (random Ready replica).
    expect(screen.getByTestId("terminal-target")).toHaveTextContent("any");
    // The picker shows the default "Any ready instance" selection.
    expect(screen.getByText("Any ready instance")).toBeInTheDocument();
  });

  // w4/501: the combobox is named by the visible "Instance" text.
  it("names the instance picker", () => {
    render(<WebShellPanel serviceId="srv-x" shell={allowed} />);
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
    render(
      <WebShellPanel
        serviceId="srv-x"
        initialInstance="srv-x-pod02"
        shell={allowed}
      />,
    );
    expect(screen.getByTestId("terminal-target")).toHaveTextContent(
      "srv-x-pod02",
    );
    expect(screen.getByText("Instance pod02")).toBeInTheDocument();
  });

  it("falls back to any ready instance for one that is gone", () => {
    render(
      <WebShellPanel
        serviceId="srv-x"
        initialInstance="srv-x-gone"
        shell={allowed}
      />,
    );
    expect(screen.getByTestId("terminal-target")).toHaveTextContent("any");
  });

  it("opens no terminal until the instance list has loaded", () => {
    instancesState.loading = true;
    instancesState.instances = [];
    const { rerender } = render(
      <WebShellPanel
        serviceId="srv-x"
        initialInstance="srv-x-pod02"
        shell={allowed}
      />,
    );
    expect(screen.queryByTestId("terminal-target")).toBeNull();
    expect(
      screen.getByTestId("web-shell-awaiting-instances"),
    ).toBeInTheDocument();
    instancesState.loading = false;
    instancesState.instances = [{ id: "srv-x-pod02", createdAt: "" }];
    rerender(
      <WebShellPanel
        serviceId="srv-x"
        initialInstance="srv-x-pod02"
        shell={allowed}
      />,
    );
    expect(screen.getByTestId("terminal-target")).toHaveTextContent(
      "srv-x-pod02",
    );
  });
});

// w4/726: Render keeps a protected environment's shell to Admins, so the
// terminal, which mints its ticket as it mounts, opens only once the shell
// decision allows it; a refused caller reads why in its place.
describe("WebShellPanel's shell decision", () => {
  beforeEach(() => {
    instancesState.loading = false;
    instancesState.instances = [{ id: "srv-x-pod01", createdAt: "" }];
  });

  it("shows a protected Developer Render's reason and opens no terminal", () => {
    render(<WebShellPanel serviceId="srv-x" shell={adminOnly} />);
    expect(screen.queryByTestId("terminal-target")).toBeNull();
    expect(screen.getByTestId("web-shell-refused")).toHaveTextContent(
      "This action is limited to admins.",
    );
    expect(screen.getByRole("combobox", { name: "Instance" })).toBeDisabled();
  });

  it("opens no terminal until the decision arrives, then opens it", () => {
    const { rerender } = render(
      <WebShellPanel serviceId="srv-x" shell={checking} />,
    );
    expect(screen.queryByTestId("terminal-target")).toBeNull();
    expect(
      screen.getByTestId("web-shell-awaiting-decision"),
    ).toBeInTheDocument();
    rerender(<WebShellPanel serviceId="srv-x" shell={allowed} />);
    expect(screen.getByTestId("terminal-target")).toHaveTextContent("any");
  });

  it("fails closed when the decision is unavailable before the first open", () => {
    render(<WebShellPanel serviceId="srv-x" shell={unconfirmed} />);
    expect(screen.queryByTestId("terminal-target")).toBeNull();
    expect(screen.getByTestId("web-shell-refused")).toHaveTextContent(
      "The server could not confirm this action right now.",
    );
  });

  it("waits for another service's own decision before it opens its terminal", () => {
    const { rerender } = render(
      <WebShellPanel serviceId="srv-x" shell={allowed} />,
    );
    rerender(<WebShellPanel serviceId="srv-y" shell={checking} />);
    expect(screen.queryByTestId("terminal-target")).toBeNull();
    expect(
      screen.getByTestId("web-shell-awaiting-decision"),
    ).toBeInTheDocument();
  });

  // A routine re-check must not drop a live session; a refusal ends it.
  it("keeps an open terminal through a re-check and closes it on a refusal", () => {
    const { rerender } = render(
      <WebShellPanel serviceId="srv-x" shell={allowed} />,
    );
    rerender(<WebShellPanel serviceId="srv-x" shell={checking} />);
    expect(screen.getByTestId("terminal-target")).toBeInTheDocument();
    rerender(<WebShellPanel serviceId="srv-x" shell={unconfirmed} />);
    expect(screen.getByTestId("terminal-target")).toBeInTheDocument();
    rerender(<WebShellPanel serviceId="srv-x" shell={adminOnly} />);
    expect(screen.queryByTestId("terminal-target")).toBeNull();
    expect(screen.getByTestId("web-shell-refused")).toHaveTextContent(
      "This action is limited to admins.",
    );
  });
});
