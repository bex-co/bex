import { useState } from "react";
import { MoreHorizontal, Loader2 } from "lucide-react";
import { Button } from "@/common/components/ui/button.tsx";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu.tsx";
import { MoveToProjectMenu } from "@/features/projects/components/move-to-project-menu";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import type { en } from "@/i18n";
import type { ServiceView, LifecycleAction } from "@/features/services/types";
import type { ProtectedActionResult } from "@/features/services/lib/protected-confirmation";
import { ProtectedConfirmationDialog } from "@/common/components/protected-confirmation-dialog";
import { publiclyRoutable } from "@/features/services/lib/service-type";

const ACTION_LABEL: Record<LifecycleAction, keyof typeof en> = {
  suspend: "services.actionSuspend",
  resume: "services.actionResume",
  restart: "services.actionRestart",
};

// Confirm copy per verb. Suspend and restart are disruptive, so they confirm
// first (Render guards only its destructive verbs); resume is a safe recovery
// and runs immediately — expressed by its *absence* from this table, not a dead
// placeholder entry.
const CONFIRM: Partial<
  Record<LifecycleAction, { title: keyof typeof en; body: keyof typeof en }>
> = {
  suspend: {
    title: "services.confirmSuspendTitle",
    body: "services.confirmSuspendBody",
  },
  restart: {
    title: "services.confirmRestartTitle",
    body: "services.confirmRestartBody",
  },
};

/**
 * The confirm body for a verb, dropping the "Its URL and certificates are kept"
 * clause for the types that never had either — a private service has only an
 * internal address, and a worker or cron job has no address at all (w6/041).
 * Restart's copy makes no URL claim, so it is type-independent.
 */
function confirmBodyKey(
  action: LifecycleAction,
  service: ServiceView,
): keyof typeof en {
  const body = CONFIRM[action]!.body;
  return body === "services.confirmSuspendBody" &&
    !publiclyRoutable(service.type)
    ? "services.confirmSuspendBodyNoUrl"
    : body;
}

export interface ServiceRowActionsProps {
  service: ServiceView;
  /** The action in flight for this row, or null. Disables the control. */
  pending: LifecycleAction | null;
  onRun: (
    action: LifecycleAction,
    service: ServiceView,
    confirmation?: string,
  ) => Promise<ProtectedActionResult>;
  /**
   * Omit "Restart" from this menu (Render parity, service-detail header
   * only): the header's `ManualDeployButton` dropdown already carries
   * "Restart service" grouped with the deploy verbs, Render's own placement
   * — showing it here too would offer the same action under two different
   * menus. The services list still gets the full set; only the header opts in.
   */
  hideRestart?: boolean;
  /**
   * Omit "Suspend" and "Resume" from this menu (service-detail header only):
   * the settings page surfaces both as a dedicated card so they aren't
   * offered in two places simultaneously.
   */
  hideSuspend?: boolean;
}

export function ServiceRowActions({
  service,
  pending,
  onRun,
  hideRestart = false,
  hideSuspend = false,
}: ServiceRowActionsProps) {
  const { t } = useTranslations();
  const [confirm, setConfirm] = useState<LifecycleAction | null>(null);
  const [protectedConfirm, setProtectedConfirm] = useState<{
    action: LifecycleAction;
    confirmation: string;
  } | null>(null);
  const busy = pending !== null;

  // A suspended App can only be resumed; a live one can be suspended or
  // restarted (restart omitted when the caller already surfaces it elsewhere).
  // Suspend/resume are omitted when the settings page card already surfaces them.
  const actions: LifecycleAction[] = hideSuspend
    ? hideRestart
      ? []
      : ["restart"]
    : service.suspended
      ? ["resume"]
      : hideRestart
        ? ["suspend"]
        : ["suspend", "restart"];

  function handleSelect(action: LifecycleAction) {
    if (CONFIRM[action]) {
      setConfirm(action);
    } else {
      void runAction(action);
    }
  }

  async function runAction(action: LifecycleAction, confirmation?: string) {
    const result = confirmation
      ? await onRun(action, service, confirmation)
      : await onRun(action, service);
    if (result.status === "confirmation_required") {
      setProtectedConfirm({
        action,
        confirmation: result.confirmation,
      });
    } else if (result.status === "success") {
      setProtectedConfirm(null);
    }
  }

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="icon-sm"
            disabled={busy}
            aria-label={t("services.actionsMenu")}
          >
            {busy ? <Loader2 className="animate-spin" /> : <MoreHorizontal />}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          {actions.map((action) => (
            <DropdownMenuItem
              key={action}
              disabled={busy}
              variant={action === "suspend" ? "destructive" : "default"}
              onSelect={() => handleSelect(action)}
            >
              {t(ACTION_LABEL[action])}
            </DropdownMenuItem>
          ))}
          <MoveToProjectMenu
            kind="service"
            resourceId={service.id}
            resourceName={service.name}
            disabled={busy}
          />
        </DropdownMenuContent>
      </DropdownMenu>

      <ConfirmDialog
        open={confirm !== null}
        onOpenChange={(open) => !open && setConfirm(null)}
        title={confirm ? t(CONFIRM[confirm]!.title, { name: service.name }) : ""}
        description={
          confirm
            ? t(confirmBodyKey(confirm, service), { name: service.name })
            : ""
        }
        cancelLabel={t("services.confirmCancel")}
        confirmLabel={confirm ? t(ACTION_LABEL[confirm]) : ""}
        onConfirm={() => {
          if (confirm) void runAction(confirm);
          setConfirm(null);
        }}
      />

      <ProtectedConfirmationDialog
        key={
          protectedConfirm ? `open:${protectedConfirm.confirmation}` : "closed"
        }
        open={protectedConfirm !== null}
        resourceName={service.name}
        requiredConfirmation={protectedConfirm?.confirmation ?? ""}
        actionLabel={
          protectedConfirm ? t(ACTION_LABEL[protectedConfirm.action]) : ""
        }
        busy={busy}
        onOpenChange={(open) => !open && setProtectedConfirm(null)}
        onConfirm={(confirmation) =>
          protectedConfirm
            ? runAction(protectedConfirm.action, confirmation)
            : Promise.resolve()
        }
      />
    </>
  );
}
