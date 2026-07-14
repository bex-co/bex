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
import { useEnvGroupMutations } from "@/features/env-groups/hooks/use-env-groups";
import { isValidEnvGroupName } from "@/features/env-groups/lib/validation";

export interface NewEnvGroupDialogProps {
  onCreated: (id: string) => void;
  refetch?: () => Promise<unknown>;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}

/** Workspace-level create dialog; success hands the stable id to the route. */
export function NewEnvGroupDialog({
  onCreated,
  refetch,
  open: openProp,
  onOpenChange: onOpenChangeProp,
}: NewEnvGroupDialogProps) {
  const { t } = useTranslations();
  const { createGroup, busy } = useEnvGroupMutations(refetch);
  const controlled = openProp !== undefined;
  const [openState, setOpenState] = useState(false);
  const [name, setName] = useState("");
  const [invalid, setInvalid] = useState(false);
  const open = controlled ? openProp : openState;
  const canSubmit = isValidEnvGroupName(name) && !busy;

  function handleOpenChange(next: boolean) {
    if (controlled) onOpenChangeProp?.(next);
    else setOpenState(next);
    if (!next) {
      setName("");
      setInvalid(false);
    }
  }

  async function handleSubmit() {
    if (!isValidEnvGroupName(name)) {
      setInvalid(true);
      return;
    }
    const id = await createGroup(name.trim());
    if (!id) return;
    handleOpenChange(false);
    onCreated(id);
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      {controlled ? null : (
        <DialogTrigger asChild>
          <Button size="sm">
            <Plus />
            {t("envGroups.newButton")}
          </Button>
        </DialogTrigger>
      )}
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("envGroups.createTitle")}</DialogTitle>
          <DialogDescription>
            {t("envGroups.createDescription")}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-2">
          <Label htmlFor="env-group-name">{t("envGroups.nameLabel")}</Label>
          <Input
            id="env-group-name"
            value={name}
            onChange={(event) => {
              setName(event.target.value);
              setInvalid(false);
            }}
            placeholder={t("envGroups.namePlaceholder")}
            aria-invalid={invalid}
            autoComplete="off"
            onKeyDown={(event) => {
              if (event.key === "Enter") void handleSubmit();
            }}
          />
          {invalid ? (
            <p className="text-destructive text-sm">
              {t("envGroups.invalidName")}
            </p>
          ) : null}
        </div>
        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => handleOpenChange(false)}
            disabled={busy}
          >
            {t("envGroups.cancel")}
          </Button>
          <Button onClick={() => void handleSubmit()} disabled={!canSubmit}>
            {busy ? <Loader2 className="animate-spin" /> : null}
            {t("envGroups.createSubmit")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
