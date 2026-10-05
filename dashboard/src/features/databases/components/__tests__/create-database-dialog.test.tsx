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
  maxStorageGB: 1,
  supportsDiskAutoscaling: false,
  supportsConnectionPooling: false,
  maxReadReplicas: 0,
  supportsHighAvailability: false,
  monthlyUsd: "0.00",
};
const BASIC: DatabaseInstanceTypeView = {
  id: "basic-1gb",
  name: "Basic 1GB",
  cpu: "500m",
  memory: "1Gi",
  storageGB: 5,
  maxStorageGB: 16384,
  supportsDiskAutoscaling: true,
  supportsConnectionPooling: true,
  maxReadReplicas: 5,
  supportsHighAvailability: false,
  monthlyUsd: "14.00",
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

  // w4/m170: names PostgreSQL owns block Create with their own message.
  it("blocks a reserved database user or name before submit", async () => {
    const user = userEvent.setup();
    render(<CreateDatabaseDialog onCreated={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "New Database" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Name"), "shop-db");
    const submit = within(dialog).getByRole("button", {
      name: "Create database",
    });

    await user.type(within(dialog).getByLabelText("Database user"), "postgres");
    expect(
      within(dialog).getByText(
        "“postgres” is reserved by PostgreSQL. Choose another name.",
      ),
    ).toBeInTheDocument();
    expect(submit).toBeDisabled();

    await user.clear(within(dialog).getByLabelText("Database user"));
    await user.type(
      within(dialog).getByLabelText("Database name"),
      "template1",
    );
    expect(
      within(dialog).getByText(
        "“template1” is reserved by PostgreSQL. Choose another name.",
      ),
    ).toBeInTheDocument();
    expect(submit).toBeDisabled();

    await user.clear(within(dialog).getByLabelText("Database name"));
    await user.type(
      within(dialog).getByLabelText("Database name"),
      "orders_data",
    );
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

  it("keeps free storage at the API's fixed limit while allowing blank defaults", async () => {
    const user = userEvent.setup();
    render(<CreateDatabaseDialog onCreated={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "New Database" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Name"), "shop-db");
    const submit = within(dialog).getByRole("button", {
      name: "Create database",
    });
    const disk = within(dialog).getByLabelText("Disk size (GB)");

    expect(disk).toHaveAttribute("min", "1");
    expect(disk).toHaveAttribute("max", "1");
    await user.type(disk, "2");
    expect(within(dialog).getByText(/between 1 and 1 GB/i)).toBeInTheDocument();
    expect(submit).toBeDisabled();
    expect(create).not.toHaveBeenCalled();
    await user.clear(disk);
    expect(submit).toBeEnabled();
    await user.type(disk, "1");
    expect(submit).toBeEnabled();
    await user.clear(disk);
    await user.type(disk, "0");
    expect(submit).toBeDisabled();
  });

  it("uses the selected tier's disk bounds and retains the entered size across plan changes", async () => {
    instanceTypesState.instanceTypes = [FREE, { ...BASIC, maxStorageGB: 25 }];
    const user = userEvent.setup();
    render(<CreateDatabaseDialog onCreated={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "New Database" }));
    await user.type(screen.getByLabelText("Name"), "shop-db");
    await user.click(screen.getByRole("combobox", { name: "Instance type" }));
    await user.click(await screen.findByRole("option", { name: /Basic 1GB/ }));
    const disk = screen.getByLabelText("Disk size (GB)");
    const submit = screen.getByRole("button", { name: "Create database" });
    expect(disk).toHaveAttribute("min", "5");
    expect(disk).toHaveAttribute("max", "25");
    await user.type(disk, "26");
    expect(screen.getByText(/between 5 and 25 GB/i)).toBeVisible();
    expect(submit).toBeDisabled();
    await user.clear(disk);
    await user.type(disk, "25");
    expect(submit).toBeEnabled();
    await user.click(screen.getByRole("combobox", { name: "Instance type" }));
    await user.click(await screen.findByRole("option", { name: /^Free/ }));
    expect(disk).toHaveValue(25);
    expect(submit).toBeDisabled();
    expect(create).not.toHaveBeenCalled();
  });

  it("waits for the selected tier's storage limit before allowing creation", async () => {
    instanceTypesState.instanceTypes = [{ ...FREE, maxStorageGB: null }];
    const user = userEvent.setup();
    render(<CreateDatabaseDialog onCreated={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "New Database" }));
    await user.type(screen.getByLabelText("Name"), "shop-db");
    expect(screen.getByLabelText("Disk size (GB)")).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Create database" }),
    ).toBeDisabled();
    expect(create).not.toHaveBeenCalled();
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

  // w4/156: the plan options state their price, like the service picker.
  it("prices each plan option", async () => {
    const user = userEvent.setup();
    render(<CreateDatabaseDialog onCreated={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "New Database" }));
    const dialog = await screen.findByRole("dialog");
    await user.click(within(dialog).getByLabelText("Instance type"));
    expect(
      await screen.findByRole("option", {
        name: /Basic 1GB .*\$14\.00\/month/,
      }),
    ).toBeInTheDocument();
  });
});
