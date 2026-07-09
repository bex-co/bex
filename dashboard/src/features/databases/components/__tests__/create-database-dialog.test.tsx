import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CreateDatabaseDialog } from "@/features/databases/components/create-database-dialog";
import type { DatabaseInstanceTypeView } from "@/features/databases/types";

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

    // Invalid: must start with a letter — the inline error shows, submit stays off.
    await user.type(within(dialog).getByLabelText("Name"), "1bad");
    expect(
      within(dialog).getByText(/must start with a letter/i),
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
      plan: "free",
      version: "",
      diskSizeGB: 0,
      public: false,
    });
    expect(onCreated).toHaveBeenCalledWith("shop-db");
  });
});
