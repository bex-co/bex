import { useState } from "react";
import { MoreHorizontal, Loader2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { useTranslations } from "@/common/hooks/use-translations";
import { useDeleteDatabase } from "@/features/databases/hooks/use-delete-database";
import type { DatabaseView } from "@/features/databases/types";

export interface DatabaseRowActionsProps {
  database: DatabaseView;
  /** Called after a successful delete (refetch the list / leave the detail page). */
  onDeleted: (id: string) => void;
}

/**
 * The per-database actions menu. Delete is the only lifecycle verb bex serves
 * today (suspend/resume/failover/PITR are deferred — omitted, not faked). It's
 * destructive and irreversible (cascades the CNPG cluster + PVC), so it's gated
 * behind a typed-name confirmation, matching Render's own delete-DB modal.
 */
export function DatabaseRowActions({
  database,
  onDeleted,
}: DatabaseRowActionsProps) {
  const { t } = useTranslations();
  const { remove, deleting } = useDeleteDatabase();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [typed, setTyped] = useState("");

  const busy = deleting === database.id;
  const canDelete = typed === database.name && !busy;

  async function handleDelete() {
    const ok = await remove(database.id, database.name);
    if (ok) {
      setConfirmOpen(false);
      onDeleted(database.id);
    }
  }

  function handleOpenChange(next: boolean) {
    setConfirmOpen(next);
    if (!next) setTyped("");
  }

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="icon-sm"
            disabled={busy}
            aria-label={t("databases.actionsMenu")}
          >
            {busy ? <Loader2 className="animate-spin" /> : <MoreHorizontal />}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem
            variant="destructive"
            onSelect={() => setConfirmOpen(true)}
          >
            {t("databases.actionDelete")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <Dialog open={confirmOpen} onOpenChange={handleOpenChange}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {t("databases.deleteConfirmTitle", { name: database.name })}
            </DialogTitle>
            <DialogDescription>
              {t("databases.deleteConfirmBody")}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="db-delete-confirm">
              {t("databases.deleteConfirmPrompt", { name: database.name })}
            </Label>
            <Input
              id="db-delete-confirm"
              value={typed}
              onChange={(e) => setTyped(e.target.value)}
              autoComplete="off"
              placeholder={database.name}
            />
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => handleOpenChange(false)}
              disabled={busy}
            >
              {t("databases.deleteCancel")}
            </Button>
            <Button
              variant="destructive"
              onClick={() => void handleDelete()}
              disabled={!canDelete}
            >
              {busy ? <Loader2 className="animate-spin" /> : null}
              {t("databases.deleteConfirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
