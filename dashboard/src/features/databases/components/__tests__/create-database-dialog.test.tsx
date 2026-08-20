import { describe, it, expect, vi, beforeAll, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CreateDatabaseDialog } from "@/features/databases/components/create-database-dialog";
import type { DatabaseInstanceTypeView } from "@/features/databases/types";

beforeAll(() => {
  if (!Element.prototype.hasPointerCapture) {
    Element.prototype.hasPointerCapture = () => false;
  }
  if (!Element.prototype.releasePointerCapture) {
    Element.prototype.releasePointerCapture = () => {};
  }
});

const instanceTypesState: {
  instanceTypes: DatabaseInstanceTypeView[];
  loading: boolean;
  error: Error | undefined;
} = { instanceTypes: [], loading: false, error: undefined };

vi.mock("@/features/databases/hooks/use-database-instance-types", () => ({
  useDatabaseInstanceTypes: () => instanceTypesState,
}));

const create = vi.fn();
vi.mock("@/features/databases/hooks/use-create-database", () => ({
  useCreateDatabase: () => ({ create, busy: false }),
}));

vi.mock("@/features/projects/hooks/use-projects", () => ({
  useProjects: () => ({
    projects: [{ id: "prj-1", name: "Commerce" }],
  }),
}));

vi.mock("@/features/environments/hooks/use-environments", () => ({
  useEnvironments: (projectId: string | null) => ({
    environments:
      projectId === "prj-1"
        ? [{ id: "env-1", projectId: "prj-1", name: "Production" }]
        : [],
  }),
}));

const FREE: DatabaseInstanceTypeView = {
  id: "free",
  name: "Free",
  cpu: "100m",
  memory: "256Mi",
  storageGB: 1,
};
const BASIC: DatabaseInstanceTypeView = {
  id: "basic-1gb",
  name: "Basic 1GB",
  cpu: "500m",
  memory: "1Gi",
  storageGB: 5,
};

beforeEach(() => {
  instanceTypesState.instanceTypes = [FREE, BASIC];
  create.mockReset();
  create.mockResolvedValue("shop-db");
});

describe("CreateDatabaseDialog", () => {
  it("validates the name and keeps submit disabled until it's valid", async () => {
    const user = userEvent.setup();
    render(<CreateDatabaseDialog onCreated={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "New Database" }));
    const dialog = await screen.findByRole("dialog");
    const submit = within(dialog).getByRole("button", {
      name: "Create database",
    });
    expect(submit).toBeDisabled(); // empty name

    // Invalid: can't start with a hyphen — the inline error shows, submit stays off.
    await user.type(within(dialog).getByLabelText("Name"), "-bad");
    expect(
      within(dialog).getByText(/can't start or end with a hyphen/i),
    ).toBeInTheDocument();
    expect(submit).toBeDisabled();

    // Valid name enables submit.
    await user.clear(within(dialog).getByLabelText("Name"));
    await user.type(within(dialog).getByLabelText("Name"), "shop-db");
    expect(submit).toBeEnabled();
  });

  it('submits with the default plan and omits an unset version (default -> "")', async () => {
    const onCreated = vi.fn();
    const user = userEvent.setup();
    render(<CreateDatabaseDialog onCreated={onCreated} />);

    await user.click(screen.getByRole("button", { name: "New Database" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Name"), "shop-db");
    await user.click(
      within(dialog).getByRole("button", { name: "Create database" }),
    );

    // plan defaults to the catalog's first tier; version "default" maps to "";
    // an empty disk field becomes 0 (operator applies the plan floor).
    expect(create).toHaveBeenCalledWith({
      name: "shop-db",
      databaseName: "",
      databaseUser: "",
      plan: "free",
      version: "",
      diskSizeGB: 0,
      public: false,
      environmentId: undefined,
    });
    expect(onCreated).toHaveBeenCalledWith("shop-db");
  });

  it("validates and submits optional physical PostgreSQL names", async () => {
    const user = userEvent.setup();
    render(<CreateDatabaseDialog onCreated={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "New Database" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Name"), "shop-db");
    await user.type(
      within(dialog).getByLabelText("Database name"),
      "Orders-Data",
    );
    expect(
      within(dialog).getByRole("button", { name: "Create database" }),
    ).toBeDisabled();

    await user.clear(within(dialog).getByLabelText("Database name"));
    await user.type(
      within(dialog).getByLabelText("Database name"),
      "orders_data",
    );
    await user.type(
      within(dialog).getByLabelText("Database user"),
      "orders_owner",
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Create database" }),
    );

    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({
        databaseName: "orders_data",
        databaseUser: "orders_owner",
      }),
    );
  });

  it("blocks a disk size outside [plan floor, volume cap] but allows a large in-range one", async () => {
    const user = userEvent.setup();
    render(<CreateDatabaseDialog onCreated={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "New Database" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Name"), "shop-db");
    const submit = within(dialog).getByRole("button", {
      name: "Create database",
    });
    const disk = within(dialog).getByLabelText("Disk size (GB)");

    // Above the 16384 GB volume cap → refused inline, submit disabled.
    await user.type(disk, "99999");
    expect(
      within(dialog).getByText(/between 1 and 16384 GB/i),
    ).toBeInTheDocument();
    expect(submit).toBeDisabled();

    // A large but in-range size (below the cap) is valid — the plan's advertised
    // storage is a floor, not a ceiling, so this must NOT be blocked.
    await user.clear(disk);
    await user.type(disk, "9999");
    expect(
      within(dialog).queryByText(/between 1 and 16384 GB/i),
    ).not.toBeInTheDocument();
    expect(submit).toBeEnabled();

    // Below the plan floor (1 GB) → refused.
    await user.clear(disk);
    await user.type(disk, "0");
    expect(submit).toBeDisabled();
  });

  it("submits the selected environment so the backend auto-joins its project", async () => {
    const user = userEvent.setup();
    render(<CreateDatabaseDialog onCreated={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "New Database" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Name"), "shop-db");
    await user.click(within(dialog).getByRole("combobox", { name: "Project" }));
    await user.click(await screen.findByRole("option", { name: "Commerce" }));
    await user.click(
      within(dialog).getByRole("combobox", { name: "Environment" }),
    );
    await user.click(await screen.findByRole("option", { name: "Production" }));
    await user.click(
      within(dialog).getByRole("button", { name: "Create database" }),
    );

    expect(create).toHaveBeenCalledWith(
      expect.objectContaining({ environmentId: "env-1" }),
    );
  });
});
