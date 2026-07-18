import { useState } from "react";
import { Loader2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { SudoCommandField } from "@/common/components/sudo-command-field";
import { useTranslations } from "@/common/hooks/use-translations";
import type { DatabaseLifecycleAction } from "@/features/databases/hooks/use-database-lifecycle";
import type { DatabaseView } from "@/features/databases/types";

export interface DeleteDatabaseDialogProps {
  database: DatabaseView;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  busy: boolean;
  onConfirm: () => void | Promise<void>;
}

/**
 * Delete: Render-style sudo type-to-confirm gate (destructive, irreversible —
 * it cascades the CNPG cluster + PVC). Owns the typed state so every caller
 * gets the same gate.
 */
export function DeleteDatabaseDialog({
  database,
  open,
  onOpenChange,
  busy,
  onConfirm,
}: DeleteDatabaseDialogProps) {
  const { t } = useTranslations();
  const [typed, setTyped] = useState("");
  const confirmPhrase = `sudo delete postgres ${database.name}`;
  const canDelete = typed === confirmPhrase && !busy;

  function handleOpenChange(next: boolean) {
    onOpenChange(next);
    if (!next) setTyped("");
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {t("databases.deleteConfirmTitle", { name: database.name })}
          </DialogTitle>
          <DialogDescription>
            {t("databases.deleteConfirmBody")}
          </DialogDescription>
        </DialogHeader>
        <SudoCommandField
          id="db-delete-confirm"
          promptKey="databases.deleteConfirmPrompt"
          phrase={confirmPhrase}
          value={typed}
          onValueChange={setTyped}
        />
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
            onClick={() => void onConfirm()}
            disabled={!canDelete}
          >
            {busy ? <Loader2 className="animate-spin" /> : null}
            {t("databases.deleteConfirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export interface DatabaseLifecycleConfirmDialogProps {
  database: DatabaseView;
  /** The disruptive verb awaiting confirmation, or null (closed). */
  verb: Extract<DatabaseLifecycleAction, "suspend" | "restart"> | null;
  busy: boolean;
  onClose: () => void;
  onConfirm: (verb: DatabaseLifecycleAction) => void;
}

/** Suspend / restart: simple confirm (disruptive but reversible). */
export function DatabaseLifecycleConfirmDialog({
  database,
  verb,
  busy,
  onClose,
  onConfirm,
}: DatabaseLifecycleConfirmDialogProps) {
  const { t } = useTranslations();
  return (
    <Dialog open={verb !== null} onOpenChange={(next) => !next && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {verb === "suspend"
              ? t("databases.suspendConfirmTitle", { name: database.name })
              : t("databases.restartConfirmTitle", { name: database.name })}
          </DialogTitle>
          <DialogDescription>
            {verb === "suspend"
              ? t("databases.suspendConfirmBody")
              : t("databases.restartConfirmBody")}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={busy}>
            {t("databases.deleteCancel")}
          </Button>
          <Button
            onClick={() => {
              const confirmed = verb;
              onClose();
              if (confirmed) onConfirm(confirmed);
            }}
            disabled={busy}
          >
            {busy ? <Loader2 className="animate-spin" /> : null}
            {verb === "suspend"
              ? t("databases.actionSuspend")
              : t("databases.actionRestart")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
