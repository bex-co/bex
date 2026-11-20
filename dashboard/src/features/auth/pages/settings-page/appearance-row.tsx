import type { ReactNode } from "react";

/**
 * One of Appearance's three rows (w3/m531): the setting's name, with its
 * description when it has one, beside its control, stacked on a phone. The
 * card and its skeleton share it, so the two keep one geometry; every control
 * sits in an `h-9` slot.
 */
export function AppearanceRow({
  labelId,
  label,
  description,
  children,
}: {
  labelId?: string;
  label: ReactNode;
  description?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-3 px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
      <div className="min-w-0 space-y-1">
        <p id={labelId} className="text-sm leading-none font-medium">
          {label}
        </p>
        {description ? (
          <p className="text-sm text-muted-foreground">{description}</p>
        ) : null}
      </div>
      <div className="flex h-9 shrink-0 items-center">{children}</div>
    </div>
  );
}

/** The card's own frame: one bordered list of rows, no header. */
export const APPEARANCE_CARD_CLASS = "gap-0 divide-y py-0";
/** Each select's width, the skeleton's box included. */
export const APPEARANCE_SELECT_CLASS = "w-full sm:w-56";
