import { useState } from "react";
import { ChevronDown } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
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
import { useTriggerDeploy } from "@/features/services/hooks/use-trigger-deploy";
import type { ServiceView, LifecycleAction } from "@/features/services/types";

export interface ManualDeployButtonProps {
  service: ServiceView;
  /** The lifecycle action in flight for this service, or null — shared with `ServiceRowActions`. */
  pending: LifecycleAction | null;
  onRun: (action: LifecycleAction, service: ServiceView) => void;
}

type ConfirmAction = "deploy" | "restart";

/**
 * Render's "Manual Deploy" header dropdown (captured live from
 * dashboard.render.com/web/.../ — "Deploy latest commit" / "Deploy a specific
 * commit" / "Clear build cache & deploy", a divider, then "Restart service").
 * bex only ever redeploys a service's current source as a whole — there's no
 * build cache to selectively clear and no per-commit picker (`deploys.Trigger`
 * always rebuilds from the configured branch's HEAD, confirmed in
 * `lego/backend/internal/deploys/rest.go`'s own comment: a request `clearCache`
 * flag is accepted-and-ignored) — so the menu keeps only the two items bex can
 * honestly perform, each labeled and confirmed for what it actually does
 * instead of one generic "Trigger a new deploy?" click. "Restart service"
 * reuses the exact `onRun`/`pending` lifecycle plumbing `ServiceRowActions`
 * uses elsewhere — the header hides restart from its own "•••" menu
 * (`ServiceRowActions`'s `hideRestart`) so the same verb doesn't appear twice.
 */
export function ManualDeployButton({
  service,
  pending,
  onRun,
}: ManualDeployButtonProps) {
  const { t } = useTranslations();
  const { deploying, trigger } = useTriggerDeploy();
  const [confirm, setConfirm] = useState<ConfirmAction | null>(null);
  const busy = deploying || pending !== null;
  const repoBacked = !!service.repo;

  const deployLabel = repoBacked
    ? t("services.deployMenuLatestCommit")
    : t("services.deployMenuLatestImage");

  const confirmTitle =
    confirm === "restart"
      ? t("services.confirmRestartTitle", { name: service.name })
      : repoBacked
        ? t("services.deployConfirmCommitTitle", {
            branch: service.branch ?? "",
          })
        : t("services.deployConfirmImageTitle", { name: service.name });

  const confirmBody =
    confirm === "restart"
      ? t("services.confirmRestartBody")
      : repoBacked
        ? t("services.deployConfirmCommitBody", {
            name: service.name,
            branch: service.branch ?? "",
          })
        : t("services.deployConfirmImageBody", { name: service.name });

  async function handleConfirm() {
    if (confirm === "restart") {
      onRun("restart", service);
      setConfirm(null);
      return;
    }
    await trigger(service.id);
    setConfirm(null);
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
          <DropdownMenuItem disabled={busy} onSelect={() => setConfirm("deploy")}>
            {deployLabel}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            disabled={busy}
            onSelect={() => setConfirm("restart")}
          >
            {t("services.deployMenuRestart")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <AlertDialog
        open={confirm !== null}
        onOpenChange={(open) => !open && setConfirm(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{confirmTitle}</AlertDialogTitle>
            <AlertDialogDescription>{confirmBody}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>
              {t("services.eventsConfirmCancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={(e) => {
                // keep the dialog up until the mutation settles, so the button's
                // disabled state is the only "in flight" signal the user needs
                e.preventDefault();
                void handleConfirm();
              }}
              disabled={busy}
            >
              {t("services.eventsConfirmProceed")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
