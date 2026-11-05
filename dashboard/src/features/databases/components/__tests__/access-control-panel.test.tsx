import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AccessControlPanel } from "@/features/databases/components/access-control-panel";

const createCredential = vi.fn();
const deleteCredential = vi.fn();
let credentials: { username: string; default: boolean }[] = [];
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
    credentials,
    loading: false,
    savingAllowList: false,
    creatingCredential: false,
    saveAllowList: vi.fn(),
    createCredential,
    deleteCredential,
    pooled,
    poolLoading: false,
    revealPooled: vi.fn(),
  }),
}));

beforeEach(() => {
  createCredential.mockReset();
  deleteCredential.mockReset();
  credentials = [];
  pooled = null;
});

// w5/m265: Render's Credentials section — a new credential becomes the
// default, the default cannot be deleted, and no password is shown here.
describe("AccessControlPanel credentials", () => {
  it("marks the default and refuses to delete it, with the reason", async () => {
    credentials = [
      { username: "dpg_x_user", default: false },
      { username: "rotated", default: true },
    ];
    deleteCredential.mockResolvedValue(undefined);
    const user = userEvent.setup();
    render(<AccessControlPanel id="dpg-source" plan="free" />);

    const locked = screen.getByRole("button", { name: "Delete rotated" });
    expect(locked).toBeDisabled();
    expect(locked).toHaveAttribute(
      "title",
      "The default credential can't be deleted. Add a new default credential first.",
    );
    expect(screen.getAllByText("Default")).toHaveLength(1);

    await user.click(screen.getByRole("button", { name: "Delete dpg_x_user" }));
    expect(deleteCredential).toHaveBeenCalledWith("dpg_x_user");
  });

  it("creates a new default credential and clears the input, showing no password", async () => {
    createCredential.mockResolvedValue(true);
    const user = userEvent.setup();
    render(<AccessControlPanel id="dpg-source" plan="free" />);

    const name = screen.getByRole("textbox", {
      name: "New credential username",
    });
    await user.type(name, "rotated");
    await user.click(
      screen.getByRole("button", { name: "New default credential" }),
    );

    expect(createCredential).toHaveBeenCalledWith("rotated");
    expect(name).toHaveValue("");
    // Render echoes no password; it is read through the connection info.
    expect(
      screen.queryByRole("button", { name: /copy/i }),
    ).not.toBeInTheDocument();
  });

  it("keeps the username recoverable on failure", async () => {
    createCredential.mockResolvedValue(false);
    const user = userEvent.setup();
    render(<AccessControlPanel id="dpg-source" plan="free" />);

    const name = screen.getByRole("textbox", {
      name: "New credential username",
    });
    await user.type(name, "analytics");
    await user.click(
      screen.getByRole("button", { name: "New default credential" }),
    );
    expect(name).toHaveValue("analytics");
  });

  // w4/m170: adding "postgres" wedged the database; say so before the click.
  it("refuses a reserved role name before submit", async () => {
    const user = userEvent.setup();
    render(<AccessControlPanel id="dpg-source" plan="free" />);
    const name = screen.getByRole("textbox", {
      name: "New credential username",
    });
    await user.type(name, "postgres");
    const add = screen.getByRole("button", { name: "New default credential" });
    expect(add).toBeDisabled();
    expect(
      screen.getByText(
        "“postgres” is reserved by PostgreSQL. Choose another name.",
      ),
    ).toBeInTheDocument();
    expect(createCredential).not.toHaveBeenCalled();

    await user.clear(name);
    await user.type(name, "qa_extra");
    expect(add).toBeEnabled();
  });

  it("names the allowlist inputs", () => {
    render(<AccessControlPanel id="dpg-source" plan="free" />);
    expect(
      screen.getByRole("textbox", { name: "New CIDR block" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("textbox", { name: "New rule description" }),
    ).toBeInTheDocument();
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
