import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AccessControlPanel } from "@/features/databases/components/access-control-panel";

const createUser = vi.fn();
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
    users: [],
    loading: false,
    savingAllowList: false,
    creatingUser: false,
    saveAllowList: vi.fn(),
    createUser,
    deleteUser: vi.fn(),
    pooled,
    poolLoading: false,
    revealPooled: vi.fn(),
  }),
}));

beforeEach(() => {
  createUser.mockReset();
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
