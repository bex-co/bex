import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  PushNotificationEvent,
  PushNotificationUrgency,
  type PushNotificationSettingsInput,
} from "@/graphql/definitions";
import { PushNotificationSettingsPanel } from "@/features/notifications/components/push-notification-settings-panel";

// The panel guards unsaved push edits with the router's blocker (w4/435). The
// suite renders without a router, so capture the latest blocker options.
const blocker = vi.hoisted(() => ({
  options: null as null | {
    shouldBlockFn: () => boolean;
    enableBeforeUnload: boolean;
  },
}));
vi.mock("@tanstack/react-router", () => ({
  useBlocker: (options: typeof blocker.options) => {
    blocker.options = options;
    return { status: "idle" };
  },
}));
import {
  publishedPushEvents,
  type PushEventUrgencies,
} from "@/features/notifications/lib/push-policy";

// bex-api's published table (notifications.pushEventUrgency, w4/m231): four
// Routine events; everything else Important or Critical. Nothing sends
// agent_needs_decision yet (w4/349).
const { Routine, Important, Critical } = PushNotificationUrgency;
const publishedUrgencies: PushEventUrgencies = publishedPushEvents(
  (
    [
      [PushNotificationEvent.DeployStarted, Routine],
      [PushNotificationEvent.DeploySucceeded, Routine],
      [PushNotificationEvent.DeployFailed, Important],
      [PushNotificationEvent.ServerFailed, Critical],
      [PushNotificationEvent.ServerAvailable, Important],
      [PushNotificationEvent.ServiceSuspended, Routine],
      [PushNotificationEvent.ServiceResumed, Routine],
      [PushNotificationEvent.CronFailed, Important],
      [PushNotificationEvent.AgentNeedsDecision, Important],
      [PushNotificationEvent.AgentPrReady, Important],
      [PushNotificationEvent.AgentFailed, Important],
      [PushNotificationEvent.PostgresUnavailable, Critical],
      [PushNotificationEvent.PostgresAvailable, Important],
      [PushNotificationEvent.KeyValueUnhealthy, Critical],
      [PushNotificationEvent.KeyValueAvailable, Important],
      [PushNotificationEvent.PostgresBackupFailed, Important],
      [PushNotificationEvent.PostgresRestoreFailed, Important],
      [PushNotificationEvent.PostgresUpgradeFailed, Important],
    ] as const
  ).map(([event, urgency]) => ({
    event,
    urgency,
    sent: event !== PushNotificationEvent.AgentNeedsDecision,
  })),
);
const SERVICE_ID = "srv-c185th5c2rvvnhbfiltg";

