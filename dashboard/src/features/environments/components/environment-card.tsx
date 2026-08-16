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
  toEnvGroupRow,
  toKeyValueRow,
  toServiceRow,
} from "@/features/projects/hooks/use-grouped-resources";
import {
  resourceSelectionKey,
  type ResourceRow,
} from "@/features/projects/types";
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
import type { EnvGroupView } from "@/features/env-groups/types";
import { ProjectResourceFilters } from "@/features/projects/components/project-resource-filters";
import {
  countProjectResources,
  filterProjectResources,
  type ProjectResourceFilterState,
} from "@/features/projects/lib/resource-filter";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { useSetEnvironmentServices } from "@/features/environments/hooks/use-set-environment-services";
import { useSetEnvironmentDatabases } from "@/features/environments/hooks/use-set-environment-databases";
import { useSetEnvironmentKeyValues } from "@/features/environments/hooks/use-set-environment-keyvalues";
import { useSetEnvironmentEnvGroups } from "@/features/environments/hooks/use-set-environment-env-groups";

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
  envGroups?: EnvGroupView[];
  allEnvironments?: EnvironmentView[];
  resourceFilter?: ProjectResourceFilterState;
  onResourceFilterChange?: (filter: ProjectResourceFilterState) => void;
}

/**
 * One environment rendered as a card: its name with rename/delete affordances
 * and a "Manage resources" action, over a reused `ResourceTable` of the
 * services, databases, key-value instances, and Env Groups assigned to it.
 * The Environment's full management dialogs remain here alongside the selected
 * operating view (docs/ADR032-environments.md).
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
  envGroups = [],
  allEnvironments = [environment],
  resourceFilter = {
    environmentId: environment.id,
    query: "",
    kind: "all",
  },
  onResourceFilterChange = () => undefined,
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
  const [selectedKeys, setSelectedKeys] = useState<Set<string>>(
    () => new Set(),
  );
  const [moveTargetId, setMoveTargetId] = useState("");
  const { setServices, busyId: servicesBusyId } = useSetEnvironmentServices();
  const { setDatabases, busyId: databasesBusyId } =
    useSetEnvironmentDatabases();
  const { setKeyValues, busyId: keyValuesBusyId } =
    useSetEnvironmentKeyValues();
  const { setEnvGroups, busyId: envGroupsBusyId } =
    useSetEnvironmentEnvGroups();

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
    const envGroupById = new Map(envGroups.map((group) => [group.id, group]));
    const envGroupRows = environment.envGroupIds
      .map((id) => envGroupById.get(id))
      .filter((group): group is EnvGroupView => group != null)
      .map(toEnvGroupRow);
    return [...serviceRows, ...databaseRows, ...keyValueRows, ...envGroupRows];
  }, [
    environment.serviceIds,
    environment.databaseIds,
    environment.keyValueIds,
    services,
    databases,
    keyValues,
    environment.envGroupIds,
    envGroups,
  ]);
  const visibleRows = filterProjectResources(rows, resourceFilter);
  const counts = countProjectResources(rows);
  const moveTargets = allEnvironments.filter(
    (candidate) => candidate.id !== environment.id,
  );
  const moving = [
    servicesBusyId,
    databasesBusyId,
    keyValuesBusyId,
    envGroupsBusyId,
  ].some((id) => id === moveTargetId);

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

  async function handleMove() {
    const target = moveTargets.find(
      (candidate) => candidate.id === moveTargetId,
    );
    if (!target || selectedKeys.size === 0) return;

    const selectedRows = rows.filter((row) =>
      selectedKeys.has(resourceSelectionKey(row)),
    );
    // One entry per resource kind: which of the target's ids that kind already
    // holds, and the setter that replaces them. All four setters share the same
    // (id, name, ids) => Promise<boolean> shape, so the move is one loop.
    const kinds: Array<{
      kind: ResourceRow["kind"];
      current: string[];
      set: (id: string, name: string, ids: string[]) => Promise<boolean>;
    }> = [
      { kind: "service", current: target.serviceIds, set: setServices },
      { kind: "database", current: target.databaseIds, set: setDatabases },
      { kind: "keyvalue", current: target.keyValueIds, set: setKeyValues },
      { kind: "envgroup", current: target.envGroupIds, set: setEnvGroups },
    ];
    const operations = kinds.flatMap(({ kind, current, set }) => {
      const moving = selectedRows.filter((row) => row.kind === kind);
      if (moving.length === 0) return [];
      return [
        {
          keys: moving.map(resourceSelectionKey),
          run: set(
            target.id,
            target.name,
            mergeIds(
              current,
              moving.map((row) => row.id),
            ),
          ),
        },
      ];
    });

    const results = await Promise.all(
      operations.map(async (operation) => ({
        keys: operation.keys,
        succeeded: await operation.run,
      })),
    );
    const succeeded = new Set(
      results
        .filter((result) => result.succeeded)
        .flatMap((result) => result.keys),
    );
    setSelectedKeys(
      new Set([...selectedKeys].filter((key) => !succeeded.has(key))),
    );
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
      <CardContent className="space-y-4">
        <ProjectResourceFilters
          environmentName={environment.name}
          filter={resourceFilter}
          counts={counts}
          onChange={(next) =>
            onResourceFilterChange({ ...next, environmentId: environment.id })
          }
        />
        {rows.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            {t("environments.cardEmpty")}
          </p>
        ) : visibleRows.length === 0 ? (
          <div className="py-6 text-center" role="status">
            <p className="font-medium">{t("projects.noMatchesTitle")}</p>
            <p className="mt-1 text-sm text-muted-foreground">
              {t("projects.noMatchesBody")}
            </p>
          </div>
        ) : (
          <div className="space-y-3">
            <div
              className="flex flex-wrap items-center gap-2"
              aria-live="polite"
            >
              <span className="text-sm text-muted-foreground">
                {t("projects.selectedCount", { count: selectedKeys.size })}
              </span>
              <Select value={moveTargetId} onValueChange={setMoveTargetId}>
                <SelectTrigger
                  className="w-48"
                  aria-label={t("projects.moveTargetLabel")}
                  disabled={moveTargets.length === 0 || moving}
                >
                  <SelectValue
                    placeholder={t("projects.moveTargetPlaceholder")}
                  />
                </SelectTrigger>
                <SelectContent>
                  {moveTargets.map((target) => (
                    <SelectItem key={target.id} value={target.id}>
                      {target.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button
                variant="outline"
                size="sm"
                disabled={selectedKeys.size === 0 || !moveTargetId || moving}
                onClick={() => void handleMove()}
              >
                {moving ? <Loader2 className="animate-spin" /> : null}
                {t("projects.moveSelected")}
              </Button>
            </div>
            <ResourceTable
              rows={visibleRows}
              servicePending={servicePending}
              onRunServiceAction={onRunServiceAction}
              onDatabaseDeleted={onDatabaseDeleted}
              onKeyValueDeleted={onKeyValueDeleted}
              projectMetadata
              selectedKeys={selectedKeys}
              onSelectedKeysChange={setSelectedKeys}
            />
          </div>
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

function mergeIds(existing: string[], additions: string[]): string[] {
  return [...new Set([...existing, ...additions])];
}
