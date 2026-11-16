import { describe, expect, it, vi } from "vitest";
import { render } from "@testing-library/react";
import {
  PushNotificationEvent,
  PushNotificationUrgency,
} from "@/graphql/definitions";
import { NotificationsPageSkeleton } from "@/common/components/route-skeletons";
import { NotificationSettingsPanel } from "@/features/notifications/components/notification-settings-panel";
import { PushNotificationSettingsPanel } from "@/features/notifications/components/push-notification-settings-panel";
import { WebPushSettingsPanel } from "@/features/notifications/components/web-push-settings-panel";
import {
  publishedPushEvents,
  pushEvents,
} from "@/features/notifications/lib/push-policy";

// The ready panels, rendered from settled data: the pending frame has to
// preview each one's regions at the same heights (w4/680).
vi.mock("@tanstack/react-router", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  useBlocker: () => ({ status: "idle" }),
}));

vi.mock("@/features/notifications/hooks/use-notification-settings", () => ({
  useNotificationSettings: () => ({
    settings: {
      deployStarted: true,
      deploySucceeded: true,
      deployFailed: true,
    },
    loading: false,
    error: undefined,
  }),
}));
vi.mock(
  "@/features/notifications/hooks/use-update-notification-settings",
  () => ({ useUpdateNotificationSettings: () => ({ update: vi.fn() }) }),
);
vi.mock(
  "@/features/notifications/hooks/use-update-push-notification-settings",
  () => ({ useUpdatePushNotificationSettings: () => ({ update: vi.fn() }) }),
);
vi.mock(
  "@/features/notifications/hooks/use-push-notification-settings",
  async (importOriginal) => {
    const original =
      await importOriginal<
        typeof import("@/features/notifications/hooks/use-push-notification-settings")
      >();
    return {
      ...original,
      usePushNotificationSettings: () => ({
        settings: original.defaultPushNotificationSettings,
        // bex-api's table: nothing sends agent_needs_decision yet (w4/349).
        eventUrgencies: publishedPushEvents(
          pushEvents.map((event) => ({
            event,
            urgency: PushNotificationUrgency.Important,
            sent: event !== PushNotificationEvent.AgentNeedsDecision,
          })),
        ),
        available: true,
        loading: false,
        error: undefined,
        webPushAvailable: true,
        vapidPublicKey: "BKAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
      }),
    };
  },
);
vi.mock("@/features/services/hooks/use-services", () => ({
  useServices: () => ({
    services: [{ id: "srv-c185th5c2rvvnhbfiltg", name: "api" }],
    loading: false,
    error: undefined,
  }),
}));
vi.mock("@/features/notifications/hooks/use-web-push-subscription", () => ({
  useWebPushSubscription: () => ({
    status: "prompt",
    failedAction: null,
    subscribe: vi.fn(),
    unsubscribe: vi.fn(),
  }),
}));

/**
 * The geometry-bearing frames of a card: switch rows, event options,
 * fieldsets, 36px controls and buttons. A skeleton bar standing in for a
 * control counts as that control.
 */
function frames(root: Element) {
  const count = (selector: string) => root.querySelectorAll(selector).length;
  return {
    switchRows: count(".rounded-md.border.p-3"),
    eventOptions: count(".rounded-md.border.p-2"),
    fieldsets: count("fieldset"),
    controls:
      count("select, input:not([type='checkbox'])") +
      count("[data-slot='skeleton'].h-9.w-full"),
    switches: count("[role='switch'], [data-slot='skeleton'].h-5.w-9"),
    buttons:
      count("button:not([role='checkbox']):not([role='switch'])") +
      count("span.relative.inline-flex > span.invisible"),
  };
}

function region(name: string) {
  const { container } = render(<NotificationsPageSkeleton />);
  const element = container.querySelector(`[data-skeleton-region="${name}"]`);
  expect(element).not.toBeNull();
  return element!;
}

describe("NotificationsPageSkeleton (w4/680)", () => {
  it.each([
    ["email-notifications", <NotificationSettingsPanel key="email" />],
    ["push-notifications", <PushNotificationSettingsPanel key="push" />],
    ["web-push", <WebPushSettingsPanel key="web" />],
  ])("previews the ready %s card's frames", (name, panel) => {
    const { container, unmount } = render(panel);
    const ready = frames(container);
    expect(ready.buttons + ready.switches).toBeGreaterThan(0);
    expect(frames(region(name))).toEqual(ready);
    unmount();
  });

  it("reserves every selectable event's own label, and the unsent note", () => {
    const push = region("push-notifications");
    const { container, unmount } = render(<PushNotificationSettingsPanel />);
    const ready = [...container.querySelectorAll(".rounded-md.border.p-2")];
    const options = [...push.querySelectorAll("[data-push-event-skeleton]")];
    expect(options).toHaveLength(pushEvents.length);
    expect(ready).toHaveLength(pushEvents.length);
    // Each option holds the ready option's lines: the event name, and the
    // not-sent note where the ready option has one.
    options.forEach((option, index) => {
      expect(option.textContent).toContain(
        ready[index].querySelector("[id$='-name']")!.textContent,
      );
      expect(option.querySelectorAll(".block.text-xs")).toHaveLength(
        ready[index].querySelectorAll(".block.text-xs").length,
      );
    });
    unmount();
  });
});
