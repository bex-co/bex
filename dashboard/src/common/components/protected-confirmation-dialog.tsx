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

export interface ProtectedConfirmationDialogProps {
  open: boolean;
  resourceName: string;
  requiredConfirmation: string;
  actionLabel: string;
  busy: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: (confirmation: string) => Promise<void>;
}

/** Typed retry dialog driven by the exact phrase returned by bex-api. */
export function ProtectedConfirmationDialog({
  open,
  resourceName,
  requiredConfirmation,
  actionLabel,
  busy,
  onOpenChange,
  onConfirm,
}: ProtectedConfirmationDialogProps) {
  const { t } = useTranslations();
  const [confirmation, setConfirmation] = useState("");
  const matches = confirmation === requiredConfirmation;

  function handleOpenChange(next: boolean) {
    onOpenChange(next);
    if (!next) setConfirmation("");
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("common.protectedConfirmationTitle")}</DialogTitle>
          <DialogDescription>
            {t("common.protectedConfirmationBody", { name: resourceName })}
          </DialogDescription>
        </DialogHeader>
        <SudoCommandField
          id="protected-resource-confirmation"
          promptKey="common.protectedConfirmationPrompt"
          phrase={requiredConfirmation}
          value={confirmation}
          onValueChange={setConfirmation}
        />
        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => handleOpenChange(false)}
            disabled={busy}
          >
            {t("common.protectedConfirmationCancel")}
          </Button>
          <Button
            variant="destructive"
            onClick={() => void onConfirm(confirmation)}
            disabled={!matches || busy}
          >
            {busy ? <Loader2 className="animate-spin" /> : null}
            {actionLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
