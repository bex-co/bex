import { useState } from "react";
import { Loader2 } from "lucide-react";
import { Button, buttonVariants } from "@/common/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/common/components/ui/alert-dialog";
import { SudoCommandField } from "@/common/components/sudo-command-field";
import { useTranslations } from "@/common/hooks/use-translations";
import type { KeyValueView } from "@/features/keyvalue/types";

export interface DeleteKeyValueDialogProps {
  keyValue: KeyValueView;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  busy: boolean;
  onConfirm: () => void | Promise<void>;
}

/** Shared Render-style sudo delete gate for detail and table placements. */
export function DeleteKeyValueDialog({
  keyValue,
  open,
  onOpenChange,
  busy,
  onConfirm,
}: DeleteKeyValueDialogProps) {
  const { t } = useTranslations();
  const [typed, setTyped] = useState("");
  const confirmPhrase = `sudo delete key value ${keyValue.name}`;
  const canDelete = typed === confirmPhrase && !busy;

  function handleOpenChange(next: boolean) {
    onOpenChange(next);
    if (!next) setTyped("");
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("keyvalue.deleteConfirmTitle")}</DialogTitle>
          <DialogDescription>
            {t("keyvalue.deleteConfirmBody")}
          </DialogDescription>
        </DialogHeader>
        <SudoCommandField
          id="kv-delete-confirm"
          promptKey="keyvalue.deleteConfirmPrompt"
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
            {t("keyvalue.deleteCancel")}
          </Button>
          <Button
            variant="destructive"
            onClick={() => void onConfirm()}
            disabled={!canDelete}
          >
            {busy ? <Loader2 className="animate-spin" /> : null}
            {t("keyvalue.deleteConfirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export interface SuspendKeyValueDialogProps {
  keyValue: KeyValueView;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  busy: boolean;
  onConfirm: () => void | Promise<void>;
}

/** Render-parity suspend gate: require the exact sudo phrase before submit. */
export function SuspendKeyValueDialog({
  keyValue,
  open,
  onOpenChange,
  busy,
  onConfirm,
}: SuspendKeyValueDialogProps) {
  const { t } = useTranslations();
  const [confirmation, setConfirmation] = useState("");
  const confirmPhrase = `sudo suspend key value ${keyValue.name}`;
  const canSuspend = confirmation === confirmPhrase && !busy;

  function handleOpenChange(next: boolean) {
    onOpenChange(next);
    if (!next) setConfirmation("");
  }

  return (
    <AlertDialog open={open} onOpenChange={handleOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>
            {t("keyvalue.confirmSuspendTitle")}
          </AlertDialogTitle>
          <AlertDialogDescription>
            <span className="block">{t("keyvalue.confirmSuspendBody")}</span>
            <span className="mt-2 block">
              {t("keyvalue.confirmSuspendDetail", { name: keyValue.name })}
            </span>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <SudoCommandField
          id="kv-suspend-confirm"
          promptKey="keyvalue.confirmSuspendPrompt"
          phrase={confirmPhrase}
          value={confirmation}
          onValueChange={setConfirmation}
        />
        <AlertDialogFooter>
          <AlertDialogCancel>{t("keyvalue.confirmCancel")}</AlertDialogCancel>
          <AlertDialogAction
            className={buttonVariants({ variant: "destructive" })}
            onClick={() => void onConfirm()}
            disabled={!canSuspend}
          >
            {t("keyvalue.actionSuspend")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
