import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ServiceNotificationsRow } from "../service-notifications-row";

const setNotificationsToSend = vi.fn(async () => true);
vi.mock("@/features/services/hooks/use-service-notifications", () => ({
  useServiceNotifications: () => ({ setNotificationsToSend, busy: false }),
}));

beforeEach(() => setNotificationsToSend.mockClear());

describe("ServiceNotificationsRow", () => {
  it("renders the current policy in a disabled select with a pencil", () => {
    render(
      <ServiceNotificationsRow serviceId="srv-1" notificationsToSend={null} />,
    );
    const trigger = screen.getByRole("combobox", {
      name: "Service Notifications",
    });
    expect(trigger).toBeDisabled();
    expect(trigger).toHaveTextContent(
      "Use workspace default (Only failure notifications)",
    );
    expect(
      screen.queryByRole("button", { name: "Save changes" }),
    ).not.toBeInTheDocument();
  });

  it("edits and persists an explicit override via the select variant", async () => {
    const user = userEvent.setup();
    render(
      <ServiceNotificationsRow
        serviceId="srv-1"
        notificationsToSend="default"
      />,
    );

    await user.click(
      screen.getByRole("button", { name: "Edit service notifications" }),
    );
    await user.click(
      screen.getByRole("combobox", { name: "Service Notifications" }),
    );
    await user.click(
      screen.getByRole("option", { name: "Only failure notifications" }),
    );
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(setNotificationsToSend).toHaveBeenCalledWith("srv-1", "failure");
  });
});
