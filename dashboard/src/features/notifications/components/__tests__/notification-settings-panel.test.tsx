import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NotificationSettingsPanel } from "@/features/notifications/components/notification-settings-panel";

const settingsState: {
  settings: { deploySucceeded: boolean; deployFailed: boolean };
  loading: boolean;
  error: Error | undefined;
} = { settings: { deploySucceeded: true, deployFailed: true }, loading: false, error: undefined };
vi.mock("@/features/notifications/hooks/use-notification-settings", () => ({
  useNotificationSettings: () => settingsState,
}));

const update = vi.fn();
vi.mock("@/features/notifications/hooks/use-update-notification-settings", () => ({
  useUpdateNotificationSettings: () => ({ update, busy: false }),
}));

beforeEach(() => {
  settingsState.settings = { deploySucceeded: true, deployFailed: true };
  settingsState.loading = false;
  settingsState.error = undefined;
  update.mockReset();
  update.mockResolvedValue(true);
});

describe("NotificationSettingsPanel", () => {
  it("reflects the current preferences in the two switches", () => {
    settingsState.settings = { deploySucceeded: false, deployFailed: true };
    render(<NotificationSettingsPanel />);

    const switches = screen.getAllByRole("switch");
    expect(switches[0]).toHaveAttribute("aria-checked", "false"); // deploySucceeded
    expect(switches[1]).toHaveAttribute("aria-checked", "true"); // deployFailed
  });

  it("toggling a switch calls update with the flipped field and the other unchanged", async () => {
    const user = userEvent.setup();
    render(<NotificationSettingsPanel />);

    const switches = screen.getAllByRole("switch");
    await user.click(switches[0]); // flip deploySucceeded true -> false

    expect(update).toHaveBeenCalledWith({ deploySucceeded: false, deployFailed: true });
  });

  it("toggling the other switch flips deployFailed and leaves deploySucceeded unchanged", async () => {
    const user = userEvent.setup();
    render(<NotificationSettingsPanel />);

    await user.click(screen.getAllByRole("switch")[1]); // flip deployFailed true -> false

    expect(update).toHaveBeenCalledWith({ deploySucceeded: true, deployFailed: false });
  });

  it("shows an error state and no switches when the query fails", () => {
    settingsState.error = new Error("boom");
    render(<NotificationSettingsPanel />);

    expect(screen.getByText("Couldn't load notification settings")).toBeInTheDocument();
    expect(screen.queryAllByRole("switch")).toHaveLength(0);
  });
});
