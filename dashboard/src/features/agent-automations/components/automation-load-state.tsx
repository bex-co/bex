import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { isForbiddenError, isNotFoundError } from "@/common/lib/graphql-error";

/**
 * A failed automations read: a refusal (the caller cannot start sessions, so
 * cannot see automations), a missing automation, or an outage with Retry.
 */
export function AutomationLoadError({
  error,
  onRetry,
}: {
  error: unknown;
  onRetry: () => void;
}) {
  const { t } = useTranslations();
  const forbidden = isForbiddenError(error);
  const notFound = isNotFoundError(error);
  return (
    <Alert variant="destructive" data-testid="automation-load-error">
      <AlertDescription className="flex flex-wrap items-center gap-3">
        <span>
          {forbidden
            ? t("agentAutomations.forbidden")
            : notFound
              ? t("agentAutomations.notFound")
              : t("agentAutomations.loadFailed")}
        </span>
        {forbidden || notFound ? null : (
          <Button size="sm" variant="outline" onClick={onRetry}>
            {t("agentAutomations.retry")}
          </Button>
        )}
      </AlertDescription>
    </Alert>
  );
}
