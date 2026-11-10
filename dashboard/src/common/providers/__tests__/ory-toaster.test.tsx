import { afterEach, describe, expect, it } from "vitest";
import { act, render, screen } from "@testing-library/react";
import { useEffect } from "react";
import { toast } from "sonner";
import OryToaster from "@/common/providers/ory-toaster";
import i18n from "@/i18n/init";

afterEach(async () => {
  act(() => {
    toast.dismiss();
  });
  await act(async () => {
    await i18n.changeLanguage("en");
  });
});

// w4/158: sonner hands a toast only to Toasters subscribed when it is raised.
// The Toaster is a lazy chunk (w9/m60 t004), so a toast raised during
// hydration, such as the not-found toast a directly opened dead-id URL fires,
// used to vanish. These pin the replay that shows it anyway, and with it the
// sonner behavior the replay relies on (a store that keeps the toast, and an
// in-place create by id).
describe("OryToaster", () => {
  it("names its landmark in the selected language (w21/017)", async () => {
    const { container, unmount } = render(<OryToaster />);
    expect(
      container.ownerDocument.querySelector(
        'section[aria-label^="Notifications"]',
      ),
    ).not.toBeNull();
    unmount();
    await act(async () => {
      await i18n.changeLanguage("zh");
    });
    render(<OryToaster />);
    expect(
      document.querySelector('section[aria-label^="通知"]'),
    ).not.toBeNull();
    expect(
      document.querySelector('section[aria-label^="Notifications"]'),
    ).toBeNull();
  });

  it("shows a toast raised before the Toaster mounted", async () => {
    toast.error("That resource doesn't exist or was deleted.", {
      id: "resource-not-found",
    });

    render(<OryToaster />);

    expect(
      await screen.findByText("That resource doesn't exist or was deleted."),
    ).toBeInTheDocument();
  });

  it("shows an early toast once, even if it is raised again after mount", async () => {
    toast.error("Early and repeated", { id: "repeat" });
    function RaiseAgain() {
      useEffect(() => {
        toast.error("Early and repeated", { id: "repeat" });
      }, []);
      return null;
    }

    render(
      <>
        <OryToaster />
        <RaiseAgain />
      </>,
    );

    expect(await screen.findAllByText("Early and repeated")).toHaveLength(1);
  });

  it("does not resurrect a toast dismissed before the Toaster mounted", async () => {
    toast.error("Dismissed early", { id: "gone" });
    toast.dismiss("gone");

    render(<OryToaster />);
    // Let the replay effect and sonner's deferred delivery run.
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 20));
    });
    expect(screen.queryByText("Dismissed early")).not.toBeInTheDocument();
  });
});
