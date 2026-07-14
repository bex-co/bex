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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { useTranslations } from "@/common/hooks/use-translations";
import { useDeleteDatabase } from "@/features/databases/hooks/use-delete-database";
import { isSuspended } from "@/features/databases/lib/status";
import { MoveToProjectMenu } from "@/features/projects/components/move-to-project-menu";
import type {
  DatabaseLifecycleAction,
  UseDatabaseLifecycleResult,
} from "@/features/databases/hooks/use-database-lifecycle";
import type { DatabaseView } from "@/features/databases/types";

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
  const [typed, setTyped] = useState("");
  // The disruptive lifecycle verb awaiting confirmation (suspend | restart).
  const [confirmVerb, setConfirmVerb] =
    useState<DatabaseLifecycleAction | null>(null);

  const lifecycleBusy = lifecycle?.pending?.id === database.id;
  const busy = deleting === database.id || lifecycleBusy;
  const canDelete = typed === database.name && !busy;
  const suspended = isSuspended(database);

  async function handleDelete() {
    const ok = await remove(database.id, database.name);
    if (ok) {
      setConfirmOpen(false);
      onDeleted(database.id);
    }
  }

  function handleOpenChange(next: boolean) {
    setConfirmOpen(next);
    if (!next) setTyped("");
  }

  async function runLifecycle(action: DatabaseLifecycleAction) {
    await lifecycle?.run(action, database);
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

      {/* Delete: typed-name confirm (destructive, irreversible). */}
      <Dialog open={confirmOpen} onOpenChange={handleOpenChange}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {t("databases.deleteConfirmTitle", { name: database.name })}
            </DialogTitle>
            <DialogDescription>
              {t("databases.deleteConfirmBody")}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="db-delete-confirm">
              {t("databases.deleteConfirmPrompt", { name: database.name })}
            </Label>
            <Input
              id="db-delete-confirm"
              value={typed}
              onChange={(e) => setTyped(e.target.value)}
              autoComplete="off"
              placeholder={database.name}
            />
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => handleOpenChange(false)}
              disabled={busy}
            >
              {t("databases.deleteCancel")}
            </Button>
            <Button
              variant="destructive"
              onClick={() => void handleDelete()}
              disabled={!canDelete}
            >
              {busy ? <Loader2 className="animate-spin" /> : null}
              {t("databases.deleteConfirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Suspend / restart: simple confirm (disruptive but reversible). */}
      <Dialog
        open={confirmVerb !== null}
        onOpenChange={(next) => !next && setConfirmVerb(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {confirmVerb === "suspend"
                ? t("databases.suspendConfirmTitle", { name: database.name })
                : t("databases.restartConfirmTitle", { name: database.name })}
            </DialogTitle>
            <DialogDescription>
              {confirmVerb === "suspend"
                ? t("databases.suspendConfirmBody")
                : t("databases.restartConfirmBody")}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setConfirmVerb(null)}
              disabled={lifecycleBusy}
            >
              {t("databases.deleteCancel")}
            </Button>
            <Button
              onClick={() => {
                const verb = confirmVerb;
                setConfirmVerb(null);
                if (verb) void runLifecycle(verb);
              }}
              disabled={lifecycleBusy}
            >
              {lifecycleBusy ? <Loader2 className="animate-spin" /> : null}
              {confirmVerb === "suspend"
                ? t("databases.actionSuspend")
                : t("databases.actionRestart")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
