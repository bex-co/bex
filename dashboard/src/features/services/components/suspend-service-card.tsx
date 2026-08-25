import { useState } from "react";
import { Loader2 } from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/common/components/ui/card";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import type { ServiceView, LifecycleAction } from "@/features/services/types";
import type { ProtectedActionResult } from "@/features/services/lib/protected-confirmation";

export interface SuspendServiceCardProps {
  service: ServiceView;
  pending: LifecycleAction | null;
  onRun: (
    action: LifecycleAction,
    service: ServiceView,
    confirmation?: string,
  ) => Promise<ProtectedActionResult>;
}

/**
 * Settings-tab suspend/resume card: mirrors the bottom-of-settings pattern
 * from Render's own dashboard. Shows "Suspend Service" when live and
 * "Resume Service" when suspended. Suspend opens an alert confirmation;
 * resume runs immediately (same policy as the row-actions dropdown).
 */
export function SuspendServiceCard({
  service,
  pending,
  onRun,
}: SuspendServiceCardProps) {
  const { t } = useTranslations();
  const [open, setOpen] = useState(false);
  const busy = pending !== null;
  const isSuspended = service.suspended;

  async function handleResume() {
    await onRun("resume", service);
  }

  async function handleSuspendConfirm() {
    setOpen(false);
    await onRun("suspend", service);
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>
          {t(
            isSuspended
              ? "services.resumeCardTitle"
              : "services.suspendCardTitle",
          )}
        </CardTitle>
        <CardDescription>
          {t(
            isSuspended
              ? "services.resumeCardDescription"
              : "services.suspendCardDescription",
          )}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {isSuspended ? (
          <Button onClick={() => void handleResume()} disabled={busy}>
            {busy && pending === "resume" ? (
              <Loader2 className="animate-spin" />
            ) : null}
            {t("services.actionResume")}
          </Button>
        ) : (
          <Button
            variant="outline"
            onClick={() => setOpen(true)}
            disabled={busy}
          >
            {busy && pending === "suspend" ? (
              <Loader2 className="animate-spin" />
            ) : null}
            {t("services.actionSuspend")}
          </Button>
        )}
      </CardContent>

      <ConfirmDialog
        open={open}
        onOpenChange={setOpen}
        title={t("services.confirmSuspendTitle", { name: service.name })}
        description={t("services.confirmSuspendBody", { name: service.name })}
        cancelLabel={t("services.confirmCancel")}
        confirmLabel={t("services.actionSuspend")}
        onConfirm={() => void handleSuspendConfirm()}
      />
    </Card>
  );
}
