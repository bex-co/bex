import { Loader2, Trash2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
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

export interface RevokeIconButtonProps {
  label: string;
  confirmTitle: string;
  confirmBody: string;
  cancelLabel: string;
  confirmLabel: string;
  onConfirm: () => void;
  /** True while the revoke is in flight — disables the control and swaps the icon for a spinner. */
  pending: boolean;
}

/**
 * A destructive icon button behind an `AlertDialog` confirm — the shared shape
 * of a table row's "revoke this" control (`connected-agent-row.tsx`,
 * `session-row.tsx`).
 */
export function RevokeIconButton({
  label,
  confirmTitle,
  confirmBody,
  cancelLabel,
  confirmLabel,
  onConfirm,
  pending,
}: RevokeIconButtonProps) {
  return (
    <AlertDialog>
      <AlertDialogTrigger asChild>
        <Button
          size="icon"
          variant="ghost"
          aria-label={label}
          disabled={pending}
        >
          {pending ? (
            <Loader2 className="animate-spin" />
          ) : (
            <Trash2 className="text-destructive" />
          )}
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{confirmTitle}</AlertDialogTitle>
          <AlertDialogDescription>{confirmBody}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{cancelLabel}</AlertDialogCancel>
          <AlertDialogAction onClick={onConfirm}>
            {confirmLabel}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
