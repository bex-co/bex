import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { Check, ChevronDown, GitCommitHorizontal, Package, TriangleAlert } from "lucide-react";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import { Skeleton } from "@/common/components/ui/skeleton";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { TextField } from "@/common/components/text-field";
import { useNow } from "@/common/hooks/use-now";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatTimeAgo } from "@/common/lib/format";
import { cn } from "@/common/lib/utils/utils";
import {
  useBuildSourceBuilds,
  type BuildSourceBuildView,
} from "@/features/services/hooks/use-build-sources";
import { useTriggerDeploy } from "@/features/services/hooks/use-trigger-deploy";
import { serviceBaseForType } from "@/features/services/lib/service-base";
import { isCron } from "@/features/services/lib/service-type";
import type { ServiceView } from "@/features/services/types";
import { PermissionMenuItem } from "@/features/capabilities/components/permission-menu-item";
import { ConfirmGateStatus } from "@/features/capabilities/components/confirm-gate-status";
import { useDeployActions } from "@/features/capabilities/hooks/use-resource-actions";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  useBoundActionConfirm,
  type ActionConfirmBinding,
} from "@/features/capabilities/hooks/use-bound-action-confirm";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import {
  gateAction,
  gateReason,
  resourceDecision,
} from "@/features/capabilities/lib/resource-actions";

/** A full or abbreviated Git commit SHA — the refs the dialog accepts. */
const COMMIT_SHA = /^[0-9a-f]{7,40}$/i;

export interface ManualDeployButtonProps {
  service: ServiceView;
  /** Whether any action is already in flight for this service. */
  pending: boolean;
}

/**
 * Render's "Manual Deploy" header dropdown: "Deploy latest commit" (or
 * "Deploy latest image"), "Deploy a specific commit" for a repo-backed service,
 * "Clear build cache & deploy", a divider, then "Restart service".
 *
 * Every item opens a deploy-history row and lands on its page. Restart runs
 * `restartServer`, which keeps the commit or image that is live; a
 * parameter-free `triggerDeploy` would build the branch head (w1/m148). A
 * specific-commit deploy also turns auto-deploy off in the same mutation, as
 * Render's dashboard does, so the next push cannot replace the pin. A specific commit also needs
 * the create grant because it chooses executable content.
 */
