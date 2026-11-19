import { cn } from "@/common/lib/utils/utils";

/**
 * The radio indicator drawn inside a card-style radio option (Edge Caching,
 * workspace settings, the instance-type picker): decorative, the option
 * itself carries the radio role and its checked state.
 */
export function RadioDot({
  checked,
  className,
}: {
  checked: boolean;
  className?: string;
}) {
  return (
    <span
      aria-hidden
      className={cn(
        "flex size-4 shrink-0 items-center justify-center rounded-full border",
        checked ? "border-primary" : "border-muted-foreground/50",
        className,
      )}
    >
      {checked ? <span className="bg-primary size-2 rounded-full" /> : null}
    </span>
  );
}
