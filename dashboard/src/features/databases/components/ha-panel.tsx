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
import { Switch } from "@/common/components/ui/switch";
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
  UpdateDatabaseHighAvailabilityDocument,
} from "@/graphql/definitions";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilityGate } from "@/features/capabilities/hooks/use-capability-gate";
import { useDatabaseInstanceTypes } from "@/features/databases/hooks/use-database-instance-types";
import {
  highAvailabilityRequested,
  isSuspended,
} from "@/features/databases/lib/status";
import type { DatabaseDetailView } from "@/features/databases/types";

interface HAPanelProps {
  /** The detail view, already fetched by the parent route. */
  database: DatabaseDetailView;
  /** Called after a successful failover so the header re-reads current status. */
  refetch: () => void;
}

/**
 * The HA panel, the Plan page's last card: whether HA is enabled, its switch,
 * and a failover button for HA databases (with a confirmation dialog). Like
 * Render's High Availability card it lists no read replicas; those are a field
 * of Info's Details card (DatabaseReadReplicas, w3/116).
 * Mirrors Render's HA surface (docs/render-artifacts/postgres-ha.md):
 *   - highAvailabilityEnabled: reflects the operator's live observed state
 *   - failoverDatabase mutation: sends POST /v1/postgres/{id}/failover intent
 */
export function HAPanel({ database, refetch }: HAPanelProps) {
  const { t } = useTranslations();
  // Failover is a lifecycle verb (can_operate); the HA switch is a setting
  // PATCH /v1/postgres changes (can_create). The verb refuses a suspended
  // database (409 since w32/111), so the button holds while suspended the way
  // restart holds its own.
  const suspended = isSuspended(database);
  const { denied: operateDenied, reason: operateReason } =
    useCapabilityGate("canOperate");
  const { denied: createDenied, reason: createReason } =
    useCapabilityGate("canCreate");
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [failover, { loading }] = useMutation(FailoverDatabaseDocument);
  const { instanceTypes } = useDatabaseInstanceTypes();
  const plan = instanceTypes.find((it) => it.id === database.plan);
  // Unknown while the catalog loads: keep the neutral "not enabled" line.
  const planOffersHA = plan?.supportsHighAvailability ?? true;
  // The HA switch (w2/m171) shows the spec intent; the badge shows the
  // observed standby, which follows a few minutes later. Enabling needs a plan
  // the catalog says offers HA — bex-api's rule (Render: at least 1 CPU) —
  // and turning it off is always allowed.
  const [setHighAvailability, { loading: savingHA }] = useMutation(
    UpdateDatabaseHighAvailabilityDocument,
  );
  const [haTarget, setHaTarget] = useState<boolean | null>(null);
  const requested = highAvailabilityRequested(database);
  const enableBlocked = plan?.supportsHighAvailability !== true;
  const haLine = requested
    ? database.highAvailabilityEnabled
      ? null
      : t("databases.haStarting")
    : database.highAvailabilityEnabled
      ? t("databases.haStopping")
      : planOffersHA
        ? t("databases.haNotEnabled")
        : t("databases.haPlanUnsupported");

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

  async function handleSetHighAvailability(enabled: boolean) {
    const params = { name: database.name };
    try {
      await setHighAvailability({ variables: { id: database.id, enabled } });
      toast.success(
        t(
          enabled ? "databases.haEnableSuccess" : "databases.haDisableSuccess",
          params,
        ),
      );
      setHaTarget(null);
      refetch();
    } catch (err) {
      toast.error(
        mutationErrorMessage(
          err,
          t(
            enabled ? "databases.haEnableError" : "databases.haDisableError",
            params,
          ),
        ),
      );
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
            <div className="flex items-center gap-3">
              {database.highAvailabilityEnabled ? (
                <PermissionTooltip
                  reason={!suspended ? operateReason : undefined}
                >
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setConfirmOpen(true)}
                    disabled={operateDenied || loading || suspended}
                  >
                    {loading ? <Loader2 className="animate-spin" /> : null}
                    {t("databases.haFailover")}
                  </Button>
                </PermissionTooltip>
              ) : null}
              <PermissionTooltip reason={createReason}>
                <Switch
                  aria-label={t("databases.haToggleLabel")}
                  checked={requested}
                  disabled={
                    createDenied || savingHA || (!requested && enableBlocked)
                  }
                  onCheckedChange={setHaTarget}
                />
              </PermissionTooltip>
            </div>
          </div>

          {haLine ? (
            <p className="text-sm text-muted-foreground">{haLine}</p>
          ) : null}
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

      {/* Enable / disable high availability (w2/m171) */}
      <ConfirmDialog
        open={haTarget != null}
        onOpenChange={(open) => {
          if (!open && !savingHA) setHaTarget(null);
        }}
        title={t(
          haTarget ? "databases.haEnableTitle" : "databases.haDisableTitle",
          { name: database.name },
        )}
        description={
          haTarget
            ? plan?.monthlyUsd
              ? `${t("databases.haEnableBody")} ${t("databases.haEnablePrice", { price: t("common.pricePerMonth", { price: plan.monthlyUsd }) })}`
              : t("databases.haEnableBody")
            : t("databases.haDisableBody")
        }
        confirmLabel={t(
          haTarget ? "databases.haEnableConfirm" : "databases.haDisableConfirm",
        )}
        destructive={!haTarget}
        pending={savingHA}
        closeOnConfirm={false}
        onConfirm={() => {
          if (haTarget != null) void handleSetHighAvailability(haTarget);
        }}
      />
    </>
  );
}