export function ManualDeployButton({
  service,
  pending,
}: ManualDeployButtonProps) {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const capabilities = useCapabilities();
  const deployActions = useDeployActions(service.id);
  const actionConfirm = useBoundActionConfirm({ resourceId: service.id });
  const commitConfirm = useBoundActionConfirm({
    resourceId: service.id,
    requiredCapability: "can_create",
  });
  const { deploying, trigger, restart } = useTriggerDeploy();
  const navigate = useNavigate();
  const [storedDialog, setDialog] = useState<{
    kind: "restart" | "commit" | "build";
    binding: ActionConfirmBinding;
    commitId: string;
    buildId: string;
  } | null>(null);
  const [checking, setChecking] = useState(false);
  const dialog =
    storedDialog?.binding ===
    (storedDialog?.kind === "restart"
      ? actionConfirm.pending
      : commitConfirm.pending)
      ? storedDialog
      : null;
  const commitId = dialog?.kind === "commit" ? dialog.commitId : "";
  const selectedBuildId = dialog?.kind === "build" ? dialog.buildId : "";
  const base = serviceBaseForType(service.type);
  const busy = checking || deploying || pending;
  const buildSourceBacked = !!service.buildSourceId;
  const repoBacked = !buildSourceBacked && !!service.repo;
  // Render: a specific commit is "Not supported for cron jobs".
  const canDeployCommit = repoBacked && !isCron(service);
  const sha = commitId.trim();
  const shaValid = COMMIT_SHA.test(sha);
  const shaError = sha !== "" && !shaValid;

  const deployLabel = buildSourceBacked
    ? t("services.deployMenuLatestBuild")
    : repoBacked
      ? t("services.deployMenuLatestCommit")
      : t("services.deployMenuLatestImage");

  const decision =
    deployActions.status === "ready"
      ? resourceDecision(
          deployActions.snapshot,
          currentWorkspaceId,
          service.id,
          "deploy",
        )
      : null;
  const gate = gateAction(
    decision,
    deployActions.status === "ready" ? "ready" : deployActions.status,
  );
  const permissionReason = gateReason(gate, t);
  const commitReason =
    permissionReason ??
    (capabilities.canCreate
      ? undefined
      : t(
          capabilities.reasonKey("can_create") ?? "capabilities.actionChecking",
        ));
  // A routine permission refresh keeps an open dialog (and its typed SHA) but
  // blocks confirm, with the reason shown, until access is fresh (w4/m159).
  const restartBlocked = actionConfirm.blockedReason ?? permissionReason;
  const commitBlocked = commitConfirm.blockedReason ?? commitReason;

  function openDeploy(deployId: string | null) {
    if (!deployId) return;
    void navigate({
      to: `${base}/$serviceId/deploys/$deployId`,
      params: { serviceId: service.id, deployId },
    });
  }

  function openDialog(kind: "restart" | "commit" | "build") {
    const needsCreate = kind === "commit" || kind === "build";
    if (busy || (needsCreate ? commitReason : permissionReason)) return;
    const confirmation = needsCreate ? commitConfirm : actionConfirm;
    const binding = confirmation.openConfirm("deploy");
    if (binding) setDialog({ kind, binding, commitId: "", buildId: "" });
  }

  function closeDialog() {
    actionConfirm.clearConfirm();
    commitConfirm.clearConfirm();
    setDialog(null);
  }

  async function runDeploy(
    confirmation: typeof actionConfirm,
    binding: ActionConfirmBinding | null,
    run: () => Promise<string | null>,
  ) {
    if (busy || !binding) return;
    setChecking(true);
    let dispatched = false;
    try {
      const { ok } = await confirmation.recheckBeforeDispatch(binding);
      if (!ok) return;
      dispatched = true;
      const deployId = await run();
      // The mutation already ran; landing on it is presentation, so only a
      // context change (not a routine refresh) suppresses it.
      if (confirmation.isIntentCurrent(binding)) openDeploy(deployId);
    } finally {
      // An access lapse mid-check leaves an open same-context dialog for a
      // new click; a refusal has already closed it.
      if (dispatched || dialog?.binding !== binding)
        confirmation.clearConfirm();
      setChecking(false);
    }
  }

  async function handleDeploy(opts?: { clearCache?: boolean }) {
    if (busy || permissionReason) return;
    await runDeploy(actionConfirm, actionConfirm.openConfirm("deploy"), () =>
      opts?.clearCache
        ? trigger(service.id, { clearCache: "clear" })
        : trigger(service.id),
    );
  }

  async function handleRestart() {
    if (restartBlocked || dialog?.kind !== "restart") return;
    await runDeploy(actionConfirm, dialog.binding, () => restart(service.id));
  }

  async function handleDeployCommit() {
    if (commitBlocked || !shaValid || dialog?.kind !== "commit") return;
    const { binding } = dialog;
    // Pinning also turns auto-deploy off, in the same write, so no push can
    // land between the pin and the switch-off (w4/m431).
    await runDeploy(commitConfirm, binding, () =>
      trigger(service.id, { commitId: sha, disableAutoDeploy: true }),
    );
  }

  async function handleDeployBuild() {
    if (commitBlocked || !selectedBuildId || dialog?.kind !== "build") return;
    const { binding } = dialog;
    await runDeploy(commitConfirm, binding, () =>
      trigger(service.id, { buildId: selectedBuildId }),
    );
  }

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button size="sm" disabled={busy}>
            {t("services.eventsManualDeploy")}
            <ChevronDown className="size-3.5" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <PermissionMenuItem
            disabled={busy}
            permissionReason={permissionReason}
            onSelect={() => void handleDeploy()}
          >
            {deployLabel}
          </PermissionMenuItem>
          {buildSourceBacked && (
            <PermissionMenuItem
              disabled={busy}
              permissionReason={commitReason}
              onSelect={() => openDialog("build")}
            >
              {t("services.deployMenuSpecificBuild")}
            </PermissionMenuItem>
          )}
          {canDeployCommit && (
            <PermissionMenuItem
              disabled={busy}
              permissionReason={commitReason}
              onSelect={() => openDialog("commit")}
            >
              {t("services.deployMenuSpecificCommit")}
            </PermissionMenuItem>
          )}
          {/* An image-backed service has no build, so no build cache to
              clear (w4/m141). */}
          {repoBacked && (
            <PermissionMenuItem
              disabled={busy}
              permissionReason={permissionReason}
              onSelect={() => void handleDeploy({ clearCache: true })}
            >
              {t("services.deployMenuClearCache")}
            </PermissionMenuItem>
          )}
          {/* Render: restart is "Not supported for cron jobs" — there is
              no running instance to restart (w8/040). */}
          {!isCron(service) && (
            <>
              <DropdownMenuSeparator />
              <PermissionMenuItem
                disabled={busy}
                permissionReason={permissionReason}
                onSelect={() => openDialog("restart")}
              >
                {t("services.deployMenuRestart")}
              </PermissionMenuItem>
            </>
          )}
        </DropdownMenuContent>
      </DropdownMenu>

      <ConfirmDialog
        open={dialog?.kind === "restart"}
        onOpenChange={(open) => !open && closeDialog()}
        title={t("services.confirmRestartTitle", { name: service.name })}
        description={t(
          service.type === "static_site"
            ? "services.confirmRestartBodyStatic"
            : "services.confirmRestartBody",
          { name: service.name },
        )}
        cancelLabel={t("services.confirmCancel")}
        confirmLabel={t("services.actionRestart")}
        destructive={false}
        pending={busy}
        closeOnConfirm={false}
        confirmDisabled={!!restartBlocked}
        onConfirm={() => void handleRestart()}
      >
        <ConfirmGateStatus reason={restartBlocked} />
      </ConfirmDialog>

      <ConfirmDialog
        open={dialog?.kind === "commit"}
        onOpenChange={(open) => !open && closeDialog()}
        title={t("services.deployCommitTitle")}
        description={t("services.deployCommitBody")}
        cancelLabel={t("services.confirmCancel")}
        confirmLabel={t("services.deployCommitConfirm")}
        destructive={false}
        pending={busy}
        closeOnConfirm={false}
        confirmDisabled={!shaValid || !!commitBlocked}
        onConfirm={() => void handleDeployCommit()}
      >
        <TextField
          id="deploy-commit-sha"
          label={t("services.deployCommitLabel")}
          value={commitId}
          onChange={(value) => {
            if (dialog) setDialog({ ...dialog, commitId: value });
          }}
          error={shaError ? t("services.deployCommitInvalid") : undefined}
        />
        <ConfirmGateStatus reason={commitBlocked} />
      </ConfirmDialog>

      <ConfirmDialog
        open={dialog?.kind === "build"}
        onOpenChange={(open) => !open && closeDialog()}
        title={t("services.deployBuildTitle")}
        description={t("services.deployBuildBody")}
        cancelLabel={t("services.confirmCancel")}
        confirmLabel={t("services.deployBuildConfirm")}
        destructive={false}
        pending={busy}
        closeOnConfirm={false}
        confirmDisabled={!selectedBuildId || !!commitBlocked}
        onConfirm={() => void handleDeployBuild()}
      >
        {dialog?.kind === "build" && service.buildSourceId ? (
          <SpecificBuildPicker
            buildSourceId={service.buildSourceId}
            selectedBuildId={selectedBuildId}
            onSelect={(buildId) => {
              if (dialog) setDialog({ ...dialog, buildId });
            }}
          />
        ) : null}
        <ConfirmGateStatus reason={commitBlocked} />
      </ConfirmDialog>
    </>
  );
}

