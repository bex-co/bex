import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AccessControlPanel } from "@/features/databases/components/access-control-panel";

const createUser = vi.fn();
const deleteUser = vi.fn();
let users: string[] = [];
let pooled: { internal: string; external: string } | null = null;
vi.mock("@/features/databases/hooks/use-database-instance-types", () => ({
  useDatabaseInstanceTypes: () => ({
    instanceTypes: [
      { id: "free", supportsConnectionPooling: false },
      { id: "basic-1gb", supportsConnectionPooling: true },
    ],
  }),
}));
vi.mock("@/features/databases/hooks/use-access-control", () => ({
  useAccessControl: () => ({
    allowList: [],
    users,
    loading: false,
    savingAllowList: false,
    creatingUser: false,
    saveAllowList: vi.fn(),
    createUser,
    deleteUser,
    pooled,
    poolLoading: false,
    revealPooled: vi.fn(),
  }),
}));

beforeEach(() => {
  createUser.mockReset();
  deleteUser.mockReset();
  users = [];
  pooled = null;
});

describe("AccessControlPanel database-user creation", () => {
  it("keeps the returned password visible on the owning database page", async () => {
    createUser.mockResolvedValue("one-time-password");
    const user = userEvent.setup();
    render(<AccessControlPanel id="dpg-source" plan="free" />);

    await user.type(screen.getByPlaceholderText("reporting"), "analytics");
    await user.click(screen.getByRole("button", { name: "Add user" }));

    expect(createUser).toHaveBeenCalledWith("analytics");
    expect(await screen.findByText("one-time-password")).toBeInTheDocument();
    expect(
      screen.getByText("Password for analytics — shown once:"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Copy .*password/i }),
    ).toBeInTheDocument();
    expect(screen.getByPlaceholderText("reporting")).toHaveValue("");
  });

  it("drops the one-time password once that user is deleted, and only then", async () => {
    createUser.mockResolvedValue("one-time-password");
    users = ["analytics", "other"];
    const user = userEvent.setup();
    render(<AccessControlPanel id="dpg-source" plan="free" />);
    await user.type(screen.getByPlaceholderText("reporting"), "analytics");
    await user.click(screen.getByRole("button", { name: "Add user" }));
    expect(await screen.findByText("one-time-password")).toBeInTheDocument();

    deleteUser.mockResolvedValue(true);
    await user.click(screen.getByRole("button", { name: /Delete .*other/i }));
    expect(screen.getByText("one-time-password")).toBeInTheDocument();

    deleteUser.mockResolvedValue(false); // a failed delete leaves the user
    await user.click(
      screen.getByRole("button", { name: /Delete .*analytics/i }),
    );
    expect(screen.getByText("one-time-password")).toBeInTheDocument();

    deleteUser.mockResolvedValue(true);
    await user.click(
      screen.getByRole("button", { name: /Delete .*analytics/i }),
    );
    expect(screen.queryByText("one-time-password")).not.toBeInTheDocument();
    expect(deleteUser).toHaveBeenLastCalledWith("analytics");
  });

  // w4/m170: adding "postgres" wedged the database; say so before the click.
  it("refuses a reserved role name before submit", async () => {
    const user = userEvent.setup();
    render(<AccessControlPanel id="dpg-source" plan="free" />);
    await user.type(screen.getByPlaceholderText("reporting"), "postgres");
    expect(screen.getByRole("button", { name: "Add user" })).toBeDisabled();
    expect(
      screen.getByText(
        "“postgres” is reserved by PostgreSQL. Choose another name.",
      ),
    ).toBeInTheDocument();
    expect(createUser).not.toHaveBeenCalled();

    await user.clear(screen.getByPlaceholderText("reporting"));
    await user.type(screen.getByPlaceholderText("reporting"), "qa_extra");
    expect(screen.getByRole("button", { name: "Add user" })).toBeEnabled();
  });

  it("names the allowlist and database-user inputs", () => {
    render(<AccessControlPanel id="dpg-source" plan="free" />);
    expect(
      screen.getByRole("textbox", { name: "New CIDR block" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("textbox", { name: "New rule description" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("textbox", { name: "Database username" }),
    ).toBeInTheDocument();
  });

  it("keeps the username recoverable and shows no credential on failure", async () => {
    createUser.mockResolvedValue(null);
    const user = userEvent.setup();
    render(<AccessControlPanel id="dpg-source" plan="free" />);

    const name = screen.getByRole("textbox", { name: "Database username" });
    await user.type(name, "analytics");
    await user.click(screen.getByRole("button", { name: "Add user" }));

    expect(name).toHaveValue("analytics");
    expect(
      screen.queryByText("Password for analytics — shown once:"),
    ).not.toBeInTheDocument();
  });
});

describe("AccessControlPanel connection pooling", () => {
  it("explains the plan restriction instead of offering a refused enable call", () => {
    pooled = { internal: "", external: "" };
    render(<AccessControlPanel id="dpg-source" plan="free" />);
    expect(
      screen.getByText(/This plan does not support connection pooling/),
    ).toBeVisible();
    expect(screen.queryByText(/PATCH \/v1\/postgres/)).not.toBeInTheDocument();
  });

  it("keeps the enable guidance for a supported plan without a pooler", () => {
    pooled = { internal: "", external: "" };
    render(<AccessControlPanel id="dpg-source" plan="basic-1gb" />);
    expect(screen.getByText(/PATCH \/v1\/postgres/)).toBeVisible();
  });

  it("keeps existing pooled connections visible on an unsupported plan", () => {
    pooled = { internal: "postgresql://existing-pool", external: "" };
    render(<AccessControlPanel id="dpg-source" plan="free" />);
    expect(screen.getByText("postgresql://existing-pool")).toBeVisible();
    expect(
      screen.queryByText(/This plan does not support connection pooling/),
    ).not.toBeInTheDocument();
  });
});
