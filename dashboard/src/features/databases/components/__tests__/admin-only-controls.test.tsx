import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ConnectionInfoPanel } from "@/features/databases/components/connection-info-panel";
import { AccessControlPanel } from "@/features/databases/components/access-control-panel";
import { useAdminOnlyActions } from "@/features/capabilities/hooks/use-admin-only-actions";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
import { codedGraphQLError } from "@/test/mocks/apollo";
import {
  ADMIN_ONLY_REASON,
  ROLE_REASON,
  adminOnlyAs,
} from "@/test/mocks/admin-only";

const reveal = vi.fn();
const revealPooled = vi.fn();
const saveAllowList = vi.fn();
const conn = vi.hoisted(() => ({ error: undefined as Error | undefined }));

vi.mock("@/features/databases/hooks/use-connection-info", () => ({
  useConnectionInfo: () => ({
    info: null,
    loading: false,
    error: conn.error,
    reveal,
    hide: vi.fn(),
  }),
}));
vi.mock("@apollo/client/react", () => ({
  useMutation: () => [vi.fn(), { loading: false }],
}));
vi.mock("@/features/databases/hooks/use-database-instance-types", () => ({
  useDatabaseInstanceTypes: () => ({
    instanceTypes: [{ id: "basic-1gb", supportsConnectionPooling: true }],
  }),
}));
vi.mock("@/features/databases/hooks/use-access-control", () => ({
  useAccessControl: () => ({
    allowList: [{ cidrBlock: "203.0.113.0/24", description: "office" }],
    allowListPending: false,
    credentials: [{ username: "app", default: true }],
    credentialsPending: false,
    loading: false,
    savingAllowList: false,
    creatingCredential: false,
    deletingCredential: false,
    saveAllowList,
    createCredential: vi.fn(),
    deleteCredential: vi.fn(),
    pooled: null,
    poolLoading: false,
    revealPooled,
    resetPooled: vi.fn(),
  }),
}));

const DATABASE = {
  id: "dpg-orders",
  name: "orders",
  plan: "basic-1gb",
  poolerEnabled: true,
  highAvailabilityEnabled: false,
};

beforeEach(() => {
  reveal.mockReset();
  revealPooled.mockReset();
  saveAllowList.mockReset();
  conn.error = undefined;
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
  adminOnlyAs("admin");
});

async function tooltipOf(
  user: ReturnType<typeof userEvent.setup>,
  control: HTMLElement,
) {
  await user.hover(control.parentElement!);
  return (await screen.findByRole("tooltip")).textContent;
}

// w4/m594: in a protected environment only an Admin reveals a Postgres's
// connection info, reads its pooled strings or changes its IP rules
// (render.com/docs/projects). A Developer used to be offered all three and
// read a 403 after the click.
describe("a protected Postgres's admin-only controls", () => {
  it("disables the connection-info reveal for a Developer with Render's reason, and sends nothing", async () => {
    adminOnlyAs("developer");
    const user = userEvent.setup();
    render(<ConnectionInfoPanel id="dpg-orders" />);
    expect(useAdminOnlyActions).toHaveBeenCalledWith("database", "dpg-orders");
    const button = screen.getByRole("button", {
      name: /reveal connection info/i,
    });
    expect(button).toBeDisabled();
    expect(await tooltipOf(user, button)).toBe(ADMIN_ONLY_REASON);
    await user.click(button);
    expect(reveal).not.toHaveBeenCalled();
  });

  it("keeps the reveal for an Admin", async () => {
    const user = userEvent.setup();
    render(<ConnectionInfoPanel id="dpg-orders" />);
    await user.click(
      screen.getByRole("button", { name: /reveal connection info/i }),
    );
    expect(reveal).toHaveBeenCalledTimes(1);
  });

  it("gives a Viewer the role reason", async () => {
    adminOnlyAs("viewer");
    const user = userEvent.setup();
    render(<ConnectionInfoPanel id="dpg-orders" />);
    const button = screen.getByRole("button", {
      name: /reveal connection info/i,
    });
    expect(button).toBeDisabled();
    expect(await tooltipOf(user, button)).toBe(ROLE_REASON);
  });

  it("reads a raced refusal as the same reason", () => {
    conn.error = codedGraphQLError("PROTECTED_ENVIRONMENT_ADMIN_REQUIRED");
    render(<ConnectionInfoPanel id="dpg-orders" />);
    expect(screen.getByRole("alert")).toHaveTextContent(ADMIN_ONLY_REASON);
  });

  it("disables the IP rules and the pooled reveal for a Developer, keeping the rules listed", async () => {
    adminOnlyAs("developer");
    const user = userEvent.setup();
    render(<AccessControlPanel database={DATABASE} onChanged={vi.fn()} />);
    expect(screen.getByDisplayValue("203.0.113.0/24")).toBeDisabled();
    expect(
      screen.getByRole("textbox", { name: "New CIDR block" }),
    ).toBeDisabled();
    const save = screen.getByRole("button", { name: "Save allowlist" });
    expect(save).toBeDisabled();
    expect(await tooltipOf(user, save)).toBe(ADMIN_ONLY_REASON);
    const remove = screen.getByRole("button", { name: /remove 203.0.113.0/i });
    expect(remove).toBeDisabled();
    await user.click(remove);
    const pooled = screen.getByRole("button", {
      name: /reveal pooled/i,
    });
    expect(pooled).toBeDisabled();
    await user.click(pooled);
    expect(saveAllowList).not.toHaveBeenCalled();
    expect(revealPooled).not.toHaveBeenCalled();
    expect(screen.getByDisplayValue("203.0.113.0/24")).toBeInTheDocument();
  });

  it("keeps both for an Admin", async () => {
    const user = userEvent.setup();
    render(<AccessControlPanel database={DATABASE} onChanged={vi.fn()} />);
    await user.type(
      screen.getByRole("textbox", { name: "New CIDR block" }),
      "198.51.100.0/24",
    );
    await user.click(screen.getByRole("button", { name: "Add" }));
    await user.click(screen.getByRole("button", { name: "Save allowlist" }));
    expect(saveAllowList).toHaveBeenCalledTimes(1);
    await user.click(screen.getByRole("button", { name: /reveal pooled/i }));
    expect(revealPooled).toHaveBeenCalledTimes(1);
  });
});
