import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { toast } from "sonner";
import { MoreHorizontal, Settings, ShieldOff } from "lucide-react";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import { useUpdateEnvironment } from "@/features/environments/hooks/use-update-environment";
import { IsolationConfirmDialog } from "@/features/environments/components/isolation-confirm-dialog";
import {
  PLACEHOLDER_SENTINEL,
  aroundPlaceholder,
} from "@/common/lib/i18n-placeholder";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { isolationGated } from "@/features/environments/lib/settings";
import { useWorkspace } from "@/features/workspaces/context";
import { GenerateBlueprintMenuItem } from "@/features/blueprints/components/generate-blueprint-menu-item";
import type { BlueprintExportResource } from "@/features/blueprints/lib/blueprint-export";

/**
 * An environment section's ••• menu, Render's: the Block
 * cross-environment connections toggle, which confirms in Render's isolation
 * dialog and toasts the outcome, then All settings, the environment's settings
 * page. Below Pro the toggle is disabled with Render's "Upgrade to unlock this
 * feature", unless the environment is already isolated, which bex always lets
 * turn off (ADR030). Render shows the toggle only to a member who may change
 * the isolation: here an Admin.
 *
 * Generate Blueprint, while the environment has a service, Postgres or Key
 * Value, opens the Create Blueprint page with all of them titled for the
 * project and environment (w3/m681). Render offers it in the environment's
 * New menu, which w1/m684 brings; until then it is here.
 */
export function EnvironmentMenu({
  environment,
  projectName,
  exportResources = NO_EXPORT,
}: {
  environment: EnvironmentView;
  projectName: string;
  /** The environment's services, Postgres and Key Value, as Render's
   *  environment menu hands them to Generate Blueprint. */
  exportResources?: BlueprintExportResource[];
}) {
  const { t } = useTranslations();
  const capabilities = useCapabilities();
  const { currentWorkspace } = useWorkspace();
  const { update, saving } = useUpdateEnvironment();
  // The value the open dialog asks to set, fixed when it opens so the dialog
  // keeps its words while the saved environment changes under it.
  const [target, setTarget] = useState<boolean | null>(null);
  const [error, setError] = useState<string | null>(null);
  const isolated = environment.networkIsolationEnabled;
  const showToggle = !capabilities.denied("can_manage");
  const gated = isolationGated(currentWorkspace?.plan, isolated);
  const [upgradeBefore, upgradeAfter] = aroundPlaceholder(
    t("environments.menuUpgradeToUnlock", { upgrade: PLACEHOLDER_SENTINEL }),
  );

  async function toggle(block: boolean) {
    setError(null);
    const result = await update(environment.id, {
      networkIsolationEnabled: block,
    });
    if (!result.ok) {
      setError(
        mutationErrorMessage(
          result.error,
          t("environments.settingsUpdateError"),
        ),
      );
      return;
    }
    setTarget(null);
    toast.success(
      t(
        block
          ? "environments.isolationBlockedToast"
          : "environments.isolationAllowedToast",
        { name: environment.name },
      ),
    );
  }

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={t("environments.moreActions")}
          >
            <MoreHorizontal />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-80">
          {showToggle ? (
            <>
              <DropdownMenuItem
                role="menuitemcheckbox"
                aria-checked={isolated}
                disabled={gated}
                onSelect={() => {
                  setError(null);
                  setTarget(!isolated);
                }}
                className="items-start gap-3 py-2"
              >
                <ShieldOff className="mt-0.5" />
                <span className="flex min-w-0 flex-1 flex-col gap-1">
                  <span className="flex items-center gap-2">
                    {t("environments.menuBlockConnections")}
                    {gated ? (
                      <Badge variant="secondary">
                        {t("workspaces.planProName")}
                      </Badge>
                    ) : null}
                  </span>
                  <span className="text-xs whitespace-normal text-muted-foreground">
                    {t("environments.menuBlockConnectionsHint")}
                  </span>
                </span>
                <SwitchLook checked={isolated} />
              </DropdownMenuItem>
              {gated ? (
                <DropdownMenuItem asChild className="pl-9 text-xs">
                  <Link to="/workspace/settings" search={{ plan: "change" }}>
                    <span>
                      {upgradeBefore}
                      <span className="font-medium underline underline-offset-4">
                        {t("environments.upgrade")}
                      </span>
                      {upgradeAfter}
                    </span>
                  </Link>
                </DropdownMenuItem>
              ) : null}
              <DropdownMenuSeparator />
            </>
          ) : null}
          {exportResources.length > 0 ? (
            <GenerateBlueprintMenuItem
              handed={{
                resources: exportResources,
                source: "project environment menu",
                projectName,
                environmentName: environment.name,
              }}
            />
          ) : null}
          <DropdownMenuItem asChild>
            <Link
              to="/project/$projectId/environment/$environmentId/settings"
              params={{
                projectId: environment.projectId,
                environmentId: environment.id,
              }}
            >
              <Settings />
              {t("environments.settingsAction")}
            </Link>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
      {showToggle ? (
        <IsolationConfirmDialog
          open={target !== null}
          onOpenChange={(open) => {
            if (!open && !saving) setTarget(null);
          }}
          restrict={target ?? !isolated}
          projectName={projectName}
          environmentName={environment.name}
          pending={saving}
          error={error}
          onConfirm={() => {
            if (target !== null) void toggle(target);
          }}
        />
      ) : null}
    </>
  );
}

const NO_EXPORT: BlueprintExportResource[] = [];

/** The toggle's on/off look inside the menu item, which is the control. */
function SwitchLook({ checked }: { checked: boolean }) {
  return (
    <span
      aria-hidden="true"
      data-state={checked ? "checked" : "unchecked"}
      className={cn(
        "mt-0.5 inline-flex h-5 w-9 shrink-0 items-center rounded-full border-2 border-transparent transition-colors",
        checked ? "bg-primary" : "border-muted-foreground/70 bg-input",
      )}
    >
      <span
        className={cn(
          "block size-4 rounded-full bg-background shadow-lg transition-transform",
          checked ? "translate-x-4" : "translate-x-0",
        )}
      />
    </span>
  );
}
