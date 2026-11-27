import { useState } from "react";
import { Link, Navigate, useRouterState } from "@tanstack/react-router";
import { Button } from "@/common/components/ui/button";
import { Card, CardContent } from "@/common/components/ui/card";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import { ResourceLoadError } from "@/common/components/resource-load-error";
import { BlueprintSyncPageSkeleton } from "@/common/components/route-skeletons";
import { useIsHydrated } from "@/common/hooks/use-is-hydrated";
import { useTranslations } from "@/common/hooks/use-translations";
import { isNotFoundError } from "@/common/lib/graphql-error";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilityGate } from "@/features/capabilities/hooks/use-capability-gate";
import { BlueprintErrorList } from "@/features/blueprints/components/blueprint-error-list";
import { BlueprintSyncCard } from "@/features/blueprints/components/blueprint-sync-card";
import { BlueprintSyncActions } from "@/features/blueprints/components/blueprint-sync-actions";
import { changingSyncActions } from "@/features/blueprints/lib/sync-state";
import {
  manualEnvVarKey,
  useBlueprintSync,
} from "@/features/blueprints/hooks/use-blueprint-sync";
import { useBlueprintDetail } from "@/features/blueprints/lib/blueprint-detail";
import { handedOverNoChangesPlan } from "@/features/blueprints/lib/manual-sync";
import type {
  BlueprintManualEnvVar,
  BlueprintSyncDetailView,
} from "@/features/blueprints/types";

/** The page's heading: Render's `Sync: <short id>`, `Sync` with no commit. */
function SyncHeading({ commitId }: { commitId: string }) {
  const { t } = useTranslations();
  return (
    <h2 className="text-lg leading-7 font-semibold tracking-tight">
      {commitId
        ? t("blueprints.syncPageTitle", { sha: commitId.slice(0, 7) })
        : t("blueprints.syncPageTitleNoCommit")}
    </h2>
  );
}

/** Approve for a waiting plan, with a field per sync:false value it needs. */
function ApproveSync({
  required,
  approving,
  error,
  onApprove,
}: {
  required: BlueprintManualEnvVar[];
  approving: boolean;
  error: string | null;
  onApprove: (values: Record<string, string>) => Promise<void>;
}) {
  const { t } = useTranslations();
  // bex-api approves a Blueprint under can_create, as it syncs one.
  const gate = useCapabilityGate("canCreate");
  const [values, setValues] = useState<Record<string, string>>({});
  const missing = required.some(
    (v) => !(values[manualEnvVarKey(v)] ?? "").trim(),
  );
  return (
    <div className="space-y-4 border-t pt-4">
      {required.length > 0 ? (
        <div className="space-y-3">
          <p className="text-sm">{t("blueprints.plannedManualEnvVars")}</p>
          {required.map((v) => {
            const key = manualEnvVarKey(v);
            const id = `manual-env-${key}`;
            return (
              <div key={key} className="space-y-1">
                <Label htmlFor={id}>
                  {t("blueprints.plannedManualEnvVarLabel", {
                    service: v.serviceName,
                    key: v.key,
                  })}
                </Label>
                <Input
                  id={id}
                  type="password"
                  autoComplete="off"
                  required
                  value={values[key] ?? ""}
                  onChange={(e) =>
                    setValues((prev) => ({ ...prev, [key]: e.target.value }))
                  }
                />
              </div>
            );
          })}
        </div>
      ) : null}
      <PermissionTooltip reason={gate.denied ? gate.reason : undefined}>
        <Button
          size="sm"
          disabled={gate.denied || approving || missing}
          onClick={() => void onApprove(values)}
        >
          {t("blueprints.plannedApprove")}
        </Button>
      </PermissionTooltip>
      {error && !approving ? (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      ) : null}
    </div>
  );
}

/**
 * What a sync did, in Render's order (docs/render-artifacts/blueprint-pages.md
 * § A sync's page): its heading and card, the card of its actions with
 * Approve while it waits, then its errors. The actions' card shows when the
 * sync planned changes or has no errors, so a failed plan shows its errors
 * alone.
 */
