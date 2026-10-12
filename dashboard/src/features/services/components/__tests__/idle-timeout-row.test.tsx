import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { IdleTimeoutRow } from "@/features/services/components/idle-timeout-row";

const setIdleTimeout = vi.fn(async () => true);

vi.mock("@/features/services/hooks/use-idle-timeout", () => ({
  useIdleTimeout: () => ({ setIdleTimeout, busy: false }),
}));

beforeEach(() => {
  setIdleTimeout.mockClear();
});

describe("IdleTimeoutRow", () => {
  it("shows an always-on notice (no control) for a paid plan", () => {
    render(
      <IdleTimeoutRow
        serviceId="app"
        serviceType="web_service"
        plan="pro_plus"
        idleTTLSeconds={0}
      />,
    );
    expect(
      screen.getByText("Paid services stay always-on and never sleep."),
    ).toBeInTheDocument();
    // no select control on a paid plan
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
  });

  it("shows the idle-timeout control for a free plan, reflecting the current window", () => {
    render(
      <IdleTimeoutRow
        serviceId="app"
        serviceType="web_service"
        plan="free"
        idleTTLSeconds={900}
      />,
    );
    const control = screen.getByRole("combobox", { name: "Idle timeout" });
    expect(control).toBeInTheDocument();
    // the current value renders as its human label (15 min), not raw seconds
    expect(control).toHaveTextContent("15 min");
  });

  // Only public traffic reaches the activator (ADR003); internal calls from a
  // sibling neither count as activity nor wake the service (w4/252).
  it("says only public requests keep a free service awake or wake it", () => {
    render(
      <IdleTimeoutRow
        serviceId="app"
        serviceType="web_service"
        plan="free"
        idleTTLSeconds={0}
      />,
    );
    expect(
      screen.getByText(
        "Free services sleep after this long without public HTTP requests or WebSocket messages, then wake on the next public request. Internal requests from other services neither keep them awake nor wake them.",
      ),
    ).toBeInTheDocument();
  });

  it("treats an untiered App (null plan) as free — the control is shown", () => {
    render(
      <IdleTimeoutRow
        serviceId="app"
        serviceType="web_service"
        plan={null}
        idleTTLSeconds={0}
      />,
    );
    expect(
      screen.getByRole("combobox", { name: "Idle timeout" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("combobox", { name: "Idle timeout" }),
    ).toHaveTextContent("Platform default");
  });

  it.each(["private_service", "background_worker", "cron_job", "static_site"])(
    "does not render for %s",
    (serviceType) => {
      const { container } = render(
        <IdleTimeoutRow
          serviceId="app"
          serviceType={serviceType}
          plan="free"
          idleTTLSeconds={60}
        />,
      );
      expect(container).toBeEmptyDOMElement();
    },
  );
});
