import { useCallback, useMemo, useState } from "react";
import {
  resourceSelectionKey,
  type ResourceRow,
} from "@/features/projects/types";

export interface ResourceSelection {
  /** The selected keys still present in `rows` (see `useResourceSelection`). */
  selected: ReadonlySet<string>;
  /** The selected rows, in list order. */
  selectedRows: ResourceRow[];
  /** Replace the selection; the table's checkboxes call this. */
  setSelected: (keys: Set<string>) => void;
  /** Deselect these keys, keeping every other selected row. */
  deselect: (keys: ReadonlySet<string>) => void;
}

/**
 * Row selection for one resource list, shared by a project environment's card
 * and the Overview's ungrouped list (w4/m550).
 *
 * A selection only counts while its resource is still in `rows`: a move that
 * failed partway, or timed out after it was applied, leaves keys for rows that
 * are gone (w4/m206). Pass the list's whole membership, not its filtered view,
 * so a row merely hidden by the search stays selected. The keys are derived,
 * not deleted, so a list that is momentarily unread loses nothing.
 */
export function useResourceSelection(rows: ResourceRow[]): ResourceSelection {
  const [keys, setKeys] = useState<Set<string>>(() => new Set());
  const selected = useMemo(() => {
    const here = new Set(rows.map(resourceSelectionKey));
    const kept = [...keys].filter((key) => here.has(key));
    return kept.length === keys.size ? keys : new Set(kept);
  }, [rows, keys]);
  const selectedRows = useMemo(
    () => rows.filter((row) => selected.has(resourceSelectionKey(row))),
    [rows, selected],
  );
  // An update, not a replacement: an action that settles after the user
  // changed the selection must not undo the change.
  const deselect = useCallback(
    (drop: ReadonlySet<string>) =>
      setKeys((previous) => {
        const next = new Set([...previous].filter((key) => !drop.has(key)));
        return next.size === previous.size ? previous : next;
      }),
    [],
  );
  return { selected, selectedRows, setSelected: setKeys, deselect };
}
