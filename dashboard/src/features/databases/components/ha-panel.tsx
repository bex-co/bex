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
import { Loader2, ShieldCheck } from "lucide-react";
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import { FailoverDatabaseDocument } from "@/features/databases/api/operations";
import type { DatabaseDetailView } from "@/features/databases/types";

interface HAPanelProps {
  /** The detail view, already fetched by the parent route. */
  database: DatabaseDetailView;
  /** Called after a successful failover so the header re-reads current status. */
  refetch: () => void;
}

/**
 * The HA panel: shows whether HA is enabled, a failover button for HA databases
 * (with a confirmation dialog), and the named read-replica connection hosts.
 * Mirrors Render's HA surface (docs/render-artifacts/postgres-ha.md):
 *   - highAvailabilityEnabled: reflects the operator's live observed state
 *   - failoverDatabase mutation: sends POST /v1/postgres/{id}/failover intent
 *   - readReplicas: each named replica with its internal + external host
 */
export function HAPanel({ database, refetch }: HAPanelProps) {
  const { t } = useTranslations();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [failover, { loading }] = useMutation(FailoverDatabaseDocument);

  const hasContent =
    database.highAvailabilityEnabled || database.readReplicas.length > 0;

  async function handleFailover() {
    try {
      await failover({ variables: { id: database.id } });
      toast.success(t("databases.haFailoverSuccess", { name: database.name }));
      setConfirmOpen(false);
      refetch();
    } catch {
      toast.error(t("databases.haFailoverError", { name: database.name }));
    }
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
              <Badge variant={database.highAvailabilityEnabled ? "default" : "secondary"}>
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

          {/* Named read replicas */}
          {database.readReplicas.length > 0 ? (
            <div className="space-y-3">
              <p className="text-sm font-medium">
                {t("databases.haReadReplicas")}
              </p>
              <ul className="space-y-2">
                {database.readReplicas.map((r) => (
                  <li
                    key={r.name}
                    className="rounded-md border p-3 text-sm"
                  >
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
                  </li>
                ))}
              </ul>
            </div>
          ) : database.highAvailabilityEnabled ? null : (
            <p className="text-sm text-muted-foreground">
              {t("databases.haNotEnabled")}
            </p>
          )}

          {!hasContent ? null : null}
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
    </>
  );
}
