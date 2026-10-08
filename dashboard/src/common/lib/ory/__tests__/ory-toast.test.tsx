import { describe, it, expect, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { IntlProvider } from "react-intl";
import { toast } from "sonner";
import { OryLocales } from "@ory/elements-react";
import { OryToast } from "../ory-toast";

vi.mock("sonner", () => ({ toast: { dismiss: vi.fn() } }));

const message = {
  id: 1060001,
  type: "success" as const,
  text: "You successfully recovered your account.",
};

function renderToast() {
  return render(
    <IntlProvider locale="en" defaultLocale="en" messages={OryLocales.en}>
      <OryToast id="t1" message={message} />
    </IntlProvider>,
  );
}

describe("OryToast", () => {
  // Ory's DefaultToast relies on `.ory-elements`-scoped classes that never
  // match inside the app-wide Toaster portal, leaving it unthemed (light box,
  // unreadable text) in dark mode. The override must use the app's tokens.
  it("renders with the app's theme-aware popover tokens", () => {
    renderToast();
    const root = screen.getByTestId("ory/message/1060001");
    expect(root).toHaveClass("bg-popover", "text-popover-foreground");
    expect(screen.getByText("Settings updated")).toBeInTheDocument();
    expect(
      screen.getByText(/You successfully recovered your account/),
    ).toBeInTheDocument();
  });

  it("dismisses its own toast when closed", () => {
    renderToast();
    fireEvent.click(screen.getByTestId("ory/message/1060001.close"));
    expect(toast.dismiss).toHaveBeenCalledWith("t1");
  });
});
