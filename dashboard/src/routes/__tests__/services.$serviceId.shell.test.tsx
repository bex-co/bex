import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRouter,
} from "@tanstack/react-router";
import type { UseServerResult } from "@/features/services/hooks/use-server";
import type { ServiceView } from "@/features/services/types";
import { ServiceShellPage } from "@/features/services/components/service-shell-page";

const serverState: UseServerResult = {
  service: null,
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => []),
};

vi.mock("@/features/services/hooks/use-server", () => ({
  useServer: () => serverState,
}));

// The real panel queries instances and opens a gateway WebSocket via Apollo;
// those paths have their own tests. Here we only assert the page composes the
// Web Shell alongside the SSH command.
vi.mock("@/features/services/components/web-shell-panel", () => ({
  WebShellPanel: ({ serviceId }: { serviceId: string }) => (
    <div data-testid="web-shell-terminal">{serviceId}</div>
  ),
}));

function renderPage() {
  const rootRoute = createRootRoute({
    component: () => <ServiceShellPage serviceId="srv-example" />,
  });
  const router = createRouter({
    routeTree: rootRoute,
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

beforeEach(() => {
  serverState.service = {
    id: "srv-example",
    name: "example",
    type: "web_service",
    sshAddress: "srv-example@ssh.bex.co",
  } as ServiceView;
  serverState.loading = false;
});

describe("service Shell page", () => {
  it("hosts the browser terminal alongside the copy-ready SSH command", async () => {
    renderPage();

    expect(
      await screen.findByRole("heading", { name: "Shell" }),
    ).toBeInTheDocument();
    // The in-browser Web Shell terminal (w2/m55).
    expect(screen.getByTestId("web-shell-terminal")).toHaveTextContent(
      "srv-example",
    );
    // Render shows both: the terminal and the copy-ready ssh command.
    expect(screen.getByText("ssh srv-example@ssh.bex.co")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Manage SSH public keys" }),
    ).toHaveAttribute("href", "/settings#ssh-public-keys");
  });

  it("explains why Shell is unavailable without inventing an address or terminal", async () => {
    serverState.service = {
      ...serverState.service!,
      sshAddress: null,
    };
    renderPage();

    expect(
      await screen.findByText("Shell access isn't available"),
    ).toBeInTheDocument();
    // No address => no browser terminal and no fabricated ssh command.
    expect(screen.queryByTestId("web-shell-terminal")).not.toBeInTheDocument();
    expect(screen.queryByText(/^ssh /)).not.toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Manage SSH public keys" }),
    ).toBeInTheDocument();
  });
});
