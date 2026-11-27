import { Check, Loader2, TriangleAlert } from "lucide-react";
import { cn } from "@/common/lib/utils/utils";
import type { ImageCheck } from "@/features/services/hooks/use-image-check";

/** The field's end icon: a spinner while it checks, then a mark. */
export function ImageCheckIcon({
  status,
  invalid,
  checkingLabel,
}: {
  status: ImageCheck["status"];
  /** The field shows a failure: its own syntax, the server's, or the check's. */
  invalid: boolean;
  checkingLabel: string;
}) {
  const className =
    "pointer-events-none absolute inset-y-0 right-3 my-auto size-4 shrink-0";
  if (invalid) {
    return (
      <TriangleAlert
        aria-hidden="true"
        data-testid="image-check-caution"
        className={cn(className, "text-destructive")}
      />
    );
  }
  if (status === "checking") {
    return (
      <Loader2
        role="status"
        aria-label={checkingLabel}
        data-testid="image-check-spinner"
        className={cn(className, "animate-spin text-muted-foreground")}
      />
    );
  }
  if (status === "deployable") {
    return (
      <Check
        aria-hidden="true"
        data-testid="image-check-success"
        className={cn(className, "text-green-700 dark:text-green-500")}
      />
    );
  }
  return null;
}
