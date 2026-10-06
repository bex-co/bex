import { ProtectedConfirmationDialog } from "@/common/components/protected-confirmation-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  takeoverCopy,
  type BlueprintConfirmationRequired,
} from "@/features/blueprints/lib/takeover";
import { protectedServiceName } from "@/features/services/lib/protected-confirmation";

/**
 * The typed-phrase retry for a Blueprint create or sync bex-api refused: a
 * protected environment, or a takeover the dialog names (w4/189). Open while
 * `pending` is set; keyed by the phrase so a new refusal starts empty.
 */
export function BlueprintConfirmationDialog({
  pending,
  actionLabel,
  busy,
  onDismiss,
  onConfirm,
}: {
  pending: BlueprintConfirmationRequired | null;
  actionLabel: string;
  busy: boolean;
  onDismiss: () => void;
  onConfirm: (confirmation: string) => Promise<void>;
}) {
  const { t } = useTranslations();
  return (
    <ProtectedConfirmationDialog
      key={pending ? `open:${pending.confirmation}` : "closed"}
      open={pending !== null}
      resourceName={pending ? protectedServiceName(pending.confirmation) : ""}
      requiredConfirmation={pending?.confirmation ?? ""}
      actionLabel={actionLabel}
      {...(pending?.takeover ? takeoverCopy(pending.takeover, t) : {})}
      busy={busy}
      onOpenChange={(open) => {
        if (!open) onDismiss();
      }}
      onConfirm={onConfirm}
    />
  );
}
