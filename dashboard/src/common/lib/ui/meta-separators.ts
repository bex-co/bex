/**
 * A meta line's items separated by a muted "·" before each item after the
 * first (the deploy list's and the event feed's detail lines). Put it on the
 * flex container whose direct children are the items.
 */
export const META_SEPARATED_CLASS =
  "[&>*:not(:first-child)]:before:mr-1.5 [&>*:not(:first-child)]:before:inline-block [&>*:not(:first-child)]:before:text-muted-foreground [&>*:not(:first-child)]:before:content-['·']";