const BUILD_STATUS_KEY: Record<string, string> = {
  created: "services.buildStatusCreated",
  in_progress: "services.buildStatusInProgress",
  succeeded: "services.buildStatusSucceeded",
  failed: "services.buildStatusFailed",
  canceled: "services.buildStatusCanceled",
};

function buildStatusVariant(
  status: string,
): "default" | "secondary" | "destructive" | "outline" {
  switch (status) {
    case "succeeded":
      return "default";
    case "failed":
      return "destructive";
    case "in_progress":
    case "created":
      return "secondary";
    default:
      return "outline";
  }
}

function SpecificBuildPicker({
  buildSourceId,
  selectedBuildId,
  onSelect,
}: {
  buildSourceId: string;
  selectedBuildId: string;
  onSelect: (buildId: string) => void;
}) {
  const { t, i18n } = useTranslations();
  const now = useNow();
  const justNow = t("common.justNow");
  const { builds, loading, error, refetch } = useBuildSourceBuilds(
    buildSourceId,
    true,
  );

  if (loading) {
    return (
      <div className="space-y-2" aria-busy="true" data-testid="build-picker-loading">
        <Skeleton className="h-14 w-full rounded-md" />
        <Skeleton className="h-14 w-full rounded-md" />
        <Skeleton className="h-14 w-full rounded-md" />
      </div>
    );
  }

  if (error && builds.length === 0) {
    return (
      <div
        role="alert"
        className="flex flex-col items-center gap-2 rounded-lg border p-4 text-center text-sm"
      >
        <TriangleAlert className="size-4 text-destructive" aria-hidden="true" />
        <p className="text-muted-foreground">
          {t("services.deployBuildLoadError")}
        </p>
        <Button type="button" variant="outline" size="sm" onClick={refetch}>
          {t("common.tryAgain")}
        </Button>
      </div>
    );
  }

  if (builds.length === 0) {
    return (
      <p className="rounded-lg border p-4 text-center text-sm text-muted-foreground">
        {t("services.deployBuildEmpty")}
      </p>
    );
  }

  const hasSucceeded = builds.some((b) => b.succeeded);

  return (
    <div className="space-y-2">
      {!hasSucceeded ? (
        <p className="text-xs text-muted-foreground">
          {t("services.deployBuildNoSucceeded")}
        </p>
      ) : null}
      <div
        role="group"
        aria-label={t("services.deployBuildListLabel")}
        className="max-h-64 divide-y overflow-y-auto rounded-lg border"
      >
        {builds.map((b) => (
          <BuildPickerRow
            key={b.id}
            build={b}
            selected={b.id === selectedBuildId}
            ago={formatTimeAgo(b.createdAt, {
              now,
              language: i18n.language,
              justNow,
            })}
            onSelect={() => onSelect(b.id)}
          />
        ))}
      </div>
    </div>
  );
}

