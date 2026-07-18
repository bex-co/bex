import { useState } from "react";
import { Loader2, Pencil, Trash2 } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { SudoCommandField } from "@/common/components/sudo-command-field";
import { useTranslations } from "@/common/hooks/use-translations";
import { isValidEnvGroupName } from "@/features/env-groups/lib/validation";
import type { EnvGroupView } from "@/features/env-groups/types";

export function EnvGroupActions({
  group,
  renameGroup,
  deleteGroup,
  busy,
  onDeleted,
}: {
  group: EnvGroupView;
  renameGroup: (id: string, name: string) => Promise<boolean>;
  deleteGroup: (id: string) => Promise<boolean>;
  busy: boolean;
  onDeleted: () => void;
}) {
  const { t } = useTranslations();
  const [renameOpen, setRenameOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [name, setName] = useState(group.name);
  const [confirmation, setConfirmation] = useState("");
  const deletePhrase = `sudo delete env group ${group.name}`;

  async function handleRename() {
    if (!isValidEnvGroupName(name) || busy) return;
    if (await renameGroup(group.id, name.trim())) setRenameOpen(false);
  }

  async function handleDelete() {
    if (confirmation !== deletePhrase || busy) return;
    if (await deleteGroup(group.id)) {
      setDeleteOpen(false);
      onDeleted();
    }
  }

  return (
    <>
      <div className="flex items-center gap-2">
        <Button
          variant="outline"
          size="sm"
          onClick={() => {
            setName(group.name);
            setRenameOpen(true);
          }}
        >
          <Pencil />
          {t("envGroups.renameButton")}
        </Button>
        <Button
          variant="destructive"
          size="sm"
          onClick={() => setDeleteOpen(true)}
        >
          <Trash2 />
          {t("envGroups.deleteButton")}
        </Button>
      </div>

      <Dialog open={renameOpen} onOpenChange={setRenameOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("envGroups.renameTitle")}</DialogTitle>
            <DialogDescription>
              {t("envGroups.renameDescription")}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="env-group-rename">{t("envGroups.nameLabel")}</Label>
            <Input
              id="env-group-rename"
              value={name}
              onChange={(event) => setName(event.target.value)}
              aria-invalid={!isValidEnvGroupName(name)}
              autoComplete="off"
            />
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setRenameOpen(false)}
              disabled={busy}
            >
              {t("envGroups.cancel")}
            </Button>
            <Button
              onClick={() => void handleRename()}
              disabled={!isValidEnvGroupName(name) || busy}
            >
              {busy ? <Loader2 className="animate-spin" /> : null}
              {t("envGroups.renameSubmit")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={deleteOpen}
        onOpenChange={(open) => {
          setDeleteOpen(open);
          if (!open) setConfirmation("");
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {t("envGroups.deleteTitle", { name: group.name })}
            </DialogTitle>
            <DialogDescription>
              {t("envGroups.deleteDescription")}
            </DialogDescription>
          </DialogHeader>
          <SudoCommandField
            id="env-group-delete-confirm"
            promptKey="envGroups.deletePrompt"
            phrase={deletePhrase}
            value={confirmation}
            onValueChange={setConfirmation}
          />
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setDeleteOpen(false)}
              disabled={busy}
            >
              {t("envGroups.cancel")}
            </Button>
            <Button
              variant="destructive"
              onClick={() => void handleDelete()}
              disabled={confirmation !== deletePhrase || busy}
            >
              {busy ? <Loader2 className="animate-spin" /> : null}
              {t("envGroups.deleteConfirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
