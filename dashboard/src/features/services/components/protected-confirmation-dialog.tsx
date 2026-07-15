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
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { useTranslations } from "@/common/hooks/use-translations";

export interface ProtectedConfirmationDialogProps {
  open: boolean;
  serviceName: string;
  requiredConfirmation: string;
  actionLabel: string;
  busy: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: (confirmation: string) => Promise<void>;
}

/** Typed retry dialog driven by the exact phrase returned by bex-api. */
export function ProtectedConfirmationDialog({
  open,
  serviceName,
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
          <DialogTitle>{t("services.protectedConfirmationTitle")}</DialogTitle>
          <DialogDescription>
            {t("services.protectedConfirmationBody", { name: serviceName })}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-2">
          <Label htmlFor="protected-service-confirmation">
            {t("services.protectedConfirmationPrompt", {
              confirmation: requiredConfirmation,
            })}
          </Label>
          <Input
            id="protected-service-confirmation"
            value={confirmation}
            onChange={(event) => setConfirmation(event.target.value)}
            autoComplete="off"
            placeholder={requiredConfirmation}
          />
        </div>
        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => handleOpenChange(false)}
            disabled={busy}
          >
            {t("services.confirmCancel")}
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
