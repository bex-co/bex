import { describe, expect, it, vi } from "vitest";
import { useState } from "react";
import { act, fireEvent, render, screen } from "@testing-library/react";
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

type CardProps = {
  group: EnvGroupView;
  services: ServiceView[];
  serviceEnvironmentById: ReadonlyMap<string, string>;
  scopeReady?: boolean;
  error?: Error;
};

function renderCard(initial: CardProps) {
  let setProps: (update: (prev: CardProps) => CardProps) => void = () => {};
  function Harness() {
    const [props, set] = useState(initial);
    setProps = set;
    return (
      <LinkedServicesCard
        group={props.group}
        services={props.services}
        linkGroup={vi.fn()}
        unlinkGroup={unlinkGroup}
        busy={!(props.scopeReady ?? true)}
        error={props.error}
        serviceEnvironmentById={props.serviceEnvironmentById}
        scopeReady={props.scopeReady ?? true}
      />
    );
  }
  const root = createRootRoute();
  const route = createRoute({
    getParentRoute: () => root,
    path: "/",
    component: Harness,
  });
  const router = createRouter({
    routeTree: root.addChildren([route]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  return {
    ...render(<RouterProvider router={router} />),
    update: (patch: Partial<CardProps>) =>
      act(() => setProps((prev) => ({ ...prev, ...patch }))),
  };
}

const unlinkGroup = vi.fn(async () => true);

const WORKSPACE_GROUP: EnvGroupView = {
  id: "eg-1",
  name: "shared",
  ownerId: "tea-1",
  environmentId: null,
  createdAt: null,
  updatedAt: null,
  revision: "1",
  availability: null,
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

    const selector = await screen.findByRole("combobox", {
      name: "Select a service",
    });
    fireEvent.keyDown(selector, { key: "ArrowDown" });
    fireEvent.click(await screen.findByRole("option", { name: "api" }));
    expect(
      screen.getByRole("combobox", { name: "Select a service" }),
    ).toHaveTextContent("api");
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

// w4/150: each row's Unlink (which redeploys that service) names the service.
describe("LinkedServicesCard — row control names (w4/150)", () => {
  it("names each Unlink control after its service", async () => {
    renderCard({
      group: { ...WORKSPACE_GROUP, serviceLinks: ["srv-web", "srv-api"] },
      services: [service("srv-web", "storefront"), service("srv-api", "api")],
      serviceEnvironmentById: new Map(),
    });

    expect(
      await screen.findByRole("button", { name: "Unlink storefront" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Unlink api" }),
    ).toBeInTheDocument();
  });
});

// w4/257: on a fresh load the scope index is empty until its
// query resolves, and every linked row of an Environment-scoped group read as
// "Legacy link outside this group's Environment" for several seconds.
describe("LinkedServicesCard — scope index still loading (w4/257)", () => {
  const ENV_GROUP: EnvGroupView = {
    ...WORKSPACE_GROUP,
    environmentId: "env-prod",
    serviceLinks: ["srv-web"],
  };

  it("makes no incompatibility claim until the index resolves, then none for a match", async () => {
    const { update } = renderCard({
      group: ENV_GROUP,
      services: [],
      serviceEnvironmentById: new Map(),
      scopeReady: false,
    });

    await screen.findByRole("link", { name: "srv-web" });
    expect(screen.queryByText(/Legacy link/i)).not.toBeInTheDocument();
    expect(screen.getByTestId("linked-row-pending")).toBeInTheDocument();

    update({
      services: [service("srv-web", "storefront")],
      serviceEnvironmentById: new Map([["srv-web", "env-prod"]]),
      scopeReady: true,
    });

    expect(
      await screen.findByRole("link", { name: "storefront" }),
    ).toBeInTheDocument();
    expect(screen.getByText("srv-web")).toBeInTheDocument();
    expect(screen.queryByText(/Legacy link/i)).not.toBeInTheDocument();
    expect(screen.queryByTestId("linked-row-pending")).not.toBeInTheDocument();
  });

  it("warns once a genuine mismatch settles, and Unlink still works", async () => {
    const { update } = renderCard({
      group: ENV_GROUP,
      services: [service("srv-web", "storefront")],
      serviceEnvironmentById: new Map(),
      scopeReady: false,
    });
    await screen.findByRole("link", { name: "storefront" });
    expect(screen.queryByText(/Legacy link/i)).not.toBeInTheDocument();

    update({
      serviceEnvironmentById: new Map([["srv-web", "env-staging"]]),
      scopeReady: true,
    });

    expect(await screen.findByText(/Legacy link/i)).toBeInTheDocument();
    unlinkGroup.mockClear();
    fireEvent.click(screen.getByRole("button", { name: "Unlink storefront" }));
    expect(unlinkGroup).toHaveBeenCalledWith("eg-1", "srv-web");
  });

  it("leaves a failed index read to the error alert, with no warning or placeholder", async () => {
    renderCard({
      group: ENV_GROUP,
      services: [],
      serviceEnvironmentById: new Map(),
      scopeReady: false,
      error: new Error("environment index failed"),
    });

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(screen.queryByText(/Legacy link/i)).not.toBeInTheDocument();
    expect(screen.queryByTestId("linked-row-pending")).not.toBeInTheDocument();
  });
});
