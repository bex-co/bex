import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toast } from "sonner";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";
import { WorkspaceNotificationsCard } from "@/features/notifications/components/workspace-notifications-card";
import type {
  OwnerNotificationSettingsView,
  UseOwnerNotificationSettingsResult,
} from "@/features/notifications/hooks/use-owner-notification-settings";

const hook = vi.hoisted(() => ({
  current: null as unknown as UseOwnerNotificationSettingsResult,
}));

vi.mock(
  "@/features/notifications/hooks/use-owner-notification-settings",
  () => ({ useOwnerNotificationSettings: () => hook.current }),
);

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const POLICY: OwnerNotificationSettingsView = {
  __typename: "OwnerNotificationSettings",
  ownerId: "tea-1",
  emailEnabled: true,
  notificationsToSend: "failure",
  slackEnabled: false,
  previewNotificationsEnabled: false,
};

function setup(
  settings: OwnerNotificationSettingsView | null = POLICY,
  loading = false,
  update = vi.fn(async (input) => ({ ...POLICY, ...input })),
) {
  hook.current = {
    settings,
    loading,
    error: undefined,
    refetch: vi.fn(async () => undefined),
    update,
    saving: false,
  };
  render(<WorkspaceNotificationsCard ownerId="tea-1" />);
  return { update, user: userEvent.setup() };
}

const radio = (name: RegExp) => screen.getByRole("radio", { name });
const save = () => screen.getByRole("button", { name: "Save" });
const emailSwitch = () =>
  screen.getByRole("switch", { name: "Email notifications" });

describe("WorkspaceNotificationsCard", () => {
  beforeEach(() => {
    vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
    vi.mocked(toast.success).mockClear();
  });

  it("renders the skeleton while loading", () => {
    setup(null, true);
    expect(
      screen.getByTestId("workspace-notifications-skeleton"),
    ).toBeInTheDocument();
  });

  it("previews the ready card's regions in the skeleton", () => {
    setup(null, true);
    const skeleton = screen.getByTestId("workspace-notifications-skeleton");
    // One email row and three level options, on the ready card's frames.
    expect(
      skeleton.querySelectorAll(".min-h-16.rounded-md.border"),
    ).toHaveLength(1);
    expect(
      skeleton.querySelectorAll(".min-h-20.rounded-lg.border"),
    ).toHaveLength(3);
    expect(skeleton.querySelector('[data-slot="card-footer"]')).not.toBeNull();
  });

  it("puts the email row and options on the same frames as the skeleton", () => {
    setup();
    expect(emailSwitch().parentElement).toHaveClass(
      "min-h-16",
      "rounded-md",
      "border",
    );
    for (const option of screen.getAllByRole("radio")) {
      expect(option).toHaveClass("min-h-20", "rounded-lg", "border");
    }
    expect(screen.getAllByRole("radio")).toHaveLength(3);
  });

  it("shows the saved policy with Save disabled until something changes", () => {
    setup();
    expect(emailSwitch()).toBeChecked();
    expect(radio(/^Only failure notifications/)).toHaveAttribute(
      "aria-checked",
      "true",
    );
    expect(radio(/^All notifications/)).toHaveAttribute(
      "aria-checked",
      "false",
    );
    expect(save()).toBeDisabled();
  });

  it("saves only the changed fields as a partial update", async () => {
    const { update, user } = setup();
    await user.click(radio(/^All notifications/));
    await user.click(save());
    expect(update).toHaveBeenCalledWith({ notificationsToSend: "all" });
    expect(toast.success).toHaveBeenCalledWith("Notification defaults saved");

    await user.click(emailSwitch());
    await user.click(save());
    expect(update).toHaveBeenLastCalledWith({ emailEnabled: false });
  });

  it("is read-only for a member who cannot manage the workspace", async () => {
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ canManage: false }),
    );
    const { update, user } = setup();
    expect(
      screen.getByText("Only workspace admins can change these defaults."),
    ).toBeInTheDocument();
    expect(emailSwitch()).toBeDisabled();
    expect(radio(/^None/)).toBeDisabled();
    await user.click(radio(/^None/));
    expect(save()).toBeDisabled();
    expect(update).not.toHaveBeenCalled();
  });

  it("keeps the edit and reports a failed save", async () => {
    const update = vi.fn(async () => {
      throw new Error("forbidden");
    });
    const { user } = setup(POLICY, false, update);
    await user.click(radio(/^None/));
    await user.click(save());
    expect(
      screen.getByText("Couldn't save the notification defaults. Try again."),
    ).toBeInTheDocument();
    expect(radio(/^None/)).toHaveAttribute("aria-checked", "true");
    expect(save()).toBeEnabled();
  });

  it("offers a retry when the policy cannot be read", async () => {
    const { user } = setup(null, false);
    expect(
      screen.getByText("Couldn't load the workspace notification defaults."),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(hook.current.refetch).toHaveBeenCalled();
  });
});
