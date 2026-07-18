import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EnvironmentsPanel } from "@/features/environments/components/environments-panel";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";

vi.mock("@tanstack/react-router", () => ({
  Link: ({ children }: { children: React.ReactNode }) => (
    <a href="#new">{children}</a>
  ),
}));

const environmentsState: {
  environments: EnvironmentView[];
  loading: boolean;
  error: Error | undefined;
  refetch: () => Promise<unknown>;
} = {
  environments: [],
  loading: false,
  error: undefined,
  refetch: vi.fn(async () => undefined),
};
vi.mock("@/features/environments/hooks/use-environments", () => ({
  useEnvironments: () => environmentsState,
}));
vi.mock("@/features/env-groups/hooks/use-env-groups", () => ({
  useEnvGroups: () => ({ groups: [], loading: false, error: undefined }),
}));

// Stub the children so this test targets the panel's own states + wiring; the
// card and dialog have their own tests and reach Apollo directly.
vi.mock("@/features/environments/components/environment-card", () => ({
  EnvironmentCard: ({ environment }: { environment: EnvironmentView }) => (
    <div data-testid="env-card">{environment.name}</div>
  ),
}));
vi.mock("@/features/environments/components/new-environment-dialog", () => ({
  NewEnvironmentDialog: ({ open }: { open: boolean }) =>
    open ? <div data-testid="new-env-dialog" /> : null,
}));
vi.mock("@/features/projects/components/resource-table", () => ({
  ResourceTable: ({ rows }: { rows: Array<{ name: string }> }) => (
    <div data-testid="resource-table">
      {rows.map((row) => row.name).join(",")}
    </div>
  ),
}));

function renderPanel(
  props: Partial<React.ComponentProps<typeof EnvironmentsPanel>> = {},
) {
  return render(
    <EnvironmentsPanel
      projectId="prj-1"
      services={[]}
      databases={[]}
      keyValues={[]}
      servicePending={null}
      onRunServiceAction={vi.fn()}
      onDatabaseDeleted={vi.fn()}
      onKeyValueDeleted={vi.fn()}
      {...props}
    />,
  );
}

beforeEach(() => {
  environmentsState.environments = [];
  environmentsState.loading = false;
  environmentsState.error = undefined;
});

describe("EnvironmentsPanel", () => {
  it("shows the empty state when the project has no environments", () => {
    renderPanel();
    expect(screen.getByText(/No environments yet/)).toBeInTheDocument();
    expect(screen.queryByTestId("env-card")).not.toBeInTheDocument();
  });

  it("shows an error state when the query fails", () => {
    environmentsState.error = new Error("boom");
    renderPanel();
    expect(
      screen.getByText(/Something went wrong loading environments/),
    ).toBeInTheDocument();
  });

  it("renders only the selected environment", () => {
    environmentsState.environments = [
      {
        id: "env-1",
        projectId: "prj-1",
        name: "staging",
        ownerId: "tea-1",
        createdAt: null,
        serviceIds: [],
        databaseIds: [],
        keyValueIds: [],
        envGroupIds: [],
        protectedStatus: "unprotected",
        networkIsolationEnabled: false,
        ipAllowListEntries: [],
      },
      {
        id: "env-2",
        projectId: "prj-1",
        name: "production",
        ownerId: "tea-1",
        createdAt: null,
        serviceIds: ["api"],
        databaseIds: [],
        keyValueIds: [],
        envGroupIds: [],
        protectedStatus: "unprotected",
        networkIsolationEnabled: false,
        ipAllowListEntries: [],
      },
    ];
    renderPanel();
    const cards = screen.getAllByTestId("env-card");
    expect(cards.map((c) => c.textContent)).toEqual(["staging"]);
  });

  it("opens the new-environment dialog from the header button", async () => {
    const user = userEvent.setup();
    renderPanel();
    expect(screen.queryByTestId("new-env-dialog")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /New Environment/ }));
    expect(screen.getByTestId("new-env-dialog")).toBeInTheDocument();
  });

  it("renders the URL-selected Environment and falls back an unknown id", async () => {
    environmentsState.environments = [
      {
        id: "env-1",
        projectId: "prj-1",
        name: "staging",
        ownerId: "tea-1",
        createdAt: null,
        serviceIds: [],
        databaseIds: [],
        keyValueIds: [],
        envGroupIds: [],
        protectedStatus: "unprotected",
        networkIsolationEnabled: false,
        ipAllowListEntries: [],
      },
      {
        id: "env-2",
        projectId: "prj-1",
        name: "production",
        ownerId: "tea-1",
        createdAt: null,
        serviceIds: [],
        databaseIds: [],
        keyValueIds: [],
        envGroupIds: [],
        protectedStatus: "unprotected",
        networkIsolationEnabled: false,
        ipAllowListEntries: [],
      },
    ];
    const { unmount } = renderPanel({
      resourceFilter: { environmentId: "env-2", query: "", kind: "all" },
    });
    expect(screen.getByTestId("env-card")).toHaveTextContent("production");
    unmount();

    const onResourceFilterChange = vi.fn();
    renderPanel({
      resourceFilter: {
        environmentId: "env-deleted",
        query: "api",
        kind: "services",
      },
      onResourceFilterChange,
    });
    expect(screen.getByTestId("env-card")).toHaveTextContent("staging");
    await waitFor(() =>
      expect(onResourceFilterChange).toHaveBeenCalledWith({
        environmentId: "env-1",
        query: "api",
        kind: "services",
      }),
    );
  });

  it("keeps project-only resources reachable through Unassigned", () => {
    environmentsState.environments = [];
    renderPanel({
      projectRows: [
        {
          kind: "service",
          id: "srv-legacy",
          name: "Legacy API",
          createdAt: null,
          updatedAt: null,
          runtime: null,
          region: null,
        },
      ],
      resourceFilter: {
        environmentId: "unassigned",
        query: "",
        kind: "all",
      },
    });

    expect(
      screen.getByRole("heading", { name: "Unassigned" }),
    ).toBeInTheDocument();
    expect(screen.getByTestId("resource-table")).toHaveTextContent(
      "Legacy API",
    );
  });

  it("does not canonicalize an explicit Unassigned URL while rows are empty", async () => {
    environmentsState.environments = [
      {
        id: "env-1",
        projectId: "prj-1",
        name: "production",
        ownerId: "tea-1",
        createdAt: null,
        serviceIds: [],
        databaseIds: [],
        keyValueIds: [],
        envGroupIds: [],
        protectedStatus: "unprotected",
        networkIsolationEnabled: false,
        ipAllowListEntries: [],
      },
    ];
    const onResourceFilterChange = vi.fn();
    renderPanel({
      resourceFilter: {
        environmentId: "unassigned",
        query: "",
        kind: "all",
      },
      onResourceFilterChange,
    });

    expect(
      screen.getByRole("heading", { name: "Unassigned" }),
    ).toBeInTheDocument();
    await waitFor(() => expect(onResourceFilterChange).not.toHaveBeenCalled());
  });
});
