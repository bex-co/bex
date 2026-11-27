import { useState } from "react";
import { Loader2, RefreshCw } from "lucide-react";
import { useRouter } from "@tanstack/react-router";
import { useTranslations } from "@/common/hooks/use-translations";
import { Button } from "@/common/components/ui/button";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilityGate } from "@/features/capabilities/hooks/use-capability-gate";
import { useBlueprintPreview } from "@/features/blueprints/hooks/use-blueprint-preview";
import { BlueprintPlanSummary } from "@/features/blueprints/components/blueprint-plan-summary";
import { BlueprintErrorList } from "@/features/blueprints/components/blueprint-error-list";
import { BlueprintConfirmationDialog } from "@/features/blueprints/components/blueprint-confirmation-dialog";
import {
  useSyncBlueprint,
  type ReviewedBlueprintSource,
} from "@/features/blueprints/hooks/use-sync-blueprint";
import type { BlueprintConfirmationRequired } from "@/features/blueprints/lib/takeover";
import type { BlueprintView } from "@/features/blueprints/types";

function shortCommit(sha: string): string {
  return sha.slice(0, 8);
}

/**
 * The Blueprint header's one action, Render's **Manual sync** (w3/m677), with
 * a refresh icon after its label. It still opens bex's reviewed-commit sync
 * dialog: the plan the file would apply, pinned to the commit the user read
 * (w8/m41), retried through the server's phrase when a protected environment
 * or a takeover asks for one. w3/m678 makes it plan a sync and open that
 * sync's page, as Render's does.
 *
 * As on Render it is disabled with the reason for a role that cannot change
 * the Blueprint (bex-api syncs a Blueprint under `can_create`). While a
 * created Blueprint's first plan waits for approval it is disabled too, with
 * where to go: that Blueprint applies only by approving its plan, and the
 * reviewed sync refuses it (w2/m256 t012).
 */
