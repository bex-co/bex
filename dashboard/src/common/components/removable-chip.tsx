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
 *  an instance selection, a composer mention. */
export function RemovableChip({
  icon,
  children,
  removeLabel,
  onRemove,
  className,
}: RemovableChipProps) {
  return (
    <Badge variant="secondary" className={cn("gap-1 pr-1", className)}>
      {icon}
      {children}
      <button
        type="button"
        aria-label={removeLabel}
        onClick={onRemove}
        className="hover:bg-muted-foreground/20 rounded-full p-0.5"
      >
        <X className="size-3" />
      </button>
    </Badge>
  );
}
