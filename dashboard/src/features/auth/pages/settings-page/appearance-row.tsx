import type { ReactNode } from "react";

/**
 * One of Appearance's three rows (w3/m531): the setting's name, with its
 * description when it has one, beside its control, stacked on a phone. The
 * card and its skeleton share it, so the two keep one geometry. The name is
 * centred on the control's first line, an `h-9` select or switch, and a
 * theme row's actions sit under its select (w3/097).
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
    <div className="flex flex-col gap-3 px-5 py-4 sm:flex-row sm:items-start sm:justify-between">
      <div className="min-w-0 space-y-1 sm:flex sm:min-h-9 sm:flex-col sm:justify-center">
        <p id={labelId} className="text-sm leading-none font-medium">
          {label}
        </p>
        {description ? (
          <p className="text-sm text-muted-foreground">{description}</p>
        ) : null}
      </div>
      <div className="shrink-0">{children}</div>
    </div>
  );
}

/** The card's own frame: one bordered list of rows, no header. */
export const APPEARANCE_CARD_CLASS = "gap-0 divide-y py-0";
/** A theme row's control: its select over its actions, both on the right. */
export const APPEARANCE_CONTROL_CLASS = "flex flex-col gap-2 sm:items-end";
/** Each select's width, the skeleton's box included. */
export const APPEARANCE_SELECT_CLASS = "h-9 w-full sm:w-56";
/** A theme row's actions under its select: Edit, or Cancel and Save changes,
 *  as Render's `RowInput` places them. */
export const APPEARANCE_ACTIONS_CLASS = "flex h-8 justify-end gap-2";
/** High Contrast Mode's switch and its Enabled or Disabled, one select high. */
export const APPEARANCE_SWITCH_CLASS = "flex h-9 items-center gap-2 text-sm";
