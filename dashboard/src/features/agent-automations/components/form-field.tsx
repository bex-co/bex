import type { ReactNode } from "react";
import { Label } from "@/common/components/ui/label";
import { cn } from "@/common/lib/utils/utils";

/**
 * A labelled form control with optional help and an error line. The ready
 * form and its pending skeleton both lay their fields out through this, so
 * label, control and help occupy the same boxes in both. `error` is announced
 * (role="alert") and `describedBy` ties it to the control.
 */
export function FormField({
  id,
  label,
  help,
  error,
  className,
  children,
}: {
  id: string;
  label: ReactNode;
  help?: ReactNode;
  error?: string | null;
  className?: string;
  children: ReactNode;
}) {
  return (
    <div className={cn("space-y-1.5", className)}>
      <Label htmlFor={id}>{label}</Label>
      {children}
      {error ? (
        <p id={`${id}-error`} role="alert" className="text-destructive text-sm">
          {error}
        </p>
      ) : null}
      {help ? (
        <p id={`${id}-help`} className="text-muted-foreground text-xs">
          {help}
        </p>
      ) : null}
    </div>
  );
}

/** A titled group of fields: a fieldset with a visible legend. */
export function FormSection({
  title,
  description,
  children,
}: {
  title: string;
  description?: string;
  children: ReactNode;
}) {
  return (
    <fieldset className="space-y-4">
      <legend className="mb-1 text-base font-semibold">{title}</legend>
      {description ? (
        <p className="text-muted-foreground -mt-2 text-sm">{description}</p>
      ) : null}
      {children}
    </fieldset>
  );
}
