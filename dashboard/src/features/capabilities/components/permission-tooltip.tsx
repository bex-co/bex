import type { ReactElement, ReactNode } from "react";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/common/components/ui/tooltip";

interface PermissionTooltipProps {
  /** The role reason to show; when falsy the child renders unwrapped (allowed). */
  reason?: ReactNode;
  /** Use the child itself as the trigger when it remains focusable (for example,
   *  an aria-disabled menu item). Disabled native buttons need the default
   *  focusable wrapper because they swallow pointer and focus events. */
  triggerAsChild?: boolean;
  children: ReactElement;
}

/**
 * Attaches a role-reason tooltip to a (typically disabled) control (w9/m84). A
 * disabled button swallows pointer events, so the hover/focus target is a
 * wrapping span rather than the child — that is what lets the reason surface on
 * a control the caller cannot use. When `reason` is falsy the child renders
 * unwrapped, so an allowed control keeps its normal markup and behavior.
 */
export function PermissionTooltip({
  reason,
  triggerAsChild = false,
  children,
}: PermissionTooltipProps) {
  if (!reason) return <>{children}</>;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        {triggerAsChild ? (
          children
        ) : (
          <span className="inline-flex" tabIndex={0}>
            {children}
          </span>
        )}
      </TooltipTrigger>
      <TooltipContent>{reason}</TooltipContent>
    </Tooltip>
  );
}
