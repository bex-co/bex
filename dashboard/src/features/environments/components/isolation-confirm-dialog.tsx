import { Loader2 } from "lucide-react";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * Render's confirmation before an environment's cross-environment connections
 * are blocked or allowed again (w1/m683), shared by the settings page's
 * Cross-Environment Connections card and the section's ••• menu toggle:
 * "Restrict connections" warns that cross-boundary traffic is cut and that
 * open connections need a restart; "Remove restrictions" says traffic will no
 * longer be blocked. The menu keeps a refusal in the dialog (`error`), as
 * Render's menu does; the card closes it and shows the refusal on the card.
 */
export function IsolationConfirmDialog({
  open,
  onOpenChange,
  restrict,
  projectName,
  environmentName,
  pending,
  error,
  onConfirm,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** True to block the connections, false to allow them again. */
  restrict: boolean;
  projectName: string;
  environmentName: string;
  pending: boolean;
  /** The refusal of the last attempt, shown in the dialog. */
  error: string | null;
  onConfirm: () => void;
}) {
  const { t } = useTranslations();
  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t(
        restrict
          ? "environments.isolateTitle"
          : "environments.removeRestrictionsTitle",
      )}
      description={t("environments.isolateDescription", {
        environment: `${projectName} / ${environmentName}`,
      })}
      cancelLabel={t("environments.cancel")}
      confirmLabel={
        <>
          {pending ? <Loader2 className="animate-spin" /> : null}
          {t(
            restrict
              ? "environments.isolateTitle"
              : "environments.removeRestrictionsConfirm",
          )}
        </>
      }
      pending={pending}
      closeOnConfirm={false}
      onConfirm={onConfirm}
    >
      <div className="space-y-3 text-sm">
        {restrict ? (
          <>
            <p>{t("environments.isolateRestrictBody")}</p>
            <p>{t("environments.isolateRestrictRestart")}</p>
          </>
        ) : (
          <p>{t("environments.isolateRemoveBody")}</p>
        )}
        <p>{t("environments.isolateQuestion")}</p>
        {error ? (
          <Alert variant="destructive" role="alert">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        ) : null}
      </div>
    </ConfirmDialog>
  );
}
