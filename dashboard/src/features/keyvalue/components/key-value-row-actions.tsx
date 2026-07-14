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
 * StatefulSet + PVC), so it's gated behind a typed-name confirmation, matching
 * databases' DatabaseRowActions.
 */
export function KeyValueRowActions({
  keyValue,
  onDeleted,
}: KeyValueRowActionsProps) {
  const { t } = useTranslations();
  const { remove, deleting } = useDeleteKeyValue();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [typed, setTyped] = useState("");

  const busy = deleting === keyValue.id;
  const canDelete = typed === keyValue.name && !busy;

  async function handleDelete() {
    const ok = await remove(keyValue.id, keyValue.name);
    if (ok) {
      setConfirmOpen(false);
      onDeleted(keyValue.id);
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

      <Dialog open={confirmOpen} onOpenChange={handleOpenChange}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {t("keyvalue.deleteConfirmTitle", { name: keyValue.name })}
            </DialogTitle>
            <DialogDescription>
              {t("keyvalue.deleteConfirmBody")}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="kv-delete-confirm">
              {t("keyvalue.deleteConfirmPrompt", { name: keyValue.name })}
            </Label>
            <Input
              id="kv-delete-confirm"
              value={typed}
              onChange={(e) => setTyped(e.target.value)}
              autoComplete="off"
              placeholder={keyValue.name}
            />
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => handleOpenChange(false)}
              disabled={busy}
            >
              {t("keyvalue.deleteCancel")}
            </Button>
            <Button
              variant="destructive"
              onClick={() => void handleDelete()}
              disabled={!canDelete}
            >
              {busy ? <Loader2 className="animate-spin" /> : null}
              {t("keyvalue.deleteConfirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
