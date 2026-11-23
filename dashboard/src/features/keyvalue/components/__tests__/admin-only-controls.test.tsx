import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ConnectionInfoPanel } from "@/features/keyvalue/components/connection-info-panel";
import { KeyValueNetworkingPanel } from "@/features/keyvalue/components/key-value-networking-panel";
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
const saveAllowList = vi.fn();
const conn = vi.hoisted(() => ({ error: undefined as Error | undefined }));

vi.mock("@/features/keyvalue/hooks/use-enable-key-value-internal-auth", () => ({
  useEnableKeyValueInternalAuth: () => ({ enable: vi.fn(), loading: false }),
}));
vi.mock("@/features/keyvalue/hooks/use-connection-info", () => ({
  useConnectionInfo: () => ({
    info: null,
    loading: false,
    error: conn.error,
    reveal,
    hide: vi.fn(),
  }),
}));
vi.mock("@/features/keyvalue/hooks/use-key-value-networking", () => ({
  useKeyValueNetworking: () => ({
    allowList: [{ cidrBlock: "203.0.113.0/24", description: "office" }],
    isPublic: true,
    externalAccessRestarts: false,
    loading: false,
    savingAllowList: false,
    saveAllowList,
  }),
}));

function connections() {
  return (
    <ConnectionInfoPanel id="red-cache" name="cache" internalAuth isPublic />
  );
}

beforeEach(() => {
  reveal.mockReset();
  saveAllowList.mockReset().mockResolvedValue(true);
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

// w4/m594: in a protected environment only an Admin reveals a Key Value's
// connection URLs or changes its IP rules (render.com/docs/projects).
describe("a protected Key Value's admin-only controls", () => {
  it("disables the reveal for a Developer with Render's reason, and sends nothing", async () => {
    adminOnlyAs("developer");
    const user = userEvent.setup();
    render(connections());
    expect(useAdminOnlyActions).toHaveBeenCalledWith("keyValue", "red-cache");
    const button = screen.getByRole("button", { name: /reveal/i });
    expect(button).toBeDisabled();
    expect(await tooltipOf(user, button)).toBe(ADMIN_ONLY_REASON);
    await user.click(button);
    expect(reveal).not.toHaveBeenCalled();
  });

  it("keeps the reveal for an Admin and gives a Viewer the role reason", async () => {
    const user = userEvent.setup();
    const { unmount } = render(connections());
    await user.click(screen.getByRole("button", { name: /reveal/i }));
    expect(reveal).toHaveBeenCalledTimes(1);
    unmount();

    adminOnlyAs("viewer");
    render(connections());
    const button = screen.getByRole("button", { name: /reveal/i });
    expect(button).toBeDisabled();
    expect(await tooltipOf(user, button)).toBe(ROLE_REASON);
  });

  it("reads a raced refusal as the same reason", () => {
    conn.error = codedGraphQLError("PROTECTED_ENVIRONMENT_ADMIN_REQUIRED");
    render(connections());
    expect(screen.getByRole("alert")).toHaveTextContent(ADMIN_ONLY_REASON);
  });

  it("disables the IP rules for a Developer, keeping them listed", async () => {
    adminOnlyAs("developer");
    const user = userEvent.setup();
    render(<KeyValueNetworkingPanel id="red-cache" />);
    expect(useAdminOnlyActions).toHaveBeenCalledWith("keyValue", "red-cache");
    expect(screen.getByDisplayValue("203.0.113.0/24")).toBeDisabled();
    const save = screen.getByRole("button", { name: "Save allowlist" });
    expect(save).toBeDisabled();
    expect(await tooltipOf(user, save)).toBe(ADMIN_ONLY_REASON);
    await user.click(
      screen.getByRole("button", { name: /remove 203.0.113.0/i }),
    );
    expect(saveAllowList).not.toHaveBeenCalled();
    expect(screen.getByDisplayValue("203.0.113.0/24")).toBeInTheDocument();
  });

  it("keeps them for an Admin", async () => {
    const user = userEvent.setup();
    render(<KeyValueNetworkingPanel id="red-cache" />);
    await user.click(
      screen.getByRole("button", { name: /remove 203.0.113.0/i }),
    );
    await user.click(screen.getByRole("button", { name: "Save allowlist" }));
    expect(saveAllowList).toHaveBeenCalledWith([]);
  });
});
