import { useState } from "react";
import { Loader2, Plus } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/common/components/ui/dialog";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { useTranslations } from "@/common/hooks/use-translations";
import { useCreateEnvironment } from "@/features/environments/hooks/use-create-environment";

export interface NewEnvironmentDialogProps {
  /** The project the environment is created under (docs/ADR032-environments.md). */
  projectId: string;
  /**
   * Controlled variant: when both are provided, the dialog's open state is
   * driven externally (the Environments panel's own "New environment" button)
   * and it renders no trigger of its own. Omitted, it manages its own state
   * behind its default trigger button — mirrors NewProjectDialog's shape.
   */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}

/**
 * "New Environment" on a Project's page (docs/ADR032-environments.md, w1/m32).
 * An environment groups a subset of the project's services under a name
 * (staging/production), so this dialog just collects a name — services are
 * assigned afterward from the environment's "Manage services" action.
 */
export function NewEnvironmentDialog({
  projectId,
  open: openProp,
  onOpenChange: onOpenChangeProp,
}: NewEnvironmentDialogProps) {
  const { t } = useTranslations();
  const { create, busy } = useCreateEnvironment(projectId);
  const controlled = openProp !== undefined;
  const [openState, setOpenState] = useState(false);
  const open = controlled ? openProp : openState;
  const [name, setName] = useState("");

  const canSubmit = name.trim().length > 0 && !busy;

  function handleOpenChange(next: boolean) {
    if (controlled) onOpenChangeProp?.(next);
    else setOpenState(next);
    if (!next) setName("");
  }

  async function handleSubmit() {
    if (!canSubmit) return;
    // The create hook refetches the Environments list by name, so the new
    // environment appears in the panel once this closes — no callback needed.
    const id = await create(name.trim());
    if (id) handleOpenChange(false);
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      {controlled ? null : (
        <DialogTrigger asChild>
          <Button variant="outline">
            <Plus />
            {t("environments.newButton")}
          </Button>
        </DialogTrigger>
      )}
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("environments.createTitle")}</DialogTitle>
          <DialogDescription>
            {t("environments.createDescription")}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-2">
          <Label htmlFor="environment-name">{t("environments.fieldName")}</Label>
          <Input
            id="environment-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={t("environments.fieldNamePlaceholder")}
            autoComplete="off"
            onKeyDown={(e) => {
              if (e.key === "Enter") void handleSubmit();
            }}
          />
        </div>

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => handleOpenChange(false)}
            disabled={busy}
          >
            {t("environments.cancel")}
          </Button>
          <Button onClick={() => void handleSubmit()} disabled={!canSubmit}>
            {busy ? <Loader2 className="animate-spin" /> : null}
            {t("environments.createSubmit")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
