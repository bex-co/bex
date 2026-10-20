/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { useState } from "react";
import { Loader2, Plus, ShieldCheck } from "lucide-react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { Badge } from "@/common/components/ui/badge";
import { TextField } from "@/common/components/text-field";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import {
  FailoverDatabaseDocument,
  SetDatabaseReadReplicasDocument,
} from "@/graphql/definitions";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useDatabaseInstanceTypes } from "@/features/databases/hooks/use-database-instance-types";
import {
  READ_REPLICA_NAME_MAX_LENGTH,
  readReplicaAddBlock,
  readReplicaNameProblem,
  type ReadReplicaAddBlock,
} from "@/features/databases/lib/read-replicas";
import type { DatabaseDetailView } from "@/features/databases/types";

interface HAPanelProps {
  /** The detail view, already fetched by the parent route. */
  database: DatabaseDetailView;
  /** Called after a successful failover so the header re-reads current status. */
  refetch: () => void;
}

/**
 * The HA panel: shows whether HA is enabled, a failover button for HA databases
 * (with a confirmation dialog), and the named read replicas, which a member
 * who can operate the database may add and remove (w5/m138).
 * Mirrors Render's HA surface (docs/render-artifacts/postgres-ha.md):
 *   - highAvailabilityEnabled: reflects the operator's live observed state
 *   - failoverDatabase mutation: sends POST /v1/postgres/{id}/failover intent
 *   - readReplicas: each named replica with its internal + external host;
 *     setDatabaseReadReplicas replaces the set (REST's PATCH readReplicas)
 */
