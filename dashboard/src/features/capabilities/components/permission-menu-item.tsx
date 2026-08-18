import type { ComponentProps } from "react";
import { DropdownMenuItem } from "@/common/components/ui/dropdown-menu";
import { cn } from "@/common/lib/utils/utils";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";

type DropdownItemProps = ComponentProps<typeof DropdownMenuItem>;

export interface PermissionMenuItemProps extends DropdownItemProps {
  /** A definitive authorization denial. The item stays focusable so keyboard
   *  users can discover this explanation, but selection is suppressed. */
  permissionReason?: string;
}

/**
 * A role-aware Radix menu item. Radix's native `disabled` items are removed
 * from roving focus and disable pointer events, making a permission reason
 * unreachable. Permission-denied items instead use `aria-disabled`, remain a
 * tooltip trigger, and guard `onSelect` for both pointer and keyboard input.
 * Ordinary state-based disabling (busy, suspended, etc.) still uses Radix's
 * real `disabled` behavior.
 */
export function PermissionMenuItem({
  permissionReason,
  disabled,
  className,
  onSelect,
  ...props
}: PermissionMenuItemProps) {
  if (!permissionReason) {
    return (
      <DropdownMenuItem
        {...props}
        className={className}
        disabled={disabled}
        onSelect={onSelect}
      />
    );
  }

  return (
    <PermissionTooltip reason={permissionReason} triggerAsChild>
      <DropdownMenuItem
        {...props}
        aria-disabled="true"
        className={cn(
          "cursor-not-allowed opacity-50 focus:bg-transparent",
          className,
        )}
        onSelect={(event) => {
          event.preventDefault();
        }}
      />
    </PermissionTooltip>
  );
}
