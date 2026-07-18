import { useState } from "react";
import { MoreHorizontal, Loader2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import { useTranslations } from "@/common/hooks/use-translations";
import { DeleteKeyValueDialog } from "@/features/keyvalue/components/key-value-confirm-dialogs";
import { useDeleteKeyValue } from "@/features/keyvalue/hooks/use-delete-key-value";
import type { KeyValueView } from "@/features/keyvalue/types";
import { MoveToProjectMenu } from "@/features/projects/components/move-to-project-menu";

export interface KeyValueRowActionsProps {
  keyValue: KeyValueView;
  /** Called after a successful delete (refetch the list / leave the detail page). */
  onDeleted: (id: string) => void;
}

/**
 * The per-store actions menu. Delete is the only lifecycle verb this menu
 * serves — suspend/resume live on the detail page's own action bar (matching
 * Render's KV detail capture, docs/render-artifacts/key-value.md), not this
 * compact row menu. Destructive and irreversible (cascades the Valkey
 * StatefulSet + PVC), so it uses the same exact typed sudo confirmation as
 * Render's Key Value detail page.
 */
export function KeyValueRowActions({
  keyValue,
  onDeleted,
}: KeyValueRowActionsProps) {
  const { t } = useTranslations();
  const { remove, deleting } = useDeleteKeyValue();
  const [confirmOpen, setConfirmOpen] = useState(false);

  const busy = deleting === keyValue.id;

  async function handleDelete() {
    const ok = await remove(keyValue.id, keyValue.name);
    if (ok) {
      setConfirmOpen(false);
      onDeleted(keyValue.id);
    }
  }

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="icon-sm"
            disabled={busy}
            aria-label={t("keyvalue.actionsMenu")}
          >
            {busy ? <Loader2 className="animate-spin" /> : <MoreHorizontal />}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem
            variant="destructive"
            onSelect={() => setConfirmOpen(true)}
          >
            {t("keyvalue.actionDelete")}
          </DropdownMenuItem>
          <MoveToProjectMenu
            kind="keyvalue"
            resourceId={keyValue.id}
            resourceName={keyValue.name}
            disabled={busy}
          />
        </DropdownMenuContent>
      </DropdownMenu>

      <DeleteKeyValueDialog
        keyValue={keyValue}
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        busy={busy}
        onConfirm={handleDelete}
      />
    </>
  );
}
