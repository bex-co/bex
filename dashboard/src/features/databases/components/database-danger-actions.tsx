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
  const { remove, deleting } = useDeleteDatabase();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [confirmVerb, setConfirmVerb] = useState<
    Extract<DatabaseLifecycleAction, "suspend" | "restart"> | null
  >(null);

  const lifecycleBusy = lifecycle.pending?.id === database.id;
  const busy = deleting === database.id || lifecycleBusy;
  const suspended = isSuspended(database);

  async function handleDelete() {
    const ok = await remove(database.id, database.name);
    if (ok) {
      setConfirmOpen(false);
      onDeleted(database.id);
    }
  }

  async function runLifecycle(action: DatabaseLifecycleAction) {
    await lifecycle.run(action, database);
  }

  return (
    <div className="flex flex-wrap items-center gap-3">
      <Button
        variant="destructive"
        disabled={busy}
        onClick={() => setConfirmOpen(true)}
      >
        {busy && deleting === database.id ? (
          <Loader2 className="animate-spin" />
        ) : (
          <Trash2 />
        )}
        {t("databases.dangerDelete")}
      </Button>
      <Button
        variant="ghost"
        className="text-destructive hover:text-destructive"
        disabled={busy || suspended}
        onClick={() => setConfirmVerb("restart")}
      >
        <RotateCw />
        {t("databases.dangerRestart")}
      </Button>
      {suspended ? (
        <Button
          variant="ghost"
          disabled={busy}
          onClick={() => void runLifecycle("resume")}
        >
          {lifecycleBusy ? <Loader2 className="animate-spin" /> : <Play />}
          {t("databases.dangerResume")}
        </Button>
      ) : (
        <Button
          variant="ghost"
          className="text-destructive hover:text-destructive"
          disabled={busy}
          onClick={() => setConfirmVerb("suspend")}
        >
          <Pause />
          {t("databases.dangerSuspend")}
        </Button>
      )}

      <DeleteDatabaseDialog
        database={database}
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
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
    </div>
  );
}