export function BlueprintManualSync({
  blueprint,
}: {
  blueprint: BlueprintView;
}) {
  const { t } = useTranslations();
  const router = useRouter();
  const gate = useCapabilityGate("canCreate");
  const { sync, busy: syncBusy } = useSyncBlueprint();
  const awaitingApproval = blueprint.status === "created";

  const [confirming, setConfirming] = useState(false);
  // Snapshot of the preview the user is about to confirm (w8/m41). Never
  // silently adopt a newer preview after this is set — dialog close and each
  // opening clear it; source-changed forces an explicit refresh.
  const [reviewedSource, setReviewedSource] =
    useState<ReviewedBlueprintSource | null>(null);
  const [sourceChangedHint, setSourceChangedHint] = useState(false);
  // After BLUEPRINT_SOURCE_CHANGED, block auto-capture until the user
  // explicitly refreshes — otherwise the still-cached valid preview re-pins.
  const [awaitingFreshPreview, setAwaitingFreshPreview] = useState(false);
  // Pre-sync plan (w8/m21 t002): fetched only while the dialog is open — an
  // empty repo skips the query. Render shows a computed diff before sync;
  // applying blindly is the pre-m21 behavior this replaces.
  const {
    preview: syncPreview,
    loading: syncPreviewLoading,
    error: syncPreviewError,
    refetch: refetchSyncPreview,
  } = useBlueprintPreview(
    confirming ? blueprint.repo : "",
    confirming ? blueprint.branch : "",
    confirming ? blueprint.path : "",
    // On behalf of this blueprint: the sync is planned against its own
    // resources, which a create's preview would report as already existing
    // (w8/m153).
    blueprint.id,
  );
  // The server's refusal awaiting its phrase (a protected environment, or a
  // takeover of a resource another Blueprint manages), with the pin its retry
  // resends. Closing the sync dialog drops the reviewed pin, and the phrase
  // only arrives after that close, so the retry carries its own (w8/m41).
  const [pendingConfirmation, setPendingConfirmation] = useState<
    | (BlueprintConfirmationRequired & { reviewed: ReviewedBlueprintSource })
    | null
  >(null);

  // Capture a reviewed pin only from a valid preview for the current
  // blueprint settings. Do not overwrite an existing pin with a later fetch
  // the user has not re-confirmed (w8/m41). Captured while rendering the
  // preview the user is looking at, not in an effect one paint later.
  if (
    !reviewedSource &&
    confirming &&
    !awaitingFreshPreview &&
    syncPreview?.found &&
    syncPreview.validation?.valid === true &&
    syncPreview.commitId
  ) {
    setReviewedSource({
      commitId: syncPreview.commitId,
      path: blueprint.path,
      repo: blueprint.repo,
    });
  }

  function resetReview() {
    setReviewedSource(null);
    setSourceChangedHint(false);
    setAwaitingFreshPreview(false);
  }

  function open() {
    // Each opening reviews the file as it is now: a path edited on Settings
    // since the last opening must not keep the old pin.
    resetReview();
    setConfirming(true);
  }

  async function handleSync(
    reviewed: ReviewedBlueprintSource,
    confirmation?: string,
  ) {
    const result = await sync(blueprint.id, { reviewed, confirmation });
    if (result.status === "confirmation_required") {
      setConfirming(false);
      setPendingConfirmation({ ...result, reviewed });
      return;
    }
    if (result.status === "source_changed") {
      setPendingConfirmation(null);
      setConfirming(true);
      setReviewedSource(null);
      setAwaitingFreshPreview(true);
      setSourceChangedHint(true);
      return;
    }
    if (result.status === "success") {
      setConfirming(false);
      setPendingConfirmation(null);
      resetReview();
      void router.invalidate();
    }
  }

  async function handleRefreshPreview() {
    resetReview();
    await refetchSyncPreview();
  }

  const canConfirmSync = !!reviewedSource && !syncPreviewLoading;
  const reason = gate.denied
    ? gate.reason
    : awaitingApproval
      ? t("blueprints.manualSyncAwaitingApproval")
      : undefined;

  return (
    <>
      <PermissionTooltip reason={reason}>
        <Button
          size="sm"
          onClick={open}
          disabled={gate.denied || awaitingApproval || syncBusy}
        >
          {t("blueprints.syncButton")}
          <RefreshCw aria-hidden="true" />
        </Button>
      </PermissionTooltip>

      <ConfirmDialog
        open={confirming}
        onOpenChange={(next) => {
          setConfirming(next);
          if (!next) resetReview();
        }}
        contentClassName="max-h-[85vh] overflow-y-auto sm:max-w-xl"
        title={t("blueprints.syncConfirmTitle")}
        description={t("blueprints.syncConfirmBody")}
        cancelLabel={t("blueprints.syncCancel")}
        confirmLabel={t("blueprints.syncConfirmAction")}
        // Syncing is the primary action, and it is gated on a reviewed commit
        // pin so nobody applies bytes they have not seen (w8/m41).
        destructive={false}
        confirmDisabled={!canConfirmSync || syncBusy}
        onConfirm={() => reviewedSource && void handleSync(reviewedSource)}
      >
        {syncPreviewLoading ? (
          <div className="flex items-center gap-2 py-2 text-sm text-muted-foreground">
            <Loader2 className="size-4 animate-spin" />
            {t("blueprints.syncPreviewLoading")}
          </div>
        ) : syncPreview?.found && syncPreview.validation?.valid === true ? (
          <div className="space-y-3">
            <BlueprintPlanSummary
              plan={syncPreview.validation.plan}
              pricing={syncPreview.validation.estimatedPricing}
              note={
                reviewedSource ? (
                  <p className="font-mono text-xs text-muted-foreground">
                    {t("blueprints.syncPreviewCommit", {
                      sha: shortCommit(reviewedSource.commitId),
                    })}
                  </p>
                ) : null
              }
            />
          </div>
        ) : syncPreview ? (
          <div className="space-y-2 rounded-md border border-destructive/50 p-3 text-sm">
            <p className="font-medium text-destructive">
              {t("blueprints.syncPreviewInvalid")}
            </p>
            <BlueprintErrorList
              className="text-muted-foreground"
              errors={(
                syncPreview.validation?.errors ?? [syncPreview.error]
              ).filter((e): e is string => !!e)}
              details={syncPreview.validation?.errorDetails ?? []}
            />
            <Button
              size="sm"
              variant="outline"
              onClick={() => void handleRefreshPreview()}
            >
              {t("blueprints.syncRefreshPreview")}
            </Button>
          </div>
        ) : syncPreviewError || sourceChangedHint ? (
          <div className="space-y-2">
            <p className="text-sm text-amber-600 dark:text-amber-500">
              {sourceChangedHint
                ? t("blueprints.syncSourceChanged")
                : t("blueprints.syncPreviewUnavailable")}
            </p>
            <Button
              size="sm"
              variant="outline"
              onClick={() => void handleRefreshPreview()}
            >
              {t("blueprints.syncRefreshPreview")}
            </Button>
          </div>
        ) : null}
      </ConfirmDialog>

      <BlueprintConfirmationDialog
        pending={pendingConfirmation}
        actionLabel={t("blueprints.syncConfirmAction")}
        busy={syncBusy}
        onDismiss={() => setPendingConfirmation(null)}
        onConfirm={async (confirmation) => {
          if (pendingConfirmation)
            await handleSync(pendingConfirmation.reviewed, confirmation);
        }}
      />
    </>
  );
}
