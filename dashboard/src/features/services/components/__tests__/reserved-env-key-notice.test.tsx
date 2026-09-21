import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { ReservedEnvKeyNotice } from "@/features/services/components/reserved-env-key-notice";

function renderNotice(ui: React.ReactNode) {
  const rootRoute = createRootRoute();
  const indexRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: () => <>{ui}</>,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([indexRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

// w4/m121: bex-api's refusal says "change the service's port field instead".
// Until this milestone that named no control the reader could open — on the
// dashboard the sentence was a dead end. The link is what makes it followable.
describe("ReservedEnvKeyNotice", () => {
  it("links a service's refusal to that service's Settings port control", async () => {
    renderNotice(<ReservedEnvKeyNotice envKey="PORT" serviceId="srv-1" />);

    expect(
      await screen.findByText(
        /PORT is set by bex from the service port\. Change the service's port field instead\./,
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", {
        name: "Change the service's port in Settings",
      }),
    ).toHaveAttribute("href", "/services/srv-1/settings#port");
  });

  it("links the create wizard's refusal to the wizard's own port field", async () => {
    renderNotice(<ReservedEnvKeyNotice envKey="PORT" portFieldId="svc-port" />);

    expect(
      await screen.findByRole("link", {
        name: "Set the service's port above",
      }),
    ).toHaveAttribute("href", "#svc-port");
  });

  // An env group can be linked to many services (or none yet), so there is no
  // single port to open — the notice sends the reader to the services list to
  // pick one rather than guessing.
  it("falls back to the services list when no single service owns the port", async () => {
    renderNotice(
      <ReservedEnvKeyNotice envKey="PORT" messageKey="envGroups.reservedKey" />,
    );

    expect(
      await screen.findByRole("link", {
        name: "Open a service's Settings to change its port",
      }),
    ).toHaveAttribute("href", "/");
  });
});
