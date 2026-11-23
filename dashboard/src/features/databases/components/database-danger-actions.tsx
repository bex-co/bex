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
import { useLifecycleGate } from "@/features/capabilities/hooks/use-lifecycle-gate";

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
 * Suspend and Resume gate on `databaseActions`, Restart on can_operate (w4/m578).
 */
export function DatabaseDangerActions({
  database,
  onDeleted,
  lifecycle,
}: DatabaseDangerActionsProps) {
  const { t } = useTranslations();
  const capabilities = useCapabilities();
  const { canCreate, canOperate } = capabilities;
  const createDenied = !canCreate;
  const operateDenied = !canOperate;
  const createReasonKey = capabilities.reasonKey("can_create");
  const operateReasonKey = capabilities.reasonKey("can_operate");
  const createReason = createReasonKey ? t(createReasonKey) : undefined;
  const operateReason = operateReasonKey ? t(operateReasonKey) : undefined;
  const { remove, deleting } = useDeleteDatabase();
  const suspended = isSuspended(database);
  const gate = useLifecycleGate("database", database.id, suspended);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [confirmVerb, setConfirmVerb] = useState<Extract<
    DatabaseLifecycleAction,
    "suspend" | "restart"
  > | null>(null);
  const [protectedConfirm, setProtectedConfirm] = useState<{
    action: "delete" | DatabaseLifecycleAction;
    confirmation: string;
  } | null>(null);

  const lifecycleBusy = lifecycle.pending?.id === database.id || gate.checking;
  const busy = deleting === database.id || lifecycleBusy;
  const lifecycleDenied = (action: DatabaseLifecycleAction) =>
    action === "restart" ? operateDenied : !!gate.reason(action);

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
    if (action === "restart" && operateDenied) return;
    const run = () =>
      confirmation
        ? lifecycle.run(action, database, confirmation)
        : lifecycle.run(action, database);
    const result =
      action === "restart" ? await run() : await gate.dispatch(action, run);
    if (result?.status === "confirmation_required") {
      setProtectedConfirm({ action, confirmation: result.confirmation });
    } else if (result?.status === "success") {
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
        <PermissionTooltip reason={gate.reason("resume")}>
          <Button
            variant="ghost"
            disabled={busy || lifecycleDenied("resume")}
            onClick={() => {
              if (!lifecycleDenied("resume")) void runLifecycle("resume");
            }}
          >
            {lifecycleBusy ? <Loader2 className="animate-spin" /> : <Play />}
            {t("databases.dangerResume")}
          </Button>
        </PermissionTooltip>
      ) : (
        <PermissionTooltip reason={gate.reason("suspend")}>
          <Button
            variant="ghost"
            className="text-destructive hover:text-destructive"
            disabled={busy || lifecycleDenied("suspend")}
            onClick={() => {
              if (!lifecycleDenied("suspend")) setConfirmVerb("suspend");
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
        verb={confirmVerb}
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
