import { afterEach, describe, it, expect } from "vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import i18n from "@/i18n/init";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "../dialog";

describe("Dialog", () => {
  it("should render dialog content with scrollable overflow", () => {
    render(
      <Dialog open>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Test Dialog</DialogTitle>
            <DialogDescription>Test Description</DialogDescription>
          </DialogHeader>
        </DialogContent>
      </Dialog>,
    );

    const dialogContent = document.querySelector(
      '[data-slot="dialog-content"]',
    );
    expect(dialogContent).toBeInTheDocument();
    expect(dialogContent).toHaveClass("overflow-y-auto");
  });

  it("should render dialog content with max height constraint", () => {
    render(
      <Dialog open>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Test Dialog</DialogTitle>
            <DialogDescription>Test Description</DialogDescription>
          </DialogHeader>
        </DialogContent>
      </Dialog>,
    );

    const dialogContent = document.querySelector(
      '[data-slot="dialog-content"]',
    );
    expect(dialogContent).toBeInTheDocument();
    expect(dialogContent).toHaveClass("max-h-[90vh]");
  });

  it("should render dialog with title and description", () => {
    render(
      <Dialog open>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Test Title</DialogTitle>
            <DialogDescription>Test Description</DialogDescription>
          </DialogHeader>
        </DialogContent>
      </Dialog>,
    );

    expect(screen.getByText("Test Title")).toBeInTheDocument();
    expect(screen.getByText("Test Description")).toBeInTheDocument();
  });

  it("should render close button by default", () => {
    render(
      <Dialog open>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Test Dialog</DialogTitle>
            <DialogDescription>Test Description</DialogDescription>
          </DialogHeader>
        </DialogContent>
      </Dialog>,
    );

    const closeButton = document.querySelector('[data-slot="dialog-close"]');
    expect(closeButton).toBeInTheDocument();
    expect(screen.getByText("Close")).toBeInTheDocument();
  });

  it("should hide close button when showCloseButton is false", () => {
    render(
      <Dialog open>
        <DialogContent showCloseButton={false}>
          <DialogHeader>
            <DialogTitle>Test Dialog</DialogTitle>
            <DialogDescription>Test Description</DialogDescription>
          </DialogHeader>
        </DialogContent>
      </Dialog>,
    );

    const closeButton = document.querySelector('[data-slot="dialog-close"]');
    expect(closeButton).not.toBeInTheDocument();
  });

  it("should allow custom className on dialog content", () => {
    render(
      <Dialog open>
        <DialogContent className="custom-class">
          <DialogHeader>
            <DialogTitle>Test Dialog</DialogTitle>
            <DialogDescription>Test Description</DialogDescription>
          </DialogHeader>
        </DialogContent>
      </Dialog>,
    );

    const dialogContent = document.querySelector(
      '[data-slot="dialog-content"]',
    );
    expect(dialogContent).toHaveClass("custom-class");
  });

  describe("built-in close button (w4/310)", () => {
    afterEach(async () => {
      await i18n.changeLanguage("en");
    });

    function Controlled({ showCloseButton }: { showCloseButton?: boolean }) {
      const [open, setOpen] = useState(true);
      return (
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogContent showCloseButton={showCloseButton}>
            <DialogTitle>Title</DialogTitle>
            <DialogDescription>Body</DialogDescription>
          </DialogContent>
        </Dialog>
      );
    }

    it("names the close button in the selected language and follows a switch", async () => {
      render(<Controlled />);
      expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument();
      await act(() => i18n.changeLanguage("zh"));
      expect(screen.getByRole("button", { name: "关闭" })).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Close" })).toBeNull();
      await act(() => i18n.changeLanguage("en"));
      expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument();
    });

    it("still dismisses by click and by Escape in Chinese", async () => {
      await i18n.changeLanguage("zh");
      const user = userEvent.setup();
      const { unmount } = render(<Controlled />);
      await user.click(screen.getByRole("button", { name: "关闭" }));
      expect(screen.queryByRole("dialog")).toBeNull();
      unmount();
      render(<Controlled />);
      await user.keyboard("{Escape}");
      expect(screen.queryByRole("dialog")).toBeNull();
    });

    it("omits the built-in button when the caller suppresses it", async () => {
      await i18n.changeLanguage("zh");
      render(<Controlled showCloseButton={false} />);
      expect(screen.getByRole("dialog")).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "关闭" })).toBeNull();
    });
  });
});

