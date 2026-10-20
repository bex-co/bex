import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import type { StrandedPromptItem } from "@/features/env-groups/lib/stranded";

export type StrandedGroupPromptItem = StrandedPromptItem;

/**
 * The w4/m238 move-time confirmation: a move that leaves a service linked to
 * a group scoped elsewhere asks first, naming each stranded group and its
 * scope. Cancel writes nothing; Confirm runs the caller's original single
 * write. Shared by every dashboard move entrypoint (environment card,
 * Manage resources, Move to project).
 */
export function StrandedGroupsConfirmDialog({
  open,
  onOpenChange,
  items,
  onConfirm,
  pending = false,
}: {
  open: boolean;
  onOpenChange: (next: boolean) => void;
  items: ReadonlyArray<StrandedGroupPromptItem>;
  onConfirm: () => void;
  pending?: boolean;
}) {
  const { t } = useTranslations();
  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t("environments.strandedMoveTitle")}
      description={t("environments.strandedMoveBody")}
      confirmLabel={t("environments.strandedMoveConfirm")}
      cancelLabel={t("environments.cancel")}
      onConfirm={onConfirm}
      pending={pending}
      destructive={false}
    >
      <ul className="space-y-1 text-sm">
        {items.map((item) => (
          <li key={item.id} className="flex flex-wrap items-baseline gap-x-2">
            <span className="font-medium break-all">{item.name}</span>
            <span className="text-muted-foreground">
              {item.scopeLabel ??
                t("environments.strandedScopeWorkspace")}
            </span>
          </li>
        ))}
      </ul>
    </ConfirmDialog>
  );
}
