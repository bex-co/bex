import { AlertTriangle, RotateCw } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import type { RefusalKind } from "@/common/lib/graphql-error";
import { cn } from "@/common/lib/utils/utils";

/**
 * A failed read in one line: the warning icon, what failed and why, and Try
 * again when asking again can help. The page's one inline error shape (the
 * groups card's scope error uses it too).
 */
export function SectionErrorNotice({
  error,
  spaced = false,
}: {
  error: {
    kind: RefusalKind;
    title: string;
    body?: string;
    onRetry?: () => void;
  };
  spaced?: boolean;
}) {
  const { t } = useTranslations();
  return (
    <div
      role="alert"
      className={cn(
        "flex flex-wrap items-center gap-x-3 gap-y-2 text-sm max-md:text-base",
        spaced && "mb-4",
      )}
    >
      <AlertTriangle className="text-destructive size-4 shrink-0" />
      <p className="min-w-0 flex-1">
        <span className="text-destructive font-medium">{error.title}</span>
        {error.body ? (
          <>
            {" "}
            <span className="text-muted-foreground">{error.body}</span>
          </>
        ) : null}
      </p>
      {error.kind === "generic" && error.onRetry ? (
        <Button variant="outline" size="sm" onClick={error.onRetry}>
          <RotateCw /> {t("common.tryAgain")}
        </Button>
      ) : null}
    </div>
  );
}
