import { useState } from "react";
import { Loader2, Pause, Play, RotateCw, Trash2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { useDeleteDatabase } from "@/features/databases/hooks/use-delete-database";
import { isSuspended } from "@/features/databases/lib/status";
import {
  DeleteDatabaseDialog,
  DatabaseLifecycleConfirmDialog,
} from "@/features/databases/components/database-confirm-dialogs";
import type {
  DatabaseLifecycleAction,
  UseDatabaseLifecycleResult,
} from "@/features/databases/hooks/use-database-lifecycle";
import type { DatabaseView } from "@/features/databases/types";
import { ProtectedConfirmationDialog } from "@/common/components/protected-confirmation-dialog";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";

export interface DatabaseDangerActionsProps {
  database: DatabaseView;
  /** Called after a successful delete (leave the detail page). */
  onDeleted: (id: string) => void;
  lifecycle: UseDatabaseLifecycleResult;
}

/**
 * The detail page's bottom action row — Render parity: the database Info page
 * ends with "Delete Database" / "Restart Database" / "Suspend Database"
 * (resume replaces suspend while hibernated). Same verbs and confirm gates as
 * the header dropdown (DatabaseRowActions); this is the discoverable placement.
 */
export function DatabaseDangerActions({
  database,
  onDeleted,
  lifecycle,
}: DatabaseDangerActionsProps) {
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
  const { remove, deleting } = useDeleteDatabase();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [confirmVerb, setConfirmVerb] = useState<Extract<
    DatabaseLifecycleAction,
    "suspend" | "restart"
  > | null>(null);
  const [protectedConfirm, setProtectedConfirm] = useState<{
    action: "delete" | DatabaseLifecycleAction;
    confirmation: string;
  } | null>(null);

  const lifecycleBusy = lifecycle.pending?.id === database.id;
  const busy = deleting === database.id || lifecycleBusy;
  const suspended = isSuspended(database);

  async function handleDelete(confirmation?: string) {
    if (createDenied) return;
    const result = confirmation
      ? await remove(database.id, database.name, confirmation)
      : await remove(database.id, database.name);
    if (result.status === "confirmation_required") {
      setConfirmOpen(false);
      setProtectedConfirm({
        action: "delete",
        confirmation: result.confirmation,
      });
    } else if (result.status === "success") {
      setConfirmOpen(false);
      setProtectedConfirm(null);
      onDeleted(database.id);
    }
  }

  async function runLifecycle(
    action: DatabaseLifecycleAction,
    confirmation?: string,
  ) {
    if (operateDenied) return;
    const result = confirmation
      ? await lifecycle.run(action, database, confirmation)
      : await lifecycle.run(action, database);
    if (result.status === "confirmation_required") {
      setProtectedConfirm({ action, confirmation: result.confirmation });
    } else if (result.status === "success") {
      setProtectedConfirm(null);
    }
  }

  return (
    <div className="flex flex-wrap items-center gap-3">
      <PermissionTooltip reason={createReason}>
        <Button
          variant="destructive"
          disabled={busy || createDenied}
          onClick={() => {
            if (!createDenied) setConfirmOpen(true);
          }}
        >
          {busy && deleting === database.id ? (
            <Loader2 className="animate-spin" />
          ) : (
            <Trash2 />
          )}
          {t("databases.dangerDelete")}
        </Button>
      </PermissionTooltip>
      <PermissionTooltip reason={!suspended ? operateReason : undefined}>
        <Button
          variant="ghost"
          className="text-destructive hover:text-destructive"
          disabled={busy || suspended || operateDenied}
          onClick={() => {
            if (!operateDenied) setConfirmVerb("restart");
          }}
        >
          <RotateCw />
          {t("databases.dangerRestart")}
        </Button>
      </PermissionTooltip>
      {suspended ? (
        <PermissionTooltip reason={operateReason}>
          <Button
            variant="ghost"
            disabled={busy || operateDenied}
            onClick={() => void runLifecycle("resume")}
          >
            {lifecycleBusy ? <Loader2 className="animate-spin" /> : <Play />}
            {t("databases.dangerResume")}
          </Button>
        </PermissionTooltip>
      ) : (
        <PermissionTooltip reason={operateReason}>
          <Button
            variant="ghost"
            className="text-destructive hover:text-destructive"
            disabled={busy || operateDenied}
            onClick={() => {
              if (!operateDenied) setConfirmVerb("suspend");
            }}
          >
            <Pause />
            {t("databases.dangerSuspend")}
          </Button>
        </PermissionTooltip>
      )}

      <DeleteDatabaseDialog
        database={database}
        open={confirmOpen && !createDenied}
        onOpenChange={(open) => {
          if (!createDenied) setConfirmOpen(open);
        }}
        busy={busy}
        onConfirm={() => void handleDelete()}
      />

      <DatabaseLifecycleConfirmDialog
        database={database}
        verb={operateDenied ? null : confirmVerb}
        busy={lifecycleBusy}
        onClose={() => setConfirmVerb(null)}
        onConfirm={(verb) => void runLifecycle(verb)}
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
        resourceName={database.name}
        requiredConfirmation={protectedConfirm?.confirmation ?? ""}
        actionLabel={
          protectedConfirm?.action === "delete"
            ? t("databases.deleteConfirm")
            : protectedConfirm?.action === "suspend"
              ? t("databases.actionSuspend")
              : protectedConfirm?.action === "resume"
                ? t("databases.actionResume")
                : t("databases.actionRestart")
        }
        busy={busy}
        onOpenChange={(open) => !open && setProtectedConfirm(null)}
        onConfirm={(confirmation) =>
          protectedConfirm?.action === "delete"
            ? handleDelete(confirmation)
            : protectedConfirm
              ? runLifecycle(protectedConfirm.action, confirmation)
              : Promise.resolve()
        }
      />
    </div>
  );
}
