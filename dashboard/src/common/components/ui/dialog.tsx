"use client";

import * as React from "react";
import * as DialogPrimitive from "@radix-ui/react-dialog";
import { XIcon } from "lucide-react";

import { cn } from "@/common/lib/utils/utils.ts";
import { useTranslations } from "@/common/hooks/use-translations";

// recentOutsideFocus tracks the last few elements focused outside any dialog.
// A dialog's content mounts only while open and Radix moves focus inside it, so
// the opener has to be remembered while the dialog is closed. More than one is
// kept because a dialog opened from a menu item (which unmounts with its menu)
// should return to the menu's trigger, focused just before the item (w4/463).
const RECENT_OUTSIDE_FOCUS = 8;
const recentFocusedOutside: HTMLElement[] = [];
if (typeof document !== "undefined") {
  document.addEventListener("focusin", (event) => {
    const target = event.target;
    if (
      target instanceof HTMLElement &&
      target !== document.body &&
      !target.closest('[role="dialog"], [role="alertdialog"]')
    ) {
      const at = recentFocusedOutside.indexOf(target);
      if (at >= 0) recentFocusedOutside.splice(at, 1);
      recentFocusedOutside.push(target);
      if (recentFocusedOutside.length > RECENT_OUTSIDE_FOCUS) {
        recentFocusedOutside.shift();
      }
    }
  });
}
function recentOutsideFocus(): HTMLElement[] {
  return [...recentFocusedOutside];
}

function Dialog({
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Root>) {
  return <DialogPrimitive.Root data-slot="dialog" {...props} />;
}

function DialogTrigger({
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Trigger>) {
  return <DialogPrimitive.Trigger data-slot="dialog-trigger" {...props} />;
}

function DialogPortal({
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Portal>) {
  return <DialogPrimitive.Portal data-slot="dialog-portal" {...props} />;
}

function DialogClose({
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Close>) {
  return <DialogPrimitive.Close data-slot="dialog-close" {...props} />;
}

function DialogOverlay({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Overlay>) {
  return (
    <DialogPrimitive.Overlay
      data-slot="dialog-overlay"
      className={cn(
        "data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 fixed inset-0 z-50 bg-black/50",
        className,
      )}
      {...props}
    />
  );
}

// OpenerSnapshot runs when the dialog's content mounts, i.e. when it opens, and
// records the elements focused outside any dialog at that moment.
function OpenerSnapshot({
  intoRef,
}: {
  intoRef: React.RefObject<HTMLElement[]>;
}) {
  React.useLayoutEffect(() => {
    intoRef.current = recentOutsideFocus();
  }, [intoRef]);
  return null;
}

function DialogContent({
  className,
  children,
  showCloseButton = true,
  onOpenAutoFocus,
  onCloseAutoFocus,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Content> & {
  showCloseButton?: boolean;
}) {
  const { t } = useTranslations();
  // The control that opened the dialog: most callers control `open` and
  // render no Trigger, so Radix has nothing to return focus to (w4/459).
  // Snapshotted when the content mounts (see OpenerSnapshot), not in Radix's
  // open-autofocus event: that event is skipped when a child with `autoFocus`
  // already holds focus (w4/463).
  const openersRef = React.useRef<HTMLElement[]>([]);
  const handleCloseAutoFocus = (event: Event) => {
    onCloseAutoFocus?.(event);
    if (event.defaultPrevented) return;
    // The latest opener still in the page: a menu item that unmounted with its
    // menu falls back to the menu's trigger. If the action removed every
    // candidate (create then navigate, delete), leave Radix's default alone.
    const opener = [...openersRef.current]
      .reverse()
      .find((el) => el.isConnected);
    openersRef.current = [];
    if (!opener) return;
    event.preventDefault();
    opener.focus();
  };
  return (
    <DialogPortal data-slot="dialog-portal">
      <DialogOverlay />
      <DialogPrimitive.Content
        data-slot="dialog-content"
        className={cn(
          "bg-background data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95 fixed top-[50%] left-[50%] z-50 grid w-full max-w-[calc(100%-2rem)] max-h-[90vh] translate-x-[-50%] translate-y-[-50%] gap-4 rounded-lg border p-6 shadow-lg duration-200 overflow-y-auto sm:max-w-lg",
          className,
        )}
        onOpenAutoFocus={onOpenAutoFocus}
        onCloseAutoFocus={handleCloseAutoFocus}
        {...props}
      >
        <OpenerSnapshot intoRef={openersRef} />
        {children}
        {showCloseButton && (
          <DialogPrimitive.Close
            data-slot="dialog-close"
            className="ring-offset-background focus:ring-ring data-[state=open]:bg-accent data-[state=open]:text-muted-foreground absolute top-4 right-4 rounded-xs opacity-70 transition-opacity hover:opacity-100 focus:ring-2 focus:ring-offset-2 focus:outline-hidden disabled:pointer-events-none cursor-pointer [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4"
          >
            <XIcon />
            <span className="sr-only">{t("common.close")}</span>
          </DialogPrimitive.Close>
        )}
      </DialogPrimitive.Content>
    </DialogPortal>
  );
}

function DialogHeader({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="dialog-header"
      className={cn("flex flex-col gap-2 text-center sm:text-left", className)}
      {...props}
    />
  );
}

function DialogFooter({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="dialog-footer"
      className={cn(
        "flex flex-col-reverse gap-2 sm:flex-row sm:justify-end",
        className,
      )}
      {...props}
    />
  );
}

function DialogTitle({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Title>) {
  return (
    <DialogPrimitive.Title
      data-slot="dialog-title"
      className={cn("text-lg leading-none font-semibold", className)}
      {...props}
    />
  );
}

function DialogDescription({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Description>) {
  return (
    <DialogPrimitive.Description
      data-slot="dialog-description"
      className={cn("text-muted-foreground text-sm", className)}
      {...props}
    />
  );
}

export {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogOverlay,
  DialogPortal,
  DialogTitle,
  DialogTrigger,
};
