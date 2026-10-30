import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { WebPushSettingsPanel } from "@/features/notifications/components/web-push-settings-panel";
import type { WebPushAction } from "@/features/notifications/hooks/use-web-push-subscription";

const queryState = {
  webPushAvailable: true,
  vapidPublicKey: "BKAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
  loading: false,
};

vi.mock(
  "@/features/notifications/hooks/use-push-notification-settings",
  () => ({
    usePushNotificationSettings: () => queryState,
  }),
);

const webPush = {
  status: "prompt" as
    | "unsupported"
    | "unconfigured"
    | "denied"
    | "prompt"
    | "subscribed"
    | "busy"
    | "error",
  failedAction: null as WebPushAction | null,
  subscribe: vi.fn(),
  unsubscribe: vi.fn(),
};

vi.mock("@/features/notifications/hooks/use-web-push-subscription", () => ({
  useWebPushSubscription: () => webPush,
}));

beforeEach(() => {
  queryState.webPushAvailable = true;
  queryState.vapidPublicKey = "BKAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA";
  queryState.loading = false;
  webPush.status = "prompt";
  webPush.failedAction = null;
  webPush.subscribe.mockReset();
  webPush.unsubscribe.mockReset();
  webPush.subscribe.mockResolvedValue(true);
  webPush.unsubscribe.mockResolvedValue(true);
});

describe("WebPushSettingsPanel", () => {
  it("shows unsupported copy", () => {
    webPush.status = "unsupported";
    render(<WebPushSettingsPanel />);
    expect(
      screen.getByText("This browser does not support web push."),
    ).toBeInTheDocument();
  });

  it("shows unconfigured copy when the server has no VAPID transport", () => {
    webPush.status = "unconfigured";
    render(<WebPushSettingsPanel />);
    expect(
      screen.getByText("This bex server has not enabled browser push."),
    ).toBeInTheDocument();
  });

  it("shows denied copy with a recovery hint", () => {
    webPush.status = "denied";
    render(<WebPushSettingsPanel />);
    expect(screen.getByText(/Notifications are blocked/)).toBeInTheDocument();
  });

  it("subscribes from the enable button", async () => {
    const user = userEvent.setup();
    render(<WebPushSettingsPanel />);
    await user.click(
      screen.getByRole("button", { name: /Enable in this browser/ }),
    );
    expect(webPush.subscribe).toHaveBeenCalledTimes(1);
  });

  it("unsubscribes from the disable button", async () => {
    webPush.status = "subscribed";
    const user = userEvent.setup();
    render(<WebPushSettingsPanel />);
    await user.click(
      screen.getByRole("button", { name: /Disable in this browser/ }),
    );
    expect(webPush.unsubscribe).toHaveBeenCalledTimes(1);
  });

  it("offers the failed disable again instead of a dead end", async () => {
    webPush.status = "error";
    webPush.failedAction = "unsubscribe";
    render(<WebPushSettingsPanel />);
    expect(screen.getByRole("alert")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /disable/i }));
    expect(webPush.unsubscribe).toHaveBeenCalledTimes(1);
  });

  it("offers the failed enable again", async () => {
    webPush.status = "error";
    webPush.failedAction = "subscribe";
    render(<WebPushSettingsPanel />);
    await userEvent.click(screen.getByRole("button", { name: /enable/i }));
    expect(webPush.subscribe).toHaveBeenCalledTimes(1);
  });
});
