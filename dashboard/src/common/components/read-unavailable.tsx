import { Loader2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * A form region whose supporting read failed says so and offers to read it
 * again in place, instead of looking empty or forever loading. Nothing else
 * on the form resets.
 */
export function ReadUnavailable({
  message,
  onRetry,
  retrying,
}: {
  message: string;
  onRetry: () => void;
  retrying: boolean;
}) {
  const { t } = useTranslations();
  return (
    <div
      role="alert"
      className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-dashed p-4 text-sm"
    >
      <p className="text-muted-foreground">{message}</p>
      <Button
        type="button"
        variant="outline"
        size="sm"
        disabled={retrying}
        aria-busy={retrying || undefined}
        onClick={onRetry}
      >
        {retrying ? <Loader2 className="animate-spin" /> : null}
        {t("common.tryAgain")}
      </Button>
    </div>
  );
}
