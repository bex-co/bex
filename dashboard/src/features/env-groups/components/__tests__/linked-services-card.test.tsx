import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  RouterProvider,
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from "@tanstack/react-router";
import { LinkedServicesCard } from "@/features/env-groups/components/linked-services-card";
import type { EnvGroupView } from "@/features/env-groups/types";
import type { ServiceView } from "@/features/services/types";

function service(id: string, name: string): ServiceView {
  return { id, name } as unknown as ServiceView;
}

function renderCard(props: {
  group: EnvGroupView;
  services: ServiceView[];
  serviceEnvironmentById: ReadonlyMap<string, string>;
}) {
  const root = createRootRoute();
  const route = createRoute({
    getParentRoute: () => root,
    path: "/",
    component: () => (
      <LinkedServicesCard
        group={props.group}
        services={props.services}
        linkGroup={vi.fn()}
        unlinkGroup={vi.fn()}
        busy={false}
        serviceEnvironmentById={props.serviceEnvironmentById}
        scopeReady
      />
    ),
  });
  const router = createRouter({
    routeTree: root.addChildren([route]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

const WORKSPACE_GROUP: EnvGroupView = {
  id: "eg-1",
  name: "shared",
  ownerId: "tea-1",
  environmentId: null,
  createdAt: null,
  updatedAt: null,
  revision: "1",
  serviceLinks: ["web"],
  envVarKeys: [],
  secretFileNames: [],
};

describe("LinkedServicesCard — w6/m48 workspace-scope compatibility", () => {
  it("shows no Legacy-link warning for a workspace-scoped group linked to a workspace-scoped service", async () => {
    renderCard({
      group: WORKSPACE_GROUP,
      services: [service("web", "web")],
      serviceEnvironmentById: new Map(), // "web" absent => environment-less
    });

    await screen.findByRole("link", { name: "web" }); // linked-service row rendered
    expect(screen.queryByText(/Legacy link/i)).not.toBeInTheDocument();
  });

  it("lists another unlinked, environment-less service as available to link", async () => {
    renderCard({
      group: WORKSPACE_GROUP,
      services: [service("web", "web"), service("api", "api")],
      serviceEnvironmentById: new Map(),
    });

    expect(await screen.findByText("Select a service")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /link service/i }),
    ).toBeInTheDocument();
  });

  it("still flags a genuinely cross-scope link and excludes it from available", async () => {
    renderCard({
      group: WORKSPACE_GROUP, // workspace-scoped (no Environment)
      services: [service("web", "web"), service("api", "api")],
      serviceEnvironmentById: new Map([
        ["web", "env-prod"], // linked "web" is actually in env-prod: genuine mismatch
        ["api", "env-prod"], // candidate "api" is also in env-prod: incompatible
      ]),
    });

    expect(await screen.findByText(/Legacy link/i)).toBeInTheDocument();
    // "api" is environment-scoped while the group is workspace-scoped, so no
    // compatible candidate exists — the Link Service control stays hidden.
    expect(screen.queryByText("Select a service")).not.toBeInTheDocument();
  });
});
