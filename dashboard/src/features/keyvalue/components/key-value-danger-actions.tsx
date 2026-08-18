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
import { ProtectedConfirmationDialog } from "@/common/components/protected-confirmation-dialog";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";

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
  const {
    canCreate,
    canOperate,
    loaded: capabilitiesLoaded,
  } = useCapabilities();
  const createDenied = capabilitiesLoaded && !canCreate;
  const operateDenied = capabilitiesLoaded && !canOperate;
  const createReason = createDenied
    ? t("capabilities.reasonCanCreate")
    : undefined;
  const operateReason = operateDenied
    ? t("capabilities.reasonCanOperate")
    : undefined;
  const { remove, deleting } = useDeleteKeyValue();
  const { pending, run } = useKeyValueLifecycle();
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [suspendOpen, setSuspendOpen] = useState(false);
  const [protectedConfirm, setProtectedConfirm] = useState<{
    action: "delete" | "suspend";
    confirmation: string;
  } | null>(null);

  const deleteBusy = deleting === keyValue.id;
  const busy = deleteBusy || pending !== null;

  async function handleDelete(confirmation?: string) {
    if (createDenied) return;
    const result = confirmation
      ? await remove(keyValue.id, keyValue.name, confirmation)
      : await remove(keyValue.id, keyValue.name);
    if (result.status === "confirmation_required") {
      setDeleteOpen(false);
      setProtectedConfirm({
        action: "delete",
        confirmation: result.confirmation,
      });
    } else if (result.status === "success") {
      setDeleteOpen(false);
      setProtectedConfirm(null);
      onDeleted(keyValue.id);
    }
  }

  async function handleSuspend(confirmation?: string) {
    if (operateDenied) return;
    setSuspendOpen(false);
    const result = confirmation
      ? await run("suspend", keyValue.id, keyValue.name, confirmation)
      : await run("suspend", keyValue.id, keyValue.name);
    if (result.status === "confirmation_required") {
      setProtectedConfirm({
        action: "suspend",
        confirmation: result.confirmation,
      });
    } else if (result.status === "success") {
      setProtectedConfirm(null);
      onChanged();
    }
  }

  async function handleResume() {
    if (operateDenied) return;
    const result = await run("resume", keyValue.id, keyValue.name);
    if (result.status === "success") onChanged();
  }

  return (
    <div className="flex flex-wrap items-center gap-3">
      <PermissionTooltip reason={createReason}>
        <Button
          variant="destructive"
          disabled={busy || createDenied}
          onClick={() => {
            if (!createDenied) setDeleteOpen(true);
          }}
        >
          {deleteBusy ? <Loader2 className="animate-spin" /> : <Trash2 />}
          {t("keyvalue.dangerDelete")}
        </Button>
      </PermissionTooltip>

      {keyValue.suspended ? (
        <PermissionTooltip reason={operateReason}>
          <Button
            variant="ghost"
            disabled={busy || operateDenied}
            onClick={() => void handleResume()}
          >
            {pending === "resume" ? (
              <Loader2 className="animate-spin" />
            ) : (
              <Play />
            )}
            {t("keyvalue.actionResume")}
          </Button>
        </PermissionTooltip>
      ) : (
        <PermissionTooltip reason={operateReason}>
          <Button
            variant="ghost"
            className="text-destructive hover:text-destructive"
            disabled={busy || operateDenied}
            onClick={() => {
              if (!operateDenied) setSuspendOpen(true);
            }}
          >
            {pending === "suspend" ? (
              <Loader2 className="animate-spin" />
            ) : (
              <Pause />
            )}
            {t("keyvalue.actionSuspend")}
          </Button>
        </PermissionTooltip>
      )}

      <DeleteKeyValueDialog
        keyValue={keyValue}
        open={deleteOpen && !createDenied}
        onOpenChange={(open) => {
          if (!createDenied) setDeleteOpen(open);
        }}
        busy={busy}
        onConfirm={handleDelete}
      />
      <SuspendKeyValueDialog
        keyValue={keyValue}
        open={suspendOpen && !operateDenied}
        onOpenChange={(open) => {
          if (!operateDenied) setSuspendOpen(open);
        }}
        busy={busy}
        onConfirm={handleSuspend}
      />
      <ProtectedConfirmationDialog
        key={
          protectedConfirm ? `open:${protectedConfirm.confirmation}` : "closed"
        }
        open={
          protectedConfirm !== null &&
          (protectedConfirm.action === "delete"
            ? !createDenied
            : !operateDenied)
        }
        resourceName={keyValue.name}
        requiredConfirmation={protectedConfirm?.confirmation ?? ""}
        actionLabel={
          protectedConfirm?.action === "delete"
            ? t("keyvalue.deleteConfirm")
            : t("keyvalue.actionSuspend")
        }
        busy={busy}
        onOpenChange={(open) => !open && setProtectedConfirm(null)}
        onConfirm={(confirmation) =>
          protectedConfirm?.action === "delete"
            ? handleDelete(confirmation)
            : handleSuspend(confirmation)
        }
      />
    </div>
  );
}
