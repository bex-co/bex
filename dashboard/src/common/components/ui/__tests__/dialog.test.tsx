import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
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
});
