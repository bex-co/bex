import { afterEach, describe, it, expect } from "vitest";
import { act, render, screen } from "@testing-library/react";
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
