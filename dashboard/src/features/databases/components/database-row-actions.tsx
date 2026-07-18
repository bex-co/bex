import { useState } from "react";
import { MoreHorizontal, Loader2, Pause, Play, RotateCw } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import { useTranslations } from "@/common/hooks/use-translations";
import { useDeleteDatabase } from "@/features/databases/hooks/use-delete-database";
import { isSuspended } from "@/features/databases/lib/status";
import {
  DeleteDatabaseDialog,
  DatabaseLifecycleConfirmDialog,
} from "@/features/databases/components/database-confirm-dialogs";
import { MoveToProjectMenu } from "@/features/projects/components/move-to-project-menu";
import type {
  DatabaseLifecycleAction,
  UseDatabaseLifecycleResult,
} from "@/features/databases/hooks/use-database-lifecycle";
import type { DatabaseView } from "@/features/databases/types";
import { ProtectedConfirmationDialog } from "@/common/components/protected-confirmation-dialog";

export interface DatabaseRowActionsProps {
  database: DatabaseView;
  /** Called after a successful delete (refetch the list / leave the detail page). */
  onDeleted: (id: string) => void;
  /** When present, the menu also offers suspend/resume/restart (detail page). */
  lifecycle?: UseDatabaseLifecycleResult;
}

/**
 * The per-database actions menu. Delete (a typed-name confirm, since it cascades
 * the CNPG cluster + PVC) is always available; when `lifecycle` is wired (the
 * detail page) the menu also offers suspend/resume/restart — resume runs
 * immediately, the disruptive verbs (suspend/restart) go through a confirm.
 */
export function DatabaseRowActions({
  database,
  onDeleted,
  lifecycle,
}: DatabaseRowActionsProps) {
  const { t } = useTranslations();
  const { remove, deleting } = useDeleteDatabase();
  const [confirmOpen, setConfirmOpen] = useState(false);
  // The disruptive lifecycle verb awaiting confirmation (suspend | restart).
  const [confirmVerb, setConfirmVerb] = useState<Extract<
    DatabaseLifecycleAction,
    "suspend" | "restart"
  > | null>(null);
  const [protectedConfirm, setProtectedConfirm] = useState<{
    action: "delete" | DatabaseLifecycleAction;
    confirmation: string;
  } | null>(null);

  const lifecycleBusy = lifecycle?.pending?.id === database.id;
  const busy = deleting === database.id || lifecycleBusy;
  const suspended = isSuspended(database);

  async function handleDelete(confirmation?: string) {
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
    const result = confirmation
      ? await lifecycle?.run(action, database, confirmation)
      : await lifecycle?.run(action, database);
    if (result?.status === "confirmation_required") {
      setProtectedConfirm({ action, confirmation: result.confirmation });
    } else if (result?.status === "success") {
      setProtectedConfirm(null);
    }
  }

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="icon-sm"
            disabled={busy}
            aria-label={t("databases.actionsMenu")}
          >
            {busy ? <Loader2 className="animate-spin" /> : <MoreHorizontal />}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          {lifecycle ? (
            <>
              {suspended ? (
                <DropdownMenuItem onSelect={() => void runLifecycle("resume")}>
                  <Play />
                  {t("databases.actionResume")}
                </DropdownMenuItem>
              ) : (
                <DropdownMenuItem onSelect={() => setConfirmVerb("suspend")}>
                  <Pause />
                  {t("databases.actionSuspend")}
                </DropdownMenuItem>
              )}
              <DropdownMenuItem
                onSelect={() => setConfirmVerb("restart")}
                disabled={suspended}
              >
                <RotateCw />
                {t("databases.actionRestart")}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
            </>
          ) : null}
          <DropdownMenuItem
            variant="destructive"
            onSelect={() => setConfirmOpen(true)}
          >
            {t("databases.actionDelete")}
          </DropdownMenuItem>
          <MoveToProjectMenu
            kind="database"
            resourceId={database.id}
            resourceName={database.name}
            disabled={busy}
          />
        </DropdownMenuContent>
      </DropdownMenu>

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

      <ProtectedConfirmationDialog
        key={
          protectedConfirm ? `open:${protectedConfirm.confirmation}` : "closed"
        }
        open={protectedConfirm !== null}
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
    </>
  );
}
