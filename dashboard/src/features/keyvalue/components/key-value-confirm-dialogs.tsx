import { useState } from "react";
import { Loader2 } from "lucide-react";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
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
    // AlertDialog via ConfirmDialog, matching SuspendKeyValueDialog below: a
    // delete that destroys data must announce as `alertdialog` and refuse
    // outside-click dismissal, so a stray click cannot discard a half-typed
    // sudo phrase (w7/053). The gate stays the caller's — the shared
    // SudoCommandField is the house sudo control — so it rides
    // confirmDisabled rather than the primitive's own `phrase` prop.
    <ConfirmDialog
      open={open}
      onOpenChange={handleOpenChange}
      title={t("keyvalue.deleteConfirmTitle")}
      description={t("keyvalue.deleteConfirmBody")}
      cancelLabel={t("keyvalue.deleteCancel")}
      confirmLabel={
        <>
          {busy ? <Loader2 className="animate-spin" /> : null}
          {t("keyvalue.deleteConfirm")}
        </>
      }
      confirmDisabled={!canDelete}
      pending={busy}
      onConfirm={() => void onConfirm()}
    >
      <SudoCommandField
        id="kv-delete-confirm"
        promptKey="keyvalue.deleteConfirmPrompt"
        phrase={confirmPhrase}
        value={typed}
        onValueChange={setTyped}
      />
    </ConfirmDialog>
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
    <ConfirmDialog
      open={open}
      onOpenChange={handleOpenChange}
      title={t("keyvalue.confirmSuspendTitle")}
      description={
        <>
          <span className="block">{t("keyvalue.confirmSuspendBody")}</span>
          <span className="mt-2 block">
            {t("keyvalue.confirmSuspendDetail", { name: keyValue.name })}
          </span>
        </>
      }
      cancelLabel={t("keyvalue.confirmCancel")}
      confirmLabel={t("keyvalue.actionSuspend")}
      // This dialog keeps the shared SudoCommandField rather than the
      // primitive's own `phrase` input — it is the house sudo control, used
      // identically by the datastore suspend flows — so the gate is the
      // caller's and rides confirmDisabled.
      confirmDisabled={!canSuspend}
      onConfirm={() => void onConfirm()}
    >
      <SudoCommandField
        id="kv-suspend-confirm"
        promptKey="keyvalue.confirmSuspendPrompt"
        phrase={confirmPhrase}
        value={confirmation}
        onValueChange={setConfirmation}
      />
    </ConfirmDialog>
  );
}
