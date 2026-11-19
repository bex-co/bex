import { useMemo, useState } from "react";
import { Settings } from "lucide-react";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/common/components/ui/dialog";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";
import { ApiKeyPermissionsPicker } from "@/features/api-keys/components/api-key-permissions-picker";
import {
  PICKER_DIALOG_BODY_CLASS,
  PICKER_DIALOG_CLASS,
  SCROLLING_DIALOG_CLASS,
} from "@/features/api-keys/components/picker-dialog-layout";
import { useApiKeyPermissionCatalog } from "@/features/api-keys/hooks/use-api-key-permission-catalog";
import { useUpdateApiKeyPermissions } from "@/features/api-keys/hooks/use-update-api-key-permissions";
import {
  sameSelection,
  selectionFromPermissions,
  selectionToInput,
} from "@/features/api-keys/lib/permissions";
import type {
  ApiKeyView,
  PermissionSelection,
} from "@/features/api-keys/types";

export interface EditApiKeyPermissionsDialogProps {
  entry: ApiKeyView;
}

/**
 * The gear on a key's row (w9/m496), Render's "Click the gear icon next to the
 * key. Change your key's permissions, then click Save changes." It opens the
 * create dialog's picker on the key's current grant. bex-api decides who may
 * save: the key's creator or a workspace Admin, and never past the creator's
 * role; a refusal is toasted in its own words. A save's answer carries the
 * key's id and new grant, so Apollo updates the listed row itself.
 */
export function EditApiKeyPermissionsDialog({
  entry,
}: EditApiKeyPermissionsDialogProps) {
  const { t } = useTranslations();
  const [open, setOpen] = useState(false);

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button
          size="icon-sm"
          variant="ghost"
          aria-label={t("apiKeys.editPermissionsNamed", { name: entry.name })}
        >
          <Settings />
        </Button>
      </DialogTrigger>
      {/* A long key name wraps instead of widening the dialog. */}
      <DialogContent
        className={cn(
          SCROLLING_DIALOG_CLASS,
          PICKER_DIALOG_CLASS,
          "[overflow-wrap:anywhere]",
        )}
      >
        <EditPermissionsForm entry={entry} onSaved={() => setOpen(false)} />
      </DialogContent>
    </Dialog>
  );
}

/** The editor's body, mounted only while it is open. */
function EditPermissionsForm({
  entry,
  onSaved,
}: {
  entry: ApiKeyView;
  onSaved: () => void;
}) {
  const { t } = useTranslations();
  const catalog = useApiKeyPermissionCatalog();
  const { update, busy } = useUpdateApiKeyPermissions();
  const saved = useMemo(
    () => selectionFromPermissions(entry.permissions),
    [entry.permissions],
  );
  const [selection, setSelection] = useState<PermissionSelection>(saved);
  const changed = !sameSelection(selection, saved);

  async function handleSave() {
    if (!catalog.areas || !changed || busy) return;
    const ok = await update(
      entry.id,
      entry.name,
      selectionToInput(selection, catalog.areas),
    );
    if (ok) onSaved();
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>{t("apiKeys.editPermissionsTitle")}</DialogTitle>
        <DialogDescription>
          {t("apiKeys.editPermissionsDescription", { name: entry.name })}
        </DialogDescription>
      </DialogHeader>
      <div className={PICKER_DIALOG_BODY_CLASS}>
        <ApiKeyPermissionsPicker
          areas={catalog.areas}
          error={catalog.error}
          errorMessage={t("apiKeys.permCatalogErrorEdit")}
          refetch={catalog.refetch}
          value={selection}
          onChange={setSelection}
          disabled={busy}
        />
      </div>
      <DialogFooter>
        <DialogClose asChild>
          <Button variant="outline" disabled={busy}>
            {t("apiKeys.editPermissionsCancel")}
          </Button>
        </DialogClose>
        <Button
          onClick={() => void handleSave()}
          disabled={!catalog.areas || !changed}
          loading={busy}
        >
          {t("apiKeys.editPermissionsSave")}
        </Button>
      </DialogFooter>
    </>
  );
}
