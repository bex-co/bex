/**
 * The project page header's boxes, shared by the ready header
 * (`ProjectPageHeader`) and its loading preview (`ProjectOverviewPageSkeleton`)
 * so both lay out the same rows (w1/m682). Kept apart from the header's
 * component so the preview, which ships in the entry chunk, pulls in no popover.
 */
export const PROJECT_HEADER_CLASS = "space-y-1 border-b pb-6";
export const PROJECT_HEADER_ROW_CLASS =
  "flex flex-col items-start gap-3 sm:min-h-9 sm:flex-row sm:items-center sm:justify-between";
