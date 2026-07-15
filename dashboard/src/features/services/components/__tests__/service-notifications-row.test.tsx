import type { ReactNode } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ServiceNotificationsRow } from "../service-notifications-row";

const setNotificationsToSend = vi.fn(async () => true);
vi.mock("@/features/services/hooks/use-service-notifications", () => ({
  useServiceNotifications: () => ({ setNotificationsToSend, busy: false }),
}));
vi.mock("@/common/hooks/use-translations", () => ({
  useTranslations: () => ({ t: (key: string) => key }),
}));
vi.mock("@/common/components/ui/select", () => ({
  Select: ({
    value,
    onValueChange,
    children,
  }: {
    value: string;
    onValueChange: (value: string) => void;
    children: ReactNode;
  }) => (
    <select
      aria-label="policy"
      value={value}
      onChange={(event) => onValueChange(event.target.value)}
    >
      {children}
    </select>
  ),
  SelectContent: ({ children }: { children: ReactNode }) => <>{children}</>,
  SelectItem: ({ value, children }: { value: string; children: ReactNode }) => (
    <option value={value}>{children}</option>
  ),
  SelectTrigger: ({ children }: { children: ReactNode }) => <>{children}</>,
  SelectValue: () => null,
}));

describe("ServiceNotificationsRow", () => {
  beforeEach(() => setNotificationsToSend.mockClear());
  it("defaults to the workspace policy and exposes all choices", () => {
    render(
      <ServiceNotificationsRow serviceId="srv-1" notificationsToSend={null} />,
    );
    expect(screen.getByLabelText("policy")).toHaveValue("default");
    for (const key of ["Default", "All", "Failure", "None"])
      expect(
        screen.getByText(`services.notificationsOption${key}`),
      ).toBeInTheDocument();
  });
  it("persists an explicit override", () => {
    render(
      <ServiceNotificationsRow
        serviceId="srv-1"
        notificationsToSend="default"
      />,
    );
    fireEvent.change(screen.getByLabelText("policy"), {
      target: { value: "failure" },
    });
    expect(setNotificationsToSend).toHaveBeenCalledWith("srv-1", "failure");
  });
});