function BuildPickerRow({
  build,
  selected,
  ago,
  onSelect,
}: {
  build: BuildSourceBuildView;
  selected: boolean;
  ago: string | null;
  onSelect: () => void;
}) {
  const { t } = useTranslations();
  const statusKey =
    BUILD_STATUS_KEY[build.status] ?? "services.buildStatusCreated";
  const refText = build.commitId
    ? build.commitId.slice(0, 7)
    : build.imageRef ?? null;

  return (
    <button
      type="button"
      disabled={!build.succeeded}
      aria-pressed={build.succeeded ? selected : undefined}
      onClick={onSelect}
      className={cn(
        "flex w-full items-start gap-2.5 px-3 py-2.5 text-left text-sm transition-colors",
        build.succeeded
          ? "hover:bg-muted focus-visible:bg-muted focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none focus-visible:ring-inset"
          : "cursor-not-allowed opacity-60",
        selected &&
          "bg-primary/5 shadow-[inset_2px_0_0_var(--primary)] hover:bg-primary/10 dark:bg-primary/15",
      )}
    >
      <div className="min-w-0 flex-1 space-y-1">
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="font-mono text-xs font-medium break-all">
            {build.id}
          </span>
          <Badge variant={buildStatusVariant(build.status)} className="text-[11px]">
            {t(statusKey as Parameters<typeof t>[0])}
          </Badge>
        </div>
        <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5 text-xs text-muted-foreground">
          {refText ? (
            <span className="inline-flex min-w-0 items-center gap-1 font-mono">
              {build.commitId ? (
                <GitCommitHorizontal className="size-3 shrink-0" aria-hidden="true" />
              ) : (
                <Package className="size-3 shrink-0" aria-hidden="true" />
              )}
              <span className="truncate">{refText}</span>
            </span>
          ) : null}
          {ago ? <span>{ago}</span> : null}
        </div>
      </div>
      {selected ? (
        <Check className="mt-0.5 size-4 shrink-0 text-primary" aria-hidden="true" />
      ) : null}
    </button>
  );
}