const state: {
  settings: PushNotificationSettingsInput;
  eventUrgencies: PushEventUrgencies;
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
  eventUrgencies: publishedUrgencies,
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
  useServices: () => ({
    services: [{ id: SERVICE_ID, name: "api" }],
    loading: false,
    error: undefined,
  }),
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
  state.eventUrgencies = publishedUrgencies;
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
    // An edit arms Save (w4/435); the missing provider never disables it.
    await user.click(screen.getByText("Deploy failed"));
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

  it("arms Save, the hint and the navigation guard only while push edits are unsaved (w4/435)", async () => {
    const user = userEvent.setup();
    render(<PushNotificationSettingsPanel />);
    const save = screen.getByRole("button", { name: "Save push settings" });

    expect(save).toBeDisabled();
    expect(screen.queryByText("Unsaved changes")).toBeNull();
    expect(blocker.options?.shouldBlockFn()).toBe(false);
    expect(blocker.options?.enableBeforeUnload).toBe(false);

    await user.click(screen.getByText("Deploy failed"));
    expect(save).toBeEnabled();
    expect(screen.getByText("Unsaved changes")).toBeInTheDocument();
    expect(blocker.options?.shouldBlockFn()).toBe(true);
    expect(blocker.options?.enableBeforeUnload).toBe(true);

    await user.click(screen.getByText("Deploy failed"));
    expect(save).toBeDisabled();
    expect(blocker.options?.shouldBlockFn()).toBe(false);
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
    // Edit an unrelated workspace filter so Save is armed (w4/435).
    await user.click(screen.getAllByText("Deploy failed")[0]);
    await user.click(
      screen.getByRole("button", { name: "Save push settings" }),
    );

    const saved = update.mock.calls[0][0] as PushNotificationSettingsInput;
    expect(saved.serviceOverrides[0].events).toEqual([]);
  });

  // w4/m231: bex-api drops a selected event whose urgency is below the
  // minimum, so the panel must say so instead of promising a push.
  describe("urgency filter", () => {
    const save = () =>
      userEvent
        .setup()
        .click(screen.getByRole("button", { name: "Save push settings" }));
    // A row of the workspace-wide filter (an override's exact list repeats
    // the event names).
    const eventRow = (name: string) =>
      within(screen.getByRole("group", { name: "Event filters" }))
        .getByText(name)
        .closest("label")!;
    const summary = () =>
      screen.queryByText("Some selected events won't be pushed");

    it("labels every event with the urgency it is delivered at", () => {
      render(<PushNotificationSettingsPanel />);

      expect(eventRow("Deploy succeeded")).toHaveTextContent("Routine");
      expect(eventRow("Deploy failed")).toHaveTextContent("Important");
      expect(eventRow("Service crashed")).toHaveTextContent("Critical");
    });

    // w4/349: an event nothing sends yet is labeled, not hidden, and a
    // selection of it is kept.
    it("labels an event nothing sends yet and keeps a selection of it", async () => {
      const user = userEvent.setup();
      render(<PushNotificationSettingsPanel />);

      expect(eventRow("Agent needs a decision")).toHaveTextContent(
        "Not sent yet: nothing sends this event today.",
      );
      expect(eventRow("Deploy failed")).not.toHaveTextContent("Not sent yet");
      const checkbox = within(eventRow("Agent needs a decision")).getByRole(
        "checkbox",
      );
      expect(checkbox).toHaveAccessibleName("Agent needs a decision Important");
      expect(checkbox).toHaveAccessibleDescription(
        "Not sent yet: nothing sends this event today.",
      );

      await user.click(eventRow("Agent needs a decision"));
      await user.click(
        screen.getByRole("button", { name: "Save push settings" }),
      );
      expect(update.mock.lastCall?.[0].events).toContain(
        PushNotificationEvent.AgentNeedsDecision,
      );
    });

    it("flags a Routine selection under Important, saves it as chosen, and clears at Routine", async () => {
      const user = userEvent.setup();
      render(<PushNotificationSettingsPanel />);
      expect(summary()).not.toBeInTheDocument();

      await user.click(eventRow("Deploy succeeded"));

      expect(eventRow("Deploy succeeded")).toHaveTextContent(
        "Not pushed while Minimum urgency is Important.",
      );
      expect(summary()).toBeInTheDocument();
      expect(
        screen.getByText(
          "Deploy succeeded is Routine and won't be pushed while Minimum urgency is Important.",
        ),
      ).toBeInTheDocument();
      expect(eventRow("Deploy failed")).not.toHaveTextContent("Not pushed");

      await save();
      const saved = update.mock.calls[0][0] as PushNotificationSettingsInput;
      expect(saved.events).toEqual([
        PushNotificationEvent.DeployFailed,
        PushNotificationEvent.AgentPrReady,
        PushNotificationEvent.DeploySucceeded,
      ]);
      expect(saved.minimumUrgency).toBe(Important);

      await user.selectOptions(
        screen.getByLabelText("Minimum urgency"),
        Routine,
      );
      expect(summary()).not.toBeInTheDocument();
      expect(eventRow("Deploy succeeded")).not.toHaveTextContent("Not pushed");
    });

    it("flags nothing while push is off, even under an enabled override", () => {
      state.settings = {
        ...state.settings,
        enabled: false,
        events: [PushNotificationEvent.DeploySucceeded],
        serviceOverrides: [
          {
            serviceId: SERVICE_ID,
            enabled: true,
            events: [PushNotificationEvent.ServiceResumed],
          },
        ],
      };
      render(<PushNotificationSettingsPanel />);

      expect(summary()).not.toBeInTheDocument();
      expect(eventRow("Deploy succeeded")).not.toHaveTextContent("Not pushed");
    });

    it("shows a service override's own drop for that service only, and clears it at Routine", async () => {
      state.settings = {
        ...state.settings,
        serviceOverrides: [
          {
            serviceId: SERVICE_ID,
            events: [PushNotificationEvent.ServiceResumed],
          },
        ],
      };
      const user = userEvent.setup();
      render(<PushNotificationSettingsPanel />);

      const card = screen.getByRole("group", { name: "api" });
      expect(card).toHaveTextContent(
        "1 selected event won't be pushed for this service while its Minimum urgency is Important.",
      );
      expect(
        screen.getByText(
          "api: Service resumed is Routine and won't be pushed while this service's Minimum urgency is Important.",
        ),
      ).toBeInTheDocument();
      // Only the override drops it: the workspace filter flags nothing, and
      // the summary carries just that service's line.
      expect(eventRow("Service resumed")).not.toHaveTextContent("Not pushed");
      expect(
        within(screen.getByRole("status")).getAllByRole("listitem"),
      ).toHaveLength(1);

      const overrideUrgency = within(card).getByLabelText("Minimum urgency");
      await user.selectOptions(overrideUrgency, Routine);
      expect(card).not.toHaveTextContent("won't be pushed");
      expect(summary()).not.toBeInTheDocument();
    });
  });
});

// w4/410: the push card sits under the page h1 as a section heading.
describe("PushNotificationSettingsPanel heading", () => {
  it("renders its title as an h2", async () => {
    render(<PushNotificationSettingsPanel />);
    expect(
      await screen.findByRole("heading", { level: 2, name: "Mobile push" }),
    ).toBeInTheDocument();
  });
});
