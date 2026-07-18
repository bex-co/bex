import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { WebShellPanel } from "@/features/services/components/web-shell-panel";

vi.mock("@/features/services/hooks/use-service-instances", () => ({
  useServiceInstances: () => ({
    instances: [
      { id: "srv-x-pod01", createdAt: "" },
      { id: "srv-x-pod02", createdAt: "" },
    ],
    loading: false,
    refetch: vi.fn(),
  }),
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
});