// w4/459: a controlled Dialog with no Trigger returns focus to its opener.
describe("Dialog focus return", () => {
  function Harness({ withTrigger = false }: { withTrigger?: boolean }) {
    const [open, setOpen] = useState(false);
    const [mounted, setMounted] = useState(true);
    return (
      <>
        {mounted && <button onClick={() => setOpen(true)}>opener</button>}
        <button onClick={() => setMounted(false)}>remove opener</button>
        <Dialog open={open} onOpenChange={setOpen}>
          {withTrigger && <span />}
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Title</DialogTitle>
              <DialogDescription>Body</DialogDescription>
            </DialogHeader>
          </DialogContent>
        </Dialog>
      </>
    );
  }

  it("returns focus to the opener on Escape and on the close button", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const opener = screen.getByRole("button", { name: "opener" });
    for (const close of ["escape", "button"]) {
      opener.focus();
      await user.keyboard("{Enter}");
      expect(screen.getByRole("dialog")).toBeInTheDocument();
      if (close === "escape") await user.keyboard("{Escape}");
      else await user.click(screen.getByRole("button", { name: /close/i }));
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
      await waitFor(() => expect(opener, close).toHaveFocus());
    }
  });

  it("does not force focus onto an opener that was removed", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    screen.getByRole("button", { name: "opener" }).focus();
    await user.keyboard("{Enter}");
    // Remove the opener while the dialog is open (programmatic, like a
    // create that navigates away), then close.
    fireEvent.click(screen.getByText("remove opener"));
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    await new Promise((resolve) => setTimeout(resolve, 20));
    expect(document.body).toHaveFocus();
  });
});

// w4/463: Radix skips its open-autofocus event when a child with `autoFocus`
// already holds focus, which used to leave the opener unrecorded.
describe("Dialog focus return with an autofocused field", () => {
  it("returns focus to the opener when the first field autofocuses", async () => {
    const user = userEvent.setup();
    function Harness() {
      const [open, setOpen] = useState(false);
      return (
        <>
          <button onClick={() => setOpen(true)}>opener</button>
          <Dialog open={open} onOpenChange={setOpen}>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Title</DialogTitle>
                <DialogDescription>Body</DialogDescription>
              </DialogHeader>
              <input aria-label="field" autoFocus />
            </DialogContent>
          </Dialog>
        </>
      );
    }
    render(<Harness />);
    const opener = screen.getByRole("button", { name: "opener" });
    opener.focus();
    await user.keyboard("{Enter}");
    expect(screen.getByLabelText("field")).toHaveFocus();
    await user.keyboard("{Escape}");
    await waitFor(() => expect(opener).toHaveFocus());
  });

  it("returns to the menu's trigger when the opening menu item unmounts", async () => {
    const user = userEvent.setup();
    function Harness() {
      const [menu, setMenu] = useState(false);
      const [open, setOpen] = useState(false);
      return (
        <>
          <button onClick={() => setMenu(true)}>menu trigger</button>
          {menu && (
            <button
              onClick={() => {
                setMenu(false);
                setOpen(true);
              }}
            >
              menu item
            </button>
          )}
          <Dialog open={open} onOpenChange={setOpen}>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Title</DialogTitle>
                <DialogDescription>Body</DialogDescription>
              </DialogHeader>
              <input aria-label="field" autoFocus />
            </DialogContent>
          </Dialog>
        </>
      );
    }
    render(<Harness />);
    const trigger = screen.getByRole("button", { name: "menu trigger" });
    trigger.focus();
    await user.keyboard("{Enter}");
    screen.getByRole("button", { name: "menu item" }).focus();
    await user.keyboard("{Enter}");
    expect(screen.getByLabelText("field")).toHaveFocus();
    await user.keyboard("{Escape}");
    await waitFor(() => expect(trigger).toHaveFocus());
  });
});
