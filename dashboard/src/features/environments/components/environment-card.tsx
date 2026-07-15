import { useMemo, useState } from "react";
import {
  Loader2,
  LockKeyhole,
  MoreVertical,
  Pencil,
  Settings2,
  ShieldCheck,
  Trash2,
} from "lucide-react";
import {
  Card,
  CardAction,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { Badge } from "@/common/components/ui/badge";
import { Input } from "@/common/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/common/components/ui/alert-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import { ResourceTable } from "@/features/projects/components/resource-table";
import {
  toDatabaseRow,
  toKeyValueRow,
  toServiceRow,
} from "@/features/projects/hooks/use-grouped-resources";
import type { ResourceRow } from "@/features/projects/types";
import { useRenameEnvironment } from "@/features/environments/hooks/use-rename-environment";
import { useDeleteEnvironment } from "@/features/environments/hooks/use-delete-environment";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import { ManageResourcesDialog } from "@/features/environments/components/manage-resources-dialog";
import { EnvironmentSettingsDialog } from "@/features/environments/components/environment-settings-dialog";
import type { ServiceView } from "@/features/services/types";
import type {
  PendingLifecycle,
  RunServiceAction,
} from "@/features/services/hooks/use-service-lifecycle";
import type { DatabaseView } from "@/features/databases/types";
import type { KeyValueView } from "@/features/keyvalue/types";

export interface EnvironmentCardProps {
  environment: EnvironmentView;
  /** All the workspace's services — used to resolve this env's rows and as assign candidates. */
  services: ServiceView[];
  /** All the workspace's databases — same role as `services` (w6/m20 extension). */
  databases: DatabaseView[];
  /** All the workspace's key-value instances — same role as `services` (w6/m20 extension). */
  keyValues: KeyValueView[];
  servicePending: PendingLifecycle | null;
  onRunServiceAction: RunServiceAction;
  onDatabaseDeleted: (id: string) => void;
  onKeyValueDeleted: (id: string) => void;
}

/**
 * One environment rendered as a card: its name with rename/delete affordances
 * and a "Manage resources" action, over a reused `ResourceTable` of the
 * services, databases, and key-value instances assigned to it (w6/m20
 * extension — an environment groups all three resource types, matching what
 * Projects already did; see docs/ADR032-environments.md).
 */
export function EnvironmentCard({
  environment,
  services,
  databases,
  keyValues,
  servicePending,
  onRunServiceAction,
  onDatabaseDeleted,
  onKeyValueDeleted,
}: EnvironmentCardProps) {
  const { t } = useTranslations();

  const { rename, busy: renaming } = useRenameEnvironment();
  const { remove, deleting } = useDeleteEnvironment();
  const deletingThis = deleting === environment.id;

  const [renameOpen, setRenameOpen] = useState(false);
  const [renameValue, setRenameValue] = useState("");
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [assignOpen, setAssignOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);

  const rows = useMemo((): ResourceRow[] => {
    const serviceById = new Map(services.map((s) => [s.id, s]));
    const databaseById = new Map(databases.map((d) => [d.id, d]));
    const keyValueById = new Map(keyValues.map((k) => [k.id, k]));
    const serviceRows = environment.serviceIds
      .map((id) => serviceById.get(id))
      .filter((s): s is ServiceView => s != null)
      .map(toServiceRow);
    const databaseRows = environment.databaseIds
      .map((id) => databaseById.get(id))
      .filter((d): d is DatabaseView => d != null)
      .map(toDatabaseRow);
    const keyValueRows = environment.keyValueIds
      .map((id) => keyValueById.get(id))
      .filter((k): k is KeyValueView => k != null)
      .map(toKeyValueRow);
    return [...serviceRows, ...databaseRows, ...keyValueRows];
  }, [
    environment.serviceIds,
    environment.databaseIds,
    environment.keyValueIds,
    services,
    databases,
    keyValues,
  ]);

  function openRename() {
    setRenameValue(environment.name);
    setRenameOpen(true);
  }

  async function handleRename() {
    const trimmed = renameValue.trim();
    if (!trimmed || trimmed === environment.name) {
      setRenameOpen(false);
      return;
    }
    // The hook refetches the Environments list, so closing is all that's left.
    const ok = await rename(environment.id, trimmed);
    if (ok) setRenameOpen(false);
  }

  async function handleDelete() {
    const ok = await remove(environment.id, environment.name);
    if (ok) setDeleteOpen(false);
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          {environment.name}
          {environment.protectedStatus === "protected" ? (
            <Badge variant="secondary">
              <LockKeyhole />
              {t("environments.protectedBadge")}
            </Badge>
          ) : null}
          <span className="text-xs font-normal text-muted-foreground">
            {t("environments.resourceCount", {
              count: rows.length,
            })}
          </span>
        </CardTitle>
        <CardAction className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => setAssignOpen(true)}
          >
            <Settings2 />
            {t("environments.manageButton")}
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={t("environments.moreActions")}
              >
                <MoreVertical />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={openRename}>
                <Pencil />
                {t("environments.renameAction")}
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => setSettingsOpen(true)}>
                <ShieldCheck />
                {t("environments.settingsAction")}
              </DropdownMenuItem>
              <DropdownMenuItem
                variant="destructive"
                onClick={() => setDeleteOpen(true)}
              >
                <Trash2 />
                {t("environments.deleteAction")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </CardAction>
      </CardHeader>
      <CardContent>
        {rows.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            {t("environments.cardEmpty")}
          </p>
        ) : (
          <ResourceTable
            rows={rows}
            servicePending={servicePending}
            onRunServiceAction={onRunServiceAction}
            onDatabaseDeleted={onDatabaseDeleted}
            onKeyValueDeleted={onKeyValueDeleted}
          />
        )}
      </CardContent>

      <Dialog open={renameOpen} onOpenChange={setRenameOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("environments.renameTitle")}</DialogTitle>
          </DialogHeader>
          <Input
            value={renameValue}
            onChange={(e) => setRenameValue(e.target.value)}
            autoComplete="off"
            onKeyDown={(e) => {
              if (e.key === "Enter") void handleRename();
            }}
          />
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setRenameOpen(false)}
              disabled={renaming}
            >
              {t("environments.cancel")}
            </Button>
            <Button
              onClick={() => void handleRename()}
              disabled={renaming || renameValue.trim().length === 0}
            >
              {renaming ? <Loader2 className="animate-spin" /> : null}
              {t("environments.renameSubmit")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("environments.deleteConfirmTitle", { name: environment.name })}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("environments.deleteConfirmBody")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deletingThis}>
              {t("environments.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={(e) => {
                e.preventDefault();
                void handleDelete();
              }}
              disabled={deletingThis}
            >
              {deletingThis ? <Loader2 className="animate-spin" /> : null}
              {t("environments.deleteAction")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <ManageResourcesDialog
        environment={environment}
        services={services}
        databases={databases}
        keyValues={keyValues}
        open={assignOpen}
        onOpenChange={setAssignOpen}
      />
      <EnvironmentSettingsDialog
        environment={environment}
        open={settingsOpen}
        onOpenChange={setSettingsOpen}
      />
    </Card>
  );
}
