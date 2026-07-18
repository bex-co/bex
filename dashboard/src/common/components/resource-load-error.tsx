import { AlertTriangle } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * Settled-error state for a detail page whose resource query failed (w9/m55).
 * Distinct from not-found — a dead id redirects home via
 * `useNotFoundRedirect`; a failed query stays put here so a backend outage
 * never masquerades as a deleted resource.
 */
export function ResourceLoadError({ onRetry }: { onRetry?: () => void }) {
  const { t } = useTranslations();
  return (
    <div className="flex flex-col items-center gap-2 py-12 text-center">
      <AlertTriangle className="size-8 text-destructive" />
      <p className="font-medium">{t("common.errorTitle")}</p>
      <p className="text-sm text-muted-foreground">
        {t("common.resourceErrorBody")}
      </p>
      {onRetry ? (
        <Button variant="outline" size="sm" onClick={onRetry}>
          {t("common.tryAgain")}
        </Button>
      ) : null}
    </div>
  );
}
