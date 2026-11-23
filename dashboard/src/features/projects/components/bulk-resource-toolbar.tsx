import { useState } from "react";
import { Loader2, Pause, Play } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { useTranslations } from "@/common/hooks/use-translations";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useProtectedPlacement } from "@/features/environments/hooks/use-protected-placement";
import { StrandedGroupsConfirmDialog } from "@/features/environments/components/stranded-groups-confirm-dialog";
import { BulkLifecycleDialog } from "@/features/projects/components/bulk-lifecycle-dialog";
import {
  useBulkMove,
  type BulkMoveTarget,
} from "@/features/projects/hooks/use-bulk-move";
import type { ResourceSelection } from "@/features/projects/hooks/use-resource-selection";
import {
  bulkLifecycleReason,
  lifecycleTargets,
  targetsFor,
  type BulkLifecycleAction,
  type LifecycleTarget,
} from "@/features/projects/lib/bulk-lifecycle";

export interface BulkResourceToolbarProps {
  selection: ResourceSelection;
  /** The environments the selection may move into. */
  moveTargets: BulkMoveTarget[];
  /** Why Move is unavailable for this list as a whole, or undefined. */
  moveReason?: string;
  /** The list is a protected environment's: lifecycle actions are Admins'. */
  protectedEnvironment?: boolean;
}

/**
 * The selection toolbar of a resource list (w4/m550), shared by a project
 * environment's card and the Overview's ungrouped list: the selection count,
 * Move, and Render's Resume and Suspend. Resume shows while a selected
 * resource is suspended and Suspend while one is running, both in a mixed
 * selection, as on Render; a refusal disables the action with its reason
 * rather than dropping rows (docs/render-artifacts/service-list-bulk-actions.md).
 */
export function BulkResourceToolbar({
  selection,
  moveTargets,
  moveReason,
  protectedEnvironment = false,
}: BulkResourceToolbarProps) {
  const { t } = useTranslations();
  const capabilities = useCapabilities();
  const placement = useProtectedPlacement();
  const bulkMove = useBulkMove(
    selection,
    moveTargets.map((target) => target.environment),
  );
  const [moveTargetId, setMoveTargetId] = useState("");
  const [lifecycle, setLifecycle] = useState<{
    action: BulkLifecycleAction;
    targets: LifecycleTarget[];
  } | null>(null);
  const moving = bulkMove.movingTo(moveTargetId);
  const count = selection.selected.size;
  // bex-api gates placement on can_create, and Render disables Move without
  // moveResource: a Viewer reads the role's reason, never a 403 after a click.
  const createReasonKey = capabilities.reasonKey("can_create");
  const moveBlocked =
    (createReasonKey ? t(createReasonKey) : undefined) ?? moveReason;

  const lifecycleSelection = lifecycleTargets(selection.selectedRows);
  const toResume = targetsFor("resume", lifecycleSelection);
  const toSuspend = targetsFor("suspend", lifecycleSelection);
  const lifecycleReason = bulkLifecycleReason(
    capabilities,
    protectedEnvironment,
    t,
  );

  function lifecycleButton(action: BulkLifecycleAction) {
    const targets = action === "suspend" ? toSuspend : toResume;
    if (targets.length === 0) return null;
    const Icon = action === "suspend" ? Pause : Play;
    return (
      <PermissionTooltip reason={lifecycleReason}>
        <Button
          variant="ghost"
          size="sm"
          className={action === "suspend" ? "text-destructive" : undefined}
          disabled={lifecycleReason !== undefined || lifecycle !== null}
          onClick={() => setLifecycle({ action, targets })}
        >
          <Icon />
          {t(
            action === "suspend"
              ? "projects.bulkSuspend"
              : "projects.bulkResume",
          )}
        </Button>
      </PermissionTooltip>
    );
  }

  return (
    <>
      <div className="flex flex-wrap items-center gap-2" aria-live="polite">
        <span className="text-sm text-muted-foreground">
          {t("projects.selectedCount", { count })}
        </span>
        <Select value={moveTargetId} onValueChange={setMoveTargetId}>
          <SelectTrigger
            className="w-48"
            aria-label={t("projects.moveTargetLabel")}
            disabled={
              moveTargets.length === 0 || moving || moveBlocked !== undefined
            }
          >
            <SelectValue placeholder={t("projects.moveTargetPlaceholder")} />
          </SelectTrigger>
          <SelectContent>
            {moveTargets.map(({ environment, label }) =>
              placement.restricts(environment) ? (
                <SelectItem
                  key={environment.id}
                  value={environment.id}
                  disabled
                >
                  {t("environments.protectedOptionAdminOnly", {
                    environment: label,
                  })}
                </SelectItem>
              ) : (
                <SelectItem key={environment.id} value={environment.id}>
                  {label}
                </SelectItem>
              ),
            )}
          </SelectContent>
        </Select>
        <PermissionTooltip reason={moveBlocked}>
          <Button
            variant="outline"
            size="sm"
            disabled={
              count === 0 ||
              !moveTargetId ||
              moving ||
              moveBlocked !== undefined
            }
            onClick={() => void bulkMove.move(moveTargetId)}
          >
            {moving ? <Loader2 className="animate-spin" /> : null}
            {t("projects.moveSelected")}
          </Button>
        </PermissionTooltip>
        {lifecycleButton("resume")}
        {lifecycleButton("suspend")}
      </div>

      <StrandedGroupsConfirmDialog
        open={bulkMove.strandedPrompt != null}
        onOpenChange={(next) => {
          if (!next) bulkMove.dismissStranded();
        }}
        items={bulkMove.strandedPrompt?.items ?? []}
        onConfirm={bulkMove.confirmStranded}
      />

      {lifecycle ? (
        <BulkLifecycleDialog
          action={lifecycle.action}
          targets={lifecycle.targets}
          protectedEnvironment={protectedEnvironment}
          onClose={(succeeded) => {
            // Render clears a selection that all changed; a row that failed
            // stays selected, so its retry is one click (w4/m550 DoD).
            selection.deselect(succeeded);
            setLifecycle(null);
          }}
        />
      ) : null}
    </>
  );
}
