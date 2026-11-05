import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ServiceNotificationsRow } from "../service-notifications-row";

const setNotificationsToSend = vi.fn(async () => true);
vi.mock("@/features/services/hooks/use-service-notifications", () => ({
  useServiceNotifications: () => ({ setNotificationsToSend, busy: false }),
}));

const owner = vi.hoisted(() => ({
  level: "failure" as string | null,
  ownerIds: [] as Array<string | null | undefined>,
}));
vi.mock(
  "@/features/notifications/hooks/use-owner-notification-settings",
  () => ({
    useOwnerNotificationSettings: (ownerId: string | null | undefined) => {
      owner.ownerIds.push(ownerId);
      return {
        settings:
          owner.level == null
            ? null
            : {
                __typename: "OwnerNotificationSettings",
                ownerId: ownerId ?? "",
                emailEnabled: true,
                notificationsToSend: owner.level,
                slackEnabled: false,
                previewNotificationsEnabled: false,
              },
        loading: owner.level == null,
      };
    },
  }),
);

beforeEach(() => {
  setNotificationsToSend.mockClear();
  owner.level = "failure";
  owner.ownerIds = [];
});

const trigger = () =>
  screen.getByRole("combobox", { name: "Service Notifications" });

describe("ServiceNotificationsRow", () => {
  it("renders the current policy in a disabled select with a pencil", () => {
    render(
      <ServiceNotificationsRow
        serviceId="srv-1"
        ownerId="tea-1"
        notificationsToSend={null}
      />,
    );
    expect(trigger()).toBeDisabled();
    expect(trigger()).toHaveTextContent(
      "Use workspace default (Only failure notifications)",
    );
    expect(owner.ownerIds).toContain("tea-1");
    expect(
      screen.queryByRole("button", { name: "Save changes" }),
    ).not.toBeInTheDocument();
  });

  it("names the workspace's saved level, not a hard-coded one (w3/m271)", () => {
    owner.level = "all";
    const { rerender } = render(
      <ServiceNotificationsRow
        serviceId="srv-1"
        ownerId="tea-1"
        notificationsToSend="default"
      />,
    );
    expect(trigger()).toHaveTextContent(
      "Use workspace default (All notifications)",
    );

    owner.level = "none";
    rerender(
      <ServiceNotificationsRow
        serviceId="srv-1"
        ownerId="tea-1"
        notificationsToSend="default"
      />,
    );
    expect(trigger()).toHaveTextContent("Use workspace default (None)");
  });

  it("claims no level while the workspace policy is unknown", () => {
    owner.level = null;
    render(
      <ServiceNotificationsRow
        serviceId="srv-1"
        ownerId="tea-1"
        notificationsToSend={null}
      />,
    );
    expect(trigger()).toHaveTextContent(/^Use workspace default$/);
  });

  it("edits and persists an explicit override via the select variant", async () => {
    const user = userEvent.setup();
    render(
      <ServiceNotificationsRow
        serviceId="srv-1"
        ownerId="tea-1"
        notificationsToSend="default"
      />,
    );

    await user.click(
      screen.getByRole("button", { name: "Edit service notifications" }),
    );
    await user.click(trigger());
    await user.click(
      screen.getByRole("option", { name: "Only failure notifications" }),
    );
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(setNotificationsToSend).toHaveBeenCalledWith("srv-1", "failure");
  });
});
