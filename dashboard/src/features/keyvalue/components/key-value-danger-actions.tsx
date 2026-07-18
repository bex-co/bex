import { useState } from "react";
import { Loader2, Pause, Play, Trash2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  DeleteKeyValueDialog,
  SuspendKeyValueDialog,
} from "@/features/keyvalue/components/key-value-confirm-dialogs";
import { useDeleteKeyValue } from "@/features/keyvalue/hooks/use-delete-key-value";
import { useKeyValueLifecycle } from "@/features/keyvalue/hooks/use-key-value-lifecycle";
import type { KeyValueView } from "@/features/keyvalue/types";

export interface KeyValueDangerActionsProps {
  keyValue: KeyValueView;
  /** Called after a successful delete so the detail page can navigate away. */
  onDeleted: (id: string) => void;
  /** Called after a successful suspend/resume so the detail can refetch. */
  onChanged: () => void;
}

/**
 * Render's Key Value Info page ends with one discoverable action row: Delete
 * Key Value Instance plus Suspend (or Resume while hibernated). Delete and
 * suspend use typed confirmation dialogs; resume is immediate.
 */
export function KeyValueDangerActions({
  keyValue,
  onDeleted,
  onChanged,
}: KeyValueDangerActionsProps) {
  const { t } = useTranslations();
  const { remove, deleting } = useDeleteKeyValue();
  const { pending, run } = useKeyValueLifecycle();
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [suspendOpen, setSuspendOpen] = useState(false);

  const deleteBusy = deleting === keyValue.id;
  const busy = deleteBusy || pending !== null;

  async function handleDelete() {
    const ok = await remove(keyValue.id, keyValue.name);
    if (!ok) return;
    setDeleteOpen(false);
    onDeleted(keyValue.id);
  }

  async function handleSuspend() {
    setSuspendOpen(false);
    const ok = await run("suspend", keyValue.id, keyValue.name);
    if (ok) onChanged();
  }

  async function handleResume() {
    const ok = await run("resume", keyValue.id, keyValue.name);
    if (ok) onChanged();
  }

  return (
    <div className="flex flex-wrap items-center gap-3">
      <Button
        variant="destructive"
        disabled={busy}
        onClick={() => setDeleteOpen(true)}
      >
        {deleteBusy ? <Loader2 className="animate-spin" /> : <Trash2 />}
        {t("keyvalue.dangerDelete")}
      </Button>

      {keyValue.suspended ? (
        <Button
          variant="ghost"
          disabled={busy}
          onClick={() => void handleResume()}
        >
          {pending === "resume" ? (
            <Loader2 className="animate-spin" />
          ) : (
            <Play />
          )}
          {t("keyvalue.actionResume")}
        </Button>
      ) : (
        <Button
          variant="ghost"
          className="text-destructive hover:text-destructive"
          disabled={busy}
          onClick={() => setSuspendOpen(true)}
        >
          {pending === "suspend" ? (
            <Loader2 className="animate-spin" />
          ) : (
            <Pause />
          )}
          {t("keyvalue.actionSuspend")}
        </Button>
      )}

      <DeleteKeyValueDialog
        keyValue={keyValue}
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        busy={busy}
        onConfirm={handleDelete}
      />
      <SuspendKeyValueDialog
        keyValue={keyValue}
        open={suspendOpen}
        onOpenChange={setSuspendOpen}
        busy={busy}
        onConfirm={handleSuspend}
      />
    </div>
  );
}
