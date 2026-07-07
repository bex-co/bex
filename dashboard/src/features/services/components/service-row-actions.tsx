import { useState } from "react";
import { MoreHorizontal, Loader2 } from "lucide-react";
import { Button } from "@/common/components/ui/button.tsx";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu.tsx";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/common/components/ui/alert-dialog.tsx";
import { useTranslations } from "@/common/hooks/use-translations";
import type { en } from "@/i18n";
import type { ServiceView, LifecycleAction } from "@/features/services/types";

// Suspend and restart are disruptive, so they confirm first; resume is a safe
// recovery and runs immediately (matches Render's dashboard, which only guards
// the destructive verbs).
const CONFIRMS: Record<LifecycleAction, boolean> = {
  suspend: true,
  restart: true,
  resume: false,
};

const ACTION_LABEL: Record<LifecycleAction, keyof typeof en> = {
  suspend: "services.actionSuspend",
  resume: "services.actionResume",
  restart: "services.actionRestart",
};

const CONFIRM_TITLE: Record<LifecycleAction, keyof typeof en> = {
  suspend: "services.confirmSuspendTitle",
  restart: "services.confirmRestartTitle",
  resume: "services.confirmSuspendTitle", // unused (resume never confirms)
};

const CONFIRM_BODY: Record<LifecycleAction, keyof typeof en> = {
  suspend: "services.confirmSuspendBody",
  restart: "services.confirmRestartBody",
  resume: "services.confirmSuspendBody", // unused
};

export interface ServiceRowActionsProps {
  service: ServiceView;
  /** The action in flight for this row, or null. Disables the control. */
  pending: LifecycleAction | null;
  onRun: (action: LifecycleAction, service: ServiceView) => void;
}

export function ServiceRowActions({
  service,
  pending,
  onRun,
}: ServiceRowActionsProps) {
  const { t } = useTranslations();
  const [confirm, setConfirm] = useState<LifecycleAction | null>(null);
  const busy = pending !== null;

  // A suspended App can only be resumed; a live one can be suspended or restarted.
  const actions: LifecycleAction[] = service.suspended
    ? ["resume"]
    : ["suspend", "restart"];

  function handleSelect(action: LifecycleAction) {
    if (CONFIRMS[action]) {
      setConfirm(action);
    } else {
      onRun(action, service);
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
            {busy ? (
              <Loader2 className="animate-spin" />
            ) : (
              <MoreHorizontal />
            )}
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
        </DropdownMenuContent>
      </DropdownMenu>

      <AlertDialog
        open={confirm !== null}
        onOpenChange={(open) => !open && setConfirm(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {confirm && t(CONFIRM_TITLE[confirm], { name: service.name })}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {confirm && t(CONFIRM_BODY[confirm], { name: service.name })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("services.confirmCancel")}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (confirm) onRun(confirm, service);
                setConfirm(null);
              }}
            >
              {confirm && t(ACTION_LABEL[confirm])}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
