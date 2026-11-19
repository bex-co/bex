/**
 * A row's one link that opens the whole row: its ::after covers the nearest
 * positioned ancestor (the row, `relative`), and the focus ring is drawn on
 * that cover, around the row rather than the link's text. Anything else in the
 * row that must stay clickable sits above it (`relative z-10`).
 */
export const STRETCHED_LINK_CLASS =
  "after:absolute after:inset-0 after:content-[''] focus-visible:outline-none focus-visible:after:ring-2 focus-visible:after:ring-ring focus-visible:after:ring-inset";
