import type { ReactNode } from "react";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/common/components/ui/tooltip";

interface PermissionTooltipProps {
  /** The role reason to show; when falsy the child renders unwrapped (allowed). */
  reason?: ReactNode;
  children: ReactNode;
}

/**
 * Attaches a role-reason tooltip to a (typically disabled) control (w9/m84). A
 * disabled button swallows pointer events, so the hover/focus target is a
 * wrapping span rather than the child — that is what lets the reason surface on
 * a control the caller cannot use. When `reason` is falsy the child renders
 * unwrapped, so an allowed control keeps its normal markup and behavior.
 */
export function PermissionTooltip({ reason, children }: PermissionTooltipProps) {
  if (!reason) return <>{children}</>;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="inline-flex" tabIndex={0}>
          {children}
        </span>
      </TooltipTrigger>
      <TooltipContent>{reason}</TooltipContent>
    </Tooltip>
  );
}
