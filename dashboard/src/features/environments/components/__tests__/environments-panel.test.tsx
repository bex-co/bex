import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EnvironmentsPanel } from "@/features/environments/components/environments-panel";
import { namesSection } from "@/features/environments/lib/sections";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import type { ResourceRow } from "@/features/projects/types";

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
vi.mock("@/features/sandbox-groups/hooks/use-sandbox-groups", () => ({
  useSandboxGroups: () => ({ groups: [], loading: false, error: undefined }),
}));

// Stub the sections so this test targets the panel's own stacking and states;
// a section has its own tests and reaches Apollo directly.
vi.mock("@/features/environments/components/environment-section", () => ({
  EnvironmentSection: ({
    environment,
    focused,
  }: {
    environment: EnvironmentView;
    focused?: boolean;
  }) => (
    <div data-testid="env-section" data-focused={focused ? "" : undefined}>
      {environment.name}
    </div>
  ),
}));
vi.mock("@/features/projects/components/resource-table", () => ({
  ResourceTable: ({ rows }: { rows: Array<{ name: string }> }) => (
    <div data-testid="resource-table">
      {rows.map((row) => row.name).join(",")}
    </div>
  ),
}));

const empty: EnvironmentView = {
  id: "evm-1",
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
};
const environment = (id: string, name: string, extra = {}) => ({
  ...empty,
  id,
  name,
  ...extra,
});

// An environment id's 20-character xid suffix.
const XID = "davm6imde41s73canq0g";

const projectOnly: ResourceRow = {
  kind: "service",
  id: "srv-unassigned",
  name: "Unassigned API",
  createdAt: null,
  updatedAt: null,
  runtime: null,
  region: null,
};

function renderPanel(
  props: Partial<React.ComponentProps<typeof EnvironmentsPanel>> = {},
) {
  return render(
    <EnvironmentsPanel
      projectName="shop"
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

const sectionNames = () =>
  screen.getAllByTestId("env-section").map((section) => section.textContent);

beforeEach(() => {
  environmentsState.environments = [];
  environmentsState.loading = false;
  environmentsState.error = undefined;
});

describe("EnvironmentsPanel", () => {
  // Render's project page stacks every environment (w1/m682).
  it("stacks every environment in the project's order", () => {
    environmentsState.environments = [
      environment("evm-1", "production"),
      environment("evm-2", "staging"),
      environment("evm-3", "preview"),
    ];
    renderPanel();
    expect(sectionNames()).toEqual(["production", "staging", "preview"]);
    // More than one environment: no Add environment card.
    expect(
      screen.queryByRole("button", { name: "Add environment" }),
    ).toBeNull();
  });

  it("shows Render's Add environment card under a project's only environment", async () => {
    const user = userEvent.setup();
    const onAddEnvironment = vi.fn();
    environmentsState.environments = [environment("evm-1", "production")];
    renderPanel({ onAddEnvironment });
    await user.click(screen.getByRole("button", { name: "Add environment" }));
    expect(onAddEnvironment).toHaveBeenCalledTimes(1);
  });

  it("disables the Add environment card with the role's reason", () => {
    environmentsState.environments = [environment("evm-1", "production")];
    renderPanel({ addEnvironmentReason: "Viewers can't create resources." });
    expect(
      screen.getByRole("button", { name: "Add environment" }),
    ).toBeDisabled();
  });

  it("keeps project-only resources in an Unassigned section after the environments", () => {
    environmentsState.environments = [
      environment("evm-1", "production", { serviceIds: ["srv-in"] }),
      environment("evm-2", "staging"),
    ];
    renderPanel({ projectRows: [projectOnly] });
    const unassigned = screen.getByRole("region", { name: "Unassigned" });
    expect(within(unassigned).getByTestId("resource-table")).toHaveTextContent(
      "Unassigned API",
    );
    // After the last environment, in document order.
    const last = screen.getAllByTestId("env-section").at(-1)!;
    expect(
      last.compareDocumentPosition(unassigned) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("draws no Unassigned section when every project resource has an environment", () => {
    environmentsState.environments = [
      environment("evm-1", "production", { serviceIds: ["srv-unassigned"] }),
      environment("evm-2", "staging"),
    ];
    renderPanel({ projectRows: [projectOnly] });
    expect(screen.queryByRole("region", { name: "Unassigned" })).toBeNull();
  });

  it("keeps Unassigned to say a failed list may hide its rows (w4/m227)", () => {
    environmentsState.environments = [
      environment("evm-1", "production"),
      environment("evm-2", "staging"),
    ];
    renderPanel({
      catalog: {
        pending: [],
        failed: ["services"],
        retry: async () => undefined,
      },
    });
    const unassigned = screen.getByRole("region", { name: "Unassigned" });
    expect(within(unassigned).getByRole("alert")).toBeVisible();
  });

  it("outlines the section the environment URL names, whichever id spelling it uses", () => {
    environmentsState.environments = [
      environment("evm-1", "production"),
      environment(`evm-${XID}`, "staging"),
    ];
    // A pre-w2/m165 link spells the same environment `env-`.
    const { rerender } = renderPanel({ focusedId: `env-${XID}` });
    const focused = () =>
      screen
        .getAllByTestId("env-section")
        .filter((section) => section.hasAttribute("data-focused"))
        .map((section) => section.textContent);
    expect(focused()).toEqual(["staging"]);

    rerender(
      <EnvironmentsPanel
        projectName="shop"
        projectId="prj-1"
        services={[]}
        databases={[]}
        keyValues={[]}
        servicePending={null}
        onRunServiceAction={vi.fn()}
        onDatabaseDeleted={vi.fn()}
        onKeyValueDeleted={vi.fn()}
        focusedId="evm-unknown"
      />,
    );
    expect(focused()).toEqual([]);
  });

  it("shows the empty state when the project has no environments", () => {
    renderPanel();
    expect(screen.getByText(/No environments yet/)).toBeInTheDocument();
    expect(screen.queryByTestId("env-section")).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Add environment" }),
    ).toBeVisible();
  });

  it("previews a section while environments load", () => {
    environmentsState.loading = true;
    const { container } = renderPanel();
    expect(
      container.querySelector('[data-skeleton-item="environment-section"]'),
    ).not.toBeNull();
    expect(screen.queryByTestId("env-section")).toBeNull();
  });

  it("shows an error state when the query fails", () => {
    environmentsState.error = new Error("boom");
    renderPanel();
    expect(
      screen.getByText(/Something went wrong loading environments/),
    ).toBeInTheDocument();
  });

  // w1/m153: a refresh that fails over environments already on screen keeps
  // the sections (and whatever dialog or draft they hold) and says so inline.
  it("keeps the sections mounted and shows the error inline when a refresh fails over cached data", () => {
    environmentsState.environments = [environment("evm-1", "staging")];
    environmentsState.error = new Error("poll failed");
    renderPanel();

    expect(screen.getByTestId("env-section")).toHaveTextContent("staging");
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Couldn't refresh environments. Showing the last loaded data.",
    );
  });
});

describe("namesSection", () => {
  it("knows the project's environments, in either id spelling, and Unassigned", () => {
    const environments = [{ id: "evm-1" }, { id: `evm-${XID}` }];
    expect(namesSection(environments, `evm-${XID}`)).toBe(true);
    expect(namesSection(environments, `env-${XID}`)).toBe(true);
    expect(namesSection(environments, "unassigned")).toBe(true);
    expect(namesSection(environments, "evm-3")).toBe(false);
  });
});
