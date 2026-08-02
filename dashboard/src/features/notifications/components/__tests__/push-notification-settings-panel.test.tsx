import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  PushNotificationEvent,
  PushNotificationUrgency,
  type PushNotificationSettingsInput,
} from "@/graphql/definitions";
import { PushNotificationSettingsPanel } from "@/features/notifications/components/push-notification-settings-panel";

const state: {
  settings: PushNotificationSettingsInput;
  available: boolean;
  loading: boolean;
  error: Error | undefined;
} = {
  settings: {
    enabled: true,
    events: [
      PushNotificationEvent.DeployFailed,
      PushNotificationEvent.AgentPrReady,
    ],
    minimumUrgency: PushNotificationUrgency.Important,
    timeZone: "UTC",
    workingHours: [],
    quietHours: [],
    maxDeferralSeconds: 28_800,
    serviceOverrides: [],
  },
  available: true,
  loading: false,
  error: undefined,
};

vi.mock(
  "@/features/notifications/hooks/use-push-notification-settings",
  async (importOriginal) => {
    const original =
      await importOriginal<
        typeof import("@/features/notifications/hooks/use-push-notification-settings")
      >();
    return {
      ...original,
      usePushNotificationSettings: () => state,
    };
  },
);

const update = vi.fn();
vi.mock(
  "@/features/notifications/hooks/use-update-push-notification-settings",
  () => ({
    useUpdatePushNotificationSettings: () => ({ update, busy: false }),
  }),
);

vi.mock("@/features/services/hooks/use-services", () => ({
  useServices: () => ({ services: [], loading: false, error: undefined }),
}));

beforeEach(() => {
  state.settings = {
    enabled: true,
    events: [
      PushNotificationEvent.DeployFailed,
      PushNotificationEvent.AgentPrReady,
    ],
    minimumUrgency: PushNotificationUrgency.Important,
    timeZone: "UTC",
    workingHours: [],
    quietHours: [],
    maxDeferralSeconds: 28_800,
    serviceOverrides: [],
  };
  state.loading = false;
  state.available = true;
  state.error = undefined;
  update.mockReset();
  update.mockResolvedValue(true);
});

describe("PushNotificationSettingsPanel", () => {
  it("states when the server has no delivery provider without disabling policy edits", async () => {
    state.available = false;
    const user = userEvent.setup();
    render(<PushNotificationSettingsPanel />);

    expect(screen.getByRole("status")).toHaveTextContent(
      "Push delivery is not configured",
    );
    await user.click(
      screen.getByRole("button", { name: "Save push settings" }),
    );
    expect(update).toHaveBeenCalledTimes(1);
  });

  it("blocks an invalid timezone locally and does not mutate", async () => {
    const user = userEvent.setup();
    render(<PushNotificationSettingsPanel />);

    const timezone = screen.getByLabelText("IANA timezone");
    await user.clear(timezone);
    await user.type(timezone, "Mars/Olympus");
    await user.click(
      screen.getByRole("button", { name: "Save push settings" }),
    );

    expect(update).not.toHaveBeenCalled();
    expect(screen.getByRole("alert")).toHaveTextContent("valid IANA timezone");
  });

  it("preserves returned closed events while editing another filter", async () => {
    const user = userEvent.setup();
    render(<PushNotificationSettingsPanel />);

    await user.click(screen.getByText("Deploy failed"));
    await user.click(
      screen.getByRole("button", { name: "Save push settings" }),
    );

    expect(update).toHaveBeenCalledTimes(1);
    const saved = update.mock.calls[0][0] as PushNotificationSettingsInput;
    expect(saved.events).not.toContain(PushNotificationEvent.DeployFailed);
    expect(saved.events).toContain(PushNotificationEvent.AgentPrReady);
  });

  it("retains an explicit empty per-service event filter", async () => {
    state.settings = {
      ...state.settings,
      serviceOverrides: [{ serviceId: "srv-c185th5c2rvvnhbfiltg", events: [] }],
    };
    const user = userEvent.setup();
    render(<PushNotificationSettingsPanel />);

    expect(
      screen.getByRole("checkbox", {
        name: "Override with an exact event list",
      }),
    ).toBeChecked();
    await user.click(
      screen.getByRole("button", { name: "Save push settings" }),
    );

    const saved = update.mock.calls[0][0] as PushNotificationSettingsInput;
    expect(saved.serviceOverrides[0].events).toEqual([]);
  });
});