export function HAPanel({ database, refetch }: HAPanelProps) {
  const { t } = useTranslations();
  const capabilities = useCapabilities();
  const operateReasonKey = capabilities.reasonKey("can_operate");
  const operateReason = operateReasonKey ? t(operateReasonKey) : undefined;
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [adding, setAdding] = useState(false);
  const [newName, setNewName] = useState("");
  const [removing, setRemoving] = useState<string | null>(null);
  const [failover, { loading }] = useMutation(FailoverDatabaseDocument);
  const [setReplicas, { loading: savingReplicas }] = useMutation(
    SetDatabaseReadReplicasDocument,
  );
  const { instanceTypes } = useDatabaseInstanceTypes();
  const plan = instanceTypes.find((it) => it.id === database.plan);
  // Unknown while the catalog loads: keep the neutral "not enabled" line.
  const planOffersHA = plan?.supportsHighAvailability ?? true;

  const names = database.readReplicas.map((r) => r.name);
  const addBlock = readReplicaAddBlock(database, plan);
  const addDisabledReason =
    operateReason ?? (addBlock ? addBlockMessage(t, addBlock) : undefined);
  const nameProblem = readReplicaNameProblem(newName, names);
  // An empty field shows the hint, not an error; submit stays disabled.
  const nameError = !newName
    ? undefined
    : nameProblem === "duplicate"
      ? t("databases.haReplicaNameDuplicate", { name: newName })
      : nameProblem === "invalid"
        ? t("databases.haReplicaNameInvalid", {
            max: READ_REPLICA_NAME_MAX_LENGTH,
          })
        : undefined;

  async function handleFailover() {
    try {
      await failover({ variables: { id: database.id } });
      toast.success(t("databases.haFailoverSuccess", { name: database.name }));
      setConfirmOpen(false);
      refetch();
    } catch (err) {
      toast.error(
        mutationErrorMessage(
          err,
          t("databases.haFailoverError", { name: database.name }),
        ),
      );
    }
  }

  // The whole replica set travels on every change (Render's PATCH semantics):
  // the current names plus the one added, or minus the one removed. The
  // mutation answers the new set and status into the cache, so no refetch.
  async function saveReplicas(
    next: string[],
    success: string,
    failure: string,
  ): Promise<boolean> {
    try {
      await setReplicas({
        variables: {
          id: database.id,
          readReplicas: next.map((name) => ({ name })),
        },
      });
      toast.success(success);
      return true;
    } catch (err) {
      toast.error(mutationErrorMessage(err, failure));
      return false;
    }
  }

  async function handleAdd() {
    if (nameProblem) return;
    const saved = await saveReplicas(
      [...names, newName],
      t("databases.haAddReplicaSuccess", { name: newName }),
      t("databases.haAddReplicaError", { name: newName }),
    );
    if (saved) setAdding(false);
  }

  async function handleRemove(name: string) {
    const saved = await saveReplicas(
      names.filter((n) => n !== name),
      t("databases.haRemoveReplicaSuccess", { name }),
      t("databases.haRemoveReplicaError", { name }),
    );
    if (saved) setRemoving(null);
  }

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle>{t("databases.haTitle")}</CardTitle>
          <CardDescription>{t("databases.haDescription")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* HA status row */}
          <div className="flex items-center justify-between gap-4">
            <div className="flex items-center gap-2">
              <ShieldCheck className="h-4 w-4 text-muted-foreground" />
              <span className="text-sm font-medium">
                {t("databases.haStatus")}
              </span>
              <Badge
                variant={
                  database.highAvailabilityEnabled ? "default" : "secondary"
                }
              >
                {database.highAvailabilityEnabled
                  ? t("databases.haEnabled")
                  : t("databases.haDisabled")}
              </Badge>
            </div>
            {database.highAvailabilityEnabled ? (
              <Button
                variant="outline"
                size="sm"
                onClick={() => setConfirmOpen(true)}
                disabled={loading}
              >
                {loading ? <Loader2 className="animate-spin" /> : null}
                {t("databases.haFailover")}
              </Button>
            ) : null}
          </div>

          {database.highAvailabilityEnabled ? null : (
            <p className="text-sm text-muted-foreground">
              {planOffersHA
                ? t("databases.haNotEnabled")
                : t("databases.haPlanUnsupported")}
            </p>
          )}

          {/* Named read replicas */}
          <div className="space-y-3">
            <div className="flex items-center justify-between gap-4">
              <p className="text-sm font-medium">
                {t("databases.haReadReplicas")}
              </p>
              <PermissionTooltip reason={addDisabledReason}>
                <Button
                  variant="outline"
                  size="sm"
                  disabled={addDisabledReason != null || savingReplicas}
                  onClick={() => {
                    setNewName("");
                    setAdding(true);
                  }}
                >
                  <Plus />
                  {t("databases.haAddReplica")}
                </Button>
              </PermissionTooltip>
            </div>
            {database.readReplicas.length > 0 ? (
              <ul className="space-y-2">
                {database.readReplicas.map((r) => (
                  <li
                    key={r.name}
                    className="flex items-start justify-between gap-4 rounded-md border p-3 text-sm"
                  >
                    <div className="min-w-0">
                      <p className="font-medium">{r.name}</p>
                      {r.connectionInfo ? (
                        <div className="mt-1 space-y-1 text-xs text-muted-foreground">
                          {r.connectionInfo.internalHost ? (
                            <div className="flex gap-2">
                              <span className="shrink-0 font-medium">
                                {t("databases.haReplicaInternal")}
                              </span>
                              <code className="truncate font-mono">
                                {r.connectionInfo.internalHost}
                              </code>
                            </div>
                          ) : null}
                          {r.connectionInfo.externalHost ? (
                            <div className="flex gap-2">
                              <span className="shrink-0 font-medium">
                                {t("databases.haReplicaExternal")}
                              </span>
                              <code className="truncate font-mono">
                                {r.connectionInfo.externalHost}
                              </code>
                            </div>
                          ) : null}
                        </div>
                      ) : null}
                    </div>
                    <PermissionTooltip reason={operateReason}>
                      <Button
                        variant="ghost"
                        size="sm"
                        disabled={operateReason != null || savingReplicas}
                        onClick={() => setRemoving(r.name)}
                      >
                        {t("databases.haRemoveReplica")}
                      </Button>
                    </PermissionTooltip>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-sm text-muted-foreground">
                {t("databases.haReadReplicasEmpty")}
              </p>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Failover confirmation dialog */}
      <Dialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {t("databases.haFailoverConfirmTitle", { name: database.name })}
            </DialogTitle>
            <DialogDescription>
              {t("databases.haFailoverConfirmBody")}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmOpen(false)}>
              {t("databases.haFailoverCancel")}
            </Button>
            <Button onClick={() => void handleFailover()} disabled={loading}>
              {loading ? <Loader2 className="animate-spin" /> : null}
              {t("databases.haFailoverConfirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Add-replica dialog */}
      <Dialog
        open={adding}
        onOpenChange={(open) => {
          if (!savingReplicas) setAdding(open);
        }}
      >
        <DialogContent>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void handleAdd();
            }}
            className="space-y-4"
          >
            <DialogHeader>
              <DialogTitle>{t("databases.haAddReplicaTitle")}</DialogTitle>
              <DialogDescription>
                {t("databases.haAddReplicaBody")}
              </DialogDescription>
            </DialogHeader>
            <TextField
              id="read-replica-name"
              label={t("databases.haReplicaNameLabel")}
              value={newName}
              onChange={setNewName}
              maxLength={READ_REPLICA_NAME_MAX_LENGTH}
              hint={t("databases.haReplicaNameHint", {
                max: READ_REPLICA_NAME_MAX_LENGTH,
              })}
              error={nameError}
            />
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                disabled={savingReplicas}
                onClick={() => setAdding(false)}
              >
                {t("common.cancel")}
              </Button>
              <Button
                type="submit"
                disabled={nameProblem != null || savingReplicas}
              >
                {savingReplicas ? <Loader2 className="animate-spin" /> : null}
                {t("databases.haAddReplicaSubmit")}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Remove-replica confirmation */}
      <ConfirmDialog
        open={removing != null}
        onOpenChange={(open) => {
          if (!open && !savingReplicas) setRemoving(null);
        }}
        title={t("databases.haRemoveReplicaTitle", { name: removing ?? "" })}
        description={t("databases.haRemoveReplicaBody")}
        confirmLabel={t("databases.haRemoveReplicaConfirm")}
        pending={savingReplicas}
        closeOnConfirm={false}
        onConfirm={() => {
          if (removing) void handleRemove(removing);
        }}
      />
    </>
  );
}

function addBlockMessage(
  t: ReturnType<typeof useTranslations>["t"],
  block: ReadReplicaAddBlock,
): string {
  switch (block.kind) {
    case "plan":
      return t("databases.haReplicaPlanUnsupported");
    case "limit":
      return t("databases.haReplicaLimitReached", { max: block.max });
    case "storage":
      return t("databases.haReplicaStorageTooSmall", {
        min: block.min,
        current: block.current,
      });
  }
}
