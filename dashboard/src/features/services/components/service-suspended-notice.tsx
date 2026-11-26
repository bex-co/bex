import { PauseCircle } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { ResumeServiceButton } from "@/features/services/components/resume-service-button";
import type { ProtectedActionResult } from "@/features/services/lib/protected-confirmation";
import { suspendedNoticeTitleKey } from "@/features/services/lib/status";
import type { LifecycleAction, ServiceView } from "@/features/services/types";

/** The notice's box, which its loading frame draws too. */
export const SUSPENDED_NOTICE_CLASS =
  "bg-muted/40 flex min-h-11 flex-wrap items-center gap-x-2 gap-y-0.5 rounded-md border px-3 py-1.5 text-sm";

/**
 * Render's notice under a suspended service's facts, which stands in for the
 * header's actions while it is suspended, on every type (w3/m555, w3/107):
 * Render's title for whoever suspended it, that a suspended service is not
 * billed, and Resume service, gated by the server's `resume` decision like
 * every other Resume.
 */
export function ServiceSuspendedNotice({
  service,
  pending,
  onRun,
}: {
  service: ServiceView;
  pending: LifecycleAction | null;
  onRun?: (
    action: LifecycleAction,
    service: ServiceView,
    confirmation?: string,
  ) => Promise<ProtectedActionResult>;
}) {
  const { t } = useTranslations();
  return (
    <div
      role="status"
      data-suspended-notice=""
      className={SUSPENDED_NOTICE_CLASS}
    >
      <PauseCircle
        aria-hidden="true"
        className="text-muted-foreground size-4 shrink-0"
      />
      <span className="font-medium">{t(suspendedNoticeTitleKey(service))}</span>{" "}
      <span className="text-muted-foreground">
        {t("services.suspendedNoticeBody")}
      </span>{" "}
      {onRun ? (
        <ResumeServiceButton
          service={service}
          pending={pending}
          onRun={onRun}
          size="sm"
          variant="link"
          label={t("services.suspendedNoticeResume")}
          className="h-8 px-0"
        />
      ) : null}
    </div>
  );
}
