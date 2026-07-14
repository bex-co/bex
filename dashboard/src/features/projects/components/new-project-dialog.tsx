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
import { useCreateProject } from "@/features/projects/hooks/use-create-project";

export interface NewProjectDialogProps {
  /** Called once the project has been created (refetch the projects list). */
  onCreated: (id: string) => void;
  /**
   * Controlled variant: when both are provided, the dialog's open state is
   * driven externally (the unified Projects page's single segmented "+ New"
   * menu, matching render.com) and it renders no trigger of its own. Omitted,
   * it manages its own state behind its default trigger button.
   */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}

/**
 * "New Project" on the unified Projects page (Render parity — w1/m31). A
 * project only groups existing resources (assigned from their own row's
 * "Move to project" menu), so this dialog just collects a name. Self-contained
 * by default (owns its trigger button and resets on close); pass
 * `open`/`onOpenChange` to drive it externally instead — mirrors
 * CreateDatabaseDialog's controlled-variant shape.
 */
export function NewProjectDialog({
  onCreated,
  open: openProp,
  onOpenChange: onOpenChangeProp,
}: NewProjectDialogProps) {
  const { t } = useTranslations();
  const { create, busy } = useCreateProject();
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
    const id = await create(name.trim());
    if (id) {
      handleOpenChange(false);
      onCreated(id);
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      {controlled ? null : (
        <DialogTrigger asChild>
          <Button variant="outline">
            <Plus />
            {t("projects.newProjectButton")}
          </Button>
        </DialogTrigger>
      )}
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("projects.createTitle")}</DialogTitle>
          <DialogDescription>{t("projects.createDescription")}</DialogDescription>
        </DialogHeader>

        <div className="space-y-2">
          <Label htmlFor="project-name">{t("projects.fieldName")}</Label>
          <Input
            id="project-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={t("projects.fieldNamePlaceholder")}
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
            {t("projects.createCancel")}
          </Button>
          <Button onClick={() => void handleSubmit()} disabled={!canSubmit}>
            {busy ? <Loader2 className="animate-spin" /> : null}
            {t("projects.createSubmit")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