function BlueprintSyncDetail({
  sync,
  approve,
}: {
  sync: BlueprintSyncDetailView;
  approve: {
    approving: boolean;
    error: string | null;
    onApprove: (values: Record<string, string>) => Promise<void>;
  };
}) {
  const { t } = useTranslations();
  const { blueprint, linkId } = useBlueprintDetail();
  const hasErrors = sync.errors.length > 0;
  const showActions =
    changingSyncActions(sync.actions).length > 0 ||
    !hasErrors ||
    sync.state === "created";
  return (
    <>
      <SyncHeading commitId={sync.commitId} />
      <BlueprintSyncCard
        sync={sync}
        repo={blueprint.repo}
        blueprintId={linkId}
        showDetails={false}
      />
      {showActions ? (
        <Card data-testid="blueprint-sync-actions">
          <CardContent className="space-y-4">
            <BlueprintSyncActions
              state={sync.state}
              actions={sync.actions}
              actionsRecorded={sync.actionsRecorded}
              hasErrors={hasErrors}
            />
            {sync.state === "created" ? (
              <ApproveSync
                key={sync.id}
                required={sync.requiredManualEnvVars}
                {...approve}
              />
            ) : null}
          </CardContent>
        </Card>
      ) : null}
      {hasErrors ? (
        <Card data-testid="blueprint-sync-errors">
          <CardContent className="space-y-3">
            <p className="text-sm text-muted-foreground">
              {t("blueprints.syncErrorsIntro", { count: sync.errors.length })}
            </p>
            <BlueprintErrorList
              className="text-sm text-destructive"
              errors={sync.errors.map((e) => e.error)}
              details={sync.errors}
            />
          </CardContent>
        </Card>
      ) : null}
      {sync.note ? (
        <p className="whitespace-pre-line text-sm text-muted-foreground">
          {sync.note}
        </p>
      ) : null}
    </>
  );
}

/**
 * One sync's page, Render's `/blueprint/<id>/sync/<syncId>` (w3/m678): read
 * until the sync settles, so the page follows it to its end, with Approve
 * while it waits.
 */
export function BlueprintSyncPage({ syncId }: { syncId: string }) {
  const { t } = useTranslations();
  const { blueprint, linkId } = useBlueprintDetail();
  const { sync, loading, error, refetch, approving, approveError, approve } =
    useBlueprintSync(blueprint.id, syncId);

  if (sync) {
    return (
      <BlueprintSyncDetail
        sync={sync}
        approve={{ approving, error: approveError, onApprove: approve }}
      />
    );
  }
  if (loading) return <BlueprintSyncPageSkeleton />;
  if (error && !isNotFoundError(error)) {
    return <ResourceLoadError onRetry={refetch} />;
  }
  return (
    <>
      <SyncHeading commitId="" />
      <p className="text-sm text-muted-foreground">
        {t("blueprints.syncNotFound")}{" "}
        <Link
          to="/blueprints/$blueprintId/syncs"
          params={{ blueprintId: linkId }}
          className="underline underline-offset-2"
        >
          {t("blueprints.resourcesViewSyncs")}
        </Link>
      </p>
    </>
  );
}

/**
 * Render's `/blueprint/<id>/sync` with no id (w3/m678): the plan Manual sync
 * just made, handed over in the navigation's state, when it saved no sync (a
 * plan with no changes, which reads Resources already up to date); with none,
 * Syncs.
 */
export function BlueprintPlannedSyncPage() {
  const { linkId } = useBlueprintDetail();
  const noChanges = useRouterState({
    select: (s) => handedOverNoChangesPlan(s.location.state),
  });
  // The navigation's state lives in the browser's history, which the server
  // render cannot see: the frame until the page has hydrated.
  const hydrated = useIsHydrated();
  if (!hydrated) return <BlueprintSyncPageSkeleton />;
  if (!noChanges) {
    return (
      <Navigate
        to="/blueprints/$blueprintId/syncs"
        params={{ blueprintId: linkId }}
        replace
      />
    );
  }
  return (
    <>
      <SyncHeading commitId="" />
      <Card data-testid="blueprint-sync-actions">
        <CardContent>
          <BlueprintSyncActions
            state=""
            actions={[]}
            actionsRecorded
            hasErrors={false}
          />
        </CardContent>
      </Card>
    </>
  );
}
