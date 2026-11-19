// The create dialog's name step and the permissions editor share one layout:
// the header and footer stay put and the middle row (name and picker) scrolls
// between them, so a long permission list never pushes the action out of
// reach. Below `sm` the padding narrows so the picker's widest row (None,
// Read, Write, Read & write) fits a 360 px screen on one line.

/** DialogContent: a bounded column whose middle row scrolls. */
export const SCROLLING_DIALOG_CLASS =
  "max-h-[90dvh] grid-cols-[minmax(0,1fr)] grid-rows-[auto_minmax(0,1fr)_auto]";

/** DialogContent while it shows the picker: wider, with the narrow-screen padding. */
export const PICKER_DIALOG_CLASS = "p-4 sm:max-w-2xl sm:p-6";

/** The scrolling middle row, reaching the dialog's edges so its scrollbar sits there. */
export const PICKER_DIALOG_BODY_CLASS =
  "-mx-4 -my-1 min-h-0 overflow-y-auto px-4 py-1 sm:-mx-6 sm:px-6";
