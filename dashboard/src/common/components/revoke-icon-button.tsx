import { Loader2, Trash2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { ConfirmDialog } from "@/common/components/confirm-dialog";

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
 * A destructive icon button behind a confirm — the shared shape of a table
 * row's "revoke this" control (`connected-agent-row.tsx`, `session-row.tsx`).
 *
 * A thin specialization of ConfirmDialog: it owns only the icon-button trigger
 * and its pending spinner. Reimplementing it on the primitive is what proves
 * the primitive actually fits the call sites it is meant to replace (w1/m89).
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
    <ConfirmDialog
      trigger={
        <Button size="icon" variant="ghost" aria-label={label} disabled={pending}>
          {pending ? <Loader2 className="animate-spin" /> : <Trash2 />}
        </Button>
      }
      title={confirmTitle}
      description={confirmBody}
      cancelLabel={cancelLabel}
      confirmLabel={confirmLabel}
      onConfirm={onConfirm}
      pending={pending}
    />
  );
}
