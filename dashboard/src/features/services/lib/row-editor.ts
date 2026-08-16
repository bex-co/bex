/**
 * Row-array helpers shared by the create-time env-var and secret-file editors.
 * Both edit a list of plain objects by index, and both wrote the same two
 * one-liners before this.
 */

/** Drops the row at index i. */
export function removeRow<T>(rows: T[], i: number): T[] {
  return rows.filter((_, idx) => idx !== i);
}

/** Merges patch into the row at index i, leaving every other row untouched. */
export function updateRow<T>(rows: T[], i: number, patch: Partial<T>): T[] {
  return rows.map((row, idx) => (idx === i ? { ...row, ...patch } : row));
}
