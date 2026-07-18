import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AccessControlPanel } from "@/features/databases/components/access-control-panel";

const createUser = vi.fn();
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
    pooled: null,
    poolLoading: false,
    revealPooled: vi.fn(),
  }),
}));

beforeEach(() => {
  createUser.mockReset();
});

describe("AccessControlPanel database-user creation", () => {
  it("keeps the returned password visible on the owning database page", async () => {
    createUser.mockResolvedValue("one-time-password");
    const user = userEvent.setup();
    render(<AccessControlPanel id="dpg-source" />);

    await user.type(screen.getByPlaceholderText("reporting"), "analytics");
    await user.click(screen.getByRole("button", { name: "Add user" }));

    expect(createUser).toHaveBeenCalledWith("analytics");
    expect(await screen.findByText("one-time-password")).toBeInTheDocument();
    expect(
      screen.getByText("Password for analytics — shown once:"),
    ).toBeInTheDocument();
    expect(screen.getByPlaceholderText("reporting")).toHaveValue("");
  });

  it("keeps the username recoverable and shows no credential on failure", async () => {
    createUser.mockResolvedValue(null);
    const user = userEvent.setup();
    render(<AccessControlPanel id="dpg-source" />);

    const name = screen.getByPlaceholderText("reporting");
    await user.type(name, "analytics");
    await user.click(screen.getByRole("button", { name: "Add user" }));

    expect(name).toHaveValue("analytics");
    expect(
      screen.queryByText("Password for analytics — shown once:"),
    ).not.toBeInTheDocument();
  });
});
