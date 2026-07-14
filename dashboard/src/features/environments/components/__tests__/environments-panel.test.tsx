import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EnvironmentsPanel } from "@/features/environments/components/environments-panel";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";

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

function renderPanel() {
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

  it("renders one card per environment", () => {
    environmentsState.environments = [
      { id: "env-1", projectId: "prj-1", name: "staging", ownerId: "tea-1", createdAt: null, serviceIds: [], databaseIds: [], keyValueIds: [] },
      { id: "env-2", projectId: "prj-1", name: "production", ownerId: "tea-1", createdAt: null, serviceIds: ["api"], databaseIds: [], keyValueIds: [] },
    ];
    renderPanel();
    const cards = screen.getAllByTestId("env-card");
    expect(cards.map((c) => c.textContent)).toEqual(["staging", "production"]);
  });

  it("opens the new-environment dialog from the header button", async () => {
    const user = userEvent.setup();
    renderPanel();
    expect(screen.queryByTestId("new-env-dialog")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /New Environment/ }));
    expect(screen.getByTestId("new-env-dialog")).toBeInTheDocument();
  });
});
