import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { useIntl } from "react-intl";
import { toast } from "sonner";
import { RootProvider } from "@/common/providers/root-provider";

// The WorkspaceProvider pulls in Apollo/GraphQL; stub it out — this suite is
// about the intl context around the Toaster, nothing else.
vi.mock("@/features/workspaces/context", () => ({
  WorkspaceProvider: ({ children }: { children: React.ReactNode }) => children,
}));
vi.mock("@/features/usage/context/payment-required", () => ({
  PaymentRequiredProvider: ({ children }: { children: React.ReactNode }) =>
    children,
}));

/**
 * Stands in for Ory Elements' `DefaultToast`, which calls `useIntl()` and is
 * rendered through sonner's `toast()` — i.e. PORTALED into <Toaster/>, outside
 * the IntlProvider that <Settings>/<Login> mount internally.
 *
 * Regression guard: shipping the Toaster without its own IntlProvider made
 * `useIntl()` throw "Could not find required `intl` object", which the root
 * error boundary turned into a full-page crash — /settings was unusable the
 * moment a Kratos flow produced any message (e.g. arriving from a completed
 * recovery, or a rejected password).
 */
function OryStyleToast() {
  const intl = useIntl();
  return (
    <span>
      {intl.formatMessage({
        id: "settings.messages.toast-title.success",
        defaultMessage: "Settings updated",
      })}
    </span>
  );
}

describe("RootProvider", () => {
  it("gives sonner-portaled Ory toasts an intl context (no IntlProvider => full-page crash)", async () => {
    render(<RootProvider>{null}</RootProvider>);

    // OryToaster is lazy-loaded (w9/m60 t004 keeps react-intl out of the entry
    // chunk); wait for its <Toaster/> region to mount before firing — real Ory
    // flow toasts always fire after user interaction, long after this resolves.
    await screen.findByLabelText(/Notifications/);

    // Fire the toast the way Ory Elements does — the content renders inside
    // <Toaster/>'s tree, not where toast() was called.
    toast.custom(() => <OryStyleToast />);

    // It must render rather than throw the react-intl invariant.
    await waitFor(() =>
      expect(screen.getByText("Settings updated")).toBeInTheDocument(),
    );
  });

  it("translates the toast through Ory's own catalog, not just English defaults", async () => {
    render(<RootProvider>{null}</RootProvider>);
    await screen.findByLabelText(/Notifications/);
    toast.custom(() => <OryStyleToast />);

    // OryLocales supplies this id, so the rendered text comes from the catalog.
    // (Locale is `en` in tests; the assertion is that formatMessage resolved at
    // all — an absent provider would have thrown before reaching this point.)
    await waitFor(() =>
      expect(screen.getByText("Settings updated")).toBeInTheDocument(),
    );
  });
});
