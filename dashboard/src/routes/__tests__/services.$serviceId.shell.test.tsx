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
  it("presents bex running-instance SSH without a browser terminal", async () => {
    renderPage();

    expect(
      await screen.findByRole("heading", { name: "Shell" }),
    ).toBeInTheDocument();
    expect(screen.getByText("ssh srv-example@ssh.bex.co")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Manage SSH public keys" }),
    ).toHaveAttribute("href", "/settings#ssh-public-keys");
    expect(
      screen.getByText(/connects to an existing ready instance/i),
    ).toBeInTheDocument();
  });

  it("explains why Shell is unavailable without inventing an address", async () => {
    serverState.service = {
      ...serverState.service!,
      sshAddress: null,
    };
    renderPage();

    expect(
      await screen.findByText("Shell access isn't available"),
    ).toBeInTheDocument();
    expect(screen.queryByText(/^ssh /)).not.toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Manage SSH public keys" }),
    ).toBeInTheDocument();
  });
});
