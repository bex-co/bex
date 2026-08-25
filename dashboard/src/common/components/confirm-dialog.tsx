import { type ReactNode, useState } from "react";

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/common/components/ui/alert-dialog";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils.ts";

/**
 * The dashboard's one confirm dialog (w1/m89).
 *
 * Before this existed, 27 files re-spelled the same header/description/
 * cancel/confirm JSX, so any improvement to the shape — a pending spinner, a
 * consistent destructive variant, an aria fix — had to land 27 times or land
 * inconsistently. In practice it landed inconsistently.
 *
 * The API is taken from the call sites that exist, not from the ones that would
 * be tidy. Three shapes are in real use and all three are supported here:
 *
 *   1. **Uncontrolled** — pass `trigger`; the dialog owns its own open state.
 *      This is the common table-row / toolbar-button case.
 *   2. **Controlled** — pass `open` + `onOpenChange`; the caller drives it from
 *      a selection (a snapshot row, a delivery row) where the trigger and the
 *      dialog are far apart in the tree. A caller may pass BOTH — a trigger to
 *      open it, and controlled state so it can close itself once its action
 *      resolves.
 *   3. **Phrase-gated** — additionally pass `phrase`; confirm stays disabled
 *      until the user types it exactly. For actions that destroy data and leave
 *      no second signal that they happened.
 *
 * `ProtectedConfirmationDialog` stays separate: its phrase is issued by the
 * server and it is part of an API handshake, not a local are-you-sure.
 */
export interface ConfirmDialogProps {
  /** Uncontrolled: the element that opens the dialog. Omit when controlling it. */
  trigger?: ReactNode;
  /** Controlled: current open state. Omit when passing `trigger`. */
  open?: boolean;
  onOpenChange?: (next: boolean) => void;
  title: string;
  description: ReactNode;
  /**
   * Confirm button label. A node, not a string: several sites swap in a
   * spinner + text while their action is in flight.
   */
  confirmLabel: ReactNode;
  /** Cancel label; defaults to the shared `common.cancel`. */
  cancelLabel?: string;
  onConfirm: () => void;
  /**
   * Runs when the user dismisses via Cancel. Some sites need it — a navigation
   * blocker has to be reset, not merely left alone.
   */
  onCancel?: () => void;
  /** Styles confirm as destructive. Default true — nearly every caller is one. */
  destructive?: boolean;
  /**
   * An action is in flight: confirm is disabled, and so is cancel — dismissing
   * mid-flight would hide an operation that is still going to happen.
   */
  pending?: boolean;
  /**
   * When set, confirm is disabled until the user types this string exactly.
   * The value is shown in the prompt so it can be read and copied.
   */
  phrase?: string;
  /** Extra content between the description and the footer. */
  children?: ReactNode;
  /**
   * Gates confirm on the caller's own condition — for sites that render their
   * own confirmation control (a shared SudoCommandField, say) instead of using
   * `phrase`. Distinct from `pending`, which means an action is in flight.
   */
  confirmDisabled?: boolean;
  /** Extra classes on the dialog surface, e.g. a scroll cap for tall content. */
  contentClassName?: string;
}

export function ConfirmDialog({
  trigger,
  open,
  onOpenChange,
  title,
  description,
  confirmLabel,
  cancelLabel,
  onConfirm,
  onCancel,
  destructive = true,
  pending = false,
  phrase,
  children,
  confirmDisabled = false,
  contentClassName,
}: ConfirmDialogProps) {
  const { t } = useTranslations();
  const [typed, setTyped] = useState("");
  const gateOpen = phrase === undefined || typed === phrase;

  // Clearing on close means reopening never starts pre-armed with a phrase
  // typed for a previous — possibly different — target.
  const handleOpenChange = (next: boolean) => {
    if (!next) setTyped("");
    onOpenChange?.(next);
  };

  const body = (
    <AlertDialogContent className={contentClassName}>
      <AlertDialogHeader>
        <AlertDialogTitle>{title}</AlertDialogTitle>
        <AlertDialogDescription>{description}</AlertDialogDescription>
      </AlertDialogHeader>
      {children}
      {phrase !== undefined ? (
        <div className="space-y-2">
          <Label htmlFor="confirm-dialog-phrase">
            {t("common.confirmPhrasePrompt", { phrase })}
          </Label>
          <Input
            id="confirm-dialog-phrase"
            // The phrase doubles as the placeholder so it is visible in the
            // field itself, not only in the label above it.
            placeholder={phrase}
            value={typed}
            onChange={(e) => setTyped(e.target.value)}
            autoComplete="off"
            spellCheck={false}
            className="font-mono"
          />
        </div>
      ) : null}
      <AlertDialogFooter>
        <AlertDialogCancel disabled={pending} onClick={onCancel}>
          {cancelLabel ?? t("common.cancel")}
        </AlertDialogCancel>
        <AlertDialogAction
          disabled={pending || confirmDisabled || !gateOpen}
          className={cn(
            destructive &&
              "bg-destructive text-white hover:bg-destructive/90 focus-visible:ring-destructive/20",
          )}
          onClick={() => {
            onConfirm();
            setTyped("");
          }}
        >
          {confirmLabel}
        </AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  );

  // One path serves all three shapes. `open={undefined}` leaves Radix
  // uncontrolled, which is exactly what the trigger-only case wants — and
  // several sites are BOTH: a trigger that opens the dialog plus controlled
  // state, so the caller can close it itself once its action resolves.
  return (
    <AlertDialog open={open} onOpenChange={handleOpenChange}>
      {trigger !== undefined ? (
        <AlertDialogTrigger asChild>{trigger}</AlertDialogTrigger>
      ) : null}
      {body}
    </AlertDialog>
  );
}
