import { Children } from "react";
import { X } from "lucide-react";
import { Badge } from "@/common/components/ui/badge";
import { cn } from "@/common/lib/utils/utils";

export interface RemovableChipProps {
  /** Optional leading glyph, sized to the chip (`size-3`). */
  icon?: React.ReactNode;
  children: React.ReactNode;
  /** Accessible label of the remove button — always name what is removed. */
  removeLabel: string;
  onRemove: () => void;
  className?: string;
}

/** A secondary Badge carrying an inline dismiss button: an applied log filter,
 *  an instance selection, a composer mention.
 *
 *  The chip never outgrows its row (w4/324: a discovered instance id made a
 *  406px chip in a 358px phone row, with its remove button off screen). Its
 *  text shrinks and ellipsizes; the full value stays in the DOM for assistive
 *  technology and in a tooltip, and the remove button keeps its size. */
export function RemovableChip({
  icon,
  children,
  removeLabel,
  onRemove,
  className,
}: RemovableChipProps) {
  const text = Children.toArray(children)
    .filter((child) => typeof child === "string" || typeof child === "number")
    .join("");
  return (
    <Badge
      variant="secondary"
      className={cn("max-w-full min-w-0 gap-1 pr-1", className)}
    >
      {icon}
      <span className="min-w-0 truncate" title={text || undefined}>
        {children}
      </span>
      <button
        type="button"
        aria-label={removeLabel}
        onClick={onRemove}
        className="hover:bg-muted-foreground/20 shrink-0 rounded-full p-0.5"
      >
        <X className="size-3" />
      </button>
    </Badge>
  );
}
